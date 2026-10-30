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
	"path"
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
	Image struct {
		ContentURL string `json:"content_url"` // the original upload, often tens of megapixels
		Width      int    `json:"width"`
		Height     int    `json:"height"`
	} `json:"image"`
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
	a.Image = thumbImage(rec.Image.ContentURL, rec.Image.Width, rec.Image.Height)
	return a, nil
}

// thumbWidth is the widest image the feed needs; the same limit the API
// source stores (pageimages' pithumbsize).
const thumbWidth = 960

// Formats MediaWiki derives thumbnails for by this URL scheme. Others
// (tiff, pdf, video posters…) use format-specific thumb names; their rare
// originals are kept as they are rather than guessed at.
var thumbExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".svg": true,
}

// thumbImage converts a dump record's original upload into the same ≤960px
// thumbnail the API source stores: the original can be tens of megapixels,
// far too heavy for a feed card. MediaWiki thumbnail URLs are derived from
// the upload path:
//
//	…/wikipedia/commons/6/64/Name.jpg
//	→ …/wikipedia/commons/thumb/6/64/Name.jpg/960px-Name.jpg
//
// with .png appended for an SVG's raster thumb (SVGs render at any size,
// so they always take the thumb; the app can't draw raw SVG). Bitmaps
// already within the limit can't be upscaled and keep the original URL;
// so does anything whose thumb name this scheme can't derive.
func thumbImage(contentURL string, w, h int) *Image {
	if contentURL == "" {
		return nil
	}
	original := &Image{URL: contentURL, Width: w, Height: h}
	ext := strings.ToLower(path.Ext(contentURL))
	if !thumbExts[ext] {
		return original
	}
	svg := ext == ".svg"
	if !svg && (w <= thumbWidth || w <= 0 || h <= 0) {
		return original
	}
	slash := strings.LastIndex(contentURL, "/")
	name := contentURL[slash+1:]
	dir := contentURL[:slash]
	// The hashed shard lives two segments up: …/<proj>/<h1>/<h2>/<name>.
	// "thumb" goes before the shard; a URL too shallow for that shape
	// (scheme + host + a project segment + the two shard segments) can't
	// be derived — an odd mirror, or a direct thumb already.
	parts := strings.Split(dir, "/")
	if len(parts) < 6 {
		return original
	}
	proj := strings.Join(parts[:len(parts)-2], "/")
	shard := strings.Join(parts[len(parts)-2:], "/")
	thumbName := name
	if svg {
		thumbName += ".png"
	}
	scaledW := thumbWidth
	scaledH := 0
	if w > 0 && h > 0 {
		scaledH = int(float64(h)*float64(thumbWidth)/float64(w) + 0.5)
	}
	return &Image{
		URL:    fmt.Sprintf("%s/thumb/%s/%s/%dpx-%s", proj, shard, name, thumbWidth, thumbName),
		Width:  scaledW,
		Height: scaledH,
	}
}
