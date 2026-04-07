// Package cmd contains the cobra-based command-line interface that
// drives ELNSSM. It is also the entry point used when the binary is
// invoked by the Windows Service Control Manager.
package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows/svc"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "elnssm",
	Short: "ELNSSM - Even Less Non-Sucking Service Manager",
	Long: `ELNSSM is a modern Windows service manager that wraps any executable
as a Windows service with health checks, restart policies, notifications,
and a web-based management GUI.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		level := slog.LevelInfo
		if verbose {
			level = slog.LevelDebug
		}
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: level,
		})))
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// If running as a Windows service, start the Guardian
		isService, err := svc.IsWindowsService()
		if err != nil {
			return fmt.Errorf("detecting service mode: %w", err)
		}
		if isService {
			return runGuardianService()
		}
		// Otherwise show help
		return cmd.Help()
	},
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file path (default: %ProgramData%\\ELNSSM\\config\\elnssm.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}
