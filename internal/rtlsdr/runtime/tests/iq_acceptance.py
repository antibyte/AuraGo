"""Decode deterministic, locally generated radio signals, without any RF hardware.

Run Dockerfile.fixtures with --network none and without --device. ODR programs
only generate files. SDRangel FileInput and welle raw-file input exercise the
same demodulators, virtual audio sink, metadata parser and PCM fan-out as USB.
"""
import json
import os
import pathlib
import queue
import struct
import subprocess
import sys
import time
import wave
import zlib

import numpy as np

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1]))
import receiver

ROOT = pathlib.Path("/tmp/rtl-sdr-fixtures")
ROOT.mkdir(parents=True, exist_ok=True)
os.makedirs(os.environ["XDG_RUNTIME_DIR"], mode=0o700, exist_ok=True)
REAL_REQUEST = receiver.request


def command(args):
    with (ROOT / "generation.log").open("ab") as log:
        subprocess.run(args, check=True, stdout=log, stderr=log, timeout=120)


def analog_file(mode):
    rate = 2400000
    path = ROOT / "analog.sdriq"
    header = struct.pack("<IQQII", rate, 100000000, 1700000000000, 16, 0)
    with path.open("wb") as out:
        out.write(header + struct.pack("<I", zlib.crc32(header)))
        for second in range(6):
            t = np.arange(rate, dtype=np.float64) / rate + second
            tone = np.sin(2 * np.pi * 1000 * t)
            if mode == "wfm":
                left, right = tone, np.sin(2 * np.pi * 1700 * t)
                composite = .40 * (left + right) / 2 + .40 * (left - right) / 2 * np.sin(2 * np.pi * 38000 * t) + .10 * np.cos(2 * np.pi * 19000 * t)
                phase = np.cumsum(composite) * 2 * np.pi * 75000 / rate
                samples = .55 * np.exp(1j * phase)
            elif mode == "nfm":
                samples = .55 * np.exp(1j * np.cumsum(tone) * 2 * np.pi * 2500 / rate)
            elif mode == "am":
                samples = .45 * (1 + .5 * tone) + np.zeros(rate) * 1j
            else:
                samples = .55 * np.exp((1 if mode == "usb" else -1) * 1j * 2 * np.pi * 1000 * t)
            iq = np.empty(rate * 2, dtype="<i2")
            iq[0::2], iq[1::2] = (samples.real * 32700).astype("<i2"), (samples.imag * 32700).astype("<i2")
            out.write(iq.tobytes())
    return path


def file_request(port, path, method="GET", data=None, timeout=3):
    if port == 8091 and path == "/sdrangel/deviceset/0/device" and method == "PUT":
        data = {"hwType": "FileInput", "direction": 0}
    if port == 8091 and path == "/sdrangel/deviceset/0/device/settings" and method == "PATCH":
        data = {"deviceHwType": "FileInput", "direction": 0,
                "fileInputSettings": {"fileName": str(ROOT / "analog.sdriq"), "loop": 1, "accelerationFactor": 1}}
    return REAL_REQUEST(port, path, method, data, timeout)


class FixtureReceiver(receiver.Receiver):
    def probe(self):
        self.index, self.serial = 0, "fixture"
        self.meta.update(device="Synthetic IQ file", tuner="FileInput", minimum_hz=100000,
                         maximum_hz=2200000000, gains_db=[0])

    def start_dab(self, tuning, existing):
        if not existing:
            self.children.append(receiver.spawn(["welle-cli", "-f", str(ROOT / "dab.iq"), "-w", "7979", "-O", "flac"]))
            self.wait_http(7979, "/mux.json")
        super().start_dab(tuning, True)


def pcm(radio, seconds=3):
    subscription = radio.pcm.subscribe()
    end, chunks, size = time.monotonic() + 18, [], 0
    try:
        while time.monotonic() < end and size < seconds * 48000 * 4:
            try:
                data = subscription.get(timeout=1)
            except queue.Empty:
                continue
            chunks.append(data)
            size += len(data)
    finally:
        radio.pcm.remove(subscription)
    if size < seconds * 48000 * 4:
        details = {"mode": radio.mode, "bytes":size}
        if radio.mode != "dab":
            details["audio"] = REAL_REQUEST(8091,"/sdrangel/audio")
            details["channel"] = REAL_REQUEST(8091,"/sdrangel/deviceset/0/channel/0/settings")
        else:
            details["decoder_exit"] = radio.decoder.poll() if radio.decoder else None
            details["mux"] = REAL_REQUEST(7979,"/mux.json")
        raise AssertionError(f"Missing PCM: {details}")
    samples = np.frombuffer(b"".join(chunks), dtype="<i2").reshape(-1, 2).astype(float) / 32768
    samples = samples[-48000:]
    rms = float(np.sqrt(np.mean(samples ** 2)))
    assert rms > .0001, f"Silent decoded audio: {rms}"
    peaks = []
    for channel in (0, 1):
        spectrum = np.abs(np.fft.rfft(samples[:, channel] * np.hanning(len(samples))))
        spectrum[:20] = 0
        peaks.append(int(np.argmax(spectrum)))
    return {"rms": round(rms, 5), "peaks_hz": peaks, "difference_rms": float(np.sqrt(np.mean((samples[:,0]-samples[:,1])**2)))}


def dab_file():
    audio = (.25 * np.sin(2 * np.pi * 1000 * np.arange(48000 * 24) / 48000) * 32767).astype("<i2")
    with wave.open(str(ROOT / "source.wav"), "wb") as out:
        out.setparams((2, 2, 48000, 0, "NONE", "not compressed"))
        out.writeframes(np.repeat(audio, 2).tobytes())
    command(["odr-audioenc", "-i", str(ROOT / "source.wav"), "-b", "96", "--aaclc", "-o", str(ROOT / "audio.dabp")])
    (ROOT / "fixture.mux").write_text(f'''
general {{ dabmode 1
 nbframes 1000
 tist false
}}
remotecontrol {{ telnetport 0 }}
ensemble {{ id 0xdfff
 ecc 0xe0
 local-time-offset 0
 label "AuraGo Test"
 shortlabel "Test"
}}
services {{ news {{ id 0xd210
 label "AuraGo Fixture"
 shortlabel "Fixture"
}} }}
subchannels {{ audio {{ type dabplus
 bitrate 96
 id 0
 protection 3
 inputfile "{ROOT / 'audio.dabp'}"
}} }}
components {{ main {{ service news
 subchannel audio
}} }}
outputs {{ file "file://{ROOT / 'fixture.eti'}?type=raw" }}
''')
    command(["odr-dabmux", str(ROOT / "fixture.mux")])
    (ROOT / "fixture.ini").write_text(f'''
[input]
transport=file
source={ROOT / 'fixture.eti'}
loop=0
[modulator]
gainmode=var
digital_gain=1
rate=2048000
mode=1
[output]
output=file
[fileoutput]
filename={ROOT / 'dab.iq'}
format=u8
''')
    command(["odr-dabmod", "-C", str(ROOT / "fixture.ini")])


def main():
    receiver.request = file_request
    radio = FixtureReceiver()
    tuning = dict(frequency_hz=100000000, bandwidth_hz=180000, gain_db=0,
                  agc=True, ppm=0, squelch_db=-100, stereo=True)
    try:
        for mode, width in [("wfm", 180000), ("nfm", 12500), ("am", 9000), ("usb", 2800), ("lsb", 2800)]:
            if os.environ.get("FIXTURE_DAB_ONLY") == "1":
                continue
            analog_file(mode)
            radio.tune(dict(tuning, mode=mode, bandwidth_hz=width))
            audio = pcm(radio)
            processes = [child.pid for child in radio.children] + [radio.decoder.pid]
            radio.tune(dict(tuning, mode=mode, bandwidth_hz=width, gain_db=1))
            assert processes == [child.pid for child in radio.children] + [radio.decoder.pid], "Retune restarted the decoder"
            assert any(abs(p - 1000) < 5 for p in audio["peaks_hz"]), (mode, audio)
            assert len(radio.snapshot().get("spectrum", [])) >= 512, "No spectrum from SDRangel"
            if mode == "wfm":
                assert radio.snapshot().get("stereo"), "Stereo pilot did not lock"
                assert abs(audio["peaks_hz"][0] - audio["peaks_hz"][1]) > 600, f"Stereo channels not separated: {audio}"
            print(json.dumps({"mode": mode, "audio": audio, "spectrum": True, "continuous_retuning": True}), flush=True)
            radio.stop()
        dab_file()
        radio.tune(dict(tuning, mode="dab", frequency_hz=178352000, bandwidth_hz=1536000,
                        dab_block="5C", service_id="0xd210"))
        audio = pcm(radio)
        stations = radio.dab_stations()
        assert any(s["name"].strip() == "AuraGo Fixture" for s in stations), stations
        assert all(abs(p - 1000) < 5 for p in audio["peaks_hz"]), audio
        assert len(radio.snapshot().get("spectrum", [])) == 1024, "Missing DAB spectrum"
        print(json.dumps({"mode": "dab+", "audio": audio, "stations": len(stations), "spectrum": True}), flush=True)
    finally:
        radio.stop()


if __name__ == "__main__":
    main()
