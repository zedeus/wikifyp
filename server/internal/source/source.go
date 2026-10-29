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

// wikis is the one table of wiki identity: a wiki's dump identifier mapped
// to the source name stored with its articles, the site name attribution
// uses and its API endpoint. The MediaWiki constructors and the dump reader
// both read it, so articles of one wiki are identical whichever way they
// arrived.
var wikis = map[string]struct{ source, site, api, root string }{
	// root is the wiki's top content category: its non-hidden child
	// categories are the topic branches (domains) stratified exploration
	// rotates through.
	"enwiki":     {"wikipedia", "Wikipedia", "https://en.wikipedia.org/w/api.php", "Main topic classifications"},
	"simplewiki": {"simplewiki", "Simple English Wikipedia", "https://simple.wikipedia.org/w/api.php", "Articles"},
}

// attribution builds the attribution line stored with every article.
func attribution(site, title, url string, revisionID int64, license string) string {
	return fmt.Sprintf("Text from the %s article %q, %s (revision %d), licensed %s",
		site, title, url, revisionID, license)
}

// Article is one revision of a wiki article, as fetched from a source.
// Text is the raw plain-text extract; cleaning happens at ingest so every
// source goes through the same pipeline.
//
// The JSON tags are the fixture metadata format, and the field order is its
// on-disk key order: a fixture is this struct marshaled to <slug>.json, with
// Text in <slug>.txt beside it.
type Article struct {
	Title       string   `json:"title"`                 // canonical title, redirects resolved
	Source      string   `json:"source"`                // which wiki: "wikipedia" or "simplewiki"
	URL         string   `json:"url"`                   // canonical page URL
	PageID      int64    `json:"page_id"`               // wiki page ID, the key grouping revisions of one page
	RevisionID  int64    `json:"revision_id"`           // revision the text was taken from
	License     string   `json:"license"`               // e.g. "CC BY-SA 4.0"
	Attribution string   `json:"attribution"`           // human-readable attribution line
	Fetched     string   `json:"fetched"`               // date the text was fetched, YYYY-MM-DD
	Description string   `json:"description,omitempty"` // the wiki's short description, when it has one
	Image       *Image   `json:"image,omitempty"`       // free lead image thumbnail, when the page has one
	Links       []string `json:"links,omitempty"`       // outgoing article links (main namespace)
	Categories  []string `json:"categories,omitempty"`  // visible categories, "Category:" stripped
	Text        string   `json:"-"`                     // raw plain-text extract
}

// Image is an article's lead image thumbnail: the free (pilicense=free)
// image the wiki shows for the page, scaled to at most 960px wide.
type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
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
		{"page_id", a.PageID > 0},
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
