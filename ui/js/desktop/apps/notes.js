(function () {
    'use strict';

    window.NotesApp = {
        render(host, windowId, ctx) {
            if (!host) return;
            const readonly = !!(ctx.readonly);
            let currentPath = (ctx.context && ctx.context.path) || '';
            let dirty = false;
            host.innerHTML = `<div class="vd-notes-app">
                <div class="vd-notes-toolbar">
                    <button type="button" class="vd-notes-btn" data-action="new"${readonly ? ' disabled' : ''}>${ctx.esc(ctx.t('desktop.notes_new'))}</button>
                    <button type="button" class="vd-notes-btn" data-action="open">${ctx.esc(ctx.t('desktop.file_dialog_open'))}</button>
                    <button type="button" class="vd-notes-btn" data-action="save"${readonly ? ' disabled' : ''}>${ctx.esc(ctx.t('desktop.writer_save'))}</button>
                    <span class="vd-notes-path" data-notes-path></span>
                </div>
                <textarea class="vd-notes-editor" data-notes-editor spellcheck="true"${readonly ? ' readonly' : ''} placeholder="${ctx.esc(ctx.t('desktop.notes_placeholder'))}"></textarea>
            </div>`;
            const editor = host.querySelector('[data-notes-editor]');
            const pathLabel = host.querySelector('[data-notes-path]');
            function setPathLabel() {
                if (pathLabel) pathLabel.textContent = currentPath || ctx.t('desktop.notes_unsaved');
            }
            async function loadPath(path) {
                const body = await ctx.api('/api/desktop/files/read?path=' + encodeURIComponent(path));
                editor.value = body.content || '';
                currentPath = path;
                dirty = false;
                setPathLabel();
                if (typeof ctx.recordRecentFile === 'function') ctx.recordRecentFile(path, 'notes');
            }
            async function saveCurrent() {
                if (readonly) return;
                let path = currentPath;
                if (!path) {
                    path = 'Documents/Notes/' + (editor.value.split('\n')[0] || 'note').slice(0, 40).replace(/[^\w\- ]+/g, '').trim() + '.md';
                    if (!path.endsWith('.md')) path += '.md';
                }
                await ctx.api('/api/desktop/files/write', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path, content: editor.value })
                });
                currentPath = path;
                dirty = false;
                setPathLabel();
                ctx.notify({ title: ctx.t('desktop.notification'), message: ctx.t('desktop.saved') });
            }
            editor.addEventListener('input', () => { dirty = true; });
            host.querySelector('[data-action="save"]').addEventListener('click', () => saveCurrent().catch(err => ctx.notify({ title: ctx.t('desktop.notification'), message: err.message })));
            host.querySelector('[data-action="new"]').addEventListener('click', () => {
                currentPath = '';
                editor.value = '';
                dirty = false;
                setPathLabel();
                editor.focus();
            });
            host.querySelector('[data-action="open"]').addEventListener('click', async () => {
                if (typeof ctx.openFileDialog !== 'function') return;
                const picked = await ctx.openFileDialog({ title: ctx.t('desktop.file_dialog_open'), path: 'Documents/Notes' });
                if (picked && picked.path) loadPath(picked.path).catch(err => ctx.notify({ title: ctx.t('desktop.notification'), message: err.message }));
            });
            setPathLabel();
            if (currentPath) loadPath(currentPath).catch(err => ctx.notify({ title: ctx.t('desktop.notification'), message: err.message }));
        },
        dispose() {}
    };
})();
