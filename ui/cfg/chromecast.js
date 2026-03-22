// cfg/chromecast.js — Chromecast device management section module
let ccDevicesCache = [];
let ccDiscoveredCache = [];

function renderChromecastSection(section) {
    const data = configData['chromecast'] || {};
    const enabledOn = data.enabled === true;
    const ttsPort = data.tts_port || 8090;

    let html = '<div class="cfg-section active">';
    html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
    html += '<div class="section-desc">' + section.desc + '</div>';

    // mDNS discovery unavailable banner (Docker bridge network)
    const ccDiscBanner = featureUnavailableBanner('chromecast_discovery');
    if (ccDiscBanner) html += ccDiscBanner;
    const ccDiscBlocked = !!(runtimeData.features && runtimeData.features.chromecast_discovery && !runtimeData.features.chromecast_discovery.available);

    // ── Enabled toggle ──
    const helpEnabled = t('help.chromecast.enabled');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.chromecast.enabled_label') + '</div>';
    if (helpEnabled) html += '<div class="field-help">' + helpEnabled + '</div>';
    html += '<div class="toggle-wrap">';
    html += '<div class="toggle' + (enabledOn ? ' on' : '') + '" data-path="chromecast.enabled" onclick="toggleBool(this)"></div>';
    html += '<span class="toggle-label">' + (enabledOn ? t('config.toggle.active') : t('config.toggle.inactive')) + '</span>';
    html += '</div></div>';

    // ── TTS Port ──
    const helpPort = t('help.chromecast.tts_port');
    html += '<div class="field-group">';
    html += '<div class="field-label">' + t('config.chromecast.tts_port_label') + '</div>';
    if (helpPort) html += '<div class="field-help">' + helpPort + '</div>';
    html += '<input class="field-input" type="number" data-path="chromecast.tts_port" value="' + ttsPort + '" style="width:120px;">';
    html += '</div>';

    // ── Devices section ──
    html += '<div style="margin-top:2rem;margin-bottom:0.5rem;font-weight:600;font-size:0.95rem;color:var(--accent);border-bottom:1px solid var(--border-subtle);padding-bottom:0.4rem;">📺 ' + t('config.chromecast.devices_title') + '</div>';
    html += '<div class="field-help" style="margin-bottom:1rem;">' + t('config.chromecast.devices_desc') + '</div>';

    // Action buttons row
    html += '<div style="display:flex;gap:0.6rem;align-items:center;margin-bottom:1rem;flex-wrap:wrap;">';
    html += '<button class="btn-save" style="padding:0.45rem 1.1rem;font-size:0.82rem;' + (ccDiscBlocked ? 'opacity:0.45;pointer-events:none;' : '') + '" onclick="ccDiscoverDevices()" id="cc-discover-btn">🔍 ' + t('config.chromecast.discover_btn') + '</button>';
    html += '<button class="btn-save" style="padding:0.45rem 1.1rem;font-size:0.82rem;background:var(--bg-tertiary);color:var(--text-primary);border:1px solid var(--border-subtle);" onclick="ccShowAddManual()">＋ ' + t('config.chromecast.add_manual_btn') + '</button>';
    html += '</div>';

    // Discovery results area (hidden initially)
    html += '<div id="cc-discover-area" style="display:none;margin-bottom:1.2rem;"></div>';

    // Device table
    html += `
    <div id="cc-table-wrap" style="overflow-x:auto;display:none;">
        <table style="width:100%;border-collapse:collapse;font-size:0.82rem;">
            <thead>
                <tr style="border-bottom:2px solid var(--border-subtle);text-align:left;">
                    <th style="padding:0.5rem 0.6rem;">${t('config.chromecast.col_name')}</th>
                    <th style="padding:0.5rem 0.6rem;">${t('config.chromecast.col_ip')}</th>
                    <th style="padding:0.5rem 0.6rem;">${t('config.chromecast.col_port')}</th>
                    <th style="padding:0.5rem 0.6rem;">${t('config.chromecast.col_desc')}</th>
                    <th style="padding:0.5rem 0.6rem;text-align:right;">${t('config.chromecast.col_actions')}</th>
                </tr>
            </thead>
            <tbody id="cc-tbody"></tbody>
        </table>
    </div>
    <div id="cc-empty" style="display:none;text-align:center;padding:2rem;color:var(--text-tertiary);font-size:0.85rem;">
        ${t('config.chromecast.no_devices')}
    </div>
    <div id="cc-loading" style="text-align:center;padding:2rem;color:var(--text-secondary);font-size:0.85rem;">
        ${t('config.chromecast.loading')}
    </div>`;

    // ── Edit modal ──
    html += `
    <div id="cc-modal-overlay" style="display:none;position:fixed;inset:0;background:rgba(0,0,0,0.55);z-index:1000;backdrop-filter:blur(4px);" onclick="ccCloseModal(event)">
        <div style="position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);background:var(--bg-primary);border:1px solid var(--border-subtle);border-radius:14px;padding:1.5rem;width:min(480px,90vw);max-height:85vh;overflow-y:auto;" onclick="event.stopPropagation()">
            <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1.2rem;">
                <span id="cc-modal-title" style="font-size:1rem;font-weight:600;"></span>
                <button onclick="ccCloseModal()" style="background:none;border:none;color:var(--text-secondary);font-size:1.2rem;cursor:pointer;">✕</button>
            </div>
            <input type="hidden" id="cc-edit-id">
            <div class="field-group" style="margin-bottom:0.8rem;">
                <div class="field-label">${t('config.chromecast.field_name')} *</div>
                <div class="field-help">${t('config.chromecast.field_name_help')}</div>
                <input type="text" id="cc-field-name" class="field-input" placeholder="${t('config.chromecast.name_placeholder')}">
            </div>
            <div style="display:grid;grid-template-columns:2fr 1fr;gap:0.8rem;">
                <div class="field-group">
                    <div class="field-label">${t('config.chromecast.field_ip')}</div>
                    <input type="text" id="cc-field-ip" class="field-input" placeholder="192.168.1.50">
                </div>
                <div class="field-group">
                    <div class="field-label">Port</div>
                    <input type="number" id="cc-field-port" class="field-input" placeholder="8009" value="8009">
                </div>
            </div>
            <div class="field-group" style="margin-top:0.8rem;">
                <div class="field-label">${t('config.chromecast.field_desc')}</div>
                <input type="text" id="cc-field-desc" class="field-input" placeholder="${t('config.chromecast.desc_placeholder')}">
            </div>
            <div id="cc-modal-error" style="display:none;margin-top:0.7rem;padding:0.5rem 0.8rem;background:rgba(239,68,68,0.1);border:1px solid rgba(239,68,68,0.3);border-radius:8px;font-size:0.8rem;color:var(--danger);"></div>
            <div style="display:flex;justify-content:flex-end;gap:0.6rem;margin-top:1.2rem;">
                <button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;background:var(--bg-tertiary);color:var(--text-primary);border:1px solid var(--border-subtle);" onclick="ccCloseModal()">${t('config.chromecast.cancel')}</button>
                <button class="btn-save" style="padding:0.45rem 1.2rem;font-size:0.82rem;" id="cc-modal-save" onclick="ccSaveDevice()">${t('config.chromecast.save')}</button>
            </div>
        </div>
    </div>`;

    html += '</div>';
    document.getElementById('content').innerHTML = html;
    ccLoadDevices();
}

// ── Load chromecast devices from registry ──
async function ccLoadDevices() {
    const tbody = document.getElementById('cc-tbody');
    const empty = document.getElementById('cc-empty');
    const loading = document.getElementById('cc-loading');
    const table = document.getElementById('cc-table-wrap');
    loading.style.display = 'block';
    table.style.display = 'none';
    empty.style.display = 'none';

    try {
        const resp = await fetch('/api/devices');
        if (!resp.ok) throw new Error(await resp.text());
        const allDevices = await resp.json();
        ccDevicesCache = allDevices.filter(d => d.type === 'chromecast');
    } catch (e) {
        loading.textContent = '❌ ' + e.message;
        return;
    }

    loading.style.display = 'none';
    if (ccDevicesCache.length === 0) {
        empty.style.display = 'block';
        return;
    }
    table.style.display = 'block';
    ccRenderRows(ccDevicesCache);
}

function ccRenderRows(devices) {
    const tbody = document.getElementById('cc-tbody');
    tbody.innerHTML = '';
    devices.forEach(d => {
        const tr = document.createElement('tr');
        tr.style.borderBottom = '1px solid var(--border-subtle)';
        tr.dataset.id = d.id;
        tr.innerHTML = `
            <td style="padding:0.5rem 0.6rem;font-weight:500;">${escapeHtml(d.name)}</td>
            <td style="padding:0.5rem 0.6rem;font-family:monospace;font-size:0.8rem;">${escapeHtml(d.ip_address || '—')}</td>
            <td style="padding:0.5rem 0.6rem;">${d.port || 8009}</td>
            <td style="padding:0.5rem 0.6rem;color:var(--text-secondary);font-size:0.8rem;">${escapeHtml(d.description || '—')}</td>
            <td style="padding:0.5rem 0.6rem;text-align:right;white-space:nowrap;">
                <button onclick="ccShowEditModal('${d.id}')" style="background:none;border:none;cursor:pointer;font-size:0.9rem;" title="${t('config.chromecast.edit_tooltip')}">✏️</button>
                <button onclick="ccDeleteDevice('${d.id}','${escapeHtml(d.name)}')" style="background:none;border:none;cursor:pointer;font-size:0.9rem;margin-left:0.3rem;" title="${t('config.chromecast.delete_tooltip')}">🗑️</button>
            </td>`;
        tbody.appendChild(tr);
    });
}

// ── mDNS discovery ──
async function ccDiscoverDevices() {
    const btn = document.getElementById('cc-discover-btn');
    const area = document.getElementById('cc-discover-area');
    btn.disabled = true;
    btn.textContent = '⏳ ' + t('config.chromecast.discovering');
    area.style.display = 'block';
    area.innerHTML = '<div style="text-align:center;padding:1rem;color:var(--text-secondary);font-size:0.85rem;">⏳ ' + t('config.chromecast.discovering') + '</div>';

    try {
        const resp = await fetch('/api/chromecast/discover');
        if (!resp.ok) throw new Error(await resp.text());
        const result = await resp.json();

        if (result.status !== 'success') {
            throw new Error(result.message || 'Discovery failed');
        }

        const devices = result.devices || [];
        if (devices.length === 0) {
            area.innerHTML = '<div style="padding:0.8rem 1rem;border-radius:9px;background:rgba(251,191,36,0.08);border:1px solid rgba(251,191,36,0.28);font-size:0.82rem;color:var(--text-secondary);">⚠️ ' + t('config.chromecast.no_devices_found') + '</div>';
            return;
        }

        // Filter out already-registered devices by IP
        const existingIPs = new Set(ccDevicesCache.map(d => d.ip_address));
        ccDiscoveredCache = devices;

        let dhtml = '<div style="padding:0.8rem 1rem;border-radius:9px;background:rgba(99,179,237,0.08);border:1px solid rgba(99,179,237,0.22);font-size:0.82rem;">';
        dhtml += '<div style="font-weight:600;margin-bottom:0.6rem;">' + t('config.chromecast.found_devices', { count: devices.length }) + '</div>';
        dhtml += '<div style="display:flex;flex-direction:column;gap:0.4rem;">';

        devices.forEach((d, i) => {
            const alreadyAdded = existingIPs.has(d.addr);
            const opacity = alreadyAdded ? 'opacity:0.5;' : '';
            dhtml += '<div style="display:flex;align-items:center;justify-content:space-between;padding:0.4rem 0.6rem;border-radius:8px;background:var(--bg-secondary);border:1px solid var(--border-subtle);' + opacity + '">';
            dhtml += '<div>';
            dhtml += '<span style="font-weight:500;">' + escapeHtml(d.name || 'Unknown') + '</span>';
            dhtml += ' <span style="color:var(--text-tertiary);font-family:monospace;font-size:0.78rem;margin-left:0.5rem;">' + escapeHtml(d.addr) + ':' + d.port + '</span>';
            dhtml += '</div>';
            if (alreadyAdded) {
                dhtml += '<span style="font-size:0.78rem;color:var(--text-tertiary);">✓ ' + t('config.chromecast.already_added') + '</span>';
            } else {
                dhtml += '<button class="btn-save" style="padding:0.3rem 0.8rem;font-size:0.78rem;" data-cc-idx="' + i + '">＋ ' + t('config.chromecast.add_device') + '</button>';
            }
            dhtml += '</div>';
        });

        dhtml += '</div></div>';
        area.innerHTML = dhtml;

        // Attach click handlers via JS — inline onclick on dynamically created
        // innerHTML elements can be unreliable; event listeners are always safe.
        area.querySelectorAll('button[data-cc-idx]').forEach(function(btn) {
            btn.addEventListener('click', function() {
                ccAddDiscovered(parseInt(this.getAttribute('data-cc-idx'), 10));
            });
        });

    } catch (e) {
        area.innerHTML = '<div style="padding:0.8rem 1rem;border-radius:9px;background:rgba(239,68,68,0.08);border:1px solid rgba(239,68,68,0.25);font-size:0.82rem;color:var(--danger);">❌ ' + escapeHtml(e.message) + '</div>';
    } finally {
        btn.disabled = false;
        btn.textContent = '🔍 ' + t('config.chromecast.discover_btn');
    }
}

// ── Add discovered device → opens modal with pre-filled data ──
function ccAddDiscovered(index) {
    const d = ccDiscoveredCache[index];
    if (!d) return;
    ccShowEditModal(null, d);
}

// ── Manual add → empty modal ──
function ccShowAddManual() {
    ccShowEditModal(null, null);
}

// ── Show modal (edit existing or add new) ──
function ccShowEditModal(id, prefill) {
    const overlay = document.getElementById('cc-modal-overlay');
    const title = document.getElementById('cc-modal-title');
    document.getElementById('cc-modal-error').style.display = 'none';

    if (id) {
        // Editing existing device
        const d = ccDevicesCache.find(x => x.id === id);
        if (!d) return;
        title.textContent = '✏️ ' + t('config.chromecast.edit_device');
        document.getElementById('cc-edit-id').value = d.id;
        document.getElementById('cc-field-name').value = d.name || '';
        document.getElementById('cc-field-ip').value = d.ip_address || '';
        document.getElementById('cc-field-port').value = d.port || 8009;
        document.getElementById('cc-field-desc').value = d.description || '';
    } else if (prefill) {
        // Adding from discovery
        title.textContent = '＋ ' + t('config.chromecast.add_device');
        document.getElementById('cc-edit-id').value = '';
        document.getElementById('cc-field-name').value = prefill.name || '';
        document.getElementById('cc-field-ip').value = prefill.addr || '';
        document.getElementById('cc-field-port').value = prefill.port || 8009;
        document.getElementById('cc-field-desc').value = '';
    } else {
        // Manual add
        title.textContent = '＋ ' + t('config.chromecast.add_device');
        document.getElementById('cc-edit-id').value = '';
        document.getElementById('cc-field-name').value = '';
        document.getElementById('cc-field-ip').value = '';
        document.getElementById('cc-field-port').value = '8009';
        document.getElementById('cc-field-desc').value = '';
    }
    overlay.style.display = 'block';
}

function ccCloseModal(e) {
    if (e && e.target !== e.currentTarget) return;
    document.getElementById('cc-modal-overlay').style.display = 'none';
}

// ── Save device (create/update via device registry API) ──
async function ccSaveDevice() {
    const errBox = document.getElementById('cc-modal-error');
    errBox.style.display = 'none';

    const name = document.getElementById('cc-field-name').value.trim();
    if (!name) {
        errBox.textContent = t('config.chromecast.name_required');
        errBox.style.display = 'block';
        return;
    }

    const ip = document.getElementById('cc-field-ip').value.trim();
    if (!ip) {
        errBox.textContent = t('config.chromecast.ip_required');
        errBox.style.display = 'block';
        return;
    }

    const payload = {
        name: name,
        type: 'chromecast',
        ip_address: ip,
        port: parseInt(document.getElementById('cc-field-port').value) || 8009,
        description: document.getElementById('cc-field-desc').value.trim(),
        tags: ['chromecast', 'smart-home'],
        username: '',
        mac_address: '',
        vault_secret_id: ''
    };

    const editId = document.getElementById('cc-edit-id').value;
    const isEdit = !!editId;
    const btn = document.getElementById('cc-modal-save');
    btn.disabled = true;

    try {
        const url = isEdit ? '/api/devices/' + editId : '/api/devices';
        const method = isEdit ? 'PUT' : 'POST';
        const resp = await fetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload)
        });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || 'Request failed');
        }
        ccCloseModal();
        await ccLoadDevices();
        // Re-render discovery results to update "already added" state
        const area = document.getElementById('cc-discover-area');
        if (area.style.display !== 'none' && ccDiscoveredCache.length > 0) {
            // Trigger a soft re-render of the discovery panel
            const existingIPs = new Set(ccDevicesCache.map(d => d.ip_address));
            area.querySelectorAll('button[data-cc-idx]').forEach(btn => {
                const idx = parseInt(btn.getAttribute('data-cc-idx'), 10);
                const d = ccDiscoveredCache[idx];
                if (d && existingIPs.has(d.addr)) {
                    const parent = btn.parentElement;
                    parent.style.opacity = '0.5';
                    btn.replaceWith(Object.assign(document.createElement('span'), {
                        style: 'font-size:0.78rem;color:var(--text-tertiary);',
                        textContent: '✓ ' + t('config.chromecast.already_added')
                    }));
                }
            });
        }
    } catch (e) {
        errBox.textContent = '❌ ' + e.message;
        errBox.style.display = 'block';
    } finally {
        btn.disabled = false;
    }
}

// ── Delete device ──
async function ccDeleteDevice(id, name) {
    const msg = t('config.chromecast.delete_confirm', { name: name });
    if (!confirm(msg)) return;

    try {
        const resp = await fetch('/api/devices/' + id, { method: 'DELETE' });
        if (!resp.ok) {
            const data = await resp.json().catch(() => ({}));
            throw new Error(data.error || 'Delete failed');
        }
        await ccLoadDevices();
    } catch (e) {
        alert('❌ ' + e.message);
    }
}
