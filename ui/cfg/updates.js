// cfg/updates.js — Updates section module

function renderUpdatesSection(section) {
    const content = document.getElementById('content');
    content.innerHTML = `
    <div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>
        <div id="updates-body" class="updates-body"></div>
    </div>`;

    renderUpdatesBody();
}

async function renderUpdatesBody() {
    const body = document.getElementById('updates-body');
    if (!body) return;

    body.innerHTML = `
    <div class="updates-warning-card">
        <div class="updates-warning-text">
            ⚠️ ${t('config.updates.warning_banner')}
        </div>
    </div>

    <div id="updates-check-result" class="updates-check-result"></div>

    <div class="updates-actions">
        <button class="btn-save updates-btn" id="updates-check-btn" onclick="updatesCheck()">
            ${t('config.updates.check_button')}
        </button>
        <button class="btn-save updates-btn is-hidden" id="updates-install-btn" onclick="updatesInstall()">
            ${t('config.updates.install_button')}
        </button>
        <span id="updates-status" class="updates-status"></span>
    </div>`;
}

async function updatesCheck() {
    const btn = document.getElementById('updates-check-btn');
    const resultDiv = document.getElementById('updates-check-result');
    const installBtn = document.getElementById('updates-install-btn');
    const status = document.getElementById('updates-status');
    if (!btn || !resultDiv) return;

    btn.disabled = true;
    btn.textContent = t('config.updates.checking');
    status.textContent = '';
    if (installBtn) installBtn.classList.add('is-hidden');

    try {
        const resp = await fetch('/api/updates/check');
        const data = await resp.json();

        if (data.error) {
            resultDiv.innerHTML = `
            <div class="updates-result-card updates-result-error">
                <div class="updates-result-title">❌ ${t('config.updates.error')}</div>
                <div class="updates-result-detail">${data.error}</div>
            </div>`;
        } else if (data.update_available) {
            let changelogHtml = '';
            if (data.changelog) {
                const lines = data.changelog.split('\n').filter(l => l.trim());
                changelogHtml = `
                <div class="updates-changelog-wrap">
                    <div class="updates-changelog-title">${t('config.updates.changelog')}</div>
                    <div class="updates-changelog-box">
                        ${lines.map(l => `<div>${escapeHtml(l)}</div>`).join('')}
                    </div>
                </div>`;
            }
            resultDiv.innerHTML = `
            <div class="updates-result-card updates-result-warn">
                <div class="updates-result-title updates-result-title-warn">🆕 ${t('config.updates.available')}</div>
                <div class="updates-meta-row">
                    ${data.current_version ? `<span>${t('config.updates.current')}: <strong>${data.current_version}</strong></span>` : ''}
                    ${data.latest_version ? `<span>${t('config.updates.latest')}: <strong class="updates-latest-version">${data.latest_version}</strong></span>` : ''}
                    ${data.commit_count ? `<span>${data.commit_count} commit(s)</span>` : ''}
                </div>
                ${changelogHtml}
            </div>`;
            if (installBtn) installBtn.classList.remove('is-hidden');
        } else {
            resultDiv.innerHTML = `
            <div class="updates-result-card updates-result-success">
                <div class="updates-result-title updates-result-title-success">✅ ${t('config.updates.up_to_date')}</div>
                <div class="updates-meta-row updates-meta-row-tight">
                    ${data.current_version ? `<span>${t('config.updates.version')}: <strong>${data.current_version}</strong></span>` : ''}
                    <span>${data.message || ''}</span>
                </div>
            </div>`;
        }
    } catch (e) {
        resultDiv.innerHTML = `<div class="updates-inline-error">❌ ${e.message}</div>`;
    } finally {
        btn.disabled = false;
        btn.textContent = t('config.updates.check_button');
    }
}

async function updatesInstall() {
    const msg = t('config.updates.confirm_install');
    if (!confirm(msg)) return;

    const installBtn = document.getElementById('updates-install-btn');
    const checkBtn = document.getElementById('updates-check-btn');
    const status = document.getElementById('updates-status');
    if (installBtn) { installBtn.disabled = true; installBtn.textContent = t('config.updates.installing'); }
    if (checkBtn) checkBtn.disabled = true;
    if (status) status.textContent = t('config.updates.running');

    try {
        const resp = await fetch('/api/updates/install', { method: 'POST' });
        const data = await resp.json();
        if (data.error) {
            if (status) status.textContent = '❌ ' + data.error;
            if (installBtn) { installBtn.disabled = false; installBtn.textContent = t('config.updates.install_button'); }
            if (checkBtn) checkBtn.disabled = false;
        } else {
            if (status) status.textContent = t('config.updates.started');
            // Auto-reload after 75 seconds
            setTimeout(() => location.reload(), 75000);
        }
    } catch (e) {
        if (status) status.textContent = '❌ ' + e.message;
        if (installBtn) { installBtn.disabled = false; }
        if (checkBtn) checkBtn.disabled = false;
    }
}
