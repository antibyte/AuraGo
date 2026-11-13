"""Reproduce pet notices without Blender or model regeneration."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("model_export", ROOT / "assets/desktop-pets/model_export.py")
EXPORT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(EXPORT)


class PetNoticesTest(unittest.TestCase):
    def test_scoped_notices_preserve_models_and_license_texts(self):
        catalog = json.loads((ROOT / "assets/desktop-pets/catalog.json").read_text(encoding="utf-8"))
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "assets/desktop-pets"
            source.mkdir(parents=True)
            (source / "catalog.json").write_text(json.dumps(catalog), encoding="utf-8")
            punk = root / "ui/3d/pets/punk"
            punk.mkdir(parents=True)
            for name in ["LICENSE-CC0.txt", "LICENSE-MIT.txt"]:
                (punk / name).write_bytes((ROOT / "ui/3d/pets/punk" / name).read_bytes())
            for pet, entry in catalog.items():
                with self.subTest(pet=pet):
                    out = root / f"ui/3d/pets/{pet}"
                    out.mkdir(parents=True, exist_ok=True)
                    (out / "model.glb").write_bytes(b"unchanged model fixture")
                    EXPORT.write_notices(root, pet)
                    notice = (out / "NOTICE.txt").read_bytes()
                    self.assertEqual(notice, (ROOT / f"ui/3d/pets/{pet}/NOTICE.txt").read_bytes())
                    self.assertIn(b"Geometry, textures and character rigs:", notice)
                    self.assertIn(b"do not license or relicense these assets", notice)
                    self.assertEqual((out / "model.glb").read_bytes(), b"unchanged model fixture")
                    if entry["kind"] == "humanoid":
                        for name in ["LICENSE-CC0.txt", "LICENSE-MIT.txt"]:
                            self.assertEqual((out / name).read_bytes(), (punk / name).read_bytes())
                            self.assertEqual((ROOT / f"ui/3d/pets/{pet}" / name).read_bytes(), (punk / name).read_bytes())
                    else:
                        self.assertIn(b"GPL-3.0-only", notice)
                        self.assertFalse((out / "LICENSE-CC0.txt").exists())


if __name__ == "__main__":
    unittest.main()
