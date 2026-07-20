# Git Update Zipper

Simple Windows desktop tool: select a local git repo, pick what changed files to include, click a button → get a zip with full files and their paths preserved. Ready for upload to your server.

## What it does
- Point it at any local git repo
- Choose what to zip:
  - **Uncommitted changes** — files modified but not committed
  - **Unpushed commits** — all changes in commits not yet pushed
  - **Last commit only** — just the most recent commit
  - **Select commits** — pick a range of commits to combine
- Produces `update_<timestamp>.zip` with all changed files, preserving their directory paths

## Stack
- Go + Wails v2 (native Windows window)
- Plain HTML/CSS/JS frontend
- Local `git` CLI for diffing

## Prerequisites
- Go 1.22+
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- `git` on PATH

## Run in dev mode
```
wails dev
```

## Build
```
wails build -platform windows/amd64
```
Output: `build/bin/github-update-tracker.exe`
