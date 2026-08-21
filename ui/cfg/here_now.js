// cfg/here_now.js — native here.now integration section

let hereNowStatusCache = null;
let hereNowAccountsCache = null;

function hereNowAccountRows(payload) {
    if (Array.isArray(payload)) return payload;
    if (!payload || typeof payload !== 'object') return [];
    if (Array.isArray(payload.accounts)) return payload.accounts;
    if (Array.isArray(payload.items)) return payload.items;
    if (payload.accounts && Array.isArray(payload.accounts.items)) return payload.accounts.items;
    return [];
}

function hereNowActiveAccountRows(payload) {
    return hereNowAccountRows(payload).filter(account => {
        const status = String(account.status || '').toLowerCase();
        const provisioning = String(account.provisioningStatus || '').toLowerCase();
        return (status === '' || status === 'active') && (provisioning === '' || provisioning === 'active');
    });
}

function hereNowAccountValue(account) {
    if (String(account.type || '').toLowerCase() === 'personal') return '';
    const selector = account.selector && typeof account.selector === 'object' ? account.selector : {};
    return String(selector.id || account.accountId || account.id || selector.subdomain || account.subdomain || '').trim();
}

function hereNowAccountLabel(account) {
    const value = hereNowAccountValue(account);
    return String(account.displayName || account.name || account.label || value).trim() || value;
}

function hereNowAccountOptions(accounts, currentAccount) {
    const options = [
        `<option value=""${currentAccount === '' ? ' selected' : ''}>${escapeAttr(t('config.here_now.personal_account'))}</option>`,
        ...accounts.map(account => {
            const value = hereNowAccountValue(account);
            if (!value) return '';
            const label = hereNowAccountLabel(account);
            return `<option value="${escapeAttr(value)}"${value === currentAccount ? ' selected' : ''}>${escapeAttr(label)}</option>`;
        })
    ];
    if (currentAccount !== '' && !accounts.some(account => hereNowAccountValue(account) === currentAccount)) {
        options.push(`<option value="${escapeAttr(currentAccount)}" selected disabled>${escapeAttr(currentAccount)}</option>`);
    }
    return options.join('');
}

async function hereNowLoadAccounts() {
    if (hereNowAccountsCache !== null) return hereNowAccountsCache;
    try {
        const resp = await fetch('/api/here-now/accounts', { cache: 'no-store' });
        if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
        hereNowAccountsCache = hereNowActiveAccountRows(await resp.json());
    } catch (_) {
        hereNowAccountsCache = [];
    }
    return hereNowAccountsCache;
}

async function renderHereNowSection(section) {
    if (hereNowStatusCache === null) {
        try {
            const resp = await fetch('/api/here-now/status', { cache: 'no-store' });
            hereNowStatusCache = resp.ok ? await resp.json() : {};
        } catch (_) { hereNowStatusCache = {}; }
    }
    const cfg = configData.here_now || {};
    const enabled = cfg.enabled === true;
    const status = hereNowStatusCache || {};
    const ready = enabled && status.status === 'ready';
    const accounts = ready ? await hereNowLoadAccounts() : [];
    const currentAccount = String(cfg.default_account || '');
    const accountOptions = hereNowAccountOptions(accounts, currentAccount);

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    if (ready) {
        html += `<div class="wh-notice nf-status-ok"><span>✅</span><div><strong>${t('config.here_now.status_ready')}</strong><br><small>${t('config.here_now.status_ready_desc')}</small></div></div>`;
    } else if (enabled && status.status === 'no_key') {
        html += `<div class="wh-notice nf-status-warn"><span>🔑</span><div><strong>${t('config.here_now.no_key')}</strong><br><small>${t('config.here_now.no_key_desc')}</small></div></div>`;
    }

    html += `<div class="cfg-toggle-row-highlight">
        <div class="cfg-toggle-copy"><span class="cfg-toggle-label">${t('config.here_now.enabled')}</span><div class="field-help">${t('config.here_now.enabled_help')}</div></div>
        <div class="toggle ${enabled ? 'on' : ''}" data-path="here_now.enabled" onclick="toggleBool(this)"></div>
    </div>`;

    html += `<div class="field-group"><div class="field-group-title">🔐 ${t('config.here_now.permissions')}</div><div class="field-grid two-cols">`;
    const toggles = [
        ['readonly', 'config.here_now.readonly', 'config.here_now.readonly_help'],
        ['allow_publish', 'config.here_now.allow_publish', 'config.here_now.allow_publish_help'],
        ['allow_site_management', 'config.here_now.allow_site_management', 'config.here_now.allow_site_management_help'],
        ['allow_access_management', 'config.here_now.allow_access_management', 'config.here_now.allow_access_management_help'],
        ['allow_delete', 'config.here_now.allow_delete', 'config.here_now.allow_delete_help']
    ];
    for (const [key, label, help] of toggles) {
        html += `<div class="cfg-toggle-row-compact"><div class="toggle ${cfg[key] === true ? 'on' : ''}" data-path="here_now.${key}" onclick="toggleBool(this)"></div><div class="cfg-toggle-copy"><span class="cfg-toggle-label">${t(label)}</span><div class="field-help">${t(help)}</div></div></div>`;
    }
    html += `</div></div>`;

    html += `<div class="field-group"><div class="field-group-title">🏢 ${t('config.here_now.account')}</div>
        <div class="field-help">${t('config.here_now.account_help')}</div>
        <select class="field-select" data-path="here_now.default_account" ${ready ? '' : 'disabled'}>${accountOptions}</select>
    </div>`;

    const keyPlaceholder = cfgSecretPlaceholder(cfg.api_key);
    html += `<div class="field-group"><div class="field-group-title">🔑 ${t('config.here_now.api_key')}</div><div class="field-help">${t('config.here_now.api_key_help')}</div>
        <label><span class="cfg-label">${t('config.here_now.api_key')} <small class="hp-text-tertiary">🔐 vault</small></span><div class="password-wrap">
            <input class="field-input" type="password" id="hn-api-key" value="${escapeAttr(cfgSecretValue(cfg.api_key))}" placeholder="${escapeAttr(keyPlaceholder)}" autocomplete="off">
            <button type="button" class="password-toggle" data-visible="false" onclick="togglePassword(this)">${EYE_OPEN_SVG}</button>
        </div></label>
        <div class="adg-password-row"><button class="btn-save adg-save-btn" onclick="hereNowSaveKey()">💾 ${t('config.here_now.save_key')}</button><span id="hn-key-status" class="adg-test-result" role="status"></span></div>
    </div>`;

    html += `<div class="field-group"><div class="field-group-title">🔌 ${t('config.here_now.test')}</div><div class="field-help">${t('config.here_now.test_help')}</div>
        <div class="cfg-actions-row"><button class="btn-save adg-test-btn" id="hn-test-btn" onclick="hereNowTestConnection()">🔌 ${t('config.here_now.test_button')}</button><span id="hn-test-result" class="adg-test-result" role="status"></span></div>
    </div></div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
}

async function hereNowSaveKey() {
    const input = document.getElementById('hn-api-key');
    const result = document.getElementById('hn-key-status');
    const value = input ? input.value.trim() : '';
    if (!value) {
        result.textContent = t('config.here_now.key_required');
        result.className = 'adg-test-result is-danger';
        return;
    }
    result.textContent = t('config.here_now.saving');
    result.className = 'adg-test-result';
    try {
        const resp = await fetch('/api/vault/secrets', {
            method: 'POST', headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ key: 'here_now_api_key', value })
        });
        if (!resp.ok) throw new Error(await resp.text());
        cfgMarkSecretStored(input, 'here_now.api_key');
        hereNowStatusCache = null;
        hereNowAccountsCache = null;
        result.textContent = t('config.here_now.key_saved');
        result.className = 'adg-test-result is-success';
    } catch (error) {
        result.textContent = error.message;
        result.className = 'adg-test-result is-danger';
    }
}

async function hereNowTestConnection() {
    const button = document.getElementById('hn-test-btn');
    const result = document.getElementById('hn-test-result');
    if (button) button.disabled = true;
    result.textContent = t('config.here_now.testing');
    result.className = 'adg-test-result';
    try {
        const resp = await fetch('/api/here-now/test-connection', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: '{}' });
        const payload = await resp.json();
        if (!resp.ok || payload.status !== 'ok') throw new Error(payload.message || t('config.here_now.test_failed'));
        hereNowStatusCache = { status: 'ready' };
        hereNowAccountsCache = hereNowActiveAccountRows(payload.accounts);
        const accountSelect = document.querySelector('select[data-path="here_now.default_account"]');
        if (accountSelect) {
            accountSelect.innerHTML = hereNowAccountOptions(hereNowAccountsCache, String((configData.here_now || {}).default_account || ''));
            accountSelect.disabled = false;
        }
        result.textContent = t('config.here_now.test_success');
        result.className = 'adg-test-result is-success';
    } catch (error) {
        result.textContent = error.message;
        result.className = 'adg-test-result is-danger';
    } finally {
        if (button) button.disabled = false;
    }
}

window.addEventListener('aurago:config-saved', () => {
    hereNowStatusCache = null;
    hereNowAccountsCache = null;
});
