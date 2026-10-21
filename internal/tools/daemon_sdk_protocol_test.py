"""Offline fixtures for the daemon's single stdin owner; no host tools or network."""
import importlib.util
import json
import os
from pathlib import Path
import queue
import sys
import threading
import unittest
from unittest.mock import patch


class Input:
    def __init__(self):
        self.lines = queue.Queue()
    def readline(self, limit):
        return self.lines.get(timeout=3)
    def send(self, message):
        self.lines.put(json.dumps(message) + "\n")


class Output:
    def __init__(self):
        self.lines = queue.Queue()
    def write(self, value):
        self.lines.put(json.loads(value))
    def flush(self):
        pass


def load_sdk(name):
    path = Path(__file__).resolve().parents[2] / "agent_workspace" / "skills" / (name + ".py")
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class DaemonSDKProtocolTest(unittest.TestCase):
    def test_local_validation_errors_do_not_consume_request_identity(self):
        daemon_sdk = load_sdk("aurago_daemon")
        incoming, outgoing = Input(), Output()
        with patch.object(sys, "stdin", incoming), patch.object(sys, "stdout", outgoing):
            with self.assertRaises(ValueError):
                daemon_sdk._call_tool("docker", {}, "not-a-timeout")
            with self.assertRaises(TypeError):
                daemon_sdk._call_tool("docker", {"nested": {"value": object()}}, 1)
            self.assertTrue(outgoing.lines.empty())
            results = []
            thread = threading.Thread(target=lambda: results.append(daemon_sdk._call_tool("docker", {}, 1)))
            thread.start()
            request = outgoing.lines.get(timeout=1)
            self.assertEqual(request["data"]["id"], 1)
            incoming.send({"type": "tool_result", "data": {"id": 1, "status": "success", "result": "ok"}})
            thread.join(1)
            self.assertFalse(thread.is_alive())
            self.assertEqual(results, [{"id": 1, "status": "success", "result": "ok"}])
            self.assertTrue(outgoing.lines.empty())
            incoming.lines.put("")

    def test_shared_reader_preserves_results_commands_and_heartbeat(self):
        daemon_sdk, tools_sdk = load_sdk("aurago_daemon"), load_sdk("aurago_tools")
        incoming, outgoing = Input(), Output()
        with patch.dict(sys.modules, {"aurago_daemon": daemon_sdk}), patch.dict(os.environ, {
                "AURAGO_DAEMON": "1", "AURAGO_DAEMON_TOOL_BRIDGE": "stdio-v1",
                "AURAGO_TOOL_BRIDGE_TOKEN": "", "AURAGO_TOOL_BRIDGE_URL": ""}), \
                patch.object(sys, "stdin", incoming), patch.object(sys, "stdout", outgoing):
            daemon = daemon_sdk.AuraGoDaemon()
            client = tools_sdk.AuraGoTools()
            self.assertTrue(client.is_available())
            results = []
            thread = threading.Thread(target=lambda: results.append(client.call("docker", {"operation": "list"})))
            thread.start()
            request = outgoing.lines.get(timeout=1)
            self.assertEqual(request["type"], "tool_request")
            self.assertEqual(request["data"]["id"], 1)
            incoming.send({"type": "config_update", "env": {"INTERVAL": "2"}})
            daemon.heartbeat()
            self.assertEqual(outgoing.lines.get(timeout=1), {"type": "heartbeat"})
            incoming.send({"type": "tool_result", "data": {"id": 999, "status": "success", "result": "wrong request"}})
            incoming.send({"type": "tool_result", "data": {"id": 1, "status": "success", "result": '{"ok":true}'}})
            thread.join(1)
            self.assertFalse(thread.is_alive())
            self.assertEqual(results, [{"ok": True}])
            self.assertEqual(daemon.next_command(), {"type": "config_update", "env": {"INTERVAL": "2"}})
            errors = []
            def pending():
                try:
                    client.call_raw("docker", {"operation": "list"})
                except tools_sdk.AuraGoToolError as error:
                    errors.append(str(error))
            waiting = threading.Thread(target=pending)
            waiting.start()
            self.assertEqual(outgoing.lines.get(timeout=1)["data"]["id"], 2)
            incoming.send({"type": "stop", "reason": "fixture"})
            waiting.join(1)
            self.assertFalse(waiting.is_alive())
            self.assertFalse(daemon.is_running())
            self.assertTrue(errors)
            self.assertIn("stopped", errors[0])
            self.assertEqual(daemon.next_command()["type"], "stop")
            incoming.lines.put("")

    def test_missing_reply_and_oversized_request_never_retry(self):
        daemon_sdk, tools_sdk = load_sdk("aurago_daemon"), load_sdk("aurago_tools")
        incoming, outgoing = Input(), Output()
        with patch.dict(sys.modules, {"aurago_daemon": daemon_sdk}), patch.dict(os.environ, {
                "AURAGO_DAEMON_TOOL_BRIDGE": "stdio-v1"}), \
                patch.object(sys, "stdin", incoming), patch.object(sys, "stdout", outgoing):
            client = tools_sdk.AuraGoTools()
            with self.assertRaises(tools_sdk.AuraGoToolError):
                client.call("docker", {"body": "x" * 65536})
            self.assertTrue(outgoing.lines.empty())
            errors = []
            def call():
                try:
                    client.call_raw("docker", {})
                except tools_sdk.AuraGoToolError as error:
                    errors.append(str(error))
            thread = threading.Thread(target=call)
            thread.start()
            self.assertEqual(outgoing.lines.get(timeout=1)["data"]["id"], 1)
            incoming.lines.put("")
            thread.join(1)
            self.assertTrue(errors)
            self.assertTrue(outgoing.lines.empty())


if __name__ == "__main__":
    unittest.main()
