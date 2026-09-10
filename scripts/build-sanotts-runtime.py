"""Build the pinned, small sanoTTS wheel; only developers download the source archive."""
import hashlib
from pathlib import Path
import subprocess
import sys
import urllib.request
import zipfile

REVISION = "de3f71a8603ee979a74e6f2a7592a0ce795d608b"
ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / "disposable" / "sanotts-build"
OUT = ROOT / "internal" / "sanotts" / "runtime"

WORK.mkdir(parents=True, exist_ok=True)
OUT.mkdir(parents=True, exist_ok=True)
urllib.request.urlretrieve(f"https://raw.githubusercontent.com/Ampixa/sanoTTS/{REVISION}/LICENSE.MIT", OUT / "LICENSE.MIT")
archive = WORK / f"{REVISION}.zip"
if not archive.exists():
    urllib.request.urlretrieve(f"https://github.com/Ampixa/sanoTTS/archive/{REVISION}.zip", archive)
prefix = f"sanoTTS-{REVISION}/pypkg/"
with zipfile.ZipFile(archive) as source:
    for item in source.infolist():
        if not item.filename.startswith(prefix) or item.is_dir():
            continue
        target = (WORK / "source" / item.filename[len(prefix):]).resolve()
        if not target.is_relative_to((WORK / "source").resolve()):
            raise ValueError("unsafe source archive path")
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(source.read(item))
subprocess.run([sys.executable, "-m", "pip", "wheel", "--no-deps", "--wheel-dir", str(OUT),
                str(WORK / "source")], check=True)
wheel = OUT / "sanotts-0.6.0-py3-none-any.whl"
print(f"{hashlib.sha256(wheel.read_bytes()).hexdigest()}  {wheel.name}")
