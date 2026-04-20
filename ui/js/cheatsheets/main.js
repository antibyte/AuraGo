// AuraGo – Cheat Sheets page logic

let sheetsData = [];
let deleteTarget = null;
let viewMode = localStorage.getItem('cheatsheets-view-mode') || 'auto'; // 'grid' | 'list' | 'auto'
let expandedCards = new Set(); // Track expanded card IDs
let currentAttachments = []; // Saved attachments for the currently edited sheet
let pendingAttachments = [];  // Staged attachments for an unsaved new sheet
let knowledgePickerSelection = new Set(); // Selected knowledge files in picker
let currentCheatTab = localStorage.getItem('cheatsheets-tab') || 'user'; // 'user' | 'agent'

// ── Init ─────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', () => {
    document.title = t('cheatsheets.page_title') || 'AuraGo - Cheat Sheets';
    updateViewToggle();
    loadSheets();
});

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

// ── View Mode Functions ──────────────────────────────────
function getEffectiveViewMode() {
    if (viewMode !== 'auto') return viewMode;
    return sheetsData.length > 8 ? 'list' : 'grid';
}

function setViewMode(mode) {
    viewMode = mode;
    localStorage.setItem('cheatsheets-view-mode', mode);
    renderSheets();
    updateViewToggle();
}

function updateViewToggle() {
    const effective = getEffectiveViewMode();
    document.querySelectorAll('#view-toggle button').forEach(btn => {
        const isActive = (viewMode === 'auto' && btn.dataset.mode === effective) ||
                        (viewMode !== 'auto' && btn.dataset.mode === viewMode);
        btn.classList.toggle('active', isActive);
    });
}

function toggleCardExpand(id) {
    if (expandedCards.has(id)) {
        expandedCards.delete(id);
    } else {
        expandedCards.add(id);
    }
    renderSheets();
}

// ── Tab Switching ────────────────────────────────────────
function switchCheatTab(tab) {
    currentCheatTab = tab;
    localStorage.setItem('cheatsheets-tab', tab);

    document.querySelectorAll('.cheatsheet-tab').forEach(function (btn) {
        btn.classList.remove('active');
        btn.setAttribute('aria-selected', 'false');
    });
    document.querySelectorAll('.cheatsheet-panel').forEach(function (panel) {
        panel.classList.remove('active');
    });

    const activeTab = document.getElementById('tab-' + tab);
    const activePanel = document.getElementById('panel-' + tab);
    if (activeTab) { activeTab.classList.add('active'); activeTab.setAttribute('aria-selected', 'true'); }
    if (activePanel) { activePanel.classList.add('active'); }
}

// ── Render ───────────────────────────────────────────────
function renderSheets() {
    const empty = document.getElementById('sheets-empty');
    const mode = getEffectiveViewMode();
    const userSheets = sheetsData.filter(s => s.created_by !== 'agent');
    const agentSheets = sheetsData.filter(s => s.created_by === 'agent');

    if (!sheetsData || sheetsData.length === 0) {
        empty.classList.remove('is-hidden');
        renderSheetGroup('user', [], mode);
        renderSheetGroup('agent', [], mode);
        return;
    }
    empty.classList.add('is-hidden');

    renderSheetGroup('user', userSheets, mode);
    renderSheetGroup('agent', agentSheets, mode);
    setGroupCount('user', userSheets.length);
    setGroupCount('agent', agentSheets.length);
    updateViewToggle();
    switchCheatTab(currentCheatTab);
}

function renderSheetGroup(groupKey, sheets, mode) {
    const grid = document.getElementById(`sheets-grid-${groupKey}`);
    const empty = document.getElementById(`sheets-empty-${groupKey}`);
    grid.classList.toggle('list-view', mode === 'list');

    if (!sheets || sheets.length === 0) {
        grid.innerHTML = '';
        empty.classList.remove('is-hidden');
        return;
    }

    empty.classList.add('is-hidden');
    grid.innerHTML = mode === 'list'
        ? sheets.map(s => renderSheetCompact(s)).join('')
        : sheets.map(s => renderSheetGrid(s)).join('');
}

function setGroupCount(groupKey, count) {
    const badge = document.getElementById(`sheets-count-${groupKey}`);
    const header = document.getElementById(`sheets-header-count-${groupKey}`);
    if (badge) badge.textContent = count;
    if (header) header.textContent = count;
}

// Compact List View
function renderSheetCompact(s) {
    const statusIcon = s.active ? '🟢' : '⚪';
    const creatorIcon = s.created_by === 'agent' ? '🤖' : '';
    const creatorTitle = s.created_by === 'agent'
        ? esc(t('cheatsheets.created_by_agent') || 'Created by agent')
        : esc(t('cheatsheets.created_by_user') || 'Created by user');

    return `
        <div class="card-compact" onclick="if(event.target.closest('.card-actions')) return; openEdit('${escJs(s.id)}')">
            <span class="card-icon" title="${s.active ? t('cheatsheets.active') : t('cheatsheets.inactive')}">${statusIcon}</span>
            <span class="card-name">${esc(s.name)}</span>
            ${creatorIcon ? `<span class="card-icon" title="${creatorTitle}">${creatorIcon}</span>` : ''}
            <div class="card-actions" onclick="event.stopPropagation()">
                <button class="btn btn-sm btn-secondary" onclick="openEdit('${escJs(s.id)}')" title="${esc(t('cheatsheets.edit'))}">✏️</button>
                <button class="btn btn-sm ${s.active ? 'btn-secondary' : 'btn-primary'}" onclick="toggleActive('${escJs(s.id)}', ${!s.active})" title="${s.active ? esc(t('cheatsheets.deactivate')) : esc(t('cheatsheets.activate'))}">${s.active ? '⏸️' : '▶️'}</button>
                <button class="btn btn-sm btn-danger" onclick="requestDelete('${escJs(s.id)}', '${esc(s.name)}')" title="${esc(t('cheatsheets.delete'))}">🗑️</button>
            </div>
        </div>
    `;
}

// Grid View (Expandable)
function renderSheetGrid(s) {
    const isExpanded = expandedCards.has(s.id);
    const statusBadge = s.active
        ? `<span class="badge badge-active">${esc(t('cheatsheets.active'))}</span>`
        : `<span class="badge badge-inactive">${esc(t('cheatsheets.inactive'))}</span>`;
    const creatorBadge = s.created_by === 'agent'
        ? `<span class="badge badge-agent">🤖 ${esc(t('cheatsheets.group_agent') || 'Agent')}</span>`
        : '';
    const attachBadge = (s.attachment_count > 0)
        ? `<span class="badge badge-attachment">📎 ${s.attachment_count}</span>`
        : '';
    const preview = esc((s.content || '').substring(0, 150).replace(/\n/g, ' '));
    const updated = s.updated_at ? timeAgo(s.updated_at) : '';

    return `
        <div class="card card-expanded ${isExpanded ? 'expanded' : ''}">
            <div class="card-header" onclick="toggleCardExpand('${escJs(s.id)}')">
                <span class="card-toggle">▶</span>
                <div class="card-header-content">
                    <div class="card-title">${esc(s.name)}</div>
                    <div class="card-badges">${statusBadge}${creatorBadge}${attachBadge}</div>
                </div>
            </div>
            <div class="card-body">
                <div class="card-preview">${preview || '<em>' + esc(t('cheatsheets.no_content')) + '</em>'}</div>
                ${updated ? `<div class="card-meta-line">🕐 ${updated}</div>` : ''}
            </div>
            <div class="card-footer">
                <button class="btn btn-primary btn-sm" onclick="openEdit('${escJs(s.id)}')">${esc(t('cheatsheets.edit'))}</button>
                <button class="btn btn-secondary btn-sm" onclick="toggleActive('${escJs(s.id)}', ${!s.active})">${s.active ? esc(t('cheatsheets.deactivate')) : esc(t('cheatsheets.activate'))}</button>
                <button class="btn btn-danger btn-sm" onclick="requestDelete('${escJs(s.id)}', '${esc(s.name)}')">${esc(t('cheatsheets.delete'))}</button>
            </div>
        </div>
    `;
}

// ── Create / Edit ────────────────────────────────────────
function openCreate() {
    document.getElementById('sheet-id').value = '';
    document.getElementById('sheet-name').value = '';
    document.getElementById('sheet-content').value = '';
    document.getElementById('sheet-active').checked = true;
    document.getElementById('modal-title').textContent = t('cheatsheets.create_new');
    currentAttachments = [];
    pendingAttachments = [];
    renderAttachments();
    switchEditorTab('edit');
    openModal('edit-modal');
    document.getElementById('sheet-name').focus();
}

async function openEdit(id) {
    // Fetch full sheet data (includes attachments array)
    let s;
    try {
        s = await api('/' + id);
    } catch (e) {
        showToast(t('cheatsheets.error') + ': ' + e.message, 'error');
        return;
    }
    document.getElementById('sheet-id').value = s.id;
    document.getElementById('sheet-name').value = s.name;
    document.getElementById('sheet-content').value = s.content;
    document.getElementById('sheet-active').checked = s.active;
    document.getElementById('modal-title').textContent = t('cheatsheets.edit_sheet');
    currentAttachments = s.attachments || [];
    pendingAttachments = [];
    renderAttachments();
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
        let targetId = id;
        if (id) {
            await api('/' + id, {
                method: 'PUT',
                body: JSON.stringify({ name, content, active })
            });
        } else {
            const created = await api('', {
                method: 'POST',
                body: JSON.stringify({ name, content })
            });
            targetId = created.id;
        }
        // Upload any staged pending attachments
        for (const p of pendingAttachments) {
            try {
                if (p.source === 'upload') {
                    const blob = new Blob([p.content], { type: 'text/plain' });
                    const file = new File([blob], p.filename, { type: 'text/plain' });
                    const form = new FormData();
                    form.append('file', file);
                    const resp = await fetch(`/api/cheatsheets/${targetId}/attachments`, { method: 'POST', body: form });
                    if (!resp.ok) {
                        const err = await resp.json().catch(() => ({ error: resp.statusText }));
                        showToast(`${p.filename}: ${err.error}`, 'warning');
                    }
                } else {
                    const resp = await fetch(`/api/cheatsheets/${targetId}/attachments`, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ source: 'knowledge', filename: p.filename })
                    });
                    if (!resp.ok) {
                        const err = await resp.json().catch(() => ({ error: resp.statusText }));
                        showToast(`${p.filename}: ${err.error}`, 'warning');
                    }
                }
            } catch (e) {
                showToast(`${p.filename}: ${e.message}`, 'warning');
            }
        }
        pendingAttachments = [];
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
    openModal('delete-modal');
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
        editor.classList.remove('is-hidden');
        preview.classList.add('is-hidden');
    } else {
        editor.classList.add('is-hidden');
        preview.classList.remove('is-hidden');
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

// ── Attachments ──────────────────────────────────────────

const MAX_ATTACHMENT_CHARS = 25000;

function renderAttachments() {
    const list = document.getElementById('attachments-list');
    const counter = document.getElementById('attachment-char-counter');
    const savedChars = currentAttachments.reduce((sum, a) => sum + (a.char_count || 0), 0);
    const pendingChars = pendingAttachments.reduce((sum, a) => sum + (a.charCount || 0), 0);
    const totalChars = savedChars + pendingChars;

    counter.textContent = `${totalChars.toLocaleString()} / ${MAX_ATTACHMENT_CHARS.toLocaleString()} ${t('cheatsheets.characters') || 'characters'}`;
    counter.classList.toggle('over-limit', totalChars > MAX_ATTACHMENT_CHARS);

    const allItems = [
        ...currentAttachments.map(a => ({ ...a, isPending: false, displayId: a.id })),
        ...pendingAttachments.map(a => ({ ...a, char_count: a.charCount, isPending: true, displayId: a.localId }))
    ];

    if (allItems.length === 0) {
        list.innerHTML = `<div class="attachments-empty">${esc(t('cheatsheets.no_attachments') || 'No attachments')}</div>`;
        return;
    }

    list.innerHTML = allItems.map(a => {
        const sourceIcon = a.isPending ? '⏳' : (a.source === 'knowledge' ? '📚' : '📎');
        return `
            <div class="attachment-item${a.isPending ? ' attachment-pending' : ''}">
                <span class="attachment-icon">${sourceIcon}</span>
                <span class="attachment-name" title="${esc(a.filename)}">${esc(a.filename)}</span>
                <span class="attachment-size">${(a.char_count || a.charCount || 0).toLocaleString()} chars</span>
                <button class="btn btn-sm btn-danger attachment-remove" onclick="removeAttachment('${esc(a.displayId)}')" title="${esc(t('cheatsheets.remove_attachment') || 'Remove')}">✕</button>
            </div>`;
    }).join('');
}

async function uploadAttachment(input) {
    const file = input.files[0];
    if (!file) return;
    input.value = ''; // reset for re-upload of same file

    const ext = file.name.toLowerCase().split('.').pop();
    if (ext !== 'txt' && ext !== 'md') {
        showToast(t('cheatsheets.invalid_file_type') || 'Only .txt and .md files are allowed.', 'warning');
        return;
    }

    const csID = document.getElementById('sheet-id').value;
    if (!csID) {
        // No ID yet — buffer locally until the sheet is saved
        try {
            const content = await file.text();
            const charCount = [...content].length;
            const totalChars = currentAttachments.reduce((s, a) => s + (a.char_count || 0), 0)
                             + pendingAttachments.reduce((s, a) => s + (a.charCount || 0), 0);
            if (totalChars + charCount > MAX_ATTACHMENT_CHARS) {
                showToast(t('cheatsheets.char_limit_exceeded') || `Attachment exceeds the ${MAX_ATTACHMENT_CHARS.toLocaleString()} character limit.`, 'warning');
                return;
            }
            pendingAttachments.push({
                localId: 'p-' + Date.now() + '-' + Math.random().toString(36).slice(2),
                filename: file.name,
                source: 'upload',
                content,
                charCount,
                isPending: true
            });
            renderAttachments();
        } catch (e) {
            showToast((t('cheatsheets.error') || 'Error') + ': ' + e.message, 'error');
        }
        return;
    }

    // Existing sheet — upload immediately
    const form = new FormData();
    form.append('file', file);

    try {
        const resp = await fetch(`/api/cheatsheets/${csID}/attachments`, {
            method: 'POST',
            body: form
        });
        if (!resp.ok) {
            const err = await resp.json().catch(() => ({ error: resp.statusText }));
            throw new Error(err.error || resp.statusText);
        }
        const attachment = await resp.json();
        currentAttachments.push(attachment);
        renderAttachments();
        showToast(t('cheatsheets.attachment_added') || 'Attachment added.', 'success');
    } catch (e) {
        showToast((t('cheatsheets.error') || 'Error') + ': ' + e.message, 'error');
    }
}

async function removeAttachment(attachmentID) {
    // Check if it's a pending (not yet saved) attachment
    const pendingIdx = pendingAttachments.findIndex(a => a.localId === attachmentID);
    if (pendingIdx !== -1) {
        pendingAttachments.splice(pendingIdx, 1);
        renderAttachments();
        return;
    }

    const csID = document.getElementById('sheet-id').value;
    if (!csID) return;

    try {
        const resp = await fetch(`/api/cheatsheets/${csID}/attachments/${attachmentID}`, {
            method: 'DELETE'
        });
        if (!resp.ok) {
            const err = await resp.json().catch(() => ({ error: resp.statusText }));
            throw new Error(err.error || resp.statusText);
        }
        currentAttachments = currentAttachments.filter(a => a.id !== attachmentID);
        renderAttachments();
        showToast(t('cheatsheets.attachment_removed') || 'Attachment removed.', 'success');
    } catch (e) {
        showToast((t('cheatsheets.error') || 'Error') + ': ' + e.message, 'error');
    }
}

// ── Knowledge Picker ─────────────────────────────────────

async function openKnowledgePicker() {
    knowledgePickerSelection.clear();
    document.getElementById('btn-knowledge-confirm').disabled = true;

    const list = document.getElementById('knowledge-picker-list');
    list.innerHTML = `<div class="knowledge-picker-loading">${esc(t('cheatsheets.loading') || 'Loading...')}</div>`;
    openModal('knowledge-picker-modal');

    try {
        const resp = await fetch('/api/knowledge');
        if (!resp.ok) throw new Error(resp.statusText);
        const files = await resp.json();

        // Filter to .txt and .md only
        const textFiles = files.filter(f => {
            const ext = (f.extension || '').toLowerCase();
            return ext === 'txt' || ext === 'md';
        });

        // Exclude already attached (saved and pending) filenames
        const attachedNames = new Set([
            ...currentAttachments.map(a => a.filename),
            ...pendingAttachments.map(a => a.filename)
        ]);
        const available = textFiles.filter(f => !attachedNames.has(f.name));

        if (available.length === 0) {
            list.innerHTML = `<div class="knowledge-picker-empty">${esc(t('cheatsheets.no_knowledge_files') || 'No compatible files found (.txt, .md)')}</div>`;
            return;
        }

        list.innerHTML = available.map(f => `
            <label class="knowledge-picker-item">
                <input type="checkbox" value="${esc(f.name)}" onchange="toggleKnowledgePick(this)">
                <span class="knowledge-picker-name">${esc(f.name)}</span>
                <span class="knowledge-picker-size">${formatFileSize(f.size)}</span>
            </label>
        `).join('');
    } catch (e) {
        list.innerHTML = `<div class="knowledge-picker-empty">${esc((t('cheatsheets.error') || 'Error') + ': ' + e.message)}</div>`;
    }
}

function toggleKnowledgePick(checkbox) {
    if (checkbox.checked) {
        knowledgePickerSelection.add(checkbox.value);
    } else {
        knowledgePickerSelection.delete(checkbox.value);
    }
    document.getElementById('btn-knowledge-confirm').disabled = knowledgePickerSelection.size === 0;
}

async function confirmKnowledgePick() {
    if (knowledgePickerSelection.size === 0) return;

    const csID = document.getElementById('sheet-id').value;
    let added = 0;

    if (!csID) {
        // No ID yet — buffer as pending attachments
        for (const filename of knowledgePickerSelection) {
            try {
                const resp = await fetch(`/api/knowledge/${encodeURIComponent(filename)}?inline=1`);
                if (!resp.ok) {
                    showToast(`${filename}: ${resp.statusText}`, 'warning');
                    continue;
                }
                const content = await resp.text();
                const charCount = [...content].length;
                const totalChars = currentAttachments.reduce((s, a) => s + (a.char_count || 0), 0)
                                 + pendingAttachments.reduce((s, a) => s + (a.charCount || 0), 0);
                if (totalChars + charCount > MAX_ATTACHMENT_CHARS) {
                    showToast(`${filename}: ${t('cheatsheets.char_limit_exceeded') || 'Character limit exceeded'}`, 'warning');
                    continue;
                }
                pendingAttachments.push({
                    localId: 'p-' + Date.now() + '-' + Math.random().toString(36).slice(2),
                    filename,
                    source: 'knowledge',
                    content,
                    charCount,
                    isPending: true
                });
                added++;
            } catch (e) {
                showToast(`${filename}: ${e.message}`, 'error');
            }
        }
        closeModal('knowledge-picker-modal');
        renderAttachments();
        if (added > 0) {
            showToast(t('cheatsheets.attachments_added', { count: added }) || `${added} attachment(s) added.`, 'success');
        }
        return;
    }

    // Existing sheet — upload immediately
    for (const filename of knowledgePickerSelection) {
        try {
            const resp = await fetch(`/api/cheatsheets/${csID}/attachments`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ source: 'knowledge', filename })
            });
            if (!resp.ok) {
                const err = await resp.json().catch(() => ({ error: resp.statusText }));
                showToast(`${filename}: ${err.error || resp.statusText}`, 'error');
                continue;
            }
            const attachment = await resp.json();
            currentAttachments.push(attachment);
            added++;
        } catch (e) {
            showToast(`${filename}: ${e.message}`, 'error');
        }
    }

    closeModal('knowledge-picker-modal');
    renderAttachments();
    if (added > 0) {
        showToast(t('cheatsheets.attachments_added', { count: added }) || `${added} attachment(s) added.`, 'success');
    }
}

function formatFileSize(bytes) {
    if (bytes < 1024) return bytes + ' B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
    return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
}
