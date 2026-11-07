"""Build the original Earshot chamber-score loops (unused masters) and ship the effect sounds.

The runtime room music is the YuE2 song set built by build-songs.mjs. The eight Earshot loops
are written to masters/audio/loops/ only: they are not in the runtime payload or the manifest.

Authoring uses committed Earshot 0.4 instrument takes, numpy, soundfile and FFmpeg.
Runtime needs only the resulting local Ogg/WAV files.
"""
from pathlib import Path
import hashlib
import json
import os
import shutil
import subprocess

for variable in ('OPENBLAS_NUM_THREADS', 'OMP_NUM_THREADS', 'MKL_NUM_THREADS', 'EARSHOT_THREADS'):
    os.environ.setdefault(variable, '3')

import numpy as np
import soundfile as sf

ROOT = Path(__file__).resolve().parent
REPO = ROOT.parents[1]
OUT = REPO / "ui/img/goldfish-adventure/audio"
LOOPS = ROOT / "masters/audio/loops"
LOOPS.mkdir(parents=True, exist_ok=True)
SAMPLES = ROOT / "masters/audio/samples"
SR = 48000
SOURCES = REPO / "internal/gamemaker/asset_packs/aurago-sounds"
OUT.mkdir(parents=True, exist_ok=True)
SAMPLES.mkdir(parents=True, exist_ok=True)
FFMPEG = shutil.which("ffmpeg")
if not FFMPEG:
    raise RuntimeError("FFmpeg is required for authoring")
notes = {}
records = []

def sample(voice, midi):
    key = (voice, midi)
    if key not in notes:
        name = f"{voice}-{midi}.wav"
        path = SAMPLES / name
        if not path.exists():
            raise RuntimeError('Missing authored Earshot instrument take: ' + name)
        value, rate = sf.read(path, dtype="float32", always_2d=True)
        assert rate == SR
        if value.shape[1] == 1:
            value = np.repeat(value, 2, axis=1)
        notes[key] = value[:, :2]
    return notes[key]

def add(buffer, value, seconds, gain=1, pan=0):
    offset = int(seconds * SR) % len(buffer)
    stereo = value * np.array([1 - max(0, pan) * .45, 1 + min(0, pan) * .45]) * gain
    end = min(len(stereo), len(buffer) - offset)
    buffer[offset:offset + end] += stereo[:end]
    if end < len(stereo):
        buffer[:len(stereo) - end] += stereo[end:]

# Original motifs: no franchise melodies, recordings or dialogue.
THEMES = [
    ("second-chances", 96, 55, [0, 2, 3, 7, 5, 2, 0, -2], "industrial"),
    ("paper-moons", 112, 57, [0, 4, 2, 7, 9, 7, 4, 2], None),
    ("borrowed-stars", 76, 55, [7, 5, 3, 2, 0, 2, 5, 7], "space-hum"),
    ("unkept-promises", 104, 60, [0, 3, 7, 6, 5, 2, 3, 0], None),
    ("alma", 72, 53, [0, 2, 5, 7, 9, 7, 5, 2], "forest-day"),
    ("weather-permission", 88, 55, [0, 3, 2, 5, 7, 5, 2, 0], "rain-light"),
    ("a-useful-no", 80, 52, [0, 1, 3, 7, 6, 3, 1, 0], "space-hum"),
    ("the-next-question", 84, 55, [0, 4, 7, 9, 7, 4, 2, 0], None),
]
progression = [0, 3, 8, 5, 0, 3, 5, 7]
for name, bpm, tonic, motif, ambience in THEMES:
    beat = 60 / bpm
    bars = 24
    length = bars * 3 * beat
    mix = np.zeros((round(length * SR), 2), dtype=np.float32)
    for bar in range(bars):
        root = tonic + progression[bar % 8]
        major = name in ("paper-moons", "alma", "the-next-question")
        third = 4 if major else 3
        time = bar * 3 * beat
        # Bass and alternating inner voices leave breathing room for text.
        add(mix, sample("marimba", root), time, .45, -.18)
        for j, degree in enumerate([7, third, 7, third + 12]):
            add(mix, sample("pluck", root + degree),
                time + (j + 1) * .5 * beat, .14 + .025 * (bar % 3), .25 * (-1) ** j)
        # A/B/A form: the middle eight bars answer in a lower, sparse register.
        if bar % 2 == 0:
            phrase = motif if bar < 8 or bar >= 16 else list(reversed(motif))
            for j in range(3):
                degree = phrase[(bar + j) % len(phrase)]
                note = tonic + 12 + degree + (12 if bar >= 16 and j == 2 else 0)
                add(mix, sample("celesta", note), time + (j + .15) * beat,
                    .28 if bar < 8 or bar >= 16 else .19, -.12 + j * .12)
    # Circular, low-level room returns keep loop boundaries continuous.
    dry = mix.copy()
    for delay, level in [(.119, .10), (.181, .075), (.293, .06), (.431, .04), (.613, .025)]:
        mix += np.roll(dry[:, ::-1], int(delay * SR), axis=0) * level
    if ambience:
        path = SOURCES / ("sounds/" + ambience + ".wav")
        bed, rate = sf.read(path, dtype="float32", always_2d=True)
        assert rate == SR and bed.shape[1] == 2
        bed = np.tile(bed, (int(np.ceil(len(mix) / len(bed))) + 1, 1))[:len(mix)]
        # The scene loop need not be a multiple of the ambience's source period.
        edge = round(.03 * SR)
        fade = np.linspace(0, 1, edge, dtype=np.float32)[:, None] ** 2
        bed[:edge] *= fade
        bed[-edge:] *= fade[::-1]
        mix += bed * .032 / max(.02, float(np.max(np.abs(bed))))
    mix -= mix.mean(axis=0)
    peak = float(np.max(np.abs(mix)))
    mix *= 10 ** (-8 / 20) / max(peak, 1e-8)
    master = ROOT / ("masters/audio/" + name + ".wav")
    sf.write(master, mix, SR, subtype="PCM_24")
    target = LOOPS / (name + ".ogg")
    subprocess.run([FFMPEG, "-hide_banner", "-loglevel", "error", "-y", "-i", str(master),
                    "-c:a", "libvorbis", "-q:a", "5", str(target)], check=True)
    print("Composed", name, round(length, 1), "s", flush=True)

# Actual recorded metal impact plus original local cues already owned by AuraGo.
for name in ["door-metal", "water-splash", "ui-click", "electric-spark"]:
    source = SOURCES / ("sounds/" + name + ".wav")
    target = OUT / (name + ".wav")
    shutil.copyfile(source, target)
    records.append(dict(id=name, path="audio/" + target.name,
                        source="AuraGo presentation pack; see audio-notices.txt",
                        sha256=hashlib.sha256(target.read_bytes()).hexdigest()))
shutil.copyfile(ROOT / "audio-notices.txt", OUT / "audio-notices.txt")
manifest_path = ROOT / "audio-manifest.json"
manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
# Keep the room songs written by build-songs.mjs; refresh only the effect entries.
keep = [a for a in manifest["assets"] if a["id"].startswith("song-")]
manifest["assets"] = keep + records
manifest_path.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
                         encoding="utf-8", newline="\n")
print("Built", len(records), "audio assets")
