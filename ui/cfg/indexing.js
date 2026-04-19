// cfg/indexing.js — File Indexing section module

        async function renderIndexingSection(section) {
            const content = document.getElementById('content');

            let html = '<div class="cfg-section active">';
            html += '<div class="section-header">' + section.icon + ' ' + section.label + '</div>';
            html += '<div class="section-desc">' + section.desc + '</div>';

            // Fetch status
            let status = null;
            try {
                const resp = await fetch('/api/indexing/status');
                if (resp.ok) status = await resp.json();
            } catch (_) {}

            const isEnabled = status && status.enabled;

            // ── General Settings Card ──
            html += `<div class="idx-card">
                <div class="idx-card-header">
                    <span class="idx-card-icon">⚙️</span>
                    <span class="idx-card-title">${t('config.indexing.general_settings')}</span>
                </div>
                <div class="idx-card-body">`;

            // Enabled toggle
            const enabledVal = configData.indexing && configData.indexing.enabled;
            html += `<div class="cfg-field idx-field-block">
                <div class="idx-field-row">
                    <label class="idx-field-label">${t('config.indexing.enabled_label')}</label>
                    <div class="toggle ${enabledVal ? 'on' : ''}" onclick="toggleBool(this)" data-path="indexing.enabled"></div>
                </div>
                <div class="idx-field-help">
                    ${t('config.indexing.enabled_desc')}
                </div>
            </div>`;

            // Status card
            if (isEnabled && status.status) {
                const st = status.status;
                html += `<div class="idx-status-card">
                    <div class="idx-status-header">
                        <span class="idx-status-dot">${st.running ? '🟢' : '🔴'}</span>
                        <span class="idx-status-text">
                            ${st.running ? t('config.indexing.running') : t('config.indexing.stopped')}
                        </span>
                        <button class="btn-save idx-rescan-btn" onclick="idxRescan()">
                            🔄 ${t('config.indexing.scan_now')}
                        </button>
                    </div>
                    <div class="idx-status-meta">
                        <span>📁 ${t('config.indexing.files')}: <strong>${st.total_files}</strong></span>
                        <span>🔗 ${t('config.indexing.indexed')}: <strong>${st.indexed_files}</strong></span>
                        ${st.last_scan_at ? `<span>⏱️ ${t('config.indexing.last_scan')}: ${st.last_scan_duration || '—'}</span>` : ''}
                    </div>
                    ${st.errors && st.errors.length > 0 ? `<div class="idx-status-errors">⚠️ ${st.errors.length} ${t('config.indexing.errors')}</div>` : ''}
                </div>`;
            }

            html += `</div></div>`; // close idx-card-body & idx-card (General Settings)

            // ── Directories Card ──
            html += `<div class="idx-card">
                <div class="idx-card-header">
                    <span class="idx-card-icon">📂</span>
                    <span class="idx-card-title">${t('config.indexing.watched_dirs')}</span>
                </div>
                <div class="idx-card-body">`;

            // Directories list
            html += `<div class="idx-dirs-wrap">
                <div id="idx-dirs-list">`;

            // Fetch current directories from API
            let dirs = [];
            try {
                const resp = await fetch('/api/indexing/directories');
                if (resp.ok) {
                    const data = await resp.json();
                    dirs = data.directories || [];
                }
            } catch (_) {}

            if (dirs.length === 0) {
                html += `<div class="idx-empty-state">
                    ${t('config.indexing.no_dirs')}
                </div>`;
            } else {
                dirs.forEach(d => {
                    const safeDir = escapeHtml(d);
                    const attrDir = escapeAttr(d);
                    html += `<div class="idx-dir-item">
                        <span class="idx-dir-icon">📁</span>
                        <span class="idx-dir-path" title="${attrDir}">${safeDir}</span>
                        <button onclick="idxRemoveDir(this.dataset.dir)" data-dir="${attrDir}" class="idx-dir-remove-btn" title="${t('config.indexing.remove')}">✕</button>
                    </div>`;
                });
            }

            html += `</div>
                <div class="idx-add-row">
                    <input type="text" id="idx-new-dir" placeholder="${t('config.indexing.dir_placeholder')}"
                        class="idx-dir-input"
                        onkeydown="if(event.key==='Enter')idxAddDir()">
                    <button class="btn-save idx-add-btn" onclick="idxAddDir()">+ ${t('config.indexing.add')}</button>
                </div>
            </div>`;

            html += `</div></div>`; // close idx-card-body & idx-card (Directories)

            // ── File Types Card (reuse existing idx-types-card inside a card) ──
            html += `<div class="idx-card">
                <div class="idx-card-header">
                    <span class="idx-card-icon">📋</span>
                    <span class="idx-card-title">${t('config.indexing.supported_types')}</span>
                </div>
                <div class="idx-card-body">`;

            // Extensions info
            const exts = (configData.indexing && configData.indexing.extensions) || ['.txt', '.md', '.json', '.csv', '.log', '.yaml', '.yml', '.pdf', '.docx', '.xlsx', '.pptx', '.odt', '.rtf'];
            const docExts = ['.pdf', '.docx', '.xlsx', '.pptx', '.odt', '.rtf'];
            const imgExts = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp'];
            html += `<div class="idx-types-subtitle">
                    ${t('config.indexing.text_files')}
                </div>
                <div class="idx-chip-row">
                    ${exts.filter(e => !docExts.includes(e) && !imgExts.includes(e)).map(e => '<span class="idx-chip idx-chip-text">' + e + '</span>').join('')}
                </div>
                <div class="idx-types-subtitle">
                    ${t('config.indexing.documents')}
                </div>
                <div class="idx-chip-row">
                    ${docExts.map(e => '<span class="idx-chip idx-chip-doc">' + e + '</span>').join('')}
                </div>
                <div class="idx-types-subtitle">
                    ${t('config.indexing.images')}
                </div>
                <div class="idx-chip-row idx-chip-row-last">
                    ${imgExts.map(e => '<span class="idx-chip idx-chip-img">' + e + '</span>').join('')}
                </div>`;

            html += `</div></div>`; // close idx-card-body & idx-card (File Types)

            // ── Advanced Settings Card ──
            html += `<div class="idx-card">
                <div class="idx-card-header">
                    <span class="idx-card-icon">🔧</span>
                    <span class="idx-card-title">${t('config.indexing.advanced_settings')}</span>
                </div>
                <div class="idx-card-body">`;

            // Poll interval
            const poll = (configData.indexing && configData.indexing.poll_interval_seconds) || 60;
            html += `<div class="cfg-field idx-field-block idx-field-block-top">
                <label class="idx-field-label">${t('config.indexing.poll_interval')}</label>
                <input type="number" min="10" max="3600" value="${poll}"
                    data-path="indexing.poll_interval_seconds"
                    oninput="setNestedValue(configData,'indexing.poll_interval_seconds', parseInt(this.value));markDirty()"
                    class="idx-poll-input">
                <div class="idx-field-help idx-field-help-tight">
                    ${t('config.indexing.poll_help')}
                </div>
            </div>`;

            // ── Index Images toggle ──
            const indexImagesVal = configData.indexing && configData.indexing.index_images;
            html += `<div class="cfg-field idx-field-block idx-field-block-xl">
                <div class="idx-field-row">
                    <label class="idx-field-label">🖼️ ${t('config.indexing.index_images_label')}</label>
                    <div class="toggle ${indexImagesVal ? 'on' : ''}" onclick="toggleBool(this)" data-path="indexing.index_images"></div>
                </div>
                <div class="idx-field-help">
                    ${t('config.indexing.index_images_desc')}
                </div>
            </div>`;

            // Cost warning for image indexing
            html += `<div class="idx-image-notice">
                <span class="idx-image-notice-icon">⚠️</span>
                <div class="idx-image-notice-text">
                    <strong class="idx-image-notice-title">${t('config.indexing.image_notice_title')}</strong><br>
                    ${t('config.indexing.image_notice_desc')}
                </div>
            </div>`;

            html += `</div></div>`; // close idx-card-body & idx-card (Advanced Settings)

            html += '</div>';
            content.innerHTML = html;
        }


        async function idxAddDir() {
            const input = document.getElementById('idx-new-dir');
            const path = input.value.trim();
            if (!path) return;

            try {
                const resp = await fetch('/api/indexing/directories', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path })
                });
                if (!resp.ok) {
                    const text = await resp.text();
                    showToast(text || t('config.common.error'), 'error');
                    return;
                }
                input.value = '';
                // Re-render
                const sectionMeta = SECTIONS.flatMap(g => g.items).find(s => s.key === 'indexing');
                await renderIndexingSection(sectionMeta);
            } catch (e) {
                showToast(t('config.common.error') + ': ' + e.message, 'error');
            }
        }

        async function idxRemoveDir(path) {
            if (!(await showConfirm(t('config.indexing.remove_confirm_title'), t('config.indexing.remove_confirm')))) return;
            try {
                const resp = await fetch('/api/indexing/directories', {
                    method: 'DELETE',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path })
                });
                if (!resp.ok) {
                    const text = await resp.text();
                    showToast(text || t('config.common.error'), 'error');
                    return;
                }
                const sectionMeta = SECTIONS.flatMap(g => g.items).find(s => s.key === 'indexing');
                await renderIndexingSection(sectionMeta);
            } catch (e) {
                showToast(t('config.common.error') + ': ' + e.message, 'error');
            }
        }

        async function idxRescan() {
            try {
                const resp = await fetch('/api/indexing/rescan', { method: 'POST' });
                if (resp.ok) {
                    // Re-render after a short delay to allow scan to start
                    setTimeout(async () => {
                        const sectionMeta = SECTIONS.flatMap(g => g.items).find(s => s.key === 'indexing');
                        await renderIndexingSection(sectionMeta);
                    }, 1500);
                }
            } catch (_) {}
        }
