(function () {
    'use strict';
    // Structural commands the engine cannot record as a tracked change.
    const UNTRACKABLE = new Set([
        'insertTable', 'insertRow', 'insertColumn', 'insertImage', 'insertNote', 'insertToc', 'insertBreak', 'insertPageField', 'insertHyperlink',
        'deleteTable', 'deleteRow', 'deleteColumn', 'deleteNote', 'mergeCells', 'splitCell',
        'setPageSetup', 'setTableBorders', 'setCellFill', 'setImageProperties', 'setImageWrapType', 'setNoteProperties', 'convertNote'
    ]);
    // The engine refuses these two; scripts/writer-table-ops.js edits the document tree instead.
    const TABLE_OPS = new Set(['mergeCells', 'splitCell']);
    // The OOXML highlight palette holds named colours only.
    const HIGHLIGHTS = {black: '000000', blue: '0000ff', cyan: '00ffff', darkBlue: '000080', darkCyan: '008080', darkGray: '808080', darkGreen: '008000',
        darkMagenta: '800080', darkRed: '800000', darkYellow: '808000', green: '00ff00', lightGray: 'c0c0c0', magenta: 'ff00ff', red: 'ff0000', white: 'ffffff', yellow: 'ffff00'};
    // '#rrggbb' for a hex string or an engine colour object, '' for anything else.
    function colorHex(value) {
        const raw = value && typeof value === 'object' ? (value.value ?? value.resolvedHex ?? '') : value;
        const body = String(raw ?? '').trim().replace(/^#/, '');
        return /^[0-9a-fA-F]{6}$/.test(body) ? '#' + body.toLowerCase() : '';
    }
    // The palette name closest to a picked colour.
    function highlightName(value) {
        const hex = colorHex(value);
        if (!hex) return '';
        const red = parseInt(hex.slice(1, 3), 16), green = parseInt(hex.slice(3, 5), 16), blue = parseInt(hex.slice(5, 7), 16);
        let nearest = '', distance = Infinity;
        for (const [name, candidate] of Object.entries(HIGHLIGHTS)) {
            const delta = (red - parseInt(candidate.slice(0, 2), 16)) ** 2 + (green - parseInt(candidate.slice(2, 4), 16)) ** 2 + (blue - parseInt(candidate.slice(4, 6), 16)) ** 2;
            if (delta < distance) { distance = delta; nearest = name; }
        }
        return nearest;
    }
    function highlightHex(value) {
        const name = String(value ?? '').trim();
        return HIGHLIGHTS[name] ? '#' + HIGHLIGHTS[name] : colorHex(value);
    }
    const shows = name => snapshot => (snapshot.reviewDisplayMode || 'all-markup') === name;
    const zoom = percent => ({id: 'zoom' + percent, text: percent + '%', needsEditor: true});
    const lines = value => ({id: 'spacing' + Math.round(value * 100), command: {type: 'setLineSpacing', rule: 'multiple', value}, number: value,
        active: snapshot => snapshot.formatting?.lineSpacing?.rule === 'multiple' && snapshot.formatting.lineSpacing.value === value});
    // One entry per command a surface can offer. An entry is one of three kinds:
    //   slot       an engine toolbar slot
    //   command    an engine command that has no slot
    //   (neither)  an app action; its `case` lives in act() in writer.js
    // Fields:
    //   label      desktop.writer_* key; fullLabel names a key outside that prefix; text is a literal; number is formatted for the locale
    //   icon       shared icon key, for menus and the toolbar; glyph is drawn by writer-toolbar.js and used there only
    //   shortcut   shown in menus and tooltips, and dispatched by the shell outside editable content; keys is what Writer binds itself
    //   hint       a shortcut that is only shown, because the engine or the browser owns the key
    //   toggle     pressed state from the engine, for the toolbar; active(snapshot) is a state menus show as a check mark
    //   control    'select', 'number' or 'color' instead of a button; options are the values of a select; value is a colour default
    //   write      refused in a read-only window; needsEditor: refused without a document
    //   scope      editing scope ('headerFooter') an app action belongs to; refused anywhere else
    //   untracked  command type whose tracking refusal disables the action
    //   clipboard  'read' or 'write': refused when the browser gives no clipboard access (plain http)
    const DEFINITIONS = [
        {id: 'undo', slot: 'history.undo', label: 'undo', icon: 'undo', fallback: '↶', shortcut: 'Ctrl+Z'},
        {id: 'redo', slot: 'history.redo', label: 'redo', icon: 'redo', fallback: '↷', shortcut: 'Ctrl+Y'},
        {id: 'cut', command: {type: 'cut'}, label: 'cut', icon: 'cut', hint: 'Ctrl+X', clipboard: 'write'},
        {id: 'copy', command: {type: 'copy'}, label: 'copy', icon: 'copy', hint: 'Ctrl+C', clipboard: 'write'},
        {id: 'paste', label: 'paste', icon: 'paste', hint: 'Ctrl+V', write: true, needsEditor: true, clipboard: 'read'},
        {id: 'pastePlain', label: 'paste_plain', hint: 'Ctrl+Shift+V', write: true, needsEditor: true, clipboard: 'read'},
        {id: 'selectAll', command: {type: 'selectAll'}, label: 'select_all', hint: 'Ctrl+A'},
        {id: 'style', slot: 'styles.style', label: 'style', control: 'select'},
        {id: 'fontFamily', slot: 'font.family', label: 'font', control: 'select'},
        {id: 'fontSize', slot: 'font.size', label: 'size', control: 'number'},
        {id: 'bold', slot: 'text.bold', label: 'bold', icon: 'format-bold', fallback: 'B', toggle: true, hint: 'Ctrl+B'},
        {id: 'italic', slot: 'text.italic', label: 'italic', icon: 'format-italic', fallback: 'I', toggle: true, hint: 'Ctrl+I'},
        {id: 'underline', slot: 'text.underline', label: 'underline', glyph: 'format-underline', fallback: 'U', toggle: true, hint: 'Ctrl+U'},
        {id: 'strike', slot: 'text.strike', label: 'strike', icon: 'format-strike', fallback: 'S', toggle: true},
        {id: 'superscript', slot: 'script.super', label: 'superscript', toggle: true},
        {id: 'subscript', slot: 'script.sub', label: 'subscript', toggle: true},
        {id: 'clearFormat', slot: 'format.clear', label: 'clear_format', glyph: 'clear-format', fallback: 'T'},
        {id: 'copyFormat', command: {type: 'copyFormatting'}, label: 'copy_format'},
        {id: 'pasteFormat', command: {type: 'pasteFormatting'}, label: 'paste_format'},
        {id: 'alignLeft', slot: 'alignment.left', label: 'align_left', glyph: 'align-left', fallback: '≡', toggle: true},
        {id: 'alignCenter', slot: 'alignment.center', label: 'align_center', glyph: 'align-center', fallback: '≡', toggle: true},
        {id: 'alignRight', slot: 'alignment.right', label: 'align_right', glyph: 'align-right', fallback: '≡', toggle: true},
        {id: 'alignJustify', slot: 'alignment.justify', label: 'justify', glyph: 'align-justify', fallback: '≡', toggle: true},
        {id: 'lineSpacing', slot: 'list.lineSpacing', label: 'line_spacing', control: 'select', options: [1, 1.15, 1.5, 2]},
        lines(1), lines(1.15), lines(1.5), lines(2),
        {id: 'bullets', slot: 'list.bullet', label: 'bullets', icon: 'list', fallback: '•', toggle: true},
        {id: 'numbering', slot: 'list.numbered', label: 'numbering', icon: 'format-numbered', fallback: '1.', toggle: true},
        {id: 'outdent', slot: 'list.outdent', label: 'outdent', glyph: 'outdent', fallback: '⇤'},
        {id: 'indent', slot: 'list.indent', label: 'indent', glyph: 'indent', fallback: '⇥'},
        {id: 'textColor', slot: 'text.color', label: 'color', glyph: 'text-color', control: 'color', value: '#172033'},
        {id: 'highlight', slot: 'text.highlight', label: 'highlight', glyph: 'highlight', control: 'color', value: '#ffff00'},
        {id: 'new', label: 'new', icon: 'file-plus', shortcut: 'Ctrl+N', keys: ['n'], write: true},
        {id: 'open', label: 'open', icon: 'folder-open', shortcut: 'Ctrl+O', keys: ['o']},
        {id: 'save', label: 'save', icon: 'save', shortcut: 'Ctrl+S', keys: ['s'], write: true, needsEditor: true},
        {id: 'saveAs', label: 'save_as', icon: 'copy', shortcut: 'Ctrl+Shift+S', keys: ['shift+s'], write: true, needsEditor: true},
        {id: 'docx', label: 'download_docx', icon: 'file-text', needsEditor: true},
        {id: 'html', label: 'export_html', icon: 'file-code', needsEditor: true},
        {id: 'md', label: 'export_md', icon: 'notes', needsEditor: true},
        {id: 'txt', label: 'export_txt', icon: 'file-text', needsEditor: true},
        {id: 'print', label: 'print', icon: 'printer', shortcut: 'Ctrl+P', keys: ['p'], needsEditor: true},
        {id: 'search', label: 'find', icon: 'search', fallback: '⌕', shortcut: 'Ctrl+F', keys: ['f', 'h']},
        {id: 'goToPage', label: 'go_page', shortcut: 'Ctrl+G', keys: ['g'], needsEditor: true},
        {id: 'outline', label: 'outline', icon: 'list', fallback: '☷'},
        {id: 'format', label: 'format', icon: 'sliders', fallback: 'A'},
        {id: 'review', label: 'review', icon: 'chat', fallback: '☷'},
        {id: 'assist', label: 'assist', icon: 'star', fallback: '✦'},
        {id: 'insert', label: 'insert', icon: 'plus', fallback: '＋'},
        {id: 'marks', slot: 'review.paragraphMarks', label: 'marks', active: snapshot => !!snapshot.showParagraphMarks},
        {id: 'markupAll', slot: 'review.allMarkup', label: 'markup_all', active: shows('all-markup')},
        {id: 'markupFinal', slot: 'review.noMarkup', label: 'markup_final', active: shows('proposed')},
        {id: 'markupOriginal', slot: 'review.original', label: 'original', active: shows('original')},
        {id: 'focus', label: 'focus_mode', icon: 'maximize', fallback: '⛶'},
        {id: 'fit', label: 'fit', icon: 'monitor'},
        {id: 'zoomPage', label: 'zoom_page', needsEditor: true},
        zoom(50), zoom(75), zoom(100), zoom(125), zoom(150), zoom(200),
        {id: 'zoomIn', label: 'zoom_in', icon: 'zoom-in', needsEditor: true},
        {id: 'zoomOut', label: 'zoom_out', icon: 'zoom-out', needsEditor: true},
        {id: 'table', label: 'table', glyph: 'table', fallback: '▦', write: true, needsEditor: true, untracked: 'insertTable'},
        {id: 'image', label: 'image', icon: 'image', fallback: '▣', write: true, needsEditor: true},
        {id: 'qr', fullLabel: 'desktop.qr_insert', icon: 'image', write: true, needsEditor: true, untracked: 'insertImage'},
        {id: 'link', label: 'hyperlink', icon: 'link', fallback: '∞', shortcut: 'Ctrl+K', keys: ['k'], write: true, needsEditor: true, untracked: 'insertHyperlink'},
        {id: 'pageBreak', slot: 'insert.pageBreak', label: 'page_break'},
        {id: 'sectionNext', slot: 'insert.sectionBreakNextPage', label: 'section_next'},
        {id: 'sectionContinuous', slot: 'insert.sectionBreakContinuous', label: 'section_continuous'},
        {id: 'footnote', slot: 'insert.footnote', label: 'footnote'},
        {id: 'endnote', slot: 'insert.endnote', label: 'endnote'},
        {id: 'toc', slot: 'insert.toc', label: 'toc'},
        {id: 'textBox', slot: 'insert.textBox', label: 'text_box'},
        {id: 'header', command: {type: 'editHeaderFooter', position: 'header'}, label: 'header'},
        {id: 'footer', command: {type: 'editHeaderFooter', position: 'footer'}, label: 'footer'},
        {id: 'pageNumber', slot: 'insert.pageXofY', label: 'page_numbers'},
        {id: 'backDocument', command: {type: 'exitHeaderFooter'}, label: 'back_document'},
        {id: 'rowAbove', command: {type: 'insertRow', where: 'above'}, label: 'row_above', glyph: 'row-above', fallback: '↥'},
        {id: 'rowBelow', command: {type: 'insertRow', where: 'below'}, label: 'row_below', glyph: 'row-below', fallback: '↧'},
        {id: 'columnLeft', command: {type: 'insertColumn', where: 'left'}, label: 'column_left', glyph: 'col-left', fallback: '↤'},
        {id: 'columnRight', command: {type: 'insertColumn', where: 'right'}, label: 'column_right', glyph: 'col-right', fallback: '↦'},
        {id: 'deleteRow', command: {type: 'deleteRow'}, label: 'delete_row', glyph: 'row-delete', fallback: '⊟'},
        {id: 'deleteColumn', command: {type: 'deleteColumn'}, label: 'delete_column', glyph: 'col-delete', fallback: '⊟'},
        {id: 'deleteTable', command: {type: 'deleteTable'}, label: 'table_delete', glyph: 'table-delete', fallback: '⊠'},
        {id: 'mergeCells', command: {type: 'mergeCells'}, label: 'merge_cells', glyph: 'merge-cells', fallback: '⧉'},
        {id: 'splitCell', command: {type: 'splitCell', rows: 1, cols: 2}, label: 'split_cell', glyph: 'split-cell', fallback: '◫'},
        {id: 'borders', command: {type: 'setTableBorders', scope: 'all', spec: {style: 'single', size: 4, color: {kind: 'hex', value: '64748B'}}}, label: 'borders', glyph: 'borders-all', fallback: '▦'},
        {id: 'noBorders', command: {type: 'setTableBorders', scope: 'none', target: 'all'}, label: 'no_borders', glyph: 'borders-none', fallback: '▢'},
        {id: 'rotateLeft', command: {type: 'transformImage', action: 'rotateCCW'}, label: 'rotate_left', glyph: 'rotate-left', fallback: '↺'},
        {id: 'rotateRight', command: {type: 'transformImage', action: 'rotateCW'}, label: 'rotate_right', glyph: 'rotate-right', fallback: '↻'},
        {id: 'deleteImage', command: {type: 'deleteImage'}, label: 'image_delete', icon: 'trash'},
        {id: 'comment', label: 'comment', icon: 'chat', write: true, needsEditor: true},
        {id: 'headerFirstPage', label: 'header_first_page', write: true, needsEditor: true, scope: 'headerFooter'},
        {id: 'headerOddEven', label: 'header_odd_even', write: true, needsEditor: true, scope: 'headerFooter'},
        {id: 'wordCount', label: 'word_count_title', needsEditor: true},
        {id: 'tracking', label: 'tracking', write: true, needsEditor: true},
        {id: 'previousChange', slot: 'review.previousChange', label: 'previous_change'},
        {id: 'nextChange', slot: 'review.nextChange', label: 'next_change'},
        {id: 'shortcuts', label: 'shortcuts', icon: 'keyboard'}
    ];
    // host: {tr, t, readonly, editor(), lib(), loading(), act(id), notice(message, error), refresh(),
    //        active(id)?: whether an app action is switched on, format(number)?: a number in the user's locale,
    //        clipboard(kind)?: whether the system clipboard can be read or written}
    function create(host) {
        const byId = new Map(DEFINITIONS.map(definition => [definition.id, definition]));
        const get = id => byId.get(id);
        const tracking = () => host.editor()?.snapshot().editingMode === 'suggesting';
        // Why a command type is refused before the engine is asked; '' when it is not.
        const blocked = type => tracking() && UNTRACKABLE.has(type) ? host.tr('tracking_unsupported') : '';
        // Why a clipboard command is refused before anything runs; '' when it is not.
        const denied = definition => definition.clipboard && host.clipboard && !host.clipboard(definition.clipboard)
            ? host.t('desktop.clipboard_' + definition.clipboard + '_unavailable') : '';
        function can(command) {
            const editor = host.editor();
            if (!editor || !command) return {ok: false};
            const reason = blocked(command.type);
            if (reason) return {ok: false, reason};
            return TABLE_OPS.has(command.type) ? host.lib().tableCommand(editor, command, true) : editor.can(command);
        }
        const refuse = reason => { host.notice(host.tr('command_unavailable') + (reason ? ' · ' + reason : ''), true); return false; };
        function run(command) {
            const editor = host.editor();
            if (!editor || !command || host.loading()) return false;
            const allowed = can(command);
            if (!allowed.ok) return refuse(allowed.reason);
            const result = TABLE_OPS.has(command.type) ? host.lib().tableCommand(editor, command) : editor.exec(command);
            if (!result.ok) refuse(result.reason);
            host.refresh();
            return result.ok;
        }
        const format = value => (host.format || String)(value);
        function label(id) {
            const definition = get(id);
            if (definition.text) return definition.text;
            if (definition.number !== undefined) return format(definition.number);
            return definition.fullLabel ? host.t(definition.fullLabel) : host.tr(definition.label);
        }
        const tooltip = id => label(id) + (get(id).shortcut || get(id).hint ? ' (' + (get(id).shortcut || get(id).hint) + ')' : '');
        // The engine command behind an entry; null for value slots and app actions.
        function command(id) {
            const definition = get(id);
            if (definition?.command) return definition.command;
            return definition?.slot && host.lib() ? host.lib().commandForSlot(definition.slot) : null;
        }
        function state(id) {
            const definition = get(id), editor = host.editor(), off = {enabled: false, active: false, reason: ''};
            if (!definition) return off;
            const refusal = denied(definition);
            if (refusal) return {enabled: false, active: false, reason: refusal};
            if (definition.slot || definition.command) {
                const engineCommand = editor ? command(id) : null;
                if (!engineCommand) return off;
                const allowed = can(engineCommand);
                const active = definition.active ? !!definition.active(editor.snapshot()) : !!definition.toggle && !!editor.isActive(engineCommand);
                return {enabled: !!allowed.ok, active, reason: allowed.reason || ''};
            }
            if ((definition.write && host.readonly) || (definition.needsEditor && !editor)) return off;
            if (definition.scope && editor.snapshot().scope?.kind !== definition.scope) return off;
            const reason = definition.untracked ? blocked(definition.untracked) : '';
            return {enabled: !reason, active: !!host.active?.(id), reason};
        }
        // Engine commands announce their own refusals in run(); app actions and clipboard commands are gated here.
        function invoke(id) {
            const definition = get(id);
            if (!definition) return false;
            const engine = definition.slot || definition.command;
            if (!engine || definition.clipboard) {
                const current = state(id);
                if (!current.enabled) return current.reason ? refuse(current.reason) : false;
            }
            return engine ? run(command(id)) : host.act(id);
        }
        // The engine wants a bare hex for colours, a palette name for highlights, half-points for sizes and lines for spacing.
        function valueCommand(slot, value) {
            const lib = host.lib();
            if (slot === 'text.color') return lib.commandForSlotValue(slot, colorHex(value).replace('#', ''));
            if (slot === 'text.highlight') return lib.commandForSlotValue(slot, highlightName(value));
            if (slot === 'font.size') return lib.commandForSlotValue(slot, Number(value) * 2);
            if (slot === 'list.lineSpacing') return lib.commandForSlotValue(slot, Number(value) || 1);
            return lib.commandForSlotValue(slot, value);
        }
        // The command a key press asks for, or '' when Writer does not bind it. Alt is excluded
        // because AltGr reports Ctrl+Alt, and those letters are text.
        function actionForKey(event) {
            if (!(event.ctrlKey || event.metaKey) || event.altKey) return '';
            const key = (event.shiftKey ? 'shift+' : '') + String(event.key).toLowerCase();
            return DEFINITIONS.find(definition => definition.keys?.includes(key))?.id || '';
        }
        // A menu item with the state of this moment. The shell reads `disabled` and `checked` when the
        // menus are set, so writer-menus.js sets them again when that state changes.
        function menuItem(id) {
            const definition = get(id), current = state(id);
            const item = {id, label: label(id), icon: definition.icon || '', disabled: !current.enabled, action: () => invoke(id)};
            if (definition.shortcut) item.shortcut = definition.shortcut;
            else if (definition.hint) item.shortcutHint = definition.hint;
            // A pressed toolbar toggle follows the selection; menus only mark modes and open panels.
            if (current.active && !definition.toggle) item.checked = true;
            if (!current.enabled && current.reason) item.disabledHint = current.reason;
            return item;
        }
        return {get, can, run, blocked, tracking, label, tooltip, format, command, state, invoke, valueCommand, actionForKey, menuItem};
    }
    window.WriterCommands = {create, definitions: DEFINITIONS, untrackable: type => UNTRACKABLE.has(type), colors: {hex: colorHex, highlightName, highlightHex}};
})();
