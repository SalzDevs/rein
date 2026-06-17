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
