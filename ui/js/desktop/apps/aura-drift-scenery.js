/* Aura Drift - world building: terrain chunks, scenery props and trackside furniture. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const CHUNK_PX = 320;

    const SIDELINES = {
        alpine: ['cow', 'podium', 'snack'], desert: ['llama', 'slalom', 'podium'],
        harbour: ['crab', 'snack', 'slalom'], salt: ['llama', 'podium', 'slalom'],
        forest: ['sheep', 'snack', 'podium'], industry: ['crab', 'slalom', 'snack'],
        glacier: ['penguin', 'slalom', 'podium'], metro: ['penguin', 'snack', 'slalom']
    };
    const sitesByTrack = new WeakMap();

    AD.roadsideSites = function (track) {
        if (sitesByTrack.has(track)) return sitesByTrack.get(track);
        const sites = [], occupied = (AD.furnitureProps?.(track) || []).concat(AD.petDecorPlacements?.(track) || []);
        for (const [index, fraction] of [0.055, 0.58, 0.31].entries()) {
            for (let attempt = 0; attempt < 50; attempt++) {
                const s = track.length * fraction + Math.floor(attempt / 2) * 11;
                const p = AD.pointAt(track, s), side = (attempt + index) % 2 ? 1 : -1;
                const off = side * (p.halfWidth + 12), x = p.x + p.nx * off, y = p.y + p.ny * off;
                const near = AD.sampleTrack(track, x, y), dist = Math.hypot(x - near.x, y - near.y);
                if (dist < near.halfWidth + 10 || occupied.some(o => Math.hypot(o.x - x, o.y - y) < o.r + 11)) continue;
                /* The ground module knows which side of the loop the site is on, so the
                   harbour keeps its stages on the quay and not out on the water. */
                const z = typeof AD.groundHeight === 'function' ? AD.groundHeight(track, track.biomeId, x, y) : 0;
                if (z < -1 || z > 2.5) continue;
                sites.push({ slot: index, s, x, y, z, tx: p.tx, ty: p.ty, r: 9, stage: true, furniture: true, side });
                break;
            }
        }
        sitesByTrack.set(track, sites);
        return sites;
    };

    function cast(scene, animal) {
        const parts = [];
        function add(model, x, y, z, height, motion = '', phase = 0) {
            parts.push({ model, x, y, z, height, motion, phase });
        }
        if (scene === 'podium') {
            for (const [x, podium, winner] of [[-3.8, 1.45, 'gnome_mine'], [0, 2.6, animal], [3.8, .95, 'rubber_duck']]) {
                add('hay_bale', x, 0, 0, podium);
                add(winner, x, 0, podium, winner === animal ? 3.5 : 2.2, 'bow', x);
            }
        } else if (scene === 'slalom') {
            for (const x of [-3.2, 0, 3.2]) add('cone', x, 0, 0, 1.35);
            add(animal, 0, 0, 0, 2.9, 'weave');
            add('gnome_mine', 5.7, 1.6, 0, 2.6, 'marshal');
            add('cone', 5.7, 1.6, 2.15, 1.45, 'marshal');
        } else if (scene === 'snack') {
            add('hay_bale', 2.7, 0, 0, 1.55);
            add('bratwurst', 2.7, 0, 1.65, 1.25, 'turn');
            add('gnome_mine', 4.7, .6, 0, 2.75, 'bow');
            add(animal, -1.1, 0, 0, 3.2, 'queue');
            add('rubber_duck', -5.5, .25, 0, 2.1, 'queue', 1.2);
        } else if (scene === 'taxi') {
            add('rubber_duck', 0, 0, 0, 4.6, 'ride');
            add(animal, .65, 0, 2.65, 2.8, 'ride');
            add('gnome_mine', -5.8, 2.8, 0, 2.5, 'bow');
            add('gnome_mine', 5.7, 2.6, 0, 2.5, 'bow', 1.5);
        } else if (scene === 'disco') {
            add('tyre_stack', 0, 1.8, 0, 2.35);
            add('disco_ball', 0, 1.8, 2.35, 3.25, 'turn');
            add(animal, -3.7, -.4, 0, 3.2, 'dance');
            add('rubber_duck', 0, -2.4, 0, 2.2, 'dance', 1.6);
            add('gnome_mine', 3.7, -.4, 0, 2.7, 'dance', 3.2);
        } else if (scene === 'duck-parade' || scene === 'gnome-parade') {
            const ducksLead = scene === 'gnome-parade';
            add(animal, 5.6, 2.9, 0, 2.9, 'bow');
            add(ducksLead ? 'rubber_duck' : 'gnome_mine', 0, 0, 0, ducksLead ? 3.8 : 2.7, 'waddle', .85);
            for (const phase of [0, -.85, -1.7]) {
                add(ducksLead ? 'gnome_mine' : 'rubber_duck', 0, 0, 0, ducksLead ? 2.1 : 1.65, 'waddle', phase);
            }
        } else if (scene === 'duck-tower') {
            add('gnome_mine', -4.8, .3, 0, 2.7, 'bow');
            add(animal, 4.8, .3, 0, 3, 'bow', 1.2);
            for (const [z, height] of [[0, 3], [2.15, 2.2], [3.78, 1.6]]) add('rubber_duck', 0, 0, z, height, 'balance');
            add('cone', 0, 0, 4.95, 1.35, 'balance');
        } else if (scene === 'cone-juggler' || scene === 'sausage-juggler' || scene === 'duck-juggler') {
            const finale = scene === 'duck-juggler';
            add(animal, 5.5, 2.5, 0, 2.9, 'bow');
            add(finale ? 'rubber_duck' : 'gnome_mine', 0, 0, 0, 3, 'bow');
            const prop = scene === 'sausage-juggler' ? 'bratwurst' : 'cone';
            for (let i = 0; i < 3; i++) {
                const gnome = finale && i === 1;
                add(gnome ? 'gnome_mine' : prop, 0, 0, 2.6, gnome ? 2.2 : 1.45, 'toss', i * 2 / 3);
            }
        }
        return parts;
    }

    AD.roadsideScenes = function (track, finishes = 0) {
        const [animal, first, second] = SIDELINES[track.biomeId] || SIDELINES.alpine;
        const encore = finishes >= 2 && finishes % 3 === 2 ? 'taxi' :
            finishes >= 4 && finishes % 3 === 1 ? 'disco' : second;
        const acts = animal === 'crab' || animal === 'llama'
            ? ['cone-juggler', 'sausage-juggler', 'duck-juggler'] : ['duck-parade', 'gnome-parade', 'duck-tower'];
        const stories = [first, encore, acts[finishes % acts.length]];
        return AD.roadsideSites(track).map(site => {
            const story = stories[site.slot];
            return { site, story, parts: cast(story, animal) };
        });
    };

    /* Each audience remembers its own cue until the car leaves earshot. */
    AD.watchRoadside = function (race, shows, stands) {
        const car = race.cars[0], v = car.v;
        if (race.state !== 'running' || v.out) return;
        const cs = Math.cos(v.psi), sn = Math.sin(v.psi);
        for (const group of [shows, stands]) {
            if (!group) continue;
            for (const audience of group) {
                const site = audience.site || audience, dx = site.x - v.x, dy = site.y - v.y;
                const distance2 = dx * dx + dy * dy;
                if (distance2 > 65 * 65) { audience.reaction = null; continue; }
                if (distance2 > 52 * 52) continue;
                const ahead = dx * cs + dy * sn, sideways = Math.abs(dy * cs - dx * sn);
                let kind;
                if (!v.airborne && car.style?.drifting) kind = 'drift';
                else if (!v.airborne && v.vx > 20 && ahead >= 0 && ahead < Math.min(42, v.vx * .8) && sideways < 17) kind = 'pass';
                for (const e of race.events) {
                    if (!(e.car === 0 || (e.type === 'car_hit' && (e.a === 0 || e.b === 0)))) continue;
                    if (!(Math.hypot(e.x - site.x, e.y - site.y) < 52)) continue;
                    if (e.type === 'near_miss') kind = kind || 'pass';
                    if (e.type === 'car_hit' || e.type === 'wall_hit' || e.type === 'smash' || e.type === 'animal_hit') {
                        kind = 'bump'; break;
                    }
                }
                const last = audience.reaction;
                if (!kind || last?.kind === kind) continue;
                if (last && race.time - last.at < 1.8 && (kind === 'pass' || last.kind === 'bump')) continue;
                audience.reaction = { kind, at: race.time };
            }
        }
    };

    AD.roadsidePulse = function (reaction, time, delay = 0) {
        if (!reaction) return 0;
        const u = (time - reaction.at - delay) / (reaction.kind === 'pass' ? .82 : 1.35);
        if (u <= 0 || u >= 1) return 0;
        if (reaction.kind === 'bump') return Math.min(1, u * 9, (1 - u) * 3);
        if (reaction.kind === 'drift') return Math.abs(Math.sin(2 * Math.PI * u));
        return Math.sin(Math.PI * Math.sqrt(u));
    };

    /* Local metres along and across the verge. Both views share the same choreography. */
    AD.roadsidePose = function (part, site, time, pose, cheerAt, reaction) {
        const t = time + part.phase, beat = Math.sin(t * 2.6);
        let x = part.x, y = part.y, z = part.z, yaw = -site.side * 1.75, lean = 0;
        switch (part.motion) {
            case 'weave':
                x += 4.7 * Math.sin(t * .9); y += 1.95 * Math.sin(t * 2.7 + .7);
                yaw = Math.atan2(5.265 * Math.cos(t * 2.7 + .7), 4.23 * Math.cos(t * .9));
                z += .13 * Math.abs(Math.sin(t * 8)); break;
            case 'ride':
                x += 2 * Math.sin(t * .82); z += .2 + .18 * Math.sin(t * 2);
                yaw = Math.PI + site.side * .78; break;
            case 'queue': x += .4 * beat; yaw = 0; lean = .12 * beat; break;
            case 'bow': lean = .20 * beat; break;
            case 'turn': yaw = t * 1.35; break;
            case 'dance': z += .5 * Math.abs(beat); lean = .24 * beat; yaw += .55 * Math.sin(t * 1.3); break;
            case 'waddle': {
                const angle = time * .78 + part.phase;
                x += 4.3 * Math.sin(angle); y += 2.5 * Math.cos(angle);
                yaw = Math.atan2(-2.5 * Math.sin(angle), 4.3 * Math.cos(angle));
                lean = .16 * Math.sin(6 * angle); z += Math.abs(lean);
                break;
            }
            case 'balance':
                lean = .13 * Math.sin(t * 2); x += z * lean;
                break;
            case 'toss': {
                const flight = time * .85 + part.phase, turn = Math.floor(flight), u = flight - turn;
                x += (turn % 2 ? -1 : 1) * 2.1 * (2 * u - 1);
                z += 4 * u * (1 - u) * 3.5; lean = flight * Math.PI * 2;
                break;
            }
        }
        const spectator = part.motion === 'balance' || part.motion === 'marshal' || part.model === 'gnome_mine' || part.model === 'rubber_duck' || SIDELINE_ANIMALS.has(part.model);
        if (cheerAt !== undefined && spectator) {
            const beatTime = (time - cheerAt - (part.x + 6) * .043) / .76;
            if (beatTime > 0 && beatTime < 1) {
                const wave = Math.sin(Math.PI * beatTime);
                z += .95 * wave; lean += site.side * .31 * wave;
            }
        }
        const pulse = spectator && AD.roadsidePulse(reaction, time, (part.x + 6) * .018);
        if (pulse) {
            const planted = part.z > 0 || part.motion === 'balance' || part.motion === 'ride' || part.motion === 'marshal';
            if (reaction.kind === 'drift') {
                z += 1.15 * pulse;
                lean += site.side * .34 * pulse;
                if (part.model === 'llama') yaw += 1.35 * pulse;
            } else {
                const bump = reaction.kind === 'bump';
                if (!planted) y += site.side * pulse * (part.model === 'crab' ? 2.4 : 1.1);
                lean -= site.side * pulse * (part.model === 'penguin' && bump ? 1.18 : .52);
                if (part.model === 'cow' && bump) yaw += 1.45 * pulse;
                if (part.model === 'sheep' || !bump) z += .65 * pulse;
            }
        }
        pose.x = x; pose.y = y; pose.z = z; pose.yaw = yaw; pose.lean = lean;
        return pose;
    };

    const SIDELINE_ANIMALS = new Set(Object.values(SIDELINES).map(([animal]) => animal));
    const ICONS = { rubber_duck: 'ducks', gnome_mine: 'gnome_mine', cow: 'cow', disco_ball: 'disco' };
    AD.drawRoadside = function (c, show, cam, centre, theta, time) {
        const site = show.site, k = cam.scale, pose = {}, angle = Math.atan2(site.ty, site.tx);
        for (const part of show.parts) {
            AD.roadsidePose(part, site, time, pose, show.cheerAt, show.reaction);
            const x = centre[0] + (site.tx * pose.x - site.ty * pose.y) * k;
            const y = centre[1] + (site.ty * pose.x + site.tx * pose.y) * k;
            const size = part.height * k, kind = part.model;
            c.save(); c.translate(x, y); c.rotate(-theta); c.translate(0, -pose.z * k * .65);
            c.fillStyle = '#12232e60'; c.beginPath(); c.ellipse(0, 0, size * .55, size * .21, 0, 0, 2 * Math.PI); c.fill();
            c.translate(0, -size * .35); c.rotate(-pose.lean);
            if (ICONS[kind] && AD.drawWeaponGlyph) AD.drawWeaponGlyph(c, ICONS[kind], 0, 0, size * 1.35, 1);
            else if (kind === 'bratwurst') {
                c.rotate(pose.yaw); c.lineCap = 'round'; c.strokeStyle = '#9c4325'; c.lineWidth = size * .64;
                c.beginPath(); c.moveTo(-size, 0); c.quadraticCurveTo(0, size * .58, size, 0); c.stroke();
                c.strokeStyle = '#efb55d'; c.lineWidth = size * .13; c.stroke();
            }
            else if (kind === 'cone' || kind === 'hay_bale' || kind === 'tyre_stack') {
                c.fillStyle = { cone: '#ff792c', hay_bale: '#cba04a', tyre_stack: '#334051' }[kind];
                c.beginPath();
                if (kind === 'cone') { c.moveTo(0, -size * .65); c.lineTo(size * .45, size * .4); c.lineTo(-size * .45, size * .4); }
                else c.rect(-size * .65, -size * .4, size * 1.3, size * .8);
                c.fill(); c.fillStyle = kind === 'cone' ? '#fff2cb' : '#796237'; c.fillRect(-size * .22, 0, size * .44, size * .13);
            } else {
                c.scale(Math.cos(angle + theta + pose.yaw) < 0 ? -size : size, size);
                const oval = (x0, y0, rx, ry, col) => { c.fillStyle = col; c.beginPath(); c.ellipse(x0, y0, rx, ry, 0, 0, Math.PI * 2); c.fill(); };
                const fur = { sheep: '#f2edda', llama: '#cd9a65', penguin: '#243242', crab: '#ee6450' }[kind];
                oval(0, .04, .54, .34, fur);
                if (kind === 'crab') { oval(-.45, -.25, .22, .15, fur); oval(.45, -.25, .22, .15, fur); }
                else if (kind === 'penguin') { oval(.13, .1, .31, .24, '#fff1d1'); oval(.48, -.15, .23, .08, '#ffaf3a'); }
                else { oval(.32, -.2, .15, kind === 'llama' ? .43 : .24, fur); oval(.46, kind === 'llama' ? -.53 : -.26, .20, .14, '#775a46'); }
                oval(.46, kind === 'llama' ? -.56 : -.23, .045, .045, '#17212a');
            }
            c.restore();
        }
    };

    /* Two-colour road paint in world metres. Both renderers bake the same quads at track setup. */
    AD.driftZonePaint = function (track, draw) {
        const zones = track.driftZones || [];
        function slash(s, side, reach, depth) {
            const p = AD.pointAt(track, s);
            const point = (inset, along) => [
                p.x + p.nx * side * (p.halfWidth - 1.4 - inset) + p.tx * along,
                p.y + p.ny * side * (p.halfWidth - 1.4 - inset) + p.ty * along
            ];
            const split = reach * .68;
            for (const [u, v, colour] of [[0, split, '#ac78e6'], [split, reach, '#eee2ff']]) {
                draw(colour, [point(u, u * .55), point(v, v * .55), point(v, v * .55 + depth), point(u, u * .55 + depth)]);
            }
        }
        for (let i = 0; i < zones.length; i++) {
            const zone = zones[i];
            if (zone.to <= zone.from) continue;
            const a = AD.pointAt(track, zone.from), b = AD.pointAt(track, Math.min(zone.to, zone.from + 30));
            const outside = a.tx * b.ty - a.ty * b.tx >= 0 ? -1 : 1;
            const previous = zones[(i + zones.length - 1) % zones.length];
            const clear = zone.from - previous.to + (i === 0 ? track.length : 0);
            // Leave a short gap between adjacent scoring sections clear of another countdown.
            if (clear >= 52) {
                for (let count = 3; count > 0; count--) {
                    for (let bar = 0; bar < count; bar++) slash(zone.from - count * 16 + bar * 2.6, outside, 2.9, .95);
                }
            }
            for (let s = zone.from; s + 2.8 <= zone.to; s += 17) {
                slash(s, -1, 1.25, 2.1);
                slash(s, 1, 1.25, 2.1);
            }
        }
    };
    const foliage = new WeakMap();
    const meadow = ['#497342', '#729447', '#a6b761', '#fff6d4', '#f6bf43'];
    const woodland = ['#395d47', '#618954', '#94ac6d', '#ae88cf', '#d6b3e4'];
    const reeds = ['#58694d', '#879660', '#b5ad70', '#866548', '#c59d71'];
    const scrub = ['#82603f', '#a18b54', '#c3aa69', '#cdad6b', '#e8cf8b'];
    AD.vergePlants = function (track) {
        if (foliage.has(track)) return foliage.get(track);
        const plants = [];
        foliage.set(track, plants);
        if (track.biomeId === 'salt' || track.biomeId === 'glacier') return plants;
        const lush = track.biomeId === 'alpine' || track.biomeId === 'forest';
        const palette = { alpine: meadow, forest: woodland, harbour: reeds, metro: reeds }[track.biomeId] || scrub;
        const flowerType = track.biomeId === 'alpine' ? 'daisy' : track.biomeId === 'forest' ? 'spike' : 'seed';
        const occupied = AD.roadsideSites(track).concat(AD.petDecorPlacements?.(track) || [], AD.furnitureProps?.(track) || []);
        const rng = AD.rng(AD.hashString(track.id + '/verge/' + track.seed));
        for (let s = 58; s < track.length - 45; s += (lush ? 24 : 54) + rng() * 44) {
            const side = rng() < 0.5 ? 1 : -1, count = 2 + Math.floor(rng() * 4);
            for (let j = 0; j < count; j++) {
                const p = AD.pointAt(track, s + j * 2.6), offset = side * (p.halfWidth + 5 + rng() * 7);
                const x = p.x + p.nx * offset, y = p.y + p.ny * offset;
                if (track.biomeId === 'harbour' && typeof AD.propSide === 'function' && AD.propSide(track, 'harbour', x, y) === 'water') continue;
                const near = AD.sampleTrack(track, x, y), dist = Math.hypot(x - near.x, y - near.y);
                if (dist < near.halfWidth + 4 || occupied.some(o => Math.hypot(x - o.x, y - o.y) < o.r + 2)) continue;
                const z = AD.groundHeight?.(track, track.biomeId, x, y) || 0;
                if (z < -1 || (AD.terrainSlopeAt && AD.terrainSlopeAt(track, track.biomeId, x, y, 2.5).slope > 0.65)) continue;
                const shrub = rng() < 0.28, size = 0.9 + rng() * 0.85, angle = rng() * Math.PI * 2;
                const bloom = !shrub && Math.floor(s / 33) % 3 !== 0 && j % 3 !== 2 ? flowerType : null;
                plants.push({ x, y, z, palette, shrub, size, angle, bloom });
            }
        }
        return plants;
    };

    // World-space triangles feed the terrain mesh and the cached overhead map tiles.
    AD.vergeTriangles = function (p, face) {
        const point = (x, h, y) => [p.x + x * p.size, p.z + h * p.size, p.y + y * p.size];
        if (p.shrub) {
            for (let clump = 0; clump < 3; clump++) {
                const dir = p.angle + clump * 2.2, r = 0.78 - clump * 0.13;
                const x = Math.cos(dir) * clump * 0.31, y = Math.sin(dir) * clump * 0.31;
                const crown = point(x - 0.1, r * 0.96, y + 0.06);
                const rim = a => point(x + Math.cos(a) * r, -0.06, y + Math.sin(a) * r);
                for (let i = 0; i < 6; i++) face(crown, rim(dir + (i + 1) * Math.PI / 3), rim(dir + i * Math.PI / 3), i % 3);
            }
        } else {
            for (let i = 0; i < 8; i++) {
                const a = p.angle + i * 2.4, dx = Math.cos(a), dy = Math.sin(a), reach = 0.32 + (i % 4) * 0.17;
                const tip = point(dx * reach + .17, .48 + ((i * 3) % 5) * .12, dy * reach);
                const foot = point(dx * .12, -0.05, dy * .12);
                const left = point(dx * .34 - dy * .13, .38, dy * .34 + dx * .13);
                const right = point(dx * .34 + dy * .13, .38, dy * .34 - dx * .13);
                face(foot, left, right, i % 2); face(left, tip, right, 1 + i % 2);
            }
            if (!p.bloom) return;
            for (let i = 0; i < 3; i++) {
                const a = p.angle + i * 2.4, x = .64 * Math.cos(a), y = .56 * Math.sin(a), h = .9 + i * .21;
                const head = point(x, h, y);
                face(point(x * .6 - .07, 0, y * .6), head, point(x * .6 + .07, 0, y * .6 + .06), 1);
                if (p.bloom === 'daisy') {
                    // Tilt the flowers towards different sides so their faces read at driving height.
                    const flower = (dx, dy, lift) => point(x + dx, h + lift + .58 * (dx * Math.cos(a) + dy * Math.sin(a)), y + dy);
                    for (let j = 0; j < 5; j++) {
                        const dir = a + j * Math.PI * 2 / 5;
                        const petal = d => flower(Math.cos(dir + d) * .32, Math.sin(dir + d) * .32, .08);
                        face(head, petal(.53), petal(-.53), 3);
                    }
                    face(flower(-.10, -.08, .035), flower(0, .12, .07), flower(.12, -.06, .035), 4);
                } else {
                    const w = p.bloom === 'spike' ? .19 : .105, peak = point(x + .09, h + .43, y);
                    for (let j = 0; j < 4; j++) {
                        const rim = k => point(x + w * Math.cos(k * Math.PI / 2), h + .13, y + w * Math.sin(k * Math.PI / 2));
                        face(head, rim(j), rim(j + 1), 3); face(peak, rim(j + 1), rim(j), 3 + j % 2);
                    }
                }
            }
        }
    };

    // The two views share the lightning plate and the ribbon on the surprise crate.
    AD.paintSuperPad = function (g) {
        g.fillStyle = '#402138'; g.fillRect(0, 0, 100, 100);
        for (const y of [0, 87]) {
            g.fillStyle = '#ffa52c'; g.fillRect(0, y, 100, 13);
            g.fillStyle = '#fff0ac';
            for (let x = 3; x < 93; x += 15) {
                g.beginPath(); g.moveTo(x, y + 13); g.lineTo(x + 5, y);
                g.lineTo(x + 10, y); g.lineTo(x + 5, y + 13); g.fill();
            }
        }
        g.fillStyle = '#ffe97a';
        g.beginPath(); g.moveTo(12, 57); g.lineTo(54, 20); g.lineTo(49, 42);
        g.lineTo(89, 37); g.lineTo(41, 83); g.lineTo(47, 57); g.closePath(); g.fill();
    };
    AD.paintGift = function (g) {
        g.fillStyle = '#ffbb46'; g.fillRect(0, 0, 100, 100);
        g.fillStyle = '#ffd879'; g.fillRect(5, 5, 90, 90);
        g.fillStyle = '#983856'; g.fillRect(39, 0, 22, 100); g.fillRect(0, 39, 100, 22);
        g.fillStyle = '#c74a69'; g.fillRect(43, 0, 4, 100); g.fillRect(0, 43, 100, 4);
        g.beginPath();
        for (let i = 0; i < 10; i++) {
            const a = i * Math.PI / 5 - Math.PI / 2, r = i % 2 ? 12 : 27;
            const x = 50 + Math.cos(a) * r, y = 50 + Math.sin(a) * r;
            if (i) g.lineTo(x, y); else g.moveTo(x, y);
        }
        g.closePath(); g.fillStyle = '#fff4b4'; g.fill(); g.strokeStyle = '#713248'; g.lineWidth = 3; g.stroke();
    };

    /* Both road renderers use this waterline, in unit-ellipse coordinates. */
    AD.paintPuddle = function (g) {
        const water = g.createLinearGradient(-.7, -.8, .65, .9);
        water.addColorStop(0, '#394c5780'); water.addColorStop(.43, '#627785b5');
        water.addColorStop(.60, '#a8b6bac4'); water.addColorStop(1, '#52646b7a');
        g.fillStyle = water;
        g.beginPath(); g.moveTo(-.94, -.22);
        g.bezierCurveTo(-.85, -.61, -.62, -.52, -.45, -.82);
        g.bezierCurveTo(-.27, -1.04, -.05, -.78, .14, -.91);
        g.bezierCurveTo(.44, -1.07, .51, -.56, .76, -.49);
        g.bezierCurveTo(1.03, -.39, .86, -.15, .98, .04);
        g.bezierCurveTo(1.02, .31, .67, .40, .59, .65);
        g.bezierCurveTo(.43, .85, .26, .70, .01, .92);
        g.bezierCurveTo(-.35, 1.03, -.36, .69, -.63, .62);
        g.bezierCurveTo(-.83, .54, -.72, .29, -.94, .21);
        g.bezierCurveTo(-1.02, .11, -.88, -.04, -.94, -.22);
        g.closePath(); g.fill();
        g.strokeStyle = '#a7bdc244'; g.lineWidth = .017; g.stroke();
        g.strokeStyle = '#d6e2e3a6'; g.lineWidth = .013;
        for (const [x, y, w] of [[-.35, -.25, .19], [-.31, -.24, .25], [.29, .18, .22]]) {
            g.beginPath(); g.ellipse(x, y, .075, w, 0, -.95, 1.14); g.stroke();
        }
    };

    /* --- terrain -------------------------------------------------------- */
    AD.makeChunkCanvas = function (track, cx, cy) {
        const canvas = document.createElement('canvas');
        canvas.width = CHUNK_PX; canvas.height = CHUNK_PX;
        const c = canvas.getContext('2d');
        const biome = track.biome;
        const rng = AD.rng(AD.hashString(track.id + '/' + track.seed + '/' + cx + '/' + cy));
        c.fillStyle = biome.ground;
        c.fillRect(0, 0, CHUNK_PX, CHUNK_PX);

        /* Broad tonal patches break the flat fill. */
        for (let i = 0; i < 26; i++) {
            const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
            const r = 16 + rng() * 72;
            const g = c.createRadialGradient(x, y, 0, x, y, r);
            const tint = rng() > 0.5 ? biome.groundAlt : biome.edge;
            g.addColorStop(0, tint);
            g.addColorStop(1, 'rgba(0,0,0,0)');
            c.globalAlpha = 0.05 + rng() * 0.08;
            c.fillStyle = g;
            c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill();
        }
        c.globalAlpha = 1;

        /* Fine grain: stones, grit and grass blades. */
        for (let i = 0; i < 520; i++) {
            const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
            c.fillStyle = rng() > 0.55 ? biome.edge : biome.groundAlt;
            c.globalAlpha = 0.05 + rng() * 0.13;
            c.fillRect(x, y, 1 + rng() * 1.4, 1 + rng() * 1.4);
        }
        c.globalAlpha = 1;
        for (let i = 0; i < 90; i++) {
            const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
            c.strokeStyle = rng() > 0.5 ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.08)';
            c.lineWidth = 1;
            c.beginPath(); c.moveTo(x, y); c.lineTo(x + (rng() - 0.5) * 4, y - 1 - rng() * 3); c.stroke();
        }

        const id = track.biomeId;
        if (id === 'salt') {
            /* Polygonal salt crust. */
            c.strokeStyle = 'rgba(120,130,125,0.16)'; c.lineWidth = 1;
            for (let i = 0; i < 16; i++) {
                c.beginPath();
                let x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
                c.moveTo(x, y);
                for (let j = 0; j < 4; j++) { x += (rng() - 0.5) * 48; y += (rng() - 0.5) * 48; c.lineTo(x, y); }
                c.stroke();
            }
        } else if (id === 'desert') {
            /* Wind-blown sand ripples. */
            c.strokeStyle = 'rgba(228,190,130,0.10)'; c.lineWidth = 1.5;
            for (let i = 0; i < 10; i++) {
                const y = rng() * CHUNK_PX;
                c.beginPath();
                c.moveTo(0, y);
                for (let x = 0; x <= CHUNK_PX; x += 24) c.lineTo(x, y + Math.sin(x * 0.05 + rng() * 2) * 5);
                c.stroke();
            }
        } else if (id === 'harbour' || id === 'metro') {
            /* Poured concrete slabs with expansion joints. */
            c.strokeStyle = 'rgba(140,170,200,0.10)'; c.lineWidth = 2;
            for (let i = 0; i < 6; i++) {
                const x = rng() * CHUNK_PX;
                c.beginPath(); c.moveTo(x, 0); c.lineTo(x, CHUNK_PX); c.stroke();
            }
            c.strokeStyle = 'rgba(0,0,0,0.12)';
            for (let i = 0; i < 4; i++) {
                const y = rng() * CHUNK_PX;
                c.beginPath(); c.moveTo(0, y); c.lineTo(CHUNK_PX, y); c.stroke();
            }
        } else if (id === 'industry') {
            /* Steel decking with oil staining. */
            c.strokeStyle = 'rgba(150,165,180,0.10)'; c.lineWidth = 2;
            for (let i = 0; i < 6; i++) {
                const x = rng() * CHUNK_PX;
                c.beginPath(); c.moveTo(x, 0); c.lineTo(x, CHUNK_PX); c.stroke();
            }
            for (let i = 0; i < 5; i++) {
                const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX, r = 10 + rng() * 26;
                const g = c.createRadialGradient(x, y, 0, x, y, r);
                g.addColorStop(0, 'rgba(20,14,10,0.30)');
                g.addColorStop(1, 'rgba(20,14,10,0)');
                c.fillStyle = g;
                c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill();
            }
        } else if (id === 'forest') {
            /* Leaf litter and moss. */
            for (let i = 0; i < 160; i++) {
                const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
                c.fillStyle = rng() > 0.5 ? 'rgba(126,158,90,0.16)' : 'rgba(58,44,26,0.18)';
                c.beginPath();
                c.ellipse(x, y, 1.5 + rng() * 3, 1 + rng() * 2, rng() * 3, 0, Math.PI * 2);
                c.fill();
            }
        } else if (id === 'alpine') {
            /* Scree and old snow patches. */
            for (let i = 0; i < 90; i++) {
                const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX, r = 8 + rng() * 30;
                const g = c.createRadialGradient(x, y, 0, x, y, r);
                g.addColorStop(0, 'rgba(232,240,246,0.14)');
                g.addColorStop(1, 'rgba(232,240,246,0)');
                c.fillStyle = g;
                c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill();
            }
        } else if (id === 'glacier') {
            /* Blue ice veins and drift snow. */
            for (let i = 0; i < 70; i++) {
                const x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
                const g = c.createRadialGradient(x, y, 0, x, y, 12 + rng() * 40);
                g.addColorStop(0, 'rgba(198,230,255,0.18)');
                g.addColorStop(1, 'rgba(198,230,255,0)');
                c.fillStyle = g;
                c.beginPath(); c.arc(x, y, 12 + rng() * 40, 0, Math.PI * 2); c.fill();
            }
            c.strokeStyle = 'rgba(214,238,255,0.18)'; c.lineWidth = 1.4;
            for (let i = 0; i < 10; i++) {
                c.beginPath();
                let x = rng() * CHUNK_PX, y = rng() * CHUNK_PX;
                c.moveTo(x, y);
                for (let j = 0; j < 5; j++) { x += (rng() - 0.5) * 60; y += (rng() - 0.5) * 60; c.lineTo(x, y); }
                c.stroke();
            }
        }
        const px = CHUNK_PX / AD.CHUNK_METRES, left = cx * AD.CHUNK_METRES, top = cy * AD.CHUNK_METRES;
        c.setTransform(px, 0, 0, px, -left * px, -top * px);
        for (const plant of AD.vergePlants(track)) {
            if (plant.x < left - 3 || plant.y < top - 3 || plant.x > left + AD.CHUNK_METRES + 3 || plant.y > top + AD.CHUNK_METRES + 3) continue;
            AD.vergeTriangles(plant, (a, b, d, shade) => {
                c.fillStyle = plant.palette[shade]; c.beginPath();
                c.moveTo(a[0], a[2]); c.lineTo(b[0], b[2]); c.lineTo(d[0], d[2]); c.closePath(); c.fill();
            });
        }
        return canvas;
    };

    /* --- scenery props --------------------------------------------------- */
    function propShape(c, prop, size) {
        switch (prop.kind) {
            case 'alpine': {
                c.fillStyle = '#4a3b2c';
                c.fillRect(-size * 0.07, size * 0.1, size * 0.14, size * 0.38);
                c.fillStyle = '#1c3323';
                c.beginPath(); c.moveTo(-size * 0.52, size * 0.22); c.lineTo(0, -size * 0.72); c.lineTo(size * 0.52, size * 0.22); c.closePath(); c.fill();
                c.fillStyle = '#2c5236';
                c.beginPath(); c.moveTo(-size * 0.38, -size * 0.02); c.lineTo(0, -size * 0.62); c.lineTo(size * 0.38, -size * 0.02); c.closePath(); c.fill();
                c.fillStyle = 'rgba(226,238,246,0.5)';
                c.beginPath(); c.moveTo(-size * 0.16, -size * 0.36); c.lineTo(0, -size * 0.62); c.lineTo(size * 0.16, -size * 0.36); c.closePath(); c.fill();
                break;
            }
            case 'desert':
                if (prop.size > 1.1) {
                    c.fillStyle = '#7d5a34';
                    c.beginPath(); c.moveTo(-size * 0.55, size * 0.42); c.lineTo(-size * 0.1, -size * 0.62); c.lineTo(size * 0.58, size * 0.42); c.closePath(); c.fill();
                    c.fillStyle = '#b98a4c';
                    c.beginPath(); c.moveTo(-size * 0.24, size * 0.42); c.lineTo(-size * 0.06, -size * 0.42); c.lineTo(size * 0.16, size * 0.42); c.closePath(); c.fill();
                } else {
                    c.fillStyle = '#4e7c46';
                    c.fillRect(-size * 0.12, -size * 0.5, size * 0.24, size * 0.95);
                    c.fillRect(-size * 0.42, -size * 0.22, size * 0.3, size * 0.16);
                    c.fillRect(size * 0.12, -size * 0.38, size * 0.3, size * 0.16);
                    c.fillStyle = '#6d9c5c';
                    c.fillRect(-size * 0.07, -size * 0.5, size * 0.08, size * 0.95);
                }
                break;
            case 'harbour': {
                const hue = ['#2c4055', '#5a3b46', '#3b5546', '#57492e'][Math.floor(prop.size * 3) % 4];
                c.fillStyle = hue;
                c.fillRect(-size * 0.62, -size * 0.36, size * 1.24, size * 0.72);
                c.strokeStyle = 'rgba(190,215,235,0.32)'; c.lineWidth = 1;
                for (let i = -2; i <= 2; i++) {
                    c.beginPath();
                    c.moveTo(i * size * 0.22, -size * 0.36); c.lineTo(i * size * 0.22, size * 0.36);
                    c.stroke();
                }
                c.strokeRect(-size * 0.62, -size * 0.36, size * 1.24, size * 0.72);
                break;
            }
            case 'salt':
                c.fillStyle = '#8d959c';
                c.fillRect(-size * 0.05, -size * 0.7, size * 0.1, size * 1.05);
                c.fillStyle = '#e8564a';
                c.fillRect(-size * 0.08, -size * 0.78, size * 0.16, size * 0.18);
                c.fillStyle = 'rgba(255,255,255,0.55)';
                c.fillRect(-size * 0.03, -size * 0.5, size * 0.06, size * 0.3);
                break;
            case 'forest':
                c.fillStyle = '#4a3b2c';
                c.fillRect(-size * 0.08, size * 0.08, size * 0.16, size * 0.4);
                c.fillStyle = '#1d3a26';
                c.beginPath(); c.arc(0, -size * 0.1, size * 0.58, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#315c39';
                c.beginPath(); c.arc(-size * 0.16, -size * 0.22, size * 0.38, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#4a7c4c';
                c.beginPath(); c.arc(-size * 0.22, -size * 0.34, size * 0.2, 0, Math.PI * 2); c.fill();
                break;
            case 'industry':
                if (prop.size > 1.15) {
                    c.fillStyle = '#4a5057';
                    c.beginPath(); c.arc(0, 0, size * 0.52, 0, Math.PI * 2); c.fill();
                    c.strokeStyle = '#6b747d'; c.lineWidth = 2;
                    c.beginPath(); c.arc(0, 0, size * 0.52, 0, Math.PI * 2); c.stroke();
                    c.fillStyle = '#77828c';
                    c.beginPath(); c.arc(0, 0, size * 0.16, 0, Math.PI * 2); c.fill();
                } else {
                    c.fillStyle = '#4a5057'; c.fillRect(-size * 0.4, -size * 0.55, size * 0.8, size * 1.1);
                    c.fillStyle = '#ff9d4a'; c.globalAlpha = 0.55;
                    c.fillRect(-size * 0.25, -size * 0.3, size * 0.5, size * 0.14);
                    c.fillRect(-size * 0.25, -size * 0.02, size * 0.5, size * 0.14);
                    c.globalAlpha = 1;
                    c.fillStyle = '#2f343a'; c.fillRect(-size * 0.4, -size * 0.55, size * 0.8, size * 0.12);
                }
                break;
            case 'glacier':
                c.fillStyle = 'rgba(188,222,255,0.55)';
                c.beginPath();
                c.moveTo(-size * 0.5, size * 0.4); c.lineTo(-size * 0.3, -size * 0.5);
                c.lineTo(size * 0.1, -size * 0.66); c.lineTo(size * 0.52, size * 0.36);
                c.closePath(); c.fill();
                c.fillStyle = 'rgba(240,250,255,0.6)';
                c.beginPath();
                c.moveTo(-size * 0.16, size * 0.38); c.lineTo(size * 0.02, -size * 0.42);
                c.lineTo(size * 0.2, -size * 0.36); c.lineTo(size * 0.08, size * 0.38);
                c.closePath(); c.fill();
                break;
            case 'metro': {
                c.fillStyle = '#3c4148';
                c.fillRect(-size * 0.5, -size * 0.34, size * 1.0, size * 0.68);
                c.fillStyle = '#5b636d';
                c.fillRect(-size * 0.5, -size * 0.34, size * 1.0, size * 0.12);
                c.fillStyle = 'rgba(120,220,255,0.5)';
                c.fillRect(-size * 0.34, -size * 0.16, size * 0.68, size * 0.1);
                break;
            }
            default:
                c.fillStyle = '#8f9a86';
                c.beginPath(); c.arc(0, 0, size * 0.4, 0, Math.PI * 2); c.fill();
        }
    }

    AD.drawProp = function (c, prop, sx, sy, scale) {
        const size = prop.size * 3.3 * scale;
        c.save(); c.translate(sx, sy); c.rotate(prop.angle);
        c.fillStyle = 'rgba(6,10,16,0.30)';
        c.beginPath(); c.ellipse(size * 0.16, size * 0.2, size * 0.62, size * 0.4, 0, 0, Math.PI * 2); c.fill();
        propShape(c, prop, size);
        /* Rim light from the top-left grounds every prop in sun and neon. */
        c.fillStyle = 'rgba(255,255,255,0.08)';
        c.beginPath(); c.arc(-size * 0.15, -size * 0.2, size * 0.3, 0, Math.PI * 2); c.fill();
        c.restore();
    };

    /* --- trackside furniture --------------------------------------------- */
    /* Every item is world anchored, deterministic per track and kept clear of
       the drivable corridor so it never becomes an invisible wall. */
    AD.trackFurniture = function (track) {
        const rng = AD.rng(AD.hashString(track.id + '/furniture/' + track.seed));
        const items = [];
        const biomeId = track.biomeId;

        function clear(x, y, margin) {
            const sample = AD.sampleTrack(track, x, y);
            return Math.abs(sample.offset) > sample.halfWidth + (margin || 3.5);
        }

        function place(kind, s, side, extra) {
            const p = AD.pointAt(track, s);
            const offset = p.halfWidth + (extra && extra.offset !== undefined ? extra.offset : 4.2);
            const x = p.x + p.nx * offset * side;
            const y = p.y + p.ny * offset * side;
            if (!clear(x, y, (extra && extra.margin) || 2.5)) return null;
            const item = Object.assign({
                kind: kind, x: x, y: y,
                angle: Math.atan2(p.ny * side, p.nx * side),
                size: 1, seed: Math.floor(rng() * 100000)
            }, extra || {});
            items.push(item);
            return item;
        }

        /* Grandstands and pit structures frame the start straight. */
        const standOffsets = [18, 34, -26];
        for (const off of standOffsets) {
            place('grandstand', track.length * 0.02 + off, off < 0 ? -1 : 1, { offset: 9 + rng() * 6, width: 26 + rng() * 10 });
        }
        place('paddock', track.length * 0.94, -1, { offset: 11, width: 22 });

        /* Sponsor hoardings every 70 m, alternating sides. */
        for (let s = 0, i = 0; s < track.length; s += 70, i++) {
            place('board', s, i % 2 === 0 ? 1 : -1, { width: 12 + rng() * 5, accent: i % 3 });
        }

        /* Marshal posts and braking boards at the tight corners. */
        const samples = track.samples;
        for (let i = 0; i < samples.length; i += 11) {
            const a = samples[i];
            const b = samples[(i + 16) % samples.length];
            const cross = Math.abs(a.tx * b.ty - a.ty * b.tx);
            if (cross < 0.2 || cross > 0.85) continue;
            const turn = a.tx * b.ny - a.ty * b.nx;
            const side = turn > 0 ? 1 : -1;
            place('marshal', a.s + 12, side, {});
            place('marker100', a.s + 46, side, { label: '100' });
            place('marker50', a.s + 66, side, { label: '50' });
            place('tyrewall', a.s + 8, side, { width: 16 });
            if (items.length > 260) break;
        }

        /* Safety fencing runs beside the fastest sections. */
        for (let s = 0; s < track.length; s += 46) {
            if (rng() > 0.55) continue;
            place('fence', s, rng() > 0.5 ? 1 : -1, { width: 22 });
        }

        /* Biome set pieces: the things that make a venue recognisable. */
        const setPieces = {
            alpine: ['pine', 'pine', 'rock', 'cabin'],
            desert: ['cactus', 'rock', 'billboard', 'dune'],
            harbour: ['crane', 'container', 'container', 'lightmast'],
            salt: ['beacon', 'sign', 'rock'],
            forest: ['pine', 'pine', 'stump', 'hut'],
            industry: ['silo', 'pipe', 'pylon', 'lightmast'],
            glacier: ['iceblock', 'iceblock', 'mast', 'aurora'],
            metro: ['neon', 'neon', 'lightmast', 'barrier']
        };
        const kinds = setPieces[biomeId] || ['rock'];
        for (let i = 0; i < 90; i++) {
            const s = rng() * track.length;
            const p = AD.pointAt(track, s);
            const side = rng() > 0.5 ? 1 : -1;
            const offset = p.halfWidth + 12 + rng() * 90;
            const x = p.x + p.nx * offset * side;
            const y = p.y + p.ny * offset * side;
            if (!clear(x, y, 4)) continue;
            items.push({
                kind: kinds[Math.floor(rng() * kinds.length)],
                x: x, y: y, angle: rng() * Math.PI * 2, size: 0.8 + rng() * 0.9,
                seed: Math.floor(rng() * 100000)
            });
        }
        const stages = AD.roadsideSites(track);
        return items.filter(item => stages.every(site => Math.hypot(item.x - site.x, item.y - site.y) > site.r + (item.width || 6) / 2));
    };

    AD.drawFurniture = function (c, item, sx, sy, scale, frame, time = 0) {
        const s = scale;
        const size = (item.size || 1) * s;
        c.save();
        c.translate(sx, sy);
        c.rotate(item.angle);
        const accent = item.accent === 1 ? '#5eead4' : (item.accent === 2 ? '#ffd166' : '#ff5a4e');

        switch (item.kind) {
            case 'grandstand': {
                const w = (item.width || 24) * s;
                const d = 8 * s;
                c.fillStyle = 'rgba(6,10,16,0.32)';
                c.fillRect(-d / 2 + 2 * s, -w / 2 + 2 * s, d, w);
                /* Tiers of seating with a dense crowd: the stand has to read
                   as a building full of people, not a grey slab. */
                const tiers = 4;
                const seatDepth = d * 0.55;
                for (let i = 0; i < tiers; i++) {
                    const x = -d / 2 + (i / tiers) * seatDepth;
                    const band = seatDepth / tiers;
                    const pulse = AD.roadsidePulse(item.reaction, time, i * .055);
                    const shift = pulse * s * (item.reaction?.kind === 'drift' ? -1.35 : 1.15);
                    c.fillStyle = i % 2 ? '#2b313a' : '#242a33';
                    c.fillRect(x, -w / 2, band, w);
                    for (let j = 0; j < 30; j++) {
                        const cy = -w / 2 + ((j * 37 + i * 13) % w);
                        const cx = x + band * (0.22 + ((j * 17) % 52) / 100);
                        c.fillStyle = ['rgba(226,234,246,0.55)', 'rgba(160,190,220,0.45)',
                            'rgba(236,198,162,0.42)', 'rgba(200,214,236,0.38)'][j % 4];
                        c.fillRect(cx + shift, cy, (1.1 + pulse * .5) * s, 1.7 * s);
                    }
                }
                /* Roof over the outer half, carried on pillars. */
                c.fillStyle = '#39404a';
                c.fillRect(d * 0.04, -w / 2 - 1.2 * s, d * 0.46, w + 2.4 * s);
                c.fillStyle = '#4d5764';
                c.fillRect(d * 0.04, -w / 2 - 1.2 * s, d * 0.46, 1.5 * s);
                c.fillStyle = 'rgba(16,20,26,0.85)';
                for (let i = 0; i <= 6; i++) {
                    const cy = -w / 2 + (i / 6) * w;
                    c.fillRect(d * 0.02, cy - 0.5 * s, d * 0.5, 1 * s);
                }
                /* Safety fence between the stand and the track. */
                c.fillStyle = 'rgba(190,205,225,0.20)';
                c.fillRect(-d / 2 - 0.7 * s, -w / 2, 0.5 * s, w);
                break;
            }
            case 'paddock': {
                const w = (item.width || 22) * s;
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-4 * s + 2 * s, -w / 2 + 2 * s, 8 * s, w);
                c.fillStyle = '#2f353e';
                c.fillRect(-4 * s, -w / 2, 8 * s, w);
                /* Garage bays with lit doorways. */
                for (let i = 0; i < 5; i++) {
                    const y = -w / 2 + (i + 0.5) * (w / 5);
                    c.fillStyle = '#171b21';
                    c.fillRect(-3.2 * s, y - 1.8 * s, 5.4 * s, 3.6 * s);
                    c.fillStyle = 'rgba(130,160,190,0.20)';
                    c.fillRect(-3.2 * s, y - 1.8 * s, 5.4 * s, 0.8 * s);
                }
                /* Roof band in the team colour. */
                c.fillStyle = '#454e5a';
                c.fillRect(-4.2 * s, -w / 2, 8.4 * s, 1.8 * s);
                c.fillStyle = accent;
                c.globalAlpha = 0.85;
                c.fillRect(-4.2 * s, -w / 2 + 1.2 * s, 8.4 * s, 0.6 * s);
                c.globalAlpha = 1;
                break;
            }
            case 'board': {
                const w = (item.width || 12) * s;
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-1.4 * s + 1.4 * s, -w / 2 + 1.4 * s, 2.8 * s, w);
                c.fillStyle = '#e8ebef';
                c.fillRect(-1.4 * s, -w / 2, 2.8 * s, w);
                c.fillStyle = accent;
                for (let i = 0; i < 3; i++) {
                    const y = -w / 2 + (i + 0.6) * (w / 3.4);
                    c.fillRect(-1.1 * s, y, 2.2 * s, w / 8);
                }
                c.fillStyle = 'rgba(20,24,30,0.55)';
                c.fillRect(-1.4 * s, -w / 2, 2.8 * s, 0.7 * s);
                break;
            }
            case 'tyrewall': {
                const w = (item.width || 16) * s;
                const r = 1.5 * s;
                for (let i = 0; i < w / (r * 2.1); i++) {
                    const y = -w / 2 + i * r * 2.1;
                    c.fillStyle = 'rgba(8,10,14,0.55)';
                    c.beginPath(); c.arc(0, y, r, 0, Math.PI * 2); c.fill();
                    c.fillStyle = i % 2 ? '#e8ecef' : '#c22f2f';
                    c.beginPath(); c.arc(0, y, r * 0.8, 0, Math.PI * 2); c.fill();
                    c.fillStyle = 'rgba(8,10,14,0.55)';
                    c.beginPath(); c.arc(0, y, r * 0.34, 0, Math.PI * 2); c.fill();
                }
                break;
            }
            case 'fence': {
                const w = (item.width || 22) * s;
                c.strokeStyle = 'rgba(190,205,225,0.22)';
                c.lineWidth = 0.7 * s;
                c.beginPath(); c.moveTo(0, -w / 2); c.lineTo(0, w / 2); c.stroke();
                for (let i = 0; i <= 8; i++) {
                    const y = -w / 2 + (i / 8) * w;
                    c.fillStyle = 'rgba(120,132,148,0.5)';
                    c.fillRect(-0.5 * s, y - 0.5 * s, 1.1 * s, 1.1 * s);
                }
                break;
            }
            case 'marshal': {
                c.fillStyle = 'rgba(6,10,16,0.32)';
                c.beginPath(); c.ellipse(0.6 * s, 1.4 * s, 2.6 * s, 1.8 * s, 0, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#e8ebef';
                c.fillRect(-1.6 * s, -1.6 * s, 3.2 * s, 3.2 * s);
                c.fillStyle = '#ffcf6b';
                c.fillRect(-1.6 * s, -1.6 * s, 3.2 * s, 1.1 * s);
                /* Waving flag. */
                const wave = Math.sin((frame || 0) * 0.12) * 0.3;
                c.save();
                c.rotate(wave);
                c.fillStyle = '#ff5a4e';
                c.beginPath();
                c.moveTo(0, 0); c.lineTo(3.6 * s, -0.8 * s); c.lineTo(0, -1.7 * s);
                c.closePath(); c.fill();
                c.restore();
                break;
            }
            case 'marker100':
            case 'marker50': {
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.beginPath(); c.ellipse(0.5 * s, 1.2 * s, 1.9 * s, 1.2 * s, 0, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#f4f6f2';
                c.fillRect(-1.3 * s, -1.3 * s, 2.6 * s, 2.6 * s);
                c.fillStyle = '#12141a';
                c.font = 'bold ' + (1.5 * s) + 'px system-ui, sans-serif';
                c.textAlign = 'center';
                c.textBaseline = 'middle';
                c.fillText(item.label || '50', 0, 0.1 * s);
                break;
            }
            case 'crane': {
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-1 * s + 3 * s, -2 * s + 3 * s, 2 * s, 4 * s);
                c.fillStyle = '#6b747d';
                c.fillRect(-1 * s, -2 * s, 2 * s, 4 * s);
                c.fillStyle = '#8b959f';
                c.fillRect(-1 * s, -30 * s, 2 * s, 30 * s);
                c.fillRect(-1 * s, -30 * s, 26 * s, 2 * s);
                c.fillStyle = '#ffd166';
                c.fillRect(-1 * s, -28.6 * s, 26 * s, 0.8 * s);
                break;
            }
            case 'container': {
                const hue = ['#2c4055', '#5a3b46', '#3b5546', '#57492e'][(item.seed || 0) % 4];
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-5 * s + 1.6 * s, -3.2 * s + 1.6 * s, 10 * s, 6.4 * s);
                c.fillStyle = hue;
                c.fillRect(-5 * s, -3.2 * s, 10 * s, 6.4 * s);
                c.strokeStyle = 'rgba(190,215,235,0.28)'; c.lineWidth = 0.8 * s;
                for (let i = -2; i <= 2; i++) {
                    c.beginPath();
                    c.moveTo(i * 2 * s, -3.2 * s); c.lineTo(i * 2 * s, 3.2 * s);
                    c.stroke();
                }
                break;
            }
            case 'silo': {
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.beginPath(); c.ellipse(1.4 * s, 1.8 * s, 4 * s, 3.4 * s, 0, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#6a7079';
                c.beginPath(); c.arc(0, 0, 4 * s, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#8d959f';
                c.beginPath(); c.arc(-0.8 * s, -0.8 * s, 2.2 * s, 0, Math.PI * 2); c.fill();
                break;
            }
            case 'pipe': {
                c.fillStyle = 'rgba(6,10,16,0.28)';
                c.fillRect(-1.4 * s + 1.2 * s, -12 * s + 1.2 * s, 2.8 * s, 24 * s);
                c.fillStyle = '#5a6169';
                c.fillRect(-1.4 * s, -12 * s, 2.8 * s, 24 * s);
                c.fillStyle = 'rgba(255,255,255,0.12)';
                c.fillRect(-1.4 * s, -12 * s, 0.8 * s, 24 * s);
                break;
            }
            case 'pylon': {
                c.strokeStyle = 'rgba(120,132,148,0.55)';
                c.lineWidth = 0.9 * s;
                c.beginPath();
                c.moveTo(-2.4 * s, 5 * s); c.lineTo(0, -6 * s); c.lineTo(2.4 * s, 5 * s);
                c.moveTo(-1.6 * s, 1 * s); c.lineTo(1.6 * s, 1 * s);
                c.moveTo(-0.8 * s, -3 * s); c.lineTo(0.8 * s, -3 * s);
                c.stroke();
                break;
            }
            case 'lightmast': {
                c.fillStyle = 'rgba(6,10,16,0.26)';
                c.beginPath(); c.ellipse(0.8 * s, 4 * s, 1.2 * s, 1.2 * s, 0, 0, Math.PI * 2); c.fill();
                c.fillStyle = '#3a414a';
                c.fillRect(-0.5 * s, -7 * s, 1 * s, 11 * s);
                c.fillStyle = '#20252c';
                c.fillRect(-1.8 * s, -7.6 * s, 3.6 * s, 1.4 * s);
                c.fillStyle = 'rgba(255,214,150,0.9)';
                c.fillRect(-1.4 * s, -6.4 * s, 2.8 * s, 0.7 * s);
                break;
            }
            case 'mast': {
                c.strokeStyle = 'rgba(190,205,225,0.5)';
                c.lineWidth = 1 * s;
                c.beginPath(); c.moveTo(0, 4 * s); c.lineTo(0, -9 * s); c.stroke();
                c.beginPath();
                c.moveTo(-3 * s, 4 * s); c.lineTo(0, -2 * s); c.lineTo(3 * s, 4 * s);
                c.stroke();
                const blink = ((frame || 0) + (item.seed || 0)) % 90 < 45;
                c.fillStyle = blink ? '#ff4d4d' : 'rgba(255,77,77,0.25)';
                c.beginPath(); c.arc(0, -9 * s, 0.9 * s, 0, Math.PI * 2); c.fill();
                break;
            }
            case 'aurora': {
                /* A slow ribbon of northern light over the glacier. */
                const g = c.createLinearGradient(-14 * s, 0, 14 * s, 0);
                g.addColorStop(0, 'rgba(94,234,212,0)');
                g.addColorStop(0.4, 'rgba(94,234,212,0.30)');
                g.addColorStop(0.7, 'rgba(159,122,255,0.22)');
                g.addColorStop(1, 'rgba(94,234,212,0)');
                c.fillStyle = g;
                c.beginPath();
                c.moveTo(-14 * s, -2 * s);
                for (let i = 0; i <= 8; i++) {
                    const x = -14 * s + i * 3.5 * s;
                    c.lineTo(x, -2 * s + Math.sin(i * 0.9 + (frame || 0) * 0.02) * 2.4 * s);
                }
                c.lineTo(14 * s, 3 * s);
                for (let i = 8; i >= 0; i--) {
                    const x = -14 * s + i * 3.5 * s;
                    c.lineTo(x, 3 * s + Math.sin(i * 0.9 + (frame || 0) * 0.02) * 2.4 * s);
                }
                c.closePath(); c.fill();
                break;
            }
            case 'iceblock': {
                c.fillStyle = 'rgba(188,222,255,0.28)';
                c.beginPath();
                c.moveTo(-4 * s, 3 * s); c.lineTo(-2.6 * s, -3.4 * s);
                c.lineTo(1 * s, -4.4 * s); c.lineTo(4.2 * s, 2.8 * s);
                c.closePath(); c.fill();
                c.strokeStyle = 'rgba(230,246,255,0.45)'; c.lineWidth = 0.8 * s;
                c.stroke();
                break;
            }
            case 'snowdrift': {
                c.fillStyle = 'rgba(236,244,252,0.4)';
                c.beginPath(); c.ellipse(0, 0, 6 * s, 3 * s, 0.3, 0, Math.PI * 2); c.fill();
                break;
            }
            case 'neon': {
                const pulse = 0.7 + 0.3 * Math.sin((frame || 0) * 0.08 + (item.seed || 0));
                c.fillStyle = 'rgba(8,10,16,0.4)';
                c.fillRect(-1.6 * s + 1.4 * s, -5 * s + 1.4 * s, 3.2 * s, 10 * s);
                c.fillStyle = 'rgba(18,22,30,0.95)';
                c.fillRect(-1.6 * s, -5 * s, 3.2 * s, 10 * s);
                c.fillStyle = accent;
                c.globalAlpha = 0.55 + pulse * 0.45;
                c.fillRect(-1.2 * s, -4.4 * s, 2.4 * s, 8.8 * s);
                c.globalAlpha = 1;
                c.fillStyle = 'rgba(255,255,255,0.5)';
                c.fillRect(-1.2 * s, -1.2 * s, 2.4 * s, 2.4 * s);
                break;
            }
            case 'barrier': {
                c.fillStyle = 'rgba(6,10,16,0.3)';
                c.fillRect(-0.8 * s + 1.2 * s, -5 * s + 1.2 * s, 1.6 * s, 10 * s);
                c.fillStyle = '#c9ced6';
                c.fillRect(-0.8 * s, -5 * s, 1.6 * s, 10 * s);
                c.fillStyle = '#ffcf6b';
                for (let i = 0; i < 4; i++) c.fillRect(-0.8 * s, -5 * s + i * 2.5 * s, 1.6 * s, 1.2 * s);
                break;
            }
            case 'pine':
            case 'rock':
            case 'cactus':
            case 'billboard':
            case 'dune':
            case 'beacon':
            case 'sign':
            case 'stump':
            case 'hut':
            case 'cabin': {
                /* Landscape props reuse the chunk prop vocabulary, mapped to a
                   readable silhouette per biome. */
                const map = {
                    pine: 'forest', rock: 'salt', cactus: 'desert', billboard: 'salt',
                    dune: 'desert', beacon: 'salt', sign: 'salt', stump: 'forest',
                    hut: 'alpine', cabin: 'alpine'
                };
                AD.drawProp(c, {
                    kind: map[item.kind] || 'alpine',
                    size: 1.5, angle: (item.seed || 0) % 7
                }, 0, 0, s * 0.85);
                break;
            }
            default:
                break;
        }
        c.restore();
    };
})();
