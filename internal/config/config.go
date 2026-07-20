package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/google/uuid"
)

// RepoConfig describes one watched repository.
type RepoConfig struct {
	ID          string `json:"id"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Branch      string `json:"branch"`
	Mode        string `json:"mode"` // "git" or "api"
	PollSeconds int    `json:"poll_seconds"`
	OutputDir   string `json:"output_dir"`
	GitHubToken string `json:"github_token,omitempty"`
}

type fileFormat struct {
	Repos []RepoConfig `json:"repos"`
}

// Store is a thread-safe, JSON-file-backed config store.
type Store struct {
	mu   sync.Mutex
	path string
	data fileFormat
}

func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	appDir := filepath.Join(dir, "GitHubUpdateTracker")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.json"), nil
}

func New() *Store {
	p, _ := configPath()
	return &Store{path: p, data: fileFormat{Repos: []RepoConfig{}}}
}

func Load() (*Store, error) {
	p, err := configPath()
	if err != nil {
		return nil, err
	}
	s := &Store{path: p}
	b, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		s.data = fileFormat{Repos: []RepoConfig{}}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) save() error {
	b, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}

func (s *Store) AddRepo(cfg RepoConfig) (RepoConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cfg.ID == "" {
		cfg.ID = uuid.NewString()
	}
	if cfg.Branch == "" {
		cfg.Branch = "main"
	}
	if cfg.Mode == "" {
		cfg.Mode = "git"
	}
	if cfg.PollSeconds <= 0 {
		cfg.PollSeconds = 60
	}
	s.data.Repos = append(s.data.Repos, cfg)
	return cfg, s.save()
}

func (s *Store) RemoveRepo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := s.data.Repos[:0]
	for _, r := range s.data.Repos {
		if r.ID != id {
			out = append(out, r)
		}
	}
	s.data.Repos = out
	return s.save()
}

func (s *Store) ListRepos() []RepoConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]RepoConfig{}, s.data.Repos...)
}

func (s *Store) GetRepo(id string) (RepoConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.data.Repos {
		if r.ID == id {
			return r, nil
		}
	}
	return RepoConfig{}, os.ErrNotExist
}
