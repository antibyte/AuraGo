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
            { id: 'fit', type: 'choice', min: 0, max: 0, default: 'contain', choices: ['contain', 'cover'], label_key: 'fit' },
            { id: 'crop.l', type: 'number', min: 0, max: 0.95, default: 0, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'crop_l' },
            { id: 'crop.t', type: 'number', min: 0, max: 0.95, default: 0, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'crop_t' },
            { id: 'crop.r', type: 'number', min: 0, max: 0.95, default: 0, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'crop_r' },
            { id: 'crop.b', type: 'number', min: 0, max: 0.95, default: 0, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'crop_b' },
            { id: 'flip_h', type: 'bool', min: 0, max: 0, default: false, label_key: 'flip_h' },
            { id: 'flip_v', type: 'bool', min: 0, max: 0, default: false, label_key: 'flip_v' },
            { id: 'blend', type: 'choice', min: 0, max: 0, default: 'normal', choices: ['normal', 'multiply', 'screen', 'overlay', 'softlight', 'hardlight', 'darken', 'lighten', 'difference', 'exclusion', 'addition', 'subtract', 'dodge', 'burn'], label_key: 'blend' },
            { id: 'mask.shape', type: 'choice', min: 0, max: 0, default: 'rect', choices: ['rect', 'ellipse'], label_key: 'mask_shape' },
            { id: 'mask.x', type: 'number', min: -1, max: 1, default: 0.25, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'mask_x' },
            { id: 'mask.y', type: 'number', min: -1, max: 1, default: 0.25, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'mask_y' },
            { id: 'mask.w', type: 'number', min: 0.01, max: 2, default: 0.5, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'mask_w' },
            { id: 'mask.h', type: 'number', min: 0.01, max: 2, default: 0.5, step: 0.001, keyframable: true, unit: 'fraction', label_key: 'mask_h' },
            { id: 'mask.rotation', type: 'number', min: -360, max: 360, default: 0, step: 0.1, keyframable: true, unit: 'deg', label_key: 'mask_rotation' },
            { id: 'mask.feather', type: 'number', min: 0, max: 0.5, default: 0, step: 0.001, keyframable: true, unit: 'height', label_key: 'mask_feather' },
            { id: 'mask.invert', type: 'bool', min: 0, max: 0, default: false, label_key: 'mask_invert' }
        ],
        effects: [
            { id: 'color', category: 'color', label_key: 'effect.color.name', filters: ['lutrgb', 'colorchannelmixer'], params: [
                { id: 'exposure', type: 'number', min: -3, max: 3, default: 0, step: 0.01, keyframable: true, unit: 'ev', label_key: 'effect.color.exposure' },
                { id: 'contrast', type: 'number', min: 0, max: 3, default: 1, step: 0.01, keyframable: true, label_key: 'effect.color.contrast' },
                { id: 'saturation', type: 'number', min: 0, max: 3, default: 1, step: 0.01, keyframable: true, label_key: 'effect.color.saturation' },
                { id: 'temperature', type: 'number', min: 2000, max: 12000, default: 6500, step: 10, keyframable: true, unit: 'K', label_key: 'effect.color.temperature' },
                { id: 'tint', type: 'number', min: -1, max: 1, default: 0, step: 0.01, keyframable: true, label_key: 'effect.color.tint' },
                { id: 'lift_x', type: 'number', min: -1, max: 1, default: 0, step: 0.001, keyframable: true, unit: 'wheel', label_key: 'effect.color.lift_x' },
                { id: 'lift_y', type: 'number', min: -1, max: 1, default: 0, step: 0.001, keyframable: true, unit: 'wheel', label_key: 'effect.color.lift_y' },
                { id: 'lift', type: 'number', min: -0.5, max: 0.5, default: 0, step: 0.001, keyframable: true, label_key: 'effect.color.lift' },
                { id: 'gamma_x', type: 'number', min: -1, max: 1, default: 0, step: 0.001, keyframable: true, unit: 'wheel', label_key: 'effect.color.gamma_x' },
                { id: 'gamma_y', type: 'number', min: -1, max: 1, default: 0, step: 0.001, keyframable: true, unit: 'wheel', label_key: 'effect.color.gamma_y' },
                { id: 'gamma', type: 'number', min: 0.2, max: 5, default: 1, step: 0.01, keyframable: true, label_key: 'effect.color.gamma' },
                { id: 'gain_x', type: 'number', min: -1, max: 1, default: 0, step: 0.001, keyframable: true, unit: 'wheel', label_key: 'effect.color.gain_x' },
                { id: 'gain_y', type: 'number', min: -1, max: 1, default: 0, step: 0.001, keyframable: true, unit: 'wheel', label_key: 'effect.color.gain_y' },
                { id: 'gain', type: 'number', min: 0, max: 4, default: 1, step: 0.01, keyframable: true, label_key: 'effect.color.gain' }
            ] },
            { id: 'lut', category: 'color', label_key: 'effect.lut.name', filters: ['lut3d', 'blend'], params: [
                { id: 'lut', type: 'lut', min: 0, max: 0, default: '', label_key: 'effect.lut.lut' },
                { id: 'intensity', type: 'number', min: 0, max: 1, default: 1, step: 0.01, keyframable: true, label_key: 'effect.lut.intensity' }
            ] },
            { id: 'blur', category: 'detail', label_key: 'effect.blur.name', filters: ['gblur', 'premultiply', 'unpremultiply'], params: [
                { id: 'radius', type: 'number', min: 0, max: 0.05, default: 0.01, step: 0.0005, unit: 'height', label_key: 'effect.blur.radius' }
            ] },
            { id: 'sharpen', category: 'detail', label_key: 'effect.sharpen.name', filters: ['cas'], params: [
                { id: 'strength', type: 'number', min: 0, max: 1, default: 0.5, step: 0.01, keyframable: true, label_key: 'effect.sharpen.strength' }
            ] },
            { id: 'vignette', category: 'stylize', label_key: 'effect.vignette.name', filters: ['vignette', 'alphaextract', 'alphamerge'], params: [
                { id: 'amount', type: 'number', min: 0, max: 1, default: 0.5, step: 0.01, keyframable: true, label_key: 'effect.vignette.amount' },
                { id: 'size', type: 'number', min: 0.25, max: 2, default: 1, step: 0.01, keyframable: true, unit: 'height', label_key: 'effect.vignette.size' }
            ] },
            { id: 'chroma_key', category: 'key', label_key: 'effect.chroma_key.name', filters: ['colorkey', 'despill', 'alphaextract', 'alphamerge', 'blend'], params: [
                { id: 'color', type: 'color', min: 0, max: 0, default: '#00b140', label_key: 'effect.chroma_key.color' },
                { id: 'similarity', type: 'number', min: 0.01, max: 1, default: 0.25, step: 0.005, keyframable: true, label_key: 'effect.chroma_key.similarity' },
                { id: 'smoothness', type: 'number', min: 0, max: 1, default: 0.1, step: 0.005, keyframable: true, label_key: 'effect.chroma_key.smoothness' },
                { id: 'spill', type: 'number', min: 0, max: 1, default: 0.5, step: 0.01, keyframable: true, label_key: 'effect.chroma_key.spill' }
            ] }
        ],
        transitions: [
            { id: 'dissolve', xfade: 'fade', category: 'basic', label_key: 'dissolve' },
            { id: 'black', xfade: 'fadeblack', category: 'basic', label_key: 'black' },
            { id: 'wipeleft', xfade: 'wiperight', category: 'wipe', label_key: 'wipeleft' },
            { id: 'wiperight', xfade: 'wipeleft', category: 'wipe', label_key: 'wiperight' },
            { id: 'fadewhite', xfade: 'fadewhite', category: 'basic', label_key: 'transition_fadewhite' },
            { id: 'fadegrays', xfade: 'fadegrays', category: 'basic', label_key: 'transition_fadegrays' },
            { id: 'pixelize', xfade: 'pixelize', category: 'basic', label_key: 'transition_pixelize' },
            { id: 'wipeup', xfade: 'wipeup', category: 'wipe', label_key: 'transition_wipeup' },
            { id: 'wipedown', xfade: 'wipedown', category: 'wipe', label_key: 'transition_wipedown' },
            { id: 'smoothleft', xfade: 'smoothleft', category: 'wipe', label_key: 'transition_smoothleft' },
            { id: 'smoothright', xfade: 'smoothright', category: 'wipe', label_key: 'transition_smoothright' },
            { id: 'diagtl', xfade: 'diagtl', category: 'wipe', label_key: 'transition_diagtl' },
            { id: 'diagbr', xfade: 'diagbr', category: 'wipe', label_key: 'transition_diagbr' },
            { id: 'radial', xfade: 'radial', category: 'wipe', label_key: 'transition_radial' },
            { id: 'slideleft', xfade: 'slideleft', category: 'motion', label_key: 'transition_slideleft' },
            { id: 'slideright', xfade: 'slideright', category: 'motion', label_key: 'transition_slideright' },
            { id: 'slideup', xfade: 'slideup', category: 'motion', label_key: 'transition_slideup' },
            { id: 'slidedown', xfade: 'slidedown', category: 'motion', label_key: 'transition_slidedown' },
            { id: 'squeezeh', xfade: 'squeezeh', category: 'motion', label_key: 'transition_squeezeh' },
            { id: 'zoomin', xfade: 'zoomin', category: 'motion', label_key: 'transition_zoomin' },
            { id: 'circleopen', xfade: 'circleopen', category: 'shape', label_key: 'transition_circleopen' },
            { id: 'circleclose', xfade: 'circleclose', category: 'shape', label_key: 'transition_circleclose' },
            { id: 'circlecrop', xfade: 'circlecrop', category: 'shape', label_key: 'transition_circlecrop' },
            { id: 'rectcrop', xfade: 'rectcrop', category: 'shape', label_key: 'transition_rectcrop' }
        ],
        blend_modes: [
            { id: 'normal', blend: '', label_key: 'blend_normal' },
            { id: 'multiply', blend: 'multiply', label_key: 'blend_multiply' },
            { id: 'screen', blend: 'screen', label_key: 'blend_screen' },
            { id: 'overlay', blend: 'overlay', label_key: 'blend_overlay' },
            { id: 'softlight', blend: 'softlight', label_key: 'blend_softlight' },
            { id: 'hardlight', blend: 'hardlight', label_key: 'blend_hardlight' },
            { id: 'darken', blend: 'darken', label_key: 'blend_darken' },
            { id: 'lighten', blend: 'lighten', label_key: 'blend_lighten' },
            { id: 'difference', blend: 'difference', label_key: 'blend_difference' },
            { id: 'exclusion', blend: 'exclusion', label_key: 'blend_exclusion' },
            { id: 'addition', blend: 'addition', label_key: 'blend_addition' },
            { id: 'subtract', blend: 'subtract', label_key: 'blend_subtract' },
            { id: 'dodge', blend: 'dodge', label_key: 'blend_dodge' },
            { id: 'burn', blend: 'burn', label_key: 'blend_burn' }
        ],
        anim_presets: [
            { id: 'fade', label_key: 'anim_fade', filters: ['sendcmd', 'colorchannelmixer'] },
            { id: 'slide_up', label_key: 'anim_slide_up', filters: ['sendcmd', 'colorchannelmixer'] },
            { id: 'slide_down', label_key: 'anim_slide_down', filters: ['sendcmd', 'colorchannelmixer'] },
            { id: 'slide_left', label_key: 'anim_slide_left', filters: ['sendcmd', 'colorchannelmixer'] },
            { id: 'slide_right', label_key: 'anim_slide_right', filters: ['sendcmd', 'colorchannelmixer'] },
            { id: 'pop', label_key: 'anim_pop', filters: ['sendcmd', 'colorchannelmixer'] },
            { id: 'wipe', label_key: 'anim_wipe', filters: ['geq', 'blend'] },
            { id: 'blur_in', label_key: 'anim_blur_in', filters: ['gblur', 'maskedmerge', 'sendcmd', 'colorchannelmixer', 'premultiply', 'unpremultiply', 'extractplanes', 'alphamerge'] }
        ],
        looks: [
            { id: 'warm', label_key: 'look_warm' },
            { id: 'cool', label_key: 'look_cool' },
            { id: 'teal_orange', label_key: 'look_teal_orange' },
            { id: 'mono', label_key: 'look_mono' },
            { id: 'faded_film', label_key: 'look_faded_film' },
            { id: 'high_contrast', label_key: 'look_high_contrast' },
            { id: 'vintage', label_key: 'look_vintage' },
            { id: 'bleach_bypass', label_key: 'look_bleach_bypass' }
        ]
    });

    const LISTS = ['base', 'effects', 'transitions', 'blend_modes', 'anim_presets', 'looks'];
    const UNAVAILABLE_LISTS = ['effects', 'transitions', 'blend_modes', 'anim_presets'];
    const isText = value => typeof value === 'string' && value !== '';
    const finite = value => typeof value === 'number' && Number.isFinite(value);
    const clamp01 = value => Math.max(0, Math.min(1, value));
    const emptyUnavailable = () => ({ effects: new Set(), transitions: new Set(), blend_modes: new Set(), anim_presets: new Set() });
    // Keyframe, speed-ramp and effect names and limits (index §4.4, §4.18), the same as Go: the Easing constants
    // (keyframes.go), MaxKeyframesPerParam, MaxKeyframesPerClip, MaxRampPoints, MaxRampPieces, the unexported
    // rampEaseSteps (ramp.go) and MaxEffectsPerClip (model.go, 2c).
    const EASINGS = Object.freeze(['linear', 'ease_in', 'ease_out', 'ease_in_out', 'hold']);
    const LIMITS = Object.freeze({ keyframesPerParam: 64, keyframesPerClip: 512, rampPoints: 16, rampPieces: 64, rampEaseSteps: 4, effectsPerClip: 8 });
    // Go's MinClipSpeed and MaxClipSpeed.
    const MIN_SPEED = 0.25, MAX_SPEED = 16;
    // Go's MinBoxSize: the smallest width or height a keyframe may hold (the editor's 1 % box minimum).
    const MIN_BOX_SIZE = 0.01;
    // Go's maxCropFraction: the largest share of one crop side, and of l + r and t + b (with 1e-9 of float slack).
    const CROP_MAX = 0.95;
    const CROP_PATHS = Object.freeze(['crop.l', 'crop.t', 'crop.r', 'crop.b']);
    // The keyframable mask paths (2d) in registry order (Go maskPaths). animatablePaths lists them only while the clip
    // has a mask.
    const MASK_PATHS = Object.freeze(['mask.x', 'mask.y', 'mask.w', 'mask.h', 'mask.rotation', 'mask.feather']);
    const isObject = value => !!value && typeof value === 'object' && !Array.isArray(value);
    // Keys in a keyframe map (clip.keyframes or an effect entry's keyframes); anything but a list counts 0.
    const keyCount = map => (isObject(map) ? Object.values(map).reduce((n, list) => n + (Array.isArray(list) ? list.length : 0), 0) : 0);
    // The keys of a clip's effect entries. They count toward the 512 keys of the clip (spec §5.8, Go countKeyframes).
    const effectKeyCount = clip => (clip && Array.isArray(clip.effects) ? clip.effects.reduce((n, entry) => n + keyCount(entry && entry.keyframes), 0) : 0);
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
    // Effects in menu order. available is false when the server's FFmpeg lacks one of their filters: the effect still
    // previews, and the export refuses it. effectAvailable(id) is false for unknown ids too.
    function effects() { return current.effects.map(spec => Object.assign({}, spec, { available: !missing.effects.has(spec.id) })); }
    function effectAvailable(id) { return !!effect(id) && !missing.effects.has(id); }
    // The built-in looks in menu order, [{id, label_key}].
    function looks() { return current.looks.map(look => ({ id: look.id, label_key: look.label_key })); }
    // Blend modes (2d) in menu order, [{id, blend, label_key, available}]. available is false when the server's FFmpeg
    // cannot export the mode; the preview still draws it. blendAvailable(id) is false for unknown ids too.
    function blendMode(id) { return current.blend_modes.find(spec => spec.id === id) || null; }
    function blendModes() { return current.blend_modes.map(spec => Object.assign({}, spec, { available: !missing.blend_modes.has(spec.id) })); }
    function blendAvailable(id) { return !!blendMode(id) && !missing.blend_modes.has(id); }

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
            // Base and effect keys share the 512 keys of a clip (spec §5.8, Go countKeyframes).
            if (keyCount(keyframes) + effectKeyCount(clip) > LIMITS.keyframesPerClip) return false;
            // Mask keys need a mask (2d, Go validateMask).
            if (paths.some(path => MASK_PATHS.includes(path)) && !isObject(clip.mask)) return false;
            const bound = MAX_SPEED * maxFrames;
            for (const path of paths) {
                const spec = base(path), list = keyframes[path];
                if (!spec || !spec.keyframable || spec.type !== 'number' || !Array.isArray(list) || !list.length || list.length > LIMITS.keyframesPerParam) return false;
                for (let i = 0; i < list.length; i++) {
                    const key = list[i];
                    if (!key || !Number.isSafeInteger(key.f) || Math.abs(key.f) > bound || (i > 0 && key.f <= list[i - 1].f)) return false;
                    if (!finite(key.v) || key.v < spec.min || key.v > spec.max || ((path === 'width' || path === 'height') && key.v < MIN_BOX_SIZE) || !EASINGS.includes(key.e)) return false;
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

    // ---- Plan 2c: crop, flips and the effect stack (index §4.1, §4.2, §4.4). ----
    // One params value against its registry spec (Go validEffectParam): a finite number in range, a known LUT ref,
    // a boolean, a listed choice or a #rrggbb colour.
    function validParam(spec, value, project) {
        switch (spec.type) {
            case 'number': return finite(value) && value >= spec.min && value <= spec.max;
            case 'lut': return typeof value === 'string' && validLUTRef(value, project);
            case 'bool': return typeof value === 'boolean';
            case 'choice': return typeof value === 'string' && (spec.choices || []).includes(value);
            case 'color': return typeof value === 'string' && /^#[0-9a-fA-F]{6}$/.test(value);
            default: return false;
        }
    }
    // A valid LUT ref is '' (pass-through), 'builtin:<look id>' of the registry, or the id of a lut asset of the
    // project (Go validLUTRef).
    function validLUTRef(ref, project) {
        if (ref === '') return true;
        if (ref.startsWith('builtin:')) return current.looks.some(look => look.id === ref.slice('builtin:'.length));
        return ((project && project.assets) || []).some(asset => !!asset && asset.id === ref && asset.kind === 'lut');
    }
    // The keys of one effect parameter with the base rules (Go validEffectKeys): 1..64 keys, strictly increasing integer
    // frames within ±bound, values in the parameter's range and known easings.
    function validKeyList(list, spec, bound) {
        if (!Array.isArray(list) || !list.length || list.length > LIMITS.keyframesPerParam) return false;
        return list.every((key, i) => !!key && Number.isSafeInteger(key.f) && Math.abs(key.f) <= bound && (i === 0 || key.f > list[i - 1].f) &&
            finite(key.v) && key.v >= spec.min && key.v <= spec.max && EASINGS.includes(key.e));
    }
    // Mirror of Go validateBlend and validateMask (2d, index §4.1). Both belong to visual clips only.
    //  - blend: absent, null or '' means normal; otherwise a registry mode other than 'normal' (the editor deletes the
    //    field for normal).
    //  - mask: an object with shape rect or ellipse; x, y, w, h, rotation and feather finite and inside their base rows
    //    (a missing number reads 0, as Go decodes it); invert absent or a boolean.
    // The mask keys go through validAnimation.
    function validCompositing(clip, audio) {
        const blend = clip.blend, mask = clip.mask;
        if (blend != null && blend !== '' && (audio || typeof blend !== 'string' || blend === 'normal' || !blendMode(blend))) return false;
        if (mask == null) return true;
        if (audio || !isObject(mask) || (mask.shape !== 'rect' && mask.shape !== 'ellipse')) return false;
        if (mask.invert != null && typeof mask.invert !== 'boolean') return false;
        return MASK_PATHS.every(path => {
            const spec = base(path), field = mask[path.slice('mask.'.length)], value = field == null ? 0 : field;
            return !!spec && finite(value) && value >= spec.min && value <= spec.max;
        });
    }
    // Mirror of Go validateCrop and validateEffects. Crop, flips and effects belong to visual clips only.
    //  - crop: four numbers 0..0.95, l + r and t + b at most 0.95, not all zero (the editor deletes an empty crop).
    //  - effects: at most 8 entries. Each has a known registry id (on or off) and a params object of known parameters
    //    whose values have the registry type and range. LUT refs name a built-in look or a lut asset of the project.
    //  - entry keyframes sit on keyframable number parameters only (not the blur radius).
    //  - base and entry keys together stay within 512 per clip.
    function validPicture(clip, track, project) {
        if (!clip) return false;
        const crop = clip.crop, stack = clip.effects, audio = !track || track.kind === 'audio';
        if ((clip.flip_h != null && typeof clip.flip_h !== 'boolean') || (clip.flip_v != null && typeof clip.flip_v !== 'boolean')) return false;
        if (audio && (crop != null || clip.flip_h || clip.flip_v)) return false;
        if (crop != null) {
            if (!isObject(crop)) return false;
            const sides = [crop.l, crop.t, crop.r, crop.b];
            if (!sides.every(side => finite(side) && side >= 0 && side <= CROP_MAX)) return false;
            if (crop.l + crop.r > CROP_MAX + 1e-9 || crop.t + crop.b > CROP_MAX + 1e-9 || sides.every(side => side === 0)) return false;
        }
        if (!validCompositing(clip, audio)) return false;
        if (stack == null) return true;
        if (!Array.isArray(stack)) return false;
        if (!stack.length) return true;
        if (audio || stack.length > LIMITS.effectsPerClip || keyCount(clip.keyframes) + effectKeyCount(clip) > LIMITS.keyframesPerClip) return false;
        const bound = MAX_SPEED * window.VideoStudioTime.maxFrames(project);
        return stack.every(entry => {
            const spec = isObject(entry) ? effect(entry.id) : null;
            if (!spec || (entry.on != null && typeof entry.on !== 'boolean') || !isObject(entry.params)) return false;
            const param = id => spec.params.find(item => item.id === id) || null;
            if (!Object.keys(entry.params).every(id => !!param(id) && validParam(param(id), entry.params[id], project))) return false;
            if (entry.keyframes == null) return true;
            return isObject(entry.keyframes) && Object.keys(entry.keyframes).every(id => {
                const p = param(id);
                return !!p && !!p.keyframable && p.type === 'number' && validKeyList(entry.keyframes[id], p, bound);
            });
        });
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
    // The keyframable base parameters of the registry (2b: x, y, width, height, rotation, opacity; 2c: crop.*; 2d:
    // mask.*), in registry order.
    function keyframable() { return current.base.filter(spec => spec.keyframable && spec.type === 'number').map(spec => spec.id); }
    // True when a crop side has keyframes. The export then crops after the effects (index §4.20).
    function cropAnimated(clip) {
        const keyframes = clip && isObject(clip.keyframes) ? clip.keyframes : {};
        return CROP_PATHS.some(path => Array.isArray(keyframes[path]) && keyframes[path].length > 0);
    }
    // True when a mask path has keyframes (2d). The export then evaluates the mask per frame.
    function maskAnimated(clip) {
        const keyframes = clip && isObject(clip.keyframes) ? clip.keyframes : {};
        return MASK_PATHS.some(path => Array.isArray(keyframes[path]) && keyframes[path].length > 0);
    }
    // Every path of a clip that can carry keyframes, in registry and stack order: the keyframable base paths (mask.*
    // only while the clip has a mask, 2d), then effects.<i>.<param> for the keyframable number parameters of each known
    // entry (on or off).
    function animatablePaths(clip) {
        const masked = !!clip && isObject(clip.mask);
        const paths = keyframable().filter(path => masked || !MASK_PATHS.includes(path));
        (clip && Array.isArray(clip.effects) ? clip.effects : []).forEach((entry, index) => {
            const spec = entry ? effect(entry.id) : null;
            if (spec) spec.params.forEach(param => { if (param.keyframable && param.type === 'number') paths.push('effects.' + index + '.' + param.id); });
        });
        return paths;
    }
    // The ParamSpec of any path of a clip (a base path, or effects.<i>.<param> of a known entry), or null.
    function paramSpec(clip, path) {
        const parts = String(path || '').split('.');
        if (parts[0] !== 'effects') return base(path);
        if (parts.length !== 3 || !/^(0|[1-9][0-9]*)$/.test(parts[1])) return null;
        const entry = clip && Array.isArray(clip.effects) ? clip.effects[Number(parts[1])] : null;
        const spec = entry ? effect(entry.id) : null;
        return spec ? spec.params.find(param => param.id === parts[2]) || null : null;
    }
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
    // The effect entries as both painters see them (index §4.4, 2c): every entry that is on and has a known id
    // (available or not), with its stack index and every parameter at the frame. Numbers go through valueAt
    // (keyframes, params, registry default); other types come from params when they have the default's type, else the
    // default.
    function effectStates(clip, frame, project) {
        const out = [];
        (clip && Array.isArray(clip.effects) ? clip.effects : []).forEach((entry, index) => {
            const spec = entry && entry.on === true ? effect(entry.id) : null;
            if (!spec) return;
            const params = isObject(entry.params) ? entry.params : {}, values = {};
            spec.params.forEach(param => {
                values[param.id] = param.type === 'number' ? valueAt(clip, 'effects.' + index + '.' + param.id, frame, project)
                    : typeof params[param.id] === typeof param.default ? params[param.id] : param.default;
            });
            out.push({ index, id: entry.id, values });
        });
        return out;
    }
    // The clip's mask at a frame (2d, index §4.4): null without a mask object, else {shape, invert, x, y, w, h, rotation,
    // feather} with the numbers through valueAt (keys at the source position, else the field) and the static shape and
    // invert.
    function maskState(clip, frame, project) {
        const mask = clip && isObject(clip.mask) ? clip.mask : null;
        if (!mask) return null;
        const at = path => valueAt(clip, path, frame, project);
        return {
            shape: mask.shape === 'ellipse' ? 'ellipse' : 'rect', invert: mask.invert === true,
            x: at('mask.x'), y: at('mask.y'), w: at('mask.w'), h: at('mask.h'), rotation: at('mask.rotation'), feather: at('mask.feather')
        };
    }
    // The resolved state of a clip at a frame, which both painters draw from (index §4.4). The shape stays fixed; later
    // slices change how values are evaluated (2b keyframes, 2c crop/flip/effects, 2d blend/mask, 2e animation). The crop
    // is evaluated like any base path (keyframes, else the field, else 0); blend is the clip's mode (available or not)
    // or 'normal'.
    function clipState(clip, frame, project) {
        const at = path => valueAt(clip, path, frame, project);
        return {
            x: at('x'), y: at('y'), width: at('width'), height: at('height'), rotation: at('rotation'),
            opacity: clamp01(at('opacity')), fit: clip && clip.fit === 'cover' ? 'cover' : 'contain',
            crop: { l: at('crop.l'), t: at('crop.t'), r: at('crop.r'), b: at('crop.b') },
            flipH: !!(clip && clip.flip_h === true), flipV: !!(clip && clip.flip_v === true),
            blend: clip && typeof clip.blend === 'string' && clip.blend ? clip.blend : 'normal', mask: maskState(clip, frame, project),
            effects: effectStates(clip, frame, project)
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

    // ---- Plan 2b Task 9: head edits, still requests and rate changes of ramped clips (index §4.18). ----
    // {offset, ramp} after the clip's first frame moves by `frames` timeline frames.
    // - Positive frames move the head later: a split's right part, a start trim, roll or slide. Negative frames grow it.
    // - The source that the head passes is baked into the offset, rounded down and never below 0, exactly like phase 1
    //   (offset + frames x speed without a ramp).
    // - Ramp points are in clip time, so they shift by -frames and the curve stays on the same timeline frames.
    // - ramp is null without a ramp.
    function headShift(clip, frames) {
        const d = Number(frames) || 0, offset = Number(clip && clip.offset) || 0;
        if (!hasRamp(clip)) {
            const speed = Number(clip && clip.speed) > 0 ? Number(clip.speed) : 1;
            return { offset: Math.max(0, Math.floor(offset + d * speed + 1e-9)), ramp: null };
        }
        // Pieces over a range that holds 0 and d. Their nodes depend only on the spans, so the range does not change S.
        const passed = rampSource(rampPieces(clip.ramp, Math.min(0, d) - 1, Math.max(0, d) + 1), d);
        return { offset: Math.max(0, Math.floor(offset + passed + 1e-9)), ramp: clip.ramp.map(point => ({ f: point.f - d, speed: point.speed, e: point.e })) };
    }
    // The constant speed that consumes as much source over the clip's first t frames as the clip does:
    // clipSource(clip, t) / t, kept inside 0.25..16 against float rounding, and 1 at t <= 0. Without a ramp it is the
    // clip's own speed. Still requests (freeze frame, export frame) of a ramped clip send it. The server's unchanged
    // floor(offset + t x speed) check and ExtractStill then pick floor(offset + S(t)), the frame the export shows.
    function meanSpeed(clip, t) {
        const speed = Number(clip && clip.speed) > 0 ? Number(clip.speed) : 1, n = Number(t) || 0;
        if (!hasRamp(clip)) return speed;
        return n > 0 ? Math.max(0.25, Math.min(16, clipSource(clip, n) / n)) : 1;
    }
    // Frame-rate change (VideoStudioTime.rescaleProject): {keyframes, ramp} of the clip at ratio = new rate / old rate.
    // - Ramp points are clip durations, so f scales and rounds like a duration.
    // - A point that rounds onto the one before is dropped. One point left means a constant speed; rescaleProject
    //   applies it.
    // - Speeds are frames per frame and stay. ramp is null without a ramp.
    // - Key frames scale like the offset (rounded; a key that lands on the one before it is dropped, the first stays).
    //   keyframes is undefined without keys, so rescaleProject deletes the field.
    function rescaleAnimation(clip, ratio) {
        const ramp = hasRamp(clip) ? clip.ramp.map(point => ({ f: Math.round(point.f * ratio), speed: point.speed, e: point.e }))
            .filter((point, index, list) => index === 0 || point.f !== list[index - 1].f) : null;
        return { keyframes: remapKeyframes(clip && clip.keyframes, f => Math.round(f * ratio)), ramp };
    }

    // ---- Plan 2b Task 10: keyframes under reverse, paste attributes and rate changes (index §4.18). ----
    // Keyframe maps under a change of source time. map takes a key's source frame and returns its new one (non-decreasing
    // in f); a key that lands on the frame of the key before it is dropped (the first one stays). Paths keep alphabetical
    // order (Go marshals map keys sorted). undefined when no key remains, so callers delete the field.
    function remapKeyframes(keyframes, map) {
        const out = {};
        Object.keys(keyframes || {}).sort().forEach(path => {
            const list = [];
            (Array.isArray(keyframes[path]) ? keyframes[path] : []).forEach(key => {
                const f = map(Number(key.f));
                if (!list.length || f > list[list.length - 1].f) list.push({ f, v: key.v, e: key.e });
            });
            if (list.length) out[path] = list;
        });
        return Object.keys(out).length ? out : undefined;
    }
    const MIRRORED_EASING = { ease_in: 'ease_out', ease_out: 'ease_in' };
    // Reverse: f' = pivot - f in reversed order. The span right of a mirrored key is the original span left of it, so
    // the key takes the mirrored easing of the key before it (ease_in <-> ease_out); the last mirrored key takes that of
    // the original last key, so mirroring twice gives the keys back. A hold jumps at the end of its span and, mirrored,
    // would have to jump at the start: a hold longer than one frame gets an extra key one source frame after the
    // mirrored key, carrying the held value (whole source frames show the original values; a fractional position may
    // lead by up to one frame). undefined when there are no keys.
    function mirrorKeyframes(keyframes, pivot) {
        const out = {};
        Object.keys(keyframes || {}).sort().forEach(path => {
            const list = (Array.isArray(keyframes[path]) ? keyframes[path] : []).slice().sort((a, b) => a.f - b.f), n = list.length, mirrored = [];
            for (let j = 0; j < n; j++) {
                const key = list[n - 1 - j], before = list[n - 2 - j], easing = (before || list[n - 1]).e;
                mirrored.push({ f: pivot - key.f, v: key.v, e: MIRRORED_EASING[easing] || easing });
                if (before && before.e === 'hold' && key.f - before.f >= 2) mirrored.push({ f: pivot - key.f + 1, v: before.v, e: 'linear' });
            }
            if (mirrored.length) out[path] = mirrored;
        });
        return Object.keys(out).length ? out : undefined;
    }
    // Paste attributes: source's keys at the same timing relative to each clip's start on target. A key
    // dt = timelineAt(source, f) - source.start frames after the source's start lands at
    // round(sourceAt(target, target.start + dt)). undefined when source has no keys.
    function reanchorKeyframes(source, target) {
        const from = Number(source && source.start) || 0, to = Number(target && target.start) || 0;
        return remapKeyframes(source && source.keyframes, f => Math.round(sourceAt(target, to + timelineAt(source, f) - from)));
    }

    // ---- Plan 2c Task 13: effect-entry keys under reverse, rate changes and paste attributes (index §4.4). Each returns a
    // new stack (entries copied with their params; keys mapped like the clip's own) or undefined without entries.
    function mapEffects(effects, map) {
        if (!Array.isArray(effects) || !effects.length) return undefined;
        return effects.map(entry => {
            const out = { id: entry.id, on: !!entry.on, params: JSON.parse(JSON.stringify(entry.params || {})) };
            const keyframes = map(entry.keyframes);
            if (keyframes) out.keyframes = keyframes;
            return out;
        });
    }
    // Reverse: entry keys mirror around pivot like mirrorKeyframes (ease_in <-> ease_out, hold spans shifted).
    function mirrorEffects(effects, pivot) { return mapEffects(effects, keyframes => mirrorKeyframes(keyframes, pivot)); }
    // Frame-rate change: entry key frames scale like the offset (rounded; a key landing on the one before it is dropped).
    function rescaleEffects(effects, ratio) { return mapEffects(effects, keyframes => remapKeyframes(keyframes, f => Math.round(f * ratio))); }
    // Paste attributes: source's entries, their keys at the same timing relative to each clip's start on target.
    function reanchorEffects(source, target) {
        const from = Number(source && source.start) || 0, to = Number(target && target.start) || 0;
        return mapEffects(source && source.effects, keyframes => remapKeyframes(keyframes, f => Math.round(sourceAt(target, to + timelineAt(source, f) - from))));
    }

    window.VideoStudioEffects = { BUILTIN, EASINGS, LIMITS, MIN_BOX_SIZE, MASK_PATHS, adopt, registry, transitions, transition, isTransition, base, effect, effects, effectAvailable, looks, blendModes, blendAvailable, defaults, validAnimation, validPicture, ease, sample, keyframable, cropAnimated, maskAnimated, animatablePaths, paramSpec, hasRamp, rampPieces, rampSource, rampInverse, rampSpeed, clipSource, sourceAt, timelineAt, speedAt, sourceExtent, maxDurationFor, headroom, headShift, meanSpeed, rescaleAnimation, mirrorKeyframes, reanchorKeyframes, mirrorEffects, rescaleEffects, reanchorEffects, valueAt, clipState, placement, fadeBlackWeights, wipeEdge };
})();
