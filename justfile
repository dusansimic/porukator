binary_name := "porukator"
docker_compose := "docker compose -f deployments/docker-compose.yml"
migrations_dir := "internal/db/migrations"
ghcr_image := "ghcr.io/dusansimic/porukator"

# list available recipes
default:
    @just --list

# Quitting mprocs (`q`) or Ctrl-C stops Postgres too. Backend needs :8080 free.
# Run the whole stack in one terminal: Postgres + backend + web UI under mprocs.
dev: dev-setup gen
    #!/usr/bin/env bash
    set -u
    trap '{{docker_compose}} down' EXIT
    {{docker_compose}} up -d --wait postgres
    mprocs

# Install Go + web UI dependencies.
dev-setup:
    go mod download
    pnpm --dir webui install

# Regenerate all generated code: Go (gen/go) + web UI TypeScript (webui/src/gen).
gen:
    buf generate
    pnpm --dir webui run proto

# Start only Postgres (waited healthy); leaves it running.
infra-up:
    {{docker_compose}} up -d --wait postgres

# Regenerate Go code from proto (TS/Kotlin are generated in webui/ and android/).
proto:
    buf lint
    buf generate

# Regenerate sqlc repository from queries + migrations.
sqlc-gen:
    sqlc generate

# Regenerate the Android client stubs (Java-Lite + connect-kotlin).
android-gen:
    cd android && buf generate

# Build the debug APK. Picks a JDK 17-21 (Android Studio's JBR) since newer
# JDKs break AGP; override with JAVA_HOME=/path/to/jdk just android-apk.
android-apk:
    #!/usr/bin/env bash
    set -eu
    cd android
    jdk="${JAVA_HOME:-}"
    if [ -z "$jdk" ] || ! "$jdk/bin/java" -version 2>&1 | grep -qE '"(17|21)\.'; then
        jdk="$(ls -d "$HOME/.local/lib/android-studio/jbr" /opt/android-studio/jbr "$HOME/android-studio/jbr" /usr/lib/jvm/java-21-openjdk* 2>/dev/null | head -1)"
    fi
    [ -n "$jdk" ] || { echo "No JDK 17-21 found; set JAVA_HOME"; exit 1; }
    export JAVA_HOME="$jdk"
    echo "Using JAVA_HOME=$JAVA_HOME"
    ./gradlew :app:assembleDebug
    echo "APK: android/app/build/outputs/apk/debug/app-debug.apk"

# Install the debug APK on a connected device/emulator (adb).
android-install:
    #!/usr/bin/env bash
    set -eu
    cd android
    jdk="${JAVA_HOME:-}"
    if [ -z "$jdk" ] || ! "$jdk/bin/java" -version 2>&1 | grep -qE '"(17|21)\.'; then
        jdk="$(ls -d "$HOME/.local/lib/android-studio/jbr" /opt/android-studio/jbr "$HOME/android-studio/jbr" /usr/lib/jvm/java-21-openjdk* 2>/dev/null | head -1)"
    fi
    [ -n "$jdk" ] || { echo "No JDK 17-21 found; set JAVA_HOME"; exit 1; }
    export JAVA_HOME="$jdk"
    ./gradlew :app:installDebug

# Build the server binary.
build:
    go build -o {{binary_name}} ./cmd/porukator

# Build and run.
run: build
    ./{{binary_name}}

# Run tests.
test:
    go test -v -race ./...

# Format code.
fmt:
    go fmt ./...
    buf format -w

# Resolve dependencies.
deps:
    go mod download
    go mod tidy

# Create a timestamped migration pair. Usage: just migrate-new name=add_foo
migrate-new name="":
    migrate create -ext sql -dir {{migrations_dir}} -seq {{name}}

# Apply migrations. Expects PORUKATOR_POSTGRES_URL.
migrate-up:
    migrate -path {{migrations_dir}} -database "$PORUKATOR_POSTGRES_URL" up

# Roll back the last migration.
migrate-down:
    migrate -path {{migrations_dir}} -database "$PORUKATOR_POSTGRES_URL" down 1

# Start dev infrastructure + service.
docker-up:
    {{docker_compose}} up -d --build

# Stop dev environment.
docker-down:
    {{docker_compose}} down

# Tail container logs.
docker-logs:
    {{docker_compose}} logs -f

# Build the server + webui images tagged for GHCR.
images-build:
    docker build -t {{ghcr_image}}/server:latest -f deployments/Dockerfile.server .
    docker build -t {{ghcr_image}}/webui:latest -f deployments/Dockerfile.webui .

# Build and push both images to GHCR. Authenticate first:
#   echo $GITHUB_TOKEN | docker login ghcr.io -u dusansimic --password-stdin
images-push: images-build
    docker push {{ghcr_image}}/server:latest
    docker push {{ghcr_image}}/webui:latest
