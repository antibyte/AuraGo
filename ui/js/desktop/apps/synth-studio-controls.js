(function () {
    'use strict';
    // Knobs and faders for Synth Studio. A range input stays in the DOM as the
    // single source of truth (value, events, keyboard, forms); this module draws
    // a 3D dial next to it and forwards drags/wheel to the input.
    const START = -135, END = 135, CX = 28, CY = 28, R = 22;
    const KNOB_SELECTOR = '[data-param],[data-master-param],[data-designer-param],[data-fx-param],[data-macro]';
    const clamp = (value, low, high) => Math.min(high, Math.max(low, value));
    const decimals = step => (String(step).split('.')[1] || '').length;

    function snap(value, spec) {
        const { min, max, step } = spec;
        if (!(step > 0)) return clamp(value, min, max);
        const n = Math.round((value - min) / step);
        return clamp(Number((min + n * step).toFixed(decimals(step))), min, max);
    }
    function fraction(value, spec) {
        const span = spec.max - spec.min;
        return span > 0 ? clamp((value - spec.min) / span, 0, 1) : 0;
    }
    // Where a pointer sits on a fader track, 0..1. The thumb centre travels
    // `length - thumb`; `invert` makes the far end the minimum (vertical faders).
    function trackFraction(pos, start, length, thumb, invert) {
        const f = clamp((pos - start - thumb / 2) / Math.max(1, length - thumb), 0, 1);
        return invert ? 1 - f : f;
    }
    function angle(f) { return START + (END - START) * f; }
    function polar(deg) {
        const rad = (deg - 90) * Math.PI / 180;
        return [CX + R * Math.cos(rad), CY + R * Math.sin(rad)];
    }
    function arcPath(f0, f1) {
        const a = Math.min(f0, f1), b = Math.max(f0, f1);
        if (b - a < 0.002) return '';
        const [x0, y0] = polar(angle(a)), [x1, y1] = polar(angle(b));
        const large = angle(b) - angle(a) > 180 ? 1 : 0;
        return 'M' + x0.toFixed(2) + ' ' + y0.toFixed(2) + 'A' + R + ' ' + R + ' 0 ' + large + ' 1 ' + x1.toFixed(2) + ' ' + y1.toFixed(2);
    }
    // delta is in pixels, positive = increase. `pixels` is the travel for the full range.
    function dragValue(start, delta, pixels, spec, fine) {
        const span = spec.max - spec.min;
        return snap(start + (delta / (pixels * (fine ? 4 : 1))) * span, spec);
    }
    function kindFor(input) {
        if (!input || input.type !== 'range' || input.dataset.ssPlain !== undefined) return null;
        if (input.dataset.ssKind) return input.dataset.ssKind;
        if (input.hasAttribute('data-track-volume')) return 'hfader';
        return input.matches(KNOB_SELECTOR) ? 'knob' : null;
    }

    const spec = input => ({ min: Number(input.min || 0), max: Number(input.max || 100), step: Number(input.step) || 1 });
    const dialOf = input => {
        const next = input.nextElementSibling;
        return next && next.classList.contains('ss-ctl-dial') ? next : null;
    };

    function paint(input, dial) {
        const s = spec(input), f = fraction(Number(input.value), s);
        dial.style.setProperty('--ss-f', f.toFixed(4));
        if (dial.classList.contains('ss-ctl-knob')) {
            dial.style.setProperty('--ss-a', angle(f).toFixed(1) + 'deg');
            const origin = s.min < 0 && s.max > 0 ? fraction(0, s) : 0;
            dial.querySelector('.ss-ctl-arc').setAttribute('d', arcPath(origin, f));
        }
    }

    function bind(input, dial, kind) {
        let drag = null, wheelTimer = 0;
        const horizontal = kind === 'hfader', vertical = kind === 'vfader';
        const thumb = () => (dial.querySelector('.ss-ctl-thumb') || {})[vertical ? 'offsetHeight' : 'offsetWidth'] || 14;
        const set = value => {
            const next = snap(value, spec(input));
            if (next === Number(input.value)) return;
            input.value = String(next);
            paint(input, dial);
            input.dispatchEvent(new Event('input', { bubbles: true }));
        };
        const fakeDown = pointerId => input.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, pointerId }));
        dial.addEventListener('pointerdown', event => {
            if (event.button !== 0 || input.disabled) return;
            event.preventDefault();
            try { dial.setPointerCapture(event.pointerId); } catch (_) { /* synthetic pointer ids */ }
            input.focus({ preventScroll: true });
            fakeDown(event.pointerId);
            if (horizontal || vertical) {
                const rect = dial.getBoundingClientRect(), s = spec(input);
                const f = horizontal
                    ? trackFraction(event.clientX, rect.left, rect.width, thumb(), false)
                    : trackFraction(event.clientY, rect.top, rect.height, thumb(), true);
                set(s.min + f * (s.max - s.min));
            }
            drag = { x: event.clientX, y: event.clientY, value: Number(input.value) };
            dial.classList.add('is-dragging');
        });
        dial.addEventListener('pointermove', event => {
            if (!drag) return;
            const delta = horizontal ? event.clientX - drag.x : drag.y - event.clientY;
            const pixels = horizontal ? Math.max(40, dial.clientWidth - thumb()) : vertical ? Math.max(40, dial.clientHeight - thumb()) : 160;
            set(dragValue(drag.value, delta, pixels, spec(input), event.shiftKey));
        });
        dial.addEventListener('pointerup', () => {
            if (!drag) return;
            drag = null;
            dial.classList.remove('is-dragging');
            input.dispatchEvent(new Event('change', { bubbles: true }));
        });
        dial.addEventListener('pointercancel', () => {
            drag = null;
            dial.classList.remove('is-dragging');
        });
        // The wheel only acts on the focused control, so scrolling a panel past a
        // knob never changes a sound by accident.
        dial.addEventListener('wheel', event => {
            if (document.activeElement !== input || input.disabled) return;
            event.preventDefault();
            if (!wheelTimer) fakeDown(1);
            const s = spec(input);
            set(Number(input.value) + (event.deltaY < 0 ? 1 : -1) * s.step * (event.shiftKey ? 1 : 5));
            clearTimeout(wheelTimer);
            wheelTimer = setTimeout(() => {
                wheelTimer = 0;
                input.dispatchEvent(new Event('change', { bubbles: true }));
                window.dispatchEvent(new PointerEvent('pointerup'));
            }, 250);
        }, { passive: false });
        // Double click resets: through the row's reset button when it has one,
        // otherwise to the control's declared default (the designer faders).
        dial.addEventListener('dblclick', () => {
            const reset = input.closest('label')?.querySelector('[data-reset-param]');
            if (reset) { if (!reset.disabled) reset.click(); return; }
            const fallback = Number(input.dataset.ssDefault);
            if (input.dataset.ssDefault === undefined || input.disabled || !Number.isFinite(fallback)) return;
            fakeDown(1);
            set(fallback);
            input.dispatchEvent(new Event('change', { bubbles: true }));
            window.dispatchEvent(new PointerEvent('pointerup'));
        });
    }

    function upgrade(input) {
        const kind = kindFor(input);
        if (!kind || input.dataset.ssCtl) return null;
        input.dataset.ssCtl = kind;
        input.classList.add('ss-ctl-native');
        const dial = document.createElement('span');
        dial.className = 'ss-ctl-dial ss-ctl-' + kind;
        dial.setAttribute('aria-hidden', 'true');
        dial.innerHTML = kind === 'knob'
            ? '<svg viewBox="0 0 56 56"><path class="ss-ctl-groove" d="' + arcPath(0, 1) + '"/><path class="ss-ctl-arc" d=""/></svg><span class="ss-ctl-cap"></span>'
            : '<span class="ss-ctl-groove-' + (kind === 'vfader' ? 'v' : 'h') + '"></span><span class="ss-ctl-fill"></span><span class="ss-ctl-thumb"></span>';
        input.after(dial);
        paint(input, dial);
        bind(input, dial, kind);
        return dial;
    }
    function upgradeAll(root) {
        const inputs = root.querySelectorAll('input[type=range]:not([data-ss-ctl])');
        inputs.forEach(upgrade);
        return inputs.length;
    }
    function sync(root) {
        root.querySelectorAll('input[data-ss-ctl]').forEach(input => {
            const dial = dialOf(input);
            if (dial) paint(input, dial);
        });
    }
    // The app re-renders whole panels. One observer upgrades whatever appears.
    function observe(root, signal) {
        upgradeAll(root);
        let queued = false;
        const observer = new MutationObserver(() => {
            if (queued) return;
            queued = true;
            queueMicrotask(() => { queued = false; if (upgradeAll(root) > 0) sync(root); });
        });
        observer.observe(root, { childList: true, subtree: true });
        if (signal && typeof signal.addEventListener === 'function') signal.addEventListener('abort', () => observer.disconnect(), { once: true });
        return observer;
    }

    window.SynthStudioControls = { KNOB_SELECTOR, snap, fraction, trackFraction, angle, arcPath, dragValue, kindFor, upgrade, upgradeAll, sync, observe };
})();
