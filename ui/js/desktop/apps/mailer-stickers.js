(function () {
    'use strict';

    function sticker(attachment) {
        if (!attachment?.inline || !attachment.content_id) return null;
        const match = /^sticker-([a-z-]+)\.png$/.exec(attachment.filename || '');
        return match && window.AuraStickerLibrary?.byId.get(match[1]);
    }

    function html(source, attachments) {
        const root = document.createElement('template');
        root.innerHTML = source;
        for (const image of root.content.querySelectorAll('img[src^="cid:"]')) {
            const attachment = (attachments || []).find(item => 'cid:' + item.content_id === image.getAttribute('src'));
            if (sticker(attachment)) { image.width = 192; image.height = 192; }
            else if (!attachment) image.replaceWith(document.createTextNode(image.alt));
        }
        return root.innerHTML;
    }

    function mount(inst, actions) {
        const library = window.AuraStickerLibrary;
        const panel = inst.host.querySelector('[data-mailer-stickers]');
        const toggle = inst.host.querySelector('[data-stickers-open]');
        const pending = new Map();
        const abort = new AbortController();
        let bookmark = null, fallbackSelection = null;
        panel.id = 'mailer-stickers-' + inst.windowId;
        panel.setAttribute('role', 'region');
        panel.setAttribute('aria-label', inst.ctx.t('stickers.title'));
        toggle.setAttribute('aria-controls', panel.id);
        const picker = library.mount(panel, {
            t: key => inst.ctx.t(key),
            async onSelect(id) {
                if (actions.editingLocked(inst) || inst.preserveHTML) return;
                await inst.editorPromise;
                await actions.queueUpload(inst, async () => {
                    const file = await library.file(id, abort.signal);
                    abort.signal.throwIfAborted();
                    pending.set(file, { id, bookmark, fallbackSelection });
                    return [file];
                });
            }
        });
        function close() {
            panel.hidden = true;
            toggle.setAttribute('aria-expanded', 'false');
            picker.setActive(false);
        }
        toggle.addEventListener('click', () => {
            if (actions.editingLocked(inst) || inst.preserveHTML) return;
            if (!panel.hidden) { close(); return; }
            bookmark = inst.editorView?.state.selection.getBookmark() || null;
            const fallback = inst.host.querySelector('[data-body-fallback]');
            fallbackSelection = [fallback.selectionStart, fallback.selectionEnd];
            panel.hidden = false;
            toggle.setAttribute('aria-expanded', 'true');
            picker.setActive(true);
            panel.querySelector('input').focus();
        }, { signal: abort.signal });
        panel.addEventListener('keydown', event => {
            if (event.key === 'Escape') { event.preventDefault(); close(); toggle.focus(); }
        }, { signal: abort.signal });

        return {
            forget(file) { pending.delete(file); },
            remove(attachment) {
                if (!sticker(attachment)) return;
                const src = 'cid:' + attachment.content_id;
                if (inst.editorView) {
                    const view = inst.editorView, tr = view.state.tr;
                    view.state.doc.descendants((node, pos) => {
                        if (node.type.name === 'image' && node.attrs.src === src) tr.delete(tr.mapping.map(pos), tr.mapping.map(pos + node.nodeSize));
                    });
                    if (tr.docChanged) view.dispatch(tr);
                } else {
                    const escaped = src.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
                    inst.setMarkdown(inst.markdown().replace(new RegExp('!\\[[^\\]]*\\]\\(' + escaped + '\\)', 'g'), ''));
                }
            },
            map(mapping) {
                bookmark = bookmark?.map(mapping);
                for (const item of pending.values()) item.bookmark = item.bookmark?.map(mapping);
            },
            sync() {
                const disabled = actions.editingLocked(inst) || inst.preserveHTML || inst.attachmentBusy;
                toggle.disabled = disabled;
                picker.setDisabled(disabled);
            },
            imageView(node) {
                const dom = document.createElement('img');
                const attachment = (inst.draft.attachments || []).find(item => 'cid:' + item.content_id === node.attrs.src);
                const item = sticker(attachment);
                dom.alt = node.attrs.alt || '';
                if (node.attrs.title) dom.title = node.attrs.title;
                if (item) {
                    dom.src = library.url(item.id, true);
                    dom.width = dom.height = 192;
                    dom.className = 'vd-mailer-sticker-image';
                } else if (!String(node.attrs.src).startsWith('cid:')) dom.src = node.attrs.src;
                return { dom };
            },
            uploaded(file, previousIDs) {
                const item = pending.get(file);
                if (!item || inst.disposed) return;
                const attachment = inst.draft.attachments.find(value => !previousIDs.has(value.id));
                if (!attachment) throw new Error('sticker_upload_missing');
                attachment.inline = true;
                attachment.content_id = attachment.id + '@aurago';
                const src = 'cid:' + attachment.content_id;
                const alt = inst.ctx.t('stickers.name.' + item.id);
                if (inst.editorView) {
                    const view = inst.editorView;
                    const tr = inst.editorLib.closeHistory(view.state.tr);
                    if (item.bookmark) tr.setSelection(item.bookmark.resolve(view.state.doc));
                    tr.replaceSelectionWith(view.state.schema.nodes.image.create({ src, alt }));
                    view.dispatch(tr.scrollIntoView());
                    view.focus();
                } else {
                    const field = inst.host.querySelector('[data-body-fallback]');
                    const [start, end] = item.fallbackSelection || [field.value.length, field.value.length];
                    field.setRangeText('![' + alt.replace(/[\\\[\]]/g, '\\$&') + '](' + src + ')', start, end, 'end');
                    field.focus();
                }
                pending.delete(file);
                inst.bodyChanged = true;
                actions.changed(inst);
                close();
            },
            dispose() { abort.abort(); pending.clear(); picker.dispose(); }
        };
    }
    window.MailerStickers = { mount, html };
})();
