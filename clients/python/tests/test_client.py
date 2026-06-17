"""Tests for the rein Python client.

These tests require the rein binary to be installed and on $PATH
(or in the parent directory of clients/python/ as a relative
``rein`` binary).
"""

import os
import subprocess
import sys
import time
import unittest

# Add the parent directory to sys.path so we can import the
# rein package.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from rein import Rein, ReinError, Line  # noqa: E402


def rein_binary_available() -> bool:
    """Return True if the rein binary is available."""
    try:
        result = subprocess.run(
            ["rein", "version"],
            capture_output=True,
            text=True,
            timeout=5,
        )
        return result.returncode == 0 and "rein" in result.stdout
    except (FileNotFoundError, subprocess.TimeoutExpired):
        return False


@unittest.skipUnless(rein_binary_available(), "rein binary not available")
class TestReinRun(unittest.TestCase):
    def setUp(self):
        self.rein = Rein()

    def test_run_basic(self):
        result = self.rein.run("echo hello")
        self.assertEqual(result.exit_code, 0)
        self.assertIn("hello", result.stdout)
        self.assertTrue(result.ok)

    def test_run_nonzero_exit(self):
        result = self.rein.run("exit 7")
        self.assertEqual(result.exit_code, 7)
        self.assertFalse(result.ok)

    def test_run_timeout(self):
        start = time.time()
        result = self.rein.run("sleep 5", timeout=0.2)
        elapsed = time.time() - start
        self.assertLess(elapsed, 2.0)
        self.assertNotEqual(result.exit_code, 0)
        self.assertIn("timed out", (result.err or "").lower())

    def test_run_env(self):
        result = self.rein.run("echo $REIN_TEST_VAR", env={"REIN_TEST_VAR": "from-env"})
        self.assertIn("from-env", result.stdout)


@unittest.skipUnless(rein_binary_available(), "rein binary not available")
class TestReinStart(unittest.TestCase):
    def setUp(self):
        self.rein = Rein()

    def test_start_streams_lines(self):
        with self.rein.start("echo line-1; echo line-2; echo line-3") as session:
            lines = list(session.lines())
        # We may get all three lines (most likely) or fewer if
        # the process exited before we drained everything.
        texts = [l.text for l in lines]
        self.assertIn("line-1", texts)
        self.assertIn("line-2", texts)
        self.assertIn("line-3", texts)

    def test_start_stop(self):
        with self.rein.start("sleep 30") as session:
            result = session.stop()
        self.assertNotEqual(result.exit_code, 0)


@unittest.skipUnless(rein_binary_available(), "rein binary not available")
class TestReinExec(unittest.TestCase):
    def setUp(self):
        self.rein = Rein()

    def test_exec_input(self):
        with self.rein.exec("cat") as session:
            session.write("hello-from-input\n")
            # Read at least one line.
            lines = []
            for line in session.lines():
                lines.append(line)
                if line.text == "hello-from-input":
                    break
            session.stop()
        texts = [l.text for l in lines]
        self.assertIn("hello-from-input", texts)


if __name__ == "__main__":
    unittest.main()
