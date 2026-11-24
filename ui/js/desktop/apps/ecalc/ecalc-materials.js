/* ECalc materials: the fixed colours of real materials for the component and connector drawings. A DOM-free classic script
   that publishes window.ECalcMaterials. A material is data, like the hex of a colour band or of a pin type: it reads the same
   in every desktop skin (tin stays tin, epoxy stays black). This file is the only ECalc drawing file that may hold colour
   literals; the drawing builders (ecalc-code-draw.js, ecalc-pinout-draw.js) refer to materials by name and take the markup
   from a scope.

   ECalcMaterials.MATERIALS  frozen table name -> material:
       {kind: 'linear', stops: [[offset, hex, opacity?], ...], ink?}
       {kind: 'radial', cx, cy, r, stops, ink?}
       {kind: 'flat', hex, opacity?, ink?}
     ink   a key of INKS: the print colour for text and marks drawn on that material; it must hold (4.5:1) against every stop
   ECalcMaterials.INKS       frozen {light, dark, sepia}
   ECalcMaterials.ink(name)  hex of the print colour for that material, '' when it has no ink
   ECalcMaterials.safeId(uid) uid reduced to [A-Za-z0-9_-]; prefixed with 'ec' when empty or not starting with a letter
   ECalcMaterials.scope(uid) one collector per drawing:
       .fill(name, across)   'url(#<safeId>-<name>)' for a gradient (remembered for defs()), the hex for a flat material, ''
                             for an unknown name; across === 'x' asks for the linear gradient running left to right (id
                             '<safeId>-<name>-x'), the default runs top to bottom; a radial gradient ignores across
       .paint(name, across)  ' fill="..."' (plus ' fill-opacity="..."' for a flat material with an opacity), '' when unknown
       .defs()               '<defs>...</defs>' with every gradient asked for so far, each once, in order of first use; '' when none
   The output is deterministic: no counters, no clock, no random numbers. */
(function () {
    'use strict';

    const INKS = Object.freeze({ light: '#f1f1ee', dark: '#1d1d1b', sepia: '#2a1a08' });

    const linear = (stops, extra) => Object.assign({ kind: 'linear', stops }, extra || {});
    const flat = (hex, extra) => Object.assign({ kind: 'flat', hex }, extra || {});

    const TABLE = {
        // Leads and sheet metal.
        tin: linear([[0, '#7d8388'], [0.35, '#f3f5f6'], [0.7, '#a9afb4'], [1, '#6a7075']]),
        steel: linear([[0, '#aeb4ba'], [0.3, '#f5f7f8'], [0.65, '#bcc2c8'], [1, '#7f868d']], { ink: 'dark' }),
        gold: linear([[0, '#a87b22'], [0.35, '#f7dc8c'], [0.7, '#d4a640'], [1, '#8a631a']]),
        // Component bodies: resistor, inductor, moulded package, disc capacitor, SMD chip.
        carbon: linear([[0, '#b3935f'], [0.3, '#ecd8ad'], [0.65, '#d6bc8a'], [1, '#94774a']]),
        film: linear([[0, '#4d84ab'], [0.3, '#b4d8ee'], [0.65, '#86b6d6'], [1, '#3b6a8d']]),
        choke: linear([[0, '#4f8f6c'], [0.3, '#b9dfc6'], [0.65, '#86bd9c'], [1, '#3c7355']]),
        epoxy: linear([[0, '#43474d'], [0.5, '#23262a'], [1, '#131517']], { ink: 'light' }),
        ceramic: { kind: 'radial', cx: 0.36, cy: 0.3, r: 0.8, stops: [[0, '#f3cf93'], [0.55, '#cf9645'], [1, '#b47d34']], ink: 'sepia' },
        dip: flat('#a9712c'),
        alumina: flat('#e6e2d8'),
        recess: flat('#0b0c0d'),
        // Housings of the connectors: cavities, plastics, circuit board, rubber.
        cavity: flat('#16181b', { ink: 'light' }),
        pbt: linear([[0, '#3a3d42'], [1, '#1e2023']], { ink: 'light' }),
        nylon: linear([[0, '#fbf8ef'], [1, '#d9d3c2']], { ink: 'dark' }),
        grey: linear([[0, '#c9ccd0'], [1, '#9da1a6']], { ink: 'dark' }),
        pcb: linear([[0, '#24744b'], [1, '#154d31']], { ink: 'light' }),
        rubber: linear([[0, '#2b2d30'], [1, '#151618']], { ink: 'light' }),
        // Effects laid over a body: gloss, soft sheen over contacts and wires, a cast shadow, a small highlight.
        gloss: linear([[0, '#ffffff', 0], [0.14, '#ffffff', 0.5], [0.34, '#ffffff', 0], [0.7, '#000000', 0], [1, '#000000', 0.38]]),
        sheen: linear([[0, '#ffffff', 0], [0.16, '#ffffff', 0.26], [0.4, '#ffffff', 0], [0.7, '#000000', 0], [1, '#000000', 0.18]]),
        shadow: flat('#000000', { opacity: 0.16 }),
        glint: flat('#ffffff', { opacity: 0.14 })
    };

    const MATERIALS = Object.freeze(Object.fromEntries(Object.entries(TABLE).map(([name, material]) => [name, Object.freeze(material)])));
    const has = name => typeof name === 'string' && Object.prototype.hasOwnProperty.call(MATERIALS, name);

    // Numbers print without a leading zero (".35") and without trailing zeros.
    const num = value => String(value).replace(/^0\./, '.');

    const ink = name => (has(name) && MATERIALS[name].ink ? INKS[MATERIALS[name].ink] : '');

    function safeId(uid) {
        const clean = String(uid == null ? '' : uid).replace(/[^A-Za-z0-9_-]/g, '');
        return /^[A-Za-z]/.test(clean) ? clean : 'ec' + clean;
    }

    function gradientMarkup(id, material, across) {
        const stops = material.stops.map(stop => '<stop offset="' + num(stop[0]) + '" stop-color="' + stop[1] + '"' + (stop.length > 2 ? ' stop-opacity="' + num(stop[2]) + '"' : '') + '/>').join('');
        if (material.kind === 'radial') return '<radialGradient id="' + id + '" cx="' + num(material.cx) + '" cy="' + num(material.cy) + '" r="' + num(material.r) + '">' + stops + '</radialGradient>';
        return '<linearGradient id="' + id + '" x1="0" y1="0" x2="' + (across === 'x' ? '1' : '0') + '" y2="' + (across === 'x' ? '0' : '1') + '">' + stops + '</linearGradient>';
    }

    function scope(uid) {
        const prefix = safeId(uid);
        const used = new Map();
        function fill(name, across) {
            if (!has(name)) return '';
            const material = MATERIALS[name];
            if (material.kind === 'flat') return material.hex;
            const id = prefix + '-' + name + (material.kind === 'linear' && across === 'x' ? '-x' : '');
            if (!used.has(id)) used.set(id, gradientMarkup(id, material, across));
            return 'url(#' + id + ')';
        }
        function paint(name, across) {
            const value = fill(name, across);
            if (value === '') return '';
            const material = MATERIALS[name];
            return ' fill="' + value + '"' + (material.kind === 'flat' && material.opacity != null ? ' fill-opacity="' + num(material.opacity) + '"' : '');
        }
        function defs() {
            return used.size === 0 ? '' : '<defs>' + [...used.values()].join('') + '</defs>';
        }
        return { fill, paint, defs };
    }

    window.ECalcMaterials = { MATERIALS, INKS, ink, safeId, scope };
})();
