// Unit + integration checks for Pixel 3D Text (pixel-lib-text3d.js, pixel-text3d-gl.js, pixel-text3d.js, pixel-text3d-ui.js):
// catalogs, exact anti-aliased 2D SDF, deterministic procedural textures, orthonormal 3D rotations, all 12 deformations,
// 3D box projection, CPU 3D SDF raymarching & worker op, plus full Pixel runtime tool/panel/menu/command/drag/apply/undo.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { installFakeDom, makeEl, loadPixelApp, englishT } from './pixel-test-dom.mjs';
import { appDir } from './pixel-test-util.mjs';

installFakeDom();
const consoleErrors = [];
const realConsoleError = console.error;
console.error = (...args) => { consoleErrors.push(args.map(a => (a && a.message) || String(a)).join(' ')); };
const Pixel = loadPixelApp();

const T = globalThis.AuraPixelLib && globalThis.AuraPixelLib.text3d;
assert.ok(T, 'AuraPixelLib.text3d loaded');

const runtimes = [];
Pixel.registerModule('text3d-test-capture', r => { runtimes.push(r); });
const notes = [];
let menusSeen = [];
function openWindow(id, opts) {
    const host = makeEl('div');
    window.PixelApp.render(host, id, Object.assign({
        esc: v => String(v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'),
        t: englishT(),
        api: async () => ({}),
        iconMarkup: () => '',
        notify: n => notes.push(n),
        setWindowMenus: (_, m) => { menusSeen = m; },
        readonly: false
    }, opts || {}));
    return { host, rt: runtimes[runtimes.length - 1] };
}

const { host, rt } = openWindow('t3d-1');
console.error = realConsoleError;
assert.equal(consoleErrors.length, 0, 'no install console errors: ' + consoleErrors.join('; '));

const results = [];
const pending = [];
function check(name, fn) {
    const fail = err => results.push('FAIL ' + name + ': ' + (err && err.stack || err));
    try {
        const out = fn();
        if (out && typeof out.then === 'function') {
            pending.push(out.then(() => results.push('ok   ' + name), fail));
            return;
        }
        results.push('ok   ' + name);
    } catch (err) { fail(err); }
}

function makeBlockAlpha(w, h) {
    const alpha = new Uint8ClampedArray(w * h);
    for (let y = Math.floor(h * 0.25); y < Math.ceil(h * 0.75); y++) {
        for (let x = Math.floor(w * 0.2); x < Math.ceil(w * 0.8); x++) alpha[y * w + x] = 255;
    }
    return alpha;
}

function bufferDiff(a, b) {
    let diff = 0;
    for (let i = 0; i < a.length; i++) diff += Math.abs(a[i] - b[i]);
    return diff;
}

// --- pure library tests ---

check('catalogs expose unique fonts, textures, warps, bevels and presets', () => {
    assert.ok(T.FONTS.length >= 16, 'at least 16 fonts');
    assert.ok(T.TEXTURES.length >= 16, 'at least 16 textures');
    assert.ok(T.WARPS.length >= 12, 'at least 12 warps');
    assert.equal(T.BEVELS.length, 4);
    assert.ok(T.ANGLE_PRESETS.length >= 6);
    assert.ok(T.STYLE_PRESETS.length >= 6);
    for (const list of [T.FONTS, T.TEXTURES, T.WARPS, T.BEVELS, T.ANGLE_PRESETS, T.STYLE_PRESETS]) {
        const ids = list.map(x => x.id);
        assert.equal(new Set(ids).size, ids.length, 'unique ids');
    }
});

check('computeSDF produces accurate signed distances and RGBA8 encoding', () => {
    const w = 48, h = 32;
    const alpha = makeBlockAlpha(w, h);
    const res = T.computeSDF(alpha, w, h, 24);
    assert.equal(res.sdf.length, w * h);
    assert.equal(res.rgba8.length, w * h * 4);
    const center = res.sdf[Math.floor(h / 2) * w + Math.floor(w / 2)];
    const corner = res.sdf[0];
    assert.ok(center < -4, 'inside glyph is negative: ' + center);
    assert.ok(corner > 4, 'outside glyph is positive: ' + corner);
    // Empty alpha should stay positive everywhere without NaNs.
    const empty = T.computeSDF(new Uint8ClampedArray(16 * 16), 16, 16, 20);
    assert.ok(empty.sdf.every(v => Number.isFinite(v) && v > 0));
});

check('all procedural textures are deterministic, valid and distinct', () => {
    const seen = new Map();
    for (const tx of T.TEXTURES) {
        const a = T.generateTexture(tx.id, 32, { color: '#ff5522', sideColor: '#2255ff' });
        const b = T.generateTexture(tx.id, 32, { color: '#ff5522', sideColor: '#2255ff' });
        assert.equal(a.width, 32);
        assert.equal(a.height, 32);
        assert.equal(a.data.length, 32 * 32 * 4);
        assert.equal(bufferDiff(a.data, b.data), 0, tx.id + ' must be deterministic');
        const sw = T.generateTexture(tx.id, 24, { opaque: true });
        for (let i = 3; i < sw.data.length; i += 4) assert.equal(sw.data[i], 255, tx.id + ' swatch alpha opaque');
        const sig = Array.from(a.data.subarray(0, 64)).join(',');
        assert.ok(!seen.has(sig), tx.id + ' collides with ' + seen.get(sig));
        seen.set(sig, tx.id);
    }
});

check('rotationMatrix is strictly orthonormal with determinant +1', () => {
    for (const [p, y, r] of [[0, 0, 0], [-20, 35, 12], [45, -120, -65], [-85, 180, 90]]) {
        const R = T.rotationMatrix(p, y, r);
        const Rt = T.transposeMat3(R);
        // R * Rt == I
        for (let row = 0; row < 3; row++) {
            for (let col = 0; col < 3; col++) {
                let sum = 0;
                for (let k = 0; k < 3; k++) sum += R[row * 3 + k] * Rt[k * 3 + col];
                assert.ok(Math.abs(sum - (row === col ? 1 : 0)) < 1e-6);
            }
        }
        const det = R[0] * (R[4] * R[8] - R[5] * R[7]) - R[1] * (R[3] * R[8] - R[5] * R[6]) + R[2] * (R[3] * R[7] - R[4] * R[6]);
        assert.ok(Math.abs(det - 1) < 1e-6, 'det=' + det);
    }
});

check('all 12 warps produce finite coordinates and non-none warps deform distinctly', () => {
    const base = T.inverseWarp(0.8, 0.4, 0.2, 2.0, 0, 50, 2);
    assert.deepEqual(base, [0.8, 0.4, 0.2]);
    for (const w of T.WARPS) {
        const q = T.inverseWarp(0.8, 0.4, 0.2, 2.0, w.code, 55, 2);
        assert.equal(q.length, 3);
        assert.ok(q.every(Number.isFinite), w.id + ' finite');
        if (w.id !== 'none') {
            const delta = Math.hypot(q[0] - 0.8, q[1] - 0.4, q[2] - 0.2);
            assert.ok(delta > 0.01, w.id + ' must deform coordinates (delta=' + delta + ')');
        }
        const bounds = T.deformedBounds(2.0, 0.3, w.id, 60, 2);
        assert.ok(bounds.hx >= 2.0 && bounds.hy >= 1.0 && bounds.hz >= 0.3, w.id + ' bounds');
    }
});

check('projectBox produces valid projected corners and clamped viewport rect', () => {
    const proj = T.projectBox(T.defaultParams({ yaw: 30, pitch: -15, fontSize: 120 }), 2.2, 800, 600, 1);
    assert.equal(proj.front.length, 4);
    assert.equal(proj.back.length, 4);
    assert.ok(proj.rect.x >= 0 && proj.rect.y >= 0 && proj.rect.x + proj.rect.w <= 800 && proj.rect.y + proj.rect.h <= 600);
    assert.ok(proj.rect.w > 40 && proj.rect.h > 20);
    assert.ok(proj.orbitR >= 36 && Number.isFinite(proj.rollHandle.x) && Number.isFinite(proj.rollHandle.y));
});

check('CPU 3D raymarching renders shaded opaque text, shadow, and reacts to rotation/texture/warp', async () => {
    const sdf = T.computeSDF(makeBlockAlpha(64, 32), 64, 32, 24);
    const goldTex = T.generateTexture('gold', 64, {});
    const lavaTex = T.generateTexture('lava', 64, {});
    const baseParams = T.defaultParams({ fontSize: 64, yaw: 0, pitch: 0, warp: 'none', texture: 'gold', depth: 50, shadow: true });
    const imgFront = T.renderCPU(sdf, goldTex, baseParams, 96, 64, { docW: 96, docH: 64, aspect: 2.0, padFactor: 1.5, draft: true });
    let opaqueCount = 0, transparentCount = 0, shadowCount = 0;
    const lumSet = new Set();
    for (let i = 0; i < imgFront.data.length; i += 4) {
        const a = imgFront.data[i + 3];
        if (a === 255) {
            opaqueCount++;
            lumSet.add(Math.round((imgFront.data[i] + imgFront.data[i + 1] + imgFront.data[i + 2]) / 12));
        } else if (a === 0) transparentCount++;
        else shadowCount++;
    }
    assert.ok(opaqueCount > 80, 'opaque 3D text pixels rendered: ' + opaqueCount);
    assert.ok(transparentCount > 80, 'background remains transparent: ' + transparentCount);
    assert.ok(lumSet.size >= 4, '3D shading produces tonal variation: ' + lumSet.size);

    const imgRotated = T.renderCPU(sdf, goldTex, Object.assign({}, baseParams, { yaw: 48, pitch: -28 }), 96, 64, { docW: 96, docH: 64, aspect: 2.0, padFactor: 1.5, draft: true });
    assert.ok(bufferDiff(imgFront.data, imgRotated.data) > 5000, '3D rotation changes rendered pixels');

    const imgWarped = T.renderCPU(sdf, goldTex, Object.assign({}, baseParams, { warp: 'twist', warpStrength: 75 }), 96, 64, { docW: 96, docH: 64, aspect: 2.0, padFactor: 1.5, draft: true });
    assert.ok(bufferDiff(imgFront.data, imgWarped.data) > 5000, '3D warp changes rendered pixels');

    const imgLava = T.renderCPU(sdf, lavaTex, Object.assign({}, baseParams, { texture: 'lava' }), 96, 64, { docW: 96, docH: 64, aspect: 2.0, padFactor: 1.5, draft: true });
    assert.ok(bufferDiff(imgFront.data, imgLava.data) > 5000, 'texture switch changes rendered pixels');

    const workerRes = await rt.runJob('text3d.render', { sdf, texture: goldTex, params: baseParams, width: 48, height: 32, docW: 48, docH: 32, aspect: 2.0, padFactor: 1.5, draft: true });
    assert.equal(workerRes.width, 48);
    assert.equal(workerRes.height, 32);
    assert.equal(workerRes.data.length, 48 * 32 * 4);
});

// --- Pixel module / tool / panel / interaction tests ---

check('tool, panel, generate menu item and command are registered cleanly', () => {
    const tool = Pixel.getTool('text-3d');
    assert.ok(tool, 'text-3d tool registered');
    assert.equal(tool.group, 'type');
    assert.equal(tool.shortcut, 'Shift+T');
    const panel = Pixel.getPanel('text3d');
    assert.ok(panel, 'text3d panel registered');
    assert.equal(panel.dock, 'inspector');
    const genMenu = menusSeen.find(m => m.id === 'generate');
    assert.ok(genMenu && genMenu.items.some(i => i.id === 'text-3d'), 'generate menu has text-3d');
    assert.ok(rt.listCommands().some(c => c.id === 'text3d.open'), 'command palette has text3d.open');
});

check('openText3D creates transient preview without polluting history, drag moves and rotates in 3D', () => {
    rt.newBlankCanvas(240, 160);
    const histBefore = rt.history.length;
    const layersBefore = rt.layers.length;
    rt.openText3D({ text: 'HELLO', font: 'oxanium', texture: 'chrome', warp: 'arc' });
    const st = rt.text3DState();
    assert.equal(rt.activeTool, 'text-3d');
    assert.equal(rt.activePanels.inspector, 'text3d');
    assert.equal(st.active, true);
    assert.ok(st.transient && st.transient.transient === true, 'transient preview layer created');
    assert.equal(rt.history.length, histBefore, 'preview does not push history');

    // Drag inside bounding box -> moves position.
    const tool = Pixel.getTool('text-3d');
    const cx = st.proj.cx, cy = st.proj.cy;
    tool.onDown(rt, { button: 0 }, { x: cx, y: cy, shift: false, alt: false });
    tool.onMove(rt, { button: 0 }, { x: cx + 36, y: cy - 20, shift: false, alt: false });
    tool.onUp(rt, { button: 0 }, { x: cx + 36, y: cy - 20, shift: false, alt: false });
    assert.ok(st.params.posX > 0.6, 'posX moved right: ' + st.params.posX);
    assert.ok(st.params.posY < 0.42, 'posY moved up: ' + st.params.posY);

    // Drag outside bounding box -> rotates yaw and pitch in 3D space.
    const yaw0 = st.params.yaw, pitch0 = st.params.pitch;
    tool.onDown(rt, { button: 0 }, { x: 2, y: 2, shift: false, alt: false });
    tool.onMove(rt, { button: 0 }, { x: 62, y: 42, shift: false, alt: false });
    tool.onUp(rt, { button: 0 }, { x: 62, y: 42, shift: false, alt: false });
    assert.notEqual(st.params.yaw, yaw0, 'yaw rotated');
    assert.notEqual(st.params.pitch, pitch0, 'pitch rotated');

    // Drag roll handle -> updates roll.
    const roll0 = st.params.roll;
    const rh = st.proj.rollHandle;
    tool.onDown(rt, { button: 0 }, { x: rh.x, y: rh.y, shift: false, alt: false });
    tool.onMove(rt, { button: 0 }, { x: rh.x + 35, y: rh.y + 15, shift: false, alt: false });
    tool.onUp(rt, { button: 0 }, { x: rh.x + 35, y: rh.y + 15, shift: false, alt: false });
    assert.notEqual(st.params.roll, roll0, 'roll rotated');

    // Drag corner handle -> scales fontSize.
    const size0 = st.params.fontSize;
    tool.onDown(rt, { button: 0 }, { x: st.proj.maxX, y: st.proj.maxY, shift: false, alt: false });
    tool.onMove(rt, { button: 0 }, { x: st.proj.maxX + 25, y: st.proj.maxY + 25, shift: false, alt: false });
    tool.onUp(rt, { button: 0 }, { x: st.proj.maxX + 25, y: st.proj.maxY + 25, shift: false, alt: false });
    assert.ok(st.params.fontSize > size0, 'fontSize scaled up');

    // Escape cancels cleanly without adding layers or history.
    assert.equal(tool.onKey(rt, { key: 'Escape' }), true);
    assert.equal(st.active, false);
    assert.equal(rt.layers.length, layersBefore);
    assert.equal(rt.history.length, histBefore);
});

check('applyText3D commits a raster layer with single undoable history entry', () => {
    rt.newBlankCanvas(200, 140);
    const hist0 = rt.history.length;
    rt.openText3D({ text: 'AURAGO 3D', texture: 'gold', warp: 'cylinder' });
    assert.equal(rt.applyText3D(), true);
    assert.equal(rt.layers.length, 2, 'new raster layer added');
    assert.equal(rt.layers[1].name, '3D: AURAGO 3D');
    assert.equal(rt.layers[1].transient, undefined);
    assert.equal(rt.history.length, hist0 + 1, 'one undoable history entry');
    assert.equal(rt.history[rt.historyIdx].label, 'text-3d');
    rt.undo();
    assert.equal(rt.layers.length, 1, 'undo removes 3D text layer');
    rt.redo();
    assert.equal(rt.layers.length, 2, 'redo restores 3D text layer');
});

check('readonly and empty text refuse commit; no-document commit creates canvas', () => {
    const roWin = openWindow('t3d-ro', { readonly: true });
    roWin.rt.newBlankCanvas(100, 80);
    roWin.rt.openText3D({ text: 'LOCKED' });
    assert.equal(roWin.rt.applyText3D(), false);
    assert.equal(roWin.rt.layers.filter(l => !l.transient).length, 1);
    window.PixelApp.dispose('t3d-ro');

    rt.openText3D({ text: '   ' });
    assert.equal(rt.applyText3D(), false);
    rt.cancelText3D();

    const fresh = openWindow('t3d-empty');
    fresh.rt.openText3D({ text: 'FRESH' });
    assert.equal(fresh.rt.applyText3D(), true);
    assert.equal(fresh.rt.imgWidth, 1024);
    assert.equal(fresh.rt.imgHeight, 768);
    assert.equal(fresh.rt.layers.length, 2);
    window.PixelApp.dispose('t3d-empty');
});

check('new 3D text scripts stay under line budget and lib stays DOM-free', () => {
    const libSrc = fs.readFileSync(path.join(appDir, 'pixel-lib-text3d.js'), 'utf8');
    for (const forbidden of ['document.', 'window.', 'localStorage', 'createElement', 'Math.random']) {
        assert.ok(!libSrc.includes(forbidden), 'pixel-lib-text3d.js must not contain ' + forbidden);
    }
    for (const file of ['pixel-lib-text3d.js', 'pixel-text3d-gl.js', 'pixel-text3d.js', 'pixel-text3d-ui.js']) {
        const lines = fs.readFileSync(path.join(appDir, file), 'utf8').split('\n').length;
        assert.ok(lines < 1100, file + ' has ' + lines + ' lines, want < 1100');
    }
});

await Promise.all(pending);
window.PixelApp.dispose('t3d-1');
const failed = results.filter(r => r.startsWith('FAIL '));
results.forEach(line => console.log(line));
if (failed.length) {
    console.error('test-pixel-text3d: ' + failed.length + ' failed');
    process.exit(1);
}
console.log('test-pixel-text3d: ' + results.length + ' checks passed');
