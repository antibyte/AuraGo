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
        const KnowledgeGraphState = { nodes: [], edges: [], importantNodes: [], importantEdges: [], stats: null, focusNodeId: '', focusPayload: null, editingNodeId: '', editingEdgeKey: '', showAll: false, filterType: '', filterSource: '' };
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

        function renderKnowledgeGraphSummary(nodes, edges, stats) {
            const grid = document.getElementById('knowledge-summary-grid');
            if (!grid) return;

            const totalNodes = stats?.total_nodes ?? nodes.length;
            const totalEdges = stats?.total_edges ?? edges.length;
            const meaningfulEdges = stats?.meaningful_edges ?? edges.length;

            const types = new Map();
            (nodes || []).forEach(node => {
                const type = node?.properties?.type || 'untyped';
                types.set(type, (types.get(type) || 0) + 1);
            });

            const statsHTML = `
                <div class="mem-stat">
                    <div class="mem-stat-val">${esc(String(totalNodes))}</div>
                    <div class="mem-stat-lbl">${t('dashboard.knowledge_nodes')}</div>
                </div>
                <div class="mem-stat">
                    <div class="mem-stat-val">${esc(String(meaningfulEdges))}</div>
                    <div class="mem-stat-lbl">${t('dashboard.knowledge_meaningful_edges')}</div>
                </div>
                <div class="mem-stat">
                    <div class="mem-stat-val">${esc(String(types.size))}</div>
                    <div class="mem-stat-lbl">${t('dashboard.knowledge_types')}</div>
                </div>
            `;

            const typeBadges = Array.from(types.entries())
                .sort((a, b) => b[1] - a[1])
                .slice(0, 6)
                .map(([type, count]) => {
                    const color = knowledgeGraphTypeColor(type);
                    return `<span class="knowledge-type-badge" style="--badge-color: ${color}">${esc(type)} (${count})</span>`;
                }).join('');

            grid.innerHTML = statsHTML + (typeBadges ? `<div class="knowledge-type-badges">${typeBadges}</div>` : '');
        }

        function renderKnowledgeGraphQuality(report) {
            const metrics = document.getElementById('knowledge-quality-metrics');
            const isolated = document.getElementById('knowledge-quality-isolated');
            const untyped = document.getElementById('knowledge-quality-untyped');
            const duplicates = document.getElementById('knowledge-quality-duplicates');
            if (!metrics || !isolated || !untyped || !duplicates) return;

            const stats = [
                { val: Number(report?.protected_nodes || 0), lbl: t('dashboard.knowledge_quality_protected') },
                { val: Number(report?.isolated_nodes || 0), lbl: t('dashboard.knowledge_quality_isolated') },
                { val: Number(report?.untyped_nodes || 0), lbl: t('dashboard.knowledge_quality_untyped') },
                { val: Number(report?.duplicate_groups || 0), lbl: t('dashboard.knowledge_quality_duplicates') },
            ];
            metrics.innerHTML = stats.map(stat => `
                <div class="mem-stat">
                    <div class="mem-stat-val">${esc(String(stat.val))}</div>
                    <div class="mem-stat-lbl">${esc(stat.lbl)}</div>
                </div>
            `).join('');

            renderKnowledgeGraphQualityNodeList(isolated, report?.isolated_sample, 'dashboard.knowledge_quality_empty_isolated');
            renderKnowledgeGraphQualityNodeList(untyped, report?.untyped_sample, 'dashboard.knowledge_quality_empty_untyped');

            const candidates = Array.isArray(report?.duplicate_candidates) ? report.duplicate_candidates : [];
            if (!candidates.length) {
                duplicates.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_quality_empty_duplicates')}</div>`;
            } else {
                duplicates.innerHTML = renderCollapsibleList(candidates, candidate => `
                    <div class="knowledge-item">
                        <div class="knowledge-item-head">
                            <span class="knowledge-item-title">${escapeHtml(candidate.label || candidate.normalized_label || 'Node')}</span>
                            <span class="knowledge-item-badge">${escapeHtml(t('dashboard.knowledge_quality_duplicate_count', { count: Number(candidate.count || 0) }))}</span>
                        </div>
                        <div class="knowledge-item-props">
                            ${(Array.isArray(candidate.ids) ? candidate.ids : []).map(id => `
                                <span class="knowledge-inline-link" data-kg-node-id="${escapeHtml(id || '')}">${escapeHtml(id || '')}</span>
                            `).join('')}
                        </div>
                    </div>
                `, 5);
            }
        }

        function renderKnowledgeGraphQualityNodeList(container, nodes, emptyKey) {
            if (!container) return;
            if (!Array.isArray(nodes) || nodes.length === 0) {
                container.innerHTML = `<div class="empty-state">${t(emptyKey)}</div>`;
                return;
            }
            container.innerHTML = renderCollapsibleList(nodes, node => {
                const props = renderKnowledgeProps(node.properties);
                return `
                    <div class="knowledge-item clickable" data-kg-node-id="${escapeHtml(node.id || '')}">
                        <div class="knowledge-item-head">
                            <span class="knowledge-item-title">${escapeHtml(node.label || node.id || 'Node')}</span>
                            <span class="knowledge-item-badge">${escapeHtml(node.id || '')}</span>
                        </div>
                        ${props ? `<div class="knowledge-item-props">${props}</div>` : ''}
                    </div>
                `;
            }, 5);
        }

        function renderKnowledgeGraphLists(nodes, edges) {
            renderKnowledgeNodeList(document.getElementById('knowledge-node-list'), nodes);
            renderKnowledgeEdgeList(document.getElementById('knowledge-edge-list'), edges);
        }

        function renderKnowledgeNodeList(container, nodes) {
            if (!container) return;
            if (!Array.isArray(nodes) || nodes.length === 0) {
                container.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_empty')}</div>`;
                return;
            }
            container.innerHTML = renderCollapsibleList(nodes, node => {
                const props = renderKnowledgeProps(node.properties);
                const score = node.importance_score;
                const scoreBadge = (typeof score === 'number')
                    ? `<span class="knowledge-item-badge knowledge-score-badge" title="${t('dashboard.knowledge_importance_score')}">${score}</span>`
                    : '';
                const typeBadge = node?.properties?.type
                    ? `<span class="knowledge-type-dot" style="--dot-color: ${knowledgeGraphTypeColor(node.properties.type)}"></span>`
                    : '';
                return `
                    <div class="knowledge-item clickable" data-kg-node-id="${escapeHtml(node.id || '')}">
                        <div class="knowledge-item-head">
                            ${typeBadge}
                            <span class="knowledge-item-title">${escapeHtml(node.label || node.id || 'Node')}</span>
                            <span class="knowledge-item-badge">${escapeHtml(node.id || '')}</span>
                            ${scoreBadge}
                        </div>
                        ${props ? `<div class="knowledge-item-props">${props}</div>` : ''}
                    </div>
                `;
            }, 8);
        }

        function renderKnowledgeEdgeList(container, edges) {
            if (!container) return;
            if (!Array.isArray(edges) || edges.length === 0) {
                container.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_empty')}</div>`;
                return;
            }
            container.innerHTML = renderCollapsibleList(edges, edge => {
                const props = renderKnowledgeProps(edge.properties);
                return `
                    <div class="knowledge-item">
                        <div class="knowledge-item-head">
                            <span class="knowledge-item-title">${escapeHtml(edge.source || '')} → ${escapeHtml(edge.target || '')}</span>
                            <span class="knowledge-item-badge">${escapeHtml(edge.relation || '')}</span>
                        </div>
                        ${props ? `<div class="knowledge-item-props">${props}</div>` : ''}
                    </div>
                `;
            }, 8);
        }

        function renderKnowledgeProps(properties) {
            if (!properties || typeof properties !== 'object') return '';
            const entries = Object.entries(properties)
                .filter(([key, value]) => value && !['source', 'extracted_at'].includes(String(key)))
                .slice(0, 4);
            if (!entries.length) return '';
            return entries.map(([key, value]) => `
                <span class="knowledge-item-prop">${escapeHtml(String(key))}: ${escapeHtml(truncate(String(value), 42))}</span>
            `).join('');
        }

        function renderKnowledgeGraphSearchState(query, payload) {
            const meta = document.getElementById('knowledge-search-meta');
            const results = document.getElementById('knowledge-search-results');
            if (!meta || !results) return;

            if (!query) {
                meta.textContent = t('dashboard.knowledge_search_hint');
                results.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_no_search')}</div>`;
                return;
            }

            const nodes = Array.isArray(payload?.nodes) ? payload.nodes : [];
            const edges = Array.isArray(payload?.edges) ? payload.edges : [];
            meta.textContent = t('dashboard.knowledge_search_meta', { query, nodes: nodes.length, edges: edges.length });

            if (!nodes.length && !edges.length) {
                results.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_search_empty')}</div>`;
                return;
            }

            const nodeBlock = nodes.map(node => `
                <div class="knowledge-item clickable" data-kg-node-id="${escapeHtml(node.id || '')}">
                    <div class="knowledge-item-head">
                        <span class="knowledge-item-title">${escapeHtml(node.label || node.id || 'Node')}</span>
                        <span class="knowledge-item-badge">${escapeHtml(t('dashboard.knowledge_nodes'))}</span>
                    </div>
                    <div class="knowledge-item-meta">${escapeHtml(node.id || '')}</div>
                    ${renderKnowledgeProps(node.properties) ? `<div class="knowledge-item-props">${renderKnowledgeProps(node.properties)}</div>` : ''}
                </div>
            `).join('');
            const edgeBlock = edges.map(edge => `
                <div class="knowledge-item">
                    <div class="knowledge-item-head">
                        <span class="knowledge-item-title">${escapeHtml(edge.source || '')} → ${escapeHtml(edge.target || '')}</span>
                        <span class="knowledge-item-badge">${escapeHtml(edge.relation || '')}</span>
                    </div>
                    ${renderKnowledgeProps(edge.properties) ? `<div class="knowledge-item-props">${renderKnowledgeProps(edge.properties)}</div>` : ''}
                </div>
            `).join('');
            results.innerHTML = nodeBlock + edgeBlock;
        }

        async function executeKnowledgeGraphSearch(query) {
            const payload = await API.get('/api/knowledge-graph/search?q=' + encodeURIComponent(query));
            renderKnowledgeGraphSearchState(query, payload || { nodes: [], edges: [] });
        }

        function renderKnowledgeGraphDetailEmpty() {
            const panel = document.getElementById('knowledge-detail-panel');
            if (!panel) return;
            panel.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_detail_empty')}</div>`;
        }

        async function loadKnowledgeGraphNodeDetail(nodeID) {
            const panel = document.getElementById('knowledge-detail-panel');
            if (!panel || !nodeID) return;

            panel.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_detail_loading')}</div>`;
            const payload = await API.get('/api/knowledge-graph/node?id=' + encodeURIComponent(nodeID) + '&limit=20');
            const node = payload?.node;
            const neighbors = Array.isArray(payload?.neighbors) ? payload.neighbors : [];
            const edges = Array.isArray(payload?.edges) ? payload.edges : [];

            if (!node) {
                KnowledgeGraphState.focusNodeId = '';
                KnowledgeGraphState.focusPayload = null;
                KnowledgeGraphState.editingEdgeKey = '';
                renderKnowledgeGraphVisual();
                panel.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_detail_missing')}</div>`;
                return;
            }

            KnowledgeGraphState.focusNodeId = node.id || nodeID;
            KnowledgeGraphState.focusPayload = payload;
            renderKnowledgeGraphVisual();

            const isEditing = KnowledgeGraphState.editingNodeId === node.id;
            const isProtected = !!node.protected;
            const filteredNodeProperties = filterKnowledgeGraphEditableProperties(node.properties);
            const editingEdge = edges.find(edge => knowledgeGraphEdgeIdentity(edge) === KnowledgeGraphState.editingEdgeKey) || null;

            const nodeProps = Object.entries(filteredNodeProperties).map(([key, value]) => `
                <div class="knowledge-detail-row"><strong>${escapeHtml(String(key))}</strong>: ${escapeHtml(String(value))}</div>
            `).join('') || `<div class="knowledge-detail-row">${t('dashboard.knowledge_empty')}</div>`;

            const neighborRows = neighbors.map(neighbor => `
                <div class="knowledge-detail-row clickable" data-kg-node-id="${escapeHtml(neighbor.id || '')}"><strong>${escapeHtml(neighbor.label || neighbor.id || '')}</strong> <span class="knowledge-detail-id">${escapeHtml(neighbor.id || '')}</span></div>
            `).join('') || `<div class="knowledge-detail-row">${t('dashboard.knowledge_detail_no_neighbors')}</div>`;

            const edgeRows = edges.map(edge => `
                <div class="knowledge-detail-row knowledge-edge-row">
                    <div><strong>${escapeHtml(edge.relation || '')}</strong>: ${escapeHtml(edge.source || '')} → ${escapeHtml(edge.target || '')}</div>
                    <div class="knowledge-detail-actions">
                        <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphEdgeEdit('${escapeJsString(knowledgeGraphEdgeIdentity(edge))}')">${KnowledgeGraphState.editingEdgeKey === knowledgeGraphEdgeIdentity(edge) ? t('dashboard.knowledge_action_cancel') : t('dashboard.knowledge_action_edit')}</button>
                        <button type="button" class="btn btn-danger btn-sm" onclick="deleteKnowledgeGraphEdge('${escapeJsString(edge.source || '')}', '${escapeJsString(edge.target || '')}', '${escapeJsString(edge.relation || '')}')">${t('dashboard.knowledge_edge_delete')}</button>
                    </div>
                </div>
            `).join('') || `<div class="knowledge-detail-row">${t('dashboard.knowledge_detail_no_edges')}</div>`;

            panel.innerHTML = `
                <div class="knowledge-detail-head">
                    <div>
                        <div class="knowledge-detail-title">${escapeHtml(node.label || node.id || 'Node')}</div>
                        <div class="knowledge-detail-id">${escapeHtml(node.id || '')}</div>
                    </div>
                    <div class="knowledge-detail-actions">
                        ${isProtected ? `<span class="knowledge-item-badge knowledge-item-badge-protected">${t('dashboard.knowledge_detail_protected')}</span>` : ''}
                        <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphEdit('${escapeJsString(node.id || '')}')">${isEditing ? t('dashboard.knowledge_action_cancel') : t('dashboard.knowledge_action_edit')}</button>
                        <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphProtection('${escapeJsString(node.id || '')}', ${isProtected ? 'false' : 'true'})">${isProtected ? t('dashboard.knowledge_action_unprotect') : t('dashboard.knowledge_action_protect')}</button>
                        <button type="button" class="btn btn-danger btn-sm" onclick="deleteKnowledgeGraphNode('${escapeJsString(node.id || '')}', '${escapeJsString(node.label || node.id || 'Node')}')">${t('dashboard.knowledge_action_delete')}</button>
                    </div>
                </div>
                ${renderKnowledgeGraphNodeEditor(node, isEditing)}
                ${renderKnowledgeGraphEdgeEditor(editingEdge)}
                <div class="knowledge-detail-grid">
                    <div class="knowledge-detail-section">
                        <h4 class="card-section-title">${t('dashboard.knowledge_detail_properties')}</h4>
                        <div class="knowledge-detail-list">${nodeProps}</div>
                    </div>
                    <div class="knowledge-detail-section">
                        <h4 class="card-section-title">${t('dashboard.knowledge_detail_neighbors')}</h4>
                        <div class="knowledge-detail-list">${neighborRows}</div>
                    </div>
                    <div class="knowledge-detail-section">
                        <h4 class="card-section-title">${t('dashboard.knowledge_detail_edges')}</h4>
                        <div class="knowledge-detail-list">${edgeRows}</div>
                    </div>
                </div>
            `;
        }

        function resetKnowledgeGraphFocus() {
            KnowledgeGraphState.focusNodeId = '';
            KnowledgeGraphState.focusPayload = null;
            KnowledgeGraphState.editingNodeId = '';
            KnowledgeGraphState.editingEdgeKey = '';
            renderKnowledgeGraphVisual();
            renderKnowledgeGraphDetailEmpty();
        }

        function filterKnowledgeGraphEditableProperties(properties) {
            const out = {};
            Object.entries(properties || {}).forEach(([key, value]) => {
                if (!key || ['protected', 'access_count'].includes(String(key))) return;
                out[key] = value;
            });
            return out;
        }

        function renderKnowledgeGraphNodeEditor(node, isEditing) {
            if (!isEditing) return '';
            const propsJSON = JSON.stringify(filterKnowledgeGraphEditableProperties(node.properties), null, 2);
            return `
                <div class="knowledge-detail-editor">
                    <h4 class="card-section-title">${t('dashboard.knowledge_editor_title')}</h4>
                    <div class="knowledge-editor-grid">
                        <label class="knowledge-editor-field">
                            <span>${t('dashboard.knowledge_editor_label')}</span>
                            <input type="text" id="knowledge-edit-label" class="profile-search" value="${escapeHtml(node.label || '')}">
                        </label>
                        <label class="knowledge-editor-field">
                            <span>${t('dashboard.knowledge_editor_properties')}</span>
                            <textarea id="knowledge-edit-properties" class="knowledge-editor-textarea" spellcheck="false">${escapeHtml(propsJSON)}</textarea>
                        </label>
                    </div>
                    <div class="knowledge-editor-hint">${t('dashboard.knowledge_editor_hint')}</div>
                    <div class="knowledge-detail-actions">
                        <button type="button" class="btn btn-primary btn-sm" onclick="saveKnowledgeGraphNodeEdit('${escapeJsString(node.id || '')}')">${t('dashboard.knowledge_action_save')}</button>
                        <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphEdit('${escapeJsString(node.id || '')}')">${t('dashboard.knowledge_action_cancel')}</button>
                    </div>
                </div>
            `;
        }

        function renderKnowledgeGraphEdgeEditor(edge) {
            if (!edge) return '';
            const propsJSON = JSON.stringify(filterKnowledgeGraphEditableProperties(edge.properties), null, 2);
            return `
                <div class="knowledge-detail-editor">
                    <h4 class="card-section-title">${t('dashboard.knowledge_edge_editor_title')}</h4>
                    <div class="knowledge-editor-grid">
                        <label class="knowledge-editor-field">
                            <span>${t('dashboard.knowledge_edge_editor_relation')}</span>
                            <input type="text" id="knowledge-edit-edge-relation" class="profile-search" value="${escapeHtml(edge.relation || '')}">
                        </label>
                        <label class="knowledge-editor-field">
                            <span>${t('dashboard.knowledge_edge_editor_properties')}</span>
                            <textarea id="knowledge-edit-edge-properties" class="knowledge-editor-textarea" spellcheck="false">${escapeHtml(propsJSON)}</textarea>
                        </label>
                    </div>
                    <div class="knowledge-editor-hint">${t('dashboard.knowledge_editor_hint')}</div>
                    <div class="knowledge-detail-actions">
                        <button type="button" class="btn btn-primary btn-sm" onclick="saveKnowledgeGraphEdgeEdit('${escapeJsString(edge.source || '')}', '${escapeJsString(edge.target || '')}', '${escapeJsString(edge.relation || '')}')">${t('dashboard.knowledge_action_save')}</button>
                        <button type="button" class="btn btn-secondary btn-sm" onclick="toggleKnowledgeGraphEdgeEdit('${escapeJsString(knowledgeGraphEdgeIdentity(edge))}')">${t('dashboard.knowledge_action_cancel')}</button>
                    </div>
                </div>
            `;
        }

        function toggleKnowledgeGraphEdit(nodeID) {
            KnowledgeGraphState.editingNodeId = KnowledgeGraphState.editingNodeId === nodeID ? '' : nodeID;
            if (KnowledgeGraphState.editingNodeId) {
                KnowledgeGraphState.editingEdgeKey = '';
            }
            if (KnowledgeGraphState.focusNodeId) {
                loadKnowledgeGraphNodeDetail(KnowledgeGraphState.focusNodeId);
            }
        }

        function toggleKnowledgeGraphEdgeEdit(edgeKey) {
            KnowledgeGraphState.editingEdgeKey = KnowledgeGraphState.editingEdgeKey === edgeKey ? '' : edgeKey;
            if (KnowledgeGraphState.editingEdgeKey) {
                KnowledgeGraphState.editingNodeId = '';
            }
            if (KnowledgeGraphState.focusNodeId) {
                loadKnowledgeGraphNodeDetail(KnowledgeGraphState.focusNodeId);
            }
        }

        async function saveKnowledgeGraphNodeEdit(nodeID) {
            const labelInput = document.getElementById('knowledge-edit-label');
            const propsInput = document.getElementById('knowledge-edit-properties');
            if (!labelInput || !propsInput || !nodeID) return;

            let properties = {};
            const raw = propsInput.value.trim();
            if (raw) {
                try {
                    properties = JSON.parse(raw);
                } catch (err) {
                    if (typeof showAlert === 'function') {
                        await showAlert(t('dashboard.knowledge_editor_invalid_title'), t('dashboard.knowledge_editor_invalid'));
                    }
                    return;
                }
                if (!properties || Array.isArray(properties) || typeof properties !== 'object') {
                    if (typeof showAlert === 'function') {
                        await showAlert(t('dashboard.knowledge_editor_invalid_title'), t('dashboard.knowledge_editor_invalid'));
                    }
                    return;
                }
                properties = Object.entries(properties).reduce((acc, [key, value]) => {
                    if (!key || value === null || value === undefined) return acc;
                    acc[String(key)] = String(value);
                    return acc;
                }, {});
            }

            const response = await fetch('/api/knowledge-graph/node', {
                method: 'PUT',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    id: nodeID,
                    label: labelInput.value.trim(),
                    properties,
                }),
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_mutation_failed'), 'error', 5000);
                return;
            }

            KnowledgeGraphState.editingNodeId = '';
            if (typeof showToast === 'function') showToast(t('dashboard.knowledge_saved'), 'success', 2500);
            await loadTabKnowledge();
        }

        async function saveKnowledgeGraphEdgeEdit(source, target, relation) {
            const relationInput = document.getElementById('knowledge-edit-edge-relation');
            const propsInput = document.getElementById('knowledge-edit-edge-properties');
            if (!relationInput || !propsInput) return;

            let properties = {};
            const raw = propsInput.value.trim();
            if (raw) {
                try {
                    properties = JSON.parse(raw);
                } catch (err) {
                    if (typeof showAlert === 'function') {
                        await showAlert(t('dashboard.knowledge_editor_invalid_title'), t('dashboard.knowledge_editor_invalid'));
                    }
                    return;
                }
                if (!properties || Array.isArray(properties) || typeof properties !== 'object') {
                    if (typeof showAlert === 'function') {
                        await showAlert(t('dashboard.knowledge_editor_invalid_title'), t('dashboard.knowledge_editor_invalid'));
                    }
                    return;
                }
                properties = Object.entries(properties).reduce((acc, [key, value]) => {
                    if (!key || value === null || value === undefined) return acc;
                    acc[String(key)] = String(value);
                    return acc;
                }, {});
            }

            const response = await fetch('/api/knowledge-graph/edge', {
                method: 'PUT',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    source,
                    target,
                    relation,
                    new_relation: relationInput.value.trim(),
                    properties,
                }),
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_edge_mutation_failed'), 'error', 5000);
                return;
            }

            KnowledgeGraphState.editingEdgeKey = '';
            if (typeof showToast === 'function') showToast(t('dashboard.knowledge_edge_saved'), 'success', 2500);
            await loadTabKnowledge();
        }

        async function toggleKnowledgeGraphProtection(nodeID, shouldProtect) {
            const response = await fetch('/api/knowledge-graph/node/protect', {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ id: nodeID, protected: !!shouldProtect }),
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_mutation_failed'), 'error', 5000);
                return;
            }

            if (typeof showToast === 'function') showToast(shouldProtect ? t('dashboard.knowledge_protected') : t('dashboard.knowledge_unprotected'), 'success', 2500);
            await loadTabKnowledge();
        }

        async function deleteKnowledgeGraphEdge(source, target, relation) {
            const confirmed = typeof showConfirm === 'function'
                ? await showConfirm(t('dashboard.knowledge_edge_delete_title'), t('dashboard.knowledge_edge_delete_confirm', { relation }))
                : true;
            if (!confirmed) return;

            const url = '/api/knowledge-graph/edge?source=' + encodeURIComponent(source) + '&target=' + encodeURIComponent(target) + '&relation=' + encodeURIComponent(relation);
            const response = await fetch(url, {
                method: 'DELETE',
                credentials: 'same-origin',
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_edge_delete_failed'), 'error', 5000);
                return;
            }

            KnowledgeGraphState.editingEdgeKey = '';
            if (typeof showToast === 'function') showToast(t('dashboard.knowledge_edge_deleted'), 'success', 2500);
            await loadTabKnowledge();
        }

        async function deleteKnowledgeGraphNode(nodeID, label) {
            const confirmed = typeof showConfirm === 'function'
                ? await showConfirm(t('dashboard.knowledge_delete_title'), t('dashboard.knowledge_delete_confirm', { label: label || nodeID }))
                : true;
            if (!confirmed) return;

            const response = await fetch('/api/knowledge-graph/node?id=' + encodeURIComponent(nodeID), {
                method: 'DELETE',
                credentials: 'same-origin',
            });
            const payload = await safeReadJSON(response);
            if (!response.ok) {
                if (typeof showToast === 'function') showToast(payload?.error || t('dashboard.knowledge_delete_failed'), 'error', 5000);
                return;
            }

            KnowledgeGraphState.editingNodeId = '';
            KnowledgeGraphState.focusNodeId = '';
            KnowledgeGraphState.focusPayload = null;
            if (typeof showToast === 'function') showToast(t('dashboard.knowledge_deleted'), 'success', 2500);
            await loadTabKnowledge();
        }

        async function safeReadJSON(response) {
            try {
                return await response.json();
            } catch (_) {
                return null;
            }
        }

        function knowledgeGraphEdgeIdentity(edge) {
            return `${edge?.source || ''}::${edge?.target || ''}::${edge?.relation || ''}`;
        }



        function buildKnowledgeGraphOverviewModel(nodes, edges) {
            const safeNodes = dedupeKnowledgeGraphNodes(nodes || []);
            const safeEdges = Array.isArray(edges) ? edges : [];
            if (!safeNodes.length) return null;

            const degree = new Map();
            safeEdges.forEach(edge => {
                if (edge?.source) degree.set(edge.source, (degree.get(edge.source) || 0) + 1);
                if (edge?.target) degree.set(edge.target, (degree.get(edge.target) || 0) + 1);
            });

            const MAX_PER_TYPE = 4;
            const MAX_NODES = 15;
            const typeCount = new Map();
            const selectedNodes = [];

            const sortedNodes = [...safeNodes].sort((a, b) => {
                const scoreA = a.importance_score ?? 0;
                const scoreB = b.importance_score ?? 0;
                if (scoreB !== scoreA) return scoreB - scoreA;
                return String(a.label || a.id || '').localeCompare(String(b.label || b.id || ''));
            });

            for (const node of sortedNodes) {
                if (selectedNodes.length >= MAX_NODES) break;
                const type = node?.properties?.type || '_untyped';
                const currentTypeCount = typeCount.get(type) || 0;
                if (currentTypeCount >= MAX_PER_TYPE) continue;
                typeCount.set(type, currentTypeCount + 1);
                selectedNodes.push(node);
            }

            const selectedIDs = new Set(selectedNodes.map(node => node.id));
            const selectedEdges = safeEdges
                .filter(edge => selectedIDs.has(edge?.source) && selectedIDs.has(edge?.target))
                .slice(0, 25);

            const maxScore = Math.max(1, ...selectedNodes.map(n => n.importance_score ?? 0));

            return {
                nodes: selectedNodes.map(n => {
                    const score = n.importance_score ?? 0;
                    const radius = 10 + (score / maxScore) * 15;
                    return {...n, isFocus: false, r: radius};
                }),
                edges: selectedEdges,
                width: 720,
                height: 360,
                focusNode: null,
            };
        }

        function buildKnowledgeGraphFocusedModel(payload) {
            const node = payload?.node;
            if (!node) return null;

            const neighbors = dedupeKnowledgeGraphNodes((payload?.neighbors || []).slice(0, 10));
            const nodes = [node, ...neighbors];
            const nodeIDs = new Set(nodes.map(item => item.id));
            const edges = (payload?.edges || [])
                .filter(edge => nodeIDs.has(edge?.source) && nodeIDs.has(edge?.target))
                .slice(0, 20);

            return {
                nodes: [
                    {
                        ...node,
                        r: 20,
                        isFocus: true,
                    },
                    ...neighbors.map(n => ({...n, isFocus: false, r: 15}))
                ],
                edges,
                width: 720,
                height: 360,
                focusNode: node,
            };
        }

        function renderKnowledgeGraphVisual() {
            const wrap = document.getElementById('knowledge-graph-visual');
            const mode = document.getElementById('knowledge-graph-mode');
            const caption = document.getElementById('knowledge-graph-caption');
            const resetButton = document.getElementById('knowledge-graph-reset');
            if (!wrap || !mode || !caption || !resetButton) return;

            const focusedModel = buildKnowledgeGraphFocusedModel(KnowledgeGraphState.focusPayload);
            const model = focusedModel || buildKnowledgeGraphOverviewModel(KnowledgeGraphState.nodes, KnowledgeGraphState.edges);

            if (!model || !model.nodes.length) {
                mode.textContent = t('dashboard.knowledge_visual_overview');
                caption.textContent = t('dashboard.knowledge_visual_empty');
                resetButton.classList.add('is-hidden');
                
                // Clear any existing force graph instance
                if (wrap._forceGraph) {
                    wrap._forceGraph._destructor();
                    delete wrap._forceGraph;
                }
                wrap.innerHTML = `<div class="empty-state">${t('dashboard.knowledge_visual_empty')}</div>`;
                return;
            }

            if (focusedModel) {
                const focusLabel = focusedModel.focusNode?.label || focusedModel.focusNode?.id || t('dashboard.knowledge_nodes');
                mode.textContent = t('dashboard.knowledge_visual_focus');
                caption.textContent = t('dashboard.knowledge_visual_focus_caption', { label: focusLabel, neighbors: Math.max(0, focusedModel.nodes.length - 1) });
                resetButton.classList.remove('is-hidden');
            } else {
                mode.textContent = t('dashboard.knowledge_visual_overview');
                caption.textContent = t('dashboard.knowledge_visual_overview_caption', { nodes: model.nodes.length, edges: model.edges.length });
                resetButton.classList.add('is-hidden');
                renderKnowledgeGraphLegend();
            }

            if (!wrap._forceGraph) {
                wrap.innerHTML = '';
                wrap._forceGraph = ForceGraph()(wrap);
            }

            wrap._forceGraph
                .width(wrap.clientWidth || 720)
                .height(wrap.clientHeight || 360)
                .backgroundColor('transparent')
                .graphData({
                    nodes: model.nodes.map(n => {
                        const score = n.importance_score ?? 0;
                        const type = n?.properties?.type || '';
                        return {
                            id: n.id,
                            label: truncate(String(n.label || n.id || 'Node'), 18),
                            meta: buildKnowledgeGraphNodeTooltip(n),
                            type: type,
                            val: (n.r || 15) / 2,
                            color: knowledgeGraphNodeColor(n),
                            isFocus: n.isFocus || false,
                            importance: score
                        };
                    }),
                    links: model.edges.map(e => ({
                        source: e.source,
                        target: e.target,
                        relation: truncate(String(e.relation || ''), 18),
                        relationFull: String(e.relation || '')
                    }))
                })
                .nodeId('id')
                .nodeVal('val')
                .nodeColor('color')
                .nodeLabel('meta')
                .linkDirectionalArrowLength(3.5)
                .linkDirectionalArrowRelPos(1)
                .linkColor(() => cv('--border-subtle') || '#334155')
                .linkLabel('relationFull')
                .nodeCanvasObject((node, ctx, globalScale) => {
                    const fontSize = Math.max(12 / globalScale, 4);
                    let radius = node.val;
                    if (node.isFocus) {
                        radius = node.val * 1.5;
                    }
                    
                    ctx.beginPath();
                    ctx.arc(node.x, node.y, radius, 0, 2 * Math.PI, false);
                    ctx.fillStyle = node.color;
                    ctx.fill();
                    
                    if (node.isFocus) {
                        ctx.lineWidth = 2 / globalScale;
                        ctx.strokeStyle = cv('--text-primary') || '#fff';
                        ctx.stroke();
                    }

                    ctx.font = `${fontSize}px Sans-Serif`;
                    ctx.textAlign = 'center';
                    ctx.textBaseline = 'middle';
                    ctx.fillStyle = node.isFocus ? (cv('--text-primary') || '#f8fafc') : (cv('--text-secondary') || '#94a3b8');

                    const textY = node.y + radius + 4 + fontSize/2;
                    ctx.fillText(node.label, node.x, textY);
                })
                .onNodeClick(node => {
                    KnowledgeGraphState.focusNodeId = node.id;
                    loadKnowledgeGraphNodeDetail(node.id);
                });
            
            // Add a small delay then run a quick pulse layout reset if needed
            setTimeout(() => {
                if (wrap._forceGraph && typeof wrap._forceGraph.zoomToFit === 'function') {
                    wrap._forceGraph.zoomToFit(400, 20);
                }
            }, 300);
        }

        function dedupeKnowledgeGraphNodes(nodes) {
            const seen = new Set();
            return (Array.isArray(nodes) ? nodes : []).filter(node => {
                const id = node?.id;
                if (!id || seen.has(id)) return false;
                seen.add(id);
                return true;
            });
        }

        function buildKnowledgeGraphNodeTooltip(node) {
            const parts = [];
            const label = node?.label || node?.id || 'Node';
            parts.push(label);
            const type = node?.properties?.type;
            if (type) parts.push(`[${type}]`);
            const score = node?.importance_score;
            if (typeof score === 'number') parts.push(`Score: ${score}`);
            const ip = node?.properties?.ip;
            if (ip) parts.push(`IP: ${ip}`);
            const os = node?.properties?.os;
            if (os) parts.push(`OS: ${os}`);
            return parts.join(' | ');
        }

        const KNOWLEDGE_GRAPH_TYPE_COLORS = {
            'device':     '#3b82f6',
            'service':    '#22c55e',
            'person':     '#f59e0b',
            'container':  '#8b5cf6',
            'software':   '#14b8a6',
            'location':   '#ef4444',
            'concept':    '#ec4899',
            'event':      '#f97316',
            'network':    '#06b6d4',
            'organization': '#6366f1',
            '_default':   '#6b7280',
        };

        function knowledgeGraphTypeColor(type) {
            if (!type) return KNOWLEDGE_GRAPH_TYPE_COLORS._default;
            const lower = String(type).toLowerCase();
            for (const [key, color] of Object.entries(KNOWLEDGE_GRAPH_TYPE_COLORS)) {
                if (key !== '_default' && lower.includes(key)) return color;
            }
            return KNOWLEDGE_GRAPH_TYPE_COLORS._default;
        }

        function knowledgeGraphNodeColor(node) {
            if (node?.isFocus) return cv('--accent') || '#3b82f6';
            const type = node?.properties?.type || '';
            return knowledgeGraphTypeColor(type);
        }

        function renderKnowledgeGraphLegend() {
            const legend = document.getElementById('knowledge-graph-legend');
            if (!legend) return;

            const typesInGraph = new Set();
            const allNodes = KnowledgeGraphState.nodes || [];
            allNodes.forEach(n => {
                if (n?.properties?.type) typesInGraph.add(n.properties.type);
            });

            if (typesInGraph.size === 0) {
                legend.innerHTML = '';
                return;
            }

            const entries = Array.from(typesInGraph).sort().map(type => {
                const color = knowledgeGraphTypeColor(type);
                const count = allNodes.filter(n => n?.properties?.type === type).length;
                return `<span class="kg-legend-entry"><span class="kg-legend-dot" style="background:${color}"></span>${esc(type)} (${count})</span>`;
            }).join('');

            legend.innerHTML = `<div class="kg-legend-row">${entries}</div>`;
        }

        function renderKnowledgeGraphFilters() {
            const filterBar = document.getElementById('knowledge-filter-bar');
            if (!filterBar) return;

            const stats = KnowledgeGraphState.stats;
            if (!stats) { filterBar.innerHTML = ''; return; }

            const types = stats.by_type || {};
            const sources = stats.by_source || {};

            const typeOptions = Object.entries(types).sort((a, b) => b[1] - a[1]).map(([type, count]) =>
                `<option value="${escapeHtml(type)}" ${KnowledgeGraphState.filterType === type ? 'selected' : ''}>${escapeHtml(type)} (${count})</option>`
            ).join('');

            const sourceOptions = Object.entries(sources).sort((a, b) => b[1] - a[1]).map(([source, count]) =>
                `<option value="${escapeHtml(source)}" ${KnowledgeGraphState.filterSource === source ? 'selected' : ''}>${escapeHtml(source)} (${count})</option>`
            ).join('');

            filterBar.innerHTML = `
                <label class="kg-filter-toggle">
                    <input type="checkbox" id="kg-show-all" ${KnowledgeGraphState.showAll ? 'checked' : ''} onchange="toggleKnowledgeGraphShowAll()">
                    <span>${t('dashboard.knowledge_filter_show_all')}</span>
                </label>
                <select class="kg-filter-select" id="kg-filter-type" onchange="applyKnowledgeGraphFilters()">
                    <option value="">${t('dashboard.knowledge_filter_all_types')}</option>
                    ${typeOptions}
                </select>
                <select class="kg-filter-select" id="kg-filter-source" onchange="applyKnowledgeGraphFilters()">
                    <option value="">${t('dashboard.knowledge_filter_all_sources')}</option>
                    ${sourceOptions}
                </select>
            `;

            renderKnowledgeGraphLegend();
        }

        function toggleKnowledgeGraphShowAll() {
            const checkbox = document.getElementById('kg-show-all');
            KnowledgeGraphState.showAll = checkbox ? checkbox.checked : false;
            loadTabKnowledge();
        }

        async function applyKnowledgeGraphFilters() {
            const typeSelect = document.getElementById('kg-filter-type');
            const sourceSelect = document.getElementById('kg-filter-source');
            KnowledgeGraphState.filterType = typeSelect ? typeSelect.value : '';
            KnowledgeGraphState.filterSource = sourceSelect ? sourceSelect.value : '';

            let baseNodes = KnowledgeGraphState.showAll
                ? await loadAllNodes()
                : KnowledgeGraphState.importantNodes;

            if (KnowledgeGraphState.filterType) {
                baseNodes = baseNodes.filter(n => n?.properties?.type === KnowledgeGraphState.filterType);
            }
            if (KnowledgeGraphState.filterSource) {
                baseNodes = baseNodes.filter(n => n?.properties?.source === KnowledgeGraphState.filterSource);
            }

            KnowledgeGraphState.nodes = baseNodes;
            KnowledgeGraphState.edges = filterEdgesForDisplay(KnowledgeGraphState.importantEdges, baseNodes);

            renderKnowledgeGraphLists(KnowledgeGraphState.nodes, KnowledgeGraphState.edges);
            renderKnowledgeGraphVisual();
            renderKnowledgeGraphLegend();
        }


        // ══════════════════════════════════════════════════════════════════════════════

        function createGauge(canvasId, value, label) {
            const ctx = document.getElementById(canvasId);
            if (!ctx) return null;
            const accent = cv('--accent');
            const dim = cv('--border-subtle');
            return new Chart(ctx, {
                type: 'doughnut',
                data: {
                    datasets: [{
                        data: [value, 100 - value],
                        backgroundColor: [accent, dim],
                        borderWidth: 0,
                        cutout: '78%',
                    }]
                },
                options: {
                    responsive: true, maintainAspectRatio: true,
                    plugins: { tooltip: { enabled: false } },
                    rotation: -90, circumference: 180,
                }
            });
        }

        function updateGauge(chart, valId, value) {
            if (!chart) return;
            chart.data.datasets[0].data = [value, 100 - value];
            chart.update('none');
            const el = document.getElementById(valId);
            if (el) el.textContent = Math.round(value) + '%';
        }

        function createBudgetDoughnut(canvasId, spent, total) {
            const ctx = document.getElementById(canvasId);
            if (!ctx) return null;
            const remaining = Math.max(0, total - spent);
            return new Chart(ctx, {
                type: 'doughnut',
                data: {
                    labels: [t('dashboard.budget_chart_spent'), t('dashboard.budget_chart_remaining')],
                    datasets: [{
                        data: [spent, remaining],
                        backgroundColor: [cv('--accent'), cv('--border-subtle')],
                        borderWidth: 0, cutout: '72%',
                    }]
                },
                options: {
                    plugins: { tooltip: { enabled: true } },
                }
            });
        }

        function createBudgetModelsChart(canvasId, models) {
            const ctx = document.getElementById(canvasId);
            if (!ctx) return null;
            const labels = Object.keys(models || {});
            const inputData = labels.map(m => (models[m].input_tokens || 0) / 1000);
            const outputData = labels.map(m => (models[m].output_tokens || 0) / 1000);
            return new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [
                        { label: t('dashboard.budget_chart_input'), data: inputData, backgroundColor: cv('--accent') + 'aa', borderRadius: 4 },
                        { label: t('dashboard.budget_chart_output'), data: outputData, backgroundColor: cv('--success') + 'aa', borderRadius: 4 },
                    ]
                },
                options: {
                    indexAxis: 'y',
                    plugins: { legend: { display: true, position: 'top', labels: { boxWidth: 12, padding: 8 } } },
                    scales: {
                        x: { grid: { color: cv('--border-subtle') }, ticks: { color: cv('--text-secondary') } },
                        y: { grid: { display: false }, ticks: { color: cv('--text-primary'), font: { size: 10 } } },
                    }
                }
            });
        }

        function createLLMAvgChart(canvasId, models) {
            const ctx = document.getElementById(canvasId);
            if (!ctx) return null;
            const labels = Object.keys(models || {}).filter(m => (models[m].calls || 0) > 0);
            if (labels.length === 0) return null;
            const avgIn  = labels.map(m => models[m].avg_input_tokens  || 0);
            const avgOut = labels.map(m => models[m].avg_output_tokens || 0);
            return new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [
                        { label: t('dashboard.budget_llm_avg_input'),  data: avgIn,  backgroundColor: cv('--accent') + 'bb', borderRadius: 4 },
                        { label: t('dashboard.budget_llm_avg_output'), data: avgOut, backgroundColor: cv('--success') + 'bb', borderRadius: 4 },
                    ]
                },
                options: {
                    indexAxis: 'y',
                    responsive: true, maintainAspectRatio: false,
                    plugins: { legend: { display: true, position: 'top', labels: { boxWidth: 12, padding: 6, color: cv('--text-primary') } } },
                    scales: {
                        x: { grid: { color: cv('--border-subtle') }, ticks: { color: cv('--text-secondary') }, title: { display: true, text: t('dashboard.budget_llm_avg_xlabel'), color: cv('--text-secondary'), font: { size: 10 } } },
                        y: { grid: { display: false }, ticks: { color: cv('--text-primary'), font: { size: 10 } } },
                    }
                }
            });
        }

        function createPromptSectionDistChart(canvasId, avgSections) {
            const ctx = document.getElementById(canvasId);
            if (!ctx || !avgSections) return null;
            const sectionNameMap = {
                modules:     t('dashboard.prompt_section_modules'),
                memories:    t('dashboard.prompt_section_memories'),
                guides:      t('dashboard.prompt_section_guides'),
                personality: t('dashboard.prompt_section_personality'),
                context:     t('dashboard.prompt_section_context'),
            };
            const order = ['modules', 'memories', 'guides', 'personality', 'context'];
            const labels = order.filter(k => (avgSections[k] || 0) > 0).map(k => sectionNameMap[k] || k);
            const values = order.filter(k => (avgSections[k] || 0) > 0).map(k => avgSections[k]);
            const colors = [cv('--accent'), '#8b5cf6', '#f59e0b', '#ec4899', cv('--text-secondary')];
            if (labels.length === 0) return null;
            return new Chart(ctx, {
                type: 'doughnut',
                data: {
                    labels: labels,
                    datasets: [{
                        data: values,
                        backgroundColor: colors.slice(0, labels.length),
                        borderWidth: 0,
                        cutout: '60%',
                    }]
                },
                options: {
                    responsive: true, maintainAspectRatio: false,
                    plugins: {
                        legend: { display: false },
                        tooltip: {
                            callbacks: {
                                label: (item) => {
                                    const total = values.reduce((a, b) => a + b, 0);
                                    const pct = total > 0 ? ((item.parsed / total) * 100).toFixed(1) : 0;
                                    return ` ${item.label}: ${item.parsed.toLocaleString()} (${pct}%)`;
                                }
                            }
                        }
                    }
                }
            });
        }

        function createRadarChart(canvasId, traits) {
            const ctx = document.getElementById(canvasId);
            if (!ctx) return null;
            const traitOrder = ['curiosity', 'thoroughness', 'creativity', 'empathy', 'confidence', 'affinity', 'loneliness'];
            const traitNameMap = {
                curiosity: t('dashboard.personality_trait_curiosity'),
                thoroughness: t('dashboard.personality_trait_thoroughness'),
                creativity: t('dashboard.personality_trait_creativity'),
                empathy: t('dashboard.personality_trait_empathy'),
                confidence: t('dashboard.personality_trait_confidence'),
                affinity: t('dashboard.personality_trait_affinity'),
                loneliness: t('dashboard.personality_trait_loneliness')
            };
            const labels = traitOrder.map(t2 => traitNameMap[t2] || t2);
            const data = traitOrder.map(t2 => (traits && traits[t2] != null) ? traits[t2] : 0.5);
            return new Chart(ctx, {
                type: 'radar',
                data: {
                    labels: labels,
                    datasets: [{
                        data: data,
                        backgroundColor: cv('--accent') + '33',
                        borderColor: cv('--accent'),
                        borderWidth: 2,
                        pointBackgroundColor: cv('--accent'),
                        pointRadius: 4,
                    }]
                },
                options: {
                    scales: {
                        r: {
                            min: 0, max: 1,
                            ticks: { stepSize: 0.25, display: false },
                            grid: { color: cv('--border-subtle') },
                            angleLines: { color: cv('--border-subtle') },
                            pointLabels: { color: cv('--text-primary'), font: { size: 11 } },
                        }
                    },
                    plugins: {
                        tooltip: {
                            callbacks: { label: ctx => ctx.parsed.r.toFixed(2) }
                        }
                    }
                }
            });
        }

        const MOOD_MAP = {
            excited: 5, curious: 4, creative: 4, playful: 4,
            focused: 3, analytical: 3,
            neutral: 2, cautious: 2,
            bored: 1, frustrated: 0
        };
        const MOOD_LABELS = [
            t('dashboard.personality_mood_frustrated'),
            t('dashboard.personality_mood_bored'),
            t('dashboard.personality_mood_neutral'),
            t('dashboard.personality_mood_focused'),
            t('dashboard.personality_mood_curious'),
            t('dashboard.personality_mood_excited')
        ];

        function moodToNum(mood) {
            const m = (mood || '').toLowerCase();
            return MOOD_MAP[m] != null ? MOOD_MAP[m] : 2;
        }

        function createMoodLineChart(canvasId, entries) {
            const ctx = document.getElementById(canvasId);
            if (!ctx) return null;
            const labels = (entries || []).map(e => {
                const d = new Date(e.timestamp);
                return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
            });
            const data = (entries || []).map(e => moodToNum(e.mood));
            const triggers = (entries || []).map(e => e.trigger || '');
            const moods = (entries || []).map(e => e.mood || 'neutral');
            return new Chart(ctx, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [{
                        data: data,
                        borderColor: cv('--accent'),
                        backgroundColor: cv('--accent') + '22',
                        fill: true, tension: 0.3, pointRadius: 3,
                        pointBackgroundColor: cv('--accent'),
                    }]
                },
                options: {
                    scales: {
                        y: {
                            min: 0, max: 5,
                            ticks: {
                                stepSize: 1,
                                callback: (v) => MOOD_LABELS[v] || '',
                                color: cv('--text-secondary'),
                            },
                            grid: { color: cv('--border-subtle') },
                        },
                        x: {
                            ticks: { color: cv('--text-secondary'), maxTicksLimit: 12, maxRotation: 0 },
                            grid: { color: cv('--border-subtle') },
                        }
                    },
                    plugins: {
                        tooltip: {
                            callbacks: {
                                title: (items) => {
                                    const moodNameMap = {
                                        curious: t('dashboard.personality_mood_curious'),
                                        focused: t('dashboard.personality_mood_focused'),
                                        creative: t('dashboard.personality_mood_creative'),
                                        analytical: t('dashboard.personality_mood_analytical'),
                                        cautious: t('dashboard.personality_mood_cautious'),
                                        playful: t('dashboard.personality_mood_playful'),
                                        excited: t('dashboard.personality_mood_excited'),
                                        neutral: t('dashboard.personality_mood_neutral'),
                                        bored: t('dashboard.personality_mood_bored'),
                                        frustrated: t('dashboard.personality_mood_frustrated')
                                    };
                                    const m = moods[items[0].dataIndex] || '';
                                    return moodNameMap[m] || m;
                                },
                                label: (item) => triggers[item.dataIndex] || t('dashboard.personality_no_trigger'),
                            }
                        }
                    }
                }
            });
        }

        function createMemoryBarChart(canvasId, data) {
            const ctx = document.getElementById(canvasId);
            if (!ctx) return null;
            const embLabel = data.vectordb_disabled ? t('dashboard.memory_embeddings_disabled') : t('dashboard.memory_embeddings');
            const labels = [t('dashboard.memory_chart_core_memory'), t('dashboard.memory_chart_messages'), embLabel, t('dashboard.memory_chart_graph_nodes'), t('dashboard.memory_chart_graph_edges'), t('dashboard.memory_journal'), t('dashboard.memory_notes'), t('dashboard.memory_error_patterns'), t('dashboard.memory_episodic')];
            const values = [
                data.core_memory_facts || 0,
                data.chat_messages || 0,
                data.vectordb_entries || 0,
                (data.knowledge_graph || {}).nodes || 0,
                (data.knowledge_graph || {}).edges || 0,
                data.journal_entries || 0,
                data.notes_count || 0,
                data.error_patterns || 0,
                data.episodic?.total_count || 0,
            ];
            const colors = [cv('--accent'), cv('--success'), '#8b5cf6', '#f59e0b', '#ec4899', '#06b6d4', '#10b981', '#ef4444', '#f97316'];
            return new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [{ data: values, backgroundColor: colors.map(c => c + 'cc'), borderRadius: 6 }]
                },
                options: {
                    indexAxis: 'y',
                    scales: {
                        x: { grid: { color: cv('--border-subtle') }, ticks: { color: cv('--text-secondary') } },
                        y: { grid: { display: false }, ticks: { color: cv('--text-primary'), font: { size: 11 }, autoSkip: false } },
                    }
                }
            });
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // RENDER FUNCTIONS
        // ══════════════════════════════════════════════════════════════════════════════

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

        function renderSystemStats(data) {
            if (!data) return;
            document.getElementById('net-sent').textContent = formatBytes(data.network?.bytes_sent || 0);
            document.getElementById('net-recv').textContent = formatBytes(data.network?.bytes_recv || 0);
            document.getElementById('sse-clients').textContent = data.sse_clients || 0;
            document.getElementById('uptime-val').textContent = formatUptime(data.uptime_seconds || 0);
        }

        function renderBudget(data, credits) {
            if (!data || !data.enabled) {
                dashSetHidden(document.getElementById('budget-content'), true);
                dashSetHidden(document.getElementById('budget-disabled'), false);
                return;
            }
            dashSetHidden(document.getElementById('budget-content'), false);
            dashSetHidden(document.getElementById('budget-disabled'), true);
            document.getElementById('budget-spent').textContent = '$' + (data.spent_usd || 0).toFixed(2);
            document.getElementById('budget-sublabel').textContent = t('dashboard.budget_sublabel', {amount: '$' + (data.daily_limit_usd || 0).toFixed(2)});

            // Status badges
            const badgesEl = document.getElementById('budget-badges');
            let badges = '';
            if (data.is_blocked) badges += '<span class="budget-badge danger">🚫 ' + t('dashboard.budget_blocked') + '</span>';
            else if (data.is_exceeded) badges += '<span class="budget-badge danger">⚠️ ' + t('dashboard.budget_exceeded') + '</span>';
            else if (data.is_warning) badges += '<span class="budget-badge warning">⚡ ' + t('dashboard.budget_warning') + '</span>';
            if (data.enforcement) {
                const enfMap = { warn: t('dashboard.budget_enforcement_warn'), partial: t('dashboard.budget_enforcement_partial'), full: t('dashboard.budget_enforcement_full') };
                badges += `<span class="budget-badge info">🛡️ ${enfMap[data.enforcement] || data.enforcement}</span>`;
            }
            badgesEl.innerHTML = badges;

            // Reset countdown
            const resetEl = document.getElementById('budget-reset');
            if (data.reset_time) {
                const resetDate = new Date(data.reset_time);
                const now = new Date();
                const diffMs = resetDate - now;
                if (diffMs > 0) {
                    const hrs = Math.floor(diffMs / 3600000);
                    const mins = Math.floor((diffMs % 3600000) / 60000);
                    resetEl.textContent = t('dashboard.budget_reset_in', {hours: hrs, minutes: mins});
                } else {
                    resetEl.textContent = '';
                }
            } else {
                resetEl.textContent = '';
            }

            // OpenRouter credits
            const creditsRow = document.getElementById('credits-row');
            if (credits && credits.available && !credits.error) {
                dashSetHidden(creditsRow, false);
                document.getElementById('credits-balance').textContent = '$' + (credits.balance || 0).toFixed(2);
            } else {
                dashSetHidden(creditsRow, true);
            }

            // Per-LLM average token consumption chart
            const llmAvgWrap = document.getElementById('budget-llm-avg-wrap');
            const models = data.models || {};
            const hasCallData = Object.values(models).some(m => (m.calls || 0) > 0);
            if (hasCallData && llmAvgWrap) {
                dashSetHidden(llmAvgWrap, false);
                if (Charts.llmAvg) { Charts.llmAvg.destroy(); Charts.llmAvg = null; }
                Charts.llmAvg = createLLMAvgChart('budget-llm-avg-chart', models);
            } else if (llmAvgWrap) {
                dashSetHidden(llmAvgWrap, true);
            }
        }

        function renderMoodBadge(data) {
            if (!data || !data.enabled) {
                dashSetHidden(document.getElementById('personality-content'), true);
                dashSetHidden(document.getElementById('personality-disabled'), false);
                return;
            }
            const moodNameMap = {
                curious: t('dashboard.personality_mood_curious'),
                focused: t('dashboard.personality_mood_focused'),
                creative: t('dashboard.personality_mood_creative'),
                analytical: t('dashboard.personality_mood_analytical'),
                cautious: t('dashboard.personality_mood_cautious'),
                playful: t('dashboard.personality_mood_playful')
            };
            const badge = document.getElementById('mood-badge');
            const trigger = document.getElementById('mood-trigger');
            const nameLocalized = moodNameMap[data.mood] || data.mood;
            if (badge) badge.textContent = '🎭 ' + nameLocalized;
            if (trigger && data.trigger) trigger.textContent = '"' + data.trigger + '"';

            // Emotion Synthesizer display
            const emotionDisplay = document.getElementById('emotion-display');
            const emotionText = document.getElementById('emotion-text');
            const emotionMeta = document.getElementById('emotion-meta');
            const causePill = document.getElementById('emotion-cause-pill');
            const stylePill = document.getElementById('emotion-style-pill');
            const sourcePill = document.getElementById('emotion-source-pill');
            if (emotionDisplay && emotionText) {
                if (data.current_emotion) {
                    emotionText.textContent = data.current_emotion;
                    dashSetHidden(emotionDisplay, false);
                    const state = data.current_emotion_state || {};
                    const metaParts = [];
                    if (causePill) {
                        causePill.textContent = state.cause ? '↳ ' + state.cause : '';
                        dashSetHidden(causePill, !state.cause);
                    }
                    if (stylePill) {
                        stylePill.textContent = state.recommended_response_style ? '✦ ' + state.recommended_response_style : '';
                        dashSetHidden(stylePill, !state.recommended_response_style);
                    }
                    if (sourcePill) {
                        sourcePill.textContent = state.source ? '⚙ ' + state.source : '';
                        dashSetHidden(sourcePill, !state.source);
                    }
                    if (emotionMeta) {
                        dashSetHidden(emotionMeta, !((state.cause || state.recommended_response_style || state.source)));
                    }
                } else {
                    dashSetHidden(emotionDisplay, true);
                }
            }
        }

        function formatEmotionTriggerSummary(summary) {
            if (!summary || !summary.trigger_counts) return '';
            const entries = Object.entries(summary.trigger_counts)
                .sort((a, b) => b[1] - a[1])
                .slice(0, 2)
                .map(([label, count]) => `${count}× ${label}`);
            return entries.join(' · ');
        }

        function renderEmotionHistory(payload, personality) {
            const summaryEl = document.getElementById('emotion-summary');
            const timelineEl = document.getElementById('emotion-timeline');
            const timelineList = document.getElementById('emotion-timeline-list');
            if (!summaryEl || !timelineEl || !timelineList) return;

            const entries = Array.isArray(payload) ? payload : (payload?.entries || []);
            const summary = Array.isArray(payload) ? null : (payload?.summary || null);

            const currentState = personality?.current_emotion_state || null;
            const summaryCards = [];
            if (currentState && currentState.valence != null) {
                summaryCards.push({ label: t('dashboard.emotion_valence'), value: Number(currentState.valence).toFixed(2) });
            } else if (summary && summary.average_valence != null) {
                summaryCards.push({ label: t('dashboard.emotion_valence'), value: Number(summary.average_valence).toFixed(2) });
            }
            if (currentState && currentState.arousal != null) {
                summaryCards.push({ label: t('dashboard.emotion_arousal'), value: Number(currentState.arousal).toFixed(2) });
            } else if (summary && summary.average_arousal != null) {
                summaryCards.push({ label: t('dashboard.emotion_arousal'), value: Number(summary.average_arousal).toFixed(2) });
            }
            if (summary && summary.latest_cause) {
                summaryCards.push({ label: t('dashboard.emotion_latest_cause'), value: summary.latest_cause });
            }
            const triggerSummary = formatEmotionTriggerSummary(summary);
            if (triggerSummary) {
                summaryCards.push({ label: t('dashboard.emotion_trigger_mix'), value: triggerSummary });
            }

            if (summaryCards.length > 0) {
                summaryEl.innerHTML = summaryCards.map(card =>
                    `<div class="emotion-summary-card"><div class="emotion-summary-label">${card.label}</div><div class="emotion-summary-value">${escapeHtml(card.value || '—')}</div></div>`
                ).join('');
                dashSetHidden(summaryEl, false);
            } else {
                summaryEl.innerHTML = '';
                dashSetHidden(summaryEl, true);
            }

            if (!entries || entries.length === 0) {
                timelineList.innerHTML = `<div class="empty-state">${t('dashboard.emotion_no_history')}</div>`;
                dashSetHidden(timelineEl, false);
                return;
            }

            timelineList.innerHTML = renderCollapsibleList(entries, entry => {
                const ts = entry.timestamp ? new Date(entry.timestamp).toLocaleString([], { month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit' }) : '—';
                const moodLabel = t('dashboard.personality_mood_' + String(entry.primary_mood || 'neutral').toLowerCase());
                const cause = entry.cause || entry.trigger_summary || t('dashboard.personality_no_trigger');
                const desc = entry.description || '';
                return `
                    <div class="emotion-entry">
                        <div class="emotion-entry-top">
                            <div class="emotion-entry-mood">${escapeHtml(moodLabel || entry.primary_mood || '—')}</div>
                            <div class="emotion-entry-time">${escapeHtml(ts)}</div>
                        </div>
                        <div class="emotion-entry-cause">${escapeHtml(cause)}</div>
                        <div class="emotion-entry-desc">${escapeHtml(desc)}</div>
                    </div>
                `;
            }, 6);
            dashSetHidden(timelineEl, false);
        }

        function renderMemoryStats(data) {
            if (!data) return;
            const container = document.getElementById('memory-stats');
            const gn = (data.knowledge_graph || {}).nodes || 0;
            const ge = (data.knowledge_graph || {}).edges || 0;

            let embeddingVal = data.vectordb_entries || 0;
            let embeddingLbl = t('dashboard.memory_embeddings');
            if (data.vectordb_disabled) {
                embeddingLbl = t('dashboard.memory_embeddings_disabled');
            }

            const stats = [
                { val: data.core_memory_facts || 0, lbl: t('dashboard.memory_core_facts'), clickable: true },
                { val: data.chat_messages || 0, lbl: t('dashboard.memory_messages') },
                { val: embeddingVal, lbl: embeddingLbl },
                { val: gn + ' / ' + ge, lbl: t('dashboard.memory_graph_label') },
                { val: data.journal_entries || 0, lbl: t('dashboard.memory_journal') },
                { val: data.notes_count || 0, lbl: t('dashboard.memory_notes') },
                { val: data.error_patterns || 0, lbl: t('dashboard.memory_error_patterns') },
                { val: data.episodic?.total_count || 0, lbl: t('dashboard.memory_episodic') },
            ];
            container.innerHTML = stats.map(s =>
                `<div class="mem-stat${s.clickable ? ' clickable' : ''}"${s.clickable ? ' onclick="openCoreFactsModal()" title="' + t('dashboard.memory_show_core_facts') + '"' : ''}><div class="mem-stat-val">${s.val}</div><div class="mem-stat-lbl">${s.lbl}${s.clickable ? ' 🔍' : ''}</div></div>`
            ).join('');
        }

        function renderMemoryHealth(data) {
            if (!data) return;

            const health = data.memory_health || {};
            const confidence = health.confidence || {};
            const usage = health.usage || {};
            const curator = health.curator || {};
            const strategy = health.strategy || {};
            const episodic = data.episodic || {};
            const pendingActions = Array.isArray(data.pending_actions) ? data.pending_actions : [];
            const conflicts = Array.isArray(data.memory_conflicts) ? data.memory_conflicts : [];

            const summaryEl = document.getElementById('memory-health-summary');
            if (summaryEl) {
                const modeKey = 'dashboard.memory_strategy_mode_' + String(strategy.mode || 'unavailable').toLowerCase();
                const translatedMode = t(modeKey);
                const modeLabel = translatedMode === modeKey ? String(strategy.mode || 'unavailable') : translatedMode;
                const reason = strategy.reason || t('dashboard.memory_strategy_reason_empty');
                const items = [
                    { value: Number(usage.retrieved_events || 0).toLocaleString(), label: t('dashboard.memory_health_retrieved') },
                    { value: Number(usage.predicted_events || 0).toLocaleString(), label: t('dashboard.memory_health_predicted') },
                    { value: Number(usage.distinct_memories || 0).toLocaleString(), label: t('dashboard.memory_health_distinct') },
                    { value: Number(confidence.unverified || 0).toLocaleString(), label: t('dashboard.memory_health_unverified') },
                    { value: Number(curator.stale_candidates || 0).toLocaleString(), label: t('dashboard.memory_health_stale') },
                    { value: Number(episodic.recent_count || 0).toLocaleString(), label: t('dashboard.memory_health_recent_episodes') },
                    { value: Number(pendingActions.length || 0).toLocaleString(), label: t('dashboard.memory_pending_title') },
                    { value: Number(conflicts.length || 0).toLocaleString(), label: t('dashboard.memory_conflicts_title') },
                ];
                summaryEl.innerHTML = `
                    <div class="memory-health-strategy">
                        <div class="memory-health-strategy-head">
                            <span class="memory-health-strategy-label">${esc(t('dashboard.memory_strategy_mode'))}</span>
                            <span class="memory-health-strategy-chip">${esc(modeLabel)}</span>
                        </div>
                        <div class="memory-health-strategy-reason-wrap">
                            <span class="memory-health-strategy-label">${esc(t('dashboard.memory_strategy_reason'))}</span>
                            <span class="memory-health-strategy-reason">${esc(reason)}</span>
                        </div>
                    </div>
                    <div class="memory-health-summary">` + items.map(item => `
                    <div class="memory-health-item">
                        <span class="memory-health-value">${esc(item.value)}</span>
                        <span class="memory-health-label">${esc(item.label)}</span>
                    </div>
                `).join('') + '</div>';
            }

            const curatorEl = document.getElementById('memory-curator-list');
            if (curatorEl) {
                const suggestions = Array.isArray(curator.suggestions) ? curator.suggestions : [];
                const stale = Array.isArray(curator.top_stale) ? curator.top_stale : [];
                const overused = Array.isArray(curator.top_overused) ? curator.top_overused : [];
                const conflictItems = conflicts;
                const facts = [
                    t('dashboard.memory_curator_fact_verification', { count: Number(curator.verification_backlog || 0) }),
                    t('dashboard.memory_curator_fact_low_confidence', { count: Number(curator.low_confidence || 0) }),
                    t('dashboard.memory_curator_fact_contradictions', { count: Number(curator.contradictions || 0) }),
                    t('dashboard.memory_curator_fact_overused', { count: Number(curator.overused_memories || 0) }),
                ];
                curatorEl.innerHTML = '<div class="memory-curator-grid">' +
                    '<div class="memory-curator-section"><div class="memory-curator-list">' + facts.map(item => `<div class="memory-curator-row">${esc(item)}</div>`).join('') + '</div></div>' +
                    '<div class="memory-curator-section"><div class="memory-curator-list">' +
                    (suggestions.length ? renderCollapsibleList(suggestions, item => `<div class="memory-curator-row">${esc(item)}</div>`, 4) : `<div class="empty-state dash-empty-tight">${t('dashboard.memory_curator_empty')}</div>`) +
                    '</div></div>' +
                    '<div class="memory-curator-section"><div class="memory-curator-list">' +
                    (stale.length ? renderCollapsibleList(stale, item => `<div class="memory-curator-row mono">${esc(item)}</div>`, 3) : `<div class="memory-curator-row">${t('dashboard.memory_curator_no_stale')}</div>`) +
                    '</div></div>' +
                    '<div class="memory-curator-section"><div class="memory-curator-list">' +
                    (overused.length ? renderCollapsibleList(overused, item => `<div class="memory-curator-row mono">${esc(item)}</div>`, 3) : `<div class="memory-curator-row">${t('dashboard.memory_curator_no_overused')}</div>`) +
                    '</div></div>' +
                    '<div class="memory-curator-section"><div class="memory-subsection-title">' + esc(t('dashboard.memory_conflicts_title')) + '</div><div class="memory-curator-list">' +
                    (conflictItems.length ? renderCollapsibleList(conflictItems, item => `<div class="memory-curator-row"><span class="memory-conflict-pair mono">${esc(item.left_value || item.doc_id_left || '')} ↔ ${esc(item.right_value || item.doc_id_right || '')}</span><span>${esc(item.reason || '')}</span></div>`, 3) : `<div class="memory-curator-row">${t('dashboard.memory_conflicts_empty')}</div>`) +
                    '</div></div>' +
                '</div>';
            }

            const episodicEl = document.getElementById('memory-episodic-list');
            if (episodicEl) {
                const cards = Array.isArray(episodic.recent_cards) ? episodic.recent_cards : [];
                if (!cards.length && !pendingActions.length) {
                    episodicEl.innerHTML = `<div class="empty-state dash-empty-tight">${t('dashboard.memory_episodic_empty')}</div>`;
                } else {
                    const renderPendingCard = card => `
                        <div class="memory-episodic-item memory-episodic-item-pending">
                            <div class="memory-episodic-head">
                                <span class="memory-episodic-title">${esc(card.title || '')}</span>
                                <span class="memory-pending-chip">${esc(t('dashboard.memory_pending_title'))}</span>
                            </div>
                            <div class="memory-episodic-summary">${esc(card.summary || '')}</div>
                            <div class="memory-episodic-meta"><span>${esc(card.event_date || '')}</span><span>${esc((card.trigger_query || t('dashboard.memory_pending_trigger')))}</span></div>
                        </div>`;
                    const renderRecentCard = card => `
                        <div class="memory-episodic-item">
                            <div class="memory-episodic-head">
                                <span class="memory-episodic-title">${esc(card.title || '')}</span>
                                <span class="memory-episodic-date">${esc(card.event_date || '')}</span>
                            </div>
                            <div class="memory-episodic-summary">${esc(card.summary || '')}</div>
                            <div class="memory-episodic-meta">
                                <span>${esc(card.source || '')}</span>
                                <span>${esc(Array.isArray(card.participants) && card.participants.length ? card.participants.join(', ') : t('dashboard.memory_episodic_agent_user'))}</span>
                            </div>
                        </div>`;
                    const pendingHtml = '<div class="memory-episodic-subsection"><div class="memory-subsection-title">' + esc(t('dashboard.memory_pending_title')) + '</div>' + (pendingActions.length ? renderCollapsibleList(pendingActions, renderPendingCard, 4) : `<div class="empty-state dash-empty-tight">${t('dashboard.memory_pending_empty')}</div>`) + '</div>';
                    const recentHtml = '<div class="memory-episodic-subsection"><div class="memory-subsection-title">' + esc(t('dashboard.memory_episodic_title')) + '</div>' + (cards.length ? renderCollapsibleList(cards, renderRecentCard, 4) : `<div class="empty-state dash-empty-tight">${t('dashboard.memory_episodic_empty')}</div>`) + '</div>';
                    episodicEl.innerHTML = '<div class="memory-episodic-list">' + pendingHtml + recentHtml + '</div>';
                }
            }
        }

        function renderMilestones(milestones) {
            const container = document.getElementById('milestone-list');
            if (!milestones || milestones.length === 0) {
                container.innerHTML = '<div class="empty-state">' + t('dashboard.memory_no_milestones') + '</div>';
                return;
            }
            container.innerHTML = renderCollapsibleList(milestones, m => {
                const d = new Date(m.timestamp);
                const dateStr = d.toLocaleDateString([], { month: 'short', day: 'numeric' });
                return `<div class="milestone-item">
            <span class="milestone-icon">🏆</span>
            <span class="milestone-text">${esc(m.label)}${m.details ? ': ' + esc(m.details) : ''}</span>
            <span class="milestone-date">${dateStr}</span>
        </div>`;
            }, 5);
        }

        function renderProfile(data) {
            const container = document.getElementById('profile-content');
            if (!data || !data.categories || Object.keys(data.categories).length === 0) {
                container.innerHTML = '<div class="empty-state">' + t('dashboard.profile_empty') + '</div>';
                return;
            }
            const catIcons = { tech: '💻', prefs: '⭐', preferences: '⭐', interests: '🎯', context: '📋', comm: '💬', communication: '💬' };
            const catNameMap = {
                tech: t('dashboard.profile_cat_tech'),
                prefs: t('dashboard.profile_cat_preferences'),
                preferences: t('dashboard.profile_cat_preferences'),
                interests: t('dashboard.profile_cat_interests'),
                context: t('dashboard.profile_cat_context'),
                comm: t('dashboard.profile_cat_communication'),
                communication: t('dashboard.profile_cat_communication')
            };
            let html = '';
            for (const [cat, entries] of Object.entries(data.categories)) {
                html += `<div class="profile-category" data-cat="${esc(cat)}">
            <div class="profile-cat-header" onclick="toggleCategory(this)">
                <span class="profile-cat-toggle">▶</span>
                ${catIcons[cat] || '📦'} ${catNameMap[cat] || esc(cat)} (${entries.length})
            </div>
            <div class="profile-entries is-hidden">`;
                for (const e of entries) {
                    const confClass = 'conf-' + Math.min(3, Math.max(1, e.confidence || 1));
                    const firstSeen = e.first_seen ? e.first_seen.replace('T', ' ').slice(0, 16) : '';
                    const updatedAt = e.updated_at ? e.updated_at.replace('T', ' ').slice(0, 16) : '';
                    const tip = (firstSeen ? t('dashboard.profile_tooltip_created') + ' ' + firstSeen + '\n' : '') + (updatedAt ? t('dashboard.profile_tooltip_updated') + ' ' + updatedAt : '');
                    html += `<div class="profile-entry" data-search="${esc(e.key + ' ' + e.value)}" title="${esc(tip)}">
                <span class="profile-key">${esc(e.key)}</span>
                <span class="profile-val">${esc(e.value)}</span>
                <span class="confidence-badge ${confClass}" title="${t('dashboard.profile_confidence')} ${e.confidence}">${e.confidence}</span>
                <span class="profile-actions">
                    <button class="profile-btn-edit" data-cat="${esc(cat)}" data-key="${esc(e.key)}" onclick="editProfileEntry(this)" title="${t('dashboard.profile_edit_save')}">✏️</button>
                    <button class="profile-btn-delete" data-cat="${esc(cat)}" data-key="${esc(e.key)}" onclick="deleteProfileEntry(this)" title="${t('dashboard.profile_delete_confirm')}">🗑️</button>
                </span>
                <span class="profile-edit-form is-hidden">
                    <input type="text" class="profile-edit-input" value="${esc(e.value)}">
                    <button class="profile-btn-save" onclick="saveProfileEntry(this)" title="${t('dashboard.profile_edit_save')}">✓</button>
                    <button class="profile-btn-cancel" onclick="cancelProfileEdit(this)" title="${t('dashboard.profile_edit_cancel')}">✗</button>
                </span>
            </div>`;
                }
                html += '</div></div>';
            }
            container.innerHTML = html;
            // Auto-open first category
            const first = container.querySelector('.profile-cat-header');
            if (first) toggleCategory(first);
        }

        function toggleCategory(header) {
            const entries = header.nextElementSibling;
            const toggle = header.querySelector('.profile-cat-toggle');
            if (entries.classList.contains('is-hidden')) {
                entries.classList.remove('is-hidden');
                toggle.classList.add('open');
            } else {
                entries.classList.add('is-hidden');
                toggle.classList.remove('open');
            }
        }

        async function deleteProfileEntry(btn) {
            const cat = btn.dataset.cat;
            const key = btn.dataset.key;
            if (!await showConfirm(t('dashboard.profile_delete_confirm') + ' "' + key + '"?')) return;
            fetch('/api/dashboard/profile/entry?' + new URLSearchParams({ category: cat, key: key }), {
                method: 'DELETE', credentials: 'same-origin'
            }).then(r => { if (r.ok) loadTabUser(); }).catch(() => {});
        }

        function editProfileEntry(btn) {
            const entry = btn.closest('.profile-entry');
            dashSetHidden(entry.querySelector('.profile-val'), true);
            dashSetHidden(entry.querySelector('.profile-actions'), true);
            const editForm = entry.querySelector('.profile-edit-form');
            editForm.classList.remove('is-hidden');
            editForm.classList.add('profile-edit-form-open');
            editForm.querySelector('.profile-edit-input').focus();
        }

        function cancelProfileEdit(btn) {
            const entry = btn.closest('.profile-entry');
            dashSetHidden(entry.querySelector('.profile-val'), false);
            dashSetHidden(entry.querySelector('.profile-actions'), false);
            const editForm = entry.querySelector('.profile-edit-form');
            editForm.classList.remove('profile-edit-form-open');
            editForm.classList.add('is-hidden');
        }

        function saveProfileEntry(btn) {
            const entry = btn.closest('.profile-entry');
            const cat = entry.closest('.profile-category').dataset.cat;
            const key = entry.querySelector('.profile-key').textContent;
            const newVal = entry.querySelector('.profile-edit-input').value.trim();
            if (!newVal) return;
            fetch('/api/dashboard/profile/entry', {
                method: 'PUT', credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ category: cat, key: key, value: newVal })
            }).then(r => { if (r.ok) loadTabUser(); }).catch(() => {});
        }

        function renderActivity(data) {
            if (!data) return;
            const statsEl = document.getElementById('activity-stats');
            const detailsEl = document.getElementById('activity-details');

            const cronCount = Array.isArray(data.cron_jobs) ? data.cron_jobs.length : 0;
            const procCount = Array.isArray(data.processes) ? data.processes.length : 0;
            const whCount = (data.webhooks || {}).count || 0;
            const coCount = Array.isArray(data.coagents) ? data.coagents.length : 0;
            const bgSummary = data.background_task_summary || {};
            const bgTasks = Array.isArray(data.background_tasks) ? data.background_tasks : [];
            const bgActive = (bgSummary.queued || 0) + (bgSummary.waiting || 0) + (bgSummary.running || 0);

            statsEl.innerHTML = [
                { icon: '⏰', val: cronCount, lbl: t('dashboard.activity_scheduled') },
                { icon: '🔄', val: procCount, lbl: t('dashboard.activity_processes') },
                { icon: '🔗', val: whCount, lbl: t('dashboard.activity_webhooks') },
                { icon: '🤖', val: coCount, lbl: t('dashboard.activity_coagents') },
                { icon: '🗂️', val: bgActive, lbl: t('dashboard.activity_background') },
            ].map(s =>
                `<div class="activity-stat">
            <div class="activity-stat-icon">${s.icon}</div>
            <div class="activity-stat-val">${s.val}</div>
            <div class="activity-stat-lbl">${s.lbl}</div>
        </div>`
            ).join('');

            let details = '';

            // Cron Jobs
            if (cronCount > 0) {
                details += '<div class="activity-section"><div class="activity-section-title">⏰ ' + t('dashboard.activity_scheduled_tasks') + '</div>';
                for (const job of data.cron_jobs) {
                    const safeId     = esc(job.id || 'unknown');
                    const safeExpr   = esc(job.cron_expr || '');
                    const safePrompt = esc(job.task_prompt || '');
                    details += `<div class="activity-item">
                <span class="activity-item-name">${safeId}</span>
                <div class="activity-item-row">
                    <span class="activity-item-detail">${safeExpr} — ${esc(truncate(job.task_prompt || '', 60))}</span>
                    <span class="activity-item-actions-inline">
                        <button class="cf-fact-btn"
                            data-cron-id="${safeId}"
                            data-cron-expr="${safeExpr}"
                            data-cron-prompt="${safePrompt}"
                            onclick="openCronEditModal(this)"
                            title="${t('dashboard.cron_edit_title')}">✏️</button>
                        <button class="cf-fact-btn danger"
                            data-cron-id="${safeId}"
                            onclick="deleteCronJob(this.dataset.cronId)"
                            title="${t('dashboard.cron_btn_delete')}">🗑️</button>
                    </span>
                </div>
            </div>`;
                }
                details += '</div>';
            }

            // Processes
            if (procCount > 0) {
                details += '<div class="activity-section"><div class="activity-section-title">🔄 ' + t('dashboard.activity_running_processes') + '</div>';
                for (const p of data.processes) {
                    const alive = p.alive ? 'pill-running' : 'pill-idle';
                    details += `<div class="activity-item">
                <span class="activity-item-name">PID ${p.pid}</span>
                <span><span class="pill-status ${alive}">${p.alive ? t('dashboard.activity_process_active') : t('dashboard.activity_process_stopped')}</span>
                <span class="activity-item-detail activity-item-detail-spaced">${esc(p.uptime || '')}</span></span>
            </div>`;
                }
                details += '</div>';
            }

            // Co-Agents
            if (coCount > 0) {
                details += '<div class="activity-section"><div class="activity-section-title">🤖 ' + t('dashboard.activity_coagents') + '</div>';
                for (const ca of data.coagents) {
                    const stateMap = {
                        queued: t('dashboard.activity_coagent_queued'),
                        running: t('dashboard.activity_coagent_running'),
                        completed: t('dashboard.activity_coagent_completed'),
                        failed: t('dashboard.activity_coagent_failed'),
                        cancelled: t('dashboard.activity_coagent_cancelled')
                    };
                    const stateClass = ca.state === 'queued' ? 'pill-idle' :
                        ca.state === 'running' ? 'pill-running' :
                        ca.state === 'completed' ? 'pill-completed' :
                            ca.state === 'failed' ? 'pill-failed' : 'pill-idle';
                    const specIcons = { researcher: '\uD83D\uDD0D', coder: '\uD83D\uDCBB', designer: '\uD83C\uDFA8', security: '\uD83D\uDEE1\uFE0F', writer: '\u270D\uFE0F' };
                    const specBadge = ca.specialist && specIcons[ca.specialist] ? '<span title="' + esc(ca.specialist) + '" style="margin-right:0.3rem;">' + specIcons[ca.specialist] + '</span>' : '';
                    const extra = [];
                    if (ca.queue_position) extra.push('Q' + esc(String(ca.queue_position)));
                    if (ca.retry_count) extra.push('R' + esc(String(ca.retry_count)));
                    if (ca.last_event) extra.push(esc(String(ca.last_event)));
                    details += `<div class="activity-item">
                <span class="activity-item-name">${specBadge}${esc(truncate(ca.task || ca.id, 50))}</span>
                <span><span class="pill-status ${stateClass}">${esc(stateMap[ca.state] || ca.state)}</span>
                <span class="activity-item-detail activity-item-detail-spaced">${esc(ca.runtime || '')}${extra.length ? ' · ' + extra.join(' · ') : ''}</span></span>
            </div>`;
                }
                details += '</div>';
            }

            // Background Tasks
            if (bgTasks.length > 0) {
                details += '<div class="activity-section"><div class="activity-section-title">🗂️ ' + t('dashboard.activity_background_tasks') + '</div>';
                for (const task of bgTasks) {
                    const status = String(task.status || 'queued');
                    const statusClass =
                        status === 'running' ? 'pill-running' :
                        status === 'completed' ? 'pill-completed' :
                        status === 'failed' ? 'pill-failed' :
                        status === 'waiting' ? 'pill-warning' :
                        status === 'canceled' ? 'pill-idle' : 'pill-idle';
                    const detailParts = [
                        task.type ? esc(task.type) : '',
                        task.last_error ? esc(truncate(task.last_error, 60)) : '',
                        task.result && !task.last_error ? esc(truncate(task.result, 60)) : ''
                    ].filter(Boolean);
                    details += `<div class="activity-item">
                <span class="activity-item-name">${esc(truncate(task.description || task.id || 'background-task', 56))}</span>
                <span><span class="pill-status ${statusClass}">${esc(status)}</span>
                <span class="activity-item-detail activity-item-detail-spaced">${detailParts.join(' · ')}</span></span>
            </div>`;
                }
                details += '</div>';
            }

            if (!details) {
                details = '<div class="empty-state">' + t('dashboard.activity_no_automations') + '</div>';
            }
            detailsEl.innerHTML = details;
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // PROMPT BUILDER ANALYTICS
        // ══════════════════════════════════════════════════════════════════════════════

        function renderPromptStats(data) {
            if (!data || data.total_builds === 0) {
                dashSetHidden(document.getElementById('prompt-no-data'), false);
                dashSetHidden(document.getElementById('prompt-content'), true);
                return;
            }
            dashSetHidden(document.getElementById('prompt-no-data'), true);
            dashSetHidden(document.getElementById('prompt-content'), false);

            // Main KPI grid
            const kpis = document.getElementById('prompt-kpis');
            const optPct = data.avg_optimization_pct ? data.avg_optimization_pct.toFixed(1) : '0';
            const kpiItems = [
                { val: data.total_builds, lbl: t('dashboard.prompt_kpi_total_builds') },
                { val: data.avg_tokens.toLocaleString(), lbl: t('dashboard.prompt_kpi_avg_tokens') },
                { val: data.avg_raw_len.toLocaleString(), lbl: t('dashboard.prompt_kpi_avg_raw_chars') },
                { val: data.avg_optimized_len.toLocaleString(), lbl: t('dashboard.prompt_kpi_avg_opt_chars') },
                { val: optPct + '%', lbl: t('dashboard.prompt_kpi_avg_saving_pct'), highlight: parseFloat(optPct) >= 20 },
                { val: data.total_saved_chars.toLocaleString(), lbl: t('dashboard.prompt_kpi_total_savings') },
                { val: data.budget_shed_count, lbl: t('dashboard.prompt_kpi_budget_sheds') },
                { val: (data.shed_rate_pct || 0).toFixed(1) + '%', lbl: t('dashboard.prompt_kpi_shed_rate') },
                { val: data.avg_modules_loaded, lbl: t('dashboard.prompt_kpi_avg_modules_loaded') },
                { val: data.avg_modules_used, lbl: t('dashboard.prompt_kpi_avg_modules_used') },
                { val: (data.avg_module_filter_rate_pct || 0).toFixed(1) + '%', lbl: t('dashboard.prompt_kpi_filter_rate') },
                { val: data.avg_guides_count, lbl: t('dashboard.prompt_kpi_avg_guides') },
            ];
            kpis.innerHTML = kpiItems.map(k =>
                `<div class="prompt-kpi${k.highlight ? ' prompt-kpi-highlight' : ''}"><div class="prompt-kpi-val">${k.val}</div><div class="prompt-kpi-lbl">${k.lbl}</div></div>`
            ).join('');

            // Savings breakdown KPI grid
            const savingsKpis = document.getElementById('prompt-savings-kpis');
            if (savingsKpis) {
                const avgFormat  = (data.avg_format_savings  || 0).toLocaleString();
                const avgShed    = (data.avg_shed_savings    || 0).toLocaleString();
                const avgFilter  = (data.avg_filter_savings  || 0).toLocaleString();
                const totFormat  = (data.total_format_savings  || 0).toLocaleString();
                const totShed    = (data.total_shed_savings    || 0).toLocaleString();
                const totFilter  = (data.total_filter_savings  || 0).toLocaleString();
                const totalSaved = (data.total_saved_chars     || 0).toLocaleString();
                const rawAvg     = data.avg_raw_len || 1;
                const fmtPct    = data.avg_raw_len > 0 ? ((data.avg_format_savings  || 0) / rawAvg * 100).toFixed(1) : '0';
                const shedPct   = data.avg_raw_len > 0 ? ((data.avg_shed_savings    || 0) / rawAvg * 100).toFixed(1) : '0';
                const filterPct = data.avg_raw_len > 0 ? ((data.avg_filter_savings  || 0) / rawAvg * 100).toFixed(1) : '0';
                const breakdownItems = [
                    { val: avgFormat,  sub: fmtPct + '%',    lbl: t('dashboard.prompt_kpi_format_savings'),  colorClass: 'prompt-kpi-dot-success' },
                    { val: avgShed,    sub: shedPct + '%',   lbl: t('dashboard.prompt_kpi_shed_savings'),    colorClass: 'prompt-kpi-dot-warn' },
                    { val: avgFilter,  sub: filterPct + '%', lbl: t('dashboard.prompt_kpi_filter_savings'),  colorClass: 'prompt-kpi-dot-violet' },
                    { val: totFormat,  sub: null,            lbl: t('dashboard.prompt_kpi_format_savings') + ' total', colorClass: null },
                    { val: totShed,    sub: null,            lbl: t('dashboard.prompt_kpi_shed_savings')   + ' total', colorClass: null },
                    { val: totFilter,  sub: null,            lbl: t('dashboard.prompt_kpi_filter_savings') + ' total', colorClass: null },
                ];
                savingsKpis.innerHTML = breakdownItems.map(k =>
                    `<div class="prompt-kpi">${k.colorClass ? `<div class="prompt-kpi-dot ${k.colorClass}"></div>` : ''}<div class="prompt-kpi-val">${k.val}${k.sub ? `<span class="prompt-kpi-sub"> (${k.sub})</span>` : ''}</div><div class="prompt-kpi-lbl">${k.lbl}</div></div>`
                ).join('');
            }

            // Shed section list
            const shedEl = document.getElementById('shed-list');
            const shedCounts = data.shed_section_counts || {};
            const shedKeys = Object.keys(shedCounts).sort((a, b) => shedCounts[b] - shedCounts[a]);
            if (shedKeys.length === 0) {
                shedEl.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.prompt_no_sections_shed') + '</div>';
            } else {
                shedEl.innerHTML = shedKeys.map(k =>
                    `<div class="shed-item"><span>${esc(k)}</span><span class="shed-count">${shedCounts[k]}×</span></div>`
                ).join('');
            }

            // Prompt section distribution chart + legend
            const avgSections = data.avg_section_sizes || {};
            const sectionOrder = ['modules', 'memories', 'guides', 'personality', 'context'];
            const sectionNameMap = {
                modules:     t('dashboard.prompt_section_modules'),
                memories:    t('dashboard.prompt_section_memories'),
                guides:      t('dashboard.prompt_section_guides'),
                personality: t('dashboard.prompt_section_personality'),
                context:     t('dashboard.prompt_section_context'),
            };
            const legendEl = document.getElementById('prompt-section-legend');
            if (legendEl && Object.keys(avgSections).length > 0) {
                const total = sectionOrder.reduce((s, k) => s + (avgSections[k] || 0), 0);
                legendEl.innerHTML = sectionOrder
                    .filter(k => (avgSections[k] || 0) > 0)
                    .map((k, i) => {
                        const pct = total > 0 ? ((avgSections[k] / total) * 100).toFixed(1) : 0;
                        const colorIdx = sectionOrder.indexOf(k);
                        return `<div class="prompt-section-legend-item">
                            <span class="prompt-section-legend-dot prompt-section-legend-dot-${colorIdx}"></span>
                            <span class="prompt-section-legend-label">${esc(sectionNameMap[k] || k)}</span>
                            <span class="prompt-section-legend-val">${(avgSections[k] || 0).toLocaleString()} <span class="prompt-section-legend-pct">(${pct}%)</span></span>
                        </div>`;
                    }).join('');
            }
        }

        function createPromptSizeChart(canvasId, recent) {
            const ctx = document.getElementById(canvasId);
            if (!ctx || !recent || recent.length === 0) return null;
            const labels = recent.map((_, i) => '#' + (i + 1));
            return new Chart(ctx, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [
                        {
                            label: t('dashboard.prompt_chart_raw'),
                            data: recent.map(r => r.raw_len),
                            borderColor: cv('--text-secondary'),
                            backgroundColor: 'transparent',
                            borderWidth: 1.5,
                            pointRadius: 2,
                            tension: 0.3,
                        },
                        {
                            label: t('dashboard.prompt_chart_optimized'),
                            data: recent.map(r => r.optimized_len),
                            borderColor: cv('--accent'),
                            backgroundColor: cv('--accent') + '22',
                            fill: true,
                            borderWidth: 2,
                            pointRadius: 2,
                            tension: 0.3,
                        },
                    ]
                },
                options: {
                    plugins: { legend: { display: true, position: 'top', labels: { boxWidth: 12, padding: 6 } } },
                    scales: {
                        x: { display: false },
                        y: { grid: { color: cv('--border-subtle') }, ticks: { color: cv('--text-secondary') } },
                    }
                }
            });
        }

        function createPromptTierChart(canvasId, tiers) {
            const ctx = document.getElementById(canvasId);
            if (!ctx || !tiers) return null;
            const labels = Object.keys(tiers);
            const values = Object.values(tiers);
            if (labels.length === 0) return null;
            const colors = { full: cv('--accent'), compact: '#f59e0b', minimal: '#ec4899' };
            return new Chart(ctx, {
                type: 'doughnut',
                data: {
                    labels: labels.map(l => l.charAt(0).toUpperCase() + l.slice(1)),
                    datasets: [{
                        data: values,
                        backgroundColor: labels.map(l => colors[l] || '#8b5cf6'),
                        borderWidth: 0,
                    }]
                },
                options: {
                    plugins: {
                        legend: { display: true, position: 'bottom', labels: { boxWidth: 12, padding: 8, color: cv('--text-primary') } },
                        tooltip: { enabled: true },
                    }
                }
            });
        }

        function createPromptSavingsChart(canvasId, recent) {
            const ctx = document.getElementById(canvasId);
            if (!ctx || !recent || recent.length === 0) return null;
            const labels = recent.map((_, i) => '#' + (i + 1));
            // Stacked: show all three savings components per build
            // Falls back gracefully for older records where breakdown fields are 0
            return new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [
                        {
                            label: t('dashboard.prompt_chart_format_savings'),
                            data: recent.map(r => r.format_savings || r.saved_chars || 0),
                            backgroundColor: cv('--success') + 'cc',
                            borderRadius: 3,
                            stack: 'savings',
                        },
                        {
                            label: t('dashboard.prompt_chart_shed_savings'),
                            data: recent.map(r => r.shed_savings || 0),
                            backgroundColor: '#f59e0b' + 'cc',
                            borderRadius: 3,
                            stack: 'savings',
                        },
                        {
                            label: t('dashboard.prompt_chart_filter_savings'),
                            data: recent.map(r => r.filter_savings || 0),
                            backgroundColor: '#8b5cf6' + 'cc',
                            borderRadius: 3,
                            stack: 'savings',
                        },
                    ]
                },
                options: {
                    plugins: {
                        legend: { display: true, position: 'top', labels: { boxWidth: 10, padding: 6, color: cv('--text-primary'), font: { size: 10 } } },
                        tooltip: {
                            callbacks: {
                                footer: (items) => {
                                    const total = items.reduce((s, i) => s + i.parsed.y, 0);
                                    return 'Total: ' + total.toLocaleString();
                                }
                            }
                        }
                    },
                    scales: {
                        x: { display: false, stacked: true },
                        y: { stacked: true, grid: { color: cv('--border-subtle') }, ticks: { color: cv('--text-secondary') } },
                    }
                }
            });
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // ADAPTIVE TOOL STATS
        // ══════════════════════════════════════════════════════════════════════════════

        function renderAdaptiveToolStats(data) {
            const card = document.getElementById('card-adaptive-tools');
            if (!data || !data.adaptive_enabled) {
                dashSetHidden(card, true);
                return;
            }

            const scores = data.adaptive_scores || [];
            const totalTracked = scores.length;
            const maxTools = data.max_tools || 0;
            const activeCount = maxTools > 0 ? Math.min(totalTracked, maxTools) : totalTracked;
            const totalCalls = data.total_calls || 0;

            dashSetHidden(card, false);

            const kpis = document.getElementById('adaptive-tools-kpis');
            if (kpis) {
                const kpiItems = [
                    { val: `${activeCount}/${totalTracked}`, lbl: t('dashboard.adaptive_tools_active') },
                    { val: maxTools || '∞', lbl: t('dashboard.adaptive_tools_max') },
                    { val: totalCalls.toLocaleString(), lbl: t('dashboard.adaptive_tools_total_calls') },
                ];
                kpis.innerHTML = kpiItems.map(k =>
                    `<div class="prompt-kpi"><div class="prompt-kpi-val">${k.val}</div><div class="prompt-kpi-lbl">${k.lbl}</div></div>`
                ).join('');
            }

            const list = document.getElementById('adaptive-tools-list');
            if (list && scores.length > 0) {
                const maxScore = scores[0]?.score || 1;
                list.innerHTML = `<div class="adaptive-tools-grid">` +
                    scores.slice(0, 30).map((s, i) => {
                        const pct = maxScore > 0 ? Math.round((s.score / maxScore) * 100) : 0;
                        const isActive = maxTools <= 0 || i < maxTools;
                        return `<div class="adaptive-tool-row ${isActive ? '' : 'adaptive-tool-row-inactive'}">
                            <span class="adaptive-tool-name" title="${esc(s.tool)}">${esc(s.tool)}</span>
                            <div class="adaptive-tool-bar-bg">
                                <div class="adaptive-tool-bar-fill w-pct-${pct}"></div>
                            </div>
                            <span class="adaptive-tool-count">${s.count}×</span>
                        </div>`;
                    }).join('') + `</div>`;
            }
        }

        function toolingTelemetryParseLabel(key) {
            const labels = {
                native: t('dashboard.tooling_telemetry_parse_native'),
                reasoning_clean_json: t('dashboard.tooling_telemetry_parse_reasoning_clean_json'),
                content_json: t('dashboard.tooling_telemetry_parse_content_json'),
            };
            return labels[key] || key;
        }

        function toolingTelemetryRecoveryLabel(key) {
            const labels = {
                provider_422_recovered: t('dashboard.tooling_telemetry_recovery_provider_422_recovered'),
                provider_422_aborted: t('dashboard.tooling_telemetry_recovery_provider_422_aborted'),
                empty_response_recovered: t('dashboard.tooling_telemetry_recovery_empty_response_recovered'),
                duplicate_tool_call_blocked: t('dashboard.tooling_telemetry_recovery_duplicate_tool_call_blocked'),
                identical_tool_error_blocked: t('dashboard.tooling_telemetry_recovery_identical_tool_error_blocked'),
                tool_output_truncated: t('dashboard.tooling_telemetry_recovery_tool_output_truncated'),
                error_output_truncated_preserved: t('dashboard.tooling_telemetry_recovery_error_output_truncated_preserved'),
            };
            return labels[key] || key;
        }

        function toolingTelemetryPolicyLabel(key) {
            const labels = {
                conservative_profile_applied: t('dashboard.tooling_telemetry_policy_conservative_profile_applied'),
                prompt_tier_compact: t('dashboard.tooling_telemetry_policy_prompt_tier_compact'),
            };
            if (labels[key]) return labels[key];
            if (String(key || '').startsWith('family_guarded_')) {
                const family = String(key).replace('family_guarded_', '');
                return t('dashboard.tooling_telemetry_policy_family_guarded', { family });
            }
            return labels[key] || key;
        }

        function toolingTelemetryRetrievalLabel(key) {
            const rawKey = String(key || '');
            if (!rawKey) return rawKey;

            const replacements = {
                rag_auto_attempt: 'Auto-RAG searches',
                rag_auto_hit: 'Auto-RAG hits',
                rag_auto_miss: 'Auto-RAG misses',
                rag_auto_filtered_out: 'Auto-RAG filtered after ranking',
                rag_auto_error: 'Auto-RAG errors',
                rag_predictive_attempt: 'Predictive prefetch searches',
                rag_predictive_hit: 'Predictive prefetch hits',
                rag_predictive_miss: 'Predictive prefetch misses',
                rag_predictive_error: 'Predictive prefetch errors',
            };
            if (replacements[rawKey]) return replacements[rawKey];
            if (rawKey.startsWith('rag_auto_source:')) {
                return 'Auto-RAG source: ' + rawKey.split(':')[1].replaceAll('_', ' ');
            }
            if (rawKey.startsWith('rag_predictive_source:')) {
                return 'Predictive source: ' + rawKey.split(':')[1].replaceAll('_', ' ');
            }
            if (rawKey.startsWith('rag_auto_latency:')) {
                return 'Auto-RAG latency ' + rawKey.split(':')[1].replaceAll('_', '-');
            }
            if (rawKey.startsWith('rag_predictive_latency:')) {
                return 'Predictive latency ' + rawKey.split(':')[1].replaceAll('_', '-');
            }
            if (rawKey.startsWith('memory_prompt_tokens:')) {
                return 'Memory prompt tokens ' + rawKey.split(':')[1].replaceAll('_', '-');
            }
            if (rawKey.startsWith('memory_prompt_share:')) {
                return 'Memory prompt share ' + rawKey.split(':')[1].replaceAll('_', '-');
            }
            if (rawKey.startsWith('memory_prompt_share_value:')) {
                return 'Memory prompt share ' + rawKey.split(':')[1] + '%';
            }
            return rawKey;
        }

        function toolingTelemetrySummarizeRetrieval(eventMap) {
            const entries = Object.entries(eventMap || {}).filter(([, count]) => Number(count || 0) > 0);
            let total = 0;
            let weightedShare = 0;
            let weightedShareCount = 0;
            const visibleEntries = [];

            entries.forEach(([key, count]) => {
                const numericCount = Number(count || 0);
                total += numericCount;
                if (String(key).startsWith('memory_prompt_share_value:')) {
                    const share = Number(String(key).split(':')[1] || 0);
                    if (Number.isFinite(share)) {
                        weightedShare += share * numericCount;
                        weightedShareCount += numericCount;
                    }
                    return;
                }
                visibleEntries.push([key, numericCount]);
            });

            visibleEntries.sort((a, b) => Number(b[1]) - Number(a[1]) || String(a[0]).localeCompare(String(b[0])));
            return {
                total,
                visibleEntries,
                avgShare: weightedShareCount > 0 ? (weightedShare / weightedShareCount) : 0,
                primary: visibleEntries[0] || null,
            };
        }

        function renderTelemetryGroup(container, items, labelFn) {
            if (!container) return;
            if (!items.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_empty') + '</div>';
                return;
            }

            container.innerHTML = '<div class="tooling-telemetry-list">' + items.map(([key, count]) => `
                <div class="tooling-telemetry-chip">
                    <span class="tooling-telemetry-chip-value">${Number(count || 0).toLocaleString()}</span>
                    <span class="tooling-telemetry-chip-label">${esc(labelFn(key))}</span>
                </div>
            `).join('') + '</div>';
        }

        function renderToolingSummary(summaryEl, parseSources, recoveryEvents, policyEvents, retrievalSummary, failingTools) {
            if (!summaryEl) return;
            const totalRecoveries = recoveryEvents.reduce((sum, [, count]) => sum + Number(count || 0), 0);
            const totalPolicyEvents = policyEvents.reduce((sum, [, count]) => sum + Number(count || 0), 0);
            const primaryRecovery = recoveryEvents[0] ? toolingTelemetryRecoveryLabel(recoveryEvents[0][0]) : t('dashboard.tooling_telemetry_none');
            const primaryPolicy = policyEvents[0] ? toolingTelemetryPolicyLabel(policyEvents[0][0]) : t('dashboard.tooling_telemetry_none');
            const primaryRetrieval = retrievalSummary.primary ? toolingTelemetryRetrievalLabel(retrievalSummary.primary[0]) : t('dashboard.tooling_telemetry_none');
            const primaryFailureTool = failingTools[0]?.tool || t('dashboard.tooling_telemetry_none');

            const items = [
                { value: totalRecoveries.toLocaleString(), label: t('dashboard.tooling_telemetry_recoveries_total') },
                { value: totalPolicyEvents.toLocaleString(), label: t('dashboard.tooling_telemetry_policy_adjustments_total') },
                { value: retrievalSummary.total.toLocaleString(), label: t('dashboard.tooling_telemetry_retrieval_total') },
                { value: t('dashboard.tooling_telemetry_retrieval_share_short', { pct: retrievalSummary.avgShare.toFixed(0) }), label: t('dashboard.tooling_telemetry_retrieval_avg_share') },
                { value: failingTools.length.toLocaleString(), label: t('dashboard.tooling_telemetry_tools_with_failures') },
                { value: primaryRecovery, label: t('dashboard.tooling_telemetry_primary_issue') },
                { value: primaryPolicy, label: t('dashboard.tooling_telemetry_active_policy_signal') },
                { value: primaryRetrieval, label: t('dashboard.tooling_telemetry_retrieval_primary') },
                { value: primaryFailureTool, label: t('dashboard.tooling_telemetry_primary_failure_tool') },
            ];

            summaryEl.innerHTML = '<div class="tooling-telemetry-summary">' + items.map(item => `
                <div class="tooling-telemetry-summary-item">
                    <span class="tooling-telemetry-summary-value">${esc(item.value)}</span>
                    <span class="tooling-telemetry-summary-label">${esc(item.label)}</span>
                </div>
            `).join('') + '</div>';
        }

        function renderFailureTools(container, failingTools) {
            if (!container) return;
            if (!failingTools.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_no_failures') + '</div>';
                return;
            }

            container.innerHTML = '<div class="tooling-telemetry-failure-list">' + renderCollapsibleList(failingTools, item => `
                <div class="tooling-telemetry-failure-item">
                    <div>
                        <span class="tooling-telemetry-failure-tool">${esc(item.tool)}</span>
                        <span class="tooling-telemetry-failure-meta">${esc(t('dashboard.tooling_telemetry_failure_count', { fails: item.failures, calls: item.total }))}</span>
                    </div>
                    <span class="tooling-telemetry-failure-badge">${Number(item.failures).toLocaleString()}</span>
                </div>
            `, 6) + '</div>';
        }

        function toolingTelemetryFamilyLabel(key) {
            const labels = {
                files: t('dashboard.tooling_telemetry_family_files'),
                shell: t('dashboard.tooling_telemetry_family_shell'),
                coding: t('dashboard.tooling_telemetry_family_coding'),
                memory: t('dashboard.tooling_telemetry_family_memory'),
                web: t('dashboard.tooling_telemetry_family_web'),
                deployment: t('dashboard.tooling_telemetry_family_deployment'),
                network: t('dashboard.tooling_telemetry_family_network'),
                infra: t('dashboard.tooling_telemetry_family_infra'),
                communication: t('dashboard.tooling_telemetry_family_communication'),
                automation: t('dashboard.tooling_telemetry_family_automation'),
                media: t('dashboard.tooling_telemetry_family_media'),
                misc: t('dashboard.tooling_telemetry_family_misc'),
            };
            return labels[key] || key;
        }

        function classifyToolFamily(tool) {
            const name = String(tool || '').toLowerCase();
            if (!name) return 'misc';
            if (name.startsWith('file') || name === 'filesystem' || name.includes('_editor') || name === 'pdf_operations' || name === 'detect_file_type' || name === 'archive') return 'files';
            if (name.includes('shell') || name.includes('sudo') || name === 'process_analyzer' || name === 'process_management') return 'shell';
            if (name.includes('python') || name.includes('sandbox') || name.includes('skill') || name.includes('generate_image') || name === 'document_creator') return 'coding';
            if (name.includes('memory') || name === 'remember' || name === 'knowledge_graph' || name === 'cheatsheet' || name.includes('journal') || name.includes('notes')) return 'memory';
            if (name.includes('web_') || name === 'site_crawler' || name === 'api_request' || name === 'virustotal_scan' || name === 'form_automation') return 'web';
            if (name.includes('homepage') || name === 'netlify' || name.includes('update') || name === 'cloudflare_tunnel') return 'deployment';
            if (name.includes('network') || name.includes('dns_') || name.includes('port_') || name.includes('mdns_') || name.includes('whois') || name.includes('upnp') || name.includes('wake_on_lan') || name.includes('fritzbox')) return 'network';
            if (name === 'docker' || name === 'proxmox' || name === 'tailscale' || name === 'ansible' || name === 'github' || name === 'mcp_call' || name.startsWith('sql_') || name === 'manage_sql_connections' || name.includes('meshcentral') || name.includes('remote_') || name === 'invasion_control' || name === 'home_assistant' || name === 'ollama' || name === 'adguard' || name.startsWith('mqtt_') || name === 's3_storage') return 'infra';
            if (name.includes('email') || name.includes('webhook') || name.includes('telnyx') || name === 'address_book') return 'communication';
            if (name.includes('cron') || name.includes('follow_up') || name.includes('mission') || name === 'co_agent' || name === 'co_agents') return 'automation';
            if (name.includes('image') || name.includes('audio') || name === 'tts' || name.includes('transcribe') || name.includes('media_')) return 'media';
            return 'misc';
        }

        function renderToolFamilies(container, byTool) {
            if (!container) return;
            const families = new Map();
            Object.entries(byTool || {}).forEach(([tool, stats]) => {
                const family = classifyToolFamily(tool);
                if (!families.has(family)) {
                    families.set(family, { family, total: 0, failures: 0, tools: 0 });
                }
                const entry = families.get(family);
                entry.total += Number(stats?.total_calls || 0);
                entry.failures += Number(stats?.failure_count || 0);
                entry.tools += 1;
            });

            const ranked = Array.from(families.values())
                .filter(item => item.total > 0)
                .map(item => ({
                    ...item,
                    failureRate: item.total > 0 ? item.failures / item.total : 0,
                }))
                .sort((a, b) =>
                    (b.failureRate - a.failureRate) ||
                    (b.failures - a.failures) ||
                    (b.total - a.total) ||
                    a.family.localeCompare(b.family)
                );

            if (!ranked.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_no_tool_families') + '</div>';
                return;
            }

            container.innerHTML = '<div class="tooling-telemetry-family-list">' + renderCollapsibleList(ranked, item => {
                const risk = item.failureRate >= 0.35 ? t('dashboard.tooling_telemetry_family_risk_high')
                    : item.failureRate >= 0.15 ? t('dashboard.tooling_telemetry_family_risk_medium')
                    : t('dashboard.tooling_telemetry_family_risk_low');
                return `
                    <div class="tooling-telemetry-family-item">
                        <div>
                            <span class="tooling-telemetry-family-name">${esc(toolingTelemetryFamilyLabel(item.family))}</span>
                            <span class="tooling-telemetry-family-meta">${esc(t('dashboard.tooling_telemetry_family_meta', { fails: item.failures, calls: item.total, tools: item.tools }))}</span>
                        </div>
                        <div class="tooling-telemetry-family-side">
                            <span class="tooling-telemetry-family-rate">${esc(t('dashboard.tooling_telemetry_failure_rate_short', { pct: (item.failureRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-family-risk">${esc(risk)}</span>
                        </div>
                    </div>
                `;
            }, 6) + '</div>';
        }

        function renderTelemetryScopes(container, scopes) {
            if (!container) return;
            if (!scopes.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_no_scopes') + '</div>';
                return;
            }

            container.innerHTML = '<div class="tooling-telemetry-scope-list">' + renderCollapsibleList(scopes, scope => {
                const parseTotal = Object.values(scope.parse_sources || {}).reduce((sum, count) => sum + Number(count || 0), 0);
                const recoveryTotal = Object.values(scope.recovery_events || {}).reduce((sum, count) => sum + Number(count || 0), 0);
                const policyTotal = Object.values(scope.policy_events || {}).reduce((sum, count) => sum + Number(count || 0), 0);
                const retrieval = toolingTelemetrySummarizeRetrieval(scope.retrieval_events || {});
                const provider = scope.provider_type || t('dashboard.tooling_telemetry_none');
                const model = scope.model || t('dashboard.tooling_telemetry_none');
                return `
                    <div class="tooling-telemetry-scope-item">
                        <div class="tooling-telemetry-scope-head">
                            <div>
                                <span class="tooling-telemetry-scope-model">${esc(model)}</span>
                                <span class="tooling-telemetry-scope-provider">${esc(provider)}</span>
                            </div>
                            <span class="tooling-telemetry-scope-total">${t('dashboard.tooling_telemetry_total_events', { count: Number(scope.total_events || 0).toLocaleString() })}</span>
                        </div>
                        <div class="tooling-telemetry-scope-meta">
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_parse_sources_short', { count: parseTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_recovery_events_short', { count: recoveryTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_policy_events_short', { count: policyTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_retrieval_events_short', { count: retrieval.total }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_retrieval_share_short', { pct: retrieval.avgShare.toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_tool_calls_short', { count: Number(scope.tool_calls || 0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_success_rate_short', { pct: ((Number(scope.success_rate || 0) * 100).toFixed(0)) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_failure_rate_short', { pct: ((Number(scope.failure_rate || 0) * 100).toFixed(0)) }))}</span>
                        </div>
                    </div>
                `;
            }, 6) + '</div>';
        }

        function toolingTelemetryScopeStatus(item) {
            if (item.failureRate >= 0.4 || item.recoveryRate >= 0.5 || item.fallbackRate >= 0.7) {
                return {
                    key: 'struggling',
                    label: t('dashboard.tooling_telemetry_status_struggling'),
                    className: 'is-struggling',
                };
            }
            if (item.failureRate >= 0.2 || item.recoveryRate >= 0.25 || item.fallbackRate >= 0.45) {
                return {
                    key: 'mixed',
                    label: t('dashboard.tooling_telemetry_status_mixed'),
                    className: 'is-mixed',
                };
            }
            return {
                key: 'stable',
                label: t('dashboard.tooling_telemetry_status_stable'),
                className: 'is-stable',
            };
        }

        function renderTelemetryComparison(container, scopes) {
            if (!container) return;
            if (!scopes.length) {
                container.innerHTML = '<div class="empty-state dash-empty-tight">' + t('dashboard.tooling_telemetry_no_model_comparison') + '</div>';
                return;
            }

            const comparison = scopes.map(scope => {
                const parseSources = scope.parse_sources || {};
                const recoveryEvents = scope.recovery_events || {};
                const policyEvents = scope.policy_events || {};
                const retrieval = toolingTelemetrySummarizeRetrieval(scope.retrieval_events || {});
                const parseTotal = Object.values(parseSources).reduce((sum, count) => sum + Number(count || 0), 0);
                const nativeCount = Number(parseSources.native || 0);
                const fallbackCount = Math.max(0, parseTotal - nativeCount);
                const recoveryTotal = Object.values(recoveryEvents).reduce((sum, count) => sum + Number(count || 0), 0);
                const policyTotal = Object.values(policyEvents).reduce((sum, count) => sum + Number(count || 0), 0);
                const toolCalls = Number(scope.tool_calls || 0);
                return {
                    provider: scope.provider_type || t('dashboard.tooling_telemetry_none'),
                    model: scope.model || t('dashboard.tooling_telemetry_none'),
                    successRate: Number(scope.success_rate || 0),
                    failureRate: Number(scope.failure_rate || 0),
                    fallbackRate: parseTotal > 0 ? fallbackCount / parseTotal : 0,
                    recoveryRate: toolCalls > 0 ? recoveryTotal / toolCalls : 0,
                    retrievalTotal: retrieval.total,
                    retrievalAvgShare: retrieval.avgShare,
                    toolCalls,
                    policyTotal,
                    totalEvents: Number(scope.total_events || 0),
                };
            }).sort((a, b) =>
                (b.failureRate - a.failureRate) ||
                (b.retrievalAvgShare - a.retrievalAvgShare) ||
                (b.recoveryRate - a.recoveryRate) ||
                (b.fallbackRate - a.fallbackRate) ||
                (b.toolCalls - a.toolCalls) ||
                a.model.localeCompare(b.model)
            );

            container.innerHTML = '<div class="tooling-telemetry-compare-list">' + renderCollapsibleList(comparison, item => {
                const status = toolingTelemetryScopeStatus(item);
                return `
                    <div class="tooling-telemetry-compare-item">
                        <div class="tooling-telemetry-compare-head">
                            <div>
                                <span class="tooling-telemetry-compare-model">${esc(item.model)}</span>
                                <span class="tooling-telemetry-compare-provider">${esc(item.provider)}</span>
                            </div>
                            <span class="tooling-telemetry-compare-status ${status.className}">${esc(status.label)}</span>
                        </div>
                        <div class="tooling-telemetry-compare-meta">
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_success_rate_short', { pct: (item.successRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_failure_rate_short', { pct: (item.failureRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_fallback_rate_short', { pct: (item.fallbackRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_recovery_rate_short', { pct: (item.recoveryRate * 100).toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_retrieval_events_short', { count: item.retrievalTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_retrieval_share_short', { pct: item.retrievalAvgShare.toFixed(0) }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_policy_events_short', { count: item.policyTotal }))}</span>
                            <span class="tooling-telemetry-scope-pill">${esc(t('dashboard.tooling_telemetry_tool_calls_short', { count: item.toolCalls }))}</span>
                        </div>
                    </div>
                `;
            }, 6) + '</div>';
        }

        function renderToolingTelemetry(data) {
            const card = document.getElementById('card-tooling-telemetry');
            const telemetry = data?.agent_telemetry || {};
            const parseSources = Object.entries(telemetry.parse_sources || {}).filter(([, count]) => Number(count) > 0);
            const recoveryEvents = Object.entries(telemetry.recovery_events || {}).filter(([, count]) => Number(count) > 0);
            const policyEvents = Object.entries(telemetry.policy_events || {}).filter(([, count]) => Number(count) > 0);
            const retrievalSummary = toolingTelemetrySummarizeRetrieval(telemetry.retrieval_events || {});
            const scopes = Array.isArray(telemetry.scopes) ? telemetry.scopes.filter(scope => Number(scope?.total_events || 0) > 0) : [];
            const failingTools = Object.entries(data?.by_tool || {})
                .map(([tool, stats]) => ({
                    tool,
                    failures: Number(stats?.failure_count || 0),
                    total: Number(stats?.total_calls || 0),
                }))
                .filter(item => item.failures > 0)
                .sort((a, b) => (b.failures - a.failures) || (b.total - a.total) || a.tool.localeCompare(b.tool));

            if (!parseSources.length && !recoveryEvents.length && !policyEvents.length && !retrievalSummary.visibleEntries.length && !failingTools.length && !scopes.length) {
                dashSetHidden(card, true);
                return;
            }

            dashSetHidden(card, false);
            parseSources.sort((a, b) => Number(b[1]) - Number(a[1]));
            recoveryEvents.sort((a, b) => Number(b[1]) - Number(a[1]));
            policyEvents.sort((a, b) => Number(b[1]) - Number(a[1]));

            renderToolingSummary(document.getElementById('tooling-telemetry-summary'), parseSources, recoveryEvents, policyEvents, retrievalSummary, failingTools);
            renderTelemetryGroup(document.getElementById('tooling-telemetry-parse'), parseSources, toolingTelemetryParseLabel);
            renderTelemetryGroup(document.getElementById('tooling-telemetry-recovery'), recoveryEvents, toolingTelemetryRecoveryLabel);
            renderTelemetryGroup(document.getElementById('tooling-telemetry-policy'), policyEvents, toolingTelemetryPolicyLabel);
            renderTelemetryGroup(document.getElementById('tooling-telemetry-retrieval'), retrievalSummary.visibleEntries, toolingTelemetryRetrievalLabel);
            renderFailureTools(document.getElementById('tooling-telemetry-failures'), failingTools);
            renderToolFamilies(document.getElementById('tooling-telemetry-families'), data?.by_tool || {});
            renderTelemetryScopes(document.getElementById('tooling-telemetry-scopes'), scopes);
            renderTelemetryComparison(document.getElementById('tooling-telemetry-comparison'), scopes);
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // LOG VIEWER
        // ══════════════════════════════════════════════════════════════════════════════

        let logData = [];

        function renderLogs(data) {
            if (!data) return;
            logData = data.lines || [];
            applyLogFilter();
            const meta = document.getElementById('log-meta');
            if (meta) meta.textContent = t('dashboard.logs_meta', {count: data.count || 0, file: data.log_file || 'unknown'});
        }

        function applyLogFilter() {
            const viewer = document.getElementById('log-viewer');
            const filterStr = (document.getElementById('log-filter')?.value || '').trim();
            let regex = null;
            if (filterStr) {
                try { regex = new RegExp(filterStr, 'i'); } catch (e) { /* ignore invalid regex */ }
            }

            const lines = regex ? logData.filter(l => regex.test(l)) : logData;

            if (lines.length === 0) {
                viewer.innerHTML = '<div class="log-line log-line-muted">' + t('dashboard.logs_no_match') + '</div>';
                return;
            }

            viewer.innerHTML = lines.map(line => {
                let cls = '';
                if (/\blevel=ERROR\b/i.test(line)) cls = 'log-level-error';
                else if (/\blevel=WARN/i.test(line)) cls = 'log-level-warn';
                else if (/\blevel=INFO\b/i.test(line)) cls = 'log-level-info';
                else if (/\blevel=DEBUG\b/i.test(line)) cls = 'log-level-debug';
                return `<div class="log-line ${cls}">${esc(line)}</div>`;
            }).join('');
        }

        function scrollLogsToBottom() {
            const viewer = document.getElementById('log-viewer');
            if (viewer) viewer.scrollTop = viewer.scrollHeight;
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // GITHUB REPOS WIDGET
        // ══════════════════════════════════════════════════════════════════════════════

        function renderGitHubRepos(data) {
            const card = document.getElementById('card-github');
            const container = document.getElementById('github-content');
            if (!data || !data.enabled) {
                dashSetHidden(card, true);
                return;
            }
            dashSetHidden(card, false);

            if (data.error) {
                container.innerHTML = `<div class="empty-state">⚠️ ${esc(data.error)}</div>`;
                return;
            }

            const repos = data.repos || [];
            if (repos.length === 0) {
                container.innerHTML = '<div class="empty-state">' + t('dashboard.github_no_repos') + '</div>';
                return;
            }

            // Sort: tracked first, then by updated_at
            repos.sort((a, b) => {
                if (a.tracked && !b.tracked) return -1;
                if (!a.tracked && b.tracked) return 1;
                return (b.updated_at || '').localeCompare(a.updated_at || '');
            });

            const owner = data.owner || '';
            let countInfo = `<div class="gh-count-row">
                <span class="gh-count-text">
                    👤 <strong>${esc(owner)}</strong> — ${t('dashboard.github_repositories_count', {n: repos.length})}
                </span>
            </div>`;

            const renderRepo = r => {
                const vis = r.private ? '<span class="gh-badge gh-badge-private">🔒 ' + t('dashboard.github_badge_private') + '</span>' : '<span class="gh-badge gh-badge-public">🌐 ' + t('dashboard.github_badge_public') + '</span>';
                const tracked = r.tracked ? '<span class="gh-badge gh-badge-tracked">📌 ' + t('dashboard.github_badge_tracked') + '</span>' : '';
                const lang = r.language ? `<span>💻 ${esc(r.language)}</span>` : '';
                const updated = r.updated_at ? `<span>🕐 ${new Date(r.updated_at).toLocaleDateString()}</span>` : '';
                const desc = r.description ? `<div class="gh-repo-desc">${esc(r.description)}</div>` : '';
                return `<div class="gh-repo">
                    <a href="${esc(r.html_url)}" target="_blank" rel="noopener" class="gh-repo-name">
                        📦 ${esc(r.name)} ${vis} ${tracked}
                    </a>
                    ${desc}
                    <div class="gh-repo-meta">${lang} ${updated}</div>
                </div>`;
            };
            container.innerHTML = countInfo + '<div class="gh-repo-list">' + renderCollapsibleList(repos, renderRepo, 6) + '</div>';
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // AGENT STATUS BANNER
        // ══════════════════════════════════════════════════════════════════════════════

        function renderAgentBanner(overview, contextChars) {
            if (!overview) return;
            const a = overview.agent || {};
            // Model
            const modelEl = document.getElementById('ab-model');
            if (modelEl) modelEl.textContent = a.model || '—';

            // Status dot
            const dot = document.getElementById('ab-status-dot');
            if (dot) {
                if (a.busy) { dot.className = 'status-dot yellow'; dot.title = t('dashboard.agent_banner_status_busy'); }
                else if (a.maintenance) { dot.className = 'status-dot yellow'; dot.title = t('dashboard.agent_banner_status_maintenance'); }
                else if (a.debug) { dot.className = 'status-dot red'; dot.title = t('dashboard.agent_banner_status_debug'); }
                else { dot.className = 'status-dot green'; dot.title = t('dashboard.agent_banner_status_ready'); }
            }

            // Personality
            const persEl = document.getElementById('ab-personality');
            if (persEl) persEl.textContent = a.personality || t('dashboard.agent_banner_personality_default');

            // Context gauge
            const ctxWindow = a.context_window || 128000;
            const ctxChars = contextChars || (overview.context?.total_chars || 0);
            // Rough estimate: ~4 chars per token
            const ctxTokensEst = Math.round(ctxChars / 4);
            const ctxPct = ctxWindow > 0 ? Math.min(100, Math.round((ctxTokensEst / ctxWindow) * 100)) : 0;
            const ctxFill = document.getElementById('ab-ctx-fill');
            const ctxPctEl = document.getElementById('ab-ctx-pct');
            if (ctxFill) {
                ctxFill.className = 'agent-banner-ctx-fill w-pct-' + ctxPct + (ctxPct > 80 ? ' ctx-level-high' : ctxPct > 60 ? ' ctx-level-med' : ' ctx-level-ok');
            }
            if (ctxPctEl) ctxPctEl.textContent = ctxPct + '%';

            // Integration count
            const intEl = document.getElementById('ab-integrations');
            if (intEl && overview.integrations) {
                const visibleIntegrations = Object.entries(overview.integrations).filter(([key]) => !HIDDEN_INTEGRATIONS.has(key));
                const active = visibleIntegrations.filter(([, value]) => value).length;
                const total = visibleIntegrations.length;
                intEl.innerHTML = `🔌 <strong>${active}</strong>/${total} ${t('dashboard.agent_banner_integrations')}`;
            }

            // Last activity
            const actEl = document.getElementById('ab-last-activity');
            if (actEl) {
                const h = overview.last_activity_hours;
                if (h >= 0 && h < 1) actEl.innerHTML = '💬 <span>' + t('dashboard.agent_banner_just_active') + '</span>';
                else if (h >= 1 && h < 24) actEl.innerHTML = '💬 <span>' + t('dashboard.agent_banner_hours_ago', {n: Math.round(h)}) + '</span>';
                else if (h >= 24) actEl.innerHTML = '💬 <span>' + t('dashboard.agent_banner_days_ago', {n: Math.round(h / 24)}) + '</span>';
                else actEl.innerHTML = '💬 <span>' + t('dashboard.agent_banner_no_activity') + '</span>';
            }
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // OPERATIONS & INTEGRATIONS
        // ══════════════════════════════════════════════════════════════════════════════

        function renderOperations(overview) {
            if (!overview) return;
            const grid = document.getElementById('ops-grid');
            const m = overview.missions || {};
            const inv = overview.invasion || {};
            const idx = overview.indexer || {};
            const mq = overview.mqtt || {};
            const notes = overview.notes || {};
            const sec = overview.security || {};

            const items = [
                { icon: '🚀', val: m.total || 0, lbl: t('dashboard.operations_missions'), sub: m.running ? t('dashboard.operations_missions_active', {n: m.running}) : (m.queued ? t('dashboard.operations_missions_queued', {n: m.queued}) : t('dashboard.operations_missions_active', {n: m.enabled || 0})) },
                { icon: '🥚', val: inv.nests || 0, lbl: t('dashboard.operations_nests'), sub: t('dashboard.operations_eggs_connected', {n: inv.connected_eggs || 0}) },
                { icon: '📂', val: idx.indexed_files || 0, lbl: t('dashboard.operations_indexed'), sub: idx.enabled ? t('dashboard.operations_of_files', {n: idx.total_files || 0}) : t('dashboard.operations_disabled') },
                { icon: '📡', val: mq.connected ? '✓' : '✗', lbl: t('dashboard.operations_mqtt'), sub: mq.enabled ? t('dashboard.operations_buffered', {n: mq.buffer || 0}) : t('dashboard.operations_disabled') },
                { icon: '�', val: notes.open || 0, lbl: t('dashboard.operations_notes_open'), sub: t('dashboard.operations_notes_sub', {total: notes.total || 0, done: notes.done || 0}) },
                { icon: '🔐', val: sec.vault_keys || 0, lbl: t('dashboard.operations_vault_keys'), sub: t('dashboard.operations_api_tokens', {n: sec.tokens || 0}) },
                { icon: '📱', val: overview.devices || 0, lbl: t('dashboard.operations_devices'), sub: t('dashboard.operations_inventory') },
                { icon: '🧠', val: overview.context?.has_summary ? '✓' : '✗', lbl: t('dashboard.operations_summary'), sub: t('dashboard.operations_chars_count', {n: ((overview.context?.total_chars || 0) / 1000).toFixed(1)}) },
                { icon: '📋', val: (overview.cheatsheets?.total || 0), lbl: t('dashboard.operations_cheatsheets'), sub: t('dashboard.operations_cheatsheets_active', {n: overview.cheatsheets?.active || 0}) },
                { icon: '🌐', val: overview.tunnel?.running ? '✓' : '✗', lbl: t('dashboard.operations_tunnel'), sub: overview.tunnel?.url ? truncate(overview.tunnel.url, 24) : t('dashboard.operations_disabled') },
            ];

            grid.innerHTML = items.map(s =>
                `<div class="ops-stat">
                    <div class="ops-stat-icon">${s.icon}</div>
                    <div class="ops-stat-val">${s.val}</div>
                    <div class="ops-stat-lbl">${s.lbl}</div>
                    <div class="ops-stat-sub">${s.sub}</div>
                </div>`
            ).join('');
        }

        function renderQuickStatus(overview) {
            const el = document.getElementById('qs-grid');
            if (!el || !overview) return;

            const tunnel = overview.tunnel || {};
            const mqtt = overview.mqtt || {};
            const sec = overview.security || {};
            const m = overview.missions || {};
            const sk = overview.skills || {};
            const integrations = overview.integrations || {};
            const visibleIntegrations = Object.entries(integrations).filter(([key]) => !HIDDEN_INTEGRATIONS.has(key));
            const activeInts = visibleIntegrations.filter(([, value]) => value).length;
            const totalInts = visibleIntegrations.length;

            const items = [
                {
                    icon: '🌐',
                    lbl: t('dashboard.operations_tunnel'),
                    val: tunnel.running ? t('dashboard.quickstatus_online') : t('dashboard.quickstatus_offline'),
                    status: tunnel.running ? 'ok' : 'offline',
                    info: tunnel.running && tunnel.url ? truncate(tunnel.url, 24) : ''
                },
                {
                    icon: '📡',
                    lbl: t('dashboard.integration_mqtt'),
                    val: mqtt.enabled ? (mqtt.connected ? t('dashboard.quickstatus_connected') : t('dashboard.quickstatus_not_connected')) : t('dashboard.operations_disabled'),
                    status: mqtt.enabled ? (mqtt.connected ? 'ok' : 'warning') : 'neutral',
                    info: (mqtt.enabled && mqtt.buffer) ? `${mqtt.buffer} buffered` : ''
                },
                {
                    icon: '🔗',
                    lbl: t('dashboard.quickstatus_integrations'),
                    val: `${activeInts} / ${totalInts}`,
                    status: 'neutral',
                    info: ''
                },
                {
                    icon: '🚀',
                    lbl: t('dashboard.operations_missions'),
                    val: `${m.running || 0} / ${m.total || 0}`,
                    status: (m.running || 0) > 0 ? 'ok' : 'neutral',
                    info: m.queued ? `${m.queued} queued` : ''
                },
                {
                    icon: '🔐',
                    lbl: t('dashboard.operations_vault_keys'),
                    val: sec.vault_keys || 0,
                    status: 'neutral',
                    info: sec.tokens ? `${sec.tokens} tokens` : ''
                },
                {
                    icon: '📱',
                    lbl: t('dashboard.operations_devices'),
                    val: overview.devices || 0,
                    status: 'neutral',
                    info: ''
                },
                {
                    icon: '🧩',
                    lbl: t('dashboard.quickstatus_skills'),
                    val: sk.total || 0,
                    status: sk.pending > 0 ? 'warning' : 'neutral',
                    info: sk.pending > 0 ? `${sk.pending} pending` : ''
                },
                {
                    icon: '🪶',
                    lbl: t('dashboard.integration_helper_llm'),
                    val: integrations.helper_llm ? t('dashboard.helper_llm_state_enabled') : t('dashboard.helper_llm_state_disabled'),
                    status: integrations.helper_llm ? 'ok' : 'neutral',
                    info: ''
                },
            ];

            // Add daemon health item only when daemons are configured
            const dm = overview.daemons || {};
            if ((dm.total || 0) > 0) {
                const autoDisabled = dm.auto_disabled || 0;
                const running = dm.running || 0;
                const total = dm.total || 0;
                items.push({
                    icon: '👹',
                    lbl: t('dashboard.quickstatus_daemons') || 'Daemons',
                    val: `${running} / ${total}`,
                    status: autoDisabled > 0 ? 'warning' : (running > 0 ? 'ok' : 'neutral'),
                    info: autoDisabled > 0 ? `${autoDisabled} auto-disabled` : ''
                });
            }

            el.innerHTML = items.map(s =>
                `<div class="qs-item ${s.status}">
                    <div class="qs-icon">${s.icon}</div>
                    <div class="qs-label">${s.lbl}</div>
                    <div class="qs-val">${s.val}</div>
                    ${s.info ? `<div class="qs-info">${esc(s.info)}</div>` : ''}
                </div>`
            ).join('');
        }

        function renderOptimizationStats(opt) {
            const el = document.getElementById('opt-grid');
            if (!el || !opt) return;

            const items = [
                {
                    lbl: t('dashboard.opt_active_overrides'),
                    val: opt.active_overrides || 0
                },
                {
                    lbl: t('dashboard.opt_running_shadows'),
                    val: opt.running_shadows || 0
                },
                {
                    lbl: t('dashboard.opt_rejected_mutations'),
                    val: opt.rejected_mutations || 0
                },
                {
                    lbl: t('dashboard.opt_total_trace_events'),
                    val: opt.total_trace_events || 0
                },
                {
                    lbl: t('dashboard.opt_global_success_rate'),
                    val: `${(opt.global_success_rate || 0).toFixed(1)}%`
                }
            ];

            el.innerHTML = items.map(s =>
                `<div class="stat-item">
                    <div class="stat-value">${s.val}</div>
                    <div class="stat-label">${s.lbl}</div>
                </div>`
            ).join('');
        }

        // ── Output Compression Stats ───────────────────────────────────────────
        function formatChars(n) {
            if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
            if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
            return String(n);
        }

        function renderCompressionStats(comp) {
            const statsEl = document.getElementById('compression-stats');
            const detailsEl = document.getElementById('compression-details');
            const emptyEl = document.getElementById('compression-empty');
            const statusBadge = document.getElementById('compression-status-badge');
            if (!statsEl || !comp) return;

            // Enabled Status Badge aktualisieren
            if (statusBadge) {
                const enabled = comp.enabled !== false;
                statusBadge.className = `status-badge ${enabled ? 'is-active' : 'is-disabled'}`;
                statusBadge.textContent = enabled ? t('dashboard.compression_enabled') : t('dashboard.compression_disabled');
            }

            const applied = comp.compressions_applied || 0;
            const skipped = comp.compressions_skipped || 0;
            const total = applied + skipped;

            // No data yet
            if (total === 0) {
                if (statsEl) statsEl.innerHTML = '';
                if (detailsEl) detailsEl.classList.add('is-hidden');
                if (emptyEl) emptyEl.classList.remove('is-hidden');
                return;
            }
            if (emptyEl) emptyEl.classList.add('is-hidden');
            if (detailsEl) detailsEl.classList.remove('is-hidden');

            const savedChars = comp.total_saved_chars || 0;
            const ratio = comp.average_savings_ratio || 0;
            const rawChars = comp.total_raw_chars || 0;
            const compressedChars = comp.total_compressed_chars || 0;
            const avgProcessing = comp.average_processing_ms || 0;
            const errors = comp.errors_count || 0;

            const items = [
                { lbl: t('dashboard.compression_saved_chars'), val: formatChars(savedChars) },
                { lbl: t('dashboard.compression_savings_ratio'), val: (ratio * 100).toFixed(1) + '%' },
                { lbl: t('dashboard.compression_applied'), val: applied },
                { lbl: t('dashboard.compression_skipped'), val: skipped },
                { lbl: t('dashboard.compression_raw_chars'), val: formatChars(rawChars) },
                { lbl: t('dashboard.compression_compressed_chars'), val: formatChars(compressedChars) },
                { lbl: t('dashboard.compression_avg_processing_ms'), val: avgProcessing.toFixed(2) + ' ms' },
            ];

            // Errors nur anzeigen wenn > 0
            if (errors > 0) {
                items.push({ lbl: t('dashboard.compression_errors'), val: errors, class: 'is-error' });
            }

            statsEl.innerHTML = items.map(s =>
                `<div class="stat-item">
                    <div class="stat-value">${s.val}</div>
                    <div class="stat-label">${s.lbl}</div>
                </div>`
            ).join('');

            // Top tools
            const toolsList = document.getElementById('compression-tools-list');
            const toolsWrap = document.getElementById('compression-tools-wrap');
            const topTools = (comp.top_tools || []).slice(0, 5);
            if (toolsWrap) toolsWrap.classList.toggle('is-hidden', topTools.length === 0);
            if (toolsList && topTools.length > 0) {
                toolsList.innerHTML = topTools.map(e =>
                    `<div class="compression-rank-item">
                        <span class="compression-rank-name">${esc(e.tool)}</span>
                        <span class="compression-rank-bar" style="width:${Math.max(4, (e.savings_ratio || 0) * 100)}%"></span>
                        <span class="compression-rank-val">${formatChars(e.saved_chars)} (${(e.savings_ratio * 100).toFixed(0)}%)</span>
                    </div>`
                ).join('');
            }

            // Top filters
            const filtersList = document.getElementById('compression-filters-list');
            const filtersWrap = document.getElementById('compression-filters-wrap');
            const topFilters = (comp.top_filters || []).slice(0, 5);
            if (filtersWrap) filtersWrap.classList.toggle('is-hidden', topFilters.length === 0);
            if (filtersList && topFilters.length > 0) {
                const maxSaved = Math.max(1, topFilters[0].saved_chars || 1);
                filtersList.innerHTML = topFilters.map(e =>
                    `<div class="compression-rank-item">
                        <span class="compression-rank-name">${esc(e.filter)}</span>
                        <span class="compression-rank-bar" style="width:${Math.max(4, ((e.saved_chars || 0) / maxSaved) * 100)}%"></span>
                        <span class="compression-rank-val">${formatChars(e.saved_chars)} ×${e.count || 0}</span>
                    </div>`
                ).join('');
            }
        }

        function renderIntegrations(overview) {
            if (!overview || !overview.integrations) return;
            const grid = document.getElementById('integration-grid');
            const icons = {
                telegram: '📱', discord: '💬', email: '📧', home_assistant: '🏠',
                docker: '🐳', co_agents: '🤖', webhooks: '🔗', webdav: '☁️',
                koofr: '☁️', chromecast: '📺', proxmox: '🖥️', ollama: '🧠',
                rocketchat: '💬', tailscale: '🔒', ansible: '🔧', invasion: '🥚',
                github: '🐙', mqtt: '📡', budget: '💰', indexing: '📂',
                auth: '🔑', fallback_llm: '🔄', helper_llm: '🪶', personality_v2: '🎭', user_profiling: '👤', tts: '🔊',
                piper_tts: '🗣️',
                paperless_ngx: '📄', cloudflare_tunnel: '☁️',
                n8n: '🔀', fritzbox: '📡', meshcentral: '🖥️', a2a: '🔗',
                adguard: '🛡️', s3: '🪣', mcp: '🔌', mcp_server: '🔌',
                memory_analysis: '🧠', llm_guardian: '🛡️', security_proxy: '🔐',
                sandbox: '📦', ai_gateway: '🌐', image_generation: '🎨',
                google_workspace: '📧', netlify: '🚀',
                homepage: '🏠', virustotal: '🦠', brave_search: '🔍',
                firewall: '🔥', remote_control: '🖥️', web_scraper: '🕷️',
                skill_manager: '🧩'
            };
            const names = {
                telegram: t('dashboard.integration_telegram'), discord: t('dashboard.integration_discord'),
                email: t('dashboard.integration_email'), home_assistant: t('dashboard.integration_home_assistant'),
                docker: t('dashboard.integration_docker'), co_agents: t('dashboard.integration_co_agents'),
                webhooks: t('dashboard.integration_webhooks'), webdav: t('dashboard.integration_webdav'),
                koofr: t('dashboard.integration_koofr'), chromecast: t('dashboard.integration_chromecast'),
                proxmox: t('dashboard.integration_proxmox'), ollama: t('dashboard.integration_ollama'),
                rocketchat: t('dashboard.integration_rocketchat'), tailscale: t('dashboard.integration_tailscale'),
                ansible: t('dashboard.integration_ansible'), invasion: t('dashboard.integration_invasion'),
                github: t('dashboard.integration_github'), mqtt: t('dashboard.integration_mqtt'),
                budget: t('dashboard.integration_budget'), indexing: t('dashboard.integration_indexing'),
                auth: t('dashboard.integration_auth'), fallback_llm: t('dashboard.integration_fallback_llm'),
                helper_llm: t('dashboard.integration_helper_llm'),
                personality_v2: t('dashboard.integration_personality_v2'), user_profiling: t('dashboard.integration_user_profiling'),
                tts: t('dashboard.integration_tts'),
                piper_tts: t('dashboard.integration_piper_tts'),
                paperless_ngx: t('dashboard.integration_paperless_ngx'),
                cloudflare_tunnel: t('dashboard.integration_cloudflare_tunnel'),
                n8n: t('dashboard.integration_n8n'), fritzbox: t('dashboard.integration_fritzbox'),
                meshcentral: t('dashboard.integration_meshcentral'), a2a: t('dashboard.integration_a2a'),
                adguard: t('dashboard.integration_adguard'), s3: t('dashboard.integration_s3'),
                mcp: t('dashboard.integration_mcp'), mcp_server: t('dashboard.integration_mcp_server'),
                memory_analysis: t('dashboard.integration_memory_analysis'),
                llm_guardian: t('dashboard.integration_llm_guardian'),
                security_proxy: t('dashboard.integration_security_proxy'),
                sandbox: t('dashboard.integration_sandbox'), ai_gateway: t('dashboard.integration_ai_gateway'),
                image_generation: t('dashboard.integration_image_generation'),
                google_workspace: t('dashboard.integration_google_workspace'),
                netlify: t('dashboard.integration_netlify'),
                homepage: t('dashboard.integration_homepage'), virustotal: t('dashboard.integration_virustotal'),
                brave_search: t('dashboard.integration_brave_search'), firewall: t('dashboard.integration_firewall'),
                remote_control: t('dashboard.integration_remote_control'),
                web_scraper: t('dashboard.integration_web_scraper'),
                skill_manager: t('dashboard.integration_skill_manager')
            };

            // Sort: active first
            const sorted = Object.entries(overview.integrations)
                .filter(([key]) => !HIDDEN_INTEGRATIONS.has(key))
                .sort((a, b) => (b[1] ? 1 : 0) - (a[1] ? 1 : 0));
            grid.innerHTML = sorted.map(([key, active]) => {
                let cls = active ? 'active' : 'inactive';
                // MQTT: distinguish "enabled but disconnected" from "enabled and connected"
                if (key === 'mqtt' && active && overview.mqtt && overview.mqtt.connected === false) cls = 'active-warning';
                return `<span class="int-badge ${cls}">${icons[key] || '•'} ${names[key] || key}</span>`;
            }).join('');
        }

        // ── Helpers (esc() is now provided by shared.js) ─────────────────────

        function truncate(s, max) {
            if (!s || s.length <= max) return s || '';
            return s.substring(0, max) + '…';
        }

        // ── LLM Guardian Card ───────────────────────────────────────────────────────

        async function loadGuardianCard() {
            const data = await API.get('/api/dashboard/guardian');
            const card = document.getElementById('card-guardian');
            if (!data || !data.enabled) {
                dashSetHidden(card, true);
                return;
            }
            dashSetHidden(card, false);
            renderGuardianCard(data);
        }

        async function loadHelperLLMCard() {
            const data = await API.get('/api/dashboard/helper-llm');
            renderHelperLLMCard(data);
        }

        // ── Daemon Skills Card ──────────────────────────────────────────────

        async function loadDaemonsCard() {
            try {
                const resp = await fetch('/api/daemons');
                if (!resp.ok) return;
                const data = await resp.json();
                const daemons = data.daemons || data || [];
                if (!Array.isArray(daemons) || daemons.length === 0) return;
                renderDaemonsCard(daemons);
            } catch (_) {}
        }

        function renderDaemonsCard(daemons) {
            const card = document.getElementById('card-daemons');
            if (!card) return;
            card.classList.remove('is-hidden');

            const summaryEl = document.getElementById('daemon-summary');
            const listEl = document.getElementById('daemon-list');

            const running = daemons.filter(d => ['running', 'starting'].includes((d.status || '').toLowerCase())).length;
            const stopped = daemons.filter(d => (d.status || '').toLowerCase() === 'stopped').length;
            const errored = daemons.filter(d => (d.status || '').toLowerCase() === 'error').length;
            const autoDisabled = daemons.filter(d => d.auto_disabled || (d.status || '').toLowerCase() === 'disabled').length;

            summaryEl.innerHTML = `
                <div class="guardian-metrics-grid">
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${daemons.length}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.daemons_total') || 'Total'}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val ok">${running}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.daemons_running') || 'Running'}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${stopped}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.daemons_stopped') || 'Stopped'}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val${errored > 0 ? ' warn' : ''}">${errored}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.daemons_error') || 'Error'}</div>
                    </div>
                </div>`;

            // Auto-disabled alert banner
            const alertHTML = autoDisabled > 0 ? `
                <div class="daemon-disabled-alert">
                    ⚠ ${t('dashboard.daemons_auto_disabled_alert') || `${autoDisabled} daemon(s) auto-disabled — re-enable via Skills page`}
                </div>` : '';

            const statusIcon = { running: '🟢', starting: '🟡', stopped: '⏹', error: '🔴', disabled: '⛔' };

            const rowsHTML = renderCollapsibleList(daemons, d => {
                const s = (d.status || 'stopped').toLowerCase();
                const icon = statusIcon[s] || '⏹';
                const name = esc(d.skill_name || d.skill_id || '?');

                // Uptime from started_at
                let uptimeHtml = '';
                if (d.started_at && (s === 'running' || s === 'starting')) {
                    const ms = Date.now() - Date.parse(d.started_at);
                    uptimeHtml = `<span class="daemon-uptime">${formatDuration(ms)}</span>`;
                }

                // Wake-up stats badge
                const wakeCount = d.wake_up_count || 0;
                const suppressedCount = d.suppressed_count || 0;
                const wakeLabel = t('dashboard.daemons_wakeups') || 'Wake-ups';
                const wakeHtml = wakeCount > 0
                    ? `<span class="daemon-badge daemon-badge-wake" title="${wakeLabel}: ${wakeCount}${suppressedCount > 0 ? ` (${suppressedCount} suppressed)` : ''}">💬 ${wakeCount}</span>`
                    : '';

                // Restart count badge
                const restartCount = d.restart_count || 0;
                const restartLabel = t('dashboard.daemons_restarts') || 'Restarts';
                const restartHtml = restartCount > 0
                    ? `<span class="daemon-badge daemon-badge-restart${restartCount >= 3 ? ' warn' : ''}" title="${restartLabel}: ${restartCount}">↻ ${restartCount}</span>`
                    : '';

                // Last wake-up time
                let lastWakeHtml = '';
                if (d.last_wake_up) {
                    const wLabel = t('dashboard.daemons_last_wakeup') || 'Last wake-up';
                    lastWakeHtml = `<span class="daemon-meta-item" title="${wLabel}">${relativeTime(Date.parse(d.last_wake_up))}</span>`;
                }

                // Error detail
                const errHtml = d.last_error
                    ? `<div class="daemon-meta daemon-meta-error"><span title="${esc(d.last_error)}">⚠ ${esc(d.last_error.length > 60 ? d.last_error.substring(0, 60) + '…' : d.last_error)}</span></div>`
                    : '';

                return `<div class="daemon-row${d.auto_disabled ? ' daemon-row-disabled' : ''}">
                    <span class="daemon-icon">${icon}</span>
                    <div class="daemon-row-body">
                        <div class="daemon-row-main">
                            <span class="daemon-name">${name}</span>
                            ${uptimeHtml}
                            ${wakeHtml}
                            ${restartHtml}
                            ${lastWakeHtml}
                        </div>
                        ${errHtml}
                    </div>
                </div>`;
            }, 5);

            listEl.innerHTML = alertHTML + rowsHTML;
        }

        // formatDuration converts milliseconds to a human-readable "Xh Ym" string.
        function formatDuration(ms) {
            const totalSec = Math.floor(ms / 1000);
            const h = Math.floor(totalSec / 3600);
            const m = Math.floor((totalSec % 3600) / 60);
            if (h > 0) return `${h}h ${m}m`;
            if (m > 0) return `${m}m`;
            return `${totalSec}s`;
        }

        function helperLLMOperationLabel(operation) {
            const labels = {
                analyze_turn: t('dashboard.helper_llm_operation_analyze_turn'),
                maintenance_summary_kg: t('dashboard.helper_llm_operation_maintenance_summary_kg'),
                consolidation_batches: t('dashboard.helper_llm_operation_consolidation_batches'),
                compress_memories: t('dashboard.helper_llm_operation_compress_memories'),
                content_summaries: t('dashboard.helper_llm_operation_content_summaries'),
                rag_batch: t('dashboard.helper_llm_operation_rag_batch'),
            };
            return labels[operation] || String(operation || '').replace(/_/g, ' ');
        }

        function helperLLMOperationDescription(operation) {
            const descriptions = {
                analyze_turn: t('dashboard.helper_llm_operation_desc_analyze_turn'),
                maintenance_summary_kg: t('dashboard.helper_llm_operation_desc_maintenance_summary_kg'),
                rag_batch: t('dashboard.helper_llm_operation_desc_rag_batch'),
            };
            const description = descriptions[operation];
            if (!description || description.startsWith('dashboard.')) return '';
            return description;
        }

        function renderHelperLLMCard(data) {
            const statusEl = document.getElementById('helper-llm-status');
            const metricsEl = document.getElementById('helper-llm-metrics');
            const operationsEl = document.getElementById('helper-llm-operations');
            if (!statusEl || !metricsEl || !operationsEl) return;

            const enabled = !!data?.enabled;
            const updatedAt = data?.updated_at ? Date.parse(data.updated_at) : 0;
            const totals = data?.totals || {};
            const operations = Object.entries(data?.operations || {})
                .sort((a, b) => {
                    const reqDiff = Number(b[1]?.requests || 0) - Number(a[1]?.requests || 0);
                    if (reqDiff !== 0) return reqDiff;
                    return a[0].localeCompare(b[0]);
                });

            statusEl.innerHTML = `
                <div class="guardian-status-row">
                    <span class="guardian-lbl">${t('dashboard.helper_llm_state')}:</span>
                    <span class="guardian-val">${enabled ? t('dashboard.helper_llm_state_enabled') : t('dashboard.helper_llm_state_disabled')}</span>
                </div>
                <div class="guardian-status-row">
                    <span class="guardian-lbl">${t('dashboard.helper_llm_last_update')}:</span>
                    <span class="guardian-val">${updatedAt ? relativeTime(updatedAt) : '—'}</span>
                </div>`;

            if (!enabled && !operations.length) {
                metricsEl.innerHTML = `<div class="empty-state">${t('dashboard.helper_llm_disabled')}</div>`;
                operationsEl.innerHTML = '';
                return;
            }

            metricsEl.innerHTML = `
                <div class="guardian-metrics-grid">
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${Number(totals.requests || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_requests')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val ok">${Number(totals.cache_hits || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_cache_hits')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${Number(totals.llm_calls || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_llm_calls')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val warn">${Number(totals.fallbacks || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_fallbacks')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val ok">${Number(totals.saved_calls || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_saved_calls')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${Number(totals.batched_items || 0)}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.helper_llm_batched_items')}</div>
                    </div>
                </div>`;

            if (!operations.length) {
                operationsEl.innerHTML = `<div class="empty-state dash-empty-tight">${t('dashboard.helper_llm_operation_empty')}</div>`;
                return;
            }

            operationsEl.innerHTML = `
                <div class="helper-llm-operation-list">
                    ${renderCollapsibleList(operations, ([name, stats]) => `
                        <div class="helper-llm-operation-item">
                            <div class="helper-llm-operation-head">
                                <span class="helper-llm-operation-name">${escapeHtml(helperLLMOperationLabel(name))}</span>
                                <span class="helper-llm-operation-pill">${Number(stats.requests || 0)} ${t('dashboard.helper_llm_requests')}</span>
                            </div>
                            <div class="helper-llm-operation-meta">
                                <span class="tooling-telemetry-scope-pill">${Number(stats.cache_hits || 0)} ${t('dashboard.helper_llm_cache_hits')}</span>
                                <span class="tooling-telemetry-scope-pill">${Number(stats.llm_calls || 0)} ${t('dashboard.helper_llm_llm_calls')}</span>
                                <span class="tooling-telemetry-scope-pill">${Number(stats.fallbacks || 0)} ${t('dashboard.helper_llm_fallbacks')}</span>
                                <span class="tooling-telemetry-scope-pill">${Number(stats.saved_calls || 0)} ${t('dashboard.helper_llm_saved_calls')}</span>
                                <span class="tooling-telemetry-scope-pill">${Number(stats.batched_items || 0)} ${t('dashboard.helper_llm_batched_items')}</span>
                            </div>
                            ${helperLLMOperationDescription(name) ? `<div class="helper-llm-operation-detail">${escapeHtml(helperLLMOperationDescription(name))}</div>` : ''}
                            ${stats.last_detail ? `<div class="helper-llm-operation-detail">${t('dashboard.helper_llm_operation_last')}: ${escapeHtml(stats.last_detail)}</div>` : ''}
                        </div>
                    `, 5)}
                </div>`;
        }

        function renderGuardianCard(data) {
            const statusEl = document.getElementById('guardian-status');
            const metricsEl = document.getElementById('guardian-metrics');
            if (!statusEl || !metricsEl) return;

            const levelLabels = { off: '⚪ Off', low: '🟢 Low', medium: '🟡 Medium', high: '🔴 High' };
            const fsLabels = { block: '🚫 Block', quarantine: '⚠️ Quarantine', allow: '✅ Allow' };

            statusEl.innerHTML = `
                <div class="guardian-status-row">
                    <span class="guardian-lbl">${t('dashboard.guardian_level')}:</span>
                    <span class="guardian-val">${levelLabels[data.level] || data.level}</span>
                </div>
                <div class="guardian-status-row">
                    <span class="guardian-lbl">${t('dashboard.guardian_failsafe')}:</span>
                    <span class="guardian-val">${fsLabels[data.fail_safe] || data.fail_safe}</span>
                </div>`;

            const m = data.metrics;
            if (!m || m.total_checks === 0) {
                metricsEl.innerHTML = `<div class="empty-state">${t('dashboard.guardian_no_data')}</div>`;
                return;
            }

            metricsEl.innerHTML = `
                <div class="guardian-metrics-grid">
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${m.total_checks}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_total')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val ok">${m.allows}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_allowed')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val warn">${m.quarantines}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_quarantined')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val danger">${m.blocks}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_blocked')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${(m.cache_hit_rate * 100).toFixed(0)}%</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_cache_rate')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${m.total_tokens}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_tokens')}</div>
                    </div>
                    ${m.clarifications ? `<div class="guardian-metric">
                        <div class="guardian-metric-val">${m.clarifications}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_clarifications')}</div>
                    </div>` : ''}
                    ${m.content_scans ? `<div class="guardian-metric">
                        <div class="guardian-metric-val">${m.content_scans}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_content_scans')}</div>
                    </div>` : ''}
                    ${m.errors ? `<div class="guardian-metric">
                        <div class="guardian-metric-val warn">${m.errors}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.guardian_errors')}</div>
                    </div>` : ''}
                </div>
                ${m.last_check_time > 0 ? `<div class="guardian-last-check">${t('dashboard.guardian_last_check')}: ${relativeTime(m.last_check_time)}</div>` : ''}`;
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // INITIALIZATION
        // ══════════════════════════════════════════════════════════════════════════════

        // ── Journal Timeline ────────────────────────────────────────────────────────
        const JOURNAL_ICONS = {
            reflection: '💭', milestone: '🏆', preference: '⭐', task_completed: '✅',
            integration: '🔌', learning: '📚', error_recovery: '🔧', system_event: '⚙️',
            decision: '🎯', error: '❌', budget_exceeded: '💸', security_event: '🔒',
            error_learned: '🧠', alert: '⚠️'
        };

        function renderJournalTimeline(entries) {
            const el = document.getElementById('journal-timeline');
            if (!el) return;
            if (!entries || entries.length === 0) {
                el.innerHTML = `<div class="empty-state">${t('dashboard.journal_empty')}</div>`;
                return;
            }
            el.innerHTML = renderCollapsibleList(entries, e => {
                const icon = JOURNAL_ICONS[e.entry_type] || '📔';
                const date = e.created_at ? new Date(e.created_at).toLocaleString(LANG, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : '';
                // tags is a JSON array from the backend; support both array and legacy comma-string
                const rawTags = Array.isArray(e.tags) ? e.tags : (typeof e.tags === 'string' ? e.tags.split(',') : []);
                const tags = rawTags.filter(Boolean).map(tag => `<span class="je-tag">${escapeHtml(tag.trim())}</span>`).join('');
                const auto = e.auto_generated ? ' 🤖' : '';
                const imp = e.importance || 2;
                return `<div class="journal-entry" data-importance="${imp}">
                    <div class="je-icon">${icon}</div>
                    <div class="je-body">
                        <div class="je-title">${escapeHtml(e.title || '')}${auto}</div>
                        <div class="je-meta">${date} · ${e.entry_type || 'reflection'}</div>
                        ${tags ? `<div class="je-tags">${tags}</div>` : ''}
                    </div>
                </div>`;
            }, 8);
        }

        function renderJournalSummary(summaries) {
            const el = document.getElementById('journal-summary');
            if (!el) return;
            if (!summaries || summaries.length === 0) {
                el.innerHTML = '';
                return;
            }
            const latest = summaries[0];
            el.innerHTML = `<div class="journal-summary-label">📋 ${latest.date}</div><div>${escapeHtml(latest.summary || '')}</div>`;

            // Sentiment trend + key topics
            const sentimentWrap = document.getElementById('journal-sentiment-wrap');
            const sentimentRow = document.getElementById('journal-sentiment-row');
            const topicsWrap = document.getElementById('journal-key-topics-wrap');
            if (sentimentWrap && sentimentRow) {
                const sentimentEmoji = { positive: '😊', neutral: '😐', frustrated: '😤' };
                const withSentiment = summaries.filter(s => s.sentiment);
                if (withSentiment.length > 0) {
                    dashSetHidden(sentimentWrap, false);
                    sentimentRow.innerHTML = withSentiment.map(s =>
                        `<span class="sentiment-day-badge ${esc(s.sentiment)}">${sentimentEmoji[s.sentiment] || '•'} ${esc(s.date)}</span>`
                    ).join('');
                } else {
                    dashSetHidden(sentimentWrap, true);
                }
                // Key topics from latest summary
                if (topicsWrap) {
                    const topics = latest.key_topics && latest.key_topics.length > 0 ? latest.key_topics : [];
                    if (topics.length > 0) {
                        topicsWrap.innerHTML = `<div class="journal-key-topics-label">${t('dashboard.journal_key_topics')}:</div><div class="journal-key-topics">${topics.map(tp => `<span class="journal-topic-chip">${esc(tp)}</span>`).join('')}</div>`;
                    } else {
                        topicsWrap.innerHTML = '';
                    }
                }
            }
        }

        function renderActivityOverview(data) {
            const summaryEl = document.getElementById('activity-overview-summary');
            const highlightsEl = document.getElementById('activity-overview-highlights');
            const pendingEl = document.getElementById('activity-overview-pending');
            const daysEl = document.getElementById('activity-overview-days');
            if (!summaryEl || !highlightsEl || !pendingEl || !daysEl) return;
            if (!data || (!data.overview_summary && !(data.days || []).length)) {
                summaryEl.innerHTML = '';
                highlightsEl.innerHTML = '';
                pendingEl.innerHTML = '';
                daysEl.innerHTML = `<div class="empty-state">${t('dashboard.activity_overview_empty')}</div>`;
                return;
            }

            summaryEl.innerHTML = data.overview_summary ? `<div class="journal-summary-label">🧭 ${t('dashboard.activity_overview_summary')}</div><div>${escapeHtml(data.overview_summary)}</div>` : '';

            const highlights = Array.isArray(data.highlights) ? data.highlights.slice(0, 3) : [];
            highlightsEl.innerHTML = highlights.length
                ? `<div class="journal-summary-label">✨ ${t('dashboard.activity_overview_highlights')}</div><div>${highlights.map(item => `<span class="journal-topic-chip">${escapeHtml(item)}</span>`).join('')}</div>`
                : '';

            const pending = Array.isArray(data.pending_items) ? data.pending_items.slice(0, 5) : [];
            pendingEl.innerHTML = pending.length
                ? `<div class="journal-summary-label">📌 ${t('dashboard.activity_overview_pending')}</div><div>${pending.map(item => `<div class="journal-entry"><div class="je-body"><div class="je-title">${escapeHtml(item)}</div></div></div>`).join('')}</div>`
                : '';

            const days = Array.isArray(data.days) ? data.days.slice(0, 3) : [];
            if (!days.length) {
                daysEl.innerHTML = `<div class="empty-state">${t('dashboard.activity_overview_empty')}</div>`;
                return;
            }
            daysEl.innerHTML = days.map(day => `
                <div class="journal-entry" data-importance="2">
                    <div class="je-icon">🗓️</div>
                    <div class="je-body">
                        <div class="je-title">${escapeHtml(day.date || '')}</div>
                        <div class="je-meta">${escapeHtml(day.summary || '')}</div>
                        ${(Array.isArray(day.highlights) && day.highlights.length) ? `<div class="je-tags">${day.highlights.slice(0, 2).map(item => `<span class="je-tag">${escapeHtml(item)}</span>`).join('')}</div>` : ''}
                    </div>
                </div>
            `).join('');
        }

        function renderErrorPatterns(data) {
            const wrap = document.getElementById('error-patterns-wrap');
            const list = document.getElementById('error-patterns-list');
            if (!wrap || !list) return;
            const frequent = data?.frequent || [];
            const recent = data?.recent || [];
            if (frequent.length === 0 && recent.length === 0) {
                dashSetHidden(wrap, true);
                return;
            }
            dashSetHidden(wrap, false);
            let html = '';
            const renderItem = (p) => {
                const resolved = p.resolution ? `<div class="error-pattern-resolution">✓ ${esc(p.resolution.substring(0, 80))}${p.resolution.length > 80 ? '…' : ''}</div>` : '';
                return `<div class="error-pattern-item">
                    <div class="error-pattern-header">
                        <span class="error-pattern-tool">${esc(p.tool_name || '?')}</span>
                        <span class="error-pattern-msg" title="${esc(p.error_message)}">${esc((p.error_message || '').substring(0, 60))}${(p.error_message || '').length > 60 ? '…' : ''}</span>
                        ${p.occurrence_count > 1 ? `<span class="error-pattern-count">${p.occurrence_count}×</span>` : ''}
                    </div>
                    ${resolved}
                </div>`;
            };
            // Deduplicate: show frequent, merge unique recents not already shown
            const shownIds = new Set();
            frequent.forEach(p => shownIds.add(p.id));
            if (frequent.length > 0) {
                html += `<div class="error-section-label">${t('dashboard.errors_frequent')}</div>`;
                html += renderCollapsibleList(frequent, renderItem, 5);
            }
            const newRecent = recent.filter(p => !shownIds.has(p.id));
            if (newRecent.length > 0) {
                html += `<div class="error-section-label">${t('dashboard.errors_recent')}</div>`;
                html += renderCollapsibleList(newRecent, renderItem, 5);
            }
            list.innerHTML = html;
        }

        async function loadJournal() {
            const [entries, summaries] = await Promise.all([
                API.get('/api/dashboard/journal?limit=15'),
                API.get('/api/dashboard/journal/summaries?days=7')
            ]);
            renderJournalTimeline(entries?.entries);
            renderJournalSummary(summaries?.summaries);
        }

        async function loadErrorPatterns() {
            const data = await API.get('/api/dashboard/errors');
            renderErrorPatterns(data);
        }

        // ── Escape HTML ─────────────────────────────────────────────────────────────
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
                const nodeCard = e.target.closest('[data-kg-node-id]');
                if (!nodeCard) return;
                const nodeID = nodeCard.getAttribute('data-kg-node-id');
                if (nodeID) {
                    loadKnowledgeGraphNodeDetail(nodeID);
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
        }

        // ── Auto-Refresh ────────────────────────────────────────────────────────────
        function startAutoRefresh() {
            // System metrics are now pushed via SSE (EventSystemMetrics) every 10s.
            // Mission updates are pushed via SSE (EventMissionUpdate) on state change.
            // No interval polling needed; SSE handles all live updates.
        }

        // ── Mood Time Range Buttons ─────────────────────────────────────────────────
        document.querySelector('.mood-mini-btns').addEventListener('click', async (e) => {
            const btn = e.target.closest('.mood-btn');
            if (!btn) return;
            document.querySelectorAll('.mood-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            currentMoodHours = parseInt(btn.dataset.hours) || 24;
            const [moodData, emotionData, personality] = await Promise.all([
                API.get('/api/dashboard/mood-history?hours=' + currentMoodHours),
                API.get('/api/dashboard/emotion-history?hours=' + currentMoodHours),
                API.get('/api/personality/state'),
            ]);
            if (Charts.mood) { Charts.mood.destroy(); }
            Charts.mood = createMoodLineChart('mood-chart', moodData || []);
            renderEmotionHistory(emotionData, personality);
        });

        // ── Profile Search ──────────────────────────────────────────────────────────
        document.getElementById('profile-search').addEventListener('input', function () {
            const q = this.value.toLowerCase();
            document.querySelectorAll('.profile-entry').forEach(el => {
                const match = !q || (el.dataset.search || '').toLowerCase().includes(q);
                dashSetHidden(el, !match);
            });
            // Show categories that have visible entries
            document.querySelectorAll('.profile-category').forEach(cat => {
                const entries = cat.querySelectorAll('.profile-entry');
                const hasVisible = Array.from(entries).some(e => !e.classList.contains('is-hidden'));
                dashSetHidden(cat, !hasVisible);
                // Auto-open when searching
                if (q && hasVisible) {
                    const entriesDiv = cat.querySelector('.profile-entries');
                    if (entriesDiv) entriesDiv.classList.remove('is-hidden');
                    const toggle = cat.querySelector('.profile-cat-toggle');
                    if (toggle) toggle.classList.add('open');
                }
            });
        });

        // ── Theme Update for Charts ─────────────────────────────────────────────────
        function updateChartColors() {
            setChartDefaults();
            // Destroy all charts
            for (const key of Object.keys(Charts)) {
                if (Charts[key] && Charts[key].destroy) {
                    Charts[key].destroy();
                    Charts[key] = null;
                }
            }
            // Reset loaded state so tab switch forces re-render
            Object.keys(TabState.loaded).forEach(k => { TabState.loaded[k] = false; });
            loadTabContent(TabState.active);
        }

        // ── SSE Live Updates ────────────────────────────────────────────────────────
        let _sseBanner = null;
        function showSSEBanner(msg) {
            if (!_sseBanner) {
                _sseBanner = document.createElement('div');
                _sseBanner.className = 'dash-sse-banner';
                _sseBanner.classList.add('is-hidden');
                document.body.prepend(_sseBanner);
            }
            _sseBanner.textContent = msg;
            dashSetHidden(_sseBanner, false);
        }
        function hideSSEBanner() {
            dashSetHidden(_sseBanner, true);
        }

        let _dashSSERegistered = false;
        let _sseReconnectTimer = null;
        let _sseReconnectTimeout = null;
        function connectSSE() {
            if (_dashSSERegistered) return;
            _dashSSERegistered = true;
            // Use the shared AuraSSE singleton — no dedicated EventSource needed.
            window.AuraSSE.on('_open', function () {
                // Clear any pending reconnect timers
                if (_sseReconnectTimer) { clearTimeout(_sseReconnectTimer); _sseReconnectTimer = null; }
                if (_sseReconnectTimeout) { clearTimeout(_sseReconnectTimeout); _sseReconnectTimeout = null; }
                hideSSEBanner();
            });
            window.AuraSSE.on('_error', function () {
                // Don't show banner immediately — wait 3 seconds of sustained disconnection
                if (_sseReconnectTimer) return; // Already pending
                _sseReconnectTimer = setTimeout(function () {
                    _sseReconnectTimer = null;
                    showSSEBanner('⚠ ' + (t('dashboard.sse_reconnecting') || 'Reconnecting…'));
                }, 3000);
                // Also check auth status after a longer delay
                if (_sseReconnectTimeout) clearTimeout(_sseReconnectTimeout);
                _sseReconnectTimeout = setTimeout(function () {
                    fetch('/api/auth/status', { credentials: 'same-origin' }).then(function (r) {
                        if (r.status === 401) {
                            window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
                        }
                    }).catch(function () { });
                }, 8000);
            });
            window.AuraSSE.on('system_metrics', function (sys) {
                updateGauge(Charts.cpu, 'cpu-val', (sys.cpu && sys.cpu.usage_percent) || 0);
                updateGauge(Charts.ram, 'ram-val', (sys.memory && sys.memory.used_percent) || 0);
                updateGauge(Charts.disk, 'disk-val', (sys.disk && sys.disk.used_percent) || 0);
                renderSystemStats(sys);
            });
            window.AuraSSE.on('memory_update', function (mem) {
                if (mem && Charts.memory) {
                    Charts.memory.data.datasets[0].data = [
                        mem.core_memory_facts || 0,
                        mem.chat_messages || 0,
                        mem.vectordb_entries || 0,
                        (mem.knowledge_graph || {}).nodes || 0,
                        (mem.knowledge_graph || {}).edges || 0,
                    ];
                    Charts.memory.update('none');
                    renderMemoryStats(mem);
                }
            });
            window.AuraSSE.on('personality_update', function (personality) {
                renderMoodBadge(personality);
                if (TabState.active === 'agent') {
                    API.get('/api/dashboard/emotion-history?hours=' + currentMoodHours).then(data => renderEmotionHistory(data, personality));
                }
            });
            window.AuraSSE.on('daemon_update', function () {
                if (TabState.active === 'system') {
                    loadDaemonsCard();
                }
            });
            window.AuraSSE.onLegacy(function (event) {
                try {
                    const data = JSON.parse(event.data);
                    if (data.event === 'budget_update') {
                        if (data.spent_usd != null) {
                            document.getElementById('budget-spent').textContent = '$' + (data.spent_usd || 0).toFixed(2);
                        }
                    } else if (data.event === 'budget_warning') {
                        if (typeof showToast === 'function') showToast('⚡ ' + (data.detail || t('dashboard.budget_warning')), 'warning', 5000);
                    } else if (data.event === 'budget_blocked') {
                        if (typeof showToast === 'function') showToast('🚫 ' + (data.detail || t('dashboard.budget_blocked')), 'error', 0);
                    }
                } catch (e) { /* ignore non-JSON */ }
            });
        }

        // ── Boot ────────────────────────────────────────────────────────────────────
        document.addEventListener('DOMContentLoaded', () => {
            initDashboard();
            connectSSE();
        });

        // Radial menu is initialized by shared.js (initRadialMenu)

        // ── Cron Edit Modal ──────────────────────────────────────────────────────────
        function closeCronEditModal() {
            document.getElementById('cronEditOverlay').classList.remove('open');
        }

        function openCronEditModal(btn) {
            document.getElementById('cronEditId').value     = btn.dataset.cronId    || '';
            document.getElementById('cronEditExpr').value   = btn.dataset.cronExpr  || '';
            document.getElementById('cronEditPrompt').value = btn.dataset.cronPrompt || '';
            document.getElementById('cronEditOverlay').classList.add('open');
            document.getElementById('cronEditExpr').focus();
        }

        async function saveCronEdit() {
            const id     = document.getElementById('cronEditId').value;
            const expr   = (document.getElementById('cronEditExpr').value || '').trim();
            const prompt = (document.getElementById('cronEditPrompt').value || '').trim();
            if (!id || !expr || !prompt) return;
            const saveBtn = document.getElementById('cronEditSaveBtn');
            if (saveBtn) saveBtn.disabled = true;
            try {
                const resp = await fetch('/api/cron', {
                    method: 'PUT',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id, cron_expr: expr, task_prompt: prompt })
                });
                if (!resp.ok) throw new Error('Update failed');
                closeCronEditModal();
                const activity = await API.get('/api/dashboard/activity');
                renderActivity(activity);
            } catch (e) {
                await showAlert('Error', '❌ ' + e.message);
            } finally {
                if (saveBtn) saveBtn.disabled = false;
            }
        }

        async function deleteCronJob(id) {
            if (!await showConfirm(t('dashboard.cron_delete_confirm', { id }))) return;
            try {
                const resp = await fetch('/api/cron?id=' + encodeURIComponent(id), {
                    method: 'DELETE',
                    credentials: 'same-origin'
                });
                if (!resp.ok) throw new Error('Delete failed');
                const activity = await API.get('/api/dashboard/activity');
                renderActivity(activity);
            } catch (e) {
                await showAlert('Error', '❌ ' + e.message);
            }
        }

        // Attach close handlers for cron edit modal
        document.addEventListener('DOMContentLoaded', () => {
            const cronClose   = document.getElementById('cronEditClose');
            const cronOverlay = document.getElementById('cronEditOverlay');
            if (cronClose)   cronClose.addEventListener('click', closeCronEditModal);
            if (cronOverlay) cronOverlay.addEventListener('click', e => {
                if (e.target === cronOverlay) closeCronEditModal();
            });
            document.addEventListener('keydown', e => {
                if (e.key === 'Escape' && cronOverlay && cronOverlay.classList.contains('open'))
                    closeCronEditModal();
            });
        });

        // ── Core-Facts Modal ─────────────────────────────────────────────────────────
        function closeCoreFactsModal() {
            document.getElementById('cfOverlay').classList.remove('open');
        }

        async function openCoreFactsModal() {
            const overlay = document.getElementById('cfOverlay');
            const body = document.getElementById('cfBody');
            body.innerHTML = '<div class="empty-state">' + t('dashboard.core_facts_modal_loading') + '</div>';
            overlay.classList.add('open');

            try {
                const data = await fetch('/api/dashboard/core-memory', { credentials: 'same-origin' }).then(r => r.json());
                const facts = data.facts || [];

                let html = `<div class="cf-add-bar">
                    <input type="text" class="cf-add-input" id="cfAddInput" placeholder="${t('dashboard.core_facts_modal_add_placeholder')}" onkeydown="if(event.key==='Enter')cfAddFact()">
                    <button class="cf-add-btn" onclick="cfAddFact()">＋</button>
                </div>`;

                if (facts.length === 0) {
                    html += '<div class="empty-state">' + t('dashboard.core_facts_modal_empty') + '</div>';
                } else {
                    html += facts.map(f =>
                        `<div class="cf-fact" data-id="${f.id}">
                            <span class="cf-id">[${f.id}]</span>
                            <span class="cf-fact-text" id="cf-text-${f.id}">${esc(f.fact)}</span>
                            <span class="cf-fact-actions">
                                <button class="cf-fact-btn" onclick="cfEditFact(${f.id})" title="${t('dashboard.core_facts_modal_edit')}">✏️</button>
                                <button class="cf-fact-btn danger" onclick="cfDeleteFact(${f.id})" title="${t('dashboard.core_facts_modal_delete')}">🗑️</button>
                            </span>
                        </div>`
                    ).join('');
                }
                body.innerHTML = html;
            } catch (e) {
                body.innerHTML = '<div class="empty-state">' + t('dashboard.core_facts_modal_error_load') + '</div>';
            }
        }

        async function cfAddFact() {
            const input = document.getElementById('cfAddInput');
            const fact = (input.value || '').trim();
            if (!fact) return;
            input.disabled = true;
            try {
                const resp = await fetch('/api/dashboard/core-memory/mutate', {
                    method: 'POST',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ fact })
                });
                if (!resp.ok) throw new Error(t('dashboard.core_facts_modal_error_add'));
                await openCoreFactsModal(); // reload
            } catch (e) {
                await showAlert('Error', '❌ ' + e.message);
                input.disabled = false;
            }
        }

        function cfEditFact(id) {
            const textEl = document.getElementById('cf-text-' + id);
            if (!textEl) return;
            const currentText = textEl.textContent;
            const factDiv = textEl.closest('.cf-fact');
            const actionsEl = factDiv.querySelector('.cf-fact-actions');

            // Replace text with inline input
            textEl.outerHTML = `<input type="text" class="cf-add-input cf-edit-input" id="cf-edit-${id}" value="${esc(currentText)}" onkeydown="if(event.key==='Enter')cfSaveEdit(${id});if(event.key==='Escape')openCoreFactsModal();">`;
            actionsEl.innerHTML = `
                <button class="cf-fact-btn cf-fact-btn-success" onclick="cfSaveEdit(${id})" title="${t('dashboard.core_facts_modal_save')}">✅</button>
                <button class="cf-fact-btn" onclick="openCoreFactsModal()" title="${t('dashboard.core_facts_modal_cancel')}">❌</button>`;
            document.getElementById('cf-edit-' + id).focus();
        }

        async function cfSaveEdit(id) {
            const input = document.getElementById('cf-edit-' + id);
            if (!input) return;
            const fact = (input.value || '').trim();
            if (!fact) return;
            input.disabled = true;
            try {
                const resp = await fetch('/api/dashboard/core-memory/mutate', {
                    method: 'PUT',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id, fact })
                });
                if (!resp.ok) throw new Error(t('dashboard.core_facts_modal_error_save'));
                await openCoreFactsModal(); // reload
            } catch (e) {
                await showAlert('Error', '❌ ' + e.message);
                input.disabled = false;
            }
        }

        async function cfDeleteFact(id) {
            if (!await showConfirm(t('dashboard.core_facts_modal_confirm_delete', {id: id}))) return;
            try {
                const resp = await fetch('/api/dashboard/core-memory/mutate', {
                    method: 'DELETE',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id })
                });
                if (!resp.ok) throw new Error(t('dashboard.core_facts_modal_error_delete'));
                await openCoreFactsModal(); // reload
            } catch (e) {
                await showAlert('Error', '❌ ' + e.message);
            }
        }

        // Attach close handlers after DOM is ready
        document.addEventListener('DOMContentLoaded', () => {
            const cfCloseBtn = document.getElementById('cfClose');
            const cfOverlay = document.getElementById('cfOverlay');
            if (cfCloseBtn) cfCloseBtn.addEventListener('click', closeCoreFactsModal);
            if (cfOverlay) cfOverlay.addEventListener('click', (e) => {
                if (e.target === cfOverlay) closeCoreFactsModal();
            });
            document.addEventListener('keydown', (e) => {
                if (e.key === 'Escape' && cfOverlay.classList.contains('open')) closeCoreFactsModal();
            });
        });

        // ── Collapsible Cards ──────────────────────────────────────────────────────
        (function initCollapsible() {
            document.querySelectorAll('.collapse-toggle').forEach(toggle => {
                const cardId = toggle.dataset.card;
                if (!cardId) return;
                // Restore state from localStorage
                const stored = localStorage.getItem('dash-collapse-' + cardId);
                if (stored === 'true') {
                    toggle.classList.add('collapsed');
                    const body = toggle.closest('.dash-card').querySelector('.dash-card-body');
                    if (body) body.classList.add('collapsed');
                }
                toggle.addEventListener('click', (e) => {
                    e.stopPropagation();
                    toggle.classList.toggle('collapsed');
                    const body = toggle.closest('.dash-card').querySelector('.dash-card-body');
                    if (body) body.classList.toggle('collapsed');
                    localStorage.setItem('dash-collapse-' + cardId, toggle.classList.contains('collapsed'));
                });
            });
        })();

        // Auth check is handled by shared.js (checkAuth)
