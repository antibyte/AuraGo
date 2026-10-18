(function () {
    'use strict';

    // Effects library tab and LUT media (2c, spec §8.4) with the transitions (2e, plan index §4.9).
    // - VideoStudioEffectsLibrary is DOM-free (Node-tested): the registry's effects and the built-in looks by category,
    //   then the transitions by category, item lookup, the cut a transition drop takes (cutAt), and the LUT helpers of
    //   the media bin.
    // - The app parts below render the tab ([data-effects-library]) and wire drags: MIME application/x-aurago-vs-effect
    //   for effects and application/x-aurago-vs-transition for transitions, with delegated drops on [data-timeline].
    // - A double-click or keyboard click adds an item to the selected clip.
    // - The parts also apply a LUT asset dropped on a clip and explain LUT upload errors.
    const P = window.VideoStudioParts = window.VideoStudioParts || {};
    const FX = () => window.VideoStudioEffects;
    const MIME = 'application/x-aurago-vs-effect';
    // Transitions drag with a type of their own: dragover sees only the types, and a transition marks a cut, not a clip.
    const TRANSITION_MIME = 'application/x-aurago-vs-transition';
    const MAX_LUT_BYTES = 16 * 1024 * 1024; // Go videostudio.MaxLUTBytes (Node check)
    const LOOK = 'look:';
    const TRANSITION = 'transition:';

    // Groups and items:
    // - Categories come in registry order (an effect's first appearance places its category). The built-in looks join
    //   the colour category directly after the lut effect.
    // - One group per transition category follows the effect groups (VideoStudioEffects.transitionGroups); those groups
    //   carry kind 'transition'.
    // - Each item is a fresh object: {key, effect, params, label_key, category, available, look?} for effects and looks,
    //   {key, transition, label_key, category, available} for transitions. key is the effect id, 'look:<id>' or
    //   'transition:<id>'.
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
        const transitions = typeof FX().transitionGroups === 'function' ? FX().transitionGroups() : [];
        transitions.forEach(group => groups.push({ category: group.category, label_key: group.label_key, kind: 'transition', items: group.items.map(spec => ({ key: TRANSITION + spec.id, transition: spec.id, label_key: spec.label_key, category: group.category, available: !!spec.available })) }));
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
    // The cut that a transition dropped on one half of a clip takes (index §4.9):
    // - side 'before' is the clip's cut with the clip before it, 'after' its cut with the clip after it, on the same
    //   visual track (clips in start order);
    // - a cut exists when the two clips touch or already overlap through the outgoing clip's transition; the result is
    //   {out, inc}, the outgoing and the incoming clip;
    // - a gap, the track's ends, an audio track, an unknown clip or an unknown side give null.
    function cutAt(project, trackId, clipId, side) {
        const track = ((project && project.tracks) || []).find(entry => entry.id === trackId);
        if (!track || track.kind === 'audio' || (side !== 'before' && side !== 'after')) return null;
        const order = (track.clips || []).slice().sort((a, b) => a.start - b.start), index = order.findIndex(clip => clip.id === clipId);
        if (index < 0) return null;
        const out = order[side === 'before' ? index - 1 : index], inc = order[side === 'before' ? index : index + 1];
        return out && inc && out.start + out.duration >= inc.start ? { out, inc } : null;
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
    const Lib = window.VideoStudioEffectsLibrary = { MIME, TRANSITION_MIME, MAX_LUT_BYTES, model, item, cutAt, isLUTFile, lutRefs, removeLUTRefs };

    const I = (...args) => P.I(...args);
    const T = (...args) => P.T(...args);
    const esc = (...args) => P.esc(...args);
    const icon = (...args) => P.icon(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);
    const fill = (text, vars) => Object.keys(vars).reduce((out, key) => out.split('{{' + key + '}}').join(String(vars[key])), text);
    const UNAVAILABLE = ['effectsLibraryUnavailable', "Not available: this server's FFmpeg lacks this effect"];
    const TRANSITION_UNAVAILABLE = ['transitionUnavailable', "Not available: this server's FFmpeg lacks this transition"];
    const NEEDS_CLIP = ['effectsLibraryNeedsClip', 'Select a picture clip first, or drop the effect on one.'];
    const DROP_ON_CLIP = ['effectsLibraryDropOnClip', 'Drop it on a picture clip.'];
    const DROP_ON_CUT = ['effectsLibraryDropOnCut', 'Drop a transition on the cut between two clips on one track.'];
    // Generated transition icons (VideoStudioIcons.transition, 30×18); none without the icon module (Node checks).
    const transitionIcon = id => (window.VideoStudioIcons && window.VideoStudioIcons.transition ? window.VideoStudioIcons.transition(id, 30) : '');

    // The tab: an intro, then one group per category, with the transition groups last. Items are buttons with
    // aria-disabled (not disabled), so an unavailable one keeps its title for pointer and keyboard users.
    function renderEffectsLibrary(s) {
        const host = s.q && s.q('[data-effects-library]');
        if (!host) return;
        const off = !!s.readonly || !s.project;
        const groups = Lib.model().map(group => {
            const transitions = group.kind === 'transition', reason = transitions ? TRANSITION_UNAVAILABLE : UNAVAILABLE, unavailable = tr(s, reason[0], reason[1]);
            const heading = 'vs-fxlib-' + s.id + '-' + (transitions ? 'tx-' : '') + group.category;
            const items = group.items.map(entry => {
                const name = tr(s, entry.label_key, entry.key), disabled = off || !entry.available;
                const glyph = entry.transition ? transitionIcon(entry.transition) : icon(entry.effect === 'lut' ? 'lut' : entry.effect === 'color' ? 'sliders' : entry.effect === 'chroma_key' ? 'key' : 'effects', 16);
                return `<li><button type="button" class="vs-fxlib-item${entry.transition ? ' vs-fxlib-transition' : ''}" data-effect-item="${esc(s, entry.key)}" draggable="${!disabled}" aria-disabled="${disabled}" title="${esc(s, entry.available ? name : name + ' (' + unavailable + ')')}">${glyph}<span>${esc(s, name)}</span></button></li>`;
            }).join('');
            return `<section class="vs-fxlib-group" aria-labelledby="${esc(s, heading)}"><h3 id="${esc(s, heading)}">${esc(s, tr(s, group.label_key, group.category))}</h3><ul class="vs-fxlib-list">${items}</ul></section>`;
        }).join('');
        const markup = `<p class="vs-panel-intro">${esc(s, tr(s, 'effectsLibraryIntro', 'Drag an effect onto a clip, or a transition onto the cut between two clips. Double-click adds it to the selected clip.'))}</p>${groups}`;
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
    // Adds one effect entry to a visual clip in one undo step (VideoStudioInspectorEffects.addEntry: complete params with
    // the item's values on top) and selects the clip, so the inspector shows the new entry. A chroma_key entry goes to stack index 0.
    function addEffectTo(s, clipId, effectId, params) {
        if (!s.project) return false;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return false; }
        const found = clipId ? T().clipFor(s.project, clipId) : null;
        if (!found || found.track.kind === 'audio') { showNotice(s, NEEDS_CLIP[0], NEEDS_CLIP[1], false); return false; }
        const limit = (FX().LIMITS && FX().LIMITS.effectsPerClip) || 8;
        if ((found.clip.effects || []).length >= limit) { showNotice(s, 'effectsLibraryFull', 'This clip already has 8 effects. Remove one first.', false); return false; }
        const ok = P.applyEdit(s, 'add effect', project => window.VideoStudioInspectorEffects.addEntry(project, found.clip.id, effectId, Object.assign({}, params || {})), () => [found.clip.id]);
        if (ok && s.app && s.app.getBoundingClientRect().width <= 920 && P.setInspectorOpen) P.setInspectorOpen(s, true, false);
        return ok;
    }
    // Sets a library transition on the cut after the outgoing clip (index §4.9) in one undo step through
    // P.applyTransition. That path checks the following clip and the length, and selects the outgoing clip. An existing
    // transition keeps its length; a new one gets the inspector's default (VideoStudioInspector.transitionLength).
    function addTransitionTo(s, clipId, type) {
        if (!s.project) return false;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return false; }
        const found = clipId ? T().clipFor(s.project, clipId) : null;
        if (!found || found.track.kind === 'audio') { showNotice(s, NEEDS_CLIP[0], NEEDS_CLIP[1], false); return false; }
        if (found.track.locked) { P.showReason(s, 'locked'); return false; }
        const order = found.track.clips.slice().sort((a, b) => a.start - b.start), next = order[order.indexOf(found.clip) + 1] || null;
        const ok = !!P.applyTransition(s, type, I().transitionLength(s.project, found.clip, next).value, found.clip.id);
        if (ok && s.app && s.app.getBoundingClientRect().width <= 920 && P.setInspectorOpen) P.setInspectorOpen(s, true, false);
        return ok;
    }
    function addLibraryItem(s, clipId, entry) {
        if (!entry) return false;
        if (s.readonly) { showNotice(s, 'readonly', 'Read-only mode prevents editing.', true); return false; }
        const reason = entry.transition ? TRANSITION_UNAVAILABLE : UNAVAILABLE;
        if (!entry.available) { showNotice(s, reason[0], reason[1], false); return false; }
        return entry.transition ? addTransitionTo(s, clipId, entry.transition) : addEffectTo(s, clipId, entry.effect, entry.params);
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
            event.dataTransfer.setData(entry.transition ? TRANSITION_MIME : MIME, entry.key);
            event.dataTransfer.effectAllowed = 'copy';
        }, { signal });
        // A double-click adds to the selected clip, and so does a keyboard click (Enter or Space give detail 0). A single
        // mouse click only focuses the item, so a double-click adds once.
        const activate = event => { const el = itemEl(event); if (el) addLibraryItem(s, s.selectedClipId, Lib.item(el.dataset.effectItem)); };
        panel.addEventListener('dblclick', activate, { signal });
        panel.addEventListener('click', event => { if (event.detail === 0) activate(event); }, { signal });
        const dragging = (event, type) => Array.from(event.dataTransfer && event.dataTransfer.types || []).includes(type);
        const clear = () => timeline.querySelectorAll('.vs-clip.is-effect-drop, .vs-clip.is-cut-drop-before, .vs-clip.is-cut-drop-after').forEach(el => el.classList.remove('is-effect-drop', 'is-cut-drop-before', 'is-cut-drop-after'));
        // The cut under the pointer for a transition (index §4.9): the hovered clip's first half is its cut with the clip
        // before it, the second half its cut with the clip after it (Lib.cutAt), or null.
        const cutUnder = event => {
            const el = clipEl(event), found = el && s.project ? T().clipFor(s.project, el.dataset.clipId) : null;
            if (!found) return null;
            const rect = el.getBoundingClientRect(), side = event.clientX < rect.left + rect.width / 2 ? 'before' : 'after';
            const cut = Lib.cutAt(s.project, found.track.id, found.clip.id, side);
            return cut ? { el, side, cut, locked: !!found.track.locked } : null;
        };
        // The timeline's own dragover runs first (it was attached first) and offers a move. An effect is a copy onto the
        // clip under the pointer, and a transition a copy onto the cut nearest to the pointer.
        timeline.addEventListener('dragover', event => {
            const effect = dragging(event, MIME), transition = dragging(event, TRANSITION_MIME);
            if (!effect && !transition) return;
            clear();
            event.preventDefault();
            if (transition) {
                const hit = s.readonly ? null : cutUnder(event), ok = !!hit && !hit.locked;
                event.dataTransfer.dropEffect = ok ? 'copy' : 'none';
                if (ok) hit.el.classList.add('is-cut-drop-' + hit.side);
                return;
            }
            const clip = clipEl(event), ok = !!clip && !s.readonly && !!dropTarget(s, clip.dataset.clipId);
            event.dataTransfer.dropEffect = ok ? 'copy' : 'none';
            if (ok) clip.classList.add('is-effect-drop');
        }, { signal });
        timeline.addEventListener('dragleave', event => { if (!timeline.contains(event.relatedTarget)) clear(); }, { signal });
        timeline.addEventListener('drop', event => {
            const effect = dragging(event, MIME), transition = dragging(event, TRANSITION_MIME);
            if (!effect && !transition) return;
            clear();
            event.preventDefault();
            if (transition) {
                const entry = Lib.item(event.dataTransfer.getData(TRANSITION_MIME)), hit = cutUnder(event);
                if (!entry || !entry.transition) return;
                if (!hit && !s.readonly) { showNotice(s, DROP_ON_CUT[0], DROP_ON_CUT[1], false); return; }
                addLibraryItem(s, hit ? hit.cut.out.id : '', entry);
                return;
            }
            const clip = clipEl(event), entry = Lib.item(event.dataTransfer.getData(MIME));
            if (!entry) return;
            if (!clip) { showNotice(s, DROP_ON_CLIP[0], DROP_ON_CLIP[1], false); return; }
            addLibraryItem(s, clip.dataset.clipId, entry);
        }, { signal });
        window.addEventListener('dragend', clear, { signal });
    }

    Object.assign(P, { renderEffectsLibrary, wireEffectsLibrary, addEffectTo, addTransitionTo, addLibraryItem, dropLUTAsset, lutUploadNotice });
})();
