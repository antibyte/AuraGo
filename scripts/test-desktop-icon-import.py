#!/usr/bin/env python3
"""Trust-boundary regression tests for the curated icon importer (stdlib only)."""
import importlib.util
import io
import json
import tarfile
import tempfile
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location("icon_import", Path(__file__).with_name("import-desktop-icon-themes.py"))
icons = importlib.util.module_from_spec(spec)
spec.loader.exec_module(icons)


class ImportSafetyTests(unittest.TestCase):
    def test_control_contrast_corrections(self):
        def luminance(color):
            channels = [int(color[i:i + 2], 16) / 255 for i in (1, 3, 5)]
            linear = [c / 12.92 if c <= .04045 else ((c + .055) / 1.055) ** 2.4 for c in channels]
            return sum(c * w for c, w in zip(linear, (.2126, .7152, .0722)))
        sources = json.loads((icons.INPUT / 'sources.json').read_text())
        for theme, spec in sources.items():
            for scheme, replacements in spec.get('action_colors', {}).items():
                background = luminance('#f9f8f6' if scheme == 'light' else '#191d23')
                for color in replacements.values():
                    ink = luminance(color)
                    with self.subTest(theme=theme, scheme=scheme, color=color):
                        self.assertGreaterEqual((max(ink, background) + .05) / (min(ink, background) + .05), 3)

    def test_svg_rejects_active_or_external_content(self):
        for body in ['<script>alert(1)</script>', '<path onclick="x()"/>', '<image href="https://example.com/a.png"/>', '<use href="../file.svg#x"/>', '<path fill="url(https://example.com/x)"/>', '<style>@import "https://example.com/x";</style>', '<path fill="url(#missing)"/>']:
            with self.subTest(body=body), self.assertRaises(ValueError):
                icons.safe_svg(('<svg xmlns="http://www.w3.org/2000/svg">' + body + '</svg>').encode())
        with self.assertRaises(ValueError):
            icons.safe_svg(b'<!DOCTYPE svg [<!ENTITY x SYSTEM "file:///etc/passwd">]><svg xmlns="http://www.w3.org/2000/svg">&x;</svg>')

    def test_local_gradients_and_editor_metadata(self):
        svg = b'<svg xmlns="http://www.w3.org/2000/svg" xmlns:editor="urn:editor"><editor:metadata/><defs><linearGradient id="g"><stop stop-color="#fff"/></linearGradient></defs><path fill="url(#g)" d="M0 0h2v2z"/></svg>'
        result = icons.safe_svg(svg)
        self.assertNotIn(b'urn:editor', result)
        self.assertIn(b'url(#g)', result)
        self.assertEqual(icons.safe_svg(result), result)

    def test_paths(self):
        for path in ['/absolute.svg', '../escape.svg', 'a/../escape.svg', r'a\escape.svg', 'C:/escape.svg']:
            with self.subTest(path=path), self.assertRaises(ValueError):
                icons.safe_path(path)

    def test_link_overlay_cycle_escape_and_checksum(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'icons.tar'
            with tarfile.open(path, 'w') as archive:
                data = b'<svg xmlns="http://www.w3.org/2000/svg"/>'
                item = tarfile.TarInfo('src/16/places/real.svg')
                item.size = len(data)
                archive.addfile(item, io.BytesIO(data))
                for name, target in [('folder', 'real.svg'), ('cycle', 'cycle.svg'), ('escape', '../../../../outside.svg')]:
                    link = tarfile.TarInfo('links/16/places/' + name + '.svg')
                    link.type, link.linkname = tarfile.SYMTYPE, target
                    archive.addfile(link)
            archive = icons.Archive(path, {'sha256': icons.digest(path.read_bytes()), 'overlay': True})
            self.assertEqual(archive.read('links/16/places/folder.svg')[1], 'src/16/places/real.svg')
            for name in ['cycle', 'escape']:
                with self.assertRaises(ValueError):
                    archive.read('links/16/places/' + name + '.svg')
            with self.assertRaises(ValueError):
                icons.Archive(path, {'sha256': 'wrong'})


if __name__ == '__main__':
    unittest.main()
