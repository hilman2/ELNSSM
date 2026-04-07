package guardian

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const restartStateFile = "restart_state.json"

// restartState holds the PIDs of detached services during a graceful restart.
type restartState struct {
	Orphans map[string]int `json:"orphans"` // service ID -> PID
}

// restartStatePath returns the full path to the restart state file.
func restartStatePath(dataDir string) string {
	return filepath.Join(dataDir, "data", restartStateFile)
}

// writeRestartState persists the PID map so the next Guardian instance can re-adopt.
func writeRestartState(dataDir string, orphans map[string]int) error {
	state := restartState{Orphans: orphans}
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("marshaling restart state: %w", err)
	}

	path := restartStatePath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing restart state: %w", err)
	}
	return nil
}

// readRestartState loads the PID map from a previous restart. Returns nil if no state file exists.
func readRestartState(dataDir string) (map[string]int, error) {
	path := restartStatePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading restart state: %w", err)
	}

	var state restartState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing restart state: %w", err)
	}

	return state.Orphans, nil
}

// clearRestartState removes the state file after successful adoption.
func clearRestartState(dataDir string) {
	_ = os.Remove(restartStatePath(dataDir))
}
