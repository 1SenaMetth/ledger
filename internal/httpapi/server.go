package httpapi

import (
	"context"
	"net/http"
	"time"
)

// Pinger is the piece of the database the health check needs. Depending on a
// one-method interface instead of *pgxpool.Pool keeps this package testable
// without a database.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Server holds the dependencies the HTTP handlers need.
type Server struct {
	DB      Pinger
	Version string
	Commit  string
}

// Routes builds the router. Go's standard ServeMux supports method and path
// patterns, so a third-party router is not required. Swap in gin or chi later
// if you actually need what they add.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", s.handleLive)
	mux.HandleFunc("GET /readyz", s.handleReady)
	mux.HandleFunc("GET /version", s.handleVersion)

	// TODO(phase1): register the real routes as you build them.
	//   POST   /v1/auth/register
	//   POST   /v1/auth/login
	//   POST   /v1/auth/refresh
	//   POST   /v1/accounts
	//   GET    /v1/accounts/{id}
	//   GET    /v1/accounts/{id}/entries
	//   POST   /v1/transfers
	//   POST   /v1/deposits
	//   POST   /v1/withdrawals

	return Chain(mux,
		RequestID,
		Recoverer,
		Logger,
		Timeout(10*time.Second),
	)
}

// handleLive answers "is this process running". It must not touch the database:
// a liveness probe that fails during a brief database blip will restart every
// pod at once and turn a small outage into a large one.
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady answers "can this process serve traffic". This one does check the
// database, so Kubernetes takes the pod out of the load balancer when its
// dependencies are unavailable.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if s.DB != nil {
		if err := s.DB.Ping(ctx); err != nil {
			WriteProblem(w, r, http.StatusServiceUnavailable,
				"Not Ready", "database is unreachable")
			return
		}
	}
	WriteJSON(w, r, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, r, http.StatusOK, map[string]string{
		"version": s.Version,
		"commit":  s.Commit,
	})
}
