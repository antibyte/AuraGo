(function (root) {
    "use strict";

    // Programmable synthesizer for Synth Studio: the patch schema of the voice
    // engine exposed as an editable instrument, plus the browser-local user
    // patch bank. The presets catalog gains the stored patches as visible
    // instruments (`user` category) and a runtime-only preview entry that never
    // appears in lists. The project format only ever stores the instrument id,
    // so no project schema change is needed.

    const STORAGE_KEY = "aurago:synth-studio:patches:1";
    const PREVIEW_ID = "user-preview";
    const MAX_PATCHES = 24;
    const MAX_NAME = 60;
    const USER_ID = /^user-[A-Za-z0-9_-]{1,95}$/;

    const WAVES = ["sine", "triangle", "sawtooth", "square", "pulse25", "pulse12", "supersaw", "w9999", "formant", "bell"];
    const SUB_WAVES = ["sine", "triangle", "sawtooth", "square"];
    const NOISES = ["n0", "n1", "n2", "n3"];
    const MOD_WAVES = ["sine", "triangle", "sawtooth", "square", "pulse25", "formant", "bell"];
    const FILTER_TYPES = ["lowpass", "highpass", "bandpass", "notch"];
    const LFO_TARGETS = ["pitch", "amp", "filter", "pulse"];
    const LFO_WAVES = ["sine", "triangle", "sawtooth", "square"];

    // Ranges mirror the voice engine clamps (customPatch, createVoice,
    // attachLfo in synth-studio-voices.js); normalization always clamps to
    // the engine bounds again before a patch reaches audio or storage.
    const DEFAULT_LFO = { target: "pitch", wave: "sine", rate: 3, depth: 0, delay: 0.05 };
    const DEFAULT_PATCH = {
        wave: "sawtooth", level: 0.22, second: 0, detune: 8, unison: 1, pulseWidth: 0,
        sub: 0, subWave: "sine", subOctave: 1, noise: 0, noiseWave: "n0",
        mod: "", ratio: 2, depth: 2, mod2: "", ratio2: 4, depth2: 1.4, decay2: 0.12, mod3: "", ratio3: 3, depth3: 1, decay3: 0.12,
        attack: 0.008, decay: 0.25, sustain: 0.65, release: 0.18,
        pitchEnv: 1, pitchTime: 0.045,
        cutoff: 0.65, filterType: "lowpass", filterEnv: 0, filterDecay: 0.22, filterQ: 1,
        lfo: { ...DEFAULT_LFO }
    };

    function clampTo(value, min, max, fallback) {
        const number = Number(value);
        if (!Number.isFinite(number)) return fallback;
        return Math.max(min, Math.min(max, Math.round(number * 1000) / 1000));
    }
    function oneOf(list, value, fallback) { return list.includes(value) ? value : fallback; }
    function waveOrOff(value) { return (typeof value === "string" && MOD_WAVES.includes(value)) ? value : ""; }
    function normalizeLfo(source) {
        const lfo = source && typeof source === "object" ? source : {};
        return {
            target: oneOf(LFO_TARGETS, lfo.target, "pitch"),
            wave: oneOf(LFO_WAVES, lfo.wave, "sine"),
            rate: clampTo(lfo.rate, 0.05, 20, 3),
            depth: clampTo(lfo.depth, 0, 1, 0),
            delay: clampTo(lfo.delay, 0, 8, 0.05)
        };
    }
    function normalize(source) {
        const p = source && typeof source === "object" ? source : {};
        return {
            wave: oneOf(WAVES, p.wave, "sawtooth"),
            level: clampTo(p.level, 0, 1.5, 0.22),
            second: clampTo(p.second, 0, 1, 0),
            detune: clampTo(p.detune, 0, 1200, 8),
            unison: p.unison === 3 || p.unison === 5 ? p.unison : 1,
            pulseWidth: clampTo(p.pulseWidth, 0, 0.95, 0),
            sub: clampTo(p.sub, 0, 0.6, 0),
            subWave: oneOf(SUB_WAVES, p.subWave, "sine"),
            subOctave: p.subOctave === 2 ? 2 : 1,
            noise: clampTo(p.noise, 0, 1, 0),
            noiseWave: oneOf(NOISES, p.noiseWave, "n0"),
            mod: waveOrOff(p.mod),
            ratio: clampTo(p.ratio, 0.01, 32, 2),
            depth: clampTo(p.depth, 0, 24, 2),
            mod2: waveOrOff(p.mod2),
            ratio2: clampTo(p.ratio2, 0.01, 32, 4),
            depth2: clampTo(p.depth2, 0, 24, 1.4),
            decay2: clampTo(p.decay2, 0.02, 8, 0.12),
            mod3: waveOrOff(p.mod3),
            ratio3: clampTo(p.ratio3, 0.01, 32, 3),
            depth3: clampTo(p.depth3, 0, 24, 1),
            decay3: clampTo(p.decay3, 0.02, 8, 0.12),
            attack: clampTo(p.attack, 0, 5, 0.008),
            decay: clampTo(p.decay, 0, 12, 0.25),
            sustain: clampTo(p.sustain, 0, 1.5, 0.65),
            release: clampTo(p.release, 0, 30, 0.18),
            pitchEnv: clampTo(p.pitchEnv, 0.2, 4, 1),
            pitchTime: clampTo(p.pitchTime, 0.015, 0.6, 0.045),
            cutoff: clampTo(p.cutoff, 0.05, 1, 0.65),
            filterType: oneOf(FILTER_TYPES, p.filterType, "lowpass"),
            filterEnv: clampTo(p.filterEnv, -1, 1, 0),
            filterDecay: clampTo(p.filterDecay, 0.04, 2.5, 0.22),
            filterQ: clampTo(p.filterQ, 0.7, 15, 1),
            lfo: normalizeLfo(p.lfo)
        };
    }
    function compact(patch) {
        const out = { ...normalize(patch) };
        if (!out.mod) { delete out.mod; delete out.ratio; delete out.depth; }
        if (!out.mod2) { delete out.mod2; delete out.ratio2; delete out.depth2; delete out.decay2; }
        if (!out.mod3) { delete out.mod3; delete out.ratio3; delete out.depth3; delete out.decay3; }
        if (!out.second) delete out.second;
        if (!out.pulseWidth) delete out.pulseWidth;
        if (!out.sub) { delete out.sub; delete out.subWave; delete out.subOctave; }
        if (!out.noise) { delete out.noise; delete out.noiseWave; }
        if (Math.abs(out.pitchEnv - 1) < 0.001) { delete out.pitchEnv; delete out.pitchTime; }
        if (Math.abs(out.filterEnv) < 0.001) delete out.filterEnv;
        if (!out.lfo.depth) delete out.lfo;
        return out;
    }
    function clone(value) { return JSON.parse(JSON.stringify(value)); }

    let patches = [];
    function cleanName(value, fallback) {
        if (typeof value !== "string") return fallback;
        const name = value.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, MAX_NAME);
        return name || fallback;
    }
    function register() {
        const catalog = root.SynthStudioPresets;
        if (catalog && typeof catalog.setUser === "function") catalog.setUser(clone(patches));
    }
    function persist() {
        const payload = JSON.stringify(patches.map(({ id, name, patch }) => ({ id, name, patch })));
        try { root.localStorage?.setItem(STORAGE_KEY, payload); } catch (_) {}
    }
    function newId() {
        const cryptoApi = root.crypto;
        const random = cryptoApi && typeof cryptoApi.randomUUID === "function"
            ? cryptoApi.randomUUID()
            : `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 12)}`;
        return `user-${random}`.slice(0, 100);
    }

    function install() {
        let raw = [];
        try {
            const parsed = JSON.parse(root.localStorage?.getItem(STORAGE_KEY) || "[]");
            if (Array.isArray(parsed)) raw = parsed;
        } catch (_) {}
        const seen = new Set();
        patches = [];
        for (const entry of raw) {
            if (!entry || typeof entry !== "object") continue;
            if (typeof entry.id !== "string" || !USER_ID.test(entry.id) || seen.has(entry.id)) continue;
            seen.add(entry.id);
            patches.push({ id: entry.id, name: cleanName(entry.name, "Synth"), patch: normalize(entry.patch) });
            if (patches.length >= MAX_PATCHES) break;
        }
        register();
        return patches;
    }
    function update(id, name, patch) {
        const entry = patches.find(item => item.id === id);
        if (!entry) return null;
        entry.name = cleanName(name, entry.name);
        entry.patch = compact(patch);
        persist(); register();
        return clone(entry);
    }
    function create(name, patch) {
        if (patches.length >= MAX_PATCHES) return null;
        const entry = { id: newId(), name: cleanName(name, "Synth"), patch: compact(patch) };
        patches.push(entry);
        persist(); register();
        return clone(entry);
    }
    function remove(id) {
        const index = patches.findIndex(item => item.id === id);
        if (index < 0) return false;
        patches.splice(index, 1);
        persist(); register();
        return true;
    }
    function userPatches() { return clone(patches); }
    function isUserPatch(id) { return patches.some(item => item.id === id); }

    function describe(id) {
        const catalog = root.SynthStudioPresets;
        const preset = catalog && typeof catalog.get === "function" ? catalog.get(id) : null;
        if (id === "drums-kit" || id === "gm-drums" || (preset && preset.category === "drums")) {
            return { id, name: preset?.name || id, isUser: false, canEdit: false, patch: null };
        }
        const mine = patches.find(item => item.id === id);
        if (mine) return { id, name: mine.name, isUser: true, canEdit: true, patch: clone(mine.patch) };
        return { id, name: preset?.name || id, isUser: false, canEdit: true, patch: preset?.patch ? clone(preset.patch) : clone(DEFAULT_PATCH) };
    }

    // The preview instrument: a byId-only catalog entry, never listed and never
    // persisted. `live` refreshes a stored user patch in the catalog so a track
    // that plays it hears draft edits on its next notes without saving.
    function setPreview(patch) {
        const catalog = root.SynthStudioPresets;
        if (!catalog || typeof catalog.setRuntime !== "function") return;
        if (patch) catalog.setRuntime(PREVIEW_ID, { id: PREVIEW_ID, name: "Programmable Synth", category: "user", tags: [], synthUser: true, runtime: true, patch: normalize(patch) });
        else catalog.setRuntime(PREVIEW_ID, null);
    }
    function live(id, patch) {
        if (!isUserPatch(id) || !patch) return;
        const catalog = root.SynthStudioPresets;
        const preset = catalog?.get?.(id);
        if (preset) preset.patch = normalize(patch);
    }

    // --- editable field surface -----------------------------------------
    const FIELD_GROUPS = [
        { labelKey: "grp_oscillator", fields: [
            { key: "wave", type: "select", options: WAVES, optionKey: value => `wave_${value}`, labelKey: "param_wave" },
            { key: "level", type: "range", min: 0, max: 1.5, step: 0.005, labelKey: "param_level" },
            { key: "unison", type: "select", options: [1, 3, 5], optionKey: value => `unison_${value}`, labelKey: "param_unison" },
            { key: "detune", type: "range", min: 0, max: 120, step: 1, labelKey: "param_detune" },
            { key: "second", type: "range", min: 0, max: 1, step: 0.005, labelKey: "param_second" },
            { key: "pulseWidth", type: "range", min: 0, max: 0.95, step: 0.01, labelKey: "param_pulse_width" }
        ] },
        { labelKey: "grp_sub_noise", fields: [
            { key: "sub", type: "range", min: 0, max: 0.6, step: 0.005, labelKey: "param_sub" },
            { key: "subWave", type: "select", options: SUB_WAVES, optionKey: value => `wave_${value}`, labelKey: "param_sub_wave" },
            { key: "subOctave", type: "select", options: [1, 2], optionKey: value => `sub_octave_${value}`, labelKey: "param_sub_octave" },
            { key: "noise", type: "range", min: 0, max: 1, step: 0.005, labelKey: "param_noise" },
            { key: "noiseWave", type: "select", options: NOISES, optionKey: value => `wave_${value}`, labelKey: "param_noise_wave" }
        ] },
        { labelKey: "grp_fm", fields: [
            { key: "mod", type: "select", options: MOD_WAVES, off: true, optionKey: value => `wave_${value}`, labelKey: "param_mod" },
            { key: "ratio", type: "range", min: 0.25, max: 16, step: 0.05, labelKey: "param_ratio" },
            { key: "depth", type: "range", min: 0, max: 12, step: 0.05, labelKey: "param_depth" },
            { key: "mod2", type: "select", options: MOD_WAVES, off: true, optionKey: value => `wave_${value}`, labelKey: "param_mod2" },
            { key: "ratio2", type: "range", min: 0.25, max: 16, step: 0.05, labelKey: "param_ratio2" },
            { key: "depth2", type: "range", min: 0, max: 12, step: 0.05, labelKey: "param_depth2" },
            { key: "decay2", type: "range", min: 0.02, max: 4, step: 0.01, labelKey: "param_decay2" },
            { key: "mod3", type: "select", options: MOD_WAVES, off: true, optionKey: value => `wave_${value}`, labelKey: "param_mod3" },
            { key: "ratio3", type: "range", min: 0.25, max: 16, step: 0.05, labelKey: "param_ratio3" },
            { key: "depth3", type: "range", min: 0, max: 12, step: 0.05, labelKey: "param_depth3" },
            { key: "decay3", type: "range", min: 0.02, max: 4, step: 0.01, labelKey: "param_decay3" }
        ] },
        { labelKey: "grp_filter", fields: [
            { key: "filterType", type: "select", options: FILTER_TYPES, optionKey: value => `filter_${value}`, labelKey: "param_filter_type" },
            { key: "cutoff", type: "range", min: 0.05, max: 1, step: 0.005, labelKey: "param_cutoff" },
            { key: "filterQ", type: "range", min: 0.7, max: 12, step: 0.05, labelKey: "resonance" },
            { key: "filterEnv", type: "range", min: -1, max: 1, step: 0.01, labelKey: "param_filter_env" },
            { key: "filterDecay", type: "range", min: 0.04, max: 2.5, step: 0.01, labelKey: "param_filter_decay" }
        ] },
        { labelKey: "grp_envelope", fields: [
            { key: "attack", type: "range", min: 0, max: 4, step: 0.005, labelKey: "attack" },
            { key: "decay", type: "range", min: 0.01, max: 6, step: 0.01, labelKey: "param_decay" },
            { key: "sustain", type: "range", min: 0, max: 1.2, step: 0.005, labelKey: "param_sustain" },
            { key: "release", type: "range", min: 0.02, max: 6, step: 0.01, labelKey: "release" }
        ] },
        { labelKey: "grp_pitch", fields: [
            { key: "pitchEnv", type: "range", min: 0.25, max: 4, step: 0.01, labelKey: "param_pitch_env" },
            { key: "pitchTime", type: "range", min: 0.015, max: 0.6, step: 0.005, labelKey: "param_pitch_time" }
        ] },
        { labelKey: "grp_lfo", fields: [
            { key: "lfo.target", type: "select", options: LFO_TARGETS, optionKey: value => `lfo_${value}`, labelKey: "param_lfo_target" },
            { key: "lfo.wave", type: "select", options: LFO_WAVES, optionKey: value => `wave_${value}`, labelKey: "param_lfo_wave" },
            { key: "lfo.rate", type: "range", min: 0.05, max: 20, step: 0.05, labelKey: "param_lfo_rate" },
            { key: "lfo.depth", type: "range", min: 0, max: 1, step: 0.005, labelKey: "param_lfo_depth" },
            { key: "lfo.delay", type: "range", min: 0, max: 6, step: 0.01, labelKey: "param_lfo_delay" }
        ] }
    ];

    function readField(patch, key) {
        if (!key.includes(".")) return patch[key];
        const [section, field] = key.split(".");
        return patch[section] ? patch[section][field] : undefined;
    }
    function writeField(patch, key, value) {
        if (!key.includes(".")) { patch[key] = value; return; }
        const [section, field] = key.split(".");
        patch[section] = { ...(patch[section] || {}), [field]: value };
    }

    // --- editor -----------------------------------------------------------
    // createEditor({host, tr, esc, signal, state, blocked, onChange, onAction})
    // renders the parameter surface into `host`. `state` is the app's draft
    // object ({sourceId, sourceName, isUser, patch}); the editor mutates the
    // patch in place and reports every change through `onChange`. `onAction`
    // receives 'save' | 'saveAs' | 'remove' | 'reset' | 'close'.
    function createEditor(options) {
        const host = options.host;
        const tr = options.tr;
        const esc = options.esc;
        const state = options.state;
        const signal = options.signal;
        if (!host || !tr || !esc || !state || !state.patch) throw new Error("designer options incomplete");
        const patch = normalize(state.patch);
        Object.keys(state.patch).forEach(key => delete state.patch[key]);
        Object.assign(state.patch, patch);

        function fieldMarkup(field) {
            const value = readField(state.patch, field.key);
            const label = esc(tr(field.labelKey));
            if (field.type === "select") {
                const options = field.off ? ["", ...field.options] : field.options;
                const body = options.map(option => {
                    const optionValue = String(option);
                    const text = option === "" ? tr("fx_off") : tr(field.optionKey(option));
                    return `<option value="${esc(optionValue)}"${optionValue === String(value) ? " selected" : ""}>${esc(text)}</option>`;
                }).join("");
                const inLfo = field.key.includes(".");
                return `<label class="ss-designer-field"><span>${label}</span><select data-designer-param="${esc(field.key)}" aria-label="${label}">${body}</select></label>`;
            }
            return `<label class="ss-designer-field"><span>${label}</span><input data-designer-param="${esc(field.key)}" type="range" min="${field.min}" max="${field.max}" step="${field.step}" value="${value}" aria-label="${label}"><output>${Number(value).toFixed(2)}</output></label>`;
        }
        function render() {
            const disabled = options.blocked && options.blocked();
            const groups = FIELD_GROUPS.map(group => `<section class="ss-designer-group"><h4>${esc(tr(group.labelKey))}</h4><div class="ss-param-grid">${group.fields.map(fieldMarkup).join("")}</div></section>`).join("");
            const keys = [60, 62, 64, 65, 67, 69, 71, 72];
            host.innerHTML = `<section class="ss-designer" data-designer>` +
                `<header class="ss-designer-head"><h3>${esc(tr("designer"))}</h3>` +
                (state.isUser ? `<input data-designer-name type="text" inputmode="text" enterkeyhint="done" maxlength="${MAX_NAME}" value="${esc(state.name)}" aria-label="${esc(tr("designer_name"))}">` : `<span class="ss-designer-source">${esc(state.sourceName || tr("designer"))}</span>`) +
                `</header><p class="ss-help">${esc(tr("designer_hint"))}</p>` +
                groups +
                `<div class="ss-mini-keys">${keys.map(pitch => `<button data-designer-note="${pitch}" aria-label="${esc(tr("preview"))}">♪</button>`).join("")}</div>` +
                `<footer class="ss-designer-actions">` +
                (state.isUser ? `<button type="button" data-designer-action="save"${disabled ? " disabled" : ""}>${esc(tr("designer_update"))}</button>` : "") +
                `<button type="button" data-designer-action="saveAs"${disabled ? " disabled" : ""}>${esc(tr("designer_save_as"))}</button>` +
                (state.isUser ? `<button type="button" data-designer-action="remove"${disabled ? " disabled" : ""}>${esc(tr("designer_delete"))}</button>` : "") +
                `<button type="button" data-designer-action="reset">${esc(tr("designer_reset"))}</button>` +
                `<button type="button" data-designer-action="close">${esc(tr("designer_close"))}</button>` +
                `</footer></section>`;
        }
        render();

        const listeners = [];
        function listen(target, type, handler) {
            target.addEventListener(type, handler, { signal });
            listeners.push([target, type, handler]);
        }
        function commit() {
            if (typeof options.onChange === "function") options.onChange();
        }
        listen(host, "input", event => {
            const control = event.target.closest("[data-designer-param]");
            if (!control) return;
            const field = FIELD_GROUPS.flatMap(group => group.fields).find(item => item.key === control.dataset.designerParam);
            if (!field) return;
            const value = field.type === "select" ? control.value : Number(control.value);
            writeField(state.patch, field.key, value);
            const output = control.parentElement.querySelector("output");
            if (output) output.textContent = Number(value).toFixed(2);
            commit();
        });
        listen(host, "change", event => {
            const control = event.target.closest("[data-designer-param]");
            if (!control) return;
            const field = FIELD_GROUPS.flatMap(group => group.fields).find(item => item.key === control.dataset.designerParam);
            if (!field || field.type !== "select") return;
            writeField(state.patch, field.key, control.value);
            commit();
        });
        listen(host, "input", event => {
            const name = event.target.closest("[data-designer-name]");
            if (!name) return;
            state.name = name.value;
        });
        listen(host, "click", event => {
            const action = event.target.closest("[data-designer-action]");
            if (action) {
                if (typeof options.onAction === "function") options.onAction(action.dataset.designerAction, action);
                return;
            }
            const note = event.target.closest("[data-designer-note]");
            if (note && typeof options.onPreview === "function") options.onPreview(Number(note.dataset.designerNote));
        });
        return {
            refresh() { render(); },
            dispose() {
                for (const [target, type, handler] of listeners) target.removeEventListener(type, handler);
                listeners.length = 0;
            }
        };
    }

    root.SynthStudioDesigner = {
        PREVIEW_ID,
        MAX_PATCHES,
        MAX_NAME,
        USER_ID,
        NORMALIZE_FIELDS: Object.keys(DEFAULT_PATCH),
        install,
        create,
        update,
        remove,
        userPatches,
        isUserPatch,
        describe,
        setPreview,
        live,
        normalize,
        compact,
        createEditor
    };
})(typeof window !== "undefined" ? window : globalThis);
