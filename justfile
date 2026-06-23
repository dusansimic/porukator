binary_name := "porukator"
docker_compose := "docker compose -f deployments/docker-compose.yml"
migrations_dir := "internal/db/migrations"

# Regenerate Go code from proto (TS/Kotlin are generated in webui/ and android/).
proto:
    buf lint
    buf generate

# Regenerate sqlc repository from queries + migrations.
sqlc-gen:
    sqlc generate

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
