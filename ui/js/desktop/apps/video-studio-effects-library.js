(function () {
    'use strict';

    // Effects library tab and LUT media (2c, spec §8.4). VideoStudioEffectsLibrary is DOM-free (Node-tested): the
    // registry's effects and the built-in looks by category, item lookup, and the LUT helpers of the media bin. The app
    // parts below render the tab ([data-effects-library]), wire drags (MIME application/x-aurago-vs-effect, a delegated
    // drop on [data-timeline]) and double-clicks or keyboard clicks (add to the selected clip), apply a LUT asset dropped
    // on a clip, and explain LUT upload errors.
    const P = window.VideoStudioParts = window.VideoStudioParts || {};
    const FX = () => window.VideoStudioEffects;
    const MIME = 'application/x-aurago-vs-effect';
    const MAX_LUT_BYTES = 16 * 1024 * 1024; // Go videostudio.MaxLUTBytes (Node check)
    const LOOK = 'look:';

    // Categories in registry order (an effect's first appearance places its category); the built-in looks join the
    // colour category directly after the lut effect. Each item is a fresh object: {key, effect, params, label_key,
    // category, available, look?}, key being the effect id or 'look:<id>'.
    function model() {
        const groups = [], byCategory = new Map();
        const specs = typeof FX().effects === 'function' ? FX().effects() : [];
        specs.forEach(spec => {
            if (!byCategory.has(spec.category)) {
                const created = { category: spec.category, label_key: 'effectCategory_' + spec.category, items: [] };
                byCategory.set(spec.category, created);
                groups.push(created);
            }
            const group = byCategory.get(spec.category), available = !!spec.available;
            group.items.push({ key: spec.id, effect: spec.id, params: {}, label_key: spec.label_key, category: spec.category, available });
            if (spec.id === 'lut') FX().looks().forEach(look => group.items.push({ key: LOOK + look.id, effect: 'lut', params: { lut: 'builtin:' + look.id }, label_key: look.label_key, category: spec.category, available, look: look.id }));
        });
        return groups;
    }
    function item(key) {
        const text = String(key == null ? '' : key);
        for (const group of model()) {
            const found = group.items.find(entry => entry.key === text);
            if (found) return found;
        }
        return null;
    }
    // .cube files are recognized by their extension: browsers report no MIME type for them.
    function isLUTFile(name) { return /\.cube$/i.test(String(name || '')); }
    // Every lut effect entry, on or off, that uses one of the asset ids: [{track, clip, index}] in track, clip and stack
    // order.
    function lutRefs(project, ids) {
        const wanted = ids instanceof Set ? ids : new Set(ids || []), out = [];
        ((project && project.tracks) || []).forEach(track => (track.clips || []).forEach(clip => (Array.isArray(clip.effects) ? clip.effects : []).forEach((entry, index) => {
            if (entry && entry.id === 'lut' && entry.params && wanted.has(entry.params.lut)) out.push({ track, clip, index });
        })));
        return out;
    }
    // Removes those entries in place (a clip left without effects loses the field) and returns how many went.
    function removeLUTRefs(project, ids) {
        const refs = lutRefs(project, ids);
        refs.slice().reverse().forEach(ref => {
            ref.clip.effects.splice(ref.index, 1);
            if (!ref.clip.effects.length) delete ref.clip.effects;
        });
        return refs.length;
    }
    const Lib = window.VideoStudioEffectsLibrary = { MIME, MAX_LUT_BYTES, model, item, isLUTFile, lutRefs, removeLUTRefs };

    const T = (...args) => P.T(...args);
    const esc = (...args) => P.esc(...args);
    const icon = (...args) => P.icon(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);
    const fill = (text, vars) => Object.keys(vars).reduce((out, key) => out.split('{{' + key + '}}').join(String(vars[key])), text);
    const UNAVAILABLE = ['effectsLibraryUnavailable', "Not available: this server's FFmpeg lacks this effect"];
    const NEEDS_CLIP = ['effectsLibraryNeedsClip', 'Select a picture clip first, or drop the effect on one.'];
    const DROP_ON_CLIP = ['effectsLibraryDropOnClip', 'Drop it on a picture clip.'];

    // The tab: an intro, then one group per category. Items are buttons with aria-disabled (not disabled), so an
    // unavailable one keeps its title for pointer and keyboard users.
    function renderEffectsLibrary(s) {
        const host = s.q && s.q('[data-effects-library]');
        if (!host) return;
        const off = !!s.readonly || !s.project, unavailable = tr(s, UNAVAILABLE[0], UNAVAILABLE[1]);
        const groups = Lib.model().map(group => {
            const heading = 'vs-fxlib-' + s.id + '-' + group.category;
            const items = group.items.map(entry => {
                const name = tr(s, entry.label_key, entry.key), disabled = off || !entry.available;
                const glyph = entry.effect === 'lut' ? 'lut' : entry.effect === 'color' ? 'sliders' : 'effects';
                return `<li><button type="button" class="vs-fxlib-item" data-effect-item="${esc(s, entry.key)}" draggable="${!disabled}" aria-disabled="${disabled}" title="${esc(s, entry.available ? name : name + ' (' + unavailable + ')')}">${icon(glyph, 16)}<span>${esc(s, name)}</span></button></li>`;
            }).join('');
            return `<section class="vs-fxlib-group" aria-labelledby="${esc(s, heading)}"><h3 id="${esc(s, heading)}">${esc(s, tr(s, group.label_key, group.category))}</h3><ul class="vs-fxlib-list">${items}</ul></section>`;
        }).join('');
        const markup = `<p class="vs-panel-intro">${esc(s, tr(s, 'effectsLibraryIntro', 'Drag an effect onto a clip in the timeline, or double-click it to add it to the selected clip.'))}</p>${groups}`;
        // renderUI calls this after every edit; unchanged markup is not redrawn, so focus and hover stay.
        if (host._vsMarkup === markup) return;
        const active = typeof document !== 'undefined' && host.contains(document.activeElement) ? document.activeElement.dataset.effectItem : '';
        host._vsMarkup = markup;
        host.innerHTML = markup;
        if (active) host.querySelector(`[data-effect-item="${CSS.escape(active)}"]`)?.focus({ preventScroll: true });
    }
    function dropTarget(s, clipId) {
        const found = s.project && clipId ? T().clipFor(s.project, clipId) : null;
        return found && found.track.kind !== 'audio' && !found.track.locked ? found : null;
    }
    // Adds one effect entry to a visual clip in one undo step (VideoStudioKeyframeEdit.addEffect: complete params with
    // the item's values on top) and selects the clip, so the inspector shows the new entry.
    function addEffectTo(s, clipId, effectId, params) {
        if (!s.project) return false;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return false; }
        const found = clipId ? T().clipFor(s.project, clipId) : null;
        if (!found || found.track.kind === 'audio') { showNotice(s, NEEDS_CLIP[0], NEEDS_CLIP[1], false); return false; }
        const limit = (FX().LIMITS && FX().LIMITS.effectsPerClip) || 8;
        if ((found.clip.effects || []).length >= limit) { showNotice(s, 'effectsLibraryFull', 'This clip already has 8 effects. Remove one first.', false); return false; }
        const ok = P.applyEdit(s, 'add effect', project => window.VideoStudioKeyframeEdit.addEffect(project, found.clip.id, effectId, Object.assign({}, params || {})), () => [found.clip.id]);
        if (ok && s.app && s.app.getBoundingClientRect().width <= 920 && P.setInspectorOpen) P.setInspectorOpen(s, true, false);
        return ok;
    }
    function addLibraryItem(s, clipId, entry) {
        if (!entry) return false;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return false; }
        if (!entry.available) { showNotice(s, UNAVAILABLE[0], UNAVAILABLE[1], false); return false; }
        return addEffectTo(s, clipId, entry.effect, entry.params);
    }
    // A LUT asset dropped on a timeline clip (or added with its + to the selected clip) adds a lut effect with it.
    function dropLUTAsset(s, asset, trackId, at) {
        let clipId = s.selectedClipId;
        if (trackId) {
            const track = s.project.tracks.find(entry => entry.id === trackId), frame = Math.floor(Number(at));
            const hit = track && Number.isFinite(frame) ? track.clips.find(clip => frame >= clip.start && frame < clip.start + clip.duration) : null;
            if (!hit) { showNotice(s, DROP_ON_CLIP[0], DROP_ON_CLIP[1], false); return false; }
            clipId = hit.id;
        }
        return addEffectTo(s, clipId, 'lut', { lut: asset.id });
    }
    // .cube upload errors (multipart or Desktop file): true when the body was a LUT error and a notice explains it.
    function lutUploadNotice(s, body) {
        const code = body && body.code;
        if (code === 'invalid_lut') {
            // One notice with the line (Task 14's lutInvalid); ParseCube errors always name one, the end of the file counting as
            // the line after the last.
            const line = Math.max(1, Math.floor(Number(body.line) || 0));
            showNotice(s, '', fill(tr(s, 'lutInvalid', 'This .cube file cannot be used: line {{line}} is not valid.'), { line }), true);
            return true;
        }
        if (code === 'lut_size_limit') { showNotice(s, 'lutTooLarge', 'A .cube file may be at most 16 MB.', true); return true; }
        if (code === 'lut_count_limit') { showNotice(s, 'lutCountLimit', 'A project holds at most 32 LUTs. Remove one first.', true); return true; }
        return false;
    }
    function wireEffectsLibrary(s) {
        const panel = s.q('[data-effects-library]'), timeline = s.q('[data-timeline]'), signal = s.abort.signal;
        if (!panel || !timeline) return;
        renderEffectsLibrary(s);
        const itemEl = event => (event.target && typeof event.target.closest === 'function' ? event.target.closest('[data-effect-item]') : null);
        const clipEl = event => (event.target && typeof event.target.closest === 'function' ? event.target.closest('[data-clip-id]') : null);
        panel.addEventListener('dragstart', event => {
            const el = itemEl(event);
            if (!el) return;
            const entry = el.getAttribute('aria-disabled') === 'true' ? null : Lib.item(el.dataset.effectItem);
            if (!entry) { event.preventDefault(); return; }
            event.dataTransfer.setData(MIME, entry.key);
            event.dataTransfer.effectAllowed = 'copy';
        }, { signal });
        // A double-click adds to the selected clip, and so does a keyboard click (Enter or Space give detail 0). A single
        // mouse click only focuses the item, so a double-click adds once.
        const activate = event => { const el = itemEl(event); if (el) addLibraryItem(s, s.selectedClipId, Lib.item(el.dataset.effectItem)); };
        panel.addEventListener('dblclick', activate, { signal });
        panel.addEventListener('click', event => { if (event.detail === 0) activate(event); }, { signal });
        const effectDrag = event => Array.from(event.dataTransfer && event.dataTransfer.types || []).includes(MIME);
        const clear = () => timeline.querySelectorAll('.vs-clip.is-effect-drop').forEach(el => el.classList.remove('is-effect-drop'));
        // The timeline's own dragover runs first (it was attached first) and offers a move; an effect is a copy onto the
        // clip under the pointer.
        timeline.addEventListener('dragover', event => {
            if (!effectDrag(event)) return;
            clear();
            event.preventDefault();
            const clip = clipEl(event), ok = !!clip && !s.readonly && !!dropTarget(s, clip.dataset.clipId);
            event.dataTransfer.dropEffect = ok ? 'copy' : 'none';
            if (ok) clip.classList.add('is-effect-drop');
        }, { signal });
        timeline.addEventListener('dragleave', event => { if (!timeline.contains(event.relatedTarget)) clear(); }, { signal });
        timeline.addEventListener('drop', event => {
            if (!effectDrag(event)) return;
            clear();
            event.preventDefault();
            const clip = clipEl(event), entry = Lib.item(event.dataTransfer.getData(MIME));
            if (!entry) return;
            if (!clip) { showNotice(s, DROP_ON_CLIP[0], DROP_ON_CLIP[1], false); return; }
            addLibraryItem(s, clip.dataset.clipId, entry);
        }, { signal });
        window.addEventListener('dragend', clear, { signal });
    }

    Object.assign(P, { renderEffectsLibrary, wireEffectsLibrary, addEffectTo, addLibraryItem, dropLUTAsset, lutUploadNotice });
})();
