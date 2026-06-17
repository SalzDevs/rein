package rein

import "time"

// Options configures a single Run or Start call. Zero values are
// replaced with sensible defaults; see the [Option] functions.
type Options struct {
	// Timeout is the maximum wall-clock duration before the process
	// is signalled. Default: 30s. Zero or negative means no timeout.
	Timeout time.Duration

	// GracefulTimeout is how long to wait after sending SIGTERM
	// before sending SIGKILL. Default: 5s. Only takes effect when
	// the command is being stopped (timeout, context cancel, or Stop).
	GracefulTimeout time.Duration

	// IdleTimeout is the maximum duration of output silence before
	// the process is signalled. Useful for long-running commands
	// like `npm run dev` that should be considered stuck if they
	// stop emitting output. Default: 0 (disabled). Only takes
	// effect on Start, not Run.
	IdleTimeout time.Duration

	// Env is the environment for the child process. If nil, the
	// parent environment is inherited.
	Env []string

	// Dir is the working directory for the child process. If empty,
	// the parent's working directory is used.
	Dir string

	// PTY, if true, allocates a pseudo-terminal for the child. The
	// child sees a real TTY (good for `sudo`, `ssh-add`, colored
	// output, and tools that check `isatty`). Stderr is merged
	// with stdout on the same line. Default: false.
	//
	// Only implemented on POSIX in v0.0.1.
	PTY bool

	// LineBuffer is the size of the channel that buffers output
	// lines from a Start session before the producer blocks.
	// Default: 4096. Increase for fast-output processes whose
	// consumer is slow.
	LineBuffer int
}

// Option is a functional option for [Run] or [Start].
type Option func(*Options)

// WithTimeout sets the maximum wall-clock duration of the command.
// If the command runs longer, the process tree is signalled
// (see [WithGracefulTimeout] for the escalation policy).
func WithTimeout(d time.Duration) Option {
	return func(o *Options) { o.Timeout = d }
}

// WithGracefulTimeout sets how long to wait after a graceful signal
// before sending SIGKILL. Default: 5s. Only takes effect when the
// command is being stopped.
func WithGracefulTimeout(d time.Duration) Option {
	return func(o *Options) { o.GracefulTimeout = d }
}

// WithIdleTimeout sets the maximum duration of output silence
// before the process is signalled. Useful for long-running
// commands like `npm run dev` that should be considered stuck if
// they stop emitting output. Default: 0 (disabled).
//
// Only takes effect on [Start], not [Run].
func WithIdleTimeout(d time.Duration) Option {
	return func(o *Options) { o.IdleTimeout = d }
}

// WithEnv sets the environment for the child process. Pass nil to
// inherit the parent environment.
func WithEnv(env []string) Option {
	return func(o *Options) { o.Env = env }
}

// WithDir sets the working directory for the child process.
func WithDir(dir string) Option {
	return func(o *Options) { o.Dir = dir }
}

// WithPTY allocates a pseudo-terminal for the child. The child sees
// a real TTY (good for `sudo`, `ssh-add`, colored output, and
// tools that check isatty). Stderr is merged with stdout on the
// same line. Default: false.
//
// Only implemented on POSIX in v0.0.1.
func WithPTY() Option {
	return func(o *Options) { o.PTY = true }
}

// WithLineBuffer sets the size of the channel that buffers output
// lines from a Start session. Default: 4096.
func WithLineBuffer(n int) Option {
	return func(o *Options) { o.LineBuffer = n }
}

// defaultOptions returns the Options used when the caller does not
// override them.
func defaultOptions() *Options {
	return &Options{
		Timeout:         30 * time.Second,
		GracefulTimeout: 5 * time.Second,
		LineBuffer:      4096,
	}
}
