// Sound engine for desktop UI sounds: event catalogue, theme validation and registry, and the render pipeline
// (instrument voices -> bus -> master). Pure JavaScript on top of DesktopSoundDSP and DesktopSoundInstruments, so the same
// code renders in a Worker, on the main thread and in Node. Deterministic: every random draw comes from seeded generators.
(function (root) {
    'use strict';

    const D = root.DesktopSoundDSP, I = root.DesktopSoundInstruments;
    if (!D || !I) throw new Error('DesktopSoundDSP and DesktopSoundInstruments must be loaded before DesktopSoundEngine');

    const THEME_IDS = ['crystal', 'wood', 'analog', 'workshop', 'water', 'amiga', 'cyberpunk', 'black_gloss', 'crazy', 'lollipop', 'organic', 'electronic_dreams', 'windows_xp', 'green_slime', 'voxel_world', 'orbit'];
    const CATEGORIES = ['windows', 'notifications', 'navigation', 'files', 'interface', 'startup'];
    const EVENTS = ['session.start', 'notify.message', 'notify.info', 'notify.success', 'notify.warning', 'notify.error',
        'dialog.open', 'dialog.confirm', 'dialog.cancel', 'window.open', 'window.close', 'window.maximize', 'window.minimize',
        'window.restore', 'window.snap', 'window.deny', 'file.drop', 'file.trash', 'file.delete', 'space.switch',
        'menu.open', 'menu.close', 'ui.select', 'ui.toggle.on', 'ui.toggle.off', 'ui.tick'];
    const role = (category, lufs, maxSeconds, variants) => Object.freeze({ category, lufs, maxSeconds, variants });
    // Loudness target = maximum momentary loudness (LUFS) before the volume setting; maxSeconds includes the tail.
    const ROLES = Object.freeze({
        'session.start': role('startup', -18, 2.8, 1),
        'notify.message': role('notifications', -18, 1.4, 1),
        'notify.info': role('notifications', -19, 1.2, 1),
        'notify.success': role('notifications', -19, 1.2, 1),
        'notify.warning': role('notifications', -18, 1.2, 1),
        'notify.error': role('notifications', -18, 1.2, 1),
        'dialog.open': role('files', -22, 0.6, 1),
        'dialog.confirm': role('files', -21, 0.7, 1),
        'dialog.cancel': role('files', -23, 0.5, 1),
        'window.open': role('windows', -23, 0.6, 3),
        'window.close': role('windows', -24, 0.5, 3),
        'window.maximize': role('windows', -24, 0.6, 3),
        'window.minimize': role('windows', -25, 0.4, 3),
        'window.restore': role('windows', -25, 0.4, 3),
        'window.snap': role('windows', -26, 0.25, 3),
        'window.deny': role('windows', -23, 0.35, 2),
        'file.drop': role('files', -23, 0.5, 3),
        'file.trash': role('files', -23, 0.5, 2),
        'file.delete': role('files', -22, 0.6, 2),
        'space.switch': role('navigation', -25, 0.6, 2),
        'menu.open': role('navigation', -28, 0.2, 3),
        'menu.close': role('navigation', -30, 0.15, 3),
        'ui.select': role('interface', -31, 0.12, 3),
        'ui.toggle.on': role('interface', -31, 0.12, 3),
        'ui.toggle.off': role('interface', -32, 0.12, 3),
        'ui.tick': role('interface', -34, 0.06, 3)
    });
    const fxRole = (theme, category, maxSeconds, names) => Object.freeze({ theme, category, lufs: -23, maxSeconds, names: Object.freeze(names) });
    const FX_ROLES = Object.freeze({
        'window.fracture': fxRole('voxel_world', 'windows', 0.7, ['wood', 'stone', 'earth']),
        'window.impact': fxRole('voxel_world', 'windows', 0.7, ['wood', 'stone', 'earth']),
        'slime.pop': fxRole('green_slime', 'windows', 0.3, ['small', 'medium', 'large']),
        'slime.snap': fxRole('green_slime', 'windows', 0.3, ['small', 'medium', 'large']),
        'orbit.ping': fxRole('orbit', 'navigation', 1.0, ['small', 'medium', 'large'])
    });

    // ---- validation ----------------------------------------------------------------------------
    const has = (o, k) => Object.prototype.hasOwnProperty.call(o, k);
    const isObj = v => !!v && typeof v === 'object' && !Array.isArray(v);
    const BUS = {
        space: { size: [0.2, 2], rt60: [0.05, 3], damping: [1000, 16000], predelay: [0, 0.08], mix: [0, 0.6], lowcut: [80, 1000] },
        chorus: { rate: [0.1, 5], depth: [0.0005, 0.008], delay: [0.004, 0.03], mix: [0, 0.8] },
        echo: { time: [0.02, 0.8], feedback: [0, 0.85], tone: [500, 12000], mix: [0, 0.7], spread: [0, 1] },
        tone: { low: [-12, 12], high: [-12, 12] }
    };
    const THEME_FIELDS = ['id', 'key', 'space', 'voices', 'events', 'fx'];
    const SOUND_FIELDS = ['notes', 'layers', 'space', 'chorus', 'echo', 'drive', 'tone'];
    const ITEM_FIELDS = { v: null, n: [-36, 36], t: [0, 3], g: [0, 2], pan: [-1, 1], dur: [0.005, 3], detune: [-50, 50] };

    function validateTheme(def) {
        const errors = [];
        const fail = (path, message) => { errors.push(path + ': ' + message); };
        const number = (path, value, range) => {
            if (typeof value !== 'number' || !Number.isFinite(value)) fail(path, 'must be a finite number');
            else if (value < range[0] || value > range[1]) fail(path, value + ' is outside ' + range[0] + '..' + range[1]);
        };
        const busBlock = (path, kind, value) => {
            if (!isObj(value)) return fail(path, 'must be an object');
            Object.keys(value).forEach(k => { if (!BUS[kind][k]) fail(path + '.' + k, 'unknown ' + kind + ' setting'); else number(path + '.' + k, value[k], BUS[kind][k]); });
            if (kind === 'chorus' && value.depth != null && value.delay != null && value.depth >= value.delay) fail(path, 'depth must be smaller than delay');
        };
        const params = (path, modelName, values, skip) => {
            const spec = I.MODELS[modelName].params;
            Object.keys(values).forEach(k => {
                if (skip.includes(k)) return;
                const s = has(spec, k) ? spec[k] : null, p = path + '.' + k, v = values[k];
                if (!s) return fail(p, 'unknown parameter of model ' + modelName);
                if (Array.isArray(s)) return number(p, v, s);
                if (s.values) { if (!s.values.includes(v)) fail(p, 'must be one of ' + s.values.join(', ')); return; }
                if (s.pair) {
                    if (!Array.isArray(v) || v.length !== 2) return fail(p, 'must be [low, high]');
                    number(p + '[0]', v[0], s.pair); number(p + '[1]', v[1], s.pair);
                    if (v[0] >= v[1]) fail(p, 'low must be below high');
                    return;
                }
                if (s.list) {
                    if (!Array.isArray(v) || v.length > s.max) return fail(p, 'must be a list of at most ' + s.max + ' values');
                    v.forEach((x, i) => { if (!Number.isInteger(x)) fail(p + '[' + i + ']', 'must be an integer'); else number(p + '[' + i + ']', x, s.list); });
                }
            });
        };
        if (!isObj(def)) return ['theme: must be an object'];
        if (!THEME_IDS.includes(def.id)) fail('id', 'unknown theme id ' + JSON.stringify(def.id));
        Object.keys(def).forEach(k => { if (!THEME_FIELDS.includes(k)) fail(k, 'unknown theme field'); });
        if (!Number.isInteger(def.key)) fail('key', 'must be an integer MIDI note'); else number('key', def.key, [36, 96]);
        if (def.space == null) fail('space', 'missing'); else busBlock('space', 'space', def.space);
        const voices = isObj(def.voices) ? def.voices : {};
        if (!Object.keys(voices).length) fail('voices', 'must be a non-empty object');
        Object.keys(voices).forEach(name => {
            const voice = voices[name];
            if (!isObj(voice) || !has(I.MODELS, voice.model)) return fail('voices.' + name, 'unknown model ' + JSON.stringify(voice && voice.model));
            params('voices.' + name, voice.model, voice, ['model']);
        });
        const sound = (path, s, soundRole, isFx) => {
            if (!isObj(s)) return fail(path, 'must be an object');
            Object.keys(s).forEach(k => { if (!SOUND_FIELDS.includes(k)) fail(path + '.' + k, 'unknown sound field'); });
            const notes = s.notes || [], layers = s.layers || [];
            if (!Array.isArray(notes) || !Array.isArray(layers)) return fail(path, 'notes and layers must be arrays');
            if (!notes.length && !layers.length) fail(path, 'needs at least one note or layer');
            [[notes, 'notes', true], [layers, 'layers', false]].forEach(([list, field, asNote]) => list.forEach((item, i) => {
                const p = path + '.' + field + '[' + i + ']';
                if (!isObj(item)) return fail(p, 'must be an object');
                const voice = has(voices, item.v) ? voices[item.v] : null;
                if (!voice || !has(I.MODELS, voice.model)) return fail(p, 'unknown voice ' + JSON.stringify(item.v));
                const model = I.MODELS[voice.model];
                Object.keys(ITEM_FIELDS).forEach(k => { if (k !== 'v' && k !== 'n' && item[k] != null) number(p + '.' + k, item[k], ITEM_FIELDS[k]); });
                if (item.t != null && item.t > soundRole.maxSeconds - 0.01) fail(p + '.t', 'starts too close to the role maximum of ' + soundRole.maxSeconds + ' s');
                const overrides = {};
                Object.keys(item).forEach(k => { if (!has(ITEM_FIELDS, k)) overrides[k] = item[k]; });
                params(p, voice.model, overrides, []);
                const merged = I.resolveParams(voice.model, Object.assign({}, voice, overrides));
                const pitched = model.pitched(merged);
                if (asNote && !pitched) return fail(p, 'voice ' + item.v + ' is unpitched: put it in layers');
                if (!asNote && pitched) return fail(p, 'voice ' + item.v + ' is pitched: put it in notes with n');
                if (asNote) {
                    if (!Number.isInteger(item.n)) return fail(p + '.n', 'must be an integer semitone offset');
                    number(p + '.n', item.n, ITEM_FIELDS.n);
                    const freq = D.midiToHz(def.key + item.n), floor = model.minFreq(merged);
                    if (freq < floor) fail(p + '.n', freq.toFixed(1) + ' Hz is below the ' + floor + ' Hz floor of ' + voice.model);
                    if (freq > 12000) fail(p + '.n', 'above 12 kHz');
                    const ceiling = model.maxFreq ? model.maxFreq(merged) : null;
                    if (ceiling != null && freq * Math.pow(2, 54 / 1200) > ceiling) fail(p + '.n', freq.toFixed(1) + ' Hz is above the ' + ceiling + ' Hz limit of ' + voice.model);
                } else if (item.n != null) fail(p + '.n', 'layers take no n');
            }));
            if (s.space != null) { if (isFx) fail(path + '.space', 'effects use no room reverb'); else busBlock(path + '.space', 'space', s.space); }
            ['chorus', 'echo', 'tone'].forEach(k => { if (s[k] != null) busBlock(path + '.' + k, k, s[k]); });
            if (s.drive != null) number(path + '.drive', s.drive, [0, 6]);
        };
        const events = isObj(def.events) ? def.events : {};
        if (!isObj(def.events)) fail('events', 'must be an object');
        EVENTS.forEach(id => { if (!events[id]) fail('events.' + id, 'missing'); else sound('events.' + id, events[id], ROLES[id], false); });
        Object.keys(events).forEach(id => { if (!ROLES[id]) fail('events.' + id, 'unknown event'); });
        const fx = def.fx == null ? {} : def.fx;
        if (!isObj(fx)) fail('fx', 'must be an object');
        else {
            Object.keys(fx).forEach(id => {
                const r = FX_ROLES[id];
                if (!r) return fail('fx.' + id, 'unknown effect');
                if (r.theme !== def.id) return fail('fx.' + id, 'belongs to theme ' + r.theme);
                if (!isObj(fx[id])) return fail('fx.' + id, 'must map variant names to sounds');
                Object.keys(fx[id]).forEach(name => { if (!r.names.includes(name)) fail('fx.' + id + '.' + name, 'unknown variant'); });
                r.names.forEach(name => { if (!fx[id][name]) fail('fx.' + id + '.' + name, 'missing'); else sound('fx.' + id + '.' + name, fx[id][name], r, true); });
            });
            Object.keys(FX_ROLES).forEach(id => { if (FX_ROLES[id].theme === def.id && !fx[id]) fail('fx.' + id, 'missing'); });
        }
        return errors;
    }

    // ---- registry ------------------------------------------------------------------------------
    const registry = {};
    root.DesktopSoundThemes = root.DesktopSoundThemes || {};
    const warned = new Set();
    function deepFreeze(v) {
        if (v && typeof v === 'object' && !Object.isFrozen(v)) { Object.freeze(v); Object.keys(v).forEach(k => deepFreeze(v[k])); }
        return v;
    }
    function defineTheme(def) {
        const errors = validateTheme(def);
        if (errors.length) {
            const id = def && def.id;
            const message = 'desktop sound theme ' + id + ' is invalid:\n  ' + errors.join('\n  ');
            if (root.DesktopSoundEngineStrict) throw new Error(message);
            if (!warned.has(id)) { warned.add(id); console.warn(message); }
            return false;
        }
        deepFreeze(def);
        registry[def.id] = def;
        root.DesktopSoundThemes[def.id] = def;
        return true;
    }
    const themes = () => Object.keys(registry);

    // ---- rendering -----------------------------------------------------------------------------
    // A shell `variant` is an index 0..variants-1 (a number or a numeric string); an effect `variant` is a name.
    // The engine keeps no cache and returns fresh arrays, because the host transfers their buffers.
    function render(themeId, eventId, variant) {
        const theme = has(registry, themeId) ? registry[themeId] : null;
        if (!theme) throw new Error('unknown sound theme: ' + themeId);
        const fxR = Object.prototype.hasOwnProperty.call(FX_ROLES, eventId) ? FX_ROLES[eventId] : null;
        const r = fxR || (Object.prototype.hasOwnProperty.call(ROLES, eventId) ? ROLES[eventId] : null);
        if (!r) throw new Error('unknown sound event: ' + eventId);
        let def, index = 0;
        if (fxR) {
            const named = theme.fx && theme.fx[eventId];
            def = named && Object.prototype.hasOwnProperty.call(named, variant) ? named[variant] : null;
            if (!def) throw new Error('theme ' + themeId + ' has no effect ' + eventId + ':' + variant);
        } else {
            index = typeof variant === 'string' && variant.trim() === '' ? 0 : Number(variant || 0);
            if (!Number.isInteger(index) || index < 0 || index >= r.variants) throw new Error('variant ' + variant + ' out of range for ' + eventId);
            def = theme.events[eventId];
        }
        const seed = D.hashSeed(themeId, eventId, String(fxR ? variant : index));
        const vary = !fxR && index > 0 ? D.makeRng(D.hashSeed(seed, 'vary')) : null; // variant 0 is the canonical sound
        const items = (def.notes || []).map(x => [x, true]).concat((def.layers || []).map(x => [x, false]));
        const placed = items.map(([item, asNote], i) => {
            const voice = theme.voices[item.v];
            const params = {};
            Object.keys(voice).forEach(k => { if (k !== 'model') params[k] = voice[k]; });
            Object.keys(item).forEach(k => { if (!has(ITEM_FIELDS, k)) params[k] = item[k]; });
            const model = I.MODELS[voice.model];
            if (vary && model.params.hardness) params.hardness = D.clamp(I.resolveParams(voice.model, params).hardness + (vary() * 2 - 1) * 0.08, 0, 1);
            const dt = vary ? (vary() * 2 - 1) * 0.002 : 0;
            const dur = Math.min(item.dur ?? model.duration(I.resolveParams(voice.model, params)), r.maxSeconds);
            const note = { freq: D.midiToHz(theme.key + (asNote ? item.n : 0) + (item.detune || 0) / 100), dur,
                gain: item.g ?? 1, pan: item.pan ?? 0, at: Math.max(0, (item.t || 0) + dt), detune: vary ? 4 : 0 };
            return { model: voice.model, params, note, rand: D.makeRng(D.hashSeed(seed, 'note', i)) };
        });
        let mix = D.createMix(Math.max(0.05, ...placed.map(p => p.note.at + p.note.dur)) + 0.02);
        placed.forEach(p => D.addStereo(mix, I.render(p.model, p.params, p.note, p.rand), p.note.at));
        mix = bus(mix, def, theme, !!fxR);
        const m = D.master(mix, { lufs: r.lufs, ceiling: -3, lowcut: 110, maxSeconds: r.maxSeconds });
        if (!m.seconds) throw new Error(themeId + '/' + eventId + ': silent sound');
        let lufs = m.lufs, peakDb = m.peakDb;
        if (vary) { // level deviation of a variant, after mastering so the loudness gate still holds (+-0.4 dB)
            const offset = m.limited ? Math.min(0, (vary() * 2 - 1) * 0.4) : (vary() * 2 - 1) * 0.4;
            const g = D.dbToGain(offset);
            for (let i = 0; i < m.mix.n; i++) { m.mix.L[i] *= g; m.mix.R[i] *= g; }
            lufs += offset; peakDb += offset;
        }
        return { L: m.mix.L, R: m.mix.R, sampleRate: D.SR, seconds: m.seconds, naturalSeconds: m.naturalSeconds, lufs, peakDb, limited: m.limited };
    }
    function bus(mix, def, theme, isFx) { // fixed order: drive, tone, chorus, echo, space
        if (def.drive) { D.saturate(mix.L, def.drive); D.saturate(mix.R, def.drive); }
        if (def.tone && def.tone.low) { D.shelf(mix.L, 250, def.tone.low, false); D.shelf(mix.R, 250, def.tone.low, false); }
        if (def.tone && def.tone.high) { D.shelf(mix.L, 5000, def.tone.high, true); D.shelf(mix.R, 5000, def.tone.high, true); }
        if (def.chorus) mix = D.chorus(mix, def.chorus);
        if (def.echo) mix = D.echo(mix, def.echo);
        if (!isFx && def.space !== null) mix = D.reverb(mix, Object.assign({}, theme.space, def.space || {}));
        return mix;
    }

    const api = { THEME_IDS, EVENTS, ROLES, CATEGORIES, FX_ROLES, defineTheme, validateTheme, themes, render };
    root.DesktopSoundEngine = api;
    if (typeof module === 'object' && module && module.exports) module.exports = api;
})(typeof self !== 'undefined' ? self : globalThis);
