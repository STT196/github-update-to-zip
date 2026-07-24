package gitdiff

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/thisara/github-update-tracker/internal/zipper"
)

// CommitInfo holds minimal info for showing a commit in the picker.
type CommitInfo struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

// DiffMode selects what to diff against.
type DiffMode string

const (
	ModeUncommitted DiffMode = "uncommitted" // working tree vs HEAD
	ModeUnpushed    DiffMode = "unpushed"    // HEAD vs origin/<branch>
	ModeLastCommit  DiffMode = "last"        // HEAD~1 vs HEAD
	ModeRange       DiffMode = "range"       // user-picked from..to
	ModeBaseline    DiffMode = "baseline"    // file mod times vs snapshot (no git)
)

// LocalRepo wraps a local git repo path and provides diff operations.
type LocalRepo struct {
	Path   string
	Branch string // detected from repo
}

// GitAvailable reports whether git is on PATH.
func GitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// DetectBranch returns the current branch name.
func (r *LocalRepo) DetectBranch() (string, error) {
	out, err := r.output("rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	b := strings.TrimSpace(out)
	r.Branch = b
	return b, nil
}

// RecentCommits returns the last n commits on the current branch.
func (r *LocalRepo) RecentCommits(n int) ([]CommitInfo, error) {
	out, err := r.output("log", fmt.Sprintf("-%d", n), "--format=%H|%s|%an|%ai")
	if err != nil {
		return nil, err
	}
	var commits []CommitInfo
	sc := bufio.NewScanner(bytes.NewBufferString(out))
	for sc.Scan() {
		parts := strings.SplitN(sc.Text(), "|", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, CommitInfo{
			SHA:     parts[0],
			Message: parts[1],
			Author:  parts[2],
			Date:    parts[3],
		})
	}
	return commits, nil
}

// ChangedFiles returns files changed between two commits/trees.
// from can be "HEAD", "HEAD~1", "origin/main", a SHA, etc.
// to can be "HEAD", a SHA, or empty string (meaning working tree).
// mode is just informational.
func (r *LocalRepo) ChangedFiles(mode DiffMode, from, to string) ([]zipper.ChangedFile, error) {
	switch mode {
	case ModeUncommitted:
		// Use git status --porcelain to include untracked files,
		// not just tracked changes (git diff --name-status misses untracked).
		return r.diffPorcelainStatus()
	case ModeUnpushed:
		remote := "origin/" + r.Branch
		return r.diffNameStatus(remote, "HEAD")
	case ModeLastCommit:
		return r.diffNameStatus("HEAD~1", "HEAD")
	case ModeRange:
		return r.diffNameStatus(from, to)
	case ModeBaseline:
		return r.baselineDiff()
	default:
		return nil, fmt.Errorf("unknown diff mode: %s", mode)
	}
}

// diffNameStatus runs git diff --name-status <from> <to>.
// If "to" is empty, compares <from> to working tree.
func (r *LocalRepo) diffNameStatus(from, to string) ([]zipper.ChangedFile, error) {
	args := []string{"diff", "--name-status", from}
	if to != "" {
		args = append(args, to)
	}
	out, err := r.output(args...)
	if err != nil {
		return nil, err
	}

	var files []zipper.ChangedFile
	sc := bufio.NewScanner(bytes.NewBufferString(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		// git diff --name-status uses TAB as field delimiter.
		// Format: <status>\t<path>  or  <status>\t<old>\t<new> (rename)
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			continue
		}

		status := statusFromCode(fields[0])
		// The last field is always the path (for renames: new path)
		relPath := unquoteGitPath(fields[len(fields)-1])
		relPath = filepath.ToSlash(relPath)

		files = append(files, zipper.ChangedFile{
			Path:          relPath,
			Status:        status,
			SourceAbsPath: filepath.Join(r.Path, filepath.FromSlash(relPath)),
		})
	}
	return files, nil
}

// diffPorcelainStatus runs "git status --porcelain" and returns all
// staged, unstaged, and untracked files. Unlike git diff --name-status,
// this includes untracked (??) files which git diff does not report.
func (r *LocalRepo) diffPorcelainStatus() ([]zipper.ChangedFile, error) {
	out, err := r.output("status", "--porcelain")
	if err != nil {
		return nil, err
	}

	var files []zipper.ChangedFile
	sc := bufio.NewScanner(bytes.NewBufferString(out))
	for sc.Scan() {
		line := sc.Text()
		// porcelain format: "XY filename" (2 status chars + space + path)
		// For renamed: "R  old -> new" (score is in the status field)
		if len(line) < 4 {
			continue
		}

		statusField := strings.TrimSpace(line[:2])
		rest := strings.TrimSpace(line[3:])

		// Extract path (handle rename "old -> new")
		relPath := rest
		if idx := strings.Index(rest, " -> "); idx >= 0 {
			relPath = rest[idx+4:] // take the "new" path
		}

		// Unquote paths that git wrapped in double-quotes (files with spaces, etc.)
		relPath = unquoteGitPath(relPath)

		// Skip empty paths
		if relPath == "" {
			continue
		}

		status := porcelainStatus(statusField)
		files = append(files, zipper.ChangedFile{
			Path:          filepath.ToSlash(relPath),
			Status:        status,
			SourceAbsPath: filepath.Join(r.Path, filepath.FromSlash(relPath)),
		})
	}
	return files, nil
}

// unquoteGitPath handles git's C-style quoted paths.
// Git wraps paths in double quotes and uses backslash escapes
// when they contain special characters like spaces, tabs, etc.
// e.g. "file with spaces.txt" → file with spaces.txt
//      "file\\with\\backslashes.txt" → file\with\backslashes.txt
func unquoteGitPath(s string) string {
	if len(s) < 2 || s[0] != '"' {
		return s // not quoted
	}

	// Remove surrounding quotes and unescape
	s = s[1 : len(s)-1] // strip leading/trailing "

	var buf strings.Builder
	buf.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			// Handle C-style escape sequences
			switch s[i+1] {
			case 'n':
				buf.WriteByte('\n')
			case 't':
				buf.WriteByte('\t')
			case '\\':
				buf.WriteByte('\\')
			case '"':
				buf.WriteByte('"')
			default:
				// For octal escapes like \123, git uses them for non-printable chars
				// but for most practical cases, just keep the literal
				buf.WriteByte(s[i])
				buf.WriteByte(s[i+1])
			}
			i++
		} else {
			buf.WriteByte(s[i])
		}
	}
	return buf.String()
}

// porcelainStatus converts a git status --porcelain code to a human-readable status.
func porcelainStatus(xy string) string {
	// Index vs worktree: X=index(staging), Y=worktree
	// ?? = untracked, !! = ignored
	x := byte(' ')
	y := byte(' ')
	if len(xy) >= 1 {
		x = xy[0]
	}
	if len(xy) >= 2 {
		y = xy[1]
	}

	switch {
	case x == '?' && y == '?':
		return "added" // treat untracked as added
	case x == 'A' || y == 'A':
		return "added"
	case x == 'D' || y == 'D':
		return "deleted"
	case x == 'R' || y == 'R':
		return "renamed"
	case x == 'C' || y == 'C':
		return "added" // copied → treat as added
	case x == 'M' || y == 'M':
		return "modified"
	default:
		return "modified"
	}
}

func statusFromCode(code string) string {
	switch code[0] {
	case 'A':
		return "added"
	case 'M':
		return "modified"
	case 'D':
		return "deleted"
	case 'R':
		return "renamed"
	default:
		return "modified"
	}
}

// FileDiff returns the unified diff for a single file between from and to.
// from/to follow the same semantics as ChangedFiles.
func (r *LocalRepo) FileDiff(mode DiffMode, from, to, filePath string) (string, error) {
	var fromRef, toRef string
	switch mode {
	case ModeUncommitted:
		// For uncommitted mode, check if the file is untracked.
		// If untracked, show the entire file as added lines.
		if r.isUntracked(filePath) {
			return r.untrackedFileDiff(filePath)
		}
		fromRef = "HEAD"
		toRef = ""
	case ModeUnpushed:
		fromRef = "origin/" + r.Branch
		toRef = "HEAD"
	case ModeLastCommit:
		fromRef = "HEAD~1"
		toRef = "HEAD"
	case ModeRange:
		fromRef = from
		toRef = to
	}

	args := []string{"diff", fromRef}
	if toRef != "" {
		args = append(args, toRef)
	}
	args = append(args, "--", filePath)
	return r.output(args...)
}

// isUntracked returns true if the file is not tracked by git.
func (r *LocalRepo) isUntracked(filePath string) bool {
	err := r.run("ls-files", "--error-unmatch", "--", filePath)
	return err != nil
}

// untrackedFileDiff reads the file and returns a synthetic diff
// showing all lines as added (like git diff --no-index /dev/null <file>).
func (r *LocalRepo) untrackedFileDiff(filePath string) (string, error) {
	absPath := filepath.Join(r.Path, filepath.FromSlash(filePath))
	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("cannot read untracked file %s: %w", filePath, err)
	}

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("diff --git a/%s b/%s\n", filePath, filePath))
	buf.WriteString("new file mode 100644\n")
	buf.WriteString(fmt.Sprintf("--- /dev/null\n+++ b/%s\n", filePath))
	buf.WriteString("@@ -0,0 +1," + fmt.Sprintf("%d", countLines(data)) + " @@\n")

	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		buf.WriteString("+" + sc.Text() + "\n")
	}
	return buf.String(), nil
}

func countLines(data []byte) int {
	n := 0
	for _, b := range data {
		if b == '\n' {
			n++
		}
	}
	return n
}

func (r *LocalRepo) run(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return nil
}

func (r *LocalRepo) output(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path
	hideWindow(cmd)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}

// ── Baseline mode (no git required) ────────────────────────────────────

var ignoreDirs = map[string]bool{
	".git": true, "node_modules": true, ".svn": true, ".hg": true,
}

const baselineFile = ".gitzip-baseline.json"

type baselineEntry struct {
	Path    string    `json:"path"`
	ModTime time.Time `json:"mod_time"`
	Size    int64     `json:"size"`
}

// SaveBaseline walks the repo and saves file mod times for future comparison.
func (r *LocalRepo) SaveBaseline() error {
	var entries []baselineEntry

	err := filepath.WalkDir(r.Path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip inaccessible files
		}
		rel, _ := filepath.Rel(r.Path, p)
		parts := strings.Split(rel, string(filepath.Separator))

		// Skip ignored dirs
		for _, part := range parts {
			if ignoreDirs[part] {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		entries = append(entries, baselineEntry{
			Path:    filepath.ToSlash(rel),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
		return nil
	})
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(r.Path, baselineFile), b, 0o644)
}

// baselineDiff compares current file mod times against the stored baseline.
func (r *LocalRepo) baselineDiff() ([]zipper.ChangedFile, error) {
	data, err := os.ReadFile(filepath.Join(r.Path, baselineFile))
	if err != nil {
		return nil, fmt.Errorf("no baseline found — click 'Set baseline' first")
	}

	var baseline []baselineEntry
	if err := json.Unmarshal(data, &baseline); err != nil {
		return nil, fmt.Errorf("corrupt baseline: %w", err)
	}

	oldMap := make(map[string]baselineEntry)
	for _, e := range baseline {
		oldMap[e.Path] = e
	}

	// Walk current state
	currentMap := make(map[string]os.FileInfo)
	filepath.WalkDir(r.Path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(r.Path, p)
		parts := strings.Split(rel, string(filepath.Separator))
		for _, part := range parts {
			if ignoreDirs[part] {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		if !d.IsDir() {
			info, _ := d.Info()
			if info != nil {
				currentMap[filepath.ToSlash(rel)] = info
			}
		}
		return nil
	})

	var files []zipper.ChangedFile

	// Modified or added
	for path, info := range currentMap {
		old, existed := oldMap[path]
		if !existed {
			files = append(files, zipper.ChangedFile{
				Path:          path,
				Status:        "added",
				SourceAbsPath: filepath.Join(r.Path, filepath.FromSlash(path)),
			})
		} else if info.ModTime().After(old.ModTime) || info.Size() != old.Size {
			files = append(files, zipper.ChangedFile{
				Path:          path,
				Status:        "modified",
				SourceAbsPath: filepath.Join(r.Path, filepath.FromSlash(path)),
			})
		}
	}

	// Deleted
	for path := range oldMap {
		if _, exists := currentMap[path]; !exists {
			files = append(files, zipper.ChangedFile{Path: path, Status: "deleted"})
		}
	}

	return files, nil
}
