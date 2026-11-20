// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

// Module Worker: offline WAV renderer of the OpenRB-338 engine (own wasm instance, never WebAudio).
//   main -> worker: init{module|bytes, kit} render{id, song: ArrayBuffer, mode: 'song'|'pattern'|0|1, bars, preroll}
//                   check{id, song: ArrayBuffer}
//   worker -> main: ready{} rendered{id, wav: ArrayBuffer, frames} checked{id, ok, code} error{id, code}
// Error codes: 1 not a song, 2 cannot convert, 3 engine refused the load / kit failed, 10 ABI mismatch,
// 20 not initialised, 21 wasm trap or bad request.

let x = null;
let h = 0;
let stored = null; // { mod, kit } kept so a trapped instance can be re-created
const IDLE_BLOCKS = 9088;

function u8() {
    return new Uint8Array(x.memory.buffer);
}

function withBytes(src, fn) {
    const bytes = src instanceof Uint8Array ? src : new Uint8Array(src);
    const ptr = x.orb_alloc(bytes.length);
    try {
        u8().set(bytes, ptr);
        return fn(ptr, bytes.length);
    } finally {
        x.orb_free(ptr, bytes.length);
    }
}

// Instantiates the engine from the stored module + kit. Returns 0 on success, else an error code.
function instantiate() {
    x = null;
    h = 0;
    const inst = new WebAssembly.Instance(stored.mod, {}).exports;
    if (typeof inst.orb_abi_version !== 'function' || inst.orb_abi_version() !== 1) return 10;
    const ptr = inst.orb_alloc(stored.kit.length);
    let handle = 0;
    try {
        new Uint8Array(inst.memory.buffer).set(stored.kit, ptr);
        handle = inst.orb_create(ptr, stored.kit.length);
    } finally {
        inst.orb_free(ptr, stored.kit.length);
    }
    if (!handle) return inst.orb_last_error() || 3;
    x = inst;
    h = handle;
    return 0;
}

function init(d) {
    stored = { mod: d.module || new WebAssembly.Module(d.bytes), kit: new Uint8Array(d.kit) };
    const code = instantiate();
    if (code) {
        x = null;
        h = 0;
        self.postMessage({ type: 'error', id: 0, code });
        return;
    }
    self.postMessage({ type: 'ready' });
}

// Re-creates a dropped (trapped) instance on demand. Returns true when the engine is usable.
function ensure() {
    if (x && h) return true;
    if (!stored) return false;
    try {
        return instantiate() === 0;
    } catch (_err) {
        x = null;
        h = 0;
        return false;
    }
}

// Pre-check of a song file: loads it into this worker's own engine and resets it again.
function check(d) {
    if (!ensure()) {
        self.postMessage({ type: 'error', id: d.id, code: 20 });
        return;
    }
    const code = withBytes(new Uint8Array(d.song), (p, n) => x.orb_load_song(h, p, n));
    if (code === 0) x.orb_new_song(h);
    self.postMessage({ type: 'checked', id: d.id, ok: code === 0, code });
}

// Keeps the second of two equal cycles of a 16-bit stereo 44.1 kHz WAV (44-byte header).
// The 2*bars export has N frames = the chunk sum of a [0, 2*bars*768] range; the cycle boundary is
// at tick bars*768 = frame N/2 (to within one frame: frames are an integer truncation of the same
// linear tick->frame map), so the result is frames [floor(N/2), floor(N/2)*2).
function secondCycle(wav) {
    const total = (wav.length - 44) >> 2;
    const cycle = total >> 1;
    const body = wav.subarray(44 + cycle * 4, 44 + cycle * 8);
    const out = new Uint8Array(44 + body.length);
    out.set(wav.subarray(0, 44));
    out.set(body, 44);
    const dv = new DataView(out.buffer);
    dv.setUint32(4, 36 + body.length, true);
    dv.setUint32(40, body.length, true);
    return { wav: out, frames: cycle };
}

function render(d) {
    if (!ensure()) {
        self.postMessage({ type: 'error', id: d.id, code: 20 });
        return;
    }
    const pattern = d.mode === 'pattern' || d.mode === 1;
    const bars = Math.max(1, Math.min(256, d.bars | 0 || 4));
    const pre = !!d.preroll && pattern;
    const song = new Uint8Array(d.song);
    const code = withBytes(song, (p, n) => x.orb_export_wav(h, p, n, pattern ? 1 : 0, pre ? bars * 2 : bars, IDLE_BLOCKS));
    if (code !== 0) {
        self.postMessage({ type: 'error', id: d.id, code });
        return;
    }
    let wav = u8().slice(x.orb_io_ptr(h), x.orb_io_ptr(h) + x.orb_io_len(h));
    let frames = (wav.length - 44) >> 2;
    if (pre) {
        const r = secondCycle(wav);
        wav = r.wav;
        frames = r.frames;
    }
    self.postMessage({ type: 'rendered', id: d.id, wav: wav.buffer, frames }, [wav.buffer]);
}

self.onmessage = (e) => {
    const d = e.data;
    if (!d) return;
    try {
        if (d.type === 'init') init(d);
        else if (d.type === 'render') render(d);
        else if (d.type === 'check') check(d);
    } catch (err) {
        // a trapped instance is unusable: drop it, the next request re-instantiates
        x = null;
        h = 0;
        self.postMessage({ type: 'error', id: d.id || 0, code: 21, message: String((err && err.message) || err) });
    }
};
