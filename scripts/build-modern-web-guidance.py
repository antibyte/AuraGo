"""Package the pinned GoogleChrome guides for AuraGo; check existing output offline."""

import argparse
import hashlib
import io
import json
import lzma
from pathlib import Path, PurePosixPath
import zipfile

ROOT = Path(__file__).resolve().parents[1]
BUNDLE = ROOT / "internal/tools/bundled_agent_skills"
COMMIT = "4dd968349e54ca45a05dd5c818fb035c9054b022"
ARCHIVE_SHA256 = "e6a252c7a572d52bcda56fb593b6e83f93a0bdcff118e76978d095330178de9f"
SOURCE = "https://github.com/GoogleChrome/modern-web-guidance-src"
DISTRIBUTION = "https://github.com/GoogleChrome/modern-web-guidance"
ZIP_PATH = BUNDLE / "modern-web-guidance.zip.xz"
MANIFEST_PATH = BUNDLE / "modern-web-guidance.json"


def digest(data):
    return hashlib.sha256(data).hexdigest()


def make_index(files):
    lines = ["# Modern Web Guidance index", "", "Read only guides relevant to the task.", ""]
    category = None
    for name, data in sorted(files.items()):
        if not name.startswith("references/guides/"):
            continue
        path = PurePosixPath(name)
        if path.parent.name != category:
            category = path.parent.name
            lines.extend(["## " + category.replace("-", " ").title(), ""])
        title = data.decode("utf-8").splitlines()[0].lstrip("# ")
        lines.append(f"- [{path.stem}]({name.removeprefix('references/')}): {title}")
    return ("\n".join(lines) + "\n").encode()


def check():
    manifest = json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))
    data = ZIP_PATH.read_bytes()
    assert manifest["revision"] == COMMIT
    assert manifest["upstream_archive_sha256"] == ARCHIVE_SHA256
    assert digest(data) == manifest["bundle_sha256"], "Bundle checksum mismatch"
    with zipfile.ZipFile(io.BytesIO(lzma.decompress(data))) as archive:
        assert len(archive.namelist()) == len(set(archive.namelist())), "Duplicate archive paths"
        files = {name: archive.read(name) for name in archive.namelist()}
    assert {name: digest(data) for name, data in files.items()} == manifest["files"]
    assert files["SKILL.md"] == (BUNDLE / "modern-web-guidance/SKILL.md").read_bytes()
    assert files["references/INDEX.md"] == make_index(files)
    assert sum(name.startswith("references/guides/") for name in files) == 209
    for name, data in files.items():
        path = PurePosixPath(name)
        assert not path.is_absolute() and ".." not in path.parts and "\\" not in name
        assert name == "SKILL.md" or name.startswith("references/")
        data.decode("utf-8")
    print(f"Modern Web Guidance: {len(files)} verified files; offline check passed")


def build(archive_path):
    data = archive_path.read_bytes()
    if digest(data) != ARCHIVE_SHA256:
        raise ValueError("Upstream archive does not match the reviewed SHA-256")
    prefix = f"modern-web-guidance-{COMMIT}/"
    guides = prefix + "skills/modern-web-guidance/guides/"
    with zipfile.ZipFile(io.BytesIO(data)) as archive:
        files = {
            "references/guides/" + name.removeprefix(guides): archive.read(name)
            for name in archive.namelist()
            if name.startswith(guides) and name.endswith(".md")
        }
        files["references/LICENSE.txt"] = archive.read(prefix + "LICENSE")
    files["SKILL.md"] = (BUNDLE / "modern-web-guidance/SKILL.md").read_bytes()
    files["references/INDEX.md"] = make_index(files)
    files["references/UPSTREAM.md"] = (
        "# Provenance\n\nModern Web Guidance by GoogleChrome and contributors.\n\n"
        f"Source: {SOURCE}\n\nDistribution: {DISTRIBUTION}\n\n"
        f"Revision: `{COMMIT}` (v1.0.0).\n\n"
        f"Archive SHA-256: `{ARCHIVE_SHA256}`.\n\n"
        "All 209 upstream guides are included unchanged under `references/guides/`.\n"
        "AuraGo replaces the CLI-based entrypoint with its native file-reader workflow\n"
        "and adds the local index and this provenance note. No upstream executable ships.\n"
        "License: Apache-2.0; see `LICENSE.txt`.\n"
    ).encode()
    output = io.BytesIO()
    # One shared compression dictionary keeps repeated browser tables within the binary budget.
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_STORED) as archive:
        for name, content in sorted(files.items()):
            info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
            info.compress_type = zipfile.ZIP_STORED
            info.external_attr = 0o100644 << 16
            archive.writestr(info, content)
    packed = lzma.compress(output.getvalue(), preset=6)
    manifest = {
        "source": SOURCE,
        "distribution": DISTRIBUTION,
        "revision": COMMIT,
        "upstream_archive_sha256": ARCHIVE_SHA256,
        "bundle_sha256": digest(packed),
        "files": {name: digest(content) for name, content in sorted(files.items())},
    }
    ZIP_PATH.write_bytes(packed)
    MANIFEST_PATH.write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8", newline="\n")
    check()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--check", action="store_true")
    mode.add_argument("--archive", type=Path, help="Downloaded archive of the pinned distribution")
    args = parser.parse_args()
    if args.check:
        check()
    else:
        build(args.archive)
