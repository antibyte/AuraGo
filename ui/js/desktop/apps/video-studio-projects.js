(function () {
    'use strict';

    // Project settings and the new-project dialog. Phase 1e adds the project manager here.
    const P = window.VideoStudioParts = window.VideoStudioParts || {};
    const { CANVASES } = P;
    const createProject = (...args) => P.createProject(...args);
    const esc = (...args) => P.esc(...args);
    const icon = (...args) => P.icon(...args);
    const loadProjects = (...args) => P.loadProjects(...args);
    const mutate = (...args) => P.mutate(...args);
    const openModal = (...args) => P.openModal(...args);
    const saveProject = (...args) => P.saveProject(...args);
    const saveUntilClean = (...args) => P.saveUntilClean(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);
    const Time = () => window.VideoStudioTime;

    function rateValue(rate) { return rate.fps + (rate.ntsc ? ':ntsc' : ''); }
    function parseRate(value) { const [fps, ntsc] = String(value).split(':'); return { fps: Number(fps) || 30, ntsc: ntsc === 'ntsc' }; }
    function sizes(s) { return s.status && s.status.limits && s.status.limits.canvas_sizes || CANVASES.map(c => ({ width: c.width, height: c.height })); }
    function rates(s) { return s.status && s.status.limits && s.status.limits.frame_rates || Time().FRAME_RATES; }

    function fieldsMarkup(s, values) {
        const locale = P.uiLocale();
        const formats = sizes(s).map(size => {
            const key = (CANVASES.find(c => c.width === size.width && c.height === size.height) || {}).key;
            const value = `${Number(size.width)}x${Number(size.height)}`;
            return `<option value="${value}" ${value === values.size ? 'selected' : ''}>${esc(s, key ? tr(s, key, `${size.width} × ${size.height}`) : `${size.width} × ${size.height}`)}</option>`;
        }).join('');
        const rateOptions = rates(s).map(rate => `<option value="${esc(s, rateValue(rate))}" ${rateValue(rate) === values.rate ? 'selected' : ''}>${esc(s, Time().rateLabel(rate, locale))}</option>`).join('');
        return `<label class="vs-field vs-field-wide"><span>${esc(s, tr(s, 'projectNameLabel', 'Project name'))}</span><input type="text" data-settings-name maxlength="120" required value="${esc(s, values.name)}"></label><div class="vs-field-row vs-settings-row"><label class="vs-field"><span>${esc(s, tr(s, 'format', 'Format'))}</span><select data-settings-size>${formats}</select></label><label class="vs-field"><span>${esc(s, tr(s, 'frameRate', 'Frame rate'))}</span><select data-settings-rate>${rateOptions}</select></label></div>`;
    }

    function readFields(host) {
        const [width, height] = host.querySelector('[data-settings-size]').value.split('x').map(Number);
        return Object.assign({ name: host.querySelector('[data-settings-name]').value.trim().slice(0, 120), width, height }, parseRate(host.querySelector('[data-settings-rate]').value));
    }

    function openProjectSettings(s) {
        if (!s.project || s.readonly) return;
        const projectId = s.projectId, epoch = s.projectEpoch;
        const values = { name: s.project.name || '', size: `${s.project.width}x${s.project.height}`, rate: rateValue({ fps: Number(s.project.fps) || 30, ntsc: !!s.project.ntsc }) };
        const host = openModal(s, `<form class="vs-modal" data-settings-form role="dialog" aria-modal="true" aria-labelledby="vs-settings-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('sliders', 22)}</span><h2 id="vs-settings-title-${esc(s, s.id)}">${esc(s, tr(s, 'projectSettingsTitle', 'Project settings'))}</h2><p>${esc(s, tr(s, 'projectSettingsIntro', 'Boxes keep their position relative to the frame when the format changes.'))}</p>${fieldsMarkup(s, values)}<div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${esc(s, tr(s, 'apply', 'Apply'))}</button></div></form>`);
        host.querySelector('[data-settings-form]').addEventListener('submit', async event => {
            event.preventDefault();
            const next = readFields(host);
            host.replaceChildren();
            s.app.focus({ preventScroll: true });
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const before = { fps: Number(s.project.fps) || 30, ntsc: !!s.project.ntsc };
            const rateChanged = next.fps !== before.fps || next.ntsc !== before.ntsc;
            if (rateChanged && !(await s.ctx.confirmDialog(tr(s, 'changeRateTitle', 'Change the frame rate?'), tr(s, 'changeRateCopy', 'All clip times are rounded to {{rate}}. You can undo this.').replace('{{rate}}', Time().rateLabel(next, P.uiLocale()))))) return;
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const ratio = Time().rate(next) / Time().rate(before);
            const renamed = !!next.name && next.name !== s.project.name;
            const ok = mutate(s, 'project settings', project => {
                if (next.name) project.name = next.name;
                project.width = next.width; project.height = next.height;
                if (rateChanged) Time().rescaleProject(project, next.fps, next.ntsc);
            });
            if (ok && rateChanged) { s.frame = Math.round(s.frame * ratio); s.preview.seek(s.frame); }
            if (!ok && rateChanged) showNotice(s, 'rateChangeFailed', 'The frame rate could not be changed because some clips would overlap.', true);
            // The project list shows names from the server; refresh it once the new name is saved.
            if (ok && renamed) saveProject(s).then(() => loadProjects(s)).catch(() => {});
        });
    }

    async function openNewProject(s) {
        if (s.readonly) return showNotice(s, 'readonly', 'Read-only mode prevents editing.', true);
        await P.flushArtwork(s);
        if (s.dirty && !(await saveUntilClean(s))) return;
        if (s.disposed) return;
        const values = { name: tr(s, 'untitled', 'Untitled project'), size: '1920x1080', rate: '30' };
        const host = openModal(s, `<form class="vs-modal" data-new-project-form role="dialog" aria-modal="true" aria-labelledby="vs-new-title-${esc(s, s.id)}"><button type="button" class="vs-modal-close" data-modal-close aria-label="${esc(s, tr(s, 'close', 'Close'))}">${icon('close', 18)}</button><span class="vs-modal-icon">${icon('plus', 22)}</span><h2 id="vs-new-title-${esc(s, s.id)}">${esc(s, tr(s, 'newProject', 'New project'))}</h2>${fieldsMarkup(s, values)}<div class="vs-modal-actions"><button type="button" data-modal-close>${esc(s, tr(s, 'cancel', 'Cancel'))}</button><button type="submit" class="vs-primary">${icon('plus', 16)}<span>${esc(s, tr(s, 'createProject', 'Create project'))}</span></button></div></form>`);
        host.querySelector('[data-settings-name]').select();
        host.querySelector('[data-new-project-form]').addEventListener('submit', async event => {
            event.preventDefault();
            const next = readFields(host);
            host.replaceChildren();
            if (!next.name) return;
            const settings = { width: next.width, height: next.height, fps: next.fps };
            if (next.ntsc) settings.ntsc = true;
            try { await createProject(s, next.name, settings); }
            catch (_) { showNotice(s, 'createFailed', 'Could not create the project.', true); }
        });
    }

    Object.assign(P, { openProjectSettings, openNewProject });
})();
