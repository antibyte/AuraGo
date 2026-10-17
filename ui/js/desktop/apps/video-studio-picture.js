(function () {
    'use strict';

    // DOM-free colour and picture math of Video Studio (plan index §4.4a, §4.20).
    //  - It uses the same formulas and numbers as Go internal/videostudio/picture_math.go and the colour constants of
    //    effects.go.
    //  - The parity topics color, wheels and picture (TestVideoStudioEffectsParity) and
    //    TestVideoStudioPictureConstantsMatchGo keep the two equal.
    //  - Painters, the inspector and the colour wheels call in at call time; this file loads directly after
    //    video-studio-effects.js.
    // parseCube, sampleLUT and unitDomain mirror the Go LUT parser and sampler (plan 2c Task 4); Task 10 adds capSize and
    // passUniforms.

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

    // ---- .cube LUTs (index §4.20): the mirror of Go ParseCube and (*LUT3D).Sample (parity topic "lut") ----
    const LUT_MAX_BYTES = 16 * 1024 * 1024;
    const LUT_MIN_GRID = 2, LUT_MAX_GRID = 65, LUT_MAX_VALUE = 16;
    const CUBE_LINE_BYTES = 1024, CUBE_TITLE_BYTES = 256;
    const CUBE_NUMBER = /^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/;
    const CUBE_INTEGER = /^[0-9]+$/;

    // cubeError is what parseCube throws: an Error carrying the 1-based line and the reason, like Go's LUTError.
    function cubeError(line, reason) {
        const error = new Error('LUT line ' + line + ': ' + reason);
        error.line = line;
        error.reason = reason;
        return error;
    }

    // utf8Length counts UTF-8 bytes, so the line and title limits count bytes as Go does.
    function utf8Length(text) {
        let bytes = 0;
        for (let i = 0; i < text.length; i++) {
            const code = text.charCodeAt(i);
            if (code < 0x80) bytes += 1;
            else if (code < 0x800) bytes += 2;
            else if (code >= 0xD800 && code <= 0xDBFF && i + 1 < text.length && text.charCodeAt(i + 1) >= 0xDC00 && text.charCodeAt(i + 1) <= 0xDFFF) { bytes += 4; i++; }
            else bytes += 3;
        }
        return bytes;
    }

    // cubeNumbers reads exactly count decimal numbers within ±16 (Go cubeNumbers).
    function cubeNumbers(line, fields, count) {
        if (fields.length !== count) throw cubeError(line, 'expected ' + count + ' numbers');
        const values = new Array(count);
        for (let i = 0; i < count; i++) {
            if (!CUBE_NUMBER.test(fields[i])) throw cubeError(line, 'invalid number');
            const value = Number(fields[i]);
            if (!Number.isFinite(value) || Math.abs(value) > LUT_MAX_VALUE) throw cubeError(line, 'number outside -16..16');
            values[i] = value;
        }
        return values;
    }

    // parseCube parses .cube text into {size, domainMin, domainMax, data}. data is a Float32Array of size³ RGB triplets,
    // red fastest. It throws cubeError(line, reason) on the same line as Go's ParseCube.
    function parseCube(text) {
        if (typeof text !== 'string') throw cubeError(1, 'the file is not text');
        const source = text.charCodeAt(0) === 0xFEFF ? text.slice(1) : text;
        const lines = source.split('\n');
        if (lines[lines.length - 1] === '') lines.pop();
        const domainMin = [0, 0, 0], domainMax = [1, 1, 1];
        let size = 0, expected = 0, count = 0, data = null, bytes = 0;
        let title = false, minSet = false, maxSet = false, domainLine = 0, domainChecked = false;
        const checkDomain = () => {
            domainChecked = true;
            for (let c = 0; c < 3; c++) {
                if (!(domainMin[c] < domainMax[c])) throw cubeError(domainLine, 'DOMAIN_MIN must be below DOMAIN_MAX');
            }
        };
        for (let i = 0; i < lines.length; i++) {
            const number = i + 1;
            let line = lines[i];
            bytes += utf8Length(line) + 1;
            if (bytes > LUT_MAX_BYTES + 1) throw cubeError(number, 'the file is larger than 16 MiB');
            if (line.endsWith('\r')) line = line.slice(0, -1);
            if (utf8Length(line) > CUBE_LINE_BYTES) throw cubeError(number, 'line longer than 1024 bytes');
            const trimmed = line.replace(/^[ \t]+|[ \t]+$/g, '');
            if (trimmed === '' || trimmed[0] === '#') continue;
            const fields = trimmed.split(/[ \t]+/);
            const keyword = fields[0];
            if (/^[A-Za-z]/.test(keyword)) {
                if (count > 0) throw cubeError(number, 'keyword after the data');
                if (keyword === 'TITLE') {
                    if (title) throw cubeError(number, 'TITLE appears twice');
                    title = true;
                    if (utf8Length(trimmed.slice(5).replace(/^[ \t]+/, '')) > CUBE_TITLE_BYTES) throw cubeError(number, 'TITLE longer than 256 bytes');
                } else if (keyword === 'LUT_3D_SIZE') {
                    if (size) throw cubeError(number, 'LUT_3D_SIZE appears twice');
                    if (fields.length !== 2 || !CUBE_INTEGER.test(fields[1])) throw cubeError(number, 'LUT_3D_SIZE needs one whole number');
                    const grid = Number(fields[1]);
                    if (grid < LUT_MIN_GRID || grid > LUT_MAX_GRID) throw cubeError(number, 'LUT_3D_SIZE must be 2 to 65');
                    size = grid;
                    expected = grid * grid * grid;
                    data = new Float32Array(expected * 3);
                } else if (keyword === 'DOMAIN_MIN' || keyword === 'DOMAIN_MAX') {
                    const isMin = keyword === 'DOMAIN_MIN';
                    if (isMin ? minSet : maxSet) throw cubeError(number, 'the input domain is given twice');
                    const values = cubeNumbers(number, fields.slice(1), 3);
                    const target = isMin ? domainMin : domainMax;
                    for (let c = 0; c < 3; c++) target[c] = values[c];
                    if (isMin) minSet = true; else maxSet = true;
                    domainLine = number;
                } else if (keyword === 'LUT_3D_INPUT_RANGE') {
                    if (minSet || maxSet) throw cubeError(number, 'the input domain is given twice');
                    const values = cubeNumbers(number, fields.slice(1), 2);
                    for (let c = 0; c < 3; c++) { domainMin[c] = values[0]; domainMax[c] = values[1]; }
                    minSet = maxSet = true;
                    domainLine = number;
                } else if (keyword === 'LUT_1D_SIZE' || keyword === 'LUT_1D_INPUT_RANGE') {
                    throw cubeError(number, '1D LUTs are not supported');
                } else {
                    throw cubeError(number, 'unknown keyword');
                }
                continue;
            }
            if (!size) throw cubeError(number, 'data before LUT_3D_SIZE');
            if (!domainChecked) checkDomain();
            if (count === expected) throw cubeError(number, 'more data lines than the grid holds');
            const values = cubeNumbers(number, fields, 3);
            data[count * 3] = values[0];
            data[count * 3 + 1] = values[1];
            data[count * 3 + 2] = values[2];
            count++;
        }
        const end = lines.length + 1;
        if (!size) throw cubeError(end, 'missing LUT_3D_SIZE');
        if (!domainChecked) checkDomain();
        if (count !== expected) throw cubeError(end, 'expected ' + expected + ' data lines, found ' + count);
        return { size, domainMin, domainMax, data };
    }

    // sampleLUT is (1 − k)·rgb + k·L(clamp_domain(rgb)) with trilinear L (Go (*LUT3D).Sample; NaN reads as the domain
    // minimum). intensity k defaults to 1 and is clamped to 0..1.
    function sampleLUT(lut, rgb, intensity) {
        let k = Number(intensity);
        if (intensity == null || !Number.isFinite(k)) k = 1;
        k = Math.min(1, Math.max(0, k));
        const n = lut.size, data = lut.data, base = [0, 0, 0], frac = [0, 0, 0];
        for (let c = 0; c < 3; c++) {
            const low = lut.domainMin[c], high = lut.domainMax[c];
            let v = rgb[c];
            if (!(v > low)) v = low; else if (v > high) v = high;
            const position = (v - low) / (high - low) * (n - 1);
            const index = Math.min(Math.floor(position), n - 2);
            base[c] = index;
            frac[c] = position - index;
        }
        const at = (r, g, b, c) => data[((b * n + g) * n + r) * 3 + c];
        const lerp = (a, b, t) => a + (b - a) * t;
        const r = base[0], g = base[1], b = base[2], out = [0, 0, 0];
        for (let c = 0; c < 3; c++) {
            const c00 = lerp(at(r, g, b, c), at(r + 1, g, b, c), frac[0]);
            const c10 = lerp(at(r, g + 1, b, c), at(r + 1, g + 1, b, c), frac[0]);
            const c01 = lerp(at(r, g, b + 1, c), at(r + 1, g, b + 1, c), frac[0]);
            const c11 = lerp(at(r, g + 1, b + 1, c), at(r + 1, g + 1, b + 1, c), frac[0]);
            const looked = lerp(lerp(c00, c10, frac[1]), lerp(c01, c11, frac[1]), frac[2]);
            // At full intensity the input does not enter (0 · NaN would turn a NaN input into NaN, while the domain
            // clamp reads it as the domain minimum).
            out[c] = k === 1 ? looked : (1 - k) * rgb[c] + k * looked;
        }
        return out;
    }

    // unitDomain is the table resampled onto [0, 1]³ on the same grid, the mirror of Go unitDomainLUT. Go stages such a
    // copy for FFmpeg (its lut3d ignores DOMAIN_MIN), and the GL preview uploads the same table (Task 10), so preview and
    // export sample one table. A table whose domain already is [0, 1]³ is returned as it is; data are parseCube's RGB
    // triplets.
    function unitDomain(lut) {
        const min = lut.domainMin || [0, 0, 0], max = lut.domainMax || [1, 1, 1];
        if ([0, 1, 2].every(c => min[c] === 0 && max[c] === 1)) return lut;
        const n = lut.size, step = 1 / (n - 1), data = new Float32Array(n * n * n * 3);
        let i = 0;
        for (let b = 0; b < n; b++) for (let g = 0; g < n; g++) for (let r = 0; r < n; r++) {
            const v = sampleLUT(lut, [r * step, g * step, b * step], 1);
            data[i++] = v[0]; data[i++] = v[1]; data[i++] = v[2];
        }
        return { size: n, domainMin: [0, 0, 0], domainMax: [1, 1, 1], data };
    }

    window.VideoStudioPicture = { WHEEL_K, NEUTRAL_K, TEMPERATURE_GAIN, TINT_GAIN, LUMA, whiteBalance, wheel, colorCoefficients, applyColor, saturationMatrix, cropRect, mirroredCrop, vignetteAngle, blurSigma, parseCube, sampleLUT, unitDomain };
})();
