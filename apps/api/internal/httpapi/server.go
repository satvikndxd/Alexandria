// Package httpapi is the HTTP edge of the modular monolith.
// Handlers stay thin: parse → validate (domain) → store → respond.
// Slow work (metadata fetching, search indexing) is enqueued, never inlined.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/alexandria-reads/alexandria/apps/api/internal/config"
	"github.com/alexandria-reads/alexandria/apps/api/internal/domain"
	"github.com/alexandria-reads/alexandria/apps/api/internal/ratelimit"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

type Server struct {
	cfg      config.Config
	store    *store.Store
	friction domain.FrictionPolicy
	limiter  *ratelimit.Limiter
	router   chi.Router
}

func New(cfg config.Config, st *store.Store) *Server {
	s := &Server{
		cfg:   cfg,
		store: st,
		friction: domain.FrictionPolicy{
			MinBodyChars:      cfg.ReviewMinChars,
			MaxBodyChars:      20000,
			NewAccountDaily:   cfg.NewAccountDaily,
			TrustedDaily:      cfg.TrustedDaily,
			TrustedReputation: cfg.TrustedReputation,
		},
		limiter: ratelimit.New(120, 30), // 120 req/min sustained, burst 30, per IP
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(slogLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(s.rateLimit)

	r.Get("/healthz", s.handleHealth)
	r.Route("/v1", func(r chi.Router) {
		r.Get("/works/{slug}", s.handleGetWork)
		r.Get("/works/{slug}/reviews", s.handleListReviews)
		r.Post("/works/{slug}/reviews", s.handleCreateReview)
	})

	s.router = r
	return s
}

func (s *Server) Handler() http.Handler { return s.router }

// Run serves until ctx is cancelled, then drains gracefully.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	slog.Info("api listening", "addr", s.cfg.Addr)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), s.cfg.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// ---- middleware -------------------------------------------------------------

func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if !s.limiter.Allow(host) {
			respondError(w, http.StatusTooManyRequests, "rate_limited", "Slow down — Alexandria favors deliberate reading over rapid clicking.")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func slogLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("http",
			"method", r.Method, "path", r.URL.Path,
			"status", ww.Status(), "bytes", ww.BytesWritten(),
			"dur_ms", time.Since(start).Milliseconds(),
			"req_id", middleware.GetReqID(r.Context()))
	})
}

// ---- responses --------------------------------------------------------------

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func respondError(w http.ResponseWriter, status int, code, msg string) {
	respondJSON(w, status, map[string]apiError{"error": {Code: code, Message: msg}})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
