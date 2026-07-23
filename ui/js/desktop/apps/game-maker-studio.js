(function () {
    'use strict';

    const instances = new Map();
    const eventTypes = [
        'project_created', 'project_updated', 'job_status', 'phase', 'text_delta',
        'skill_activation', 'file_changed', 'asset_changed', 'preview_reload',
        'diagnostic', 'revision'
    ];
    const activeStatuses = new Set(['queued', 'planning', 'building', 'validating', 'polishing']);
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
            disposed: false
        };
        instances.set(windowId, state);
        container.innerHTML = shell(state);
        bindShell(state);
        window.addEventListener('message', state.previewListener = event => handlePreviewMessage(state, event));
        initialize(state);
    }

    function shell(state) {
        const { esc, t } = state.context;
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
                        <button type="button" data-gm-action="rename" disabled>${esc(t('game_maker.rename'))}</button>
                        <button type="button" data-gm-action="delete" disabled>${esc(t('game_maker.delete'))}</button>
                        <button type="button" data-gm-action="skills">${esc(t('game_maker.skills'))}</button>
                        <button type="button" data-gm-action="revisions" disabled>${esc(t('game_maker.revisions'))}</button>
                        <button type="button" data-gm-action="code" disabled>${esc(t('game_maker.open_code_studio'))}</button>
                        <button type="button" data-gm-action="export" disabled>${esc(t('game_maker.export_zip'))}</button>
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
                                <button type="button" class="gm-danger" data-gm-action="stop" hidden>${esc(t('game_maker.stop'))}</button>
                            </div>
                            <ol class="gm-phases" data-gm-phases>${phaseMarkup(state, '')}</ol>
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
                            </form>
                        </section>
                        <section class="gm-preview-pane">
                            <div class="gm-pane-head">
                                <div><span class="gm-kicker">${esc(t('game_maker.live_preview'))}</span><h2>${esc(t('game_maker.play_here'))}</h2></div>
                                <button type="button" data-gm-action="reload" disabled>${esc(t('game_maker.reload'))}</button>
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
            state.projects = projects.projects || [];
            applyCapabilities(state);
            renderProjects(state);
            if (state.projects.length) openProject(state, state.projects[0].id);
            else updateStatus(state, capabilities.enabled ? 'ready' : 'disabled');
        } catch (error) {
            fail(state, error);
        }
    }

    function bindShell(state) {
        const root = state.container.querySelector('.gm-studio');
        root.addEventListener('click', state.clickListener = event => {
            const button = event.target.closest('[data-gm-action]');
            if (!button) return;
            const actions = {
                new: () => showCreateModal(state),
                skills: () => showSkillsModal(state),
                revisions: () => showRevisionsModal(state),
                code: () => openInCodeStudio(state),
                export: () => exportProject(state),
                stop: () => stopJob(state),
                reload: () => refreshPreview(state),
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
        root.querySelector('[data-gm-change-form]').addEventListener('submit', event => {
            event.preventDefault();
            submitChange(state);
        });
    }

    function applyCapabilities(state) {
        const cap = state.capabilities || {};
        const newButton = state.container.querySelector('[data-gm-action="new"]');
        newButton.disabled = !cap.enabled || !cap.allow_create || !cap.skills_ready;
        if (!cap.enabled) showNotice(state, state.context.t('game_maker.disabled_notice'), 'warning');
        else if (!cap.skills_ready) showNotice(state, state.context.t('game_maker.skills_blocked_notice'), 'warning');
    }

    function renderProjects(state) {
        const { esc, t } = state.context;
        const list = state.container.querySelector('[data-gm-projects]');
        state.container.querySelector('[data-gm-count]').textContent = String(state.projects.length);
        if (!state.projects.length) {
            list.innerHTML = `<div class="gm-library-empty">${esc(t('game_maker.no_projects'))}</div>`;
            return;
        }
        list.innerHTML = state.projects.map(project => `
            <button type="button" class="gm-project-card ${state.project && state.project.id === project.id ? 'is-active' : ''}"
                data-project-id="${esc(project.id)}" role="listitem">
                <span class="gm-project-dimension">${esc(String(project.dimension).toUpperCase())}</span>
                <span><strong>${esc(project.name)}</strong><small>${esc(project.status || 'draft')}</small></span>
                <span class="gm-project-revision">v${Number(project.current_revision || 0)}</span>
            </button>`).join('');
    }

    async function openProject(state, projectID) {
        if (!projectID || state.disposed) return;
        closeEvents(state);
        try {
            const body = await state.api.getProject(projectID);
            if (state.disposed) return;
            state.project = body.project;
            state.messages = body.messages || [];
            state.job = null;
            state.diagnostics = [];
            renderProjects(state);
            renderProject(state);
            connectEvents(state);
            if (state.project.current_revision) refreshPreview(state);
        } catch (error) {
            fail(state, error);
        }
    }

    function renderProject(state) {
        const { esc, t } = state.context;
        const project = state.project;
        state.container.querySelector('[data-gm-title]').textContent = project.name;
        state.container.querySelector('[data-gm-engine]').textContent = project.dimension === '3d'
            ? `Three.js ${state.capabilities.three_version}` : `Phaser ${state.capabilities.phaser_version}`;
        state.container.querySelector('[data-gm-conversation]').innerHTML = state.messages.length
            ? state.messages.map(message => messageMarkup(state, message)).join('')
            : `<div class="gm-empty"><strong>${esc(t('game_maker.agent_ready'))}</strong><span>${esc(project.description)}</span></div>`;
        setButton(state, 'revisions', true);
        setButton(state, 'rename', Boolean(state.capabilities.allow_edit));
        setButton(state, 'delete', Boolean(state.capabilities.allow_delete));
        setButton(state, 'export', Boolean(project.current_revision));
        setButton(state, 'reload', Boolean(project.current_revision));
        setButton(state, 'code', Boolean(state.capabilities.code_studio && project.current_revision));
        syncJobControls(state);
        updateStatus(state, project.status || 'draft');
        scrollConversation(state);
    }

    function messageMarkup(state, message) {
        const { esc, t } = state.context;
        const role = message.role === 'assistant' ? 'assistant' : 'user';
        return `<article class="gm-message gm-message-${role}">
            <span>${esc(role === 'assistant' ? t('game_maker.agent') : t('game_maker.you'))}</span>
            <p>${esc(message.content)}</p>
        </article>`;
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
        source.onerror = () => updateStatus(state, 'reconnecting');
    }

    function parseEvent(event) {
        try { return JSON.parse(event.data); } catch (_) { return null; }
    }

    function handleEvent(state, event) {
        const payload = event.payload || {};
        switch (event.type) {
        case 'job_status':
            if (payload.job) state.job = payload.job;
            if (state.job && payload.status) state.job.status = payload.status;
            updateStatus(state, payload.status || 'working');
            syncJobControls(state);
            if (payload.status === 'ready' || payload.status === 'failed' || payload.status === 'cancelled') {
                state.container.querySelectorAll('.gm-message-assistant[data-streaming="true"]').forEach(message => {
                    message.dataset.streaming = 'false';
                });
                reloadProjectRecord(state);
            }
            break;
        case 'phase':
            if (!state.job) state.job = { id: event.job_id, status: payload.phase };
            state.job.phase = payload.phase;
            state.container.querySelector('[data-gm-phases]').innerHTML = phaseMarkup(state, payload.phase);
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
            reloadProjectRecord(state).then(() => refreshPreview(state));
            break;
        }
    }

    async function reloadProjectRecord(state) {
        if (!state.project) return;
        try {
            const body = await state.api.getProject(state.project.id);
            state.project = body.project;
            state.messages = body.messages || state.messages;
            const index = state.projects.findIndex(item => item.id === state.project.id);
            if (index >= 0) state.projects[index] = state.project;
            renderProjects(state);
            renderProject(state);
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
        if (!state.project || !state.project.current_revision) return;
        try {
            const grant = await state.api.previewGrant(state.project.id);
            if (state.disposed) return;
            const channelID = crypto.getRandomValues(new Uint32Array(4)).join('-');
            state.channelID = channelID;
            const frame = document.createElement('iframe');
            frame.className = 'gm-preview-frame';
            frame.title = state.context.t('game_maker.live_preview');
            frame.setAttribute('sandbox', 'allow-scripts');
            frame.setAttribute('referrerpolicy', 'no-referrer');
            frame.src = grant.url + '#gm-channel=' + encodeURIComponent(channelID);
            const shell = state.container.querySelector('[data-gm-preview]');
            shell.replaceChildren(frame);
            state.frame = frame;
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
        if (data.type !== 'ready') addDiagnostic(state, {
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
        if (diagnostic.level === 'error' || diagnostic.level === 'runtime') {
            state.container.querySelector('[data-gm-diagnostics]').open = true;
        }
    }

    function showCreateModal(state) {
        const cap = state.capabilities;
        if (!cap || !cap.allow_create) return;
        const { esc, t } = state.context;
        const providers = (cap.providers || []).map(provider =>
            `<option value="${esc(provider.id)}" data-model="${esc(provider.model || '')}"
                ${provider.id === cap.default_provider_id ? 'selected' : ''}>${esc(provider.name || provider.id)}</option>`
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
            form.addEventListener('submit', async event => {
                event.preventDefault();
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
                try {
                    setModalBusy(layer, true);
                    const project = await state.api.createProject(request);
                    closeModal(state);
                    state.projects.unshift(project);
                    await openProject(state, project.id);
                    state.job = await state.api.startJob(project.id, {
                        prompt: request.description,
                        provider_id: request.provider_id,
                        model: request.model,
                        image_generation: request.use_image_generation,
                        music_generation: request.use_music_generation
                    });
                    syncJobControls(state);
                } catch (error) {
                    setModalBusy(layer, false);
                    modalError(layer, error.message || String(error));
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
        try {
            state.container.querySelectorAll('.gm-message-assistant[data-streaming="true"]').forEach(message => {
                message.dataset.streaming = 'false';
            });
            state.job = await state.api.startJob(state.project.id, {
                prompt,
                provider_id: state.project.provider_id,
                model: state.project.model
            });
            state.messages.push({ role: 'user', content: prompt });
            state.container.querySelector('[data-gm-conversation]').insertAdjacentHTML('beforeend',
                messageMarkup(state, { role: 'user', content: prompt }));
            input.value = '';
            syncJobControls(state);
        } catch (error) {
            fail(state, error);
        }
    }

    async function stopJob(state) {
        if (!state.job || !activeStatuses.has(state.job.status)) return;
        try {
            await state.api.cancelJob(state.job.id);
            state.job.status = 'cancelling';
            syncJobControls(state);
        } catch (error) {
            fail(state, error);
        }
    }

    async function showSkillsModal(state) {
        const { esc, t } = state.context;
        const skills = (state.capabilities && state.capabilities.skills) || [];
        showModal(state, `<section class="gm-modal gm-skills-modal">
            <header><div><span>${esc(t('game_maker.curated'))}</span><h2>${esc(t('game_maker.skills_title'))}</h2></div>
                <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
            <div class="gm-skill-list">${skills.map(skill => `<article>
                <div><strong>${esc(skill.name)}</strong><span class="gm-skill-status is-${esc(skill.status)}">${esc(skill.status)}</span></div>
                <p>${esc(skill.description)}</p>
                <dl><dt>${esc(t('game_maker.source'))}</dt><dd>${esc(skill.source)}</dd>
                    <dt>${esc(t('game_maker.commit'))}</dt><dd>${esc(skill.commit)}</dd>
                    <dt>${esc(t('game_maker.license'))}</dt><dd>${esc(skill.license)}</dd></dl>
            </article>`).join('')}</div>
            <footer><button type="button" data-modal-close>${esc(t('game_maker.close'))}</button></footer>
        </section>`);
    }

    async function showRevisionsModal(state) {
        if (!state.project) return;
        const { esc, t } = state.context;
        try {
            const body = await state.api.revisions(state.project.id);
            showModal(state, `<section class="gm-modal gm-revisions-modal">
                <header><div><span>${esc(t('game_maker.history'))}</span><h2>${esc(t('game_maker.revisions_title'))}</h2></div>
                    <button type="button" data-modal-close aria-label="${esc(t('game_maker.close'))}">×</button></header>
                <div class="gm-revision-list">${(body.revisions || []).map(revision => `<article>
                    <div><strong>v${revision.number}</strong><span>${esc(revision.source)}</span></div>
                    <p>${esc(revision.summary)}</p><small>${revision.file_count} ${esc(t('game_maker.files'))}</small>
                    <button type="button" data-restore="${revision.number}"
                        ${revision.number === state.project.current_revision ? 'disabled' : ''}>${esc(t('game_maker.restore'))}</button>
                </article>`).join('') || `<div class="gm-library-empty">${esc(t('game_maker.no_revisions'))}</div>`}</div>
                <footer><button type="button" data-modal-close>${esc(t('game_maker.close'))}</button></footer>
            </section>`, layer => {
                layer.querySelectorAll('[data-restore]').forEach(button => button.addEventListener('click', async () => {
                    const confirmed = await confirmAction(state, t('game_maker.restore_title'), t('game_maker.restore_confirm'));
                    if (!confirmed) return;
                    try {
                        setModalBusy(layer, true);
                        await state.api.restore(state.project.id, Number(button.dataset.restore));
                        closeModal(state);
                        await reloadProjectRecord(state);
                        await refreshPreview(state);
                    } catch (error) {
                        setModalBusy(layer, false);
                        modalError(layer, error.message || String(error));
                    }
                }));
            });
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
            closeEvents(state);
            state.container.innerHTML = shell(state);
            bindShell(state);
            applyCapabilities(state);
            renderProjects(state);
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
        let error = layer.querySelector('.gm-modal-error');
        if (!error) {
            error = document.createElement('p');
            error.className = 'gm-modal-error';
            layer.querySelector('.gm-modal').appendChild(error);
        }
        error.textContent = message;
    }

    function syncJobControls(state) {
        const active = Boolean(state.job && activeStatuses.has(state.job.status));
        state.jobActive = active;
        const stop = state.container.querySelector('[data-gm-action="stop"]');
        stop.hidden = !active;
        stop.disabled = state.job && state.job.status === 'cancelling';
        const form = state.container.querySelector('[data-gm-change-form]');
        const editable = Boolean(state.project && state.project.current_revision && !active &&
            state.capabilities && state.capabilities.allow_edit);
        form.querySelector('textarea').disabled = !editable;
        form.querySelector('button').disabled = !editable;
    }

    function updateStatus(state, status) {
        const node = state.container.querySelector('[data-gm-status]');
        if (!node) return;
        const key = 'game_maker.status_' + String(status || 'ready').replaceAll('-', '_');
        node.className = 'gm-status is-' + String(status || 'ready');
        node.textContent = state.context.t(key);
    }

    function setButton(state, action, enabled) {
        const button = state.container.querySelector(`[data-gm-action="${action}"]`);
        if (button) button.disabled = !enabled;
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
        if (state.previewListener) window.removeEventListener('message', state.previewListener);
        instances.delete(windowId);
    }

    window.GameMakerStudioApp = { render, dispose, instances };
})();
