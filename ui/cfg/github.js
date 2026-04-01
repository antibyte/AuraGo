// cfg/github.js — GitHub integration section module

let _githubReposData = null; // cached repos fetched from API

async function renderGitHubSection(section) {
    const data = configData['github'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const defaultPrivOn = data.default_private === true;
    const allowedRepos = Array.isArray(data.allowed_repos) ? data.allowed_repos : [];

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    // ── Enabled toggle ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.github.enabled_label')}</div>
        <div class="toggle-wrap">
            <div class="toggle${enabledOn ? ' on' : ''}" data-path="github.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // ── ReadOnly toggle ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.github.readonly_label')}</div>
        <div class="field-hint">${t('config.github.readonly_hint')}</div>
        <div class="toggle-wrap">
            <div class="toggle${readonlyOn ? ' on' : ''}" data-path="github.readonly" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // ── Owner ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.github.owner_label')}</div>
        <div class="field-hint">${t('config.github.owner_hint')}</div>
        <input class="field-input" type="text" data-path="github.owner" value="${escapeAttr(data.owner || '')}" placeholder="your-github-username">
    </div>`;

    // ── API Token (vault) ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.github.token_label')} <span style="font-size:0.65rem;color:var(--warning);">🔒</span></div>
        <div class="field-hint">${t('config.github.token_hint')}</div>
        <div style="display:flex;gap:0.5rem;align-items:center;">
            <div class="password-wrap" style="flex:1;">
                <input class="field-input" type="password" id="github-token-input" placeholder="ghp_••••••••••••••••••••" autocomplete="off">
                    <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
            <button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;white-space:nowrap;" onclick="githubSaveToken()">💾 ${t('config.github.save_vault')}</button>
        </div>
        <div id="github-token-status" style="margin-top:0.4rem;font-size:0.78rem;"></div>
    </div>`;

    // ── Base URL (GitHub Enterprise) ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.github.base_url_label')}</div>
        <div class="field-hint">${t('config.github.base_url_hint')}</div>
        <input class="field-input" type="text" data-path="github.base_url" value="${escapeAttr(data.base_url || '')}" placeholder="https://api.github.com">
    </div>`;

    // ── Default Private toggle ──
    html += `<div class="field-group">
        <div class="field-label">${t('config.github.default_private_label')}</div>
        <div class="field-hint">${t('config.github.default_private_hint')}</div>
        <div class="toggle-wrap">
            <div class="toggle${defaultPrivOn ? ' on' : ''}" data-path="github.default_private" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${defaultPrivOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    // ── Allowed Repos section ──
    html += `<div style="margin-top:1.5rem;padding-top:1.2rem;border-top:1px solid var(--border-subtle);">
        <div style="font-weight:600;font-size:0.95rem;color:var(--accent);margin-bottom:0.5rem;">🔐 ${t('config.github.allowed_repos_title')}</div>
        <div style="font-size:0.82rem;color:var(--text-secondary);line-height:1.6;margin-bottom:1rem;">
            ${t('config.github.allowed_repos_desc')}
        </div>`;

    // Hidden input keeping the allowed repos list (used by global saveConfig)
    const currentAllowed = allowedRepos.join(', ');
    html += `<input type="hidden" id="github-allowed-repos-input" data-path="github.allowed_repos" data-type="array" value="${escapeAttr(currentAllowed)}">`;

    // Count display
    const countLabel = allowedRepos.length === 0
        ? `<span style="color:var(--text-tertiary);">${t('config.github.no_repos_selected')}</span>`
        : `<span style="color:var(--success);">${allowedRepos.length} ${t('config.github.repos_selected_count')}</span>`;

    html += `<div style="display:flex;align-items:center;gap:0.8rem;margin-bottom:0.8rem;flex-wrap:wrap;">
        <button class="btn-save" style="padding:0.45rem 1.1rem;font-size:0.82rem;" onclick="githubFetchRepos()" id="github-fetch-btn">
            🔄 ${t('config.github.fetch_repos_btn')}
        </button>
        <span id="github-fetch-status" style="font-size:0.8rem;color:var(--text-secondary);"></span>
        <span id="github-allowed-count" style="font-size:0.82rem;margin-left:auto;">${countLabel}</span>
    </div>`;

    // Repos list container (scrollable)
    html += `<div id="github-repos-list" style="max-height:360px;overflow-y:auto;border:1px solid var(--border-subtle);border-radius:10px;background:var(--bg-secondary);">`;

    if (_githubReposData) {
        html += githubBuildRepoList(_githubReposData, allowedRepos);
    } else if (allowedRepos.length > 0) {
        // Show current allowed repos as static list until user fetches
        html += `<div style="padding:1rem 1.1rem;font-size:0.82rem;color:var(--text-secondary);">
            <div style="margin-bottom:0.6rem;font-weight:500;">${t('config.github.current_allowed_repos')}</div>
            <ul style="list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:0.3rem;">
            ${allowedRepos.map(r => `<li style="display:flex;align-items:center;gap:0.5rem;font-size:0.82rem;">
                <span style="color:var(--success);">✓</span>
                <span style="font-family:monospace;">${escapeAttr(r)}</span>
            </li>`).join('')}
            </ul>
            <div style="margin-top:0.8rem;color:var(--text-tertiary);font-size:0.78rem;">${t('config.github.fetch_to_edit_hint')}</div>
        </div>`;
    } else {
        html += `<div style="padding:2rem;text-align:center;color:var(--text-tertiary);font-size:0.84rem;">
            <div style="font-size:1.5rem;margin-bottom:0.5rem;">📋</div>
            ${t('config.github.repos_list_empty')}
        </div>`;
    }

    html += `</div>`; // repos-list
    html += `<div style="margin-top:0.6rem;font-size:0.77rem;color:var(--text-tertiary);line-height:1.5;">
        💡 ${t('config.github.agent_created_note')}
    </div>`;
    html += `</div>`; // allowed-repos section

    html += `</div>`; // cfg-section

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function githubBuildRepoList(repos, allowedRepos) {
    if (!repos || repos.length === 0) {
        return `<div style="padding:2rem;text-align:center;color:var(--text-tertiary);font-size:0.84rem;">${t('config.github.no_repos_found')}</div>`;
    }

    const allowedSet = new Set(Array.isArray(allowedRepos) ? allowedRepos : []);
    let html = '';
    repos.forEach(repo => {
        const name = repo.name || '';
        const fullName = repo.full_name || name;
        const desc = repo.description || '';
        const isPrivate = repo.private === true;
        const stars = repo.stargazers_count || 0;
        const isAllowed = allowedSet.has(name);
        const isAgentCreated = repo.agent_created === true;
        const privBadge = isPrivate
            ? `<span style="font-size:0.68rem;padding:0.1rem 0.4rem;border-radius:4px;background:rgba(239,68,68,0.15);color:#f87171;">🔒 private</span>`
            : `<span style="font-size:0.68rem;padding:0.1rem 0.4rem;border-radius:4px;background:rgba(34,197,94,0.12);color:var(--success);">public</span>`;
        const agentBadge = isAgentCreated
            ? `<span style="font-size:0.68rem;padding:0.1rem 0.4rem;border-radius:4px;background:rgba(139,92,246,0.15);color:#a78bfa;">🤖 ${t('config.github.agent_badge')}</span>`
            : '';
        const starStr = stars > 0 ? `<span style="font-size:0.75rem;color:var(--text-tertiary);">⭐ ${stars}</span>` : '';

        html += `<label style="display:flex;align-items:flex-start;gap:0.75rem;padding:0.65rem 1rem;border-bottom:1px solid var(--border-subtle);cursor:pointer;transition:background 0.15s;" onmouseover="this.style.background='var(--bg-tertiary)'" onmouseout="this.style.background=''">
            <input type="checkbox" ${isAllowed ? 'checked' : ''} ${isAgentCreated ? 'disabled title="' + t('config.github.agent_created_always_allowed') + '"' : ''}
                style="accent-color:var(--accent);width:15px;height:15px;margin-top:2px;flex-shrink:0;"
                onchange="githubUpdateAllowedRepos()">
            <div style="flex:1;min-width:0;">
                <div style="display:flex;align-items:center;gap:0.4rem;flex-wrap:wrap;">
                    <span style="font-size:0.84rem;font-weight:500;font-family:monospace;">${escapeAttr(name)}</span>
                    ${privBadge}${agentBadge}${starStr}
                </div>
                ${desc ? `<div style="font-size:0.75rem;color:var(--text-secondary);margin-top:0.15rem;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">${escapeAttr(desc)}</div>` : ''}
            </div>
        </label>`;
    });
    return html;
}

async function githubFetchRepos() {
    const btn = document.getElementById('github-fetch-btn');
    const status = document.getElementById('github-fetch-status');
    if (btn) { btn.disabled = true; btn.innerHTML = '⏳ ' + t('config.github.fetching_repos'); }
    if (status) status.textContent = '';

    try {
        const resp = await fetch('/api/github/repos');
        const data = resp.ok ? await resp.json() : { status: 'error', message: 'HTTP ' + resp.status };

        if (data.status === 'error') {
            if (status) status.textContent = '✗ ' + (data.message || t('config.github.fetch_error'));
            if (btn) { btn.disabled = false; btn.innerHTML = '🔄 ' + t('config.github.fetch_repos_btn'); }
            return;
        }

        _githubReposData = data.repos || [];
        const currentAllowed = (document.getElementById('github-allowed-repos-input') || {}).value || '';
        const allowedArr = currentAllowed.split(',').map(s => s.trim()).filter(Boolean);

        const listEl = document.getElementById('github-repos-list');
        if (listEl) listEl.innerHTML = githubBuildRepoList(_githubReposData, allowedArr);

        githubUpdateAllowedCount(_githubReposData.length, allowedArr.length);
        if (status) status.textContent = '✓ ' + _githubReposData.length + ' ' + t('config.github.repos_loaded');
    } catch (e) {
        if (status) status.textContent = '✗ ' + e.message;
    } finally {
        if (btn) { btn.disabled = false; btn.innerHTML = '🔄 ' + t('config.github.fetch_repos_btn'); }
    }
}

function githubUpdateAllowedRepos() {
    const listEl = document.getElementById('github-repos-list');
    if (!listEl) return;

    const checkedRepos = [];
    const boxes = listEl.querySelectorAll('input[type="checkbox"]');
    boxes.forEach(cb => {
        if (cb.checked && !cb.disabled) {
            // Get repo name from the sibling span
            const nameEl = cb.closest('label').querySelector('span[style*="monospace"]');
            if (nameEl) checkedRepos.push(nameEl.textContent.trim());
        }
    });

    const hiddenInput = document.getElementById('github-allowed-repos-input');
    if (hiddenInput) {
        hiddenInput.value = checkedRepos.join(', ');
        markDirty();
    }

    const total = _githubReposData ? _githubReposData.length : checkedRepos.length;
    githubUpdateAllowedCount(total, checkedRepos.length);
}

function githubUpdateAllowedCount(total, allowedCount) {
    const countEl = document.getElementById('github-allowed-count');
    if (!countEl) return;
    if (allowedCount === 0) {
        countEl.innerHTML = `<span style="color:var(--text-tertiary);">${t('config.github.no_repos_selected')}</span>`;
    } else {
        countEl.innerHTML = `<span style="color:var(--success);">${allowedCount} / ${total} ${t('config.github.repos_selected_count')}</span>`;
    }
}

function githubSaveToken() {
    const input = document.getElementById('github-token-input');
    const statusEl = document.getElementById('github-token-status');
    const token = input ? input.value.trim() : '';
    if (!token) {
        if (statusEl) { statusEl.style.color = 'var(--danger)'; statusEl.textContent = t('config.github.token_empty'); }
        return;
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'github_token', value: token })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            if (statusEl) { statusEl.style.color = 'var(--success)'; statusEl.textContent = '✓ ' + t('config.github.token_saved'); }
            if (input) input.value = '';
        } else {
            if (statusEl) { statusEl.style.color = 'var(--danger)'; statusEl.textContent = '✗ ' + (res.message || t('config.github.token_save_failed')); }
        }
        setTimeout(() => { if (statusEl) statusEl.textContent = ''; }, 4000);
    })
    .catch(() => {
        if (statusEl) { statusEl.style.color = 'var(--danger)'; statusEl.textContent = '✗ ' + t('config.github.token_save_failed'); }
    });
}
