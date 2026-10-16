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

    window.VideoStudioEffects = { BUILTIN, adopt, registry, transitions, transition, isTransition, base, effect, defaults, valueAt, clipState };
})();
