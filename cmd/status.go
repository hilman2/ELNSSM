package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/hilman2/ELNSSM/internal/model"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status [name]",
	Short: "Show status of services",
	Long:  `Shows the status of a specific service, or all services if no name is given.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return showServiceStatus(args[0])
		}
		return showAllStatus()
	},
}

func showServiceStatus(name string) error {
	resp, err := apiRequest("GET", fmt.Sprintf("/api/v1/services/%s", name), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return readAPIError(resp)
	}

	var result struct {
		Data model.Service `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	svc := result.Data
	fmt.Printf("Service:     %s\n", svc.Name)
	fmt.Printf("State:       %s\n", svc.State)
	fmt.Printf("PID:         %d\n", svc.PID)
	fmt.Printf("Executable:  %s\n", svc.Executable)
	if svc.StartedAt != nil {
		fmt.Printf("Started At:  %s\n", svc.StartedAt.Format("2006-01-02 15:04:05"))
		fmt.Printf("Uptime:      %s\n", svc.Uptime)
	}
	fmt.Printf("Restarts:    %d\n", svc.RestartCount)
	if svc.LastExitCode != 0 {
		fmt.Printf("Last Exit:   %d\n", svc.LastExitCode)
	}
	if svc.LastError != "" {
		fmt.Printf("Last Error:  %s\n", svc.LastError)
	}
	return nil
}

func showAllStatus() error {
	resp, err := apiRequest("GET", "/api/v1/services", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return readAPIError(resp)
	}

	var result struct {
		Data []model.Service `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	if len(result.Data) == 0 {
		fmt.Println("No services registered.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATE\tPID\tUPTIME\tRESTARTS")
	for _, svc := range result.Data {
		uptime := "-"
		if svc.State == model.ServiceStateRunning && svc.StartedAt != nil {
			uptime = svc.Uptime.String()
		}
		pid := "-"
		if svc.PID > 0 {
			pid = fmt.Sprintf("%d", svc.PID)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n", svc.Name, svc.State, pid, uptime, svc.RestartCount)
	}
	w.Flush()
	return nil
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
