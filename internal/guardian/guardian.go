// Package guardian is the long-running root process of ELNSSM. It hosts
// the Windows service handler, owns the manager, the API server and all
// shared subsystems, and provides install/uninstall helpers for the SCM.
package guardian

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/hilman2/ELNSSM/internal/api"
	"github.com/hilman2/ELNSSM/internal/cluster"
	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/logging"
	"github.com/hilman2/ELNSSM/internal/manager"
	"github.com/hilman2/ELNSSM/internal/notify"
	"github.com/hilman2/ELNSSM/internal/process"
	"github.com/hilman2/ELNSSM/internal/store"
)

// Guardian is the core orchestrator of the ELNSSM service manager.
type Guardian struct {
	cfg            *config.Config
	store          store.Store
	manager        *manager.Manager
	notifier       *notify.Dispatcher
	apiServer      *api.Server
	hostMonitor    *process.HostMonitor
	clusterManager *cluster.Manager
	startedAt      time.Time
	restartMode    atomic.Bool // true = detach children instead of killing them
}

// New creates a new Guardian instance.
func New(cfg *config.Config) (*Guardian, error) {
	// Open bbolt store
	s, err := store.NewBoltStore(cfg.DataPath())
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}

	if err := s.RunMigrations(); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	// Create notification dispatcher
	notifier := notify.NewDispatcher(cfg)

	// Create log streamer (shared between manager and API server)
	streamer := logging.NewStreamer()

	// Create service manager
	mgr := manager.New(s, notifier, cfg, streamer)

	// Create host-level resource monitor
	hostMon := process.NewHostMonitor(2 * time.Second)

	// Create cluster manager
	clusterMgr := cluster.New(&cfg.Cluster)

	g := &Guardian{
		cfg:            cfg,
		store:          s,
		manager:        mgr,
		notifier:       notifier,
		hostMonitor:    hostMon,
		clusterManager: clusterMgr,
	}

	// Create API server only if enabled
	if cfg.API.Enabled {
		g.apiServer = api.NewServer(cfg, mgr, s, streamer, hostMon, clusterMgr, g)
	} else {
		slog.Info("API server disabled via config")
	}

	return g, nil
}

// Run starts the Guardian and blocks until the context is cancelled.
func (g *Guardian) Run(ctx context.Context) error {
	g.startedAt = time.Now()

	// Start host-level resource monitor
	go g.hostMonitor.Run(ctx)

	// Load and start services
	slog.Info("Loading managed services...")
	if err := g.manager.LoadAll(ctx); err != nil {
		slog.Error("Failed to load services", "error", err)
	}

	// Check for restart state (orphaned processes from a previous graceful restart)
	orphans, err := readRestartState(g.cfg.Guardian.DataDir)
	if err != nil {
		slog.Warn("Could not read restart state", "error", err)
	}
	if len(orphans) > 0 {
		slog.Info("Found orphaned processes from previous restart, re-adopting...", "count", len(orphans))
		g.manager.AdoptOrphans(ctx, orphans)
		clearRestartState(g.cfg.Guardian.DataDir)
	} else {
		// Only auto-start if we didn't adopt orphans (they're already running)
		slog.Info("Starting auto-start services...")
		g.manager.AutoStart(ctx)
	}

	// Start cluster manager (heartbeat for slaves)
	if g.clusterManager != nil && g.clusterManager.Role() != "standalone" {
		slog.Info("Starting cluster manager", "role", g.clusterManager.Role())
		g.clusterManager.Start(ctx)
	}

	// Start API server
	if g.apiServer != nil {
		slog.Info("Starting API server", "listen", g.cfg.API.Listen)
		go func() {
			if err := g.apiServer.Start(); err != nil {
				slog.Error("API server error", "error", err)
			}
		}()
	}

	// Wait for shutdown
	<-ctx.Done()
	slog.Info("Shutting down Guardian...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if g.restartMode.Load() {
		// Graceful restart: detach children so they survive
		slog.Info("Restart mode: detaching child processes...")
		orphans := g.manager.DetachAll()
		if err := writeRestartState(g.cfg.Guardian.DataDir, orphans); err != nil {
			slog.Error("Failed to write restart state, killing processes instead", "error", err)
			if err := g.manager.StopAll(shutdownCtx); err != nil {
				slog.Error("Error stopping services", "error", err)
			}
		} else {
			slog.Info("Restart state saved", "orphans", len(orphans))
		}
	} else {
		// Normal shutdown: stop all children
		if err := g.manager.StopAll(shutdownCtx); err != nil {
			slog.Error("Error stopping services", "error", err)
		}
	}
	g.manager.Shutdown()

	// Stop cluster manager
	if g.clusterManager != nil {
		g.clusterManager.Stop()
	}

	// Stop API server
	if g.apiServer != nil {
		if err := g.apiServer.Stop(shutdownCtx); err != nil {
			slog.Error("Error stopping API server", "error", err)
		}
	}

	// Close store
	if err := g.store.Close(); err != nil {
		slog.Error("Error closing store", "error", err)
	}

	slog.Info("Guardian shutdown complete")
	return nil
}

// RequestRestart sets restart mode and returns. The actual restart is
// triggered by the SCM (service_handler.go) or by the caller stopping the context.
func (g *Guardian) RequestRestart() {
	g.restartMode.Store(true)
}
