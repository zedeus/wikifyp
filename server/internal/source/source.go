// Package source fetches Wikipedia articles and defines their shape.
//
// A Source produces Articles: the fixture source reads them from the
// committed fixtures tree, and the MediaWiki source fetches them live from
// a wiki's API (used only by the ingest command, never by tests). Article
// text is kept exactly as the API's plain-text extract returns it; Clean
// strips what the extract format leaves behind before the text is split
// and stored.
package source

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrNotFound reports that a source has no article with the requested title.
var ErrNotFound = errors.New("article not found")

// Article is one revision of a wiki article, as fetched from a source.
// Text is the raw plain-text extract; cleaning happens at ingest so every
// source goes through the same pipeline.
//
// The JSON tags are the fixture metadata format, and the field order is its
// on-disk key order: a fixture is this struct marshaled to <slug>.json, with
// Text in <slug>.txt beside it.
type Article struct {
	Title       string   `json:"title"`                // canonical title, redirects resolved
	Source      string   `json:"source"`               // which wiki: "wikipedia" or "simplewiki"
	URL         string   `json:"url"`                  // canonical page URL
	PageID      int64    `json:"page_id,omitempty"`    // wiki page ID; 0 when unknown (older fixtures)
	RevisionID  int64    `json:"revision_id"`          // revision the text was taken from
	License     string   `json:"license"`              // e.g. "CC BY-SA 4.0"
	Attribution string   `json:"attribution"`          // human-readable attribution line
	Fetched     string   `json:"fetched"`              // date the text was fetched, YYYY-MM-DD
	Links       []string `json:"links,omitempty"`      // outgoing article links (main namespace)
	Categories  []string `json:"categories,omitempty"` // visible categories, "Category:" stripped
	Text        string   `json:"-"`                    // raw plain-text extract
}

// Validate reports the fields an article cannot be ingested without.
// Revision, license and attribution are what let stored text be traced to
// its source and redistributed; the rest is what ingestion itself needs.
func (a *Article) Validate() error {
	var missing []string
	for _, f := range []struct {
		name string
		ok   bool
	}{
		{"source", a.Source != ""},
		{"title", a.Title != ""},
		{"url", a.URL != ""},
		{"revision_id", a.RevisionID > 0},
		{"license", a.License != ""},
		{"attribution", a.Attribution != ""},
		{"fetched", validDate(a.Fetched)},
		{"text", strings.TrimSpace(a.Text) != ""},
	} {
		if !f.ok {
			missing = append(missing, f.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("article %q: missing or invalid %s", a.Title, strings.Join(missing, ", "))
	}
	return nil
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// Source fetches one article by title.
type Source interface {
	Fetch(ctx context.Context, title string) (*Article, error)
}
