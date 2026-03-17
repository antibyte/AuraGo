/**
 * Drag & Drop Module
 * File upload with drag & drop and touch support for mobile
 */

(function() {
    'use strict';

    const DragDrop = {
        container: null,
        overlay: null,
        queuePanel: null,
        uploadQueue: [],
        isProcessing: false,
        maxFileSize: 32 * 1024 * 1024, // 32MB
        
        onUpload: null,
        onError: null,

        init(options = {}) {
            this.container = options.container || document.getElementById('chat-box');
            this.onUpload = options.onUpload || (() => {});
            this.onError = options.onError || console.error;
            
            if (!this.container) return;
            
            this.createOverlay();
            this.createQueuePanel();
            this.bindEvents();
        },

        createOverlay() {
            this.overlay = document.createElement('div');
            this.overlay.className = 'drag-drop-overlay';
            this.overlay.innerHTML = `
                <div class="dd-message">
                    <div class="dd-icon">📁</div>
                    <div class="dd-text">Drop files to upload</div>
                </div>
            `;
            document.body.appendChild(this.overlay);
        },

        createQueuePanel() {
            this.queuePanel = document.createElement('div');
            this.queuePanel.className = 'upload-queue';
            this.queuePanel.style.display = 'none';
            document.body.appendChild(this.queuePanel);
        },

        bindEvents() {
            // Prevent defaults
            ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
                document.body.addEventListener(eventName, (e) => this.preventDefaults(e), false);
            });

            // Highlight
            ['dragenter', 'dragover'].forEach(eventName => {
                document.body.addEventListener(eventName, () => this.highlight(), false);
            });

            ['dragleave', 'drop'].forEach(eventName => {
                document.body.addEventListener(eventName, (e) => this.unhighlight(e), false);
            });

            // Handle drop
            document.body.addEventListener('drop', (e) => this.handleDrop(e), false);
        },

        preventDefaults(e) {
            e.preventDefault();
            e.stopPropagation();
        },

        highlight() {
            this.overlay.classList.add('active');
        },

        unhighlight(e) {
            // Only unhighlight if leaving document, not entering a child element
            if (e.relatedTarget && document.body.contains(e.relatedTarget)) return;
            this.overlay.classList.remove('active');
        },

        handleDrop(e) {
            this.overlay.classList.remove('active');
            
            const dt = e.dataTransfer;
            const files = dt.files;
            
            if (files.length > 0) {
                this.addFiles(Array.from(files));
            }
        },

        addFiles(files) {
            const validFiles = files.filter(file => this.validateFile(file));
            
            validFiles.forEach(file => {
                this.uploadQueue.push({
                    file,
                    id: 'upload-' + Math.random().toString(36).substr(2, 9),
                    status: 'pending',
                    progress: 0
                });
            });

            this.renderQueue();
            this.processQueue();
        },

        validateFile(file) {
            if (file.size > this.maxFileSize) {
                this.onError(`File "${file.name}" too large (max 32MB)`);
                return false;
            }
            return true;
        },

        renderQueue() {
            if (this.uploadQueue.length === 0) {
                this.queuePanel.style.display = 'none';
                return;
            }

            this.queuePanel.style.display = 'block';
            
            const pending = this.uploadQueue.filter(f => f.status === 'pending').length;
            const uploading = this.uploadQueue.filter(f => f.status === 'uploading').length;
            const completed = this.uploadQueue.filter(f => f.status === 'complete').length;

            let html = `
                <div class="uq-header">
                    <span>${this.uploadQueue.length} files</span>
                    <button class="uq-close">✕</button>
                </div>
                <div class="uq-items">
            `;

            this.uploadQueue.forEach(item => {
                const icon = this.getFileIcon(item.file);
                html += `
                    <div class="uq-item ${item.status}" data-id="${item.id}">
                        <span class="uq-icon">${icon}</span>
                        <div class="uq-info">
                            <div class="uq-name" title="${item.file.name}">${item.file.name}</div>
                            <div class="uq-meta">
                                ${this.formatSize(item.file.size)} • ${item.status}
                            </div>
                            ${item.status === 'uploading' ? `
                                <div class="uq-progress">
                                    <div class="uq-bar">
                                        <div class="uq-fill" style="width: ${item.progress}%"></div>
                                    </div>
                                    <span>${item.progress}%</span>
                                </div>
                            ` : ''}
                        </div>
                        ${item.status === 'error' ? `
                            <button class="uq-retry" data-id="${item.id}">↻</button>
                        ` : ''}
                        <button class="uq-remove" data-id="${item.id}">✕</button>
                    </div>
                `;
            });

            html += '</div>';
            
            if (completed === this.uploadQueue.length) {
                html += `
                    <div class="uq-footer">
                        <span>✓ Complete</span>
                        <button class="uq-clear">Clear</button>
                    </div>
                `;
            }

            this.queuePanel.innerHTML = html;

            // Bind events
            this.queuePanel.querySelector('.uq-close')?.addEventListener('click', () => {
                this.queuePanel.style.display = 'none';
            });

            this.queuePanel.querySelector('.uq-clear')?.addEventListener('click', () => {
                this.uploadQueue = [];
                this.renderQueue();
            });

            this.queuePanel.querySelectorAll('.uq-retry').forEach(btn => {
                btn.addEventListener('click', () => this.retryItem(btn.dataset.id));
            });

            this.queuePanel.querySelectorAll('.uq-remove').forEach(btn => {
                btn.addEventListener('click', () => this.removeItem(btn.dataset.id));
            });
        },

        async processQueue() {
            if (this.isProcessing) return;
            this.isProcessing = true;

            while (this.uploadQueue.length > 0) {
                const item = this.uploadQueue.find(i => i.status === 'pending');
                if (!item) break;

                item.status = 'uploading';
                item.progress = 0;
                this.renderQueue();

                try {
                    await this.uploadFile(item);
                    item.status = 'complete';
                    item.progress = 100;
                } catch (err) {
                    item.status = 'error';
                }

                this.renderQueue();
            }

            this.isProcessing = false;
            
            // Auto-clear after delay if all complete
            const allComplete = this.uploadQueue.every(i => i.status === 'complete');
            if (allComplete) {
                setTimeout(() => {
                    this.uploadQueue = [];
                    this.renderQueue();
                }, 3000);
            }
        },

        uploadFile(item) {
            return new Promise((resolve, reject) => {
                const formData = new FormData();
                formData.append('file', item.file);

                const xhr = new XMLHttpRequest();
                
                xhr.upload.addEventListener('progress', (e) => {
                    if (e.lengthComputable) {
                        item.progress = Math.round((e.loaded / e.total) * 100);
                        this.updateProgress(item.id, item.progress);
                    }
                });

                xhr.addEventListener('load', () => {
                    if (xhr.status >= 200 && xhr.status < 300) {
                        try {
                            const response = JSON.parse(xhr.responseText);
                            this.onUpload({
                                path: response.path,
                                filename: response.filename
                            });
                            resolve(response);
                        } catch (e) {
                            reject(e);
                        }
                    } else {
                        reject(new Error('Upload failed'));
                    }
                });

                xhr.addEventListener('error', () => reject(new Error('Network error')));
                xhr.addEventListener('abort', () => reject(new Error('Aborted')));

                xhr.open('POST', '/api/upload');
                xhr.send(formData);
            });
        },

        updateProgress(id, progress) {
            const item = this.queuePanel.querySelector(`[data-id="${id}"]`);
            if (item) {
                const fill = item.querySelector('.uq-fill');
                const text = item.querySelector('.uq-progress span');
                if (fill) fill.style.width = progress + '%';
                if (text) text.textContent = progress + '%';
            }
        },

        retryItem(id) {
            const item = this.uploadQueue.find(i => i.id === id);
            if (item) {
                item.status = 'pending';
                item.progress = 0;
                this.renderQueue();
                this.processQueue();
            }
        },

        removeItem(id) {
            const index = this.uploadQueue.findIndex(i => i.id === id);
            if (index > -1) {
                this.uploadQueue.splice(index, 1);
                this.renderQueue();
            }
        },

        getFileIcon(file) {
            if (file.type.startsWith('image/')) return '🖼️';
            if (file.type.startsWith('video/')) return '🎬';
            if (file.type.startsWith('audio/')) return '🎵';
            if (file.type.includes('pdf')) return '📕';
            if (file.type.includes('zip')) return '📦';
            return '📎';
        },

        formatSize(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
        }
    };

    window.DragDrop = DragDrop;
})();
