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
                badges += `<span class="budget-badge info">🛡️ ${esc(enfMap[data.enforcement] || String(data.enforcement))}</span>`;
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
                playful: t('dashboard.personality_mood_playful'),
                frustrated: t('dashboard.personality_mood_frustrated'),
                concerned: t('dashboard.personality_mood_concerned'),
                relaxed: t('dashboard.personality_mood_relaxed')
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
            const kg = data.knowledge_graph || {};
            const gn = kg.nodes || 0;
            const ge = kg.edges || 0;
            let graphVal = gn + ' / ' + ge;
            if ((kg.dirty_nodes || 0) > 0) {
                graphVal += ' · ' + t('dashboard.memory_graph_dirty_hint', { count: kg.dirty_nodes });
            }

            let embeddingVal = data.vectordb_entries || 0;
            let embeddingLbl = t('dashboard.memory_embeddings');
            if (data.vectordb_disabled) {
                embeddingLbl = t('dashboard.memory_embeddings_disabled');
            }

            const stats = [
                { val: data.core_memory_facts || 0, lbl: t('dashboard.memory_core_facts'), clickable: true },
                { val: data.chat_messages || 0, lbl: t('dashboard.memory_messages') },
                { val: embeddingVal, lbl: embeddingLbl },
                { val: graphVal, lbl: t('dashboard.memory_graph_label') },
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
            const latestReflection = data.latest_reflection || null;
            const reflectionActionables = Number(data.reflection_actionable_count || latestReflection?.actionable_count || 0);
            renderWeeklyReflection(latestReflection, reflectionActionables);

            const summaryEl = document.getElementById('memory-health-summary');
            if (summaryEl) {
                const modeKey = 'dashboard.memory_strategy_mode_' + String(strategy.mode || 'unavailable').toLowerCase();
                const translatedMode = t(modeKey);
                const modeLabel = translatedMode === modeKey ? String(strategy.mode || 'unavailable') : translatedMode;
                const reason = strategy.reason || t('dashboard.memory_strategy_reason_empty');
                const totalTracked = Number(confidence.total || 0);
                const confirmedCount = Number(confidence.confirmed || 0);
                const archivedCount = Number(confidence.archived || 0);
                const confirmedPct = totalTracked > 0 ? Math.round((confirmedCount / totalTracked) * 100) + '%' : '0%';
                const items = [
                    { value: Number(usage.retrieved_events || 0).toLocaleString(), label: t('dashboard.memory_health_retrieved') },
                    { value: Number(usage.predicted_events || 0).toLocaleString(), label: t('dashboard.memory_health_predicted') },
                    { value: Number(usage.distinct_memories || 0).toLocaleString(), label: t('dashboard.memory_health_distinct') },
                    { value: totalTracked.toLocaleString(), label: t('dashboard.memory_health_total') },
                    { value: confirmedPct, label: t('dashboard.memory_health_confirmed_rate') },
                    { value: Number(confidence.unverified || 0).toLocaleString(), label: t('dashboard.memory_health_unverified') },
                    { value: archivedCount.toLocaleString(), label: t('dashboard.memory_curator_archived') },
                    { value: Number(curator.stale_candidates || 0).toLocaleString(), label: t('dashboard.memory_health_stale') },
                    { value: Number(episodic.recent_count || 0).toLocaleString(), label: t('dashboard.memory_health_recent_episodes') },
                    { value: Number(pendingActions.length || 0).toLocaleString(), label: t('dashboard.memory_pending_title') },
                    { value: Number(conflicts.length || 0).toLocaleString(), label: t('dashboard.memory_conflicts_title') },
                    { value: Number(data.pending_memory_writes || 0).toLocaleString(), label: t('dashboard.memory_pending_writes') },
                    { value: reflectionActionables.toLocaleString(), label: t('dashboard.memory_reflection_actionables') },
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
                    t('dashboard.memory_curator_fact_archived', { count: Number(confidence.archived || 0) }),
                ];
                curatorEl.innerHTML = '<div class="memory-curator-actionbar">' +
                    '<button type="button" class="mh-more-btn" onclick="runMemoryCurationDryRun()">' + esc(t('dashboard.memory_curator_preview')) + '</button>' +
                    '<button type="button" class="mh-more-btn memory-curator-apply-btn" onclick="applyMemoryCurationSafeActions()">' + esc(t('dashboard.memory_curator_apply')) + '</button>' +
                    '<button type="button" class="mh-more-btn" onclick="runMemoryCurationDryRun(true)">' + esc(t('dashboard.memory_curator_show_archived')) + '</button>' +
                    '</div><div id="memory-curator-preview" class="memory-curator-preview"></div>' +
                    '<div id="memory-hygiene-panel" class="memory-hygiene-panel">' +
                    '<div class="memory-subsection-title">' + esc(t('dashboard.memory_hygiene_title')) + '</div>' +
                    '<div class="memory-curator-actionbar">' +
                    '<button type="button" class="mh-more-btn" onclick="runMemoryHygieneDryRun()">' + esc(t('dashboard.memory_hygiene_preview')) + '</button>' +
                    '<button type="button" class="mh-more-btn memory-curator-apply-btn" onclick="applyMemoryHygieneSafeActions()">' + esc(t('dashboard.memory_hygiene_apply')) + '</button>' +
                    '</div><div id="memory-hygiene-preview" class="memory-curator-preview"></div>' +
                    '</div>' +
                    '<div class="memory-curator-grid">' +
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

        function renderWeeklyReflection(latestReflection, reflectionActionables) {
            const cardEl = document.querySelector('.memory-reflection-card');
            const summaryEl = document.getElementById('memory-reflection-summary');
            const dateEl = document.getElementById('memory-reflection-date');
            const actionablesEl = document.getElementById('memory-reflection-actionables');
            const reflection = latestReflection && typeof latestReflection === 'object' ? latestReflection : null;
            const summary = reflection?.summary || t('dashboard.memory_latest_reflection_empty');
            const date = reflection?.date || formatMemoryReflectionDate(reflection?.created_at) || t('dashboard.memory_reflection_date_empty');
            const actionables = Number(reflectionActionables || reflection?.actionable_count || 0);

            if (summaryEl) {
                summaryEl.textContent = summary;
                summaryEl.classList.toggle('is-empty', !reflection?.summary);
            }
            if (cardEl) {
                cardEl.classList.toggle('has-reflection', Boolean(reflection?.summary));
            }
            if (dateEl) {
                dateEl.textContent = date;
            }
            if (actionablesEl) {
                actionablesEl.textContent = t('dashboard.memory_reflection_actionables') + ': ' + actionables.toLocaleString();
            }
        }

        function formatMemoryReflectionDate(raw) {
            if (!raw) return '';
            const date = new Date(raw);
            if (Number.isNaN(date.getTime())) return String(raw);
            return date.toLocaleString(document.documentElement.lang || LANG, { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
        }

        function setWeeklyReflectionRunState(isRunning) {
            const btn = document.getElementById('memory-reflection-run');
            if (!btn) return;
            btn.disabled = Boolean(isRunning);
            btn.classList.toggle('is-busy', Boolean(isRunning));
            if (isRunning) {
                btn.setAttribute('aria-busy', 'true');
            } else {
                btn.removeAttribute('aria-busy');
            }
            const label = btn.querySelector('[data-i18n="dashboard.memory_reflection_run"]');
            if (label) {
                label.textContent = t(isRunning ? 'dashboard.memory_reflection_running' : 'dashboard.memory_reflection_run');
            }
        }

        async function runWeeklyReflectionNow() {
            const confirmed = typeof showConfirm === 'function'
                ? await showConfirm(t('dashboard.memory_reflection_run_confirm_title'), t('dashboard.memory_reflection_run_confirm'))
                : true;
            if (!confirmed) return;

            setWeeklyReflectionRunState(true);
            try {
                const resp = await fetch('/api/dashboard/memory/reflection/run', {
                    method: 'POST',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({})
                });
                let data = {};
                try {
                    data = await resp.json();
                } catch (e) {
                    data = {};
                }
                if (!resp.ok) throw new Error(data.error || data.message || 'memory reflection run failed');
                renderWeeklyReflection(data.latest_reflection || null, Number(data.reflection_actionable_count || data.latest_reflection?.actionable_count || 0));
                if (typeof showToast === 'function') showToast(t('dashboard.memory_reflection_run_success'), 'success', 3500);
                if (typeof loadTabAgent === 'function') await loadTabAgent();
            } catch (e) {
                if (typeof showToast === 'function') showToast(t('dashboard.memory_reflection_run_error'), 'error', 5000);
            } finally {
                setWeeklyReflectionRunState(false);
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
                    <button type="button" class="profile-btn-edit" data-cat="${esc(cat)}" data-key="${esc(e.key)}" onclick="editProfileEntry(this)" title="${t('dashboard.btn_edit')}">${t('dashboard.btn_edit')}</button>
                    <button type="button" class="profile-btn-delete" data-cat="${esc(cat)}" data-key="${esc(e.key)}" onclick="deleteProfileEntry(this)" title="${t('dashboard.btn_delete')}">${t('dashboard.btn_delete')}</button>
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
            try {
                const r = await fetch('/api/dashboard/profile/entry?' + new URLSearchParams({ category: cat, key: key }), {
                    method: 'DELETE', credentials: 'same-origin'
                });
                if (!r.ok) throw new Error('delete failed');
                loadTabUser();
            } catch (_) {
                if (typeof showToast === 'function') showToast(t('dashboard.profile_delete_error'), 'error', 4000);
            }
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

        async function saveProfileEntry(btn) {
            const entry = btn.closest('.profile-entry');
            const cat = entry.closest('.profile-category').dataset.cat;
            const key = entry.querySelector('.profile-key').textContent;
            const newVal = entry.querySelector('.profile-edit-input').value.trim();
            if (!newVal) return;
            try {
                const r = await fetch('/api/dashboard/profile/entry', {
                    method: 'PUT', credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ category: cat, key: key, value: newVal })
                });
                if (!r.ok) throw new Error('save failed');
                loadTabUser();
            } catch (_) {
                if (typeof showToast === 'function') showToast(t('dashboard.profile_save_error'), 'error', 4000);
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
                    const safeSource = esc(job.source || 'agent');
                    const safeDisabled = job.disabled ? 'true' : 'false';
                    details += `<div class="activity-item">
                <span class="activity-item-name">${safeId}</span>
                <div class="activity-item-row">
                    <span class="activity-item-detail">${safeExpr} — ${esc(truncate(job.task_prompt || '', 60))}</span>
                    <span class="activity-item-actions-inline">
                        <button class="cf-fact-btn"
                            data-cron-id="${safeId}"
                            data-cron-expr="${safeExpr}"
                            data-cron-prompt="${safePrompt}"
                            data-cron-source="${safeSource}"
                            data-cron-disabled="${safeDisabled}"
                            onclick="openCronEditModal(this)"
                            title="${t('dashboard.cron_edit_title')}">${t('dashboard.btn_edit')}</button>
                        <button class="cf-fact-btn danger"
                            data-cron-id="${safeId}"
                            onclick="deleteCronJob(this.dataset.cronId)"
                            title="${t('dashboard.cron_btn_delete')}">${t('dashboard.btn_delete')}</button>
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
                    const specBadge = ca.specialist && specIcons[ca.specialist] ? '<span class="ca-spec-icon" title="' + esc(ca.specialist) + '">' + specIcons[ca.specialist] + '</span>' : '';
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
            const maxTotalTools = data.max_total_tools || 0;
            const lastReport = data.last_tool_filter_report || {};
            const lastToolCount = lastReport.final_tool_count || 0;
            const originalSchemaTokens = Number(lastReport.original_schema_tokens || 0);
            const finalSchemaTokens = Number(lastReport.final_schema_tokens || 0);
            const schemaReduction = originalSchemaTokens > 0
                ? Math.max(0, Math.round(((originalSchemaTokens - finalSchemaTokens) / originalSchemaTokens) * 100))
                : 0;
            const largestSchemas = Array.isArray(lastReport.largest_schemas) ? lastReport.largest_schemas : [];
            const activeCount = maxTools > 0 ? Math.min(totalTracked, maxTools) : totalTracked;
            const totalCalls = data.total_calls || 0;

            dashSetHidden(card, false);

            const kpis = document.getElementById('adaptive-tools-kpis');
            if (kpis) {
                const kpiItems = [
                    { val: `${activeCount}/${totalTracked}`, lbl: t('dashboard.adaptive_tools_active') },
                    { val: maxTools || '∞', lbl: t('dashboard.adaptive_tools_adaptive_cap') },
                    { val: maxTotalTools || '∞', lbl: t('dashboard.adaptive_tools_total_cap') },
                    { val: lastToolCount || '-', lbl: t('dashboard.adaptive_tools_last_count') },
                    { val: finalSchemaTokens ? finalSchemaTokens.toLocaleString() : '-', lbl: t('dashboard.adaptive_tools_schema_tokens') },
                    { val: originalSchemaTokens ? schemaReduction + '%' : '-', lbl: t('dashboard.adaptive_tools_schema_reduction') },
                    { val: totalCalls.toLocaleString(), lbl: t('dashboard.adaptive_tools_total_calls') },
                ];
                kpis.innerHTML = kpiItems.map(k =>
                    `<div class="prompt-kpi"><div class="prompt-kpi-val">${k.val}</div><div class="prompt-kpi-lbl">${k.lbl}</div></div>`
                ).join('');
            }

            const list = document.getElementById('adaptive-tools-list');
            if (list) {
                const maxScore = scores[0]?.score || 1;
                const maxSchemaTokens = Math.max(...largestSchemas.map(s => Number(s.rough_tokens || 0)), 1);
                const schemaHtml = largestSchemas.length > 0
                    ? `<div class="cfg-group-title cfg-group-title-underlined">${esc(t('dashboard.adaptive_tools_largest_schemas'))}</div>
                        <div class="adaptive-tools-grid">` + largestSchemas.slice(0, 5).map(s => {
                            const tokens = Number(s.rough_tokens || 0);
                            const pct = Math.max(1, Math.round((tokens / maxSchemaTokens) * 100));
                            return `<div class="adaptive-tool-row">
                                <span class="adaptive-tool-name" title="${esc(s.name)}">${esc(s.name)}</span>
                                <div class="adaptive-tool-bar-bg">
                                    <div class="adaptive-tool-bar-fill w-pct-${pct}"></div>
                                </div>
                                <span class="adaptive-tool-count">${tokens.toLocaleString()}</span>
                            </div>`;
                        }).join('') + `</div>`
                    : '';
                const scoreHtml = scores.length > 0 ? `<div class="adaptive-tools-grid">` +
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
                    }).join('') + `</div>` : '';
                list.innerHTML = schemaHtml + scoreHtml;
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

        function toolingTelemetryClassifierLabel(key) {
            const labels = {
                classifier_empty_response_announcement_only: t('dashboard.tooling_telemetry_classifier_empty_response_announcement_only'),
                classifier_empty_response_no_output: t('dashboard.tooling_telemetry_classifier_empty_response_no_output'),
                classifier_format_error_raw_code: t('dashboard.tooling_telemetry_classifier_format_error_raw_code'),
                classifier_format_error_orphaned_bracket_tag: t('dashboard.tooling_telemetry_classifier_format_error_orphaned_bracket_tag'),
                classifier_format_error_bare_xml_in_native_mode: t('dashboard.tooling_telemetry_classifier_format_error_bare_xml_in_native_mode'),
                classifier_format_error_tool_in_fence: t('dashboard.tooling_telemetry_classifier_format_error_tool_in_fence'),
                classifier_schema_error_incomplete_tool_call: t('dashboard.tooling_telemetry_classifier_schema_error_incomplete_tool_call'),
                classifier_schema_error_xml_fallback_format: t('dashboard.tooling_telemetry_classifier_schema_error_xml_fallback_format'),
                classifier_schema_error_invalid_native_args: t('dashboard.tooling_telemetry_classifier_schema_error_invalid_native_args'),
                classifier_schema_error_missing_action_field: t('dashboard.tooling_telemetry_classifier_schema_error_missing_action_field'),
                classifier_schema_error_malformed_native_args: t('dashboard.tooling_telemetry_classifier_schema_error_malformed_native_args'),
                classifier_schema_error_unrecognized_json_structure: t('dashboard.tooling_telemetry_classifier_schema_error_unrecognized_json_structure'),
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
            if (labels[key]) return labels[key];
            if (String(key || '').startsWith('classifier_')) {
                return toolingTelemetryClassifierLabel(key);
            }
            return key;
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

            const labels = {
                rag_auto_attempt: t('dashboard.tooling_telemetry_retrieval_rag_auto_attempt'),
                rag_auto_hit: t('dashboard.tooling_telemetry_retrieval_rag_auto_hit'),
                rag_auto_miss: t('dashboard.tooling_telemetry_retrieval_rag_auto_miss'),
                rag_auto_filtered_out: t('dashboard.tooling_telemetry_retrieval_rag_auto_filtered_out'),
                rag_auto_error: t('dashboard.tooling_telemetry_retrieval_rag_auto_error'),
                rag_predictive_attempt: t('dashboard.tooling_telemetry_retrieval_rag_predictive_attempt'),
                rag_predictive_hit: t('dashboard.tooling_telemetry_retrieval_rag_predictive_hit'),
                rag_predictive_miss: t('dashboard.tooling_telemetry_retrieval_rag_predictive_miss'),
                rag_predictive_error: t('dashboard.tooling_telemetry_retrieval_rag_predictive_error'),
            };
            if (labels[rawKey]) return labels[rawKey];
            if (rawKey.startsWith('memory_prompt_share_value:')) {
                return t('dashboard.tooling_telemetry_retrieval_memory_prompt_share_value', { pct: rawKey.split(':')[1] });
            }
            if (rawKey.startsWith('rag_auto_source:')) {
                return t('dashboard.tooling_telemetry_retrieval_rag_auto_source', { source: rawKey.split(':')[1].replaceAll('_', ' ') });
            }
            if (rawKey.startsWith('rag_predictive_source:')) {
                return t('dashboard.tooling_telemetry_retrieval_rag_predictive_source', { source: rawKey.split(':')[1].replaceAll('_', ' ') });
            }
            if (rawKey.startsWith('rag_auto_latency:')) {
                return t('dashboard.tooling_telemetry_retrieval_rag_auto_latency', { range: rawKey.split(':')[1].replaceAll('_', '-') });
            }
            if (rawKey.startsWith('rag_predictive_latency:')) {
                return t('dashboard.tooling_telemetry_retrieval_rag_predictive_latency', { range: rawKey.split(':')[1].replaceAll('_', '-') });
            }
            if (rawKey.startsWith('memory_prompt_tokens:')) {
                return t('dashboard.tooling_telemetry_retrieval_memory_prompt_tokens', { range: rawKey.split(':')[1].replaceAll('_', '-') });
            }
            if (rawKey.startsWith('memory_prompt_share:')) {
                return t('dashboard.tooling_telemetry_retrieval_memory_prompt_share', { range: rawKey.split(':')[1].replaceAll('_', '-') });
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
            if (name === 'docker' || name === 'proxmox' || name === 'frigate' || name === 'tailscale' || name === 'ansible' || name === 'github' || name === 'mcp_call' || name.startsWith('sql_') || name === 'manage_sql_connections' || name.includes('meshcentral') || name.includes('remote_') || name === 'invasion_control' || name === 'home_assistant' || name === 'ollama' || name === 'adguard' || name.startsWith('mqtt_') || name === 's3_storage') return 'infra';
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
                ctxFill.className = 'context-gauge-fill w-pct-' + ctxPct + (ctxPct > 80 ? ' ctx-level-high' : ctxPct > 60 ? ' ctx-level-med' : ' ctx-level-ok');
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

            const maintEl = document.getElementById('ab-maintenance');
            if (maintEl) {
                const m = overview.maintenance || {};
                const statusKey = 'dashboard.maintenance_status_' + (m.last_status || 'never');
                const statusLabel = t(statusKey);
                const lastRun = m.last_run ? formatDashboardTimestamp(m.last_run) : t('dashboard.maintenance_status_never');
                const nextRun = m.next_run ? formatDashboardTimestamp(m.next_run) : '—';
                maintEl.innerHTML = '🛠 <span>' + esc(t('dashboard.maintenance_nightly_title')) + ': ' + esc(statusLabel) + ' · ' + esc(t('dashboard.maintenance_last_run', { time: lastRun })) + ' · ' + esc(t('dashboard.maintenance_next_run', { time: nextRun })) + '</span>';
            }
        }

        function formatDashboardTimestamp(value) {
            if (!value) return '—';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return String(value);
            return date.toLocaleString();
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // OPERATIONS & INTEGRATIONS
        // ══════════════════════════════════════════════════════════════════════════════

        // ═══ Cronjobs ═══════════════════════════════════════════════════════════════
        let cronjobsSearchTimer = null;

        async function loadTabCronjobs() {
            CardState.setLoading('card-cronjobs');
            const params = cronjobsQueryParams();
            try {
                const resp = await fetch('/api/dashboard/cronjobs?' + params.toString(), { credentials: 'same-origin' });
                if (!resp.ok) throw new Error('Cronjobs load failed');
                const data = await resp.json();
                renderCronjobs(data);
                CardState.setLoaded('card-cronjobs');
            } catch (e) {
                console.warn('Cronjobs load failed', e);
                CardState.setError('card-cronjobs', loadTabCronjobs, { status: 0 });
                if (typeof showToast === 'function') showToast(t('dashboard.cronjobs_error_load'), 'error', 5000);
            }
        }

        function renderMemoryCurationPreview(plan) {
            const el = document.getElementById('memory-curator-preview');
            if (!el) return;
            if (!plan) {
                el.innerHTML = '';
                return;
            }
            const actions = [
                { count: Number(plan.auto_confirm_count || 0), label: t('dashboard.memory_curator_auto_confirm') },
                { count: Number(plan.auto_archive_count || 0), label: t('dashboard.memory_curator_auto_archive') },
                { count: Number(plan.review_required_count || 0), label: t('dashboard.memory_curator_review_required') },
            ];
            const renderAction = action => `
                <div class="memory-curator-preview-card">
                    <div class="memory-curator-preview-value">${esc(String(action.count))}</div>
                    <div class="memory-curator-preview-label">${esc(action.label)}</div>
                </div>`;
            const sampleActions = []
                .concat(Array.isArray(plan.auto_confirm) ? plan.auto_confirm.slice(0, 3) : [])
                .concat(Array.isArray(plan.auto_archive) ? plan.auto_archive.slice(0, 3) : [])
                .concat(Array.isArray(plan.review_required) ? plan.review_required.slice(0, 3) : []);
            const samples = sampleActions.length ? `
                <div class="memory-curator-preview-samples">
                    ${sampleActions.slice(0, 6).map(item => `
                        <div class="memory-curator-preview-row">
                            <span class="mono">${esc(item.doc_id || '')}</span>
                            <span>${esc(item.reason || item.action || '')}</span>
                        </div>`).join('')}
                </div>` : `<div class="empty-state dash-empty-tight">${t('dashboard.memory_curator_empty')}</div>`;
            el.innerHTML = `<div class="memory-curator-preview-grid">${actions.map(renderAction).join('')}</div>${samples}`;
        }

        async function runMemoryCurationDryRun(showArchived) {
            try {
                if (showArchived) {
                    const archiveResp = await fetch('/api/dashboard/memory/curation', { credentials: 'same-origin' });
                    if (!archiveResp.ok) throw new Error('memory curation archive load failed');
                    const archiveData = await archiveResp.json();
                    renderMemoryCurationArchive(archiveData.archived || []);
                    return;
                }
                const resp = await fetch('/api/dashboard/memory/curation/dry-run', {
                    method: 'POST',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ limit: 100 })
                });
                if (!resp.ok) throw new Error('memory curation dry-run failed');
                const data = await resp.json();
                renderMemoryCurationPreview(data.plan);
                if (typeof showToast === 'function') showToast(t('dashboard.memory_curator_preview_ready'), 'success', 2500);
            } catch (e) {
                if (typeof showToast === 'function') showToast(t('dashboard.memory_curator_error'), 'error', 5000);
            }
        }

        function renderMemoryCurationArchive(items) {
            const el = document.getElementById('memory-curator-preview');
            if (!el) return;
            const archived = Array.isArray(items) ? items : [];
            if (!archived.length) {
                el.innerHTML = `<div class="empty-state dash-empty-tight">${t('dashboard.memory_curator_no_archived')}</div>`;
                return;
            }
            el.innerHTML = `<div class="memory-curator-preview-samples">` + archived.slice(0, 12).map(item => `
                <div class="memory-curator-preview-row">
                    <span class="mono">${esc(item.doc_id || '')}</span>
                    <span>${esc(item.archived_reason || item.review_note || item.verification_status || '')}</span>
                </div>
            `).join('') + `</div>`;
        }

        async function applyMemoryCurationSafeActions() {
            const confirmed = await showConfirm(
                t('dashboard.memory_curator_apply_title'),
                t('dashboard.memory_curator_apply_confirm')
            );
            if (!confirmed) return;
            try {
                const resp = await fetch('/api/dashboard/memory/curation/apply', {
                    method: 'POST',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ limit: 100, confirm: 'APPLY_MEMORY_CURATION' })
                });
                if (!resp.ok) throw new Error('memory curation apply failed');
                const data = await resp.json();
                if (typeof showToast === 'function') showToast(t('dashboard.memory_curator_applied', { count: Number(data.applied || 0) }), 'success', 3500);
                if (typeof loadTabAgent === 'function') await loadTabAgent();
            } catch (e) {
                if (typeof showToast === 'function') showToast(t('dashboard.memory_curator_error'), 'error', 5000);
            }
        }

        function renderMemoryHygienePreview(plan, failedActions) {
            const el = document.getElementById('memory-hygiene-preview');
            if (!el) return;
            if (!plan) {
                el.innerHTML = '';
                return;
            }
            const failures = Array.isArray(failedActions) ? failedActions : [];
            const totals = plan.totals || {};
            const kgPlan = plan.kg || {};
            const cards = [
                { count: Number(totals.journal_removed || 0), label: t('dashboard.memory_hygiene_journal_removed') },
                { count: Number(totals.notes_auto_archive || 0), label: t('dashboard.memory_hygiene_notes_archive') },
                { count: Number(totals.canonical_repairs || 0), label: t('dashboard.memory_hygiene_canonical') },
                { count: Number(totals.memory_auto_confirm || 0) + Number(totals.memory_auto_archive || 0), label: t('dashboard.memory_hygiene_vector') },
                { count: Number(totals.notes_review || 0) + Number(totals.memory_review || 0), label: t('dashboard.memory_hygiene_review') },
            ];
            if (kgPlan.available || Number(totals.kg_open_conflicts || 0) || Number(totals.kg_duplicates || 0) || Number(totals.kg_removed || 0)) {
                cards.push(
                    { count: Number(totals.kg_open_conflicts || 0), label: t('dashboard.memory_hygiene_kg_conflicts') },
                    { count: Number(totals.kg_duplicates || 0), label: t('dashboard.memory_hygiene_kg_duplicates') },
                    { count: Number(totals.kg_removed || 0), label: t('dashboard.memory_hygiene_kg_removed') },
                );
            }
            const samples = [];
            const journalItems = plan.journal && Array.isArray(plan.journal.items) ? plan.journal.items : [];
            journalItems.slice(0, 2).forEach(item => samples.push({
                id: item.title || item.entry_type || '',
                reason: item.reason || t('dashboard.memory_hygiene_journal_removed')
            }));
            const noteItems = plan.notes && Array.isArray(plan.notes.auto_archive) ? plan.notes.auto_archive : [];
            noteItems.slice(0, 2).forEach(item => samples.push({
                id: item.title || String(item.note_id || ''),
                reason: item.reason || t('dashboard.memory_hygiene_notes_archive')
            }));
            const canonicalItems = plan.canonical && Array.isArray(plan.canonical.items) ? plan.canonical.items : [];
            canonicalItems.slice(0, 2).forEach(item => samples.push({
                id: item.old_doc_id || '',
                reason: item.reason || t('dashboard.memory_hygiene_canonical')
            }));
            const kgSuggestions = kgPlan && Array.isArray(kgPlan.conflict_suggestions) ? kgPlan.conflict_suggestions : [];
            kgSuggestions.slice(0, 2).forEach(item => samples.push({
                id: item.winning_claim_id || String(item.conflict_id || ''),
                reason: t('dashboard.memory_hygiene_kg_suggestion')
            }));
            const renderCard = card => `
                <div class="memory-curator-preview-card">
                    <div class="memory-curator-preview-value">${esc(String(card.count))}</div>
                    <div class="memory-curator-preview-label">${esc(card.label)}</div>
                </div>`;
            const sampleHtml = samples.length ? `
                <div class="memory-curator-preview-samples">
                    ${samples.map(item => `
                        <div class="memory-curator-preview-row">
                            <span class="mono">${esc(item.id)}</span>
                            <span>${esc(item.reason)}</span>
                        </div>`).join('')}
                </div>` : `<div class="empty-state dash-empty-tight">${t('dashboard.memory_hygiene_empty')}</div>`;
            const failureHtml = failures.length ? `
                <div class="memory-curator-preview-samples memory-hygiene-failures">
                    <div class="memory-subsection-title">${esc(t('dashboard.memory_hygiene_partial_failures', { count: failures.length }))}</div>
                    ${failures.map(item => `
                        <div class="memory-curator-preview-row">
                            <span class="mono">${esc(t('dashboard.memory_hygiene_failed_action', { domain: item.domain || '', target: item.target || '', error: item.error || '' }))}</span>
                        </div>`).join('')}
                </div>` : '';
            el.innerHTML = `<div class="memory-curator-preview-grid">${cards.map(renderCard).join('')}</div>${sampleHtml}${failureHtml}`;
        }

        async function runMemoryHygieneDryRun() {
            try {
                const resp = await fetch('/api/dashboard/memory/hygiene/dry-run', {
                    method: 'POST',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ limit: 100, include_kg: true })
                });
                if (!resp.ok) throw new Error('memory hygiene dry-run failed');
                const data = await resp.json();
                renderMemoryHygienePreview(data.plan, data.failed_actions || []);
                if (typeof showToast === 'function') showToast(t('dashboard.memory_hygiene_preview_ready'), 'success', 2500);
            } catch (e) {
                if (typeof showToast === 'function') showToast(t('dashboard.memory_hygiene_error'), 'error', 5000);
            }
        }

        async function applyMemoryHygieneSafeActions() {
            const confirmed = await showConfirm(
                t('dashboard.memory_hygiene_apply_title'),
                t('dashboard.memory_hygiene_apply_confirm')
            );
            if (!confirmed) return;
            try {
                const resp = await fetch('/api/dashboard/memory/hygiene/apply', {
                    method: 'POST',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ limit: 100, confirm: 'APPLY_MEMORY_HYGIENE' })
                });
                if (!resp.ok) throw new Error('memory hygiene apply failed');
                const data = await resp.json();
                renderMemoryHygienePreview(data.plan, data.failed_actions || []);
                const applied = data.applied || {};
                const count = Number(applied.memory || 0) + Number(applied.journal || 0) + Number(applied.notes || 0) + Number(applied.canonical || 0) + Number(applied.kg || 0);
                const failed = Array.isArray(data.failed_actions) ? data.failed_actions.length : 0;
                if (failed > 0 && typeof showToast === 'function') {
                    showToast(t('dashboard.memory_hygiene_partial_failures', { count: failed }), 'warning', 5000);
                } else if (typeof showToast === 'function') {
                    showToast(t('dashboard.memory_hygiene_applied', { count }), 'success', 3500);
                }
                if (typeof loadTabAgent === 'function') await loadTabAgent();
            } catch (e) {
                if (typeof showToast === 'function') showToast(t('dashboard.memory_hygiene_error'), 'error', 5000);
            }
        }

        function setupCronjobsControls() {
            const search = document.getElementById('cronjobs-search');
            const refresh = document.getElementById('cronjobs-refresh');
            const filterIDs = ['cronjobs-source-filter', 'cronjobs-status-filter'];
            if (search) {
                search.addEventListener('input', () => {
                    clearTimeout(cronjobsSearchTimer);
                    cronjobsSearchTimer = setTimeout(loadTabCronjobs, 250);
                });
                search.addEventListener('keydown', (e) => {
                    if (e.key === 'Enter') {
                        e.preventDefault();
                        clearTimeout(cronjobsSearchTimer);
                        loadTabCronjobs();
                    }
                });
            }
            filterIDs.forEach(id => {
                const el = document.getElementById(id);
                if (el) el.addEventListener('change', loadTabCronjobs);
            });
            if (refresh) {
                refresh.addEventListener('click', async () => {
                    refresh.classList.add('is-busy');
                    refresh.disabled = true;
                    refresh.setAttribute('aria-busy', 'true');
                    try {
                        await loadTabCronjobs();
                    } finally {
                        refresh.classList.remove('is-busy');
                        refresh.disabled = false;
                        refresh.removeAttribute('aria-busy');
                    }
                });
            }
        }

        function cronjobsQueryParams() {
            const params = new URLSearchParams();
            const q = (document.getElementById('cronjobs-search')?.value || '').trim();
            const source = document.getElementById('cronjobs-source-filter')?.value || '';
            const status = document.getElementById('cronjobs-status-filter')?.value || '';
            if (q) params.set('q', q);
            if (source) params.set('source', source);
            if (status) params.set('status', status);
            return params;
        }

        function renderCronjobs(data) {
            const tbody = document.getElementById('cronjobs-tbody');
            const emptyEl = document.getElementById('cronjobs-empty');
            const summaryEl = document.getElementById('cronjobs-summary');
            if (!tbody) return;
            const jobs = Array.isArray(data?.jobs) ? data.jobs : [];
            if (summaryEl) {
                const items = [
                    { value: Number(data?.total || 0), label: t('dashboard.cronjobs_total') },
                    { value: Number(data?.enabled || 0), label: t('dashboard.cronjobs_enabled') },
                    { value: Number(data?.disabled || 0), label: t('dashboard.cronjobs_disabled') },
                    { value: Number(data?.errors || 0), label: t('dashboard.cronjobs_errors') },
                ];
                summaryEl.innerHTML = items.map(item => `
                    <div class="cronjobs-summary-item">
                        <span class="cronjobs-summary-value">${Number(item.value).toLocaleString()}</span>
                        <span>${esc(item.label)}</span>
                    </div>
                `).join('');
            }

            tbody.innerHTML = '';
            if (jobs.length === 0) {
                if (emptyEl) emptyEl.style.display = '';
                return;
            }
            if (emptyEl) emptyEl.style.display = 'none';

            jobs.forEach(job => {
                const disabled = !!job.disabled;
                const rawStatus = typeof job.status === 'string' ? job.status : '';
                const status = ['enabled', 'disabled', 'error'].includes(rawStatus) ? rawStatus : (disabled ? 'disabled' : 'enabled');
                const hasError = status === 'error';
                const tr = document.createElement('tr');
                const rowClasses = ['cronjobs-row'];
                if (disabled) rowClasses.push('cronjobs-row-disabled');
                if (hasError) rowClasses.push('cronjobs-row-error');
                tr.className = rowClasses.join(' ');
                const nextRun = cronjobFormatNextRun(job.next_run, disabled, hasError);
                const lastError = job.last_error || '';
                const promptTitle = lastError ? `${lastError}\n\n${job.task_prompt || ''}` : job.task_prompt || '';
                const statusTitle = lastError || cronjobStatusLabel(status);
                tr.innerHTML = `
                    <td data-label="${esc(t('dashboard.cronjobs_col_id'))}"><span class="cronjobs-id">${esc(job.id || '—')}</span></td>
                    <td data-label="${esc(t('dashboard.cronjobs_col_source'))}"><span class="cronjobs-source cronjobs-source-${esc(job.source || 'agent')}">${esc(cronjobSourceLabel(job.source))}</span></td>
                    <td data-label="${esc(t('dashboard.cronjobs_col_schedule'))}"><code class="cronjobs-expr">${esc(job.cron_expr || '—')}</code></td>
                    <td data-label="${esc(t('dashboard.cronjobs_col_next_run'))}">${esc(nextRun)}</td>
                    <td data-label="${esc(t('dashboard.cronjobs_col_status'))}"><span class="cronjobs-status cronjobs-status-${status}" title="${esc(statusTitle)}">${esc(cronjobStatusLabel(status))}</span></td>
                    <td data-label="${esc(t('dashboard.cronjobs_col_prompt'))}" title="${esc(promptTitle)}">${esc(truncate(job.task_prompt || '', 120) || '—')}</td>
                    <td data-label="${esc(t('dashboard.cronjobs_col_actions'))}">
                        <div class="cronjobs-row-actions">
                        <button type="button" class="cronjobs-row-btn"
                            data-cron-id="${esc(job.id || '')}"
                            data-cron-expr="${esc(job.cron_expr || '')}"
                            data-cron-prompt="${esc(job.task_prompt || '')}"
                            data-cron-source="${esc(job.source || 'agent')}"
                            data-cron-disabled="${disabled ? 'true' : 'false'}"
                            onclick="openCronEditModal(this)"
                            title="${esc(t('dashboard.cron_edit_title'))}">${esc(t('dashboard.btn_edit'))}</button>
                        <button type="button" class="cronjobs-row-btn danger"
                            data-cron-id="${esc(job.id || '')}"
                            onclick="deleteCronJob(this.dataset.cronId)"
                            title="${esc(t('dashboard.cron_btn_delete'))}">${esc(t('dashboard.btn_delete'))}</button>
                        </div>
                    </td>`;
                tbody.appendChild(tr);
            });
        }

        function cronjobSourceLabel(source) {
            return t('dashboard.cronjobs_source_' + (source || 'agent')) || source || 'agent';
        }

        function cronjobStatusLabel(status) {
            if (status === 'disabled') return t('dashboard.cronjobs_status_disabled');
            if (status === 'error') return t('dashboard.cronjobs_status_error');
            return t('dashboard.cronjobs_status_enabled');
        }

        function cronjobFormatNextRun(value, disabled, hasError) {
            if (disabled) return t('dashboard.cronjobs_next_run_disabled');
            if (hasError) return t('dashboard.cronjobs_next_run_error');
            if (!value) return '—';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '—';
            return date.toLocaleString(document.documentElement.lang || LANG, { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' });
        }

        // ═══ Audit Log ══════════════════════════════════════════════════════════════
        let auditOffset = 0;
        let auditTotal = 0;
        const AUDIT_PAGE_SIZE = 25;
        let auditSearchTimer = null;
        let auditRefreshTimer = null;

        async function loadTabAudit() {
            auditOffset = 0;
            await loadAuditPage(0);
        }

        function scheduleAuditRefresh() {
            if (TabState.active !== 'audit') return;
            clearTimeout(auditRefreshTimer);
            auditRefreshTimer = setTimeout(() => {
                auditRefreshTimer = null;
                loadTabAudit();
            }, 200);
        }
        window.scheduleAuditRefresh = scheduleAuditRefresh;

        async function loadAuditPage(offset) {
            auditOffset = Math.max(0, offset || 0);
            CardState.setLoading('card-audit-log');
            const params = auditQueryParams();
            params.set('limit', String(AUDIT_PAGE_SIZE));
            params.set('offset', String(auditOffset));
            try {
                const resp = await fetch('/api/dashboard/audit?' + params.toString(), { credentials: 'same-origin' });
                if (!resp.ok) throw new Error('Audit load failed');
                const page = await resp.json();
                renderAuditEvents(page);
                CardState.setLoaded('card-audit-log');
            } catch (e) {
                console.warn('Audit load failed', e);
                CardState.setError('card-audit-log', () => loadAuditPage(auditOffset), { status: 0 });
                if (typeof showToast === 'function') showToast(t('dashboard.audit_error_load'), 'error', 5000);
            }
        }

        function setupAuditControls() {
            const search = document.getElementById('audit-search');
            const refresh = document.getElementById('audit-refresh');
            const clear = document.getElementById('audit-clear-filtered');
            const prev = document.getElementById('audit-prev');
            const next = document.getElementById('audit-next');
            const filterIDs = ['audit-source-filter', 'audit-status-filter', 'audit-type-filter', 'audit-from-filter', 'audit-to-filter'];
            if (search) {
                search.addEventListener('input', () => {
                    clearTimeout(auditSearchTimer);
                    auditSearchTimer = setTimeout(() => loadTabAudit(), 250);
                });
                search.addEventListener('keydown', (e) => {
                    if (e.key === 'Enter') {
                        e.preventDefault();
                        clearTimeout(auditSearchTimer);
                        loadTabAudit();
                    }
                });
            }
            filterIDs.forEach(id => {
                const el = document.getElementById(id);
                if (el) el.addEventListener('change', () => loadTabAudit());
            });
            if (refresh) {
                refresh.addEventListener('click', async () => {
                    refresh.classList.add('is-busy');
                    refresh.disabled = true;
                    refresh.setAttribute('aria-busy', 'true');
                    try {
                        await loadTabAudit();
                    } finally {
                        refresh.classList.remove('is-busy');
                        refresh.disabled = false;
                        refresh.removeAttribute('aria-busy');
                    }
                });
            }
            if (clear) clear.addEventListener('click', clearFilteredAuditEvents);
            if (prev) prev.addEventListener('click', () => loadAuditPage(Math.max(0, auditOffset - AUDIT_PAGE_SIZE)));
            if (next) next.addEventListener('click', () => loadAuditPage(auditOffset + AUDIT_PAGE_SIZE));
        }

        function auditQueryParams() {
            const params = new URLSearchParams();
            const q = (document.getElementById('audit-search')?.value || '').trim();
            const source = document.getElementById('audit-source-filter')?.value || '';
            const status = document.getElementById('audit-status-filter')?.value || '';
            const type = document.getElementById('audit-type-filter')?.value || '';
            const from = auditDateToRFC3339(document.getElementById('audit-from-filter')?.value || '');
            const to = auditDateToRFC3339(document.getElementById('audit-to-filter')?.value || '');
            if (q) params.set('q', q);
            if (source) params.set('source', source);
            if (status) params.set('status', status);
            if (type) params.set('type', type);
            if (from) params.set('from', from);
            if (to) params.set('to', to);
            return params;
        }

        function auditDateToRFC3339(value) {
            if (!value) return '';
            const date = new Date(value);
            if (Number.isNaN(date.getTime())) return '';
            return date.toISOString();
        }

        function renderAuditEvents(page) {
            const tbody = document.getElementById('audit-tbody');
            const emptyEl = document.getElementById('audit-empty');
            const prev = document.getElementById('audit-prev');
            const next = document.getElementById('audit-next');
            const meta = document.getElementById('audit-page-meta');
            if (!tbody) return;
            const entries = Array.isArray(page?.entries) ? page.entries : [];
            auditTotal = Number(page?.total || 0);
            tbody.innerHTML = '';
            if (entries.length === 0) {
                if (emptyEl) emptyEl.style.display = '';
            } else {
                if (emptyEl) emptyEl.style.display = 'none';
            }
            entries.forEach(event => {
                const tr = document.createElement('tr');
                const timeLabel = event.timestamp ? new Date(event.timestamp).toLocaleString(document.documentElement.lang || LANG, { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }) : '—';
                const duration = formatAuditDuration(event.duration_ms || 0);
                const status = event.status || 'warning';
                const metadata = parseAuditMetadata(event.metadata_json);
                const stateHistory = Array.isArray(metadata?.state_history) ? metadata.state_history.join(' -> ') : '';
                const operation = metadata?.operation ? `${t('dashboard.audit_col_type')}: ${metadata.operation}` : '';
                const detail = [event.summary || '', event.detail || '', stateHistory ? `State: ${stateHistory}` : '', operation].filter(Boolean).join('\n\n');
                tr.innerHTML = `
                    <td class="audit-cell-time" data-label="${esc(t('dashboard.audit_col_time'))}">${esc(timeLabel)}</td>
                    <td class="audit-cell-source" data-label="${esc(t('dashboard.audit_col_source'))}"><span class="audit-source audit-source-${esc(event.source || 'system')}">${esc(auditSourceLabel(event.source))}</span></td>
                    <td class="audit-cell-type" data-label="${esc(t('dashboard.audit_col_type'))}">${esc(auditTypeLabel(event.event_type))}</td>
                    <td class="audit-cell-target" data-label="${esc(t('dashboard.audit_col_target'))}" title="${esc(event.target_id || '')}">${esc(event.target_name || event.target_id || '—')}</td>
                    <td class="audit-cell-status" data-label="${esc(t('dashboard.audit_col_status'))}"><span class="audit-status audit-status-${esc(status)}">${esc(auditStatusLabel(status))}</span></td>
                    <td class="audit-cell-summary" data-label="${esc(t('dashboard.audit_col_summary'))}" title="${esc(detail)}"><span class="audit-summary-text">${esc(event.summary || '—')}</span></td>
                    <td class="audit-cell-duration" data-label="${esc(t('dashboard.audit_col_duration'))}">${esc(duration)}</td>
                    <td class="audit-cell-actions" data-label="${esc(t('dashboard.audit_col_actions'))}"><button type="button" class="audit-row-delete" onclick="deleteAuditEvent(${Number(event.id || 0)})" title="${esc(t('dashboard.audit_delete'))}">${esc(t('dashboard.btn_delete'))}</button></td>`;
                tbody.appendChild(tr);
            });
            const end = Math.min(auditOffset + entries.length, auditTotal);
            if (meta) meta.textContent = auditTotal > 0 ? t('dashboard.audit_page_meta', { start: auditOffset + 1, end, total: auditTotal }) : '';
            if (prev) prev.disabled = auditOffset <= 0;
            if (next) next.disabled = auditOffset + AUDIT_PAGE_SIZE >= auditTotal;
        }

        function formatAuditDuration(ms) {
            const n = Number(ms || 0);
            if (n <= 0) return '—';
            if (n >= 60000) return `${(n / 60000).toFixed(1)}m`;
            if (n >= 1000) return `${(n / 1000).toFixed(1)}s`;
            return `${n}ms`;
        }

        function auditSourceLabel(source) {
            return t('dashboard.audit_source_' + (source || 'system')) || source || 'system';
        }

        function auditStatusLabel(status) {
            return t('dashboard.audit_status_' + (status || 'warning')) || status || 'warning';
        }

        function auditTypeLabel(type) {
            return t('dashboard.audit_type_' + (type || '').replace(/-/g, '_')) || type || '—';
        }

        function parseAuditMetadata(raw) {
            if (!raw) return {};
            try {
                const parsed = JSON.parse(raw);
                return parsed && typeof parsed === 'object' ? parsed : {};
            } catch (e) {
                return {};
            }
        }

        async function deleteAuditEvent(id) {
            if (!id) return;
            if (!await showConfirm(t('dashboard.audit_confirm_delete'))) return;
            try {
                const resp = await fetch('/api/dashboard/audit/' + encodeURIComponent(String(id)), {
                    method: 'DELETE',
                    credentials: 'same-origin'
                });
                if (!resp.ok) throw new Error('Delete failed');
                if (typeof showToast === 'function') showToast(t('dashboard.audit_deleted'), 'success', 2500);
                await loadAuditPage(auditOffset);
            } catch (e) {
                await showAlert(t('dashboard.error_title'), t('dashboard.audit_error_delete'));
            }
        }

        async function clearFilteredAuditEvents() {
            const params = auditQueryParams();
            const hasFilter = Array.from(params.keys()).some(k => !['limit', 'offset'].includes(k));
            const confirmed = await showConfirm(
                t('dashboard.audit_confirm_clear_title'),
                hasFilter ? t('dashboard.audit_confirm_clear_filtered') : t('dashboard.audit_confirm_clear_all')
            );
            if (!confirmed) return;
            const body = Object.fromEntries(params.entries());
            body.confirm = 'DELETE_AUDIT_EVENTS';
            try {
                const resp = await fetch('/api/dashboard/audit', {
                    method: 'DELETE',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                if (!resp.ok) throw new Error('Clear failed');
                const data = await resp.json().catch(() => ({ deleted: 0 }));
                if (typeof showToast === 'function') showToast(t('dashboard.audit_cleared', { count: data.deleted || 0 }), 'success', 3500);
                await loadTabAudit();
            } catch (e) {
                await showAlert(t('dashboard.error_title'), t('dashboard.audit_error_clear'));
            }
        }

        // ═══ Mission History ═══════════════════════════════════════════════════════
        let mhOffset = 0;
        const MH_PAGE_SIZE = 10;

        async function loadMissionHistory(append) {
            // Reset offset on a fresh load so re-entering the overview tab starts from the top.
            if (!append) mhOffset = 0;
            try {
                const url = `/api/dashboard/mission-history?limit=${MH_PAGE_SIZE}&offset=${mhOffset}`;
                const resp = await fetch(url, { credentials: 'same-origin' });
                if (!resp.ok) {
                    if (!append && window.CardState) {
                        window.CardState.setError('card-mission-history', () => loadMissionHistory(false), { status: resp.status });
                    }
                    return;
                }
                const data = await resp.json();
                renderMissionHistory(data, append);
                if (!append && window.CardState) {
                    window.CardState.setLoaded('card-mission-history');
                }
            } catch (e) {
                console.warn('Mission history load failed', e);
                if (!append && window.CardState) {
                    window.CardState.setError('card-mission-history', () => loadMissionHistory(false), { status: 0 });
                }
            }
        }

        function renderMissionHistory(page, append) {
            const tbody = document.getElementById('mission-history-tbody');
            const emptyEl = document.getElementById('mission-history-empty');
            const moreBtn = document.getElementById('mission-history-more');
            if (!tbody) return;

            const entries = page.entries || [];
            if (!append) tbody.innerHTML = '';

            if (entries.length === 0 && !append) {
                if (emptyEl) emptyEl.style.display = '';
                if (moreBtn) moreBtn.style.display = 'none';
                return;
            }
            if (emptyEl) emptyEl.style.display = 'none';

            // Use SVG sprite icons (see dash-icons.js) so the status indicator renders
            // consistently across platforms and stays accessible for screen readers.
            const statusIcons = { success: dashIcon('check'), error: dashIcon('x'), running: dashIcon('play') };
            const triggerLabels = {
                manual: dashIcon('edit'), cron: dashIcon('cron'), webhook: dashIcon('link'), email: dashIcon('email'),
                mqtt: dashIcon('wifi'), daemon_wake: dashIcon('daemon'), mission_completed: dashIcon('check'),
                system_startup: dashIcon('play'), egg_hatched: dashIcon('egg'), nest_cleared: dashIcon('box'),
                device_connected: dashIcon('plug'), device_disconnected: dashIcon('plug'),
                fritzbox_call: dashIcon('speaker'), budget_warning: dashIcon('warning'), budget_exceeded: dashIcon('warning'),
                home_assistant_state: dashIcon('home'),
            };

            entries.forEach(e => {
                const tr = document.createElement('tr');
                const statusCls = e.status === 'success' ? 'mh-status-success' : e.status === 'error' ? 'mh-status-error' : 'mh-status-running';
                const icon = statusIcons[e.status] || '❓';
                const trigIcon = triggerLabels[e.trigger_type] || '⚡';
                const dur = e.duration_ms > 0 ? (e.duration_ms >= 60000 ? `${(e.duration_ms / 60000).toFixed(1)}m` : `${(e.duration_ms / 1000).toFixed(1)}s`) : '—';
                const started = e.started_at ? new Date(e.started_at).toLocaleString(document.documentElement.lang || LANG, { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' }) : '—';
                tr.innerHTML = `
                    <td title="${esc(e.mission_id)}">${esc(e.mission_name || e.mission_id)}</td>
                    <td><span class="mh-status ${statusCls}">${icon} ${tOr('dashboard.mh_status_' + e.status, esc(e.status))}</span></td>
                    <td><span class="mh-trigger">${trigIcon} ${esc(e.trigger_type || '—')}</span></td>
                    <td>${started}</td>
                    <td>${dur}</td>`;
                tbody.appendChild(tr);
            });

            mhOffset += entries.length;
            const hasMore = page.total > mhOffset;
            if (moreBtn) {
                moreBtn.style.display = hasMore ? '' : 'none';
                moreBtn.onclick = () => loadMissionHistory(true);
            }
        }

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
                { icon: dashIcon('rocket'), val: m.total || 0, lbl: t('dashboard.operations_missions'), sub: m.running ? t('dashboard.operations_missions_active', {n: m.running}) : (m.queued ? t('dashboard.operations_missions_queued', {n: m.queued}) : t('dashboard.operations_missions_active', {n: m.enabled || 0})) },
                { icon: dashIcon('egg'), val: inv.nests || 0, lbl: t('dashboard.operations_nests'), sub: t('dashboard.operations_eggs_connected', {n: inv.connected_eggs || 0}) },
                { icon: dashIcon('folder'), val: idx.indexed_files || 0, lbl: t('dashboard.operations_indexed'), sub: idx.enabled ? t('dashboard.operations_of_files', {n: idx.total_files || 0}) : t('dashboard.operations_disabled') },
                { icon: dashIcon('wifi'), val: mq.connected ? '✓' : '✗', lbl: t('dashboard.operations_mqtt'), sub: mq.enabled ? t('dashboard.operations_buffered', {n: mq.buffer || 0}) : t('dashboard.operations_disabled') },
                { icon: dashIcon('note'), val: notes.open || 0, lbl: t('dashboard.operations_notes_open'), sub: t('dashboard.operations_notes_sub', {total: notes.total || 0, done: notes.done || 0}) },
                { icon: dashIcon('lock'), val: sec.vault_keys || 0, lbl: t('dashboard.operations_vault_keys'), sub: t('dashboard.operations_api_tokens', {n: sec.tokens || 0}) },
                { icon: dashIcon('device'), val: overview.devices || 0, lbl: t('dashboard.operations_devices'), sub: t('dashboard.operations_inventory') },
                { icon: dashIcon('brain'), val: overview.context?.has_summary ? '✓' : '✗', lbl: t('dashboard.operations_summary'), sub: t('dashboard.operations_chars_count', {n: ((overview.context?.total_chars || 0) / 1000).toFixed(1)}) },
                { icon: dashIcon('clipboard'), val: (overview.cheatsheets?.total || 0), lbl: t('dashboard.operations_cheatsheets'), sub: t('dashboard.operations_cheatsheets_active', {n: overview.cheatsheets?.active || 0}) },
                { icon: dashIcon('globe'), val: overview.tunnel?.running ? '✓' : '✗', lbl: t('dashboard.operations_tunnel'), sub: overview.tunnel?.url ? truncate(overview.tunnel.url, 24) : t('dashboard.operations_disabled') },
            ];

            grid.innerHTML = items.map(s =>
                `<div class="ops-stat">
                    <div class="ops-stat-icon">${s.icon}</div>
                    <div class="ops-stat-val">${esc(String(s.val))}</div>
                    <div class="ops-stat-lbl">${esc(s.lbl)}</div>
                    <div class="ops-stat-sub">${esc(s.sub)}</div>
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
            const planner = overview.planner || {};
            const appointments = planner.appointments || {};
            const todos = planner.todos || {};
            const plans = planner.plans || {};
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
                    info: (mqtt.enabled && mqtt.buffer) ? t('dashboard.operations_buffered', {n: mqtt.buffer}) : ''
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
                    info: m.queued ? t('dashboard.operations_missions_queued', {n: m.queued}) : ''
                },
                {
                    icon: '🔐',
                    lbl: t('dashboard.operations_vault_keys'),
                    val: sec.vault_keys || 0,
                    status: 'neutral',
                    info: sec.tokens ? t('dashboard.operations_api_tokens', {n: sec.tokens}) : ''
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
                    info: sk.pending > 0 ? t('dashboard.quickstatus_n_pending', {n: sk.pending}) : ''
                },
                {
                    icon: '🪶',
                    lbl: t('dashboard.integration_helper_llm'),
                    val: integrations.helper_llm ? t('dashboard.helper_llm_state_enabled') : t('dashboard.helper_llm_state_disabled'),
                    status: integrations.helper_llm ? 'ok' : 'neutral',
                    info: ''
                },
                {
                    icon: '📅',
                    lbl: t('dashboard.quickstatus_appointments'),
                    val: `${appointments.upcoming || 0} / ${appointments.total || 0}`,
                    status: (appointments.upcoming || 0) > 0 ? 'ok' : 'neutral',
                    info: '',
                    href: '/knowledge'
                },
                {
                    icon: '✅',
                    lbl: t('dashboard.quickstatus_todos'),
                    val: `${(todos.open || 0) + (todos.in_progress || 0)} / ${todos.total || 0}`,
                    status: (todos.overdue || 0) > 0 ? 'warning' : (((todos.open || 0) + (todos.in_progress || 0)) > 0 ? 'ok' : 'neutral'),
                    info: '',
                    href: '/knowledge'
                },
                {
                    icon: '🗺️',
                    lbl: t('dashboard.quickstatus_plans'),
                    val: plans.active || 0,
                    status: (plans.blocked || 0) > 0 ? 'warning' : ((plans.active || 0) > 0 ? 'ok' : 'neutral'),
                    info: plans.progress_pct ? `${plans.progress_pct}%` : '',
                    href: '/plans'
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
                    lbl: t('dashboard.quickstatus_daemons'),
                    val: `${running} / ${total}`,
                    status: autoDisabled > 0 ? 'warning' : (running > 0 ? 'ok' : 'neutral'),
                    info: autoDisabled > 0 ? t('dashboard.quickstatus_n_auto_disabled', {n: autoDisabled}) : ''
                });
            }

            el.innerHTML = items.map(s => {
                const tag = s.href ? 'a' : 'div';
                const href = s.href ? ` href="${esc(s.href)}"` : '';
                return `<${tag} class="qs-item ${esc(s.status || '')}"${href}>
                    <div class="qs-icon">${s.icon}</div>
                    <div class="qs-label">${esc(s.lbl)}</div>
                    <div class="qs-val">${esc(String(s.val))}</div>
                    ${s.info ? `<div class="qs-info" title="${esc(s.info)}">${esc(s.info)}</div>` : ''}
                </${tag}>`;
            }).join('');
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
                        <span class="compression-rank-bar" data-bar-width="${Math.max(4, (e.savings_ratio || 0) * 100)}"></span>
                        <span class="compression-rank-val">${formatChars(e.saved_chars)} (${(e.savings_ratio * 100).toFixed(0)}%)</span>
                    </div>`
                ).join('');
                if (typeof applyDynamicSurfaceVars === 'function') applyDynamicSurfaceVars(toolsList);
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
                        <span class="compression-rank-bar" data-bar-width="${Math.max(4, ((e.saved_chars || 0) / maxSaved) * 100)}"></span>
                        <span class="compression-rank-val">${formatChars(e.saved_chars)} ×${e.count || 0}</span>
                    </div>`
                ).join('');
                if (typeof applyDynamicSurfaceVars === 'function') applyDynamicSurfaceVars(filtersList);
            }
        }

        function renderIntegrations(overview) {
            if (!overview || !overview.integrations) return;
            const grid = document.getElementById('integration-grid');
            const icons = {
                telegram: dashIcon('chat'), discord: dashIcon('chat'), email: dashIcon('email'), home_assistant: dashIcon('home'),
                docker: dashIcon('docker'), co_agents: dashIcon('bot'), webhooks: dashIcon('link'), webdav: dashIcon('cloud'),
                koofr: dashIcon('cloud'), chromecast: dashIcon('monitor'), bluetooth: dashIcon('speaker'), proxmox: dashIcon('device'), frigate: dashIcon('video'), three_d_printers: dashIcon('printer'), ollama: dashIcon('brain'),
                rocketchat: dashIcon('chat'), tailscale: dashIcon('lock'), ansible: dashIcon('wrench'), invasion: dashIcon('egg'),
                github: dashIcon('package'), mqtt: dashIcon('wifi'), budget: dashIcon('wallet'), indexing: dashIcon('folder'),
                auth: dashIcon('key'), fallback_llm: dashIcon('refresh'), helper_llm: dashIcon('feather'), personality_v2: dashIcon('drama'), user_profiling: dashIcon('user'), tts: dashIcon('speaker'),
                piper_tts: dashIcon('speaker'), supertonic_tts: dashIcon('speaker'),
                paperless_ngx: dashIcon('doc'), cloudflare_tunnel: dashIcon('cloud'),
                fritzbox: dashIcon('plug'), meshcentral: dashIcon('monitor'), a2a: dashIcon('link'),
                adguard: dashIcon('shield'), s3: dashIcon('database'), mcp: dashIcon('plug'), mcp_server: dashIcon('plug'), dograh: dashIcon('cube'),
                memory_analysis: dashIcon('brain'), llm_guardian: dashIcon('shield'), security_proxy: dashIcon('lock'),
                sandbox: dashIcon('box'), ai_gateway: dashIcon('globe'), image_generation: dashIcon('palette'),
                evomap: dashIcon('globe'),
                google_workspace: dashIcon('email'), netlify: dashIcon('rocket'),
                homepage: dashIcon('home'), virustotal: dashIcon('virus'), brave_search: dashIcon('search'),
                firewall: dashIcon('firewall'), remote_control: dashIcon('device'), web_scraper: dashIcon('spider'),
                skill_manager: dashIcon('puzzle')
            };
            const names = {
                telegram: t('dashboard.integration_telegram'), discord: t('dashboard.integration_discord'),
                email: t('dashboard.integration_email'), home_assistant: t('dashboard.integration_home_assistant'),
                docker: t('dashboard.integration_docker'), co_agents: t('dashboard.integration_co_agents'),
                webhooks: t('dashboard.integration_webhooks'), webdav: t('dashboard.integration_webdav'),
                koofr: t('dashboard.integration_koofr'), chromecast: t('dashboard.integration_chromecast'), bluetooth: t('dashboard.integration_bluetooth'),
                proxmox: t('dashboard.integration_proxmox'), frigate: t('dashboard.integration_frigate'), three_d_printers: t('dashboard.integration_three_d_printers'), ollama: t('dashboard.integration_ollama'),
                rocketchat: t('dashboard.integration_rocketchat'), tailscale: t('dashboard.integration_tailscale'),
                ansible: t('dashboard.integration_ansible'), invasion: t('dashboard.integration_invasion'),
                github: t('dashboard.integration_github'), mqtt: t('dashboard.integration_mqtt'),
                budget: t('dashboard.integration_budget'), indexing: t('dashboard.integration_indexing'),
                auth: t('dashboard.integration_auth'), fallback_llm: t('dashboard.integration_fallback_llm'),
                helper_llm: t('dashboard.integration_helper_llm'),
                personality_v2: t('dashboard.integration_personality_v2'), user_profiling: t('dashboard.integration_user_profiling'),
                tts: t('dashboard.integration_tts'),
                piper_tts: t('dashboard.integration_piper_tts'), supertonic_tts: t('dashboard.integration_supertonic_tts'),
                paperless_ngx: t('dashboard.integration_paperless_ngx'),
                cloudflare_tunnel: t('dashboard.integration_cloudflare_tunnel'),
                fritzbox: t('dashboard.integration_fritzbox'),
                meshcentral: t('dashboard.integration_meshcentral'), a2a: t('dashboard.integration_a2a'),
                adguard: t('dashboard.integration_adguard'), s3: t('dashboard.integration_s3'),
                mcp: t('dashboard.integration_mcp'), mcp_server: t('dashboard.integration_mcp_server'), dograh: t('dashboard.integration_dograh'),
                memory_analysis: t('dashboard.integration_memory_analysis'),
                llm_guardian: t('dashboard.integration_llm_guardian'),
                security_proxy: t('dashboard.integration_security_proxy'),
                sandbox: t('dashboard.integration_sandbox'), ai_gateway: t('dashboard.integration_ai_gateway'),
                evomap: t('dashboard.integration_evomap'),
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
                return `<span class="int-badge ${cls}">${icons[key] || '•'} ${esc(names[key] || key)}</span>`;
            }).join('');
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
            try {
                const data = await API.get('/api/dashboard/helper-llm');
                renderHelperLLMCard(data);
                if (typeof CardState !== 'undefined' && CardState.setLoaded) {
                    CardState.setLoaded('card-helper-llm');
                }
            } catch (_) {
                if (typeof CardState !== 'undefined' && CardState.setError) {
                    CardState.setError('card-helper-llm', loadHelperLLMCard);
                }
            }
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
                        <div class="guardian-metric-lbl">${t('dashboard.daemons_total')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val ok">${running}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.daemons_running')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val">${stopped}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.daemons_stopped')}</div>
                    </div>
                    <div class="guardian-metric">
                        <div class="guardian-metric-val${errored > 0 ? ' warn' : ''}">${errored}</div>
                        <div class="guardian-metric-lbl">${t('dashboard.daemons_error')}</div>
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
                    const startedMs = Date.parse(d.started_at);
                    if (!Number.isNaN(startedMs)) {
                        const ms = Date.now() - startedMs;
                        uptimeHtml = `<span class="daemon-uptime">${formatDuration(ms)}</span>`;
                    }
                }

                // Wake-up stats badge
                const wakeCount = d.wake_up_count || 0;
                const suppressedCount = d.suppressed_count || 0;
                const wakeLabel = t('dashboard.daemons_wakeups');
                const wakeHtml = wakeCount > 0
                    ? `<span class="daemon-badge daemon-badge-wake" title="${wakeLabel}: ${wakeCount}${suppressedCount > 0 ? ` (${suppressedCount} suppressed)` : ''}">💬 ${wakeCount}</span>`
                    : '';

                // Restart count badge
                const restartCount = d.restart_count || 0;
                const restartLabel = t('dashboard.daemons_restarts');
                const restartHtml = restartCount > 0
                    ? `<span class="daemon-badge daemon-badge-restart${restartCount >= 3 ? ' warn' : ''}" title="${restartLabel}: ${restartCount}">↻ ${restartCount}</span>`
                    : '';

                // Last wake-up time
                let lastWakeHtml = '';
                if (d.last_wake_up) {
                    const wakeMs = Date.parse(d.last_wake_up);
                    if (!Number.isNaN(wakeMs)) {
                        const wLabel = t('dashboard.daemons_last_wakeup');
                        lastWakeHtml = `<span class="daemon-meta-item" title="${wLabel}">${relativeTime(wakeMs)}</span>`;
                    }
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
            const results = await Promise.all([
                API.getWithStatus('/api/dashboard/journal?limit=15'),
                API.getWithStatus('/api/dashboard/journal/summaries?days=7')
            ]);
            const [entriesR, summariesR] = results;
            if (!entriesR.ok && !summariesR.ok) {
                CardState.setError('card-journal', loadJournal, { status: entriesR.status || summariesR.status || 0 });
                return;
            }
            CardState.setLoaded('card-journal');
            const entries = entriesR.ok ? entriesR.data : null;
            const summaries = summariesR.ok ? summariesR.data : null;
            renderJournalTimeline(entries?.entries);
            renderJournalSummary(summaries?.summaries);
        }

        async function loadErrorPatterns() {
            const data = await API.get('/api/dashboard/errors');
            renderErrorPatterns(data);
        }
