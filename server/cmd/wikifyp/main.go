// Command wikifyp runs the wikifyp API server.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/zedeus/wikifyp/server/internal/api"
	"github.com/zedeus/wikifyp/server/internal/config"
	"github.com/zedeus/wikifyp/server/internal/devui"
	"github.com/zedeus/wikifyp/server/internal/migrate"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var err error
	switch {
	case len(os.Args) == 1:
		err = serve(ctx)
	case os.Args[1] == "ingest":
		err = runIngest(ctx, os.Args[2:])
	case os.Args[1] == "import-dump":
		err = runImportDump(ctx, os.Args[2:])
	case os.Args[1] == "fetch-categories":
		err = runFetchCategories(ctx, os.Args[2:])
	case os.Args[1] == "export-assets":
		err = runExportAssets(ctx, os.Args[2:])
	default:
		// A typo must not silently start the server instead of ingesting.
		err = fmt.Errorf("unknown command %q (commands are \"ingest\", \"import-dump\", \"fetch-categories\" and \"export-assets\"; run with no arguments to serve)", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

// openDB opens the database and brings its schema up to date. Both serving
// and ingesting go through it, so the two paths cannot drift.
func openDB(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := migrate.Apply(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func serve(ctx context.Context) error {
	cfg := config.FromEnv()

	var db *sql.DB
	var checks []func(context.Context) error
	if cfg.DatabaseURL != "" {
		var err error
		if db, err = openDB(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
		defer db.Close()
		checks = append(checks, db.PingContext)
	}

	handler := http.Handler(api.New(db, checks...))
	if db != nil {
		// The development inspection UI. It can write reader state, so it
		// rides inside the server rather than being exposed separately;
		// the whole server is a local prototype today.
		dev := devui.New(db, "")
		root := http.NewServeMux()
		root.Handle("/", handler)
		root.Handle("/dev", dev)
		root.Handle("/dev/", dev)
		handler = root
	}

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: handler,
		// A client trickling its header bytes must not hold a connection
		// open forever.
		ReadHeaderTimeout: 5 * time.Second,
	}
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
