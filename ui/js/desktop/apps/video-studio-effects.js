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

    // Easing curves (spec §5.2, index §4.18), the same numbers as Go Ease:
    //  - linear t; ease_in t³; ease_out 1 − (1 − t)³;
    //  - ease_in_out 4t³ below one half, 1 − (2 − 2t)³ / 2 from there;
    //  - hold 0 until t reaches 1.
    // t is clamped to 0..1; unknown names are linear.
    function ease(name, t) {
        const x = Math.max(0, Math.min(1, Number(t) || 0));
        switch (name) {
            case 'ease_in': return x * x * x;
            case 'ease_out': { const u = 1 - x; return 1 - u * u * u; }
            case 'ease_in_out': {
                if (x < 0.5) return 4 * x * x * x;
                const u = -2 * x + 2;
                return 1 - u * u * u / 2;
            }
            case 'hold': return x < 1 ? 0 : 1;
            default: return x;
        }
    }
    // The value of a keyframe list [{f, v, e}] at source position f (fractional), like Go sampleKeyframes. Before the
    // first key it is the first value, after the last key the last value, and between keys i and i + 1 the value eased
    // with key i's easing. null for an empty list.
    function sample(list, f) {
        if (!Array.isArray(list) || !list.length) return null;
        const x = Number(f), last = list[list.length - 1];
        if (!(x > list[0].f)) return list[0].v;
        if (x >= last.f) return last.v;
        let lo = 0, hi = list.length - 1; // list[lo].f <= x < list[hi].f
        while (hi - lo > 1) {
            const mid = (lo + hi) >> 1;
            if (list[mid].f <= x) lo = mid; else hi = mid;
        }
        const a = list[lo], b = list[hi];
        return a.v + (b.v - a.v) * ease(a.e, (x - a.f) / (b.f - a.f));
    }
    // The keyframable base parameters of the registry (2b: x, y, width, height, rotation, opacity), in registry order.
    function keyframable() { return current.base.filter(spec => spec.keyframable && spec.type === 'number').map(spec => spec.id); }
    // Speed ramps (spec §5.5, index §4.18), the same pieces as Go rampPieces.
    //  - A linear span is one piece, and a hold span one constant piece.
    //  - An eased span is LIMITS.rampEaseSteps uniform sub-pieces with node speeds from ease.
    //  - The speed is constant before the first and after the last point.
    //  - S (source frames consumed) is measured from clip time 0, so S(0) = 0, even when from > 0.
    // Pieces are {t0, t1, s0, a, b}: speed(t) = a + b·(t − t0) and S(t) = s0 + d·(a + b·d/2) with d = t − t0.
    // Outside [from, to] the edge speeds continue.
    function rampPieces(points, from, to) {
        const list = Array.isArray(points) ? points : [], start = Number(from), end = Number(to);
        if (!list.length || !(end > start)) return [];
        const lo = Math.min(start, 0), hi = Math.max(end, 0), steps = LIMITS.rampEaseSteps, raw = [];
        // A segment whose speed runs linearly from v0 at u0 to v1 at u1, cut to [lo, hi].
        const add = (u0, u1, v0, v1) => {
            const c0 = Math.max(u0, lo), c1 = Math.min(u1, hi);
            if (!(c1 > c0)) return;
            const piece = { t0: c0, t1: c1, s0: 0, a: v0, b: 0 };
            if (v1 !== v0) { piece.b = (v1 - v0) / (u1 - u0); piece.a = v0 + piece.b * (c0 - u0); }
            raw.push(piece);
        };
        const first = list[0], last = list[list.length - 1];
        add(-Infinity, first.f, first.speed, first.speed);
        for (let i = 0; i + 1 < list.length; i++) {
            const p = list[i], q = list[i + 1], u0 = p.f, u1 = q.f;
            if (p.e === 'hold') add(u0, u1, p.speed, p.speed);
            else if (p.e === 'ease_in' || p.e === 'ease_out' || p.e === 'ease_in_out') {
                for (let k = 0; k < steps; k++) {
                    add(u0 + (u1 - u0) * k / steps, u0 + (u1 - u0) * (k + 1) / steps,
                        p.speed + (q.speed - p.speed) * ease(p.e, k / steps), p.speed + (q.speed - p.speed) * ease(p.e, (k + 1) / steps));
                }
            } else add(u0, u1, p.speed, q.speed);
        }
        add(last.f, Infinity, last.speed, last.speed);
        // S from lo, shifted so that S(0) = 0 (0 lies in [lo, hi]); then cut to [from, to].
        let s = 0, zero = 0, found = false;
        raw.forEach(p => {
            p.s0 = s;
            if (!found && p.t0 <= 0 && 0 <= p.t1) { const lead = -p.t0; zero = s + lead * (p.a + p.b * lead / 2); found = true; }
            const d = p.t1 - p.t0;
            s += d * (p.a + p.b * d / 2);
        });
        const out = [];
        raw.forEach(p => {
            const c0 = Math.max(p.t0, start), c1 = Math.min(p.t1, end);
            if (!(c1 > c0)) return;
            const piece = { t0: p.t0, t1: c1, s0: p.s0 - zero, a: p.a, b: p.b };
            const d = c0 - p.t0;
            if (d > 0) { piece.s0 += d * (piece.a + piece.b * d / 2); piece.a += piece.b * d; piece.t0 = c0; }
            out.push(piece);
        });
        return out;
    }
    // The index of the piece holding clip time t: the last piece that starts at or before t, or the first.
    function rampPieceAt(pieces, t) {
        let lo = 0, hi = pieces.length;
        while (lo < hi) { const mid = (lo + hi) >> 1; if (pieces[mid].t0 > t) hi = mid; else lo = mid + 1; }
        return Math.max(0, lo - 1);
    }
    // S(t), its inverse and the speed, like Go rampSource, rampInverse and rampSpeed. Without pieces the clip plays at
    // speed 1. The inverse per piece is t0 + 2(s − s0)/(a + sqrt(a² + 2b(s − s0))), which also holds for b = 0.
    function rampSource(pieces, t) {
        const x = Number(t);
        if (!Array.isArray(pieces) || !pieces.length) return x;
        const p = pieces[rampPieceAt(pieces, x)];
        if (x < p.t0) return p.s0 - p.a * (p.t0 - x);
        if (x > p.t1) { const d = p.t1 - p.t0; return p.s0 + d * (p.a + p.b * d / 2) + (p.a + p.b * d) * (x - p.t1); }
        const d = x - p.t0;
        return p.s0 + d * (p.a + p.b * d / 2);
    }
    function rampInverse(pieces, s) {
        const x = Number(s);
        if (!Array.isArray(pieces) || !pieces.length) return x;
        let lo = 0, hi = pieces.length;
        while (lo < hi) { const mid = (lo + hi) >> 1; if (pieces[mid].s0 > x) hi = mid; else lo = mid + 1; }
        if (lo === 0) { const p = pieces[0]; return p.t0 - (p.s0 - x) / p.a; }
        const i = lo - 1, p = pieces[i], d = p.t1 - p.t0, end = p.s0 + d * (p.a + p.b * d / 2);
        if (i === pieces.length - 1 && x > end) return p.t1 + (x - end) / (p.a + p.b * d);
        const ds = x - p.s0;
        return p.t0 + 2 * ds / (p.a + Math.sqrt(p.a * p.a + 2 * p.b * ds));
    }
    function rampSpeed(pieces, t) {
        if (!Array.isArray(pieces) || !pieces.length) return 1;
        const x = Number(t), p = pieces[rampPieceAt(pieces, x)];
        return p.a + p.b * Math.max(0, Math.min(x, p.t1) - p.t0);
    }
    function speedOf(clip) { const speed = Number(clip && clip.speed); return speed > 0 ? speed : 1; }
    function hasRamp(clip) { return !!clip && Array.isArray(clip.ramp) && clip.ramp.length > 0; }
    // A clip's pieces over the clip and every point, ending one frame past the last point, so S is exact at any clip
    // time (Go clipRampPieces).
    function clipPieces(clip) {
        const points = clip.ramp;
        return rampPieces(points, Math.min(0, Number(points[0].f)), Math.max(Number(clip.duration) || 0, Number(points[points.length - 1].f) + 1));
    }
    // Source frames consumed after offset at clip time t (fractional, any sign): S(t) with a ramp, t·speed without.
    function clipSource(clip, t) { return hasRamp(clip) ? rampSource(clipPieces(clip), Number(t)) : Number(t) * speedOf(clip); }
    // The source position a clip shows at timeline frame frame, offset + clipSource(frame − start) (content anchor; Go
    // ClipSourceAt). Keyframes are sampled there.
    function sourceAt(clip, frame) { return (Number(clip && clip.offset) || 0) + clipSource(clip, Number(frame) - (Number(clip && clip.start) || 0)); }
    // The timeline frame (fractional) where the clip shows source position f; diamonds sit there.
    function timelineAt(clip, f) {
        const delta = Number(f) - (Number(clip && clip.offset) || 0);
        return (Number(clip && clip.start) || 0) + (hasRamp(clip) ? rampInverse(clipPieces(clip), delta) : delta / speedOf(clip));
    }
    // The playback speed at clip time t (the preview's playbackRate).
    function speedAt(clip, t) { return hasRamp(clip) ? rampSpeed(clipPieces(clip), Number(t)) : speedOf(clip); }
    // S(duration − 1): the source extent rule is offset + sourceExtent(clip) <= duration_frames − 1 (Go clipSourceExtent).
    function sourceExtent(clip) { return clipSource(clip, (Number(clip && clip.duration) || 0) - 1); }
    // The longest duration that fits the media from offset on: the largest d with offset + S(d − 1) <= frames − 1.
    // assetFrames is the asset's duration_frames; pass Infinity for a still (the result is Infinity).
    function maxDurationFor(clip, assetFrames, offset) {
        const frames = Number(assetFrames);
        if (!Number.isFinite(frames)) return Infinity;
        const room = frames - 1 - (Number(offset) || 0);
        return Math.floor((hasRamp(clip) ? rampInverse(clipPieces(clip), room) : room / speedOf(clip)) + 1e-9) + 1;
    }
    // Timeline frames the head can grow before the source runs out: the largest k with offset + S(−k) >= 0. Callers
    // treat stills as unbounded.
    function headroom(clip) {
        const offset = Number(clip && clip.offset) || 0;
        return Math.floor((hasRamp(clip) ? -rampInverse(clipPieces(clip), -offset) : offset / speedOf(clip)) + 1e-9);
    }
    // The value of a parameter path at an absolute timeline frame. Paths: x, y, width, height, rotation, opacity,
    // crop.l|t|r|b, mask.<param> and effects.<index>.<param> (the effect entry's params and keyframes). With keyframes,
    // the path's list is sampled at sourceAt(clip, frame), like Go ValueAt. Without keyframes it is the clip's own
    // number, else the registry default, else null. project is not used yet; the signature stays for later slices.
    function valueAt(clip, path, frame, project) {
        const parts = String(path || '').split('.');
        if (parts[0] === 'effects' && parts.length === 3) {
            const entry = clip && Array.isArray(clip.effects) ? clip.effects[Number(parts[1])] : null;
            const keys = entry && entry.keyframes ? entry.keyframes[parts[2]] : null;
            if (Array.isArray(keys) && keys.length) return sample(keys, sourceAt(clip, frame));
            const value = entry && entry.params ? entry.params[parts[2]] : undefined;
            if (finite(value)) return value;
            const param = entry ? ((effect(entry.id) || {}).params || []).find(item => item.id === parts[2]) : null;
            return param && finite(param.default) ? param.default : null;
        }
        const keys = clip && clip.keyframes ? clip.keyframes[path] : null;
        if (Array.isArray(keys) && keys.length) return sample(keys, sourceAt(clip, frame));
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

    window.VideoStudioEffects = { BUILTIN, EASINGS, LIMITS, adopt, registry, transitions, transition, isTransition, base, effect, defaults, validAnimation, ease, sample, keyframable, hasRamp, rampPieces, rampSource, rampInverse, rampSpeed, clipSource, sourceAt, timelineAt, speedAt, sourceExtent, maxDurationFor, headroom, valueAt, clipState, placement, fadeBlackWeights, wipeEdge };
})();
