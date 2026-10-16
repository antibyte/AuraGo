(function () {
    'use strict';

    // DOM-free effect registry of the editor (spec §4.2, plan index §4.4). BUILTIN mirrors Go's
    // videostudio.EffectRegistry() JSON exactly (TestVideoStudioEffectsRegistryMatchesGo); at start the app adopts the
    // status endpoint's registry and its "unavailable" lists. Readers (timeline check, inspector, painters) call in at
    // call time, so this file loads directly after video-studio-time.js and before all of them. One registry per page:
    // every editor window talks to the same server.
    function deepFreeze(value) {
        if (value && typeof value === 'object' && !Object.isFrozen(value)) {
            Object.freeze(value);
            Object.values(value).forEach(deepFreeze);
        }
        return value;
    }

    const BUILTIN = deepFreeze({
        version: 1,
        base: [
            { id: 'x', type: 'number', min: 0, max: 1, default: 0, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'box_x' },
            { id: 'y', type: 'number', min: 0, max: 1, default: 0, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'box_y' },
            { id: 'width', type: 'number', min: 0, max: 1, default: 1, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'width' },
            { id: 'height', type: 'number', min: 0, max: 1, default: 1, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'height' },
            { id: 'rotation', type: 'number', min: -360, max: 360, default: 0, step: 0.1, keyframable: true, unit: 'deg', label_key: 'rotation' },
            { id: 'opacity', type: 'number', min: 0, max: 1, default: 1, step: 0.01, keyframable: true, label_key: 'opacity' },
            { id: 'fit', type: 'choice', min: 0, max: 0, default: 'contain', choices: ['contain', 'cover'], label_key: 'fit' }
        ],
        effects: [],
        transitions: [
            { id: 'dissolve', xfade: 'fade', category: 'basic', label_key: 'dissolve' },
            { id: 'black', xfade: 'fadeblack', category: 'basic', label_key: 'black' },
            { id: 'wipeleft', xfade: 'wiperight', category: 'wipe', label_key: 'wipeleft' },
            { id: 'wiperight', xfade: 'wipeleft', category: 'wipe', label_key: 'wiperight' }
        ],
        blend_modes: [],
        anim_presets: [],
        looks: []
    });

    const LISTS = ['base', 'effects', 'transitions', 'blend_modes', 'anim_presets', 'looks'];
    const UNAVAILABLE_LISTS = ['effects', 'transitions', 'blend_modes', 'anim_presets'];
    const isText = value => typeof value === 'string' && value !== '';
    const finite = value => typeof value === 'number' && Number.isFinite(value);
    const clamp01 = value => Math.max(0, Math.min(1, value));
    const emptyUnavailable = () => ({ effects: new Set(), transitions: new Set(), blend_modes: new Set(), anim_presets: new Set() });
    // Keyframe and speed-ramp names and limits (index §4.4, §4.18), the same as Go: the Easing constants
    // (keyframes.go), MaxKeyframesPerParam, MaxKeyframesPerClip, MaxRampPoints, MaxRampPieces and the unexported
    // rampEaseSteps (ramp.go).
    const EASINGS = Object.freeze(['linear', 'ease_in', 'ease_out', 'ease_in_out', 'hold']);
    const LIMITS = Object.freeze({ keyframesPerParam: 64, keyframesPerClip: 512, rampPoints: 16, rampPieces: 64, rampEaseSteps: 4 });
    // Go's MinClipSpeed and MaxClipSpeed.
    const MIN_SPEED = 0.25, MAX_SPEED = 16;
    let current = BUILTIN, missing = emptyUnavailable();

    // A registry from the server is used only when it has this build's version and every list is well formed.
    function validRegistry(value) {
        if (!value || typeof value !== 'object' || value.version !== BUILTIN.version) return false;
        if (!LISTS.every(key => Array.isArray(value[key]) && value[key].every(item => item && typeof item === 'object' && isText(item.id) && isText(item.label_key)))) return false;
        return value.transitions.every(item => isText(item.xfade)) && value.effects.every(item => item.params == null || Array.isArray(item.params));
    }
    // Takes the status endpoint's registry and unavailable lists. Invalid or missing input falls back to BUILTIN, and
    // an invalid unavailable object marks nothing (a server from before 2a sends neither). True when the registry was used.
    function adopt(registry, unavailable) {
        const ok = validRegistry(registry);
        current = ok ? deepFreeze(JSON.parse(JSON.stringify(registry))) : BUILTIN;
        missing = emptyUnavailable();
        if (unavailable && typeof unavailable === 'object') {
            UNAVAILABLE_LISTS.forEach(key => {
                if (Array.isArray(unavailable[key])) unavailable[key].filter(isText).forEach(id => missing[key].add(id));
            });
        }
        return ok;
    }
    function registry() { return current; }
    // Transitions in menu order; available is false when the server's FFmpeg lacks one. transition(id) returns the same
    // entry (or null).
    function transitions() { return current.transitions.map(spec => Object.assign({}, spec, { available: !missing.transitions.has(spec.id) })); }
    function transition(id) { return transitions().find(spec => spec.id === id) || null; }
    // A known registry transition, available or not (validation accepts unavailable ones; only the export refuses them).
    function isTransition(id) { return current.transitions.some(spec => spec.id === id); }
    function base(id) { return current.base.find(spec => spec.id === id) || null; }
    function effect(id) { return current.effects.find(spec => spec.id === id) || null; }
    function defaults(effectId) {
        const out = {};
        ((effect(effectId) || {}).params || []).forEach(param => { out[param.id] = param.default; });
        return out;
    }

    // Mirror of Go validateKeyframes and validateRamp (index §4.18).
    // Keyframes:
    //  - only on visual tracks, and only on keyframable number paths;
    //  - 1..64 per path and at most 512 per clip;
    //  - strictly increasing integer frames within ±16·maxFrames;
    //  - values inside the registry range (width and height above 0) and known easings.
    // Ramps:
    //  - only on a video track with a video asset, with speed absent or 1;
    //  - 2..16 points with strictly increasing integer frames within ±maxFrames;
    //  - speeds 0.25..16 and known easings.
    // The source extent rule stays in validTimeline.
    function validAnimation(clip, track, asset, maxFrames) {
        const keyframes = clip ? clip.keyframes : null, ramp = clip ? clip.ramp : null;
        if (keyframes != null) {
            if (typeof keyframes !== 'object' || Array.isArray(keyframes)) return false;
            const paths = Object.keys(keyframes);
            if (paths.length && (!track || track.kind === 'audio')) return false;
            if (paths.reduce((n, path) => n + (Array.isArray(keyframes[path]) ? keyframes[path].length : 0), 0) > LIMITS.keyframesPerClip) return false;
            const bound = MAX_SPEED * maxFrames;
            for (const path of paths) {
                const spec = base(path), list = keyframes[path];
                if (!spec || !spec.keyframable || spec.type !== 'number' || !Array.isArray(list) || !list.length || list.length > LIMITS.keyframesPerParam) return false;
                for (let i = 0; i < list.length; i++) {
                    const key = list[i];
                    if (!key || !Number.isSafeInteger(key.f) || Math.abs(key.f) > bound || (i > 0 && key.f <= list[i - 1].f)) return false;
                    if (!finite(key.v) || key.v < spec.min || key.v > spec.max || ((path === 'width' || path === 'height') && key.v <= 0) || !EASINGS.includes(key.e)) return false;
                }
            }
        }
        if (ramp != null && !Array.isArray(ramp)) return false;
        if (Array.isArray(ramp) && ramp.length) {
            if (!track || track.kind !== 'video' || !asset || asset.kind !== 'video') return false;
            if (ramp.length < 2 || ramp.length > LIMITS.rampPoints || (clip.speed != null && Number(clip.speed) !== 1)) return false;
            for (let i = 0; i < ramp.length; i++) {
                const point = ramp[i];
                if (!point || !Number.isSafeInteger(point.f) || Math.abs(point.f) > maxFrames || (i > 0 && point.f <= ramp[i - 1].f)) return false;
                if (!finite(point.speed) || point.speed < MIN_SPEED || point.speed > MAX_SPEED || !EASINGS.includes(point.e)) return false;
            }
        }
        return true;
    }

    // The value of a parameter path at an absolute timeline frame. Paths: x, y, width, height, rotation, opacity,
    // crop.l|t|r|b, mask.<param> and effects.<index>.<param> (the effect entry's params). 2a has no keyframes: the clip's
    // own number, else the registry default, else null. Plan 2b replaces the body with keyframe sampling; the signature
    // stays (frame and project are used from then on).
    function valueAt(clip, path, frame, project) {
        const parts = String(path || '').split('.');
        if (parts[0] === 'effects' && parts.length === 3) {
            const entry = clip && Array.isArray(clip.effects) ? clip.effects[Number(parts[1])] : null;
            const value = entry && entry.params ? entry.params[parts[2]] : undefined;
            if (finite(value)) return value;
            const param = entry ? ((effect(entry.id) || {}).params || []).find(item => item.id === parts[2]) : null;
            return param && finite(param.default) ? param.default : null;
        }
        const value = parts.reduce((node, key) => (node && typeof node === 'object' ? node[key] : undefined), clip);
        if (finite(value)) return value;
        const spec = base(path);
        return spec && finite(spec.default) ? spec.default : null;
    }
    // The resolved state of a clip at a frame, which both painters draw from (index §4.4). The shape stays fixed; later
    // slices change how values are evaluated (2b keyframes, 2c crop/flip/effects, 2d blend/mask, 2e animation).
    function clipState(clip, frame, project) {
        const at = path => valueAt(clip, path, frame, project);
        return {
            x: at('x'), y: at('y'), width: at('width'), height: at('height'), rotation: at('rotation'),
            opacity: clamp01(at('opacity')), fit: clip && clip.fit === 'cover' ? 'cover' : 'contain',
            crop: { l: 0, t: 0, r: 0, b: 0 }, flipH: false, flipV: false, blend: 'normal', mask: null, effects: []
        };
    }

    // Go's math.Round, which rounds halves away from zero (Math.round rounds them up).
    const roundHalfAway = value => (value < 0 ? -Math.round(-value) : Math.round(value));
    // evenPixels, evenPosition and clipBox mirror internal/videostudio/clip_chain.go: the export's clip box in project
    // pixels, even-sized and even-placed, at least 2 px and inside the canvas (parity fixture geometry.json).
    function evenPixels(value, bound) {
        const pixels = roundHalfAway(value / 2) * 2;
        return pixels < 2 ? 2 : Math.min(pixels, bound);
    }
    function evenPosition(value, box, bound) {
        const position = roundHalfAway(value / 2) * 2;
        return position <= 0 ? 0 : Math.min(position, bound - box);
    }
    function clipBox(state, width, height) {
        const w = evenPixels(Number(state.width) * width, width), h = evenPixels(Number(state.height) * height, height);
        return { x: evenPosition(Number(state.x) * width, w, width), y: evenPosition(Number(state.y) * height, h, height), w, h };
    }
    // FFmpeg's av_rescale for picture sizes: a * b / c rounded to the nearest integer (non-negative integers).
    const rescale = (a, b, c) => Math.floor((a * b + Math.floor(c / 2)) / c);
    // The size the export's scale filter gives the source in a box (force_original_aspect_ratio with
    // force_divisible_by=2, as documented): the largest even size inside it for contain, the smallest even size covering
    // it for cover. Without a source size the picture fills the box.
    function fittedSize(box, source, cover) {
        const sw = Math.round(Number(source && source.width) || 0), sh = Math.round(Number(source && source.height) || 0);
        if (sw <= 0 || sh <= 0) return { w: box.w, h: box.h };
        const tw = rescale(box.h, sw, sh), th = rescale(box.w, sh, sw);
        if (cover) return { w: Math.ceil(Math.max(tw, box.w) / 2) * 2, h: Math.ceil(Math.max(th, box.h) / 2) * 2 };
        return { w: Math.floor(Math.min(tw, box.w) / 2) * 2, h: Math.floor(Math.min(th, box.h) / 2) * 2 };
    }
    // Geometry of one clip in canvas pixels (index §4.4). box is the export's clip box (clipBox in project pixels)
    // scaled to the canvas per axis; draw is where the source picture lands (contain: inside the box, centred; cover:
    // over the box, with clip true so the painter clips to box); angle is the rotation in radians, clockwise on screen,
    // around the box centre. state comes from clipState; canvas and project are {width, height}; source is the picture's
    // intrinsic {width, height} or null. For 4:2:0 video the export's centred crop may land one pixel off (chroma
    // alignment); SSIM absorbs that.
    function placement(state, canvas, project, source) {
        const pw = Number(project && project.width) || 1280, ph = Number(project && project.height) || 720;
        const kx = (Number(canvas && canvas.width) || pw) / pw, ky = (Number(canvas && canvas.height) || ph) / ph;
        const box = clipBox(state, pw, ph), cover = state.fit === 'cover', size = fittedSize(box, source, cover);
        // Centred like the export's pad ((ow-iw)/2) and crop defaults; box and picture sizes are even, so the offsets
        // are whole project pixels.
        const dx = box.x + (box.w - size.w) / 2, dy = box.y + (box.h - size.h) / 2;
        return {
            box: { x: box.x * kx, y: box.y * ky, w: box.w * kx, h: box.h * ky },
            draw: { x: dx * kx, y: dy * ky, w: size.w * kx, h: size.h * ky },
            clip: cover,
            angle: (Number(state.rotation) || 0) * Math.PI / 180
        };
    }

    // Fade through black (export: xfade fadeblack on the premultiplied track layers). Weights of the outgoing layer,
    // the incoming layer and opaque black at progress p = i / D; they sum to 1. Same formula and numbers as Go
    // fadeBlackWeights (internal/videostudio/effects.go, which documents the measured fit), parity fixture
    // testdata/parity/transitions.json. With S the smoothstep: out = (1 - p) * (1 - S(p; 0, 0.2)),
    // inc = p * S(p; 0, 0.8), black = 1 - out - inc.
    function fadeRamp(x, edge0, edge1) { return Math.max(0, Math.min(1, (x - edge0) / (edge1 - edge0))); }
    function fadeEase(x, edge0, edge1) { const t = fadeRamp(x, edge0, edge1); return t * t * (3 - 2 * t); }
    function fadeBlackWeights(p) {
        const t = Math.max(0, Math.min(1, Number(p) || 0));
        const out = (1 - t) * (1 - fadeEase(t, 0, 0.2)), inc = t * fadeEase(t, 0, 0.8);
        return { out, inc, black: 1 - out - inc };
    }
    // Wipes reveal the incoming layer across the full frame: the incoming layer shows in [left, left + reveal).
    // "wipeleft" reveals from the left edge (export xfade wiperight), "wiperight" from the right edge (xfade wipeleft).
    // Any other type has no edge (null).
    function wipeEdge(type, p, width) {
        if (type !== 'wipeleft' && type !== 'wiperight') return null;
        const w = Math.max(0, Number(width) || 0), reveal = w * Math.max(0, Math.min(1, Number(p) || 0));
        return { left: type === 'wipeleft' ? 0 : w - reveal, reveal };
    }

    window.VideoStudioEffects = { BUILTIN, EASINGS, LIMITS, adopt, registry, transitions, transition, isTransition, base, effect, defaults, validAnimation, valueAt, clipState, placement, fadeBlackWeights, wipeEdge };
})();
