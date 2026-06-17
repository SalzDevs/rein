"""High-level Python client for the rein CLI.

Usage:

    from rein import Rein

    rein = Rein()
    result = rein.run("echo hello", timeout=30)
    print(result.stdout)

    with rein.start("npm run dev", idle_timeout=120) as session:
        for line in session.lines():
            print(line.text)
"""

from __future__ import annotations

import json
import os
import subprocess
import threading
from contextlib import contextmanager
from dataclasses import dataclass, field
from typing import Iterator, Optional


@dataclass
class Result:
    """The outcome of a one-shot command run via :meth:`Rein.run`."""

    exit_code: int
    duration_ms: int
    stdout: str = ""
    stderr: str = ""
    err: Optional[str] = None

    @property
    def ok(self) -> bool:
        return self.exit_code == 0


@dataclass
class Line:
    """A single line of process output."""

    stream: str
    text: str


class ReinError(Exception):
    """Raised when the rein CLI itself fails (not the command it ran)."""


def _find_rein(path: Optional[str] = None) -> str:
    """Locate the rein binary, raising ReinError if not found."""
    if path is not None:
        return path
    # Common locations: $REIN_BIN, alongside the venv's bin, or
    # just "rein" on $PATH.
    candidates = [
        os.environ.get("REIN_BIN"),
        os.path.join(os.path.dirname(__file__), "..", "..", "..", "..", "rein"),
        "rein",
    ]
    for c in candidates:
        if c is None:
            continue
        # Strip and check existence.
        c = c.strip()
        if not c:
            continue
        # If it's an absolute path, verify it exists.
        if os.path.isabs(c) and not os.path.isfile(c):
            continue
        return c
    raise ReinError(
        "rein binary not found. Set REIN_BIN environment variable, "
        "or pass rein_path= to Rein(...), or install rein with "
        "`go install github.com/SalzDevs/rein/cmd/rein@latest`."
    )


class Rein:
    """High-level Python client for the rein CLI.

    Parameters
    ----------
    rein_path
        Path to the rein binary. If None, searches $REIN_BIN,
        the repo root, then $PATH.
    """

    def __init__(self, rein_path: Optional[str] = None):
        self.rein_path = _find_rein(rein_path)

    def run(
        self,
        command: str,
        timeout: Optional[float] = None,
        graceful_timeout: Optional[float] = None,
        working_dir: Optional[str] = None,
        env: Optional[dict] = None,
        pty: bool = False,
    ) -> Result:
        """Run a command to completion and return its result.

        Parameters
        ----------
        command
            The shell command to run.
        timeout
            Wall-clock cap in seconds. None means no timeout.
        graceful_timeout
            Time to wait between SIGTERM and SIGKILL in seconds.
        working_dir
            Override the working directory.
        env
            Extra environment variables to set.
        pty
            If True, allocate a pseudo-terminal for the child.
        """
        args = [self.rein_path, "run"]
        if timeout is not None:
            args.extend(["--timeout", f"{int(timeout * 1000)}ms"])
        if graceful_timeout is not None:
            args.extend(["--graceful-timeout", f"{int(graceful_timeout * 1000)}ms"])
        if working_dir is not None:
            args.extend(["--dir", working_dir])
        if env:
            for k, v in env.items():
                args.extend(["--env", f"{k}={v}"])
        if pty:
            args.append("--pty")
        args.append(command)

        proc = subprocess.run(args, capture_output=True, text=True)
        if not proc.stdout.strip():
            raise ReinError(f"rein run failed: {proc.stderr}")
        msg = json.loads(proc.stdout.strip())
        return Result(
            exit_code=msg.get("exit_code", -1),
            duration_ms=msg.get("duration_ms", 0),
            stdout=msg.get("stdout", ""),
            stderr=msg.get("stderr", ""),
            err=msg.get("err"),
        )

    def start(
        self,
        command: str,
        timeout: Optional[float] = None,
        idle_timeout: Optional[float] = None,
        graceful_timeout: Optional[float] = None,
        working_dir: Optional[str] = None,
        env: Optional[dict] = None,
        pty: bool = False,
        line_buffer: int = 4096,
    ) -> "Session":
        """Start a long-running command and return a Session.

        See :meth:`run` for parameter meanings.
        """
        args = [self.rein_path, "start"]
        if timeout is not None:
            args.extend(["--timeout", f"{int(timeout * 1000)}ms"])
        if idle_timeout is not None:
            args.extend(["--idle-timeout", f"{int(idle_timeout * 1000)}ms"])
        if graceful_timeout is not None:
            args.extend(["--graceful-timeout", f"{int(graceful_timeout * 1000)}ms"])
        if working_dir is not None:
            args.extend(["--dir", working_dir])
        if env:
            for k, v in env.items():
                args.extend(["--env", f"{k}={v}"])
        if pty:
            args.append("--pty")
        if line_buffer != 4096:
            args.extend(["--line-buffer", str(line_buffer)])
        args.append(command)

        proc = subprocess.Popen(
            args,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        return Session(proc, pty=pty)

    def exec(
        self,
        command: str,
        timeout: Optional[float] = None,
        idle_timeout: Optional[float] = None,
        graceful_timeout: Optional[float] = None,
        working_dir: Optional[str] = None,
        env: Optional[dict] = None,
        initial_size: str = "24x80",
    ) -> "Session":
        """Start a command with a PTY, return a Session for interactive use.

        See :meth:`run` for parameter meanings. Unlike :meth:`start`,
        a PTY is always allocated.
        """
        args = [self.rein_path, "exec"]
        if timeout is not None:
            args.extend(["--timeout", f"{int(timeout * 1000)}ms"])
        if idle_timeout is not None:
            args.extend(["--idle-timeout", f"{int(idle_timeout * 1000)}ms"])
        if graceful_timeout is not None:
            args.extend(["--graceful-timeout", f"{int(graceful_timeout * 1000)}ms"])
        if working_dir is not None:
            args.extend(["--dir", working_dir])
        if env:
            for k, v in env.items():
                args.extend(["--env", f"{k}={v}"])
        args.extend(["--initial-size", initial_size])
        args.append(command)

        proc = subprocess.Popen(
            args,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
        return Session(proc, pty=True)


class Session:
    """A handle to a long-running rein process.

    Use it as a context manager or as an iterator over output
    lines. To stop the process, call :meth:`stop` or let the
    ``with`` block exit (which sends ``{"type": "stop"}`` to the
    rein CLI's stdin).
    """

    def __init__(self, proc: subprocess.Popen, pty: bool = False):
        self._proc = proc
        self._pty = pty
        self._stdout_thread: Optional[threading.Thread] = None
        self._stderr_thread: Optional[threading.Thread] = None
        self._lines: list[Line] = []
        self._lock = threading.Lock()
        self._cond = threading.Condition(self._lock)
        self._closed = False
        self._result: Optional[Result] = None
        self._start_reader_threads()

    def _start_reader_threads(self):
        def reader(stream, s):
            try:
                for raw in stream:
                    raw = raw.rstrip("\n")
                    if not raw:
                        continue
                    try:
                        msg = json.loads(raw)
                    except json.JSONDecodeError:
                        continue
                    self._handle_message(msg)
            except (ValueError, OSError):
                pass
            finally:
                try:
                    stream.close()
                except Exception:
                    pass

        self._stdout_thread = threading.Thread(
            target=reader, args=("stdout", self._proc.stdout), daemon=True
        )
        self._stdout_thread.start()
        self._stderr_thread = threading.Thread(
            target=reader, args=("stderr", self._proc.stderr), daemon=True
        )
        self._stderr_thread.start()

    def _handle_message(self, msg: dict):
        t = msg.get("type")
        if t == "line":
            with self._cond:
                self._lines.append(Line(stream=msg.get("stream", ""), text=msg.get("text", "")))
                self._cond.notify_all()
        elif t == "started":
            with self._cond:
                self._pid = msg.get("pid")
                self._cond.notify_all()
        elif t == "exit":
            with self._cond:
                self._result = Result(
                    exit_code=msg.get("exit_code", -1),
                    duration_ms=msg.get("duration_ms", 0),
                    stdout=msg.get("stdout", ""),
                    stderr=msg.get("stderr", ""),
                    err=msg.get("err"),
                )
                self._closed = True
                self._cond.notify_all()
        elif t == "error":
            with self._cond:
                self._result = Result(
                    exit_code=-1,
                    duration_ms=0,
                    err=msg.get("err"),
                )
                self._closed = True
                self._cond.notify_all()

    def lines(self) -> Iterator[Line]:
        """Iterate over output lines as they arrive. Blocks until the process exits."""
        while True:
            with self._cond:
                while self._lines:
                    yield self._lines.pop(0)
                if self._closed:
                    return
                self._cond.wait()

    def pid(self) -> Optional[int]:
        with self._cond:
            return getattr(self, "_pid", None)

    def wait(self) -> Result:
        """Block until the process exits and return the Result."""
        with self._cond:
            while not self._closed:
                self._cond.wait()
            return self._result

    def stop(self) -> Result:
        """Ask the rein CLI to stop the process. Returns the final Result."""
        if not self._closed and self._proc.stdin:
            try:
                self._proc.stdin.write(json.dumps({"type": "stop"}) + "\n")
                self._proc.stdin.flush()
            except (BrokenPipeError, OSError):
                pass
        return self.wait()

    def write(self, text: str) -> None:
        """Write text to the child's stdin. Requires pty=True."""
        if not self._pty:
            raise ReinError("write() requires exec() (PTY); use start() for non-PTY sessions")
        if not self._proc.stdin:
            raise ReinError("session stdin is closed")
        msg = json.dumps({"type": "input", "text": text}) + "\n"
        self._proc.stdin.write(msg)
        self._proc.stdin.flush()

    def resize(self, rows: int, cols: int) -> None:
        """Resize the PTY window. Requires pty=True."""
        if not self._pty:
            raise ReinError("resize() requires exec() (PTY)")
        if not self._proc.stdin:
            raise ReinError("session stdin is closed")
        msg = json.dumps({"type": "resize", "rows": rows, "cols": cols}) + "\n"
        self._proc.stdin.write(msg)
        self._proc.stdin.flush()

    def close(self) -> Result:
        """Alias for :meth:`stop`."""
        return self.stop()

    def __enter__(self) -> "Session":
        return self

    def __exit__(self, exc_type, exc_val, exc_tb) -> None:
        self.stop()
