# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| 0.1.x   | Yes       |

## Reporting a vulnerability

If you discover a security vulnerability in rein, please report it
privately by emailing **security@salzdevs.com**. Do not open a
public GitHub issue for security bugs.

We will acknowledge receipt within 48 hours and aim to provide a
fix or mitigation within 14 days for critical issues, or 30 days
for non-critical issues.

## Scope

rein executes shell commands on behalf of AI agent frameworks. The
security model is:

- **Trusted input**: the agent is trusted to specify what commands
  to run. rein does not sandbox untrusted input by default.
- **Process isolation**: rein puts each child in its own process
  group (POSIX) or Job Object (Windows) so it can be killed as a
  unit.
- **PTY allocation**: when WithPTY is set, the child sees a real
  TTY. This is required for some programs but does not by itself
  add security.

If you need to run **untrusted** commands, see the roadmap for the
sandbox feature (seccomp on Linux, sandbox-exec on macOS). Until
that lands, do not point rein at untrusted input.
