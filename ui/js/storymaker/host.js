// Storymaker host (document realm): entry point. window.StorymakerHost.mount(opts) with opts = {mode, canvas, base,
// boot?} mounts the app protocol or the render driver; on load the bundle mounts itself from the markup the server wrote
// (<html data-sm-mode data-sm-base> and <canvas id="sm-stage">). The page runs no inline script. Classic script.
(function (root) {
    'use strict';
    var H = root.StorymakerHost || (root.StorymakerHost = {});

    // mount(opts): opts = {mode: "app" | "render", canvas, base, window?, env?, fetch?}
    H.mount = function (opts) {
        if (!opts || !opts.canvas || !opts.base) throw new Error('StorymakerHost.mount needs a canvas and a base URL.');
        if (opts.mode === 'app') return H.mountApp(opts);
        if (opts.mode === 'render') return H.mountRender(opts);
        throw new Error('Unknown Storymaker mode "' + opts.mode + '".');
    };

    function autoMount() {
        var doc = root.document;
        if (!doc || !doc.documentElement || !doc.documentElement.dataset) return;
        var ds = doc.documentElement.dataset;
        if (!ds.smMode || !ds.smBase) return;
        var canvas = doc.getElementById('sm-stage');
        if (!canvas) return;
        H.mounted = H.mount({ mode: ds.smMode, base: ds.smBase, canvas: canvas });
    }

    if (root.document && root.document.readyState === 'loading') root.document.addEventListener('DOMContentLoaded', autoMount);
    else autoMount();
})(typeof globalThis !== 'undefined' ? globalThis : self);
