import base64
import json
import threading
import unittest
import urllib.error
import urllib.request
from http.server import ThreadingHTTPServer

import server

PNG_1X1 = base64.b64encode(
    bytes.fromhex(
        "89504e470d0a1a0a0000000d49484452000000010000000108060000001f15c489"
        "0000000d4944415478da63fcffff3f0300050001a5f645400000000049454e44ae426082"
    )
).decode()
DATA_URL = "data:image/png;base64," + PNG_1X1

SAMPLE = {
    "scene": "A street at dusk.",
    "objects": [
        {"name": "car", "position": "bottom left", "description": {"color": "red"}},
        {"name": "sign", "position": "top"},
    ],
    "text": [{"string": "STOP", "bbox": [450, 50, 550, 150]}, "Main St"],
    "relations": [{"subject": "car", "relation": "near", "object": "sign"}],
}


class FakeEngine:
    backend = "cpu"

    def __init__(self, result=None):
        self.result = SAMPLE if result is None else result
        self.calls = []

    def describe(self, image_bytes, max_objects, max_text_lines, language):
        self.calls.append((len(image_bytes), max_objects, max_text_lines, language))
        return self.result


def start(state):
    httpd = ThreadingHTTPServer(("127.0.0.1", 0), server.make_handler(state))
    thread = threading.Thread(target=httpd.serve_forever, daemon=True)
    thread.start()
    return httpd, "http://127.0.0.1:%d" % httpd.server_address[1]


def post(base, payload):
    req = urllib.request.Request(
        base + "/v1/chat/completions",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return resp.status, json.loads(resp.read())
    except urllib.error.HTTPError as err:
        return err.code, json.loads(err.read())


def image_message(url=DATA_URL):
    return {"messages": [{"role": "user", "content": [{"type": "image_url", "image_url": {"url": url}}]}]}


class FormatDescriptionTest(unittest.TestCase):
    def test_formats_all_sections(self):
        text = server.format_description(SAMPLE)
        self.assertIn("Scene: A street at dusk.", text)
        self.assertIn("car (red) at bottom left", text)
        self.assertIn("sign at top", text)
        self.assertIn('Text in image: "STOP" | "Main St"', text)
        self.assertIn("Relations: car near sign.", text)

    def test_respects_limits(self):
        many = {"objects": [{"name": "o%d" % i} for i in range(50)], "text": ["t%d" % i for i in range(50)]}
        text = server.format_description(many, max_objects=3, max_text_lines=2)
        self.assertIn("o0; o1; o2.", text)
        self.assertNotIn("o3", text)
        self.assertIn('"t0" | "t1"', text)
        self.assertNotIn("t2", text)

    def test_tolerates_garbage_and_empty(self):
        self.assertEqual(server.format_description({}), "No content could be described.")
        self.assertEqual(server.format_description({"objects": "x", "text": 3, "relations": None}), "No content could be described.")
        self.assertEqual(server.format_description("plain"), "plain")


class ServerTest(unittest.TestCase):
    def setUp(self):
        self.state = server.State(max_objects=20, max_text_lines=30, language="", gpu_backend="none")
        self.engine = FakeEngine()
        self.state.engine = self.engine
        self.httpd, self.base = start(self.state)
        self.addCleanup(self.httpd.shutdown)

    def test_health_reports_ready_and_backend(self):
        with urllib.request.urlopen(self.base + "/health", timeout=5) as resp:
            body = json.loads(resp.read())
        self.assertTrue(body["ready"])
        self.assertEqual(body["backend"], "cpu")
        self.assertEqual(body["gpu_requested"], "none")

    def test_chat_completion_returns_text(self):
        status, body = post(self.base, image_message())
        self.assertEqual(status, 200)
        content = body["choices"][0]["message"]["content"]
        self.assertIn("Scene: A street at dusk.", content)
        self.assertEqual(self.engine.calls[0][1:], (20, 30, ""))

    def test_missing_image_is_400(self):
        status, body = post(self.base, {"messages": [{"role": "user", "content": "hi"}]})
        self.assertEqual(status, 400)
        self.assertIn("image_url", body["error"]["message"])

    def test_non_data_url_is_400(self):
        status, body = post(self.base, image_message("https://example.com/a.png"))
        self.assertEqual(status, 400)
        self.assertIn("data URL", body["error"]["message"])

    def test_engine_not_loaded_is_503(self):
        self.state.engine = None
        status, body = post(self.base, image_message())
        self.assertEqual(status, 503)
        self.assertIn("loading", body["error"]["message"])

    def test_engine_failure_is_500_without_details(self):
        class Boom(FakeEngine):
            def describe(self, *args, **kwargs):
                raise RuntimeError("secret path /root/x")

        self.state.engine = Boom()
        status, body = post(self.base, image_message())
        self.assertEqual(status, 500)
        self.assertNotIn("secret", json.dumps(body))


if __name__ == "__main__":
    unittest.main()
