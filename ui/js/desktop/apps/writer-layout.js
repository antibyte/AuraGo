(function () {
    'use strict';
    // Page and paragraph layout as the dialogs and the Format panel show it. Pure: no DOM, no editor.
    const PER = {cm: 1440 / 2.54, in: 1440};
    // Lengths are shown in centimetres, or in inches for an English interface in a US browser.
    // locale is the interface language, region the browser's language tag.
    function units(locale, region) {
        const unit = /^en\b/i.test(String(locale || '')) && /^en-US$/i.test(String(region || '')) ? 'in' : 'cm';
        const options = {maximumFractionDigits: 2, useGrouping: false};
        let format;
        try { format = new Intl.NumberFormat(locale || 'en', options); } catch (_) { format = new Intl.NumberFormat('en', options); }
        const known = value => value !== null && value !== undefined && value !== '' && Number.isFinite(Number(value));
        // A number as the user reads it; '' for an unknown or mixed value.
        const number = value => known(value) ? format.format(Number(value)) : '';
        // A typed number, with a comma or a point: null for an empty field, NaN for anything else.
        function parse(text) {
            const raw = String(text ?? '').trim().replace('−', '-').replace(',', '.');
            if (!raw) return null;
            return /^-?(\d+\.?\d*|\.\d+)$/.test(raw) ? Number(raw) : NaN;
        }
        const suffix = unit === 'in' ? /\s*(in|"|″)\s*$/i : /\s*cm\s*$/i;
        const show = twips => known(twips) ? number(Number(twips) / PER[unit]) : '';
        // Twips for a typed length; the unit's own name may follow the number.
        function read(text) {
            const value = parse(String(text ?? '').replace(suffix, ''));
            return value === null || Number.isNaN(value) ? value : Math.round(value * PER[unit]);
        }
        const points = text => parse(String(text ?? '').replace(/\s*pt\s*$/i, ''));
        return {unit, number, parse, show, read, points};
    }
    const PAPERS = [{id: 'a4', name: 'A4', width: 11906, height: 16838}, {id: 'a5', name: 'A5', width: 8391, height: 11906}, {id: 'a3', name: 'A3', width: 16838, height: 23811},
        {id: 'b5', name: 'B5', width: 9978, height: 14173}, {id: 'letter', name: 'Letter', width: 12240, height: 15840}, {id: 'legal', name: 'Legal', width: 12240, height: 20160}];
    // The paper a size is, in either orientation; 'custom' for anything else.
    function paperOf(width, height) {
        const short = Math.min(width, height), long = Math.max(width, height);
        return PAPERS.find(paper => Math.abs(short - paper.width) < 20 && Math.abs(long - paper.height) < 20)?.id || 'custom';
    }
    // Margin presets as [top, right, bottom, left] in twips.
    const MARGINS = {normal: [1417, 1417, 1417, 1417], narrow: [720, 720, 720, 720], wide: [1440, 2880, 1440, 2880]};
    const SIDES = ['top', 'right', 'bottom', 'left'];
    const presetOf = margins => Object.keys(MARGINS).find(id => MARGINS[id].every((twips, index) => Math.abs(twips - margins[index]) < 6)) || 'custom';
    // A typed length shows two decimals. Where it shows exactly like a paper edge or a preset margin,
    // that exact value is meant, so choosing A5 in inches does not shave the page.
    const KNOWN = [...new Set([...PAPERS.flatMap(paper => [paper.width, paper.height]), ...Object.values(MARGINS).flat()])];
    function exact(text, u) {
        const twips = u.read(text);
        return Number.isFinite(twips) ? KNOWN.find(value => u.show(value) === u.show(twips)) ?? twips : twips;
    }
    // The Page setup dialog's fields for a PageSetup, as the texts the dialog shows.
    function pageFields(setup, u) {
        const margins = SIDES.map(side => setup.marginsTwips[side]);
        return {paper: paperOf(setup.pageWidthTwips, setup.pageHeightTwips), width: u.show(setup.pageWidthTwips), height: u.show(setup.pageHeightTwips),
            orientation: setup.pageWidthTwips > setup.pageHeightTwips ? 'landscape' : 'portrait', preset: presetOf(margins),
            top: u.show(margins[0]), right: u.show(margins[1]), bottom: u.show(margins[2]), left: u.show(margins[3]), scope: 'document'};
    }
    // What else changes in the Page setup dialog when one field was edited: {field: text}.
    function pageInput(changed, values, u) {
        const width = exact(values.width, u), height = exact(values.height, u), sized = Number.isFinite(width) && Number.isFinite(height);
        if (changed === 'paper') {
            const paper = PAPERS.find(candidate => candidate.id === values.paper);
            if (!paper) return {};
            const landscape = values.orientation === 'landscape';
            return {width: u.show(landscape ? paper.height : paper.width), height: u.show(landscape ? paper.width : paper.height)};
        }
        if (changed === 'orientation') return sized && (width > height) !== (values.orientation === 'landscape') ? {width: u.show(height), height: u.show(width)} : {};
        if (changed === 'width' || changed === 'height') return sized ? {paper: paperOf(width, height), orientation: width > height ? 'landscape' : 'portrait'} : {};
        if (changed === 'preset') return MARGINS[values.preset] ? Object.fromEntries(SIDES.map((side, index) => [side, u.show(MARGINS[values.preset][index])])) : {};
        if (SIDES.includes(changed)) {
            const margins = SIDES.map(side => exact(values[side], u));
            return margins.every(Number.isFinite) ? {preset: presetOf(margins)} : {};
        }
        return {};
    }
    // The setPageSetup command for the dialog's answer: only what differs from what the dialog opened
    // with. {command} (null when nothing changed) or {error} with a desktop.writer_ key.
    function pageCommand(values, initial, u) {
        const width = exact(values.width, u), height = exact(values.height, u), margins = SIDES.map(side => exact(values[side], u));
        if (![width, height, ...margins].every(Number.isFinite) || margins.some(twips => twips < 0)) return {error: 'length_invalid'};
        if (Math.min(width, height) < 1440 || Math.max(width, height) > 31680) return {error: 'page_size_range'};
        if (margins[1] + margins[3] > width - 1440 || margins[0] + margins[2] > height - 1440) return {error: 'margins_too_large'};
        const command = {type: 'setPageSetup'};
        if (values.width !== initial.width || values.height !== initial.height) Object.assign(command, {pageWidth: width, pageHeight: height, orientation: width > height ? 'landscape' : 'portrait'});
        SIDES.forEach((side, index) => { if (values[side] !== initial[side]) command['margin' + side[0].toUpperCase() + side.slice(1)] = margins[index]; });
        if (Object.keys(command).length === 1) return {command: null};
        command.scope = values.scope === 'section' ? 'section' : 'document';
        return {command};
    }
    const LINES = {single: 1, '115': 1.15, '150': 1.5, double: 2};
    const FLAGS = ['keepNext', 'keepLines', 'widowControl', 'pageBreakBefore'];
    // The Paragraph dialog's fields for snapshot().formatting. '' is a text the selection disagrees
    // on or cannot state, null a flag it disagrees on.
    function paragraphFields(formatting, u) {
        const f = formatting || {}, off = f.disagrees || {}, indent = f.indent, mixed = indent?.mixed || {};
        const first = indent && !mixed.firstLine ? indent.firstLine : null, line = f.lineSpacing;
        const preset = line?.rule === 'multiple' ? Object.keys(LINES).find(id => LINES[id] === line.value) : '';
        const fields = {
            alignment: off.alignment || !f.alignment ? '' : f.alignment === 'both' ? 'justify' : f.alignment,
            left: indent && !mixed.left ? u.show(indent.left) : '', right: indent && !mixed.right ? u.show(indent.right) : '',
            special: first === null ? '' : first > 0 ? 'first' : first < 0 ? 'hanging' : 'none', by: first ? u.show(Math.abs(first)) : '',
            before: off.spaceBeforePt ? '' : u.number(f.spaceBeforePt ?? 0), after: off.spaceAfterPt ? '' : u.number(f.spaceAfterPt ?? 0),
            lineRule: off.lineSpacing ? '' : !line ? 'single' : preset || line.rule, lineValue: off.lineSpacing || !line || preset ? '' : u.number(line.value)
        };
        for (const flag of FLAGS) fields[flag] = f.paragraphFlags ? f.paragraphFlags[flag] : null;
        return fields;
    }
    // What else changes in the Paragraph dialog when one field was edited.
    function paragraphInput(changed, values) {
        if (changed === 'special' && (values.special === 'none' || values.special === '')) return {by: ''};
        if (changed === 'lineRule' && (LINES[values.lineRule] || values.lineRule === '')) return {lineValue: ''};
        return {};
    }
    // The setParagraphFormat command for the dialog's answer: only what differs from what the dialog
    // opened with. An emptied length or spacing is null, which hands the value back to the style.
    function paragraphCommand(values, initial, u) {
        const command = {type: 'setParagraphFormat'};
        if (values.alignment !== initial.alignment && values.alignment) command.alignment = values.alignment;
        for (const [field, key] of [['left', 'indentLeftTwips'], ['right', 'indentRightTwips']]) {
            if (values[field] === initial[field]) continue;
            const twips = u.read(values[field]);
            if (Number.isNaN(twips)) return {error: 'length_invalid'};
            command[key] = twips;
        }
        if (values.special !== initial.special || values.by !== initial.by) {
            if (values.special === 'none') command.indentFirstLineTwips = 0;
            else if (values.special) {
                const by = u.read(values.by);
                if (!Number.isFinite(by) || by <= 0) return {error: 'length_invalid'};
                command.indentFirstLineTwips = values.special === 'hanging' ? -by : by;
            }
        }
        for (const [field, key] of [['before', 'spaceBeforePt'], ['after', 'spaceAfterPt']]) {
            if (values[field] === initial[field]) continue;
            const points = u.points(values[field]);
            if (Number.isNaN(points) || points < 0 || points > 1584) return {error: 'spacing_invalid'};
            command[key] = points;
        }
        if (values.lineRule !== initial.lineRule || values.lineValue !== initial.lineValue) {
            if (LINES[values.lineRule]) command.lineSpacing = {rule: 'multiple', value: LINES[values.lineRule]};
            else if (values.lineRule) {
                const value = u.points(values.lineValue), multiple = values.lineRule === 'multiple';
                if (!Number.isFinite(value) || value < (multiple ? 0.5 : 1) || value > (multiple ? 10 : 1584)) return {error: 'line_spacing_invalid'};
                command.lineSpacing = {rule: values.lineRule, value};
            }
        }
        for (const flag of FLAGS) if (values[flag] !== initial[flag] && typeof values[flag] === 'boolean') command[flag] = values[flag];
        return {command: Object.keys(command).length > 1 ? command : null};
    }
    window.WriterLayout = {units, papers: PAPERS, margins: MARGINS, lines: LINES, paperOf, pageFields, pageInput, pageCommand, paragraphFields, paragraphInput, paragraphCommand};
})();
