// Storymaker app assets: what the parent hands the sandboxed player. The catalog (catalog.json and the part files it lists)
// and the fonts come from the static files below /img/storymaker and /fonts, the code-scene bundles from the project API.
// prepare() builds the first `load` message; resolve() answers `need_assets` and serves only what the catalog lists
// (rig.file, entries[].file, its fonts) or the bundle of a code scene of the loaded story; every other key is answered in
// `missing`, never silently. Parsed files and font bytes are cached per catalog version. Classic script, publishes
// S.createAssets.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};
    const KEY_RE = /^(catalog|font|bundle):(\S{1,230})$/;
    const FONT_FILE_RE = /^\/fonts\/(qr\/)?[A-Za-z0-9_.-]+$/;

    function createAssets(api) {
        let catalogPromise = null;
        let current = null;           // {raw, index, paths}
        const cache = new Map();      // asset key -> parsed JSON or ArrayBuffer

        function allowedPaths(raw) {
            const paths = new Set();
            if (raw.rig && typeof raw.rig.file === 'string') paths.add(raw.rig.file);
            (raw.entries || []).forEach(entry => { if (entry && typeof entry.file === 'string') paths.add(entry.file); });
            return paths;
        }

        function loadCatalog() {
            if (!catalogPromise) {
                catalogPromise = api.catalogJSON('catalog.json').then(async raw => {
                    // the coverage file travels on the catalog as glyph_data: in memory only, never in catalog.json
                    if (raw && raw.glyphs && typeof raw.glyphs.file === 'string') raw.glyph_data = await api.catalogJSON(raw.glyphs.file);
                    return raw;
                }).then(raw => {
                    if (!raw || raw.schema !== 'storymaker-catalog/1' || !Array.isArray(raw.entries)) throw S.errors.normalize({ message: 'The catalog has an unknown format.' });
                    if (current && current.raw.version !== raw.version) cache.clear();
                    current = { raw, index: S.model.indexCatalog(raw), paths: allowedPaths(raw) };
                    return current;
                }, err => {
                    catalogPromise = null;
                    throw err;
                });
            }
            return catalogPromise;
        }

        async function cached(key, fetcher) {
            if (cache.has(key)) return cache.get(key);
            const value = await fetcher();
            cache.set(key, value);
            return value;
        }

        function fontFaces(raw, name) {
            return (raw.fonts || []).filter(font => font && font.name === name && typeof font.file === 'string' && FONT_FILE_RE.test(font.file));
        }

        // The fonts the story uses, as the `load.fonts` entries {name, family, weight, data}.
        async function fontsFor(raw, story) {
            const name = story && story.style && story.style.font ? story.style.font : 'default';
            const faces = fontFaces(raw, name);
            return Promise.all(faces.map(async face => ({
                name: face.name,
                family: String(face.family),
                weight: Number(face.weight),
                data: await cached('font:' + face.name + '/' + face.weight, () => api.fontBuffer(face.file))
            })));
        }

        // glyphFace(raw, id): the fallback face of the coverage file, with its file and max_bytes. Its key is always font:<id>/400.
        function glyphFace(raw, id) {
            const faces = raw.glyph_data && Array.isArray(raw.glyph_data.faces) ? raw.glyph_data.faces : [];
            return faces.find(face => face && face.id === id && typeof face.file === 'string' && FONT_FILE_RE.test(face.file));
        }

        function codeSceneIds(story) {
            return S.model.scenesOf(story).filter(scene => scene && scene.kind === 'code' && typeof scene.id === 'string').map(scene => scene.id);
        }

        // prepare(story, projectId) -> the parts of the `load` message that do not depend on the story text
        async function prepare(story, projectId) {
            const { raw } = await loadCatalog();
            const assets = {};
            if (raw.rig && raw.rig.file) assets['catalog:' + raw.rig.file] = await cached('catalog:' + raw.rig.file, () => api.catalogJSON(raw.rig.file));
            const fonts = await fontsFor(raw, story);
            const bundles = {};
            await Promise.all(codeSceneIds(story).map(async id => {
                try {
                    const text = await api.getBundle(projectId, id);
                    if (typeof text === 'string') bundles[id] = text;
                } catch (_) { /* the player reports the scene without a bundle */ }
            }));
            return { catalog: raw, assets, fonts, code_bundles: bundles };
        }

        // resolve(keys, story, projectId) -> {assets, missing}
        async function resolve(keys, story, projectId) {
            const { raw, paths } = await loadCatalog();
            const assets = {};
            const missing = [];
            const codeIds = new Set(codeSceneIds(story));
            await Promise.all((Array.isArray(keys) ? keys : []).map(async key => {
                const m = typeof key === 'string' ? KEY_RE.exec(key) : null;
                if (!m) { missing.push(String(key).slice(0, 240)); return; }
                try {
                    if (m[1] === 'catalog') {
                        if (!paths.has(m[2])) { missing.push(key); return; }
                        assets[key] = await cached(key, () => api.catalogJSON(m[2]));
                    } else if (m[1] === 'font') {
                        const parts = /^([^/]+)\/(\d{3})$/.exec(m[2]);
                        const face = parts && (fontFaces(raw, parts[1]).find(font => String(font.weight) === parts[2]) || (parts[2] === '400' && glyphFace(raw, parts[1])));
                        if (!face) { missing.push(key); return; }
                        assets[key] = await cached(key, () => api.fontBuffer(face.file, face.max_bytes));
                    } else if (m[1] === 'bundle') {
                        if (!codeIds.has(m[2])) { missing.push(key); return; }
                        const text = await api.getBundle(projectId, m[2]);
                        if (typeof text === 'string') assets[key] = text; else missing.push(key);
                    }
                } catch (_) {
                    missing.push(key);
                }
            }));
            return { assets, missing };
        }

        return { loadCatalog, prepare, resolve, index: () => (current ? current.index : null), raw: () => (current ? current.raw : null) };
    }

    S.createAssets = createAssets;
})();
