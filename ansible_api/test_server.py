import io
import os
import sys
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


if __name__ == "__main__":
    unittest.main()
