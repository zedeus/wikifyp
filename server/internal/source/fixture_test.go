package source

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestFixtureRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := validArticle()
	want.PageID = 6761
	want.Links = []string{"Alchemy", "Atom"}
	want.Categories = []string{"Chemistry", "Physical sciences"}

	slug, err := WriteFixture(dir, &want)
	if err != nil {
		t.Fatal(err)
	}
	if slug != "chemistry" {
		t.Errorf("slug = %q, want %q", slug, "chemistry")
	}

	f := &Fixtures{Dir: dir}
	got, err := f.Fetch(context.Background(), "Chemistry")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("round trip changed the article:\n got %+v\nwant %+v", got, want)
	}
}

func TestFixtureSlugPrefixesOtherSources(t *testing.T) {
	dir := t.TempDir()
	a := validArticle()
	a.Source = "simplewiki"

	slug, err := WriteFixture(dir, &a)
	if err != nil {
		t.Fatal(err)
	}
	if slug != "simplewiki-chemistry" {
		t.Fatalf("slug = %q, want simplewiki-chemistry", slug)
	}

	// Both the simplewiki fixture and a wikipedia fixture of the same topic
	// can now live in one directory.
	w := validArticle()
	if _, err := WriteFixture(dir, &w); err != nil {
		t.Fatal(err)
	}

	f := &Fixtures{Dir: dir}
	simple, err := f.Fetch(context.Background(), "simplewiki-chemistry")
	if err != nil {
		t.Fatal(err)
	}
	if simple.Source != "simplewiki" {
		t.Errorf("fetched source = %q, want simplewiki", simple.Source)
	}
	en, err := f.Fetch(context.Background(), "Chemistry")
	if err != nil {
		t.Fatal(err)
	}
	if en.Source != "wikipedia" {
		t.Errorf("fetched source = %q, want wikipedia", en.Source)
	}
}

// TestFixtureSlugCollisionRefused pins that a different article landing on
// an existing fixture's slug is an error, not a silent overwrite: "C" and
// "C++" both slug to "c".
func TestFixtureSlugCollisionRefused(t *testing.T) {
	dir := t.TempDir()
	c := validArticle()
	c.Title = "C"
	c.Text = "C is a language."
	if _, err := WriteFixture(dir, &c); err != nil {
		t.Fatal(err)
	}

	cpp := validArticle()
	cpp.Title = "C++"
	cpp.Text = "C++ is another language."
	if _, err := WriteFixture(dir, &cpp); err == nil {
		t.Fatal("colliding fixture silently overwrote a different article")
	}

	f := &Fixtures{Dir: dir}
	got, err := f.Fetch(context.Background(), "C")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "C" || got.Text != "C is a language." {
		t.Errorf("original fixture damaged: %+v", got)
	}

	// Re-writing the same article (say a newer revision) stays allowed.
	c.RevisionID++
	if _, err := WriteFixture(dir, &c); err != nil {
		t.Errorf("re-writing the same article refused: %v", err)
	}
}

func TestFixtureNotFound(t *testing.T) {
	f := &Fixtures{Dir: t.TempDir()}
	_, err := f.Fetch(context.Background(), "No Such Article")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"Chemistry", "chemistry"},
		{"Barack Obama", "barack-obama"},
		{"Computer (disambiguation)", "computer-disambiguation"},
		{"Kurt Gödel", "kurt-gödel"},
		{"C++", "c"},
		{"  Spaced  Out  ", "spaced-out"},
		{"History of science", "history-of-science"},
		{"Æthelred the Unready", "æthelred-the-unready"},
	}
	for _, tt := range tests {
		if got := Slug(tt.title); got != tt.want {
			t.Errorf("Slug(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestFixtureMissingTextFile(t *testing.T) {
	dir := t.TempDir()
	a := validArticle()
	if _, err := WriteFixture(dir, &a); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "chemistry.txt")); err != nil {
		t.Fatal(err)
	}
	f := &Fixtures{Dir: dir}
	_, err := f.Fetch(context.Background(), "Chemistry")
	if err == nil {
		t.Fatal("fixture with a missing text file accepted")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatal("half-present fixture reported as not found; that hides a broken fixture")
	}
}

func TestFixtureFetchRefusesCollidingTitle(t *testing.T) {
	dir := t.TempDir()
	c := validArticle()
	c.Title = "C"
	if _, err := WriteFixture(dir, &c); err != nil {
		t.Fatal(err)
	}
	f := &Fixtures{Dir: dir}
	// "C++" slugs to "c" too; the fixture there must not answer for it.
	_, err := f.Fetch(context.Background(), "C++")
	if err == nil {
		t.Fatal("fixture for \"C\" answered a fetch of \"C++\"")
	}
	if errors.Is(err, ErrNotFound) {
		t.Fatalf("collision reported as not found, hiding the mismatch: %v", err)
	}
}

func TestWriteFixtureRefusesUnparseableExisting(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "chemistry.json"), []byte("{truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := validArticle()
	if _, err := WriteFixture(dir, &a); err == nil {
		t.Fatal("unverifiable existing fixture was overwritten")
	}
}
