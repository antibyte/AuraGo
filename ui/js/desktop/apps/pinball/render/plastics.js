// AuraPinball printed plastics (plan R3b, AM1/AM3/AM7): alpha-tested quads cut from the shared sprite sheet
// (tables/kit/plastics.js) for slings, return-lane guides, back-corner panels and the apron badge, a darker under-quad for
// the acrylic edge, chrome standoffs with screw heads, and printed discs for the pop bumper caps. Visual only.
// buildPlastics(ctx, image) returns null without a sheet image (the slings then keep their own plates); otherwise
// { mat, edgeMat, slingMats: {slingL, slingR}, capDisc(r) } which builders.js stores as ctx.plastics.
import * as THREE from 'three';
import { canvasTexture, merge, quad } from './materials.js';
import { plasticPieces, cellUV } from '../tables/kit/plastics.js';
import { addScrew } from './hardware.js';

const EDGE_GAP = 0.0022, STAND_R = 0.003, CAP_H = 0.0392, CAP_EDGE_GAP = 0.0015, CAP_K = 0.78;

// ImageBitmaps ignore texture.flipY, so the sheet goes through a canvas (flipY texture, row 0 at the top) like the other art.
function sheetTexture(image, aniso) {
    const c = document.createElement('canvas'); c.width = image.width; c.height = image.height;
    c.getContext('2d').drawImage(image, 0, 0);
    return canvasTexture(c, { aniso });
}

const pieceQuad = (piece, h) => {
    const [x0, y0, x1, y1] = piece.rect;
    return quad([x0, h, -y0], [x1, h, -y0], [x1, h, -y1], [x0, h, -y1], cellUV(piece.cell, piece.flipU));
};

export function buildPlastics(ctx, image) {
    if (!image) return null;
    let pieces;
    try { pieces = plasticPieces(ctx.layout); } catch (e) { console.warn('plastics skipped:', e.message); return null; }
    const { D, low } = ctx, tex = D.t(sheetTexture(image, ctx.tier && ctx.tier.aniso || 1));
    const base = { map: tex, alphaTest: 0.5, side: THREE.DoubleSide, roughness: 0.25, emissive: new THREE.Color('#ffffff'), emissiveMap: tex, emissiveIntensity: 0.2, envMap: ctx.env, envMapIntensity: 0.5 };
    const mat = D.m(low ? new THREE.MeshStandardMaterial(base) : new THREE.MeshPhysicalMaterial({ ...base, clearcoat: 0.6, clearcoatRoughness: 0.2 }));
    const edgeMat = D.m(new THREE.MeshStandardMaterial({ map: tex, color: '#3a3a3a', alphaTest: 0.5, side: THREE.DoubleSide, roughness: 0.5, metalness: 0.1, envMap: ctx.env, envMapIntensity: 0.3 }));
    const group = new THREE.Group(), flat = [], under = [], slingMats = {};
    group.name = 'plastics';
    const add = (geo, m, name, shadow = true) => {
        D.g(geo);
        const o = new THREE.Mesh(geo, m); o.name = name; o.castShadow = shadow && !low; o.receiveShadow = false; group.add(o);
        return o;
    };
    for (const piece of pieces) {
        under.push(pieceQuad(piece, piece.h - EDGE_GAP));
        if (piece.cell === 'sling') {
            // Each sling owns a material so it can flash when kicked (builders.buildSling drives emissiveIntensity).
            slingMats[piece.id] = D.m(mat.clone());
            add(pieceQuad(piece, piece.h), slingMats[piece.id], piece.id + '_plastic');
        } else flat.push(pieceQuad(piece, piece.h));
        for (const [x, y] of piece.posts) {
            ctx.statics.chrome.push(new THREE.CylinderGeometry(STAND_R, STAND_R, piece.h, low ? 6 : 10).translate(x, piece.h / 2, -y));
            addScrew(ctx, x, y, piece.h, x * 53 + y * 29);
        }
    }
    const plates = merge(flat), edges = merge(under);
    if (plates) add(plates, mat, 'plastics_plates');
    if (edges) add(edges, edgeMat, 'plastics_edges', false);
    ctx.root.add(group);

    // Printed disc on top of a pop bumper cap: returns { group, mat } the bumper animates (group.position.y, mat.emissiveIntensity).
    function capDisc(r) {
        const [u0, v0, u1, v1] = cellUV('cap'), grp = new THREE.Group(), rad = r * CAP_K, seg = low ? 16 : 32;
        const disc = () => {
            const g = new THREE.CircleGeometry(rad, seg).rotateX(-Math.PI / 2), uv = g.attributes.uv;
            for (let i = 0; i < uv.count; i++) uv.setXY(i, u0 + uv.getX(i) * (u1 - u0), v0 + uv.getY(i) * (v1 - v0));
            return D.g(g);
        };
        const m = D.m(mat.clone()), top = new THREE.Mesh(disc(), m), rim = new THREE.Mesh(disc(), edgeMat);
        top.name = 'cap_print'; rim.position.y = -CAP_EDGE_GAP; rim.name = 'cap_print_edge';
        grp.add(top, rim); grp.position.y = CAP_H;
        return { group: grp, mat: m, h: CAP_H };
    }
    return { group, mat, edgeMat, slingMats, capDisc };
}
