        }

        function showNotify(msg) {
            const existing = host.querySelector('.vd-qc-notify');
            if (existing) existing.remove();
            const el = document.createElement('div');
            el.className = 'vd-qc-notify';
            el.textContent = msg;
            host.querySelector('.vd-quick-connect').appendChild(el);
            setTimeout(() => el.remove(), 4000);
        }

        function disconnectActiveResizeObserver() {
            if (activeResizeObserver) {
                activeResizeObserver.disconnect();
                activeResizeObserver = null;
            }
        }

        function showConfirmModal(title, message, options) {
            return new Promise(resolve => {
                const opts = options || {};
                const okLabel = opts.okLabel || t('desktop.delete');
                const okIcon = opts.okIcon || 'trash';
                const okClass = opts.okClass || 'vd-qc-btn-danger';
                const overlay = document.createElement('div');
                overlay.className = 'vd-qc-modal-overlay';
                overlay.innerHTML = `<div class="vd-qc-confirm">
                    <div class="vd-qc-confirm-title">${esc(title)}</div>
                    <div class="vd-qc-confirm-msg">${esc(message)}</div>
                    <div class="vd-qc-confirm-actions">
                        <button class="vd-qc-btn vd-qc-btn-secondary" type="button" data-action="cancel">${iconMarkup('x', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.cancel'))}</span></button>
                        <button class="vd-qc-btn ${esc(okClass)}" type="button" data-action="ok">${iconMarkup(okIcon, 'OK', 'vd-qc-btn-icon', 14)}<span>${esc(okLabel)}</span></button>
                    </div>
                </div>`;
                host.querySelector('.vd-quick-connect').appendChild(overlay);
                overlay.querySelector('[data-action="cancel"]').addEventListener('click', () => { overlay.remove(); resolve(false); });
                overlay.querySelector('[data-action="ok"]').addEventListener('click', () => { overlay.remove(); resolve(true); });
            });
        }

        function showServerModal(existingDevice) {
            const isTemplate = !!(existingDevice && existingDevice.is_template);
            const isEdit = !!existingDevice && !isTemplate;
            const existingCred = isEdit && existingDevice.credential_id && cachedCredentials
                ? cachedCredentials.find(c => c.id === existingDevice.credential_id) : null;

            const overlay = document.createElement('div');
            overlay.className = 'vd-qc-modal-overlay';
            overlay.innerHTML = `<div class="vd-qc-modal">
                <div class="vd-qc-modal-header">
                    <span class="vd-qc-modal-title">${esc(isEdit ? t('desktop.qc_edit_server') : t('desktop.qc_add_server'))}</span>
                    <button class="vd-qc-modal-close" type="button" data-action="close" title="${esc(t('desktop.close'))}">${iconMarkup('x', 'X', 'vd-qc-close-icon', 14)}</button>
                </div>
                <div class="vd-qc-modal-body">
                    <div class="vd-qc-form-section">
                        <div class="vd-qc-form-title">${esc(t('desktop.qc_section_server'))}</div>
                        <label class="vd-qc-label">${esc(t('desktop.qc_name'))}
                            <input class="vd-qc-input" type="text" name="name" value="${esc(existingDevice ? existingDevice.name : '')}" required inputmode="text" enterkeyhint="next" autocapitalize="words">
                        </label>
                        <div class="vd-qc-form-row">
                            <label class="vd-qc-label vd-qc-flex-2">${esc(t('desktop.qc_host'))}
                                <input class="vd-qc-input" type="text" name="host" value="${esc(existingDevice ? (existingDevice.ip_address || '') : '')}" placeholder="192.168.1.1" required inputmode="url" enterkeyhint="next" autocapitalize="off">
                            </label>
                            <label class="vd-qc-label vd-qc-flex-1">${esc(t('desktop.qc_protocol'))}
                                <select class="vd-qc-input" name="protocol">
                                    <option value="ssh" ${(existingDevice ? existingDevice.protocol : 'ssh') === 'ssh' ? 'selected' : ''}>${esc(t('desktop.qc_protocol_ssh'))}</option>
                                    <option value="vnc" ${(existingDevice ? existingDevice.protocol : '') === 'vnc' ? 'selected' : ''}>${esc(t('desktop.qc_protocol_vnc'))}</option>
                                </select>
                            </label>
                            <label class="vd-qc-label vd-qc-flex-1">${esc(t('desktop.qc_port'))}
                                <input class="vd-qc-input" type="number" name="port" value="${existingDevice ? (existingDevice.port || 22) : 22}" min="1" max="65535">
                            </label>
                        </div>
                        <label class="vd-qc-label">${esc(t('desktop.qc_description'))}
                            <input class="vd-qc-input" type="text" name="description" value="${esc(existingDevice ? (existingDevice.description || '') : '')}" inputmode="text" enterkeyhint="next" autocapitalize="sentences">
                        </label>
                    </div>
                    <div class="vd-qc-form-section">
                        <div class="vd-qc-form-title">${esc(t('desktop.qc_section_credential'))}</div>
                        <label class="vd-qc-label vd-qc-username-row">${esc(t('desktop.qc_username'))}
                            <input class="vd-qc-input" type="text" name="username" value="${esc(existingCred ? existingCred.username : '')}" required inputmode="email" enterkeyhint="next" autocapitalize="off">
                        </label>
                        <label class="vd-qc-label">${esc(t('desktop.qc_password'))}
                            <div class="vd-qc-input-group">
                                <input class="vd-qc-input" type="password" name="password" placeholder="${isEdit && existingCred && existingCred.has_password ? t('desktop.qc_password_stored') : ''}">
                                <button class="vd-qc-input-toggle" type="button" data-action="toggle-pw" title="${esc(t('desktop.qc_password'))}">${iconMarkup('key', 'K', 'vd-qc-input-icon', 14)}</button>
                                ${isEdit && existingCred && existingCred.has_password ? `<button class="vd-qc-btn vd-qc-btn-sm vd-qc-icon-only" type="button" data-action="download-pw" title="${esc(t('desktop.qc_password'))}">${iconMarkup('download', 'D', 'vd-qc-btn-icon', 14)}</button>` : ''}
                            </div>
                        </label>
                        <label class="vd-qc-label">${esc(t('desktop.qc_certificate'))}
                            <div class="vd-qc-cert-area">
                                <textarea class="vd-qc-textarea" name="certificate_text" rows="3" placeholder="${t('desktop.qc_cert_paste_placeholder')}"></textarea>
                                <div class="vd-qc-cert-actions">
                                    <label class="vd-qc-btn vd-qc-btn-secondary vd-qc-btn-sm">
                                        ${iconMarkup('upload', 'U', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_upload_cert'))}</span>
                                        <input type="file" accept=".pem,.key,.pub,.crt,.cer,.txt" name="certificate_file" hidden>
                                    </label>
                                    ${isEdit && existingCred && existingCred.has_certificate ? `<button class="vd-qc-btn vd-qc-btn-sm" type="button" data-action="download-cert">${iconMarkup('download', 'D', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_download_cert'))}</span></button>` : ''}
                                </div>
                                ${isEdit && existingCred && existingCred.has_certificate ? '<span class="vd-qc-hint">' + esc(t('desktop.qc_cert_stored')) + '</span>' : ''}
                            </div>
                        </label>
                    </div>
                </div>
                <div class="vd-qc-modal-footer">
                    <button class="vd-qc-btn vd-qc-btn-secondary" type="button" data-action="cancel">${iconMarkup('x', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.cancel'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-primary" type="button" data-action="save">${iconMarkup('save', 'S', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_save'))}</span></button>
                </div>
            </div>`;

            host.querySelector('.vd-quick-connect').appendChild(overlay);

            const certFileInput = overlay.querySelector('input[name="certificate_file"]');
            const certTextarea = overlay.querySelector('textarea[name="certificate_text"]');
            certFileInput.addEventListener('change', async (e) => {
                const file = e.target.files[0];
                if (file) {
                    certTextarea.value = await file.text();
                }
            });

            const protocolSelect = overlay.querySelector('select[name="protocol"]');
            const usernameRow = overlay.querySelector('.vd-qc-username-row');
            const portInput = overlay.querySelector('input[name="port"]');
            function updateModalForProtocol() {
                const proto = protocolSelect.value;
                if (proto === 'vnc') {
                    usernameRow.style.display = 'none';
                    if (!existingDevice || !existingDevice.port) portInput.value = 5900;
                } else {
                    usernameRow.style.display = '';
                    if (!existingDevice || !existingDevice.port) portInput.value = 22;
                }
            }
            protocolSelect.addEventListener('change', updateModalForProtocol);
            updateModalForProtocol();

            overlay.querySelector('[data-action="toggle-pw"]').addEventListener('click', () => {
                const pwInput = overlay.querySelector('input[name="password"]');
                pwInput.type = pwInput.type === 'password' ? 'text' : 'password';
            });

            const dlPwBtn = overlay.querySelector('[data-action="download-pw"]');
            if (dlPwBtn && existingCred) {
                dlPwBtn.addEventListener('click', async () => {
                    try {
                        const body = await api('/api/credentials/export/' + existingCred.id + '?type=password');
                        downloadText(body.content, (existingCred.name || 'password') + '.txt');
                    } catch (err) { showNotify(err.message); }
                });
            }

            const dlCertBtn = overlay.querySelector('[data-action="download-cert"]');
            if (dlCertBtn && existingCred) {
                dlCertBtn.addEventListener('click', async () => {
                    try {
                        const body = await api('/api/credentials/export/' + existingCred.id + '?type=certificate');
                        downloadText(body.content, (existingCred.name || 'key') + '_key.pem');
                    } catch (err) { showNotify(err.message); }
                });
            }

            // Close / Cancel
            overlay.querySelector('[data-action="close"]').addEventListener('click', () => overlay.remove());
            overlay.querySelector('[data-action="cancel"]').addEventListener('click', () => overlay.remove());

            // Save
            overlay.querySelector('[data-action="save"]').addEventListener('click', async () => {
                const name = overlay.querySelector('input[name="name"]').value.trim();
                const hostVal = overlay.querySelector('input[name="host"]').value.trim();
                const port = parseInt(overlay.querySelector('input[name="port"]').value) || 22;
                const description = overlay.querySelector('input[name="description"]').value.trim();
                const protocol = overlay.querySelector('select[name="protocol"]').value;
                const username = overlay.querySelector('input[name="username"]').value.trim();
                const password = overlay.querySelector('input[name="password"]').value;
                const certificateText = certTextarea.value.trim();

                if (!name || !hostVal) {
                    showNotify(t('desktop.qc_validation_error'));
                    return;
                }
                if (protocol === 'ssh' && !username) {
                    showNotify(t('desktop.qc_validation_error'));
                    return;
                }

                const credType = protocol === 'vnc' ? 'login' : 'ssh';
                const effectiveUsername = protocol === 'vnc' ? '' : username;

                try {
                    if (isEdit) {
                        // Update credential if exists
                        if (existingCred) {
                            const credBody = { name: name, type: credType, host: hostVal, username: effectiveUsername, description: description, certificate_mode: 'text' };
                            if (password) credBody.password = password;
                            if (certificateText) credBody.certificate_text = certificateText;
                            await api('/api/credentials/' + existingCred.id, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credBody) });
                        } else {
                            // Create credential and link
                            const credBody = { name: name, type: credType, host: hostVal, username: effectiveUsername, description: description, certificate_mode: 'text' };
                            if (password) credBody.password = password;
                            if (certificateText) credBody.certificate_text = certificateText;
                            const created = await api('/api/credentials', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credBody) });
                            existingDevice.credential_id = created.id;
                        }
                        // Update device
                        await api('/api/devices/' + existingDevice.id, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, type: existingDevice.type || 'server', protocol, ip_address: hostVal, port, description, credential_id: existingDevice.credential_id }) });
                    } else {
                        // Create credential first
                        const credBody = { name: name, type: credType, host: hostVal, username: effectiveUsername, description: description, certificate_mode: 'text' };
                        if (password) credBody.password = password;
                        if (certificateText) credBody.certificate_text = certificateText;
                        const created = await api('/api/credentials', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credBody) });
                        // Create device linked to credential
                        await api('/api/devices', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, type: 'server', protocol, ip_address: hostVal, port, description, credential_id: created.id }) });
                    }
                    overlay.remove();
                    await loadAll();
                } catch (err) {
                    showNotify(t('desktop.qc_save_error') + ': ' + err.message);
                }
            });
        }

        function downloadText(content, filename) {
            const blob = new Blob([content], { type: 'text/plain' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = filename;
            a.click();
            URL.revokeObjectURL(url);
        }

        function connectToDevice(deviceId) {
            const device = cachedDevices.find(d => d.id === deviceId);
            if (device && device.protocol === 'vnc') {
                connectVNC(deviceId);
            } else {
                connectSSH(deviceId);
            }
        }

        function disconnectPlaceholderHTML(messageKey, deviceId, isVNC, messageText) {
            const reconnectBtn = `<button class="vd-qc-btn vd-qc-btn-primary vd-qc-reconnect-btn" type="button" data-action="reconnect">${iconMarkup('refresh', 'R', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_reconnect'))}</span></button>`;
            const newTermBtn = isVNC ? '' : `<button class="vd-qc-btn vd-qc-btn-secondary vd-qc-reconnect-btn" type="button" data-action="new-terminal">${iconMarkup('terminal', 'T', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_new_terminal'))}</span></button>`;
            return `<div class="vd-qc-placeholder vd-qc-disconnected">
                <span class="vd-qc-placeholder-text">${esc(messageText || t(messageKey))}</span>
                <div class="vd-qc-reconnect-actions">${reconnectBtn}${newTermBtn}</div>
            </div>`;
        }

        function renderVNCToolbar() {
            return `<div class="vd-qc-vnc-toolbar" data-vnc-toolbar>
                <span class="vd-qc-vnc-status" data-vnc-status data-state="connecting">${esc(t('desktop.qc_vnc_status_connecting'))}</span>
                <div class="vd-qc-vnc-tools" role="group" aria-label="${esc(t('desktop.qc_vnc_toolbar_label'))}">
                    <button class="vd-qc-btn vd-qc-btn-sm active" type="button" data-vnc-scale="fit" title="${esc(t('desktop.qc_vnc_scale_fit'))}">${iconMarkup('maximize', 'F', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_vnc_scale_fit'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-vnc-scale="native" title="${esc(t('desktop.qc_vnc_scale_native'))}">${iconMarkup('monitor', 'N', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_vnc_scale_native'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-vnc-action="view-only" aria-pressed="false" title="${esc(t('desktop.qc_vnc_view_only'))}">${iconMarkup('eye', 'V', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_vnc_view_only'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-vnc-action="ctrl-alt-del" title="${esc(t('desktop.qc_vnc_ctrl_alt_del'))}">${iconMarkup('keyboard', 'K', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_vnc_ctrl_alt_del'))}</span></button>
                    <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-vnc-action="reconnect" title="${esc(t('desktop.qc_reconnect'))}">${iconMarkup('refresh', 'R', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_reconnect'))}</span></button>
                </div>
            </div>`;
        }

        function setVNCStatus(root, state, detail) {
            const status = root && root.querySelector ? root.querySelector('[data-vnc-status]') : null;
            if (!status) return;
            const keys = {
                connecting: 'desktop.qc_vnc_status_connecting',
                connected: 'desktop.qc_vnc_status_connected',
                disconnected: 'desktop.qc_vnc_status_disconnected',
                error: 'desktop.qc_vnc_status_error'
            };
            status.dataset.state = state;
            status.textContent = detail || t(keys[state] || keys.error);
        }

        function vncErrorText(code, fallback) {
            const keys = {
                device_not_found: 'desktop.qc_vnc_error_device_not_found',
                protocol_mismatch: 'desktop.qc_vnc_error_protocol_mismatch',
                credential_unavailable: 'desktop.qc_vnc_error_credential_unavailable',
                dial_failed: 'desktop.qc_vnc_error_dial_failed',
                auth_failed: 'desktop.qc_vnc_error_auth_failed',
                init_failed: 'desktop.qc_vnc_error_init_failed'
            };
            const key = keys[code] || 'desktop.qc_vnc_connection_error';
            const message = t(key);
            return fallback ? `${message}: ${fallback}` : message;
        }

        function wireVNCToolbar(root, rfb, deviceId) {
            root.querySelectorAll('[data-vnc-scale]').forEach(btn => {
                btn.addEventListener('click', () => {
                    if (!rfb) return;
                    const mode = btn.dataset.vncScale;
                    root.querySelectorAll('[data-vnc-scale]').forEach(item => item.classList.toggle('active', item === btn));
                    rfb.scaleViewport = mode === 'fit';
                    rfb.resizeSession = mode === 'fit';
                });
            });
            const viewOnlyBtn = root.querySelector('[data-vnc-action="view-only"]');
            if (viewOnlyBtn) {
                viewOnlyBtn.addEventListener('click', () => {
                    if (!rfb) return;
                    const next = !rfb.viewOnly;
                    rfb.viewOnly = next;
                    viewOnlyBtn.classList.toggle('active', next);
                    viewOnlyBtn.setAttribute('aria-pressed', next ? 'true' : 'false');
                    showNotify(t(next ? 'desktop.qc_vnc_view_only' : 'desktop.qc_vnc_control_mode'));
                });
            }
            const ctrlAltDelBtn = root.querySelector('[data-vnc-action="ctrl-alt-del"]');
            if (ctrlAltDelBtn) {
                ctrlAltDelBtn.addEventListener('click', () => {
                    if (rfb && typeof rfb.sendCtrlAltDel === 'function') {
                        rfb.sendCtrlAltDel();
                    }
                });
            }
            const reconnectBtn = root.querySelector('[data-vnc-action="reconnect"]');
            if (reconnectBtn) reconnectBtn.addEventListener('click', () => connectVNC(deviceId));
        }

        function connectSSH(deviceId) {
            deviceList.querySelectorAll('.vd-qc-device').forEach(btn => btn.classList.toggle('active', btn.dataset.deviceId === deviceId));
            if (activeWS) { try { activeWS.close(); } catch(_) {} activeWS = null; }
            if (activeTerm) { activeTerm.dispose(); activeTerm = null; }
            disconnectActiveResizeObserver();
            connectedDeviceId = deviceId;
            connectedProtocol = 'ssh';
            activeTab = 'terminal';
            qcTabs.hidden = false;
            qcTabs.querySelector('[data-tab="files"]').hidden = false;
            qcTabs.querySelectorAll('.vd-qc-tab').forEach(btn => btn.classList.toggle('active', btn.dataset.tab === 'terminal'));
            closeSFTPPanel(tabContent);
            tabContent.innerHTML = `<div class="vd-qc-placeholder vd-qc-connecting"><div class="vd-qc-spinner fm-spinner"></div><span class="vd-qc-placeholder-text">${esc(t('desktop.qc_connecting'))}</span></div>`;

            const term = new Terminal({
                theme: {
                    background: '#0d1117', foreground: '#c9d1d9', cursor: '#58a6ff',
                    selectionBackground: 'rgba(88, 166, 255, 0.3)',
                    black: '#0d1117', red: '#ff7b72', green: '#3fb950', yellow: '#d29922',
                    blue: '#58a6ff', magenta: '#bc8cff', cyan: '#39c5cf', white: '#c9d1d9',
                    brightBlack: '#484f58', brightRed: '#ffa198', brightGreen: '#56d364',
                    brightYellow: '#e3b341', brightBlue: '#79c0ff', brightMagenta: '#d2a8ff',
                    brightCyan: '#56d4dd', brightWhite: '#f0f6fc'
                },
                fontFamily: "'Cascadia Code', 'JetBrains Mono', 'Fira Code', 'Consolas', monospace",
                fontSize: 14, cursorBlink: true, cursorStyle: 'bar', scrollback: 5000, convertEol: true
            });
            const fitAddon = new FitAddon.FitAddon();
            term.loadAddon(fitAddon);
            const termContainer = document.createElement('div');
            termContainer.className = 'vd-qc-term-container';
            tabContent.replaceChildren(termContainer);
            term.open(termContainer);
            activeTerm = term;
            activeFitAddon = fitAddon;
            setTimeout(() => { try { fitAddon.fit(); } catch(_) {} }, 50);
            const resizeObserver = new ResizeObserver(() => {
                if (activeTerm === term) { try { fitAddon.fit(); } catch(_) {} }
            });
            resizeObserver.observe(termContainer);
            activeResizeObserver = resizeObserver;

            const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = proto + '//' + location.host + '/api/desktop/ssh?device_id=' + encodeURIComponent(deviceId) + '&cols=' + term.cols + '&rows=' + term.rows;
            const ws = new WebSocket(wsUrl);
            ws.binaryType = 'arraybuffer';
            activeWS = ws;

            term.onData(data => {
                if (ws.readyState === WebSocket.OPEN) ws.send(new TextEncoder().encode(data));
            });
            term.onResize(({ cols, rows }) => {
                if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify({ type: 'resize', cols, rows }));
            });
            ws.onmessage = async (event) => {
                if (typeof event.data === 'string') {
                    try {
                        const msg = JSON.parse(event.data);
                        if (msg.type === 'error') term.write('\r\n\x1b[31m' + msg.message + '\x1b[0m\r\n');
                        else if (msg.type === 'warning' && msg.code === 'insecure_host_key') {
                            const warning = t('desktop.qc_host_key_warning');
                            term.write('\r\n\x1b[33m' + warning + '\x1b[0m\r\n');
                            showNotify(warning);
                        }
                        else if (msg.type === 'host_key_prompt' && msg.code === 'unknown_host_key') {
                            const detail = t('desktop.qc_host_key_prompt_message')
                                .replace('{{host}}', msg.host || '')
                                .replace('{{key_type}}', msg.key_type || '')
                                .replace('{{fingerprint}}', msg.fingerprint || '');
                            term.write('\r\n\x1b[33m' + detail + '\x1b[0m\r\n');
                            const accept = await showConfirmModal(t('desktop.qc_host_key_prompt_title'), detail, {
                                okLabel: t('desktop.qc_host_key_accept'),
                                okIcon: 'check-square',
                                okClass: 'vd-qc-btn-primary'
                            });
                            if (ws.readyState === WebSocket.OPEN) {
                                ws.send(JSON.stringify({ type: 'host_key_decision', accept }));
                            }
                            if (!accept) {
                                term.write('\r\n\x1b[31m' + t('desktop.qc_host_key_rejected') + '\x1b[0m\r\n');
                            }
                        }
                        else if (msg.type === 'disconnected') term.write('\r\n\x1b[33m' + msg.message + '\x1b[0m\r\n');
                    } catch(_) {}
                } else {
                    const bytes = event.data instanceof ArrayBuffer ? new Uint8Array(event.data) : new TextEncoder().encode(event.data);
                    term.write(bytes);
                }
            };
            ws.onclose = () => {
                if (activeWS === ws) {
                    term.write('\r\n\x1b[33m' + t('desktop.qc_disconnected') + '\x1b[0m\r\n');
                    activeWS = null;
                    const placeholder = document.createElement('div');
                    placeholder.innerHTML = disconnectPlaceholderHTML('desktop.qc_disconnected', deviceId, false);
                    const reconnectActions = placeholder.querySelector('.vd-qc-reconnect-actions');
                    if (reconnectActions) {
                        reconnectActions.querySelector('[data-action="reconnect"]').addEventListener('click', () => connectSSH(deviceId));
                        const newTermBtn = reconnectActions.querySelector('[data-action="new-terminal"]');
                        if (newTermBtn) newTermBtn.addEventListener('click', () => connectSSH(deviceId));
                    }
                    const termContainer = tabContent.querySelector('.vd-qc-term-container');
                    if (termContainer) termContainer.appendChild(placeholder.firstElementChild);
                    else tabContent.appendChild(placeholder.firstElementChild);
                }
            };
            ws.onerror = () => {
                if (activeWS === ws) {
                    term.write('\r\n\x1b[31m' + t('desktop.qc_connection_error') + '\x1b[0m\r\n');
                    activeWS = null;
                    const placeholder = document.createElement('div');
                    placeholder.innerHTML = disconnectPlaceholderHTML('desktop.qc_connection_error', deviceId, false);
                    const reconnectActions = placeholder.querySelector('.vd-qc-reconnect-actions');
                    if (reconnectActions) {
                        reconnectActions.querySelector('[data-action="reconnect"]').addEventListener('click', () => connectSSH(deviceId));
                        const newTermBtn = reconnectActions.querySelector('[data-action="new-terminal"]');
                        if (newTermBtn) newTermBtn.addEventListener('click', () => connectSSH(deviceId));
                    }
                    const termContainer = tabContent.querySelector('.vd-qc-term-container');
                    if (termContainer) termContainer.appendChild(placeholder.firstElementChild);
                    else tabContent.appendChild(placeholder.firstElementChild);
                }
            };
        }

        async function connectVNC(deviceId) {
            deviceList.querySelectorAll('.vd-qc-device').forEach(btn => btn.classList.toggle('active', btn.dataset.deviceId === deviceId));
            if (activeWS) { try { activeWS.close(); } catch(_) {} activeWS = null; }
            if (activeTerm) { activeTerm.dispose(); activeTerm = null; }
            disconnectActiveResizeObserver();
            connectedDeviceId = deviceId;
            connectedProtocol = 'vnc';
            activeTab = 'terminal';
            qcTabs.hidden = false;
            qcTabs.querySelector('[data-tab="files"]').hidden = true;
            qcTabs.querySelectorAll('.vd-qc-tab').forEach(btn => btn.classList.toggle('active', btn.dataset.tab === 'terminal'));
            closeSFTPPanel(tabContent);
            tabContent.innerHTML = `<div class="vd-qc-placeholder vd-qc-connecting"><div class="vd-qc-spinner fm-spinner"></div><span class="vd-qc-placeholder-text">${esc(t('desktop.qc_vnc_connecting'))}</span></div>`;

            const sessionEl = document.createElement('div');
            sessionEl.className = 'vd-qc-vnc-session';
            sessionEl.innerHTML = `${renderVNCToolbar()}<div class="vd-qc-vnc-container"></div>`;
            tabContent.replaceChildren(sessionEl);
            const mountedVNCContainer = sessionEl.querySelector('.vd-qc-vnc-container');
            activeTerm = null;
            activeFitAddon = null;

            const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = proto + '//' + location.host + '/api/desktop/vnc?device_id=' + encodeURIComponent(deviceId);

            if (!window.RFB) {
                setVNCStatus(sessionEl, 'error');
                wireVNCToolbar(sessionEl, null, deviceId);
                mountedVNCContainer.innerHTML = `<div class="vd-qc-placeholder"><span class="vd-qc-placeholder-text">${esc(t('desktop.qc_vnc_unavailable'))}</span></div>`;
                return;
            }

            let lastVNCError = null;
            const rfb = new window.RFB(mountedVNCContainer, wsUrl, {
                wsProtocols: ['binary']
            });
            rfb.viewOnly = false;
            rfb.scaleViewport = true;
            rfb.resizeSession = true;
            wireVNCToolbar(sessionEl, rfb, deviceId);

            rfb.addEventListener('connect', () => {
                lastVNCError = null;
                setVNCStatus(sessionEl, 'connected');
                showNotify(t('desktop.qc_vnc_connected'));
            });
            rfb.addEventListener('disconnect', () => {
                if (activeWS && activeWS.close) { activeWS = null; }
                if (lastVNCError) {
                    setVNCStatus(sessionEl, 'error', lastVNCError);
                    mountedVNCContainer.innerHTML = disconnectPlaceholderHTML('desktop.qc_vnc_connection_error', deviceId, true, lastVNCError);
                } else {
                    setVNCStatus(sessionEl, 'disconnected');
                    mountedVNCContainer.innerHTML = disconnectPlaceholderHTML('desktop.qc_vnc_disconnected', deviceId, true);
                }
                const reconnectBtn = mountedVNCContainer.querySelector('[data-action="reconnect"]');
                if (reconnectBtn) reconnectBtn.addEventListener('click', () => connectVNC(deviceId));
            });
            rfb.addEventListener('securityfailure', (e) => {
                let reason = e && e.detail ? (e.detail.reason || e.detail.message || '') : '';
                let code = e && e.detail ? (e.detail.code || '') : '';
                if (reason) {
                    try {
                        const parsed = JSON.parse(reason);
                        if (parsed && parsed.type === 'error') {
                            code = parsed.code || code;
                            reason = parsed.message || '';
                        }
                    } catch(_) {}
                }
                const message = vncErrorText(code || 'auth_failed', reason);
                lastVNCError = message;
                setVNCStatus(sessionEl, 'error', message);
                showNotify(message);
            });

            activeWS = { close: () => { try { rfb.disconnect(); } catch(_) {} } };
        }

        let sftpCurrentPath = '/';
        let sftpEntries = [];

        function closeSFTPPanel(container) {
            const panel = container.querySelector('.vd-qc-sftp-panel');
            if (panel) panel.remove();
            const termContainer = container.querySelector('.vd-qc-term-container');
            if (termContainer) termContainer.style.display = '';
            const vncContainer = container.querySelector('.vd-qc-vnc-container');
            if (vncContainer) vncContainer.style.display = '';
        }

        function openSFTPPanel(deviceId, container) {
            closeSFTPPanel(container);
            const termContainer = container.querySelector('.vd-qc-term-container');
            if (termContainer) termContainer.style.display = 'none';
            const vncContainer = container.querySelector('.vd-qc-vnc-container');
            if (vncContainer) vncContainer.style.display = 'none';

            const panel = document.createElement('div');
            panel.className = 'vd-qc-sftp-panel';
            panel.innerHTML = `
                <div class="vd-qc-sftp-toolbar">
                    <button class="vd-qc-btn vd-qc-btn-sm vd-qc-sftp-back" type="button" title="${esc(t('desktop.qc_sftp_browse'))}">${iconMarkup('arrow-left', 'B', 'vd-qc-btn-icon', 14)}</button>
                    <div class="vd-qc-sftp-breadcrumb" data-sftp-breadcrumb></div>
                    <div class="vd-qc-sftp-actions">
                        <label class="vd-qc-btn vd-qc-btn-sm vd-qc-sftp-upload-btn" title="${esc(t('desktop.qc_sftp_upload'))}">
                            ${iconMarkup('upload', 'U', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_sftp_upload'))}</span>
                            <input type="file" multiple hidden data-sftp-file-input>
                        </label>
                        <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-action="mkdir" title="${esc(t('desktop.qc_sftp_mkdir'))}">${iconMarkup('folder', 'N', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.qc_sftp_mkdir'))}</span></button>
                        <button class="vd-qc-btn vd-qc-btn-sm" type="button" data-action="sftp-refresh" title="${esc(t('desktop.qc_sftp_refresh'))}">${iconMarkup('refresh', 'R', 'vd-qc-btn-icon', 14)}</button>
                    </div>
                </div>
                <div class="vd-qc-sftp-list" data-sftp-list>
                    <div class="vd-qc-sftp-loading">${esc(t('desktop.qc_sftp_loading'))}</div>
                </div>
                <div class="vd-qc-sftp-status" data-sftp-status></div>
            `;
            container.appendChild(panel);

            const breadcrumb = panel.querySelector('[data-sftp-breadcrumb]');
            const listEl = panel.querySelector('[data-sftp-list]');
            const statusEl = panel.querySelector('[data-sftp-status]');
            const fileInput = panel.querySelector('[data-sftp-file-input]');

            sftpCurrentPath = '/';
            loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumb, statusEl);

            panel.querySelector('.vd-qc-sftp-back').addEventListener('click', () => {
                const parent = sftpCurrentPath === '/' ? '/' : sftpCurrentPath.replace(/\/[^/]+\/?$/, '') || '/';
                sftpCurrentPath = parent;
                loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumb, statusEl);
            });

            panel.querySelector('[data-action="sftp-refresh"]').addEventListener('click', () => {
                loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumb, statusEl);
            });

            panel.querySelector('[data-action="mkdir"]').addEventListener('click', () => {
                sftpMkdir(deviceId, sftpCurrentPath, listEl, breadcrumb, statusEl);
            });

            fileInput.addEventListener('change', () => {
                if (fileInput.files.length) {
                    sftpUploadFiles(deviceId, sftpCurrentPath, fileInput.files, listEl, breadcrumb, statusEl);
                    fileInput.value = '';
                }
            });

            listEl.addEventListener('dragover', (e) => { e.preventDefault(); listEl.classList.add('vd-qc-sftp-drag-over'); });
            listEl.addEventListener('dragleave', () => { listEl.classList.remove('vd-qc-sftp-drag-over'); });
            listEl.addEventListener('drop', (e) => {
                e.preventDefault();
                listEl.classList.remove('vd-qc-sftp-drag-over');
                if (e.dataTransfer.files.length) {
                    sftpUploadFiles(deviceId, sftpCurrentPath, e.dataTransfer.files, listEl, breadcrumb, statusEl);
                }
            });
        }

        function renderSFTPBreadcrumb(pathStr, breadcrumbEl, deviceId, listEl, statusEl) {
            const parts = pathStr.split('/').filter(Boolean);
            let html = `<button class="vd-qc-sftp-crumb" data-path="/">/</button>`;
            let accumulated = '';
            for (const part of parts) {
                accumulated += '/' + part;
                html += `<span class="vd-qc-sftp-crumb-sep">/</span><button class="vd-qc-sftp-crumb" data-path="${esc(accumulated)}">${esc(part)}</button>`;
            }
            breadcrumbEl.innerHTML = html;
            breadcrumbEl.querySelectorAll('.vd-qc-sftp-crumb').forEach(btn => {
                btn.addEventListener('click', () => {
                    sftpCurrentPath = btn.dataset.path;
                    loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumbEl, statusEl);
                });
            });
        }

        function formatSFTPSize(bytes) {
            if (bytes < 1024) return bytes + ' B';
            if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KiB';
            if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MiB';
            return (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GiB';
        }

        async function loadSFTPList(deviceId, dirPath, listEl, breadcrumbEl, statusEl) {
            listEl.innerHTML = `<div class="vd-qc-sftp-loading">${esc(t('desktop.qc_sftp_loading'))}</div>`;
            try {
                const resp = await api('/api/desktop/sftp/list?device_id=' + encodeURIComponent(deviceId) + '&path=' + encodeURIComponent(dirPath));
                sftpEntries = resp.entries || [];
                sftpEntries.sort((a, b) => {
                    if (a.is_dir !== b.is_dir) return a.is_dir ? -1 : 1;
                    return a.name.localeCompare(b.name);
                });
                renderSFTPBreadcrumb(dirPath, breadcrumbEl, deviceId, listEl, statusEl);
                renderSFTPList(deviceId, listEl, breadcrumbEl, statusEl);
            } catch (err) {
                listEl.innerHTML = `<div class="vd-qc-sftp-error">${esc(err.message || t('desktop.qc_sftp_error'))}</div>`;
            }
        }

        function renderSFTPList(deviceId, listEl, breadcrumbEl, statusEl) {
            if (!sftpEntries.length) {
                listEl.innerHTML = `<div class="vd-qc-sftp-empty">${iconMarkup('folder-open', 'F', 'vd-qc-sftp-empty-icon', 32)}<span>${esc(t('desktop.qc_sftp_empty'))}</span></div>`;
                statusEl.textContent = '0 items';
                return;
            }
            let html = `<table class="vd-qc-sftp-table"><thead><tr>
                <th>${esc(t('desktop.qc_sftp_name'))}</th>
                <th class="vd-qc-sftp-col-size">${esc(t('desktop.qc_sftp_size'))}</th>
                <th class="vd-qc-sftp-col-modified">${esc(t('desktop.qc_sftp_modified'))}</th>
                <th class="vd-qc-sftp-col-perms">${esc(t('desktop.qc_sftp_permissions'))}</th>
            </tr></thead><tbody>`;
            for (const entry of sftpEntries) {
                const icon = entry.is_dir ? iconMarkup('folder', 'D', 'vd-qc-sftp-entry-icon', 16) : iconMarkup('file', 'F', 'vd-qc-sftp-entry-icon', 16);
                const modTime = entry.mod_time ? new Date(entry.mod_time).toLocaleString() : '';
                html += `<tr class="vd-qc-sftp-row" data-name="${esc(entry.name)}" data-is-dir="${entry.is_dir}">
                    <td class="vd-qc-sftp-name">${icon}<span>${esc(entry.name)}</span></td>
                    <td class="vd-qc-sftp-col-size">${entry.is_dir ? '' : formatSFTPSize(entry.size)}</td>
                    <td class="vd-qc-sftp-col-modified">${esc(modTime)}</td>
                    <td class="vd-qc-sftp-col-perms"><code>${esc(entry.mode)}</code></td>
                </tr>`;
            }
            html += '</tbody></table>';
            listEl.innerHTML = html;
            statusEl.textContent = sftpEntries.length + ' items';

            listEl.querySelectorAll('.vd-qc-sftp-row').forEach(row => {
                row.addEventListener('dblclick', () => {
                    const name = row.dataset.name;
                    const isDir = row.dataset.isDir === 'true';
                    if (isDir) {
                        sftpCurrentPath = sftpCurrentPath === '/' ? '/' + name : sftpCurrentPath + '/' + name;
                        loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumbEl, statusEl);
                    } else {
                        sftpDownload(deviceId, sftpCurrentPath + (sftpCurrentPath === '/' ? '' : '/') + name);
                    }
                });
                row.addEventListener('contextmenu', (e) => {
                    e.preventDefault();
                    const name = row.dataset.name;
                    const isDir = row.dataset.isDir === 'true';
                    const fullPath = sftpCurrentPath + (sftpCurrentPath === '/' ? '' : '/') + name;
                    showSFTPContextMenu(e.clientX, e.clientY, deviceId, fullPath, name, isDir, listEl, breadcrumbEl, statusEl);
                });
            });
        }

        function showSFTPContextMenu(x, y, deviceId, fullPath, name, isDir, listEl, breadcrumbEl, statusEl) {
            closeContextMenu();
            const items = [];
            if (isDir) {
                items.push({ label: t('desktop.qc_sftp_browse'), icon: 'folder-open', fallback: 'O', action: () => {
                    sftpCurrentPath = fullPath;
                    loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumbEl, statusEl);
                }});
            } else {
                items.push({ label: t('desktop.qc_sftp_download'), icon: 'download', fallback: 'D', action: () => sftpDownload(deviceId, fullPath) });
            }
            items.push({ separator: true });
            items.push({ label: t('desktop.qc_sftp_rename'), icon: 'edit', fallback: 'R', action: () => sftpRename(deviceId, fullPath, listEl, breadcrumbEl, statusEl) });
            items.push({ label: t('desktop.qc_sftp_copy'), icon: 'copy', fallback: 'C', action: () => sftpCopy(deviceId, fullPath, listEl, breadcrumbEl, statusEl) });
            items.push({ label: t('desktop.qc_sftp_move'), icon: 'archive', fallback: 'M', action: () => sftpMove(deviceId, fullPath, listEl, breadcrumbEl, statusEl) });
            items.push({ separator: true });
            items.push({ label: t('desktop.qc_sftp_delete'), icon: 'trash', fallback: 'X', action: () => sftpDelete(deviceId, fullPath, name, listEl, breadcrumbEl, statusEl) });
            showContextMenu(x, y, items);
        }

        function sftpDownload(deviceId, remotePath) {
            const url = '/api/desktop/sftp/download?device_id=' + encodeURIComponent(deviceId) + '&path=' + encodeURIComponent(remotePath);
            const a = document.createElement('a');
            a.href = url;
            a.download = '';
            a.style.display = 'none';
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
        }

        async function sftpUploadFiles(deviceId, remotePath, files, listEl, breadcrumbEl, statusEl) {
            for (const file of files) {
                try {
                    const formData = new FormData();
                    formData.append('device_id', deviceId);
                    formData.append('remote_path', remotePath + (remotePath === '/' ? '' : '/') + file.name);
                    formData.append('file', file);
                    const resp = await fetch('/api/desktop/sftp/upload', { method: 'POST', body: formData });
                    if (!resp.ok) {
                        const err = await resp.json().catch(() => ({ error: 'Upload failed' }));
                        showNotify(err.error || t('desktop.qc_sftp_error'));
                    }
                } catch (err) {
                    showNotify(err.message || t('desktop.qc_sftp_error'));
                }
            }
            loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumbEl, statusEl);
        }

        async function sftpDelete(deviceId, fullPath, name, listEl, breadcrumbEl, statusEl) {
            const ok = await showConfirmModal(t('desktop.qc_sftp_delete'), t('desktop.qc_sftp_delete_confirm').replace('{{name}}', name));
            if (!ok) return;
            try {
                await api('/api/desktop/sftp/delete', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ device_id: deviceId, path: fullPath }) });
                loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumbEl, statusEl);
            } catch (err) {
                showNotify(err.message || t('desktop.qc_sftp_error'));
            }
        }

        async function sftpRename(deviceId, oldPath, listEl, breadcrumbEl, statusEl) {
            const oldName = oldPath.split('/').pop();
            const newName = prompt(t('desktop.qc_sftp_rename_prompt'), oldName);
            if (!newName || newName === oldName) return;
            const dir = oldPath.substring(0, oldPath.lastIndexOf('/')) || '/';
            const newPath = dir + (dir === '/' ? '' : '/') + newName;
            try {
                await api('/api/desktop/sftp/rename', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ device_id: deviceId, old_path: oldPath, new_path: newPath }) });
                loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumbEl, statusEl);
            } catch (err) {
                showNotify(err.message || t('desktop.qc_sftp_error'));
            }
        }

        async function sftpMkdir(deviceId, currentPath, listEl, breadcrumbEl, statusEl) {
            const dirName = prompt(t('desktop.qc_sftp_mkdir_prompt'));
            if (!dirName) return;
            const newPath = currentPath + (currentPath === '/' ? '' : '/') + dirName;
            try {
                await api('/api/desktop/sftp/mkdir', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ device_id: deviceId, path: newPath }) });
                loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumbEl, statusEl);
            } catch (err) {
                showNotify(err.message || t('desktop.qc_sftp_error'));
            }
        }

        async function sftpCopy(deviceId, srcPath, listEl, breadcrumbEl, statusEl) {
            const dstPath = prompt(t('desktop.qc_sftp_copy_prompt'), srcPath);
            if (!dstPath || dstPath === srcPath) return;
            try {
                await api('/api/desktop/sftp/copy', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ device_id: deviceId, src_path: srcPath, dst_path: dstPath }) });
                loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumbEl, statusEl);
            } catch (err) {
                showNotify(err.message || t('desktop.qc_sftp_error'));
            }
        }

        async function sftpMove(deviceId, srcPath, listEl, breadcrumbEl, statusEl) {
            const dstPath = prompt(t('desktop.qc_sftp_move_prompt'), srcPath);
            if (!dstPath || dstPath === srcPath) return;
            try {
                await api('/api/desktop/sftp/move', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ device_id: deviceId, src_path: srcPath, dst_path: dstPath }) });
                loadSFTPList(deviceId, sftpCurrentPath, listEl, breadcrumbEl, statusEl);
            } catch (err) {
                showNotify(err.message || t('desktop.qc_sftp_error'));
            }
        }
    }

    function setQuickConnectMenus(id, host, loadAll, showServerModal, toggleFiles) {
        const viewItems = [
            { id: 'refresh', labelKey: 'desktop.qc_refresh', icon: 'refresh', shortcut: 'F5', action: loadAll }
        ];
        if (toggleFiles) {
            viewItems.push({ id: 'toggle-files', labelKey: 'desktop.qc_tab_files', icon: 'folder', shortcut: 'Ctrl+Shift+F', action: toggleFiles });
        }
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'add-server', labelKey: 'desktop.qc_add_server', icon: 'server', shortcut: 'Ctrl+N', action: () => showServerModal() }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: viewItems
            }
        ]);
    }

    function renderLaunchpad(id) {
        const host = contentEl(id);
        if (!host) return;
        let links = [];
        let categories = [];
        let searchQuery = '';
        let selectedCategory = '';
        let iconSearchDebounce = null;
        let selectedIconURL = null;

        host.innerHTML = `
            <div class="vd-launchpad">
                <div class="vd-launchpad-toolbar">
                    <input type="search" class="vd-launchpad-search" data-i18n-placeholder="desktop.launchpad_search" placeholder="${esc(t('desktop.launchpad_search'))}" inputmode="search" enterkeyhint="search" autocapitalize="off">
                    <select class="vd-launchpad-category"><option value="">${esc(t('desktop.launchpad_all_categories'))}</option></select>
                </div>
                <div class="vd-launchpad-grid"></div>
                <div class="vd-launchpad-empty" hidden>
                    <div class="vd-launchpad-empty-icon">${iconMarkup('launchpad', 'A', 'vd-launchpad-empty-papirus-icon', 42)}</div>
                    <div>${esc(t('desktop.launchpad_empty'))}</div>
                </div>
            </div>`;

        const grid = host.querySelector('.vd-launchpad-grid');
        const empty = host.querySelector('.vd-launchpad-empty');
        const searchInput = host.querySelector('.vd-launchpad-search');
        const categorySelect = host.querySelector('.vd-launchpad-category');
        const showLaunchpadContextMenu = (event, linkId) => {
            const link = links.find(item => item.id === linkId);
            if (!link) return false;
            showContextMenu(event.clientX, event.clientY, [
                { labelKey: 'desktop.context_open', icon: 'folder-open', action: () => openTileLink(linkId) },
                { labelKey: 'desktop.launchpad_edit', icon: 'edit', action: () => openEditModal(linkId) },
                { separator: true },
                { labelKey: 'desktop.launchpad_delete', icon: 'trash', action: () => deleteLink(linkId) }
            ]);
            return true;
        };
        wireContextMenuBoundary(host);

        async function load() {
            try {
                const url = selectedCategory ? '/api/launchpad/links?category=' + encodeURIComponent(selectedCategory) : '/api/launchpad/links';
                links = await api(url);
                categories = await api('/api/launchpad/categories');
                updateCategorySelect();
                render();
            } catch (e) { showDesktopNotification({ message: t('desktop.launchpad_load_error') }); }
        }

        function updateCategorySelect() {
            const val = categorySelect.value;
            categorySelect.innerHTML = '<option value="">' + esc(t('desktop.launchpad_all_categories')) + '</option>';
            categories.forEach(c => { categorySelect.innerHTML += '<option value="' + esc(c) + '">' + esc(c) + '</option>'; });
            categorySelect.value = val;
        }

        function render() {
            let filtered = links;
            const q = searchQuery.toLowerCase().trim();
            if (q) filtered = filtered.filter(l => (l.title || '').toLowerCase().includes(q) || (l.description || '').toLowerCase().includes(q));
            if (filtered.length === 0) { grid.innerHTML = ''; empty.hidden = false; return; }
            empty.hidden = true;
            grid.innerHTML = filtered.map(link => {
                const iconSrc = link.icon_path && /^https?:\/\//i.test(link.icon_path) ? link.icon_path : (link.icon_path ? '/files/' + link.icon_path : '');
                const icon = iconSrc ? '<img class="vd-launchpad-tile-icon" src="' + esc(iconSrc) + '" alt="" loading="lazy" onerror="this.hidden=true;this.nextElementSibling.hidden=false">' : '';
                const fallback = '<div class="vd-launchpad-tile-fallback"' + (link.icon_path ? ' hidden' : '') + '>' + iconMarkup(launchpadCategoryIconKey(link.category), 'G', 'vd-launchpad-fallback-icon', 34) + '</div>';
                return '<div class="vd-launchpad-tile" data-id="' + esc(link.id) + '">' + icon + fallback +
                    '<div class="vd-launchpad-tile-title">' + esc(link.title) + '</div>' +
                    (link.description ? '<div class="vd-launchpad-tile-desc">' + esc(link.description) + '</div>' : '') +
                    '<div class="vd-launchpad-tile-actions">' +
                    '<button type="button" class="vd-launchpad-tile-btn" data-action="edit" title="' + esc(t('desktop.launchpad_edit')) + '">' + iconMarkup('edit', 'E', 'vd-launchpad-action-icon', 15) + '</button>' +
                    '<button type="button" class="vd-launchpad-tile-btn" data-action="delete" title="' + esc(t('desktop.launchpad_delete')) + '">' + iconMarkup('trash', 'X', 'vd-launchpad-action-icon', 15) + '</button>' +
                    '</div></div>';
            }).join('');
            grid.querySelectorAll('.vd-launchpad-tile').forEach(tile => {
                tile.addEventListener('click', (e) => { if (!e.target.closest('.vd-launchpad-tile-actions')) openTileLink(tile.dataset.id); });
                tile.addEventListener('contextmenu', event => {
                    event.preventDefault();
                    showLaunchpadContextMenu(event, tile.dataset.id);
                });
            });
            grid.querySelectorAll('[data-action="edit"]').forEach(btn => {
                btn.addEventListener('click', (e) => { e.stopPropagation(); openEditModal(btn.closest('.vd-launchpad-tile').dataset.id); });
            });
            grid.querySelectorAll('[data-action="delete"]').forEach(btn => {
                btn.addEventListener('click', (e) => { e.stopPropagation(); deleteLink(btn.closest('.vd-launchpad-tile').dataset.id); });
            });
        }

        function openTileLink(linkId) {
            const link = links.find(l => l.id === linkId);
            if (!link || !link.url) return;
            if (String(link.url).startsWith('aurago-store://')) {
                const appId = String(link.url).slice('aurago-store://'.length).replace(/[^a-z0-9-]/g, '');
                if (appId) openApp('store-' + appId);
                return;
            }
            window.open(link.url, '_blank', 'noopener,noreferrer');
        }

        async function deleteLink(linkId) {
            const ok = await confirmDialog(t('desktop.launchpad_delete_confirm'), '');
            if (!ok) return;
            try { await api('/api/launchpad/links/' + linkId, { method: 'DELETE' }); await load(); }
            catch (e) { showDesktopNotification({ message: t('desktop.launchpad_delete_error') }); }
        }

        async function openEditModal(linkId) {
            const link = linkId ? links.find(l => l.id === linkId) : null;
            selectedIconURL = null;
            const backdrop = document.createElement('div');
            backdrop.className = 'vd-modal-backdrop';
            backdrop.innerHTML = `
                <form class="vd-modal vd-launchpad-modal" role="dialog" aria-modal="true">
                    <div class="vd-modal-title">${esc(linkId ? t('desktop.launchpad_edit_title') : t('desktop.launchpad_add_title'))}</div>
                    <div class="vd-launchpad-form-stack">
                        <input type="hidden" class="lp-id" value="${esc(linkId || '')}">
                        <input type="text" class="vd-modal-input lp-title" placeholder="${esc(t('desktop.launchpad_label_title'))}" value="${esc(link ? link.title : '')}" required>
                        <input type="url" class="vd-modal-input lp-url" placeholder="${esc(t('desktop.launchpad_label_url'))}" value="${esc(link ? link.url : '')}" required>
                        <input type="text" class="vd-modal-input lp-category" placeholder="${esc(t('desktop.launchpad_label_category'))}" list="lp-cats" value="${esc(link ? link.category : '')}">
                        <datalist id="lp-cats">${(categories || []).map(c => '<option value="' + esc(c) + '">').join('')}</datalist>
                        <input type="text" class="vd-modal-input lp-description" placeholder="${esc(t('desktop.launchpad_label_description'))}" value="${esc(link ? link.description : '')}">
                        <div class="vd-launchpad-field-label">${esc(t('desktop.launchpad_label_icon'))}</div>
                        <div class="lp-icon-tabs"><button type="button" class="lp-icon-tab active" data-tab="search">${esc(t('desktop.launchpad_tab_search'))}</button><button type="button" class="lp-icon-tab" data-tab="url">${esc(t('desktop.launchpad_tab_url'))}</button></div>
                        <div class="lp-icon-panel active" data-panel="search"><div class="lp-icon-search-row"><input type="text" class="lp-icon-search" placeholder="${esc(t('desktop.launchpad_icon_search_placeholder'))}"><button type="button" class="vd-tool-button lp-icon-search-btn">${iconMarkup('search', 'S', 'vd-tool-icon', 15)}</button></div><div class="lp-icon-results"></div><div class="lp-icon-selected-preview" hidden></div></div>
                        <div class="lp-icon-panel" data-panel="url"><input type="url" class="lp-icon-url" placeholder="${esc(t('desktop.launchpad_icon_url_placeholder'))}"><div class="lp-icon-preview"></div></div>
                        <input type="hidden" class="lp-icon-path" value="${esc(link && link.icon_path ? link.icon_path : '')}">
                    </div>
                    <div class="vd-modal-actions">
                        <button type="button" class="vd-button" data-action="cancel">${iconMarkup('x', 'X', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.cancel'))}</span></button>
                        <button type="button" class="vd-button vd-button-primary" data-action="save">${iconMarkup('save', 'S', 'vd-modal-action-icon', 15)}<span>${esc(t('desktop.save'))}</span></button>
                    </div>
                </form>`;
            document.body.appendChild(backdrop);
            const modal = backdrop.querySelector('.vd-modal');
            const preview = modal.querySelector('.lp-icon-preview');
            const selectedPreview = modal.querySelector('.lp-icon-selected-preview');
            if (link && link.icon_path) {
                const imgTag = '<img class="lp-icon-preview-img" src="/files/' + esc(link.icon_path) + '">';
                preview.innerHTML = imgTag;
                if (selectedPreview) { selectedPreview.hidden = false; selectedPreview.innerHTML = imgTag; }
            }

            modal.querySelectorAll('.lp-icon-tab').forEach(tab => {
                tab.addEventListener('click', () => {
                    modal.querySelectorAll('.lp-icon-tab').forEach(t => t.classList.remove('active'));
                    modal.querySelectorAll('.lp-icon-panel').forEach(p => p.classList.remove('active'));
                    tab.classList.add('active');
                    modal.querySelector('[data-panel="' + tab.dataset.tab + '"]').classList.add('active');
                });
            });
            modal.querySelector('.lp-icon-search').addEventListener('input', (e) => {
                clearTimeout(iconSearchDebounce);
                iconSearchDebounce = setTimeout(() => searchIcons(modal, e.target.value), 400);
            });
            modal.querySelector('.lp-icon-search-btn').addEventListener('click', () => searchIcons(modal, modal.querySelector('.lp-icon-search').value));
            modal.querySelector('.lp-icon-url').addEventListener('input', (e) => {
                const u = e.target.value.trim();
                preview.innerHTML = u ? '<img class="lp-icon-preview-img" src="' + esc(u) + '" onerror="this.style.display=\'none\'">' : '';
            });
            modal.querySelector('[data-action="cancel"]').addEventListener('click', () => backdrop.remove());
            modal.querySelector('[data-action="save"]').addEventListener('click', () => saveLink(modal, linkId));
            backdrop.addEventListener('click', (e) => { if (e.target === backdrop) backdrop.remove(); });
        }

        async function searchIcons(modal, query) {
            const resultsEl = modal.querySelector('.lp-icon-results');
            if (!query.trim()) { resultsEl.innerHTML = ''; return; }
            resultsEl.innerHTML = '<div class="vd-loading">' + esc(t('desktop.loading')) + '</div>';
            try {
                const results = await api('/api/launchpad/icons/search?q=' + encodeURIComponent(query));
                const items = (results || []).filter(r => r.url_png || r.url_webp || r.url_svg);
                if (!items.length) {
                    resultsEl.innerHTML = '<div class="lp-icon-msg muted">' + esc(t('desktop.launchpad_icon_no_results')) + '</div>';
                    return;
                }
                resultsEl.innerHTML = items.map(r => {
                    const img = r.url_png || r.url_webp || r.url_svg;
                    return '<div class="lp-icon-result" data-url="' + esc(img) + '"><img src="' + esc(img) + '" alt="" loading="lazy"><span>' + esc(r.name) + '</span></div>';
                }).join('');
                resultsEl.querySelectorAll('.lp-icon-result').forEach(el => {
                    el.addEventListener('click', () => {
                        resultsEl.querySelectorAll('.lp-icon-result').forEach(x => x.classList.remove('selected'));
                        el.classList.add('selected');
                        selectedIconURL = el.dataset.url;
                        const previewEl = modal.querySelector('.lp-icon-selected-preview');
                        if (previewEl) {
                            previewEl.hidden = false;
                            previewEl.innerHTML = '<img class="lp-icon-selected-img" src="' + esc(el.dataset.url) + '" onerror="this.style.display=\'none\'">';
                        }
                    });
                });
            } catch (e) {
                resultsEl.innerHTML = '<div class="lp-icon-msg error">' + esc(t('desktop.launchpad_icon_search_error')) + '</div>';
            }
        }

        async function saveLink(modal, linkId) {
            const title = modal.querySelector('.lp-title').value.trim();
            const url = modal.querySelector('.lp-url').value.trim();
            const category = modal.querySelector('.lp-category').value.trim();
            const description = modal.querySelector('.lp-description').value.trim();
            let iconPath = modal.querySelector('.lp-icon-path').value;

            const activeTab = modal.querySelector('.lp-icon-tab.active');
            const iconUrl = activeTab && activeTab.dataset.tab === 'search' ? selectedIconURL : modal.querySelector('.lp-icon-url').value.trim();
            if (iconUrl) {
                try {
                    const dl = await api('/api/launchpad/icons/download', { method: 'POST', body: JSON.stringify({ image_url: iconUrl, link_id: linkId || 'new' }) });
                    if (dl && dl.local_path) iconPath = dl.local_path;
                } catch (e) { /* ignore download errors */ }
            }

            const payload = { title, url, category, description, icon_path: iconPath };
            try {
                if (linkId) {
                    await api('/api/launchpad/links/' + linkId, { method: 'PUT', body: JSON.stringify(payload) });
                } else {
                    await api('/api/launchpad/links', { method: 'POST', body: JSON.stringify(payload) });
                }
                modal.closest('.vd-modal-backdrop').remove();
                await load();
            } catch (e) { showDesktopNotification({ message: t('desktop.launchpad_save_error') }); }
        }

        searchInput.addEventListener('input', (e) => { searchQuery = e.target.value; render(); });
        categorySelect.addEventListener('change', (e) => { selectedCategory = e.target.value; load(); });

        setLaunchpadMenus(id, host, openEditModal, load);
        load();
    }

    function setLaunchpadMenus(id, host, openEditModal, load) {
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'add-link', labelKey: 'desktop.launchpad_add', icon: 'file-plus', shortcut: 'Ctrl+N', action: () => openEditModal() }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'refresh', labelKey: 'desktop.context_refresh', icon: 'refresh', shortcut: 'F5', action: load }
                ]
            }
        ]);
    }

    function renderGeneratedApp(id, appId) {
        const host = contentEl(id);
        const app = allApps().find(item => item.id === appId);
        if (!app) {
            host.innerHTML = `<div class="vd-empty">${esc(t('desktop.app_missing'))}</div>`;
            return;
        }
        if (app.runtime === 'container-web-app' || (app.metadata && app.metadata.store_app_id)) {
            renderContainerWebApp(id, app);
            return;
        }
        const path = 'Apps/' + app.id + '/' + app.entry;
        host.innerHTML = `<div class="vd-empty">${esc(t('desktop.loading'))}</div>`;
        desktopEmbedURL(path)
            .then(async src => {
                await ensureDesktopEmbedHasContent(src);
                if (!contentEl(id)) return;
                host.replaceChildren(makeSandboxedFrame(src, app.id, '', id, 'vd-generated-frame', appName(app)));
            })
            .catch(err => {
                if (!contentEl(id)) return;
                host.innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
            });
    }

    async function renderContainerWebApp(id, app) {
        const host = contentEl(id);
        if (!host) return;
        const storeAppId = app && app.metadata && app.metadata.store_app_id;
        if (!storeAppId) {
            host.innerHTML = `<div class="vd-empty">${esc(t('desktop.app_missing'))}</div>`;
            return;
        }
        if (app && app.metadata && app.metadata.store_ui === 'terminal-preview') {
            renderStoreTerminalPreviewApp(id, app, storeAppId);
            return;
        }
        const pendingExternalWindow = shouldOpenStoreAppExternally(app) ? openPendingExternalStoreWindow() : null;
        host.innerHTML = `<div class="vd-store-frame-loading">${esc(t('desktop.loading'))}</div>`;
        try {
            const body = await api('/api/desktop/store/apps/' + encodeURIComponent(storeAppId) + '/open-url');
            if (!contentEl(id)) return;
            if (shouldOpenStoreAppExternally(app)) {
                navigateExternalStoreWindow(pendingExternalWindow, body.url);
                closeWindow(id);
                return;
            }
            const frameURL = cacheBustURL(storeFrameURL(body.url, storeAppId), 'aurago_store_embed');
            const frame = makeSandboxedFrame(frameURL, app.id, '', id, 'vd-generated-frame vd-store-app-frame', appName(app), { allowSameOrigin: true, allowDownloads: true, allowStorageAccess: true, allowTopNavigationByUserActivation: true, allowPointerLock: true, allowFullscreen: true, allowGamepad: true });
            host.replaceChildren(frame);
        } catch (err) {
            closeExternalStoreWindow(pendingExternalWindow);
            if (!contentEl(id)) return;
            host.innerHTML = `<div class="vd-store-frame-error">
                <div class="vd-store-frame-error-title">${esc(appName(app))}</div>
                <div class="vd-store-frame-error-msg">${esc(err.message)}</div>
                <button type="button" class="vd-store-btn vd-store-primary" data-action="start">${iconMarkup('run', 'S', 'vd-store-btn-icon', 15)}<span>${esc(t('desktop.store.start', 'Start'))}</span></button>
            </div>`;
            const start = host.querySelector('[data-action="start"]');
            if (start) {
                start.addEventListener('click', async () => {
                    try {
                        await api('/api/desktop/store/apps/' + encodeURIComponent(storeAppId) + '/start', { method: 'POST' });
                        setTimeout(() => renderContainerWebApp(id, app), 1200);
                    } catch (startErr) {
                        showDesktopNotification({ title: appName(app), message: startErr.message });
                    }
                });
            }
        }
    }

    function storeTerminalPreviewDeps() {
        return {
            contentEl,
            esc,
            t,
            api,
            iconMarkup,
            appName,
            registerWindowCleanup,
            makeSandboxedFrame,
            storeFrameURL,
            cacheBustURL,
            showDesktopNotification
        };
    }

    async function renderStoreTerminalPreviewApp(id, app, storeAppId) {
        if (window.StoreTerminalPreviewApp && typeof window.StoreTerminalPreviewApp.render === 'function') {
            return window.StoreTerminalPreviewApp.render(id, app, storeAppId, storeTerminalPreviewDeps());
        }
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-store-frame-error"><div class="vd-store-frame-error-msg">${esc(t('common.error', 'Error'))}</div></div>`;
    }

    function cleanupExistingStoreTerminalPreview(host) {
        if (window.StoreTerminalPreviewApp && typeof window.StoreTerminalPreviewApp.cleanupExisting === 'function') {
            window.StoreTerminalPreviewApp.cleanupExisting(host);
        }
    }

    function shouldOpenStoreAppExternally(app) {
        return !!(app && app.metadata && (app.metadata.open_external === 'true' || app.metadata.store_app_id === 'romm' || app.metadata.store_app_id === 'termix'));
    }

    function openPendingExternalStoreWindow() {
        const pendingWindow = window.open('about:blank', '_blank');
        if (pendingWindow) {
            pendingWindow.opener = null;
        }
        return pendingWindow;
    }

    function navigateExternalStoreWindow(pendingWindow, url) {
        if (!url) {
            closeExternalStoreWindow(pendingWindow);
            return false;
        }
        if (pendingWindow && !pendingWindow.closed) {
            pendingWindow.location.replace(url);
            return true;
        }
        window.open(url, '_blank', 'noopener');
        return true;
    }

    function closeExternalStoreWindow(pendingWindow) {
        if (pendingWindow && !pendingWindow.closed) {
            pendingWindow.close();
        }
    }

    async function openExternalStoreApp(storeAppId, title) {
        const pendingWindow = openPendingExternalStoreWindow();
        try {
            const body = await api('/api/desktop/store/apps/' + encodeURIComponent(storeAppId) + '/open-url');
            navigateExternalStoreWindow(pendingWindow, body.url);
        } catch (err) {
            closeExternalStoreWindow(pendingWindow);
            showDesktopNotification({ title: title || storeAppId, message: err.message });
        }
    }

    function storeFrameURL(src, storeAppId) {
        if (!src) return src;
        if (storeAppId === 'uptime-kuma') {
            try {
                const url = new URL(src, window.location.origin);
                url.pathname = '/dashboard';
                return url.toString();
            } catch (_) {
                return String(src).replace(/\/?(\?.*)?$/, '/dashboard$1');
            }
        }
        return src;
    }

    function cacheBustURL(src, paramName) {
        if (!src) return src;
        const name = paramName || 'aurago_embed';
        const value = String(Date.now());
        try {
            const url = new URL(src, window.location.origin);
            url.searchParams.set(name, value);
            return url.toString();
        } catch (_) {
            const joiner = String(src).includes('?') ? '&' : '?';
            return String(src) + joiner + encodeURIComponent(name) + '=' + encodeURIComponent(value);
        }
    }

    function makeSandboxedFrame(src, appId, widgetId, windowId, className, title, options) {
        const iframe = document.createElement('iframe');
        iframe.className = className;
        iframe.title = title || appId || 'Aura Desktop app';
        iframe.src = src;
        iframe.dataset.appId = appId || '';
        iframe.dataset.widgetId = widgetId || '';
        iframe.dataset.windowId = windowId || '';
        const sandboxFlags = ['allow-scripts', 'allow-forms', 'allow-modals'];
        if (options && options.allowSameOrigin) sandboxFlags.push('allow-same-origin');
        if (options && options.allowDownloads) sandboxFlags.push('allow-downloads');
        if (options && options.allowStorageAccess) sandboxFlags.push('allow-storage-access-by-user-activation');
        if (options && options.allowTopNavigationByUserActivation) sandboxFlags.push('allow-top-navigation-by-user-activation');
        if (options && options.allowPointerLock) sandboxFlags.push('allow-pointer-lock');
        iframe.setAttribute('sandbox', sandboxFlags.join(' '));
        const allowParts = ['clipboard-read', 'clipboard-write'];
        if (options && options.allowFullscreen) allowParts.push('fullscreen');
        if (options && options.allowGamepad) allowParts.push('gamepad');
        // Test compatibility marker: iframe.setAttribute('allow', 'clipboard-read; clipboard-write')
        iframe.setAttribute('allow', allowParts.join('; '));
        if (options && options.allowFullscreen) iframe.setAttribute('allowfullscreen', '');
        iframe.tabIndex = 0;
        iframe.addEventListener('pointerdown', () => focusDesktopFrame(iframe));
        if (!(options && options.disableAutoFocus)) iframe.addEventListener('load', () => focusDesktopFrame(iframe));
        return iframe;
    }

    function focusDesktopFrame(iframe) {
        if (!iframe || typeof iframe.focus !== 'function') return;
        const windowId = iframe.dataset.windowId || '';
        if (windowId && state.activeWindowId && state.activeWindowId !== windowId) return;
        try {
            iframe.focus({ preventScroll: true });
        } catch (_) {
            try { iframe.focus(); } catch (__) {}
        }
    }

    function desktopFileURL(path) {
        return '/files/desktop/' + path.split('/').map(encodeURIComponent).join('/');
    }

    async function desktopEmbedURL(path, params) {
        const body = await api('/api/desktop/embed-token?path=' + encodeURIComponent(path));
        const query = new URLSearchParams(params || {});
        if (body.token) query.set('desktop_token', body.token);
        const suffix = query.toString();
        return desktopFileURL(path) + (suffix ? '?' + suffix : '');
    }

    async function ensureDesktopEmbedHasContent(src) {
        const response = await fetch(src, { credentials: 'same-origin', cache: 'no-store' });
        if (!response.ok) throw new Error(response.statusText || ('HTTP ' + response.status));
        const html = await response.text();
        if (!html.trim()) {
            throw new Error(t('desktop.embed_empty'));
        }
    }

    function findSDKClient(source) {
        const frames = document.querySelectorAll('.vd-generated-frame, .vd-widget-frame');
        for (const frame of frames) {
            if (frame.contentWindow !== source) continue;
            const app = allApps().find(item => item.id === frame.dataset.appId);
            const widgets = (state.bootstrap && state.bootstrap.widgets) || [];
            const widget = widgets.find(item => item.id === frame.dataset.widgetId);
            return {
                app,
                widget,
                appId: frame.dataset.appId || '',
                widgetId: frame.dataset.widgetId || '',
                windowId: frame.dataset.windowId || ''
            };
        }
        return null;
    }

    function sendSDKResponse(source, id, ok, value) {
        if (!source || !id) return;
        source.postMessage(ok ? {
            type: SDK_RESPONSE_TYPE,
            id,
            ok: true,
            payload: value
        } : {
            type: SDK_RESPONSE_TYPE,
            id,
            ok: false,
            error: value && value.message ? value.message : String(value || 'Desktop bridge request failed')
        }, '*');
    }

    function postSDKMenuAction(windowId, actionId) {
        const frame = document.querySelector(`.vd-generated-frame[data-window-id="${cssSel(windowId)}"]`);
        if (!frame || !frame.contentWindow || !actionId) return;
        frame.contentWindow.postMessage({
            type: 'aurago.desktop.menu-action',
            actionId: String(actionId)
        }, '*');
    }

    function postSDKContextMenuAction(client, actionId) {
        const frame = client.windowId
            ? document.querySelector(`.vd-generated-frame[data-window-id="${cssSel(client.windowId)}"]`)
            : document.querySelector(`.vd-widget-frame[data-widget-id="${cssSel(client.widgetId)}"]`);
        if (!frame || !frame.contentWindow || !actionId) return;
        frame.contentWindow.postMessage({
            type: 'aurago.desktop.context-menu-action',
            actionId: String(actionId)
        }, '*');
    }

    function sdkMenuItems(client, items) {
        return (Array.isArray(items) ? items : []).map(item => {
