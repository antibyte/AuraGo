"""Cross-install reviewed wheels without running target-architecture binaries."""
import subprocess
import sys

arch = sys.argv[1]
machine = {"amd64": "x86_64", "arm64": "aarch64"}[arch]
args = [sys.executable, "-m", "pip", "install", "--no-compile", "--require-hashes",
        "--only-binary=:all:", "--implementation=cp", "--python-version=3.12",
        "--abi=cp312", "--target=/packages", "-r", "/app/requirements.txt"]
for platform in ("manylinux_2_28", "manylinux_2_27", "manylinux_2_17", "manylinux2014"):
    args.append("--platform=" + platform + "_" + machine)
subprocess.run(args, check=True)
