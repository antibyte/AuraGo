(function () {
    'use strict';

    // On-canvas mask handles (plan 2d Task 15; spec §8.1; plan index §4.22). Mask mode swaps the transform box for the
    // mask overlay, like crop mode (video-studio-stage.js), and each mode ends the other.
    // - P.setMaskMode(s, on, options) needs a mask on the clip the transform box would show. That clip comes from
    //   P.transformTarget: the selected visual clip, titles included, on an unlocked visible track, with the playhead
    //   inside it. The narrow inspector drawer closes so the mask can take focus.
    // - P.wireMaskBox builds [data-mask-box] once in .vs-preview-mat. P.updateMaskBox (called by updateTransform) keeps it
    //   on the clip box at the playhead, turned with the clip, and ends the mode when the clip or its mask goes away.
    //   [data-mask-shape] (value rect or ellipse) is the mask at its unrotated place in percent of the box, turned about
    //   its centre.
    // - The shape's [data-mask-handle] buttons:
    //   - eight size handles; the opposite edge stays where it is in mask space;
    //   - "rotate", ROTATE_KNOB px above the top edge;
    //   - "feather", F/2 below the bottom edge (at least FEATHER_KNOB px); a drag by d widens the ramp F by 2d.
    //   Dragging inside the shape moves the mask.
    // - A drag moves only the overlay. pointerup commits once through VideoStudioKeyframeEdit.setMask with the values the
    //   gesture changed, so animated mask paths get a key at the playhead.
    // - P.handleMaskKeys (handleKeys, after P.handleCropKeys):
    //   - the arrows move the mask by 1 % of the box (Shift 10 %), or a focused size handle's edges in mask space;
    //   - on the rotate knob they turn it by 1° (Shift 15°), on the feather knob they change it by 0.001 (Shift 0.01);
    //     Right and Up increase;
    //   - Enter and Escape leave, and Delete and Backspace stay here.
    // The overlay is DOM, so it also shows over the 2D painter, which draws no mask. maskGeometry, maskDrag,
    // maskDragDelta and maskNudge are DOM-free and Node-tested.
    const P = window.VideoStudioParts;
    const tr = (...args) => P.tr(...args);
    const esc = (...args) => P.esc(...args);
    const FX = () => window.VideoStudioEffects;
    const MASK_HANDLES = [
        ['nw', 'maskHandle_nw', 'Top left corner of the mask'], ['n', 'maskHandle_n', 'Top edge of the mask'], ['ne', 'maskHandle_ne', 'Top right corner of the mask'],
        ['e', 'maskHandle_e', 'Right edge of the mask'], ['se', 'maskHandle_se', 'Bottom right corner of the mask'], ['s', 'maskHandle_s', 'Bottom edge of the mask'],
        ['sw', 'maskHandle_sw', 'Bottom left corner of the mask'], ['w', 'maskHandle_w', 'Left edge of the mask'],
        ['rotate', 'maskHandle_rotate', 'Rotate mask'], ['feather', 'maskHandle_feather', 'Soften edge']
    ];
    const SIZE_HANDLES = new Set(['nw', 'n', 'ne', 'e', 'se', 's', 'sw', 'w']);
    // The registry ranges of the mask.* rows (plan index §4.2); NUMBERS are the six keyframable values.
    const RANGES = Object.freeze({ x: [-1, 1], y: [-1, 1], w: [0.01, 2], h: [0.01, 2], rotation: [-360, 360], feather: [0, 0.5] });
    const NUMBERS = ['x', 'y', 'w', 'h', 'rotation', 'feather'];
    const ARROWS = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] };
    // Screen px from the top edge to the rotate knob, and the feather knob's smallest distance below the bottom edge.
    const ROTATE_KNOB = 24, FEATHER_KNOB = 12;
    const DEG = Math.PI / 180;
    const PERCENT_TEXT = 'Mask: x {{x}} %, y {{y}} %, width {{w}} %, height {{h}} %, rotation {{r}}°, feather {{f}} %';
    const HINT_TEXT = 'Drag inside to move the mask, a corner or an edge to resize it, the round handle to rotate it and the outer handle to soften its edge. The arrow keys move the mask, or the focused handle (Shift: ten times as far). Enter or Esc finishes.';
    // Rounding to 1e-6 keeps the fixed edge of a size drag within a millionth of the box; adding 0 turns -0 into 0.
    const round6 = value => Math.round(value * 1e6) / 1e6 + 0;
    const within = (value, range) => Math.max(range[0], Math.min(range[1], value));

    // The six numbers of a mask (a clipState mask or clip.mask); a missing one takes its registry default.
    function maskValues(mask) {
        const m = mask || {}, out = {};
        NUMBERS.forEach(key => {
            const value = Number(m[key]), spec = FX().paramSpec(null, 'mask.' + key);
            out[key] = m[key] != null && Number.isFinite(value) ? value : spec && Number.isFinite(Number(spec.default)) ? Number(spec.default) : 0;
        });
        return out;
    }
    // Clamps the six numbers to the registry ranges and rounds them.
    function settled(values) {
        const out = {};
        NUMBERS.forEach(key => { out[key] = round6(within(Number(values[key]) || 0, RANGES[key])); });
        return out;
    }
    // The overlay geometry for a clip state with a mask (clipState at the playhead), in project pixels:
    // - box is the export's clip box (VideoStudioEffects.placement), and angle the clip rotation (radians, clockwise
    //   on screen);
    // - in box pixels, cx/cy is the mask centre, rx/ry its half sizes, theta its rotation (radians), and ramp the
    //   full edge ramp F = max(feather·H, 1) of law L (plan index §4.22);
    // - mask holds the six numbers plus shape and invert.
    function maskGeometry(state, project) {
        const size = { width: Number(project && project.width) || 1280, height: Number(project && project.height) || 720 };
        const place = FX().placement(state, size, size, null), box = place.box, raw = (state && state.mask) || {}, m = maskValues(raw);
        return {
            box, angle: place.angle, mask: Object.assign({ shape: raw.shape === 'ellipse' ? 'ellipse' : 'rect', invert: !!raw.invert }, m),
            cx: (m.x + m.w / 2) * box.w, cy: (m.y + m.h / 2) * box.h, rx: m.w * box.w / 2, ry: m.h * box.h / 2,
            theta: m.rotation * DEG, ramp: Math.max(m.feather * box.h, 1)
        };
    }
    // Screen pixels of a drag (already scaled to project pixels) to box pixels: the delta turns back by the clip's
    // rotation.
    function maskDragDelta(dxPx, dyPx, angle) {
        const cos = Math.cos(Number(angle) || 0), sin = Math.sin(Number(angle) || 0), x = Number(dxPx) || 0, y = Number(dyPx) || 0;
        return { dx: x * cos + y * sin, dy: -x * sin + y * cos };
    }
    // A drag of the mask ('move'), a size handle ('nw' … 'w'), the rotate knob or the feather knob.
    // - dx and dy are box pixels in box directions (maskDragDelta has undone the clip rotation).
    // - box is {w, h} in the same pixels, plus scale (screen px per box px) for the knob distances; scale is 1 when
    //   missing.
    // - Returns the six numbers, clamped to the registry ranges and rounded to 1e-6.
    function maskDrag(mask, handle, dx, dy, box) {
        const m = maskValues(mask), out = Object.assign({}, m), x = Number(dx) || 0, y = Number(dy) || 0;
        const W = Math.max(1e-9, Number(box && box.w) || 0), H = Math.max(1e-9, Number(box && box.h) || 0), scale = Number(box && box.scale) > 0 ? Number(box.scale) : 1;
        const t = m.rotation * DEG, cos = Math.cos(t), sin = Math.sin(t), lx = x * cos + y * sin, ly = -x * sin + y * cos;
        if (handle === 'move') { out.x = m.x + x / W; out.y = m.y + y / H; }
        else if (SIZE_HANDLES.has(handle)) {
            // The dragged edges move by the delta in mask space, and the centre follows by half of that.
            const rw = m.w * W, rh = m.h * H;
            let nw = rw, nh = rh, sx = 0, sy = 0;
            if (/[ew]/.test(handle)) { const sign = handle.includes('e') ? 1 : -1; nw = within(rw + sign * lx, [RANGES.w[0] * W, RANGES.w[1] * W]); sx = sign * (nw - rw) / 2; }
            if (/[ns]/.test(handle)) { const sign = handle.includes('s') ? 1 : -1; nh = within(rh + sign * ly, [RANGES.h[0] * H, RANGES.h[1] * H]); sy = sign * (nh - rh) / 2; }
            const cx = (m.x + m.w / 2) * W + sx * cos - sy * sin, cy = (m.y + m.h / 2) * H + sx * sin + sy * cos;
            out.w = nw / W; out.h = nh / H; out.x = cx / W - out.w / 2; out.y = cy / H - out.h / 2;
        } else if (handle === 'rotate') {
            // The knob starts at (0, -(ry + ROTATE_KNOB/scale)) in mask space. The mask turns by the pointer's angle
            // around its centre, the short way.
            const r = m.h * H / 2 + ROTATE_KNOB / scale, px = r * sin, py = -r * cos;
            const turn = ((Math.atan2(py + y, px + x) - Math.atan2(py, px)) + 3 * Math.PI) % (2 * Math.PI) - Math.PI;
            out.rotation = m.rotation + turn / DEG;
            if (out.rotation > 360) out.rotation -= 360; else if (out.rotation < -360) out.rotation += 360;
        } else if (handle === 'feather') out.feather = m.feather + 2 * ly / H;
        return settled(out);
    }
    // One arrow key on the mask ('move'), a size handle, the rotate knob or the feather knob; null for other keys.
    function maskNudge(mask, handle, key, shift, box) {
        const arrow = ARROWS[key];
        if (!arrow) return null;
        const m = maskValues(mask), out = Object.assign({}, m), more = arrow[0] > 0 || arrow[1] < 0 ? 1 : -1;
        if (handle === 'rotate') out.rotation = m.rotation + more * (shift ? 15 : 1);
        else if (handle === 'feather') out.feather = m.feather + more * (shift ? 0.01 : 0.001);
        else if (SIZE_HANDLES.has(handle)) {
            // 1 % (Shift 10 %) of the box per press along the mask's own axes, turned into box directions for maskDrag.
            const step = shift ? 0.1 : 0.01, t = m.rotation * DEG, lx = arrow[0] * step * (Number(box && box.w) || 0), ly = arrow[1] * step * (Number(box && box.h) || 0);
            return maskDrag(m, handle, lx * Math.cos(t) - ly * Math.sin(t), lx * Math.sin(t) + ly * Math.cos(t), box);
        } else { const step = shift ? 0.1 : 0.01; out.x = m.x + arrow[0] * step; out.y = m.y + arrow[1] * step; }
        return settled(out);
    }

    // The clip mask mode works on: the transform target, when it has a mask.
    function maskTarget(s) {
        const found = P.transformTarget(s);
        return found && found.clip.mask ? found : null;
    }
    // Places the overlay. The box goes in percent of the mat and the shape in percent of the box. values (a gesture's
    // live mask) replaces the geometry's mask numbers.
    function drawMaskBox(s, el, g, values) {
        const pw = Number(s.project.width) || 1280, ph = Number(s.project.height) || 720, pct = value => value * 100 + '%', m = values || g.mask;
        el.style.left = pct(g.box.x / pw); el.style.top = pct(g.box.y / ph); el.style.width = pct(g.box.w / pw); el.style.height = pct(g.box.h / ph);
        el.style.transform = g.angle ? `rotate(${g.angle}rad)` : '';
        const shape = el.querySelector('[data-mask-shape]'), mat = s.q('.vs-preview-mat'), scale = mat ? mat.getBoundingClientRect().height / ph : 1;
        shape.dataset.maskShape = g.mask.shape;
        shape.style.left = pct(m.x); shape.style.top = pct(m.y); shape.style.width = pct(m.w); shape.style.height = pct(m.h);
        shape.style.transform = m.rotation ? `rotate(${m.rotation}deg)` : '';
        // The feather knob marks the outer end of the soft edge: F/2 below the bottom edge, at least FEATHER_KNOB px.
        shape.querySelector('[data-mask-handle="feather"]').style.top = `calc(100% + ${Math.max(FEATHER_KNOB, m.feather * g.box.h / 2 * scale)}px)`;
        const locale = P.uiLocale(), percent = value => (Math.round(value * 1000) / 10).toLocaleString(locale);
        const vars = { x: percent(m.x), y: percent(m.y), w: percent(m.w), h: percent(m.h), r: (Math.round(m.rotation * 10) / 10).toLocaleString(locale), f: percent(m.feather) };
        shape.setAttribute('aria-label', Object.keys(vars).reduce((text, key) => text.split('{{' + key + '}}').join(vars[key]), tr(s, 'maskBoxLabel', PERCENT_TEXT)));
    }
    function syncMaskButton(s) {
        const button = s.q('[data-inspector] [data-action="mask-mode"]');
        if (button) button.setAttribute('aria-pressed', String(!!s.maskMode));
    }
    function updateMaskBox(s) {
        const el = s.q('[data-mask-box]');
        if (!el) return;
        const found = s.maskMode && s.project && !s.q('.vs-no-project') ? maskTarget(s) : null;
        if (s.maskMode && (!found || found.clip.id !== s.maskMode.clipId)) {
            // The clip left the stage (another selection, the playhead outside it, a lock, a removed mask, the source
            // monitor): the mode ends and keyboard focus stays in the editor.
            const focused = el.contains(document.activeElement);
            s.maskMode = null; s.maskDrag = null;
            el.classList.remove('is-dragging');
            syncMaskButton(s);
            if (focused && s.app) s.app.focus({ preventScroll: true });
        }
        el.hidden = !s.maskMode;
        if (!s.maskMode || s.maskDrag) return;
        drawMaskBox(s, el, maskGeometry(FX().clipState(found.clip, s.frame, s.project), s.project), null);
    }
    function setMaskMode(s, on, options) {
        const found = on ? maskTarget(s) : null;
        if (on && !found) { P.showNotice(s, 'maskModeOutside', 'Move the playhead into the selected clip to edit its mask in the preview.', false); return false; }
        // Crop mode and mask mode exclude each other.
        if (found && s.cropMode && P.setCropMode) P.setCropMode(s, false);
        // The narrow inspector drawer makes the preview inert; it closes so the mask can take focus.
        if (found && s.app.classList.contains('vs-show-inspector')) P.setInspectorOpen(s, false, false);
        s.maskMode = found ? { clipId: found.clip.id } : null;
        s.maskDrag = null;
        s.q('[data-mask-box]')?.classList.remove('is-dragging');
        P.updateTransform(s);
        syncMaskButton(s);
        if (s.maskMode) s.q('[data-mask-shape]')?.focus({ preventScroll: true });
        else if (options && options.restoreFocus) (s.q('[data-inspector] [data-action="mask-mode"]:not(:disabled)') || s.app).focus({ preventScroll: true });
        return true;
    }
    // One undo step with the values that changed; setMask keys animated mask paths at the playhead.
    function commitMask(s, clipId, before, after) {
        const values = {};
        NUMBERS.forEach(key => { if (Math.abs(after[key] - before[key]) > 1e-9) values[key] = after[key]; });
        if (!Object.keys(values).length) return false;
        return P.applyEdit(s, 'mask', project => window.VideoStudioKeyframeEdit.setMask(project, clipId, values, s.frame));
    }
    // Keys inside the mask overlay (handleKeys calls this after the crop keys).
    function handleMaskKeys(s, event) {
        if (!s.maskMode || typeof event.target.closest !== 'function' || !event.target.closest('[data-mask-box]')) return false;
        if (event.key === 'Escape' || event.key === 'Enter') { event.preventDefault(); event.stopPropagation(); setMaskMode(s, false, { restoreFocus: true }); return true; }
        if (event.key === 'Delete' || event.key === 'Backspace') { event.preventDefault(); return true; }
        if (!ARROWS[event.key] || event.altKey || event.ctrlKey || event.metaKey) return false;
        event.preventDefault(); event.stopPropagation();
        const found = maskTarget(s);
        if (!found || s.maskDrag) return true;
        const g = maskGeometry(FX().clipState(found.clip, s.frame, s.project), s.project), handle = event.target.closest('[data-mask-handle]');
        commitMask(s, found.clip.id, g.mask, maskNudge(g.mask, handle ? handle.dataset.maskHandle : 'move', event.key, event.shiftKey, g.box));
        return true;
    }
    function wireMaskBox(s) {
        const mat = s.q('.vs-preview-mat'), anchor = s.q('[data-crop-box]') || s.q('[data-transform]');
        if (!mat || !anchor || s.q('[data-mask-box]')) return;
        const label = (key, fallback) => esc(s, tr(s, key, fallback)), signal = s.abort.signal;
        const handles = MASK_HANDLES.map(([id, key, fallback]) => `<button type="button" class="${SIZE_HANDLES.has(id) ? 'vs-mask-handle' : 'vs-mask-knob'}" data-mask-handle="${id}" aria-label="${label(key, fallback)}" title="${label(key, fallback)}"></button>`).join('');
        anchor.insertAdjacentHTML('afterend', `<div class="vs-mask-box" data-mask-box role="group" aria-label="${label('maskMode', 'Edit mask in the preview')}" hidden><div class="vs-mask-shape" data-mask-shape="rect" tabindex="0" title="${label('maskBoxHint', HINT_TEXT)}">${handles}</div></div>`);
        const el = s.q('[data-mask-box]'), shape = el.querySelector('[data-mask-shape]');
        shape.addEventListener('pointerdown', event => {
            const found = s.maskMode ? maskTarget(s) : null;
            if (!found || event.button !== 0 || s.timelineDragging) return;
            const geometry = maskGeometry(FX().clipState(found.clip, s.frame, s.project), s.project), area = mat.getBoundingClientRect();
            const pw = Number(s.project.width) || 1280, ph = Number(s.project.height) || 720, handle = event.target.closest('[data-mask-handle]');
            s.maskDrag = {
                clipId: found.clip.id, handle: handle ? handle.dataset.maskHandle : 'move', x: event.clientX, y: event.clientY, geometry, values: geometry.mask,
                kx: pw / (area.width || pw), ky: ph / (area.height || ph), scale: (area.height || ph) / ph, epoch: s.projectEpoch
            };
            (handle || shape).focus({ preventScroll: true });
            try { shape.setPointerCapture(event.pointerId); } catch (_) { /* synthetic pointer */ }
            el.classList.add('is-dragging');
            event.preventDefault(); event.stopPropagation();
        }, { signal });
        shape.addEventListener('pointermove', event => {
            const drag = s.maskDrag;
            if (!drag) return;
            const delta = maskDragDelta((event.clientX - drag.x) * drag.kx, (event.clientY - drag.y) * drag.ky, drag.geometry.angle);
            drag.values = Object.assign({}, drag.geometry.mask, maskDrag(drag.geometry.mask, drag.handle, delta.dx, delta.dy, { w: drag.geometry.box.w, h: drag.geometry.box.h, scale: drag.scale }));
            drawMaskBox(s, el, drag.geometry, drag.values);
        }, { signal });
        const finish = event => {
            const drag = s.maskDrag;
            if (!drag) return;
            s.maskDrag = null;
            el.classList.remove('is-dragging');
            if (event.type !== 'pointercancel' && drag.epoch === s.projectEpoch) commitMask(s, drag.clipId, drag.geometry.mask, drag.values);
            updateMaskBox(s);
        };
        shape.addEventListener('pointerup', finish, { signal });
        shape.addEventListener('pointercancel', finish, { signal });
        shape.addEventListener('lostpointercapture', () => { if (s.maskDrag) finish({ type: 'pointercancel' }); }, { signal });
        window.addEventListener('blur', () => { if (s.maskDrag) finish({ type: 'pointercancel' }); }, { signal });
        // Clicks inside the overlay stay there: no clip picking under the picture (mat click) and no focus jump to the app.
        el.addEventListener('click', event => event.stopPropagation(), { signal });
    }

    Object.assign(P, { setMaskMode, updateMaskBox, wireMaskBox, handleMaskKeys, maskGeometry, maskDrag, maskDragDelta, maskNudge });
})();
