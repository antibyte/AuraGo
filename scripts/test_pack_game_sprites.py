import contextlib
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest
from unittest import mock

from PIL import Image

import pack_game_sprites as packer


EXTERNAL_IDS = {
    "aurago-effects",
    "aurago-sounds",
    "aurago-pirates-3d",
    "aurago-pirates-topdown",
    "aurago-pirates-side",
    "aurago-isometric",
}


class SpriteCatalogTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.packs = self.root / "internal/gamemaker/asset_packs"
        self.production = self.packs / "production"
        self.production.mkdir(parents=True)
        (self.root / "LICENSE").write_text("test license\n", encoding="utf8")

        self.old_root, self.old_packs = packer.ROOT, packer.PACKS
        packer.ROOT, packer.PACKS = self.root, self.packs
        self.addCleanup(setattr, packer, "ROOT", self.old_root)
        self.addCleanup(setattr, packer, "PACKS", self.old_packs)

        art = Image.new("RGBA", (30, 30), (20, 30, 40, 255))
        art.putpixel((0, 0), (0, 0, 0, 0))
        art.save(self.production / "art.png")

        assets = [
            {
                "id": f"prop-{i:03d}", "name": f"Prop {i}", "description": "Static side-view prop",
                "tags": ["everyday"], "view": "side", "entity": f"prop-{i:03d}", "action": "static",
                "transform": {"mode": "fixed", "flip_x": False, "flip_y": False},
                "source": "art", "source_frames": [i],
            }
            for i in range(100)
        ]
        definition = {
            "id": "mixed-test", "name": "Mixed Test", "description": "Test pack", "tags": ["test"],
            "version": "1", "sources": {"art": {"file": "art.png", "columns": 10, "rows": 10}},
            "assets": assets,
        }
        self.manifest = self.production / "manifest.json"
        self.manifest.write_text(json.dumps([definition], indent=2) + "\n", encoding="utf8")
        (self.packs / "aurago-low-poly").mkdir()
        self.model = {
            "id": "aurago-low-poly", "name": "Canonical Model", "description": "Model manifest",
            "tags": ["3d"], "version": "1.0.0", "kind": "model3d",
        }
        (self.packs / "aurago-low-poly/manifest.json").write_text(json.dumps(self.model), encoding="utf8")

        current_catalog = json.loads((Path(__file__).resolve().parents[1] / "internal/gamemaker/asset_packs/catalog.json").read_text(encoding="utf8"))
        self.external = [item for item in current_catalog if item.get("id") in EXTERNAL_IDS]
        self.assertEqual({item["id"] for item in self.external}, EXTERNAL_IDS)
        self.catalog = self.packs / "catalog.json"
        stale_rows = [
            {"id": "mixed-test", "name": "Stale generated row", "version": "old", "kind": "sprite2d"},
            {**self.model, "version": "old"},
        ]
        duplicate = {**self.external[0], "description": "duplicate must not replace the first row"}
        self.catalog.write_text(json.dumps(self.external + stale_rows + [duplicate], indent=2) + "\n", encoding="utf8")

    def run_packer(self, check=False):
        argv = ["pack_game_sprites.py", "--manifest", str(self.manifest), "--source-root", str(self.production)]
        if check:
            argv.append("--check")
        with mock.patch.object(sys, "argv", argv), contextlib.redirect_stdout(io.StringIO()):
            packer.main()

    def test_catalog_merge_preserves_curated_rows_and_check_is_stable(self):
        self.run_packer()
        catalog_bytes = self.catalog.read_bytes()
        catalog = json.loads(catalog_bytes)
        by_id = {item["id"]: item for item in catalog}

        self.assertEqual(len(catalog), len(by_id))
        for record in self.external:
            self.assertEqual(by_id[record["id"]], record)
        self.assertEqual(by_id["aurago-low-poly"], self.model)
        self.assertEqual(by_id["mixed-test"]["version"], "1")

        self.run_packer(check=True)
        self.assertEqual(self.catalog.read_bytes(), catalog_bytes)

        by_id["mixed-test"]["version"] = "tampered"
        self.catalog.write_text(json.dumps(catalog, indent=2) + "\n", encoding="utf8")
        with self.assertRaisesRegex(ValueError, "Generated sprite artifact differs"):
            self.run_packer(check=True)

    def test_expanded_grid_preserves_cells_and_explicit_animation_timing(self):
        definition = json.loads(self.manifest.read_text(encoding="utf8"))[0]
        definition["rows"] = 12
        first = definition["assets"][0]
        first.update(action="walk", source_frames=[0, 1, 2], animation_frames=[2, 0, 2, 0], frame_rate=7)
        definition["assets"][1:3] = []
        definition["assets"].extend(
            {**definition["assets"][-1], "id": f"extra-{i}", "source_frames": [i]}
            for i in range(20)
        )
        metadata = packer.pack(definition, self.production)
        self.assertEqual(len(metadata["frames"]), 120)
        self.assertEqual(metadata["assets"][0]["frames"], [0, 1, 2])
        animation = metadata["animations"][0]
        self.assertEqual(animation["frames"], [2, 0, 2, 0])
        self.assertEqual(animation["frame_rate"], 7)
        self.assertEqual(animation["repeat"], -1)
        with Image.open(self.packs / "mixed-test/sheet.png") as image:
            self.assertEqual(image.size, (640, 768))
        packer.pack(definition, self.production, check=True)

    def test_invalid_animation_controls_fail_before_publishing(self):
        definition = json.loads(self.manifest.read_text(encoding="utf8"))[0]
        asset = definition["assets"][0]
        asset["action"] = "idle"
        for field, value in [("animation_frames", []), ("animation_frames", [1]),
                             ("animation_frames", [-1]), ("animation_frames", [True]),
                             ("frame_rate", 0), ("frame_rate", 61), ("frame_rate", True)]:
            with self.subTest(field=field, value=value):
                asset.pop("animation_frames", None)
                asset.pop("frame_rate", None)
                asset[field] = value
                with self.assertRaisesRegex(ValueError, "Invalid animation"):
                    packer.pack(definition, self.production)
                self.assertFalse((self.packs / "mixed-test/sheet.json").exists())

    def test_failed_replacement_preserves_previous_artifact(self):
        path = self.production / "artifact.bin"
        path.write_bytes(b"previous")
        with mock.patch("os.replace", side_effect=OSError("replacement failed")):
            with self.assertRaisesRegex(OSError, "replacement failed"):
                packer.write_output(path, b"updated", False)
        self.assertEqual(path.read_bytes(), b"previous")
        self.assertEqual(list(self.production.glob(".sprite-*.tmp")), [])
        with mock.patch("tempfile.NamedTemporaryFile", side_effect=AssertionError("unchanged output was rewritten")):
            packer.write_output(path, b"previous", False)


if __name__ == "__main__":
    unittest.main()
