let _cydSection = null;
let _cydFlashToken = '';
let _cydFirmware = null;
let _cydEspToolsPromise = null;

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

    html += `<div class="cfg-section-title">${t('config.cyd.flash_title')}</div>`;
    html += `<p class="field-help">${t('config.cyd.flash_help')}</p>`;
    html += `<p class="field-help">${t('config.cyd.flash_erase_hint')}</p>`;
    html += `<div class="field-group">
        <div class="field-label">${t('config.cyd.flash_variant')}</div>
        <select id="cyd-flash-variant" class="field-input" onchange="cydUpdateFlashStatus()">
            <option value="cyd">${t('config.cyd.flash_variant_cyd')}</option>
            <option value="cyd2usb">${t('config.cyd.flash_variant_cyd2usb')}</option>
        </select>
    </div>`;
    html += `<div class="field-group">
        <button type="button" class="btn-save" id="cyd-flash-btn" onclick="cydFlashDisplay()">${t('config.cyd.flash_btn')}</button>
        <div id="cyd-ewt-host" class="cyd-ewt-host"></div>
        <div id="cyd-flash-status" class="field-help"></div>
    </div>`;

    html += `<div class="field-group">
        <div class="field-label">${t('config.cyd.device_url_label')}</div>
        <div class="field-help">${t('help.cyd.device_url')}</div>
        <code id="cyd-device-url" class="cyd-token-code">…</code>
        <button type="button" class="btn-save" id="cyd-copy-url-btn" onclick="cydCopyDeviceUrl()">${t('config.cyd.device_url_copy')}</button>
        <span id="cyd-copy-url-result" class="field-help"></span>
    </div>`;

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
    cydLoadFirmwareStatus();
}

async function cydLoadStatus() {
    const banner = document.getElementById('cyd-status-banner');
    const list = document.getElementById('cyd-devices');
    if (!banner) return;
    try {
        const resp = await fetch('/api/cyd/status');
        if (!resp.ok) throw new Error('status ' + resp.status);
        const data = await resp.json();
        const urlEl = document.getElementById('cyd-device-url');
        if (urlEl && data.device_url) {
            urlEl.textContent = data.device_url;
        }
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

async function cydCopyDeviceUrl() {
    const el = document.getElementById('cyd-device-url');
    const out = document.getElementById('cyd-copy-url-result');
    const url = el ? String(el.textContent || '').trim() : '';
    if (!url || url === '…') {
        if (out) out.textContent = t('config.cyd.device_url_copy_fail');
        return;
    }
    try {
        if (navigator.clipboard && navigator.clipboard.writeText) {
            await navigator.clipboard.writeText(url);
        } else {
            const tmp = document.createElement('textarea');
            tmp.value = url;
            document.body.appendChild(tmp);
            tmp.select();
            document.execCommand('copy');
            document.body.removeChild(tmp);
        }
        if (out) out.textContent = t('config.cyd.device_url_copied');
    } catch (err) {
        if (out) out.textContent = t('config.cyd.device_url_copy_fail');
    }
}

function cydFormatTokenDisplay(token) {
    const raw = String(token || '');
    const body = raw.replace(/^aura_/i, '').replace(/[\s-]/g, '').toUpperCase();
    if (body.length === 9) {
        return body.slice(0, 3) + ' ' + body.slice(3, 6) + ' ' + body.slice(6);
    }
    return raw;
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
        if (token) _cydFlashToken = token;
        const grouped = data.display || cydFormatTokenDisplay(token);
        if (box) {
            box.innerHTML = `<div>${t('config.cyd.token_created')}</div>
                <div class="cyd-token-code"><span class="cyd-token-prefix">aura_</span><code id="cyd-token-value">${escapeAttr(grouped)}</code></div>
                <p class="field-help">${t('config.cyd.token_copy_hint')}</p>`;
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

async function cydLoadFirmwareStatus() {
    try {
        const resp = await fetch('/api/cyd/firmware/status');
        if (!resp.ok) throw new Error('status ' + resp.status);
        _cydFirmware = await resp.json();
    } catch (err) {
        _cydFirmware = null;
    }
    cydUpdateFlashStatus();
}

function cydSelectedVariant() {
    const sel = document.getElementById('cyd-flash-variant');
    return sel && sel.value ? sel.value : 'cyd';
}

function cydVariantInfo(id) {
    const list = (_cydFirmware && _cydFirmware.variants) || [];
    for (let i = 0; i < list.length; i++) {
        if (list[i] && list[i].id === id) return list[i];
    }
    return null;
}

function cydUpdateFlashStatus() {
    const el = document.getElementById('cyd-flash-status');
    const btn = document.getElementById('cyd-flash-btn');
    if (!el) return;
    const sel = document.getElementById('cyd-flash-variant');
    if (sel && _cydFirmware && _cydFirmware.variants) {
        for (let i = 0; i < sel.options.length; i++) {
            const info = cydVariantInfo(sel.options[i].value);
            sel.options[i].disabled = !info || !info.available;
        }
    }
    if (!window.isSecureContext) {
        el.textContent = t('config.cyd.flash_unsupported');
        if (btn) btn.disabled = true;
        return;
    }
    if (!('serial' in navigator)) {
        el.textContent = t('config.cyd.flash_unsupported');
        if (btn) btn.disabled = true;
        return;
    }
    const info = cydVariantInfo(cydSelectedVariant());
    if (!info || !info.available) {
        el.textContent = t('config.cyd.flash_no_firmware');
        if (btn) btn.disabled = true;
        return;
    }
    if (btn) btn.disabled = false;
    el.textContent = t('config.cyd.flash_ready')
        .replace('{version}', info.version || 'dev')
        .replace('{variant}', info.id);
}

async function cydEnsureToken() {
    if (_cydFlashToken) return _cydFlashToken;
    await cydCreateToken();
    if (!_cydFlashToken) throw new Error(t('config.cyd.token_failed'));
    return _cydFlashToken;
}

async function cydEnsureEspWebTools() {
    if (window.customElements && window.customElements.get('esp-web-install-button')) return;
    if (!_cydEspToolsPromise) {
        _cydEspToolsPromise = import('/js/vendor/esp-web-tools/install-button.js');
    }
    await _cydEspToolsPromise;
}

async function cydFlashDisplay() {
    const status = document.getElementById('cyd-flash-status');
    const host = document.getElementById('cyd-ewt-host');
    if (status) status.textContent = t('config.cyd.flash_need_token');
    try {
        await cydLoadFirmwareStatus();
        const variant = cydSelectedVariant();
        const info = cydVariantInfo(variant);
        if (!info || !info.available) throw new Error(t('config.cyd.flash_no_firmware'));
        const token = await cydEnsureToken();
        const urlEl = document.getElementById('cyd-device-url');
        const url = urlEl ? String(urlEl.textContent || '').trim() : '';
        const provResp = await fetch('/api/cyd/firmware/provision', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url: url, token: token })
        });
        if (!provResp.ok) throw new Error(t('config.cyd.flash_no_firmware'));
        const provBuf = await provResp.arrayBuffer();
        const provURL = URL.createObjectURL(new Blob([provBuf], { type: 'application/octet-stream' }));
        const parts = (info.parts || []).map(function (p) {
            return {
                path: window.location.origin + '/api/cyd/firmware/' + encodeURIComponent(variant) + '/' + encodeURIComponent(p.name),
                offset: p.offset
            };
        });
        parts.push({ path: provURL, offset: _cydFirmware.provision_offset || 0x1F0000 });
        const manifest = {
            name: 'AuraGo CYD',
            version: info.version || '0.2.1',
            new_install_prompt_erase: true,
            new_install_improv_wait_time: 0,
            builds: [{ chipFamily: 'ESP32', parts: parts }]
        };
        const manURL = URL.createObjectURL(new Blob([JSON.stringify(manifest)], { type: 'application/json' }));
        await cydEnsureEspWebTools();
        if (host) {
            host.innerHTML = '<esp-web-install-button id="cyd-ewt"><button slot="activate" class="btn-save" id="cyd-ewt-install">' +
                t('config.cyd.flash_install') + '</button><span slot="unsupported">' +
                t('config.cyd.flash_unsupported') + '</span></esp-web-install-button>';
            const ewt = document.getElementById('cyd-ewt');
            if (ewt) ewt.manifest = manURL;
        }
        if (status) status.textContent = t('config.cyd.flash_click_install');
        const install = document.getElementById('cyd-ewt-install');
        if (install) install.click();
    } catch (err) {
        if (status) status.textContent = (err && err.message) ? err.message : t('config.cyd.flash_no_firmware');
    }
}
