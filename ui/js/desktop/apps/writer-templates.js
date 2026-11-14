(function () {
    'use strict';
    // The built-in templates and the rules for the user's own. A built-in template is a recipe: a paper, margins,
    // a list of blocks and a footer. build() turns a recipe into DOCX bytes in a scratch editor, so a document
    // made from it starts like one made from a user's own template: from finished bytes, with no undo steps.
    // A block is one of
    //   {text, style?, align?, size?, bold?, before?, after?}   a paragraph; style is a paragraph style id, size and
    //                                                            spacing are points
    //   {list: [text], ordered?}                                 a bulleted or numbered list
    //   {table: [[cell]], widths: [n], head?, right?: [column]}  a bordered table; a cell is a text or {text, bold};
    //                                                            widths are proportions, head makes the first row
    //                                                            bold, right names the right-aligned columns
    const ids = ['letter', 'invoice', 'minutes', 'report'];
    const folder = 'Documents/Templates';
    // Top, right, bottom, left: 25 mm at the top and the left, 20 mm at the right and the bottom. These are the
    // side margins of a DIN 5008 letter, and they leave the 6.5 inch the engine gives a pasted table on A4.
    const MARGINS = [1417, 1134, 1134, 1417];
    const names = tr => ({letter: tr('letter'), invoice: tr('invoice'), minutes: tr('minutes'), report: tr('report')});
    const label = (id, tr) => names(tr)[id];
    // The sender as a letterhead at the right, the small return line and the recipient. This follows the address
    // field of a DIN 5008 letter with paragraph spacing; the engine has no positioned frames.
    const address = tr => [
        {text: tr('template_sender_name'), align: 'right', bold: true, size: 14, after: 0},
        {text: tr('template_street'), align: 'right', after: 0},
        {text: tr('template_city'), align: 'right', after: 0},
        {text: tr('template_sender_contact'), align: 'right', after: 6},
        {text: [tr('template_sender_name'), tr('template_street'), tr('template_city')].join(' · '), size: 8, after: 6},
        {text: tr('template_recipient_name'), after: 0},
        {text: tr('template_street'), after: 0},
        {text: tr('template_city'), after: 0}
    ];
    const closing = tr => [{text: tr('template_closing'), before: 12, after: 36}, {text: tr('template_signature')}];
    const strong = text => ({text, bold: true});
    const RECIPES = {
        letter: (tr, date) => address(tr).concat([
            {text: date, align: 'right', before: 60},
            {text: tr('template_subject'), bold: true, before: 12, after: 24},
            {text: tr('template_greeting')},
            {text: tr('template_body')}], closing(tr)),
        invoice: (tr, date) => address(tr).concat([
            {text: tr('invoice_number') + ': …', align: 'right', before: 36, after: 0},
            {text: tr('invoice_date') + ': ' + date, align: 'right', after: 0},
            {text: tr('invoice_customer') + ': …', align: 'right'},
            {text: tr('invoice'), bold: true, size: 16, before: 12},
            {text: tr('template_greeting')},
            {text: tr('invoice_intro')},
            {table: [[tr('invoice_pos'), tr('invoice_description'), tr('invoice_quantity'), tr('invoice_unit_price'), tr('invoice_amount')],
                ['1', '…', '…', '…', '…'], ['2', '…', '…', '…', '…'],
                ['', '', '', tr('invoice_net'), '…'], ['', '', '', tr('invoice_tax'), '…'], ['', '', '', strong(tr('invoice_total')), strong('…')]],
                widths: [50, 250, 80, 110, 110], head: true, right: [2, 3, 4]},
            {text: tr('invoice_terms'), before: 12},
            {text: tr('invoice_bank')}], closing(tr)),
        minutes: (tr, date) => [
            {text: tr('minutes'), style: 'Title'},
            {table: [[strong(tr('insert_date')), date], [strong(tr('minutes_place')), '…'], [strong(tr('minutes_chair')), '…'], [strong(tr('minutes_keeper')), '…'],
                [strong(tr('participants')), '…']], widths: [150, 450]},
            {text: tr('agenda'), style: 'Heading1'},
            {list: [tr('minutes_topic'), tr('minutes_topic')], ordered: true},
            {text: tr('decisions'), style: 'Heading1'},
            {list: ['…']},
            {text: tr('next_steps'), style: 'Heading1'},
            {table: [[tr('task'), tr('owner'), tr('due')], ['…', '…', '…'], ['…', '…', '…']], widths: [300, 150, 150], head: true}],
        report: (tr, date) => [
            {text: tr('report'), style: 'Title'},
            {text: tr('report_author') + ' · ' + date, style: 'Subtitle'},
            {text: tr('summary'), style: 'Heading1'},
            {text: tr('template_body')},
            {text: tr('findings'), style: 'Heading1'},
            {text: tr('template_body')},
            {text: tr('next_steps'), style: 'Heading1'},
            {list: [tr('template_task')]}]
    };
    // options: {date: today's date as text, inches: true where lengths are inches; such a user gets Letter, not A4}.
    function recipe(id, tr, options = {}) {
        const make = RECIPES[id];
        if (!make) return null;
        const paper = window.WriterLayout.papers.find(candidate => candidate.id === (options.inches ? 'letter' : 'a4'));
        return {id, paper: {width: paper.width, height: paper.height}, margins: MARGINS.slice(), blocks: make(tr, options.date || ''), footer: true};
    }
    const escape = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
    // The engine's HTML import keeps alignment, font size, bold and the space before and after a paragraph.
    function paragraph(block) {
        const css = [];
        if (block.align) css.push('text-align:' + block.align);
        if (block.size) css.push('font-size:' + block.size + 'pt');
        if (block.before !== undefined) css.push('margin-top:' + block.before + 'pt');
        if (block.after !== undefined) css.push('margin-bottom:' + block.after + 'pt');
        const text = escape(block.text);
        return '<p' + (css.length ? ' style="' + css.join(';') + '"' : '') + '>' + (block.bold ? '<strong>' + text + '</strong>' : text) + '</p>';
    }
    // In a table the import reads the column width from the width attribute only, the borders from the cell style,
    // and an alignment only from a paragraph inside the cell.
    function table(block) {
        return '<table border="1" style="border-collapse:collapse">' + block.table.map((row, r) => '<tr>' + row.map((cell, c) => {
            const item = typeof cell === 'string' ? {text: cell} : cell;
            return '<td width="' + block.widths[c] + '" style="border:1px solid #64748b">' + paragraph({text: item.text, before: 3, after: 3,
                bold: item.bold || (!!block.head && r === 0), align: (block.right || []).includes(c) ? 'right' : undefined}) + '</td>';
        }).join('') + '</tr>').join('') + '</table>';
    }
    function list(block) {
        const tag = block.ordered ? 'ol' : 'ul';
        return '<' + tag + '>' + block.list.map(text => '<li>' + escape(text) + '</li>').join('') + '</' + tag + '>';
    }
    const html = blocks => blocks.map(block => block.table ? table(block) : block.list ? list(block) : paragraph(block)).join('');
    // The paragraphs that carry a paragraph style, in document order: [[text, style id]]. The HTML import knows no
    // styles, so build() sets them afterwards.
    const styled = blocks => blocks.filter(block => block.style).map(block => [block.text, block.style]);
    // The file name of a template the user names: {name} with .docx, or {error}. The same name must be valid on
    // every system the workspace may live on.
    function fileName(value) {
        const base = String(value ?? '').trim().replace(/\.docx$/i, '').trim();
        if (!base || base.length > 80 || /[\\/:*?"<>|\x00-\x1f]/.test(base) || /^\.|[. ]$/.test(base) || /^(con|prn|aux|nul|com[1-9]|lpt[1-9])$/i.test(base))
            return {error: 'template_name_invalid'};
        return {name: base + '.docx'};
    }
    // The user's templates out of a folder listing of the workspace: [{path, title}], in the listing's order.
    const own = files => (Array.isArray(files) ? files : [])
        .filter(file => file && file.type === 'file' && /\.docx$/i.test(file.name || '') && !/^(\.|~\$)/.test(file.name))
        .map(file => ({path: file.path, title: file.name.replace(/\.docx$/i, '')}));
    // What a document is called without its folder and extension; nothing for a document that has no name yet.
    function title(path) {
        const base = String(path ?? '').split('/').pop().replace(/\.docx$/i, '');
        return /^untitled-[0-9a-f]{8}$/.test(base) ? '' : base;
    }
    // Builds a recipe in a scratch editor inside mount (an attached, empty element) and answers the DOCX bytes.
    async function build(plan, lib, mount, fonts) {
        const scratch = lib.createDocxEditor({container: mount, document: 'blank', fonts, modules: [lib.reviewModule]});
        try {
            if (scratch.snapshot().isOpening) await scratch.save();
            const run = command => {
                const result = scratch.exec(command);
                if (!result.ok) throw new Error('template ' + plan.id + ': ' + command.type + ' ' + (result.reason || ''));
            };
            const [top, right, bottom, left] = plan.margins;
            run({type: 'setPageSetup', pageWidth: plan.paper.width, pageHeight: plan.paper.height, marginTop: top, marginRight: right, marginBottom: bottom, marginLeft: left});
            run({type: 'paste', text: '', html: html(plan.blocks)});
            const paragraphs = scratch.query({type: 'paragraphs'}), paragraphIds = scratch.surface.session.paragraphIds();
            let cursor = 0;
            for (const [text, style] of styled(plan.blocks)) {
                const index = paragraphs.findIndex((candidate, at) => at >= cursor && candidate.text === text);
                if (index < 0) throw new Error('template ' + plan.id + ': no paragraph "' + text + '"');
                cursor = index + 1;
                const start = {paragraphId: paragraphIds[index], offset: 0};
                run({type: 'setSelection', range: {anchor: start, head: start}});
                run(lib.commandForSlotValue('styles.style', style));
            }
            // "1 / 3", centred and small. Page fields exist only inside a header or footer.
            if (plan.footer) for (const command of [{type: 'editHeaderFooter', position: 'footer'}, {type: 'setAlignment', align: 'center'},
                lib.commandForSlotValue('font.size', 18), {type: 'insertPageField', field: 'PAGE'}, {type: 'paste', text: ' / '},
                {type: 'insertPageField', field: 'NUMPAGES'}, {type: 'exitHeaderFooter'}]) run(command);
            return await scratch.save();
        } finally { scratch.destroy(); }
    }
    window.WriterTemplates = {ids, folder, label, recipe, html, styled, fileName, own, title, build};
})();
