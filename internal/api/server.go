package api

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/hilman2/ELNSSM/internal/cluster"
	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/logging"
	"github.com/hilman2/ELNSSM/internal/manager"
	"github.com/hilman2/ELNSSM/internal/process"
	"github.com/hilman2/ELNSSM/internal/sspi"
	"github.com/hilman2/ELNSSM/internal/store"
	"github.com/hilman2/ELNSSM/internal/web"
	"github.com/go-chi/chi/v5"
)

// Restarter is the interface for triggering a Guardian restart.
type Restarter interface {
	RequestRestart()
}

// Server is the HTTP API server.
type Server struct {
	cfg         *config.Config
	manager     *manager.Manager
	store       store.Store
	streamer    *logging.Streamer
	hostMonitor *process.HostMonitor
	cluster     *cluster.Manager
	restarter   Restarter
	server      *http.Server
	apiLimiter  *RateLimiter
	sspiStore   *sspi.SessionStore
}

// NewServer creates a new API server.
func NewServer(cfg *config.Config, mgr *manager.Manager, s store.Store, streamer *logging.Streamer, hostMon *process.HostMonitor, clusterMgr *cluster.Manager, restarter Restarter) *Server {
	srv := &Server{
		cfg:         cfg,
		manager:     mgr,
		store:       s,
		streamer:    streamer,
		hostMonitor: hostMon,
		cluster:     clusterMgr,
		restarter:   restarter,
		apiLimiter:  NewRateLimiter(120, 1*time.Minute),
	}

	// Initialize SSPI session store if auth type is "sspi"
	if cfg.API.Auth.Enabled && cfg.API.Auth.Type == "sspi" {
		ss, err := sspi.NewSessionStore()
		if err != nil {
			slog.Warn("SSPI initialization failed, falling back to token-only auth", "error", err)
		} else {
			srv.sspiStore = ss
			slog.Info("SSPI Negotiate authentication enabled")
		}
	}

	router := srv.buildRouter()
	srv.server = &http.Server{
		Addr:    cfg.API.Listen,
		Handler: router,
	}

	// Set up connection tracking for SSPI
	if srv.sspiStore != nil {
		srv.server.ConnContext = func(ctx context.Context, c net.Conn) context.Context {
			return context.WithValue(ctx, ctxKeyConnID, c.RemoteAddr().String())
		}
		srv.server.ConnState = func(c net.Conn, state http.ConnState) {
			if state == http.StateClosed || state == http.StateHijacked {
				srv.sspiStore.Remove(c.RemoteAddr().String())
			}
		}
	}

	return srv
}

// Streamer returns the log streamer for use by the manager.
func (s *Server) Streamer() *logging.Streamer {
	return s.streamer
}

func (s *Server) buildRouter() *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(securityHeaders)
	r.Use(requestLogger)
	r.Use(ipWhitelist(&s.cfg.API))

	// API v1
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(rateLimitMiddleware(s.apiLimiter))
		r.Use(requestBodyLimit(1 << 20)) // 1 MB
		r.Use(authMiddleware(&s.cfg.API.Auth, s.sspiStore))

		// Services
		r.Get("/services", s.handleListServices)
		r.Post("/services", s.handleAddService)
		r.Get("/services/{id}", s.handleGetService)
		r.Put("/services/{id}", s.handleUpdateService)
		r.Delete("/services/{id}", s.handleDeleteService)
		r.Post("/services/{id}/start", s.handleStartService)
		r.Post("/services/{id}/stop", s.handleStopService)
		r.Post("/services/{id}/restart", s.handleRestartService)
		r.Get("/services/{id}/resources", s.handleGetResources)

		// Logs
		r.Get("/services/{id}/logs", s.handleGetLogs)
		r.Get("/services/{id}/logs/download", s.handleDownloadLog)
		r.Get("/services/{id}/logs/stream", s.handleStreamLogs)

		// Health
		r.Get("/services/{id}/health", s.handleGetHealth)
		r.Get("/services/{id}/health/history", s.handleGetHealthHistory)

		// Performance
		r.Get("/services/{id}/performance", s.handleGetPerformance)

		// Native Windows Services (opt-in via config)
		if s.cfg.Guardian.EnableNativeServices {
			r.Get("/native-services", s.handleListNativeServices)
			r.Get("/native-services/{name}", s.handleGetNativeService)
			r.Post("/native-services/{name}/start", s.handleStartNativeService)
			r.Post("/native-services/{name}/stop", s.handleStopNativeService)
			r.Post("/native-services/{name}/restart", s.handleRestartNativeService)
		}

		// Events
		r.Get("/events", s.handleListEvents)
		r.Get("/events/stream", s.handleStreamEvents)

		// Cluster
		r.Get("/cluster/status", s.handleClusterStatus)
		r.Get("/cluster/nodes", s.handleClusterNodes)
		r.Post("/cluster/heartbeat", s.handleClusterHeartbeat)
		r.HandleFunc("/cluster/nodes/{nodeId}/proxy/*", s.handleClusterProxy)

		// Config
		r.Get("/config", s.handleGetConfig)
		r.Put("/config", s.handleUpdateConfig)

		// System
		r.Get("/system/status", s.handleSystemStatus)
		r.Get("/system/version", s.handleVersion)
		r.Post("/system/restart", s.handleRestart)
		r.Get("/system/resources/stream", s.handleStreamResources)
	})

	// Web GUI (embedded SPA) - no auth required (login prompt handled in frontend)
	r.Handle("/*", web.SPAHandler())

	return r
}

// Start begins listening for HTTP connections.
func (s *Server) Start() error {
	return s.server.ListenAndServe()
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.sspiStore != nil {
		s.sspiStore.Close()
	}
	return s.server.Shutdown(ctx)
}
