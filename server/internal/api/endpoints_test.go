package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/zedeus/wikifyp/server/internal/feed"
	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/migrate"
	"github.com/zedeus/wikifyp/server/internal/source"
	"github.com/zedeus/wikifyp/server/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

// newHandler returns the API over a migrated test database loaded with two
// fixture articles and the dump sample (which brings one featured article).
func newHandler(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db := testdb.New(t)
	if err := migrate.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	src := &source.Fixtures{Dir: "../../../fixtures/articles"}
	for _, slug := range []string{"philosophy", "metaphysics"} {
		a, err := src.Fetch(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ingest.Article(ctx, db, a); err != nil {
			t.Fatal(err)
		}
	}
	f, err := os.Open("../../../fixtures/dumps/simplewiki-sample.json.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, _, err := source.ReadDump(ctx, f, time.Now(), func(a *source.Article) error {
		_, err := ingest.Article(ctx, db, a)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return New(db), db
}

func get(t *testing.T, h http.Handler, url string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

type feedResponse struct {
	Articles []struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Source      string `json:"source"`
		Excerpt     string `json:"excerpt"`
		Sentences   int    `json:"sentences"`
		Words       int    `json:"words"`
		Attribution string `json:"attribution"`
	} `json:"articles"`
}

func TestFeedEndpoint(t *testing.T) {
	h, db := newHandler(t)

	// A new reader gets the featured article.
	rec := get(t, h, "/v1/feed?reader=r-new")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/feed = %d: %s", rec.Code, rec.Body)
	}
	var resp feedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Articles) != 1 || resp.Articles[0].Title != "Red Hot Chili Peppers" {
		t.Fatalf("new reader feed = %+v, want the featured article", resp.Articles)
	}
	card := resp.Articles[0]
	if card.ID == 0 || card.Source != "simplewiki" || card.Excerpt == "" ||
		card.Sentences == 0 || card.Words == 0 || card.Attribution == "" {
		t.Errorf("incomplete card: %+v", card)
	}

	// Finishing Philosophy brings its neighbour and drops Philosophy.
	philosophy, err := (&source.Fixtures{Dir: "../../../fixtures/articles"}).Fetch(context.Background(), "philosophy")
	if err != nil {
		t.Fatal(err)
	}
	if err := feed.SetState(context.Background(), db, "r1", philosophy.Source, philosophy.PageID, feed.StateFinished); err != nil {
		t.Fatal(err)
	}
	rec = get(t, h, "/v1/feed?reader=r1")
	resp = feedResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	var sawMetaphysics bool
	for _, a := range resp.Articles {
		if a.Title == "Philosophy" {
			t.Error("finished Philosophy served")
		}
		if a.Title == "Metaphysics" {
			sawMetaphysics = true
		}
	}
	if !sawMetaphysics {
		t.Errorf("neighbour Metaphysics missing: %+v", resp.Articles)
	}

	// The limit caps the response.
	rec = get(t, h, "/v1/feed?reader=r1&limit=1")
	resp = feedResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Articles) != 1 {
		t.Errorf("limit=1 returned %d articles", len(resp.Articles))
	}

	// Parameter validation.
	for url, want := range map[string]int{
		"/v1/feed":                     http.StatusBadRequest, // no reader
		"/v1/feed?reader=r1&limit=0":   http.StatusBadRequest,
		"/v1/feed?reader=r1&limit=abc": http.StatusBadRequest,
	} {
		if rec := get(t, h, url); rec.Code != want {
			t.Errorf("GET %s = %d, want %d", url, rec.Code, want)
		}
	}
}

func TestArticleEndpoint(t *testing.T) {
	h, db := newHandler(t)
	var id int64
	var title string
	err := db.QueryRowContext(context.Background(),
		"SELECT id, title FROM articles WHERE title = 'Fossil word'").Scan(&id, &title)
	if err != nil {
		t.Fatal(err)
	}

	rec := get(t, h, "/v1/articles/"+strconv.FormatInt(id, 10))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/articles/%d = %d: %s", id, rec.Code, rec.Body)
	}
	var resp struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Attribution string `json:"attribution"`
		Sentences   []struct {
			Text         string `json:"text"`
			HeadingLevel int    `json:"heading_level"`
		} `json:"sentences"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.ID != id || resp.Title != "Fossil word" || resp.Attribution == "" {
		t.Errorf("article header = %+v", resp)
	}
	// The stored rows come back in address order with heading levels: the
	// cleaned Fossil word is its intro sentence, the Examples heading and
	// the four list items (fixtures/dumps/fossil-word.expected.txt, minus
	// the stripped References section).
	if len(resp.Sentences) != 6 {
		t.Fatalf("%d sentences, want 6: %+v", len(resp.Sentences), resp.Sentences)
	}
	if resp.Sentences[0].HeadingLevel != 0 || resp.Sentences[1].HeadingLevel != 2 {
		t.Errorf("heading levels = %d, %d; want 0, 2", resp.Sentences[0].HeadingLevel, resp.Sentences[1].HeadingLevel)
	}
	if resp.Sentences[1].Text != "Examples" {
		t.Errorf("sentence 1 = %q, want the Examples heading", resp.Sentences[1].Text)
	}

	// Unknown and malformed ids.
	if rec := get(t, h, "/v1/articles/999999"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown article = %d, want 404", rec.Code)
	}
	if rec := get(t, h, "/v1/articles/xyz"); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed id = %d, want 400", rec.Code)
	}
}

// Without a database the data endpoints answer 503, not a panic.
func TestEndpointsWithoutDatabase(t *testing.T) {
	h := New(nil)
	for _, url := range []string{"/v1/feed?reader=r1", "/v1/articles/1"} {
		if rec := get(t, h, url); rec.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s without db = %d, want 503", url, rec.Code)
		}
	}
}

// A limit above the cap is clamped, not rejected: the client stays simple.
func TestFeedLimitClamped(t *testing.T) {
	h, _ := newHandler(t)
	rec := get(t, h, "/v1/feed?reader=r1&limit=9999")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET with huge limit = %d: %s", rec.Code, rec.Body)
	}
	var resp feedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Articles) > 50 {
		t.Errorf("limit not clamped: %d articles", len(resp.Articles))
	}
}
