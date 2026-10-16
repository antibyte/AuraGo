#!/usr/bin/env python3
"""Package a built runtime with redistributables; never include model weights."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[2]

def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()

def copy_member(source, target, name=None):
    source = source.resolve(strict=True)
    destination = target / (name or source.name)
    if destination.exists() and digest(source) != digest(destination):
        raise ValueError(f'Conflicting dependency: {destination.name}')
    shutil.copy2(source, destination)

def matching_dll(paths, architecture):
    expected = {'amd64': 0x8664, 'arm64': 0xaa64}[architecture]
    for path in sorted(paths):
        with path.open('rb') as stream:
            if stream.read(2) != b'MZ': continue
            stream.seek(0x3c)
            offset = struct.unpack('<I', stream.read(4))[0]
            stream.seek(offset)
            if stream.read(4) == b'PE\x00\x00' and struct.unpack('<H', stream.read(2))[0] == expected:
                return path
    raise ValueError(f'No {architecture} redistributable in {paths}')

def linux_dependencies(binary, target):
    output = subprocess.check_output(['ldd', str(binary)], text=True)
    system = re.compile(r'^(linux-vdso|ld-linux|lib(c|m|pthread|dl|rt|resolv|util)\.so|libcuda\.so)')
    for line in output.splitlines():
        if 'not found' in line and 'libcuda.so' not in line:
            raise ValueError(f'Missing dependency: {line}')
        match = re.match(r'\s*(\S+) => (/\S+)', line)
        if match and not system.match(match[1]):
            copy_member(Path(match[2]), target, match[1])
    for folder in ['libc6', 'libstdc++6', 'libgcc-s1', 'libvulkan1']:
        license_file = Path('/usr/share/doc') / folder / 'copyright'
        if license_file.exists():
            copy_member(license_file, target, f'LICENSE-{folder}.txt')

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--target', required=True)
    parser.add_argument('--backend', choices=['cpu', 'cuda', 'vulkan', 'metal'], required=True)
    parser.add_argument('--version', required=True)
    parser.add_argument('--out', type=Path, default=ROOT / 'artifacts')
    args = parser.parse_args()
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+', args.version):
        raise ValueError('Version must be numeric major.minor.patch')
    binary_name = 'aurago-decisions-runtime' + ('.exe' if args.target.startswith('windows-') else '')
    binary = ROOT / 'bin/decisions-runtime' / args.target / args.backend / binary_name
    args.out.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='decisions-package-') as temporary:
        stage = Path(temporary)
        copy_member(binary, stage)
        copy_member(ROOT / 'LICENSE', stage, 'LICENSE-AuraGo.txt')
        llama = ROOT / 'bin/decisions-runtime' / f'build-{args.target}-{args.backend}' / '_deps/llama_cpp-src'
        if not llama.exists():
            llama = ROOT / 'bin/decisions-runtime/llama-src'
        copy_member(llama / 'LICENSE', stage, 'LICENSE-llama.cpp.txt')
        if args.target.startswith('linux-'):
            linux_dependencies(binary, stage)
        if args.backend == 'cuda':
            cuda = Path(os.environ.get('CUDA_PATH', '/usr/local/cuda'))
            licenses = sorted({p for p in list(cuda.rglob('EULA.txt')) + list(cuda.rglob('LICENSE*')) if p.is_file()})
            if not licenses:
                raise ValueError('CUDA redistribution notices missing')
            for index, license_file in enumerate(licenses):
                if license_file.is_file():
                    copy_member(license_file, stage, f'LICENSE-CUDA-{index}.txt')
            if os.name == 'nt':
                for pattern in ['cublas64_*.dll', 'cublasLt64_*.dll']:
                    matches = list(cuda.rglob(pattern))
                    if not matches: raise ValueError(f'Missing CUDA redistributable {pattern}')
                    copy_member(matching_dll(matches, args.target.split('-')[1]), stage)
        if args.backend == 'vulkan' and os.name == 'nt':
            sdk = Path(os.environ['VULKAN_SDK'])
            loaders = list(sdk.rglob('vulkan-1.dll'))
            if not loaders: loaders = [Path(os.environ['SystemRoot']) / 'System32/vulkan-1.dll']
            copy_member(matching_dll(loaders, args.target.split('-')[1]), stage)
            licenses = sorted(p for p in sdk.rglob('*') if p.is_file() and p.name.lower() in ['license.txt', 'license', 'copyright.txt'])
            if not licenses: raise ValueError('Vulkan redistribution notices missing')
            for index, license_file in enumerate(licenses):
                copy_member(license_file, stage, f'LICENSE-Vulkan-{index}.txt')
        subprocess.run(['python', str(ROOT / 'scripts/write-decisions-runtime-manifest.py'), '--binary', str(stage / binary_name), '--target', args.target, '--backend', args.backend, '--release'], check=True)
        environment = {key: value for key, value in os.environ.items() if key not in ['CUDA_PATH', 'VULKAN_SDK', 'LD_LIBRARY_PATH', 'DYLD_LIBRARY_PATH'] and not key.startswith('CUDA_PATH_')}
        environment['PATH'] = os.pathsep.join([str(stage), str(Path(os.environ.get('SystemRoot', '/')) / 'System32')]) if os.name == 'nt' else '/usr/bin:/bin'
        environment['LD_LIBRARY_PATH'] = str(stage)
        subprocess.run([str(stage / binary_name), '--self-test-protocol'], env=environment, cwd=stage, check=True, timeout=120)
        discovered = json.loads(subprocess.check_output([str(stage / binary_name), '--discover'], env=environment, cwd=stage, timeout=120))
        assert discovered['protocol'] == 'aurago.decisions.v1' and isinstance(discovered['devices'], list)
        name = f'decisions-{args.version}-{args.target}-{args.backend}'
        archive = args.out / f'{name}.zip'
        with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED) as bundle:
            for member in sorted(stage.iterdir()): bundle.write(member, member.name)
        record = dict(id=name, target=args.target, backend=args.backend, url=f'https://github.com/antibyte/AuraGo/releases/download/decisions-runtime-v{args.version}/{archive.name}',
                      size=archive.stat().st_size, sha256=digest(archive), manifest_sha256=digest(stage / 'release-manifest.json'),
                      unpacked_size=sum(p.stat().st_size for p in stage.iterdir()),
                      requirements=json.loads((stage / 'release-manifest.json').read_text())['build']['required_system_components'])
        (args.out / f'{name}.json').write_text(json.dumps(record, indent=2) + '\n', encoding='utf-8')

if __name__ == '__main__':
    main()
