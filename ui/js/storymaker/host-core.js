// Storymaker host (document realm): the engine. It owns the player worker and one worker per code scene, routes
// messages with per-call deadlines (a worker that overruns is terminated), runs the load flow (plan -> build code scenes
// -> compile -> assets) and renders frames. When workers or OffscreenCanvas are unavailable it runs the trusted player
// runtime in this realm (storyboard fallback; code scenes show a placeholder). The app protocol (host-app.js) and the render
// driver (host-render.js) both sit on this engine. Classic script; publishes StorymakerHost.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    var LIMITS = {
        compileDeadlineMs: 10000, sceneBuildDeadlineMs: 2000, frameDeadlineMs: 250, warmupFrames: 3, warmupDeadlineMs: 4000,
        helloDeadlineMs: 5000, assetsDeadlineMs: 20000
    };

    function fail(code, message, extra) {
        var e = { code: code, message: message };
        if (extra) for (var k in extra) e[k] = extra[k];
        return e;
    }
    function asError(err, code) {
        if (err && typeof err.code === 'string') return err;
        return fail(code || 'code_error', String(err && err.message || err));
    }

    // ---- worker channel ------------------------------------------------------------------------------------------
    // A channel wraps one worker: call() sends a message and resolves with the reply; a call that takes longer than its
    // deadline terminates the worker and rejects with code "deadline".
    function Channel(worker, env, label) {
        var self = this;
        this.worker = worker;
        this.env = env;
        this.label = label;
        this.dead = false;
        this.pending = {};
        this.nextReq = 1;
        this.helloSeen = false;
        this.helloWaiters = [];
        this.onDeath = null;
        function onMessage(event) {
            var d = event && event.data;
            if (!d || typeof d !== 'object') return;
            if (d.type === 'hello') {
                self.helloSeen = true;
                self.helloWaiters.splice(0).forEach(function (w) { w.resolve(); });
                return;
            }
            if (d.type !== 'reply') return;
            var p = self.pending[d.req];
            if (!p) return;
            delete self.pending[d.req];
            env.clearTimer(p.timer);
            if (d.ok) p.resolve(d); else p.reject(asError(d.error));
        }
        function onError(event) {
            var msg = event && event.message ? event.message : 'The worker failed to start.';
            if (event && event.preventDefault) event.preventDefault();
            self.kill(fail('code_error', msg));
        }
        if (typeof worker.addEventListener === 'function') {
            worker.addEventListener('message', onMessage);
            worker.addEventListener('error', onError);
        } else {
            worker.onmessage = onMessage;
            worker.onerror = onError;
        }
    }

    Channel.prototype.hello = function (ms) {
        var self = this;
        if (this.helloSeen) return Promise.resolve();
        return new Promise(function (resolve, reject) {
            var timer = self.env.setTimer(function () { reject(fail('deadline', self.label + ' did not start within ' + ms + ' ms.')); }, ms);
            self.helloWaiters.push({
                resolve: function () { self.env.clearTimer(timer); resolve(); },
                reject: function (e) { self.env.clearTimer(timer); reject(e); }
            });
        });
    };

    Channel.prototype.call = function (msg, ms, transfer) {
        var self = this;
        return new Promise(function (resolve, reject) {
            if (self.dead) { reject(fail('code_error', self.label + ' is not running.')); return; }
            var req = self.nextReq++;
            msg.req = req;
            var timer = self.env.setTimer(function () {
                delete self.pending[req];
                var e = fail('deadline', self.label + ' took longer than ' + ms + ' ms for "' + msg.type + '" and was stopped.');
                self.kill(e);
                reject(e);
            }, ms);
            self.pending[req] = { resolve: resolve, reject: reject, timer: timer };
            try { self.worker.postMessage(msg, transfer || []); } catch (err) {
                delete self.pending[req];
                self.env.clearTimer(timer);
                reject(fail('code_error', 'Could not send to ' + self.label + ': ' + (err && err.message || err)));
            }
        });
    };

    Channel.prototype.kill = function (reason) {
        if (this.dead) return;
        this.dead = true;
        try { this.worker.terminate(); } catch (e) { /* already gone */ }
        var pending = this.pending, env = this.env;
        this.pending = {};
        Object.keys(pending).forEach(function (k) { env.clearTimer(pending[k].timer); pending[k].reject(reason || fail('code_error', 'worker stopped')); });
        this.helloWaiters.splice(0).forEach(function (w) { w.reject(reason || fail('code_error', 'worker stopped')); });
        if (this.onDeath) this.onDeath(reason);
    };

    // ---- default environment ---------------------------------------------------------------------------------------
    // blobWorker(base, role, sceneText): one blob per worker = prelude + (scene bundle text). The prelude is exactly one
    // line, so a line number inside the blob minus one is a line of the scene bundle.
    function blobWorker(base, role, sceneText) {
        var prelude = 'self.__SM_ROLE__ = ' + JSON.stringify(role) + '; importScripts(' + JSON.stringify(base + '/js/storymaker/player.bundle.js') + ');\n';
        var url = root.URL.createObjectURL(new root.Blob([prelude + (sceneText || '')], { type: 'text/javascript' }));
        var worker = new root.Worker(url);
        var revoke = function () { try { root.URL.revokeObjectURL(url); } catch (e) { /* ignore */ } };
        if (worker.addEventListener) worker.addEventListener('message', function once() { revoke(); worker.removeEventListener('message', once); });
        return worker;
    }

    function loadCoreScript(base) {
        return new Promise(function (resolve, reject) {
            if (root.StorymakerCore && root.StorymakerCore.workerPlayer) { resolve(root.StorymakerCore); return; }
            var s = root.document.createElement('script');
            s.src = base + '/js/storymaker/player.bundle.js';
            s.onload = function () { resolve(root.StorymakerCore); };
            s.onerror = function () { reject(fail('readiness', 'The player bundle could not be loaded.')); };
            root.document.head.appendChild(s);
        });
    }

    // defaultEnv(base, overrides)
    function defaultEnv(base, over) {
        over = over || {};
        var hasDom = typeof root.document !== 'undefined';
        var env = {
            base: base,
            hasWorkers: typeof root.Worker === 'function' && typeof root.OffscreenCanvas === 'function',
            createWorker: function (role, text) { return blobWorker(base, role, text); },
            loadCore: function () { return loadCoreScript(base); },
            makeCanvas: function (w, h) { var c = root.document.createElement('canvas'); c.width = w; c.height = h; return c; },
            FontFace: root.FontFace,
            fonts: hasDom ? root.document.fonts : null,
            setTimer: function (fn, ms) { return root.setTimeout(fn, ms); },
            clearTimer: function (h) { root.clearTimeout(h); },
            now: function () { return root.performance.now(); },
            requestAssets: function () { return Promise.reject(fail('readiness', 'No asset source is configured.')); },
            onError: function () {},
            limits: LIMITS
        };
        for (var k in over) env[k] = over[k];
        env.limits = Object.assign({}, LIMITS, over.limits || {});
        return env;
    }

    // ---- engine -----------------------------------------------------------------------------------------------------
    function Engine(env) {
        this.env = env;
        this.mode = 'idle';
        this.player = null;
        this.runtime = null;
        this.core = null;
        this.sceneChannels = {};
        this.codeErrors = {};
        this.schedule = null;
        this.report = null;
        this.rendered = 0;
        this.chain = Promise.resolve();
        this.dead = false;
        this.loadId = 0;
    }

    Engine.prototype.dispose = function () {
        this.loadId++;
        var self = this;
        if (this.player) { this.player.kill(fail('cancelled', 'disposed')); this.player = null; }
        Object.keys(this.sceneChannels).forEach(function (id) { self.sceneChannels[id].kill(fail('cancelled', 'disposed')); });
        this.sceneChannels = {};
        this.runtime = null;
        this.schedule = null;
        this.mode = 'idle';
        this.chain = Promise.resolve();
    };

    Engine.prototype.startPlayer = async function () {
        var env = this.env;
        if (env.hasWorkers) {
            var channel = null;
            try {
                channel = new Channel(env.createWorker('player', ''), env, 'The player worker');
                await channel.hello(env.limits.helloDeadlineMs);
                this.player = channel;
                this.mode = 'workers';
                return;
            } catch (err) {
                if (channel) channel.kill(err);
            }
        }
        // storyboard fallback in this realm
        this.core = await env.loadCore();
        if (!this.core || !this.core.workerPlayer) throw fail('readiness', 'The player runtime is not available in this browser.');
        this.runtime = new this.core.workerPlayer.Runtime({ makeCanvas: env.makeCanvas, FontFace: env.FontFace, fonts: env.fonts });
        this.mode = 'fallback';
    };

    // send(msg, ms, transfer): to the player (worker or in-realm runtime).
    Engine.prototype.send = async function (msg, ms, transfer) {
        if (this.mode === 'workers') return this.player.call(msg, ms, transfer);
        var res = await this.runtime.handle(msg), out = { ok: true };
        for (var k in res.payload) out[k] = res.payload[k];
        return out;
    };

    Engine.prototype.buildScene = async function (id, p) {
        var env = this.env, bundle = p.msg.code_bundles && p.msg.code_bundles[id];
        if (typeof bundle !== 'string' || !bundle) return { error: fail('code_error', 'No bundle for code scene "' + id + '". Save the scene again to rebuild it.') };
        var channel;
        try {
            channel = new Channel(env.createWorker('scene', bundle), env, 'Code scene "' + id + '"');
            var opts = {
                sceneId: id, index: p.plan.scenes[id].index, catalog: p.msg.catalog, aspect: p.plan.aspect, width: p.plan.width, height: p.plan.height,
                fps: p.plan.fps, cues: p.plan.scenes[id].cues, palette: p.plan.palette, families: p.plan.fontFamilies, styleFont: p.plan.styleFont,
                safe: p.msg.catalog && p.msg.catalog.stage && p.msg.catalog.stage.safe || null,
                language: p.plan.language, fontChain: p.plan.fontChain
            };
            var started = env.now();
            await channel.hello(env.limits.sceneBuildDeadlineMs);
            var left = Math.max(50, env.limits.sceneBuildDeadlineMs - (env.now() - started));
            var reply = await channel.call({ type: 'build', opts: opts }, left);
            this.sceneChannels[id] = channel;
            return { duration: reply.duration, explicit: reply.explicit === true, needs: reply.needs || [], issues: reply.issues || [] };
        } catch (err) {
            if (channel) channel.kill(err);
            var e = asError(err);
            if (typeof e.line === 'number' && e.line > 1) e.line -= 1;   // the prelude is one line
            return { error: e };
        }
    };

    // load(msg): msg = {story, catalog, audio_info, assets, fonts, code_bundles, format}. Resolves with the compile report
    // as soon as it exists and prepare(), which loads the assets (the readiness barrier before frame 0).
    Engine.prototype.load = async function (msg) {
        this.dispose();
        var env = this.env, id = ++this.loadId, self = this;
        this.dead = false;
        this.codeErrors = {};
        this.rendered = 0;
        await this.startPlayer();
        if (id !== this.loadId) throw fail('cancelled', 'superseded');
        var fmt = msg.format || {};
        var sizeOpts = { width: fmt.width > 0 ? fmt.width : 0, height: fmt.height > 0 ? fmt.height : 0 };
        var story = Object.assign({}, msg.story);
        if (fmt.aspect || fmt.fps) story.format = Object.assign({}, story.format || {}, fmt.aspect ? { aspect: fmt.aspect } : {}, fmt.fps ? { fps: fmt.fps } : {});

        var planReply = await this.send({ type: 'plan', story: story, catalog: msg.catalog, audioInfo: msg.audio_info || {}, opts: sizeOpts }, env.limits.compileDeadlineMs);
        var plan = planReply.plan;
        var codeScenes = {}, sceneIssues = [], sceneNeeds = {}, codeIds = Object.keys(plan.scenes).filter(function (k) { return plan.scenes[k].kind === 'code'; });
        await Promise.all(codeIds.map(async function (cid) {
            if (self.mode === 'fallback') {
                codeScenes[cid] = {};
                self.codeErrors[cid] = 'Code scenes are not previewed in this browser. Export renders them.';
                return;
            }
            var built = await self.buildScene(cid, { msg: msg, plan: plan });
            if (built.error) {
                codeScenes[cid] = { error: built.error };
                self.codeErrors[cid] = built.error.message;
                env.onError({ code: built.error.code, message: built.error.message, scene_id: cid, line: built.error.line, column: built.error.column });
            } else {
                codeScenes[cid] = { duration: built.duration, explicit: built.explicit };
                sceneNeeds[cid] = built.needs;
                built.issues.forEach(function (i) { sceneIssues.push(i); });
            }
        }));
        if (id !== this.loadId) throw fail('cancelled', 'superseded');

        var compiled = await this.send({ type: 'compile', story: story, catalog: msg.catalog, audioInfo: msg.audio_info || {}, opts: Object.assign({ codeScenes: codeScenes }, sizeOpts) }, env.limits.compileDeadlineMs);
        var report = compiled.report;
        sceneIssues.forEach(function (i) { report.issues.push(i); });
        this.report = report;
        this.schedule = compiled.schedule;
        var prepared = null;
        // prepare() starts the asset phase (once) and resolves when every asset is loaded: the readiness barrier before frame 0
        return {
            report: report,
            prepare: function () {
                if (!prepared) prepared = self.ensureAssets(msg, compiled.needs || [], sceneNeeds, id);
                return prepared;
            }
        };
    };

    Engine.prototype.fontDescriptor = function (catalog, key, data) {
        var m = /^font:(.+)\/(\d+)$/.exec(key), name = m ? m[1] : key, weight = m ? parseInt(m[2], 10) : 400, family = name;
        ((catalog && catalog.fonts) || []).forEach(function (f) { if (f.name === name && f.weight === weight) family = f.family; });
        var out = { name: name, family: family, weight: weight, data: data };
        var faces = catalog && catalog.glyph_data && catalog.glyph_data.faces;
        (Array.isArray(faces) ? faces : []).forEach(function (f) {
            if (!f || f.id !== name) return;
            out.family = f.family;
            out.weight_range = f.weight;
            out.unicode_range = f.unicode_range;
        });
        return out;
    };

    // ensureAssets: gather what the player and the code scenes need from msg.assets / msg.fonts / the asset source.
    Engine.prototype.ensureAssets = async function (msg, needs, sceneNeeds, id) {
        var env = this.env, self = this, have = Object.assign({}, msg.assets || {});
        var fontsHave = {};
        (msg.fonts || []).forEach(function (f) { fontsHave['font:' + f.name + '/' + f.weight] = f; });
        var all = {};
        needs.forEach(function (k) { all[k] = true; });
        Object.keys(sceneNeeds).forEach(function (cid) { sceneNeeds[cid].forEach(function (k) { all[k] = true; }); });
        var wanted = Object.keys(all), missing = wanted.filter(function (k) { return k.indexOf('font:') === 0 ? !fontsHave[k] : have[k] === undefined; });
        if (missing.length) {
            var got = await env.requestAssets(missing);
            var assets = got && got.assets || {};
            Object.keys(assets).forEach(function (k) {
                if (k.indexOf('font:') === 0) fontsHave[k] = self.fontDescriptor(msg.catalog, k, assets[k]); else have[k] = assets[k];
            });
            var still = missing.filter(function (k) { return k.indexOf('font:') === 0 ? !fontsHave[k] : have[k] === undefined; });
            if (still.length) throw fail('readiness', 'Missing assets: ' + still.join(', '));
        }
        if (id !== this.loadId) throw fail('cancelled', 'superseded');
        var fonts = Object.keys(fontsHave).filter(function (k) { return all[k]; }).map(function (k) { return fontsHave[k]; });
        var catalogAssets = {};
        wanted.forEach(function (k) { if (k.indexOf('catalog:') === 0) catalogAssets[k] = have[k]; });
        var reply = await this.send({ type: 'assets', assets: catalogAssets, fonts: fonts }, env.limits.assetsDeadlineMs);
        if (reply.missing && reply.missing.length) throw fail('readiness', 'Missing assets: ' + reply.missing.join(', '));
        await Promise.all(Object.keys(sceneNeeds).map(function (cid) {
            var channel = self.sceneChannels[cid];
            if (!channel || channel.dead) return null;
            var sceneAssets = {};
            sceneNeeds[cid].forEach(function (k) { if (k.indexOf('catalog:') === 0) sceneAssets[k] = have[k]; });
            var sceneFonts = sceneNeeds[cid].filter(function (k) { return k.indexOf('font:') === 0 && fontsHave[k]; }).map(function (k) { return fontsHave[k]; });
            return channel.call({ type: 'assets', assets: sceneAssets, fonts: sceneFonts }, env.limits.assetsDeadlineMs).catch(function (err) {
                var e = asError(err, 'readiness');
                self.codeErrors[cid] = e.message;
                env.onError({ code: e.code, message: e.message, scene_id: cid });
            });
        }));
    };

    // render(n, opts): the frame as an ImageBitmap (a canvas in the fallback). Calls are serialised.
    Engine.prototype.render = function (n, opts) {
        var self = this;
        var run = function () { return self.renderNow(n, opts || {}); };
        var result = this.chain.then(run, run);
        this.chain = result.catch(function () { /* the next call still runs */ });
        return result;
    };

    Engine.prototype.renderNow = async function (n, opts) {
        if (!this.schedule) throw fail('readiness', 'Nothing is loaded.');
        if (this.dead) throw fail('code_error', 'The player stopped; load the story again.');
        var env = this.env, core = root.StorymakerCore, plan = core.schedule.framePlan(this.schedule, n);
        var warm = this.rendered < env.limits.warmupFrames;
        var ms = warm ? env.limits.warmupDeadlineMs : env.limits.frameDeadlineMs;
        var codeFrames = {}, transfer = [], self = this;
        var width = opts.width > 0 ? opts.width : 0;
        for (var i = 0; i < plan.layers.length; i++) {
            var layer = plan.layers[i], sc = this.schedule.scenes[layer.i];
            if (sc.kind !== 'code') continue;
            var channel = this.sceneChannels[sc.id];
            if (!channel || channel.dead) continue;
            try {
                var reply = await channel.call({ type: 'frame', t: layer.f / this.schedule.fps, width: width }, ms);
                codeFrames[sc.id] = reply.bitmap;
                if (typeof root.ImageBitmap === 'function' && reply.bitmap instanceof root.ImageBitmap) transfer.push(reply.bitmap);
            } catch (err) {
                var e = asError(err);
                if (typeof e.line === 'number' && e.line > 1) e.line -= 1;
                channel.kill(e);   // a scene that failed once shows its placeholder from now on
                this.codeErrors[sc.id] = e.message;
                env.onError({ code: e.code, message: e.message, scene_id: sc.id, line: e.line, column: e.column });
            }
        }
        try {
            var out = await this.send({ type: 'render', n: n, width: width, codeFrames: codeFrames, codeErrors: this.codeErrors }, ms, transfer);
            this.rendered++;
            return out.bitmap;
        } catch (err) {
            var pe = asError(err);
            if (pe.code === 'deadline') { this.dead = true; Object.keys(this.sceneChannels).forEach(function (k) { self.sceneChannels[k].kill(pe); }); }
            throw pe;
        }
    };

    H.LIMITS = LIMITS;
    H.fail = fail;
    H.asError = asError;
    H.Channel = Channel;
    H.Engine = Engine;
    H.createEngine = function (env) { return new Engine(env); };
    H.defaultEnv = defaultEnv;
})(typeof globalThis !== 'undefined' ? globalThis : self);
