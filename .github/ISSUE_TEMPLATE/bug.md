---
name: Bug report
about: Something isn't working as documented
title: ''
labels: bug
assignees: ''
---

## What happened

A clear, concise description of the bug.

## Reproduction

The shortest possible code or command that triggers the bug. For
shell bugs, this is usually a `rein run` or `rein start` command.

```bash
rein start --idle-timeout 200ms "echo hello; sleep 30"
```

## Expected behavior

What you expected to happen.

## Actual behavior

What actually happened. Include any error messages and exit codes.

```
{"type":"error","err":"..."}
```

## Environment

- OS: (e.g. macOS 14.4, Ubuntu 24.04, Windows 11)
- Go version: (`go version`)
- rein version: (`rein version`)

## Additional context

Anything else that might be relevant.
