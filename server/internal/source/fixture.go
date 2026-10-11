package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// A fixture is two files in one directory: <slug>.json holding the metadata
// and <slug>.txt holding the raw extract text. Wikipedia articles are named
// by their title alone; any other source prefixes its name so the same topic
// from two wikis cannot collide ("chemistry" vs "simplewiki-chemistry").

// Fixtures is a Source reading committed article fixtures from a directory.
// Fetch accepts a title or a fixture slug.
type Fixtures struct {
	Dir string
}

// Fetch reads the fixture for a title (or slug) from the directory.
func (f *Fixtures) Fetch(ctx context.Context, title string) (*Article, error) {
	slug := Slug(title)
	meta, err := os.ReadFile(filepath.Join(f.Dir, slug+".json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("fixture %s: %w", slug, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("fixture %s: %w", slug, err)
	}
	var a Article
	if err := json.Unmarshal(meta, &a); err != nil {
		return nil, fmt.Errorf("fixture %s: %w", slug, err)
	}
	// Colliding slugs must not answer for each other: a request for "C++"
	// also slugs to "c", but the fixture there is the article "C".
	if title != a.Title && title != fixtureSlug(&a) {
		return nil, fmt.Errorf("fixture %s holds %s article %q, not %q", slug, a.Source, a.Title, title)
	}
	text, err := os.ReadFile(filepath.Join(f.Dir, slug+".txt"))
	if err != nil {
		return nil, fmt.Errorf("fixture %s: %w", slug, err)
	}
	a.Text = string(text)
	return &a, nil
}

// WriteFixture saves an article as a fixture pair in dir and returns the
// slug the files were written under. Overwriting the same article (a newer
// revision) is fine; a different article landing on the same slug ("C" and
// "C++" both slug to "c") is refused rather than silently clobbered.
func WriteFixture(dir string, a *Article) (string, error) {
	slug := fixtureSlug(a)
	metaPath := filepath.Join(dir, slug+".json")
	if existing, err := os.ReadFile(metaPath); err == nil {
		// An existing fixture that cannot be verified as the same article —
		// unparseable included — is never clobbered.
		var old Article
		if err := json.Unmarshal(existing, &old); err != nil {
			return "", fmt.Errorf("fixture %s exists but its metadata does not parse (%v); refusing to overwrite it", slug, err)
		}
		if old.Title != a.Title || old.Source != a.Source {
			return "", fmt.Errorf("fixture %s already holds %s article %q; refusing to overwrite it with %q", slug, old.Source, old.Title, a.Title)
		}
	}
	meta, err := json.MarshalIndent(a, "", " ")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	// Text first: if the second write fails, an orphan .txt is inert
	// (Fetch finds fixtures by their .json), while an orphan .json would be
	// a fixture with no text.
	if err := os.WriteFile(filepath.Join(dir, slug+".txt"), []byte(a.Text), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(metaPath, append(meta, '\n'), 0o644); err != nil {
		return "", err
	}
	return slug, nil
}

func fixtureSlug(a *Article) string {
	if a.Source == "wikipedia" {
		return Slug(a.Title)
	}
	return Slug(a.Source + " " + a.Title)
}

// Slug turns a title into a fixture file name: lowercased, with runs of
// anything but letters and digits collapsed to single hyphens.
func Slug(title string) string {
	var b strings.Builder
	hyphen := false
	for _, r := range strings.ToLower(title) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if hyphen && b.Len() > 0 {
				b.WriteByte('-')
			}
			hyphen = false
			b.WriteRune(r)
		default:
			hyphen = true
		}
	}
	return b.String()
}
