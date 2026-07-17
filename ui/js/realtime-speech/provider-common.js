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

    function randomID(prefix) {
        if (window.crypto && typeof window.crypto.randomUUID === 'function') {
            return String(prefix || 'id') + '-' + window.crypto.randomUUID();
        }
        const bytes = new Uint8Array(16);
        if (window.crypto && window.crypto.getRandomValues) window.crypto.getRandomValues(bytes);
        return String(prefix || 'id') + '-' + Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('');
    }

    class PCMPlayer extends EventTarget {
        constructor(sampleRate) {
            super();
            this.sampleRate = sampleRate || 24000;
            this.context = null;
            this.nextStart = 0;
            this.sources = new Set();
            this.active = false;
        }

        async ensureContext() {
            if (!this.context) {
                const AudioContextClass = window.AudioContext || window.webkitAudioContext;
                this.context = new AudioContextClass({ latencyHint: 'interactive', sampleRate: this.sampleRate });
            }
            if (this.context.state === 'suspended') await this.context.resume();
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
            source.connect(this.context.destination);
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
            if (this.context && this.context.state !== 'closed') {
                try { await this.context.close(); } catch (_) { }
            }
            this.context = null;
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
        floatToPCM16,
        resampleLinear,
        bytesToBase64,
        base64ToBytes,
        floatToBase64PCM16,
        safeJSON,
        randomID
    };
})();
