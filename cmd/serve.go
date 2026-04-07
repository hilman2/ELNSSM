package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/guardian"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run Guardian in foreground (debug mode)",
	Long:  `Starts the ELNSSM Guardian in the foreground for debugging. Press Ctrl+C to stop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			slog.Warn("Could not load config, using defaults", "error", err)
			cfg = config.DefaultConfig()
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Handle Ctrl+C
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			sig := <-sigCh
			slog.Info("Received signal, shutting down...", "signal", sig)
			cancel()
		}()

		g, err := guardian.New(cfg)
		if err != nil {
			return fmt.Errorf("creating guardian: %w", err)
		}

		slog.Info("Starting ELNSSM Guardian in foreground mode", "listen", cfg.API.Listen)
		return g.Run(ctx)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

// runGuardianService starts the Guardian as a Windows service (called from root command).
func runGuardianService() error {
	cfgPath := cfgFile
	if cfgPath == "" {
		cfgPath = config.DefaultConfigPath()
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		cfg = config.DefaultConfig()
	}

	return guardian.RunAsService(cfg)
}
