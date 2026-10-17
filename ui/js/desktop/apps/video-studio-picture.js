(function () {
    'use strict';

    // DOM-free colour and picture math of Video Studio (plan index §4.4a, §4.20).
    //  - It uses the same formulas and numbers as Go internal/videostudio/picture_math.go and the colour constants of
    //    effects.go.
    //  - The parity topics color, wheels and picture (TestVideoStudioEffectsParity) and
    //    TestVideoStudioPictureConstantsMatchGo keep the two equal.
    //  - Painters, the inspector and the colour wheels call in at call time; this file loads directly after
    //    video-studio-effects.js.
    // Plan 2c Task 4 adds parseCube and sampleLUT, Task 10 capSize and passUniforms.

    // Go ColorWheelK, ColorNeutralKelvin, colorTemperatureGain, colorTintGain and lumaBT709.
    const WHEEL_K = 0.2, NEUTRAL_K = 6500, TEMPERATURE_GAIN = 0.5, TINT_GAIN = 0.3;
    const LUMA = Object.freeze([0.2126, 0.7152, 0.0722]);
    // (cos φ, sin φ) of the channel angles 0°, 120° and 240° (Go wheelPhases).
    const PHASES = [[1, 0], [-0.5, Math.sqrt(3) / 2], [-0.5, -Math.sqrt(3) / 2]];
    // The largest share of one crop side, and of l + r and t + b (Go maxCropFraction).
    const CROP_MAX = 0.95;
    const num = (value, fallback) => (typeof value === 'number' && Number.isFinite(value) ? value : fallback);
    // Go clampUnit: 0..1, NaN becomes 0.
    const unit = value => (value > 0 ? (value > 1 ? 1 : value) : 0);
    // Go's math.Round: halves away from zero.
    const roundHalfAway = value => (value < 0 ? -Math.round(-value) : Math.round(value));
    const luma = rgb => LUMA[0] * rgb[0] + LUMA[1] * rgb[1] + LUMA[2] * rgb[2];

    // White-balance gains of a temperature (kelvin) and a tint (Go whiteBalance):
    //   (e^{0.5·s}, e^{−0.3·tint}, e^{−0.5·s}) with s = ln(T/6500), divided by their BT.709 luma.
    // 6500 K and tint 0 give [1, 1, 1]. Higher K warms; +tint is magenta.
    function whiteBalance(temperature, tint) {
        const s = Math.log(num(temperature, NEUTRAL_K) / NEUTRAL_K), t = num(tint, 0);
        const gains = [Math.exp(TEMPERATURE_GAIN * s), Math.exp(-TINT_GAIN * t), Math.exp(-TEMPERATURE_GAIN * s)];
        const weight = luma(gains);
        return gains.map(gain => gain / weight);
    }
    // The per-channel offsets of a wheel position (x, y): x·cos φ_c + y·sin φ_c, without WHEEL_K (Go wheelOffset).
    function wheel(x, y) {
        const a = num(x, 0), b = num(y, 0);
        return PHASES.map(phase => a * phase[0] + b * phase[1]);
    }
    // Steps 1–5 of the colour formula per channel, plus the saturation (Go colorCurves):
    //   mul = 2^exposure · wb; lift and gain = master + WHEEL_K · wheel; invGamma = 1 / (gamma · e^{WHEEL_K · wheel}).
    // values holds the colour parameters by id (the clipState effect values); a missing one takes its registry
    // default.
    function colorCoefficients(values) {
        const v = values || {};
        const wb = whiteBalance(v.temperature, v.tint), exposure = Math.pow(2, num(v.exposure, 0));
        const lift = wheel(v.lift_x, v.lift_y), gamma = wheel(v.gamma_x, v.gamma_y), gain = wheel(v.gain_x, v.gain_y);
        const liftMaster = num(v.lift, 0), gammaMaster = num(v.gamma, 1), gainMaster = num(v.gain, 1);
        return {
            mul: wb.map(g => exposure * g), contrast: num(v.contrast, 1),
            lift: lift.map(w => liftMaster + WHEEL_K * w), gain: gain.map(w => gainMaster + WHEEL_K * w),
            invGamma: gamma.map(w => 1 / (gammaMaster * Math.exp(WHEEL_K * w))), saturation: num(v.saturation, 1)
        };
    }
    // The colour formula in float for one RGB triple in 0..1 (Go applyColor).
    function applyColor(values, rgb) {
        const c = colorCoefficients(values), input = Array.isArray(rgb) ? rgb : [];
        const x = [0, 1, 2].map(i => {
            const v = unit((num(input[i], 0) * c.mul[i] - 0.5) * c.contrast + 0.5);
            return Math.pow(unit((v * (1 - c.lift[i]) + c.lift[i]) * c.gain[i]), c.invGamma[i]);
        });
        const y = luma(x);
        return x.map(v => unit(y + c.saturation * (v - y)));
    }
    // Step 6 as the colorchannelmixer coefficients rr, rg, rb, gr, gg, gb, br, bg, bb (Go saturationMatrix).
    function saturationMatrix(s) {
        const k = num(s, 1), out = [];
        for (let row = 0; row < 3; row++) for (let col = 0; col < 3; col++) out.push((1 - k) * LUMA[col] + (row === col ? k : 0));
        return out;
    }
    // A crop {l, t, r, b} in pixels of a width × height picture (Go cropRect):
    //   r' = min(r, 0.95 − l), b' = min(b, 0.95 − t);
    //   x = min(2·round(l·W/2), max(0, W − 2)), w = min(W − x, max(2, W − x − 2·round(r'·W/2))); y and h likewise.
    // The rect stays inside the picture, so a 1 × 1 picture gives x 0 and w 1 (Go cropRect does the same).
    function cropRect(crop, width, height) {
        const c = crop || {}, W = Math.round(num(width, 0)), H = Math.round(num(height, 0));
        const l = num(c.l, 0), t = num(c.t, 0), r = Math.min(num(c.r, 0), CROP_MAX - l), b = Math.min(num(c.b, 0), CROP_MAX - t);
        const x = Math.min(2 * roundHalfAway(l * W / 2), Math.max(0, W - 2)), y = Math.min(2 * roundHalfAway(t * H / 2), Math.max(0, H - 2));
        return { x, y, w: Math.min(W - x, Math.max(2, W - x - 2 * roundHalfAway(r * W / 2))), h: Math.min(H - y, Math.max(2, H - y - 2 * roundHalfAway(b * H / 2))) };
    }
    // The crop of a flipped picture: flip_h swaps l and r, flip_v swaps t and b (Go mirroredCrop).
    function mirroredCrop(crop, flipH, flipV) {
        const c = crop || {}, l = num(c.l, 0), t = num(c.t, 0), r = num(c.r, 0), b = num(c.b, 0);
        return { l: flipH ? r : l, t: flipV ? b : t, r: flipH ? l : r, b: flipV ? t : b };
    }
    // The vignette angle in radians: min(π/2, amount·(π/2)·R/(size·H)), with R = hypot(W/2, H/2) (Go vignetteAngle).
    function vignetteAngle(amount, size, width, height) {
        const W = num(width, 0), H = num(height, 0), s = num(size, 1);
        if (!(H > 0) || !(s > 0)) return 0;
        return Math.min(Math.PI / 2, num(amount, 0) * (Math.PI / 2) * Math.hypot(W / 2, H / 2) / (s * H));
    }
    // The Gaussian sigma in pixels of a blur radius, a share of the picture height (Go blurSigma).
    function blurSigma(radius, height) { return num(radius, 0) * num(height, 0); }

    window.VideoStudioPicture = { WHEEL_K, NEUTRAL_K, TEMPERATURE_GAIN, TINT_GAIN, LUMA, whiteBalance, wheel, colorCoefficients, applyColor, saturationMatrix, cropRect, mirroredCrop, vignetteAngle, blurSigma };
})();
