// CYD Creator window (layout A "Werkbank"): toolbar, Design/Code switch, screens + palette left,
// canvas or code editor in the centre, Properties | Agent on the right, the bottom dock. Owns the
// per-window inst object (see the overview's "inst" contract), the layout store + autosave, the
// event stream, toolchain/lock/readonly states and window disposal. Loaded last.
(function () {
    'use strict';

    const P = window.CYDCreatorParts = window.CYDCreatorParts || {};
    const instances = new Map();
    const SAVE_RANK = ['saved', 'dirty', 'saving', 'offline', 'locked', 'failed', 'conflict'];
    const ACTIVE_BUILD = new Set(['queued', 'running']);
    const PULL_POLL_MS = 2000;
    // After this many stream failures in a row the window asks the API whether reconnecting can work.
    const STREAM_PROBE_AFTER = 3;

    function createInstance(host, windowId, ctx) {
        const c = ctx || {};
        const bus = P.createBus();
        const inst = {
            windowId,
            host,
            ctx: c,
            t: P.i18n(null),
            api: P.api,
            bus,
            caps: null,
            projectId: '',
            project: null,
            readonly: !!c.readonly,
            // ctxReadonly is the desktop's read-only policy (ctx.readonly, then aurago:desktop-policy).
            ctxReadonly: !!c.readonly,
            layoutStore: null,
            layoutVersion: 0,
            layoutSnapshot: null,
            layoutSaver: null,
            dock: null,
            agent: null,
            code: null,
            designer: null,
            home: null,
            mode: 'design',
            panel: 'properties',
            root: null,
            disposed: false,
            bag: P.util.bag(),
            view: null,
            events: null,
            codeState: 'saved',
            codeMounted: false,
            projectBag: null,
            // projectSeq changes whenever a project view is torn down: async work of an earlier
            // project (or of the start screen it replaced) compares it before touching the window.
            projectSeq: 0,
            streamAfter: 0,
            probing: false,
            // uploads holds the projectSeq of every asset upload in flight (see uploadAsset).
            uploads: [],
            // skippedAssets: projectSeq → highest version of the assets events skipped while uploads ran.
            skippedAssets: new Map(),
            layoutIssues: [],
            pullTimer: 0
        };
        inst.domId = suffix => 'cc-' + String(windowId).replace(/[^A-Za-z0-9_-]/g, '_') + '-' + suffix;
        inst.isLocked = () => !!(inst.project && inst.project.locked_by_job);
        inst.layout = () => ({ version: inst.layoutVersion, project: inst.layoutStore ? inst.layoutStore.get() : inst.layoutSnapshot });
        inst.assetUrl = assetId => P.api.assetUrl(inst.projectId, assetId);
        // notify stays safe after the window (or the start screen) is gone: a late failure of a
        // dismissed create still reaches the desktop's notifications.
        inst.notify = (kind, text) => {
            if (!text || typeof c.notify !== 'function') return;
            try {
                c.notify({ title: inst.t('cyd_creator.app.title'), message: text, type: kind === 'error' ? 'error' : 'info' });
            } catch (err) {
                console.error('cyd-creator notify failed', err);
            }
        };
        inst.confirm = spec => {
            const s = spec || {};
            const d = P.dialog.open(inst.root, {
                title: inst.t(s.titleKey || 'cyd_creator.app.confirm_title', s.vars),
                body: s.bodyKey ? P.util.el('p', { text: inst.t(s.bodyKey, s.vars) }) : null,
                actions: [{ id: 'cancel', label: inst.t('cyd_creator.app.cancel') }, { id: 'ok', label: inst.t(s.okKey || 'cyd_creator.app.ok', s.vars), kind: s.danger ? 'danger' : 'primary' }],
                owner: inst.bag
            });
            return d.promise.then(value => value === 'ok');
        };
        inst.lastGreenBuild = () => (inst.dock ? inst.dock.builds.list().find(build => build.status === 'succeeded') : null) || null;
        inst.openProject = (id, opts) => openProject(inst, id, opts);
        inst.openFile = (path, line) => openFile(inst, path, line);
        inst.reveal = spec => reveal(inst, spec);
        inst.setMode = mode => setMode(inst, mode);
        inst.showPanel = id => showPanel(inst, id);
        inst.setPanelBadge = (id, on) => setPanelBadge(inst, id, on);
        inst.refreshProject = () => refreshProject(inst);
        inst.reloadProject = () => reloadProject(inst);
        inst.renderToolchain = el => renderToolchain(inst, el);
        inst.uploadAsset = file => uploadAsset(inst, file);
        inst.flushAll = () => flushAll(inst);
        // Agent jobs start from the saved project. Every start (composer, generate code, resume, the
        // description of a new project) first saves unsaved layout and code edits; when they cannot be
        // saved the job does not start and the save error says why. The agent's restore flushes itself.
        inst.api = Object.assign(Object.create(P.api), {
            startJob: async (id, body) => {
                if (!(await flushAll(inst))) throw unsavedError(inst);
                return P.api.startJob(id, body);
            }
        });
        return inst;
    }

    // isCurrent: no other project view (or the start screen) replaced the one async work started in.
    function isCurrent(inst, seq) {
        return !inst.disposed && seq === inst.projectSeq;
    }

    // problemDetail shows details.problem of CYD_DISABLED (a generic reason, no paths or secrets).
    function problemDetail(err) {
        const problem = err && err.details && typeof err.details.problem === 'string' ? err.details.problem : '';
        return problem ? P.util.el('code', { class: 'cc-banner-detail', text: problem }) : null;
    }

    // ── start and states ────────────────────────────────────────────────────

    function stateCard(inst, titleKey, text, actions, detail) {
        const { el } = P.util;
        return el('div', { class: 'cc-state-card', dataset: { ccState: titleKey } },
            el('h2', { text: inst.t(titleKey) }),
            el('p', { text }),
            detail || null,
            actions ? el('div', { class: 'cc-state-actions' }, actions) : null);
    }

    async function start(inst) {
        const { el } = P.util;
        inst.root.replaceChildren(el('div', { class: 'cc-loading', role: 'status', text: inst.t('cyd_creator.app.loading') }));
        try {
            inst.caps = await P.api.capabilities();
        } catch (err) {
            if (inst.disposed) return;
            const disabled = err && (err.code === 'CYD_DISABLED' || err.status === 404);
            const retry = el('button', { type: 'button', class: 'cc-btn cc-btn--primary', dataset: { ccAction: 'retry' }, text: inst.t('cyd_creator.app.retry'), onClick: () => start(inst) });
            inst.root.replaceChildren(stateCard(inst, disabled ? 'cyd_creator.app.disabled_title' : 'cyd_creator.app.error_title',
                disabled ? inst.t('cyd_creator.error.CYD_DISABLED') : P.errors.message(inst.t, err), retry, problemDetail(err)));
            return;
        }
        if (inst.disposed) return;
        if (inst.caps.enabled === false) {
            inst.root.replaceChildren(stateCard(inst, 'cyd_creator.app.disabled_title', inst.t('cyd_creator.error.CYD_DISABLED'),
                el('button', { type: 'button', class: 'cc-btn cc-btn--primary', dataset: { ccAction: 'retry' }, text: inst.t('cyd_creator.app.retry'), onClick: () => start(inst) })));
            return;
        }
        inst.readonly = !!(inst.ctxReadonly || inst.caps.readonly);
        if (inst.caps.toolchain && inst.caps.toolchain.pulling) pollToolchain(inst);
        if (inst.ctx.projectId) await openProject(inst, inst.ctx.projectId);
        else showHome(inst);
    }

    function showHome(inst) {
        teardownProject(inst);
        if (inst.home) inst.home.dispose();
        inst.home = P.home.create(inst);
        const { el } = P.util;
        const scroller = el('div', { class: 'cc-home-scroll' });
        inst.root.replaceChildren(scroller);
        inst.root.dataset.ccView = 'home';
        inst.home.mount(scroller);
    }

    // setReadonly applies a read-only change (the desktop policy or the capabilities): the toolbar,
    // the toolchain banner (its install button), the code editor, the agent panel (it re-renders on
    // project:updated) and the start screen.
    function setReadonly(inst) {
        const next = !!(inst.ctxReadonly || (inst.caps && inst.caps.readonly));
        if (next === inst.readonly) return;
        inst.readonly = next;
        renderToolbar(inst);
        renderSaveState(inst);
        renderToolchain(inst, inst.view && inst.view.toolchainSlot);
        if (inst.code) inst.code.updateReadOnly();
        if (inst.project) inst.bus.emit('project:updated', inst.project);
        inst.bus.emit('readonly:changed', next);
        if (inst.home) inst.home.refresh();
    }

    // ── toolchain ───────────────────────────────────────────────────────────

    function toolchain(inst) {
        return (inst.caps && inst.caps.toolchain) || { docker_available: false, present: false, pulling: false, progress: 0, error: '' };
    }

    function buildBlocker(inst) {
        const tc = toolchain(inst);
        if (inst.readonly) return 'cyd_creator.app.readonly_hint';
        // A fenced project refuses builds (CYD_BUSY publication_review) until the fence is resolved.
        if (inst.project && inst.project.publish_review) return 'cyd_creator.error.publication_review';
        if (inst.isLocked()) return 'cyd_creator.app.locked_banner';
        if (!tc.docker_available) return 'cyd_creator.error.CYD_DOCKER_UNAVAILABLE';
        if (!tc.present) return 'cyd_creator.error.CYD_IMAGE_MISSING';
        return '';
    }

    function renderToolchain(inst, container) {
        const { el } = P.util;
        const t = inst.t;
        const tc = toolchain(inst);
        if (!container) return;
        if (tc.docker_available && tc.present) { container.replaceChildren(); container.hidden = true; return; }
        container.hidden = false;
        if (!tc.docker_available) {
            container.replaceChildren(el('div', { class: 'cc-banner cc-banner--danger', role: 'status', dataset: { ccToolchainState: 'docker' } },
                el('strong', { text: t('cyd_creator.error.CYD_DOCKER_UNAVAILABLE') }), el('span', { text: t('cyd_creator.app.docker_help') })));
            return;
        }
        if (tc.pulling) {
            const percent = Math.round(P.util.clamp(tc.progress, 0, 1) * 100);
            container.replaceChildren(el('div', { class: 'cc-banner', role: 'status', dataset: { ccToolchainState: 'pulling' } },
                el('span', { text: t('cyd_creator.app.toolchain_pulling', { percent }) }),
                el('progress', { class: 'cc-progress', max: '100', value: String(percent), 'aria-label': t('cyd_creator.app.toolchain_pulling', { percent }) })));
            return;
        }
        const pull = el('button', { type: 'button', class: 'cc-btn cc-btn--primary', dataset: { ccAction: 'pull-toolchain' }, disabled: inst.readonly, text: t(tc.error ? 'cyd_creator.app.toolchain_retry' : 'cyd_creator.app.toolchain_pull') });
        pull.addEventListener('click', () => pullToolchain(inst));
        container.replaceChildren(el('div', { class: 'cc-banner ' + (tc.error ? 'cc-banner--danger' : 'cc-banner--warn'), role: 'status', dataset: { ccToolchainState: tc.error ? 'error' : 'missing' } },
            el('span', { text: tc.error ? t('cyd_creator.error.CYD_IMAGE_PULL_FAILED') : t('cyd_creator.app.toolchain_missing') }),
            tc.error ? el('code', { class: 'cc-banner-detail', text: String(tc.error) }) : null,
            pull));
    }

    async function pullToolchain(inst) {
        try {
            const status = await P.api.pullToolchain();
            if (inst.disposed) return;
            inst.caps.toolchain = Object.assign({}, toolchain(inst), status || {}, { pulling: true });
            toolchainChanged(inst);
            pollToolchain(inst);
        } catch (err) {
            inst.notify('error', P.errors.message(inst.t, err));
        }
    }

    function pollToolchain(inst) {
        if (inst.pullTimer) return;
        inst.pullTimer = setInterval(async () => {
            try {
                const caps = await P.api.capabilities();
                if (inst.disposed) return;
                inst.caps.toolchain = caps.toolchain;
                toolchainChanged(inst);
                if (!caps.toolchain || !caps.toolchain.pulling) { clearInterval(inst.pullTimer); inst.pullTimer = 0; }
            } catch (_) { /* keep polling: the pull continues server-side */ }
        }, PULL_POLL_MS);
    }

    function toolchainChanged(inst) {
        if (inst.view) {
            renderToolchain(inst, inst.view.toolchainSlot);
            renderToolbar(inst);
        }
        inst.bus.emit('toolchain:changed', toolchain(inst));
    }

    // ── project view ────────────────────────────────────────────────────────

    function buildView(inst) {
        const { el } = P.util;
        const t = inst.t;
        const btn = (action, label, extra) => el('button', Object.assign({ type: 'button', class: 'cc-btn', dataset: { ccAction: action }, text: label }, extra || {}));
        const v = {};
        v.homeBtn = el('button', { type: 'button', class: 'cc-icon-btn', dataset: { ccAction: 'home' }, 'aria-label': t('cyd_creator.toolbar.home'), title: t('cyd_creator.toolbar.home'), text: '‹' });
        v.title = el('span', { class: 'cc-project-title', dataset: { ccProjectTitle: '' } });
        v.boardChip = el('span', { class: 'cc-board-chip' });
        // extra replaces the default class: the segment buttons keep the button frame and add .cc-seg.
        v.modeDesign = btn('mode-design', t('cyd_creator.toolbar.design'), { class: 'cc-btn cc-seg', 'aria-pressed': 'true' });
        v.modeCode = btn('mode-code', t('cyd_creator.toolbar.code'), { class: 'cc-btn cc-seg', 'aria-pressed': 'false' });
        v.saveChip = el('span', { class: 'cc-pill', role: 'status', dataset: { ccSaveState: '' } });
        v.streamChip = el('span', { class: 'cc-pill', dataset: { ccStreamState: '' }, hidden: true, text: t('cyd_creator.app.reconnecting') });
        v.readonlyChip = el('span', { class: 'cc-pill', dataset: { tone: 'warning' }, hidden: true, text: t('cyd_creator.app.readonly') });
        v.buildBtn = btn('build', t('cyd_creator.toolbar.build'), { class: 'cc-btn cc-btn--primary' });
        v.flashBtn = btn('flash', t('cyd_creator.toolbar.flash'));
        v.otaBtn = btn('ota', t('cyd_creator.toolbar.ota'));
        v.exportBtn = btn('export', t('cyd_creator.toolbar.export'));
        v.revisionsBtn = btn('revisions', t('cyd_creator.toolbar.revisions'), { class: 'cc-btn cc-btn--ghost' });
        v.toolbar = el('header', { class: 'cc-toolbar', role: 'toolbar', 'aria-label': t('cyd_creator.toolbar.label') },
            v.homeBtn,
            el('div', { class: 'cc-toolbar-project' }, v.title, v.boardChip),
            el('div', { class: 'cc-segmented', role: 'group', 'aria-label': t('cyd_creator.toolbar.mode') }, v.modeDesign, v.modeCode),
            el('span', { class: 'cc-dock-spacer' }),
            v.readonlyChip, v.streamChip, v.saveChip,
            v.revisionsBtn, v.exportBtn, v.otaBtn, v.flashBtn, v.buildBtn);
        v.lockBanner = el('div', { class: 'cc-banner cc-banner--lock', role: 'status', dataset: { ccLock: '' }, hidden: true },
            el('span', { class: 'cc-spinner', 'aria-hidden': 'true' }), el('span', { text: t('cyd_creator.app.locked_banner') }));
        // A project fenced after an interrupted publication refuses every change until it is resolved.
        // renderToolbar inserts the sentence when the banner appears, so screen readers announce it.
        v.reviewText = el('span', { class: 'cc-banner-text' });
        v.reviewBanner = el('div', { class: 'cc-banner cc-banner--danger', role: 'alert', dataset: { ccPublishReview: '' }, hidden: true }, v.reviewText);
        // The event stream stopped for good (see probeStream): its reason and a Retry button.
        v.streamBanner = el('div', { class: 'cc-banner cc-banner--danger', role: 'alert', dataset: { ccStreamStopped: '' }, hidden: true });
        v.toolchainSlot = el('div', { class: 'cc-toolchain-slot', dataset: { ccToolchain: '' } });
        v.screensHost = el('div', { class: 'cc-left-screens', dataset: { ccScreensHost: '' } });
        v.paletteHost = el('div', { class: 'cc-left-palette', dataset: { ccPaletteHost: '' } });
        v.left = el('aside', { class: 'cc-left', 'aria-label': t('cyd_creator.app.left_label') }, v.screensHost, v.paletteHost);
        v.canvasHost = el('div', { class: 'cc-center-design', dataset: { ccCanvasHost: '' } });
        v.codeHost = el('div', { class: 'cc-center-code', hidden: true });
        v.center = el('main', { class: 'cc-center' }, v.canvasHost, v.codeHost);
        v.tabProps = el('button', { type: 'button', role: 'tab', class: 'cc-panel-tab', id: inst.domId('tab-properties'), 'aria-controls': inst.domId('panel-properties'), dataset: { ccPanel: 'properties' } },
            el('span', { text: t('cyd_creator.app.panel_properties') }));
        v.agentDot = el('span', { class: 'cc-dot', hidden: true, 'aria-label': t('cyd_creator.agent.running'), dataset: { ccAgentDot: '' } });
        v.tabAgent = el('button', { type: 'button', role: 'tab', class: 'cc-panel-tab', id: inst.domId('tab-agent'), 'aria-controls': inst.domId('panel-agent'), dataset: { ccPanel: 'agent' } },
            el('span', { text: t('cyd_creator.app.panel_agent') }), v.agentDot);
        v.inspectorHost = el('div', { class: 'cc-panel', role: 'tabpanel', id: inst.domId('panel-properties'), 'aria-labelledby': inst.domId('tab-properties'), dataset: { ccInspectorHost: '' } });
        v.agentHost = el('div', { class: 'cc-panel', role: 'tabpanel', id: inst.domId('panel-agent'), 'aria-labelledby': inst.domId('tab-agent'), hidden: true, dataset: { ccAgentHost: '' } });
        v.right = el('aside', { class: 'cc-right' },
            el('div', { class: 'cc-panel-tabs', role: 'tablist', 'aria-label': t('cyd_creator.app.right_label') }, v.tabProps, v.tabAgent),
            v.inspectorHost, v.agentHost);
        v.workbench = el('div', { class: 'cc-workbench' }, v.left, v.center, v.right);
        v.root = el('div', { class: 'cc-project' }, v.toolbar, v.lockBanner, v.reviewBanner, v.streamBanner, v.toolchainSlot, v.workbench);
        return v;
    }

    function placeholder(inst, key) {
        return P.util.el('div', { class: 'cc-empty cc-designer-placeholder', dataset: { ccDesignerPlaceholder: '' }, text: inst.t(key) });
    }

    function wireView(inst) {
        const v = inst.view;
        const b = inst.projectBag;
        b.on(v.homeBtn, 'click', async () => {
            if (!(await leaveProject(inst))) return;
            showHome(inst);
        });
        b.on(v.modeDesign, 'click', () => setMode(inst, 'design'));
        b.on(v.modeCode, 'click', () => setMode(inst, 'code'));
        b.on(v.buildBtn, 'click', () => startBuild(inst));
        b.on(v.revisionsBtn, 'click', () => inst.agent && inst.agent.openRevisions());
        b.on(v.flashBtn, 'click', () => P.flasher && P.flasher.open(inst));
        b.on(v.otaBtn, 'click', () => P.devices && P.devices.open(inst));
        b.on(v.exportBtn, 'click', () => P.transfer && P.transfer.exportProject(inst));
        [v.tabProps, v.tabAgent].forEach(tab => {
            b.on(tab, 'click', () => showPanel(inst, tab.dataset.ccPanel));
            b.on(tab, 'keydown', event => {
                if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
                event.preventDefault();
                const other = tab === v.tabProps ? v.tabAgent : v.tabProps;
                showPanel(inst, other.dataset.ccPanel);
                other.focus();
            });
        });
    }

    function renderToolbar(inst) {
        const v = inst.view;
        if (!v) return;
        const t = inst.t;
        const project = inst.project || {};
        v.title.textContent = project.name || '';
        const board = ((inst.caps && inst.caps.boards) || []).find(x => x.id === project.board);
        v.boardChip.textContent = board ? board.label : (project.board || '');
        v.boardChip.title = board ? t.or('cyd_creator.board.status_' + board.status, board.status) : '';
        v.flashBtn.hidden = !(P.flasher && typeof P.flasher.open === 'function');
        v.otaBtn.hidden = !(P.devices && typeof P.devices.open === 'function');
        v.exportBtn.hidden = !(P.transfer && typeof P.transfer.exportProject === 'function');
        const running = inst.dock && inst.dock.builds.list().some(build => ACTIVE_BUILD.has(build.status));
        const blocker = buildBlocker(inst);
        v.buildBtn.disabled = !!blocker || !!running;
        v.buildBtn.title = blocker ? t(blocker) : (running ? t('cyd_creator.build.status_running') : t('cyd_creator.toolbar.build_hint'));
        v.readonlyChip.hidden = !inst.readonly;
        v.lockBanner.hidden = !inst.isLocked();
        v.reviewBanner.hidden = !project.publish_review;
        const reviewText = project.publish_review ? t('cyd_creator.error.publication_review') : '';
        if (v.reviewText.textContent !== reviewText) v.reviewText.textContent = reviewText;
        inst.root.classList.toggle('is-locked', inst.isLocked());
        inst.root.classList.toggle('is-readonly', !!inst.readonly);
    }

    function renderSaveState(inst) {
        const v = inst.view;
        if (!v) return;
        const saver = inst.layoutSaver;
        const layoutState = saver ? saver.state() : 'saved';
        const worst = SAVE_RANK[Math.max(SAVE_RANK.indexOf(layoutState), SAVE_RANK.indexOf(inst.codeState))] || 'saved';
        v.saveChip.textContent = inst.t('cyd_creator.code.state_' + worst);
        v.saveChip.dataset.state = worst;
        v.saveChip.hidden = !!inst.readonly;
        // The reason of a failed or offline layout save (e.g. the publication fence) is the tooltip.
        const err = (worst === 'failed' || worst === 'offline') && layoutState === worst ? saver.error() : null;
        v.saveChip.title = err ? P.errors.message(inst.t, err) : '';
    }

    function setMode(inst, mode) {
        const v = inst.view;
        if (!v) return;
        inst.mode = mode === 'code' ? 'code' : 'design';
        const code = inst.mode === 'code';
        v.modeDesign.setAttribute('aria-pressed', code ? 'false' : 'true');
        v.modeCode.setAttribute('aria-pressed', code ? 'true' : 'false');
        v.canvasHost.hidden = code;
        v.codeHost.hidden = !code;
        v.left.hidden = code;
        inst.root.dataset.ccMode = inst.mode;
        if (code && inst.code && !inst.codeMounted) {
            inst.codeMounted = true;
            inst.code.mount(v.codeHost);
        }
        inst.bus.emit('mode:changed', inst.mode);
    }

    function showPanel(inst, id) {
        const v = inst.view;
        if (!v) return;
        inst.panel = id === 'agent' ? 'agent' : 'properties';
        const agent = inst.panel === 'agent';
        v.tabProps.setAttribute('aria-selected', agent ? 'false' : 'true');
        v.tabAgent.setAttribute('aria-selected', agent ? 'true' : 'false');
        v.tabProps.tabIndex = agent ? -1 : 0;
        v.tabAgent.tabIndex = agent ? 0 : -1;
        v.inspectorHost.hidden = agent;
        v.agentHost.hidden = !agent;
    }

    function setPanelBadge(inst, id, on) {
        const v = inst.view;
        if (!v || id !== 'agent') return;
        v.agentDot.hidden = !on;
        v.tabAgent.classList.toggle('is-busy', !!on);
    }

    // ── layout ──────────────────────────────────────────────────────────────

    function emitIssues(inst, issues) {
        inst.layoutIssues = Array.isArray(issues) ? issues : [];
        inst.bus.emit('layout:issues', inst.layoutIssues);
    }

    function layoutIdle(inst) {
        const saver = inst.layoutSaver;
        return !saver || (!saver.dirty() && saver.state() === 'saved');
    }

    function setupLayout(inst, payload) {
        inst.layoutVersion = Number(payload.version) || 0;
        inst.layoutSnapshot = P.util.deepFreeze(P.util.deepClone(payload.project));
        if (P.model && typeof P.model.createStore === 'function') {
            inst.layoutStore = P.model.createStore(payload.project, { limit: 300, coalesceMs: 1200 });
        }
        // Every edit is saved, also one with validation issues: the designer applies edits without a
        // gate, the server stores semantic issues and returns them (only an unparseable layout is a 422).
        inst.layoutSaver = P.autosave.create({
            token: inst.layoutVersion,
            save: (project, version) => P.api.putLayout(inst.projectId, project, version).then(res => {
                inst.layoutVersion = Number(res.version) || inst.layoutVersion;
                inst.layoutSnapshot = P.util.deepFreeze(P.util.deepClone(project));
                emitIssues(inst, res.issues);
                return inst.layoutVersion;
            }),
            onState: (state, err) => {
                renderSaveState(inst);
                // A save refused because the project is fenced: the summary then reports
                // publish_review, which shows the window banner.
                if (state === 'failed' && P.errors.reason(err) === 'publication_review' && inst.project && !inst.project.publish_review) refreshProject(inst);
            },
            onConflict: err => resolveLayoutConflict(inst, err)
        });
        if (inst.layoutStore) {
            inst.projectBag.add(inst.layoutStore.subscribe((project, info) => {
                const origin = info && info.origin;
                if (inst.readonly || (origin !== 'local' && origin !== 'undo' && origin !== 'redo')) return;
                inst.layoutSaver.change(project);
            }));
        }
        // Emitted once every part listens (end of openProject), so dock and designer see the first issues.
        inst.layoutIssues = Array.isArray(payload.issues) ? payload.issues : [];
    }

    // reloadLayout reads the saved layout and replaces the window's copy (store, snapshot, saver token).
    // o.ifIdle (a change from elsewhere, a stream reload, the window's own upload) replaces only while
    // nothing is unsaved, checked once the answer is in: an edit made during the request stays and
    // meets the new version through the If-Match conflict choice. o.newer drops an answer older than
    // the window's version (a save, e.g. an Overwrite, landed meanwhile); a stream reload does not use
    // it, since a restored database may go back. o.keepHistory keeps the undo history (the window's
    // own upload). Returns the answer, applied or not.
    async function reloadLayout(inst, opts) {
        const o = opts || {};
        const seq = inst.projectSeq;
        const payload = await P.api.getLayout(inst.projectId);
        if (!isCurrent(inst, seq)) return payload;
        if (o.ifIdle && !layoutIdle(inst)) return payload;
        if (o.newer && Number(payload.version) < inst.layoutVersion) return payload;
        inst.layoutVersion = Number(payload.version) || 0;
        inst.layoutSnapshot = P.util.deepFreeze(P.util.deepClone(payload.project));
        if (inst.layoutStore) inst.layoutStore.replace(payload.project, { resetHistory: !o.keepHistory });
        if (inst.layoutSaver) inst.layoutSaver.reset(inst.layoutVersion);
        emitIssues(inst, payload.issues);
        return payload;
    }

    async function resolveLayoutConflict(inst, err) {
        const t = inst.t;
        const d = P.dialog.open(inst.root, {
            title: t('cyd_creator.app.conflict_title'),
            body: P.util.el('p', { text: t('cyd_creator.app.conflict_body') }),
            actions: [
                { id: 'keep', label: t('cyd_creator.code.conflict_keep') },
                { id: 'reload', label: t('cyd_creator.code.conflict_reload') },
                { id: 'overwrite', label: t('cyd_creator.code.conflict_overwrite'), kind: 'danger' }
            ],
            owner: inst.projectBag || inst.bag
        });
        const choice = await d.promise;
        try {
            if (choice === 'reload') {
                const payload = await reloadLayout(inst);
                return { action: 'reload', token: Number(payload.version) || 0 };
            }
            if (choice === 'overwrite') {
                const version = err && err.details && Number(err.details.version);
                const token = version || Number((await P.api.getLayout(inst.projectId)).version) || 0;
                return { action: 'overwrite', token };
            }
        } catch (e) {
            inst.notify('error', P.errors.message(t, e));
        }
        return { action: 'keep' };
    }

    async function applyOrientation(inst, orientation) {
        const current = inst.layout().project || {};
        if (!orientation || current.orientation === orientation || inst.readonly) return;
        if (inst.layoutStore) {
            inst.layoutStore.change('orientation', draft => { draft.orientation = orientation; });
            await inst.layoutSaver.flush();
            return;
        }
        const next = P.util.deepClone(current);
        next.orientation = orientation;
        inst.layoutSaver.change(next);
        await inst.layoutSaver.flush();
    }

    // uploadAsset saves pending layout edits first (the upload bumps the layout version) and refuses
    // with the save error when they cannot be saved. The new layout replaces the store only while
    // nothing is unsaved: edits made during the upload stay and meet the new version through the
    // If-Match conflict choice, like a project_changed from elsewhere. An answer older than the
    // window's version (a save, e.g. an Overwrite, landed while it was on the way) is dropped. The
    // server announces the upload with project_changed (what: assets) before it answers: while the
    // upload runs, onProjectChanged leaves that reload (which would reset the undo history) to the
    // re-read here, which keeps the history. An assets change skipped meanwhile that the re-read does
    // not cover (another tab committed after the GET was served, or the upload failed) is caught up
    // when the last upload ends, like any remote change (history reset).
    async function uploadAsset(inst, file) {
        const seq = inst.projectSeq;
        const id = inst.projectId;
        if (!(await inst.layoutSaver.flush())) throw unsavedError(inst);
        inst.uploads.push(seq);
        try {
            const res = await P.api.uploadAsset(id, file);
            if (!isCurrent(inst, seq) || !layoutIdle(inst)) return res.asset;
            await reloadLayout(inst, { ifIdle: true, newer: true, keepHistory: true });
            return res.asset;
        } finally {
            inst.uploads.splice(inst.uploads.indexOf(seq), 1);
            if (!inst.uploads.includes(seq)) await catchUpSkippedAssets(inst, seq);
        }
    }

    async function catchUpSkippedAssets(inst, seq) {
        const skipped = inst.skippedAssets.get(seq) || 0;
        inst.skippedAssets.delete(seq);
        if (skipped <= inst.layoutVersion || !isCurrent(inst, seq) || !layoutIdle(inst)) return;
        try {
            await reloadLayout(inst, { ifIdle: true, newer: true });
        } catch (err) {
            if (isCurrent(inst, seq)) inst.notify('error', P.errors.message(inst.t, err));
        }
    }

    // ── open / leave ────────────────────────────────────────────────────────

    async function openProject(inst, id, opts) {
        const o = opts || {};
        const t = inst.t;
        teardownProject(inst);
        const seq = inst.projectSeq;
        if (inst.home) { inst.home.dispose(); inst.home = null; }
        inst.root.replaceChildren(P.util.el('div', { class: 'cc-loading', role: 'status', text: t('cyd_creator.app.loading') }));
        let summary;
        let layout;
        try {
            [summary, layout] = await Promise.all([P.api.getProject(id), P.api.getLayout(id)]);
        } catch (err) {
            if (!isCurrent(inst, seq)) return;
            inst.notify('error', P.errors.message(t, err));
            showHome(inst);
            return;
        }
        if (!isCurrent(inst, seq)) return;
        inst.projectId = id;
        inst.project = summary;
        inst.projectBag = P.util.bag();
        inst.view = buildView(inst);
        inst.root.replaceChildren(inst.view.root);
        inst.root.dataset.ccView = 'project';
        setupLayout(inst, layout);
        inst.dock = P.dock.create(inst);
        inst.root.appendChild(inst.dock.el);
        if (P.monitor && typeof P.monitor.attach === 'function') {
            // Plan 5: the Monitor dock tab and its serial session live as long as the project view.
            const monitor = P.monitor.attach(inst);
            inst.monitor = monitor;
            inst.projectBag.add(() => { monitor.dispose(); if (inst.monitor === monitor) inst.monitor = null; });
        }
        inst.code = P.code.create(inst);
        inst.codeMounted = false;
        inst.agent = P.agent.create(inst);
        inst.agent.mount(inst.view.agentHost);
        if (P.designer && typeof P.designer.create === 'function' && inst.layoutStore) {
            inst.designer = P.designer.create(inst);
            inst.designer.mountScreens(inst.view.screensHost);
            inst.designer.mountPalette(inst.view.paletteHost);
            inst.designer.mountCanvas(inst.view.canvasHost);
            inst.designer.mountInspector(inst.view.inspectorHost);
        } else {
            inst.view.canvasHost.appendChild(placeholder(inst, 'cyd_creator.app.designer_missing'));
            inst.view.inspectorHost.appendChild(placeholder(inst, 'cyd_creator.app.properties_missing'));
        }
        inst.root.classList.toggle('no-designer', !inst.designer);
        wireView(inst);
        wireProjectEvents(inst);
        emitIssues(inst, inst.layoutIssues);
        renderToolchain(inst, inst.view.toolchainSlot);
        setMode(inst, o.mode === 'code' ? 'code' : 'design');
        showPanel(inst, o.prompt ? 'agent' : 'properties');
        renderToolbar(inst);
        renderSaveState(inst);
        inst.dock.builds.load().then(() => renderToolbar(inst));
        inst.events = P.events.connect(inst);
        if (o.orientation) await applyOrientation(inst, o.orientation);
        // The job starts through inst.api.startJob, which saves the orientation (and anything else) first.
        if (o.prompt && isCurrent(inst, seq) && inst.agent) await inst.agent.startFromDescription(o.prompt);
    }

    function wireProjectEvents(inst) {
        const bus = inst.bus;
        const b = inst.projectBag;
        let codeFailureChecked = false;
        // resumed: the stream failed since its last stream_ready (a lost connection, a server restart).
        let resumed = false;
        b.add(bus.on('sse:project_changed', data => onProjectChanged(inst, data || {})));
        b.add(bus.on('sse:lock_changed', data => setLock(inst, (data && data.locked_by_job) || '')));
        b.add(bus.on('sse:build_status', data => {
            if (!data || !data.build) return;
            bus.emit('build:status', data.build);
            renderToolbar(inst);
        }));
        b.add(bus.on('sse:job_status', data => { if (data && data.job) bus.emit('job:status', data.job); }));
        b.add(bus.on('sse:device_changed', data => { if (data && data.device) bus.emit('device:changed', data.device); }));
        // The stream cannot continue the window's state (pruned events, a restored database): reload
        // it. The agent panel reloads itself, the dock re-reads a running build's log.
        // A stream back after a failure re-reads the summary: state the server keeps in memory ends
        // without an event (a restart clears the publication fence, and its stream_ready has no reload).
        b.add(bus.on('sse:stream_ready', data => {
            if (data && data.reload) reloadProject(inst, { keepEdits: true });
            else if (resumed) refreshProject(inst);
            resumed = false;
        }));
        b.add(bus.on('events:failed', failures => {
            resumed = true;
            if (failures >= STREAM_PROBE_AFTER) probeStream(inst);
        }));
        b.add(bus.on('events:state', state => { if (inst.view) inst.view.streamChip.hidden = state !== 'reconnecting'; }));
        b.add(bus.on('code:state', state => {
            inst.codeState = state || 'saved';
            renderSaveState(inst);
            // A refused code save may come from the publication fence: the summary tells (asked once
            // per run of failed saves, not on every retry).
            if (state === 'saved') codeFailureChecked = false;
            if (state === 'failed' && !codeFailureChecked && inst.project && !inst.project.publish_review) {
                codeFailureChecked = true;
                refreshProject(inst);
            }
        }));
        b.on(inst.root, 'keydown', event => {
            if ((event.ctrlKey || event.metaKey) && !event.shiftKey && String(event.key).toLowerCase() === 's') {
                event.preventDefault();
                flushAll(inst);
            }
        });
    }

    function setLock(inst, jobId) {
        if (!inst.project) return;
        const was = inst.isLocked();
        inst.project.locked_by_job = jobId;
        renderToolbar(inst);
        if (inst.code) inst.code.updateReadOnly();
        if (!jobId && inst.layoutSaver) inst.layoutSaver.unlock();
        if (was !== inst.isLocked()) inst.bus.emit('lock:changed', inst.isLocked(), jobId);
    }

    async function onProjectChanged(inst, data) {
        const seq = inst.projectSeq;
        const what = data.what || 'layout';
        try {
            if (what === 'layout' || what === 'assets') {
                // The window's own upload re-reads the layout itself (keeping the undo history); an
                // upload only ever announces "assets".
                const ownUpload = what === 'assets' && inst.uploads.includes(seq);
                if (ownUpload) inst.skippedAssets.set(seq, Math.max(inst.skippedAssets.get(seq) || 0, Number(data.version) || 0));
                if (!ownUpload && Number(data.version) > inst.layoutVersion && layoutIdle(inst)) await reloadLayout(inst, { ifIdle: true, newer: true });
            } else if (what === 'files') {
                if (inst.code) await inst.code.refreshFiles();
            } else if (what === 'meta') {
                await refreshProject(inst);
            }
        } catch (err) {
            if (isCurrent(inst, seq)) inst.notify('error', P.errors.message(inst.t, err));
        }
        if (isCurrent(inst, seq)) inst.bus.emit('project:changed', data);
    }

    async function refreshProject(inst) {
        if (!inst.projectId) return;
        const seq = inst.projectSeq;
        try {
            const summary = await P.api.getProject(inst.projectId);
            if (!isCurrent(inst, seq) || !inst.project || summary.id !== inst.projectId) return;
            const lockedBefore = inst.project.locked_by_job;
            inst.project = summary;
            renderToolbar(inst);
            if (lockedBefore !== summary.locked_by_job) {
                inst.project.locked_by_job = lockedBefore;
                setLock(inst, summary.locked_by_job);
            }
            inst.bus.emit('project:updated', summary);
        } catch (err) {
            if (isCurrent(inst, seq)) inst.notify('error', P.errors.message(inst.t, err));
        }
    }

    // reloadProject re-reads the summary, the layout, the file list and the builds: after a restore
    // and when the event stream cannot continue (stream_ready with reload). With keepEdits a layout
    // with unsaved edits stays; its next save ends in the If-Match conflict choice.
    async function reloadProject(inst, opts) {
        const seq = inst.projectSeq;
        const keepLayout = !!(opts && opts.keepEdits) && !layoutIdle(inst);
        try {
            await Promise.all([
                keepLayout ? null : reloadLayout(inst, { ifIdle: !!(opts && opts.keepEdits) }),
                inst.code ? inst.code.refreshFiles() : null,
                inst.dock ? inst.dock.builds.load() : null,
                refreshProject(inst)
            ]);
        } catch (err) {
            if (isCurrent(inst, seq)) inst.notify('error', P.errors.message(inst.t, err));
            return;
        }
        if (!isCurrent(inst, seq)) return;
        renderToolbar(inst);
        if (inst.code) await inst.code.reloadOpen().catch(() => {});
    }

    async function flushAll(inst) {
        const results = await Promise.all([
            inst.layoutSaver ? inst.layoutSaver.flush() : true,
            inst.code ? inst.code.flush() : true
        ]);
        return results.every(Boolean);
    }

    // unsavedError explains why unsaved edits blocked an agent job or an upload: the error of the
    // failed layout save, else that of the failed code save (P.errors.message turns either into its
    // sentence), else a generic one.
    function unsavedError(inst) {
        const saver = inst.layoutSaver;
        if (saver && saver.dirty() && saver.error()) return saver.error();
        const code = inst.code;
        const codeError = code && code.isDirty() && typeof code.saveError === 'function' ? code.saveError() : null;
        if (codeError) return codeError;
        return new P.api.ApiError(409, '', 'unsaved changes', {});
    }

    async function leaveProject(inst) {
        if (!inst.projectId) return true;
        if (await flushAll(inst)) return true;
        return inst.confirm({ titleKey: 'cyd_creator.app.unsaved_title', bodyKey: 'cyd_creator.app.unsaved_body', okKey: 'cyd_creator.app.leave_anyway', danger: true });
    }

    function teardownProject(inst) {
        inst.projectSeq++;
        if (inst.events) { inst.events.close(); inst.events = null; }
        ['designer', 'agent', 'code', 'dock'].forEach(name => {
            const part = inst[name];
            if (part && typeof part.dispose === 'function') {
                try { part.dispose(); } catch (err) { console.error('cyd-creator dispose failed', name, err); }
            }
            inst[name] = null;
        });
        if (inst.layoutSaver) inst.layoutSaver.dispose();
        if (inst.projectBag) inst.projectBag.dispose();
        inst.projectBag = null;
        inst.layoutSaver = null;
        inst.layoutStore = null;
        inst.layoutSnapshot = null;
        inst.layoutVersion = 0;
        inst.view = null;
        inst.project = null;
        inst.projectId = '';
        inst.codeState = 'saved';
        inst.streamAfter = 0;
        // A probe of the closed project that never answers must not hold off the next project's.
        inst.probing = false;
    }

    // ── event stream give-up ────────────────────────────────────────────────

    // probeStream runs after STREAM_PROBE_AFTER stream failures in a row: GET /projects/{id} tells
    // whether reconnecting can ever work. A deleted project returns to the start screen; an expired
    // session (401), a token without the desktop scope or a switched-off CYD Creator (503
    // CYD_DISABLED, maybe with details.problem) stop the stream with their message and a Retry
    // button. Network failures and other answers keep reconnecting.
    async function probeStream(inst) {
        if (inst.probing || !inst.events) return;
        const seq = inst.projectSeq;
        inst.probing = true;
        try {
            await P.api.getProject(inst.projectId);
        } catch (err) {
            if (!isCurrent(inst, seq) || !inst.events) return;
            const status = Number(err && err.status) || 0;
            const code = P.errors.code(err);
            if (status === 404) {
                inst.notify('error', inst.t('cyd_creator.error.CYD_NOT_FOUND'));
                showHome(inst);
            } else if (status === 401 || code === 'unauthorized' || code === 'desktop_scope_required' || code === 'CYD_DISABLED') {
                stopStream(inst, err);
            }
        } finally {
            if (isCurrent(inst, seq)) inst.probing = false;
        }
    }

    function stopStream(inst, err) {
        const v = inst.view;
        if (inst.events) {
            // Not .after: during a catch-up the cursor already counts buffered, undelivered events.
            inst.streamAfter = inst.events.resumeFrom;
            inst.events.close();
            inst.events = null;
        }
        if (!v) return;
        const { el } = P.util;
        v.streamChip.hidden = true;
        const retry = el('button', { type: 'button', class: 'cc-btn cc-btn--small', dataset: { ccAction: 'reconnect' }, text: inst.t('cyd_creator.app.retry'), onClick: () => reconnectStream(inst) });
        v.streamBanner.replaceChildren(...[el('span', { class: 'cc-banner-text', text: P.errors.message(inst.t, err) }), problemDetail(err), retry].filter(Boolean));
        v.streamBanner.hidden = false;
    }

    // reconnectStream resumes the stopped stream after the last event it applied.
    function reconnectStream(inst) {
        const v = inst.view;
        if (!v || inst.events) return;
        v.streamBanner.hidden = true;
        v.streamBanner.replaceChildren();
        inst.events = P.events.connect(inst, { after: inst.streamAfter || 0 });
    }

    // ── build, files, reveal ────────────────────────────────────────────────

    async function startBuild(inst) {
        const t = inst.t;
        if (buildBlocker(inst) || !inst.view) return;
        const seq = inst.projectSeq;
        inst.view.buildBtn.disabled = true;
        try {
            const saved = await flushAll(inst);
            if (!isCurrent(inst, seq)) return;
            if (!saved) { inst.notify('error', t('cyd_creator.app.unsaved_build')); return; }
            const build = await P.api.startBuild(inst.projectId);
            if (!isCurrent(inst, seq) || !inst.dock) return;
            inst.dock.trackBuild(build);
        } catch (err) {
            if (isCurrent(inst, seq)) inst.notify('error', P.errors.message(t, err));
        } finally {
            if (isCurrent(inst, seq)) renderToolbar(inst);
        }
    }

    async function openFile(inst, path, line) {
        if (!inst.code) return false;
        setMode(inst, 'code');
        return inst.code.openFile(path, line);
    }

    function reveal(inst, spec) {
        setMode(inst, 'design');
        // The designer of plan 4b listens for designer:reveal on the bus (no direct call, so it reveals once).
        inst.bus.emit('designer:reveal', spec || {});
    }

    // ── public API ──────────────────────────────────────────────────────────

    function render(host, windowId, ctx) {
        dispose(windowId);
        const inst = createInstance(host, windowId, ctx);
        inst.root = P.util.el('div', { class: 'cc-app', dataset: { ccRoot: '', ccWindow: String(windowId) } });
        host.replaceChildren(inst.root);
        instances.set(windowId, inst);
        if (inst.ctx && typeof inst.ctx.setWindowBeforeClose === 'function') {
            inst.ctx.setWindowBeforeClose(windowId, () => leaveProject(inst));
        }
        // The desktop policy can switch read-only on or off while the window is open.
        inst.bag.on(document, 'aurago:desktop-policy', event => {
            const detail = event && event.detail;
            if (!detail || typeof detail.readonly !== 'boolean') return;
            inst.ctxReadonly = detail.readonly;
            setReadonly(inst);
        });
        start(inst);
        return inst;
    }

    function dispose(windowId) {
        const inst = instances.get(windowId);
        if (!inst) return;
        inst.disposed = true;
        instances.delete(windowId);
        if (inst.pullTimer) clearInterval(inst.pullTimer);
        teardownProject(inst);
        if (inst.home) { inst.home.dispose(); inst.home = null; }
        inst.bag.dispose();
        inst.bus.clear();
        if (inst.ctx && typeof inst.ctx.setWindowBeforeClose === 'function') inst.ctx.setWindowBeforeClose(windowId, null);
        if (inst.root) inst.root.remove();
    }

    window.CYDCreatorApp = { render, dispose, instances };
})();
