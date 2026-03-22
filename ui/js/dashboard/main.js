// AuraGo – dashboard page logic
// Extracted from dashboard.html

        // ══════════════════════════════════════════════════════════════════════════════
        // AuraGo Dashboard – JavaScript
        // ══════════════════════════════════════════════════════════════════════════════

        const LANG = document.documentElement.lang || 'en';

        // Theme handled by shared.js; just need to apply initial theme and update charts
        (function initDashboardTheme() {
            const saved = localStorage.getItem('aurago-theme') || 'dark';
            document.documentElement.setAttribute('data-theme', saved);
        })();

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
        const API = {
            get: url => fetch(url, { credentials: 'same-origin' }).then(r => {
                if (r.status === 401) {
                    window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
                    return null;
                }
                return r.ok ? r.json() : null;
            }).catch(() => null),
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
        const VALID_TABS = ['overview', 'agent', 'user', 'system'];

        function showTab(tabId) {
            if (!VALID_TABS.includes(tabId)) tabId = 'overview';
            localStorage.setItem('aurago-dash-tab', tabId);
            history.replaceState(null, '', '#' + tabId);
            document.querySelectorAll('.dash-tab').forEach(btn => {
                btn.classList.toggle('active', btn.dataset.tab === tabId);
            });
            document.querySelectorAll('.dash-tab-panel').forEach(panel => {
                panel.style.display = panel.id === 'tab-' + tabId ? '' : 'none';
            });
            TabState.active = tabId;
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
                case 'system':   return loadTabSystem();
            }
        }

        async function loadTabOverview() {
            const [system, budget, overview, credits] = await Promise.all([
                API.get('/api/dashboard/system'),
                API.get('/api/budget'),
                API.get('/api/dashboard/overview'),
                API.get('/api/credits'),
            ]);
            renderAgentBanner(overview, overview?.context?.total_chars);
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
            const [personality, moodHistory, memData] = await Promise.all([
                API.get('/api/personality/state'),
                API.get('/api/dashboard/mood-history?hours=' + currentMoodHours),
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
            if (memData) {
                renderMemoryStats(memData);
                if (Charts.memory) { Charts.memory.destroy(); Charts.memory = null; }
                Charts.memory = createMemoryBarChart('memory-chart', memData);
                renderMilestones(memData.milestones);
            }
            loadErrorPatterns();
        }

        async function loadTabUser() {
            const profile = await API.get('/api/dashboard/profile');
            renderProfile(profile);
            loadJournal();
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
            renderLogs(logResults);
            scrollLogsToBottom();
            renderGitHubRepos(githubRepos);
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
            const labels = [t('dashboard.memory_chart_core_memory'), t('dashboard.memory_chart_messages'), embLabel, t('dashboard.memory_chart_graph_nodes'), t('dashboard.memory_chart_graph_edges'), t('dashboard.memory_journal'), t('dashboard.memory_notes'), t('dashboard.memory_error_patterns')];
            const values = [
                data.core_memory_facts || 0,
                data.chat_messages || 0,
                data.vectordb_entries || 0,
                (data.knowledge_graph || {}).nodes || 0,
                (data.knowledge_graph || {}).edges || 0,
                data.journal_entries || 0,
                data.notes_count || 0,
                data.error_patterns || 0,
            ];
            const colors = [cv('--accent'), cv('--success'), '#8b5cf6', '#f59e0b', '#ec4899', '#06b6d4', '#10b981', '#ef4444'];
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
                        y: { grid: { display: false }, ticks: { color: cv('--text-primary'), font: { size: 11 } } },
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
                document.getElementById('budget-content').style.display = 'none';
                document.getElementById('budget-disabled').style.display = 'block';
                return;
            }
            document.getElementById('budget-content').style.display = '';
            document.getElementById('budget-disabled').style.display = 'none';
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
                creditsRow.style.display = '';
                document.getElementById('credits-balance').textContent = '$' + (credits.balance || 0).toFixed(2);
            } else {
                creditsRow.style.display = 'none';
            }

            // Per-LLM average token consumption chart
            const llmAvgWrap = document.getElementById('budget-llm-avg-wrap');
            const models = data.models || {};
            const hasCallData = Object.values(models).some(m => (m.calls || 0) > 0);
            if (hasCallData && llmAvgWrap) {
                llmAvgWrap.style.display = '';
                if (Charts.llmAvg) { Charts.llmAvg.destroy(); Charts.llmAvg = null; }
                Charts.llmAvg = createLLMAvgChart('budget-llm-avg-chart', models);
            } else if (llmAvgWrap) {
                llmAvgWrap.style.display = 'none';
            }
        }

        function renderMoodBadge(data) {
            if (!data || !data.enabled) {
                document.getElementById('personality-content').style.display = 'none';
                document.getElementById('personality-disabled').style.display = 'block';
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
            if (emotionDisplay && emotionText) {
                if (data.current_emotion) {
                    emotionText.textContent = data.current_emotion;
                    emotionDisplay.style.display = '';
                } else {
                    emotionDisplay.style.display = 'none';
                }
            }
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
            ];
            container.innerHTML = stats.map(s =>
                `<div class="mem-stat${s.clickable ? ' clickable' : ''}"${s.clickable ? ' onclick="openCoreFactsModal()" title="' + t('dashboard.memory_show_core_facts') + '"' : ''}><div class="mem-stat-val">${s.val}</div><div class="mem-stat-lbl">${s.lbl}${s.clickable ? ' 🔍' : ''}</div></div>`
            ).join('');
        }

        function renderMilestones(milestones) {
            const container = document.getElementById('milestone-list');
            if (!milestones || milestones.length === 0) {
                container.innerHTML = '<div class="empty-state">' + t('dashboard.memory_no_milestones') + '</div>';
                return;
            }
            container.innerHTML = milestones.map(m => {
                const d = new Date(m.timestamp);
                const dateStr = d.toLocaleDateString([], { month: 'short', day: 'numeric' });
                return `<div class="milestone-item">
            <span class="milestone-icon">🏆</span>
            <span class="milestone-text">${esc(m.label)}${m.details ? ': ' + esc(m.details) : ''}</span>
            <span class="milestone-date">${dateStr}</span>
        </div>`;
            }).join('');
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
            <div class="profile-entries" style="display:none;">`;
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
                <span class="profile-edit-form" style="display:none">
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
            if (entries.style.display === 'none') {
                entries.style.display = 'block';
                toggle.classList.add('open');
            } else {
                entries.style.display = 'none';
                toggle.classList.remove('open');
            }
        }

        function deleteProfileEntry(btn) {
            const cat = btn.dataset.cat;
            const key = btn.dataset.key;
            if (!confirm(t('dashboard.profile_delete_confirm') + ' "' + key + '"?')) return;
            fetch('/api/dashboard/profile/entry?' + new URLSearchParams({ category: cat, key: key }), {
                method: 'DELETE', credentials: 'same-origin'
            }).then(r => { if (r.ok) loadTabUser(); }).catch(() => {});
        }

        function editProfileEntry(btn) {
            const entry = btn.closest('.profile-entry');
            entry.querySelector('.profile-val').style.display = 'none';
            entry.querySelector('.profile-actions').style.display = 'none';
            const editForm = entry.querySelector('.profile-edit-form');
            editForm.style.display = 'inline-flex';
            editForm.querySelector('.profile-edit-input').focus();
        }

        function cancelProfileEdit(btn) {
            const entry = btn.closest('.profile-entry');
            entry.querySelector('.profile-val').style.display = '';
            entry.querySelector('.profile-actions').style.display = '';
            entry.querySelector('.profile-edit-form').style.display = 'none';
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

            statsEl.innerHTML = [
                { icon: '⏰', val: cronCount, lbl: t('dashboard.activity_scheduled') },
                { icon: '🔄', val: procCount, lbl: t('dashboard.activity_processes') },
                { icon: '🔗', val: whCount, lbl: t('dashboard.activity_webhooks') },
                { icon: '🤖', val: coCount, lbl: t('dashboard.activity_coagents') },
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
                <div style="display:flex;align-items:center;gap:0.4rem;">
                    <span class="activity-item-detail">${safeExpr} — ${esc(truncate(job.task_prompt || '', 60))}</span>
                    <span style="display:flex;gap:0.2rem;flex-shrink:0;">
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
                <span class="activity-item-detail" style="margin-left:0.4rem;">${esc(p.uptime || '')}</span></span>
            </div>`;
                }
                details += '</div>';
            }

            // Co-Agents
            if (coCount > 0) {
                details += '<div class="activity-section"><div class="activity-section-title">🤖 ' + t('dashboard.activity_coagents') + '</div>';
                for (const ca of data.coagents) {
                    const stateMap = {
                        running: t('dashboard.activity_coagent_running'),
                        completed: t('dashboard.activity_coagent_completed'),
                        failed: t('dashboard.activity_coagent_failed')
                    };
                    const stateClass = ca.state === 'running' ? 'pill-running' :
                        ca.state === 'completed' ? 'pill-completed' :
                            ca.state === 'failed' ? 'pill-failed' : 'pill-idle';
                    const specIcons = { researcher: '\uD83D\uDD0D', coder: '\uD83D\uDCBB', designer: '\uD83C\uDFA8', security: '\uD83D\uDEE1\uFE0F', writer: '\u270D\uFE0F' };
                    const specBadge = ca.specialist && specIcons[ca.specialist] ? '<span title="' + esc(ca.specialist) + '" style="margin-right:0.3rem;">' + specIcons[ca.specialist] + '</span>' : '';
                    details += `<div class="activity-item">
                <span class="activity-item-name">${specBadge}${esc(truncate(ca.task || ca.id, 50))}</span>
                <span><span class="pill-status ${stateClass}">${esc(stateMap[ca.state] || ca.state)}</span>
                <span class="activity-item-detail" style="margin-left:0.4rem;">${esc(ca.runtime || '')}</span></span>
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
                document.getElementById('prompt-no-data').style.display = '';
                document.getElementById('prompt-content').style.display = 'none';
                return;
            }
            document.getElementById('prompt-no-data').style.display = 'none';
            document.getElementById('prompt-content').style.display = '';

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
                    { val: avgFormat,  sub: fmtPct + '%',    lbl: t('dashboard.prompt_kpi_format_savings'),  color: 'var(--success)' },
                    { val: avgShed,    sub: shedPct + '%',   lbl: t('dashboard.prompt_kpi_shed_savings'),    color: '#f59e0b' },
                    { val: avgFilter,  sub: filterPct + '%', lbl: t('dashboard.prompt_kpi_filter_savings'),  color: '#8b5cf6' },
                    { val: totFormat,  sub: null,            lbl: t('dashboard.prompt_kpi_format_savings') + ' total', color: null },
                    { val: totShed,    sub: null,            lbl: t('dashboard.prompt_kpi_shed_savings')   + ' total', color: null },
                    { val: totFilter,  sub: null,            lbl: t('dashboard.prompt_kpi_filter_savings') + ' total', color: null },
                ];
                savingsKpis.innerHTML = breakdownItems.map(k =>
                    `<div class="prompt-kpi">${k.color ? `<div class="prompt-kpi-dot" style="background:${k.color};"></div>` : ''}<div class="prompt-kpi-val">${k.val}${k.sub ? `<span class="prompt-kpi-sub"> (${k.sub})</span>` : ''}</div><div class="prompt-kpi-lbl">${k.lbl}</div></div>`
                ).join('');
            }

            // Shed section list
            const shedEl = document.getElementById('shed-list');
            const shedCounts = data.shed_section_counts || {};
            const shedKeys = Object.keys(shedCounts).sort((a, b) => shedCounts[b] - shedCounts[a]);
            if (shedKeys.length === 0) {
                shedEl.innerHTML = '<div class="empty-state" style="padding:0.5rem 0;">' + t('dashboard.prompt_no_sections_shed') + '</div>';
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
            const sectionColors = ['var(--accent)', '#8b5cf6', '#f59e0b', '#ec4899', 'var(--text-secondary)'];
            const legendEl = document.getElementById('prompt-section-legend');
            if (legendEl && Object.keys(avgSections).length > 0) {
                const total = sectionOrder.reduce((s, k) => s + (avgSections[k] || 0), 0);
                legendEl.innerHTML = sectionOrder
                    .filter(k => (avgSections[k] || 0) > 0)
                    .map((k, i) => {
                        const pct = total > 0 ? ((avgSections[k] / total) * 100).toFixed(1) : 0;
                        return `<div class="prompt-section-legend-item">
                            <span class="prompt-section-legend-dot" style="background:${sectionColors[sectionOrder.indexOf(k)]};"></span>
                            <span class="prompt-section-legend-label">${esc(sectionNameMap[k] || k)}</span>
                            <span class="prompt-section-legend-val">${(avgSections[k] || 0).toLocaleString()} <span style="opacity:.6;">(${pct}%)</span></span>
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
                if (card) card.style.display = 'none';
                return;
            }

            const scores = data.adaptive_scores || [];
            const totalTracked = scores.length;
            const maxTools = data.max_tools || 0;
            const activeCount = maxTools > 0 ? Math.min(totalTracked, maxTools) : totalTracked;
            const totalCalls = data.total_calls || 0;

            if (card) card.style.display = 'block';

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
                list.innerHTML = `<div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(260px,1fr));gap:0.4rem;">` +
                    scores.slice(0, 30).map((s, i) => {
                        const pct = maxScore > 0 ? Math.round((s.score / maxScore) * 100) : 0;
                        const isActive = maxTools <= 0 || i < maxTools;
                        const opacity = isActive ? '1' : '0.4';
                        return `<div style="display:flex;align-items:center;gap:0.5rem;opacity:${opacity};">
                            <span style="flex:0 0 120px;font-size:var(--text-xs);overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="${esc(s.tool)}">${esc(s.tool)}</span>
                            <div style="flex:1;height:6px;background:var(--border-subtle);border-radius:3px;overflow:hidden;">
                                <div style="height:100%;width:${pct}%;background:var(--accent);border-radius:3px;"></div>
                            </div>
                            <span style="flex:0 0 40px;font-size:var(--text-xs);text-align:right;color:var(--text-secondary);">${s.count}×</span>
                        </div>`;
                    }).join('') + `</div>`;
            }
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
                viewer.innerHTML = '<div class="log-line" style="color:var(--text-secondary);">' + t('dashboard.logs_no_match') + '</div>';
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
                card.style.display = 'none';
                return;
            }
            card.style.display = 'block';

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
            let countInfo = `<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:0.75rem;">
                <span style="font-size:var(--text-sm);color:var(--text-secondary);">
                    👤 <strong>${esc(owner)}</strong> — ${t('dashboard.github_repositories_count', {n: repos.length})}
                </span>
            </div>`;

            let html = countInfo + '<div class="gh-repo-list">';
            for (const r of repos) {
                const vis = r.private ? '<span class="gh-badge gh-badge-private">🔒 ' + t('dashboard.github_badge_private') + '</span>' : '<span class="gh-badge gh-badge-public">🌐 ' + t('dashboard.github_badge_public') + '</span>';
                const tracked = r.tracked ? '<span class="gh-badge gh-badge-tracked">📌 ' + t('dashboard.github_badge_tracked') + '</span>' : '';
                const lang = r.language ? `<span>💻 ${esc(r.language)}</span>` : '';
                const updated = r.updated_at ? `<span>🕐 ${new Date(r.updated_at).toLocaleDateString()}</span>` : '';
                const desc = r.description ? `<div class="gh-repo-desc">${esc(r.description)}</div>` : '';

                html += `<div class="gh-repo">
                    <a href="${esc(r.html_url)}" target="_blank" rel="noopener" class="gh-repo-name">
                        📦 ${esc(r.name)} ${vis} ${tracked}
                    </a>
                    ${desc}
                    <div class="gh-repo-meta">${lang} ${updated}</div>
                </div>`;
            }
            html += '</div>';
            container.innerHTML = html;
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
                ctxFill.style.width = ctxPct + '%';
                ctxFill.style.background = ctxPct > 80 ? 'var(--danger)' : ctxPct > 60 ? 'var(--warning)' : 'var(--accent)';
            }
            if (ctxPctEl) ctxPctEl.textContent = ctxPct + '%';

            // Integration count
            const intEl = document.getElementById('ab-integrations');
            if (intEl && overview.integrations) {
                const active = Object.values(overview.integrations).filter(v => v).length;
                const total = Object.keys(overview.integrations).length;
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

        function renderIntegrations(overview) {
            if (!overview || !overview.integrations) return;
            const grid = document.getElementById('integration-grid');
            const icons = {
                telegram: '📱', discord: '💬', email: '📧', home_assistant: '🏠',
                docker: '🐳', co_agents: '🤖', webhooks: '🔗', webdav: '☁️',
                koofr: '☁️', chromecast: '📺', proxmox: '🖥️', ollama: '🧠',
                rocketchat: '💬', tailscale: '🔒', ansible: '🔧', invasion: '🥚',
                github: '🐙', mqtt: '📡', budget: '💰', indexing: '📂',
                auth: '🔑', fallback_llm: '🔄', personality_v2: '🎭', user_profiling: '👤', tts: '🔊',
                piper_tts: '🗣️',
                paperless_ngx: '📄', cloudflare_tunnel: '☁️',
                n8n: '🔀', fritzbox: '📡', meshcentral: '🖥️', a2a: '🔗',
                adguard: '🛡️', s3: '🪣', mcp: '🔌', mcp_server: '🔌',
                memory_analysis: '🧠', llm_guardian: '🛡️', security_proxy: '🔐',
                sandbox: '📦', ai_gateway: '🌐', image_generation: '🎨',
                google_workspace: '📧', onedrive: '☁️', netlify: '🚀',
                homepage: '🏠', virustotal: '🦠', brave_search: '🔍',
                firewall: '🔥', remote_control: '🖥️', web_scraper: '🕷️'
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
                onedrive: t('dashboard.integration_onedrive'), netlify: t('dashboard.integration_netlify'),
                homepage: t('dashboard.integration_homepage'), virustotal: t('dashboard.integration_virustotal'),
                brave_search: t('dashboard.integration_brave_search'), firewall: t('dashboard.integration_firewall'),
                remote_control: t('dashboard.integration_remote_control'),
                web_scraper: t('dashboard.integration_web_scraper')
            };

            // Sort: active first
            const sorted = Object.entries(overview.integrations).sort((a, b) => (b[1] ? 1 : 0) - (a[1] ? 1 : 0));
            grid.innerHTML = sorted.map(([key, active]) =>
                `<span class="int-badge ${active ? 'active' : 'inactive'}">${icons[key] || '•'} ${names[key] || key}</span>`
            ).join('');
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
                if (card) card.style.display = 'none';
                return;
            }
            if (card) card.style.display = 'block';
            renderGuardianCard(data);
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
                ${m.last_check_time > 0 ? `<div style="font-size:var(--text-xs);color:var(--text-secondary);margin-top:0.35rem;">${t('dashboard.guardian_last_check')}: ${relativeTime(m.last_check_time)}</div>` : ''}`;
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
            el.innerHTML = entries.slice(0, 15).map(e => {
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
            }).join('');
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
                    sentimentWrap.style.display = '';
                    sentimentRow.innerHTML = withSentiment.map(s =>
                        `<span class="sentiment-day-badge ${esc(s.sentiment)}">${sentimentEmoji[s.sentiment] || '•'} ${esc(s.date)}</span>`
                    ).join('');
                } else {
                    sentimentWrap.style.display = 'none';
                }
                // Key topics from latest summary
                if (topicsWrap) {
                    const topics = latest.key_topics && latest.key_topics.length > 0 ? latest.key_topics : [];
                    if (topics.length > 0) {
                        topicsWrap.innerHTML = `<div style="font-size:var(--text-xs);color:var(--text-secondary);margin-bottom:0.25rem;">${t('dashboard.journal_key_topics')}:</div><div class="journal-key-topics">${topics.map(tp => `<span class="journal-topic-chip">${esc(tp)}</span>`).join('')}</div>`;
                    } else {
                        topicsWrap.innerHTML = '';
                    }
                }
            }
        }

        function renderErrorPatterns(data) {
            const wrap = document.getElementById('error-patterns-wrap');
            const list = document.getElementById('error-patterns-list');
            if (!wrap || !list) return;
            const frequent = data?.frequent || [];
            const recent = data?.recent || [];
            if (frequent.length === 0 && recent.length === 0) {
                wrap.style.display = 'none';
                return;
            }
            wrap.style.display = '';
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
            if (frequent.length > 0) {
                html += `<div class="error-section-label">${t('dashboard.errors_frequent')}</div>`;
                frequent.slice(0, 5).forEach(p => { html += renderItem(p); shownIds.add(p.id); });
            }
            const newRecent = recent.filter(p => !shownIds.has(p.id));
            if (newRecent.length > 0) {
                html += `<div class="error-section-label">${t('dashboard.errors_recent')}</div>`;
                newRecent.slice(0, 5).forEach(p => { html += renderItem(p); });
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

            // Determine initial tab from hash or localStorage
            const hashTab = window.location.hash.replace('#', '');
            const savedTab = localStorage.getItem('aurago-dash-tab') || 'overview';
            const initialTab = VALID_TABS.includes(hashTab) ? hashTab : (VALID_TABS.includes(savedTab) ? savedTab : 'overview');

            // Show initial tab – triggers lazy load
            showTab(initialTab);

            // Start auto-refresh
            startAutoRefresh();
        }

        // ── Auto-Refresh ────────────────────────────────────────────────────────────
        function startAutoRefresh() {
            // System metrics are pushed via SSE (EventSystemMetrics) every 10s.
            // Auto-refresh every 10s: always update banner, refresh only the active tab.
            setInterval(async () => {
                const ov = await API.get('/api/dashboard/overview');
                renderAgentBanner(ov, ov?.context?.total_chars);

                TabState.loaded[TabState.active] = false;
                await loadTabContent(TabState.active);
            }, 10_000);
        }

        // ── Mood Time Range Buttons ─────────────────────────────────────────────────
        document.querySelector('.mood-mini-btns').addEventListener('click', async (e) => {
            const btn = e.target.closest('.mood-btn');
            if (!btn) return;
            document.querySelectorAll('.mood-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            currentMoodHours = parseInt(btn.dataset.hours) || 24;
            const data = await API.get('/api/dashboard/mood-history?hours=' + currentMoodHours);
            if (Charts.mood) { Charts.mood.destroy(); }
            Charts.mood = createMoodLineChart('mood-chart', data || []);
        });

        // ── Profile Search ──────────────────────────────────────────────────────────
        document.getElementById('profile-search').addEventListener('input', function () {
            const q = this.value.toLowerCase();
            document.querySelectorAll('.profile-entry').forEach(el => {
                const match = !q || (el.dataset.search || '').toLowerCase().includes(q);
                el.style.display = match ? '' : 'none';
            });
            // Show categories that have visible entries
            document.querySelectorAll('.profile-category').forEach(cat => {
                const entries = cat.querySelectorAll('.profile-entry');
                const hasVisible = Array.from(entries).some(e => e.style.display !== 'none');
                cat.style.display = hasVisible ? '' : 'none';
                // Auto-open when searching
                if (q && hasVisible) {
                    const entriesDiv = cat.querySelector('.profile-entries');
                    if (entriesDiv) entriesDiv.style.display = 'block';
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
                _sseBanner.style.cssText = 'position:fixed;top:0;left:0;right:0;background:var(--warning,#f59e0b);color:#000;text-align:center;padding:6px 12px;font-size:13px;z-index:9999;';
                document.body.prepend(_sseBanner);
            }
            _sseBanner.textContent = msg;
            _sseBanner.style.display = 'block';
        }
        function hideSSEBanner() {
            if (_sseBanner) _sseBanner.style.display = 'none';
        }

        function connectSSE() {
            const es = new EventSource('/events', { withCredentials: true });
            es.onopen = function () {
                hideSSEBanner();
            };
            es.onmessage = function (event) {
                try {
                    const data = JSON.parse(event.data);
                    // Typed events: {type, payload}
                    if (data.type === 'system_metrics') {
                        const sys = data.payload;
                        updateGauge(Charts.cpu, 'cpu-val', sys.cpu?.usage_percent || 0);
                        updateGauge(Charts.ram, 'ram-val', sys.memory?.used_percent || 0);
                        updateGauge(Charts.disk, 'disk-val', sys.disk?.used_percent || 0);
                        renderSystemStats(sys);
                    } else if (data.type === 'memory_update') {
                        const mem = data.payload;
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
                    }
                    // Legacy events: {event, detail}
                    else if (data.event === 'budget_update') {
                        if (data.spent_usd != null) {
                            document.getElementById('budget-spent').textContent = '$' + (data.spent_usd || 0).toFixed(2);
                        }
                    } else if (data.event === 'budget_warning') {
                        if (typeof showToast === 'function') showToast('⚡ ' + (data.detail || t('dashboard.budget_warning')), 'warning', 5000);
                    } else if (data.event === 'budget_blocked') {
                        if (typeof showToast === 'function') showToast('🚫 ' + (data.detail || t('dashboard.budget_blocked')), 'error', 0);
                    }
                } catch (e) { /* ignore non-JSON */ }
            };
            es.onerror = function () {
                es.close();
                showSSEBanner('⚠ ' + (t('dashboard.sse_reconnecting') || 'Reconnecting…'));
                // Check if auth error (401) - redirect to login
                fetch('/api/auth/status', { credentials: 'same-origin' }).then(r => {
                    if (r.status === 401) {
                        window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
                    }
                }).catch(() => {});
                setTimeout(connectSSE, 5000);
            };
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
                alert('❌ ' + e.message);
            } finally {
                if (saveBtn) saveBtn.disabled = false;
            }
        }

        async function deleteCronJob(id) {
            if (!confirm(t('dashboard.cron_delete_confirm', { id }))) return;
            try {
                const resp = await fetch('/api/cron?id=' + encodeURIComponent(id), {
                    method: 'DELETE',
                    credentials: 'same-origin'
                });
                if (!resp.ok) throw new Error('Delete failed');
                const activity = await API.get('/api/dashboard/activity');
                renderActivity(activity);
            } catch (e) {
                alert('❌ ' + e.message);
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
                alert('❌ ' + e.message);
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
            textEl.outerHTML = `<input type="text" class="cf-add-input" id="cf-edit-${id}" value="${esc(currentText)}" style="flex:1;" onkeydown="if(event.key==='Enter')cfSaveEdit(${id});if(event.key==='Escape')openCoreFactsModal();">`;
            actionsEl.innerHTML = `
                <button class="cf-fact-btn" onclick="cfSaveEdit(${id})" title="${t('dashboard.core_facts_modal_save')}" style="color:var(--success);">✅</button>
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
                alert('❌ ' + e.message);
                input.disabled = false;
            }
        }

        async function cfDeleteFact(id) {
            if (!confirm(t('dashboard.core_facts_modal_confirm_delete', {id: id}))) return;
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
                alert('❌ ' + e.message);
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
