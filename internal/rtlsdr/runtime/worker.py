"""AuraGo's private HTTP-over-Unix-socket receiver worker. No public listeners."""
import argparse
import http.server
import io
import json
import os
import queue
import re
import signal
import socketserver
import subprocess
import threading
import time
import urllib.parse
import wave

from receiver import Receiver, terminate

BLOCKS = dict(zip(
    [f"{n}{c}" for n in range(5, 13) for c in "ABCD"] + [f"13{c}" for c in "ABCDEF"],
    [174928,176640,178352,180064,181936,183648,185360,187072,188928,190640,192352,194064,
     195936,197648,199360,201072,202928,204640,206352,208064,209936,211648,213360,215072,
     216928,218640,220352,222064,223936,225648,227360,229072,230784,232496,234208,235776,237488,239200]))


class Server(socketserver.ThreadingMixIn, socketserver.UnixStreamServer):
    daemon_threads = True
    allow_reuse_address = True


class Handler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, *_):
        pass

    @property
    def receiver(self):
        return self.server.receiver

    def reply(self, status, value):
        body = json.dumps(value, allow_nan=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def begin_stream(self, content_type, trailers=False):
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Transfer-Encoding", "chunked")
        if trailers:
            self.send_header("Trailer", "X-Capture-Status, X-Capture-Seconds, X-Capture-Gaps")
        self.end_headers()

    def chunk(self, data):
        self.wfile.write(f"{len(data):x}\r\n".encode() + data + b"\r\n")
        self.wfile.flush()

    def do_GET(self):
        try:
            url = urllib.parse.urlsplit(self.path)
            params = urllib.parse.parse_qs(url.query)
            if url.path == "/state":
                self.reply(200, self.receiver.snapshot())
            elif url.path == "/audio":
                client = self.receiver.mp3.subscribe()
                self.begin_stream("audio/mpeg")
                try:
                    while True:
                        self.chunk(client.get(timeout=15))
                finally:
                    self.receiver.mp3.remove(client)
            elif url.path == "/capture":
                seconds = int(params.get("seconds", [0])[0])
                if not 5 <= seconds <= 7200 or not self.receiver.snapshot()["ready"]:
                    self.reply(409, {"error": "sdr_not_receiving"})
                    return
                self.capture(seconds)
            elif url.path == "/wav":
                ident = params.get("id", [""])[0]
                offset = float(params.get("offset", [0])[0])
                seconds = int(params.get("seconds", [0])[0])
                if not re.fullmatch(r"[0-9a-f]{32}", ident) or not 0 <= offset <= 7200 or not 1 <= seconds <= 60:
                    self.reply(400, {"error": "sdr_invalid_request"})
                    return
                # Filenames are application-owned IDs, never caller-supplied paths.
                result = subprocess.run(["ffmpeg", "-nostdin", "-loglevel", "error", "-ss", str(offset),
                    "-i", f"/data/{ident}.flac", "-t", str(seconds), "-ac", "1", "-ar", "16000",
                    "-c:a", "pcm_s16le", "-f", "s16le", "pipe:1"], capture_output=True, timeout=30, check=True)
                output = io.BytesIO()
                with wave.open(output, "wb") as audio:
                    audio.setnchannels(1)
                    audio.setsampwidth(2)
                    audio.setframerate(16000)
                    audio.writeframes(result.stdout)
                wav = output.getvalue()
                self.send_response(200)
                self.send_header("Content-Type", "audio/wav")
                self.send_header("Content-Length", str(len(wav)))
                self.end_headers()
                self.wfile.write(wav)
            elif url.path == "/recording-info":
                ident = params.get("id", [""])[0]
                if not re.fullmatch(r"[0-9a-f]{32}", ident):
                    self.reply(400, {"error": "sdr_invalid_request"})
                    return
                result = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "frame=nb_samples",
                    "-of", "csv=p=0", f"/data/{ident}.flac"], capture_output=True, timeout=120)
                samples = sum(int(line) for line in result.stdout.splitlines() if line.isdigit())
                self.reply(200, {"seconds": samples / 48000})
            elif url.path == "/scan":
                self.server.scan_cancel.set()
                cancelled = threading.Event()
                self.server.scan_cancel = cancelled
                self.begin_stream("application/x-ndjson")
                for block, frequency in BLOCKS.items():
                    with self.server.control:
                        if cancelled.is_set():
                            break
                        self.receiver.tune({"mode": "dab", "frequency_hz": frequency * 1000, "bandwidth_hz": 1536000,
                            "dab_block": block, "service_id": "", "agc": True, "gain_db": 0, "ppm": 0, "squelch_db": -100, "stereo": True})
                    stations = []
                    for _ in range(20):
                        if cancelled.wait(0.5):
                            break
                        stations = self.receiver.dab_stations()
                        # Writing progress also detects cancellation before the next retune.
                        self.chunk(json.dumps({"block": block, "stations": stations}).encode() + b"\n")
                        if stations:
                            time.sleep(1)
                            stations = self.receiver.dab_stations()
                            self.chunk(json.dumps({"block": block, "stations": stations}).encode() + b"\n")
                            break
                self.chunk(json.dumps({"complete": not cancelled.is_set()}).encode() + b"\n")
                self.wfile.write(b"0\r\n\r\n")
            else:
                self.reply(404, {"error": "sdr_not_found"})
        except (OSError, queue.Empty):
            self.close_connection = True
        except Exception:
            if os.environ.get("RTL_SDR_DIAGNOSTICS") == "1":
                import traceback
                traceback.print_exc()
            if url.path == "/scan":
                try:
                    self.chunk(b'{"error":"sdr_scan_failed"}\n')
                    self.wfile.write(b"0\r\n\r\n")
                except OSError:
                    pass
            self.close_connection = True

    def do_POST(self):
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if not 0 <= length <= 65536:
                self.reply(400, {"error": "sdr_invalid_request"})
                return
            body = json.loads(self.rfile.read(length)) if length else {}
            if self.path == "/tune":
                self.server.scan_cancel.set()
                with self.server.control:
                    self.receiver.tune(body)
            elif self.path == "/stop":
                self.server.scan_cancel.set()
                with self.server.control:
                    self.receiver.stop()
            else:
                self.reply(404, {"error": "sdr_not_found"})
                return
            self.reply(200, {"status": "ok"})
        except Exception as error:
            if os.environ.get("RTL_SDR_DIAGNOSTICS") == "1":
                import traceback
                traceback.print_exc()
            code = str(error)
            if not re.fullmatch(r"sdr_[a-z_]+", code):
                code = "sdr_decoder_failed"
            self.reply(503, {"error": code})

    def capture(self, duration):
        client = self.receiver.pcm.subscribe()
        stopped = threading.Event()
        info = {"bytes": 0, "status": "partial"}
        initial_gaps = self.receiver.gaps
        process = subprocess.Popen(["ffmpeg", "-nostdin", "-loglevel", "error", "-f", "s16le", "-ar", "48000", "-ac", "2",
            "-i", "pipe:0", "-c:a", "flac", "-f", "flac", "pipe:1"], stdin=subprocess.PIPE,
            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, bufsize=0)

        def feed():
            try:
                remaining = duration * 48000 * 4
                while remaining > 0 and not stopped.is_set():
                    data = client.get(timeout=5)
                    data = data[:remaining]
                    process.stdin.write(data)
                    info["bytes"] += len(data)
                    remaining -= len(data)
                if remaining == 0:
                    info["status"] = "complete"
            except (OSError, queue.Empty):
                pass
            finally:
                try:
                    process.stdin.close()
                except OSError:
                    pass

        threading.Thread(target=feed, daemon=True).start()
        self.begin_stream("audio/flac", trailers=True)
        try:
            while True:
                data = process.stdout.read(16384)
                if not data:
                    break
                self.chunk(data)
            if process.wait(5) != 0:
                info["status"] = "partial"
            seconds = info["bytes"] / (48000 * 4)
            gaps = self.receiver.gaps + client.dropped - initial_gaps
            self.wfile.write((f"0\r\nX-Capture-Status: {info['status']}\r\n"
                f"X-Capture-Seconds: {seconds:.3f}\r\nX-Capture-Gaps: {gaps}\r\n\r\n").encode())
        finally:
            stopped.set()
            self.receiver.pcm.remove(client)
            terminate(process)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--socket", default="/run/aurago-rtlsdr/worker.sock")
    args = parser.parse_args()
    os.umask(0o077)
    os.makedirs(os.environ.get("HOME", "/tmp/rtl-sdr"), exist_ok=True)
    os.makedirs(os.path.dirname(args.socket), mode=0o700, exist_ok=True)
    if os.path.exists(args.socket):
        os.unlink(args.socket)
    receiver = Receiver()
    server = Server(args.socket, Handler)
    server.receiver = receiver
    server.control = threading.Lock()
    server.scan_cancel = threading.Event()
    def stop(*_):
        receiver.stop()
        os._exit(0)
    signal.signal(signal.SIGTERM, stop)
    signal.signal(signal.SIGINT, stop)
    server.serve_forever(poll_interval=0.2)


if __name__ == "__main__":
    main()
