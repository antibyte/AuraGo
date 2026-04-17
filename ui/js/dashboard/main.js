// AuraGo – dashboard page logic
// Extracted from dashboard.html

        // ══════════════════════════════════════════════════════════════════════════════
        // AuraGo Dashboard – JavaScript
        // ══════════════════════════════════════════════════════════════════════════════

        const LANG = document.documentElement.lang || 'en';
        const HIDDEN_INTEGRATIONS = new Set(['onedrive']);

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
                if (key) el.textContent = t(key);
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
            get: url => fetch(url, { credentials: 'same-origin' }).then(r => {
                if (r.status === 401) {
                    window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
                    return null;
                }
                return r.ok ? r.json() : null;
            }).catch(() => {
                if (!_apiErrorShown && typeof showToast === 'function') {
                    _apiErrorShown = true;
                    showToast(t('dashboard.api_error') || 'Dashboard data could not be loaded', 'error', 5000);
                    setTimeout(() => { _apiErrorShown = false; }, 15000);
                }
                return null;
            }),
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

        let currentMoodHours = 24;

        // ══════════════════════════════════════════════════════════════════════════════
        // TAB SYSTEM
        // ══════════════════════════════════════════════════════════════════════════════

        const TabState = { active: 'overview', loaded: {} };
        const KnowledgeGraphState = { nodes: [], edges: [], importantNodes: [], importantEdges: [], stats: null, focusNodeId: '', focusPayload: null, editingNodeId: '', editingEdgeKey: '', showAll: false, filterType: '', filterSource: '', modalKind: '', modalNodeId: '', modalTriggerEl: null };
        const VALID_TABS = ['overview', 'agent', 'user', 'knowledge', 'filesync', 'system'];
        function dashSetHidden(el, hidden) {
            if (!el) return;
            el.classList.toggle('is-hidden', hidden);
        }

        function showTab(tabId) {
            if (!VALID_TABS.includes(tabId)) tabId = 'overview';
            localStorage.setItem('aurago-dash-tab', tabId);
            history.replaceState(null, '', '#' + tabId);
            document.querySelectorAll('.dash-tab').forEach(btn => {
                btn.classList.toggle('active', btn.dataset.tab === tabId);
            });
            document.querySelectorAll('.dash-tab-panel').forEach(panel => {
                dashSetHidden(panel, panel.id !== 'tab-' + tabId);
            });
            TabState.active = tabId;
            if (tabId === 'system' && TabState.loaded[tabId]) {
                loadGuardianCard();
                loadHelperLLMCard();
            }
            if (tabId === 'knowledge' && TabState.loaded[tabId]) {
                loadTabKnowledge();
            }
            if (tabId === 'filesync' && TabState.loaded[tabId]) {
                loadFileSyncStatus();
            }
            if (!TabState.loaded[tabId]) {
                loadTabContent(tabId);
            }
        }

        async function loadTabContent(tabId) {
            TabState.loaded[tabId] = true;
            switch (tabId) {
                case 'overview': return loadTabOverview();
                case 'agent':    return loadTabAgent();
                case 'user':     return loadTabUser();
                case 'knowledge': return loadTabKnowledge();
                case 'filesync':  return loadFileSyncStatus();
                case 'system':   return loadTabSystem();
            }
        }

        async function loadAgentBanner() {
            const overview = await API.get('/api/dashboard/overview');
            if (overview) {
                renderAgentBanner(overview, overview?.context?.total_chars);
            }
        }

        async function loadTabOverview() {
            const [system, budget, overview, credits, opt, comp] = await Promise.all([
                API.get('/api/dashboard/system'),
                API.get('/api/budget'),
                API.get('/api/dashboard/overview'),
                API.get('/api/credits'),
                API.get('/api/dashboard/optimization'),
                API.get('/api/dashboard/compression'),
            ]);
            renderAgentBanner(overview, overview?.context?.total_chars);
            renderQuickStatus(overview);
            renderOptimizationStats(opt);
            renderCompressionStats(comp);
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
            const [personality, moodHistory, emotionHistory, memData] = await Promise.all([
                API.get('/api/personality/state'),
                API.get('/api/dashboard/mood-history?hours=' + currentMoodHours),
                API.get('/api/dashboard/emotion-history?hours=' + currentMoodHours),
                API.get('/api/dashboard/memory'),
            ]);
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
            const [profile, activityOverview] = await Promise.all([
                API.get('/api/dashboard/profile'),
                API.get('/api/memory/activity-overview?days=7')
            ]);
            renderProfile(profile);
            renderActivityOverview(activityOverview);
            loadJournal();
        }

        async function loadTabKnowledge() {
            const [important, stats, quality] = await Promise.all([
                API.get('/api/knowledge-graph/important?limit=30&min_score=15'),
                API.get('/api/knowledge-graph/stats'),
                API.get('/api/knowledge-graph/quality?limit=6'),
            ]);

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
            const [activity, promptStats, logResults, githubRepos, overview, toolStats] = await Promise.all([
                API.get('/api/dashboard/activity'),
                API.get('/api/dashboard/prompt-stats'),
                API.get('/api/dashboard/logs?lines=100'),
                API.get('/api/dashboard/github-repos'),
                API.get('/api/dashboard/overview'),
                API.get('/api/dashboard/tool-stats'),
            ]);
            renderOperations(overview);
            renderIntegrations(overview);
            loadGuardianCard();
            loadHelperLLMCard();
            loadDaemonsCard();
            renderActivity(activity);
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
            renderAdaptiveToolStats(toolStats);
            renderToolingTelemetry(toolStats);
            renderLogs(logResults);
            scrollLogsToBottom();
            renderGitHubRepos(githubRepos);
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

            // Set up log viewer listeners (elements are in DOM even when tab is hidden)
            const logFilter = document.getElementById('log-filter');
            const logScrollBtn = document.getElementById('log-scroll-btn');
            const logRefreshBtn = document.getElementById('log-refresh-btn');
            if (logFilter) logFilter.addEventListener('input', applyLogFilter);
            if (logScrollBtn) logScrollBtn.addEventListener('click', scrollLogsToBottom);
            if (logRefreshBtn) logRefreshBtn.addEventListener('click', async () => {
                const data = await API.get('/api/dashboard/logs?lines=100');
                renderLogs(data);
                scrollLogsToBottom();
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
            document.addEventListener('click', (e) => {
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

            // Show initial tab – triggers lazy load
            showTab(initialTab);

            // Force-load initial tab content in case it wasn't loaded yet
            // (handles edge case where page opens directly to a non-overview tab)
            if (!TabState.loaded[initialTab]) {
                TabState.loaded[initialTab] = true;
                switch (initialTab) {
                    case 'overview': loadTabOverview(); break;
                    case 'agent':    loadTabAgent();    break;
                    case 'user':     loadTabUser();     break;
                    case 'knowledge': loadTabKnowledge(); break;
                    case 'system':   loadTabSystem();   break;
                }
            }

            // Start auto-refresh
            startAutoRefresh();

            // Always load agent banner regardless of initial tab
            if (initialTab !== 'overview') {
                loadAgentBanner();
            }
        }

        // ── Auto-Refresh ────────────────────────────────────────────────────────────
        function startAutoRefresh() {
            // System metrics are now pushed via SSE (EventSystemMetrics) every 10s.
            // Mission updates are pushed via SSE (EventMissionUpdate) on state change.
            // No interval polling needed; SSE handles all live updates.
        }

