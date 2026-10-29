package source

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// serveRecorded serves the recorded Photosynthesis API responses: the first
// request gets page one, and the two continuation requests are routed by the
// plcontinue value they carry, exactly as the live API chained them.
func serveRecorded(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var agents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agents = append(agents, r.Header.Get("User-Agent"))
		var file string
		switch cont := r.URL.Query().Get("plcontinue"); cont {
		case "":
			file = "photosynthesis-1.json"
		case "24544|0|History_of_plant_systematics":
			file = "photosynthesis-2.json"
		case "24544|0|Staminode":
			file = "photosynthesis-3.json"
		default:
			t.Errorf("unexpected plcontinue %q", cont)
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join("testdata", file))
	}))
	t.Cleanup(srv.Close)
	return srv, &agents
}

// testClient points a wiki client at a test server and disables its rate
// limit.
func testClient(w *MediaWiki, api string) *MediaWiki {
	w.API = api
	w.Interval = 0
	return w
}

func TestMediaWikiFetch(t *testing.T) {
	srv, agents := serveRecorded(t)
	a, err := testClient(Wikipedia(), srv.URL).Fetch(context.Background(), "Photosynthesis")
	if err != nil {
		t.Fatal(err)
	}

	if a.Source != "wikipedia" {
		t.Errorf("source = %q", a.Source)
	}
	if a.Title != "Photosynthesis" {
		t.Errorf("title = %q", a.Title)
	}
	if a.PageID != 24544 {
		t.Errorf("page id = %d", a.PageID)
	}
	if a.RevisionID != 1379238357 {
		t.Errorf("revision = %d", a.RevisionID)
	}
	if a.URL != "https://en.wikipedia.org/wiki/Photosynthesis" {
		t.Errorf("url = %q", a.URL)
	}
	if a.License != "CC BY-SA 4.0" {
		t.Errorf("license = %q", a.License)
	}
	if !strings.HasPrefix(a.Text, "Photosynthesis is a system of biological processes") {
		t.Errorf("extract starts %q", firstLine(a.Text))
	}

	// Links accumulate across all three recorded pages: 500 + 500 + 109.
	if len(a.Links) != 1109 {
		t.Errorf("links = %d, want 1109", len(a.Links))
	}
	if a.Links[0] != "3-phosphoglycerate" {
		t.Errorf("first link = %q", a.Links[0])
	}
	// Categories arrive complete on the first page, "Category:" stripped.
	if len(a.Categories) != 10 {
		t.Errorf("categories = %d, want 10", len(a.Categories))
	}
	if len(a.Categories) > 0 && a.Categories[0] != "Agronomy" {
		t.Errorf("first category = %q", a.Categories[0])
	}

	for _, want := range []string{`"Photosynthesis"`, "revision 1379238357", "CC BY-SA 4.0", a.URL, "Wikipedia"} {
		if !strings.Contains(a.Attribution, want) {
			t.Errorf("attribution %q missing %q", a.Attribution, want)
		}
	}
	if err := a.Validate(); err != nil {
		t.Errorf("fetched article invalid: %v", err)
	}

	if len(*agents) != 3 {
		t.Fatalf("requests = %d, want 3", len(*agents))
	}
	for _, ua := range *agents {
		if !strings.Contains(ua, "wikifyp") {
			t.Errorf("User-Agent = %q, want a descriptive one", ua)
		}
	}
}

// TestMediaWikiFetchDropsFinishedContinueParams covers modules finishing at
// different times: categories complete after page two, so page three's
// request must not carry the stale clcontinue — re-sending it would make the
// API return those categories again and duplicate them.
func TestMediaWikiFetchDropsFinishedContinueParams(t *testing.T) {
	page := func(cont, links, cats string) string {
		return `{` + cont + `"query":{"pages":[{"pageid":1,"title":"T",
			"extract":"One sentence.","fullurl":"https://en.wikipedia.org/wiki/T",
			"revisions":[{"revid":7}],"links":[` + links + `],"categories":[` + cats + `]}]}}`
	}
	responses := []string{
		page(`"continue":{"plcontinue":"1|0|A","clcontinue":"1|X","continue":"||extracts|revisions|info"},`,
			`{"title":"A0"}`, `{"title":"Category:C0"}`),
		page(`"continue":{"plcontinue":"1|0|B","continue":"||extracts|revisions|info|categories"},`,
			`{"title":"A1"}`, `{"title":"Category:C1"}`),
		page(``, `{"title":"A2"}`, ``),
	}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n == 2 && r.URL.Query().Has("clcontinue") {
			t.Errorf("request %d carries stale clcontinue=%q after categories finished",
				n+1, r.URL.Query().Get("clcontinue"))
		}
		if n >= len(responses) {
			t.Fatalf("request %d: more requests than responses", n+1)
		}
		w.Write([]byte(responses[n]))
		n++
	}))
	t.Cleanup(srv.Close)

	a, err := testClient(Wikipedia(), srv.URL).Fetch(context.Background(), "T")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"A0", "A1", "A2"}; !slices.Equal(a.Links, want) {
		t.Errorf("links = %q, want %q", a.Links, want)
	}
	if want := []string{"C0", "C1"}; !slices.Equal(a.Categories, want) {
		t.Errorf("categories = %q, want %q", a.Categories, want)
	}
}

// TestMediaWikiFetchDeferredProps covers the API deferring a prop module's
// data past the first response: the extract and revision arrive only on page
// two, and the continuation value is a number (as TextExtracts' excontinue
// is), which must be echoed back digit for digit.
func TestMediaWikiFetchDeferredProps(t *testing.T) {
	responses := []string{
		`{"continue":{"excontinue":1379238357,"continue":"||info"},
		  "query":{"pages":[{"pageid":1,"title":"T",
		  "fullurl":"https://en.wikipedia.org/wiki/T",
		  "links":[{"title":"A0"}]}]}}`,
		`{"query":{"pages":[{"pageid":1,"title":"T",
		  "extract":"One sentence.","fullurl":"https://en.wikipedia.org/wiki/T",
		  "revisions":[{"revid":7}]}]}}`,
	}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n == 1 {
			if got := r.URL.Query().Get("excontinue"); got != "1379238357" {
				t.Errorf("continuation request carries excontinue=%q, want 1379238357", got)
			}
		}
		if n >= len(responses) {
			t.Fatalf("request %d: more requests than responses", n+1)
		}
		w.Write([]byte(responses[n]))
		n++
	}))
	t.Cleanup(srv.Close)

	a, err := testClient(Wikipedia(), srv.URL).Fetch(context.Background(), "T")
	if err != nil {
		t.Fatal(err)
	}
	if a.Text != "One sentence." {
		t.Errorf("text = %q, want the deferred extract", a.Text)
	}
	if a.RevisionID != 7 {
		t.Errorf("revision = %d, want 7", a.RevisionID)
	}
	if err := a.Validate(); err != nil {
		t.Errorf("fetched article invalid: %v", err)
	}
}

// TestMediaWikiFetchImageAndDescription covers the pageimages and
// description props: both present, both absent, and each alone. Missing
// image or description is an answer (the page has none), never an error.
func TestMediaWikiFetchImageAndDescription(t *testing.T) {
	page := func(extra string) string {
		return `{"query":{"pages":[{"pageid":1,"title":"T",
			"extract":"One sentence.","fullurl":"https://en.wikipedia.org/wiki/T",
			"revisions":[{"revid":7}]` + extra + `}]}}`
	}
	thumb := `,"thumbnail":{"source":"https://upload.wikimedia.org/x.jpg","width":960,"height":540}`
	desc := `,"description":"Branch of science"`
	cases := []struct {
		name     string
		body     string
		wantImg  *Image
		wantDesc string
	}{
		{"both present", page(thumb + desc),
			&Image{URL: "https://upload.wikimedia.org/x.jpg", Width: 960, Height: 540}, "Branch of science"},
		{"both absent", page(""), nil, ""},
		{"image only", page(thumb),
			&Image{URL: "https://upload.wikimedia.org/x.jpg", Width: 960, Height: 540}, ""},
		{"description only", page(desc), nil, "Branch of science"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range map[string]string{"piprop": "thumbnail", "pithumbsize": "960", "pilicense": "free"} {
					if got := r.URL.Query().Get(k); got != v {
						t.Errorf("request %s = %q, want %q", k, got, v)
					}
				}
				if props := r.URL.Query().Get("prop"); !strings.Contains(props, "pageimages") || !strings.Contains(props, "description") {
					t.Errorf("request prop = %q, want pageimages and description in it", props)
				}
				w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)

			a, err := testClient(Wikipedia(), srv.URL).Fetch(context.Background(), "T")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(a.Image, tc.wantImg) {
				t.Errorf("image = %+v, want %+v", a.Image, tc.wantImg)
			}
			if a.Description != tc.wantDesc {
				t.Errorf("description = %q, want %q", a.Description, tc.wantDesc)
			}
			if err := a.Validate(); err != nil {
				t.Errorf("fetched article invalid: %v", err)
			}
		})
	}
}

// TestMediaWikiFetchMeta covers the metadata-only fetch the refresh mode
// uses: it asks for pageimages and description by page ID and nothing else.
func TestMediaWikiFetchMeta(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		wantImg  *Image
		wantDesc string
		wantErr  error
	}{
		{"both present",
			`{"query":{"pages":[{"pageid":5,"title":"T",
			  "thumbnail":{"source":"https://upload.wikimedia.org/y.png","width":800,"height":600},
			  "description":"A thing"}]}}`,
			&Image{URL: "https://upload.wikimedia.org/y.png", Width: 800, Height: 600}, "A thing", nil},
		{"neither",
			`{"query":{"pages":[{"pageid":5,"title":"T"}]}}`, nil, "", nil},
		{"missing page",
			`{"query":{"pages":[{"pageid":5,"missing":true}]}}`, nil, "", ErrNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("pageids"); got != "5" {
					t.Errorf("pageids = %q, want 5", got)
				}
				if got := r.URL.Query().Get("prop"); got != "pageimages|description" {
					t.Errorf("prop = %q, want pageimages|description", got)
				}
				w.Write([]byte(tc.body))
			}))
			t.Cleanup(srv.Close)

			img, desc, err := testClient(Wikipedia(), srv.URL).FetchMeta(context.Background(), 5)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(img, tc.wantImg) {
				t.Errorf("image = %+v, want %+v", img, tc.wantImg)
			}
			if desc != tc.wantDesc {
				t.Errorf("description = %q, want %q", desc, tc.wantDesc)
			}
		})
	}
}

func TestMediaWikiFetchMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("testdata", "missing-page.json"))
	}))
	t.Cleanup(srv.Close)

	_, err := testClient(Wikipedia(), srv.URL).Fetch(context.Background(), "NoSuchArticleXyzzy123")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMediaWikiFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down for maintenance", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)

	_, err := testClient(Wikipedia(), srv.URL).Fetch(context.Background(), "Photosynthesis")
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("err = %v, want HTTP 503", err)
	}
}

func TestMediaWikiSimpleWikiParse(t *testing.T) {
	// The Simple English wiki's response shape must parse identically. The
	// recorded sample is served with its continue block renamed away so the
	// test stops after one request (the continuation pages for this article
	// are not committed); everything else is the raw recorded body.
	data, err := os.ReadFile(filepath.Join("testdata", "simplewiki-chemistry.json"))
	if err != nil {
		t.Fatal(err)
	}
	body := strings.Replace(string(data), `"continue":`, `"ignored":`, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	a, err := testClient(SimpleWiki(), srv.URL).Fetch(context.Background(), "Chemistry")
	if err != nil {
		t.Fatal(err)
	}
	if a.Source != "simplewiki" {
		t.Errorf("source = %q", a.Source)
	}
	if a.RevisionID != 10976485 {
		t.Errorf("revision = %d", a.RevisionID)
	}
	if !strings.Contains(a.Attribution, "Simple English Wikipedia") {
		t.Errorf("attribution = %q", a.Attribution)
	}
	if err := a.Validate(); err != nil {
		t.Errorf("fetched article invalid: %v", err)
	}
}

// TestMediaWikiCategoryPages covers streaming the wiki's category pages —
// hidden flag and parents — across a continuation boundary, including a
// page whose parent list is split over two responses (clcontinue mid-batch,
// the generator standing still) and so is emitted twice.
func TestMediaWikiCategoryPages(t *testing.T) {
	responses := []string{
		`{"continue":{"clcontinue":"2|Music","continue":"gapcontinue||"},
		  "query":{"pages":[
		   {"pageid":1,"title":"Category:Black metal bands","pageprops":{},
		    "categories":[{"title":"Category:Heavy metal"}]},
		   {"pageid":2,"title":"Category:Articles with hCards","pageprops":{"hiddencat":""}}]}}`,
		`{"continue":{"gapcontinue":"Cities","continue":"gapcontinue||"},
		  "query":{"pages":[
		   {"pageid":2,"title":"Category:Articles with hCards","pageprops":{"hiddencat":""},
		    "categories":[{"title":"Category:Tracking"}]}]}}`,
		`{"query":{"pages":[
		   {"pageid":3,"title":"Category:Cities","categories":[{"title":"Category:Geography"}]}]}}`,
	}
	n := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if n == 2 && r.URL.Query().Get("gapcontinue") != "Cities" {
			t.Errorf("request 3: gapcontinue = %q, want %q", r.URL.Query().Get("gapcontinue"), "Cities")
		}
		if n >= len(responses) {
			t.Fatalf("request %d: more requests than responses", n+1)
		}
		w.Write([]byte(responses[n]))
		n++
	}))
	t.Cleanup(srv.Close)

	var got []CategoryPage
	err := testClient(SimpleWiki(), srv.URL).CategoryPages(context.Background(), func(p CategoryPage) error {
		got = append(got, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []CategoryPage{
		{Name: "Black metal bands", Parents: []string{"Heavy metal"}},
		{Name: "Articles with hCards", Hidden: true},
		{Name: "Articles with hCards", Hidden: true, Parents: []string{"Tracking"}},
		{Name: "Cities", Parents: []string{"Geography"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("category pages = %+v, want %+v", got, want)
	}
}

// TestDomainMap: the walk starts at the root's non-hidden children, breadth
// first; hidden categories neither become domains nor relay one; unreached
// categories get none.
func TestDomainMap(t *testing.T) {
	pages := []CategoryPage{
		{Name: "Nature", Parents: []string{"Articles"}},
		{Name: "People", Parents: []string{"Articles"}},
		{Name: "Very good articles", Hidden: true, Parents: []string{"Articles"}},
		{Name: "Fungi", Parents: []string{"Nature"}},
		{Name: "Mushrooms", Parents: []string{"Fungi", "Food"}},
		{Name: "Hidden relay", Hidden: true, Parents: []string{"Nature"}},
		{Name: "Behind the relay", Parents: []string{"Hidden relay"}},
		{Name: "Orphan", Parents: []string{"Nowhere"}},
	}
	got := DomainMap(pages, "Articles")
	want := map[string]string{
		"Nature":    "Nature",
		"People":    "People",
		"Fungi":     "Nature",
		"Mushrooms": "Nature",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DomainMap = %v, want %v", got, want)
	}
}
