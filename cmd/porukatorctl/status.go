package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
)

var statusByName = map[string]porukatorv1.MessageStatus{
	"pending":    porukatorv1.MessageStatus_MESSAGE_STATUS_PENDING,
	"dispatched": porukatorv1.MessageStatus_MESSAGE_STATUS_DISPATCHED,
	"sent":       porukatorv1.MessageStatus_MESSAGE_STATUS_SENT,
	"failed":     porukatorv1.MessageStatus_MESSAGE_STATUS_FAILED,
}

func statusLabel(s porukatorv1.MessageStatus) string {
	switch s {
	case porukatorv1.MessageStatus_MESSAGE_STATUS_PENDING:
		return "pending"
	case porukatorv1.MessageStatus_MESSAGE_STATUS_DISPATCHED:
		return "dispatched"
	case porukatorv1.MessageStatus_MESSAGE_STATUS_SENT:
		return "sent"
	case porukatorv1.MessageStatus_MESSAGE_STATUS_FAILED:
		return "failed"
	default:
		return "unspecified"
	}
}

func newStatusCmd() *cobra.Command {
	var (
		ids    []string
		batch  string
		status string
		limit  int32
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check message status: by id (--id) or by batch/filter (--batch/--status)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var msgs []*porukatorv1.Message

			if len(ids) > 0 {
				resp, err := client.GetMessages(cmd.Context(), connect.NewRequest(&porukatorv1.GetMessagesRequest{
					MessageIds: ids,
				}))
				if err != nil {
					return err
				}
				msgs = resp.Msg.Messages
			} else {
				req := &porukatorv1.ListMessagesRequest{BatchId: batch, Limit: limit}
				if status != "" {
					s, ok := statusByName[strings.ToLower(status)]
					if !ok {
						return fmt.Errorf("invalid --status %q (pending|dispatched|sent|failed)", status)
					}
					req.Status = s
				}
				resp, err := client.ListMessages(cmd.Context(), connect.NewRequest(req))
				if err != nil {
					return err
				}
				msgs = resp.Msg.Messages
			}

			if flagJSON {
				out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.
					Marshal(&porukatorv1.GetMessagesResponse{Messages: msgs})
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tPHONE\tSTATUS\tSENT AT\tERROR")
			for _, m := range msgs {
				sentAt := "—"
				if m.SentAt != nil {
					sentAt = m.SentAt.AsTime().Local().Format("2006-01-02 15:04:05")
				}
				errMsg := m.Error
				if errMsg == "" {
					errMsg = "—"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", m.Id, m.PhoneNumber, statusLabel(m.Status), sentAt, errMsg)
			}
			return tw.Flush()
		},
	}

	cmd.Flags().StringSliceVar(&ids, "id", nil, "message id to look up (repeatable; uses GetMessages)")
	cmd.Flags().StringVar(&batch, "batch", "", "batch id to list (uses ListMessages)")
	cmd.Flags().StringVar(&status, "status", "", "status filter for --batch/list: pending|dispatched|sent|failed")
	cmd.Flags().Int32Var(&limit, "limit", 0, "max rows for the list query (server clamps)")

	return cmd
}
