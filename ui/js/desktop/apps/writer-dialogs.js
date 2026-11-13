(function () {
    'use strict';
    // app: the state facade of writer.js. Every dialog asks through the shared form dialog and
    // acts through the registry gate, so tracking and read-only refusals are announced the same way.
    function create(app) {
        const {tr, ctx, commands} = app;
        const ask = (title, fields, more) => window.AuraFormDialog.open(Object.assign({title, fields, submitLabel: ctx.t('desktop.ok'),
            cancelLabel: ctx.t('desktop.cancel'), signal: app.signal()}, more));
        // A read-only dialog: rows of [label, value] and one button.
        const tell = (title, rows) => window.AuraFormDialog.open({title, submitLabel: ctx.t('desktop.ok'), signal: app.signal(),
            fields: rows.map(([label, value], index) => ({id: 'row' + index, type: 'static', label, value}))});
        async function table() {
            const size = await ask(tr('table'), [
                {id: 'grid', type: 'grid', label: tr('table'), rows: 8, cols: 10, targets: ['rows', 'cols']},
                {id: 'rows', label: tr('table_rows'), type: 'number', value: 3, min: 1, max: 100, step: 1, required: true},
                {id: 'cols', label: tr('table_columns'), type: 'number', value: 3, min: 1, max: 30, step: 1, required: true}]);
            if (size) commands.run({type: 'insertTable', rows: size.rows, cols: size.cols});
        }
        // An address without a scheme is a web address; only web, mail, phone and in-document targets are links.
        const address = value => (/^[a-z][a-z0-9+.-]*:/i.test(value) && !/^[^:/?#]+:\d/.test(value)) || value.startsWith('#') ? value : 'https://' + value;
        async function link() {
            const editor = app.editor;
            if (!editor) return;
            const current = editor.query({type: 'hyperlinkAt'});
            const selected = editor.snapshot().selectionCollapsed ? '' : editor.query({type: 'selectedText'});
            const fields = [{id: 'href', label: tr('link_address'), value: current?.href || '', placeholder: 'https://', required: true, maxlength: 2000}];
            if (!current && !selected) fields.unshift({id: 'text', label: tr('link_text'), value: '', maxlength: 500});
            const answer = await ask(tr('hyperlink'), fields, {extraLabel: current ? tr('link_remove') : '',
                validate: values => /^(https?:|ftps?:|mailto:|tel:|#)/i.test(address(values.href)) ? '' : tr('link_invalid')});
            if (!answer || app.editor !== editor) return;
            // The engine edits a link where the selection is: inserting over a link replaces its
            // address and keeps its text, and unlinking keeps the text. hyperlinkAt's range is the
            // whole paragraph, so it is never used as a target.
            if (answer.extra) { commands.run({type: 'removeHyperlink'}); return; }
            const command = {type: 'insertHyperlink', href: address(answer.href)};
            if (!current && answer.text) command.text = answer.text;
            commands.run(command);
        }
        async function goToPage() {
            const editor = app.editor;
            if (!editor) return;
            const total = Math.max(1, editor.getTotalPages()), current = editor.getCurrentPage('viewport') || 1;
            const answer = await ask(tr('go_page'), [{id: 'page', label: tr('page_count', {current, total}), type: 'number', value: current, min: 1, max: total, step: 1, required: true}]);
            if (answer && app.editor === editor) editor.scrollToPage(Math.max(1, Math.min(total, answer.page)));
        }
        function wordCount() {
            const editor = app.editor;
            if (!editor) return Promise.resolve(null);
            const text = editor.surface.session.bodyText();
            const selected = editor.snapshot().selectionCollapsed ? '' : editor.query({type: 'selectedText'}) || '';
            const rows = [[tr('pages'), editor.getTotalPages()], [tr('words'), app.countWords(text)], [tr('chars'), [...text].length],
                [tr('chars_no_spaces'), [...text.replace(/\s/g, '')].length]];
            if (selected.trim()) rows.push([tr('selected_words'), app.countWords(selected)]);
            return tell(tr('word_count_title'), rows.map(([label, value]) => [label, commands.format(value)]));
        }
        const shortcuts = () => tell(tr('shortcuts'), window.WriterCommands.definitions.filter(definition => definition.shortcut || definition.hint)
            .map(definition => [commands.label(definition.id), definition.shortcut || definition.hint]));
        // A comment belongs to the selection; the engine says so when there is none.
        async function comment() {
            const editor = app.editor;
            if (!editor) return;
            if (editor.snapshot().selectionCollapsed) { app.notice(tr('select_text'), true); return; }
            const text = await app.prompt('comment');
            if (!text?.trim() || app.editor !== editor) return;
            const result = editor.addComment(text, app.author);
            if (!result.ok) app.notice(result.reason || tr('select_text'), true);
        }
        const draftLabel = entry => {
            const parts = String(entry.path).split('/'), name = parts.pop();
            const when = entry.updated ? new Date(entry.updated).toLocaleString(document.documentElement.lang || 'en', {dateStyle: 'medium', timeStyle: 'short'}) : '';
            return [name, parts.join('/'), when].filter(Boolean).join(' · ');
        };
        // The drafts this browser kept because a save failed or the browser closed first. The list
        // never loads the documents themselves; one draft at a time is restored or deleted.
        async function drafts() {
            let list;
            try { list = (await window.WriterSession.drafts()).filter(entry => entry.path); }
            catch (_) { app.notice(tr('draft_unavailable'), true); return; }
            if (!list.length) { await tell(tr('drafts'), [[tr('drafts_none'), '']]); return; }
            const answer = await ask(tr('drafts'), [{id: 'draft', type: 'select', label: tr('draft_pick'), size: Math.min(8, Math.max(2, list.length)),
                value: list[0].key, options: list.map(entry => ({value: entry.key, label: draftLabel(entry)}))}],
                {submitLabel: tr('draft_restore'), extraLabel: tr('delete')});
            const picked = answer && list.find(entry => entry.key === answer.draft);
            if (!picked) return;
            if (!answer.extra) return app.restoreDraft(picked);
            // deleteIf leaves a draft alone that was written again since the list was read.
            await window.WriterSession.draft('deleteIf', picked.key, {id: picked.id}).catch(() => {});
            app.notice(tr('draft_deleted'));
            return drafts();
        }
        return {table, link, goToPage, wordCount, shortcuts, comment, drafts};
    }
    window.WriterDialogs = {create};
})();
