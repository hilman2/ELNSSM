package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/guardian"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install ELNSSM Guardian as a Windows service",
	Long:  `Registers the ELNSSM Guardian as a Windows service that manages all child services. Requires administrator privileges.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		slog.Info("Installing ELNSSM Guardian service...")

		if err := guardian.Install(); err != nil {
			return fmt.Errorf("installation failed: %w", err)
		}

		// Create default config if it doesn't exist
		cfgPath := config.DefaultConfigPath()
		cfg := config.DefaultConfig()

		// Generate API auth token
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return fmt.Errorf("generating token: %w", err)
		}
		token := hex.EncodeToString(tokenBytes)

		hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hashing token: %w", err)
		}
		cfg.API.Auth.Enabled = true
		cfg.API.Auth.Type = "token"
		cfg.API.Auth.TokenHash = string(hash)

		if err := cfg.Save(cfgPath); err != nil {
			slog.Warn("Could not write default config", "path", cfgPath, "error", err)
		} else {
			slog.Info("Default config written", "path", cfgPath)
		}

		// Create data directories
		for _, dir := range []string{cfg.ServicesDir(), cfg.LogsDir()} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				slog.Warn("Could not create directory", "path", dir, "error", err)
			}
		}

		fmt.Println("ELNSSM Guardian installed successfully.")
		fmt.Println("Start the service with: elnssm start-guardian  (or: sc start ELNSSM)")
		fmt.Println()
		fmt.Println("API Authentication Token (save this now, it will not be shown again):")
		fmt.Println(token)
		fmt.Println()
		fmt.Println("If you lose this token, run: elnssm reset-token")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove ELNSSM Guardian Windows service",
	Long:  `Unregisters the ELNSSM Guardian Windows service. Data files are preserved.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		slog.Info("Uninstalling ELNSSM Guardian service...")

		if err := guardian.Uninstall(); err != nil {
			return fmt.Errorf("uninstall failed: %w", err)
		}

		fmt.Println("ELNSSM Guardian service removed.")
		fmt.Println("Data files in %ProgramData%\\ELNSSM have been preserved.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
}
