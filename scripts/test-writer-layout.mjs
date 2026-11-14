import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-layout.js', 'utf8'), context);
const L = context.window.WriterLayout, same = (actual, expected, message) => assert.equal(JSON.stringify(actual), JSON.stringify(expected), message);

// Units: centimetres, or inches for an English interface in a US browser; comma or point on input.
const cm = L.units('de', 'de-DE'), inch = L.units('en', 'en-US');
assert.equal(cm.unit, 'cm');
assert.equal(inch.unit, 'in');
assert.equal(L.units('en', 'en-GB').unit, 'cm');
assert.equal(L.units('de', 'en-US').unit, 'cm');
assert.equal(cm.show(1417), '2,5');
assert.equal(cm.read('2,5'), 1417);
assert.equal(cm.read(' 2.5 cm '), 1417);
assert.equal(cm.read(''), null);
assert.ok(Number.isNaN(cm.read('wide')));
assert.ok(Number.isNaN(cm.read('25 mm')), 'another unit is not read as centimetres');
assert.equal(cm.read('−0,5'), -283, 'a typographic minus is a minus');
assert.equal(cm.show(null), '');
assert.equal(cm.show(''), '');
assert.equal(inch.show(1440), '1');
assert.equal(inch.read('8.5'), 12240);
assert.equal(inch.read('8.5 in'), 12240);
assert.equal(cm.number(1.15), '1,15');
assert.equal(cm.points('12,5 pt'), 12.5);
assert.equal(cm.points(''), null);

// Papers.
assert.equal(L.papers.map(paper => paper.id).join(), 'a4,a5,a3,b5,letter,legal');
assert.equal(L.paperOf(12240, 15840), 'letter');
assert.equal(L.paperOf(15840, 12240), 'letter');
assert.equal(L.paperOf(12259, 15840), 'letter');
assert.equal(L.paperOf(12260, 15840), 'custom');
assert.equal(L.paperOf(11906, 16838), 'a4');
assert.equal(L.paperOf(16838, 11906), 'a4');
assert.equal(L.paperOf(11925, 16838), 'a4');
assert.equal(L.paperOf(8391, 11906), 'a5');
assert.equal(L.paperOf(9000, 9000), 'custom');

// The Page setup dialog: fields, dependent fields, and a command that holds only what changed.
const a4 = {pageWidthTwips: 11906, pageHeightTwips: 16838, orientation: 'portrait', marginsTwips: {top: 1417, right: 1417, bottom: 1417, left: 1417}};
const page = L.pageFields(a4, cm);
same(page, {paper: 'a4', width: '21', height: '29,7', orientation: 'portrait', preset: 'normal', top: '2,5', right: '2,5', bottom: '2,5', left: '2,5', scope: 'document'});
same(L.pageInput('paper', {...page, paper: 'a5'}, cm), {width: '14,8', height: '21'});
same(L.pageInput('paper', {...page, paper: 'a5', orientation: 'landscape'}, cm), {width: '21', height: '14,8'});
same(L.pageInput('paper', {...page, paper: 'custom'}, cm), {});
same(L.pageInput('orientation', {...page, orientation: 'landscape'}, cm), {width: '29,7', height: '21'});
same(L.pageInput('orientation', {...page, orientation: 'portrait'}, cm), {});
same(L.pageInput('width', {...page, width: '29,7', height: '21'}, cm), {paper: 'a4', orientation: 'landscape'});
same(L.pageInput('height', {...page, height: '30'}, cm), {paper: 'custom', orientation: 'portrait'});
same(L.pageInput('width', {...page, width: 'x'}, cm), {});
same(L.pageInput('preset', {...page, preset: 'narrow'}, cm), {top: '1,27', right: '1,27', bottom: '1,27', left: '1,27'});
same(L.pageInput('preset', {...page, preset: 'custom'}, cm), {});
same(L.pageInput('left', {...page, left: '3'}, cm), {preset: 'custom'});
same(L.pageInput('left', {...page, left: '2,5'}, cm), {preset: 'normal'});
same(L.pageInput('scope', {...page, scope: 'section'}, cm), {});
same(L.pageCommand(page, a4, cm), {command: null});
same(L.pageCommand({...page, paper: 'a5', width: '14,8', height: '21'}, a4, cm), {command: {type: 'setPageSetup', pageWidth: 8391, pageHeight: 11906, orientation: 'portrait', scope: 'document'}});
same(L.pageCommand({...page, width: '29,7', height: '21', orientation: 'landscape'}, a4, cm), {command: {type: 'setPageSetup', pageWidth: 16838, pageHeight: 11906, orientation: 'landscape', scope: 'document'}});
same(L.pageCommand({...page, left: '3', scope: 'section'}, a4, cm), {command: {type: 'setPageSetup', marginLeft: 1701, scope: 'section'}});
same(L.pageCommand({...page, scope: 'section'}, a4, cm), {command: null});
same(L.pageCommand({...page, top: 'x'}, a4, cm), {error: 'length_invalid'});
same(L.pageCommand({...page, top: ''}, a4, cm), {error: 'length_invalid'});
same(L.pageCommand({...page, top: '-1'}, a4, cm), {error: 'length_invalid'});
same(L.pageCommand({...page, width: '1'}, a4, cm), {error: 'page_size_range'});
same(L.pageCommand({...page, left: '10', right: '10'}, a4, cm), {error: 'margins_too_large'});
// A retyped value that still shows the same is no change, and an untouched side of an odd page keeps its exact size.
same(L.pageCommand({...page, left: '2.50', width: '21,0'}, a4, cm), {command: null});
const odd = {...a4, pageWidthTwips: 11890}, oddFields = L.pageFields(odd, cm);
assert.equal(oddFields.width, '20,97');
same(L.pageCommand({...oddFields, height: '25'}, odd, cm).command, {type: 'setPageSetup', pageWidth: 11890, pageHeight: 14173, orientation: 'portrait', scope: 'document'});
same(L.pageCommand({...oddFields, ...L.pageInput('orientation', {...oddFields, orientation: 'landscape'}, cm), orientation: 'landscape'}, odd, cm).command,
    {type: 'setPageSetup', pageWidth: 16838, pageHeight: 11890, orientation: 'landscape', scope: 'document'});
// In inches a paper keeps its exact size although the text is rounded to two places.
const inches = L.pageFields(a4, inch);
assert.equal(inches.width, '8.27');
same(L.pageCommand({...inches, ...L.pageInput('paper', {...inches, paper: 'a5'}, inch), paper: 'a5'}, a4, inch).command,
    {type: 'setPageSetup', pageWidth: 8391, pageHeight: 11906, orientation: 'portrait', scope: 'document'});

// The Paragraph dialog.
const plain = {alignment: 'left', lineSpacing: {rule: 'multiple', value: 1.08}, spaceAfterPt: 8,
    paragraphFlags: {contextualSpacing: false, keepNext: false, keepLines: false, widowControl: true, pageBreakBefore: false},
    indent: {left: 0, right: 0, firstLine: 0, mixed: {left: false, right: false, firstLine: false}},
    disagrees: {alignment: false, direction: false, spaceBeforePt: false, spaceAfterPt: false, lineSpacing: false, tabStops: false}};
const paragraph = L.paragraphFields(plain, cm);
same(paragraph, {alignment: 'left', left: '0', right: '0', special: 'none', by: '', before: '0', after: '8', lineRule: 'multiple', lineValue: '1,08',
    keepNext: false, keepLines: false, widowControl: true, pageBreakBefore: false});
const shaped = L.paragraphFields({...plain, alignment: 'both', lineSpacing: {rule: 'multiple', value: 1.5}, indent: {...plain.indent, firstLine: -283}}, cm);
assert.equal([shaped.alignment, shaped.special, shaped.by, shaped.lineRule, shaped.lineValue].join('|'), 'justify|hanging|0,5|150|');
assert.equal(L.paragraphFields({...plain, lineSpacing: {rule: 'exact', value: 18}}, cm).lineRule, 'exact');
assert.equal(L.paragraphFields({...plain, lineSpacing: undefined}, cm).lineRule, 'single');
const mixedFormatting = {paragraphFlags: {contextualSpacing: null, keepNext: null, keepLines: false, widowControl: true, pageBreakBefore: false},
    indent: {left: 0, right: 0, firstLine: 0, mixed: {left: true, right: false, firstLine: true}},
    disagrees: {alignment: true, spaceBeforePt: true, spaceAfterPt: false, lineSpacing: true}};
const mixed = L.paragraphFields(mixedFormatting, cm);
same(mixed, {alignment: '', left: '', right: '0', special: '', by: '', before: '', after: '0', lineRule: '', lineValue: '',
    keepNext: null, keepLines: false, widowControl: true, pageBreakBefore: false});
assert.equal(L.paragraphFields(undefined, cm).keepNext, null);
same(L.paragraphInput('special', {...paragraph, special: 'none'}), {by: ''});
same(L.paragraphInput('special', {...paragraph, special: 'first'}), {});
same(L.paragraphInput('lineRule', {...paragraph, lineRule: 'double'}), {lineValue: ''});
same(L.paragraphInput('lineRule', {...paragraph, lineRule: 'exact'}), {});
same(L.paragraphCommand(paragraph, plain, cm), {command: null});
same(L.paragraphCommand({...paragraph, alignment: 'justify', left: '1', special: 'hanging', by: '0,5', before: '6', lineRule: 'exact', lineValue: '18', keepNext: true}, plain, cm),
    {command: {type: 'setParagraphFormat', alignment: 'justify', indentLeftTwips: 567, indentFirstLineTwips: -283, spaceBeforePt: 6, lineSpacing: {rule: 'exact', value: 18}, keepNext: true}});
same(L.paragraphCommand({...paragraph, left: '', after: ''}, plain, cm), {command: {type: 'setParagraphFormat', indentLeftTwips: null, spaceAfterPt: null}});
same(L.paragraphCommand({...paragraph, lineRule: '150', lineValue: ''}, plain, cm).command.lineSpacing, {rule: 'multiple', value: 1.5});
same(L.paragraphCommand({...paragraph, special: 'first', by: '1,25'}, plain, cm).command, {type: 'setParagraphFormat', indentFirstLineTwips: 709});
same(L.paragraphCommand({...paragraph, special: 'first', by: ''}, plain, cm), {error: 'length_invalid'});
same(L.paragraphCommand({...paragraph, right: 'x'}, plain, cm), {error: 'length_invalid'});
same(L.paragraphCommand({...paragraph, before: '-1'}, plain, cm), {error: 'spacing_invalid'});
same(L.paragraphCommand({...paragraph, lineRule: 'multiple', lineValue: '20'}, plain, cm), {error: 'line_spacing_invalid'});
same(L.paragraphCommand({...paragraph, lineRule: 'atLeast', lineValue: ''}, plain, cm), {error: 'line_spacing_invalid'});
same(L.paragraphCommand({...mixed, keepNext: true}, mixedFormatting, cm), {command: {type: 'setParagraphFormat', keepNext: true}});
// A retyped value is no change; an invalid text in a mixed field is refused; inside a table there is no indent to show.
same(L.paragraphCommand({...paragraph, after: '8,0', left: '0.0', lineValue: '1.08'}, plain, cm), {command: null});
same(L.paragraphCommand({...mixed, left: 'x'}, mixedFormatting, cm), {error: 'length_invalid'});
const cell = {...plain, indent: undefined}, cellFields = L.paragraphFields(cell, cm);
assert.equal([cellFields.left, cellFields.right, cellFields.special].join('|'), '||');
same(L.paragraphCommand({...cellFields, keepLines: true}, cell, cm), {command: {type: 'setParagraphFormat', keepLines: true}});
console.log('Writer layout: units, papers, page setup and paragraph models pass');
