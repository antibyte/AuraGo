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
                ]);
            }
        };

        let currentMoodHours = 24;

        // ══════════════════════════════════════════════════════════════════════════════
        // CHART CREATORS
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
            const labels = [t('dashboard.memory_chart_core_memory'), t('dashboard.memory_chart_messages'), embLabel, t('dashboard.memory_chart_graph_nodes'), t('dashboard.memory_chart_graph_edges')];
            const values = [
                data.core_memory_facts || 0,
                data.chat_messages || 0,
                data.vectordb_entries || 0,
                (data.knowledge_graph || {}).nodes || 0,
                (data.knowledge_graph || {}).edges || 0,
            ];
            const colors = [cv('--accent'), cv('--success'), '#8b5cf6', '#f59e0b', '#ec4899'];
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
            const catIcons = { tech: '💻', preferences: '⭐', interests: '🎯', context: '📋', communication: '💬' };
            const catNameMap = {
                tech: t('dashboard.profile_cat_tech'),
                preferences: t('dashboard.profile_cat_preferences'),
                interests: t('dashboard.profile_cat_interests'),
                context: t('dashboard.profile_cat_context'),
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
                    details += `<div class="activity-item">
                <span class="activity-item-name">${esc(job.id || 'unknown')}</span>
                <span class="activity-item-detail">${esc(job.cron_expr || '')} — ${esc(truncate(job.task_prompt || '', 60))}</span>
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
                    details += `<div class="activity-item">
                <span class="activity-item-name">${esc(truncate(ca.task || ca.id, 50))}</span>
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

            // KPI grid
            const kpis = document.getElementById('prompt-kpis');
            const optPct = data.avg_optimization_pct ? data.avg_optimization_pct.toFixed(1) : '0';
            const kpiItems = [
                { val: data.total_builds, lbl: t('dashboard.prompt_kpi_total_builds') },
                { val: data.avg_tokens.toLocaleString(), lbl: t('dashboard.prompt_kpi_avg_tokens') },
                { val: data.avg_raw_len.toLocaleString(), lbl: t('dashboard.prompt_kpi_avg_raw_chars') },
                { val: data.avg_optimized_len.toLocaleString(), lbl: t('dashboard.prompt_kpi_avg_opt_chars') },
                { val: optPct + '%', lbl: t('dashboard.prompt_kpi_avg_saving_pct') },
                { val: data.total_saved_chars.toLocaleString(), lbl: t('dashboard.prompt_kpi_chars_saved') },
                { val: data.budget_shed_count, lbl: t('dashboard.prompt_kpi_budget_sheds') },
                { val: data.avg_guides_count, lbl: t('dashboard.prompt_kpi_avg_guides') },
            ];
            kpis.innerHTML = kpiItems.map(k =>
                `<div class="prompt-kpi"><div class="prompt-kpi-val">${k.val}</div><div class="prompt-kpi-lbl">${k.lbl}</div></div>`
            ).join('');

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
            return new Chart(ctx, {
                type: 'bar',
                data: {
                    labels: labels,
                    datasets: [{
                        label: t('dashboard.prompt_chart_chars_saved'),
                        data: recent.map(r => r.saved_chars),
                        backgroundColor: cv('--success') + 'aa',
                        borderRadius: 3,
                    }]
                },
                options: {
                    plugins: { legend: { display: false } },
                    scales: {
                        x: { display: false },
                        y: { grid: { color: cv('--border-subtle') }, ticks: { color: cv('--text-secondary') } },
                    }
                }
            });
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
            card.style.display = '';

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
                paperless_ngx: '📄'
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
                paperless_ngx: t('dashboard.integration_paperless_ngx')
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
            if (card) card.style.display = '';
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
                </div>`;
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // INITIALIZATION
        // ══════════════════════════════════════════════════════════════════════════════

        // ── Journal Timeline ────────────────────────────────────────────────────────
        const JOURNAL_ICONS = {
            reflection: '💭', milestone: '🏆', preference: '⭐', task_completed: '✅',
            integration: '🔌', learning: '📚', error_recovery: '🔧', system_event: '⚙️'
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
                const tags = (e.tags || '').split(',').filter(Boolean).map(t => `<span class="je-tag">${escapeHtml(t.trim())}</span>`).join('');
                const auto = e.auto_generated ? ' 🤖' : '';
                return `<div class="journal-entry" data-importance="${e.importance || 3}">
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
        }

        async function loadJournal() {
            const [entries, summaries] = await Promise.all([
                API.get('/api/dashboard/journal?limit=15'),
                API.get('/api/dashboard/journal/summaries?days=1')
            ]);
            renderJournalTimeline(entries?.entries);
            renderJournalSummary(summaries?.summaries);
        }

        // ── Escape HTML ─────────────────────────────────────────────────────────────
        function escapeHtml(str) {
            const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
            return String(str).replace(/[&<>"']/g, c => map[c]);
        }

        async function initDashboard() {
            const [system, budget, personality, moodHistory, memData, profile, activity, promptStats, logResults, githubRepos, overview, credits] = await API.fetchAll(currentMoodHours);

            // Agent Status Banner
            renderAgentBanner(overview, overview?.context?.total_chars);

            // System Health
            if (system) {
                Charts.cpu = createGauge('cpu-chart', system.cpu?.usage_percent || 0);
                updateGauge(Charts.cpu, 'cpu-val', system.cpu?.usage_percent || 0);
                Charts.ram = createGauge('ram-chart', system.memory?.used_percent || 0);
                updateGauge(Charts.ram, 'ram-val', system.memory?.used_percent || 0);
                Charts.disk = createGauge('disk-chart', system.disk?.used_percent || 0);
                updateGauge(Charts.disk, 'disk-val', system.disk?.used_percent || 0);
                renderSystemStats(system);
            }

            // Budget
            if (budget) {
                renderBudget(budget, credits);
                if (budget.enabled) {
                    Charts.budget = createBudgetDoughnut('budget-chart', budget.spent_usd || 0, budget.daily_limit_usd || 0);
                    Charts.budgetModels = createBudgetModelsChart('budget-models-chart', budget.models || {});
                }
            }

            // Personality
            if (personality) {
                renderMoodBadge(personality);
                if (personality.enabled) {
                    Charts.traits = createRadarChart('traits-chart', personality.traits);
                }
            }

            // Mood Timeline (now inline in personality)
            Charts.mood = createMoodLineChart('mood-chart', moodHistory || []);

            // Memory
            if (memData) {
                renderMemoryStats(memData);
                Charts.memory = createMemoryBarChart('memory-chart', memData);
                renderMilestones(memData.milestones);
            }

            // Profile
            renderProfile(profile);

            // Journal
            loadJournal();

            // Operations & Integrations
            renderOperations(overview);
            renderIntegrations(overview);

            // LLM Guardian
            loadGuardianCard();

            // Activity
            renderActivity(activity);

            // Prompt Builder Analytics
            if (promptStats) {
                renderPromptStats(promptStats);
                Charts.promptSize = createPromptSizeChart('prompt-size-chart', promptStats.recent);
                Charts.promptTier = createPromptTierChart('prompt-tier-chart', promptStats.tier_distribution);
                Charts.promptSavings = createPromptSavingsChart('prompt-savings-chart', promptStats.recent);
            }

            // Log Viewer
            renderLogs(logResults);
            scrollLogsToBottom();

            // GitHub Repos
            renderGitHubRepos(githubRepos);

            // Log viewer event listeners
            document.getElementById('log-filter').addEventListener('input', applyLogFilter);
            document.getElementById('log-scroll-btn').addEventListener('click', scrollLogsToBottom);
            document.getElementById('log-refresh-btn').addEventListener('click', async () => {
                const data = await API.get('/api/dashboard/logs?lines=100');
                renderLogs(data);
                scrollLogsToBottom();
            });

            // Start auto-refresh
            startAutoRefresh();
        }

        // ── Auto-Refresh ────────────────────────────────────────────────────────────
        function startAutoRefresh() {
            // System every 10s
            setInterval(async () => {
                const sys = await API.get('/api/dashboard/system');
                if (sys) {
                    updateGauge(Charts.cpu, 'cpu-val', sys.cpu?.usage_percent || 0);
                    updateGauge(Charts.ram, 'ram-val', sys.memory?.used_percent || 0);
                    updateGauge(Charts.disk, 'disk-val', sys.disk?.used_percent || 0);
                    renderSystemStats(sys);
                }
            }, 10_000);

            // Everything else every 30s
            setInterval(async () => {
                const [, budget, personality, moodHistory, memData, profile, activity, promptStats, logResults, githubRepos, overview, credits] = await API.fetchAll(currentMoodHours);

                // Agent banner
                renderAgentBanner(overview, overview?.context?.total_chars);

                if (budget && budget.enabled && Charts.budget) {
                    const remaining = Math.max(0, (budget.daily_limit_usd || 0) - (budget.spent_usd || 0));
                    Charts.budget.data.datasets[0].data = [budget.spent_usd || 0, remaining];
                    Charts.budget.update('none');
                    renderBudget(budget, credits);
                    // Rebuild model chart
                    if (Charts.budgetModels) { Charts.budgetModels.destroy(); }
                    Charts.budgetModels = createBudgetModelsChart('budget-models-chart', budget.models || {});
                }

                if (personality && personality.enabled && Charts.traits) {
                    const traitOrder = ['curiosity', 'thoroughness', 'creativity', 'empathy', 'confidence', 'affinity', 'loneliness'];
                    Charts.traits.data.datasets[0].data = traitOrder.map(t => personality.traits?.[t] ?? 0.5);
                    Charts.traits.update('none');
                    renderMoodBadge(personality);
                }

                if (moodHistory && Charts.mood) {
                    Charts.mood.destroy();
                    Charts.mood = createMoodLineChart('mood-chart', moodHistory);
                }

                if (memData) {
                    renderMemoryStats(memData);
                    if (Charts.memory) { Charts.memory.destroy(); }
                    Charts.memory = createMemoryBarChart('memory-chart', memData);
                    renderMilestones(memData.milestones);
                }

                renderProfile(profile);
                renderActivity(activity);

                // Prompt Builder Analytics
                if (promptStats) {
                    renderPromptStats(promptStats);
                    if (Charts.promptSize) { Charts.promptSize.destroy(); }
                    Charts.promptSize = createPromptSizeChart('prompt-size-chart', promptStats.recent);
                    if (Charts.promptTier) { Charts.promptTier.destroy(); }
                    Charts.promptTier = createPromptTierChart('prompt-tier-chart', promptStats.tier_distribution);
                    if (Charts.promptSavings) { Charts.promptSavings.destroy(); }
                    Charts.promptSavings = createPromptSavingsChart('prompt-savings-chart', promptStats.recent);
                }

                // Logs auto-refresh
                renderLogs(logResults);

                // GitHub repos
                renderGitHubRepos(githubRepos);

                // Operations & Integrations
                renderOperations(overview);
                renderIntegrations(overview);

                // LLM Guardian
                loadGuardianCard();

                // Journal
                loadJournal();
            }, 30_000);
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
            // Destroy all charts and re-init
            for (const key of Object.keys(Charts)) {
                if (Charts[key] && Charts[key].destroy) {
                    Charts[key].destroy();
                    Charts[key] = null;
                }
            }
            initDashboard();
        }

        // ── SSE Live Updates ────────────────────────────────────────────────────────
        function connectSSE() {
            const es = new EventSource('/events', { withCredentials: true });
            es.onmessage = function (event) {
                try {
                    const data = JSON.parse(event.data);
                    if (data.event === 'budget_update') {
                        // Quick update budget display
                        if (data.spent_usd != null) {
                            document.getElementById('budget-spent').textContent = '$' + (data.spent_usd || 0).toFixed(2);
                        }
                    }
                } catch (e) { /* ignore non-JSON */ }
            };
            es.onerror = function () {
                es.close();
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
