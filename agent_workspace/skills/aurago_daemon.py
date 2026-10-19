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
import threading
import queue

_stdout_lock = threading.Lock()
_transport_lock = threading.Lock()
_transport_instance = None


def _write_message(obj):
    line = json.dumps(obj, ensure_ascii=False, default=str)
    with _stdout_lock:
        sys.stdout.write(line + "\n")
        sys.stdout.flush()


class _DaemonTransport:
    """The only stdin reader, shared by daemon control and tool callers."""
    def __init__(self):
        self.stopped = threading.Event()
        self.commands = queue.Queue(maxsize=16)
        self._call_lock = threading.Lock()
        self._pending_lock = threading.Lock()
        self._pending = {}
        self._sequence = 0
        self._input = sys.stdin
        threading.Thread(target=self._read, name="aurago-daemon-stdin", daemon=True).start()

    def _stop(self):
        self.stopped.set()
        with self._pending_lock:
            for waiter in self._pending.values():
                try:
                    waiter.put_nowait({"status": "error", "result": "Daemon owner stopped; verify any pending action before retrying"})
                except queue.Full:
                    pass

    def _read(self):
        try:
            while True:
                line = self._input.readline((1 << 20) + 1)
                if not line or len(line.encode("utf-8")) > 1 << 20:
                    return
                try:
                    command = json.loads(line)
                except (ValueError, TypeError):
                    continue
                if not isinstance(command, dict):
                    continue
                if command.get("type") == "tool_result":
                    result = command.get("data", {})
                    if not isinstance(result, dict):
                        continue
                    with self._pending_lock:
                        waiter = self._pending.get(result.get("id"))
                        if waiter is not None:
                            try:
                                waiter.put_nowait(result)
                            except queue.Full:
                                pass
                elif command.get("type") in ("stop", "config_update"):
                    if command["type"] == "stop":
                        self._stop()
                    try:
                        self.commands.put_nowait(command)
                    except queue.Full:
                        pass
        finally:
            self._stop()

    def call_tool(self, tool, parameters, timeout):
        # One in-flight request keeps stdout protocol and quotas deterministic.
        with self._call_lock:
            if self.stopped.is_set():
                raise RuntimeError("Daemon owner has stopped")
            identity = self._sequence + 1
            timeout = min(60, max(1, int(timeout or 60)))
            frame = {"type": "tool_request", "data": {"id": identity, "tool": tool, "parameters": parameters, "timeout": timeout}}
            if len(json.dumps(frame).encode("utf-8")) > 64 << 10:
                raise ValueError("Daemon tool request exceeds 64 KiB")
            self._sequence = identity
            waiter = queue.Queue(maxsize=1)
            with self._pending_lock:
                self._pending[identity] = waiter
            try:
                _write_message(frame)
                try:
                    return waiter.get(timeout=timeout + 5)
                except queue.Empty:
                    self._stop()
                    raise TimeoutError("Daemon tool outcome unknown; verify effects before retrying")
            finally:
                with self._pending_lock:
                    self._pending.pop(identity, None)


def _daemon_transport():
    global _transport_instance
    with _transport_lock:
        if _transport_instance is None:
            _transport_instance = _DaemonTransport()
        return _transport_instance


def _call_tool(tool, parameters, timeout):
    return _daemon_transport().call_tool(tool, parameters, timeout)


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
        self._transport = _daemon_transport() if os.environ.get("AURAGO_DAEMON") == "1" else None

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
        return self._running and (self._transport is None or not self._transport.stopped.is_set())

    def next_command(self):
        """Return a queued stop/config_update command without consuming tool replies."""
        if self._transport is None:
            return None
        try:
            return self._transport.commands.get_nowait()
        except queue.Empty:
            return None

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
            _write_message(obj)
        except (BrokenPipeError, OSError):
            self._running = False

    def _handle_signal(self, signum, frame):
        """Handle SIGTERM/SIGINT for graceful shutdown."""
        self._running = False
