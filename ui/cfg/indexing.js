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

            // Enabled toggle
            const enabledVal = configData.indexing && configData.indexing.enabled;
            html += `<div class="cfg-field" style="margin-bottom:1.2rem;">
                <div style="display:flex;align-items:center;gap:0.6rem;">
                    <label style="font-size:0.82rem;font-weight:600;color:var(--text-primary);">${t('config.indexing.enabled_label')}</label>
                    <div class="toggle ${enabledVal ? 'on' : ''}" onclick="toggleBool(this)" data-path="indexing.enabled"></div>
                </div>
                <div style="font-size:0.72rem;color:var(--text-secondary);margin-top:0.3rem;">
                    ${t('config.indexing.enabled_desc')}
                </div>
            </div>`;

            // Status card
            if (isEnabled && status.status) {
                const st = status.status;
                html += `<div style="padding:0.8rem 1rem;border-radius:10px;border:1px solid var(--border-subtle);background:var(--bg-secondary);margin-bottom:1.2rem;">
                    <div style="display:flex;align-items:center;gap:0.5rem;margin-bottom:0.5rem;">
                        <span style="font-size:0.9rem;">${st.running ? '🟢' : '🔴'}</span>
                        <span style="font-size:0.82rem;font-weight:600;color:var(--text-primary);">
                            ${st.running ? t('config.indexing.running') : t('config.indexing.stopped')}
                        </span>
                        <button class="btn-save" style="margin-left:auto;padding:0.3rem 0.8rem;font-size:0.72rem;" onclick="idxRescan()">
                            🔄 ${t('config.indexing.scan_now')}
                        </button>
                    </div>
                    <div style="display:flex;flex-wrap:wrap;gap:1rem;font-size:0.75rem;color:var(--text-secondary);">
                        <span>📁 ${t('config.indexing.files')}: <strong>${st.total_files}</strong></span>
                        <span>🔗 ${t('config.indexing.indexed')}: <strong>${st.indexed_files}</strong></span>
                        ${st.last_scan_at ? `<span>⏱️ ${t('config.indexing.last_scan')}: ${st.last_scan_duration || '—'}</span>` : ''}
                    </div>
                    ${st.errors && st.errors.length > 0 ? `<div style="margin-top:0.5rem;font-size:0.72rem;color:var(--danger);">⚠️ ${st.errors.length} ${t('config.indexing.errors')}</div>` : ''}
                </div>`;
            }

            // Directories list
            html += `<div style="margin-top:0.5rem;">
                <div style="font-size:0.85rem;font-weight:600;color:var(--text-primary);margin-bottom:0.6rem;">
                    📂 ${t('config.indexing.watched_dirs')}
                </div>
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
                html += `<div style="padding:1rem;text-align:center;color:var(--text-secondary);font-size:0.8rem;">
                    ${t('config.indexing.no_dirs')}
                </div>`;
            } else {
                dirs.forEach(d => {
                    html += `<div class="idx-dir-item" style="display:flex;align-items:center;gap:0.5rem;padding:0.55rem 0.8rem;border:1px solid var(--border-subtle);border-radius:8px;background:var(--surface-elevated);margin-bottom:0.4rem;">
                        <span style="font-size:0.85rem;">📁</span>
                        <span style="flex:1;font-size:0.8rem;color:var(--text-primary);font-family:monospace;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;" title="${d}">${d}</span>
                        <button onclick="idxRemoveDir('${d.replace(/'/g, "\\'").replace(/\\/g, "\\\\")}')" style="background:none;border:none;color:var(--danger);cursor:pointer;font-size:0.9rem;padding:0.2rem 0.4rem;" title="${t('config.indexing.remove')}">✕</button>
                    </div>`;
                });
            }

            html += `</div>
                <div style="display:flex;gap:0.5rem;margin-top:0.6rem;">
                    <input type="text" id="idx-new-dir" placeholder="${t('config.indexing.dir_placeholder')}"
                        style="flex:1;padding:0.45rem 0.7rem;border:1px solid var(--border-subtle);border-radius:8px;background:var(--input-bg);color:var(--text-primary);font-size:0.8rem;font-family:monospace;"
                        onkeydown="if(event.key==='Enter')idxAddDir()">
                    <button class="btn-save" style="padding:0.45rem 1rem;font-size:0.78rem;" onclick="idxAddDir()">+ ${t('config.indexing.add')}</button>
                </div>
            </div>`;

            // Extensions info
            const exts = (configData.indexing && configData.indexing.extensions) || ['.txt', '.md', '.json', '.csv', '.log', '.yaml', '.yml', '.pdf', '.docx', '.xlsx', '.pptx', '.odt', '.rtf'];
            const docExts = ['.pdf', '.docx', '.xlsx', '.pptx', '.odt', '.rtf'];
            const imgExts = ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp'];
            html += `<div style="margin-top:1.2rem;padding:0.8rem 1rem;border-radius:10px;border:1px solid var(--border-subtle);background:var(--bg-secondary);">
                <div style="font-size:0.78rem;font-weight:600;color:var(--text-secondary);margin-bottom:0.4rem;">
                    📋 ${t('config.indexing.supported_types')}
                </div>
                <div style="font-size:0.7rem;color:var(--text-secondary);margin-bottom:0.3rem;">
                    ${t('config.indexing.text_files')}
                </div>
                <div style="display:flex;flex-wrap:wrap;gap:0.3rem;margin-bottom:0.5rem;">
                    ${exts.filter(e => !docExts.includes(e) && !imgExts.includes(e)).map(e => '<span style="padding:0.15rem 0.5rem;border-radius:6px;background:var(--accent-subtle);color:var(--accent);font-size:0.72rem;font-weight:600;">' + e + '</span>').join('')}
                </div>
                <div style="font-size:0.7rem;color:var(--text-secondary);margin-bottom:0.3rem;">
                    ${t('config.indexing.documents')}
                </div>
                <div style="display:flex;flex-wrap:wrap;gap:0.3rem;margin-bottom:0.5rem;">
                    ${docExts.map(e => '<span style="padding:0.15rem 0.5rem;border-radius:6px;background:rgba(59,130,246,0.12);color:#3b82f6;font-size:0.72rem;font-weight:600;">' + e + '</span>').join('')}
                </div>
                <div style="font-size:0.7rem;color:var(--text-secondary);margin-bottom:0.3rem;">
                    ${t('config.indexing.images')}
                </div>
                <div style="display:flex;flex-wrap:wrap;gap:0.3rem;">
                    ${imgExts.map(e => '<span style="padding:0.15rem 0.5rem;border-radius:6px;background:rgba(168,85,247,0.12);color:#a855f7;font-size:0.72rem;font-weight:600;">' + e + '</span>').join('')}
                </div>
            </div>`;

            // Poll interval
            const poll = (configData.indexing && configData.indexing.poll_interval_seconds) || 60;
            html += `<div class="cfg-field" style="margin-top:1rem;">
                <label style="font-size:0.82rem;font-weight:600;color:var(--text-primary);">${t('config.indexing.poll_interval')}</label>
                <input type="number" min="10" max="3600" value="${poll}"
                    data-path="indexing.poll_interval_seconds"
                    oninput="idxSetVal('indexing.poll_interval_seconds', parseInt(this.value))"
                    style="width:100px;padding:0.4rem 0.6rem;border:1px solid var(--border-subtle);border-radius:8px;background:var(--input-bg);color:var(--text-primary);font-size:0.8rem;margin-top:0.3rem;">
                <div style="font-size:0.7rem;color:var(--text-secondary);margin-top:0.2rem;">
                    ${t('config.indexing.poll_help')}
                </div>
            </div>`;

            // ── Index Images toggle ──
            const indexImagesVal = configData.indexing && configData.indexing.index_images;
            html += `<div class="cfg-field" style="margin-top:1.2rem;">
                <div style="display:flex;align-items:center;gap:0.6rem;">
                    <label style="font-size:0.82rem;font-weight:600;color:var(--text-primary);">🖼️ ${t('config.indexing.index_images_label')}</label>
                    <div class="toggle ${indexImagesVal ? 'on' : ''}" onclick="toggleBool(this)" data-path="indexing.index_images"></div>
                </div>
                <div style="font-size:0.72rem;color:var(--text-secondary);margin-top:0.3rem;">
                    ${t('config.indexing.index_images_desc')}
                </div>
            </div>`;

            // Cost warning for image indexing
            html += `<div style="margin-top:0.6rem;padding:0.7rem 1rem;border-radius:10px;border:1px solid rgba(234,179,8,0.3);background:rgba(234,179,8,0.08);display:flex;align-items:flex-start;gap:0.5rem;">
                <span style="font-size:1rem;line-height:1;">⚠️</span>
                <div style="font-size:0.72rem;color:var(--text-secondary);">
                    <strong style="color:var(--text-primary);">${t('config.indexing.image_notice_title')}</strong><br>
                    ${t('config.indexing.image_notice_desc')}
                </div>
            </div>`;

            html += '</div>';
            content.innerHTML = html;
        }



        function idxSetVal(path, val) {
            const parts = path.split('.');
            let obj = configData;
            for (let i = 0; i < parts.length - 1; i++) {
                if (!obj[parts[i]]) obj[parts[i]] = {};
                obj = obj[parts[i]];
            }
            obj[parts[parts.length - 1]] = val;
            setDirty(true);
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
                    alert(text || t('config.common.error'));
                    return;
                }
                input.value = '';
                // Re-render
                const sectionMeta = SECTIONS.flatMap(g => g.items).find(s => s.key === 'indexing');
                await renderIndexingSection(sectionMeta);
            } catch (e) {
                alert(t('config.common.error') + ': ' + e.message);
            }
        }

        async function idxRemoveDir(path) {
            if (!confirm(t('config.indexing.remove_confirm'))) return;
            try {
                const resp = await fetch('/api/indexing/directories', {
                    method: 'DELETE',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path })
                });
                if (!resp.ok) {
                    const text = await resp.text();
                    alert(text || t('config.common.error'));
                    return;
                }
                const sectionMeta = SECTIONS.flatMap(g => g.items).find(s => s.key === 'indexing');
                await renderIndexingSection(sectionMeta);
            } catch (e) {
                alert(t('config.common.error') + ': ' + e.message);
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
