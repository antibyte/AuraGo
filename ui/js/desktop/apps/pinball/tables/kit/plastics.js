// Printed plastics sprite sheet shared by all tables: cell layout and the derived pieces. DOM-free.
// Verbatim from the plastics template (scripts/pinball-art-guide.mjs plastics, `--check` keeps this in sync).
// px = [x, y, w, h] in the 1024x1536 sheet (row 0 at the top); world = [x0, y0, x1, y1] in metres.
// For mirror pieces the world rect is the LEFT piece in kit coordinates u = x - cx (absolute x for `corner`); the right
// piece is the mirror image (about x = cx, or about x = 0 for `corner`) with U flipped.
export const SHEET = Object.freeze({
    w: 1024, h: 1536,
    cells: Object.freeze({
        sling: Object.freeze({ px: Object.freeze([48.61, 44, 292.77, 732]), world: Object.freeze([-0.17750000000000005, 0.20383866945841858, -0.09501698681141207, 0.4100657733443529]), mirror: true, height: 0.034 }),
        guide: Object.freeze({ px: Object.freeze([394, 87.08, 272, 645.83]), world: Object.freeze([-0.216, 0.1785, -0.1145, 0.4195]), mirror: true, height: 0.030 }),
        corner: Object.freeze({ px: Object.freeze([714, 64.92, 272, 410.16]), world: Object.freeze([-0.2575, 0.8325, -0.1, 1.07]), mirror: true, height: 0.040, absolute: true }),
        cap: Object.freeze({ px: Object.freeze([714, 544, 272, 272]), world: Object.freeze([-0.029, -0.029, 0.029, 0.029]) }),
        sign1: Object.freeze({ px: Object.freeze([113.33, 844, 473.33, 142]), world: Object.freeze([-0.030, -0.009, 0.030, 0.009]) }),
        sign2: Object.freeze({ px: Object.freeze([113.33, 1024, 473.33, 142]), world: Object.freeze([-0.030, -0.009, 0.030, 0.009]) }),
        badge: Object.freeze({ px: Object.freeze([144.17, 1214, 735.65, 282]), world: Object.freeze([-0.060, -0.023, 0.060, 0.023]) })
    })
});

// The badge lies flat on the apron (playfield metres, see AM5).
const BADGE_RECT = Object.freeze([-0.068, -0.087, 0.052, -0.041]);
const BADGE_H = 0.0128;
// Return-lane plastic centre line (kit u, y), second and third strip points: the inlane guide is mirrored the same way.
const GUIDE_STRIP = Object.freeze([[-0.2045, 0.300], [-0.150, 0.2046]]);
const CORNER_INSET = 0.02;

// UV rect [u0, v0, u1, v1] of a cell for a three.js texture with flipY = true (v0 = bottom edge). flipU swaps u0 and u1.
export function cellUV(id, flipU = false) {
    const cell = SHEET.cells[id];
    if (!cell) throw new Error('Unknown plastics cell: ' + id);
    const [x, y, w, h] = cell.px;
    const u0 = x / SHEET.w, u1 = (x + w) / SHEET.w, v0 = 1 - (y + h) / SHEET.h, v1 = 1 - y / SHEET.h;
    return flipU ? [u1, v0, u0, v1] : [u0, v0, u1, v1];
}

// Pieces of the sheet that sit on the playfield, in the order slingL, slingR, guideL, guideR, cornerL, cornerR, badge.
// posts are the standoff feet under the piece (playfield metres).
export function plasticPieces(layout) {
    const cx = layout.meta.cx, byId = new Map((layout.posts || []).map(p => [p.id, p]));
    const post = id => {
        const p = byId.get(id);
        if (!p) throw new Error('Plastics need layout post ' + id);
        return [p.x, p.y];
    };
    const pieces = [];
    for (const side of ['L', 'R']) {
        const right = side === 'R', cell = SHEET.cells.sling, [u0, y0, u1, y1] = cell.world;
        pieces.push({
            id: 'sling' + side, cell: 'sling', rect: right ? [cx - u1, y0, cx - u0, y1] : [cx + u0, y0, cx + u1, y1],
            h: cell.height, flipU: right, posts: [1, 2, 3].map(i => post('sling' + side + '_p' + i))
        });
    }
    for (const side of ['L', 'R']) {
        const right = side === 'R', cell = SHEET.cells.guide, [u0, y0, u1, y1] = cell.world;
        pieces.push({
            id: 'guide' + side, cell: 'guide', rect: right ? [cx - u1, y0, cx - u0, y1] : [cx + u0, y0, cx + u1, y1],
            h: cell.height, flipU: right, posts: [post('dividerPost' + side), ...GUIDE_STRIP.map(([u, y]) => [right ? cx - u : cx + u, y])]
        });
    }
    for (const side of ['L', 'R']) {
        const right = side === 'R', cell = SHEET.cells.corner, [x0, y0, x1, y1] = cell.world;
        const feet = [[x0 + CORNER_INSET, 0.96], [x0 + CORNER_INSET, y1 - CORNER_INSET]];
        pieces.push({
            id: 'corner' + side, cell: 'corner', rect: right ? [-x1, y0, -x0, y1] : [x0, y0, x1, y1],
            h: cell.height, flipU: right, posts: feet.map(([x, y]) => [right ? -x : x, y])
        });
    }
    pieces.push({ id: 'badge', cell: 'badge', rect: [...BADGE_RECT], h: BADGE_H, flipU: false, posts: [] });
    return pieces;
}
