package watcher

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	ctx    context.Context
	repo   config.RepoConfig
	cancel chan struct{}
}

func NewWatcher(ctx context.Context, repo config.RepoConfig) *Watcher {
	return &Watcher{ctx: ctx, repo: repo, cancel: make(chan struct{})}
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

// CheckOnce runs a single detect -> diff -> zip cycle.
// This is the core loop described in the design: detect new SHA,
// diff old vs new, zip the changed files with their paths.
func (w *Watcher) CheckOnce() (UpdateResult, error) {
	useGit := w.repo.Mode == "git" && gitdiff.GitAvailable()

	st, err := state.Load(w.repo.ID)
	if err != nil {
		return UpdateResult{}, err
	}

	var newSHA string
	var changedFiles []zipper.ChangedFile

	if useGit {
		g := &gitdiff.LocalGit{
			Owner:    w.repo.Owner,
			Repo:     w.repo.Repo,
			Branch:   w.repo.Branch,
			CloneDir: filepath.Join(os.TempDir(), "gh-update-tracker", w.repo.ID),
			Token:    w.repo.GitHubToken,
		}
		if err := g.EnsureClone(); err != nil {
			return UpdateResult{}, err
		}
		newSHA, err = g.LatestRemoteSHA()
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
		changedFiles, err = g.DiffFiles(st.LastSHA, newSHA)
		if err != nil {
			return UpdateResult{}, err
		}
		if err := g.CheckoutNew(newSHA); err != nil {
			return UpdateResult{}, err
		}
	} else {
		a := &gitdiff.GitHubAPI{
			Owner:   w.repo.Owner,
			Repo:    w.repo.Repo,
			Branch:  w.repo.Branch,
			Token:   w.repo.GitHubToken,
			TempDir: filepath.Join(os.TempDir(), "gh-update-tracker-api", w.repo.ID),
		}
		newSHA, err = a.LatestRemoteSHA()
		if err != nil {
			return UpdateResult{}, err
		}
		if newSHA == st.LastSHA {
			return UpdateResult{Changed: false}, nil
		}
		if st.LastSHA == "" {
			st.LastSHA = newSHA
			_ = state.Save(w.repo.ID, st)
			return UpdateResult{Changed: false}, nil
		}
		changedFiles, err = a.DiffFiles(newSHA)
		if err != nil {
			return UpdateResult{}, err
		}
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

	if err := zipper.BuildZip(changedFiles, zipPath); err != nil {
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

	notify.Send("New update: "+w.repo.Owner+"/"+w.repo.Repo,
		fmt.Sprintf("%d files changed -> %s", len(fileNames), zipName))

	return UpdateResult{Changed: true, SHA: shortSHA, ZipPath: zipPath, Files: fileNames}, nil
}
