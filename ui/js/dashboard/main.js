// AuraGo – dashboard page logic
// Extracted from dashboard.html

        // ══════════════════════════════════════════════════════════════════════════════
        // AuraGo Dashboard – JavaScript
        // ══════════════════════════════════════════════════════════════════════════════

        const LANG = document.documentElement.lang || 'en';
        // Expose LANG explicitly so it survives ES-Module conversion and is traceable across scripts.
        window.LANG = LANG;

        // Theme is handled by shared.js (initTheme/toggleTheme); no separate init needed here.

        // ── CSS Variable Helper ─────────────────────────────────────────────────────
        function cv(name) { return getComputedStyle(document.documentElement).getPropertyValue(name).trim(); }

        // ── Chart.js Global Defaults ────────────────────────────────────────────────
        function setChartDefaults() {
            Chart.defaults.color = cv('--text-secondary');
            Chart.defaults.borderColor = cv('--border-subtle');
            Chart.defaults.font.family = "'Inter', system-ui, sans-serif";
            Chart.defaults.font.size = 11;
            Chart.defaults.plugins.legend.display = false;
            Chart.defaults.responsive = true;
            Chart.defaults.maintainAspectRatio = false;
        }
        setChartDefaults();

        // ── i18n: apply translations to static HTML elements ─────────────────────
        function applyTranslations() {
            document.querySelectorAll('[data-i18n]').forEach(el => {
                const key = el.getAttribute('data-i18n');
                if (!key) return;
                const value = t(key);
                // Keep nested markup (SVG icons inside buttons). Prefer an explicit
                // text child when present; otherwise update only non-empty text nodes.
                if (el.children.length > 0) {
                    const textTarget = el.querySelector(':scope > [data-i18n-text], :scope > .i18n-text');
                    if (textTarget) {
                        textTarget.textContent = value;
                        return;
                    }
                    let updated = false;
                    el.childNodes.forEach(node => {
                        if (node.nodeType === Node.TEXT_NODE && node.textContent && node.textContent.trim()) {
                            const lead = (node.textContent.match(/^\s*/) || [''])[0];
                            const trail = (node.textContent.match(/\s*$/) || [''])[0];
                            node.textContent = lead + value + trail;
                            updated = true;
                        }
                    });
                    if (!updated) {
                        const span = document.createElement('span');
                        span.className = 'i18n-text';
                        span.textContent = value;
                        el.appendChild(document.createTextNode(' '));
                        el.appendChild(span);
                    }
                    return;
                }
                el.textContent = value;
            });
            document.querySelectorAll('[data-i18n-title]').forEach(el => {
                const key = el.getAttribute('data-i18n-title');
                if (key) el.title = t(key);
            });
            document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
                const key = el.getAttribute('data-i18n-placeholder');
                if (key) el.placeholder = t(key);
            });
            document.querySelectorAll('[data-i18n-aria-label]').forEach(el => {
                const key = el.getAttribute('data-i18n-aria-label');
                if (key) el.setAttribute('aria-label', t(key));
            });
            document.title = t('dashboard.page_title');
        }
        applyTranslations();

        // ── Chart Instances ─────────────────────────────────────────────────────────
        const Charts = {};

        // ── API ─────────────────────────────────────────────────────────────────────
        // Thin wrapper over DashAPI (dash-api.js, loaded before this file). The default ttl is 0:
        // calls outside the section loaders (pagers, searches, reloads after an edit, SSE follow-ups)
        // always get fresh data; concurrent calls for one URL still share one request. Section
        // loaders pass sectionFetch(force) and use the 15 s cache.
        const SECTION_CACHE_TTL_MS = 15000;
        const API = {
            /**
             * GET request. Returns parsed JSON or null on failure (401 -> login redirect).
             * Card-level error handling is done via CardState helpers.
             */
            get: (url, opts) => DashAPI.get(url, Object.assign({ ttl: 0 }, opts)),
            /**
             * GET request with structured result for per-card error handling.
             * Returns { ok: true, data } on success, { ok: false, status } on failure.
             */
            getWithStatus: (url, opts) => DashAPI.getWithStatus(url, Object.assign({ ttl: 0 }, opts)),
        };
        // Fetch options for section loaders: cached for SECTION_CACHE_TTL_MS unless forced.
        function sectionFetch(force) {
            return { ttl: SECTION_CACHE_TTL_MS, force: force === true };
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // CARD STATE MANAGER — loading / error / loaded states per card
        // Uses data-state attribute for CSS shimmer overlay (non-destructive).
        // On error, body content is replaced with error state + retry button.
        // ══════════════════════════════════════════════════════════════════════════════
        const CardState = {
            _retryFns: {},

            /**
             * Set a card to loading state (shimmer overlay on existing content).
             * Non-destructive — preserves existing child elements for render functions.
             * @param {string} cardId - The card element id (e.g. 'card-system')
             */
            setLoading(cardId) {
                const card = document.getElementById(cardId);
                if (!card) return;
                card.setAttribute('data-state', 'loading');
            },

            /**
             * Set a card to error state with a retry button.
             * Replaces card body with error state. Original content is saved
             * and restored on successful retry.
             * @param {string} cardId - The card element id
             * @param {Function} retryFn - Async function to call on retry
             * @param {object} [opts] - { title, desc, status }
             */
            setError(cardId, retryFn, opts) {
                const card = document.getElementById(cardId);
                if (!card) return;
                card.setAttribute('data-state', 'error');
                const body = card.querySelector('.dash-card-body');
                if (!body) return;
                if (!body.dataset.originalContent) {
                    body.dataset.originalContent = body.innerHTML;
                }
                const o = opts || {};
                const title = o.title || t('dashboard.error_title');
                let desc = o.desc || t('dashboard.error_generic');
                if (o.status === 401) desc = t('dashboard.error_auth');
                else if (o.status >= 500) desc = t('dashboard.error_server');
                else if (o.status === 0) desc = t('dashboard.error_network');
                body.innerHTML = `<div class="dash-error-state">
                    <span class="dash-error-icon">⚠</span>
                    <p class="dash-error-title">${esc(title)}</p>
                    <p class="dash-error-desc">${esc(desc)}</p>
                    <button type="button" class="dash-error-retry" onclick="CardState.retry('${cardId}')">
                        <span class="dash-error-spinner"></span>
                        <span class="dash-error-retry-label">${esc(t('dashboard.retry'))}</span>
                    </button>
                </div>`;
                this._retryFns[cardId] = retryFn;
            },

            /**
             * Set a card to loaded state, restoring original content if it was
             * replaced by an error state.
             * @param {string} cardId - The card element id
             */
            setLoaded(cardId) {
                const card = document.getElementById(cardId);
                if (!card) return;
                card.setAttribute('data-state', 'loaded');
                const body = card.querySelector('.dash-card-body');
                if (!body) return;
                if (body.dataset.originalContent) {
                    // Destroy Chart.js instances in the body before replacing
                    body.querySelectorAll('canvas').forEach(canvas => {
                        const chartInstance = Chart.getChart(canvas);
                        if (chartInstance) chartInstance.destroy();
                    });
                    body.innerHTML = body.dataset.originalContent;
                    delete body.dataset.originalContent;
                }
                // Clean up retry function reference
                delete this._retryFns[cardId];
            },

            /**
             * Clear all stored retry functions (call on tab switch to prevent leaks).
             */
            clearAllRetryFns() {
                this._retryFns = {};
            },

            /**
             * Retry handler for error-state retry buttons.
             * @param {string} cardId
             */
            async retry(cardId) {
                const retryFn = this._retryFns[cardId];
                if (!retryFn) return;
                const btn = document.querySelector('#' + cardId + ' .dash-error-retry');
                if (btn) btn.classList.add('loading');
                try {
                    await retryFn();
                } catch (e) {
                    // Error state will be re-set by the loader if it fails again
                }
            },
        };
        // Expose CardState globally for inline onclick handlers
        window.CardState = CardState;

        let currentMoodHours = 24;

        // ══════════════════════════════════════════════════════════════════════════════
        // SECTIONS — DashRouter (dash-router.js) shows a section, then calls loadSectionContent(id)
        // ══════════════════════════════════════════════════════════════════════════════

        const SectionState = { loaded: {} };
        const KnowledgeGraphState = { nodes: [], edges: [], importantNodes: [], importantEdges: [], stats: null, focusNodeId: '', focusPayload: null, editingNodeId: '', editingEdgeKey: '', showAll: false, filterType: '', filterSource: '', modalKind: '', modalNodeId: '', modalTriggerEl: null };
        function dashSetHidden(el, hidden) {
            if (!el) return;
            el.classList.toggle('is-hidden', hidden);
        }

        // Every show reloads its section through the loader its module registered (DashSections, dash-sections.js;
        // modules in sections/<id>.js); within SECTION_CACHE_TTL_MS the data comes from the DashAPI cache. force
        // (section refresh button, reload after an edit) bypasses the cache.
        async function loadSectionContent(id, { force = false } = {}) {
            const section = DashSections.get(id);
            if (!section) return;
            try {
                const outcome = await section.load({ force });
                SectionState.loaded[id] = true;
                // A loader that resolves to false failed (its cards show the error state): keep the last stamp.
                if (outcome === false) return;
                const updated = document.querySelector('[data-section-updated="' + id + '"]');
                if (updated) updated.textContent = t('dashboard.section.updated').replace('{{time}}', new Date().toLocaleTimeString());
            } catch (err) {
                console.warn('loadSectionContent failed', id, err);
            }
        }

        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
        }

        function renderCollapsibleList(items, renderItem, limit = 5) {
            if (!Array.isArray(items) || items.length === 0) return '';
            if (items.length <= limit) return items.map(renderItem).join('');
            const visible = items.slice(0, limit).map(renderItem).join('');
            const hidden = items.slice(limit).map(renderItem).join('');
            return `${visible}<div class="collapsible-hidden is-hidden">${hidden}</div><button type="button" class="collapsible-toggle" onclick="toggleCollapsible(this)">${esc(t('dashboard.show_more'))}</button>`;
        }

        function toggleCollapsible(btn) {
            const hidden = btn.previousElementSibling;
            const isHidden = hidden.classList.contains('is-hidden');
            hidden.classList.toggle('is-hidden', !isHidden);
            btn.textContent = isHidden ? t('dashboard.show_less') : t('dashboard.show_more');
        }

        function formatUptime(seconds) {
            const d = Math.floor(seconds / 86400);
            const h = Math.floor((seconds % 86400) / 3600);
            const m = Math.floor((seconds % 3600) / 60);
            if (d > 0) return d + 'd ' + h + 'h';
            if (h > 0) return h + 'h ' + m + 'm';
            return m + 'm';
        }
        function truncate(s, max) {
            if (!s || s.length <= max) return s || '';
            return s.substring(0, max) + '…';
        }

        function escapeHtml(str) {
            const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
            return String(str).replace(/[&<>"']/g, c => map[c]);
        }

        function escapeJsString(str) {
            return String(str)
                .replace(/\\/g, '\\\\')
                .replace(/'/g, "\\'")
                .replace(/\r/g, '\\r')
                .replace(/\n/g, '\\n');
        }

        // ── Relative Time ────────────────────────────────────────────────────────────
        function relativeTime(unixMillis) {
            const diff = Math.floor((Date.now() - unixMillis) / 1000);
            if (diff < 60) return diff + 's';
            if (diff < 3600) return Math.floor(diff / 60) + 'm';
            if (diff < 86400) return Math.floor(diff / 3600) + 'h';
            return Math.floor(diff / 86400) + 'd';
        }

        function initDashboard() {
            // Log viewer listeners (the elements exist while the logs section is hidden)
            const logFilter = document.getElementById('log-filter');
            const logScrollBtn = document.getElementById('log-scroll-btn');
            const logRefreshBtn = document.getElementById('log-refresh-btn');
            if (logFilter) logFilter.addEventListener('input', applyLogFilter);
            if (logScrollBtn) logScrollBtn.addEventListener('click', scrollLogsToBottom);
            if (logRefreshBtn) logRefreshBtn.addEventListener('click', async () => {
                logRefreshBtn.classList.add('is-busy');
                logRefreshBtn.disabled = true;
                logRefreshBtn.setAttribute('aria-busy', 'true');
                try {
                    // Same path as the section refresh button: one logs request, cache bypassed.
                    await loadSectionContent('logs', { force: true });
                } finally {
                    logRefreshBtn.classList.remove('is-busy');
                    logRefreshBtn.disabled = false;
                    logRefreshBtn.removeAttribute('aria-busy');
                }
            });
            const knowledgeSearchInput = document.getElementById('knowledge-search-input');
            const knowledgeSearchButton = document.getElementById('knowledge-search-button');
            if (knowledgeSearchButton) {
                knowledgeSearchButton.addEventListener('click', () => executeKnowledgeGraphSearch((knowledgeSearchInput?.value || '').trim()));
            }
            if (knowledgeSearchInput) {
                knowledgeSearchInput.addEventListener('keydown', (e) => {
                    if (e.key === 'Enter') {
                        e.preventDefault();
                        executeKnowledgeGraphSearch(knowledgeSearchInput.value.trim());
                    }
                });
            }
            const knowledgeGraphReset = document.getElementById('knowledge-graph-reset');
            if (knowledgeGraphReset) {
                knowledgeGraphReset.addEventListener('click', resetKnowledgeGraphFocus);
            }
            const knowledgeGraphViewToggle = document.getElementById('knowledge-graph-view-toggle');
            if (knowledgeGraphViewToggle) {
                knowledgeGraphViewToggle.addEventListener('click', (e) => {
                    const btn = e.target.closest('[data-kg-view]');
                    if (btn && !btn.disabled && typeof setKnowledgeGraphViewMode === 'function') {
                        setKnowledgeGraphViewMode(btn.dataset.kgView);
                    }
                });
            }
            setupAuditControls();
            setupCronjobsControls();
            document.addEventListener('click', (e) => {
                const mergeButton = e.target.closest('[data-kg-merge-source]');
                if (mergeButton) {
                    e.preventDefault();
                    const sourceID = mergeButton.getAttribute('data-kg-merge-source');
                    const targetID = mergeButton.getAttribute('data-kg-merge-target');
                    const label = mergeButton.getAttribute('data-kg-merge-label') || '';
                    if (sourceID && targetID && typeof mergeKnowledgeGraphNodes === 'function') {
                        mergeKnowledgeGraphNodes(targetID, sourceID, label);
                    }
                    return;
                }
                const nodeLink = e.target.closest('[data-kg-open-node]');
                if (nodeLink) {
                    const nodeID = nodeLink.getAttribute('data-kg-open-node');
                    if (nodeID) {
                        e.preventDefault();
                        openKGDetailModal(nodeID, nodeLink);
                    }
                    return;
                }
                const nodeCard = e.target.closest('[data-kg-node-id]');
                if (nodeCard) {
                    const nodeID = nodeCard.getAttribute('data-kg-node-id');
                    if (nodeID) {
                        openKGDetailModal(nodeID, nodeCard);
                    }
                }
            });
            // Section header refresh buttons reload their section and bypass the DashAPI cache.
            // aria-busy (not disabled) guards double clicks without dropping keyboard focus.
            document.addEventListener('click', async (e) => {
                const refreshBtn = e.target.closest('[data-section-refresh]');
                if (!refreshBtn || refreshBtn.getAttribute('aria-busy') === 'true') return;
                refreshBtn.setAttribute('aria-busy', 'true');
                try {
                    await loadSectionContent(refreshBtn.getAttribute('data-section-refresh'), { force: true });
                } finally {
                    refreshBtn.removeAttribute('aria-busy');
                }
            });

            // Sidebar status block, badges and the summary refresh schedule run on every section
            // (dash-summary.js); the overview tiles and "Customize" belong to dash-overview.js.
            DashSummary.init();
            DashOverview.init();

            // Show the section from the hash (old tab ids are mapped) or the stored one.
            // Every show loads its section (served from the DashAPI cache while fresh).
            DashRouter.init({ onShow: id => loadSectionContent(id) });
        }

        // Note: live system, mission, memory, personality, daemon, and audit updates flow through
        // SSE handlers registered in dashboard-events.js#connectSSE. The only polling is the summary
        // feed (dash-summary.js): every 60 s while the page is visible, for sources without events.
