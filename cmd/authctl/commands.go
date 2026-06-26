package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jackc/pgx/v5"
	"github.com/spf13/cobra"

	"github.com/dusansimic/porukator/internal/auth"
	"github.com/dusansimic/porukator/internal/pgconv"
	"github.com/dusansimic/porukator/internal/repository"
)

// roleFromFlag maps the --admin bool to a DB role.
func roleFromAdmin(admin bool) repository.UserRole {
	if admin {
		return repository.UserRoleAdmin
	}
	return repository.UserRoleManager
}

// lookupUser resolves a username to its row, with a friendly not-found error.
func lookupUser(ctx context.Context, q *repository.Queries, username string) (repository.User, error) {
	u, err := q.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repository.User{}, fmt.Errorf("no such user: %q", username)
		}
		return repository.User{}, err
	}
	return u, nil
}

func newCreateCmd() *cobra.Command {
	var username, password string
	var admin bool
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user (manager by default; --admin for admin)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if username == "" {
				return errors.New("--username is required")
			}
			if password == "" {
				var err error
				if password, err = promptPassword(); err != nil {
					return err
				}
			}
			if password == "" {
				return errors.New("password must not be empty")
			}
			ctx := cmd.Context()
			pool, q, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer pool.Close()

			hash, err := auth.HashPassword(password)
			if err != nil {
				return err
			}
			u, err := q.CreateUser(ctx, repository.CreateUserParams{
				Username:     username,
				PasswordHash: hash,
				Role:         roleFromAdmin(admin),
			})
			if err != nil {
				return fmt.Errorf("create user (is the username taken?): %w", err)
			}
			fmt.Printf("created %s user %q (%s)\n", u.Role, u.Username, pgconv.UUIDString(u.ID))
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login name (required)")
	cmd.Flags().StringVar(&password, "password", "", "password (prompted if omitted)")
	cmd.Flags().BoolVar(&admin, "admin", false, "create an admin (default role is manager)")
	return cmd
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all users",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			pool, q, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer pool.Close()

			users, err := q.ListUsers(ctx)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "USERNAME\tROLE\tSTATUS\tID")
			for _, u := range users {
				status := "active"
				if u.Disabled {
					status = "disabled"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", u.Username, u.Role, status, pgconv.UUIDString(u.ID))
			}
			return tw.Flush()
		},
	}
}

func newSetRoleCmd() *cobra.Command {
	var username, role string
	cmd := &cobra.Command{
		Use:   "set-role",
		Short: "Change a user's role (admin|manager)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if username == "" {
				return errors.New("--username is required")
			}
			var r repository.UserRole
			switch role {
			case "admin":
				r = repository.UserRoleAdmin
			case "manager":
				r = repository.UserRoleManager
			default:
				return errors.New("--role must be admin or manager")
			}
			ctx := cmd.Context()
			pool, q, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer pool.Close()

			u, err := lookupUser(ctx, q, username)
			if err != nil {
				return err
			}
			if _, err := q.SetUserRole(ctx, repository.SetUserRoleParams{ID: u.ID, Role: r}); err != nil {
				return err
			}
			fmt.Printf("%q is now %s\n", username, r)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login name (required)")
	cmd.Flags().StringVar(&role, "role", "", "admin|manager (required)")
	return cmd
}

func newDisableCmd() *cobra.Command {
	var username string
	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable a user (blocks login and revokes their sessions)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if username == "" {
				return errors.New("--username is required")
			}
			ctx := cmd.Context()
			pool, q, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer pool.Close()

			u, err := lookupUser(ctx, q, username)
			if err != nil {
				return err
			}
			if _, err := q.SetUserDisabled(ctx, repository.SetUserDisabledParams{ID: u.ID, Disabled: true}); err != nil {
				return err
			}
			if err := q.DeleteSessionsByUser(ctx, u.ID); err != nil {
				return err
			}
			fmt.Printf("disabled %q (sessions revoked)\n", username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login name (required)")
	return cmd
}

func newEnableCmd() *cobra.Command {
	var username string
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Re-enable a disabled user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if username == "" {
				return errors.New("--username is required")
			}
			ctx := cmd.Context()
			pool, q, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer pool.Close()

			u, err := lookupUser(ctx, q, username)
			if err != nil {
				return err
			}
			if _, err := q.SetUserDisabled(ctx, repository.SetUserDisabledParams{ID: u.ID, Disabled: false}); err != nil {
				return err
			}
			fmt.Printf("enabled %q\n", username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login name (required)")
	return cmd
}

func newDeleteCmd() *cobra.Command {
	var username string
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a user (sessions and API keys cascade; devices become unowned)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if username == "" {
				return errors.New("--username is required")
			}
			ctx := cmd.Context()
			pool, q, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer pool.Close()

			u, err := lookupUser(ctx, q, username)
			if err != nil {
				return err
			}
			if err := q.DeleteUser(ctx, u.ID); err != nil {
				return err
			}
			fmt.Printf("deleted %q\n", username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login name (required)")
	return cmd
}

func newPasswdCmd() *cobra.Command {
	var username, password string
	cmd := &cobra.Command{
		Use:   "passwd",
		Short: "Reset a user's password",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if username == "" {
				return errors.New("--username is required")
			}
			if password == "" {
				var err error
				if password, err = promptPassword(); err != nil {
					return err
				}
			}
			if password == "" {
				return errors.New("password must not be empty")
			}
			ctx := cmd.Context()
			pool, q, err := openStore(ctx)
			if err != nil {
				return err
			}
			defer pool.Close()

			u, err := lookupUser(ctx, q, username)
			if err != nil {
				return err
			}
			hash, err := auth.HashPassword(password)
			if err != nil {
				return err
			}
			if err := q.SetUserPassword(ctx, repository.SetUserPasswordParams{ID: u.ID, PasswordHash: hash}); err != nil {
				return err
			}
			fmt.Printf("password updated for %q\n", username)
			return nil
		},
	}
	cmd.Flags().StringVar(&username, "username", "", "login name (required)")
	cmd.Flags().StringVar(&password, "password", "", "new password (prompted if omitted)")
	return cmd
}
