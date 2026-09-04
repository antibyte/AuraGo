let _cydSection = null;

async function renderCYDSection(section) {
    if (section) _cydSection = section; else section = _cydSection;
    const data = configData['cyd'] || {};
    const enabled = data.enabled === true;
    const poll = data.poll_seconds || 5;
    const ttl = data.overlay_ttl_seconds || 30;
    const allowControl = data.allow_agent_control !== false;

    let html = `<div class="cfg-section active">
        <div class="section-header">${section.label}</div>
        <div class="section-desc">${section.desc}</div>`;

    html += `<div id="cyd-status-banner" class="adg-status-banner">${t('config.cyd.checking')}</div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.cyd.enabled_label')}</div>
        <div class="field-help">${t('help.cyd.enabled')}</div>
        <div class="toggle-wrap">
            <div class="toggle${enabled ? ' on' : ''}" data-path="cyd.enabled" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${enabled ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.cyd.poll_seconds_label')}</div>
        <div class="field-help">${t('help.cyd.poll_seconds')}</div>
        <input class="field-input" type="number" min="2" max="60" data-path="cyd.poll_seconds" value="${escapeAttr(String(poll))}" placeholder="5">
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.cyd.overlay_ttl_label')}</div>
        <div class="field-help">${t('help.cyd.overlay_ttl')}</div>
        <input class="field-input" type="number" min="5" max="300" data-path="cyd.overlay_ttl_seconds" value="${escapeAttr(String(ttl))}" placeholder="30">
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.cyd.allow_agent_control_label')}</div>
        <div class="field-help">${t('help.cyd.allow_agent_control')}</div>
        <div class="toggle-wrap">
            <div class="toggle${allowControl ? ' on' : ''}" data-path="cyd.allow_agent_control" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${allowControl ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.cyd.mqtt_mirror_label')}</div>
        <div class="field-help">${t('help.cyd.mqtt_mirror')}</div>
        <div class="toggle-wrap">
            <div class="toggle${data.mqtt_mirror ? ' on' : ''}" data-path="cyd.mqtt_mirror" onclick="toggleBool(this)"></div>
            <span class="toggle-label">${data.mqtt_mirror ? t('config.toggle.active') : t('config.toggle.inactive')}</span>
        </div>
    </div>`;

    html += `<hr class="cfg-section-hr">`;
    html += `<div class="cfg-section-title">${t('config.cyd.setup_title')}</div>`;
    html += `<p class="field-help">${t('config.cyd.setup_help')}</p>`;
    html += `<p class="field-help">${t('config.cyd.flash_hint')}</p>`;

    html += `<div class="field-group">
        <button type="button" class="btn-save" id="cyd-create-token-btn" onclick="cydCreateToken()">${t('config.cyd.create_token_btn')}</button>
        <div id="cyd-token-result" class="field-help"></div>
    </div>`;

    html += `<hr class="cfg-section-hr">`;
    html += `<div class="cfg-section-title">${t('config.cyd.devices_title')}</div>`;
    html += `<div id="cyd-devices"></div>`;

    html += `<div class="field-group">
        <button type="button" class="btn-save" id="cyd-test-btn" onclick="cydSendTest()">${t('config.cyd.test_btn')}</button>
        <span id="cyd-test-result" class="field-help"></span>
    </div>`;

    html += `</div>`;
    document.getElementById('content').innerHTML = html;
    attachChangeListeners();
    cydLoadStatus();
}

async function cydLoadStatus() {
    const banner = document.getElementById('cyd-status-banner');
    const list = document.getElementById('cyd-devices');
    if (!banner) return;
    try {
        const resp = await fetch('/api/cyd/status');
        if (!resp.ok) throw new Error('status ' + resp.status);
        const data = await resp.json();
        if (!data.enabled) {
            banner.textContent = t('config.cyd.status_disabled');
        } else if (!data.devices || data.devices.length === 0) {
            banner.textContent = t('config.cyd.status_no_devices');
        } else {
            banner.textContent = t('config.cyd.status_devices').replace('{count}', String(data.devices.length));
        }
        if (list) {
            if (!data.devices || data.devices.length === 0) {
                list.innerHTML = `<p class="field-help">${t('config.cyd.devices_empty')}</p>`;
            } else {
                list.innerHTML = data.devices.map(function (d) {
                    const seen = d.last_seen ? new Date(d.last_seen).toLocaleString() : t('config.cyd.never');
                    return `<div class="field-group"><strong>${escapeAttr(d.name || d.token_id || 'CYD')}</strong>
                        <div class="field-help">${t('config.cyd.firmware')}: ${escapeAttr(d.firmware || '—')} ·
                        ${t('config.cyd.rssi')}: ${escapeAttr(String(d.rssi || '—'))} ·
                        ${t('config.cyd.last_seen')}: ${escapeAttr(seen)}</div></div>`;
                }).join('');
            }
        }
    } catch (err) {
        banner.textContent = t('config.cyd.status_error');
    }
}

async function cydCreateToken() {
    const box = document.getElementById('cyd-token-result');
    if (box) box.textContent = t('config.cyd.token_creating');
    try {
        const resp = await fetch('/api/tokens', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name: 'Cheap Yellow Display', scopes: ['cyd'] })
        });
        const data = await resp.json();
        if (!resp.ok) throw new Error(data.error || data.message || ('HTTP ' + resp.status));
        const token = data.token || data.raw || '';
        if (box) {
            box.innerHTML = `${t('config.cyd.token_created')} <code id="cyd-token-value">${escapeAttr(token)}</code>. ${t('config.cyd.token_copy_hint')}`;
        }
    } catch (err) {
        if (box) box.textContent = t('config.cyd.token_failed') + ' ' + (err && err.message ? err.message : '');
    }
}

async function cydSendTest() {
    const el = document.getElementById('cyd-test-result');
    if (el) el.textContent = t('config.cyd.testing');
    try {
        const resp = await fetch('/api/cyd/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                title: 'AuraGo',
                message: t('config.cyd.test_message'),
                priority: 'normal'
            })
        });
        const data = await resp.json();
        if (!resp.ok) throw new Error(data.error || data.message || ('HTTP ' + resp.status));
        if (el) el.textContent = t('config.cyd.test_ok');
        cydLoadStatus();
    } catch (err) {
        if (el) el.textContent = t('config.cyd.test_fail') + ' ' + (err && err.message ? err.message : '');
    }
}
