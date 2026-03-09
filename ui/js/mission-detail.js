// AuraGo – mission page logic
// Extracted from mission.html

        // ══════════════════════════════════════════════════════════════════════════════
        // Mission Control – JavaScript
        // ══════════════════════════════════════════════════════════════════════════════

        // ── i18n (centralized) ────────────────────────────────────────────────────
        function applyTranslations() {
            document.querySelectorAll('[data-i18n]').forEach(el => {
                const key = el.getAttribute('data-i18n');
                if (key) el.innerText = t(key);
            });
            // Update placeholders
            const nameInput = document.getElementById('missionName');
            if (nameInput) nameInput.placeholder = t('mission.placeholder_name');
            const promptInput = document.getElementById('missionPrompt');
            if (promptInput) promptInput.placeholder = t('mission.placeholder_prompt');
            // Page title
            document.title = t('mission.page_title');
        }

        // ── State ────────────────────────────────────────────────────────────────────
        let missions = [];
        let editingId = null;
        let deleteId = null;
        let scheduleType = 'manual';

        // ── API ──────────────────────────────────────────────────────────────────────
        const API = {
            async list() {
                const r = await fetch('/api/missions');
                if (!r.ok) return [];
                return r.json();
            },
            async create(data) {
                const r = await fetch('/api/missions', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data),
                });
                return r.ok;
            },
            async update(id, data) {
                const r = await fetch('/api/missions/' + id, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data),
                });
                return r.ok;
            },
            async remove(id) {
                const r = await fetch('/api/missions/' + id, { method: 'DELETE' });
                return r.ok;
            },
            async run(id) {
                const r = await fetch('/api/missions/' + id + '/run', { method: 'POST' });
                return r.ok;
            },
        };

        // ── Mission-specific Toast (uses shared showToast if available) ─────────────
        function showMissionToast(msg) {
            if (typeof showToast === 'function') {
                showToast(msg, 'success', 2500);
            } else {
                // Fallback for mission-specific toast
                const el = document.getElementById('mcToast');
                if (el) {
                    el.textContent = msg;
                    el.classList.add('show');
                    setTimeout(() => el.classList.remove('show'), 2500);
                }
            }
        }

        // ── Render ───────────────────────────────────────────────────────────────────
        function timeAgoMission(dateStr) {
            if (!dateStr) return t('mission.lbl_never_run');
            const d = new Date(dateStr);
            if (isNaN(d.getTime()) || d.getFullYear() < 2000) return t('mission.lbl_never_run');
            const secs = Math.floor((Date.now() - d.getTime()) / 1000);
            if (secs < 60) return t('mission.lbl_just_now');
            if (secs < 3600) return Math.floor(secs / 60) + t('mission.lbl_ago_m');
            if (secs < 86400) return Math.floor(secs / 3600) + t('mission.lbl_ago_h');
            return Math.floor(secs / 86400) + t('mission.lbl_ago_d');
        }

        function renderStats() {
            document.getElementById('stat-total').textContent = missions.length;
            document.getElementById('stat-scheduled').textContent = missions.filter(m => m.schedule).length;
            document.getElementById('stat-runs').textContent = missions.reduce((s, m) => s + (m.run_count || 0), 0);
            document.getElementById('stat-success').textContent = missions.filter(m => m.last_result === 'success').length;
        }

        function renderMissions() {
            const list = document.getElementById('missionList');
            const empty = document.getElementById('emptyState');

            if (missions.length === 0) {
                list.innerHTML = '';
                empty.style.display = 'block';
                return;
            }

            empty.style.display = 'none';

            // Sort: high priority first, then by creation date (newest first)
            const prioOrder = { high: 0, medium: 1, low: 2 };
            const sorted = [...missions].sort((a, b) => {
                const pa = prioOrder[a.priority] ?? 1;
                const pb = prioOrder[b.priority] ?? 1;
                if (pa !== pb) return pa - pb;
                return new Date(b.created_at) - new Date(a.created_at);
            });

            list.innerHTML = sorted.map(m => {
                const scheduleBadge = m.schedule
                    ? `<span class="badge badge-schedule">⏰ ${escHtml(m.schedule)}</span>`
                    : `<span class="badge badge-manual">${t('mission.badge_manual')}</span>`;
                const prioBadge = `<span class="badge badge-${m.priority}">${m.priority}</span>`;
                const resultBadge = m.last_result
                    ? `<span class="badge badge-${m.last_result}">${m.last_result === 'success' ? t('mission.badge_result_success') : t('mission.badge_result_error')}</span>`
                    : '';
                const runsBadge = m.run_count > 0
                    ? `<span class="badge badge-runs">${m.run_count} ${m.run_count !== 1 ? t('mission.badge_run_plural') : t('mission.badge_run_singular')}</span>`
                    : '';
                const disabledClass = m.enabled ? '' : ' disabled';
                const pauseIcon = m.enabled ? '⏸' : '▶️';
                const pauseTitle = m.enabled ? t('mission.title_pause') : t('mission.title_resume');

                return `
        <div class="mission-card${disabledClass}" data-id="${m.id}">
            <div class="mission-card-top">
                <div class="mission-meta">
                    <h3 class="mission-name">
                        ${escHtml(m.name)}
                        ${m.locked ? `<span style="font-size:0.9rem;" title="${t('mission.title_locked_no_del')}">🔒</span>` : ''}
                        ${!m.enabled ? `<span style="opacity:0.5;font-size:var(--text-xs);">${t('mission.lbl_paused')}</span>` : ''}
                    </h3>
                    <div class="mission-prompt">${escHtml(m.prompt)}</div>
                    <div class="mission-badges">
                        ${scheduleBadge} ${prioBadge} ${resultBadge} ${runsBadge}
                    </div>
                </div>
            </div>
            <div class="mission-card-bottom">
                <div class="mission-last-run">
                    ${m.last_run && new Date(m.last_run).getFullYear() > 2000
                        ? t('mission.lbl_last_run') + timeAgoMission(m.last_run)
                        : t('mission.lbl_never_run')}
                </div>
                <div class="mission-actions">
                    <button class="btn-action run-btn" onclick="runMission('${m.id}')" title="${t('mission.title_run')}">▶</button>
                    <button class="btn-action" onclick="toggleMission('${m.id}', ${!m.enabled})" title="${pauseTitle}">${pauseIcon}</button>
                    <button class="btn-action" onclick="openEditModal('${m.id}')" title="${t('mission.title_edit')}">✏️</button>
                    ${m.locked
                        ? `<button class="btn-action" onclick="toggleLock('${m.id}', false)" title="${t('mission.title_unlock')}">🔓</button>
                           <button class="btn-action danger disabled" style="opacity:0.3;cursor:not-allowed;" title="${t('mission.title_locked_no_del')}">🗑</button>`
                        : `<button class="btn-action" onclick="toggleLock('${m.id}', true)" title="${t('mission.title_lock')}">🔒</button>
                           <button class="btn-action danger" onclick="openDeleteModal('${m.id}')" title="${t('mission.title_delete')}">🗑</button>`
                    }
                </div>
            </div>
        </div>`;
            }).join('');
        }

        function escHtml(s) {
            return (s || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
        }

        // ── Modal Logic ──────────────────────────────────────────────────────────────
        function setScheduleType(type) {
            scheduleType = type;
            document.querySelectorAll('.schedule-type-btn').forEach(btn => {
                btn.classList.toggle('active', btn.dataset.type === type);
            });
            document.getElementById('cronGroup').style.display = type === 'cron' ? 'block' : 'none';
        }

        function applyCronPreset() {
            const presetSelect = document.getElementById('cronPreset');
            const cronInput = document.getElementById('missionCron');
            if (presetSelect.value) {
                cronInput.value = presetSelect.value;
            }
        }

        function matchCronPreset(expr) {
            const presetSelect = document.getElementById('cronPreset');
            presetSelect.value = ""; // Default to custom
            for (let i = 0; i < presetSelect.options.length; i++) {
                if (presetSelect.options[i].value === expr) {
                    presetSelect.selectedIndex = i;
                    break;
                }
            }
        }

        function openCreateModal() {
            editingId = null;
            document.getElementById('modalTitle').innerHTML = `🚀 <span>${t('mission.modal_title_new')}</span>`;
            document.getElementById('btnSave').textContent = t('mission.btn_save_mission');
            document.getElementById('missionName').value = '';
            document.getElementById('missionPrompt').value = '';
            document.getElementById('missionCron').value = '';
            document.getElementById('cronPreset').value = '';
            document.getElementById('missionPriority').value = 'medium';
            document.getElementById('missionEnabled').value = 'true';
            setScheduleType('manual');
            document.getElementById('missionModal').classList.add('active');
            document.body.style.overflow = 'hidden';
            document.getElementById('missionName').focus();
        }

        function openEditModal(id) {
            const m = missions.find(x => x.id === id);
            if (!m) return;
            editingId = id;
            document.getElementById('modalTitle').innerHTML = `✏️ <span>${t('mission.modal_title_edit')}</span>`;
            document.getElementById('btnSave').textContent = t('mission.btn_save_changes');
            document.getElementById('missionName').value = m.name;
            document.getElementById('missionPrompt').value = m.prompt;
            document.getElementById('missionCron').value = m.schedule || '';
            matchCronPreset(m.schedule || '');
            document.getElementById('missionPriority').value = m.priority || 'medium';
            document.getElementById('missionEnabled').value = m.enabled ? 'true' : 'false';
            setScheduleType(m.schedule ? 'cron' : 'manual');
            document.getElementById('missionModal').classList.add('active');
            document.body.style.overflow = 'hidden';
            document.getElementById('missionName').focus();
        }

        function closeMissionModal() {
            document.getElementById('missionModal').classList.remove('active');
            document.body.style.overflow = '';
            editingId = null;
        }

        async function saveMission() {
            const name = document.getElementById('missionName').value.trim();
            const prompt = document.getElementById('missionPrompt').value.trim();
            const cron = scheduleType === 'cron' ? document.getElementById('missionCron').value.trim() : '';
            const priority = document.getElementById('missionPriority').value;
            const enabled = document.getElementById('missionEnabled').value === 'true';

            if (!name || !prompt) {
                showMissionToast(t('mission.toast_name_req'));
                return;
            }
            if (scheduleType === 'cron' && !cron) {
                showMissionToast(t('mission.toast_cron_req'));
                return;
            }

            const data = { name, prompt, schedule: cron, priority, enabled };

            let ok;
            if (editingId) {
                ok = await API.update(editingId, data);
                if (ok) showMissionToast(t('mission.toast_mission_upd'));
            } else {
                ok = await API.create(data);
                if (ok) showMissionToast(t('mission.toast_mission_add'));
            }

            if (ok) {
                closeMissionModal();
                await refresh();
            } else {
                showMissionToast(t('mission.toast_failed_save'));
            }
        }

        // ── Delete ───────────────────────────────────────────────────────────────────
        function openDeleteModal(id) {
            const m = missions.find(x => x.id === id);
            if (!m) return;
            deleteId = id;
            document.getElementById('deleteModal').classList.add('active');
            document.body.style.overflow = 'hidden';
        }

        function closeDeleteModal() {
            document.getElementById('deleteModal').classList.remove('active');
            document.body.style.overflow = '';
            deleteId = null;
        }

        async function confirmDelete() {
            if (!deleteId) return;
            const ok = await API.remove(deleteId);
            closeDeleteModal();
            if (ok) {
                showMissionToast(t('mission.toast_mission_del'));
                await refresh();
            } else {
                showMissionToast(t('mission.toast_failed_del'));
            }
        }

        // ── Actions ──────────────────────────────────────────────────────────────────
        async function runMission(id) {
            const ok = await API.run(id);
            if (ok) {
                showMissionToast(t('mission.toast_mission_start'));
            } else {
                showMissionToast(t('mission.toast_failed_start'));
            }
        }

        async function toggleMission(id, enabled) {
            const m = missions.find(x => x.id === id);
            if (!m) return;
            const data = { ...m, enabled };
            const ok = await API.update(id, data);
            if (ok) {
                showMissionToast(enabled ? t('mission.toast_mission_res') : t('mission.toast_mission_pau'));
                await refresh();
            }
        }

        async function toggleLock(id, locked) {
            const m = missions.find(x => x.id === id);
            if (!m) return;
            const data = { ...m, locked };
            const ok = await API.update(id, data);
            if (ok) {
                showMissionToast(locked ? t('mission.toast_mission_lck') : t('mission.toast_mission_ulck'));
                await refresh();
            }
        }

        // ── Refresh ──────────────────────────────────────────────────────────────────
        async function refresh() {
            missions = await API.list();
            renderStats();
            renderMissions();
        }

        // ── Close modal on Esc or outside click ─────────────────────────────────────
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape') {
                closeMissionModal();
                closeDeleteModal();
            }
        });
        document.getElementById('missionModal').addEventListener('click', (e) => {
            if (e.target === e.currentTarget) closeMissionModal();
        });
        document.getElementById('deleteModal').addEventListener('click', (e) => {
            if (e.target === e.currentTarget) closeDeleteModal();
        });

        // ── Init ─────────────────────────────────────────────────────────────────────
        applyTranslations();
        refresh();
        // Auto-refresh every 30 seconds
        setInterval(refresh, 30000);
