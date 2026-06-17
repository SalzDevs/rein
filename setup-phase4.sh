#!/usr/bin/env bash
# setup-phase4.sh — Add the Phase 4 commits on top of Phase 3
# (27 commits total at this point).
#
# Phase 4 adds:
#   - Session.Write() and Session.Resize() for interactive PTY use
#   - input and resize NDJSON message types
#   - The 'exec' CLI subcommand for driving interactive commands
#   - cmd.Stdin fix so non-PTY children don't read the agent's
#     control pipe
#
# Usage:
#   ./setup-phase4.sh          # commit and run tests
#   ./setup-phase4.sh --push   # commit, run tests, push to GitHub
set -euo pipefail

cd "$(dirname "$0")"

GIT_USER_NAME="Ricardo Ceia"
GIT_USER_EMAIL="a49000@alunos.isel.pt"
GITHUB_REMOTE="${GITHUB_REMOTE:-git@github.com:SalzDevs/rein.git}"

echo "==> Verifying we are on top of Phase 3 (27 commits)"
COMMIT_COUNT=$(git log --oneline | wc -l | tr -d ' ')
if [ "$COMMIT_COUNT" != "27" ]; then
    echo "ERROR: expected 27 commits from Phase 1+2+3, found $COMMIT_COUNT."
    echo "Phase 4 should be run on top of a complete Phase 1+2+3 setup."
    exit 1
fi

echo "==> Configuring local git author"
git config user.name "$GIT_USER_NAME"
git config user.email "$GIT_USER_EMAIL"

# ---------------------------------------------------------------------------
# Commit 28: fix cmd.Stdin for non-PTY sessions
# ---------------------------------------------------------------------------
echo "==> [28/35] fix: set cmd.Stdin to /dev/null for non-PTY sessions"
git add session.go
git commit -m "fix: set cmd.Stdin to /dev/null for non-PTY sessions

Previously, non-PTY children inherited the parent's stdin. For
'rein start' invoked by an agent framework, that meant the child
read from the agent's control pipe (where NDJSON stop/input
messages were being sent). This is wrong — the child has no
business reading the agent's control messages.

Now non-PTY children get /dev/null as their stdin, so programs
like cat exit immediately on EOF rather than reading the
agent's control pipe by accident.

PTY sessions are unaffected: creack/pty sets stdin to the PTY
slave, which is what we want." --quiet

# ---------------------------------------------------------------------------
# Commit 29: add input and resize NDJSON message types
# ---------------------------------------------------------------------------
echo "==> [29/35] feat: add input and resize NDJSON message types"
git add internal/ndjson/message.go
git commit -m "feat: add input and resize NDJSON message types

Adds two new incoming message types for interactive PTY use:

  {\"type\":\"input\",\"text\":\"password\\\\n\"}    Write to PTY stdin
  {\"type\":\"resize\",\"rows\":40,\"cols\":120}  Resize the PTY window

Both are no-ops for non-PTY sessions. The 'input' message is the
missing piece for AI agents to drive commands that prompt the
user (sudo, ssh-add, npm init, any CLI that reads from a TTY).
The 'resize' message is for agents that emulate a TUI (vim, less,
htop) where the size of the window changes the program's
behaviour." --quiet

# ---------------------------------------------------------------------------
# Commit 30: add tests for new NDJSON message types
# ---------------------------------------------------------------------------
echo "==> [30/35] test: add tests for new NDJSON message types"
git add internal/ndjson/stream_test.go
git commit -m "test: add tests for new NDJSON message types

Adds input and resize cases to the existing round-trip test, and
verifies that the pointer fields (Rows, Cols) round-trip
correctly for both zero and non-zero values." --quiet

# ---------------------------------------------------------------------------
# Commit 31: add Write() and Resize() methods to Session
# ---------------------------------------------------------------------------
echo "==> [31/35] feat: add Write() and Resize() methods to Session"
git add session.go pty.go pty_posix.go pty_windows.go
git commit -m "feat: add Write() and Resize() methods to Session

Session.Write(p []byte) writes to the child's stdin. Only works
for PTY sessions; returns an error for non-PTY.

Session.Resize(rows, cols int) resizes the PTY window by
sending the TIOCSWINSZ ioctl to the master FD.

Both methods are safe to call from any goroutine (they take the
session's mutex briefly to read the ptyMaster pointer).

The pty_posix.go platform code now returns *os.File instead of
io.Reader so the Session can hold a reference to the master and
call pty.Setsize on it." --quiet

# ---------------------------------------------------------------------------
# Commit 32: add tests for Write() and Resize()
# ---------------------------------------------------------------------------
echo "==> [32/35] test: add tests for Write() and Resize()"
git add session_test.go
git commit -m "test: add tests for Write() and Resize()

Covers:
  - Session.Write on a non-PTY session returns an error.
  - Session.Resize on a non-PTY session returns an error.
  - Session.Write on a PTY session writes to the child's stdin
    (verified with 'cat', which echoes its stdin to stdout).
  - Session.Resize on a PTY session does not error.

The PTY tests skip themselves in environments where PTY
allocation is blocked (e.g. the pi agent's sandboxed shell)." --quiet

# ---------------------------------------------------------------------------
# Commit 33: add 'exec' subcommand
# ---------------------------------------------------------------------------
echo "==> [33/35] feat: add 'exec' subcommand to CLI"
git add cmd/rein/exec.go cmd/rein/main.go
git commit -m "feat: add 'exec' subcommand to CLI

Usage: rein exec [options] <command>

A PTY is always allocated. This subcommand is for interactive
commands that prompt the user (sudo, ssh-add, npm init, etc.)
and for any TTY-aware tool that needs isatty() to return true.

Differences from 'start':
  - PTY is always on (no --pty flag; --pty would be redundant)
  - Accepts 'input' NDJSON messages on stdin (writes to PTY)
  - Accepts 'resize' NDJSON messages on stdin (resizes PTY)
  - --initial-size flag for the initial window size (default 24x80)

The 'input' and 'resize' handlers are the killer feature for
AI agents: they let an agent type into the prompt of an
interactive command and respond to the prompt's output, all
over the same NDJSON stream." --quiet

# ---------------------------------------------------------------------------
# Commit 34: add tests for 'exec' subcommand
# ---------------------------------------------------------------------------
echo "==> [34/35] test: add tests for 'exec' subcommand"
git add cmd/rein/main_test.go
git commit -m "test: add tests for 'exec' subcommand

Covers: streaming + exit, stop via stdin NDJSON, input + resize
via stdin NDJSON (uses 'cat' to verify input flows through the
PTY), and no-command error.

The PTY-dependent tests skip themselves in environments where
PTY allocation is blocked." --quiet

# ---------------------------------------------------------------------------
# Commit 35: update README
# ---------------------------------------------------------------------------
echo "==> [35/35] docs: update README with exec subcommand and input protocol"
git add README.md
git commit -m "docs: update README with exec subcommand and input protocol

The Phase 3 README covered 'run' and 'start'. The new README adds:
  - The 'exec' subcommand for interactive PTY use
  - Two new NDJSON message types: input and resize
  - Updated protocol table with all 7 types
  - Updated roadmap (exec checked off, Session.Write and
    Session.Resize checked off)" --quiet

echo ""
echo "==> Done. Commit log (top 8):"
git log --oneline | head -8

echo ""
echo "==> Verifying tests pass after commit history..."
go test -race -count=1 -timeout 30s ./...

echo ""
echo "==> Next steps:"
echo "    Push to GitHub:"
echo "         git push -u origin main"

if [ "${1:-}" = "--push" ]; then
    echo ""
    echo "==> --push requested. Pushing to GitHub."
    git push -u origin main
fi
