(function () {
    'use strict';

    const Common = window.AuraRealtimeProviderCommon;
    const MAX_SAMPLES = 16000 * 120;

    function encodeWAV(samples, sampleRate) {
        const buffer = new ArrayBuffer(44 + samples.length * 2);
        const view = new DataView(buffer);
        const text = (offset, value) => {
            for (let i = 0; i < value.length; i += 1) view.setUint8(offset + i, value.charCodeAt(i));
        };
        text(0, 'RIFF');
        view.setUint32(4, 36 + samples.length * 2, true);
        text(8, 'WAVE');
        text(12, 'fmt ');
        view.setUint32(16, 16, true);
        view.setUint16(20, 1, true);
        view.setUint16(22, 1, true);
        view.setUint32(24, sampleRate, true);
        view.setUint32(28, sampleRate * 2, true);
        view.setUint16(32, 2, true);
        view.setUint16(34, 16, true);
        text(36, 'data');
        view.setUint32(40, samples.length * 2, true);
        for (let i = 0; i < samples.length; i += 1) {
            const value = Math.max(-1, Math.min(1, samples[i]));
            view.setInt16(44 + i * 2, value < 0 ? value * 32768 : value * 32767, true);
        }
        return new Blob([buffer], { type: 'audio/wav' });
    }

    class SpeechLabRealtimeAdapter extends Common.ProviderAdapter {
        constructor(options) {
            super(options);
            this.clientId = (options && options.clientId) || '';
            this.chunks = [];
            this.sampleCount = 0;
            this.busy = false;
            this.player = null;
            this.outputAudio = null;
            this.outputContext = null;
            this.outputAnalyser = null;
            this.outputBuffer = null;
            this.outputSource = null;
        }

        async connect(connectOptions) {
            this.closed = false;
            this.setState('connecting');
            const response = await connectOptions.createSession({ transport: 'local_s2s' });
            this.session = response;
            this.connected = true;
            this.setState('listening');
            return response;
        }

        sendAudio(samples) {
            if (!this.connected || !samples || !samples.length || this.busy) return;
            const remaining = MAX_SAMPLES - this.sampleCount;
            if (remaining <= 0) return;
            const frame = samples.length > remaining ? samples.slice(0, remaining) : samples.slice();
            this.chunks.push(frame);
            this.sampleCount += frame.length;
        }

        endTurn() {
            if (!this.connected || this.busy) return;
            void this.commitTurn();
        }

        mergeChunks() {
            if (!this.chunks.length) return new Float32Array(0);
            const merged = new Float32Array(this.sampleCount);
            let offset = 0;
            this.chunks.forEach(chunk => {
                merged.set(chunk, offset);
                offset += chunk.length;
            });
            this.chunks = [];
            this.sampleCount = 0;
            return merged;
        }

        async commitTurn() {
            const samples = this.mergeChunks();
            if (samples.length < 1600) return;
            this.busy = true;
            this.setState('executing');
            try {
                const wav = encodeWAV(samples, 16000);
                const form = new FormData();
                form.append('audio', wav, 'speech.wav');
                const sessionId = this.session && this.session.session_id;
                const response = await fetch('/api/realtime-speech/transcribe?session_id=' + encodeURIComponent(sessionId || ''), {
                    method: 'POST',
                    credentials: 'same-origin',
                    cache: 'no-store',
                    headers: {
                        'X-Realtime-Speech-Client-ID': this.clientId,
                        'X-Realtime-Speech-Session-ID': sessionId || ''
                    },
                    body: form
                });
                if (!response.ok) {
                    const body = await response.json().catch(() => ({}));
                    if (body && body.error === 'speech_lab_no_speech') return;
                    throw new Error(body.message || body.error || ('HTTP ' + response.status));
                }
                const payload = await response.json();
                const text = String(payload.transcription || '').trim();
                if (!text) return;
                const callId = Common.randomID('s2s');
                this.transcript('user', text, true, callId);
                this.emit('toolCall', {
                    name: 'aurago_execute',
                    callId,
                    arguments: { request: text }
                });
            } catch (error) {
                this.fail(error);
            } finally {
                this.busy = false;
                if (this.connected && !this.closed) this.setState('listening');
            }
        }

        async sendToolResult(call, result) {
            const text = String((result && (result.text || result.error)) || '').trim();
            if (!text) return;
            this.transcript('assistant', text, true, call && call.callId);
            try {
                await this.speak(text);
            } catch (error) {
                this.fail(error);
            }
        }

        // attachOutputTap routes the utterance element through an AnalyserNode for
        // output visualizations. MediaElementSource takes over the element's audio
        // path, so the tap is only installed when the context is running — plain
        // element playback remains the fallback otherwise.
        async attachOutputTap(audio) {
            try {
                const AudioContextClass = window.AudioContext || window.webkitAudioContext;
                if (!AudioContextClass || !audio) return;
                if (!this.outputContext) {
                    this.outputContext = new AudioContextClass({ latencyHint: 'interactive' });
                    this.outputAnalyser = this.outputContext.createAnalyser();
                    this.outputAnalyser.fftSize = 256;
                    this.outputAnalyser.smoothingTimeConstant = 0.55;
                    this.outputAnalyser.connect(this.outputContext.destination);
                    this.outputBuffer = new Float32Array(this.outputAnalyser.fftSize);
                }
                await this.outputContext.resume();
                if (this.outputContext.state !== 'running') return;
                this.detachOutputTap();
                this.outputSource = this.outputContext.createMediaElementSource(audio);
                this.outputSource.connect(this.outputAnalyser);
            } catch (_) { /* the visualization tap is optional */ }
        }

        detachOutputTap() {
            const source = this.outputSource;
            this.outputSource = null;
            if (source) {
                try { source.disconnect(); } catch (_) { }
            }
        }

        getOutputLevel() {
            if (!this.outputAnalyser || !this.outputBuffer) return 0;
            return Common.analyserLevel(this.outputAnalyser, this.outputBuffer);
        }

        async speak(text) {
            this.interruptOutput();
            const sessionId = this.session && this.session.session_id;
            const response = await fetch('/api/realtime-speech/synthesize', {
                method: 'POST',
                credentials: 'same-origin',
                cache: 'no-store',
                headers: {
                    'Content-Type': 'application/json',
                    'X-Realtime-Speech-Client-ID': this.clientId
                },
                body: JSON.stringify({
                    session_id: sessionId,
                    client_id: this.clientId,
                    text
                })
            });
            if (!response.ok) {
                const body = await response.json().catch(() => ({}));
                throw new Error(body.message || body.error || ('HTTP ' + response.status));
            }
            const blob = await response.blob();
            const url = URL.createObjectURL(blob);
            const audio = new Audio(url);
            this.outputAudio = audio;
            await this.attachOutputTap(audio);
            this.emit('audio', { active: true });
            await new Promise((resolve, reject) => {
                audio.onended = () => resolve();
                audio.onerror = () => reject(new Error('Speech Lab playback failed'));
                void audio.play().catch(reject);
            });
            URL.revokeObjectURL(url);
            this.detachOutputTap();
            if (this.outputAudio === audio) this.outputAudio = null;
            this.emit('audio', { active: false });
        }

        interruptOutput() {
            if (this.outputAudio) {
                try { this.outputAudio.pause(); } catch (_) { }
                this.outputAudio = null;
            }
            this.detachOutputTap();
            this.emit('audio', { active: false });
        }

        async park() {
            this.setState('parked', { warm: true });
        }

        async resume() {
            if (!this.connected) throw new Error('Speech Lab session is no longer available');
            this.setState('listening');
            return this.session;
        }

        async close() {
            this.closed = true;
            this.connected = false;
            this.interruptOutput();
            this.chunks = [];
            this.sampleCount = 0;
            if (this.outputContext && this.outputContext.state !== 'closed') {
                try { await this.outputContext.close(); } catch (_) { }
            }
            this.outputContext = null;
            this.outputAnalyser = null;
            this.outputBuffer = null;
            this.setState('closed');
        }
    }

    window.AuraRealtimeProviders = window.AuraRealtimeProviders || {};
    window.AuraRealtimeProviders.speech_lab = SpeechLabRealtimeAdapter;
})();
