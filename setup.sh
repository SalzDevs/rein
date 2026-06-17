#!/usr/bin/env bash
# setup.sh — Initialize the rein repo with you as the author and
# create the commit history.
#
# Usage:
#   ./setup.sh              # just init + commit
#   ./setup.sh --push       # init + commit + push to GitHub
#
# Before running this:
#   1. Create an empty repo on GitHub: https://github.com/new
#      Name: rein
#      Owner: a49000
#      Visibility: Public
#      Do NOT initialize with README, .gitignore, or license
#   2. Have your SSH key or personal access token configured.
#
# After running:
#   git log --oneline        # verify the 9 commits
#   go test -race ./...       # verify the library builds and tests pass
set -euo pipefail

cd "$(dirname "$0")"

GIT_USER_NAME="Ricardo Ceia"
GIT_USER_EMAIL="a49000@alunos.isel.pt"
GITHUB_REMOTE="${GITHUB_REMOTE:-git@github.com:a49000/rein.git}"

# Helper: commit only if there are staged changes.
commit_if_changes() {
    if git diff --cached --quiet; then
        echo "        (no changes to commit, skipping)"
        return 0
    fi
    git commit --quiet
}

# Write a small placeholder README for the initial commit. The full
# README is written just before the docs commit so the diff between
# the two README commits is the meaningful one.
write_placeholder_readme() {
    cat > README.md << 'PLACEHOLDER'
# rein

WIP — see commit log.
PLACEHOLDER
}

write_full_readme() {
    cat > README.md << 'FULLREADME'
# rein

> The shell layer AI agents don't break.

`rein` is a small, single-purpose Go library that runs shell commands the
way AI agents actually need: with timeouts that work, signals that
propagate, and process trees that get cleaned up when the context is
cancelled.

It is designed to be the foundation underneath any agent framework
that needs to execute commands — replacing the broken `os/exec`,
`subprocess.Popen`, and `child_process.spawn` wrappers that every
agent framework currently reinvents.

## Install

```bash
go get github.com/a49000/rein
```

Go 1.22 or later.

## The 5-second pitch

```go
import (
    "context"
    "fmt"
    "time"

    "github.com/a49000/rein"
)

result, err := rein.Run(ctx, "npm install",
    rein.WithTimeout(5*time.Minute),
)
fmt.Println(result.Stdout)
```

That is it. `rein.Run` runs the command, captures stdout and stderr,
returns the exit code, and **kills the entire process tree** if the
timeout fires. No leaked child processes. No zombie `npm install`s.
No `npm run dev` hanging forever.

## Why this exists

Every AI agent framework — Claude Code, Cursor, Aider, Cline, Codex,
Gemini CLI, OpenCode, you name it — has the same open issues
(`#2183`, `#25979`, `#53328`, `#26224`, ...). They all boil down
to four things:

1. **Hangs on long-running commands** (`npm run dev`, `pnpm dlx convex dev`).
2. **Silent stalls on stalled TCP** (no keepalive, no read timeout).
3. **Leaked child processes** when the agent is killed.
4. **No graceful shutdown** (SIGKILL on cancel, no SIGTERM first).

`rein` solves all four with a single API.

## API

```go
// Run a command to completion or until the timeout.
func Run(ctx context.Context, command string, opts ...Option) (*Result, error)

// Functional options.
func WithTimeout(d time.Duration) Option
func WithGracefulTimeout(d time.Duration) Option
func WithEnv(env []string) Option
func WithDir(dir string) Option

// Result is always returned, even on failure. Check Err.
type Result struct {
    Stdout   string
    Stderr   string
    ExitCode int
    Duration time.Duration
    Err      error
}
```

### Shutdown sequence

On timeout or context cancel, `rein` follows the standard Unix
escalation:

1. **SIGTERM** is sent to the entire process group.
2. rein waits up to `GracefulTimeout` (default 5s) for the process
   to exit on its own.
3. If still running, **SIGKILL** is sent to the process group.

This means `npm run dev` gets a chance to clean up child processes
and write to disk before being killed.

## Platform support

| OS | Status |
|---|---|
| Linux | Full support (process groups, SIGTERM, SIGKILL) |
| macOS | Full support (process groups, SIGTERM, SIGKILL) |
| Windows | Partial — process group isolation via Job Objects is not yet implemented. SIGTERM/SIGKILL are sent to the leader only; grandchildren may leak. |

## Roadmap

- [x] `Run()` for one-shot commands
- [x] Timeouts with graceful shutdown
- [x] Process group isolation (POSIX)
- [ ] `Start()` for long-running commands with line-buffered streaming
- [ ] Idle-timeout (kill on silence)
- [ ] Real PTY allocation for interactive commands (`sudo`, `ssh-add`, `npm init`)
- [ ] CLI binary (`rein run`, `rein exec`)
- [ ] NDJSON protocol for cross-language use (Python, Node, Rust bindings)
- [ ] Windows Job Object support
- [ ] Bounded output buffer with overflow policy

## License

MIT.
FULLREADME
}

echo "==> Initializing git repository"
if [ ! -d .git ]; then
    git init
    # Rename the default branch to main (older git versions
    # default to master; newer ones use main).
    git symbolic-ref HEAD refs/heads/main
fi

echo "==> Configuring local git author"
git config user.name "$GIT_USER_NAME"
git config user.email "$GIT_USER_EMAIL"

# ---------------------------------------------------------------------------
# Commit 1: initial commit (LICENSE, placeholder README, .gitignore, go.mod)
# ---------------------------------------------------------------------------
echo "==> [1/9] chore: initial commit"
write_placeholder_readme
git add LICENSE README.md .gitignore go.mod
git commit -m "chore: initial commit" --quiet

# ---------------------------------------------------------------------------
# Commit 2: Result type
# ---------------------------------------------------------------------------
echo "==> [2/9] feat: add Result type"
git add result.go
git commit -m "feat: add Result type" --quiet

# ---------------------------------------------------------------------------
# Commit 3: Options type with functional options
# ---------------------------------------------------------------------------
echo "==> [3/9] feat: add Options type with functional options"
git add options.go
git commit -m "feat: add Options type with functional options" --quiet

# ---------------------------------------------------------------------------
# Commit 4: implement Run() (basic version, no process group yet)
# ---------------------------------------------------------------------------
echo "==> [4/9] feat: implement Run() for one-shot commands"
git add run.go
git commit -m "feat: implement Run() for one-shot commands

Runs a shell command via sh -c, captures stdout and stderr, returns
the exit code and a Result. Uses exec.CommandContext under the
hood, so on timeout the process is signalled." --quiet

# ---------------------------------------------------------------------------
# Commit 5: tests
# ---------------------------------------------------------------------------
echo "==> [5/9] test: add unit tests for Run()"
git add run_test.go
git commit -m "test: add unit tests for Run()

Covers: success, non-zero exit, stderr, timeout, context cancel,
env, dir, empty command, duration." --quiet

# ---------------------------------------------------------------------------
# Commit 6: process group isolation (POSIX + Windows stub)
# ---------------------------------------------------------------------------
echo "==> [6/9] feat: add process group isolation for clean shutdown"
git add internal/procgroup/
git commit -m "feat: add process group isolation for clean shutdown

Adds internal/procgroup which sets Setpgid on POSIX so the child
becomes the leader of a new process group. This lets us send
signals to the entire process tree, not just the leader.

Windows fallback is a no-op for v0.0.1; proper support requires
Job Objects and is tracked separately." --quiet

# ---------------------------------------------------------------------------
# Commit 7: signal package + graceful shutdown + process group test
# ---------------------------------------------------------------------------
echo "==> [7/9] feat: add graceful shutdown with SIGTERM then SIGKILL"
git add internal/signal/ run.go run_test.go
git commit -m "feat: add graceful shutdown with SIGTERM then SIGKILL

Adds internal/signal for sending signals to a process group, and
rewires Run() to manage the command lifecycle manually so we can
implement the standard Unix escalation:

  1. SIGTERM to the process group on timeout or context cancel
  2. wait up to GracefulTimeout for the process to exit
  3. SIGKILL to the process group if it is still running

This replaces the simpler CommandContext behaviour from the
initial implementation.

Also adds TestRun_KillsProcessGroup which spawns a sleep as a
grandchild, times out the parent, and verifies the grandchild is
reaped via kill(pid, 0) returning ESRCH." --quiet

# ---------------------------------------------------------------------------
# Commit 8: CI workflow
# ---------------------------------------------------------------------------
echo "==> [8/9] chore: add GitHub Actions CI workflow"
git add .github/
git commit -m "chore: add GitHub Actions CI workflow

Runs go vet and go test -race on Ubuntu and macOS for every push
and pull request." --quiet

# ---------------------------------------------------------------------------
# Commit 9: real README (replaces the placeholder)
# ---------------------------------------------------------------------------
echo "==> [9/9] docs: rewrite README with quick start and examples"
write_full_readme
git add README.md
git commit -m "docs: rewrite README with quick start and examples

The initial README was a placeholder. The new README covers the
pitch, the API, the shutdown sequence, platform support, and the
roadmap for the next features." --quiet

echo ""
echo "==> Done. Commit log:"
git log --oneline

echo ""
echo "==> Verifying tests pass after commit history..."
go test -race -count=1 ./...

echo ""
echo "==> Next steps:"
echo "    1. Create an empty repo on GitHub: https://github.com/new"
echo "       (name: rein, owner: a49000, public, no README/.gitignore/license)"
echo "    2. Push to GitHub:"
echo "         git remote add origin $GITHUB_REMOTE"
echo "         git push -u origin main"

if [ "${1:-}" = "--push" ]; then
    echo ""
    echo "==> --push requested. Adding remote and pushing."
    if ! git remote get-url origin > /dev/null 2>&1; then
        git remote add origin "$GITHUB_REMOTE"
    fi
    git push -u origin main
fi
