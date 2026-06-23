package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"

	porukatorv1 "github.com/dusansimic/porukator/gen/go/porukator/v1"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List devices known to the service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := client.ListClients(cmd.Context(), connect.NewRequest(&porukatorv1.ListClientsRequest{}))
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

			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tONLINE\tLAST SEEN")
			for _, c := range resp.Msg.Clients {
				online := "no"
				if c.Online {
					online = "yes"
				}
				lastSeen := "—"
				if c.LastSeenAt != nil {
					lastSeen = c.LastSeenAt.AsTime().Local().Format("2006-01-02 15:04:05")
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", c.Id, c.Name, online, lastSeen)
			}
			return tw.Flush()
		},
	}
}
