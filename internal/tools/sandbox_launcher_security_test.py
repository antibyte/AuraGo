"""Protocol fixtures for the pinned supervisor; never starts Docker."""

import importlib.util
import io
import json
import os
from pathlib import Path
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
    modules = {name: types.ModuleType(name) for name in ("llm_sandbox", "llm_sandbox.mcp_server")}
    modules.update({server.__name__: server, docker.__name__: docker})
    spec = importlib.util.spec_from_file_location("supervisor_fixture", Path(__file__).with_name("sandbox_mcp_launch.py"))
    module = importlib.util.module_from_spec(spec)
    with patch.dict(sys.modules, modules), patch.dict(os.environ, {
        "AURAGO_SANDBOX_IMAGES": json.dumps({"python": "python@sha256:" + "a" * 64}),
        "IMAGE": "", "AURAGO_SANDBOX_OUTPUT": "1024", "SANDBOX_NETWORK_MODE": "none",
    }):
        spec.loader.exec_module(module)
    return module, server


class SupervisorSecurityTest(unittest.TestCase):
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
