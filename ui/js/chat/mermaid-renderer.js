// AuraGo Mermaid Diagram Renderer
// Handles rendering and interaction with Mermaid diagrams

class MermaidRenderer {
    constructor() {
        this.loaded = false;
        this.queue = [];
        this.renderedDiagrams = new Map();
        this.zoomLevels = new Map();

        window.addEventListener('aurago:themechange', () => {
            if (window.mermaid) {
                this.initMermaid();
            }
        });
    }

    // Lazy load Mermaid library
    async load() {
        if (this.loaded) return Promise.resolve();

        return new Promise((resolve, reject) => {
            // Check if already loaded
            if (window.mermaid) {
                this.loaded = true;
                this.initMermaid();
                resolve();
                return;
            }

            // Load Mermaid from bundled vendor assets
            const script = document.createElement('script');
            script.src = '/js/vendor/mermaid.min.js';
            script.async = true;
            script.onload = () => {
                this.loaded = true;
                this.initMermaid();
                resolve();
            };
            script.onerror = () => reject(new Error('Failed to load Mermaid'));
            document.head.appendChild(script);
        });
    }

    initMermaid() {
        if (window.mermaid) {
            window.mermaid.initialize(this.getThemeConfig());
        }
    }

    getThemeConfig() {
        const theme = document.documentElement.getAttribute('data-theme');
        const baseConfig = {
            startOnLoad: false,
            securityLevel: 'strict',
            fontFamily: theme === '8bit'
                ? '"Press Start 2P", ui-monospace, SFMono-Regular, Consolas, monospace'
                : theme === 'cyberwar' || theme === 'dark-sun' || theme === 'black-matrix'
                ? '"Oxanium", "Inter", system-ui, sans-serif'
                : theme === 'papyrus'
                    ? '"Darker Grotesque", "Inter", system-ui, sans-serif'
                    : 'Inter, system-ui, sans-serif',
            flowchart: {
                curve: 'basis',
                padding: 15
            },
            sequence: {
                diagramMarginX: 50,
                diagramMarginY: 10
            }
        };

        if (theme === 'light') {
            return {
                ...baseConfig,
                startOnLoad: false,
                theme: 'default'
            };
        }

        if (theme === 'papyrus') {
            return {
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
            };
        }

        if (theme === 'lollipop') {
            return {
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
            };
        }

        if (theme === 'dark-sun') {
            return {
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
            };
        }

        if (theme === 'ocean') {
            return {
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
            };
        }

        if (theme === 'sandstorm') {
            return {
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
            };
        }

        if (theme === 'cyberwar') {
            return {
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
            };
        }

        if (theme === 'black-matrix') {
            return {
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
            };
        }

        return {
            ...baseConfig,
            theme: 'dark'
        };
    }

    // Process all mermaid code blocks in a container
    async render(container) {
        await this.load();

        const blocks = container.querySelectorAll('.mermaid-raw');
        
        for (let i = 0; i < blocks.length; i++) {
            const block = blocks[i];
            const code = block.textContent.trim();
            const id = `mermaid-${Date.now()}-${i}`;
            
            try {
                // Use Mermaid's render API
                const { svg } = await window.mermaid.render(id, code);
                
                // Create container with controls
                const wrapper = this.createDiagramContainer(svg, code, id);
                block.replaceWith(wrapper);
                
                // Store reference
                this.renderedDiagrams.set(id, {
                    element: wrapper,
                    code: code,
                    zoom: 1
                });
                
            } catch (err) {
                console.error('Mermaid render error:', err);
                block.innerHTML = `
                    <div class="mermaid-error">
                        <div class="error-title">${window.chatUiIconMarkup ? window.chatUiIconMarkup('warning') : ''} ${t('chat.mermaid_error_title')}</div>
                        <div class="error-message">${mermaidEscapeHtml(err.message || t('chat.mermaid_error_failed'))}</div>
                        <details>
                            <summary>${t('chat.mermaid_view_source')}</summary>
                            <pre>${mermaidEscapeHtml(code)}</pre>
                        </details>
                    </div>
                `;
            }
        }
    }

    createDiagramContainer(svg, code, id) {
        const container = document.createElement('div');
        container.className = 'mermaid-container';
        container.dataset.diagramId = id;
        
        container.innerHTML = `
            <div class="mermaid-diagram" id="${id}">
                ${svg}
            </div>
            <div class="mermaid-controls">
                <div class="mermaid-zoom-controls">
                    <button class="mermaid-btn zoom-out" title="${t('chat.mermaid_zoom_out')}">−</button>
                    <span class="zoom-level">100%</span>
                    <button class="mermaid-btn zoom-in" title="${t('chat.mermaid_zoom_in')}">+</button>
                    <button class="mermaid-btn zoom-reset" title="${t('chat.mermaid_zoom_reset')}">⟲</button>
                </div>
                <div class="mermaid-actions">
                    <button class="mermaid-btn copy-source" title="${t('chat.mermaid_copy_source')}">
                        ${window.chatUiIconMarkup ? window.chatUiIconMarkup('clipboard') : ''} ${t('chat.mermaid_view_source')}
                    </button>
                    <button class="mermaid-btn expand" title="${t('chat.mermaid_expand')}">
                        ${window.chatUiIconMarkup ? window.chatUiIconMarkup('expand') : ''} ${t('chat.mermaid_expand')}
                    </button>
                    <button class="mermaid-btn download-svg" title="${t('chat.mermaid_download_svg')}">
                        ${window.chatUiIconMarkup ? window.chatUiIconMarkup('download') : ''} SVG
                    </button>
                    <button class="mermaid-btn download-png" title="${t('chat.mermaid_download_png')}">
                        ${window.chatUiIconMarkup ? window.chatUiIconMarkup('download') : ''} PNG
                    </button>
                </div>
            </div>
        `;

        // Attach event listeners
        this.attachControls(container, code, id);
        
        // Initialize zoom/pan
        this.initZoomPan(container, id);
        
        return container;
    }

    attachControls(container, code, id) {
        // Copy source
        container.querySelector('.copy-source').addEventListener('click', () => {
            navigator.clipboard.writeText(code).then(() => {
                showToast(t('chat.mermaid_source_copied'), 'success');
            });
        });

        // Expand
        container.querySelector('.expand').addEventListener('click', () => {
            this.openFullscreen(container, code, id);
        });

        // Download SVG
        container.querySelector('.download-svg').addEventListener('click', () => {
            this.downloadSVG(container, id);
        });

        // Download PNG
        container.querySelector('.download-png').addEventListener('click', () => {
            this.downloadPNG(container, id);
        });

        // Zoom controls
        container.querySelector('.zoom-in').addEventListener('click', () => {
            this.zoom(id, 1.2);
        });

        container.querySelector('.zoom-out').addEventListener('click', () => {
            this.zoom(id, 0.8);
        });

        container.querySelector('.zoom-reset').addEventListener('click', () => {
            this.resetZoom(id);
        });
    }

    initZoomPan(container, id) {
        const diagram = container.querySelector('.mermaid-diagram');
        let scale = 1;
        let isDragging = false;
        let startX, startY, translateX = 0, translateY = 0;

        // Mouse wheel zoom
        diagram.addEventListener('wheel', (e) => {
            if (e.ctrlKey || e.metaKey) {
                e.preventDefault();
                const delta = e.deltaY > 0 ? 0.9 : 1.1;
                this.zoom(id, delta);
            }
        });

        // Pan functionality
        diagram.addEventListener('mousedown', (e) => {
            if (scale > 1) {
                isDragging = true;
                startX = e.clientX - translateX;
                startY = e.clientY - translateY;
                diagram.style.cursor = 'grabbing';
            }
        });

        window.addEventListener('mousemove', (e) => {
            if (isDragging) {
                translateX = e.clientX - startX;
                translateY = e.clientY - startY;
                this.applyTransform(diagram, scale, translateX, translateY);
            }
        });

        window.addEventListener('mouseup', () => {
            isDragging = false;
            diagram.style.cursor = 'grab';
        });

        // Store state
        this.zoomLevels.set(id, { scale, translateX, translateY });
    }

    zoom(id, factor) {
        const container = document.querySelector(`[data-diagram-id="${id}"]`);
        if (!container) return;

        const diagram = container.querySelector('.mermaid-diagram');
        let { scale, translateX, translateY } = this.zoomLevels.get(id) || { scale: 1, translateX: 0, translateY: 0 };
        
        scale = Math.max(0.2, Math.min(5, scale * factor));
        
        this.zoomLevels.set(id, { scale, translateX, translateY });
        this.applyTransform(diagram, scale, translateX, translateY);
        
        // Update zoom level display
        const zoomDisplay = container.querySelector('.zoom-level');
        if (zoomDisplay) {
            zoomDisplay.textContent = Math.round(scale * 100) + '%';
        }
    }

    resetZoom(id) {
        const container = document.querySelector(`[data-diagram-id="${id}"]`);
        if (!container) return;

        const diagram = container.querySelector('.mermaid-diagram');
        this.zoomLevels.set(id, { scale: 1, translateX: 0, translateY: 0 });
        this.applyTransform(diagram, 1, 0, 0);
        
        const zoomDisplay = container.querySelector('.zoom-level');
        if (zoomDisplay) {
            zoomDisplay.textContent = '100%';
        }
    }

    applyTransform(element, scale, x, y) {
        element.style.transform = `translate(${x}px, ${y}px) scale(${scale})`;
        element.style.transformOrigin = 'center center';
    }

    openFullscreen(container, code, id) {
        const modal = document.createElement('div');
        modal.className = 'mermaid-modal';
        
        const diagram = container.querySelector('.mermaid-diagram').cloneNode(true);
        
        modal.innerHTML = `
            <div class="mermaid-modal-overlay"></div>
            <div class="mermaid-modal-content">
                <button class="mermaid-modal-close">${window.chatUiIconMarkup ? window.chatUiIconMarkup('close') : ''}</button>
                <div class="mermaid-modal-diagram"></div>
            </div>
        `;
        
        modal.querySelector('.mermaid-modal-diagram').appendChild(diagram);
        
        modal.querySelector('.mermaid-modal-close').addEventListener('click', () => {
            modal.remove();
        });
        
        modal.querySelector('.mermaid-modal-overlay').addEventListener('click', () => {
            modal.remove();
        });
        
        document.body.appendChild(modal);
        
        // Apply current zoom
        const { scale } = this.zoomLevels.get(id) || { scale: 1 };
        if (scale > 1) {
            diagram.style.transform = `scale(${scale})`;
        }
    }

    downloadSVG(container, id) {
        const svg = container.querySelector('svg');
        if (!svg) return;

        const svgData = new XMLSerializer().serializeToString(svg);
        const blob = new Blob([svgData], { type: 'image/svg+xml' });
        const url = URL.createObjectURL(blob);
        
        const a = document.createElement('a');
        a.href = url;
        a.download = `diagram-${id}.svg`;
        a.click();
        
        URL.revokeObjectURL(url);
    }

    downloadPNG(container, id) {
        const svg = container.querySelector('svg');
        if (!svg) return;

        const svgData = new XMLSerializer().serializeToString(svg);
        const canvas = document.createElement('canvas');
        const ctx = canvas.getContext('2d');
        const img = new Image();
        
        // Get SVG dimensions
        const svgRect = svg.getBoundingClientRect();
        canvas.width = svgRect.width * 2; // 2x for retina
        canvas.height = svgRect.height * 2;
        
        img.onload = () => {
            ctx.fillStyle = document.documentElement.getAttribute('data-theme') === 'light' ? '#ffffff' : '#1a1a1a';
            ctx.fillRect(0, 0, canvas.width, canvas.height);
            ctx.drawImage(img, 0, 0, canvas.width, canvas.height);
            
            const a = document.createElement('a');
            a.href = canvas.toDataURL('image/png');
            a.download = `diagram-${id}.png`;
            a.click();
        };
        
        img.src = 'data:image/svg+xml;base64,' + btoa(unescape(encodeURIComponent(svgData)));
    }

    // Update theme when switched
    updateTheme(theme) {
        if (window.mermaid) {
            window.mermaid.initialize({
                theme: theme === 'light' ? 'default' : 'dark'
            });
        }
    }
}

// Global instance
const mermaidRenderer = new MermaidRenderer();

// Helper to escape HTML
function mermaidEscapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Export
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { MermaidRenderer, mermaidRenderer };
}
