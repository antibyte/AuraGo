(function () {
    'use strict';

    const instances = new Map();

    function render(container, windowId, context) {
        dispose(windowId);

        const { esc, api, t, iconMarkup, notify } = context;

        const state = {
            chatBusy: false,
            abortCtrl: null,
            lastRole: null,
            target: 'local',
            previewUrl: '',
            statusLoaded: false,
            homepageEnabled: false,
            deploymentTargets: [],
            targetsLoading: null,
            disposed: false,
            activePanel: 'preview',
            historyQuery: '',
            historyFilter: '',
            historyEntries: [],
            historyAbortCtrl: null
        };
        instances.set(windowId, state);

        container.innerHTML = `
            <div class="vd-hp-studio">
                <aside class="vd-hp-chat" aria-label="${esc(t('homepage_studio.welcome_heading'))}">
                    <header class="vd-hp-chat-header">
                        <label class="vd-hp-chat-header-label" for="hp-target-${windowId}">${esc(t('homepage_studio.target_label'))}</label>
                        <select class="vd-hp-target-select" id="hp-target-${windowId}" aria-describedby="hp-status-${windowId}">
                            <option value="local">${esc(t('homepage_studio.target_local'))}</option>
                            <option value="vercel">${esc(t('homepage_studio.target_vercel'))}</option>
                            <option value="netlify">${esc(t('homepage_studio.target_netlify'))}</option>
                            <option value="remote">${esc(t('homepage_studio.target_remote'))}</option>
                        </select>
                        <span class="vd-hp-status-dot loading" id="hp-status-${windowId}" role="status" aria-live="polite" title="${esc(t('homepage_studio.checking_status'))}"></span>
                    </header>
                    <section class="vd-hp-chat-log" id="hp-log-${windowId}" aria-live="polite">
                        <div class="vd-hp-welcome">
                            <div class="vd-hp-welcome-icon" aria-hidden="true">🌐</div>
                            <h2 class="vd-hp-welcome-heading">${esc(t('homepage_studio.welcome_heading'))}</h2>
                            <p class="vd-hp-welcome-sub">${esc(t('homepage_studio.welcome'))}</p>
                        </div>
                    </section>
                    <form class="vd-hp-chat-form" id="hp-form-${windowId}">
                        <textarea class="vd-hp-chat-input" id="hp-input-${windowId}" rows="1" placeholder="${esc(t('homepage_studio.chat_placeholder'))}" autocomplete="off" enterkeyhint="send" aria-label="${esc(t('homepage_studio.chat_placeholder'))}"></textarea>
                        <button type="submit" class="vd-hp-send-btn" id="hp-send-${windowId}">
                            ${iconMarkup('chat', 'S', 'vd-hp-send-icon', 15)}
                            <span id="hp-send-label-${windowId}">${esc(t('desktop.send'))}</span>
                        </button>
                    </form>
                </aside>
                <main class="vd-hp-preview">
                    <header class="vd-hp-preview-header">
                        <div class="vd-hp-preview-tabs" role="tablist" aria-label="${esc(t('homepage_studio.preview_tabs'))}">
                            <button type="button" class="vd-hp-preview-tab is-active" id="hp-tab-preview-${windowId}" role="tab" aria-selected="true" aria-controls="hp-panel-preview-${windowId}">
                                ${esc(t('homepage_studio.preview_tab'))}
                            </button>
                            <button type="button" class="vd-hp-preview-tab" id="hp-tab-history-${windowId}" role="tab" aria-selected="false" aria-controls="hp-panel-history-${windowId}">
                                ${esc(t('homepage_studio.history_tab'))}
                            </button>
                        </div>
                        <div class="vd-hp-preview-url" id="hp-url-${windowId}" title="${esc(t('homepage_studio.no_url'))}">—</div>
                        <div class="vd-hp-preview-actions">
                            <button type="button" class="vd-hp-preview-btn" id="hp-refresh-${windowId}" title="${esc(t('homepage_studio.refresh_preview'))}">
                                ${iconMarkup('refresh', '↻', 'vd-hp-btn-icon', 14)}
                                <span>${esc(t('homepage_studio.refresh'))}</span>
                            </button>
                            <button type="button" class="vd-hp-preview-btn is-disabled" id="hp-external-${windowId}" disabled title="${esc(t('homepage_studio.open_external'))}" aria-label="${esc(t('homepage_studio.open_external'))}">
                                ${iconMarkup('external', '↗', 'vd-hp-btn-icon', 14)}
                            </button>
                        </div>
                    </header>
                    <section class="vd-hp-preview-body" id="hp-preview-body-${windowId}">
                        <div class="vd-hp-preview-panel is-active" id="hp-panel-preview-${windowId}" role="tabpanel" aria-labelledby="hp-tab-preview-${windowId}">
                            <div class="vd-hp-preview-placeholder" id="hp-placeholder-${windowId}">
                                <div class="vd-hp-preview-placeholder-icon" aria-hidden="true">🌐</div>
                                <h3 class="vd-hp-preview-placeholder-title">${esc(t('homepage_studio.preview_empty_title'))}</h3>
                                <p class="vd-hp-preview-placeholder-text">${esc(t('homepage_studio.preview_unavailable'))}</p>
                            </div>
                            <div class="vd-hp-preview-loading" id="hp-loading-${windowId}" aria-hidden="true">
                                <span class="vd-hp-preview-loading-label">${esc(t('homepage_studio.preview_loading'))}</span>
                                <div class="vd-hp-preview-skeleton" aria-hidden="true">
                                    <div class="vd-hp-skel-bar"></div>
                                    <div class="vd-hp-skel-hero"></div>
                                    <div class="vd-hp-skel-row"><span></span><span></span><span></span></div>
                                </div>
                            </div>
                        </div>
                        <div class="vd-hp-preview-panel vd-hp-history-panel" id="hp-panel-history-${windowId}" role="tabpanel" aria-labelledby="hp-tab-history-${windowId}" hidden>
                            <div class="vd-hp-history-controls">
                                <input type="search" class="vd-hp-history-search" id="hp-history-search-${windowId}" placeholder="${esc(t('homepage_studio.history_search_placeholder'))}" aria-label="${esc(t('homepage_studio.history_search_placeholder'))}">
                                <select class="vd-hp-history-filter" id="hp-history-filter-${windowId}" aria-label="${esc(t('homepage_studio.history_filter_label'))}">
                                    <option value="">${esc(t('homepage_studio.history_filter_all'))}</option>
                                    <option value="note">${esc(t('homepage_studio.history_filter_note'))}</option>
                                    <option value="decision">${esc(t('homepage_studio.history_filter_decision'))}</option>
                                    <option value="milestone">${esc(t('homepage_studio.history_filter_milestone'))}</option>
                                    <option value="feedback">${esc(t('homepage_studio.history_filter_feedback'))}</option>
                                    <option value="question">${esc(t('homepage_studio.history_filter_question'))}</option>
                                    <option value="observation">${esc(t('homepage_studio.history_filter_observation'))}</option>
                                </select>
                                <button type="button" class="vd-hp-history-refresh" id="hp-history-refresh-${windowId}" title="${esc(t('homepage_studio.refresh'))}">
                                    ${iconMarkup('refresh', '↻', 'vd-hp-btn-icon', 14)}
                                </button>
                            </div>
                            <div class="vd-hp-history-list" id="hp-history-list-${windowId}">
                                <div class="vd-hp-history-empty">${esc(t('homepage_studio.history_loading'))}</div>
                            </div>
                        </div>
                    </section>
                </main>
            </div>
        `;

        const $ = id => container.querySelector('#' + id);
        const chatLog = $(`hp-log-${windowId}`);
        const chatInput = $(`hp-input-${windowId}`);
        const chatForm = $(`hp-form-${windowId}`);
        const sendBtn = $(`hp-send-${windowId}`);
        const sendLabel = $(`hp-send-label-${windowId}`);
        const targetSelect = $(`hp-target-${windowId}`);
        const statusDot = $(`hp-status-${windowId}`);
        const previewUrl = $(`hp-url-${windowId}`);
        const previewBody = $(`hp-preview-body-${windowId}`);
        const previewPlaceholder = $(`hp-placeholder-${windowId}`);
        const previewLoading = $(`hp-loading-${windowId}`);
        const refreshBtn = $(`hp-refresh-${windowId}`);
        const externalBtn = $(`hp-external-${windowId}`);
        const previewTab = $(`hp-tab-preview-${windowId}`);
        const historyTab = $(`hp-tab-history-${windowId}`);
        const previewPanel = $(`hp-panel-preview-${windowId}`);
        const historyPanel = $(`hp-panel-history-${windowId}`);
        const historySearch = $(`hp-history-search-${windowId}`);
        const historyFilter = $(`hp-history-filter-${windowId}`);
        const historyRefresh = $(`hp-history-refresh-${windowId}`);
        const historyList = $(`hp-history-list-${windowId}`);

        autoResizeTextarea(chatInput);

        chatInput.addEventListener('input', () => autoResizeTextarea(chatInput));
        chatInput.addEventListener('keydown', e => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                chatForm.dispatchEvent(new Event('submit', { cancelable: true }));
            }
        });

        chatForm.addEventListener('submit', e => {
            e.preventDefault();
            if (state.chatBusy) {
                if (state.abortCtrl) state.abortCtrl.abort();
                return;
            }
            const msg = chatInput.value.trim();
            if (msg) sendMessage(msg);
        });

        targetSelect.addEventListener('change', () => {
            state.target = targetSelect.value;
            loadStatus();
        });

        refreshBtn.addEventListener('click', () => refreshPreview());
        externalBtn.addEventListener('click', () => {
            if (state.previewUrl) window.open(state.previewUrl, '_blank');
        });

        previewTab.addEventListener('click', () => switchPanel('preview'));
        historyTab.addEventListener('click', () => {
            switchPanel('history');
            loadHistory();
        });
        historySearch.addEventListener('input', debounce(() => {
            state.historyQuery = historySearch.value.trim();
            loadHistory();
        }, 250));
        historyFilter.addEventListener('change', () => {
            state.historyFilter = historyFilter.value;
            loadHistory();
        });
        historyRefresh.addEventListener('click', () => loadHistory());

        loadStatus();

        function autoResizeTextarea(el) {
            if (!el) return;
            el.style.height = 'auto';
            const max = 120;
            el.style.height = Math.min(el.scrollHeight, max) + 'px';
            el.style.overflowY = el.scrollHeight > max ? 'auto' : 'hidden';
        }

        async function loadStatus() {
            statusDot.className = 'vd-hp-status-dot loading';
            statusDot.title = t('homepage_studio.checking_status');
            try {
                const data = await api('/api/homepage/status');
                state.statusLoaded = true;
                state.homepageEnabled = data && data.status !== 'disabled';

                if (!state.homepageEnabled) {
                    statusDot.className = 'vd-hp-status-dot offline';
                    statusDot.title = t('homepage_studio.status_disabled');
                    state.previewUrl = '';
                    updatePreviewUrl();
                    return;
                }

                const webRunning = data && data.web_container && data.web_container.running;
                const pythonRunning = data && data.python_server && data.python_server.running;
                const serverRunning = webRunning || pythonRunning;
                statusDot.className = serverRunning ? 'vd-hp-status-dot online' : 'vd-hp-status-dot offline';
                statusDot.title = serverRunning
                    ? t('homepage_studio.status_online')
                    : t('homepage_studio.status_offline');

                state.previewUrl = homepageStatusPreviewURL(data, state.target);
                updatePreviewUrl();
                refreshHomepageTargets(data);
            } catch (_) {
                statusDot.className = 'vd-hp-status-dot offline';
                statusDot.title = t('homepage_studio.status_error');
                state.previewUrl = '';
                updatePreviewUrl();
            }
        }

        function refreshHomepageTargets(statusData) {
            loadHomepageTargets().then(() => {
                if (state.disposed) return;
                const nextURL = homepageStatusPreviewURL(statusData, state.target);
                if (nextURL !== state.previewUrl) {
                    state.previewUrl = nextURL;
                    updatePreviewUrl();
                }
            });
        }

        function firstString(...values) {
            for (const value of values) {
                if (typeof value === 'string' && value.trim()) {
                    return value.trim();
                }
            }
            return '';
        }

        function firstPreviewURL(...values) {
            for (const value of values) {
                const url = firstString(value);
                if (url && /^https?:\/\//i.test(url)) {
                    return url;
                }
            }
            return '';
        }

        async function loadHomepageTargets() {
            if (state.targetsLoading) return state.targetsLoading;
            state.targetsLoading = (async () => {
                const nextTargets = [];
                const [sitesData, webhostsData] = await Promise.all([
                    safeHomepageApi('/api/homepage/sites'),
                    safeHomepageApi('/api/integrations/webhosts')
                ]);

                const webhosts = Array.isArray(webhostsData && webhostsData.webhosts) ? webhostsData.webhosts : [];
                for (const item of webhosts) {
                    if (item && item.id === 'homepage') {
                        addHomepageTarget(nextTargets, 'local', item.url, item.name || 'Homepage');
                    }
                }

                const sites = Array.isArray(sitesData && sitesData.sites) ? sitesData.sites : [];
                for (const site of sites) {
                    collectHomepageTargetsFromSite(site, nextTargets);
                }

                await Promise.all(sites.map(async site => {
                    const id = Number(site && site.id);
                    if (!Number.isFinite(id) || id <= 0) return;
                    const detail = await safeHomepageApi('/api/homepage/sites/' + encodeURIComponent(String(id)));
                    collectHomepageTargetsFromSite(detail && detail.site, nextTargets);
                }));

                state.deploymentTargets = nextTargets;
                return nextTargets;
            })().catch(() => {
                state.deploymentTargets = [];
                return state.deploymentTargets;
            }).finally(() => {
                state.targetsLoading = null;
            });
            return state.targetsLoading;
        }

        async function safeHomepageApi(path) {
            try {
                return await api(path);
            } catch (_) {
                return null;
            }
        }

        function collectHomepageTargetsFromSite(site, targets) {
            if (!site || typeof site !== 'object') return;
            addHomepageTarget(targets, 'remote', site.last_deploy_url, site.name || site.project_dir);
            const deployTargets = Array.isArray(site.deploy_targets) ? site.deploy_targets : [];
            for (const target of deployTargets) {
                const provider = normalizeHomepageTargetProvider(target && target.provider);
                const label = firstString(target && target.provider_target_id, site.name, site.project_dir, provider);
                const url = firstPreviewURL(target && target.url, target && target.remote_path);
                addHomepageTarget(targets, provider, url, label);
            }
            const deployments = Array.isArray(site.deployments) ? site.deployments : [];
            for (const deployment of deployments) {
                const provider = normalizeHomepageTargetProvider(deployment && deployment.provider);
                const label = firstString(deployment && deployment.provider_deploy_id, site.name, site.project_dir, provider);
                addHomepageTarget(targets, provider, deployment && deployment.url, label);
            }
            const observations = Array.isArray(site.remote_observations) ? site.remote_observations : [];
            for (const observation of observations) {
                const provider = normalizeHomepageTargetProvider(observation && observation.provider);
                const label = firstString(observation && observation.provider_deploy_id, site.name, site.project_dir, provider);
                addHomepageTarget(targets, provider, observation && observation.url, label);
            }
        }

        function addHomepageTarget(targets, provider, url, label) {
            const normalizedProvider = normalizeHomepageTargetProvider(provider);
            const normalizedURL = firstPreviewURL(url);
            if (!normalizedProvider || !normalizedURL) return;
            const key = normalizedProvider + '\n' + normalizedURL;
            if (targets.some(item => item.key === key)) return;
            targets.push({
                key,
                provider: normalizedProvider,
                url: normalizedURL,
                label: firstString(label, normalizedProvider)
            });
        }

        function normalizeHomepageTargetProvider(provider) {
            const value = firstString(provider).toLowerCase();
            if (value === 'homepage') return 'local';
            if (value === 'sftp' || value === 'scp' || value === 'ssh') return 'remote';
            return value || 'remote';
        }

        function homepageExternalTargetURL(target, deploymentTargets) {
            if (!Array.isArray(deploymentTargets) || !deploymentTargets.length) return '';
            const selected = normalizeHomepageTargetProvider(target);
            const aliases = {
                local: ['local', 'homepage'],
                vercel: ['vercel'],
                netlify: ['netlify'],
                remote: ['remote', 'sftp', 'scp', 'ssh']
            };
            const allowed = aliases[selected] || [selected];
            const exact = deploymentTargets.find(item => item && allowed.includes(item.provider) && item.url);
            if (exact) return exact.url;
            const externalTargets = ['remote', 'vercel', 'netlify'];
            if (externalTargets.includes(selected)) {
                const fallback = deploymentTargets.find(item => item && item.provider !== 'local' && item.url);
                if (fallback) return fallback.url;
            }
            return '';
        }

        function homepageStatusPreviewURL(data, target) {
            if (!data) return '';
            const webRunning = data.web_container && data.web_container.running;
            const pythonRunning = data.python_server && data.python_server.running;
            const serverRunning = webRunning || pythonRunning;
            const externalURL = homepageExternalTargetURL(target, state.deploymentTargets);

            const objectURL = key => {
                const obj = data[key];
                if (!obj || typeof obj !== 'object') return '';
                return firstString(obj.preview_url, obj.url, obj.deployment_url, obj.deploy_url, obj.browser_url);
            };

            switch (target) {
                case 'vercel':
                    return firstString(data.vercel_url, data.vercel_deployment_url, data.deployment_url, objectURL('vercel'), externalURL);
                case 'netlify':
                    return firstString(data.netlify_url, data.netlify_deploy_url, data.deploy_url, objectURL('netlify'), externalURL);
                case 'remote':
                    return firstString(data.remote_url, data.remote_deploy_url, objectURL('remote'), externalURL);
                case 'local':
                default:
                    break;
            }

            if (data.preview_url) return String(data.preview_url);
            if (serverRunning && data.tunnel_url) return String(data.tunnel_url);
            if (webRunning && data.web_container.browser_url) {
                return String(data.web_container.browser_url);
            }
            if (pythonRunning && data.python_server.browser_url) {
                return String(data.python_server.browser_url);
            }
            if (externalURL) return externalURL;
            return '';
        }

        function updatePreviewUrl() {
            const hasUrl = !!state.previewUrl;
            externalBtn.disabled = !hasUrl;
            externalBtn.classList.toggle('is-disabled', !hasUrl);
            if (hasUrl) {
                previewUrl.textContent = state.previewUrl;
                previewUrl.title = state.previewUrl;
                showPreview(state.previewUrl);
            } else {
                previewUrl.textContent = '—';
                previewUrl.title = t('homepage_studio.no_url');
                hidePreview();
            }
        }

        function showPreview(url) {
            let iframe = previewBody.querySelector('.vd-hp-preview-iframe');
            if (!iframe) {
                previewPlaceholder.style.display = 'none';
                iframe = document.createElement('iframe');
                iframe.className = 'vd-hp-preview-iframe';
                iframe.sandbox = 'allow-scripts allow-forms';
                iframe.referrerPolicy = 'no-referrer';
                previewPanel.insertBefore(iframe, previewLoading);
            }
            if (iframe.src !== url) {
                previewLoading.classList.add('active');
                previewLoading.setAttribute('aria-hidden', 'false');
                iframe.onload = () => {
                    previewLoading.classList.remove('active');
                    previewLoading.setAttribute('aria-hidden', 'true');
                };
                iframe.onerror = () => {
                    previewLoading.classList.remove('active');
                    previewLoading.setAttribute('aria-hidden', 'true');
                };
                iframe.src = url;
            }
        }

        function hidePreview() {
            const iframe = previewBody.querySelector('.vd-hp-preview-iframe');
            if (iframe) iframe.remove();
            previewPlaceholder.style.display = '';
        }

        function refreshPreview() {
            if (!state.previewUrl) {
                loadStatus();
                return;
            }
            previewLoading.classList.add('active');
            previewLoading.setAttribute('aria-hidden', 'false');
            const iframe = previewBody.querySelector('.vd-hp-preview-iframe');
            if (iframe) {
                iframe.onload = () => {
                    previewLoading.classList.remove('active');
                    previewLoading.setAttribute('aria-hidden', 'true');
                };
                iframe.src = state.previewUrl;
            } else {
                showPreview(state.previewUrl);
            }
        }

        function switchPanel(panel) {
            state.activePanel = panel;
            if (panel === 'preview') {
                previewTab.classList.add('is-active');
                previewTab.setAttribute('aria-selected', 'true');
                historyTab.classList.remove('is-active');
                historyTab.setAttribute('aria-selected', 'false');
                previewPanel.classList.add('is-active');
                previewPanel.removeAttribute('hidden');
                historyPanel.classList.remove('is-active');
                historyPanel.setAttribute('hidden', '');
            } else {
                historyTab.classList.add('is-active');
                historyTab.setAttribute('aria-selected', 'true');
                previewTab.classList.remove('is-active');
                previewTab.setAttribute('aria-selected', 'false');
                historyPanel.classList.add('is-active');
                historyPanel.removeAttribute('hidden');
                previewPanel.classList.remove('is-active');
                previewPanel.setAttribute('hidden', '');
            }
        }

        async function loadHistory() {
            if (!state.homepageEnabled) {
                renderHistory([], t('homepage_studio.history_disabled'));
                return;
            }
            if (state.historyAbortCtrl) {
                state.historyAbortCtrl.abort();
            }
            const abortCtrl = new AbortController();
            state.historyAbortCtrl = abortCtrl;
            try {
                const params = new URLSearchParams();
                if (state.historyQuery) params.set('q', state.historyQuery);
                if (state.historyFilter) params.set('entry_type', state.historyFilter);
                params.set('limit', '100');
                const url = '/api/homepage/history' + (params.toString() ? '?' + params.toString() : '');
                const data = await api(url, { signal: abortCtrl.signal });
                if (data && data.status === 'success') {
                    state.historyEntries = data.entries || [];
                    renderHistory(state.historyEntries);
                } else {
                    renderHistory([], data && data.message ? data.message : t('homepage_studio.history_error'));
                }
            } catch (err) {
                if (err.name === 'AbortError') return;
                renderHistory([], t('homepage_studio.history_error'));
            } finally {
                if (state.historyAbortCtrl === abortCtrl) {
                    state.historyAbortCtrl = null;
                }
            }
        }

        function renderHistory(entries, emptyMessage) {
            if (!historyList) return;
            if (entries.length === 0) {
                historyList.innerHTML = `<div class="vd-hp-history-empty">${esc(emptyMessage || t('homepage_studio.history_empty'))}</div>`;
                return;
            }
            const typeLabel = type => t('homepage_studio.history_type_' + type, type);
            const html = entries.map(e => {
                const date = e.created_at ? new Date(e.created_at).toLocaleString() : '';
                const type = esc(e.entry_type || 'note');
                const content = esc(e.content || '');
                const source = e.source ? `<span class="vd-hp-history-source">${esc(e.source)}</span>` : '';
                const tags = (e.tags || []).map(tag => `<span class="vd-hp-history-tag">${esc(tag)}</span>`).join('');
                const id = esc(String(e.id || ''));
                return `
                    <article class="vd-hp-history-entry vd-hp-history-type-${type}">
                        <header class="vd-hp-history-entry-header">
                            <span class="vd-hp-history-entry-type">${typeLabel(type)}</span>
                            <time class="vd-hp-history-entry-time" datetime="${esc(e.created_at || '')}">${esc(date)}</time>
                            <button type="button" class="vd-hp-history-delete" data-id="${id}" title="${esc(t('homepage_studio.history_delete'))}" aria-label="${esc(t('homepage_studio.history_delete'))}">×</button>
                        </header>
                        <p class="vd-hp-history-entry-content">${content}</p>
                        <footer class="vd-hp-history-entry-footer">${source}${tags}</footer>
                    </article>
                `;
            }).join('');
            historyList.innerHTML = html;
            historyList.querySelectorAll('.vd-hp-history-delete').forEach(btn => {
                btn.addEventListener('click', async () => {
                    const id = btn.getAttribute('data-id');
                    if (!id) return;
                    if (!confirm(t('homepage_studio.history_delete_confirm'))) return;
                    try {
                        await api('/api/homepage/history?id=' + encodeURIComponent(id), { method: 'DELETE' });
                        loadHistory();
                    } catch (err) {
                        notify(t('homepage_studio.history_delete_error'));
                    }
                });
            });
        }

        function debounce(fn, ms) {
            let t;
            return function (...args) {
                clearTimeout(t);
                t = setTimeout(() => fn.apply(this, args), ms);
            };
        }

        function homepageWindowContext() {
            return {
                source: 'homepage-studio',
                app_id: 'homepage-studio',
                window_id: windowId,
                label: t('homepage_studio.welcome_heading'),
                purpose: 'Homepage Studio edits AuraGo homepage websites and pages in the managed homepage workspace.',
                guide: 'Use homepage_project, homepage_file, homepage_quality, homepage_deploy, and homepage_git. Do not use virtual_desktop apps, widgets, or files for Homepage Studio site changes.',
                resources: [{
                    kind: 'homepage_target',
                    label: state.target,
                    path: state.target
                }]
            };
        }

        async function sendMessage(message) {
            chatInput.value = '';
            autoResizeTextarea(chatInput);
            state.chatBusy = true;
            setBusy(true);

            const welcome = chatLog.querySelector('.vd-hp-welcome');
            if (welcome) welcome.remove();

            appendBubble('user', message);

            const renderer = window.DesktopChatRenderer;
            const statusEl = renderer ? renderer.createThinkingStatus() : null;
            if (statusEl) chatLog.appendChild(statusEl);

            let streamingBubble = null;
            let streamingContent = '';
            let streamTextFrame = 0;
            let finalized = false;

            function flushStreamingBubble() {
                streamTextFrame = 0;
                if (!streamingBubble || !streamingBubble.classList.contains('vd-streaming')) return;
                streamingBubble.textContent = streamingContent;
                scrollToEnd();
            }

            function queueFlush() {
                if (streamTextFrame) return;
                const schedule = window.requestAnimationFrame || (cb => window.setTimeout(cb, 16));
                streamTextFrame = schedule(flushStreamingBubble);
            }

            function scrollToEnd() {
                chatLog.scrollTop = chatLog.scrollHeight;
            }

            function doFinalize() {
                if (finalized) return;
                finalized = true;
                if (streamTextFrame) {
                    (window.cancelAnimationFrame || window.clearTimeout)(streamTextFrame);
                    streamTextFrame = 0;
                }
                if (statusEl && statusEl.parentNode) statusEl.remove();
                if (streamingBubble) {
                    streamingBubble.classList.remove('vd-streaming');
                    if (renderer && streamingContent.trim()) {
                        streamingBubble.innerHTML = renderer.renderMarkdown(streamingContent);
                        renderer.processImages(streamingBubble);
                        renderer.enhanceCodeBlocks(streamingBubble);
                        if (window.MermaidLoader) window.MermaidLoader.processBlocks(streamingBubble);
                    }
                }
                state.chatBusy = false;
                state.abortCtrl = null;
                setBusy(false);
                scrollToEnd();
                refreshPreview();
                loadHistory();

                if (streamingContent.trim()) {
                    setTimeout(() => refreshPreview(), 500);
                }
            }

            function doReject(err) {
                if (finalized) return;
                finalized = true;
                if (streamTextFrame) {
                    (window.cancelAnimationFrame || window.clearTimeout)(streamTextFrame);
                    streamTextFrame = 0;
                }
                if (statusEl && statusEl.parentNode) statusEl.remove();
                const msg = (err && err.message) || String(err || 'Request failed');
                appendBubble('agent', msg);
                state.chatBusy = false;
                state.abortCtrl = null;
                setBusy(false);
            }

            const ctrl = new AbortController();
            state.abortCtrl = ctrl;

            const chatContext = {
                source: 'homepage-studio',
                target: state.target,
                homepage_mode: true,
                window_context: homepageWindowContext()
            };

            try {
                const response = await fetch('/api/desktop/chat/stream', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ message, context: chatContext }),
                    signal: ctrl.signal
                });

                if (!response.ok) {
                    const text = await response.text();
                    throw new Error(text || ('HTTP ' + response.status));
                }

                const parser = window.AuraChatStreamParser;
                if (!parser) throw new Error('Chat stream parser not loaded');

                await parser.readFetchEventStream(response, {
                    onEvent: data => {
                        if (!data) return;
                        const event = data.event || data.type;

                        if (event === 'llm_stream_delta') {
                            const content = data.content || '';
                            if (!content) return;
                            if (!streamingBubble) {
                                streamingBubble = document.createElement('div');
                                streamingBubble.className = 'vd-chat-bubble agent vd-streaming';
                                chatLog.appendChild(streamingBubble);
                                if (renderer) renderer.appendTimestamp(chatLog, 'agent');
                                state.lastRole = 'agent';
                            }
                            streamingContent += content;
                            if (streamingBubble.classList.contains('vd-streaming')) queueFlush();
                        } else if (event === 'thinking_block') {
                            if (statusEl && (data.state === 'start') && renderer) {
                                renderer.updateStatus(statusEl, t('desktop.chat_thinking'));
                            }
                        } else if (event === 'thinking' || event === 'tool_start' || event === 'tool_end' ||
                            event === 'co_agent_spawn' || event === 'workflow_plan' || event === 'coding' ||
                            event === 'error_recovery' || event === 'agent_action') {
                            if (statusEl && renderer) {
                                const status = renderer.formatAgentActionStatus(data);
                                if (status) renderer.updateStatus(statusEl, status);
                            }
                        } else if (event === 'tool_call') {
                            if (renderer) {
                                const text = renderer.extractToolCallNarration(data.detail || data.message || '');
                                if (text) {
                                    appendBubble('agent', text);
                                }
                            }
                        } else if (event === 'final_response') {
                            if (data.detail || data.message) {
                                const text = data.detail || data.message || '';
                                if (!streamingBubble && text.trim()) {
                                    appendBubble('agent', text);
                                } else if (streamingBubble && !streamingContent.trim() && text.trim()) {
                                    streamingContent = text;
                                    flushStreamingBubble();
                                }
                            }
                        } else if (event === 'done') {
                            doFinalize();
                        }
                    },
                    onDone: () => doFinalize(),
                    onError: err => doReject(err)
                });
            } catch (err) {
                if (err.name === 'AbortError') {
                    doFinalize();
                } else {
                    doReject(err);
                }
            }
        }

        function appendBubble(role, text) {
            const renderer = window.DesktopChatRenderer;
            if (renderer) {
                renderer.appendRichBubble(chatLog, role, text, state.lastRole);
            } else {
                const bubble = document.createElement('div');
                bubble.className = 'vd-chat-bubble ' + role;
                bubble.textContent = text;
                chatLog.appendChild(bubble);
            }
            state.lastRole = role;
            chatLog.scrollTop = chatLog.scrollHeight;
        }

        function setBusy(busy) {
            chatInput.disabled = !!busy;
            sendBtn.classList.toggle('is-stop', !!busy);
            const sendText = t('desktop.send');
            const stopText = t('desktop.chat_stop');
            sendLabel.textContent = busy ? stopText : sendText;
            sendBtn.title = busy ? stopText : sendText;
        }
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        state.disposed = true;
        if (state.abortCtrl) { state.abortCtrl.abort(); state.abortCtrl = null; }
        if (state.historyAbortCtrl) { state.historyAbortCtrl.abort(); state.historyAbortCtrl = null; }
        instances.delete(windowId);
    }

    window.HomepageStudioApp = { render, dispose };
})();
