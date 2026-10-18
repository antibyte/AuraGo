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
            window.AuraPersonalityDynamics.render(document.getElementById('personality-dynamics'), data, renderMoodBadge);
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
            dashSetHidden(document.getElementById('personality-content'), false);
            dashSetHidden(document.getElementById('personality-disabled'), true);
            const badge = document.getElementById('mood-badge');
            const trigger = document.getElementById('mood-trigger');
            const nameLocalized = moodNameMap[data.mood] || data.mood;
            if (badge) badge.textContent = '🎭 ' + nameLocalized;
            if (trigger && data.trigger) trigger.textContent = '"' + affectCauseLabel(data.trigger) + '"';
            renderCharacterNotes(data);
            renderAffectPanel(data);

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

        function affectCauseLabel(code) {
            const key = 'dashboard.personality_affect_cause_' + String(code || '').trim();
            const label = typeof tOr === 'function' ? tOr(key, '') : t(key);
            if (label && label !== key) return label;
            return code || t('dashboard.personality_no_trigger');
        }

        function affectSourceLabel(source) {
            const key = 'dashboard.personality_affect_source_' + String(source || '').trim();
            const label = typeof tOr === 'function' ? tOr(key, '') : t(key);
            if (label && label !== key) return label;
            return source || '';
        }

        function renderAffectPanel(data) {
            const panel = document.getElementById('affect-panel');
            const list = document.getElementById('affect-event-list');
            const empty = document.getElementById('affect-empty');
            const valenceEl = document.getElementById('affect-valence');
            const arousalEl = document.getElementById('affect-arousal');
            if (!panel || !list) return;
            dashSetHidden(panel, false);
            if (valenceEl) {
                valenceEl.textContent = data.affect_valence == null ? '—' : Number(data.affect_valence).toFixed(2);
            }
            if (arousalEl) {
                arousalEl.textContent = data.affect_arousal == null ? '—' : Number(data.affect_arousal).toFixed(2);
            }
            const events = Array.isArray(data.affect_events) ? data.affect_events : [];
            list.replaceChildren();
            events.slice(0, 8).forEach((event) => {
                const item = document.createElement('li');
                const cause = document.createElement('strong');
                cause.textContent = affectCauseLabel(event.cause_code);
                const meta = document.createElement('span');
                const bits = [];
                const source = affectSourceLabel(event.source);
                if (source) bits.push(source);
                if (event.detail) bits.push(event.detail);
                meta.textContent = bits.join(' · ');
                item.append(cause, meta);
                list.appendChild(item);
            });
            dashSetHidden(empty, events.length > 0);
            dashSetHidden(list, events.length === 0);
        }

        async function characterNoteAction(payload) {
            const resp = await fetch('/api/personality/character-notes', {
                method: 'POST',
                credentials: 'same-origin',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
            if (!resp.ok) return null;
            // The mutation changes /api/personality/state without a personality_update SSE.
            DashAPI.invalidate('/api/personality/');
            const data = await resp.json();
            renderMoodBadge(data);
            return data;
        }

        function renderCharacterNotes(data) {
            const wrap = document.getElementById('character-notes');
            const list = document.getElementById('character-notes-list');
            const empty = document.getElementById('character-notes-empty');
            const causeEl = document.getElementById('character-notes-cause');
            const milestoneEl = document.getElementById('character-notes-milestones');
            const toggle = document.getElementById('character-narrative-toggle');
            if (!wrap || !list) return;
            dashSetHidden(wrap, false);
            if (toggle) {
                toggle.checked = !!data.narrative_visible;
                if (!toggle.dataset.bound) {
                    toggle.dataset.bound = '1';
                    toggle.addEventListener('change', async () => {
                        await characterNoteAction({ action: 'narrative', visible: toggle.checked });
                    });
                }
            }
            if (causeEl) {
                const cause = data.affect_cause || '';
                causeEl.textContent = cause ? (t('dashboard.personality_notes_cause') + ': ' + affectCauseLabel(cause)) : '';
                dashSetHidden(causeEl, !cause);
            }
            if (milestoneEl) {
                const reviews = Array.isArray(data.needs_discussion) ? data.needs_discussion : [];
                milestoneEl.replaceChildren();
                reviews.forEach((label) => {
                    const row = document.createElement('div');
                    const text = document.createElement('span');
                    text.textContent = t('dashboard.personality_notes_milestone') + ': ' + label;
                    const btn = document.createElement('button');
                    btn.type = 'button';
                    btn.className = 'wh-btn wh-btn-sm';
                    btn.textContent = t('dashboard.personality_notes_review');
                    btn.addEventListener('click', () => characterNoteAction({ action: 'review_milestone', label }));
                    row.append(text, btn);
                    milestoneEl.appendChild(row);
                });
                dashSetHidden(milestoneEl, reviews.length === 0);
            }
            const notes = Array.isArray(data.character_notes) ? data.character_notes : [];
            list.replaceChildren();
            notes.forEach((note) => {
                const item = document.createElement('li');
                const text = document.createElement('span');
                text.textContent = note.text || '';
                const actions = document.createElement('div');
                actions.className = 'character-notes-actions';
                const pin = document.createElement('button');
                pin.type = 'button';
                pin.className = 'wh-btn wh-btn-sm';
                pin.textContent = note.protected ? t('dashboard.personality_notes_unprotect') : t('dashboard.personality_notes_protect');
                pin.addEventListener('click', () => characterNoteAction({ action: note.protected ? 'unprotect' : 'protect', id: note.id }));
                const del = document.createElement('button');
                del.type = 'button';
                del.className = 'wh-btn wh-btn-sm';
                del.textContent = t('dashboard.personality_notes_delete');
                del.addEventListener('click', async () => {
                    const ok = typeof showConfirm === 'function'
                        ? await showConfirm(t('dashboard.personality_notes_delete'), t('dashboard.personality_notes_delete_confirm'))
                        : true;
                    if (ok) await characterNoteAction({ action: 'delete', id: note.id });
                });
                actions.append(pin, del);
                item.append(text, actions);
                list.appendChild(item);
            });
            dashSetHidden(empty, notes.length > 0);
            dashSetHidden(list, notes.length === 0);
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
            renderManagedNoteProtection();
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

        let noteProtectionRequest = 0;
        async function renderManagedNoteProtection() {
            const container = document.getElementById('memory-note-protection');
            if (!container) return;
            const request = ++noteProtectionRequest;
            try {
                const response = await fetch('/api/dashboard/notes', { credentials: 'same-origin' });
                if (!response.ok) throw new Error(t('dashboard.notes_protection_failed'));
                const data = await response.json();
                if (request !== noteProtectionRequest || !container.isConnected) return;
                container.replaceChildren();
                for (const note of data.notes || []) {
                    const row = document.createElement('div');
                    row.className = 'mt-sm';
                    const title = document.createElement('strong');
                    title.textContent = note.title;
                    row.append(title);
                    const controls = [];
                    for (const [field, key] of [['protected', 'dashboard.notes_protected'], ['keep_forever', 'dashboard.notes_keep_forever']]) {
                        const label = document.createElement('label');
                        const input = document.createElement('input');
                        input.type = 'checkbox';
                        input.checked = !!note[field];
                        controls.push(input);
                        label.append(input, document.createTextNode(t(key)));
                        row.append(label);
                        input.addEventListener('change', async () => {
                            controls.forEach(control => { control.disabled = true; });
                            status.textContent = t('dashboard.notes_protection_saving');
                            const proposed = { id: note.id, protected: controls[0].checked, keep_forever: controls[1].checked };
                            try {
                                const result = await fetch('/api/dashboard/notes', { method: 'PATCH', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(proposed) });
                                if (!result.ok) throw new Error(t('dashboard.notes_protection_failed'));
                                note.protected = proposed.protected;
                                note.keep_forever = proposed.keep_forever;
                                status.textContent = t('dashboard.notes_protection_saved');
                            } catch (_) {
                                controls[0].checked = !!note.protected;
                                controls[1].checked = !!note.keep_forever;
                                status.textContent = t('dashboard.notes_protection_failed');
                            } finally { controls.forEach(control => { control.disabled = false; }); }
                        });
                    }
                    const status = document.createElement('span');
                    status.setAttribute('role', 'status');
                    row.append(status);
                    container.append(row);
                }
            } catch (_) { if (request === noteProtectionRequest) container.textContent = t('dashboard.notes_protection_failed'); }
        }

        async function dismissMemoryConflict(button, conflictID) {
            const status = button.parentElement.querySelector('[role="status"]');
            button.disabled = true;
            status.textContent = t('dashboard.memory_conflict_saving');
            try {
                const response = await fetch('/api/memory/conflicts/resolve', {
                    method: 'POST', headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ conflict_id: conflictID, action: 'dismiss' })
                });
                if (!response.ok) throw new Error('dismiss failed');
                status.textContent = t('dashboard.memory_conflict_dismissed');
                button.remove();
            } catch (_) {
                status.textContent = t('dashboard.memory_conflict_failed');
                button.disabled = false;
            }
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
                    (conflictItems.length ? renderCollapsibleList(conflictItems, item => `<div class="memory-curator-row"><span class="memory-conflict-pair mono">${esc(item.left_value || item.doc_id_left || '')} ↔ ${esc(item.right_value || item.doc_id_right || '')}</span><span>${esc(item.reason || '')}</span><button type="button" class="mh-more-btn" onclick="dismissMemoryConflict(this, ${Number(item.id)})">${esc(t('dashboard.memory_conflict_both_valid'))}</button><span role="status"></span></div>`, 3) : `<div class="memory-curator-row">${t('dashboard.memory_conflicts_empty')}</div>`) +
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
                if (typeof loadSectionContent === 'function') await loadSectionContent('memory', { force: true });
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
                loadSectionContent('user', { force: true });
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
                loadSectionContent('user', { force: true });
            } catch (_) {
                if (typeof showToast === 'function') showToast(t('dashboard.profile_save_error'), 'error', 4000);
            }
        }

        // ══════════════════════════════════════════════════════════════════════════════
        // PROMPT BUILDER ANALYTICS
        // ══════════════════════════════════════════════════════════════════════════════

        function formatPromptNumber(value, percent = false) {
            if (!Number.isFinite(value)) return '—';
            const locale = document.documentElement.lang || 'en';
            if (percent && value > 0 && value < 0.1) return '< ' + (0.1).toLocaleString(locale) + ' %';
            return value.toLocaleString(locale, { maximumFractionDigits: 1 }) + (percent ? ' %' : '');
        }

        function renderPromptStats(data) {
            if (!data || !(data.total_builds > 0 || data.fit_count > 0)) {
                dashSetHidden(document.getElementById('prompt-no-data'), false);
                dashSetHidden(document.getElementById('prompt-content'), true);
                return;
            }
            dashSetHidden(document.getElementById('prompt-no-data'), true);
            dashSetHidden(document.getElementById('prompt-content'), false);

            const render = (id, available, items) => {
                const el = document.getElementById(id);
                if (!el) return;
                el.innerHTML = available ? items.map(([value, key, percent]) =>
                    `<div class="prompt-kpi"><div class="prompt-kpi-val">${esc(formatPromptNumber(value, percent))}</div><div class="prompt-kpi-lbl">${esc(t('dashboard.' + key))}</div></div>`
                ).join('') : `<p class="empty-state dash-empty-tight">${esc(t('dashboard.prompt_no_measurements'))}</p>`;
            };
            render('prompt-module-kpis', data.total_builds > 0, [
                [data.avg_modules_loaded, 'prompt_kpi_avg_modules_loaded'],
                [data.avg_modules_used, 'prompt_kpi_avg_modules_used'],
                [data.avg_modules_filtered, 'prompt_kpi_filtered'],
                [data.avg_module_filter_rate_pct, 'prompt_kpi_filter_rate', true],
                [data.avg_filter_savings, 'prompt_kpi_filtered_bytes'],
            ]);
            render('prompt-kpis', data.total_builds > 0, [
                [data.total_builds, 'prompt_kpi_total_builds'],
                [data.avg_raw_len, 'prompt_kpi_raw_bytes'],
                [data.avg_optimized_len, 'prompt_kpi_opt_bytes'],
                [data.avg_optimization_pct, 'prompt_kpi_avg_saving_pct', true],
                [data.total_format_savings, 'prompt_kpi_format_bytes'],
            ]);
            const measured = data.measured_fit_count > 0;
            render('prompt-fit-kpis', data.fit_count > 0, [
                [data.fit_count, 'prompt_kpi_fit_count'],
                [measured ? data.avg_fit_input_tokens : undefined, 'prompt_kpi_input_tokens'],
                [measured ? data.avg_fit_output_tokens : undefined, 'prompt_kpi_output_tokens'],
                [measured ? data.avg_fit_saved_tokens : undefined, 'prompt_kpi_saved_tokens'],
                [measured ? data.avg_fit_input_guides : undefined, 'prompt_kpi_input_guides'],
                [measured ? data.avg_fit_output_guides : undefined, 'prompt_kpi_output_guides'],
                [data.budget_shed_count, 'prompt_kpi_budget_cuts'],
                [data.budget_exceeded_count, 'prompt_kpi_exceeded'],
                [data.shed_rate_pct, 'prompt_kpi_cut_rate', true],
            ]);
            render('prompt-cache-kpis', data.request_count > 0, [
                [data.request_count, 'prompt_kpi_requests'],
                [data.cache_hit_count, 'prompt_kpi_cache_hits'],
                [data.cache_hit_rate_pct, 'prompt_kpi_cache_rate', true],
            ]);

            // Shed section list
            const shedEl = document.getElementById('shed-list');
            const shedCounts = data.shed_section_counts || {};
            const shedKeys = Object.keys(shedCounts).sort((a, b) => shedCounts[b] - shedCounts[a]);
            if (shedKeys.length === 0) {
                shedEl.innerHTML = '<div class="empty-state dash-empty-tight">' + esc(t(data.fit_count > 0 ? 'dashboard.prompt_no_sections_shed' : 'dashboard.prompt_no_measurements')) + '</div>';
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
            if (legendEl) legendEl.textContent = '';
            if (legendEl && Object.keys(avgSections).length > 0) {
                const total = sectionOrder.reduce((s, k) => s + (avgSections[k] || 0), 0);
                legendEl.innerHTML = sectionOrder
                    .filter(k => (avgSections[k] || 0) > 0)
                    .map((k, i) => {
                        const pct = formatPromptNumber(total > 0 ? avgSections[k] / total * 100 : 0, true);
                        const colorIdx = sectionOrder.indexOf(k);
                        return `<div class="prompt-section-legend-item">
                            <span class="prompt-section-legend-dot prompt-section-legend-dot-${colorIdx}"></span>
                            <span class="prompt-section-legend-label">${esc(sectionNameMap[k] || k)}</span>
                            <span class="prompt-section-legend-val">${esc(formatPromptNumber(avgSections[k]))} <span class="prompt-section-legend-pct">(${esc(pct)})</span></span>
                        </div>`;
                    }).join('');
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
        // OPERATIONS & INTEGRATIONS
        // ══════════════════════════════════════════════════════════════════════════════

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
                if (typeof loadSectionContent === 'function') await loadSectionContent('memory', { force: true });
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
                if (typeof loadSectionContent === 'function') await loadSectionContent('memory', { force: true });
            } catch (e) {
                if (typeof showToast === 'function') showToast(t('dashboard.memory_hygiene_error'), 'error', 5000);
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
                    lbl: t('dashboard.opt_verified_trace_events'),
                    val: opt.total_trace_events || 0
                },
                {lbl: t('dashboard.opt_exposures'), val: opt.exposures || 0},
                {lbl: t('dashboard.opt_legacy'), val: opt.legacy_trace_events || 0},
                {
                    lbl: t('dashboard.opt_global_success_rate'),
                    val: opt.total_trace_events > 0 ? `${((opt.global_success_rate || 0) * 100).toFixed(1)}%` : '—'
                }
            ];

            el.innerHTML = items.map(s =>
                `<div class="stat-item">
                    <div class="stat-value">${s.val}</div>
                    <div class="stat-label">${s.lbl}</div>
                </div>`
            ).join('');
            if (Array.isArray(opt.variants) && opt.variants.length) {
                const rows = opt.variants.slice(0, 10).map(v => `<li>${esc(v.manual)} · ${esc(t(v.active ? 'dashboard.opt_variant_active' : v.shadow ? 'dashboard.opt_variant_shadow' : 'dashboard.opt_variant_retired'))}: ${esc(t('dashboard.opt_reason_' + v.reason))}</li>`).join('');
                el.insertAdjacentHTML('beforeend', `<details><summary>${esc(t('dashboard.opt_evidence'))}</summary><ul>${rows}</ul></details>`);
            }
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
                koofr: dashIcon('cloud'), chromecast: dashIcon('monitor'), bluetooth: dashIcon('speaker'), network_shares: dashIcon('folder'), proxmox: dashIcon('device'), frigate: dashIcon('video'), go2rtc: dashIcon('video'), ruview: dashIcon('wifi'), three_d_printers: dashIcon('printer'), ollama: dashIcon('brain'),
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
                google_workspace: dashIcon('email'), netlify: dashIcon('rocket'), here_now: dashIcon('rocket'),
                treg: dashIcon('globe'),
                local_wikipedia: dashIcon('book'),
                homepage: dashIcon('home'), virustotal: dashIcon('virus'), brave_search: dashIcon('search'),
                firewall: dashIcon('firewall'), remote_control: dashIcon('device'), web_scraper: dashIcon('spider'),
                skill_manager: dashIcon('puzzle')
            };
            const names = {
                telegram: t('dashboard.integration_telegram'), discord: t('dashboard.integration_discord'),
                email: t('dashboard.integration_email'), home_assistant: t('dashboard.integration_home_assistant'),
                docker: t('dashboard.integration_docker'), co_agents: t('dashboard.integration_co_agents'),
                webhooks: t('dashboard.integration_webhooks'), webdav: t('dashboard.integration_webdav'),
                koofr: t('dashboard.integration_koofr'), chromecast: t('dashboard.integration_chromecast'), bluetooth: t('dashboard.integration_bluetooth'), network_shares: t('dashboard.integration_network_shares'),
                proxmox: t('dashboard.integration_proxmox'), frigate: t('dashboard.integration_frigate'), go2rtc: t('dashboard.integration_go2rtc'), ruview: 'RuView', three_d_printers: t('dashboard.integration_three_d_printers'), ollama: t('dashboard.integration_ollama'),
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
                netlify: t('dashboard.integration_netlify'), here_now: t('dashboard.integration_here_now'),
                treg: 'treg',
                local_wikipedia: t('dashboard.integration_local_wikipedia'),
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
                const content = `${icons[key] || '•'} ${esc(names[key] || key)}`;
                if (key === 'ruview') return `<a class="int-badge ${cls}" href="/desktop?app=ruview">${content}</a>`;
                if (key === 'go2rtc') return `<a class="int-badge ${cls}" href="/desktop?app=network-cameras">${content}</a>`;
                if (key === 'local_wikipedia') {
                    const update = active && overview.local_wikipedia && overview.local_wikipedia.update_available;
                    if (update) {
                        const hint = t('dashboard.local_wikipedia_update_hint', { date: update.date });
                        const title = t('dashboard.local_wikipedia_update_title', { date: update.date });
                        return `<a class="int-badge active-warning" href="/config#local_wikipedia" title="${escAttr(title)}">${content} · ${esc(hint)}</a>`;
                    }
                    return `<a class="int-badge ${cls}" href="/config#local_wikipedia">${content}</a>`;
                }
                return `<span class="int-badge ${cls}">${content}</span>`;
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
