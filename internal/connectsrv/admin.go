package connectsrv

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
	"github.com/dusansimic/porukator/internal/auth"
	"github.com/dusansimic/porukator/internal/pgconv"
	"github.com/dusansimic/porukator/internal/repository"
)

// sessionTTL is how long a web-UI session stays valid after creation.
const sessionTTL = 30 * 24 * time.Hour

// --- Authentication ---

// Login authenticates a username + password and returns a session token. Exempt
// from the auth interceptor.
func (h *Handler) Login(ctx context.Context, req *connect.Request[porukatorv1.LoginRequest]) (*connect.Response[porukatorv1.LoginResponse], error) {
	invalid := connect.NewError(connect.CodeUnauthenticated, errors.New("invalid username or password"))

	u, err := h.q().GetUserByUsername(ctx, req.Msg.Username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Still run a hash to blunt username enumeration via timing.
			_ = auth.VerifyPassword("$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", req.Msg.Password)
			return nil, invalid
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if u.Disabled || !auth.VerifyPassword(u.PasswordHash, req.Msg.Password) {
		return nil, invalid
	}

	token, err := auth.GenerateToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if _, err := h.q().CreateSession(ctx, repository.CreateSessionParams{
		UserID:    u.ID,
		TokenHash: auth.HashToken(token),
		ExpiresAt: pgtype.Timestamptz{Time: time.Now().Add(sessionTTL), Valid: true},
	}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&porukatorv1.LoginResponse{
		Token: token,
		User:  userToProto(u),
	}), nil
}

// Logout revokes the caller's current session.
func (h *Handler) Logout(ctx context.Context, req *connect.Request[porukatorv1.LogoutRequest]) (*connect.Response[porukatorv1.LogoutResponse], error) {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no session"))
	}
	id, err := pgconv.ParseUUID(user.SessionID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := h.q().DeleteSession(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.LogoutResponse{}), nil
}

// GetCurrentUser returns the authenticated user (whoami).
func (h *Handler) GetCurrentUser(ctx context.Context, req *connect.Request[porukatorv1.GetCurrentUserRequest]) (*connect.Response[porukatorv1.GetCurrentUserResponse], error) {
	user, ok := auth.UserFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no session"))
	}
	id, err := pgconv.ParseUUID(user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	u, err := h.q().GetUser(ctx, id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.GetCurrentUserResponse{User: userToProto(u)}), nil
}

// --- Client devices (ownership-scoped) ---

// CreateClient registers a device owned by the caller.
func (h *Handler) CreateClient(ctx context.Context, req *connect.Request[porukatorv1.CreateClientRequest]) (*connect.Response[porukatorv1.CreateClientResponse], error) {
	user, _ := auth.UserFromContext(ctx)
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	ownerID, err := pgconv.ParseUUID(user.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	token, err := auth.GenerateToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	c, err := h.q().CreateClient(ctx, repository.CreateClientParams{
		Name:            req.Msg.Name,
		AccessTokenHash: auth.HashToken(token),
		CreatedBy:       ownerID,
	})
	if err != nil {
		h.logger.Error("create client failed", zapErr(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	proto := h.clientToProto(c)
	proto.OwnerUsername = user.Username
	return connect.NewResponse(&porukatorv1.CreateClientResponse{
		Client:      proto,
		AccessToken: token,
		Host:        h.cfg.HTTP.PublicHost,
	}), nil
}

// ListClients returns all devices for admins, own devices for managers.
func (h *Handler) ListClients(ctx context.Context, req *connect.Request[porukatorv1.ListClientsRequest]) (*connect.Response[porukatorv1.ListClientsResponse], error) {
	user, _ := auth.UserFromContext(ctx)
	var out []*porukatorv1.Client

	if user.IsAdmin() {
		rows, err := h.q().ListClientsWithOwner(ctx)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out = make([]*porukatorv1.Client, len(rows))
		for i, r := range rows {
			out[i] = h.clientWithOwnerToProto(r.ID, r.Name, r.LastSeenAt, r.CreatedAt, r.CreatedBy, r.OwnerUsername)
		}
	} else {
		ownerID, err := pgconv.ParseUUID(user.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rows, err := h.q().ListClientsForOwner(ctx, ownerID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		out = make([]*porukatorv1.Client, len(rows))
		for i, r := range rows {
			out[i] = h.clientWithOwnerToProto(r.ID, r.Name, r.LastSeenAt, r.CreatedAt, r.CreatedBy, r.OwnerUsername)
		}
	}
	return connect.NewResponse(&porukatorv1.ListClientsResponse{Clients: out}), nil
}

// ownedClientOr403 loads a device and verifies the caller may modify it: admins
// may touch any, managers only their own.
func (h *Handler) ownedClientOr403(ctx context.Context, user auth.AuthUser, idStr string) (repository.Client, error) {
	id, err := pgconv.ParseUUID(idStr)
	if err != nil {
		return repository.Client{}, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	c, err := h.q().GetClient(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.Client{}, connect.NewError(connect.CodeNotFound, errors.New("device not found"))
		}
		return repository.Client{}, connect.NewError(connect.CodeInternal, err)
	}
	if !user.IsAdmin() && pgconv.UUIDString(c.CreatedBy) != user.ID {
		return repository.Client{}, connect.NewError(connect.CodePermissionDenied, errors.New("not your device"))
	}
	return c, nil
}

// RenameClient changes a device's display name (managers: own devices only).
func (h *Handler) RenameClient(ctx context.Context, req *connect.Request[porukatorv1.RenameClientRequest]) (*connect.Response[porukatorv1.RenameClientResponse], error) {
	user, _ := auth.UserFromContext(ctx)
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	c, err := h.ownedClientOr403(ctx, user, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	updated, err := h.q().RenameClient(ctx, repository.RenameClientParams{ID: c.ID, Name: req.Msg.Name})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.RenameClientResponse{Client: h.clientToProto(updated)}), nil
}

// RevokeClient deletes a device (managers: own devices only).
func (h *Handler) RevokeClient(ctx context.Context, req *connect.Request[porukatorv1.RevokeClientRequest]) (*connect.Response[porukatorv1.RevokeClientResponse], error) {
	user, _ := auth.UserFromContext(ctx)
	c, err := h.ownedClientOr403(ctx, user, req.Msg.Id)
	if err != nil {
		return nil, err
	}
	if err := h.q().DeleteClient(ctx, c.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.RevokeClientResponse{}), nil
}

// --- API tokens (admin only, gated by the interceptor) ---

// CreateApiToken issues a producer token, returned once.
func (h *Handler) CreateApiToken(ctx context.Context, req *connect.Request[porukatorv1.CreateApiTokenRequest]) (*connect.Response[porukatorv1.CreateApiTokenResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	secret, err := auth.GenerateToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	t, err := h.q().CreateApiToken(ctx, repository.CreateApiTokenParams{
		Name:      req.Msg.Name,
		TokenHash: auth.HashToken(secret),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.CreateApiTokenResponse{
		Token:  apiTokenToProto(t),
		Secret: secret,
	}), nil
}

// ListApiTokens lists producer tokens without secrets.
func (h *Handler) ListApiTokens(ctx context.Context, req *connect.Request[porukatorv1.ListApiTokensRequest]) (*connect.Response[porukatorv1.ListApiTokensResponse], error) {
	rows, err := h.q().ListApiTokens(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*porukatorv1.ApiToken, len(rows))
	for i, r := range rows {
		out[i] = apiTokenToProto(r)
	}
	return connect.NewResponse(&porukatorv1.ListApiTokensResponse{Tokens: out}), nil
}

// RevokeApiToken deletes a producer token.
func (h *Handler) RevokeApiToken(ctx context.Context, req *connect.Request[porukatorv1.RevokeApiTokenRequest]) (*connect.Response[porukatorv1.RevokeApiTokenResponse], error) {
	id, err := pgconv.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	if err := h.q().DeleteApiToken(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.RevokeApiTokenResponse{}), nil
}

// --- Settings (admin only) ---

// GetSettings returns the pacing configuration.
func (h *Handler) GetSettings(ctx context.Context, req *connect.Request[porukatorv1.GetSettingsRequest]) (*connect.Response[porukatorv1.GetSettingsResponse], error) {
	s, err := h.q().GetSettings(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.GetSettingsResponse{
		Settings: &porukatorv1.Settings{DelayMs: s.DelayMs, JitterMs: s.JitterMs},
	}), nil
}

// UpdateSettings replaces the pacing configuration.
func (h *Handler) UpdateSettings(ctx context.Context, req *connect.Request[porukatorv1.UpdateSettingsRequest]) (*connect.Response[porukatorv1.UpdateSettingsResponse], error) {
	in := req.Msg.Settings
	if in == nil || in.DelayMs < 0 || in.JitterMs < 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("delay_ms and jitter_ms must be non-negative"))
	}
	s, err := h.q().UpdateSettings(ctx, repository.UpdateSettingsParams{DelayMs: in.DelayMs, JitterMs: in.JitterMs})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.UpdateSettingsResponse{
		Settings: &porukatorv1.Settings{DelayMs: s.DelayMs, JitterMs: s.JitterMs},
	}), nil
}

// --- Messages (ownership-scoped) ---

// ListMessages returns all messages for admins, own-device messages for managers.
func (h *Handler) ListMessages(ctx context.Context, req *connect.Request[porukatorv1.ListMessagesRequest]) (*connect.Response[porukatorv1.ListMessagesResponse], error) {
	user, _ := auth.UserFromContext(ctx)
	limit := req.Msg.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	clientID := newNullUUID()
	if req.Msg.ClientId != "" {
		id, err := pgconv.ParseUUID(req.Msg.ClientId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid client_id"))
		}
		clientID = id
	}

	var rows []repository.Message
	if user.IsAdmin() {
		var err error
		rows, err = h.q().ListMessages(ctx, repository.ListMessagesParams{
			Status:   statusFromProto(req.Msg.Status),
			ClientID: clientID,
			Lim:      limit,
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		ownerID, err := pgconv.ParseUUID(user.ID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rows, err = h.q().ListMessagesForOwner(ctx, repository.ListMessagesForOwnerParams{
			Owner:    ownerID,
			Status:   statusFromProto(req.Msg.Status),
			ClientID: clientID,
			Lim:      limit,
		})
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	out := make([]*porukatorv1.Message, len(rows))
	for i, r := range rows {
		out[i] = messageToProto(r)
	}
	return connect.NewResponse(&porukatorv1.ListMessagesResponse{Messages: out}), nil
}

// --- User management (admin only) ---

// CreateUser creates a web-UI account.
func (h *Handler) CreateUser(ctx context.Context, req *connect.Request[porukatorv1.CreateUserRequest]) (*connect.Response[porukatorv1.CreateUserResponse], error) {
	if req.Msg.Username == "" || req.Msg.Password == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("username and password are required"))
	}
	role, ok := roleFromProto(req.Msg.Role)
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role must be admin or manager"))
	}
	hash, err := auth.HashPassword(req.Msg.Password)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	u, err := h.q().CreateUser(ctx, repository.CreateUserParams{
		Username:     req.Msg.Username,
		PasswordHash: hash,
		Role:         role,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("username already taken"))
	}
	return connect.NewResponse(&porukatorv1.CreateUserResponse{User: userToProto(u)}), nil
}

// ListUsers returns all accounts.
func (h *Handler) ListUsers(ctx context.Context, req *connect.Request[porukatorv1.ListUsersRequest]) (*connect.Response[porukatorv1.ListUsersResponse], error) {
	rows, err := h.q().ListUsers(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*porukatorv1.User, len(rows))
	for i, r := range rows {
		out[i] = userToProto(r)
	}
	return connect.NewResponse(&porukatorv1.ListUsersResponse{Users: out}), nil
}

// SetUserRole changes an account's role.
func (h *Handler) SetUserRole(ctx context.Context, req *connect.Request[porukatorv1.SetUserRoleRequest]) (*connect.Response[porukatorv1.SetUserRoleResponse], error) {
	id, err := pgconv.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	role, ok := roleFromProto(req.Msg.Role)
	if !ok {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role must be admin or manager"))
	}
	u, err := h.q().SetUserRole(ctx, repository.SetUserRoleParams{ID: id, Role: role})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.SetUserRoleResponse{User: userToProto(u)}), nil
}

// SetUserDisabled enables/disables an account; disabling revokes its sessions.
func (h *Handler) SetUserDisabled(ctx context.Context, req *connect.Request[porukatorv1.SetUserDisabledRequest]) (*connect.Response[porukatorv1.SetUserDisabledResponse], error) {
	id, err := pgconv.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	u, err := h.q().SetUserDisabled(ctx, repository.SetUserDisabledParams{ID: id, Disabled: req.Msg.Disabled})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if req.Msg.Disabled {
		if err := h.q().DeleteSessionsByUser(ctx, id); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&porukatorv1.SetUserDisabledResponse{User: userToProto(u)}), nil
}

// DeleteUser removes an account (sessions cascade; owned devices become unowned).
func (h *Handler) DeleteUser(ctx context.Context, req *connect.Request[porukatorv1.DeleteUserRequest]) (*connect.Response[porukatorv1.DeleteUserResponse], error) {
	id, err := pgconv.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	if err := h.q().DeleteUser(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.DeleteUserResponse{}), nil
}

// --- Session management (admin only) ---

// ListSessions returns active sessions across all users.
func (h *Handler) ListSessions(ctx context.Context, req *connect.Request[porukatorv1.ListSessionsRequest]) (*connect.Response[porukatorv1.ListSessionsResponse], error) {
	user, _ := auth.UserFromContext(ctx)
	rows, err := h.q().ListSessions(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*porukatorv1.Session, len(rows))
	for i, r := range rows {
		out[i] = &porukatorv1.Session{
			Id:         pgconv.UUIDString(r.ID),
			UserId:     pgconv.UUIDString(r.UserID),
			Username:   r.Username,
			CreatedAt:  pgconv.PbTime(r.CreatedAt),
			LastUsedAt: pgconv.PbTime(r.LastUsedAt),
			Current:    pgconv.UUIDString(r.ID) == user.SessionID,
		}
	}
	return connect.NewResponse(&porukatorv1.ListSessionsResponse{Sessions: out}), nil
}

// RevokeSession deletes one session by id.
func (h *Handler) RevokeSession(ctx context.Context, req *connect.Request[porukatorv1.RevokeSessionRequest]) (*connect.Response[porukatorv1.RevokeSessionResponse], error) {
	id, err := pgconv.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	if err := h.q().DeleteSession(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.RevokeSessionResponse{}), nil
}
