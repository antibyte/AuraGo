/**
 * Local Speech Lab recorder. Audio stays in the browser until the user sends
 * it, then a bounded mono PCM16/16 kHz WAV is uploaded to AuraGo.
 */
(function() {
    'use strict';

    const MAX_DURATION_MS = 120000;
    const TARGET_SAMPLE_RATE = 16000;
    const MAX_WAV_BYTES = 8 * 1024 * 1024;

    const START_RECORDING = 'recording';
    const START_BROWSER = 'browser';
    const START_FAILED = 'failed';
    const START_BUSY = 'busy';

    const SpeechLabRecorder = {
        isSupported: !!(window.AudioContext && window.AudioWorkletNode && navigator.mediaDevices &&
            typeof navigator.mediaDevices.getUserMedia === 'function'),
        state: 'idle',
        status: null,
        statusRequestOK: false,
        statusRequestID: 0,
        chunks: [],
        totalFrames: 0,
        onTranscription: () => {},
        onError: () => {},

        get isRecording() {
            return this.state === 'recording';
        },

        get isTransitioning() {
            return this.state === 'starting' || this.state === 'stopping';
        },

        init(options = {}) {
            this.onTranscription = options.onTranscription || (() => {});
            this.onError = options.onError || (() => {});
            this._createUI();
        },

        _t(key, fallback) {
            if (typeof I18N !== 'undefined' && I18N['chat.' + key]) {
                return I18N['chat.' + key];
            }
            return fallback || key;
        },

        async refreshStatus() {
            const requestID = ++this.statusRequestID;
            try {
                const response = await fetch('/api/speech-lab/status', { headers: { Accept: 'application/json' } });
                if (!response.ok) {
                    if (requestID === this.statusRequestID) this.statusRequestOK = false;
                    return null;
                }
                const status = await response.json();
                if (requestID === this.statusRequestID) {
                    this.status = status;
                    this.statusRequestOK = true;
                }
                return status;
            } catch (_) {
                if (requestID === this.statusRequestID) this.statusRequestOK = false;
                return null;
            }
        },

        async selected() {
            const status = await this.refreshStatus();
            return this._statusSelectsSpeechLab(status);
        },

        async start() {
            if (this.state === 'recording') return START_RECORDING;
            if (this.state !== 'idle') return START_BUSY;

            this.state = 'starting';
            const previouslySelected = this._statusSelectsSpeechLab(this.status);
            let initialResume = Promise.resolve(null);
            let contextError = null;

            // Creating and resuming the context before the first await preserves
            // the browser's user activation from the microphone button click.
            if (this.isSupported) {
                try {
                    this.audioContext = new AudioContext();
                    initialResume = this._ensureAudioContextRunning().then(() => null, error => error);
                } catch (error) {
                    contextError = error;
                }
            }

            const status = await this.refreshStatus();
            const resumeError = await initialResume;
            if (!status) {
                await this._finishStartWithoutRecording();
                if (previouslySelected) {
                    this._fail(this._t('speech_lab_asr_not_ready', 'Speech Lab ASR is not ready. Check Media → Speech Lab.'));
                    return START_FAILED;
                }
                return START_BROWSER;
            }
            if (!this._statusSelectsSpeechLab(status)) {
                await this._finishStartWithoutRecording();
                return START_BROWSER;
            }
            if (!this.isSupported || contextError || !this.audioContext || !this.audioContext.audioWorklet ||
                typeof this.audioContext.audioWorklet.addModule !== 'function') {
                await this._finishStartWithoutRecording();
                return START_BROWSER;
            }
            if (!status.asr_ok) {
                await this._finishStartWithoutRecording();
                this._fail(this._t('speech_lab_asr_not_ready', 'Speech Lab ASR is not ready. Check Media → Speech Lab.'));
                return START_FAILED;
            }
            try {
                if (resumeError) throw resumeError;
                this.stream = await navigator.mediaDevices.getUserMedia({
                    audio: { echoCancellation: true, noiseSuppression: true, autoGainControl: true, channelCount: 1 }
                });
                await this.audioContext.audioWorklet.addModule('/js/chat/modules/speech-lab-worklet.js');
                this.source = this.audioContext.createMediaStreamSource(this.stream);
                this.node = new AudioWorkletNode(this.audioContext, 'aurago-speech-lab-recorder');
                this.silence = this.audioContext.createGain();
                this.silence.gain.value = 0;
                this.chunks = [];
                this.totalFrames = 0;
                this.node.port.onmessage = (event) => {
                    if (this.state !== 'recording' || !(event.data instanceof Float32Array)) return;
                    this.chunks.push(event.data);
                    this.totalFrames += event.data.length;
                };
                await this._ensureAudioContextRunning();
                this.state = 'recording';
                this.source.connect(this.node);
                this.node.connect(this.silence);
                this.silence.connect(this.audioContext.destination);
                this.startedAt = Date.now();
                this._showUI();
                this.timer = setInterval(() => this._updateTimer(), 250);
                this.limitTimer = setTimeout(() => this.send(), MAX_DURATION_MS);
                return START_RECORDING;
            } catch (error) {
                this.state = 'stopping';
                await this._cleanup();
                this.state = 'idle';
                this._fail(error && error.name === 'NotAllowedError'
                    ? this._t('speech_lab_microphone_denied', 'Microphone permission was denied.')
                    : this._t('speech_lab_recorder_start_failed', 'Speech Lab could not start the microphone recorder.'));
                return START_FAILED;
            }
        },

        async send() {
            if (this.state !== 'recording') return false;
            this.state = 'stopping';
            const sourceRate = this.audioContext ? this.audioContext.sampleRate : TARGET_SAMPLE_RATE;
            const samples = this._mergeChunks();
            await this._cleanup();
            this.state = 'idle';
            if (!samples.length) {
                this._fail(this._t('speech_lab_no_audio', 'No speech audio was recorded.'));
                return false;
            }
            const resampled = this._resample(samples, sourceRate, TARGET_SAMPLE_RATE);
            const wav = this._encodeWAV(resampled, TARGET_SAMPLE_RATE);
            if (wav.size > MAX_WAV_BYTES) {
                this._fail(this._t('speech_lab_too_large', 'Speech Lab recording exceeds 8 MiB. Record a shorter message.'));
                return false;
            }
            const form = new FormData();
            form.append('audio', wav, 'speech-lab.wav');
            try {
                const headers = {};
                const sessionID = typeof window.activeSessionId === 'function' ? window.activeSessionId() : '';
                if (sessionID && sessionID !== 'default') headers['X-Session-ID'] = sessionID;
                const response = await fetch('/api/upload-voice', { method: 'POST', headers, body: form });
                if (!response.ok) {
                    const contentType = response.headers.get('content-type') || '';
                    const payload = contentType.includes('application/json') ? await response.json().catch(() => ({})) : {};
                    if (payload.error === 'speech_lab_no_speech') {
                        throw new Error(this._t('speech_lab_no_audio', 'No speech was detected. Please record again.'));
                    }
                    throw new Error(payload.message || ('HTTP ' + response.status + ': ' + this._t('speech_lab_transcription_failed', 'Speech Lab transcription failed.')));
                }
                const payload = await response.json();
                this.onTranscription(payload.transcription || '', payload.speech_lab_turn_token || '');
                return true;
            } catch (error) {
                this._fail(error.message || this._t('speech_lab_transcription_failed', 'Speech Lab transcription failed.'));
                return false;
            }
        },

        async cancel() {
            this.state = 'stopping';
            this.chunks = [];
            this.totalFrames = 0;
            await this._cleanup();
            this.state = 'idle';
        },

        _statusSelectsSpeechLab(status) {
            return !!(status && status.enabled && status.chat_input_enabled);
        },

        async _ensureAudioContextRunning() {
            if (!this.audioContext) throw new Error('speech_lab_audio_context_missing');
            if (this.audioContext.state === 'suspended' || this.audioContext.state === 'interrupted') {
                await this.audioContext.resume();
            }
            if (this.audioContext.state !== 'running') {
                throw new Error('speech_lab_audio_context_not_running');
            }
        },

        async _finishStartWithoutRecording() {
            this.state = 'stopping';
            await this._cleanup();
            this.state = 'idle';
        },

        _mergeChunks() {
            const merged = new Float32Array(this.totalFrames);
            let offset = 0;
            for (const chunk of this.chunks) {
                merged.set(chunk, offset);
                offset += chunk.length;
            }
            this.chunks = [];
            this.totalFrames = 0;
            return merged;
        },

        _resample(input, sourceRate, targetRate) {
            if (sourceRate === targetRate) return input;
            const length = Math.max(1, Math.round(input.length * targetRate / sourceRate));
            const output = new Float32Array(length);
            const ratio = sourceRate / targetRate;
            for (let i = 0; i < length; i++) {
                const position = i * ratio;
                const left = Math.floor(position);
                const right = Math.min(left + 1, input.length - 1);
                const mix = position - left;
                output[i] = input[left] * (1 - mix) + input[right] * mix;
            }
            return output;
        },

        _encodeWAV(samples, sampleRate) {
            const buffer = new ArrayBuffer(44 + samples.length * 2);
            const view = new DataView(buffer);
            const text = (offset, value) => {
                for (let i = 0; i < value.length; i++) view.setUint8(offset + i, value.charCodeAt(i));
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
            for (let i = 0; i < samples.length; i++) {
                const value = Math.max(-1, Math.min(1, samples[i]));
                view.setInt16(44 + i * 2, value < 0 ? value * 32768 : value * 32767, true);
            }
            return new Blob([buffer], { type: 'audio/wav' });
        },

        _createUI() {
            if (this.overlay) return;
            this.overlay = document.createElement('div');
            this.overlay.className = 'voice-recorder-overlay';
            this.overlay.style.display = 'none';
            this.overlay.innerHTML = `
                <div class="voice-recorder-panel">
                    <div class="vr-header"><div class="vr-pulse"></div><span>Speech Lab</span><span class="vr-timer">00:00</span></div>
                    <p class="vr-status">${this._t('speech_lab_local_recording', 'Local PCM/WAV recording')}</p>
                    <div class="vr-controls">
                        <button class="vr-btn vr-cancel" type="button" aria-label="${this._t('voice_cancel', 'Cancel')}">${window.chatUiIconMarkup ? window.chatUiIconMarkup('close') : '×'}</button>
                        <button class="vr-btn vr-send" type="button" aria-label="${this._t('speech_lab_transcribe', 'Transcribe')}">${window.chatUiIconMarkup ? window.chatUiIconMarkup('send') : '✓'}</button>
                    </div>
                    <a href="/config#speech_lab">${this._t('speech_lab_settings', 'Speech Lab settings')}</a>
                </div>`;
            document.body.appendChild(this.overlay);
            this.timerElement = this.overlay.querySelector('.vr-timer');
            this.statusElement = this.overlay.querySelector('.vr-status');
            this.sendButton = this.overlay.querySelector('.vr-send');
            this.overlay.querySelector('.vr-cancel').addEventListener('click', () => this.cancel());
            this.sendButton.addEventListener('click', () => this.send());
        },

        _showUI() {
            if (this.statusElement) this.statusElement.textContent = this._t('speech_lab_local_recording', 'Local PCM/WAV recording');
            if (this.sendButton) this.sendButton.style.display = '';
            this.overlay.style.display = 'flex';
            document.body.style.overflow = 'hidden';
        },

        _updateTimer() {
            const seconds = Math.min(120, Math.floor((Date.now() - this.startedAt) / 1000));
            this.timerElement.textContent = `${String(Math.floor(seconds / 60)).padStart(2, '0')}:${String(seconds % 60).padStart(2, '0')}`;
        },

        async _cleanup() {
            clearInterval(this.timer);
            clearTimeout(this.limitTimer);
            if (this.node) try { this.node.disconnect(); } catch (_) {}
            if (this.source) try { this.source.disconnect(); } catch (_) {}
            if (this.silence) try { this.silence.disconnect(); } catch (_) {}
            if (this.stream) this.stream.getTracks().forEach(track => track.stop());
            if (this.audioContext) await this.audioContext.close().catch(() => {});
            this.node = this.source = this.silence = this.stream = this.audioContext = null;
            if (this.overlay) this.overlay.style.display = 'none';
            document.body.style.overflow = '';
        },

        _fail(message) {
            if (this.overlay) {
                if (this.statusElement) this.statusElement.textContent = message;
                if (this.sendButton) this.sendButton.style.display = 'none';
                this.overlay.style.display = 'flex';
                document.body.style.overflow = 'hidden';
            }
            this.onError(message);
        }
    };

    window.SpeechLabRecorder = SpeechLabRecorder;
})();
