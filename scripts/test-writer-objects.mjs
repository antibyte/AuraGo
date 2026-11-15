import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import vm from 'node:vm';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-objects.js', 'utf8'), context);
const O = context.window.WriterObjects;
const same = (actual, expected, message) => assert.equal(JSON.stringify(actual), JSON.stringify(expected), message);

// The pen: what a new window draws with, and what stays of a value nobody can draw with.
same(O.pen(), {style: 'single', size: 4, color: '64748B'});
same(O.pen({style: 'double', size: 12, color: '#ff0000'}), {style: 'double', size: 12, color: 'FF0000'});
same(O.pen({style: 'wave', size: 5, color: 'red'}), {style: 'single', size: 4, color: '64748B'}, 'unknown values fall back');
same(O.pen({style: 'dashed'}).style, 'single', 'the engine does not paint a dashed left edge, so the pen has no dashed line');
same(O.pen({style: 'thick', size: '24', color: 'abcdef'}), {style: 'thick', size: 24, color: 'ABCDEF'});
same(O.spec(O.pen({color: '#0000ff'})), {style: 'single', size: 4, color: {kind: 'hex', value: '0000FF'}});
same(O.styles, ['single', 'double', 'triple', 'thick']);
same(O.widths, [4, 6, 8, 12, 18, 24]);
same(O.edges, ['all', 'outside', 'inside', 'top', 'bottom', 'left', 'right', 'none']);
assert.equal(O.widthLabel(4, value => String(value).replace('.', ',')), '0,5 pt');
assert.equal(O.widthLabel(18, String), '2.25 pt');
same(O.edges.map(O.edgeLabel), ['borders', 'border_outside', 'border_inside', 'margin_top', 'margin_bottom', 'margin_left', 'margin_right', 'no_borders']);

// Border commands: one per edge button; none needs no pen.
const pen = O.pen({style: 'triple', size: 8, color: '00AA00'});
same(O.borderCommand('outside', pen), {type: 'setTableBorders', scope: 'outside', spec: {style: 'triple', size: 8, color: {kind: 'hex', value: '00AA00'}}});
same(O.borderCommand('all', pen).scope, 'all');
same(O.borderCommand('none', pen), {type: 'setTableBorders', scope: 'none', target: 'all'});
assert.equal(O.borderCommand('diagonal', pen), null);

// A picture's size: locked keeps the proportions, unlocked changes one side.
const picture = {widthEmu: 1905000, heightEmu: 952500};
same(O.sizeCommand(picture, 'width', 952500, true), {type: 'setImageProperties', widthEmu: 952500, heightEmu: 476250});
same(O.sizeCommand(picture, 'height', 1905000, true), {type: 'setImageProperties', widthEmu: 3810000, heightEmu: 1905000});
same(O.sizeCommand(picture, 'width', 952500, false), {type: 'setImageProperties', widthEmu: 952500});
same(O.sizeCommand(picture, 'height', 635000.4, false), {type: 'setImageProperties', heightEmu: 635000});
assert.equal(O.sizeCommand(picture, 'width', 0, true), null);
assert.equal(O.sizeCommand(picture, 'width', NaN, true), null);
assert.equal(O.sizeCommand({widthEmu: 0, heightEmu: 10}, 'width', 100, true), null, 'no proportions without a size');
assert.equal(O.fitHeight(1905000, 120, 120), 1905000);
assert.equal(O.fitHeight(1905000, 200, 100), 952500);

// Crop: percent of the picture; something of it has to stay.
same(O.crop({left: 10, top: 20, right: 30, bottom: 0}), {command: {type: 'setImageProperties', crop: {left: 10, top: 20, right: 30, bottom: 0}}});
same(O.crop({left: 0, top: 0, right: 0, bottom: 0}).command.crop, {left: 0, top: 0, right: 0, bottom: 0});
for (const bad of [{left: -1, top: 0, right: 0, bottom: 0}, {left: 100, top: 0, right: 0, bottom: 0}, {left: 60, top: 0, right: 40, bottom: 0}, {left: 0, top: 50, right: 0, bottom: 50},
    {left: NaN, top: 0, right: 0, bottom: 0}, {left: 0, top: 0, right: 0}]) same(O.crop(bad), {error: 'crop_invalid'}, JSON.stringify(bad));

console.log('Writer objects: border pen, border commands, picture size and crop pass');
