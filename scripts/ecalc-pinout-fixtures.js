// Fixture pinouts of the ECalc pinout page: one function expression `() => [item, ...]` that builds pinout items in exactly the
// shape of the catalog JSON (internal/ecalc/pinouts.go, "Catalog JSON" of the contract: every shape field present). The node
// test scripts/test-ecalc-pinouts.mjs evaluates it in a vm, ui/desktop_ecalc_pinouts_browser_test.go puts the same text into
// the served catalog. The names are German like the rest of the fixture catalog. The pins are made up (no real connector).
//   Every outline of `rows` appears once (fx_pin_<outline>), plus the dense ones: usb_c (24 contacts), hdmi_a (19),
//   fx_pin_header40 (2 x 20), fx_pin_board38 (38 pins), fx_pin_dip28; the shapes circular (two), dip (two), to (to92, to220),
//   jack (TRS, TRRS with variants) and board (two, with inner headers); a single row with wires and variants (fx_pin_rj45); a
//   page with long notes and long names (fx_pin_long); an item with 8-character numbers (fx_pin_wide_numbers).
() => {
    const TYPES = ['power', 'ground', 'signal', 'data', 'io', 'nc'];
    const NAMES = ['VBUS', 'GND', 'D+', 'D-', 'CC1', 'SBU1', 'TX1+', 'RX2-', 'GPIO23', 'SDA', 'SCL', '3V3', 'RESET', 'TMDS D2+', 'HSYNC', 'NC'];
    const shape = extra => Object.assign({
        type: '', outline: '', contact: '', drawn: '', key: null, rows: [], pins: null, package: '', positions: [], segments: [],
        left: [], right: [], top: [], bottom: [], connector: '', inner: []
    }, extra);
    const pinsOf = (numbers, offset, wires) => numbers.map((number, i) => {
        const type = TYPES[(i + (offset || 0)) % TYPES.length];
        const pin = {
            number, name: NAMES[(i + (offset || 0)) % NAMES.length] + (i >= NAMES.length ? String(i) : ''), type,
            function: 'Funktion von Pin ' + number + ' (' + type + ')', notes: i % 5 === 3 ? 'Hinweis zu Pin ' + number + ': nur im Testbetrieb.' : '', wire: ''
        };
        if (wires) pin.wire = wires[i % wires.length];
        return pin;
    });
    const item = (id, group, name, sh, numbers, extra) => Object.assign({
        kind: 'pinout', id, category: 'pinouts', group, name, name_en: name + ' (en)', doc: 'Ein Testanschluss für die Zeichnung.', notes: '', tags: [id.replace(/_/g, ' '), 'fixture'],
        source: 'Fixture, kein echter Anschluss', views: null, shape: sh, pins: pinsOf(numbers, extra && extra.offset, extra && extra.wires), variants: []
    }, extra && extra.fields);
    const two = { plug: 'Stecker (Stifte), Blick auf die Steckseite', socket: 'Buchse (Kontakte), Blick auf die Steckseite' };
    const range = (from, to) => Array.from({ length: to - from + 1 }, (_, i) => String(from + i));
    const rows = (outline, contact, drawn, lists, key) => shape({ type: 'rows', outline, contact, drawn, key: key || 'none', rows: lists.map(l => (Array.isArray(l) ? { pins: l, offset: 0 } : l)) });
    const flat = lists => lists.flatMap(l => (Array.isArray(l) ? l : l.pins)).filter(n => n !== null);
    const make = (id, group, name, outline, contact, lists, extra, key) => {
        const sh = rows(outline, contact, 'plug', lists, key);
        return item(id, group, name, sh, flat(lists), Object.assign({ fields: { views: two } }, extra));
    };
    const out = [];

    // ---- one item per outline of rows ----
    out.push(make('fx_pin_rect', 'boards', 'Fixture: Pfostenstecker 2x5', 'rect', 'square', [['1', '3', '5', '7', '9'], ['2', '4', '6', '8', '10']], {}, 'top'));
    out.push(make('fx_pin_dsub', 'data', 'Fixture: Sub-D 9', 'dsub', 'round', [['1', '2', '3', '4', '5'], { pins: ['6', '7', '8', '9'], offset: 0.5 }], { offset: 3 }));
    out.push(make('fx_pin_dsub15', 'data', 'Fixture: Sub-D 15 (3 Reihen)', 'dsub', 'round', [range(1, 5), range(6, 10), range(11, 15)]));
    out.push(make('fx_pin_usb_a', 'data', 'Fixture: USB-A', 'usb_a', 'blade', [['1', '2', '3', '4']]));
    out.push(make('fx_pin_usb_a3', 'data', 'Fixture: USB-A 3.0', 'usb_a', 'blade', [['1', '2', '3', '4'], { pins: ['5', '6', '7', '8', '9'], offset: 0 }]));
    out.push(make('fx_pin_usb_b', 'data', 'Fixture: USB-B', 'usb_b', 'blade', [['1', '2'], ['3', '4']]));
    out.push(make('fx_pin_usb_mini_b', 'data', 'Fixture: USB Mini-B', 'usb_mini_b', 'blade', [['1', '2', '3', '4', '5']]));
    out.push(make('fx_pin_usb_micro_b', 'data', 'Fixture: USB Micro-B', 'usb_micro_b', 'blade', [['1', '2', '3', '4', '5']]));
    out.push(make('fx_pin_usb_c', 'data', 'Fixture: USB-C (24 Kontakte)', 'usb_c', 'blade', [['A1', 'A2', 'A3', 'A4', 'A5', 'A6', 'A7', 'A8', 'A9', 'A10', 'A11', 'A12'], ['B12', 'B11', 'B10', 'B9', 'B8', 'B7', 'B6', 'B5', 'B4', 'B3', 'B2', 'B1']]));
    out.push(make('fx_pin_hdmi_a', 'data', 'Fixture: HDMI Typ A (19 Kontakte)', 'hdmi_a', 'blade', [['1', '3', '5', '7', '9', '11', '13', '15', '17', '19'], { pins: ['2', '4', '6', '8', '10', '12', '14', '16', '18'], offset: 0.5 }]));
    out.push(make('fx_pin_displayport', 'data', 'Fixture: DisplayPort', 'displayport', 'blade', [['1', '3', '5', '7', '9', '11', '13', '15', '17', '19'], { pins: ['2', '4', '6', '8', '10', '12', '14', '16', '18', '20'], offset: 0.5 }]));
    out.push(make('fx_pin_rj45', 'data', 'Fixture: RJ45', 'rj45', 'blade', [range(1, 8)], { wires: ['white_orange', 'orange', 'white_green', 'blue', 'white_blue', 'green', 'white_brown', 'brown'] }));
    out.push(make('fx_pin_sata_data', 'pc_power', 'Fixture: SATA Daten', 'sata_data', 'blade', [range(1, 7)]));
    out.push(make('fx_pin_sata_power', 'pc_power', 'Fixture: SATA Strom', 'sata_power', 'blade', [range(1, 15)]));
    out.push(make('fx_pin_molex', 'pc_power', 'Fixture: Molex 4', 'molex', 'round', [range(1, 4)]));
    out.push(make('fx_pin_minifit', 'pc_power', 'Fixture: Mini-Fit 2x6', 'minifit', 'square', [range(1, 6), range(7, 12)]));
    out.push(make('fx_pin_atx24', 'pc_power', 'Fixture: ATX 24 (Namen neben den Kontakten)', 'minifit', 'square', Array.from({ length: 12 }, (_, i) => [String(i + 1), String(i + 13)])));
    out.push(make('fx_pin_obd2', 'debug', 'Fixture: OBD-II', 'obd2', 'blade', [range(1, 8), range(9, 16)]));
    out.push(make('fx_pin_jst', 'debug', 'Fixture: JST 4', 'jst', 'blade', [range(1, 4)]));
    // dense: a pin header 2 x 20 and a single row of 40
    out.push(make('fx_pin_header40', 'boards', 'Fixture: Stiftleiste 2x20', 'rect', 'square', [Array.from({ length: 20 }, (_, i) => String(2 * i + 1)), Array.from({ length: 20 }, (_, i) => String(2 * i + 2))]));
    // a stable item without the key and with the wires of two variants
    const withVariants = make('fx_pin_variants', 'data', 'Fixture: Varianten', 'rj45', 'blade', [range(1, 8)], { wires: ['white_orange', 'orange', 'white_green', 'blue', 'white_blue', 'green', 'white_brown', 'brown'] });
    const second = withVariants.pins.map((pin, i) => Object.assign({}, pin, { name: 'V' + pin.name, wire: i % 2 ? 'red' : 'black' }));
    withVariants.variants = [{ id: 'va', name: 'Variante A', pins: withVariants.pins }, { id: 'vb', name: 'Variante B', pins: second }];
    withVariants.pins = [];
    out.push(withVariants);

    // a single column with symbols as numbers and wire colours (a servo): the wires leave to the left, the names stand on the right
    out.push(make('fx_pin_servo', 'debug', 'Fixture: Servo (Symbole als Pinnummern)', 'jst', 'square', [['S'], ['+'], ['-']], { wires: ['orange', 'red', 'brown'] }));

    // ---- circular ----
    out.push(item('fx_pin_din5', 'audio', 'Fixture: DIN 5 (rund)', shape({
        type: 'circular', contact: 'round', drawn: 'plug', key: 180,
        positions: [['1', 270, 0.7], ['4', 315, 0.7], ['2', 0, 0.7], ['5', 45, 0.7], ['3', 90, 0.7]].map(p => ({ pin: p[0], angle: p[1], radius: p[2] }))
    }), ['1', '4', '2', '5', '3'], { fields: { views: two } }));
    out.push(item('fx_pin_xlr', 'audio', 'Fixture: XLR 3', shape({
        type: 'circular', contact: 'round', drawn: 'socket', key: 0,
        positions: [{ pin: '1', angle: 300, radius: 0.55 }, { pin: '2', angle: 60, radius: 0.55 }, { pin: '3', angle: 180, radius: 0.55 }]
    }), ['1', '2', '3'], { fields: { views: two } }));
    out.push(item('fx_pin_circ12', 'debug', 'Fixture: Rund 12 (dicht)', shape({
        type: 'circular', contact: 'round', drawn: 'plug', key: 90,
        positions: Array.from({ length: 11 }, (_, i) => ({ pin: String(i + 1), angle: 30 * i + 15, radius: 0.8 })).concat([{ pin: '12', angle: 0, radius: 0 }])
    }), range(1, 12), { fields: { views: two } }));

    // ---- dip, to, jack ----
    out.push(item('fx_pin_dip8', 'ics', 'Fixture: DIP-8', shape({ type: 'dip', drawn: 'top', pins: 8 }), range(1, 8)));
    out.push(item('fx_pin_dip28', 'ics', 'Fixture: DIP-28', shape({ type: 'dip', drawn: 'top', pins: 28 }), range(1, 28)));
    out.push(item('fx_pin_to92', 'ics', 'Fixture: TO-92', shape({ type: 'to', drawn: 'front', package: 'to92', pins: ['1', '2', '3'] }), ['1', '2', '3']));
    out.push(item('fx_pin_to220', 'ics', 'Fixture: TO-220', shape({ type: 'to', drawn: 'front', package: 'to220', pins: ['IN', 'GND', 'OUT'] }), ['IN', 'GND', 'OUT']));
    out.push(item('fx_pin_trs', 'audio', 'Fixture: Klinke 3,5 mm', shape({ type: 'jack', drawn: 'side', segments: ['T', 'R', 'S'] }), ['T', 'R', 'S']));
    const trrs = item('fx_pin_trrs', 'audio', 'Fixture: Klinke 4-polig', shape({ type: 'jack', drawn: 'side', segments: ['T', 'R1', 'R2', 'S'] }), ['T', 'R1', 'R2', 'S']);
    trrs.variants = [{ id: 'ctia', name: 'CTIA', pins: trrs.pins }, { id: 'omtp', name: 'OMTP', pins: trrs.pins.map(pin => Object.assign({}, pin, { name: pin.name + '*' })) }];
    trrs.pins = [];
    out.push(trrs);

    // seven variants with long names: the selector wraps inside the page
    const many = item('fx_pin_many_variants', 'ics', 'Fixture: Sieben Varianten', shape({ type: 'to', drawn: 'front', package: 'to92', pins: ['1', '2', '3'] }), ['1', '2', '3']);
    many.variants = ['BC547', 'BC337', '2N3904', '2N2222A', 'LM35DZ', 'TL431LP', 'Eine Variante mit sehr langem Namen'].map((name, i) => ({ id: 'v' + i, name, pins: many.pins.map(pin => Object.assign({}, pin, { name: pin.name + i })) }));
    many.pins = [];
    out.push(many);

    // ---- boards ----
    const left38 = range(1, 19), right38 = range(20, 38);
    out.push(item('fx_pin_board38', 'boards', 'Fixture: Board mit 38 Pins', shape({ type: 'board', drawn: 'top', left: left38, right: right38, connector: 'bottom' }), left38.concat(right38)));
    const tops = ['D0', 'D1', 'D2', 'D3', 'D4', 'D5', 'D6', 'D7'], bottoms = ['A0', 'A1', 'A2', 'A3', 'A4', 'A5'];
    out.push(item('fx_pin_uno', 'boards', 'Fixture: Board mit ICSP', shape({
        type: 'board', drawn: 'top', top: tops, bottom: bottoms, left: ['L1', null, 'L2', 'L3'], connector: 'left',
        inner: [{ label: 'ICSP', rows: [{ pins: ['I1', 'I2', 'I3'], offset: 0 }, { pins: ['I4', 'I5', 'I6'], offset: 0 }] }]
    }), tops.concat(bottoms, ['L1', 'L2', 'L3', 'I1', 'I2', 'I3', 'I4', 'I5', 'I6'])));

    // ---- long texts and long numbers ----
    const long = make('fx_pin_long', 'data', 'Fixture: Sehr langer Name eines Anschlusses mit viel Text im Titel', 'dsub', 'round', [range(1, 5), { pins: range(6, 9), offset: 0.5 }]);
    long.notes = 'Eine lange Anmerkung unter der Tabelle, die über mehrere Zeilen umbricht und keinesfalls die Seite seitwärts wachsen lassen darf, auch nicht bei einem sehr schmalen Fenster.';
    long.doc = 'Eine längere Beschreibung über der Zeichnung, die ebenfalls umbrechen muss, ohne dass ein Wort die Breite sprengt: Donaudampfschifffahrtsgesellschaftskapitän.';
    long.pins[2].notes = 'Sehr lange Anmerkung zu diesem einen Pin, die in der Tabelle in mehreren Zeilen umbricht: Abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz.';
    long.pins[3].name = 'NAME_MIT_24_ZEICHEN_1234';
    long.pins[3].function = 'Eine ziemlich lange Funktionsbeschreibung, die in der Tabelle und im Detailfeld umbrechen muss';
    out.push(long);
    out.push(item('fx_pin_wide_numbers', 'boards', 'Fixture: Lange Pinnummern', shape({ type: 'board', drawn: 'top', left: ['J2.5-LONG', 'J2.6-LONG', 'J2.7'], right: ['P1.0', 'P1.1', 'P1.2'], connector: 'none' }), ['J2.5-LONG', 'J2.6-LONG', 'J2.7', 'P1.0', 'P1.1', 'P1.2']));
    return out;
}
