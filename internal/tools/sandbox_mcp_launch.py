"""Launch llm-sandbox MCP and apply AuraGo's IMAGE to every session."""

import os
import json
import io
import tarfile
import threading
import urllib.error
import urllib.parse
import urllib.request

import llm_sandbox.mcp_server.server as server

_IMAGE = os.environ.get("IMAGE", "").strip()
_IMAGES = json.loads(os.environ.get("AURAGO_SANDBOX_IMAGES", "{}"))

_BROKER_PREFIX = b"\x1eAURAGO_TOOL:"
_BROKER_FRAME_LIMIT = 65536
_BROKER_RESPONSE_LIMIT = 1048576

_BROKER_SDK = r'''
import sys as _aurago_sys, types as _aurago_types, json as _aurago_json
import os as _aurago_os, time as _aurago_time
class _AuraGoBrokerModule(_aurago_types.ModuleType):
    def __init__(self):
        super().__init__("aurago")
        self._calls = 0
    def call_tool(self, tool_name, parameters=None, timeout=60):
        if not isinstance(tool_name, str) or not tool_name.strip():
            raise ValueError("tool_name must be a non-empty string")
        if parameters is None:
            parameters = {}
        if not isinstance(parameters, dict):
            raise TypeError("parameters must be a dict")
        if self._calls >= 32:
            raise RuntimeError("AuraGo tool bridge call limit exceeded")
        self._calls += 1
        request = {"id": self._calls, "tool": tool_name.strip(), "parameters": parameters, "timeout": min(60, max(1, int(timeout or 60)))}
        encoded = _aurago_json.dumps(request, separators=(",", ":"))
        if len(encoded.encode()) > 65536:
            raise ValueError("AuraGo tool bridge request exceeds frame limit")
        _aurago_sys.stdout.write("\n\x1eAURAGO_TOOL:" + encoded + "\n")
        _aurago_sys.stdout.flush()
        path = "/sandbox/.aurago_bridge_%08d.json" % self._calls
        deadline = _aurago_time.monotonic() + request["timeout"] + 10
        while not _aurago_os.path.isfile(path):
            if _aurago_time.monotonic() >= deadline:
                raise TimeoutError("AuraGo tool bridge response timed out")
            _aurago_time.sleep(0.02)
        with open(path, "r", encoding="utf-8") as response:
            payload = _aurago_json.load(response)
        _aurago_os.unlink(path)
        if payload.get("status") == "error":
            raise RuntimeError(str(payload.get("result", "AuraGo tool bridge call failed")))
        return payload
_aurago_sys.modules["aurago"] = _AuraGoBrokerModule()
del _AuraGoBrokerModule
'''


class _Broker:
    def __init__(self, url, token, allowed, limit):
        parsed = urllib.parse.urlsplit(url)
        if parsed.scheme not in ("http", "https") or parsed.hostname not in ("localhost", "127.0.0.1", "::1") or parsed.username or parsed.password or parsed.query or parsed.fragment:
            raise ValueError("Tool Bridge broker requires the local control origin")
        self.url, self.token = url.rstrip("/"), token
        self.allowed = set(allowed)
        self.limit = min(32, max(1, int(limit or 8)))
        self.calls = 0
        self.pending = bytearray()

    def stdout(self, session, data):
        self.pending.extend(data)
        result = bytearray()
        while b"\n" in self.pending:
            line, _, rest = self.pending.partition(b"\n")
            self.pending = bytearray(rest)
            if line.startswith(_BROKER_PREFIX):
                frame = line[len(_BROKER_PREFIX):]
                if len(frame) > _BROKER_FRAME_LIMIT:
                    raise ValueError("Tool Bridge request exceeds frame limit")
                self.dispatch(session, json.loads(frame))
            else:
                result.extend(line + b"\n")
        if len(self.pending) > _BROKER_FRAME_LIMIT and self.pending.startswith(_BROKER_PREFIX):
            raise ValueError("Tool Bridge request exceeds frame limit")
        if len(self.pending) > _BROKER_RESPONSE_LIMIT:
            raise ValueError("Sandbox output exceeds byte limit")
        return bytes(result)

    def dispatch(self, session, frame):
        self.calls += 1
        if not isinstance(frame, dict) or frame.get("id") != self.calls or self.calls > self.limit:
            raise ValueError("Tool Bridge call limit or sequence rejected")
        tool = frame.get("tool")
        if not isinstance(tool, str) or tool not in self.allowed or not isinstance(frame.get("parameters"), dict):
            payload = {"status": "error", "result": "Tool Bridge tool or parameters denied"}
        else:
            timeout = min(60, max(1, int(frame.get("timeout", 30))))
            request = urllib.request.Request(self.url + "/" + urllib.parse.quote(tool, safe=""),
                data=json.dumps({"parameters": frame["parameters"], "timeout": timeout}).encode(),
                headers={"Content-Type": "application/json", "X-Internal-Token": self.token}, method="POST")
            # Never redirect the scoped grant away from its local origin.
            class NoRedirect(urllib.request.HTTPRedirectHandler):
                def redirect_request(self, req, fp, code, msg, headers, newurl):
                    return None
            try:
                with urllib.request.build_opener(NoRedirect).open(request, timeout=timeout + 5) as response:
                    body = response.read(_BROKER_RESPONSE_LIMIT + 1)
                    if len(body) > _BROKER_RESPONSE_LIMIT:
                        raise ValueError("Tool Bridge response exceeds byte limit")
                    payload = json.loads(body)
            except Exception:
                # Transport diagnostics may include the grant-bearing request.
                payload = {"status": "error", "result": "Tool Bridge request failed or denied"}
        encoded = json.dumps(payload).encode()
        if len(encoded) > _BROKER_RESPONSE_LIMIT:
            encoded = b'{"status":"error","result":"Tool Bridge response exceeds byte limit"}'
        archive = io.BytesIO()
        with tarfile.open(fileobj=archive, mode="w") as tar:
            entry = tarfile.TarInfo(".aurago_bridge_%08d.json" % self.calls)
            entry.size, entry.mode, entry.uid, entry.gid = len(encoded), 0o600, 0, 0
            tar.addfile(entry, io.BytesIO(encoded))
        if not session.container.put_archive("/sandbox", archive.getvalue()):
            raise RuntimeError("Tool Bridge response publication failed")


def _harden(kwargs):
    if not _IMAGES:
        if _IMAGE and "image" not in kwargs:
            kwargs["image"] = _IMAGE
        return kwargs
    language = kwargs.get("lang", "python")
    image = _IMAGE or _IMAGES.get(language)
    if not image:
        raise ValueError("Configure a digest-pinned sandbox image for " + str(language))
    kwargs["image"] = image
    kwargs["commit_container"] = False
    kwargs["keep_template"] = True
    kwargs["skip_environment_setup"] = True
    kwargs["stream"] = True
    kwargs["workdir"] = "/sandbox"
    kwargs["runtime_configs"] = {
        "network_mode": "none",
        "read_only": True,
        "privileged": False,
        "cap_drop": ["ALL"],
        # Upstream uploads private 0600 files with their supervisor UID. Root
        # needs DAC_OVERRIDE to read those; all other capabilities stay dropped.
        "cap_add": ["DAC_OVERRIDE"],
        "security_opt": ["no-new-privileges:true"],
        "mem_limit": os.environ.get("SANDBOX_MEM_LIMIT", "512m"),
        "cpu_period": 100000,
        "cpu_quota": int(float(os.environ.get("SANDBOX_CPUS", "1")) * 100000),
        "pids_limit": int(os.environ.get("AURAGO_SANDBOX_PIDS", "64")),
        # An independent PID-1 deadline also ends the worker if its supervisor
        # dies while a request is cancelled. No container can run indefinitely.
        "entrypoint": ["/bin/sh", "-c"],
        "command": ["exec sleep " + str(min(3600, max(1, int(kwargs.get("session_timeout") or 30))))],
        "auto_remove": True,
        "tmpfs": {"/sandbox": "rw,nosuid,nodev,size=128m,mode=1777", "/tmp": "rw,nosuid,nodev,noexec,size=64m,mode=1777"},
        "environment": {"PATH": "/usr/local/bin:/usr/bin:/bin", "HOME": "/sandbox", "PYTHONDONTWRITEBYTECODE": "1"},
    }
    return kwargs


def _wrap_class(cls):
    original = cls.__init__

    def init(self, *args, **kwargs):
        return original(self, *args, **_harden(kwargs))

    cls.__init__ = init


def _session_function(original):
    def session(*args, **kwargs):
        return original(*args, **_harden(kwargs))

    return session


if isinstance(server.SandboxSession, type):
    _wrap_class(server.SandboxSession)
else:
    server.SandboxSession = _session_function(server.SandboxSession)

if isinstance(server.ArtifactSandboxSession, type):
    _wrap_class(server.ArtifactSandboxSession)

if _IMAGES:
    # Generic workers never install plotting libraries or execute setup code.
    server._supports_visualization = lambda language: False
    from llm_sandbox.docker import SandboxDockerSession

    def _bounded_output(self, output, on_stdout=None, on_stderr=None):
        limit = int(os.environ.get("AURAGO_SANDBOX_OUTPUT", "1048576"))
        chunks = [bytearray(), bytearray()]
        total = 0
        broker = getattr(self, "_aurago_broker", None)
        for pair in output:
            for index, chunk in enumerate(pair):
                if chunk:
                    data = chunk if isinstance(chunk, bytes) else str(chunk).encode()
                    if broker and index == 0:
                        data = broker.stdout(self, data)
                    remaining = max(0, limit - total)
                    chunks[index].extend(data[:remaining])
                    total += len(data)
                    if total > limit:
                        self.container.kill()
                        raise RuntimeError("Sandbox output exceeded configured byte limit")
        if broker and broker.pending:
            if total + len(broker.pending) > limit:
                raise RuntimeError("Sandbox output exceeded configured byte limit")
            chunks[0].extend(broker.pending)
        return tuple(bytes(value).decode("utf-8", errors="replace") for value in chunks)

    SandboxDockerSession._process_stream_output = _bounded_output

    def _execute_supervised(code, language, timeout, cancel_file, broker=None):
        if not os.path.isabs(cancel_file) or not os.path.basename(cancel_file).startswith("aurago-sandbox-cancel-"):
            raise ValueError("Invalid supervisor cancellation channel")
        finished = threading.Event()
        session = server.SandboxSession(lang=language, backend=server._get_backend(), session_timeout=timeout)
        def watch_cancel():
            while not finished.wait(0.025):
                try:
                    with open(cancel_file, "rb") as marker:
                        cancelled = marker.read(32) == b"cancel"
                    container = getattr(session, "container", None)
                    if cancelled and container is not None:
                        container.kill()
                        with open(cancel_file, "wb") as marker:
                            marker.write(b"terminated")
                        return
                except (OSError, Exception):
                    if finished.is_set():
                        return
        watcher = threading.Thread(target=watch_cancel, name="aurago-worker-cancellation", daemon=True)
        watcher.start()
        try:
            with session:
                if broker:
                    session._aurago_broker = broker
                return session.run(code=code, libraries=[], timeout=timeout)
        finally:
            finished.set()
            watcher.join(timeout=1)

    @server.mcp.tool()
    def execute_supervised_code(code: str, cancel_file: str, language: str = "python", timeout: int = 30) -> list[server.TextContent]:
        try:
            result = _execute_supervised(code, language, timeout, cancel_file)
            return [server.TextContent(text=result.to_json(include_plots=False), type="text")]
        except Exception:
            return [server.TextContent(text=json.dumps({"exit_code": 1, "stdout": "", "stderr": "Supervised sandbox execution failed; verify runtime setup"}), type="text")]

    @server.mcp.tool()
    def execute_brokered_code(code: str, broker_url: str, broker_token: str,
                              allowed_tools: list[str], cancel_file: str, call_limit: int = 8,
                              timeout: int = 30) -> list[server.TextContent]:
        try:
            broker = _Broker(broker_url, broker_token, allowed_tools, call_limit)
            result = _execute_supervised(_BROKER_SDK + "\n" + code, "python", timeout, cancel_file, broker)
            return [server.TextContent(text=result.to_json(include_plots=False), type="text")]
        except Exception:
            return [server.TextContent(text=json.dumps({"exit_code": 1, "stdout": "", "stderr": "Scoped Tool Bridge sandbox execution failed"}), type="text")]

server.main()
