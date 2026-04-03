package health

import (
	"context"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
)

// Handler serves Kubernetes-style liveness and readiness probes over HTTP.
type Handler struct {
	DB       DatabasePinger
	Exchange ExchangeDepthReadiness
	Timeout  time.Duration
}

// NewHandler builds probe handlers. timeout caps each readiness check (PostgreSQL + Grinex in parallel).
func NewHandler(db DatabasePinger, exchange ExchangeDepthReadiness, timeout time.Duration) *Handler {
	return &Handler{DB: db, Exchange: exchange, Timeout: timeout}
}

// Live reports that the process is up (no dependency checks).
func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Ready checks PostgreSQL and Grinex API. Returns 503 if any check fails or times out.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	timeout := h.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return h.DB.Ping(gctx)
	})
	g.Go(func() error {
		_, err := h.Exchange.Fetch(gctx)
		return err
	})

	if err := g.Wait(); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Mount registers standard probe paths on mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/healthz/live", h.Live)
	mux.HandleFunc("/healthz/ready", h.Ready)
}
