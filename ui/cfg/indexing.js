// cfg/indexing.js — File Indexing section module

        async function renderIndexingSection(section) {
            const content = document.getElementById('content');

            let html = '<div class="cfg-section active">';
            html += '<div class="section-header">' + section.label + '</div>';
            html += '<div class="section-desc">' + section.desc + '</div>';

            // Fetch status
            let status = null;
            try {
                const resp = await fetch('/api/indexing/status');
                if (resp.ok) status = await resp.json();
            } catch (_) {}

            const isEnabled = status && status.enabled;
            const indexingCfg = configData.indexing || (configData.indexing = {});
            const chunkingCfg = indexingCfg.chunking || (indexingCfg.chunking = {});
            const chunkingStrategy = chunkingCfg.strategy || status?.chunking_strategy || 'recursive';
            const chunkMaxChars = chunkingCfg.max_chars || 3500;
            const chunkOverlapChars = chunkingCfg.overlap_chars || 200;
            const chunkMaxChunks = chunkingCfg.max_chunks_per_file || 200;

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
                        <button class="btn-save idx-rescan-btn" id="idx-rescan-btn" onclick="idxRescan()">
                            🔄 ${t('config.indexing.scan_now')}
                        </button>
                        ${cfgActionResult.html('idx-rescan-msg', 'idx-field-help')}
                    </div>
                    <div class="idx-status-meta">
                        <span>📁 ${t('config.indexing.files')}: <strong>${st.total_files}</strong></span>
                        <span>🔗 ${t('config.indexing.indexed')}: <strong>${st.indexed_files}</strong></span>
                        <span>🧩 ${t('config.indexing.indexed_documents')}: <strong>${status.indexed_documents ?? st.indexed_documents ?? 0}</strong></span>
                        <span>${t('config.indexing.chunking_strategy')}: <strong>${escapeHtml(status.chunking_strategy || st.chunking_strategy || chunkingStrategy)}</strong></span>
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

            // Fetch current directories from API
            let dirs = [];
            let mediaAvailable = null; // null = backend did not say
            try {
                const resp = await fetch('/api/indexing/directories');
                if (resp.ok) {
                    const data = await resp.json();
                    dirs = data.directories || [];
                    if (typeof data.media_analysis_available === 'boolean') mediaAvailable = data.media_analysis_available;
                }
            } catch (_) {}

            // Directories list
            const showMediaHint = dirs.length > 0 && mediaAvailable === false;
            html += `<div class="idx-dirs-wrap">`;
            if (showMediaHint) {
                html += `<div class="idx-media-hint" id="idx-media-hint" role="note">${escapeHtml(t('config.indexing.media_inactive_hint'))}</div>`;
            }
            html += `<div id="idx-dirs-list">`;

            if (dirs.length === 0) {
                html += `<div class="idx-empty-state">
                    ${t('config.indexing.no_dirs')}
                </div>`;
            } else {
                dirs.forEach(d => {
                    const dirPath = idxDirPath(d);
                    const dirLabel = idxDirLabel(d);
                    const safeDir = escapeHtml(dirLabel);
                    const attrDir = escapeAttr(dirPath);
                    const mediaOn = idxDirMediaAllowed(d);
                    const mediaLabel = t('config.indexing.media_toggle_label');
                    const mediaName = escapeAttr(mediaLabel + ': ' + dirPath);
                    const mediaTitle = escapeAttr(t('config.indexing.media_toggle_title'));
                    const mediaDesc = showMediaHint ? ' aria-describedby="idx-media-hint"' : '';
                    html += `<div class="idx-dir-item">
                        <span class="idx-dir-icon">📁</span>
                        <span class="idx-dir-path" title="${attrDir}">${safeDir}</span>
                        <div class="idx-media-toggle" title="${mediaTitle}">
                            <span class="idx-media-label" aria-hidden="true">${escapeHtml(mediaLabel)}</span>
                            <button type="button" class="idx-media-switch" role="switch" aria-checked="${mediaOn ? 'true' : 'false'}" aria-label="${mediaName}"${mediaDesc} data-dir="${attrDir}" onclick="idxToggleMediaAnalysis(this)"></button>
                        </div>
                        <button onclick="idxRemoveDir(this.dataset.dir, this)" data-dir="${attrDir}" class="idx-dir-remove-btn" title="${t('config.indexing.remove')}">✕</button>
                    </div>`;
                });
            }

            html += `</div>
                <div class="idx-add-row">
                    <input type="text" id="idx-new-dir" placeholder="${t('config.indexing.dir_placeholder')}"
                        class="idx-dir-input"
                        onkeydown="if(event.key==='Enter')idxAddDir()">
                    <button class="btn-save idx-add-btn" id="idx-add-btn" onclick="idxAddDir()">+ ${t('config.indexing.add')}</button>
                </div>
                ${cfgActionResult.html('idx-dir-msg', 'idx-field-help')}
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
            const exts = (configData.indexing && configData.indexing.extensions) || ['.txt', '.md', '.json', '.csv', '.log', '.yaml', '.yml', '.pdf', '.docx', '.xlsx', '.pptx', '.odt', '.ods', '.odp', '.rtf'];
            const docExts = ['.pdf', '.docx', '.xlsx', '.pptx', '.odt', '.ods', '.odp', '.rtf'];
            const imgExts = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp'];
            html += `<div class="idx-types-subtitle">
                    ${t('config.indexing.text_files')}
                </div>
                <div class="idx-chip-row">
                    ${exts.filter(e => !docExts.includes(e) && !imgExts.includes(e)).map(e => '<span class="idx-chip idx-chip-text">' + escapeHtml(String(e)) + '</span>').join('')}
                </div>
                <div class="idx-types-subtitle">
                    ${t('config.indexing.documents')}
                </div>
                <div class="idx-chip-row">
                    ${docExts.map(e => '<span class="idx-chip idx-chip-doc">' + escapeHtml(String(e)) + '</span>').join('')}
                </div>
                <div class="idx-types-subtitle">
                    ${t('config.indexing.images')}
                </div>
                <div class="idx-chip-row idx-chip-row-last">
                    ${imgExts.map(e => '<span class="idx-chip idx-chip-img">' + escapeHtml(String(e)) + '</span>').join('')}
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
            const poll = indexingCfg.poll_interval_seconds || 60;
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

            html += `<div class="cfg-field idx-field-block idx-field-block-xl">
                <label class="idx-field-label">${t('config.indexing.chunking_strategy')}</label>
                <select data-path="indexing.chunking.strategy"
                    onchange="setNestedValue(configData,'indexing.chunking.strategy', this.value);markDirty()"
                    class="field-select idx-poll-input">
                    <option value="recursive" ${chunkingStrategy === 'recursive' ? 'selected' : ''}>${t('config.indexing.chunking_recursive')}</option>
                    <option value="legacy" ${chunkingStrategy === 'legacy' ? 'selected' : ''}>${t('config.indexing.chunking_legacy')}</option>
                </select>
                <div class="idx-field-help idx-field-help-tight">
                    ${t('config.indexing.chunking_help')}
                </div>
            </div>
            <div class="cfg-field idx-field-block idx-field-block-xl">
                <label class="idx-field-label">${t('config.indexing.chunk_max_chars')}</label>
                <input type="number" min="500" max="50000" value="${chunkMaxChars}"
                    data-path="indexing.chunking.max_chars"
                    oninput="setNestedValue(configData,'indexing.chunking.max_chars', parseInt(this.value));markDirty()"
                    class="idx-poll-input">
            </div>
            <div class="cfg-field idx-field-block idx-field-block-xl">
                <label class="idx-field-label">${t('config.indexing.chunk_overlap_chars')}</label>
                <input type="number" min="1" max="5000" value="${chunkOverlapChars}"
                    data-path="indexing.chunking.overlap_chars"
                    oninput="setNestedValue(configData,'indexing.chunking.overlap_chars', parseInt(this.value));markDirty()"
                    class="idx-poll-input">
            </div>
            <div class="cfg-field idx-field-block idx-field-block-xl">
                <label class="idx-field-label">${t('config.indexing.chunk_max_chunks')}</label>
                <input type="number" min="1" max="2000" value="${chunkMaxChunks}"
                    data-path="indexing.chunking.max_chunks_per_file"
                    oninput="setNestedValue(configData,'indexing.chunking.max_chunks_per_file', parseInt(this.value));markDirty()"
                    class="idx-poll-input">
                <div class="idx-field-help idx-field-help-tight">
                    ${t('config.indexing.chunking_reindex_notice')}
                </div>
            </div>`;

            // ── Index Images toggle ──
            const indexImagesVal = indexingCfg.index_images;
            html += `<div class="cfg-field idx-field-block idx-field-block-xl">
                <div class="idx-field-row">
                    <label class="idx-field-label">🖼️ ${t('config.indexing.index_images_label')}</label>
                    <div class="toggle ${indexImagesVal ? 'on' : ''}" onclick="toggleBool(this)" data-path="indexing.index_images"></div>
                </div>
                <div class="idx-field-help">
                    ${t('config.indexing.index_images_desc')}
                </div>
                <div class="idx-field-help idx-field-help-tight">
                    ${t('config.indexing.index_images_per_folder_hint')}
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


        // Inline message below the folder list / next to the scan button (shared result line).
        function idxSetInlineMsg(id, text, isError) {
            return !!cfgActionResult.show(id, !text ? '' : (isError ? 'error' : 'success'), text || '');
        }

        function idxReloadSection() {
            const sectionMeta = SECTIONS.flatMap(g => g.items).find(s => s.key === 'indexing');
            return renderIndexingSection(sectionMeta);
        }

        async function idxAddDir() {
            const input = document.getElementById('idx-new-dir');
            const path = input.value.trim();
            if (!path) {
                idxSetInlineMsg('idx-dir-msg', t('config.feedback.missing_fields'), true);
                input.focus();
                return { skipped: 'empty' };
            }

            return cfgActionResult(document.getElementById('idx-add-btn'), () => fetch('/api/indexing/directories', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path })
            }), {
                target: 'idx-dir-msg',
                failure: t('config.common.error'),
                onSuccess: async () => {
                    input.value = '';
                    await idxReloadSection();
                    idxSetInlineMsg('idx-dir-msg', t('config.feedback.done'), false);
                }
            });
        }

        async function idxRemoveDir(path, button) {
            if (!(await showConfirm(t('config.indexing.remove_confirm_title'), t('config.indexing.remove_confirm')))) return { skipped: 'cancelled' };
            idxSetInlineMsg('idx-dir-msg', '', true);
            return cfgActionResult(button || document.getElementById('idx-add-btn'), () => fetch('/api/indexing/directories', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ path })
            }), {
                target: 'idx-dir-msg',
                failure: t('config.common.error'),
                onSuccess: async () => {
                    await idxReloadSection();
                    idxSetInlineMsg('idx-dir-msg', t('config.feedback.done'), false);
                }
            });
        }

        // Per-folder opt-in: images, audio and scanned PDFs of a folder may only go to the
        // Vision/multimodal provider after the user enabled it for that folder (default off).
        // Not a config-path field, so it deliberately bypasses toggleBool/data-path.
        let idxMediaToggleBusy = false;

        async function idxToggleMediaAnalysis(btn) {
            if (!btn || idxMediaToggleBusy) return;
            const path = btn.dataset.dir || '';
            if (!path) return;
            const enable = btn.getAttribute('aria-checked') !== 'true';
            const restoreFocus = document.activeElement === btn;
            idxMediaToggleBusy = true;
            btn.setAttribute('aria-disabled', 'true');
            try {
                if (enable && !(await showConfirm(t('config.indexing.media_confirm_title'), t('config.indexing.media_confirm')))) return;
                const resp = await fetch('/api/indexing/directories', {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path, allow_media_analysis: enable })
                });
                if (!resp.ok) {
                    idxSetInlineMsg('idx-dir-msg', t('config.common.error') + ': ' + await idxResponseError(resp), true);
                    return;
                }
                await idxReloadSection();
                idxSetInlineMsg('idx-dir-msg', t('config.feedback.done'), false);
                if (restoreFocus) idxFocusMediaSwitch(path);
            } catch (e) {
                idxSetInlineMsg('idx-dir-msg', t('config.common.error') + ': ' + e.message, true);
            } finally {
                idxMediaToggleBusy = false;
                if (btn.isConnected) btn.removeAttribute('aria-disabled');
            }
        }

        function idxFocusMediaSwitch(path) {
            const sw = Array.from(document.querySelectorAll('.idx-media-switch')).find(b => b.dataset.dir === path);
            if (sw) sw.focus();
        }

        // Backend errors come as plain text or as {"error": "..."}; show the readable part.
        async function idxResponseError(resp) {
            let text = '';
            try { text = (await resp.text()).trim(); } catch (_) {}
            if (text.charAt(0) === '{') {
                try {
                    const body = JSON.parse(text);
                    const msg = body && (body.error || body.message);
                    if (typeof msg === 'string' && msg) return msg;
                } catch (_) {}
            }
            return text || t('config.common.error');
        }

        function idxRescan() {
            return cfgActionResult(document.getElementById('idx-rescan-btn'), () => fetch('/api/indexing/rescan', { method: 'POST' }), {
                target: 'idx-rescan-msg',
                pending: t('config.indexing.rescan_starting'),
                success: t('config.indexing.rescan_started'),
                failure: t('config.indexing.rescan_failed'),
                // Re-render after a short delay to allow the scan to start
                onSuccess: () => setTimeout(() => { if (document.getElementById('idx-rescan-btn')) idxReloadSection(); }, 1500)
            });
        }

        function idxDirPath(dir) {
            if (typeof dir === 'string') return dir;
            if (dir && typeof dir === 'object') return dir.path || dir.Path || '';
            return '';
        }

        function idxDirMediaAllowed(dir) {
            return !!dir && typeof dir === 'object' && dir.allow_media_analysis === true;
        }

        function idxDirLabel(dir) {
            if (typeof dir === 'string') return dir;
            if (!dir || typeof dir !== 'object') return '';
            const path = dir.path || dir.Path || '';
            const collection = dir.collection || dir.Collection || '';
            return collection ? `${path} (${collection})` : path;
        }
