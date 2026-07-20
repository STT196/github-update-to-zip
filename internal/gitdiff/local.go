package gitdiff

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

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
		return r.diffNameStatus("HEAD", "") // working tree vs HEAD
	case ModeUnpushed:
		remote := "origin/" + r.Branch
		return r.diffNameStatus(remote, "HEAD")
	case ModeLastCommit:
		return r.diffNameStatus("HEAD~1", "HEAD")
	case ModeRange:
		return r.diffNameStatus(from, to)
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
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		status := statusFromCode(parts[0])
		// The path is the last field (handles rename lines like "R100 old new")
		relPath := filepath.ToSlash(parts[len(parts)-1])

		files = append(files, zipper.ChangedFile{
			Path:          relPath,
			Status:        status,
			SourceAbsPath: filepath.Join(r.Path, filepath.FromSlash(relPath)),
		})
	}
	return files, nil
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

func (r *LocalRepo) run(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Path
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
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}
