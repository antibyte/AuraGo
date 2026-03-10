// cfg/updates.js — Updates section module

function renderUpdatesSection(section) {
    const content = document.getElementById('section-content');
    content.innerHTML = `
    <div class="cfg-section active">
        <div class="section-header">${section.icon} ${section.label}</div>
        <div class="section-desc">${section.desc}</div>
        <div id="updates-body" style="margin-top:1.5rem;"></div>
    </div>`;

    renderUpdatesBody();
}

async function renderUpdatesBody() {
    const body = document.getElementById('updates-body');
    if (!body) return;

    body.innerHTML = `
    <div style="background:var(--bg-secondary);border:1px solid var(--border-subtle);border-radius:12px;padding:1.25rem;margin-bottom:1.25rem;">
        <div style="font-size:0.8rem;color:var(--text-secondary);line-height:1.6;">
            ⚠️ ${t('config.updates.warning_banner')}
        </div>
    </div>

    <div id="updates-check-result" style="margin-bottom:1.25rem;"></div>

    <div style="display:flex;gap:0.75rem;align-items:center;flex-wrap:wrap;">
        <button class="btn-save" style="padding:0.55rem 1.4rem;font-size:0.85rem;" id="updates-check-btn" onclick="updatesCheck()">
            ${t('config.updates.check_button')}
        </button>
        <button class="btn-save" style="padding:0.55rem 1.4rem;font-size:0.85rem;display:none;" id="updates-install-btn" onclick="updatesInstall()">
            ${t('config.updates.install_button')}
        </button>
        <span id="updates-status" style="font-size:0.8rem;color:var(--text-secondary);"></span>
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
    if (installBtn) installBtn.style.display = 'none';

    try {
        const resp = await fetch('/api/updates/check');
        const data = await resp.json();

        if (data.error) {
            resultDiv.innerHTML = `
            <div style="border:1px solid var(--border-subtle);border-radius:10px;padding:1rem 1.2rem;background:rgba(239,68,68,0.07);">
                <div style="font-weight:600;color:var(--danger);">❌ ${t('config.updates.error')}</div>
                <div style="font-size:0.82rem;color:var(--text-secondary);margin-top:0.3rem;">${data.error}</div>
            </div>`;
        } else if (data.update_available) {
            let changelogHtml = '';
            if (data.changelog) {
                const lines = data.changelog.split('\n').filter(l => l.trim());
                changelogHtml = `
                <div style="margin-top:0.75rem;">
                    <div style="font-size:0.78rem;font-weight:600;color:var(--text-secondary);margin-bottom:0.4rem;">${t('config.updates.changelog')}</div>
                    <div style="font-family:monospace;font-size:0.77rem;color:var(--text-secondary);background:var(--bg-secondary);border-radius:7px;padding:0.65rem 0.85rem;max-height:120px;overflow-y:auto;line-height:1.55;">
                        ${lines.map(l => `<div>${l}</div>`).join('')}
                    </div>
                </div>`;
            }
            resultDiv.innerHTML = `
            <div style="border:1px solid rgba(245,158,11,0.35);border-radius:10px;padding:1rem 1.2rem;background:rgba(245,158,11,0.07);">
                <div style="font-weight:600;color:var(--warning);">🆕 ${t('config.updates.available')}</div>
                <div style="display:flex;gap:1.2rem;margin-top:0.5rem;font-size:0.82rem;color:var(--text-secondary);">
                    ${data.current_version ? `<span>${t('config.updates.current')}: <strong>${data.current_version}</strong></span>` : ''}
                    ${data.latest_version ? `<span>${t('config.updates.latest')}: <strong style="color:var(--accent);">${data.latest_version}</strong></span>` : ''}
                    ${data.commit_count ? `<span>${data.commit_count} commit(s)</span>` : ''}
                </div>
                ${changelogHtml}
            </div>`;
            if (installBtn) installBtn.style.display = '';
        } else {
            resultDiv.innerHTML = `
            <div style="border:1px solid rgba(34,197,94,0.3);border-radius:10px;padding:1rem 1.2rem;background:rgba(34,197,94,0.07);">
                <div style="font-weight:600;color:var(--success,#22c55e);">✅ ${t('config.updates.up_to_date')}</div>
                <div style="display:flex;gap:1.2rem;margin-top:0.4rem;font-size:0.82rem;color:var(--text-secondary);">
                    ${data.current_version ? `<span>${t('config.updates.version')}: <strong>${data.current_version}</strong></span>` : ''}
                    <span>${data.message || ''}</span>
                </div>
            </div>`;
        }
    } catch (e) {
        resultDiv.innerHTML = `<div style="color:var(--danger);font-size:0.85rem;">❌ ${e.message}</div>`;
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
