/* Aura Drift - the worker entry of the Underdog Woof voice commentary.
 *
 * The model lives in aura-drift-woof-runtime.js and its text side in
 * aura-drift-woof-text.js. This file only opens the door to them and answers the two
 * messages the game sends: load the weights, speak a line.
 *
 * Model card: ConwayResearch/Underdog-Woof on the Hugging Face model hub.
 * Licence: Underdog Non-Commercial License 1.0 - free for non-commercial use with
 * the credit "Voice by Underdog Woof".
 */
(function () {
    'use strict';

    /* Inside a worker the runtime is pulled in here. The node test harness reads
       both files itself, so it loads them in that order. */
    if (typeof importScripts === 'function') {
        const version = (self.location && self.location.search) || '';
        importScripts('/js/desktop/apps/aura-drift-woof-text.js' + version);
        importScripts('/js/desktop/apps/aura-drift-woof-runtime.js' + version);
    }

    function runtime() {
        const R = self.AuraDriftWoofRuntime;
        if (!R) throw new Error('woof: the runtime is missing');
        return R;
    }

    let model = null;

    self.onmessage = function (event) {
        const msg = event.data || {};
        try {
            const R = runtime();
            if (msg.cmd === 'load') {
                const buffer = msg.buffer;
                model = new R.Woof(buffer);
                self.postMessage({
                    type: 'ready',
                    voices: Object.keys(model.speech.voices),
                    rate: R.SR,
                    maxFrames: model.maxFrames,
                    fps: model.fps
                });
                return;
            }
            if (msg.cmd === 'speak') {
                if (!model) throw new Error('woof: model is not loaded');
                const audio = model.speak(msg.text, msg.voice, msg.seed);
                const copy = new Float32Array(audio);
                self.postMessage({ type: 'audio', id: msg.id, rate: R.SR, samples: copy }, [copy.buffer]);
                return;
            }
        } catch (error) {
            self.postMessage({ type: 'error', id: msg.id, message: String(error && error.message || error) });
        }
    };
})();
