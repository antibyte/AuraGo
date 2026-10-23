/* Homepage Studio workbench — preview-first website builder.
   Grid: [chat | splitter | preview | splitter | inspector(Sites|History)].
   URL/sandbox-sensitive helpers stay in this module (security-lint pinned);
   preview chrome, sites and history live in homepage-studio-{preview,sites,history}.js */
(function () {
    'use strict';

    const instances = new Map();

    function render(container, windowId, context) {
        dispose(windowId);

        const { esc, api, t, iconMarkup, notify, confirmDialog } = context;
        const readonly = !!context.readonly;

        /* t(k, p) treats its second argument as a placeholder map, so a string
           fallback never applies. tr() compares against the raw key instead. */
        const tr = (key, fallback) => {
            const value = t(key);
            return value === key ? fallback : value;
        };

        /* Deployment target collection lives in the sites panel module. */
        const homepageTargets = (window.HomepageStudioSites && window.HomepageStudioSites.targets) || {
            collect: () => {},
            add: () => {},
            external: () => ''
        };

        /* Dates follow the app language, not the browser language. */
        const appLocale = window.SYSTEM_LANG || document.documentElement.lang || 'en';
        const DEFAULT_CHAT_WIDTH = 320;
        const DEFAULT_INSPECTOR_WIDTH = 300;
        const MIN_PANEL_WIDTH = 240;
        const MAX_PANEL_WIDTH = 560;

        const state = {
            chatBusy: false,
            abortCtrl: null,
            lastRole: null,
            target: 'local',
            previewUrl: '',
            statusLoaded: false,
            lastStatus: null,
            homepageEnabled: false,
            deploymentTargets: [],
            targetsLoading: null,
            disposed: false,
            device: 'desktop',
            inspectorTab: 'sites',
            chatCollapsed: false, inspectorCollapsed: false,
            chatWidth: 0, inspectorWidth: 0,
            selectedSiteId: 0, selectedSiteName: '', busyStart: 0,
            busyTimer: null, persistTimer: null, refreshTimer: null,
            listeners: [],
            modules: { preview: null, sites: null, history: null }
        };        instances.set(windowId, state);

        const suggestions = [
            { key: 'suggest_landing', icon: 'globe' },
            { key: 'suggest_redesign', icon: 'sliders' },
            { key: 'suggest_section', icon: 'file-plus' },
            { key: 'suggest_publish', icon: 'run' }
        ];
        const historyTypes = ['', 'note', 'decision', 'milestone', 'feedback', 'question', 'observation'];

        container.innerHTML = `
            <div class="vd-hp-studio" id="hp-root-${windowId}">
                <header class="vd-hp-header">
                    <div class="vd-hp-brand">
                        <span class="vd-hp-brand-icon">${iconMarkup('globe', 'H', '', 18)}</span>
                        <div class="vd-hp-brand-text">
                            <h2>${esc(t('homepage_studio.welcome_heading'))}</h2>
                            <p>${esc(t('homepage_studio.brand_subtitle'))}</p>
                        </div>
                    </div>
                    <div class="vd-hp-header-status">
                        <span class="vd-hp-status-pill" id="hp-status-pill-${windowId}" role="status" aria-live="polite">
                            <span class="vd-hp-status-dot loading" id="hp-status-dot-${windowId}"></span>
                            <span class="vd-hp-status-text" id="hp-status-text-${windowId}">${esc(t('homepage_studio.checking_status'))}</span>
                        </span>
                        <label class="vd-hp-target-wrap" for="hp-target-${windowId}">
                            <span class="vd-hp-target-label">${esc(t('homepage_studio.target_label'))}</span>
                            <select class="vd-hp-target-select" id="hp-target-${windowId}">
                                <option value="local">${esc(t('homepage_studio.target_local'))}</option>
                                <option value="vercel">${esc(t('homepage_studio.target_vercel'))}</option>
                                <option value="netlify">${esc(t('homepage_studio.target_netlify'))}</option>
                                <option value="here_now">${esc(t('homepage_studio.target_here_now'))}</option>
                                <option value="remote">${esc(t('homepage_studio.target_remote'))}</option>
                            </select>
                        </label>
                    </div>
                    <div class="vd-hp-header-actions">
                        <button type="button" class="vd-hp-icon-btn active" id="hp-toggle-chat-${windowId}" data-hp-toggle="chat" title="${esc(t('homepage_studio.assistant_toggle'))}" aria-label="${esc(t('homepage_studio.assistant_toggle'))}" aria-expanded="true">
                            ${iconMarkup('chat', 'A', 'vd-hp-btn-icon', 15)}
                        </button>
                        <button type="button" class="vd-hp-icon-btn active" id="hp-toggle-inspector-${windowId}" data-hp-toggle="inspector" title="${esc(t('homepage_studio.inspector_toggle'))}" aria-label="${esc(t('homepage_studio.inspector_toggle'))}" aria-expanded="true">
                            ${iconMarkup('sliders', 'I', 'vd-hp-btn-icon', 15)}
                        </button>
                    </div>
                </header>
                <div class="vd-hp-main" id="hp-main-${windowId}">
                    <aside class="vd-hp-chat" id="hp-chat-${windowId}" aria-label="${esc(t('homepage_studio.assistant_label'))}">
                        <div class="vd-hp-chat-head">
                            <span class="vd-hp-panel-label">${esc(t('homepage_studio.assistant_label'))}</span>
                            <button type="button" class="vd-hp-icon-btn" id="hp-collapse-chat-${windowId}" title="${esc(t('homepage_studio.collapse_panel'))}" aria-label="${esc(t('homepage_studio.collapse_panel'))}">
                                ${iconMarkup('chevron-left', '‹', 'vd-hp-btn-icon', 14)}
                            </button>
                        </div>
                        <section class="vd-hp-chat-log" id="hp-log-${windowId}" aria-busy="false">
                            <div class="vd-hp-welcome" id="hp-welcome-${windowId}">
                                <span class="vd-hp-welcome-icon" aria-hidden="true">${iconMarkup('globe', '🌐', '', 30)}</span>
                                <p class="vd-hp-welcome-heading">${esc(t('homepage_studio.welcome_heading'))}</p>
                                <p class="vd-hp-welcome-sub">${esc(t('homepage_studio.welcome'))}</p>
                                <div class="vd-hp-chips" role="group" aria-label="${esc(t('homepage_studio.suggest_label'))}">
                                    ${suggestions.map(item => `<button type="button" class="vd-hp-chip" data-hp-suggest="${item.key}"${readonly ? ' disabled' : ''}>${iconMarkup(item.icon, '•', 'vd-hp-btn-icon', 13)}<span>${esc(t('homepage_studio.' + item.key))}</span></button>`).join('')}
                                </div>
                            </div>
                        </section>
                        <div class="vd-hp-chat-announce visually-hidden" id="hp-announce-${windowId}" role="status" aria-live="polite" aria-atomic="true"></div>
                        ${readonly ? `<div class="vd-hp-readonly-hint" id="hp-readonly-hint-${windowId}" role="note">${esc(t('homepage_studio.readonly_hint'))}</div>` : ''}
                        <form class="vd-hp-chat-form" id="hp-form-${windowId}">
                            <textarea class="vd-hp-chat-input" id="hp-input-${windowId}" rows="1" placeholder="${esc(t(readonly ? 'homepage_studio.readonly_placeholder' : 'homepage_studio.chat_placeholder'))}" autocomplete="off" enterkeyhint="send" aria-label="${esc(t('homepage_studio.chat_placeholder'))}"${readonly ? ' disabled' : ''}></textarea>
                            <button type="submit" class="vd-hp-send-btn" id="hp-send-${windowId}"${readonly ? ' disabled' : ''}>${iconMarkup('chat', 'S', 'vd-hp-send-icon', 15)}<span id="hp-send-label-${windowId}">${esc(t('desktop.send'))}</span></button>
                        </form>
                        <div class="vd-hp-agent-row" id="hp-agent-row-${windowId}">
                            <span class="vd-hp-agent-pill" id="hp-agent-pill-${windowId}" role="status" hidden>
                                <span class="vd-hp-agent-spinner" aria-hidden="true"></span>
                                <span>${esc(t('homepage_studio.agent_working'))}</span>
                                <span class="vd-hp-agent-elapsed" id="hp-agent-elapsed-${windowId}" aria-hidden="true"></span>
                            </span>
                        </div>
                    </aside>
                    <div class="vd-hp-splitter" data-hp-splitter="chat" role="separator" aria-orientation="vertical" tabindex="0" aria-label="${esc(t('homepage_studio.resize_assistant'))}" aria-controls="hp-chat-${windowId}" aria-valuemin="${MIN_PANEL_WIDTH}" aria-valuemax="${MAX_PANEL_WIDTH}" aria-valuenow="${DEFAULT_CHAT_WIDTH}"></div>
                    <main class="vd-hp-preview-zone" id="hp-preview-zone-${windowId}">
                        <div class="vd-hp-stage" data-hp-stage data-device="desktop">
                            <div class="vd-hp-preview-panel" id="hp-panel-preview-${windowId}">
                                <div class="vd-hp-preview-frame" id="hp-frame-${windowId}"></div>
                                <div class="vd-hp-preview-placeholder" id="hp-placeholder-${windowId}">
                                    <span class="vd-hp-preview-placeholder-icon" aria-hidden="true">${iconMarkup('globe', '🌐', '', 34)}</span>
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
                        </div>
                        <div class="vd-hp-viewport-head">
                            <div class="vd-hp-viewport-title">
                                <span>${esc(t('homepage_studio.preview_tab'))}</span>
                                <strong class="vd-hp-preview-url" id="hp-url-${windowId}" title="${esc(t('homepage_studio.no_url'))}">—</strong>
                            </div>
                            <div class="vd-hp-viewport-toolbar">
                                <div class="vd-hp-device-seg" role="radiogroup" aria-label="${esc(t('homepage_studio.device_label'))}">
                                    <button type="button" data-hp-device="desktop" class="active" role="radio" aria-checked="true">${esc(t('homepage_studio.device_desktop'))}</button>
                                    <button type="button" data-hp-device="tablet" role="radio" aria-checked="false">${esc(t('homepage_studio.device_tablet'))}</button>
                                    <button type="button" data-hp-device="mobile" role="radio" aria-checked="false">${esc(t('homepage_studio.device_mobile'))}</button>
                                </div>
                                <span class="vd-hp-device-width" id="hp-device-width-${windowId}"></span>
                                <span class="vd-hp-toolbar-sep" aria-hidden="true"></span>
                                <button type="button" class="vd-hp-icon-btn" id="hp-refresh-${windowId}" title="${esc(t('homepage_studio.refresh_preview'))}" aria-label="${esc(t('homepage_studio.refresh_preview'))}">
                                    ${iconMarkup('refresh', '↻', 'vd-hp-btn-icon', 14)}
                                </button>
                                <button type="button" class="vd-hp-icon-btn is-disabled" id="hp-external-${windowId}" disabled title="${esc(t('homepage_studio.open_external'))}" aria-label="${esc(t('homepage_studio.open_external'))}">
                                    ${iconMarkup('external', '↗', 'vd-hp-btn-icon', 14)}
                                </button>
                                <button type="button" class="vd-hp-icon-btn" data-hp-fullscreen title="${esc(t('homepage_studio.fullscreen'))}" aria-label="${esc(t('homepage_studio.fullscreen'))}" aria-pressed="false">
                                    ${iconMarkup('maximize', '⛶', 'vd-hp-btn-icon', 14)}
                                </button>
                            </div>
                        </div>
                        <div class="vd-hp-footer"><span class="vd-hp-site-pill" id="hp-site-pill-${windowId}" hidden></span></div>
                    </main>
                    <div class="vd-hp-splitter" data-hp-splitter="inspector" role="separator" aria-orientation="vertical" tabindex="0" aria-label="${esc(t('homepage_studio.resize_inspector'))}" aria-controls="hp-inspector-${windowId}" aria-valuemin="${MIN_PANEL_WIDTH}" aria-valuemax="${MAX_PANEL_WIDTH}" aria-valuenow="${DEFAULT_INSPECTOR_WIDTH}"></div>
                    <aside class="vd-hp-inspector" id="hp-inspector-${windowId}" aria-label="${esc(t('homepage_studio.inspector_label'))}">
                        <div class="vd-hp-preview-tabs">
                            <div class="vd-hp-preview-tablist" role="tablist" aria-label="${esc(t('homepage_studio.inspector_tabs'))}">
                                <button type="button" class="vd-hp-preview-tab is-active" id="hp-tab-sites-${windowId}" role="tab" aria-selected="true" aria-controls="hp-panel-sites-${windowId}" tabindex="0">
                                    ${esc(t('homepage_studio.sites_tab'))}
                                </button>
                                <button type="button" class="vd-hp-preview-tab" id="hp-tab-history-${windowId}" role="tab" aria-selected="false" aria-controls="hp-panel-history-${windowId}" tabindex="-1">
                                    ${esc(t('homepage_studio.history_tab'))}
                                </button>
                            </div>
                            <button type="button" class="vd-hp-icon-btn" id="hp-collapse-inspector-${windowId}" title="${esc(t('homepage_studio.collapse_panel'))}" aria-label="${esc(t('homepage_studio.collapse_panel'))}">
                                ${iconMarkup('chevron-right', '›', 'vd-hp-btn-icon', 14)}
                            </button>
                        </div>
                        <div class="vd-hp-inspector-body">
                            <div class="vd-hp-inspector-panel is-active" id="hp-panel-sites-${windowId}" role="tabpanel" aria-labelledby="hp-tab-sites-${windowId}">
                                <div class="vd-hp-sites-list" id="hp-sites-list-${windowId}" role="listbox" aria-label="${esc(t('homepage_studio.sites_tab'))}">
                                    <div class="vd-hp-sites-empty">${esc(t('homepage_studio.sites_loading'))}</div>
                                </div>
                            </div>
                            <div class="vd-hp-inspector-panel vd-hp-history-panel" id="hp-panel-history-${windowId}" role="tabpanel" aria-labelledby="hp-tab-history-${windowId}" hidden>
                                <div class="vd-hp-history-controls" id="hp-history-controls-${windowId}">
                                    <input type="search" class="vd-hp-history-search" id="hp-history-search-${windowId}" placeholder="${esc(t('homepage_studio.history_search_placeholder'))}" aria-label="${esc(t('homepage_studio.history_search_placeholder'))}">
                                    <select class="vd-hp-history-filter" id="hp-history-filter-${windowId}" aria-label="${esc(t('homepage_studio.history_filter_label'))}">
                                        ${historyTypes.map(value => `<option value="${value}">${esc(tr('homepage_studio.history_filter_' + (value || 'all'), value || 'all'))}</option>`).join('')}
                                    </select>
                                    <button type="button" class="vd-hp-history-refresh" id="hp-history-refresh-${windowId}" title="${esc(t('homepage_studio.refresh'))}" aria-label="${esc(t('homepage_studio.refresh'))}">
                                        ${iconMarkup('refresh', '↻', 'vd-hp-btn-icon', 14)}
                                    </button>
                                </div>
                                <div class="vd-hp-history-list" id="hp-history-list-${windowId}" role="list" aria-label="${esc(t('homepage_studio.history_tab'))}">
                                    <div class="vd-hp-history-empty">${esc(t('homepage_studio.history_loading'))}</div>
                                </div>
                            </div>
                        </div>
                    </aside>
                </div>
            </div>
        `;

        const $ = id => container.querySelector('#' + id);
        const root = $(`hp-root-${windowId}`);
        const chatLog = $(`hp-log-${windowId}`);
        const chatInput = $(`hp-input-${windowId}`);
        const chatForm = $(`hp-form-${windowId}`);
        const sendBtn = $(`hp-send-${windowId}`);
        const sendLabel = $(`hp-send-label-${windowId}`);
        const targetSelect = $(`hp-target-${windowId}`);
        const statusDot = $(`hp-status-dot-${windowId}`);
        const statusText = $(`hp-status-text-${windowId}`);
        const statusPill = $(`hp-status-pill-${windowId}`);
        const previewZone = $(`hp-preview-zone-${windowId}`);
        const previewUrl = $(`hp-url-${windowId}`);
        const previewPanel = $(`hp-panel-preview-${windowId}`);
        const previewPlaceholder = $(`hp-placeholder-${windowId}`);
        const previewLoading = $(`hp-loading-${windowId}`);
        const refreshBtn = $(`hp-refresh-${windowId}`);
        const externalBtn = $(`hp-external-${windowId}`);
        const toggleChatBtn = $(`hp-toggle-chat-${windowId}`);
        const toggleInspectorBtn = $(`hp-toggle-inspector-${windowId}`);
        const collapseChatBtn = $(`hp-collapse-chat-${windowId}`);
        const collapseInspectorBtn = $(`hp-collapse-inspector-${windowId}`);
        const sitesTab = $(`hp-tab-sites-${windowId}`);
        const historyTab = $(`hp-tab-history-${windowId}`);
        const sitesPanel = $(`hp-panel-sites-${windowId}`);
        const historyPanel = $(`hp-panel-history-${windowId}`);
        const sitePill = $(`hp-site-pill-${windowId}`);
        const agentPill = $(`hp-agent-pill-${windowId}`);
        const agentElapsed = $(`hp-agent-elapsed-${windowId}`);
        const announceEl = $(`hp-announce-${windowId}`);
        const previewFrame = $(`hp-frame-${windowId}`);
        const tabList = root ? root.querySelector('.vd-hp-preview-tablist') : null;
        const stage = previewZone ? previewZone.querySelector('[data-hp-stage]') : null;

        function listen(target, event, handler, options) {
            if (!target) return;
            target.addEventListener(event, handler, options);
            state.listeners.push(() => target.removeEventListener(event, handler, options));
        }

        /* ---------- Persistence (per-window draft) ---------- */

        function persistDraft() {
            persistHomepageDraft(state, windowId);
        }

        function schedulePersist() {
            if (state.persistTimer) clearTimeout(state.persistTimer);
            state.persistTimer = setTimeout(() => {
                state.persistTimer = null;
                if (!state.disposed) persistDraft();
            }, 300);
        }

        function loadDraft() {
            try {
                const raw = window.localStorage.getItem('aurago.desktop.homepage.draft.' + windowId);
                if (!raw) return null;
                const draft = JSON.parse(raw);
                return draft && typeof draft === 'object' ? draft : null;
            } catch (_) {
                return null;
            }
        }
        /* Panel collapse / splitters live in the preview chrome module. */
        const clampPanelWidth = (window.HomepageStudioPreview && window.HomepageStudioPreview.clampPanelWidth)
            || (px => Math.max(MIN_PANEL_WIDTH, Math.min(MAX_PANEL_WIDTH, px)));

        function applyWidths() {
            if (state.modules.preview) state.modules.preview.applyWidths();
        }

        function setCollapsed(side, collapsed) {
            if (state.modules.preview) state.modules.preview.setCollapsed(side, collapsed);
        }

        /* Sub modules */
        if (window.HomepageStudioPreview && stage) {
            state.modules.preview = window.HomepageStudioPreview.create({
                root,
                zone: previewZone,
                stage,
                state,
                t,
                onDeviceChange: device => {
                    state.device = device;
                    schedulePersist();
                },
                onLayoutChange: () => schedulePersist()
            });
        }

        if (window.HomepageStudioSites) {
            state.modules.sites = window.HomepageStudioSites.create({
                api,
                esc,
                t,
                notify,
                iconMarkup,
                readonly,
                container: $(`hp-sites-list-${windowId}`),
                isDisposed: () => state.disposed,
                onSiteSelected: (siteId, detail) => {
                    state.selectedSiteId = siteId || 0;
                    state.selectedSiteName = detail ? (detail.name || detail.project_dir || '') : '';
                    updateSitePill(detail);
                    if (state.modules.history) state.modules.history.setProjectId(state.selectedSiteId);
                    if (state.lastStatus) refreshHomepageTargets(state.lastStatus);
                    schedulePersist();
                },
                safeExternalURL,
                tr,
                locale: appLocale
            });
        }

        if (window.HomepageStudioHistory) {
            state.modules.history = window.HomepageStudioHistory.create({
                api,
                esc,
                t,
                notify,
                confirmDialog,
                readonly,
                controls: $(`hp-history-controls-${windowId}`),
                list: $(`hp-history-list-${windowId}`),
                locale: appLocale,
                tr,
                isDisposed: () => state.disposed
            });
        }

        function updateSitePill(detail) {
            if (!sitePill) return;
            const liveFrame = previewFrame ? previewFrame.querySelector('.vd-hp-preview-iframe') : null;
            if (liveFrame) liveFrame.title = previewFrameTitle();
            if (!detail) {
                sitePill.hidden = true;
                sitePill.textContent = '';
                return;
            }
            const name = detail.name || detail.project_dir || '';
            const drift = homepageDrift(detail.drift_status);
            sitePill.innerHTML = `<span class="vd-hp-site-pill-name">${esc(name)}</span>` +
                `<span class="vd-hp-drift vd-hp-drift-${drift.key}">${esc(tr('homepage_studio.sites_drift_' + drift.status, drift.status))}</span>`;
            sitePill.hidden = false;
        }

        /* One drift enum for the whole studio: the sites module owns it. */
        function homepageDrift(status) {
            const module = window.HomepageStudioSites || {};
            const known = typeof module.driftStatus === 'function' ? module.driftStatus(status) : 'not_deployed';
            return { status: known, key: (module.DRIFT_KEYS || {})[known] || known.replace(/_/g, '-') };
        }
        /* Restore draft */
        const draft = loadDraft();
        if (draft) {
            if (typeof draft.target === 'string' && targetSelect.querySelector(`option[value="${draft.target}"]`)) {
                state.target = draft.target;
                targetSelect.value = draft.target;
            }
            if (draft.device && state.modules.preview) state.modules.preview.setDevice(draft.device);
            state.device = state.modules.preview ? state.modules.preview.getDevice() : (draft.device || 'desktop');
            state.chatWidth = Number(draft.chatWidth) > 0 ? clampPanelWidth(Number(draft.chatWidth)) : 0;
            state.inspectorWidth = Number(draft.inspectorWidth) > 0 ? clampPanelWidth(Number(draft.inspectorWidth)) : 0;
            applyWidths();
            if (draft.chatCollapsed) setCollapsed('chat', true);
            if (draft.inspectorCollapsed) setCollapsed('inspector', true);
            if (draft.selectedSiteId && state.modules.sites) {
                state.modules.sites.setSelected(draft.selectedSiteId);
                state.selectedSiteId = Number(draft.selectedSiteId) || 0;
            }
            if (state.modules.history) state.modules.history.syncFilters(draft.historyQuery, draft.historyFilter);
            const restoredTab = draft.inspectorTab === 'history' ? 'history' : 'sites';
            switchPanel(restoredTab);
            if (restoredTab === 'history') loadHistoryPanel();
        }
        /* Chat */
        autoResizeTextarea(chatInput);

        listen(chatInput, 'input', () => autoResizeTextarea(chatInput));
        listen(chatInput, 'keydown', e => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                chatForm.dispatchEvent(new Event('submit', { cancelable: true }));
            }
        });

        listen(chatForm, 'submit', e => {
            e.preventDefault();
            if (readonly) return;
            if (state.chatBusy) {
                if (state.abortCtrl) state.abortCtrl.abort();
                return;
            }
            const msg = chatInput.value.trim();
            if (msg) sendMessage(msg);
        });

        root.querySelectorAll('[data-hp-suggest]').forEach(chip => {
            listen(chip, 'click', () => {
                if (readonly || state.chatBusy) return;
                const prompt = t('homepage_studio.' + chip.getAttribute('data-hp-suggest'));
                if (prompt) sendMessage(prompt);
            });
        });

        listen(targetSelect, 'change', () => {
            state.target = targetSelect.value;
            schedulePersist();
            loadStatus();
        });

        listen(refreshBtn, 'click', () => refreshPreview());
        listen(externalBtn, 'click', () => {
            const safeURL = safeExternalURL(state.previewUrl);
            if (safeURL) window.open(safeURL, '_blank', 'noopener,noreferrer');
        });

        listen(toggleChatBtn, 'click', () => setCollapsed('chat', !state.chatCollapsed));
        listen(toggleInspectorBtn, 'click', () => setCollapsed('inspector', !state.inspectorCollapsed));
        listen(collapseChatBtn, 'click', () => setCollapsed('chat', true));
        listen(collapseInspectorBtn, 'click', () => setCollapsed('inspector', true));

        listen(sitesTab, 'click', () => selectTab(sitesTab, false));
        listen(historyTab, 'click', () => selectTab(historyTab, false));

        /* WAI-ARIA tabs pattern: arrow keys move selection and focus. */
        listen(tabList, 'keydown', event => {
            if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight' && event.key !== 'Home' && event.key !== 'End') return;
            const tabs = [sitesTab, historyTab];
            const currentIndex = tabs.indexOf(document.activeElement);
            if (currentIndex < 0) return;
            event.preventDefault();
            let nextIndex = currentIndex;
            if (event.key === 'ArrowRight') nextIndex = (currentIndex + 1) % tabs.length;
            else if (event.key === 'ArrowLeft') nextIndex = (currentIndex - 1 + tabs.length) % tabs.length;
            else if (event.key === 'Home') nextIndex = 0;
            else nextIndex = tabs.length - 1;
            selectTab(tabs[nextIndex], true);
        });

        listen(root, 'keydown', e => {
            if (e.key !== 'Escape') return;
            /* A running agent may be stopped from anywhere, including the composer. */
            if (state.chatBusy && state.abortCtrl) {
                state.abortCtrl.abort();
                return;
            }
            const typing = e.target.closest('input, textarea, select, [contenteditable]');
            if (typing) return;
            if (!state.inspectorCollapsed) {
                setCollapsed('inspector', true);
            } else if (!state.chatCollapsed) {
                setCollapsed('chat', true);
            }
        });

        /* The sites list is loaded first so the target resolver can reuse it
           instead of fetching a second copy. */
        const sitesReady = state.modules.sites ? state.modules.sites.load() : Promise.resolve();
        Promise.resolve(sitesReady).then(() => {
            if (state.disposed) return;
            loadStatus();
        });

        function loadHistoryPanel() {
            if (state.disposed) return;
            if (!state.modules.history) return;
            state.modules.history.setEnabled(!state.statusLoaded || state.homepageEnabled);
            state.modules.history.loadHistory();
        }

        function autoResizeTextarea(el) {
            if (!el) return;
            el.style.height = 'auto';
            const max = 120;
            el.style.height = Math.min(el.scrollHeight, max) + 'px';
            el.style.overflowY = el.scrollHeight > max ? 'auto' : 'hidden';
        }
        /* Status */
        async function loadStatus() {
            statusDot.className = 'vd-hp-status-dot loading';
            statusText.textContent = t('homepage_studio.checking_status');
            statusPill.title = t('homepage_studio.checking_status');
            try {
                const data = await api('/api/desktop/integrations/homepage/status');
                state.statusLoaded = true;
                state.lastStatus = data;
                state.homepageEnabled = data && data.status !== 'disabled';

                if (!state.homepageEnabled) {
                    setStatus('offline', t('homepage_studio.status_disabled'));
                    state.previewUrl = '';
                    updatePreviewUrl();
                    if (state.modules.history) state.modules.history.setEnabled(false);
                    return;
                }

                const webRunning = data && data.web_container && data.web_container.running;
                const pythonRunning = data && data.python_server && data.python_server.running;
                const serverRunning = webRunning || pythonRunning;

                let kind = serverRunning ? 'online' : 'offline';
                let text = serverRunning
                    ? t('homepage_studio.status_online')
                    : t('homepage_studio.status_offline');
                const target = String(state.target || 'local');
                if (target !== 'local') {
                    /* A cloud target is only healthy when it actually resolves. */
                    const targetLabel = t('homepage_studio.target_' + target);
                    if (homepageStatusPreviewURL(data, target)) {
                        kind = 'online';
                        text = targetLabel + ' · ' + t('homepage_studio.status_target_reachable');
                    } else {
                        kind = 'offline';
                        text = targetLabel + ' · ' + t('homepage_studio.target_unreachable');
                    }
                } else {
                    const extras = [];
                    if (serverRunning && (data.mode === 'python_fallback' || (pythonRunning && !webRunning))) {
                        extras.push(t('homepage_studio.status_python_fallback'));
                    }
                    if (data && data.tunnel_url) {
                        extras.push(t('homepage_studio.status_tunnel'));
                    }
                    if (extras.length) text += ' · ' + extras.join(' · ');
                }
                setStatus(kind, text);

                state.previewUrl = homepageStatusPreviewURL(data, state.target);
                updatePreviewUrl();
                refreshHomepageTargets(data);
            } catch (_) {
                setStatus('offline', t('homepage_studio.status_error'));
                state.previewUrl = '';
                updatePreviewUrl();
            }
        }

        function setStatus(kind, text) {
            statusDot.className = 'vd-hp-status-dot ' + kind;
            statusText.textContent = text;
            statusPill.title = text;
        }

        function homepageTargetLabel(target) { const value = String(target || 'local'); return t('homepage_studio.target_' + value); }

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
                const url = safeExternalURL(value);
                if (url) {
                    return url;
                }
            }
            return '';
        }

        function safeExternalURL(raw) {
            const value = firstString(raw);
            if (!value) return '';
            try {
                const url = new URL(value, window.location.origin);
                if (url.protocol === 'http:' || url.protocol === 'https:') return url.href;
            } catch (_) {}
            return '';
        }

        async function loadHomepageTargets() {
            if (state.targetsLoading) return state.targetsLoading;
            state.targetsLoading = (async () => {
                const nextTargets = [];
                const webhostsData = await safeHomepageApi('/api/desktop/integrations/integrations/webhosts');
                const webhosts = Array.isArray(webhostsData && webhostsData.webhosts) ? webhostsData.webhosts : [];
                for (const item of webhosts) {
                    if (item && item.id === 'homepage') {
                        homepageTargets.add(nextTargets, 'local', item.url, item.name || t('homepage_studio.default_name'), firstPreviewURL);
                    }
                }

                /* The sites panel owns the site list; the target resolver reuses
                   that list instead of fetching a second copy of it. */
                let sites = state.modules.sites && typeof state.modules.sites.getSites === 'function'
                    ? state.modules.sites.getSites()
                    : [];
                if (!sites.length) {
                    const sitesData = await safeHomepageApi('/api/desktop/integrations/homepage/sites');
                    sites = Array.isArray(sitesData && sitesData.sites) ? sitesData.sites : [];
                }
                for (const site of sites) {
                    homepageTargets.collect(site, nextTargets, firstPreviewURL);
                }

                /* Only the selected site needs its detail; the provider URLs of
                   the other sites come from the status payload. */
                const detailId = Number(state.selectedSiteId) || 0;
                if (detailId > 0) {
                    const cached = state.modules.sites && typeof state.modules.sites.getDetail === 'function'
                        ? state.modules.sites.getDetail(detailId)
                        : null;
                    let site = cached;
                    if (!site) {
                        const response = await safeHomepageApi('/api/desktop/integrations/homepage/sites/' + encodeURIComponent(String(detailId)));
                        site = response && response.site ? response.site : null;
                    }
                    homepageTargets.collect(site, nextTargets, firstPreviewURL);
                }

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

        function homepageStatusPreviewURL(data, target) {
            if (!data) return '';
            const webRunning = data.web_container && data.web_container.running;
            const pythonRunning = data.python_server && data.python_server.running;
            const serverRunning = webRunning || pythonRunning;
            const externalURL = homepageTargets.external(target, state.deploymentTargets);

            const objectURL = key => {
                const obj = data[key];
                if (!obj || typeof obj !== 'object') return '';
                return firstPreviewURL(obj.preview_url, obj.url, obj.deployment_url, obj.deploy_url, obj.browser_url);
            };

            switch (target) {
                case 'vercel':
                    return firstPreviewURL(data.vercel_url, data.vercel_deployment_url, data.deployment_url, objectURL('vercel'), externalURL);
                case 'netlify':
                    return firstPreviewURL(data.netlify_url, data.netlify_deploy_url, data.deploy_url, objectURL('netlify'), externalURL);
                case 'here_now':
                    return firstPreviewURL(data.here_now_url, data.site_url, data.verified_url, objectURL('here_now'), externalURL);
                case 'remote':
                    return firstPreviewURL(data.remote_url, data.remote_deploy_url, objectURL('remote'), externalURL);
                case 'local':
                default:
                    break;
            }

            if (data.preview_url) return firstPreviewURL(data.preview_url);
            if (serverRunning && data.tunnel_url) return firstPreviewURL(data.tunnel_url);
            if (webRunning && data.web_container.browser_url) {
                return firstPreviewURL(data.web_container.browser_url);
            }
            if (pythonRunning && data.python_server.browser_url) {
                return firstPreviewURL(data.python_server.browser_url);
            }
            if (externalURL) return externalURL;
            return '';
        }

        function updatePreviewUrl() {
            const safeURL = safeExternalURL(state.previewUrl);
            const hasUrl = !!safeURL;
            externalBtn.disabled = !hasUrl;
            externalBtn.classList.toggle('is-disabled', !hasUrl);
            if (hasUrl) {
                previewUrl.textContent = safeURL;
                previewUrl.title = safeURL;
                showPreview(safeURL);
            } else {
                previewUrl.textContent = '—';
                previewUrl.title = t('homepage_studio.no_url');
                hidePreview();
            }
        }

        function showPreview(url) {
            const safeURL = safeExternalURL(url);
            if (!safeURL) {
                hidePreview();
                return;
            }
            let iframe = previewFrame ? previewFrame.querySelector('.vd-hp-preview-iframe') : null;
            if (!iframe) {
                previewPlaceholder.style.display = 'none';
                iframe = document.createElement('iframe');
                iframe.className = 'vd-hp-preview-iframe';
                iframe.sandbox = 'allow-scripts allow-forms';
                iframe.referrerPolicy = 'no-referrer';
                iframe.title = previewFrameTitle();
                if (previewFrame) previewFrame.appendChild(iframe);
                else previewPanel.insertBefore(iframe, previewLoading);
            }
            if (iframe.src !== safeURL) {
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
                iframe.src = safeURL;
            }
        }

        function hidePreview() {
            const iframe = previewFrame ? previewFrame.querySelector('.vd-hp-preview-iframe') : null;
            if (iframe) iframe.remove();
            previewPlaceholder.style.display = '';
        }

        /* The preview frame needs an accessible name. */
        function previewFrameTitle() { const label = t('homepage_studio.preview_tab'); return state.selectedSiteName ? label + ': ' + state.selectedSiteName : label; }

        function refreshPreview() {
            if (state.disposed) return;
            if (!state.previewUrl) {
                loadStatus();
                return;
            }
            const safeURL = safeExternalURL(state.previewUrl);
            if (!safeURL) {
                previewLoading.classList.remove('active');
                previewLoading.setAttribute('aria-hidden', 'true');
                hidePreview();
                return;
            }
            previewLoading.classList.add('active');
            previewLoading.setAttribute('aria-hidden', 'false');
            const iframe = previewFrame ? previewFrame.querySelector('.vd-hp-preview-iframe') : null;
            if (iframe) {
                iframe.title = previewFrameTitle();
                iframe.onload = () => {
                    previewLoading.classList.remove('active');
                    previewLoading.setAttribute('aria-hidden', 'true');
                };
                iframe.src = safeURL;
            } else {
                showPreview(safeURL);
            }
        }

        function switchPanel(panel) {
            state.inspectorTab = panel === 'history' ? 'history' : 'sites';
            const historyActive = state.inspectorTab === 'history';
            historyTab.classList.toggle('is-active', historyActive);
            historyTab.setAttribute('aria-selected', historyActive ? 'true' : 'false');
            sitesTab.classList.toggle('is-active', !historyActive);
            sitesTab.setAttribute('aria-selected', historyActive ? 'false' : 'true');
            historyTab.setAttribute('tabindex', historyActive ? '0' : '-1');
            sitesTab.setAttribute('tabindex', historyActive ? '-1' : '0');
            historyPanel.classList.toggle('is-active', historyActive);
            sitesPanel.classList.toggle('is-active', !historyActive);
            if (historyActive) {
                historyPanel.removeAttribute('hidden');
                sitesPanel.setAttribute('hidden', '');
            } else {
                sitesPanel.removeAttribute('hidden');
                historyPanel.setAttribute('hidden', '');
            }
            schedulePersist();
        }

        function selectTab(tab, moveFocus) {
            if (!tab) return;
            const historyActive = tab === historyTab;
            switchPanel(historyActive ? 'history' : 'sites');
            if (historyActive) loadHistoryPanel();
            if (moveFocus) tab.focus();
        }

        /* The chat log is a transcript, not a live region: the streaming bubble
           is rewritten per frame. Only finished events are announced. */
        function announce(message) { if (announceEl && message) announceEl.textContent = message; }
        /* Agent chat stream */
        function homepageWindowContext() {
            return {
                source: 'homepage-studio',
                app_id: 'homepage-studio',
                window_id: windowId,
                label: t('homepage_studio.welcome_heading'),
                purpose: 'Homepage Studio edits AuraGo homepage websites and pages in the managed homepage workspace.',
                guide: 'Use homepage_project, homepage_file, homepage_quality, homepage_deploy, and homepage_git. Do not use virtual_desktop apps, widgets, or files for Homepage Studio site changes.',
                resources: [{ kind: 'homepage_target', label: state.target, path: state.target }]
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
                announce(t('homepage_studio.agent_finished'));
                if (state.disposed) return;
                refreshPreview();
                loadHistoryPanel();
                if (state.modules.sites) state.modules.sites.load();

                if (streamingContent.trim()) {
                    if (state.refreshTimer) clearTimeout(state.refreshTimer);
                    state.refreshTimer = setTimeout(() => {
                        state.refreshTimer = null;
                        if (!state.disposed) refreshPreview();
                    }, 500);
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
                const msg = (err && err.message) || String(err || t('desktop.chat_request_failed'));
                appendBubble('agent', msg);
                announce(msg);
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
                if (!parser) throw new Error(t('desktop.chat_request_failed'));

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
            chatInput.disabled = !!busy || readonly;
            sendBtn.classList.toggle('is-stop', !!busy);
            chatLog.setAttribute('aria-busy', busy ? 'true' : 'false');
            const sendText = t('desktop.send');
            const stopText = t('desktop.chat_stop');
            sendLabel.textContent = busy ? stopText : sendText;
            sendBtn.title = busy ? stopText : sendText;
            if (agentPill) {
                agentPill.hidden = !busy;
            }
            if (busy) {
                state.busyStart = Date.now();
                announce(t('homepage_studio.agent_working'));
                const elapsedText = (seconds) => t('desktop.noisemaker_progress_elapsed', { seconds });
                if (agentElapsed) agentElapsed.textContent = elapsedText(0);
                state.busyTimer = setInterval(() => {
                    if (agentElapsed) agentElapsed.textContent = elapsedText(Math.round((Date.now() - state.busyStart) / 1000));
                }, 500);
            } else if (state.busyTimer) {
                clearInterval(state.busyTimer);
                state.busyTimer = null;
                if (agentElapsed) agentElapsed.textContent = '';
            }
        }
    }

    function persistHomepageDraft(state, windowId) {
        try {
            const historyModule = state.modules && state.modules.history;
            const filters = historyModule ? historyModule.getFilters() : { query: '', filter: '' };
            const payload = {
                v: 1,
                target: state.target,
                device: state.device,
                chatCollapsed: state.chatCollapsed,
                inspectorCollapsed: state.inspectorCollapsed,
                inspectorTab: state.inspectorTab,
                chatWidth: state.chatWidth,
                inspectorWidth: state.inspectorWidth,
                selectedSiteId: state.selectedSiteId,
                historyQuery: filters.query,
                historyFilter: filters.filter,
                savedAt: Date.now()
            };
            window.localStorage.setItem('aurago.desktop.homepage.draft.' + windowId, JSON.stringify(payload));
        } catch (_) {}
    }

    function dispose(windowId) {
        const state = instances.get(windowId);
        if (!state) return;
        if (state.abortCtrl) { state.abortCtrl.abort(); state.abortCtrl = null; }
        if (state.persistTimer) { clearTimeout(state.persistTimer); state.persistTimer = null; }
        if (state.refreshTimer) { clearTimeout(state.refreshTimer); state.refreshTimer = null; }
        persistHomepageDraft(state, windowId);
        state.disposed = true;
        if (state.busyTimer) { clearInterval(state.busyTimer); state.busyTimer = null; }
        if (state.listeners) {
            state.listeners.forEach(off => off());
            state.listeners.length = 0;
        }
        if (state.modules) {
            Object.keys(state.modules).forEach(key => {
                const mod = state.modules[key];
                if (mod && typeof mod.dispose === 'function') mod.dispose();
            });
        }
        instances.delete(windowId);
    }

    window.HomepageStudioApp = { render, dispose };
})();
