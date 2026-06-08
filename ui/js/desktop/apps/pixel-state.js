(function () {
    'use strict';

    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const MAX_HISTORY = 30;
    const canvasPool = [];
    const IMAGE_EXTS = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'bmp', 'svg', 'ico', 'tiff', 'tif', 'avif'];
    const PRESET_COLORS = [
        '#000000', '#404040', '#808080', '#c0c0c0', '#ffffff',
        '#ff0000', '#ff6600', '#ffcc00', '#33cc33', '#0099ff',
        '#6633cc', '#ff3399', '#993300', '#006633', '#003366',
        '#660066', '#ffcccc', '#ffffcc', '#ccffcc', '#ccffff',
        '#ccccff', '#ffccff', '#ffe0cc', '#cce0ff'
    ];
    const FONT_FAMILIES = ['Arial', 'Helvetica', 'Times New Roman', 'Georgia', 'Courier New', 'Verdana', 'Impact', 'Comic Sans MS'];

    function acquireTempCanvas(width, height) {
        const c = canvasPool.pop() || document.createElement('canvas');
        c.width = width;
        c.height = height;
        return c;
    }

    function releaseTempCanvas(canvas) {
        if (!canvas) return;
        const ctx = canvas.getContext('2d');
        if (ctx) ctx.clearRect(0, 0, canvas.width, canvas.height);
        if (canvasPool.length < 4) canvasPool.push(canvas);
    }

    function clamp255(v) { return Math.max(0, Math.min(255, Math.round(v))); }

    function hexToRgb(hex) {
        const h = hex.replace('#', '');
        return { r: parseInt(h.substring(0, 2), 16), g: parseInt(h.substring(2, 4), 16), b: parseInt(h.substring(4, 6), 16) };
    }

    function rgbToHex(r, g, b) {
        return '#' + [r, g, b].map(v => clamp255(v).toString(16).padStart(2, '0')).join('');
    }

    function colorDist(r1, g1, b1, r2, g2, b2) {
        return Math.abs(r1 - r2) + Math.abs(g1 - g2) + Math.abs(b1 - b2);
    }

    const TOOL_SVGS = {
        'select-rect': '<svg width="16" height="16" viewBox="0 0 16 16"><rect x="2" y="2" width="12" height="12" fill="none" stroke="currentColor" stroke-width="1.5" stroke-dasharray="2 2"/></svg>',
        'select-ellipse': '<svg width="16" height="16" viewBox="0 0 16 16"><ellipse cx="8" cy="8" rx="6" ry="6" fill="none" stroke="currentColor" stroke-width="1.5" stroke-dasharray="2 2"/></svg>',
        'brush': '<svg width="16" height="16" viewBox="0 0 16 16"><path d="M13 1.5a1.5 1.5 0 010 2.1L6.3 10.3 4 12l1.7-2.3L12.4 3a1.5 1.5 0 00-2.1-2.1L3.6 7.6 2 14l6-2 6.7-6.7a1.5 1.5 0 000-2.1L13 1.5z" fill="currentColor" opacity="0.85"/></svg>',
        'eraser': '<svg width="16" height="16" viewBox="0 0 16 16"><path d="M5.5 14h8M2.5 10l4.2-5.5a1 1 0 011.4 0l4.2 4.2a1 1 0 010 1.4L8 14.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/><line x1="2.5" y1="14" x2="13.5" y2="14" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/></svg>',
        'pencil': '<svg width="16" height="16" viewBox="0 0 16 16"><path d="M11.5 2.5l2 2-8 8H3.5v-2l8-8z" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linejoin="round"/><line x1="3.5" y1="14" x2="13" y2="14" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" opacity="0.5"/></svg>',
        'line': '<svg width="16" height="16" viewBox="0 0 16 16"><line x1="2" y1="14" x2="14" y2="2" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
        'rectangle': '<svg width="16" height="16" viewBox="0 0 16 16"><rect x="2" y="3" width="12" height="10" fill="none" stroke="currentColor" stroke-width="1.5" rx="1"/></svg>',
        'ellipse': '<svg width="16" height="16" viewBox="0 0 16 16"><ellipse cx="8" cy="8" rx="6" ry="5" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>',
        'arrow': '<svg width="16" height="16" viewBox="0 0 16 16"><line x1="2" y1="14" x2="12" y2="4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/><polyline points="7,3 14,3 14,10" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
        'text': '<svg width="16" height="16" viewBox="0 0 16 16"><text x="3.5" y="13" font-size="13" font-weight="bold" fill="currentColor" font-family="sans-serif">T</text></svg>',
        'fill': '<svg width="16" height="16" viewBox="0 0 16 16"><path d="M3 13L8 3l5 10" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/><path d="M14 11c0 1.5-1.5 3-3 3s-3-1.5-3-3 3-5 3-5 3 3.5 3 5z" fill="currentColor" opacity="0.6"/></svg>',
        'eyedropper': '<svg width="16" height="16" viewBox="0 0 16 16"><path d="M13.4 2.6a1.8 1.8 0 00-2.5 0L9.2 4.3 7 2.1l-1 1 2.2 2.2-5.4 5.4V14h3.3l5.4-5.4L13.7 11l1-1-2.2-2.2 1.7-1.7a1.8 1.8 0 000-2.5z" fill="none" stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/></svg>'
    };

    function bindRuntime(runtime, fn) {
        return function boundRuntimeFunction(...args) {
            return fn.apply(runtime, args);
        };
    }

    Object.assign(Pixel, {
        MAX_HISTORY,
        IMAGE_EXTS,
        PRESET_COLORS,
        FONT_FAMILIES,
        TOOL_SVGS,
        acquireTempCanvas,
        releaseTempCanvas,
        clamp255,
        hexToRgb,
        rgbToHex,
        colorDist,
        bindRuntime
    });
})();
