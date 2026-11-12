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
for (const id of placed) assert.ok(known.get(id).control || known.get(id).icon, id + ' needs an icon');
const groupIds = WriterToolbar.groups.map(group => group.id);
assert.equal(new Set(groupIds).size, groupIds.length, 'group ids are unique');
assert.equal(new Set(WriterToolbar.groups.map(group => group.priority)).size, groupIds.length, 'priorities are distinct, so the overflow order is defined');

// Glyphs the shared atlases lack are drawn locally; everything else asks the shell.
const icon = WriterToolbar.icons({iconMarkup: (key, fallback) => '<i>' + key + '|' + fallback + '</i>'}, value => String(value));
assert.ok(icon('align-left', '≡').startsWith('<svg class="writer-glyph"'));
assert.equal(icon('undo', '↶'), '<i>undo|↶</i>');
assert.equal(WriterToolbar.icons({}, value => String(value))('undo', '↶'), '↶');
console.log('Writer toolbar: groups and glyphs pass');
