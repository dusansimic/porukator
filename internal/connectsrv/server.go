// Package connectsrv hosts the three Porukator Connect services on one HTTP
// server. A single Handler implements all of AdminService, ProducerService and
// ClientService, sharing the pool, registry and config.
package connectsrv

import (
	"net/http"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
	"github.com/dusansimic/porukator/gen/go/porukator/v1/porukatorv1connect"
	"github.com/dusansimic/porukator/internal/config"
	"github.com/dusansimic/porukator/internal/pgconv"
	"github.com/dusansimic/porukator/internal/registry"
	"github.com/dusansimic/porukator/internal/repository"
)

const (
	defaultListLimit = 100
	maxListLimit     = 1000
)

// Handler implements all three Porukator services.
type Handler struct {
	logger   *zap.Logger
	pool     *pgxpool.Pool
	registry *registry.Registry
	cfg      *config.Config
}

func NewHandler(logger *zap.Logger, pool *pgxpool.Pool, reg *registry.Registry, cfg *config.Config) *Handler {
	return &Handler{logger: logger, pool: pool, registry: reg, cfg: cfg}
}

func (h *Handler) q() *repository.Queries { return repository.New(h.pool) }

// zapErr is a shorthand for zap.Error used across handlers.
func zapErr(err error) zap.Field { return zap.Error(err) }

// zapStr is a shorthand for zap.String used across handlers.
func zapStr(k, v string) zap.Field { return zap.String(k, v) }

// newNullUUID returns a null pgtype.UUID (used as "no filter").
func newNullUUID() pgtype.UUID { return pgtype.UUID{} }

// clientToProto renders a stored client, overlaying live online state.
func (h *Handler) clientToProto(c repository.Client) *porukatorv1.Client {
	id := pgconv.UUIDString(c.ID)
	return &porukatorv1.Client{
		Id:         id,
		Name:       c.Name,
		Online:     h.registry.IsOnline(id),
		LastSeenAt: pgconv.PbTime(c.LastSeenAt),
		CreatedAt:  pgconv.PbTime(c.CreatedAt),
	}
}

func apiTokenToProto(t repository.ApiToken) *porukatorv1.ApiToken {
	return &porukatorv1.ApiToken{
		Id:         pgconv.UUIDString(t.ID),
		Name:       t.Name,
		CreatedAt:  pgconv.PbTime(t.CreatedAt),
		LastUsedAt: pgconv.PbTime(t.LastUsedAt),
	}
}

func messageToProto(m repository.Message) *porukatorv1.Message {
	return &porukatorv1.Message{
		Id:           pgconv.UUIDString(m.ID),
		PhoneNumber:  m.PhoneNumber,
		Content:      m.Content,
		ClientId:     pgconv.UUIDString(m.ClientID),
		Status:       statusToProto(m.Status),
		Error:        m.Error,
		ReceivedAt:   pgconv.PbTime(m.ReceivedAt),
		DispatchedAt: pgconv.PbTime(m.DispatchedAt),
		SentAt:       pgconv.PbTime(m.SentAt),
		BatchId:      pgconv.UUIDString(m.BatchID),
	}
}

func statusToProto(s repository.MessageStatus) porukatorv1.MessageStatus {
	switch s {
	case repository.MessageStatusPending:
		return porukatorv1.MessageStatus_MESSAGE_STATUS_PENDING
	case repository.MessageStatusDispatched:
		return porukatorv1.MessageStatus_MESSAGE_STATUS_DISPATCHED
	case repository.MessageStatusSent:
		return porukatorv1.MessageStatus_MESSAGE_STATUS_SENT
	case repository.MessageStatusFailed:
		return porukatorv1.MessageStatus_MESSAGE_STATUS_FAILED
	default:
		return porukatorv1.MessageStatus_MESSAGE_STATUS_UNSPECIFIED
	}
}

func statusFromProto(s porukatorv1.MessageStatus) repository.NullMessageStatus {
	switch s {
	case porukatorv1.MessageStatus_MESSAGE_STATUS_PENDING:
		return repository.NullMessageStatus{MessageStatus: repository.MessageStatusPending, Valid: true}
	case porukatorv1.MessageStatus_MESSAGE_STATUS_DISPATCHED:
		return repository.NullMessageStatus{MessageStatus: repository.MessageStatusDispatched, Valid: true}
	case porukatorv1.MessageStatus_MESSAGE_STATUS_SENT:
		return repository.NullMessageStatus{MessageStatus: repository.MessageStatusSent, Valid: true}
	case porukatorv1.MessageStatus_MESSAGE_STATUS_FAILED:
		return repository.NullMessageStatus{MessageStatus: repository.MessageStatusFailed, Valid: true}
	default:
		return repository.NullMessageStatus{}
	}
}

// NewHTTPServer mounts all three services behind the auth interceptor and
// serves Connect/gRPC/gRPC-web over h2c (needed for the StreamJobs server
// stream). Front with TLS at the edge.
func NewHTTPServer(addr string, handler *Handler, interceptor connect.Interceptor) *http.Server {
	opts := connect.WithInterceptors(interceptor)

	mux := http.NewServeMux()
	adminPath, adminH := porukatorv1connect.NewAdminServiceHandler(handler, opts)
	mux.Handle(adminPath, adminH)
	prodPath, prodH := porukatorv1connect.NewProducerServiceHandler(handler, opts)
	mux.Handle(prodPath, prodH)
	clientPath, clientH := porukatorv1connect.NewClientServiceHandler(handler, opts)
	mux.Handle(clientPath, clientH)

	return &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}
}
