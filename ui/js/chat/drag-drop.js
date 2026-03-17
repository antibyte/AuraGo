// AuraGo Drag & Drop File Upload
// Handles drag & drop file uploads with queue visualization

class DragDropManager {
    constructor(options = {}) {
        this.container = options.container || document.getElementById('chat-box');
        this.onUpload = options.onUpload || (() => {});
        this.uploadQueue = [];
        this.isProcessing = false;
        this.maxFileSize = options.maxFileSize || 32 * 1024 * 1024; // 32MB
        this.allowedTypes = options.allowedTypes || null; // null = all
        
        this.overlay = null;
        this.queuePanel = null;
        
        this.init();
    }

    init() {
        if (!this.container) return;

        // Create overlay
        this.createOverlay();
        
        // Create queue panel
        this.createQueuePanel();

        // Event listeners
        ['dragenter', 'dragover', 'dragleave', 'drop'].forEach(eventName => {
            this.container.addEventListener(eventName, (e) => this.preventDefaults(e), false);
            document.body.addEventListener(eventName, (e) => this.preventDefaults(e), false);
        });

        ['dragenter', 'dragover'].forEach(eventName => {
            this.container.addEventListener(eventName, (e) => this.highlight(e), false);
        });

        ['dragleave', 'drop'].forEach(eventName => {
            this.container.addEventListener(eventName, (e) => this.unhighlight(e), false);
        });

        this.container.addEventListener('drop', (e) => this.handleDrop(e), false);
    }

    preventDefaults(e) {
        e.preventDefault();
        e.stopPropagation();
    }

    createOverlay() {
        this.overlay = document.createElement('div');
        this.overlay.className = 'drag-drop-overlay';
        this.overlay.innerHTML = `
            <div class="drag-drop-message">
                <div class="drag-drop-icon">📁</div>
                <div class="drag-drop-text">Drop files here to upload</div>
            </div>
        `;
        document.body.appendChild(this.overlay);
    }

    createQueuePanel() {
        this.queuePanel = document.createElement('div');
        this.queuePanel.className = 'upload-queue-panel';
        this.queuePanel.style.display = 'none';
        document.body.appendChild(this.queuePanel);
    }

    highlight(e) {
        this.overlay.classList.add('active');
    }

    unhighlight(e) {
        // Only unhighlight if leaving the container, not entering a child
        if (e.type === 'dragleave' && this.container.contains(e.relatedTarget)) {
            return;
        }
        this.overlay.classList.remove('active');
    }

    handleDrop(e) {
        this.overlay.classList.remove('active');
        
        const dt = e.dataTransfer;
        const files = dt.files;
        
        if (files.length > 0) {
            this.addFilesToQueue(Array.from(files));
        }
    }

    addFilesToQueue(files) {
        const validFiles = files.filter(file => this.validateFile(file));
        
        if (validFiles.length === 0) return;

        validFiles.forEach(file => {
            this.uploadQueue.push({
                file,
                id: 'upload-' + Math.random().toString(36).substr(2, 9),
                status: 'pending',
                progress: 0,
                error: null
            });
        });

        this.renderQueue();
        this.processQueue();
    }

    validateFile(file) {
        // Check file size
        if (file.size > this.maxFileSize) {
            showToast(`File "${file.name}" is too large (max ${this.formatSize(this.maxFileSize)})`, 'error');
            return false;
        }

        // Check file type if restricted
        if (this.allowedTypes && !this.allowedTypes.includes(file.type)) {
            showToast(`File type "${file.type}" not allowed`, 'error');
            return false;
        }

        return true;
    }

    renderQueue() {
        if (this.uploadQueue.length === 0) {
            this.queuePanel.style.display = 'none';
            return;
        }

        this.queuePanel.style.display = 'block';
        
        const pending = this.uploadQueue.filter(f => f.status === 'pending').length;
        const uploading = this.uploadQueue.filter(f => f.status === 'uploading').length;
        const completed = this.uploadQueue.filter(f => f.status === 'complete').length;
        const errors = this.uploadQueue.filter(f => f.status === 'error').length;

        let html = `
            <div class="queue-header">
                <span class="queue-title">
                    Uploading ${this.uploadQueue.length} file${this.uploadQueue.length !== 1 ? 's' : ''}
                </span>
                <button class="queue-close" onclick="dragDropManager.closeQueue()">✕</button>
            </div>
            <div class="queue-items">
        `;

        this.uploadQueue.forEach(item => {
            const icon = this.getFileIcon(item.file);
            const statusIcon = this.getStatusIcon(item.status);
            
            html += `
                <div class="queue-item ${item.status}" data-id="${item.id}">
                    <div class="queue-item-icon">${icon}</div>
                    <div class="queue-item-info">
                        <div class="queue-item-name" title="${escapeHtml(item.file.name)}">
                            ${escapeHtml(item.file.name)}
                        </div>
                        <div class="queue-item-meta">
                            ${this.formatSize(item.file.size)} • ${statusIcon} ${item.status}
                        </div>
                        ${item.status === 'uploading' ? `
                            <div class="queue-item-progress">
                                <div class="progress-bar">
                                    <div class="progress-fill" style="width: ${item.progress}%"></div>
                                </div>
                                <span class="progress-text">${item.progress}%</span>
                            </div>
                        ` : ''}
                    </div>
                    ${item.status === 'error' ? `
                        <button class="queue-item-retry" onclick="dragDropManager.retryItem('${item.id}')" title="Retry">↻</button>
                    ` : ''}
                    <button class="queue-item-remove" onclick="dragDropManager.removeItem('${item.id}')" title="Remove">✕</button>
                </div>
            `;
        });

        html += '</div>';
        
        if (completed === this.uploadQueue.length) {
            html += `
                <div class="queue-footer">
                    <span class="queue-complete">✓ All uploads complete</span>
                    <button class="queue-clear-btn" onclick="dragDropManager.clearCompleted()">Clear</button>
                </div>
            `;
        }

        this.queuePanel.innerHTML = html;
    }

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
                item.error = err.message;
                console.error('Upload error:', err);
            }

            this.renderQueue();
        }

        this.isProcessing = false;
        
        // Auto-clear if all completed
        const allComplete = this.uploadQueue.every(i => i.status === 'complete');
        if (allComplete) {
            setTimeout(() => this.clearCompleted(), 3000);
        }
    }

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
                        // Call the upload callback
                        this.onUpload({
                            path: response.path,
                            filename: response.filename,
                            file: item.file
                        });
                        resolve(response);
                    } catch (e) {
                        reject(new Error('Invalid response'));
                    }
                } else {
                    reject(new Error(xhr.statusText || 'Upload failed'));
                }
            });

            xhr.addEventListener('error', () => reject(new Error('Network error')));
            xhr.addEventListener('abort', () => reject(new Error('Upload aborted')));

            xhr.open('POST', '/api/upload');
            xhr.send(formData);
        });
    }

    updateProgress(id, progress) {
        const itemEl = this.queuePanel.querySelector(`[data-id="${id}"]`);
        if (itemEl) {
            const fill = itemEl.querySelector('.progress-fill');
            const text = itemEl.querySelector('.progress-text');
            if (fill) fill.style.width = progress + '%';
            if (text) text.textContent = progress + '%';
        }
    }

    getFileIcon(file) {
        const typeIcons = {
            'image/': '🖼️',
            'video/': '🎬',
            'audio/': '🎵',
            'text/': '📄',
            'application/pdf': '📕',
            'application/zip': '📦',
            'application/json': '📋'
        };

        for (const [type, icon] of Object.entries(typeIcons)) {
            if (file.type.startsWith(type)) return icon;
        }
        return '📎';
    }

    getStatusIcon(status) {
        const icons = {
            'pending': '⏳',
            'uploading': '📤',
            'complete': '✅',
            'error': '❌'
        };
        return icons[status] || '⏳';
    }

    formatSize(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }

    retryItem(id) {
        const item = this.uploadQueue.find(i => i.id === id);
        if (item) {
            item.status = 'pending';
            item.error = null;
            item.progress = 0;
            this.renderQueue();
            this.processQueue();
        }
    }

    removeItem(id) {
        const index = this.uploadQueue.findIndex(i => i.id === id);
        if (index > -1) {
            this.uploadQueue.splice(index, 1);
            this.renderQueue();
        }
    }

    clearCompleted() {
        this.uploadQueue = this.uploadQueue.filter(i => i.status !== 'complete');
        this.renderQueue();
    }

    closeQueue() {
        // Only close if all done, or confirm
        const hasActive = this.uploadQueue.some(i => i.status === 'uploading');
        if (hasActive) {
            if (!confirm('Uploads in progress. Close anyway?')) return;
        }
        this.queuePanel.style.display = 'none';
    }

    destroy() {
        if (this.overlay) this.overlay.remove();
        if (this.queuePanel) this.queuePanel.remove();
    }
}

// Helper function
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Export
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { DragDropManager };
}
