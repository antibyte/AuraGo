// WP2 OpenRaster (pixel-ora.js): zip + stack.xml + write/read round trips against a fake canvas codec, the SHA-256 used for
// file versions, and the runtime glue (save data URL, open, flatten hint) with a hand-made runtime. No browser needed.
import assert from 'node:assert/strict';
import zlib from 'node:zlib';
import crypto from 'node:crypto';
import { loadAppScripts } from './pixel-test-util.mjs';

// pixel-state.js supplies BLEND_MODES / MAX_LAYERS / bindRuntime; the module itself registers through a stub.
const registered = [];
globalThis.window = globalThis;
globalThis.AuraPixelApp = { registerModule: (id, install) => registered.push({ id, install }) };
loadAppScripts(['pixel-state.js', 'pixel-ora.js']);
const Pixel = globalThis.AuraPixelApp;
const Ora = Pixel.ora;
assert.ok(Ora && typeof Ora.write === 'function' && typeof Ora.read === 'function', 'Pixel.ora.write/read exist');
assert.deepEqual(registered.map(entry => entry.id), ['ora'], 'the module registers itself once');

const results = [];
const pending = [];
function check(name, fn) {
    const fail = err => results.push('FAIL ' + name + ': ' + (err && err.stack || err));
    try {
        const out = fn();
        if (out && typeof out.then === 'function') pending.push(out.then(() => results.push('ok   ' + name), fail));
        else results.push('ok   ' + name);
    } catch (err) { fail(err); }
}
const rejects = async (promise, code) => {
    let error = null;
    try { await promise; } catch (err) { error = err; }
    assert.ok(error, 'expected a rejection');
    if (code) assert.equal(error.oraCode, code, 'error code, got ' + (error && error.message));
    return error;
};
const text = bytes => new TextDecoder().decode(bytes);
const bytesOf = value => new TextEncoder().encode(value);
const ab = view => view.buffer.slice(view.byteOffset, view.byteOffset + view.length);

// --- fake canvases and a real PNG codec on top of zlib ---------------------------------------------------------------

const canvas = (width, height, data) => ({ width, height, data: data || new Uint8ClampedArray(width * height * 4) });

function pngChunk(type, data) {
    const body = Buffer.concat([Buffer.from(type, 'latin1'), Buffer.from(data)]);
    const out = Buffer.alloc(12 + data.length);
    out.writeUInt32BE(data.length, 0);
    body.copy(out, 4);
    out.writeUInt32BE(zlib.crc32(body) >>> 0, 8 + data.length);
    return out;
}

function pngEncode(c) {
    const rows = Buffer.alloc((c.width * 4 + 1) * c.height);
    for (let y = 0; y < c.height; y++) Buffer.from(c.data.buffer, c.data.byteOffset + y * c.width * 4, c.width * 4).copy(rows, y * (c.width * 4 + 1) + 1);
    const header = Buffer.alloc(13);
    header.writeUInt32BE(c.width, 0);
    header.writeUInt32BE(c.height, 4);
    header[8] = 8;
    header[9] = 6;
    return new Uint8Array(Buffer.concat([Buffer.from([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A]), pngChunk('IHDR', header), pngChunk('IDAT', zlib.deflateSync(rows)), pngChunk('IEND', Buffer.alloc(0))]));
}

function pngDecode(bytes) {
    const buf = Buffer.from(bytes);
    assert.equal(buf.subarray(1, 4).toString('latin1'), 'PNG');
    let pos = 8, width = 0, height = 0;
    const idat = [];
    while (pos < buf.length) {
        const length = buf.readUInt32BE(pos), type = buf.subarray(pos + 4, pos + 8).toString('latin1');
        const body = buf.subarray(pos + 8, pos + 8 + length);
        if (type === 'IHDR') { width = body.readUInt32BE(0); height = body.readUInt32BE(4); }
        if (type === 'IDAT') idat.push(body);
        pos += 12 + length;
    }
    const rows = zlib.inflateSync(Buffer.concat(idat));
    const data = new Uint8ClampedArray(width * height * 4);
    for (let y = 0; y < height; y++) {
        assert.equal(rows[y * (width * 4 + 1)], 0, 'test decoder only reads filter 0');
        data.set(rows.subarray(y * (width * 4 + 1) + 1, (y + 1) * (width * 4 + 1)), y * width * 4);
    }
    return canvas(width, height, data);
}

const codec = {
    encode: async c => pngEncode(c),
    decode: async bytes => pngDecode(bytes),
    getPixels: c => ({ width: c.width, height: c.height, data: new Uint8ClampedArray(c.data) }),
    fromPixels: p => canvas(p.width, p.height, new Uint8ClampedArray(p.data)),
    place(c, x, y, width, height) {
        const out = canvas(width, height);
        for (let sy = 0; sy < c.height; sy++) {
            const ty = sy + y;
            if (ty < 0 || ty >= height) continue;
            for (let sx = 0; sx < c.width; sx++) {
                const tx = sx + x;
                if (tx < 0 || tx >= width) continue;
                out.data.set(c.data.subarray((sy * c.width + sx) * 4, (sy * c.width + sx) * 4 + 4), (ty * width + tx) * 4);
            }
        }
        return out;
    },
    scale(c, maxEdge) {
        const longest = Math.max(c.width, c.height);
        if (longest <= maxEdge) return c;
        const f = maxEdge / longest;
        const width = Math.max(1, Math.round(c.width * f)), height = Math.max(1, Math.round(c.height * f));
        const out = canvas(width, height);
        for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
            const sx = Math.min(c.width - 1, Math.floor(x / f)), sy = Math.min(c.height - 1, Math.floor(y / f));
            out.data.set(c.data.subarray((sy * c.width + sx) * 4, (sy * c.width + sx) * 4 + 4), (y * width + x) * 4);
        }
        return out;
    }
};

function pattern(width, height, seed, translucent) {
    const c = canvas(width, height);
    let s = seed;
    for (let i = 0; i < width * height; i++) {
        s = (s * 1103515245 + 12345) >>> 0;
        c.data[i * 4] = (s >>> 8) & 255;
        c.data[i * 4 + 1] = (s >>> 16) & 255;
        c.data[i * 4 + 2] = (s >>> 24) & 255;
        c.data[i * 4 + 3] = translucent ? (s >>> 4) & 255 : 255;
    }
    return c;
}

function layer(extra) {
    return Object.assign({
        name: 'Layer', canvas: null, visible: true, opacity: 1, blend: 'normal', lockPixels: false, lockAlpha: false, lockPosition: false,
        clip: false, maskCanvas: null, maskEnabled: true
    }, extra);
}

const W = 300, H = 120;
function sampleProject() {
    const mask = canvas(W, H);
    for (let y = 0; y < H; y++) for (let x = 0; x < W; x++) mask.data.set([255, 255, 255, Math.round(x * 255 / (W - 1))], (y * W + x) * 4);
    return [
        layer({ name: 'Background', canvas: pattern(W, H, 1, false) }),
        layer({ name: 'Shadow & <glow> "q" \'s\'', canvas: pattern(W, H, 2, true), opacity: 0.37, blend: 'multiply', visible: false, lockAlpha: true, clip: true }),
        layer({ name: 'Top ' + String.fromCodePoint(0x1F600) + String.fromCharCode(0xE9), canvas: pattern(W, H, 3, true), opacity: 0.5, blend: 'color-dodge', lockPixels: true, lockPosition: true, maskCanvas: mask, maskEnabled: false })
    ];
}

// --- helpers for hand-made archives ----------------------------------------------------------------------------------

function buildZip(entries) {
    const parts = [];
    const central = [];
    let offset = 0;
    entries.forEach(entry => {
        const name = Buffer.from(entry.name, 'utf8');
        const data = Buffer.from(entry.data);
        const body = entry.method === 8 ? zlib.deflateRawSync(data) : data;
        const crc = entry.crc != null ? entry.crc : zlib.crc32(data) >>> 0;
        const size = entry.size != null ? entry.size : data.length;
        const head = Buffer.alloc(30);
        head.writeUInt32LE(0x04034b50, 0); head.writeUInt16LE(20, 4); head.writeUInt16LE(0, 6); head.writeUInt16LE(entry.method || 0, 8);
        head.writeUInt32LE(crc, 14); head.writeUInt32LE(body.length, 18); head.writeUInt32LE(size, 22); head.writeUInt16LE(name.length, 26);
        parts.push(head, name, body);
        const rec = Buffer.alloc(46);
        rec.writeUInt32LE(0x02014b50, 0); rec.writeUInt16LE(20, 4); rec.writeUInt16LE(20, 6); rec.writeUInt16LE(entry.method || 0, 10);
        rec.writeUInt32LE(crc, 16); rec.writeUInt32LE(body.length, 20); rec.writeUInt32LE(size, 24); rec.writeUInt16LE(name.length, 28);
        rec.writeUInt32LE(offset, 42);
        central.push(rec, name);
        offset += head.length + name.length + body.length;
    });
    const directory = Buffer.concat(central);
    const end = Buffer.alloc(22);
    end.writeUInt32LE(0x06054b50, 0); end.writeUInt16LE(entries.length, 8); end.writeUInt16LE(entries.length, 10);
    end.writeUInt32LE(directory.length, 12); end.writeUInt32LE(offset, 16);
    return new Uint8Array(Buffer.concat([...parts, directory, end]));
}

const simpleStack = (layers, attrs) => '<?xml version="1.0"?><image w="4" h="3"' + (attrs || '') + '><stack>' + layers + '</stack></image>';
const MIME = { name: 'mimetype', data: bytesOf('image/openraster') };
const onePixelPng = rgba => pngEncode(canvas(1, 1, new Uint8ClampedArray(rgba)));

// --- tests -----------------------------------------------------------------------------------------------------------

check('CRC-32 of a known string and of other inputs', () => {
    assert.equal(Ora.crc32(bytesOf('123456789')), 0xCBF43926);
    assert.equal(Ora.crc32(new Uint8Array(0)), 0);
    const random = crypto.randomBytes(5000);
    assert.equal(Ora.crc32(new Uint8Array(random)), zlib.crc32(random) >>> 0);
});

check('SHA-256 in script matches node crypto for edge-case lengths and the async helper', async () => {
    for (const length of [0, 1, 3, 55, 56, 57, 63, 64, 65, 119, 120, 128, 1000, 100003]) {
        const data = crypto.randomBytes(length);
        const expected = crypto.createHash('sha256').update(data).digest('hex');
        assert.equal(Ora.sha256Js(new Uint8Array(data)), expected, 'length ' + length);
    }
    const data = crypto.randomBytes(4096);
    const expected = crypto.createHash('sha256').update(data).digest('hex');
    assert.equal(await Ora.sha256Hex(data.buffer.slice(data.byteOffset, data.byteOffset + data.length)), expected, 'ArrayBuffer input');
    assert.equal(Ora.sha256Js(bytesOf('abc')), 'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad');
});

check('zip writer: mimetype first, stored, no extra field, readable back, deterministic', async () => {
    const date = new Date(2026, 9, 22, 12, 0, 0);
    const entries = [{ name: 'mimetype', data: bytesOf('image/openraster') }, { name: 'a/b.txt', data: bytesOf('hello') }, { name: 'empty', data: new Uint8Array(0) }];
    const zip = Ora.zipWrite(entries, { date });
    assert.deepEqual(Array.from(Ora.zipWrite(entries, { date })), Array.from(zip));
    const view = new DataView(zip.buffer);
    assert.equal(view.getUint32(0, true), 0x04034b50);
    assert.equal(view.getUint16(6, true), 0, 'no flags on the mimetype header');
    assert.equal(view.getUint16(8, true), 0, 'stored');
    assert.equal(view.getUint16(26, true), 8);
    assert.equal(view.getUint16(28, true), 0, 'no extra field');
    assert.equal(text(zip.subarray(30, 38)), 'mimetype');
    assert.equal(text(zip.subarray(38, 54)), 'image/openraster', 'content right behind the header');
    const reader = Ora.zipRead(zip);
    assert.deepEqual(reader.names, ['mimetype', 'a/b.txt', 'empty']);
    assert.equal(text(await reader.read('a/b.txt')), 'hello');
    assert.equal((await reader.read('empty')).length, 0);
    await rejects(reader.read('missing'), 'invalid');
});

check('zip reader supports DEFLATE entries (zlib.deflateRawSync) next to stored ones', async () => {
    const xml = bytesOf('<image w="4" h="3">' + 'x'.repeat(5000) + '</image>');
    const zip = buildZip([{ name: 'mimetype', data: bytesOf('image/openraster') }, { name: 'stack.xml', data: xml, method: 8 }]);
    const reader = Ora.zipRead(zip);
    assert.deepEqual(Array.from(await reader.read('stack.xml')), Array.from(xml));
    assert.equal(text(await reader.read('mimetype')), 'image/openraster');
});

check('zip reader refuses lying sizes, bad checksums, bombs and budget overruns', async () => {
    const big = new Uint8Array(2 * 1024 * 1024);
    const bomb = buildZip([{ name: 'x', data: big, method: 8, size: 100 }]);
    await rejects(Ora.zipRead(bomb).read('x'), 'invalid');
    const shortClaim = buildZip([{ name: 'x', data: bytesOf('hello world'), method: 8, size: 20 }]);
    await rejects(Ora.zipRead(shortClaim).read('x'), 'invalid');
    const storedLie = buildZip([{ name: 'x', data: bytesOf('hello world'), size: 5 }]);
    await rejects(Ora.zipRead(storedLie).read('x'), 'invalid');
    const wrongCrc = buildZip([{ name: 'x', data: bytesOf('hello world'), crc: 1234 }]);
    await rejects(Ora.zipRead(wrongCrc).read('x'), 'invalid');
    const honest = buildZip([{ name: 'x', data: bytesOf('hello world') }]);
    await rejects(Ora.zipRead(honest).read('x', 5), 'too_large');
    await rejects(Ora.zipRead(honest, { maxTotalBytes: 8 }).read('x'), 'too_large');
    const reader = Ora.zipRead(buildZip([{ name: 'a', data: bytesOf('12345') }, { name: 'b', data: bytesOf('12345') }]), { maxTotalBytes: 8 });
    await reader.read('a');
    await rejects(reader.read('b'), 'too_large');
    assert.throws(() => Ora.zipRead(bytesOf('this is not a zip file at all, really not')), err => err.oraCode === 'invalid');
    assert.throws(() => Ora.zipRead(buildZip(Array.from({ length: 129 }, (_, i) => ({ name: 'e' + i, data: bytesOf('x') })))), err => err.oraCode === 'too_large');
    const flagged = Buffer.from(buildZip([{ name: 'x', data: bytesOf('abc') }]));
    flagged.writeUInt16LE(1, flagged.indexOf(Buffer.from([0x50, 0x4b, 0x01, 0x02])) + 8);
    await rejects(Ora.zipRead(new Uint8Array(flagged)).read('x'), 'unsupported');
});

check('blend modes map to svg composite ops and back; unknown ops fall back to normal', () => {
    assert.equal(Ora.blendToOp('normal'), 'svg:src-over');
    assert.equal(Ora.blendToOp('multiply'), 'svg:multiply');
    Pixel.BLEND_MODES.forEach(mode => assert.equal(Ora.opToBlend(Ora.blendToOp(mode)), mode, mode));
    assert.equal(Ora.opToBlend('svg:plus'), 'screen');
    assert.equal(Ora.opToBlend('svg:dst-in'), 'normal');
    assert.equal(Ora.opToBlend('krita:linear_dodge'), 'normal');
    assert.equal(Ora.opToBlend(''), 'normal');
    assert.equal(Ora.opToBlend(undefined), 'normal');
});

check('stack.xml: text is escaped, attributes survive a build/parse round trip, order is reversed to bottom first', () => {
    const tricky = 'A & B <x> "q" \'s\' ' + String.fromCodePoint(0x1F600) + String.fromCharCode(0xE9) + '\t\n' + String.fromCharCode(1);
    const xml = Ora.buildStackXml(40, 30, [
        { name: tricky, src: 'data/layer-1.png', opacity: 0.37, visible: false, blend: 'screen', lockPixels: true, lockAlpha: true, lockPosition: true, clip: true, mask: 'data/layer-1-mask.png', maskEnabled: false },
        { name: 'Bottom', src: 'data/layer-0.png', opacity: 1, visible: true, blend: 'normal' }
    ]);
    assert.ok(xml.startsWith('<?xml version="1.0" encoding="UTF-8"?>'));
    assert.ok(xml.includes('version="0.0.5"'));
    assert.ok(xml.includes('composite-op="svg:screen"') && xml.includes('composite-op="svg:src-over"'));
    assert.ok(xml.indexOf('layer-1.png') < xml.indexOf('layer-0.png'), 'top layer first');
    assert.ok(!/[\x00-\x08\x0b\x0c\x0e-\x1f]/.test(xml), 'no control characters in the XML');
    const model = Ora.parseStackXml(xml);
    assert.equal(model.width, 40);
    assert.equal(model.height, 30);
    assert.deepEqual(model.layers.map(l => l.src), ['data/layer-0.png', 'data/layer-1.png']);
    const top = model.layers[1];
    // Control characters (tab, newline, 0x01) become spaces and trailing ones are trimmed.
    assert.equal(top.name, 'A & B <x> "q" \'s\' ' + String.fromCodePoint(0x1F600) + String.fromCharCode(0xE9));
    assert.equal(top.opacity, 0.37);
    assert.equal(top.visible, false);
    assert.equal(top.blend, 'screen');
    assert.ok(top.lockPixels && top.lockAlpha && top.lockPosition && top.clip);
    assert.equal(top.mask, 'data/layer-1-mask.png');
    assert.equal(top.maskEnabled, false);
    assert.equal(model.layers[0].blend, 'normal');
    assert.equal(model.layers[0].maskEnabled, true);
    assert.equal(model.layers[0].mask, '');
});

check('stack.xml parser reads foreign files: nested stacks, offsets, quotes, unknown attributes, comments, entities', () => {
    const xml = [
        "<?xml version='1.0' encoding='UTF-8'?>",
        '<!-- written by another editor -->',
        '<image w="400" h="300" version="0.0.3" xres="72" yres="72" some-new="thing">',
        '  <stack opacity="1" name="root" visibility="visible">',
        "    <layer name='Top &#65;&#x42;' src='data/top.png' x='-5' y='7' opacity='0.25' composite-op='svg:overlay' visibility='visible' selected='true'/>",
        '    <stack opacity="0.5" visibility="hidden" x="10" y="20">',
        '      <layer name="in group" src="./data/grp.png" x="1" y="2" opacity="0.5"/>',
        '      <text>ignored element</text>',
        '    </stack>',
        '    <layer name="no source"/>',
        '    <layer name="Bottom" src="data/bottom.png"><extra/></layer>',
        '  </stack>',
        '</image>'
    ].join('\n');
    const model = Ora.parseStackXml(xml);
    assert.deepEqual(model.layers.map(l => l.name), ['Bottom', 'in group', 'Top AB']);
    const [bottom, group, top] = model.layers;
    assert.equal(bottom.opacity, 1);
    assert.equal(bottom.visible, true);
    assert.equal(group.visible, false, 'a hidden stack hides its layers');
    assert.equal(group.opacity, 0.25);
    assert.equal(group.src, 'data/grp.png');
    assert.equal(group.x, 11);
    assert.equal(group.y, 22);
    assert.equal(top.x, -5);
    assert.equal(top.y, 7);
    assert.equal(top.blend, 'overlay');
    assert.equal(top.opacity, 0.25);
});

check('stack.xml parser rejects DOCTYPE, broken, hostile and oversized XML', () => {
    const bad = [
        '<!DOCTYPE image [<!ENTITY a "b">]><image w="4" h="3"><stack/></image>',
        '<image w="4" h="3"><stack>',
        '<image w="4" h="3"><stack></image></stack>',
        '<image w="4" h="3"/><image w="4" h="3"/>',
        'text<image w="4" h="3"/>',
        '<image w="4" h="3" w="5"/>',
        '<image w="4" h="3" x=unquoted/>',
        '<image w="4" h="3" name="&nope;"/>',
        '<image w="4" h="3" name="a & b"/>',
        '<image w="4" h="3" name="&#0;"/>',
        '<image w="4" h="3"><stack><layer name="x< "/></stack></image>',
        '<picture w="4" h="3"/>',
        '<image h="3"/>',
        '<image w="0" h="3"/>',
        '<image w="x" h="3"/>',
        '',
        '<image w="4" h="3"><!-- never closed </image>',
        '<image w="4" h="3">' + '<stack>'.repeat(80) + '</stack>'.repeat(80) + '</image>'
    ];
    bad.forEach(xml => assert.throws(() => Ora.parseStackXml(xml), err => !!err.oraCode, 'accepted: ' + xml.slice(0, 60)));
    assert.throws(() => Ora.parseStackXml('<image w="20000" h="20000"><stack/></image>'), err => err.oraCode === 'too_large');
    assert.throws(() => Ora.parseStackXml('<image w="9000" h="9000"><stack/></image>'), err => err.oraCode === 'too_large', 'more than the 32 MP the server accepts');
    assert.equal(Ora.parseStackXml('<image w="4" h="3"><stack/></image>').layers.length, 0);
    assert.equal(Ora.parseStackXml(String.fromCharCode(0xFEFF) + '<image w="4" h="3"/>').width, 4, 'a byte order mark is skipped');
});

check('write -> read round trip of three layers keeps names, opacity, visibility, blend, locks, clip, mask and every pixel', async () => {
    const layers = sampleProject();
    const merged = pattern(W, H, 9, false);
    const bytes = await Ora.writeBytes(layers, W, H, { codec, merged, date: new Date(2026, 9, 22) });

    const zip = Ora.zipRead(bytes);
    assert.deepEqual(zip.names, ['mimetype', 'stack.xml', 'data/layer-0.png', 'data/layer-1.png', 'data/layer-2.png', 'data/layer-2-mask.png', 'mergedimage.png', 'Thumbnails/thumbnail.png']);
    assert.equal(text(await zip.read('mimetype')), 'image/openraster');
    const thumb = pngDecode(await zip.read('Thumbnails/thumbnail.png'));
    assert.equal(Math.max(thumb.width, thumb.height), 256);
    assert.equal(thumb.width, 256);
    assert.equal(thumb.height, Math.round(H * 256 / W));
    assert.deepEqual(Array.from(pngDecode(await zip.read('mergedimage.png')).data), Array.from(merged.data));
    const stackText = text(await zip.read('stack.xml'));
    assert.ok(stackText.indexOf('layer-2.png') < stackText.indexOf('layer-1.png') && stackText.indexOf('layer-1.png') < stackText.indexOf('layer-0.png'), 'top first');

    const project = await Ora.read(bytes, { codec });
    assert.equal(project.width, W);
    assert.equal(project.height, H);
    assert.equal(project.layers.length, 3);
    project.layers.forEach((got, i) => {
        const want = layers[i];
        assert.equal(got.name, want.name, 'name ' + i);
        assert.equal(got.visible, want.visible, 'visible ' + i);
        assert.equal(got.opacity, want.opacity, 'opacity ' + i);
        assert.equal(got.blend, want.blend, 'blend ' + i);
        assert.equal(got.lockPixels, want.lockPixels, 'lockPixels ' + i);
        assert.equal(got.lockAlpha, want.lockAlpha, 'lockAlpha ' + i);
        assert.equal(got.lockPosition, want.lockPosition, 'lockPosition ' + i);
        assert.equal(got.clip, want.clip, 'clip ' + i);
        assert.equal(got.maskEnabled, want.maskEnabled, 'maskEnabled ' + i);
        assert.equal(got.canvas.width, W);
        assert.equal(got.canvas.height, H);
        assert.deepEqual(Array.from(got.canvas.data), Array.from(want.canvas.data), 'pixels ' + i);
        assert.equal(!!got.maskCanvas, !!want.maskCanvas, 'mask presence ' + i);
    });
    const gotMask = project.layers[2].maskCanvas, wantMask = layers[2].maskCanvas;
    assert.deepEqual(Array.from(gotMask.data), Array.from(wantMask.data), 'mask pixels incl. alpha ramp');
    assert.equal(gotMask.data[3], 0);
    assert.equal(gotMask.data[(W - 1) * 4 + 3], 255);

    // Same project, same date: the archive is byte-identical, and the Blob wrapper carries the OpenRaster type.
    const again = await Ora.writeBytes(sampleProject(), W, H, { codec, merged, date: new Date(2026, 9, 22) });
    assert.deepEqual(Array.from(again), Array.from(bytes));
    const blob = await Ora.write(layers, W, H, { codec, merged });
    assert.equal(blob.type, 'image/openraster');
    assert.ok(blob.size > 0);
});

check('write without a merged image skips mergedimage.png and the thumbnail', async () => {
    const bytes = await Ora.writeBytes([layer({ name: 'Only', canvas: pattern(8, 6, 4, false) })], 8, 6, { codec });
    assert.deepEqual(Ora.zipRead(bytes).names, ['mimetype', 'stack.xml', 'data/layer-0.png']);
});

check('entry limit: 62 layers with masks fit into 128 entries, one more layer is refused up front', async () => {
    const make = count => Array.from({ length: count }, (_, i) => layer({ name: 'L' + i, canvas: canvas(1, 1, new Uint8ClampedArray([i, 0, 0, 255])), maskCanvas: canvas(1, 1, new Uint8ClampedArray([255, 255, 255, 128])) }));
    const merged = canvas(1, 1, new Uint8ClampedArray([0, 0, 0, 255]));
    const bytes = await Ora.writeBytes(make(62), 1, 1, { codec, merged });
    assert.equal(Ora.zipRead(bytes).names.length, 128);
    const project = await Ora.read(bytes, { codec });
    assert.equal(project.layers.length, 62);
    assert.equal(project.layers[61].canvas.data[0], 61);
    await rejects(Ora.writeBytes(make(63), 1, 1, { codec, merged }), 'too_many_entries');
    await rejects(Ora.writeBytes([], 1, 1, { codec }), 'invalid');
});

check('write refuses projects above the 32 MiB archive limit', async () => {
    const huge = { width: 2, height: 2, data: new Uint8ClampedArray(16) };
    const bigCodec = Object.assign({}, codec, { encode: async () => new Uint8Array(20 * 1024 * 1024) });
    await rejects(Ora.writeBytes([layer({ canvas: huge }), layer({ canvas: huge })], 2, 2, { codec: bigCodec }), 'too_large');
});

check('read applies layer offsets and cuts off what leaves the document', async () => {
    const small = pattern(3, 2, 7, false);
    const stack = simpleStack('<layer name="moved" src="data/a.png" x="2" y="1"/><layer name="negative" src="data/a.png" x="-2" y="-1"/>', ' xres="72"');
    const zip = buildZip([MIME, { name: 'stack.xml', data: bytesOf(stack), method: 8 }, { name: 'data/a.png', data: pngEncode(small), method: 8 }]);
    const project = await Ora.read(zip, { codec });
    assert.equal(project.layers.length, 2);
    const at = (c, x, y) => Array.from(c.data.subarray((y * c.width + x) * 4, (y * c.width + x) * 4 + 4));
    const [negative, moved] = project.layers.map(l => l.canvas);
    assert.deepEqual(at(moved, 2, 1), at(small, 0, 0));
    assert.deepEqual(at(moved, 3, 2), at(small, 1, 1));
    assert.deepEqual(at(moved, 0, 0), [0, 0, 0, 0]);
    assert.deepEqual(at(negative, 0, 0), at(small, 2, 1), 'the cut-off part is dropped');
    assert.deepEqual(at(negative, 1, 0), [0, 0, 0, 0]);
});

check('read falls back to mergedimage.png when no layer is usable, and rejects what is not OpenRaster', async () => {
    const merged = pattern(4, 3, 5, false);
    const flat = buildZip([MIME, { name: 'stack.xml', data: bytesOf(simpleStack('<layer name="gone" src="data/missing.png"/>')) }, { name: 'mergedimage.png', data: pngEncode(merged) }]);
    const project = await Ora.read(flat, { codec });
    assert.equal(project.layers.length, 1);
    assert.deepEqual(Array.from(project.layers[0].canvas.data), Array.from(merged.data));
    const nothing = buildZip([MIME, { name: 'stack.xml', data: bytesOf(simpleStack('')) }]);
    await rejects(Ora.read(nothing, { codec }), 'invalid');
    await rejects(Ora.read(buildZip([{ name: 'mimetype', data: bytesOf('application/zip') }, { name: 'stack.xml', data: bytesOf(simpleStack('')) }]), { codec }), 'invalid');
    await rejects(Ora.read(buildZip([{ name: 'stack.xml', data: bytesOf(simpleStack('')) }]), { codec }), 'invalid');
    await rejects(Ora.read(bytesOf('definitely not a zip file, just text that is long enough'), { codec }), 'invalid');
    const two = buildZip([MIME, { name: 'stack.xml', data: bytesOf(simpleStack('<layer src="data/a.png"/><layer src="data/a.png"/><layer src="data/a.png"/>')) }, { name: 'data/a.png', data: onePixelPng([1, 2, 3, 255]) }]);
    await rejects(Ora.read(two, { codec, maxLayers: 2 }), 'too_many_layers');
    assert.equal((await Ora.read(two, { codec, maxLayers: 3 })).layers.length, 3);
});

check('read skips layers whose PNG is not a PNG or claims a huge size', async () => {
    const hugeHeader = Buffer.from(pngEncode(canvas(1, 1)));
    hugeHeader.writeUInt32BE(20000, 16);
    hugeHeader.writeUInt32BE(20000, 20);
    const stack = simpleStack('<layer name="a" src="data/a.png"/><layer name="b" src="data/b.png"/><layer name="c" src="data/c.png"/>');
    const zip = buildZip([MIME, { name: 'stack.xml', data: bytesOf(stack) }, { name: 'data/a.png', data: onePixelPng([9, 9, 9, 255]) }, { name: 'data/b.png', data: bytesOf('GIF89a not a png') }, { name: 'data/c.png', data: new Uint8Array(hugeHeader) }]);
    const project = await Ora.read(zip, { codec });
    assert.deepEqual(project.layers.map(l => l.name), ['a']);
});

// --- runtime glue ----------------------------------------------------------------------------------------------------

const install = registered[0].install;
Pixel.acquireTempCanvas = (w, h) => canvas(w, h);
Pixel.releaseTempCanvas = () => {};

function fakeRuntime(extra) {
    const calls = [];
    const r = Object.assign({
        calls,
        notes: [],
        t: key => key,
        notify(note) { this.notes.push(note); },
        state: { disposed: false, imageRead: null, documentRevision: 0 },
        layers: [],
        imgWidth: 0,
        imgHeight: 0,
        MAX_LAYERS: 64,
        oraCodec: codec,
        makeCanvas: canvas,
        filePath: '', fileName: '', isDirty: true,
        saveRecentFile(path) { calls.push(['recent', path]); },
        updateStatus() { calls.push(['status']); }
    }, extra);
    install(r);
    return r;
}

check('oraDataURL produces a data:application/zip URL holding the whole project', async () => {
    const layers = sampleProject();
    const r = fakeRuntime({ layers, imgWidth: W, imgHeight: H, flattenDocument: target => target });
    const url = await r.oraDataURL();
    assert.ok(url.startsWith('data:application/zip;base64,'));
    const bytes = new Uint8Array(Buffer.from(url.slice('data:application/zip;base64,'.length), 'base64'));
    const zip = Ora.zipRead(bytes);
    assert.equal(zip.names[0], 'mimetype');
    assert.ok(zip.has('mergedimage.png') && zip.has('Thumbnails/thumbnail.png'));
    const project = await Ora.read(bytes, { codec });
    assert.equal(project.layers.length, 3);
    await rejects(fakeRuntime({ layers: [], imgWidth: 0, imgHeight: 0 }).oraDataURL(), 'invalid');
});

check('flatten hint appears once, only for documents with more than one layer or a mask', () => {
    const single = fakeRuntime({ layers: [layer({ canvas: canvas(1, 1) })] });
    single.noteFlattenedLayers();
    assert.equal(single.notes.length, 0);
    const masked = fakeRuntime({ layers: [layer({ canvas: canvas(1, 1), maskCanvas: canvas(1, 1) })] });
    masked.noteFlattenedLayers();
    assert.equal(masked.notes.length, 1);
    const stacked = fakeRuntime({ layers: [layer({ canvas: canvas(1, 1) }), layer({ canvas: canvas(1, 1) })] });
    stacked.noteFlattenedLayers();
    stacked.noteFlattenedLayers();
    assert.equal(stacked.notes.length, 1, 'one-time');
    assert.equal(stacked.notes[0].type, 'info');
    assert.ok(/save as \.ora/.test(stacked.notes[0].message), 'falls back to the English text while the key is untranslated');
    assert.ok(!stacked.notes[0].message.startsWith('pixel.'));
    const translated = fakeRuntime({ t: () => 'Ebenen werden zusammengefuehrt', layers: [layer({}), layer({})] });
    translated.noteFlattenedLayers();
    assert.equal(translated.notes[0].message, 'Ebenen werden zusammengefuehrt');
});

check('error messages: specific ones for .ora problems, generic ones otherwise', () => {
    const r = fakeRuntime();
    const msg = (code, mode, status) => r.oraErrorMessage(Object.assign(new Error('x'), { oraCode: code, status }), mode);
    assert.ok(/Too many layers/.test(msg('too_many_entries', 'save')));
    assert.ok(/too large/.test(msg('too_large', 'save')));
    assert.ok(/too large/.test(msg(undefined, 'save', 413)));
    assert.equal(msg(undefined, 'save'), 'Failed to save');
    assert.ok(/not a valid OpenRaster/.test(msg('invalid', 'open')));
    assert.ok(/not supported/.test(msg('unsupported', 'open')));
    assert.ok(/too large/.test(msg('too_large', 'open')));
    assert.ok(/more layers/.test(msg('too_many_layers', 'open')));
    assert.equal(msg(undefined, 'open'), 'Failed to load image');
});

check('loadOraProject downloads, versions by SHA-256, decodes and applies; failures notify; newer loads win', async () => {
    const bytes = await Ora.writeBytes(sampleProject(), W, H, { codec, merged: pattern(W, H, 9, false) });
    const expectedVersion = '"' + crypto.createHash('sha256').update(bytes).digest('hex') + '"';
    const urls = [];
    const gates = [];
    globalThis.fetch = (url, options) => {
        urls.push(url);
        return new Promise((resolve, reject) => {
            options.signal.addEventListener('abort', () => reject(Object.assign(new Error('aborted'), { name: 'AbortError' })));
            gates.push(() => resolve({ ok: true, arrayBuffer: async () => ab(bytes) }));
        });
    };
    const applied = [];
    const r = fakeRuntime({});
    r.applyOraProject = project => applied.push(project);

    const first = r.loadOraProject('Pictures/My Art.ora');
    const second = r.loadOraProject('Pictures/My Art.ora', { saveRecent: false });
    gates.forEach(open => open());
    await Promise.all([first, second]);
    assert.deepEqual(urls, ['/api/desktop/download?path=Pictures%2FMy%20Art.ora', '/api/desktop/download?path=Pictures%2FMy%20Art.ora']);
    assert.equal(applied.length, 1, 'the superseded load applies nothing');
    assert.equal(applied[0].layers.length, 3);
    assert.equal(r.filePath, 'Pictures/My Art.ora');
    assert.equal(r.fileName, 'My Art.ora');
    assert.equal(r.state.fileVersion, expectedVersion);
    assert.equal(r.state.versionPath, 'Pictures/My Art.ora');
    assert.equal(r.isDirty, false);
    assert.equal(r.calls.filter(c => c[0] === 'recent').length, 0, 'saveRecent: false is honoured');

    const failing = fakeRuntime({});
    failing.applyOraProject = () => { throw new Error('must not apply'); };
    globalThis.fetch = async () => ({ ok: false });
    await failing.loadOraProject('Pictures/missing.ora');
    assert.deepEqual(failing.notes.map(n => n.message), ['Failed to load image']);
    globalThis.fetch = async () => ({ ok: true, arrayBuffer: async () => ab(bytesOf('PK this is garbage, not an openraster file at all')) });
    await failing.loadOraProject('Pictures/garbage.ora');
    assert.equal(failing.notes[1].message, 'This is not a valid OpenRaster file.');
    assert.equal(failing.notes[1].type, 'error');
    assert.equal(failing.filePath, '', 'a failed open leaves the current document alone');

    const disposed = fakeRuntime({});
    disposed.state.disposed = true;
    disposed.applyOraProject = () => { throw new Error('must not apply'); };
    globalThis.fetch = async () => ({ ok: true, arrayBuffer: async () => ab(bytes) });
    await disposed.loadOraProject('Pictures/closed.ora');
    assert.equal(disposed.notes.length, 0);
});

check('applyOraProject replaces the document with the decoded layers, top layer active, clean history', async () => {
    const project = await Ora.read(await Ora.writeBytes(sampleProject(), W, H, { codec }), { codec });
    const log = [];
    const createdWith = [];
    const r = fakeRuntime({
        canvas: { width: 0, height: 0 }, overlayCanvas: { width: 0, height: 0 }, canvasWrap: { hidden: true }, emptyState: { hidden: false },
        resetStickerDocument() { log.push('resetSticker'); }, beginDocumentReplace() { log.push('begin'); },
        createLayer(spec) { createdWith.push(spec); return Object.assign({ id: createdWith.length }, spec); },
        pushHistory(label) { log.push('history:' + label); }, clearOverlay() { log.push('overlay'); }, captureAdjustmentSource() { log.push('capture'); },
        compositeLayers() { log.push('composite'); }, zoomFit() { log.push('zoom'); }, refreshLayerPanel() { log.push('panel'); },
        syncActiveLayer() { log.push('sync'); }, notifyLayersChanged() { log.push('notify'); }, emit(name, data) { log.push('emit:' + name + ':' + data.reason + ':' + data.width + 'x' + data.height); },
        t: key => ({ 'pixel.layer_background': 'Background', 'pixel.layer': 'Layer' }[key] || key)
    });
    r.applyOraProject(project);
    assert.equal(r.canvas.width, W);
    assert.equal(r.overlayCanvas.height, H);
    assert.equal(r.imgWidth, W);
    assert.equal(r.layers.length, 3);
    assert.equal(r.activeLayerIdx, 2);
    assert.equal(r.selection, null);
    assert.equal(r.canvasWrap.hidden, false);
    assert.equal(r.emptyState.hidden, true);
    assert.deepEqual(createdWith.map(c => c.name), project.layers.map(l => l.name));
    assert.ok(createdWith.every(c => c.canvas && c.canvas.width === W), 'document-sized canvases are handed to createLayer');
    assert.equal(createdWith[2].maskEnabled, false);
    assert.ok(createdWith[2].maskCanvas);
    assert.equal(log[0], 'resetSticker');
    assert.equal(log[1], 'begin');
    assert.ok(log.includes('history:open') && log.includes('emit:document-changed:open:300x120'));
    assert.ok(log.indexOf('history:open') < log.indexOf('composite'));

    const unnamed = fakeRuntime({ canvas: { width: 0, height: 0 }, overlayCanvas: { width: 0, height: 0 }, canvasWrap: {}, emptyState: {}, resetStickerDocument() {}, beginDocumentReplace() {}, createLayer: spec => spec, pushHistory() {}, clearOverlay() {}, captureAdjustmentSource() {}, compositeLayers() {}, zoomFit() {}, refreshLayerPanel() {}, syncActiveLayer() {}, notifyLayersChanged() {}, emit() {}, t: key => ({ 'pixel.layer_background': 'Background', 'pixel.layer': 'Layer' }[key] || key) });
    unnamed.applyOraProject({ width: 2, height: 2, layers: [{ name: '', canvas: canvas(2, 2) }, { name: '', canvas: canvas(2, 2) }] });
    assert.deepEqual(unnamed.layers.map(l => l.name), ['Background', 'Layer 2'], 'files without layer names get readable defaults');
});

await Promise.all(pending);
console.log(results.join('\n'));
if (results.some(line => line.startsWith('FAIL'))) process.exit(1);
console.log('test-pixel-ora: ' + results.length + ' checks passed');
