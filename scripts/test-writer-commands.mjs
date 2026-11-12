import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync, readdirSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-commands.js', 'utf8'), context);
const WriterCommands = context.window.WriterCommands;

// The untrackable set is the rule writer.js used to spell as a regular expression.
const legacy = /^(insert(Table|Row|Column|Image|Note|Toc|Break|PageField|Hyperlink)|delete(Table|Row|Column|Note)|mergeCells|splitCell|set(PageSetup|TableBorders|CellFill|ImageProperties|ImageWrapType|NoteProperties)|convertNote)$/;
const types = ['insertTable', 'insertRow', 'insertColumn', 'insertImage', 'insertNote', 'insertToc', 'insertBreak', 'insertPageField', 'insertHyperlink',
    'deleteTable', 'deleteRow', 'deleteColumn', 'deleteNote', 'mergeCells', 'splitCell', 'setPageSetup', 'setTableBorders', 'setCellFill',
    'setImageProperties', 'setImageWrapType', 'setNoteProperties', 'convertNote',
    'undo', 'redo', 'paste', 'toggleMark', 'setAlignment', 'setIndent', 'setLineSpacing', 'setParagraphSpacing', 'setSelection', 'clearFormatting',
    'insertTextBox', 'setEditingMode', 'editHeaderFooter', 'exitHeaderFooter', 'replaceMatch', 'selectAll', 'copy', 'cut', 'removeHyperlink'];
for (const type of types) assert.equal(WriterCommands.untrackable(type), legacy.test(type), type);

function harness(options = {}) {
    const log = {exec: [], table: [], notices: [], refreshed: 0, acted: []};
    const editor = {
        mode: options.mode || 'editing', refused: new Set(options.refused || []), active: new Set(options.active || []),
        snapshot() { return Object.assign({editingMode: this.mode}, options.snapshot); },
        can(command) { return this.refused.has(command.type) ? {ok: false, reason: 'engine'} : {ok: true}; },
        exec(command) { log.exec.push(command); return {ok: true}; },
        isActive(command) { return this.active.has(command.type); }
    };
    const slotTypes = {'history.undo': 'undo', 'history.redo': 'redo', 'text.bold': 'toggleBold', 'text.italic': 'toggleItalic',
        'insert.pageBreak': 'insertBreak', 'insert.endnote': 'insertNote', 'insert.toc': 'insertToc'};
    const lib = {
        commandForSlot: slot => ({type: slotTypes[slot] || slot}),
        commandForSlotValue: (slot, value) => ({type: 'value', slot, value}),
        tableCommand(target, command, dry) { log.table.push(command.type + ':' + !!dry); return {ok: true}; }
    };
    const state = {editor: options.noEditor ? null : editor, loading: !!options.loading};
    const commands = WriterCommands.create({
        tr: key => key, t: key => key, readonly: !!options.readonly, format: value => 'n' + value,
        editor: () => state.editor, lib: () => lib, loading: () => state.loading,
        act: id => { log.acted.push(id); return true; }, active: id => (options.open || []).includes(id),
        notice: (message, error) => log.notices.push({message, error}),
        refresh: () => { log.refreshed++; },
        clipboard: kind => options.clipboard ? options.clipboard[kind] : true
    });
    return {commands, editor, log, state};
}

{ // Without clipboard access the clipboard commands are refused with a reason, and nothing runs.
    const {commands, log} = harness({clipboard: {write: false, read: true}});
    assert.equal(commands.state('cut').enabled, false);
    assert.equal(commands.state('cut').reason, 'desktop.clipboard_write_unavailable');
    assert.equal(commands.state('copy').enabled, false);
    assert.equal(commands.state('paste').enabled, true);
    assert.equal(commands.menuItem('cut').disabledHint, 'desktop.clipboard_write_unavailable');
    assert.equal(commands.invoke('cut'), false);
    assert.equal(log.exec.length, 0);
    assert.equal(log.notices[0].message, 'command_unavailable · desktop.clipboard_write_unavailable');
    assert.equal(commands.invoke('selectAll'), true);
}
{ // A refused app action does not reach the app; it explains itself only when there is a reason.
    const {commands, log} = harness({readonly: true});
    assert.equal(commands.invoke('link'), false);
    assert.equal(commands.invoke('new'), false);
    assert.equal(log.acted.length, 0);
    assert.equal(log.notices.length, 0);
    const tracked = harness({mode: 'suggesting'});
    assert.equal(tracked.commands.invoke('table'), false);
    assert.equal(tracked.log.acted.length, 0);
    assert.equal(tracked.log.notices[0].message, 'command_unavailable · tracking_unsupported');
    assert.equal(tracked.commands.invoke('outline'), true);
}

{ // Suggesting mode refuses structural commands before the engine is asked.
    const {commands, log} = harness({mode: 'suggesting'});
    const refused = commands.can({type: 'insertTable'});
    assert.equal(refused.ok, false);
    assert.equal(refused.reason, 'tracking_unsupported');
    assert.equal(commands.can({type: 'undo'}).ok, true);
    assert.equal(commands.blocked('setPageSetup'), 'tracking_unsupported');
    assert.equal(commands.blocked('paste'), '');
    assert.equal(commands.tracking(), true);
    assert.equal(commands.run({type: 'insertTable'}), false);
    assert.equal(log.exec.length, 0);
    assert.equal(log.notices[0].message, 'command_unavailable · tracking_unsupported');
    assert.equal(log.notices[0].error, true);
    assert.equal(log.refreshed, 0);
}
{ // Merge and split go through the table operations, dry for can and live for run.
    const {commands, log} = harness();
    assert.equal(commands.can({type: 'mergeCells'}).ok, true);
    assert.equal(commands.run({type: 'splitCell', rows: 1, cols: 2}), true);
    assert.equal(commands.invoke('mergeCells'), true);
    assert.equal(log.table.join(), 'mergeCells:true,splitCell:true,splitCell:false,mergeCells:true,mergeCells:false');
    assert.equal(log.exec.length, 0);
    assert.equal(log.refreshed, 2);
}
{ // Nothing runs while a document loads, and nothing is announced.
    const {commands, log} = harness({loading: true});
    assert.equal(commands.run({type: 'undo'}), false);
    assert.equal(log.exec.length, 0);
    assert.equal(log.notices.length, 0);
}
{ // Without a document only document-free actions are enabled.
    const {commands} = harness({noEditor: true});
    assert.equal(commands.can({type: 'undo'}).ok, false);
    assert.equal(commands.tracking(), false);
    for (const id of ['save', 'bold', 'copy', 'paste', 'goToPage', 'wordCount', 'zoom100', 'table', 'link']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'shortcuts', 'outline']) assert.equal(commands.state(id).enabled, true, id);
    assert.equal(commands.menuItem('undo').disabled, true);
    assert.equal(commands.menuItem('print').disabled, true);
    assert.equal(commands.menuItem('open').disabled, false);
}
{ // State comes from the engine; only toggles report a pressed state.
    const {commands, log} = harness({refused: ['undo'], active: ['toggleBold', 'redo']});
    assert.equal(commands.state('undo').enabled, false);
    assert.equal(commands.state('undo').reason, 'engine');
    assert.equal(commands.state('bold').active, true);
    assert.equal(commands.state('italic').active, false);
    assert.equal(commands.state('redo').active, false);
    assert.equal(commands.invoke('bold'), true);
    assert.equal(log.exec[0].type, 'toggleBold');
    assert.equal(commands.invoke('outline'), true);
    assert.equal(log.acted[0], 'outline');
}
{ // An entry can name its engine command directly.
    const {commands, log} = harness({refused: ['cut']});
    assert.equal(commands.state('copy').enabled, true);
    assert.equal(commands.state('cut').enabled, false);
    assert.equal(commands.state('cut').reason, 'engine');
    assert.equal(commands.invoke('selectAll'), true);
    assert.equal(log.exec[0].type, 'selectAll');
    assert.equal(commands.invoke('rowAbove'), true);
    assert.equal(log.exec[1].type + ':' + log.exec[1].where, 'insertRow:above');
    assert.equal(commands.invoke('spacing150'), true);
    assert.equal(log.exec[2].rule + ':' + log.exec[2].value, 'multiple:1.5');
    assert.equal(log.acted.length, 0);
}
{ // Structural entries are refused while changes are tracked.
    const {commands} = harness({mode: 'suggesting'});
    for (const id of ['rowAbove', 'deleteTable', 'pageBreak', 'endnote', 'toc']) assert.equal(commands.state(id).reason, 'tracking_unsupported', id);
    for (const id of ['copy', 'selectAll', 'superscript', 'marks', 'spacing115']) assert.equal(commands.state(id).enabled, true, id);
}
{ // A checked state comes from the snapshot, or from the app for its own actions.
    const {commands} = harness({snapshot: {showParagraphMarks: true, reviewDisplayMode: 'proposed', formatting: {lineSpacing: {rule: 'multiple', value: 1.5}}}, open: ['outline']});
    assert.equal(commands.state('marks').active, true);
    assert.equal(commands.state('markupFinal').active, true);
    assert.equal(commands.state('markupAll').active, false);
    assert.equal(commands.state('spacing150').active, true);
    assert.equal(commands.state('spacing100').active, false);
    assert.equal(commands.state('outline').active, true);
    assert.equal(commands.state('review').active, false);
    assert.equal(commands.menuItem('marks').checked, true);
    assert.equal(commands.menuItem('outline').checked, true);
    assert.equal(commands.menuItem('markupAll').checked, undefined);
}
{ // Without a display mode in the snapshot, all changes are shown.
    const {commands} = harness();
    assert.equal(commands.state('markupAll').active, true);
    assert.equal(commands.state('marks').active, false);
}
{ // Read-only windows keep reading actions.
    const {commands} = harness({readonly: true});
    for (const id of ['new', 'save', 'saveAs', 'image', 'qr', 'paste', 'pastePlain', 'table', 'link', 'tracking']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'print', 'docx', 'search', 'goToPage', 'wordCount', 'zoomPage', 'shortcuts']) assert.equal(commands.state(id).enabled, true, id);
}
{ // An action can name the command type that tracking refuses.
    const {commands} = harness({mode: 'suggesting'});
    for (const id of ['qr', 'table', 'link']) {
        assert.equal(commands.state(id).enabled, false, id);
        assert.equal(commands.state(id).reason, 'tracking_unsupported', id);
    }
    assert.equal(commands.state('image').enabled, true);
    assert.equal(commands.state('paste').enabled, true);
}
{ // Shortcuts match the exact modifier set.
    const {commands} = harness();
    const key = (name, mods = {}) => commands.actionForKey({key: name, ctrlKey: true, metaKey: false, shiftKey: false, altKey: false, ...mods});
    assert.equal(key('s'), 'save');
    assert.equal(key('S', {shiftKey: true}), 'saveAs');
    assert.equal(key('f'), 'search');
    assert.equal(key('h'), 'search');
    assert.equal(key('n'), 'new');
    assert.equal(key('o'), 'open');
    assert.equal(key('p'), 'print');
    assert.equal(key('k'), 'link');
    assert.equal(key('g'), 'goToPage');
    assert.equal(key('s', {ctrlKey: false, metaKey: true}), 'save', 'Cmd counts as Ctrl');
    assert.equal(key('s', {altKey: true}), '', 'AltGr letters are text, not shortcuts');
    assert.equal(key('o', {shiftKey: true}), '');
    assert.equal(key('s', {ctrlKey: false}), '');
    for (const name of ['z', 'c', 'v', 'x', 'a']) assert.equal(key(name), '', 'the engine owns Ctrl+' + name);
}
{ // Labels, tooltips and menu items.
    const {commands, log} = harness({active: ['toggleBold']});
    assert.equal(commands.label('saveAs'), 'save_as');
    assert.equal(commands.label('qr'), 'desktop.qr_insert');
    assert.equal(commands.label('zoom125'), '125%');
    assert.equal(commands.label('spacing115'), 'n1.15');
    assert.equal(commands.format(1.5), 'n1.5');
    assert.equal(commands.tooltip('save'), 'save (Ctrl+S)');
    assert.equal(commands.tooltip('copy'), 'copy (Ctrl+C)');
    assert.equal(commands.tooltip('bold'), 'bold (Ctrl+B)');
    assert.equal(commands.tooltip('strike'), 'strike');
    const item = commands.menuItem('print');
    assert.equal(item.id, 'print');
    assert.equal(item.label, 'print');
    assert.equal(item.icon, 'printer');
    assert.equal(item.shortcut, 'Ctrl+P');
    assert.equal(item.shortcutHint, undefined);
    assert.equal(item.disabled, false);
    item.action();
    assert.equal(log.acted[0], 'print');
    assert.equal(commands.menuItem('undo').disabled, false);
    assert.equal(commands.menuItem('search').icon, 'search');
    // The shell dispatches `shortcut` itself; clipboard keys are only shown.
    assert.equal(commands.menuItem('copy').shortcut, undefined);
    assert.equal(commands.menuItem('copy').shortcutHint, 'Ctrl+C');
    // A glyph is drawn by the toolbar only, and a pressed toggle is not a check mark.
    assert.equal(commands.menuItem('alignLeft').icon, '');
    assert.equal(commands.menuItem('bold').checked, undefined);
}
{ // The engine wants a bare hex, a palette name, half-points and a number of lines.
    const colors = WriterCommands.colors;
    assert.equal(colors.hex('#AABBCC'), '#aabbcc');
    assert.equal(colors.hex('aabbcc'), '#aabbcc');
    assert.equal(colors.hex({value: 'FF0000'}), '#ff0000');
    assert.equal(colors.hex({resolvedHex: '00ff00'}), '#00ff00');
    assert.equal(colors.hex('red'), '');
    assert.equal(colors.hex(null), '');
    assert.equal(colors.highlightName('#fffe00'), 'yellow');
    assert.equal(colors.highlightName('#000081'), 'darkBlue');
    assert.equal(colors.highlightName('nope'), '');
    assert.equal(colors.highlightHex('yellow'), '#ffff00');
    assert.equal(colors.highlightHex('#123456'), '#123456');
    const {commands} = harness();
    assert.equal(commands.valueCommand('text.color', '#AABBCC').value, 'aabbcc');
    assert.equal(commands.valueCommand('text.highlight', '#fffe00').value, 'yellow');
    assert.equal(commands.valueCommand('font.size', 11.5).value, 23);
    assert.equal(commands.valueCommand('font.family', 'Carlito').value, 'Carlito');
    assert.equal(commands.valueCommand('list.lineSpacing', '1.5').value, 1.5);
    assert.equal(commands.valueCommand('list.lineSpacing', '').value, 1, 'an unknown spacing still probes with a valid value');
}

{ // A command can belong to one editing scope; outside it the command is off.
    const body = harness();
    assert.equal(body.commands.state('headerFirstPage').enabled, false);
    assert.equal(body.commands.invoke('headerFirstPage'), false);
    assert.equal(body.log.acted.length, 0);
    const header = harness({snapshot: {scope: {kind: 'headerFooter'}}, open: ['headerFirstPage']});
    assert.equal(header.commands.state('headerFirstPage').enabled, true);
    assert.equal(header.commands.state('headerFirstPage').active, true);
    assert.equal(header.commands.state('headerOddEven').active, false);
    assert.equal(header.commands.invoke('headerOddEven'), true);
    assert.equal(header.log.acted[0], 'headerOddEven');
    assert.equal(harness({snapshot: {scope: {kind: 'headerFooter'}}, readonly: true}).commands.state('headerFirstPage').enabled, false);
}
{ // The Wave 1b engine commands carry their exact shapes.
    const {commands, log} = harness();
    for (const id of ['rotateLeft', 'rotateRight', 'deleteImage', 'borders', 'noBorders']) assert.equal(commands.invoke(id), true, id);
    assert.equal(log.exec.map(command => command.type + ':' + (command.action || command.scope || '')).join(), 'transformImage:rotateCCW,transformImage:rotateCW,deleteImage:,setTableBorders:all,setTableBorders:none');
    assert.equal(log.exec[3].spec.style, 'single');
    assert.equal(log.exec[4].target, 'all');
    assert.equal(commands.state('comment').enabled, true);
    assert.equal(harness({readonly: true}).commands.state('comment').enabled, false);
    for (const id of ['rowAbove', 'rowBelow', 'columnLeft', 'columnRight', 'deleteRow', 'deleteColumn', 'deleteTable', 'mergeCells', 'splitCell', 'borders', 'noBorders', 'rotateLeft', 'rotateRight'])
        assert.ok(commands.get(id).glyph, id + ' has a glyph for the contextual bar');
}
// The table itself.
const definitions = WriterCommands.definitions, ids = definitions.map(definition => definition.id);
assert.equal(new Set(ids).size, ids.length, 'ids are unique');
const bound = definitions.flatMap(definition => definition.keys || []);
assert.equal(new Set(bound).size, bound.length, 'no key is bound twice');
for (const definition of definitions) {
    assert.ok(definition.label || definition.fullLabel || definition.text || definition.number !== undefined, definition.id + ' has a label');
    assert.ok(!(definition.slot && definition.command), definition.id + ' is a slot or a command, not both');
    assert.ok(definition.slot || definition.command || !definition.toggle, definition.id + ': only engine commands toggle');
    assert.ok(!(definition.shortcut && definition.hint), definition.id + ' shows one shortcut');
    if (definition.keys) assert.ok(definition.shortcut, definition.id + ' shows the shortcut it binds');
    if (definition.options) assert.equal(definition.control, 'select', definition.id + ': only a select has options');
    if (definition.scope) assert.ok(!definition.slot && !definition.command, definition.id + ': only app actions are scoped');
}

// Every label of the table and every literal tr('key') of the Writer files exists in all sixteen languages.
const used = new Set(definitions.filter(definition => definition.fullLabel || definition.label).map(definition => definition.fullLabel || 'desktop.writer_' + definition.label));
for (const file of readdirSync('ui/js/desktop/apps').filter(name => /^writer(-[a-z]+)?\.js$/.test(name)))
    for (const match of readFileSync('ui/js/desktop/apps/' + file, 'utf8').matchAll(/\btr\('([a-z0-9_]+)'[,)]/g)) used.add('desktop.writer_' + match[1]);
const languages = readdirSync('ui/lang/desktop').filter(name => name.endsWith('.json'));
assert.equal(languages.length, 16);
for (const file of languages) {
    const labels = JSON.parse(readFileSync('ui/lang/desktop/' + file, 'utf8'));
    for (const key of used) assert.ok(labels[key], file + ' lacks ' + key);
}
console.log('Writer commands: tracking rule, gate, state, shortcuts, menu items, colours and ' + used.size + ' language keys pass');
