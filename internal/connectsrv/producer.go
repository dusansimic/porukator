package connectsrv

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
	"github.com/dusansimic/porukator/internal/auth"
	"github.com/dusansimic/porukator/internal/pgconv"
	"github.com/dusansimic/porukator/internal/repository"
)

// SendMessages accepts a batch and distributes it round-robin across the given
// client devices. Each message is persisted as PENDING; if its assigned client
// is currently online, the job is pushed to that client's stream. The stream
// path (not this one) marks messages DISPATCHED, which keeps delivery
// exactly-once across the connect/reconnect race.
func (h *Handler) SendMessages(ctx context.Context, req *connect.Request[porukatorv1.SendMessagesRequest]) (*connect.Response[porukatorv1.SendMessagesResponse], error) {
	msgs := req.Msg.Messages
	clientIDs := req.Msg.ClientIds
	if len(msgs) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("messages must not be empty"))
	}
	if len(clientIDs) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("client_ids must not be empty"))
	}

	// A manager-owned key may only send through its owner's devices; admin-owned
	// and legacy keys may use any device.
	tok, _ := auth.TokenFromContext(ctx)

	// Parse and validate every target client up front.
	pgIDs := make([]pgtype.UUID, len(clientIDs))
	for i, cid := range clientIDs {
		id, err := pgconv.ParseUUID(cid)
		if err != nil || !id.Valid {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid client_id: "+cid))
		}
		c, err := h.q().GetClient(ctx, id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("unknown client_id: "+cid))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if !tok.GrantsAll && pgconv.UUIDString(c.CreatedBy) != tok.OwnerID {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("not your device: "+cid))
		}
		pgIDs[i] = id
	}

	batchID := uuid.New().String()
	batchUUID, err := pgconv.ParseUUID(batchID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	settings, err := h.q().GetSettings(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	ids := make([]string, len(msgs))
	for i, m := range msgs {
		if m.PhoneNumber == "" || m.Content == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("phone_number and content are required"))
		}
		target := pgIDs[i%len(pgIDs)] // round-robin balancing
		row, err := h.q().InsertMessage(ctx, repository.InsertMessageParams{
			BatchID:     batchUUID,
			PhoneNumber: m.PhoneNumber,
			Content:     m.Content,
			ClientID:    target,
		})
		if err != nil {
			h.logger.Error("insert message failed", zapErr(err))
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		mid := pgconv.UUIDString(row.ID)
		ids[i] = mid

		// Best-effort push to an online client; the stream path owns the
		// DISPATCHED transition. If offline or buffer full, it stays PENDING
		// and is drained on next connect.
		h.registry.Push(clientIDs[i%len(clientIDs)], &porukatorv1.Job{
			MessageId:   mid,
			PhoneNumber: m.PhoneNumber,
			Content:     m.Content,
			DelayMs:     settings.DelayMs,
			JitterMs:    settings.JitterMs,
		})
	}

	return connect.NewResponse(&porukatorv1.SendMessagesResponse{
		BatchId:    batchID,
		MessageIds: ids,
	}), nil
}
