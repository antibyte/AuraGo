// AuraGo Audio Player Component
// Handles inline audio playback with speed control

class ChatAudioPlayer {
    constructor(url, duration = 0) {
        this.url = url;
        this.duration = duration;
        this.currentTime = 0;
        this.isPlaying = false;
        this.playbackRate = 1.0;
        this.audio = new Audio(url);
        this.element = null;
        this.progressInterval = null;
        
        this.init();
    }

    init() {
        this.audio.addEventListener('loadedmetadata', () => {
            this.duration = this.audio.duration;
            this.updateTimeDisplay();
        });

        this.audio.addEventListener('ended', () => {
            this.isPlaying = false;
            this.currentTime = 0;
            this.updatePlayButton();
            this.stopProgressUpdate();
        });

        this.audio.addEventListener('error', (e) => {
            console.error('Audio error:', e);
            this.showError();
        });

        this.element = this.createElement();
    }

    createElement() {
        const container = document.createElement('div');
        container.className = 'chat-audio-player';
        
        container.innerHTML = `
            <button class="audio-play-btn" title="Play/Pause (Space)">
                <span class="play-icon">▶</span>
                <span class="pause-icon" style="display:none">⏸</span>
            </button>
            <div class="audio-progress-container">
                <div class="audio-progress-bar">
                    <div class="audio-progress-fill"></div>
                    <div class="audio-progress-handle"></div>
                </div>
            </div>
            <div class="audio-time">
                <span class="current">0:00</span>
                <span class="separator">/</span>
                <span class="total">${this.formatTime(this.duration)}</span>
            </div>
            <button class="audio-speed-btn" title="Playback speed">1.0x</button>
            <button class="audio-download-btn" title="Download">⬇</button>
        `;

        // Event listeners
        const playBtn = container.querySelector('.audio-play-btn');
        playBtn.addEventListener('click', () => this.togglePlay());

        const progressBar = container.querySelector('.audio-progress-bar');
        progressBar.addEventListener('click', (e) => this.seek(e));

        const speedBtn = container.querySelector('.audio-speed-btn');
        speedBtn.addEventListener('click', () => this.cycleSpeed());

        const downloadBtn = container.querySelector('.audio-download-btn');
        downloadBtn.addEventListener('click', () => this.download());

        // Keyboard shortcuts
        container.addEventListener('keydown', (e) => {
            if (e.code === 'Space') {
                e.preventDefault();
                this.togglePlay();
            } else if (e.code === 'ArrowLeft') {
                e.preventDefault();
                this.seekBackward();
            } else if (e.code === 'ArrowRight') {
                e.preventDefault();
                this.seekForward();
            }
        });

        container.tabIndex = 0;

        return container;
    }

    togglePlay() {
        if (this.isPlaying) {
            this.pause();
        } else {
            this.play();
        }
    }

    play() {
        this.audio.play().then(() => {
            this.isPlaying = true;
            this.updatePlayButton();
            this.startProgressUpdate();
        }).catch(err => {
            console.error('Play failed:', err);
        });
    }

    pause() {
        this.audio.pause();
        this.isPlaying = false;
        this.updatePlayButton();
        this.stopProgressUpdate();
    }

    seek(e) {
        const rect = e.currentTarget.getBoundingClientRect();
        const percent = (e.clientX - rect.left) / rect.width;
        const time = percent * this.duration;
        
        this.audio.currentTime = time;
        this.currentTime = time;
        this.updateProgress();
    }

    seekForward(seconds = 10) {
        this.audio.currentTime = Math.min(this.audio.currentTime + seconds, this.duration);
    }

    seekBackward(seconds = 10) {
        this.audio.currentTime = Math.max(this.audio.currentTime - seconds, 0);
    }

    cycleSpeed() {
        const speeds = [0.5, 0.75, 1.0, 1.25, 1.5, 2.0];
        const currentIndex = speeds.indexOf(this.playbackRate);
        const nextIndex = (currentIndex + 1) % speeds.length;
        
        this.playbackRate = speeds[nextIndex];
        this.audio.playbackRate = this.playbackRate;
        
        const speedBtn = this.element.querySelector('.audio-speed-btn');
        speedBtn.textContent = this.playbackRate.toFixed(2).replace('.00', '') + 'x';
    }

    startProgressUpdate() {
        this.stopProgressUpdate();
        this.progressInterval = setInterval(() => {
            this.currentTime = this.audio.currentTime;
            this.updateProgress();
            this.updateTimeDisplay();
        }, 100);
    }

    stopProgressUpdate() {
        if (this.progressInterval) {
            clearInterval(this.progressInterval);
            this.progressInterval = null;
        }
    }

    updateProgress() {
        const percent = this.duration > 0 ? (this.currentTime / this.duration) * 100 : 0;
        const fill = this.element.querySelector('.audio-progress-fill');
        const handle = this.element.querySelector('.audio-progress-handle');
        
        if (fill) fill.style.width = percent + '%';
        if (handle) handle.style.left = percent + '%';
    }

    updateTimeDisplay() {
        const currentEl = this.element.querySelector('.audio-time .current');
        const totalEl = this.element.querySelector('.audio-time .total');
        
        if (currentEl) currentEl.textContent = this.formatTime(this.currentTime);
        if (totalEl) totalEl.textContent = this.formatTime(this.duration);
    }

    updatePlayButton() {
        const playIcon = this.element.querySelector('.play-icon');
        const pauseIcon = this.element.querySelector('.pause-icon');
        
        if (this.isPlaying) {
            playIcon.style.display = 'none';
            pauseIcon.style.display = 'inline';
        } else {
            playIcon.style.display = 'inline';
            pauseIcon.style.display = 'none';
        }
    }

    formatTime(seconds) {
        if (isNaN(seconds)) return '0:00';
        const mins = Math.floor(seconds / 60);
        const secs = Math.floor(seconds % 60);
        return `${mins}:${secs.toString().padStart(2, '0')}`;
    }

    download() {
        const a = document.createElement('a');
        a.href = this.url;
        a.download = 'audio-message.mp3';
        a.click();
    }

    showError() {
        this.element.innerHTML = `
            <div class="audio-error">
                ❌ Failed to load audio
                <button onclick="this.parentElement.parentElement.remove()">Dismiss</button>
            </div>
        `;
    }

    destroy() {
        this.stopProgressUpdate();
        this.pause();
        this.audio = null;
    }
}

// Factory function for easy integration
function createAudioPlayer(url, duration) {
    const player = new ChatAudioPlayer(url, duration);
    return player.element;
}

// Export for module usage if needed
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { ChatAudioPlayer, createAudioPlayer };
}
