// cfg/devices.js — Device Registry section module
let devicesCache = [];

        function renderDevicesSection(section) {
            let html = '<div class="cfg-section active">';
            html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
            html += '<div class="section-desc">' + section.desc + '</div>';

            html += `
            <div style="display:flex;justify-content:space-between;align-items:center;margin-top:1.5rem;margin-bottom:1rem;">
                <div style="display:flex;gap:0.5rem;align-items:center;">
                    <input type="text" id="devices-filter" class="field-input" placeholder="${t('config.devices.filter_placeholder')}" style="width:220px;padding:0.4rem 0.7rem;font-size:0.82rem;" oninput="devicesApplyFilter()">
                </div>
                <button class="btn-save" style="padding:0.45rem 1.1rem;font-size:0.82rem;" onclick="devicesShowModal()">
                    ＋ ${t('config.devices.add_device')}
                </button>
            </div>
            <div id="devices-table-wrap" style="overflow-x:auto;">
                <table style="width:100%;border-collapse:collapse;font-size:0.82rem;">
                    <thead>
                        <tr style="border-bottom:2px solid var(--border-subtle);text-align:left;">
                            <th style="padding:0.5rem 0.6rem;">${t('config.devices.col_name')}</th>
                            <th style="padding:0.5rem 0.6rem;">${t('config.devices.col_type')}</th>
                            <th style="padding:0.5rem 0.6rem;">${t('config.devices.col_ip')}</th>
                            <th style="padding:0.5rem 0.6rem;">${t('config.devices.col_port')}</th>
                            <th style="padding:0.5rem 0.6rem;">${t('config.devices.col_user')}</th>
                            <th style="padding:0.5rem 0.6rem;">${t('config.devices.col_mac')}</th>
                            <th style="padding:0.5rem 0.6rem;">${t('config.devices.col_tags')}</th>
                            <th style="padding:0.5rem 0.6rem;text-align:right;">${t('config.devices.col_actions')}</th>
                        </tr>
                    </thead>
                    <tbody id="devices-tbody"></tbody>
                </table>
            </div>
            <div id="devices-empty" style="display:none;text-align:center;padding:2rem;color:var(--text-tertiary);font-size:0.85rem;">
                ${t('config.devices.empty')}
            </div>
            <div id="devices-loading" style="text-align:center;padding:2rem;color:var(--text-secondary);font-size:0.85rem;">
                ${t('config.devices.loading')}
            </div>`;

            // Modal overlay
            html += `
            <div id="device-modal-overlay" style="display:none;position:fixed;inset:0;background:rgba(0,0,0,0.55);z-index:1000;backdrop-filter:blur(4px);" onclick="devicesCloseModal(event)">
                <div style="position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);background:var(--bg-primary);border:1px solid var(--border-subtle);border-radius:14px;padding:1.5rem;width:min(520px,90vw);max-height:85vh;overflow-y:auto;" onclick="event.stopPropagation()">
                    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1.2rem;">
                        <span id="device-modal-title" style="font-size:1rem;font-weight:600;"></span>
                        <button onclick="devicesCloseModal()" style="background:none;border:none;color:var(--text-secondary);font-size:1.2rem;cursor:pointer;">✕</button>
                    </div>
                    <input type="hidden" id="device-edit-id">
                    <div class="field-group" style="margin-bottom:0.8rem;">
                        <div class="field-label">Name *</div>
                        <input type="text" id="device-field-name" class="field-input" placeholder="${t('config.devices.name_placeholder')}">
                    </div>
                    <div style="display:grid;grid-template-columns:1fr 1fr;gap:0.8rem;">
                        <div class="field-group">
                            <div class="field-label">${t('config.devices.type_label')}</div>
                            <select id="device-field-type" class="field-input" style="padding:0.45rem 0.6rem;">
                                <option value="server">Server</option>
                                <option value="router">Router</option>
                                <option value="switch">Switch</option>
                                <option value="printer">Printer</option>
                                <option value="nas">NAS</option>
                                <option value="camera">Camera</option>
                                <option value="iot">IoT</option>
                                <option value="vm">VM</option>
                                <option value="container">Container</option>
                                <option value="generic">Generic</option>
                            </select>
                        </div>
                        <div class="field-group">
                            <div class="field-label">Port</div>
                            <input type="number" id="device-field-port" class="field-input" placeholder="22" value="22">
                        </div>
                    </div>
                    <div style="display:grid;grid-template-columns:2fr 1fr;gap:0.8rem;margin-top:0.8rem;">
                        <div class="field-group">
                            <div class="field-label">${t('config.devices.ip_address')}</div>
                            <input type="text" id="device-field-ip" class="field-input" placeholder="192.168.1.100">
                        </div>
                        <div class="field-group">
                            <div class="field-label">${t('config.devices.username')}</div>
                            <input type="text" id="device-field-username" class="field-input" placeholder="root">
                        </div>
                    </div>
                    <div class="field-group" style="margin-top:0.8rem;">
                        <div class="field-label">${t('config.devices.description')}</div>
                        <input type="text" id="device-field-desc" class="field-input" placeholder="${t('config.devices.desc_placeholder')}">
                    </div>
                    <div class="field-group" style="margin-top:0.8rem;">
                        <div class="field-label">${t('config.devices.mac_address')} <span style="color:var(--text-tertiary);font-weight:400;">(${t('config.devices.mac_hint')})</span></div>
                        <input type="text" id="device-field-mac" class="field-input" placeholder="AA:BB:CC:DD:EE:FF">
                    </div>
                    <div class="field-group" style="margin-top:0.8rem;">
                        <div class="field-label">Tags <span style="color:var(--text-tertiary);font-weight:400;">(${t('config.devices.tags_hint')})</span></div>
                        <input type="text" id="device-field-tags" class="field-input" placeholder="${t('config.devices.tags_placeholder')}">
                    </div>
                    <div class="field-group" style="margin-top:0.8rem;">
                        <div class="field-label">Vault Secret ID <span style="color:var(--text-tertiary);font-weight:400;">(${t('config.devices.vault_read_only')})</span></div>
                        <input type="text" id="device-field-vault" class="field-input" disabled style="opacity:0.6;">
                    </div>
                    <div id="device-modal-error" style="display:none;margin-top:0.7rem;padding:0.5rem 0.8rem;background:rgba(239,68,68,0.1);border:1px solid rgba(239,68,68,0.3);border-radius:8px;font-size:0.8rem;color:var(--danger);"></div>
                    <div style="display:flex;justify-content:flex-end;gap:0.6rem;margin-top:1.2rem;">
                        <button class="btn-save" style="padding:0.45rem 1rem;font-size:0.82rem;background:var(--bg-secondary);color:var(--text-primary);box-shadow:none;" onclick="devicesCloseModal()">${t('config.devices.cancel')}</button>
                        <button class="btn-save" style="padding:0.45rem 1.2rem;font-size:0.82rem;" id="device-modal-save" onclick="devicesSave()">${t('config.devices.save')}</button>
                    </div>
                </div>
            </div>`;

            html += '</div>';
            document.getElementById('content').innerHTML = html;
            devicesLoad();
        }

        async function devicesLoad() {
            const tbody = document.getElementById('devices-tbody');
            const empty = document.getElementById('devices-empty');
            const loading = document.getElementById('devices-loading');
            const table = document.getElementById('devices-table-wrap');
            loading.style.display = 'block';
            table.style.display = 'none';
            empty.style.display = 'none';

            try {
                const resp = await fetch('/api/devices');
                if (!resp.ok) throw new Error(await resp.text());
                devicesCache = await resp.json();
            } catch (e) {
                loading.textContent = '❌ ' + e.message;
                return;
            }

            loading.style.display = 'none';
            if (devicesCache.length === 0) {
                empty.style.display = 'block';
                return;
            }
            table.style.display = 'block';
            devicesRenderRows(devicesCache);
        }

        function devicesRenderRows(devices) {
            const tbody = document.getElementById('devices-tbody');
            tbody.innerHTML = '';
            devices.forEach(d => {
                const tags = (d.tags || []).map(tag => `<span style="display:inline-block;background:var(--bg-secondary);border:1px solid var(--border-subtle);border-radius:6px;padding:0.1rem 0.45rem;font-size:0.72rem;margin-right:0.25rem;">${escapeHtml(tag)}</span>`).join('');
                const tr = document.createElement('tr');
                tr.style.borderBottom = '1px solid var(--border-subtle)';
                tr.dataset.id = d.id;
                tr.innerHTML = `
                    <td style="padding:0.5rem 0.6rem;font-weight:500;">${escapeHtml(d.name)}</td>
                    <td style="padding:0.5rem 0.6rem;"><span style="background:var(--bg-secondary);border:1px solid var(--border-subtle);border-radius:6px;padding:0.15rem 0.5rem;font-size:0.75rem;">${escapeHtml(d.type)}</span></td>
                    <td style="padding:0.5rem 0.6rem;font-family:monospace;font-size:0.8rem;">${escapeHtml(d.ip_address || '—')}</td>
                    <td style="padding:0.5rem 0.6rem;">${d.port || '—'}</td>
                    <td style="padding:0.5rem 0.6rem;">${escapeHtml(d.username || '—')}</td>
                    <td style="padding:0.5rem 0.6rem;font-family:monospace;font-size:0.78rem;">${escapeHtml(d.mac_address || '—')}</td>
                    <td style="padding:0.5rem 0.6rem;">${tags || '—'}</td>
                    <td style="padding:0.5rem 0.6rem;text-align:right;white-space:nowrap;">
                        <button onclick="devicesShowModal('${d.id}')" style="background:none;border:none;cursor:pointer;font-size:0.9rem;" title="${t('config.devices.edit_tooltip')}">✏️</button>
                        <button onclick="devicesDelete('${d.id}','${escapeHtml(d.name)}')" style="background:none;border:none;cursor:pointer;font-size:0.9rem;margin-left:0.3rem;" title="${t('config.devices.delete_tooltip')}">🗑️</button>
                    </td>`;
                tbody.appendChild(tr);
            });
        }

        function devicesApplyFilter() {
            const q = (document.getElementById('devices-filter').value || '').toLowerCase();
            const filtered = devicesCache.filter(d => {
                const hay = [d.name, d.type, d.ip_address, d.username, d.description, d.mac_address, ...(d.tags || [])].join(' ').toLowerCase();
                return hay.includes(q);
            });
            devicesRenderRows(filtered);
            document.getElementById('devices-empty').style.display = filtered.length === 0 ? 'block' : 'none';
            document.getElementById('devices-table-wrap').style.display = filtered.length > 0 ? 'block' : 'none';
        }

        function devicesShowModal(id) {
            const overlay = document.getElementById('device-modal-overlay');
            const title = document.getElementById('device-modal-title');
            document.getElementById('device-modal-error').style.display = 'none';

            if (id) {
                const d = devicesCache.find(x => x.id === id);
                if (!d) return;
                title.textContent = t('config.devices.edit_device');
                document.getElementById('device-edit-id').value = d.id;
                document.getElementById('device-field-name').value = d.name || '';
                document.getElementById('device-field-type').value = d.type || 'generic';
                document.getElementById('device-field-ip').value = d.ip_address || '';
                document.getElementById('device-field-port').value = d.port || 22;
                document.getElementById('device-field-username').value = d.username || '';
                document.getElementById('device-field-desc').value = d.description || '';
                document.getElementById('device-field-mac').value = d.mac_address || '';
                document.getElementById('device-field-tags').value = (d.tags || []).join(', ');
                document.getElementById('device-field-vault').value = d.vault_secret_id || '';
            } else {
                title.textContent = t('config.devices.new_device');
                document.getElementById('device-edit-id').value = '';
                document.getElementById('device-field-name').value = '';
                document.getElementById('device-field-type').value = 'server';
                document.getElementById('device-field-ip').value = '';
                document.getElementById('device-field-port').value = '22';
                document.getElementById('device-field-username').value = '';
                document.getElementById('device-field-desc').value = '';
                document.getElementById('device-field-mac').value = '';
                document.getElementById('device-field-tags').value = '';
                document.getElementById('device-field-vault').value = '';
            }
            overlay.style.display = 'block';
        }

        function devicesCloseModal(e) {
            if (e && e.target !== e.currentTarget) return;
            document.getElementById('device-modal-overlay').style.display = 'none';
        }

        async function devicesSave() {
            const errBox = document.getElementById('device-modal-error');
            errBox.style.display = 'none';

            const name = document.getElementById('device-field-name').value.trim();
            if (!name) {
                errBox.textContent = t('config.devices.name_required');
                errBox.style.display = 'block';
                return;
            }

            const tagsRaw = document.getElementById('device-field-tags').value.trim();
            const tags = tagsRaw ? tagsRaw.split(',').map(s => s.trim()).filter(Boolean) : [];

            const payload = {
                name: name,
                type: document.getElementById('device-field-type').value,
                ip_address: document.getElementById('device-field-ip').value.trim(),
                port: parseInt(document.getElementById('device-field-port').value) || 22,
                username: document.getElementById('device-field-username').value.trim(),
                description: document.getElementById('device-field-desc').value.trim(),
                mac_address: document.getElementById('device-field-mac').value.trim(),
                tags: tags,
                vault_secret_id: document.getElementById('device-field-vault').value || ''
            };

            const editId = document.getElementById('device-edit-id').value;
            const isEdit = !!editId;
            const btn = document.getElementById('device-modal-save');
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
                devicesCloseModal();
                await devicesLoad();
            } catch (e) {
                errBox.textContent = '❌ ' + e.message;
                errBox.style.display = 'block';
            } finally {
                btn.disabled = false;
            }
        }

        async function devicesDelete(id, name) {
            const msg = t('config.devices.delete_confirm', {name: name});
            if (!confirm(msg)) return;

            try {
                const resp = await fetch('/api/devices/' + id, { method: 'DELETE' });
                if (!resp.ok) {
                    const data = await resp.json().catch(() => ({}));
                    throw new Error(data.error || 'Delete failed');
                }
                await devicesLoad();
            } catch (e) {
                alert('❌ ' + e.message);
            }
        }
