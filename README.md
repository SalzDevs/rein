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
go get github.com/SalzDevs/rein
```

Go 1.22 or later.

## The 5-second pitch

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

For long-running commands, use `rein.Start` to get a streaming
`Session` that emits output line-by-line and can be cleanly stopped.

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

func (s *Session) Lines() <-chan Line  // closed when process exits
func (s *Session) Wait() (*Result, error)
func (s *Session) Done() <-chan struct{}
func (s *Session) Stop() error          // blocks until process exits
func (s *Session) PID() int
```

### Functional options

```go
func WithTimeout(d time.Duration) Option           // wall-clock cap
func WithGracefulTimeout(d time.Duration) Option   // SIGTERM → wait → SIGKILL
func WithIdleTimeout(d time.Duration) Option       // kill on output silence (Start only)
func WithEnv(env []string) Option                  // nil = inherit
func WithDir(dir string) Option                    // working directory
func WithPTY() Option                              // allocate a real pseudo-terminal
func WithLineBuffer(n int) Option                  // channel buffer for Lines()
```

## Examples

### Stream a long-running process and stop it on idle

```go
session, err := rein.Start(ctx, "npm run dev",
    rein.WithIdleTimeout(2*time.Minute),  // kill if no output for 2 min
)
if err != nil { return err }

go func() {
    for line := range session.Lines() {
        log.Printf("[%s] %s", line.Stream, line.Text)
    }
}()

result, err := session.Wait()  // blocks until process exits
```

### Stop a process explicitly

```go
session, _ := rein.Start(ctx, "long-running-task")
time.Sleep(10 * time.Second)
session.Stop()  // SIGTERM, wait 5s, then SIGKILL
```

### Run an interactive command that needs a TTY

```go
session, err := rein.Start(ctx, "sudo apt update",
    rein.WithPTY(),  // gives the child a real /dev/pts/N
)
if err != nil { return err }
defer session.Stop()

for line := range session.Lines() {
    fmt.Println(line.Text)  // sudo's password prompt arrives as a line
}
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
| Linux | Full support (process groups, SIGTERM, SIGKILL, PTY) |
| macOS | Full support (process groups, SIGTERM, SIGKILL, PTY) |
| Windows | Partial — process group isolation via Job Objects is not yet implemented. PTY uses ConPTY (not yet wired). SIGTERM/SIGKILL are sent to the leader only; grandchildren may leak. |

## Roadmap

- [x] `Run()` for one-shot commands
- [x] Timeouts with graceful shutdown
- [x] Process group isolation (POSIX)
- [x] `Start()` for long-running commands with line-buffered streaming
- [x] Idle timeout (kill on silence)
- [x] `Stop()` for explicit shutdown
- [x] Real PTY allocation for interactive commands (POSIX)
- [ ] CLI binary (`rein run`, `rein exec`)
- [ ] NDJSON protocol for cross-language use (Python, Node, Rust bindings)
- [ ] Windows ConPTY + Job Object support
- [ ] Bounded output buffer with overflow policy
- [ ] Persistent cross-process state (for long-lived agents)

## License

MIT.
