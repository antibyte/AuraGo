/* AuraGo – Knowledge Center: Appointments */
/* global t, esc, showToast, closeModal */

// ═══════════════════════════════════════════════════════════════
// STATE
// ═══════════════════════════════════════════════════════════════
let allAppointments = [];
let appointmentSearchTimer = null;
let selectedParticipantIds = []; // { id, name, email }

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
                    ${a.status === 'upcoming' || a.status === 'overdue' ? `
                        <button class="btn btn-sm btn-secondary" onclick="completeAppointment('${esc(a.id)}')" title="${t('knowledge.appointments_complete')}">✅</button>
                        <button class="btn btn-sm btn-secondary" onclick="cancelAppointment('${esc(a.id)}')" title="${t('knowledge.appointments_cancel')}">❌</button>
                    ` : ''}
                    <button class="btn btn-sm btn-secondary" onclick="editAppointment('${esc(a.id)}')" title="${t('common.btn_edit')}">✏️</button>
                    <button class="btn btn-sm btn-danger" onclick="askDeleteAppointment('${esc(a.id)}', '${esc(a.title)}')" title="${t('common.btn_delete')}">🗑️</button>
                </div>
            </div>
            ${a.description ? `<p class="kc-appointment-desc">${esc(a.description)}</p>` : ''}
            ${a.participants && a.participants.length ? `<div class="kc-appointment-participants">${renderAppointmentParticipantsInline(a.participants)}</div>` : ''}
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

    // Initialize participants
    selectedParticipantIds = [];
    if (appointment && appointment.participants) {
        appointment.participants.forEach(p => {
            selectedParticipantIds.push({ id: p.id, name: p.name, email: p.email || '' });
        });
    }
    renderParticipantChips();
    document.getElementById('appointment-participant-search').value = '';
    hideParticipantDropdown();

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
        contact_ids: selectedParticipantIds.map(p => p.id),
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
    // ISSUE-11: Send only the status field to avoid overwriting server-side changes
    // with stale cached data from the frontend.
    try {
        const resp = await fetch('/api/appointments/' + encodeURIComponent(id), {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ status }),
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
        case 'overdue': return 'kc-status-overdue';
        default: return 'kc-status-upcoming';
    }
}

function appointmentStatusLabel(status) {
    switch (status) {
        case 'completed': return t('knowledge.appointments_status_completed');
        case 'cancelled': return t('knowledge.appointments_status_cancelled');
        case 'overdue': return t('knowledge.appointments_status_overdue');
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

// ═══════════════════════════════════════════════════════════════
// PARTICIPANTS
// ═══════════════════════════════════════════════════════════════

function renderParticipantChips() {
    const container = document.getElementById('appointment-participant-chips');
    if (!container) return;
    if (selectedParticipantIds.length === 0) {
        container.innerHTML = '<span style="color:var(--text-secondary);font-size:0.82rem;line-height:1.6">' + esc(t('knowledge.appointments_participants_empty') || 'No participants added') + '</span>';
        return;
    }
    container.innerHTML = selectedParticipantIds.map(p => {
        const initial = p.name ? p.name.charAt(0).toUpperCase() : '?';
        return `<span class="kc-chip">
            <span class="kc-chip-avatar">${esc(initial)}</span>
            <span>${esc(p.name)}</span>
            <button class="kc-chip-remove" onclick="removeParticipantChip('${esc(p.id)}')" title="${esc(t('common.btn_remove') || 'Remove')}">&times;</button>
        </span>`;
    }).join('');
}

function renderAppointmentParticipantsInline(participants) {
    if (!participants || participants.length === 0) return '';
    const maxVisible = 3;
    const visible = participants.slice(0, maxVisible);
    const names = visible.map(p => esc(p.name)).join(', ');
    if (participants.length <= maxVisible) {
        return `<span class="kc-appointment-participants-inline">👥 ${names}</span>`;
    }
    const remaining = participants.length - maxVisible;
    return `<span class="kc-appointment-participants-inline">👥 ${names} +${remaining}</span>`;
}

function removeParticipantChip(id) {
    selectedParticipantIds = selectedParticipantIds.filter(p => p.id !== id);
    renderParticipantChips();
}

let participantSearchTimer = null;

function onParticipantSearchInput() {
    clearTimeout(participantSearchTimer);
    participantSearchTimer = setTimeout(filterParticipantSuggestions, 150);
}

async function filterParticipantSuggestions() {
    const input = document.getElementById('appointment-participant-search');
    const dropdown = document.getElementById('appointment-participant-dropdown');
    if (!input || !dropdown) return;

    const q = (input.value || '').trim().toLowerCase();

    if (q === '') {
        hideParticipantDropdown();
        return;
    }

    // Load contacts if not already loaded
    if (typeof allContacts === 'undefined' || !allContacts || allContacts.length === 0) {
        try {
            const r = await fetch('/api/contacts');
            if (r.ok) allContacts = await r.json();
        } catch { allContacts = []; }
    }

    const selectedIds = new Set(selectedParticipantIds.map(p => p.id));
    const matches = (allContacts || []).filter(c => {
        if (selectedIds.has(c.id)) return false;
        const name = (c.name || '').toLowerCase();
        const email = (c.email || '').toLowerCase();
        const phone = (c.phone || '').toLowerCase();
        return name.includes(q) || email.includes(q) || phone.includes(q);
    }).slice(0, 8);

    if (matches.length === 0) {
        dropdown.innerHTML = `<div class="kc-picker-empty">${esc(t('knowledge.contacts_empty_title') || 'No contacts found')}</div>`;
    } else {
        dropdown.innerHTML = matches.map(c => {
            const initial = c.name ? c.name.charAt(0).toUpperCase() : '?';
            const meta = [c.email, c.relationship].filter(Boolean).join(' · ');
            return `<div class="kc-picker-item" onclick="addParticipantFromPicker('${esc(c.id)}', '${esc(c.name)}', '${esc(c.email || '')}')">
                <div class="kc-picker-item-avatar">${esc(initial)}</div>
                <div class="kc-picker-item-info">
                    <div class="kc-picker-item-name">${esc(c.name)}</div>
                    ${meta ? `<div class="kc-picker-item-meta">${esc(meta)}</div>` : ''}
                </div>
            </div>`;
        }).join('');
    }
    dropdown.classList.remove('is-hidden');
}

function addParticipantFromPicker(id, name, email) {
    if (selectedParticipantIds.some(p => p.id === id)) return;
    selectedParticipantIds.push({ id, name, email });
    renderParticipantChips();
    const input = document.getElementById('appointment-participant-search');
    if (input) input.value = '';
    hideParticipantDropdown();
}

function hideParticipantDropdown() {
    const dropdown = document.getElementById('appointment-participant-dropdown');
    if (dropdown) dropdown.classList.add('is-hidden');
}

// Close dropdown when clicking outside
document.addEventListener('click', function(e) {
    const section = document.getElementById('appointment-participants-section');
    if (section && !section.contains(e.target)) {
        hideParticipantDropdown();
    }
});

// ═══════════════════════════════════════════════════════════════
// INIT
// ═══════════════════════════════════════════════════════════════

document.addEventListener('DOMContentLoaded', function() {
    const searchInput = document.getElementById('appointment-participant-search');
    if (searchInput) {
        searchInput.addEventListener('input', onParticipantSearchInput);
        searchInput.addEventListener('focus', onParticipantSearchInput);
    }
});
