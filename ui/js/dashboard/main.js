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

        // One loader per section id (sidebar order). Loaders from scripts that run after main.js
        // (file_sync_status.js, dashboard-widgets.js) are wrapped so they resolve at call time and
        // never receive the options object.
        const SECTION_LOADERS = {
            'overview': loadSectionOverview,
            'personality': loadSectionPersonality,
            'memory': loadSectionMemory,
            'user': loadSectionUser,
            'knowledge': loadSectionKnowledge,
            'filesync': () => loadFileSyncStatus(),
            'problems': loadSectionProblems,
            'automations': loadSectionAutomations,
            'system': loadSectionSystem,
            'audit': () => loadTabAudit(),
            'logs': loadSectionLogs,
            'llm': loadSectionLlm,
            'prompts': loadSectionPrompts,
            'tools': loadSectionTools,
        };

        // Every show reloads its section; within SECTION_CACHE_TTL_MS the data comes from the
        // DashAPI cache. force (section refresh button, reload after an edit) bypasses the cache.
        async function loadSectionContent(id, { force = false } = {}) {
            if (!Object.prototype.hasOwnProperty.call(SECTION_LOADERS, id)) return;
            try {
                await SECTION_LOADERS[id]({ force });
                SectionState.loaded[id] = true;
                const updated = document.querySelector('[data-section-updated="' + id + '"]');
                if (updated) updated.textContent = t('dashboard.section.updated').replace('{{time}}', new Date().toLocaleTimeString());
            } catch (err) {
                console.warn('loadSectionContent failed', id, err);
            }
        }

        // Fresh reload after knowledge-graph edits: widgets-knowledge.js and the knowledge
        // card retry buttons call this name.
        function loadTabKnowledge() {
            return loadSectionContent('knowledge', { force: true });
        }

        async function loadSectionOverview({ force = false } = {}) {
            CardState.setLoading('ov-status');
            CardState.setLoading('ov-attention');
            // One summary answer (dash-summary.js, also behind the sidebar status block and badges) renders
            // every overview tile; only the system section still reads the old overview answer (card-operations).
            const result = await DashSummary.refresh({ force });
            const summary = DashSummary.latest();
            if (!summary) {
                const retry = () => loadSectionContent('overview', { force: true });
                CardState.setError('ov-status', retry, { status: result.status });
                CardState.setError('ov-attention', retry, { status: result.status });
                return;
            }
            DashOverview.render(summary);
        }

        async function loadSectionPersonality({ force = false } = {}) {
            CardState.setLoading('card-personality');
            const opts = sectionFetch(force);
            const [personalityR, moodHistoryR, emotionHistoryR] = await Promise.all([
                API.getWithStatus('/api/personality/state', opts),
                API.getWithStatus('/api/dashboard/mood-history?hours=' + currentMoodHours, opts),
                API.getWithStatus('/api/dashboard/emotion-history?hours=' + currentMoodHours, opts),
            ]);
            const personality = personalityR.ok ? personalityR.data : null;
            const moodHistory = moodHistoryR.ok ? moodHistoryR.data : null;
            const emotionHistory = emotionHistoryR.ok ? emotionHistoryR.data : null;

            personalityR.ok ? CardState.setLoaded('card-personality') : CardState.setError('card-personality', loadSectionPersonality, { status: personalityR.status });

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
        }

        async function loadSectionMemory({ force = false } = {}) {
            CardState.setLoading('card-memory');
            const memDataR = await API.getWithStatus('/api/dashboard/memory', sectionFetch(force));
            const memData = memDataR.ok ? memDataR.data : null;
            memDataR.ok ? CardState.setLoaded('card-memory') : CardState.setError('card-memory', loadSectionMemory, { status: memDataR.status });
            if (memData) {
                renderMemoryStats(memData);
                // renderMemoryHealth also fills card-memory-reflection (renderWeeklyReflection).
                renderMemoryHealth(memData);
                if (Charts.memory) { Charts.memory.destroy(); Charts.memory = null; }
                Charts.memory = createMemoryBarChart('memory-chart', memData);
                renderMilestones(memData.milestones);
            }
        }

        async function loadSectionUser({ force = false } = {}) {
            CardState.setLoading('card-profile');
            CardState.setLoading('card-activity-overview');
            CardState.setLoading('card-journal');
            CardState.setLoading('card-newspaper');

            const opts = sectionFetch(force);
            const [profileR, activityOverviewR, newspaperCapsR, newspaperEditionsR] = await Promise.all([
                API.getWithStatus('/api/dashboard/profile', opts),
                API.getWithStatus('/api/memory/activity-overview?days=7', opts),
                API.getWithStatus('/api/desktop/newspaper/capabilities', opts),
                API.getWithStatus('/api/desktop/newspaper/editions', opts),
            ]);
            const profile = profileR.ok ? profileR.data : null;
            const activityOverview = activityOverviewR.ok ? activityOverviewR.data : null;

            profileR.ok ? CardState.setLoaded('card-profile') : CardState.setError('card-profile', loadSectionUser, { status: profileR.status });
            activityOverviewR.ok ? CardState.setLoaded('card-activity-overview') : CardState.setError('card-activity-overview', loadSectionUser, { status: activityOverviewR.status });

            if (profile) renderProfile(profile);
            if (activityOverview) renderActivityOverview(activityOverview);
            const newsStatus = document.getElementById('newspaper-dashboard-status');
            const newsNext = document.getElementById('newspaper-dashboard-next');
            if (newsStatus && newsNext) {
                const caps = newspaperCapsR.data, issues = newspaperEditionsR.data?.editions || [];
                newsStatus.textContent = !newspaperCapsR.ok ? t('dashboard.newspaper_unavailable') : !caps.enabled ? t('dashboard.newspaper_disabled') : issues.length ? t('dashboard.newspaper_latest') + ': ' + issues[0].lead : t('dashboard.newspaper_empty');
                newsNext.textContent = caps?.daily && caps.next_run ? t('dashboard.newspaper_next') + ': ' + new Date(caps.next_run).toLocaleString(LANG) : '';
                CardState.setLoaded('card-newspaper');
            }
            await loadJournal();
        }

        async function loadSectionKnowledge({ force = false } = {}) {
            const kgCards = [
                'card-knowledge-graph-summary',
                'card-knowledge-graph-health',
                'card-knowledge-graph-quality',
                'card-knowledge-graph-results',
                'card-knowledge-graph-visual',
            ];
            kgCards.forEach((id) => CardState.setLoading(id));

            const opts = sectionFetch(force);
            const results = await Promise.all([
                API.getWithStatus('/api/knowledge-graph/important?limit=30&min_score=15', opts),
                API.getWithStatus('/api/knowledge-graph/stats', opts),
                API.getWithStatus('/api/knowledge-graph/quality?limit=6', opts),
                API.getWithStatus('/api/knowledge-graph/health', opts),
            ]);
            const [importantR, statsR, qualityR, healthR] = results;
            const anyOk = results.some((r) => r.ok);
            if (!anyOk) {
                const status = importantR.status || statsR.status || 0;
                kgCards.forEach((id) => CardState.setError(id, loadTabKnowledge, { status }));
                return;
            }

            const firstKnowledgeStatus = (...responses) => {
                const failed = responses.find((r) => !r.ok);
                return failed ? failed.status : 0;
            };
            const markKnowledgeCard = (cardId, ok, status) => {
                ok ? CardState.setLoaded(cardId) : CardState.setError(cardId, loadTabKnowledge, { status });
            };
            markKnowledgeCard('card-knowledge-graph-summary', importantR.ok && statsR.ok, firstKnowledgeStatus(importantR, statsR));
            markKnowledgeCard('card-knowledge-graph-health', healthR.ok, healthR.status);
            markKnowledgeCard('card-knowledge-graph-quality', qualityR.ok, qualityR.status);
            markKnowledgeCard('card-knowledge-graph-results', importantR.ok, importantR.status);
            markKnowledgeCard('card-knowledge-graph-visual', importantR.ok, importantR.status);

            const important = importantR.ok ? importantR.data : null;
            const stats = statsR.ok ? statsR.data : null;
            const quality = qualityR.ok ? qualityR.data : null;
            const health = healthR.ok ? healthR.data : null;

            KnowledgeGraphState.importantNodes = Array.isArray(important) ? important : [];
            const importantIDs = new Set(KnowledgeGraphState.importantNodes.map(n => n.id));

            let allEdges = [];
            if (importantIDs.size > 0) {
                const edgeResp = await API.get('/api/knowledge-graph/edges?limit=200', opts);
                allEdges = Array.isArray(edgeResp) ? edgeResp.filter(e => e.relation !== 'co_mentioned_with') : [];
            }
            KnowledgeGraphState.importantEdges = allEdges;
            KnowledgeGraphState.stats = stats || null;

            const displayNodes = KnowledgeGraphState.showAll
                ? await loadAllNodes(opts)
                : KnowledgeGraphState.importantNodes;

            KnowledgeGraphState.nodes = displayNodes;
            KnowledgeGraphState.edges = filterEdgesForDisplay(allEdges, displayNodes);

            if (importantR.ok && statsR.ok) {
                renderKnowledgeGraphSummary(KnowledgeGraphState.importantNodes, KnowledgeGraphState.importantEdges, KnowledgeGraphState.stats);
            }
            if (healthR.ok) {
                renderKnowledgeGraphHealth(health);
            }
            if (qualityR.ok) {
                renderKnowledgeGraphQuality(quality);
            }
            if (importantR.ok) {
                renderKnowledgeGraphLists(KnowledgeGraphState.nodes, KnowledgeGraphState.edges);
                renderKnowledgeGraphVisual();
                renderKnowledgeGraphFilters();
            }

            if (importantR.ok) {
                if (KnowledgeGraphState.focusNodeId) {
                    await loadKnowledgeGraphNodeDetail(KnowledgeGraphState.focusNodeId);
                } else {
                    renderKnowledgeGraphDetailEmpty();
                }
            }

            const searchInput = document.getElementById('knowledge-search-input');
            const query = searchInput ? searchInput.value.trim() : '';
            if (importantR.ok && query) {
                await executeKnowledgeGraphSearch(query);
            } else if (importantR.ok) {
                renderKnowledgeGraphSearchState('');
            }
        }

        async function loadAllNodes(opts) {
            const nodes = await API.get('/api/knowledge-graph/nodes?limit=200', opts);
            return Array.isArray(nodes) ? nodes : [];
        }

        function filterEdgesForDisplay(edges, nodes) {
            if (!Array.isArray(edges) || !Array.isArray(nodes)) return [];
            const nodeIDs = new Set(nodes.map(n => n.id));
            return edges.filter(e => nodeIDs.has(e.source) && nodeIDs.has(e.target));
        }

        async function loadSectionProblems({ force = false } = {}) {
            CardState.setLoading('card-error-patterns');
            const issues = typeof window.loadOperationalIssues === 'function' ? window.loadOperationalIssues() : null;
            const errorsR = await API.getWithStatus('/api/dashboard/errors', sectionFetch(force));
            const card = document.getElementById('card-error-patterns');
            if (errorsR.ok) {
                CardState.setLoaded('card-error-patterns');
                renderErrorPatterns(errorsR.data);
                // renderErrorPatterns hides #error-patterns-wrap when nothing recurs; hide the card then.
                const wrap = document.getElementById('error-patterns-wrap');
                dashSetHidden(card, !!wrap && wrap.classList.contains('is-hidden'));
            } else {
                dashSetHidden(card, false);
                CardState.setError('card-error-patterns', loadSectionProblems, { status: errorsR.status });
            }
            await issues;
        }

        async function loadSectionAutomations() {
            CardState.setLoading('card-mission-history');
            await Promise.all([loadTabCronjobs(), loadMissionHistory(), loadDaemonsCard()]);
        }

        async function loadSectionSystem({ force = false } = {}) {
            CardState.setLoading('card-system');
            CardState.setLoading('card-operations');
            const opts = sectionFetch(force);
            const cards = Promise.all([
                loadGuardianCard(),
                typeof window.loadRealtimeSpeechStatus === 'function' ? window.loadRealtimeSpeechStatus() : null,
            ]);
            const [systemR, overviewR, githubReposR] = await Promise.all([
                API.getWithStatus('/api/dashboard/system', opts),
                // card-operations and the integration flags (the overview reads /api/dashboard/summary).
                API.getWithStatus('/api/dashboard/overview', opts),
                API.getWithStatus('/api/dashboard/github-repos', opts),
            ]);
            const system = systemR.ok ? systemR.data : null;
            const overview = overviewR.ok ? overviewR.data : null;
            const githubRepos = githubReposR.ok ? githubReposR.data : null;

            systemR.ok ? CardState.setLoaded('card-system') : CardState.setError('card-system', loadSectionSystem, { status: systemR.status });
            overviewR.ok ? CardState.setLoaded('card-operations') : CardState.setError('card-operations', loadSectionSystem, { status: overviewR.status });

            if (system) {
                if (!Charts.cpu) Charts.cpu = createGauge('cpu-chart', system.cpu?.usage_percent || 0);
                updateGauge(Charts.cpu, 'cpu-val', system.cpu?.usage_percent || 0);
                if (!Charts.ram) Charts.ram = createGauge('ram-chart', system.memory?.used_percent || 0);
                updateGauge(Charts.ram, 'ram-val', system.memory?.used_percent || 0);
                if (!Charts.disk) Charts.disk = createGauge('disk-chart', system.disk?.used_percent || 0);
                updateGauge(Charts.disk, 'disk-val', system.disk?.used_percent || 0);
                renderSystemStats(system);
            }
            if (overview) {
                renderOperations(overview);
                renderIntegrations(overview);
            }
            if (githubRepos) renderGitHubRepos(githubRepos);
            await cards;
        }

        async function loadSectionLogs({ force = false } = {}) {
            CardState.setLoading('card-logs');
            const logResultsR = await API.getWithStatus('/api/dashboard/logs?lines=100', sectionFetch(force));
            const logResults = logResultsR.ok ? logResultsR.data : null;
            logResultsR.ok ? CardState.setLoaded('card-logs') : CardState.setError('card-logs', loadSectionLogs, { status: logResultsR.status });
            if (logResults) {
                renderLogs(logResults);
                scrollLogsToBottom();
            }
        }

        async function loadSectionLlm({ force = false } = {}) {
            CardState.setLoading('card-budget');
            CardState.setLoading('card-helper-llm');
            const opts = sectionFetch(force);
            const cards = Promise.all([loadTaskRouterStats(), loadHelperLLMCard()]);
            const [budgetR, creditsR] = await Promise.all([
                API.getWithStatus('/api/budget', opts),
                API.getWithStatus('/api/credits', opts),
            ]);
            const budget = budgetR.ok ? budgetR.data : null;
            const credits = creditsR.ok ? creditsR.data : null;

            budgetR.ok ? CardState.setLoaded('card-budget') : CardState.setError('card-budget', loadSectionLlm, { status: budgetR.status });

            if (budget) {
                renderBudget(budget, credits);
                if (budget.enabled) {
                    if (Charts.budget) { Charts.budget.destroy(); Charts.budget = null; }
                    Charts.budget = createBudgetDoughnut('budget-chart', budget.spent_usd || 0, budget.daily_limit_usd || 0);
                    if (Charts.budgetModels) { Charts.budgetModels.destroy(); Charts.budgetModels = null; }
                    Charts.budgetModels = createBudgetModelsChart('budget-models-chart', budget.models || {});
                }
            }
            await cards;
        }

        async function loadSectionPrompts({ force = false } = {}) {
            CardState.setLoading('card-prompt');
            CardState.setLoading('card-optimization');
            CardState.setLoading('card-compression');
            const opts = sectionFetch(force);
            const [promptStatsR, optR, compR] = await Promise.all([
                API.getWithStatus('/api/dashboard/prompt-stats', opts),
                API.getWithStatus('/api/dashboard/optimization', opts),
                API.getWithStatus('/api/dashboard/compression', opts),
            ]);
            const promptStats = promptStatsR.ok ? promptStatsR.data : null;
            const opt = optR.ok ? optR.data : null;
            const comp = compR.ok ? compR.data : null;

            promptStatsR.ok ? CardState.setLoaded('card-prompt') : CardState.setError('card-prompt', loadSectionPrompts, { status: promptStatsR.status });
            optR.ok ? CardState.setLoaded('card-optimization') : CardState.setError('card-optimization', loadSectionPrompts, { status: optR.status });
            compR.ok ? CardState.setLoaded('card-compression') : CardState.setError('card-compression', loadSectionPrompts, { status: compR.status });

            if (promptStats) {
                renderPromptStats(promptStats);
                if (Charts.promptSize) Charts.promptSize.destroy();
                Charts.promptSize = createPromptSizeChart('prompt-size-chart', promptStats.recent);
                if (Charts.promptTier) Charts.promptTier.destroy();
                Charts.promptTier = createPromptTierChart('prompt-tier-chart', promptStats.tier_distribution);
                if (Charts.promptSavings) Charts.promptSavings.destroy();
                Charts.promptSavings = createPromptSavingsChart('prompt-savings-chart', promptStats.recent_fits);
                if (Charts.promptSectionDist) Charts.promptSectionDist.destroy();
                Charts.promptSectionDist = createPromptSectionDistChart('prompt-section-dist-chart', promptStats.avg_section_sizes);
            }
            if (opt) renderOptimizationStats(opt);
            if (comp) renderCompressionStats(comp);
        }

        async function loadSectionTools({ force = false } = {}) {
            CardState.setLoading('card-activity');
            const opts = sectionFetch(force);
            const [activityR, toolStatsR] = await Promise.all([
                API.getWithStatus('/api/dashboard/activity', opts),
                API.getWithStatus('/api/dashboard/tool-stats', opts),
            ]);
            const activity = activityR.ok ? activityR.data : null;
            const toolStats = toolStatsR.ok ? toolStatsR.data : null;

            activityR.ok ? CardState.setLoaded('card-activity') : CardState.setError('card-activity', loadSectionTools, { status: activityR.status });

            if (activity) renderActivity(activity);
            // One tool-stats answer feeds both tool cards.
            if (toolStats) {
                renderAdaptiveToolStats(toolStats);
                renderToolingTelemetry(toolStats);
            }
        }

        async function loadTaskRouterStats() {
            const container = document.getElementById('llm-router-stats');
            if (!container) return;
            const response = await API.getWithStatus('/api/llm-router/status');
            container.replaceChildren();
            const line = text => { const p = document.createElement('p'); p.textContent = text; container.appendChild(p); };
            if (!response.ok) { line(t('common.error')); return; }
            const data = response.data;
            const stats = data.stats || {};
            const label = key => t('common.llm_router.' + key);
            line(t(data.enabled ? 'common.enabled' : 'common.disabled') + ' · ' + (data.default_model || ''));
            line(label('since') + ' ' + new Date(stats.since).toLocaleString());
            line(label('helper_calls') + ': ' + Number(stats.helper_attempts || 0) + ' · ' + label('failures') + ': ' + Number(stats.helper_failures || 0));
            const tokens = Number((stats.helper_input_tokens || 0) + (stats.helper_output_tokens || 0));
            const usage = stats.helper_usage_missing ? (stats.helper_usage_reports ? '≥ ' + tokens : '?') : String(tokens);
            line(label('tokens') + ': ' + usage);
            const decision = stats.decision_api || {};
            const decisionTokens = Number(decision.input_tokens || 0) + Number(decision.output_tokens || 0);
            const decisionUsage = decision.usage_missing ? (decision.usage_reports ? '≥ ' + decisionTokens : '?') : String(decisionTokens);
            const decisionCost = decision.cost_missing ? (decision.cost_reports ? '≥ ' + Number(decision.cost_usd).toFixed(4) : '?') : (decision.cost_reports ? Number(decision.cost_usd).toFixed(4) : '?');
            line(label('source_decisions') + ': ' + Number(decision.attempts || 0) + ' · ' + label('failures') + ': ' + Number(decision.failures || 0) + ' · ' + label('decisions_uncertain') + ': ' + Number(decision.uncertain || 0));
            line(label('source_decisions') + ' · ' + label('tokens') + ': ' + decisionUsage + ' · USD: ' + decisionCost);
            line(label('fallback') + ': ' + Number(stats.fallbacks || 0));
            for (const area of ['general', 'easy', 'normal', 'complex', 'coding', 'research', 'creativity', 'security', 'writing']) {
                if (stats.decisions?.[area]) line(label('area_' + area) + ': ' + Number(stats.decisions[area]));
            }
            for (const source of ['rules', 'cache', 'helper', 'decisions', 'default']) {
                line(label('source_' + source) + ': ' + Number(stats.sources?.[source] || 0));
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
