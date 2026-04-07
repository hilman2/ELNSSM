package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"

	"github.com/hilman2/ELNSSM/internal/config"
)

var resetTokenCmd = &cobra.Command{
	Use:   "reset-token",
	Short: "Generate a new API authentication token",
	Long:  `Generates a new random API token, stores its bcrypt hash in the config, and prints the plaintext token once.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgPath := cfgFile
		if cfgPath == "" {
			cfgPath = config.DefaultConfigPath()
		}

		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		token, err := generateToken()
		if err != nil {
			return fmt.Errorf("generating token: %w", err)
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hashing token: %w", err)
		}

		cfg.API.Auth.Enabled = true
		cfg.API.Auth.Type = "token"
		cfg.API.Auth.TokenHash = string(hash)

		if err := cfg.Save(cfgPath); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		slog.Info("New API token generated", "config", cfgPath)
		fmt.Println("New API token (save this now, it will not be shown again):")
		fmt.Println(token)
		return nil
	},
}

// generateToken creates a cryptographically random 32-byte hex token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func init() {
	rootCmd.AddCommand(resetTokenCmd)
}
