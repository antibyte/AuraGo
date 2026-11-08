# Mirage Vision Sidecar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** AuraGo gets an optional local vision source, the Cactus Mirage model in a managed Docker sidecar, that is used automatically when no vision provider is configured and the main model cannot read images. It uses a detected GPU where the engine can.

**Architecture:** A self-built Python sidecar (`aurago_mirage`, stdlib HTTP server + `cactus-compute`) exposes `POST /v1/chat/completions` and `GET /health`. It turns the image into a plain-text description. Go side: new config block `vision.mirage`, a routing hook at the top of `analyzeImageReferenceWithPromptContext` (`internal/tools/vision.go`), a sidecar manager modelled on Piper plus the Ollama GPU code, a status endpoint and a status banner in the Vision config section.

**Tech Stack:** Go (`internal/config`, `internal/tools`, `internal/server`), Python 3 stdlib (`http.server`, `unittest`), Docker Engine API via `dockerRequest`, vanilla JS config UI, 16-locale i18n JSON.

**Spec:** `docs/superpowers/specs/2026-11-08-mirage-vision-design.md`

## Deviations from the spec (decided while mapping the code)

1. **No provider entry.** Mirage is selected by `vision.mirage.enabled` while `vision.provider` is empty (an explicit provider always wins). `vision.mirage.mode` is `fallback` (default: only when the main model cannot take images) or `always`. This avoids a fake provider ID that `provider_references.go` would flag as dangling.
2. **Text formatting lives in the sidecar (Python)**, not in Go. It is tested with `unittest`. The Go side only forwards the text.
3. **`vision.mirage.image` is the Python base image** of the sidecar build (default `python:3.12-slim`), not a prebuilt Mirage image. The built tag is content-hashed like go2rtc.
4. **`vision.mirage.language`** (empty = engine default) replaces "follows the AuraGo language". No reliable language field was found in the config map; a later change can feed it.
5. **Main-key safety.** The Mirage branch never reads `cfg.LLM.APIKey` (`vision.go` otherwise inherits it). The Telegram "default Gemini model" injection (`internal/telegram/vision.go:22-24`) is harmless because the Mirage branch runs before any model resolution.

## Ground rules for every task

- Other sessions leave many modified files in this checkout. **Never `git add -A`.** Stage and commit only the paths named in the task: `git add <paths>` then `git commit -m "..." -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- <paths>`.
- Go tests: `go test <pkg> -run '<Name>' -count=1`. For several names group them as `(?i)(A|B)` (top-level `|` is split before regex flags) and count the PASS lines.
- Write Go files containing the two-character sequence backslash-u via `\x5c` (the Write/Edit tools decode it). None of the code below needs it.
- Project rule (CLAUDE.md): before editing an existing function run GitNexus `impact` upstream on it and report callers/risk; run `detect_changes` before the last commit. The index may be stale (`npx gitnexus analyze`), treat `UNKNOWN` as unresolved.

## File structure

| File | Responsibility |
| --- | --- |
| `internal/tools/mirage_sidecar/server.py` (create) | Sidecar HTTP server: parse OpenAI request, call engine, format text |
| `internal/tools/mirage_sidecar/test_server.py` (create) | `unittest` for the server with a fake engine |
| `internal/tools/mirage_sidecar/Dockerfile` (create) | Sidecar image |
| `internal/tools/mirage_sidecar/CACTUS_VERSION` (create) | Pinned `cactus-compute` version, embedded |
| `internal/config/config_types.go` (modify ~1491-1500) | `Vision.Mirage` struct |
| `internal/config/config.go` (modify ~840) | Defaults |
| `internal/config/vision_mirage.go` (create) | Mode normalisation, `MirageBaseURL` |
| `config_template.yaml` (modify ~1056) | Documented keys |
| `internal/tools/vision_mirage.go` (create) | `UseMirageVision`, `analyzeWithMirage` |
| `internal/tools/vision.go` (modify :155) | Routing hook |
| `internal/tools/mirage_sidecar.go` (create) | Embed, image build, GPU, `EnsureMirageRunning`, `MirageStatus` |
| `internal/tools/managed_sidecar_names.go` (modify :34) | Reserve container name |
| `internal/dockerutil/managed_resources.go` (modify ~15) | `MirageOwner` |
| `internal/server/containers_protection.go` (modify :38-52) | Protect owner |
| `internal/server/mirage_handlers.go` (create) | `/api/mirage/status`, `mirageInputsChanged` |
| `internal/server/server.go` (modify ~1790), `config_handlers_main.go` (modify ~1499), `server_routes_config.go` (modify ~318) | Wiring |
| `ui/js/config/main.js` (modify ~2138, ~2180) | Status banner for the Vision section |
| `ui/lang/config/generic/<16 locales>.json` (modify) | 5 banner keys |
| `documentation/configuration.md`, `documentation/manuals/{en,de}/07-configuration.md` (modify) | Docs |

---

### Task 1: Sidecar server (Python) with unit tests

**Files:**
- Create: `internal/tools/mirage_sidecar/server.py`
- Create: `internal/tools/mirage_sidecar/test_server.py`

- [ ] **Step 1: Write the failing tests**

Create `internal/tools/mirage_sidecar/test_server.py`:

```python
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
        {"name": "car", "bbox": [100, 600, 400, 900], "attributes": ["red"]},
        {"name": "sign", "bbox": [450, 50, 550, 150]},
    ],
    "text": [{"text": "STOP", "bbox": [450, 50, 550, 150]}, "Main St"],
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `python -m unittest discover -s internal/tools/mirage_sidecar -p "test_*.py" -v`
Expected: FAIL / ERROR with `ModuleNotFoundError: No module named 'server'`.

- [ ] **Step 3: Write the server**

Create `internal/tools/mirage_sidecar/server.py`:

```python
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `python -m unittest discover -s internal/tools/mirage_sidecar -p "test_*.py" -v`
Expected: 9 tests, all `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/tools/mirage_sidecar/server.py internal/tools/mirage_sidecar/test_server.py
git commit -m "feat(mirage): sidecar HTTP server with text formatting" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- internal/tools/mirage_sidecar/server.py internal/tools/mirage_sidecar/test_server.py
```

---

### Task 2: Dockerfile, pinned version, and a smoke run against the real package

This task checks the parts of the spec that are still open: the real result shape of `describe()`, whether `backend` exists, and whether a GPU is usable.

**Files:**
- Create: `internal/tools/mirage_sidecar/Dockerfile`
- Create: `internal/tools/mirage_sidecar/CACTUS_VERSION`
- Create: `internal/tools/mirage_sidecar/fixture_real.json`
- Create: `internal/tools/mirage_sidecar/test_real_fixture.py`
- Modify (only if the real shape differs): `internal/tools/mirage_sidecar/server.py`
- Scratch (not committed): `smoke.py` in the session scratchpad

- [ ] **Step 1: Create the smoke script**

Create `smoke.py` in the scratchpad directory (use the Write tool, not a heredoc):

```python
import json
import sys

import cactus

model = cactus.Mirage()
result = model.describe(sys.argv[1])
print("RESULT_TYPE", type(result).__name__)
print(json.dumps(result, indent=2, default=str)[:8000])
print("BACKEND_ATTR", repr(getattr(model, "backend", "<none>")))
print("PUBLIC_ATTRS", [n for n in dir(model) if not n.startswith("_")])
```

Pick any local photo that contains a visible object and some text, copy it next to `smoke.py` as `photo.png`.

- [ ] **Step 2: Find the version and run the smoke test in a Linux container (CPU)**

Run (adjust `<SCRATCH>` to the absolute scratchpad path):

```bash
docker run --rm -v "<SCRATCH>:/work" -w /work python:3.12-slim sh -c "pip install -q cactus-compute && pip show cactus-compute | head -3 && python smoke.py /work/photo.png"
```

Expected: the `Version:` line, then `RESULT_TYPE dict` and JSON. Write down: (a) the version, (b) the top-level keys, (c) the shape of one object / text / relation entry, (d) the `BACKEND_ATTR` line. The first run downloads the weights and takes a while.

- [ ] **Step 3: Pin the version and keep the real output as a fixture**

Create `internal/tools/mirage_sidecar/CACTUS_VERSION` containing only the version number from step 2 plus a trailing newline.
Save the printed JSON object to `internal/tools/mirage_sidecar/fixture_real.json`.

- [ ] **Step 4: Fixture test against the formatter**

Create `internal/tools/mirage_sidecar/test_real_fixture.py`:

```python
import json
import os
import unittest

import server


class RealFixtureTest(unittest.TestCase):
    def test_real_output_is_formatted(self):
        path = os.path.join(os.path.dirname(__file__), "fixture_real.json")
        with open(path, encoding="utf-8") as fh:
            result = json.load(fh)
        text = server.format_description(result)
        self.assertNotEqual(text, "No content could be described.")
        self.assertTrue(text.strip())


if __name__ == "__main__":
    unittest.main()
```

Run: `python -m unittest discover -s internal/tools/mirage_sidecar -p "test_*.py" -v`
Expected: PASS. If it fails with "No content could be described.", the real key names differ from `scene` / `objects` / `text` / `relations`. Then adjust the key names in `format_description` and the `_first(...)` alternatives in `server.py` to the real ones from step 2, extend `SAMPLE` in `test_server.py` accordingly, and re-run until all tests pass. If `BACKEND_ATTR` printed `'<none>'`, keep the `"cpu"` default in `CactusEngine` and note it in step 7.

- [ ] **Step 5: Dockerfile**

Create `internal/tools/mirage_sidecar/Dockerfile`:

```dockerfile
ARG BASE_IMAGE=python:3.12-slim
FROM ${BASE_IMAGE}
ARG CACTUS_VERSION
ENV PYTHONUNBUFFERED=1 PYTHONDONTWRITEBYTECODE=1 HOME=/data PIP_NO_CACHE_DIR=1
RUN pip install "cactus-compute==${CACTUS_VERSION}"
WORKDIR /app
COPY server.py /app/server.py
EXPOSE 8080
CMD ["python", "/app/server.py"]
```

- [ ] **Step 6: Build and run the real image once**

Run (version from `CACTUS_VERSION`):

```bash
docker build -t aurago-mirage-smoke --build-arg CACTUS_VERSION=<version> internal/tools/mirage_sidecar
docker run --rm -d --name aurago_mirage_smoke -p 127.0.0.1:18791:8080 -v "<SCRATCH>/mirage-data:/data" aurago-mirage-smoke
```

Poll `http://127.0.0.1:18791/health` until `"ready": true` (the first start downloads the weights into the mounted volume). Then send a request with a real data URL, e.g. with PowerShell:

```powershell
$b64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes("<SCRATCH>\photo.png"))
$body = @{ model="mirage"; messages=@(@{ role="user"; content=@(@{ type="image_url"; image_url=@{ url="data:image/png;base64,$b64" } }) }) } | ConvertTo-Json -Depth 8
Invoke-RestMethod -Uri http://127.0.0.1:18791/v1/chat/completions -Method Post -ContentType application/json -Body $body
```

Expected: `choices[0].message.content` is a readable description. Then stop and remove only this container: `docker rm -f aurago_mirage_smoke`.

- [ ] **Step 7: GPU probe (only if the host has an NVIDIA GPU with Docker support)**

Run `docker run --rm --gpus all python:3.12-slim true`. If it succeeds, repeat step 6 with `--gpus all` added and `-e MIRAGE_GPU_BACKEND=nvidia`, and compare `/health` `backend` and the time per request with the CPU run. If it fails, skip this step. Record the outcome (GPU used / CPU only / not testable here) in the spec section "Offene Punkte" 1 and 3, replacing the open question with the finding. No code change is needed either way: the Go side passes the devices through and the UI shows the backend the engine reports.

- [ ] **Step 8: Commit**

```bash
git add internal/tools/mirage_sidecar/Dockerfile internal/tools/mirage_sidecar/CACTUS_VERSION internal/tools/mirage_sidecar/fixture_real.json internal/tools/mirage_sidecar/test_real_fixture.py internal/tools/mirage_sidecar/server.py internal/tools/mirage_sidecar/test_server.py
git commit -m "feat(mirage): Dockerfile, pinned cactus version, real-output fixture" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- internal/tools/mirage_sidecar
```

If step 7 changed the spec, commit it separately (the `docs/` directory is git-ignored, so it needs `-f`):

```bash
git add -f docs/superpowers/specs/2026-11-08-mirage-vision-design.md
git commit -m "docs(mirage): record GPU and package findings in the spec" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- docs/superpowers/specs/2026-11-08-mirage-vision-design.md
```

---

### Task 3: Config block `vision.mirage`

**Files:**
- Modify: `internal/config/config_types.go:1491-1500` (Vision struct)
- Modify: `internal/config/config.go` (defaults, next to the Piper defaults at ~840)
- Create: `internal/config/vision_mirage.go`
- Create: `internal/config/vision_mirage_test.go`
- Modify: `config_template.yaml` (~1056)

- [ ] **Step 1: Write the failing tests**

Create `internal/config/vision_mirage_test.go`. First open `internal/config/config_test.go:184-231` (`TestLoadAppliesSupertonicTTSDefaults`) and copy its way of writing an empty YAML file and calling `Load`; the `Load` call below must match that signature.

```go
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesMirageVisionDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := cfg.Vision.Mirage
	if m.Enabled {
		t.Fatal("Mirage must be disabled by default")
	}
	if m.Mode != MirageModeFallback || m.ContainerName != MirageDefaultContainerName || m.ContainerPort != 8791 ||
		m.Image != "python:3.12-slim" || m.DataPath != "data/mirage" || m.GPU != "auto" ||
		m.MaxObjects != 20 || m.MaxTextLines != 30 || m.Language != "" {
		t.Fatalf("unexpected defaults: %+v", m)
	}
}

func TestMirageModeNormalisesUnknownValues(t *testing.T) {
	cfg := &Config{}
	for in, want := range map[string]string{"": MirageModeFallback, "ALWAYS": MirageModeAlways, " always ": MirageModeAlways, "weird": MirageModeFallback} {
		cfg.Vision.Mirage.Mode = in
		if got := cfg.MirageMode(); got != want {
			t.Fatalf("MirageMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMirageBaseURLEndsWithV1(t *testing.T) {
	cfg := &Config{}
	cfg.Vision.Mirage.ContainerName = MirageDefaultContainerName
	cfg.Vision.Mirage.ContainerPort = 8791
	got := cfg.MirageBaseURL()
	if !strings.HasSuffix(got, "/v1") {
		t.Fatalf("MirageBaseURL() = %q, want a /v1 suffix", got)
	}
	if !cfg.Runtime.IsDocker && !probeDockerContainer() && !strings.Contains(got, ":8791") {
		t.Fatalf("MirageBaseURL() = %q, want the published host port", got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/config -run '(?i)(TestLoadAppliesMirageVisionDefaults|TestMirageModeNormalisesUnknownValues|TestMirageBaseURLEndsWithV1)' -count=1`
Expected: build FAIL (`cfg.Vision.Mirage undefined`, `MirageModeFallback undefined`).

- [ ] **Step 3: Add the struct**

In `internal/config/config_types.go`, inside the `Vision struct { ... } \`yaml:"vision"\`` block (lines 1491-1500), add before the closing `}`:

```go
		Mirage struct {
			Enabled       bool   `yaml:"enabled"        json:"enabled"`
			Mode          string `yaml:"mode"           json:"mode"`           // "fallback" (default) | "always"
			ContainerName string `yaml:"container_name" json:"container_name"` // default aurago_mirage
			ContainerPort int    `yaml:"container_port" json:"container_port"` // host port bound on 127.0.0.1
			Image         string `yaml:"image"          json:"image"`          // Python base image for the sidecar build
			DataPath      string `yaml:"data_path"      json:"data_path"`      // weights cache, default data/mirage
			GPU           string `yaml:"gpu"            json:"gpu"`            // auto | none | nvidia | amd | intel | vulkan
			MaxObjects    int    `yaml:"max_objects"    json:"max_objects"`
			MaxTextLines  int    `yaml:"max_text_lines" json:"max_text_lines"`
			Language      string `yaml:"language"       json:"language"` // empty = engine default
		} `yaml:"mirage" json:"mirage"`
```

- [ ] **Step 4: Add the defaults**

In `internal/config/config.go`, directly after the line `cfg.TTS.Piper.Voice = "de_DE-thorsten-high"` (~843) add:

```go
	cfg.Vision.Mirage.Mode = MirageModeFallback
	cfg.Vision.Mirage.ContainerName = MirageDefaultContainerName
	cfg.Vision.Mirage.ContainerPort = 8791
	cfg.Vision.Mirage.Image = "python:3.12-slim"
	cfg.Vision.Mirage.DataPath = "data/mirage"
	cfg.Vision.Mirage.GPU = "auto"
	cfg.Vision.Mirage.MaxObjects = 20
	cfg.Vision.Mirage.MaxTextLines = 30
```

- [ ] **Step 5: Helpers**

Create `internal/config/vision_mirage.go`. Copy the `providerutil` import line from the import block of `internal/config/config_migrate.go` (it already imports it).

```go
package config

import (
	"strings"

	"aurago/internal/providerutil" // copy the exact path from config_migrate.go
)

const (
	// MirageDefaultContainerName is the managed sidecar's container name.
	MirageDefaultContainerName = "aurago_mirage"
	// MirageInternalPort is the port the sidecar listens on inside its container.
	MirageInternalPort = 8080
	MirageModeFallback = "fallback"
	MirageModeAlways   = "always"
)

// MirageMode returns "always" or "fallback"; anything else is "fallback".
func (c *Config) MirageMode() string {
	if strings.EqualFold(strings.TrimSpace(c.Vision.Mirage.Mode), MirageModeAlways) {
		return MirageModeAlways
	}
	return MirageModeFallback
}

// MirageBaseURL is the OpenAI-style base URL (ending in /v1) of the sidecar.
// On the host it is the published loopback port; inside Docker it is the
// container name on the shared network (same helper the Manifest sidecar uses).
func (c *Config) MirageBaseURL() string {
	inDocker := c.Runtime.IsDocker || probeDockerContainer()
	port := c.Vision.Mirage.ContainerPort
	if inDocker {
		port = MirageInternalPort
	}
	name := strings.TrimSpace(c.Vision.Mirage.ContainerName)
	if name == "" {
		name = MirageDefaultContainerName
	}
	base := providerutil.NormalizeBaseURL(defaultSidecarURL(inDocker, name, port))
	return strings.TrimRight(base, "/") + "/v1"
}
```

Open `internal/config/config_migrate.go:192-208` (`ManifestProviderBaseURL`) and confirm that `defaultSidecarURL(inDocker, name, port)` has that argument order and returns a string; fix the call if the signature differs.

- [ ] **Step 6: Template**

In `config_template.yaml`, replace

```yaml
vision:
    provider: ""
```

(lines ~1056-1057) with:

```yaml
vision:
    provider: ""
    # Optional local vision for weak systems: the Cactus Mirage model in a managed Docker sidecar.
    # Used only while `provider` is empty. mode: fallback = when the main model cannot read images,
    # always = whenever provider is empty. gpu: auto | none | nvidia | amd | intel | vulkan (Linux detection).
    mirage:
        enabled: false
        mode: fallback
        container_name: aurago_mirage
        container_port: 8791
        image: python:3.12-slim
        data_path: data/mirage
        gpu: auto
        max_objects: 20
        max_text_lines: 30
        language: ""
```

- [ ] **Step 7: Run to verify pass**

Run: `go test ./internal/config -run '(?i)(TestLoadAppliesMirageVisionDefaults|TestMirageModeNormalisesUnknownValues|TestMirageBaseURLEndsWithV1)' -count=1 -v` (expect 3 `--- PASS`), then `go test ./internal/config -count=1` and `go test ./internal/server -run 'TestConfigSchemaConsistency' -count=1`.
Expected: PASS. If a template/schema consistency test names the new keys, fix the template or struct so both agree.

- [ ] **Step 8: Commit**

```bash
git add internal/config/config_types.go internal/config/config.go internal/config/vision_mirage.go internal/config/vision_mirage_test.go config_template.yaml
git commit -m "feat(mirage): vision.mirage config block with defaults" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- internal/config/config_types.go internal/config/config.go internal/config/vision_mirage.go internal/config/vision_mirage_test.go config_template.yaml
```

---

### Task 4: Vision routing (Go client + hook)

**Files:**
- Create: `internal/tools/vision_mirage.go`
- Create: `internal/tools/vision_mirage_test.go`
- Modify: `internal/tools/vision.go:155` (`analyzeImageReferenceWithPromptContext`)

- [ ] **Step 1: Impact check (project rule)**

Run GitNexus `impact({target: "analyzeImageReferenceWithPromptContext", direction: "upstream"})` and report callers and risk. Expected callers: `AnalyzeImageWithPromptContext`, `AnalyzeImageURLWithPromptContext`, `analyzeLocalImageWithPromptContext`. The change is additive (an early return that only fires when Mirage is enabled and `vision.provider` is empty), so the risk for existing setups is nil.

- [ ] **Step 2: Write the failing tests**

Create `internal/tools/vision_mirage_test.go`. Reuse the server pattern from `vision_test.go:22-30` (`testutil.NewHTTPServer`); copy its import for `testutil`.

```go
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

func mirageTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Vision.Mirage.Enabled = true
	cfg.Vision.Mirage.Mode = config.MirageModeFallback
	return cfg
}

func overrideMirageHooks(t *testing.T, baseURL string, mainSupportsImage bool) {
	t.Helper()
	prevURL, prevMain := mirageBaseURLFor, mirageMainSupportsImage
	mirageBaseURLFor = func(*config.Config) string { return baseURL }
	mirageMainSupportsImage = func(*config.Config) bool { return mainSupportsImage }
	t.Cleanup(func() { mirageBaseURLFor, mirageMainSupportsImage = prevURL, prevMain })
}

func TestUseMirageVisionDecisionTable(t *testing.T) {
	overrideMirageHooks(t, "http://unused/v1", false)
	cases := []struct {
		name   string
		mutate func(*config.Config)
		main   bool
		want   bool
	}{
		{"disabled", func(c *config.Config) { c.Vision.Mirage.Enabled = false }, false, false},
		{"explicit provider wins", func(c *config.Config) { c.Vision.Provider = "openrouter" }, false, false},
		{"fallback and main cannot see", func(c *config.Config) {}, false, true},
		{"fallback and main can see", func(c *config.Config) {}, true, false},
		{"always even if main can see", func(c *config.Config) { c.Vision.Mirage.Mode = config.MirageModeAlways }, true, true},
		{"always but explicit provider", func(c *config.Config) { c.Vision.Mirage.Mode = config.MirageModeAlways; c.Vision.Provider = "p" }, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mirageMainSupportsImage = func(*config.Config) bool { return tc.main }
			cfg := mirageTestConfig()
			tc.mutate(cfg)
			if got := UseMirageVision(cfg); got != tc.want {
				t.Fatalf("UseMirageVision = %v, want %v", got, tc.want)
			}
		})
	}
	if UseMirageVision(nil) {
		t.Fatal("nil config must not use Mirage")
	}
}

func TestAnalyzeWithMirageSendsOnlyTheImageAndNeverTheMainKey(t *testing.T) {
	var gotAuth, gotModel, gotURL string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var req struct {
			Model    string `json:"model"`
			Messages []struct {
				Content []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotModel = req.Model
		gotURL = req.Messages[0].Content[0].ImageURL.URL
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"Scene: a cat."}}]}`)
	}))
	defer server.Close()
	overrideMirageHooks(t, server.URL+"/v1", false)

	cfg := mirageTestConfig()
	cfg.LLM.APIKey = "main-secret"
	text, in, out, err := analyzeImageReferenceWithPromptContext(context.Background(), "data:image/png;base64,AAAA", "what is this", cfg)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if text != "Scene: a cat." || in != 0 || out != 0 {
		t.Fatalf("got (%q,%d,%d)", text, in, out)
	}
	if strings.Contains(gotAuth, "main-secret") || gotAuth != "" {
		t.Fatalf("Authorization = %q, want none", gotAuth)
	}
	if gotModel != "mirage" || gotURL != "data:image/png;base64,AAAA" {
		t.Fatalf("model=%q url=%q", gotModel, gotURL)
	}
}

func TestAnalyzeWithMirageRejectsPublicURLs(t *testing.T) {
	overrideMirageHooks(t, "http://unused/v1", false)
	_, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), "https://example.com/a.png", "x", mirageTestConfig())
	if err == nil || !strings.Contains(err.Error(), "local images") {
		t.Fatalf("err = %v, want a local-images error", err)
	}
}

func TestAnalyzeWithMirageMapsStatusCodes(t *testing.T) {
	for status, want := range map[int]string{http.StatusServiceUnavailable: "loading", http.StatusInternalServerError: "status 500"} {
		status, want := status, want
		server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(status)
			_, _ = io.WriteString(w, `{"error":{"message":"secret detail"}}`)
		}))
		overrideMirageHooks(t, server.URL+"/v1", false)
		_, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), "data:image/png;base64,AAAA", "x", mirageTestConfig())
		server.Close()
		if err == nil || !strings.Contains(err.Error(), want) || strings.Contains(err.Error(), "secret detail") {
			t.Fatalf("status %d: err = %v, want %q without the body", status, err, want)
		}
	}
}

func TestAnalyzeWithMirageUnreachableSidecarIsAClearError(t *testing.T) {
	overrideMirageHooks(t, "http://127.0.0.1:1/v1", false)
	_, _, _, err := analyzeImageReferenceWithPromptContext(context.Background(), "data:image/png;base64,AAAA", "x", mirageTestConfig())
	if err == nil || !strings.Contains(err.Error(), "not reachable") {
		t.Fatalf("err = %v", err)
	}
}

func TestTrustedLocalFileReachesMirageAsDataURL(t *testing.T) {
	var gotURL string
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []struct {
				Content []struct {
					ImageURL struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotURL = req.Messages[0].Content[0].ImageURL.URL
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()
	overrideMirageHooks(t, server.URL+"/v1", false)

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	text, _, _, err := AnalyzeTrustedImageFileWithPromptContext(context.Background(), path, "describe", mirageTestConfig())
	if err != nil || text != "ok" {
		t.Fatalf("got (%q, %v)", text, err)
	}
	if !strings.HasPrefix(gotURL, "data:image/") {
		t.Fatalf("sidecar received %.30q, want a data URL", gotURL)
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/tools -run '(?i)(TestUseMirageVisionDecisionTable|TestAnalyzeWithMirage|TestTrustedLocalFileReachesMirage)' -count=1`
Expected: build FAIL (`UseMirageVision undefined`).

- [ ] **Step 4: Implement**

Create `internal/tools/vision_mirage.go`:

{% raw %}
```go
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/llm"
)

const (
	mirageModelName        = "mirage"
	mirageRequestTimeout   = 90 * time.Second // first request after a cold start may wait for the model
	mirageMaxResponseBytes = 1 << 20
)

// Hooks so tests can point the client at a fake sidecar and fake the main
// model's capabilities.
var (
	mirageBaseURLFor        = func(cfg *config.Config) string { return cfg.MirageBaseURL() }
	mirageMainSupportsImage = func(cfg *config.Config) bool { return llm.ResolveConfigProviderCapabilities(cfg).Image }
)

// UseMirageVision reports whether image analysis should go to the local Mirage
// sidecar. An explicitly configured vision provider always wins. In "fallback"
// mode Mirage is used only while the main model cannot take images (a
// simplification of the server's mainProviderSupportsImageMultimodal, which
// additionally consults multimodal_provider_types_extra).
func UseMirageVision(cfg *config.Config) bool {
	if cfg == nil || !cfg.Vision.Mirage.Enabled {
		return false
	}
	if strings.TrimSpace(cfg.Vision.Provider) != "" {
		return false
	}
	if cfg.MirageMode() == config.MirageModeAlways {
		return true
	}
	return !mirageMainSupportsImage(cfg)
}

// analyzeWithMirage asks the sidecar for a text description of a data-URL
// image. It deliberately never reads cfg.LLM.APIKey: the sidecar needs no key
// and must not receive the main provider's token.
func analyzeWithMirage(ctx context.Context, imageReference string, cfg *config.Config) (string, int, int, error) {
	if !strings.HasPrefix(imageReference, "data:image/") {
		return "", 0, 0, fmt.Errorf("mirage vision can only analyze local images, not URLs")
	}
	body, err := json.Marshal(map[string]interface{}{
		"model": mirageModelName,
		"messages": []map[string]interface{}{{
			"role": "user",
			"content": []map[string]interface{}{
				{"type": "image_url", "image_url": map[string]string{"url": imageReference}},
			},
		}},
	})
	if err != nil {
		return "", 0, 0, fmt.Errorf("mirage request encoding failed: %w", err)
	}
	endpoint := strings.TrimRight(mirageBaseURLFor(cfg), "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, 0, fmt.Errorf("mirage request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{
		Timeout:       mirageRequestTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, 0, fmt.Errorf("mirage sidecar is not reachable (is it enabled and started?): %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, mirageMaxResponseBytes))
	if err != nil {
		return "", 0, 0, fmt.Errorf("mirage response read failed: %w", err)
	}
	switch {
	case resp.StatusCode == http.StatusServiceUnavailable:
		return "", 0, 0, fmt.Errorf("mirage model is still loading, try again shortly")
	case resp.StatusCode != http.StatusOK:
		return "", 0, 0, fmt.Errorf("mirage vision error (status %d)", resp.StatusCode)
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil || len(parsed.Choices) == 0 {
		return "", 0, 0, fmt.Errorf("no analysis received in mirage response")
	}
	return parsed.Choices[0].Message.Content, 0, 0, nil
}
```
{% endraw %}

If `llm.ResolveConfigProviderCapabilities` has a different signature than `(cfg) caps` with an `.Image` field, open `internal/llm/model_capabilities.go:65` and `internal/server/multimodal_ingest.go:219-237` and adapt the one-line hook; the tests stub it.

- [ ] **Step 5: Add the routing hook**

In `internal/tools/vision.go`, in `analyzeImageReferenceWithPromptContext` (line 155), directly after the `if ctx == nil { ctx = context.Background() }` block and before `prompt, _ = PrepareVisionPrompt(prompt)` add:

```go
	if UseMirageVision(cfg) {
		return analyzeWithMirage(ctx, imageReference, cfg)
	}
```

- [ ] **Step 6: Run to verify pass**

Run: `go test ./internal/tools -run '(?i)(TestUseMirageVisionDecisionTable|TestAnalyzeWithMirage|TestTrustedLocalFileReachesMirage)' -count=1 -v` (expect PASS lines for every subtest), then `go test ./internal/tools -run 'TestAnalyzeImage' -count=1` and `go test ./internal/telegram -count=1` to confirm the existing vision paths are unchanged.

- [ ] **Step 7: Commit**

```bash
git add internal/tools/vision_mirage.go internal/tools/vision_mirage_test.go internal/tools/vision.go
git commit -m "feat(mirage): route vision to the local sidecar when no provider is set" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- internal/tools/vision_mirage.go internal/tools/vision_mirage_test.go internal/tools/vision.go
```

---

### Task 5: Sidecar manager (image, GPU, container, status)

**Files:**
- Create: `internal/tools/mirage_sidecar.go`
- Create: `internal/tools/mirage_sidecar_test.go`
- Modify: `internal/tools/managed_sidecar_names.go:34`
- Modify: `internal/tools/managed_sidecar_names_test.go:14-17`
- Modify: `internal/dockerutil/managed_resources.go` (const block, near `OllamaOwner` at ~15)
- Modify: `internal/server/containers_protection.go:38-52`

- [ ] **Step 1: Owner constant and protection**

In `internal/dockerutil/managed_resources.go`, in the const block that defines `OllamaOwner = "ollama"` (line ~15), add the same-style line:

```go
	MirageOwner = "mirage"
```

In `internal/server/containers_protection.go`, in `containerProtectedOwners` (lines 38-52) add `dockerutil.MirageOwner` next to `dockerutil.OllamaOwner`, using the same syntax as the neighbouring entries. Run `go build ./internal/dockerutil ./internal/server`.
Expected: success.

- [ ] **Step 2: Reserve the container name**

In `internal/tools/managed_sidecar_names.go` after the line `add(gotenbergContainerName, ollamaManagedContainerName, ollamaEmbContainerName, piperContainerName)` add:

```go
	add(config.MirageDefaultContainerName, cfg.Vision.Mirage.ContainerName)
```

In `internal/tools/managed_sidecar_names_test.go`, add `"aurago_mirage"` to the `want` list (lines 14-17). Run `go test ./internal/tools -run TestManagedSidecarContainerNamesListEveryManagedSidecar -count=1`.
Expected: PASS.

- [ ] **Step 3: Write the failing tests**

Create `internal/tools/mirage_sidecar_test.go`. The fake-Docker helpers `configureDockerSecurityTestPermissions` and `fakeDockerHost` live in `docker_security_test.go:22,30`. Check `docker_test.go:65` for what a fake `/build` response must look like and keep `{"stream":"ok"}` only if that test accepts it.

```go
package tools

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/testutil"
)

// Reuse an existing shared test logger if package tools already has one
// (grep -rn "func testLogger\|func discardLogger" internal/tools) and delete this.
func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func mirageSidecarTestConfig(t *testing.T, dockerHost string) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = dockerHost
	m := &cfg.Vision.Mirage
	m.Enabled = true
	m.Mode = config.MirageModeFallback
	m.ContainerName = config.MirageDefaultContainerName
	m.ContainerPort = 8791
	m.Image = "python:3.12-slim"
	m.DataPath = t.TempDir()
	m.GPU = "none"
	m.MaxObjects = 20
	m.MaxTextLines = 30
	return cfg
}

func readyHealthServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ready":true,"backend":"cpu","gpu_requested":"none"}`)
	}))
	prev := mirageBaseURLFor
	mirageBaseURLFor = func(*config.Config) string { return server.URL + "/v1" }
	t.Cleanup(func() { mirageBaseURLFor = prev; server.Close() })
	return server
}

func TestMirageImageTagChangesWithTheBaseImage(t *testing.T) {
	a, b := mirageImageTag("python:3.12-slim"), mirageImageTag("python:3.13-slim")
	if a == b || !strings.HasPrefix(a, "aurago/mirage:") {
		t.Fatalf("tags %q / %q", a, b)
	}
	if a != mirageImageTag("python:3.12-slim") {
		t.Fatal("tag must be stable")
	}
}

func TestMirageApplyGPUNvidiaUsesDeviceRequests(t *testing.T) {
	host := map[string]interface{}{}
	mirageApplyGPU(host, GPUInfo{Backend: "nvidia"})
	reqs, ok := host["DeviceRequests"].([]map[string]interface{})
	if !ok || len(reqs) != 1 || reqs[0]["Driver"] != "nvidia" {
		t.Fatalf("DeviceRequests = %#v", host["DeviceRequests"])
	}
	if _, has := host["Devices"]; has {
		t.Fatal("nvidia must not add raw devices")
	}
}

func TestMirageApplyGPUDriNodesBecomeDevices(t *testing.T) {
	host := map[string]interface{}{}
	mirageApplyGPU(host, GPUInfo{Backend: "vulkan", Devices: []string{"/dev/dri/renderD128"}})
	devices, ok := host["Devices"].([]map[string]string)
	if !ok || len(devices) != 1 || devices[0]["PathOnHost"] != "/dev/dri/renderD128" || devices[0]["PathInContainer"] != "/dev/dri/renderD128" {
		t.Fatalf("Devices = %#v", host["Devices"])
	}
	none := map[string]interface{}{}
	mirageApplyGPU(none, GPUInfo{Backend: "none"})
	if len(none) != 0 {
		t.Fatalf("none must not touch the host config: %#v", none)
	}
}

func TestMirageDetectGPUNoneSkipsDetection(t *testing.T) {
	if got := mirageDetectGPU(" None "); got.Backend != "none" {
		t.Fatalf("backend = %q", got.Backend)
	}
}

func TestMirageStatusDisabled(t *testing.T) {
	if got := MirageStatus(MirageStatusTarget{}); got.Status != "disabled" {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestMirageStatusReportsTheEngineBackend(t *testing.T) {
	server := readyHealthServer(t)
	got := MirageStatus(MirageStatusTarget{Enabled: true, HealthURL: server.URL + "/health"})
	if got.Status != "running" || !got.Ready || got.Backend != "cpu" {
		t.Fatalf("status = %+v", got)
	}
}

func TestMirageStatusStoppedWhenNothingAnswers(t *testing.T) {
	got := MirageStatus(MirageStatusTarget{Enabled: true, HealthURL: "http://127.0.0.1:1/health"})
	if got.Status != "stopped" {
		t.Fatalf("status = %q", got.Status)
	}
}

type mirageDockerRecorder struct {
	built, created, started, deleted atomic.Int32
	createBody                       map[string]interface{}
}

func newMirageFakeDocker(t *testing.T, rec *mirageDockerRecorder, existing func() (code int, body string), onDelete func()) string {
	t.Helper()
	return fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case r.Method == http.MethodGet && strings.Contains(path, "/images/") && strings.HasSuffix(path, "/json"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/build"):
			rec.built.Add(1)
			_, _ = io.WriteString(w, "{\"stream\":\"ok\"}\n")
		case r.Method == http.MethodGet && strings.Contains(path, "/containers/aurago_mirage/json"):
			code, body := existing()
			w.WriteHeader(code)
			_, _ = io.WriteString(w, body)
		case r.Method == http.MethodDelete && strings.Contains(path, "/containers/aurago_mirage"):
			rec.deleted.Add(1)
			if onDelete != nil {
				onDelete()
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/containers/create"):
			rec.created.Add(1)
			_ = json.NewDecoder(r.Body).Decode(&rec.createBody)
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"Id":"abc"}`)
		case r.Method == http.MethodPost && strings.HasSuffix(path, "/containers/aurago_mirage/start"):
			rec.started.Add(1)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func TestEnsureMirageRunningBuildsAndCreatesTheContainer(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	readyHealthServer(t)
	mirageBuildBackoff.reset()
	rec := &mirageDockerRecorder{}
	host := newMirageFakeDocker(t, rec, func() (int, string) { return http.StatusNotFound, "" }, nil)
	cfg := mirageSidecarTestConfig(t, host)

	if err := EnsureMirageRunning(cfg, testLogger(t), true); err != nil {
		t.Fatalf("EnsureMirageRunning: %v", err)
	}
	if rec.built.Load() != 1 || rec.created.Load() != 1 || rec.started.Load() != 1 {
		t.Fatalf("built=%d created=%d started=%d", rec.built.Load(), rec.created.Load(), rec.started.Load())
	}
	labels, _ := rec.createBody["Labels"].(map[string]interface{})
	if labels[mirageFingerprintLabel] == nil {
		t.Fatalf("labels = %#v, want the fingerprint label", labels)
	}
	hostCfg, _ := rec.createBody["HostConfig"].(map[string]interface{})
	if caps, _ := hostCfg["CapDrop"].([]interface{}); len(caps) != 1 || caps[0] != "ALL" {
		t.Fatalf("CapDrop = %#v", hostCfg["CapDrop"])
	}
	ports, _ := hostCfg["PortBindings"].(map[string]interface{})
	if _, ok := ports["8080/tcp"]; !ok {
		t.Fatalf("PortBindings = %#v", ports)
	}
	binds, _ := hostCfg["Binds"].([]interface{})
	if len(binds) != 1 || !strings.HasSuffix(binds[0].(string), ":/data") {
		t.Fatalf("Binds = %#v", binds)
	}
}

func TestEnsureMirageRunningReplacesAContainerWithAStaleFingerprint(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	readyHealthServer(t)
	mirageBuildBackoff.reset()
	rec := &mirageDockerRecorder{}
	var deleted atomic.Bool
	host := newMirageFakeDocker(t, rec, func() (int, string) {
		if deleted.Load() {
			return http.StatusNotFound, ""
		}
		return http.StatusOK, `{"State":{"Running":true},"Config":{"Labels":{"` + mirageFingerprintLabel + `":"stale"}}}`
	}, func() { deleted.Store(true) }) // after DELETE the container is gone
	cfg := mirageSidecarTestConfig(t, host)
	if err := EnsureMirageRunning(cfg, testLogger(t), true); err != nil {
		t.Fatalf("EnsureMirageRunning: %v", err)
	}
	if rec.deleted.Load() != 1 || rec.created.Load() != 1 {
		t.Fatalf("deleted=%d created=%d", rec.deleted.Load(), rec.created.Load())
	}
}

func TestEnsureMirageRunningKeepsAMatchingRunningContainer(t *testing.T) {
	configureDockerSecurityTestPermissions(t, false)
	readyHealthServer(t)
	mirageBuildBackoff.reset()
	rec := &mirageDockerRecorder{}
	var fp string
	host := newMirageFakeDocker(t, rec, func() (int, string) {
		return http.StatusOK, `{"State":{"Running":true},"Config":{"Labels":{"` + mirageFingerprintLabel + `":"` + fp + `"}}}`
	}, nil)
	cfg := mirageSidecarTestConfig(t, host)
	fp = mirageCurrentFingerprint(cfg)
	if err := EnsureMirageRunning(cfg, testLogger(t), true); err != nil {
		t.Fatalf("EnsureMirageRunning: %v", err)
	}
	if rec.deleted.Load() != 0 || rec.created.Load() != 0 {
		t.Fatalf("deleted=%d created=%d, want the container left alone", rec.deleted.Load(), rec.created.Load())
	}
}

func TestEnsureMirageRunningDoesNothingWhenDisabled(t *testing.T) {
	cfg := &config.Config{}
	if err := EnsureMirageRunning(cfg, testLogger(t), true); err != nil {
		t.Fatalf("err = %v", err)
	}
}
```

Check before running: `testutil.NewHTTPServer` must return `*httptest.Server` (it is used as `server.URL` / `server.Close()` in `vision_test.go`); if a `testLogger` helper already exists in package `tools`, delete the one above so the names do not clash.

- [ ] **Step 4: Run to verify failure**

Run: `go test ./internal/tools -run '(?i)(TestMirage|TestEnsureMirage)' -count=1`
Expected: build FAIL (`mirageImageTag undefined`, ...).

- [ ] **Step 5: Implement**

Create `internal/tools/mirage_sidecar.go`. Copy the `dockerutil` import path from `internal/tools/ollama_container.go`.

{% raw %}
```go
package tools

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil" // copy the exact path from ollama_container.go
)

//go:embed mirage_sidecar/Dockerfile
var mirageSidecarDockerfile []byte

//go:embed mirage_sidecar/server.py
var mirageSidecarServer []byte

//go:embed mirage_sidecar/CACTUS_VERSION
var mirageSidecarVersionRaw []byte

const (
	mirageFingerprintLabel = "aurago.mirage.fingerprint"
	mirageReadyTimeout     = 10 * time.Minute // first start downloads the weights
)

var (
	mirageEnsureMu     sync.Mutex
	mirageBuildBackoff go2RTCBuildBackoff
)

func mirageCactusVersion() string { return strings.TrimSpace(string(mirageSidecarVersionRaw)) }

// mirageImageTag is content-addressed: any change to the Dockerfile, the
// server, the pinned cactus version or the base image yields a new tag and a
// rebuild.
func mirageImageTag(base string) string {
	h := sha256.New()
	for _, part := range [][]byte{mirageSidecarDockerfile, mirageSidecarServer, []byte(mirageCactusVersion()), []byte(base)} {
		h.Write(part)
		h.Write([]byte{0})
	}
	return "aurago/mirage:" + hex.EncodeToString(h.Sum(nil)[:8])
}

func mirageContainerName(cfg *config.Config) string {
	if name := strings.TrimSpace(cfg.Vision.Mirage.ContainerName); name != "" {
		return name
	}
	return config.MirageDefaultContainerName
}

func mirageDetectGPU(pref string) GPUInfo {
	pref = strings.ToLower(strings.TrimSpace(pref))
	if pref == "none" || pref == "off" {
		return GPUInfo{Backend: "none"}
	}
	return DetectGPU(pref)
}

// mirageApplyGPU adds the device passthrough for the detected backend, the same
// shapes the Ollama sidecar uses (applyGPUToCreateHost).
func mirageApplyGPU(host map[string]interface{}, gpu GPUInfo) {
	switch gpu.Backend {
	case "nvidia":
		host["DeviceRequests"] = []map[string]interface{}{{
			"Driver":       "nvidia",
			"Count":        -1,
			"Capabilities": [][]string{{"gpu"}},
		}}
	case "amd", "intel", "vulkan":
		devices := make([]map[string]string, 0, len(gpu.Devices))
		for _, path := range gpu.Devices {
			devices = append(devices, map[string]string{"PathOnHost": path, "PathInContainer": path, "CgroupPermissions": "rwm"})
		}
		if len(devices) > 0 {
			host["Devices"] = devices
		}
		if gids := hostGroupIDs([]string{"video", "render"}); len(gids) > 0 {
			host["GroupAdd"] = gids
		}
	}
}

func mirageHostConfig(cfg *config.Config, absData string, gpu GPUInfo) map[string]interface{} {
	host := map[string]interface{}{
		"RestartPolicy": map[string]interface{}{"Name": "unless-stopped"},
		"PortBindings": map[string]interface{}{
			fmt.Sprintf("%d/tcp", config.MirageInternalPort): []map[string]string{
				{"HostIp": "127.0.0.1", "HostPort": strconv.Itoa(cfg.Vision.Mirage.ContainerPort)},
			},
		},
		"Binds":     []string{absData + ":/data"},
		"Memory":    int64(2) << 30,
		"PidsLimit": 256,
	}
	mirageApplyGPU(host, gpu)
	return hardenManagedSidecarHostConfig(host)
}

func mirageEnv(cfg *config.Config, gpu GPUInfo) []string {
	m := cfg.Vision.Mirage
	return []string{
		"MIRAGE_PORT=" + strconv.Itoa(config.MirageInternalPort),
		"MIRAGE_MAX_OBJECTS=" + strconv.Itoa(m.MaxObjects),
		"MIRAGE_MAX_TEXT_LINES=" + strconv.Itoa(m.MaxTextLines),
		"MIRAGE_GPU_BACKEND=" + gpu.Backend,
		"MIRAGE_LANGUAGE=" + strings.TrimSpace(m.Language),
		"HOME=/data",
	}
}

func mirageFingerprint(image string, cfg *config.Config, gpu GPUInfo, absData string) string {
	m := cfg.Vision.Mirage
	parts := []string{image, strconv.Itoa(m.ContainerPort), gpu.Backend, strings.Join(gpu.Devices, ","), absData,
		strconv.Itoa(m.MaxObjects), strconv.Itoa(m.MaxTextLines), strings.TrimSpace(m.Language)}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:8])
}

// mirageCurrentFingerprint recomputes the fingerprint EnsureMirageRunning would
// stamp on a new container for this config (used by tests).
func mirageCurrentFingerprint(cfg *config.Config) string {
	absData, _ := filepath.Abs(cfg.Vision.Mirage.DataPath)
	return mirageFingerprint(mirageImageTag(cfg.Vision.Mirage.Image), cfg, mirageDetectGPU(cfg.Vision.Mirage.GPU), absData)
}

type mirageContainerState struct {
	Exists      bool
	Running     bool
	Fingerprint string
}

func mirageInspect(docker DockerConfig, name string) (mirageContainerState, error) {
	data, code, err := dockerRequest(docker, http.MethodGet, "/containers/"+name+"/json", "")
	if err != nil {
		return mirageContainerState{}, err
	}
	if code == http.StatusNotFound {
		return mirageContainerState{}, nil
	}
	if code != http.StatusOK {
		return mirageContainerState{}, fmt.Errorf("docker inspect of %s returned status %d", name, code)
	}
	var info struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
		Config struct {
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(data, &info); err != nil {
		return mirageContainerState{}, fmt.Errorf("docker inspect of %s was not valid JSON: %w", name, err)
	}
	return mirageContainerState{Exists: true, Running: info.State.Running, Fingerprint: info.Config.Labels[mirageFingerprintLabel]}, nil
}

func ensureMirageImage(ctx context.Context, docker DockerConfig, image, base string, logger *slog.Logger) error {
	if _, code, err := dockerRequest(docker, http.MethodGet, "/images/"+url.PathEscape(image)+"/json", ""); err == nil && code == http.StatusOK {
		mirageBuildBackoff.reset()
		return nil
	}
	if retryAt, cause, paused := mirageBuildBackoff.paused(image); paused {
		return fmt.Errorf("mirage image build is paused until %s after an earlier failure: %w", retryAt.Format(time.RFC3339), cause)
	}
	logger.Info("[Mirage] Building sidecar image", "image", image, "cactus_version", mirageCactusVersion())
	err := BuildImageContextWait(ctx, docker, image, "Dockerfile", mirageSidecarDockerfile,
		map[string][]byte{"server.py": mirageSidecarServer},
		map[string]string{"BASE_IMAGE": base, "CACTUS_VERSION": mirageCactusVersion()}, logger)
	if err != nil {
		failures, delay, retryAt := mirageBuildBackoff.failed(image, err)
		logger.Warn("[Mirage] Sidecar image build failed; automatic retries back off",
			"error", err, "failed_attempts", failures, "retry_in", delay.String(), "next_retry_at", retryAt.UTC().Format(time.RFC3339))
		return fmt.Errorf("mirage image build failed: %w", err)
	}
	mirageBuildBackoff.reset()
	return nil
}

func mirageHealthURL(cfg *config.Config) string {
	return strings.TrimSuffix(strings.TrimRight(mirageBaseURLFor(cfg), "/"), "/v1") + "/health"
}

func waitForMirageReady(cfg *config.Config, logger *slog.Logger) error {
	deadline := time.Now().Add(mirageReadyTimeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(mirageHealthURL(cfg))
		if err == nil {
			var body struct {
				Ready bool   `json:"ready"`
				Error string `json:"error"`
			}
			decodeErr := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body)
			resp.Body.Close()
			if decodeErr == nil && body.Ready {
				return nil
			}
			if decodeErr == nil && body.Error != "" {
				return fmt.Errorf("mirage model failed to load: %s", body.Error)
			}
		}
		time.Sleep(2 * time.Second)
	}
	logger.Warn("[Mirage] Sidecar did not become ready in time")
	return fmt.Errorf("mirage sidecar did not become ready within %s", mirageReadyTimeout)
}

// EnsureMirageRunning builds the sidecar image if needed and makes sure a
// container with the current settings is running. userInitiated lifts an active
// build back-off (a settings save should retry at once; boot should not).
func EnsureMirageRunning(cfg *config.Config, logger *slog.Logger, userInitiated bool) error {
	if cfg == nil || !cfg.Vision.Mirage.Enabled || !cfg.Docker.Enabled {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	mirageEnsureMu.Lock()
	defer mirageEnsureMu.Unlock()
	if userInitiated {
		mirageBuildBackoff.reset()
	}

	m := cfg.Vision.Mirage
	docker := DockerConfig{Host: cfg.Docker.Host}
	name := mirageContainerName(cfg)
	absData, err := filepath.Abs(m.DataPath)
	if err != nil {
		return fmt.Errorf("mirage data path: %w", err)
	}
	if err := os.MkdirAll(absData, 0o755); err != nil {
		return fmt.Errorf("mirage data path: %w", err)
	}
	gpu := mirageDetectGPU(m.GPU)
	image := mirageImageTag(m.Image)
	if err := ensureMirageImage(context.Background(), docker, image, m.Image, logger); err != nil {
		return err
	}
	fp := mirageFingerprint(image, cfg, gpu, absData)

	state, err := mirageInspect(docker, name)
	if err != nil {
		return fmt.Errorf("mirage: docker unavailable: %w", err)
	}
	if state.Exists && state.Fingerprint == fp {
		if !state.Running {
			if _, code, err := dockerRequest(docker, http.MethodPost, "/containers/"+name+"/start", ""); err != nil || (code != http.StatusNoContent && code != http.StatusNotModified) {
				return fmt.Errorf("mirage: starting the existing container failed (status %d, err %v)", code, err)
			}
		}
		return waitForMirageReady(cfg, logger)
	}
	if state.Exists {
		logger.Info("[Mirage] Container settings changed; recreating", "container", name)
		if _, code, err := dockerRequest(docker, http.MethodDelete, "/containers/"+name+"?force=true", ""); err != nil || (code != http.StatusNoContent && code != http.StatusNotFound) {
			return fmt.Errorf("mirage: removing the old container failed (status %d, err %v)", code, err)
		}
	}

	labels := map[string]string{}
	for k, v := range dockerutil.ManagedLabels(dockerutil.MirageOwner, "mirage", "vision", fp) {
		labels[k] = v
	}
	labels[mirageFingerprintLabel] = fp
	payload := map[string]interface{}{
		"Image":        image,
		"Env":          mirageEnv(cfg, gpu),
		"Labels":       labels,
		"ExposedPorts": map[string]interface{}{fmt.Sprintf("%d/tcp", config.MirageInternalPort): struct{}{}},
		"HostConfig":   mirageHostConfig(cfg, absData, gpu),
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("mirage create payload: %w", err)
	}
	if _, code, err := dockerRequest(docker, http.MethodPost, "/containers/create?name="+url.QueryEscape(name), string(body)); err != nil || code != http.StatusCreated {
		return fmt.Errorf("mirage: creating the container failed (status %d, err %v)", code, err)
	}
	if _, code, err := dockerRequest(docker, http.MethodPost, "/containers/"+name+"/start", ""); err != nil || (code != http.StatusNoContent && code != http.StatusNotModified) {
		return fmt.Errorf("mirage: starting the container failed (status %d, err %v)", code, err)
	}
	logger.Info("[Mirage] Sidecar started", "container", name, "gpu_requested", gpu.Backend)
	return waitForMirageReady(cfg, logger)
}

// MirageStatusTarget is a lock-free snapshot of what MirageStatus needs, so a
// handler can release the config lock before the network call.
type MirageStatusTarget struct {
	Enabled       bool
	HealthURL     string
	DockerEnabled bool
	DockerHost    string
	ContainerName string
}

func MirageStatusTargetFor(cfg *config.Config) MirageStatusTarget {
	return MirageStatusTarget{
		Enabled:       cfg.Vision.Mirage.Enabled,
		HealthURL:     mirageHealthURL(cfg),
		DockerEnabled: cfg.Docker.Enabled,
		DockerHost:    cfg.Docker.Host,
		ContainerName: mirageContainerName(cfg),
	}
}

type MirageHealth struct {
	Status       string `json:"status"` // disabled | running | starting | stopped | error
	Ready        bool   `json:"ready"`
	Backend      string `json:"backend,omitempty"`
	GPURequested string `json:"gpu_requested,omitempty"`
	Message      string `json:"message,omitempty"`
}

func MirageStatus(t MirageStatusTarget) MirageHealth {
	if !t.Enabled {
		return MirageHealth{Status: "disabled"}
	}
	client := &http.Client{Timeout: 3 * time.Second}
	if resp, err := client.Get(t.HealthURL); err == nil {
		defer resp.Body.Close()
		var body struct {
			Ready        bool   `json:"ready"`
			Backend      string `json:"backend"`
			GPURequested string `json:"gpu_requested"`
			Error        string `json:"error"`
		}
		if resp.StatusCode == http.StatusOK && json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body) == nil {
			h := MirageHealth{Status: "starting", Ready: body.Ready, Backend: body.Backend, GPURequested: body.GPURequested, Message: body.Error}
			switch {
			case body.Ready:
				h.Status = "running"
			case body.Error != "":
				h.Status = "error"
			}
			return h
		}
	}
	if t.DockerEnabled {
		if st, err := mirageInspect(DockerConfig{Host: t.DockerHost}, t.ContainerName); err == nil && st.Running {
			return MirageHealth{Status: "starting"}
		}
	}
	return MirageHealth{Status: "stopped"}
}
```
{% endraw %}

Notes for the engineer:
- `mirageBuildBackoff` reuses `go2RTCBuildBackoff` (see `go2rtc_build_backoff.go`); it is used as a zero-value field on `Go2RTCManager` (`go2rtc.go:152`). Confirm that no constructor initialises it; if it needs one, mirror what the go2rtc manager constructor does.
- If `dockerutil.ManagedLabels` does not return a map, adapt the copy loop to its return type (`managed_resources.go:93`).
- `hostGroupIDs` and `hardenManagedSidecarHostConfig` exist in package `tools` (`ollama_container.go`, `managed_sidecar_security.go:9`).

- [ ] **Step 6: Run to verify pass**

Run: `go test ./internal/tools -run '(?i)(TestMirage|TestEnsureMirage|TestManagedSidecarContainerNames)' -count=1 -v` and count the PASS lines (12 expected: 3 image/GPU, 1 detect, 3 status, 4 ensure, 1 names). Then `go vet ./internal/tools`.

- [ ] **Step 7: Commit**

```bash
git add internal/tools/mirage_sidecar.go internal/tools/mirage_sidecar_test.go internal/tools/managed_sidecar_names.go internal/tools/managed_sidecar_names_test.go internal/dockerutil/managed_resources.go internal/server/containers_protection.go
git commit -m "feat(mirage): managed sidecar with GPU passthrough and fingerprinted recreate" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- internal/tools/mirage_sidecar.go internal/tools/mirage_sidecar_test.go internal/tools/managed_sidecar_names.go internal/tools/managed_sidecar_names_test.go internal/dockerutil/managed_resources.go internal/server/containers_protection.go
```

---

### Task 6: Server wiring (boot, settings save, status endpoint)

**Files:**
- Create: `internal/server/mirage_handlers.go`
- Create: `internal/server/mirage_handlers_test.go`
- Modify: `internal/server/server.go:1790-1793`
- Modify: `internal/server/config_handlers_main.go:1497-1499`
- Modify: `internal/server/server_routes_config.go:318-319`

- [ ] **Step 1: Impact check**

Run GitNexus `impact` upstream on the function that contains `server.go:1790` and on the config-save handler that contains `config_handlers_main.go:1497`. The edits add independent blocks next to the Piper blocks; report the result.

- [ ] **Step 2: Write the failing tests**

Create `internal/server/mirage_handlers_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
)

func TestHandleMirageStatusDisabled(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	handleMirageStatus(s)(rec, httptest.NewRequest(http.MethodGet, "/api/mirage/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["status"] != "disabled" {
		t.Fatalf("body = %s (%v)", rec.Body.String(), err)
	}
}

func TestHandleMirageStatusRejectsOtherMethods(t *testing.T) {
	s := &Server{Cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	handleMirageStatus(s)(rec, httptest.NewRequest(http.MethodPost, "/api/mirage/status", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code = %d, want 405", rec.Code)
	}
}

func TestMirageInputsChanged(t *testing.T) {
	a, b := &config.Config{}, &config.Config{}
	if mirageInputsChanged(a, b) {
		t.Fatal("equal configs must not report a change")
	}
	b.Vision.Mirage.GPU = "nvidia"
	if !mirageInputsChanged(a, b) {
		t.Fatal("a GPU change must be reported")
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/server -run '(?i)(TestHandleMirageStatus|TestMirageInputsChanged)' -count=1`
Expected: build FAIL (`handleMirageStatus undefined`).

- [ ] **Step 4: Implement the handler**

Create `internal/server/mirage_handlers.go`:

```go
package server

import (
	"encoding/json"
	"net/http"

	"aurago/internal/config"
	"aurago/internal/tools"
)

// handleMirageStatus reports the Mirage sidecar state for the Vision config banner.
func handleMirageStatus(s *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")

		s.CfgMu.RLock()
		target := tools.MirageStatusTargetFor(s.Cfg)
		s.CfgMu.RUnlock()

		_ = json.NewEncoder(w).Encode(tools.MirageStatus(target))
	}
}

// mirageInputsChanged reports whether a settings save changed anything the
// sidecar container depends on.
func mirageInputsChanged(oldCfg, newCfg *config.Config) bool {
	return oldCfg.Vision.Mirage != newCfg.Vision.Mirage
}
```

- [ ] **Step 5: Boot, settings save, route**

In `internal/server/server.go`, directly after the Piper block (lines 1790-1793, `if cfg.TTS.Piper.Enabled { go tools.EnsurePiperRunning(cfg, logger) }`) add:

```go
		if cfg.Vision.Mirage.Enabled {
			go func() {
				if err := tools.EnsureMirageRunning(cfg, logger, false); err != nil {
					logger.Warn("[Mirage] Sidecar start failed", "error", err)
				}
			}()
		}
```

In `internal/server/config_handlers_main.go`, directly after the Piper auto-start block (lines 1497-1499) add:

```go
	// Start or reconcile the Mirage vision sidecar when it was just enabled or its settings changed
	if newCfg.Docker.Enabled && newCfg.Vision.Mirage.Enabled && (!oldCfg.Vision.Mirage.Enabled || mirageInputsChanged(&oldCfg, newCfg)) {
		go func() {
			if err := tools.EnsureMirageRunning(newCfg, s.Logger, true); err != nil {
				s.Logger.Warn("[Mirage] Sidecar start failed", "error", err)
			}
		}()
	}
```

(`oldCfg` is a value and `newCfg` a pointer in that function, as in `ollamaEmbeddingsInputsChanged(&oldCfg, newCfg)` at line 1471-1475; adjust the `&` if the local types differ.)

In `internal/server/server_routes_config.go`, next to the registration of `/api/piper/status` (line 318), add a registration of `/api/mirage/status` that uses exactly the same wrapper form as the Piper line, with `handleMirageStatus(s)`.

- [ ] **Step 6: Run to verify pass**

Run: `go test ./internal/server -run '(?i)(TestHandleMirageStatus|TestMirageInputsChanged)' -count=1 -v` (expect 3 PASS), then `go build ./...` and `go vet ./internal/server`.

- [ ] **Step 7: Commit**

```bash
git add internal/server/mirage_handlers.go internal/server/mirage_handlers_test.go internal/server/server.go internal/server/config_handlers_main.go internal/server/server_routes_config.go
git commit -m "feat(mirage): start sidecar on boot/save and expose /api/mirage/status" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- internal/server/mirage_handlers.go internal/server/mirage_handlers_test.go internal/server/server.go internal/server/config_handlers_main.go internal/server/server_routes_config.go
```

---

### Task 7: Config UI status banner and i18n

The Vision section uses the generic renderer, which already renders the nested `mirage` object (toggle for `enabled`, inputs for the rest) because Task 3 gave the fields json tags. This task adds the status banner and its texts.

**Files:**
- Modify: `ui/js/config/main.js` (~2138 and ~2180, plus a new function)
- Modify: `ui/lang/config/generic/{cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh}.json`
- Scratch: `add-mirage-i18n.mjs` in the scratchpad (not committed)

- [ ] **Step 1: Confirm the banner helper is global on the config page**

Run: `grep -rn "cfgStatusBanner" ui/js ui/cfg --include=*.js -l` and find where `cfgStatusBanner` is defined (it is called as `cfgStatusBanner.html(id, state, {detail})`, `cfgStatusBanner(id, state, {detail})` and `cfgStatusBanner.check(id, fetchFn, mapFn, {checkingDetail})` in `ui/cfg/tts.js`). It must be defined in a script that is loaded before sections render on the config page; if it is only defined inside a lazily loaded module, add `await loadModule('<that module>')` before using it in step 2.

- [ ] **Step 2: Banner in the Vision section**

In `ui/js/config/main.js`, in `renderSection`, directly after the line `if (integrationActions) html += integrationActions.render(key);` (~2138) add:

```js
    if (key === 'vision') {
        html += cfgStatusBanner.html('mirage-status-banner', 'checking', { detail: t('config.vision.mirage_checking') });
    }
```

Directly after `if (integrationActions) integrationActions.bind(key);` (~2180) add:

```js
    if (key === 'vision') mirageCheckStatus();
```

Add this function next to `refreshOllamaEmbeddingStatus` (~2236):

```js
function mirageCheckStatus() {
    return cfgStatusBanner.check('mirage-status-banner', () => fetch('/api/mirage/status'), body => {
        if (body.status === 'disabled') return { state: 'off', detail: t('config.vision.mirage_disabled') };
        if (body.status === 'running') return { state: 'connected', detail: t('config.vision.mirage_running') + (body.backend ? ' (' + body.backend + ')' : '') };
        if (body.status === 'starting') return { state: 'degraded', detail: t('config.vision.mirage_starting') };
        return { state: 'error', detail: t('config.vision.mirage_unavailable') };
    }, { checkingDetail: t('config.vision.mirage_checking') });
}
```

- [ ] **Step 3: Add the five keys to all 16 locale files**

Create `add-mirage-i18n.mjs` in the scratchpad with the Write tool. It inserts five lines after the existing `"config.vision.provider_label"` line of each `ui/lang/config/generic/<lang>.json`, keeping the file's indentation and line endings, so the diff stays tiny:

```js
import fs from 'node:fs';
import path from 'node:path';

const dir = process.argv[2]; // ...\ui\lang\config\generic
const T = {
  en: ['Checking Mirage sidecar…', 'Mirage is not enabled', 'Mirage is running', 'Mirage is starting (the model loads on first start)', 'Mirage sidecar is not reachable'],
  de: ['Mirage-Sidecar wird geprüft…', 'Mirage ist nicht aktiviert', 'Mirage läuft', 'Mirage startet (das Modell wird beim ersten Start geladen)', 'Mirage-Sidecar ist nicht erreichbar'],
  es: ['Comprobando el sidecar de Mirage…', 'Mirage no está activado', 'Mirage está en funcionamiento', 'Mirage se está iniciando (el modelo se carga en el primer inicio)', 'No se puede acceder al sidecar de Mirage'],
  fr: ['Vérification du sidecar Mirage…', "Mirage n'est pas activé", "Mirage est en cours d'exécution", 'Mirage démarre (le modèle se charge au premier démarrage)', 'Le sidecar Mirage est injoignable'],
  it: ['Verifica del sidecar Mirage…', 'Mirage non è attivato', 'Mirage è in esecuzione', 'Mirage si sta avviando (il modello viene caricato al primo avvio)', 'Il sidecar Mirage non è raggiungibile'],
  nl: ['Mirage-sidecar wordt gecontroleerd…', 'Mirage is niet ingeschakeld', 'Mirage draait', 'Mirage start op (het model wordt bij de eerste start geladen)', 'Mirage-sidecar is niet bereikbaar'],
  pt: ['A verificar o sidecar do Mirage…', 'O Mirage não está ativado', 'O Mirage está em execução', 'O Mirage está a iniciar (o modelo é carregado no primeiro arranque)', 'O sidecar do Mirage não está acessível'],
  pl: ['Sprawdzanie sidecara Mirage…', 'Mirage nie jest włączony', 'Mirage działa', 'Mirage się uruchamia (model jest ładowany przy pierwszym starcie)', 'Sidecar Mirage jest niedostępny'],
  cs: ['Kontrola sidecaru Mirage…', 'Mirage není zapnutý', 'Mirage běží', 'Mirage se spouští (model se načítá při prvním spuštění)', 'Sidecar Mirage není dostupný'],
  da: ['Kontrollerer Mirage-sidecar…', 'Mirage er ikke aktiveret', 'Mirage kører', 'Mirage starter (modellen indlæses ved første start)', 'Mirage-sidecar kan ikke nås'],
  no: ['Kontrollerer Mirage-sidecar…', 'Mirage er ikke aktivert', 'Mirage kjører', 'Mirage starter (modellen lastes ved første start)', 'Mirage-sidecar er ikke tilgjengelig'],
  sv: ['Kontrollerar Mirage-sidecar…', 'Mirage är inte aktiverat', 'Mirage körs', 'Mirage startar (modellen laddas vid första start)', 'Mirage-sidecar går inte att nå'],
  el: ['Έλεγχος του sidecar Mirage…', 'Το Mirage δεν είναι ενεργοποιημένο', 'Το Mirage λειτουργεί', 'Το Mirage ξεκινά (το μοντέλο φορτώνεται στην πρώτη εκκίνηση)', 'Το sidecar Mirage δεν είναι προσβάσιμο'],
  hi: ['Mirage साइडकार की जाँच हो रही है…', 'Mirage सक्षम नहीं है', 'Mirage चल रहा है', 'Mirage शुरू हो रहा है (मॉडल पहली बार शुरू होने पर लोड होता है)', 'Mirage साइडकार तक पहुँचा नहीं जा सकता'],
  ja: ['Mirage サイドカーを確認しています…', 'Mirage は有効になっていません', 'Mirage は実行中です', 'Mirage を起動しています(モデルは初回起動時に読み込まれます)', 'Mirage サイドカーに接続できません'],
  zh: ['正在检查 Mirage 侧车…', 'Mirage 未启用', 'Mirage 正在运行', 'Mirage 正在启动(首次启动时加载模型)', '无法连接到 Mirage 侧车'],
};
const KEYS = ['config.vision.mirage_checking', 'config.vision.mirage_disabled', 'config.vision.mirage_running', 'config.vision.mirage_starting', 'config.vision.mirage_unavailable'];

for (const [lang, values] of Object.entries(T)) {
  const file = path.join(dir, lang + '.json');
  let text = fs.readFileSync(file, 'utf8');
  if (text.includes('"config.vision.mirage_checking"')) { console.log('skip', lang); continue; }
  const eol = text.includes('\r\n') ? '\r\n' : '\n';
  const m = text.match(/^([ \t]*)"config\.vision\.provider_label":.*$/m);
  if (!m) throw new Error('anchor not found in ' + file);
  const indent = m[1];
  const anchorLine = m[0].trimEnd().endsWith(',') ? m[0] : m[0] + ',';
  const added = KEYS.map((k, i) => indent + JSON.stringify(k) + ': ' + JSON.stringify(values[i]) + ',').join(eol);
  text = text.replace(m[0], anchorLine + eol + added);
  JSON.parse(text.replace(/,(\s*})/, '$1')); // must still parse
  fs.writeFileSync(file, text);
  console.log('ok', lang);
}
```

Run: `node <SCRATCH>\add-mirage-i18n.mjs C:\Users\Andi\Documents\repo\AuraGo\ui\lang\config\generic`
Expected: 16 lines starting with `ok`. If a file's `config.vision.provider_label` entry is the last one (no trailing comma), the script adds a comma to it and the new last line keeps its trailing comma, which would break JSON: in that case remove the trailing comma from the last inserted line by hand and re-validate with `node -e "JSON.parse(require('fs').readFileSync('<file>','utf8'))"`.

- [ ] **Step 4: Run the UI contract tests**

Run: `go test ./ui -run '(?i)(TestTranslations_AllKeysPresentInAllLanguages|TestTranslations_AllJSONFilesParse|TestTranslations_NoNestedStructures|TestFrontend_StaticI18nKeysExistInEnglishBundle|TestFrontend_JS_NoLiteralTranslationFallbacks)' -count=1 -v`
Expected: PASS lines for all five. Then `go test ./ui -run 'TestConfigLayout' -count=1` to catch banner-contract tests (`ui/config_layout_regression_test.go`). If a test requires every `cfgStatusBanner.html` id to be documented or to have a retry contract, follow the Piper example at `ui/config_layout_regression_test.go:182`.

- [ ] **Step 5: Look at it in the running UI**

Start the config page the way the project documents (see the `run` skill / memory `reference_local_scratch_instance_windows`), open the Vision section and confirm: the Mirage group renders with an `enabled` toggle, the banner shows "Mirage is not enabled" (or the localised text), and no console errors appear. If the scratch instance cannot be started, say so in the commit message instead of claiming it was checked.

- [ ] **Step 6: Commit**

```bash
git add ui/js/config/main.js ui/lang/config/generic
git commit -m "feat(mirage): status banner and texts in the Vision config section" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- ui/js/config/main.js ui/lang/config/generic
```

---

### Task 8: Docs, live test, final verification

**Files:**
- Modify: `documentation/configuration.md`, `documentation/manuals/en/07-configuration.md`, `documentation/manuals/de/07-configuration.md`
- Create: `internal/tools/mirage_live_test.go`

- [ ] **Step 1: Docs**

Run `grep -n -i "vision" documentation/configuration.md documentation/manuals/en/07-configuration.md documentation/manuals/de/07-configuration.md` and add, under the existing Vision heading of each file, this block (English for `configuration.md` and `en`, German for `de`):

English:

```markdown
#### Local vision with Mirage (optional)

For systems without a vision-capable model, AuraGo can run the small Cactus **Mirage** model (51M parameters, about 22 MB, Apache-2.0) in a managed Docker sidecar. It is used only while `vision.provider` is empty: in `mode: fallback` when the main model cannot read images, in `mode: always` whenever no provider is set. Mirage returns a text description (scene, objects, text found in the image, relations); the main model answers questions about the image from that text. It cannot answer free-form questions itself and cannot analyze image URLs, only local images.

| Key | Default | Meaning |
| --- | --- | --- |
| `vision.mirage.enabled` | `false` | Build and run the sidecar |
| `vision.mirage.mode` | `fallback` | `fallback` or `always` |
| `vision.mirage.container_name` / `container_port` | `aurago_mirage` / `8791` | Container name, host port on 127.0.0.1 |
| `vision.mirage.image` | `python:3.12-slim` | Python base image of the sidecar build |
| `vision.mirage.data_path` | `data/mirage` | Weights cache (downloaded on first start) |
| `vision.mirage.gpu` | `auto` | `auto`, `none`, `nvidia`, `amd`, `intel`, `vulkan` (detection works on Linux; Windows/macOS run on CPU) |
| `vision.mirage.max_objects` / `max_text_lines` | `20` / `30` | Limits for the description |
| `vision.mirage.language` | empty | Output language (en, de, fr, es, it, nl, pl); empty = engine default |

The Vision page shows the sidecar state and the backend the engine reports. The first start builds the image and downloads the model, which takes a few minutes. Changing a setting recreates the container.
```

German (same table, translated text):

```markdown
#### Lokale Bildanalyse mit Mirage (optional)

Für Systeme ohne bildfähiges Modell kann AuraGo das kleine Cactus-Modell **Mirage** (51M Parameter, etwa 22 MB, Apache-2.0) in einem verwalteten Docker-Sidecar betreiben. Es wird nur genutzt, solange `vision.provider` leer ist: im `mode: fallback`, wenn das Hauptmodell keine Bilder lesen kann, im `mode: always` immer, wenn kein Anbieter gesetzt ist. Mirage liefert eine Textbeschreibung (Szene, Objekte, Text im Bild, Beziehungen); das Hauptmodell beantwortet Fragen zum Bild anhand dieses Textes. Mirage selbst beantwortet keine freien Fragen und analysiert keine Bild-URLs, nur lokale Bilder.

Schlüssel, Standardwerte und Bedeutung: wie in der englischen Tabelle (`vision.mirage.enabled`, `mode`, `container_name`, `container_port`, `image`, `data_path`, `gpu`, `max_objects`, `max_text_lines`, `language`). Die GPU-Erkennung funktioniert unter Linux; Windows und macOS laufen auf der CPU.

Die Vision-Seite zeigt den Zustand des Sidecars und das von der Engine gemeldete Backend. Der erste Start baut das Image und lädt das Modell, das dauert einige Minuten. Eine geänderte Einstellung erstellt den Container neu.
```

If a docs-consistency test pins heading levels or lists of config keys (`go test ./... -run 'Docs|Manual' -count=1`), run it and adjust.

- [ ] **Step 2: Live test against Docker Desktop (skipped unless enabled)**

Create `internal/tools/mirage_live_test.go`. It builds and runs a separate, uniquely named container, calls the real model, and removes only that container:

```go
package tools

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"aurago/internal/config"
)

// Run with: AURAGO_LIVE_MIRAGE=1 go test ./internal/tools -run TestLiveMirageSidecar -count=1 -timeout 30m -v
// Optional: AURAGO_LIVE_DOCKER_HOST (default: the Docker Desktop named pipe).
func TestLiveMirageSidecar(t *testing.T) {
	if os.Getenv("AURAGO_LIVE_MIRAGE") != "1" {
		t.Skip("set AURAGO_LIVE_MIRAGE=1 to run against a real Docker daemon")
	}
	configureDockerSecurityTestPermissions(t, false)
	host := os.Getenv("AURAGO_LIVE_DOCKER_HOST")
	if host == "" {
		host = "npipe:////./pipe/docker_engine"
	}
	cfg := &config.Config{}
	cfg.Docker.Enabled = true
	cfg.Docker.Host = host
	m := &cfg.Vision.Mirage
	m.Enabled = true
	m.Mode = config.MirageModeAlways
	m.ContainerName = "aurago_mirage_livetest"
	m.ContainerPort = 18791
	m.Image = "python:3.12-slim"
	m.DataPath = filepath.Join(t.TempDir(), "mirage")
	m.GPU = "auto"
	m.MaxObjects = 20
	m.MaxTextLines = 30

	prev := mirageBaseURLFor
	mirageBaseURLFor = func(*config.Config) string { return "http://127.0.0.1:" + strconv.Itoa(m.ContainerPort) + "/v1" }
	t.Cleanup(func() {
		mirageBaseURLFor = prev
		_, _, _ = dockerRequest(DockerConfig{Host: host}, http.MethodDelete, "/containers/"+m.ContainerName+"?force=true", "")
	})

	if err := EnsureMirageRunning(cfg, testLogger(t), true); err != nil {
		t.Fatalf("EnsureMirageRunning: %v", err)
	}
	status := MirageStatus(MirageStatusTargetFor(cfg))
	t.Logf("status: %+v", status)
	if !status.Ready {
		t.Fatalf("sidecar not ready: %+v", status)
	}

	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for x := 0; x < 256; x++ {
		for y := 0; y < 256; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	text, _, _, err := AnalyzeTrustedImageFileWithPromptContext(context.Background(), path, "describe", cfg)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	t.Logf("description:\n%s", text)
	if text == "" {
		t.Fatal("empty description")
	}
}
```

Reuse the `testLogger` helper from Task 5. Run (the build and model download take several minutes; per the Docker-sidecar notes in memory this uses the local Docker Desktop and removes only the `aurago_mirage_livetest` container; its built image `aurago/mirage:<hash>` stays as a cache):

```bash
AURAGO_LIVE_MIRAGE=1 go test ./internal/tools -run TestLiveMirageSidecar -count=1 -timeout 30m -v
```

Expected: PASS, with logged `status: {Status:running Ready:true Backend:...}` and a description. Note which `Backend` was reported.

- [ ] **Step 3: Full verification**

Run, and report the real output of each:

```bash
python -m unittest discover -s internal/tools/mirage_sidecar -p "test_*.py" -v
go build ./...
go vet ./internal/config ./internal/tools ./internal/server
go test ./internal/config ./internal/tools ./internal/server ./internal/telegram -count=1
go test ./ui -run '(?i)(TestTranslations|TestFrontend_StaticI18nKeys|TestConfigLayout)' -count=1
```

Expected: all PASS. The `./internal/server` and `./ui` packages are large; if a failure also occurs on a clean checkout of `main`, name it as pre-existing instead of fixing unrelated tests.

- [ ] **Step 4: Graph check before the last commit (project rule)**

Run GitNexus `detect_changes({scope: "compare", base_ref: "main"})` (or the CLI form from CLAUDE.md). It must not be `partial` or `truncated`; re-run if it is. Report the changed symbols and confirm they are limited to the Mirage work.

- [ ] **Step 5: Commit**

```bash
git add documentation/configuration.md documentation/manuals/en/07-configuration.md documentation/manuals/de/07-configuration.md internal/tools/mirage_live_test.go
git commit -m "docs(mirage): configuration docs and opt-in live test" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>" -- documentation/configuration.md documentation/manuals/en/07-configuration.md documentation/manuals/de/07-configuration.md internal/tools/mirage_live_test.go
```

---

## Self-review against the spec

| Spec requirement | Task |
| --- | --- |
| Managed sidecar `aurago_mirage`, built image, weights volume, hardened, reserved name | 1, 2, 5 |
| `/v1/chat/completions` + `/health` | 1 |
| GPU auto via `DetectGPU`, passthrough, backend reported | 5 (host config), 1 (`/health`), 6/7 (status, banner), 2 step 7 (verification) |
| Provider routing, no API key, explicit provider wins, fallback when main cannot see | 4 (see deviation 1) |
| Applies to `analyze_image`, Telegram, chat fallback | 4 (single hook in `analyzeImageReferenceWithPromptContext`; fallback path `analyzeImageForFallback` and Telegram both end there) |
| Text output with limits, sanitised by existing `GateScan`/`IsolateExternalData` | 1 (limits), unchanged `fallbackVisionAnalysis` for the scan |
| Config keys + UI status + 16 languages + manuals | 3, 7, 8 |
| Errors: no Docker, build failure (back-off), not ready, bad image | 4, 5 |
| Tests: Go units, sidecar server tests, live test, i18n lint | 1, 3, 4, 5, 6, 7, 8 |
| Open points 1-3 (GPU in container, Python/packages, download location) | 2 steps 2, 6, 7 (HOME=/data keeps the cache on the volume) |

Known limits to state when reporting completion: Windows/macOS run Mirage on the CPU (no GPU detection there); the `mode` and `gpu` fields render as plain text inputs in the generic renderer; `analyze_image` with a public URL is refused in Mirage mode; Mirage returns no token counts, so vision usage is not budget-tracked.
