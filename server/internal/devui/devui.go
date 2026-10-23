// Package devui serves a development inspection UI under /dev: search the
// stored corpus, inspect an article's sentences, tokens and addresses, walk
// its link and category relationships, replay the ingest pipeline over
// pasted text or Parsoid HTML, and simulate a reader's feed with the
// ranking's working shown. It is a tool for evaluating the system, not part
// of the product API; it can also write reader state, so it is only mounted
// by the server, never exposed beyond it.
package devui

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/zedeus/wikifyp/server/internal/feed"
	"github.com/zedeus/wikifyp/server/internal/ingest"
	"github.com/zedeus/wikifyp/server/internal/source"
)

// New returns the /dev handler. fixturesDir, when it exists, lets article
// pages show the raw fixture extract next to what ingest stored; pass "" to
// probe the default locations.
func New(db *sql.DB, fixturesDir string) http.Handler {
	s := &server{db: db, fixtures: findFixtures(fixturesDir)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /dev", s.handleHome)
	mux.HandleFunc("GET /dev/search", s.handleSearch)
	mux.HandleFunc("GET /dev/article/{id}", s.handleArticle)
	mux.HandleFunc("GET /dev/article/{id}/raw", s.handleRaw)
	mux.HandleFunc("GET /dev/category/{name}", s.handleCategory)
	mux.HandleFunc("GET /dev/feed", s.handleFeed)
	mux.HandleFunc("POST /dev/state", s.handleState)
	mux.HandleFunc("GET /dev/pipeline", s.handlePipeline)
	mux.HandleFunc("POST /dev/pipeline", s.handlePipeline)
	return mux
}

// findFixtures resolves the fixtures directory: the explicit argument, the
// WIKIFYP_FIXTURES variable, or the repo-layout defaults relative to where
// the server usually runs. Empty means the raw-extract view stays hidden.
func findFixtures(dir string) string {
	candidates := []string{dir, os.Getenv("WIKIFYP_FIXTURES"), "fixtures/articles", "../fixtures/articles"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

type server struct {
	db       *sql.DB
	fixtures string // fixtures/articles directory, "" when unavailable
}

// render writes one page. Each page template extends the shared layout.
func (s *server) render(w http.ResponseWriter, page string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, page, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *server) fail(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// article is the metadata row the pages share.
type article struct {
	ID          int64
	Title       string
	Source      string
	PageID      int64
	RevisionID  int64
	URL         string
	License     string
	Attribution string
	Fetched     string
}

func (s *server) loadArticle(ctx context.Context, id int64) (*article, error) {
	var a article
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, source, page_id, revision_id, url, license, attribution, fetched_at
		FROM articles WHERE id = $1`, id).
		Scan(&a.ID, &a.Title, &a.Source, &a.PageID, &a.RevisionID, &a.URL, &a.License, &a.Attribution, &a.Fetched)
	if err != nil {
		return nil, err
	}
	a.Fetched = strings.TrimSuffix(strings.Split(a.Fetched, "T")[0], " 00:00:00 +0000 +0000")
	return &a, nil
}

// --- home ---

type statRow struct {
	Source    string
	Articles  int
	Revisions int
}

type catCount struct {
	Category string
	Count    int
}

func (s *server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	data := struct {
		Stats                  []statRow
		Sentences, Links, Cats int
		TopCategories          []catCount
		FixturesDir            string
	}{FixturesDir: s.fixtures}

	rows, err := s.db.QueryContext(ctx, `
		SELECT l.source, count(*), (SELECT count(*) FROM articles a WHERE a.source = l.source)
		FROM latest_articles l GROUP BY l.source ORDER BY l.source`)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var row statRow
		if err := rows.Scan(&row.Source, &row.Articles, &row.Revisions); err != nil {
			s.fail(w, err)
			return
		}
		data.Stats = append(data.Stats, row)
	}
	if err := rows.Err(); err != nil {
		s.fail(w, err)
		return
	}
	for _, q := range []struct {
		query string
		into  *int
	}{
		{"SELECT count(*) FROM sentences", &data.Sentences},
		{"SELECT count(*) FROM article_links", &data.Links},
		{"SELECT count(*) FROM article_categories", &data.Cats},
	} {
		if err := s.db.QueryRowContext(ctx, q.query).Scan(q.into); err != nil {
			s.fail(w, err)
			return
		}
	}
	catRows, err := s.db.QueryContext(ctx, `
		SELECT category, count(*) FROM article_categories
		GROUP BY category ORDER BY count(*) DESC, category LIMIT 30`)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer catRows.Close()
	for catRows.Next() {
		var c catCount
		if err := catRows.Scan(&c.Category, &c.Count); err != nil {
			s.fail(w, err)
			return
		}
		data.TopCategories = append(data.TopCategories, c)
	}
	if err := catRows.Err(); err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, "home", data)
}

// --- search ---

type hit struct {
	ID         int64
	Title      string
	Source     string
	RevisionID int64
	Sentences  int
}

func (s *server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.FormValue("q"))
	data := struct {
		Query string
		Hits  []hit
	}{Query: q}
	if q != "" {
		// Earliest and shortest match first: a search for "japan" puts the
		// article "Japan" above "History of Japan" above "Sea of Japan".
		rows, err := s.db.QueryContext(r.Context(), `
			SELECT a.id, a.title, a.source, a.revision_id,
			       (SELECT count(*) FROM sentences WHERE article_id = a.id)
			FROM latest_articles a
			WHERE a.title ILIKE '%' || $1 || '%'
			ORDER BY position(lower($1) in lower(a.title)), length(a.title), a.title, a.source
			LIMIT 50`, q)
		if err != nil {
			s.fail(w, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var h hit
			if err := rows.Scan(&h.ID, &h.Title, &h.Source, &h.RevisionID, &h.Sentences); err != nil {
				s.fail(w, err)
				return
			}
			data.Hits = append(data.Hits, h)
		}
		if err := rows.Err(); err != nil {
			s.fail(w, err)
			return
		}
	}
	s.render(w, "search", data)
}

// --- article inspector ---

type sentenceView struct {
	Idx     int
	Heading int
	Text    string
	Tokens  []string
}

type linkOut struct {
	Target    string
	ArticleID int64 // 0: the target is not in the corpus
}

// stateForm feeds the statebuttons template partial.
type stateForm struct {
	Reader, Source, Back string
	PageID               int64
}

type articlePage struct {
	A           *article
	Sentences   []sentenceView
	Words       int
	ShowTokens  bool
	LinksOut    []linkOut
	LinksIn     []hit
	LinksInMore bool
	Categories  []string
	Reader      string
	State       string // this reader's state for the page, if any
	HasFixture  bool
}

// StateForm is the article page's set-state form data.
func (p articlePage) StateForm() stateForm {
	return stateForm{
		Reader: p.Reader,
		Source: p.A.Source,
		PageID: p.A.PageID,
		Back:   fmt.Sprintf("/dev/article/%d?reader=%s", p.A.ID, url.QueryEscape(p.Reader)),
	}
}

func (s *server) handleArticle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad article id", http.StatusBadRequest)
		return
	}
	a, err := s.loadArticle(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	page := articlePage{
		A:          a,
		ShowTokens: r.FormValue("tokens") == "1",
		Reader:     readerOf(r),
		HasFixture: s.fixtureFor(a) != "",
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT sentence_idx, heading_level, text FROM sentences WHERE article_id = $1 ORDER BY sentence_idx", id)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var sv sentenceView
		if err := rows.Scan(&sv.Idx, &sv.Heading, &sv.Text); err != nil {
			s.fail(w, err)
			return
		}
		// Tokens are derived, never stored: the token list IS the stored
		// text split on single spaces, and token_idx indexes that split.
		sv.Tokens = strings.Split(sv.Text, " ")
		page.Words += len(sv.Tokens)
		page.Sentences = append(page.Sentences, sv)
	}
	if err := rows.Err(); err != nil {
		s.fail(w, err)
		return
	}

	out, err := s.db.QueryContext(ctx, `
		SELECT l.target, coalesce(t.id, 0)
		FROM article_links l
		LEFT JOIN latest_articles t ON t.source = $2 AND t.title = l.target
		WHERE l.article_id = $1
		ORDER BY l.target`, id, a.Source)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer out.Close()
	for out.Next() {
		var l linkOut
		if err := out.Scan(&l.Target, &l.ArticleID); err != nil {
			s.fail(w, err)
			return
		}
		page.LinksOut = append(page.LinksOut, l)
	}
	if err := out.Err(); err != nil {
		s.fail(w, err)
		return
	}

	const maxIn = 200
	in, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.title, a.source FROM latest_articles a
		JOIN article_links l ON l.article_id = a.id
		WHERE a.source = $1 AND l.target = $2
		ORDER BY a.title LIMIT $3`, a.Source, a.Title, maxIn+1)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer in.Close()
	for in.Next() {
		var h hit
		if err := in.Scan(&h.ID, &h.Title, &h.Source); err != nil {
			s.fail(w, err)
			return
		}
		page.LinksIn = append(page.LinksIn, h)
	}
	if err := in.Err(); err != nil {
		s.fail(w, err)
		return
	}
	if len(page.LinksIn) > maxIn {
		page.LinksIn, page.LinksInMore = page.LinksIn[:maxIn], true
	}

	cats, err := s.db.QueryContext(ctx,
		"SELECT category FROM article_categories WHERE article_id = $1 ORDER BY category", id)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer cats.Close()
	for cats.Next() {
		var c string
		if err := cats.Scan(&c); err != nil {
			s.fail(w, err)
			return
		}
		page.Categories = append(page.Categories, c)
	}
	if err := cats.Err(); err != nil {
		s.fail(w, err)
		return
	}

	err = s.db.QueryRowContext(ctx,
		"SELECT state FROM reader_article_state WHERE reader_id = $1 AND source = $2 AND page_id = $3",
		page.Reader, a.Source, a.PageID).Scan(&page.State)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.fail(w, err)
		return
	}
	s.render(w, "article", page)
}

// fixtureFor returns the slug of the committed fixture holding this article's
// title for its source, or "".
func (s *server) fixtureFor(a *article) string {
	if s.fixtures == "" {
		return ""
	}
	src := &source.Fixtures{Dir: s.fixtures}
	// A Wikipedia fixture is named by its bare title; other sources prefix
	// their name to the slug ("simplewiki-chemistry").
	for _, name := range []string{a.Title, a.Source + "-" + source.Slug(a.Title)} {
		f, err := src.Fetch(context.Background(), name)
		if err == nil && f.Source == a.Source {
			return name
		}
	}
	return ""
}

// --- raw fixture vs stored pipeline output ---

func (s *server) handleRaw(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad article id", http.StatusBadRequest)
		return
	}
	a, err := s.loadArticle(r.Context(), id)
	if err != nil {
		s.fail(w, err)
		return
	}
	slug := s.fixtureFor(a)
	if slug == "" {
		http.Error(w, "no committed fixture holds this article", http.StatusNotFound)
		return
	}
	f, err := (&source.Fixtures{Dir: s.fixtures}).Fetch(r.Context(), slug)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, "raw", struct {
		A       *article
		Fixture *source.Article
		Cleaned string
		SameRev bool
	}{a, f, source.Clean(f.Text), f.RevisionID == a.RevisionID})
}

// --- category ---

func (s *server) handleCategory(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT a.id, a.title, a.source, a.revision_id,
		       (SELECT count(*) FROM sentences WHERE article_id = a.id)
		FROM latest_articles a
		JOIN article_categories c ON c.article_id = a.id
		WHERE c.category = $1
		ORDER BY a.title LIMIT 500`, name)
	if err != nil {
		s.fail(w, err)
		return
	}
	defer rows.Close()
	data := struct {
		Name string
		Hits []hit
	}{Name: name}
	for rows.Next() {
		var h hit
		if err := rows.Scan(&h.ID, &h.Title, &h.Source, &h.RevisionID, &h.Sentences); err != nil {
			s.fail(w, err)
			return
		}
		data.Hits = append(data.Hits, h)
	}
	if err := rows.Err(); err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, "category", data)
}

// --- feed simulator ---

type statedArticle struct {
	feed.StateArticle
	ArticleID int64
}

func (s *server) handleFeed(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	reader := readerOf(r)
	type weighted struct {
		Category string
		Weight   float64
	}
	data := struct {
		Reader   string
		Cards    []feed.Card
		State    []statedArticle
		Ranked   []feed.Scored
		Interest []weighted
	}{Reader: reader}
	if reader != "" {
		// One pipeline run serves both the feed and the working view.
		cards, state, weights, ranked, err := feed.ForExplained(ctx, s.db, reader, 20)
		if err != nil {
			s.fail(w, err)
			return
		}
		data.Cards = cards
		if len(ranked) > 100 {
			ranked = ranked[:100]
		}
		data.Ranked = ranked
		for c, wt := range weights {
			data.Interest = append(data.Interest, weighted{c, wt})
		}
		slices.SortFunc(data.Interest, func(a, b weighted) int {
			if a.Weight != b.Weight {
				return cmp.Compare(b.Weight, a.Weight)
			}
			return cmp.Compare(a.Category, b.Category)
		})
		if len(data.Interest) > 25 {
			data.Interest = data.Interest[:25]
		}
		// Resolve the pages' latest revisions in one query so the titles
		// link back to the inspector; missing ids are cosmetic.
		srcs := make([]string, len(state))
		pids := make([]int64, len(state))
		for i, st := range state {
			srcs[i], pids[i] = st.Source, st.PageID
		}
		ids := make(map[[2]any]int64, len(state))
		if rows, err := s.db.QueryContext(ctx, `
			SELECT a.source, a.page_id, a.id FROM latest_articles a
			JOIN unnest($1::text[], $2::bigint[]) AS f (source, page_id)
			  ON a.source = f.source AND a.page_id = f.page_id`, srcs, pids); err == nil {
			for rows.Next() {
				var src string
				var pid, id int64
				if rows.Scan(&src, &pid, &id) == nil {
					ids[[2]any{src, pid}] = id
				}
			}
			rows.Close()
		}
		for _, st := range state {
			sa := statedArticle{StateArticle: st}
			sa.ArticleID = ids[[2]any{st.Source, st.PageID}]
			data.State = append(data.State, sa)
		}
	}
	s.render(w, "feed", data)
}

// handleState writes (or clears) a reader's state for a page and bounces
// back to where the button was pressed.
func (s *server) handleState(w http.ResponseWriter, r *http.Request) {
	reader := strings.TrimSpace(r.FormValue("reader"))
	src := r.FormValue("source")
	pageID, err := strconv.ParseInt(r.FormValue("page_id"), 10, 64)
	state := r.FormValue("state")
	if reader == "" || src == "" || err != nil {
		http.Error(w, "state needs reader, source and page_id", http.StatusBadRequest)
		return
	}
	if state == "clear" {
		// Dev-only escape hatch; the product never deletes state.
		_, err = s.db.ExecContext(r.Context(),
			"DELETE FROM reader_article_state WHERE reader_id = $1 AND source = $2 AND page_id = $3",
			reader, src, pageID)
	} else {
		err = feed.SetState(r.Context(), s.db, reader, src, pageID, state)
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	back := r.FormValue("back")
	if back == "" || !strings.HasPrefix(back, "/dev") {
		back = "/dev/feed?reader=" + url.QueryEscape(reader)
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// --- pipeline playground ---

type pipelinePage struct {
	Input      string
	Mode       string // "extract" or "parsoid"
	Ran        bool
	Extracted  string // parsoid mode: the converted extract-shaped text
	Links      []string
	Redirect   bool
	Cleaned    string
	Rows       []ingest.SentenceRow
	ShowTokens bool
	Error      string
}

func (s *server) handlePipeline(w http.ResponseWriter, r *http.Request) {
	page := pipelinePage{
		Input:      r.FormValue("input"),
		Mode:       r.FormValue("mode"),
		ShowTokens: r.FormValue("tokens") == "1",
	}
	if page.Mode == "" {
		page.Mode = "extract"
	}
	if r.Method == http.MethodPost && strings.TrimSpace(page.Input) != "" {
		page.Ran = true
		text := page.Input
		if page.Mode == "parsoid" {
			parsed, err := source.ParseParsoid(strings.NewReader(page.Input))
			if err != nil {
				page.Error = err.Error()
				s.render(w, "pipeline", page)
				return
			}
			text, page.Extracted, page.Links, page.Redirect = parsed.Text, parsed.Text, parsed.Links, parsed.Redirect
		}
		page.Cleaned = source.Clean(text)
		page.Rows = ingest.SentenceRows(page.Cleaned)
	}
	s.render(w, "pipeline", page)
}

// readerOf is the simulated reader id, defaulting so every page's state
// buttons work without typing one first.
func readerOf(r *http.Request) string {
	if v := strings.TrimSpace(r.FormValue("reader")); v != "" {
		return v
	}
	return "dev"
}

var tmpl = template.Must(template.New("dev").Funcs(template.FuncMap{
	"tokens": func(s string) []string { return strings.Split(s, " ") },
	"sub":    func(a, b int) int { return a - b },
	// pathescape keeps category names containing "/" (or "?", "#") inside
	// one path segment; html/template's URL normalizer does not escape
	// slashes, and the /dev/category/{name} route matches a single segment.
	"pathescape": url.PathEscape,
}).Parse(pages))
