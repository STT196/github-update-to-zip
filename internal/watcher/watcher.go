package watcher

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thisara/github-update-tracker/internal/config"
	"github.com/thisara/github-update-tracker/internal/gitdiff"
	"github.com/thisara/github-update-tracker/internal/notify"
	"github.com/thisara/github-update-tracker/internal/state"
	"github.com/thisara/github-update-tracker/internal/zipper"
)

// UpdateResult summarizes the outcome of a single check.
type UpdateResult struct {
	Changed bool     `json:"changed"`
	SHA     string   `json:"sha"`
	ZipPath string   `json:"zip_path"`
	Files   []string `json:"files"`
}

// Watcher polls one repo on an interval and produces update zips.
type Watcher struct {
	ctx      context.Context
	repo     config.RepoConfig
	cancel   chan struct{}
	cloneDir string
}

func NewWatcher(ctx context.Context, repo config.RepoConfig) *Watcher {
	return &Watcher{
		ctx:      ctx,
		repo:     repo,
		cancel:   make(chan struct{}),
		cloneDir: filepath.Join(os.TempDir(), "gh-update-tracker", repo.ID),
	}
}

func (w *Watcher) Run() {
	interval := time.Duration(w.repo.PollSeconds) * time.Second
	if interval <= 0 {
		interval = 60 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.cancel:
			return
		case <-ticker.C:
			_, _ = w.CheckOnce()
		}
	}
}

func (w *Watcher) Stop() {
	close(w.cancel)
}

// ensureClone clones the repo if not already present, or pulls latest changes.
func (w *Watcher) ensureClone() error {
	if _, err := os.Stat(filepath.Join(w.cloneDir, ".git")); os.IsNotExist(err) {
		// Clone fresh
		cloneURL := fmt.Sprintf("https://%s@github.com/%s/%s.git", w.repo.GitHubToken, w.repo.Owner, w.repo.Repo)
		if w.repo.GitHubToken == "" {
			cloneURL = fmt.Sprintf("https://github.com/%s/%s.git", w.repo.Owner, w.repo.Repo)
		}
		cmd := exec.Command("git", "clone", "--branch", w.repo.Branch, cloneURL, w.cloneDir)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("clone failed: %w\n%s", err, string(out))
		}
		return nil
	}
	// Pull latest
	cmd := exec.Command("git", "-C", w.cloneDir, "fetch", "origin", w.repo.Branch)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("fetch failed: %w\n%s", err, string(out))
	}
	cmd = exec.Command("git", "-C", w.cloneDir, "reset", "--hard", "origin/"+w.repo.Branch)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err = cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reset failed: %w\n%s", err, string(out))
	}
	return nil
}

// latestSHA returns the current HEAD SHA of the cloned repo.
func (w *Watcher) latestSHA() (string, error) {
	cmd := exec.Command("git", "-C", w.cloneDir, "rev-parse", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// CheckOnce runs a single detect -> diff -> zip cycle.
func (w *Watcher) CheckOnce() (UpdateResult, error) {
	if !gitdiff.GitAvailable() {
		return UpdateResult{}, fmt.Errorf("git not available on PATH")
	}

	st, err := state.Load(w.repo.ID)
	if err != nil {
		return UpdateResult{}, err
	}

	if err := w.ensureClone(); err != nil {
		return UpdateResult{}, err
	}

	newSHA, err := w.latestSHA()
	if err != nil {
		return UpdateResult{}, err
	}

	if newSHA == st.LastSHA {
		return UpdateResult{Changed: false}, nil
	}
	if st.LastSHA == "" {
		// first run: nothing to diff against yet, just record HEAD
		st.LastSHA = newSHA
		_ = state.Save(w.repo.ID, st)
		return UpdateResult{Changed: false}, nil
	}

	// Diff using LocalRepo on the cloned directory
	lr := &gitdiff.LocalRepo{Path: w.cloneDir}
	changedFiles, err := lr.ChangedFiles(gitdiff.ModeRange, st.LastSHA, newSHA)
	if err != nil {
		return UpdateResult{}, err
	}

	if len(changedFiles) == 0 {
		st.LastSHA = newSHA
		_ = state.Save(w.repo.ID, st)
		return UpdateResult{Changed: false}, nil
	}

	if err := os.MkdirAll(w.repo.OutputDir, 0o755); err != nil {
		return UpdateResult{}, err
	}
	shortSHA := newSHA
	if len(shortSHA) > 7 {
		shortSHA = shortSHA[:7]
	}
	zipName := fmt.Sprintf("update_%s_%s.zip", time.Now().Format("20060102_150405"), shortSHA)
	zipPath := filepath.Join(w.repo.OutputDir, zipName)

	if err := zipper.BuildZip(changedFiles, zipPath, ""); err != nil {
		return UpdateResult{}, err
	}

	var fileNames []string
	for _, f := range changedFiles {
		fileNames = append(fileNames, f.Path)
	}

	_ = state.AppendHistory(w.repo.ID, newSHA, state.HistoryEntry{
		SHA:   shortSHA,
		Zip:   zipName,
		Files: fileNames,
		Time:  time.Now().Format(time.RFC3339),
	})
	st.LastSHA = newSHA
	_ = state.Save(w.repo.ID, st)

	notify.Send("GitHub Update Tracker", fmt.Sprintf("%s/%s: %d files changed\n%s", w.repo.Owner, w.repo.Repo, len(fileNames), shortSHA))

	return UpdateResult{Changed: true, SHA: shortSHA, ZipPath: zipPath, Files: fileNames}, nil
}
