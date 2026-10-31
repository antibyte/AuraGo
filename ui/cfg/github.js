// cfg/github.js — GitHub integration section module

let _githubReposData = null;

async function renderGitHubSection(section) {
    const data = configData['github'] || {};
    const enabledOn = data.enabled === true;
    const readonlyOn = data.readonly === true;
    const allowDeleteOn = data.allow_delete === true;
    const defaultPrivOn = data.default_private === true;
    const allowedRepos = Array.isArray(data.allowed_repos) ? data.allowed_repos : [];

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.github.enabled_label')}</div>
        <div class="field-help">${t('help.github.enabled')}</div>
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
        <div class="field-label">${t('config.github.allow_delete_label')}</div>
        <div class="field-help">${t('config.github.allow_delete_hint')}</div>
        <div class="toggle-wrap">
            <div class="toggle${allowDeleteOn ? ' on' : ''}" data-path="github.allow_delete" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${allowDeleteOn ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
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
                <input class="field-input adg-password-input" type="password" id="github-token-input" data-path="github.token" value="${escapeAttr(cfgSecretValue(data.token))}" placeholder="${escapeAttr(cfgSecretPlaceholder(data.token, 'ghp_••••••••••••••••••••'))}" autocomplete="off">
                    <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
            </div>
        </div>
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
        <button class="btn-save adg-test-btn" onclick="githubTestConnection()" id="github-test-btn" data-config-action-draft="true">🔌 ${t('config.github.test_btn')}</button>
        ${cfgActionResult.html('github-test-result')}
        <button class="btn-save btn-secondary" onclick="githubFetchRepos()" id="github-fetch-btn">
            🔄 ${t('config.github.fetch_repos_btn')}
        </button>
        ${cfgActionResult.html('github-fetch-status')}
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

function githubRepoChecked(fullName, shortName, repoOwner, allowedRepos, configuredOwner) {
    const allowed = new Set((allowedRepos || []).map(entry => String(entry).trim().toLowerCase()).filter(Boolean));
    const full = String(fullName || '').trim().toLowerCase();
    const owner = String(configuredOwner || '').trim().toLowerCase();
    const repo = String(repoOwner || '').trim().toLowerCase();
    if (full && allowed.has(full)) return true;
    const bare = String(shortName || '').trim().toLowerCase();
    return bare !== '' && allowed.has(bare) && owner !== '' && repo === owner;
}

function githubBuildRepoList(repos, allowedRepos) {
    if (!repos || repos.length === 0) {
        return `<div class="gh-empty-state">${t('config.github.no_repos_found')}</div>`;
    }

    const ownerInput = document.querySelector('[data-path="github.owner"]');
    const configuredOwner = ownerInput
        ? ownerInput.value
        : ((typeof configData !== 'undefined' && configData.github && configData.github.owner) || '');
    let html = '';
    repos.forEach(repo => {
        const name = repo.name || '';
        const fullName = repo.full_name || name;
        const desc = repo.description || '';
        const isPrivate = repo.private === true;
        const stars = repo.stargazers_count || 0;
        const isAgentCreated = repo.agent_created === true;
        const isAllowed = isAgentCreated || githubRepoChecked(fullName, name, String(fullName).split('/')[0] || '', allowedRepos, configuredOwner);
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
                    <span class="gh-repo-name" data-repo-name="${escapeAttr(name)}" data-repo-full-name="${escapeAttr(fullName)}">${escapeAttr(name)}</span>
                    ${privBadge}${agentBadge}${starStr}
                </div>
                ${desc ? `<div class="gh-repo-desc">${escapeAttr(desc)}</div>` : ''}
                ${repo.trust_migration_required === true ? `<div class="field-help gh-trust-migration">${t('config.github.trust_migration_hint')}</div>` : ''}
            </div>
        </label>`;
    });
    return html;
}

// Both buttons send the form values (draft); the server answers 200 with status "error" and a message on failure.
function githubReposRequest() {
    return fetch('/api/github/repos', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ draft: cfgDraftPatch(['github']) })
    });
}

function githubFetchRepos() {
    const btn = document.getElementById('github-fetch-btn');
    return cfgActionResult(btn, githubReposRequest, {
        target: 'github-fetch-status',
        pending: t('config.github.fetching_repos'),
        failure: t('config.github.fetch_error'),
        success: outcome => ((outcome.body && outcome.body.repos) || []).length + ' ' + t('config.github.repos_loaded'),
        onSuccess: outcome => {
            _githubReposData = (outcome.body && outcome.body.repos) || [];
            const currentAllowed = (document.getElementById('github-allowed-repos-input') || {}).value || '';
            const allowedArr = currentAllowed.split(',').map(s => s.trim()).filter(Boolean);
            const listEl = document.getElementById('github-repos-list');
            if (listEl) listEl.innerHTML = githubBuildRepoList(_githubReposData, allowedArr);
            githubUpdateAllowedCount(_githubReposData.length, allowedArr.length);
        }
    });
}

function githubUpdateAllowedRepos() {
    const listEl = document.getElementById('github-repos-list');
    const hiddenInput = document.getElementById('github-allowed-repos-input');
    if (!listEl || !hiddenInput) return;
    const ownerInput = document.querySelector('[data-path="github.owner"]');
    const configuredOwner = (ownerInput && ownerInput.value || '').trim().toLowerCase();
    const previous = hiddenInput.value.split(',').map(entry => entry.trim()).filter(Boolean);
    const visible = [];
    listEl.querySelectorAll('input[type="checkbox"]').forEach(box => {
        const nameEl = box.closest('label') && box.closest('label').querySelector('span[data-repo-name]');
        if (!nameEl) return;
        const fullName = nameEl.dataset.repoFullName || nameEl.dataset.repoName;
        const shortName = nameEl.dataset.repoName || '';
        const repoOwner = String(fullName).includes('/') ? String(fullName).split('/')[0] : configuredOwner;
        visible.push({ fullName, shortName, repoOwner, checked: box.checked, disabled: box.disabled });
    });
    const represented = new Set();
    visible.forEach(repo => {
        represented.add(String(repo.fullName).toLowerCase());
        if (String(repo.repoOwner).toLowerCase() === configuredOwner) {
            represented.add(String(repo.shortName).toLowerCase());
        }
    });
    const merged = [];
    const seen = new Set();
    function pushUnique(value) {
        const key = String(value).trim().toLowerCase();
        if (!key || seen.has(key)) return;
        seen.add(key);
        merged.push(String(value).trim());
    }
    previous.forEach(entry => {
        if (!represented.has(entry.toLowerCase())) pushUnique(entry);
    });
    visible.forEach(repo => {
        if (repo.checked && !repo.disabled) pushUnique(repo.fullName);
    });
    hiddenInput.value = merged.join(', ');
    markDirty();
    githubUpdateAllowedCount(_githubReposData ? _githubReposData.length : visible.length, merged.length);
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

function githubTestConnection() {
    const btn = document.getElementById('github-test-btn');
    return cfgActionResult(btn, githubReposRequest, {
        target: 'github-test-result',
        pending: t('config.github.loading'),
        success: t('config.github.test_ok'),
        failure: t('config.github.test_fail')
    });
}
