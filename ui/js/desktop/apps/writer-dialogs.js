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
        // The layout dialogs. WriterLayout turns the engine's state into the texts the fields show
        // and the answer into one command with only what changed; lengths carry the user's unit.
        const L = window.WriterLayout;
        const unit = () => tr(app.units.unit === 'in' ? 'unit_in' : 'unit_cm');
        const length = (id, label, values) => ({id, label, value: values[id], suffix: unit(), inputmode: 'decimal', maxlength: 8, half: true});
        // A select; a value the selection disagrees on shows as a dash that sends nothing.
        const choice = (id, label, values, pairs, half) => ({id, type: 'select', label, value: values[id], half,
            options: (values[id] === '' ? [['', '—']] : []).concat(pairs).map(([value, text]) => ({value, label: text}))});
        const check = (id, label, values) => ({id, type: 'checkbox', label, value: values[id] === true, mixed: values[id] === null, half: true});
        const layout = (title, fields, model, command) => ask(title, fields, {wide: true,
            onInput: (changed, values, set) => { for (const [id, value] of Object.entries(model(changed, values, app.units))) set(id, value); },
            validate: values => { const {error} = command(values); return error ? tr(error) : ''; }});
        async function page() {
            const editor = app.editor, setup = editor?.getPageSetup();
            if (!setup) return;
            const initial = L.pageFields(setup, app.units), command = values => L.pageCommand(values, initial, app.units);
            const answer = await layout(tr('page_setup'), [
                choice('paper', tr('paper'), initial, L.papers.map(paper => [paper.id, paper.name]).concat([['custom', tr('paper_custom')]])),
                length('width', tr('page_width'), initial), length('height', tr('page_height'), initial),
                choice('orientation', tr('orientation'), initial, [['portrait', tr('portrait')], ['landscape', tr('landscape')]]),
                {id: 'marginsHeading', type: 'heading', label: tr('margins')},
                choice('preset', tr('margin_preset'), initial, [['normal', tr('margins_normal')], ['narrow', tr('margins_narrow')], ['wide', tr('margins_wide')], ['custom', tr('margins_custom')]]),
                length('top', tr('margin_top'), initial), length('bottom', tr('margin_bottom'), initial),
                length('left', tr('margin_left'), initial), length('right', tr('margin_right'), initial),
                choice('scope', tr('apply_to'), initial, [['document', tr('apply_document')], ['section', tr('apply_section')]])
            ], L.pageInput, command);
            if (!answer || app.editor !== editor) return;
            const result = command(answer);
            if (result.command) commands.run(result.command);
        }
        async function paragraph() {
            const editor = app.editor;
            if (!editor) return;
            const u = app.units, initial = L.paragraphFields(editor.snapshot().formatting, u), command = values => L.paragraphCommand(values, initial, u);
            const answer = await layout(tr('paragraph'), [
                choice('alignment', tr('alignment'), initial, [['left', tr('align_left')], ['center', tr('align_center')], ['right', tr('align_right')], ['justify', tr('justify')]]),
                {id: 'indentHeading', type: 'heading', label: tr('indent_heading')},
                length('left', tr('left_indent'), initial), length('right', tr('right_indent'), initial),
                choice('special', tr('indent_special'), initial, [['none', tr('indent_none')], ['first', tr('indent_first')], ['hanging', tr('indent_hanging')]], true),
                length('by', tr('indent_by'), initial),
                {id: 'spacingHeading', type: 'heading', label: tr('spacing_heading')},
                {id: 'before', label: tr('before_spacing'), value: initial.before, inputmode: 'decimal', maxlength: 8, half: true},
                {id: 'after', label: tr('after_spacing'), value: initial.after, inputmode: 'decimal', maxlength: 8, half: true},
                choice('lineRule', tr('line_spacing'), initial, [['single', tr('line_single')], ['115', u.number(1.15)], ['150', u.number(1.5)], ['double', tr('line_double')],
                    ['atLeast', tr('line_at_least')], ['exact', tr('line_exact')], ['multiple', tr('line_multiple')]], true),
                {id: 'lineValue', label: tr('line_value'), value: initial.lineValue, inputmode: 'decimal', maxlength: 8, half: true},
                {id: 'breaksHeading', type: 'heading', label: tr('breaks_heading')},
                check('keepNext', tr('keep_next'), initial), check('keepLines', tr('keep_lines'), initial),
                check('widowControl', tr('widow_control'), initial), check('pageBreakBefore', tr('page_break_before'), initial)
            ], L.paragraphInput, command);
            if (!answer || app.editor !== editor) return;
            const result = command(answer);
            if (result.command) commands.run(result.command);
        }
        return {table, link, goToPage, wordCount, shortcuts, comment, drafts, page, paragraph};
    }
    window.WriterDialogs = {create};
})();
