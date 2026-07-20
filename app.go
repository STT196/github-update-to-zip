package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/thisara/github-update-tracker/internal/gitdiff"
	"github.com/thisara/github-update-tracker/internal/notify"
	"github.com/thisara/github-update-tracker/internal/zipper"
)

// App is the Wails-bound application struct.
type App struct {
	ctx       context.Context
	repo      *gitdiff.LocalRepo
	repoPath  string
	outputDir string
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {}

// ---- Bound methods ----

type SetRepoResult struct {
	Branch string `json:"branch"`
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
}

func (a *App) SetRepo(path string) SetRepoResult {
	a.repoPath = path
	a.repo = &gitdiff.LocalRepo{Path: path}
	branch, err := a.repo.DetectBranch()
	if err != nil {
		return SetRepoResult{OK: false, Error: err.Error()}
	}
	a.SaveRecentPath(path) // auto-save on successful load
	return SetRepoResult{Branch: branch, OK: true}
}

type ZipResult struct {
	OK      bool     `json:"ok"`
	ZipPath string   `json:"zip_path"`
	ZipName string   `json:"zip_name"`
	Files   []string `json:"files"`
	Count   int      `json:"count"`
	Error   string   `json:"error,omitempty"`
}

type ChangedFileInfo struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

// GetChangedFiles returns the list of changed files for the given mode.
func (a *App) GetChangedFiles(mode string, fromSHA string, toSHA string) ([]ChangedFileInfo, error) {
	if a.repo == nil {
		return nil, fmt.Errorf("no repo selected")
	}
	diffMode := gitdiff.DiffMode(mode)
	files, err := a.repo.ChangedFiles(diffMode, fromSHA, toSHA)
	if err != nil {
		return nil, err
	}
	var out []ChangedFileInfo
	for _, f := range files {
		out = append(out, ChangedFileInfo{Path: f.Path, Status: f.Status})
	}
	return out, nil
}

// GetFileDiff returns the unified diff for a single file.
func (a *App) GetFileDiff(mode string, fromSHA string, toSHA string, filePath string) (string, error) {
	if a.repo == nil {
		return "", fmt.Errorf("no repo selected")
	}
	diffMode := gitdiff.DiffMode(mode)
	return a.repo.FileDiff(diffMode, fromSHA, toSHA, filePath)
}

func (a *App) GenerateZip(mode string, fromSHA string, toSHA string, outputDir string, selectedFiles []string) ZipResult {
	if a.repo == nil {
		return ZipResult{OK: false, Error: "No repo selected. Set a repo path first."}
	}
	a.outputDir = outputDir

	diffMode := gitdiff.DiffMode(mode)
	allFiles, err := a.repo.ChangedFiles(diffMode, fromSHA, toSHA)
	if err != nil {
		return ZipResult{OK: false, Error: err.Error()}
	}

	// Build a set of selected file paths
	selected := make(map[string]bool)
	for _, p := range selectedFiles {
		selected[filepath.ToSlash(p)] = true
	}

	var toZip []zipper.ChangedFile
	for _, f := range allFiles {
		if f.Status == "deleted" {
			continue
		}
		if selected[f.Path] {
			toZip = append(toZip, f)
		}
	}

	if len(toZip) == 0 {
		return ZipResult{OK: true, Files: nil, Count: 0}
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ZipResult{OK: false, Error: err.Error()}
	}

	ts := time.Now().Format("20060102_150405")
	zipName := fmt.Sprintf("update_%s.zip", ts)
	zipPath := filepath.Join(outputDir, zipName)

	// Use the repo folder name as the root folder inside the zip
	rootName := filepath.Base(a.repoPath)
	if err := zipper.BuildZip(toZip, zipPath, rootName); err != nil {
		return ZipResult{OK: false, Error: err.Error()}
	}

	var names []string
	for _, f := range toZip {
		names = append(names, f.Path)
	}

	notify.Send("Zip ready",
		fmt.Sprintf("%d files → %s", len(names), zipName))

	return ZipResult{
		OK:      true,
		ZipPath: zipPath,
		ZipName: zipName,
		Files:   names,
		Count:   len(names),
	}
}

func (a *App) GetRecentCommits(count int) ([]gitdiff.CommitInfo, error) {
	if a.repo == nil {
		return nil, fmt.Errorf("no repo selected")
	}
	return a.repo.RecentCommits(count)
}

func (a *App) GitAvailable() bool {
	return gitdiff.GitAvailable()
}

func (a *App) SaveBaseline() error {
	if a.repo == nil {
		return fmt.Errorf("no repo selected")
	}
	return a.repo.SaveBaseline()
}

func (a *App) HasBaseline() bool {
	if a.repo == nil {
		return false
	}
	_, err := os.Stat(filepath.Join(a.repoPath, ".gitzip-baseline.json"))
	return err == nil
}

func (a *App) OpenFolder(path string) error {
	return openInExplorer(path)
}

func (a *App) PickFolder() (string, error) {
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select folder",
	})
}

// ── Recent paths ────────────────────────────────────────────────────────

type RecentEntry struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
}

func recentPath() string {
	dir, _ := os.UserConfigDir()
	p := filepath.Join(dir, "GitHubUpdateTracker")
	os.MkdirAll(p, 0o755)
	return filepath.Join(p, "recent.json")
}

func (a *App) SaveRecentPath(path string) error {
	p := recentPath()
	var entries []RecentEntry

	data, _ := os.ReadFile(p)
	json.Unmarshal(data, &entries) // ignore errors, start fresh on corrupt file

	// Remove duplicate
	filtered := entries[:0]
	for _, e := range entries {
		if !strings.EqualFold(e.Path, path) {
			filtered = append(filtered, e)
		}
	}

	entry := RecentEntry{Path: path}
	if a.repo != nil && a.repo.Branch != "" {
		entry.Branch = a.repo.Branch
	}

	// Prepend, cap at 10
	entries = append([]RecentEntry{entry}, filtered...)
	if len(entries) > 10 {
		entries = entries[:10]
	}

	b, _ := json.MarshalIndent(entries, "", "  ")
	return os.WriteFile(p, b, 0o644)
}

func (a *App) GetRecentPaths() []RecentEntry {
	data, err := os.ReadFile(recentPath())
	if err != nil {
		return nil
	}
	var entries []RecentEntry
	json.Unmarshal(data, &entries)
	return entries
}

func (a *App) RemoveRecentPath(path string) error {
	p := recentPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	var entries []RecentEntry
	json.Unmarshal(data, &entries)

	filtered := entries[:0]
	for _, e := range entries {
		if !strings.EqualFold(e.Path, path) {
			filtered = append(filtered, e)
		}
	}
	b, _ := json.MarshalIndent(filtered, "", "  ")
	return os.WriteFile(p, b, 0o644)
}
