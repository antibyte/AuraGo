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

    const PIXEL_SMALL = 36;
    const PIXEL_LARGE = 64;

    const _cache = new Map();
    const _observed = new WeakSet();

    function is8BitTheme() {
        return document.documentElement.getAttribute('data-theme') === '8bit';
    }

    function rememberOriginal(img, src) {
        img.dataset.aurago8bitSrc = src || img.getAttribute('src') || '';
        if (img.hasAttribute('srcset')) {
            img.dataset.aurago8bitSrcset = img.getAttribute('srcset') || '';
        }
    }

    function restoreImage(img) {
        if (!img || !img.dataset) return;
        const originalSrc = img.dataset.aurago8bitSrc;
        const originalSrcset = img.dataset.aurago8bitSrcset;
        if (originalSrcset !== undefined) {
            if (originalSrcset) img.setAttribute('srcset', originalSrcset);
            else img.removeAttribute('srcset');
        }
        if (originalSrc && img.getAttribute('src') !== originalSrc) {
            img.setAttribute('src', originalSrc);
        }
        delete img.dataset.aurago8bitSrc;
        delete img.dataset.aurago8bitSrcset;
        delete img.dataset.aurago8bitPixelated;
    }

    function pixelateImage(img, px) {
        if (!img) return;
        if (!is8BitTheme()) {
            restoreImage(img);
            return;
        }
        const src = img.currentSrc || img.src;
        if (!src || src.startsWith('data:') || src.startsWith('blob:')) return;
        rememberOriginal(img, src);
        const key = src + '@' + px;
        if (_cache.has(key)) {
            const dataURL = _cache.get(key);
            img.removeAttribute('srcset');
            if (img.src !== dataURL) img.src = dataURL;
            img.dataset.aurago8bitPixelated = 'true';
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
            img.removeAttribute('srcset');
            img.src = dataURL;
            img.dataset.aurago8bitPixelated = 'true';
        } catch (_) { /* CORS / tainted canvas */ }
    }

    function watchImage(img, px) {
        if (!img || _observed.has(img)) return;
        _observed.add(img);
        // Pixelate on every load in 8bit mode; restore if the theme changed away.
        img.addEventListener('load', () => is8BitTheme() ? pixelateImage(img, px) : restoreImage(img));
    }

    function targetImages() {
        const targets = [];
        // Avatar <img> inside .avatar containers
        document.querySelectorAll('.avatar img, .avatar .persona-avatar-img').forEach(img => {
            targets.push([img, PIXEL_SMALL]);
        });

        // Personality current icon (standalone <img>)
        const currentIcon = document.getElementById('personality-current-icon');
        if (currentIcon) targets.push([currentIcon, PIXEL_SMALL]);

        // Personality preview image (appears on hover)
        const previewImg = document.getElementById('personality-preview-image');
        if (previewImg) targets.push([previewImg, PIXEL_LARGE]);

        // Any persona images in personality options / drawers
        document.querySelectorAll('.personality-option img, img[class*="persona"]').forEach(img => {
            targets.push([img, PIXEL_SMALL]);
        });
        return targets;
    }

    function pixelateAll() {
        targetImages().forEach(([img, px]) => {
            watchImage(img, px);
            pixelateImage(img, px);
        });
    }

    function restoreAll() {
        document.querySelectorAll('img[data-aurago8bit-pixelated], img[data-aurago8bit-src]').forEach(restoreImage);
    }

    function sync() {
        if (is8BitTheme()) setTimeout(pixelateAll, 400);
        else restoreAll();
    }

    function init() {
        sync();

        // MutationObserver for new chat messages
        const chatBox = document.getElementById('chat-content') || document.getElementById('chat-box');
        if (chatBox && typeof MutationObserver !== 'undefined') {
            new MutationObserver(() => setTimeout(sync, 80))
                .observe(chatBox, { childList: true, subtree: true });
        }

        // Observe personality dropdown and preview panel for src changes
        const personalityPicker = document.querySelector('.personality-select-wrapper')
            || document.getElementById('personality-dropdown');
        if (personalityPicker && typeof MutationObserver !== 'undefined') {
            new MutationObserver(() => setTimeout(sync, 50))
                .observe(personalityPicker, { childList: true, subtree: true, attributes: true, attributeFilter: ['src'] });
        }

        window.addEventListener('aurago:themechange', sync);
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();



