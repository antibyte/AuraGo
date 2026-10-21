// Shared helpers for the scripts/test-pixel-*.mjs node tests (not a test itself).
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

export const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
export const appDir = path.join(repoRoot, 'ui', 'js', 'desktop', 'apps');

// Every pixel-lib-*.js in worker import order (core first).
export function pixelLibFiles() {
    const all = fs.readdirSync(appDir).filter(name => /^pixel-lib-.+\.js$/.test(name));
    return ['pixel-lib-core.js'].concat(all.filter(name => name !== 'pixel-lib-core.js').sort());
}

// Loads pixel lib files into a fresh vm context that only has `self` (like a Worker) and returns { self, AuraPixelLib }.
export function loadPixelLibs(files) {
    const list = files || pixelLibFiles();
    const sandbox = { console, Math, Date, Uint8ClampedArray, Uint8Array, Uint16Array, Int32Array, Float32Array, Float64Array, ArrayBuffer };
    sandbox.self = sandbox;
    vm.createContext(sandbox);
    list.forEach(name => {
        const file = path.join(appDir, name);
        vm.runInContext(fs.readFileSync(file, 'utf8'), sandbox, { filename: name });
    });
    return { self: sandbox, AuraPixelLib: sandbox.AuraPixelLib };
}

// Loads app scripts into the current node context (window === globalThis) in the given order.
export function loadAppScripts(files) {
    files.forEach(name => {
        vm.runInThisContext(fs.readFileSync(path.join(appDir, name), 'utf8'), { filename: name });
    });
}

export function readAppFile(name) {
    return fs.readFileSync(path.join(appDir, name), 'utf8');
}
