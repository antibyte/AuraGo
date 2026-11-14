/* Aura Drift - optional, stationary pet mascots beside the track. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const PETS = {
        punk: { height: 3.2, radius: 2.5, clips: ['idle-a', 'greeting', 'clapping'] },
        hoodie: { height: 3.2, radius: 2.5, clips: ['idle-a', 'greeting', 'clapping'] },
        pippin: { height: 2.4, radius: 2.5, clips: ['idle-a', 'greeting', 'clapping'] },
        tux: { height: 2.7, radius: 2.5, clips: ['idle-a', 'greeting', 'clapping'] },
        buddy: { height: 2.2, radius: 3, clips: ['idle-a', 'greeting', 'head-nod'] },
        rexi: { height: 3.8, radius: 4, clips: ['idle-a', 'greeting', 'victory-fist-pump'] },
        kroko: { height: 2.1, radius: 4, clips: ['idle-a', 'greeting', 'head-nod'] },
        zorbit: { height: 3, radius: 2.5, clips: ['idle-a', 'greeting', 'head-nod'] },
        'purple-tentacle': { height: 3.2, radius: 2.5, clips: ['idle-a', 'greeting', 'victory-fist-pump'] }
    };
    const BIOMES = {
        alpine: ['pippin', 'buddy'], desert: ['rexi', 'zorbit'],
        harbour: ['punk', 'purple-tentacle'], salt: ['zorbit', 'purple-tentacle'],
        forest: ['rexi', 'kroko'], industry: ['hoodie', 'purple-tentacle'],
        glacier: ['tux', 'pippin'], metro: ['punk', 'hoodie']
    };
    // The offline motion builder reads this same, fixed roster; imported pets cannot add URLs.
    AD.PET_DECOR = { pets: PETS, biomes: BIOMES };

    AD.petDecorPlacements = function (track) {
        if (!Object.hasOwn(BIOMES, track.biomeId)) return [];
        const out = [], occupied = typeof AD.furnitureProps === 'function' ? AD.furnitureProps(track) : [];
        const spots = [track.length - 24, 16, track.length * .2, track.length * .42, track.length * .64, track.length * .84];
        for (let i = 0; i < spots.length; i++) {
            const folder = BIOMES[track.biomeId][i % 2], spec = PETS[folder], side = i % 2 ? -1 : 1;
            for (let attempt = 0; attempt < 24; attempt++) {
                const s = ((spots[i] + attempt * 12) % track.length + track.length) % track.length;
                const p = AD.pointAt(track, s), off = side * (p.halfWidth + spec.radius + 5);
                const x = p.x + p.nx * off, y = p.y + p.ny * off, near = AD.sampleTrack(track, x, y);
                if (Math.hypot(x - near.x, y - near.y) < near.halfWidth + spec.radius + 3) continue;
                const a = track.samples[near.index], b = track.samples[(near.index + 4) % track.samples.length];
                if (track.curvature[near.index] > .014 && Math.sign(a.tx * b.ty - a.ty * b.tx) === Math.sign(near.offset)) continue;
                if (occupied.concat(out).some(o => Math.hypot(x - o.x, y - o.y) < spec.radius + o.r + 2)) continue;
                let z = 0;
                if (typeof AD.terrainHeight === 'function') z = AD.terrainHeight(track.biomeId, Math.hypot(x - near.x, y - near.y), x, y, near);
                if (!Number.isFinite(z) || (track.biomeId === 'harbour' && z < -1)) continue;
                out.push({ folder, x, y, z, s, r: spec.radius, h: spec.height, furniture: true,
                    yaw: Math.atan2(-side * p.nx * .8 - p.tx * .6, -side * p.ny * .8 - p.ty * .6) });
                break;
            }
        }
        return out;
    };

    function disposeTemplate(root) {
        const geometries = new Set(), materials = new Set(), textures = new Set();
        root.traverse(o => {
            if (o.geometry) geometries.add(o.geometry);
            for (const m of Array.isArray(o.material) ? o.material : o.material ? [o.material] : []) materials.add(m);
        });
        for (const m of materials) for (const value of Object.values(m)) if (value && value.isTexture) textures.add(value);
        for (const g of geometries) g.dispose();
        for (const m of materials) m.dispose();
        for (const t of textures) { t.dispose(); if (t.image && typeof t.image.close === 'function') t.image.close(); }
    }

    AD.createPetDecor = function (T, scene, track, quality, ground) {
        if (!T.cloneSkeleton || !T.GLTFLoader || !AD.assets || !AD.assets.fetchBytes || !AD.assets.fetchJson) return null;
        const placements = AD.petDecorPlacements(track), group = new T.Group();
        group.name = 'pet-decor'; scene.add(group);
        const templates = [], actors = [];
        const groundRay = new T.Raycaster(new T.Vector3(), new T.Vector3(0, -1, 0), 0, 20000);
        let disposed = false, failed = 0, loaded = 0, visible = 0, animated = 0, clock = 0, ticks = 0, glq = quality;
        const folders = [...new Set(placements.map(p => p.folder))];
        const ready = Promise.allSettled(folders.map(async folder => {
            let template = null;
            try {
                const [bytes, pack] = await Promise.all([
                    AD.assets.fetchBytes('/3d/pets/' + folder + '/model.glb'),
                    AD.assets.fetchJson('/3d/aura-drift/pets/' + folder + '.json')
                ]);
                if (disposed) return;
                if (pack.version !== 1 || pack.folder !== folder || !Array.isArray(pack.clips) || pack.clips.length !== 3) throw new Error('Invalid mascot motion pack');
                const gltf = await new T.GLTFLoader().parseAsync(bytes, '');
                template = gltf.scene;
                if (disposed) { disposeTemplate(template); return; }
                template.updateMatrixWorld(true);
                const box = new T.Box3().setFromObject(template), size = box.getSize(new T.Vector3()), center = box.getCenter(new T.Vector3());
                if (!(size.y > .1 && size.y < 10)) throw new Error('Invalid mascot height');
                const clips = pack.clips.map(item => T.AnimationClip.parse(item.clip));
                if (clips.some((clip, i) => pack.clips[i].id !== PETS[folder].clips[i] || !Number.isFinite(clip.duration) || clip.duration <= 0)) throw new Error('Invalid mascot clips');
                templates.push(template);
                for (const placement of placements.filter(p => p.folder === folder)) {
                    const model = T.cloneSkeleton(template), holder = new T.Group(), scale = placement.h / size.y;
                    if (ground) {
                        // Coarse terrain triangles can stand above the analytic height near a slope.
                        ground.updateWorldMatrix(true, true);
                        groundRay.ray.origin.set(placement.x, 10000, placement.y);
                        const hit = groundRay.intersectObject(ground, true)[0];
                        if (hit) placement.z = hit.point.y + .02;
                    }
                    model.position.add(new T.Vector3(-center.x, -box.min.y, -center.z));
                    model.traverse(o => { if (o.isMesh) { o.castShadow = false; o.receiveShadow = true; o.frustumCulled = false; } });
                    holder.name = 'mascot-' + folder; holder.position.set(placement.x, placement.z, placement.y);
                    holder.rotation.y = placement.yaw; holder.scale.setScalar(scale); holder.add(model); holder.visible = false; group.add(holder);
                    const mixer = new T.AnimationMixer(model), actions = clips.map(clip => mixer.clipAction(clip));
                    actions[0].play(); mixer.update(.7);
                    actors.push({ holder, model, mixer, actions, current: 0, placement, phase: actors.length * 1.7 });
                }
                loaded++;
            } catch (_) { failed++; if (template && !templates.includes(template)) disposeTemplate(template); }
        }));

        return {
            ready,
            update(pose, race, dt, reduced) {
                if (disposed) return;
                const advance = !reduced && dt > 0;
                if (advance) { clock += Math.min(dt, .05); ticks++; }
                // shortcut: shared 50k-triangle Desktop LODs; add crowd LODs if more than two nearby mascots are needed.
                const limit = glq && glq.triangles <= 250000 ? 1 : 2;
                const target = pose.target || pose.eye, fx = target[0] - pose.eye[0], fy = target[1] - pose.eye[1], fz = target[2] - pose.eye[2], length = Math.hypot(fx, fy, fz);
                const ranked = actors.map(a => {
                    const dx = a.placement.x - pose.eye[0], dy = a.placement.z + a.placement.h * .5 - pose.eye[1], dz = a.placement.y - pose.eye[2];
                    return { a, distance: Math.hypot(dx, dz), front: !length || (dx * fx + dy * fy + dz * fz) / length > -a.placement.r };
                }).filter(x => x.distance < 105 && x.front).sort((a, b) => a.distance - b.distance).slice(0, limit);
                const selected = new Set(ranked.map(x => x.a));
                visible = ranked.length; animated = advance ? ranked.length : 0;
                for (const actor of actors) {
                    actor.holder.visible = selected.has(actor);
                    if (!actor.holder.visible || !advance) continue;
                    const car = race && race.cars && race.cars[0] && race.cars[0].v;
                    const cheering = car && Math.hypot(car.x - actor.placement.x, car.y - actor.placement.y) < 55;
                    const next = cheering ? 1 + Math.floor((clock + actor.phase) / 5) % 2 : 0;
                    if (actor.current !== next) {
                        actor.actions[actor.current].fadeOut(.4);
                        actor.actions[next].reset().fadeIn(.4).play(); actor.current = next;
                    }
                    actor.mixer.update(Math.min(dt, .05));
                }
            },
            setQuality(next) { glq = next; },
            stats() { return { loaded, failed, total: actors.length, visible, animated, ticks, placements: placements.map(p => ({ folder: p.folder, x: p.x, y: p.y, z: p.z, s: p.s })) }; },
            dispose() {
                if (disposed) return;
                disposed = true;
                for (const a of actors) {
                    a.mixer.stopAllAction(); a.mixer.uncacheRoot(a.model);
                    a.model.traverse(o => { if (o.isSkinnedMesh) o.skeleton.dispose(); });
                }
                for (const template of templates) disposeTemplate(template);
                actors.length = templates.length = visible = animated = 0; group.removeFromParent(); group.clear();
            }
        };
    };
})();
