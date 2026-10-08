(function () {
    'use strict';

    // Popovers, the narrow drawer, timeline height, the preview transform box and inspector field edits.
    const P = window.VideoStudioParts;
    const { PREF_TIMELINE } = P;
    const I = (...args) => P.I(...args);
    const T = (...args) => P.T(...args);
    const canonicalProject = (...args) => P.canonicalProject(...args);
    const clamp = (...args) => P.clamp(...args);
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
        for (const attr of ['field', 'textField', 'textStyle', 'position', 'fit', 'transition', 'transitionDuration', 'textAlign', 'textToggle', 'action']) {
            if (active.dataset && active.dataset[attr] != null) return { attr: attr.replace(/[A-Z]/g, c => '-' + c.toLowerCase()), value: active.dataset[attr] };
        }
        return null;
    }
    function restoreFocus(s, key) {
        if (!key) return;
        const el = s.q('[data-inspector]').querySelector(`[data-${key.attr}${key.value ? `="${CSS.escape(key.value)}"` : ''}]`);
        if (el && !el.disabled) el.focus({ preventScroll: true });
    }
    // Transitions apply immediately: the outgoing clip owns an exact overlap with the next clip.
    function applyTransition(s, type, frames) {
        const found = T().clipFor(s.project, s.selectedClipId);
        if (!found || s.readonly || found.track.locked || found.track.kind === 'audio') return;
        const ordered = found.track.clips.slice().sort((a, b) => a.start - b.start);
        const next = ordered[ordered.findIndex(clip => clip.id === found.clip.id) + 1];
        if (type !== 'none' && !next) return showNotice(s, 'transitionNeedsNext', 'Add a following clip on this track first.', true);
        const duration = clamp(Math.round(Number(frames) || 15), 1, 90);
        if (type !== 'none' && duration >= Math.min(found.clip.duration, next.duration)) return showNotice(s, 'transitionTooLong', 'Transition duration must be shorter than both clips.', true);
        mutate(s, 'transition', project => {
            const track = project.tracks.find(item => item.id === found.track.id), clip = track.clips.find(item => item.id === found.clip.id);
            const target = track.clips.slice().sort((a, b) => a.start - b.start), following = target[target.findIndex(item => item.id === clip.id) + 1];
            if (type === 'none') { clip.transition = null; if (following) following.start = clip.start + clip.duration; }
            else { clip.transition = { type, duration }; following.start = clip.start + clip.duration - duration; }
        });
    }
    function percentText(value) { return Math.round(value) + ' %'; }
    function updateOutput(input, text) { const output = input.parentElement && input.parentElement.querySelector('output'); if (output) output.textContent = text; }
    function liveField(s, input) {
        const found = selectedClip(s);
        if (!found || found.track.locked) return;
        const field = input.dataset.field, value = Number(input.value);
        if (!input._vsBefore) { input._vsBefore = snapshot(s); input._vsBox = { x: found.clip.x, y: found.clip.y, width: found.clip.width, height: found.clip.height }; }
        clearTimeout(s.autosaveTimer);
        if (field === 'opacity') { found.clip.opacity = clamp(value / 100, 0, 1); updateOutput(input, percentText(value)); }
        else if (field === 'volume') { found.clip.volume = clamp(value / 100, 0, 4); updateOutput(input, percentText(value)); }
        else if (field === 'rotation') { found.clip.rotation = clamp(Math.round(value), -360, 360); updateOutput(input, Math.round(value) + '°'); }
        else if (field === 'size') { Object.assign(found.clip, I().scaleBox(input._vsBox, clamp(value / 100, 0.05, 1))); updateOutput(input, percentText(value)); }
        else if (field === 'fade_in' || field === 'fade_out') { found.clip[field] = clamp(Math.round(value), 0, found.clip.duration); updateOutput(input, I().formatSeconds(found.clip[field]) + ' s'); }
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
        if (['start', 'duration', 'offset'].includes(field)) {
            const frames = I().parseTime(input.value);
            if (frames == null) { showNotice(s, 'invalidTime', 'Enter a time like 4,5 or 1:04.5.', true); renderInspector(s); return; }
            const asset = s.project.assets.find(item => item.id === found.clip.asset_id) || {};
            mutate(s, field, project => {
                const target = T().clipFor(project, found.clip.id).clip;
                if (field === 'start') target.start = clamp(frames, 0, maxFrames(s) - target.duration);
                else if (field === 'offset') target.offset = asset.kind === 'image' ? 0 : clamp(frames, 0, Math.max(0, asset.duration_frames - target.duration));
                else {
                    const sourceLimit = asset.kind === 'image' || target.text ? maxFrames(s) : asset.duration_frames - target.offset;
                    target.duration = clamp(frames, 1, Math.min(sourceLimit, maxFrames(s) - target.start));
                    target.fade_in = Math.min(target.fade_in, target.duration); target.fade_out = Math.min(target.fade_out, target.duration);
                }
            });
            return;
        }
        if (['x', 'y', 'width', 'height'].includes(field)) {
            const value = Number(input.value) / 100;
            if (!Number.isFinite(value)) { renderInspector(s); return; }
            mutate(s, 'box', project => { const target = T().clipFor(project, found.clip.id).clip; Object.assign(target, I().clampBox(Object.assign({ x: target.x, y: target.y, width: target.width, height: target.height }, { [field]: value }))); });
            return;
        }
        const before = input._vsBefore; delete input._vsBefore; delete input._vsBox;
        if (!before) return;
        if (T().validTimeline && !T().validTimeline(s.project)) {
            s.project = hydrateProject(s, before);
            showNotice(s, 'invalidTiming', 'That edit would create invalid timing or an overlap.', true);
            renderUI(s); return;
        }
        s.history.push(before); if (s.history.length > 50) s.history.shift(); s.redo = [];
        s.dirty = JSON.stringify(canonicalProject(s.project)) !== JSON.stringify(s.savedProject);
        if (s.dirty) { persistDraft(s); scheduleSave(s); } else clearDraft(s);
        renderInspector(s); renderToolbar(s); T().render(s.q('[data-timeline]'), s.timelineOptions); s.preview.seek(s.frame);
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
        pop.innerHTML = `${s.project ? `<div class="vs-pop-section"><label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'projectNameLabel', 'Project name'))}</span><span class="vs-inline-field"><input type="text" data-project-rename maxlength="120" value="${esc(s, s.project.name || '')}" ${s.readonly ? 'disabled' : ''}><button type="button" data-action="rename-project" ${s.readonly ? 'disabled' : ''}>${esc(s, tr(s, 'rename', 'Rename'))}</button></span></label>${folder ? `<p class="vs-hint">${icon('folder', 13)} ${esc(s, tr(s, 'savedIn', 'Saved in'))} ${esc(s, folder)}</p>` : ''}</div>` : ''}<div class="vs-pop-section"><h4>${esc(s, tr(s, 'projects', 'Projects'))}</h4>${projects ? `<ul class="vs-project-list">${projects}</ul>` : `<p class="vs-hint">${esc(s, tr(s, 'noProjectsYet', 'No projects yet.'))}</p>`}</div><div class="vs-pop-foot"><button type="button" class="vs-primary" data-action="new-project" ${s.readonly ? 'disabled' : ''}>${icon('plus', 15)}<span>${esc(s, tr(s, 'newProject', 'New project'))}</span></button></div>`;
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
        const found = selectedClip(s);
        if (!found || s.readonly || found.track.locked || found.track.hidden || found.track.kind === 'audio') return null;
        const clip = found.clip;
        if (s.frame < clip.start || s.frame >= clip.start + clip.duration) return null;
        return found;
    }
    function updateTransform(s) {
        const box = s.q('[data-transform]');
        if (!box) return;
        const found = s.project && !s.q('.vs-no-project') ? transformTarget(s) : null;
        box.hidden = !found;
        if (!found) return;
        const clip = found.clip;
        box.style.left = clip.x * 100 + '%'; box.style.top = clip.y * 100 + '%';
        box.style.width = clip.width * 100 + '%'; box.style.height = clip.height * 100 + '%';
        box.style.transform = clip.rotation ? `rotate(${clip.rotation}deg)` : '';
        box.title = tr(s, 'dragToMove', 'Drag to move, corners to resize');
    }
    function wireTransform(s) {
        const box = s.q('[data-transform]'), mat = s.q('.vs-preview-mat'), signal = s.abort.signal;
        let gesture = null;
        box.addEventListener('pointerdown', event => {
            const found = transformTarget(s);
            if (!found || event.button !== 0 || s.timelineDragging) return;
            const rect = mat.getBoundingClientRect();
            const clip = found.clip;
            gesture = { clipId: clip.id, handle: event.target.closest('[data-handle]')?.dataset.handle || '', x: event.clientX, y: event.clientY, w: rect.width, h: rect.height, box: { x: clip.x, y: clip.y, width: clip.width, height: clip.height }, before: snapshot(s), epoch: s.projectEpoch, projectId: s.projectId };
            s.timelineDragging = true; s.transformDrag = true;
            clearTimeout(s.autosaveTimer);
            box.setPointerCapture(event.pointerId);
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
            Object.assign(found.clip, next);
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
            if (event.type === 'pointercancel') { s.project = hydrateProject(s, before); renderUI(s); return; }
            if (JSON.stringify(before) !== JSON.stringify(s.project)) recordChange(s, 'transform', before);
            P.retryArtwork(s);
            if (s.pendingRefresh) { s.pendingRefresh = false; refreshProject(s).catch(() => {}); }
        };
        box.addEventListener('pointerup', finish, { signal });
        box.addEventListener('pointercancel', finish, { signal });
        // Clicking the picture selects the front-most visible clip under the pointer.
        mat.addEventListener('click', event => {
            if (event.target.closest('[data-transform],[data-no-project],button') || !s.project) return;
            const rect = mat.getBoundingClientRect();
            const px = (event.clientX - rect.left) / rect.width, py = (event.clientY - rect.top) / rect.height;
            const hit = I().displayTracks(s.project.tracks).filter(track => track.kind !== 'audio' && !track.hidden).flatMap(track => track.clips.filter(clip => s.frame >= clip.start && s.frame < clip.start + clip.duration && px >= clip.x && px <= clip.x + clip.width && py >= clip.y && py <= clip.y + clip.height))[0];
            if ((hit ? hit.id : '') === s.selectedClipId) return;
            s.timelineOptions.onSelect(hit ? hit.id : '');
            T().render(s.q('[data-timeline]'), s.timelineOptions);
        }, { signal });
    }

    Object.assign(P, { focusKey, restoreFocus, applyTransition, percentText, updateOutput, liveField, commitField, liveText, commitText, mutateSelected, syncDrawer, setInspectorOpen, POPOVERS, togglePopover, closePopovers, closePopoversOutside, renderProjectPopover, renameProject, wireResize, transformTarget, updateTransform, wireTransform });
})();
