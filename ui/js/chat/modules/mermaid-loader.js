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

            window.addEventListener('aurago:themechange', () => {
                if (window.mermaid) {
                    this.initMermaid();
                }
            });
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

            const theme = document.documentElement.getAttribute('data-theme');
            const baseConfig = {
                startOnLoad: false,
                securityLevel: 'strict',
                fontFamily: theme === 'cyberwar'
                    ? '"Oxanium", "Inter", system-ui, sans-serif'
                    : 'Inter, system-ui, sans-serif'
            };

            if (theme === 'light') {
                window.mermaid.initialize({
                    ...baseConfig,
                    theme: 'default'
                });
                return;
            }

            if (theme === 'lollipop') {
                window.mermaid.initialize({
                    ...baseConfig,
                    theme: 'base',
                    themeVariables: {
                        background: '#fff8fd',
                        primaryColor: '#ffe0f2',
                        primaryTextColor: '#56384d',
                        primaryBorderColor: '#f39cca',
                        lineColor: '#ba81b2',
                        secondaryColor: '#efe7ff',
                        tertiaryColor: '#e3fff2',
                        mainBkg: '#fff3fb',
                        secondBkg: '#f5eeff',
                        tertiaryBkg: '#effff6',
                        clusterBkg: '#fff7dc',
                        clusterBorder: '#f0b56c',
                        edgeLabelBackground: '#fff9fd',
                        nodeTextColor: '#56384d',
                        textColor: '#5b4860',
                        actorBkg: '#ffe6f4',
                        actorBorder: '#e39bd2',
                        actorTextColor: '#56384d',
                        signalColor: '#ba81b2',
                        signalTextColor: '#56384d',
                        labelBoxBkgColor: '#fff0f9',
                        labelBoxBorderColor: '#f39cca',
                        noteBkgColor: '#fff8cf',
                        noteBorderColor: '#ebb46b',
                        noteTextColor: '#6a4f35',
                        activationBorderColor: '#d392c8',
                        activationBkgColor: '#fff0fb'
                    }
                });
                return;
            }

            if (theme === 'cyberwar') {
                window.mermaid.initialize({
                    ...baseConfig,
                    theme: 'base',
                    themeVariables: {
                        background: '#071128',
                        primaryColor: '#122955',
                        primaryTextColor: '#eef7ff',
                        primaryBorderColor: '#54f7ff',
                        lineColor: '#54f7ff',
                        secondaryColor: '#1d1443',
                        tertiaryColor: '#160f34',
                        mainBkg: '#0a1535',
                        secondBkg: '#121f47',
                        tertiaryBkg: '#091126',
                        clusterBkg: '#091126',
                        clusterBorder: '#68f1ff',
                        edgeLabelBackground: '#091126',
                        nodeTextColor: '#eef7ff',
                        textColor: '#d7e6ff',
                        actorBkg: '#101d42',
                        actorBorder: '#8b7dff',
                        actorTextColor: '#eef7ff',
                        signalColor: '#54f7ff',
                        signalTextColor: '#eef7ff',
                        labelBoxBkgColor: '#111d44',
                        labelBoxBorderColor: '#68f1ff',
                        noteBkgColor: '#261348',
                        noteBorderColor: '#ff47c8',
                        noteTextColor: '#f8dcff',
                        activationBorderColor: '#54f7ff',
                        activationBkgColor: '#0f2550'
                    }
                });
                return;
            }

            window.mermaid.initialize({
                ...baseConfig,
                theme: 'dark'
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
