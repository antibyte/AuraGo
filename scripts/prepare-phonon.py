#!/usr/bin/env python3
"""Prepare/check immutable Phonon release resources. Maintainer-only; stdlib Python 3.10+."""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import sys
import tarfile
import tempfile
import urllib.error
import urllib.request
from urllib.parse import quote, urlsplit
import zipfile

REPO = Path(__file__).resolve().parents[1]
MANIFEST = json.loads((REPO / "internal/phonon/manifest.json").read_text(encoding="utf-8"))
STAGE = REPO / "deploy/phonon"
BUNDLE_ID = MANIFEST["runtime_version"] + "-" + MANIFEST["model_revision"][:12] + "-" + MANIFEST["voices_revision"][:12]


class CredentialSafeRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        if urlsplit(newurl).scheme != "https":
            raise ValueError("resource redirect must use HTTPS")
        redirected = super().redirect_request(req, fp, code, msg, headers, newurl)
        if redirected is not None:
            redirected.remove_header("Authorization")
        return redirected


def digest(data):
    return hashlib.sha256(data).hexdigest()


def verify(data, expected):
    if len(data) != expected["size"]:
        raise ValueError("resource size mismatch")
    if digest(data) != expected.get("sha256"):
        raise ValueError("resource SHA-256 mismatch")


def fetch(url, limit, token=""):
    if token and (not token.isascii() or any(character.isspace() for character in token)):
        raise ValueError("invalid HF_TOKEN format")
    request = urllib.request.Request(url, headers={"User-Agent": "AuraGo-Phonon-Packager/1"})
    if token:
        request.add_header("Authorization", "Bearer " + token)
    opener = urllib.request.build_opener(CredentialSafeRedirect())
    try:
        with opener.open(request, timeout=120) as response:
            data = response.read(limit + 1)
    except urllib.error.HTTPError as error:
        raise RuntimeError(f"resource download returned HTTP {error.code}") from None
    if len(data) > limit:
        raise ValueError("resource download exceeds size limit")
    return data


def write(path, data, executable=False):
    path.parent.mkdir(parents=True, exist_ok=True)
    staged = None
    try:
        with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as temp:
            staged = Path(temp.name)
            temp.write(data)
            temp.flush()
            os.fsync(temp.fileno())
        staged.chmod(0o755 if executable else 0o644)
        os.replace(staged, path)
    finally:
        if staged is not None:
            staged.unlink(missing_ok=True)


def resource(path, expected, url, download, source=None, token=""):
    if path.is_file() and not path.is_symlink():
        try:
            verify(path.read_bytes(), expected)
            return
        except ValueError:
            pass
    if not download and source is None:
        raise RuntimeError(f"missing/invalid {path.relative_to(REPO)}; run scripts/prepare-phonon.py --fetch first")
    data = source.read_bytes() if source is not None else fetch(url, expected["size"], token)
    verify(data, expected)
    write(path, data)


def runtime_binary(archive, filename, name):
    if filename.endswith(".zip"):
        with zipfile.ZipFile(io.BytesIO(archive)) as bundle:
            matches = [entry for entry in bundle.infolist() if Path(entry.filename).name == name and not entry.is_dir()]
            if len(matches) != 1 or matches[0].file_size > 128 << 20:
                raise ValueError("invalid runtime archive")
            return bundle.read(matches[0])
    with tarfile.open(fileobj=io.BytesIO(archive), mode="r:gz") as bundle:
        matches = [entry for entry in bundle.getmembers() if Path(entry.name).name == name and entry.isfile()]
        if len(matches) != 1 or matches[0].size > 128 << 20:
            raise ValueError("invalid runtime archive")
        return bundle.extractfile(matches[0]).read()


def prepare_runtime(runtime, download):
    name = "ptts.exe" if runtime["target"].startswith("windows_") else "ptts"
    path = STAGE / "runtime" / runtime["target"] / name
    expected_binary = runtime.get("binary_sha256")
    if path.is_file() and not path.is_symlink() and expected_binary and digest(path.read_bytes()) == expected_binary:
        return
    if not download:
        raise RuntimeError(f"missing/invalid runtime {runtime['target']}; run --fetch first")
    url = f"https://github.com/gradium-ai/xn-ptts/releases/download/v{MANIFEST['runtime_version']}/{runtime['archive']}"
    archive = fetch(url, 32 << 20)
    if digest(archive) != runtime["sha256"]:
        raise ValueError("runtime archive checksum mismatch")
    binary = runtime_binary(archive, runtime["archive"], name)
    if not expected_binary or digest(binary) != expected_binary:
        raise ValueError("runtime binary checksum is not pinned or does not match")
    write(path, binary, executable=True)


def release_model(tag):
    """Reuse the maintainer's published bundle without passing HF credentials to CI."""
    expected = [("model/" + file["name"], file) for file in MANIFEST["model_files"]]
    expected += [(voice["filename"], voice) for voice in MANIFEST["voices"] if voice["bundled"]]
    try:
        for name, file in expected:
            path = STAGE / name
            if path.is_symlink():
                raise ValueError("unexpected resource link")
            verify(path.read_bytes(), file)
        return
    except (OSError, ValueError):
        pass
    suffix = "latest/download" if tag == "latest" else "download/" + quote(tag, safe="")
    base = "https://github.com/antibyte/AuraGo/releases/" + suffix + "/"
    checksums = fetch(base + "SHA256SUMS", 4 << 20).decode("utf-8")
    matches = [line.split()[0] for line in checksums.splitlines() if len(line.split()) == 2 and line.split()[1] == "phonon-model.dat"]
    if len(matches) != 1:
        raise ValueError("release has no unique Phonon model checksum")
    data = fetch(base + "phonon-model.dat", 128 << 20)
    if digest(data) != matches[0]:
        raise ValueError("release Phonon model checksum mismatch")
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:gz") as archive:
        members = archive.getmembers()
        if len(members) > 128 or len({member.name for member in members}) != len(members):
            raise ValueError("invalid Phonon resource archive")
        for name, file in expected:
            member = archive.getmember("phonon/" + BUNDLE_ID + "/" + name)
            if not member.isfile() or member.size != file["size"]:
                raise ValueError("invalid Phonon resource archive entry")
            content = archive.extractfile(member).read(file["size"] + 1)
            verify(content, file)
            write(STAGE / name, content)


def pack(path, files):
    records = {str(file.relative_to(STAGE)).replace(os.sep, "/"): digest(file.read_bytes()) for file in files}
    with tempfile.NamedTemporaryFile(dir=path.parent, delete=False) as temp:
        staged = Path(temp.name)
    try:
        with tarfile.open(staged, "w:gz", format=tarfile.PAX_FORMAT) as archive:
            for file in sorted(files):
                info = archive.gettarinfo(str(file), "phonon/" + BUNDLE_ID + "/" + str(file.relative_to(STAGE)).replace(os.sep, "/"))
                info.uid = info.gid = 0
                info.uname = info.gname = ""
                info.mtime = 0
                info.mode = 0o755 if file.name in ("ptts", "ptts.exe") else 0o644
                with file.open("rb") as content:
                    archive.addfile(info, content)
            metadata = (json.dumps(records, sort_keys=True, indent=2) + "\n").encode()
            info = tarfile.TarInfo("phonon/" + BUNDLE_ID + "/" + path.stem + ".sha256.json")
            info.size, info.mode, info.mtime = len(metadata), 0o644, 0
            archive.addfile(info, io.BytesIO(metadata))
        os.replace(staged, path)
    finally:
        staged.unlink(missing_ok=True)
    print(f"Prepared {path.name}: {path.stat().st_size} bytes, SHA-256 {digest(path.read_bytes())}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fetch", action="store_true", help="download missing pinned resources; HF_TOKEN is used only for the gated model")
    parser.add_argument("--check", action="store_true", help="verify prepared resources without network access")
    parser.add_argument("--pack", action="store_true", help="verify and create the release resource archives")
    parser.add_argument("--target", choices=[runtime["target"] for runtime in MANIFEST["runtimes"]], help="prepare/check one platform (default: all)")
    parser.add_argument("--source-dir", type=Path, help="copy the pinned model from an authorized local checkpoint")
    parser.add_argument("--install-dir", type=Path, help="stage verified resources beside an executable (requires --target)")
    parser.add_argument("--from-release", metavar="TAG", help="reuse a verified AuraGo speech bundle (or latest), without HF access")
    args = parser.parse_args()
    download = (args.fetch or args.from_release is not None) and not args.check
    if args.from_release and not args.check:
        release_model(args.from_release)
    for runtime in MANIFEST["runtimes"]:
        if args.target is None or runtime["target"] == args.target:
            prepare_runtime(runtime, download)
    voices_base = f"https://huggingface.co/{MANIFEST['voices_repo']}/resolve/{MANIFEST['voices_revision']}/"
    for voice in MANIFEST["voices"]:
        if voice["bundled"]:
            resource(STAGE / voice["filename"], voice, voices_base + voice["filename"], download)
    # Pass a local catalog to the CLI so inference never initializes a Hub lookup.
    local_catalog = "".join(json.dumps({"name": voice["name"], "filename": voice["filename"]}, ensure_ascii=False) + "\n" for voice in MANIFEST["voices"] if voice["bundled"])
    if download or args.pack:
        write(STAGE / "voices.jsonl", local_catalog.encode())
    elif not (STAGE / "voices.jsonl").is_file() or (STAGE / "voices.jsonl").read_bytes() != local_catalog.encode():
        raise ValueError("missing/invalid local voice catalog")
    model_base = f"https://huggingface.co/{MANIFEST['model_repo']}/resolve/{MANIFEST['model_revision']}/"
    for file in MANIFEST["model_files"]:
        source = args.source_dir / file["name"] if args.source_dir else None
        try:
            resource(STAGE / "model" / file["name"], file, model_base + file["name"], download, source, os.environ.get("HF_TOKEN", ""))
        except RuntimeError as error:
            raise RuntimeError(str(error) + "; Phonon 40m requires prior model approval and HF_TOKEN, --source-dir, or --from-release") from None
    license_files = [REPO / "licences/texts/CC-BY-NC-4.0.txt", REPO / "licences/texts/Apache-2.0.txt", REPO / "licences/texts/Phonon-MIT.txt", REPO / "documentation/phonon.md"]
    if args.pack or args.install_dir:
        for file in license_files:
            if not file.is_file():
                raise ValueError(f"missing required notice {file.name}")
            write(STAGE / "licences" / file.name, file.read_bytes())
    if args.install_dir:
        if args.target is None:
            raise ValueError("--install-dir requires --target")
        target = args.install_dir / "phonon" / BUNDLE_ID
        files = [STAGE / "model" / file["name"] for file in MANIFEST["model_files"]]
        files += [STAGE / voice["filename"] for voice in MANIFEST["voices"] if voice["bundled"]]
        files += [STAGE / "licences" / file.name for file in license_files] + [STAGE / "voices.jsonl"]
        name = "ptts.exe" if args.target.startswith("windows_") else "ptts"
        files += [STAGE / "runtime" / args.target / name]
        for file in files:
            destination = target / file.relative_to(STAGE)
            destination.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(file, destination)
            destination.chmod(0o755 if file.name in ("ptts", "ptts.exe") else 0o644)
    if args.pack:
        files = [STAGE / "model" / file["name"] for file in MANIFEST["model_files"]] + [STAGE / voice["filename"] for voice in MANIFEST["voices"] if voice["bundled"]]
        files += [STAGE / "voices.jsonl"] + [STAGE / "licences" / file.name for file in license_files]
        pack(REPO / "deploy/phonon-model.dat", files)
        for runtime in MANIFEST["runtimes"]:
            if args.target is None or runtime["target"] == args.target:
                name = "ptts.exe" if runtime["target"].startswith("windows_") else "ptts"
                files = [STAGE / "runtime" / runtime["target"] / name] + [STAGE / "licences" / file.name for file in license_files]
                pack(REPO / f"deploy/phonon-runtime_{runtime['target']}.dat", files)
    print("Phonon resources verified")


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, RuntimeError, KeyError, tarfile.TarError) as error:
        sys.exit(str(error))
