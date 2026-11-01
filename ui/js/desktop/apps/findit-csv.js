(function () {
    'use strict';

    const aliases = {
        entity: {
            items: {
                name: ['name', 'item', 'part', 'bezeichnung', 'artikel', 'komponente'],
                description: ['description', 'notes', 'beschreibung', 'notiz'],
                category: ['category', 'type', 'kategorie', 'typ'],
                manufacturer: ['manufacturer', 'brand', 'hersteller', 'marke'],
                part_number: ['part number', 'part_number', 'mpn', 'sku', 'artikelnummer', 'teilenummer'],
                unit: ['unit', 'uom', 'einheit'],
                minimum_stock: ['minimum stock', 'minimum_stock', 'min stock', 'minbestand'],
                package_family: ['package', 'package_family', 'footprint', 'gehäuse', 'gehaeuse'],
                pin_count: ['pin count', 'pin_count', 'pins', 'pins count', 'anschlüsse', 'anschluesse'],
                pitch_mm: ['pitch', 'pitch_mm', 'raster', 'pin pitch'],
                tags: ['tags', 'labels', 'etiketten']
            },
            project_lines: {
                item_id: ['item id', 'item_id', 'id'],
                manufacturer: ['manufacturer', 'brand', 'hersteller'],
                part_number: ['part number', 'part_number', 'mpn', 'sku', 'artikelnummer', 'teilenummer'],
                quantity: ['quantity', 'qty', 'menge', 'bedarf'],
                note: ['note', 'notes', 'comment', 'notiz', 'kommentar']
            }
        }
    };
    function normalizeHeader(value) {
        return String(value || '').replace(/^\uFEFF/, '').trim().toLowerCase().replace(/[ _-]+/g, ' ').replace(/\s+/g, ' ');
    }
    function delimiterFor(text) {
        const sample = String(text || '').split(/\r?\n/, 1)[0] || '';
        let comma = 0, semicolon = 0, quoted = false;
        for (let i = 0; i < sample.length; i++) {
            if (sample[i] === '"') {
                if (quoted && sample[i + 1] === '"') i++;
                else quoted = !quoted;
            } else if (!quoted && sample[i] === ',') comma++;
            else if (!quoted && sample[i] === ';') semicolon++;
        }
        return semicolon > comma ? ';' : ',';
    }
    function parse(text) {
        const input = String(text || '').replace(/^\uFEFF/, ''), delimiter = delimiterFor(input), rows = [];
        let row = [], cell = '', quoted = false;
        for (let i = 0; i < input.length; i++) {
            const char = input[i];
            if (char === '"') {
                if (quoted && input[i + 1] === '"') { cell += '"'; i++; }
                else quoted = !quoted;
            } else if (!quoted && char === delimiter) { row.push(cell); cell = ''; }
            else if (!quoted && (char === '\n' || char === '\r')) {
                if (char === '\r' && input[i + 1] === '\n') i++;
                row.push(cell); cell = '';
                if (row.some(value => String(value).trim())) rows.push(row);
                row = [];
            } else cell += char;
        }
        if (quoted) throw new Error('unterminated_quote');
        row.push(cell); if (row.some(value => String(value).trim())) rows.push(row);
        if (!rows.length) throw new Error('empty_csv');
        const headers = rows.shift().map(value => String(value).trim());
        if (headers.some(header => !header)) throw new Error('empty_header');
        const normalized = headers.map(normalizeHeader);
        if (new Set(normalized).size !== normalized.length) throw new Error('duplicate_header');
        const data = rows.map((values, index) => ({
            row: index + 2,
            values: headers.reduce((record, header, column) => { record[header] = String(values[column] ?? ''); return record; }, {})
        }));
        return { headers, rows: data, delimiter };
    }
    function guessMapping(headers, entity) {
        const candidates = aliases.entity[entity] || {};
        const normalizedHeaders = headers.map(header => ({ header, normalized: normalizeHeader(header) }));
        const mapping = {};
        for (const [field, names] of Object.entries(candidates)) {
            const accepted = new Set([normalizeHeader(field), ...names.map(normalizeHeader)]);
            const match = normalizedHeaders.find(item => accepted.has(item.normalized));
            if (match) mapping[field] = match.header;
        }
        return mapping;
    }
    function mappingFields(entity) {
        const fields = aliases.entity[entity] || {};
        return Object.keys(fields);
    }
    window.FindItCSV = { parse, guessMapping, mappingFields };
})();
