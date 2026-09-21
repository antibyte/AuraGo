"""Private, receive-only adapters for the pinned SDRangel and welle-cli builds."""
import ctypes
import json
import math
import os
import queue
import struct
import subprocess
import threading
import time
import urllib.request

import websocket


def request(port, path, method="GET", data=None, timeout=3):
    raw = data if isinstance(data, bytes) else json.dumps(data).encode() if data is not None else None
    headers = {"Content-Type": "text/plain" if isinstance(data, bytes) else "application/json"}
    req = urllib.request.Request(f"http://127.0.0.1:{port}{path}", raw, headers, method=method)
    with urllib.request.urlopen(req, timeout=timeout) as response:
        body = response.read(2 * 1024 * 1024)
        return json.loads(body) if body and response.headers.get_content_type() == "application/json" else body


def spawn(args, **kwargs):
    diagnostic = None if os.environ.get("RTL_SDR_DIAGNOSTICS") == "1" else subprocess.DEVNULL
    return subprocess.Popen(args, stdin=subprocess.DEVNULL, stdout=diagnostic,
                            stderr=diagnostic, **kwargs)


def terminate(process):
    if process and process.poll() is None:
        process.terminate()
        try:
            process.wait(4)
        except subprocess.TimeoutExpired:
            process.kill()
            process.wait()


class Hub:
    def __init__(self):
        self.lock = threading.Lock()
        self.clients = set()
        self.dropped = 0

    def subscribe(self):
        client = queue.Queue(100)
        client.dropped = 0
        with self.lock:
            self.clients.add(client)
        return client

    def remove(self, client):
        with self.lock:
            self.clients.discard(client)

    def publish(self, data):
        with self.lock:
            for client in tuple(self.clients):
                try:
                    client.put_nowait(data)
                except queue.Full:
                    # A slow listener never stalls the receiver or a recording.
                    try:
                        client.get_nowait()
                    except queue.Empty:
                        pass
                    client.put_nowait(data)
                    client.dropped += 1
                    self.dropped += 1


class Receiver:
    def __init__(self):
        self.lock = threading.RLock()
        self.children = []
        self.decoder = None
        self.mode = None
        self.tuning = {}
        self.generation = 0
        self.pcm = Hub()
        self.mp3 = Hub()
        self.last_audio = 0
        self.gaps = 0
        self.meta = {"ready": False, "spectrum": [], "gains_db": [], "error": "sdr_stopped"}
        self.index = None
        try:
            self.probe()
        except RuntimeError as error:
            self.meta["error"] = str(error)
        threading.Thread(target=self.encode_live, daemon=True).start()
        threading.Thread(target=self.monitor, daemon=True).start()

    def probe(self):
        lib = ctypes.CDLL("librtlsdr.so.0")
        lib.rtlsdr_get_device_count.restype = ctypes.c_uint32
        count = lib.rtlsdr_get_device_count()
        if count == 0:
            raise RuntimeError("sdr_device_missing")
        serial = os.environ.get("RTL_SDR_SERIAL", "")
        self.index = None
        for index in range(count):
            manufacturer, product, value = [ctypes.create_string_buffer(256) for _ in range(3)]
            lib.rtlsdr_get_device_usb_strings(index, manufacturer, product, value)
            if not serial or value.value.decode(errors="replace") == serial:
                self.index = index
                self.serial = value.value.decode(errors="replace")
                self.product = product.value.decode(errors="replace")
                break
        if self.index is None:
            raise RuntimeError("sdr_device_missing")
        device = ctypes.c_void_p()
        if lib.rtlsdr_open(ctypes.byref(device), self.index) != 0:
            raise RuntimeError("sdr_device_busy_or_permissions")
        try:
            tuner = lib.rtlsdr_get_tuner_type(device)
            names = {1: "E4000", 2: "FC0012", 3: "FC0013", 4: "FC2580", 5: "R820T", 6: "R828D"}
            ranges = {1: (52000000, 2200000000), 2: (22000000, 948000000), 3: (22000000, 1100000000),
                      4: (146000000, 924000000), 5: (24000000, 1766000000), 6: (24000000, 1766000000)}
            low, high = ranges.get(tuner, (24000000, 1766000000))
            if "V4" in self.product:
                low = 500000
            length = lib.rtlsdr_get_tuner_gains(device, None)
            if not 0 <= length <= 100:
                raise RuntimeError("sdr_invalid_tuner")
            gains = (ctypes.c_int * max(length, 1))()
            lib.rtlsdr_get_tuner_gains(device, gains)
            self.meta.update(device=self.product, tuner=names.get(tuner, "Unknown"),
                             minimum_hz=low, maximum_hz=high,
                             gains_db=[gains[i] / 10 for i in range(length)])
        finally:
            lib.rtlsdr_close(device)

    def snapshot(self):
        with self.lock:
            result = dict(self.meta)
            result["ready"] = bool(self.mode and self.children and all(p.poll() is None for p in self.children)
                                   and (self.decoder is None or self.decoder.poll() is None))
            if self.mode and not result["ready"]:
                result["error"] = "sdr_device_lost"
            return result

    def stop(self):
        with self.lock:
            self.generation += 1
            self.mode = None
            self.last_audio = 0
            terminate(self.decoder)
            self.decoder = None
            for child in reversed(self.children):
                terminate(child)
            self.children = []
            self.meta.update(ready=False, spectrum=[], error="sdr_stopped", label="", text="")

    def wait_http(self, port, path, timeout=12):
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            if any(p.poll() is not None for p in self.children):
                raise RuntimeError("sdr_decoder_failed")
            try:
                return request(port, path)
            except (OSError, ValueError):
                time.sleep(0.15)
        raise RuntimeError("sdr_decoder_timeout")

    def tune(self, tuning):
        with self.lock:
            if self.index is None:
                self.probe()
            frequency = int(tuning["frequency_hz"])
            if not self.meta["minimum_hz"] <= frequency <= self.meta["maximum_hz"]:
                raise ValueError("sdr_frequency_unsupported")
            mode = tuning["mode"]
            if mode not in ("wfm", "nfm", "am", "usb", "lsb", "dab"):
                raise ValueError("sdr_invalid_request")
            if mode != "dab" and self.mode == mode and self.snapshot()["ready"]:
                # Knob and digit adjustments retain the decoder and audio sink.
                # A mode change still releases the previous receive chain fully.
                self.configure_analog(tuning)
                self.tuning = dict(tuning)
                self.meta.update(center_hz=frequency, label="", text="", error="")
                return
            same_dab = self.mode == mode == "dab" and all(p.poll() is None for p in self.children)
            if same_dab and any(self.tuning.get(k) != tuning.get(k) for k in ("gain_db", "agc", "ppm")):
                same_dab = False
            if same_dab:
                self.generation += 1
                terminate(self.decoder)
                self.decoder = None
                request(7979, "/channel", "POST", tuning["dab_block"].encode())
            else:
                self.stop()
            self.tuning = dict(tuning)
            self.meta.update(error="", spectrum=[], label="", text="", center_hz=frequency,
                             span_hz=2048000 if mode == "dab" else 2400000)
            try:
                if mode == "dab":
                    self.start_dab(tuning, same_dab)
                else:
                    self.start_analog(tuning)
                self.mode = mode
            except Exception:
                self.stop()
                raise

    def start_analog(self, tuning):
        os.makedirs(os.environ["XDG_RUNTIME_DIR"], mode=0o700, exist_ok=True)
        pulse = spawn(["pulseaudio", "--daemonize=no", "--exit-idle-time=-1", "--log-target=stderr",
                       "-n", "--load=module-native-protocol-unix",
                       "--load=module-null-sink sink_name=receiver rate=48000 channels=2"])
        self.children.append(pulse)
        time.sleep(0.3)
        self.children.append(spawn(["sdrangelsrv", "-a", "127.0.0.1", "-p", "8091"]))
        self.wait_http(8091, "/sdrangel")
        api = lambda path, method="GET", data=None: request(8091, "/sdrangel" + path, method, data)
        outputs = api("/audio").get("outputDevices", [])
        output = next((d for d in outputs if d.get("name") == "receiver"), None)
        if output is None:
            raise RuntimeError("sdr_virtual_audio_unavailable")
        api("/deviceset?direction=0", "POST")
        api("/deviceset/0/device", "PUT", {"hwType": "RTLSDR", "direction": 0, "serial": self.serial})
        self.configure_analog(tuning, create=True)
        api("/deviceset/0/spectrum/settings", "PATCH", {"fftSize": 1024, "fftWindow": 3,
            "fpsPeriodMs": 100, "wsSpectrumAddress": "127.0.0.1", "wsSpectrumPort": 8887})
        api("/deviceset/0/spectrum/server", "POST")
        api("/deviceset/0/device/run", "POST")
        # Read the dedicated null sink, independent of SDRangel's asynchronous
        # FIFO registration. Nothing in this container can alter browser volume.
        self.decoder = subprocess.Popen(["parec", "--device=receiver.monitor", "--raw",
            "--format=s16le", "--rate=48000", "--channels=2", "--latency-msec=20"],
            stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, bufsize=0)
        threading.Thread(target=self.read_pcm, args=(self.decoder, self.generation), daemon=True).start()
        generation = self.generation
        threading.Thread(target=self.spectrum_socket, args=(generation,), daemon=True).start()

    def configure_analog(self, tuning, create=False):
        api = lambda path, method="GET", data=None: request(8091, "/sdrangel" + path, method, data)
        gain = min(self.meta["gains_db"], key=lambda value: abs(value - tuning["gain_db"])) if self.meta["gains_db"] else 0
        api("/deviceset/0/device/settings", "PATCH", {"deviceHwType": "RTLSDR", "direction": 0, "rtlSdrSettings": {
            "centerFrequency": tuning["frequency_hz"], "devSampleRate": 2400000,
            "log2Decim": 0, "fcPos": 2, "gain": round(gain * 10), "agc": int(tuning["agc"]),
            "loPpmCorrection": tuning["ppm"], "dcBlock": 1, "iqImbalance": 1, "biasTee": 0,
            "noModMode": 0, "iqOrder": 1, "rfBandwidth": 2400000}})
        mode = tuning["mode"]
        kind = {"wfm": "BFMDemod", "nfm": "NFMDemod", "am": "AMDemod", "usb": "SSBDemod", "lsb": "SSBDemod"}[mode]
        key = kind + "Settings"
        if create:
            api("/deviceset/0/channel", "POST", {"channelType": kind, "direction": 0})
        settings = {"inputFrequencyOffset": 0, "rfBandwidth": tuning["bandwidth_hz"], "volume": 1.0,
                    "audioMute": 0, "squelch": tuning["squelch_db"], "audioDeviceName": "receiver"}
        if mode == "wfm":
            settings.update(afBandwidth=15000, deEmphasis=0, audioStereo=int(tuning["stereo"]), rdsActive=1)
        elif mode in ("usb", "lsb"):
            settings.update(rfBandwidth=tuning["bandwidth_hz"] * (-1 if mode == "lsb" else 1),
                            lowCutoff=-300 if mode == "lsb" else 300, agc=1)
            settings.pop("squelch")
        api("/deviceset/0/channel/0/settings", "PATCH", {"channelType": kind, "direction": 0, key: settings})

    def start_dab(self, tuning, existing):
        if not existing:
            self.children.append(spawn(["rtl_tcp", "-a", "127.0.0.1", "-p", "1234", "-d", str(self.index),
                                        "-P", str(tuning["ppm"])]))
            # Do not consume rtl_tcp's single client slot for a readiness probe.
            deadline = time.monotonic() + 12
            while time.monotonic() < deadline:
                with open("/proc/net/tcp", encoding="ascii") as sockets:
                    if any("0100007F:04D2" in row and row.split()[3] == "0A" for row in sockets if ":" in row):
                        break
                if self.children[-1].poll() is not None:
                    raise RuntimeError("sdr_device_unavailable")
                time.sleep(0.15)
            else:
                raise RuntimeError("sdr_decoder_timeout")
            self.children.append(spawn(["welle-cli", "-F", "rtl_tcp,127.0.0.1:1234", "-c", tuning["dab_block"],
                "-w", "7979", "-O", "flac", "-g", "-1" if tuning["agc"] else str(round(tuning["gain_db"]))]))
            self.wait_http(7979, "/mux.json")
        sid = tuning.get("service_id", "").lower()
        if not sid:
            return
        sid = f"0x{int(sid, 16):04x}"
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            mux = request(7979, "/mux.json")
            if any(str(service.get("sid", "")).lower() == sid for service in mux.get("services", [])):
                started = time.monotonic()
                self.decoder = subprocess.Popen(["ffmpeg", "-nostdin", "-loglevel", "error", "-i",
                    f"http://127.0.0.1:7979/stream/{sid}", "-f", "s16le", "-ar", "48000", "-ac", "2", "pipe:1"],
                    stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, bufsize=0)
                threading.Thread(target=self.read_pcm, args=(self.decoder, self.generation), daemon=True).start()
                # FIC services appear before welle's two-second handler pass.
                # Retry transient HTTP 503 responses until actual PCM arrives.
                while time.monotonic() < deadline and self.decoder.poll() is None:
                    if self.last_audio > started:
                        return
                    time.sleep(0.1)
                terminate(self.decoder)
            time.sleep(0.3)
        raise RuntimeError("sdr_dab_service_missing")

    def read_pcm(self, process, generation):
        pending = b""
        while generation == self.generation:
            data = process.stdout.read(4096)
            if not data:
                break
            pending += data
            length = len(pending) // 4 * 4
            if length:
                now = time.monotonic()
                if self.last_audio and now - self.last_audio > 0.25:
                    self.gaps += 1
                self.last_audio = now
                self.pcm.publish(pending[:length])
                pending = pending[length:]

    def spectrum_socket(self, generation):
        ws = None
        try:
            ws = websocket.create_connection("ws://127.0.0.1:8887", timeout=3)
            try:
                while generation == self.generation:
                    data = ws.recv()
                    if not isinstance(data, bytes) or len(data) < 36:
                        continue
                    center, _, _, size, span, flags = struct.unpack_from("<qqqIII", data)
                    if not 16 <= size <= 16384 or len(data) != 36 + size * 4:
                        continue
                    values = struct.unpack_from(f"<{size}f", data, 36)
                    values = [10 * math.log10(max(v, 1e-14)) if flags & 1 else v for v in values]
                    self.meta.update(spectrum=[max(-140, min(20, v)) if math.isfinite(v) else -140 for v in values[::max(1, size//1024)]],
                                     center_hz=center, span_hz=span)
            finally:
                ws.close()
        except (OSError, websocket.WebSocketException):
            pass

    def dab_stations(self):
        mux = request(7979, "/mux.json")
        stations = []
        for service in mux.get("services", []):
            if not any(c.get("transportmode") == "audio" for c in service.get("components", [])):
                continue
            sid = str(service["sid"])
            label = str(service.get("label", {}).get("label", sid))[:120]
            tuning = dict(self.tuning, service_id=sid, label=label)
            stations.append({"id": self.tuning["dab_block"] + ":" + sid, "name": label, "tuning": tuning})
            if sid == self.tuning.get("service_id"):
                self.meta.update(label=label, text=str(service.get("dls", {}).get("label", ""))[:512])
        self.meta["snr_db"] = mux.get("demodulator", {}).get("snr", 0)
        return stations

    def monitor(self):
        while True:
            time.sleep(0.25)
            try:
                mode, generation = self.mode, self.generation
                if mode == "dab":
                    self.dab_stations()
                    raw = request(7979, "/spectrum")
                    if raw and len(raw) % 4 == 0 and len(raw) <= 65536:
                        values = struct.unpack(f"<{len(raw)//4}f", raw)
                        self.meta["spectrum"] = [max(-140, min(20, 20 * math.log10(max(abs(v) / 2048, 1e-7)))) for v in values[::2]]
                elif mode:
                    report = request(8091, "/sdrangel/deviceset/0/channel/0/report")
                    value = next((v for k, v in report.items() if k.endswith("Report") and isinstance(v, dict)), {})
                    if generation == self.generation:
                        rds = value.get("rdsReport") or {}
                        self.meta.update(power_db=value.get("channelPowerDB", -100), stereo=bool(value.get("pilotLocked")),
                                         label=str(rds.get("progServiceName", ""))[:120], text=str(rds.get("radioText", ""))[:512])
            except (OSError, ValueError, TypeError, AttributeError):
                pass

    def encode_live(self):
        while True:
            client = self.pcm.subscribe()
            process = subprocess.Popen(["ffmpeg", "-nostdin", "-loglevel", "error", "-f", "s16le", "-ar", "48000", "-ac", "2",
                "-i", "pipe:0", "-c:a", "libmp3lame", "-b:a", "192k", "-flush_packets", "1", "-f", "mp3", "pipe:1"],
                stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, bufsize=0)
            def feed(process=process, client=client):
                try:
                    while process.poll() is None:
                        try:
                            data = client.get(timeout=1)
                        except queue.Empty:
                            continue
                        process.stdin.write(data)
                except (OSError, ValueError):
                    pass
            threading.Thread(target=feed, daemon=True).start()
            try:
                while True:
                    data = process.stdout.read(4096)
                    if not data:
                        break
                    self.mp3.publish(data)
            finally:
                self.pcm.remove(client)
                terminate(process)
            time.sleep(1)
