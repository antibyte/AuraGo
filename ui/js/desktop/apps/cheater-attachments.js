(function () {
    'use strict';

    const ALLOWED_EXTENSIONS = ['.txt', '.md'];
    const MAX_UPLOAD_BYTES = 1 * 1024 * 1024;
    const MAX_ATTACHMENT_CHARS = 25000;

    function openAttachmentPanel(state) {
        const t = state.t;
        const esc = state.esc;
        const panel = document.createElement('aside');
        panel.className = 'cheater-attach-panel';
        panel.setAttribute('aria-label', t('cheater.attachments'));
        panel.innerHTML = `
            <header class="cheater-attach-header">
                <h3>${esc(t('cheater.attachments'))}</h3>
                <button type="button" class="cheater-attach-close" data-action="close" aria-label="${esc(t('cheater.close'))}">×</button>
            </header>
            <div class="cheater-attach-drop" data-drop>
                <p>${esc(t('cheater.attach_drop_hint'))}</p>
                <input type="file" data-file hidden>
            </div>
            <ul class="cheater-attach-list" data-list role="list"></ul>
        `;
        document.body.appendChild(panel);

        const dropZone = panel.querySelector('[data-drop]');
        const fileInput = panel.querySelector('[data-file]');
        const list = panel.querySelector('[data-list]');
        const closeBtn = panel.querySelector('[data-action="close"]');

        async function refresh() {
            try {
                const data = await state.api('/api/cheatsheets/' + encodeURIComponent(state.sheet.id));
                state.sheet.attachments = data.attachments || [];
                render();
            } catch (err) {
                console.error('cheater attach refresh failed', err);
            }
        }

        function render() {
            const items = (state.sheet.attachments || []);
            list.innerHTML = items.length === 0
                ? `<li class="cheater-attach-empty">${esc(t('cheater.attach_empty'))}</li>`
                : items.map(a => `<li class="cheater-attach-item" data-id="${esc(a.id)}">
                    <span class="cheater-attach-icon">📄</span>
                    <span class="cheater-attach-name">${esc(a.filename)}</span>
                    <span class="cheater-attach-size">${esc(String(a.char_count || 0))} ${esc(t('cheater.chars'))}</span>
                    <button type="button" class="cheater-attach-delete" data-action="delete" data-id="${esc(a.id)}" aria-label="${esc(t('cheater.delete'))}">🗑️</button>
                </li>`).join('');
            const countNode = state.host.querySelector('[data-attach-count]');
            if (countNode) countNode.textContent = String(items.length);
        }

        dropZone.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', () => {
            if (fileInput.files && fileInput.files[0]) uploadFile(fileInput.files[0]);
        });
        ['dragover', 'dragenter'].forEach(ev => dropZone.addEventListener(ev, e => { e.preventDefault(); dropZone.classList.add('is-drag'); }));
        ['dragleave', 'drop'].forEach(ev => dropZone.addEventListener(ev, e => { e.preventDefault(); dropZone.classList.remove('is-drag'); }));
        dropZone.addEventListener('drop', (e) => {
            if (e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0]) uploadFile(e.dataTransfer.files[0]);
        });

        list.addEventListener('click', (e) => {
            const btn = e.target.closest('[data-action="delete"]');
            if (!btn) return;
            const id = btn.dataset.id;
            deleteAttachment(id);
        });

        closeBtn.addEventListener('click', close);
        document.addEventListener('keydown', onKey);

        function onKey(e) {
            if (e.key === 'Escape') { e.preventDefault(); close(); }
        }

        async function uploadFile(file) {
            const name = String(file && file.name || '');
            const lower = name.toLowerCase();
            if (!ALLOWED_EXTENSIONS.some(ext => lower.endsWith(ext))) {
                state.notify('cheater.error.invalid_attachment_type', 'error');
                return;
            }
            if (file.size > MAX_UPLOAD_BYTES) {
                state.notify('cheater.error.attachment_too_large', 'error');
                return;
            }
            const text = await file.text().catch(() => '');
            if ([...text].length > MAX_ATTACHMENT_CHARS) {
                state.notify('cheater.error.attachment_too_large', 'error');
                return;
            }
            const form = new FormData();
            form.append('file', file);
            try {
                await state.api('/api/cheatsheets/' + encodeURIComponent(state.sheet.id) + '/attachments', {
                    method: 'POST',
                    body: form
                });
                if (fileInput) fileInput.value = '';
                await refresh();
            } catch (err) {
                state.notify('cheater.error.upload_failed', 'error');
                console.error('cheater upload failed', err);
            }
        }

        async function deleteAttachment(id) {
            const toast = showToast(t('cheater.attach_undeleted') + ' · ' + t('cheater.undo'), () => {
                clearTimeout(timer);
                refresh();
            });
            const timer = setTimeout(async () => {
                try {
                    await state.api('/api/cheatsheets/' + encodeURIComponent(state.sheet.id) + '/attachments/' + encodeURIComponent(id), { method: 'DELETE' });
                } catch (err) {
                    console.error('cheater delete attachment failed', err);
                }
                toast.close();
            }, 5000);
        }

        function showToast(message, onUndo) {
            const el = document.createElement('div');
            el.className = 'cheater-toast';
            el.innerHTML = `<span>${esc(message)}</span><button type="button" data-undo>${esc(t('cheater.undo'))}</button>`;
            el.querySelector('[data-undo]').addEventListener('click', () => { onUndo(); el.remove(); });
            document.body.appendChild(el);
            return { close: () => el.remove() };
        }

        function close() {
            panel.remove();
            document.removeEventListener('keydown', onKey);
        }

        render();
    }

    window.CheaterAttachments = window.CheaterAttachments || {};
    window.CheaterAttachments.open = openAttachmentPanel;
})();
