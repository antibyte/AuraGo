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
