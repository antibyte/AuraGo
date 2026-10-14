/**
 * View geometry shared by every renderer: projection, panel layout, and the
 * drawing primitives a backend has to put on the page.
 *
 * `svg.ts` and `png.ts` both consume this. Keeping it separate is what makes
 * the two outputs agree by construction rather than by two copies of the same
 * arithmetic: a panel is laid out once, and each backend only decides how to
 * put a polyline, a label and a background on its own canvas.
 *
 * @module
 */
const ORIGIN = { x: 0, y: 0, z: 0 };
// Screen-up is (-dir) x sx: with the viewer on the -dir side, (sx, sy, -dir) is
// right-handed, so the panel is a true view rather than a mirror image.
function basis(dir, sx) {
    return { dir, sx, sy: cross(neg(dir), sx) };
}
export function basisFor(view) {
    switch (view) {
        case "front":
            return basis(v(0, 1, 0), v(1, 0, 0));
        case "back":
            return basis(v(0, -1, 0), v(-1, 0, 0));
        case "top":
            return basis(v(0, 0, -1), v(1, 0, 0));
        case "bottom":
            return basis(v(0, 0, 1), v(1, 0, 0));
        case "right":
            return basis(v(-1, 0, 0), v(0, 1, 0));
        case "left":
            return basis(v(1, 0, 0), v(0, -1, 0));
        case "iso":
            // Camera in the +X+Y+Z octant looking at the origin with +Z up, so
            // +X runs to the lower left and +Y to the lower right.
            return basis(normalize(v(-1, -1, -1)), normalize(v(-1, 1, 0)));
    }
}
export const VIEW_LABEL = {
    front: "Front",
    back: "Back",
    top: "Top",
    bottom: "Bottom",
    left: "Left",
    right: "Right",
    iso: "Iso",
};
// --- vector helpers ---------------------------------------------------------
function v(x, y, z) {
    return { x, y, z };
}
function dot(a, b) {
    return a.x * b.x + a.y * b.y + a.z * b.z;
}
function cross(a, b) {
    return {
        x: a.y * b.z - a.z * b.y,
        y: a.z * b.x - a.x * b.z,
        z: a.x * b.y - a.y * b.x,
    };
}
function neg(a) {
    return { x: -a.x, y: -a.y, z: -a.z };
}
function normalize(a) {
    const len = Math.hypot(a.x, a.y, a.z) || 1;
    return { x: a.x / len, y: a.y / len, z: a.z / len };
}
const PROJECTION_FIELDS = [
    "visibleOutline",
    "visibleSmooth",
    "visibleSharp",
    "hiddenOutline",
    "hiddenSmooth",
    "hiddenSharp",
];
export function collectEdges(kernel, shape, viewBasis, deflection) {
    // HLR puts the viewer on the +Z side of the gp_Ax2 it projects with, so the
    // direction it gets is the view-plane normal toward the camera, -dir. The
    // result comes back in that frame: x along sx, y along (-dir) x sx, which
    // is sy, and z always 0. The in-plane coordinates are the screen
    // coordinates as they stand.
    const proj = kernel.projectEdges(shape, ORIGIN, neg(viewBasis.dir), viewBasis.sx);
    try {
        const toLines = (h) => {
            if (Number(h) === 0)
                return [];
            const { points, edgeGroups } = kernel.wireframe(h, deflection);
            const lines = [];
            for (let g = 0; g < edgeGroups.length; g += 3) {
                const start = edgeGroups[g];
                const count = edgeGroups[g + 1];
                const line = [];
                for (let i = 0; i < count; i += 3) {
                    line.push(points[start + i], points[start + i + 1]);
                }
                if (line.length >= 4)
                    lines.push(line);
            }
            return lines;
        };
        return {
            visible: [
                ...toLines(proj.visibleOutline),
                ...toLines(proj.visibleSmooth),
                ...toLines(proj.visibleSharp),
            ],
            hidden: [
                ...toLines(proj.hiddenOutline),
                ...toLines(proj.hiddenSmooth),
                ...toLines(proj.hiddenSharp),
            ],
        };
    }
    finally {
        for (const field of PROJECTION_FIELDS) {
            const h = proj[field];
            if (Number(h) !== 0)
                kernel.release(h);
        }
    }
}
export function extentOf(views) {
    let minU = Infinity;
    let maxU = -Infinity;
    let minV = Infinity;
    let maxV = -Infinity;
    for (const view of views) {
        for (const line of [...view.visible, ...view.hidden]) {
            for (let i = 0; i < line.length; i += 2) {
                const u = line[i];
                const vv = line[i + 1];
                if (u < minU)
                    minU = u;
                if (u > maxU)
                    maxU = u;
                if (vv < minV)
                    minV = vv;
                if (vv > maxV)
                    maxV = vv;
            }
        }
    }
    if (!Number.isFinite(minU))
        return { minU: 0, maxU: 0, minV: 0, maxV: 0 };
    return { minU, maxU, minV, maxV };
}
/**
 * Build a view→pixel mapping for one panel. All panels share `scale` (so the
 * part is the same size everywhere) but center on their own projected midpoint.
 */
export function panelTransform(ext, scale, panelW, panelH, pad) {
    const midU = (ext.minU + ext.maxU) / 2;
    const midV = (ext.minV + ext.maxV) / 2;
    // Screen y is inverted (v-up → y-down); center within the panel.
    const offsetX = panelW / 2 - midU * scale;
    const offsetY = panelH / 2 + midV * scale;
    return { scale, offsetX, offsetY, inner: Math.min(panelW, panelH) - 2 * pad, panelW, panelH, pad };
}
/** Map a polyline from view coords into panel pixels: [x0, y0, x1, y1, ...]. */
export function toPixels(line, t) {
    const out = [];
    for (let i = 0; i < line.length; i += 2) {
        out.push(line[i] * t.scale + t.offsetX, -line[i + 1] * t.scale + t.offsetY);
    }
    return out;
}
export function resolved(options) {
    return {
        width: options.width ?? 240,
        height: options.height ?? 240,
        padding: options.padding ?? 14,
        showHidden: options.showHidden ?? true,
        showGnomon: options.showGnomon ?? true,
        strokeWidth: options.strokeWidth ?? 1,
        background: options.background ?? "#ffffff",
        visibleColor: options.visibleColor ?? "#111111",
        hiddenColor: options.hiddenColor ?? "#9e9e9e",
    };
}
export function deflectionFor(kernel, shape, options) {
    if (options.deflection !== undefined)
        return options.deflection;
    const bb = kernel.getBoundingBox(shape);
    const diag = Math.hypot(bb.xmax - bb.xmin, bb.ymax - bb.ymin, bb.zmax - bb.zmin);
    return Math.max(diag * 0.002, 1e-4);
}
/** The gnomon arms for a view, or an empty list when every axis points into
 *  the screen. Axes within 0.05 of the view direction are dropped. */
export function gnomonArms(viewBasis, t) {
    // The label sits past the tip, along the arm. Centred on the tip, the line
    // would run through the letter, and the iso view's X and Y tips are close
    // enough that their letters would touch. Arm + label stay within ~22 px
    // of the origin, so the gnomon keeps to the corner, clear of the drawing.
    const len = 15;
    const gap = 7;
    const ox = t.pad + len + gap;
    const oy = t.panelH - t.pad - len - gap;
    const axes = [
        [v(1, 0, 0), "#d33", "X"],
        [v(0, 1, 0), "#3a3", "Y"],
        [v(0, 0, 1), "#36c", "Z"],
    ];
    const arms = [];
    for (const [axis, color, label] of axes) {
        const dx = dot(axis, viewBasis.sx);
        const dy = -dot(axis, viewBasis.sy);
        const m = Math.hypot(dx, dy);
        if (m < 0.05)
            continue; // axis points into the screen
        arms.push({
            x0: ox, y0: oy, x1: ox + dx * len, y1: oy + dy * len, color, label,
            lx: ox + dx * len + (dx / m) * gap, ly: oy + dy * len + (dy / m) * gap,
        });
    }
    return arms;
}
export function gridLayout(extents, o, columns, showDimensions) {
    // One global scale so the part is sized consistently across all panels.
    const inner = Math.min(o.width, o.height) - 2 * o.padding;
    let maxRange = 1e-9;
    for (const ext of extents) {
        maxRange = Math.max(maxRange, ext.maxU - ext.minU, ext.maxV - ext.minV);
    }
    const scale = inner / maxRange;
    if (!Number.isInteger(columns) || columns < 1) {
        throw new RangeError(`columns must be a positive integer, got ${columns}`);
    }
    const rows = Math.ceil(extents.length / columns);
    const footerH = showDimensions ? 22 : 0;
    const panels = extents.map((ext, i) => ({
        px: (i % columns) * o.width,
        py: Math.floor(i / columns) * o.height,
        t: panelTransform(ext, scale, o.width, o.height, o.padding),
    }));
    return {
        columns,
        rows,
        totalW: columns * o.width,
        totalH: rows * o.height + footerH,
        footerH,
        scale,
        panels,
    };
}
/** The single-panel equivalent of gridLayout. */
export function singleLayout(ext, o) {
    const inner = Math.min(o.width, o.height) - 2 * o.padding;
    const range = Math.max(ext.maxU - ext.minU, ext.maxV - ext.minV, 1e-9);
    return panelTransform(ext, inner / range, o.width, o.height, o.padding);
}
/** The "100 × 60 × 40 (X×Y×Z)" footer text. */
export function dimensionsText(kernel, shape, round) {
    const bb = kernel.getBoundingBox(shape);
    return `${round(bb.xmax - bb.xmin)} × ${round(bb.ymax - bb.ymin)} × ${round(bb.zmax - bb.zmin)} (X×Y×Z)`;
}
//# sourceMappingURL=views.js.map