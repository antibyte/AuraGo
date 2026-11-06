/* Aura Drift - versioned URLs, GLB loading, templates and stand-ins.
 *
 * This is the only module allowed to call the network, and every call goes through AD.assetURL.
 * Templates are CPU-side THREE objects shared by every window (refcounted: acquire / release).
 * A getter never fails: when a GLB is missing it returns a stand-in built from primitives with the
 * same node names, so the actors need no special case. Renderers clone templates; they never own
 * geometry or the shared materials of a template. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const ROOT = '/3d/aura-drift/v1/';
    const CONCURRENCY = 4;
    const CORE = ['items.glb', 'track.glb', 'animals.glb', 'shared.glb'];

    AD.assetURL = path => (window.AuraLazyAssets && window.AuraLazyAssets.versionedURL(path)) ||
        path + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');

    /* Colour and size of each stand-in: [shape, colour, w, h, d]. */
    AD.STAND_INS = {
        item: {
            crate: ['box', '#f0b030', 1.6, 1.6, 1.6], gnome_mine: ['cone', '#d94a3a', 1.2, 1.8, 1.2], rubber_duck: ['sphere', '#ffd23f', 1.2, 1.2, 1.2],
            rocket: ['cone', '#e8e8f0', 0.8, 2.6, 0.8], bratwurst: ['box', '#b5603a', 1.8, 0.5, 0.5], boxing_glove: ['sphere', '#d93a3a', 1.4, 1.4, 1.4],
            disco_ball: ['sphere', '#c8d8ff', 1.4, 1.4, 1.4], harpoon: ['cone', '#9aa4b0', 0.4, 3.4, 0.4], air_horn: ['cone', '#e03030', 0.9, 1.6, 0.9],
            frost_grenade: ['sphere', '#8ce9ff', 0.9, 0.9, 0.9]
        },
        animal: {
            cow: ['box', '#f2f2ee', 1.0, 1.5, 2.2], sheep: ['box', '#f4f0e6', 0.9, 1.1, 1.4], penguin: ['cone', '#1c2430', 0.7, 1.2, 0.7],
            crab: ['box', '#e0553a', 1.0, 0.5, 0.8], llama: ['box', '#d8c4a0', 0.8, 1.7, 1.6]
        },
        track: {
            ramp: ['box', '#c8a050', 9, 1.6, 9], boost_pad: ['box', '#30d0ff', 10, 0.12, 6], cone: ['cone', '#ff7a1a', 0.6, 0.8, 0.6],
            hay_bale: ['box', '#d8b84a', 1.9, 1.9, 1.9], barrel: ['box', '#cc3a2a', 1.0, 1.4, 1.0], tyre_stack: ['box', '#222222', 1.6, 1.5, 1.6],
            barrier_segment: ['box', '#cfd4da', 4, 1, 0.4], gantry: ['box', '#e0e4ea', 1, 7, 14.6], rumble_bars: ['box', '#e04040', 8.3, 0.1, 12]
        },
        prop: { default: ['cone', '#7a8a6a', 4, 8, 4] }
    };

    const st = {
        refs: 0, gen: 0, manifest: null, manifestP: null, files: {}, pending: {}, standIn: {}, standNames: {},
        asked: 0, done: 0, loaded: 0, failed: 0, manifestSettled: false
    };
    const queue = [];
    let running = 0;

    function T3() { return window.AuraDriftThree || null; }

    function pump() {
        while (running < CONCURRENCY && queue.length) {
            const job = queue.shift();
            running++;
            job().then(() => { running--; pump(); }, () => { running--; pump(); });
        }
    }
    function schedule(work) {
        return new Promise(resolve => { queue.push(() => work().then(resolve, resolve)); pump(); });
    }

    /* A stalled request must end as a failure so stand-ins take over; the abort also covers reading the body. */
    const FETCH_TIMEOUT_MS = 20000;
    function fetchTimed(path, init, read) {
        const ctl = typeof AbortController === 'function' ? new AbortController() : null;
        const timer = ctl ? setTimeout(() => ctl.abort(), FETCH_TIMEOUT_MS) : 0;
        const options = Object.assign({}, init, ctl ? { signal: ctl.signal } : {});
        return fetch(AD.assetURL(path), options).then(r => {
            if (!r.ok) throw new Error('asset ' + r.status);
            return read(r);
        }).then(v => { clearTimeout(timer); return v; }, e => { clearTimeout(timer); throw e; });
    }
    AD.assetsFetchBytes = path => fetchTimed(path, { cache: 'force-cache' }, r => r.arrayBuffer());
    AD.assetsFetchJson = path => fetchTimed(path, {}, r => r.json());

    function loadManifest() {
        if (st.manifest) return Promise.resolve(st.manifest);
        if (!st.manifestP) {
            const gen = st.gen;
            st.manifestP = AD.assetsFetchJson(ROOT + 'manifest.json').then(m => {
                if (gen === st.gen) { st.manifest = m; st.manifestSettled = true; }
                return m;
            }).catch(() => {
                /* Cleared so a later window can retry; progress no longer waits for it. */
                if (gen === st.gen) { st.manifestP = null; st.manifestSettled = true; }
                return null;
            });
        }
        return st.manifestP;
    }

    function disposeObject(root) {
        const seen = new Set();
        root.traverse(o => {
            if (o.geometry) o.geometry.dispose();
            const mats = Array.isArray(o.material) ? o.material : (o.material ? [o.material] : []);
            for (const m of mats) {
                if (seen.has(m)) continue;
                seen.add(m);
                for (const key of Object.keys(m)) if (m[key] && m[key].isTexture) m[key].dispose();
                m.dispose();
            }
        });
    }

    function loadFile(name) {
        if (st.pending[name]) return st.pending[name];
        const T = T3();
        const gen = st.gen;
        st.asked++;
        st.pending[name] = schedule(() => {
            if (!T || !T.GLTFLoader) return Promise.reject(new Error('no three'));
            return AD.assetsFetchBytes(ROOT + name).then(bytes => new T.GLTFLoader().parseAsync(bytes, ''));
        }).then(gltf => {
            st.done++;
            if (!gltf || !gltf.scene) { st.failed++; if (gen === st.gen) delete st.pending[name]; return null; }
            if (gen !== st.gen) { disposeObject(gltf.scene); return null; }
            st.loaded++;
            st.files[name] = gltf.scene;
            return gltf.scene;
        }, () => { st.done++; st.failed++; if (gen === st.gen) delete st.pending[name]; return null; });
        return st.pending[name];
    }

    function carFiles() { return AD.CARS.map(c => 'car-' + c.id + '.glb'); }

    function node(file, name) {
        const root = st.files[file];
        return root ? root.getObjectByName(name) || null : null;
    }

    /* ---- stand-ins ---------------------------------------------------- */
    function mat(T, name, colour, extra) {
        const m = new T.MeshStandardMaterial(Object.assign({ color: colour, roughness: 0.6, metalness: 0.05 }, extra || {}));
        m.name = name;
        return m;
    }
    function shape(T, spec) {
        const s = spec || AD.STAND_INS.prop.default;
        let g;
        if (s[0] === 'sphere') g = new T.SphereGeometry(0.5, 14, 10);
        else if (s[0] === 'cone') g = new T.ConeGeometry(0.5, 1, 10);
        else g = new T.BoxGeometry(1, 1, 1);
        const mesh = new T.Mesh(g, mat(T, 'standin', s[1]));
        mesh.scale.set(s[2], s[3], s[4]);
        mesh.position.y = s[3] / 2;
        const group = new T.Group();
        group.add(mesh);
        return group;
    }
    function standCar(T, id) {
        const meta = (st.manifest && st.manifest.cars && st.manifest.cars[id]) || { length: 4.4, width: 1.9, height: 1.15, wheelRadius: 0.36, wheelbase: 2.7 };
        const group = new T.Group();
        const body = new T.Group();
        body.name = 'body';
        const lift = meta.wheelRadius * 0.9;
        const paint = new T.Mesh(new T.BoxGeometry(meta.length, meta.height * 0.42, meta.width * 0.92), mat(T, 'paint', '#ffffff', { metalness: 0.3, roughness: 0.3 }));
        paint.position.y = lift + meta.height * 0.2;
        const glass = new T.Mesh(new T.BoxGeometry(meta.length * 0.45, meta.height * 0.3, meta.width * 0.8), mat(T, 'glass', '#16202e', { roughness: 0.05 }));
        glass.position.set(-meta.length * 0.05, lift + meta.height * 0.5, 0);
        const trim = new T.Mesh(new T.BoxGeometry(meta.length * 0.98, meta.height * 0.12, meta.width * 0.95), mat(T, 'trim', '#1c2026'));
        trim.position.y = lift;
        body.add(paint, glass, trim);
        group.add(body);
        const wheelGeo = new T.CylinderGeometry(meta.wheelRadius, meta.wheelRadius, meta.width * 0.14, 14);
        wheelGeo.rotateX(Math.PI / 2);
        const tyre = mat(T, 'tyre', '#1a1a1a', { roughness: 0.8 });
        const half = meta.wheelbase / 2, side = meta.width * 0.5;
        for (const [n, x, z] of [['wheel_fl', half, -side], ['wheel_fr', half, side], ['wheel_rl', -half, -side], ['wheel_rr', -half, side]]) {
            const w = new T.Mesh(wheelGeo, tyre);
            w.name = n;
            w.position.set(x, meta.wheelRadius, z);
            group.add(w);
        }
        const anchors = [['anchor_exhaust_l', -1, 0.4, -0.4], ['anchor_exhaust_r', -1, 0.4, 0.4], ['anchor_head_l', 1, 0.6, -0.6],
            ['anchor_head_r', 1, 0.6, 0.6], ['anchor_tail_l', -1, 0.7, -0.6], ['anchor_tail_r', -1, 0.7, 0.6], ['anchor_weapon', 0, meta.height + 0.1, 0]];
        for (const [n, fx, y, fz] of anchors) {
            const a = new T.Object3D();
            a.name = n;
            a.position.set(fx * meta.length * 0.5, y, fz * meta.width * 0.5 / 0.6 * 0.6);
            group.add(a);
        }
        return group;
    }
    /* The real template when the GLB arrived, else a cached stand-in. A stand-in whose
       GLB has since loaded is freed, so stats().standIns counts only what is still missing. */
    function standFor(kind, name, real, build) {
        const key = kind + ':' + name;
        if (real) {
            if (st.standIn[key]) { disposeObject(st.standIn[key]); delete st.standIn[key]; delete st.standNames[key]; }
            return real;
        }
        if (!st.standIn[key]) {
            const T = T3();
            if (!T) return null;
            st.standIn[key] = build(T);
            st.standNames[key] = true;
        }
        return st.standIn[key];
    }

    function disposeAll() {
        for (const root of Object.values(st.files)) disposeObject(root);
        for (const root of Object.values(st.standIn)) disposeObject(root);
        st.gen++;
        st.manifest = null; st.manifestP = null; st.files = {}; st.pending = {}; st.standIn = {}; st.standNames = {};
        st.asked = 0; st.done = 0; st.loaded = 0; st.failed = 0; st.manifestSettled = false;
    }

    AD.assets = {
        acquire() { st.refs++; },
        release() {
            st.refs = Math.max(0, st.refs - 1);
            if (st.refs === 0) disposeAll();
        },
        manifest() { return st.manifest; },
        loadCore() {
            return loadManifest().then(() => Promise.all(carFiles().concat(CORE).map(loadFile))).then(() => true);
        },
        loadBiome(id) {
            if (!AD.LOOK || !AD.LOOK[id]) return Promise.resolve(false);
            return loadManifest().then(() => loadFile('scenery-' + id + '.glb')).then(() => true);
        },
        car(id) {
            return standFor('car', id, st.files['car-' + id + '.glb'], T => standCar(T, id));
        },
        item(name) {
            return standFor('item', name, node('items.glb', 'item-' + name), T => shape(T, AD.STAND_INS.item[name]));
        },
        trackPiece(name) {
            return standFor('track', name, node('track.glb', 'track-' + name), T => shape(T, AD.STAND_INS.track[name]));
        },
        animal(kind) {
            return standFor('animal', kind, node('animals.glb', 'animal-' + kind), T => shape(T, AD.STAND_INS.animal[kind]));
        },
        prop(biomeId, name) {
            const file = biomeId === 'shared' ? 'shared.glb' : 'scenery-' + biomeId + '.glb';
            return standFor('prop', biomeId + '/' + name, node(file, 'prop-' + name), T => {
                const meta = st.manifest && st.manifest.props && st.manifest.props[biomeId] && st.manifest.props[biomeId][name];
                const r = meta ? meta.radius : 2, h = meta && meta.height ? meta.height : r * 2;
                return shape(T, [h > r * 1.6 ? 'cone' : 'box', '#7a8a6a', r * 1.4, h, r * 1.4]);
            });
        },
        isStandIn(obj) { return Object.values(st.standIn).indexOf(obj) >= 0; },
        progress() { return st.manifestSettled && st.asked ? Math.min(1, st.done / st.asked) : 0; },
        stats() { return { loaded: st.loaded, failed: st.failed, standIns: Object.keys(st.standNames).length, names: Object.keys(st.standNames) }; },
        fetchBytes: AD.assetsFetchBytes,
        fetchJson: AD.assetsFetchJson
    };
})();
