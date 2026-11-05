// Storymaker host (document realm): app mode. The page is the sandboxed player iframe of the desktop app. It speaks the
// typed protocol of contract section 8 with its parent only: messages are accepted only when event.source is the parent
// window, only in the fixed schema with size caps, and replies are posted to the origin of data-sm-base. Every load resets
// the previous state. Playback is real time with frame dropping; the visible canvas shows the same bitmap the render page
// would encode. Classic script; publishes StorymakerHost.mountApp, checkMessage.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    var CAPS = {
        maxFallbackFontBytes: 18 * 1048576,
        maxScenes: 400, maxEntries: 6000, maxAssetKeys: 2000, maxFonts: 16, maxFontBytes: 8 * 1048576, maxFontTotal: 32 * 1048576,
        maxAudioScenes: 400, maxAudioBlocks: 61, maxAudioPieces: 800, maxLevels: 27000, maxTotalLevels: 200000, maxGlyphFaces: 16, maxGlyphRanges: 60000,
        maxBundles: 400, maxBundleChars: 262144, maxFrame: 100000000, thumbMin: 16, thumbMax: 1920, maxKey: 240, maxReq: 64
    };
    var TYPES = ['load', 'assets', 'seek', 'play', 'pause', 'thumb', 'reset'];
    var KEY_RE = /^(catalog|font|bundle):[^\s]{1,230}$/;
    var ID_RE = /^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$/;

    function isObject(v) { return v !== null && typeof v === 'object' && !Array.isArray(v); }
    function isBuffer(v) { return Object.prototype.toString.call(v) === '[object ArrayBuffer]'; }
    function isInt(v, lo, hi) { return typeof v === 'number' && isFinite(v) && Math.floor(v) === v && v >= lo && v <= hi; }
    function okReq(v) { return (typeof v === 'string' || typeof v === 'number') && String(v).length <= CAPS.maxReq; }

    // checkAudioInfo(info) = null when audio_info fits its caps, else a short reason. maxAudioBlocks counts per scene (one narration
    // plus 60 beats), maxAudioPieces and maxLevels per block, maxTotalLevels per message.
    function checkAudioInfo(info) {
        if (!isObject(info)) return 'load.audio_info is invalid';
        if (info.narration === undefined) return null;
        if (!isObject(info.narration)) return 'load.audio_info.narration is invalid';
        var ids = Object.keys(info.narration), total = 0, i, j;
        if (ids.length > CAPS.maxAudioScenes) return 'load.audio_info has too many scenes';
        for (i = 0; i < ids.length; i++) {
            var scene = info.narration[ids[i]];
            if (!isObject(scene) || !Array.isArray(scene.blocks)) return 'load.audio_info has a bad scene';
            if (scene.blocks.length > CAPS.maxAudioBlocks) return 'load.audio_info has too many blocks in a scene';
            for (j = 0; j < scene.blocks.length; j++) {
                var block = scene.blocks[j];
                if (!isObject(block)) return 'load.audio_info has a bad block';
                var pieces = block.pieces || [], levels = block.levels || [];
                if (!Array.isArray(pieces) || !Array.isArray(levels)) return 'load.audio_info has a bad block';
                if (pieces.length > CAPS.maxAudioPieces) return 'load.audio_info has too many pieces in a block';
                if (levels.length > CAPS.maxLevels) return 'load.audio_info has too many levels in a block';
                total += levels.length;
                if (total > CAPS.maxTotalLevels) return 'load.audio_info has too many levels';
            }
        }
        return null;
    }

    // checkGlyphData(data) = null when catalog.glyph_data has the schema string and fits its caps, else a short reason.
    // Each range takes two values of d. maxGlyphRanges counts across all faces.
    function checkGlyphData(data) {
        if (!isObject(data) || data.schema !== 'storymaker-glyphs/1') return 'load.catalog.glyph_data needs the schema string';
        if (!Array.isArray(data.faces) || data.faces.length > CAPS.maxGlyphFaces) return 'load.catalog.glyph_data has bad faces or too many faces';
        var ranges = 0;
        for (var i = 0; i < data.faces.length; i++) {
            var face = data.faces[i];
            if (!isObject(face) || !Array.isArray(face.d)) return 'load.catalog.glyph_data has bad faces or too many faces';
            ranges += face.d.length / 2;
            if (ranges > CAPS.maxGlyphRanges) return 'load.catalog.glyph_data has too many ranges';
        }
        return null;
    }

    // facesOf(catalog): the fallback faces of a catalog's glyph data. Only these may name a font over maxFontBytes.
    function facesOf(catalog) {
        var data = isObject(catalog) ? catalog.glyph_data : null;
        return isObject(data) && data.schema === 'storymaker-glyphs/1' && Array.isArray(data.faces) ? data.faces : [];
    }

    // fontLimit(name, faces): the byte limit of one font. A face with that id lifts it to its max_bytes, at most maxFallbackFontBytes.
    function fontLimit(name, faces) {
        var limit = CAPS.maxFontBytes;
        faces.forEach(function (face) {
            if (isObject(face) && face.id === name && typeof face.max_bytes === 'number' && face.max_bytes > limit) limit = Math.min(face.max_bytes, CAPS.maxFallbackFontBytes);
        });
        return limit;
    }

    // checkMessage(msg, loaded) = null when the message fits the schema, else a short reason. loaded = {faces, fontBytes} comes
    // from the last accepted load, because an assets message carries no catalog. A missing loaded means no faces and 0 bytes.
    function checkMessage(msg, loaded) {
        if (!isObject(msg) || msg.sm !== 1) return 'not a storymaker message';
        if (TYPES.indexOf(msg.type) < 0) return 'unknown message type';
        var i, k, total = 0;
        var heldFaces = loaded && Array.isArray(loaded.faces) ? loaded.faces : [], heldBytes = loaded && loaded.fontBytes > 0 ? loaded.fontBytes : 0;
        switch (msg.type) {
            case 'load': {
                if (!isObject(msg.story) || !Array.isArray(msg.story.scenes) || msg.story.scenes.length > CAPS.maxScenes) return 'load.story is invalid or too large';
                if (!isObject(msg.catalog) || !Array.isArray(msg.catalog.entries) || msg.catalog.entries.length > CAPS.maxEntries) return 'load.catalog is invalid or too large';
                if (msg.catalog.glyph_data !== undefined) {
                    var badGlyphs = checkGlyphData(msg.catalog.glyph_data);
                    if (badGlyphs) return badGlyphs;
                }
                if (msg.audio_info !== undefined) {
                    var badAudio = checkAudioInfo(msg.audio_info);
                    if (badAudio) return badAudio;
                }
                if (msg.assets !== undefined) {
                    if (!isObject(msg.assets) || Object.keys(msg.assets).length > CAPS.maxAssetKeys) return 'load.assets is invalid or too large';
                    for (k in msg.assets) if (!KEY_RE.test(k)) return 'load.assets has a bad key';
                }
                if (msg.fonts !== undefined) {
                    if (!Array.isArray(msg.fonts) || msg.fonts.length > CAPS.maxFonts) return 'load.fonts is invalid or too large';
                    var loadFaces = facesOf(msg.catalog).concat(heldFaces);
                    for (i = 0; i < msg.fonts.length; i++) {
                        var f = msg.fonts[i];
                        if (!isObject(f) || typeof f.name !== 'string' || f.name.length > 64 || typeof f.family !== 'string' || f.family.length > 128 || !isInt(f.weight, 100, 900) || !isBuffer(f.data)) return 'load.fonts has a bad entry';
                        if (f.weight_range !== undefined && (typeof f.weight_range !== 'string' || f.weight_range.length > 16)) return 'load.fonts has a bad entry';
                        if (f.unicode_range !== undefined && (typeof f.unicode_range !== 'string' || f.unicode_range.length > 256)) return 'load.fonts has a bad entry';
                        if (f.data.byteLength > fontLimit(f.name, loadFaces)) return 'load.fonts has an oversized font';
                        total += f.data.byteLength;
                    }
                    if (total > CAPS.maxFontTotal) return 'load.fonts is too large';
                }
                if (msg.code_bundles !== undefined) {
                    if (!isObject(msg.code_bundles) || Object.keys(msg.code_bundles).length > CAPS.maxBundles) return 'load.code_bundles is invalid or too large';
                    for (k in msg.code_bundles) {
                        if (!ID_RE.test(k) || typeof msg.code_bundles[k] !== 'string' || msg.code_bundles[k].length > CAPS.maxBundleChars) return 'load.code_bundles has a bad entry';
                    }
                }
                if (msg.format !== undefined) {
                    var fm = msg.format;
                    if (!isObject(fm)) return 'load.format is invalid';
                    if (fm.aspect !== undefined && ['16:9', '9:16', '1:1'].indexOf(fm.aspect) < 0) return 'load.format.aspect is invalid';
                    if (fm.fps !== undefined && [24, 25, 30].indexOf(fm.fps) < 0) return 'load.format.fps is invalid';
                    if (fm.width !== undefined && !isInt(fm.width, 64, 3840)) return 'load.format.width is invalid';
                }
                return null;
            }
            case 'assets': {
                if (!okReq(msg.req)) return 'assets.req is invalid';
                if (!isObject(msg.assets) || Object.keys(msg.assets).length > CAPS.maxAssetKeys) return 'assets.assets is invalid or too large';
                for (k in msg.assets) if (!KEY_RE.test(k)) return 'assets.assets has a bad key';
                for (k in msg.assets) {
                    if (k.indexOf('font:') !== 0) continue;
                    if (!isBuffer(msg.assets[k])) return 'assets.assets has a bad font';
                    if (msg.assets[k].byteLength > fontLimit(k.slice(5).replace(/\/\d+$/, ''), heldFaces)) return 'assets.assets has an oversized font';
                    total += msg.assets[k].byteLength;
                }
                if (heldBytes + total > CAPS.maxFontTotal) return 'assets.assets is too large';
                if (msg.missing !== undefined) {
                    if (!Array.isArray(msg.missing) || msg.missing.length > CAPS.maxAssetKeys) return 'assets.missing is invalid';
                    for (i = 0; i < msg.missing.length; i++) if (typeof msg.missing[i] !== 'string' || msg.missing[i].length > CAPS.maxKey) return 'assets.missing has a bad key';
                }
                return null;
            }
            case 'seek':
                return isInt(msg.frame, 0, CAPS.maxFrame) ? null : 'seek.frame is invalid';
            case 'thumb':
                if (!okReq(msg.req)) return 'thumb.req is invalid';
                if (typeof msg.scene_id !== 'string' || !ID_RE.test(msg.scene_id)) return 'thumb.scene_id is invalid';
                if (msg.frame !== undefined && !isInt(msg.frame, 0, CAPS.maxFrame)) return 'thumb.frame is invalid';
                return isInt(msg.width, CAPS.thumbMin, CAPS.thumbMax) ? null : 'thumb.width is invalid';
            default:
                return null;
        }
    }

    function originOf(base) {
        try { return new root.URL(base).origin; } catch (e) { return '*'; }
    }

    // mountApp(opts): opts = {canvas, base, window?, env?}. Returns {dispose, state} (state is for tests).
    function mountApp(opts) {
        var win = opts.window || root, canvas = opts.canvas, base = opts.base;
        var env = H.defaultEnv(base, Object.assign({ raf: function (fn) { return win.requestAnimationFrame(fn); }, cancelRaf: function (h) { win.cancelAnimationFrame(h); } }, opts.env || {}));
        var targetOrigin = opts.targetOrigin || originOf(base);
        var ctx = canvas.getContext('2d');
        var state = { engine: null, report: null, frame: 0, playing: false, loadToken: 0, busy: false, shown: -1, lastTime: -1e9, pendingAssets: {}, nextAsset: 1, loaded: { faces: [], fontBytes: 0 } };
        var disposed = false;

        function post(msg, transfer) {
            var parent = win.parent;
            if (!parent || parent === win) return;
            var out = { sm: 1 };
            for (var k in msg) out[k] = msg[k];
            parent.postMessage(out, targetOrigin, transfer || []);
        }
        function report(err, sceneId) {
            var e = H.asError(err);
            var m = { type: 'error', code: e.code, message: String(e.message).slice(0, 600) };
            var sid = sceneId || e.scene_id;
            if (sid) m.scene_id = sid;
            if (typeof e.line === 'number') { m.line = e.line; if (typeof e.column === 'number') m.column = e.column; }
            post(m);
        }
        env.onError = function (e) { report(e, e.scene_id); };

        env.requestAssets = function (keys) {
            return new Promise(function (resolve, reject) {
                var req = 'a' + (state.nextAsset++);
                var timer = env.setTimer(function () { delete state.pendingAssets[req]; reject(H.fail('readiness', 'The assets were not delivered in time.')); }, env.limits.assetsDeadlineMs);
                state.pendingAssets[req] = { resolve: resolve, reject: reject, timer: timer };
                post({ type: 'need_assets', req: req, keys: keys });
            });
        };

        function stopLoop() {
            state.playing = false;
            if (state.rafHandle !== undefined) { env.cancelRaf(state.rafHandle); state.rafHandle = undefined; }
        }
        function reset() {
            state.loadToken++;
            stopLoop();
            if (state.engine) state.engine.dispose();
            state.engine = null;
            state.report = null;
            state.frame = 0;
            state.shown = -1;
            state.busy = false;
            Object.keys(state.pendingAssets).forEach(function (r) { env.clearTimer(state.pendingAssets[r].timer); state.pendingAssets[r].reject(H.fail('cancelled', 'reset')); });
            state.pendingAssets = {};
            state.loaded = { faces: [], fontBytes: 0 };
            if (canvas.width) ctx.clearRect(0, 0, canvas.width, canvas.height);
        }
        function postTime(force) {
            var now = env.now();
            if (!force && now - state.lastTime < 90) return;
            state.lastTime = now;
            post({ type: 'time', frame: state.frame, playing: state.playing });
        }
        function draw(bitmap) {
            ctx.clearRect(0, 0, canvas.width, canvas.height);
            ctx.drawImage(bitmap, 0, 0);
            if (typeof bitmap.close === 'function') bitmap.close();
        }
        async function show(n, token) {
            var bmp = await state.engine.render(n);
            if (token !== state.loadToken) { if (bmp && bmp.close) bmp.close(); return false; }
            draw(bmp);
            state.shown = n;
            state.frame = n;
            return true;
        }

        async function onLoad(msg) {
            reset();
            var token = state.loadToken;
            try {
                state.loaded = { faces: facesOf(msg.catalog), fontBytes: (msg.fonts || []).reduce(function (sum, f) { return sum + f.data.byteLength; }, 0) };
                var engine = H.createEngine(env);
                state.engine = engine;
                var result = await engine.load(msg);
                if (token !== state.loadToken) return;
                var rep = result.report;
                state.report = rep;
                canvas.width = rep.width;
                canvas.height = rep.height;
                post({ type: 'compiled', report: rep });
                await result.prepare();
                if (token !== state.loadToken) return;
                if (rep.frame_count > 0 && await show(0, token)) postTime(true);
            } catch (err) {
                if (token === state.loadToken && !(err && err.code === 'cancelled')) report(err);
            }
        }

        function tick(ts) {
            state.rafHandle = undefined;
            if (!state.playing || !state.report) return;
            if (state.startTs === undefined) state.startTs = ts;
            var fps = state.report.fps, last = state.report.frame_count - 1;
            var target = state.startFrame + Math.floor((ts - state.startTs) / 1000 * fps), ended = false;
            if (target >= last) { target = last; ended = true; }
            if (!state.busy && target !== state.shown) {
                var token = state.loadToken;
                state.busy = true;
                show(target, token).then(function (ok) {
                    state.busy = false;
                    if (!ok) return;
                    if (ended) { state.playing = false; postTime(true); } else postTime(false);
                }, function (err) { state.busy = false; stopLoop(); report(err); postTime(true); });
            } else if (ended && !state.busy) { state.playing = false; postTime(true); }
            if (state.playing) state.rafHandle = env.raf(tick);
        }

        function onPlay() {
            if (!state.report || state.playing || !(state.report.frame_count > 0)) return;
            if (state.frame >= state.report.frame_count - 1) state.frame = 0;
            state.playing = true;
            state.startFrame = state.frame;
            state.startTs = undefined;
            state.shown = -1;
            postTime(true);
            state.rafHandle = env.raf(tick);
        }
        function onPause() {
            var was = state.playing;
            stopLoop();
            if (was) postTime(true);
        }
        async function onSeek(msg) {
            if (!state.report) return;
            var n = Math.min(msg.frame, Math.max(0, state.report.frame_count - 1)), token = state.loadToken;
            state.frame = n;
            if (state.playing) { state.startFrame = n; state.startTs = undefined; state.shown = -1; return; }
            try { if (await show(n, token)) postTime(true); } catch (err) { report(err); }
        }
        async function onThumb(msg) {
            var engine = state.engine, token = state.loadToken;
            if (!engine || !engine.schedule) { post({ type: 'error', code: 'readiness', message: 'thumb before load', scene_id: msg.scene_id }); return; }
            var sc = null;
            engine.schedule.scenes.forEach(function (s, i) { if (s.id === msg.scene_id) sc = { s: s, i: i }; });
            if (!sc) { post({ type: 'error', code: 'unknown_name', message: 'No scene "' + msg.scene_id + '"', scene_id: msg.scene_id }); return; }
            var len = sc.s.end_frame - sc.s.start_frame, tf = sc.s.tf || 0;
            var frame = msg.frame !== undefined ? msg.frame : sc.s.start_frame + Math.min(len - 1, tf + Math.floor(Math.max(0, len - tf) * 0.4));
            frame = Math.min(Math.max(0, frame), Math.max(0, engine.schedule.frame_count - 1));
            try {
                var bmp = await engine.render(frame, { width: msg.width });
                if (token !== state.loadToken) { if (bmp && bmp.close) bmp.close(); return; }
                if (typeof root.ImageBitmap === 'function' && !(bmp instanceof root.ImageBitmap) && typeof win.createImageBitmap === 'function') bmp = await win.createImageBitmap(bmp);
                post({ type: 'thumb', req: msg.req, scene_id: msg.scene_id, bitmap: bmp }, [bmp]);
            } catch (err) { report(err, msg.scene_id); }
        }
        function onAssets(msg) {
            var p = state.pendingAssets[msg.req];
            if (!p) return;
            delete state.pendingAssets[msg.req];
            env.clearTimer(p.timer);
            Object.keys(msg.assets).forEach(function (k) { if (k.indexOf('font:') === 0) state.loaded.fontBytes += msg.assets[k].byteLength; });
            p.resolve({ assets: msg.assets, missing: msg.missing || [] });
        }

        function onMessage(event) {
            if (disposed || event.source !== win.parent) return;
            var why = checkMessage(event.data, state.loaded);
            if (why) { post({ type: 'error', code: 'bad_message', message: why }); return; }
            var msg = event.data;
            switch (msg.type) {
                case 'load': onLoad(msg); break;
                case 'assets': onAssets(msg); break;
                case 'seek': onSeek(msg); break;
                case 'play': onPlay(); break;
                case 'pause': onPause(); break;
                case 'thumb': onThumb(msg); break;
                case 'reset': reset(); break;
            }
        }

        win.addEventListener('message', onMessage);
        post({ type: 'ready', caps: { workers: env.hasWorkers, offscreen: typeof root.OffscreenCanvas === 'function' } });
        return {
            state: state,
            dispose: function () { disposed = true; reset(); win.removeEventListener('message', onMessage); }
        };
    }

    H.CAPS = CAPS;
    H.checkMessage = checkMessage;
    H.mountApp = mountApp;
})(typeof globalThis !== 'undefined' ? globalThis : self);
