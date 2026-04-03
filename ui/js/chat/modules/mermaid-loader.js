/**
 * Mermaid Loader Module
 * Lazy-loads Mermaid and renders diagrams
 */

(function() {
    'use strict';

    function escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    const MermaidLoader = {
        loaded: false,
        loading: false,
        queue: [],

        async init() {
            // Process any existing mermaid blocks
            this.processBlocks();
            
            // Set up observer for new content
            this.setupObserver();
        },

        async load() {
            if (this.loaded) return Promise.resolve();
            if (this.loading) return this.loadingPromise;

            this.loading = true;
            this.loadingPromise = new Promise((resolve, reject) => {
                if (window.mermaid) {
                    this.loaded = true;
                    this.loading = false;
                    resolve();
                    return;
                }

                const script = document.createElement('script');
                script.src = 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.min.js';
                script.async = true;
                script.onload = () => {
                    this.initMermaid();
                    this.loaded = true;
                    this.loading = false;
                    resolve();
                };
                script.onerror = () => {
                    this.loading = false;
                    reject(new Error('Failed to load Mermaid'));
                };
                document.head.appendChild(script);
            });

            return this.loadingPromise;
        },

        initMermaid() {
            if (!window.mermaid) return;
            
            const isDark = document.documentElement.getAttribute('data-theme') !== 'light';
            
            window.mermaid.initialize({
                startOnLoad: false,
                theme: isDark ? 'dark' : 'default',
                securityLevel: 'strict',
                fontFamily: 'Inter, system-ui, sans-serif'
            });
        },

        async processBlocks(container = document) {
            const blocks = container.querySelectorAll('.mermaid-raw');
            if (blocks.length === 0) return;

            try {
                await this.load();
                
                for (let i = 0; i < blocks.length; i++) {
                    const block = blocks[i];
                    const code = block.textContent.trim();
                    const id = 'mermaid-' + Date.now() + '-' + i;
                    
                    try {
                        const { svg } = await window.mermaid.render(id, code);
                        const wrapper = this.createWrapper(svg, code);
                        block.replaceWith(wrapper);
                    } catch (err) {
                        console.error('[MermaidLoader] Render error:', err);
                        block.innerHTML = `
                            <div class="mermaid-error">
                                <div class="me-title">⚠️ Diagram Error</div>
                                <div class="me-msg">${escapeHtml(err.message || 'Failed to render')}</div>
                            </div>
                        `;
                    }
                }
            } catch (err) {
                console.error('[MermaidLoader] Failed to load:', err);
            }
        },

        createWrapper(svg, code) {
            const wrapper = document.createElement('div');
            wrapper.className = 'mermaid-container';
            
            wrapper.innerHTML = `
                <div class="mermaid-diagram">${svg}</div>
                <div class="mermaid-controls">
                    <button class="mc-btn mc-copy" title="Copy source">📋</button>
                    <button class="mc-btn mc-expand" title="Expand">⛶</button>
                    <button class="mc-btn mc-download" title="Download SVG">⬇</button>
                </div>
            `;

            // Bind events
            wrapper.querySelector('.mc-copy').addEventListener('click', () => {
                navigator.clipboard.writeText(code).then(() => {
                    this.showToast('Diagram source copied');
                });
            });

            wrapper.querySelector('.mc-expand').addEventListener('click', () => {
                this.expandDiagram(svg);
            });

            wrapper.querySelector('.mc-download').addEventListener('click', () => {
                this.downloadSVG(svg);
            });

            return wrapper;
        },

        expandDiagram(svg) {
            const modal = document.createElement('div');
            modal.className = 'mermaid-modal';
            modal.innerHTML = `
                <div class="mm-overlay"></div>
                <div class="mm-content">
                    <button class="mm-close">✕</button>
                    <div class="mm-diagram">${svg}</div>
                </div>
            `;
            
            modal.querySelector('.mm-close').addEventListener('click', () => modal.remove());
            modal.querySelector('.mm-overlay').addEventListener('click', () => modal.remove());
            
            document.body.appendChild(modal);
        },

        downloadSVG(svg) {
            const blob = new Blob([svg], { type: 'image/svg+xml' });
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = `diagram-${Date.now()}.svg`;
            a.click();
            URL.revokeObjectURL(url);
        },

        showToast(message) {
            if (window.showToast) {
                window.showToast(message, 'success');
            }
        },

        setupObserver() {
            // Watch for new content
            const observer = new MutationObserver((mutations) => {
                mutations.forEach((mutation) => {
                    mutation.addedNodes.forEach((node) => {
                        if (node.nodeType === 1) { // Element
                            this.processBlocks(node);
                        }
                    });
                });
            });

            observer.observe(document.getElementById('chat-content'), {
                childList: true,
                subtree: true
            });
        }
    };

    window.MermaidLoader = MermaidLoader;
})();
