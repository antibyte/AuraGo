/* ECalc code drawings: the parts that the code pages show. A DOM-free classic script that publishes window.ECalcCodeDraw and
   builds inline SVG strings, like the chart and the schematics do. The colour of a band is the `hex` of the code item's data
   and nothing else (a black band stays black in every desktop skin). Bodies, leads, terminations and print are real materials
   (metal, epoxy, ceramic, dipped coating) that this file asks for by name from window.ECalcMaterials (loaded before it): the fill
   comes from the scope of the drawing as a presentation attribute, so the CSS rules of those elements declare no fill. This
   file holds no colour literal. Every silhouette carries a thin edge on top (class ec-draw-edge, the text token, so it stays
   legible in every desktop skin); decorative layers (shadow, gloss, glint) carry ec-draw-fx. The captions and the edges take
   the classes ec-draw-*, ec-band-* and ec-chip-* of css/desktop-app-ecalc.css (tokens only).
   Every spec may carry `uid`: unique per window and item. It prefixes the ids of the gradients and the clip path, because
   several windows can show the same drawing at once. The markup is deterministic for the same spec.

   ECalcCodeDraw.bandsSVG(spec) -> SVG markup of an axial part with colour bands.
     spec: {kind: 'resistor' | 'inductor', uid, label, interactive, open, bands: [{color, name, role, caption, aria, wide}]}
           kind         the body shape and the band positions (resistor: 3 to 6 bands, inductor: 4 or 5)
           label        accessible name of the drawing
           interactive  the bands are buttons (tabindex 0, aria-haspopup="listbox") that open the colour menu of the page
           open         index of the band whose menu is open (aria-expanded), or -1
           bands[]      color   the hex of the colour ('transparent' draws a dashed empty band)
                        name    the colour name in the language of the catalog (part of the accessible name)
                        role    digit, multiplier, tolerance, tcr, identifier, value (data-role, for the page and the tests)
                        caption the short text under the band (the digit, the multiplier, the tolerance ...)
                        aria    the accessible name of the band
                        wide    the band is drawn wider (the tolerance band and the military identifier)
   ECalcCodeDraw.chipSVG(spec) -> SVG markup of an SMD chip or a disc capacitor with its printed code.
     spec: {shape: 'chip' | 'disc', uid, lines: [text], label, valid}   (disc: up to two lines, chip: the first line)
   ECalcCodeDraw.safeColor(text) -> the text when it is a hex colour or 'transparent', else ''. Both builders escape everything. */
(function () {
    'use strict';

    const escape = text => String(text == null ? '' : text).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[char]));
    const HEX = /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$/;
    const safeColor = text => (typeof text === 'string' && (HEX.test(text) || text === 'transparent') ? text : '');

    const WIDTH = 400, HEIGHT = 130, MIDDLE = 60;
    const BAND_WIDTH = 14, WIDE_WIDTH = 20;
    // The straight part of the body is x 112 .. 288; every band lies inside it. Each entry is the left edge of a band.
    const POSITIONS = {
        resistor: { 3: [132, 160, 188], 4: [128, 156, 184, 246], 5: [124, 148, 172, 196, 248], 6: [122, 144, 166, 188, 232, 262] },
        inductor: { 4: [136, 164, 192, 250], 5: [124, 158, 182, 206, 252] }
    };
    const BODY = {
        resistor: { x: 88, y: 30, width: 224, height: 60 },
        inductor: { x: 88, y: 22, width: 224, height: 76 }
    };

    // The silhouette of an axial part. A resistor is a dog-bone (end caps 2 units taller than the box, the waist 4 units
    // lower); an inductor is a fat barrel. `top` and `height` are the bounding box of the path.
    function silhouette(kind, box) {
        const x = box.x, right = box.x + box.width, y = box.y, bottom = box.y + box.height;
        if (kind === 'inductor') {
            const r = 22;
            return { top: y, height: box.height, d: 'M' + (x + r) + ' ' + y + 'H' + (right - r) + 'Q' + right + ' ' + y + ' ' + right + ' ' + (y + r) + 'V' + (bottom - r) + 'Q' + right + ' ' + bottom + ' ' + (right - r) + ' ' + bottom
                + 'H' + (x + r) + 'Q' + x + ' ' + bottom + ' ' + x + ' ' + (bottom - r) + 'V' + (y + r) + 'Q' + x + ' ' + y + ' ' + (x + r) + ' ' + y + 'Z' };
        }
        const capTop = y - 2, capBottom = bottom + 2, waistTop = y + 4, waistBottom = bottom - 4;
        return { top: capTop, height: capBottom - capTop, d: 'M' + (x + 12) + ' ' + capTop + 'H' + (x + 30) + 'C' + (x + 38) + ' ' + capTop + ' ' + (x + 40) + ' ' + waistTop + ' ' + (x + 48) + ' ' + waistTop
            + 'H' + (right - 48) + 'C' + (right - 40) + ' ' + waistTop + ' ' + (right - 38) + ' ' + capTop + ' ' + (right - 30) + ' ' + capTop + 'H' + (right - 12) + 'Q' + right + ' ' + capTop + ' ' + right + ' ' + (y + 10)
            + 'V' + (bottom - 10) + 'Q' + right + ' ' + capBottom + ' ' + (right - 12) + ' ' + capBottom + 'H' + (right - 30) + 'C' + (right - 38) + ' ' + capBottom + ' ' + (right - 40) + ' ' + waistBottom + ' ' + (right - 48) + ' ' + waistBottom
            + 'H' + (x + 48) + 'C' + (x + 40) + ' ' + waistBottom + ' ' + (x + 38) + ' ' + capBottom + ' ' + (x + 30) + ' ' + capBottom + 'H' + (x + 12) + 'Q' + x + ' ' + capBottom + ' ' + x + ' ' + (bottom - 10)
            + 'V' + (y + 10) + 'Q' + x + ' ' + capTop + ' ' + (x + 12) + ' ' + capTop + 'Z' };
    }

    function bandMarkup(band, index, shape, left, interactive, open, box, m, clipId) {
        const wide = !!band.wide, width = wide ? WIDE_WIDTH : BAND_WIDTH;
        const colour = safeColor(band.color), empty = colour === '' || colour === 'transparent';
        const y = shape.top, height = shape.height;
        const clip = ' clip-path="url(#' + clipId + ')"';
        const attributes = ' data-index="' + index + '" data-role="' + escape(band.role || '') + '" data-color="' + escape(band.name || '') + '"'
            + (interactive ? ' tabindex="0" role="button" aria-haspopup="listbox" aria-expanded="' + (open === index ? 'true' : 'false') + '" aria-label="' + escape(band.aria || band.name || '') + '"' : '');
        return '<g class="ec-band"' + attributes + '>'
            + '<rect class="ec-band-hit" x="' + (left - 5) + '" y="' + (box.y - 8) + '" width="' + (width + 10) + '" height="' + (box.height + 16) + '"/>'
            + '<rect class="ec-band-fill' + (empty ? ' ec-band-empty' : '') + '" x="' + left + '" y="' + y + '" width="' + width + '" height="' + height + '"' + clip + (empty ? ' fill="none"' : ' fill="' + colour + '"') + '/>'
            + (empty ? '' : '<rect class="ec-draw-fx" x="' + left + '" y="' + y + '" width="' + width + '" height="' + height + '"' + clip + m.paint('gloss') + '/>')
            + '<rect class="ec-band-ring" x="' + (left - 3) + '" y="' + (y - 3) + '" width="' + (width + 6) + '" height="' + (height + 6) + '" rx="3"/>'
            + (band.caption ? '<text class="ec-draw-cap" x="' + (left + width / 2) + '" y="' + (box.y + box.height + 26) + '" text-anchor="middle">' + escape(band.caption) + '</text>' : '')
            + '</g>';
    }

    function bandsSVG(spec) {
        const kind = spec && spec.kind === 'inductor' ? 'inductor' : 'resistor';
        const bands = spec && Array.isArray(spec.bands) ? spec.bands : [];
        const positions = POSITIONS[kind][bands.length];
        if (!positions) return '';
        const box = BODY[kind], interactive = !!spec.interactive, open = Number.isInteger(spec.open) ? spec.open : -1;
        const m = window.ECalcMaterials.scope(spec.uid), clipId = window.ECalcMaterials.safeId(spec.uid) + '-body';
        const shape = silhouette(kind, box);
        const material = kind === 'inductor' ? 'choke' : (bands.length >= 5 ? 'film' : 'carbon');
        const lead = (x1, x2) => '<rect class="ec-draw-lead" x="' + x1 + '" y="' + (MIDDLE - 3) + '" width="' + (x2 - x1) + '" height="6" rx="3"' + m.paint('tin') + '/>';
        const content = '<ellipse class="ec-draw-fx" cx="200" cy="' + (box.y + box.height + 8) + '" rx="' + (box.width / 2) + '" ry="5"' + m.paint('shadow') + '/>'
            + lead(12, box.x + 10) + lead(box.x + box.width - 10, 388)
            + '<path class="ec-draw-body" d="' + shape.d + '"' + m.paint(material) + '/>'
            + '<rect class="ec-draw-fx" x="' + box.x + '" y="' + shape.top + '" width="' + box.width + '" height="' + shape.height + '" clip-path="url(#' + clipId + ')"' + m.paint('gloss') + '/>'
            + bands.map((band, index) => bandMarkup(band, index, shape, positions[index], interactive, open, box, m, clipId)).join('')
            + '<path class="ec-draw-edge" d="' + shape.d + '"/>';
        return '<svg class="ec-code-art" data-art="' + kind + '" data-bands="' + bands.length + '" viewBox="0 0 ' + WIDTH + ' ' + HEIGHT + '" role="group" aria-label="' + escape(spec.label || '') + '" focusable="false">'
            + m.defs() + '<clipPath id="' + clipId + '"><path d="' + shape.d + '"/></clipPath>' + content + '</svg>';
    }

    // The printed text of a chip or a disc: the size follows the longest line so that it always fits the body.
    function inkSize(lines, room, limit) {
        const longest = Math.max(2, ...lines.map(line => line.length));
        return Math.max(12, Math.min(limit, Math.floor(room / (0.62 * longest))));
    }

    function chipSVG(spec) {
        const disc = !!spec && spec.shape === 'disc';
        const lines = (spec && Array.isArray(spec.lines) ? spec.lines : []).map(line => String(line == null ? '' : line)).filter(line => line !== '').slice(0, disc ? 2 : 1);
        const valid = !spec || spec.valid !== false;
        const materials = window.ECalcMaterials, m = materials.scope(spec && spec.uid);
        let body, ink = '';
        if (disc) {
            const dip = x => '<path class="ec-draw-fx" d="M' + (x - 6) + ' 96Q' + x + ' 114 ' + x + ' 118Q' + x + ' 114 ' + (x + 6) + ' 96Z"' + m.paint('dip') + '/>';
            const lead = x => '<rect class="ec-draw-lead" x="' + (x - 2.5) + '" y="94" width="5" height="34"' + m.paint('tin', 'x') + '/>';
            const size = inkSize(lines, 78, 30), inkColour = materials.ink('ceramic');
            body = lead(92) + lead(148) + dip(92) + dip(148)
                + '<circle class="ec-disc-body" cx="120" cy="58" r="48"' + m.paint('ceramic') + '/>'
                + '<ellipse class="ec-draw-fx" cx="100" cy="36" rx="24" ry="12" transform="rotate(-28 100 36)"' + m.paint('glint') + '/>';
            ink = lines.map((line, index) => '<text class="ec-disc-text" x="120" y="' + (lines.length === 1 ? 58 + size * 0.36 : 50 + index * (size + 4) + size * 0.3) + '" text-anchor="middle" font-size="' + size + '" fill="' + inkColour + '">' + escape(line) + '</text>').join('')
                + '<circle class="ec-draw-edge" cx="120" cy="58" r="48"/>';
        } else {
            const size = inkSize(lines, 104, 40), inkColour = materials.ink('epoxy');
            body = '<ellipse class="ec-draw-fx" cx="120" cy="106" rx="94" ry="5"' + m.paint('shadow') + '/>'
                + '<rect x="30" y="22" width="180" height="76" rx="4"' + m.paint('alumina') + '/>'
                + '<rect class="ec-chip-end" x="30" y="22" width="32" height="76" rx="4"' + m.paint('steel') + '/><rect class="ec-chip-end" x="178" y="22" width="32" height="76" rx="4"' + m.paint('steel') + '/>'
                + '<rect class="ec-chip-body" x="60" y="25" width="120" height="70" rx="2"' + m.paint('epoxy') + '/>'
                + '<rect class="ec-draw-fx" x="60" y="25" width="120" height="3"' + m.paint('glint') + '/>';
            ink = lines.map(line => '<text class="ec-chip-text" x="120" y="' + (60 + size * 0.36) + '" text-anchor="middle" font-size="' + size + '" fill="' + inkColour + '">' + escape(line) + '</text>').join('')
                + '<rect class="ec-draw-edge" x="30" y="22" width="180" height="76" rx="4"/>';
        }
        return '<svg class="ec-code-art" data-art="' + (disc ? 'disc' : 'chip') + '" data-valid="' + (valid ? 'true' : 'false') + '" viewBox="0 0 240 130" role="img" aria-label="' + escape(spec && spec.label) + '" focusable="false">' + m.defs() + body + ink + '</svg>';
    }

    window.ECalcCodeDraw = { bandsSVG, chipSVG, safeColor, POSITIONS };
})();
