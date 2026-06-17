# rein

> The shell layer AI agents don't break.

`rein` is a small Go library **and CLI** that runs shell commands the
way AI agents actually need: with timeouts that work, signals that
propagate, and process trees that get cleaned up when the context is
cancelled.

It is designed to be the foundation underneath any agent framework
that needs to execute commands — replacing the broken `os/exec`,
`subprocess.Popen`, and `child_process.spawn` wrappers that every
agent framework currently reinvents.

Two ways to use it:

1. **As a Go library** (typed, structured, fast)
2. **As a CLI binary** via NDJSON over stdio (works with any language)

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
timeout fires.

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
Gemini CLI, OpenCode, you name it — has the same open issues
(`#2183`, `#25979`, `#53328`, `#26224`, ...). They all boil down
to four things:

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
```

### Functional options

```go
func WithTimeout(d time.Duration) Option
func WithGracefulTimeout(d time.Duration) Option
func WithIdleTimeout(d time.Duration) Option   // Start only
func WithEnv(env []string) Option
func WithDir(dir string) Option
func WithPTY() Option                          // POSIX only
func WithLineBuffer(n int) Option
```

## CLI reference

```
rein <command> [arguments]

Commands:
  run       Run a command to completion, print the result as NDJSON
  start     Start a long-running command, stream lines as NDJSON
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

## NDJSON protocol

The CLI speaks newline-delimited JSON on stdio. Five message types:

### Outgoing (stdout)

| Type | Fields | When |
|---|---|---|
| `started` | `pid` | The process is running. |
| `line` | `stream` (`"stdout"`/`"stderr"`), `text` | Each line of output. |
| `exit` | `exit_code`, `duration_ms`, `stdout`, `stderr`, `err` | The process has exited. |
| `error` | `err` | Error before the process started. |

### Incoming (stdin)

| Type | Fields | Effect |
|---|---|---|
| `stop` | (none) | Ask rein to stop the running process. |

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

// Stop after 30s
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
| Linux | Full support (process groups, SIGTERM, SIGKILL, PTY, CLI) |
| macOS | Full support (process groups, SIGTERM, SIGKILL, PTY, CLI) |
| Windows | Partial — process group isolation via Job Objects is not yet implemented. PTY uses ConPTY (not yet wired). SIGTERM/SIGKILL are sent to the leader only; grandchildren may leak. |

## Roadmap

- [x] `Run()` for one-shot commands
- [x] Timeouts with graceful shutdown
- [x] Process group isolation (POSIX)
- [x] `Start()` for long-running commands with line-buffered streaming
- [x] Idle timeout (kill on silence)
- [x] `Stop()` for explicit shutdown
- [x] Real PTY allocation for interactive commands (POSIX)
- [x] CLI binary (`rein run`, `rein start`)
- [x] NDJSON protocol for cross-language use
- [ ] Windows ConPTY + Job Object support
- [ ] Bounded output buffer with overflow policy
- [ ] Persistent cross-process state (for long-lived agents)
- [ ] `rein exec` subcommand for interactive PTY sessions

## License

MIT.
