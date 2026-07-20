package watcher

import (
	"context"
	"sync"

	"github.com/thisara/github-update-tracker/internal/config"
)

// Manager supervises one Watcher goroutine per repo.
type Manager struct {
	ctx      context.Context
	cfg      *config.Store
	mu       sync.Mutex
	watchers map[string]*Watcher
}

func NewManager(ctx context.Context, cfg *config.Store) *Manager {
	return &Manager{
		ctx:      ctx,
		cfg:      cfg,
		watchers: make(map[string]*Watcher),
	}
}

// ResumeAll starts watching for every repo in config (call on app startup).
func (m *Manager) ResumeAll() {
	for _, r := range m.cfg.ListRepos() {
		_ = m.Start(r.ID)
	}
}

func (m *Manager) Start(repoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.watchers[repoID]; exists {
		return nil // already running
	}
	repo, err := m.cfg.GetRepo(repoID)
	if err != nil {
		return err
	}

	w := NewWatcher(m.ctx, repo)
	m.watchers[repoID] = w
	go w.Run()
	return nil
}

func (m *Manager) Stop(repoID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if w, ok := m.watchers[repoID]; ok {
		w.Stop()
		delete(m.watchers, repoID)
	}
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, w := range m.watchers {
		w.Stop()
		delete(m.watchers, id)
	}
}

func (m *Manager) IsRunning(repoID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.watchers[repoID]
	return exists
}

func (m *Manager) CheckNow(repoID string) (UpdateResult, error) {
	m.mu.Lock()
	w, exists := m.watchers[repoID]
	m.mu.Unlock()

	if !exists {
		repo, err := m.cfg.GetRepo(repoID)
		if err != nil {
			return UpdateResult{}, err
		}
		w = NewWatcher(m.ctx, repo)
	}
	return w.CheckOnce()
}
