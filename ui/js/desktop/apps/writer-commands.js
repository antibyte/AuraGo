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
    // One entry per command a surface can offer.
    //   slot       engine toolbar slot; without it the id is an app action handled by writer.js
    //   label      desktop.writer_* key; fullLabel names a key outside that prefix
    //   icon       shared icon key or a glyph drawn by writer-toolbar.js; fallback is the text stand-in
    //   shortcut   what menus and tooltips show; keys is what Writer itself binds (the engine binds the rest)
    //   write      refused in a read-only window; needsEditor: refused without a document
    //   untracked  command type whose tracking refusal disables the action
    const DEFINITIONS = [
        {id: 'undo', slot: 'history.undo', label: 'undo', icon: 'undo', fallback: '↶', shortcut: 'Ctrl+Z'},
        {id: 'redo', slot: 'history.redo', label: 'redo', icon: 'redo', fallback: '↷', shortcut: 'Ctrl+Y'},
        {id: 'style', slot: 'styles.style', label: 'style', control: 'select'},
        {id: 'fontFamily', slot: 'font.family', label: 'font', control: 'select'},
        {id: 'fontSize', slot: 'font.size', label: 'size', control: 'number'},
        {id: 'bold', slot: 'text.bold', label: 'bold', icon: 'format-bold', fallback: 'B', toggle: true},
        {id: 'italic', slot: 'text.italic', label: 'italic', icon: 'format-italic', fallback: 'I', toggle: true},
        {id: 'underline', slot: 'text.underline', label: 'underline', icon: 'format-underline', fallback: 'U', toggle: true},
        {id: 'strike', slot: 'text.strike', label: 'strike', icon: 'format-strike', fallback: 'S', toggle: true},
        {id: 'alignLeft', slot: 'alignment.left', label: 'align_left', icon: 'align-left', fallback: '≡', toggle: true},
        {id: 'alignCenter', slot: 'alignment.center', label: 'align_center', icon: 'align-center', fallback: '≡', toggle: true},
        {id: 'alignRight', slot: 'alignment.right', label: 'align_right', icon: 'align-right', fallback: '≡', toggle: true},
        {id: 'alignJustify', slot: 'alignment.justify', label: 'justify', icon: 'align-justify', fallback: '≡', toggle: true},
        {id: 'bullets', slot: 'list.bullet', label: 'bullets', icon: 'list', fallback: '•', toggle: true},
        {id: 'numbering', slot: 'list.numbered', label: 'numbering', icon: 'format-numbered', fallback: '1.', toggle: true},
        {id: 'outdent', slot: 'list.outdent', label: 'outdent', icon: 'outdent', fallback: '⇤'},
        {id: 'indent', slot: 'list.indent', label: 'indent', icon: 'indent', fallback: '⇥'},
        {id: 'textColor', slot: 'text.color', label: 'color', icon: 'text-color', control: 'color', value: '#172033'},
        {id: 'highlight', slot: 'text.highlight', label: 'highlight', icon: 'highlight', control: 'color', value: '#ffff00'},
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
        {id: 'outline', label: 'outline', icon: 'list', fallback: '☷'},
        {id: 'format', label: 'format', icon: 'sliders', fallback: 'A'},
        {id: 'review', label: 'review', icon: 'chat', fallback: '☷'},
        {id: 'assist', label: 'assist', icon: 'star', fallback: '✦'},
        {id: 'insert', label: 'insert', icon: 'plus', fallback: '＋'},
        {id: 'image', label: 'image', icon: 'image', write: true, needsEditor: true},
        {id: 'qr', fullLabel: 'desktop.qr_insert', icon: 'image', write: true, needsEditor: true, untracked: 'insertImage'},
        {id: 'focus', label: 'focus_mode', icon: 'maximize', fallback: '⛶'},
        {id: 'fit', label: 'fit', icon: 'monitor'}
    ];
    // host: {tr, t, readonly, editor(), lib(), loading(), act(id), notice(message, error), refresh()}
    function create(host) {
        const byId = new Map(DEFINITIONS.map(definition => [definition.id, definition]));
        const get = id => byId.get(id);
        const tracking = () => host.editor()?.snapshot().editingMode === 'suggesting';
        // Why a command type is refused before the engine is asked; '' when it is not.
        const blocked = type => tracking() && UNTRACKABLE.has(type) ? host.tr('tracking_unsupported') : '';
        function can(command) {
            const editor = host.editor();
            if (!editor || !command) return {ok: false};
            const reason = blocked(command.type);
            if (reason) return {ok: false, reason};
            return TABLE_OPS.has(command.type) ? host.lib().tableCommand(editor, command, true) : editor.can(command);
        }
        function run(command) {
            const editor = host.editor();
            if (!editor || !command || host.loading()) return false;
            const refuse = result => host.notice(host.tr('command_unavailable') + (result.reason ? ' · ' + result.reason : ''), true);
            const allowed = can(command);
            if (!allowed.ok) { refuse(allowed); return false; }
            const result = TABLE_OPS.has(command.type) ? host.lib().tableCommand(editor, command) : editor.exec(command);
            if (!result.ok) refuse(result);
            host.refresh();
            return result.ok;
        }
        const label = id => get(id).fullLabel ? host.t(get(id).fullLabel) : host.tr(get(id).label);
        const tooltip = id => label(id) + (get(id).shortcut ? ' (' + get(id).shortcut + ')' : '');
        // The engine command behind a slot entry; null for value slots and app actions.
        const command = id => get(id)?.slot && host.lib() ? host.lib().commandForSlot(get(id).slot) : null;
        function state(id) {
            const definition = get(id), editor = host.editor(), off = {enabled: false, active: false, reason: ''};
            if (!definition) return off;
            if (definition.slot) {
                const engineCommand = editor ? command(id) : null;
                if (!engineCommand) return off;
                const allowed = can(engineCommand);
                return {enabled: !!allowed.ok, active: !!definition.toggle && !!editor.isActive(engineCommand), reason: allowed.reason || ''};
            }
            if ((definition.write && host.readonly) || (definition.needsEditor && !editor)) return off;
            const reason = definition.untracked ? blocked(definition.untracked) : '';
            return {enabled: !reason, active: false, reason};
        }
        const invoke = id => get(id)?.slot ? run(command(id)) : host.act(id);
        // The engine wants a bare hex for colours, a palette name for highlights and half-points for sizes.
        function valueCommand(slot, value) {
            const lib = host.lib();
            if (slot === 'text.color') return lib.commandForSlotValue(slot, colorHex(value).replace('#', ''));
            if (slot === 'text.highlight') return lib.commandForSlotValue(slot, highlightName(value));
            if (slot === 'font.size') return lib.commandForSlotValue(slot, Number(value) * 2);
            return lib.commandForSlotValue(slot, value);
        }
        // The command a key press asks for, or '' when Writer does not bind it. Alt is excluded
        // because AltGr reports Ctrl+Alt, and those letters are text.
        function actionForKey(event) {
            if (!(event.ctrlKey || event.metaKey) || event.altKey) return '';
            const key = (event.shiftKey ? 'shift+' : '') + String(event.key).toLowerCase();
            return DEFINITIONS.find(definition => definition.keys?.includes(key))?.id || '';
        }
        // Window menus are built once per document, so a slot item is disabled only without a document.
        const menuItem = id => ({id, label: label(id), icon: get(id).icon || id, shortcut: get(id).shortcut,
            disabled: get(id).slot ? !host.editor() : !state(id).enabled, action: () => invoke(id)});
        return {get, can, run, blocked, label, tooltip, command, state, invoke, valueCommand, actionForKey, menuItem};
    }
    window.WriterCommands = {create, definitions: DEFINITIONS, untrackable: type => UNTRACKABLE.has(type), colors: {hex: colorHex, highlightName, highlightHex}};
})();
