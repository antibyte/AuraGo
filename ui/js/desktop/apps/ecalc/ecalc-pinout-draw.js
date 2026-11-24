/* ECalc pinout drawings: the connector, the board header or the IC package that a pinout page shows. A DOM-free classic script
   that publishes window.ECalcPinoutDraw and builds inline SVG strings, like ecalc-code-draw.js does. The fill of a contact is
   the `hex` of its pin type and its number is written in the `ink` of that type (both come from the catalog data, as
   attributes: a power pin stays red in every desktop skin). The shells are made of materials (sheet steel, plastics, epoxy,
   circuit board): a part names its material and ecalc-materials.js (window.ECalcMaterials, loaded first) turns the name into a
   gradient of the drawing's own scope. A material reads the same in every skin; what keeps the part legible on every skin is
   the thin edge line in the text token (ec-shell-edge) that follows each silhouette. The leaders, the names and the numbers
   that lie on the bare stage take the text token through the CSS (class ec-on-stage) and the classes ec-pin-* and ec-shell-* of css/desktop-app-ecalc.css (tokens only).

   ECalcPinoutDraw.layout(shape, pins, {view}) -> the geometry of the drawing, plain data without markup:
     {type, view, mirrored, width, height, shellBox: [x0, y0, x1, y1], face,
      shell: [{cls, d, at?, mat?, across?, fx?, edge?}], texts: [{x, y, text, cls, anchor, on?}],
      contacts: [{number, name, type, wire, shape: 'circle' | 'rect' | 'path', x, y, w, h, r, d, kind, mark, on,
                  num: {mode: 'in' | 'out', x, y, fs}, label: {x, y, anchor, rotate} | null,
                  leader: {x1, y1, x2, y2} | null, stub: {x, y, w, h} | null}]}   (stub: the rectangle of the wire end)
     A shell part: mat is the name of its material (none: the class styles it, as the holes show the stage), across 'x' asks for
     the gradient that runs left to right, fx marks a decorative layer (shadow, bevel), edge asks for the edge line, at is
     [x, y, degrees] for a part drawn turned about its own origin. face is the material that lies under the contacts ('' when they
     sit on the bare stage); on is that surface under the mark and the outside number of one contact (and under a text): the
     print on a material takes the ink of that material, on the bare stage the text token. under is a different question: the
     material that lies under the WHOLE contact ('' when it is on the stage or straddles an edge, as the pads on the edge of a
     board do); the rim of the contact takes the ink of that material (stroke attribute, data-on), else the text token (the class
     ec-rim-stage). The shell of a polygon outline keeps 2 units off every contact and every outside number (their text boxes).
     shape  the `shape` of the catalog JSON (see internal/ecalc/pinouts.go); pins the pins of the shown variant (number, name, type, wire).
     view   'plug' or 'socket'; only the shapes `rows` and `circular` have two views. The view that is not `shape.drawn` is
            the mirror image about the vertical axis: every x of a contact, a number, a name, a leader and a wire stub becomes
            width - x, the names change their side (start <-> end) and no text is ever flipped. Only the shell (paths without
            text) is mirrored by the transform of its own group. A contact keeps its height. One exception: the tongue of the `usb_a`
            connector depends on the view (it is not simply mirrored).
   ECalcPinoutDraw.svg(spec) -> SVG markup. spec: {layout, types: {id: {hex, ink}}, wires: {id: {hex, stripe}}, label, uid}. uid
     makes the gradient ids unique (several windows can show the same drawing); the same spec gives the same markup. The first
     child is the <defs> of the materials. One g.ec-pin per contact (data-pin, data-type, data-num="in" | "out"); the page toggles
     data-hover, data-selected and data-dim on it. Over the body of a contact (the fill of its type) lies a soft ec-pin-sheen.
   The shape `to` draws four packages: to92 and to220 seen from the front (leads down), sot23 and sot223 seen from above (marking
     side up, `drawn` is 'top'): epoxy body, tin gull-wing leads as filled shapes, the feet as contacts, a pin-1 dot (recess); the
     tab of the sot223 is the last pin and is drawn in tin at the top.
   ECalcPinoutDraw.OUTLINES -> the sixteen outlines of `rows` (the list of ecalc.PinoutOutlines).
   ECalcPinoutDraw.fitNumber(text, kind, width, height) -> the font size (11, 10 or 9) at which the number fits inside a contact
     of that kind, or 0 when it does not: the page and the tests use the same rule as the drawing.

   The rule for the numbers: a number is written inside its contact at the largest of 11, 10 and 9 units at which its estimated
   width (digits 0.6 em, capitals 0.72 em, narrow signs 0.36 em) fits the contact (a round contact: 84 percent of its diameter,
   any other: width - 4) and its height fits too. When no size fits (a blade of a USB-C contact, an HDMI contact) the number moves
   out of the contact: above the first row, below the last row (a single row: above). The real browser run confirms the estimate. */
(function () {
    'use strict';

    const escape = text => String(text == null ? '' : text).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
    const HEX = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/;
    const safeColor = text => (typeof text === 'string' && HEX.test(text) ? text : '');
    const round = value => Math.round(value * 100) / 100;

    const OUTLINES = ['displayport', 'dsub', 'hdmi_a', 'jst', 'minifit', 'molex', 'obd2', 'rect', 'rj45', 'sata_data', 'sata_power', 'usb_a', 'usb_b', 'usb_c', 'usb_micro_b', 'usb_mini_b'];
    const MARGIN = 10, NAME_FS = 11, OUT_FS = 10, STUB = 14, OUT_ROOM = 13;
    const FALLBACK_HEX = '#9e9e9e', FALLBACK_INK = '#000000';

    // ---- text and path helpers --------------------------------------------------------------------------------------------

    function charWidth(ch) {
        if (/[0-9]/.test(ch)) return 0.6;
        if (/[A-Z]/.test(ch)) return 0.72;
        if (/[mwMW@%]/.test(ch)) return 0.9;
        if (/[iljtf.,:;|!'() ]/.test(ch)) return 0.36;
        if (ch === '-') return 0.4;
        return 0.62;
    }
    const textWidth = (text, fs) => [...String(text)].reduce((sum, ch) => sum + charWidth(ch), 0) * fs;
    const points = list => 'M' + list.map(point => round(point[0]) + ' ' + round(point[1])).join('L') + 'Z';
    function roundRect(x, y, w, h, r) {
        r = Math.max(0, Math.min(r, w / 2, h / 2));
        return 'M' + round(x + r) + ' ' + round(y) + 'H' + round(x + w - r) + 'A' + round(r) + ' ' + round(r) + ' 0 0 1 ' + round(x + w) + ' ' + round(y + r)
            + 'V' + round(y + h - r) + 'A' + round(r) + ' ' + round(r) + ' 0 0 1 ' + round(x + w - r) + ' ' + round(y + h) + 'H' + round(x + r)
            + 'A' + round(r) + ' ' + round(r) + ' 0 0 1 ' + round(x) + ' ' + round(y + h - r) + 'V' + round(y + r) + 'A' + round(r) + ' ' + round(r) + ' 0 0 1 ' + round(x + r) + ' ' + round(y) + 'Z';
    }
    const circle = (cx, cy, r) => 'M' + round(cx - r) + ' ' + round(cy) + 'a' + round(r) + ' ' + round(r) + ' 0 1 0 ' + round(2 * r) + ' 0a' + round(r) + ' ' + round(r) + ' 0 1 0 ' + round(-2 * r) + ' 0Z';

    // ---- shell parts ------------------------------------------------------------------------------------------------------

    // A silhouette (material and edge line), a plain layer inside it (material, no edge) and the cast shadow of a silhouette: its
    // own outline, 3 units lower, in the shadow material.
    const solid = (cls, d, mat, extra) => Object.assign({ cls, d, mat, edge: true }, extra);
    const layer = (cls, d, mat, extra) => Object.assign({ cls, d, mat }, extra);
    const cast = part => ({ cls: 'ec-shell-shadow', d: part.d, at: [0, 3, 0], mat: 'shadow', fx: true });

    // The openings of a convex polygon: the polygon moved inward by k on every side (each edge to its parallel, the corners where
    // the new edges meet), and the clear distance between the polygon and the nearest corner of a contact rectangle [x0, y0, x1, y1].
    const turn = poly => (poly.reduce((sum, p, i) => sum + p[0] * poly[(i + 1) % poly.length][1] - poly[(i + 1) % poly.length][0] * p[1], 0) > 0 ? 1 : -1);
    function insetPoly(poly, k) {
        const n = poly.length, s = turn(poly);
        const lines = poly.map((p, i) => {
            const q = poly[(i + 1) % n], dx = q[0] - p[0], dy = q[1] - p[1], len = Math.hypot(dx, dy) || 1;
            return { x: p[0] - s * k * dy / len, y: p[1] + s * k * dx / len, dx, dy };
        });
        return lines.map((b, i) => {
            const a = lines[(i + n - 1) % n], det = a.dx * b.dy - a.dy * b.dx;
            if (Math.abs(det) < 1e-9) return [b.x, b.y];
            const t = ((b.x - a.x) * b.dy - (b.y - a.y) * b.dx) / det;
            return [a.x + t * a.dx, a.y + t * a.dy];
        });
    }
    function roomIn(poly, cells) {
        const n = poly.length, s = turn(poly);
        let room = Infinity;
        for (const [x0, y0, x1, y1] of cells) for (const [x, y] of [[x0, y0], [x1, y0], [x1, y1], [x0, y1]]) {
            poly.forEach((p, i) => {
                const q = poly[(i + 1) % n], dx = q[0] - p[0], dy = q[1] - p[1], len = Math.hypot(dx, dy) || 1;
                room = Math.min(room, ((x - p[0]) * -s * dy + (y - p[1]) * s * dx) / len);
            });
        }
        return room;
    }
    // The opening inside a polygon shell, `want` units from its rim, never closer than 2 units to a contact; '' when it would be
    // thinner than one unit (the contacts then sit on the shell itself).
    function opening(poly, cells, want) {
        const k = Math.min(want, roomIn(poly, cells) - 2);
        return k >= 1 ? points(insetPoly(poly, k)) : '';
    }

    // The font size at which `text` fits inside a contact, 0 when none of 11, 10 and 9 does.
    function fitNumber(text, kind, width, height) {
        for (const fs of [11, 10, 9]) {
            const room = kind === 'round' ? 0.84 * Math.min(width, height) : width - 4;
            if (textWidth(text, fs) <= room && fs + 2 <= height) return fs;
        }
        return 0;
    }

    // ---- contact records --------------------------------------------------------------------------------------------------

    const pinMap = pins => new Map((Array.isArray(pins) ? pins : []).map(pin => [String(pin.number), pin]));
    const given = value => value !== null && value !== undefined && value !== '';
    function contact(map, number, geometry) {
        const pin = map.get(String(number)) || {};
        return Object.assign({
            number: String(number), name: pin.name || '', type: pin.type || 'nc', wire: pin.wire || '', shape: 'rect', kind: 'square', x: 0, y: 0, w: 0, h: 0, r: 2, d: '',
            mark: false, num: { mode: 'in', x: 0, y: 0, fs: OUT_FS }, label: null, leader: null, stub: null
        }, geometry);
    }
    // A number inside the contact at its centre.
    function inside(c, fs) {
        c.num = { mode: 'in', x: c.x, y: c.y + fs * 0.36, fs };
        return c;
    }
    // The first contact in drawing order that is pin "1", else the first one, carries the mark.
    function markFirst(contacts) {
        if (!contacts.length) return;
        const first = contacts.find(c => c.number === '1') || contacts[0];
        first.mark = true;
    }

    // ---- rows ---------------------------------------------------------------------------------------------------------------

    // p: pitch, rp: row pitch, padX / padT / padB: space between the contacts and the shell, bladeH: height of a blade contact,
    // cap: the largest round or square contact.
    const ROW_STYLE = {
        rect: { p: 26, rp: 26, padX: 10, padT: 10, padB: 10, bladeH: 18, cap: 21 },
        dsub: { p: 26, rp: 28, padX: 14, padT: 14, padB: 14, bladeH: 18, cap: 21, ears: 22 },
        usb_a: { p: 26, rp: 24, padX: 12, padT: 12, padB: 24, bladeH: 16, cap: 21 },
        usb_b: { p: 26, rp: 26, padX: 14, padT: 14, padB: 12, bladeH: 16, cap: 21 },
        usb_mini_b: { p: 24, rp: 22, padX: 14, padT: 12, padB: 12, bladeH: 16, cap: 21 },
        usb_micro_b: { p: 24, rp: 22, padX: 14, padT: 12, padB: 12, bladeH: 14, cap: 21 },
        usb_c: { p: 22, rp: 26, padX: 26, padT: 10, padB: 10, bladeH: 16, cap: 20 },
        hdmi_a: { p: 22, rp: 26, padX: 18, padT: 10, padB: 10, bladeH: 16, cap: 20 },
        displayport: { p: 22, rp: 26, padX: 16, padT: 10, padB: 10, bladeH: 16, cap: 20 },
        rj45: { p: 22, rp: 26, padX: 12, padT: 10, padB: 20, bladeH: 22, cap: 20 },
        sata_data: { p: 22, rp: 24, padX: 10, padT: 8, padB: 18, bladeH: 14, cap: 20 },
        sata_power: { p: 22, rp: 24, padX: 10, padT: 8, padB: 18, bladeH: 14, cap: 20 },
        molex: { p: 34, rp: 34, padX: 14, padT: 12, padB: 16, bladeH: 20, cap: 26 },
        minifit: { p: 30, rp: 30, padX: 12, padT: 16, padB: 12, bladeH: 20, cap: 24 },
        obd2: { p: 26, rp: 28, padX: 16, padT: 12, padB: 12, bladeH: 16, cap: 21 },
        jst: { p: 26, rp: 26, padX: 12, padT: 14, padB: 12, bladeH: 16, cap: 21 }
    };

    function contactDims(kind, style) {
        if (kind === 'blade') return { w: Math.min(14, Math.max(9, style.p * 0.55)), h: style.bladeH };
        const size = Math.min(style.p - 5, style.cap);
        return { w: size, h: size };
    }

    // The shell of every outline, in the coordinates of its own box b = {x, y, w, h}; ctx.rowsBox holds the contact extents per
    // row (left edge, right edge, bottom edge, relative to the box). A trapezoid never cuts a contact: slantInset limits its slant.
    function slantInset(ctx, tilt) {
        let inset = tilt;
        for (const row of ctx.rowsBox) {
            if (row.b <= 0) continue;
            inset = Math.min(inset, (row.l - 5) * ctx.h / row.b, (ctx.w - row.r - 5) * ctx.h / row.b);
        }
        return Math.max(0, inset);
    }
    // Every builder returns {parts, face}: the parts bottom to top (cast shadow first) and the material that the contacts lie on.
    // ctx.cells holds the rectangles of the contacts [x0, y0, x1, y1] in the same coordinates, so that an opening in a metal shell
    // keeps 2 units away from them (and is left out when it cannot).
    const STEEL = 'steel';
    // One silhouette of one material and nothing else.
    const plain = (mat, d, extra) => {
        const main = solid('ec-shell', d, mat);
        return { parts: [cast(main), ...(extra && extra.before || []), main, ...(extra && extra.after || [])], face: mat };
    };
    // The cut or slant of a polygon outline is nominal at t = 1 and shrinks (in steps of 5 percent) until the outline keeps clear of every
    // contact and every outside number (ctx.cells) by 2 units plus the inset of the opening that goes inside it (3 at most).
    function fitCut(make, ctx, want) {
        for (let t = 1; t > 0.001; t -= 0.05) {
            const poly = make(t);
            if (roomIn(poly, ctx.cells) >= 2 + Math.min(want || 0, 3)) return poly;
        }
        return make(0);
    }
    // A metal shell with the opening of the receptacle inside it (the face is the opening, or the metal when there is no room).
    function metal(make, ctx, want, inner) {
        const poly = fitCut(make, ctx, want), main = solid('ec-shell', points(poly), STEEL), open = opening(poly, ctx.cells, want);
        return { parts: [cast(main), main, ...(open ? [layer('ec-shell-cavity', open, inner)] : [])], face: open ? inner : STEEL };
    }
    const SHELLS = {
        rect(b, ctx) {
            const key = ctx.key, cx = b.x + b.w / 2, cy = b.y + b.h / 2, n = 9, depth = 7;
            if (key === 'top') return plain('pbt', points([[b.x, b.y], [cx - n, b.y], [cx - n, b.y + depth], [cx + n, b.y + depth], [cx + n, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h], [b.x, b.y + b.h]]));
            if (key === 'bottom') return plain('pbt', points([[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h], [cx + n, b.y + b.h], [cx + n, b.y + b.h - depth], [cx - n, b.y + b.h - depth], [cx - n, b.y + b.h], [b.x, b.y + b.h]]));
            if (key === 'left') return plain('pbt', points([[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h], [b.x, b.y + b.h], [b.x, cy + n], [b.x + depth, cy + n], [b.x + depth, cy - n], [b.x, cy - n]]));
            if (key === 'right') return plain('pbt', points([[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, cy - n], [b.x + b.w - depth, cy - n], [b.x + b.w - depth, cy + n], [b.x + b.w, cy + n], [b.x + b.w, b.y + b.h], [b.x, b.y + b.h]]));
            return plain('pbt', roundRect(b.x, b.y, b.w, b.h, 4));
        },
        // The D shell with its mounting flange: the wide row on top, sides leaning 10 degrees, two screw holes (each on a dark
        // recess, the flange is sheet steel). Inside the steel rim lies the black insulator that carries the contacts.
        dsub(b, ctx) {
            const inset = slantInset(ctx, b.h * 0.18), ears = ctx.ears, cy = b.y + b.h / 2;
            const poly = fitCut(t => [[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w - inset * t, b.y + b.h], [b.x + inset * t, b.y + b.h]], ctx, 4);
            const flange = solid('ec-shell-flange', roundRect(b.x - ears, b.y - 4, b.w + 2 * ears, b.h + 8, 5), STEEL), open = opening(poly, ctx.cells, 4);
            const hole = x => [layer('ec-shell-detail', circle(x, cy, 5.8), 'recess'), { cls: 'ec-shell-hole', d: circle(x, cy, 4.2) }];
            return {
                parts: [cast(flange), flange, ...hole(b.x - ears / 2), ...hole(b.x + b.w + ears / 2), solid('ec-shell', points(poly), STEEL), ...(open ? [layer('ec-shell-insulator', open, 'pbt')] : [])],
                face: open ? 'pbt' : STEEL
            };
        },
        // A steel shell around the insulating tongue. The two tongues meet at the contacts: the tongue of the receptacle fills the
        // upper half and the contacts sit under it, the tongue of the plug fills the lower half and the contacts sit on it. In the
        // plug view the contacts lie 4 units over the tongue's edge only, so the face there is the dark opening.
        usb_a(b, ctx) {
            const last = ctx.rowsBox.length ? ctx.rowsBox[ctx.rowsBox.length - 1].b : b.h / 2;
            const tongue = ctx.view === 'plug' ? roundRect(b.x + 5, b.y + last - 4, b.w - 10, b.h - last - 1, 2) : roundRect(b.x + 5, b.y + 5, b.w - 10, last + 4 - 5, 2);
            const main = solid('ec-shell', roundRect(b.x, b.y, b.w, b.h, 3), STEEL);
            return { parts: [cast(main), main, layer('ec-shell-cavity', roundRect(b.x + 4, b.y + 4, b.w - 8, b.h - 8, 2), 'cavity'), layer('ec-shell-tongue', tongue, 'nylon')], face: ctx.view === 'plug' ? 'cavity' : 'nylon' };
        },
        // The square B plug: the two upper corners cut.
        usb_b(b, ctx) {
            return metal(t => [[b.x, b.y + 12 * t], [b.x + 12 * t, b.y], [b.x + b.w - 12 * t, b.y], [b.x + b.w, b.y + 12 * t], [b.x + b.w, b.y + b.h], [b.x, b.y + b.h]], ctx, 4, 'cavity');
        },
        // Mini B: a trapezoid, wide side up.
        usb_mini_b(b, ctx) {
            const inset = slantInset(ctx, b.h * 0.22);
            const out = plain(STEEL, points(fitCut(t => [[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w - inset * t, b.y + b.h], [b.x + inset * t, b.y + b.h]], ctx)), { after: [layer('ec-shell-tongue', roundRect(b.x + 8, b.y + 5, b.w - 16, b.h - 10, 2), 'pbt')] });
            return Object.assign(out, { face: 'pbt' });
        },
        // Micro B: flat and wide, both lower corners cut.
        usb_micro_b(b, ctx) {
            const out = plain(STEEL, points(fitCut(t => [[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h - 9 * t], [b.x + b.w - 9 * t, b.y + b.h], [b.x + 9 * t, b.y + b.h], [b.x, b.y + b.h - 9 * t]], ctx)), { after: [layer('ec-shell-tongue', roundRect(b.x + 8, b.y + 5, b.w - 16, b.h - 10, 2), 'pbt')] });
            return Object.assign(out, { face: 'pbt' });
        },
        // Type C: the stadium around a centre tongue between the two rows.
        usb_c(b) {
            const main = solid('ec-shell', roundRect(b.x, b.y, b.w, b.h, b.h / 2), STEEL);
            return { parts: [cast(main), main, layer('ec-shell-cavity', roundRect(b.x + 3, b.y + 3, b.w - 6, b.h - 6, b.h / 2 - 3), 'cavity'), layer('ec-shell-tongue', roundRect(b.x + 14, b.y + b.h / 2 - 3, b.w - 28, 6, 3), 'pbt')], face: 'cavity' };
        },
        // Type A: flat on top, the lower corners cut at an angle.
        hdmi_a(b, ctx) {
            const c = b.h * 0.38;
            return metal(t => [[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h * 0.4], [b.x + b.w - c * t, b.y + b.h], [b.x + c * t, b.y + b.h], [b.x, b.y + b.h * 0.4]], ctx, 3, 'cavity');
        },
        // One cut corner, the lower left.
        displayport(b, ctx) {
            const c = 14;
            return metal(t => [[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h], [b.x + c * t, b.y + b.h], [b.x, b.y + b.h - c * t]], ctx, 3, 'cavity');
        },
        // 8P8C: contacts at the top, the latch slot in the bottom edge.
        rj45(b) {
            const cx = b.x + b.w / 2, half = Math.max(10, b.w * 0.17), depth = 10;
            return plain('grey', points([[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h], [cx + half, b.y + b.h], [cx + half, b.y + b.h - depth], [cx - half, b.y + b.h - depth], [cx - half, b.y + b.h], [b.x, b.y + b.h]]));
        },
        // SATA data plug: the L, the step at the right.
        sata_data(b) {
            const kw = b.w * 0.2, step = 14;
            return plain('pbt', points([[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h - step], [b.x + b.w - kw, b.y + b.h - step], [b.x + b.w - kw, b.y + b.h], [b.x, b.y + b.h]]));
        },
        // SATA power plug: the same L, the step at the left.
        sata_power(b) {
            const kw = b.w * 0.14, step = 14;
            return plain('pbt', points([[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h], [b.x + kw, b.y + b.h], [b.x + kw, b.y + b.h - step], [b.x, b.y + b.h - step]]));
        },
        // Peripheral power: two cut corners on one long side.
        molex(b, ctx) {
            const c = 14;
            return plain('nylon', points(fitCut(t => [[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h - c * t], [b.x + b.w - c * t, b.y + b.h], [b.x + c * t, b.y + b.h], [b.x, b.y + b.h - c * t]], ctx)));
        },
        // Mini-Fit: the latch on top, small cuts at the lower corners.
        minifit(b, ctx) {
            const cx = b.x + b.w / 2, c = 7;
            return plain('pbt', points(fitCut(t => [[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h - c * t], [b.x + b.w - c * t, b.y + b.h], [b.x + c * t, b.y + b.h], [b.x, b.y + b.h - c * t]], ctx)), { before: [solid('ec-shell-latch', roundRect(cx - 14, b.y - 7, 28, 10, 2), 'pbt')] });
        },
        // The trapezoid of the diagnostic socket with the small latch on the wide side.
        obd2(b, ctx) {
            const inset = slantInset(ctx, b.h * 0.26), cx = b.x + b.w / 2;
            return plain('pbt', points(fitCut(t => [[b.x, b.y], [b.x + b.w, b.y], [b.x + b.w - inset * t, b.y + b.h], [b.x + inset * t, b.y + b.h]], ctx)), { before: [solid('ec-shell-latch', roundRect(cx - 10, b.y - 6, 20, 9, 2), 'pbt')] });
        },
        // A small housing: the locking ramp on top and the slot around the contacts.
        jst(b) {
            const cx = b.x + b.w / 2;
            const out = plain('nylon', points([[b.x, b.y], [cx - 12, b.y], [cx - 6, b.y - 6], [cx + 6, b.y - 6], [cx + 12, b.y], [b.x + b.w, b.y], [b.x + b.w, b.y + b.h], [b.x, b.y + b.h]]), { after: [layer('ec-shell-tongue', roundRect(b.x + 5, b.y + 6, b.w - 10, b.h - 12, 2), 'cavity')] });
            return Object.assign(out, { face: 'cavity' });
        }
    };

    function layoutRows(shape, map, view) {
        const style = ROW_STYLE[shape.outline] || ROW_STYLE.rect;
        const kind = shape.contact === 'blade' || shape.contact === 'square' ? shape.contact : 'round';
        const dims = contactDims(kind, style), p = style.p, rp = style.rp;
        const rows = (Array.isArray(shape.rows) ? shape.rows : []).map(row => ({ pins: Array.isArray(row.pins) ? row.pins : [], offset: Number(row.offset) || 0 }));
        const count = rows.length;
        const cols = Math.max(1, ...rows.map(row => row.pins.length + row.offset));
        const named = number => (map.get(String(number)) || {}).name || '';
        const namesMode = count > 0 && rows.every(row => row.pins.length <= 2) && rows.some(row => row.pins.some(number => given(number) && named(number)));

        // 1. the numbers and the wire stubs decide the space above and below
        const records = [];
        let outsideTop = false, outsideBottom = false, stubTop = false, stubBottom = false, stubLeft = false;
        // A single column (a servo, a Qwiic plug) has its wires leave to the left, where no name stands.
        const column = count > 0 && rows.every(row => row.pins.length === 1);
        // One rule per drawing: when a number does not fit inside its contact, the numbers of the outer rows all go outside.
        const allFit = rows.every(row => row.pins.every(number => !given(number) || fitNumber(String(number), kind, dims.w, dims.h)));
        rows.forEach((row, r) => row.pins.forEach((number, c) => {
            if (!given(number)) return;
            const fs = allFit ? fitNumber(String(number), kind, dims.w, dims.h) : 0;
            const side = fs ? '' : (r === 0 ? 'top' : r === count - 1 ? 'bottom' : '');
            if (side === 'top') outsideTop = true;
            if (side === 'bottom') outsideBottom = true;
            const wire = (map.get(String(number)) || {}).wire;
            const stub = !wire ? '' : column ? 'left' : r === count - 1 ? 'down' : r === 0 ? 'up' : '';
            const stubbed = stub && !(stub === 'up' && side === 'top') && !(stub === 'down' && side === 'bottom') ? stub : '';
            if (stubbed === 'up') stubTop = true;
            if (stubbed === 'down') stubBottom = true;
            if (stubbed === 'left') stubLeft = true;
            records.push({ number: String(number), r, c, off: row.offset, fs, side, stub: stubbed });
        }));
        const padT = style.padT + (outsideTop ? OUT_ROOM : 0), padB = style.padB + (outsideBottom ? OUT_ROOM : 0);
        const sw = 2 * style.padX + (cols - 1) * p + dims.w, sh = padT + padB + (count - 1) * rp + dims.h;
        const ears = style.ears || 0;

        // 2. the room for the names beside the contacts
        let leftW = 0, rightW = 0;
        const sideOf = (row, c) => (row.pins.length === 2 && c === 0 ? 'left' : 'right');
        if (namesMode) {
            rows.forEach(row => row.pins.forEach((number, c) => {
                if (!given(number) || !named(number)) return;
                const room = textWidth(named(number), NAME_FS) + 14;
                if (sideOf(row, c) === 'left') leftW = Math.max(leftW, room); else rightW = Math.max(rightW, room);
            }));
        }
        const ox = MARGIN + leftW + ears + (stubLeft ? STUB : 0), oy = MARGIN + (stubTop ? STUB : 0);
        const width = ox + sw + ears + rightW + MARGIN, height = oy + sh + (stubBottom ? STUB : 0) + MARGIN;

        // 3. the contacts
        const contacts = records.map(rec => {
            const x = ox + style.padX + dims.w / 2 + (rec.c + rec.off) * p, y = oy + padT + dims.h / 2 + rec.r * rp;
            const c = contact(map, rec.number, { shape: kind === 'blade' ? 'rect' : kind === 'round' ? 'circle' : 'rect', kind, x, y, w: dims.w, h: dims.h, r: kind === 'blade' ? 2 : 3 });
            if (rec.fs) inside(c, rec.fs);
            else if (rec.side === 'top') c.num = { mode: 'out', x, y: y - dims.h / 2 - 4, fs: OUT_FS };
            else if (rec.side === 'bottom') c.num = { mode: 'out', x, y: y + dims.h / 2 + 4 + OUT_FS * 0.8, fs: OUT_FS };
            else inside(c, 8);
            if (rec.stub === 'left') c.stub = { x: ox - STUB, y: y - 3.5, w: x - dims.w / 2 - (ox - STUB), h: 7 };
            else if (rec.stub === 'down') c.stub = { x: x - 3.5, y: y + dims.h / 2, w: 7, h: oy + sh + STUB - (y + dims.h / 2) };
            else if (rec.stub === 'up') c.stub = { x: x - 3.5, y: oy - STUB, w: 7, h: y - dims.h / 2 - (oy - STUB) };
            if (namesMode && c.name) {
                const row = rows[rec.r], side = sideOf(row, rec.c);
                if (side === 'left') {
                    c.label = { x: ox - ears - 8, y: y + NAME_FS * 0.36, anchor: 'end', rotate: 0 };
                    c.leader = { x1: x - dims.w / 2, y1: y, x2: ox - ears - 5, y2: y };
                } else {
                    c.label = { x: ox + sw + ears + 8, y: y + NAME_FS * 0.36, anchor: 'start', rotate: 0 };
                    c.leader = { x1: x + dims.w / 2, y1: y, x2: ox + sw + ears + 5, y2: y };
                }
            }
            return c;
        });
        markFirst(contacts);

        // 4. the shell around them
        const rowsBox = rows.map((row, r) => {
            const at = row.pins.map((number, c) => (given(number) ? c : -1)).filter(c => c >= 0);
            if (!at.length) return { l: 0, r: sw, b: 0 };
            return { l: style.padX + (Math.min(...at) + row.offset) * p, r: style.padX + (Math.max(...at) + row.offset) * p + dims.w, b: padT + r * rp + dims.h };
        });
        const key = shape.key && shape.key !== 'none' ? shape.key : '';
        const build = SHELLS[shape.outline] || SHELLS.rect;
        // The rectangles the shell keeps clear of: the contacts and the text boxes of the numbers outside them (width of the digits, height of the font).
        const cells = contacts.map(c => [c.x - c.w / 2, c.y - c.h / 2, c.x + c.w / 2, c.y + c.h / 2]);
        contacts.forEach(c => { if (c.num.mode === 'out') cells.push([c.num.x - textWidth(c.number, c.num.fs) / 2, c.num.y - c.num.fs, c.num.x + textWidth(c.number, c.num.fs) / 2, c.num.y]); });
        const built = build({ x: ox, y: oy, w: sw, h: sh }, { w: sw, h: sh, rowsBox, ears, key, view, cells });
        return { width, height, shellBox: [ox - ears, oy, ox + sw + ears, oy + sh], shell: built.parts, face: built.face, texts: [], contacts, mirrorable: true };
    }

    // ---- circular ---------------------------------------------------------------------------------------------------------

    function layoutCircular(shape, map) {
        const positions = Array.isArray(shape.positions) ? shape.positions : [];
        const unit = positions.map(position => {
            const angle = (Number(position.angle) || 0) * Math.PI / 180, radius = Number(position.radius) || 0;
            return { pin: String(position.pin), x: radius * Math.sin(angle), y: -radius * Math.cos(angle) };
        });
        // The ring is as large as the closest pair of contacts needs (22 units apart at least).
        let closest = Infinity;
        unit.forEach((a, i) => unit.forEach((b, j) => { if (j > i) closest = Math.min(closest, Math.hypot(a.x - b.x, a.y - b.y)); }));
        const reach = Math.min(120, Math.max(54, Number.isFinite(closest) && closest > 0 ? 24 / closest : 54));
        const rim = reach + 20, size = Math.min(22, Math.max(14, Number.isFinite(closest) ? closest * reach - 3 : 22));
        const cx = MARGIN + rim + 10, cy = MARGIN + rim + 10, width = 2 * cx, height = 2 * cy;
        const contacts = unit.map(u => {
            const c = contact(map, u.pin, { shape: 'circle', kind: 'round', x: cx + u.x * reach, y: cy + u.y * reach, w: size, h: size });
            return inside(c, fitNumber(c.number, 'round', c.w, c.h) || 8);
        });
        markFirst(contacts);
        // A steel ring around the black insert that carries the contacts.
        const ring = solid('ec-shell', circle(cx, cy, rim), STEEL);
        const shell = [cast(ring), ring, layer('ec-shell-tongue', circle(cx, cy, rim - 8), 'pbt')];
        if (shape.key !== null && shape.key !== undefined && shape.key !== '' && Number.isFinite(Number(shape.key))) {
            // the key notch: a small tab across the rim, turned to the angle
            const degrees = Number(shape.key), angle = degrees * Math.PI / 180;
            shell.push(solid('ec-shell-key', 'M-6 -7H6V7H-6Z', STEEL, { at: [round(cx + rim * Math.sin(angle)), round(cy - rim * Math.cos(angle)), round(degrees)] }));
        }
        return { width, height, shellBox: [cx - rim, cy - rim, cx + rim, cy + rim], shell, face: 'pbt', texts: [], contacts, mirrorable: true };
    }

    // ---- dip --------------------------------------------------------------------------------------------------------------

    function layoutDip(shape, map) {
        const n = Math.max(4, Number(shape.pins) || 0), half = n / 2, pitch = 24, leg = { w: 26, h: 16 }, bodyW = 64;
        const names = i => (map.get(String(i)) || {}).name || '';
        let leftW = 0, rightW = 0;
        for (let i = 1; i <= half; i++) leftW = Math.max(leftW, textWidth(names(i), NAME_FS) + 14);
        for (let i = half + 1; i <= n; i++) rightW = Math.max(rightW, textWidth(names(i), NAME_FS) + 14);
        const bx = MARGIN + leftW + leg.w - 6, by = MARGIN + 6, bodyH = half * pitch + 8;
        const width = bx + bodyW + leg.w - 6 + rightW + MARGIN, height = by + bodyH + MARGIN + 6;
        const cx = bx + bodyW / 2, r = 9;
        const contacts = [];
        for (let i = 1; i <= n; i++) {
            const left = i <= half, row = left ? i - 1 : n - i;
            const x = left ? bx - leg.w / 2 + 6 : bx + bodyW + leg.w / 2 - 6, y = by + 4 + pitch / 2 + row * pitch;
            const c = inside(contact(map, i, { shape: 'rect', kind: 'square', x, y, w: leg.w, h: leg.h, r: 1.5 }), fitNumber(String(i), 'square', leg.w, leg.h) || 9);
            c.label = left ? { x: bx - leg.w + 6 - 6, y: y + NAME_FS * 0.36, anchor: 'end', rotate: 0 } : { x: bx + bodyW + leg.w - 6 + 6, y: y + NAME_FS * 0.36, anchor: 'start', rotate: 0 };
            contacts.push(c);
        }
        markFirst(contacts);
        const body = 'M' + bx + ' ' + by + 'H' + (cx - r) + 'A' + r + ' ' + r + ' 0 0 0 ' + (cx + r) + ' ' + by + 'H' + (bx + bodyW) + 'V' + (by + bodyH) + 'H' + bx + 'Z';
        // Black epoxy with a faint highlight along the top (two strips, the notch lies between them); the notch itself is cut out of
        // the outline and filled with the dark recess, and so is the dimple at pin 1.
        const main = solid('ec-shell', body, 'epoxy'), strip = (x0, x1) => layer('ec-shell-detail', roundRect(x0, by + 2, x1 - x0, 1.5, 0.75), 'glint', { fx: true });
        const notch = 'M' + (cx - r) + ' ' + by + 'A' + r + ' ' + r + ' 0 0 0 ' + (cx + r) + ' ' + by + 'Z';
        const shell = [cast(main), main, layer('ec-shell-detail', notch, 'recess'),
            strip(bx + 4, cx - r - 3), strip(cx + r + 3, bx + bodyW - 4), layer('ec-shell-detail', circle(bx + 11, by + 15, 3), 'recess')];
        return { width, height, shellBox: [bx - leg.w + 6, by, bx + bodyW + leg.w - 6, by + bodyH], shell, face: '', texts: [], contacts, mirrorable: false };
    }

    // ---- staggered names under leads (to, jack) ---------------------------------------------------------------------------

    // The names hang below the contacts on leader lines of different length: the first pin has the longest leader, so that no
    // name crosses the leader of another pin.
    function staggerNames(contacts, top, step) {
        const count = contacts.length;
        let reach = 0;
        contacts.forEach((c, index) => {
            const level = top + (count - 1 - index) * step + 12;
            c.leader = { x1: c.x, y1: c.y + c.h / 2, x2: c.x, y2: level - 4 };
            c.label = { x: c.x + 6, y: level + NAME_FS * 0.3, anchor: 'start', rotate: 0 };
            reach = Math.max(reach, c.x + 6 + textWidth(c.name, NAME_FS));
        });
        return { reach, bottom: top + (count - 1) * step + 12 + 8 };
    }

    // ---- to ---------------------------------------------------------------------------------------------------------------

    // The surface-mount packages sot23 and sot223, seen from above with the marking side up (the pin order is in pinouts.go): the
    // epoxy body, tin gull-wing leads (filled rectangles that run under the body to the foot) and the feet as contacts. sot23 lists
    // [1 lower left, 2 lower right, 3 top centre]; sot223 lists [1, 2, 3 along the bottom, then the tab (pin 4) at the top, in tin
    // with its contact in the middle]. The names of the bottom row are staggered under the feet like the TO leads, the name of the
    // top pin stands to the right of it. The dot at the lower left corner marks pin 1.
    function layoutSot(shape, map) {
        const numbers = (Array.isArray(shape.pins) ? shape.pins : []).map(String), is223 = shape.package === 'sot223';
        const nb = Math.max(numbers.length - 1, 1), bodyW = is223 ? 100 : 76, bodyH = is223 ? 54 : 40, gap = is223 ? 34 : 50;
        const foot = { w: 24, h: 18 }, leadLen = 14, tabW = 68, tabH = 30;
        const x0 = MARGIN, y0 = MARGIN, cx = x0 + bodyW / 2;
        const by = y0 + (is223 ? tabH : foot.h + leadLen), footY = by + bodyH + leadLen + foot.h / 2;
        const body = solid('ec-shell', roundRect(x0, by, bodyW, bodyH, 3), 'epoxy');
        const shell = [cast(body)], contacts = [];
        const lead = (x, ya, yb) => solid('ec-shell-lead', 'M' + round(x - 3) + ' ' + round(ya) + 'h6V' + round(yb) + 'h-6Z', 'tin', { across: 'x' });
        const bottom = [];
        numbers.slice(0, nb).forEach((number, i) => {
            const x = cx + (i - (nb - 1) / 2) * gap;
            shell.push(lead(x, by + bodyH - 2, footY));
            bottom.push(inside(contact(map, number, { shape: 'rect', kind: 'square', x, y: footY, w: foot.w, h: foot.h, r: 2 }), fitNumber(number, 'square', foot.w, foot.h) || 9));
        });
        const upper = numbers[nb];
        let upperRight = cx + foot.w / 2;
        if (upper !== undefined) {
            let c;
            if (is223) {
                const tab = solid('ec-shell-tab', roundRect(cx - tabW / 2, y0, tabW, tabH + 2, 3), 'tin', { across: 'x' });
                shell.splice(1, 0, cast(tab));
                shell.push(tab);
                c = contact(map, upper, { shape: 'rect', kind: 'square', x: cx, y: y0 + tabH / 2, w: 44, h: foot.h, r: 2 });
                upperRight = cx + tabW / 2;
            } else {
                shell.push(lead(cx, y0 + foot.h / 2, by + 2));
                c = contact(map, upper, { shape: 'rect', kind: 'square', x: cx, y: y0 + foot.h / 2, w: foot.w, h: foot.h, r: 2 });
            }
            inside(c, fitNumber(upper, 'square', c.w, c.h) || 9);
            c.label = { x: upperRight + 8, y: c.y + NAME_FS * 0.36, anchor: 'start', rotate: 0 };
            contacts.push(c);
        }
        shell.push(body, layer('ec-shell-detail', roundRect(x0 + 4, by + 2, bodyW - 8, 1.5, 0.75), 'glint', { fx: true }),
            layer('ec-shell-detail', circle(x0 + 9, by + bodyH - 9, 2.6), 'recess'));
        contacts.unshift(...bottom);
        markFirst(contacts);
        const names = staggerNames(bottom, footY + foot.h / 2 + 6, 16);
        const nameReach = upper === undefined ? 0 : upperRight + 8 + textWidth((map.get(upper) || {}).name || '', NAME_FS);
        const width = Math.max(x0 + bodyW, names.reach, nameReach) + MARGIN, height = names.bottom + MARGIN;
        return { width, height, shellBox: [x0, by, x0 + bodyW, by + bodyH], shell, face: '', texts: [], contacts, mirrorable: false };
    }

    function layoutTo(shape, map) {
        if (shape.package === 'sot23' || shape.package === 'sot223') return layoutSot(shape, map);
        const numbers = (Array.isArray(shape.pins) ? shape.pins : []).map(String), n = Math.max(numbers.length, 2), gap = 28;
        const is220 = shape.package === 'to220';
        const bodyW = Math.max(is220 ? 76 : 64, (n - 1) * gap + (is220 ? 40 : 36));
        const x0 = MARGIN, y0 = MARGIN, cx = x0 + bodyW / 2;
        const shell = [], leadTop = is220 ? y0 + 86 : y0 + bodyW / 2 + 24;
        // The TO-220 has a steel tab (its hole on a thin dark ring) over the epoxy body; the TO-92 is the half-round epoxy body.
        if (is220) {
            const tab = solid('ec-shell-tab', roundRect(x0, y0, bodyW, 40, 3), STEEL), body = solid('ec-shell', roundRect(x0, y0 + 40, bodyW, 46, 2), 'epoxy');
            shell.push(cast(tab), cast(body), tab, layer('ec-shell-detail', circle(cx, y0 + 17, 7.6), 'recess'), { cls: 'ec-shell-hole', d: circle(cx, y0 + 17, 6) }, body);
        } else {
            const r = bodyW / 2, body = solid('ec-shell', 'M' + x0 + ' ' + (y0 + r) + 'A' + r + ' ' + r + ' 0 0 1 ' + (x0 + bodyW) + ' ' + (y0 + r) + 'V' + (y0 + r + 24) + 'H' + x0 + 'Z', 'epoxy');
            shell.push(cast(body), body);
        }
        const size = { w: 26, h: 20 }, leadLen = 14;
        const contacts = numbers.map((number, i) => {
            const x = cx + (i - (numbers.length - 1) / 2) * gap, y = leadTop + leadLen + size.h / 2;
            // A lead is a filled rectangle (a gradient does not paint a line of no width), tin, from the body to the contact.
            shell.push(solid('ec-shell-lead', 'M' + round(x - 3) + ' ' + round(leadTop - 1) + 'h6V' + round(y) + 'h-6Z', 'tin', { across: 'x' }));
            return inside(contact(map, number, { shape: 'rect', kind: 'square', x, y, w: size.w, h: size.h, r: 2 }), fitNumber(number, 'square', size.w, size.h) || 9);
        });
        markFirst(contacts);
        const names = staggerNames(contacts, leadTop + leadLen + size.h + 6, 16);
        const width = Math.max(x0 + bodyW, names.reach) + MARGIN, height = names.bottom + MARGIN;
        return { width, height, shellBox: [x0, y0, x0 + bodyW, leadTop], shell, face: '', texts: [], contacts, mirrorable: false };
    }

    // ---- jack ---------------------------------------------------------------------------------------------------------------

    function layoutJack(shape, map) {
        const numbers = (Array.isArray(shape.segments) ? shape.segments : []).map(String), n = Math.max(numbers.length, 2);
        const half = 14, gap = 7, tip = 36, ring = 24, sleeve = 72, handle = 64;
        const x0 = MARGIN + 4, cy = MARGIN + half + 18;
        const shell = [], contacts = [];
        let x = x0;
        numbers.forEach((number, i) => {
            const last = i === n - 1, w = i === 0 ? tip : last ? sleeve : ring;
            const mid = x + w / 2;
            const d = i === 0
                ? 'M' + (x + w) + ' ' + (cy - half) + 'H' + (x + half) + 'A' + half + ' ' + half + ' 0 0 0 ' + (x + half) + ' ' + (cy + half) + 'H' + (x + w) + 'Z'
                : 'M' + x + ' ' + (cy - half) + 'H' + (x + w) + 'V' + (cy + half) + 'H' + x + 'Z';
            const c = contact(map, number, { shape: 'path', kind: 'square', x: mid + (i === 0 ? half / 2 : 0), y: cy, w: w - (i === 0 ? half : 0), h: half * 2, d });
            const fs = fitNumber(number, 'square', c.w, c.h);
            if (fs) inside(c, fs); else c.num = { mode: 'out', x: c.x, y: cy - half - 4, fs: OUT_FS };
            contacts.push(c);
            x += w;
            if (!last) { shell.push(layer('ec-shell-insulator', 'M' + x + ' ' + (cy - half) + 'h' + gap + 'v' + half * 2 + 'h' + (-gap) + 'Z', 'pbt')); x += gap; }
        });
        // The segments are the contacts (turned metal, a strong gloss over their colour); the handle is rubber with four grip rings.
        const grip = solid('ec-shell', roundRect(x, cy - 20, handle, 40, 5), 'rubber');
        shell.unshift(cast(grip), layer('ec-shell-shadow', roundRect(x0, cy - half + 3, x - x0 + 6, half * 2, half), 'shadow', { fx: true }));
        shell.push(grip);
        for (let i = 0; i < 4; i++) shell.push(layer('ec-shell-detail', roundRect(x + 12 + i * 9, cy - 17, 2.5, 34, 1), 'glint', { fx: true }));
        markFirst(contacts);
        const names = staggerNames(contacts, cy + half + 6, 16);
        const width = Math.max(x + handle, names.reach) + MARGIN, height = names.bottom + MARGIN;
        return { width, height, shellBox: [x0, cy - 20, x + handle, cy + 20], shell, face: '', texts: [], contacts, mirrorable: false };
    }

    // ---- board ------------------------------------------------------------------------------------------------------------

    function layoutBoard(shape, map) {
        const lists = { left: shape.left || [], right: shape.right || [], top: shape.top || [], bottom: shape.bottom || [] };
        const pitch = 24, h = 18;
        const all = [];
        for (const side of ['left', 'right', 'top', 'bottom']) lists[side].forEach(number => { if (given(number)) all.push(String(number)); });
        const inner = Array.isArray(shape.inner) ? shape.inner : [];
        // the pads of the inner headers are as wide as their longest number needs (ICSP.2 is no "2")
        const innerNumbers = [];
        inner.forEach(group => (group.rows || []).forEach(row => (row.pins || []).forEach(number => { if (given(number)) innerNumbers.push(String(number)); })));
        const iw = Math.max(18, ...innerNumbers.map(number => textWidth(number, 10) + 8)), ipitch = iw + 4;
        inner.forEach(group => (group.rows || []).forEach(row => (row.pins || []).forEach(number => { if (given(number)) all.push(String(number)); })));
        const w = Math.max(18, ...all.map(number => textWidth(number, 10) + 8));
        const nameWidth = side => Math.max(0, ...lists[side].filter(given).map(number => textWidth((map.get(String(number)) || {}).name || '', NAME_FS)));
        const margin = {
            left: MARGIN + (lists.left.some(given) ? nameWidth('left') + 12 + w / 2 : 0),
            right: MARGIN + (lists.right.some(given) ? nameWidth('right') + 12 + w / 2 : 0),
            top: MARGIN + (lists.top.some(given) ? nameWidth('top') + 12 + h / 2 : 0),
            bottom: MARGIN + (lists.bottom.some(given) ? nameWidth('bottom') + 12 + h / 2 : 0)
        };
        const connector = ['top', 'bottom', 'left', 'right'].includes(shape.connector) ? shape.connector : 'none';
        const protrude = connector !== 'none' && !lists[connector].some(given) ? 14 : 0;
        margin[connector === 'none' ? 'top' : connector] = Math.max(margin[connector === 'none' ? 'top' : connector], MARGIN + protrude);
        // the inner headers
        const innerSize = inner.map(group => {
            const rows = group.rows || [];
            const cols = Math.max(1, ...rows.map(row => (row.pins || []).length + (Number(row.offset) || 0)));
            return { cols, rows: rows.length, w: (cols - 1) * ipitch + iw, h: (rows.length - 1) * 22 + 18 + 16 };
        });
        const innerW = Math.max(0, ...innerSize.map(size => size.w)), innerH = innerSize.reduce((sum, size) => sum + size.h + 8, 0);
        const bodyW = Math.max(130, Math.max(lists.top.length, lists.bottom.length) * pitch + 40, innerW + 2 * (w + 14) + 20);
        const bodyH = Math.max(150, Math.max(lists.left.length, lists.right.length) * pitch + 40, innerH + 60);
        const bx = margin.left, by = margin.top;
        const width = bx + bodyW + margin.right, height = by + bodyH + margin.bottom;
        // The board is green circuit board; the connector block of sheet steel sits on its edge (drawn over the board) and may stick
        // out of it.
        const board = solid('ec-shell', roundRect(bx, by, bodyW, bodyH, 6), 'pcb');
        const shell = [cast(board), board];
        const ccx = bx + bodyW / 2, ccy = by + bodyH / 2;
        const inward = 22, out = protrude, obstacles = [];
        if (connector !== 'none') {
            const box = connector === 'top' ? [ccx - 26, by - out, 52, out + inward] : connector === 'bottom' ? [ccx - 26, by + bodyH - inward, 52, out + inward]
                : connector === 'left' ? [bx - out, ccy - 26, out + inward, 52] : [bx + bodyW - inward, ccy - 26, out + inward, 52];
            shell.push(solid('ec-shell-conn', roundRect(box[0], box[1], box[2], box[3], 3), STEEL));
            obstacles.push([box[0], box[1], box[0] + box[2], box[1] + box[3]]);
        }
        const contacts = [];
        const add = (number, x, y, label, size) => {
            const c = contact(map, number, { shape: 'rect', kind: 'square', x, y, w: size.w, h: size.h, r: 3 });
            inside(c, fitNumber(String(number), 'square', size.w, size.h) || 9);
            c.label = label;
            contacts.push(c);
        };
        const gap = (side, i) => (side === 'left' || side === 'right' ? by + 20 + i * pitch + pitch / 2 : bx + 20 + i * pitch + pitch / 2);
        lists.left.forEach((number, i) => { if (given(number)) add(number, bx, gap('left', i), { x: bx - w / 2 - 6, y: gap('left', i) + NAME_FS * 0.36, anchor: 'end', rotate: 0 }, { w, h }); });
        lists.right.forEach((number, i) => { if (given(number)) add(number, bx + bodyW, gap('right', i), { x: bx + bodyW + w / 2 + 6, y: gap('right', i) + NAME_FS * 0.36, anchor: 'start', rotate: 0 }, { w, h }); });
        lists.top.forEach((number, i) => { if (given(number)) add(number, gap('top', i), by, { x: gap('top', i) + NAME_FS * 0.36, y: by - h / 2 - 6, anchor: 'start', rotate: -90 }, { w, h }); });
        lists.bottom.forEach((number, i) => { if (given(number)) add(number, gap('bottom', i), by + bodyH, { x: gap('bottom', i) - NAME_FS * 0.36, y: by + bodyH + h / 2 + 6, anchor: 'start', rotate: 90 }, { w, h }); });
        const texts = [];
        let y = ccy - innerH / 2 + 4;
        inner.forEach((group, index) => {
            const size = innerSize[index], left = ccx - size.w / 2;
            texts.push({ x: ccx, y: y + 10, text: String(group.label || ''), cls: 'ec-pin-inner-label', anchor: 'middle', on: 'pcb' });
            (group.rows || []).forEach((row, r) => (row.pins || []).forEach((number, c) => {
                if (!given(number)) return;
                add(number, left + iw / 2 + (c + (Number(row.offset) || 0)) * ipitch, y + 16 + 9 + r * 22, null, { w: iw, h: 18 });
            }));
            y += size.h + 8;
        });
        markFirst(contacts);
        // The mark of a pad lies on the board when its triangle (anchored 4 units off the pad's upper left corner) is inside it.
        for (const c of contacts) {
            const mx = c.x - c.w / 2 - 1, my = c.y - c.h / 2 - 1;
            c.on = mx > bx && mx < bx + bodyW && my > by && my < by + bodyH ? 'pcb' : '';
            c.under = c.x - c.w / 2 >= bx && c.x + c.w / 2 <= bx + bodyW && c.y - c.h / 2 >= by && c.y + c.h / 2 <= by + bodyH ? 'pcb' : '';
            obstacles.push([c.x - c.w / 2, c.y - c.h / 2, c.x + c.w / 2, c.y + c.h / 2]);
        }
        // Mounting holes: all four corners or none (a single hole looks like a defect), only when no pad, inner header or connector is within 10 units of any.
        const corners = [[bx + 10, by + 10], [bx + bodyW - 10, by + 10], [bx + 10, by + bodyH - 10], [bx + bodyW - 10, by + bodyH - 10]];
        const free = corners.every(([hx, hy]) => !obstacles.some(([x0, y0, x1, y1]) => Math.hypot(Math.max(x0 - hx, 0, hx - x1), Math.max(y0 - hy, 0, hy - y1)) < 14));
        if (free) for (const [hx, hy] of corners) shell.push({ cls: 'ec-shell-hole', d: circle(hx, hy, 4) });
        return { width, height, shellBox: [bx, by, bx + bodyW, by + bodyH], shell, face: 'pcb', texts, contacts, mirrorable: false };
    }

    // ---- layout and mirror image ------------------------------------------------------------------------------------------

    const LAYOUTS = { rows: layoutRows, circular: layoutCircular, dip: layoutDip, to: layoutTo, jack: layoutJack, board: layoutBoard };

    function mirror(result) {
        const width = result.width, flip = x => width - x;
        for (const c of result.contacts) {
            c.x = flip(c.x);
            c.num = Object.assign({}, c.num, { x: flip(c.num.x) });
            if (c.label) c.label = Object.assign({}, c.label, { x: flip(c.label.x), anchor: c.label.anchor === 'start' ? 'end' : c.label.anchor === 'end' ? 'start' : 'middle' });
            if (c.leader) c.leader = { x1: flip(c.leader.x1), y1: c.leader.y1, x2: flip(c.leader.x2), y2: c.leader.y2 };
            if (c.stub) c.stub = Object.assign({}, c.stub, { x: flip(c.stub.x + c.stub.w) });
        }
        result.shellBox = [flip(result.shellBox[2]), result.shellBox[1], flip(result.shellBox[0]), result.shellBox[3]];
        result.mirrored = true;
    }

    function layout(shape, pins, options) {
        const type = shape && shape.type;
        const build = LAYOUTS[type];
        if (!build) return { type: type || '', view: '', mirrored: false, width: 0, height: 0, shellBox: [0, 0, 0, 0], shell: [], texts: [], contacts: [] };
        const view = options && (options.view === 'plug' || options.view === 'socket') ? options.view : shape.drawn;
        const result = build(shape, pinMap(pins), view);
        result.type = type;
        result.view = result.mirrorable ? view : shape.drawn;
        result.mirrored = false;
        result.width = Math.ceil(result.width); // whole units first: the mirror image is about the middle of the viewBox
        result.height = Math.ceil(result.height);
        if (result.mirrorable && view !== shape.drawn && (view === 'plug' || view === 'socket')) mirror(result);
        delete result.mirrorable;
        result.face = result.face || '';
        for (const c of result.contacts) { c.x = round(c.x); c.y = round(c.y); if (c.on === undefined) c.on = result.face; if (c.under === undefined) c.under = result.face; }
        return result;
    }

    // ---- markup -----------------------------------------------------------------------------------------------------------

    // The body of a contact, `grow` units larger on every side (the selection ring).
    function bodyMarkup(c, cls, grow, extra) {
        const g = grow || 0, tail = extra || '';
        if (c.shape === 'circle') return '<circle class="' + cls + '" cx="' + c.x + '" cy="' + c.y + '" r="' + round(c.w / 2 + g) + '"' + tail + '/>';
        if (c.shape === 'path') return '<path class="' + cls + '" d="' + c.d + '"' + tail + '/>';
        return '<rect class="' + cls + '" x="' + round(c.x - c.w / 2 - g) + '" y="' + round(c.y - c.h / 2 - g) + '" width="' + round(c.w + 2 * g) + '" height="' + round(c.h + 2 * g) + '" rx="' + round(c.r + g) + '"' + tail + '/>';
    }
    // m is the material scope of the drawing. The mark and a number outside its contact lie on the surface c.on (a material: print in
    // its ink, as an attribute; the bare stage: the text token, by the class ec-on-stage); data-on names that material.
    function pinMarkup(c, types, wires, m) {
        const print = window.ECalcMaterials.ink(c.on), onFace = print ? ' fill="' + print + '" data-on="' + escape(c.on) + '"' : '', onClass = print ? '' : ' ec-on-stage';
        const style = types[c.type] || types.nc || {};
        const hex = safeColor(style.hex) || FALLBACK_HEX, ink = safeColor(style.ink) || FALLBACK_INK;
        const out = c.num.mode === 'out';
        let markup = '<g class="ec-pin" data-pin="' + escape(c.number) + '" data-type="' + escape(c.type) + '" data-num="' + c.num.mode + '">';
        if (c.stub) {
            const wire = wires[c.wire] || {}, base = safeColor(wire.hex), stripe = safeColor(wire.stripe);
            if (base) {
                const st = c.stub, horizontal = st.w > st.h;
                markup += '<g class="ec-wire-stub" data-wire="' + escape(c.wire) + '"><rect class="ec-wire-base" x="' + round(st.x) + '" y="' + round(st.y) + '" width="' + round(st.w) + '" height="' + round(st.h) + '" rx="1.5" fill="' + base + '"/>'
                    + '<rect class="ec-wire-sheen" x="' + round(st.x) + '" y="' + round(st.y) + '" width="' + round(st.w) + '" height="' + round(st.h) + '" rx="1.5"' + m.paint('sheen', horizontal ? '' : 'x') + '/>'
                    + (stripe ? '<rect class="ec-wire-stripe" x="' + round(horizontal ? st.x : st.x + st.w / 2 - 1.2) + '" y="' + round(horizontal ? st.y + st.h / 2 - 1.2 : st.y) + '" width="' + (horizontal ? round(st.w) : 2.4) + '" height="' + (horizontal ? 2.4 : round(st.h)) + '" fill="' + stripe + '"/>' : '') + '</g>';
            }
        }
        if (c.leader) markup += '<path class="ec-leader" d="M' + round(c.leader.x1) + ' ' + round(c.leader.y1) + 'L' + round(c.leader.x2) + ' ' + round(c.leader.y2) + '"/>';
        markup += '<rect class="ec-pin-hit" x="' + round(c.x - c.w / 2 - 3) + '" y="' + round(c.y - c.h / 2 - 3) + '" width="' + (c.w + 6) + '" height="' + (c.h + 6) + '"/>';
        // The rim lies on the surface under the whole contact: the ink of that material, else (bare stage, or straddling the edge) the text token.
        const rim = window.ECalcMaterials.ink(c.under);
        markup += bodyMarkup(c, 'ec-pin-body' + (rim ? '' : ' ec-rim-stage'), 0, ' fill="' + hex + '"' + (rim ? ' stroke="' + rim + '" data-on="' + escape(c.under) + '"' : ''));
        // The soft sheen over the colour of the contact; the long segments of a jack read as turned metal and get the strong gloss.
        markup += bodyMarkup(c, 'ec-pin-sheen', 0, m.paint(c.shape === 'path' ? 'gloss' : 'sheen'));
        markup += bodyMarkup(c, 'ec-pin-ring', c.shape === 'path' ? 0 : 3);
        // The mark sits at the upper left corner, or at the lower left one when the number stands above the contact.
        if (c.mark) markup += c.num.mode === 'out' && c.num.y < c.y
            ? '<path class="ec-pin-mark' + onClass + '" d="M' + round(c.x - c.w / 2 - 4) + ' ' + round(c.y + c.h / 2 + 4) + 'h9l-9 -9Z"' + onFace + '/>'
            : '<path class="ec-pin-mark' + onClass + '" d="M' + round(c.x - c.w / 2 - 4) + ' ' + round(c.y - c.h / 2 - 4) + 'h9l-9 9Z"' + onFace + '/>';
        markup += '<text class="ec-pin-num' + (out ? ' ec-pin-num-out' + onClass : '') + '" x="' + round(c.num.x) + '" y="' + round(c.num.y) + '" text-anchor="middle" font-size="' + c.num.fs + '"' + (out ? onFace : ' fill="' + ink + '"') + '>' + escape(c.number) + '</text>';
        if (c.label && c.name) {
            const turn = c.label.rotate ? ' transform="rotate(' + c.label.rotate + ' ' + round(c.label.x) + ' ' + round(c.label.y) + ')"' : '';
            markup += '<text class="ec-pin-name" x="' + round(c.label.x) + '" y="' + round(c.label.y) + '" text-anchor="' + c.label.anchor + '" font-size="' + NAME_FS + '"' + turn + '>' + escape(c.name) + '</text>';
        }
        return markup + '</g>';
    }

    function svg(spec) {
        const view = spec && spec.layout;
        if (!view || !view.width || !view.height) return '';
        const types = spec.types || {}, wires = spec.wires || {};
        const flip = view.mirrored ? ' transform="translate(' + view.width + ' 0) scale(-1 1)"' : '';
        // The materials of this drawing: every gradient it asks for is defined once in <defs>, under ids of this uid.
        const m = window.ECalcMaterials.scope(spec.uid);
        // A part is its path with the fill of its material; a silhouette is followed by its edge line (the same outline and turn).
        const shell = view.shell.map(part => {
            const turned = part.at ? ' transform="translate(' + part.at[0] + ' ' + part.at[1] + ') rotate(' + part.at[2] + ')"' : '';
            return '<path class="' + part.cls + (part.fx ? ' ec-shell-fx' : '') + '" d="' + part.d + '"' + (part.mat ? m.paint(part.mat, part.across) : '') + turned + '/>'
                + (part.edge ? '<path class="ec-shell-edge" d="' + part.d + '"' + turned + '/>' : '');
        }).join('');
        const texts = view.texts.map(text => {
            const print = window.ECalcMaterials.ink(text.on);
            return '<text class="' + text.cls + (print ? '' : ' ec-on-stage') + '" x="' + round(text.x) + '" y="' + round(text.y) + '" text-anchor="' + text.anchor + '" font-size="10"' + (print ? ' fill="' + print + '"' : '') + '>' + escape(text.text) + '</text>';
        }).join('');
        const pins = view.contacts.map(c => pinMarkup(c, types, wires, m)).join('');
        return '<svg class="ec-pin-art" data-shape="' + escape(view.type) + '" data-view="' + escape(view.view) + '" viewBox="0 0 ' + view.width + ' ' + view.height + '" width="' + view.width + '" height="' + view.height + '" role="img" aria-label="' + escape(spec.label || '') + '" focusable="false">'
            + m.defs()
            + '<g class="ec-pin-shell"' + flip + '>' + shell + '</g>'
            + texts + pins
            + '</svg>';
    }

    window.ECalcPinoutDraw = { layout, svg, OUTLINES, fitNumber, safeColor, textWidth };
})();
