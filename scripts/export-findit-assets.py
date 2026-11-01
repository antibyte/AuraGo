"""Export the original FindIt atlas without changing its artwork."""
from __future__ import annotations

import argparse
import hashlib
import io
import json
from pathlib import Path

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
IDS = "resistor capacitor inductor potentiometer diode led transistor mosfet opamp comparator regulator logic microcontroller memory adc interface sensor optocoupler relay switch connector fuse crystal oscillator display motor fan devboard radio cpu mainboard ram gpu ssd hdd psu cooler case expansion network sbc cable adapter tool meter battery fastener generic dip sip soic tssop sot to dpak qfp qfn bga lga plcc axial radial chip storage".split()


def webp(image: Image.Image) -> bytes:
    output = io.BytesIO()
    image.save(output, "WEBP", lossless=True, method=6, exact=True)
    return output.getvalue()


def outputs() -> dict[Path, bytes]:
    source = ROOT / "assets/findit/sources/miniatures-v1.png"
    master = Image.open(source).convert("RGBA")
    assert master.width == master.height and len(IDS) == len(set(IDS)) == 64
    assert master.getchannel("A").getextrema() == (0, 255)
    atlas = master.resize((1024, 1024), Image.Resampling.LANCZOS)
    atlas_bytes = webp(atlas)
    manifest = {
        "version": 1, "atlas": "/img/findit/atlas.webp", "columns": 8, "rows": 8,
        "cell_size": 128, "source_sha256": hashlib.sha256(source.read_bytes()).hexdigest(),
        "atlas_sha256": hashlib.sha256(atlas_bytes).hexdigest(),
        "icons": [{"id": name, "index": index} for index, name in enumerate(IDS)],
    }
    result = {
        ROOT / "ui/img/findit/atlas.webp": atlas_bytes,
        ROOT / "ui/img/findit/manifest.json": (json.dumps(manifest, indent=2) + "\n").encode(),
    }
    icon_source = ROOT / "assets/findit/sources/icon-v1.png"
    if icon_source.exists():
        icon = Image.open(icon_source).convert("RGBA").resize((256, 256), Image.Resampling.LANCZOS)
        result[ROOT / "ui/img/findit/icon.webp"] = webp(icon)
    return result


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    for path, data in outputs().items():
        if args.check:
            assert path.exists() and path.read_bytes() == data, f"Stale FindIt asset: {path}"
        else:
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
    print("FindIt assets verified" if args.check else "FindIt assets exported")


if __name__ == "__main__":
    main()
