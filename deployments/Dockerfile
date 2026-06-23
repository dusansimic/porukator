# Build stage. Context is the porukator repo root (self-contained module).
FROM golang:1.25-alpine AS builder

WORKDIR /build

RUN apk add --no-cache git

# Module files first, for layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Sources.
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -installsuffix cgo \
    -o porukator \
    ./cmd/porukator

# Runtime stage.
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/porukator .

RUN addgroup -S appgroup && \
    adduser -S appuser -G appgroup && \
    chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/porukator"]
