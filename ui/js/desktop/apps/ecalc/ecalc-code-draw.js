/* ECalc code drawings: the parts that the code pages show. A DOM-free classic script that publishes window.ECalcCodeDraw and
   builds inline SVG strings, like the chart and the schematics do. The colour of a band is the `hex` of the code item's data
   and nothing else (a black band stays black in every desktop skin); the body, the leads, the outlines and the captions take
   currentColor and the classes ec-draw-*, ec-band-* and ec-chip-* of css/desktop-app-ecalc.css (tokens only).

   ECalcCodeDraw.bandsSVG(spec) -> SVG markup of an axial part with colour bands.
     spec: {kind: 'resistor' | 'inductor', label, interactive, open, bands: [{color, name, role, roleLabel, caption, aria, wide}]}
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
     spec: {shape: 'chip' | 'disc', lines: [text], label, valid}   (disc: up to two lines, chip: the first line)
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
        resistor: { x: 88, y: 30, width: 224, height: 60, rx: 26 },
        inductor: { x: 88, y: 22, width: 224, height: 76, rx: 14 }
    };

    function bandMarkup(band, index, box, left, interactive, open) {
        const wide = !!band.wide, width = wide ? WIDE_WIDTH : BAND_WIDTH;
        const colour = safeColor(band.color), empty = colour === '' || colour === 'transparent';
        const y = box.y + 1, height = box.height - 2;
        const attributes = ' data-index="' + index + '" data-role="' + escape(band.role || '') + '" data-color="' + escape(band.name || '') + '"'
            + (interactive ? ' tabindex="0" role="button" aria-haspopup="listbox" aria-expanded="' + (open === index ? 'true' : 'false') + '" aria-label="' + escape(band.aria || band.name || '') + '"' : '');
        return '<g class="ec-band"' + attributes + '>'
            + '<rect class="ec-band-hit" x="' + (left - 5) + '" y="' + (box.y - 8) + '" width="' + (width + 10) + '" height="' + (box.height + 16) + '"/>'
            + '<rect class="ec-band-fill' + (empty ? ' ec-band-empty' : '') + '" x="' + left + '" y="' + y + '" width="' + width + '" height="' + height + '"' + (empty ? ' fill="none"' : ' fill="' + colour + '"') + '/>'
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
        return '<svg class="ec-code-art" data-art="' + kind + '" data-bands="' + bands.length + '" viewBox="0 0 ' + WIDTH + ' ' + HEIGHT + '" role="group" aria-label="' + escape(spec.label || '') + '" focusable="false">'
            + '<line class="ec-draw-lead" x1="12" y1="' + MIDDLE + '" x2="' + box.x + '" y2="' + MIDDLE + '"/><line class="ec-draw-lead" x1="' + (box.x + box.width) + '" y1="' + MIDDLE + '" x2="388" y2="' + MIDDLE + '"/>'
            + '<rect class="ec-draw-body" x="' + box.x + '" y="' + box.y + '" width="' + box.width + '" height="' + box.height + '" rx="' + box.rx + '"/>'
            + bands.map((band, index) => bandMarkup(band, index, box, positions[index], interactive, open)).join('')
            + '</svg>';
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
        let body, ink = '';
        if (disc) {
            body = '<line class="ec-draw-lead" x1="92" y1="104" x2="92" y2="126"/><line class="ec-draw-lead" x1="148" y1="104" x2="148" y2="126"/>'
                + '<circle class="ec-disc-body" cx="120" cy="58" r="48"/>';
            const size = inkSize(lines, 78, 30);
            ink = lines.map((line, index) => '<text class="ec-disc-text" x="120" y="' + (lines.length === 1 ? 58 + size * 0.36 : 50 + index * (size + 4) + size * 0.3) + '" text-anchor="middle" font-size="' + size + '">' + escape(line) + '</text>').join('');
        } else {
            body = '<rect class="ec-chip-end" x="30" y="24" width="26" height="72" rx="3"/><rect class="ec-chip-end" x="184" y="24" width="26" height="72" rx="3"/>'
                + '<rect class="ec-chip-body" x="48" y="20" width="144" height="80" rx="4"/>';
            const size = inkSize(lines, 118, 40);
            ink = lines.map(line => '<text class="ec-chip-text" x="120" y="' + (60 + size * 0.36) + '" text-anchor="middle" font-size="' + size + '">' + escape(line) + '</text>').join('');
        }
        return '<svg class="ec-code-art" data-art="' + (disc ? 'disc' : 'chip') + '" data-valid="' + (valid ? 'true' : 'false') + '" viewBox="0 0 240 130" role="img" aria-label="' + escape(spec && spec.label) + '" focusable="false">' + body + ink + '</svg>';
    }

    window.ECalcCodeDraw = { bandsSVG, chipSVG, safeColor, POSITIONS };
})();
