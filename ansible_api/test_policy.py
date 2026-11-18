import json
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import server
import test_server


class LiteralPolicyTests(unittest.TestCase):
    def test_shared_policy_cases(self):
        cases = json.loads(Path(__file__).with_name("policy_cases.json").read_text(encoding="utf-8"))
        for case in cases["extra_vars"]:
            with self.subTest(case=case["name"]):
                try:
                    server.extra_vars_arg(json.loads(case["raw"]))
                    accepted = True
                except ValueError:
                    accepted = False
                self.assertEqual(accepted, case["valid"])
        for case in cases["selected_hosts"]:
            with self.subTest(case=case["name"]):
                try:
                    server.validate_selected_hosts(case["stdout"], {"server", "worker"})
                    accepted = True
                except ValueError:
                    accepted = False
                self.assertEqual(accepted, case["valid"])


class ExecutionTargetTests(unittest.TestCase):
    setUp = test_server.InventorySecurityTests.setUp
    def simulate(self, cmd, selected="server", normalized=None):
        calls = []
        inventory = normalized or {"all": {"hosts": ["server"]}, "_meta": {"hostvars": {"server": {"ansible_host": "192.168.1.10"}}}}

        def execute(argv, **kwargs):
            calls.append(argv)
            self.assertNotIn("ANSIBLE_API_TOKEN", kwargs["env"])
            if argv[0] == "ansible-inventory":
                output = json.dumps(inventory)
            elif "--list-hosts" in argv:
                output = "  hosts (1):\n    " + selected + "\n"
            else:
                output = "fixture result"
            return SimpleNamespace(returncode=0, stdout=output, stderr="")

        with patch.object(server, "READ_ONLY", True), patch.object(server.subprocess, "run", side_effect=execute):
            result = server.run_ansible(cmd)
        return result, calls

    def test_explicit_host_runs_only_after_validation(self):
        result, calls = self.simulate(["ansible", "server", "-i", str(self.default), "-m", "setup", "-e", '{"release":"1.2"}'])
        self.assertEqual(result["rc"], 0)
        self.assertEqual(len(calls), 3)
        self.assertIn("--export", calls[0])
        self.assertIn("--list-hosts", calls[1])
        self.assertNotIn("--list-hosts", calls[2])

    def test_readonly_setup_cannot_select_implicit_localhost(self):
        result, calls = self.simulate(["ansible", "localhost", "-i", str(self.default), "-m", "setup"], selected="localhost")
        self.assertNotEqual(result["rc"], 0)
        self.assertEqual(len(calls), 2)

    def test_connection_override_is_rejected_before_any_child(self):
        for key in ["ansible_host", "ansible_connection", "ansible_python_interpreter"]:
            result, calls = self.simulate(["ansible", "server", "-i", str(self.default), "-m", "setup", "-e", json.dumps({key: "fixture"})])
            self.assertNotEqual(result["rc"], 0)
            self.assertEqual(calls, [])

    def test_readonly_module_arguments_are_literal_before_any_child(self):
        cases = json.loads(Path(__file__).with_name("policy_cases.json").read_text(encoding="utf-8"))
        for case in cases["module_args"]:
            with self.subTest(case=case["name"]):
                result, calls = self.simulate(["ansible", "server", "-i", str(self.default), "-m", "ping", "-a", case["raw"]])
                self.assertEqual(result["rc"] == 0, case["valid"])
                self.assertEqual(len(calls), 3 if case["valid"] else 0)

    def test_raw_and_decoded_inventory_templates_are_rejected(self):
        inventory = self.playbooks / "template.ini"
        inventory.write_text("server ansible_host=\"{{ lookup('env', 'FIXTURE') }}\"\n", encoding="utf-8")
        result, calls = self.simulate(["ansible", "server", "-i", str(inventory), "-m", "setup"])
        self.assertNotEqual(result["rc"], 0)
        self.assertEqual(calls, [])
        result, calls = self.simulate(["ansible", "server", "-i", str(self.default), "-m", "setup"], normalized={"all": {"hosts": ["server"]}, "_meta": {"hostvars": {"server": {"ansible_host": "{{ lookup('env', 'FIXTURE') }}"}}}})
        self.assertNotEqual(result["rc"], 0)
        self.assertEqual(len(calls), 1)

    def test_limits_cannot_read_host_files(self):
        result, calls = self.simulate(["ansible", "server", "-i", str(self.default), "-m", "setup", "--limit", "@/fixture/private"])
        self.assertNotEqual(result["rc"], 0)
        self.assertEqual(calls, [])

    def test_empty_default_keeps_explicit_private_localhost(self):
        with patch.object(server, "DEFAULT_INVENTORY", ""):
            result, calls = self.simulate(["ansible", "localhost", "-i", "", "-m", "setup"], selected="localhost", normalized={"all": {"hosts": ["localhost"]}, "_meta": {"hostvars": {}}})
        self.assertEqual(result["rc"], 0)
        self.assertEqual(len(calls), 3)


if __name__ == "__main__":
    unittest.main()
