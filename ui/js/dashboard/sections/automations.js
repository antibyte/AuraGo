// AuraGo dashboard – section "automations": cron jobs, mission history and daemon skills.
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        async function loadSectionAutomations() {
            CardState.setLoading('card-mission-history');
            await Promise.all([loadTabCronjobs(), loadMissionHistory(), loadDaemonsCard()]);
        }

        // ═══ Cronjobs ═══════════════════════════════════════════════════════════════
        let cronjobsSearchTimer = null;

        const cronjobsRequests = createLatestRequestGate();

        async function loadTabCronjobs() {
            const request = cronjobsRequests.begin();
            CardState.setLoading('card-cronjobs');
            const params = cronjobsQueryParams();
            try {
                const resp = await fetch('/api/dashboard/cronjobs?' + params.toString(), { credentials: 'same-origin', signal: request.signal });
                if (!resp.ok) throw new Error('Cronjobs load failed');
                const data = await resp.json();
                if (!request.isCurrent()) return;
                renderCronjobs(data);
                CardState.setLoaded('card-cronjobs');
            } catch (e) {
                if (!request.isCurrent() || (e && e.name === 'AbortError')) return;
                console.warn('Cronjobs load failed', e);
                CardState.setError('card-cronjobs', loadTabCronjobs, { status: 0 });
                if (typeof showToast === 'function') showToast(t('dashboard.cronjobs_error_load'), 'error', 5000);
            } finally {
                request.finish();
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
                // A flow's schedule is managed in EasyDrag: the server refuses edit, toggle and delete (409).
                const actions = job.managed_by === 'easydrag' ? cronjobManagedLabel() : `<button type="button" class="cronjobs-row-btn"
                            data-cron-id="${escAttr(job.id || '')}"
                            data-cron-expr="${escAttr(job.cron_expr || '')}"
                            data-cron-prompt="${escAttr(job.task_prompt || '')}"
                            data-cron-source="${escAttr(job.source || 'agent')}"
                            data-cron-disabled="${disabled ? 'true' : 'false'}"
                            onclick="openCronEditModal(this)"
                            title="${escAttr(t('dashboard.cron_edit_title'))}">${esc(t('dashboard.btn_edit'))}</button>
                        <button type="button" class="cronjobs-row-btn danger"
                            data-cron-id="${escAttr(job.id || '')}"
                            onclick="deleteCronJob(this.dataset.cronId)"
                            title="${escAttr(t('dashboard.cron_btn_delete'))}">${esc(t('dashboard.btn_delete'))}</button>`;
                tr.innerHTML = `
                    <td data-label="${escAttr(t('dashboard.cronjobs_col_id'))}"><span class="cronjobs-id">${esc(job.id || '—')}</span></td>
                    <td data-label="${escAttr(t('dashboard.cronjobs_col_source'))}"><span class="cronjobs-source cronjobs-source-${escAttr(job.source || 'agent')}">${esc(cronjobSourceLabel(job.source))}</span></td>
                    <td data-label="${escAttr(t('dashboard.cronjobs_col_schedule'))}"><code class="cronjobs-expr">${esc(job.cron_expr || '—')}</code></td>
                    <td data-label="${escAttr(t('dashboard.cronjobs_col_next_run'))}">${esc(nextRun)}</td>
                    <td data-label="${escAttr(t('dashboard.cronjobs_col_status'))}"><span class="cronjobs-status cronjobs-status-${status}" title="${escAttr(statusTitle)}">${esc(cronjobStatusLabel(status))}</span></td>
                    <td data-label="${escAttr(t('dashboard.cronjobs_col_prompt'))}" title="${escAttr(promptTitle)}">${esc(truncate(job.task_prompt || '', 120) || '—')}</td>
                    <td data-label="${escAttr(t('dashboard.cronjobs_col_actions'))}">
                        <div class="cronjobs-row-actions">
                        ${actions}
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
                    <td title="${escAttr(e.mission_id)}">${esc(e.mission_name || e.mission_id)}</td>
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
                    ? `<div class="daemon-meta daemon-meta-error"><span title="${escAttr(d.last_error)}">⚠ ${esc(d.last_error.length > 60 ? d.last_error.substring(0, 60) + '…' : d.last_error)}</span></div>`
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

        DashSections.register({ id: 'automations', load: loadSectionAutomations });
