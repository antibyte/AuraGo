(function () {
    'use strict';
    // Tables and pictures: what the Format panel holds, as engine commands. Nothing here touches the editor.
    // The engine also knows dashed and dotted, but does not paint them on a left edge that had no line before.
    const STYLES = ['single', 'double', 'triple', 'thick'];
    // Line widths in eighths of a point, the engine's unit: 1/2, 3/4, 1, 1 1/2, 2 1/4 and 3 pt.
    const WIDTHS = [4, 6, 8, 12, 18, 24];
    const EDGES = ['all', 'outside', 'inside', 'top', 'bottom', 'left', 'right', 'none'];
    const EDGE_LABELS = {all: 'borders', outside: 'border_outside', inside: 'border_inside', top: 'margin_top', bottom: 'margin_bottom', left: 'margin_left', right: 'margin_right', none: 'no_borders'};
    const PEN = {style: 'single', size: 4, color: '64748B'};

    // pen is the line every border button draws with. A value nobody can draw with falls back to the default.
    function pen(value = {}) {
        const color = String(value.color || '').replace(/^#/, '').toUpperCase(), size = Number(value.size);
        return {style: STYLES.includes(value.style) ? value.style : PEN.style, size: WIDTHS.includes(size) ? size : PEN.size, color: /^[0-9A-F]{6}$/.test(color) ? color : PEN.color};
    }
    // spec is the pen in the shape the engine takes.
    const spec = value => ({style: value.style, size: value.size, color: {kind: 'hex', value: value.color}});
    const widthLabel = (size, format) => format(size / 8) + ' pt';
    const edgeLabel = edge => EDGE_LABELS[edge];

    // borderCommand is the command of one edge button. It acts on the selected cells, or on the cell at the caret.
    function borderCommand(edge, value) {
        if (edge === 'none') return {type: 'setTableBorders', scope: 'none', target: 'all'};
        return EDGES.includes(edge) ? {type: 'setTableBorders', scope: edge, spec: spec(value)} : null;
    }

    // sizeCommand sets a picture's width or height in EMU. locked changes the other side with it.
    function sizeCommand(image, side, emu, locked) {
        const size = Math.round(emu);
        if (!(size > 0)) return null;
        if (!locked) return {type: 'setImageProperties', [side === 'width' ? 'widthEmu' : 'heightEmu']: size};
        if (!(image.widthEmu > 0) || !(image.heightEmu > 0)) return null;
        return side === 'width' ? {type: 'setImageProperties', widthEmu: size, heightEmu: Math.round(size * image.heightEmu / image.widthEmu)}
            : {type: 'setImageProperties', widthEmu: Math.round(size * image.widthEmu / image.heightEmu), heightEmu: size};
    }
    // fitHeight is the height a picture of the given pixel size has at a width.
    const fitHeight = (widthEmu, pixelWidth, pixelHeight) => Math.round(widthEmu * pixelHeight / pixelWidth);

    // crop takes the four sides in percent of the picture. Something of the picture has to stay.
    function crop(values) {
        const sides = ['left', 'top', 'right', 'bottom'].map(side => Number(values[side]));
        if (sides.some(side => !Number.isFinite(side) || side < 0 || side >= 100) || sides[0] + sides[2] >= 100 || sides[1] + sides[3] >= 100) return {error: 'crop_invalid'};
        return {command: {type: 'setImageProperties', crop: {left: sides[0], top: sides[1], right: sides[2], bottom: sides[3]}}};
    }

    window.WriterObjects = {styles: STYLES, widths: WIDTHS, edges: EDGES, pen, spec, widthLabel, edgeLabel, borderCommand, sizeCommand, fitHeight, crop};
})();
