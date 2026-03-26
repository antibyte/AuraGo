/* AuraGo – Skills Manager page JS */
/* global I18N, t, applyI18n, esc */
'use strict';

let allSkills = [];
let allTemplates = [];
let currentTypeFilter = 'all';
let currentSecFilter = 'all';
let currentDetailId = '';
let deleteTargetId = '';
let selectedFile = null;
let codeEditorView = null;
let codeEditorSkillId = '';
let vaultKeyTargetId = '';
let allVaultSecrets = [];

// ── Initialization ──────────────────────────────────────────────────────────

document.addEventListener('DOMContentLoaded', () => {
    loadSkills();
    loadTemplates();
    initDropzone();
});

// ── Data fetching ───────────────────────────────────────────────────────────

async function loadSkills() {
    try {
        const resp = await fetch('/api/skills');
        if (resp.status === 503) {
            showDisabledState();
            return;
        }
        const data = await resp.json();
        if (data.status !== 'ok') {
            showDisabledState();
            return;
        }
        allSkills = data.skills || [];
        updateStats(data.stats);
        renderSkills();
    } catch (e) {
        console.error('Failed to load skills:', e);
    }
}

async function loadTemplates() {
    try {
        const resp = await fetch('/api/skills/templates');
        const data = await resp.json();
        if (data.status === 'ok') {
            allTemplates = data.templates || [];
            populateTemplateSelect();
        }
    } catch (e) {
        console.error('Failed to load templates:', e);
    }
}

function showDisabledState() {
    document.getElementById('sk-grid').style.display = 'none';
    document.getElementById('sk-empty').style.display = 'none';
    document.getElementById('sk-disabled').style.display = '';
    document.getElementById('sk-status-bar').style.display = 'none';
    document.getElementById('sk-toolbar-actions').style.display = 'none';
    document.getElementById('sk-security-filters').style.display = 'none';
}

// ── Stats ───────────────────────────────────────────────────────────────────

function updateStats(stats) {
    if (!stats) return;
    document.getElementById('sk-total').textContent = stats.total || 0;
    document.getElementById('sk-agent').textContent = stats.agent || 0;
    document.getElementById('sk-user').textContent = stats.user || 0;
    document.getElementById('sk-pending').textContent = stats.pending || 0;
}

// ── Rendering ───────────────────────────────────────────────────────────────

function renderSkills() {
    const grid = document.getElementById('sk-grid');
    const empty = document.getElementById('sk-empty');
    const disabled = document.getElementById('sk-disabled');
    disabled.style.display = 'none';

    const filtered = getFilteredSkills();

    if (filtered.length === 0) {
        grid.style.display = 'none';
        empty.style.display = '';
        return;
    }
    empty.style.display = 'none';
    grid.style.display = '';

    grid.innerHTML = filtered.map(s => renderCard(s)).join('');
    if (typeof applyI18n === 'function') applyI18n();
}

function renderCard(skill) {
    const name = esc(skill.Name || skill.name || 'Unknown');
    const desc = esc(skill.Description || skill.description || '');
    const type = (skill.Type || skill.type || 'user').toLowerCase();
    const secStatus = (skill.SecurityStatus || skill.security_status || 'pending').toLowerCase();
    const enabled = skill.Enabled !== undefined ? skill.Enabled : skill.enabled;
    const id = esc(skill.ID || skill.id || '');
    const deps = skill.Dependencies || skill.dependencies || [];
    const vaultKeys = skill.VaultKeys || skill.vault_keys || [];

    const typeLabel = type.charAt(0).toUpperCase() + type.slice(1);
    const secBadge = renderSecurityBadge(secStatus);
    const enabledClass = enabled ? 'sk-enabled' : 'sk-disabled-card';
    const toggleLabel = enabled ? (t('skills.btn_disable') || 'Disable') : (t('skills.btn_enable') || 'Enable');
    const toggleClass = enabled ? 'btn-secondary' : 'btn-primary';

    let depTags = '';
    if (deps.length > 0) {
        depTags = `<div class="sk-card-deps">${deps.slice(0, 5).map(d => `<span class="sk-dep-tag">${esc(d)}</span>`).join('')}${deps.length > 5 ? `<span class="sk-dep-tag">+${deps.length - 5}</span>` : ''}</div>`;
    }

    let vaultRow = '';
    if (vaultKeys.length > 0) {
        const keyTags = vaultKeys.map(k => `<code class="sk-vault-key-tag">${esc(k)}</code>`).join('');
        vaultRow = `<div class="sk-card-vault">
            <span class="sk-vault-icon" title="${t('skills.vault_keys_label') || 'Vault Keys'}">🔑</span>
            <span class="sk-vault-keys">${keyTags}</span>
            <button class="btn btn-xs btn-secondary sk-vault-edit-btn" onclick="openVaultKeyModal('${id}')" title="${t('skills.btn_edit_secrets') || 'Edit Secrets'}">✏️</button>
        </div>`;
    }

    return `
    <div class="sk-card ${enabledClass}" data-id="${id}" data-type="${type}" data-sec="${secStatus}">
        <div class="sk-card-header">
            <div class="sk-card-name" title="${name}">${name}</div>
            <div class="sk-card-badges">
                <span class="sk-type-badge sk-type-${type}">${typeLabel}</span>
                ${secBadge}
            </div>
        </div>
        <div class="sk-card-desc">${desc || '<em>' + (t('skills.no_description') || 'No description') + '</em>'}</div>
        ${depTags}
        ${vaultRow}
        <div class="sk-card-actions">
            <button class="btn btn-sm btn-secondary" onclick="showDetail('${id}')" data-i18n="skills.btn_details">Details</button>
            <button class="btn btn-sm btn-secondary" onclick="viewCode('${id}')" data-i18n="skills.btn_view_code">Code</button>
            <button class="btn btn-sm ${toggleClass}" onclick="toggleSkill('${id}', ${!enabled})">${toggleLabel}</button>
            <button class="btn btn-sm btn-danger" onclick="showDeleteSkillModal('${id}', '${name}')" data-i18n="skills.btn_delete">Delete</button>
        </div>
    </div>`;
}

function renderSecurityBadge(status) {
    const labels = {
        clean: t('skills.sec_clean') || 'Clean',
        warning: t('skills.sec_warning') || 'Warning',
        dangerous: t('skills.sec_dangerous') || 'Dangerous',
        pending: t('skills.sec_pending') || 'Pending',
        error: t('skills.sec_error') || 'Error'
    };
    const label = labels[status] || status;
    return `<span class="sk-sec-badge sk-sec-${status}">${label}</span>`;
}

// ── Filtering ───────────────────────────────────────────────────────────────

function getFilteredSkills() {
    const search = (document.getElementById('sk-search').value || '').toLowerCase();
    return allSkills.filter(s => {
        const type = (s.Type || s.type || '').toLowerCase();
        const sec = (s.SecurityStatus || s.security_status || 'pending').toLowerCase();
        const name = (s.Name || s.name || '').toLowerCase();
        const desc = (s.Description || s.description || '').toLowerCase();

        if (currentTypeFilter !== 'all' && type !== currentTypeFilter) return false;
        if (currentSecFilter !== 'all' && sec !== currentSecFilter) return false;
        if (search && !name.includes(search) && !desc.includes(search)) return false;
        return true;
    });
}

// eslint-disable-next-line no-unused-vars
function filterSkills() {
    renderSkills();
}

// eslint-disable-next-line no-unused-vars
function setSkillFilter(filter) {
    currentTypeFilter = filter;
    document.querySelectorAll('.sk-filter-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.filter === filter);
    });
    renderSkills();
}

// eslint-disable-next-line no-unused-vars
function setSecurityFilter(filter) {
    currentSecFilter = filter;
    document.querySelectorAll('.sk-pill').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.sec === filter);
    });
    renderSkills();
}

// ── Skill Actions ───────────────────────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
async function toggleSkill(id, enabled) {
    try {
        const resp = await fetch(`/api/skills/${encodeURIComponent(id)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ enabled })
        });
        const data = await resp.json();
        if (data.status === 'ok') {
            showToast(t('skills.toggle_success') || 'Skill updated', 'success');
            await loadSkills();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    } catch (e) {
        showToast(t('common.error') || 'Error', 'error');
    }
}

// ── Detail Modal ────────────────────────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
async function showDetail(id) {
    currentDetailId = id;
    const body = document.getElementById('detail-modal-body');
    body.innerHTML = `<p>${t('common.loading') || 'Loading...'}</p>`;
    document.getElementById('detail-modal').classList.add('active');

    try {
        const resp = await fetch(`/api/skills/${encodeURIComponent(id)}`);
        const data = await resp.json();
        if (data.status !== 'ok') {
            body.innerHTML = `<p class="sk-error">${esc(data.message || 'Not found')}</p>`;
            return;
        }
        const s = data.skill;
        const sec = s.SecurityReport || s.security_report;
        const deps = (s.Dependencies || s.dependencies || []);
        const vaultKeys = (s.VaultKeys || s.vault_keys || []);

        let secHTML = '';
        if (sec) {
            const findings = sec.StaticAnalysis || sec.static_analysis || [];
            if (findings.length > 0) {
                secHTML = `<div class="sk-findings">
                    <h4 data-i18n="skills.findings_title">${t('skills.findings_title') || 'Security Findings'}</h4>
                    <ul>${findings.map(f => `<li class="sk-finding sk-finding-${(f.Severity || f.severity || 'info').toLowerCase()}">
                        <strong>${esc(f.Category || f.category || '')}</strong>: ${esc(f.Message || f.message || '')}
                        ${f.Line || f.line ? ` <span class="sk-finding-line">(line ${f.Line || f.line})</span>` : ''}
                    </li>`).join('')}</ul>
                </div>`;
            }
            if (sec.GuardianDecision || sec.guardian_decision) {
                secHTML += `<div class="sk-guardian-result">
                    <h4>LLM Guardian</h4>
                    <p><strong>${t('skills.guardian_decision') || 'Decision'}:</strong> ${esc(sec.GuardianDecision || sec.guardian_decision)}</p>
                    ${sec.GuardianReason || sec.guardian_reason ? `<p>${esc(sec.GuardianReason || sec.guardian_reason)}</p>` : ''}
                </div>`;
            }
        }

        body.innerHTML = `
            <div class="sk-detail-grid">
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_name') || 'Name'}:</span> <span>${esc(s.Name || s.name)}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_type') || 'Type'}:</span> <span class="sk-type-badge sk-type-${(s.Type || s.type || '').toLowerCase()}">${esc(s.Type || s.type)}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_status') || 'Security'}:</span> ${renderSecurityBadge((s.SecurityStatus || s.security_status || 'pending').toLowerCase())}</div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_enabled') || 'Enabled'}:</span> <span>${s.Enabled || s.enabled ? '✅' : '❌'}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_created') || 'Created'}:</span> <span>${esc(s.CreatedAt || s.created_at || '-')}</span></div>
                <div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_description') || 'Description'}:</span> <span>${esc(s.Description || s.description || '-')}</span></div>
                ${deps.length > 0 ? `<div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_deps') || 'Dependencies'}:</span> <span>${deps.map(d => `<span class="sk-dep-tag">${esc(d)}</span>`).join(' ')}</span></div>` : ''}
                ${vaultKeys.length > 0 ? `<div class="sk-detail-row"><span class="sk-detail-label">${t('skills.detail_vault') || 'Vault Keys'}:</span> <span>${vaultKeys.map(k => `<code>${esc(k)}</code>`).join(', ')}</span></div>` : ''}
            </div>
            ${secHTML}`;
        if (typeof applyI18n === 'function') applyI18n();
    } catch (e) {
        body.innerHTML = `<p class="sk-error">${t('common.error') || 'Error'}</p>`;
    }
}

// eslint-disable-next-line no-unused-vars
function closeDetailModal() {
    document.getElementById('detail-modal').classList.remove('active');
    currentDetailId = '';
}

// eslint-disable-next-line no-unused-vars
async function verifyCurrentSkill() {
    if (!currentDetailId) return;
    const btn = document.getElementById('detail-verify-btn');
    btn.disabled = true;
    btn.textContent = t('common.loading') || 'Scanning...';

    try {
        const resp = await fetch(`/api/skills/${encodeURIComponent(currentDetailId)}/verify`, { method: 'POST' });
        const data = await resp.json();
        if (data.status === 'scanned') {
            showToast(t('skills.scan_complete') || 'Scan complete', 'success');
            showDetail(currentDetailId);
            loadSkills();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    } catch (e) {
        showToast(t('common.error') || 'Error', 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = t('skills.btn_verify') || 'Re-Scan';
    }
}

// ── Code Editor Modal (CodeMirror 6) ────────────────────────────────────────

function createEditorState(code, readOnly) {
    const cm = window.CM6;
    const extensions = [
        cm.lineNumbers(),
        cm.highlightActiveLineGutter(),
        cm.highlightSpecialChars(),
        cm.history(),
        cm.foldGutter(),
        cm.drawSelection(),
        cm.dropCursor(),
        cm.indentOnInput(),
        cm.syntaxHighlighting(cm.defaultHighlightStyle, { fallback: true }),
        cm.bracketMatching(),
        cm.closeBrackets(),
        cm.autocompletion(),
        cm.rectangularSelection(),
        cm.crosshairCursor(),
        cm.highlightActiveLine(),
        cm.highlightSelectionMatches(),
        cm.keymap.of([
            ...cm.closeBracketsKeymap,
            ...cm.defaultKeymap,
            ...cm.searchKeymap,
            ...cm.historyKeymap,
            ...cm.foldKeymap,
            ...cm.completionKeymap,
            cm.indentWithTab,
        ]),
        cm.python(),
        cm.oneDark,
    ];
    if (readOnly) {
        extensions.push(cm.EditorState.readOnly.of(true));
    }
    return cm.EditorState.create({ doc: code, extensions });
}

// eslint-disable-next-line no-unused-vars
function openCodeEditor(id, code, readOnly) {
    codeEditorSkillId = id;
    const container = document.getElementById('code-editor-container');
    container.innerHTML = '';

    if (codeEditorView) {
        codeEditorView.destroy();
        codeEditorView = null;
    }

    const state = createEditorState(code || '', readOnly);
    codeEditorView = new window.CM6.EditorView({
        state,
        parent: container,
    });

    const saveBtn = document.getElementById('code-save-btn');
    saveBtn.style.display = readOnly ? 'none' : '';

    document.getElementById('code-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
async function viewCode(id) {
    document.getElementById('code-modal').classList.add('active');
    const container = document.getElementById('code-editor-container');
    container.innerHTML = `<p style="padding:16px;color:var(--text-secondary);">${t('common.loading') || 'Loading...'}</p>`;

    try {
        const resp = await fetch(`/api/skills/${encodeURIComponent(id)}?code=true`);
        const data = await resp.json();
        if (data.status === 'ok' && data.code) {
            container.innerHTML = '';
            openCodeEditor(id, data.code, false);
        } else {
            container.innerHTML = `<p style="padding:16px;color:var(--text-secondary);">${t('skills.no_code') || 'Code not available'}</p>`;
        }
    } catch (e) {
        container.innerHTML = '<p style="padding:16px;color:var(--text-secondary);">Failed to load code</p>';
    }
}

// eslint-disable-next-line no-unused-vars
async function saveSkillCode() {
    if (!codeEditorView || !codeEditorSkillId) return;
    const code = codeEditorView.state.doc.toString();
    const btn = document.getElementById('code-save-btn');
    btn.disabled = true;
    btn.textContent = t('common.loading') || 'Saving...';

    try {
        const resp = await fetch(`/api/skills/${encodeURIComponent(codeEditorSkillId)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ code })
        });
        const data = await resp.json();
        if (data.status === 'ok') {
            showToast(t('skills.code_saved') || 'Code saved', 'success');
            await loadSkills();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    } catch (e) {
        showToast(t('common.error') || 'Error', 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = t('skills.btn_save_code') || 'Save';
    }
}

// eslint-disable-next-line no-unused-vars
function closeCodeModal() {
    document.getElementById('code-modal').classList.remove('active');
    if (codeEditorView) {
        codeEditorView.destroy();
        codeEditorView = null;
    }
    codeEditorSkillId = '';
}

// ── Upload Modal ────────────────────────────────────────────────────────────

function initDropzone() {
    const dz = document.getElementById('sk-dropzone');
    if (!dz) return;

    dz.addEventListener('click', () => document.getElementById('upload-file').click());
    dz.addEventListener('dragover', (e) => { e.preventDefault(); dz.classList.add('sk-dropzone-active'); });
    dz.addEventListener('dragleave', () => dz.classList.remove('sk-dropzone-active'));
    dz.addEventListener('drop', (e) => {
        e.preventDefault();
        dz.classList.remove('sk-dropzone-active');
        if (e.dataTransfer.files.length > 0) {
            selectFile(e.dataTransfer.files[0]);
        }
    });
}

// eslint-disable-next-line no-unused-vars
function handleFileSelect(event) {
    if (event.target.files.length > 0) {
        selectFile(event.target.files[0]);
    }
}

function selectFile(file) {
    if (!file.name.endsWith('.py')) {
        showToast(t('skills.upload_py_only') || 'Only .py files are allowed', 'error');
        return;
    }
    selectedFile = file;
    document.getElementById('sk-dropzone').style.display = 'none';
    document.getElementById('sk-selected-file').style.display = '';
    document.getElementById('sk-selected-name').textContent = file.name;
    document.getElementById('upload-submit-btn').disabled = false;
}

// eslint-disable-next-line no-unused-vars
function clearFileSelection() {
    selectedFile = null;
    document.getElementById('sk-dropzone').style.display = '';
    document.getElementById('sk-selected-file').style.display = 'none';
    document.getElementById('upload-submit-btn').disabled = true;
    document.getElementById('upload-file').value = '';
}

// eslint-disable-next-line no-unused-vars
function showUploadModal() {
    clearFileSelection();
    document.getElementById('upload-name').value = '';
    document.getElementById('upload-description').value = '';
    document.getElementById('upload-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
function closeUploadModal() {
    document.getElementById('upload-modal').classList.remove('active');
}

// eslint-disable-next-line no-unused-vars
async function submitUpload() {
    if (!selectedFile) return;
    const btn = document.getElementById('upload-submit-btn');
    btn.disabled = true;
    btn.textContent = t('common.loading') || 'Uploading...';

    const fd = new FormData();
    fd.append('file', selectedFile);
    fd.append('name', document.getElementById('upload-name').value.trim());
    fd.append('description', document.getElementById('upload-description').value.trim());

    try {
        const resp = await fetch('/api/skills/upload', { method: 'POST', body: fd });
        const data = await resp.json();

        if (data.status === 'uploaded' || data.status === 'created') {
            showToast(t('skills.upload_success') || 'Skill uploaded', 'success');
            closeUploadModal();
            await loadSkills();
        } else if (data.status === 'rejected') {
            const msgs = (data.validation && data.validation.Errors) ? data.validation.Errors.join(', ') : (data.message || 'Validation failed');
            showToast(msgs, 'error');
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    } catch (e) {
        showToast(t('common.error') || 'Error', 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = t('skills.btn_upload') || 'Upload';
    }
}

// ── Template Modal ──────────────────────────────────────────────────────────

function populateTemplateSelect() {
    const sel = document.getElementById('template-select');
    if (!sel) return;
    // Keep the first "choose" option
    while (sel.options.length > 1) sel.remove(1);
    allTemplates.forEach(tmpl => {
        const opt = document.createElement('option');
        opt.value = tmpl.Name || tmpl.name || '';
        opt.textContent = opt.value;
        sel.appendChild(opt);
    });
}

// eslint-disable-next-line no-unused-vars
function onTemplateSelect() {
    const sel = document.getElementById('template-select');
    const descDiv = document.getElementById('template-description');
    const btn = document.getElementById('template-submit-btn');
    const tmpl = allTemplates.find(t => (t.Name || t.name) === sel.value);

    if (tmpl) {
        descDiv.textContent = tmpl.Description || tmpl.description || '';
        descDiv.style.display = '';
        btn.disabled = false;
    } else {
        descDiv.style.display = 'none';
        btn.disabled = true;
    }
}

// eslint-disable-next-line no-unused-vars
function showTemplateModal() {
    document.getElementById('template-skill-name').value = '';
    document.getElementById('template-description-input').value = '';
    document.getElementById('template-select').selectedIndex = 0;
    document.getElementById('template-description').style.display = 'none';
    document.getElementById('template-submit-btn').disabled = true;
    document.getElementById('template-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
function closeTemplateModal() {
    document.getElementById('template-modal').classList.remove('active');
}

// eslint-disable-next-line no-unused-vars
async function submitTemplate() {
    const templateName = document.getElementById('template-select').value;
    const skillName = document.getElementById('template-skill-name').value.trim();
    const description = document.getElementById('template-description-input').value.trim();

    if (!templateName || !skillName) {
        showToast(t('skills.template_required') || 'Template and skill name are required', 'error');
        return;
    }

    const btn = document.getElementById('template-submit-btn');
    btn.disabled = true;
    btn.textContent = t('common.loading') || 'Creating...';

    try {
        const resp = await fetch('/api/skills/templates', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                template_name: templateName,
                skill_name: skillName,
                description: description
            })
        });
        const data = await resp.json();

        if (data.status === 'created') {
            showToast(t('skills.template_success') || 'Skill created from template', 'success');
            closeTemplateModal();
            await loadSkills();
            // Open the code editor for the newly created skill
            if (data.skill_id) {
                viewCode(data.skill_id);
            }
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    } catch (e) {
        showToast(t('common.error') || 'Error', 'error');
    } finally {
        btn.disabled = false;
        btn.textContent = t('skills.btn_create') || 'Create';
    }
}

// ── Delete Modal ────────────────────────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
function showDeleteSkillModal(id, name) {
    deleteTargetId = id;
    document.getElementById('delete-skill-name').textContent = name;
    document.getElementById('delete-files-checkbox').checked = true;
    document.getElementById('delete-skill-modal').classList.add('active');
}

// eslint-disable-next-line no-unused-vars
function closeDeleteSkillModal() {
    document.getElementById('delete-skill-modal').classList.remove('active');
    deleteTargetId = '';
}

// eslint-disable-next-line no-unused-vars
async function confirmDeleteSkill() {
    if (!deleteTargetId) return;
    const deleteFiles = document.getElementById('delete-files-checkbox').checked;

    try {
        const resp = await fetch(`/api/skills/${encodeURIComponent(deleteTargetId)}?delete_files=${deleteFiles}`, { method: 'DELETE' });
        const data = await resp.json();
        if (data.status === 'deleted') {
            showToast(t('skills.delete_success') || 'Skill deleted', 'success');
            closeDeleteSkillModal();
            closeDetailModal();
            await loadSkills();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    } catch (e) {
        showToast(t('common.error') || 'Error', 'error');
    }
}

// ── Toast helper ────────────────────────────────────────────────────────────

function showToast(msg, type) {
    let c = document.getElementById('sk-toast-container');
    if (!c) {
        c = document.createElement('div');
        c.id = 'sk-toast-container';
        c.className = 'sk-toast-container';
        document.body.appendChild(c);
    }
    const el = document.createElement('div');
    el.className = `sk-toast sk-toast-${type || 'info'}`;
    el.textContent = msg;
    c.appendChild(el);
    setTimeout(() => { el.classList.add('sk-toast-out'); setTimeout(() => el.remove(), 300); }, 3500);
}

// ── Vault Key Assignment Modal ───────────────────────────────────────────────

// eslint-disable-next-line no-unused-vars
async function openVaultKeyModal(id) {
    vaultKeyTargetId = id;
    const listEl = document.getElementById('vault-key-list');
    const emptyEl = document.getElementById('vault-key-empty');
    listEl.innerHTML = `<p>${t('common.loading') || 'Loading...'}</p>`;
    emptyEl.style.display = 'none';
    document.getElementById('vault-key-modal').classList.add('active');

    try {
        // Load available vault secrets and current skill in parallel
        const [vaultResp, skillResp] = await Promise.all([
            fetch('/api/vault/secrets'),
            fetch(`/api/skills/${encodeURIComponent(id)}`)
        ]);
        const vaultData = await vaultResp.json();
        const skillData = await skillResp.json();

        allVaultSecrets = (vaultData.secrets || []).map(s => s.key || s);
        const currentKeys = skillData.skill ? (skillData.skill.VaultKeys || skillData.skill.vault_keys || []) : [];

        if (allVaultSecrets.length === 0) {
            listEl.innerHTML = '';
            emptyEl.style.display = '';
            return;
        }

        listEl.innerHTML = allVaultSecrets.map(key => {
            const checked = currentKeys.includes(key) ? 'checked' : '';
            return `<label class="sk-vault-checkbox-row">
                <input type="checkbox" class="sk-vault-checkbox" value="${esc(key)}" ${checked}>
                <code class="sk-vault-key-tag">${esc(key)}</code>
            </label>`;
        }).join('');
    } catch (e) {
        listEl.innerHTML = `<p class="sk-error">${t('common.error') || 'Error loading secrets'}</p>`;
    }
}

// eslint-disable-next-line no-unused-vars
function closeVaultKeyModal() {
    document.getElementById('vault-key-modal').classList.remove('active');
    vaultKeyTargetId = '';
}

// eslint-disable-next-line no-unused-vars
async function saveVaultKeys() {
    if (!vaultKeyTargetId) return;
    const checkboxes = document.querySelectorAll('#vault-key-list .sk-vault-checkbox:checked');
    const selectedKeys = Array.from(checkboxes).map(cb => cb.value);

    try {
        const resp = await fetch(`/api/skills/${encodeURIComponent(vaultKeyTargetId)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ vault_keys: selectedKeys })
        });
        const data = await resp.json();
        if (data.status === 'ok') {
            showToast(t('skills.vault_save_success') || 'Secrets updated', 'success');
            closeVaultKeyModal();
            await loadSkills();
        } else {
            showToast(data.message || t('common.error'), 'error');
        }
    } catch (e) {
        showToast(t('common.error') || 'Error', 'error');
    }
}
