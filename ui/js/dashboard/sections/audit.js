// AuraGo dashboard – section "audit": audit log (a section load starts at the first page).
// Classic script, loaded with defer in sidebar order after widgets-knowledge.js and before dashboard-events.js.
// Its declarations stay global: inline onclick strings and other dashboard scripts call them by name. The last
// statement registers the section loader with DashSections (dash-sections.js); main.js#loadSectionContent runs it.

        // ═══ Audit Log ══════════════════════════════════════════════════════════════
        let auditOffset = 0;

        let auditTotal = 0;

        const AUDIT_PAGE_SIZE = 25;

        let auditSearchTimer = null;

        let auditRefreshTimer = null;

        const auditRequests = createLatestRequestGate();

        async function loadTabAudit() {
            await loadAuditPage(0);
        }

        function scheduleAuditRefresh() {
            if (typeof DashRouter === 'undefined' || DashRouter.current() !== 'audit') return;
            clearTimeout(auditRefreshTimer);
            auditRefreshTimer = setTimeout(() => {
                auditRefreshTimer = null;
                loadTabAudit();
            }, 200);
        }

        window.scheduleAuditRefresh = scheduleAuditRefresh;

        async function loadAuditPage(offset) {
            const requestedOffset = Math.max(0, offset || 0);
            const request = auditRequests.begin();
            CardState.setLoading('card-audit-log');
            const params = auditQueryParams();
            params.set('limit', String(AUDIT_PAGE_SIZE));
            params.set('offset', String(requestedOffset));
            try {
                const resp = await fetch('/api/dashboard/audit?' + params.toString(), { credentials: 'same-origin', signal: request.signal });
                if (!resp.ok) throw new Error('Audit load failed');
                const page = await resp.json();
                if (!request.isCurrent()) return;
                // Commit the offset only with the page it belongs to (renderAuditEvents reads it).
                auditOffset = requestedOffset;
                renderAuditEvents(page);
                CardState.setLoaded('card-audit-log');
            } catch (e) {
                if (!request.isCurrent() || (e && e.name === 'AbortError')) return;
                console.warn('Audit load failed', e);
                CardState.setError('card-audit-log', () => loadAuditPage(requestedOffset), { status: 0 });
                if (typeof showToast === 'function') showToast(t('dashboard.audit_error_load'), 'error', 5000);
            } finally {
                request.finish();
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
                    <td class="audit-cell-time" data-label="${escAttr(t('dashboard.audit_col_time'))}">${esc(timeLabel)}</td>
                    <td class="audit-cell-source" data-label="${escAttr(t('dashboard.audit_col_source'))}"><span class="audit-source audit-source-${escAttr(event.source || 'system')}">${esc(auditSourceLabel(event.source))}</span></td>
                    <td class="audit-cell-type" data-label="${escAttr(t('dashboard.audit_col_type'))}">${esc(auditTypeLabel(event.event_type))}</td>
                    <td class="audit-cell-target" data-label="${escAttr(t('dashboard.audit_col_target'))}" title="${escAttr(event.target_id || '')}">${esc(event.target_name || event.target_id || '—')}</td>
                    <td class="audit-cell-status" data-label="${escAttr(t('dashboard.audit_col_status'))}"><span class="audit-status audit-status-${escAttr(status)}">${esc(auditStatusLabel(status))}</span></td>
                    <td class="audit-cell-summary" data-label="${escAttr(t('dashboard.audit_col_summary'))}" title="${escAttr(detail)}"><span class="audit-summary-text">${esc(event.summary || '—')}</span></td>
                    <td class="audit-cell-duration" data-label="${escAttr(t('dashboard.audit_col_duration'))}">${esc(duration)}</td>
                    <td class="audit-cell-actions" data-label="${escAttr(t('dashboard.audit_col_actions'))}"><button type="button" class="audit-row-delete" onclick="deleteAuditEvent(${Number(event.id || 0)})" title="${escAttr(t('dashboard.audit_delete'))}">${esc(t('dashboard.btn_delete'))}</button></td>`;
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

        DashSections.register({ id: 'audit', load: () => loadTabAudit() });
