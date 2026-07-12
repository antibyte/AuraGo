// AuraGo – dashboard page logic
// Extracted from dashboard.html

        // ══════════════════════════════════════════════════════════════════════════════
        // AuraGo Dashboard – JavaScript
        // ══════════════════════════════════════════════════════════════════════════════

        const LANG = document.documentElement.lang || 'en';
        const HIDDEN_INTEGRATIONS = new Set(['onedrive']);
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
        let _apiErrorShown = false;
        const API = {
            /**
             * GET request. Returns parsed JSON or null on failure.
             * Card-level error handling is done via CardState helpers.
             */
            get: url => fetch(url, { credentials: 'same-origin' }).then(r => {
                if (r.status === 401) {
                    window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
                    return null;
                }
                return r.ok ? r.json() : null;
            }).catch(() => {
                if (!_apiErrorShown && typeof showToast === 'function') {
                    _apiErrorShown = true;
                    showToast(t('dashboard.api_error'), 'error', 5000);
                    setTimeout(() => { _apiErrorShown = false; }, 15000);
                }
                return null;
            }),
            /**
             * GET request with structured result for per-card error handling.
             * Returns { ok: true, data } on success, { ok: false, status } on failure.
             * Use this with CardState.load() for automatic loading/error states.
             */
            getWithStatus(url) {
                return fetch(url, { credentials: 'same-origin' }).then(r => {
                    if (r.status === 401) {
                        window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
                        return { ok: false, status: 401 };
                    }
                    if (!r.ok) return { ok: false, status: r.status };
                    return r.json().then(data => ({ ok: true, data })).catch(() => ({ ok: false, status: 0 }));
                }).catch(() => ({ ok: false, status: 0 }));
            },
            fetchAll(hours) {
                return Promise.all([
                    this.get('/api/dashboard/system'),
                    this.get('/api/budget'),
                    this.get('/api/personality/state'),
                    this.get('/api/dashboard/mood-history?hours=' + (hours || 24)),
                    this.get('/api/dashboard/memory'),
                    this.get('/api/dashboard/profile'),
                    this.get('/api/dashboard/activity'),
                    this.get('/api/dashboard/prompt-stats'),
                    this.get('/api/dashboard/logs?lines=100'),
                    this.get('/api/dashboard/github-repos'),
                    this.get('/api/dashboard/overview'),
                    this.get('/api/credits'),
                    this.get('/api/dashboard/tool-stats'),
                ]);
            }
        };

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
        // TAB SYSTEM
        // ══════════════════════════════════════════════════════════════════════════════

        const TabState = { active: 'overview', loaded: {} };
        const KnowledgeGraphState = { nodes: [], edges: [], importantNodes: [], importantEdges: [], stats: null, focusNodeId: '', focusPayload: null, editingNodeId: '', editingEdgeKey: '', showAll: false, filterType: '', filterSource: '', modalKind: '', modalNodeId: '', modalTriggerEl: null };
        const VALID_TABS = ['overview', 'agent', 'user', 'knowledge', 'filesync', 'audit', 'cronjobs', 'system'];
        function dashSetHidden(el, hidden) {
            if (!el) return;
            el.classList.toggle('is-hidden', hidden);
        }

        function showTab(tabId) {
            if (!VALID_TABS.includes(tabId)) tabId = 'overview';
            history.replaceState(null, '', '#' + tabId);
            // Persist last tab only after the next microtask. This guards against a failing
            // initial network call leaving the user with a broken saved tab on reload.
            queueMicrotask(() => { try { localStorage.setItem('aurago-dash-tab', tabId); } catch (_) {} });
            // Clean up stale retry function references from previous tab
            CardState.clearAllRetryFns();
            const tablist = document.getElementById('dashTabs');
            const buttons = tablist ? Array.from(tablist.querySelectorAll('.dash-tab')) : [];
            buttons.forEach(btn => {
                const isActive = btn.dataset.tab === tabId;
                btn.classList.toggle('active', isActive);
                btn.setAttribute('aria-selected', String(isActive));
                btn.tabIndex = isActive ? 0 : -1;
            });
            document.querySelectorAll('.dash-tab-panel').forEach(panel => {
                dashSetHidden(panel, panel.id !== 'tab-' + tabId);
            });
            TabState.active = tabId;
            // Always refresh the active tab so Overview/Agent/User stay current (M7).
            loadTabContent(tabId);
        }

        // ── Tab keyboard navigation (ARIA tablist pattern) ─────────────────────────
        function initTabKeyboardNav() {
            const tablist = document.getElementById('dashTabs');
            if (!tablist) return;
            tablist.addEventListener('keydown', (e) => {
                if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) return;
                const buttons = Array.from(tablist.querySelectorAll('.dash-tab'));
                const currentIndex = buttons.findIndex(b => b.classList.contains('active'));
                if (currentIndex === -1) return;
                e.preventDefault();
                let nextIndex;
                if (e.key === 'ArrowRight') nextIndex = (currentIndex + 1) % buttons.length;
                else if (e.key === 'ArrowLeft') nextIndex = (currentIndex - 1 + buttons.length) % buttons.length;
                else if (e.key === 'Home') nextIndex = 0;
                else if (e.key === 'End') nextIndex = buttons.length - 1;
                if (nextIndex == null) return;
                const targetBtn = buttons[nextIndex];
                if (targetBtn) {
                    showTab(targetBtn.dataset.tab);
                    targetBtn.focus();
                }
            });
        }

        async function loadTabContent(tabId) {
            try {
                switch (tabId) {
                    case 'overview': await loadTabOverview(); break;
                    case 'agent':    await loadTabAgent(); break;
                    case 'user':     await loadTabUser(); break;
                    case 'knowledge': await loadTabKnowledge(); break;
                    case 'filesync':  await loadFileSyncStatus(); break;
                    case 'audit':     await loadTabAudit(); break;
                    case 'cronjobs':  await loadTabCronjobs(); break;
                    case 'system':   await loadTabSystem(); break;
                }
                TabState.loaded[tabId] = true;
            } catch (err) {
                console.warn('loadTabContent failed', tabId, err);
            }
        }

        async function loadAgentBanner() {
            const overview = await API.get('/api/dashboard/overview');
            if (overview) {
                renderAgentBanner(overview, overview?.context?.total_chars);
            }
        }

        async function loadTabOverview() {
            // Set loading state on all overview cards
            CardState.setLoading('card-system');
            CardState.setLoading('card-quickstatus');
            CardState.setLoading('card-budget');
            CardState.setLoading('card-optimization');
            CardState.setLoading('card-compression');
            CardState.setLoading('card-mission-history');

            const results = await Promise.all([
                API.getWithStatus('/api/dashboard/system'),
                API.getWithStatus('/api/budget'),
                API.getWithStatus('/api/dashboard/overview'),
                API.getWithStatus('/api/credits'),
                API.getWithStatus('/api/dashboard/optimization'),
                API.getWithStatus('/api/dashboard/compression'),
            ]);
            const [systemR, budgetR, overviewR, creditsR, optR, compR] = results;
            const system = systemR.ok ? systemR.data : null;
            const budget = budgetR.ok ? budgetR.data : null;
            const overview = overviewR.ok ? overviewR.data : null;
            const credits = creditsR.ok ? creditsR.data : null;
            const opt = optR.ok ? optR.data : null;
            const comp = compR.ok ? compR.data : null;

            // Set loaded/error per card
            systemR.ok ? CardState.setLoaded('card-system') : CardState.setError('card-system', loadTabOverview, { status: systemR.status });
            overviewR.ok ? CardState.setLoaded('card-quickstatus') : CardState.setError('card-quickstatus', loadTabOverview, { status: overviewR.status });
            budgetR.ok ? CardState.setLoaded('card-budget') : CardState.setError('card-budget', loadTabOverview, { status: budgetR.status });
            optR.ok ? CardState.setLoaded('card-optimization') : CardState.setError('card-optimization', loadTabOverview, { status: optR.status });
            compR.ok ? CardState.setLoaded('card-compression') : CardState.setError('card-compression', loadTabOverview, { status: compR.status });

            if (overview) {
                renderAgentBanner(overview, overview?.context?.total_chars);
                renderQuickStatus(overview);
            }
            if (opt) renderOptimizationStats(opt);
            if (comp) renderCompressionStats(comp);
            loadMissionHistory();
            if (system) {
                if (!Charts.cpu) Charts.cpu = createGauge('cpu-chart', system.cpu?.usage_percent || 0);
                updateGauge(Charts.cpu, 'cpu-val', system.cpu?.usage_percent || 0);
                if (!Charts.ram) Charts.ram = createGauge('ram-chart', system.memory?.used_percent || 0);
                updateGauge(Charts.ram, 'ram-val', system.memory?.used_percent || 0);
                if (!Charts.disk) Charts.disk = createGauge('disk-chart', system.disk?.used_percent || 0);
                updateGauge(Charts.disk, 'disk-val', system.disk?.used_percent || 0);
                renderSystemStats(system);
            }
            if (budget) {
                renderBudget(budget, credits);
                if (budget.enabled) {
                    if (Charts.budget) { Charts.budget.destroy(); Charts.budget = null; }
                    Charts.budget = createBudgetDoughnut('budget-chart', budget.spent_usd || 0, budget.daily_limit_usd || 0);
                    if (Charts.budgetModels) { Charts.budgetModels.destroy(); Charts.budgetModels = null; }
                    Charts.budgetModels = createBudgetModelsChart('budget-models-chart', budget.models || {});
                }
            }
        }

        async function loadTabAgent() {
            // Set loading state on agent cards
            CardState.setLoading('card-personality');
            CardState.setLoading('card-memory');

            const results = await Promise.all([
                API.getWithStatus('/api/personality/state'),
                API.getWithStatus('/api/dashboard/mood-history?hours=' + currentMoodHours),
                API.getWithStatus('/api/dashboard/emotion-history?hours=' + currentMoodHours),
                API.getWithStatus('/api/dashboard/memory'),
            ]);
            const [personalityR, moodHistoryR, emotionHistoryR, memDataR] = results;
            const personality = personalityR.ok ? personalityR.data : null;
            const moodHistory = moodHistoryR.ok ? moodHistoryR.data : null;
            const emotionHistory = emotionHistoryR.ok ? emotionHistoryR.data : null;
            const memData = memDataR.ok ? memDataR.data : null;

            personalityR.ok ? CardState.setLoaded('card-personality') : CardState.setError('card-personality', loadTabAgent, { status: personalityR.status });
            memDataR.ok ? CardState.setLoaded('card-memory') : CardState.setError('card-memory', loadTabAgent, { status: memDataR.status });

            if (personality) {
                renderMoodBadge(personality);
                if (personality.enabled) {
                    if (Charts.traits) { Charts.traits.destroy(); Charts.traits = null; }
                    Charts.traits = createRadarChart('traits-chart', personality.traits);
                }
            }
            if (Charts.mood) { Charts.mood.destroy(); Charts.mood = null; }
            Charts.mood = createMoodLineChart('mood-chart', moodHistory || []);
            renderEmotionHistory(emotionHistory, personality);
            if (memData) {
                renderMemoryStats(memData);
                renderMemoryHealth(memData);
                if (Charts.memory) { Charts.memory.destroy(); Charts.memory = null; }
                Charts.memory = createMemoryBarChart('memory-chart', memData);
                renderMilestones(memData.milestones);
            }
            loadErrorPatterns();
        }

        async function loadTabUser() {
            // Set loading state on user cards
            CardState.setLoading('card-profile');
            CardState.setLoading('card-activity-overview');
            CardState.setLoading('card-journal');

            const results = await Promise.all([
                API.getWithStatus('/api/dashboard/profile'),
                API.getWithStatus('/api/memory/activity-overview?days=7')
            ]);
            const [profileR, activityOverviewR] = results;
            const profile = profileR.ok ? profileR.data : null;
            const activityOverview = activityOverviewR.ok ? activityOverviewR.data : null;

            profileR.ok ? CardState.setLoaded('card-profile') : CardState.setError('card-profile', loadTabUser, { status: profileR.status });
            activityOverviewR.ok ? CardState.setLoaded('card-activity-overview') : CardState.setError('card-activity-overview', loadTabUser, { status: activityOverviewR.status });

            if (profile) renderProfile(profile);
            if (activityOverview) renderActivityOverview(activityOverview);
            loadJournal();
        }

        async function loadTabKnowledge() {
            const kgCards = [
                'card-knowledge-graph-summary',
                'card-knowledge-graph-health',
                'card-knowledge-graph-quality',
                'card-knowledge-graph-results',
                'card-knowledge-graph-visual',
            ];
            kgCards.forEach((id) => CardState.setLoading(id));

            const results = await Promise.all([
                API.getWithStatus('/api/knowledge-graph/important?limit=30&min_score=15'),
                API.getWithStatus('/api/knowledge-graph/stats'),
                API.getWithStatus('/api/knowledge-graph/quality?limit=6'),
                API.getWithStatus('/api/knowledge-graph/health'),
            ]);
            const [importantR, statsR, qualityR, healthR] = results;
            const anyOk = results.some((r) => r.ok);
            if (!anyOk) {
                const status = importantR.status || statsR.status || 0;
                kgCards.forEach((id) => CardState.setError(id, loadTabKnowledge, { status }));
                return;
            }

            const important = importantR.ok ? importantR.data : null;
            const stats = statsR.ok ? statsR.data : null;
            const quality = qualityR.ok ? qualityR.data : null;
            const health = healthR.ok ? healthR.data : null;

            KnowledgeGraphState.importantNodes = Array.isArray(important) ? important : [];
            const importantIDs = new Set(KnowledgeGraphState.importantNodes.map(n => n.id));

            let allEdges = [];
            if (importantIDs.size > 0) {
                const edgeResp = await API.get('/api/knowledge-graph/edges?limit=200');
                allEdges = Array.isArray(edgeResp) ? edgeResp.filter(e => e.relation !== 'co_mentioned_with') : [];
            }
            KnowledgeGraphState.importantEdges = allEdges;
            KnowledgeGraphState.stats = stats || null;

            const displayNodes = KnowledgeGraphState.showAll
                ? await loadAllNodes()
                : KnowledgeGraphState.importantNodes;

            KnowledgeGraphState.nodes = displayNodes;
            KnowledgeGraphState.edges = filterEdgesForDisplay(allEdges, displayNodes);

            renderKnowledgeGraphSummary(KnowledgeGraphState.importantNodes, KnowledgeGraphState.importantEdges, KnowledgeGraphState.stats);
            renderKnowledgeGraphHealth(health || {});
            renderKnowledgeGraphQuality(quality || {});
            renderKnowledgeGraphLists(KnowledgeGraphState.nodes, KnowledgeGraphState.edges);
            renderKnowledgeGraphVisual();
            renderKnowledgeGraphFilters();

            if (KnowledgeGraphState.focusNodeId) {
                await loadKnowledgeGraphNodeDetail(KnowledgeGraphState.focusNodeId);
            } else {
                renderKnowledgeGraphDetailEmpty();
            }

            const searchInput = document.getElementById('knowledge-search-input');
            const query = searchInput ? searchInput.value.trim() : '';
            if (query) {
                await executeKnowledgeGraphSearch(query);
            } else {
                renderKnowledgeGraphSearchState('');
            }

            kgCards.forEach((id) => CardState.setLoaded(id));
        }

        async function loadAllNodes() {
            const nodes = await API.get('/api/knowledge-graph/nodes?limit=200');
            return Array.isArray(nodes) ? nodes : [];
        }

        function filterEdgesForDisplay(edges, nodes) {
            if (!Array.isArray(edges) || !Array.isArray(nodes)) return [];
            const nodeIDs = new Set(nodes.map(n => n.id));
            return edges.filter(e => nodeIDs.has(e.source) && nodeIDs.has(e.target));
        }

        async function loadTabSystem() {
            // Set loading state on system cards
            CardState.setLoading('card-operations');
            CardState.setLoading('card-helper-llm');
            CardState.setLoading('card-activity');
            CardState.setLoading('card-prompt');
            CardState.setLoading('card-logs');

            const results = await Promise.all([
                API.getWithStatus('/api/dashboard/activity'),
                API.getWithStatus('/api/dashboard/prompt-stats'),
                API.getWithStatus('/api/dashboard/logs?lines=100'),
                API.getWithStatus('/api/dashboard/github-repos'),
                API.getWithStatus('/api/dashboard/overview'),
                API.getWithStatus('/api/dashboard/tool-stats'),
            ]);
            const [activityR, promptStatsR, logResultsR, githubReposR, overviewR, toolStatsR] = results;
            const activity = activityR.ok ? activityR.data : null;
            const promptStats = promptStatsR.ok ? promptStatsR.data : null;
            const logResults = logResultsR.ok ? logResultsR.data : null;
            const githubRepos = githubReposR.ok ? githubReposR.data : null;
            const overview = overviewR.ok ? overviewR.data : null;
            const toolStats = toolStatsR.ok ? toolStatsR.data : null;

            overviewR.ok ? CardState.setLoaded('card-operations') : CardState.setError('card-operations', loadTabSystem, { status: overviewR.status });
            activityR.ok ? CardState.setLoaded('card-activity') : CardState.setError('card-activity', loadTabSystem, { status: activityR.status });
            promptStatsR.ok ? CardState.setLoaded('card-prompt') : CardState.setError('card-prompt', loadTabSystem, { status: promptStatsR.status });
            logResultsR.ok ? CardState.setLoaded('card-logs') : CardState.setError('card-logs', loadTabSystem, { status: logResultsR.status });

            if (overview) {
                renderOperations(overview);
                renderIntegrations(overview);
            }
            loadGuardianCard();
            loadHelperLLMCard();
            loadDaemonsCard();
            if (activity) renderActivity(activity);
            if (promptStats) {
                renderPromptStats(promptStats);
                if (Charts.promptSize) Charts.promptSize.destroy();
                Charts.promptSize = createPromptSizeChart('prompt-size-chart', promptStats.recent);
                if (Charts.promptTier) Charts.promptTier.destroy();
                Charts.promptTier = createPromptTierChart('prompt-tier-chart', promptStats.tier_distribution);
                if (Charts.promptSavings) Charts.promptSavings.destroy();
                Charts.promptSavings = createPromptSavingsChart('prompt-savings-chart', promptStats.recent);
                if (Charts.promptSectionDist) Charts.promptSectionDist.destroy();
                Charts.promptSectionDist = createPromptSectionDistChart('prompt-section-dist-chart', promptStats.avg_section_sizes);
            }
            if (toolStats) {
                renderAdaptiveToolStats(toolStats);
                renderToolingTelemetry(toolStats);
            }
            if (logResults) {
                renderLogs(logResults);
                scrollLogsToBottom();
            }
            if (githubRepos) renderGitHubRepos(githubRepos);
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

        async function initDashboard() {
            // Set up tab click listeners
            document.querySelectorAll('.dash-tab').forEach(btn => {
                btn.addEventListener('click', () => showTab(btn.dataset.tab));
            });
            // Set up tab keyboard navigation (arrow keys, Home, End)
            initTabKeyboardNav();

            // Set up log viewer listeners (elements are in DOM even when tab is hidden)
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
                    const data = await API.get('/api/dashboard/logs?lines=100');
                    renderLogs(data);
                    scrollLogsToBottom();
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

            // Determine initial tab from hash or localStorage
            const hashTab = window.location.hash.replace('#', '');
            const savedTab = localStorage.getItem('aurago-dash-tab') || 'overview';
            const initialTab = VALID_TABS.includes(hashTab) ? hashTab : (VALID_TABS.includes(savedTab) ? savedTab : 'overview');

            // Show initial tab – showTab() invokes loadTabContent() synchronously when
            // TabState.loaded[tabId] is false, and loadTabContent() marks the tab loaded before
            // its async body runs, so no defensive double-load is required here.
            showTab(initialTab);

            // Always load agent banner regardless of initial tab
            if (initialTab !== 'overview') {
                loadAgentBanner();
            }
        }

        // Note: live system, mission, memory, personality, daemon, and audit updates flow through
        // SSE handlers registered in dashboard-events.js#connectSSE; no client-side polling is needed.

