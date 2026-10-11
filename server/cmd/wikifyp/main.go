// Command wikifyp runs the wikifyp API server.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/zedeus/wikifyp/server/internal/api"
	"github.com/zedeus/wikifyp/server/internal/config"
	"github.com/zedeus/wikifyp/server/internal/migrate"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config.FromEnv()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var checks []func(context.Context) error
	if cfg.DatabaseURL != "" {
		db, err := sql.Open("pgx", cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()
		if err := migrate.Apply(ctx, db); err != nil {
			return err
		}
		checks = append(checks, db.PingContext)
	}

	srv := &http.Server{Addr: cfg.Addr, Handler: api.New(checks...)}
	errc := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", cfg.Addr)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			if !errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			log.Print("shutdown timed out; abandoning open connections")
		}
		return nil
	}
}
