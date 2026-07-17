(function () {
    'use strict';

    const MODEL_URL = '/js/realtime-speech/vendor/silero_vad_v6.2.1.onnx';
    const WASM_ROOT = '/js/realtime-speech/vendor/';
    const SAMPLE_RATE = 16000;
    const WINDOW_SAMPLES = 512;
    const CONTEXT_SAMPLES = 64;

    class SileroVAD {
        constructor(options) {
            this.options = Object.assign({ threshold: 0.5 }, options || {});
            this.session = null;
            this.state = null;
            this.context = new Float32Array(CONTEXT_SAMPLES);
            this.loading = null;
            this.fallback = false;
        }

        async load() {
            if (this.session || this.fallback) return this;
            if (this.loading) return this.loading;
            this.loading = (async () => {
                try {
                    if (!window.ort || !window.ort.InferenceSession) {
                        throw new Error('ONNX Runtime Web is unavailable');
                    }
                    window.ort.env.wasm.wasmPaths = WASM_ROOT;
                    window.ort.env.wasm.numThreads = 1;
                    this.session = await window.ort.InferenceSession.create(MODEL_URL, {
                        executionProviders: ['wasm'],
                        graphOptimizationLevel: 'all'
                    });
                    this.reset();
                } catch (error) {
                    this.fallback = true;
                    console.warn('[RealtimeSpeech] Silero VAD could not start; using the local energy fallback.', error);
                }
                return this;
            })();
            return this.loading;
        }

        reset() {
            this.context.fill(0);
            if (window.ort && window.ort.Tensor) {
                this.state = new window.ort.Tensor('float32', new Float32Array(2 * 1 * 128), [2, 1, 128]);
            } else {
                this.state = null;
            }
        }

        energyProbability(frame) {
            let energy = 0;
            let peak = 0;
            for (let i = 0; i < frame.length; i += 1) {
                const value = frame[i];
                energy += value * value;
                peak = Math.max(peak, Math.abs(value));
            }
            const rms = Math.sqrt(energy / Math.max(1, frame.length));
            return Math.max(0, Math.min(1, (rms - 0.006) / 0.035 + peak * 0.08));
        }

        async probability(frame) {
            if (!(frame instanceof Float32Array) || frame.length !== WINDOW_SAMPLES) {
                throw new Error('Silero VAD requires 512 samples at 16 kHz');
            }
            await this.load();
            if (this.fallback || !this.session || !this.state) return this.energyProbability(frame);

            const input = new Float32Array(CONTEXT_SAMPLES + WINDOW_SAMPLES);
            input.set(this.context, 0);
            input.set(frame, CONTEXT_SAMPLES);
            this.context.set(frame.subarray(WINDOW_SAMPLES - CONTEXT_SAMPLES));

            const feeds = {
                input: new window.ort.Tensor('float32', input, [1, input.length]),
                state: this.state,
                sr: new window.ort.Tensor('int64', BigInt64Array.from([BigInt(SAMPLE_RATE)]), [])
            };
            const output = await this.session.run(feeds);
            const probabilityTensor = output.output || output.probability || output[Object.keys(output)[0]];
            const nextState = output.stateN || output.state || output[Object.keys(output)[1]];
            if (nextState) this.state = nextState;
            const value = probabilityTensor && probabilityTensor.data ? Number(probabilityTensor.data[0]) : 0;
            return Number.isFinite(value) ? Math.max(0, Math.min(1, value)) : 0;
        }

        async isSpeech(frame) {
            return (await this.probability(frame)) >= this.options.threshold;
        }
    }

    window.AuraSileroVAD = {
        SileroVAD,
        MODEL_URL,
        SAMPLE_RATE,
        WINDOW_SAMPLES,
        CONTEXT_SAMPLES
    };
})();
