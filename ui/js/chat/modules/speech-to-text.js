/**
 * Speech-to-Text Module
 * Uses the browser-native Web Speech API (SpeechRecognition) for real-time
 * streaming transcription with a fancy full-screen overlay.
 * Falls back silently when the API is unavailable (Firefox, older Safari).
 */

(function() {
    'use strict';

    const SpeechRecognitionAPI = window.SpeechRecognition || window.webkitSpeechRecognition;

    const SpeechToText = {
        isSupported: !!SpeechRecognitionAPI,
        isActive: false,

        // Internals
        _recognition: null,
        _stream: null,
        _audioCtx: null,
        _analyser: null,
        _dataArray: null,
        _animFrameId: null,
        _timerInterval: null,
        _startTime: 0,
        _prevSessionText: '',
        _finalTranscript: '',
        _interimTranscript: '',
        _intentionalStop: false,
        _finalized: false,
        _restartCount: 0,
        _maxRestarts: 50,

        // DOM
        _overlay: null,
        _transcriptEl: null,
        _timerEl: null,
        _statusEl: null,
        _rings: [],

        // Callbacks
        onInterimResult: null,
        onFinalResult: null,
        onEnd: null,
        onError: null,

        init(options = {}) {
            if (!this.isSupported) return;
            this.onInterimResult = options.onInterimResult || (() => {});
            this.onFinalResult = options.onFinalResult || (() => {});
            this.onEnd = options.onEnd || (() => {});
            this.onError = options.onError || (() => {});
            this._createOverlay();
        },

        _t(key) {
            if (typeof I18N !== 'undefined' && I18N['chat.' + key]) {
                return I18N['chat.' + key];
            }
            const fallback = {
                stt_listening: 'Listening...',
                stt_processing: 'Processing...',
                stt_tap_to_stop: 'Tap to stop',
                stt_no_speech: 'No speech detected',
                stt_error: 'Speech recognition error',
                stt_not_supported: 'Speech recognition not supported',
                stt_permission_denied: 'Microphone permission denied',
                stt_cancel: 'Cancel',
                stt_done: 'Done'
            };
            return fallback[key] || key;
        },

        _createOverlay() {
            this._overlay = document.createElement('div');
            this._overlay.className = 'stt-overlay';

            this._overlay.innerHTML = `
                <div class="stt-overlay-inner">
                    <div class="stt-mic-area">
                        <div class="stt-ring stt-ring-1"></div>
                        <div class="stt-ring stt-ring-2"></div>
                        <div class="stt-ring stt-ring-3"></div>
                        <div class="stt-ring stt-ring-4"></div>
                        <div class="stt-mic-icon">
                            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                                <path d="M12 1a4 4 0 0 0-4 4v6a4 4 0 0 0 8 0V5a4 4 0 0 0-4-4Z"/>
                                <path d="M19 10v1a7 7 0 0 1-14 0v-1"/>
                                <line x1="12" y1="19" x2="12" y2="23"/>
                                <line x1="8" y1="23" x2="16" y2="23"/>
                            </svg>
                        </div>
                    </div>
                    <div class="stt-status">${this._t('stt_listening')}</div>
                    <div class="stt-timer">00:00</div>
                    <div class="stt-transcript" aria-live="polite">
                        <span class="stt-transcript-final"></span><span class="stt-transcript-interim"></span><span class="stt-cursor"></span>
                    </div>
                    <div class="stt-lang-badge"></div>
                    <div class="stt-controls">
                        <button class="stt-btn stt-cancel" title="${this._t('stt_cancel')}">
                            <span>✕</span>
                        </button>
                        <button class="stt-btn stt-done" title="${this._t('stt_done')}">
                            <span>➤</span>
                        </button>
                    </div>
                </div>
            `;

            document.body.appendChild(this._overlay);

            this._transcriptEl = {
                final: this._overlay.querySelector('.stt-transcript-final'),
                interim: this._overlay.querySelector('.stt-transcript-interim')
            };
            this._timerEl = this._overlay.querySelector('.stt-timer');
            this._statusEl = this._overlay.querySelector('.stt-status');
            this._rings = Array.from(this._overlay.querySelectorAll('.stt-ring'));
            this._langBadge = this._overlay.querySelector('.stt-lang-badge');

            this._overlay.querySelector('.stt-cancel').addEventListener('click', () => this.cancel());
            this._overlay.querySelector('.stt-done').addEventListener('click', () => this.stop());
            this._overlay.addEventListener('click', (e) => {
                if (e.target === this._overlay) this.cancel();
            });
        },

        _detectLang() {
            // Use the page language attribute (e.g. "de", "en") and map to BCP-47
            const pageLang = document.documentElement.lang || '';
            const map = {
                de: 'de-DE', en: 'en-US', es: 'es-ES', fr: 'fr-FR',
                it: 'it-IT', pt: 'pt-BR', nl: 'nl-NL', pl: 'pl-PL',
                cs: 'cs-CZ', da: 'da-DK', el: 'el-GR', hi: 'hi-IN',
                ja: 'ja-JP', no: 'nb-NO', sv: 'sv-SE', zh: 'zh-CN'
            };
            return map[pageLang] || pageLang || '';
        },

        async start() {
            if (!this.isSupported) {
                this.onError(this._t('stt_not_supported'));
                return;
            }
            if (this.isActive) return;

            try {
                // Get mic stream for audio-reactive visuals
                this._stream = await navigator.mediaDevices.getUserMedia({
                    audio: { echoCancellation: true, noiseSuppression: true }
                });
                this._setupAudioAnalysis();
            } catch (err) {
                console.error('[SpeechToText] Mic access failed:', err);
                this.onError(err.name === 'NotAllowedError'
                    ? this._t('stt_permission_denied')
                    : this._t('stt_error'));
                return;
            }

            this._prevSessionText = '';
            this._finalTranscript = '';
            this._interimTranscript = '';
            this._intentionalStop = false;
            this._finalized = false;
            this._restartCount = 0;

            this._recognition = new SpeechRecognitionAPI();
            // Use continuous=false to avoid Chrome carrying over previous session
            // results into the new session's event.results after auto-restart.
            // We restart manually in onend when the user is still recording.
            this._recognition.continuous = false;
            this._recognition.interimResults = true;
            this._recognition.maxAlternatives = 1;

            const lang = this._detectLang();
            if (lang) {
                this._recognition.lang = lang;
                this._langBadge.textContent = lang;
                this._langBadge.style.display = '';
            } else {
                this._langBadge.style.display = 'none';
            }

            this._recognition.onresult = (event) => {
                // With continuous=false each session is standalone — no cross-session
                // result carry-over from Chrome, so iterating from 0 is safe and correct.
                let sessionFinal = '';
                let interim = '';
                for (let i = 0; i < event.results.length; i++) {
                    const transcript = event.results[i][0].transcript;
                    if (event.results[i].isFinal) {
                        sessionFinal += transcript;
                    } else {
                        interim += transcript;
                    }
                }
                // Add a space between accumulated previous sessions and the current one.
                const sep = (this._prevSessionText && sessionFinal) ? ' ' : '';
                this._finalTranscript = this._prevSessionText + sep + sessionFinal;
                this._interimTranscript = interim;
                this._updateTranscript();

                const combined = this._finalTranscript + interim;
                if (interim) {
                    this.onInterimResult(combined);
                } else {
                    this.onFinalResult(combined);
                }
            };

            this._recognition.onerror = (event) => {
                console.warn('[SpeechToText] Error:', event.error);
                if (event.error === 'not-allowed' || event.error === 'service-not-allowed') {
                    this.onError(this._t('stt_permission_denied'));
                    this._cleanup();
                } else if (event.error === 'no-speech') {
                    this._statusEl.textContent = this._t('stt_no_speech');
                    // Don't stop — Chrome fires this after silence, recognition continues
                } else if (event.error === 'aborted') {
                    // Intentional abort, do nothing
                } else {
                    this.onError(this._t('stt_error'));
                    this._cleanup();
                }
            };

            this._recognition.onend = () => {
                // Guard: _finalize() sets _finalized=true; ignore stale onend events.
                if (this._finalized) return;
                // Auto-restart when the user is still recording (continuous=false fires
                // onend after each utterance).
                if (!this._intentionalStop && this.isActive && this._restartCount < this._maxRestarts) {
                    this._restartCount++;
                    // Carry over everything (finals + any unfinalised interim) before restart.
                    const interimTrim = this._interimTranscript.trim();
                    const sep = (this._finalTranscript && interimTrim) ? ' ' : '';
                    this._prevSessionText = (this._finalTranscript + sep + interimTrim).trim();
                    this._interimTranscript = '';
                    try {
                        this._recognition.start();
                    } catch (e) {
                        console.warn('[SpeechToText] Restart failed:', e);
                        this._finalize();
                    }
                    return;
                }
                this._finalize();
            };

            try {
                this._recognition.start();
            } catch (e) {
                console.error('[SpeechToText] Start failed:', e);
                this.onError(this._t('stt_error'));
                this._stopStream();
                return;
            }

            this.isActive = true;
            this._startTime = Date.now();
            this._showOverlay();
            this._startTimer();
            this._startVisualization();
        },

        stop() {
            if (!this.isActive) return;
            this._intentionalStop = true;
            this._statusEl.textContent = this._t('stt_processing');
            if (this._recognition) {
                try { this._recognition.stop(); } catch (_) { /* ignore */ }
            }
            // _finalize will be called from onend
        },

        cancel() {
            if (!this.isActive) return;
            this._intentionalStop = true;
            this._finalized = true;  // Prevent _finalize() from running
            this._finalTranscript = '';
            this._interimTranscript = '';
            if (this._recognition) {
                try { this._recognition.abort(); } catch (_) { /* ignore */ }
            }
            this._cleanup();
            // Notify so callers can reset button state etc.
            this.onEnd('');
        },

        _finalize() {
            if (this._finalized) return;  // Guard against double-execution
            this._finalized = true;
            const interimTrim = this._interimTranscript.trim();
            const sep = (this._finalTranscript && interimTrim) ? ' ' : '';
            const text = (this._finalTranscript + sep + interimTrim).trim();
            this._cleanup();
            if (text) {
                this.onFinalResult(text);
            }
            this.onEnd(text);
        },

        _cleanup() {
            this.isActive = false;
            // Detach all handlers BEFORE aborting so stale Chrome callbacks
            // (e.g. a second onend) don't trigger _finalize() again.
            if (this._recognition) {
                this._recognition.onresult = null;
                this._recognition.onerror = null;
                this._recognition.onend = null;
                try { this._recognition.abort(); } catch (_) {}
            }
            this._stopTimer();
            this._stopVisualization();
            this._stopStream();
            this._hideOverlay();
            this._recognition = null;
        },

        _setupAudioAnalysis() {
            this._audioCtx = new (window.AudioContext || window.webkitAudioContext)();
            const source = this._audioCtx.createMediaStreamSource(this._stream);
            this._analyser = this._audioCtx.createAnalyser();
            this._analyser.fftSize = 256;
            this._analyser.smoothingTimeConstant = 0.85;
            source.connect(this._analyser);
            this._dataArray = new Uint8Array(this._analyser.frequencyBinCount);
        },

        _startVisualization() {
            const draw = () => {
                if (!this.isActive || !this._analyser) return;
                this._animFrameId = requestAnimationFrame(draw);

                this._analyser.getByteFrequencyData(this._dataArray);

                // Compute RMS level (0..1)
                let sum = 0;
                for (let i = 0; i < this._dataArray.length; i++) {
                    sum += this._dataArray[i];
                }
                const avg = sum / this._dataArray.length / 255;
                const level = Math.min(1, avg * 2.5); // amplify for visual effect

                // Modulate ring scale and opacity based on audio level
                for (let i = 0; i < this._rings.length; i++) {
                    const ring = this._rings[i];
                    const baseScale = 1 + (i + 1) * 0.25;
                    const boost = level * (0.15 + i * 0.08);
                    const scale = baseScale + boost;
                    const opacity = 0.15 + level * (0.4 - i * 0.08);
                    ring.style.transform = `translate(-50%, -50%) scale(${scale})`;
                    ring.style.opacity = Math.max(0.05, opacity);
                }
            };
            draw();
        },

        _stopVisualization() {
            if (this._animFrameId) {
                cancelAnimationFrame(this._animFrameId);
                this._animFrameId = null;
            }
        },

        _stopStream() {
            if (this._stream) {
                this._stream.getTracks().forEach(t => t.stop());
                this._stream = null;
            }
            if (this._audioCtx) {
                this._audioCtx.close().catch(() => {});
                this._audioCtx = null;
                this._analyser = null;
                this._dataArray = null;
            }
        },

        _updateTranscript() {
            if (this._transcriptEl) {
                this._transcriptEl.final.textContent = this._finalTranscript;
                this._transcriptEl.interim.textContent = this._interimTranscript;
            }
            // Auto-scroll transcript into view
            const container = this._overlay?.querySelector('.stt-transcript');
            if (container) container.scrollTop = container.scrollHeight;
        },

        _startTimer() {
            this._timerInterval = setInterval(() => {
                const elapsed = Date.now() - this._startTime;
                this._timerEl.textContent = this._formatTime(elapsed);
            }, 1000);
        },

        _stopTimer() {
            if (this._timerInterval) {
                clearInterval(this._timerInterval);
                this._timerInterval = null;
            }
        },

        _showOverlay() {
            this._overlay.classList.add('active');
            document.body.style.overflow = 'hidden';
            // Force reflow then trigger entrance animation
            this._overlay.offsetHeight; // eslint-disable-line no-unused-expressions
            this._overlay.classList.add('visible');
        },

        _hideOverlay() {
            if (!this._overlay) return;
            this._overlay.classList.remove('visible');
            // Wait for exit animation to complete
            setTimeout(() => {
                this._overlay.classList.remove('active');
                document.body.style.overflow = '';
                // Reset transcript display
                if (this._transcriptEl) {
                    this._transcriptEl.final.textContent = '';
                    this._transcriptEl.interim.textContent = '';
                }
                if (this._timerEl) this._timerEl.textContent = '00:00';
                if (this._statusEl) this._statusEl.textContent = this._t('stt_listening');
                // Reset rings
                this._rings.forEach(ring => {
                    ring.style.transform = '';
                    ring.style.opacity = '';
                });
            }, 300);
        },

        _formatTime(ms) {
            const totalSeconds = Math.floor(ms / 1000);
            const mins = Math.floor(totalSeconds / 60);
            const secs = totalSeconds % 60;
            return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
        }
    };

    window.SpeechToText = SpeechToText;
})();
