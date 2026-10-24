"""Small offline checks for the maintainer-only Phonon resource helper."""
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch
import urllib.request

spec = importlib.util.spec_from_file_location("prepare_phonon", Path(__file__).with_name("prepare-phonon.py"))
prepare = importlib.util.module_from_spec(spec)
spec.loader.exec_module(prepare)


class PhononPreparationTests(unittest.TestCase):
    @unittest.skipUnless(shutil.which("powershell"), "Windows Python discovery requires PowerShell")
    def test_windows_release_python_discovery(self):
        powershell = shutil.which("powershell")
        for candidates in (("py",), ("python",), ("python3",), ("missing-py", "python3"), ("old-py", "python3"), ()):
            with self.subTest(candidates=candidates), tempfile.TemporaryDirectory(prefix="release python ") as directory:
                root = Path(directory)
                for candidate in candidates:
                    name = "py" if candidate.endswith("-py") else candidate
                    command = f'"{sys.executable}" %*'
                    if name == "py":
                        command = f'if not "%~1"=="-3" exit /b 99\n"{sys.executable}" %2 %3'
                    if candidate == "missing-py":
                        command = "echo No installed Python found! 1>&2\nexit /b 1"
                    elif candidate == "old-py":
                        command = f'"{sys.executable}" -c "import sys; sys.version_info=(3, 9); exec(sys.argv[1])" %3'
                    (root / (name + ".cmd")).write_text(f'@echo off\necho {name}>>"%~dp0calls.txt"\n{command}\n', encoding="utf-8")
                result = subprocess.run([powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File",
                                         str(prepare.REPO / "scripts/find-release-python.ps1")],
                                        cwd=root, env=dict(os.environ, PATH=str(root)), capture_output=True, text=True)
                if candidates:
                    self.assertEqual(result.returncode, 0, result.stderr)
                    self.assertEqual(Path(result.stdout.strip()), Path(sys.executable))
                    self.assertEqual((root / "calls.txt").read_text().splitlines(),
                                     ["py" if name.endswith("-py") else name for name in candidates])
                else:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("Python 3.10", result.stderr)

    @unittest.skipUnless(shutil.which("powershell"), "Windows release preflight requires PowerShell")
    def test_windows_release_python_preflight_and_pack_invocation(self):
        powershell = shutil.which("powershell")
        for script, available in (("make_release.bat", True), ("make_release.bat", False),
                                  ("make_release.ps1", True), ("make_release.ps1", False)):
            with self.subTest(script=script, available=available), tempfile.TemporaryDirectory(prefix="release preflight ") as directory:
                root = Path(directory)
                (root / "scripts").mkdir()
                shutil.copyfile(prepare.REPO / "scripts/find-release-python.ps1", root / "scripts/find-release-python.ps1")
                if available:
                    (root / "py.cmd").write_text(f'@echo off\nif not "%~1"=="-3" exit /b 99\n"{sys.executable}" %2 %3\n', encoding="utf-8")
                (root / "scripts/prepare-phonon.py").write_text("import sys\nfrom pathlib import Path\nassert sys.argv[1:] == ['--pack']\nPath('packed.txt').touch()\n", encoding="utf-8")
                sentinel = root / "keep.dat"
                sentinel.write_bytes(b"existing release artifact")
                source = (prepare.REPO / script).read_text(encoding="utf-8")
                self.assertLess(source.index("find-release-python.ps1"), source.index("Get-ChildItem -LiteralPath 'deploy'"))
                if script.endswith(".bat"):
                    block = re.search(r'(set "RELEASE_PYTHON=".*?^echo     Python:[^\n]*\n)', source, re.M | re.S)
                    self.assertIsNotNone(block, "Python preflight block not found")
                    pack = re.search(r'^"!RELEASE_PYTHON!"[^\n]*', source, re.M)
                    self.assertIsNotNone(pack, "verified Python pack invocation not found")
                    runner = root / script
                    runner.write_text('@echo off\nsetlocal enabledelayedexpansion\n' + block[1] + pack[0] + '\nif errorlevel 1 exit /b 1\ndel "%~dp0keep.dat"\n', encoding="utf-8")
                    command = [os.environ["COMSPEC"], "/d", "/c", str(runner)]
                else:
                    block = re.search(r'(^\$releasePython = .*?^Write-Host "    Python:[^\n]*\n)', source, re.M | re.S)
                    self.assertIsNotNone(block, "Python preflight block not found")
                    pack = re.search(r'^& \$releasePython[^\n]*', source, re.M)
                    self.assertIsNotNone(pack, "verified Python pack invocation not found")
                    runner = root / script
                    runner.write_text("$ErrorActionPreference = 'Stop'\n$scriptDir = $PSScriptRoot\n" + block[1] + pack[0] + "\nif ($LASTEXITCODE -ne 0) { exit 1 }\nRemove-Item -LiteralPath (Join-Path $PSScriptRoot 'keep.dat')\n", encoding="utf-8")
                    command = [powershell, "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", str(runner)]
                result = subprocess.run(command, cwd=root, env=dict(os.environ, PATH=os.pathsep.join((str(root), str(Path(powershell).parent)))),
                                        capture_output=True, text=True)
                if available:
                    self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                    self.assertTrue((root / "packed.txt").exists())
                    self.assertFalse(sentinel.exists())
                else:
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("Python 3.10", result.stdout + result.stderr)
                    self.assertEqual(sentinel.read_bytes(), b"existing release artifact")
                    self.assertFalse((root / "packed.txt").exists())

    @unittest.skipUnless(shutil.which("powershell"), "Windows release cleanup requires PowerShell")
    def test_windows_release_cleanup_preserves_prepared_resources(self):
        for script in ("make_release.bat", "make_release.ps1"):
            with self.subTest(script=script), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                keep = ("deploy/phonon/model/checkpoint.bin", "deploy/docker/Dockerfile")
                stale = ("deploy/old [version].dat", "deploy/old-runtime/ptts")
                for name in keep + stale:
                    path = root / name
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_bytes(b"fixture")
                source = (prepare.REPO / script).read_text(encoding="utf-8")
                if script.endswith(".bat"):
                    match = re.search(r'^powershell .*?-command "([^"]*Get-ChildItem[^\n]*)"$', source, re.M)
                else:
                    match = re.search(r'(^if \(Test-Path "deploy"\) \{.*?^New-Item[^\n]*"deploy"[^\n]*)', source, re.M | re.S)
                self.assertIsNotNone(match, "release cleanup block not found")
                subprocess.run(["powershell", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference='Stop'; " + match[1]], cwd=root, check=True, capture_output=True)
                for name in keep:
                    self.assertEqual((root / name).read_bytes(), b"fixture")
                for name in stale:
                    self.assertFalse((root / name).exists())

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
        with self.assertRaises(ValueError):
            prepare.verify(data, {"size": len(data), "sha256": ""})

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

    def test_latest_uses_pinned_package_without_application_release_checksums(self):
        content = b"pinned model config"
        source = {"name": "config.json", "size": len(content), "sha256": hashlib.sha256(content).hexdigest()}
        raw = io.BytesIO()
        with tarfile.open(fileobj=raw, mode="w:gz") as archive:
            entry = tarfile.TarInfo("phonon/" + prepare.BUNDLE_ID + "/model/config.json")
            entry.size = len(content)
            archive.addfile(entry, io.BytesIO(content))
        payload = raw.getvalue()
        package = {"url": "https://github.com/antibyte/s2s/releases/download/fixture/phonon-model.dat",
                   "size": len(payload), "sha256": hashlib.sha256(payload).hexdigest()}
        manifest = {"model_files": [source], "voices": [], "model_package": package}
        with tempfile.TemporaryDirectory() as directory, \
                patch.object(prepare, "STAGE", Path(directory)), patch.object(prepare, "MANIFEST", manifest):
            destination = prepare.STAGE / "model/config.json"
            destination.parent.mkdir()
            for label, data in (("size", payload[:-1]), ("checksum", b"x" * len(payload)), ("valid", payload)):
                with self.subTest(label=label), patch.object(prepare, "fetch", return_value=data) as fetch:
                    destination.write_bytes(b"previous resource")
                    if label == "valid":
                        prepare.release_model("latest")
                        self.assertEqual(destination.read_bytes(), content)
                    else:
                        with self.assertRaises(ValueError):
                            prepare.release_model("latest")
                        self.assertEqual(destination.read_bytes(), b"previous resource")
                    fetch.assert_called_once_with(package["url"], package["size"])
            with patch.object(prepare, "fetch") as fetch:
                prepare.release_model("latest")
                fetch.assert_not_called()


if __name__ == "__main__":
    unittest.main()
