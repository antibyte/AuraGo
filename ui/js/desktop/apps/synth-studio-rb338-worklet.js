// SPDX-License-Identifier: GPL-3.0-only
// Derived from OpenRB-338 (commit 0bebfcd), Copyright (C) 2026 the OpenRB-338 authors

// AudioWorkletProcessor that runs the OpenRB-338 engine (WebAssembly, tools/rb338-wasm).
// Protocol: see plans/2026-11-20-synth-studio-rb338.md section A4.
//   main -> worklet: init{module|bytes, kit, song?} msgs{buf, seq} load{id, bytes} new{id} save{id} startup
//   worklet -> main: ready{defaults, patterns, info} loaded{id, ok, code, patterns, info}
//                    saved{id, bytes} snap{buf} error{code, id?} crash{message}
// Error codes: 1..3 = orb_last_error (kit), 10 = wasm ABI mismatch, 11 = command queue overflow, 12 = save failed, 20 = no engine, 21 = engine crashed.

(function () {
    'use strict';

    const CALL = 1024;
    const QUANTUM = 128;
    const SNAP_LEN = 192;
    const PATTERNS_LEN = 18688;
    const DEFAULTS_LEN = 120;

    function utf8(bytes) {
        let s = '';
        for (let i = 0; i < bytes.length;) {
            const b = bytes[i++];
            if (b < 0x80) s += String.fromCharCode(b);
            else if (b < 0xe0) s += String.fromCharCode(((b & 31) << 6) | (bytes[i++] & 63));
            else if (b < 0xf0) {
                s += String.fromCharCode(((b & 15) << 12) | ((bytes[i++] & 63) << 6) | (bytes[i++] & 63));
            } else {
                const cp = ((b & 7) << 18) | ((bytes[i++] & 63) << 12) | ((bytes[i++] & 63) << 6) | (bytes[i++] & 63);
                s += String.fromCodePoint(cp);
            }
        }
        return s;
    }

    class RB338Processor extends AudioWorkletProcessor {
        constructor(options) {
            super();
            const po = (options && options.processorOptions) || {};
            this.ratio = po.ratio && po.ratio !== 1 ? po.ratio : 1;
            this.x = null;
            this.h = 0;
            this.dead = false;
            this.left = new Float32Array(CALL);
            this.right = new Float32Array(CALL);
            this.have = 0; // frames of the current call still unread (ratio 1)
            this.at = CALL; // read position in the call buffer (ratio 1)
            // resampler state: arrays hold 1 carried sample + CALL new ones
            this.rl = new Float32Array(CALL + 1);
            this.rr = new Float32Array(CALL + 1);
            this.pos = CALL; // forces a render on first use
            this.first = true;
            this.port.onmessage = (e) => this.onMessage(e.data);
        }

        u8() {
            return new Uint8Array(this.x.memory.buffer);
        }

        copyOut(ptr, len) {
            return this.u8().slice(ptr, ptr + len);
        }

        withBytes(src, fn) {
            const x = this.x;
            const bytes = src instanceof Uint8Array ? src : new Uint8Array(src);
            const ptr = x.orb_alloc(bytes.length);
            try {
                this.u8().set(bytes, ptr);
                return fn(ptr, bytes.length);
            } finally {
                x.orb_free(ptr, bytes.length);
            }
        }

        info() {
            const x = this.x;
            x.orb_song_info(this.h);
            const raw = this.copyOut(x.orb_io_ptr(this.h), x.orb_io_len(this.h));
            try {
                return JSON.parse(utf8(raw));
            } catch (_err) {
                return null;
            }
        }

        patterns() {
            return this.copyOut(this.x.orb_patterns(this.h), PATTERNS_LEN);
        }

        fail(err) {
            this.dead = true;
            this.h = 0;
            try {
                this.port.postMessage({ type: 'crash', message: String((err && err.message) || err) });
            } catch (_err) { /* nothing left to do */ }
        }

        init(d) {
            let mod = d.module;
            if (!mod) mod = new WebAssembly.Module(d.bytes);
            this.x = new WebAssembly.Instance(mod, {}).exports;
            const x = this.x;
            if (typeof x.orb_abi_version !== 'function' || x.orb_abi_version() !== 1) {
                this.port.postMessage({ type: 'error', code: 10 });
                this.dead = true;
                return;
            }
            const kit = new Uint8Array(d.kit);
            this.h = this.withBytes(kit, (p, n) => x.orb_create(p, n));
            if (!this.h) {
                this.port.postMessage({ type: 'error', code: x.orb_last_error() || 1 });
                this.dead = true;
                return;
            }
            if (d.song && d.song.byteLength) {
                // a refused song leaves the empty song untouched
                this.withBytes(new Uint8Array(d.song), (p, n) => x.orb_load_song(this.h, p, n));
            }
            const defaults = this.copyOut(x.orb_defaults(this.h), DEFAULTS_LEN);
            const patterns = this.patterns();
            const info = this.info();
            this.port.postMessage({ type: 'ready', defaults, patterns, info }, [defaults.buffer, patterns.buffer]);
        }

        onMessage(d) {
            if (!d) return;
            if (this.dead) {
                // answer id-bearing requests so the host never waits forever
                if (d.id && (d.type === 'load' || d.type === 'new' || d.type === 'save')) {
                    this.port.postMessage({ type: 'error', code: 21, id: d.id });
                }
                return;
            }
            try {
                const x = this.x;
                switch (d.type) {
                    case 'init':
                        this.init(d);
                        break;
                    case 'msgs': {
                        if (!this.h) break;
                        const rc = this.withBytes(d.buf, (p, n) => x.orb_push(this.h, p, n, d.seq >>> 0));
                        if (rc) this.port.postMessage({ type: 'error', code: 11 });
                        break;
                    }
                    case 'load': {
                        if (!this.h) { this.noEngine(d); break; }
                        const code = this.withBytes(d.bytes, (p, n) => x.orb_load_song(this.h, p, n));
                        this.postLoaded(d.id, code);
                        break;
                    }
                    case 'new':
                        if (!this.h) { this.noEngine(d); break; }
                        x.orb_new_song(this.h);
                        this.postLoaded(d.id, 0);
                        break;
                    case 'save': {
                        if (!this.h) { this.noEngine(d); break; }
                        const rc = x.orb_save_song(this.h);
                        if (rc !== 0) {
                            this.port.postMessage({ type: 'error', code: 12, id: d.id });
                            break;
                        }
                        const bytes = this.copyOut(x.orb_io_ptr(this.h), x.orb_io_len(this.h));
                        this.port.postMessage({ type: 'saved', id: d.id, bytes }, [bytes.buffer]);
                        break;
                    }
                    case 'startup':
                        if (this.h) x.orb_startup_sound(this.h);
                        break;
                    default:
                        break;
                }
            } catch (err) {
                this.fail(err);
            }
        }

        noEngine(d) {
            if (d.id) this.port.postMessage({ type: 'error', code: 20, id: d.id });
        }

        postLoaded(id, code) {
            const patterns = this.patterns();
            const info = this.info();
            this.port.postMessage({ type: 'loaded', id, ok: code === 0, code, patterns, info }, [patterns.buffer]);
        }

        // Renders one 1024-frame call into dl/dr at offset `off` and posts the snapshot.
        renderCall(dl, dr, off) {
            const x = this.x;
            const ptr = x.orb_render(this.h);
            const f = new Float32Array(x.memory.buffer, ptr, CALL * 2);
            dl.set(f.subarray(0, CALL), off);
            dr.set(f.subarray(CALL, CALL * 2), off);
            const snap = this.copyOut(x.orb_snapshot(this.h), SNAP_LEN);
            this.port.postMessage({ type: 'snap', buf: snap }, [snap.buffer]);
        }

        process(_inputs, outputs) {
            const out = outputs[0];
            if (!out || !out[0]) return true;
            if (this.dead || !this.h) return !this.dead;
            const L = out[0];
            const R = out[1] || out[0];
            try {
                if (this.ratio === 1) {
                    if (this.at >= CALL) {
                        this.renderCall(this.left, this.right, 0);
                        this.at = 0;
                    }
                    L.set(this.left.subarray(this.at, this.at + L.length));
                    if (R !== L) R.set(this.right.subarray(this.at, this.at + L.length));
                    this.at += L.length;
                } else {
                    const rl = this.rl;
                    const rr = this.rr;
                    for (let i = 0; i < L.length; i++) {
                        while (this.pos >= CALL) {
                            // carry the last sample of the previous call, then fetch the next call
                            const cl = this.first ? 0 : rl[CALL];
                            const cr = this.first ? 0 : rr[CALL];
                            this.first = false;
                            this.renderCall(rl, rr, 1);
                            rl[0] = cl;
                            rr[0] = cr;
                            this.pos -= CALL;
                        }
                        const k = Math.floor(this.pos);
                        const fr = this.pos - k;
                        L[i] = rl[k] + (rl[k + 1] - rl[k]) * fr;
                        R[i] = rr[k] + (rr[k + 1] - rr[k]) * fr;
                        this.pos += this.ratio;
                    }
                }
            } catch (err) {
                L.fill(0);
                if (R !== L) R.fill(0);
                this.fail(err);
                return false;
            }
            return true;
        }
    }

    registerProcessor('aurago-rb338', RB338Processor);
    void QUANTUM;
})();
