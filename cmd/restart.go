package cmd

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart <name>",
	Short: "Restart a managed service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		resp, err := apiRequest("POST", fmt.Sprintf("/api/v1/services/%s/restart", name), strings.NewReader(""))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return readAPIError(resp)
		}

		fmt.Printf("Service %q restarted.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
