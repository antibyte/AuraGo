"""Protocol fixtures for the pinned supervisor; never starts Docker."""

import importlib.util
import io
import json
import os
import http.server
from pathlib import Path
import shutil
import ssl
import subprocess
import sys
import tarfile
import tempfile
import threading
import time
import types
import unittest
from unittest.mock import patch


class MCP:
    def __init__(self):
        self.tools = {}

    def tool(self):
        def register(fn):
            self.tools[fn.__name__] = fn
            return fn
        return register


class Session:
    def __init__(self, **kwargs):
        self.options = kwargs


class Container:
    def __init__(self):
        self.killed = False
        self.responses = []

    def kill(self):
        self.killed = True

    def put_archive(self, path, data):
        with tarfile.open(fileobj=io.BytesIO(data)) as archive:
            entry = archive.getmembers()[0]
            self.responses.append((path, entry, json.load(archive.extractfile(entry))))
        return True


def load_launcher():
    server = types.ModuleType("llm_sandbox.mcp_server.server")
    server.SandboxSession = lambda **kwargs: Session(**kwargs)
    server.ArtifactSandboxSession = type("ArtifactSession", (), {"__init__": lambda self, **kwargs: None})
    server.mcp = MCP()
    server.main = lambda: None
    server.TextContent = lambda **kwargs: kwargs
    server._get_backend = lambda: "docker"
    docker = types.ModuleType("llm_sandbox.docker")
    docker.SandboxDockerSession = Session
    data = types.ModuleType("llm_sandbox.data")
    data.ConsoleOutput = lambda **kwargs: types.SimpleNamespace(**kwargs)
    modules = {name: types.ModuleType(name) for name in ("llm_sandbox", "llm_sandbox.mcp_server")}
    modules.update({server.__name__: server, docker.__name__: docker, data.__name__: data})
    spec = importlib.util.spec_from_file_location("supervisor_fixture", Path(__file__).with_name("sandbox_mcp_launch.py"))
    module = importlib.util.module_from_spec(spec)
    with patch.dict(sys.modules, modules), patch.dict(os.environ, {
        "AURAGO_SANDBOX_IMAGES": json.dumps({"python": "python@sha256:" + "a" * 64}),
        "IMAGE": "", "AURAGO_SANDBOX_OUTPUT": "1024", "SANDBOX_NETWORK_MODE": "none",
    }):
        spec.loader.exec_module(module)
    return module, server


class SupervisorSecurityTest(unittest.TestCase):
    def test_streaming_exec_inspects_final_status_after_output(self):
        module, _ = load_launcher()
        for exit_code in (0, 7, 137, None, True, -1):
            with self.subTest(exit_code=exit_code):
                events = []
                class ExecAPI:
                    def exec_create(self, container, command, **kwargs):
                        self.options = kwargs
                        return {"Id": "exact-worker-exec"}
                    def exec_start(self, identity, **kwargs):
                        self.stream_options = kwargs
                        if identity != "exact-worker-exec":
                            raise AssertionError("wrong execution identity")
                        # Real Docker stream=True exposes no exit code until
                        # after the iterator is exhausted (high-level gives None).
                        self.high_level_exit_code = None
                        yield b"partial stdout\n", None
                        yield None, b"worker stderr\n"
                        events.append("drained")
                    def exec_inspect(self, identity):
                        if identity != "exact-worker-exec" or events != ["drained"]:
                            raise AssertionError("inspect preceded output drain")
                        events.append("inspected")
                        return {"Running": False, "ExitCode": exit_code}
                api = ExecAPI()
                session = Session()
                session.container = types.SimpleNamespace(id="worker-container", client=types.SimpleNamespace(api=api))
                if type(exit_code) is int and exit_code >= 0:
                    result = session.execute_command("fixture command", workdir="/sandbox")
                    self.assertEqual(result.exit_code, exit_code)
                    self.assertEqual(result.stdout, "partial stdout\n")
                    self.assertEqual(result.stderr, "worker stderr\n")
                    self.assertEqual(events, ["drained", "inspected"])
                    self.assertEqual(api.stream_options, {"stream": True, "demux": True, "tty": False})
                else:
                    with self.assertRaises(RuntimeError):
                        session.execute_command("fixture command")

    def test_streaming_exec_missing_inspection_never_reports_success(self):
        load_launcher()
        for status in ({"Running": True, "ExitCode": 0}, {"ExitCode": 0}, {}):
            api = types.SimpleNamespace(exec_create=lambda *_, **__: {"Id": "exec"},
                exec_start=lambda *_, **__: iter([(b"output", None)]), exec_inspect=lambda _: status)
            session = Session()
            session.container = types.SimpleNamespace(id="worker", client=types.SimpleNamespace(api=api))
            with self.assertRaises(RuntimeError):
                session.execute_command("fixture")

    def test_broker_https_self_signed_ignores_proxy_and_refuses_redirect(self):
        module, _ = load_launcher()
        calls, proxies = [], []
        class Handler(http.server.BaseHTTPRequestHandler):
            def do_POST(self):
                calls.append((self.path, self.headers.get("X-Internal-Token")))
                if self.path.startswith("/redirect/"):
                    self.send_response(307)
                    self.send_header("Location", "http://127.0.0.1:%d/stolen" % proxy.server_port)
                    self.end_headers()
                    return
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b'{"status":"success","result":"local fixture"}')
            def log_message(self, *_):
                pass
        class Proxy(Handler):
            def do_POST(self):
                proxies.append(self.headers.get("X-Internal-Token"))
                self.send_response(500)
                self.end_headers()
            do_CONNECT = do_POST
        openssl = shutil.which("openssl")
        if openssl is None and os.name == "nt":
            candidate = Path(os.environ.get("ProgramFiles", r"C:\Program Files")) / "Git/usr/bin/openssl.exe"
            if candidate.is_file():
                openssl = str(candidate)
        self.assertIsNotNone(openssl, "OpenSSL is required to create the offline TLS fixture")
        with tempfile.TemporaryDirectory(prefix="aurago-broker-tls-") as directory:
            cert, key = Path(directory) / "cert.pem", Path(directory) / "key.pem"
            subprocess.run([openssl, "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1",
                "-subj", "/CN=localhost", "-keyout", str(key), "-out", str(cert)], check=True,
                stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, timeout=15)
            proxy = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Proxy)
            origin = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Handler)
            tls = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
            tls.load_cert_chain(cert, key)
            origin.socket = tls.wrap_socket(origin.socket, server_side=True)
            workers = [threading.Thread(target=server.serve_forever, daemon=True) for server in (proxy, origin)]
            for worker in workers:
                worker.start()
            try:
                proxy_url = "http://127.0.0.1:%d" % proxy.server_port
                with patch.dict(os.environ, {"http_proxy": proxy_url, "https_proxy": proxy_url,
                        "HTTP_PROXY": proxy_url, "HTTPS_PROXY": proxy_url, "no_proxy": "", "NO_PROXY": ""}):
                    for path, expected in (("/bridge", "success"), ("/redirect", "error")):
                        broker = module._Broker("https://127.0.0.1:%d%s" % (origin.server_port, path), "fixture-grant", ["allowed"], 2)
                        session = Session(); session.container = Container()
                        broker.dispatch(session, {"id": 1, "tool": "allowed", "parameters": {}})
                        payload = session.container.responses[0][2]
                        self.assertEqual(payload["status"], expected)
                        self.assertNotIn("fixture-grant", json.dumps(payload))
                self.assertEqual(proxies, [])
                self.assertEqual(calls, [("/bridge/allowed", "fixture-grant"), ("/redirect/allowed", "fixture-grant")])
            finally:
                for server in (proxy, origin):
                    server.shutdown()
                    server.server_close()
                for worker in workers:
                    worker.join(2)

    def test_fixed_spec_has_no_host_mount_or_environment(self):
        module, server = load_launcher()
        spec = server.SandboxSession(lang="python", runtime_configs={"privileged": True, "volumes": ["/:/host"]}).options
        runtime = spec["runtime_configs"]
        self.assertTrue(runtime["read_only"])
        self.assertFalse(runtime["privileged"])
        self.assertEqual(runtime["network_mode"], "none")
        self.assertEqual(runtime["cap_drop"], ["ALL"])
        self.assertEqual(runtime["cap_add"], ["DAC_OVERRIDE"])
        self.assertEqual(runtime["security_opt"], ["no-new-privileges:true"])
        self.assertNotIn("volumes", runtime)
        self.assertNotIn("mounts", runtime)
        self.assertNotIn("AURAGO_MASTER_KEY", runtime["environment"])
        self.assertNotIn("DOCKER_HOST", runtime["environment"])
        self.assertFalse(spec["commit_container"])
        self.assertTrue(spec["skip_environment_setup"])
        self.assertIn("execute_brokered_code", server.mcp.tools)
        compile(module._BROKER_SDK, "worker_sdk", "exec")

    def test_broker_frames_split_across_chunks_and_denied_tool(self):
        module, _ = load_launcher()
        broker = module._Broker("http://127.0.0.1:1234/api/tool-bridge", "scoped-secret-fixture", ["allowed"], 2)
        session = Session(); session.container = Container()
        frame = module._BROKER_PREFIX + json.dumps({"id": 1, "tool": "denied", "parameters": {}}).encode() + b"\n"
        self.assertEqual(broker.stdout(session, b"visible\n" + frame[:8]), b"visible\n")
        self.assertEqual(broker.stdout(session, frame[8:]), b"")
        path, entry, payload = session.container.responses[0]
        self.assertEqual(path, "/sandbox")
        self.assertEqual(entry.name, ".aurago_bridge_00000001.json")
        self.assertEqual(entry.mode, 0o600)
        self.assertEqual(payload["status"], "error")
        self.assertNotIn("scoped-secret-fixture", json.dumps(payload))
        with self.assertRaises(ValueError):
            broker.dispatch(session, {"id": 99, "tool": "allowed", "parameters": {}})

    def test_broker_rejects_origin_and_oversized_frame(self):
        module, _ = load_launcher()
        for origin in ("http://example.com", "http://127.0.0.1@evil.test", "file:///tmp/x"):
            with self.assertRaises(ValueError):
                module._Broker(origin, "secret", ["ok"], 2)
        broker = module._Broker("http://127.0.0.1:1234/api/tool-bridge", "secret", ["ok"], 2)
        with self.assertRaises(ValueError):
            broker.stdout(Session(), module._BROKER_PREFIX + b"x" * 65537)

    def test_output_limit_kills_worker(self):
        module, _ = load_launcher()
        session = Session(); session.container = Container()
        with patch.dict(os.environ, {"AURAGO_SANDBOX_OUTPUT": "8"}):
            with self.assertRaises(RuntimeError):
                module._bounded_output(session, [(b"123456789", None)])
        self.assertTrue(session.container.killed)

    def test_private_cancellation_channel_kills_only_its_worker(self):
        module, server = load_launcher()
        started = threading.Event()
        container = Container()
        class CancelSession:
            def __init__(self):
                self.container = None
            def __enter__(self):
                self.container = container
                return self
            def __exit__(self, *_):
                return False
            def run(self, **_):
                started.set()
                deadline = time.monotonic() + 2
                while not container.killed and time.monotonic() < deadline:
                    time.sleep(0.01)
                return types.SimpleNamespace(to_json=lambda **_: '{"exit_code":1,"stdout":"","stderr":"cancelled"}')
        server.SandboxSession = lambda **_: CancelSession()
        with tempfile.NamedTemporaryFile(prefix="aurago-sandbox-cancel-", delete=False) as channel:
            path = channel.name
        try:
            result = []
            worker = threading.Thread(target=lambda: result.extend(server.mcp.tools["execute_supervised_code"]("print(1)", path)))
            worker.start()
            self.assertTrue(started.wait(1))
            Path(path).write_bytes(b"cancel")
            worker.join(2)
            self.assertFalse(worker.is_alive())
            self.assertTrue(container.killed)
            self.assertEqual(Path(path).read_bytes(), b"terminated")
            self.assertTrue(result)
        finally:
            Path(path).unlink(missing_ok=True)


if __name__ == "__main__":
    unittest.main()
