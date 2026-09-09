// Package httpapi is the HTTP edge of the modular monolith.
//
// Handlers stay thin: parse → validate (internal/domain) → store/search →
// respond. Slow work (metadata fetching, cover caching, index updates, email
// delivery) is enqueued through the transactional outbox and never inlined in
// a request, so a slow upstream can never become a slow page.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/alexandria-reads/alexandria/apps/api/internal/auth"
	"github.com/alexandria-reads/alexandria/apps/api/internal/config"
	"github.com/alexandria-reads/alexandria/apps/api/internal/domain"
	"github.com/alexandria-reads/alexandria/apps/api/internal/ratelimit"
	"github.com/alexandria-reads/alexandria/apps/api/internal/reader"
	"github.com/alexandria-reads/alexandria/apps/api/internal/realtime"
	"github.com/alexandria-reads/alexandria/apps/api/internal/search"
	"github.com/alexandria-reads/alexandria/apps/api/internal/store"
)

type Server struct {
	cfg       config.Config
	store     *store.Store
	authSvc   *auth.Service
	magic     *auth.MagicLinks
	search    *search.Client
	textCache *reader.Cache
	lk        realtime.Config
	bootCtx   context.Context
	friction  domain.FrictionPolicy
	limiter   *ratelimit.Limiter
	router    chi.Router
}

// New wires the whole HTTP edge. Construction is side-effect free so tests can
// build a server against a scratch database and drive it with httptest.
func New(cfg config.Config, st *store.Store, authSvc *auth.Service, magic *auth.MagicLinks, sc *search.Client) *Server {
	s := &Server{
		cfg:       cfg,
		store:     st,
		authSvc:   authSvc,
		magic:     magic,
		search:    sc,
		textCache: reader.NewCache(cfg.ReaderCacheDir),
		lk: realtime.Config{
			URL: cfg.LiveKitURL, APIKey: cfg.LiveKitAPIKey, APISecret: cfg.LiveKitAPISecret,
		},
		bootCtx:   context.Background(),
		friction: domain.FrictionPolicy{
			MinBodyChars:      cfg.ReviewMinChars,
			MaxBodyChars:      20000,
			NewAccountDaily:   cfg.NewAccountDaily,
			TrustedDaily:      cfg.TrustedDaily,
			TrustedReputation: cfg.TrustedReputation,
		},
		// Token bucket per client IP: generous for a human, punishing for a
		// scraper. Tests raise both knobs so suites are not rate-limited.
		limiter: ratelimit.New(cfg.RatePerMin, cfg.RateBurst),
	}
	s.router = s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) routes() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(slogLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(s.securityHeaders)
	r.Use(s.rateLimit)
	r.Use(s.resolveSession)
	r.Use(s.csrfProtect)

	r.Get("/healthz", s.handleHealth)

	r.Route("/v1", func(r chi.Router) {
		// ---- identity ----
		r.Post("/auth/register", s.handleRegister)
		r.Post("/auth/passkey/finish-registration", s.handleFinishRegistration)
		r.Post("/auth/passkey/begin-login", s.handleBeginLogin)
		r.Post("/auth/passkey/finish-login", s.handleFinishLogin)
		r.Post("/auth/magic-link", s.handleRequestMagicLink)
		r.Post("/auth/magic-link/verify", s.handleVerifyMagicLink)
		r.Post("/auth/logout", s.handleLogout)
		r.Get("/me", s.handleMe)
		r.Patch("/me", requireAuth(s.handleUpdateProfile))

		// ---- catalogue ----
		r.Get("/works", s.handleListWorks)
		r.Get("/works/{slug}", s.handleGetWork)
		r.Get("/works/{slug}/editions", s.handleListEditions)
		r.Get("/works/{slug}/reviews", s.handleListReviews)
		r.Post("/works/{slug}/reviews", requireAuth(s.handleCreateReview))
		r.Get("/works/{slug}/notes", s.handleListScholarNotes)
		r.Get("/works/{slug}/related", s.handleRelatedWorks)
		r.Get("/authors/{slug}", s.handleGetAuthor)
		r.Get("/authors/{slug}/works", s.handleAuthorWorks)
		r.Get("/subjects", s.handleListSubjects)

		// ---- reviews as addressable objects ----
		r.Get("/reviews/{id}", s.handleGetReview)
		r.Patch("/reviews/{id}", requireAuth(s.handleUpdateReview))
		r.Delete("/reviews/{id}", requireAuth(s.handleDeleteReview))
		r.Post("/reviews/{id}/like", requireAuth(s.handleLikeReview))
		r.Get("/reviews/{id}/comments", s.handleListComments)
		r.Post("/reviews/{id}/comments", requireAuth(s.handleCreateComment))
		r.Delete("/comments/{id}", requireAuth(s.handleDeleteComment))

		// ---- personal library (RLS-scoped) ----
		r.Get("/me/shelves", requireAuth(s.handleListShelves))
		r.Post("/me/shelves", requireAuth(s.handleCreateShelf))
		r.Delete("/me/shelves/{id}", requireAuth(s.handleDeleteShelf))
		r.Get("/me/library", requireAuth(s.handleLibrary))
		r.Post("/me/library", requireAuth(s.handleAddToLibrary))
		r.Delete("/me/library/{slug}", requireAuth(s.handleRemoveFromLibrary))
		r.Post("/me/progress", requireAuth(s.handleSetProgress))
		r.Post("/me/progress/finish", requireAuth(s.handleFinishBook))
		r.Post("/me/progress/dnf", requireAuth(s.handleDNF))
		r.Get("/me/currently-reading", requireAuth(s.handleCurrentlyReading))
		r.Get("/me/stats", requireAuth(s.handleStats))
		r.Post("/me/annotations", requireAuth(s.handleCreateAnnotation))
		r.Get("/me/annotations", requireAuth(s.handleListAnnotations))
		r.Delete("/me/annotations/{id}", requireAuth(s.handleDeleteAnnotation))

		// ---- social ----
		r.Get("/feed", requireAuth(s.handleFeed))
		r.Get("/community-feed", s.handleCommunityFeed)
		r.Get("/users/{username}", s.handlePublicProfile)
		r.Get("/users/{username}/reviews", s.handleUserReviews)
		r.Get("/users/{username}/followers", s.handleFollowers)
		r.Get("/users/{username}/following", s.handleFollowing)
		r.Post("/users/{username}/follow", requireAuth(s.handleFollow))
		r.Delete("/users/{username}/follow", requireAuth(s.handleUnfollow))
		r.Post("/users/{username}/block", requireAuth(s.handleBlock))
		r.Delete("/users/{username}/block", requireAuth(s.handleUnblock))
		r.Get("/me/notifications", requireAuth(s.handleNotifications))
		r.Post("/me/notifications/read", requireAuth(s.handleMarkNotificationsRead))

		// ---- clubs ----
		r.Get("/clubs", s.handleListClubs)
		r.Post("/clubs", requireAuth(s.handleCreateClub))
		r.Get("/clubs/{slug}", s.handleGetClub)
		r.Post("/clubs/{slug}/join", requireAuth(s.handleJoinClub))
		r.Delete("/clubs/{slug}/join", requireAuth(s.handleLeaveClub))
		r.Get("/clubs/{slug}/channels", s.handleListChannels)
		r.Post("/channels/{id}/room/token", requireAuth(s.handleRoomToken))
		r.Get("/channels/{id}/messages", s.handleListMessages)
		r.Post("/channels/{id}/messages", requireAuth(s.handlePostMessage))

		// ---- discovery ----
		r.Get("/search", s.handleSearch)

		// ---- portability (Phase 5) ----
		r.Post("/imports/goodreads", requireAuth(s.handleImportGoodreads))
		r.Post("/imports/storygraph", requireAuth(s.handleImportStoryGraph))
		r.Post("/imports/kindle", requireAuth(s.handleImportKindle))
		r.Get("/export", requireAuth(s.handleExport))
		r.Post("/account/delete", requireAuth(s.handleDeleteAccount))

		// ---- scholarship (Phase 4) ----
		r.Post("/works/{slug}/notes", requireAuth(s.handleCreateNote))
		r.Get("/notes/{id}", s.handleGetNote)
		r.Patch("/notes/{id}", requireAuth(s.handleUpdateNote))
		r.Post("/notes/{id}/submit", requireAuth(s.handleSubmitNote))
		r.Post("/notes/{id}/review", requireAuth(s.handleReviewNote))
		r.Post("/notes/{id}/retract", requireAuth(s.handleRetractNote))
		r.Get("/me/scholar-profile", requireAuth(s.handleScholarProfile))
		r.Post("/me/scholar-profile", requireAuth(s.handleScholarApply))
		r.Get("/scholar/queue", requireAuth(s.handleScholarQueue))
		r.Get("/moderation/scholars", requireModeration(s.handlePendingScholars))
		r.Post("/moderation/scholars/{userID}", requireModeration(s.handleSetScholarStatus))

		// ---- reader (Phase 3) ----
		r.Get("/editions/{id}/reader", s.handleReaderMeta)
		r.Get("/editions/{id}/reader/chapter/{idx}", s.handleReaderChapter)

		// ---- trust & safety ----
		r.Post("/reports", requireAuth(s.handleCreateReport))
		r.Get("/moderation/reports", requireModeration(s.handleListReports))
		r.Post("/moderation/reports/{id}/action", requireModeration(s.handleModerationAction))
	})

	return r
}

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

// ---- middleware -----------------------------------------------------------------------

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy",
			"default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := s.clientIP(r)
		if !s.limiter.Allow(ip) {
			respondError(w, http.StatusTooManyRequests, "rate_limited",
				"Slow down — Alexandria favors deliberate reading over rapid clicking.")
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

// ---- health ---------------------------------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	body := map[string]any{"status": "ok"}
	if s.search.Enabled() {
		if err := s.search.Health(r.Context()); err != nil {
			body["search"] = "degraded"
		} else {
			body["search"] = "ok"
		}
	}
	respondJSON(w, http.StatusOK, body)
}
