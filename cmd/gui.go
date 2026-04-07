package cmd

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Open the web GUI in the default browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		addr := apiAddr()
		url := fmt.Sprintf("http://%s", addr)
		fmt.Printf("Opening %s ...\n", url)
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
