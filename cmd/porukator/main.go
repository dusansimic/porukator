package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/oklog/run"
	"go.uber.org/zap"

	"github.com/dusansimic/porukator/internal/auth"
	"github.com/dusansimic/porukator/internal/config"
	"github.com/dusansimic/porukator/internal/connectsrv"
	"github.com/dusansimic/porukator/internal/db"
	"github.com/dusansimic/porukator/internal/registry"
	"github.com/dusansimic/porukator/pkg/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	log, err := logger.New(cfg.Logging)
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer log.Sync()

	log.Info("starting porukator",
		zap.String("env", cfg.App.Env),
		zap.String("http_addr", cfg.HTTP.Addr))

	bootCtx := context.Background()

	if err := db.Migrate(bootCtx, cfg.Postgres.URL, db.MigrationsFS, "migrations"); err != nil {
		log.Fatal("migrate failed", zap.Error(err))
	}
	log.Info("migrations at head")

	pool, err := db.NewPool(bootCtx, cfg.Postgres.URL)
	if err != nil {
		log.Fatal("pg pool failed", zap.Error(err))
	}
	defer pool.Close()
	log.Info("pg pool ready")

	reg := registry.New()
	handler := connectsrv.NewHandler(log, pool, reg, cfg)
	interceptor := auth.NewInterceptor(pool)
	httpSrv := connectsrv.NewHTTPServer(cfg.HTTP.Addr, handler, interceptor)

	var g run.Group

	// Signal handler.
	{
		ctx, cancel := context.WithCancel(context.Background())
		g.Add(func() error {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			select {
			case sig := <-sigCh:
				log.Info("received signal", zap.String("signal", sig.String()))
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}, func(error) {
			cancel()
		})
	}

	// Connect-RPC HTTP server.
	{
		g.Add(func() error {
			log.Info("http server listening", zap.String("addr", httpSrv.Addr))
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		}, func(error) {
			shutdownCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_ = httpSrv.Shutdown(shutdownCtx)
		})
	}

	if err := g.Run(); err != nil {
		log.Error("application error", zap.Error(err))
		os.Exit(1)
	}
	log.Info("application stopped gracefully")
}
