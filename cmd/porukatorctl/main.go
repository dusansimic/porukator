// porukatorctl is a small client for ProducerService, used to exercise a running
// Porukator service by hand: list devices and submit messages.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/dusansimic/porukator/gen/go/porukator/v1/porukatorv1connect"
)

// shared across subcommands, populated in the root PersistentPreRunE.
var (
	flagHost  string
	flagToken string
	flagJSON  bool

	client porukatorv1connect.ProducerServiceClient
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "porukatorctl",
		Short: "Test client for a Porukator service (ProducerService API)",
		Long: "porukatorctl exercises the Porukator ProducerService: list devices and\n" +
			"submit messages. Authenticates with an API token.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if flagHost == "" {
				flagHost = os.Getenv("PORUKATOR_HOST")
			}
			if flagToken == "" {
				flagToken = os.Getenv("PORUKATOR_TOKEN")
			}
			if flagHost == "" {
				return fmt.Errorf("--host is required (or set PORUKATOR_HOST)")
			}
			if flagToken == "" {
				return fmt.Errorf("--token is required (or set PORUKATOR_TOKEN)")
			}

			base := flagHost
			if !strings.Contains(base, "://") {
				base = "http://" + base
			}
			base = strings.TrimRight(base, "/")

			client = porukatorv1connect.NewProducerServiceClient(
				http.DefaultClient, base,
				connect.WithInterceptors(authInterceptor(flagToken)),
			)
			return nil
		},
	}

	root.PersistentFlags().StringVarP(&flagHost, "host", "H", "", "service base URL, e.g. http://localhost:8080 (env PORUKATOR_HOST)")
	root.PersistentFlags().StringVarP(&flagToken, "token", "t", "", "API token (env PORUKATOR_TOKEN)")
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "output raw JSON instead of a table")

	root.AddCommand(newListCmd(), newSendCmd())
	return root
}

// authInterceptor attaches the API token as a bearer header on every request.
func authInterceptor(token string) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Authorization", "Bearer "+token)
			return next(ctx, req)
		}
	}
}
