/* AuraGo – Knowledge Center JavaScript */
/* global t, showToast, closeModal */

// ═══════════════════════════════════════════════════════════════
// STATE
// ═══════════════════════════════════════════════════════════════
let allContacts = [];
let allFiles = [];
let allDevices = [];
let contactSearchTimer = null;

// ═══════════════════════════════════════════════════════════════
// INIT
// ═══════════════════════════════════════════════════════════════
document.addEventListener('DOMContentLoaded', () => {
    loadContacts();
    loadFiles();
    loadDevices();
});

// ═══════════════════════════════════════════════════════════════
// TAB SWITCHING
// ═══════════════════════════════════════════════════════════════
function switchKCTab(tab) {
    document.querySelectorAll('.kc-tab').forEach(t => {
        t.classList.remove('active');
        t.setAttribute('aria-selected', 'false');
    });
    document.querySelectorAll('.kc-panel').forEach(p => p.classList.remove('active'));

    document.getElementById('tab-' + tab).classList.add('active');
    document.getElementById('tab-' + tab).setAttribute('aria-selected', 'true');
    document.getElementById('panel-' + tab).classList.add('active');

    if (tab === 'secrets') {
        if (!window._secretsModuleLoaded) {
            window._secretsModuleLoaded = true;
            const s = document.createElement('script');
            s.src = '/cfg/secrets.js';
            s.onload = () => {
                if (typeof renderSecretsSection === 'function') {
                    renderSecretsSection({
                        key: 'secrets',
                        icon: '🔐',
                        label: window.t ? window.t('knowledge.tab_secrets') : 'Secrets',
                        container: 'secrets-section-content'
                    });
                }
            };
            document.head.appendChild(s);
        } else if (typeof renderSecretsSection === 'function') {
            renderSecretsSection({
                key: 'secrets',
                icon: '🔐',
                label: window.t ? window.t('knowledge.tab_secrets') : 'Secrets',
                container: 'secrets-section-content'
            });
        }
    }
}

// ═══════════════════════════════════════════════════════════════
// CONTACTS
// ═══════════════════════════════════════════════════════════════

function debounceContactSearch() {
    clearTimeout(contactSearchTimer);
    contactSearchTimer = setTimeout(() => loadContacts(), 300);
}

async function loadContacts() {
    const q = (document.getElementById('contacts-search')?.value || '').trim();
    const url = q ? '/api/contacts?q=' + encodeURIComponent(q) : '/api/contacts';
    try {
        const resp = await fetch(url).then(r => r.json());
        allContacts = resp || [];
        renderContacts();
    } catch (e) {
        console.error('Failed to load contacts:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function renderContacts() {
    const tbody = document.getElementById('contacts-tbody');
    const empty = document.getElementById('contacts-empty');
    const tableWrap = tbody.closest('.kc-table-wrap');

    if (!allContacts.length) {
        tbody.innerHTML = '';
        tableWrap.classList.add('is-hidden');
        empty.classList.remove('is-hidden');
        return;
    }
    tableWrap.classList.remove('is-hidden');
    empty.classList.add('is-hidden');

    tbody.innerHTML = allContacts.map(c => `
        <tr>
            <td class="kc-name">${esc(c.name)}</td>
            <td>${c.email ? '<a href="mailto:' + esc(c.email) + '">' + esc(c.email) + '</a>' : '—'}</td>
            <td>${esc(c.phone || '—')}</td>
            <td>${esc(c.mobile || '—')}</td>
            <td>${esc(c.relationship || '—')}</td>
            <td class="kc-actions">
                <button class="btn btn-sm btn-secondary" onclick="editContact('${esc(c.id)}')" title="${t('common.btn_edit')}">✏️</button>
                <button class="btn btn-sm btn-danger" onclick="askDeleteContact('${esc(c.id)}', '${esc(c.name)}')" title="${t('common.btn_delete')}">🗑️</button>
            </td>
        </tr>
    `).join('');
}

function openContactModal(contact) {
    const modal = document.getElementById('contact-modal');
    const title = document.getElementById('contact-modal-title');

    document.getElementById('contact-id').value = contact ? contact.id : '';
    document.getElementById('contact-name').value = contact ? contact.name : '';
    document.getElementById('contact-email').value = contact ? contact.email : '';
    document.getElementById('contact-phone').value = contact ? contact.phone : '';
    document.getElementById('contact-mobile').value = contact ? contact.mobile : '';
    document.getElementById('contact-address').value = contact ? contact.address : '';
    document.getElementById('contact-relationship').value = contact ? contact.relationship : '';
    document.getElementById('contact-notes').value = contact ? contact.notes : '';

    title.textContent = contact ? t('knowledge.contacts_edit') : t('knowledge.contacts_add');
    modal.classList.add('active');
}

function editContact(id) {
    const c = allContacts.find(x => x.id === id);
    if (c) openContactModal(c);
}

async function saveContact() {
    const id = document.getElementById('contact-id').value;
    const data = {
        name: document.getElementById('contact-name').value.trim(),
        email: document.getElementById('contact-email').value.trim(),
        phone: document.getElementById('contact-phone').value.trim(),
        mobile: document.getElementById('contact-mobile').value.trim(),
        address: document.getElementById('contact-address').value.trim(),
        relationship: document.getElementById('contact-relationship').value.trim(),
        notes: document.getElementById('contact-notes').value.trim(),
    };

    if (!data.name) {
        showToast(t('knowledge.error_name_required'), 'error');
        return;
    }

    try {
        let resp;
        if (id) {
            data.id = id;
            resp = await fetch('/api/contacts/' + encodeURIComponent(id), {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        } else {
            resp = await fetch('/api/contacts', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        }

        if (!resp.ok) {
            const err = await resp.text();
            throw new Error(err);
        }

        closeModal('contact-modal');
        showToast(t('common.success'), 'success');
        loadContacts();
    } catch (e) {
        console.error('Save contact failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function askDeleteContact(id, name) {
    document.getElementById('delete-target-id').value = id;
    document.getElementById('delete-target-type').value = 'contact';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.delete_confirm_contact').replace('{name}', name);
    document.getElementById('delete-modal').classList.add('active');
}

// ═══════════════════════════════════════════════════════════════
// KNOWLEDGE FILES
// ═══════════════════════════════════════════════════════════════

async function loadFiles() {
    try {
        const resp = await fetch('/api/knowledge').then(r => r.json());
        allFiles = resp || [];
        renderFiles();
    } catch (e) {
        console.error('Failed to load files:', e);
    }
}

function filterFiles() {
    renderFiles();
}

function renderFiles() {
    const tbody = document.getElementById('files-tbody');
    const empty = document.getElementById('files-empty');
    const tableWrap = tbody.closest('.kc-table-wrap');
    const q = (document.getElementById('files-search')?.value || '').toLowerCase();

    const filtered = q ? allFiles.filter(f => f.name.toLowerCase().includes(q)) : allFiles;

    if (!filtered.length) {
        tbody.innerHTML = '';
        tableWrap.classList.add('is-hidden');
        empty.classList.remove('is-hidden');
        return;
    }
    tableWrap.classList.remove('is-hidden');
    empty.classList.add('is-hidden');

    tbody.innerHTML = filtered.map(f => {
        const icon = fileIcon(f.name);
        return `
        <tr>
            <td><span class="kc-file-icon">${icon}</span>${esc(f.name)}</td>
            <td class="kc-size">${formatSize(f.size)}</td>
            <td>${formatDate(f.modified)}</td>
            <td class="kc-actions">
                <a class="btn btn-sm btn-secondary" href="/api/knowledge/${encodeURIComponent(f.name)}" target="_blank" title="${t('knowledge.files_download')}">⬇️</a>
                <button class="btn btn-sm btn-danger" onclick="askDeleteFile('${esc(f.name)}')" title="${t('common.btn_delete')}">🗑️</button>
            </td>
        </tr>`;
    }).join('');
}

async function uploadFile(input) {
    const file = input.files[0];
    if (!file) return;

    const form = new FormData();
    form.append('file', file);

    try {
        const resp = await fetch('/api/knowledge/upload', {
            method: 'POST',
            body: form,
        });
        if (!resp.ok) {
            const err = await resp.text();
            throw new Error(err);
        }
        showToast(t('knowledge.files_uploaded'), 'success');
        loadFiles();
    } catch (e) {
        console.error('Upload failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
    input.value = '';
}

function askDeleteFile(name) {
    document.getElementById('delete-target-id').value = name;
    document.getElementById('delete-target-type').value = 'file';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.delete_confirm_file').replace('{name}', name);
    document.getElementById('delete-modal').classList.add('active');
}

// ═══════════════════════════════════════════════════════════════
// DEVICES
// ═══════════════════════════════════════════════════════════════

async function loadDevices() {
    try {
        const resp = await fetch('/api/devices');
        if (!resp.ok) throw new Error(await resp.text());
        allDevices = await resp.json() || [];
        renderDevices();
    } catch (e) {
        console.error('Failed to load devices:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function filterDevices() {
    renderDevices();
}

function renderDevices() {
    const tbody = document.getElementById('devices-tbody');
    const empty = document.getElementById('devices-empty');
    const tableWrap = tbody.closest('.kc-table-wrap');
    const q = (document.getElementById('devices-search')?.value || '').toLowerCase();

    const filtered = q ? allDevices.filter(d => {
        const hay = [d.name, d.type, d.ip_address, d.username, d.description, d.mac_address, ...(d.tags || [])].join(' ').toLowerCase();
        return hay.includes(q);
    }) : allDevices;

    if (!filtered.length) {
        tbody.innerHTML = '';
        tableWrap.style.display = 'none';
        empty.style.display = '';
        return;
    }
    tableWrap.style.display = '';
    empty.style.display = 'none';

    tbody.innerHTML = filtered.map(d => {
        const tags = (d.tags || []).map(tag =>
            '<span class="kc-tag">' + esc(tag) + '</span>'
        ).join('') || '—';
        return `
        <tr>
            <td class="kc-name">${esc(d.name)}</td>
            <td><span class="kc-type-badge">${esc(d.type)}</span></td>
            <td class="kc-mono">${esc(d.ip_address || '—')}</td>
            <td>${d.port || '—'}</td>
            <td>${esc(d.username || '—')}</td>
            <td class="kc-mono kc-size">${esc(d.mac_address || '—')}</td>
            <td>${tags}</td>
            <td class="kc-actions">
                <button class="btn btn-sm btn-secondary" onclick="editDevice('${esc(d.id)}')" title="${t('common.btn_edit')}">✏️</button>
                <button class="btn btn-sm btn-danger" onclick="askDeleteDevice('${esc(d.id)}', '${esc(d.name)}')" title="${t('common.btn_delete')}">🗑️</button>
            </td>
        </tr>`;
    }).join('');
}

function openDeviceModal(device) {
    const modal = document.getElementById('device-modal');
    const title = document.getElementById('device-modal-title');

    document.getElementById('device-id').value = device ? device.id : '';
    document.getElementById('device-name').value = device ? device.name : '';
    document.getElementById('device-type').value = device ? (device.type || 'server') : 'server';
    document.getElementById('device-ip').value = device ? device.ip_address : '';
    document.getElementById('device-port').value = device ? (device.port || 22) : 22;
    document.getElementById('device-username').value = device ? device.username : '';
    document.getElementById('device-description').value = device ? device.description : '';
    document.getElementById('device-mac').value = device ? device.mac_address : '';
    document.getElementById('device-tags').value = device ? (device.tags || []).join(', ') : '';
    document.getElementById('device-vault').value = device ? (device.vault_secret_id || '') : '';

    title.textContent = device ? t('knowledge.devices_edit') : t('knowledge.devices_add');
    modal.classList.add('active');
}

function editDevice(id) {
    const d = allDevices.find(x => x.id === id);
    if (d) openDeviceModal(d);
}

async function saveDevice() {
    const name = document.getElementById('device-name').value.trim();
    if (!name) {
        showToast(t('knowledge.devices_name_required'), 'error');
        return;
    }

    const tagsRaw = document.getElementById('device-tags').value.trim();
    const tags = tagsRaw ? tagsRaw.split(',').map(s => s.trim()).filter(Boolean) : [];

    const data = {
        name: name,
        type: document.getElementById('device-type').value,
        ip_address: document.getElementById('device-ip').value.trim(),
        port: parseInt(document.getElementById('device-port').value) || 22,
        username: document.getElementById('device-username').value.trim(),
        description: document.getElementById('device-description').value.trim(),
        mac_address: document.getElementById('device-mac').value.trim(),
        tags: tags,
        vault_secret_id: document.getElementById('device-vault').value || ''
    };

    const id = document.getElementById('device-id').value;
    try {
        let resp;
        if (id) {
            resp = await fetch('/api/devices/' + encodeURIComponent(id), {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        } else {
            resp = await fetch('/api/devices', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data),
            });
        }
        if (!resp.ok) {
            const err = await resp.json().catch(() => ({}));
            throw new Error(err.error || 'Request failed');
        }
        closeModal('device-modal');
        showToast(t('common.success'), 'success');
        loadDevices();
    } catch (e) {
        console.error('Save device failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

function askDeleteDevice(id, name) {
    document.getElementById('delete-target-id').value = id;
    document.getElementById('delete-target-type').value = 'device';
    document.getElementById('delete-confirm-text').textContent =
        t('knowledge.devices_delete_confirm').replace('{name}', name);
    document.getElementById('delete-modal').classList.add('active');
}

// ═══════════════════════════════════════════════════════════════
// DELETE CONFIRM
// ═══════════════════════════════════════════════════════════════

async function confirmDelete() {
    const id = document.getElementById('delete-target-id').value;
    const type = document.getElementById('delete-target-type').value;

    try {
        let resp;
        if (type === 'contact') {
            resp = await fetch('/api/contacts/' + encodeURIComponent(id), { method: 'DELETE' });
        } else if (type === 'device') {
            resp = await fetch('/api/devices/' + encodeURIComponent(id), { method: 'DELETE' });
        } else {
            resp = await fetch('/api/knowledge/' + encodeURIComponent(id), { method: 'DELETE' });
        }
        if (!resp.ok) {
            const err = await resp.text();
            throw new Error(err);
        }
        closeModal('delete-modal');
        showToast(t('common.success'), 'success');
        if (type === 'contact') loadContacts();
        else if (type === 'device') loadDevices();
        else loadFiles();
    } catch (e) {
        console.error('Delete failed:', e);
        showToast(t('common.error') + ': ' + e.message, 'error');
    }
}

// ═══════════════════════════════════════════════════════════════
// HELPERS
// ═══════════════════════════════════════════════════════════════

function formatSize(bytes) {
    if (!bytes || bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    let i = 0;
    let size = bytes;
    while (size >= 1024 && i < units.length - 1) { size /= 1024; i++; }
    return size.toFixed(i === 0 ? 0 : 1) + ' ' + units[i];
}

function formatDate(iso) {
    if (!iso) return '—';
    try {
        const d = new Date(iso);
        return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch { return iso; }
}

function fileIcon(name) {
    const ext = (name.split('.').pop() || '').toLowerCase();
    const icons = {
        md: '📝', txt: '📄', json: '📋', yaml: '⚙️', yml: '⚙️',
        csv: '📊', log: '📃', pdf: '📕', xml: '📰', html: '🌐',
        py: '🐍', go: '🔷', js: '🟨', sh: '🖥️',
    };
    return icons[ext] || '📄';
}
