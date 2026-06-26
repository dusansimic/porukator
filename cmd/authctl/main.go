// authctl manages Porukator web-UI user accounts directly against the database.
// It ships in the server image for in-container administration — notably
// bootstrapping the first admin, which cannot go through AdminService (that
// requires an existing admin to authenticate).
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/dusansimic/porukator/internal/config"
	"github.com/dusansimic/porukator/internal/db"
	"github.com/dusansimic/porukator/internal/repository"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "authctl",
		Short: "Manage Porukator user accounts (direct DB access)",
		Long: "authctl manages web-UI user accounts directly against the database.\n" +
			"Reads the same config/env as the server (PORUKATOR_POSTGRES_URL).",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newCreateCmd(),
		newListCmd(),
		newSetRoleCmd(),
		newDisableCmd(),
		newEnableCmd(),
		newDeleteCmd(),
		newPasswdCmd(),
	)
	return root
}

// openStore loads config, applies migrations (idempotent), and opens a pool.
// The caller must close the returned pool.
func openStore(ctx context.Context) (*pgxpool.Pool, *repository.Queries, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	if err := db.Migrate(ctx, cfg.Postgres.URL, db.MigrationsFS, "migrations"); err != nil {
		return nil, nil, fmt.Errorf("migrate: %w", err)
	}
	pool, err := db.NewPool(ctx, cfg.Postgres.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("connect db: %w", err)
	}
	return pool, repository.New(pool), nil
}

// promptPassword reads a password without echo from a terminal, or a line from
// stdin when piped.
func promptPassword() (string, error) {
	if term.IsTerminal(int(syscall.Stdin)) {
		fmt.Print("Password: ")
		b, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		return string(b), err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}
