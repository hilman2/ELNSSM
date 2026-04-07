package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hilman2/ELNSSM/internal/buildinfo"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ELNSSM %s\n", buildinfo.Version)
		fmt.Printf("  Commit:  %s\n", buildinfo.Commit)
		fmt.Printf("  Built:   %s\n", buildinfo.BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
