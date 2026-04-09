// AuraGo Voice Recorder Component
// Handles voice recording with live waveform visualization

class VoiceRecorder {
    constructor() {
        this.isRecording = false;
        this.isPaused = false;
        this.mediaRecorder = null;
        this.audioChunks = [];
        this.startTime = 0;
        this.pauseTime = 0;
        this.totalPausedDuration = 0;
        this.stream = null;
        this.analyser = null;
        this.dataArray = null;
        this.animationId = null;
        this.maxDuration = 5 * 60 * 1000; // 5 minutes
        
        this.element = null;
        this.onSend = null;
        this.onCancel = null;
    }

    createUI() {
        const container = document.createElement('div');
        container.className = 'voice-recorder';
        container.classList.add('is-hidden');
        
        container.innerHTML = `
            <div class="recorder-inner">
                <div class="recording-indicator">
                    <div class="recorder-pulse"></div>
                    <span class="recorder-status">${t('chat.voice_recording')}</span>
                </div>
                <div class="recorder-timer">00:00</div>
                <canvas class="recorder-waveform" width="200" height="40"></canvas>
                <div class="recorder-controls">
                    <button class="recorder-btn recorder-pause" title="${t('chat.voice_pause_resume')}">
                        <span class="pause-icon">⏸</span>
                        <span class="resume-icon is-hidden">▶</span>
                    </button>
                    <button class="recorder-btn recorder-cancel" title="${t('chat.voice_cancel')}">✕</button>
                    <button class="recorder-btn recorder-send" title="${t('chat.voice_send')}">➤</button>
                </div>
            </div>
        `;

        // Event listeners
        container.querySelector('.recorder-pause').addEventListener('click', () => this.togglePause());
        container.querySelector('.recorder-cancel').addEventListener('click', () => this.cancel());
        container.querySelector('.recorder-send').addEventListener('click', () => this.send());

        this.element = container;
        this.canvas = container.querySelector('.recorder-waveform');
        this.canvasCtx = this.canvas.getContext('2d');
        this.timerEl = container.querySelector('.recorder-timer');
        this.statusEl = container.querySelector('.recorder-status');
        
        return container;
    }

    async start() {
        try {
            this.stream = await navigator.mediaDevices.getUserMedia({ 
                audio: {
                    echoCancellation: true,
                    noiseSuppression: true,
                    sampleRate: 44100
                } 
            });
            
            this.setupAudioAnalysis();
            this.setupRecorder();
            
            this.isRecording = true;
            this.isPaused = false;
            this.startTime = Date.now();
            this.totalPausedDuration = 0;
            this.audioChunks = [];
            
            this.mediaRecorder.start(100); // Collect data every 100ms
            this.startTimer();
            this.startVisualization();
            
            this.updateUI();
            this.show();
            
            // Auto-stop after max duration
            this.maxDurationTimer = setTimeout(() => {
                if (this.isRecording) {
                    this.send();
                }
            }, this.maxDuration);
            
        } catch (err) {
            console.error('Failed to start recording:', err);
            await this.showError(t('chat.voice_mic_denied'));
        }
    }

    setupAudioAnalysis() {
        const audioContext = new (window.AudioContext || window.webkitAudioContext)();
        this.audioContext = audioContext;
        const source = audioContext.createMediaStreamSource(this.stream);
        
        this.analyser = audioContext.createAnalyser();
        this.analyser.fftSize = 256;
        this.analyser.smoothingTimeConstant = 0.8;
        
        source.connect(this.analyser);
        
        const bufferLength = this.analyser.frequencyBinCount;
        this.dataArray = new Uint8Array(bufferLength);
    }

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
            console.error('MediaRecorder error:', e);
            this.showError(t('chat.voice_recording_error'));  // Fire-and-forget in callback is acceptable
        };
    }

    getSupportedMimeType() {
        const types = [
            'audio/webm;codecs=opus',
            'audio/webm',
            'audio/ogg;codecs=opus',
            'audio/ogg',
            'audio/mp4',
            'audio/wav'
        ];
        
        for (const type of types) {
            if (MediaRecorder.isTypeSupported(type)) {
                return type;
            }
        }
        
        return '';
    }

    togglePause() {
        if (this.isPaused) {
            this.resume();
        } else {
            this.pause();
        }
    }

    pause() {
        if (!this.isRecording || this.isPaused) return;
        
        this.mediaRecorder.pause();
        this.isPaused = true;
        this.pauseTime = Date.now();
        
        this.stopVisualization();
        this.updateUI();
    }

    resume() {
        if (!this.isRecording || !this.isPaused) return;
        
        this.mediaRecorder.resume();
        this.isPaused = false;
        this.totalPausedDuration += Date.now() - this.pauseTime;
        
        this.startVisualization();
        this.updateUI();
    }

    stopStream() {
        if (this.stream) {
            this.stream.getTracks().forEach(track => track.stop());
            this.stream = null;
        }
        
        if (this.analyser) {
            this.analyser = null;
        }
    }

    startTimer() {
        this.timerInterval = setInterval(() => {
            if (!this.isPaused) {
                const elapsed = Date.now() - this.startTime - this.totalPausedDuration;
                this.timerEl.textContent = this.formatTime(elapsed);
            }
        }, 1000);
    }

    stopTimer() {
        if (this.timerInterval) {
            clearInterval(this.timerInterval);
            this.timerInterval = null;
        }
        
        if (this.maxDurationTimer) {
            clearTimeout(this.maxDurationTimer);
            this.maxDurationTimer = null;
        }
    }

    startVisualization() {
        const draw = () => {
            if (!this.analyser || this.isPaused) return;
            
            this.animationId = requestAnimationFrame(draw);
            
            this.analyser.getByteFrequencyData(this.dataArray);
            
            const canvas = this.canvas;
            const ctx = this.canvasCtx;
            const width = canvas.width;
            const height = canvas.height;
            
            ctx.clearRect(0, 0, width, height);
            
            const barCount = 30;
            const barWidth = width / barCount;
            const step = Math.floor(this.dataArray.length / barCount);
            
            for (let i = 0; i < barCount; i++) {
                const value = this.dataArray[i * step];
                const percent = value / 255;
                const barHeight = percent * height * 0.8;
                
                const x = i * barWidth;
                const y = (height - barHeight) / 2;
                
                // Gradient based on amplitude
                const gradient = ctx.createLinearGradient(0, y, 0, y + barHeight);
                gradient.addColorStop(0, 'var(--accent)');
                gradient.addColorStop(1, 'var(--accent-secondary, #0d9488)');
                
                ctx.fillStyle = gradient;
                ctx.fillRect(x + 1, y, barWidth - 2, barHeight);
            }
        };
        
        draw();
    }

    stopVisualization() {
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
            this.animationId = null;
        }
        
        // Clear canvas
        const canvas = this.canvas;
        const ctx = this.canvasCtx;
        ctx.clearRect(0, 0, canvas.width, canvas.height);
    }

    updateUI() {
        const pauseBtn = this.element.querySelector('.recorder-pause');
        const pauseIcon = pauseBtn.querySelector('.pause-icon');
        const resumeIcon = pauseBtn.querySelector('.resume-icon');
        
        if (this.isPaused) {
            pauseIcon.classList.add('is-hidden');
            resumeIcon.classList.remove('is-hidden');
            this.statusEl.textContent = t('chat.voice_paused');
            this.element.classList.add('paused');
        } else {
            pauseIcon.classList.remove('is-hidden');
            resumeIcon.classList.add('is-hidden');
            this.statusEl.textContent = t('chat.voice_recording');
            this.element.classList.remove('paused');
        }
    }

    async send() {
        if (!this.isRecording) return;
        
        this.stopTimer();
        this.stopVisualization();
        
        return new Promise((resolve) => {
            this.mediaRecorder.onstop = () => {
                this.stopStream();
                
                const mimeType = this.mediaRecorder.mimeType || 'audio/webm';
                const blob = new Blob(this.audioChunks, { type: mimeType });
                
                if (this.onSend) {
                    this.onSend(blob, mimeType);
                }
                
                this.reset();
                resolve();
            };
            
            this.mediaRecorder.stop();
        });
    }

    cancel() {
        if (!this.isRecording) return;
        
        this.stopTimer();
        this.stopVisualization();
        this.stopStream();
        
        if (this.mediaRecorder && this.mediaRecorder.state !== 'inactive') {
            this.mediaRecorder.stop();
        }
        
        if (this.onCancel) {
            this.onCancel();
        }
        
        this.reset();
    }

    reset() {
        this.isRecording = false;
        this.isPaused = false;
        this.audioChunks = [];
        if (this.audioContext && this.audioContext.state !== 'closed') {
            this.audioContext.close();
            this.audioContext = null;
        }
        this.hide();
    }

    show() {
        if (this.element) {
            this.element.classList.remove('is-hidden');
            this.element.classList.add('active');
        }
    }

    hide() {
        if (this.element) {
            this.element.classList.add('is-hidden');
            this.element.classList.remove('active');
        }
    }

    formatTime(ms) {
        const totalSeconds = Math.floor(ms / 1000);
        const mins = Math.floor(totalSeconds / 60);
        const secs = totalSeconds % 60;
        return `${mins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
    }

    async showError(message) {
        // Could integrate with toast system
        console.error('VoiceRecorder:', message);
        await showAlert(t('chat.voice_error'), message);
    }
}

// Export for module usage
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { VoiceRecorder };
}
