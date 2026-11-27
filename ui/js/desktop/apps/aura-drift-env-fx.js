/* Aura Drift - 3D environmental spectacle, optical highlights and instanced ground contact shadows.
 *
 * AD.createEnvFx(T, scene, track, look, q, placements) -> {
 *   update(dt, pose, race, wetness), rebuild(placements), setQuality(q), setReduced(on), stats(), dispose()
 * }.
 * Purely visual; zero external assets, zero ad-filter words, zero GLSL hash noise.
 * Every mesh sets transparent:true, depthWrite:false so GTAO normal pass ignores it automatically. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const WATER_Y = typeof AD.HARBOUR_WATER_Y === 'number' ? AD.HARBOUR_WATER_Y : -2.2;
    const SHADOW_CAPS = { low: 140, medium: 300, high: 560, ultra: 720 };
    const NO_SHADOW = new Set(['gantry', 'inflatable_arch', 'road_sign', 'windsock', 'tugboat', 'fishing_boat', 'buoy_cluster', 'flower_meadow', 'fern_glade', 'geyser_pool']);
    const BEAM_VERT = [
        'varying vec2 vUv; varying vec3 vNrm; varying vec3 vView;',
        'void main() {',
        '  vUv = uv;',
        '  vec4 mv = modelViewMatrix * vec4(position, 1.0);',
        '  vNrm = normalize(normalMatrix * normal);',
        '  vView = normalize(-mv.xyz);',
        '  gl_Position = projectionMatrix * mv;',
        '}'
    ].join('\n');
    const BEAM_FRAG = [
        'uniform vec3 uColor; uniform float uAlpha, uEdgePow;',
        'varying vec2 vUv; varying vec3 vNrm; varying vec3 vView;',
        'void main() {',
        '  float edge = pow(abs(dot(normalize(vNrm), normalize(vView))), uEdgePow);',
        '  float endFade = smoothstep(0.0, 0.24, vUv.y) * smoothstep(1.0, 0.14, vUv.y);',
        '  gl_FragColor = vec4(uColor, edge * endFade * uAlpha);',
        '}'
    ].join('\n');

    function tierOf(q) {
        for (const k in AD.QUALITY) if (AD.QUALITY[k] && AD.QUALITY[k].gl === q) return k;
        return 'high';
    }
    function seeded(seed) {
        let s = (seed >>> 0) || 1;
        return () => { s = (Math.imul(s, 1664525) + 1013904223) >>> 0; return s / 4294967296; };
    }

    function radialTex(T, size, stops) {
        const c = document.createElement('canvas'); c.width = c.height = size;
        const g = c.getContext('2d'), h = size / 2;
        const gr = g.createRadialGradient(h, h, 0, h, h, h);
        for (const [stop, col] of stops) gr.addColorStop(stop, col);
        g.fillStyle = gr; g.fillRect(0, 0, size, size);
        const t = new T.CanvasTexture(c);
        t.colorSpace = T.SRGBColorSpace;
        return t;
    }

    function wakeTex(T) {
        const size = 96, c = document.createElement('canvas'); c.width = c.height = size;
        const g = c.getContext('2d'), h = size / 2;
        for (const [r, w, a] of [[h * 0.82, 3.2, 0.55], [h * 0.54, 2.2, 0.36]]) {
            g.strokeStyle = 'rgba(205,244,255,' + a + ')';
            g.lineWidth = w;
            g.beginPath(); g.arc(h, h, r, 0, Math.PI * 2); g.stroke();
        }
        const t = new T.CanvasTexture(c);
        t.colorSpace = T.SRGBColorSpace;
        return t;
    }

    function smokeTex(T) {
        const size = 128, c = document.createElement('canvas'); c.width = c.height = size;
        const g = c.getContext('2d'), rnd = seeded(7711);
        for (let i = 0; i < 14; i++) {
            const x = size * (0.3 + rnd() * 0.4), y = size * (0.3 + rnd() * 0.4), r = size * (0.16 + rnd() * 0.16);
            const gr = g.createRadialGradient(x, y, 0, x, y, r);
            gr.addColorStop(0, 'rgba(255,255,255,0.22)');
            gr.addColorStop(0.55, 'rgba(245,248,255,0.10)');
            gr.addColorStop(1, 'rgba(255,255,255,0)');
            g.fillStyle = gr; g.fillRect(0, 0, size, size);
        }
        const t = new T.CanvasTexture(c);
        t.colorSpace = T.SRGBColorSpace;
        return t;
    }

    function catenaryPoints(T, a, b, sag, steps) {
        const pts = [];
        for (let i = 0; i <= steps; i++) {
            const u = i / steps, p = new T.Vector3().lerpVectors(a, b, u);
            p.y -= sag * 4 * u * (1 - u);
            pts.push(p);
        }
        return pts;
    }

    AD.createEnvFx = function (T, scene, track, look, q, initialPlacements) {
        const biomeId = (track && track.biomeId) || 'alpine';
        const night = !!(look && look.night);
        let glq = q || (AD.QUALITY && AD.QUALITY.high.gl);
        let tier = tierOf(glq);
        let reduced = !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
        let placements = initialPlacements || [];
        const root = new T.Group();
        root.name = 'env-fx';
        scene.add(root);

        const own = [], mats = [], geos = [];
        const ownTex = t => { own.push(t); return t; };
        const ownMat = m => { mats.push(m); return m; };
        const ownGeo = g => { geos.push(g); return g; };

        const shadowMap = ownTex(radialTex(T, 64, [[0, 'rgba(6,8,14,0.75)'], [0.45, 'rgba(8,10,16,0.42)'], [1, 'rgba(8,10,16,0)']]));
        const glowMap = ownTex(radialTex(T, 64, [[0, 'rgba(255,255,255,1)'], [0.28, 'rgba(255,255,255,0.55)'], [0.65, 'rgba(255,255,255,0.14)'], [1, 'rgba(255,255,255,0)']]));
        const smokeMap = ownTex(smokeTex(T));
        const wakeMap = ownTex(wakeTex(T));

        function mkBeamMat(color, alpha, edgePow) {
            return ownMat(new T.ShaderMaterial({
                uniforms: { uColor: { value: new T.Color(color) }, uAlpha: { value: alpha }, uEdgePow: { value: edgePow || 1.6 } },
                vertexShader: BEAM_VERT, fragmentShader: BEAM_FRAG,
                transparent: true, depthWrite: false, blending: T.AdditiveBlending, side: T.DoubleSide
            }));
        }

        const UP = new T.Vector3(0, 1, 0), tmpV = new T.Vector3(), tmpQ = new T.Quaternion(), tmpS = new T.Vector3(), tmpM = new T.Matrix4(), tmpC = new T.Color();
        let shadowMesh = null, shadowList = [], lastShadowView = null;
        let beacons = null, plumes = null, wakes = null, sparks = null, lampPools = null, lampCones = null;
        const rotators = [], gondolas = [], skyBeams = [], ringWaves = [];
        let fxCount = 0;

        function clearFx() {
            for (const c of root.children.slice()) root.remove(c);
            for (const m of mats) m.dispose();
            for (const g of geos) g.dispose();
            mats.length = 0; geos.length = 0;
            shadowMesh = null; shadowList = []; lastShadowView = null;
            beacons = plumes = wakes = sparks = lampPools = lampCones = null;
            rotators.length = 0; gondolas.length = 0; skyBeams.length = 0; ringWaves.length = 0;
            fxCount = 0;
        }

        function propY(p) { return (p.gy || 0) + (p.footSink || 0); }

        function buildShadows() {
            const cap = SHADOW_CAPS[tier] || 480;
            shadowList = placements.filter(p => !NO_SHADOW.has(p.name) && !(p.back && p.dist > 340) && (p.r || 0) >= 0.9);
            if (!shadowList.length) return;
            const geo = ownGeo(new T.PlaneGeometry(2, 2).rotateX(-Math.PI / 2));
            const mat = ownMat(new T.MeshBasicMaterial({
                map: shadowMap, transparent: true, opacity: night ? 0.34 : 0.46,
                depthWrite: false, polygonOffset: true, polygonOffsetFactor: -1, polygonOffsetUnits: -1
            }));
            shadowMesh = new T.InstancedMesh(geo, mat, Math.min(cap, shadowList.length));
            shadowMesh.count = 0; shadowMesh.frustumCulled = false; shadowMesh.renderOrder = -1;
            shadowMesh.instanceMatrix.setUsage(T.DynamicDrawUsage);
            root.add(shadowMesh);
        }

        function updateShadows(eye, dir) {
            if (!shadowMesh || !shadowList.length) return;
            if (lastShadowView && Math.hypot(eye[0] - lastShadowView.x, eye[2] - lastShadowView.y) < 12) return;
            lastShadowView = { x: eye[0], y: eye[2] };
            const cap = shadowMesh.instanceMatrix.count, hits = [];
            for (let i = 0; i < shadowList.length; i++) {
                const p = shadowList[i], dx = p.x - eye[0], dz = p.y - eye[2], d = Math.hypot(dx, dz);
                if (d > 260) continue;
                if (d > 55 && (dx * dir[0] + dz * dir[1]) / d < 0.2) continue;
                hits.push([d, p]);
            }
            hits.sort((a, b) => a[0] - b[0]);
            const n = Math.min(cap, hits.length);
            for (let i = 0; i < n; i++) {
                const p = hits[i][1], r = clamp((p.r || 2.2) * 1.15, 1.3, 15.5);
                tmpV.set(p.x, propY(p) + 0.05, p.y);
                tmpQ.setFromAxisAngle(UP, p.yaw || 0);
                tmpS.set(r, 1, r * (p.name === 'camper_van' || p.name === 'container_stack' ? 1.3 : 1));
                tmpM.compose(tmpV, tmpQ, tmpS);
                shadowMesh.setMatrixAt(i, tmpM);
            }
            shadowMesh.count = n;
            shadowMesh.instanceMatrix.needsUpdate = true;
            shadowMesh.visible = n > 0;
        }

        function addSunShafts(color, count, alpha) {
            if (tier === 'low' || !track || !track.length) return;
            const sd = AD.sunDirection(look, 18);
            const sunV = new T.Vector3(sd[0], sd[1], sd[2]).normalize();
            const n = tier === 'medium' ? Math.max(3, count >> 1) : count;
            const shaftGeo = ownGeo(new T.CylinderGeometry(14, 42, 150, 14, 1, true));
            const mat = mkBeamMat(color, alpha * 0.58, 2.1);
            const qAlign = new T.Quaternion().setFromUnitVectors(UP, sunV);
            for (let i = 0; i < n; i++) {
                const s = track.length * ((i + 0.35) / n), p = AD.pointAt(track, s);
                const side = i % 2 ? 1 : -1, lat = side * (p.halfWidth + 32 + (i % 3) * 20);
                const x = p.x + p.nx * lat, z = p.y + p.ny * lat;
                const y0 = AD.groundHeight ? AD.groundHeight(track, biomeId, x, z) : 0;
                const m = new T.Mesh(shaftGeo, mat);
                m.position.set(x, y0 + 18, z).addScaledVector(sunV, 26);
                m.quaternion.copy(qAlign);
                m.renderOrder = 14; m.frustumCulled = false;
                root.add(m); fxCount++;
            }
        }

        function mkBeaconPool(list) {
            if (!list.length) return;
            const geo = ownGeo(new T.PlaneGeometry(1, 1));
            const mat = ownMat(new T.MeshBasicMaterial({ map: glowMap, color: 0xffffff, transparent: true, depthWrite: false, blending: T.AdditiveBlending }));
            const im = new T.InstancedMesh(geo, mat, list.length);
            im.frustumCulled = false; im.renderOrder = 18;
            im.instanceMatrix.setUsage(T.DynamicDrawUsage);
            root.add(im);
            beacons = { mesh: im, list: list };
            fxCount += list.length;
        }

        function mkPlumePool(emitters, perEmit, opts) {
            if (!emitters.length || tier === 'low') return;
            const nEach = tier === 'medium' ? Math.max(3, perEmit >> 1) : perEmit;
            const items = [];
            const rnd = seeded(AD.hashString('plume/' + biomeId));
            for (const e of emitters) for (let i = 0; i < nEach; i++) items.push({ x: e.x, y: e.y, z: e.z, ph: (i / nEach + rnd() * 0.15) % 1, drift: (rnd() - 0.5) * 2.5, r0: e.r0 || opts.r0 });
            const geo = ownGeo(new T.PlaneGeometry(1, 1));
            const mat = ownMat(new T.MeshBasicMaterial({ map: smokeMap, color: opts.color, transparent: true, opacity: opts.opacity, depthWrite: false, blending: opts.add ? T.AdditiveBlending : T.NormalBlending }));
            const im = new T.InstancedMesh(geo, mat, items.length);
            im.frustumCulled = false; im.renderOrder = 15;
            im.instanceMatrix.setUsage(T.DynamicDrawUsage);
            root.add(im);
            plumes = { mesh: im, items: items, opts: opts };
            fxCount += items.length;
        }

        function mkSparkPool(points, color, count, radius, speed) {
            if (!points.length || tier === 'low') return;
            const n = tier === 'medium' ? Math.max(12, count >> 1) : count;
            const rnd = seeded(AD.hashString('spark/' + biomeId)), items = [];
            for (let i = 0; i < n; i++) {
                const src = points[i % points.length];
                items.push({ x: src.x + (rnd() - 0.5) * radius, y: src.y + 0.8 + rnd() * 5.2, z: src.z + (rnd() - 0.5) * radius, ph: rnd() * 6.28, sp: speed * (0.7 + rnd() * 0.6), sz: 0.45 + rnd() * 0.55 });
            }
            const geo = ownGeo(new T.PlaneGeometry(1, 1));
            const mat = ownMat(new T.MeshBasicMaterial({ map: glowMap, color: color, transparent: true, opacity: 0.9, depthWrite: false, blending: T.AdditiveBlending }));
            const im = new T.InstancedMesh(geo, mat, items.length);
            im.frustumCulled = false; im.renderOrder = 17;
            im.instanceMatrix.setUsage(T.DynamicDrawUsage);
            root.add(im);
            sparks = { mesh: im, items: items };
            fxCount += items.length;
        }

        function buildChains(pName, hOff, sag, cableColor, withGondolas) {
            const chains = new Map();
            for (const p of placements) {
                if (p.name !== pName || p.chain === undefined) continue;
                (chains.get(p.chain) || chains.set(p.chain, []).get(p.chain)).push(p);
            }
            const cableMat = ownMat(new T.LineBasicMaterial({ color: cableColor, transparent: true, opacity: 0.72, depthWrite: false }));
            const bodyGeo = withGondolas ? ownGeo(new T.BoxGeometry(2.3, 2.1, 3.2)) : null;
            const glassGeo = withGondolas ? ownGeo(new T.BoxGeometry(2.45, 0.85, 2.6)) : null;
            const hangerGeo = withGondolas ? ownGeo(new T.CylinderGeometry(0.08, 0.08, 2.2, 6)) : null;
            const bodyMat = withGondolas ? ownMat(new T.MeshStandardMaterial({ color: 0xd9362b, roughness: 0.4, metalness: 0.2 })) : null;
            const glassMat = withGondolas ? ownMat(new T.MeshStandardMaterial({ color: 0xffe49e, emissive: 0xffb84a, emissiveIntensity: 0.6, roughness: 0.2 })) : null;
            const steelMat = withGondolas ? ownMat(new T.MeshStandardMaterial({ color: 0x454d58, metalness: 0.8, roughness: 0.35 })) : null;

            for (const list of chains.values()) {
                list.sort((a, b) => a.chainIdx - b.chainIdx);
                if (list.length < 2) continue;
                for (const offset of (withGondolas ? [-1.6, 1.6] : [-2.8, 0, 2.8])) {
                    const linePts = [];
                    for (let i = 0; i < list.length - 1; i++) {
                        const a = list[i], b = list[i + 1];
                        const dx = b.x - a.x, dz = b.y - a.y, l = Math.hypot(dx, dz) || 1, nx = -dz / l * offset, nz = dx / l * offset;
                        const pa = new T.Vector3(a.x + nx, propY(a) + hOff, a.y + nz), pb = new T.Vector3(b.x + nx, propY(b) + hOff, b.y + nz);
                        const segs = catenaryPoints(T, pa, pb, sag, 14);
                        for (let j = 0; j < segs.length - 1; j++) linePts.push(segs[j], segs[j + 1]);
                    }
                    const lgeo = ownGeo(new T.BufferGeometry().setFromPoints(linePts));
                    const line = new T.LineSegments(lgeo, cableMat);
                    line.frustumCulled = false; root.add(line); fxCount++;
                }
                if (withGondolas && list.length >= 2) {
                    const a = list[0], b = list[list.length - 1];
                    const dx = b.x - a.x, dz = b.y - a.y, l = Math.hypot(dx, dz) || 1, yaw = Math.atan2(dx, dz);
                    for (let g = 0; g < 2; g++) {
                        const grp = new T.Group();
                        const cab = new T.Mesh(bodyGeo, bodyMat), win = new T.Mesh(glassGeo, glassMat), bar = new T.Mesh(hangerGeo, steelMat);
                        win.position.y = 0.25; bar.position.y = 1.9; grp.add(cab, win, bar); grp.rotation.y = yaw;
                        root.add(grp);
                        gondolas.push({ grp, ax: a.x, ay: propY(a) + hOff - 3.0, az: a.y, bx: b.x, by: propY(b) + hOff - 3.0, bz: b.y, nx: -dz / l * (g === 0 ? -1.6 : 1.6), nz: dx / l * (g === 0 ? -1.6 : 1.6), phase: g * 0.5, sag: sag });
                        fxCount++;
                    }
                }
            }
        }

        function buildBiomeFx() {
            const beaconList = [], sparkPts = [];
            if (biomeId === 'alpine') {
                addSunShafts('#ffd594', 7, 0.085);
                buildChains('cablecar_pylon', 19.5, 4.2, '#4a525e', true);
            } else if (biomeId === 'desert') {
                addSunShafts('#ffb970', 7, 0.09);
                const dustPts = placements.filter(p => p.name === 'mesa_rock' || p.name === 'palm_oasis' || p.name === 'sunshade').slice(0, 10).map(p => ({ x: p.x, y: propY(p) + 0.5, z: p.y, r0: 4.5 }));
                mkPlumePool(dustPts, 5, { color: '#dfb27a', opacity: 0.22, r0: 4.5, rise: 24, spread: 11, add: false, speed: 0.14 });
            } else if (biomeId === 'harbour') {
                const beamGeo = ownGeo(new T.CylinderGeometry(1.0, 22, 320, 14, 1, true).translate(0, -160, 0).rotateX(Math.PI / 2));
                const beamMat = mkBeamMat('#ffe4a8', 0.20, 1.7);
                for (const p of placements) {
                    const yBase = (p.gy || 0) + (p.groundOffset || 0);
                    if (p.name === 'lighthouse') {
                        const rig = new T.Group(), ly = (p.gy || 0) + 21.5 * p.sy;
                        rig.position.set(p.x, ly, p.y);
                        const b1 = new T.Mesh(beamGeo, beamMat), b2 = new T.Mesh(beamGeo, beamMat);
                        b1.rotation.x = 0.09; b2.rotation.y = Math.PI; b2.rotation.x = -0.09;
                        rig.add(b1, b2); rig.rotation.y = 0.6;
                        root.add(rig); rotators.push(rig);
                        beaconList.push({ x: p.x, y: ly, z: p.y, col: '#ffdf8e', sz: 12, speed: 1.8, ph: p.x * 0.1 });
                    } else if (p.name === 'crane') beaconList.push({ x: p.x, y: yBase + 22 * p.sy, z: p.y, col: '#ff5278', sz: 4.5, speed: 2.6, ph: p.x });
                    else if (p.name === 'signal_mast') beaconList.push({ x: p.x, y: yBase + 9.5 * p.sy, z: p.y, col: '#48f2ff', sz: 3.4, speed: 3.2, ph: p.y });
                    else if (p.name === 'storm_lantern') beaconList.push({ x: p.x, y: yBase + 5.2 * p.sy, z: p.y, col: '#ffcf7a', sz: 3.8, speed: 0.8, ph: p.x });
                }
                const boatList = placements.filter(p => p.name === 'tugboat' || p.name === 'fishing_boat' || p.name === 'buoy_cluster').slice(0, 64);
                if (boatList.length) {
                    const wgeo = ownGeo(new T.PlaneGeometry(2, 2).rotateX(-Math.PI / 2));
                    const wmat = ownMat(new T.MeshBasicMaterial({ map: wakeMap, transparent: true, opacity: 0.42, depthWrite: false }));
                    wakes = { mesh: new T.InstancedMesh(wgeo, wmat, boatList.length), list: boatList };
                    wakes.mesh.frustumCulled = false; wakes.mesh.renderOrder = -2; root.add(wakes.mesh);
                    fxCount += boatList.length;
                }
            } else if (biomeId === 'salt') {
                let sp = null;
                try { if (typeof AD.createFeatures === 'function') sp = AD.createFeatures(track, track.seed, { crates: false }).setpieces?.[0]; } catch (_) { sp = null; }
                if (sp) {
                    const beam = new T.Mesh(ownGeo(new T.CylinderGeometry(4.5, 21, 56, 20, 1, true)), mkBeamMat('#5effb6', 0.17, 1.5));
                    beam.position.set(sp.x, 28, sp.y); beam.renderOrder = 16; beam.frustumCulled = false; root.add(beam); fxCount++;
                    const rGeo = ownGeo(new T.TorusGeometry(1, 0.08, 8, 32).rotateX(Math.PI / 2));
                    const rMat = ownMat(new T.MeshBasicMaterial({ color: 0x7effc8, transparent: true, opacity: 0.52, depthWrite: false, blending: T.AdditiveBlending }));
                    for (let i = 0; i < 3; i++) {
                        const rm = new T.Mesh(rGeo, rMat);
                        rm.position.set(sp.x, 6 + i * 16, sp.y); rm.frustumCulled = false; root.add(rm);
                        ringWaves.push({ mesh: rm, x: sp.x, z: sp.y, ph: i / 3 });
                    }
                }
                const geysers = placements.filter(p => p.name === 'geyser_pool').map(p => ({ x: p.x, y: propY(p) + 0.4, z: p.y, r0: 2.2 * p.sy }));
                mkPlumePool(geysers, 7, { color: '#f2fafc', opacity: 0.36, r0: 2.4, rise: 22, spread: 3.5, add: false, speed: 0.32 });
                for (const p of placements) if (p.name === 'salt_crystal' && sparkPts.length < 40) sparkPts.push({ x: p.x, y: propY(p) + 1.4, z: p.y });
                mkSparkPool(sparkPts, '#ffffff', 42, 4, 2.2);
            } else if (biomeId === 'forest') {
                addSunShafts('#e8ffad', 9, 0.095);
                for (const p of placements) if ((p.name === 'fern_glade' || p.name === 'mushroom_cluster' || p.name === 'watermill') && sparkPts.length < 60) sparkPts.push({ x: p.x, y: propY(p) + 0.6, z: p.y });
                mkSparkPool(sparkPts, '#d8ff66', 72, 5.5, 1.5);
            } else if (biomeId === 'industry') {
                buildChains('pylon', 24.5, 3.8, '#8a95a5', false);
                const steam = [];
                for (const p of placements) {
                    const y0 = (p.gy || 0) + (p.groundOffset || 0);
                    if (p.name === 'cooling_tower') {
                        steam.push({ x: p.x, y: y0 + 21 * p.sy, z: p.y, r0: 5.5 * p.sy });
                        beaconList.push({ x: p.x, y: y0 + 20 * p.sy, z: p.y, col: '#ff4228', sz: 4.5, speed: 2.8, ph: p.x });
                    } else if (p.name === 'smokestack') {
                        steam.push({ x: p.x, y: y0 + 24 * p.sy, z: p.y, r0: 2.8 * p.sy });
                        beaconList.push({ x: p.x, y: y0 + 25 * p.sy, z: p.y, col: '#ff5228', sz: 4.2, speed: 2.4, ph: p.y });
                    } else if (p.name === 'silo' && sparkPts.length < 24) {
                        sparkPts.push({ x: p.x, y: y0 + 1.5, z: p.y });
                    }
                }
                mkPlumePool(steam.slice(0, 22), 7, { color: '#ffd4b0', opacity: 0.28, r0: 4.5, rise: 38, spread: 8, add: false, speed: 0.2 });
                mkSparkPool(sparkPts, '#ff7d2a', 36, 7, 1.8);
            } else if (biomeId === 'glacier') {
                const camps = placements.filter(p => p.name === 'research_station' || p.name === 'igloo' || p.name === 'snow_cat').slice(0, 14);
                if (camps.length) {
                    const coneGeo = ownGeo(new T.CylinderGeometry(0.8, 7.5, 10, 12, 1, true));
                    const coneMat = mkBeamMat('#ffcf84', 0.16, 1.5);
                    const poolGeo = ownGeo(new T.PlaneGeometry(16, 16).rotateX(-Math.PI / 2));
                    const poolMat = ownMat(new T.MeshBasicMaterial({ map: glowMap, color: 0xffc670, transparent: true, opacity: 0.28, depthWrite: false, blending: T.AdditiveBlending }));
                    for (const p of camps) {
                        const y0 = propY(p);
                        const cone = new T.Mesh(coneGeo, coneMat); cone.position.set(p.x, y0 + 5, p.y); cone.renderOrder = 15; root.add(cone);
                        const pool = new T.Mesh(poolGeo, poolMat); pool.position.set(p.x, y0 + 0.08, p.y); pool.renderOrder = 2; root.add(pool);
                        fxCount += 2;
                    }
                }
                const spinPts = placements.filter(p => p.name === 'ice_spire' || p.name === 'ice_arch').slice(0, 16).map(p => ({ x: p.x, y: propY(p) + 1.2, z: p.y, r0: 4.8 }));
                mkPlumePool(spinPts, 4, { color: '#b8e4ff', opacity: 0.24, r0: 4.8, rise: 10, spread: 14, add: true, speed: 0.18 });
                for (const p of placements) if (p.name === 'ice_spire' && sparkPts.length < 40) sparkPts.push({ x: p.x, y: propY(p) + 3.0, z: p.y });
                mkSparkPool(sparkPts, '#9ef4ff', 48, 5, 2.0);
            } else if (biomeId === 'metro') {
                const lamps = placements.filter(p => p.name === 'street_lamp');
                if (lamps.length) {
                    const cGeo = ownGeo(new T.CylinderGeometry(0.28, 4.4, 7.8, 10, 1, true));
                    const cMat = mkBeamMat('#7cf4ff', 0.13, 1.5);
                    lampCones = { mesh: new T.InstancedMesh(cGeo, cMat, lamps.length), list: lamps };
                    lampCones.mesh.frustumCulled = false; lampCones.mesh.renderOrder = 14;
                    const pGeo = ownGeo(new T.PlaneGeometry(11, 11).rotateX(-Math.PI / 2));
                    const pMat = ownMat(new T.MeshBasicMaterial({ map: glowMap, color: 0x5ae8ff, transparent: true, opacity: 0.24, depthWrite: false, blending: T.AdditiveBlending }));
                    lampPools = { mesh: new T.InstancedMesh(pGeo, pMat, lamps.length), list: lamps };
                    lampPools.mesh.frustumCulled = false; lampPools.mesh.renderOrder = 2;
                    lamps.forEach((p, i) => {
                        const y0 = propY(p);
                        tmpV.set(p.x, y0 + 3.9, p.y); tmpQ.identity(); tmpS.set(1, 1, 1);
                        tmpM.compose(tmpV, tmpQ, tmpS); lampCones.mesh.setMatrixAt(i, tmpM);
                        tmpV.set(p.x, y0 + 0.06, p.y);
                        tmpM.compose(tmpV, tmpQ, tmpS); lampPools.mesh.setMatrixAt(i, tmpM);
                    });
                    lampCones.mesh.instanceMatrix.needsUpdate = true; lampPools.mesh.instanceMatrix.needsUpdate = true;
                    root.add(lampCones.mesh, lampPools.mesh); fxCount += lamps.length * 2;
                }
                const roofs = placements.filter(p => p.name === 'skyscraper_c').slice(0, 6);
                if (roofs.length) {
                    const sGeo = ownGeo(new T.CylinderGeometry(14, 1.4, 190, 12, 1, true).translate(0, 95, 0));
                    roofs.forEach((p, i) => {
                        const col = i % 2 ? '#ff3cc8' : '#3eeeff';
                        const m = new T.Mesh(sGeo, mkBeamMat(col, 0.14, 1.6));
                        m.position.set(p.x, propY(p) + (p.h || 42) * 0.92, p.y);
                        m.frustumCulled = false; m.renderOrder = 15; root.add(m);
                        skyBeams.push({ mesh: m, baseTilt: 0.28 + (i % 3) * 0.08, ph: i * 1.3 });
                        fxCount++;
                    });
                }
                for (const p of placements) {
                    if (p.name === 'metro_entrance' || p.name === 'arcade_kiosk' || p.name === 'holo_arrow') {
                        beaconList.push({ x: p.x, y: propY(p) + 4.4, z: p.y, col: p.name === 'metro_entrance' ? '#ff58c6' : '#44f2ff', sz: 6.2, speed: 1.9, ph: p.x });
                    }
                }
            }
            mkBeaconPool(beaconList);
        }

        function buildAll() {
            clearFx();
            buildShadows();
            buildBiomeFx();
        }

        let clock = 0;
        buildAll();

        const api = {
            rebuild(nextPlacements) {
                if (nextPlacements) placements = nextPlacements;
                buildAll();
            },
            setQuality(next) {
                if (!next) return;
                const nt = tierOf(next);
                glq = next;
                if (nt !== tier) { tier = nt; buildAll(); }
            },
            setReduced(on) { reduced = !!on; },
            stats() { return { fx: fxCount, shadows: shadowMesh ? shadowMesh.count : 0 }; },
            update(dt, pose) {
                if (!pose || !pose.eye) return;
                if (!reduced) clock += clamp(dt || 0, 0, 0.1);
                const eye = pose.eye, tg = pose.target || eye;
                const dx = tg[0] - eye[0], dz = tg[2] - eye[2], dl = Math.hypot(dx, dz) || 1;
                updateShadows(eye, [dx / dl, dz / dl]);

                const t = reduced ? 1.2 : clock;
                for (let i = 0; i < rotators.length; i++) rotators[i].rotation.y = 0.6 + t * 0.55 + i * 1.4;
                for (const g of gondolas) {
                    const u = 0.5 + 0.44 * Math.sin(t * 0.16 + g.phase * Math.PI * 2);
                    const ySag = g.sag * 4 * u * (1 - u);
                    g.grp.position.set(g.ax + (g.bx - g.ax) * u + g.nx, g.ay + (g.by - g.ay) * u - ySag, g.az + (g.bz - g.az) * u + g.nz);
                }
                for (const sb of skyBeams) sb.mesh.rotation.set(Math.cos(t * 0.45 + sb.ph) * sb.baseTilt, 0, Math.sin(t * 0.37 + sb.ph) * sb.baseTilt);
                for (const rw of ringWaves) {
                    const u = (t * 0.35 + rw.ph) % 1;
                    rw.mesh.position.set(rw.x, 6 + u * 46, rw.z);
                    const rad = 4.5 + u * 16; rw.mesh.scale.set(rad, 1, rad);
                }
                if (wakes) {
                    for (let i = 0; i < wakes.list.length; i++) {
                        const p = wakes.list[i], u = (t * 0.45 + i * 0.27) % 1, s = (p.r || 3) * (0.9 + 1.3 * u);
                        tmpV.set(p.x, WATER_Y + 0.05, p.y); tmpQ.identity(); tmpS.set(s, 1, s);
                        tmpM.compose(tmpV, tmpQ, tmpS); wakes.mesh.setMatrixAt(i, tmpM);
                    }
                    wakes.mesh.instanceMatrix.needsUpdate = true;
                }
                const camQ = new T.Quaternion().setFromRotationMatrix(tmpM.lookAt(new T.Vector3(eye[0], eye[1], eye[2]), new T.Vector3(tg[0], tg[1], tg[2]), UP));
                if (beacons) {
                    for (let i = 0; i < beacons.list.length; i++) {
                        const b = beacons.list[i], pulse = 0.45 + 0.55 * (0.5 + 0.5 * Math.sin(t * b.speed + b.ph));
                        const s = b.sz * pulse;
                        tmpV.set(b.x, b.y, b.z); tmpS.set(s, s, 1); tmpM.compose(tmpV, camQ, tmpS);
                        beacons.mesh.setMatrixAt(i, tmpM);
                        beacons.mesh.setColorAt(i, tmpC.set(b.col).multiplyScalar(pulse));
                    }
                    beacons.mesh.instanceMatrix.needsUpdate = true;
                    if (beacons.mesh.instanceColor) beacons.mesh.instanceColor.needsUpdate = true;
                }
                if (plumes) {
                    const o = plumes.opts;
                    for (let i = 0; i < plumes.items.length; i++) {
                        const it = plumes.items[i], u = (it.ph + t * o.speed) % 1;
                        const s = it.r0 * (1 + u * 2.4);
                        tmpV.set(it.x + it.drift * u * o.spread, it.y + u * o.rise, it.z + o.spread * 0.4 * u);
                        tmpS.set(s, s * 0.9, 1); tmpM.compose(tmpV, camQ, tmpS);
                        plumes.mesh.setMatrixAt(i, tmpM);
                    }
                    plumes.mesh.instanceMatrix.needsUpdate = true;
                }
                if (sparks) {
                    for (let i = 0; i < sparks.items.length; i++) {
                        const it = sparks.items[i], pulse = 0.35 + 0.65 * Math.abs(Math.sin(t * it.sp + it.ph));
                        const s = it.sz * pulse;
                        tmpV.set(it.x + Math.sin(t * 0.7 + it.ph) * 0.8, it.y + Math.sin(t * 1.1 + it.ph) * 0.6, it.z + Math.cos(t * 0.6 + it.ph) * 0.8);
                        tmpS.set(s, s, 1); tmpM.compose(tmpV, camQ, tmpS);
                        sparks.mesh.setMatrixAt(i, tmpM);
                    }
                    sparks.mesh.instanceMatrix.needsUpdate = true;
                }
            },
            dispose() {
                clearFx();
                for (const t of own) t.dispose();
                own.length = 0;
                root.removeFromParent();
            }
        };
        return api;
    };
})();
