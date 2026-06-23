// Package auth provides token hashing/generation and a Connect interceptor that
// applies the right credential check per service:
//   - AdminService/*    require the master password as a bearer token.
//   - ProducerService/* require a valid API token.
//   - ClientService/*   require a valid client access token; the client id is
//     injected into the request context.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dusansimic/porukator/internal/pgconv"
	"github.com/dusansimic/porukator/internal/repository"
)

// HashToken returns the hex sha256 of a token; only hashes are stored.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GenerateToken returns a new random URL-safe token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type ctxKey int

const clientIDKey ctxKey = iota

// ClientIDFromContext returns the authenticated client id for ClientService
// requests.
func ClientIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(clientIDKey).(string)
	return id, ok
}

// Interceptor enforces per-service authentication.
type Interceptor struct {
	pool           *pgxpool.Pool
	masterPassword string
}

func NewInterceptor(pool *pgxpool.Pool, masterPassword string) *Interceptor {
	return &Interceptor{pool: pool, masterPassword: masterPassword}
}

var errUnauthenticated = connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or missing credentials"))

// loginProcedure is exempt from auth: it validates the password itself.
const loginProcedure = "/porukator.v1.AdminService/Login"

func bearer(h interface{ Get(string) string }) string {
	v := h.Get("Authorization")
	if v == "" {
		return ""
	}
	const p = "Bearer "
	if strings.HasPrefix(v, p) {
		return strings.TrimPrefix(v, p)
	}
	return v
}

// authenticate checks the credential for a procedure and returns a possibly
// augmented context (with the client id for ClientService).
func (i *Interceptor) authenticate(ctx context.Context, procedure, token string) (context.Context, error) {
	switch {
	case strings.Contains(procedure, "AdminService"):
		if token == "" || i.masterPassword == "" ||
			subtle.ConstantTimeCompare([]byte(token), []byte(i.masterPassword)) != 1 {
			return ctx, errUnauthenticated
		}
		return ctx, nil

	case strings.Contains(procedure, "ProducerService"):
		if token == "" {
			return ctx, errUnauthenticated
		}
		q := repository.New(i.pool)
		tok, err := q.GetApiTokenByHash(ctx, HashToken(token))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ctx, errUnauthenticated
			}
			return ctx, connect.NewError(connect.CodeInternal, err)
		}
		_ = q.TouchApiTokenLastUsed(ctx, tok.ID)
		return ctx, nil

	case strings.Contains(procedure, "ClientService"):
		if token == "" {
			return ctx, errUnauthenticated
		}
		q := repository.New(i.pool)
		c, err := q.GetClientByTokenHash(ctx, HashToken(token))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ctx, errUnauthenticated
			}
			return ctx, connect.NewError(connect.CodeInternal, err)
		}
		return context.WithValue(ctx, clientIDKey, pgconv.UUIDString(c.ID)), nil

	default:
		return ctx, errUnauthenticated
	}
}

// WrapUnary implements connect.Interceptor.
func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if req.Spec().Procedure == loginProcedure {
			return next(ctx, req)
		}
		ctx, err := i.authenticate(ctx, req.Spec().Procedure, bearer(req.Header()))
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

// WrapStreamingClient implements connect.Interceptor (no-op; we are server-side).
func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

// WrapStreamingHandler implements connect.Interceptor.
func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authenticate(ctx, conn.Spec().Procedure, bearer(conn.RequestHeader()))
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}
