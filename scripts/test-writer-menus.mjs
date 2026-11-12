import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-commands.js', 'utf8'), context);
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-menus.js', 'utf8'), context);
const {WriterCommands, WriterMenus} = context.window;
const flat = items => items.flatMap(item => item.items ? flat(item.items) : [item]);

// The model names registry commands only, and every command is reachable from a menu or the toolbar.
const used = [];
const menus = WriterMenus.build(id => { used.push(id); return {id}; }, key => 'tr:' + key, {file: [{id: 'cloud'}]});
assert.equal(menus.map(menu => menu.id).join(), 'file,edit,view,insert,format,table,tools,agent');
const known = new Set(WriterCommands.definitions.map(definition => definition.id));
for (const id of used) assert.ok(known.has(id), 'menu item ' + id + ' is not a registry command');
const absent = WriterCommands.definitions.map(definition => definition.id).filter(id => !used.includes(id));
assert.equal(absent.join(), 'style,fontFamily,fontSize,lineSpacing,textColor,highlight,insert', 'only toolbar controls and the insert panel stay out of the menus');
assert.ok(flat(menus[0].items).some(item => item.id === 'cloud'), 'the app-supplied file items are placed');
for (const menu of menus) {
    assert.ok(menu.labelKey || String(menu.label).startsWith('tr:'), menu.id + ' has a title');
    const ids = flat(menu.items).filter(item => item.id).map(item => item.id);
    assert.equal(new Set(ids).size, ids.length, menu.id + ' lists no command twice');
    for (const item of menu.items) if (item.items) assert.ok(String(item.label).startsWith('tr:'), item.id + ' submenu has a title');
}
assert.equal(menus.at(-1).items[0].id, 'assist');

// The bar is rebuilt only when the signature changes, and an engine command hands the focus back.
const log = {set: 0, focused: 0, ran: []};
let signature = 'a';
const app = {
    windowId: 'w1', tr: key => key, editor: {focus() { log.focused++; }},
    ctx: {setWindowMenus(id, model) { log.set++; log.window = id; log.model = model; }},
    commands: {
        get: id => WriterCommands.definitions.find(definition => definition.id === id),
        menuItem: id => ({id, action: () => log.ran.push(id)})
    },
    menuSignature: () => signature, fileMenuItems: () => []
};
const bar = WriterMenus.create(app);
bar.sync();
bar.sync();
assert.equal(log.set, 1);
assert.equal(log.window, 'w1');
signature = 'b';
bar.sync();
assert.equal(log.set, 2);
bar.sync(true);
assert.equal(log.set, 3, 'a forced sync always rebuilds');
const find = id => flat(log.model.flatMap(menu => menu.items)).find(item => item.id === id);
find('bold').action();
assert.equal(log.ran.join(), 'bold');
assert.equal(log.focused, 1, 'an engine command returns the focus to the document');
find('copy').action();
assert.equal(log.focused, 2);
find('wordCount').action();
assert.equal(log.focused, 2, 'an app action keeps the focus where its dialog puts it');
console.log('Writer menus: model, coverage and rebuild rule pass');
