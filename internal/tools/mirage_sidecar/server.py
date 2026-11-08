#!/usr/bin/env python3
"""AuraGo Mirage vision sidecar.

Speaks a tiny subset of the OpenAI chat-completions API: the image of the last
user message that carries one is described by the Cactus Mirage model and the
description comes back as plain text. Standard library only; the Cactus engine
is imported lazily so the unit tests run without it.
"""
import base64
import binascii
import json
import os
import re
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

MAX_BODY_BYTES = 40 * 1024 * 1024
_DATA_URL = re.compile(r"^data:image/[A-Za-z0-9.+-]+;base64,(.+)$", re.DOTALL)


class RequestError(Exception):
    def __init__(self, status, message):
        super().__init__(message)
        self.status = status
        self.message = message


def decode_data_url(url):
    match = _DATA_URL.match(url or "")
    if not match:
        raise RequestError(400, "image must be a base64 data URL")
    try:
        data = base64.b64decode(match.group(1))
    except (binascii.Error, ValueError):
        raise RequestError(400, "image data is not valid base64")
    if not data:
        raise RequestError(400, "image data is empty")
    return data


def extract_image(payload):
    messages = payload.get("messages") if isinstance(payload, dict) else None
    if not isinstance(messages, list):
        raise RequestError(400, "messages must be a list")
    for message in reversed(messages):
        content = message.get("content") if isinstance(message, dict) else None
        if not isinstance(content, list):
            continue
        for part in content:
            if not isinstance(part, dict) or part.get("type") != "image_url":
                continue
            ref = part.get("image_url")
            url = ref.get("url") if isinstance(ref, dict) else ref
            return decode_data_url(url if isinstance(url, str) else "")
    raise RequestError(400, "no image_url part found in messages")


def _first(mapping, *keys):
    if not isinstance(mapping, dict):
        return None
    for key in keys:
        value = mapping.get(key)
        if value not in (None, "", [], {}):
            return value
    return None


def _as_list(value):
    return value if isinstance(value, list) else []


def _position(box):
    try:
        x0, y0, x1, y1 = [float(v) for v in list(box)[:4]]
    except (TypeError, ValueError):
        return ""
    cx, cy = (x0 + x1) / 2, (y0 + y1) / 2
    horizontal = "left" if cx < 333 else "right" if cx > 666 else "center"
    vertical = "top" if cy < 333 else "bottom" if cy > 666 else "middle"
    if horizontal == "center" and vertical == "middle":
        return "center"
    if horizontal == "center":
        return vertical
    if vertical == "middle":
        return horizontal
    return vertical + " " + horizontal


def format_description(result, max_objects=20, max_text_lines=30):
    """Turn Mirage's structured result into the text the main model reads."""
    if not isinstance(result, dict):
        return str(result).strip() or "No content could be described."
    lines = []

    scene = result.get("scene")
    if isinstance(scene, dict):
        scene = _first(scene, "description", "caption", "text", "summary")
    if isinstance(scene, str) and scene.strip():
        lines.append("Scene: " + scene.strip())

    entries = []
    for obj in _as_list(result.get("objects"))[:max_objects]:
        if isinstance(obj, str):
            entries.append(obj)
            continue
        name = _first(obj, "name", "label", "class")
        if not name:
            continue
        text = str(name)
        attrs = _first(obj, "attributes", "attrs")
        if isinstance(attrs, (list, tuple)):
            text += " (" + ", ".join(str(a) for a in attrs) + ")"
        elif isinstance(attrs, str):
            text += " (" + attrs + ")"
        place = _position(_first(obj, "bbox", "box") or [])
        if place:
            text += " at " + place
        entries.append(text)
    if entries:
        lines.append("Objects: " + "; ".join(entries) + ".")

    texts = []
    for item in _as_list(result.get("text"))[:max_text_lines]:
        value = item if isinstance(item, str) else _first(item, "text", "content", "value")
        if value and str(value).strip():
            texts.append('"' + str(value).strip() + '"')
    if texts:
        lines.append("Text in image: " + " | ".join(texts))

    relations = []
    for rel in _as_list(result.get("relations"))[:max_objects]:
        if isinstance(rel, str):
            relations.append(rel)
            continue
        subject = _first(rel, "subject", "source")
        verb = _first(rel, "relation", "predicate", "verb")
        target = _first(rel, "object", "target")
        if subject and verb and target:
            relations.append("%s %s %s" % (subject, verb, target))
    if relations:
        lines.append("Relations: " + "; ".join(relations) + ".")

    return "\n".join(lines) if lines else "No content could be described."


class CactusEngine:
    """Thin wrapper around cactus.Mirage()."""

    def __init__(self):
        import cactus  # heavy and absent in the unit tests

        self._model = cactus.Mirage()
        self.backend = str(getattr(self._model, "backend", "") or "cpu")

    def describe(self, image_bytes, max_objects, max_text_lines, language):
        options = {"max_objects": max_objects, "max_text_lines": max_text_lines}
        if language:
            options["language"] = language
        return self._model.describe(image_bytes, **options)


class State:
    def __init__(self, max_objects, max_text_lines, language, gpu_backend):
        self.max_objects = max_objects
        self.max_text_lines = max_text_lines
        self.language = language
        self.gpu_backend = gpu_backend
        self.engine = None
        self.error = ""
        self._lock = threading.Lock()

    def ready(self):
        return self.engine is not None

    def describe(self, image_bytes):
        engine = self.engine
        if engine is None:
            raise RequestError(503, self.error or "model is still loading")
        with self._lock:
            result = engine.describe(image_bytes, self.max_objects, self.max_text_lines, self.language)
        return format_description(result, self.max_objects, self.max_text_lines)


def _completion(text):
    return {
        "id": "mirage",
        "object": "chat.completion",
        "model": "mirage",
        "choices": [{"index": 0, "message": {"role": "assistant", "content": text}, "finish_reason": "stop"}],
        "usage": {"prompt_tokens": 0, "completion_tokens": 0, "total_tokens": 0},
    }


def make_handler(state):
    class Handler(BaseHTTPRequestHandler):
        def log_message(self, fmt, *args):
            sys.stderr.write("[mirage] " + (fmt % args) + "\n")

        def _send(self, status, body):
            data = json.dumps(body).encode()
            self.send_response(status)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

        def do_GET(self):
            if self.path != "/health":
                self._send(404, {"error": {"message": "not found"}})
                return
            engine = state.engine
            self._send(200, {
                "ready": engine is not None,
                "backend": getattr(engine, "backend", "") if engine else "",
                "gpu_requested": state.gpu_backend,
                "error": state.error,
            })

        def do_POST(self):
            if self.path != "/v1/chat/completions":
                self._send(404, {"error": {"message": "not found"}})
                return
            try:
                length = int(self.headers.get("Content-Length") or 0)
                if length <= 0:
                    raise RequestError(400, "empty request body")
                if length > MAX_BODY_BYTES:
                    raise RequestError(413, "request body too large")
                payload = json.loads(self.rfile.read(length))
                text = state.describe(extract_image(payload))
            except RequestError as err:
                self._send(err.status, {"error": {"message": err.message}})
                return
            except (ValueError, UnicodeDecodeError):
                self._send(400, {"error": {"message": "request body is not valid JSON"}})
                return
            except Exception as err:  # engine failure: log here, keep details off the wire
                sys.stderr.write("[mirage] describe failed: %r\n" % (err,))
                self._send(500, {"error": {"message": "image analysis failed"}})
                return
            self._send(200, _completion(text))

    return Handler


def load_engine_in_background(state, factory=CactusEngine):
    def run():
        try:
            state.engine = factory()
            sys.stderr.write("[mirage] model ready, backend=%s\n" % state.engine.backend)
        except Exception as err:
            state.error = "model failed to load: %s" % err
            sys.stderr.write("[mirage] %s\n" % state.error)

    threading.Thread(target=run, daemon=True).start()


def main():
    state = State(
        max_objects=int(os.environ.get("MIRAGE_MAX_OBJECTS", "20")),
        max_text_lines=int(os.environ.get("MIRAGE_MAX_TEXT_LINES", "30")),
        language=os.environ.get("MIRAGE_LANGUAGE", ""),
        gpu_backend=os.environ.get("MIRAGE_GPU_BACKEND", "none"),
    )
    load_engine_in_background(state)
    port = int(os.environ.get("MIRAGE_PORT", "8080"))
    ThreadingHTTPServer(("0.0.0.0", port), make_handler(state)).serve_forever()


if __name__ == "__main__":
    main()
