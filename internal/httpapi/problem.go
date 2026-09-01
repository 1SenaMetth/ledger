// Package httpapi contains the HTTP transport layer: routing, middleware, and
// the translation between domain errors and HTTP responses. Business rules
// live in internal/domain and must not leak into this package.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Problem is an RFC 9457 "Problem Details" response body. Using a documented
// error format instead of ad-hoc JSON is the kind of small decision reviewers
// notice.
type Problem struct {
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Status   int               `json:"status"`
	Detail   string            `json:"detail,omitempty"`
	Instance string            `json:"instance,omitempty"`
	Errors   map[string]string `json:"errors,omitempty"`
}

// WriteProblem sends a problem document. Never pass a raw internal error as
// detail: it can leak table names, file paths, and query fragments.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, title, detail string) {
	p := Problem{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode problem response", "error", err)
	}
}

// WriteJSON sends a successful JSON response.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.ErrorContext(r.Context(), "failed to encode response", "error", err)
	}
}

// TODO(phase1): add a MapDomainError(err) (status int, title string) helper
// that translates domain sentinel errors into status codes, for example:
//
//	ErrInsufficientFunds  -> 422 Unprocessable Content
//	ErrAccountNotFound    -> 404 Not Found
//	ErrCurrencyMismatch   -> 400 Bad Request
//	anything unrecognised -> 500, logged, with a generic detail
