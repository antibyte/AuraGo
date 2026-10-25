#!/usr/bin/env python3
"""Build reproducible offline helpers and maintain trusted installer bootstrap pins."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess

ROOT = Path(__file__).resolve().parent.parent
PINS = ROOT / "scripts/release-verifier.json"
BEGIN = "# BEGIN GENERATED RELEASE VERIFIER\n"
END = "# END GENERATED RELEASE VERIFIER\n"
TARGETS = [f"{system}/{arch}" for system in ("linux", "darwin", "windows") for arch in ("amd64", "arm64")]


def embedded(pins):
    entries = []
    for target, asset in pins["assets"].items():
        system, arch = target.split("/")
        spec = f'{asset["name"]}|{asset["size"]}|{asset["sha256"]}|{pins["base_url"]}'
        entries.append(f"        native:{system}:{arch}) printf '%s\\n' '{spec}' ;;")
    # Official upstream v3.1.4 assets: only the historical keyless/custom-key path.
    cosign = {
        "linux/amd64": (145688404, "424e2c97d0b45fd7a6b608a87f090f35633f4c23ece5f092d48bba11aaf4bbfa"),
        "linux/arm64": (136437761, "ade83b7aac62e131cacfe96d958948b2150cc9efec46a570a2649b8e27dedad2"),
        "darwin/amd64": (105951973, "9d1e4abe2b6be279ed0d2ff706fde1ccd738747c834e3e6d44039c238867334f"),
        "darwin/arm64": (99023586, "9f9b8cd557cf2e5b3df3373a00baa005a86b02391e3e06dd0ce37b3a1533d3a4"),
        "windows/amd64": (191066624, "cbe953787074611f1334cbf37dc1cf953995bb8c45398eb4fc7bb781d29c1f2a"),
    }
    for target, (size, digest) in cosign.items():
        system, arch = target.split("/")
        name = f'cosign-{system}-{arch}' + (".exe" if system == "windows" else "")
        spec = f"{name}|{size}|{digest}|https://github.com/sigstore/cosign/releases/download/v3.1.4"
        entries.append(f"        cosign:{system}:{arch}) printf '%s\\n' '{spec}' ;;")
    template = (ROOT / "scripts/release-bootstrap.sh").read_text(encoding="utf-8")
    return BEGIN + template.replace("        # @release-verifier-pins@", "\n".join(entries)) + END


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--pin", action="store_true", help="record reviewed builds and regenerate trusted scripts")
    parser.add_argument("--check", action="store_true", help="rebuild and check all pins/embedded scripts")
    parser.add_argument("--scripts-only", action="store_true", help="check generated scripts without rebuilding")
    parser.add_argument("--require-published", action="store_true", help="require the public immutable bootstrap before application publication")
    parser.add_argument("--verify-manifest", type=Path, help="check a signed application manifest against consumer trust before publishing")
    parser.add_argument("--out", type=Path, default=ROOT / "deploy/release-verifier")
    args = parser.parse_args()
    if args.pin and args.check:
        parser.error("--pin and --check are mutually exclusive")
    pins = json.loads(PINS.read_text(encoding="utf-8")) if PINS.exists() else {
        "version": 1,
        "toolchain": "go1.27.2",
        "base_url": "https://github.com/antibyte/AuraGo/releases/download/release-verifier-v1",
        "assets": {},
    }
    if not args.scripts_only:
        args.out.mkdir(parents=True, exist_ok=True)
        for target in TARGETS:
            system, arch = target.split("/")
            name = f"release-verifier_{system}_{arch}" + (".exe" if system == "windows" else "")
            output = args.out / name
            env = os.environ | {"GOTOOLCHAIN": pins["toolchain"], "GOOS": system, "GOARCH": arch, "CGO_ENABLED": "0", "GOAMD64": "v1", "GOARM64": "v8.0", "GOFLAGS": "", "GOEXPERIMENT": ""}
            subprocess.run(["go", "build", "-buildvcs=false", "-trimpath", "-ldflags=-s -w -buildid=", "-o", str(output), "./cmd/release-verifier"], cwd=ROOT, env=env, check=True)
            asset = {"name": name, "size": output.stat().st_size, "sha256": hashlib.sha256(output.read_bytes()).hexdigest()}
            if args.pin:
                pins["assets"][target] = asset
            elif asset != pins["assets"].get(target):
                raise SystemExit(f"Bootstrap differs from reviewed pins: {target}; review source/toolchain before repinning.")
            print(f"{target}: {asset['size']} bytes, SHA-256 verified")
        expected = {asset["name"] for asset in pins["assets"].values()}
        if {path.name for path in args.out.glob("release-verifier_*")} != expected:
            raise SystemExit("Unexpected verifier artifacts in output directory; refusing to package them.")
        rotation = ROOT / "internal/releaseverify"
        rotation_names = ("release-keyring.json", "release-keyring.json.sig")
        if (rotation / rotation_names[0]).exists() != (rotation / rotation_names[1]).exists():
            raise SystemExit("Release keyring and its root signature must be supplied together.")
        for name in rotation_names:
            source, destination = rotation / name, args.out.parent / name
            if source.exists():
                shutil.copyfile(source, destination)
            else:
                destination.unlink(missing_ok=True)
    if args.pin:
        PINS.write_text(json.dumps(pins, indent=2) + "\n", encoding="utf-8", newline="\n")
    block = embedded(pins)
    for name in ("install.sh", "update.sh", "repair-update.sh"):
        path = ROOT / name
        source = path.read_text(encoding="utf-8")
        if BEGIN in source:
            start, end = source.index(BEGIN), source.index(END) + len(END)
        else:
            start = source.index("verify_release_checksums_signature() {")
            end_marker = "\n# REPAIR RELEASE PREFLIGHT" if name == "repair-update.sh" else "\nfetch_release_checksums() {"
            end = source.index(end_marker, start)
        generated = source[:start] + block + source[end:]
        if args.pin:
            path.write_text(generated, encoding="utf-8", newline="\n")
        elif source != generated:
            raise SystemExit(f"Generated release bootstrap drift: {name}")
    print("Trusted release bootstrap blocks match.")
    if args.require_published:
        tag = pins["base_url"].rsplit("/", 1)[1]
        result = subprocess.run(["gh", "api", f"repos/antibyte/AuraGo/releases/tags/{tag}"], capture_output=True, text=True)
        if result.returncode:
            raise SystemExit(f"Publish the immutable {tag} bootstrap with release-verifier.yml before publishing application installers.")
        release = json.loads(result.stdout)
        assets = {asset["name"]: asset for asset in release["assets"]}
        if release["draft"] or not release["prerelease"]:
            raise SystemExit("Bootstrap must be a public prerelease retained separately from application releases.")
        for pin in pins["assets"].values():
            asset = assets.get(pin["name"], {})
            if asset.get("size") != pin["size"] or asset.get("digest") != "sha256:" + pin["sha256"]:
                raise SystemExit(f"Published bootstrap is missing or differs from trusted pins: {pin['name']}")
        print("Published bootstrap assets match every trusted pin.")
    if args.verify_manifest:
        manifest = args.verify_manifest.resolve()
        if Path(str(manifest) + ".pem").exists():
            subprocess.run(["cosign", "verify-blob", "--certificate", str(manifest) + ".pem", "--signature", str(manifest) + ".sig", "--certificate-identity-regexp", "https://github.com/antibyte/AuraGo/.*", "--certificate-oidc-issuer", "https://token.actions.githubusercontent.com", str(manifest)], check=True)
        else:
            system = {"Linux": "linux", "Darwin": "darwin", "Windows": "windows"}.get(platform.system())
            machine = (platform.machine() or os.environ.get("PROCESSOR_ARCHITECTURE", "")).lower()
            arch = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}.get(machine)
            pin = pins["assets"].get(f"{system}/{arch}")
            if pin is None:
                raise SystemExit("No native verifier for the publishing host.")
            tool = args.out / pin["name"]
            if tool.stat().st_size != pin["size"] or hashlib.sha256(tool.read_bytes()).hexdigest() != pin["sha256"]:
                raise SystemExit("Publishing-host verifier differs from trusted pins.")
            command = [str(tool.resolve()), "--manifest", str(manifest), "--signature", str(manifest) + ".sig"]
            result = subprocess.run(command, capture_output=True, text=True)
            if result.returncode:
                ring = manifest.parent / "release-keyring.json"
                proof = manifest.parent / "release-keyring.json.sig"
                if ring.exists() and proof.exists():
                    subprocess.run(command + ["--keyring", str(ring), "--keyring-signature", str(proof)], check=True)
                else:
                    raise SystemExit("Application signature is not trusted by automatic installers: " + result.stderr.strip())
        print("Application signature is accepted by consumer trust.")


if __name__ == "__main__":
    main()
