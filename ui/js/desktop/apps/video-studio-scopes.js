(function () {
    'use strict';

    // The scopes panel (2f; spec §6.7, §8.5, §8.6; plan index §4.26.2, §4.26.5): histogram, waveform and vectorscope of
    // the stage preview.
    // - P.wireScopes(s) runs once, from appRender, and inserts two things:
    //   - the toggle [data-action="scopes"] in the transport, directly before "Render range exactly";
    //   - the dock .vs-scopes-dock, directly after the timeline stage.
    //   The dock carries data-stage="timeline", so source mode hides it with the stage. Its section [data-scopes]
    //   carries the open state. Without a project the toggle is disabled, and the CSS hides the dock.
    // - s.scopes = {open, show: Set of kinds, last, pending, dirty} is view state: never an undo step and never saved
    //   with the project. It is remembered per viewer in localStorage aurago.videoStudio.scopes
    //   (VideoStudioScopeMath.prefs).
    // - Readback (§4.26.2): the preview's onPaint hook calls P.scopesAfterPaint. While the panel is open and the timeline
    //   stage shows, a paint reads when VideoStudioScopeMath.due says so: every paint while paused, and at most every
    //   100 ms while playing.
    //   - One read is in flight at a time. A due paint during it sets dirty, and one trailing read of the last frame
    //     follows when it resolves.
    //   - Opening reads at once. A closed panel reads and analyses nothing, and the panel never paints.
    // - Keys: Escape inside the panel closes it and focuses the toggle (P.handleScopeKeys runs after handleRangeKeys).
    //   Every control is a native button. There is no shortcut.
    const P = window.VideoStudioParts;
    const esc = (...args) => P.esc(...args);
    const icon = (...args) => P.icon(...args);
    const tr = (...args) => P.tr(...args);
    const M = () => window.VideoStudioScopeMath;
    const PREF_SCOPES = 'aurago.videoStudio.scopes';
    // Keys that stay with the focused tile row, so it scrolls sideways instead of moving the playhead.
    const SCROLL_KEYS = new Set(['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End', 'PageUp', 'PageDown']);
    // Per kind: [short name key, English, canvas label key, English].
    const LABELS = Object.freeze({
        histogram: ['scopeHistogram', 'Histogram', 'scopeHistogramLabel', 'Histogram of the preview: red, green, blue and luma (BT.709), dark on the left, bright on the right'],
        waveform: ['scopeWaveform', 'Waveform', 'scopeWaveformLabel', 'Waveform of the preview: luma (BT.709) of each column from left to right, 0 % at the bottom, 100 % at the top'],
        vectorscope: ['scopeVectorscope', 'Vectorscope', 'scopeVectorscopeLabel', 'Vectorscope of the preview: colour as BT.709 Cb to the right and Cr upwards, with the skin-tone line']
    });
    const now = () => (typeof performance !== 'undefined' && typeof performance.now === 'function' ? performance.now() : Date.now());
    const sizeOf = kind => (kind === 'histogram' ? M().HISTOGRAM : kind === 'waveform' ? M().WAVEFORM : M().VECTORSCOPE);
    const imageOf = (kind, analysis) => (kind === 'histogram' ? M().histogramImage(analysis) : kind === 'waveform' ? M().waveformImage(analysis) : M().vectorscopeImage(analysis));
    function savePrefs(s) { P.writePref(PREF_SCOPES, JSON.stringify({ open: s.scopes.open, show: M().KINDS.filter(kind => s.scopes.show.has(kind)) })); }

    // Inserts the toggle and the dock and reads the stored preference. Returns false when the panel is wired already, or
    // when the layout lacks the transport or the stage.
    function wireScopes(s) {
        if (!s || s.scopes || !M() || typeof s.q !== 'function') return false;
        const range = s.q('.vs-transport-right [data-action="range-render"]'), stage = s.q('.vs-preview-stage[data-stage="timeline"]');
        if (!range || !stage) return false;
        const label = (key, fallback) => esc(s, tr(s, key, fallback)), id = esc(s, 'vs-scopes-' + s.id);
        range.insertAdjacentHTML('beforebegin', `<button type="button" data-action="scopes" aria-expanded="false" aria-controls="${id}" aria-label="${label('scopes', 'Scopes')}" title="${label('scopesHint', 'Show the histogram, waveform and vectorscope of the preview')}">${icon('scopes', 17)}</button>`);
        const buttons = M().KINDS.map(kind => `<button type="button" data-scope="${kind}" aria-pressed="true">${label(LABELS[kind][0], LABELS[kind][1])}</button>`).join('');
        const tiles = M().KINDS.map(kind => `<figure class="vs-scope" data-scope-tile="${kind}"><canvas data-scope-canvas="${kind}" width="${sizeOf(kind).width}" height="${sizeOf(kind).height}" role="img" aria-label="${label(LABELS[kind][2], LABELS[kind][3])}"></canvas></figure>`).join('');
        stage.insertAdjacentHTML('afterend', `<div class="vs-scopes-dock" data-stage="timeline"><section class="vs-scopes" id="${id}" data-scopes role="region" aria-label="${label('scopes', 'Scopes')}" hidden><div class="vs-segmented vs-scopes-bar" role="group" aria-label="${label('scopesShow', 'Scopes to show')}">${buttons}</div><div class="vs-scopes-tiles" role="group" aria-label="${label('scopes', 'Scopes')}" tabindex="0">${tiles}</div></section></div>`);
        const stored = M().prefs(P.readPref(PREF_SCOPES));
        s.scopes = { open: stored.open, show: new Set(stored.show), last: 0, pending: false, dirty: false };
        syncScopes(s);
        return true;
    }

    // Keeps the toggle, the section, the kind buttons and the tiles in step with s.scopes and the project. renderUI
    // calls it on every render.
    function syncScopes(s) {
        const sc = s && s.scopes;
        if (!sc || typeof s.q !== 'function') return;
        const open = sc.open && !!s.project, toggle = s.q('[data-action="scopes"]'), section = s.q('[data-scopes]');
        if (toggle) { toggle.disabled = !s.project; toggle.setAttribute('aria-expanded', String(open)); }
        if (section) section.hidden = !open;
        M().KINDS.forEach(kind => {
            const shown = sc.show.has(kind), button = s.q(`[data-scope="${kind}"]`), tile = s.q(`[data-scope-tile="${kind}"]`);
            if (button) button.setAttribute('aria-pressed', String(shown));
            if (tile) tile.hidden = !shown;
        });
    }

    // Opens or closes the panel; returns true when the state changed.
    // - Opening needs a project and reads the last frame at once, without a paint.
    // - Closing drops a trailing read.
    // - options.focus moves focus to the toggle (Escape).
    function setScopesOpen(s, open, options) {
        const sc = s && s.scopes, next = !!open;
        if (!sc || (next && !s.project)) return false;
        const changed = sc.open !== next;
        if (changed) {
            sc.open = next;
            if (!next) sc.dirty = false;
            savePrefs(s);
            syncScopes(s);
            if (next) readScopes(s);
        }
        const toggle = options && options.focus ? s.q('[data-action="scopes"]') : null;
        if (toggle && !toggle.disabled) toggle.focus({ preventScroll: true });
        return changed;
    }

    // Shows or hides one kind; the last shown kind stays shown. A change reads the last frame again, so a newly shown
    // scope fills at once. Returns true when it changed.
    function toggleScope(s, kind) {
        const sc = s && s.scopes;
        if (!sc || !M().KINDS.includes(kind)) return false;
        if (sc.show.has(kind)) {
            if (sc.show.size === 1) return false;
            sc.show.delete(kind);
        } else sc.show.add(kind);
        savePrefs(s);
        syncScopes(s);
        readScopes(s);
        return true;
    }

    // Clicks on the toggle and the kind buttons; handleClick calls it after handleRangeClick. These are view state, so
    // they work in read-only projects too. Returns true when the click was the panel's.
    function handleScopeClick(s, event) {
        const sc = s && s.scopes, target = event && event.target;
        if (!sc || !target || typeof target.closest !== 'function') return false;
        const toggle = target.closest('[data-action="scopes"]');
        if (toggle) { if (!toggle.disabled) setScopesOpen(s, !sc.open); return true; }
        const kind = target.closest('[data-scope]');
        if (!kind) return false;
        toggleScope(s, kind.dataset.scope);
        return true;
    }

    // Keys inside the open panel; handleKeys calls it directly after handleRangeKeys. Returns true when the key was the
    // panel's.
    // - Escape closes the panel and focuses the toggle.
    // - Plain scroll keys on the tile row stay with it (no preventDefault), so the row scrolls natively and the
    //   playhead does not move.
    // - Everything else goes on.
    function handleScopeKeys(s, event) {
        const sc = s && s.scopes, target = event && event.target;
        if (!sc || !sc.open || !target || typeof target.closest !== 'function' || !target.closest('[data-scopes]')) return false;
        if (event.key === 'Escape') { event.preventDefault(); setScopesOpen(s, false, { focus: true }); return true; }
        if (event.ctrlKey || event.metaKey || event.altKey) return false;
        return SCROLL_KEYS.has(event.key) && !!target.closest('.vs-scopes-tiles');
    }

    // The preview's onPaint hook (index §4.26.2). A paint reads only when it is due; readScopes turns a due paint during
    // a pending read into dirty. It never paints.
    function scopesAfterPaint(s) {
        const sc = s && s.scopes;
        if (!sc || !sc.open || !s.preview || typeof s.preview.isPlaying !== 'function') return;
        if (M().due(sc.last, now(), !!s.preview.isPlaying())) readScopes(s);
    }

    // One read of the last frame the stage painter drew, without a paint. It calls
    // Promise.resolve(preview.readScaled(256, 144)), so both painters take the same path.
    // - While a read is pending it only sets dirty, and that read ends with one trailing read.
    // - The result is drawn only while the panel is still open.
    // - It reads nothing when the panel is closed, without a project, in source mode, or after dispose.
    // Returns true when a read started.
    function readScopes(s) {
        const sc = s && s.scopes, preview = s && s.preview;
        if (!sc || !sc.open || s.disposed || !s.project || s.stageMode === 'source' || !preview || typeof preview.readScaled !== 'function') return false;
        if (sc.pending) { sc.dirty = true; return false; }
        sc.pending = true; sc.dirty = false; sc.last = now();
        let request;
        try { request = Promise.resolve(preview.readScaled(M().SIZE.width, M().SIZE.height)); } catch (_) { request = Promise.resolve(null); }
        request.catch(() => null).then(frame => {
            sc.pending = false;
            if (s.disposed || s.scopes !== sc || !sc.open) return;
            drawScopes(s, frame);
            if (sc.dirty) readScopes(s);
        });
        return true;
    }

    // Draws the shown scopes from one frame (index §4.26.4). It analyses once; then, per shown kind, it calls
    // putImageData with the kind's image and strokes graticule(kind) over it. A null frame draws nothing (false).
    function drawScopes(s, frame) {
        const sc = s && s.scopes, analysis = frame ? M().analyse(frame) : null;
        if (!sc || !analysis) return false;
        M().KINDS.forEach(kind => {
            const canvas = sc.show.has(kind) ? s.q(`[data-scope-canvas="${kind}"]`) : null;
            const ctx = canvas && typeof canvas.getContext === 'function' ? canvas.getContext('2d') : null;
            if (!ctx) return;
            const size = sizeOf(kind), image = ctx.createImageData(size.width, size.height);
            image.data.set(imageOf(kind, analysis));
            ctx.putImageData(image, 0, 0);
            strokeGraticule(ctx, M().graticule(kind));
        });
        return true;
    }
    function strokeGraticule(ctx, shapes) {
        ctx.save();
        ctx.lineWidth = 1;
        shapes.forEach(shape => {
            ctx.strokeStyle = shape.color;
            ctx.beginPath();
            if (shape.kind === 'line') { ctx.moveTo(shape.x0, shape.y0); ctx.lineTo(shape.x1, shape.y1); }
            else if (shape.kind === 'circle') ctx.arc(shape.cx, shape.cy, shape.r, 0, Math.PI * 2);
            else if (shape.kind === 'rect') ctx.rect(shape.x, shape.y, shape.w, shape.h);
            ctx.stroke();
        });
        ctx.restore();
    }

    Object.assign(P, { wireScopes, syncScopes, setScopesOpen, toggleScope, handleScopeClick, handleScopeKeys, scopesAfterPaint, readScopes, drawScopes });
})();
