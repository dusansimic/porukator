// Package auth provides token/password hashing and a Connect interceptor that
// applies the right credential check per service:
//   - AdminService/*    require a session token (from Login); the session's user
//     is injected into the context and admin-only procedures reject managers.
//   - ProducerService/* require a valid API token.
//   - ClientService/*   require a valid client access token; the client id is
//     injected into the request context.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dusansimic/porukator/internal/pgconv"
	"github.com/dusansimic/porukator/internal/repository"
)

// HashToken returns the hex sha256 of a token; only hashes are stored. Used for
// high-entropy tokens (sessions, API tokens, client tokens) — not passwords.
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

// AuthUser is the authenticated web-UI user attached to AdminService requests.
type AuthUser struct {
	ID        string
	Username  string
	Role      repository.UserRole
	SessionID string
}

// IsAdmin reports whether the user has the admin role.
func (u AuthUser) IsAdmin() bool { return u.Role == repository.UserRoleAdmin }

// TokenPrincipal is the API-token caller attached to ProducerService requests.
// GrantsAll is true for admin-owned and legacy (unowned) tokens; otherwise the
// caller is scoped to OwnerID's devices.
type TokenPrincipal struct {
	OwnerID   string
	GrantsAll bool
}

type ctxKey int

const (
	clientIDKey ctxKey = iota
	userKey
	tokenKey
)

// ClientIDFromContext returns the authenticated client id for ClientService
// requests.
func ClientIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(clientIDKey).(string)
	return id, ok
}

// UserFromContext returns the authenticated user for AdminService requests.
func UserFromContext(ctx context.Context) (AuthUser, bool) {
	u, ok := ctx.Value(userKey).(AuthUser)
	return u, ok
}

// TokenFromContext returns the API-token principal for ProducerService requests.
func TokenFromContext(ctx context.Context) (TokenPrincipal, bool) {
	t, ok := ctx.Value(tokenKey).(TokenPrincipal)
	return t, ok
}

// Interceptor enforces per-service authentication.
type Interceptor struct {
	pool *pgxpool.Pool
}

func NewInterceptor(pool *pgxpool.Pool) *Interceptor {
	return &Interceptor{pool: pool}
}

var (
	errUnauthenticated = connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or missing credentials"))
	errForbidden       = connect.NewError(connect.CodePermissionDenied, errors.New("admin role required"))
)

// adminPrefix is the procedure prefix for AdminService.
const adminPrefix = "/porukator.v1.AdminService/"

// loginProcedure is exempt from auth: it validates the credentials itself.
const loginProcedure = adminPrefix + "Login"

// managerAllowed is the set of AdminService procedures a manager may call. Any
// other AdminService procedure requires the admin role. Ownership (manager may
// only touch their own devices/messages) is enforced in the handlers.
var managerAllowed = map[string]bool{
	adminPrefix + "Logout":         true,
	adminPrefix + "GetCurrentUser": true,
	adminPrefix + "CreateClient":   true,
	adminPrefix + "ListClients":    true,
	adminPrefix + "RenameClient":   true,
	adminPrefix + "RevokeClient":   true,
	adminPrefix + "ListMessages":   true,
	// API keys are owned + manager-creatable; ownership is enforced in handlers.
	adminPrefix + "CreateApiToken": true,
	adminPrefix + "ListApiTokens":  true,
	adminPrefix + "RevokeApiToken": true,
}

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
// augmented context (with the user for AdminService, the client id for
// ClientService).
func (i *Interceptor) authenticate(ctx context.Context, procedure, token string) (context.Context, error) {
	switch {
	case strings.HasPrefix(procedure, adminPrefix):
		if token == "" {
			return ctx, errUnauthenticated
		}
		q := repository.New(i.pool)
		s, err := q.GetSessionByHash(ctx, HashToken(token))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ctx, errUnauthenticated
			}
			return ctx, connect.NewError(connect.CodeInternal, err)
		}
		if s.Disabled || !s.ExpiresAt.Valid || s.ExpiresAt.Time.Before(time.Now()) {
			return ctx, errUnauthenticated
		}
		_ = q.TouchSession(ctx, s.ID)

		user := AuthUser{
			ID:        pgconv.UUIDString(s.UserID),
			Username:  s.Username,
			Role:      s.Role,
			SessionID: pgconv.UUIDString(s.ID),
		}
		// Coarse role gate: managers may only call the allowed procedures.
		if user.Role != repository.UserRoleAdmin && !managerAllowed[procedure] {
			return ctx, errForbidden
		}
		return context.WithValue(ctx, userKey, user), nil

	case strings.Contains(procedure, "ProducerService"):
		if token == "" {
			return ctx, errUnauthenticated
		}
		q := repository.New(i.pool)
		tok, err := q.GetApiTokenByHashWithOwner(ctx, HashToken(token))
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ctx, errUnauthenticated
			}
			return ctx, connect.NewError(connect.CodeInternal, err)
		}
		// A disabled owner's keys stop working (mirrors dropping their sessions).
		if tok.OwnerDisabled.Valid && tok.OwnerDisabled.Bool {
			return ctx, errUnauthenticated
		}
		_ = q.TouchApiTokenLastUsed(ctx, tok.ID)

		// Admin-owned and legacy (unowned) keys reach all devices; a
		// manager-owned key is scoped to its owner's devices.
		grantsAll := !tok.CreatedBy.Valid ||
			(tok.OwnerRole.Valid && tok.OwnerRole.UserRole == repository.UserRoleAdmin)
		principal := TokenPrincipal{
			OwnerID:   pgconv.UUIDString(tok.CreatedBy),
			GrantsAll: grantsAll,
		}
		return context.WithValue(ctx, tokenKey, principal), nil

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
