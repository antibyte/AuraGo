(function () {
    'use strict';

    const active = new Map();
    const editorLib = () => import('/js/vendor/notes/engine.js?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev'));

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }

    function t(ctx, key, values) {
        return ctx && typeof ctx.t === 'function' ? ctx.t('mailer.' + key, values) : 'mailer.' + key;
    }

    function providerRows(ctx) {
        const stateRows = Array.isArray(ctx?.state?.providers) ? ctx.state.providers : [];
        const passedRows = Array.isArray(ctx?.providers) ? ctx.providers : [];
        return stateRows.length ? stateRows : passedRows.length ? passedRows : stateRows;
    }

    function defaultProviderLabel(ctx) {
        const id = ctx.state?.default_provider_id || '';
        const provider = providerRows(ctx).find(item => item.id === id);
        return provider ? `${t(ctx, 'provider_default')} · ${provider.name || provider.id} · ${provider.model || provider.type || ''}` : t(ctx, 'provider_default');
    }

    function emailList(value) {
        return (Array.isArray(value) ? value : String(value || '').split(/[;,\n]+/))
            .map(item => typeof item === 'string' ? item.trim() : item.email || '')
            .filter(Boolean);
    }

    function validAddress(address) {
        return /^[^\s<>@,;]+@[^\s<>@,;]+\.[^\s<>@,;]+$/.test(address);
    }

    function cleanHtml(markdown, attachments) {
        const parser = window.marked;
        const html = parser && typeof parser.parse === 'function' ? parser.parse(markdown || '') : '<p>' + esc(markdown).replace(/\n/g, '<br>') + '</p>';
        const safe = window.DOMPurify ? window.DOMPurify.sanitize(html, { FORBID_TAGS: ['script', 'iframe', 'form', 'object', 'embed', 'style'], FORBID_ATTR: ['style'] }) : esc(markdown).replace(/\n/g, '<br>');
        return window.MailerStickers ? window.MailerStickers.html(safe, attachments) : safe;
    }

    function normalizeHTML(html) {
        const safe = window.DOMPurify ? window.DOMPurify.sanitize(String(html || ''), { FORBID_TAGS: ['script', 'form', 'iframe', 'object', 'embed', 'svg', 'math', 'base'], FORBID_ATTR: ['srcset', 'action', 'formaction'], ALLOW_DATA_ATTR: false }) : String(html || '');
        return safe.replace(/\s+/g, ' ').replace(/>\s+</g, '><').trim();
    }

    function previewHTML(html) {
        const safe = normalizeHTML(html);
        return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; img-src 'none'; style-src 'unsafe-inline'; font-src 'none'; connect-src 'none'; form-action 'none'; base-uri 'none'"><meta name="referrer" content="no-referrer"></head><body>${safe}</body></html>`;
    }

    function recipientGroup(name, label, people, ctx) {
        return `<div class="vd-mailer-recipient-row" data-recipient-row="${name}">
            <span class="vd-mailer-field-label">${esc(label)}</span>
            <div class="vd-mailer-recipient-control"><div class="vd-mailer-recipient-chips" data-chips="${name}" role="list"></div>
                <input type="email" data-recipient-input="${name}" autocomplete="off" aria-label="${esc(label)}" placeholder="${esc(t(ctx, 'recipient_placeholder'))}">
                <div class="vd-mailer-contact-suggestions" data-suggestions="${name}" role="listbox" hidden></div>
            </div>
        </div>`;
    }

    function render(host, windowId, ctx, options) {
        dispose(windowId);
        const api = window.MailerAPI.create(ctx.api);
        const draft = Object.assign({ to: [], cc: [], bcc: [], subject: '', text: '', html: '', reply_remote_id: '', attachments: [], revision: 0 }, options.draft || {});
        const accounts = ctx.state.accounts || [];
        const account = accounts.find(item => item.id === (draft.account_id || options.accountId)) || accounts.find(item => !item.disabled) || accounts[0];
        const inst = {
            windowId, host, ctx, api, draft, accountId: account ? account.id : '', readonly: !!ctx.readonly,
            recipients: { to: emailList(draft.to), cc: emailList(draft.cc), bcc: emailList(draft.bcc) },
            contacts: [], editor: null, disposed: false, editorPromise: null, saveTimer: 0,
            saveQueue: Promise.resolve(), editGeneration: 0, savedGeneration: 0, saveError: null,
            assistController: null, assistPreview: null, uploads: new Map(), failedUploads: new Map(), uploadQueueCount: 0, attachmentBusy: false,
            committed: false, sending: false, sendLockSnapshot: null, sendLockActive: false, draftConflict: false, conflictResolving: false, conflictMissingAttachments: [], conflictLocalPayload: null, conflictLocalAttachments: [], editorLib: null, editorView: null,
            forwardSources: (options.forwardMessage?.attachments || []).map(item => ({
                messageId: String(options.forwardMessage.id || ''),
                attachmentId: String(item.id || ''),
                filename: String(item.filename || item.name || ''),
                size: Number(item.size) || 0
            })).filter(item => item.messageId && item.attachmentId),
            forwardCopied: new Set(), forwardSkipped: new Set(), forwardCopying: false, forwardCopyPromise: Promise.resolve(), attachmentTask: Promise.resolve(),
            preserveHTML: !!draft.html && normalizeHTML(draft.html) !== normalizeHTML(cleanHtml(draft.text || '', draft.attachments)),
            bodyChanged: false,
            statusTimer: 0, listeners: [], onClose: null, uploadTask: Promise.resolve(), uploadController: new AbortController()
        };
        active.set(windowId, inst);
        const composeTitle = options.replyType === 'forward' ? t(ctx, 'forward') : options.replyType ? t(ctx, 'reply') : t(ctx, 'compose');
        host.innerHTML = `<section class="vd-mailer-compose" aria-label="${esc(composeTitle)}">
            <header class="vd-mailer-compose-head"><div><p class="vd-mailer-eyebrow">${esc(composeTitle)}</p><h2>${esc(t(ctx, 'compose_title'))}</h2></div>
                <span class="vd-mailer-save-state" data-save-state role="status" aria-live="polite">${esc(t(ctx, draft.id ? 'draft_saved' : 'draft_unsaved'))}</span></header>
            <form class="vd-mailer-compose-form" data-compose-form novalidate>
                <label class="vd-mailer-compose-row"><span>${esc(t(ctx, 'from'))}</span><select name="account_id" data-account ${inst.readonly ? 'disabled' : ''}>${accounts.map(item => `<option value="${esc(item.id)}" ${item.id === inst.accountId ? 'selected' : ''}>${esc(item.name || item.address)} · ${esc(item.address)}</option>`).join('')}</select></label>
                <div class="vd-mailer-compose-snippets"><label><span>${esc(t(ctx, 'insert_template'))}</span><select data-insert-template ${inst.readonly || inst.preserveHTML ? 'disabled' : ''}><option value="">${esc(t(ctx, 'choose_template'))}</option></select></label><label><span>${esc(t(ctx, 'insert_signature'))}</span><select data-insert-signature ${inst.readonly || inst.preserveHTML ? 'disabled' : ''}><option value="">${esc(t(ctx, 'choose_signature'))}</option></select></label></div>
                ${recipientGroup('to', t(ctx, 'to'), inst.recipients.to, ctx)}
                <div class="vd-mailer-recipient-tools"><button class="vd-mailer-text-button" type="button" data-add-copy="cc" ${inst.readonly ? 'disabled' : ''} ${draft.cc?.length ? 'hidden' : ''}>${esc(t(ctx, 'show_cc'))}</button><button class="vd-mailer-text-button" type="button" data-add-copy="bcc" ${inst.readonly ? 'disabled' : ''} ${draft.bcc?.length ? 'hidden' : ''}>${esc(t(ctx, 'show_bcc'))}</button></div>
                <div data-copy-row="cc" ${draft.cc?.length ? '' : 'hidden'}>${recipientGroup('cc', t(ctx, 'cc'), inst.recipients.cc, ctx)}</div>
                <div data-copy-row="bcc" ${draft.bcc?.length ? '' : 'hidden'}>${recipientGroup('bcc', t(ctx, 'bcc'), inst.recipients.bcc, ctx)}</div>
                <label class="vd-mailer-compose-row"><span>${esc(t(ctx, 'subject'))}</span><input name="subject" data-subject maxlength="998" value="${esc(draft.subject)}" ${inst.readonly ? 'readonly' : ''} autocomplete="off"></label>
                <div class="vd-mailer-compose-toolbar"><div class="vd-mailer-compose-toolbar-left"><button type="button" class="vd-mailer-button" data-attach ${inst.readonly ? 'disabled' : ''}>${esc(t(ctx, 'attach_file'))}</button><input type="file" data-file-input multiple hidden>
                    <button type="button" class="vd-mailer-button" data-stickers-open aria-expanded="false" ${inst.readonly || inst.preserveHTML ? 'disabled' : ''}>${esc(ctx.t('stickers.title'))}</button>
                    <label class="vd-mailer-schedule"><span>${esc(t(ctx, 'send_at'))}</span><input type="datetime-local" data-send-at ${inst.readonly ? 'disabled' : ''}></label></div>
                    <button type="button" class="vd-mailer-button vd-mailer-assist-open" data-assist-open ${inst.readonly || inst.preserveHTML ? 'disabled' : ''}>${esc(t(ctx, 'assist'))}</button></div>
                <div class="vd-mailer-stickers" data-mailer-stickers hidden></div>
                <div class="vd-mailer-format-toolbar" data-format-toolbar role="toolbar" aria-label="${esc(t(ctx, 'message_body'))}" ${inst.preserveHTML ? 'hidden' : ''}>${[
                    ['bold', 'B', 'format_bold'], ['italic', 'I', 'format_italic'], ['bulleted_list', '•', 'format_bulleted_list'], ['numbered_list', '1.', 'format_numbered_list'], ['quote', '❞', 'format_quote'], ['heading', 'H2', 'format_heading']
                ].map(([format, label, key]) => `<button type="button" class="vd-mailer-format-button" data-format="${format}" aria-label="${esc(t(ctx, key))}" title="${esc(t(ctx, key))}" ${inst.readonly ? 'disabled' : ''}>${esc(label)}</button>`).join('')}</div>
                <div class="vd-mailer-compose-body" data-compose-body><div class="vd-mailer-preserved-html" data-preserved-html ${inst.preserveHTML ? '' : 'hidden'}><p>${esc(t(ctx, 'foreign_html_preserved'))}</p><iframe sandbox="" referrerpolicy="no-referrer" data-preserved-html-frame title="${esc(t(ctx, 'foreign_html_preview'))}"></iframe><button type="button" class="vd-mailer-button" data-convert-html ${inst.readonly ? 'disabled' : ''}>${esc(t(ctx, 'convert_html'))}</button></div><div data-rich-editor ${inst.preserveHTML ? 'hidden' : ''}></div><textarea data-body-fallback aria-label="${esc(t(ctx, 'message_body'))}" hidden ${inst.readonly ? 'readonly' : ''}></textarea></div>
                <p class="vd-mailer-compose-hint" data-editor-error hidden role="status"></p>
                <div class="vd-mailer-attachments" data-attachments aria-live="polite"></div>
                <div class="vd-mailer-forward-copy" data-forward-copy hidden role="status" aria-live="polite"><p data-forward-copy-status></p><div data-forward-copy-actions></div></div>
                <div class="vd-mailer-forward-copy vd-mailer-draft-conflict" data-draft-conflict hidden role="alert" aria-live="assertive"><p data-draft-conflict-message></p><div data-draft-conflict-attachments></div><div data-draft-conflict-actions><button type="button" class="vd-mailer-button vd-mailer-primary" data-resolve-draft-conflict>${esc(t(ctx, 'draft_conflict_keep'))}</button><button type="button" class="vd-mailer-button" data-reload-latest-draft>${esc(t(ctx, 'draft_conflict_reload'))}</button></div></div>
                <section class="vd-mailer-assist-panel" data-assist-panel hidden aria-label="${esc(t(ctx, 'assist'))}">
                    <div class="vd-mailer-assist-grid"><label><span>${esc(t(ctx, 'assist_action'))}</span><select data-assist-action ${inst.readonly ? 'disabled' : ''}><option value="draft">${esc(t(ctx, 'assist_draft'))}</option><option value="reply">${esc(t(ctx, 'assist_reply'))}</option><option value="rewrite">${esc(t(ctx, 'assist_rewrite'))}</option><option value="shorten">${esc(t(ctx, 'assist_shorten'))}</option><option value="tone">${esc(t(ctx, 'assist_tone'))}</option><option value="translate">${esc(t(ctx, 'assist_translate'))}</option><option value="summarize">${esc(t(ctx, 'assist_summarize'))}</option></select></label>
            <label><span>${esc(t(ctx, 'provider'))}</span><select data-assist-provider><option value="">${esc(defaultProviderLabel(ctx))}</option>${providerRows(ctx).map(provider => `<option value="${esc(provider.id)}">${esc(provider.name || provider.id)} · ${esc(provider.model || provider.type || '')}</option>`).join('')}</select></label>
                    <label data-assist-instruction-wrap><span>${esc(t(ctx, 'assist_instruction'))}</span><input data-assist-instruction maxlength="240"></label>
                    <label data-assist-language-wrap hidden><span>${esc(t(ctx, 'language'))}</span><input data-assist-language maxlength="80" autocomplete="language"></label>
                    <label class="vd-mailer-assist-context"><span>${esc(t(ctx, 'assist_context'))}</span><textarea data-assist-context rows="2" maxlength="6000"></textarea></label></div>
                    <details class="vd-mailer-assist-transfer" open><summary>${esc(t(ctx, 'assist_transfer_review'))}</summary><p>${esc(t(ctx, 'assist_transfer_note'))}</p><b>${esc(t(ctx, 'assist_body_sent'))}</b><pre data-assist-body-review></pre><b>${esc(t(ctx, 'assist_context_sent'))}</b><pre data-assist-context-review></pre><p data-assist-provider-review></p></details>
                    <div class="vd-mailer-assist-actions"><span data-assist-status role="status" aria-live="polite"></span><button type="button" class="vd-mailer-button" data-assist-cancel>${esc(t(ctx, 'cancel'))}</button><button type="button" class="vd-mailer-button vd-mailer-primary" data-assist-run>${esc(t(ctx, 'assist_preview'))}</button></div>
                    <div class="vd-mailer-assist-preview" data-assist-preview hidden><div data-assist-preview-content></div><button type="button" class="vd-mailer-button" data-assist-discard>${esc(t(ctx, 'discard'))}</button><button type="button" class="vd-mailer-button vd-mailer-primary" data-assist-apply>${esc(t(ctx, 'apply_preview'))}</button></div>
                </section>
                <footer class="vd-mailer-compose-foot"><div><button type="button" class="vd-mailer-button" data-save-now ${inst.readonly ? 'disabled' : ''}>${esc(t(ctx, 'save_draft'))}</button><button type="button" class="vd-mailer-button vd-mailer-danger" data-discard-draft ${inst.readonly || !draft.id ? 'disabled' : ''}>${esc(t(ctx, 'discard_draft'))}</button></div>
                    <button type="submit" class="vd-mailer-button vd-mailer-primary vd-mailer-send" data-send ${inst.readonly ? 'disabled' : ''}>${esc(t(ctx, 'send'))}</button></footer>
                <div class="vd-mailer-send-undo" data-send-undo hidden role="status" aria-live="polite"><span data-send-undo-copy></span><button type="button" class="vd-mailer-button" data-undo-send>${esc(t(ctx, 'undo_send'))}</button></div>
                <p class="vd-mailer-compose-status" data-compose-status role="status" aria-live="polite"></p>
            </form>
        </section>`;
        inst.stickers = window.MailerStickers?.mount(inst, { editingLocked, queueUpload, changed });
        bind(inst);
        host.querySelector('[data-assist-context]').value = String(options.assistContext || '');
        syncAssistReview(inst);
        renderRecipients(inst);
        renderAttachments(inst);
        paintForwardCopy(inst);
        renderDraftConflict(inst);
        loadContacts(inst);
        loadComposerRecords(inst);
        if (inst.preserveHTML) {
            host.querySelector('[data-preserved-html-frame]').srcdoc = previewHTML(draft.html);
            inst.editorPromise = Promise.resolve();
        } else {
            inst.editorPromise = mountEditor(inst, String(draft.text || '')).then(() => {
                if (!draft.id && (inst.recipients.to.length || draft.subject || draft.text || draft.html)) changed(inst);
            });
        }
        if (inst.forwardSources.length) {
            inst.forwardCopyPromise = copyForwardAttachments(inst);
        }
        ctx.setWindowBeforeClose?.(windowId, () => beforeClose(inst));
        ctx.registerWindowCleanup?.(windowId, () => dispose(windowId));
        return inst;
    }

    function listen(inst, target, type, handler, options) {
        target.addEventListener(type, handler, options);
        inst.listeners.push([target, type, handler, options]);
    }

    async function mountEditor(inst, text) {
        const root = inst.host.querySelector('[data-rich-editor]');
        const fallback = inst.host.querySelector('[data-body-fallback]');
        try {
            const lib = await editorLib();
            if (inst.disposed || !root.isConnected) return;
            const crepe = new lib.CrepeBuilder({ root, defaultValue: text });
            await crepe.create();
            if (inst.disposed) { crepe.destroy(); return; }
            inst.editorLib = lib;
            crepe.setReadonly(editingLocked(inst));
            inst.editor = crepe;
            const view = crepe.editor.action(ctx => ctx.get(lib.editorViewCtx));
            inst.editorView = view;
            view.dom.setAttribute('aria-label', t(inst.ctx, 'message_body'));
            view.dom.setAttribute('spellcheck', 'true');
            view.dom.setAttribute('data-mailer-richtext', '');
            const previousDispatch = view.props.dispatchTransaction;
            view.setProps({ ...(inst.stickers ? { nodeViews: { ...view.props.nodeViews, image: node => inst.stickers.imageView(node) } } : {}), dispatchTransaction(transaction) {
                inst.stickers?.map(transaction.mapping);
                if (previousDispatch) previousDispatch.call(view, transaction);
                else view.updateState(view.state.apply(transaction));
                if (transaction.docChanged && !editingLocked(inst)) { inst.bodyChanged = true; changed(inst); }
            }});
            inst.markdown = () => crepe.getMarkdown();
            inst.setMarkdown = value => {
                const parsed = crepe.editor.action(ctx => ctx.get(lib.parserCtx)(String(value || '')));
                view.dispatch(lib.closeHistory(view.state.tr).replaceWith(0, view.state.doc.content.size, parsed.content));
            };
            root.closest('.vd-mailer-compose-body').classList.remove('is-fallback');
            fallback.hidden = true;
            root.toggleAttribute('aria-readonly', editingLocked(inst));
        } catch (_) {
            if (inst.disposed) return;
            root.hidden = true;
            fallback.hidden = false;
            inst.host.querySelector('[data-format-toolbar]').hidden = true;
            fallback.value = text;
            fallback.addEventListener('input', () => { inst.bodyChanged = true; changed(inst); });
            inst.markdown = () => fallback.value;
            inst.setMarkdown = value => { fallback.value = value; };
            const note = inst.host.querySelector('[data-editor-error]');
            note.textContent = t(inst.ctx, 'editor_unavailable_source_mode');
            note.hidden = false;
        }
    }

    async function loadContacts(inst) {
        const response = inst.ctx.state?.contacts || [];
        inst.contacts = Array.isArray(response) ? response : response.items || response.contacts || [];
    }

    async function loadComposerRecords(inst) {
        try {
            const [templateData, signatureData] = await Promise.all([inst.api.templates(), inst.api.signatures()]);
            if (inst.disposed) return;
            inst.templates = Array.isArray(templateData) ? templateData : templateData.items || templateData.templates || [];
            inst.signatures = Array.isArray(signatureData) ? signatureData : signatureData.items || signatureData.signatures || [];
            paintComposerRecords(inst);
        } catch (_) {
            if (!inst.disposed) setStatus(inst, t(inst.ctx, 'snippets_unavailable'), true);
        }
    }

    function paintComposerRecords(inst) {
        const templates = inst.host.querySelector('[data-insert-template]');
        const signatures = inst.host.querySelector('[data-insert-signature]');
        if (!templates || !signatures) return;
        const relevant = item => !item.account_id || item.account_id === inst.accountId;
        templates.innerHTML = `<option value="">${esc(t(inst.ctx, 'choose_template'))}</option>` + (inst.templates || []).filter(relevant).map(item => `<option value="${esc(item.id)}">${esc(item.name)}</option>`).join('');
        signatures.innerHTML = `<option value="">${esc(t(inst.ctx, 'choose_signature'))}</option>` + (inst.signatures || []).filter(relevant).map(item => `<option value="${esc(item.id)}">${esc(item.name)}</option>`).join('');
    }

    async function insertTemplate(inst, id) {
        if (editingLocked(inst)) return;
        const item = (inst.templates || []).find(record => String(record.id) === String(id));
        if (!item) return;
        await inst.editorPromise;
        if (editingLocked(inst)) return;
        if (item.subject) inst.host.querySelector('[data-subject]').value = item.subject;
        inst.bodyChanged = true;
        inst.setMarkdown?.(item.text || '');
        changed(inst);
    }

    async function insertSignature(inst, id) {
        if (editingLocked(inst)) return;
        const item = (inst.signatures || []).find(record => String(record.id) === String(id));
        if (!item) return;
        await inst.editorPromise;
        if (editingLocked(inst)) return;
        const current = inst.markdown?.() || '';
        inst.bodyChanged = true;
        inst.setMarkdown?.([current.trimEnd(), '--', item.text || ''].filter(Boolean).join('\n\n'));
        changed(inst);
    }

    async function convertForeignHTML(inst) {
        if (!inst.preserveHTML || editingLocked(inst)) return;
        const accepted = await inst.ctx.confirmDialog(t(inst.ctx, 'convert_html_title'), t(inst.ctx, 'convert_html_confirm'));
        if (!accepted || inst.disposed || editingLocked(inst)) return;
        const container = document.createElement('div');
        container.innerHTML = normalizeHTML(inst.draft.html);
        const text = String(container.innerText || container.textContent || '').trim();
        inst.preserveHTML = false;
        inst.bodyChanged = true;
        inst.draft.text = text;
        inst.draft.html = '';
        inst.host.querySelector('[data-preserved-html]').hidden = true;
        inst.host.querySelector('[data-rich-editor]').hidden = false;
        inst.host.querySelector('[data-format-toolbar]').hidden = false;
        inst.host.querySelectorAll('[data-insert-template], [data-insert-signature], [data-assist-open]').forEach(control => { control.disabled = false; });
        inst.editorPromise = mountEditor(inst, text);
        await inst.editorPromise;
        inst.stickers?.sync();
        changed(inst);
    }

    function bind(inst) {
        const host = inst.host;
        const form = host.querySelector('[data-compose-form]');
        listen(inst, form, 'input', event => {
            if (event.target.matches('[data-subject]')) changed(inst);
            if (event.target.matches('[data-recipient-input]')) showSuggestions(inst, event.target.dataset.recipientInput);
            if (event.target.matches('[data-assist-action]')) syncAssistFields(inst);
            if (event.target.matches('[data-assist-context]')) syncAssistReview(inst);
        });
        listen(inst, form, 'change', event => {
            if (event.target.matches('[data-account]')) {
                const nextAccountId = event.target.value;
                if (nextAccountId !== inst.accountId) inst.draft.reply_remote_id = '';
                inst.accountId = nextAccountId;
                paintComposerRecords(inst);
                changed(inst);
            }
            if (event.target.matches('[data-assist-action]')) syncAssistFields(inst);
            if (event.target.matches('[data-assist-provider]')) syncAssistReview(inst);
            if (event.target.matches('[data-insert-template]')) insertTemplate(inst, event.target.value);
            if (event.target.matches('[data-insert-signature]')) insertSignature(inst, event.target.value);
        });
        listen(inst, form, 'keydown', event => {
            const input = event.target.closest('[data-recipient-input]');
            if (!input) return;
            const field = input.dataset.recipientInput;
            if (event.key === 'Enter' || event.key === ',' || event.key === ';') {
                event.preventDefault(); addRecipient(inst, field, input.value);
            } else if (event.key === 'Backspace' && !input.value && inst.recipients[field].length) {
                inst.recipients[field].pop(); renderRecipients(inst); changed(inst);
            } else if (event.key === 'ArrowDown') {
                const first = host.querySelector(`[data-suggestions="${field}"] button`);
                if (first) { event.preventDefault(); first.focus(); }
            }
        });
        listen(inst, form, 'submit', event => { event.preventDefault(); send(inst); });
        listen(inst, form, 'click', event => {
            const retryUpload = event.target.closest('[data-retry-upload]');
            if (retryUpload) { retryFailedUpload(inst, retryUpload.dataset.retryUpload); return; }
            const removeUpload = event.target.closest('[data-remove-upload]');
            if (removeUpload) { removeFailedUpload(inst, removeUpload.dataset.removeUpload); return; }
            const skipForward = event.target.closest('[data-skip-forward-attachment]');
            if (skipForward) { skipForwardAttachment(inst, skipForward.dataset.skipForwardAttachment); return; }
            const removeConflictAttachment = event.target.closest('[data-remove-conflict-attachment]');
            if (removeConflictAttachment) { removeConflictAttachmentItem(inst, removeConflictAttachment.dataset.removeConflictAttachment); return; }
            if (event.target.closest('[data-resolve-draft-conflict]')) { resolveDraftConflict(inst); return; }
            if (event.target.closest('[data-reload-latest-draft]')) { reloadLatestDraft(inst); return; }
            const retryForward = event.target.closest('[data-retry-forward-attachment]');
            if (retryForward) { copyForwardAttachment(inst, retryForward.dataset.retryForwardAttachment); return; }
            const remove = event.target.closest('[data-remove-recipient]');
            if (remove) { inst.recipients[remove.dataset.field].splice(Number(remove.dataset.removeRecipient), 1); renderRecipients(inst); changed(inst); return; }
            const addCopy = event.target.closest('[data-add-copy]');
            if (addCopy) { const row = host.querySelector(`[data-copy-row="${addCopy.dataset.addCopy}"]`); row.hidden = false; addCopy.hidden = true; row.querySelector('input').focus(); return; }
            if (event.target.closest('[data-attach]')) { host.querySelector('[data-file-input]').click(); return; }
            const format = event.target.closest('[data-format]');
            if (format) { applyFormat(inst, format.dataset.format); return; }
            if (event.target.closest('[data-save-now]')) { save(inst).catch(error => setStatus(inst, errorText(inst, error), true)); return; }
            if (event.target.closest('[data-discard-draft]')) { discard(inst); return; }
            if (event.target.closest('[data-assist-open]')) { const panel = host.querySelector('[data-assist-panel]'); panel.hidden = !panel.hidden; if (!panel.hidden) { syncAssistReview(inst); panel.querySelector('[data-assist-action]').focus(); } return; }
            if (event.target.closest('[data-convert-html]')) { convertForeignHTML(inst); return; }
            if (event.target.closest('[data-assist-run]')) { runAssist(inst); return; }
            if (event.target.closest('[data-assist-cancel]')) { cancelAssist(inst); return; }
            if (event.target.closest('[data-assist-discard]')) { discardAssist(inst); return; }
            if (event.target.closest('[data-assist-apply]')) { applyAssist(inst); return; }
            if (event.target.closest('[data-undo-send]')) { undoSend(inst); return; }
            const suggestion = event.target.closest('[data-contact-email]');
            if (suggestion) { addRecipient(inst, suggestion.dataset.field, suggestion.dataset.contactEmail); }
            const removeAttachment = event.target.closest('[data-remove-attachment]');
            if (removeAttachment) removeAttachmentItem(inst, removeAttachment.dataset.removeAttachment);
        });
        const files = host.querySelector('[data-file-input]');
        listen(inst, files, 'change', () => trackUpload(inst, Array.from(files.files || [])));
        const drop = host.querySelector('[data-compose-body]');
        listen(inst, drop, 'dragover', event => { if (!editingLocked(inst)) { event.preventDefault(); drop.classList.add('is-drop-target'); } });
        listen(inst, drop, 'dragleave', () => drop.classList.remove('is-drop-target'));
        listen(inst, drop, 'drop', event => {
            drop.classList.remove('is-drop-target');
            if (!editingLocked(inst) && event.dataTransfer?.files?.length) { event.preventDefault(); trackUpload(inst, Array.from(event.dataTransfer.files)); }
        });
    }

    function renderRecipients(inst) {
        Object.keys(inst.recipients).forEach(field => {
            const chips = inst.host.querySelector(`[data-chips="${field}"]`);
            if (!chips) return;
            chips.innerHTML = inst.recipients[field].map((address, index) => `<span class="vd-mailer-recipient-chip" role="listitem"><span>${esc(address)}</span><button type="button" data-field="${field}" data-remove-recipient="${index}" aria-label="${esc(t(inst.ctx, 'remove_recipient', { email: address }))}" ${editingLocked(inst) ? 'disabled' : ''}>×</button></span>`).join('');
        });
    }

    function addRecipient(inst, field, value) {
        if (editingLocked(inst)) return;
        const address = String(value || '').trim().replace(/[;,]+$/, '');
        const input = inst.host.querySelector(`[data-recipient-input="${field}"]`);
        if (!address) return;
        if (!validAddress(address)) { input.setCustomValidity(t(inst.ctx, 'invalid_address')); input.reportValidity(); input.setCustomValidity(''); return; }
        if (!inst.recipients[field].some(existing => existing.toLowerCase() === address.toLowerCase())) inst.recipients[field].push(address);
        input.value = '';
        renderRecipients(inst);
        hideSuggestions(inst, field);
        input.focus();
        changed(inst);
    }

    function editingLocked(inst) {
        return !!inst.readonly || !!inst.sending || !!inst.committed || !!inst.conflictResolving;
    }

    function showSuggestions(inst, field) {
        const input = inst.host.querySelector(`[data-recipient-input="${field}"]`);
        const menu = inst.host.querySelector(`[data-suggestions="${field}"]`);
        const query = input.value.trim().toLowerCase();
        if (!query || query.includes('@')) { menu.hidden = true; return; }
        const matches = inst.contacts.filter(person => person.email && (String(person.name || '').toLowerCase().includes(query) || String(person.email).toLowerCase().includes(query))).slice(0, 6);
        menu.innerHTML = matches.map(person => `<button type="button" role="option" data-field="${field}" data-contact-email="${esc(person.email)}"><b>${esc(person.name || person.email)}</b><small>${esc(person.email)}</small></button>`).join('');
        menu.hidden = matches.length === 0;
    }

    function hideSuggestions(inst, field) {
        const menu = inst.host.querySelector(`[data-suggestions="${field}"]`);
        if (menu) menu.hidden = true;
    }

    function snapshot(inst) {
        const body = inst.markdown ? String(inst.markdown() || '') : String(inst.draft.text || '');
        const select = inst.host.querySelector('[data-account]');
        return Object.assign({}, inst.draft, {
            id: inst.draft.id || undefined,
            revision: Number(inst.draft.revision || 0),
            account_id: select ? select.value : inst.accountId,
            reply_remote_id: String(inst.draft.reply_remote_id || ''),
            to: inst.recipients.to.slice(), cc: inst.recipients.cc.slice(), bcc: inst.recipients.bcc.slice(),
            subject: inst.host.querySelector('[data-subject]').value,
            text: body,
            html: inst.preserveHTML && !inst.bodyChanged ? inst.draft.html : cleanHtml(body, inst.draft.attachments)
        });
    }

    function changed(inst) {
        if (inst.readonly || inst.disposed || inst.committed) return;
        inst.editGeneration++;
        const state = inst.host.querySelector('[data-save-state]');
        if (state) { state.textContent = t(inst.ctx, 'draft_saving'); state.classList.remove('is-error'); }
        clearTimeout(inst.saveTimer);
        if (inst.attachmentBusy || inst.forwardCopying || inst.draftConflict || inst.sending) return;
        inst.saveTimer = setTimeout(() => save(inst).catch(error => setStatus(inst, errorText(inst, error), true)), 650);
    }

    async function save(inst) {
        if (inst.readonly || inst.committed || inst.disposed) return inst.draft;
        if (inst.draftConflict) throw Object.assign(new Error('draft_conflict'), { status: 409 });
        clearTimeout(inst.saveTimer);
        if (inst.savedGeneration === inst.editGeneration && inst.draft.id) return inst.draft;
        inst.saveQueue = inst.saveQueue.catch(() => {}).then(async () => {
            if (inst.disposed) throw new Error('closed');
            if (inst.savedGeneration >= inst.editGeneration && inst.draft.id) return inst.draft;
            const currentGeneration = inst.editGeneration;
            const currentPayload = snapshot(inst);
            currentPayload.revision = Number(inst.draft.revision || 0);
            let result;
            try { result = await inst.api.saveDraft(currentPayload); }
            catch (error) {
                if (error && error.status === 409) setDraftConflict(inst, error, currentPayload);
                throw error;
            }
            if (inst.disposed) return result;
            inst.draft = Object.assign({}, currentPayload, result);
            inst.accountId = inst.draft.account_id || inst.accountId;
            inst.savedGeneration = currentGeneration;
            const status = inst.host.querySelector('[data-save-state]');
            if (status) { status.textContent = t(inst.ctx, 'draft_saved'); status.classList.remove('is-error'); }
            const discard = inst.host.querySelector('[data-discard-draft]');
            if (discard) discard.disabled = !inst.draft.id;
            renderDraftConflict(inst);
            return inst.draft;
        });
        const result = await inst.saveQueue;
        if (inst.savedGeneration < inst.editGeneration) return save(inst);
        return result;
    }

    function setDraftConflict(inst, error, payload) {
        inst.draftConflict = true;
        if (!inst.conflictLocalPayload) inst.conflictLocalPayload = payload || snapshot(inst);
        if (!inst.conflictLocalAttachments.length) inst.conflictLocalAttachments = (inst.draft.attachments || []).map(item => Object.assign({}, item));
        renderDraftConflict(inst);
        syncSendButton(inst);
        if (error) setStatus(inst, errorText(inst, error), true);
    }

    function renderDraftConflict(inst) {
        const panel = inst.host.querySelector('[data-draft-conflict]');
        if (!panel) return;
        panel.hidden = !inst.draftConflict && !inst.conflictMissingAttachments.length;
        const message = panel.querySelector('[data-draft-conflict-message]');
        if (message) {
            message.textContent = inst.conflictMissingAttachments.length
                ? t(inst.ctx, 'draft_conflict_attachments_changed')
                : t(inst.ctx, 'draft_conflict_message');
        }
        const attachments = panel.querySelector('[data-draft-conflict-attachments]');
        if (attachments) attachments.innerHTML = inst.conflictMissingAttachments.map(item => `<div class="vd-mailer-attachment is-upload-failed"><span aria-hidden="true">⚠</span><span><b>${esc(item.filename || item.name || t(inst.ctx, 'attachment'))}</b><small>${esc(t(inst.ctx, 'draft_conflict_attachment_missing'))}</small></span><button type="button" data-remove-conflict-attachment="${esc(item.id)}">${esc(t(inst.ctx, 'remove_attachment'))}</button></div>`).join('');
        const keep = panel.querySelector('[data-resolve-draft-conflict]');
        const reload = panel.querySelector('[data-reload-latest-draft]');
        if (keep) keep.hidden = !inst.draftConflict || inst.conflictResolving;
        if (reload) reload.hidden = !inst.draftConflict || inst.conflictResolving;
        if (keep) keep.disabled = !!inst.conflictResolving;
        if (reload) reload.disabled = !!inst.conflictResolving;
        syncSendButton(inst);
    }

    async function resolveDraftConflict(inst) {
        if (!inst.draftConflict || inst.conflictResolving || !inst.draft.id) return;
        const local = snapshot(inst);
        inst.conflictResolving = true;
        renderDraftConflict(inst);
        try {
            const response = await inst.api.draft(inst.draft.id);
            const latest = response && response.draft ? response.draft : response;
            if (!latest || !latest.id) throw new Error('draft_missing');
            const latestIDs = new Set((latest.attachments || []).map(item => String(item.id)));
            const missingById = new Map(inst.conflictMissingAttachments.map(item => [String(item.id), item]));
            for (const item of inst.conflictLocalAttachments) {
                if (!latestIDs.has(String(item.id))) missingById.set(String(item.id), item);
            }
            inst.conflictMissingAttachments = Array.from(missingById.values());
            inst.draft = Object.assign({}, inst.draft, latest);
            const accountSelect = inst.host.querySelector('[data-account]');
            const localAccountIsAvailable = Array.from(accountSelect?.options || []).some(option => option.value === local.account_id);
            const accountId = localAccountIsAvailable ? local.account_id : (latest.account_id || inst.accountId);
            if (accountSelect && accountId) accountSelect.value = accountId;
            inst.accountId = accountId;
            // Use the latest server revision while retaining this window's editable fields.
            // Imported HTML has no live editor value, so preserve its local payload explicitly.
            for (const key of ['reply_remote_id', 'in_reply_to', 'references', 'to', 'cc', 'bcc', 'subject', 'text', 'html']) {
                if (Object.prototype.hasOwnProperty.call(local, key)) inst.draft[key] = local[key];
            }
            inst.draft.account_id = accountId;
            inst.recipients = { to: emailList(local.to), cc: emailList(local.cc), bcc: emailList(local.bcc) };
            inst.host.querySelector('[data-subject]').value = String(local.subject || '');
            inst.preserveHTML = !!local.html && normalizeHTML(local.html) !== normalizeHTML(cleanHtml(local.text || '', inst.draft.attachments));
            inst.bodyChanged = false;
            if (inst.preserveHTML) {
                inst.host.querySelector('[data-preserved-html-frame]').srcdoc = previewHTML(local.html);
                inst.host.querySelector('[data-preserved-html]').hidden = false;
                inst.host.querySelector('[data-rich-editor]').hidden = true;
                inst.host.querySelector('[data-format-toolbar]').hidden = true;
            }
            inst.conflictLocalPayload = snapshot(inst);
            inst.conflictLocalAttachments = (inst.draft.attachments || []).map(item => Object.assign({}, item));
            inst.draftConflict = false;
            inst.conflictResolving = false;
            renderRecipients(inst);
            renderAttachments(inst);
            renderDraftConflict(inst);
            changed(inst);
            await save(inst);
            inst.conflictLocalPayload = null;
            inst.conflictLocalAttachments = [];
            setStatus(inst, inst.conflictMissingAttachments.length ? t(inst.ctx, 'draft_conflict_attachments_changed') : t(inst.ctx, 'draft_conflict_rebased'));
        } catch (error) {
            inst.conflictResolving = false;
            if (error?.status === 409) setDraftConflict(inst, error, snapshot(inst));
            else renderDraftConflict(inst);
            setStatus(inst, errorText(inst, error), true);
        }
    }

    async function reloadLatestDraft(inst) {
        if (!inst.draftConflict || inst.conflictResolving || !inst.draft.id) return;
        const confirmed = await inst.ctx.confirmDialog(t(inst.ctx, 'draft_conflict_reload'), t(inst.ctx, 'draft_conflict_reload_confirm'));
        if (!confirmed || inst.disposed) return;
        inst.conflictResolving = true;
        renderDraftConflict(inst);
        try {
            const response = await inst.api.draft(inst.draft.id);
            const latest = response && response.draft ? response.draft : response;
            if (!latest || !latest.id) throw new Error('draft_missing');
            inst.draft = Object.assign({}, inst.draft, latest);
            inst.recipients = { to: emailList(latest.to), cc: emailList(latest.cc), bcc: emailList(latest.bcc) };
            inst.accountId = latest.account_id || inst.accountId;
            const accountSelect = inst.host.querySelector('[data-account]');
            if (accountSelect && inst.accountId) accountSelect.value = inst.accountId;
            inst.host.querySelector('[data-subject]').value = String(latest.subject || '');
            inst.preserveHTML = !!latest.html && normalizeHTML(latest.html) !== normalizeHTML(cleanHtml(latest.text || '', latest.attachments));
            inst.bodyChanged = false;
            inst.editorPromise = Promise.resolve(inst.editorPromise).then(() => {
                if (inst.preserveHTML) {
                    inst.host.querySelector('[data-preserved-html-frame]').srcdoc = previewHTML(latest.html);
                    inst.host.querySelector('[data-preserved-html]').hidden = false;
                    inst.host.querySelector('[data-rich-editor]').hidden = true;
                    inst.host.querySelector('[data-format-toolbar]').hidden = true;
                } else {
                    inst.host.querySelector('[data-preserved-html]').hidden = true;
                    inst.host.querySelector('[data-rich-editor]').hidden = false;
                    inst.host.querySelector('[data-format-toolbar]').hidden = false;
                    inst.setMarkdown?.(String(latest.text || ''));
                }
            });
            await inst.editorPromise;
            inst.draftConflict = false;
            inst.conflictMissingAttachments = [];
            inst.conflictLocalPayload = null;
            inst.conflictLocalAttachments = [];
            inst.failedUploads.clear();
            inst.savedGeneration = inst.editGeneration;
            inst.conflictResolving = false;
            renderRecipients(inst);
            renderAttachments(inst);
            renderDraftConflict(inst);
            setStatus(inst, t(inst.ctx, 'draft_conflict_reloaded'));
        } catch (error) {
            inst.conflictResolving = false;
            renderDraftConflict(inst);
            setStatus(inst, errorText(inst, error), true);
        }
    }

    function removeConflictAttachmentItem(inst, attachmentId) {
        inst.conflictMissingAttachments = inst.conflictMissingAttachments.filter(item => String(item.id) !== String(attachmentId));
        renderDraftConflict(inst);
        if (!inst.conflictMissingAttachments.length) setStatus(inst, t(inst.ctx, 'draft_conflict_rebased'));
    }

    async function beforeClose(inst) {
        clearTimeout(inst.saveTimer);
        await inst.uploadTask.catch(() => {});
        await inst.forwardCopyPromise;
        await inst.attachmentTask;
        if (inst.failedUploads.size) {
            const keep = await inst.ctx.confirmDialog(t(inst.ctx, 'upload_failed_title'), t(inst.ctx, 'upload_failed_close_confirm'));
            if (!keep) return false;
        }
        if (inst.readonly || inst.committed || inst.savedGeneration === inst.editGeneration) return true;
        try { await save(inst); return true; }
        catch (_) {
            const keep = await inst.ctx.confirmDialog(t(inst.ctx, 'save_failed_title'), t(inst.ctx, 'close_unsaved_confirm'));
            return !!keep;
        }
    }

    function trackUpload(inst, files) {
        if (!files.length || editingLocked(inst)) return;
        return queueUpload(inst, async () => files);
    }

    function queueUpload(inst, loadFiles) {
        if (editingLocked(inst)) return;
        inst.uploadQueueCount++;
        inst.attachmentBusy = true;
        syncSendButton(inst);
        const task = inst.uploadTask.then(async () => {
            const files = await loadFiles();
            if (!inst.disposed) await uploadFiles(inst, files);
        }).finally(() => {
            inst.uploadQueueCount = Math.max(0, inst.uploadQueueCount - 1);
            inst.attachmentBusy = inst.uploadQueueCount > 0 || !!inst.attachmentMutation;
            renderAttachments(inst);
        });
        inst.uploadTask = task;
        task.catch(() => { if (inst.uploadTask === task) inst.uploadTask = Promise.resolve(); });
        return task;
    }

    async function uploadFiles(inst, files) {
        if (!files.length || inst.readonly) return;
        try {
            await save(inst);
            if (!inst.draft.id) throw new Error('draft_missing');
            for (const file of files) {
                const key = 'upload-' + Math.random().toString(36).slice(2);
                inst.uploads.set(key, { name: file.name, progress: 0 });
                renderAttachments(inst);
                try {
                    const previousIDs = new Set((inst.draft.attachments || []).map(item => item.id));
                    const result = await inst.api.upload(inst.draft.id, inst.draft.revision, file, value => {
                        const item = inst.uploads.get(key);
                        if (item) { item.progress = value; renderAttachments(inst); }
                    }, inst.uploadController.signal);
                    if (inst.disposed) return;
                    if (result && result.revision != null && Array.isArray(result.attachments)) {
                        inst.draft = Object.assign({}, inst.draft, result);
                    } else if (result && result.draft) {
                        inst.draft = Object.assign({}, inst.draft, result.draft);
                    } else {
                        inst.draft.attachments = (inst.draft.attachments || []).concat([result.attachment || result]);
                    }
                    inst.stickers?.uploaded(file, previousIDs);
                    inst.failedUploads.delete(key);
                } catch (error) {
                    if (inst.disposed) return;
                    inst.failedUploads.set(key, { file, name: file.name, error });
                    if (error?.status === 409) setDraftConflict(inst, error, snapshot(inst));
                    else setStatus(inst, t(inst.ctx, 'upload_failed_block_send'), true);
                } finally { inst.uploads.delete(key); renderAttachments(inst); }
            }
            if (!inst.draftConflict && inst.failedUploads.size === 0) {
                changed(inst);
                await save(inst);
            }
        } catch (error) {
            if (inst.disposed) return;
            for (const file of files) {
                if (!Array.from(inst.failedUploads.values()).some(item => item.file === file)) {
                    const key = 'upload-' + Math.random().toString(36).slice(2);
                    inst.failedUploads.set(key, { file, name: file.name, error });
                }
            }
            if (error?.status === 409) setDraftConflict(inst, error, snapshot(inst));
            setStatus(inst, errorText(inst, error), true);
        } finally { if (!inst.disposed) { inst.host.querySelector('[data-file-input]').value = ''; renderAttachments(inst); syncSendButton(inst); } }
    }

    function retryFailedUpload(inst, key) {
        const pending = inst.failedUploads.get(String(key));
        if (!pending || editingLocked(inst)) return;
        inst.failedUploads.delete(String(key));
        renderAttachments(inst);
        trackUpload(inst, [pending.file]);
    }

    function removeFailedUpload(inst, key) {
        if (editingLocked(inst)) return;
        inst.stickers?.forget(inst.failedUploads.get(String(key))?.file);
        if (!inst.failedUploads.delete(String(key))) return;
        renderAttachments(inst);
        changed(inst);
        save(inst).catch(error => setStatus(inst, errorText(inst, error), true));
        syncSendButton(inst);
        setStatus(inst, t(inst.ctx, 'upload_removed'));
    }

    function renderAttachments(inst) {
        if (inst.disposed) return;
        const root = inst.host.querySelector('[data-attachments]');
        if (!root) return;
        const attachments = inst.draft.attachments || [];
        const rows = attachments.map(item => `<div class="vd-mailer-attachment" data-attachment-id="${esc(item.id)}"><span aria-hidden="true">📎</span><span><b>${esc(item.filename || item.name)}</b><small>${formatBytes(item.size)}</small></span><button type="button" data-remove-attachment="${esc(item.id)}" aria-label="${esc(t(inst.ctx, 'remove_attachment'))}" ${editingLocked(inst) || inst.attachmentBusy ? 'disabled' : ''}>×</button></div>`).join('');
        const uploads = Array.from(inst.uploads, ([key, item]) => `<div class="vd-mailer-attachment is-uploading" data-upload-id="${esc(key)}"><span>${esc(item.name)}</span><progress max="1" value="${item.progress}" aria-label="${esc(t(inst.ctx, 'upload_progress'))}"></progress><span>${Math.round(item.progress * 100)}%</span></div>`).join('');
        const failed = Array.from(inst.failedUploads, ([key, item]) => `<div class="vd-mailer-attachment is-upload-failed" data-upload-failed="${esc(key)}" role="group" aria-label="${esc(t(inst.ctx, 'upload_failed_named', { filename: item.name }))}"><span aria-hidden="true">⚠</span><span><b>${esc(item.name)}</b><small>${esc(t(inst.ctx, 'upload_failed_block_send'))}</small></span><button type="button" data-retry-upload="${esc(key)}">${esc(t(inst.ctx, 'retry'))}</button><button type="button" data-remove-upload="${esc(key)}">${esc(t(inst.ctx, 'remove_attachment'))}</button></div>`).join('');
        root.innerHTML = rows + uploads + failed;
        syncSendButton(inst);
    }

    async function applyFormat(inst, format) {
        if (editingLocked(inst) || inst.preserveHTML || !inst.editorView || !inst.editorLib) return;
        const view = inst.editorView;
        const lib = inst.editorLib;
        let command = null;
        if (format === 'bold' || format === 'italic') {
            const mark = view.state.schema.marks[format === 'bold' ? 'strong' : 'em'];
            if (mark) command = lib.toggleMark(mark);
        } else if (format === 'heading') {
            const heading = view.state.schema.nodes.heading;
            if (heading) command = lib.setBlockType(heading, { level: 2 });
        } else if (format === 'quote') {
            const quote = view.state.schema.nodes.blockquote;
            if (quote) command = lib.wrapIn(quote);
        } else {
            const list = view.state.schema.nodes[format === 'bulleted_list' ? 'bullet_list' : 'ordered_list'];
            if (list) command = lib.wrapInList(list);
        }
        if (typeof command === 'function' && command(view.state, view.dispatch)) {
            view.focus();
            changed(inst);
        }
    }

    function paintForwardCopy(inst) {
        const panel = inst.host.querySelector('[data-forward-copy]');
        if (!panel) return;
        const pending = inst.forwardSources.filter(item => !inst.forwardCopied.has(item.attachmentId) && !inst.forwardSkipped.has(item.attachmentId));
        panel.hidden = !inst.forwardSources.length;
        const status = panel.querySelector('[data-forward-copy-status]');
        if (inst.forwardCopying) status.textContent = t(inst.ctx, 'forward_attachment_copying');
        else if (pending.length) status.textContent = t(inst.ctx, 'forward_attachment_copy_review');
        else if (inst.forwardSkipped.size) status.textContent = t(inst.ctx, 'forward_attachment_skipped');
        else status.textContent = '';
        panel.querySelector('[data-forward-copy-actions]').innerHTML = !inst.forwardCopying && pending.length
            ? pending.map(item => `<span class="vd-mailer-forward-copy-item"><button type="button" class="vd-mailer-button" data-retry-forward-attachment="${esc(item.attachmentId)}">${esc(t(inst.ctx, 'retry'))}: ${esc(item.filename || t(inst.ctx, 'attachment'))}</button><button type="button" class="vd-mailer-button" data-skip-forward-attachment="${esc(item.attachmentId)}">${esc(t(inst.ctx, 'forward_attachment_skip'))}</button></span>`).join('')
            : '';
        syncSendButton(inst);
    }

    function skipForwardAttachment(inst, attachmentId) {
        const item = inst.forwardSources.find(candidate => candidate.attachmentId === String(attachmentId));
        if (!item || inst.forwardCopying || editingLocked(inst)) return;
        inst.forwardSkipped.add(item.attachmentId);
        paintForwardCopy(inst);
        setStatus(inst, t(inst.ctx, 'forward_attachment_skipped'));
    }

    function syncSendButton(inst) {
        if (inst.disposed) return;
        const send = inst.host.querySelector('[data-send]');
        if (!send) return;
        const forwardPending = inst.forwardSources.some(item => !inst.forwardCopied.has(item.attachmentId) && !inst.forwardSkipped.has(item.attachmentId));
        send.disabled = !!inst.readonly || !!inst.committed || !!inst.sending || !!inst.draftConflict || inst.conflictMissingAttachments.length > 0 || inst.failedUploads.size > 0 || (forwardPending && !inst.forwardCopying);
        const editLocked = editingLocked(inst);
        const form = inst.host.querySelector('[data-compose-form]');
        if (editLocked) {
            if (!inst.sendLockActive) {
                inst.sendLockSnapshot = new Map();
                inst.sendLockActive = true;
            }
            form.querySelectorAll('input, textarea, select, button').forEach(control => {
                // These two controls have state derived outside the edit lock.
                if (control.matches('[data-undo-send], [data-send]')) return;
                if (!inst.sendLockSnapshot.has(control)) inst.sendLockSnapshot.set(control, control.disabled);
                control.disabled = true;
            });
        } else if (inst.sendLockActive) {
            for (const [control, wasDisabled] of inst.sendLockSnapshot || []) {
                if (control.isConnected) control.disabled = wasDisabled;
            }
            inst.sendLockSnapshot = null;
            inst.sendLockActive = false;
        }
        if (!editLocked) {
            const discard = inst.host.querySelector('[data-discard-draft]');
            if (discard) discard.disabled = !inst.draft.id;
        }
        inst.editor?.setReadonly?.(editLocked);
        const editorRoot = inst.host.querySelector('[data-rich-editor]');
        if (editorRoot) editorRoot.toggleAttribute('aria-readonly', editLocked);
        inst.stickers?.sync();
    }

    async function reconcileForwardDraft(inst) {
        if (!inst.draft.id || typeof inst.api.draft !== 'function') return;
        try {
            const result = await inst.api.draft(inst.draft.id);
            const current = result && result.draft ? result.draft : result;
            if (current && current.id) {
                inst.draft = Object.assign({}, inst.draft, current);
                renderAttachments(inst);
            }
        } catch (_) { /* Keep the draft and attachment status visible for manual reconciliation. */ }
    }

    async function copyForwardAttachments(inst) {
        inst.forwardCopying = true;
        paintForwardCopy(inst);
        try {
            await inst.editorPromise;
            await save(inst);
            for (const item of inst.forwardSources) {
                if (!inst.forwardCopied.has(item.attachmentId)) await copyForwardAttachment(inst, item.attachmentId, true);
                if (!inst.forwardCopied.has(item.attachmentId)) break;
            }
        } finally {
            inst.forwardCopying = false;
            paintForwardCopy(inst);
            if (!inst.disposed && !inst.draftConflict && inst.savedGeneration < inst.editGeneration) {
                try { await save(inst); } catch (error) { setStatus(inst, errorText(inst, error), true); }
            }
        }
    }

    async function copyForwardAttachment(inst, attachmentId, fromQueue) {
        const item = inst.forwardSources.find(candidate => candidate.attachmentId === String(attachmentId));
        if (!item || editingLocked(inst) && !fromQueue || inst.forwardCopied.has(item.attachmentId) || inst.forwardCopying && !fromQueue) return;
        if (!fromQueue) inst.forwardCopying = true;
        paintForwardCopy(inst);
        const task = (async () => {
            try {
                await save(inst);
                if (!inst.draft.id) throw new Error('draft_missing');
                const result = await inst.api.copyMessageAttachment(inst.draft.id, inst.draft.revision, item.messageId, item.attachmentId);
                const updated = result && result.draft ? result.draft : result;
                if (!updated || updated.revision == null || !Array.isArray(updated.attachments)) throw new Error('draft_invalid_response');
                inst.draft = Object.assign({}, inst.draft, updated);
                inst.forwardCopied.add(item.attachmentId);
                renderAttachments(inst);
            } catch (error) {
                await reconcileForwardDraft(inst);
                setStatus(inst, errorText(inst, error), true);
            } finally {
                if (!fromQueue) inst.forwardCopying = false;
                paintForwardCopy(inst);
            }
        })();
        if (fromQueue) await task;
        else {
            inst.forwardCopyPromise = task;
            await task;
        }
    }

    function formatBytes(size) {
        const n = Number(size) || 0;
        if (n < 1024) return n + ' B';
        if (n < 1048576) return (n / 1024).toFixed(1) + ' KB';
        return (n / 1048576).toFixed(1) + ' MB';
    }

    async function removeAttachmentItem(inst, attachmentId) {
        if (editingLocked(inst) || inst.attachmentBusy || !inst.draft.id) return;
        const removed = (inst.draft.attachments || []).find(item => String(item.id) === String(attachmentId));
        inst.attachmentBusy = true;
        syncSendButton(inst);
        inst.attachmentTask = inst.attachmentTask.catch(() => {}).then(async () => {
            try {
                const updated = await inst.api.removeAttachment(inst.draft.id, attachmentId, inst.draft.revision);
                inst.draft = Object.assign({}, inst.draft, updated);
                inst.draft.attachments = (inst.draft.attachments || []).filter(item => String(item.id) !== String(attachmentId));
                inst.stickers?.remove(removed);
                renderAttachments(inst);
                changed(inst);
                await save(inst);
            } catch (error) {
                if (error?.status === 409) setDraftConflict(inst, error, snapshot(inst));
                setStatus(inst, errorText(inst, error), true);
            } finally {
                inst.attachmentBusy = false;
                renderAttachments(inst);
                syncSendButton(inst);
            }
        });
        return inst.attachmentTask;
    }

    async function discard(inst) {
        if (editingLocked(inst) || !inst.draft.id || !await inst.ctx.confirmDialog(t(inst.ctx, 'discard_draft'), t(inst.ctx, 'discard_draft_confirm')) || editingLocked(inst)) return;
        try {
            await inst.api.deleteDraft(inst.draft);
            inst.committed = true;
            setStatus(inst, t(inst.ctx, 'draft_discarded'));
            inst.ctx.closeWindow?.(inst.windowId);
        } catch (error) { setStatus(inst, errorText(inst, error), true); }
    }

    async function send(inst) {
        if (inst.readonly || inst.committed || inst.sending) return;
        const recipients = [...inst.recipients.to, ...inst.recipients.cc, ...inst.recipients.bcc];
        if (!recipients.length) { inst.host.querySelector('[data-recipient-input="to"]').focus(); setStatus(inst, t(inst.ctx, 'recipient_required'), true); return; }
        if (recipients.some(address => !validAddress(address))) { setStatus(inst, t(inst.ctx, 'invalid_address'), true); return; }
        cancelAssist(inst);
        const assistRun = inst.host.querySelector('[data-assist-run]');
        if (assistRun) assistRun.disabled = false;
        inst.sending = true;
        syncSendButton(inst);
        try {
            await Promise.all([inst.uploadTask, inst.forwardCopyPromise, inst.attachmentTask]);
            if (inst.draftConflict) { setStatus(inst, t(inst.ctx, 'draft_conflict_message'), true); return; }
            if (inst.failedUploads.size) { setStatus(inst, t(inst.ctx, 'upload_failed_block_send'), true); return; }
            if (inst.conflictMissingAttachments.length) { setStatus(inst, t(inst.ctx, 'draft_conflict_attachments_changed'), true); return; }
            if (inst.forwardSources.some(item => !inst.forwardCopied.has(item.attachmentId) && !inst.forwardSkipped.has(item.attachmentId))) {
                setStatus(inst, t(inst.ctx, 'forward_attachment_copy_review'), true);
                return;
            }
            const draft = await save(inst);
            if (!draft.id || !draft.account_id) throw new Error('draft_missing');
            const sendAt = inst.host.querySelector('[data-send-at]').value;
            const payload = { draft_id: draft.id, revision: draft.revision };
            if (sendAt) { payload.send_at = new Date(sendAt).toISOString(); payload.timezone = Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'; }
            const item = await inst.api.enqueue(payload);
            inst.committed = true;
            inst.sending = false;
            inst.outboxItem = item.item || item;
            syncSendButton(inst);
            const undo = inst.host.querySelector('[data-send-undo]');
            const scheduled = !!sendAt;
            undo.hidden = !inst.outboxItem.undo_until;
            inst.host.querySelector('[data-send-undo-copy]').textContent = scheduled ? t(inst.ctx, 'scheduled_status') : t(inst.ctx, 'send_undo_window');
            inst.host.querySelector('[data-compose-form]').classList.add('is-send-queued');
            if (inst.outboxItem.undo_until && !scheduled) {
                const until = new Date(inst.outboxItem.undo_until).getTime();
                inst.statusTimer = setInterval(() => {
                    const seconds = Math.max(0, Math.ceil((until - Date.now()) / 1000));
                    inst.host.querySelector('[data-send-undo-copy]').textContent = t(inst.ctx, 'undo_countdown', { seconds });
                    if (!seconds) { clearInterval(inst.statusTimer); undo.hidden = true; }
                }, 250);
            }
            setStatus(inst, scheduled ? t(inst.ctx, 'scheduled_status') : t(inst.ctx, 'send_queued'));
        } catch (error) { setStatus(inst, errorText(inst, error), true); }
        finally { if (!inst.committed) { inst.sending = false; syncSendButton(inst); } }
    }

    async function undoSend(inst) {
        if (!inst.outboxItem || !inst.outboxItem.id) return;
        try {
            await inst.api.cancelSend(inst.outboxItem.id);
            inst.committed = false;
            inst.sending = false;
            inst.outboxItem = null;
            clearInterval(inst.statusTimer);
            inst.host.querySelector('[data-send-undo]').hidden = true;
            inst.host.querySelector('[data-compose-form]').classList.remove('is-send-queued');
            inst.host.querySelector('[data-undo-send]').disabled = false;
            syncSendButton(inst);
            changed(inst);
            setStatus(inst, t(inst.ctx, 'send_undone'));
        } catch (error) { setStatus(inst, errorText(inst, error), true); }
    }

    function syncAssistFields(inst) {
        const translate = inst.host.querySelector('[data-assist-action]').value === 'translate';
        inst.host.querySelector('[data-assist-language-wrap]').hidden = !translate;
        inst.host.querySelector('[data-assist-instruction-wrap]').hidden = translate;
    }

    function syncAssistReview(inst) {
        const body = String(inst.markdown ? inst.markdown() : inst.draft.text || '');
        const context = inst.host.querySelector('[data-assist-context]').value;
        inst.host.querySelector('[data-assist-body-review]').textContent = body || t(inst.ctx, 'assist_empty_body');
        inst.host.querySelector('[data-assist-context-review]').textContent = context || t(inst.ctx, 'assist_no_context');
        const provider = inst.host.querySelector('[data-assist-provider]');
        const label = provider.selectedOptions[0]?.textContent || defaultProviderLabel(inst.ctx);
        const guardian = inst.ctx.state?.assist_guardian;
        const transfer = `${t(inst.ctx, 'assist_transfer_provider')}: ${label}`;
        const guardianLabel = guardian ? `LLM Guardian · ${guardian.name || guardian.id || ''}${guardian.model ? ` · ${guardian.model}` : ''}` : '';
        inst.host.querySelector('[data-assist-provider-review]').textContent = [transfer, guardianLabel].filter(Boolean).join(' · ');
    }

    async function runAssist(inst) {
        if (editingLocked(inst) || inst.preserveHTML) return;
        try { await save(inst); } catch (error) { setStatus(inst, errorText(inst, error), true); return; }
        const markdown = inst.markdown ? inst.markdown() : '';
        const context = inst.host.querySelector('[data-assist-context]').value;
        const bytes = value => new TextEncoder().encode(String(value || '')).length;
        if (bytes(markdown) > 32768 || bytes(context) > 32768) { inst.host.querySelector('[data-assist-status]').textContent = t(inst.ctx, 'assist_too_large'); return; }
        syncAssistReview(inst);
        cancelAssist(inst);
        const controller = new AbortController();
        inst.assistController = controller;
        const generation = inst.editGeneration;
        const revision = Number(inst.draft.revision || 0);
        const request = {
            action: inst.host.querySelector('[data-assist-action]').value,
            draft_id: inst.draft.id || '',
            revision,
            provider_id: inst.host.querySelector('[data-assist-provider]').value,
            context,
            assist_policy: inst.ctx.state?.assist_policy || '',
            instruction: inst.host.querySelector('[data-assist-instruction]').value,
            language: inst.host.querySelector('[data-assist-language]').value,
            text: markdown
        };
        inst.host.querySelector('[data-assist-run]').disabled = true;
        inst.host.querySelector('[data-assist-status]').textContent = t(inst.ctx, 'assist_working');
        try {
            const result = await inst.api.assist(Object.assign(request, { signal: controller.signal }));
            if (inst.disposed || controller.signal.aborted || editingLocked(inst)) return;
            const preview = result.preview || result;
            const providerId = result.provider_id || request.provider_id || inst.ctx.state?.default_provider_id || '';
            const providerConfig = providerRows(inst.ctx).find(item => item.id === providerId);
            inst.assistPreview = { generation, revision, preview, provider: providerId, model: result.model || providerConfig?.model || '' };
            const node = inst.host.querySelector('[data-assist-preview-content]');
            const providerName = providerConfig?.name || inst.assistPreview.provider || t(inst.ctx, 'provider_default');
            const providerModel = inst.assistPreview.model ? ` · ${inst.assistPreview.model}` : '';
            node.innerHTML = `<p>${esc(t(inst.ctx, 'assist_preview_from'))} ${esc(providerName)}${esc(providerModel)}</p><div>${preview.html ? (window.DOMPurify ? window.DOMPurify.sanitize(preview.html) : esc(preview.text || '')) : `<pre>${esc(preview.text || '')}</pre>`}</div>`;
            inst.host.querySelector('[data-assist-preview]').hidden = false;
            inst.host.querySelector('[data-assist-status]').textContent = t(inst.ctx, 'assist_ready');
        } catch (error) {
            if (error.name !== 'AbortError') inst.host.querySelector('[data-assist-status]').textContent = errorText(inst, error);
        } finally { if (!inst.disposed) inst.host.querySelector('[data-assist-run]').disabled = editingLocked(inst); }
    }

    function cancelAssist(inst) {
        if (inst.assistController) inst.assistController.abort();
        inst.assistController = null;
    }

    function discardAssist(inst) {
        if (editingLocked(inst)) return;
        inst.assistPreview = null;
        inst.host.querySelector('[data-assist-preview]').hidden = true;
        inst.host.querySelector('[data-assist-status]').textContent = t(inst.ctx, 'assist_discarded');
    }

    function applyAssist(inst) {
        if (editingLocked(inst)) return;
        const state = inst.assistPreview;
        if (!state) return;
        if (state.generation !== inst.editGeneration || state.revision !== Number(inst.draft.revision || 0)) {
            discardAssist(inst);
            inst.host.querySelector('[data-assist-status]').textContent = t(inst.ctx, 'assist_stale');
            return;
        }
        const text = String(state.preview.text || '');
        if (inst.setMarkdown) { inst.bodyChanged = true; inst.setMarkdown(text); }
        changed(inst);
        discardAssist(inst);
        inst.host.querySelector('[data-assist-context]').value = '';
    }

    function errorText(inst, error) {
        if (error?.body?.error === 'assist_configuration_changed' || error?.message === 'assist_configuration_changed') return t(inst.ctx, 'assist_configuration_changed');
        if (error && error.status === 409) return t(inst.ctx, 'draft_conflict');
        if (error && error.status === 403) return t(inst.ctx, 'readonly');
        return t(inst.ctx, 'action_failed');
    }

    function setStatus(inst, message, isError) {
        const status = inst.host.querySelector('[data-compose-status]');
        if (status) { status.textContent = message; status.classList.toggle('is-error', !!isError); }
        const saveState = inst.host.querySelector('[data-save-state]');
        if (saveState && isError) { saveState.textContent = message; saveState.classList.add('is-error'); }
    }

    async function dispose(windowId) {
        const inst = active.get(windowId);
        if (!inst) return;
        inst.disposed = true;
        inst.stickers?.dispose();
        inst.uploadController.abort();
        inst.uploads.clear(); inst.failedUploads.clear();
        clearTimeout(inst.saveTimer);
        clearInterval(inst.statusTimer);
        cancelAssist(inst);
        inst.listeners.forEach(([target, type, handler, options]) => target.removeEventListener(type, handler, options));
        inst.editor?.destroy();
        active.delete(windowId);
    }

    window.MailerCompose = { render, dispose, beforeClose };
})();
