// Last part of desktop-sounds.bundle.js. In a Worker it answers render requests; in a window it does nothing.
(function () {
    'use strict';
    if (typeof WorkerGlobalScope === 'undefined' || !(self instanceof WorkerGlobalScope)) return;
    self.onmessage = function (event) {
        const msg = event.data || {};
        try {
            const out = self.DesktopSoundEngine.render(String(msg.theme || ''), String(msg.event || ''), msg.variant == null ? 0 : msg.variant);
            self.postMessage({ id: msg.id, ok: true, L: out.L, R: out.R, sampleRate: out.sampleRate, seconds: out.seconds, lufs: out.lufs, peakDb: out.peakDb }, [out.L.buffer, out.R.buffer]);
        } catch (err) {
            self.postMessage({ id: msg.id, ok: false, error: String((err && err.message) || err) });
        }
    };
})();
