package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/dusansimic/porukator/internal/auth"
	"github.com/dusansimic/porukator/internal/config"
	"github.com/dusansimic/porukator/internal/db"
	"github.com/dusansimic/porukator/internal/pgconv"
	"github.com/dusansimic/porukator/internal/repository"
)

// runCreateUser implements `porukator create-user`: it creates a web-UI account
// directly against the database (used to bootstrap the first admin). It loads
// the same config as the server, applies migrations, then inserts the user.
func runCreateUser(args []string) error {
	fs := flag.NewFlagSet("create-user", flag.ExitOnError)
	username := fs.String("username", "", "login name (required)")
	password := fs.String("password", "", "password (prompted if omitted)")
	admin := fs.Bool("admin", false, "create an admin (default role is manager)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *username == "" {
		return errors.New("--username is required")
	}
	pw := *password
	if pw == "" {
		var err error
		if pw, err = promptPassword(); err != nil {
			return err
		}
	}
	if pw == "" {
		return errors.New("password must not be empty")
	}

	role := repository.UserRoleManager
	if *admin {
		role = repository.UserRoleAdmin
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx := context.Background()
	if err := db.Migrate(ctx, cfg.Postgres.URL, db.MigrationsFS, "migrations"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	pool, err := db.NewPool(ctx, cfg.Postgres.URL)
	if err != nil {
		return fmt.Errorf("connect db: %w", err)
	}
	defer pool.Close()

	hash, err := auth.HashPassword(pw)
	if err != nil {
		return err
	}
	u, err := repository.New(pool).CreateUser(ctx, repository.CreateUserParams{
		Username:     *username,
		PasswordHash: hash,
		Role:         role,
	})
	if err != nil {
		return fmt.Errorf("create user (is the username taken?): %w", err)
	}

	fmt.Printf("created %s user %q (%s)\n", u.Role, u.Username, pgconv.UUIDString(u.ID))
	return nil
}

// promptPassword reads a password from the terminal without echo, or from stdin
// if it is not a terminal (e.g. piped input).
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
