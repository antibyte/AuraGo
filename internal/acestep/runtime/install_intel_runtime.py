"""Install the pinned Intel userspace driver during image build only."""
import hashlib
import os
from pathlib import Path
import subprocess
import tempfile
from urllib.request import urlopen

PACKAGES = [
    ("oneapi-src/level-zero", "v1.27.0", "level-zero_1.27.0+u24.04_amd64.deb", "0a0fd1dbc06dcce4d6118ca0086a14978efb3ce20c8b76908fb98d21ebbad591"),
    ("intel/intel-graphics-compiler", "v2.28.4", "intel-igc-core-2_2.28.4+20760_amd64.deb", "3eea502b74ca57d6050e259838a91f5384805b5bb73c9fcecc055c6f8d32389f"),
    ("intel/intel-graphics-compiler", "v2.28.4", "intel-igc-opencl-2_2.28.4+20760_amd64.deb", "9fae8175c95def354534e6d322dd1b2661eb92dec96916f50ee1f5d31c7a4f65"),
    ("intel/compute-runtime", "26.05.37020.3", "libigdgmm12_22.9.0_amd64.deb", "9d712f71c18baee076de9961dda71e8089291e1bd0deb5d649ab5ba5de114f97"),
    ("intel/compute-runtime", "26.05.37020.3", "libze-intel-gpu1_26.05.37020.3-0_amd64.deb", "59dc363ee5afcb827def9c6e3b52211a41663a991a5e414a7a0c520dc5ba7c20"),
]

if __name__ == "__main__" and os.environ.get("AURAGO_BACKEND") == "xpu":
    with tempfile.TemporaryDirectory() as directory:
        files = []
        for repo, version, name, checksum in PACKAGES:
            target = Path(directory) / name
            with urlopen(f"https://github.com/{repo}/releases/download/{version}/{name}", timeout=120) as source:
                data = source.read(256 << 20)
            if hashlib.sha256(data).hexdigest() != checksum:
                raise RuntimeError(f"Intel package checksum mismatch: {name}")
            target.write_bytes(data)
            files.append(str(target))
        # Ubuntu's 1.16 loader crashes in PyTorch 2.10 XPU device enumeration.
        subprocess.run(["dpkg", "-r", "libze1"], check=True)
        subprocess.run(["dpkg", "-i", *files], check=True)
