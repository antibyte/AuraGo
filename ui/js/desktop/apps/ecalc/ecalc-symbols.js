/* ECalc schematic symbols: the IEC (default) and ANSI symbol library and the renderer that turns a schematic spec into
   inline SVG. DOM-free classic script that publishes window.ECalcSymbols. Every stroke is currentColor, so a schematic
   follows the theme.

   Geometry: one grid unit is 20 px. A symbol is drawn in its own frame with pin 1 at the origin; a part places it with
   {sym, x, y, rot, flip} (x and y in grid units, rot in degrees clockwise, flip mirrors it across its own x axis
   before the rotation). Two-terminal symbols run from pin a (0,0) to pin b (3,0); a is the positive side of sources,
   batteries and polarised capacitors and the anode of diodes, so rot 90 stands them upright with a on top. The
   switch is such a two-terminal symbol; the potentiometer is four units wide (a (0,0), b (4,0), wiper w (2,-2)).

   A schematic spec is {w, h, parts, wires, dots, terminals, arrows, texts, anchors}: w and h in grid units, wires as
   polylines of [x, y] points, dots (junctions), terminals (open circles), arrows ({x, y, dir} with dir in degrees,
   0 = right, 90 = down) and texts ({x, y, text, anchor}). `anchors` ({var, x, y, side}) is read by the calculator
   page, not drawn here. */
(function () {
    'use strict';

    const G = 20;
    const escape = text => String(text).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
    const num = x => String(Math.round(x * 100) / 100);
    const path = d => '<path d="' + d + '"/>';
    const solid = d => '<path d="' + d + '" fill="currentColor" stroke-width="1"/>';

    // A filled arrow head whose tip is at (x, y) and that points along (dx, dy).
    function head(x, y, dx, dy, length, half) {
        const size = Math.hypot(dx, dy), ux = dx / size, uy = dy / size, bx = x - length * ux, by = y - length * uy;
        return solid('M' + num(x) + ' ' + num(y) + 'L' + num(bx - half * uy) + ' ' + num(by + half * ux) + 'L' + num(bx + half * uy) + ' ' + num(by - half * ux) + 'Z');
    }

    const twoPins = () => ({ a: [0, 0], b: [3, 0] });
    const threePins = () => ({ b: [0, 0], c: [3, -2], e: [3, 2] });
    const fetPins = () => ({ g: [0, 0], d: [3, -2], s: [3, 2] });
    const diodeBase = () => path('M0 0H22M38 0H60') + path('M22 -10L38 0L22 10Z') + path('M38 -10V10');
    const plus = (x, y) => path('M' + (x - 3) + ' ' + y + 'h6M' + x + ' ' + (y - 3) + 'v6');
    // Markup centred on (x, y) that stays upright when the part is rotated or flipped (the signs of a source, a letter).
    const upright = (part, x, y, inner) => '<g transform="translate(' + x + ' ' + y + ')' + (part.rot ? ' rotate(' + -part.rot + ')' : '') + (part.flip ? ' scale(1 -1)' : '') + '">' + inner + '</g>';

    const SYMBOLS = {
        resistor: {
            pins: twoPins,
            draw: standard => (standard === 'ansi'
                ? path('M0 0H10L14 -8L22 8L30 -8L38 8L46 -8L50 0H60')
                : path('M0 0H10M50 0H60') + '<rect x="10" y="-7" width="40" height="14"/>')
        },
        capacitor: { pins: twoPins, draw: () => path('M0 0H27M33 0H60') + path('M27 -12V12M33 -12V12') },
        polarized: {
            pins: twoPins,
            draw: standard => path('M0 0H27M37 0H60') + path('M27 -12V12') + (standard === 'ansi' ? path('M33 -12Q41 0 33 12') : '<rect x="33" y="-12" width="4" height="24"/>') + plus(19, -9)
        },
        inductor: { pins: twoPins, draw: () => path('M0 0H10M50 0H60') + path('M10 0a5 5 0 0 1 10 0a5 5 0 0 1 10 0a5 5 0 0 1 10 0a5 5 0 0 1 10 0') },
        diode: { pins: twoPins, draw: diodeBase },
        led: { pins: twoPins, draw: () => diodeBase() + path('M26 -14L36 -26') + head(36, -26, 10, -12, 7, 3) + path('M35 -10L45 -22') + head(45, -22, 10, -12, 7, 3) },
        zener: { pins: twoPins, draw: () => path('M0 0H22M38 0H60') + path('M22 -10L38 0L22 10Z') + path('M42 -14L38 -10V10L34 14') },
        crystal: { pins: twoPins, draw: () => path('M0 0H22M38 0H60') + path('M22 -12V12M38 -12V12') + '<rect x="26" y="-8" width="8" height="16"/>' },
        fuse: { pins: twoPins, draw: () => path('M0 0H10M50 0H60') + '<rect x="10" y="-7" width="40" height="14"/>' + path('M10 0H50') },
        // SPST switch, drawn open: pins a (0,0) and b (3,0); the blade lifts off the contact at b.
        switch: {
            pins: twoPins,
            draw: () => path('M0 0H15M45 0H60') + path('M15 0L42 -15') + '<circle cx="15" cy="0" r="2.5"/><circle cx="45" cy="0" r="2.5"/>'
        },
        // Potentiometer, four grid units wide: pins a (0,0), b (4,0) and the wiper w (2,-2) above the body (flip puts it below).
        pot: {
            pins: () => ({ a: [0, 0], b: [4, 0], w: [2, -2] }),
            draw: standard => (standard === 'ansi'
                ? path('M0 0H10L16 -8L28 8L40 -8L52 8L64 -8L70 0H80')
                : path('M0 0H10M70 0H80') + '<rect x="10" y="-7" width="60" height="14"/>') + path('M40 -40V-16') + head(40, -8, 0, 1, 8, 3.5)
        },
        source: { pins: twoPins, draw: (standard, part) => path('M0 0H16M44 0H60') + '<circle cx="30" cy="0" r="14"/>' + upright(part, 22, 0, plus(0, 0)) + upright(part, 38, 0, path('M-3 0h6')) },
        acsource: { pins: twoPins, draw: () => path('M0 0H16M44 0H60') + '<circle cx="30" cy="0" r="14"/>' + path('M21 0Q25.5 -9 30 0T39 0') },
        isource: { pins: twoPins, draw: () => path('M0 0H16M44 0H60') + '<circle cx="30" cy="0" r="14"/>' + path('M21 0H38') + head(40, 0, 1, 0, 7, 4) },
        battery: { pins: twoPins, draw: () => path('M0 0H20M40 0H60') + path('M20 -12V12M26 -7V7M34 -12V12M40 -7V7') },
        meter: { pins: twoPins, draw: (standard, part) => path('M0 0H16M44 0H60') + '<circle cx="30" cy="0" r="14"/>' + upright(part, 30, 0, '<text x="0" y="4.5" text-anchor="middle" class="ec-sch-text" stroke="none" fill="currentColor">' + escape(part.text || 'V') + '</text>') },
        ground: { pins: () => ({ g: [0, 0] }), draw: () => path('M0 0V10M-12 10H12M-8 15H8M-4 20H4') },
        antenna: { pins: () => ({ a: [0, 0] }), draw: () => path('M0 0V-26M-13 -44L0 -26L13 -44') },
        npn: {
            pins: threePins,
            draw: () => path('M0 0H34M34 -16V16M34 -8L60 -24V-40M34 8L60 24V40') + head(56, 21.5, 26, 16, 8, 3.5)
        },
        pnp: {
            pins: threePins,
            draw: () => path('M0 0H34M34 -16V16M34 -8L60 -24V-40M34 8L60 24V40') + head(40, 12.3, -26, -16, 8, 3.5)
        },
        nmos: {
            pins: fetPins,
            draw: () => path('M0 0H24M24 -14V14M30 -16V-9M30 -4V4M30 9V16M30 -12H60V-40M30 12H60V40M30 0H46V12') + head(32, 0, -1, 0, 7, 3.5)
        },
        pmos: {
            pins: fetPins,
            draw: () => path('M0 0H24M24 -14V14M30 -16V-9M30 -4V4M30 9V16M30 -12H60V-40M30 12H60V40M30 0H46V12') + head(44, 0, 1, 0, 7, 3.5)
        },
        opamp: {
            pins: () => ({ inn: [0, -1], inp: [0, 1], out: [5, 0] }),
            draw: () => path('M20 -40L20 40L100 0Z') + path('M0 -20H20M0 20H20') + path('M26 -20h8') + plus(30, 20)
        },
        transformer: {
            pins: () => ({ p1: [0, 0], p2: [0, 3], s1: [4, 0], s2: [4, 3] }),
            draw: () => path('M0 0H22M0 60H22M80 0H58M80 60H58') + path('M22 0a7.5 7.5 0 0 0 0 15a7.5 7.5 0 0 0 0 15a7.5 7.5 0 0 0 0 15a7.5 7.5 0 0 0 0 15')
                + path('M58 0a7.5 7.5 0 0 1 0 15a7.5 7.5 0 0 1 0 15a7.5 7.5 0 0 1 0 15a7.5 7.5 0 0 1 0 15') + path('M36 0V60M44 0V60')
        },
        // A generic IC box: part.w and part.h in grid units (the box without its pin stubs), part.pins [{n, s: 'l'|'r', y}]
        // with y in grid units from the top edge, part.text the title. Pin points sit at the outer ends of the stubs.
        ic: {
            pins: part => {
                const out = {};
                for (const pin of part.pins || []) out[pin.n] = [pin.s === 'r' ? part.w + 2 : 0, pin.y];
                return out;
            },
            draw: (standard, part) => {
                let markup = '<rect x="' + G + '" y="0" width="' + part.w * G + '" height="' + part.h * G + '"/>';
                for (const pin of part.pins || []) {
                    const right = pin.s === 'r', y = pin.y * G;
                    markup += path(right ? 'M' + (part.w + 1) * G + ' ' + y + 'H' + (part.w + 2) * G : 'M0 ' + y + 'H' + G);
                    markup += '<text x="' + num(right ? (part.w + 1) * G - 4 : G + 4) + '" y="' + num(y + 3.5) + '" text-anchor="' + (right ? 'end' : 'start') + '" class="ec-sch-pin" stroke="none" fill="currentColor">' + escape(pin.n) + '</text>';
                }
                if (part.text) markup += '<text x="' + num((part.w + 2) * G / 2) + '" y="-6" text-anchor="middle" class="ec-sch-text" stroke="none" fill="currentColor">' + escape(part.text) + '</text>';
                return markup;
            }
        }
    };

    // Label text with a subscript behind the first underscore: V_out -> V<tspan>out</tspan>.
    function textMarkup(text) {
        const label = String(text), at = label.indexOf('_');
        if (at <= 0) return escape(label);
        return escape(label.slice(0, at)) + '<tspan class="ec-sch-sub" dy="3">' + escape(label.slice(at + 1)) + '</tspan>';
    }

    function partMarkup(part, standard) {
        const symbol = SYMBOLS[part.sym];
        if (!symbol) return '';
        const transform = 'translate(' + num(part.x * G) + ' ' + num(part.y * G) + ')' + (part.rot ? ' rotate(' + part.rot + ')' : '') + (part.flip ? ' scale(1 -1)' : '');
        return '<g transform="' + transform + '">' + symbol.draw(standard, part) + '</g>';
    }

    // The grid position of a pin of a placed part (rotation and flip applied); used by tests and by schematic authors.
    function pinPoint(part, name) {
        const symbol = SYMBOLS[part.sym], pin = symbol && symbol.pins(part)[name];
        if (!pin) return null;
        const radians = (part.rot || 0) * Math.PI / 180, x = pin[0], y = part.flip ? -pin[1] : pin[1];
        const round = value => Math.round(value * 1e6) / 1e6;
        return [round(part.x + x * Math.cos(radians) - y * Math.sin(radians)), round(part.y + x * Math.sin(radians) + y * Math.cos(radians))];
    }

    // Inline SVG of a schematic spec. `standard` is 'iec' (default) or 'ansi'; `label` becomes the accessible name.
    function svg(spec, options) {
        const standard = options && options.standard === 'ansi' ? 'ansi' : 'iec', label = (options && options.label) || '';
        const width = spec.w * G, height = spec.h * G;
        const point = p => num(p[0] * G) + ' ' + num(p[1] * G);
        let body = '';
        for (const wire of spec.wires || []) body += path('M' + wire.map(point).join('L'));
        for (const part of spec.parts || []) body += partMarkup(part, standard);
        for (const dot of spec.dots || []) body += '<circle class="ec-dot" cx="' + num(dot[0] * G) + '" cy="' + num(dot[1] * G) + '" r="3.2" fill="currentColor" stroke="none"/>';
        for (const end of spec.terminals || []) body += '<circle class="ec-term" cx="' + num(end[0] * G) + '" cy="' + num(end[1] * G) + '" r="4"/>';
        for (const arrow of spec.arrows || []) {
            const radians = (arrow.dir || 0) * Math.PI / 180;
            body += head(arrow.x * G + 4.5 * Math.cos(radians), arrow.y * G + 4.5 * Math.sin(radians), Math.cos(radians), Math.sin(radians), 9, 4);
        }
        for (const text of spec.texts || []) {
            body += '<text class="ec-sch-text" x="' + num(text.x * G) + '" y="' + num(text.y * G) + '" text-anchor="' + escape(text.anchor || 'middle') + '" stroke="none" fill="currentColor">' + textMarkup(text.text) + '</text>';
        }
        return '<svg class="ec-sch" viewBox="0 0 ' + width + ' ' + height + '" role="img" aria-label="' + escape(label) + '" focusable="false" preserveAspectRatio="xMidYMid meet" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round">' + body + '</svg>';
    }

    window.ECalcSymbols = { GRID: G, names: () => Object.keys(SYMBOLS), pins: (sym, part) => (SYMBOLS[sym] ? SYMBOLS[sym].pins(part || {}) : null), pinPoint, symbolMarkup: (part, standard) => partMarkup(part, standard), svg };
})();
