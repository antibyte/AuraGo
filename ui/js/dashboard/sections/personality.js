// AuraGo dashboard – section "personality": mood, affect panel, character notes and emotion history.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

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

        DashSections.register({ id: 'personality', load: loadSectionPersonality });
