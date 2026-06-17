# Changelog

All notable changes to rein are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-06-17

### Added
- `rein.Run()` for one-shot commands with stdout/stderr capture
- `rein.Start()` for long-running commands with line-buffered streaming
- `Session.Lines()`, `Wait()`, `Done()`, `Stop()`, `PID()` accessors
- `Session.Write()` and `Session.Resize()` for interactive PTY use
- Idle timeout that kills processes on output silence
- `WithOverflowPolicy()` with three policies: Block (default), DropNewest, DropOldest
- `Session.Drops()` to query the number of dropped lines
- Process group isolation on POSIX (process gets its own process group)
- Job Object isolation on Windows (process assigned to a job with KILL_ON_JOB_CLOSE)
- Graceful shutdown: SIGTERM → wait → SIGKILL, with configurable grace period
- Real PTY support on POSIX (via `creack/pty`) and Windows (via ConPTY)
- CLI binary with four subcommands:
  - `rein run` for one-shot commands
  - `rein start` for long-running commands with idle timeout
  - `rein exec` for interactive PTY commands (input, resize, stop)
  - `rein daemon` for multi-session stateful management
- NDJSON protocol over stdio for cross-language use
- Python client library (`clients/python/`)
- Node.js client library (`clients/node/`)
- Comprehensive test suite (>50 tests, all passing with `-race`)
- CI on Linux, macOS, and Windows

### Platform support
- Linux: full support for all features
- macOS: full support for all features
- Windows: full support on Windows 10 1809+ (ConPTY, Job Objects, TerminateProcess)
