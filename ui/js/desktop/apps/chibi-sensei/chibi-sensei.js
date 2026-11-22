// Chibi Sensei window: the composition root. It owns the per-window instance (the state the other modules read and the bus they
// listen to), the start (meta call, state cards), the project list mount, the project layout (top bar, banners, step rail, main
// area, Sensei panel), step routing, the project event stream with its fan-out, the 2.5 s poll of active jobs, the read-only
// policy and the close guard. The step rail and the Sensei panel come from P.steps (derive, createRail, createPanel). Loaded last; publishes window.ChibiSenseiApp = {render, dispose, open}. Classic script.
(function () {
    'use strict';

    const P = window.ChibiSenseiParts = window.ChibiSenseiParts || {};
    const instances = new Map();
    const LIVE_CLEAR_MS = 5000;
    const POLL_MS = 2500;
    const REFRESH_MS = 1000;

    function createInstance(host, windowId, ctx) {
        const c = ctx || {};
        const inst = {
            windowId, host, ctx: c,
            t: (key, vars) => (typeof c.t === 'function' ? c.t(key, vars) : key),
            bus: P.createBus(), bag: P.bag(), api: P.createApi(), disposed: false,
            meta: null, ctxReadonly: !!c.readonly, forcedReadonly: false,
            view: 'loading',
            launchProjectId: P.UUID_RE.test(c.projectId || '') ? c.projectId : '',
            projectSeq: 0, project: null,
            step: 'goal', derived: null, primary: null,
            goalDirty: false, editorDirty: false,
            checks: null, checksStale: true, checksRunning: false,
            split: null, documents: [], jobs: new Map(), lastJobFailed: false,
            lastBatch: null,
            events: null, pollTimer: 0, liveTimer: 0, refresh: null,
            content: null, root: null,
            parts: {}
        };
        Object.defineProperty(inst, 'readonly', { get() { return inst.ctxReadonly || inst.forcedReadonly || !!(inst.meta && inst.meta.readonly); } });
        inst.notify = (text, kind) => notify(inst, text, kind);
        inst.askConfirm = (title, message) => askConfirm(inst, title, message);
        inst.handleError = err => handleError(inst, err);
        inst.showState = card => showState(inst, card);
        inst.openProject = id => openProject(inst, id);
        inst.showProjects = () => showProjects(inst);
        inst.showStep = step => showStep(inst, step);
        inst.recompute = () => recompute(inst);
        inst.runPrimary = action => runPrimary(inst, action);
        inst.applyJob = job => applyJob(inst, job);
        return inst;
    }

    // ---- small shared pieces ---------------------------------------------------------------------------------------------------

    function notify(inst, text, kind) {
        if (inst.disposed || !text) return;
        const live = inst.parts.live;
        if (!live) return;
        live.textContent = text;
        live.dataset.kind = kind || 'info';
        clearTimeout(inst.liveTimer);
        inst.liveTimer = setTimeout(() => { live.textContent = ''; }, LIVE_CLEAR_MS);
    }

    async function askConfirm(inst, title, message) {
        if (typeof inst.ctx.confirmDialog !== 'function') return false;
        try { return !!(await inst.ctx.confirmDialog(title, message)); } catch (_) { return false; }
    }

    // A read-only answer from the server turns the whole window read-only. The text is the sentence for the error.
    function handleError(inst, err) {
        const e = P.errors.normalize(err);
        if (e.code === 'desktop_readonly' && !inst.forcedReadonly) {
            inst.forcedReadonly = true;
            inst.bus.emit('readonly');
        }
        return P.errors.describe(inst.t, err);
    }

    function stateCard(inst, titleKey, textKey, actions, mood) {
        const { el, icon } = P;
        const t = inst.t;
        const art = mood && P.mascot ? P.mascot.create({ size: 88, mood }).el : icon('lock', 'csn-state-icon');
        return el('div', { class: 'csn-state', role: 'status', dataset: { csn: 'state', state: titleKey } },
            art,
            el('h2', { text: t(titleKey) }),
            el('p', { text: t(textKey) }),
            actions ? el('div', { class: 'csn-actions' }, actions) : null);
    }

    function retryButton(inst, action) {
        return P.el('button', { type: 'button', class: 'csn-btn csn-btn-primary', dataset: { csn: 'retry' }, onClick: action }, P.icon('refresh'), P.el('span', { text: inst.t('chibi_sensei.retry') }));
    }

    function startFailure(inst, err) {
        const e = P.errors.normalize(err);
        if (e.code === 'chibi_disabled') return stateCard(inst, 'chibi_sensei.state.disabled_title', 'chibi_sensei.state.disabled_text', null, 'sleeping');
        if (e.status === 401 || e.code === 'unauthorized') return stateCard(inst, 'chibi_sensei.state.unauthorized_title', 'chibi_sensei.state.unauthorized_text');
        if (e.code === 'desktop_scope_required') return stateCard(inst, 'chibi_sensei.state.scope_title', 'chibi_sensei.state.scope_text');
        const card = stateCard(inst, 'chibi_sensei.state.load_failed_title', 'chibi_sensei.state.load_failed_text', [retryButton(inst, () => start(inst))]);
        if (e.message) card.appendChild(P.el('code', { class: 'csn-detail', text: e.message.slice(0, 300) }));
        return card;
    }

    // The view area swaps (project list, project layout, state cards); the live region next to it stays.
    function showRoot(inst, node) {
        P.clear(inst.content);
        inst.content.appendChild(node);
    }

    // Where a state card goes: into the main area inside a project, else over the whole view area.
    function showState(inst, card) {
        if (inst.disposed) return;
        if (inst.view === 'project' && inst.parts.main) { P.clear(inst.parts.main); inst.parts.main.appendChild(card); return; }
        showRoot(inst, card);
    }

    function disposeView(inst) {
        const view = inst.parts.view;
        inst.parts.view = null;
        if (view && typeof view.dispose === 'function') {
            try { view.dispose(); } catch (err) { console.error('chibi-sensei view dispose failed', err); }
        }
    }

    function readonlyBanner(inst) {
        return P.el('div', { class: 'csn-banner', role: 'status', dataset: { banner: 'readonly', tone: 'info' } }, P.icon('lock'), P.el('span', { text: inst.t('chibi_sensei.readonly_banner') }));
    }

    // ---- start and project list ------------------------------------------------------------------------------------------------

    async function start(inst) {
        inst.view = 'loading';
        showRoot(inst, P.el('p', { class: 'csn-muted csn-loading', role: 'status', text: inst.t('chibi_sensei.loading') }));
        try {
            inst.meta = await inst.api.meta();
            if (inst.disposed) return;
            inst.bus.emit('readonly');
            const id = inst.launchProjectId;
            inst.launchProjectId = '';
            if (id && (await openProject(inst, id))) return;
            if (inst.disposed) return;
            await showProjects(inst);
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return;
            showRoot(inst, startFailure(inst, err));
        }
    }

    async function showProjects(inst) {
        closeProject(inst);
        inst.view = 'projects';
        if (!P.need(inst, 'projects')) return;
        const shell = P.el('div', { class: 'csn-start', dataset: { csn: 'start' } });
        const banners = P.el('div', { class: 'csn-banners', dataset: { csn: 'banners' } });
        const paintBanners = () => { P.clear(banners); if (inst.readonly) banners.appendChild(readonlyBanner(inst)); };
        paintBanners();
        inst.parts.banners = banners;
        const view = P.projects.createList(inst);
        inst.parts.view = view;
        shell.appendChild(banners);
        shell.appendChild(view.el);
        showRoot(inst, shell);
        inst.bag.add(inst.bus.on('readonly', () => { if (inst.view === 'projects') paintBanners(); }));
        if (typeof view.focus === 'function') view.focus();
    }

    // ---- project ---------------------------------------------------------------------------------------------------------------

    function closeProject(inst) {
        inst.projectSeq++;
        if (inst.events) { inst.events.close(); inst.events = null; }
        clearTimeout(inst.pollTimer);
        inst.pollTimer = 0;
        if (inst.refresh) { inst.refresh.cancel(); inst.refresh = null; }
        disposeView(inst);
        inst.project = null;
        inst.derived = null;
        inst.checks = null;
        inst.checksStale = true;
        inst.checksRunning = false;
        inst.split = null;
        inst.documents = [];
        inst.jobs = new Map();
        inst.lastJobFailed = false;
        inst.lastBatch = null;
        inst.goalDirty = false;
        inst.editorDirty = false;
        inst.parts.rail = inst.parts.railView = inst.parts.main = inst.parts.panel = inst.parts.panelView = inst.parts.banners = inst.parts.topbar = null;
        inst.primary = null;
    }

    // Loads everything one project needs in parallel. A project that cannot be loaded leaves the current view as it is and says why.
    async function openProject(inst, id) {
        if (!P.UUID_RE.test(String(id || ''))) return false;
        if (!P.need(inst, 'steps')) return false;
        let seq = ++inst.projectSeq;
        let loaded;
        try {
            const tolerant = promise => promise.catch(err => { if (err && err.aborted) throw err; return null; });
            loaded = await Promise.all([
                inst.api.getProject(id),
                tolerant(inst.api.getChecks(id)),
                tolerant(inst.api.getSplit(id)),
                tolerant(inst.api.listDocuments(id)),
                tolerant(inst.api.listJobs(id, true))
            ]);
        } catch (err) {
            if (inst.disposed || seq !== inst.projectSeq || (err && err.aborted)) return false;
            notify(inst, handleError(inst, err), 'error');
            return false;
        }
        if (inst.disposed || seq !== inst.projectSeq) return false;
        closeProject(inst);
        seq = inst.projectSeq;
        const [project, checks, split, documents, jobs] = loaded;
        inst.project = project;
        inst.checks = checks && checks.result ? checks.result : null;
        inst.checksStale = checks ? !!checks.stale || !checks.result : true;
        inst.split = split;
        inst.documents = Array.isArray(documents) ? documents : [];
        (Array.isArray(jobs) ? jobs : []).forEach(job => inst.jobs.set(job.id, job));
        inst.view = 'project';
        buildLayout(inst);
        openEvents(inst);
        recompute(inst);
        const first = P.steps.firstOpen(inst.derived);
        showStep(inst, P.STEP_IDS.indexOf(first) >= 0 ? first : 'goal');
        syncPoll(inst);
        inst.bus.emit('project', inst.project);
        return true;
    }

    function profileOf(inst) {
        const profiles = inst.meta && Array.isArray(inst.meta.profiles) ? inst.meta.profiles : [];
        const id = inst.project ? inst.project.profile_id : '';
        return profiles.find(p => p && p.id === id) || null;
    }

    function buildLayout(inst) {
        const { el, icon } = P;
        const t = inst.t;
        const back = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'back' }, onClick: () => leaveToProjects(inst) }, icon('left'), el('span', { text: t('chibi_sensei.back_to_projects') }));
        const name = el('h2', { class: 'csn-project-name', dataset: { csn: 'project-name' }, text: inst.project.name || '' });
        const rename = el('button', { type: 'button', class: 'csn-btn csn-btn-quiet', dataset: { csn: 'rename' }, onClick: () => renameProject(inst) }, icon('edit'), el('span', { text: t('chibi_sensei.rename') }));
        const chip = el('span', { class: 'csn-chip', dataset: { csn: 'model-chip' } });
        const badge = el('span', { class: 'csn-badge', text: t('chibi_sensei.experimental') });
        const topbar = el('header', { class: 'csn-topbar', dataset: { csn: 'topbar' } },
            back, el('h1', { class: 'csn-title', text: t('chibi_sensei.title') }), name, rename, el('span', { class: 'csn-spacer' }), chip, badge);
        const banners = el('div', { class: 'csn-banners', dataset: { csn: 'banners' } });
        const railView = P.steps.createRail(inst);
        const panelView = P.steps.createPanel(inst);
        const main = el('main', { class: 'csn-main', dataset: { csn: 'main' } });
        const body = el('div', { class: 'csn-body' }, railView.el, main, panelView.el);
        Object.assign(inst.parts, { topbar, banners, rail: railView.el, railView, main, panel: panelView.el, panelView, name, rename, chip, badge, back });
        showRoot(inst, el('div', { class: 'csn-project', dataset: { csn: 'project' } }, topbar, banners, body));
        const paintTop = () => {
            const profile = profileOf(inst);
            const label = profile ? (profile.display_name || profile.id) : (inst.project && inst.project.profile_id) || '';
            chip.textContent = label;
            chip.hidden = !label;
            badge.hidden = !!(profile && profile.status && profile.status !== 'experimental');
            name.textContent = inst.project ? inst.project.name || '' : '';
            rename.disabled = inst.readonly;
            rename.title = inst.readonly ? t('chibi_sensei.readonly_banner') : '';
            P.clear(banners);
            if (inst.readonly) banners.appendChild(readonlyBanner(inst));
        };
        paintTop();
        inst.bag.add(inst.bus.on('readonly', () => { if (inst.view === 'project') paintTop(); }));
        inst.bag.add(inst.bus.on('project', () => { if (inst.view === 'project') paintTop(); }));
    }

    async function leaveToProjects(inst) {
        if (!(await beforeClose(inst)) || inst.disposed) return;
        await showProjects(inst);
    }

    async function renameProject(inst) {
        if (inst.readonly || !inst.project || typeof inst.ctx.modalDialog !== 'function') return;
        let value = null;
        try { value = await inst.ctx.modalDialog({ title: inst.t('chibi_sensei.rename_title'), input: true, value: inst.project.name || '' }); } catch (_) { value = null; }
        if (inst.disposed || !inst.project || typeof value !== 'string' || !value || value === inst.project.name) return;
        const seq = inst.projectSeq;
        try {
            const updated = await inst.api.patchProject(inst.project.id, inst.project.rev, { name: value });
            if (inst.disposed || seq !== inst.projectSeq) return;
            inst.project = updated;
            inst.bus.emit('project', inst.project);
            recompute(inst);
            notify(inst, inst.t('chibi_sensei.saved'), 'info');
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return;
            notify(inst, handleError(inst, err), 'error');
        }
    }

    // ---- steps -----------------------------------------------------------------------------------------------------------------

    // Leaving a goal with unsaved changes asks first, so the draft is never dropped silently. Returns a promise only in that case.
    function showStep(inst, step) {
        if (inst.disposed || inst.view !== 'project' || !inst.parts.main) return undefined;
        const id = P.STEP_IDS.indexOf(step) >= 0 ? step : 'goal';
        if (inst.goalDirty && inst.step === 'goal' && id === 'goal') return undefined;
        if (inst.goalDirty && inst.step === 'goal' && id !== 'goal' && inst.parts.view) {
            return askConfirm(inst, inst.t('chibi_sensei.leave_title'), inst.t('chibi_sensei.leave_text')).then(ok => {
                if (!ok || inst.disposed) return;
                inst.goalDirty = false;
                enterStep(inst, id);
            });
        }
        if (inst.editorDirty && inst.step === 'data') {
            if (id === 'data') return undefined;
            return askConfirm(inst, inst.t('chibi_sensei.leave_title'), inst.t('chibi_sensei.editor.leave_text')).then(ok => {
                if (!ok || inst.disposed) return;
                inst.editorDirty = false;
                enterStep(inst, id);
            });
        }
        enterStep(inst, id);
        return undefined;
    }

    function enterStep(inst, id) {
        if (inst.disposed || inst.view !== 'project' || !inst.parts.main) return;
        disposeView(inst);
        P.clear(inst.parts.main);
        if (inst.parts.panelView) inst.parts.panelView.setExtra(null);
        inst.step = id;
        let view = null;
        if (id === 'goal') { if (P.need(inst, 'projects')) view = P.projects.createGoal(inst); }
        else if (id === 'data') { if (P.need(inst, 'data')) view = P.data.create(inst); }
        else if (id === 'training') { if (P.need(inst, 'training')) view = P.training.create(inst); }
        else if (P.need(inst, 'training')) view = P.training.createLater(inst, id);
        if (view) {
            inst.parts.view = view;
            if (view.el) inst.parts.main.appendChild(view.el);
        }
        inst.bus.emit('step', id);
        recompute(inst);
    }

    function derive(inst) {
        return P.steps.derive({
            project: inst.project, goalDirty: inst.goalDirty, checks: inst.checks, checksStale: inst.checksStale,
            jobs: Array.from(inst.jobs.values())
        });
    }

    // The goal step is valid when its draft is; every other step reads the saved project.
    function goalValid(inst) {
        const view = inst.parts.view;
        if (inst.step === 'goal' && view && typeof view.isValid === 'function') return !!view.isValid();
        return P.steps.goalComplete(inst.project);
    }

    function recompute(inst) {
        if (inst.disposed || inst.view !== 'project' || !inst.parts.railView || !inst.parts.panelView) return;
        inst.derived = derive(inst);
        inst.primary = P.steps.primaryFor(inst.step, inst.derived, {
            readonly: inst.readonly, goalValid: goalValid(inst), checksRunning: inst.checksRunning, rows: inst.project ? inst.project.rows : 0
        });
        inst.parts.railView.paint(inst.derived, inst.step);
        inst.parts.panelView.paint({ step: inst.step, derived: inst.derived, primary: inst.primary });
        inst.bus.emit('steps', inst.derived);
    }

    // The primary action of the step panel: save the goal and go on, run the checks, or go to training.
    async function runPrimary(inst, action) {
        const id = action || (inst.primary && inst.primary.action) || '';
        if (id === 'goal_continue') {
            const view = inst.parts.view;
            if (!inst.readonly && inst.step === 'goal' && view && typeof view.save === 'function') {
                const ok = await view.save();
                if (ok === false || inst.disposed) return;
            }
            showStep(inst, 'data');
        } else if (id === 'run_checks') {
            if (P.need(inst, 'data') && typeof P.data.runChecks === 'function') await P.data.runChecks(inst);
        } else if (id === 'goto_training') {
            showStep(inst, 'training');
        }
    }

    // ---- events, jobs and poll -------------------------------------------------------------------------------------------------

    function openEvents(inst) {
        const seq = inst.projectSeq;
        const live = fn => body => { if (!inst.disposed && seq === inst.projectSeq) fn(body); };
        inst.refresh = P.debounce(() => refreshProject(inst, seq), REFRESH_MS);
        inst.events = inst.api.events(inst.project.id, {
            onJob: live(job => applyJob(inst, job)),
            onRows: live(body => {
                inst.checksStale = true;
                inst.bus.emit('rows', body);
                if (inst.refresh) inst.refresh();
                recompute(inst);
            }),
            onChecks: live(() => {
                inst.checksStale = true;
                inst.bus.emit('checks', { result: inst.checks, stale: true });
                recompute(inst);
            })
        });
    }

    async function refreshProject(inst, seq) {
        if (inst.disposed || seq !== inst.projectSeq || !inst.project) return;
        const id = inst.project.id;
        const [project, split] = await Promise.all([inst.api.getProject(id).catch(() => null), inst.api.getSplit(id).catch(() => null)]);
        if (inst.disposed || seq !== inst.projectSeq) return;
        if (project) { inst.project = project; inst.bus.emit('project', project); }
        if (split) { inst.split = split; inst.bus.emit('split', split); }
        recompute(inst);
    }

    function isActive(job) { return !!job && (job.state === 'queued' || job.state === 'running'); }

    function applyJob(inst, job) {
        if (inst.disposed || !job || typeof job.id !== 'string' || !inst.project) return;
        if (job.project_id && job.project_id !== inst.project.id) return;
        inst.jobs.set(job.id, job);
        if (job.state === 'failed') inst.lastJobFailed = true;
        else if (job.state === 'queued') inst.lastJobFailed = false;
        inst.bus.emit('job', job);
        recompute(inst);
        syncPoll(inst);
    }

    function syncPoll(inst) {
        const any = Array.from(inst.jobs.values()).some(isActive);
        if (!any) { clearTimeout(inst.pollTimer); inst.pollTimer = 0; return; }
        if (inst.pollTimer) return;
        const seq = inst.projectSeq;
        inst.pollTimer = setTimeout(() => { inst.pollTimer = 0; pollJobs(inst, seq); }, POLL_MS);
    }

    // One poll round: the active list refreshes the known jobs; a job that is no longer listed is read once to learn how it ended.
    async function pollJobs(inst, seq) {
        if (inst.disposed || seq !== inst.projectSeq || !inst.project) return;
        const known = Array.from(inst.jobs.values()).filter(isActive).map(job => job.id);
        try {
            const active = await inst.api.listJobs(inst.project.id, true);
            if (inst.disposed || seq !== inst.projectSeq) return;
            const listed = new Set();
            active.forEach(job => { listed.add(job.id); applyJob(inst, job); });
            for (const id of known) {
                if (listed.has(id)) continue;
                const job = await inst.api.getJob(id);
                if (inst.disposed || seq !== inst.projectSeq) return;
                applyJob(inst, job);
            }
        } catch (err) {
            if (inst.disposed || (err && err.aborted)) return;
        }
        if (!inst.disposed && seq === inst.projectSeq) syncPoll(inst);
    }

    // ---- window ----------------------------------------------------------------------------------------------------------------

    async function beforeClose(inst) {
        if (!inst.goalDirty && !inst.editorDirty) return true;
        return askConfirm(inst, inst.t('chibi_sensei.leave_title'), inst.t('chibi_sensei.leave_text'));
    }

    function render(host, windowId, ctx) {
        dispose(windowId);
        const inst = createInstance(host, windowId, ctx);
        instances.set(windowId, inst);
        inst.content = P.el('div', { class: 'csn-view' });
        inst.parts.live = P.el('div', { class: 'csn-live', role: 'status', ariaLive: 'polite', dataset: { csn: 'live' } });
        inst.root = P.el('div', { class: 'csn-app', dataset: { csn: 'app' } }, inst.content, inst.parts.live);
        P.clear(host);
        host.appendChild(inst.root);
        inst.bag.on(document, 'aurago:desktop-policy', event => {
            const detail = event && event.detail;
            if (!detail || typeof detail.readonly !== 'boolean') return;
            inst.ctxReadonly = detail.readonly;
            if (!detail.readonly) inst.forcedReadonly = false;
            inst.bus.emit('readonly');
        });
        if (typeof inst.ctx.setWindowBeforeClose === 'function') inst.ctx.setWindowBeforeClose(windowId, () => beforeClose(inst));
        start(inst);
        return inst;
    }

    function dispose(windowId) {
        const inst = instances.get(windowId);
        if (!inst) return;
        instances.delete(windowId);
        inst.disposed = true;
        clearTimeout(inst.liveTimer);
        inst.api.abort();
        closeProject(inst);
        inst.bag.dispose();
        inst.bus.clear();
    }

    // A running window opens the project a launch names (from P4 on). Only UUIDs are accepted.
    async function open(windowId, projectId) {
        const inst = instances.get(windowId);
        if (!inst || typeof projectId !== 'string' || !P.UUID_RE.test(projectId)) return false;
        if (inst.view === 'loading') { inst.launchProjectId = projectId; return true; }
        if (inst.view === 'project' && inst.project && inst.project.id === projectId) return true;
        if (!(await beforeClose(inst)) || inst.disposed) return false;
        return openProject(inst, projectId);
    }

    window.ChibiSenseiApp = { render, dispose, open };
})();
