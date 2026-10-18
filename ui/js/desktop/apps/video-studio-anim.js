(function () {
    'use strict';

    // DOM-free in/out animation math of Video Studio (spec §5.6, plan index §4.4b, §4.24).
    //  - It computes the same numbers as Go internal/videostudio/anim.go. The parity topic anim
    //    (TestVideoStudioEffectsParity) and TestVideoStudioAnimConstantsMatchGo keep the two equal.
    //  - Spans and progress are timeline frames from the clip's edges; speed and ramps do not change them. The span on
    //    each side is the stored duration, at most half the clip (floor), so the two never overlap.
    //  - p is the rest progress: 1 at rest, 0 on the clip's first frame (in) and last frame (out), which are hidden.
    //  - The animation applies on top of the keyed values: opacity multiplies, the offset moves the box centre and the
    //    scale sizes the box about its centre. It never changes a keyed value.
    // This file loads directly after video-studio-picture.js (index §5). It reads VideoStudioTime and
    // VideoStudioEffects at call time; effects.js, the painters and time.js call it at call time.

    // Go AnimMinSeconds, AnimMaxSeconds and AnimDefaultSeconds.
    const SECONDS = Object.freeze({ min: 0.2, max: 3, default: 0.5 });
    // Go animSlideDistance, animBlurRadius and animBlurNode.
    const SLIDE = 0.15, BLUR_RADIUS = 0.02, BLUR_NODE = 0.75;
    // Go animPopFrom, animPopPeak, animPopPeakAt and animPopFadeUntil, plus the curve's spans animPopRise (peak - from),
    // animPopSettle (peak - 1) and animPopTail (1 - peakAt). The spans are written out as in Go: a JS subtraction such as
    // 1.08 - 0.6 gives a neighbouring double, and the box rounding must see the export's numbers.
    const POP = Object.freeze({ from: 0.6, peak: 1.08, peakAt: 0.7, fadeUntil: 0.3, rise: 0.48, settle: 0.08, tail: 0.3 });
    // The slide presets' unit directions in canvas axes, x right and y down (Go animSlideAxes).
    const SLIDES = Object.freeze({ slide_up: [0, -1], slide_down: [0, 1], slide_left: [-1, 0], slide_right: [1, 0] });
    const isObject = value => !!value && typeof value === 'object' && !Array.isArray(value);
    // Go's math.Round: halves away from zero.
    const roundHalfAway = value => (value < 0 ? -Math.round(-value) : Math.round(value));
    // e_out(t) = 1 - (1 - t)^3 in Go's order of operations (animEaseOut).
    function easeOut(t) { const u = 1 - t; return 1 - u * u * u; }

    // The frame range of an animation length at the project's rate (Go AnimFrameRange): max(1, round(0.2*fps)) to
    // round(3*fps), default round(0.5*fps), halves away from zero.
    function frameRange(project) {
        const fps = window.VideoStudioTime.rate(project);
        return { min: Math.max(1, roundHalfAway(SECONDS.min * fps)), max: roundHalfAway(SECONDS.max * fps), default: roundHalfAway(SECONDS.default * fps) };
    }
    // The effective span lengths {in, out} in timeline frames (Go animSpans): the stored duration, at most
    // floor(duration / 2); 0 without an animation on that side.
    function spans(clip) {
        const half = Math.floor(Math.max(0, Number(clip && clip.duration) || 0) / 2);
        const side = anim => (isObject(anim) && Number.isSafeInteger(anim.duration) && anim.duration > 0 ? Math.min(anim.duration, half) : 0);
        return { in: side(clip && clip.anim_in), out: side(clip && clip.anim_out) };
    }
    // The pop scale at p (spike S3 form, Go animPopScale): 0.6 + 0.48*e_out(p / 0.7) below 0.7, then
    // 1.08 - 0.08*smoothstep((p - 0.7) / 0.3).
    function popScale(p) {
        if (p < POP.peakAt) return POP.from + POP.rise * easeOut(p / POP.peakAt);
        const u = (p - POP.peakAt) / POP.tail;
        return POP.peak - POP.settle * (u * u * (3 - 2 * u));
    }
    // The blur_in cascade's mask values {m1, m2} at progress p (Go animBlurMasks), integers 0..255: m1 mixes g(S) into
    // g(S/4) until 0.75, m2 then mixes the sharp picture in. At p = 1 both are 255.
    function blurMasks(p) {
        const u = Math.max(0, Math.min(1, Number(p) || 0));
        return { m1: Math.round(255 * Math.min(u / BLUR_NODE, 1)), m2: Math.round(255 * Math.max((u - BLUR_NODE) / (1 - BLUR_NODE), 0)) };
    }
    // One preset at rest progress p on side 'in' or 'out' (Go animPresetAt). An unknown preset keeps the rest values.
    function presetAt(preset, p, side) {
        const anim = { opacity: 1, dx: 0, dy: 0, scale: 1, blur: null, wipe: null }, axis = SLIDES[preset];
        if (preset === 'fade') anim.opacity = p;
        else if (axis) {
            // In arrives moving along the slide's direction, out leaves along it (sign = -1 in, +1 out).
            const sign = side === 'in' ? -1 : 1, shift = (1 - easeOut(p)) * SLIDE;
            anim.dx = axis[0] ? sign * axis[0] * shift : 0;
            anim.dy = axis[1] ? sign * axis[1] * shift : 0;
            anim.opacity = p;
        } else if (preset === 'pop') {
            anim.scale = popScale(p);
            anim.opacity = Math.min(1, p / POP.fadeUntil);
        } else if (preset === 'wipe') anim.wipe = { side, p };
        else if (preset === 'blur_in') { anim.blur = blurMasks(p); anim.opacity = p; }
        return anim;
    }
    // The in/out animation of a clip at timeline frame frame (index §4.4b, Go animAt). It is null at rest and outside
    // the clip, else {opacity, dx, dy, scale, blur: null | {m1, m2}, wipe: null | {side, p}}. With j = frame - start:
    //  - the in span is j < D_in, with p = j / D_in;
    //  - the out span is j >= duration - D_out, with p = (duration - 1 - j) / D_out.
    // project belongs to the contract; the state does not depend on the rate.
    function at(clip, frame, project) {
        const duration = Number(clip && clip.duration) || 0, j = Number(frame) - (Number(clip && clip.start) || 0), s = spans(clip);
        if (!(j >= 0 && j < duration)) return null;
        if (j < s.in) return presetAt(clip.anim_in.preset, j / s.in, 'in');
        if (j >= duration - s.out) return presetAt(clip.anim_out.preset, (duration - 1 - j) / s.out, 'out');
        return null;
    }
    // The exact box of index §4.24 in project px (Go animBox). rest is the keyed clip box {x, y, w, h} (clipBox):
    //  - the size becomes max(2, 2*round(w*scale/2)), and h likewise, about the box centre;
    //  - the centre moves by 2*round(dx*W/2) and 2*round(dy*H/2) of the canvas {width, height} (project).
    // Without anim it is rest. The box may leave the canvas.
    function box(rest, anim, project) {
        const b = { x: Number(rest.x), y: Number(rest.y), w: Number(rest.w), h: Number(rest.h) };
        if (!anim) return b;
        const cw = Number(project && project.width) || 1280, ch = Number(project && project.height) || 720;
        const s = Number.isFinite(anim.scale) ? anim.scale : 1, dx = Number(anim.dx) || 0, dy = Number(anim.dy) || 0;
        const w = Math.max(2, 2 * roundHalfAway(b.w * s / 2)), h = Math.max(2, 2 * roundHalfAway(b.h * s / 2));
        const cx = b.x + b.w / 2 + 2 * roundHalfAway(dx * cw / 2), cy = b.y + b.h / 2 + 2 * roundHalfAway(dy * ch / 2);
        return { x: cx - w / 2, y: cy - h / 2, w, h };
    }
    // The visible columns [x0, x1) of a box width px wide under a wipe (Go animRevealPx), with R = round(p*width):
    // in shows [0, R) and out shows [width - R, width), so both read left to right. Without a wipe it is the whole box.
    function revealPx(anim, width) {
        const w = Math.max(0, Math.round(Number(width) || 0)), wipe = anim && anim.wipe;
        if (!wipe) return [0, w];
        const r = roundHalfAway((Number(wipe.p) || 0) * w);
        return wipe.side === 'out' ? [w - r, w] : [0, r];
    }
    // Mirror of Go validateAnim (index §4.1). anim_in and anim_out are absent or null, or objects with:
    //  - a registry preset (an unavailable one validates; only the export refuses it);
    //  - a whole duration inside frameRange(project).
    // Audio clips (and a clip without a track) have none. The duration is never checked against the clip's length.
    function valid(clip, track, project) {
        const sides = clip ? [clip.anim_in, clip.anim_out].filter(side => side != null) : [];
        if (!sides.length) return !!clip;
        if (!track || track.kind === 'audio') return false;
        const presets = window.VideoStudioEffects.registry().anim_presets, range = frameRange(project);
        return sides.every(side => isObject(side) && typeof side.preset === 'string' && presets.some(spec => spec.id === side.preset) &&
            Number.isSafeInteger(side.duration) && side.duration >= range.min && side.duration <= range.max);
    }
    // Frame-rate change (VideoStudioTime.rescaleProject): the animation at ratio = new rate / old rate. The duration
    // becomes round(duration·ratio), halves away from zero, clamped into frameRange(project) of the new rate; the preset
    // stays.
    function rescale(anim, ratio, project) {
        const range = frameRange(project);
        return { preset: anim.preset, duration: Math.max(range.min, Math.min(range.max, roundHalfAway((Number(anim.duration) || 0) * ratio))) };
    }

    window.VideoStudioAnim = { SECONDS, SLIDE, POP, BLUR_RADIUS, BLUR_NODE, frameRange, spans, at, blurMasks, box, revealPx, valid, rescale };
})();
