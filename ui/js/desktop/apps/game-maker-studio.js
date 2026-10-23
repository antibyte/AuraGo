(function () {
    'use strict';

    const { showModal, closeModal, setModalBusy, modalError, confirmAction, mediaToggle } = window.GameMakerStudioModals;
    const instances = new Map();
    const eventTypes = [
        'project_created', 'project_updated', 'job_status', 'phase', 'text_delta',
        'skill_activation', 'file_changed', 'asset_changed', 'preview_reload', 'tool_call', 'model_progress',
        'diagnostic', 'revision', 'validation_reset', 'validation_result', 'visual_result', 'visual_observation', 'visual_progress',
        'plan_summary', 'stream_ready'
    ];
    const activeStatuses = new Set(['queued', 'planning', 'building', 'validating', 'polishing', 'cancelling']);
    const terminalStatuses = new Set(['ready', 'failed', 'cancelled']);
    const phases = ['planning', 'building', 'validating', 'polishing', 'ready'];

    function render(container, windowId, context) {
        dispose(windowId);
        const state = {
            container,
            windowId,
            context,
            api: window.GameMakerStudioAPI.create(context.api),
            capabilities: null,
            projects: [],
            project: null,
            messages: [],
            job: null,
            eventSource: null,
            lastEventID: 0,
            eventCatchup: false,
            backlogControl: null,
            gameplayStatus: '',
            drift: null,
            planSummary: null,
            latestJob: null,
            frame: null,
            channelID: '',
            diagnostics: [],
            selectedAssetPackIDs: [],
            selectedModelAssetIDs: [],
            selectedAssetSelections: [],
            selectedAssetDimensions: {},
            selectedPresentation: null,
            activeJob: null,
            jobStartedAt: 0,
            elapsedTimer: null,
            repairCount: 0,
            lastPhase: '',
            validationPreviewDismissed: false,
            busyPoll: null,
            previewLoadTimer: null,
            previewLoadClear: null,
            previewStale: false,
            reconnecting: false,
            disposed: false
        };
        instances.set(windowId, state);
        state.fail = error => fail(state, error);
        state.addDiagnostic = diagnostic => addDiagnostic(state, diagnostic);
        state.reloadProjectRecord = () => reloadProjectRecord(state);
        state.refreshPreview = options => refreshPreview(state, options);
        state.preparePreviewReplacement = () => preparePreviewReplacement(state);
        state.resolvePlayStateRevisionConflict = () => refreshLatestPreview(state, { force: true });
        container.innerHTML = shell(state);
        state.activity = window.GameMakerStudioActivity?.create(state);
        bindShell(state);
        window.addEventListener('message', state.previewListener = event => handlePreviewMessage(state, event));
        context.setWindowBeforeClose?.(windowId, async () => {
            if (!await confirmLeaveValidating(state)) return false;
            return await window.GameMakerStudioPreview.flush(state)
                || await confirmAction(state, context.t('game_maker.save_failed_title'), context.t('game_maker.leave_unsaved'));
        });
        initialize(state);
    }

    function shell(state) {
        const { esc, t } = state.context;
        const menuItem = (action, key, extra) =>
            `<button type="button" role="menuitem" ${extra || ''} data-gm-action="${action}" disabled>${esc(t(key))}</button>`;
        return `
            <div class="gm-studio" data-gm-window="${esc(state.windowId)}" data-gm-step="agent">
                <header class="gm-topbar">
                    <div class="gm-brand">
                        <img src="/img/desktop-icons/game-maker-studio.svg" alt="">
                        <div><strong>${esc(t('game_maker.title'))}</strong><span>${esc(t('game_maker.subtitle'))}</span></div>
                    </div>
                    <div class="gm-project-meta">
                        <span class="gm-status" data-gm-status>${esc(t('game_maker.loading'))}</span>
                        <span class="gm-engine" data-gm-engine></span>
                    </div>
                    <nav class="gm-actions" aria-label="${esc(t('game_maker.project_actions'))}">
                        <button type="button" class="gm-action-wide" data-gm-action="rename" disabled>${esc(t('game_maker.rename'))}</button>
                        <button type="button" class="gm-action-wide" data-gm-action="delete" disabled>${esc(t('game_maker.delete'))}</button>
                        <button type="button" data-gm-action="skills">${esc(t('game_maker.sources'))}</button>
                        <button type="button" data-gm-action="knowledge" disabled>${esc(t('game_maker.knowledge'))}</button>
                        <button type="button" data-gm-action="assets">${esc(t('game_maker.assets'))}</button>
                        <button type="button" class="gm-action-wide" data-gm-action="revisions" disabled>${esc(t('game_maker.revisions'))}</button>
                        <button type="button" class="gm-action-wide" data-gm-action="code" disabled>${esc(t('game_maker.open_code_studio'))}</button>
                        <button type="button" class="gm-action-wide" data-gm-action="export" disabled>${esc(t('game_maker.export_zip'))}</button>
                        <div class="gm-more" data-gm-more>
                            <button type="button" data-gm-more-toggle aria-haspopup="menu" aria-expanded="false"
                                aria-label="${esc(t('game_maker.more_actions'))}" title="${esc(t('game_maker.more_actions'))}">⋯</button>
                            <div class="gm-more-menu" data-gm-more-menu role="menu" hidden>
                                ${menuItem('rename', 'game_maker.rename')}
                                ${menuItem('revisions', 'game_maker.revisions')}
                                ${menuItem('code', 'game_maker.open_code_studio')}
                                ${menuItem('export', 'game_maker.export_zip')}
                                ${menuItem('delete', 'game_maker.delete', 'class="gm-menu-danger"')}
                            </div>
                        </div>
                    </nav>
                </header>
                <div class="gm-layout">
                    <aside class="gm-library">
                        <div class="gm-library-head">
                            <div><span>${esc(t('game_maker.library'))}</span><small data-gm-count>0</small></div>
                            <button type="button" class="gm-primary gm-new" data-gm-action="new">+ ${esc(t('game_maker.new_game'))}</button>
                        </div>
                        <div class="gm-project-list" data-gm-projects role="list"></div>
                    </aside>
                    <main class="gm-workspace">
                        <div class="gm-steps" role="group" data-gm-steps aria-label="${esc(t('game_maker.steps'))}">
                            <button type="button" data-gm-step="agent" aria-pressed="true">${esc(t('game_maker.step_agent'))}</button>
                            <button type="button" data-gm-step="preview" aria-pressed="false">${esc(t('game_maker.step_preview'))}</button>
                        </div>
                        <section class="gm-agent-pane">
                            <div class="gm-pane-head">
                                <div><span class="gm-kicker">${esc(t('game_maker.agent'))}</span><h2 data-gm-title>${esc(t('game_maker.choose_project'))}</h2></div>
                                <select class="gm-mobile-projects" data-gm-mobile-projects
                                    aria-label="${esc(t('game_maker.choose_project'))}" hidden></select>
                                <button type="button" class="gm-primary gm-narrow-new" data-gm-action="new">+ ${esc(t('game_maker.new_game'))}</button>
                            </div>
                            <div class="gm-job-banner" data-gm-job-banner hidden>
                                <span class="gm-job-spinner" aria-hidden="true"></span>
                                <div class="gm-job-banner-text">
                                    <strong>${esc(t('game_maker.job_banner_title'))}</strong>
                                    <span data-gm-job-phase></span>
                                    <span data-gm-validating-hint hidden></span>
                                </div>
                                <span class="gm-job-elapsed" data-gm-job-elapsed></span>
                                <button type="button" class="gm-danger" data-gm-action="stop">${esc(t('game_maker.stop'))}</button>
                            </div>
                            <ol class="gm-phases" data-gm-phases>${phaseMarkup(state, '')}</ol>
                            <div class="gm-capability-notice" data-gm-capability-notice role="status" hidden>
                                <strong data-gm-capability-title></strong>
                                <span data-gm-capability-message></span>
                            </div>
                            <div class="gm-conversation" data-gm-conversation role="log" aria-live="polite">
                                <div class="gm-empty"><strong>${esc(t('game_maker.empty_title'))}</strong><span>${esc(t('game_maker.empty_hint'))}</span></div>
                            </div>
                            <form class="gm-change-form" data-gm-change-form>
                                <label for="gm-change-${esc(state.windowId)}">${esc(t('game_maker.change_request'))}</label>
                                <div>
                                    <textarea id="gm-change-${esc(state.windowId)}" rows="2" maxlength="12000" disabled
                                        placeholder="${esc(t('game_maker.change_placeholder'))}"></textarea>
                                    <button type="submit" class="gm-primary" disabled>${esc(t('game_maker.apply_change'))}</button>
                                </div>
                                <p class="gm-busy-hint" data-gm-busy-hint hidden></p>
                                <p class="gm-drift-hint" data-gm-drift-hint hidden></p>
                                ${window.GameMakerStudioAssets ? window.GameMakerStudioAssets.selectionMarkup(state) : ''}
                            </form>
                        </section>
                        <section class="gm-preview-pane">
                            <div class="gm-pane-head">
                                <div><span class="gm-kicker">${esc(t('game_maker.live_preview'))}</span><h2>${esc(t('game_maker.play_here'))}</h2></div>
                                <div class="gm-preview-tools">
                                    <button type="button" data-gm-action="scene_debug" aria-pressed="false" aria-label="${esc(t('game_maker.scene_debug'))}" title="${esc(t('game_maker.scene_debug'))}"><svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5"><path d="M3 3h18v18H3zM3 9h18M3 15h18M9 3v18M15 3v18"/></svg></button>
                                    <button type="button" data-gm-action="visual_review" disabled>${esc(t('game_maker.visual_review'))}</button>
                                    <button type="button" data-gm-action="reload" disabled>${esc(t('game_maker.reload'))}</button>
                                    <button type="button" data-gm-action="fullscreen" disabled
                                        aria-label="${esc(t('game_maker.fullscreen'))}" title="${esc(t('game_maker.fullscreen'))}">⛶</button>
                                    <button type="button" data-gm-action="open_tab" disabled
                                        aria-label="${esc(t('game_maker.open_tab'))}" title="${esc(t('game_maker.open_tab'))}">↗</button>
                                </div>
                            </div>
                            <div class="gm-preview-shell" data-gm-preview>
                                <div class="gm-drift" data-gm-drift hidden>
                                    <span data-gm-drift-text></span>
                                    <button type="button" data-gm-action="adopt" hidden>${esc(t('game_maker.adopt_changes'))}</button>
                                </div>
                                <div class="gm-preview-empty">
                                    <img src="/img/desktop-icons/game-maker-studio.svg" alt="">
                                    <strong>${esc(t('game_maker.preview_waiting'))}</strong>
                                    <span>${esc(t('game_maker.preview_waiting_hint'))}</span>
                                </div>
                            </div>
                            <div class="gm-visual-strip" data-gm-visual hidden aria-live="polite"><span data-gm-visual-status></span></div>
                            <details class="gm-diagnostics" data-gm-diagnostics>
                                <summary>${esc(t('game_maker.diagnostics'))} <span data-gm-diagnostic-count>0</span></summary>
                                <ul data-gm-diagnostic-list></ul>
                            </details>
                        </section>
                    </main>
                </div>
                <div class="gm-modal-layer" data-gm-modal hidden></div>
            </div>`;
    }

    async function initialize(state) {
        try {
            const [capabilities, projects] = await Promise.all([
                state.api.capabilities(),
                state.api.listProjects()
            ]);
            if (state.disposed) return;
            state.capabilities = capabilities;
            state.activeJob = capabilities.active_job || null;
            state.projects = projects.projects || [];
            applyCapabilities(state);
            renderProjects(state);
            syncBusyPoll(state);
            if (state.projects.length) openProject(state, state.projects[0].id);
            else updateStatus(state, capabilityState(capabilities).status);
        } catch (error) {
            fail(state, error);
        }
    }

    function bindShell(state) {
        const root = state.container.querySelector('.gm-studio');
        root.addEventListener('click', state.clickListener = event => {
            if (event.target.closest('[data-gm-more-toggle]')) {
                toggleMoreMenu(state);
                return;
            }
            if (!event.target.closest('[data-gm-more]')) closeMoreMenu(state);
            const play = event.target.closest('[data-gm-play]');
            if (play) {
                playNow(state);
                return;
            }
            const retry = event.target.closest('[data-gm-retry]');
            if (retry) {
                retryLastPrompt(state);
                return;
            }
            const retryModel = event.target.closest('[data-gm-retry-model]');
            if (retryModel) {
                showRetryModelModal(state);
                return;
            }
            const extendButton = event.target.closest('[data-gm-extend-repairs]');
            if (extendButton) {
                extendRepairsAction(state, extendButton.dataset.gmExtendRepairs);
                return;
            }
            const stepButton = event.target.closest('[data-gm-steps] [data-gm-step]');
            if (stepButton) {
                setStudioStep(state, stepButton.dataset.gmStep);
                return;
            }
            const button = event.target.closest('[data-gm-action]');
            if (!button) return;
            closeMoreMenu(state);
            const modals = window.GameMakerStudioModals;
            const preview = window.GameMakerStudioPreview;
            const actions = {
                new: () => showCreateModal(state),
                knowledge: () => {
                    if (!state.project || !window.AuraGoLearnings) return;
                    if (state.knowledgeClose) state.knowledgeClose();
                    state.knowledgeClose = window.AuraGoLearnings.open({ project: 'game-maker:' + state.project.id, parent: state.container, title: state.context.t('game_maker.knowledge') });
                },
                assets: () => window.GameMakerStudioAssets.show(state, modalHelpers),
                skills: () => modals ? modals.showSkillsModal(state, modalHelpers) : fail(state, new Error(state.context.t('game_maker.modules_load_failed'))),
                revisions: () => modals ? modals.showRevisionsModal(state, modalHelpers) : fail(state, new Error(state.context.t('game_maker.modules_load_failed'))),
                code: () => openInCodeStudio(state),
                export: () => exportProject(state),
                stop: () => stopJob(state),
                visual_review: () => preview && preview.requestCapture(state),
                reload: () => refreshLatestPreview(state, { force: true }),
                adopt: () => adoptWorkspace(state),
                scene_debug: () => preview && preview.setSceneDebug(state, !state.sceneDebug),
                fullscreen: () => preview && preview.toggleFullscreen(state),
                open_tab: () => preview && preview.openTab(state),
                rename: () => renameProject(state),
                delete: () => deleteProject(state)
            };
            const action = actions[button.dataset.gmAction];
            if (action) action();
        });
        root.querySelector('[data-gm-projects]').addEventListener('click', event => {
            const item = event.target.closest('[data-project-id]');
            if (item) openProject(state, item.dataset.projectId);
        });
        root.querySelector('[data-gm-mobile-projects]').addEventListener('change', event => {
            openProject(state, event.target.value);
        });
        const form = root.querySelector('[data-gm-change-form]');
        const textarea = form.querySelector('textarea');
        form.addEventListener('submit', event => {
            event.preventDefault();
            submitChange(state);
        });
        textarea.addEventListener('keydown', event => {
            if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
                event.preventDefault();
                form.requestSubmit();
            }
        });
        textarea.addEventListener('input', () => autoGrow(textarea));
        if (state.moreDocListener) document.removeEventListener('click', state.moreDocListener);
        if (state.moreEscListener) document.removeEventListener('keydown', state.moreEscListener);
        document.addEventListener('click', state.moreDocListener = event => {
            if (!event.target.closest || !event.target.closest('[data-gm-more]')) closeMoreMenu(state);
        });
        document.addEventListener('keydown', state.moreEscListener = event => {
            if (event.key === 'Escape') closeMoreMenu(state);
        });
    }

    function toggleMoreMenu(state) {
        const menu = state.container.querySelector('[data-gm-more-menu]');
        const toggle = state.container.querySelector('[data-gm-more-toggle]');
        const open = menu.hidden;
        menu.hidden = !open;
        toggle.setAttribute('aria-expanded', String(open));
    }

    function closeMoreMenu(state) {
        const menu = state.container.querySelector('[data-gm-more-menu]');
        if (!menu || menu.hidden) return;
        menu.hidden = true;
        state.container.querySelector('[data-gm-more-toggle]').setAttribute('aria-expanded', 'false');
    }

    function applyCapabilities(state) {
        const cap = state.capabilities || {};
        const availability = capabilityState(cap);
        const newButtons = state.container.querySelectorAll('[data-gm-action="new"]');
        const notice = state.container.querySelector('[data-gm-capability-notice]');
        let message = availability.messageKey ? state.context.t(availability.messageKey) : '';
        if (availability.status === 'skills-blocked') {
            const blocking = (cap.skills || [])
                .filter(skill => !['ready', 'verified', 'installed', 'updated'].includes(skill.status))
                .map(skill => `${skill.name}: ${skill.status}`);
            if (blocking.length) {
                message += ' ' + state.context.t('game_maker.skills_blocked_details', { details: blocking.join(', ') });
            }
        }
        newButtons.forEach(button => {
            button.disabled = availability.status !== 'ready';
            button.title = message;
        });
        notice.hidden = availability.status === 'ready';
        notice.querySelector('[data-gm-capability-title]').textContent =
            availability.status === 'ready' ? '' : state.context.t('game_maker.creation_unavailable');
        notice.querySelector('[data-gm-capability-message]').textContent = message;
        if (message) showNotice(state, message, 'warning');
    }

    function capabilityState(capabilities) {
        const cap = capabilities || {};
        if (!cap.enabled) {
            return { status: 'disabled', messageKey: 'game_maker.disabled_notice' };
        }
        if (cap.readonly) {
            return { status: 'readonly', messageKey: 'game_maker.readonly_notice' };
        }
        if (!cap.allow_create) {
            return { status: 'create-disabled', messageKey: 'game_maker.create_disabled_notice' };
        }
        if (!cap.skills_ready) {
            return { status: 'skills-blocked', messageKey: 'game_maker.skills_blocked_notice' };
        }
        return { status: 'ready', messageKey: '' };
    }

    function renderProjects(state) {
        const { esc, t } = state.context;
        const list = state.container.querySelector('[data-gm-projects]');
        state.container.querySelector('[data-gm-count]').textContent = String(state.projects.length);
        renderMobileSelect(state);
        if (!state.projects.length) {
            list.innerHTML = `<div class="gm-library-empty">${esc(t('game_maker.no_projects'))}</div>`;
            return;
        }
        const runningID = state.activeJob && state.activeJob.project_id;
        list.innerHTML = state.projects.map(project => {
            const updated = relativeTime(state, project.updated_at);
            const running = runningID === project.id;
            return `
            <button type="button" class="gm-project-card ${state.project && state.project.id === project.id ? 'is-active' : ''} ${running ? 'is-running' : ''}"
                data-project-id="${esc(project.id)}" role="listitem">
                <span class="gm-project-dimension">${esc(project.variant === 'voxel' ? t('game_maker.voxel') : String(project.dimension).toUpperCase())}</span>
                <span><strong title="${esc(project.name)}">${esc(project.name)}</strong><small>${esc(statusLabel(state, project.status))}${updated ? ' · ' + esc(updated) : ''}</small></span>
                <span class="gm-project-revision">v${Number(project.current_revision || 0)}</span>
                ${running ? `<span class="gm-project-running" title="${esc(t('game_maker.status_working'))}"></span>` : ''}
            </button>`;
        }).join('');
    }

    function statusLabel(state, status) {
        const raw = String(status || 'draft');
        const key = 'game_maker.status_' + raw.replaceAll('-', '_');
        const label = state.context.t(key);
        return label && label !== key ? label : raw;
    }

    function renderMobileSelect(state) {
        const { esc } = state.context;
        const select = state.container.querySelector('[data-gm-mobile-projects]');
        if (!select) return;
        select.hidden = !state.projects.length;
        select.innerHTML = state.projects.map(project =>
            `<option value="${esc(project.id)}" ${state.project && state.project.id === project.id ? 'selected' : ''}>${esc(project.name)}</option>`
        ).join('');
    }

    function relativeTime(state, iso) {
        const stamp = Date.parse(iso || '');
        if (!stamp) return '';
        const minutes = Math.floor(Math.max(0, Date.now() - stamp) / 60000);
        const t = state.context.t;
        if (minutes < 1) return t('game_maker.just_now');
        if (minutes < 60) return t('game_maker.minutes_ago', { n: minutes });
        const hours = Math.floor(minutes / 60);
        if (hours < 24) return t('game_maker.hours_ago', { n: hours });
        return t('game_maker.days_ago', { n: Math.floor(hours / 24) });
    }

    async function openProject(state, projectID) {
        if (!projectID || state.disposed) return;
        const requestID = state.projectRequestID = (state.projectRequestID || 0) + 1;
        const switching = state.project?.id && state.project.id !== projectID;
        if (switching && !await confirmLeaveValidating(state)) return;
        if (state.disposed || state.projectRequestID !== requestID) return;
        if (state.project?.id !== projectID && await window.GameMakerStudioPreview?.flush(state) === false) return;
        if (state.disposed || state.projectRequestID !== requestID) return;
        if (state.project?.id !== projectID) state.lastRevisionRefreshAttempt = null;
        closeEvents(state);
        stopElapsed(state);
        state.repairCount = 0;
        state.lastPhase = '';
        state.validationPreviewDismissed = false;
        state.gameplayStatus = '';
        state.previewStale = false;
        if (switching) {
            state.planSummary = null;
            state.latestJob = null;
            state.drift = null;
        }
        try {
            const body = await state.api.getProject(projectID);
            if (state.disposed || state.projectRequestID !== requestID) return;
            if (state.project?.id !== projectID) {
                window.GameMakerStudioPreview?.clearLoading(state);
                window.GameMakerStudioPreview?.cancelVisual(state);
                state.previewVisibility?.disconnect();
                state.previewRequestID = (state.previewRequestID || 0) + 1;
                state.frame = null; state.previewGrant = null; state.channelID = '';
                state.activity?.reset();
            }
            if (state.knowledgeClose && state.project?.id !== body.project.id) state.knowledgeClose();
            state.project = body.project;
            state.messages = body.messages || [];
            if (body.plan_summary) state.planSummary = body.plan_summary;
            if (body.latest_job) state.latestJob = body.latest_job;
            // A new-game dialog owns the live selection until it closes.
            if (!state.selectionHold && window.GameMakerStudioAssets?.applyProjectSelection) {
                window.GameMakerStudioAssets.applyProjectSelection(state);
            }
            state.job = null;
            clearDiagnostics(state);
            state.container.querySelector('[data-gm-phases]').innerHTML = phaseMarkup(state, '');
            renderProjects(state);
            if (state.activeJob && state.activeJob.project_id === state.project.id) {
                state.job = {
                    id: state.activeJob.job_id,
                    status: state.activeJob.status,
                    phase: state.activeJob.phase
                };
                state.jobStartedAt = Date.now();
                state.container.querySelector('[data-gm-phases]').innerHTML = phaseMarkup(state, state.activeJob.phase);
                if ((state.activeJob.phase || state.activeJob.status) === 'validating') setStudioStep(state, 'preview');
            } else if (state.latestJob && terminalStatuses.has(state.latestJob.status)) {
                state.job = state.latestJob;
            }
            renderProject(state);
            connectEvents(state);
            refreshWorkspaceDrift(state);
            if (state.project.current_revision) refreshPreview(state);
        } catch (error) {
            if (!state.disposed && state.projectRequestID === requestID) fail(state, error);
        }
    }

    function renderProject(state) {
        renderProjectMeta(state);
        renderConversation(state);
        scrollConversation(state);
    }

    function renderProjectMeta(state) {
        const project = state.project;
        state.container.querySelector('[data-gm-title]').textContent = project.name;
        state.container.querySelector('[data-gm-engine]').textContent = project.variant === 'voxel'
            ? `${state.context.t('game_maker.voxel')} ${state.capabilities.voxel_version || 1} · Three.js ${state.capabilities.three_version}` : project.dimension === '3d'
            ? `Three.js ${state.capabilities.three_version}` : `Phaser ${state.capabilities.phaser_version}`;
        setButton(state, 'revisions', true);
        setButton(state, 'knowledge', true);
        setButton(state, 'rename', Boolean(state.capabilities.allow_edit));
        setButton(state, 'delete', Boolean(state.capabilities.allow_delete));
        setButton(state, 'export', Boolean(project.current_revision));
        setButton(state, 'visual_review', Boolean(project.current_revision) && !state.jobActive && !!state.capabilities.allow_edit);
        setButton(state, 'reload', Boolean(project.current_revision));
        setButton(state, 'fullscreen', Boolean(project.current_revision));
        setButton(state, 'open_tab', Boolean(project.current_revision));
        setButton(state, 'code', Boolean(state.capabilities.code_studio && project.current_revision));
        syncJobControls(state);
        updateStatus(state, state.job?.status || project.status || 'draft');
    }

    function renderConversation(state) {
        const { esc, t } = state.context;
        state.container.querySelector('[data-gm-conversation]').innerHTML = state.messages.length
            ? state.messages.map(message => messageMarkup(state, message)).join('')
            : `<div class="gm-empty"><strong>${esc(t('game_maker.agent_ready'))}</strong><span>${esc(state.project.description)}</span></div>`;
        renderPlanCard(state);
        if (!state.jobActive) renderOutcomeCard(state);
    }

    function messageMarkup(state, message) {
        const { esc, t } = state.context;
        const role = message.role === 'assistant' ? 'assistant' : 'user';
        return `<article class="gm-message gm-message-${role}">
            <span>${esc(role === 'assistant' ? t('game_maker.agent') : t('game_maker.you'))}${esc(formatTime(message.created_at))}</span>
            <p>${esc(message.content)}</p>
        </article>`;
    }

    function formatTime(iso) {
        const stamp = Date.parse(iso || '');
        if (!stamp) return '';
        try {
            return ' · ' + new Date(stamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        } catch (_) {
            return '';
        }
    }

    function phaseMarkup(state, current) {
        const { esc, t } = state.context;
        const currentIndex = phases.indexOf(current);
        return phases.map((phase, index) => {
            const mode = phase === current ? 'is-current' : (currentIndex >= 0 && index < currentIndex ? 'is-done' : '');
            return `<li class="${mode}" data-phase="${phase}"><i>${index + 1}</i><span>${esc(t('game_maker.phase_' + phase))}</span></li>`;
        }).join('');
    }

    function connectEvents(state) {
        if (!state.project || typeof EventSource !== 'function') return;
        state.eventCatchup = true;
        state.backlogControl = null;
        const source = new EventSource(state.api.eventURL(state.project.id, state.lastEventID));
        state.eventSource = source;
        source.onopen = () => {
            if (state.disposed || source !== state.eventSource) return;
            state.reconnecting = false;
            updateStatus(state, state.job?.status || state.project?.status || 'draft');
        };
        eventTypes.forEach(type => source.addEventListener(type, event => {
            if (state.disposed || source !== state.eventSource) return;
            const body = parseEvent(event) || {};
            if (!body.type) body.type = type;
            const id = Number(body.id || event.lastEventId || 0);
            if (id) state.lastEventID = Math.max(state.lastEventID, id);
            if (body.type === 'stream_ready') {
                finishEventBacklog(state);
                return;
            }
            if (!eventMatchesJob(state, body)) return;
            if (state.eventCatchup) {
                if (body.type === 'validation_result') noteGameplay(state, body);
                if (body.type === 'plan_summary') state.planSummary = body.payload || null;
                if (body.type === 'job_status') {
                    // A later status often omits the job object. Keep the earlier
                    // one so the reloaded project record does not hide it.
                    if (body.payload?.job) state.job = body.payload.job;
                    if (state.job && body.payload?.status) state.job.status = body.payload.status;
                    rememberJobOutcome(state, body.payload);
                    state.backlogControl = body;
                } else if (body.type === 'phase') {
                    state.backlogControl = body;
                }
                return;
            }
            handleEvent(state, body);
        }));
        source.onerror = () => {
            if (state.disposed || source !== state.eventSource) return;
            state.reconnecting = true;
            updateStatus(state, terminalStatuses.has(state.job?.status) ? state.job.status : 'reconnecting');
        };
    }

    function eventMatchesJob(state, event) {
        const incoming = event.payload?.job;
        const id = event.job_id || incoming?.id;
        if (!id || !state.job?.id || id === state.job.id) return true;
        // Only a new job's initial event can replace the displayed job. Late
        // terminal/preview events must also stay isolated during history replay.
        if (event.type !== 'job_status' || event.payload?.status !== 'queued' || incoming?.id !== id) return false;
        const currentStart = Date.parse(state.job.created_at);
        return Number.isFinite(currentStart)
            ? Date.parse(incoming.created_at) >= currentStart
            : !state.eventCatchup;
    }

    function noteGameplay(state, event) {
        const status = event.payload?.result?.gameplay_status;
        if (status) state.gameplayStatus = status;
    }

    function finishEventBacklog(state) {
        if (!state.eventCatchup) return;
        state.eventCatchup = false;
        const control = state.backlogControl;
        state.backlogControl = null;
        if (control) handleEvent(state, control);
        renderPlanCard(state);
    }

    function parseEvent(event) {
        try { return JSON.parse(event.data); } catch (_) { return null; }
    }

    function handleEvent(state, event) {
        if (!eventMatchesJob(state, event)) return;
        if (state.reconnecting) {
            state.reconnecting = false;
            updateStatus(state, (state.job && state.job.status) || (state.project && state.project.status) || 'ready');
        }
        const payload = event.payload || {};
        switch (event.type) {
        case 'job_status':
            handleJobStatus(state, payload);
            break;
        case 'plan_summary':
            state.planSummary = payload;
            renderPlanCard(state);
            break;
        case 'phase': {
            if (state.lastPhase === 'validating' && payload.phase === 'building') state.repairCount += 1;
            if (payload.phase !== 'validating') state.validationPreviewDismissed = false;
            const showValidationPreview = payload.phase === 'validating' && state.lastPhase !== 'validating' && !state.validationPreviewDismissed;
            state.lastPhase = payload.phase;
            if (!state.job) state.job = { id: event.job_id };
            state.job.phase = payload.phase;
            state.job.status = payload.phase;
            if (!state.jobStartedAt) state.jobStartedAt = Date.now();
            state.container.querySelector('[data-gm-phases]').innerHTML = phaseMarkup(state, payload.phase);
            syncJobControls(state);
            if (showValidationPreview) setStudioStep(state, 'preview');
            break;
        }
        case 'text_delta':
            appendAgentDelta(state, payload.content || '');
            break;
        case 'skill_activation':
            appendActivity(state, state.context.t('game_maker.skill_loaded') + ': ' + (payload.tool_id || ''));
            break;
        case 'file_changed':
        case 'asset_changed':
            appendActivity(state, state.context.t('game_maker.updated') + ': ' + (payload.path || payload.kind || ''));
            break;
        case 'diagnostic':
            addDiagnostic(state, payload);
            break;
        case 'validation_reset':
            window.GameMakerStudioPreview?.cancelVisual(state);
            clearDiagnostics(state);
            state.previewGrant = null;
            state.channelID = '';
            state.previewRequestID = (state.previewRequestID || 0) + 1;
            break;
        case 'validation_result': {
            const result = payload.result || {};
            noteGameplay(state, event);
            appendActivity(state, state.context.t('game_maker.gameplay_checks') + ': ' + state.context.t('game_maker.check_' + (result.gameplay_status || 'unavailable')));
            appendActivity(state, state.context.t('game_maker.rules_checks') + ': ' + state.context.t('game_maker.check_' + (result.rules_status || 'unverified')));
            for (const check of result.checks || []) {
                if (check.status !== 'passed') addDiagnostic(state, { level: check.status === 'failed' ? 'error' : 'info', message: check.id + ': ' + check.expected + ' — ' + check.observed });
            }
            break;
        }
        case 'visual_progress':
            window.GameMakerStudioPreview?.visualStatus(state,payload.status);
            break;
        case 'visual_result':
            window.GameMakerStudioPreview?.showReview(state,payload);
            appendActivity(state, state.context.t('game_maker.visual_checks') + ': ' + state.context.t('game_maker.check_' + (payload.status || 'skipped')));
            break;
        case 'visual_observation':
            appendActivity(state, state.context.t('game_maker.visual_advisory') + ': ' + String(payload.message || '').slice(0, 2000));
            break;
        case 'preview_reload':
            if (state.revisionMutationInProgress === state.project?.id) break;
            if (state.project?.variant === 'voxel' && payload.revision != null) {
                refreshLatestPreview(state, { revision: payload.revision });
            } else {
                refreshPreview(state);
            }
            break;
        case 'revision':
            appendActivity(state, state.context.t('game_maker.revision_published'));
            if (state.revisionMutationInProgress === state.project?.id) break;
            if (state.project?.variant === 'voxel') {
                refreshLatestPreview(state, { revision: payload.revision });
            } else {
                reloadProjectRecord(state).then(() => {
                    if (!state.disposed) refreshPreview(state);
                });
            }
            break;
        }
        state.activity?.event(event);
    }

    function handleJobStatus(state, payload) {
        if (payload.job) state.job = payload.job;
        if (state.job && payload.status) state.job.status = payload.status;
        rememberJobOutcome(state, payload);
        if (state.job && !terminalStatuses.has(payload.status)) {
            state.activeJob = { job_id: state.job.id, project_id: state.project.id, status: state.job.status, phase: state.job.phase };
        } else if (state.activeJob?.job_id === state.job?.id) {
            state.activeJob = null;
        }
        if (!state.jobStartedAt) {
            state.jobStartedAt = state.job && state.job.started_at ? Date.parse(state.job.started_at) || Date.now() : Date.now();
        }
        updateStatus(state, payload.status || 'working');
        syncJobControls(state);
        if (!terminalStatuses.has(payload.status)) return;
        finalizeStreaming(state);
        stopElapsed(state);
        const status = payload.status;
        const job = state.job;
        state.latestJob = Object.assign({}, job || {});
        state.repairCount = 0;
        state.lastPhase = '';
        state.validationPreviewDismissed = false;
        state.container.querySelector('[data-gm-phases]').innerHTML = phaseMarkup(state, status);
        reloadProjectRecord(state).then(() => {
            if (state.disposed || state.job !== job) return;
            if (status === 'ready') {
                const gameplay = payload.gameplay_status || state.gameplayStatus || job?.gameplay_status;
                if (state.project?.dimension === '3d' && state.project?.variant !== 'voxel' && gameplay === 'unverified') {
                    appendActivity(state, state.context.t('game_maker.gameplay_unverified'));
                }
            }
            renderProjects(state);
            refreshWorkspaceDrift(state);
        });
    }

    function finalizeStreaming(state) {
        state.container.querySelectorAll('.gm-message-assistant[data-streaming="true"]').forEach(message => {
            message.dataset.streaming = 'false';
        });
    }

    const outcomeFields = ['error', 'error_code', 'checks_passed', 'checks_failed', 'repair_passes', 'gameplay_status', 'usage'];

    function rememberJobOutcome(state, payload) {
        if (!payload) return;
        if (!state.job) state.job = {};
        outcomeFields.forEach(key => {
            if (payload[key] === undefined || payload[key] === null) return;
            if (key === 'error_code' && !payload[key]) return;
            state.job[key] = payload[key];
        });
        if (terminalStatuses.has(payload.status || state.job.status)) {
            state.latestJob = Object.assign({}, state.job);
        }
    }

    function renderPlanCard(state) {
        const log = state.container.querySelector('[data-gm-conversation]');
        if (!log) return;
        const existing = log.querySelector('[data-gm-plan]');
        const html = planCardMarkup(state);
        if (!html) {
            existing?.remove();
            return;
        }
        if (existing) {
            existing.outerHTML = html;
            return;
        }
        const outcome = log.querySelector('[data-gm-outcome]');
        if (outcome) outcome.insertAdjacentHTML('beforebegin', html);
        else log.insertAdjacentHTML('beforeend', html);
    }

    function planCardMarkup(state) {
        const plan = state.planSummary;
        if (!plan || typeof plan !== 'object') return '';
        const { esc, t } = state.context;
        const controls = Array.isArray(plan.controls) ? plan.controls.filter(Boolean) : [];
        const assets = Array.isArray(plan.assets) ? plan.assets : [];
        const rows = [];
        const template = [plan.template, plan.perspective].filter(Boolean).join(' · ');
        if (template) rows.push(`<div><dt>${esc(t('game_maker.plan_template'))}</dt><dd>${esc(template)}</dd></div>`);
        if (plan.objective) rows.push(`<div><dt>${esc(t('game_maker.plan_objective'))}</dt><dd>${esc(plan.objective)}</dd></div>`);
        if (controls.length) rows.push(`<div><dt>${esc(t('game_maker.plan_controls'))}</dt><dd>${esc(controls.join(', '))}</dd></div>`);
        if (assets.length) {
            const listed = assets.map(asset => [asset.role, asset.id].filter(Boolean).join(' · ')).filter(Boolean);
            if (listed.length) rows.push(`<div><dt>${esc(t('game_maker.plan_assets'))}</dt><dd>${esc(listed.join(', '))}</dd></div>`);
        }
        return `<article class="gm-result-card is-plan" data-gm-plan><strong>${esc(t('game_maker.plan_summary_title'))}</strong><dl>${rows.join('')}</dl></article>`;
    }

    function renderOutcomeCard(state) {
        const log = state.container.querySelector('[data-gm-conversation]');
        if (!log) return;
        log.querySelector('[data-gm-outcome]')?.remove();
        if (state.jobActive) return;
        const job = state.latestJob;
        if (!job || !terminalStatuses.has(job.status)) return;
        const kind = job.status === 'ready' ? 'success' : (job.status === 'failed' ? 'error' : 'cancelled');
        const message = kind === 'success'
            ? state.context.t('game_maker.revision_ready', { n: job.result_revision || state.project?.current_revision || '' })
            : (job.error || state.context.t(kind === 'cancelled' ? 'game_maker.status_cancelled' : 'game_maker.job_failed_title'));
        log.appendChild(outcomeCardElement(state, kind, message, job));
        scrollConversation(state);
    }

    function outcomeCardElement(state, kind, message, job) {
        const { esc, t } = state.context;
        const card = document.createElement('div');
        card.className = `gm-result-card is-${kind}`;
        card.setAttribute('data-gm-outcome', '');
        const counts = checksMarkup(state, job) + repairMarkup(state, job) + usageMarkup(state, job);
        if (kind === 'success') {
            card.innerHTML = `<strong>${esc(message)}</strong>${counts}
                <button type="button" class="gm-primary" data-gm-play>${esc(t('game_maker.play_now'))}</button>`;
            return card;
        }
        // Prefer the server code. English substring matching stays only for
        // older jobs that have no error_code.
        const code = job && job.error_code;
        const cause = code || failureKind(kind, message);
        const hint = cause ? `<p class="gm-result-hint">${esc(t('game_maker.failure_hint_' + cause))}</p>` : '';
        const title = t(kind === 'cancelled' ? 'game_maker.status_cancelled' : 'game_maker.job_failed_title');
        const technical = String(message || '').trim();
        const open = openChecksMarkup(state, job);
        const used = Number(job && job.repair_passes);
        // Only a failed job that actually spent repair passes can extend them,
        // and the shared ceiling of six passes applies here too.
        const extend = kind === 'error' && Number.isFinite(used) && used > 0 && used < 6
            ? `<button type="button" data-gm-extend-repairs="${Math.min(6, used + 3)}">${esc(t('game_maker.extend_repairs', { extra: 3 }))}</button>`
            : '';
        const retryModel = kind === 'error'
            ? `<button type="button" data-gm-retry-model>${esc(t('game_maker.retry_model'))}</button>`
            : '';
        card.setAttribute('role', 'alert');
        card.innerHTML = `<strong>${esc(title)}</strong>${hint}${open}${counts}
            ${technical && technical !== title ? `<details><summary>${esc(t('game_maker.failure_details'))}</summary><p>${esc(technical)}</p></details>` : ''}
            <button type="button" data-gm-retry>${esc(t('game_maker.retry'))}</button>${retryModel}${extend}`;
        return card;
    }

    // The concrete open checks and the model-facing next step belong above the
    // technical details: they are what the operator can act on.
    function openChecksMarkup(state, job) {
        const raw = job && Array.isArray(job.open_checks) ? job.open_checks : [];
        const checks = raw.filter(check => check && String(check.id || '').trim()).slice(0, 6);
        if (!checks.length) return '';
        const { esc, t } = state.context;
        const items = checks.map(check => {
            const status = t('game_maker.check_' + (check.status === 'unavailable' ? 'unavailable' : 'failed'));
            const detail = [check.expected, check.observed].filter(Boolean).join(' → ');
            return `<li data-gm-open-check="${esc(check.id)}"><strong>${esc(check.id)}</strong>
                <span>${esc(status)}</span>${detail ? `<small>${esc(detail)}</small>` : ''}</li>`;
        }).join('');
        const next = checks.map(check => String(check.next_action || '').trim()).find(Boolean);
        return `<div class="gm-open-checks" data-gm-open-checks><strong>${esc(t('game_maker.open_checks_title'))}</strong>
            <ul>${items}</ul>
            ${next ? `<p class="gm-result-hint" data-gm-open-check-next>${esc(t('game_maker.open_check_next'))}: ${esc(next)}</p>` : ''}</div>`;
    }

    function checksMarkup(state, job) {
        if (!job || job.checks_passed == null || job.checks_failed == null) return '';
        return `<p>${state.context.esc(state.context.t('game_maker.checks_summary', {
            passed: job.checks_passed, failed: job.checks_failed
        }))}</p>`;
    }

    function repairMarkup(state, job) {
        if (!job || job.repair_passes == null || Number(job.repair_passes) <= 0) return '';
        return `<p>${state.context.esc(state.context.t('game_maker.repair_passes', { count: job.repair_passes }))}</p>`;
    }

    function usageMarkup(state, job) {
        const usage = job && job.usage;
        if (!usage || !Number(usage.calls)) return '';
        const { esc, t } = state.context;
        const tokens = usage.tokens == null
            ? t('game_maker.usage_tokens_unknown')
            : t('game_maker.usage_tokens', { count: usage.tokens });
        const cost = usage.cost_usd == null || !Number.isFinite(Number(usage.cost_usd)) || Number(usage.cost_usd) < 0
            ? t('game_maker.usage_cost_unknown')
            : t('game_maker.usage_cost', { cost: formatUsageCost(usage.cost_usd) });
        return `<p class="gm-result-usage">${esc(t('game_maker.usage_line', { calls: usage.calls, tokens, cost }))}</p>`;
    }

    function formatUsageCost(value) {
        const n = Number(value);
        const digits = n === 0 || n >= 0.01 ? 4 : 6;
        return n.toFixed(digits).replace(/\.?0+$/, '');
    }

    function failureKind(kind, message) {
        const text = String(message || '').toLowerCase();
        if (/time limit|timed out|timeout|deadline exceeded/.test(text)) return 'timeout';
        if (kind === 'cancelled') return '';
        if (/preview|browser|chrom/.test(text) && /unavailable|not available|not installed|could not start/.test(text)) return 'preview';
        if (/validation failed/.test(text)) return 'validation';
        if (/planning failed|accepted plan|^plan:/.test(text)) return 'planning';
        if (/llm|provider|model|agent loop|api key|rate limit|quota/.test(text)) return 'model';
        return 'generic';
    }

    function stepBarShown(state) {
        const bar = state.container.querySelector('[data-gm-steps]');
        return !!(bar && bar.offsetParent);
    }

    async function setStudioStep(state, step) {
        if (step !== 'agent' && step !== 'preview') return false;
        const root = state.container.querySelector('.gm-studio');
        if (!root) return false;
        const current = root.getAttribute('data-gm-step') || 'agent';
        if (current !== step && step === 'agent' && stepBarShown(state)) {
            if (!await confirmLeaveValidating(state)) return false;
            if (state.disposed) return false;
            const phase = state.job && (state.job.phase || state.job.status);
            if (phase === 'validating') state.validationPreviewDismissed = true;
        }
        if (step === 'preview') state.validationPreviewDismissed = false;
        root.setAttribute('data-gm-step', step);
        state.container.querySelectorAll('[data-gm-steps] [data-gm-step]').forEach(button => {
            button.setAttribute('aria-pressed', button.dataset.gmStep === step ? 'true' : 'false');
        });
        return true;
    }

    async function playNow(state) {
        await setStudioStep(state, 'preview');
        await refreshPreview(state);
        const shellEl = state.container.querySelector('[data-gm-preview]');
        if (shellEl && shellEl.scrollIntoView) shellEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }

    function retryLastPrompt(state) {
        return submitChange(state, true);
    }

    // Passive model-limit line for the start and retry dialogs. Only a known
    // route inside the local catalog produces text; unknown models stay silent
    // and are never blocked (the server keeps the same rule).
    function modelLimitText(state, limits) {
        if (!limits || !Number(limits.context_window)) return '';
        return state.context.t('game_maker.model_limits_line', {
            context: Number(limits.context_window),
            output: Number(limits.max_output_tokens) || 0
        });
    }

    function modelLimitWarning(state, limits) {
        if (!limits || !limits.below_minimum) return '';
        return state.context.t('game_maker.model_limits_warning', {
            minimum: Number(limits.minimum_context_window) || 0
        });
    }

    function applyModelLimits(state, layer, limits) {
        const line = layer.querySelector('[data-gm-model-limits]');
        if (line) {
            line.textContent = modelLimitText(state, limits);
            line.hidden = !line.textContent;
        }
        const warning = layer.querySelector('[data-gm-model-warning]');
        if (warning) {
            warning.textContent = modelLimitWarning(state, limits);
            warning.hidden = !warning.textContent;
        }
        const holder = layer.querySelector('[data-gm-model-state]');
        if (holder) holder.setAttribute('data-below-minimum', limits && limits.below_minimum ? 'true' : 'false');
    }

    // Debounced limit probe bound to a modal's provider/model fields. The
    // cached result decides the confirmation and the `allow_small_model` flag;
    // a failed probe degrades to "unknown" and never blocks the dialog.
    function bindModelLimits(state, layer, form, limitsState) {
        const provider = form.querySelector('[name="provider_id"]');
        const model = form.querySelector('[name="model"]');
        if (!provider || !model) return;
        let timer = null;
        let requestID = 0;
        const refresh = async () => {
            const current = ++requestID;
            const providerID = String(provider.value || '');
            const modelID = String(model.value || '').trim();
            if (!providerID && !modelID) {
                limitsState.limits = null;
                applyModelLimits(state, layer, null);
                return;
            }
            try {
                const limits = await state.api.modelLimits(providerID, modelID);
                if (current !== requestID || state.disposed) return;
                limitsState.limits = limits || null;
            } catch (error) {
                if (current !== requestID || state.disposed) return;
                limitsState.limits = null;
            }
            applyModelLimits(state, layer, limitsState.limits);
        };
        const schedule = () => {
            if (timer) clearTimeout(timer);
            timer = setTimeout(() => { timer = null; refresh(); }, 350);
        };
        provider.addEventListener('change', schedule);
        model.addEventListener('input', schedule);
        state.modalCleanup = () => {
            if (timer) clearTimeout(timer);
            timer = null;
            requestID += 1;
        };
        refresh();
    }

    async function confirmSmallModel(state, limits) {
        if (!limits || !limits.below_minimum) return true;
        return confirmAction(state, state.context.t('game_maker.small_model_title'),
            state.context.t('game_maker.small_model_confirm', {
                context: Number(limits.context_window) || 0,
                minimum: Number(limits.minimum_context_window) || 0
            }));
    }

    function smallModelFlag(limits) {
        return Boolean(limits && limits.below_minimum);
    }

    function apiErrorCode(error) {
        return error && error.body ? String(error.body.code || '') : '';
    }

    function applyStartedJob(state, job) {
        state.jobStartedAt = Date.now();
        state.repairCount = 0;
        state.lastPhase = '';
        state.validationPreviewDismissed = false;
        state.gameplayStatus = '';
        state.previewStale = true;
        state.activeJob = { job_id: job.id, project_id: state.project.id, status: job.status, phase: job.phase };
        syncJobControls(state);
    }

    // Continue the retained draft with an explicit route override. The server
    // stores the new provider/model on the project, so the note in the dialog
    // is part of the contract, not a guess.
    async function startResumedJob(state, route) {
        if (!state.project || state.jobActive) return false;
        state.jobActive = true;
        state.container.querySelector('[data-gm-outcome]')?.remove();
        try {
            finalizeStreaming(state);
            const selection = postedSelection(state, true);
            const job = await state.api.startJob(state.project.id, Object.assign({
                prompt: '',
                resume: true,
                validate_restored_draft: true,
                preview_diagnostics: state.previewProjectID === state.project.id ? (state.previewDiagnostics || []) : [],
                asset_pack_ids: selection.asset_pack_ids,
                model_asset_ids: selection.model_asset_ids,
                asset_selections: selection.asset_selections,
                presentation: selection.presentation,
                provider_id: state.project.provider_id,
                model: state.project.model
            }, route || {}));
            state.job = job;
            if (job && job.prompt) {
                state.messages.push({ role: 'user', content: job.prompt });
                renderConversation(state);
                scrollConversation(state);
            }
            if (route && route.provider_id) state.project.provider_id = route.provider_id;
            if (route && route.model) state.project.model = route.model;
            const index = state.projects.findIndex(item => item.id === state.project.id);
            if (index >= 0) state.projects[index] = state.project;
            applyStartedJob(state, job);
            return true;
        } catch (error) {
            state.jobActive = false;
            fail(state, error);
            return false;
        }
    }

    async function extendRepairsAction(state, passes) {
        if (!state.project || !passes) return;
        const count = Math.min(6, Math.max(1, Number(passes) || 0));
        const confirmed = await confirmAction(state, state.context.t('game_maker.extend_repairs'),
            state.context.t('game_maker.extend_repairs_note', {
                passes: count,
                model: state.project.model || state.latestJob?.model || ''
            }));
        if (!confirmed || state.disposed) return;
        await startResumedJob(state, { max_repair_passes: count });
    }

    function showRetryModelModal(state) {
        if (!state.project || state.jobActive) return;
        const cap = state.capabilities || {};
        const { esc, t } = state.context;
        const eligibleProviders = (cap.providers || []).filter(provider => provider && provider.id && provider.usage !== 'decisions');
        const currentProvider = String(state.project.provider_id || cap.default_provider_id || '');
        const currentModel = String(state.project.model || cap.default_model || '');
        const providers = eligibleProviders.map(provider =>
            `<option value="${esc(provider.id)}" data-model="${esc(provider.model || '')}"
                ${provider.id === currentProvider ? 'selected' : ''}>${esc(provider.name || provider.id)}</option>`
        ).join('');
        const limitsState = { limits: null };
        showModal(state, `
            <form class="gm-modal gm-retry-model-modal" data-gm-retry-model-form>
                <header><div><h2>${esc(t('game_maker.retry_model_title'))}</h2></div>
                    <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
                <p class="gm-field-help">${esc(t('game_maker.retry_model_note'))}</p>
                <div class="gm-form-grid" data-gm-model-state>
                    <label>${esc(t('game_maker.provider'))}<select name="provider_id">${providers}</select></label>
                    <label>${esc(t('game_maker.model'))}<input name="model" value="${esc(currentModel)}" required></label>
                </div>
                <p class="gm-field-help" data-gm-model-limits hidden></p>
                <p class="gm-model-warning" data-gm-model-warning role="status" hidden></p>
                <footer><button type="button" data-modal-close>${esc(t('game_maker.cancel'))}</button>
                    <button type="submit" class="gm-primary">${esc(t('game_maker.retry_model_start'))}</button></footer>
            </form>`, layer => {
            const form = layer.querySelector('[data-gm-retry-model-form]');
            form.querySelector('select[name="provider_id"]').addEventListener('change', event => {
                const model = event.target.selectedOptions[0]?.dataset.model;
                if (model) form.elements.model.value = model;
            });
            bindModelLimits(state, layer, form, limitsState);
            let submitting = false;
            form.addEventListener('submit', async event => {
                event.preventDefault();
                if (submitting) return;
                submitting = true;
                const allowed = await confirmSmallModel(state, limitsState.limits);
                if (!allowed || state.disposed) {
                    submitting = false;
                    return;
                }
                closeModal(state);
                const started = await startResumedJob(state, {
                    provider_id: String(form.elements.provider_id.value || ''),
                    model: String(form.elements.model.value || '').trim(),
                    allow_small_model: smallModelFlag(limitsState.limits)
                });
                if (!started) submitting = false;
            });
            form.elements.model?.focus();
        });
    }

    async function reloadProjectRecord(state) {
        if (!state.project) return false;
        const projectID = state.project.id;
        try {
            const body = await state.api.getProject(projectID);
            if (state.disposed || state.project?.id !== projectID) return false;
            state.project = body.project;
            state.messages = body.messages || state.messages;
            if (body.latest_job) state.latestJob = body.latest_job;
            if (body.plan_summary) state.planSummary = body.plan_summary;
            const index = state.projects.findIndex(item => item.id === state.project.id);
            if (index >= 0) state.projects[index] = state.project;
            renderProjects(state);
            if (state.jobActive) renderProjectMeta(state);
            else renderProject(state);
            return true;
        } catch (error) {
            fail(state, error);
            return false;
        }
    }

    function appendAgentDelta(state, content) {
        if (!content) return;
        const log = state.container.querySelector('[data-gm-conversation]');
        log.querySelector('.gm-empty')?.remove();
        let message = log.querySelector('.gm-message-assistant[data-streaming="true"]');
        if (!message) {
            message = document.createElement('article');
            message.className = 'gm-message gm-message-assistant';
            message.dataset.streaming = 'true';
            message.innerHTML = `<span>${state.context.esc(state.context.t('game_maker.agent'))}</span><p></p>`;
            log.appendChild(message);
        }
        message.querySelector('p').textContent += content;
        scrollConversation(state);
    }

    function appendActivity(state, content) {
        const log = state.container.querySelector('[data-gm-conversation]');
        const item = document.createElement('div');
        item.className = 'gm-activity';
        item.textContent = content;
        log.appendChild(item);
        scrollConversation(state);
    }

    async function preparePreviewReplacement(state) {
        if (await window.GameMakerStudioPreview?.flush(state) !== false) return true;
        return confirmAction(state, state.context.t('game_maker.save_failed_title'), state.context.t('game_maker.leave_unsaved'));
    }

    async function refreshLatestPreview(state, options = {}) {
        if (!state.project || state.project.variant !== 'voxel') return refreshPreview(state);
        const projectID = state.project.id, frame = state.frame, grant = state.previewGrant;
        const requestedRevision = Number(options.revision?.number ?? options.revision);
        const targetRevision = Number.isSafeInteger(requestedRevision) && requestedRevision > 0
            ? requestedRevision : null;
        const attempted = state.lastRevisionRefreshAttempt;
        if (!options.force && targetRevision !== null && attempted?.projectID === projectID && attempted.revision === targetRevision) return false;
        const activeRefresh = state.revisionRefreshIdentity;
        if (activeRefresh && activeRefresh.projectID === projectID && activeRefresh.frame === frame && activeRefresh.grant === grant) return activeRefresh.promise;
        const stillCurrentFrame = () => !state.disposed && state.project?.id === projectID
            && state.frame === frame && state.previewGrant === grant;
        const markAttempted = revision => {
            if (revision !== null && revision !== undefined) state.lastRevisionRefreshAttempt = { projectID, revision };
        };
        const task = (async () => {
            const proceed = await preparePreviewReplacement(state);
            if (!stillCurrentFrame()) return false;
            if (!proceed) {
                markAttempted(targetRevision);
                return false;
            }
            if (!await reloadProjectRecord(state) || !stillCurrentFrame()) return false;
            const currentRevision = Number(state.project.current_revision) || 0;
            if (targetRevision !== null && currentRevision < targetRevision) return false;
            state.playStateRevisionInvalid = false;
            const refreshed = await refreshPreview(state, { skipFlush: true });
            if (refreshed) markAttempted(currentRevision);
            return refreshed;
        })();
        const identity = { projectID, frame, grant, promise: task };
        state.revisionRefreshIdentity = identity;
        try {
            return await task;
        } finally {
            if (state.revisionRefreshIdentity === identity) state.revisionRefreshIdentity = null;
        }
    }

    async function refreshPreview(state, options = {}) {
        if (!state.project) return;
        const projectID = state.project.id;
        const requestID = state.previewRequestID = (state.previewRequestID || 0) + 1;
        if (!options.skipFlush && !await preparePreviewReplacement(state)) return false;
        if (state.disposed || state.project.id !== projectID || state.previewRequestID !== requestID) return;
        if (window.GameMakerStudioPreview) {window.GameMakerStudioPreview.cancelVisual(state);window.GameMakerStudioPreview.clearLoading(state);}
        try {
            const grant = await state.api.previewGrant(projectID);
            if (state.disposed || state.project.id !== projectID || state.previewRequestID !== requestID) return;
            state.previewGrant = grant;
            state.previewProjectID = projectID;
            state.playStateBusy = false;
            state.playStateRevisionInvalid = false;
            clearDiagnostics(state);
            const channelID = crypto.getRandomValues(new Uint32Array(4)).join('-');
            state.channelID = channelID;
            const frame = document.createElement('iframe');
            frame.className = 'gm-preview-frame';
            frame.title = state.context.t('game_maker.live_preview');
            frame.setAttribute('sandbox', 'allow-scripts allow-pointer-lock');
            frame.setAttribute('allowfullscreen', '');
            frame.setAttribute('referrerpolicy', 'no-referrer');
            const shell = state.container.querySelector('[data-gm-preview]');
            const drift = shell.querySelector('[data-gm-drift]');
            state.previewVisibility?.disconnect();
            shell.replaceChildren(frame);
            if (drift) shell.append(drift);
            state.frame = frame;
            state.activity?.sync();
            let visible=true;
            const signalActive=()=>frame.contentWindow?.postMessage({type:'aurago:game:active',active:visible&&!document.hidden},'*');
            state.previewVisibility=new IntersectionObserver(entries=>{visible=entries[0].isIntersecting;signalActive()});state.previewVisibility.observe(frame);
            frame.addEventListener('load',signalActive);
            state.previewStale = false;
            // Attach load listeners before assigning src so cached responses
            // cannot settle the iframe before the loading overlay is armed.
            if (window.GameMakerStudioPreview) window.GameMakerStudioPreview.showLoading(state, shell, frame);
            frame.src = grant.url + '#gm-channel=' + encodeURIComponent(channelID) + (state.sceneDebug ? '&gm-debug=1' : '');
            return true;
        } catch (error) {
            if (state.disposed || state.project.id !== projectID || state.previewRequestID !== requestID) return;
            addDiagnostic(state, { level: 'error', message: error.message || String(error) });
            return false;
        }
    }

    function handlePreviewMessage(state, event) {
        window.GameMakerStudioPreview.handleMessage(state, event);
    }

    function addDiagnostic(state, diagnostic) {
        state.diagnostics.push({
            level: String(diagnostic.level || 'info').slice(0, 32),
            message: String(diagnostic.message || '').slice(0, 1000),
            file: String(diagnostic.file || '').slice(0, 240)
        });
        state.diagnostics = state.diagnostics.slice(-50);
        const list = state.container.querySelector('[data-gm-diagnostic-list]');
        list.innerHTML = state.diagnostics.map(item => `<li class="is-${state.context.esc(item.level)}">
            <strong>${state.context.esc(item.level)}</strong>
            <span>${state.context.esc(item.message)}</span>
            ${item.file ? `<small>${state.context.esc(item.file)}</small>` : ''}
        </li>`).join('');
        state.container.querySelector('[data-gm-diagnostic-count]').textContent = String(state.diagnostics.length);
        const panel = state.container.querySelector('[data-gm-diagnostics]');
        panel.classList.toggle('has-errors',
            state.diagnostics.some(item => item.level === 'error' || item.level === 'runtime'));
        if (diagnostic.level === 'error' || diagnostic.level === 'runtime') {
            panel.open = true;
        }
    }

    function clearDiagnostics(state) {
        state.diagnostics = [];
        state.previewDiagnostics = [];
        state.previewReported = new Set();
        state.container.querySelector('[data-gm-diagnostic-list]').innerHTML = '';
        state.container.querySelector('[data-gm-diagnostic-count]').textContent = '0';
        const panel = state.container.querySelector('[data-gm-diagnostics]');
        panel.classList.remove('has-errors');
        panel.open = false;
    }

    function postedSelection(state, resume, dimension) {
        if (window.GameMakerStudioAssets?.jobSelection) {
            return window.GameMakerStudioAssets.jobSelection(state, resume, dimension);
        }
        const dim = dimension || state.project?.dimension;
        return {
            asset_pack_ids: state.selectedAssetPackIDs || [],
            model_asset_ids: dim === '3d' ? state.selectedModelAssetIDs || [] : [],
            asset_selections: state.selectedAssetSelections || [],
            presentation: state.selectedPresentation
        };
    }

    function showCreateModal(state) {
        const cap = state.capabilities;
        if (capabilityState(cap).status !== 'ready') return;
        if (!state.selectionHold) window.GameMakerStudioAssets?.holdSelection(state);
        const { esc, t } = state.context;
        const prefer3D=state.selectedModelAssetIDs?.length || state.selectedAssetSelections?.some(a=>state.selectedAssetDimensions[a.pack_id]==='3d');
        const eligibleProviders = (cap.providers || []).filter(provider => provider && provider.id && provider.usage !== 'decisions');
        const selectedProvider = String(cap.default_provider_id || '');
        const missingProvider = selectedProvider && !eligibleProviders.some(provider => String(provider.id) === selectedProvider)
            ? `<option value="${esc(selectedProvider)}" disabled selected>${esc(selectedProvider)} — ${esc(t('game_maker.provider_unavailable'))}</option>` : '';
        const providers = missingProvider + eligibleProviders.map(provider =>
            `<option value="${esc(provider.id)}" data-model="${esc(provider.model || '')}"
                ${provider.id === cap.default_provider_id ? 'selected' : ''}>${esc(provider.name || provider.id)}</option>`
        ).join('');
        const examples = [['2d', [1, 2, 4, 5, 6, 7, 8]], ['3d', [3, 9, 10, 11, 12, 13]], ['voxel', [14, 15]]];
        const chips = examples.map(([dimension, ideas]) => `
            <div class="gm-idea-chips" role="group" aria-label="${esc(dimension === 'voxel' ? t('game_maker.voxel') : dimension.toUpperCase())}">
                <span>${esc(dimension === 'voxel' ? t('game_maker.voxel') : dimension.toUpperCase())}</span>${ideas.map(n =>
                    `<button type="button" data-idea="${n}" data-dimension="${dimension}">${esc(t('game_maker.example_idea_' + n))}</button>`
                ).join('')}
            </div>`).join('');
        showModal(state, `
            <form class="gm-modal gm-create-modal" data-gm-create>
                <header><div><span>${esc(t('game_maker.create_kicker'))}</span><h2>${esc(t('game_maker.create_title'))}</h2></div>
                    <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
                <label>${esc(t('game_maker.project_name'))}<input name="name" maxlength="120" required autofocus></label>
                <fieldset><legend>${esc(t('game_maker.dimension'))}</legend>
                    <label class="gm-dimension-card"><input type="radio" name="dimension" value="2d" ${prefer3D ? '' : 'checked'}><strong>2D</strong><span>Phaser ${esc(cap.phaser_version)}</span></label>
                    <label class="gm-dimension-card"><input type="radio" name="dimension" value="3d" ${prefer3D ? 'checked' : ''}><strong>3D</strong><span>Three.js ${esc(cap.three_version)}</span></label>
                    <label class="gm-dimension-card"><input type="radio" name="dimension" value="voxel"><strong>${esc(t('game_maker.voxel'))}</strong><span>${esc(t('game_maker.voxel_description'))}</span></label>
                </fieldset>
                <label>${esc(t('game_maker.description'))}<textarea name="description" rows="5" maxlength="12000" required
                    placeholder="${esc(t('game_maker.description_placeholder'))}"></textarea></label>
                <small class="gm-field-help">${esc(t('game_maker.description_help'))}</small>
                <div class="gm-idea-examples" role="group" aria-label="${esc(t('game_maker.example_ideas_label'))}">
                    <span class="gm-field-help">${esc(t('game_maker.example_ideas_label'))}</span>${chips}
                </div>
                <div class="gm-form-grid" data-gm-model-state>
                    <label>${esc(t('game_maker.provider'))}<select name="provider_id">${providers}</select></label>
                    <label>${esc(t('game_maker.model'))}<input name="model" value="${esc(cap.default_model || '')}" required></label>
                </div>
                <p class="gm-field-help" data-gm-model-limits hidden></p>
                <p class="gm-model-warning" data-gm-model-warning role="status" hidden></p>
                <div class="gm-media-options">
                    ${mediaToggle(state, 'use_image_generation', 'image_generation', 'image_assets')}
                    ${mediaToggle(state, 'use_music_generation', 'music_generation', 'music_assets')}
                </div>
                ${window.GameMakerStudioAssets ? window.GameMakerStudioAssets.selectionMarkup(state) : ''}
                <footer><button type="button" data-modal-close>${esc(t('game_maker.cancel'))}</button>
                    <button type="submit" class="gm-primary">${esc(t('game_maker.start_creating'))}</button></footer>
            </form>`, layer => {
            const form = layer.querySelector('[data-gm-create]');
            const limitsState = { limits: null };
            form.querySelector('select[name="provider_id"]').addEventListener('change', event => {
                const model = event.target.selectedOptions[0]?.dataset.model;
                if (model) form.elements.model.value = model;
            });
            bindModelLimits(state, layer, form, limitsState);
            layer.querySelectorAll('[data-idea]').forEach(chip => chip.addEventListener('click', async () => {
                const idea = t('game_maker.example_idea_' + chip.dataset.idea);
                const target = form.elements.description;
                if (target.value.trim()) {
                    const overwrite = await confirmAction(state, t('game_maker.idea_overwrite_title'),
                        t('game_maker.idea_overwrite_confirm'));
                    if (!overwrite) return;
                }
                target.value = idea;
                form.querySelector(`input[name="dimension"][value="${chip.dataset.dimension}"]`).checked = true;
                target.focus();
            }));
            let creating = false;
            form.addEventListener('submit', async event => {
                event.preventDefault();
                if (creating) return;
                creating = true;
                const data = new FormData(form);
                const variant = data.get('dimension') === 'voxel' ? 'voxel' : '';
                const dimension = variant ? '3d' : String(data.get('dimension') || '2d');
                const selectionError=window.GameMakerStudioAssets?.selectionError(state,dimension);
                if(selectionError){creating=false;modalError(layer,selectionError);return;}
                if (dimension !== '3d' && state.selectedModelAssetIDs?.length) {
                    creating = false;
                    modalError(layer, t('game_maker.model_requires_3d'));
                    return;
                }
                const request = {
                    name: String(data.get('name') || '').trim(),
                    dimension, variant,
                    description: String(data.get('description') || '').trim(),
                    provider_id: String(data.get('provider_id') || ''),
                    model: String(data.get('model') || ''),
                    use_image_generation: data.get('use_image_generation') === 'on',
                    use_music_generation: data.get('use_music_generation') === 'on'
                };
                // A known route below the Studio floor starts only after an
                // explicit confirmation; the server enforces the same rule.
                const allowSmallModel = smallModelFlag(limitsState.limits);
                if (!await confirmSmallModel(state, limitsState.limits)) {
                    creating = false;
                    return;
                }
                setModalBusy(layer, true);
                let createdProject = null;
                try {
                    createdProject = await state.api.createProject(request);
                    if (window.GameMakerStudioAssets?.rememberAssetSelection) {
                        state.selectionHold = false;
                        state.selectionStash = null;
                        state.selectionHoldProject = '';
                        window.GameMakerStudioAssets.rememberAssetSelection(state, createdProject.id);
                    }
                    const selection = postedSelection(state, false, request.dimension);
                    const job = await state.api.startJob(createdProject.id, {
                        prompt: request.description,
                        asset_pack_ids: selection.asset_pack_ids,
                        model_asset_ids: selection.model_asset_ids,
                        asset_selections: selection.asset_selections,
                        presentation: selection.presentation,
                        provider_id: request.provider_id,
                        model: request.model,
                        image_generation: request.use_image_generation,
                        music_generation: request.use_music_generation,
                        allow_small_model: allowSmallModel
                    });
                    closeModal(state);
                    state.projects.unshift(createdProject);
                    state.jobStartedAt = Date.now();
                    state.activeJob = { job_id: job.id, project_id: createdProject.id, status: job.status, phase: job.phase };
                    await openProject(state, createdProject.id);
                    if (!state.job) state.job = job;
                    syncJobControls(state);
                } catch (error) {
                    creating = false;
                    if (createdProject) {
                        closeModal(state);
                        state.projects.unshift(createdProject);
                        await openProject(state, createdProject.id);
                        fail(state, error);
                    } else {
                        setModalBusy(layer, false);
                        modalError(layer, apiErrorCode(error) === 'model_context_too_small'
                            ? t('game_maker.small_model_blocked')
                            : (error.message || String(error)));
                    }
                }
            });
        });
    }

    async function submitChange(state, resume = false) {
        const form = state.container.querySelector('[data-gm-change-form]');
        const input = form.querySelector('textarea');
        const prompt = resume ? '' : input.value.trim();
        if ((!resume && !prompt) || !state.project || state.jobActive || input.disabled) return;
        const selectionError=window.GameMakerStudioAssets?.selectionError(state,state.project.dimension);
        if(selectionError){fail(state,new Error(selectionError));return;}
        if (state.project.dimension !== '3d' && state.selectedModelAssetIDs?.length) {
            fail(state, new Error(state.context.t('game_maker.model_requires_3d')));
            return;
        }
        // Lock the form immediately so a double click / double Enter cannot
        // fire a second request while startJob is still in flight.
        state.jobActive = true;
        state.container.querySelector('[data-gm-outcome]')?.remove();
        input.disabled = true;
        form.querySelector('button[type="submit"]').disabled = true;
        if (!resume) {
            input.value = '';
            autoGrow(input);
            state.messages.push({ role: 'user', content: prompt });
            renderConversation(state);
            scrollConversation(state);
        }
        try {
            finalizeStreaming(state);
            const selection = postedSelection(state, resume);
            state.job = await state.api.startJob(state.project.id, {
                prompt,
                resume,
                validate_restored_draft: resume,
                preview_diagnostics: state.previewProjectID === state.project.id ? (state.previewDiagnostics || []) : [],
                asset_pack_ids: selection.asset_pack_ids,
                model_asset_ids: selection.model_asset_ids,
                asset_selections: selection.asset_selections,
                presentation: selection.presentation,
                provider_id: state.project.provider_id,
                model: state.project.model
            });
            if (resume && state.job.prompt) {
                state.messages.push({ role: 'user', content: state.job.prompt });
                renderConversation(state);
                scrollConversation(state);
            }
            applyStartedJob(state, state.job);
        } catch (error) {
            if (!resume) {
                input.value = prompt;
                autoGrow(input);
                state.messages.pop();
                renderConversation(state);
            }
            state.jobActive = false;
            input.disabled = false;
            form.querySelector('button[type="submit"]').disabled = false;
            fail(state, error);
        }
    }

    async function stopJob(state) {
        if (!state.job || state.job.status === 'cancelling' || !activeStatuses.has(state.job.status)) return;
        try {
            await state.api.cancelJob(state.job.id);
            state.job.status = 'cancelling';
            syncJobControls(state);
        } catch (error) {
            fail(state, error);
        }
    }

    async function renameProject(state) {
        if (!state.project || !state.capabilities.allow_edit || typeof state.context.promptDialog !== 'function') return;
        const name = await state.context.promptDialog(state.context.t('game_maker.rename_title'), state.project.name);
        if (!name || !String(name).trim()) return;
        try {
            state.project = await state.api.renameProject(state.project.id, String(name).trim());
            await reloadProjectRecord(state);
        } catch (error) {
            fail(state, error);
        }
    }

    async function deleteProject(state) {
        if (!state.project || !state.capabilities.allow_delete) return;
        const confirmed = await confirmAction(state, state.context.t('game_maker.delete_title'),
            state.context.t('game_maker.delete_confirm', { name: state.project.name }));
        if (!confirmed) return;
        try {
            const removedID = state.project.id;
            await state.api.deleteProject(removedID);
            if (window.GameMakerStudioAssets?.forgetAssetSelection) window.GameMakerStudioAssets.forgetAssetSelection(removedID);
            state.projects = state.projects.filter(item => item.id !== removedID);
            if (state.knowledgeClose) state.knowledgeClose();
            setButton(state, 'knowledge', false);
            state.project = null;
            state.job = null;
            state.drift = null;
            state.planSummary = null;
            state.latestJob = null;
            stopElapsed(state);
            closeEvents(state);
            state.activity?.dispose();
            state.container.innerHTML = shell(state);
            state.activity = window.GameMakerStudioActivity?.create(state);
            bindShell(state);
            applyCapabilities(state);
            renderProjects(state);
            syncJobControls(state);
            updateStatus(state, state.capabilities.enabled ? 'ready' : 'disabled');
        } catch (error) {
            fail(state, error);
        }
    }

    function openInCodeStudio(state) {
        if (!state.project || !state.capabilities.code_studio || typeof state.context.openApp !== 'function') return;
        state.context.openApp('code-studio', { path: state.project.project_key, pathKind: 'directory' });
    }

    function exportProject(state) {
        if (!state.project || !state.project.current_revision) return;
        const link = document.createElement('a');
        link.href = state.api.exportURL(state.project.id);
        link.download = '';
        link.rel = 'noopener';
        link.click();
    }

    async function confirmLeaveValidating(state) {
        const phase = state.job && (state.job.phase || state.job.status);
        if (phase !== 'validating') return true;
        return confirmAction(state, state.context.t('game_maker.leave_validating_title'), state.context.t('game_maker.leave_validating'));
    }

    function driftCount(drift) {
        if (!drift || !Array.isArray(drift.changed) || !Array.isArray(drift.added) || !Array.isArray(drift.removed)) return 0;
        const count = Number(drift.changed_count) + Number(drift.added_count) + Number(drift.removed_count);
        return Number.isFinite(count) && count > 0 ? count : 0;
    }

    function renderDrift(state) {
        const badge = state.container.querySelector('[data-gm-drift]');
        const hint = state.container.querySelector('[data-gm-drift-hint]');
        const count = driftCount(state.drift);
        if (badge) {
            badge.hidden = count <= 0;
            const text = badge.querySelector('[data-gm-drift-text]');
            if (text) text.textContent = count > 0 ? state.context.t('game_maker.workspace_drift', { count }) : '';
            const adopt = badge.querySelector('[data-gm-action="adopt"]');
            if (adopt) {
                const allowed = count > 0 && !state.jobActive && !otherProjectBusy(state) && Boolean(state.capabilities?.allow_edit);
                adopt.hidden = !allowed;
            }
        }
        if (hint) {
            hint.hidden = count <= 0;
            hint.textContent = count > 0 ? state.context.t('game_maker.drift_inherits', { count }) : '';
        }
    }

    async function refreshWorkspaceDrift(state) {
        const projectID = state.project?.id;
        if (!projectID || !state.api.workspaceDrift) return;
        const requestID = state.driftRequestID = (state.driftRequestID || 0) + 1;
        let drift = null;
        try {
            drift = await state.api.workspaceDrift(projectID);
        } catch (_) {
            drift = null;
        }
        if (state.disposed || state.project?.id !== projectID || state.driftRequestID !== requestID) return;
        state.drift = drift;
        renderDrift(state);
    }

    async function adoptWorkspace(state) {
        if (state.adoptPending || state.jobActive || !state.project || !state.capabilities?.allow_edit) return;
        if (driftCount(state.drift) <= 0 || otherProjectBusy(state)) return;
        state.adoptPending = true;
        try {
            const confirmed = await confirmAction(state, state.context.t('game_maker.adopt_title'), state.context.t('game_maker.adopt_confirm'));
            if (!confirmed || state.disposed || state.jobActive || !state.capabilities?.allow_edit) return;
            if (state.project?.variant === 'voxel' && state.preparePreviewReplacement && !await state.preparePreviewReplacement()) return;
            if (state.disposed || state.jobActive || driftCount(state.drift) <= 0) return;
            state.jobActive = true;
            state.container.querySelector('[data-gm-outcome]')?.remove();
            syncJobControls(state);
            state.job = await state.api.startJob(state.project.id, { adopt_workspace: true });
            state.jobStartedAt = Date.now();
            state.repairCount = 0;
            state.lastPhase = '';
            state.validationPreviewDismissed = false;
            state.gameplayStatus = '';
            state.previewStale = true;
            state.activeJob = { job_id: state.job.id, project_id: state.project.id, status: state.job.status, phase: state.job.phase };
            syncJobControls(state);
        } catch (error) {
            state.jobActive = false;
            syncJobControls(state);
            fail(state, error);
        } finally {
            state.adoptPending = false;
        }
    }

    function otherProjectBusy(state) {
        return Boolean(state.activeJob && state.project && state.activeJob.project_id !== state.project.id);
    }

    function busyProjectName(state) {
        const match = state.projects.find(item => state.activeJob && item.id === state.activeJob.project_id);
        return match ? match.name : '';
    }

    function syncBusyPoll(state) {
        const needsPoll = otherProjectBusy(state);
        if (needsPoll && !state.busyPoll) {
            state.busyPoll = setInterval(() => refreshActiveJob(state), 15000);
        } else if (!needsPoll && state.busyPoll) {
            clearInterval(state.busyPoll);
            state.busyPoll = null;
        }
    }

    async function refreshActiveJob(state) {
        if (state.disposed) return;
        try {
            const capabilities = await state.api.capabilities();
            if (state.disposed) return;
            state.capabilities = capabilities;
            state.activeJob = capabilities.active_job || null;
            renderProjects(state);
            syncJobControls(state);
        } catch (_) {
            // Keep the previous busy state when the poll fails.
        }
    }

    function syncJobControls(state) {
        const active = Boolean(state.job && activeStatuses.has(state.job.status));
        state.jobActive = active;
        state.activity?.sync();
        updateJobBanner(state);
        const otherBusy = otherProjectBusy(state);
        const form = state.container.querySelector('[data-gm-change-form]');
        const editable = Boolean(state.project && !active &&
            state.capabilities && state.capabilities.allow_edit && !otherBusy);
        const textarea = form.querySelector('textarea');
        textarea.disabled = !editable;
        form.querySelector('button').disabled = !editable;
        const hint = form.querySelector('[data-gm-busy-hint]');
        hint.hidden = !otherBusy;
        if (otherBusy) {
            hint.textContent = state.context.t('game_maker.busy_other', { name: busyProjectName(state) });
        }
        if (window.GameMakerStudioPreview && window.GameMakerStudioPreview.updateStaleBadge) {
            window.GameMakerStudioPreview.updateStaleBadge(state);
        }
        renderDrift(state);
        syncBusyPoll(state);
    }

    function updateJobBanner(state) {
        const banner = state.container.querySelector('[data-gm-job-banner]');
        if (!banner) return;
        banner.hidden = !state.jobActive;
        if (!state.jobActive) {
            stopElapsed(state);
            return;
        }
        startElapsed(state);
        const phase = (state.job && (state.job.phase || state.job.status)) || '';
        let label = state.context.t('game_maker.phase_' + phase);
        if (state.repairCount > 0 && phase === 'building') {
            label = state.context.t('game_maker.repair_attempt', { n: state.repairCount });
        }
        banner.querySelector('[data-gm-job-phase]').textContent = label;
        const validating = banner.querySelector('[data-gm-validating-hint]');
        if (validating) {
            validating.hidden = phase !== 'validating';
            if (phase === 'validating') validating.textContent = state.context.t('game_maker.validating_preview_hint');
        }
        const stop = banner.querySelector('[data-gm-action="stop"]');
        stop.disabled = Boolean(state.job && state.job.status === 'cancelling');
    }

    function startElapsed(state) {
        if (state.elapsedTimer) return;
        if (!state.jobStartedAt) state.jobStartedAt = Date.now();
        const node = state.container.querySelector('[data-gm-job-elapsed]');
        const tick = () => {
            if (node) node.textContent = formatElapsed(Date.now() - state.jobStartedAt);
        };
        tick();
        state.elapsedTimer = setInterval(tick, 1000);
    }

    function stopElapsed(state) {
        if (state.elapsedTimer) clearInterval(state.elapsedTimer);
        state.elapsedTimer = null;
    }

    function formatElapsed(ms) {
        const total = Math.max(0, Math.floor(ms / 1000));
        const minutes = Math.floor(total / 60);
        const seconds = total % 60;
        return String(minutes).padStart(2, '0') + ':' + String(seconds).padStart(2, '0');
    }

    function autoGrow(textarea) {
        textarea.style.height = 'auto';
        textarea.style.height = Math.min(textarea.scrollHeight, 180) + 'px';
    }

    function updateStatus(state, status) {
        const node = state.container.querySelector('[data-gm-status]');
        if (!node) return;
        const key = 'game_maker.status_' + String(status || 'ready').replaceAll('-', '_');
        node.className = 'gm-status is-' + String(status || 'ready');
        node.textContent = state.context.t(key);
    }

    function setButton(state, action, enabled) {
        state.container.querySelectorAll(`[data-gm-action="${action}"]`).forEach(button => {
            button.disabled = !enabled;
        });
    }

    function showNotice(state, message, level) {
        if (typeof state.context.notify === 'function') {
            state.context.notify({ title: state.context.t('game_maker.title'), message, level });
        }
    }

    function fail(state, error) {
        const message = error && error.message ? error.message : String(error);
        addDiagnostic(state, { level: 'error', message });
        showNotice(state, message, 'error');
    }

    function scrollConversation(state) {
        const log = state.container.querySelector('[data-gm-conversation]');
        if (log) log.scrollTop = log.scrollHeight;
    }

    function closeEvents(state) {
        if (state.eventSource) state.eventSource.close();
        state.eventSource = null;
        state.reconnecting = false;
        state.lastEventID = 0;
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        if (state.knowledgeClose) state.knowledgeClose();
        state.context.setWindowBeforeClose?.(windowId, null);
        state.activity?.dispose();
        if (state.assetBrowserCleanup) state.assetBrowserCleanup();
        state.previewVisibility?.disconnect();
        closeEvents(state);
        stopElapsed(state);
        if (window.GameMakerStudioPreview) {window.GameMakerStudioPreview.cancelVisual(state);window.GameMakerStudioPreview.clearLoading(state);}
        if (state.busyPoll) clearInterval(state.busyPoll);
        if (state.moreDocListener) document.removeEventListener('click', state.moreDocListener);
        if (state.moreEscListener) document.removeEventListener('keydown', state.moreEscListener);
        if (state.previewListener) window.removeEventListener('message', state.previewListener);
        instances.delete(windowId);
    }

    const modalHelpers = { showModal, closeModal, setModalBusy, modalError, confirmAction };

    window.GameMakerStudioApp = { render, dispose, instances };
})();
