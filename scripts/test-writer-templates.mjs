import assert from 'node:assert/strict';
import vm from 'node:vm';
import {readFileSync} from 'node:fs';

const context = {window: {}};
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-layout.js', 'utf8'), context);
vm.runInNewContext(readFileSync('ui/js/desktop/apps/writer-templates.js', 'utf8'), context);
const T = context.window.WriterTemplates, tr = key => '[' + key + ']';
const same = (actual, expected, message) => assert.equal(JSON.stringify(actual), JSON.stringify(expected), message);

// The catalogue: four recipes, each with a paper, margins, blocks and a footer.
assert.equal(T.ids.join(), 'letter,invoice,minutes,report');
assert.equal(T.folder, 'Documents/Templates');
assert.equal(T.recipe('nope', tr), null);
for (const id of T.ids) {
    assert.equal(T.label(id, tr), '[' + id + ']');
    const plan = T.recipe(id, tr, {date: 'TODAY'});
    assert.equal(plan.id, id);
    same(plan.paper, {width: 11906, height: 16838}, id + ' is A4');
    same(plan.margins, [1417, 1134, 1134, 1417]);
    assert.equal(plan.footer, true);
    assert.ok(plan.blocks.length >= 8, id + ' has content');
    for (const block of plan.blocks) {
        assert.equal([block.text !== undefined, !!block.list, !!block.table].filter(Boolean).length, 1, id + ': a block is a paragraph, a list or a table');
        if (block.table) {
            for (const row of block.table) assert.equal(row.length, block.widths.length, id + ': every row has a width per cell');
            assert.equal(block.widths.reduce((sum, width) => sum + width, 0), 600, id + ': the widths add up to 600');
        }
    }
    const texts = JSON.stringify(plan.blocks);
    assert.ok(texts.includes('TODAY'), id + ' carries the date');
    assert.ok(!texts.includes('undefined'), id + ' has no missing text');
}
same(T.recipe('letter', tr, {inches: true}).paper, {width: 12240, height: 15840}, 'a user of inches gets Letter');
assert.notEqual(T.recipe('letter', tr).margins, T.recipe('invoice', tr).margins, 'margins are a copy');

// The letter: letterhead at the right, return line, recipient, date, subject, signature.
const letter = T.recipe('letter', tr, {date: 'TODAY'}).blocks;
same(letter[0], {text: '[template_sender_name]', align: 'right', bold: true, size: 14, after: 0});
assert.equal(letter[4].text, '[template_sender_name] · [template_street] · [template_city]');
assert.equal(letter[4].size, 8);
same(letter[8], {text: 'TODAY', align: 'right', before: 60});
assert.equal(letter.at(-1).text, '[template_signature]');
// The invoice: a five-column table with the amounts at the right and the total in bold.
const invoice = T.recipe('invoice', tr, {date: 'TODAY'}).blocks.find(block => block.table);
assert.equal(invoice.table[0].join(), '[invoice_pos],[invoice_description],[invoice_quantity],[invoice_unit_price],[invoice_amount]');
same(invoice.right, [2, 3, 4]);
assert.equal(invoice.head, true);
same(invoice.table.at(-1).slice(3), [{text: '[invoice_total]', bold: true}, {text: '…', bold: true}]);

// HTML for the engine's import: only what it is known to keep.
assert.equal(T.html([{text: 'A & B', align: 'right', size: 8, before: 6, after: 0}]), '<p style="text-align:right;font-size:8pt;margin-top:6pt;margin-bottom:0pt">A &amp; B</p>');
assert.equal(T.html([{text: 'Plain'}, {text: 'Bold', bold: true}]), '<p>Plain</p><p><strong>Bold</strong></p>');
assert.equal(T.html([{text: 'Styled', style: 'Heading1'}]), '<p>Styled</p>', 'a style is set after the import');
assert.equal(T.html([{list: ['a', '<b>']}, {list: ['c'], ordered: true}]), '<ul><li>a</li><li>&lt;b&gt;</li></ul><ol><li>c</li></ol>');
assert.equal(T.html([{table: [['H', 'N'], ['x', {text: 'y', bold: true}]], widths: [100, 500], head: true, right: [1]}]),
    '<table border="1" style="border-collapse:collapse">' +
    '<tr><td width="100" style="border:1px solid #64748b"><p style="margin-top:3pt;margin-bottom:3pt"><strong>H</strong></p></td>' +
    '<td width="500" style="border:1px solid #64748b"><p style="text-align:right;margin-top:3pt;margin-bottom:3pt"><strong>N</strong></p></td></tr>' +
    '<tr><td width="100" style="border:1px solid #64748b"><p style="margin-top:3pt;margin-bottom:3pt">x</p></td>' +
    '<td width="500" style="border:1px solid #64748b"><p style="text-align:right;margin-top:3pt;margin-bottom:3pt"><strong>y</strong></p></td></tr></table>');
same(T.styled(T.recipe('report', tr, {date: 'D'}).blocks), [['[report]', 'Title'], ['[report_author] · D', 'Subtitle'], ['[summary]', 'Heading1'], ['[findings]', 'Heading1'], ['[next_steps]', 'Heading1']]);
same(T.styled(T.recipe('letter', tr).blocks), []);

// A template's file name.
same(T.fileName('  House style '), {name: 'House style.docx'});
same(T.fileName('Brief.DOCX'), {name: 'Brief.docx'});
same(T.fileName('Angebot 2026 (Entwurf)'), {name: 'Angebot 2026 (Entwurf).docx'});
same(T.fileName('x'.repeat(80)), {name: 'x'.repeat(80) + '.docx'});
same(T.fileName('console'), {name: 'console.docx'}, 'a name that only starts like a device name is fine');
same(T.fileName('nulls.bak'), {name: 'nulls.bak.docx'});
for (const bad of ['', '   ', '.docx', 'a/b', 'a' + String.fromCharCode(92) + 'b', 'a:b', 'what?', 'a*b', '"q"', '<x>', 'a|b', '.hidden', 'dot.', 'CON', 'lpt1', '~$Brief', 'nul.bak', 'COM1.v2', 'x'.repeat(81), 'tab' + String.fromCharCode(9) + 'here'])
    same(T.fileName(bad), {error: 'template_name_invalid'}, JSON.stringify(bad));

// The user's own templates out of a folder listing.
same(T.own([{name: 'B.docx', path: 'Documents/Templates/B.docx', type: 'file'}, {name: 'Old', path: 'Documents/Templates/Old', type: 'directory'},
    {name: 'notes.txt', path: 'Documents/Templates/notes.txt', type: 'file'}, {name: '~$B.docx', path: 'Documents/Templates/~$B.docx', type: 'file'},
    {name: '.aurago-write-1f.docx', path: 'Documents/Templates/.aurago-write-1f.docx', type: 'file'}, {name: 'a.DOCX', path: 'Documents/Templates/a.DOCX', type: 'file'}, null]),
    [{path: 'Documents/Templates/B.docx', title: 'B'}, {path: 'Documents/Templates/a.DOCX', title: 'a'}]);
same(T.own(undefined), []);
same(T.own({error: 'x'}), []);
// The name a document offers for its template.
assert.equal(T.title('Documents/Angebot.docx'), 'Angebot');
assert.equal(T.title('Documents/untitled-1a2b3c4d.docx'), '');
assert.equal(T.title(''), '');
console.log('Writer templates: recipes, import HTML, styles, file names and the own list pass');
