# Git Branch Manager (`test-branch-manager`)

A production-ready, interactive command-line interface (CLI) written in Go designed to automate Git branch management workflows for offshore and IT test deployments.

---

## Table of Contents

- [Overview](#overview)
- [Key Features](#key-features)
- [Workflow Architecture](#workflow-architecture)
  - [Branch Naming Conventions](#branch-naming-conventions)
  - [Phase A: Offshore Branch Preparation](#phase-a-offshore-branch-preparation)
  - [Phase B: Offshore Branch Merges & Push](#phase-b-offshore-branch-merges--push)
  - [Phase C: IT Test Branch Creation & Push](#phase-c-it-test-branch-creation--push)
- [Conflict Resolution & Git Safety](#conflict-resolution--git-safety)
- [Command Execution History & Exit Summary](#command-execution-history--exit-summary)
- [Resumability & Recovery](#resumability--recovery)
- [Installation & Requirements](#installation--requirements)
- [Usage & Flags](#usage--flags)
- [Example Workflow Run](#example-workflow-run)
- [Building & Cross-Compilation](#building--cross-compilation)
- [Running Automated Tests](#running-automated-tests)
- [Troubleshooting](#troubleshooting)

---

## Overview

The `test-branch-manager` CLI standardizes and automates the multi-step process of preparing staging and offshore test branches. Instead of relying on brittle manual Git commands or unmaintained shell scripts, `test-branch-manager` executes commands through an audited execution engine that records every interaction, verifies repository state at every transition, detects and guides conflict resolution, and guarantees safe recovery on exit.

---

## Key Features

- **Robust Git Execution Engine**: Uses standard library `os/exec` with discrete arguments (no shell injection risks).
- **Comprehensive Command History**: Automatically captures sequence numbers, commands, arguments, working directories, execution durations, exit codes, and status. This summary is displayed on **every exit** (success, error, Ctrl+C, or user cancellation).
- **Interactive Terminal UI**: Keyboard-friendly selection menus with fuzzy search, color-coded indicators, and clear phase demarcations (with fallback for non-TTY/accessible modes).
- **Zero Heavy Dependencies**: Pure Go implementation using only Go standard libraries with native Windows console ANSI enablement. No external runtimes (Node.js, Python, etc.) required.
- **Strict Git Safety Validations**:
  - Validates repository cleanliness (no uncommitted or unmerged changes).
  - Checks for detached `HEAD`, rebases, or in-progress merges.
  - Verifies presence of remote base branches (`develop`).
  - Guards against accidental overwrites or force pushes.
- **First-Class Conflict Resolution**:
  - Accurately checks index unmerged entries (`diff --diff-filter=U` and unmerged status codes).
  - Pauses to allow developers to resolve conflicts using their preferred tools or IDE.
  - Automatically verifies that all conflicts are resolved and merge commits completed before proceeding.
- **Cross-Platform Support**: Compiles to standalone native binaries for Windows (`amd64`), macOS Intel (`amd64`), macOS Apple Silicon (`arm64`), and Linux (`amd64`).

---

## Workflow Architecture

### Branch Naming Conventions

The workflow uses the system's local calendar date dynamically generated at runtime in `YYYYMMDD` format:

- **Offshore test branch**: `test/offshore-test-YYYYMMDD` (e.g. `test/offshore-test-20261021`)
- **IT test branch**: `test/it-YYYYMMDD` (e.g. `test/it-20261021`)

The date is evaluated once at application startup to guarantee consistency across both branch names. If a branch already exists locally or remotely, `test-branch-manager` alerts the user and gives the option to cancel safely or select a new suffix.

### Phase A: Offshore Branch Preparation

1. **Repository Discovery & Selection**: Scans the current working directory and immediate child folders for valid Git working trees (supports `.git` directories and `.git` worktree pointer files).
2. **Safety Validation**: Confirms working directory is clean, HEAD is attached, and no rebase/merge is in progress.
3. **Remote Configuration**: Resolves the configured remote (default `origin` or user-selected).
4. **Fetch & Source Branch Selection**: Fetches remote branches and displays an interactive searchable menu for the source feature/release branch.
5. **Develop Verification**: Verifies `{remote}/develop` exists as a base.
6. **Branch Creation**: Creates `test/offshore-test-YYYYMMDD` tracking `{remote}/develop` and checks it out.
7. **Optional IT Branch Integration**: Prompts if an existing IT test branch (matching `test/it-*`) should also be merged.

### Phase B: Offshore Branch Merges & Push

1. **Merge Source Branch**: Merges the selected source branch into the offshore branch sequentially.
2. **Merge Existing IT Branch (Optional)**: If selected in Phase A, merges the existing IT branch.
3. **Interactive Conflict Handling**: If conflicts arise at any point, guides resolution loop (see below).
4. **Remote Push**: Confirms and pushes `test/offshore-test-YYYYMMDD` to the remote.

### Phase C: IT Test Branch Creation & Push

1. **Create Base IT Branch**: Creates `test/it-YYYYMMDD` based directly on the latest `{remote}/develop`. *(Note: The IT branch is based on `develop`, NOT branched from the offshore branch directly).*
2. **Checkout & Merge**: Checks out `test/it-YYYYMMDD` and merges the freshly created offshore branch into it.
3. **Conflict Resolution**: Executes conflict resolution loop if discrepancies exist between develop and the offshore changes.
4. **Push IT Branch**: Pushes `test/it-YYYYMMDD` to the remote.
5. **Completion Summary**: Displays an executive summary containing the repository, source branch, offshore branch, IT test branch, and remote.

---

## Conflict Resolution & Git Safety

`test-branch-manager` does **not** rely on fragile error message regexes. It queries Git's plumbing state:
- Tests for unmerged index entries using `git diff --name-only --diff-filter=U`.
- Validates status codes (`UU`, `AA`, `DD`, etc.) via `git status --porcelain`.
- Inspects Git directory `MERGE_HEAD` state.

When conflicts occur:
1. The tool outputs a list of conflicted files.
2. It pauses execution while preserving your working tree and Git state.
3. The developer resolves conflicts in their IDE or text editor and stages files (`git add`).
4. Upon pressing Continue, `test-branch-manager` verifies if any unmerged paths remain.
5. If clean, it completes the merge commit (`git commit --no-edit`) and proceeds.

> **Safety Notice:** If you cancel or interrupt during conflict resolution, `test-branch-manager` **never** discards your work with `git merge --abort`. It exits cleanly and prints step-by-step instructions on how to resume or complete the merge manually.

---

## Command Execution History & Exit Summary

Regardless of whether the workflow succeeds, encounters an error, or is cancelled via `Ctrl+C`, an audit report is printed to `stderr` and optionally written to `--log-file`:

```text
=== Workflow Summary ===
Status: Interrupted by unresolved merge conflicts
Repository: my-service (C:\Projects\my-service)
Source branch: origin/feature/payments
Offshore branch: test/offshore-test-20261021
IT test branch: Not created
Remote: origin

=== Git Commands Executed ===
[1] git status --porcelain
    Directory: C:\Projects\my-service
    Status: SUCCESS (0.0s)

[2] git fetch origin
    Directory: C:\Projects\my-service
    Status: SUCCESS (0.4s)

[3] git switch -c test/offshore-test-20261021 origin/develop
    Directory: C:\Projects\my-service
    Status: SUCCESS (0.1s)

[4] git merge origin/feature/payments
    Directory: C:\Projects\my-service
    Status: FAILED (0.1s)
    Reason: Merge conflicts detected

=== Recovery Instructions ===
1. Resolve the listed conflicts in your editor
2. Stage the resolved files with 'git add <file>'
3. Complete the merge commit with 'git commit'
4. Rerun this application to continue the workflow
```

---

## Resumability & Recovery

The application detects pre-existing Git state:
- If `test/offshore-test-YYYYMMDD` or `test/it-YYYYMMDD` already exists locally or remotely, it warns you and prevents accidental overwrites.
- If a merge is already in progress when the application launches, validation fails early with clear recovery instructions rather than corrupting repository state.

---

## Installation & Requirements

- **Prerequisites**: A standard Git installation (`git` available in `$PATH`).
- No Python, Node.js, or external runtime required.

Download the appropriate binary for your OS and architecture from the `dist/` directory or build from source.

---

## Usage & Flags

```bash
test-branch-manager [flags]
```

### Options

| Flag | Shorthand | Description | Default |
|---|---|---|---|
| `-dir <path>` | | Working directory to scan for Git repositories | Current working directory |
| `-verbose` | `-v` | Show detailed Git execution output | `false` |
| `-log-file <path>` | | Path to save command history and summary to a file | none |
| `-accessible` | | Accessible / non-interactive terminal prompt mode | `false` |
| `-version` | | Print application version | |
| `-help` | `-h` | Display help and usage information | |

### Example Invocations

```bash
# Run interactively in the current directory:
test-branch-manager

# Scan a specific projects directory:
test-branch-manager -dir /path/to/repositories

# Enable verbose logging and save execution log:
test-branch-manager -v -log-file git-workflow.log
```

---

## Building & Cross-Compilation

We provide a `Makefile`, bash script (`scripts/build.sh`), and Windows batch script (`scripts/build.bat`). All builds disable CGO (`CGO_ENABLED=0`) for maximum portability.

### Build All Supported Platforms

#### On Linux / macOS (using Makefile or Bash):
```bash
make build-all
# Or:
bash ./scripts/build.sh
```

#### On Windows (PowerShell or Command Prompt):
```powershell
.\scripts\build.bat
```

#### Manual Build Commands:

- **Windows (x86_64)**:
  ```bash
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/test-branch-manager-windows-amd64.exe ./cmd/test-branch-manager
  ```

- **macOS Intel (x86_64)**:
  ```bash
  GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/test-branch-manager-darwin-amd64 ./cmd/test-branch-manager
  ```

- **macOS Apple Silicon (ARM64)**:
  ```bash
  GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/test-branch-manager-darwin-arm64 ./cmd/test-branch-manager
  ```

- **Linux (x86_64)**:
  ```bash
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/test-branch-manager-linux-amd64 ./cmd/test-branch-manager
  ```

---

## Running Automated Tests

Run the full suite of unit and integration tests:

```bash
# Run all tests
go test ./...

# Run all tests with verbose output
go test -v ./...

# Run with test coverage
go test -coverprofile=coverage.out ./...
```

The test suite exercises:
- Branch name calculation and date generation.
- Repository discovery (root, immediate children, worktree `.git` files).
- Branch filtering and search logic.
- Command history tracking and thread safety.
- Complete end-to-end integration workflows using isolated temporary bare remotes.
- Merge conflict simulation and resolution loops.
- Interrupted workflows and cancellation handling.

---

## Troubleshooting

- **"No Git repositories found"**:
  Make sure `-dir` points to a folder containing Git repositories, or run `test-branch-manager` directly inside a repository folder.
- **"Repository has uncommitted changes"**:
  Run `git status` in the repository and either commit your work (`git commit`) or stash it (`git stash`) before running the tool.
- **"Branch 'develop' does not exist on remote"**:
  The workflow expects a baseline branch named `develop` on the remote. Verify your remote branches with `git branch -r`.
- **Merge conflict left in repository**:
  If you exit mid-merge, your work is saved. Run `git status`, resolve conflicts, stage changes with `git add`, and run `git commit`.
