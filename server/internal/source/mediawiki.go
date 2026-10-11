package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MediaWiki fetches articles live from a wiki's API. It is used by the
// ingest command only; tests use fixtures or recorded responses. It is not
// safe for concurrent Fetch calls: the rate limiter assumes one goroutine
// (the ingest command's pace), per Wikimedia API etiquette.
type MediaWiki struct {
	Name      string        // source name stored with articles, e.g. "wikipedia"
	Site      string        // human-readable site name for attribution
	API       string        // api.php endpoint
	UserAgent string        // descriptive UA, per Wikimedia etiquette
	Interval  time.Duration // minimum gap between requests
	Client    *http.Client

	last time.Time
}

// Wikipedia returns a source for the English Wikipedia.
func Wikipedia() *MediaWiki {
	return newMediaWiki("wikipedia", "Wikipedia", "https://en.wikipedia.org/w/api.php")
}

// SimpleWiki returns a source for the Simple English Wikipedia.
func SimpleWiki() *MediaWiki {
	return newMediaWiki("simplewiki", "Simple English Wikipedia", "https://simple.wikipedia.org/w/api.php")
}

func newMediaWiki(name, site, api string) *MediaWiki {
	return &MediaWiki{
		Name:      name,
		Site:      site,
		API:       api,
		UserAgent: "wikifyp/0.1 (https://github.com/zedeus/wikifyp; article ingest)",
		Interval:  time.Second,
		Client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// license is what Wikimedia wikis publish their text under.
const license = "CC BY-SA 4.0"

// apiResponse is the subset of the MediaWiki query API this client reads.
type apiResponse struct {
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
	// Continuation values are usually strings but some modules send numbers
	// (TextExtracts' excontinue), so this is any; get decodes with UseNumber
	// so a number round-trips to its exact digits via fmt.Sprint.
	Continue map[string]any `json:"continue"`
	Query    struct {
		Pages []struct {
			PageID    int64  `json:"pageid"`
			Title     string `json:"title"`
			Missing   bool   `json:"missing"`
			Invalid   bool   `json:"invalid"`
			Extract   string `json:"extract"`
			FullURL   string `json:"fullurl"`
			Revisions []struct {
				RevID int64 `json:"revid"`
			} `json:"revisions"`
			Links []struct {
				Title string `json:"title"`
			} `json:"links"`
			Categories []struct {
				Title string `json:"title"`
			} `json:"categories"`
		} `json:"pages"`
	} `json:"query"`
}

// Fetch retrieves one article: its plain-text extract, revision, canonical
// URL, outgoing links and visible categories, following API continuation
// until the link and category lists are complete.
func (w *MediaWiki) Fetch(ctx context.Context, title string) (*Article, error) {
	params := url.Values{
		"action":        {"query"},
		"format":        {"json"},
		"formatversion": {"2"},
		"redirects":     {"1"},
		"titles":        {title},
		"prop":          {"extracts|revisions|info|links|categories"},
		"explaintext":   {"1"},
		"rvprop":        {"ids"},
		"inprop":        {"url"},
		"pllimit":       {"max"},
		"plnamespace":   {"0"},
		"cllimit":       {"max"},
		"clshow":        {"!hidden"},
	}

	a := &Article{
		Source:  w.Name,
		License: license,
		Fetched: time.Now().UTC().Format("2006-01-02"),
	}
	var contKeys []string
	for {
		resp, err := w.get(ctx, params)
		if err != nil {
			return nil, fmt.Errorf("%s: fetch %q: %w", w.Name, title, err)
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: fetch %q: API error %s: %s", w.Name, title, resp.Error.Code, resp.Error.Info)
		}
		// Exactly one page must come back: a title containing "|" would make
		// the API answer for several, and Pages[0] would silently be the
		// wrong article.
		if len(resp.Query.Pages) != 1 {
			return nil, fmt.Errorf("%s: fetch %q: expected one page, got %d", w.Name, title, len(resp.Query.Pages))
		}
		page := resp.Query.Pages[0]
		if page.Missing {
			return nil, fmt.Errorf("%s: %q: %w", w.Name, title, ErrNotFound)
		}
		if page.Invalid {
			return nil, fmt.Errorf("%s: %q: invalid title", w.Name, title)
		}
		// With a continue chain the API may defer any prop module's data to
		// a later response, so each single-valued field is taken from
		// whichever response first carries it; links and categories
		// accumulate across all of them.
		if a.Title == "" {
			a.Title = page.Title
		}
		if a.URL == "" {
			a.URL = page.FullURL
		}
		if a.PageID == 0 {
			a.PageID = page.PageID
		}
		if a.Text == "" {
			a.Text = page.Extract
		}
		if a.RevisionID == 0 && len(page.Revisions) > 0 {
			a.RevisionID = page.Revisions[0].RevID
		}
		for _, l := range page.Links {
			a.Links = append(a.Links, l.Title)
		}
		for _, c := range page.Categories {
			name := c.Title
			if _, after, found := strings.Cut(c.Title, ":"); found && after != "" {
				name = after
			}
			a.Categories = append(a.Categories, name)
		}

		if len(resp.Continue) == 0 {
			break
		}
		// Each response's continue block is the complete set of continuation
		// params for the next request. Drop the previous round's before
		// setting this one's: a param left over from a module that has
		// finished (say clcontinue after categories completed) would make the
		// API return that module's pages again, duplicating its values.
		for _, k := range contKeys {
			params.Del(k)
		}
		contKeys = contKeys[:0]
		for k, v := range resp.Continue {
			params.Set(k, fmt.Sprint(v))
			contKeys = append(contKeys, k)
		}
	}
	a.Attribution = fmt.Sprintf("Text from the %s article %q, %s (revision %d), licensed %s",
		w.Site, a.Title, a.URL, a.RevisionID, license)
	return a, nil
}

// get performs one rate-limited API request.
func (w *MediaWiki) get(ctx context.Context, params url.Values) (*apiResponse, error) {
	if wait := w.Interval - time.Since(w.last); wait > 0 {
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	w.last = time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.API+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", w.UserAgent)
	res, err := w.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", res.Status)
	}
	dec := json.NewDecoder(io.LimitReader(res.Body, 32<<20))
	// Numbers in the continue block must keep their exact digits when echoed
	// back as parameters; the default float64 would mangle large IDs.
	dec.UseNumber()
	var resp apiResponse
	if err := dec.Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp, nil
}
