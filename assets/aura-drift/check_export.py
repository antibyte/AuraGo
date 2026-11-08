"""Blender self-check: failed GLB exports preserve the shipped file (MIT)."""
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace

sys.path.insert(0, str(Path(__file__).resolve().parent))
from kit import export


class Exporter:
    fail = True

    def get_rna_type(self):
        return SimpleNamespace(properties={"filepath": None})

    def __call__(self, **options):
        Path(options["filepath"]).write_bytes(b"partial" if self.fail else b"complete")
        if self.fail:
            raise RuntimeError("Fixture export failure")


def check():
    original = export.bpy
    operator = Exporter()
    export.bpy = SimpleNamespace(
        ops=SimpleNamespace(object=SimpleNamespace(select_all=lambda **kwargs: None),
                            export_scene=SimpleNamespace(gltf=operator)),
        context=SimpleNamespace(view_layer=SimpleNamespace(objects=SimpleNamespace(active=None))))
    model = SimpleNamespace(objects=[SimpleNamespace(select_set=lambda value: None)])
    try:
        with tempfile.TemporaryDirectory(prefix="drift-export-check-") as scratch:
            target = Path(scratch) / "car.glb"
            target.write_bytes(b"original")
            try:
                export.export_glb(model, str(target))
            except RuntimeError as error:
                assert str(error) == "Fixture export failure"
            else:
                raise AssertionError("Expected export failure")
            assert target.read_bytes() == b"original", "Failed export damaged the existing model"
            assert os.listdir(scratch) == ["car.glb"], "Failed export left staging files"
            operator.fail = False
            export.export_glb(model, str(target))
            assert target.read_bytes() == b"complete"
            assert os.listdir(scratch) == ["car.glb"], "Successful export left staging files"
    finally:
        export.bpy = original
    print("Verified atomic GLB replacement and failed-export preservation")


if __name__ == "__main__":
    check()
