package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hilman2/ELNSSM/internal/config"
	"github.com/hilman2/ELNSSM/internal/model"
)

var (
	addWorkingDir  string
	addStartupType string
	addDescription string
	addEnvVars     []string
)

var addCmd = &cobra.Command{
	Use:   "add <name> <executable> [args...]",
	Short: "Register a new managed service",
	Long:  `Adds a new service to be managed by ELNSSM. The service will not be started automatically unless --startup-type is set to "auto".`,
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		executable := args[1]
		svcArgs := args[2:]

		env := make(map[string]string)
		for _, e := range addEnvVars {
			parts := strings.SplitN(e, "=", 2)
			if len(parts) == 2 {
				env[parts[0]] = parts[1]
			}
		}

		svc := &model.Service{
			ID:          name,
			Name:        name,
			DisplayName: name,
			Description: addDescription,
			Executable:  executable,
			Arguments:   svcArgs,
			WorkingDir:  addWorkingDir,
			Environment: env,
			StartupType: model.StartupType(addStartupType),
		}

		body, _ := json.Marshal(svc)
		resp, err := apiRequest("POST", "/api/v1/services", strings.NewReader(string(body)))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			return readAPIError(resp)
		}

		fmt.Printf("Service %q added successfully.\n", name)
		return nil
	},
}

func init() {
	addCmd.Flags().StringVarP(&addWorkingDir, "working-dir", "d", "", "working directory for the service")
	addCmd.Flags().StringVar(&addStartupType, "startup-type", "manual", "startup type: auto, manual, disabled, delayed-auto")
	addCmd.Flags().StringVar(&addDescription, "description", "", "service description")
	addCmd.Flags().StringArrayVarP(&addEnvVars, "env", "e", nil, "environment variables (KEY=VALUE)")
	rootCmd.AddCommand(addCmd)
}

// apiRequest makes an HTTP request to the Guardian API.
func apiRequest(method, path string, body *strings.Reader) (*http.Response, error) {
	addr := apiAddr()
	url := fmt.Sprintf("http://%s%s", addr, path)

	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(method, url, body)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connecting to Guardian at %s: %w\nIs the ELNSSM Guardian running?", addr, err)
	}
	return resp, nil
}

func apiAddr() string {
	// Try to load from config, fall back to default
	cfg := config.DefaultConfig()
	return cfg.API.Listen
}

func readAPIError(resp *http.Response) error {
	var result struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	return fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
}
