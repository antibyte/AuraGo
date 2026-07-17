import { createHash } from 'node:crypto';
import { mkdir, readFile, writeFile, copyFile } from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const root = process.cwd();
const outputDir = path.join(root, 'ui', 'js', 'realtime-speech', 'vendor');
const checkOnly = process.argv.includes('--check');

const artifacts = [
    {
        source: path.join(root, 'node_modules', 'onnxruntime-web', 'dist', 'ort.wasm.min.js'),
        target: 'ort.wasm.min.js',
        sha256: 'ea3a767b15df7dbe3d695ec9c182ca0f15b2ce7750156c6b70276e11c28997f0'
    },
    {
        source: path.join(root, 'node_modules', 'onnxruntime-web', 'dist', 'ort-wasm-simd-threaded.mjs'),
        target: 'ort-wasm-simd-threaded.mjs',
        sha256: '0a1e718d99c41b22c21f2520ff4f9e883a6b5533856e398d21816ee8eb8185d3'
    },
    {
        source: path.join(root, 'node_modules', 'onnxruntime-web', 'dist', 'ort-wasm-simd-threaded.wasm'),
        target: 'ort-wasm-simd-threaded.wasm',
        sha256: 'd1ab1b94b16a65b29d710d0b587b29e7bed336827577623913479b8afe8113e6'
    }
];

const silero = {
    url: 'https://raw.githubusercontent.com/snakers4/silero-vad/v6.2.1/src/silero_vad/data/silero_vad.onnx',
    target: 'silero_vad_v6.2.1.onnx',
    sha256: '1a153a22f4509e292a94e67d6f9b85e8deb25b4988682b7e174c65279d8788e3'
};

function digest(buffer) {
    return createHash('sha256').update(buffer).digest('hex');
}

async function verifiedRead(file, expected) {
    const data = await readFile(file);
    const actual = digest(data);
    if (actual !== expected) {
        throw new Error(`Checksum mismatch for ${file}: ${actual} != ${expected}`);
    }
    return data;
}

await mkdir(outputDir, { recursive: true });

for (const artifact of artifacts) {
    await verifiedRead(artifact.source, artifact.sha256);
    const target = path.join(outputDir, artifact.target);
    if (checkOnly) {
        await verifiedRead(target, artifact.sha256);
    } else {
        await copyFile(artifact.source, target);
    }
}

const sileroTarget = path.join(outputDir, silero.target);
if (checkOnly) {
    await verifiedRead(sileroTarget, silero.sha256);
} else {
    let data;
    try {
        data = await verifiedRead(
            path.join(root, 'disposable', 'realtime-speech-vendor', silero.target),
            silero.sha256
        );
    } catch {
        const response = await fetch(silero.url);
        if (!response.ok) {
            throw new Error(`Unable to download Silero VAD: HTTP ${response.status}`);
        }
        data = Buffer.from(await response.arrayBuffer());
        if (digest(data) !== silero.sha256) {
            throw new Error('Downloaded Silero VAD checksum does not match the pinned release');
        }
    }
    await writeFile(sileroTarget, data);
}

console.log(`${checkOnly ? 'Verified' : 'Built'} realtime speech vendor assets in ${outputDir}`);
