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
    'insertTextBox', 'setEditingMode', 'editHeaderFooter', 'exitHeaderFooter', 'replaceMatch'];
for (const type of types) assert.equal(WriterCommands.untrackable(type), legacy.test(type), type);

function harness(options = {}) {
    const log = {exec: [], table: [], notices: [], refreshed: 0, acted: []};
    const editor = {
        mode: options.mode || 'editing', refused: new Set(options.refused || []), active: new Set(options.active || []),
        snapshot() { return {editingMode: this.mode}; },
        can(command) { return this.refused.has(command.type) ? {ok: false, reason: 'engine'} : {ok: true}; },
        exec(command) { log.exec.push(command); return {ok: true}; },
        isActive(command) { return this.active.has(command.type); }
    };
    const slotTypes = {'history.undo': 'undo', 'history.redo': 'redo', 'text.bold': 'toggleBold', 'text.italic': 'toggleItalic'};
    const lib = {
        commandForSlot: slot => ({type: slotTypes[slot] || slot}),
        commandForSlotValue: (slot, value) => ({type: 'value', slot, value}),
        tableCommand(target, command, dry) { log.table.push(command.type + ':' + !!dry); return {ok: true}; }
    };
    const state = {editor: options.noEditor ? null : editor, loading: !!options.loading};
    const commands = WriterCommands.create({
        tr: key => key, t: key => key, readonly: !!options.readonly,
        editor: () => state.editor, lib: () => lib, loading: () => state.loading,
        act: id => { log.acted.push(id); return true; },
        notice: (message, error) => log.notices.push({message, error}),
        refresh: () => { log.refreshed++; }
    });
    return {commands, editor, log, state};
}

{ // Suggesting mode refuses structural commands before the engine is asked.
    const {commands, log} = harness({mode: 'suggesting'});
    const refused = commands.can({type: 'insertTable'});
    assert.equal(refused.ok, false);
    assert.equal(refused.reason, 'tracking_unsupported');
    assert.equal(commands.can({type: 'undo'}).ok, true);
    assert.equal(commands.blocked('setPageSetup'), 'tracking_unsupported');
    assert.equal(commands.blocked('paste'), '');
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
    assert.equal(log.table.join(), 'mergeCells:true,splitCell:true,splitCell:false');
    assert.equal(log.exec.length, 0);
    assert.equal(log.refreshed, 1);
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
    assert.equal(commands.state('save').enabled, false);
    assert.equal(commands.state('bold').enabled, false);
    assert.equal(commands.state('open').enabled, true);
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
{ // Read-only windows keep reading actions.
    const {commands} = harness({readonly: true});
    for (const id of ['new', 'save', 'saveAs', 'image', 'qr']) assert.equal(commands.state(id).enabled, false, id);
    for (const id of ['open', 'print', 'docx', 'search']) assert.equal(commands.state(id).enabled, true, id);
}
{ // An action can name the command type that tracking refuses.
    const {commands} = harness({mode: 'suggesting'});
    assert.equal(commands.state('qr').enabled, false);
    assert.equal(commands.state('qr').reason, 'tracking_unsupported');
    assert.equal(commands.state('image').enabled, true);
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
    assert.equal(key('s', {ctrlKey: false, metaKey: true}), 'save', 'Cmd counts as Ctrl');
    assert.equal(key('s', {altKey: true}), '', 'AltGr letters are text, not shortcuts');
    assert.equal(key('o', {shiftKey: true}), '');
    assert.equal(key('s', {ctrlKey: false}), '');
    assert.equal(key('z'), '', 'the engine owns undo');
}
{ // Labels, tooltips and menu items.
    const {commands, log} = harness();
    assert.equal(commands.label('saveAs'), 'save_as');
    assert.equal(commands.label('qr'), 'desktop.qr_insert');
    assert.equal(commands.tooltip('save'), 'save (Ctrl+S)');
    assert.equal(commands.tooltip('bold'), 'bold');
    const item = commands.menuItem('print');
    assert.equal(item.id, 'print');
    assert.equal(item.label, 'print');
    assert.equal(item.icon, 'printer');
    assert.equal(item.shortcut, 'Ctrl+P');
    assert.equal(item.disabled, false);
    item.action();
    assert.equal(log.acted[0], 'print');
    assert.equal(commands.menuItem('undo').disabled, false, 'menus are built once, so a slot item only needs a document');
    assert.equal(commands.menuItem('search').icon, 'search');
}
{ // The engine wants a bare hex, a palette name and half-points.
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
}

// The table itself.
const definitions = WriterCommands.definitions, ids = definitions.map(definition => definition.id);
assert.equal(new Set(ids).size, ids.length, 'ids are unique');
const bound = definitions.flatMap(definition => definition.keys || []);
assert.equal(new Set(bound).size, bound.length, 'no key is bound twice');
for (const definition of definitions) {
    assert.ok(definition.label || definition.fullLabel, definition.id + ' has a label');
    assert.ok(definition.slot || !definition.toggle, definition.id + ': only engine slots toggle');
    if (definition.keys) assert.ok(definition.shortcut, definition.id + ' shows the shortcut it binds');
}

// Every label of the table and every literal tr('key') of the Writer files exists in all sixteen languages.
const used = new Set(definitions.map(definition => definition.fullLabel || 'desktop.writer_' + definition.label));
for (const file of readdirSync('ui/js/desktop/apps').filter(name => /^writer(-[a-z]+)?\.js$/.test(name)))
    for (const match of readFileSync('ui/js/desktop/apps/' + file, 'utf8').matchAll(/\btr\('([a-z0-9_]+)'[,)]/g)) used.add('desktop.writer_' + match[1]);
const languages = readdirSync('ui/lang/desktop').filter(name => name.endsWith('.json'));
assert.equal(languages.length, 16);
for (const file of languages) {
    const labels = JSON.parse(readFileSync('ui/lang/desktop/' + file, 'utf8'));
    for (const key of used) assert.ok(labels[key], file + ' lacks ' + key);
}
console.log('Writer commands: tracking rule, gate, state, shortcuts, menu items, colours and ' + used.size + ' language keys pass');
