package connectsrv

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
	"github.com/dusansimic/porukator/internal/auth"
	"github.com/dusansimic/porukator/internal/pgconv"
	"github.com/dusansimic/porukator/internal/repository"
)

// heartbeatInterval is how often the server sends a keepalive frame on an idle
// job stream. Kept well under the ~10s idle timeouts common to reverse proxies
// so a silent stream is never cut.
const heartbeatInterval = 5 * time.Second

// StreamJobs marks the calling device online and streams SMS jobs to it. On
// connect it drains any jobs that piled up while the device was offline, then
// forwards live pushes until the stream closes. A MarkDispatched guard makes
// each message stream exactly once even if a producer pushed it concurrently
// with the connect.
func (h *Handler) StreamJobs(ctx context.Context, req *connect.Request[porukatorv1.StreamJobsRequest], stream *connect.ServerStream[porukatorv1.Job]) error {
	clientID, ok := auth.ClientIDFromContext(ctx)
	if !ok {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("no client identity"))
	}
	pgID, err := pgconv.ParseUUID(clientID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	jobs, release := h.registry.Register(clientID)
	defer release()

	h.logger.Info("client connected", zapStr("client_id", clientID))
	defer func() {
		// Record last-seen on disconnect; use a fresh context since ctx is done.
		_ = h.q().TouchClientLastSeen(context.Background(), pgID)
		h.logger.Info("client disconnected", zapStr("client_id", clientID))
	}()

	settings, err := h.q().GetSettings(ctx)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}

	// Drain jobs left pending while offline.
	pending, err := h.q().ListPendingForClient(ctx, pgID)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	for _, m := range pending {
		if err := h.sendJob(ctx, stream, m.ID, m.PhoneNumber, m.Content, settings.DelayMs, settings.JitterMs); err != nil {
			return err
		}
	}

	// Forward live jobs. The heartbeat ticker shares this goroutine so all
	// stream.Send calls stay serialized (connect ServerStream.Send is not safe
	// for concurrent use).
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := stream.Send(&porukatorv1.Job{Keepalive: true}); err != nil {
				return err
			}
		case job, ok := <-jobs:
			if !ok {
				// Superseded by a newer connection for the same client.
				return nil
			}
			mid, err := pgconv.ParseUUID(job.MessageId)
			if err != nil {
				continue
			}
			if err := h.sendJob(ctx, stream, mid, job.PhoneNumber, job.Content, job.DelayMs, job.JitterMs); err != nil {
				return err
			}
		}
	}
}

// sendJob marks a message DISPATCHED and, only if that transition actually
// happened (it was still PENDING), sends it on the stream. This dedupes
// messages that appear both in the pending drain and the live channel.
func (h *Handler) sendJob(ctx context.Context, stream *connect.ServerStream[porukatorv1.Job], id pgtype.UUID, phone, content string, delayMs, jitterMs int32) error {
	n, err := h.q().MarkDispatched(ctx, id)
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if n == 0 {
		// Already dispatched elsewhere; skip to avoid a duplicate SMS.
		return nil
	}
	return stream.Send(&porukatorv1.Job{
		MessageId:   pgconv.UUIDString(id),
		PhoneNumber: phone,
		Content:     content,
		DelayMs:     delayMs,
		JitterMs:    jitterMs,
	})
}

// ReportDelivery records the outcome of a single send.
func (h *Handler) ReportDelivery(ctx context.Context, req *connect.Request[porukatorv1.ReportDeliveryRequest]) (*connect.Response[porukatorv1.ReportDeliveryResponse], error) {
	clientID, ok := auth.ClientIDFromContext(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no client identity"))
	}
	mid, err := pgconv.ParseUUID(req.Msg.MessageId)
	if err != nil || !mid.Valid {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid message_id"))
	}

	if req.Msg.Success {
		if _, err := h.q().MarkSent(ctx, repository.MarkSentParams{
			ID:     mid,
			SentAt: pgconv.Timestamptz(req.Msg.SentAt),
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	} else {
		if _, err := h.q().MarkFailed(ctx, repository.MarkFailedParams{
			ID:    mid,
			Error: req.Msg.Error,
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	if pgID, err := pgconv.ParseUUID(clientID); err == nil {
		_ = h.q().TouchClientLastSeen(ctx, pgID)
	}
	return connect.NewResponse(&porukatorv1.ReportDeliveryResponse{}), nil
}
