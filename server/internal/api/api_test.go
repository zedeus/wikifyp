package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	healthy := func(context.Context) error { return nil }
	failing := func(context.Context) error { return errors.New("refused") }

	tests := []struct {
		name       string
		checks     []func(context.Context) error
		wantStatus int
	}{
		{name: "no checks", checks: nil, wantStatus: http.StatusOK},
		{name: "check passes", checks: []func(context.Context) error{healthy}, wantStatus: http.StatusOK},
		{name: "check fails", checks: []func(context.Context) error{failing}, wantStatus: http.StatusServiceUnavailable},
		{name: "one of two fails", checks: []func(context.Context) error{healthy, failing}, wantStatus: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			New(tt.checks...).ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("GET /healthz = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

// /healthz is registered for GET only; the route pattern is part of the
// contract.
func TestHealthzMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	New().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /healthz = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
