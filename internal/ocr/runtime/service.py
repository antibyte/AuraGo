"""Private Indra adapter. Each file runs in a disposable, supervised process."""
import hashlib
import hmac
import json
import os
import signal
import subprocess
import sys
import tempfile
import threading
import time
from collections import deque
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlsplit

VERSION = "1.1.2"
REVISION = "bd02167fdf2005a19962cc15188f8496a2c87d3b"
MAX_BYTES = 32 << 20
MAX_RESULT = 16 << 20
TOKEN = os.environ.get("OCR_SERVICE_TOKEN", "")
MIMES = {"application/pdf": ".pdf", "image/png": ".png", "image/jpeg": ".jpg",
         "image/tiff": ".tiff", "image/webp": ".webp", "image/bmp": ".bmp"}
workers = {}
lock = threading.Lock()
slot = threading.BoundedSemaphore(1)
cancelled = deque(maxlen=256)


def kill(process):
    if process.poll() is None:
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass


def verify():
    with open("/models/manifest.json", encoding="utf-8") as source:
        manifest = json.load(source)
    if manifest["revision"] != REVISION:
        raise ValueError("model revision mismatch")
    for item in manifest["files"]:
        path = "/models/" + item["name"]
        if os.path.getsize(path) != item["size"]:
            raise ValueError("model size mismatch")
        digest = hashlib.sha256()
        with open(path, "rb") as source:
            for chunk in iter(lambda: source.read(1 << 20), b""):
                digest.update(chunk)
        if digest.hexdigest() != item["sha256"]:
            raise ValueError("model checksum mismatch")
    import indra_ocr
    indra_ocr.load(path="/models", threads=1)
    if indra_ocr.__version__ != VERSION:
        raise ValueError("package version mismatch")


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_args):
        pass  # Documents and credentials never enter server logs.

    def authorized(self):
        return bool(TOKEN) and hmac.compare_digest(
            self.headers.get("Authorization", ""), "Bearer " + TOKEN)

    def answer(self, code, payload):
        body = json.dumps(payload).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if not self.authorized():
            return self.answer(401, {"error": "unauthorized"})
        if self.path != "/healthz":
            return self.answer(404, {"error": "not_found"})
        self.answer(200, {"Ready": True, "Version": VERSION, "ModelRevision": REVISION})

    def do_DELETE(self):
        if not self.authorized():
            return self.answer(401, {"error": "unauthorized"})
        if not self.path.startswith("/v1/ocr/"):
            return self.answer(404, {"error": "not_found"})
        key = self.path.rsplit("/", 1)[-1]
        if len(key) != 32 or any(c not in "0123456789abcdef" for c in key):
            return self.answer(400, {"error": "invalid_id"})
        with lock:
            cancelled.append((key, time.monotonic() + 330))
            entry = workers.get(key)
            if entry:
                entry["cancelled"] = True
                if entry["process"]:
                    kill(entry["process"])
        # Acknowledgement also releases the single worker slot for the next file.
        if entry and not entry["done"].wait(3):
            return self.answer(503, {"error": "cancel_pending"})
        self.answer(200, {"cancelled": True})

    def do_POST(self):
        self.close_connection = True
        if not self.authorized():
            return self.answer(401, {"error": "unauthorized"})
        parsed = urlsplit(self.path)
        args = parse_qs(parsed.query)
        mode, key = args.get("mode", ["auto"])[0], args.get("id", [""])[0]
        mime = self.headers.get("Content-Type", "")
        try:
            length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            return self.answer(400, {"error": "invalid_length"})
        if (parsed.path != "/v1/ocr" or mode not in ("auto", "ocr")
                or len(key) != 32 or any(c not in "0123456789abcdef" for c in key)
                or mime not in MIMES or not 0 < length <= MAX_BYTES):
            return self.answer(400, {"error": "invalid_input"})
        entry = {"process": None, "cancelled": False, "done": threading.Event()}
        with lock:
            if any(old == key and expiry > time.monotonic() for old, expiry in cancelled) or not slot.acquire(blocking=False):
                entry = None
            else:
                workers[key] = entry
        if entry is None:
            return self.answer(409, {"error": "busy_or_cancelled"})
        process = timer = None
        try:
            self.connection.settimeout(30)
            with tempfile.NamedTemporaryFile(suffix=MIMES[mime]) as source:
                remaining = length
                while remaining:
                    chunk = self.rfile.read(min(remaining, 1 << 20))
                    if not chunk:
                        return self.answer(400, {"error": "incomplete_upload"})
                    source.write(chunk)
                    remaining -= len(chunk)
                source.flush()
                with lock:
                    if entry["cancelled"]:
                        raise ValueError("cancelled")
                    process = subprocess.Popen(
                        [sys.executable, "-I", os.path.join(os.path.dirname(__file__), "worker.py"), source.name, mode],
                        stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                        start_new_session=True,
                        env={key: value for key, value in os.environ.items() if key != "OCR_SERVICE_TOKEN"})
                    entry["process"] = process
                timer = threading.Timer(300, kill, args=(process,))
                timer.start()
                self.send_response(200)
                self.send_header("Content-Type", "application/x-ndjson")
                self.send_header("Connection", "close")
                self.end_headers()
                size = 0
                while True:
                    line = process.stdout.readline(MAX_RESULT + 1)
                    if not line:
                        break
                    size += len(line)
                    if size > MAX_RESULT or not line.endswith(b"\n"):
                        raise ValueError("result_limit")
                    try:
                        event = json.loads(line)
                        if event.get("type") not in ("page", "done", "error"):
                            raise ValueError("worker_protocol")
                    except (ValueError, AttributeError):
                        self.wfile.write(b'{"type":"error","code":"worker_protocol_failed"}\n')
                        break
                    self.wfile.write(line)
                    self.wfile.flush()
                if process.wait(timeout=2) != 0:
                    self.wfile.write(b'{"type":"error","code":"processing_failed"}\n')
        except (OSError, ValueError, subprocess.TimeoutExpired):
            pass
        finally:
            if timer:
                timer.cancel()
            if process:
                kill(process)
                process.wait()
                process.stdout.close()
            with lock:
                workers.pop(key, None)
            slot.release()
            entry["done"].set()


if __name__ == "__main__":
    if sys.argv[1:] == ["--verify"]:
        verify()
        raise SystemExit(0)
    if not TOKEN:
        raise SystemExit("OCR service token required")
    subprocess.run([sys.executable, "-I", __file__, "--verify"], check=True,
                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                   env={key: value for key, value in os.environ.items() if key != "OCR_SERVICE_TOKEN"})
    ThreadingHTTPServer(("0.0.0.0", 8766), Handler).serve_forever()
