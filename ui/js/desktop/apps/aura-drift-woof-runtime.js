/* Aura Drift - the model runtime of the Underdog Woof voice commentary.
 *
 * The reference implementation (underdog_woof/runtime.py) is pure numpy. This file
 * is a transliteration of it: the same weights, the same arithmetic and the same
 * numbers, written against plain typed arrays so the model runs inside a Web Worker
 * without numpy, torch or a WebAssembly toolchain.
 *
 * The worker entry (aura-drift-woof-worker.js) pulls this file in and speaks with it.
 *
 * Model card: ConwayResearch/Underdog-Woof on the Hugging Face model hub.
 * Licence: Underdog Non-Commercial License 1.0 - free for non-commercial use with
 * the credit "Voice by Underdog Woof".
 *
 * The weights arrive as a zip of stored (uncompressed) .npy arrays, so reading them
 * needs a zip directory walk and an .npy header parse, nothing more.
 */
(function () {
    'use strict';

    const SR = 24000;
    const EPS = 1.1920929e-7;               /* np.finfo(np.float32).eps */
    const MAX_LEN = 1100;                    /* speech model sequence limit */

    /* The text side (the vocabulary and the way numbers are said) lives in its own
       file and is loaded before this one. */
    const TEXT = self.AuraDriftWoofText;
    if (!TEXT) throw new Error('woof: the text runtime is missing');

    /* ---------------------------------------------------------------- zip ---- */

    /* The weights are written with numpy's savez, which stores the members
     * uncompressed. Anything else would need an inflate, which this does not have. */
    function readZip(buffer) {
        const view = new DataView(buffer);
        const bytes = new Uint8Array(buffer);
        let eocd = -1;
        const floor = Math.max(0, buffer.byteLength - 66000);
        for (let i = buffer.byteLength - 22; i >= floor; i--) {
            if (view.getUint32(i, true) === 0x06054b50) { eocd = i; break; }
        }
        if (eocd < 0) throw new Error('woof: no zip directory');
        const count = view.getUint16(eocd + 10, true);
        let off = view.getUint32(eocd + 16, true);
        const entries = new Map();
        for (let i = 0; i < count; i++) {
            if (view.getUint32(off, true) !== 0x02014b50) throw new Error('woof: broken zip directory');
            const method = view.getUint16(off + 10, true);
            const size = view.getUint32(off + 24, true);
            const nameLen = view.getUint16(off + 28, true);
            const extraLen = view.getUint16(off + 30, true);
            const commentLen = view.getUint16(off + 32, true);
            const local = view.getUint32(off + 42, true);
            const name = decodeBytes(bytes.subarray(off + 46, off + 46 + nameLen), 'utf-8');
            const localNameLen = view.getUint16(local + 26, true);
            const localExtraLen = view.getUint16(local + 28, true);
            entries.set(name, {
                method: method,
                size: size,
                start: local + 30 + localNameLen + localExtraLen
            });
            off += 46 + nameLen + extraLen + commentLen;
        }
        return entries;
    }

    function decodeBytes(bytes, encoding) {
        if (typeof TextDecoder === 'function') return new TextDecoder(encoding).decode(bytes);
        let s = '';
        for (let i = 0; i < bytes.length; i++) s += String.fromCharCode(bytes[i]);
        return s;
    }

    /* --------------------------------------------------------------- npy ---- */

    const halfScratch = new DataView(new ArrayBuffer(4));

    /* IEEE 754 binary16 to binary32, exact for every value. */
    function halfToFloat(h) {
        const sign = (h & 0x8000) >> 15;
        const exp = (h & 0x7c00) >> 10;
        const frac = h & 0x03ff;
        let bits;
        if (exp === 0) {
            if (frac === 0) {
                bits = sign << 31;
            } else {
                let e = -1, f = frac;
                do { f <<= 1; e++; } while (!(f & 0x400));
                bits = (sign << 31) | ((127 - 15 - e) << 23) | ((f & 0x3ff) << 13);
            }
        } else if (exp === 0x1f) {
            bits = (sign << 31) | 0x7f800000 | (frac << 13);
        } else {
            bits = (sign << 31) | ((exp - 15 + 127) << 23) | (frac << 13);
        }
        halfScratch.setUint32(0, bits, true);
        return halfScratch.getFloat32(0, true);
    }

    function readNpy(buffer, entry) {
        if (entry.method !== 0) throw new Error('woof: compressed weights are not supported');
        const bytes = new Uint8Array(buffer);
        const view = new DataView(buffer);
        const base = entry.start;
        const major = bytes[base + 6];
        let headerLen, headerStart;
        if (major === 1) {
            headerLen = view.getUint16(base + 8, true);
            headerStart = base + 10;
        } else {
            headerLen = view.getUint32(base + 8, true);
            headerStart = base + 12;
        }
        const header = decodeBytes(bytes.subarray(headerStart, headerStart + headerLen), 'latin1');
        const descr = /'descr'\s*:\s*'([^']*)'/.exec(header)[1];
        const fortran = /'fortran_order'\s*:\s*(True|False)/.exec(header)[1] === 'True';
        const shapeText = /'shape'\s*:\s*\(([^)]*)\)/.exec(header)[1].trim();
        /* numpy writes a one element tuple as "(448,)", so the trailing empty part
         * has to go before the numbers are read. */
        const shape = shapeText === '' ? []
            : shapeText.split(',').map(s => parseInt(s.trim(), 10)).filter(v => Number.isFinite(v));
        const dataStart = headerStart + headerLen;
        const count = shape.reduce((a, b) => a * b, 1);
        let data = null;
        if (descr === '|i1' || descr === '<i1' || descr === 'i1') {
            data = new Int8Array(count);
            for (let i = 0; i < count; i++) data[i] = view.getInt8(dataStart + i);
        } else if (descr === '<f4' || descr === '=f4' || descr === 'f4') {
            data = new Float32Array(count);
            for (let i = 0; i < count; i++) data[i] = view.getFloat32(dataStart + i * 4, true);
        } else if (descr === '<f2' || descr === '=f2' || descr === 'f2') {
            data = new Float32Array(count);
            for (let i = 0; i < count; i++) data[i] = halfToFloat(view.getUint16(dataStart + i * 2, true));
        } else if (descr.charAt(0) === '<' && descr.charAt(1) === 'U') {
            /* The weight tree is one unicode scalar array; its shape is empty, so the
             * length comes from the dtype (four bytes per scalar). */
            const scalars = parseInt(descr.slice(2), 10) || Math.floor(entry.size / 4);
            let s = '';
            for (let i = 0; i < scalars; i++) s += String.fromCharCode(view.getUint32(dataStart + i * 4, true));
            data = s;
        } else {
            throw new Error('woof: unsupported dtype ' + descr);
        }
        return { data: data, shape: shape, fortran: fortran, descr: descr };
    }

    /* ----------------------------------------------------------- tensors ---- */

    /* A weight tree entry is either a nested structure or {"@": "/path/to/array"}. */
    function isRef(node) {
        return node && typeof node === 'object' && !Array.isArray(node) &&
            Object.keys(node).length === 1 && typeof node['@'] === 'string';
    }

    function makeStore(buffer) {
        const zip = readZip(buffer);
        const cache = new Map();
        function tensor(ref) {
            const path = ref['@'];
            let hit = cache.get(path);
            if (hit) return hit;
            /* The tree is stored without a leading slash, the weights with one. */
            const bare = path.replace(/^\//, '');
            let entry = zip.get(bare + '.npy');
            if (!entry) entry = zip.get('/' + bare + '.npy');
            if (!entry) throw new Error('woof: missing tensor ' + path);
            hit = readNpy(buffer, entry);
            cache.set(path, hit);
            return hit;
        }
        return { tensor: tensor };
    }

    /* numpy's reshape only reinterprets the flat buffer, it never permutes it. */
    function reshape(flat, srcRows, srcCols, dstShape) {
        const total = srcRows * srcCols;
        const out = new Float32Array(total);
        for (let i = 0; i < total; i++) out[i] = flat[i];
        return out;
    }

    /* numpy moveaxis for a single axis, which is what the decoder quantisation uses. */
    function moveAxis(arr, shape, from, to) {
        if (from === to) return arr;
        const nd = shape.length;
        const order = [];
        for (let i = 0; i < nd; i++) order.push(i);
        const moved = order.splice(from, 1)[0];
        order.splice(to, 0, moved);
        const newShape = order.map(i => shape[i]);
        const strides = [];
        let acc = 1;
        for (let i = nd - 1; i >= 0; i--) { strides[i] = acc; acc *= shape[i]; }
        const newStrides = order.map(i => strides[i]);
        const out = new Float32Array(arr.length);
        const idx = new Array(nd).fill(0);
        for (let i = 0; i < arr.length; i++) {
            let src = 0;
            for (let d = 0; d < nd; d++) src += idx[d] * strides[order[d]];
            out[i] = arr[src];
            for (let d = nd - 1; d >= 0; d--) {
                idx[d]++;
                if (idx[d] < newShape[d]) break;
                idx[d] = 0;
            }
        }
        return out;
    }

    /* Grouped int8 dequantisation, a transliteration of runtime.deq.
     * The scales sit in groups of g along the quantised axis. */
    function deqGrouped(node, store) {
        const q = store.tensor(node.q8);
        const s = store.tensor(node.s);
        const shape = node.shape;
        const axis = node.axis, g = node.g;
        const rows = q.shape[0], cols = q.shape[1];
        const groups = s.shape[1];
        const m = new Float32Array(rows * cols);
        for (let r = 0; r < rows; r++) {
            const qo = r * cols, so = r * groups;
            for (let c = 0; c < cols; c++) m[qo + c] = q.data[qo + c] * s.data[so + ((c / g) | 0)];
        }
        const rest = shape.filter((_, i) => i !== axis);
        const dstShape = [shape[axis]].concat(rest);
        const moved = reshape(m, rows, cols, dstShape);
        return { data: moveAxis(moved, dstShape, 0, axis), shape: dstShape };
    }

    /* The speech model's linear weights: int8 with one scale per output row. */
    function linW(node, store) {
        if (node.q !== undefined) {
            const q = store.tensor(node.q);
            const s = store.tensor(node.s);
            const out = q.shape[0], inner = q.shape[1];
            const w = new Float32Array(out * inner);
            for (let o = 0; o < out; o++) {
                const scale = s.data[s.shape.length === 1 ? o : o * s.shape[1]];
                const base = o * inner;
                for (let i = 0; i < inner; i++) w[base + i] = q.data[base + i] * scale;
            }
            return { w: w, out: out, inner: inner, b: node.b ? store.tensor(node.b).data : null };
        }
        const t = store.tensor(node.w);
        const inner = t.shape[1], out = t.shape[0];
        return { w: t.data, out: out, inner: inner, b: node.b ? store.tensor(node.b).data : null };
    }

    /* ---------------------------------------------------------- matmul ------- */

    /* All linear layers of this model are stored output major, so every product is
     * a dot product per output row. Eight rows at a time keep the input in a
     * register and the weights streaming. */
    function matmulDot(A, W, M, K, N, out, add) {
        out = out || new Float32Array(M * N);
        for (let m = 0; m < M; m++) {
            const ao = m * K, oo = m * N;
            let n = 0;
            for (; n + 8 <= N; n += 8) {
                let s0 = 0, s1 = 0, s2 = 0, s3 = 0, s4 = 0, s5 = 0, s6 = 0, s7 = 0;
                const w0 = n * K, w1 = (n + 1) * K, w2 = (n + 2) * K, w3 = (n + 3) * K;
                const w4 = (n + 4) * K, w5 = (n + 5) * K, w6 = (n + 6) * K, w7 = (n + 7) * K;
                for (let k = 0; k < K; k++) {
                    const a = A[ao + k];
                    s0 += a * W[w0 + k]; s1 += a * W[w1 + k]; s2 += a * W[w2 + k]; s3 += a * W[w3 + k];
                    s4 += a * W[w4 + k]; s5 += a * W[w5 + k]; s6 += a * W[w6 + k]; s7 += a * W[w7 + k];
                }
                if (add) {
                    out[oo + n] += s0; out[oo + n + 1] += s1; out[oo + n + 2] += s2; out[oo + n + 3] += s3;
                    out[oo + n + 4] += s4; out[oo + n + 5] += s5; out[oo + n + 6] += s6; out[oo + n + 7] += s7;
                } else {
                    out[oo + n] = s0; out[oo + n + 1] = s1; out[oo + n + 2] = s2; out[oo + n + 3] = s3;
                    out[oo + n + 4] = s4; out[oo + n + 5] = s5; out[oo + n + 6] = s6; out[oo + n + 7] = s7;
                }
            }
            for (; n < N; n++) {
                const wo = n * K;
                let s = 0;
                for (let k = 0; k < K; k++) s += A[ao + k] * W[wo + k];
                if (add) out[oo + n] += s; else out[oo + n] = s;
            }
        }
        return out;
    }

    /* A single output row: out[n] = dot(x, W[n]). */
    function matvecDot(x, W, N, K, out) {
        out = out || new Float32Array(N);
        let n = 0;
        for (; n + 4 <= N; n += 4) {
            const w0 = n * K, w1 = (n + 1) * K, w2 = (n + 2) * K, w3 = (n + 3) * K;
            let s0 = 0, s1 = 0, s2 = 0, s3 = 0;
            for (let k = 0; k < K; k++) {
                const a = x[k];
                s0 += a * W[w0 + k]; s1 += a * W[w1 + k]; s2 += a * W[w2 + k]; s3 += a * W[w3 + k];
            }
            out[n] = s0; out[n + 1] = s1; out[n + 2] = s2; out[n + 3] = s3;
        }
        for (; n < N; n++) {
            const wo = n * K;
            let s = 0;
            for (let k = 0; k < K; k++) s += x[k] * W[wo + k];
            out[n] = s;
        }
        return out;
    }

    /* ------------------------------------------------------------- maths ---- */

    function silu(x) {
        return x / (1 + Math.exp(-x));
    }

    /* Abramowitz-Stegun 7.1.26, the same approximation the reference runtime uses. */
    function erf(x) {
        const a = Math.abs(x);
        const t = 1 / (1 + 0.3275911 * a);
        const y = 1 - (((((1.061405429 * t - 1.453152027) * t + 1.421413741) * t - 0.284496736) * t + 0.254829592) * t * Math.exp(-a * a));
        return x < 0 ? -y : y;
    }

    function gelu(x) {
        return 0.5 * x * (1 + erf(x * 0.7071067811865476));
    }

    function eluInPlace(x) {
        for (let i = 0; i < x.length; i++) {
            const v = x[i];
            if (v > 0) x[i] = v;
            else x[i] = Math.expm1(v);
        }
        return x;
    }

    function rmsRow(x, offset, len) {
        let ss = 0;
        for (let i = 0; i < len; i++) ss += x[offset + i] * x[offset + i];
        return 1 / Math.sqrt(ss / len + EPS);
    }

    function layerNorm(x, weight, bias, eps) {
        const n = x.length;
        let mean = 0;
        for (let i = 0; i < n; i++) mean += x[i];
        mean /= n;
        let varr = 0;
        for (let i = 0; i < n; i++) { const d = x[i] - mean; varr += d * d; }
        varr /= n;
        const inv = 1 / Math.sqrt(varr + eps);
        if (!weight) { for (let i = 0; i < n; i++) x[i] = (x[i] - mean) * inv; return x; }
        for (let i = 0; i < n; i++) x[i] = (x[i] - mean) * inv * weight[i] + bias[i];
        return x;
    }

    /* -------------------------------------------------------------- rng ----- */

    /* The reference draws its noise from torch's mt19937, so the generator has to
     * match bit for bit or every take comes out different. */
    function MT19937(seed) {
        this.mt = new Uint32Array(624);
        this.index = 625;
        this.seed(seed);
    }
    MT19937.prototype.seed = function (seed) {
        const mt = this.mt;
        mt[0] = seed >>> 0;
        for (let i = 1; i < 624; i++) {
            const prev = mt[i - 1];
            mt[i] = (Math.imul(1812433253, (prev ^ (prev >>> 30)) >>> 0) + i) >>> 0;
        }
        this.index = 624;
    };
    MT19937.prototype.next = function () {
        if (this.index >= 624) {
            const mt = this.mt;
            for (let i = 0; i < 624; i++) {
                const y = ((mt[i] & 0x80000000) | (mt[(i + 1) % 624] & 0x7fffffff)) >>> 0;
                let v = (mt[(i + 397) % 624] ^ (y >>> 1)) >>> 0;
                if (y & 1) v = (v ^ 0x9908b0df) >>> 0;
                mt[i] = v;
            }
            this.index = 0;
        }
        let y = this.mt[this.index++];
        y ^= y >>> 11;
        y = (y ^ ((y << 7) & 0x9d2c5680)) >>> 0;
        y = (y ^ ((y << 15) & 0xefc60000)) >>> 0;
        y = (y ^ (y >>> 18)) >>> 0;
        return y;
    };

    /* torch.randn(n) in float32: n 24 bit uniforms, then Box-Muller in blocks of sixteen. */
    function randn(rng, n) {
        const u = new Float64Array(n);
        for (let i = 0; i < n; i++) u[i] = (rng.next() & 0xffffff) * 5.960464477539063e-8;
        const out = new Float32Array(n);
        for (let a = 0; a < n / 16; a++) {
            for (let c = 0; c < 8; c++) {
                const r = Math.sqrt(-2 * Math.log(1 - u[a * 16 + c]));
                const th = 2 * Math.PI * u[a * 16 + 8 + c];
                out[a * 16 + c] = r * Math.cos(th);
                out[a * 16 + 8 + c] = r * Math.sin(th);
            }
        }
        return out;
    }

    /* ------------------------------------------------------------ speech ---- */

    function Speech(st, store, voices) {
        const c = st.cfg;
        this.L = c.L; this.d = c.d; this.H = c.H; this.ff = c.ff; this.hw = c.hw;
        this.latent = c.latent; this.depth = c.depth;
        this.Dh = (this.d / this.H) | 0;
        this.layers = st.layers.map(layer => layer.map(e => linW(e, store)));
        this.ce = linW(st.ce, store);
        this.ada = linW(st.ada, store);
        this.hblocks = st.hblocks.map(fs => fs.map(e => linW(e, store)));
        this.latIn = linW(st.lat_in, store);
        this.hInp = linW(st.h_inp, store);
        this.hOut = linW(st.h_out, store);
        const emb = store.tensor(st.text_emb);
        this.textEmb = emb.data;
        this.bos = store.tensor(st.bos).data;
        this.voices = {};
        for (const key of Object.keys(voices || {})) {
            this.voices[key] = store.tensor(voices[key]).data;
        }

        /* Rotary position tables, exactly as the reference builds them. */
        const Dh = this.Dh, half = Dh >> 1;
        const inv = new Float32Array(half);
        for (let i = 0; i < half; i++) inv[i] = 1 / Math.pow(c.rope_base, (2 * i) / Dh);
        this.cos = new Float32Array(MAX_LEN * Dh);
        this.sin = new Float32Array(MAX_LEN * Dh);
        for (let p = 0; p < MAX_LEN; p++) {
            for (let i = 0; i < half; i++) {
                const a = p * inv[i];
                this.cos[p * Dh + i] = Math.cos(a);
                this.cos[p * Dh + i + half] = Math.cos(a);
                this.sin[p * Dh + i] = -Math.sin(a);
                this.sin[p * Dh + i + half] = Math.sin(a);
            }
        }
        this.scale = Math.pow(Dh, -0.5);
        this.kvLen = 0;
        this.kv = null;
        this.growKv(256);
    }

    Speech.prototype.growKv = function (len) {
        const want = Math.min(MAX_LEN, Math.max(len, this.kvLen * 2, 256));
        if (want <= this.kvLen) return;
        const fresh = new Float32Array(this.L * 2 * this.H * want * this.Dh);
        if (this.kv) {
            const oldStride = this.kvLen * this.Dh;
            for (let l = 0; l < this.L; l++) {
                for (let k = 0; k < 2; k++) {
                    for (let h = 0; h < this.H; h++) {
                        const from = l * 2 * this.H * oldStride + k * this.H * oldStride + h * oldStride;
                        const to = (l * 2 * this.H * want + k * this.H * want + h * want) * this.Dh;
                        fresh.set(this.kv.subarray(from, from + oldStride), to);
                    }
                }
            }
        }
        this.kv = fresh;
        this.kvLen = want;
    };

    /* The prefix block runs over the whole prompt at once and fills the cache. */
    Speech.prototype.prefixBlock = function (layer, x, n) {
        const d = this.d, H = this.H, Dh = this.Dh, half = Dh >> 1;
        const l0 = this.layers[layer][0], l1 = this.layers[layer][1];
        const l2 = this.layers[layer][2], l3 = this.layers[layer][3];

        const xn = new Float32Array(n * d);
        for (let t = 0; t < n; t++) {
            const o = t * d, inv = rmsRow(x, o, d);
            for (let i = 0; i < d; i++) xn[o + i] = x[o + i] * inv;
        }

        const qkv = matmulDot(xn, l0.w, n, d, 3 * d);
        const q = new Float32Array(n * H * Dh);
        const k = new Float32Array(n * H * Dh);
        const v = new Float32Array(n * H * Dh);
        const cos = this.cos, sin = this.sin;
        for (let t = 0; t < n; t++) {
            const cbase = t * Dh;
            for (let h = 0; h < H; h++) {
                for (let w = 0; w < 2; w++) {
                    const src = ((t * 3 + w) * H + h) * Dh;
                    const dst = t * H * Dh + h * Dh;
                    const target = w === 0 ? q : k;
                    for (let i = 0; i < Dh; i++) {
                        const rot = i < half ? qkv[src + i + half] : qkv[src + i - half];
                        target[dst + i] = qkv[src + i] * cos[cbase + i] + rot * sin[cbase + i];
                    }
                }
                const vsrc = ((t * 3 + 2) * H + h) * Dh;
                const vdst = t * H * Dh + h * Dh;
                for (let i = 0; i < Dh; i++) v[vdst + i] = qkv[vsrc + i];
            }
        }

        const kvStride = this.kvLen * Dh;
        const kBase = layer * 2 * this.H * kvStride;
        for (let t = 0; t < n; t++) {
            for (let h = 0; h < H; h++) {
                const src = t * H * Dh + h * Dh;
                const dst = kBase + h * kvStride + t * Dh;
                for (let i = 0; i < Dh; i++) this.kv[dst + i] = k[src + i];
                const vdst = kBase + this.H * kvStride + h * kvStride + t * Dh;
                for (let i = 0; i < Dh; i++) this.kv[vdst + i] = v[src + i];
            }
        }

        /* Causal attention over the prompt. */
        const att = new Float32Array(H * n * n);
        for (let h = 0; h < H; h++) {
            for (let a = 0; a < n; a++) {
                const qo = (a * H + h) * Dh;
                const ko = kBase + h * kvStride;
                let max = -Infinity;
                const scores = new Float32Array(a + 1);
                for (let b = 0; b <= a; b++) {
                    let s = 0;
                    const base = ko + b * Dh;
                    for (let i = 0; i < Dh; i++) s += q[qo + i] * this.kv[base + i];
                    s *= this.scale;
                    scores[b] = s;
                    if (s > max) max = s;
                }
                let sum = 0;
                for (let b = 0; b <= a; b++) { const e = Math.exp(scores[b] - max); scores[b] = e; sum += e; }
                const inv = 1 / sum;
                for (let b = 0; b <= a; b++) att[h * n * n + a * n + b] = scores[b] * inv;
            }
        }

        const ctx = new Float32Array(n * d);
        for (let h = 0; h < H; h++) {
            const vo = kBase + this.H * kvStride + h * kvStride;
            for (let a = 0; a < n; a++) {
                const co = a * d + h * Dh;
                for (let b = 0; b <= a; b++) {
                    const w = att[h * n * n + a * n + b];
                    if (w === 0) continue;
                    const base = vo + b * Dh;
                    for (let i = 0; i < Dh; i++) ctx[co + i] += w * this.kv[base + i];
                }
            }
        }

        /* The projection needs its own buffer: the kernel writes its result while it
         * still reads the input. */
        const proj = matmulDot(ctx, l1.w, n, d, d);
        for (let i = 0; i < n * d; i++) x[i] += proj[i];

        /* the block normalises the updated x again before the gated part */
        const xn2 = new Float32Array(n * d);
        for (let t = 0; t < n; t++) {
            const o = t * d, inv = rmsRow(x, o, d);
            for (let i = 0; i < d; i++) xn2[o + i] = x[o + i] * inv;
        }
        const gu = matmulDot(xn2, l2.w, n, d, 2 * this.ff);
        const gate = new Float32Array(n * this.ff);
        for (let t = 0; t < n; t++) {
            const o = t * 2 * this.ff, g = t * this.ff;
            for (let i = 0; i < this.ff; i++) {
                const a = gu[o + i], b = gu[o + this.ff + i];
                gate[g + i] = silu(a) * b;
            }
        }
        matmulDot(gate, l3.w, n, this.ff, d, ctx);
        for (let i = 0; i < n * d; i++) x[i] += ctx[i];
    };

    /* One decoding step: the same block, but against the cache. */
    Speech.prototype.blockStep = function (layer, x, p) {
        const d = this.d, H = this.H, Dh = this.Dh, half = Dh >> 1;
        const l0 = this.layers[layer][0], l1 = this.layers[layer][1];
        const l2 = this.layers[layer][2], l3 = this.layers[layer][3];
        const inv = rmsRow(x, 0, d);

        const qkv = new Float32Array(3 * d);
        matvecDot(x, l0.w, 3 * d, d, qkv);
        for (let i = 0; i < 3 * d; i++) qkv[i] *= inv;

        const cos = this.cos, sin = this.sin;
        const q = new Float32Array(H * Dh);
        const k = new Float32Array(H * Dh);
        const v = new Float32Array(H * Dh);
        for (let h = 0; h < H; h++) {
            for (let w = 0; w < 2; w++) {
                const src = (w * H + h) * Dh;
                const dst = h * Dh;
                const target = w === 0 ? q : k;
                for (let i = 0; i < Dh; i++) {
                    const rot = i < half ? qkv[src + i + half] : qkv[src + i - half];
                    target[dst + i] = qkv[src + i] * cos[p * Dh + i] + rot * sin[p * Dh + i];
                }
            }
            const vsrc = (2 * H + h) * Dh;
            for (let i = 0; i < Dh; i++) v[h * Dh + i] = qkv[vsrc + i];
        }

        const kvStride = this.kvLen * Dh;
        const kBase = layer * 2 * this.H * kvStride;
        for (let h = 0; h < H; h++) {
            const dst = kBase + h * kvStride + p * Dh;
            for (let i = 0; i < Dh; i++) this.kv[dst + i] = k[h * Dh + i];
            const vdst = kBase + this.H * kvStride + h * kvStride + p * Dh;
            for (let i = 0; i < Dh; i++) this.kv[vdst + i] = v[h * Dh + i];
        }

        const scores = new Float32Array(p + 1);
        const ctx = new Float32Array(d);
        for (let h = 0; h < H; h++) {
            const ko = kBase + h * kvStride;
            let max = -Infinity;
            for (let b = 0; b <= p; b++) {
                let s = 0;
                const base = ko + b * Dh;
                for (let i = 0; i < Dh; i++) s += q[h * Dh + i] * this.kv[base + i];
                s *= this.scale;
                scores[b] = s;
                if (s > max) max = s;
            }
            let sum = 0;
            for (let b = 0; b <= p; b++) { const e = Math.exp(scores[b] - max); scores[b] = e; sum += e; }
            const invSum = 1 / sum;
            const vo = kBase + this.H * kvStride + h * kvStride;
            for (let b = 0; b <= p; b++) {
                const w = scores[b] * invSum;
                const base = vo + b * Dh;
                for (let i = 0; i < Dh; i++) ctx[h * Dh + i] += w * this.kv[base + i];
            }
        }

        const proj = matvecDot(ctx, l1.w, d, d);
        for (let i = 0; i < d; i++) x[i] += proj[i];

        const inv2 = rmsRow(x, 0, d);
        const gu = new Float32Array(2 * this.ff);
        matvecDot(x, l2.w, 2 * this.ff, d, gu);
        const gate = new Float32Array(this.ff);
        for (let i = 0; i < this.ff; i++) gate[i] = silu(gu[i] * inv2) * gu[this.ff + i] * inv2;
        const out = matvecDot(gate, l3.w, d, this.ff);
        for (let i = 0; i < d; i++) x[i] += out[i];
    };

    /* The conditioning head: the silu part drives the next step, the extra logit ends it. */
    Speech.prototype.cond = function (x) {
        const ce = matvecDot(x, this.ce.w, this.ce.out, this.ce.inner);
        const inv = rmsRow(x, 0, this.d);
        /* the reference scales first and adds the bias afterwards */
        const bias = this.ce.b;
        for (let i = 0; i < ce.length; i++) ce[i] = ce[i] * inv + (bias ? bias[i] : 0);
        const hw = this.hw;
        const s = new Float32Array(hw);
        for (let i = 0; i < hw; i++) s[i] = silu(ce[i]);
        return { s: s, e: ce[hw] };
    };

    Speech.prototype.head = function (s, z) {
        const hw = this.hw;
        const A = matvecDot(s, this.ada.w, this.ada.out, this.ada.inner);
        if (this.ada.b) for (let i = 0; i < A.length; i++) A[i] += this.ada.b[i];
        let h = matvecDot(z, this.hInp.w, this.hInp.out, this.hInp.inner);
        if (this.hInp.b) for (let i = 0; i < h.length; i++) h[i] += this.hInp.b[i];
        for (let j = 0; j < this.depth; j++) {
            const o = 3 * hw * j;
            const y = new Float32Array(hw);
            const norm = layerNorm(h.slice(), null, null, 1e-5);
            for (let i = 0; i < hw; i++) y[i] = A[o + i] + norm[i] * A[o + hw + i];
            const b1 = this.hblocks[j][0], b2 = this.hblocks[j][1];
            const inner = matvecDot(y, b1.w, b1.out, b1.inner);
            if (b1.b) for (let i = 0; i < inner.length; i++) inner[i] += b1.b[i];
            for (let i = 0; i < inner.length; i++) inner[i] = silu(inner[i]);
            const upd = matvecDot(inner, b2.w, b2.out, b2.inner);
            if (b2.b) for (let i = 0; i < upd.length; i++) upd[i] += b2.b[i];
            for (let i = 0; i < hw; i++) h[i] += A[o + 2 * hw + i] * upd[i];
        }
        const o = 3 * hw * this.depth;
        const norm = layerNorm(h.slice(), null, null, 1e-5);
        const mix = new Float32Array(hw);
        for (let i = 0; i < hw; i++) mix[i] = A[o + i] + norm[i] * A[o + hw + i];
        const out = matvecDot(mix, this.hOut.w, this.hOut.out, this.hOut.inner);
        if (this.hOut.b) for (let i = 0; i < out.length; i++) out[i] += this.hOut.b[i];
        return out;
    };

    Speech.prototype.generate = function (rng, voice, textIds, maxFrames, temp, eosThresh, minFramesPerChar) {
        const d = this.d;
        const voiceRows = voice.length / d;
        const p0 = voiceRows + textIds.length + 1;
        if (p0 + maxFrames > this.kvLen) this.growKv(p0 + maxFrames + 64);
        this.kv.fill(0);

        const x = new Float32Array(p0 * d);
        x.set(voice, 0);
        for (let i = 0; i < textIds.length; i++) {
            const id = textIds[i];
            x.set(this.textEmb.subarray(id * d, (id + 1) * d), (voiceRows + i) * d);
        }
        x.set(this.bos, (voiceRows + textIds.length) * d);

        for (let i = 0; i < this.L; i++) this.prefixBlock(i, x, p0);

        const last = new Float32Array(d);
        for (let i = 0; i < d; i++) last[i] = x[(p0 - 1) * d + i];
        let cond = this.cond(last);

        const eosT = Math.log(eosThresh / (1 - eosThresh));
        const minFrames = Math.floor(minFramesPerChar * textIds.length);
        const out = new Float32Array(maxFrames * this.latent);
        let count = 0;
        const step = new Float32Array(d);
        for (let f = 0; f < maxFrames; f++) {
            if (f > Math.max(3, minFrames) && cond.e > eosT) break;
            const noise = randn(rng, this.latent);
            for (let i = 0; i < this.latent; i++) noise[i] *= temp;
            const z = this.head(cond.s, noise);
            /* the flow head predicts a step, the noise is added on top of it */
            for (let i = 0; i < this.latent; i++) z[i] += noise[i];
            out.set(z, f * this.latent);
            count = f + 1;
            matvecDot(z, this.latIn.w, d, this.latent, step);
            if (this.latIn.b) for (let i = 0; i < d; i++) step[i] += this.latIn.b[i];
            for (let i = 0; i < this.L; i++) this.blockStep(i, step, p0 + f);
            cond = this.cond(step);
        }
        return out.subarray(0, count * this.latent);
    };

    /* ---------------------------------------------------------- decoder ---- */

    /* A plain convolution: out[t] = sum over the taps of x[t+j] with the weight row. */
    function Conv(node, store) {
        const w = deqGrouped(node.w, store);
        const shape = node.w.shape;
        const cout = shape[0], cin = shape[1], k = shape[2];
        this.cout = cout; this.cin = cin; this.k = k;
        /* One contiguous matrix per tap, output major. */
        this.weights = [];
        for (let j = 0; j < k; j++) {
            const mat = new Float32Array(cout * cin);
            for (let o = 0; o < cout; o++) {
                const src = o * cin * k + j;
                for (let i = 0; i < cin; i++) mat[o * cin + i] = w.data[src + i * k];
            }
            this.weights.push(mat);
        }
        this.b = store.tensor(node.b).data;
        this.state = null;
    }
    Conv.prototype.reset = function () {
        this.state = new Float32Array((this.k - 1) * this.cin);
    };
    Conv.prototype.call = function (x, res) {
        const T = x.length / this.cin, k = this.k, cin = this.cin, cout = this.cout;
        if (!this.state) this.reset();
        const padded = new Float32Array((T + k - 1) * cin);
        padded.set(this.state, 0);
        padded.set(x, (k - 1) * cin);
        this.state = padded.subarray(T * cin, (T + k - 1) * cin).slice();
        const out = new Float32Array(T * cout);
        for (let j = 0; j < k; j++) {
            const slice = padded.subarray(j * cin, j * cin + T * cin);
            matmulDot(slice, this.weights[j], T, cin, cout, out, true);
        }
        for (let i = 0; i < T * cout; i++) out[i] += this.b[i % cout];
        if (res) for (let i = 0; i < T * cout; i++) out[i] += res[i];
        return out;
    };

    /* A transposed convolution. The reference computes every tap and then drops the
     * ones that never reach the output, which halves the work here. */
    function ConvT(node, store) {
        const w = deqGrouped(node.w, store);
        const shape = node.w.shape;
        const cin = shape[0], cout = shape[1], k = shape[2];
        const stride = node.stride;
        this.cin = cin; this.cout = cout; this.k = k; this.stride = stride;
        /* W1 carries the taps that land on the current row, W2 the ones from the
         * previous row. */
        this.w1 = new Float32Array(stride * cout * cin);
        this.w2 = new Float32Array(stride * cout * cin);
        for (let r = 0; r < stride; r++) {
            for (let cc = 0; cc < cout; cc++) {
                const o = (r * cout + cc) * cin;
                for (let c = 0; c < cin; c++) {
                    this.w1[o + c] = w.data[(c * cout + cc) * k + r];
                    this.w2[o + c] = w.data[(c * cout + cc) * k + r + stride];
                }
            }
        }
        this.b = store.tensor(node.b).data;
        this.carry = null;
    }
    ConvT.prototype.reset = function () {
        this.carry = new Float32Array(this.stride * this.cout);
    };
    ConvT.prototype.call = function (x) {
        const T = x.length / this.cin, cin = this.cin, cout = this.cout, stride = this.stride;
        if (!this.carry) this.reset();
        const z1 = matmulDot(x, this.w1, T, cin, stride * cout);
        const z2 = matmulDot(x, this.w2, T, cin, stride * cout);
        const out = new Float32Array(T * stride * cout);
        for (let t = 0; t < T; t++) {
            for (let i = 0; i < stride * cout; i++) {
                out[t * stride * cout + i] = z1[t * stride * cout + i] + (t > 0 ? z2[(t - 1) * stride * cout + i] : this.carry[i]);
            }
        }
        for (let t = 1; t < T; t++) {
            for (let i = 0; i < stride * cout; i++) out[t * stride * cout + i] = out[t * stride * cout + i];
        }
        /* the bias is added per output row, the reference tiles it over the stride */
        for (let t = 0; t < T; t++) {
            for (let s = 0; s < stride; s++) {
                for (let cc = 0; cc < cout; cc++) {
                    out[(t * stride + s) * cout + cc] += this.b[cc];
                }
            }
        }
        for (let i = 0; i < stride * cout; i++) this.carry[i] = z2[(T - 1) * stride * cout + i];
        return out;
    };

    function Decoder(st, store, latent) {
        const c = st.cfg;
        this.nh = c.heads; this.hd = c.head_dim; this.window = c.window; this.eps = c.eps;
        this.latent = latent;
        this.D = this.nh * this.hd;
        this.split = 6;
        const inp = store.tensor(st.inp[0]);
        this.inpW = inp.data;
        this.inpB = store.tensor(st.inp[1]).data;
        this.invFreq = store.tensor(st.inv_freq).data;
        this.tf = st.tf.map(layer => {
            const qkv = deqGrouped(layer.qkv, store);
            const o = deqGrouped(layer.o, store);
            const fc1 = deqGrouped(layer.fc1, store);
            const fc2 = deqGrouped(layer.fc2, store);
            return {
                ln1: [store.tensor(layer.ln1[0]).data, store.tensor(layer.ln1[1]).data],
                ln2: [store.tensor(layer.ln2[0]).data, store.tensor(layer.ln2[1]).data],
                qkv: qkv.data, o: o.data, fc1: fc1.data, fc2: fc2.data
            };
        });
        this.ops = [];
        for (let idx = 0; idx < st.seanet.length; idx++) {
            const kind = st.seanet[idx][0];
            const param = st.seanet[idx][1];
            if (kind === 'conv') this.ops.push({ kind: 'conv', op: new Conv(param, store) });
            else if (kind === 'convt') this.ops.push({ kind: 'convt', op: new ConvT(param, store) });
            else if (kind === 'res') {
                this.ops.push({ kind: 'res', op: [new Conv(param[0], store), new Conv(param[1], store)] });
            } else this.ops.push({ kind: kind, op: null });
        }
    }

    Decoder.prototype.reset = function () {
        for (const entry of this.ops) {
            if (entry.kind === 'res') { entry.op[0].reset(); entry.op[1].reset(); }
            else if (entry.op) entry.op.reset();
        }
    };

    Decoder.prototype.transformer = function (h) {
        const T = h.length / this.D, D = this.D, nh = this.nh, hd = this.hd, half = hd >> 1;
        const cos = new Float32Array(T * hd), sin = new Float32Array(T * hd);
        for (let t = 0; t < T; t++) {
            for (let i = 0; i < half; i++) {
                const a = t * this.invFreq[i];
                cos[t * hd + i] = Math.cos(a);
                cos[t * hd + i + half] = Math.cos(a);
                sin[t * hd + i] = Math.sin(a);
                sin[t * hd + i + half] = Math.sin(a);
            }
        }
        for (const layer of this.tf) {
            const y = new Float32Array(h.length);
            for (let t = 0; t < T; t++) {
                const row = h.subarray(t * D, (t + 1) * D);
                const norm = new Float32Array(D);
                let mean = 0;
                for (let i = 0; i < D; i++) mean += row[i];
                mean /= D;
                let varr = 0;
                for (let i = 0; i < D; i++) { const dv = row[i] - mean; varr += dv * dv; }
                varr /= D;
                const inv = 1 / Math.sqrt(varr + this.eps);
                for (let i = 0; i < D; i++) norm[i] = (row[i] - mean) * inv;
                for (let i = 0; i < D; i++) y[t * D + i] = norm[i] * layer.ln1[0][i] + layer.ln1[1][i];
            }
            const qkv = matmulDot(y, layer.qkv, T, D, 3 * D);
            const q = new Float32Array(nh * T * hd);
            const k = new Float32Array(nh * T * hd);
            const v = new Float32Array(nh * T * hd);
            for (let t = 0; t < T; t++) {
                for (let hh = 0; hh < nh; hh++) {
                    for (let w = 0; w < 2; w++) {
                        const src = ((t * 3 + w) * nh + hh) * hd;
                        const dst = (hh * T + t) * hd;
                        const target = w === 0 ? q : k;
                        for (let i = 0; i < hd; i++) {
                            const rot = i < half ? -qkv[src + i + half] : qkv[src + i - half];
                            target[dst + i] = qkv[src + i] * cos[t * hd + i] + rot * sin[t * hd + i];
                        }
                    }
                    const vsrc = ((t * 3 + 2) * nh + hh) * hd;
                    const vdst = (hh * T + t) * hd;
                    for (let i = 0; i < hd; i++) v[vdst + i] = qkv[vsrc + i];
                }
            }
            const att = new Float32Array(nh * T * T);
            for (let hh = 0; hh < nh; hh++) {
                const scores = new Float32Array(T);
                for (let a = 0; a < T; a++) {
                    const lo = Math.max(0, a - this.window + 1);
                    let max = -Infinity;
                    for (let b = lo; b <= a; b++) {
                        let s = 0;
                        const qo = (hh * T + a) * hd, ko = (hh * T + b) * hd;
                        for (let i = 0; i < hd; i++) s += q[qo + i] * k[ko + i];
                        s /= Math.sqrt(hd);
                        scores[b] = s;
                        if (s > max) max = s;
                    }
                    let sum = 0;
                    for (let b = lo; b <= a; b++) { const e = Math.exp(scores[b] - max); scores[b] = e; sum += e; }
                    const inv = 1 / sum;
                    for (let b = lo; b <= a; b++) att[hh * T * T + a * T + b] = scores[b] * inv;
                }
            }
            const ctx = new Float32Array(T * D);
            for (let hh = 0; hh < nh; hh++) {
                for (let a = 0; a < T; a++) {
                    const co = (a * D + hh * hd);
                    for (let b = 0; b <= a; b++) {
                        const wv = att[hh * T * T + a * T + b];
                        if (wv === 0) continue;
                        const vo = (hh * T + b) * hd;
                        for (let i = 0; i < hd; i++) ctx[co + i] += wv * v[vo + i];
                    }
                }
            }
            const proj = matmulDot(ctx, layer.o, T, D, D);
            for (let i = 0; i < T * D; i++) h[i] += proj[i];
            const y2 = new Float32Array(h.length);
            for (let t = 0; t < T; t++) {
                const row = h.subarray(t * D, (t + 1) * D);
                let mean = 0;
                for (let i = 0; i < D; i++) mean += row[i];
                mean /= D;
                let varr = 0;
                for (let i = 0; i < D; i++) { const dv = row[i] - mean; varr += dv * dv; }
                varr /= D;
                const inv = 1 / Math.sqrt(varr + this.eps);
                for (let i = 0; i < D; i++) y2[t * D + i] = (row[i] - mean) * inv * layer.ln2[0][i] + layer.ln2[1][i];
            }
            const hidden = matmulDot(y2, layer.fc1, T, D, 2048);
            for (let i = 0; i < hidden.length; i++) hidden[i] = gelu(hidden[i]);
            matmulDot(hidden, layer.fc2, T, 2048, D, ctx);
            for (let i = 0; i < T * D; i++) h[i] += ctx[i];
        }
        return h;
    };

    Decoder.prototype.runOps = function (x, from, to) {
        for (let i = from; i < to; i++) {
            const entry = this.ops[i];
            if (entry.kind === 'elu') x = eluInPlace(x);
            else if (entry.kind === 'res') {
                const inner = eluInPlace(x.slice());
                const mid = entry.op[0].call(inner, null);
                const outer = eluInPlace(mid);
                x = entry.op[1].call(outer, x);
            } else x = entry.op.call(x);
        }
        return x;
    };

    Decoder.prototype.decode = function (lat) {
        const T = lat.length / this.latent;
        const h = new Float32Array(T * this.D);
        matmulDot(lat, this.inpW, T, this.latent, this.D, h);
        for (let i = 0; i < h.length; i++) h[i] += this.inpB[i % this.D];
        this.transformer(h);
        this.reset();
        const low = this.runOps(h, 0, this.split);
        const high = this.runOps(low, this.split, this.ops.length);
        /* the last convolution has one channel, so the samples are the whole buffer */
        const out = new Float32Array(high.length);
        for (let i = 0; i < out.length; i++) out[i] = high[i];
        return out;
    };

    class Woof {
        constructor(buffer) {
            const store = makeStore(buffer);
            const tree = JSON.parse(store.tensor({ '@': '/__tree__' }).data);
            this.tree = tree;
            this.speech = new Speech(tree.speech, store, tree.voices);
            this.decoder = new Decoder(tree.decoder, store, tree.speech.cfg.latent);
            this.mean = store.tensor(tree.mean).data;
            this.std = store.tensor(tree.std).data;
            this.maxFrames = tree.max_frames;
            this.fps = tree.fps;
            this.rng = new MT19937(0);
        }

        seed(seed) {
            this.rng.seed(seed >>> 0);
        }

        /* The latents of one utterance, the same way the reference draws them. */
        latents(text, voiceName, temp, eos) {
            const voice = this.speech.voices[voiceName];
            if (!voice) throw new Error('woof: unknown voice ' + voiceName);
            const ids = TEXT.encodeText(text);
            return this.speech.generate(this.rng, voice, ids, this.maxFrames, temp, eos, 0);
        }

        decode(lat) {
            const dim = this.speech.latent;
            const scaled = new Float32Array(lat.length);
            for (let i = 0; i < lat.length; i++) scaled[i] = lat[i] * this.std[i % dim] + this.mean[i % dim];
            return this.decoder.decode(scaled);
        }

        /* Text in, 24 kHz audio out. */
        speak(text, voiceName, seed) {
            if (seed !== undefined && seed !== null) this.seed(seed);
            const pieces = TEXT.splitText(TEXT.normalise(text));
            const wavs = [];
            for (const piece of pieces) {
                const lat = this.latents(piece, voiceName, 0.5, 0.9);
                wavs.push(this.decode(lat));
            }
            if (wavs.length === 1) return wavs[0];
            let total = 0;
            for (const w of wavs) total += w.length;
            const out = new Float32Array(total);
            let at = 0;
            for (const w of wavs) { out.set(w, at); at += w.length; }
            return out;
        }
    }


    /* Exposed for the node test harness and for the worker entry. */
    self.AuraDriftWoofRuntime = {
        MT19937: MT19937,
        randn: randn,
        readZip: readZip,
        readNpy: readNpy,
        makeStore: makeStore,
        deqGrouped: deqGrouped,
        linW: linW,
        matmulDot: matmulDot,
        matvecDot: matvecDot,
        Woof: Woof,
        SR: SR
    };
})();
