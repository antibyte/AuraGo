/* AuraGo – Knowledge Center: Appointments */
/* global t, esc, showToast, closeModal */

// ═══════════════════════════════════════════════════════════════
// STATE
// ═══════════════════════════════════════════════════════════════
let allAppointments = [];
let appointmentSearchTimer = null;

// ═══════════════════════════════════════════════════════════════
// LOAD & RENDER
// ═══════════════════════════════════════════════════════════════

function debounceAppointmentSearch() {
    clearTimeout(appointmentSearchTimer);
    appointmentSearchTimer = setTimeout(() => loadAppointments(), 300);
}

async function loadAppointments() {
    const q = (document.getElementById('appointments-search')?.value || '').trim();
    const status = document.getElementById('appointments-filter')?.value || '';
    let url = '/api/appointments?';
    if (q) url += 'q=' + encodeURIComponent(q) + '&';
    if (status) url += 'status=' + encodeURIComponent(status);
    try {
        const r = await fetch(url);
        if (!r.ok) throw new Error(r.status + ' ' + r.statusText);
        const resp = await r.json();
        allAppointments = resp || [];
        renderAppointments();
    } catch (e) {
        console.error('Failed to load appointments:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function renderAppointments() {
    const list = document.getElementById('appointments-list');
    const empty = document.getElementById('appointments-empty');

    if (!allAppointments.length) {
        list.innerHTML = '';
        list.classList.add('is-hidden');
        empty.classList.remove('is-hidden');
        return;
    }
    list.classList.remove('is-hidden');
    empty.classList.add('is-hidden');

    list.innerHTML = allAppointments.map(a => {
        const statusClass = appointmentStatusClass(a.status);
        const statusLabel = appointmentStatusLabel(a.status);
        const dt = formatAppointmentDate(a.date_time);
        const notify = a.notification_at ? formatAppointmentDate(a.notification_at) : '';
        const isPast = new Date(a.date_time) < new Date();

        return `
        <div class="kc-appointment-card ${isPast && a.status === 'upcoming' ? 'kc-appointment-past' : ''}">
            <div class="kc-appointment-header">
                <div class="kc-appointment-title-row">
                    <h3 class="kc-appointment-title">${esc(a.title)}</h3>
                    <span class="kc-status-pill ${statusClass}">${statusLabel}</span>
                </div>
                <div class="kc-appointment-actions">
                    ${a.status === 'upcoming' ? `
                        <button class="btn btn-sm btn-secondary" onclick="completeAppointment('${esc(a.id)}')" title="${t('knowledge.appointments_complete')}">✅</button>
                        <button class="btn btn-sm btn-secondary" onclick="cancelAppointment('${esc(a.id)}')" title="${t('knowledge.appointments_cancel')}">❌</button>
                    ` : ''}
                    <button class="btn btn-sm btn-secondary" onclick="editAppointment('${esc(a.id)}')" title="${t('common.btn_edit')}">✏️</button>
                    <button class="btn btn-sm btn-danger" onclick="askDeleteAppointment('${esc(a.id)}', '${esc(a.title)}')" title="${t('common.btn_delete')}">🗑️</button>
                </div>
            </div>
            ${a.description ? `<p class="kc-appointment-desc">${esc(a.description)}</p>` : ''}
            <div class="kc-appointment-meta">
                <span class="kc-appointment-meta-item">📅 ${dt}</span>
                ${notify ? `<span class="kc-appointment-meta-item">🔔 ${notify}</span>` : ''}
                ${a.wake_agent ? `<span class="kc-appointment-meta-item kc-wake-badge">🤖 ${t('knowledge.appointments_agent_wake')}</span>` : ''}
            </div>
        </div>`;
    }).join('');
}

// ═══════════════════════════════════════════════════════════════
// MODAL
// ═══════════════════════════════════════════════════════════════

function openAppointmentModal(appointment) {
    const modal = document.getElementById('appointment-modal');
    const title = document.getElementById('appointment-modal-title');

    document.getElementById('appointment-id').value = appointment ? appointment.id : '';
    document.getElementById('appointment-title').value = appointment ? appointment.title : '';
    document.getElementById('appointment-description').value = appointment ? appointment.description || '' : '';
    document.getElementById('appointment-datetime').value = appointment ? toLocalInput(appointment.date_time) : '';
    document.getElementById('appointment-notification').value = appointment && appointment.notification_at ? toLocalInput(appointment.notification_at) : '';
    document.getElementById('appointment-wake-agent').checked = appointment ? appointment.wake_agent : false;
    document.getElementById('appointment-instruction').value = appointment ? appointment.agent_instruction || '' : '';

    toggleAgentInstruction();

    title.textContent = appointment ? t('knowledge.appointments_edit') : t('knowledge.appointments_add');
    modal.classList.add('active');
}

function toggleAgentInstruction() {
    const checked = document.getElementById('appointment-wake-agent').checked;
    const group = document.getElementById('agent-instruction-group');
    if (checked) {
        group.classList.remove('is-hidden');
    } else {
        group.classList.add('is-hidden');
    }
}

function editAppointment(id) {
    const a = allAppointments.find(x => x.id === id);
    if (a) openAppointmentModal(a);
}

async function saveAppointment() {
    const id = document.getElementById('appointment-id').value;
    const data = {
        title: document.getElementById('appointment-title').value.trim(),
        description: document.getElementById('appointment-description').value.trim(),
        date_time: fromLocalInput(document.getElementById('appointment-datetime').value),
        notification_at: fromLocalInput(document.getElementById('appointment-notification').value),
        wake_agent: document.getElementById('appointment-wake-agent').checked,
        agent_instruction: document.getElementById('appointment-instruction').value.trim(),
    };

    if (!data.title) {
        showToast(t('knowledge.appointments_title_required'), 'error');
        return;
    }
    if (!data.date_time) {
        showToast(t('knowledge.appointments_datetime_required'), 'error');
        return;
    }

    try {
        let resp;
        if (id) {
            data.id = id;
            resp = await fetch('/api/appointments/' + encodeURIComponent(id), {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        } else {
            resp = await fetch('/api/appointments', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        }

        if (!resp.ok) {
            const err = await resp.text();
            throw new Error(err);
        }

        closeModal('appointment-modal');
        showToast(t('common.success'), 'success');
        loadAppointments();
    } catch (e) {
        console.error('Save appointment failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

// ═══════════════════════════════════════════════════════════════
// STATUS ACTIONS
// ═══════════════════════════════════════════════════════════════

async function completeAppointment(id) {
    await updateAppointmentStatus(id, 'completed');
}

async function cancelAppointment(id) {
    await updateAppointmentStatus(id, 'cancelled');
}

async function updateAppointmentStatus(id, status) {
    const a = allAppointments.find(x => x.id === id);
    if (!a) return;

    const data = Object.assign({}, a, { status });

    try {
        const resp = await fetch('/api/appointments/' + encodeURIComponent(id), {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data),
        });
        if (!resp.ok) throw new Error(await resp.text());
        showToast(t('common.success'), 'success');
        loadAppointments();
    } catch (e) {
        console.error('Update appointment status failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function askDeleteAppointment(id, title) {
    document.getElementById('delete-target-id').value = id;
    document.getElementById('delete-target-type').value = 'appointment';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.appointments_delete_confirm').replace('{name}', title);
    document.getElementById('delete-modal').classList.add('active');
}

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

function appointmentStatusClass(status) {
    switch (status) {
        case 'completed': return 'kc-status-completed';
        case 'cancelled': return 'kc-status-cancelled';
        default: return 'kc-status-upcoming';
    }
}

function appointmentStatusLabel(status) {
    switch (status) {
        case 'completed': return t('knowledge.appointments_status_completed');
        case 'cancelled': return t('knowledge.appointments_status_cancelled');
        default: return t('knowledge.appointments_status_upcoming');
    }
}

function formatAppointmentDate(iso) {
    if (!iso) return '';
    try {
        const d = new Date(iso);
        return d.toLocaleDateString(undefined, { weekday: 'short', year: 'numeric', month: 'short', day: 'numeric' })
            + ' ' + d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
    } catch { return iso; }
}

function toLocalInput(iso) {
    if (!iso) return '';
    try {
        const d = new Date(iso);
        const pad = n => String(n).padStart(2, '0');
        return d.getFullYear() + '-' + pad(d.getMonth() + 1) + '-' + pad(d.getDate())
            + 'T' + pad(d.getHours()) + ':' + pad(d.getMinutes());
    } catch { return ''; }
}

function fromLocalInput(val) {
    if (!val) return '';
    try {
        return new Date(val).toISOString();
    } catch { return ''; }
}
