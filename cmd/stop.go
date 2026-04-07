package cmd

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <name>",
	Short: "Stop a managed service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		resp, err := apiRequest("POST", fmt.Sprintf("/api/v1/services/%s/stop", name), strings.NewReader(""))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return readAPIError(resp)
		}

		fmt.Printf("Service %q stopped.\n", name)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
