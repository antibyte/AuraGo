/**
 * 8Bit Theme — Canvas-based real pixelation for <img> elements.
 *
 * Draws each target image onto a tiny canvas (e.g. 8×8),
 * then replaces the src with the low-res data-URL.
 * image-rendering:pixelated on the element does nearest-neighbor
 * upscaling = visible pixel blocks from actual source pixels.
 *
 * Monitors img.src changes via load events and MutationObserver
 * so dynamically loaded persona images get pixelated too.
 */
(() => {
    'use strict';

    const PIXEL_SMALL = 28;
    const PIXEL_LARGE = 48;

    const _cache = new Map();
    const _observed = new WeakSet();

    function pixelateImage(img, px) {
        if (!img) return;
        const src = img.currentSrc || img.src;
        if (!src || src.startsWith('data:') || src.startsWith('blob:')) return;
        const key = src + '@' + px;
        if (_cache.has(key)) {
            const dataURL = _cache.get(key);
            if (img.src !== dataURL) img.src = dataURL;
            return;
        }
        if (!img.complete || !img.naturalWidth) return;
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
        } catch (_) { /* CORS / tainted canvas */ }
    }

    function watchImage(img, px) {
        if (!img || _observed.has(img)) return;
        _observed.add(img);
        // Pixelate on every load (covers dynamic src changes)
        img.addEventListener('load', () => pixelateImage(img, px));
    }

    function pixelateAll() {
        // Avatar <img> inside .avatar containers
        document.querySelectorAll('.avatar img, .avatar .persona-avatar-img').forEach(img => {
            watchImage(img, PIXEL_SMALL);
            pixelateImage(img, PIXEL_SMALL);
        });

        // Personality current icon (standalone <img>)
        const currentIcon = document.getElementById('personality-current-icon');
        if (currentIcon) { watchImage(currentIcon, PIXEL_SMALL); pixelateImage(currentIcon, PIXEL_SMALL); }

        // Personality preview image (appears on hover)
        const previewImg = document.getElementById('personality-preview-image');
        if (previewImg) { watchImage(previewImg, PIXEL_LARGE); pixelateImage(previewImg, PIXEL_LARGE); }

        // Any persona images in personality options / drawers
        document.querySelectorAll('.personality-option img, img[class*="persona"]').forEach(img => {
            watchImage(img, PIXEL_SMALL);
            pixelateImage(img, PIXEL_SMALL);
        });
    }

    function init() {
        if (document.documentElement.getAttribute('data-theme') !== '8bit') return;

        // Initial pass — delayed so images start loading
        setTimeout(pixelateAll, 400);

        // MutationObserver for new chat messages
        const chatBox = document.getElementById('chat-content') || document.getElementById('chat-box');
        if (chatBox && typeof MutationObserver !== 'undefined') {
            new MutationObserver(() => setTimeout(pixelateAll, 80))
                .observe(chatBox, { childList: true, subtree: true });
        }

        // Observe personality dropdown and preview panel for src changes
        const personalityPicker = document.querySelector('.personality-select-wrapper')
            || document.getElementById('personality-dropdown');
        if (personalityPicker && typeof MutationObserver !== 'undefined') {
            new MutationObserver(() => setTimeout(pixelateAll, 50))
                .observe(personalityPicker, { childList: true, subtree: true, attributes: true, attributeFilter: ['src'] });
        }

        // Re-run on theme change
        window.addEventListener('aurago:themechange', (e) => {
            if (e.detail && e.detail.theme === '8bit') setTimeout(pixelateAll, 400);
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();


