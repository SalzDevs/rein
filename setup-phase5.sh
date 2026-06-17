#!/usr/bin/env bash
# setup-phase5.sh — Add the Phase 5 commits on top of Phase 4
# (35 commits total at this point).
#
# Phase 5 adds:
#   5a: Bounded output buffer with overflow policies
#   5b: Python and Node client libraries
#   5c: Windows ConPTY + Job Object support
#   5d: rein daemon — multi-session stateful manager
#
# Usage:
#   ./setup-phase5.sh          # commit and run tests
#   ./setup-phase5.sh --push   # commit, run tests, push to GitHub
set -euo pipefail

cd "$(dirname "$0")"

GIT_USER_NAME="Ricardo Ceia"
GIT_USER_EMAIL="a49000@alunos.isel.pt"
GITHUB_REMOTE="${GITHUB_REMOTE:-git@github.com:SalzDevs/rein.git}"

echo "==> Verifying we are on top of Phase 4 (35 commits)"
COMMIT_COUNT=$(git log --oneline | wc -l | tr -d ' ')
if [ "$COMMIT_COUNT" != "35" ]; then
    echo "ERROR: expected 35 commits from Phase 1-4, found $COMMIT_COUNT."
    echo "Phase 5 should be run on top of a complete Phase 1-4 setup."
    exit 1
fi

echo "==> Configuring local git author"
git config user.name "$GIT_USER_NAME"
git config user.email "$GIT_USER_EMAIL"

# =========================================================================
# Phase 5a: Bounded output buffer
# =========================================================================

echo "==> [36/51] feat: add OverflowPolicy type and WithOverflowPolicy option"
git add options.go
git commit -m "feat: add OverflowPolicy type and WithOverflowPolicy option

Three policies:
  PolicyBlock      (default) - producer waits
  PolicyDropNewest          - new line is dropped silently
  PolicyDropOldest          - oldest buffered line is dropped

This is the fix for the OOM case where a long-running process
emits gigabytes of output faster than the consumer can read.
With DropOldest, the agent always sees the most recent N
lines; with DropNewest, the agent sees the first N lines and
nothing else. With Block (default), the producer back-pressures
the subprocess (current behavior)." --quiet

echo "==> [37/51] feat: implement overflow policies in Session"
git add session.go pty.go pty_posix.go pty_windows.go
git commit -m "feat: implement overflow policies in Session

The line reader now applies the configured OverflowPolicy when
the lines channel is full. Adds Session.Drops() to query the
number of dropped lines.

Also refactors the PTY state into a ptyState struct with
separate read/write/resize. On POSIX, read and write point to
the same master; on Windows (next commit), they are separate
pipes for ConPTY." --quiet

echo "==> [38/51] test: add tests for overflow policies"
git add session_test.go
git commit -m "test: add tests for overflow policies

Covers: DropNewest drops the new lines, DropOldest keeps the
most recent lines, Drops() reports the correct count, the
default policy is PolicyBlock, and Drops() starts at zero." --quiet

# =========================================================================
# Phase 5b: Python and Node client libraries
# =========================================================================

echo "==> [39/51] chore: scaffold clients directory"
git add clients/python/rein/__init__.py clients/python/rein/client.py
git add clients/python/tests/test_client.py clients/python/README.md
git add clients/python/setup.py 2>/dev/null || true
git commit -m "chore: scaffold Python client library

Adds clients/python/ with:
  - rein/__init__.py         - package entry point
  - rein/client.py           - Rein, Session, Result, Line
  - tests/test_client.py     - unit tests (require rein binary)
  - README.md                - usage and API

The Python wrapper spawns the rein CLI as a subprocess and
reads NDJSON from stdout. It exposes a Pythonic API: rein.run(),
rein.start(), rein.exec() returning Result or Session objects." --quiet

echo "==> [40/51] chore: scaffold Node.js client library"
git add clients/node/package.json clients/node/src/rein.js clients/node/src/rein.d.ts
git add clients/node/test/rein.test.js clients/node/README.md
git commit -m "chore: scaffold Node.js client library

Adds clients/node/ with:
  - package.json             - npm metadata
  - src/rein.js              - run, start, exec, Session
  - src/rein.d.ts            - TypeScript definitions
  - test/rein.test.js        - tests using node:test
  - README.md                - usage and API

The Node.js wrapper is a thin Promise-based layer over
child_process.spawn and the NDJSON protocol." --quiet

# =========================================================================
# Phase 5c: Windows ConPTY + Job Object support
# =========================================================================

echo "==> [41/51] feat: add Windows ConPTY support"
git add pty_windows.go go.mod go.sum
git commit -m "feat: add Windows ConPTY support

Implements pty_windows.go using the ConPTY API
(CreatePseudoConsole + STARTUPINFOEX + CreateProcess).
This gives the child a real TTY on Windows 10 1809+.

The implementation follows the standard ConPTY flow:
  1. Create input/output anonymous pipes
  2. CreatePseudoConsole with the pipe handles
  3. Set up STARTUPINFOEX with the ConPTY attribute
  4. CreateProcess with the extended startup info
  5. Read from the output pipe, write to the input pipe

Resize is implemented via ResizePseudoConsole.

The PTY state struct (read, write, resize) handles the POSIX /
Windows difference transparently: on POSIX, read and write point
to the same master file; on Windows, they are separate pipes." --quiet

echo "==> [42/51] feat: add Windows Job Object for process group"
git add internal/procgroup/procgroup_windows.go internal/signal/signal_windows.go
git commit -m "feat: add Windows Job Object for process group

Windows has no POSIX-style process groups, so the equivalent is
a Job Object with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE set. This
guarantees that all processes assigned to the job are killed
when the job handle is closed (e.g. when rein exits), preventing
orphaned children.

Also implements signal_windows.go using TerminateProcess with
an exit code that encodes the POSIX signal number (SIGTERM=15,
SIGKILL=9) for diagnostic purposes." --quiet

# =========================================================================
# Phase 5d: rein daemon
# =========================================================================

echo "==> [43/51] feat: add session management NDJSON message types"
git add internal/ndjson/message.go
git commit -m "feat: add session management NDJSON message types

Extends the NDJSON protocol with daemon-specific types:
  create, destroy, list (incoming)
  created, destroyed, sessions (outgoing)

Adds ID, Command, and Sessions fields to the Message struct.
These let the daemon identify which session a line, exit, or
destroyed event belongs to, and let the client specify which
session a control message targets." --quiet

echo "==> [44/51] feat: implement rein daemon with multi-session support"
git add cmd/rein/daemon.go cmd/rein/main.go
git commit -m "feat: implement rein daemon with multi-session support

The 'rein daemon' subcommand runs a long-running process that
manages multiple rein sessions from a single connection. It
reads commands on stdin (create/destroy/input/resize/stop/list)
and emits events on stdout (created/line/exit/destroyed/
sessions/error), all as NDJSON.

This enables an agent to start, observe, and control many
processes from one connection - the foundation for long-lived
agents that survive restarts (the daemon can persist session
state, in a future iteration)." --quiet

echo "==> [45/51] test: add tests for rein daemon"
git add cmd/rein/main_test.go
git commit -m "test: add tests for rein daemon

Spawns 'rein daemon' as a subprocess, sends create+list+
destroy messages via stdin, and verifies the created, sessions,
and destroyed messages appear on stdout.

The test exercises the full multi-session lifecycle:
  - Create a session with id=s1
  - List active sessions
  - Destroy the session
  - Verify all three messages are received" --quiet

# =========================================================================
# Docs update
# =========================================================================

echo "==> [46/51] docs: update README with Phase 5 features"
git add README.md
git commit -m "docs: update README with Phase 5 features

  - Add Python and Node client install instructions
  - Add 'daemon' subcommand to the commands table and docs
  - Update the roadmap (overflow buffer, clients, Windows,
    daemon all checked off)" --quiet

# =========================================================================
# Verification and push
# =========================================================================

echo ""
echo "==> Done. Commit log (top 11):"
git log --oneline | head -11

echo ""
echo "==> Verifying tests pass after commit history..."
go test -race -count=1 -timeout 60s ./...

echo ""
echo "==> Next steps:"
echo "    Push to GitHub:"
echo "         git push -u origin main"

if [ "${1:-}" = "--push" ]; then
    echo ""
    echo "==> --push requested. Pushing to GitHub."
    git push -u origin main
fi
