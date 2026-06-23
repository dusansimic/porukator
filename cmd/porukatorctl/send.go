package main

import (
	"fmt"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
)

func newSendCmd() *cobra.Command {
	var (
		to      []string
		message string
		clients []string
	)

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Submit messages, balanced across the given devices",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(clients) == 0 {
				return fmt.Errorf("at least one --client is required")
			}

			msgs := make([]*porukatorv1.OutgoingMessage, len(to))
			for i, phone := range to {
				msgs[i] = &porukatorv1.OutgoingMessage{PhoneNumber: phone, Content: message}
			}

			resp, err := client.SendMessages(cmd.Context(), connect.NewRequest(&porukatorv1.SendMessagesRequest{
				Messages:  msgs,
				ClientIds: clients,
			}))
			if err != nil {
				return err
			}

			if flagJSON {
				out, err := protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(resp.Msg)
				if err != nil {
					return err
				}
				fmt.Println(string(out))
				return nil
			}

			fmt.Println("batch:", resp.Msg.BatchId)
			for i, id := range resp.Msg.MessageIds {
				fmt.Printf("  %s -> %s\n", to[i], id)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&to, "to", nil, "destination phone number (repeatable)")
	cmd.Flags().StringVarP(&message, "message", "m", "", "SMS body, sent to every --to")
	cmd.Flags().StringSliceVarP(&clients, "client", "c", nil, "device ID to balance across (repeatable, required)")
	cmd.MarkFlagRequired("to")
	cmd.MarkFlagRequired("message")

	return cmd
}
