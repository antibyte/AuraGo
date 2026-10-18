(function () {
    'use strict';

    // Popovers, the narrow drawer, timeline height, the preview transform box and inspector field edits.
    const P = window.VideoStudioParts;
    const { PREF_TIMELINE } = P;
    const I = (...args) => P.I(...args);
    const T = (...args) => P.T(...args);
    const canonicalProject = (...args) => P.canonicalProject(...args);
    const clamp = (...args) => P.clamp(...args);
    const speedOf = clip => (Number(clip.speed) > 0 ? Number(clip.speed) : 1);
    const clearDraft = (...args) => P.clearDraft(...args);
    const esc = (...args) => P.esc(...args);
    const hydrateProject = (...args) => P.hydrateProject(...args);
    const icon = (...args) => P.icon(...args);
    const loadProjects = (...args) => P.loadProjects(...args);
    const maxFrames = (...args) => P.maxFrames(...args);
    const mutate = (...args) => P.mutate(...args);
    const persistDraft = (...args) => P.persistDraft(...args);
    const recordChange = (...args) => P.recordChange(...args);
    const refreshProject = (...args) => P.refreshProject(...args);
    const renderInspector = (...args) => P.renderInspector(...args);
    const renderJobs = (...args) => P.renderJobs(...args);
    const renderJobsButton = (...args) => P.renderJobsButton(...args);
    const renderToolbar = (...args) => P.renderToolbar(...args);
    const renderUI = (...args) => P.renderUI(...args);
    const saveProject = (...args) => P.saveProject(...args);
    const scheduleSave = (...args) => P.scheduleSave(...args);
    const selectedClip = (...args) => P.selectedClip(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const snapshot = (...args) => P.snapshot(...args);
    const tr = (...args) => P.tr(...args);
    const writePref = (...args) => P.writePref(...args);

    // Inspector re-renders replace its controls; keep keyboard focus on the equivalent control.
    function focusKey(s) {
        const active = document.activeElement;
        if (!active || !s.q('[data-inspector]')?.contains(active)) return null;
        for (const attr of ['field', 'textField', 'textStyle', 'position', 'fit', 'transition', 'transitionDuration', 'textAlign', 'textToggle', 'value', 'action']) {
            if (active.dataset && active.dataset[attr] != null) return { attr: attr.replace(/[A-Z]/g, c => '-' + c.toLowerCase()), value: active.dataset[attr] };
        }
        return null;
    }
    function restoreFocus(s, key) {
        if (!key) return;
        const el = s.q('[data-inspector]').querySelector(`[data-${key.attr}${key.value ? `="${CSS.escape(key.value)}"` : ''}]`);
        if (el && !el.disabled) el.focus({ preventScroll: true });
    }
    // Transitions apply immediately: the outgoing clip owns an exact overlap with the next clip. clipId (2e, the effects
    // library's cut drop and double-click) names the outgoing clip, which the edit then selects; without it the selected
    // clip is used. True when the edit was applied.
    function applyTransition(s, type, frames, clipId) {
        const found = T().clipFor(s.project, clipId || s.selectedClipId);
        if (!found || s.readonly || found.track.locked || found.track.kind === 'audio') return false;
        const ordered = found.track.clips.slice().sort((a, b) => a.start - b.start);
        const next = ordered[ordered.findIndex(clip => clip.id === found.clip.id) + 1];
        if (type !== 'none' && !next) { showNotice(s, 'transitionNeedsNext', 'Add a following clip on this track first.', true); return false; }
        const rate = P.rateOf(s), duration = clamp(Math.round(Number(frames) || rate / 2), 1, Math.round(rate * 3));
        if (type !== 'none' && duration >= Math.min(found.clip.duration, next.duration)) { showNotice(s, 'transitionTooLong', 'Transition duration must be shorter than both clips.', true); return false; }
        return mutate(s, 'transition', project => (T().setTransition(project, found.track.id, found.clip.id, type, duration) ? (clipId ? P.assignSelection(s, [found.clip.id], found.clip.id) : undefined) : false));
    }
    function percentText(value) { return Math.round(value) + ' %'; }
    function updateOutput(input, text) { const output = input.parentElement && input.parentElement.querySelector('output'); if (output) output.textContent = text; }
    // The clip's box at the playhead, keyframes included (index §4.18): field edits and drags start from what is shown.
    function boxAt(s, clip) { const st = window.VideoStudioEffects.clipState(clip, s.frame, s.project); return { x: st.x, y: st.y, width: st.width, height: st.height }; }
    // The box values a gesture has changed so far (holder._vsTouched; once changed, a value stays in the set, so moving back
    // still writes it). Only these go to auto-key, so animated values the edit did not touch get no key. clampBox rounds
    // to 1e-6, hence the tolerance.
    function touchedBox(holder, from, box) {
        holder._vsTouched = holder._vsTouched || new Set();
        ['x', 'y', 'width', 'height'].forEach(path => { if (Math.abs(Number(box[path]) - Number(from[path])) > 1e-6) holder._vsTouched.add(path); });
        const out = {};
        holder._vsTouched.forEach(path => { out[path] = box[path]; });
        return out;
    }
    // Auto-key during a live gesture (spec §8.2): animated values get a key at the playhead, the others change their
    // field (VideoStudioKeyframeEdit.setValues). A refusal, for example the keyframe limit, is explained once per gesture.
    function liveValues(s, found, values, holder) {
        if (!Object.keys(values).length) return;
        const clip = found.clip, saved = JSON.stringify(clip);
        const result = window.VideoStudioKeyframeEdit.setValues(s.project, clip.id, values, s.frame);
        if (result === true) return;
        // A refusal can leave the clip partly changed (outside P.applyEdit nothing restores it): put it back, in place.
        const before = JSON.parse(saved);
        Object.keys(clip).forEach(key => { if (!(key in before)) delete clip[key]; });
        Object.assign(clip, before);
        if (!holder._vsRefused) { holder._vsRefused = true; P.showReason(s, result); }
    }
    function liveField(s, input) {
        const found = selectedClip(s);
        if (!found || found.track.locked) return;
        const field = input.dataset.field, value = Number(input.value);
        if (!input._vsBefore) { input._vsBefore = snapshot(s); input._vsBox = boxAt(s, found.clip); s.fieldGesture = true; }
        clearTimeout(s.autosaveTimer);
        if (field === 'opacity') { liveValues(s, found, { opacity: clamp(value / 100, 0, 1) }, input); updateOutput(input, percentText(value)); }
        else if (field === 'volume') { found.clip.volume = clamp(value / 100, 0, 4); updateOutput(input, percentText(value)); }
        else if (field === 'rotation') { liveValues(s, found, { rotation: clamp(Math.round(value), -360, 360) }, input); updateOutput(input, Math.round(value) + '°'); }
        else if (field === 'size') { liveValues(s, found, touchedBox(input, input._vsBox, I().scaleBox(input._vsBox, clamp(value / 100, 0.05, 1))), input); updateOutput(input, percentText(value)); }
        else if (field === 'fade_in' || field === 'fade_out') { found.clip[field] = clamp(Math.round(value), 0, found.clip.duration); updateOutput(input, I().formatSeconds(found.clip[field], P.uiLocale(), s.project) + ' s'); }
        else return;
        s.dirty = true; s.revision++;
        renderToolbar(s);
        s.preview.seek(s.frame);
        updateTransform(s);
    }
    function commitField(s, input) {
        const found = selectedClip(s);
        if (!found || found.track.locked) { renderInspector(s); return; }
        const field = input.dataset.field;
        if (field === 'speed') { P.commitSpeedField(s, input); return; }
        if (['start', 'duration', 'offset'].includes(field)) {
            const frames = I().parseTime(input.value, s.project);
            if (frames == null) { showNotice(s, 'invalidTime', 'Enter a time like 4,5 or 1:04.5.', true); renderInspector(s); return; }
            const asset = s.project.assets.find(item => item.id === found.clip.asset_id) || {};
            mutate(s, field, project => {
                const target = T().clipFor(project, found.clip.id).clip, FX = window.VideoStudioEffects, ramped = FX.hasRamp(target);
                // The source extent rule: offset + clipSource(duration - 1) <= duration_frames - 1, with (duration - 1) × speed
                // or a ramp's sourceExtent. The absolute offset stays editable on a ramped clip (index §4.18).
                const extent = ramped ? FX.sourceExtent(target) - 1e-9 : (target.duration - 1) * speedOf(target);
                const maxOffset = asset.kind === 'image' ? 0 : Math.max(0, Math.floor(asset.duration_frames - 1 - extent));
                if (field === 'start') target.start = clamp(frames, 0, maxFrames(s) - target.duration);
                else if (field === 'offset') target.offset = asset.kind === 'image' ? 0 : clamp(frames, 0, maxOffset);
                else {
                    const sourceLimit = asset.kind === 'image' || target.text ? maxFrames(s) : ramped ? FX.maxDurationFor(target, asset.duration_frames, target.offset) : Math.floor((asset.duration_frames - 1 - target.offset) / speedOf(target) + 1e-9) + 1;
                    target.duration = clamp(frames, 1, Math.min(sourceLimit, maxFrames(s) - target.start));
                    target.fade_in = Math.min(target.fade_in, target.duration); target.fade_out = Math.min(target.fade_out, target.duration);
                }
            });
            return;
        }
        if (['x', 'y', 'width', 'height'].includes(field)) {
            const value = Number(input.value) / 100;
            if (!Number.isFinite(value)) { renderInspector(s); return; }
            // The edit starts from the box shown at the playhead; animated values get a key there (auto-key).
            P.applyEdit(s, 'box', project => {
                const from = boxAt(s, T().clipFor(project, found.clip.id).clip);
                return window.VideoStudioKeyframeEdit.setValues(project, found.clip.id, touchedBox({ _vsTouched: new Set([field]) }, from, I().clampBox(Object.assign({}, from, { [field]: value }))), s.frame);
            });
            return;
        }
        const before = input._vsBefore; delete input._vsBefore; delete input._vsBox; delete input._vsTouched; delete input._vsRefused;
        s.fieldGesture = false;
        if (!before) { P.retryArtwork(s); return; }
        if (T().validTimeline && !T().validTimeline(s.project)) {
            s.project = hydrateProject(s, before);
            showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true);
            renderUI(s); return;
        }
        s.history.push(before); if (s.history.length > 50) s.history.shift(); s.redo = [];
        s.dirty = JSON.stringify(canonicalProject(s.project)) !== JSON.stringify(s.savedProject);
        if (s.dirty) { persistDraft(s); scheduleSave(s); } else clearDraft(s);
        renderInspector(s); renderToolbar(s); T().render(s.q('[data-timeline]'), s.timelineOptions); s.preview.seek(s.frame);
        P.retryArtwork(s);
    }
    // The pre-edit snapshot lives on the state, so an inspector re-render cannot lose it.
    function liveText(s, input) {
        const found = selectedClip(s);
        if (!found || found.track.locked) return;
        if (!s.textBefore || s.textBefore.clipId !== found.clip.id) { P.flushTextBefore(s); s.textBefore = { clipId: found.clip.id, snapshot: snapshot(s) }; }
        clearTimeout(s.autosaveTimer);
        if (input.matches('[data-text-field="text"]')) found.clip.text = input.value;
        else {
            const style = found.clip.text_style || (found.clip.text_style = {});
            style[input.dataset.textStyle] = ['font_size', 'background_opacity'].includes(input.dataset.textStyle) ? Number(input.value) : input.value;
            if (input.dataset.textStyle === 'background_opacity') updateOutput(input, percentText(Number(input.value) * 100));
        }
        s.dirty = true; s.revision++;
        renderToolbar(s);
        P.markTextStale(s, found.clip.id);
    }
    function commitText(s) {
        P.flushTextBefore(s);
        s.dirty = JSON.stringify(canonicalProject(s.project)) !== JSON.stringify(s.savedProject);
        if (s.dirty) { persistDraft(s); scheduleSave(s); } else clearDraft(s);
        renderToolbar(s);
        T().render(s.q('[data-timeline]'), s.timelineOptions);
    }
    function mutateSelected(s, label, fn) {
        const found = selectedClip(s);
        if (!found || found.track.locked || s.readonly) return false;
        return mutate(s, label, project => fn(T().clipFor(project, found.clip.id).clip, project));
    }
    // The narrow-window drawer ends above the timeline, whatever height the user gave it.
    function syncDrawer(s) {
        const panel = s.q('.vs-timeline-panel'), handle = s.q('[data-resize]');
        if (!panel || !s.app) return;
        s.app.style.setProperty('--vs-drawer-bottom', Math.round(panel.getBoundingClientRect().height + (handle ? handle.getBoundingClientRect().height / 2 : 0)) + 'px');
    }
    // Dialogs and the narrow drawer are modal: everything behind them is inert, so Tab, Space and clicks
    // cannot reach the editor underneath.
    function syncInert(s) {
        if (!s.app) return;
        const modal = !!s.q('[data-modal-host] .vs-modal, [data-conflict-host] .vs-modal');
        const drawer = s.app.classList.contains('vs-show-inspector');
        s.app.querySelectorAll(':scope > .vs-toolbar, :scope > .vs-disabled, :scope > .vs-resize, :scope > .vs-timeline-panel, :scope > .vs-workspace > .vs-library, :scope > .vs-workspace > .vs-center').forEach(el => { el.inert = modal || drawer; });
        s.app.querySelectorAll(':scope > .vs-workspace > .vs-inspector, :scope > .vs-workspace > .vs-inspector-scrim').forEach(el => { el.inert = modal; });
    }
    function setInspectorOpen(s, open, moveFocus) {
        const app = s.app, inspector = s.q('.vs-inspector');
        syncDrawer(s);
        const narrow = app.getBoundingClientRect().width <= 920;
        app.classList.toggle('vs-show-inspector', !!open && narrow);
        if (narrow && open) {
            inspector.setAttribute('role', 'dialog'); inspector.setAttribute('aria-modal', 'true');
            if (moveFocus) inspector.querySelector('button:not(:disabled),input:not(:disabled),select:not(:disabled),textarea:not(:disabled)')?.focus();
        } else {
            inspector.removeAttribute('role'); inspector.removeAttribute('aria-modal');
        }
        syncInert(s);
    }
    const POPOVERS = { project: ['project-menu', '[data-project-popover]'], jobs: ['jobs', '[data-jobs-popover]'], shortcuts: ['shortcuts', '[data-shortcuts-popover]'] };
    function togglePopover(s, name) {
        const [action, selector] = POPOVERS[name];
        const pop = s.q(selector), button = s.q(`[data-action="${action}"]`);
        const open = pop.hidden;
        closePopovers(s);
        if (!open) return;
        if (name === 'project') { renderProjectPopover(s); if (!s.featureDisabled) loadProjects(s).catch(() => {}); }
        if (name === 'jobs') renderJobs(s);
        pop.hidden = false;
        button.setAttribute('aria-expanded', 'true');
        pop.querySelector('input,button:not(:disabled),a')?.focus();
    }
    function closePopovers(s, except, returnFocus) {
        Object.entries(POPOVERS).forEach(([name, [action, selector]]) => {
            if (name === except) return;
            const pop = s.q(selector);
            if (pop && !pop.hidden) {
                const trigger = s.q(`[data-action="${action}"]`), hadFocus = pop.contains(document.activeElement);
                pop.hidden = true;
                if (trigger) { trigger.setAttribute('aria-expanded', 'false'); if (returnFocus || hadFocus) trigger.focus({ preventScroll: true }); }
            }
        });
        renderJobsButton(s);
    }
    function closePopoversOutside(s, target) {
        const anchor = target.closest('.vs-menu-anchor');
        Object.entries(POPOVERS).forEach(([name, [, selector]]) => {
            const pop = s.q(selector);
            if (pop && !pop.hidden && (!anchor || !anchor.contains(pop))) closePopovers(s);
        });
    }
    function renderProjectPopover(s) {
        const pop = s.q('[data-project-popover]');
        if (!pop) return;
        const folder = String(s.desktopPath || '').split('/').filter(part => part && part !== s.projectId).join(' / ');
        const projects = s.projects.map(item => {
            const id = item.id || item.project_id || '';
            const name = item.project && item.project.name || tr(s, 'untitled', 'Untitled project');
            const current = id === s.projectId;
            return `<li><button type="button" data-project-id="${esc(s, id)}" ${current ? 'aria-current="true"' : ''}>${icon(current ? 'check' : 'project', 15)}<span>${esc(s, name)}</span></button></li>`;
        }).join('');
        pop.innerHTML = `${s.project ? `<div class="vs-pop-section"><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'projectNameLabel', 'Project name'))}</span><span class="vs-inline-field"><input type="text" data-project-rename maxlength="120" value="${esc(s, s.project.name || '')}" ${s.readonly ? 'disabled' : ''}><button type="button" data-action="rename-project" ${s.readonly ? 'disabled' : ''}>${esc(s, tr(s, 'rename', 'Rename'))}</button></span></label><button type="button" class="vs-link-button" data-action="project-settings" ${s.readonly ? 'disabled' : ''}>${icon('sliders', 14)}<span>${esc(s, tr(s, 'projectSettings', 'Project settings…'))}</span></button>${folder ? `<p class="vs-hint">${icon('folder', 13)} ${esc(s, tr(s, 'savedIn', 'Saved in'))} ${esc(s, folder)}</p>` : ''}</div>` : ''}<div class="vs-pop-section"><h4>${esc(s, tr(s, 'projects', 'Projects'))}</h4>${projects ? `<ul class="vs-project-list">${projects}</ul>` : `<p class="vs-hint">${esc(s, tr(s, 'noProjectsYet', 'No projects yet.'))}</p>`}</div><div class="vs-pop-foot"><button type="button" data-action="all-projects">${icon('project', 15)}<span>${esc(s, tr(s, 'allProjects', 'All projects…'))}</span></button><button type="button" class="vs-primary" data-action="new-project" ${s.readonly ? 'disabled' : ''}>${icon('plus', 15)}<span>${esc(s, tr(s, 'newProject', 'New project'))}</span></button></div>`;
        pop.querySelector('[data-project-rename]')?.addEventListener('keydown', event => { if (event.key === 'Enter') { event.preventDefault(); renameProject(s); } });
    }
    function renameProject(s) {
        const input = s.q('[data-project-rename]');
        const name = input ? input.value.trim().slice(0, 120) : '';
        if (!name || !s.project || name === s.project.name) { closePopovers(s); return; }
        mutate(s, 'rename', project => { project.name = name; });
        closePopovers(s);
        saveProject(s).then(() => loadProjects(s)).catch(() => {});
    }
    function wireResize(s) {
        const handle = s.q('[data-resize]');
        const current = () => s.q('.vs-timeline-panel').getBoundingClientRect().height;
        const setHeight = height => {
            const max = Math.max(160, s.app.getBoundingClientRect().height - 240);
            const value = Math.round(clamp(height, 140, max));
            s.app.style.setProperty('--vs-timeline-h', value + 'px');
            updateTransform(s);
            syncDrawer(s);
            return value;
        };
        let start = null;
        handle.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            start = { y: event.clientY, height: current() };
            handle.setPointerCapture(event.pointerId);
            handle.classList.add('is-active');
            event.preventDefault();
        }, { signal: s.abort.signal });
        handle.addEventListener('pointermove', event => { if (start) setHeight(start.height - (event.clientY - start.y)); }, { signal: s.abort.signal });
        const end = () => { if (!start) return; start = null; handle.classList.remove('is-active'); writePref(PREF_TIMELINE, Math.round(current())); };
        handle.addEventListener('pointerup', end, { signal: s.abort.signal });
        handle.addEventListener('pointercancel', end, { signal: s.abort.signal });
        handle.addEventListener('keydown', event => {
            if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return;
            event.preventDefault();
            writePref(PREF_TIMELINE, setHeight(current() + (event.key === 'ArrowUp' ? 24 : -24)));
        }, { signal: s.abort.signal });
    }
    // Direct manipulation: the selected visual clip gets a box over the preview canvas.
    function transformTarget(s) {
        if (s.stageMode === 'source') return null;
        const found = selectedClip(s);
        if (!found || s.readonly || found.track.locked || found.track.hidden || found.track.kind === 'audio') return null;
        const clip = found.clip;
        if (s.frame < clip.start || s.frame >= clip.start + clip.duration) return null;
        return found;
    }
    function updateTransform(s) {
        const box = s.q('[data-transform]');
        if (!box) return;
        // Crop mode (2c) and mask mode (2d, video-studio-mask.js) show their overlay instead of the transform box.
        updateCropBox(s);
        if (P.updateMaskBox) P.updateMaskBox(s);
        const found = s.project && !s.q('.vs-no-project') && !s.cropMode && !s.maskMode ? transformTarget(s) : null;
        box.hidden = !found;
        if (!found) return;
        // The box at the playhead, keyframes included.
        const st = window.VideoStudioEffects.clipState(found.clip, s.frame, s.project);
        box.style.left = st.x * 100 + '%'; box.style.top = st.y * 100 + '%';
        box.style.width = st.width * 100 + '%'; box.style.height = st.height * 100 + '%';
        box.style.transform = st.rotation ? `rotate(${st.rotation}deg)` : '';
        box.title = tr(s, 'dragToMove', 'Drag to move, corners to resize');
    }
    // The persistent preview notice (index §4.5). It is a status line inside the mat, not a toast:
    // - the 2D painter cannot show effects, blend modes, masks or keying;
    // - a transition shader that failed shows as a cut;
    // - an effect shader that failed is skipped, and its input passes through.
    // A transition failure takes precedence over an effect failure.
    function syncPreviewNotice(s) {
        const notice = s.q('[data-preview-notice]');
        if (!notice) return;
        const preview = s.preview;
        const kind = s.project && preview && typeof preview.painterKind === 'function' ? preview.painterKind() : '';
        const failed = kind === 'gl' && typeof preview.failedPrograms === 'function' ? preview.failedPrograms() : [];
        const text = kind === '2d' ? tr(s, 'previewFallback', 'Simple preview: effects, blend modes, masks and keying are not shown here.')
            : failed.some(item => item.kind === 'transition') ? tr(s, 'previewTransitionFallback', 'A transition cannot be drawn in this preview and shows as a cut. The export is not affected.')
                : failed.some(item => item.kind === 'effect') ? tr(s, 'previewEffectFallback', 'An effect cannot be drawn in this preview and is skipped here. The export is not affected.') : '';
        if (notice.textContent !== text) notice.textContent = text;
        notice.hidden = !text;
    }
    function wireTransform(s) {
        const box = s.q('[data-transform]'), mat = s.q('.vs-preview-mat'), signal = s.abort.signal;
        let gesture = null;
        box.addEventListener('pointerdown', event => {
            const found = transformTarget(s);
            if (!found || event.button !== 0 || s.timelineDragging) return;
            const rect = mat.getBoundingClientRect();
            const clip = found.clip;
            gesture = { clipId: clip.id, handle: event.target.closest('[data-handle]')?.dataset.handle || '', x: event.clientX, y: event.clientY, w: rect.width, h: rect.height, box: boxAt(s, clip), before: snapshot(s), epoch: s.projectEpoch, projectId: s.projectId };
            s.timelineDragging = true; s.transformDrag = true;
            clearTimeout(s.autosaveTimer);
            try { box.setPointerCapture(event.pointerId); } catch (_) { /* synthetic pointer */ }
            box.classList.add('is-dragging');
            event.preventDefault(); event.stopPropagation();
        }, { signal });
        box.addEventListener('pointermove', event => {
            if (!gesture) return;
            const found = T().clipFor(s.project, gesture.clipId);
            if (!found || gesture.epoch !== s.projectEpoch) return;
            const dx = (event.clientX - gesture.x) / gesture.w, dy = (event.clientY - gesture.y) / gesture.h, b = gesture.box;
            let next;
            if (!gesture.handle) next = I().clampBox({ x: b.x + dx, y: b.y + dy, width: b.width, height: b.height });
            else {
                const east = gesture.handle.includes('e'), south = gesture.handle.includes('s');
                const ratio = b.height / b.width;
                let width = clamp(b.width + (east ? dx : -dx), 0.03, 1);
                const maxWidth = Math.min(east ? 1 - b.x : b.x + b.width, (south ? 1 - b.y : b.y + b.height) / ratio);
                width = Math.min(width, maxWidth);
                const height = width * ratio;
                next = I().clampBox({ x: east ? b.x : b.x + b.width - width, y: south ? b.y : b.y + b.height - height, width, height });
            }
            liveValues(s, found, touchedBox(gesture, gesture.box, next), gesture);
            s.preview.seek(s.frame);
            updateTransform(s);
        }, { signal });
        const finish = event => {
            if (!gesture) return;
            const { before, epoch, projectId } = gesture;
            gesture = null;
            s.timelineDragging = false; s.transformDrag = false;
            box.classList.remove('is-dragging');
            if (epoch !== s.projectEpoch || projectId !== s.projectId) return;
            if (event.type === 'pointercancel') { s.project = hydrateProject(s, before); renderUI(s); }
            else if (JSON.stringify(before) !== JSON.stringify(s.project)) recordChange(s, 'transform', before);
            P.retryArtwork(s);
            // pointerdown paused the autosave; a click without movement must not leave edits unsaved.
            if (s.dirty && !s.conflict) P.queueAutosave(s);
            if (s.pendingRefresh) { s.pendingRefresh = false; refreshProject(s).catch(() => {}); }
        };
        box.addEventListener('pointerup', finish, { signal });
        box.addEventListener('pointercancel', finish, { signal });
        box.addEventListener('lostpointercapture', () => { if (gesture) finish({ type: 'pointercancel' }); }, { signal });
        window.addEventListener('blur', () => { if (gesture) finish({ type: 'pointercancel' }); }, { signal });
        // Clicking the picture selects the front-most visible clip under the pointer.
        mat.addEventListener('click', event => {
            if (event.target.closest('[data-transform],[data-no-project],button') || !s.project) return;
            const rect = mat.getBoundingClientRect();
            const px = (event.clientX - rect.left) / rect.width, py = (event.clientY - rect.top) / rect.height;
            const hit = I().displayTracks(s.project.tracks).filter(track => track.kind !== 'audio' && !track.hidden).flatMap(track => track.clips.filter(clip => {
                if (s.frame < clip.start || s.frame >= clip.start + clip.duration) return false;
                const st = window.VideoStudioEffects.clipState(clip, s.frame, s.project);
                return px >= st.x && px <= st.x + st.width && py >= st.y && py <= st.y + st.height;
            }))[0];
            if ((hit ? hit.id : '') === s.selectedClipId) return;
            s.timelineOptions.onSelect(hit ? hit.id : '');
            T().render(s.q('[data-timeline]'), s.timelineOptions);
        }, { signal });
    }

    // ---- On-canvas crop handles (2c, spec §8.1) ---------------------------------------------------------------------
    // P.setCropMode(s, on) swaps the transform box for the crop overlay [data-crop-box]. P.wireCropBox builds it once in
    // .vs-preview-mat; P.updateCropBox (called by updateTransform) keeps it on the clip box, rotation included.
    // [data-crop-source] outlines the whole source as the clip shows it; [data-crop-rect] is the crop with eight
    // [data-crop-handle] buttons. A drag moves only the overlay; pointerup commits once through
    // VideoStudioKeyframeEdit.setCrop with the values the gesture changed (animated crop paths get a key at the
    // playhead). Arrow keys move the focused handle's edges, or the whole crop on the rectangle, by 0.01 (Shift 0.1);
    // Enter and Escape leave. Crop fractions refer to the unflipped source (index §4.20), so flips mirror the handles.
    const CROP_HANDLES = [['nw', 'cropHandle_nw', 'Top left corner'], ['n', 'cropHandle_n', 'Top edge'], ['ne', 'cropHandle_ne', 'Top right corner'], ['e', 'cropHandle_e', 'Right edge'], ['se', 'cropHandle_se', 'Bottom right corner'], ['s', 'cropHandle_s', 'Bottom edge'], ['sw', 'cropHandle_sw', 'Bottom left corner'], ['w', 'cropHandle_w', 'Left edge']];
    const CROP_MAX = 0.95;
    const CROP_ARROWS = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] };
    const PIC = () => window.VideoStudioPicture;
    // The crop.* registry step (0.001), rounded to nearest or down; adding 0 turns -0 into 0.
    const cropStep = value => Math.round(value * 1000) / 1000 + 0, cropFloor = value => Math.floor(value * 1000 + 1e-6) / 1000 + 0;
    function cropOf(state) {
        const crop = state && state.crop || {};
        return { l: Number(crop.l) || 0, t: Number(crop.t) || 0, r: Number(crop.r) || 0, b: Number(crop.b) || 0 };
    }
    // Crop mode works on the clip the transform box would show (timeline mode, the selected visual clip on an unlocked,
    // visible track, the playhead inside it), except titles.
    function cropTarget(s) {
        const found = transformTarget(s);
        return found && !found.clip.text ? found : null;
    }
    function cropAsset(s, clip) { return (s.project.assets || []).find(asset => asset.id === clip.asset_id) || null; }
    // The size the clip's media element reports (autorotated; the preview's mediaSize), or null before it is known.
    function cropElement(s, clip) { return s.preview && typeof s.preview.mediaSize === 'function' ? s.preview.mediaSize(clip.id) : null; }
    // The overlay geometry in project pixels for a clip state (clipState at the playhead), its asset and the size its
    // media element reports (element, or null). The source size is VideoStudioPicture.decodedSize, as in both painters,
    // so a rotated phone video shows upright (plan 2c R7). box is the clip box; draw is where the cropped picture lands
    // (VideoStudioEffects.placement with the cropRect size as the source); full is the whole source at the same scale,
    // flips applied; edges is the crop as the screen shows it.
    function cropGeometry(state, asset, project, element) {
        const size = { width: Number(project && project.width) || 1280, height: Number(project && project.height) || 720 };
        const flipH = !!state.flipH, flipV = !!state.flipV, crop = cropOf(state), edges = PIC().mirroredCrop(crop, flipH, flipV);
        const ew = Number(element && element.width) || 0, eh = Number(element && element.height) || 0;
        const known = (Number(asset && asset.width) > 0 && Number(asset && asset.height) > 0) || (ew > 0 && eh > 0);
        const decoded = known ? PIC().decodedSize(asset, ew, eh) : null, sw = decoded ? decoded.width : 0, sh = decoded ? decoded.height : 0;
        const rect = sw > 0 && sh > 0 ? PIC().cropRect(crop, sw, sh) : null;
        const place = window.VideoStudioEffects.placement(state, size, size, rect ? { width: rect.w, height: rect.h } : null), draw = place.draw;
        let full;
        if (rect) {
            const kx = draw.w / rect.w, ky = draw.h / rect.h;
            const left = flipH ? sw - rect.x - rect.w : rect.x, top = flipV ? sh - rect.y - rect.h : rect.y;
            full = { x: draw.x - left * kx, y: draw.y - top * ky, w: sw * kx, h: sh * ky };
        } else {
            // Without a source size the picture fills the box and the fractions place the source.
            const w = draw.w / Math.max(0.05, 1 - edges.l - edges.r), h = draw.h / Math.max(0.05, 1 - edges.t - edges.b);
            full = { x: draw.x - edges.l * w, y: draw.y - edges.t * h, w, h };
        }
        return { box: place.box, draw, full, edges, angle: place.angle, flipH, flipV };
    }
    // A drag or nudge of one handle ('nw' … 'w') or of the whole crop ('move'). dx and dy are fractions of the whole
    // source in screen directions (x right, y down). Edges stay within 0..0.95 with at most 0.95 per axis, round to the
    // 0.001 step (only the edges the gesture moves; the others stay exactly as they were) and map back to unflipped fractions.
    function cropDrag(crop, flipH, flipV, handle, dx, dy) {
        const e = PIC().mirroredCrop(cropOf({ crop }), !!flipH, !!flipV), start = Object.assign({}, e), x = Number(dx) || 0, y = Number(dy) || 0;
        if (handle === 'move') {
            const sx = clamp(x, -e.l, e.r), sy = clamp(y, -e.t, e.b);
            e.l += sx; e.r -= sx; e.t += sy; e.b -= sy;
        } else {
            if (handle.includes('w')) e.l = clamp(e.l + x, 0, CROP_MAX - e.r);
            if (handle.includes('e')) e.r = clamp(e.r - x, 0, CROP_MAX - e.l);
            if (handle.includes('n')) e.t = clamp(e.t + y, 0, CROP_MAX - e.b);
            if (handle.includes('s')) e.b = clamp(e.b - y, 0, CROP_MAX - e.t);
        }
        const settle = key => (e[key] === start[key] ? start[key] : cropStep(e[key])), out = { l: settle('l'), t: settle('t'), r: settle('r'), b: settle('b') };
        // Rounding both edges of an axis up can pass 0.95 by one step; the dragged edge gives way (rounded down, the other edge may be off the step).
        if (out.l + out.r > CROP_MAX + 1e-9) { const key = handle.includes('w') ? 'l' : 'r'; out[key] = cropFloor(CROP_MAX - out[key === 'l' ? 'r' : 'l']); }
        if (out.t + out.b > CROP_MAX + 1e-9) { const key = handle.includes('n') ? 't' : 'b'; out[key] = cropFloor(CROP_MAX - out[key === 't' ? 'b' : 't']); }
        return PIC().mirroredCrop(out, !!flipH, !!flipV);
    }
    // Screen pixels of a drag to fractions of the whole source: the delta turns back by the clip's rotation (clockwise
    // on screen) and divides by the source's size on screen.
    function cropDragDelta(dxPx, dyPx, angle, fullWidthPx, fullHeightPx) {
        const cos = Math.cos(Number(angle) || 0), sin = Math.sin(Number(angle) || 0);
        const lx = dxPx * cos + dyPx * sin, ly = -dxPx * sin + dyPx * cos;
        return { dx: fullWidthPx > 0 ? lx / fullWidthPx : 0, dy: fullHeightPx > 0 ? ly / fullHeightPx : 0 };
    }
    // Places the overlay: the box in percent of the mat, the source and the crop in percent of the box. live draws the
    // crop of a gesture from the gesture's start geometry; otherwise the crop is where the picture lands.
    function drawCropBox(s, el, g, crop, live) {
        const pw = Number(s.project.width) || 1280, ph = Number(s.project.height) || 720, pct = value => value * 100 + '%';
        const put = (node, r) => { node.style.left = pct((r.x - g.box.x) / g.box.w); node.style.top = pct((r.y - g.box.y) / g.box.h); node.style.width = pct(r.w / g.box.w); node.style.height = pct(r.h / g.box.h); };
        el.style.left = pct(g.box.x / pw); el.style.top = pct(g.box.y / ph); el.style.width = pct(g.box.w / pw); el.style.height = pct(g.box.h / ph);
        el.style.transform = g.angle ? `rotate(${g.angle}rad)` : '';
        put(el.querySelector('[data-crop-source]'), g.full);
        const e = PIC().mirroredCrop(crop, g.flipH, g.flipV), rect = el.querySelector('[data-crop-rect]');
        put(rect, live ? { x: g.full.x + e.l * g.full.w, y: g.full.y + e.t * g.full.h, w: g.full.w * (1 - e.l - e.r), h: g.full.h * (1 - e.t - e.b) } : g.draw);
        const percent = value => (Math.round(value * 1000) / 10).toLocaleString(P.uiLocale());
        rect.setAttribute('aria-label', ['l', 't', 'r', 'b'].reduce((text, key) => text.split('{{' + key + '}}').join(percent(crop[key])), tr(s, 'cropBoxLabel', 'Crop: left {{l}} %, top {{t}} %, right {{r}} %, bottom {{b}} %')));
    }
    function syncCropButton(s) {
        const button = s.q('[data-inspector] [data-action="crop-mode"]');
        if (button) button.setAttribute('aria-pressed', String(!!s.cropMode));
    }
    function updateCropBox(s) {
        const el = s.q('[data-crop-box]');
        if (!el) return;
        const found = s.cropMode && s.project && !s.q('.vs-no-project') ? cropTarget(s) : null;
        if (s.cropMode && (!found || found.clip.id !== s.cropMode.clipId)) {
            // The clip left the stage (another selection, the playhead outside it, a lock, the source monitor): the mode
            // ends and keyboard focus stays in the editor.
            const focused = el.contains(document.activeElement);
            s.cropMode = null; s.cropDrag = null;
            el.classList.remove('is-dragging');
            syncCropButton(s);
            if (focused && s.app) s.app.focus({ preventScroll: true });
        }
        el.hidden = !s.cropMode;
        if (!s.cropMode || s.cropDrag) return;
        const state = window.VideoStudioEffects.clipState(found.clip, s.frame, s.project);
        drawCropBox(s, el, cropGeometry(state, cropAsset(s, found.clip), s.project, cropElement(s, found.clip)), cropOf(state), false);
    }
    function setCropMode(s, on, options) {
        const found = on ? cropTarget(s) : null;
        if (on && !found) { showNotice(s, 'cropModeOutside', 'Move the playhead into the selected clip to crop in the preview.', false); return false; }
        // Crop mode and mask mode (video-studio-mask.js, 2d) exclude each other.
        if (found && s.maskMode && P.setMaskMode) P.setMaskMode(s, false);
        // The narrow inspector drawer makes the preview inert; it closes so the crop can take focus.
        if (found && s.app.classList.contains('vs-show-inspector')) setInspectorOpen(s, false, false);
        s.cropMode = found ? { clipId: found.clip.id } : null;
        s.cropDrag = null;
        s.q('[data-crop-box]')?.classList.remove('is-dragging');
        updateTransform(s);
        syncCropButton(s);
        if (s.cropMode) s.q('[data-crop-rect]')?.focus({ preventScroll: true });
        else if (options && options.restoreFocus) (s.q('[data-inspector] [data-action="crop-mode"]:not(:disabled)') || s.app).focus({ preventScroll: true });
        return true;
    }
    // One undo step with the values that changed (cropDrag returns the other sides untouched); setCrop keys animated crop paths at the playhead.
    function commitCrop(s, clipId, before, after) {
        const values = {};
        ['l', 't', 'r', 'b'].forEach(key => { if (Math.abs(after[key] - before[key]) > 1e-9) values[key] = after[key]; });
        if (!Object.keys(values).length) return false;
        return P.applyEdit(s, 'crop', project => window.VideoStudioKeyframeEdit.setCrop(project, clipId, values, s.frame));
    }
    // Keys inside the crop overlay (handleKeys calls this after the range keys): Enter and Escape leave, the arrows nudge
    // (Shift: ten times as far), and Delete/Backspace stay here, so they never delete the clip being cropped.
    function handleCropKeys(s, event) {
        if (!s.cropMode || typeof event.target.closest !== 'function' || !event.target.closest('[data-crop-box]')) return false;
        if (event.key === 'Escape' || event.key === 'Enter') { event.preventDefault(); event.stopPropagation(); setCropMode(s, false, { restoreFocus: true }); return true; }
        if (event.key === 'Delete' || event.key === 'Backspace') { event.preventDefault(); return true; }
        const arrow = CROP_ARROWS[event.key];
        if (!arrow || event.altKey || event.ctrlKey || event.metaKey) return false;
        event.preventDefault(); event.stopPropagation();
        const found = cropTarget(s);
        if (!found || s.cropDrag) return true;
        const state = window.VideoStudioEffects.clipState(found.clip, s.frame, s.project), step = event.shiftKey ? 0.1 : 0.01;
        const handle = event.target.closest('[data-crop-handle]'), before = cropOf(state);
        commitCrop(s, found.clip.id, before, cropDrag(before, state.flipH, state.flipV, handle ? handle.dataset.cropHandle : 'move', arrow[0] * step, arrow[1] * step));
        return true;
    }
    function wireCropBox(s) {
        const mat = s.q('.vs-preview-mat'), anchor = s.q('[data-transform]');
        if (!mat || !anchor || s.q('[data-crop-box]')) return;
        const label = (key, fallback) => esc(s, tr(s, key, fallback)), signal = s.abort.signal;
        const handles = CROP_HANDLES.map(([id, key, fallback]) => `<button type="button" class="vs-crop-handle" data-crop-handle="${id}" aria-label="${label(key, fallback)}" title="${label(key, fallback)}"></button>`).join('');
        anchor.insertAdjacentHTML('afterend', `<div class="vs-crop-box" data-crop-box role="group" aria-label="${label('cropMode', 'Crop in the preview')}" hidden><div class="vs-crop-source" data-crop-source aria-hidden="true"></div><div class="vs-crop-rect" data-crop-rect tabindex="0" title="${label('cropBoxHint', 'Drag an edge or a corner to crop, or drag inside to move the crop. The arrow keys move the crop, or the focused edge (Shift: ten times as far). Enter or Esc finishes.')}">${handles}</div></div>`);
        const el = s.q('[data-crop-box]'), rect = el.querySelector('[data-crop-rect]');
        rect.addEventListener('pointerdown', event => {
            const found = s.cropMode ? cropTarget(s) : null;
            if (!found || event.button !== 0 || s.timelineDragging) return;
            const state = window.VideoStudioEffects.clipState(found.clip, s.frame, s.project), geometry = cropGeometry(state, cropAsset(s, found.clip), s.project, cropElement(s, found.clip));
            const area = mat.getBoundingClientRect(), handle = event.target.closest('[data-crop-handle]'), start = cropOf(state);
            s.cropDrag = {
                clipId: found.clip.id, handle: handle ? handle.dataset.cropHandle : 'move', x: event.clientX, y: event.clientY, geometry, start, crop: start,
                width: geometry.full.w * area.width / (Number(s.project.width) || 1280), height: geometry.full.h * area.height / (Number(s.project.height) || 720), epoch: s.projectEpoch
            };
            (handle || rect).focus({ preventScroll: true });
            try { rect.setPointerCapture(event.pointerId); } catch (_) { /* synthetic pointer */ }
            el.classList.add('is-dragging');
            event.preventDefault(); event.stopPropagation();
        }, { signal });
        rect.addEventListener('pointermove', event => {
            const drag = s.cropDrag;
            if (!drag) return;
            const delta = cropDragDelta(event.clientX - drag.x, event.clientY - drag.y, drag.geometry.angle, drag.width, drag.height);
            drag.crop = cropDrag(drag.start, drag.geometry.flipH, drag.geometry.flipV, drag.handle, delta.dx, delta.dy);
            drawCropBox(s, el, drag.geometry, drag.crop, true);
        }, { signal });
        const finish = event => {
            const drag = s.cropDrag;
            if (!drag) return;
            s.cropDrag = null;
            el.classList.remove('is-dragging');
            if (event.type !== 'pointercancel' && drag.epoch === s.projectEpoch) commitCrop(s, drag.clipId, drag.start, drag.crop);
            updateCropBox(s);
        };
        rect.addEventListener('pointerup', finish, { signal });
        rect.addEventListener('pointercancel', finish, { signal });
        rect.addEventListener('lostpointercapture', () => { if (s.cropDrag) finish({ type: 'pointercancel' }); }, { signal });
        window.addEventListener('blur', () => { if (s.cropDrag) finish({ type: 'pointercancel' }); }, { signal });
        // Clicks inside the overlay stay there: no clip picking under the picture (mat click) and no focus jump to the app.
        el.addEventListener('click', event => event.stopPropagation(), { signal });
    }

    Object.assign(P, { syncInert, focusKey, restoreFocus, applyTransition, percentText, updateOutput, liveField, commitField, liveText, commitText, mutateSelected, syncDrawer, setInspectorOpen, POPOVERS, togglePopover, closePopovers, closePopoversOutside, renderProjectPopover, renameProject, wireResize, transformTarget, updateTransform, wireTransform, syncPreviewNotice, setCropMode, updateCropBox, wireCropBox, handleCropKeys, cropGeometry, cropDrag, cropDragDelta });
})();
