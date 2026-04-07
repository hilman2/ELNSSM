package cmd

import (
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed services",
	RunE: func(cmd *cobra.Command, args []string) error {
		return showAllStatus()
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
