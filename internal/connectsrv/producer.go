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

// GetMessages returns the status of specific messages by id, scoped to the
// calling key's owner. Strict: if any requested id is not visible to the key
// (another owner's, or unknown), the whole request fails with PermissionDenied.
func (h *Handler) GetMessages(ctx context.Context, req *connect.Request[porukatorv1.GetMessagesRequest]) (*connect.Response[porukatorv1.GetMessagesResponse], error) {
	tok, _ := auth.TokenFromContext(ctx)
	if len(req.Msg.MessageIds) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("message_ids must not be empty"))
	}

	// Parse + dedupe the requested ids.
	seen := make(map[string]bool, len(req.Msg.MessageIds))
	ids := make([]pgtype.UUID, 0, len(req.Msg.MessageIds))
	for _, s := range req.Msg.MessageIds {
		id, err := pgconv.ParseUUID(s)
		if err != nil || !id.Valid {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid message_id: "+s))
		}
		if !seen[s] {
			seen[s] = true
			ids = append(ids, id)
		}
	}

	var rows []repository.Message
	var err error
	if tok.GrantsAll {
		rows, err = h.q().GetMessagesByIDs(ctx, ids)
	} else {
		ownerID, perr := pgconv.ParseUUID(tok.OwnerID)
		if perr != nil {
			return nil, connect.NewError(connect.CodeInternal, perr)
		}
		rows, err = h.q().GetMessagesByIDsForOwner(ctx, repository.GetMessagesByIDsForOwnerParams{
			Ids:   ids,
			Owner: ownerID,
		})
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Strict: every requested id must be visible, else reject the whole request
	// (uniform for forbidden and unknown ids — does not leak existence).
	if len(rows) != len(ids) {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("one or more message_ids are not visible to this key"))
	}

	out := make([]*porukatorv1.Message, len(rows))
	for i, r := range rows {
		out[i] = messageToProto(r)
	}
	return connect.NewResponse(&porukatorv1.GetMessagesResponse{Messages: out}), nil
}
