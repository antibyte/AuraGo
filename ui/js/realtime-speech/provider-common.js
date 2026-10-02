(function () {
    'use strict';

    function floatToPCM16(samples) {
        const output = new Int16Array(samples.length);
        for (let i = 0; i < samples.length; i += 1) {
            const value = Math.max(-1, Math.min(1, samples[i]));
            output[i] = value < 0 ? Math.round(value * 32768) : Math.round(value * 32767);
        }
        return output;
    }

    function resampleLinear(samples, fromRate, toRate) {
        if (fromRate === toRate) return samples.slice();
        const outputLength = Math.max(1, Math.round(samples.length * toRate / fromRate));
        const output = new Float32Array(outputLength);
        const ratio = fromRate / toRate;
        for (let i = 0; i < outputLength; i += 1) {
            const position = i * ratio;
            const left = Math.min(samples.length - 1, Math.floor(position));
            const right = Math.min(samples.length - 1, left + 1);
            const fraction = position - left;
            output[i] = samples[left] * (1 - fraction) + samples[right] * fraction;
        }
        return output;
    }

    function bytesToBase64(bytes) {
        let binary = '';
        const chunkSize = 0x8000;
        for (let i = 0; i < bytes.length; i += chunkSize) {
            binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunkSize));
        }
        return btoa(binary);
    }

    function base64ToBytes(value) {
        const binary = atob(String(value || ''));
        const bytes = new Uint8Array(binary.length);
        for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
        return bytes;
    }

    function floatToBase64PCM16(samples, fromRate, toRate) {
        const resampled = resampleLinear(samples, fromRate, toRate);
        return bytesToBase64(new Uint8Array(floatToPCM16(resampled).buffer));
    }

    function safeJSON(value) {
        if (typeof value !== 'string') return value;
        try { return JSON.parse(value); } catch (_) { return null; }
    }

    // analyserLevel reads the current time-domain RMS of an AnalyserNode into a
    // reusable buffer and returns a normalized 0..1 level for visualizations.
    function analyserLevel(analyser, buffer) {
        if (!analyser || !buffer) return 0;
        analyser.getFloatTimeDomainData(buffer);
        let sum = 0;
        for (let i = 0; i < buffer.length; i += 1) sum += buffer[i] * buffer[i];
        const rms = Math.sqrt(sum / buffer.length);
        return Math.min(1, rms * 2.4);
    }

    function randomID(prefix) {
        if (window.crypto && typeof window.crypto.randomUUID === 'function') {
            return String(prefix || 'id') + '-' + window.crypto.randomUUID();
        }
        const bytes = new Uint8Array(16);
        if (window.crypto && window.crypto.getRandomValues) window.crypto.getRandomValues(bytes);
        return String(prefix || 'id') + '-' + Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('');
    }

    // AudioOutput routes every Live Speech output either to this browser's
    // speakers or to the server headset bridge. Callers ask once per
    // AudioContext and stream (0 = replies, 1 = progress narration).
    class AudioOutput extends EventTarget {
        constructor() {
            super();
            this.mode = 'local';
            this.bridge = null;
            this.entries = new Set();
        }

        // destination returns the node to connect output to. With localSilent
        // the local route stays silent because the caller already plays the
        // audio itself (the OpenAI <audio> element); the node is still pulled.
        destination(context, stream, options) {
            const normalized = stream === 1 ? 1 : 0;
            const localSilent = !!(options && options.localSilent);
            for (const entry of this.entries) {
                if (entry.context === context && entry.stream === normalized && entry.localSilent === localSilent) return entry.input;
            }
            const entry = { context, stream: normalized, localSilent, input: context.createGain(), mute: null, tap: null, tapPromise: null };
            this.entries.add(entry);
            this.route(entry);
            return entry.input;
        }

        release(context) {
            for (const entry of [...this.entries]) {
                if (entry.context !== context) continue;
                this.entries.delete(entry);
                try { entry.input.disconnect(); } catch (_) { }
                if (entry.tap) entry.tap.port.onmessage = null;
            }
        }

        setBridge(bridge) {
            this.bridge = bridge || null;
            const mode = this.bridge ? 'bridge' : 'local';
            if (mode === this.mode) return;
            this.mode = mode;
            this.entries.forEach(entry => this.route(entry));
            this.dispatchEvent(new CustomEvent('modechange', { detail: { mode } }));
        }

        flush(stream) {
            if (this.bridge) this.bridge.flush(stream === 1 ? 1 : 0);
        }

        route(entry) {
            try { entry.input.disconnect(); } catch (_) { }
            if (entry.context.state === 'closed') {
                this.entries.delete(entry);
                return;
            }
            if (this.mode === 'local') {
                if (!entry.localSilent) {
                    entry.input.connect(entry.context.destination);
                    return;
                }
                if (!entry.mute) {
                    entry.mute = entry.context.createGain();
                    entry.mute.gain.value = 0;
                    entry.mute.connect(entry.context.destination);
                }
                entry.input.connect(entry.mute);
                return;
            }
            void this.ensureTap(entry).then(tap => {
                if (tap && this.mode === 'bridge' && this.entries.has(entry)) entry.input.connect(tap);
            });
        }

        ensureTap(entry) {
            if (entry.tap) return Promise.resolve(entry.tap);
            if (!entry.tapPromise) {
                entry.tapPromise = (async () => {
                    await entry.context.audioWorklet.addModule('/js/realtime-speech/output-tap-worklet.js');
                    const tap = new AudioWorkletNode(entry.context, 'aurago-realtime-output-tap', {
                        numberOfInputs: 1,
                        numberOfOutputs: 1,
                        outputChannelCount: [1],
                        processorOptions: { targetRate: 24000, chunkSamples: 480 }
                    });
                    const sink = entry.context.createGain();
                    sink.gain.value = 0;
                    tap.connect(sink);
                    sink.connect(entry.context.destination);
                    tap.port.onmessage = event => {
                        if (this.mode !== 'bridge' || !this.bridge) return;
                        this.bridge.sendOutput(entry.stream, event.data);
                    };
                    entry.tap = tap;
                    return tap;
                })().catch(() => null);
            }
            return entry.tapPromise;
        }
    }

    const audioOutput = new AudioOutput();

    class PCMPlayer extends EventTarget {
        constructor(sampleRate) {
            super();
            this.sampleRate = sampleRate || 24000;
            this.context = null;
            this.nextStart = 0;
            this.sources = new Set();
            this.active = false;
            this.analyser = null;
            this.analyserTime = null;
            this.analyserBins = null;
            this.output = null;
        }

        async ensureContext() {
            if (!this.context) {
                const AudioContextClass = window.AudioContext || window.webkitAudioContext;
                this.context = new AudioContextClass({ latencyHint: 'interactive', sampleRate: this.sampleRate });
            }
            if (this.context.state === 'suspended') await this.context.resume();
        }

        outputNode() {
            if (!this.output) this.output = audioOutput.destination(this.context, 0);
            return this.output;
        }

        ensureAnalyser() {
            if (!this.context || this.analyser) return this.analyser;
            try {
                this.analyser = this.context.createAnalyser();
                this.analyser.fftSize = 256;
                this.analyser.smoothingTimeConstant = 0.55;
                this.analyser.connect(this.outputNode());
                this.analyserTime = new Float32Array(this.analyser.fftSize);
                this.analyserBins = new Uint8Array(this.analyser.frequencyBinCount);
            } catch (_) {
                this.analyser = null;
                this.analyserTime = null;
                this.analyserBins = null;
            }
            return this.analyser;
        }

        getOutputLevel() {
            if (!this.analyser || !this.active || !this.context || this.context.state !== 'running') return 0;
            return analyserLevel(this.analyser, this.analyserTime);
        }

        getOutputSpectrum(target) {
            if (!this.analyser || !target || !target.length) return false;
            this.analyser.getByteFrequencyData(this.analyserBins);
            const bins = this.analyserBins.length;
            for (let i = 0; i < target.length; i += 1) {
                target[i] = this.analyserBins[Math.min(bins - 1, Math.floor(i * bins / target.length))];
            }
            return true;
        }

        async appendBase64PCM16(base64, sampleRate) {
            const bytes = base64ToBytes(base64);
            if (bytes.length < 2) return;
            await this.ensureContext();
            const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
            const samples = new Float32Array(Math.floor(bytes.byteLength / 2));
            for (let i = 0; i < samples.length; i += 1) {
                samples[i] = view.getInt16(i * 2, true) / 32768;
            }
            const rate = sampleRate || this.sampleRate;
            const buffer = this.context.createBuffer(1, samples.length, rate);
            buffer.copyToChannel(samples, 0);
            const source = this.context.createBufferSource();
            source.buffer = buffer;
            source.connect(this.ensureAnalyser() || this.outputNode());
            const now = this.context.currentTime;
            const startAt = Math.max(now + 0.01, this.nextStart);
            this.nextStart = startAt + buffer.duration;
            source.onended = () => {
                this.sources.delete(source);
                if (this.sources.size === 0) {
                    this.active = false;
                    this.dispatchEvent(new CustomEvent('idle'));
                }
            };
            this.sources.add(source);
            if (!this.active) {
                this.active = true;
                this.dispatchEvent(new CustomEvent('playing'));
            }
            source.start(startAt);
        }

        stop() {
            audioOutput.flush(0);
            this.sources.forEach(source => {
                try { source.stop(); } catch (_) { }
            });
            this.sources.clear();
            this.nextStart = this.context ? this.context.currentTime : 0;
            if (this.active) {
                this.active = false;
                this.dispatchEvent(new CustomEvent('idle'));
            }
        }

        whenIdle() {
            if (!this.active || this.sources.size === 0) return Promise.resolve();
            return new Promise(resolve => {
                this.addEventListener('idle', resolve, { once: true });
            });
        }

        async close() {
            this.stop();
            if (this.context) audioOutput.release(this.context);
            this.output = null;
            if (this.context && this.context.state !== 'closed') {
                try { await this.context.close(); } catch (_) { }
            }
            this.context = null;
            this.analyser = null;
            this.analyserTime = null;
            this.analyserBins = null;
        }
    }

    class ProviderAdapter extends EventTarget {
        constructor(options) {
            super();
            this.options = options || {};
            this.connected = false;
            this.closed = false;
            this.session = null;
            this.responseActive = false;
        }

        emit(type, detail) {
            this.dispatchEvent(new CustomEvent(type, { detail: detail || {} }));
        }

        setState(state, extra) {
            this.emit('state', Object.assign({ state }, extra || {}));
        }

        transcript(speaker, text, final, turnId) {
            this.emit('transcript', {
                speaker,
                text: String(text || ''),
                final: !!final,
                turnId: turnId || ''
            });
        }

        fail(error) {
            const normalized = error instanceof Error ? error : new Error(String(error || 'Provider error'));
            this.emit('error', { error: normalized, message: normalized.message });
        }

        async syncContext(messages) {
            if (!Array.isArray(messages)) return;
            for (const message of messages) await this.sendContextMessage(message);
        }

        async sendContextMessage() { }
    }

    window.AuraRealtimeProviderCommon = {
        ProviderAdapter,
        PCMPlayer,
        audioOutput,
        floatToPCM16,
        resampleLinear,
        bytesToBase64,
        base64ToBytes,
        floatToBase64PCM16,
        safeJSON,
        randomID,
        analyserLevel
    };
    window.AuraRealtimeAudioOutput = audioOutput;
})();
