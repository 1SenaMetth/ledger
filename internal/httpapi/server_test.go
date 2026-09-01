package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(context.Context) error { return s.err }

func TestHealthEndpoints(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		db         Pinger
		wantStatus int
	}{
		{"live is always ok", "/healthz", stubPinger{err: errors.New("db down")}, http.StatusOK},
		{"ready with healthy db", "/readyz", stubPinger{}, http.StatusOK},
		{"ready with broken db", "/readyz", stubPinger{err: errors.New("db down")}, http.StatusServiceUnavailable},
		{"version", "/version", stubPinger{}, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &Server{DB: tt.db, Version: "test", Commit: "abc123"}
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			srv.Routes().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestRequestIDIsSetOnResponse(t *testing.T) {
	srv := &Server{DB: stubPinger{}}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if rec.Header().Get("X-Request-Id") == "" {
		t.Error("X-Request-Id header is missing")
	}
}

func TestRequestIDIsPreservedFromUpstream(t *testing.T) {
	srv := &Server{DB: stubPinger{}}
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("X-Request-Id", "upstream-id")
	rec := httptest.NewRecorder()

	srv.Routes().ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-Id"); got != "upstream-id" {
		t.Errorf("X-Request-Id = %q, want %q", got, "upstream-id")
	}
}

func TestRecovererTurnsPanicInto500(t *testing.T) {
	panicking := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	h := Chain(panicking, RequestID, Recoverer)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
}

func TestUnknownRouteIs404(t *testing.T) {
	srv := &Server{DB: stubPinger{}}
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
