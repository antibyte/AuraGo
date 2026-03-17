/**
 * Voice Recorder Module
 * Records audio with mobile-friendly UI
 */

(function() {
    'use strict';

    const VoiceRecorder = {
        isRecording: false,
        isPaused: false,
        mediaRecorder: null,
        audioChunks: [],
        startTime: 0,
        pauseTime: 0,
        totalPausedDuration: 0,
        stream: null,
        timerInterval: null,
        maxDuration: 5 * 60 * 1000, // 5 minutes
        
        // Callbacks
        onTranscription: null,
        onError: null,

        init(options = {}) {
            this.onTranscription = options.onTranscription || (() => {});
            this.onError = options.onError || (() => {});
            this.createUI();
        },

        createUI() {
            // Create overlay for recording UI
            this.overlay = document.createElement('div');
            this.overlay.className = 'voice-recorder-overlay';
            this.overlay.style.display = 'none';
            
            this.overlay.innerHTML = `
                <div class="voice-recorder-panel">
                    <div class="vr-header">
                        <div class="vr-pulse"></div>
                        <span class="vr-status">Recording...</span>
                        <span class="vr-timer">00:00</span>
                    </div>
                    <div class="vr-waveform">
                        <canvas width="300" height="60"></canvas>
                    </div>
                    <div class="vr-controls">
                        <button class="vr-btn vr-cancel" title="Cancel">
                            <span>✕</span>
                        </button>
                        <button class="vr-btn vr-send" title="Send">
                            <span>➤</span>
                        </button>
                    </div>
                </div>
            `;
            
            document.body.appendChild(this.overlay);
            
            // Bind events
            this.overlay.querySelector('.vr-cancel').addEventListener('click', () => this.cancel());
            this.overlay.querySelector('.vr-send').addEventListener('click', () => this.send());
            
            // Close on backdrop click
            this.overlay.addEventListener('click', (e) => {
                if (e.target === this.overlay) this.cancel();
            });
            
            this.canvas = this.overlay.querySelector('canvas');
            this.ctx = this.canvas.getContext('2d');
            this.timerEl = this.overlay.querySelector('.vr-timer');
            this.statusEl = this.overlay.querySelector('.vr-status');
        },

        async start() {
            // Check for browser support
            if (!navigator.mediaDevices || !navigator.mediaDevices.getUserMedia) {
                this.onError('Voice recording not supported in this browser');
                return;
            }

            try {
                this.stream = await navigator.mediaDevices.getUserMedia({
                    audio: {
                        echoCancellation: true,
                        noiseSuppression: true,
                        sampleRate: 44100
                    }
                });

                this.setupRecorder();
                this.isRecording = true;
                this.isPaused = false;
                this.startTime = Date.now();
                this.totalPausedDuration = 0;
                this.audioChunks = [];
                
                this.mediaRecorder.start(100);
                this.showUI();
                this.startTimer();
                
                // Auto-stop after max duration
                this.maxDurationTimer = setTimeout(() => {
                    if (this.isRecording) this.send();
                }, this.maxDuration);
                
            } catch (err) {
                console.error('[VoiceRecorder] Start failed:', err);
                this.onError(err.name === 'NotAllowedError' 
                    ? 'Microphone permission denied' 
                    : 'Could not access microphone');
            }
        },

        setupRecorder() {
            const mimeType = this.getSupportedMimeType();
            
            this.mediaRecorder = new MediaRecorder(this.stream, {
                mimeType: mimeType,
                audioBitsPerSecond: 128000
            });

            this.mediaRecorder.ondataavailable = (e) => {
                if (e.data.size > 0) {
                    this.audioChunks.push(e.data);
                }
            };

            this.mediaRecorder.onstop = () => {
                this.stopStream();
            };

            this.mediaRecorder.onerror = (e) => {
                console.error('[VoiceRecorder] Error:', e);
                this.onError('Recording error');
            };
        },

        getSupportedMimeType() {
            const types = [
                'audio/webm;codecs=opus',
                'audio/webm',
                'audio/ogg;codecs=opus',
                'audio/ogg',
                'audio/mp4'
            ];
            
            for (const type of types) {
                if (MediaRecorder.isTypeSupported(type)) {
                    return type;
                }
            }
            return '';
        },

        startTimer() {
            this.timerInterval = setInterval(() => {
                if (!this.isPaused) {
                    const elapsed = Date.now() - this.startTime - this.totalPausedDuration;
                    this.timerEl.textContent = this.formatTime(elapsed);
                }
            }, 1000);
        },

        stopTimer() {
            if (this.timerInterval) {
                clearInterval(this.timerInterval);
                this.timerInterval = null;
            }
            if (this.maxDurationTimer) {
                clearTimeout(this.maxDurationTimer);
                this.maxDurationTimer = null;
            }
        },

        showUI() {
            this.overlay.style.display = 'flex';
            document.body.style.overflow = 'hidden'; // Prevent background scroll on mobile
        },

        hideUI() {
            this.overlay.style.display = 'none';
            document.body.style.overflow = '';
        },

        async send() {
            if (!this.isRecording) return;
            
            this.stopTimer();
            
            return new Promise((resolve) => {
                this.mediaRecorder.onstop = () => {
                    this.stopStream();
                    
                    const mimeType = this.mediaRecorder.mimeType || 'audio/webm';
                    const blob = new Blob(this.audioChunks, { type: mimeType });
                    
                    // Upload for transcription
                    this.upload(blob);
                    
                    this.reset();
                    resolve();
                };
                
                this.mediaRecorder.stop();
            });
        },

        async upload(blob) {
            const formData = new FormData();
            formData.append('audio', blob, 'recording.webm');
            
            try {
                const res = await fetch('/api/upload-voice', {
                    method: 'POST',
                    body: formData
                });
                
                if (!res.ok) throw new Error('Upload failed');
                
                const data = await res.json();
                this.onTranscription(data.transcription);
                
            } catch (err) {
                console.error('[VoiceRecorder] Upload failed:', err);
                this.onError('Transcription failed');
            }
        },

        cancel() {
            if (!this.isRecording) return;
            
            this.stopTimer();
            this.stopStream();
            
            if (this.mediaRecorder && this.mediaRecorder.state !== 'inactive') {
                this.mediaRecorder.stop();
            }
            
            this.reset();
        },

        reset() {
            this.isRecording = false;
            this.isPaused = false;
            this.audioChunks = [];
            this.hideUI();
        },

        stopStream() {
            if (this.stream) {
                this.stream.getTracks().forEach(track => track.stop());
                this.stream = null;
            }
        },

        formatTime(ms) {
            const totalSeconds = Math.floor(ms / 1000);
            const mins = Math.floor(totalSeconds / 60);
            const secs = totalSeconds % 60;
            return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
        }
    };

    window.VoiceRecorder = VoiceRecorder;
})();
