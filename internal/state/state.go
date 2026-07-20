package state

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// HistoryEntry records one completed update (one zip produced).
type HistoryEntry struct {
	SHA   string   `json:"sha"`
	Zip   string   `json:"zip"`
	Files []string `json:"files"`
	Time  string   `json:"time"`
}

// RepoState is the persisted state for a single watched repo.
type RepoState struct {
	LastSHA     string         `json:"last_sha"`
	LastChecked string         `json:"last_checked"`
	History     []HistoryEntry `json:"history"`
}

func statePath(repoID string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	stateDir := filepath.Join(dir, "GitHubUpdateTracker", "state")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(stateDir, repoID+".json"), nil
}

func Load(repoID string) (*RepoState, error) {
	p, err := statePath(repoID)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return &RepoState{History: []HistoryEntry{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var s RepoState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func Save(repoID string, s *RepoState) error {
	p, err := statePath(repoID)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func LoadHistory(repoID string) ([]HistoryEntry, error) {
	s, err := Load(repoID)
	if err != nil {
		return nil, err
	}
	return s.History, nil
}

// AppendHistory adds a new entry and updates LastSHA in one write.
func AppendHistory(repoID, newSHA string, entry HistoryEntry) error {
	s, err := Load(repoID)
	if err != nil {
		return err
	}
	s.LastSHA = newSHA
	s.History = append([]HistoryEntry{entry}, s.History...)
	if len(s.History) > 50 {
		s.History = s.History[:50]
	}
	return Save(repoID, s)
}
