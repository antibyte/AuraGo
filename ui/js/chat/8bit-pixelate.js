/**
 * 8Bit Theme — Canvas-based real pixelation for <img> elements.
 *
 * Draws each target image onto a tiny canvas (e.g. 8×8),
 * then replaces the src with the low-res data-URL.
 * image-rendering:pixelated on the element does nearest-neighbor
 * upscaling = visible pixel blocks from actual source pixels.
 *
 * NOTE: Does NOT touch the robot mascot sprite — it uses a CSS
 * background-position animation that breaks if the image is replaced.
 */
(() => {
    'use strict';

    const PIXEL_SMALL = 8;
    const PIXEL_LARGE = 16;

    const _cache = new Map();
    const _processing = new WeakSet();

    function pixelateImage(img, px) {
        if (!img || _processing.has(img)) return;
        const src = img.currentSrc || img.src;
        if (!src || src.startsWith('data:') || src.startsWith('blob:')) return;
        const key = src + '@' + px;
        if (_cache.has(key)) {
            img.src = _cache.get(key);
            return;
        }
        // Wait for image to load if not ready
        if (!img.complete || !img.naturalWidth) {
            _processing.add(img);
            img.addEventListener('load', () => { _processing.delete(img); pixelateImage(img, px); }, { once: true });
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
        } catch (_) { /* CORS / tainted canvas */ }
    }

    function pixelateAll() {
        // Avatar <img> inside .avatar containers
        document.querySelectorAll('.avatar img, .avatar .persona-avatar-img').forEach(img => {
            pixelateImage(img, PIXEL_SMALL);
        });

        // Personality current icon (standalone <img>)
        const currentIcon = document.getElementById('personality-current-icon');
        if (currentIcon) pixelateImage(currentIcon, PIXEL_SMALL);

        // Personality preview image (appears on hover)
        const previewImg = document.getElementById('personality-preview-image');
        if (previewImg) pixelateImage(previewImg, PIXEL_LARGE);

        // Any persona images in personality options / drawers
        document.querySelectorAll('.personality-option img, .personality-option .chat-ui-icon, img[class*="persona"]').forEach(img => {
            pixelateImage(img, PIXEL_SMALL);
        });
    }

    function init() {
        if (document.documentElement.getAttribute('data-theme') !== '8bit') return;

        // Initial pass
        setTimeout(pixelateAll, 400);

        // MutationObserver for new chat messages
        const chatBox = document.getElementById('chat-content') || document.getElementById('chat-box');
        if (chatBox && typeof MutationObserver !== 'undefined') {
            new MutationObserver(() => setTimeout(pixelateAll, 80))
                .observe(chatBox, { childList: true, subtree: true });
        }

        // Also observe the personality dropdown for preview image changes
        const personalityDropdown = document.querySelector('.personality-dropdown') || document.getElementById('personality-picker');
        if (personalityDropdown && typeof MutationObserver !== 'undefined') {
            new MutationObserver(() => setTimeout(pixelateAll, 50))
                .observe(personalityDropdown, { childList: true, subtree: true, attributes: true });
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