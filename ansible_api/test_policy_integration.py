import json
import os
from pathlib import Path
import shutil
import sys
import tempfile
import unittest
from unittest.mock import patch

import server


@unittest.skipUnless(os.environ.get("AURAGO_RUN_ANSIBLE_SMOKE") == "1", "opt in to real Ansible acceptance")
class AnsiblePolicyIntegrationTests(unittest.TestCase):
    def setUp(self):
        for binary in ("ansible", "ansible-inventory", "ansible-playbook"):
            self.assertIsNotNone(shutil.which(binary), f"required acceptance binary missing: {binary}")
        self.temp = tempfile.TemporaryDirectory(prefix="aurago-ansible-policy-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.inventory = self.root / "hosts.ini"
        self.inventory.write_text(f"[lab]\nallowed ansible_connection=local ansible_python_interpreter={sys.executable}\n", encoding="utf-8")
        self.inventory.chmod(0o600)
        for key, value in {"PLAYBOOKS_DIR": str(self.root), "DEFAULT_INVENTORY": str(self.inventory), "READ_ONLY": True}.items():
            patcher = patch.object(server, key, value)
            patcher.start()
            self.addCleanup(patcher.stop)

    def run_module(self, pattern="lab", inventory=None, extra_vars=None):
        cmd = ["ansible", pattern, "-i", str(inventory or self.inventory), "-m", "ping"]
        if extra_vars is not None:
            cmd += ["-e", json.dumps(extra_vars)]
        return server.run_ansible(cmd, timeout=60)

    def test_declared_group_executes_but_implicit_localhost_does_not(self):
        result = self.run_module()
        self.assertEqual(result["rc"], 0, result)
        self.assertIn("pong", result["stdout"])
        result = self.run_module("localhost")
        self.assertNotEqual(result["rc"], 0, result)
        self.assertIn("outside the static inventory", result["stderr"])

    def test_literal_module_args_reach_declared_group(self):
        for args in ("data='fixture data'", r'{"data":"fixture\u0020data"}'):
            with self.subTest(args=args):
                result = server.run_ansible(["ansible", "lab", "-i", str(self.inventory), "-m", "ping", "-a", args], timeout=60)
                self.assertEqual(result["rc"], 0, result)
                self.assertIn("fixture data", result["stdout"])

    def test_overrides_fail_with_and_without_readonly(self):
        for readonly in (True, False):
            with patch.object(server, "READ_ONLY", readonly):
                for key, value in {"ansible_host": "127.0.0.1", "ansible_connection": "local", "ansible_python_interpreter": "/fixture/not-a-program"}.items():
                    result = self.run_module(extra_vars={key: value})
                    self.assertNotEqual(result["rc"], 0, result)
                    self.assertIn("cannot override", result["stderr"])

    def test_yaml_escaped_lookup_never_executes(self):
        marker = self.root / "lookup-must-not-run"
        inventory = self.root / "escaped.yml"
        inventory.write_text('all:\n  hosts:\n    allowed:\n      ansible_host: "' + r"\u007b\u007b lookup('pipe', 'touch " + str(marker) + r"') }}" + '"\n', encoding="utf-8")
        inventory.chmod(0o600)
        result = self.run_module("allowed", inventory)
        self.assertNotEqual(result["rc"], 0, result)
        self.assertFalse(marker.exists(), "inventory lookup executed during preflight")

    def test_playbook_preflight_retains_application_vars_and_rejects_outside_hosts(self):
        playbook = self.root / "site.yml"
        playbook.write_text("- hosts: '{{ target }}'\n  gather_facts: false\n  tasks:\n    - ansible.builtin.ping:\n", encoding="utf-8")
        with patch.object(server, "READ_ONLY", False):
            for target, valid in (("lab", True), ("localhost", False)):
                result = server.run_ansible(["ansible-playbook", str(playbook), "-i", str(self.inventory), "-e", json.dumps({"target": target})], timeout=60)
                self.assertEqual(result["rc"] == 0, valid, result)


if __name__ == "__main__":
    unittest.main()
