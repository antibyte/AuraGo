let mcpServersCache = null;

async function renderMCPSection(section) {
    if (mcpServersCache === null) {
        try {
            const resp = await fetch('/api/mcp-servers');
            mcpServersCache = resp.ok ? await resp.json() : [];
        } catch (_) { mcpServersCache = []; }
    }
    const mcpEnabled = configData.mcp && configData.mcp.enabled;
    const allowMcp = configData.agent && configData.agent.allow_mcp;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    if (!allowMcp) {
        html += `<div class="wh-notice mcp-notice-danger">
            <span>🔒</span>
            <div>
                <strong>${t('config.mcp.locked_notice')}</strong><br>
                <small>${t('config.mcp.locked_desc')}</small>
            </div>
        </div></div>`;
        document.getElementById('content').innerHTML = html;
        return;
    }

    html += `<div class="cfg-toggle-row-highlight">
        <span class="cfg-toggle-label">${t('config.mcp.enabled_label')}</span>
        <div class="toggle ${mcpEnabled ? 'on' : ''}" data-path="mcp.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    if (!mcpEnabled) {
        html += `<div class="wh-notice">
            <span>⚠️</span>
            <div>
                <strong>${t('config.mcp.disabled_notice')}</strong><br>
                <small>${t('config.mcp.disabled_desc')}</small>
            </div>
        </div>`;
    }

    html += `<div class="mcp-action-row">
        <button class="btn-save cfg-save-btn-sm" onclick="mcpServerAdd()">
            ＋ ${t('config.mcp.new_server')}
        </button>
    </div>
    <div id="mcp-servers-list"></div>
    <div id="mcp-servers-empty" class="mcp-empty-state is-hidden">
        ${t('config.mcp.empty')}
    </div>
    </div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    mcpServerRenderCards();
}

function mcpServerRenderCards() {
    const wrap = document.getElementById('mcp-servers-list');
    const empty = document.getElementById('mcp-servers-empty');
    if (!wrap) return;
    if (mcpServersCache.length === 0) {
        wrap.innerHTML = '';
        if (empty) empty.classList.remove('is-hidden');
        return;
    }
    if (empty) empty.classList.add('is-hidden');

    let html = '';
    mcpServersCache.forEach((s, idx) => {
        const enabledBadge = s.enabled
            ? `<span class="mcp-badge mcp-badge-active">✅ ${t('config.mcp.active_badge')}</span>`
            : `<span class="mcp-badge mcp-badge-inactive">⏸ ${t('config.mcp.inactive_badge')}</span>`;
        const argsStr = (s.args || []).join(' ');
        const envCount = s.env ? Object.keys(s.env).length : 0;

        html += `
        <div class="mcp-card" data-idx="${idx}">
            <div class="mcp-card-header">
                <div class="mcp-card-title">
                    🔌 ${escapeAttr(s.name || '—')}${enabledBadge}
                </div>
                <div class="mcp-card-actions">
                    <button onclick="mcpServerEdit(${idx})" class="mcp-card-btn mcp-card-btn-edit" title="Edit">✏️</button>
                    <button onclick="mcpServerDelete(${idx})" class="mcp-card-btn mcp-card-btn-delete" title="Delete">🗑️</button>
                </div>
            </div>
            <div class="mcp-card-grid">
                <div><span class="mcp-grid-label">Command:</span> <code>${escapeAttr(s.command || '—')}</code></div>
                <div><span class="mcp-grid-label">Args:</span> ${argsStr ? '<code>' + escapeAttr(argsStr) + '</code>' : '—'}</div>
                <div><span class="mcp-grid-label">Env vars:</span> ${envCount}</div>
            </div>
        </div>`;
    });
    wrap.innerHTML = html;
}

function mcpServerAdd() {
    mcpServerShowModal({name:'', command:'', args:[], env:{}, enabled:true}, -1);
}

function mcpServerEdit(idx) {
    mcpServerShowModal({...mcpServersCache[idx]}, idx);
}

async function mcpServerDelete(idx) {
    const s = mcpServersCache[idx];
    if (!await showConfirm(t('config.mcp.delete_confirm', {name: s.name}))) return;
    mcpServersCache.splice(idx, 1);
    await mcpServerSave();
    mcpServerRenderCards();
}

function mcpServerShowModal(data, idx) {
    const isEdit = idx >= 0;
    const argsStr = (data.args || []).join('\n');
    const envStr = data.env ? Object.entries(data.env).map(([k,v]) => k + '=' + v).join('\n') : '';

    const overlay = document.createElement('div');
    overlay.className = 'mcp-modal-overlay';
    overlay.innerHTML = `
    <div class="mcp-modal-box">
        <div class="mcp-modal-title">${isEdit ? t('config.mcp.edit_server') : t('config.mcp.new_server')}</div>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">Name</span>
            <input id="mcp-m-name" class="field-input cfg-input-full" value="${escapeAttr(data.name)}" placeholder="my-server">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">Command</span>
            <input id="mcp-m-command" class="field-input cfg-input-full" value="${escapeAttr(data.command)}" placeholder="npx">
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">Args <small class="mcp-modal-hint">(${t('config.mcp.args_hint')})</small></span>
            <textarea id="mcp-m-args" class="field-input mcp-modal-textarea" rows="3" placeholder="-y\n@my/mcp-server">${escapeAttr(argsStr)}</textarea>
        </label>
        <label class="mcp-modal-label">
            <span class="mcp-modal-label-text">Environment Variables <small class="mcp-modal-hint">(KEY=VALUE, ${t('config.mcp.env_hint')})</small></span>
            <textarea id="mcp-m-env" class="field-input mcp-modal-textarea" rows="3" placeholder="API_KEY=xxx\nDEBUG=1">${escapeAttr(envStr)}</textarea>
        </label>
        <label class="mcp-modal-check-row">
            <input id="mcp-m-enabled" type="checkbox" ${data.enabled ? 'checked' : ''}>
            <span class="mcp-modal-check-text">${t('config.mcp.enabled_checkbox')}</span>
        </label>
        <div class="mcp-modal-footer">
            <button class="btn-save mcp-btn-cancel" onclick="this.closest('.mcp-modal-overlay').remove()">${t('config.mcp.cancel')}</button>
            <button class="btn-save mcp-btn-save" id="mcp-m-save">${t('config.mcp.save')}</button>
        </div>
    </div>`;
    document.body.appendChild(overlay);
    overlay.addEventListener('click', e => { if (e.target === overlay) overlay.remove(); });

    document.getElementById('mcp-m-save').addEventListener('click', async () => {
        const entry = {
            name: document.getElementById('mcp-m-name').value.trim(),
            command: document.getElementById('mcp-m-command').value.trim(),
            args: document.getElementById('mcp-m-args').value.split('\n').map(l => l.trim()).filter(Boolean),
            env: {},
            enabled: document.getElementById('mcp-m-enabled').checked
        };
        document.getElementById('mcp-m-env').value.split('\n').forEach(line => {
            const eq = line.indexOf('=');
            if (eq > 0) entry.env[line.substring(0, eq).trim()] = line.substring(eq + 1).trim();
        });
        if (!entry.name || !entry.command) {
            showToast(t('config.mcp.name_command_required'), 'warn');
            return;
        }
        if (isEdit) {
            mcpServersCache[idx] = entry;
        } else {
            if (mcpServersCache.some(s => s.name === entry.name)) {
                showToast(t('config.mcp.name_exists'), 'warn');
                return;
            }
            mcpServersCache.push(entry);
        }
        await mcpServerSave();
        overlay.remove();
        mcpServerRenderCards();
    });
}

async function mcpServerSave() {
    try {
        const resp = await fetch('/api/mcp-servers', {
            method: 'PUT',
            headers: {'Content-Type': 'application/json'},
            body: JSON.stringify(mcpServersCache)
        });
        if (!resp.ok) throw new Error(await resp.text());
        const reload = await fetch('/api/mcp-servers');
        if (reload.ok) mcpServersCache = await reload.json();
    } catch (e) {
        showToast(t('config.common.error') + ': ' + e.message, 'error');
    }
}
