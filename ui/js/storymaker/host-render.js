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

    // sRGB channel (0..255) to linear light, built once.
    var LINEAR = new Float64Array(256);
    for (var li = 0; li < 256; li++) {
        var cs = li / 255;
        LINEAR[li] = cs <= 0.04045 ? cs / 12.92 : Math.pow((cs + 0.055) / 1.055, 2.4);
    }
    var PROBE_WIDTH = 64, PROBE_DEADLINE_MS = 20000;
    var UNIFORM_SPREAD = 0.01 * 255;   // every channel within 1 % of full scale of the frame mean

    // frameStats(px, prev) = {y, d, uniform} of one probed frame: px and prev are RGBA bytes of the same size (prev may be
    // null). y is the mean relative luminance (0.2126 R + 0.7152 G + 0.0722 B of the linearised channels), d the mean absolute
    // difference per colour channel to prev (0..1, 0 without prev), uniform whether every pixel is within 1 % of the mean.
    function frameStats(px, prev) {
        var n = px.length >> 2, i, r = 0, g = 0, b = 0, lum = 0, diff = 0;
        for (i = 0; i < px.length; i += 4) {
            r += px[i]; g += px[i + 1]; b += px[i + 2];
            lum += 0.2126 * LINEAR[px[i]] + 0.7152 * LINEAR[px[i + 1]] + 0.0722 * LINEAR[px[i + 2]];
            if (prev) diff += Math.abs(px[i] - prev[i]) + Math.abs(px[i + 1] - prev[i + 1]) + Math.abs(px[i + 2] - prev[i + 2]);
        }
        var mr = r / n, mg = g / n, mb = b / n, uniform = true;
        for (i = 0; i < px.length && uniform; i += 4) {
            if (Math.abs(px[i] - mr) > UNIFORM_SPREAD || Math.abs(px[i + 1] - mg) > UNIFORM_SPREAD || Math.abs(px[i + 2] - mb) > UNIFORM_SPREAD) uniform = false;
        }
        return { y: n ? lum / n : 0, d: prev && n ? diff / (n * 3 * 255) : 0, uniform: uniform };
    }

    // runProbe(o) = the frame probe of the compile check: o = {engine, schedule, width, height, makeCanvas(w, h), now(), deadlineMs}.
    // Every frame of the film is rendered in order at PROBE_WIDTH and reduced to y and d (frameStats), until the deadline or
    // the first frame that fails; the frames it did not reach are simply missing from y and d, and checks.js reports that.
    // uniform[i] holds the flat-frame test of the first frame after the entry transition of code scene i, when it was reached.
    async function runProbe(o) {
        var sched = o.schedule, total = sched.frame_count, pw = PROBE_WIDTH, ph = Math.max(1, Math.round(pw * o.height / o.width));
        var canvas = o.makeCanvas(pw, ph), cx = canvas.getContext('2d', { willReadFrequently: true });
        var settled = {}, y = [], d = [], uniform = {}, prev = null, started = o.now();
        (sched.scenes || []).forEach(function (sc, i) {
            if (sc.kind === 'code') settled[root.StorymakerCore.schedule.settledFrame(sched, i)] = i;
        });
        for (var n = 0; n < total; n++) {
            if (o.now() - started >= o.deadlineMs) break;
            try {
                var bmp = await o.engine.render(n, { width: pw });
                cx.clearRect(0, 0, pw, ph);
                cx.drawImage(bmp, 0, 0, pw, ph);
                if (typeof bmp.close === 'function') bmp.close();
            } catch (e) { break; }
            var px = cx.getImageData(0, 0, pw, ph).data, st = frameStats(px, prev);
            y.push(st.y);
            d.push(st.d);
            if (settled[n] !== undefined) uniform[settled[n]] = st.uniform;
            prev = px;
        }
        return { fps: sched.fps, frame_count: total, y: y, d: d, uniform: uniform };
    }

    // mountRender(opts): opts = {canvas, base, fetch?, window?, env?}. Resolves when the job is over (done, stopped,
    // refused, failed) or, in screenshot mode, when the page is ready for __smRenderFrame calls.
    async function mountRender(opts) {
        var win = opts.window || root, canvas = opts.canvas, base = String(opts.base).replace(/\/+$/, '');
        var doFetch = opts.fetch || function (url, init) { return win.fetch(url, init); };
        var env = H.defaultEnv(base, opts.env || {});
        var status = win.__smStatus = { phase: 'boot', frame: -1 };
        var engine = null, helpers = [];

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
            // DevTools cannot account for Blob-backed POST sizes; use bytes so its payload limits can evict frames.
            if (path.indexOf('/frame?') === 0 && body && typeof body.arrayBuffer === 'function') body = await body.arrayBuffer();
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
            // The first frames and, with the raster catalog, every paint wait for warmupDeadlineMs: it must not undercut the offered deadline.
            if (boot.frame_deadline_ms > 0) {
                env.limits.frameDeadlineMs = Math.min(60000, boot.frame_deadline_ms);
                env.limits.warmupDeadlineMs = Math.max(env.limits.warmupDeadlineMs || 0, env.limits.frameDeadlineMs);
            }
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
            if (catalog && catalog.icons && catalog.icons.file) {
                var iconFile = String(catalog.icons.file);
                if (iconFile.indexOf('..') >= 0 || iconFile.charAt(0) === '/') throw H.fail('readiness', 'Bad asset path ' + iconFile);
                catalog.icon_data = await getJSON('icons', assetsBase + iconFile);
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
                    } else if (/^(texture|model):/.test(key)) {
                        var imagePath = key.slice(key.indexOf(':') + 1);
                        var image = (catalog.files || []).find(function (f) { return f.path === imagePath && (key.indexOf('model:') === 0 ? f.type === 'model/gltf-binary' : ['image/png','image/webp','image/jpeg'].indexOf(f.type) >= 0); });
                        if (!image || !/^[A-Za-z0-9_./-]+$/.test(imagePath) || imagePath.split('/').some(function (s) { return !s || s === '.' || s === '..'; }) || image.bytes < 1 || image.bytes > 8 * 1048576) throw H.fail('readiness', 'Invalid texture dependency');
                        var imageResponse = await doFetch(assetsBase + imagePath, { cache: 'no-store' });
                        if (!imageResponse.ok) throw H.fail('readiness', 'Texture could not be fetched');
                        var imageReader = imageResponse.body.getReader(), imageParts = [], imageLength = 0;
                        try {
                            for (;;) {
                                var nextImagePart = await imageReader.read();
                                if (nextImagePart.done) break;
                                imageLength += nextImagePart.value.byteLength;
                                if (imageLength > image.bytes) throw H.fail('readiness', 'Texture exceeds its size limit');
                                imageParts.push(nextImagePart.value);
                            }
                        } catch (imageError) { await imageReader.cancel().catch(function () {}); throw imageError; }
                        finally { imageReader.releaseLock(); }
                        var combinedImage = new Uint8Array(imageLength), imageOffset = 0;
                        imageParts.forEach(function (part) { combinedImage.set(part,imageOffset); imageOffset += part.byteLength; });
                        var imageBytes = combinedImage.buffer;
                        if (imageBytes.byteLength !== image.bytes) throw H.fail('readiness', 'Texture size differs');
                        assets[key] = imageBytes;
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
            if (boot.layout === true) {
                // The compile check wants the geometry of every scene. A failure costs the geometry, never the report.
                try {
                    var measured = await engine.send({ type: 'layout' }, env.limits.layoutDeadlineMs > 0 ? env.limits.layoutDeadlineMs : 15000);
                    rep.layout = measured.layout || [];
                } catch (layoutErr) {
                    rep.layout_error = String(layoutErr && layoutErr.message || layoutErr).slice(0, 200);
                }
            }
            if (boot.layout === true) {
                // The compile check also probes every frame for flashing and frozen code scenes (checks.js). Like the layout,
                // a failure here costs the findings, never the report.
                try {
                    var probe = await runProbe({
                        engine: engine, schedule: engine.schedule, width: boot.width, height: boot.height, deadlineMs: PROBE_DEADLINE_MS,
                        makeCanvas: env.makeCanvas, now: env.now
                    });
                    var judged = await engine.send({ type: 'checks', probe: probe }, 15000);
                    rep.checks = (rep.checks || []).concat(judged.checks || []);
                } catch (probeErr) {
                    rep.probe_error = String(probeErr && probeErr.message || probeErr).slice(0, 200);
                }
            }
            // Parallel path: only PNG is encoded in the worker, and a main engine in the document-realm fallback stays serial.
            // The helpers start before the report, so their start-up cannot trip the 10 s frame watchdog of Go.
            var parallel = boot.transport === 'png' && boot.workers > 1 && boot.chunk > 0 && engine.mode === 'workers';
            if (parallel) {
                for (var hi = 1; hi < boot.workers; hi++) helpers.push(H.createEngine(env));
                await Promise.all(helpers.map(async function (helper) {
                    var made = await helper.load({
                        story: story, catalog: catalog, audio_info: boot.audio_info || {}, assets: {}, fonts: [], code_bundles: {},
                        format: { fps: boot.fps, width: boot.width, height: boot.height }, shared_code_scenes: engine.codeScenesInfo
                    });
                    if (helper.mode !== 'workers') throw H.fail('render_failed', 'A helper worker could not start.');
                    if (made.report.frame_count !== rep.frame_count || JSON.stringify(helper.schedule) !== JSON.stringify(engine.schedule)) {
                        throw H.fail('render_failed', 'A helper worker compiled ' + made.report.frame_count + ' frames, not ' + rep.frame_count + '.');
                    }
                    await made.prepare();
                }));
            }
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
            if (parallel) {
                var outcome = await H.runPool({
                    engines: [engine].concat(helpers), limit: limit, chunk: boot.chunk, preRoll: boot.pre_roll > 0 ? boot.pre_roll : 0, status: status,
                    post: function (f, blob) { return post('/frame?n=' + f, blob, { 'Content-Type': 'text/plain' }); }
                });
                helpers.forEach(function (helper) { helper.dispose(); });
                status.phase = outcome.phase;
                if (outcome.phase === 'stopped') { status.error = outcome.error; status.message = outcome.message; }
                return status;
            }
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
            helpers.forEach(function (helper) { helper.dispose(); });
            status.phase = 'error';
            status.error = H.asError(err).code;
            status.message = H.asError(err).message;
            await postError(err);
            return status;
        }
    }

    H.mountRender = mountRender;
    H.frameStats = frameStats;
    H.runProbe = runProbe;
})(typeof globalThis !== 'undefined' ? globalThis : self);
