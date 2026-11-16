#!/usr/bin/env python3
"""
Ansible API Sidecar for AuraGo.

Provides a lightweight HTTP API that wraps Ansible CLI commands.
No third-party dependencies beyond ansible itself — uses only stdlib http.server.

Endpoints:
  GET  /status          — health check, returns ansible version
  GET  /playbooks       — list .yml/.yaml files under PLAYBOOKS_DIR
  GET  /inventory       — parse inventory, return host list (optional ?inventory=path)
  POST /run/ping        — ansible ping against host(s)
  POST /run/adhoc       — run an ad-hoc ansible module
  POST /run/playbook    — run a playbook (supports --check, --diff, --tags, --limit, extra_vars)
  POST /run/facts       — gather facts from host(s) via setup module
"""

import glob
import json
import os
import re
import signal
import stat
import subprocess
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from socketserver import ThreadingMixIn
from urllib.parse import parse_qs, urlparse

# ──────────────────────────────────────────────────────────────────────────────
# Configuration (via environment variables)
# ──────────────────────────────────────────────────────────────────────────────
TOKEN             = os.environ.get("ANSIBLE_API_TOKEN", "")
ALLOW_UNAUTH      = os.environ.get("ANSIBLE_API_ALLOW_UNAUTH", "").lower() in {"1", "true", "yes"}
PLAYBOOKS_DIR     = os.environ.get("PLAYBOOKS_DIR", "/playbooks")
DEFAULT_INVENTORY = os.environ.get("DEFAULT_INVENTORY", "")
ANSIBLE_TIMEOUT   = int(os.environ.get("ANSIBLE_TIMEOUT", "300"))
PORT              = int(os.environ.get("PORT", "5001"))
MAX_BODY_SIZE     = int(os.environ.get("MAX_BODY_SIZE", str(2 * 1024 * 1024)))  # 2 MB default
READ_ONLY         = os.environ.get("ANSIBLE_READONLY", "false").lower() in {"1", "true", "yes"}

# ──────────────────────────────────────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────────────────────────────────────

def check_auth(handler: "Handler") -> bool:
    """Return True if the request is authenticated (or no token is required)."""
    if not TOKEN:
        return ALLOW_UNAUTH
    auth = handler.headers.get("Authorization", "")
    return auth == f"Bearer {TOKEN}"


def run_ansible(cmd: list, timeout: int = None) -> dict:
    """Execute an ansible command as a subprocess and return rc/stdout/stderr."""
    effective_timeout = timeout or ANSIBLE_TIMEOUT
    env = {
        **{key: os.environ[key] for key in ("PATH", "HOME", "USER", "LANG", "LC_ALL", "SYSTEMROOT", "TEMP", "TMP") if key in os.environ},
        "ANSIBLE_FORCE_COLOR": "0",
        "ANSIBLE_NOCOLOR":     "1",
        "ANSIBLE_HOST_KEY_CHECKING": "True",
        "ANSIBLE_INVENTORY_ENABLED": "ini,yaml",
    }
    try:
        if READ_ONLY and (cmd[0] == "ansible-playbook" or (cmd[0] == "ansible" and "-m" in cmd and cmd[cmd.index("-m") + 1] not in {"ping", "setup"})):
            raise ValueError("ansible.readonly blocks playbooks (including check/dry_run) and arbitrary modules")
        if cmd[0] == "ansible" and len(cmd) > 1 and cmd[1].startswith("-") and cmd[1] != "--version":
            raise ValueError("invalid host pattern")
        with tempfile.TemporaryDirectory(prefix="aurago-ansible-") as staging:
            cmd = list(cmd)
            if "-i" in cmd:
                index = cmd.index("-i") + 1
                data, extension = read_static_inventory(cmd[index]) if cmd[index] else (b"localhost ansible_connection=local\n", ".ini")
                inventory = os.path.join(staging, "inventory" + extension)
                with open(inventory, "xb") as target:
                    target.write(data)
                cmd[index] = inventory
            env["ANSIBLE_CONFIG"] = os.path.join(staging, "safe.cfg")
            with open(env["ANSIBLE_CONFIG"], "x", encoding="utf-8") as config:
                config.write("[defaults]\nhost_key_checking=True\n[inventory]\nenable_plugins=ini,yaml\n")
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=effective_timeout, env=env, cwd=staging)
        return {
            "rc":     result.returncode,
            "stdout": scrub_token(result.stdout.strip()),
            "stderr": scrub_token(result.stderr.strip()),
        }
    except subprocess.TimeoutExpired:
        return {
            "rc":     -1,
            "stdout": "",
            "stderr": f"Command timed out after {effective_timeout}s",
        }
    except FileNotFoundError:
        return {
            "rc":     -1,
            "stdout": "",
            "stderr": "Ansible not found. Is it installed in this container?",
        }
    except ValueError as exc:
        return {"rc": -1, "stdout": "", "stderr": str(exc)}
    except Exception:
        return {"rc": -1, "stdout": "", "stderr": "Ansible input or execution failed"}


def scrub_token(value: str) -> str:
    return value.replace(TOKEN, "[redacted]") if TOKEN else value


def read_static_inventory(value: str) -> tuple[bytes, str]:
    """Snapshot a bounded non-executable inventory without following symlinks."""
    if not isinstance(value, str) or not value or "\x00" in value:
        raise ValueError("inventory path is required")
    root = os.path.abspath(PLAYBOOKS_DIR)
    if value == DEFAULT_INVENTORY and os.path.isabs(value):
        root = os.path.dirname(value)
    path = os.path.abspath(value if os.path.isabs(value) else os.path.join(root, value))
    if os.path.commonpath([root, path]) != root or path == root:
        raise ValueError("inventory path escapes the configured root")
    relative = os.path.relpath(path, root)
    extension = os.path.splitext(path)[1].lower()
    if extension not in {"", ".ini", ".yaml", ".yml"}:
        raise ValueError("only static INI/YAML inventories are supported")
    descriptors = []
    try:
        if os.open in os.supports_dir_fd:
            parent = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            descriptors.append(parent)
            parts = relative.split(os.sep)
            for part in parts[:-1]:
                parent = os.open(part, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW, dir_fd=parent)
                descriptors.append(parent)
            fd = os.open(parts[-1], os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK, dir_fd=parent)
        else:
            current = root
            for part in relative.split(os.sep):
                current = os.path.join(current, part)
                info = os.lstat(current)
                if stat.S_ISLNK(info.st_mode) or getattr(info, "st_file_attributes", 0) & 0x400:
                    raise ValueError("inventory symlinks are not allowed")
            if not stat.S_ISREG(info.st_mode):
                raise ValueError("inventory must be a regular file")
            fd = os.open(path, os.O_RDONLY | getattr(os, "O_BINARY", 0))
            if not os.path.samestat(info, os.fstat(fd)):
                os.close(fd)
                raise ValueError("inventory changed while opening")
        with os.fdopen(fd, "rb") as source:
            info = os.fstat(source.fileno())
            if not stat.S_ISREG(info.st_mode) or info.st_mode & 0o111 or info.st_size > 1024 * 1024:
                raise ValueError("inventory must be a bounded non-executable regular file")
            data = source.read(1024 * 1024 + 1)
        if len(data) > 1024 * 1024 or b"\x00" in data or data.startswith(b"#!") or re.search(rb"(?im)^\s*['\"]?plugin['\"]?\s*:", data):
            raise ValueError("only bounded static INI/YAML inventories are supported")
        return data, extension or ".ini"
    finally:
        for descriptor in reversed(descriptors):
            os.close(descriptor)


def inventory_summary(data: dict) -> dict:
    """Expose topology and connection metadata, never arbitrary inventory vars."""
    result = {}
    for name, group in data.items():
        if name == "_meta" or not isinstance(group, dict):
            continue
        result[name] = {key: value for key in ("hosts", "children") if isinstance(value := group.get(key), list) and all(isinstance(item, str) for item in value)}
    allowed = {"ansible_host", "ansible_port", "ansible_user", "ansible_connection"}
    hosts = data.get("_meta", {}).get("hostvars", {})
    result["_meta"] = {"hostvars": {host: {key: value for key, value in fields.items() if key in allowed and isinstance(value, (str, int))} for host, fields in hosts.items() if isinstance(fields, dict)}}
    return result


def inventory_arg(body: dict) -> str:
    """Return the inventory path from the request body or fall back to default."""
    return body.get("inventory") or DEFAULT_INVENTORY


def extra_vars_arg(extra_vars) -> str | None:
    """Normalise extra_vars to a JSON string, or return None."""
    if not extra_vars:
        return None
    if isinstance(extra_vars, dict):
        return json.dumps(extra_vars)
    return str(extra_vars)


def safe_playbook_path(playbook: str) -> str | None:
    """Resolve a playbook name to an absolute path inside PLAYBOOKS_DIR.

    Returns None if the path escapes PLAYBOOKS_DIR (path traversal).
    """
    if not playbook:
        return None
    # Reject absolute paths that don't live under PLAYBOOKS_DIR
    if os.path.isabs(playbook):
        real = os.path.realpath(playbook)
        base = os.path.realpath(PLAYBOOKS_DIR)
        if not real.startswith(base + os.sep) and real != base:
            return None
        return real
    # Relative path — join and verify it stays inside PLAYBOOKS_DIR
    joined = os.path.realpath(os.path.join(PLAYBOOKS_DIR, playbook))
    base = os.path.realpath(PLAYBOOKS_DIR)
    if not joined.startswith(base + os.sep) and joined != base:
        return None
    return joined


# ──────────────────────────────────────────────────────────────────────────────
# HTTP Handler
# ──────────────────────────────────────────────────────────────────────────────

class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):  # noqa: N802
        print(f"[ansible-api] {self.address_string()} {fmt % args}", flush=True)

    # ── response helpers ──────────────────────────────────────────────────────

    def send_json(self, code: int, data: dict):
        body = json.dumps(data).encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def read_json_body(self) -> dict | None:
        length = int(self.headers.get("Content-Length", 0))
        if length <= 0:
            return {}
        if length > MAX_BODY_SIZE:
            self.send_json(413, {"status": "error", "message": f"Request body too large (max {MAX_BODY_SIZE} bytes)"})
            return None
        try:
            return json.loads(self.rfile.read(length))
        except Exception as exc:
            self.send_json(400, {"status": "error", "message": f"Invalid JSON: {exc}"})
            return None

    def require_auth(self) -> bool:
        """Send 401 and return False if auth fails."""
        if not check_auth(self):
            self.send_json(401, {"status": "error", "message": "Unauthorized"})
            return False
        return True

    # ── GET ───────────────────────────────────────────────────────────────────

    def do_GET(self):  # noqa: N802
        if not self.require_auth():
            return
        path = urlparse(self.path).path

        if path == "/status":
            self._handle_status()
        elif path == "/playbooks":
            self._handle_list_playbooks()
        elif path == "/inventory":
            inv = parse_qs(urlparse(self.path).query).get("inventory", [None])[0] or DEFAULT_INVENTORY
            self._handle_inventory(inv)
        else:
            self.send_json(404, {"status": "error", "message": f"Unknown endpoint: {path}"})

    def _handle_status(self):
        out = run_ansible(["ansible", "--version"])
        version_line = out["stdout"].split("\n")[0] if out["stdout"] else "unknown"
        self.send_json(200, {
            "status":             "ok",
            "security_policy":    2,
            "version":            version_line,
            "token_configured":   bool(TOKEN),
            "playbooks_dir":      PLAYBOOKS_DIR,
            "default_inventory":  DEFAULT_INVENTORY,
            "host_key_checking":  os.environ.get("ANSIBLE_HOST_KEY_CHECKING", "True"),
        })

    def _handle_list_playbooks(self):
        files = sorted(
            glob.glob(f"{PLAYBOOKS_DIR}/**/*.yml",  recursive=True) +
            glob.glob(f"{PLAYBOOKS_DIR}/**/*.yaml", recursive=True)
        )
        relative = [os.path.relpath(f, PLAYBOOKS_DIR) for f in files]
        self.send_json(200, {"status": "ok", "count": len(relative), "playbooks": relative})

    def _handle_inventory(self, inv: str):
        out = run_ansible(["ansible-inventory", "-i", inv, "--list"])
        if out["rc"] == 0:
            try:
                data = json.loads(out["stdout"])
                self.send_json(200, {"status": "ok", "inventory": inventory_summary(data)})
            except Exception:
                self.send_json(502, {"status": "error", "message": "Invalid inventory response"})
        else:
            self.send_json(400, {"status": "error", "message": "Inventory rejected or unavailable"})

    # ── POST ──────────────────────────────────────────────────────────────────

    def do_POST(self):  # noqa: N802
        if not self.require_auth():
            return
        path   = urlparse(self.path).path
        body   = self.read_json_body()
        if body is None:
            return
        routes = {
            "/run/ping":     self._handle_ping,
            "/run/adhoc":    self._handle_adhoc,
            "/run/playbook": self._handle_playbook,
            "/run/facts":    self._handle_facts,
        }
        handler = routes.get(path)
        if handler:
            handler(body)
        else:
            self.send_json(404, {"status": "error", "message": f"Unknown endpoint: {path}"})

    def _handle_ping(self, body: dict):
        hosts = body.get("hosts", "all")
        inv   = inventory_arg(body)
        cmd   = ["ansible", hosts, "-i", inv, "-m", "ping"]
        out   = run_ansible(cmd)
        self.send_json(200, {"status": "ok" if out["rc"] == 0 else "error", **out})

    def _handle_adhoc(self, body: dict):
        hosts       = body.get("hosts", "all")
        module      = body.get("module", "ping")
        module_args = body.get("args") or body.get("module_args", "")
        inv         = inventory_arg(body)
        cmd         = ["ansible", hosts, "-i", inv, "-m", module]
        if module_args:
            cmd += ["-a", module_args]
        ev = extra_vars_arg(body.get("extra_vars"))
        if ev:
            cmd += ["-e", ev]
        out = run_ansible(cmd)
        self.send_json(200, {"status": "ok" if out["rc"] == 0 else "error", **out})

    def _handle_playbook(self, body: dict):
        playbook = body.get("playbook")
        if not playbook:
            self.send_json(400, {"status": "error", "message": "'playbook' field is required"})
            return

        playbook = safe_playbook_path(playbook)
        if playbook is None:
            self.send_json(400, {"status": "error", "message": "Playbook path is invalid or escapes the playbooks directory"})
            return

        inv  = inventory_arg(body)
        cmd  = ["ansible-playbook", playbook, "-i", inv]

        if body.get("check"):
            cmd.append("--check")
        if body.get("diff"):
            cmd.append("--diff")
        if body.get("tags"):
            cmd += ["--tags", body["tags"]]
        if body.get("skip_tags"):
            cmd += ["--skip-tags", body["skip_tags"]]
        if body.get("limit"):
            cmd += ["--limit", body["limit"]]
        if body.get("verbose"):
            cmd.append("-v")
        ev = extra_vars_arg(body.get("extra_vars"))
        if ev:
            cmd += ["-e", ev]

        out = run_ansible(cmd)
        self.send_json(200, {"status": "ok" if out["rc"] == 0 else "error", **out})

    def _handle_facts(self, body: dict):
        hosts = body.get("hosts") or body.get("host", "all")
        inv   = inventory_arg(body)
        cmd   = ["ansible", hosts, "-i", inv, "-m", "setup"]
        ev    = extra_vars_arg(body.get("extra_vars"))
        if ev:
            cmd += ["-e", ev]
        out   = run_ansible(cmd)
        # setup returns a LOT of data — trim stdout to 8 KB to avoid overwhelming the agent
        out["stdout"] = out["stdout"][:8000]
        self.send_json(200, {"status": "ok" if out["rc"] == 0 else "error", **out})


# ──────────────────────────────────────────────────────────────────────────────
# Threaded HTTP Server
# ──────────────────────────────────────────────────────────────────────────────

class ThreadedHTTPServer(ThreadingMixIn, HTTPServer):
    """Handle each request in a separate thread so long-running playbook runs
    do not block other requests (e.g. /status health checks)."""
    daemon_threads = True


# ──────────────────────────────────────────────────────────────────────────────
# Entry point
# ──────────────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    print(f"[ansible-api] Starting on 0.0.0.0:{PORT}", flush=True)
    print(f"[ansible-api] Playbooks : {PLAYBOOKS_DIR}", flush=True)
    print(f"[ansible-api] Inventory : {DEFAULT_INVENTORY}", flush=True)
    print(f"[ansible-api] Max body  : {MAX_BODY_SIZE} bytes", flush=True)
    if not TOKEN:
        if not ALLOW_UNAUTH:
            print("[ansible-api] ERROR: ANSIBLE_API_TOKEN is required unless ANSIBLE_API_ALLOW_UNAUTH=1 is set.", flush=True)
            raise SystemExit(1)
        print("[ansible-api] WARNING: ANSIBLE_API_ALLOW_UNAUTH=1 — API is unauthenticated!", flush=True)

    server = ThreadedHTTPServer(("0.0.0.0", PORT), Handler)

    # Graceful shutdown on SIGTERM / SIGINT
    def _shutdown(signum, frame):
        print("[ansible-api] Shutting down …", flush=True)
        threading.Thread(target=server.shutdown).start()

    signal.signal(signal.SIGTERM, _shutdown)
    signal.signal(signal.SIGINT, _shutdown)

    server.serve_forever()
