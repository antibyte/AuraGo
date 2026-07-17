(function () {
    'use strict';

    const SAMPLE_RATE = 16000;
    const CHUNK_SAMPLES = 512;
    const PRE_ROLL_SAMPLES = 4800;
    const CONFIRM_SAMPLES = 1536;
    const TURN_SILENCE_SAMPLES = 10400;
    const WAKE_BUFFER_SAMPLES = 48000;

    function concatFrames(frames) {
        const length = frames.reduce((total, frame) => total + frame.length, 0);
        const output = new Float32Array(length);
        let offset = 0;
        frames.forEach(frame => {
            output.set(frame, offset);
            offset += frame.length;
        });
        return output;
    }

    class RealtimeAudioGate extends EventTarget {
        constructor(options) {
            super();
            this.options = Object.assign({ threshold: 0.5 }, options || {});
            this.vad = new window.AuraSileroVAD.SileroVAD({ threshold: this.options.threshold });
            this.context = null;
            this.stream = null;
            this.source = null;
            this.worklet = null;
            this.sink = null;
            this.queue = [];
            this.processing = false;
            this.preRoll = [];
            this.preRollSamples = 0;
            this.candidate = [];
            this.candidateSpeechSamples = 0;
            this.silenceSamples = 0;
            this.speaking = false;
            this.muted = false;
            this.started = false;
        }

        emit(type, detail) {
            this.dispatchEvent(new CustomEvent(type, { detail: detail || {} }));
        }

        async start() {
            if (this.started) return;
            if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
                throw new Error('Microphone access is not supported by this browser');
            }
            await this.vad.load();
            this.stream = await navigator.mediaDevices.getUserMedia({
                audio: {
                    channelCount: 1,
                    echoCancellation: true,
                    noiseSuppression: true,
                    autoGainControl: true
                },
                video: false
            });
            const AudioContextClass = window.AudioContext || window.webkitAudioContext;
            this.context = new AudioContextClass({ latencyHint: 'interactive' });
            await this.context.audioWorklet.addModule('/js/realtime-speech/audio-worklet.js');
            this.source = this.context.createMediaStreamSource(this.stream);
            this.worklet = new AudioWorkletNode(this.context, 'aurago-realtime-speech', {
                numberOfInputs: 1,
                numberOfOutputs: 1,
                outputChannelCount: [1]
            });
            this.sink = this.context.createGain();
            this.sink.gain.value = 0;
            this.worklet.port.onmessage = event => {
                const frame = event.data instanceof Float32Array ? event.data : new Float32Array(event.data);
                this.enqueue(frame);
            };
            this.source.connect(this.worklet);
            this.worklet.connect(this.sink);
            this.sink.connect(this.context.destination);
            if (this.context.state === 'suspended') await this.context.resume();
            this.started = true;
            this.emit('ready', { sampleRate: SAMPLE_RATE });
        }

        enqueue(frame) {
            if (!this.started || !(frame instanceof Float32Array) || frame.length !== CHUNK_SAMPLES) return;
            this.queue.push(frame);
            if (!this.processing) void this.drain();
        }

        async drain() {
            this.processing = true;
            while (this.queue.length) {
                const frame = this.queue.shift();
                try {
                    await this.processFrame(frame);
                } catch (error) {
                    this.emit('error', { error });
                }
            }
            this.processing = false;
        }

        appendPreRoll(frame) {
            this.preRoll.push(frame.slice());
            this.preRollSamples += frame.length;
            while (this.preRollSamples > PRE_ROLL_SAMPLES && this.preRoll.length > 1) {
                const removed = this.preRoll.shift();
                this.preRollSamples -= removed.length;
            }
        }

        async processFrame(frame) {
            const speech = !this.muted && await this.vad.isSpeech(frame);
            if (!this.speaking) {
                if (!speech) {
                    this.candidate = [];
                    this.candidateSpeechSamples = 0;
                    this.appendPreRoll(frame);
                    this.emit('level', { speech: false, probability: 0 });
                    return;
                }
                if (this.candidate.length === 0) this.candidate = this.preRoll.map(item => item.slice());
                this.candidate.push(frame.slice());
                this.candidateSpeechSamples += frame.length;
                this.emit('level', { speech: true, probability: 1 });
                if (this.candidateSpeechSamples < CONFIRM_SAMPLES) return;

                this.speaking = true;
                this.silenceSamples = 0;
                const startAudio = concatFrames(this.candidate);
                this.candidate = [];
                this.candidateSpeechSamples = 0;
                this.preRoll = [];
                this.preRollSamples = 0;
                this.emit('speechstart', {
                    audio: startAudio,
                    preRollSamples: Math.min(PRE_ROLL_SAMPLES, Math.max(0, startAudio.length - CONFIRM_SAMPLES))
                });
                return;
            }

            this.emit('audio', { audio: frame.slice(), speech });
            if (speech) {
                this.silenceSamples = 0;
                return;
            }
            this.silenceSamples += frame.length;
            if (this.silenceSamples < TURN_SILENCE_SAMPLES) return;
            this.speaking = false;
            this.silenceSamples = 0;
            this.preRoll = [frame.slice()];
            this.preRollSamples = frame.length;
            this.emit('speechend', {});
        }

        setMuted(muted) {
            this.muted = !!muted;
            if (this.muted && this.speaking) {
                this.speaking = false;
                this.silenceSamples = 0;
                this.emit('speechend', { muted: true });
            }
            this.emit('mutechange', { muted: this.muted });
        }

        async stop() {
            this.started = false;
            this.queue = [];
            this.processing = false;
            if (this.worklet) {
                this.worklet.port.onmessage = null;
                try { this.worklet.disconnect(); } catch (_) { }
            }
            if (this.source) {
                try { this.source.disconnect(); } catch (_) { }
            }
            if (this.sink) {
                try { this.sink.disconnect(); } catch (_) { }
            }
            if (this.stream) this.stream.getTracks().forEach(track => track.stop());
            if (this.context && this.context.state !== 'closed') {
                try { await this.context.close(); } catch (_) { }
            }
            this.stream = null;
            this.context = null;
            this.source = null;
            this.worklet = null;
            this.sink = null;
            this.preRoll = [];
            this.candidate = [];
            this.preRollSamples = 0;
            this.candidateSpeechSamples = 0;
            this.silenceSamples = 0;
            this.speaking = false;
            this.vad.reset();
            this.emit('stopped', {});
        }
    }

    window.AuraRealtimeAudio = {
        RealtimeAudioGate,
        constants: {
            SAMPLE_RATE,
            CHUNK_SAMPLES,
            PRE_ROLL_SAMPLES,
            CONFIRM_SAMPLES,
            TURN_SILENCE_SAMPLES,
            WAKE_BUFFER_SAMPLES
        },
        concatFrames
    };
})();
