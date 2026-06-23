package connectsrv

import (
	"context"
	"crypto/subtle"
	"errors"

	"connectrpc.com/connect"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
	"github.com/dusansimic/porukator/internal/auth"
	"github.com/dusansimic/porukator/internal/pgconv"
	"github.com/dusansimic/porukator/internal/repository"
)

// Login validates the master password. It is exempt from the auth interceptor.
func (h *Handler) Login(ctx context.Context, req *connect.Request[porukatorv1.LoginRequest]) (*connect.Response[porukatorv1.LoginResponse], error) {
	ok := h.cfg.Auth.MasterPassword != "" &&
		subtle.ConstantTimeCompare([]byte(req.Msg.Password), []byte(h.cfg.Auth.MasterPassword)) == 1
	return connect.NewResponse(&porukatorv1.LoginResponse{Ok: ok}), nil
}

// CreateClient registers a device, returning its one-time access token.
func (h *Handler) CreateClient(ctx context.Context, req *connect.Request[porukatorv1.CreateClientRequest]) (*connect.Response[porukatorv1.CreateClientResponse], error) {
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	token, err := auth.GenerateToken()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	c, err := h.q().CreateClient(ctx, repository.CreateClientParams{
		Name:            req.Msg.Name,
		AccessTokenHash: auth.HashToken(token),
	})
	if err != nil {
		h.logger.Error("create client failed", zapErr(err))
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.CreateClientResponse{
		Client:      h.clientToProto(c),
		AccessToken: token,
		Host:        h.cfg.HTTP.PublicHost,
	}), nil
}

// ListClients returns all devices (shared with ProducerService).
func (h *Handler) ListClients(ctx context.Context, req *connect.Request[porukatorv1.ListClientsRequest]) (*connect.Response[porukatorv1.ListClientsResponse], error) {
	rows, err := h.q().ListClients(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*porukatorv1.Client, len(rows))
	for i, r := range rows {
		out[i] = h.clientToProto(r)
	}
	return connect.NewResponse(&porukatorv1.ListClientsResponse{Clients: out}), nil
}

// RenameClient changes a device's display name.
func (h *Handler) RenameClient(ctx context.Context, req *connect.Request[porukatorv1.RenameClientRequest]) (*connect.Response[porukatorv1.RenameClientResponse], error) {
	id, err := pgconv.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	c, err := h.q().RenameClient(ctx, repository.RenameClientParams{ID: id, Name: req.Msg.Name})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.RenameClientResponse{Client: h.clientToProto(c)}), nil
}

// RevokeClient deletes a device.
func (h *Handler) RevokeClient(ctx context.Context, req *connect.Request[porukatorv1.RevokeClientRequest]) (*connect.Response[porukatorv1.RevokeClientResponse], error) {
	id, err := pgconv.ParseUUID(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}
	if err := h.q().DeleteClient(ctx, id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&porukatorv1.RevokeClientResponse{}), nil
}

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

// ListMessages returns recent messages with optional filters.
func (h *Handler) ListMessages(ctx context.Context, req *connect.Request[porukatorv1.ListMessagesRequest]) (*connect.Response[porukatorv1.ListMessagesResponse], error) {
	limit := req.Msg.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}
	if limit > maxListLimit {
		limit = maxListLimit
	}
	var clientID = newNullUUID()
	if req.Msg.ClientId != "" {
		id, err := pgconv.ParseUUID(req.Msg.ClientId)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid client_id"))
		}
		clientID = id
	}
	rows, err := h.q().ListMessages(ctx, repository.ListMessagesParams{
		Status:   statusFromProto(req.Msg.Status),
		ClientID: clientID,
		Lim:      limit,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*porukatorv1.Message, len(rows))
	for i, r := range rows {
		out[i] = messageToProto(r)
	}
	return connect.NewResponse(&porukatorv1.ListMessagesResponse{Messages: out}), nil
}
