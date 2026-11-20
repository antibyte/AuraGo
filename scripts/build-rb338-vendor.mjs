// SPDX-License-Identifier: GPL-3.0-only
// Builds the RB-338 browser artefacts (OpenRB-338 engine as WebAssembly) into ui/js/vendor/rb338/.
//   node scripts/build-rb338-vendor.mjs                  build and write
//   node scripts/build-rb338-vendor.mjs --check          rebuild and compare with the committed files
//   node scripts/build-rb338-vendor.mjs --verify-sources also rebuild the wasm from sources.zip, offline
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createHash } from 'node:crypto';
import { spawnSync } from 'node:child_process';
import { zipSync, unzipSync } from 'fflate';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const out = path.join(root, 'ui/js/vendor/rb338');
const crate = path.join(root, 'tools/rb338-wasm');
const manifestPath = path.join(crate, 'Cargo.toml');
const check = process.argv.includes('--check');
const verifySources = process.argv.includes('--verify-sources');

const UPSTREAM = 'https://github.com/pingw33n/OpenRB-338';
const REV = '0bebfcdfd544b3cdea14536f19fb28a8de314df5';
const TOOLCHAIN = '1.99.1';
const TARGET = 'wasm32-unknown-unknown';
const KIT_SHA256 = '6355b5f14fc2ebfe7ed9e6e42208f97817f96c5e30ed42fdca4205d3cdab5fbe';
const KIT_BYTES = 2572792;
const MAX_WASM_BYTES = 1.5 * 1024 * 1024;
const REQUIRED_EXPORTS = ['memory', 'orb_abi_version', 'orb_alloc', 'orb_free', 'orb_create', 'orb_last_error', 'orb_destroy', 'orb_new_song', 'orb_load_song', 'orb_save_song', 'orb_song_info', 'orb_patterns', 'orb_defaults', 'orb_push', 'orb_apply', 'orb_render', 'orb_snapshot', 'orb_startup_sound', 'orb_io_ptr', 'orb_io_len', 'orb_export_wav'];
// GPL JS files of the module. Whatever exists at build time goes into sources.zip.
const EXPECTED_JS = ['model', 'host', 'worklet', 'worker'].map((n) => `ui/js/desktop/apps/synth-studio-rb338-${n}.js`).concat('ui/js/desktop/apps/synth-studio-rb338.js');
const MTIME = new Date('2000-01-01T00:00:00Z');
const TEXT = /\.(rs|toml|md|txt|js|mjs|json|lock|yml|yaml|cfg|h|c|in|html|css)$|^(LICENSE|COPYING|NOTICE|README|AUTHORS)[^/]*$|\/(LICENSE|COPYING|NOTICE)[^/]*$/i;

const sha256 = (data) => createHash('sha256').update(data).digest('hex');
function run(cmd, args, opts = {}) {
    const r = spawnSync(cmd, args, { cwd: root, encoding: 'utf8', maxBuffer: 256 * 1024 * 1024, stdio: ['ignore', 'pipe', opts.quiet ? 'pipe' : 'inherit'], ...opts });
    if (r.status !== 0) throw new Error(`${cmd} ${args.join(' ')} failed (${r.status}) ${r.error || ''}\n${r.stderr || ''}`);
    return r.stdout;
}
const lf = (bytes, name) => (TEXT.test(name) ? Buffer.from(bytes.toString('utf8').replaceAll('\r\n', '\n')) : bytes);
async function walk(dir, skip) {
    const files = [];
    for (const entry of await fs.readdir(dir, { recursive: true, withFileTypes: true })) {
        if (!entry.isFile()) continue;
        const rel = path.relative(dir, path.join(entry.parentPath, entry.name)).replaceAll('\\', '/');
        if (skip(rel)) continue;
        files.push(rel);
    }
    return files.sort();
}
// Vendored sources and the kit stay byte for byte (cargo checks their hashes); our own files are LF-normalised.
const add = (zip, name, bytes) => { zip[name] = [/^(vendor|assets)\//.test(name) ? Buffer.from(bytes) : lf(Buffer.from(bytes), name), { mtime: MTIME }]; };

// 1. Standard.rbm from the pinned OpenRB checkout
const meta = JSON.parse(run('cargo', ['metadata', '--format-version', '1', '--manifest-path', manifestPath], { quiet: true }));
const rbmPkg = meta.packages.find((p) => p.name === 'rbm');
if (!rbmPkg) throw new Error('cargo metadata: package rbm not found');
const kitPath = path.join(path.dirname(rbmPkg.manifest_path), '..', '..', 'assets', 'Standard.rbm');
const kit = await fs.readFile(kitPath);
if (kit.length !== KIT_BYTES || sha256(kit) !== KIT_SHA256) throw new Error('Standard.rbm mismatch: ' + kit.length + ' bytes, sha256 ' + sha256(kit));


// 2. wasm: built against the vendored sources with remapped paths, so it is the same bytes in any checkout.
const cargoHome = process.env.CARGO_HOME || path.join(os.homedir(), '.cargo');
const fwd = (p) => p.replaceAll('\x5c', '/');
// Remapped prefixes, longest first; the same flags are documented in the sources.zip README.
const rustflags = (crateDir, vendorDir) => [`--remap-path-prefix=${crateDir}=/aurago-rb338`, `--remap-path-prefix=${vendorDir}=/vendor`, `--remap-path-prefix=${cargoHome}=/cargo`];
const buildEnv = (flags) => ({ ...process.env, RUSTFLAGS: '', CARGO_ENCODED_RUSTFLAGS: flags.join(String.fromCharCode(31)) });
const vendorConfig = (dir) => `[source.crates-io]
replace-with = "vendored-sources"

[source."git+${UPSTREAM}?rev=${REV}"]
git = "${UPSTREAM}"
rev = "${REV}"
replace-with = "vendored-sources"

[source.vendored-sources]
directory = "${dir}"
`;
const tmp = await fs.mkdtemp(path.join(os.tmpdir(), 'rb338-vendor-'));
let wasm;
let sourcesZip;
try {
    const vendorDir = path.join(tmp, 'vendor');
    run('cargo', [`+${TOOLCHAIN}`, 'vendor', '--versioned-dirs', '--locked', '--manifest-path', manifestPath, vendorDir], { quiet: true });
    // rbm includes "../../../assets/Standard.rbm" relative to vendor/rbm-*/src, i.e. <tmp>/assets/.
    await fs.mkdir(path.join(tmp, 'assets'), { recursive: true });
    await fs.writeFile(path.join(tmp, 'assets/Standard.rbm'), kit);
    const cfg = path.join(tmp, 'vendor-config.toml');
    await fs.writeFile(cfg, vendorConfig(fwd(vendorDir)));
    run('cargo', [`+${TOOLCHAIN}`, 'build', '--release', '--locked', '--offline', '--config', cfg, '--target', TARGET, '--manifest-path', manifestPath], { env: buildEnv(rustflags(crate, vendorDir)) });
    wasm = await fs.readFile(path.join(crate, 'target', TARGET, 'release', 'aurago_rb338.wasm'));

// 3. checks
const module = await WebAssembly.compile(wasm);
const imports = WebAssembly.Module.imports(module);
if (imports.length) throw new Error('wasm must have no imports: ' + JSON.stringify(imports));
const exported = new Set(WebAssembly.Module.exports(module).map((e) => e.name));
const missing = REQUIRED_EXPORTS.filter((n) => !exported.has(n));
if (missing.length) throw new Error('wasm lacks exports: ' + missing.join(', '));
if (wasm.length >= MAX_WASM_BYTES) throw new Error('wasm too large: ' + wasm.length);


// 4. sources.zip
    const zip = {};
    for (const rel of await walk(crate, (r) => r.startsWith('target/') || r.startsWith('.git/'))) add(zip, 'aurago-rb338/' + rel, await fs.readFile(path.join(crate, rel)));
    for (const rel of await walk(vendorDir, (r) => r.startsWith('.git/'))) add(zip, 'vendor/' + rel, await fs.readFile(path.join(vendorDir, rel)));
    add(zip, '.cargo/config.toml', `[source.crates-io]\nreplace-with = "vendored-sources"\n\n[source."git+${UPSTREAM}?rev=${REV}"]\ngit = "${UPSTREAM}"\nrev = "${REV}"\nreplace-with = "vendored-sources"\n\n[source.vendored-sources]\ndirectory = "vendor"\n`);
    const present = [];
    for (const name of EXPECTED_JS) {
        const bytes = await fs.readFile(path.join(root, name)).catch(() => null);
        if (bytes) { add(zip, 'aurago/' + name, bytes); present.push(name); }
    }
    add(zip, 'aurago/scripts/build-rb338-vendor.mjs', await fs.readFile(path.join(root, 'scripts/build-rb338-vendor.mjs')));
    add(zip, 'LICENSE.txt', await fs.readFile(path.join(root, 'licences/texts/GPL-3.0-only.txt')));
    add(zip, 'README.md', `# RB-338 engine: corresponding source

The OpenRB-338 ReBirth RB-338 engine compiled to WebAssembly for AuraGo's Synth Studio. GPL-3.0-only (LICENSE.txt).

- Upstream: ${UPSTREAM}, commit ${REV} (orb-engine, rbs and rbm are used unchanged).
- Wrapper crate: aurago-rb338/ (raw extern "C" ABI, no imports). Pinned toolchain: Rust ${TOOLCHAIN}, target ${TARGET}.
- vendor/ holds the exact sources of every dependency (cargo vendor --versioned-dirs). .cargo/config.toml redirects cargo to it, so the build works offline.
- The kit Standard.rbm is not repeated in this archive; it ships next to it as Standard.rbm (SHA-256 ${KIT_SHA256}, ${KIT_BYTES} bytes). rbm references it through include_bytes!("../../../assets/Standard.rbm"), which from vendor/rbm-0.1.0/src is assets/Standard.rbm at the root of the unpacked archive, so copy it there before building (step 1 below). The shipped wasm itself does not contain it; the host passes the kit to orb_create.
- aurago/ holds the AuraGo JavaScript of the module (model, host, worklet, worker and panel) and the build script. Names the build script collects when they exist: ${EXPECTED_JS.join(', ')}. This archive was built with: ${present.join(', ') || 'none of them yet'}.

## Rebuild the wasm

Unpack this archive into a directory D (absolute path, no need for a particular location), then:

\`\`\`
1. mkdir D/assets && cp Standard.rbm D/assets/Standard.rbm      (the Standard.rbm shipped next to this archive; check its SHA-256)
2. cd D/aurago-rb338
3. set the path-remap flags (below)
4. cargo +${TOOLCHAIN} build --release --locked --offline --target ${TARGET}
\`\`\`

The result is target/${TARGET}/release/aurago_rb338.wasm (rb338-engine.wasm in the AuraGo tree). Absolute build paths would otherwise end up in the binary, so the shipped wasm was built with these rustc flags (cargo reads them from CARGO_ENCODED_RUSTFLAGS, separated by the byte 0x1f, which keeps paths with spaces intact; plain RUSTFLAGS works when no path contains a space):

\`\`\`
--remap-path-prefix=D/aurago-rb338=/aurago-rb338
--remap-path-prefix=D/vendor=/vendor
--remap-path-prefix=$CARGO_HOME=/cargo      (CARGO_HOME defaults to ~/.cargo)
\`\`\`

With the same toolchain the rebuilt wasm is byte-identical to rb338-engine.wasm. node scripts/build-rb338-vendor.mjs --verify-sources in an AuraGo checkout performs exactly these steps.

## Rebuild every artefact in an AuraGo checkout

\`\`\`
npm ci
node scripts/build-rb338-vendor.mjs
\`\`\`

AuraGo: https://github.com/antibyte/AuraGo
`);
    sourcesZip = Buffer.from(zipSync(Object.fromEntries(Object.entries(zip).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))), { level: 9 }));

    if (verifySources) {
        const files = unzipSync(sourcesZip);
        const dir = path.join(tmp, 'verify');
        for (const [name, bytes] of Object.entries(files)) { const t = path.join(dir, name); await fs.mkdir(path.dirname(t), { recursive: true }); await fs.writeFile(t, bytes); }
        // README step 1: the shipped kit goes to assets/Standard.rbm of the unpacked tree.
        const shipped = await fs.readFile(path.join(out, 'Standard.rbm'));
        if (sha256(shipped) !== KIT_SHA256) throw new Error('shipped Standard.rbm has the wrong SHA-256');
        await fs.mkdir(path.join(dir, 'assets'), { recursive: true });
        await fs.writeFile(path.join(dir, 'assets/Standard.rbm'), shipped);
        run('cargo', [`+${TOOLCHAIN}`, 'build', '--release', '--locked', '--offline', '--target', TARGET, '--manifest-path', path.join(dir, 'aurago-rb338/Cargo.toml')], { cwd: path.join(dir, 'aurago-rb338'), quiet: true, env: buildEnv(rustflags(path.join(dir, 'aurago-rb338'), path.join(dir, 'vendor'))) });
        const rebuilt = await fs.readFile(path.join(dir, 'aurago-rb338/target', TARGET, 'release/aurago_rb338.wasm'));
        console.log('sources.zip rebuild ' + (rebuilt.equals(wasm) ? 'is identical to the shipped wasm' : 'DIFFERS from the shipped wasm; sha256 ' + sha256(rebuilt)));
    }
} finally {
    await fs.rm(tmp, { recursive: true, force: true, maxRetries: 3 });
}

// 5. notices and manifest
const assets = {
    'rb338-engine.wasm': wasm,
    'Standard.rbm': kit,
    'sources.zip': sourcesZip,
    'LICENSE.txt': Buffer.from((await fs.readFile(path.join(root, 'licences/texts/GPL-3.0-only.txt'), 'utf8')).replaceAll('\r\n', '\n')),
    'LICENSES.md': Buffer.from(`# RB-338 engine: GPL-3.0-only

The RB-338 engine (\`rb338-engine.wasm\`) is the OpenRB-338 crates orb-engine, rbs and rbm, compiled unchanged to WebAssembly together with AuraGo's wrapper crate \`tools/rb338-wasm\`.

Copyright (C) 2026 the OpenRB-338 authors. Upstream: ${UPSTREAM}, commit ${REV}.
AuraGo wrapper crate, build script and RB-338 browser module: added 2026-11-20, GPL-3.0-only.

The browser combination containing this module is distributed under the GPL-3.0-only terms; independent MIT-licensed AuraGo sources keep their licence. The full licence text is in [LICENSE.txt](LICENSE.txt).

Complete corresponding source (wrapper crate, every vendored dependency, the kit, the JavaScript of the module, the build script and rebuild steps): [sources.zip](sources.zip). Hashes and toolchain: [manifest.json](manifest.json).

## Standard.rbm

\`Standard.rbm\` is the standard mod of Propellerhead ReBirth 2.01 (46 drum samples, panel images and splash screen, recrypted standard mod, SHA-256 ${KIT_SHA256}), as documented in OpenRB-338 \`assets/README.md\`. It is Propellerhead material, not GPL code. AuraGo ships it by the project owner's decision of 2026-11-20. The panel images are not displayed.
`),
};
assets['manifest.json'] = Buffer.from(JSON.stringify({
    upstream: UPSTREAM,
    rev: REV,
    toolchain: { rust: TOOLCHAIN, target: TARGET },
    crate: 'tools/rb338-wasm',
    files: Object.fromEntries(Object.entries(assets).map(([name, data]) => [name, { bytes: data.length, sha256: sha256(data) }])),
}, null, 2) + '\n');

for (const [name, data] of Object.entries(assets)) {
    const target = path.join(out, name);
    if (check) {
        const current = await fs.readFile(target).catch(() => null);
        if (!current || !data.equals(current)) throw new Error('Stale RB-338 asset: ' + name);
    } else {
        await fs.mkdir(path.dirname(target), { recursive: true });
        await fs.writeFile(target, data);
    }
}
console.log('RB-338: ' + Object.entries(assets).map(([n, d]) => n + ' ' + d.length).join(', ') + ' ' + (check ? 'verified' : 'vendored'));
