// AuraGo dashboard – section "memory": memory statistics and health, weekly reflection, curation and hygiene.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionMemory({ force = false } = {}) {
            void loadMemoryPrivacyDeletionJobs();
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

        let memoryPrivacyJobsRequest = 0;

        async function loadMemoryPrivacyDeletionJobs() {
            const list = document.getElementById('memory-privacy-job-list');
            const status = document.getElementById('memory-privacy-status');
            const refresh = document.getElementById('memory-privacy-refresh');
            if (!list || !status) return false;
            const request = ++memoryPrivacyJobsRequest;
            if (refresh) refresh.disabled = true;
            status.textContent = t('dashboard.memory_privacy_loading');
            try {
                const response = await fetch('/api/privacy-deletions', { credentials: 'same-origin' });
                const data = await response.json();
                if (!response.ok || data.status === 'error') throw new Error('privacy deletion list unavailable');
                if (request !== memoryPrivacyJobsRequest || !list.isConnected) return false;
                const jobs = Array.isArray(data.jobs) ? data.jobs : [];
                list.replaceChildren();
                if (!jobs.length) {
                    const empty = document.createElement('div');
                    empty.className = 'empty-state dash-empty-tight';
                    empty.textContent = t('dashboard.memory_privacy_empty');
                    list.append(empty);
                } else {
                    for (const job of jobs) list.append(renderMemoryPrivacyDeletionJob(job));
                }
                status.textContent = '';
                return true;
            } catch (_) {
                if (request === memoryPrivacyJobsRequest && list.isConnected) {
                    status.textContent = t('dashboard.memory_privacy_load_error');
                }
                return false;
            } finally {
                if (request === memoryPrivacyJobsRequest && refresh && refresh.isConnected) refresh.disabled = false;
            }
        }

        function memoryPrivacyStatusLabel(status) {
            const normalized = String(status || '').toLowerCase();
            const keys = {
                preparing: 'dashboard.memory_privacy_in_progress',
                awaiting_contact_delete: 'dashboard.memory_privacy_in_progress',
                pending: 'dashboard.memory_privacy_in_progress',
                processing: 'dashboard.memory_privacy_in_progress',
                reconcile_pending: 'dashboard.memory_privacy_reconcile_pending',
                retry_wait: 'dashboard.memory_privacy_retry_wait',
                retained_review: 'dashboard.memory_privacy_retained_review',
                completed: 'dashboard.memory_privacy_completed',
                aborted: 'dashboard.memory_privacy_aborted',
            };
            const key = keys[normalized];
            return key ? t(key) : (normalized || t('dashboard.memory_privacy_unknown'));
        }

        function memoryPrivacyCount(value) {
            if (value === null || value === undefined || value === '') return t('dashboard.memory_privacy_unknown');
            const count = Number(value);
            return Number.isSafeInteger(count) && count >= 0 ? count.toLocaleString() : t('dashboard.memory_privacy_unknown');
        }

        function memoryPrivacyDate(value) {
            if (!value) return t('dashboard.memory_privacy_unknown');
            const date = new Date(value);
            return Number.isNaN(date.getTime()) ? t('dashboard.memory_privacy_unknown') : date.toLocaleString();
        }

        function memoryPrivacyInfoLine(parent, value) {
            const line = document.createElement('div');
            line.className = 'memory-curator-row';
            line.textContent = value;
            parent.append(line);
        }

        function renderMemoryPrivacyDeletionJob(job) {
            const item = document.createElement('article');
            item.className = 'memory-episodic-item';
            item.dataset.memoryPrivacyStatus = String(job.deletion_status || '').toLowerCase();
            const header = document.createElement('div');
            header.className = 'memory-episodic-head';
            const title = document.createElement('span');
            title.className = 'memory-episodic-title';
            title.textContent = t('dashboard.memory_privacy_job');
            const statusChip = document.createElement('span');
            statusChip.className = 'memory-health-strategy-chip';
            statusChip.textContent = memoryPrivacyStatusLabel(job.deletion_status);
            header.append(title, statusChip);
            item.append(header);

            memoryPrivacyInfoLine(item, t('dashboard.memory_privacy_created', { date: memoryPrivacyDate(job.created_at) }));
            memoryPrivacyInfoLine(item, t('dashboard.memory_privacy_counts', {
                pending: memoryPrivacyCount(job.pending_count),
                deleted: memoryPrivacyCount(job.deleted_count),
                retained: memoryPrivacyCount(job.retained_count),
            }));
            const nextRetry = job.next_retry_at ? memoryPrivacyDate(job.next_retry_at) : t('dashboard.memory_privacy_no_retry_time');
            memoryPrivacyInfoLine(item, t('dashboard.memory_privacy_attempts_next_retry', {
                count: memoryPrivacyCount(job.attempt_count),
                date: nextRetry,
            }));
            if (job.error_code) {
                memoryPrivacyInfoLine(item, t('dashboard.memory_privacy_error_code', { code: String(job.error_code) }));
            }

            const actions = document.createElement('div');
            actions.className = 'memory-curator-actionbar';
            const detailsButton = document.createElement('button');
            detailsButton.type = 'button';
            detailsButton.className = 'btn btn-secondary btn-sm mh-more-btn';
            detailsButton.textContent = t('dashboard.memory_privacy_details');
            detailsButton.setAttribute('aria-expanded', 'false');
            const details = document.createElement('div');
            details.className = 'memory-curator-preview';
            details.hidden = true;
            detailsButton.addEventListener('click', async () => {
                if (!details.hidden) {
                    details.hidden = true;
                    detailsButton.textContent = t('dashboard.memory_privacy_details');
                    detailsButton.setAttribute('aria-expanded', 'false');
                    return;
                }
                details.hidden = false;
                detailsButton.disabled = true;
                detailsButton.textContent = t('dashboard.memory_privacy_loading');
                detailsButton.setAttribute('aria-expanded', 'true');
                details.replaceChildren();
                try {
                    const jobID = String(job.deletion_job_id || '');
                    if (!jobID) throw new Error('missing deletion job ID');
                    const response = await fetch('/api/privacy-deletions/' + encodeURIComponent(jobID), { credentials: 'same-origin' });
                    const data = await response.json();
                    if (!response.ok || data.status === 'error' || !data.job) throw new Error('privacy deletion details unavailable');
                    const detail = data.job;
                    memoryPrivacyInfoLine(details, t('dashboard.memory_privacy_updated', { date: memoryPrivacyDate(detail.updated_at) }));
                    memoryPrivacyInfoLine(details, t('dashboard.memory_privacy_attempts_next_retry', {
                        count: memoryPrivacyCount(detail.attempt_count),
                        date: detail.next_retry_at ? memoryPrivacyDate(detail.next_retry_at) : t('dashboard.memory_privacy_no_retry_time'),
                    }));
                    if (detail.error_code) memoryPrivacyInfoLine(details, t('dashboard.memory_privacy_error_code', { code: String(detail.error_code) }));
                    detailsButton.textContent = t('dashboard.memory_privacy_hide_details');
                } catch (_) {
                    details.textContent = t('dashboard.memory_privacy_details_error');
                    detailsButton.textContent = t('dashboard.memory_privacy_hide_details');
                } finally {
                    detailsButton.disabled = false;
                }
            });
            actions.append(detailsButton);

            const jobID = String(job.deletion_job_id || '');
            const jobStatus = String(job.deletion_status || '').toLowerCase();
            if (jobID && (jobStatus === 'retry_wait' || jobStatus === 'reconcile_pending')) {
                const retryButton = document.createElement('button');
                retryButton.type = 'button';
                retryButton.className = 'btn btn-secondary btn-sm mh-more-btn';
                retryButton.setAttribute('data-memory-privacy-retry', '');
                retryButton.textContent = t(jobStatus === 'reconcile_pending'
                    ? 'dashboard.memory_privacy_reconcile_retry'
                    : 'dashboard.memory_privacy_retry');
                retryButton.addEventListener('click', async () => {
                    const status = document.getElementById('memory-privacy-status');
                    retryButton.disabled = true;
                    if (status) status.textContent = t('dashboard.memory_privacy_retrying');
                    let message = 'dashboard.memory_privacy_retry_error';
                    try {
                        const response = await fetch('/api/privacy-deletions/' + encodeURIComponent(jobID) + '/retry', {
                            method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': 'application/json' },
                        });
                        const data = await response.json();
                        if (response.status === 409 && (data.code || data.error) === 'deletion_job_not_retryable') {
                            message = 'dashboard.memory_privacy_not_retryable';
                        } else if (!response.ok || data.status === 'error') {
                            throw new Error('privacy deletion retry failed');
                        } else {
                            message = jobStatus === 'reconcile_pending'
                                ? 'dashboard.memory_privacy_reconcile_queued'
                                : 'dashboard.memory_privacy_retry_queued';
                        }
                    } catch (_) {
                        message = 'dashboard.memory_privacy_retry_error';
                    }
                    const refreshed = await loadMemoryPrivacyDeletionJobs();
                    if (!refreshed && retryButton.isConnected) retryButton.disabled = false;
                    const currentStatus = document.getElementById('memory-privacy-status');
                    if (currentStatus) currentStatus.textContent = t(message);
                });
                actions.append(retryButton);
            }
            item.append(actions, details);
            return item;
        }

        function renderMemoryStats(data) {
            if (!data) return;
            renderManagedNoteProtection();
            const kg = data.knowledge_graph || {};
            const embeddingLbl = data.vectordb_disabled ? t('dashboard.memory_embeddings_disabled') : t('dashboard.memory_embeddings');
            // Headline KPIs (P5); core facts open the core facts modal through the delegated memory-core-facts action.
            renderDashKpiRow(document.getElementById('memory-kpis'), [
                { label: t('dashboard.memory_core_facts'), value: Number(data.core_memory_facts || 0), action: 'memory-core-facts', title: t('dashboard.memory_show_core_facts') },
                { label: t('dashboard.memory_messages'), value: Number(data.chat_messages || 0) },
                { label: embeddingLbl, value: Number(data.vectordb_entries || 0) },
                { label: t('dashboard.memory_chart_graph_nodes'), value: Number(kg.nodes || 0) },
            ]);

            const container = document.getElementById('memory-stats');
            if (!container) return;
            let graphVal = (kg.nodes || 0) + ' / ' + (kg.edges || 0);
            if ((kg.dirty_nodes || 0) > 0) {
                graphVal += ' · ' + t('dashboard.memory_graph_dirty_hint', { count: kg.dirty_nodes });
            }
            // The other counters; the graph tile keeps edges and pending nodes next to the KPI row's node count.
            const stats = [
                { val: graphVal, lbl: t('dashboard.memory_graph_label') },
                { val: data.journal_entries || 0, lbl: t('dashboard.memory_journal') },
                { val: data.notes_count || 0, lbl: t('dashboard.memory_notes') },
                { val: data.error_patterns || 0, lbl: t('dashboard.memory_error_patterns') },
                { val: data.episodic?.total_count || 0, lbl: t('dashboard.memory_episodic') },
            ];
            container.innerHTML = stats.map(s =>
                `<div class="mem-stat"><div class="mem-stat-val">${esc(String(s.val))}</div><div class="mem-stat-lbl">${esc(s.lbl)}</div></div>`
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
            // The curation disclosure shows how many open conflicts wait inside it (P5).
            window.DashDisclosures?.setCount('card-memory:curation', conflicts.length, t('dashboard.memory_conflicts_title'));

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
                    { value: Number.isFinite(data.exhausted_memory_writes) ? data.exhausted_memory_writes.toLocaleString() : '—', label: t('dashboard.memory_exhausted_writes') },
                    { value: Number.isFinite(data.terminal_consolidation_failures) ? data.terminal_consolidation_failures.toLocaleString() : '—', label: t('dashboard.memory_terminal_consolidations') },
                    { value: reflectionActionables.toLocaleString(), label: t('dashboard.memory_reflection_actionables') },
                ];
                const corpus = health.corpus;
                const corpusValue = (value, suffix = '') => corpus?.available && Number.isFinite(value)
                    ? Number(value).toLocaleString(undefined, { maximumFractionDigits: 2 }) + suffix : '—';
                items.push(
                    { value: corpusValue(corpus?.documents), label: t('dashboard.memory_corpus_documents') },
                    { value: corpus?.available ? (corpus.dimensions || []).join(', ') || '—' : '—', label: t('dashboard.memory_corpus_dimensions') },
                    { value: corpusValue((corpus?.database_bytes + corpus?.wal_bytes) / 1048576, ' MiB'), label: t('dashboard.memory_corpus_storage') },
                    { value: corpusValue(corpus?.vector_payload_bytes / 1048576, ' MiB'), label: t('dashboard.memory_corpus_vector_estimate') },
                );
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
                    '<button type="button" class="btn btn-secondary btn-sm mh-more-btn" onclick="runMemoryCurationDryRun()">' + esc(t('dashboard.memory_curator_preview')) + '</button>' +
                    '<button type="button" class="btn btn-secondary btn-sm mh-more-btn memory-curator-apply-btn" onclick="applyMemoryCurationSafeActions()">' + esc(t('dashboard.memory_curator_apply')) + '</button>' +
                    '<button type="button" class="btn btn-secondary btn-sm mh-more-btn" onclick="runMemoryCurationDryRun(true)">' + esc(t('dashboard.memory_curator_show_archived')) + '</button>' +
                    '</div><div id="memory-curator-preview" class="memory-curator-preview"></div>' +
                    '<div id="memory-hygiene-panel" class="memory-hygiene-panel">' +
                    '<div class="memory-subsection-title">' + esc(t('dashboard.memory_hygiene_title')) + '</div>' +
                    '<div class="memory-curator-actionbar">' +
                    '<button type="button" class="btn btn-secondary btn-sm mh-more-btn" onclick="runMemoryHygieneDryRun()">' + esc(t('dashboard.memory_hygiene_preview')) + '</button>' +
                    '<button type="button" class="btn btn-secondary btn-sm mh-more-btn memory-curator-apply-btn" onclick="applyMemoryHygieneSafeActions()">' + esc(t('dashboard.memory_hygiene_apply')) + '</button>' +
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
                    (conflictItems.length ? renderCollapsibleList(conflictItems, item => `<div class="memory-curator-row"><span class="memory-conflict-pair mono">${esc(item.left_value || item.doc_id_left || '')} ↔ ${esc(item.right_value || item.doc_id_right || '')}</span><span>${esc(item.reason || '')}</span><button type="button" class="btn btn-secondary btn-sm mh-more-btn" onclick="dismissMemoryConflict(this, ${Number(item.id)})">${esc(t('dashboard.memory_conflict_both_valid'))}</button><span role="status"></span></div>`, 3) : `<div class="memory-curator-row">${t('dashboard.memory_conflicts_empty')}</div>`) +
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
            const confirmed = await showConfirm(t('dashboard.memory_reflection_run_confirm_title'), t('dashboard.memory_reflection_run_confirm'));
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

        DashSections.register({ id: 'memory', load: loadSectionMemory });
