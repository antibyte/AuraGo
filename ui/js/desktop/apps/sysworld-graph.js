(function () {
    'use strict';

    // Knowledge-graph constellation for the system-world desktop app.
    // Renders the AuraGo knowledge graph as a slowly rotating 3D constellation
    // in its own region of space (LAYOUT.graphCenter). The layout is a simple
    // synchronous force-directed placement that runs once per build/expand,
    // never per frame. Augments the shared window.SysWorld namespace.

    const NS = window.SysWorld = window.SysWorld || {};

    // Fallback hues for node types, used only when NS.PALETTE is not loaded.
    const FALLBACK_TYPE_HUES = [0x4fc3f7, 0x81c784, 0xffb74d, 0xba68c8, 0x8ea3b0, 0xf06292, 0x4dd0e1, 0xff8a65];
    const MAX_NODES = 300;       // cap for the initial build, mirrors the API limit
    const EXPAND_HEADROOM = 64;  // extra nodes expand() may add on top of the cap
    const MAX_EDGES = 900;       // defensive cap for the merged line buffer
    const GLOW_TOP = 30;         // most-accessed nodes that receive a glow sprite
    const LAYOUT_ITERS = 120;    // synchronous force-layout iterations per build
    const ROT_SPEED = 0.018;     // constellation spin (rad/s)

    // FNV-1a style string hash -> uint32, used for deterministic seeding.
    function hashString(str) {
        let h = 2166136261 >>> 0;
        for (let i = 0; i < str.length; i++) {
            h ^= str.charCodeAt(i);
            h = Math.imul(h, 16777619);
        }
        return h >>> 0;
    }

    // mulberry32 PRNG: tiny seedable generator so rebuilds look stable.
    function mulberry32(seed) {
        let a = seed >>> 0;
        return function () {
            a |= 0; a = (a + 0x6D2B79F5) | 0;
            let t = Math.imul(a ^ (a >>> 15), 1 | a);
            t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    NS.createGraph = function (inst) {
        const THREE = inst.THREE || window.THREE;
        if (!THREE) { return null; }
        const fx = inst.fx || {};
        const P = NS.PALETTE || {};
        const LAY = NS.LAYOUT || {};
        const CENTER = LAY.graphCenter || new THREE.Vector3(0, 34, -120);
        const RADIUS = LAY.graphRadius || 55;

        // Fixed ~8-hue wheel; a node's properties.type hash picks one entry.
        const TYPE_HUES = [
            P.communication != null ? P.communication : FALLBACK_TYPE_HUES[0],
            P.smarthome != null ? P.smarthome : FALLBACK_TYPE_HUES[1],
            P.infrastructure != null ? P.infrastructure : FALLBACK_TYPE_HUES[2],
            P.ai != null ? P.ai : FALLBACK_TYPE_HUES[3],
            P.storage != null ? P.storage : FALLBACK_TYPE_HUES[4],
            P.monitoring != null ? P.monitoring : FALLBACK_TYPE_HUES[5],
            P.memory != null ? P.memory : FALLBACK_TYPE_HUES[6],
            P.journal != null ? P.journal : FALLBACK_TYPE_HUES[7]
        ];
        const EDGE_COLOR = P.memory != null ? P.memory : 0x4dd0e1;
        const GOLD = P.gold != null ? P.gold : 0xffd54f;

        // Root group: everything lives in graph-local coordinates around the
        // origin, the group itself sits at the constellation center.
        const group = new THREE.Group();
        group.name = 'sysworld-graph';
        group.position.copy(CENTER);
        if (inst.stage && inst.stage.scene) { inst.stage.scene.add(group); }

        // --- shared assets (created once, disposed in dispose()) ------------
        const nodeGeo = new THREE.SphereGeometry(1, 12, 10);
        const matCache = new Map(); // hex -> shared MeshBasicMaterial
        function matFor(hex) {
            let m = matCache.get(hex);
            if (!m) {
                m = new THREE.MeshBasicMaterial({ color: hex, transparent: true, opacity: 0.95 });
                matCache.set(hex, m);
            }
            return m;
        }
        // Translucent wireframe shell around every node; one shared
        // icosahedron-ish geometry plus a small per-hue material cache.
        const shellGeo = new THREE.IcosahedronGeometry(1, 1);
        const shellMatCache = new Map(); // hex -> shared wireframe material
        function shellMatFor(hex) {
            let m = shellMatCache.get(hex);
            if (!m) {
                m = new THREE.MeshBasicMaterial({
                    color: hex, wireframe: true, transparent: true, opacity: 0.2,
                    blending: THREE.AdditiveBlending, depthWrite: false
                });
                shellMatCache.set(hex, m);
            }
            return m;
        }
        // Thin gold orbit ring reserved for protected nodes; one shared
        // geometry and one shared material for all of them.
        const protRingGeo = new THREE.RingGeometry(1.6, 1.75, 48);
        const protRingMat = new THREE.MeshBasicMaterial({
            color: GOLD, transparent: true, opacity: 0.75, side: THREE.DoubleSide,
            blending: THREE.AdditiveBlending, depthWrite: false
        });
        const protRings = []; // ring meshes rotated slowly in update()
        const edgeMat = new THREE.LineBasicMaterial({
            color: EDGE_COLOR, transparent: true, opacity: 0.22,
            blending: THREE.AdditiveBlending, depthWrite: false
        });
        const EDGE_OPACITY_DEFAULT = 0.22;
        const EDGE_OPACITY_HIGHLIGHT = 0.4;

        // Soft constellation boundary shell so the district reads as its own
        // region of space.
        const shell = new THREE.Mesh(
            new THREE.IcosahedronGeometry(RADIUS + 7, 1),
            new THREE.MeshBasicMaterial({
                color: EDGE_COLOR, wireframe: true, transparent: true, opacity: 0.045,
                blending: THREE.AdditiveBlending, depthWrite: false
            })
        );
        shell.frustumCulled = false;
        group.add(shell);

        // --- per-constellation state ---------------------------------------
        const recs = [];              // node records in layout order
        const nodeById = new Map();   // id -> node record
        const edges = [];             // { a, b, relation } with record refs
        const edgeKeys = new Set();   // dedupe keys "idA|idB|relation"
        const pickables = [];         // stable array of node meshes for raycasting
        let edgeLines = null;         // single merged THREE.LineSegments
        let glowThreshold = Infinity; // min accessCount that earned a glow
        let visible = true;

        // Guarded call into the shared effects module (may be absent).
        function fxCall(name, args) {
            const f = fx[name];
            if (typeof f !== 'function') { return undefined; }
            try { return f.apply(fx, args); } catch (err) { return undefined; }
        }

        // Tolerant normalization of a raw API node ({ id, label, properties,
        // protected, access_count } - every field may be missing).
        function normalizeNode(raw) {
            if (!raw || raw.id == null) { return null; }
            const id = String(raw.id);
            const props = raw.properties || {};
            const type = props.type || 'other';
            return {
                id: id,
                label: raw.label || id,
                type: type,
                accessCount: Math.max(0, raw.access_count | 0),
                prot: !!raw.protected,
                hue: TYPE_HUES[hashString(type) % TYPE_HUES.length],
                x: 0, y: 0, z: 0,       // layout position (graph-local)
                fx: 0, fy: 0, fz: 0,    // per-iteration force accumulators
                mesh: null, glow: null, shell: null, ring: null,
                glowBaseScale: 0
            };
        }

        // Simple Fruchterman-Reingold style force layout, run synchronously
        // once. O(n^2) repulsion is acceptable at n <= 300.
        function runLayout(list, links) {
            const n = list.length;
            if (!n) { return; }
            // Ideal pairwise distance from the sphere volume per node.
            const k = Math.cbrt((4 / 3) * Math.PI * Math.pow(RADIUS * 0.8, 3) / n) * 0.9;
            const maxR = RADIUS * 0.95;
            for (let iter = 0; iter < LAYOUT_ITERS; iter++) {
                const temp = (1 - iter / LAYOUT_ITERS) * k * 0.5 + 0.05; // cooling
                let i, j, a, b, ox, oy, oz, d, f;
                // Pairwise repulsion.
                for (i = 0; i < n; i++) {
                    a = list[i];
                    let dx = 0, dy = 0, dz = 0;
                    for (j = 0; j < n; j++) {
                        if (i === j) { continue; }
                        b = list[j];
                        ox = a.x - b.x; oy = a.y - b.y; oz = a.z - b.z;
                        d = Math.sqrt(ox * ox + oy * oy + oz * oz);
                        if (d < 0.001) { ox = 0.01; oy = 0.02; oz = 0.01; d = 0.022; }
                        f = (k * k) / d;
                        dx += (ox / d) * f; dy += (oy / d) * f; dz += (oz / d) * f;
                    }
                    a.fx = dx; a.fy = dy; a.fz = dz;
                }
                // Edge springs pulling connected nodes together.
                for (i = 0; i < links.length; i++) {
                    const lk = links[i];
                    a = lk.a; b = lk.b;
                    ox = b.x - a.x; oy = b.y - a.y; oz = b.z - a.z;
                    d = Math.sqrt(ox * ox + oy * oy + oz * oz) || 0.001;
                    f = (d * d) / k * 0.05;
                    const ux = (ox / d) * f, uy = (oy / d) * f, uz = (oz / d) * f;
                    a.fx += ux; a.fy += uy; a.fz += uz;
                    b.fx -= ux; b.fy -= uy; b.fz -= uz;
                }
                // Apply forces with gentle centering gravity, clamped by the
                // cooling temperature, and keep everything inside the sphere.
                for (i = 0; i < n; i++) {
                    a = list[i];
                    const mx = a.fx - a.x * k * 0.04;
                    const my = a.fy - a.y * k * 0.04;
                    const mz = a.fz - a.z * k * 0.04;
                    const dl = Math.sqrt(mx * mx + my * my + mz * mz) || 0.001;
                    const cl = Math.min(dl, temp);
                    a.x += (mx / dl) * cl; a.y += (my / dl) * cl; a.z += (mz / dl) * cl;
                    const rr = Math.sqrt(a.x * a.x + a.y * a.y + a.z * a.z);
                    if (rr > maxR) { const s = maxR / rr; a.x *= s; a.y *= s; a.z *= s; }
                }
            }
        }

        // Add one edge to the merged list, skipping dangling endpoints and
        // duplicates (relation-aware, direction-insensitive).
        function addEdge(raw) {
            if (!raw || edges.length >= MAX_EDGES) { return; }
            const a = nodeById.get(String(raw.source));
            const b = nodeById.get(String(raw.target));
            if (!a || !b || a === b) { return; }
            const rel = raw.relation || '';
            const key = (a.id < b.id ? a.id + '|' + b.id : b.id + '|' + a.id) + '|' + rel;
            if (edgeKeys.has(key)) { return; }
            edgeKeys.add(key);
            edges.push({ a: a, b: b, relation: rel });
        }

        // Create the sphere mesh for one node and register it as pickable.
        // Adds the translucent wireframe shell and, for protected nodes, a
        // tilted gold orbit ring from the shared assets.
        function createNodeMesh(rec) {
            const hue = rec.prot ? GOLD : rec.hue;
            const mesh = new THREE.Mesh(nodeGeo, matFor(hue));
            const s = 0.55 + Math.log(1 + rec.accessCount) * 0.35;
            mesh.scale.set(s, s, s);
            mesh.position.set(rec.x, rec.y, rec.z);
            mesh.userData = {
                kind: 'kgnode', id: rec.id, label: rec.label, type: rec.type,
                accessCount: rec.accessCount, protected: rec.prot
            };
            group.add(mesh);
            rec.mesh = mesh;
            pickables.push(mesh);

            const shell = new THREE.Mesh(shellGeo, shellMatFor(hue));
            const ss = s * 1.5;
            shell.scale.set(ss, ss, ss);
            shell.position.set(rec.x, rec.y, rec.z);
            group.add(shell);
            rec.shell = shell;

            if (rec.prot) {
                const ring = new THREE.Mesh(protRingGeo, protRingMat);
                const rs = s * 1.5;
                ring.scale.set(rs, rs, rs);
                ring.position.set(rec.x, rec.y, rec.z);
                ring.rotation.x = 1.1;
                ring.rotation.y = (hashString(rec.id) % 628) / 100;
                group.add(ring);
                rec.ring = ring;
                protRings.push(ring);
            }
        }

        // Faint glow sprite reserved for the most-accessed nodes. Remembers
        // its resting scale so highlightNeighbors() can restore it later.
        function addGlow(rec) {
            const size = 6 + Math.log(1 + rec.accessCount) * 2;
            const sprite = fxCall('makeGlowSprite', [rec.prot ? GOLD : rec.hue, size]);
            if (!sprite) { return; }
            sprite.position.set(rec.x, rec.y, rec.z);
            group.add(sprite);
            rec.glow = sprite;
            rec.glowBaseScale = size;
        }

        // Rebuild the single merged LineSegments from the current edge list.
        function rebuildEdges() {
            if (edgeLines) {
                group.remove(edgeLines);
                edgeLines.geometry.dispose();
                edgeLines = null;
            }
            if (!edges.length) { return; }
            const pos = new Float32Array(edges.length * 6);
            for (let i = 0; i < edges.length; i++) {
                const e = edges[i];
                pos[i * 6] = e.a.x; pos[i * 6 + 1] = e.a.y; pos[i * 6 + 2] = e.a.z;
                pos[i * 6 + 3] = e.b.x; pos[i * 6 + 4] = e.b.y; pos[i * 6 + 5] = e.b.z;
            }
            const geo = new THREE.BufferGeometry();
            geo.setAttribute('position', new THREE.BufferAttribute(pos, 3));
            edgeLines = new THREE.LineSegments(geo, edgeMat);
            edgeLines.frustumCulled = false;
            group.add(edgeLines);
        }

        // Remove every node/edge visual but keep shared assets alive.
        function clearConstellation() {
            for (let i = 0; i < recs.length; i++) {
                const rec = recs[i];
                if (rec.mesh) { group.remove(rec.mesh); } // shared geo/mat survive
                if (rec.shell) { group.remove(rec.shell); rec.shell = null; }
                if (rec.ring) { group.remove(rec.ring); rec.ring = null; }
                if (rec.glow) {
                    group.remove(rec.glow);
                    if (rec.glow.material) { rec.glow.material.dispose(); }
                }
            }
            recs.length = 0;
            nodeById.clear();
            edges.length = 0;
            edgeKeys.clear();
            pickables.length = 0;
            protRings.length = 0;
            edgeMat.opacity = EDGE_OPACITY_DEFAULT;
            if (edgeLines) {
                group.remove(edgeLines);
                edgeLines.geometry.dispose();
                edgeLines = null;
            }
            glowThreshold = Infinity;
        }

        // Full rebuild from the nodes/edges API payloads.
        function build(nodesRaw, edgesRaw) {
            clearConstellation();
            const list = Array.isArray(nodesRaw) ? nodesRaw.slice(0, MAX_NODES) : [];
            for (let i = 0; i < list.length; i++) {
                const rec = normalizeNode(list[i]);
                if (!rec || nodeById.has(rec.id)) { continue; }
                // Deterministic seeded start position inside a half-radius
                // sphere so repeated rebuilds of the same graph look stable.
                const rand = mulberry32(hashString(rec.id));
                const theta = rand() * Math.PI * 2;
                const phi = Math.acos(2 * rand() - 1);
                const rr = Math.cbrt(rand()) * RADIUS * 0.5;
                rec.x = rr * Math.sin(phi) * Math.cos(theta);
                rec.y = rr * Math.sin(phi) * Math.sin(theta);
                rec.z = rr * Math.cos(phi);
                nodeById.set(rec.id, rec);
                recs.push(rec);
            }
            const rawEdges = Array.isArray(edgesRaw) ? edgesRaw : [];
            for (let i = 0; i < rawEdges.length; i++) { addEdge(rawEdges[i]); }
            runLayout(recs, edges);
            // The top GLOW_TOP nodes with real traffic receive a glow sprite.
            const sorted = recs.slice().sort(function (a, b) { return b.accessCount - a.accessCount; });
            const glowCount = Math.min(GLOW_TOP, sorted.length);
            for (let i = 0; i < glowCount; i++) {
                if (sorted[i].accessCount > 0) {
                    glowThreshold = Math.min(glowThreshold, sorted[i].accessCount);
                }
            }
            for (let i = 0; i < recs.length; i++) {
                createNodeMesh(recs[i]);
                if (recs[i].accessCount >= glowThreshold) { addGlow(recs[i]); }
            }
            rebuildEdges();
        }

        // Merge a node-detail payload ({ node, neighbors, edges }) into the
        // existing constellation around the clicked node.
        function expand(nodeId, detail) {
            if (!detail) { return; }
            const anchor = nodeById.get(String(nodeId)) || null;
            const neighbors = Array.isArray(detail.neighbors) ? detail.neighbors : [];
            let spawned = 0;
            // Expand may exceed the initial build cap by a small headroom so
            // drilling into neighbors still works on a maxed-out first build.
            const expandCap = MAX_NODES + EXPAND_HEADROOM;
            for (let i = 0; i < neighbors.length && recs.length < expandCap; i++) {
                const rec = normalizeNode(neighbors[i]);
                if (!rec || nodeById.has(rec.id)) { continue; }
                // Place the new node on a small sphere around the anchor.
                const rand = mulberry32(hashString(rec.id) ^ Math.imul(recs.length + 1, 2654435761));
                const theta = rand() * Math.PI * 2;
                const phi = Math.acos(2 * rand() - 1);
                const rr = 4 + rand() * 4;
                const cx = anchor ? anchor.x : 0;
                const cy = anchor ? anchor.y : 0;
                const cz = anchor ? anchor.z : 0;
                rec.x = cx + rr * Math.sin(phi) * Math.cos(theta);
                rec.y = cy + rr * Math.sin(phi) * Math.sin(theta);
                rec.z = cz + rr * Math.cos(phi);
                // Keep merged nodes inside the constellation sphere.
                const dist = Math.sqrt(rec.x * rec.x + rec.y * rec.y + rec.z * rec.z);
                const maxR = RADIUS * 0.95;
                if (dist > maxR) { const s = maxR / dist; rec.x *= s; rec.y *= s; rec.z *= s; }
                nodeById.set(rec.id, rec);
                recs.push(rec);
                createNodeMesh(rec);
                if (rec.accessCount >= glowThreshold) { addGlow(rec); }
                // Burst feedback at the spawn position (interaction-time, so
                // the one Vector3 allocation here is fine).
                if (rec.mesh) {
                    fxCall('burst', [rec.mesh.getWorldPosition(new THREE.Vector3()), rec.prot ? GOLD : rec.hue, 14]);
                }
                spawned++;
            }
            const rawEdges = Array.isArray(detail.edges) ? detail.edges : [];
            for (let i = 0; i < rawEdges.length; i++) { addEdge(rawEdges[i]); }
            if (spawned > 0 || rawEdges.length > 0) { rebuildEdges(); }
        }

        // World position of a node for comets / camera fly-to, or null.
        function nodePosition(id) {
            const rec = nodeById.get(String(id));
            if (!rec || !rec.mesh) { return null; }
            return rec.mesh.getWorldPosition(new THREE.Vector3());
        }

        // HUD visibility toggle for the whole constellation.
        function setVisible(v) {
            visible = !!v;
            group.visible = visible;
        }

        // Hover highlight: boosts one node plus its direct neighbors (bigger
        // glow sprites, brighter edges); null restores the resting state.
        // Unknown ids are ignored gracefully.
        function highlightNeighbors(nodeIdOrNull) {
            let i, rec;
            // Always restore the resting state first so consecutive
            // highlights can never accumulate.
            for (i = 0; i < recs.length; i++) {
                rec = recs[i];
                if (rec.glow) {
                    rec.glow.scale.set(rec.glowBaseScale, rec.glowBaseScale, 1);
                }
            }
            edgeMat.opacity = EDGE_OPACITY_DEFAULT;
            if (nodeIdOrNull == null) { return; }
            const target = nodeById.get(String(nodeIdOrNull));
            if (!target) { return; }
            if (target.glow) {
                const ts = target.glowBaseScale * 1.6;
                target.glow.scale.set(ts, ts, 1);
            }
            for (i = 0; i < edges.length; i++) {
                const e = edges[i];
                let other = null;
                if (e.a === target) { other = e.b; }
                else if (e.b === target) { other = e.a; }
                if (other && other.glow) {
                    const os = other.glowBaseScale * 1.6;
                    other.glow.scale.set(os, os, 1);
                }
            }
            edgeMat.opacity = EDGE_OPACITY_HIGHLIGHT;
        }

        // Synapse pulses: one comet per interval along a random edge. The two
        // scratch vectors keep the per-pulse math allocation-free.
        const SYNAPSE_INTERVAL = 2.2;
        let synapseTimer = 0;
        const synFrom = new THREE.Vector3();
        const synTo = new THREE.Vector3();

        // Per-frame: very subtle rotation of the whole constellation plus a
        // slow shell shimmer, gold-ring tumble and synapse pulses.
        // No allocations here.
        function update(dt, elapsed) {
            if (!visible) { return; }
            group.rotation.y += dt * ROT_SPEED;
            shell.rotation.y -= dt * ROT_SPEED * 0.6;
            shell.rotation.x += dt * 0.003;
            shell.material.opacity = 0.04 + 0.015 * (0.5 + 0.5 * Math.sin(elapsed * 0.5));
            // Slow tumble of the protected-node gold orbit rings.
            for (let i = 0; i < protRings.length; i++) {
                protRings[i].rotation.y += dt * 0.5;
            }
            synapseTimer += dt;
            if (synapseTimer >= SYNAPSE_INTERVAL) {
                synapseTimer -= SYNAPSE_INTERVAL;
                const q = typeof inst.qualityScale === 'number' ? inst.qualityScale : 1;
                if (q >= 0.5 && edges.length) {
                    const e = edges[(Math.random() * edges.length) | 0];
                    if (e.a.mesh && e.b.mesh) {
                        // Exact world positions via the shared scratch
                        // vectors (the constellation group itself rotates).
                        e.a.mesh.getWorldPosition(synFrom);
                        e.b.mesh.getWorldPosition(synTo);
                        fxCall('comet', [synFrom, synTo, EDGE_COLOR, { size: 1.2, arc: 0.02 }]);
                    }
                }
            }
        }

        // Free every GPU resource and detach the constellation group.
        function dispose() {
            clearConstellation();
            edgeMat.dispose();
            nodeGeo.dispose();
            matCache.forEach(function (m) { m.dispose(); });
            matCache.clear();
            shellGeo.dispose();
            shellMatCache.forEach(function (m) { m.dispose(); });
            shellMatCache.clear();
            protRingGeo.dispose();
            protRingMat.dispose();
            shell.geometry.dispose();
            shell.material.dispose();
            if (group.parent) { group.parent.remove(group); }
        }

        return {
            build: build,
            expand: expand,
            pickables: pickables,
            nodePosition: nodePosition,
            setVisible: setVisible,
            highlightNeighbors: highlightNeighbors,
            update: update,
            dispose: dispose
        };
    };
})();
