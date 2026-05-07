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

        function showConfirmModal(title, message) {
            return new Promise(resolve => {
                const overlay = document.createElement('div');
                overlay.className = 'vd-qc-modal-overlay';
                overlay.innerHTML = `<div class="vd-qc-confirm">
                    <div class="vd-qc-confirm-title">${esc(title)}</div>
                    <div class="vd-qc-confirm-msg">${esc(message)}</div>
                    <div class="vd-qc-confirm-actions">
                        <button class="vd-qc-btn vd-qc-btn-secondary" type="button" data-action="cancel">${iconMarkup('x', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.cancel'))}</span></button>
                        <button class="vd-qc-btn vd-qc-btn-danger" type="button" data-action="ok">${iconMarkup('trash', 'X', 'vd-qc-btn-icon', 14)}<span>${esc(t('desktop.delete'))}</span></button>
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
                            <input class="vd-qc-input" type="text" name="name" value="${esc(existingDevice ? existingDevice.name : '')}" required>
                        </label>
                        <div class="vd-qc-form-row">
                            <label class="vd-qc-label vd-qc-flex-3">${esc(t('desktop.qc_host'))}
                                <input class="vd-qc-input" type="text" name="host" value="${esc(existingDevice ? (existingDevice.ip_address || '') : '')}" placeholder="192.168.1.1" required>
                            </label>
                            <label class="vd-qc-label vd-qc-flex-1">${esc(t('desktop.qc_port'))}
                                <input class="vd-qc-input" type="number" name="port" value="${existingDevice ? (existingDevice.port || 22) : 22}" min="1" max="65535">
                            </label>
                        </div>
                        <label class="vd-qc-label">${esc(t('desktop.qc_description'))}
                            <input class="vd-qc-input" type="text" name="description" value="${esc(existingDevice ? (existingDevice.description || '') : '')}">
                        </label>
                    </div>
                    <div class="vd-qc-form-section">
                        <div class="vd-qc-form-title">${esc(t('desktop.qc_section_credential'))}</div>
                        <label class="vd-qc-label">${esc(t('desktop.qc_username'))}
                            <input class="vd-qc-input" type="text" name="username" value="${esc(existingCred ? existingCred.username : '')}" required>
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

            // Certificate file upload
            const certFileInput = overlay.querySelector('input[name="certificate_file"]');
            const certTextarea = overlay.querySelector('textarea[name="certificate_text"]');
            certFileInput.addEventListener('change', async (e) => {
                const file = e.target.files[0];
                if (file) {
                    certTextarea.value = await file.text();
                }
            });

            // Password toggle
            overlay.querySelector('[data-action="toggle-pw"]').addEventListener('click', () => {
                const pwInput = overlay.querySelector('input[name="password"]');
                pwInput.type = pwInput.type === 'password' ? 'text' : 'password';
            });

            // Download password
            const dlPwBtn = overlay.querySelector('[data-action="download-pw"]');
            if (dlPwBtn && existingCred) {
                dlPwBtn.addEventListener('click', async () => {
                    try {
                        const body = await api('/api/credentials/export/' + existingCred.id + '?type=password');
                        downloadText(body.content, (existingCred.name || 'password') + '.txt');
                    } catch (err) { showNotify(err.message); }
                });
            }

            // Download certificate
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
                const username = overlay.querySelector('input[name="username"]').value.trim();
                const password = overlay.querySelector('input[name="password"]').value;
                const certificateText = certTextarea.value.trim();

                if (!name || !hostVal || !username) {
                    showNotify(t('desktop.qc_validation_error'));
                    return;
                }

                try {
                    if (isEdit) {
                        // Update credential if exists
                        if (existingCred) {
                            const credBody = { name: name, type: 'ssh', host: hostVal, username: username, description: description, certificate_mode: 'text' };
                            if (password) credBody.password = password;
                            if (certificateText) credBody.certificate_text = certificateText;
                            await api('/api/credentials/' + existingCred.id, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credBody) });
                        } else {
                            // Create credential and link
                            const credBody = { name: name, type: 'ssh', host: hostVal, username: username, description: description, certificate_mode: 'text' };
                            if (password) credBody.password = password;
                            if (certificateText) credBody.certificate_text = certificateText;
                            const created = await api('/api/credentials', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credBody) });
                            existingDevice.credential_id = created.id;
                        }
                        // Update device
                        await api('/api/devices/' + existingDevice.id, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, type: existingDevice.type || 'server', ip_address: hostVal, port, description, credential_id: existingDevice.credential_id }) });
                    } else {
                        // Create credential first
                        const credBody = { name: name, type: 'ssh', host: hostVal, username: username, description: description, certificate_mode: 'text' };
                        if (password) credBody.password = password;
                        if (certificateText) credBody.certificate_text = certificateText;
                        const created = await api('/api/credentials', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(credBody) });
                        // Create device linked to credential
                        await api('/api/devices', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ name, type: 'server', ip_address: hostVal, port, description, credential_id: created.id }) });
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

        async function connectToDevice(deviceId) {
            deviceList.querySelectorAll('.vd-qc-device').forEach(btn => btn.classList.toggle('active', btn.dataset.deviceId === deviceId));
            if (activeWS) { try { activeWS.close(); } catch(_) {} activeWS = null; }
            if (activeTerm) { activeTerm.dispose(); activeTerm = null; }
            terminalArea.innerHTML = `<div class="vd-qc-placeholder"><span class="vd-qc-placeholder-text">${esc(t('desktop.qc_connecting'))}</span></div>`;

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
            terminalArea.replaceChildren(termContainer);
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
            ws.onmessage = (event) => {
                if (typeof event.data === 'string') {
                    try {
                        const msg = JSON.parse(event.data);
                        if (msg.type === 'error') term.write('\r\n\x1b[31m' + msg.message + '\x1b[0m\r\n');
                        else if (msg.type === 'warning' && msg.code === 'insecure_host_key') {
                            const warning = t('desktop.qc_host_key_warning');
                            term.write('\r\n\x1b[33m' + warning + '\x1b[0m\r\n');
                            showNotify(warning);
                        }
                        else if (msg.type === 'disconnected') term.write('\r\n\x1b[33m' + msg.message + '\x1b[0m\r\n');
                    } catch(_) {}
                } else {
                    const bytes = event.data instanceof ArrayBuffer ? new Uint8Array(event.data) : new TextEncoder().encode(event.data);
                    term.write(bytes);
                }
            };
            ws.onclose = () => {
                if (activeWS === ws) { term.write('\r\n\x1b[33m' + t('desktop.qc_disconnected') + '\x1b[0m\r\n'); activeWS = null; }
            };
            ws.onerror = () => { term.write('\r\n\x1b[31m' + t('desktop.qc_connection_error') + '\x1b[0m\r\n'); };
        }
    }

    function setQuickConnectMenus(id, host, loadAll, showServerModal) {
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
                items: [
                    { id: 'refresh', labelKey: 'desktop.qc_refresh', icon: 'refresh', shortcut: 'F5', action: loadAll }
                ]
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
                    <input type="search" class="vd-launchpad-search" data-i18n-placeholder="desktop.launchpad_search" placeholder="${esc(t('desktop.launchpad_search'))}">
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
                const icon = link.icon_path ? '<img class="vd-launchpad-tile-icon" src="/files/' + esc(link.icon_path) + '" alt="" loading="lazy" onerror="this.hidden=true;this.nextElementSibling.hidden=false">' : '';
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
            if (link && link.url) window.open(link.url, '_blank', 'noopener,noreferrer');
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

    function renderChat(id) {
        const host = contentEl(id);
        host.innerHTML = `<div class="vd-chat">
            <div class="vd-chat-log">
                <div class="vd-chat-bubble agent">${esc(t('desktop.chat_welcome'))}</div>
            </div>
            <form class="vd-chat-form">
                <input class="vd-chat-input" autocomplete="off" data-i18n-placeholder="desktop.chat_placeholder">
                <button class="vd-chat-send" type="submit">${iconMarkup('chat', 'S', 'vd-chat-send-icon', 15)}<span>${esc(t('desktop.send'))}</span></button>
            </form>
        </div>`;
        const input = host.querySelector('.vd-chat-input');
        input.placeholder = t('desktop.chat_placeholder');
        host.querySelector('form').addEventListener('submit', async (event) => {
            event.preventDefault();
            if (state.chatBusy) return;
            const message = input.value.trim();
            if (!message) return;
            input.value = '';
            state.chatBusy = true;
            const chatLog = host.querySelector('.vd-chat-log');
            const renderer = window.DesktopChatRenderer;
            if (renderer) {
                renderer.appendRichBubble(chatLog, 'user', message);
            } else {
                appendChat(host, 'user', message);
            }
            try {
                await sendDesktopChatStream(host, message);
                await loadBootstrap();
            } catch (err) {
                const bubbles = host.querySelectorAll('.vd-chat-bubble.agent');
                if (bubbles.length > 0) {
                    bubbles[bubbles.length - 1].textContent = err.message;
                }
            } finally {
                state.chatBusy = false;
            }
        });
    }

    async function sendDesktopChatStream(host, message) {
        const chatLog = host.querySelector('.vd-chat-log');
        const renderer = window.DesktopChatRenderer;
        if (renderer) renderer.resetDedupSets();
        const statusEl = renderer ? renderer.createThinkingStatus() : null;
        if (statusEl) chatLog.appendChild(statusEl);
        let streamingBubble = null;
        let streamingContent = '';
        let streamTextFrame = 0;
        let finalized = false;

        return new Promise((resolve, reject) => {
            const ctrl = new AbortController();
            const timeout = setTimeout(() => {
                ctrl.abort();
                doReject(new Error('Request timed out'));
            }, 10 * 60 * 1000);

            function doFinalize() {
                if (finalized) return;
                finalized = true;
                clearTimeout(timeout);
                flushStreamingBubble();
                if (statusEl && statusEl.parentNode) statusEl.remove();
                if (streamingBubble) {
                    streamingBubble.classList.remove('vd-streaming');
                    if (renderer && streamingContent.trim()) {
                        const html = renderer.renderMarkdown(streamingContent);
                        streamingBubble.innerHTML = html;
                        renderer.processImages(streamingBubble);
                        if (window.MermaidLoader) {
                            window.MermaidLoader.processBlocks(streamingBubble);
                        }
                    }
                }
                resolve();
            }

            function doReject(err) {
                if (finalized) return;
                finalized = true;
                clearTimeout(timeout);
                if (streamTextFrame) {
                    const cancel = window.cancelAnimationFrame || window.clearTimeout;
                    cancel(streamTextFrame);
                    streamTextFrame = 0;
                }
                if (statusEl && statusEl.parentNode) statusEl.remove();
                reject(err);
            }

            function flushStreamingBubble() {
                streamTextFrame = 0;
                if (!streamingBubble || !streamingBubble.classList.contains('vd-streaming')) return;
                streamingBubble.textContent = streamingContent;
                streamingBubble.scrollIntoView({ block: 'end', behavior: 'smooth' });
            }

            function queueStreamingBubbleFlush() {
                if (streamTextFrame) return;
                const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
                streamTextFrame = schedule(flushStreamingBubble);
            }

            fetch('/api/desktop/chat/stream', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ message }),
                signal: ctrl.signal
            }).then(response => {
                if (!response.ok) {
                    return response.text().then(text => {
                        throw new Error(text || ('HTTP ' + response.status));
                    });
                }
                const reader = response.body.getReader();
                const decoder = new TextDecoder();
                let buffer = '';

                function processChunk() {
                    reader.read().then(({ done, value }) => {
                        if (done) {
                            doFinalize();
                            return;
                        }
                        buffer += decoder.decode(value, { stream: true });
                        const lines = buffer.split('\n');
                        buffer = lines.pop();
                        for (const line of lines) {
                            if (line.startsWith('data: ')) {
                                const data = line.slice(6).trim();
                                if (data === '[DONE]') {
                                    doFinalize();
                                    reader.cancel().catch(() => {});
                                    return;
                                }
                                try {
                                    handleStreamEvent(JSON.parse(data));
                                } catch (_) {}
                            }
                        }
                        processChunk();
                    }).catch(err => {
                        doReject(err);
                    });
                }

                function handleStreamEvent(data) {
                    if (!data) return;
                    const event = data.event || data.type;
                    if (data.event === 'llm_stream_delta' || data.type === 'llm_stream_delta') {
                        const content = data.content || '';
                        if (!content) return;
                        if (!streamingBubble) {
                            streamingBubble = document.createElement('div');
                            streamingBubble.className = 'vd-chat-bubble agent vd-streaming';
                            chatLog.appendChild(streamingBubble);
                        }
                        streamingContent += content;
                        if (streamingBubble.classList.contains('vd-streaming')) {
                            queueStreamingBubbleFlush();
                        }
                    } else if (event === 'thinking_block') {
                        const state2 = data.state || '';
                        if (statusEl && state2 === 'start' && renderer) {
                            renderer.updateStatus(statusEl, desktopText('desktop.chat_thinking', 'Reasoning...'));
                        }
                    } else if (event === 'thinking') {
                        if (statusEl && renderer) renderer.updateStatus(statusEl, data.detail || t('desktop.thinking'));
                    } else if (event === 'tool_start') {
                        if (statusEl && renderer) renderer.updateStatus(statusEl, desktopText('desktop.chat_using_tool', 'Using tool') + ': ' + (data.detail || ''));
                    } else if (event === 'tool_end') {
                        if (statusEl && renderer) renderer.updateStatus(statusEl, '');
                    } else if (event === 'image') {
                        try {
                            const imgData = typeof data.detail === 'string' ? JSON.parse(data.detail) : data.detail;
                            if (renderer) renderer.appendImageMessage(chatLog, imgData);
                        } catch (_) {}
                    } else if (event === 'video') {
                        try {
                            const videoData = typeof data.detail === 'string' ? JSON.parse(data.detail) : data.detail;
                            if (renderer) renderer.appendVideoMessage(chatLog, videoData);
                        } catch (_) {}
                    } else if (event === 'audio') {
                        try {
                            const audioData = typeof data.detail === 'string' ? JSON.parse(data.detail) : data.detail;
                            if (renderer) renderer.appendAudioMessage(chatLog, audioData);
                        } catch (_) {}
                    } else if (event === 'document') {
                        try {
                            const docData = typeof data.detail === 'string' ? JSON.parse(data.detail) : data.detail;
                            if (renderer) renderer.appendDocumentMessage(chatLog, docData);
                        } catch (_) {}
                    } else if (event === 'final_response') {
                        if (data.detail || data.message) {
                            const text = data.detail || data.message || '';
                            if (!streamingBubble && text.trim()) {
                                if (renderer) {
                                    renderer.appendRichBubble(chatLog, 'agent', text);
                                } else {
                                    appendChat(host, 'agent', text);
                                }
                            } else if (streamingBubble && !streamingContent.trim() && text.trim()) {
                                streamingContent = text;
                            }
                        }
                    } else if (event === 'done') {
                        return;
                    }
                }

                processChunk();
            }).catch(err => {
                doReject(err);
            });
        });
    }

    function appendChat(host, role, text) {
        const bubble = document.createElement('div');
        bubble.className = 'vd-chat-bubble ' + role;
        bubble.textContent = text;
        host.querySelector('.vd-chat-log').appendChild(bubble);
        bubble.scrollIntoView({ block: 'end' });
    }

    function renderGeneratedApp(id, appId) {
        const host = contentEl(id);
        const app = allApps().find(item => item.id === appId);
        if (!app) {
            host.innerHTML = `<div class="vd-empty">${esc(t('desktop.app_missing'))}</div>`;
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

    function makeSandboxedFrame(src, appId, widgetId, windowId, className, title) {
        const iframe = document.createElement('iframe');
        iframe.className = className;
        iframe.title = title || appId || 'Aura Desktop app';
        iframe.src = src;
        iframe.dataset.appId = appId || '';
        iframe.dataset.widgetId = widgetId || '';
        iframe.dataset.windowId = windowId || '';
        iframe.setAttribute('sandbox', 'allow-scripts allow-forms allow-modals');
        iframe.setAttribute('allow', 'clipboard-read; clipboard-write');
        return iframe;
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
        }, window.location.origin);
    }

    function postSDKContextMenuAction(client, actionId) {
        const frame = client.windowId
            ? document.querySelector(`.vd-generated-frame[data-window-id="${cssSel(client.windowId)}"]`)
            : document.querySelector(`.vd-widget-frame[data-widget-id="${cssSel(client.widgetId)}"]`);
        if (!frame || !frame.contentWindow || !actionId) return;
        frame.contentWindow.postMessage({
            type: 'aurago.desktop.context-menu-action',
            actionId: String(actionId)
        }, window.location.origin);
    }

    function sdkMenuItems(client, items) {
        return (Array.isArray(items) ? items : []).map(item => {
