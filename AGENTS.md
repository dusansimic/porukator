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
  ├─ AdminService     (user session auth)      ← web UI
  ├─ ProducerService  (API-token auth)         ← your upstream services
  └─ ClientService    (client-token auth)       ← Android app
```

- **Three auth surfaces**, enforced by one Connect interceptor keyed on the
  procedure path (`internal/auth`):
  - **User accounts + sessions** gate AdminService and the web UI. `Login`
    (username + password) is exempt and mints an opaque **session token** stored
    sha256-hashed in `sessions`; the browser sends it as `Authorization: Bearer`.
    Passwords are **argon2id**-hashed (`internal/auth/password.go`). The
    interceptor resolves the session → user, rejects disabled/expired, and
    injects the user into the context.
  - **Two roles**: **admin** (everything) and **manager** (only their own client
    devices + those devices' messages). A coarse role gate in the interceptor
    rejects manager calls to admin-only procedures with `PermissionDenied`;
    per-device ownership is enforced in the handlers.
  - Admin-only: settings, user management (create / set-role / disable / delete)
    and session management (list / revoke). Disabling a user deletes all their
    sessions; admins may act on themselves (no last-admin guard). Bootstrap the
    first admin with `authctl create --admin` (bundled in the server image).
  - **API tokens** (created in the UI, sha256-hashed at rest) gate
    ProducerService. **Managers may create them too** — tokens are owned
    (`api_tokens.created_by`) and managed only by their creator + admins.
  - **Client access tokens** (created per device, sha256-hashed) gate
    ClientService; the device identity is derived from the token, never sent in
    the request.
- **Ownership scoping**: `clients.created_by` / `api_tokens.created_by` record the
  creating user; managers see/manage only their own devices and tokens, admins see
  all. **A ProducerService call is scoped to its API key's owner**: a
  manager-owned key lists/sends only to that manager's devices; an admin-owned (or
  legacy NULL-owner) key reaches all. A disabled owner's keys are rejected;
  deleting an owner removes their keys (`ON DELETE CASCADE`).
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
`received_at` / `dispatched_at` / `sent_at`. Producers poll status via
`ProducerService.GetMessages` (by the ids `SendMessages` returned — strict, fails
`PermissionDenied` if any id isn't visible to the key) or `ListMessages`
(`batch_id` / `status` filters), both owner-scoped like everything else.

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
then runs the backend and the web UI as mprocs panes plus an infra log tail.
**The backend listens on `:8080`, so that port must be free.** Bootstrap a login
once with `just create-user admin dev-pass true` (username password is-admin),
then sign in. Quit with `q` (or Ctrl-C) — `just dev` then stops the docker
compose stack automatically. `mprocs.yaml` defines the panes; press `r` on the
`server` pane to rebuild + restart after Go changes.

### Service
```bash
cp config/config.example.yaml config/config.yaml
just proto      # buf lint + generate Go (gen/go)
just sqlc-gen   # regenerate repository from queries + migrations
just build && just run
just test       # go test -race ./...
just docker-up  # postgres + service via docker compose

# Bootstrap the first admin (authctl; runs migrations, then inserts):
just build-authctl && ./authctl create --username admin --password <pw> --admin
```
Config is `config/config.yaml` plus `PORUKATOR_*` env overrides
(`PORUKATOR_POSTGRES_URL`, `PORUKATOR_HTTP_ADDR`, `PORUKATOR_HTTP_PUBLIC_HOST`).
Migrations run on boot. Web-UI access is via user accounts (no master password).

### User admin CLI (`authctl`)

Manages web-UI user accounts **directly against the database** (`cmd/authctl`,
built with `just build-authctl`). It is **bundled in the server image** (on
`PATH`) for in-container administration — chiefly bootstrapping the first admin,
which can't go through AdminService (that requires an existing admin). Reads the
same config/env as the server (`PORUKATOR_POSTGRES_URL`); runs migrations on
start.

```bash
authctl create --username admin --password <pw> --admin   # manager without --admin
authctl list
authctl set-role --username u --role admin|manager
authctl disable --username u      # blocks login + revokes their sessions
authctl enable  --username u
authctl passwd  --username u      # prompts if --password omitted
authctl delete  --username u
```
In a deployment: `docker compose exec porukator authctl create --username admin
--password <pw> --admin`.

### Test CLI (`porukatorctl`)

A small ProducerService client for exercising a running service by hand
(`cmd/porukatorctl`, built with `just build-ctl`). Pass `--host`/`--token` (or
env `PORUKATOR_HOST`/`PORUKATOR_TOKEN`); add `--json` for raw output.

```bash
just build-ctl
./porukatorctl --host localhost:8080 --token <api-token> list
./porukatorctl --host localhost:8080 --token <api-token> send \
    --to +38160111 --to +38160222 --message "hello" --client <device-id>
./porukatorctl --host localhost:8080 --token <api-token> status --batch <batch-id>
./porukatorctl --host localhost:8080 --token <api-token> status --id <msg-id>
```
`send` requires at least one `--client`; the message body is sent to every
`--to`, balanced round-robin across the given devices. `status` polls delivery —
`--id` (one or more, via `GetMessages`) or `--batch`/`--status` (via
`ListMessages`).

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

## Container images

Two images, built from `deployments/`:

- **server** (`Dockerfile.server`) — builds and runs the Go service.
- **webui** (`Dockerfile.webui`) — builds the SPA in Node, serves the static
  bundle with nginx (SPA history fallback only — **no backend proxying**).

The web UI assumes an **external reverse proxy** in front that routes
`/porukator.v1.*` to the server and everything else to the webui; the SPA issues
same-origin requests that reach the backend through it.

```bash
just images-build   # build ghcr.io/dusansimic/porukator/{server,webui}:latest
just images-push    # build + push (docker login ghcr.io -u dusansimic first)
just docker-up      # full local stack: postgres + server + webui via docker compose
```

## Pre-commit hooks

`.pre-commit-config.yaml` gates commits with the project's own tooling (run
`pre-commit install` once per clone): **buf lint** (proto), **gofmt** + **go vet**
(backend), and **biome** format + lint (web UI; config in `webui/biome.json`,
scoped to `src` excluding generated code and Tailwind CSS). Run them by hand with
`pre-commit run --all-files`.

## Commit conventions

Keep commit messages clean: Conventional Commits, imperative subject, body only
when the *why* isn't obvious. **Do not add `Co-Authored-By` trailers or any AI
attribution** to commits. **When asked to commit, run the commit yourself** —
don't just print the message for the user to paste.

## End-to-end smoke test

With the service up (`just docker-up` or `just run`):
1. `authctl create --username admin --password pw --admin`, then
   `AdminService.Login` with those credentials → session token.
2. `CreateClient` → access token; `CreateApiToken` → producer secret.
3. Open `ClientService.StreamJobs` with the client token → `ListClients` shows it
   online.
4. `ProducerService.SendMessages` to that client → job arrives on the stream.
5. `ClientService.ReportDelivery` → the row flips to `SENT` with `sent_at`.
6. Close the stream → offline; messages sent meanwhile stay `PENDING` and drain
   on reconnect.
