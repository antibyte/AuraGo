// Pixel module "ora": layered project files in the OpenRaster (.ora) format.
//
// An .ora file is a ZIP archive: a stored "mimetype" entry first, "stack.xml" (the layer stack, top layer first), one PNG
// per layer and per mask under data/, mergedimage.png and Thumbnails/thumbnail.png. This file holds
//   - Pixel.ora: DOM-free helpers (CRC-32, ZIP writer/reader, stack.xml writer/parser, SHA-256) plus write()/read(), which
//     reach the canvas only through a small injectable "codec" so node tests can run them without a browser, and
//   - the runtime glue: oraDataURL() for Save, loadOraProject() for Open and the one-time "layers are flattened" hint.
// The server (internal/server/pixel_ora.go) validates every uploaded archive again; keep the limits below in step.
(function () {
    'use strict';

    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const MIMETYPE = 'image/openraster';
    const NAMESPACE = 'urn:aurago:openraster';
    const LIMITS = {
        entries: 128,
        xmlBytes: 1024 * 1024,
        // Uncompressed bytes of everything read from one archive (zip bomb guard).
        totalBytes: 512 * 1024 * 1024,
        // The server never stores more than 32 MiB per file.
        archiveBytes: 32 * 1024 * 1024,
        pixels: 32 * 1024 * 1024,
        side: 16384,
        // Pixel memory of all decoded layers and masks of one project.
        layerBytes: 1024 * 1024 * 1024,
        thumbnail: 256
    };

    function oraError(code, message) {
        const err = new Error(message || code);
        err.oraCode = code;
        return err;
    }

    // --- CRC-32 ---

    let crcTable = null;

    function crc32(bytes) {
        if (!crcTable) {
            crcTable = new Uint32Array(256);
            for (let n = 0; n < 256; n++) {
                let c = n;
                for (let k = 0; k < 8; k++) c = c & 1 ? 0xEDB88320 ^ (c >>> 1) : c >>> 1;
                crcTable[n] = c >>> 0;
            }
        }
        let crc = 0xFFFFFFFF;
        for (let i = 0; i < bytes.length; i++) crc = crcTable[(crc ^ bytes[i]) & 0xFF] ^ (crc >>> 8);
        return (crc ^ 0xFFFFFFFF) >>> 0;
    }

    // --- ZIP writer (STORE only: PNG data does not compress further) ---

    function dosStamp(date) {
        const d = date instanceof Date && !isNaN(date.getTime()) ? date : new Date();
        const year = Math.max(1980, Math.min(2107, d.getFullYear()));
        return {
            time: ((d.getHours() << 11) | (d.getMinutes() << 5) | (d.getSeconds() >> 1)) & 0xFFFF,
            date: (((year - 1980) << 9) | ((d.getMonth() + 1) << 5) | d.getDate()) & 0xFFFF
        };
    }

    // entries: [{ name, data: Uint8Array }] in archive order. Names are written as UTF-8 (flag 0x0800 only when needed,
    // so the all-ASCII "mimetype" header is byte-for-byte what the OpenRaster spec asks for: no extra field, no flags).
    function zipWrite(entries, options) {
        const encoder = new TextEncoder();
        const stamp = dosStamp(options && options.date);
        const locals = [];
        const central = [];
        let offset = 0;
        entries.forEach(entry => {
            const name = encoder.encode(entry.name);
            const data = entry.data;
            const crc = crc32(data);
            const flags = name.some(b => b > 0x7F) ? 0x0800 : 0;
            const header = new Uint8Array(30 + name.length);
            const hv = new DataView(header.buffer);
            hv.setUint32(0, 0x04034b50, true);
            hv.setUint16(4, 10, true);
            hv.setUint16(6, flags, true);
            hv.setUint16(8, 0, true);
            hv.setUint16(10, stamp.time, true);
            hv.setUint16(12, stamp.date, true);
            hv.setUint32(14, crc, true);
            hv.setUint32(18, data.length, true);
            hv.setUint32(22, data.length, true);
            hv.setUint16(26, name.length, true);
            hv.setUint16(28, 0, true);
            header.set(name, 30);
            locals.push(header, data);
            central.push({ name, flags, crc, size: data.length, offset });
            offset += header.length + data.length;
        });
        const directoryStart = offset;
        const directory = central.map(item => {
            const record = new Uint8Array(46 + item.name.length);
            const rv = new DataView(record.buffer);
            rv.setUint32(0, 0x02014b50, true);
            rv.setUint16(4, 20, true);
            rv.setUint16(6, 10, true);
            rv.setUint16(8, item.flags, true);
            rv.setUint16(10, 0, true);
            rv.setUint16(12, stamp.time, true);
            rv.setUint16(14, stamp.date, true);
            rv.setUint32(16, item.crc, true);
            rv.setUint32(20, item.size, true);
            rv.setUint32(24, item.size, true);
            rv.setUint16(28, item.name.length, true);
            rv.setUint32(42, item.offset, true);
            record.set(item.name, 46);
            offset += record.length;
            return record;
        });
        const end = new Uint8Array(22);
        const ev = new DataView(end.buffer);
        ev.setUint32(0, 0x06054b50, true);
        ev.setUint16(8, entries.length, true);
        ev.setUint16(10, entries.length, true);
        ev.setUint32(12, offset - directoryStart, true);
        ev.setUint32(16, directoryStart, true);
        const total = offset + end.length;
        if (total > 0xFFFFFFFF || entries.length > 0xFFFF) throw oraError('too_large', 'archive too large');
        const out = new Uint8Array(total);
        let pos = 0;
        locals.concat(directory, [end]).forEach(part => { out.set(part, pos); pos += part.length; });
        return out;
    }

    // --- ZIP reader (STORE and DEFLATE, no ZIP64, no encryption) ---

    async function inflateRaw(raw, expected) {
        if (typeof DecompressionStream !== 'function') throw oraError('unsupported', 'deflate is not available');
        const out = new Uint8Array(expected);
        let filled = 0;
        try {
            const reader = new Blob([raw]).stream().pipeThrough(new DecompressionStream('deflate-raw')).getReader();
            for (;;) {
                const { done, value } = await reader.read();
                if (done) break;
                if (filled + value.length > expected) {
                    reader.cancel().catch(() => {});
                    throw oraError('invalid', 'entry is larger than declared');
                }
                out.set(value, filled);
                filled += value.length;
            }
        } catch (err) {
            throw err && err.oraCode ? err : oraError('invalid', 'entry is not valid deflate data');
        }
        if (filled !== expected) throw oraError('invalid', 'entry is smaller than declared');
        return out;
    }

    // Reads the central directory. Entry data is read lazily with read(name, maxBytes); the declared size is a claim, so
    // a stored entry must fill exactly its slot and an inflated one may never exceed it.
    function zipRead(input, options) {
        const bytes = input instanceof Uint8Array ? input : new Uint8Array(input);
        const total = bytes.length;
        const maxEntries = (options && options.maxEntries) || LIMITS.entries;
        const budget = { remaining: (options && options.maxTotalBytes) || LIMITS.totalBytes };
        if (total < 22) throw oraError('invalid', 'not a zip archive');
        const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
        let end = -1;
        for (let i = total - 22, stop = Math.max(0, total - 22 - 65535); i >= stop; i--) {
            if (view.getUint32(i, true) === 0x06054b50) { end = i; break; }
        }
        if (end < 0) throw oraError('invalid', 'not a zip archive');
        const count = view.getUint16(end + 10, true);
        const directorySize = view.getUint32(end + 12, true);
        const directoryOffset = view.getUint32(end + 16, true);
        if (count === 0xFFFF || directorySize === 0xFFFFFFFF || directoryOffset === 0xFFFFFFFF) throw oraError('unsupported', 'zip64 archives are not supported');
        if (count > maxEntries) throw oraError('too_large', 'too many entries');
        if (directoryOffset + directorySize > end) throw oraError('invalid', 'broken zip directory');
        const decoder = new TextDecoder('utf-8');
        const entries = new Map();
        let pos = directoryOffset;
        for (let i = 0; i < count; i++) {
            if (pos + 46 > end || view.getUint32(pos, true) !== 0x02014b50) throw oraError('invalid', 'broken zip directory');
            const nameLength = view.getUint16(pos + 28, true);
            const extraLength = view.getUint16(pos + 30, true);
            const commentLength = view.getUint16(pos + 32, true);
            if (pos + 46 + nameLength + extraLength + commentLength > end) throw oraError('invalid', 'broken zip directory');
            const name = decoder.decode(bytes.subarray(pos + 46, pos + 46 + nameLength));
            if (entries.has(name)) throw oraError('invalid', 'duplicate entry');
            entries.set(name, {
                name,
                flags: view.getUint16(pos + 8, true),
                method: view.getUint16(pos + 10, true),
                crc: view.getUint32(pos + 16, true),
                compressedSize: view.getUint32(pos + 20, true),
                size: view.getUint32(pos + 24, true),
                offset: view.getUint32(pos + 42, true)
            });
            pos += 46 + nameLength + extraLength + commentLength;
        }
        return {
            names: Array.from(entries.keys()),
            has: name => entries.has(name),
            async read(name, maxBytes) {
                const entry = entries.get(name);
                if (!entry) throw oraError('invalid', 'missing entry ' + name);
                if (entry.flags & 1) throw oraError('unsupported', 'encrypted entry');
                if (entry.size > (maxBytes == null ? LIMITS.totalBytes : maxBytes)) throw oraError('too_large', 'entry ' + name + ' is too large');
                budget.remaining -= entry.size;
                if (budget.remaining < 0) throw oraError('too_large', 'archive is too large when unpacked');
                const at = entry.offset;
                if (at + 30 > directoryOffset || view.getUint32(at, true) !== 0x04034b50) throw oraError('invalid', 'broken local header');
                const start = at + 30 + view.getUint16(at + 26, true) + view.getUint16(at + 28, true);
                const stop = start + entry.compressedSize;
                if (stop > directoryOffset) throw oraError('invalid', 'entry runs past the archive');
                const raw = bytes.subarray(start, stop);
                let data;
                if (entry.method === 0) {
                    if (entry.compressedSize !== entry.size) throw oraError('invalid', 'stored entry size mismatch');
                    data = raw;
                } else if (entry.method === 8) {
                    data = await inflateRaw(raw, entry.size);
                } else {
                    throw oraError('unsupported', 'compression method ' + entry.method);
                }
                if (crc32(data) !== entry.crc) throw oraError('invalid', 'checksum mismatch in ' + name);
                return data;
            }
        };
    }

    // --- stack.xml ---

    function escapeAttr(value) {
        const text = String(value == null ? '' : value);
        let out = '';
        for (let i = 0; i < text.length; i++) {
            const ch = text[i];
            const code = text.charCodeAt(i);
            if (ch === '&') out += '&amp;';
            else if (ch === '<') out += '&lt;';
            else if (ch === '>') out += '&gt;';
            else if (ch === '"') out += '&quot;';
            else if (ch === '\'') out += '&apos;';
            else if (code === 9) out += '&#9;';
            else if (code === 10) out += '&#10;';
            else if (code === 13) out += '&#13;';
            else if (code < 32 || code === 0xFFFE || code === 0xFFFF) out += ' ';
            else if (code >= 0xD800 && code <= 0xDBFF) {
                const next = text.charCodeAt(i + 1);
                if (next >= 0xDC00 && next <= 0xDFFF) { out += ch + text[i + 1]; i++; } else out += '?';
            } else if (code >= 0xDC00 && code <= 0xDFFF) out += '?';
            else out += ch;
        }
        return out;
    }

    function formatNumber(value) {
        const n = Number.isFinite(value) ? Math.max(0, Math.min(1, value)) : 1;
        return n.toFixed(6).replace(/\.?0+$/, '') || '0';
    }

    // ORA composite-op names are "svg:" plus the CSS blend name; normal is svg:src-over.
    function blendToOp(mode) {
        return mode && mode !== 'normal' ? 'svg:' + mode : 'svg:src-over';
    }

    function opToBlend(op) {
        const raw = String(op || '').trim().toLowerCase();
        if (!raw.startsWith('svg:')) return 'normal';
        const name = raw.slice(4);
        if (name === 'plus') return 'screen';
        const modes = Pixel.BLEND_MODES || [];
        return name !== 'src-over' && modes.includes(name) ? name : 'normal';
    }

    // layers: records top layer first { name, src, opacity, visible, blend, lockPixels, lockAlpha, lockPosition, clip, mask,
    // maskEnabled }.
    function buildStackXml(width, height, layers) {
        const lines = [
            '<?xml version="1.0" encoding="UTF-8"?>',
            '<image w="' + Math.round(width) + '" h="' + Math.round(height) + '" version="0.0.5" xmlns:aurago="' + NAMESPACE + '">',
            ' <stack>'
        ];
        layers.forEach(layer => {
            let tag = '  <layer name="' + escapeAttr(layer.name) + '" src="' + escapeAttr(layer.src) + '" x="' + (layer.x | 0) + '" y="' + (layer.y | 0) +
                '" opacity="' + formatNumber(layer.opacity) + '" visibility="' + (layer.visible === false ? 'hidden' : 'visible') +
                '" composite-op="' + escapeAttr(blendToOp(layer.blend)) + '"';
            if (layer.lockPixels) tag += ' aurago:locked-pixels="true"';
            if (layer.lockAlpha) tag += ' aurago:locked-alpha="true"';
            if (layer.lockPosition) tag += ' aurago:locked-position="true"';
            if (layer.clip) tag += ' aurago:clip="true"';
            if (layer.mask) tag += ' aurago:mask="' + escapeAttr(layer.mask) + '" aurago:mask-enabled="' + (layer.maskEnabled === false ? 'false' : 'true') + '"';
            lines.push(tag + '/>');
        });
        lines.push(' </stack>', '</image>', '');
        return lines.join('\n');
    }

    function decodeEntities(value) {
        if (/&(?![#A-Za-z0-9]+;)/.test(value)) throw oraError('invalid', 'bad entity');
        return value.replace(/&(#x[0-9A-Fa-f]+|#[0-9]+|[A-Za-z]+);/g, (match, body) => {
            switch (body) {
                case 'amp': return '&';
                case 'lt': return '<';
                case 'gt': return '>';
                case 'quot': return '"';
                case 'apos': return '\'';
                default: {
                    const code = body[0] === '#' ? (body[1] === 'x' ? parseInt(body.slice(2), 16) : parseInt(body.slice(1), 10)) : NaN;
                    const allowed = code === 9 || code === 10 || code === 13 || (code >= 0x20 && code <= 0xD7FF) || (code >= 0xE000 && code <= 0xFFFD) || (code >= 0x10000 && code <= 0x10FFFF);
                    if (!allowed) throw oraError('invalid', 'bad entity');
                    return String.fromCodePoint(code);
                }
            }
        });
    }

    // A deliberately small XML reader: elements, attributes, comments, processing instructions and CDATA. Text is ignored,
    // DOCTYPE/entity declarations are refused, and the depth and element counts are bounded. No DOM needed.
    function parseXml(source) {
        const text = String(source);
        const length = text.length;
        let pos = text.charCodeAt(0) === 0xFEFF ? 1 : 0;
        const open = [];
        let root = null;
        let elements = 0;
        const nameRe = /[A-Za-z_:][-A-Za-z0-9_:.]*/y;
        const attrRe = /\s+([A-Za-z_:][-A-Za-z0-9_:.]*)\s*=\s*(?:"([^"<]*)"|'([^'<]*)')/y;
        const closeRe = /\s*(\/?)>/y;
        const endRe = /<\/([A-Za-z_:][-A-Za-z0-9_:.]*)\s*>/y;
        const bad = reason => oraError('invalid', 'stack.xml: ' + reason);
        while (pos < length) {
            const lt = text.indexOf('<', pos);
            const chunk = lt < 0 ? text.slice(pos) : text.slice(pos, lt);
            if (!open.length && chunk.trim()) throw bad('text outside the root element');
            if (lt < 0) break;
            pos = lt;
            if (text.startsWith('<!--', pos)) {
                const stop = text.indexOf('-->', pos + 4);
                if (stop < 0) throw bad('unterminated comment');
                pos = stop + 3;
            } else if (text.startsWith('<![CDATA[', pos)) {
                const stop = text.indexOf(']]>', pos + 9);
                if (!open.length || stop < 0) throw bad('bad CDATA section');
                pos = stop + 3;
            } else if (text.startsWith('<?', pos)) {
                const stop = text.indexOf('?>', pos + 2);
                if (stop < 0) throw bad('unterminated processing instruction');
                pos = stop + 2;
            } else if (text.startsWith('<!', pos)) {
                throw bad('declarations are not allowed');
            } else if (text.startsWith('</', pos)) {
                endRe.lastIndex = pos;
                const match = endRe.exec(text);
                if (!match || !open.length || open[open.length - 1].name !== match[1]) throw bad('mismatched end tag');
                open.pop();
                pos = endRe.lastIndex;
            } else {
                nameRe.lastIndex = pos + 1;
                const nameMatch = nameRe.exec(text);
                if (!nameMatch || nameMatch.index !== pos + 1) throw bad('bad tag');
                const element = { name: nameMatch[0], attrs: Object.create(null), children: [] };
                pos = nameRe.lastIndex;
                for (;;) {
                    attrRe.lastIndex = pos;
                    const attr = attrRe.exec(text);
                    if (!attr || attr.index !== pos) break;
                    if (attr[1] in element.attrs) throw bad('duplicate attribute');
                    element.attrs[attr[1]] = decodeEntities(attr[2] !== undefined ? attr[2] : attr[3]);
                    pos = attrRe.lastIndex;
                }
                closeRe.lastIndex = pos;
                const close = closeRe.exec(text);
                if (!close || close.index !== pos) throw bad('bad tag');
                pos = closeRe.lastIndex;
                elements++;
                if (elements > 20000 || open.length >= 64) throw bad('too complex');
                if (open.length) open[open.length - 1].children.push(element);
                else if (root) throw bad('more than one root element');
                else root = element;
                if (!close[1]) open.push(element);
            }
        }
        if (open.length || !root) throw bad('not well-formed');
        return root;
    }

    function localName(name) {
        const colon = String(name).lastIndexOf(':');
        return colon < 0 ? name : name.slice(colon + 1);
    }

    function numberAttr(value, fallback) {
        if (value == null || String(value).trim() === '') return fallback;
        const n = Number(value);
        return Number.isFinite(n) ? n : fallback;
    }

    function cleanName(value) {
        return String(value == null ? '' : value).replace(/[\x00-\x1f\x7f]/g, ' ').trim().slice(0, 120);
    }

    // Returns { width, height, layers } with layers bottom first (the order of runtime.layers). Nested stacks are
    // flattened in document order; a hidden stack hides its layers and stack opacity and offsets are carried down.
    function parseStackXml(text) {
        const root = parseXml(text);
        if (localName(root.name) !== 'image') throw oraError('invalid', 'stack.xml has no <image> element');
        const width = Math.round(numberAttr(root.attrs.w, 0));
        const height = Math.round(numberAttr(root.attrs.h, 0));
        if (!(width >= 1 && height >= 1)) throw oraError('invalid', 'stack.xml has no image size');
        if (width > LIMITS.side || height > LIMITS.side || width * height > LIMITS.pixels) throw oraError('too_large', 'image is too large');
        const found = [];
        const flag = value => value === 'true' || value === '1';
        const walk = (stack, inherited, depth) => {
            const here = {
                visible: inherited.visible && stack.attrs.visibility !== 'hidden',
                opacity: inherited.opacity * Math.max(0, Math.min(1, numberAttr(stack.attrs.opacity, 1))),
                x: inherited.x + Math.round(numberAttr(stack.attrs.x, 0)),
                y: inherited.y + Math.round(numberAttr(stack.attrs.y, 0))
            };
            stack.children.forEach(child => {
                const kind = localName(child.name);
                if (kind === 'stack') {
                    if (depth < 16) walk(child, here, depth + 1);
                } else if (kind === 'layer') {
                    const a = child.attrs;
                    if (!a.src) return;
                    found.push({
                        name: cleanName(a.name),
                        src: String(a.src).replace(/^\.\//, ''),
                        x: here.x + Math.round(numberAttr(a.x, 0)),
                        y: here.y + Math.round(numberAttr(a.y, 0)),
                        opacity: here.opacity * Math.max(0, Math.min(1, numberAttr(a.opacity, 1))),
                        visible: here.visible && a.visibility !== 'hidden',
                        blend: opToBlend(a['composite-op']),
                        lockPixels: flag(a['aurago:locked-pixels']),
                        lockAlpha: flag(a['aurago:locked-alpha']),
                        lockPosition: flag(a['aurago:locked-position']),
                        clip: flag(a['aurago:clip']),
                        mask: a['aurago:mask'] ? String(a['aurago:mask']).replace(/^\.\//, '') : '',
                        maskEnabled: a['aurago:mask-enabled'] !== 'false'
                    });
                }
            });
        };
        const rootStack = root.children.find(child => localName(child.name) === 'stack');
        if (rootStack) walk(rootStack, { visible: true, opacity: 1, x: 0, y: 0 }, 0);
        return { width, height, layers: found.reverse() };
    }

    // --- PNG helpers ---

    function pngSize(bytes) {
        const signature = [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A];
        if (bytes.length < 24 || signature.some((b, i) => bytes[i] !== b)) return null;
        if (bytes[12] !== 0x49 || bytes[13] !== 0x48 || bytes[14] !== 0x44 || bytes[15] !== 0x52) return null;
        const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
        return { width: view.getUint32(16), height: view.getUint32(20) };
    }

    // The canvas side of write()/read(). The browser codec below uses 2D canvases; node tests pass their own.
    //   encode(canvas) -> Promise<Uint8Array> (PNG)        decode(bytes) -> Promise<canvas>
    //   getPixels(canvas) -> { width, height, data }       fromPixels({ width, height, data }) -> canvas   (RGBA, straight alpha)
    //   place(canvas, x, y, width, height) -> canvas       scale(canvas, maxEdge) -> canvas
    function browserCodec(makeCanvasOption) {
        const makeCanvas = makeCanvasOption || function makeCanvas(width, height) {
            const canvas = document.createElement('canvas');
            canvas.width = width;
            canvas.height = height;
            canvas.getContext('2d', { willReadFrequently: true });
            return canvas;
        };
        const context = canvas => canvas.getContext('2d', { willReadFrequently: true });
        return {
            encode(canvas) {
                return new Promise((resolve, reject) => {
                    if (!canvas || typeof canvas.toBlob !== 'function') { reject(oraError('invalid', 'cannot encode layer')); return; }
                    canvas.toBlob(blob => {
                        if (!blob) { reject(oraError('invalid', 'png encoding failed')); return; }
                        blob.arrayBuffer().then(buffer => resolve(new Uint8Array(buffer)), reject);
                    }, 'image/png');
                });
            },
            async decode(bytes) {
                const blob = new Blob([bytes], { type: 'image/png' });
                let source;
                try {
                    if (typeof createImageBitmap === 'function') {
                        source = await createImageBitmap(blob);
                    } else {
                        const url = URL.createObjectURL(blob);
                        try {
                            source = await new Promise((resolve, reject) => {
                                const img = new Image();
                                img.onload = () => resolve(img);
                                img.onerror = () => reject(new Error('png'));
                                img.src = url;
                            });
                        } finally { URL.revokeObjectURL(url); }
                    }
                } catch (_) {
                    throw oraError('invalid', 'a layer image cannot be decoded');
                }
                const width = source.width || source.naturalWidth, height = source.height || source.naturalHeight;
                const canvas = makeCanvas(width, height);
                context(canvas).drawImage(source, 0, 0);
                if (typeof source.close === 'function') source.close();
                return canvas;
            },
            getPixels(canvas) {
                return context(canvas).getImageData(0, 0, canvas.width, canvas.height);
            },
            fromPixels(pixels) {
                const canvas = makeCanvas(pixels.width, pixels.height);
                context(canvas).putImageData(new ImageData(pixels.data, pixels.width, pixels.height), 0, 0);
                return canvas;
            },
            place(canvas, x, y, width, height) {
                if (!x && !y && canvas.width === width && canvas.height === height) return canvas;
                const out = makeCanvas(width, height);
                context(out).drawImage(canvas, x, y);
                return out;
            },
            scale(canvas, maxEdge) {
                const longest = Math.max(canvas.width, canvas.height);
                if (longest <= maxEdge) return canvas;
                const factor = maxEdge / longest;
                const width = Math.max(1, Math.round(canvas.width * factor)), height = Math.max(1, Math.round(canvas.height * factor));
                const out = makeCanvas(width, height);
                const ctx = context(out);
                ctx.imageSmoothingEnabled = true;
                ctx.imageSmoothingQuality = 'high';
                ctx.drawImage(canvas, 0, 0, width, height);
                return out;
            }
        };
    }

    // A mask is stored as an opaque grey image (white = visible), which other editors can show; the canvas mask keeps its
    // visibility in the alpha channel.
    function maskToGrey(codec, maskCanvas) {
        const source = codec.getPixels(maskCanvas);
        const data = new Uint8ClampedArray(source.data.length);
        for (let i = 0; i < data.length; i += 4) {
            const v = source.data[i + 3];
            data[i] = v; data[i + 1] = v; data[i + 2] = v; data[i + 3] = 255;
        }
        return codec.fromPixels({ width: source.width, height: source.height, data });
    }

    function greyToMask(codec, greyCanvas) {
        const source = codec.getPixels(greyCanvas);
        const data = new Uint8ClampedArray(source.data.length);
        for (let i = 0; i < data.length; i += 4) {
            data[i] = 255; data[i + 1] = 255; data[i + 2] = 255; data[i + 3] = source.data[i];
        }
        return codec.fromPixels({ width: source.width, height: source.height, data });
    }

    // --- write / read ---

    // layers: runtime layers, bottom first ({ name, canvas, visible, opacity, blend, lockPixels, lockAlpha, lockPosition,
    // clip, maskCanvas, maskEnabled }); every canvas is width x height. options.merged is the flattened composition for
    // mergedimage.png and the thumbnail. Resolves with the archive bytes.
    async function writeBytes(layers, width, height, options) {
        const opts = options || {};
        const codec = opts.codec || browserCodec(opts.makeCanvas);
        if (!layers || !layers.length) throw oraError('invalid', 'a project needs at least one layer');
        const masks = layers.filter(layer => layer.maskCanvas).length;
        if (2 + layers.length + masks + (opts.merged ? 2 : 0) > LIMITS.entries) throw oraError('too_many_entries', 'too many layers and masks');
        const records = [];
        const jobs = [];
        layers.forEach((layer, index) => {
            const record = {
                name: layer.name, src: 'data/layer-' + index + '.png', x: 0, y: 0, opacity: layer.opacity, visible: layer.visible,
                blend: layer.blend, lockPixels: layer.lockPixels, lockAlpha: layer.lockAlpha, lockPosition: layer.lockPosition,
                clip: layer.clip, mask: '', maskEnabled: layer.maskEnabled
            };
            jobs.push({ name: record.src, canvas: layer.canvas });
            if (layer.maskCanvas) {
                record.mask = 'data/layer-' + index + '-mask.png';
                jobs.push({ name: record.mask, canvas: maskToGrey(codec, layer.maskCanvas) });
            }
            records.push(record);
        });
        if (opts.merged) {
            jobs.push({ name: 'mergedimage.png', canvas: opts.merged });
            jobs.push({ name: 'Thumbnails/thumbnail.png', canvas: codec.scale(opts.merged, LIMITS.thumbnail) });
        }
        // Every encode starts now, so the layers are captured as they are at this moment even while the user keeps painting.
        const images = await Promise.all(jobs.map(job => codec.encode(job.canvas)));
        const encoder = new TextEncoder();
        const entries = [
            { name: 'mimetype', data: encoder.encode(MIMETYPE) },
            { name: 'stack.xml', data: encoder.encode(buildStackXml(width, height, records.slice().reverse())) }
        ];
        let size = entries[0].data.length + entries[1].data.length;
        images.forEach((data, i) => {
            size += data.length;
            if (size > LIMITS.archiveBytes) throw oraError('too_large', 'project is too large');
            entries.push({ name: jobs[i].name, data });
        });
        return zipWrite(entries, { date: opts.date });
    }

    async function write(layers, width, height, options) {
        return new Blob([await writeBytes(layers, width, height, options)], { type: MIMETYPE });
    }

    // Resolves with { width, height, layers } where layers (bottom first) are { name, canvas, maskCanvas, maskEnabled, visible,
    // opacity, blend, lockPixels, lockAlpha, lockPosition, clip } and every canvas is document sized. Layer offsets are
    // applied; anything outside the document is cut off.
    async function read(input, options) {
        const opts = options || {};
        const codec = opts.codec || browserCodec(opts.makeCanvas);
        const maxLayers = opts.maxLayers || Pixel.MAX_LAYERS || 64;
        const zip = zipRead(input);
        if (!zip.has('mimetype') || !zip.has('stack.xml')) throw oraError('invalid', 'not an OpenRaster file');
        const mimetype = new TextDecoder().decode(await zip.read('mimetype', 256)).trim();
        if (mimetype !== MIMETYPE) throw oraError('invalid', 'not an OpenRaster file');
        const model = parseStackXml(new TextDecoder('utf-8').decode(await zip.read('stack.xml', LIMITS.xmlBytes)));
        if (model.layers.length > maxLayers) throw oraError('too_many_layers', 'too many layers');
        const { width, height } = model;
        const usable = model.layers.filter(spec => zip.has(spec.src));
        const maskCount = usable.filter(spec => spec.mask && zip.has(spec.mask)).length;
        if (width * height * 4 * (Math.max(1, usable.length) + maskCount) > LIMITS.layerBytes) throw oraError('too_large', 'project is too large to open');

        const decodeImage = async (name, x, y) => {
            const bytes = await zip.read(name);
            const size = pngSize(bytes);
            if (!size || size.width < 1 || size.height < 1 || size.width * size.height > LIMITS.pixels) return null;
            return codec.place(await codec.decode(bytes), x, y, width, height);
        };
        const layers = [];
        for (const spec of usable) {
            const canvas = await decodeImage(spec.src, spec.x, spec.y);
            if (!canvas) continue;
            let maskCanvas = null;
            if (spec.mask && zip.has(spec.mask)) {
                const grey = await decodeImage(spec.mask, 0, 0);
                if (grey) maskCanvas = greyToMask(codec, grey);
            }
            layers.push({
                name: spec.name, canvas, maskCanvas, maskEnabled: spec.maskEnabled, visible: spec.visible, opacity: spec.opacity, blend: spec.blend,
                lockPixels: spec.lockPixels, lockAlpha: spec.lockAlpha, lockPosition: spec.lockPosition, clip: spec.clip
            });
        }
        if (!layers.length && zip.has('mergedimage.png')) {
            const canvas = await decodeImage('mergedimage.png', 0, 0);
            if (canvas) layers.push({ name: '', canvas, maskCanvas: null, maskEnabled: true, visible: true, opacity: 1, blend: 'normal', lockPixels: false, lockAlpha: false, lockPosition: false, clip: false });
        }
        if (!layers.length) throw oraError('invalid', 'the file has no readable layers');
        return { width, height, layers };
    }

    // --- SHA-256 (file versions are the hex digest of the bytes; crypto.subtle only exists on secure origins) ---

    const SHA_K = new Uint32Array([
        0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
        0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
        0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
        0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
        0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
        0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
        0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
        0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2
    ]);

    function sha256Js(input) {
        const bytes = input instanceof Uint8Array ? input : new Uint8Array(input);
        const state = new Uint32Array([0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19]);
        const w = new Uint32Array(64);
        const block = (view, at) => {
            for (let i = 0; i < 16; i++) w[i] = view.getUint32(at + i * 4);
            for (let i = 16; i < 64; i++) {
                const a = w[i - 15], b = w[i - 2];
                const s0 = ((a >>> 7) | (a << 25)) ^ ((a >>> 18) | (a << 14)) ^ (a >>> 3);
                const s1 = ((b >>> 17) | (b << 15)) ^ ((b >>> 19) | (b << 13)) ^ (b >>> 10);
                w[i] = w[i - 16] + s0 + w[i - 7] + s1;
            }
            let a = state[0], b = state[1], c = state[2], d = state[3], e = state[4], f = state[5], g = state[6], h = state[7];
            for (let i = 0; i < 64; i++) {
                const big1 = ((e >>> 6) | (e << 26)) ^ ((e >>> 11) | (e << 21)) ^ ((e >>> 25) | (e << 7));
                const choose = (e & f) ^ (~e & g);
                const t1 = (h + big1 + choose + SHA_K[i] + w[i]) | 0;
                const big0 = ((a >>> 2) | (a << 30)) ^ ((a >>> 13) | (a << 19)) ^ ((a >>> 22) | (a << 10));
                const majority = (a & b) ^ (a & c) ^ (b & c);
                const t2 = (big0 + majority) | 0;
                h = g; g = f; f = e; e = (d + t1) | 0; d = c; c = b; b = a; a = (t1 + t2) | 0;
            }
            state[0] += a; state[1] += b; state[2] += c; state[3] += d; state[4] += e; state[5] += f; state[6] += g; state[7] += h;
        };
        const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
        const full = Math.floor(bytes.length / 64);
        for (let i = 0; i < full; i++) block(view, i * 64);
        const rest = bytes.length - full * 64;
        const tail = new Uint8Array(rest < 56 ? 64 : 128);
        tail.set(bytes.subarray(full * 64));
        tail[rest] = 0x80;
        const tv = new DataView(tail.buffer);
        tv.setUint32(tail.length - 8, Math.floor(bytes.length / 0x20000000));
        tv.setUint32(tail.length - 4, (bytes.length * 8) % 0x100000000);
        for (let i = 0; i < tail.length; i += 64) block(tv, i);
        return Array.from(state, word => word.toString(16).padStart(8, '0')).join('');
    }

    async function sha256Hex(input) {
        const bytes = input instanceof Uint8Array ? input : new Uint8Array(input);
        const subtle = window.crypto && window.crypto.subtle;
        if (subtle && typeof subtle.digest === 'function') {
            try {
                const digest = new Uint8Array(await subtle.digest('SHA-256', bytes));
                return Array.from(digest, byte => byte.toString(16).padStart(2, '0')).join('');
            } catch (_) { /* fall through to the script version */ }
        }
        return sha256Js(bytes);
    }

    // data:application/zip;base64,... of a blob, the form /api/pixel/save expects.
    async function blobToDataURL(blob) {
        if (typeof FileReader === 'function') {
            const url = await new Promise((resolve, reject) => {
                const reader = new FileReader();
                reader.onload = () => resolve(String(reader.result));
                reader.onerror = () => reject(reader.error || new Error('read'));
                reader.readAsDataURL(blob);
            });
            return 'data:application/zip;base64,' + url.slice(url.indexOf(',') + 1);
        }
        const bytes = new Uint8Array(await blob.arrayBuffer());
        let binary = '';
        for (let i = 0; i < bytes.length; i += 0x8000) binary += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
        return 'data:application/zip;base64,' + btoa(binary);
    }

    Pixel.ora = {
        MIMETYPE,
        LIMITS,
        crc32,
        zipWrite,
        zipRead,
        buildStackXml,
        parseStackXml,
        blendToOp,
        opToBlend,
        browserCodec,
        writeBytes,
        write,
        read,
        sha256Js,
        sha256Hex,
        blobToDataURL
    };

    // --- runtime glue ---

    // t() returns the key itself for strings that are not translated yet; fall back to the English text then.
    function text(r, key, fallback) {
        const value = r.t(key, fallback);
        return typeof value === 'string' && value && value !== key ? value : fallback;
    }

    Pixel.registerModule('ora', function install(r) {
        Object.assign(r, {
            // Translated text with the English fallback while a key is missing (used for the file dialog filters).
            oraText: Pixel.bindRuntime(r, function oraText(key, fallback) {
                return text(this, key, fallback);
            }),
            // The whole project as an .ora blob. Hidden or pending adjust/filter previews are not part of it, like in PNG saves.
            oraProjectBlob: Pixel.bindRuntime(r, async function oraProjectBlob() {
                const width = this.imgWidth, height = this.imgHeight;
                if (!width || !height || !this.layers.length) throw oraError('invalid', 'no document');
                const merged = this.flattenDocument(Pixel.acquireTempCanvas(width, height));
                try {
                    return await Pixel.ora.write(this.layers, width, height, { merged, makeCanvas: this.makeCanvas, codec: this.oraCodec });
                } finally { Pixel.releaseTempCanvas(merged); }
            }),
            oraDataURL: Pixel.bindRuntime(r, async function oraDataURL() {
                return Pixel.ora.blobToDataURL(await this.oraProjectBlob());
            }),
            // Replaces the document with an already decoded project; mirrors loadImageToCanvas (pixel-canvas.js).
            applyOraProject: Pixel.bindRuntime(r, function applyOraProject(project) {
                const w = project.width, h = project.height;
                this.resetStickerDocument();
                this.beginDocumentReplace();
                this.canvas.width = w;
                this.canvas.height = h;
                this.overlayCanvas.width = w;
                this.overlayCanvas.height = h;
                this.imgWidth = w;
                this.imgHeight = h;
                this.layers = project.layers.map((spec, index) => this.createLayer({
                    name: spec.name || (index === 0 ? this.t('pixel.layer_background') : this.t('pixel.layer') + ' ' + (index + 1)),
                    canvas: spec.canvas,
                    maskCanvas: spec.maskCanvas,
                    maskEnabled: spec.maskEnabled,
                    visible: spec.visible,
                    opacity: spec.opacity,
                    blend: spec.blend,
                    lockPixels: spec.lockPixels,
                    lockAlpha: spec.lockAlpha,
                    lockPosition: spec.lockPosition,
                    clip: spec.clip
                }));
                this.activeLayerIdx = this.layers.length - 1;
                this.history = [];
                this.historyIdx = -1;
                this.selection = null;
                this.pushHistory('open');
                this.canvasWrap.hidden = false;
                this.emptyState.hidden = true;
                this.clearOverlay();
                this.captureAdjustmentSource();
                this.compositeLayers();
                this.zoomFit();
                this.updateStatus();
                this.refreshLayerPanel();
                this.syncActiveLayer();
                this.notifyLayersChanged();
                this.emit('document-changed', { width: w, height: h, reason: 'open' });
            }),
            // Opens an .ora project from the desktop workspace (called by loadDesktopImagePath for .ora paths).
            loadOraProject: Pixel.bindRuntime(r, async function loadOraProject(path, options) {
                const nextPath = String(path || '').trim();
                if (!nextPath) return;
                this.state.imageRead?.abort();
                const controller = new AbortController();
                this.state.imageRead = controller;
                const revision = this.state.documentRevision;
                const current = () => !controller.signal.aborted && !this.state.disposed && this.state.imageRead === controller && revision === this.state.documentRevision;
                try {
                    const response = await fetch('/api/desktop/download?path=' + encodeURIComponent(nextPath), { credentials: 'same-origin', cache: 'no-store', signal: controller.signal });
                    if (!response.ok) throw new Error('download');
                    const buffer = await response.arrayBuffer();
                    if (!current()) return;
                    // The download endpoint sends no ETag; saves compare against the SHA-256 of the stored bytes.
                    const version = '"' + await sha256Hex(buffer) + '"';
                    if (!current()) return;
                    const project = await Pixel.ora.read(buffer, { makeCanvas: this.makeCanvas, maxLayers: this.MAX_LAYERS, codec: this.oraCodec });
                    if (!current()) return;
                    this.applyOraProject(project);
                    this.filePath = nextPath;
                    this.fileName = nextPath.split('/').pop();
                    this.state.fileVersion = version;
                    this.state.versionPath = nextPath;
                    this.isDirty = false;
                    if (!options || options.saveRecent !== false) this.saveRecentFile(nextPath);
                    this.updateStatus();
                } catch (err) {
                    if ((err && err.name === 'AbortError') || this.state.disposed) return;
                    this.notify({ type: 'error', message: this.oraErrorMessage(err, 'open') });
                }
            }),
            // mode 'save' or 'open'; unknown failures keep the generic messages.
            oraErrorMessage: Pixel.bindRuntime(r, function oraErrorMessage(err, mode) {
                const code = err && err.oraCode;
                if (mode === 'save') {
                    if (code === 'too_many_entries') return text(this, 'pixel.ora_save_too_many_layers', 'Too many layers and masks to save as an .ora file.');
                    if (code === 'too_large' || (err && err.status === 413)) return text(this, 'pixel.ora_save_too_large', 'The project is too large for an .ora file.');
                    return text(this, 'pixel.error_save', 'Failed to save');
                }
                if (code === 'unsupported') return text(this, 'pixel.ora_open_unsupported', 'This OpenRaster file uses a feature that is not supported.');
                if (code === 'too_large') return text(this, 'pixel.ora_open_too_large', 'This OpenRaster file is too large to open.');
                if (code === 'too_many_layers') return text(this, 'pixel.ora_open_too_many_layers', 'This OpenRaster file has more layers than Pixel can open.');
                if (code === 'invalid') return text(this, 'pixel.ora_open_invalid', 'This is not a valid OpenRaster file.');
                return text(this, 'pixel.error_load', 'Failed to load image');
            }),
            // After a PNG/JPEG save: tell once per window that layers and masks are not part of the file.
            noteFlattenedLayers: Pixel.bindRuntime(r, function noteFlattenedLayers() {
                if (this.state.oraHintShown) return;
                if (!(this.layers.length > 1 || this.layers.some(layer => layer.maskCanvas))) return;
                this.state.oraHintShown = true;
                this.notify({ type: 'info', message: text(this, 'pixel.ora_flatten_hint', 'Layers are flattened — save as .ora to keep them') });
            })
        });
    });
})();
