// cfg/github.js — GitHub integration section module

let _githubReposData = null;

async function renderGitHubSection(section) {
    const data = configData['github'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const defaultPrivOn = data.default_private === true;
    const allowedRepos = Array.isArray(data.allowed_repos) ? data.allowed_repos : [];

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.github.enabled_label')}</div>
        <div class="toggle-wrap">
            <div class="toggle${enabledOn ? ' on' : ''}" data-path="github.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.github.readonly_label')}</div>
        <div class="field-help">${t('config.github.readonly_hint')}</div>
        <div class="toggle-wrap">
            <div class="toggle${readonlyOn ? ' on' : ''}" data-path="github.readonly" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${readonlyOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.github.owner_label')}</div>
        <div class="field-help">${t('config.github.owner_hint')}</div>
        <input class="field-input" type="text" data-path="github.owner" value="${escapeAttr(data.owner || '')}" placeholder="your-github-username">
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.github.token_label')} <span class="gh-lock-icon">🔒</span></div>
        <div class="field-help">${t('config.github.token_hint')}</div>
        <div class="adg-password-row">
            <div class="password-wrap cfg-password-input">
                <input class="field-input adg-password-input" type="password" id="github-token-input" value="${escapeAttr(cfgSecretValue(data.token))}" placeholder="${escapeAttr(cfgSecretPlaceholder(data.token, 'ghp_••••••••••••••••••••'))}" autocomplete="off">
                    <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
            <button class="btn-save adg-save-btn" onclick="githubSaveToken()">💾 ${t('config.github.save_vault')}</button>
        </div>
        <div id="github-token-status" class="adg-test-result"></div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.github.base_url_label')}</div>
        <div class="field-help">${t('config.github.base_url_hint')}</div>
        <input class="field-input" type="text" data-path="github.base_url" value="${escapeAttr(data.base_url || '')}" placeholder="https://api.github.com">
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.github.default_private_label')}</div>
        <div class="field-help">${t('config.github.default_private_hint')}</div>
        <div class="toggle-wrap">
            <div class="toggle${defaultPrivOn ? ' on' : ''}" data-path="github.default_private" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${defaultPrivOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-group-title">🔐 ${t('config.github.allowed_repos_title')}</div>
        <div class="field-group-desc">
            ${t('config.github.allowed_repos_desc')}
        </div>`;

    const currentAllowed = allowedRepos.join(', ');
    html += `<input type="hidden" id="github-allowed-repos-input" data-path="github.allowed_repos" data-type="array" value="${escapeAttr(currentAllowed)}">`;

    const countLabel = allowedRepos.length === 0
        ? `<span class="gh-count-empty">${t('config.github.no_repos_selected')}</span>`
        : `<span class="gh-count-active">${allowedRepos.length} ${t('config.github.repos_selected_count')}</span>`;

    html += `<div class="gh-actions-row cfg-actions-row">
        <button class="btn-save adg-test-btn" onclick="githubTestConnection()" id="github-test-btn">🔌 ${t('config.github.test_btn')}</button>
        <span id="github-test-result" class="adg-test-result"></span>
        <button class="btn-save btn-secondary" onclick="githubFetchRepos()" id="github-fetch-btn">
            🔄 ${t('config.github.fetch_repos_btn')}
        </button>
        <span id="github-fetch-status" class="adg-test-result"></span>
        <span id="github-allowed-count" class="gh-count">${countLabel}</span>
    </div>`;

    html += `<div id="github-repos-list" class="gh-repos-container">`;

    if (_githubReposData) {
        html += githubBuildRepoList(_githubReposData, allowedRepos);
    } else if (allowedRepos.length > 0) {
        html += `<div class="gh-static-list">
            <div class="cfg-label">${t('config.github.current_allowed_repos')}</div>
            <ul class="gh-static-list">
            ${allowedRepos.map(r => `<li class="gh-static-item">
                <span class="gh-static-name">✓</span>
                <span class="gh-repo-name">${escapeAttr(r)}</span>
            </li>`).join('')}
            </ul>
            <div class="gh-static-hint">${t('config.github.fetch_to_edit_hint')}</div>
        </div>`;
    } else {
        html += `<div class="gh-empty-state">
            <div class="gh-empty-icon">📋</div>
            ${t('config.github.repos_list_empty')}
        </div>`;
    }

    html += `</div>`;
    html += `<div class="gh-note">
        💡 ${t('config.github.agent_created_note')}
    </div>`;
    html += `</div>`;

    html += `</div>`;

    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

function githubBuildRepoList(repos, allowedRepos) {
    if (!repos || repos.length === 0) {
        return `<div class="gh-empty-state">${t('config.github.no_repos_found')}</div>`;
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
            ? `<span class="gh-badge gh-badge-private">🔒 ${t('config.github.badge_private')}</span>`
            : `<span class="gh-badge gh-badge-public">${t('config.github.badge_public')}</span>`;
        const agentBadge = isAgentCreated
            ? `<span class="gh-badge gh-badge-agent">🤖 ${t('config.github.agent_badge')}</span>`
            : '';
        const starStr = stars > 0 ? `<span class="gh-badge-stars">⭐ ${stars}</span>` : '';

        html += `<label class="gh-repo-item">
            <input type="checkbox" ${isAllowed ? 'checked' : ''} ${isAgentCreated ? 'disabled title="' + t('config.github.agent_created_always_allowed') + '"' : ''}
                class="gh-repo-check"
                onchange="githubUpdateAllowedRepos()">
            <div class="gh-repo-info">
                <div class="gh-repo-header">
                    <span class="gh-repo-name" data-repo-name="${escapeAttr(name)}">${escapeAttr(name)}</span>
                    ${privBadge}${agentBadge}${starStr}
                </div>
                ${desc ? `<div class="gh-repo-desc">${escapeAttr(desc)}</div>` : ''}
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
            if (status) {
                status.className = 'adg-test-result is-danger';
                status.textContent = data.message || t('config.github.fetch_error');
            }
            if (btn) { btn.disabled = false; btn.innerHTML = '🔄 ' + t('config.github.fetch_repos_btn'); }
            return;
        }

        _githubReposData = data.repos || [];
        const currentAllowed = (document.getElementById('github-allowed-repos-input') || {}).value || '';
        const allowedArr = currentAllowed.split(',').map(s => s.trim()).filter(Boolean);

        const listEl = document.getElementById('github-repos-list');
        if (listEl) listEl.innerHTML = githubBuildRepoList(_githubReposData, allowedArr);

        githubUpdateAllowedCount(_githubReposData.length, allowedArr.length);
        if (status) {
            status.className = 'adg-test-result is-success';
            status.textContent = _githubReposData.length + ' ' + t('config.github.repos_loaded');
        }
    } catch (e) {
        if (status) {
            status.className = 'adg-test-result is-danger';
            status.textContent = e.message;
        }
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
            const nameEl = cb.closest('label').querySelector('span[data-repo-name]');
            if (nameEl) checkedRepos.push(nameEl.dataset.repoName);
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
        countEl.innerHTML = `<span class="gh-count-empty">${t('config.github.no_repos_selected')}</span>`;
    } else {
        countEl.innerHTML = `<span class="gh-count-active">${allowedCount} / ${total} ${t('config.github.repos_selected_count')}</span>`;
    }
}

async function githubTestConnection() {
    const btn = document.getElementById('github-test-btn');
    const result = document.getElementById('github-test-result');
    if (btn) btn.disabled = true;
    if (result) {
        result.textContent = t('config.github.loading');
        result.className = 'adg-test-result';
    }

    try {
        const resp = await fetch('/api/github/repos');
        const data = resp.ok ? await resp.json() : { status: 'error', message: 'HTTP ' + resp.status };
        if (btn) btn.disabled = false;
        if (!result) return;
        if (data.status === 'error') {
            result.className = 'adg-test-result is-danger';
            result.textContent = t('config.github.status_error') + ' ' + (data.message || t('config.github.test_fail'));
            return;
        }
        result.className = 'adg-test-result is-success';
        result.textContent = t('config.github.status_success') + ' ' + t('config.github.test_ok');
    } catch (_) {
        if (btn) btn.disabled = false;
        if (result) {
            result.className = 'adg-test-result is-danger';
            result.textContent = t('config.github.status_error') + ' ' + t('config.github.test_fail');
        }
    }
}

function githubSaveToken() {
    const input = document.getElementById('github-token-input');
    const statusEl = document.getElementById('github-token-status');
    const token = input ? input.value.trim() : '';
    if (!token) {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.github.token_empty');
        }
        return;
    }

    if (statusEl) {
        statusEl.className = 'adg-test-result';
        statusEl.textContent = t('config.github.saving') || t('config.common.saved');
    }

    fetch('/api/vault/secrets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: 'github_token', value: token })
    })
    .then(r => r.json())
    .then(res => {
        if (res.status === 'ok' || res.success) {
            if (statusEl) {
                statusEl.className = 'adg-test-result is-success';
                statusEl.textContent = t('config.github.token_saved');
            }
            cfgMarkSecretStored(input, 'github.token');
        } else if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = res.message || t('config.github.token_save_failed');
        }
        setTimeout(() => { if (statusEl) statusEl.textContent = ''; }, 4000);
    })
    .catch(() => {
        if (statusEl) {
            statusEl.className = 'adg-test-result is-danger';
            statusEl.textContent = t('config.github.token_save_failed');
        }
    });
}
