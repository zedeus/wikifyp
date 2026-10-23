package devui

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/migrate"
	"github.com/zedeus/wikifyp/server/internal/source"
	"github.com/zedeus/wikifyp/server/internal/testdb"
)

func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }

const fixturesDir = "../../../fixtures/articles"

// newUI returns the dev handler over a test database holding two fixture
// articles and the dump sample.
func newUI(t *testing.T) (http.Handler, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	db := testdb.New(t)
	if err := migrate.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	src := &source.Fixtures{Dir: fixturesDir}
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
	return New(db, fixturesDir), db
}

func get(t *testing.T, h http.Handler, target string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", target, rec.Code, rec.Body)
	}
	return rec.Body.String()
}

func articleID(t *testing.T, db *sql.DB, title string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRowContext(context.Background(),
		"SELECT id FROM articles WHERE title = $1", title).Scan(&id); err != nil {
		t.Fatalf("id of %q: %v", title, err)
	}
	return id
}

func TestPages(t *testing.T) {
	h, db := newUI(t)

	home := get(t, h, "/dev")
	for _, want := range []string{"wikipedia", "simplewiki", "Largest categories"} {
		if !strings.Contains(home, want) {
			t.Errorf("home missing %q", want)
		}
	}

	search := get(t, h, "/dev/search?q=philo")
	if !strings.Contains(search, "Philosophy") {
		t.Errorf("search for philo missing Philosophy: %.300s", search)
	}

	id := articleID(t, db, "Philosophy")
	target := "/dev/article/" + strconv.FormatInt(id, 10)
	article := get(t, h, target)
	for _, want := range []string{
		"Philosophy", "Links out", "Links in", "Categories",
		"raw fixture vs stored", // the committed fixture is found
		"Metaphysics",           // linked article resolved in the corpus
	} {
		if !strings.Contains(article, want) {
			t.Errorf("article page missing %q", want)
		}
	}
	tokens := get(t, h, target+"?tokens=1")
	if !strings.Contains(tokens, `<sub>0</sub>`) {
		t.Error("token view shows no token indices")
	}

	raw := get(t, h, target+"/raw")
	for _, want := range []string{"Raw extract", "After Clean"} {
		if !strings.Contains(raw, want) {
			t.Errorf("raw page missing %q", want)
		}
	}
	// A dump-imported article has no committed fixture.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/dev/article/"+strconv.FormatInt(articleID(t, db, "Microeconomics"), 10)+"/raw", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("raw page for a dump article = %d, want 404", rec.Code)
	}

	category := get(t, h, "/dev/category/"+url.PathEscape("Very good articles"))
	if !strings.Contains(category, "Red Hot Chili Peppers") {
		t.Errorf("category page missing its member: %.300s", category)
	}

	pipeline := get(t, h, "/dev/pipeline")
	if !strings.Contains(pipeline, "Pipeline playground") {
		t.Error("pipeline form missing")
	}
}

func TestFeedSimulatorFlow(t *testing.T) {
	h, db := newUI(t)

	// A reader with no state: featured only, no ranking table.
	page := get(t, h, "/dev/feed?reader=sim")
	if !strings.Contains(page, "Red Hot Chili Peppers") {
		t.Errorf("new reader feed missing the featured article")
	}
	if strings.Contains(page, "Ranking’s working") {
		t.Error("ranking table shown with no state")
	}

	// Finish Philosophy through the state endpoint.
	var pageID int64
	if err := db.QueryRowContext(context.Background(),
		"SELECT page_id FROM articles WHERE title = 'Philosophy'").Scan(&pageID); err != nil {
		t.Fatal(err)
	}
	form := url.Values{
		"reader": {"sim"}, "source": {"wikipedia"},
		"page_id": {strconv.FormatInt(pageID, 10)}, "state": {"finished"},
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/dev/state", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /dev/state = %d: %s", rec.Code, rec.Body)
	}

	page = get(t, h, "/dev/feed?reader=sim")
	for _, want := range []string{"finished", "Ranking’s working", "Metaphysics"} {
		if !strings.Contains(page, want) {
			t.Errorf("feed page after finishing missing %q", want)
		}
	}

	// Clear it again; the ranking table disappears.
	form.Set("state", "clear")
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/dev/state", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("POST /dev/state clear = %d", rec.Code)
	}
	if page = get(t, h, "/dev/feed?reader=sim"); strings.Contains(page, "Ranking’s working") {
		t.Error("state not cleared")
	}
}

func TestPipelineRuns(t *testing.T) {
	h, _ := newUI(t)
	post := func(form url.Values) string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/dev/pipeline", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST /dev/pipeline = %d: %s", rec.Code, rec.Body)
		}
		return rec.Body.String()
	}

	out := post(url.Values{"mode": {"extract"}, "tokens": {"1"}, "input": {
		"Dr. Smith went home.\n\n== References ==\nGone.",
	}})
	for _, want := range []string{"Dr.", "<sub>0</sub>", "Sentence rows (1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("extract run missing %q", want)
		}
	}
	if strings.Contains(out, "Gone.") && strings.Contains(out, "Sentence rows (1)") == false {
		t.Error("references section not stripped")
	}

	out = post(url.Values{"mode": {"parsoid"}, "input": {
		`<html><body><p>An <a rel="mw:WikiLink" href="./Idiom" title="Idiom">idiom</a>.</p></body></html>`,
	}})
	for _, want := range []string{"An idiom.", "Idiom", "extract-shaped text"} {
		if !strings.Contains(out, want) {
			t.Errorf("parsoid run missing %q", want)
		}
	}
}
