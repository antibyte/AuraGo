// Storymaker player: the player runtime (trusted, our code only). It holds the compiled timeline and the loaded assets
// and answers the host's messages: plan (timing facts for code scenes), compile (timeline + compile report), assets
// (catalog JSON and fonts, the readiness barrier) and render (one frame as an ImageBitmap). The same Runtime serves the
// player worker and, when workers are unavailable, the host realm (storyboard fallback). Classic script; publishes
// StorymakerCore.workerPlayer.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function fail(code, message, extra) {
        var e = { code: code, message: message };
        if (extra) for (var k in extra) e[k] = extra[k];
        return e;
    }

    // env = {makeCanvas(w, h), FontFace, fonts}
    function Runtime(env) {
        this.env = env;
        this.timeline = null;
        this.files = {};
        this.canvases = {};
        this.main = null;
        this.fontsLoaded = {};
        this.fontFaces = [];
        this.bitmaps = {};
        this.textureData = {};
        this.renderers = {};
        this.graphicsGeneration = 0;
    }

    Runtime.prototype.canvas = function (name, w, h) {
        var key = name + ':' + w + 'x' + h;
        if (!this.canvases[key]) {
            var keys = Object.keys(this.canvases);
            if (keys.length >= 9) { this.canvases[keys[0]].canvas.width = 1; this.canvases[keys[0]].canvas.height = 1; delete this.canvases[keys[0]]; }
            var canvas = this.env.makeCanvas(w, h);
            this.canvases[key] = { canvas: canvas, ctx: canvas.getContext('2d') };
        }
        var result = this.canvases[key]; delete this.canvases[key]; this.canvases[key] = result;
        return result;
    };

    Runtime.prototype.missingNeeds = function () {
        var self = this, out = [];
        (this.needs || []).forEach(function (key) {
            if (key.indexOf('catalog:') === 0) { if (!self.files[key.slice(8)]) out.push(key); }
            else if (key.indexOf('texture:') === 0) { if (!self.textureData[key]) out.push(key); }
            else if (key.indexOf('font:') === 0) { if (!self.fontsLoaded[key.slice(5)]) out.push(key); }
        });
        return out;
    };

    Runtime.prototype.handle = async function (msg) {
        switch (msg.type) {
            case 'plan': {
                var plan = SM.planTiming(msg.story, msg.audioInfo || {}, msg.catalog, msg.opts);
                return { payload: { plan: plan } };
            }
            case 'compile': {
                this.disposeGraphics();
                this.catalog = msg.catalog;
                var out = SM.compile(msg.story, msg.audioInfo || {}, msg.catalog, msg.opts);
                this.timeline = out.timeline;
                this.needs = out.needs;
                this.main = null;
                return { payload: { report: out.report, schedule: out.schedule, needs: out.needs, missing: this.missingNeeds() } };
            }
            case 'assets': {
                var assets = msg.assets || {}, k;
                for (k in assets) if (k.indexOf('texture:') === 0) this.textureData[k] = assets[k];
                for (k in assets) if (k.indexOf('catalog:') === 0) this.files[k.slice(8)] = assets[k];
                var fonts = msg.fonts || [], env = this.env, generation = this.graphicsGeneration;
                for (var i = 0; i < fonts.length; i++) {
                    var f = fonts[i];
                    if (!f || !f.data) continue;
                    var key = f.name + '/' + f.weight;
                    if (this.fontsLoaded[key]) continue;
                    try {
                        var desc = { weight: f.weight_range || String(f.weight) };
                        if (f.unicode_range) desc.unicodeRange = f.unicode_range;
                        var face = new env.FontFace(f.family, f.data, desc);
                        await face.load();
                        if (generation !== this.graphicsGeneration) throw new Error('Project was disposed');
                        env.fonts.add(face);
                        this.fontFaces.push(face);
                        this.fontsLoaded[key] = true;
                    } catch (err) {
                        throw fail('readiness', 'Font "' + f.family + '" (' + f.weight + ') could not be loaded: ' + (err && err.message || err));
                    }
                }
                return { payload: { missing: this.missingNeeds() } };
            }
            case 'prepare_frame':
                return this.handle({ type: 'render', n: msg.n, width: msg.width, prepareOnly: true });
            case 'render': {
                try {
                    if (!this.timeline) throw fail('code_error', 'render before compile');
                    var tl = this.timeline, width = msg.width > 0 ? Math.round(msg.width) : tl.width;
                    var height = msg.width > 0 ? Math.round(msg.width * tl.height / tl.width) : tl.height;
                    var main = this.canvas('main', width, height), self = this;
                    var raster = this.catalog && this.catalog.schema === 'storymaker-catalog/2'
                        ? await SM.raster.get(this, width, height) : null;
                    var assetsObj = {
                        raster: raster, bitmaps: this.bitmaps,
                        rig: this.files[tl.rigFile] || null, props: this.files, envs: this.files,
                        codeFrames: msg.codeFrames || {}, codeErrors: msg.codeErrors || {},
                        canvas: function (name, w, h) { return self.canvas(name, w, h); }
                    };
                    if (raster) {
                        var rc = SM.render.makeContext(tl, assetsObj, width, height);
                        var draws = SM.schedule.framePlan(tl.schedule, msg.n).layers.map(function (layer) { return { scene: tl.scenes[layer.i], t: layer.f / tl.fps }; });
                        await SM.raster.prepare(this, draws, rc);
                    }
                    if (msg.prepareOnly) return { payload: {} };
                    SM.renderFrame(tl, msg.n, main.ctx, assetsObj, { width: width === tl.width ? 0 : width });
                    if (msg.png) {   // the parallel path of the render page: the PNG is encoded here, so the host main thread stays free
                        var blob = await main.canvas.convertToBlob({ type: 'image/png' });
                        return { payload: { blob: blob, width: width, height: height } };
                    }
                    var bitmap = typeof main.canvas.transferToImageBitmap === 'function' ? main.canvas.transferToImageBitmap() : main.canvas;
                    return { payload: { bitmap: bitmap, width: width, height: height }, transfer: typeof main.canvas.transferToImageBitmap === 'function' ? [bitmap] : [] };
                } finally {
                    var frames = msg.codeFrames || {};
                    for (var id in frames) if (frames[id] && typeof frames[id].close === 'function') frames[id].close();
                }
            }
            case 'layout': {
                if (!this.timeline) throw fail('code_error', 'layout before compile');
                var ltl = this.timeline, lself = this;
                var lassets = {
                    raster: null, bitmaps: this.bitmaps, rig: this.files[ltl.rigFile] || null, props: this.files, envs: this.files,
                    codeFrames: {}, codeErrors: {}, canvas: function (name, w, h) { return lself.canvas(name, w, h); }
                };
                return { payload: { layout: SM.layout.measure(ltl, lassets) } };
            }
            case 'dispose':
                this.disposeGraphics();
                this.timeline = null;
                this.files = {};
                this.canvases = {};
                return { payload: {} };
            default:
                throw fail('code_error', 'unknown message type "' + msg.type + '"');
        }
    };

    Runtime.prototype.disposeGraphics = function () {
        this.graphicsGeneration++;
        SM.timeline.dispose(this.timeline);
        Object.keys(this.renderers).forEach(function (k) { this.renderers[k].dispose(); }, this);
        this.renderers = {};
        SM.textures.dispose(this.bitmaps);
        Object.keys(this.canvases).forEach(function (k) { this.canvases[k].canvas.width = this.canvases[k].canvas.height = 1; }, this);
        this.canvases = {};
        this.textureData = {};
        this.fontFaces.forEach(function (face) { if (this.env.fonts && this.env.fonts.delete) this.env.fonts.delete(face); }, this);
        this.fontFaces = [];
        this.fontsLoaded = {};
    };

    SM.workerPlayer = { Runtime: Runtime, fail: fail };
})(typeof globalThis !== 'undefined' ? globalThis : self);
