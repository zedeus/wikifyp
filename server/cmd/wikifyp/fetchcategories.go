package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/zedeus/wikifyp/server/internal/config"
	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/source"
)

// runFetchCategories implements the fetch-categories subcommand: refresh the
// category_pages table for one wiki from its live API, then prune stored
// article categories the fresh metadata excludes. Like ingest, this is a
// live-access command; tests never run it.
func runFetchCategories(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("fetch-categories", flag.ContinueOnError)
	srcName := fs.String("source", "simplewiki", "wiki to fetch from: wikipedia or simplewiki")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: wikifyp fetch-categories [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	var w *source.MediaWiki
	switch *srcName {
	case "wikipedia":
		w = source.Wikipedia()
	case "simplewiki":
		w = source.SimpleWiki()
	default:
		return fmt.Errorf("fetch-categories: unknown source %q", *srcName)
	}

	dsn := config.FromEnv().DatabaseURL
	if dsn == "" {
		return errors.New("fetch-categories: WIKIFYP_DATABASE_URL is not set")
	}
	db, err := openDB(ctx, dsn)
	if err != nil {
		return fmt.Errorf("fetch-categories: %w", err)
	}
	defer db.Close()

	// Replace the source's rows in one transaction: a category deleted or
	// un-hidden on the wiki must not linger, and a failed fetch must not
	// leave a half-written table behind.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("fetch-categories: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, "DELETE FROM category_pages WHERE source = $1", w.Name); err != nil {
		return fmt.Errorf("fetch-categories: clear old rows: %w", err)
	}

	// Accumulate the whole catalogue in memory first: parent lists split
	// across continuation responses (the same page arrives more than once),
	// and the domain walk needs every edge before it can start. ~100K
	// categories with their parents is a few tens of MB.
	start := time.Now()
	byName := make(map[string]*source.CategoryPage)
	var order []string
	err = w.CategoryPages(ctx, func(p source.CategoryPage) error {
		if prev, ok := byName[p.Name]; ok {
			prev.Hidden = prev.Hidden || p.Hidden
			prev.Parents = append(prev.Parents, p.Parents...)
			return nil
		}
		cp := p
		byName[p.Name] = &cp
		order = append(order, p.Name)
		return nil
	})
	if err != nil {
		return fmt.Errorf("fetch-categories: %w", err)
	}
	pages := make([]source.CategoryPage, 0, len(order))
	var hidden int
	for _, name := range order {
		pages = append(pages, *byName[name])
		if byName[name].Hidden {
			hidden++
		}
	}
	domains := source.DomainMap(pages, w.Root)

	names := make([]string, 0, 500)
	flags := make([]bool, 0, 500)
	doms := make([]sql.NullString, 0, 500)
	flush := func() error {
		if len(names) == 0 {
			return nil
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO category_pages (source, category, hidden, domain)
			SELECT $1, * FROM unnest($2::text[], $3::boolean[], $4::text[])`,
			w.Name, names, flags, doms)
		names, flags, doms = names[:0], flags[:0], doms[:0]
		return err
	}
	for _, p := range pages {
		names = append(names, p.Name)
		flags = append(flags, p.Hidden)
		doms = append(doms, sql.NullString{String: domains[p.Name], Valid: domains[p.Name] != ""})
		if len(names) == cap(names) {
			if err := flush(); err != nil {
				return fmt.Errorf("fetch-categories: %w", err)
			}
		}
	}
	if err := flush(); err != nil {
		return fmt.Errorf("fetch-categories: %w", err)
	}

	// Stored articles were ingested under the old metadata (or, for dumps,
	// none at all); drop their categories the fresh metadata excludes so
	// the corpus and future ingests agree. Featured markers survive the
	// prune despite the wiki flagging them hidden (ingest.PruneCategories).
	pruned, err := ingest.PruneCategories(ctx, tx, w.Name)
	if err != nil {
		return fmt.Errorf("fetch-categories: %w", err)
	}

	// Rebuild the exploration domains for this source's articles: each
	// article's domain is the modal domain of its (visible, surviving)
	// categories. Ingest maintains rows for articles stored after this.
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM article_domains ad
		USING articles a WHERE a.id = ad.article_id AND a.source = $1`, w.Name); err != nil {
		return fmt.Errorf("fetch-categories: clear article domains: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO article_domains (article_id, domain)
		SELECT DISTINCT ON (c.article_id) c.article_id, cp.domain
		FROM article_categories c
		JOIN articles a ON a.id = c.article_id AND a.source = $1
		JOIN category_pages cp ON cp.source = a.source AND cp.category = c.category
		WHERE cp.domain IS NOT NULL
		GROUP BY c.article_id, cp.domain
		ORDER BY c.article_id, count(*) DESC, cp.domain`, w.Name); err != nil {
		return fmt.Errorf("fetch-categories: rebuild article domains: %w", err)
	}
	var domained int
	if err := tx.QueryRowContext(ctx, `
		SELECT count(*) FROM article_domains ad
		JOIN articles a ON a.id = ad.article_id AND a.source = $1`, w.Name).Scan(&domained); err != nil {
		return fmt.Errorf("fetch-categories: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("fetch-categories: %w", err)
	}
	if _, err := db.ExecContext(ctx, "ANALYZE"); err != nil {
		return fmt.Errorf("fetch-categories: analyze: %w", err)
	}
	log.Printf("done in %v: %d category pages (%d hidden, %d with domains), %d stored article categories pruned, %d articles assigned domains",
		time.Since(start).Round(time.Second), len(pages), hidden, len(domains), pruned, domained)
	return nil
}
