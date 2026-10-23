// Package api builds the server's HTTP handler.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

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
	writeJSON(w, map[string]any{"articles": cards})
}

// articleResponse matches the bundled-asset article format the app reads,
// so a downloaded article and a bundled one are the same thing to it.
type articleResponse struct {
	ID          int64              `json:"id"`
	Title       string             `json:"title"`
	Attribution string             `json:"attribution"`
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
	err = db.QueryRowContext(r.Context(),
		"SELECT title, attribution FROM articles WHERE id = $1", id).
		Scan(&resp.Title, &resp.Attribution)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "no such article", http.StatusNotFound)
		return
	}
	if err != nil {
		serverError(w, err)
		return
	}
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
