// Package api builds the server's HTTP handler.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/zedeus/wikifyp/server/internal/event"
	"github.com/zedeus/wikifyp/server/internal/feed"
)

// Feed sizes: the default fills a phone screen several times over; the cap
// keeps one request from ranking the corpus into a single response.
const (
	defaultFeedLimit = 20
	maxFeedLimit     = 50
)

// New returns the server's root handler, reading articles and reader state
// from db. A nil db (the server can run without one configured) leaves the
// data endpoints answering 503. Each check is a dependency probe run by
// /healthz (for example (*sql.DB).PingContext); with none, /healthz only
// reports that the process is up.
func New(db *sql.DB, checks ...func(context.Context) error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		for _, check := range checks {
			if err := check(ctx); err != nil {
				log.Printf("healthz: %v", err)
				http.Error(w, "unhealthy", http.StatusServiceUnavailable)
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("GET /v1/feed", withDB(db, handleFeed))
	mux.HandleFunc("GET /v1/articles/{id}", withDB(db, handleArticle))
	mux.HandleFunc("POST /v1/events", withDB(db, handleEvents))
	return mux
}

// withDB guards a data endpoint behind the database being configured.
func withDB(db *sql.DB, h func(*sql.DB, http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			http.Error(w, "no database configured", http.StatusServiceUnavailable)
			return
		}
		h(db, w, r)
	}
}

// handleFeed serves GET /v1/feed?reader=<id>&limit=<n>: the reader's next
// articles, best first.
func handleFeed(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	readerID := r.FormValue("reader")
	if readerID == "" {
		http.Error(w, "missing reader parameter", http.StatusBadRequest)
		return
	}
	limit := defaultFeedLimit
	if s := r.FormValue("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			http.Error(w, "limit must be a positive number", http.StatusBadRequest)
			return
		}
		limit = min(n, maxFeedLimit)
	}
	cards, err := feed.For(r.Context(), db, readerID, limit)
	if err != nil {
		serverError(w, err)
		return
	}

	// Record impressions: every card the feed serves is one impression.
	if len(cards) > 0 {
		imps := make([]struct {
			Source string
			PageID int64
		}, len(cards))
		for i, c := range cards {
			imps[i].Source = c.Source
			imps[i].PageID = c.PageID
		}
		if err := event.RecordImpressions(r.Context(), db, readerID, imps); err != nil {
			log.Printf("api: record impressions: %v", err)
		}
	}

	writeJSON(w, map[string]any{"articles": cards})
}

// articleResponse matches the bundled-asset article format the app reads,
// so a downloaded article and a bundled one are the same thing to it.
type articleResponse struct {
	ID          int64              `json:"id"`
	Title       string             `json:"title"`
	Attribution string             `json:"attribution"`
	Description string             `json:"description,omitempty"`
	Image       *feed.Image        `json:"image,omitempty"`
	Sentences   []sentenceResponse `json:"sentences"`
}

type sentenceResponse struct {
	Text         string `json:"text"`
	HeadingLevel int    `json:"heading_level,omitempty"`
}

// handleArticle serves GET /v1/articles/{id}: one stored revision's
// sentences, in address order.
func handleArticle(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "article id must be a number", http.StatusBadRequest)
		return
	}
	resp := articleResponse{ID: id, Sentences: []sentenceResponse{}}
	var desc, imgURL sql.NullString
	var imgW, imgH sql.NullInt32
	err = db.QueryRowContext(r.Context(),
		"SELECT title, attribution, description, image_url, image_width, image_height FROM articles WHERE id = $1", id).
		Scan(&resp.Title, &resp.Attribution, &desc, &imgURL, &imgW, &imgH)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "no such article", http.StatusNotFound)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
	resp.Description = desc.String
	resp.Image = feed.NullImage(imgURL, imgW, imgH)
	rows, err := db.QueryContext(r.Context(),
		"SELECT text, heading_level FROM sentences WHERE article_id = $1 ORDER BY sentence_idx", id)
	if err != nil {
		serverError(w, err)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var s sentenceResponse
		if err := rows.Scan(&s.Text, &s.HeadingLevel); err != nil {
			serverError(w, err)
			return
		}
		resp.Sentences = append(resp.Sentences, s)
	}
	if err := rows.Err(); err != nil {
		serverError(w, err)
		return
	}
	writeJSON(w, resp)
}

// maxEventBody caps the request body for event uploads: 50 events at ~300
// bytes each is well under 1 MiB, and the cap prevents a runaway client
// from exhausting memory.
const maxEventBody = 1 << 20

// handleEvents serves POST /v1/events?reader=<id>: a batch of reading
// events. Each event is decoded and validated independently; a bad event
// is reported in the per-event results without failing the batch.
func handleEvents(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	readerID := r.FormValue("reader")
	if readerID == "" {
		http.Error(w, "missing reader parameter", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxEventBody+1))
	if err != nil {
		http.Error(w, "reading body", http.StatusBadRequest)
		return
	}
	if len(body) > maxEventBody {
		http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		return
	}

	var req struct {
		Events []json.RawMessage `json:"events"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if len(req.Events) == 0 {
		http.Error(w, "empty events array", http.StatusBadRequest)
		return
	}

	type eventResult struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  string `json:"error,omitempty"`
	}

	results := make([]eventResult, 0, len(req.Events))
	valid := make([]event.Event, 0, len(req.Events))
	validIdx := make([]int, 0, len(req.Events)) // index into results

	for _, raw := range req.Events {
		e, err := event.Decode(raw)
		if err != nil {
			id := extractID(raw)
			results = append(results, eventResult{ID: id, Status: "rejected", Error: err.Error()})
			continue
		}
		idx := len(results)
		results = append(results, eventResult{ID: e.ID, Status: "ok"})
		valid = append(valid, e)
		validIdx = append(validIdx, idx)
	}

	if len(valid) > 0 {
		storeResults, err := event.Store(r.Context(), db, readerID, valid)
		if err != nil {
			serverError(w, err)
			return
		}
		for i, sr := range storeResults {
			ri := validIdx[i]
			if !sr.Stored {
				results[ri].Status = "duplicate"
			}
		}

		// Side effects for newly stored events only — duplicates were
		// already processed on the original upload.
		for i, e := range valid {
			if !storeResults[i].Stored {
				continue
			}
			meta, err := event.LookupArticle(r.Context(), db, e.Token.ArticleID)
			if err != nil {
				log.Printf("api: events side effect: %v", err)
				continue
			}

			// finish/skip → update feed state; save/unsave → the saved
			// flag. The two are orthogonal columns on the same row, so a
			// finish after a save leaves the page both finished and saved —
			// the bookmark survives finishing.
			if state, ok := e.FeedState(); ok {
				if err := feed.SetState(r.Context(), db, readerID, meta.Source, meta.PageID, state); err != nil {
					log.Printf("api: events feed state: %v", err)
				}
			}
			if saved, ok := e.SavedChange(); ok {
				if err := feed.SetSaved(r.Context(), db, readerID, meta.Source, meta.PageID, saved); err != nil {
					log.Printf("api: events saved flag: %v", err)
				}
			}

			// Every event updates completion: how far through the
			// article the reader has reached (max-so-far, never regresses).
			comp, err := event.Completion(r.Context(), db, e.Token.ArticleID, e.Token.SentenceIdx, e.Token.TokenIdx)
			if err != nil {
				log.Printf("api: events completion: %v", err)
				continue
			}
			if err := event.UpdateCompletion(r.Context(), db, readerID, meta.Source, meta.PageID, comp); err != nil {
				log.Printf("api: events completion: %v", err)
			}
		}
	}

	writeJSON(w, map[string]any{"results": results})
}

// extractID pulls the "id" field from raw JSON without fully decoding
// it, so rejected events can still be reported by their client ID.
func extractID(raw json.RawMessage) string {
	var partial struct {
		ID string `json:"id"`
	}
	json.Unmarshal(raw, &partial)
	return partial.ID
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("api: encode response: %v", err)
	}
}

// serverError hides the error's text from the client (it can carry SQL)
// but logs it for the operator.
func serverError(w http.ResponseWriter, err error) {
	log.Printf("api: %v", err)
	http.Error(w, "internal error", http.StatusInternalServerError)
}
