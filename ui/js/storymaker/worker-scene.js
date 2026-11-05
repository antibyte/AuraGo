// Storymaker player: the code-scene runtime (untrusted scene code runs here, one worker per code scene). The worker
// imports the player bundle for the scene API, the bundled scene registers `self.__smScene`, and the runtime answers: build
// (run the scene function once: duration, needs, issues), assets (catalog JSON and fonts) and frame (the scene at time t as
// an ImageBitmap from its own canvas). Errors carry the message and, when the stack names it, a line and column of the
// worker script. Classic script; publishes StorymakerCore.workerScene.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function fail(code, message, extra) {
        var e = { code: code, message: message };
        if (extra) for (var k in extra) e[k] = extra[k];
        return e;
    }

    // locate(err) = {line, column} of the first stack frame inside the worker script (a blob URL), else {}.
    function locate(err) {
        var stack = err && err.stack ? String(err.stack) : '', lines = stack.split('\n');
        for (var i = 0; i < lines.length; i++) {
            var m = /(?:blob:[^\s)]*|<anonymous>|scene\.bundle\.js|worker\.js):(\d+):(\d+)/.exec(lines[i]);
            if (m) return { line: parseInt(m[1], 10), column: parseInt(m[2], 10) };
        }
        return {};
    }

    function sceneError(err, fallback) {
        var where = locate(err), msg = err && err.message ? String(err.message) : String(err);
        var out = fail('code_error', (fallback ? fallback + ': ' : '') + msg);
        if (where.line) { out.line = where.line; out.column = where.column; }
        return out;
    }

    function Runtime(env, scope) {
        this.env = env;
        this.scope = scope;
        this.built = null;
        this.opts = null;
        this.files = {};
        this.canvases = {};
        this.fontsLoaded = {};
    }

    Runtime.prototype.handle = async function (msg) {
        switch (msg.type) {
            case 'build': {
                var fn = this.scope.__smScene;
                if (typeof fn !== 'function') throw fail('code_error', 'The scene bundle did not register a scene function (export default function scene(api)).');
                this.opts = msg.opts;
                var built;
                try { built = SM.codescene.build(fn, msg.opts); } catch (err) { throw sceneError(err, 'Scene "' + msg.opts.sceneId + '"'); }
                this.built = built;
                return { payload: { duration: built.duration, explicit: built.explicit, needs: built.needs, issues: built.issues } };
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
                        var desc = { weight: f.weight_range || String(f.weight) };
                        if (f.unicode_range) desc.unicodeRange = f.unicode_range;
                        var face = new env.FontFace(f.family, f.data, desc);
                        await face.load();
                        env.fonts.add(face);
                        this.fontsLoaded[key] = true;
                    } catch (err) {
                        throw fail('readiness', 'Font "' + f.family + '" could not be loaded: ' + (err && err.message || err));
                    }
                }
                return { payload: {} };
            }
            case 'frame': {
                if (!this.built) throw fail('code_error', 'frame before build');
                var o = this.opts, width = msg.width > 0 ? Math.round(msg.width) : o.width;
                var height = msg.width > 0 ? Math.round(msg.width * o.height / o.width) : o.height;
                var key2 = width + 'x' + height;
                if (!this.canvases[key2]) { var c = this.env.makeCanvas(width, height); this.canvases[key2] = { canvas: c, ctx: c.getContext('2d') }; }
                var main = this.canvases[key2];
                var tl = { aspect: o.aspect, fps: o.fps, palette: o.palette, cat: SM.scene.indexCatalog(o.catalog), fontFamilies: o.families || {}, fontChain: o.fontChain, style: { font: o.styleFont || 'default' }, safe: o.safe };
                var rc = SM.render.makeContext(tl, { rig: this.files[o.catalog && o.catalog.rig && o.catalog.rig.file] || null, props: this.files, envs: this.files }, width, height);
                rc.sceneId = o.sceneId;
                this.built.stages = this.built.stages || {};
                try { SM.codescene.drawFrame(this.built, msg.t, main.ctx, rc); } catch (err) { throw sceneError(err, 'Scene "' + o.sceneId + '" at ' + msg.t.toFixed(2) + ' s'); }
                var bitmap = typeof main.canvas.transferToImageBitmap === 'function' ? main.canvas.transferToImageBitmap() : main.canvas;
                return { payload: { bitmap: bitmap, width: width, height: height }, transfer: typeof main.canvas.transferToImageBitmap === 'function' ? [bitmap] : [] };
            }
            case 'dispose':
                this.built = null;
                this.canvases = {};
                return { payload: {} };
            default:
                throw fail('code_error', 'unknown message type "' + msg.type + '"');
        }
    };

    SM.workerScene = { Runtime: Runtime, locate: locate, sceneError: sceneError };
})(typeof globalThis !== 'undefined' ? globalThis : self);
