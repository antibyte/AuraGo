(function () {
    'use strict';

    // Colour wheels (spec §8.3; plan 2c Task 15). A wheel holds one x/y pair of the color effect (lift, gamma or gain).
    // Index §4.20: the channel offset is x·cos φc + y·sin φc with φ = 0°, 120°, 240°. So the point's angle is the hue the
    // wheel pushes towards (0° red, 120° green, 240° blue, counter-clockwise), and its distance from the centre is the
    // strength. Model y points up, the SVG's y points down; the UI keeps the point inside the unit disc (the model allows
    // -1..1 per value).
    // - clampDisc, fromPointer, keyStep, polar, valueText and ringColor are DOM-free and Node-tested.
    // - markup builds the SVG, update moves it in place, wire adds delegated pointer and key handling.
    // No external assets.
    const STEP = 0.01, BIG_STEP = 0.1, SEGMENTS = 72;
    const DISC = 1 / 1.2;              // the disc's share of the half size: the viewBox reaches 1.2 for the hue ring
    const RING_IN = 1.06, RING_OUT = 1.18;
    const round3 = value => Math.round(value * 1000) / 1000;
    let ring = '';

    // (x, y) inside the unit disc, rounded to the registry step 0.001 (no negative zero).
    function clampDisc(x, y) {
        let px = Number(x) || 0, py = Number(y) || 0;
        const r = Math.hypot(px, py);
        if (r > 1) { px /= r; py /= r; }
        return { x: round3(px) + 0, y: round3(py) + 0 };
    }
    // A pointer at client (clientX, clientY) over a wheel whose bounding rect is rect.
    function fromPointer(clientX, clientY, rect) {
        const radius = Math.min(rect.width, rect.height) / 2 * DISC;
        if (!(radius > 0)) return { x: 0, y: 0 };
        return clampDisc((clientX - (rect.left + rect.width / 2)) / radius, -(clientY - (rect.top + rect.height / 2)) / radius);
    }
    // Arrow keys move the point by 0.01 (Shift: 0.1), Delete and Backspace reset it; null for any other key.
    function keyStep(x, y, key, shift) {
        const d = shift ? BIG_STEP : STEP, px = Number(x) || 0, py = Number(y) || 0;
        if (key === 'ArrowRight') return clampDisc(px + d, py);
        if (key === 'ArrowLeft') return clampDisc(px - d, py);
        if (key === 'ArrowUp') return clampDisc(px, py + d);
        if (key === 'ArrowDown') return clampDisc(px, py - d);
        if (key === 'Delete' || key === 'Backspace') return { x: 0, y: 0 };
        return null;
    }
    // Hue in whole degrees (0..359) and strength in whole percent (0..100).
    function polar(x, y) {
        const px = Number(x) || 0, py = Number(y) || 0, strength = Math.min(100, Math.round(Math.hypot(px, py) * 100));
        if (!strength) return { hue: 0, strength: 0 };
        return { hue: Math.round(((Math.atan2(py, px) * 180 / Math.PI) % 360 + 360) % 360) % 360, strength };
    }
    // The screen-reader text: "Hue {{hue}}°, strength {{strength}} %" (tr(key, fallback)).
    function valueText(tr, x, y) {
        const { hue, strength } = polar(x, y);
        return tr('colorWheelValue', 'Hue {{hue}}°, strength {{strength}} %').split('{{hue}}').join(String(hue)).split('{{strength}}').join(String(strength));
    }
    // The colour a wheel pushes towards at angle a (radians), from the shared φ mapping (VideoStudioPicture.wheel), with
    // every channel at 0.5 + 0.5·offset, as #rrggbb.
    function ringColor(angle) {
        const rgb = window.VideoStudioPicture.wheel(Math.cos(angle), Math.sin(angle));
        return '#' + rgb.map(value => Math.round(Math.max(0, Math.min(255, 127.5 + 127.5 * value))).toString(16).padStart(2, '0')).join('');
    }
    // The hue ring: SEGMENTS wedges between RING_IN and RING_OUT, each in the colour of its middle angle (built once).
    function ringMarkup() {
        if (ring) return ring;
        const point = (a, r) => (r * Math.cos(a)).toFixed(4) + ' ' + (-r * Math.sin(a)).toFixed(4);
        for (let i = 0; i < SEGMENTS; i++) {
            const a0 = i / SEGMENTS * 2 * Math.PI, a1 = (i + 1.02) / SEGMENTS * 2 * Math.PI, mid = (i + 0.5) / SEGMENTS * 2 * Math.PI;
            ring += `<path d="M${point(a0, RING_IN)}L${point(a0, RING_OUT)}A${RING_OUT} ${RING_OUT} 0 0 0 ${point(a1, RING_OUT)}L${point(a1, RING_IN)}A${RING_IN} ${RING_IN} 0 0 1 ${point(a0, RING_IN)}Z" fill="${ringColor(mid)}"/>`;
        }
        return ring;
    }
    // One wheel. opts = {name, label, x, y, tr, esc, disabled}.
    // - The focusable element is the role="slider" div: aria-valuenow is the strength, aria-valuetext gives hue and
    //   strength.
    // - A disabled wheel (locked track, read-only, or animated while the playhead is outside the clip) takes no focus and
    //   no input.
    function markup(opts) {
        const v = clampDisc(opts.x, opts.y), esc = opts.esc, tr = opts.tr, off = !!opts.disabled;
        return `<div class="vs-wheel" role="slider" tabindex="${off ? -1 : 0}" data-wheel="${esc(opts.name)}" data-wheel-x="${v.x}" data-wheel-y="${v.y}" aria-label="${esc(opts.label)}" aria-roledescription="${esc(tr('colorWheelRole', 'color wheel'))}" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${polar(v.x, v.y).strength}" aria-valuetext="${esc(valueText(tr, v.x, v.y))}"${off ? ' aria-disabled="true"' : ''} title="${esc(tr('colorWheelHint', 'Drag the point or use the arrow keys (Shift: bigger steps). Double-click or Delete resets the wheel.'))}"><svg class="vs-wheel-svg" viewBox="-1.2 -1.2 2.4 2.4" aria-hidden="true" focusable="false"><g class="vs-wheel-ring">${ringMarkup()}</g><circle class="vs-wheel-disc" r="1"/><path class="vs-wheel-cross" d="M-1 0H1M0 -1V1"/><circle class="vs-wheel-point" cx="${v.x}" cy="${-v.y + 0}" r="0.09"/></svg></div>`;
    }
    // Moves a wheel's point and its ARIA values in place (during a drag, after keys and while the playhead moves).
    function update(el, x, y, tr) {
        const v = clampDisc(x, y), dot = el.querySelector('.vs-wheel-point');
        el.dataset.wheelX = String(v.x);
        el.dataset.wheelY = String(v.y);
        if (dot) { dot.setAttribute('cx', String(v.x)); dot.setAttribute('cy', String(-v.y + 0)); }
        el.setAttribute('aria-valuenow', String(polar(v.x, v.y).strength));
        el.setAttribute('aria-valuetext', valueText(tr, v.x, v.y));
        return v;
    }
    // Delegated handling on root for every [data-wheel]. handlers:
    // - live(el, x, y) runs during a drag (pointer captured);
    // - commit(el, cancelled) runs at its end;
    // - change(el, x, y) runs for keys and resets (double-click, Delete), one edit each.
    function wire(root, handlers, signal) {
        let drag = null;
        const wheelOf = target => (target && target.closest ? target.closest('[data-wheel]') : null);
        const usable = el => !!el && el.getAttribute('aria-disabled') !== 'true';
        root.addEventListener('pointerdown', event => {
            const el = wheelOf(event.target);
            if (!usable(el) || event.button !== 0) return;
            event.preventDefault();
            el.focus({ preventScroll: true });
            drag = { el, id: event.pointerId };
            try { el.setPointerCapture(event.pointerId); } catch (_) { /* synthetic or finished pointer */ }
            const v = fromPointer(event.clientX, event.clientY, el.getBoundingClientRect());
            handlers.live(el, v.x, v.y);
        }, { signal });
        root.addEventListener('pointermove', event => {
            if (!drag || event.pointerId !== drag.id) return;
            const v = fromPointer(event.clientX, event.clientY, drag.el.getBoundingClientRect());
            handlers.live(drag.el, v.x, v.y);
        }, { signal });
        const end = event => {
            if (!drag || (event.pointerId != null && event.pointerId !== drag.id)) return;
            const el = drag.el;
            drag = null;
            handlers.commit(el, event.type === 'pointercancel');
        };
        root.addEventListener('pointerup', end, { signal });
        root.addEventListener('pointercancel', end, { signal });
        root.addEventListener('lostpointercapture', end, { signal });
        root.addEventListener('dblclick', event => { const el = wheelOf(event.target); if (usable(el)) handlers.change(el, 0, 0); }, { signal });
        root.addEventListener('keydown', event => {
            const el = wheelOf(event.target);
            if (!usable(el) || event.target !== el || event.altKey || event.ctrlKey || event.metaKey) return;
            const v = keyStep(Number(el.dataset.wheelX) || 0, Number(el.dataset.wheelY) || 0, event.key, event.shiftKey);
            if (!v) return;
            event.preventDefault();
            event.stopPropagation();
            handlers.change(el, v.x, v.y);
        }, { signal });
    }

    window.VideoStudioColorWheel = { STEP, BIG_STEP, clampDisc, fromPointer, keyStep, polar, valueText, ringColor, markup, update, wire };
})();
