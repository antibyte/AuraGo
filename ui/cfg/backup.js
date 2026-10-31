// cfg/backup.js — Backup & Restore section module

// Maps the old tone classes onto the shared result line states.
function backupSetStatus(el, tone, text) {
    if (!el) return;
    const state = { 'is-success': 'success', 'is-danger': 'error', 'is-warning': 'warning', 'is-muted': 'pending' }[tone] || 'pending';
    cfgActionResult.show(el, state, text);
}

function backupSetVaultHintState(hint, hasPassword) {
    if (!hint) return;
    hint.classList.toggle('is-success', hasPassword);
    hint.classList.toggle('is-warning', !hasPassword);
    hint.textContent = hasPassword
        ? t('config.backup.vault_included_hint')
        : t('config.backup.no_password_warning');
}

function renderBackupSection(section) {
    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';
    html += '<div id="backup-restore-status" class="backup-restore-status is-hidden"></div>';

    html += `
    <!-- Tabs -->
    <div class="wh-tabs backup-tabs">
        <button class="wh-tab active" id="backup-tab-create" onclick="backupShowTab('create')">
            ${t('config.backup.tab_create')}
        </button>
        <button class="wh-tab" id="backup-tab-import" onclick="backupShowTab('import')">
            ${t('config.backup.tab_import')}
        </button>
    </div>

    <!-- Create panel -->
    <div class="wh-panel active backup-panel" id="backup-panel-create">

        <div class="bck-card">
            <div class="bck-card-header">
                <span class="bck-card-icon">💾</span>
                <span class="bck-card-title">${t('config.backup.card_create_title')}</span>
            </div>
            <div class="bck-card-body">
                <div class="backup-note-card">
                    <div class="backup-note-text">
                        ${t('config.backup.create_desc')}
                    </div>
                </div>

                <div class="backup-options-list">
                    <label class="backup-option-row">
                        <input type="checkbox" id="backup-opt-vectordb" class="backup-check-input">
                        <span>${t('config.backup.include_vectordb')}</span>
                    </label>
                    <label class="backup-option-row">
                        <input type="checkbox" id="backup-opt-workdir" class="backup-check-input">
                        <span>${t('config.backup.include_workdir')}</span>
                    </label>
                </div>
            </div>
        </div>

        <div class="bck-card">
            <div class="bck-card-header">
                <span class="bck-card-icon">🔐</span>
                <span class="bck-card-title">${t('config.backup.card_encryption_title')}</span>
            </div>
            <div class="bck-card-body">
                <div class="field-group backup-field-group">
                    <div class="field-label">${t('config.backup.password_label')}</div>
                    <div class="field-hint backup-field-hint">${t('config.backup.password_hint')}</div>
                    <div class="password-wrap">
                        <input type="password" id="backup-password" class="field-input" placeholder="${t('config.backup.encryption_placeholder')}" autocomplete="new-password" oninput="backupUpdateVaultHint()">
                        <button type="button" class="password-toggle" onclick="togglePassword(this)" title="${escapeAttr(t('config.backup.toggle_visibility'))}">${EYE_OPEN_SVG}</button>
                    </div>
                    <div id="backup-vault-hint" class="backup-vault-hint is-warning">${t('config.backup.no_password_warning')}</div>
                </div>
            </div>
        </div>

        <div class="backup-actions-row">
            <button class="btn-save backup-primary-btn" onclick="backupCreate()" id="backup-create-btn">
                ${t('config.backup.create_download')}
            </button>
            ${cfgActionResult.html('backup-create-status', 'backup-status-text')}
        </div>
    </div>

    <!-- Import panel -->
    <div class="wh-panel backup-panel" id="backup-panel-import">

        <div class="bck-card">
            <div class="bck-card-header">
                <span class="bck-card-icon">📥</span>
                <span class="bck-card-title">${t('config.backup.card_import_title')}</span>
            </div>
            <div class="bck-card-body">
                <div class="backup-warning-box">
                    ${t('config.backup.import_warning')}
                </div>

                <div class="field-group backup-field-group-file">
                    <div class="field-label">${t('config.backup.select_file')}</div>
                    <input type="file" id="backup-import-file" accept=".ago" class="field-input backup-file-input">
                </div>

                <div class="field-group backup-field-group">
                    <div class="field-label">${t('config.backup.import_password_label')}</div>
                    <div class="password-wrap">
                        <input type="password" id="backup-import-password" class="field-input" placeholder="${t('config.backup.decryption_placeholder')}" autocomplete="off">
                        <button type="button" class="password-toggle" onclick="togglePassword(this)" title="${escapeAttr(t('config.backup.toggle_visibility'))}">${EYE_OPEN_SVG}</button>
                    </div>
                </div>
            </div>
        </div>

        <div class="backup-actions-row">
            <button class="btn-save backup-import-btn" onclick="backupImport()" id="backup-import-btn">
                ${t('config.backup.import_button')}
            </button>
            ${cfgActionResult.html('backup-import-status', 'backup-status-text')}
        </div>

        <div id="backup-import-result" class="backup-import-result is-hidden"></div>
    </div>`;

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    backupLoadRestoreStatus();
}

function backupUpdateVaultHint() {
    const pw = document.getElementById('backup-password');
    const hint = document.getElementById('backup-vault-hint');
    if (!pw || !hint) return;
    backupSetVaultHintState(hint, Boolean(pw.value));
}

function backupShowTab(tab) {
    ['create', 'import'].forEach(tabId => {
        document.getElementById('backup-tab-' + tabId).classList.toggle('active', tabId === tab);
        document.getElementById('backup-panel-' + tabId).classList.toggle('active', tabId === tab);
    });
}

function backupCreate() {
    const btn = document.getElementById('backup-create-btn');

    const payload = {
        password: document.getElementById('backup-password').value || '',
        include_vectordb: document.getElementById('backup-opt-vectordb').checked,
        include_workdir: document.getElementById('backup-opt-workdir').checked,
    };

    // The archive is a binary download: run() saves it and hands the result line a small body.
    return cfgActionResult(btn, async () => {
        const resp = await fetch('/api/backup/create', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!resp.ok) return resp;
        const disposition = resp.headers.get('Content-Disposition') || '';
        const fnMatch = disposition.match(/filename="([^"]+)"/);
        const filename = fnMatch ? fnMatch[1] : 'backup.ago';
        const blob = await resp.blob();
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
        return { ok: true, filename: filename };
    }, {
        target: 'backup-create-status',
        pending: t('config.backup.creating'),
        success: outcome => t('config.backup.status_success') + t('config.backup.downloaded') + (outcome.body && outcome.body.filename ? outcome.body.filename : ''),
        failure: t('config.backup.status_error')
    });
}

// Readable reason for a failed import. body is the parsed JSON (null for a
// non-JSON answer such as a proxy error page); ctx carries the HTTP status.
function backupImportReason(body, ctx) {
    if (body && typeof body === 'object' && (body.error || body.message)) {
        if (body.error === 'password_required') return t('config.backup.password_required');
        if (body.error === 'decryption_failed') return t('config.backup.decryption_failed');
        return String(body.error || body.message);
    }
    const status = ctx && ctx.httpStatus;
    return status ? t('config.backup.error_http', { status: status }) : t('config.backup.unknown_error');
}

function backupRestartButton() {
    const restart = document.createElement('button');
    restart.type = 'button';
    restart.className = 'btn-save backup-restart-btn';
    restart.textContent = '↻ ' + t('config.backup.restart_now');
    restart.addEventListener('click', () => restartAuraGo());
    return restart;
}

// Result box of a staged restore; every server text goes in via textContent.
function backupRenderImportResult(result, data, state) {
    result.replaceChildren();
    result.classList.remove('is-hidden');
    const partial = state === 'warning';
    const box = document.createElement('div');
    box.className = 'backup-import-success-box' + (partial ? ' is-warning' : '');
    const title = document.createElement('div');
    title.className = 'backup-import-success-title';
    title.textContent = (partial ? t('config.backup.status_warning') : t('config.backup.status_success'))
        + ' ' + (partial ? t('config.backup.import_partial') : t('config.backup.import_staged'));
    box.appendChild(title);
    const pending = document.createElement('div');
    pending.className = 'backup-import-success-message';
    pending.textContent = t('config.backup.restore_pending_note');
    box.appendChild(pending);
    if (data.message) {
        const msg = document.createElement('div');
        msg.className = 'backup-import-success-message';
        msg.textContent = String(data.message);
        box.appendChild(msg);
    }
    const warnings = Array.isArray(data.warnings) ? data.warnings.filter(Boolean) : [];
    if (warnings.length) {
        const list = document.createElement('ul');
        list.className = 'backup-import-warnings';
        warnings.forEach(w => {
            const li = document.createElement('li');
            li.textContent = String(w);
            list.appendChild(li);
        });
        box.appendChild(list);
    }
    if (data.restart_required === true) box.appendChild(backupRestartButton());
    result.appendChild(box);
}

async function backupImport() {
    const btn = document.getElementById('backup-import-btn');
    const status = document.getElementById('backup-import-status');
    const result = document.getElementById('backup-import-result');
    const fileInput = document.getElementById('backup-import-file');

    if (!fileInput.files || !fileInput.files[0]) {
        backupSetStatus(status, 'is-warning', t('config.backup.select_file_warning'));
        return { skipped: 'no-file' };
    }

    if (!(await showConfirm(t('config.backup.confirm_import_title'), t('config.backup.confirm_import_message', { name: fileInput.files[0].name })))) return { skipped: 'cancelled' };

    result.classList.add('is-hidden');

    const fd = new FormData();
    fd.append('file', fileInput.files[0]);
    fd.append('password', document.getElementById('backup-import-password').value || '');

    return cfgActionResult(btn, () => fetch('/api/backup/import', { method: 'POST', body: fd }), {
        target: 'backup-import-status',
        pending: t('config.backup.importing'),
        success: t('config.backup.import_staged'),
        warning: t('config.backup.import_partial'),
        failure: t('config.backup.status_error'),
        reasonText: backupImportReason,
        onSuccess: outcome => {
            backupRenderImportResult(result, outcome.body || {}, outcome.state);
            backupLoadRestoreStatus();
        }
    });
}

// Pending / last restore outcome. A server without the endpoint (404) shows nothing.
async function backupLoadRestoreStatus(note) {
    const box = document.getElementById('backup-restore-status');
    if (!box) return null;
    let data = null;
    try {
        const resp = await fetch('/api/backup/restore-status');
        if (!resp.ok) return null;
        data = await resp.json();
    } catch (_) {
        return null;
    }
    if (!data || typeof data !== 'object' || !box.isConnected) return null;
    box.replaceChildren();
    const fmt = value => {
        const d = new Date(value);
        return isNaN(d.getTime()) ? String(value || '') : d.toLocaleString();
    };
    const addLine = (state, text, withRestart) => {
        const line = document.createElement('div');
        line.className = 'backup-restore-status-line';
        const span = document.createElement('span');
        cfgActionResult.show(span, state, text);
        line.appendChild(span);
        if (withRestart) {
            line.appendChild(backupRestartButton());
            const discard = document.createElement('button');
            discard.type = 'button';
            discard.id = 'backup-discard-btn';
            discard.className = 'btn-save backup-restart-btn';
            discard.textContent = t('config.backup.discard_pending');
            discard.addEventListener('click', () => backupDiscardPending(discard));
            line.appendChild(discard);
        }
        box.appendChild(line);
    };
    if (data.pending) {
        addLine('warning', t('config.backup.restore_pending_since', { time: fmt(data.pending_created_at) }), true);
    }
    const last = data.last;
    const RECENT_MS = 7 * 24 * 3600 * 1000;
    if (last && typeof last === 'object') {
        const when = new Date(last.applied_at).getTime();
        if (!isNaN(when) && Date.now() - when <= RECENT_MS) {
            const kind = last.status === 'applied' ? 'applied' : (last.status === 'rolled_back' ? 'rolled_back' : 'failed');
            const state = kind === 'applied' ? 'success' : (kind === 'rolled_back' ? 'warning' : 'error');
            let text = t('config.backup.last_restore_' + kind, { time: fmt(last.applied_at) });
            if (last.message) text += ' ' + String(last.message);
            addLine(state, text, false);
        }
    }
    if (note && note.text) addLine(note.state, note.text, false);
    box.classList.toggle('is-hidden', !box.childNodes.length);
    return data;
}

// Drops a restore that is staged for the next start. Older servers (404/405) answer
// without the endpoint; that is reported, not thrown.
async function backupDiscardPending(btn) {
    if (!(await showConfirm(t('config.backup.discard_confirm_title'), t('config.backup.discard_confirm_message')))) return { skipped: 'cancelled' };
    let note = null;
    const outcome = await cfgActionResult(btn, () => fetch('/api/backup/restore-status', { method: 'DELETE' }), {
        pending: t('config.backup.discarding'),
        success: t('config.backup.discard_done'),
        warning: t('config.backup.discard_nothing'),
        failure: t('config.backup.status_error'),
        interpret: o => {
            if (o.httpStatus === 404 || o.httpStatus === 405) return { state: 'error', reason: t('config.backup.discard_unsupported') };
            if (o.state === 'success' && o.body && o.body.discarded === false) return { state: 'warning', reason: '' };
            return null;
        },
        reasonText: backupImportReason
    });
    if (outcome && outcome.state === 'success') note = { state: 'success', text: t('config.backup.discard_done') };
    else if (outcome && outcome.state === 'warning') note = { state: 'warning', text: t('config.backup.discard_nothing') };
    else if (outcome && outcome.state === 'error') note = { state: 'error', text: t('config.backup.status_error') + ' ' + (outcome.reason || '') };
    await backupLoadRestoreStatus(note);
    return outcome;
}
