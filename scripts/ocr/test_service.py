"""Private protocol regressions without model or container dependencies."""
import http.client
import importlib.util
from pathlib import Path
import threading
import time
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location("adapter", Path(__file__).resolve().parents[2] / "internal/ocr/runtime/service.py")
adapter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(adapter)


class AdapterTests(unittest.TestCase):
    def setUp(self):
        adapter.TOKEN = "synthetic-test-token"
        adapter.workers.clear()
        adapter.cancelled.clear()
        self.server = adapter.ThreadingHTTPServer(("127.0.0.1", 0), adapter.Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def tearDown(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def request(self, method, path, body=None, authorized=True):
        client = http.client.HTTPConnection(*self.server.server_address, timeout=5)
        headers = {"Content-Type": "application/pdf"}
        if authorized:
            headers["Authorization"] = "Bearer " + adapter.TOKEN
        client.request(method, path, body=body, headers=headers)
        response = client.getresponse()
        status = response.status
        response.read()
        client.close()
        return status

    def test_early_cancel_prevents_a_late_worker(self):
        key = "a" * 32
        self.assertEqual(self.request("DELETE", "/v1/ocr/" + key), 200)
        with patch.object(adapter.subprocess, "Popen") as spawn:
            self.assertEqual(self.request("POST", "/v1/ocr?id=" + key, b"%PDF-fixture"), 409)
            spawn.assert_not_called()

    def test_cancel_acknowledges_slot_release(self):
        key, done = "b" * 32, threading.Event()
        adapter.slot.acquire()
        adapter.workers[key] = {"process": None, "cancelled": False, "done": done}
        def release():
            with adapter.lock:
                adapter.workers.pop(key, None)
            adapter.slot.release()
            done.set()
        timer = threading.Timer(.05, release)
        timer.start()
        started = time.monotonic()
        self.assertEqual(self.request("DELETE", "/v1/ocr/" + key), 200)
        self.assertTrue(done.is_set())
        self.assertGreaterEqual(time.monotonic() - started, .03)
        timer.join()

    def test_auth_and_length_are_checked_before_admission(self):
        self.assertEqual(self.request("GET", "/healthz", authorized=False), 401)
        self.assertEqual(self.request("POST", "/v1/ocr?id=" + "c" * 32, b""), 400)
        self.assertEqual(self.request("DELETE", "/v1/ocr/invalid"), 400)


if __name__ == "__main__":
    unittest.main()
