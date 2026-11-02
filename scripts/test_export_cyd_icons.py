"""Focused regression checks for atlas slicing and tintable frame exports."""
import importlib.util
import io
from pathlib import Path
import tempfile
import unittest

from PIL import Image

SPEC = importlib.util.spec_from_file_location("cyd_icons", Path(__file__).with_name("export-cyd-icons.py"))
ICONS = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(ICONS)


class IconExports(unittest.TestCase):
    def test_narrow_mono_icon_preserves_original_pngs(self):
        frames = ICONS.cells(ICONS.SOURCE / "sober-mono.png", 10, 6)
        frame = frames[ICONS.ORIGINAL_MOTIFS.index("thermometer")]
        for size in ICONS.SIZES:
            with self.subTest(size=size):
                exported = ICONS.export([frame.copy()], size, "mono")[0]
                buffer = io.BytesIO()
                exported.save(buffer, format="PNG", optimize=False, compress_level=9)
                original = ICONS.OUTPUT / f"sober/mono/thermometer/{size}/0.png"
                self.assertEqual(buffer.getvalue(), original.read_bytes())

    def test_expansion_uses_each_rows_gutters(self):
        atlas = Image.new("RGBA", (20, 20))
        # No single vertical cut can separate both rows without losing artwork.
        for box in ((1, 1, 5, 5), (9, 1, 12, 5), (7, 12, 13, 16), (16, 12, 19, 16)):
            atlas.paste((16, 46, 70, 255), box)
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "sample-expansion-1.png"
            atlas.save(path)
            cells = ICONS.cells(path, 2, 2)
        self.assertEqual([sum(a > 0 for a in cell.getchannel("A").get_flattened_data()) for cell in cells], [16, 12, 24, 12])
        self.assertEqual(len({cell.size for cell in cells}), 1)

    def test_union_preserves_motion_and_tintable_negative_space(self):
        frames = []
        for x in range(6):
            frame = Image.new("RGBA", (48, 48))
            frame.paste((16, 46, 70, 255), (10, 10, 38, 38))
            frame.paste((0, 0, 0, 0), (14 + x, 18, 20 + x, 30))
            frames.append(frame)
        exported = ICONS.export(frames, 24, "mono")
        self.assertEqual(len({frame.getchannel("A").tobytes() for frame in exported}), 6)
        self.assertEqual(len({frame.getbbox() for frame in exported}), 1)
        for frame in exported:
            self.assertEqual(frame.size, (24, 24))
            self.assertEqual(frame.getpixel((0, 0))[3], 0)
            self.assertTrue(all((r, g, b) == (255, 255, 255) for r, g, b, a in frame.get_flattened_data() if a))

    def test_opaque_master_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "bad-expansion-1.png"
            Image.new("RGBA", (40, 40), "navy").save(path)
            with self.assertRaisesRegex(AssertionError, "Missing transparency"):
                ICONS.cells(path, 2, 2)


if __name__ == "__main__":
    unittest.main()
