import io
import os
import sys
import json
import tempfile
from pathlib import Path
from unittest.mock import patch
import unittest

sys.path.insert(0, os.path.dirname(__file__))

import server


class FakeHandler:
    def __init__(self, body=b"", headers=None, path="/run/ping"):
        self.headers = headers or {}
        self.path = path
        self.rfile = io.BytesIO(body)
        self.sent = []
        self.ping_called = False

    def send_json(self, code, data):
        self.sent.append((code, data))

    def require_auth(self):
        return True

    def read_json_body(self):
        return server.Handler.read_json_body(self)

    def _handle_ping(self, body):
        self.ping_called = True
        self.send_json(200, {"status": "ok", "body": body})

    def _handle_adhoc(self, body):
        raise AssertionError("unexpected adhoc handler call")

    def _handle_playbook(self, body):
        raise AssertionError("unexpected playbook handler call")

    def _handle_facts(self, body):
        raise AssertionError("unexpected facts handler call")


class AnsibleAPIServerTests(unittest.TestCase):
    def setUp(self):
        self.orig_token = server.TOKEN
        self.orig_allow_unauth = getattr(server, "ALLOW_UNAUTH", None)
        self.orig_max_body_size = server.MAX_BODY_SIZE

    def tearDown(self):
        server.TOKEN = self.orig_token
        if self.orig_allow_unauth is None and hasattr(server, "ALLOW_UNAUTH"):
            delattr(server, "ALLOW_UNAUTH")
        else:
            server.ALLOW_UNAUTH = self.orig_allow_unauth
        server.MAX_BODY_SIZE = self.orig_max_body_size

    def test_empty_token_requires_explicit_unauth_opt_in(self):
        handler = FakeHandler()
        server.TOKEN = ""
        server.ALLOW_UNAUTH = False
        self.assertFalse(server.check_auth(handler))

        server.ALLOW_UNAUTH = True
        self.assertTrue(server.check_auth(handler))

    def test_invalid_json_returns_400_and_does_not_run_default_handler(self):
        handler = FakeHandler(body=b"{", headers={"Content-Length": "1"})

        server.Handler.do_POST(handler)

        self.assertFalse(handler.ping_called)
        self.assertEqual(handler.sent[0][0], 400)

    def test_oversized_json_returns_413_and_does_not_run_default_handler(self):
        server.MAX_BODY_SIZE = 2
        handler = FakeHandler(body=b"{}", headers={"Content-Length": "3"})

        server.Handler.do_POST(handler)

        self.assertFalse(handler.ping_called)
        self.assertEqual(handler.sent[0][0], 413)

    def test_empty_body_still_returns_empty_object(self):
        handler = FakeHandler(headers={"Content-Length": "0"})

        body = server.Handler.read_json_body(handler)

        self.assertEqual(body, {})
        self.assertEqual(handler.sent, [])


class InventorySecurityTests(unittest.TestCase):
    def test_unconfigured_default_does_not_authorize_outside_inventory(self):
        with patch.object(server, "DEFAULT_INVENTORY", ""):
            with self.assertRaises(ValueError):
                server.read_static_inventory(str(self.default))

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.playbooks = self.root / "playbooks"
        self.playbooks.mkdir()
        self.default = self.root / "hosts.ini"
        self.default.write_text("[all]\nserver ansible_host=192.168.1.10\n", encoding="utf-8")
        for key, value in {"PLAYBOOKS_DIR": str(self.playbooks), "DEFAULT_INVENTORY": str(self.default)}.items():
            patcher = patch.object(server, key, value)
            patcher.start()
            self.addCleanup(patcher.stop)

    def test_only_exact_default_can_escape_playbooks(self):
        self.assertIn(b"server", server.read_static_inventory(str(self.default))[0])
        for path in ["../hosts.ini", str(self.root / "secret.ini"), "/etc/passwd"]:
            with self.subTest(path=path), self.assertRaises((ValueError, OSError)):
                server.read_static_inventory(path)

    def test_dynamic_executable_oversized_and_symlink_inventories_are_rejected(self):
        for name, body in [("dynamic.yml", b"plugin: aws_ec2"), ("quoted.yml", b'"plugin": script'), ("script.ini", b"#!/bin/sh\necho secret"), ("large.ini", b"a" * (1024 * 1024 + 1))]:
            (self.playbooks / name).write_bytes(body)
            with self.subTest(name=name), self.assertRaises(ValueError):
                server.read_static_inventory(name)
        linked = self.playbooks / "link.ini"
        try:
            linked.symlink_to(self.default)
        except OSError:
            return  # Windows without symlink privilege; Linux CI exercises this case.
        with self.assertRaises((ValueError, OSError)):
            server.read_static_inventory(str(linked))

    def test_inventory_projection_excludes_secrets_and_raw_failures(self):
        data = {"all": {"hosts": ["host"], "vars": {"password": "secret"}}, "_meta": {"hostvars": {"host": {"ansible_host": "192.168.1.10", "ansible_password": "secret", "custom": "secret"}}}}
        result = server.inventory_summary(data)
        self.assertNotIn("secret", json.dumps(result))
        self.assertEqual(result["all"]["hosts"], ["host"])
        handler = FakeHandler()
        with patch.object(server, "run_ansible", return_value={"rc": 1, "stdout": "secret", "stderr": "secret"}):
            server.Handler._handle_inventory(handler, str(self.default))
        self.assertNotIn("secret", json.dumps(handler.sent))

    def test_child_environment_and_inventory_snapshot(self):
        def execute(cmd, **kwargs):
            self.assertNotIn("ANSIBLE_API_TOKEN", kwargs["env"])
            self.assertNotIn("OTHER_API_KEY", kwargs["env"])
            self.assertEqual(kwargs["env"]["ANSIBLE_INVENTORY_ENABLED"], "ini,yaml")
            snapshot = Path(cmd[cmd.index("-i") + 1])
            self.assertNotEqual(snapshot, self.default)
            self.assertEqual(snapshot.read_bytes(), self.default.read_bytes())
            return subprocess_result
        from types import SimpleNamespace
        subprocess_result = SimpleNamespace(returncode=0, stdout="ok", stderr="")
        with patch.dict(os.environ, {"ANSIBLE_API_TOKEN": "secret", "OTHER_API_KEY": "secret"}), patch.object(server.subprocess, "run", side_effect=execute):
            self.assertEqual(server.run_ansible(["ansible-inventory", "-i", str(self.default), "--list"])["rc"], 0)

    def test_readonly_denies_check_before_spawning(self):
        with patch.object(server, "READ_ONLY", True), patch.object(server.subprocess, "run") as run:
            self.assertNotEqual(server.run_ansible(["ansible-playbook", "site.yml", "--check"])["rc"], 0)
            run.assert_not_called()


if __name__ == "__main__":
    unittest.main()
