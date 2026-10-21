// Pixel image worker (classic worker, NOT listed in the module loader). Created lazily by runtime.runJob in pixel-core.js.
// Protocol: main -> worker { id, op, args }; worker -> main { ready: true } once, then { id, progress } while running and
// { id, result } or { id, error } when done (result buffers are transferred). Every pixel-lib-*.js is imported here and
// TestPixelWorkerImportsEveryLib keeps this list in sync with the folder.
'use strict';

(function () {
    // The loader's cache-busting query string (?v=<build>) is repeated for every lib file.
    const search = self.location && self.location.search ? self.location.search : '';
    [
        'pixel-lib-core.js',
        'pixel-lib-fx.js',
        'pixel-lib-gen.js',
        'pixel-lib-cutout.js',
        'pixel-lib-inpaint.js'
    ].forEach(file => importScripts(file + search));

    function reply(res) {
        if (res.error !== undefined) self.postMessage({ id: res.id, error: res.error });
        else self.postMessage({ id: res.id, result: res.result }, res.transfer || []);
    }

    self.onmessage = function onmessage(event) {
        const msg = event.data || {};
        let lastSent = 0, lastFraction = -1;
        const res = self.AuraPixelLib.handleMessage(msg, {
            // At most ~20 progress messages per second, and only when the value moved.
            progress(fraction) {
                const now = Date.now();
                if (fraction !== 1 && (now - lastSent < 50 || fraction - lastFraction < 0.01)) return;
                lastSent = now;
                lastFraction = fraction;
                self.postMessage({ id: msg.id, progress: fraction });
            }
        });
        if (res && typeof res.then === 'function') res.then(reply);
        else reply(res);
    };

    self.postMessage({ ready: true });
})();
