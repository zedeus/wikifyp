package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/zedeus/wikifyp/server/internal/config"
	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/source"
)

// runImportDump implements the import-dump subcommand: stream a Wikimedia
// Enterprise HTML dump into the database through the same ingest pipeline
// the ingest command uses. A record that cannot be ingested on its own
// (missing provenance, or nothing readable left after cleaning) is logged
// and skipped rather than aborting an hours-long import; nothing is ever
// stored without provenance, because ingest still validates every article.
func runImportDump(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import-dump", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: wikifyp import-dump <dump.json.tar.gz>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("import-dump: exactly one dump file expected")
	}

	dsn := config.FromEnv().DatabaseURL
	if dsn == "" {
		return errors.New("import-dump: WIKIFYP_DATABASE_URL is not set")
	}
	db, err := openDB(ctx, dsn)
	if err != nil {
		return fmt.Errorf("import-dump: %w", err)
	}
	defer db.Close()

	f, err := os.Open(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("import-dump: %w", err)
	}
	defer f.Close()

	start := time.Now()
	var stored, unchanged, failed int
	_, skipped, err := source.ReadDump(ctx, f, start, func(a *source.Article) error {
		res, err := ingest.Article(ctx, db, a)
		if err != nil {
			failed++
			log.Printf("skipping %q: %v", a.Title, err)
			return source.ErrSkip
		}
		if res.Created {
			stored++
		} else {
			unchanged++
		}
		if n := stored + unchanged; n%5000 == 0 {
			log.Printf("%d articles in %v (%.0f/s)", n, time.Since(start).Round(time.Second),
				float64(n)/time.Since(start).Seconds())
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("import-dump: %w", err)
	}
	log.Printf("done in %v: %d stored, %d already stored, %d failed records skipped, %d non-articles skipped",
		time.Since(start).Round(time.Second), stored, unchanged, failed, skipped-failed)
	// A bulk write leaves the planner's statistics describing the old,
	// near-empty tables; the first feed queries then pick catastrophic
	// plans (a measured 4.8 s nested loop that hashes in 0.3 s once
	// analyzed). Refreshing them is the bulk writer's job.
	if _, err := db.ExecContext(ctx, "ANALYZE"); err != nil {
		return fmt.Errorf("import-dump: analyze: %w", err)
	}
	return nil
}
