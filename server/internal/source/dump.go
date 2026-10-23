package source

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

// dumpRecord is one line of a Wikimedia Enterprise HTML dump: the fields of
// the snapshot schema that ingestion needs.
type dumpRecord struct {
	Name       string `json:"name"`
	Identifier int64  `json:"identifier"` // wiki page ID
	URL        string `json:"url"`
	Version    struct {
		Identifier int64 `json:"identifier"` // revision ID
	} `json:"version"`
	Namespace struct {
		Identifier int `json:"identifier"`
	} `json:"namespace"`
	IsPartOf struct {
		Identifier string `json:"identifier"` // e.g. "simplewiki"
	} `json:"is_part_of"`
	License []struct {
		Identifier string `json:"identifier"` // e.g. "CC-BY-SA-3.0"
	} `json:"license"`
	Categories []struct {
		Name string `json:"name"` // "Category:Chemistry"
	} `json:"categories"`
	ArticleBody struct {
		HTML string `json:"html"`
	} `json:"article_body"`
}

// ErrSkip, returned by a ReadDump callback, skips the current article
// without stopping the read.
var ErrSkip = errors.New("skip article")

// ReadDump streams the articles of a Wikimedia Enterprise HTML dump (a
// .json.tar.gz: a tar of NDJSON files, one JSON record per article) and
// calls fn for each one, in dump order. Redirect pages and records outside
// the article namespace are skipped. Any other error from fn stops the
// read; fn can return ErrSkip to move on instead. ReadDump returns how many
// articles fn accepted and how many records were skipped.
//
// The record's category names are stored without the "Category:" prefix,
// matching the API source. Unlike the API's visible-only category list, a
// dump record also names hidden maintenance categories; ingest filters them
// against the wiki's category metadata (category_pages, refreshed by
// fetch-categories), which is what makes the two sources store the same
// thing.
func ReadDump(ctx context.Context, r io.Reader, fetched time.Time, fn func(*Article) error) (accepted, skipped int, err error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return 0, 0, fmt.Errorf("read dump: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return accepted, skipped, nil
		}
		if err != nil {
			return accepted, skipped, fmt.Errorf("read dump: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		scanner := bufio.NewScanner(tr)
		// A record is a whole article with its HTML; the largest run to
		// megabytes.
		scanner.Buffer(make([]byte, 1<<20), 64<<20)
		for scanner.Scan() {
			if err := ctx.Err(); err != nil {
				return accepted, skipped, err
			}
			var rec dumpRecord
			if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
				return accepted, skipped, fmt.Errorf("read dump: record %d: %w", accepted+skipped+1, err)
			}
			a, err := rec.article(fetched)
			if err != nil {
				return accepted, skipped, fmt.Errorf("read dump: %w", err)
			}
			if a == nil {
				skipped++
				continue
			}
			switch err := fn(a); {
			case errors.Is(err, ErrSkip):
				skipped++
			case err != nil:
				return accepted, skipped, err
			default:
				accepted++
			}
		}
		if err := scanner.Err(); err != nil {
			return accepted, skipped, fmt.Errorf("read dump: %s: %w", hdr.Name, err)
		}
	}
}

// article converts one dump record, or returns nil for a page that is not
// an article (a redirect, or outside the main namespace).
func (rec *dumpRecord) article(fetched time.Time) (*Article, error) {
	if rec.Namespace.Identifier != 0 {
		return nil, nil
	}
	// is_part_of names the wiki; the shared registry keeps dump-imported
	// articles indistinguishable from API-ingested ones.
	wiki, ok := wikis[rec.IsPartOf.Identifier]
	if !ok {
		return nil, fmt.Errorf("record %q: unknown wiki %q", rec.Name, rec.IsPartOf.Identifier)
	}
	page, err := ParseParsoid(strings.NewReader(rec.ArticleBody.HTML))
	if err != nil {
		return nil, fmt.Errorf("record %q: %w", rec.Name, err)
	}
	if page.Redirect {
		return nil, nil
	}
	license := ""
	if len(rec.License) > 0 {
		license = rec.License[0].Identifier
	}
	a := &Article{
		Title:       rec.Name,
		Source:      wiki.source,
		URL:         rec.URL,
		PageID:      rec.Identifier,
		RevisionID:  rec.Version.Identifier,
		License:     license,
		Attribution: attribution(wiki.site, rec.Name, rec.URL, rec.Version.Identifier, license),
		Fetched:     fetched.Format("2006-01-02"),
		Links:       page.Links,
		Text:        page.Text,
	}
	for _, c := range rec.Categories {
		a.Categories = append(a.Categories, strings.TrimPrefix(c.Name, "Category:"))
	}
	return a, nil
}
