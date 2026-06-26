            const files = body.files || [];
            host.querySelector('.vd-file-list').innerHTML = files.length ? files.map(file => `<div class="vd-file-row" data-type="${esc(file.type)}" data-path="${esc(file.path)}" data-web-path="${esc(file.web_path || '')}" data-media-kind="${esc(file.media_kind || '')}" data-mime-type="${esc(file.mime_type || '')}">
                ${iconMarkup(iconForFile(file), file.type === 'directory' ? 'D' : file.name, 'vd-sprite-file', 26)}
                <span class="vd-file-name">${esc(file.name)}</span>
                <span class="vd-file-meta">${esc(file.type === 'directory' ? t('desktop.folder') : fmtBytes(file.size))}</span>
            </div>`).join('') : `<div class="vd-empty">${esc(t('desktop.empty_folder'))}</div>`;
            host.querySelectorAll('.vd-file-row').forEach(row => {
                row.addEventListener('dblclick', () => {
                    if (row.dataset.type === 'directory') renderFiles(id, row.dataset.path);
                    else openDesktopFileEntry(row);
                });
                row.addEventListener('click', () => {
                    host.querySelectorAll('.vd-file-row').forEach(item => item.classList.toggle('selected', item === row));
                });
                row.addEventListener('contextmenu', event => {
                    event.preventDefault();
                    const actions = [
                        { label: t('desktop.context_open'), icon: 'folder-open', fallback: 'O', action: () => row.dataset.type === 'directory' ? renderFiles(id, row.dataset.path) : openDesktopFileEntry(row) },
                        { label: t('desktop.context_rename'), icon: 'edit', fallback: 'E', action: () => renamePath(row.dataset.path) },
                        { label: t('desktop.context_delete'), icon: 'trash', fallback: 'X', action: () => deletePath(row.dataset.path) },
                    ];
                    if (row.dataset.type === 'file') {
                        const entry = {
                            name: row.querySelector('.vd-file-name') ? row.querySelector('.vd-file-name').textContent : row.dataset.path,
                            path: row.dataset.path,
                            web_path: row.dataset.webPath || '',
                            media_kind: row.dataset.mediaKind || '',
                            mime_type: row.dataset.mimeType || ''
                        };
                        actions.push(
                            { separator: true },
                            { label: t('desktop.fm.add_to_chat'), icon: 'chat', fallback: 'A', action: () => addFileContextToChat(entry) },
                            { label: t('desktop.fm.ask_agent'), icon: 'agent', fallback: 'Q', action: () => askAgentAboutFile(entry) }
                        );
                    }
                    if (row.dataset.webPath) {
                        actions.push({ label: t('desktop.media_download'), icon: 'download', fallback: 'D', action: () => downloadMediaPath(row.dataset.webPath, row.querySelector('.vd-file-name').textContent) });
                    } else if (row.dataset.type === 'file') {
                        actions.push({ label: t('desktop.media_download'), icon: 'download', fallback: 'D', action: () => downloadDesktopPath(row.dataset.path, row.querySelector('.vd-file-name').textContent) });
                    }
                    actions.push(
                        { separator: true },
                        { label: t('desktop.context_properties'), icon: 'info', fallback: 'i', action: () => showProperties(row.querySelector('.vd-file-name').textContent, row.dataset.path) }
                    );
                    showContextMenu(event.clientX, event.clientY, actions);
                });
            });
        } catch (err) {
            host.querySelector('.vd-file-list').innerHTML = `<div class="vd-empty">${esc(err.message)}</div>`;
        }
    }

    function setFallbackFileMenus(id, path) {
        const readonly = !!((state.bootstrap || {}).readonly);
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'new-file', labelKey: 'desktop.new_file', icon: 'file-plus', shortcut: 'Ctrl+N', disabled: readonly, action: () => openApp('editor', { path: workspaceJoinPath(path || state.filesPath, 'untitled.txt'), content: '' }) },
                    { id: 'new-folder', labelKey: 'desktop.new_folder', icon: 'folder-plus', disabled: readonly, action: () => createFolderInPath(path || state.filesPath) }
                ]
            },
            {
                id: 'view',
                labelKey: 'desktop.menu_view',
                items: [
                    { id: 'up', labelKey: 'desktop.up', icon: 'arrow-up', action: () => {
                        const parts = (path || state.filesPath).split('/').filter(Boolean);
                        parts.pop();
                        renderFiles(id, parts.join('/'));
                    } },
                    { id: 'refresh', labelKey: 'desktop.context_refresh', icon: 'refresh', shortcut: 'F5', action: () => renderFiles(id, path || state.filesPath) }
                ]
            }
        ]);
    }


    function openEditorFile(path) {
        openApp('editor', { path });
    }

    function fileExtension(value) {
        const parts = String(value || '').split('.');
        return parts.length > 1 ? parts.pop().toLowerCase() : '';
    }

    function isWriterFile(file) {
        const ext = fileExtension((file && (file.name || file.path)) || '');
        return ext === 'docx' || ext === 'html' || ext === 'htm';
    }

    function isSheetsFile(file) {
        const ext = fileExtension((file && (file.name || file.path)) || '');
        return ext === 'xlsx' || ext === 'xlsm' || ext === 'csv';
    }

    function isViewerFile(file) {
        const ext = fileExtension((file && (file.name || file.path)) || '');
        return ['md', 'pdf', 'docx', 'xlsx', 'xlsm', 'csv'].includes(ext);
    }

    function is3DFile(file) {
        const ext = fileExtension((file && (file.name || file.path)) || '');
        return ['stl'].includes(ext);
    }

    function isPixelImageFile(file) {
        const ext = fileExtension((file && (file.name || file.path)) || '');
        return ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'tiff', 'tif', 'avif'].includes(ext);
    }

    function entryLooksPlayableMedia(entry) {
        const kind = String((entry && entry.media_kind) || '').toLowerCase();
        const mimeType = String((entry && entry.mime_type) || '').toLowerCase();
        const ext = fileExtension((entry && (entry.name || entry.path)) || '');
        return kind === 'audio' || kind === 'video' ||
            mimeType.startsWith('audio/') || mimeType.startsWith('video/') ||
            ['mp3', 'mp4', 'm4a', 'webm', 'ogg', 'opus', 'wav', 'flac', 'mkv', 'mov'].includes(ext);
    }

    function openDesktopFileEntry(row) {
        const entry = {
            name: row.querySelector('.vd-file-name, .vd-icon-label') ? row.querySelector('.vd-file-name, .vd-icon-label').textContent : row.dataset.path,
            path: row.dataset.path,
            web_path: row.dataset.webPath,
            media_kind: row.dataset.mediaKind,
            mime_type: row.dataset.mimeType
        };
        if (fileExtension(entry.name || entry.path) === 'zip') return openApp('zipper', { path: entry.path });
        if (isWriterFile(entry)) return openApp('writer', { path: entry.path });
        if (isSheetsFile(entry)) return openApp('sheets', { path: entry.path });
        if (is3DFile(entry)) return openApp('viewer-3d', { path: entry.path });
        if (isPixelImageFile(entry)) return openApp('pixel', { path: entry.path });
        if (isViewerFile(entry)) return openApp('viewer', { path: entry.path });
        if (entry.web_path || entryLooksPlayableMedia(entry)) return openMediaPreview(entry);
        openEditorFile(entry.path);
    }

    function chatFileContextFromEntry(entry) {
        const path = normalizeDesktopPath((entry && entry.path) || '');
        if (!path) return null;
        return {
            path,
            name: (entry && (entry.name || entry.filename)) || path.split('/').pop() || path,
            web_path: (entry && entry.web_path) || '',
            media_kind: (entry && entry.media_kind) || '',
            mime_type: (entry && entry.mime_type) || ''
        };
    }

    function agentTaskPrompt(entry, task) {
        return desktopText('desktop.agent_task_prompt')
            .replaceAll('{{path}}', entry.path || '')
            .replaceAll('{{name}}', entry.name || entry.path || '')
            .replaceAll('{{task}}', task || '');
    }

    function openAgentChatForFile(file, options) {
        const entry = chatFileContextFromEntry(file);
        if (!entry) return;
        const context = { chat_files: [entry] };
        const sourceApp = String((options && (options.sourceApp || options.source_app)) || '').trim();
        if (sourceApp) context.chat_source_app = sourceApp;
        const task = String((options && options.task) || '').trim();
        if (task) context.chat_prefill = agentTaskPrompt(entry, task);
        if (options && options.autosend === true) context.chat_autosend = true;
        openApp('agent-chat', context);
    }

    function addFileContextToChat(file) {
        const entry = chatFileContextFromEntry(file);
        if (!entry) return;
        openAgentChatForFile(entry);
    }

    function askAgentAboutFile(file) {
        const entry = chatFileContextFromEntry(file);
        if (!entry) return;
        const prompt = desktopText('desktop.chat_ask_file_prompt')
            .replaceAll('{{name}}', entry.name || entry.path);
        openApp('agent-chat', {
            chat_files: [entry],
            chat_prefill: prompt
        });
    }

    function downloadDesktopPath(path, filename) {
        if (!path) return;
        const link = document.createElement('a');
        link.href = '/api/desktop/download?path=' + encodeURIComponent(path);
        link.download = filename || '';
        document.body.appendChild(link);
        link.click();
        link.remove();
    }

    function downloadMediaPath(webPath, filename) {
        if (!webPath) return;
        const link = document.createElement('a');
        link.href = webPath;
        link.download = filename || '';
        document.body.appendChild(link);
        link.click();
        link.remove();
    }

    function mediaPreviewURL(file) {
        if (!file) return '';
        if (file.web_path) {
            const raw = String(file.web_path);
            if (raw.startsWith('/files/documents/')) {
                const url = new URL(raw, window.location.origin);
                url.searchParams.set('inline', '1');
                return url.pathname + url.search;
            }
            return raw;
        }
        if (file.path) {
            const url = new URL('/api/desktop/download', window.location.origin);
            url.searchParams.set('path', file.path);
            url.searchParams.set('inline', '1');
            return url.pathname + url.search;
        }
        return '';
    }

    function mediaDownloadURL(file) {
        if (!file) return '';
        if (file.web_path) return String(file.web_path);
        if (!file.path) return '';
        const url = new URL('/api/desktop/download', window.location.origin);
        url.searchParams.set('path', file.path);
        return url.pathname + url.search;
    }

    function mediaPreviewKind(file) {
        const kind = String((file && file.media_kind) || '').toLowerCase();
        if (kind === 'audio' || kind === 'video' || kind === 'image') return kind;
        const mimeType = String((file && file.mime_type) || '').toLowerCase();
        if (mimeType.startsWith('audio/')) return 'audio';
        if (mimeType.startsWith('video/')) return 'video';
        if (mimeType.startsWith('image/')) return 'image';
        const ext = fileExtension((file && (file.name || file.path)) || '');
        if (['mp3', 'm4a', 'ogg', 'opus', 'wav', 'flac'].includes(ext)) return 'audio';
        if (['mp4', 'webm', 'mkv', 'mov'].includes(ext)) return 'video';
        return kind;
    }

    function openMediaPreview(file) {
        const previewURL = mediaPreviewURL(file);
        if (!file || !previewURL) {
            if (isWriterFile(file)) return openApp('writer', { path: file.path });
            if (isSheetsFile(file)) return openApp('sheets', { path: file.path });
            if (is3DFile(file)) return openApp('viewer-3d', { path: file.path });
            if (isViewerFile(file)) return openApp('viewer', { path: file.path });
            if (file && file.path) openEditorFile(file.path);
            return;
        }
        const kind = mediaPreviewKind(file);
        if (kind === 'document' && !String(file.mime_type || '').startsWith('text/')) {
            window.open(previewURL, '_blank', 'noopener');
            return;
        }
        const overlay = document.createElement('div');
        overlay.className = 'vd-modal-backdrop vd-media-preview-backdrop';
        const body = kind === 'video'
            ? `<video controls autoplay src="${esc(mediaPreviewURL(file))}"></video>`
            : kind === 'audio'
                ? `<audio controls autoplay src="${esc(mediaPreviewURL(file))}"></audio>`
                : kind === 'image'
                    ? `<img src="${esc(previewURL)}" alt="${esc(file.name || '')}">`
                    : `<iframe src="${esc(previewURL)}" title="${esc(file.name || '')}"></iframe>`;
        overlay.innerHTML = `<div class="vd-media-preview" role="dialog" aria-modal="true">
            <div class="vd-media-preview-bar">
                <strong>${esc(file.name || file.path || t('desktop.media_open'))}</strong>
                <div>
                    <a class="vd-button" href="${esc(mediaDownloadURL(file))}" download="${esc(file.name || '')}">${esc(t('desktop.media_download'))}</a>
                    <button class="vd-button vd-button-primary" type="button" data-close>${esc(t('desktop.close'))}</button>
                </div>
            </div>
            <div class="vd-media-preview-body">${body}</div>
        </div>`;
        document.body.appendChild(overlay);
        const close = () => overlay.remove();
        overlay.querySelector('[data-close]').addEventListener('click', close);
        overlay.addEventListener('click', event => { if (event.target === overlay) close(); });
    }

    async function renderEditor(id, path, initialContent) {
        const host = contentEl(id);
        if (!host) return;
        host.innerHTML = `<div class="vd-editor">
            <div class="vd-toolbar">
                <span class="vd-path">${esc(path)}</span>
                <span class="vd-chat-meta" data-status></span>
            </div>
            <textarea spellcheck="false"></textarea>
        </div>`;
        const textarea = host.querySelector('textarea');
        const status = host.querySelector('[data-status]');
        textarea.value = initialContent;
        if (!initialContent) {
            try {
                const body = await api('/api/desktop/file?path=' + encodeURIComponent(path));
                textarea.value = body.content || '';
            } catch (_) {
                textarea.value = '';
            }
        }
        const saveEditor = async () => {
            status.textContent = t('desktop.saving');
            try {
                await api('/api/desktop/file', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path, content: textarea.value })
                });
                status.textContent = t('desktop.saved');
                await loadBootstrap();
            } catch (err) {
                status.textContent = err.message;
                throw err;
            }
        };
        setEditorMenus(id, path, textarea, status, saveEditor);
    }

    function setEditorMenus(id, path, textarea, status, saveEditor) {
        const readonly = !!((state.bootstrap || {}).readonly);
        setWindowMenus(id, [
            {
                id: 'file',
                labelKey: 'desktop.menu_file',
                items: [
                    { id: 'save', labelKey: 'desktop.save', icon: 'save', shortcut: 'Ctrl+S', disabled: readonly, action: saveEditor }
                ]
            },
            {
                id: 'edit',
                labelKey: 'desktop.menu_edit',
                items: [
                    { id: 'cut', labelKey: 'desktop.fm.cut', icon: 'scissors', shortcut: 'Ctrl+X', disabled: readonly, action: () => document.execCommand && document.execCommand('cut') },
                    { id: 'copy', labelKey: 'desktop.fm.copy', icon: 'copy', shortcut: 'Ctrl+C', action: () => document.execCommand && document.execCommand('copy') },
                    { id: 'paste', labelKey: 'desktop.fm.paste', icon: 'clipboard', shortcut: 'Ctrl+V', disabled: readonly, action: async () => {
                        if (!textarea || readonly) return;
                        if (navigator.clipboard && navigator.clipboard.readText) {
                            const text = await navigator.clipboard.readText().catch(() => '');
                            textarea.setRangeText(text, textarea.selectionStart, textarea.selectionEnd, 'end');
                            textarea.dispatchEvent(new Event('input', { bubbles: true }));
                        }
                    } },
                    { type: 'separator' },
                    { id: 'select-all', labelKey: 'desktop.fm.select_all', icon: 'check-square', shortcut: 'Ctrl+A', action: () => textarea && textarea.select() }
                ]
            },
            {
                id: 'agent',
                labelKey: 'desktop.menu_agent',
                items: [
                    { id: 'agent-task', labelKey: 'desktop.agent_task_for_agent', icon: 'agent', action: async () => {
                        const task = await promptDialog(t('desktop.agent_task_title'), '');
                        if (!task) return;
                        await saveEditor();
                        openAgentChatForFile({ path }, { task, autosend: true, sourceApp: 'editor' });
                    } },
                    { id: 'agent-send-chat', labelKey: 'desktop.agent_send_to_chat', icon: 'chat', action: async () => {
                        await saveEditor();
                        openAgentChatForFile({ path }, { sourceApp: 'editor' });
                    } }
                ]
            },
            {
                id: 'help',
                labelKey: 'desktop.menu_help',
                items: [
                    { id: 'path', label: path, icon: 'text', disabled: true, action: () => { if (status) status.textContent = path; } }
                ]
            }
        ]);
    }

    function evaluateProgrammerExpression(expression, base) {
        const tokens = tokenizeProgrammerExpression(expression, base);
        return parseProgrammerExpression(tokens);
    }

    function tokenizeProgrammerExpression(expression, base) {
        const tokens = [];
        let index = 0;
        const isDigit = ch => {
            if (base === 8) return ch >= '0' && ch <= '7';
            if (base === 10) return ch >= '0' && ch <= '9';
