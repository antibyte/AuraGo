(function () {
    'use strict';
    // Front-panel parts of the RTL-SDR receiver: icons, the seven-segment
    // frequency readout and the knobs. Labels arrive translated from rtl-sdr.js.
    const ICONS = {
        antenna: '<circle cx="12" cy="12" r="1.9" fill="currentColor" stroke="none"/><path d="M8.3 8.3a5.2 5.2 0 0 0 0 7.4M15.7 8.3a5.2 5.2 0 0 1 0 7.4M5.4 5.4a9.3 9.3 0 0 0 0 13.2M18.6 5.4a9.3 9.3 0 0 1 0 13.2"/>',
        play: '<path d="M8 5.2v13.6L19 12z" fill="currentColor" stroke="none"/>',
        stop: '<rect x="6.5" y="6.5" width="11" height="11" rx="1.6" fill="currentColor" stroke="none"/>',
        speaker: '<path d="M4 9.5h3.4L12 6v12l-4.6-3.5H4z" fill="currentColor" stroke="none"/><path d="M15.4 9.2a4 4 0 0 1 0 5.6M18 6.6a7.6 7.6 0 0 1 0 10.8"/>',
        muted: '<path d="M4 9.5h3.4L12 6v12l-4.6-3.5H4z" fill="currentColor" stroke="none"/><path d="m16 9.5 5 5m0-5-5 5"/>',
        star: '<path d="m12 3.6 2.6 5.3 5.8.8-4.2 4.1 1 5.8-5.2-2.7-5.2 2.7 1-5.8-4.2-4.1 5.8-.8z"/>',
        starred: '<path d="m12 3.6 2.6 5.3 5.8.8-4.2 4.1 1 5.8-5.2-2.7-5.2 2.7 1-5.8-4.2-4.1 5.8-.8z" fill="currentColor"/>',
        setup: '<path d="M5 3.5v5.2M5 13v7.5M12 3.5v9.2M12 17v3.5M19 3.5v2.2M19 10v10.5"/><path d="M2.6 10.8h4.8M9.6 14.9h4.8M16.6 7.8h4.8"/>',
        lock: '<rect x="5.5" y="10.5" width="13" height="9.5" rx="2"/><path d="M8.5 10.5V8a3.5 3.5 0 0 1 7 0v2.5"/>',
        scan: '<circle cx="11" cy="11" r="6.5"/><path d="m16 16 4.5 4.5"/>',
        close: '<path d="m6.5 6.5 11 11m0-11-11 11"/>',
        left: '<path d="m14.5 6-6 6 6 6"/>',
        right: '<path d="m9.5 6 6 6-6 6"/>',
        tune: '<path d="M4.5 12h14m0 0-5-5m5 5-5 5"/>',
        record: '<circle cx="12" cy="12" r="5.5" fill="currentColor" stroke="none"/>',
        download: '<path d="M12 4v11m0 0-4.5-4.5M12 15l4.5-4.5M5 19.5h14"/>',
        trash: '<path d="M4.5 7h15M9.5 7V4.8h5V7M6.7 7l.8 12.2h9L17.3 7"/>'
    };
    // Capture keeps a drag alive outside the control; without it the drag still works inside.
    const capture = (el, e) => { try { el.setPointerCapture(e.pointerId); } catch (_) {} };
    const icon = (name, cls) => '<svg class="sdr-icon' + (cls ? ' ' + cls : '') + '" viewBox="0 0 24 24" aria-hidden="true" focusable="false" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">' + (ICONS[name] || '') + '</svg>';

    // Segment order a, b, c, d, e, f, g; one bit per segment in GLYPHS.
    const W = 52, H = 92, T = 9, G = 1.6;
    const bar = y => { const x1 = T / 2 + G, x2 = W - T / 2 - G, h = T / 2; return 'M' + x1 + ' ' + y + 'l' + h + ' ' + -h + 'H' + (x2 - h) + 'l' + h + ' ' + h + 'l' + -h + ' ' + h + 'H' + (x1 + h) + 'z'; };
    const post = (x, y1, y2) => { const h = T / 2; return 'M' + x + ' ' + y1 + 'l' + h + ' ' + h + 'V' + (y2 - h) + 'l' + -h + ' ' + h + 'l' + -h + ' ' + -h + 'V' + (y1 + h) + 'z'; };
    const SEGMENTS = [bar(T / 2), post(W - T / 2, T / 2 + G, H / 2 - G), post(W - T / 2, H / 2 + G, H - T / 2 - G), bar(H - T / 2), post(T / 2, H / 2 + G, H - T / 2 - G), post(T / 2, T / 2 + G, H / 2 - G), bar(H / 2)];
    const GLYPHS = { 0: 63, 1: 6, 2: 91, 3: 79, 4: 102, 5: 109, 6: 125, 7: 7, 8: 127, 9: 111 };
    const DIGITS = 10;
    const glyph = '<svg class="sdr-glyph" viewBox="0 0 ' + W + ' ' + H + '" aria-hidden="true" focusable="false">' + SEGMENTS.map(d => '<path d="' + d + '"/>').join('') + '</svg>';

    // Ten tunable digits, grouped MHz.kHz.Hz. Buttons are created once and only
    // repainted afterwards, so keyboard focus survives every frequency change.
    function frequency(target, hz, label) {
        if (!target.firstChild) {
            let html = '';
            for (let i = 0; i < DIGITS; i++) {
                const power = 10 ** (DIGITS - 1 - i);
                html += '<button type="button" class="sdr-digit" data-power="' + power + '" aria-label="' + label(power) + '">' + glyph + '</button>';
                if (i === 3 || i === 6) html += '<span class="sdr-point" aria-hidden="true"></span>';
            }
            target.innerHTML = html;
        }
        const text = String(Math.max(0, Math.round(hz))).padStart(7, '0').padStart(DIGITS, ' ');
        if (target.dataset.frequency === text) return;
        target.dataset.frequency = text;
        target.querySelectorAll('.sdr-digit').forEach((button, i) => {
            const char = text[i], bits = GLYPHS[char] || 0;
            if (button.dataset.digit === char) return;
            button.dataset.digit = char;
            button.classList.toggle('is-blank', char === ' ');
            button.querySelectorAll('path').forEach((path, bit) => path.classList.toggle('is-lit', !!(bits & 1 << bit)));
        });
    }

    // Bounded knob around a native range input. The input keeps keyboard and
    // assistive behaviour; the wrapper adds drag, wheel and double-click reset.
    // wheel(element) decides whether the wheel turns the control or scrolls the page.
    function knob(root, signal, wheel) {
        const input = root.querySelector('input[type="range"]');
        let drag = null, settle = 0;
        const paint = () => {
            const min = Number(input.min), max = Number(input.max), fill = max > min ? (Number(input.value) - min) / (max - min) : 0;
            root.style.setProperty('--sdr-fill', fill.toFixed(4));
            root.style.setProperty('--sdr-angle', (fill * 270 - 135).toFixed(1) + 'deg');
            root.classList.toggle('is-disabled', input.disabled);
        };
        const emit = type => input.dispatchEvent(new Event(type, { bubbles: true }));
        const nudge = steps => {
            if (input.disabled || !steps) return false;
            const before = input.value;
            if (steps > 0) input.stepUp(steps); else input.stepDown(-steps);
            if (input.value === before) return false;
            emit('input');
            return true;
        };
        const travel = () => { const count = (Number(input.max) - Number(input.min)) / (Number(input.step) || 1); return Math.max(1.2, 150 / Math.max(1, count)); };
        const on = (type, fn, options) => root.addEventListener(type, fn, Object.assign({ signal }, options));
        input.addEventListener('input', paint, { signal });
        on('pointerdown', e => {
            if (input.disabled || e.button) return;
            e.preventDefault();
            input.focus({ preventScroll: true });
            drag = { x: e.clientX, y: e.clientY, done: 0, moved: false };
            capture(root, e);
            root.classList.add('is-turning');
        });
        on('pointermove', e => {
            if (!drag) return;
            const steps = Math.trunc((drag.y - e.clientY + e.clientX - drag.x) / travel());
            if (nudge(steps - drag.done)) drag.moved = true;
            drag.done = steps;
        });
        const release = () => { if (!drag) return; const moved = drag.moved; drag = null; root.classList.remove('is-turning'); if (moved) emit('change'); };
        on('pointerup', release);
        on('pointercancel', release);
        on('wheel', e => {
            if (input.disabled || !wheel(root)) return;
            e.preventDefault();
            if (!nudge(e.deltaY < 0 ? 1 : -1)) return;
            clearTimeout(settle);
            settle = setTimeout(() => emit('change'), 250);
        }, { passive: false });
        on('dblclick', () => {
            if (input.disabled || input.dataset.reset === undefined || input.value === input.dataset.reset) return;
            input.value = input.dataset.reset;
            emit('input'); emit('change');
        });
        signal.addEventListener('abort', () => clearTimeout(settle));
        paint();
        return { input, paint };
    }

    // Endless tuning dial with detents: spinning the knob around its centre,
    // the wheel and the arrow keys all report whole steps.
    function dial(button, signal, wheel, handlers) {
        const DETENT = 9;
        let drag = null;
        const angle = e => { const r = button.getBoundingClientRect(); return Math.atan2(e.clientY - r.top - r.height / 2, e.clientX - r.left - r.width / 2) * 180 / Math.PI; };
        const on = (type, fn, options) => button.addEventListener(type, fn, Object.assign({ signal }, options));
        on('pointerdown', e => {
            if (button.disabled || e.button) return;
            drag = { last: angle(e), turned: 0, done: 0, moved: false };
            capture(button, e);
            button.classList.add('is-turning');
        });
        on('pointermove', e => {
            if (!drag) return;
            const now = angle(e);
            let delta = now - drag.last;
            if (delta > 180) delta -= 360; else if (delta < -180) delta += 360;
            drag.last = now; drag.turned += delta;
            const steps = Math.trunc(drag.turned / DETENT);
            if (steps === drag.done) return;
            handlers.step(steps - drag.done);
            drag.done = steps; drag.moved = true;
        });
        const release = () => { if (!drag) return; const moved = drag.moved; drag = null; button.classList.remove('is-turning'); if (moved) handlers.commit(); };
        on('pointerup', release);
        on('pointercancel', release);
        on('wheel', e => { if (button.disabled || !wheel(button)) return; e.preventDefault(); handlers.step(e.deltaY < 0 ? 1 : -1); handlers.commit(); }, { passive: false });
        on('keydown', e => {
            const steps = { ArrowUp: 1, ArrowRight: 1, ArrowDown: -1, ArrowLeft: -1, PageUp: 10, PageDown: -10 }[e.key];
            if (!steps && e.key !== 'Home' && e.key !== 'End') return;
            e.preventDefault();
            if (steps) handlers.step(steps); else handlers.edge(e.key === 'End');
            handlers.commit();
        });
        return { turn(steps) { button.style.setProperty('--sdr-angle', ((steps * DETENT % 360 + 360) % 360).toFixed(1) + 'deg'); } };
    }

    window.RTLSDRPanel = { icon, capture, frequency, knob, dial };
})();
