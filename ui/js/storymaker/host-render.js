// Storymaker host (document realm): render mode, the page that headless Chrome opens for an export. It drives itself from
// <base>/boot.json (contract section 7): fetches story, catalog, fonts (only from boot.fonts) and code-scene bundles from the
// base, compiles, POSTs the compile report, waits for {start: true}, then renders frames in order and POSTs each one to the
// fixed <base>/frame?n=<frame> as a CORS-simple request (Content-Type text/plain, no custom header, so Chrome sends no
// preflight per frame). Transports: png, webp, raw RGBA, or screenshot (the page only draws on
// window.__smRenderFrame(n); Go captures). Answers: {ok}, {ok, done}, {restart}, {stop}. Failures are POSTed to <base>/error.
// window.__smStatus mirrors the phase for the driver. Classic script; publishes StorymakerHost.mountRender.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    var TRANSPORTS = ['png', 'webp', 'raw', 'screenshot'];
    var MIME = { png: 'image/png', webp: 'image/webp', raw: 'application/octet-stream' };
    var MAX_FONT_BYTES = 8 * 1048576;   // boot.fonts[].max_bytes names the one exception

    function errorBody(err, sceneId) {
        var e = H.asError(err), out = { code: e.code, message: String(e.message).slice(0, 600) };
        var sid = sceneId || e.scene_id;
        if (sid) out.scene_id = sid;
        return out;
    }

    // mountRender(opts): opts = {canvas, base, fetch?, window?, env?}. Resolves when the job is over (done, stopped,
    // refused, failed) or, in screenshot mode, when the page is ready for __smRenderFrame calls.
    async function mountRender(opts) {
        var win = opts.window || root, canvas = opts.canvas, base = String(opts.base).replace(/\/+$/, '');
        var doFetch = opts.fetch || function (url, init) { return win.fetch(url, init); };
        var env = H.defaultEnv(base, opts.env || {});
        var status = win.__smStatus = { phase: 'boot', frame: -1 };
        var engine = null;

        // The token of the first boot.json answer stays here, and post() and waitForStart() read it. A failure text names a label or a route and
        // never a URL, because a URL holds the secret prefix and this text reaches __smStatus, POST /error and the job record.
        var token = '';

        async function getJSON(label, url) {
            var res = await doFetch(url, { cache: 'no-store' });
            if (!res.ok) throw H.fail('readiness', 'GET ' + label + ' answered ' + res.status);
            return res.json();
        }
        async function post(path, body, headers) {
            var url = base + path;
            if (token) url += (path.indexOf('?') >= 0 ? '&' : '?') + 't=' + encodeURIComponent(token);
            var res = await doFetch(url, { method: 'POST', headers: headers, body: body, cache: 'no-store' });
            var text = '';
            try { text = await res.text(); } catch (e) { text = ''; }
            var json = null;
            try { json = text ? JSON.parse(text) : {}; } catch (e) { json = null; }
            if (!res.ok && (!json || !(json.stop || json.restart))) throw H.fail('render_failed', 'POST ' + path.split('?')[0] + ' answered ' + res.status + (text ? ': ' + text.slice(0, 200) : ''));
            return json || {};
        }
        // The deferred start: Go holds each GET at most 20 s and answers pending, start or an error, so the page
        // needs no timer of its own. A failure text names the route only, never the token.
        async function waitForStart() {
            for (;;) {
                var res = await doFetch(base + '/start' + (token ? '?t=' + token : ''), { cache: 'no-store' });
                if (!res.ok) throw H.fail('render_failed', 'GET /start answered ' + res.status);
                var reply = await res.json();
                if (!reply.pending) return reply;
            }
        }
        async function postError(err, sceneId) {
            try { await post('/error', JSON.stringify(errorBody(err, sceneId)), { 'Content-Type': 'application/json' }); } catch (e) { /* the listener is gone */ }
        }

        try {
            // The render page has no stylesheet (its CSP has no style-src): layout goes through the CSSOM, which needs none. The
            // canvas must fill the viewport exactly, or the screenshot transport would capture scrollbars.
            if (win.document && win.document.body && win.document.body.style) win.document.body.style.margin = '0';
            if (win.document && win.document.documentElement && win.document.documentElement.style) win.document.documentElement.style.overflow = 'hidden';
            if (canvas.style) canvas.style.display = 'block';
            var boot = await getJSON('boot.json', base + '/boot.json');
            if (!boot || boot.mode !== 'render' || !(boot.fps > 0) || !(boot.width > 0) || !(boot.height > 0) || TRANSPORTS.indexOf(boot.transport) < 0) {
                throw H.fail('readiness', 'boot.json is not a render boot document.');
            }
            // The stub servers of the tests send no token, and then the page sends no t.
            token = typeof boot.token === 'string' ? boot.token : '';
            canvas.width = boot.width;
            canvas.height = boot.height;
            // A render is not real time: Go offers a per-frame deadline far above the 250 ms of the app preview.
            if (boot.frame_deadline_ms > 0) env.limits.frameDeadlineMs = Math.min(60000, boot.frame_deadline_ms);
            var ctx =canvas.getContext('2d', { willReadFrequently: boot.transport === 'raw' });
            var story = await getJSON('story', boot.story_url);
            var catalog = await getJSON('catalog', boot.catalog_url);
            var bundles = {}, ids = Object.keys(boot.code_bundles || {});
            await Promise.all(ids.map(async function (id) {
                var res = await doFetch(boot.code_bundles[id], { cache: 'no-store' });
                if (!res.ok) throw H.fail('code_error', 'Code scene "' + id + '" could not be fetched (' + res.status + ').', { scene_id: id });
                bundles[id] = await res.text();
            }));
            var assetsBase = String(boot.assets_base || (base + '/catalog/')).replace(/\/*$/, '/');
            if (catalog && catalog.glyphs && catalog.glyphs.file) {
                var glyphFile = String(catalog.glyphs.file);
                if (glyphFile.indexOf('..') >= 0 || glyphFile.charAt(0) === '/') throw H.fail('readiness', 'Bad asset path ' + glyphFile);
                catalog.glyph_data = await getJSON('glyphs', assetsBase + glyphFile);
            }
            // Fonts are lazy: load.fonts stays empty and requestAssets fetches a face when compile needs it. URLs come from boot.fonts
            // only (the catalog's file paths are AuraGo paths), and each buffer is fetched once.
            var fontByKey = {}, fontFetches = {};
            (boot.fonts || []).forEach(function (f) { fontByKey['font:' + f.name + '/' + f.weight] = f; });
            function fetchFont(key) {
                if (!fontFetches[key]) {
                    fontFetches[key] = (async function () {
                        var f = fontByKey[key], res = await doFetch(f.url, { cache: 'no-store' });
                        if (!res.ok) throw H.fail('readiness', 'Font "' + f.family + '" could not be fetched (' + res.status + ').');
                        var data = await res.arrayBuffer(), limit = f.max_bytes > 0 ? f.max_bytes : MAX_FONT_BYTES;
                        if (data.byteLength > limit) throw H.fail('readiness', 'Font "' + f.family + '" has ' + data.byteLength + ' bytes, over its limit of ' + limit + '.');
                        return data;
                    })();
                }
                return fontFetches[key];
            }

            env.onError = function (e) { status.lastError = e; postError(e, e.scene_id); };
            env.requestAssets = async function (keys) {
                var assets = {}, missing = [];
                await Promise.all(keys.map(async function (key) {
                    if (key.indexOf('catalog:') === 0) {
                        var rel = key.slice(8);
                        if (rel.indexOf('..') >= 0 || rel.charAt(0) === '/') throw H.fail('readiness', 'Bad asset path ' + rel);
                        var res = await doFetch(assetsBase + rel, { cache: 'no-store' });
                        if (!res.ok) throw H.fail('readiness', 'Asset ' + rel + ' answered ' + res.status);
                        assets[key] = await res.json();
                    } else if (key.indexOf('font:') === 0) {
                        if (fontByKey[key]) assets[key] = await fetchFont(key); else missing.push(key);
                    }
                }));
                return { assets: assets, missing: missing };
            };

            engine = H.createEngine(env);
            status.phase = 'compiling';
            var loaded = await engine.load({
                story: story, catalog: catalog, audio_info: boot.audio_info || {}, assets: {}, fonts: [], code_bundles: bundles,
                format: { fps: boot.fps, width: boot.width, height: boot.height }
            });
            await loaded.prepare();
            var rep = loaded.report;
            status.phase = 'reporting';
            var answer = await post('/report', JSON.stringify(rep), { 'Content-Type': 'application/json' });
            // Go answers at once and mixes in the background. A stand-in may still answer {start:true}.
            if (answer.pending && !answer.start && !answer.error) {
                status.phase = 'waiting';
                answer = await waitForStart();
            }
            if (!answer.start) {
                status.phase = 'refused';
                status.error = answer.error || '';
                status.message = answer.message || '';
                return status;
            }
            var limit = Math.min(rep.frame_count, boot.frame_limit > 0 ? boot.frame_limit : rep.frame_count);

            async function drawFrame(n) {
                var bmp = await engine.render(n);
                ctx.clearRect(0, 0, canvas.width, canvas.height);
                ctx.drawImage(bmp, 0, 0);
                if (typeof bmp.close === 'function') bmp.close();
                status.frame = n;
            }
            function encode(transport) {
                if (transport === 'raw') {
                    var data = ctx.getImageData(0, 0, canvas.width, canvas.height).data;
                    return Promise.resolve(data.buffer);
                }
                return new Promise(function (resolve, reject) {
                    canvas.toBlob(function (blob) { if (blob) resolve(blob); else reject(H.fail('render_failed', 'The canvas could not be encoded.')); }, MIME[transport], transport === 'webp' ? 1 : undefined);
                });
            }

            if (boot.transport === 'screenshot') {
                win.__smRenderFrame = async function (n) {
                    try {
                        if (!(n >= 0 && n < limit)) throw H.fail('render_failed', 'Frame ' + n + ' is outside 0..' + (limit - 1));
                        await drawFrame(n);
                    } catch (failure) {
                        var e = H.asError(failure), real = new Error(e.message);   // a real Error, so the driver sees the message
                        real.code = e.code;
                        postError(e);
                        throw real;
                    }
                    await new Promise(function (resolve) {
                        var done = false, finish = function () { if (!done) { done = true; resolve(); } };
                        if (typeof win.requestAnimationFrame === 'function') win.requestAnimationFrame(function () { win.requestAnimationFrame(finish); });
                        env.setTimer(finish, 150);
                    });
                    return n;
                };
                status.phase = 'ready';
                status.frame_count = limit;
                return status;
            }

            status.phase = 'rendering';
            var n = 0;
            for (;;) {
                if (n >= limit) {
                    await postError(H.fail('render_failed', 'The page ran past the last frame (' + limit + ') without a done answer.'));
                    status.phase = 'error';
                    return status;
                }
                await drawFrame(n);
                var body = await encode(boot.transport);
                var reply = await post('/frame?n=' + n, body, { 'Content-Type': 'text/plain' });
                if (reply.stop) { status.phase = 'stopped'; status.error = reply.error || ''; status.message = reply.message || ''; return status; }
                if (reply.restart) { n = 0; status.restarts = (status.restarts || 0) + 1; continue; }
                if (reply.done) { status.phase = 'done'; return status; }
                n++;
            }
        } catch (err) {
            if (engine) engine.dispose();
            status.phase = 'error';
            status.error = H.asError(err).code;
            status.message = H.asError(err).message;
            await postError(err);
            return status;
        }
    }

    H.mountRender = mountRender;
})(typeof globalThis !== 'undefined' ? globalThis : self);
