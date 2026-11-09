// Camera creative controls: WYSIWYG framing maths and controller lifecycle.
// Run with `npm run test:camera` (also chained into test:ui-regressions).
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const source = fs.readFileSync(path.join(root, 'ui/js/desktop/apps/camera-creative.js'), 'utf8');
const context = { window: {}, Intl, console };
vm.createContext(context);
vm.runInContext(source, context);
const creative = context.window.CameraCreative;
assert(creative, 'camera-creative.js must export window.CameraCreative');
assert.equal(creative.LOOKS.length, 5, 'five photo looks');
assert.equal(creative.RATIOS.length, 4, 'original plus three fixed ratios');
assert.equal(creative.ZOOM_STEPS[0], 1, 'zoom starts at 1x');

const near = (actual, expected, label) => {
    assert(Math.abs(actual - expected) < 1e-9, `${label}: got ${actual}, want ${expected}`);
};

// A 16:9 source shown in a 16:9 viewport keeps every pixel.
let rect = creative.sourceRect({ videoWidth: 1280, videoHeight: 720 }, { clientWidth: 1280, clientHeight: 720 }, 'auto', 1);
near(rect.x, 0, 'native x');
near(rect.y, 0, 'native y');
near(rect.w, 1280, 'native width');
near(rect.h, 720, 'native height');
near(rect.mask.width, 100, 'native mask width');

// object-fit: cover in a 4:3 viewport crops the sides of a 16:9 source.
rect = creative.sourceRect({ videoWidth: 1280, videoHeight: 720 }, { clientWidth: 640, clientHeight: 480 }, 'auto', 1);
near(rect.x, 160, 'cover x');
near(rect.w, 960, 'cover width');
near(rect.h, 720, 'cover height');
near(rect.w / rect.h, 4 / 3, 'capture matches the viewport aspect');

// A square crop inside that visible region has an exact on-screen mask.
rect = creative.sourceRect({ videoWidth: 1280, videoHeight: 720 }, { clientWidth: 640, clientHeight: 480 }, '1:1', 1);
near(rect.w, 720, 'square width');
near(rect.h, 720, 'square height');
near(rect.x, 280, 'square x');
near(rect.mask.left, 12.5, 'square mask left');
near(rect.mask.width, 75, 'square mask width');

// Digital zoom shrinks the sampled region; a ratio then crops inside it.
rect = creative.sourceRect({ videoWidth: 1280, videoHeight: 720 }, { clientWidth: 640, clientHeight: 480 }, '16:9', 2);
near(rect.w, 480, 'zoomed 16:9 width');
near(rect.h, 270, 'zoomed 16:9 height');
near(rect.x, 400, 'zoomed 16:9 x');
near(rect.y, 225, 'zoomed 16:9 y');
near(rect.mask.top, 12.5, 'zoomed mask top');
near(rect.mask.height, 75, 'zoomed mask height');

// 4:3 from a 16:9 viewport, and a zero-size source, stay well behaved.
rect = creative.sourceRect({ videoWidth: 1280, videoHeight: 720 }, { clientWidth: 1280, clientHeight: 720 }, '4:3', 1);
near(rect.w, 960, '4:3 width');
near(rect.x, 160, '4:3 x');
assert.equal(creative.sourceRect({ videoWidth: 0, videoHeight: 0 }, { clientWidth: 640, clientHeight: 480 }, 'auto', 1), null, 'no stream has no crop');

// Every combination stays inside the source and produces a usable mask.
for (const ratio of ['auto', '1:1', '4:3', '16:9']) {
    for (const zoom of creative.ZOOM_STEPS) {
        for (const viewport of [{ clientWidth: 320, clientHeight: 240 }, { clientWidth: 900, clientHeight: 500 }, { clientWidth: 500, clientHeight: 900 }]) {
            const candidate = creative.sourceRect({ videoWidth: 1280, videoHeight: 720 }, viewport, ratio, zoom);
            assert(candidate.w > 0 && candidate.h > 0, `empty crop for ${ratio}/${zoom}`);
            assert(candidate.x >= -1e-9 && candidate.y >= -1e-9, `negative origin for ${ratio}/${zoom}`);
            assert(candidate.x + candidate.w <= 1280 + 1e-9, `right overflow for ${ratio}/${zoom}`);
            assert(candidate.y + candidate.h <= 720 + 1e-9, `bottom overflow for ${ratio}/${zoom}`);
            assert(candidate.mask.left >= -1e-9 && candidate.mask.top >= -1e-9, `mask origin for ${ratio}/${zoom}`);
            assert(candidate.mask.width > 0 && candidate.mask.height > 0, `mask size for ${ratio}/${zoom}`);
            assert(candidate.mask.left + candidate.mask.width <= 100 + 1e-9, `mask overflow for ${ratio}/${zoom}`);
        }
    }
}

// A minimal DOM exercises create/paint/interaction/dispose without a browser.
function stub(extra = {}) {
    return Object.assign({
        style: {}, dataset: {}, hidden: false, disabled: false, title: '', textContent: '',
        classList: { toggle() {}, add() {}, remove() {} },
        setAttribute() {}, addEventListener() {}, removeEventListener() {},
        insertAdjacentHTML() {}, remove() {}, querySelector() { return stub(); },
        querySelectorAll() { return []; }
    }, extra);
}
const toggleBtn = stub();
const pauseBtn = stub();
const mask = stub();
const zoomValue = stub();
const lookBtns = ['natural', 'mono', 'warm', 'cool', 'vintage'].map(id => stub({ dataset: { ccLook: id } }));
const ratioBtns = ['auto', '1:1', '4:3', '16:9'].map(id => stub({ dataset: { ccRatio: id } }));
const zoomBtns = [stub(), stub()];
const panel = stub({
    querySelector(selector) { return selector === '[data-cc-zoom-value]' ? zoomValue : stub(); },
    querySelectorAll(selector) {
        if (selector === '[data-cc-look]') return lookBtns;
        if (selector === '[data-cc-ratio]') return ratioBtns;
        if (selector === '[data-cc-zoom]') return zoomBtns;
        return [];
    }
});
const toolsHost = stub({ querySelector: () => toggleBtn });
const pauseHost = stub({ querySelector: () => pauseBtn });
const dock = stub({ querySelector: () => panel });
const viewport = stub({ clientWidth: 640, clientHeight: 480, querySelector: () => mask });
const video = { videoWidth: 1280, videoHeight: 720, style: {} };
const dom = {
    window: context.window,
    document: {
        createElement: () => ({ getContext: () => ({ filter: 'none' }) }),
        addEventListener() {}, removeEventListener() {}
    },
    Intl,
    console
};
vm.createContext(dom);
vm.runInContext(source, dom);
const controller = dom.window.CameraCreative.create({
    t: key => key,
    esc: value => String(value == null ? '' : value),
    iconMarkup: () => '<span></span>',
    notify: () => {},
    video, viewport, dock, toolsHost, pauseHost,
    onChange: () => {}, onPause: () => {}
});
assert(controller, 'create() must return a controller');
assert.equal(controller.isOpen(), false, 'panel starts closed');
controller.toggle();
assert.equal(controller.isOpen(), true, 'panel opens');
controller.cycleLook();
controller.cycleRatio();
controller.zoomBy(1);
controller.setContext({ mode: 'video' });
assert.equal(controller.isOpen(), false, 'video mode closes the photo-only panel');
controller.setContext({ mode: 'photo', recording: true, pauseSupported: true, paused: true });
assert.equal(controller.hintPairs(false, true)[0][0], 'P', 'recording advertises the pause shortcut');
assert.equal(controller.menuItems().length, 2, 'look and ratio window-menu entries');
assert(controller.sourceRect(), 'controller exposes a capture rectangle');
assert.notEqual(video.style.filter, undefined, 'live video gets a filter value');
controller.dispose();

console.log('camera creative: framing maths, controls and disposal passed');
