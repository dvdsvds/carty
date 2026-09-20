package state

import (
	"encoding/json"
	"os"
)

type Entry struct {
	ItemPath   string `json:"item_path"`
	Item       string `json:"item"`
	Version    string `json:"version"`
	TargetFile string `json:"target_file"`
}

type State map[string]Entry

func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}

	return s, nil
}

// LoadOrEmpty behaves like Load but returns an empty State instead of an
// error when the state file doesn't exist yet.
func LoadOrEmpty(path string) (State, error) {
	s, err := Load(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(State), nil
		}
		return nil, err
	}
	return s, nil
}

func Save(path string, s State) error {
	data, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
