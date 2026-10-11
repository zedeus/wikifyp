// Package api builds the server's HTTP handler.
package api

import (
	"context"
	"log"
	"net/http"
	"time"
)

// New returns the server's root handler. Each check is a dependency probe
// run by /healthz (for example (*sql.DB).PingContext); with none, /healthz
// only reports that the process is up.
func New(checks ...func(context.Context) error) http.Handler {
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
	return mux
}
