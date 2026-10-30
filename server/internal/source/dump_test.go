package source

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const dumpSample = "../../../fixtures/dumps/simplewiki-sample.json.tar.gz"

// readSample reads the committed dump sample into a map by title.
func readSample(t *testing.T) map[string]*Article {
	t.Helper()
	f, err := os.Open(dumpSample)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	articles := make(map[string]*Article)
	fetched := time.Date(2026, 10, 14, 0, 0, 0, 0, time.UTC)
	accepted, skipped, err := ReadDump(context.Background(), f, fetched, func(a *Article) error {
		articles[a.Title] = a
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if accepted != 6 || skipped != 0 {
		t.Fatalf("accepted %d, skipped %d, want 6 and 0", accepted, skipped)
	}
	return articles
}

// TestReadDumpGroundTruth checks the Parsoid conversion of two real dump
// records against hand-written expected text (fixtures/dumps/*.expected.txt,
// authored by reading the records' HTML, never generated from code).
func TestReadDumpGroundTruth(t *testing.T) {
	articles := readSample(t)
	tests := []struct {
		title    string
		expected string // file under fixtures/dumps
		links    []string
	}{
		{
			title:    "Fossil word",
			expected: "fossil-word.expected.txt",
			links:    []string{"Idiom", "Phrase"},
		},
		{
			title:    "Cloverleaf quasar",
			expected: "cloverleaf-quasar.expected.txt",
			links:    []string{"Quasar", "Cloverleaf", "Constellation", "Boötes (constellation)", "Redshift"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			a := articles[tt.title]
			if a == nil {
				t.Fatalf("article %q not read from sample", tt.title)
			}
			raw, err := os.ReadFile(filepath.Join(filepath.Dir(dumpSample), tt.expected))
			if err != nil {
				t.Fatal(err)
			}
			want := strings.TrimRight(string(raw), "\n")
			if a.Text != want {
				t.Errorf("text:\n%s\nwant:\n%s", a.Text, want)
			}
			if !slices.Equal(a.Links, tt.links) {
				t.Errorf("links = %q, want %q", a.Links, tt.links)
			}
		})
	}
}

// TestReadDumpMetadata checks that every sample article carries full
// provenance and the fields ingest needs, straight from the record.
func TestReadDumpMetadata(t *testing.T) {
	articles := readSample(t)
	for title, a := range articles {
		if err := a.Validate(); err != nil {
			t.Errorf("%s: %v", title, err)
		}
		if a.Source != "simplewiki" {
			t.Errorf("%s: source = %q", title, a.Source)
		}
	}

	fw := articles["Fossil word"]
	if fw.PageID != 621493 || fw.RevisionID != 8803606 {
		t.Errorf("Fossil word ids = %d/%d", fw.PageID, fw.RevisionID)
	}
	if fw.License != "CC-BY-SA-4.0" {
		t.Errorf("Fossil word license = %q", fw.License)
	}
	want := `Text from the Simple English Wikipedia article "Fossil word", ` +
		`https://simple.wikipedia.org/wiki/Fossil_word (revision 8803606), licensed CC-BY-SA-4.0`
	if fw.Attribution != want {
		t.Errorf("attribution = %q", fw.Attribution)
	}
	if got := fw.Categories; !slices.Equal(got, []string{"Stubs", "Vocabulary"}) {
		t.Errorf("categories = %q", got)
	}

	// The featured marker survives: dump categories include hidden ones.
	rhcp := articles["Red Hot Chili Peppers"]
	if !slices.Contains(rhcp.Categories, "Very good articles") {
		t.Errorf("RHCP categories missing featured marker: %q", rhcp.Categories)
	}

	// A malformed redirect (external-link syntax) is an ordinary one-line
	// page, and a three-character stub comes through as its one word.
	if got := articles["Spraycan"].Text; got != "REDIRECT" {
		t.Errorf("Spraycan text = %q", got)
	}
	if got := articles["History of Canada"].Text; got != "Far" {
		t.Errorf("History of Canada text = %q", got)
	}

	// The one sample record with a lead image: 640px wide is already
	// within the thumb limit, so the original upload URL is stored as-is.
	img := rhcp.Image
	if img == nil ||
		img.URL != "https://upload.wikimedia.org/wikipedia/commons/1/16/Rhcp-live-pinkpop05.jpg" ||
		img.Width != 640 || img.Height != 474 {
		t.Errorf("RHCP image = %+v, want the original 640x474 upload", img)
	}
	if articles["Fossil word"].Image != nil {
		t.Errorf("Fossil word image = %+v, want none", articles["Fossil word"].Image)
	}
}

// TestThumbImage pins the thumbnail derivation against the URL scheme the
// API source stores (see fixtures/articles/*.json image URLs — the same
// /thumb/<shard>/<name>/960px-<name> shape, hand-checked, with .png
// appended for SVG rasters). Heights are hand-computed from the ratios.
func TestThumbImage(t *testing.T) {
	tests := []struct {
		name string
		url  string
		w, h int
		want *Image
	}{
		{
			// barack-obama.json stores this file's 960px thumb at 960x1199;
			// the original upload is 2687x3356.
			name: "large jpg scales to the 960px thumb",
			url:  "https://upload.wikimedia.org/wikipedia/commons/8/8d/President_Barack_Obama.jpg",
			w:    2687,
			h:    3356,
			want: &Image{
				URL:    "https://upload.wikimedia.org/wikipedia/commons/thumb/8/8d/President_Barack_Obama.jpg/960px-President_Barack_Obama.jpg",
				Width:  960,
				Height: 1199,
			},
		},
		{
			// antarctica.json: an SVG's raster thumb appends .png; the
			// URL-encoded parentheses ride through untouched.
			name: "svg thumbs to png even when nominally small",
			url:  "https://upload.wikimedia.org/wikipedia/commons/f/f2/Antarctica_%28orthographic_projection%29.svg",
			w:    531,
			h:    530,
			want: &Image{
				URL:    "https://upload.wikimedia.org/wikipedia/commons/thumb/f/f2/Antarctica_%28orthographic_projection%29.svg/960px-Antarctica_%28orthographic_projection%29.svg.png",
				Width:  960,
				Height: 958,
			},
		},
		{
			name: "bitmap within the limit keeps the original",
			url:  "https://upload.wikimedia.org/wikipedia/commons/1/16/Rhcp-live-pinkpop05.jpg",
			w:    640,
			h:    474,
			want: &Image{
				URL:    "https://upload.wikimedia.org/wikipedia/commons/1/16/Rhcp-live-pinkpop05.jpg",
				Width:  640,
				Height: 474,
			},
		},
		{
			name: "format without this thumb scheme keeps the original",
			url:  "https://upload.wikimedia.org/wikipedia/commons/a/ab/Scan.tiff",
			w:    4000,
			h:    3000,
			want: &Image{
				URL:    "https://upload.wikimedia.org/wikipedia/commons/a/ab/Scan.tiff",
				Width:  4000,
				Height: 3000,
			},
		},
		{
			name: "bitmap without known dimensions keeps the original",
			url:  "https://upload.wikimedia.org/wikipedia/commons/8/8d/Huge.jpg",
			w:    0,
			h:    0,
			want: &Image{
				URL:   "https://upload.wikimedia.org/wikipedia/commons/8/8d/Huge.jpg",
				Width: 0, Height: 0,
			},
		},
		{
			name: "url too shallow to carry the shard keeps the original",
			url:  "https://example.com/Photo.jpg",
			w:    4000,
			h:    3000,
			want: &Image{URL: "https://example.com/Photo.jpg", Width: 4000, Height: 3000},
		},
		{
			name: "no image",
			url:  "",
			w:    0,
			h:    0,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := thumbImage(tt.url, tt.w, tt.h)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("thumbImage = %+v, want nil", got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Errorf("thumbImage = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestReadDumpSkipAndError(t *testing.T) {
	open := func(t *testing.T) *os.File {
		f, err := os.Open(dumpSample)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}

	// ErrSkip moves on; the article is counted as skipped.
	accepted, skipped, err := ReadDump(context.Background(), open(t), time.Now(), func(a *Article) error {
		if a.Title == "Spraycan" {
			return ErrSkip
		}
		return nil
	})
	if err != nil || accepted != 5 || skipped != 1 {
		t.Errorf("skip: accepted %d, skipped %d, err %v", accepted, skipped, err)
	}

	// Any other error stops the read.
	boom := errors.New("boom")
	_, _, err = ReadDump(context.Background(), open(t), time.Now(), func(a *Article) error {
		return boom
	})
	if !errors.Is(err, boom) {
		t.Errorf("error not propagated: %v", err)
	}

	// Cancellation stops the read.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err = ReadDump(ctx, open(t), time.Now(), func(a *Article) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("cancellation not seen: %v", err)
	}

	// Not a dump at all.
	if _, _, err := ReadDump(context.Background(), strings.NewReader("not gzip"), time.Now(), nil); err == nil {
		t.Error("garbage accepted")
	}
}

// A dump cut off mid-stream (interrupted download) must fail loudly, not
// end as if complete.
func TestReadDumpTruncated(t *testing.T) {
	whole, err := os.ReadFile(dumpSample)
	if err != nil {
		t.Fatal(err)
	}
	truncated := whole[:len(whole)/2]
	_, _, err = ReadDump(context.Background(), bytes.NewReader(truncated), time.Now(), func(a *Article) error { return nil })
	if err == nil {
		t.Fatal("truncated dump read to the end without error")
	}
}
