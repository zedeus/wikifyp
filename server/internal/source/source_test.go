package source

import (
	"strings"
	"testing"
)

func validArticle() Article {
	return Article{
		Source:      "wikipedia",
		Title:       "Chemistry",
		URL:         "https://en.wikipedia.org/wiki/Chemistry",
		RevisionID:  1375507113,
		License:     "CC BY-SA 4.0",
		Attribution: `Text from the Wikipedia article "Chemistry"`,
		Fetched:     "2026-10-12",
		Text:        "Chemistry is the scientific study of matter.",
	}
}

func TestValidate(t *testing.T) {
	a := validArticle()
	if err := a.Validate(); err != nil {
		t.Fatalf("valid article rejected: %v", err)
	}

	tests := []struct {
		field   string
		corrupt func(*Article)
	}{
		{"source", func(a *Article) { a.Source = "" }},
		{"title", func(a *Article) { a.Title = "" }},
		{"url", func(a *Article) { a.URL = "" }},
		{"revision_id", func(a *Article) { a.RevisionID = 0 }},
		{"license", func(a *Article) { a.License = "" }},
		{"attribution", func(a *Article) { a.Attribution = "" }},
		{"fetched", func(a *Article) { a.Fetched = "" }},
		{"fetched", func(a *Article) { a.Fetched = "yesterday" }},
		{"text", func(a *Article) { a.Text = "" }},
		{"text", func(a *Article) { a.Text = "  \n\t " }},
	}
	for _, tt := range tests {
		a := validArticle()
		tt.corrupt(&a)
		err := a.Validate()
		if err == nil {
			t.Errorf("article with bad %s accepted", tt.field)
			continue
		}
		if !strings.Contains(err.Error(), tt.field) {
			t.Errorf("error for bad %s does not name it: %v", tt.field, err)
		}
	}

	// All failures are reported at once, not one per attempt.
	empty := Article{}
	err := empty.Validate()
	if err == nil {
		t.Fatal("empty article accepted")
	}
	for _, field := range []string{"source", "title", "url", "revision_id", "license", "attribution", "fetched", "text"} {
		if !strings.Contains(err.Error(), field) {
			t.Errorf("empty-article error missing %s: %v", field, err)
		}
	}
}
