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
    }

    Runtime.prototype.canvas = function (name, w, h) {
        var key = name + ':' + w + 'x' + h;
        if (!this.canvases[key]) {
            var canvas = this.env.makeCanvas(w, h);
            this.canvases[key] = { canvas: canvas, ctx: canvas.getContext('2d') };
        }
        return this.canvases[key];
    };

    Runtime.prototype.missingNeeds = function () {
        var self = this, out = [];
        (this.needs || []).forEach(function (key) {
            if (key.indexOf('catalog:') === 0) { if (!self.files[key.slice(8)]) out.push(key); }
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
                var out = SM.compile(msg.story, msg.audioInfo || {}, msg.catalog, msg.opts);
                this.timeline = out.timeline;
                this.needs = out.needs;
                this.main = null;
                return { payload: { report: out.report, schedule: out.schedule, needs: out.needs, missing: this.missingNeeds() } };
            }
            case 'assets': {
                var assets = msg.assets || {}, k;
                for (k in assets) if (k.indexOf('catalog:') === 0) this.files[k.slice(8)] = assets[k];
                var fonts = msg.fonts || [], env = this.env;
                for (var i = 0; i < fonts.length; i++) {
                    var f = fonts[i];
                    if (!f || !f.data) continue;
                    var key = f.name + '/' + f.weight;
                    if (this.fontsLoaded[key]) continue;
                    try {
                        var face = new env.FontFace(f.family, f.data, { weight: String(f.weight) });
                        await face.load();
                        env.fonts.add(face);
                        this.fontsLoaded[key] = true;
                    } catch (err) {
                        throw fail('readiness', 'Font "' + f.family + '" (' + f.weight + ') could not be loaded: ' + (err && err.message || err));
                    }
                }
                return { payload: { missing: this.missingNeeds() } };
            }
            case 'render': {
                if (!this.timeline) throw fail('code_error', 'render before compile');
                var tl = this.timeline, width = msg.width > 0 ? Math.round(msg.width) : tl.width;
                var height = msg.width > 0 ? Math.round(msg.width * tl.height / tl.width) : tl.height;
                var main = this.canvas('main', width, height), self = this;
                var assetsObj = {
                    rig: this.files[tl.rigFile] || null, props: this.files, envs: this.files,
                    codeFrames: msg.codeFrames || {}, codeErrors: msg.codeErrors || {},
                    canvas: function (name, w, h) { return self.canvas(name, w, h); }
                };
                SM.renderFrame(tl, msg.n, main.ctx, assetsObj, { width: width === tl.width ? 0 : width });
                var bitmap = typeof main.canvas.transferToImageBitmap === 'function' ? main.canvas.transferToImageBitmap() : main.canvas;
                var frames = msg.codeFrames || {};
                for (var id in frames) if (frames[id] && typeof frames[id].close === 'function') frames[id].close();
                return { payload: { bitmap: bitmap, width: width, height: height }, transfer: typeof main.canvas.transferToImageBitmap === 'function' ? [bitmap] : [] };
            }
            case 'dispose':
                this.timeline = null;
                this.files = {};
                this.canvases = {};
                return { payload: {} };
            default:
                throw fail('code_error', 'unknown message type "' + msg.type + '"');
        }
    };

    SM.workerPlayer = { Runtime: Runtime, fail: fail };
})(typeof globalThis !== 'undefined' ? globalThis : self);
