# rein

> **The shell layer AI agents don't break.**

[![CI](https://github.com/SalzDevs/rein/actions/workflows/ci.yml/badge.svg)](https://github.com/SalzDevs/rein/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/SalzDevs/rein)](https://goreportcard.com/report/github.com/SalzDevs/rein)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**rein** is a Go library + CLI for running shell commands inside AI
agent frameworks. It fixes the four bugs that every agent framework
has — **hangs on long-running commands**, **silent stalls on stalled
TCP**, **leaked child processes**, and **no graceful shutdown** — with
proper process group isolation, timeouts, idle kills, and real PTY
support.

The CLI speaks NDJSON over stdio so any language can use it.
Python and Node.js clients are included. Works on Linux and macOS;
Windows supports non-PTY sessions and is on the roadmap for full
ConPTY support.

## Install

### As a Go library

```bash
go get github.com/SalzDevs/rein
```

Go 1.22 or later.

### As a CLI

```bash
go install github.com/SalzDevs/rein/cmd/rein@latest
```

The `rein` binary ends up in `$GOPATH/bin`.

### As a Python client

```bash
go install github.com/SalzDevs/rein/cmd/rein@latest
pip install ./clients/python
```

### As a Node.js client

```bash
go install github.com/SalzDevs/rein/cmd/rein@latest
npm install ./clients/node
```

## The 5-second pitch (library)

```go
import (
    "context"
    "fmt"
    "time"

    "github.com/SalzDevs/rein"
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

## The 5-second pitch (CLI)

```bash
$ rein run --timeout 30s "npm install"
{"type":"exit","exit_code":0,"duration_ms":1234,"stdout":"added 1 package in 2s\n","stderr":""}

$ rein start --idle-timeout 2m "npm run dev"
{"type":"started","pid":4213}
{"type":"line","stream":"stdout","text":"> server-ready@1.0.0 dev"}
{"type":"line","stream":"stdout","text":"> next dev"}
...
```

Stop the start command with NDJSON on stdin:

```bash
$ echo '{"type":"stop"}' | rein start "sleep 30"
```

## Why this exists

Every AI agent framework — Claude Code, Cursor, Aider, Cline, Codex,
Gemini CLI, OpenCode, you name it — has the same open issues. They
all boil down to four things:

1. **Hangs on long-running commands** (`npm run dev`, `pnpm dlx convex dev`).
2. **Silent stalls on stalled TCP** (no keepalive, no read timeout).
3. **Leaked child processes** when the agent is killed.
4. **No graceful shutdown** (SIGKILL on cancel, no SIGTERM first).

`rein` solves all four with a single API.

## Library API

### One-shot commands

```go
func Run(ctx context.Context, command string, opts ...Option) (*Result, error)

type Result struct {
    Stdout   string
    Stderr   string
    ExitCode int
    Duration time.Duration
    Err      error
}
```

### Long-running commands

```go
func Start(ctx context.Context, command string, opts ...Option) (*Session, error)

type Line struct {
    Stream string  // "stdout" or "stderr"
    Text   string
}

type Session struct { ... }

func (s *Session) Lines() <-chan Line
func (s *Session) Wait() (*Result, error)
func (s *Session) Done() <-chan struct{}
func (s *Session) Stop() error
func (s *Session) PID() int
func (s *Session) Write(p []byte) (int, error)   // exec only
func (s *Session) Resize(rows, cols int) error    // exec only
func (s *Session) Drops() uint64                  // overflow count
```

### Functional options

```go
func WithTimeout(d time.Duration) Option
func WithGracefulTimeout(d time.Duration) Option
func WithIdleTimeout(d time.Duration) Option
func WithEnv(env []string) Option
func WithDir(dir string) Option
func WithPTY() Option
func WithLineBuffer(n int) Option
func WithOverflowPolicy(p OverflowPolicy) Option  // Block | DropNewest | DropOldest
```

## CLI reference

```
rein <command> [arguments]

Commands:
  run       Run a command to completion, print the result as NDJSON
  start     Start a long-running command, stream lines as NDJSON
  exec      Run a command with a PTY, drive it interactively via NDJSON
  daemon    Long-running multi-session manager (NDJSON over stdio)
  version   Print the version
  help      Print this help
```

### `rein run`

```
rein run [options] <command>

Options:
  --timeout <duration>          Maximum wall-clock duration (0 = no timeout)
  --graceful-timeout <duration> Time to wait between SIGTERM and SIGKILL
  --dir <path>                  Working directory
  --env <KEY=VALUE>             Environment variable (repeatable)
  --pty                         Allocate a pseudo-terminal

Output: a single NDJSON line on stdout.
```

### `rein start`

```
rein start [options] <command>

Options:
  --timeout <duration>          Maximum wall-clock duration
  --graceful-timeout <duration> Time to wait between SIGTERM and SIGKILL
  --idle-timeout <duration>     Kill the process if no output for this duration
  --dir <path>                  Working directory
  --env <KEY=VALUE>             Environment variable (repeatable)
  --pty                         Allocate a pseudo-terminal
  --line-buffer <n>             Size of the output channel buffer

Output: NDJSON on stdout, one message per line.
Control: NDJSON on stdin, one message per line.
Signals: SIGINT/SIGTERM stop the process.
```

### `rein exec`

```
rein exec [options] <command>

A PTY is always allocated. This subcommand is for interactive
commands that prompt the user (sudo, ssh-add, npm init, etc.).

Options:
  --timeout <duration>          Maximum wall-clock duration
  --graceful-timeout <duration> Time to wait between SIGTERM and SIGKILL
  --idle-timeout <duration>     Kill the process if no output for this duration
  --dir <path>                  Working directory
  --env <KEY=VALUE>             Environment variable (repeatable)
  --line-buffer <n>             Size of the output channel buffer
  --initial-size <rows>x<cols>  Initial PTY window size (default 24x80)

Output: NDJSON on stdout, one message per line.
Control: NDJSON on stdin, one message per line.
  {"type":"input","text":"password\n"}    Write to the child's stdin.
  {"type":"resize","rows":40,"cols":120} Resize the PTY window.
  {"type":"stop"}                           Stop the process.
Signals: SIGINT/SIGTERM stop the process.
```

### `rein daemon`

```
rein daemon
```

A long-running process that manages multiple rein sessions. The
daemon reads commands on stdin and emits events on stdout, both
as NDJSON. It enables an agent to start, observe, and control
many processes from a single connection.

Inputs:
  {"type":"create","id":"...","command":"...","options":{...}}
  {"type":"destroy","id":"..."}
  {"type":"stop","id":"..."}
  {"type":"input","id":"...","text":"..."}
  {"type":"resize","id":"...","rows":N,"cols":M}
  {"type":"list"}

Outputs:
  {"type":"created","id":"...","pid":...}
  {"type":"line","id":"...","stream":"...","text":"..."}
  {"type":"exit","id":"...","exit_code":...,"duration_ms":...}
  {"type":"destroyed","id":"...","exit_code":...}
  {"type":"sessions","sessions":[{"id":"...","pid":...}]}

Example: see `clients/python` and `clients/node` for full
client libraries that wrap the daemon protocol.

## NDJSON protocol

The CLI speaks newline-delimited JSON on stdio. Eight message types:

### Outgoing (stdout)

| Type | Fields | When |
|---|---|---|
| `started` | `pid` | The process is running. |
| `line` | `stream` (`"stdout"`/`"stderr"`), `text` | Each line of output. |
| `exit` | `exit_code`, `duration_ms`, `stdout`, `stderr`, `err` | The process has exited. |
| `error` | `err` | Error before the process started. |
| `created` (daemon) | `id`, `pid` | A new session was created. |
| `destroyed` (daemon) | `id`, `exit_code` | A session was destroyed. |
| `sessions` (daemon) | `sessions` (array of `{id, pid}`) | List of active sessions. |

### Incoming (stdin)

| Type | Fields | Effect |
|---|---|---|
| `stop` | (none) | Ask rein to stop the running process. |
| `input` | `text` | Write `text` to the child's stdin. (`exec` only) |
| `resize` | `rows`, `cols` | Resize the PTY window. (`exec` only) |
| `create` (daemon) | `id`, `command` | Create a new session. |
| `destroy` (daemon) | `id` | Destroy an existing session. |
| `list` (daemon) | (none) | List active sessions. |

### Example: Python client

```python
import subprocess, json

proc = subprocess.Popen(
    ["rein", "start", "--idle-timeout", "2m", "npm", "run", "dev"],
    stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True, bufsize=1,
)

for line in proc.stdout:
    msg = json.loads(line)
    if msg["type"] == "line":
        print(f"[{msg['stream']}] {msg['text']}")
    elif msg["type"] == "started":
        print(f"process started, pid={msg['pid']}")
    elif msg["type"] == "exit":
        print(f"process exited, code={msg.get('exit_code')}")
        break

# Stop the process
proc.stdin.write(json.dumps({"type": "stop"}) + "\n")
proc.stdin.flush()
```

### Example: Node.js client

```javascript
const { spawn } = require("child_process");
const readline = require("readline");

const child = spawn("rein", ["start", "npm", "run", "dev"], { stdio: "pipe" });

const rl = readline.createInterface({ input: child.stdout });
rl.on("line", (line) => {
  const msg = JSON.parse(line);
  if (msg.type === "line") console.log(`[${msg.stream}] ${msg.text}`);
  if (msg.type === "exit") {
    console.log(`process exited, code=${msg.exit_code}`);
    process.exit(msg.exit_code || 0);
  }
});

setTimeout(() => {
  child.stdin.write(JSON.stringify({ type: "stop" }) + "\n");
}, 30_000);
```

## Shutdown sequence

On timeout, idle timeout, context cancel, or `Session.Stop()`, `rein`
follows the standard Unix escalation:

1. **SIGTERM** is sent to the entire process group.
2. rein waits up to `GracefulTimeout` (default 5s) for the process
   to exit on its own.
3. If still running, **SIGKILL** is sent to the process group.

This means `npm run dev` gets a chance to clean up child processes
and write to disk before being killed.

## Platform support

| OS | Status |
|---|---|
| Linux | Full support (process groups, SIGTERM, SIGKILL, PTY, CLI, all features) |
| macOS | Full support (process groups, SIGTERM, SIGKILL, PTY, CLI, all features) |
| Windows | Process group via Job Objects (partial), SIGKILL via TerminateProcess. Non-PTY sessions work on Windows 10 1809+. ConPTY support is on the v0.2 roadmap. |

## Roadmap

- [x] `Run()` for one-shot commands
- [x] Timeouts with graceful shutdown
- [x] Process group isolation (POSIX) / Job Object scaffolding (Windows; v0.2)
- [x] `Start()` for long-running commands with line-buffered streaming
- [x] Idle timeout (kill on silence)
- [x] `Stop()` for explicit shutdown
- [x] Real PTY allocation (POSIX creack/pty; Windows ConPTY is on the v0.2 roadmap)
- [x] CLI binary (`rein run`, `rein start`, `rein exec`, `rein daemon`)
- [x] NDJSON protocol for cross-language use
- [x] `Session.Write()` and `Session.Resize()` for interactive PTY use
- [x] Bounded output buffer with overflow policies
- [x] Python and Node client libraries
- [x] `rein daemon` — multi-session stateful manager
- [ ] Persistent cross-process state (for long-lived agents)
- [ ] Windows ConPTY (true PTY sessions on Windows)
- [ ] `rein watch` for filesystem-triggered re-runs
- [ ] Security sandbox for untrusted commands (seccomp / sandbox-exec)

## Contributing

Contributions are welcome. Please [open an issue](https://github.com/SalzDevs/rein/issues/new) before sending a PR so we can discuss the approach. Run the test suite (`go test ./...`) before submitting.

## Security

For security issues, please email `security@salzdevs.com` instead of opening a public issue. See [SECURITY.md](SECURITY.md) for our policy.

## License

MIT.
