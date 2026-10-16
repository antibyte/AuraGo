"""Package separately generated Infra PNGs without changing their artwork/alpha.

Usage: python scripts/build-infra-assets.py reports/infra/generated-assets.json
Requires Pillow for developer-only asset packaging; runtime assets are local WebP.
"""

import hashlib
import json
from pathlib import Path
import sys

from PIL import Image, ImageDraw

ROOT = Path(__file__).resolve().parents[1]
IDS = """rack-server tower-server mini-pc sbc nas disk-shelf workstation laptop tablet smartphone router firewall modem-ont managed-switch switch access-point mesh-node ups pdu patch-panel camera nvr printer 3d-printer smart-hub sensor smart-plug tv console radio-gateway usb-gateway generic-device vm container hypervisor kubernetes database web-service reverse-proxy dns-dhcp vpn storage-share backup monitoring home-automation media-server ai-runtime mqtt logs git-ci mail-server photo-library password-manager cloud-service automation generic-app rack site room lan vlan wlan internet group icon empty-atlas""".split()


def main():
    records = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
    by_id = {entry["id"]: entry for entry in records}
    if len(records) != len(IDS) or set(by_id) != set(IDS):
        raise ValueError("Expected exactly the 64 Infra motifs, app icon and empty atlas")
    output = ROOT / "ui/img/infra"
    output.mkdir(parents=True, exist_ok=True)
    manifest, provenance, decoded = [], [], {}
    for index, key in enumerate(IDS):
        record = by_id[key]
        original = Path(record["path"])
        with Image.open(original) as source:
            if "A" not in source.getbands():
                raise ValueError(f"{key}: missing generated alpha")
            rgba = source.convert("RGBA")
            if rgba.getchannel("A").getextrema() != (0, 255):
                raise ValueError(f"{key}: expected transparent margins and opaque subject")
            size = 768 if key == "empty-atlas" else 512
            rgba.thumbnail((size, size), Image.Resampling.LANCZOS)
            destination = output / f"{key}.webp"
            rgba.save(destination, "WEBP", quality=88, method=6, exact=True)
            with Image.open(destination) as packaged:
                assert packaged.mode == "RGBA", key
                assert packaged.getchannel("A").getextrema() == (0, 255), key
                decoded[key] = packaged.copy()
            manifest.append({"id": key, "family": "device" if index < 32 else "application" if index < 56 else "structure" if index < 64 else "app",
                             "label_key": "desktop.infra_icon_" + key.replace("-", "_"),
                             "path": f"/img/infra/{key}.webp", "width": rgba.width, "height": rgba.height,
                             "sha256": hashlib.sha256(destination.read_bytes()).hexdigest()})
            provenance.append({"id": key, "source_sha256": hashlib.sha256(original.read_bytes()).hexdigest(),
                               "tool": "image_gen.imagegen", "transparent_background": True,
                               "style_reference": None if key == "rack-server" else "rack-server",
                               "prompt": record.get("prompt"),
                               "style_brief": "Orthographic isometric homelab miniature; titanium silver, ceramic white, graphite recesses, small teal and copper accents; upper-left studio light; transparent margins; no text or logos." if key == "rack-server" else None})
    (output / "manifest.json").write_text(json.dumps({"version": 1, "icons": manifest}, indent=2) + "\n", encoding="utf-8")
    (output / "provenance.json").write_text(json.dumps({"generator": "Built-in Imagegen, one independent call per motif", "assets": provenance}, indent=2) + "\n", encoding="utf-8")
    previews = ROOT / "reports/infra"
    previews.mkdir(parents=True, exist_ok=True)
    for offset in range(0, len(IDS), 22):
        keys = IDS[offset:offset + 22]
        sheet = Image.new("RGB", (1120, len(keys) * 150), "#f2f4f6")
        draw = ImageDraw.Draw(sheet)
        for row, key in enumerate(keys):
            y = row * 150
            draw.text((8, y + 64), key, fill="#182832")
            for background, start in [("#f2f4f6", 180), ("#111b25", 650)]:
                draw.rectangle((start - 10, y, start + 459, y + 149), fill=background)
                for size, left in [(32, start), (64, start + 90), (128, start + 230)]:
                    tile = decoded[key].copy()
                    tile.thumbnail((size, size), Image.Resampling.LANCZOS)
                    sheet.paste(tile, (left, y + (150 - tile.height) // 2), tile)
        sheet.save(previews / f"icons-{offset // 22 + 1}.jpg", quality=92)
    print(json.dumps({"count": len(manifest), "bytes": sum(p.stat().st_size for p in output.glob("*.webp")), "previews": str(previews)}))


if __name__ == "__main__":
    main()
