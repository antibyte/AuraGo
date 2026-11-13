import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-commands.js', 'utf8'), context);
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-context.js', 'utf8'), context);
const {WriterCommands, WriterContext} = context.window;
const known = new Set(WriterCommands.definitions.map(definition => definition.id));

// What the caret is in decides the bar: an image before a header or footer, before a table.
const {kind} = WriterContext;
assert.equal(kind(null), '');
assert.equal(kind({scope: {kind: 'body'}}), '');
assert.equal(kind({scope: {kind: 'body'}, table: {rows: 2}}), 'table');
assert.equal(kind({scope: {kind: 'headerFooter'}, table: {rows: 2}}), 'headerFooter');
assert.equal(kind({scope: {kind: 'headerFooter'}, table: {rows: 2}, image: {id: 'i'}}), 'image');
assert.equal(kind({scope: {kind: 'note'}}), '');

// Every bar names registry commands; the ones drawn as text are part of the bar.
assert.equal(Object.keys(WriterContext.bars).join(), 'table,image,headerFooter');
for (const [name, bar] of Object.entries(WriterContext.bars)) {
    const ids = bar.items.filter(id => id !== '|');
    assert.equal(new Set(ids).size, ids.length, name + ' lists no command twice');
    for (const id of ids) assert.ok(known.has(id), name + ' bar item ' + id + ' is not a registry command');
    for (const id of bar.text || []) assert.ok(ids.includes(id), name + ' draws ' + id + ' as text but does not hold it');
    assert.notEqual(bar.items[0], '|');
    assert.notEqual(bar.items.at(-1), '|');
}
for (const id of WriterContext.bars.table.items.filter(id => id !== '|')) assert.ok(WriterCommands.definitions.find(definition => definition.id === id).glyph, 'table bar item ' + id + ' needs a glyph');

// The right-click menu: sections of registry ids, '@' names for items only the app can build, one submenu.
const flatten = sections => sections.flat().flatMap(entry => typeof entry === 'string' ? [entry] : entry.items.filter(id => id !== '|'));
const custom = new Set(['@accept', '@reject', '@unlink', '@updateToc']);
const plain = WriterContext.sections({review: '', link: false, table: false, image: false, toc: '', scope: 'body'});
assert.equal(plain.filter(section => section.length).map(section => section.join()).join(' / '), 'cut,copy,paste,pastePlain / link / comment / paragraphFormat / assist');
assert.equal(WriterContext.bars.headerFooter.items.join(), 'fieldPage,pageNumber,fieldPages,insertDate,|,headerFirstPage,headerOddEven,|,backDocument');
const everything = WriterContext.sections({review: 'r1', link: true, table: true, image: true, toc: 't1', scope: 'headerFooter'});
for (const entry of flatten(everything)) assert.ok(entry.startsWith('@') ? custom.has(entry) : known.has(entry), 'menu entry ' + entry + ' is unknown');
assert.equal(everything[0].join(), '@accept,@reject', 'a change under the pointer comes first');
assert.ok(flatten(everything).includes('@unlink'));
assert.ok(flatten(everything).includes('@updateToc'));
assert.ok(flatten(everything).includes('backDocument'));
assert.ok(flatten(everything).includes('rotateRight'));
const submenu = everything.flat().find(entry => typeof entry !== 'string');
assert.equal(submenu.submenu, 'table');
assert.equal(submenu.items.join(), WriterContext.bars.table.items.join(), 'the table submenu is the table bar');
const noImage = WriterContext.sections({review: '', link: false, table: false, image: false, toc: '', scope: 'body'});
assert.ok(!flatten(noImage).includes('rotateRight'), 'image commands appear for a selected image only');
assert.ok(flatten(everything).includes('deleteImage'));
console.log('Writer context: bar kinds, bar contents and right-click sections pass');
