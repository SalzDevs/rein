"""rein client - Python wrapper for the rein CLI.

rein is a small tool that runs shell commands the way AI agents
actually need: with timeouts that work, signals that propagate,
and process trees that get cleaned up.

This package is a thin Python wrapper around the rein CLI
binary. It speaks NDJSON over stdio and exposes a Pythonic
API. Install the rein binary first (go install
github.com/SalzDevs/rein/cmd/rein@latest) and then
``pip install rein-client``.
"""

from .client import Rein, Result, Line, Session, AsyncSession

__all__ = ["Rein", "Result", "Line", "Session", "AsyncSession"]
__version__ = "0.0.1"
