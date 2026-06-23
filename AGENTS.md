# Porukator

Self-hosted SMS gateway that replaces Twilio using your own Android phones as
the SMS transport. Upstream services submit `(phone_number, message)` jobs plus
a set of client devices; the service balances the jobs across those devices,
each device sends them as real SMS from its SIM and reports delivery back. A web
UI manages credentials, devices, pacing and the message log.

This is a **self-contained monorepo**: it owns its protobuf contract, a Go
service, a React web UI, and an Android app. `CLAUDE.md` is a symlink to this
file.

## Architecture

```
proto/porukator/v1/porukator.proto   ── one contract, three services
  ├─ AdminService     (master-password auth)  ← web UI
  ├─ ProducerService  (API-token auth)         ← your upstream services
  └─ ClientService    (client-token auth)       ← Android app
```

- **Three auth surfaces**, enforced by one Connect interceptor keyed on the
  procedure path (`internal/auth`):
  - **Master password** (Viper config `auth.master_password`) gates AdminService
    and the web UI. `Login` is exempt and validates the password itself.
  - **API tokens** (created in the UI, sha256-hashed at rest) gate
    ProducerService.
  - **Client access tokens** (created per device, sha256-hashed) gate
    ClientService; the device identity is derived from the token, never sent in
    the request.
- **Pacing**: the service stores global `delay_ms` + `jitter_ms` (the `settings`
  table) and ships them with every `Job`. The **client paces its own sends** —
  it waits `delay_ms + random(0, jitter_ms)` between SMS — and **reports each
  delivery**, from which the service records `sent_at`.
- **Balancing**: `SendMessages` distributes the batch round-robin across the
  caller-supplied `client_ids`.
- **Online** = the device currently holds an open `StreamJobs` server stream.
  Tracked in-memory (`internal/registry`); `last_seen_at` is persisted on
  disconnect.
- **Exactly-once dispatch**: messages are inserted `PENDING`; the stream path is
  the sole dispatcher and uses a `MarkDispatched` row-count guard so a message
  that appears both in the offline-drain and the live channel is sent once.
  Offline devices' messages stay `PENDING` and are drained on next connect.

Message lifecycle: `PENDING → DISPATCHED → SENT | FAILED`, with timestamps
`received_at` / `dispatched_at` / `sent_at`.

## Layout

```
proto/porukator/v1/   protobuf contract
gen/go/               generated Go (committed)
cmd/porukator/        main.go
internal/
  config/   viper, env prefix PORUKATOR_
  auth/     token hash/gen + Connect interceptor
  db/       pgxpool + golang-migrate + embedded migrations
  repository/  sqlc-generated
  registry/    in-memory online clients + per-client job channels
  connectsrv/  Admin/Producer/Client handlers + h2c server
queries/      sqlc SQL
webui/        React SPA (Vite, connect-query, shadcn, zustand)
android/      Kotlin app (Compose, connect-kotlin, CameraX/ML Kit)
deployments/  Dockerfile + docker-compose.yml
```

## Tech stack

- **Go 1.25+** service: Connect (`connectrpc.com/connect`) over h2c, Postgres via
  pgx + sqlc + golang-migrate, Viper config, zap logging, oklog/run shutdown.
- **Web UI**: Vite + React + TypeScript, react-router, `@connectrpc/connect-query`
  + TanStack Query, shadcn/ui + Tailwind, zustand. **pnpm** for packages.
- **Android**: Kotlin + Jetpack Compose, connect-kotlin (Java-Lite messages),
  CameraX + ML Kit for QR scanning, DataStore for config, a foreground service
  for sending.

## Build & run

### Everything in one terminal (`just dev`)

```bash
just dev   # deps + codegen + Postgres (waited healthy) + backend + web UI, under mprocs
```
Requires [`mprocs`](https://github.com/pvolok/mprocs). It starts Postgres
(published on host port **55432** to avoid clashing with other local Postgres),
then runs the backend (master password `dev-pass`) and the web UI as mprocs
panes plus an infra log tail. **The backend listens on `:8080`, so that port must
be free.** Quit with `q` (or Ctrl-C) — `just dev` then stops the docker compose
stack automatically. `mprocs.yaml` defines the panes; press `r` on the `server`
pane to rebuild + restart after Go changes.

### Service
```bash
cp config/config.example.yaml config/config.yaml   # set auth.master_password
just proto      # buf lint + generate Go (gen/go)
just sqlc-gen   # regenerate repository from queries + migrations
just build && just run
just test       # go test -race ./...
just docker-up  # postgres + service via docker compose
```
Config is `config/config.yaml` plus `PORUKATOR_*` env overrides
(`PORUKATOR_AUTH_MASTER_PASSWORD`, `PORUKATOR_POSTGRES_URL`,
`PORUKATOR_HTTP_ADDR`, `PORUKATOR_HTTP_PUBLIC_HOST`). Migrations run on boot.

### Web UI
```bash
cd webui
pnpm install
pnpm proto      # buf generate TS into src/gen
pnpm dev        # proxies /porukator.v1.* to the Go server on :8080
pnpm build
```

### Android
```bash
cd android
buf generate    # regenerate Java-Lite + connect-kotlin into app/src/main/java
./gradlew assembleDebug    # requires Android SDK (set ANDROID_HOME / sdk.dir)
```
SMS sending needs a **real device with a SIM**. Grant SEND_SMS, CAMERA and
POST_NOTIFICATIONS. Configure a device by typing host + token or scanning the QR
the web UI shows when you add a device.

## Regenerating the contract

Edit `proto/porukator/v1/porukator.proto`, then regenerate each consumer:
`just proto` (Go), `cd webui && pnpm proto` (TS), `cd android && buf generate`
(Kotlin/Java). Generated code is committed.

## Commit conventions

Keep commit messages clean: Conventional Commits, imperative subject, body only
when the *why* isn't obvious. **Do not add `Co-Authored-By` trailers or any AI
attribution** to commits.

## End-to-end smoke test

With the service up (`just docker-up` or `just run`):
1. `AdminService.Login` with the master password.
2. `CreateClient` → access token; `CreateApiToken` → producer secret.
3. Open `ClientService.StreamJobs` with the client token → `ListClients` shows it
   online.
4. `ProducerService.SendMessages` to that client → job arrives on the stream.
5. `ClientService.ReportDelivery` → the row flips to `SENT` with `sent_at`.
6. Close the stream → offline; messages sent meanwhile stay `PENDING` and drain
   on reconnect.
