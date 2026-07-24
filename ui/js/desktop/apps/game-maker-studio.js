(function () {
    'use strict';

    const instances = new Map();
    const eventTypes = [
        'project_created', 'project_updated', 'job_status', 'phase', 'text_delta',
        'skill_activation', 'file_changed', 'asset_changed', 'preview_reload',
        'diagnostic', 'revision'
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
            frame: null,
            channelID: '',
            diagnostics: [],
            activeJob: null,
            jobStartedAt: 0,
            elapsedTimer: null,
            repairCount: 0,
            lastPhase: '',
            lastPrompt: '',
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
        state.refreshPreview = () => refreshPreview(state);
        container.innerHTML = shell(state);
        bindShell(state);
        window.addEventListener('message', state.previewListener = event => handlePreviewMessage(state, event));
        initialize(state);
    }

    function shell(state) {
        const { esc, t } = state.context;
        const menuItem = (action, key, extra) =>
            `<button type="button" role="menuitem" ${extra || ''} data-gm-action="${action}" disabled>${esc(t(key))}</button>`;
        return `
            <div class="gm-studio" data-gm-window="${esc(state.windowId)}">
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
                        <button type="button" data-gm-action="skills">${esc(t('game_maker.skills'))}</button>
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
                        <section class="gm-agent-pane">
                            <div class="gm-pane-head">
                                <div><span class="gm-kicker">${esc(t('game_maker.agent'))}</span><h2 data-gm-title>${esc(t('game_maker.choose_project'))}</h2></div>
                                <select class="gm-mobile-projects" data-gm-mobile-projects
                                    aria-label="${esc(t('game_maker.choose_project'))}" hidden></select>
                            </div>
                            <div class="gm-job-banner" data-gm-job-banner hidden>
                                <span class="gm-job-spinner" aria-hidden="true"></span>
                                <div class="gm-job-banner-text">
                                    <strong>${esc(t('game_maker.job_banner_title'))}</strong>
                                    <span data-gm-job-phase></span>
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
                            </form>
                        </section>
                        <section class="gm-preview-pane">
                            <div class="gm-pane-head">
                                <div><span class="gm-kicker">${esc(t('game_maker.live_preview'))}</span><h2>${esc(t('game_maker.play_here'))}</h2></div>
                                <div class="gm-preview-tools">
                                    <button type="button" data-gm-action="reload" disabled>${esc(t('game_maker.reload'))}</button>
                                    <button type="button" data-gm-action="fullscreen" disabled
                                        aria-label="${esc(t('game_maker.fullscreen'))}" title="${esc(t('game_maker.fullscreen'))}">⛶</button>
                                    <button type="button" data-gm-action="open_tab" disabled
                                        aria-label="${esc(t('game_maker.open_tab'))}" title="${esc(t('game_maker.open_tab'))}">↗</button>
                                </div>
                            </div>
                            <div class="gm-preview-shell" data-gm-preview>
                                <div class="gm-preview-empty">
                                    <img src="/img/desktop-icons/game-maker-studio.svg" alt="">
                                    <strong>${esc(t('game_maker.preview_waiting'))}</strong>
                                    <span>${esc(t('game_maker.preview_waiting_hint'))}</span>
                                </div>
                            </div>
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
            const button = event.target.closest('[data-gm-action]');
            if (!button) return;
            closeMoreMenu(state);
            const modals = window.GameMakerStudioModals;
            const preview = window.GameMakerStudioPreview;
            const actions = {
                new: () => showCreateModal(state),
                skills: () => modals ? modals.showSkillsModal(state, modalHelpers) : fail(state, new Error('Game Maker Studio modules failed to load')),
                revisions: () => modals ? modals.showRevisionsModal(state, modalHelpers) : fail(state, new Error('Game Maker Studio modules failed to load')),
                code: () => openInCodeStudio(state),
                export: () => exportProject(state),
                stop: () => stopJob(state),
                reload: () => refreshPreview(state),
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
        const newButton = state.container.querySelector('[data-gm-action="new"]');
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
        newButton.disabled = availability.status !== 'ready';
        newButton.title = message;
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
                <span class="gm-project-dimension">${esc(String(project.dimension).toUpperCase())}</span>
                <span><strong>${esc(project.name)}</strong><small>${esc(project.status || 'draft')}${updated ? ' · ' + esc(updated) : ''}</small></span>
                <span class="gm-project-revision">v${Number(project.current_revision || 0)}</span>
                ${running ? `<span class="gm-project-running" title="${esc(t('game_maker.status_working'))}"></span>` : ''}
            </button>`;
        }).join('');
    }

    function renderMobileSelect(state) {
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
        closeEvents(state);
        stopElapsed(state);
        state.repairCount = 0;
        state.lastPhase = '';
        state.previewStale = false;
        try {
            const body = await state.api.getProject(projectID);
            if (state.disposed) return;
            state.project = body.project;
            state.messages = body.messages || [];
            state.job = null;
            state.diagnostics = [];
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
            }
            renderProject(state);
            connectEvents(state);
            if (state.project.current_revision) refreshPreview(state);
        } catch (error) {
            fail(state, error);
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
        state.container.querySelector('[data-gm-engine]').textContent = project.dimension === '3d'
            ? `Three.js ${state.capabilities.three_version}` : `Phaser ${state.capabilities.phaser_version}`;
        setButton(state, 'revisions', true);
        setButton(state, 'rename', Boolean(state.capabilities.allow_edit));
        setButton(state, 'delete', Boolean(state.capabilities.allow_delete));
        setButton(state, 'export', Boolean(project.current_revision));
        setButton(state, 'reload', Boolean(project.current_revision));
        setButton(state, 'fullscreen', Boolean(project.current_revision));
        setButton(state, 'open_tab', Boolean(project.current_revision));
        setButton(state, 'code', Boolean(state.capabilities.code_studio && project.current_revision));
        syncJobControls(state);
        updateStatus(state, project.status || 'draft');
    }

    function renderConversation(state) {
        const { esc, t } = state.context;
        state.container.querySelector('[data-gm-conversation]').innerHTML = state.messages.length
            ? state.messages.map(message => messageMarkup(state, message)).join('')
            : `<div class="gm-empty"><strong>${esc(t('game_maker.agent_ready'))}</strong><span>${esc(state.project.description)}</span></div>`;
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
        const source = new EventSource(state.api.eventURL(state.project.id, state.lastEventID));
        state.eventSource = source;
        eventTypes.forEach(type => source.addEventListener(type, event => {
            if (state.disposed || source !== state.eventSource) return;
            const payload = parseEvent(event);
            if (!payload) return;
            state.lastEventID = Math.max(state.lastEventID, Number(payload.id || event.lastEventId || 0));
            handleEvent(state, payload);
        }));
        source.onerror = () => {
            state.reconnecting = true;
            updateStatus(state, 'reconnecting');
        };
    }

    function parseEvent(event) {
        try { return JSON.parse(event.data); } catch (_) { return null; }
    }

    function handleEvent(state, event) {
        if (state.reconnecting) {
            state.reconnecting = false;
            updateStatus(state, (state.job && state.job.status) || (state.project && state.project.status) || 'ready');
        }
        const payload = event.payload || {};
        switch (event.type) {
        case 'job_status':
            handleJobStatus(state, payload);
            break;
        case 'phase':
            if (state.lastPhase === 'validating' && payload.phase === 'building') state.repairCount += 1;
            state.lastPhase = payload.phase;
            if (!state.job) state.job = { id: event.job_id };
            state.job.phase = payload.phase;
            state.job.status = payload.phase;
            if (!state.jobStartedAt) state.jobStartedAt = Date.now();
            state.container.querySelector('[data-gm-phases]').innerHTML = phaseMarkup(state, payload.phase);
            syncJobControls(state);
            break;
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
        case 'preview_reload':
            refreshPreview(state);
            break;
        case 'revision':
            appendActivity(state, state.context.t('game_maker.revision_published'));
            reloadProjectRecord(state).then(() => {
                if (!state.disposed) refreshPreview(state);
            });
            break;
        }
    }

    function handleJobStatus(state, payload) {
        if (payload.job) state.job = payload.job;
        if (state.job && payload.status) state.job.status = payload.status;
        if (!state.jobStartedAt) {
            state.jobStartedAt = state.job && state.job.started_at ? Date.parse(state.job.started_at) || Date.now() : Date.now();
        }
        updateStatus(state, payload.status || 'working');
        syncJobControls(state);
        if (!terminalStatuses.has(payload.status)) return;
        finalizeStreaming(state);
        stopElapsed(state);
        const status = payload.status;
        state.activeJob = null;
        state.repairCount = 0;
        state.lastPhase = '';
        reloadProjectRecord(state).then(() => {
            if (state.disposed) return;
            if (status === 'ready') {
                const revision = (state.job && state.job.result_revision) ||
                    (state.project && state.project.current_revision) || '';
                appendResultCard(state, 'success',
                    state.context.t('game_maker.revision_ready', { n: revision }));
            } else if (status === 'failed') {
                appendResultCard(state, 'error', payload.error ||
                    (state.job && state.job.error) || state.context.t('game_maker.job_failed_title'));
            } else {
                appendActivity(state, state.context.t('game_maker.status_cancelled'));
            }
            renderProjects(state);
        });
    }

    function finalizeStreaming(state) {
        state.container.querySelectorAll('.gm-message-assistant[data-streaming="true"]').forEach(message => {
            message.dataset.streaming = 'false';
        });
    }

    function appendResultCard(state, kind, message) {
        const { esc, t } = state.context;
        const log = state.container.querySelector('[data-gm-conversation]');
        log.querySelector('.gm-empty')?.remove();
        const card = document.createElement('div');
        card.className = `gm-result-card is-${kind}`;
        if (kind === 'success') {
            card.innerHTML = `<strong>${esc(message)}</strong>
                <button type="button" class="gm-primary" data-gm-play>${esc(t('game_maker.play_now'))}</button>`;
        } else {
            card.innerHTML = `<strong>${esc(t('game_maker.job_failed_title'))}</strong>
                <p>${esc(message)}</p>
                <button type="button" data-gm-retry>${esc(t('game_maker.retry'))}</button>`;
        }
        log.appendChild(card);
        scrollConversation(state);
    }

    async function playNow(state) {
        await refreshPreview(state);
        const shellEl = state.container.querySelector('[data-gm-preview]');
        if (shellEl && shellEl.scrollIntoView) shellEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
    }

    function retryLastPrompt(state) {
        const form = state.container.querySelector('[data-gm-change-form]');
        const textarea = form.querySelector('textarea');
        if (!state.lastPrompt || textarea.disabled) return;
        textarea.value = state.lastPrompt;
        autoGrow(textarea);
        textarea.focus();
    }

    async function reloadProjectRecord(state) {
        if (!state.project) return;
        try {
            const body = await state.api.getProject(state.project.id);
            if (state.disposed) return;
            state.project = body.project;
            state.messages = body.messages || state.messages;
            const index = state.projects.findIndex(item => item.id === state.project.id);
            if (index >= 0) state.projects[index] = state.project;
            renderProjects(state);
            if (state.jobActive) renderProjectMeta(state);
            else renderProject(state);
        } catch (error) {
            fail(state, error);
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

    async function refreshPreview(state) {
        if (!state.project) return;
        if (window.GameMakerStudioPreview) window.GameMakerStudioPreview.clearLoading(state);
        try {
            const grant = await state.api.previewGrant(state.project.id);
            if (state.disposed) return;
            const channelID = crypto.getRandomValues(new Uint32Array(4)).join('-');
            state.channelID = channelID;
            const frame = document.createElement('iframe');
            frame.className = 'gm-preview-frame';
            frame.title = state.context.t('game_maker.live_preview');
            frame.setAttribute('sandbox', 'allow-scripts');
            frame.setAttribute('allowfullscreen', '');
            frame.setAttribute('referrerpolicy', 'no-referrer');
            frame.src = grant.url + '#gm-channel=' + encodeURIComponent(channelID);
            const shell = state.container.querySelector('[data-gm-preview]');
            shell.replaceChildren(frame);
            state.frame = frame;
            state.previewStale = false;
            if (window.GameMakerStudioPreview) window.GameMakerStudioPreview.showLoading(state, shell, frame);
        } catch (error) {
            addDiagnostic(state, { level: 'error', message: error.message || String(error) });
        }
    }

    function handlePreviewMessage(state, event) {
        if (!state.frame || event.source !== state.frame.contentWindow) return;
        const data = event.data;
        if (!data || typeof data !== 'object' || data.channel !== state.channelID || data.source !== 'aurago-game') return;
        const allowed = new Set(['ready', 'runtime_error', 'resource_error', 'diagnostic']);
        if (!allowed.has(data.type)) return;
        if (data.type === 'ready') {
            if (state.previewLoadClear) state.previewLoadClear();
            return;
        }
        addDiagnostic(state, {
            level: 'runtime',
            message: String(data.message || data.type).slice(0, 1000)
        });
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

    function showCreateModal(state) {
        const cap = state.capabilities;
        if (capabilityState(cap).status !== 'ready') return;
        const { esc, t } = state.context;
        const providers = (cap.providers || []).map(provider =>
            `<option value="${esc(provider.id)}" data-model="${esc(provider.model || '')}"
                ${provider.id === cap.default_provider_id ? 'selected' : ''}>${esc(provider.name || provider.id)}</option>`
        ).join('');
        const chips = [1, 2, 3, 4].map(n =>
            `<button type="button" data-idea="${n}">${esc(t('game_maker.example_idea_' + n))}</button>`
        ).join('');
        showModal(state, `
            <form class="gm-modal gm-create-modal" data-gm-create>
                <header><div><span>${esc(t('game_maker.create_kicker'))}</span><h2>${esc(t('game_maker.create_title'))}</h2></div>
                    <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
                <label>${esc(t('game_maker.project_name'))}<input name="name" maxlength="120" required autofocus></label>
                <fieldset><legend>${esc(t('game_maker.dimension'))}</legend>
                    <label class="gm-dimension-card"><input type="radio" name="dimension" value="2d" checked><strong>2D</strong><span>Phaser ${esc(cap.phaser_version)}</span></label>
                    <label class="gm-dimension-card"><input type="radio" name="dimension" value="3d"><strong>3D</strong><span>Three.js ${esc(cap.three_version)}</span></label>
                </fieldset>
                <label>${esc(t('game_maker.description'))}<textarea name="description" rows="5" maxlength="12000" required
                    placeholder="${esc(t('game_maker.description_placeholder'))}"></textarea></label>
                <small class="gm-field-help">${esc(t('game_maker.description_help'))}</small>
                <div class="gm-idea-chips" role="group" aria-label="${esc(t('game_maker.example_ideas_label'))}">
                    <span>${esc(t('game_maker.example_ideas_label'))}</span>${chips}
                </div>
                <div class="gm-form-grid">
                    <label>${esc(t('game_maker.provider'))}<select name="provider_id">${providers}</select></label>
                    <label>${esc(t('game_maker.model'))}<input name="model" value="${esc(cap.default_model || '')}" required></label>
                </div>
                <div class="gm-media-options">
                    ${mediaToggle(state, 'use_image_generation', 'image_generation', 'image_assets')}
                    ${mediaToggle(state, 'use_music_generation', 'music_generation', 'music_assets')}
                </div>
                <footer><button type="button" data-modal-close>${esc(t('game_maker.cancel'))}</button>
                    <button type="submit" class="gm-primary">${esc(t('game_maker.start_creating'))}</button></footer>
            </form>`, layer => {
            const form = layer.querySelector('[data-gm-create]');
            form.querySelector('select[name="provider_id"]').addEventListener('change', event => {
                const model = event.target.selectedOptions[0]?.dataset.model;
                if (model) form.elements.model.value = model;
            });
            layer.querySelectorAll('[data-idea]').forEach(chip => chip.addEventListener('click', async () => {
                const idea = t('game_maker.example_idea_' + chip.dataset.idea);
                const target = form.elements.description;
                if (target.value.trim()) {
                    const overwrite = await confirmAction(state, t('game_maker.idea_overwrite_title'),
                        t('game_maker.idea_overwrite_confirm'));
                    if (!overwrite) return;
                }
                target.value = idea;
                target.focus();
            }));
            let creating = false;
            form.addEventListener('submit', async event => {
                event.preventDefault();
                if (creating) return;
                creating = true;
                setModalBusy(layer, true);
                const data = new FormData(form);
                const request = {
                    name: String(data.get('name') || '').trim(),
                    dimension: String(data.get('dimension') || '2d'),
                    description: String(data.get('description') || '').trim(),
                    provider_id: String(data.get('provider_id') || ''),
                    model: String(data.get('model') || ''),
                    use_image_generation: data.get('use_image_generation') === 'on',
                    use_music_generation: data.get('use_music_generation') === 'on'
                };
                let createdProject = null;
                try {
                    createdProject = await state.api.createProject(request);
                    const job = await state.api.startJob(createdProject.id, {
                        prompt: request.description,
                        provider_id: request.provider_id,
                        model: request.model,
                        image_generation: request.use_image_generation,
                        music_generation: request.use_music_generation
                    });
                    closeModal(state);
                    state.projects.unshift(createdProject);
                    state.lastPrompt = request.description;
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
                        modalError(layer, error.message || String(error));
                    }
                }
            });
        });
    }

    function mediaToggle(state, name, capability, label) {
        const { esc, t } = state.context;
        const available = Boolean(state.capabilities[capability]);
        return `<label class="gm-media-toggle ${available ? '' : 'is-disabled'}">
            <input type="checkbox" name="${name}" ${available ? 'checked' : 'disabled'}>
            <span><strong>${esc(t('game_maker.' + label))}</strong>
            <small>${esc(t(available ? 'game_maker.media_auto' : 'game_maker.media_unavailable'))}</small></span>
        </label>`;
    }

    async function submitChange(state) {
        const form = state.container.querySelector('[data-gm-change-form]');
        const input = form.querySelector('textarea');
        const prompt = input.value.trim();
        if (!prompt || !state.project || state.jobActive) return;
        // Lock the form immediately so a double click / double Enter cannot
        // fire a second request while startJob is still in flight.
        state.jobActive = true;
        input.disabled = true;
        form.querySelector('button[type="submit"]').disabled = true;
        input.value = '';
        autoGrow(input);
        try {
            finalizeStreaming(state);
            state.messages.push({ role: 'user', content: prompt });
            renderConversation(state);
            scrollConversation(state);
            state.job = await state.api.startJob(state.project.id, {
                prompt,
                provider_id: state.project.provider_id,
                model: state.project.model
            });
            state.lastPrompt = prompt;
            state.jobStartedAt = Date.now();
            state.repairCount = 0;
            state.lastPhase = '';
            state.previewStale = true;
            state.activeJob = { job_id: state.job.id, project_id: state.project.id, status: state.job.status, phase: state.job.phase };
            syncJobControls(state);
        } catch (error) {
            input.value = prompt;
            autoGrow(input);
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
            await state.api.deleteProject(state.project.id);
            state.projects = state.projects.filter(item => item.id !== state.project.id);
            state.project = null;
            state.job = null;
            stopElapsed(state);
            closeEvents(state);
            state.container.innerHTML = shell(state);
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
        state.context.openApp('code-studio', { path: state.project.project_key });
    }

    function exportProject(state) {
        if (!state.project || !state.project.current_revision) return;
        const link = document.createElement('a');
        link.href = state.api.exportURL(state.project.id);
        link.download = '';
        link.rel = 'noopener';
        link.click();
    }

    function showModal(state, html, mount) {
        const layer = state.container.querySelector('[data-gm-modal]');
        layer.hidden = false;
        layer.innerHTML = html;
        layer.querySelectorAll('[data-modal-close]').forEach(button =>
            button.addEventListener('click', () => closeModal(state)));
        layer.addEventListener('click', state.modalBackdrop = event => {
            if (event.target === layer) closeModal(state);
        }, { once: true });
        if (mount) mount(layer);
    }

    function closeModal(state) {
        const layer = state.container.querySelector('[data-gm-modal]');
        if (!layer) return;
        layer.hidden = true;
        layer.replaceChildren();
    }

    function setModalBusy(layer, busy) {
        layer.querySelectorAll('button,input,textarea,select').forEach(control => { control.disabled = busy; });
    }

    function modalError(layer, message) {
        const modal = layer.querySelector('.gm-modal');
        if (!modal) return;
        let error = layer.querySelector('.gm-modal-error');
        if (!error) {
            error = document.createElement('p');
            error.className = 'gm-modal-error';
            modal.appendChild(error);
        }
        error.textContent = message;
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
        updateStatus(state, 'failed');
    }

    function scrollConversation(state) {
        const log = state.container.querySelector('[data-gm-conversation]');
        if (log) log.scrollTop = log.scrollHeight;
    }

    function confirmAction(state, title, message) {
        if (typeof state.context.confirmDialog === 'function') {
            return Promise.resolve(state.context.confirmDialog(title, message));
        }
        return Promise.resolve(false);
    }

    function closeEvents(state) {
        if (state.eventSource) state.eventSource.close();
        state.eventSource = null;
        state.lastEventID = 0;
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        closeEvents(state);
        stopElapsed(state);
        if (window.GameMakerStudioPreview) window.GameMakerStudioPreview.clearLoading(state);
        if (state.busyPoll) clearInterval(state.busyPoll);
        if (state.moreDocListener) document.removeEventListener('click', state.moreDocListener);
        if (state.moreEscListener) document.removeEventListener('keydown', state.moreEscListener);
        if (state.previewListener) window.removeEventListener('message', state.previewListener);
        instances.delete(windowId);
    }

    const modalHelpers = { showModal, closeModal, setModalBusy, modalError, confirmAction };

    window.GameMakerStudioApp = { render, dispose, instances };
})();
