#!/usr/bin/env python3
"""Validate release artifacts and generate pins only after public verification."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parents[2]
MATRIX = {target: backends for target, backends in [
    ('windows-amd64', ['cuda', 'vulkan', 'cpu']), ('windows-arm64', ['vulkan', 'cpu']),
    ('linux-amd64', ['cuda', 'vulkan', 'cpu']), ('linux-arm64', ['cuda', 'vulkan', 'cpu']),
    ('darwin-amd64', ['metal', 'cpu']), ('darwin-arm64', ['metal', 'cpu'])]}

class HTTPSRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, headers, new_url):
        if not new_url.startswith('https://'):
            raise ValueError('Insecure artifact redirect')
        return super().redirect_request(request, fp, code, message, headers, new_url)

def digest(stream):
    return hashlib.file_digest(stream, 'sha256').hexdigest()

def verify_archive(record, archive):
    with archive.open('rb') as stream:
        if archive.stat().st_size != record['size'] or digest(stream) != record['sha256']:
            raise ValueError('Archive size or digest mismatch')
    with zipfile.ZipFile(archive) as bundle:
        entries = bundle.infolist()
        names = [entry.filename for entry in entries]
        if len(names) > 1024 or len(names) != len(set(name.lower() for name in names)) or any(not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._+-]*', name) or name.endswith('.') for name in names):
            raise ValueError('Non-portable or duplicate archive members')
        if any(entry.is_dir() or entry.external_attr >> 16 & 0o170000 == 0o120000 for entry in entries):
            raise ValueError('Archive must contain regular files only')
        if sum(entry.file_size for entry in entries) != record['unpacked_size'] or record['unpacked_size'] > 8 << 30:
            raise ValueError('Unexpected extracted size')
        raw = bundle.read('release-manifest.json')
        if hashlib.sha256(raw).hexdigest() != record['manifest_sha256']:
            raise ValueError('Manifest digest mismatch')
        manifest = json.loads(raw)
        if manifest['schema'] != 'aurago.decisions.runtime-manifest.v2' or manifest['protocol'] != 'aurago.decisions.v1':
            raise ValueError('Unsupported runtime protocol or manifest')
        build = manifest['build']
        if build['target'] != record['target'] or build['backend'] != record['backend'] or build['llama_cpp_commit'] != 'd81235049384534c167caea52b85a694f6103d14' or build['llama_cpp_patch_sha256'] != 'a3e895bce9fafa23873a481ae29f31fdb98287e0f02939bdd1213c103271a790':
            raise ValueError('Runtime build identity mismatch')
        if set(names) != {'release-manifest.json', *(member['file'] for member in manifest['files'])}:
            raise ValueError('Manifest inventory mismatch')
        artifact = manifest['artifact']
        executable = 'aurago-decisions-runtime' + ('.exe' if record['target'].startswith('windows-') else '')
        if artifact['file'] != executable or artifact not in manifest['files']:
            raise ValueError('Executable missing from inventory')
        expected_models = {
            'model': ('jeff-2B.Q4_K_M.gguf', 'd4d9fbcf40345a1677dc92596c7c1e157fac5cba98e2a1b14ec92db94c67d46b', 1274396448),
            'projector': ('jeff-2B.mmproj.gguf', '1601a51a07942318aacb7a7298847fe505f8beb592be37f538ab85577772bb68', 671372608)}
        if len(manifest['models']) != 2: raise ValueError('Invalid model pins')
        for model in manifest['models']:
            if model['repo'] != 'jersobh/jeff-2b-gguf' or model['revision'] != '263ac27c59688a360d2db255c4f7e57f665bcca4' or expected_models.pop(model['role'], None) != (model['file'], model['sha256'], model['size_bytes']):
                raise ValueError('Model pin mismatch')
        for member in manifest['files']:
            with bundle.open(member['file']) as stream:
                if bundle.getinfo(member['file']).file_size != member['size_bytes'] or digest(stream) != member['sha256']:
                    raise ValueError('Packaged file digest mismatch')

def load_records(directory, version):
    records = []
    for target, backends in MATRIX.items():
        for backend in backends:
            name = f'decisions-{version}-{target}-{backend}'
            record = json.loads((directory / f'{name}.json').read_text())
            expected_url = f'https://github.com/antibyte/AuraGo/releases/download/decisions-runtime-v{version}/{name}.zip'
            if (record['id'], record['target'], record['backend'], record['url']) != (name, target, backend, expected_url):
                raise ValueError('Unexpected package identity or URL')
            verify_archive(record, directory / f'{name}.zip')
            records.append(record)
    return records

def verify_public(catalog):
    opener = urllib.request.build_opener(HTTPSRedirect())
    for record in catalog['packages']:
        with opener.open(record['url'], timeout=60) as response:
            if not response.url.startswith('https://'):
                raise ValueError('Insecure artifact redirect')
            h = hashlib.sha256()
            received = 0
            while block := response.read(1024 * 1024):
                received += len(block)
                if received > record['size']: raise ValueError('Oversized public artifact')
                h.update(block)
            if received != record['size'] or h.hexdigest() != record['sha256']:
                raise ValueError('Published archive differs from verified build')
    with tempfile.TemporaryDirectory(prefix='decisions-anonymous-docker-') as directory:
        env = {**os.environ, 'DOCKER_CONFIG': directory}
        for image in catalog['images']:
            raw = subprocess.check_output(['docker', 'manifest', 'inspect', image['reference']], env=env)
            index = json.loads(raw)
            platforms = {(entry['platform']['os'], entry['platform']['architecture']) for entry in index['manifests']}
            if platforms != {('linux', 'amd64'), ('linux', 'arm64')}:
                raise ValueError('Docker image architecture set mismatch')
            for arch in ['amd64', 'arm64']:
                subprocess.run(['docker', 'pull', '--platform', f'linux/{arch}', image['reference']], env=env, check=True)

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--version', required=True)
    parser.add_argument('--directory', type=Path, required=True)
    parser.add_argument('--prepare-images', type=Path)
    parser.add_argument('--verify-published', action='store_true')
    parser.add_argument('--write', action='store_true')
    args = parser.parse_args()
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', args.version): raise ValueError('Invalid version')
    records = load_records(args.directory, args.version)
    if args.prepare_images:
        for record in records:
            if not record['target'].startswith('linux-'): continue
            destination = args.prepare_images / f"{record['target']}-{record['backend']}"
            destination.mkdir(parents=True, exist_ok=False)
            with zipfile.ZipFile(args.directory / f"{record['id']}.zip") as bundle:
                # verify_archive above already restricts every member to a flat portable name.
                for member in bundle.infolist():
                    output = destination / member.filename
                    with bundle.open(member) as source, output.open('xb') as target: shutil.copyfileobj(source, target)
                    output.chmod(0o755 if member.filename == 'aurago-decisions-runtime' else 0o644)
        return
    images = []
    for backend in ['cuda', 'vulkan', 'cpu']:
        metadata = json.loads((args.directory / f'docker-{backend}.json').read_text())
        digest_value = metadata['containerimage.digest']
        if not re.fullmatch(r'sha256:[a-f0-9]{64}', digest_value): raise ValueError('Invalid image digest')
        images.append(dict(backend=backend, reference=f'ghcr.io/antibyte/aurago-decisions-runtime@{digest_value}'))
    catalog = dict(version=args.version, packages=records, images=images)
    if args.write and not args.verify_published: raise ValueError('Writing pins requires anonymous publication verification')
    if args.verify_published: verify_public(catalog)
    if args.write:
        encoded = json.dumps(catalog, sort_keys=True, separators=(',', ':'))
        code = '// Code generated by scripts/decisions/release-catalog.py; DO NOT EDIT.\npackage decisions\n\nimport "encoding/json"\n\nfunc init() {\n if err := json.Unmarshal([]byte(' + json.dumps(encoded) + '), &releaseRuntimeCatalog); err != nil { panic(err) }\n}\n'
        output = ROOT / 'internal/decisions/runtime_catalog_generated.go'
        output.write_text(code, encoding='utf-8')
        subprocess.run(['gofmt', '-w', str(output)], check=True)
        (args.directory / 'runtime-catalog.json').write_text(json.dumps(catalog, indent=2) + '\n', encoding='utf-8')

if __name__ == '__main__':
    main()
