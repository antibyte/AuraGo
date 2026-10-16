"""Offline release-gate regression checks; fixtures are never published."""
import hashlib
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock
import zipfile

HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('release_catalog', HERE / 'release-catalog.py')
catalog = importlib.util.module_from_spec(spec)
spec.loader.exec_module(catalog)

class ReleaseGateTests(unittest.TestCase):
    def fixture(self, directory, mutation=None):
        binary = directory / 'aurago-decisions-runtime'
        binary.write_bytes(b'protocol fixture, not an executable')
        (directory / 'LICENSE.txt').write_text('fixture license')
        subprocess.run([sys.executable, str(HERE.parent / 'write-decisions-runtime-manifest.py'),
                        '--binary', str(binary), '--target', 'linux-amd64', '--backend', 'cpu', '--release'],
                       check=True, stdout=subprocess.DEVNULL)
        manifest_path = directory / 'release-manifest.json'
        manifest = json.loads(manifest_path.read_text())
        files = {path.name: path.read_bytes() for path in directory.iterdir()}
        if mutation: mutation(manifest, files)
        files['release-manifest.json'] = json.dumps(manifest).encode()
        archive = directory / 'bundle.zip'
        with zipfile.ZipFile(archive, 'w') as bundle:
            for name, content in files.items(): bundle.writestr(name, content)
        record = dict(target='linux-amd64', backend='cpu', size=archive.stat().st_size,
                      sha256=hashlib.sha256(archive.read_bytes()).hexdigest(),
                      manifest_sha256=hashlib.sha256(files['release-manifest.json']).hexdigest(),
                      unpacked_size=sum(map(len, files.values())))
        return record, archive

    def test_complete_package(self):
        with tempfile.TemporaryDirectory() as temporary:
            catalog.verify_archive(*self.fixture(Path(temporary)))

    def test_rejects_bad_inventory_paths_and_pins(self):
        mutations = [
            lambda manifest, files: files.update({'../outside': b'escape'}),
            lambda manifest, files: files.update({'injected.dll': b'unlisted'}),
            lambda manifest, files: files.update({'LICENSE.txt': b'changed'}),
            lambda manifest, files: manifest['models'][0].update(revision='main'),
            lambda manifest, files: manifest['build'].update(llama_cpp_commit='untrusted'),
            lambda manifest, files: manifest.update(files=manifest['files'][1:]),
        ]
        for mutation in mutations:
            with self.subTest(mutation=mutations.index(mutation)), tempfile.TemporaryDirectory() as temporary:
                with self.assertRaises(ValueError): catalog.verify_archive(*self.fixture(Path(temporary), mutation))

    def test_matrix_covers_every_approved_package(self):
        self.assertEqual(len(catalog.MATRIX), 6)
        self.assertEqual(sum(map(len, catalog.MATRIX.values())), 15)
        self.assertEqual(catalog.MATRIX['linux-arm64'], ['cuda', 'vulkan', 'cpu'])
        self.assertEqual(catalog.MATRIX['windows-arm64'], ['vulkan', 'cpu'])

    def test_anonymous_pulls_use_distinct_platform_manifests(self):
        repository = 'ghcr.io/antibyte/aurago-decisions-runtime'
        reference = repository + '@sha256:' + 'a' * 64
        members = [dict(platform=dict(os='linux', architecture=arch), digest='sha256:' + char * 64)
                   for arch, char in [('amd64', 'b'), ('arm64', 'c')]]
        stored = {}
        def pull(command, **options):
            image, platform = command[-1], command[-2]
            if image in stored and stored[image] != platform:
                raise RuntimeError('classic image store cannot overwrite a digest with another architecture')
            stored[image] = platform
            self.assertTrue(options['check'])
            self.assertIn('decisions-anonymous-docker-', options['env']['DOCKER_CONFIG'])
        with mock.patch.object(catalog.subprocess, 'check_output', return_value=json.dumps(dict(manifests=members)).encode()), \
                mock.patch.object(catalog.subprocess, 'run', side_effect=pull):
            catalog.verify_public(dict(packages=[], images=[dict(reference=reference)]))
        self.assertEqual(stored, {repository + '@' + member['digest']: 'linux/' + member['platform']['architecture']
                                  for member in members})

    def test_rejects_ambiguous_or_unpinned_image_manifests(self):
        valid = [dict(platform=dict(os='linux', architecture=arch), digest='sha256:' + char * 64)
                 for arch, char in [('amd64', 'b'), ('arm64', 'c')]]
        invalid = [valid + [valid[0]], [valid[0], dict(platform=valid[1]['platform'], digest='latest')], valid[:1]]
        for members in invalid:
            with self.subTest(members=members), \
                    mock.patch.object(catalog.subprocess, 'check_output', return_value=json.dumps(dict(manifests=members)).encode()), \
                    mock.patch.object(catalog.subprocess, 'run') as pull:
                with self.assertRaises(ValueError):
                    catalog.verify_public(dict(packages=[], images=[dict(reference='ghcr.io/antibyte/aurago-decisions-runtime@sha256:' + 'a' * 64)]))
                pull.assert_not_called()

    def test_insecure_redirect_is_rejected(self):
        with self.assertRaises(ValueError):
            catalog.HTTPSRedirect().redirect_request(None, None, 302, '', {}, 'http://example.invalid/file')

if __name__ == '__main__':
    unittest.main()
