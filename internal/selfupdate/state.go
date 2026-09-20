package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func lastCheckPath(stateDir string) string {
	return filepath.Join(stateDir, "selfupdate_last_check")
}

// ShouldCheck reports whether CheckInterval has elapsed since the last
// recorded check (or none was ever recorded).
func ShouldCheck(stateDir string) bool {
	data, err := os.ReadFile(lastCheckPath(stateDir))
	if err != nil {
		return true
	}
	t, err := time.Parse(time.RFC3339, string(data))
	if err != nil {
		return true
	}
	return time.Since(t) >= CheckInterval
}

func recordChecked(stateDir string) {
	_ = os.WriteFile(lastCheckPath(stateDir), []byte(time.Now().Format(time.RFC3339)), 0644)
}

type PendingChangelog struct {
	Version string `json:"version"`
	Body    string `json:"body"`
}

func pendingChangelogPath(stateDir string) string {
	return filepath.Join(stateDir, "pending_changelog.json")
}

func SavePendingChangelog(stateDir, version, body string) error {
	data, err := json.Marshal(PendingChangelog{Version: version, Body: body})
	if err != nil {
		return err
	}
	return os.WriteFile(pendingChangelogPath(stateDir), data, 0644)
}

// LoadPendingChangelog returns the changelog recorded by the last
// self-update, if any, and clears it so it's only shown once.
func LoadPendingChangelog(stateDir string) (*PendingChangelog, error) {
	path := pendingChangelogPath(stateDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var pc PendingChangelog
	if err := json.Unmarshal(data, &pc); err != nil {
		return nil, err
	}

	_ = os.Remove(path)
	return &pc, nil
}
