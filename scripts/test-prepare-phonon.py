"""Small offline checks for the maintainer-only Phonon resource helper."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import urllib.request

spec = importlib.util.spec_from_file_location("prepare_phonon", Path(__file__).with_name("prepare-phonon.py"))
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class PhononPreparationTests(unittest.TestCase):
    def test_failed_atomic_write_keeps_previous_resource_and_cleans_temp(self):
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / "resource.bin"
            destination.write_bytes(b"previous verified resource")
            with patch.object(prepare.os, "fsync", side_effect=OSError("fixture sync failure")):
                with self.assertRaises(OSError):
                    prepare.write(destination, b"new resource")
            self.assertEqual(destination.read_bytes(), b"previous verified resource")
            self.assertEqual(list(Path(directory).iterdir()), [destination])

    def test_rejects_changed_source_bytes(self):
        data = b"pinned resource"
        sha = hashlib.sha256(data).hexdigest()
        expected = {"size": len(data), "sha256": sha}
        prepare.verify(data, expected)
        for changed in (data[:-1], b"x" * len(data), data + b"x"):
            with self.assertRaises(ValueError):
                prepare.verify(changed, expected)
        git = hashlib.sha1(b"blob " + str(len(data)).encode() + b"\0" + data).hexdigest()
        prepare.verify(data, {"size": len(data), "git_blob_sha1": git})

    def test_redirect_drops_credentials(self):
        request = urllib.request.Request("https://huggingface.co/model", headers={"Authorization": "Bearer fixture-token"})
        redirected = prepare.CredentialSafeRedirect().redirect_request(request, None, 302, "Found", {}, "https://cdn-lfs.hf.co/model")
        self.assertFalse(redirected.has_header("Authorization"))
        with self.assertRaises(ValueError):
            prepare.fetch("https://example.com", 1, "fixture-token\nprivate")

    def test_runtime_archive_rejects_links_and_duplicates(self):
        for kind in ("link", "duplicate"):
            data = io.BytesIO()
            with tarfile.open(fileobj=data, mode="w:gz") as archive:
                entry = tarfile.TarInfo("ptts")
                if kind == "link":
                    entry.type, entry.linkname = tarfile.SYMTYPE, "/tmp/escaped"
                    archive.addfile(entry)
                else:
                    archive.addfile(entry, io.BytesIO())
                    archive.addfile(entry, io.BytesIO())
            with self.assertRaises(ValueError):
                prepare.runtime_binary(data.getvalue(), "runtime.tar.gz", "ptts")

    def test_pack_uses_versioned_paths_and_file_hashes(self):
        with tempfile.TemporaryDirectory() as directory:
            stage = Path(directory)
            old_stage = prepare.STAGE
            prepare.STAGE = stage
            try:
                resource = stage / "model" / "config.json"
                resource.parent.mkdir()
                resource.write_bytes(b"{}")
                (stage / "token.env").write_bytes(b"private fixture")
                archive_path = stage / "phonon-model.dat"
                prepare.pack(archive_path, [resource])
                with tarfile.open(archive_path, "r:gz") as archive:
                    names = archive.getnames()
                    self.assertEqual(len(names), 2)
                    self.assertTrue(all(name.startswith("phonon/" + prepare.BUNDLE_ID + "/") for name in names))
                    self.assertFalse(any("token" in name for name in names))
                    self.assertIn(b'"model/config.json"', archive.extractfile(names[-1]).read())
            finally:
                prepare.STAGE = old_stage

    def test_release_bundle_checks_each_source_and_never_extracts_extra_files(self):
        with tempfile.TemporaryDirectory() as directory:
            stage = Path(directory)
            original = prepare.STAGE, prepare.MANIFEST, prepare.fetch
            content = b"pinned model config"
            source = {"name":"config.json", "size":len(content), "sha256":hashlib.sha256(content).hexdigest()}
            prepare.STAGE = stage
            prepare.MANIFEST = {"model_files":[source], "voices":[]}
            raw = io.BytesIO()
            with tarfile.open(fileobj=raw, mode="w:gz") as archive:
                for name, data in (("model/config.json", content), ("../../token.env", b"private fixture")):
                    info = tarfile.TarInfo("phonon/" + prepare.BUNDLE_ID + "/" + name)
                    info.size = len(data)
                    archive.addfile(info, io.BytesIO(data))
            payload = raw.getvalue()
            prepare.fetch = lambda url, limit: (hashlib.sha256(payload).hexdigest() + "  phonon-model.dat\n").encode() if url.endswith("SHA256SUMS") else payload
            try:
                prepare.release_model("fixture")
                self.assertEqual((stage / "model/config.json").read_bytes(), content)
                self.assertEqual([file.relative_to(stage).as_posix() for file in stage.rglob("*") if file.is_file()], ["model/config.json"])
            finally:
                prepare.STAGE, prepare.MANIFEST, prepare.fetch = original


if __name__ == "__main__":
    unittest.main()
