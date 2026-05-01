/**
 * 8Bit Theme — Canvas-based real pixelation
 *
 * Draws each target image onto a tiny canvas (e.g. 16×16),
 * then replaces the src with the low-res data-URL.
 * The browser upscales the tiny raster with image-rendering:pixelated
 * → nearest-neighbor = visible pixel blocks from actual source pixels.
 *
 * Also handles background-image elements (chat-ui-icons, robot sprite)
 * by replacing the background-image URL with a pixelated canvas version.
 */
(() => {
    'use strict';

    const PIXEL_SIZE = 8;   // target pixel resolution for small icons
    const PIXEL_SIZE_LG = 12; // target pixel resolution for larger images
    const PIXEL_SIZE_XL = 16; // target pixel resolution for preview images

    const _cache = new Map();

    function pixelateImage(img, px) {
        if (!img || !img.complete || !img.naturalWidth) return;
        const src = img.currentSrc || img.src;
        if (!src || src.startsWith('data:')) return;
        const key = src + '@' + px;
        if (_cache.has(key)) {
            if (img.src !== _cache.get(key)) img.src = _cache.get(key);
            return;
        }
        try {
            const c = document.createElement('canvas');
            c.width = px;
            c.height = px;
            const ctx = c.getContext('2d');
            ctx.imageSmoothingEnabled = false;
            ctx.drawImage(img, 0, 0, px, px);
            const dataURL = c.toDataURL('image/png');
            _cache.set(key, dataURL);
            img.src = dataURL;
        } catch (_) {
            // CORS or tainted canvas — skip
        }
    }

    function pixelateBackground(el, px) {
        if (!el) return;
        const style = getComputedStyle(el);
        const bg = style.backgroundImage;
        if (!bg || bg === 'none' || bg.includes('data:')) return;
        const urlMatch = bg.match(/url\(["']?([^"')]+)["']?\)/);
        if (!urlMatch) return;
        const src = urlMatch[1];
        const key = src + '@bg@' + px;
        if (_cache.has(key)) {
            el.style.backgroundImage = 'url(' + _cache.get(key) + ')';
            return;
        }
        const img = new Image();
        img.crossOrigin = 'anonymous';
        img.onload = () => {
            try {
                const c = document.createElement('canvas');
                c.width = px;
                c.height = px;
                const ctx = c.getContext('2d');
                ctx.imageSmoothingEnabled = false;
                ctx.drawImage(img, 0, 0, px, px);
                const dataURL = c.toDataURL('image/png');
                _cache.set(key, dataURL);
                el.style.backgroundImage = 'url(' + dataURL + ')';
            } catch (_) { }
        };
        img.src = src;
    }

    function runPixelation() {
        // Avatar <img> elements inside .avatar containers
        document.querySelectorAll('.avatar img, .avatar .persona-avatar-img').forEach(img => {
            pixelateImage(img, PIXEL_SIZE);
        });

        // Personality current icon
        const currentIcon = document.getElementById('personality-current-icon');
        if (currentIcon) pixelateImage(currentIcon, PIXEL_SIZE);

        // Personality preview image
        const previewImg = document.getElementById('personality-preview-image');
        if (previewImg) pixelateImage(previewImg, PIXEL_SIZE_XL);

        // Robot mascot sprite (background-image)
        const sprite = document.querySelector('.chat-robot-sprite');
        if (sprite) pixelateBackground(sprite, PIXEL_SIZE_LG);

        // Chat UI icons with background-image
        document.querySelectorAll('.chat-ui-icon').forEach(icon => {
            pixelateBackground(icon, PIXEL_SIZE);
        });
    }

    function init() {
        if (document.documentElement.getAttribute('data-theme') !== '8bit') return;

        // Run once after a short delay so images have started loading
        setTimeout(runPixelation, 300);

        // Re-run when new messages are added (MutationObserver)
        const chatBox = document.getElementById('chat-content') || document.getElementById('chat-box');
        if (chatBox && typeof MutationObserver !== 'undefined') {
            const observer = new MutationObserver(() => {
                setTimeout(runPixelation, 50);
            });
            observer.observe(chatBox, { childList: true, subtree: true });
        }

        // Re-run on theme change
        window.addEventListener('aurago:themechange', (e) => {
            if (e.detail && e.detail.theme === '8bit') {
                setTimeout(runPixelation, 300);
            }
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();