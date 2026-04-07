package cmd

import (
	"fmt"
	"net/http"

	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a managed service",
	Long:  `Removes a service from ELNSSM management. The service must be stopped first.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		resp, err := apiRequest("DELETE", fmt.Sprintf("/api/v1/services/%s", name), nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return readAPIError(resp)
		}

		fmt.Printf("Service %q removed.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
