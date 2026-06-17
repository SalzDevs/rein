# rein examples

Runnable programs that show how to use rein.

## Running

Each example is in its own subdirectory. From the example's directory:

```bash
go run .
```

## Examples

| Directory | What it shows |
|---|---|
| [`basic`](basic/) | Run a one-shot command and print the result. |
| [`stream`](stream/) | Start a long-running command, stream output line-by-line, with an idle timeout. |
| [`pty`](pty/) | Drive an interactive command via PTY — read a prompt, write a response. |
| [`overflow`](overflow/) | Handle a process that produces more output than the buffer can hold. |

## Note on `go.mod`

The examples use a `go.mod` with a `replace` directive pointing
back to the parent directory. This means you can run them
directly from the repo without publishing rein to a module
proxy first. If you copy an example to a different project,
update the import path.
