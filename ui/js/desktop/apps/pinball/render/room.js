// AuraPinball room (world space): legs and feet, warm parquet floor with a soft light pool, and the back wall. The wall is the
// shared room-wall bitmap when present (feathered edges over a slate wall) and a procedural dark slate wall with two framed
// colour-field posters, sconce glows and a neon stripe otherwise. Everything is unlit (MeshBasic) so the table stays the hero.
import * as THREE from 'three';
import { rngNext } from '../config.js';
import { CABINET } from '../tables/kit/cabinet-dims.js';
import { merge, canvasTexture } from './materials.js';
import { hexRgb, rgbCss } from './art-kit.js';

const { frontY, backY, bottom } = CABINET;
const canvasOf = (w, h) => { const c = document.createElement('canvas'); c.width = w; c.height = h; return c; };

// Seamless 1 m x 1 m parquet tile: 12 plank rows, boards 0.5 m long (the pattern repeats every 512 px).
function parquetTile() {
    const S = 512, c = canvasOf(S, S), g = c.getContext('2d'), holder = { seed: 0x9A2C };
    const rows = 12, rh = S / rows;
    g.fillStyle = '#1a0f08'; g.fillRect(0, 0, S, S);
    for (let r = 0; r < rows; r++) {
        const off = Math.floor(rngNext(holder) * 4) * 64, cols = [0, 1].map(() => ({ h: 24 + rngNext(holder) * 8, s: 38 + rngNext(holder) * 14, l: 27 + rngNext(holder) * 12 }));
        for (let k = -1; k <= 2; k++) {
            const col = cols[((k % 2) + 2) % 2], x = off + k * 256, y = r * rh;
            const gr = g.createLinearGradient(0, y, 0, y + rh);
            gr.addColorStop(0, `hsl(${col.h},${col.s}%,${col.l + 3}%)`); gr.addColorStop(1, `hsl(${col.h},${col.s}%,${col.l - 3}%)`);
            g.fillStyle = gr; g.fillRect(x + 1, y + 1, 254, rh - 2);
            for (let i = 0; i < 5; i++) {
                const gy = y + 3 + rngNext(holder) * (rh - 6), a = 0.06 + rngNext(holder) * 0.12;
                g.strokeStyle = rngNext(holder) > 0.5 ? `rgba(30,14,4,${a})` : `rgba(255,205,150,${a * 0.6})`; g.lineWidth = 0.7 + rngNext(holder);
                g.beginPath(); g.moveTo(x + 2, gy); g.lineTo(x + 254, gy + (rngNext(holder) - 0.5) * 3); g.stroke();
            }
        }
    }
    return c;
}

function radialCanvas(stops, size = 512) {
    const c = canvasOf(size, size), g = c.getContext('2d'), r = g.createRadialGradient(size / 2, size / 2, 0, size / 2, size / 2, size / 2);
    for (const [t, col] of stops) r.addColorStop(t, col);
    g.fillStyle = r; g.fillRect(0, 0, size, size);
    return c;
}

// Procedural back wall, 12 m x 6 m at 1024 x 512 (85.3 px per metre, canvas y = 0 at 6 m height).
function wallCanvas(acc, sec) {
    const c = canvasOf(1024, 512), g = c.getContext('2d'), holder = { seed: 0x51A7E };
    const bg = g.createLinearGradient(0, 0, 0, 512); bg.addColorStop(0, '#202938'); bg.addColorStop(1, '#161c27');
    g.fillStyle = bg; g.fillRect(0, 0, 1024, 512);
    for (let i = 0; i < 700; i++) { g.fillStyle = rngNext(holder) > 0.5 ? 'rgba(160,185,220,.035)' : 'rgba(0,0,0,.06)'; g.fillRect(rngNext(holder) * 1024, rngNext(holder) * 512, 2 + rngNext(holder) * 10, 1); }
    g.fillStyle = 'rgba(0,0,0,.18)'; for (let x = 0; x < 1024; x += 128) g.fillRect(x, 0, 2, 512);
    g.fillStyle = '#121720'; g.fillRect(0, 372, 1024, 140);                         // wainscot
    g.fillStyle = 'rgba(190,205,225,.22)'; g.fillRect(0, 370, 1024, 3);             // chair rail
    g.save(); g.globalCompositeOperation = 'lighter'; g.shadowColor = rgbCss(acc, 1, 1); g.shadowBlur = 18;
    g.fillStyle = rgbCss(acc, 0.9, 0.5); g.fillRect(0, 428, 1024, 3); g.restore();   // subtle neon stripe
    for (const sx of [-1, 1]) {                                                      // framed colour-field posters
        const cx = 512 + sx * 111, cy = 190, w = 84, h = 112, x = cx - w / 2, y = cy - h / 2;
        g.fillStyle = '#c9d1dd'; g.fillRect(x - 5, y - 5, w + 10, h + 10);
        g.fillStyle = '#0d1118'; g.fillRect(x - 2, y - 2, w + 4, h + 4);
        g.save(); g.beginPath(); g.rect(x + 5, y + 5, w - 10, h - 10); g.clip();
        const a = sx < 0 ? acc : sec, b = sx < 0 ? sec : acc, f = g.createLinearGradient(x, y, x + w, y + h);
        f.addColorStop(0, rgbCss(a, 0.75)); f.addColorStop(1, rgbCss(b, 0.7)); g.fillStyle = f; g.fillRect(x, y, w, h);
        g.shadowColor = rgbCss(b, 1, 1); g.shadowBlur = 16; g.fillStyle = rgbCss(b, 0.95, 0.85); g.fillRect(x + 14, y + 22, w - 28, 26);
        g.fillStyle = rgbCss(a, 1.1, 0.8); g.fillRect(x + 14, y + 62, w - 40, 22);
        g.restore();
    }
    g.save(); g.globalCompositeOperation = 'lighter';                                 // warm sconce glows
    for (const sx of [-1, 1]) {
        const cx = 512 + sx * 205, cy = 150, r = g.createRadialGradient(cx, cy, 0, cx, cy, 120);
        r.addColorStop(0, 'rgba(255,200,130,.62)'); r.addColorStop(0.35, 'rgba(255,170,90,.2)'); r.addColorStop(1, 'rgba(255,150,70,0)');
        g.fillStyle = r; g.fillRect(cx - 120, cy - 120, 240, 240);
    }
    g.restore();
    g.fillStyle = '#e9d6b8'; for (const sx of [-1, 1]) g.fillRect(512 + sx * 205 - 5, 146, 10, 14);
    return c;
}

// Wall bitmap with feathered edges (alpha ramps over 10 % at every side) so it melts into the slate wall behind it.
function featheredWall(img) {
    const w = 1024, h = Math.round(w * img.height / img.width), c = canvasOf(w, h), g = c.getContext('2d');
    g.drawImage(img, 0, 0, w, h);
    g.globalCompositeOperation = 'destination-in';
    const ramp = (x0, y0, x1, y1) => { const m = g.createLinearGradient(x0, y0, x1, y1); m.addColorStop(0, 'rgba(0,0,0,0)'); m.addColorStop(0.1, 'rgba(0,0,0,1)'); m.addColorStop(0.9, 'rgba(0,0,0,1)'); m.addColorStop(1, 'rgba(0,0,0,0)'); g.fillStyle = m; g.fillRect(0, 0, w, h); };
    ramp(0, 0, w, 0); ramp(0, 0, 0, h);
    return c;
}

// World-space room: legs, parquet floor with a pool of light and a back wall. Returns {group, floorY, dispose}.
export function buildRoom({ scene, tableRoot, mats, accent, secondary, images = {} }) {
    const acc = hexRgb(accent, [55, 217, 255]), sec = hexRgb(secondary, [255, 72, 196]);
    tableRoot.updateMatrixWorld(true);
    const world = (x, h, y) => new THREE.Vector3(x, h, -y).applyMatrix4(tableRoot.matrixWorld);
    const front = world(0.25, bottom, frontY + 0.04), floorY = front.y - 0.66;
    const group = new THREE.Group(); group.name = 'room'; scene.add(group);
    const legs = [], feet = [];
    for (const [x, y] of [[0.255, frontY + 0.04], [-0.255, frontY + 0.04], [0.255, backY - 0.04], [-0.255, backY - 0.04]]) {
        const p = world(x, bottom, y), l = p.y - floorY;
        legs.push(new THREE.BoxGeometry(0.055, l, 0.055).translate(p.x, floorY + l / 2, p.z));
        feet.push(new THREE.CylinderGeometry(0.03, 0.034, 0.02, 14).translate(p.x, floorY + 0.01, p.z));
    }
    const owned = [], textures = [];
    const legMesh = new THREE.Mesh(merge(legs), mats.steel), footMesh = new THREE.Mesh(merge(feet), mats.rubberBlack);
    const flat = (w, h, map, o, y, order) => {
        const m = new THREE.Mesh(new THREE.PlaneGeometry(w, h), new THREE.MeshBasicMaterial({ map, ...o }));
        m.rotation.x = -Math.PI / 2; m.position.set(0, y, -0.45); m.renderOrder = order; owned.push(m); textures.push(map); return m;
    };
    const tile = canvasTexture(parquetTile(), { aniso: 8, repeat: true }); tile.repeat.set(9, 9);
    const floor = flat(9, 9, tile, { color: new THREE.Color(0.38, 0.34, 0.31) }, floorY, 0);
    const pool = flat(6, 6, canvasTexture(radialCanvas([[0, 'rgba(255,214,160,.26)'], [0.35, 'rgba(255,190,130,.10)'], [1, 'rgba(255,180,120,0)']])),
        { transparent: true, blending: THREE.AdditiveBlending, depthWrite: false }, floorY + 0.002, 1);
    const fade = flat(9, 9, canvasTexture(radialCanvas([[0, 'rgba(2,3,6,0)'], [0.12, 'rgba(2,3,6,0.12)'], [0.4, 'rgba(2,3,6,.62)'], [0.75, 'rgba(2,3,6,.88)'], [1, 'rgba(2,3,6,.97)']])),
        { transparent: true, depthWrite: false }, floorY + 0.004, 2);
    const wt = canvasTexture(wallCanvas(acc, sec), { aniso: 1 });
    const wall = new THREE.Mesh(new THREE.PlaneGeometry(12, 6), new THREE.MeshBasicMaterial({ map: wt, color: new THREE.Color(1.6, 1.6, 1.6) }));
    wall.position.set(0, floorY + 3, -3.4); owned.push(wall); textures.push(wt);
    group.add(legMesh, footMesh, floor, pool, fade, wall);
    if (images.roomWall) {
        const imgTex = canvasTexture(featheredWall(images.roomWall), { aniso: 2 }), w = 5.0, h = w * images.roomWall.height / images.roomWall.width;
        const pic = new THREE.Mesh(new THREE.PlaneGeometry(w, h), new THREE.MeshBasicMaterial({ map: imgTex, color: new THREE.Color(0.85, 0.85, 0.85), transparent: true, depthWrite: false }));
        pic.position.set(0, floorY + 1.55, -3.38); pic.renderOrder = 3; group.add(pic); owned.push(pic); textures.push(imgTex);
    }
    return {
        group, floorY,
        dispose() {
            legMesh.geometry.dispose(); footMesh.geometry.dispose();
            for (const m of owned) { m.geometry.dispose(); m.material.dispose(); }
            textures.forEach(t => t.dispose());
            scene.remove(group);
        }
    };
}
