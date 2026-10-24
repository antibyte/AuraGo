#!/usr/bin/env python3
"""Import the reviewed, pinned icon selection without running upstream installers.

--check verifies the shipped files offline. Add --source-dir to also reproduce
them byte for byte from the archives named in assets/desktop-icon-themes/sources.json.
"""
import argparse
import base64
import binascii
import hashlib
import gzip
import io
import json
import posixpath
import re
import struct
import tarfile
import xml.etree.ElementTree as ET
from pathlib import Path, PurePosixPath

ROOT = Path(__file__).resolve().parents[1]
INPUT = ROOT / "assets/desktop-icon-themes"
SVG = "http://www.w3.org/2000/svg"
XLINK = "http://www.w3.org/1999/xlink"
ET.register_namespace("", SVG)
ET.register_namespace("xlink", XLINK)


def digest(data):
    return hashlib.sha256(data).hexdigest()


def json_bytes(value):
    return (json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n").encode()


def safe_path(path):
    if not path or "\\" in path or ":" in path or path.startswith("/") or any(p in (".", "..") for p in path.split("/")):
        raise ValueError("Unsafe path: " + path)
    return path


def validate_png(data, require_alpha=True):
    if not data.startswith(b"\x89PNG\r\n\x1a\n"):
        raise ValueError("Invalid PNG")
    offset, alpha, ended = 8, False, False
    while offset < len(data):
        if offset + 12 > len(data):
            raise ValueError("Truncated PNG")
        size = struct.unpack(">I", data[offset:offset + 4])[0]
        tag = data[offset + 4:offset + 8]
        chunk = data[offset + 8:offset + 8 + size]
        crc = data[offset + 8 + size:offset + 12 + size]
        if len(crc) != 4 or struct.unpack(">I", crc)[0] != binascii.crc32(tag + chunk) & 0xffffffff:
            raise ValueError("Invalid PNG checksum")
        if tag == b"IHDR":
            width, height, depth, color = struct.unpack(">IIBB", chunk[:10])
            if not 0 < width <= 1024 or not 0 < height <= 1024:
                raise ValueError("PNG dimensions out of range")
            alpha = color in (4, 6)
        if tag == b"tRNS":
            alpha = True
        offset += size + 12
        if tag == b"IEND":
            ended = True
            break
    if not ended or offset != len(data) or (require_alpha and not alpha):
        raise ValueError("PNG must be complete and support transparency")


def safe_svg(data):
    text = data.decode("utf-8-sig")
    if re.search(r"<!\s*(?:DOCTYPE|ENTITY)", text, re.I):
        raise ValueError("SVG document types/entities are forbidden")
    root = ET.fromstring(text)
    if root.tag == "svg":
        for node in root.iter():
            if not node.tag.startswith("{"):
                node.tag = "{" + SVG + "}" + node.tag
    if root.tag != "{" + SVG + "}svg":
        raise ValueError("Expected SVG root")
    allowed = set("svg g defs style title desc path rect circle ellipse line polyline polygon text tspan symbol linearGradient radialGradient stop clipPath mask pattern use image filter feGaussianBlur feBlend feOffset feColorMatrix feComposite feFlood feMerge feMergeNode feComponentTransfer feFuncR feFuncG feFuncB feFuncA".split())
    for parent in root.iter():
        for child in list(parent):
            # Editor metadata is not part of the rendered artwork.
            if not child.tag.startswith("{" + SVG + "}") or child.tag == "{" + SVG + "}metadata":
                parent.remove(child)
    ids = {node.get("id") for node in root.iter() if node.get("id")}
    for node in root.iter():
        tag = node.tag.split("}")[-1]
        if tag not in allowed:
            raise ValueError("Unsupported SVG element: " + tag)
        for key, value in list(node.attrib.items()):
            local = key.split("}")[-1]
            if local.lower().startswith("on") or local in ("base", "src"):
                raise ValueError("Active SVG attribute: " + local)
            if key.startswith("{") and not key.startswith("{" + XLINK + "}"):
                del node.attrib[key]
                continue
            if local == "href":
                if value.startswith("#") and value[1:] in ids:
                    pass
                elif tag == "image" and value.startswith("data:image/png;base64,"):
                    raw = base64.b64decode(re.sub(r"\s", "", value.split(",", 1)[1]), validate=True)
                    validate_png(raw, require_alpha=False)
                else:
                    raise ValueError("External or invalid SVG reference: " + value[:100])
        values = list(node.attrib.values()) + ([node.text or ""] if tag == "style" else [])
        for value in values:
            if re.search(r"@|expression\s*\(|javascript:|\\", value, re.I):
                raise ValueError("Active SVG CSS")
            for ref in re.findall(r"url\s*\(([^)]*)\)", value, re.I):
                ref = ref.strip(" \t\r\n\"'")
                if not ref.startswith("#") or ref[1:] not in ids:
                    raise ValueError("External or invalid SVG paint reference")
    return ET.tostring(root, encoding="utf-8") + b"\n"


class Archive:
    def __init__(self, path, spec):
        archive_data = path.read_bytes()
        if digest(archive_data) != spec["sha256"]:
            raise ValueError("Source archive checksum mismatch: " + str(path))
        self.tar = tarfile.open(fileobj=io.BytesIO(gzip.decompress(archive_data) if archive_data[:2] == b"\x1f\x8b" else archive_data))
        self.members = {}
        self.overlay = {}
        prefix = spec.get("archive_prefix", "")
        for item in self.tar.getmembers():
            if item.isdir():
                continue
            if not item.name.startswith(prefix):
                raise ValueError("Archive prefix mismatch")
            name = safe_path(item.name[len(prefix):])
            self.members[name] = item
            virtual = re.sub(r"^(src|links)/", "", name) if spec.get("overlay") else name
            if virtual not in self.overlay or name.startswith("src/"):
                self.overlay[virtual] = name

    def read(self, name):
        safe_path(name)
        seen = []
        while True:
            if name in seen or len(seen) >= 32:
                raise ValueError("Cyclic icon link: " + name)
            seen.append(name)
            item = self.members[name]
            if item.isfile():
                if item.size > 2 * 1024 * 1024:
                    raise ValueError("Oversize icon: " + name)
                return self.tar.extractfile(item).read(), name, seen[:-1]
            if not item.issym():
                raise ValueError("Unsupported archive entry: " + name)
            link = item.linkname
            if link.startswith("/") or "\\" in link or ":" in link:
                raise ValueError("Unsafe icon link: " + link)
            target = safe_path(posixpath.normpath(posixpath.join(posixpath.dirname(name), link)))
            if target in self.members:
                name = target
            else:
                virtual = re.sub(r"^(src|links)/", "", target)
                name = self.overlay[virtual]


def input_hashes(theme):
    paths = ["scripts/import-desktop-icon-themes.py", "ui/img/desktop-mini/symbols.svg", "LICENSE", "licences/texts/CC-BY-NC-SA-2.5.txt"]
    paths += ["assets/desktop-icon-themes/" + name for name in ("sources.json", "supplements.json", theme + ".json")]
    return {path: digest((ROOT / path).read_bytes().replace(b"\r\n", b"\n")) for path in paths}


def output_files(sources, source_dir):
    catalog = json.loads((ROOT / "ui/img/papirus/manifest.json").read_text(encoding="utf-8"))
    symbols = ET.parse(ROOT / "ui/img/desktop-mini/symbols.svg").getroot()
    geometry = json.loads((INPUT / "supplements.json").read_text(encoding="utf-8"))
    geometry.update({node.get("id"): "".join(ET.tostring(c, encoding="unicode") for c in node) for node in symbols})
    for theme, spec in sources.items():
        archive = Archive(source_dir / spec["archive"], spec)
        mappings = json.loads((INPUT / (theme + ".json")).read_text(encoding="utf-8"))
        directory = "img/" + theme.replace("_", "-")
        manifest = {"version": 2, "name": spec["name"], "source": spec["url"], "revision": spec["revision"], "icons": {}, "aliases": catalog["aliases"], "action_aliases": mappings["action_aliases"], "actions": {"light": {}, "dark": {}}, "folder_variants": {}, "folder_colors": list(spec.get("folder_colors", {"standard": ""})), "licenses": directory + "/LICENSE.txt"}
        records, outputs, emitted = {}, {}, {}
        manifest["inputs"] = input_hashes(theme)

        def emit(key, selection, role, variant=""):
            source = selection.get("source")
            changes = []
            notices = []
            if source:
                data, original, links = archive.read(source)
                source_sha = digest(data)
                extension = PurePosixPath(original).suffix
                license_id = spec["license"]
                if extension == ".svg":
                    raw_svg = data
                    try:
                        data = safe_svg(data)
                    except ValueError as error:
                        raise ValueError(theme + ": " + source + ": " + str(error)) from error
                    for node in ET.fromstring(raw_svg).iter():
                        if node.tag.split("}")[-1] in ("creator", "rights", "license"):
                            notice = " ".join(" ".join(node.itertext()).split())
                            notice += " " + " ".join(node.attrib.values())
                            if notice.strip() and notice.strip() not in notices:
                                notices.append(notice.strip())
                    changes.append("Validated SVG; removed editor metadata; normalized XML")
                    if role == "actions" and variant == "dark":
                        for color in spec.get("action_foregrounds", []):
                            data = re.sub(re.escape(color.encode()), b"#e4e7eb", data, flags=re.I)
                        changes.append("Dark surface foreground")
                    if role == "actions":
                        for color, replacement in spec.get("action_colors", {}).get(variant, {}).items():
                            if color.lower().encode() in data.lower():
                                data = re.sub(re.escape(color.encode()), replacement.encode(), data, flags=re.I)
                                changes.append("Control contrast on " + variant + ": " + color + " to " + replacement)
                    if role == "folders" and variant != "standard":
                        data = re.sub(re.escape(spec["folder_colors"]["standard"].encode()), spec["folder_colors"][variant].encode(), data, flags=re.I)
                        if theme == "tela" and variant == "grey":
                            data = data.replace(b"#ffffff", b"#666666")
                        changes.append("Folder color: " + variant)
                elif extension == ".png":
                    validate_png(data)
                else:
                    raise ValueError("Unsupported image format")
            else:
                motif = selection["supplement"]
                original, source_sha, links, extension = "AuraGo original: " + motif, digest(geometry[motif].encode()), [], ".svg"
                license_id = "MIT"
                ink, accent = spec["supplement_colors"][variant if role == "actions" else "light"]
                body = geometry[motif].replace("currentColor", ink)
                # Candy's gradient strokes and Buuf's warm ink retain contrast at 14px.
                gradient = '<defs><linearGradient id="aura-ink" x2="1" y2="1"><stop stop-color="' + ink + '"/><stop offset="1" stop-color="' + accent + '"/></linearGradient></defs>' if theme == "candy" else ""
                stroke = "url(#aura-ink)" if gradient else ink
                if gradient:
                    body = body.replace(ink, stroke)
                if role == "icons" and theme in ("tela", "fluent", "buuf_nestort"):
                    if theme == "buuf_nestort":
                        tile = '<path d="M3 3 20 2 22 19 4 22 2 8Z" fill="#e7c990" stroke="#69442b" stroke-width="1"/><path d="m4 5 14-1M5 19l13-1" stroke="#fff1cc" opacity=".65"/>'
                    elif theme == "fluent":
                        tile = '<rect x="2" y="3" width="20" height="19" rx="5" fill="#157bc6" opacity=".2"/><rect x="2" y="2" width="20" height="19" rx="5" fill="#c7e7ff"/>'
                    else:
                        tile = '<circle cx="12" cy="12" r="11" fill="#d3e8fc"/><path d="M4 18 18 4a10 10 0 0 1-14 14" fill="#5294e2" opacity=".2"/>'
                    body = tile + '<g transform="translate(3 3) scale(.75)">' + body + '</g>'
                data = safe_svg(('<svg xmlns="' + SVG + '" viewBox="0 0 24 24" width="24" height="24">' + gradient + '<g fill="none" stroke="' + stroke + '" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">' + body + '</g></svg>').encode())
                changes.append("Original AuraGo supplement; no upstream artwork used")
            relative = directory + "/" + role + "/" + (variant + "/" if variant else "") + key + extension
            sha = digest(data)
            use = {"key": key, "role": role, "variant": variant, "source": original, "requested_source": source, "source_sha256": source_sha, "resolved_links": links, "changes": changes}
            if notices:
                use["upstream_notices"] = notices
            if selection.get("reason"):
                use["selection_note"] = selection["reason"]
                use["rejected_source"] = selection.get("rejected_source")
            identical = emitted.get((license_id, sha))
            if identical:
                records[identical]["uses"].append(use)
                return identical
            emitted[(license_id, sha)] = relative
            outputs["ui/" + relative] = data
            records[relative] = {"license": license_id, "authors": spec["authors"] if source else ["AuraGo contributors"], "sha256": sha, "uses": [use]}
            return relative

        for key, selection in mappings["icons"].items():
            manifest["icons"][key] = emit(key, selection, "icons")
        for variant in ("light", "dark"):
            for key, selection in mappings["actions"].items():
                manifest["actions"][variant][key] = emit(key, selection, "actions", variant)
        for variant in manifest["folder_colors"]:
            manifest["folder_variants"][variant] = {key: emit(key, selection, "folders", variant) for key, selection in mappings["folders"].items()}
        for name, source in spec["notices"].items():
            data = archive.read(source)[0] if not source.startswith("local:") else (ROOT / source[6:]).read_bytes()
            outputs["ui/" + directory + "/" + name] = data.replace(b"\r\n", b"\n").replace(b"\r", b"\n")
        outputs["ui/" + directory + "/NOTICE.txt"] = (spec["notice"] + "\n\nSource: " + spec["url"] + "\nRevision: " + spec["revision"] + "\nPer-file provenance and modifications: provenance.json\nAuraGo original supplements: MIT (SUPPLEMENTS-LICENSE.txt).\n").encode()
        outputs["ui/" + directory + "/SUPPLEMENTS-LICENSE.txt"] = (ROOT / "LICENSE").read_bytes().replace(b"\r\n", b"\n")
        outputs["ui/" + directory + "/manifest.json"] = json_bytes(manifest)
        outputs["ui/" + directory + "/provenance.json"] = json_bytes(records)
        for name, data in outputs.items():
            yield name, data


def verify(sources):
    catalog = json.loads((ROOT / "ui/img/papirus/manifest.json").read_text(encoding="utf-8"))
    mini = json.loads((ROOT / "ui/img/desktop-mini/manifest.json").read_text(encoding="utf-8"))["icons"]
    symbols = {n.get("id") for n in ET.parse(ROOT / "ui/img/desktop-mini/symbols.svg").getroot()}
    for theme, spec in sources.items():
        directory = "img/" + theme.replace("_", "-")
        base = ROOT / "ui" / directory
        manifest = json.loads((base / "manifest.json").read_text(encoding="utf-8"))
        assert manifest["inputs"] == input_hashes(theme), theme + " importer inputs changed; reproduce from pinned sources"
        records = json.loads((base / "provenance.json").read_text(encoding="utf-8"))
        assert set(manifest["icons"]) == set(catalog["icons"]), theme + " catalog drift"
        assert manifest["aliases"] == catalog["aliases"], theme + " alias drift"
        assert all(v in manifest["icons"] for v in manifest["aliases"].values())
        assert list(manifest["folder_colors"]) == list(spec.get("folder_colors", {"standard": ""}))
        refs = list(manifest["icons"].values())
        for mode in ("light", "dark"):
            assert set(mini) | symbols <= set(manifest["actions"][mode]), theme + " missing controls"
            assert all(v in manifest["actions"][mode] for v in manifest["action_aliases"].values())
            refs += list(manifest["actions"][mode].values())
        for values in manifest["folder_variants"].values():
            refs += list(values.values())
        assert set(refs) == set(records), theme + " provenance mismatch"
        for relative, record in records.items():
            safe_path(relative)
            assert relative.startswith(directory + "/"), "Cross-theme asset"
            path = ROOT / "ui" / relative
            assert not path.is_symlink(), "Unresolved symlink"
            data = path.read_bytes()
            assert digest(data) == record["sha256"], "Asset drift: " + relative
            safe_svg(data) if path.suffix == ".svg" else validate_png(data)
        expected = {str((ROOT / "ui" / r).relative_to(base)).replace("\\", "/") for r in refs}
        actual = {str(p.relative_to(base)).replace("\\", "/") for p in base.rglob("*") if p.suffix in (".svg", ".png")}
        assert actual == expected, "Untracked generated artwork: " + theme
        for notice in [*spec["notices"], "NOTICE.txt", "SUPPLEMENTS-LICENSE.txt"]:
            assert (base / notice).stat().st_size > 50


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-dir", type=Path)
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    sources = json.loads((INPUT / "sources.json").read_text(encoding="utf-8"))
    if not args.check and not args.source_dir:
        parser.error("--source-dir is required for import; --check works offline")
    if args.source_dir:
        generated = set()
        for relative, data in output_files(sources, args.source_dir):
            generated.add(relative)
            target = ROOT / safe_path(relative)
            if args.check:
                assert target.read_bytes() == data, "Reproduction mismatch: " + relative
            else:
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(data)
        if not args.check:
            # Only prune previously generated images in these four owned output directories.
            for theme in sources:
                base = ROOT / "ui/img" / theme.replace("_", "-")
                assert base.resolve().is_relative_to((ROOT / "ui/img").resolve())
                for path in base.rglob("*"):
                    if path.is_file() and path.suffix in (".svg", ".png") and path.relative_to(ROOT).as_posix() not in generated:
                        path.unlink()
    verify(sources)
    print("Verified four icon themes: catalog, actions, folder colors, SVG/PNG safety, hashes and provenance.")


if __name__ == "__main__":
    main()
