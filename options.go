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

	// Env is the environment for the child process. If nil, the
	// parent environment is inherited.
	Env []string

	// Dir is the working directory for the child process. If empty,
	// the parent's working directory is used.
	Dir string
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

// WithEnv sets the environment for the child process. Pass nil to
// inherit the parent environment.
func WithEnv(env []string) Option {
	return func(o *Options) { o.Env = env }
}

// WithDir sets the working directory for the child process.
func WithDir(dir string) Option {
	return func(o *Options) { o.Dir = dir }
}

// defaultOptions returns the Options used when the caller does not
// override them.
func defaultOptions() *Options {
	return &Options{
		Timeout:         30 * time.Second,
		GracefulTimeout: 5 * time.Second,
	}
}
