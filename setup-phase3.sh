#!/usr/bin/env bash
# setup-phase3.sh — Add the Phase 3 commits on top of Phase 2
# (18 commits total at this point).
#
# Phase 3 adds:
#   - The rein CLI binary (cmd/rein)
#   - The NDJSON protocol (internal/ndjson)
#   - 'run' and 'start' subcommands
#   - Module rename to github.com/SalzDevs/rein
#   - Updated README
#
# Usage:
#   ./setup-phase3.sh          # commit and run tests
#   ./setup-phase3.sh --push   # commit, run tests, push to GitHub
set -euo pipefail

cd "$(dirname "$0")"

GIT_USER_NAME="Ricardo Ceia"
GIT_USER_EMAIL="a49000@alunos.isel.pt"
GITHUB_REMOTE="${GITHUB_REMOTE:-git@github.com:SalzDevs/rein.git}"

echo "==> Verifying we are on top of Phase 2 (18 commits)"
COMMIT_COUNT=$(git log --oneline | wc -l | tr -d ' ')
if [ "$COMMIT_COUNT" != "18" ]; then
    echo "ERROR: expected 18 commits from Phase 1+2, found $COMMIT_COUNT."
    echo "Phase 3 should be run on top of a complete Phase 1+2 setup."
    exit 1
fi

echo "==> Configuring local git author"
git config user.name "$GIT_USER_NAME"
git config user.email "$GIT_USER_EMAIL"

# ---------------------------------------------------------------------------
# Commit 19: rename module + update .gitignore
# ---------------------------------------------------------------------------
echo "==> [19/27] chore: rename module to github.com/SalzDevs/rein and update .gitignore"
git add go.mod run.go session.go .gitignore
git commit -m "chore: rename module to github.com/SalzDevs/rein and update .gitignore

The Go module path is now github.com/SalzDevs/rein to match the
actual GitHub URL. The earlier a49000/rein path was a holdover
from when only the email was known.

Also updates .gitignore to exclude the rein binary built by
'go build' from the cmd/rein/ directory." --quiet

# ---------------------------------------------------------------------------
# Commit 20: scaffold cmd/rein CLI binary
# ---------------------------------------------------------------------------
echo "==> [20/27] chore: scaffold cmd/rein CLI binary"
git add cmd/rein/main.go
git commit -m "chore: scaffold cmd/rein CLI binary

Adds the cmd/rein/main.go entry point with version and help
subcommands. The 'run' and 'start' subcommands are added in
subsequent commits.

The CLI is the public, language-agnostic surface of rein. Any
program that can spawn a subprocess and parse JSON can use the
NDJSON protocol to run shell commands with rein's process-group
isolation, graceful shutdown, and idle timeout." --quiet

# ---------------------------------------------------------------------------
# Commit 21: add NDJSON message types
# ---------------------------------------------------------------------------
echo "==> [21/27] feat: add NDJSON message types"
git add internal/ndjson/message.go
git commit -m "feat: add NDJSON message types

Defines the wire protocol used by the rein CLI: newline-delimited
JSON objects, one per line, over stdio.

Message types:
  started   emitted when the process is running
  line      emitted for each line of process output
  exit      emitted when the process exits
  error     emitted on error before the process started
  stop      incoming: ask rein to stop the running process

Numeric fields (PID, ExitCode, DurationMS) are pointers so a
genuine 0 is distinguishable from 'field not set'. This is the
difference between 'exit code 0 (success)' and 'no exit code
because the process never started'." --quiet

# ---------------------------------------------------------------------------
# Commit 22: add NDJSON encoder/decoder
# ---------------------------------------------------------------------------
echo "==> [22/27] feat: add NDJSON encoder/decoder"
git add internal/ndjson/stream.go
git commit -m "feat: add NDJSON encoder/decoder

Encoder writes one JSON object per line. Decoder reads one JSON
object per line. Both are tiny wrappers over encoding/json and
bufio.Scanner with a 16MB line limit (NDJSON messages should be
small; the limit is just a safety net)." --quiet

# ---------------------------------------------------------------------------
# Commit 23: add tests for NDJSON stream
# ---------------------------------------------------------------------------
echo "==> [23/27] test: add tests for NDJSON stream"
git add internal/ndjson/stream_test.go
git commit -m "test: add tests for NDJSON stream

Covers: round-trip of every message type, EOF behaviour, invalid
JSON rejection, and omitempty behaviour for both string and
pointer-to-numeric fields." --quiet

# ---------------------------------------------------------------------------
# Commit 24: add 'run' subcommand
# ---------------------------------------------------------------------------
echo "==> [24/27] feat: add 'run' subcommand"
git add cmd/rein/run.go
git commit -m "feat: add 'run' subcommand

Usage: rein run [options] <command>

Runs a command to completion and emits the result as a single
NDJSON line on stdout:

  {\"type\":\"exit\",\"exit_code\":0,\"duration_ms\":5,\"stdout\":\"...\",\"stderr\":\"...\"}

Options mirror the rein library: --timeout, --graceful-timeout,
--dir, --env (repeatable), --pty. Non-zero exit codes from the
underlying command are propagated." --quiet

# ---------------------------------------------------------------------------
# Commit 25: add 'start' subcommand
# ---------------------------------------------------------------------------
echo "==> [25/27] feat: add 'start' subcommand with NDJSON streaming"
git add cmd/rein/start.go
git commit -m "feat: add 'start' subcommand with NDJSON streaming

Usage: rein start [options] <command>

Starts a long-running command and streams output as NDJSON on
stdout. The caller can stop the process by:

  - Sending {\"type\":\"stop\"} on stdin
  - Sending SIGINT/SIGTERM to the rein process

Output:
  {\"type\":\"started\",\"pid\":1234}
  {\"type\":\"line\",\"stream\":\"stdout\",\"text\":\"...\"}
  {\"type\":\"line\",\"stream\":\"stderr\",\"text\":\"...\"}
  {\"type\":\"exit\",\"exit_code\":0,\"duration_ms\":1234}

All rein library options are supported: --timeout, --idle-timeout,
--graceful-timeout, --dir, --env, --pty, --line-buffer." --quiet

# ---------------------------------------------------------------------------
# Commit 26: add tests for CLI subcommands
# ---------------------------------------------------------------------------
echo "==> [26/27] test: add tests for CLI subcommands"
git add cmd/rein/main_test.go
git commit -m "test: add tests for CLI subcommands

Uses TestMain to build the rein binary to a temp dir, then
spawns it as a subprocess from each test.

Covers: version output, help output, unknown command error,
'rein run' basic + non-zero exit + timeout + no-command,
'rein start' streaming lines, stop via stdin NDJSON message,
and idle timeout kill." --quiet

# ---------------------------------------------------------------------------
# Commit 27: update README
# ---------------------------------------------------------------------------
echo "==> [27/27] docs: update README with CLI usage and NDJSON protocol"
git add README.md
git commit -m "docs: update README with CLI usage and NDJSON protocol

The Phase 2 README covered only the Go library. The new README
adds:
  - The rein CLI as a public, language-agnostic interface
  - The NDJSON message protocol (types and shapes)
  - A Python example showing the cross-language use case
  - A Node example showing the same
  - Updated platform support matrix
  - Updated roadmap (CLI checked off, NDJSON checked off)" --quiet

echo ""
echo "==> Done. Commit log (top 9):"
git log --oneline | head -9

echo ""
echo "==> Verifying tests pass after commit history..."
go test -race -count=1 ./...

echo ""
echo "==> Next steps:"
echo "    Push to GitHub:"
echo "         git push -u origin main"

if [ "${1:-}" = "--push" ]; then
    echo ""
    echo "==> --push requested. Pushing to GitHub."
    git push -u origin main
fi
