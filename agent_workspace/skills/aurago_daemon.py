"""AuraGo Daemon Skill SDK — minimal, zero-dependency helper for long-running skills.

Usage:
    from aurago_daemon import AuraGoDaemon

    daemon = AuraGoDaemon()
    while daemon.is_running():
        # ... your monitoring logic ...
        if something_happened:
            daemon.wake_agent("Alert: something happened", severity="warning")
        daemon.heartbeat()
        time.sleep(30)
"""

import json
import os
import signal
import sys
import time


class AuraGoDaemon:
    """Base class for AuraGo daemon skills.

    Handles IPC protocol (JSON over stdout/stdin), graceful shutdown via
    SIGTERM, rate limiting of wake_agent calls, and heartbeat emission.
    """

    def __init__(self):
        self._running = True
        self._last_wake = 0.0
        self._rate_limit = int(os.environ.get("AURAGO_WAKE_RATE_LIMIT", "3000"))
        self._start_time = time.time()

        # Register graceful shutdown handler
        try:
            signal.signal(signal.SIGTERM, self._handle_signal)
            signal.signal(signal.SIGINT, self._handle_signal)
        except (OSError, ValueError):
            pass  # Signal handling may not be available on all platforms

        # Listen for commands on stdin in a non-blocking way
        self._check_stdin = hasattr(sys.stdin, "readline")

    # ── Public API ──────────────────────────────────────────────────────

    def wake_agent(self, message: str, severity: str = "info", **data) -> bool:
        """Send a wake-up event to the AuraGo agent.

        Args:
            message: Human-readable event description.
            severity: One of "info", "warning", "critical".
            **data: Additional key-value pairs included in the event payload.

        Returns:
            True if the event was sent, False if rate-limited (too soon).
        """
        now = time.time()
        if now - self._last_wake < self._rate_limit:
            return False
        self._last_wake = now
        payload = {"type": "wake_agent", "message": str(message), "severity": severity}
        if data:
            payload["data"] = data
        self._send(payload)
        return True

    def log(self, message: str, level: str = "info", **fields):
        """Emit a log message forwarded to the AuraGo log system.

        Args:
            message: Log message text.
            level: Log level — "debug", "info", "warn", "error".
            **fields: Additional structured fields.
        """
        payload = {"type": "log", "level": level, "message": str(message)}
        if fields:
            payload.update(fields)
        self._send(payload)

    def metric(self, key: str, value, unit: str = ""):
        """Emit a metric data point.

        Args:
            key: Metric name (e.g. "cpu_percent", "disk_usage").
            value: Numeric value.
            unit: Optional unit string (e.g. "%", "MB", "ms").
        """
        self._send({"type": "metric", "key": key, "value": value, "unit": unit})

    def heartbeat(self):
        """Send a heartbeat to indicate the daemon is alive and healthy."""
        self._send({"type": "heartbeat"})

    def is_running(self) -> bool:
        """Check if the daemon should continue running.

        Returns False after SIGTERM/SIGINT or when the parent pipe breaks.
        """
        return self._running

    def uptime(self) -> float:
        """Return seconds since daemon start."""
        return time.time() - self._start_time

    def shutdown(self, reason: str = ""):
        """Request graceful shutdown from the Go supervisor."""
        payload = {"type": "shutdown"}
        if reason:
            payload["reason"] = reason
        self._send(payload)
        self._running = False

    # ── Internal ────────────────────────────────────────────────────────

    def _send(self, obj: dict):
        """Write a JSON line to stdout (IPC channel to Go supervisor)."""
        try:
            line = json.dumps(obj, ensure_ascii=False, default=str)
            sys.stdout.write(line + "\n")
            sys.stdout.flush()
        except (BrokenPipeError, OSError):
            self._running = False

    def _handle_signal(self, signum, frame):
        """Handle SIGTERM/SIGINT for graceful shutdown."""
        self._running = False
