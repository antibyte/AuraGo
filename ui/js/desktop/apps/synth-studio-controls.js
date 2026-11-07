(function () {
    'use strict';
    // Knobs and faders for Synth Studio. A range input stays in the DOM as the
    // single source of truth (value, events, keyboard, forms); this module draws
    // a 3D dial next to it and forwards drags/wheel to the input.
    const START = -135, END = 135, CX = 28, CY = 28, R = 22;
    const KNOB_SELECTOR = '[data-param],[data-master-param],[data-designer-param],[data-fx-param]';
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

    window.SynthStudioControls = { snap, fraction, angle, arcPath, dragValue, kindFor };
})();
