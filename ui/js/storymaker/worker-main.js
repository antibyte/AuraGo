// Storymaker player: role dispatch of the worker bundle. The blob prelude sets self.__SM_ROLE__ ("player" or "scene")
// before it imports this bundle. For a worker role this installs the determinism shadows, creates the runtime and the
// message handler (messages are handled one at a time, in order) and says hello. In the document realm there is no role,
// so loading the bundle there (the storyboard fallback) installs nothing. Classic script.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore;
    var role = root.__SM_ROLE__;
    if (role !== 'player' && role !== 'scene') return;

    SM.determinism.install(root);
    var env = {
        makeCanvas: function (w, h) { return new root.OffscreenCanvas(w, h); },
        FontFace: root.FontFace,
        fonts: root.fonts
    };
    var runtime = role === 'scene' ? new SM.workerScene.Runtime(env, root) : new SM.workerPlayer.Runtime(env);
    var chain = Promise.resolve();

    root.addEventListener('message', function (event) {
        var msg = event.data;
        if (!msg || typeof msg !== 'object' || typeof msg.type !== 'string') return;
        chain = chain.then(function () { return runtime.handle(msg); }).then(function (res) {
            var out = { type: 'reply', req: msg.req, ok: true }, k;
            for (k in res.payload) out[k] = res.payload[k];
            root.postMessage(out, res.transfer || []);
        }, function (err) {
            var e = err && typeof err.code === 'string' ? err : { code: 'code_error', message: String(err && err.message || err) };
            root.postMessage({ type: 'reply', req: msg.req, ok: false, error: e });
        });
    });
    root.postMessage({ type: 'hello', role: role });
})(typeof globalThis !== 'undefined' ? globalThis : self);
