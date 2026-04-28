        // ── Mood Time Range Buttons ─────────────────────────────────────────────────
        document.querySelector('.mood-mini-btns').addEventListener('click', async (e) => {
            const btn = e.target.closest('.mood-btn');
            if (!btn) return;
            document.querySelectorAll('.mood-btn').forEach(b => b.classList.remove('active'));
            btn.classList.add('active');
            currentMoodHours = parseInt(btn.dataset.hours) || 24;
            const [moodData, emotionData, personality] = await Promise.all([
                API.get('/api/dashboard/mood-history?hours=' + currentMoodHours),
                API.get('/api/dashboard/emotion-history?hours=' + currentMoodHours),
                API.get('/api/personality/state'),
            ]);
            if (Charts.mood) { Charts.mood.destroy(); }
            Charts.mood = createMoodLineChart('mood-chart', moodData || []);
            renderEmotionHistory(emotionData, personality);
        });

        // ── Profile Search ──────────────────────────────────────────────────────────
        document.getElementById('profile-search').addEventListener('input', function () {
            const q = this.value.toLowerCase();
            document.querySelectorAll('.profile-entry').forEach(el => {
                const match = !q || (el.dataset.search || '').toLowerCase().includes(q);
                dashSetHidden(el, !match);
            });
            // Show categories that have visible entries
            document.querySelectorAll('.profile-category').forEach(cat => {
                const entries = cat.querySelectorAll('.profile-entry');
                const hasVisible = Array.from(entries).some(e => !e.classList.contains('is-hidden'));
                dashSetHidden(cat, !hasVisible);
                // Auto-open when searching
                if (q && hasVisible) {
                    const entriesDiv = cat.querySelector('.profile-entries');
                    if (entriesDiv) entriesDiv.classList.remove('is-hidden');
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
                _sseBanner.className = 'dash-sse-banner';
                _sseBanner.classList.add('is-hidden');
                document.body.prepend(_sseBanner);
            }
            _sseBanner.textContent = msg;
            dashSetHidden(_sseBanner, false);
        }
        function hideSSEBanner() {
            dashSetHidden(_sseBanner, true);
        }

        let _dashSSERegistered = false;
        let _sseReconnectTimer = null;
        let _sseReconnectTimeout = null;
        function connectSSE() {
            if (_dashSSERegistered) return;
            _dashSSERegistered = true;
            // Use the shared AuraSSE singleton — no dedicated EventSource needed.
            window.AuraSSE.on('_open', function () {
                // Clear any pending reconnect timers
                if (_sseReconnectTimer) { clearTimeout(_sseReconnectTimer); _sseReconnectTimer = null; }
                if (_sseReconnectTimeout) { clearTimeout(_sseReconnectTimeout); _sseReconnectTimeout = null; }
                hideSSEBanner();
            });
            window.AuraSSE.on('_error', function () {
                // Don't show banner immediately — wait 3 seconds of sustained disconnection
                if (_sseReconnectTimer) return; // Already pending
                _sseReconnectTimer = setTimeout(function () {
                    _sseReconnectTimer = null;
                    showSSEBanner('⚠ ' + (t('dashboard.sse_reconnecting') || 'Reconnecting…'));
                }, 3000);
                // Also check auth status after a longer delay
                if (_sseReconnectTimeout) clearTimeout(_sseReconnectTimeout);
                _sseReconnectTimeout = setTimeout(function () {
                    fetch('/api/auth/status', { credentials: 'same-origin' }).then(function (r) {
                        if (r.status === 401) {
                            window.location.href = '/auth/login?redirect=' + encodeURIComponent(window.location.pathname);
                        }
                    }).catch(function () { });
                }, 8000);
            });
            window.AuraSSE.on('system_metrics', function (sys) {
                updateGauge(Charts.cpu, 'cpu-val', (sys.cpu && sys.cpu.usage_percent) || 0);
                updateGauge(Charts.ram, 'ram-val', (sys.memory && sys.memory.used_percent) || 0);
                updateGauge(Charts.disk, 'disk-val', (sys.disk && sys.disk.used_percent) || 0);
                renderSystemStats(sys);
            });
            window.AuraSSE.on('memory_update', function (mem) {
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
            });
            window.AuraSSE.on('personality_update', function (personality) {
                renderMoodBadge(personality);
                if (TabState.active === 'agent') {
                    API.get('/api/dashboard/emotion-history?hours=' + currentMoodHours).then(data => renderEmotionHistory(data, personality));
                }
            });
            window.AuraSSE.on('daemon_update', function () {
                if (TabState.active === 'system') {
                    loadDaemonsCard();
                }
            });
            window.AuraSSE.onLegacy(function (event) {
                try {
                    const data = JSON.parse(event.data);
                    if (data.event === 'budget_update') {
                        if (data.spent_usd != null) {
                            document.getElementById('budget-spent').textContent = '$' + (data.spent_usd || 0).toFixed(2);
                        }
                    } else if (data.event === 'budget_warning') {
                        if (typeof showToast === 'function') showToast('⚡ ' + (data.detail || t('dashboard.budget_warning')), 'warning', 5000);
                    } else if (data.event === 'budget_blocked') {
                        if (typeof showToast === 'function') showToast('🚫 ' + (data.detail || t('dashboard.budget_blocked')), 'error', 0);
                    }
                } catch (e) { /* ignore non-JSON */ }
            });
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
                await showAlert('Error', '❌ ' + e.message);
            } finally {
                if (saveBtn) saveBtn.disabled = false;
            }
        }

        async function deleteCronJob(id) {
            if (!await showConfirm(t('dashboard.cron_delete_confirm', { id }))) return;
            try {
                const resp = await fetch('/api/cron?id=' + encodeURIComponent(id), {
                    method: 'DELETE',
                    credentials: 'same-origin'
                });
                if (!resp.ok) throw new Error('Delete failed');
                const activity = await API.get('/api/dashboard/activity');
                renderActivity(activity);
            } catch (e) {
                await showAlert('Error', '❌ ' + e.message);
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
                    <div class="cf-add-wrap">
                        <input type="text" class="cf-add-input" id="cfAddInput" placeholder="${t('dashboard.core_facts_modal_add_placeholder')}" onkeydown="if(event.key==='Enter')cfAddFact()">
                        <button class="cf-add-btn" onclick="cfAddFact()">＋</button>
                    </div>
                    <button class="cf-add-btn cf-delete-all-btn" onclick="cfDeleteAllFacts()" ${facts.length === 0 ? 'disabled' : ''}>${t('dashboard.core_facts_modal_delete_all')}</button>
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
                await showAlert('Error', '❌ ' + e.message);
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
            textEl.outerHTML = `<input type="text" class="cf-add-input cf-edit-input" id="cf-edit-${id}" value="${esc(currentText)}" onkeydown="if(event.key==='Enter')cfSaveEdit(${id});if(event.key==='Escape')openCoreFactsModal();">`;
            actionsEl.innerHTML = `
                <button class="cf-fact-btn cf-fact-btn-success" onclick="cfSaveEdit(${id})" title="${t('dashboard.core_facts_modal_save')}">✅</button>
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
                await showAlert('Error', '❌ ' + e.message);
                input.disabled = false;
            }
        }

        async function cfDeleteFact(id) {
            if (!await showConfirm(t('dashboard.core_facts_modal_confirm_delete', {id: id}))) return;
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
                await showAlert('Error', '❌ ' + e.message);
            }
        }

        async function cfDeleteAllFacts() {
            const data = await fetch('/api/dashboard/core-memory', { credentials: 'same-origin' }).then(r => r.json()).catch(() => ({ facts: [] }));
            const count = Array.isArray(data.facts) ? data.facts.length : 0;
            if (count === 0) return;
            const confirmed = await showConfirm(
                t('dashboard.core_facts_modal_confirm_delete_all_title'),
                t('dashboard.core_facts_modal_confirm_delete_all', { count })
            );
            if (!confirmed) return;
            try {
                const resp = await fetch('/api/dashboard/core-memory/mutate', {
                    method: 'DELETE',
                    credentials: 'same-origin',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ all: true, confirm: 'DELETE_ALL_CORE_MEMORY' })
                });
                if (!resp.ok) throw new Error(t('dashboard.core_facts_modal_error_delete_all'));
                await openCoreFactsModal(); // reload
            } catch (e) {
                await showAlert('Error', '❌ ' + e.message);
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

            const kgCloseBtn = document.getElementById('kgDetailClose');
            const kgOverlay = document.getElementById('kgDetailOverlay');
            if (kgCloseBtn) kgCloseBtn.addEventListener('click', closeKGDetailModal);
            if (kgOverlay) kgOverlay.addEventListener('click', (e) => {
                if (e.target === kgOverlay) closeKGDetailModal();
            });

            document.addEventListener('keydown', (e) => {
                if (e.key === 'Escape') {
                    if (kgOverlay && kgOverlay.classList.contains('open')) { closeKGDetailModal(); return; }
                    if (cfOverlay && cfOverlay.classList.contains('open')) closeCoreFactsModal();
                }
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
