package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/zedeus/wikifyp/server/internal/api"
	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/source"
)

// runExportAssets rebuilds the app's bundled article JSONs from the fixtures,
// through the same pipeline ingest uses, so the bundled feed shows exactly
// what a served article would (api.ArticleResponse is the one shape both
// carry). Each existing <slug>.json in the assets directory is regenerated
// from the same-slug fixture. Which articles the app bundles — and two facts
// that exist only on the app side — live in the asset files themselves and
// are preserved: the id (an app-local key; bundled articles have no server
// row) and the image (hand-localized to an android_asset path, with its
// provenance in assets/images/NOTICE.md). Everything derived from article
// text comes from the fixture.
func runExportAssets(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("export-assets", flag.ContinueOnError)
	fixturesDir := fs.String("fixtures", "fixtures/articles", "fixtures directory")
	assetsDir := fs.String("assets", "android/app/src/main/assets/articles", "app asset articles directory")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: wikifyp export-assets [flags]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		// The command rewrites every asset in the directory; an argument
		// that looks like it scopes the run must not be silently ignored.
		return fmt.Errorf("export-assets: unexpected argument %q (the assets directory decides what is exported)", fs.Arg(0))
	}

	matches, err := filepath.Glob(filepath.Join(*assetsDir, "*.json"))
	if err != nil {
		return fmt.Errorf("export-assets: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("export-assets: no asset articles in %s", *assetsDir)
	}
	fix := &source.Fixtures{Dir: *fixturesDir}
	for _, path := range matches {
		slug := strings.TrimSuffix(filepath.Base(path), ".json")
		existing, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("export-assets: %w", err)
		}
		var app api.ArticleResponse
		// Unknown fields are refused rather than silently dropped on the
		// rewrite: if the app's bundled format ever grows a field this type
		// does not know, the drift must be loud, not a quiet deletion.
		dec := json.NewDecoder(bytes.NewReader(existing))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&app); err != nil {
			return fmt.Errorf("export-assets: %s: %w", slug, err)
		}

		a, err := fix.Fetch(ctx, slug)
		if err != nil {
			return fmt.Errorf("export-assets: %w", err)
		}
		rows, err := ingest.Rows(a)
		if err != nil {
			return fmt.Errorf("export-assets: %w", err)
		}
		app.Title = a.Title
		app.Attribution = a.Attribution
		app.Description = a.Description
		app.Sentences = make([]api.SentenceResponse, 0, len(rows))
		for _, r := range rows {
			app.Sentences = append(app.Sentences, api.SentenceResponse{Text: r.Text, HeadingLevel: r.Heading})
		}

		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		// These files are read by the app, not a browser: keep "&" and ">"
		// literal so the committed JSON stays diffable prose.
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(&app); err != nil {
			return fmt.Errorf("export-assets: %s: %w", slug, err)
		}
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("export-assets: %w", err)
		}
		log.Printf("%s: %d sentences (article %d)", slug, len(rows), app.ID)
	}
	return nil
}
