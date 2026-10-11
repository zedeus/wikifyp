package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/zedeus/wikifyp/server/internal/config"
	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/source"
)

// runIngest implements the ingest subcommand: fetch articles by title from a
// source and store them in the database (when DATABASE_URL is set), write
// them as fixture files (with -write-fixture), or both. This is the only
// part of the system that touches the network.
func runIngest(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	srcName := fs.String("source", "wikipedia", "article source: wikipedia, simplewiki or fixture")
	fixturesDir := fs.String("fixtures", "fixtures/articles", "fixtures directory (read by -source fixture, written by -write-fixture)")
	writeFixture := fs.Bool("write-fixture", false, "also save each fetched article as a fixture pair")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: wikifyp ingest [flags] <title>...")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			// Usage was printed; asking for help is not a failure.
			return nil
		}
		return err
	}
	titles := fs.Args()
	if len(titles) == 0 {
		fs.Usage()
		return errors.New("ingest: no titles given")
	}
	// Flag parsing stops at the first non-flag argument, so a flag placed
	// after a title would silently be fetched as an article. An explicit
	// "--" says the dash is really part of a title (Wikipedia has those).
	if !slices.Contains(args, "--") {
		for _, title := range titles {
			if strings.HasPrefix(title, "-") {
				return fmt.Errorf("ingest: %q looks like a flag: flags go before titles (or use -- before a title that starts with a dash)", title)
			}
		}
	}

	var src source.Source
	switch *srcName {
	case "wikipedia":
		src = source.Wikipedia()
	case "simplewiki":
		src = source.SimpleWiki()
	case "fixture":
		src = &source.Fixtures{Dir: *fixturesDir}
	default:
		return fmt.Errorf("ingest: unknown source %q", *srcName)
	}

	var db *sql.DB
	if dsn := config.FromEnv().DatabaseURL; dsn != "" {
		var err error
		if db, err = openDB(ctx, dsn); err != nil {
			return fmt.Errorf("ingest: %w", err)
		}
		defer db.Close()
	}
	if db == nil && !*writeFixture {
		return errors.New("ingest: nothing to do: set WIKIFYP_DATABASE_URL to store articles, or pass -write-fixture")
	}

	for _, title := range titles {
		a, err := src.Fetch(ctx, title)
		if err != nil {
			return err
		}
		// Ingest validates again, but WriteFixture has no validation of its
		// own: a fixture written without provenance must never happen.
		if err := a.Validate(); err != nil {
			return err
		}
		if *writeFixture {
			slug, err := source.WriteFixture(*fixturesDir, a)
			if err != nil {
				return fmt.Errorf("write fixture for %q: %w", a.Title, err)
			}
			log.Printf("%s: wrote fixture %s (revision %d, %d links, %d categories)",
				a.Title, slug, a.RevisionID, len(a.Links), len(a.Categories))
		}
		if db != nil {
			res, err := ingest.Article(ctx, db, a)
			if err != nil {
				return err
			}
			if res.Created {
				log.Printf("%s: stored article %d, %d sentences", a.Title, res.ArticleID, res.Sentences)
			} else {
				log.Printf("%s: revision %d already stored as article %d", a.Title, a.RevisionID, res.ArticleID)
			}
		}
	}
	return nil
}
