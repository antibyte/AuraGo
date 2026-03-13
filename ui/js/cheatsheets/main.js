// AuraGo – Cheat Sheets page logic

let sheetsData = [];
let deleteTarget = null;

// ── Init ─────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
    applyI18n();
    initModals();
    injectRadialMenu();
    checkAuth();
    injectLanguageSwitcher();
    loadSheets();
});

function applyI18n() {
    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        const val = t(key);
        if (val && val !== key) el.textContent = val;
    });
    document.querySelectorAll('[data-i18n-ph]').forEach(el => {
        const key = el.getAttribute('data-i18n-ph');
        const val = t(key);
        if (val && val !== key) el.placeholder = val;
    });
    document.title = t('cheatsheets.page_title') || 'AuraGo - Cheat Sheets';
}

// ── API ──────────────────────────────────────────────────
async function api(path, options = {}) {
    const resp = await fetch('/api/cheatsheets' + path, {
        headers: { 'Content-Type': 'application/json' },
        ...options
    });
    if (!resp.ok) {
        const err = await resp.json().catch(() => ({ error: resp.statusText }));
        throw new Error(err.error || resp.statusText);
    }
    return resp.json();
}

// ── Load Data ────────────────────────────────────────────
async function loadSheets() {
    try {
        sheetsData = (await api('')) || [];
        renderSheets();
    } catch (e) {
        showToast(t('cheatsheets.error') + ': ' + e.message, 'error');
    }
}

// ── Render ───────────────────────────────────────────────
function renderSheets() {
    const grid = document.getElementById('sheets-grid');
    const empty = document.getElementById('sheets-empty');
    if (!sheetsData || sheetsData.length === 0) {
        grid.innerHTML = '';
        empty.style.display = '';
        return;
    }
    empty.style.display = 'none';

    grid.innerHTML = sheetsData.map(s => {
        const statusBadge = s.active
            ? `<span class="badge badge-active">${esc(t('cheatsheets.active'))}</span>`
            : `<span class="badge badge-inactive">${esc(t('cheatsheets.inactive'))}</span>`;
        const creatorBadge = s.created_by === 'agent'
            ? `<span class="badge badge-agent">🤖 Agent</span>`
            : '';
        const preview = esc((s.content || '').substring(0, 150).replace(/\n/g, ' '));
        const updated = s.updated_at ? timeAgo(s.updated_at) : '';
        return `
        <div class="card">
            <div class="card-header">
                <div>
                    <div class="card-title">${esc(s.name)}</div>
                    <div class="card-meta">${updated} ${creatorBadge}</div>
                </div>
                ${statusBadge}
            </div>
            <div class="card-preview">${preview || '<em>' + esc(t('cheatsheets.no_content')) + '</em>'}</div>
            <div class="card-actions">
                <button class="btn btn-primary btn-sm" onclick="openEdit('${esc(s.id)}')">${esc(t('cheatsheets.edit'))}</button>
                <button class="btn btn-secondary btn-sm" onclick="toggleActive('${esc(s.id)}', ${!s.active})">${s.active ? esc(t('cheatsheets.deactivate')) : esc(t('cheatsheets.activate'))}</button>
                <button class="btn btn-danger btn-sm" onclick="requestDelete('${esc(s.id)}', '${esc(s.name)}')">${esc(t('cheatsheets.delete'))}</button>
            </div>
        </div>`;
    }).join('');
}

// ── Create / Edit ────────────────────────────────────────
function openCreate() {
    document.getElementById('sheet-id').value = '';
    document.getElementById('sheet-name').value = '';
    document.getElementById('sheet-content').value = '';
    document.getElementById('sheet-active').checked = true;
    document.getElementById('modal-title').textContent = t('cheatsheets.create_new');
    switchEditorTab('edit');
    openModal('edit-modal');
    document.getElementById('sheet-name').focus();
}

function openEdit(id) {
    const s = sheetsData.find(x => x.id === id);
    if (!s) return;
    document.getElementById('sheet-id').value = s.id;
    document.getElementById('sheet-name').value = s.name;
    document.getElementById('sheet-content').value = s.content;
    document.getElementById('sheet-active').checked = s.active;
    document.getElementById('modal-title').textContent = t('cheatsheets.edit_sheet');
    switchEditorTab('edit');
    openModal('edit-modal');
    document.getElementById('sheet-name').focus();
}

async function saveSheet() {
    const id = document.getElementById('sheet-id').value;
    const name = document.getElementById('sheet-name').value.trim();
    const content = document.getElementById('sheet-content').value;
    const active = document.getElementById('sheet-active').checked;

    if (!name) {
        document.getElementById('sheet-name').focus();
        showToast(t('cheatsheets.name_required'), 'warning');
        return;
    }

    try {
        if (id) {
            await api('/' + id, {
                method: 'PUT',
                body: JSON.stringify({ name, content, active })
            });
        } else {
            await api('', {
                method: 'POST',
                body: JSON.stringify({ name, content })
            });
        }
        closeModal('edit-modal');
        showToast(t('cheatsheets.saved'), 'success');
        await loadSheets();
    } catch (e) {
        showToast(t('cheatsheets.error') + ': ' + e.message, 'error');
    }
}

// ── Toggle Active ────────────────────────────────────────
async function toggleActive(id, newState) {
    try {
        await api('/' + id, {
            method: 'PUT',
            body: JSON.stringify({ active: newState })
        });
        showToast(t('cheatsheets.saved'), 'success');
        await loadSheets();
    } catch (e) {
        showToast(t('cheatsheets.error') + ': ' + e.message, 'error');
    }
}

// ── Delete ───────────────────────────────────────────────
function requestDelete(id, name) {
    deleteTarget = { id, name };
    document.getElementById('delete-message').innerHTML =
        t('cheatsheets.delete_prompt', { name: '<strong>"' + esc(name) + '"</strong>' });
    document.getElementById('delete-confirm-input').value = '';
    document.getElementById('delete-confirm-input').placeholder = name;
    document.getElementById('btn-delete-confirm').disabled = true;
    openModal('delete-modal');
    document.getElementById('delete-confirm-input').focus();
}

function checkDeleteConfirm() {
    const input = document.getElementById('delete-confirm-input').value.trim();
    document.getElementById('btn-delete-confirm').disabled = input !== deleteTarget?.name;
}

async function confirmDelete() {
    if (!deleteTarget) return;
    try {
        await api('/' + deleteTarget.id, { method: 'DELETE' });
        closeModal('delete-modal');
        showToast(t('cheatsheets.deleted'), 'success');
        await loadSheets();
    } catch (e) {
        showToast(t('cheatsheets.error') + ': ' + e.message, 'error');
    }
    deleteTarget = null;
}

// ── Editor Preview ───────────────────────────────────────
function switchEditorTab(tab) {
    document.querySelectorAll('.editor-tab').forEach(el =>
        el.classList.toggle('active', el.textContent.toLowerCase().includes(tab))
    );
    const editor = document.getElementById('sheet-content');
    const preview = document.getElementById('sheet-preview');
    if (tab === 'edit') {
        editor.style.display = '';
        preview.style.display = 'none';
    } else {
        editor.style.display = 'none';
        preview.style.display = '';
        preview.innerHTML = renderMarkdown(editor.value);
    }
}

// Simple markdown renderer (no external deps)
function renderMarkdown(md) {
    if (!md) return '<em>' + esc(t('cheatsheets.no_content')) + '</em>';
    let html = esc(md);
    // Code blocks
    html = html.replace(/```(\w*)\n([\s\S]*?)```/g, '<pre><code>$2</code></pre>');
    // Inline code
    html = html.replace(/`([^`]+)`/g, '<code>$1</code>');
    // Headers
    html = html.replace(/^### (.+)$/gm, '<h3>$1</h3>');
    html = html.replace(/^## (.+)$/gm, '<h2>$1</h2>');
    html = html.replace(/^# (.+)$/gm, '<h1>$1</h1>');
    // Bold / Italic
    html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
    html = html.replace(/\*(.+?)\*/g, '<em>$1</em>');
    // Blockquote
    html = html.replace(/^&gt; (.+)$/gm, '<blockquote>$1</blockquote>');
    // Unordered lists
    html = html.replace(/^[*-] (.+)$/gm, '<li>$1</li>');
    html = html.replace(/(<li>.*<\/li>\n?)+/g, '<ul>$&</ul>');
    // Ordered lists
    html = html.replace(/^\d+\. (.+)$/gm, '<li>$1</li>');
    // Line breaks
    html = html.replace(/\n\n/g, '</p><p>');
    html = html.replace(/\n/g, '<br>');
    return '<p>' + html + '</p>';
}
