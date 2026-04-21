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
                script.src = '/js/vendor/mermaid.min.js';
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
                fontFamily: theme === 'cyberwar' || theme === 'dark-sun' || theme === 'black-matrix'
                    ? '"Oxanium", "Inter", system-ui, sans-serif'
                    : theme === 'papyrus'
                        ? '"Darker Grotesque", "Inter", system-ui, sans-serif'
                        : 'Inter, system-ui, sans-serif'
            };

            if (theme === 'light') {
                window.mermaid.initialize({
                    ...baseConfig,
                    theme: 'default'
                });
                return;
            }

            if (theme === 'papyrus') {
                window.mermaid.initialize({
                    ...baseConfig,
                    theme: 'base',
                    themeVariables: {
                        background: '#bca784',
                        primaryColor: '#efe1c4',
                        primaryTextColor: '#3c2c1d',
                        primaryBorderColor: '#9b7448',
                        lineColor: '#8a6138',
                        secondaryColor: '#dbc59f',
                        tertiaryColor: '#ccb086',
                        mainBkg: '#ead9bb',
                        secondBkg: '#dcc29a',
                        tertiaryBkg: '#c9ad82',
                        clusterBkg: '#e5d1af',
                        clusterBorder: '#a07649',
                        edgeLabelBackground: '#eddcc0',
                        nodeTextColor: '#352517',
                        textColor: '#573f2a',
                        actorBkg: '#e8d6b7',
                        actorBorder: '#a57b4c',
                        actorTextColor: '#362618',
                        signalColor: '#8a6138',
                        signalTextColor: '#362618',
                        labelBoxBkgColor: '#ebdcc1',
                        labelBoxBorderColor: '#9b7448',
                        noteBkgColor: '#f3e6cc',
                        noteBorderColor: '#b38a5a',
                        noteTextColor: '#4a3421',
                        activationBorderColor: '#9f7649',
                        activationBkgColor: '#dcc39b'
                    }
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

            if (theme === 'dark-sun') {
                window.mermaid.initialize({
                    ...baseConfig,
                    theme: 'base',
                    themeVariables: {
                        background: '#140b09',
                        primaryColor: '#2c1611',
                        primaryTextColor: '#f8e7d8',
                        primaryBorderColor: '#f56d32',
                        lineColor: '#ff8f3d',
                        secondaryColor: '#1d120f',
                        tertiaryColor: '#22120d',
                        mainBkg: '#170d0a',
                        secondBkg: '#21110d',
                        tertiaryBkg: '#120907',
                        clusterBkg: '#130908',
                        clusterBorder: '#ff9b4c',
                        edgeLabelBackground: '#1a0f0c',
                        nodeTextColor: '#f6e7db',
                        textColor: '#e6c8b4',
                        actorBkg: '#24120e',
                        actorBorder: '#ef6a2f',
                        actorTextColor: '#f8e7d8',
                        signalColor: '#ff9b4c',
                        signalTextColor: '#f8e7d8',
                        labelBoxBkgColor: '#24120e',
                        labelBoxBorderColor: '#ef6a2f',
                        noteBkgColor: '#2d1b12',
                        noteBorderColor: '#ffb15f',
                        noteTextColor: '#f7dfc8',
                        activationBorderColor: '#ff7b35',
                        activationBkgColor: '#2b120e'
                    }
                });
                return;
            }

            if (theme === 'ocean') {
                window.mermaid.initialize({
                    ...baseConfig,
                    theme: 'base',
                    themeVariables: {
                        background: '#091827',
                        primaryColor: '#10253c',
                        primaryTextColor: '#e8f4ff',
                        primaryBorderColor: '#4f8db8',
                        lineColor: '#70b8dc',
                        secondaryColor: '#0d2135',
                        tertiaryColor: '#122b43',
                        mainBkg: '#0e2136',
                        secondBkg: '#12304a',
                        tertiaryBkg: '#0a1a2b',
                        clusterBkg: '#0d2135',
                        clusterBorder: '#4f8db8',
                        edgeLabelBackground: '#0d2236',
                        nodeTextColor: '#e8f4ff',
                        textColor: '#c6deef',
                        actorBkg: '#122a43',
                        actorBorder: '#76b9d9',
                        actorTextColor: '#edf7ff',
                        signalColor: '#70b8dc',
                        signalTextColor: '#edf7ff',
                        labelBoxBkgColor: '#11273f',
                        labelBoxBorderColor: '#5b9ec6',
                        noteBkgColor: '#14314d',
                        noteBorderColor: '#7ec9e7',
                        noteTextColor: '#e8f5ff',
                        activationBorderColor: '#68a9cf',
                        activationBkgColor: '#10253d'
                    }
                });
                return;
            }

            if (theme === 'sandstorm') {
                window.mermaid.initialize({
                    ...baseConfig,
                    theme: 'base',
                    themeVariables: {
                        background: '#1d140d',
                        primaryColor: '#4a3421',
                        primaryTextColor: '#f0dfc4',
                        primaryBorderColor: '#d4aa72',
                        lineColor: '#ddb77f',
                        secondaryColor: '#372618',
                        tertiaryColor: '#251912',
                        mainBkg: '#342315',
                        secondBkg: '#513823',
                        tertiaryBkg: '#221710',
                        clusterBkg: '#2b1d13',
                        clusterBorder: '#d7af79',
                        edgeLabelBackground: '#2b1d13',
                        nodeTextColor: '#f0dfc4',
                        textColor: '#dcc6a5',
                        actorBkg: '#4a3421',
                        actorBorder: '#d8b27c',
                        actorTextColor: '#fff1dc',
                        signalColor: '#ddb77f',
                        signalTextColor: '#fff1dc',
                        labelBoxBkgColor: '#46301f',
                        labelBoxBorderColor: '#c79358',
                        noteBkgColor: '#5c4028',
                        noteBorderColor: '#e0bf8f',
                        noteTextColor: '#fff0d8',
                        activationBorderColor: '#d5ac74',
                        activationBkgColor: '#422d1c'
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

            if (theme === 'black-matrix') {
                window.mermaid.initialize({
                    ...baseConfig,
                    theme: 'base',
                    themeVariables: {
                        background: '#030404',
                        primaryColor: '#101512',
                        primaryTextColor: '#edf5ee',
                        primaryBorderColor: '#8ee65e',
                        lineColor: '#8ee65e',
                        secondaryColor: '#0a0e0b',
                        tertiaryColor: '#121816',
                        mainBkg: '#0b100d',
                        secondBkg: '#131a16',
                        tertiaryBkg: '#060807',
                        clusterBkg: '#090c0a',
                        clusterBorder: '#95f46c',
                        edgeLabelBackground: '#080a08',
                        nodeTextColor: '#edf5ee',
                        textColor: '#cfddd0',
                        actorBkg: '#111713',
                        actorBorder: '#b7c6bb',
                        actorTextColor: '#f4faf4',
                        signalColor: '#8ee65e',
                        signalTextColor: '#f4faf4',
                        labelBoxBkgColor: '#121713',
                        labelBoxBorderColor: '#95f46c',
                        noteBkgColor: '#171d19',
                        noteBorderColor: '#b7c6bb',
                        noteTextColor: '#eff6ef',
                        activationBorderColor: '#8ee65e',
                        activationBkgColor: '#111612'
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
