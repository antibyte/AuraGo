import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-commands.js', 'utf8'), context);
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-toolbar.js', 'utf8'), context);
const {WriterCommands, WriterToolbar} = context.window;

// Every toolbar item is a registry command, placed once.
const known = new Map(WriterCommands.definitions.map(definition => [definition.id, definition]));
const placed = WriterToolbar.groups.flatMap(group => group.items);
assert.equal(new Set(placed).size, placed.length, 'no command sits in two groups');
for (const id of placed) assert.ok(known.has(id), 'toolbar item ' + id + ' is not a registry command');
for (const id of placed) assert.ok(known.get(id).control || known.get(id).icon || known.get(id).glyph,id + ' needs an icon');
const groupIds = WriterToolbar.groups.map(group => group.id);
assert.equal(new Set(groupIds).size, groupIds.length, 'group ids are unique');
assert.equal(new Set(WriterToolbar.groups.map(group => group.priority)).size, groupIds.length, 'priorities are distinct, so the overflow order is defined');

// Glyphs the shared atlases lack are drawn locally; everything else asks the shell.
const icon = WriterToolbar.icons({iconMarkup: (key, fallback) => '<i>' + key + '|' + fallback + '</i>'}, value => String(value));
assert.ok(icon('align-left', '≡').startsWith('<svg class="writer-glyph"'));
assert.equal(icon('undo', '↶'), '<i>undo|↶</i>');
assert.equal(WriterToolbar.icons({}, value => String(value))('undo', '↶'), '↶');
// Wave 1a additions sit in the bar, and their glyphs are drawn locally.
for (const id of ['clearFormat', 'lineSpacing', 'link', 'table', 'image']) assert.ok(placed.includes(id), id + ' is in the toolbar');
for (const key of ['table', 'clear-format']) assert.ok(icon(key, '?').startsWith('<svg class="writer-glyph"'), key + ' is a local glyph');
for (const definition of WriterCommands.definitions) if (definition.glyph) assert.ok(icon(definition.glyph, '?').startsWith('<svg class="writer-glyph"'), definition.glyph + ' is drawn');
// The overflow rule: the lowest priority leaves first, the bar always keeps one group,
// and the overflow button's width counts once something has left.
const bar = [{id: 'history', priority: 9, width: 70}, {id: 'fonts', priority: 8, width: 300}, {id: 'marks', priority: 7, width: 130},
    {id: 'alignment', priority: 4, width: 130}, {id: 'lists', priority: 3, width: 130}, {id: 'colors', priority: 5, width: 120}, {id: 'insert', priority: 6, width: 30}];
const fit = available => WriterToolbar.fit(bar, available, 30, 7).join();
assert.equal(fit(1000), '');
assert.equal(fit(952), '', 'an exact fit keeps everything');
assert.equal(fit(951), 'lists');
assert.equal(fit(700), 'lists,alignment,colors');
assert.equal(fit(420), 'lists,alignment,colors,insert,marks');
assert.equal(fit(100), 'lists,alignment,colors,insert,marks,fonts', 'one group always stays');
console.log('Writer toolbar: groups, glyphs and overflow fit pass');
