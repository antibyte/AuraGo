/**
 * 8Bit Theme — Canvas-based real pixelation for <img> elements and the
 * chat background logo (#chat-box::after via --bg-logo).
 *
 * Images: draw onto a tiny canvas, swap src to the low-res data-URL;
 * image-rendering:pixelated upscales with nearest-neighbor.
 *
 * Background: downsample the logo to half viewport (2×2 CSS-pixel blocks),
 * then set --bg-logo to that bitmap so cover scaling stays crisp.
 */
(() => {
    'use strict';

    const PIXEL_SMALL = 36;
    const PIXEL_LARGE = 64;
    /** Each logical pixel of the downsampled background becomes a 2×2 CSS block. */
    const BG_BLOCK = 2;

    const _cache = new Map();
    const _observed = new WeakSet();
    let _originalBgLogo = '';
    let _originalBgLogoSize = '';
    let _bgToken = 0;
    let _bgResizeTimer = 0;

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
        img.addEventListener('load', () => is8BitTheme() ? pixelateImage(img, px) : restoreImage(img));
    }

    function targetImages() {
        const targets = [];
        document.querySelectorAll('.avatar img, .avatar .persona-avatar-img').forEach(img => {
            targets.push([img, PIXEL_SMALL]);
        });

        const currentIcon = document.getElementById('personality-current-icon');
        if (currentIcon) targets.push([currentIcon, PIXEL_SMALL]);

        const previewImg = document.getElementById('personality-preview-image');
        if (previewImg) targets.push([previewImg, PIXEL_LARGE]);

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

    function parseCssUrl(value) {
        const raw = String(value || '').trim();
        if (!raw) return '';
        const match = raw.match(/url\(\s*(['"]?)(.*?)\1\s*\)/i);
        return match ? match[2] : '';
    }

    function rememberBackgroundLogo() {
        if (_originalBgLogo) return;
        const computed = getComputedStyle(document.documentElement);
        const logo = computed.getPropertyValue('--bg-logo').trim();
        if (logo && !logo.includes('data:')) {
            _originalBgLogo = logo;
            _originalBgLogoSize = computed.getPropertyValue('--bg-logo-size').trim() || 'cover';
        }
    }

    function restoreBackground() {
        _bgToken += 1;
        const root = document.documentElement;
        root.style.removeProperty('--bg-logo');
        root.style.removeProperty('--bg-logo-size');
        root.removeAttribute('data-aurago8bit-bg');
        _originalBgLogo = '';
        _originalBgLogoSize = '';
    }

    function drawImageCover(ctx, image, width, height) {
        const iw = image.naturalWidth || image.width || 1;
        const ih = image.naturalHeight || image.height || 1;
        const scale = Math.max(width / iw, height / ih);
        const dw = iw * scale;
        const dh = ih * scale;
        const dx = (width - dw) / 2;
        const dy = (height - dh) / 2;
        ctx.drawImage(image, dx, dy, dw, dh);
    }

    function loadImage(url) {
        return new Promise((resolve, reject) => {
            const img = new Image();
            img.decoding = 'async';
            img.onload = () => resolve(img);
            img.onerror = () => reject(new Error('bg logo load failed'));
            img.src = url;
        });
    }

    async function pixelateBackground() {
        if (!is8BitTheme()) {
            restoreBackground();
            return;
        }
        rememberBackgroundLogo();
        const logoValue = _originalBgLogo || getComputedStyle(document.documentElement).getPropertyValue('--bg-logo').trim();
        const src = parseCssUrl(logoValue);
        if (!src || src.startsWith('data:')) return;

        const token = ++_bgToken;
        const vw = Math.max(1, window.innerWidth || document.documentElement.clientWidth || 1);
        const vh = Math.max(1, window.innerHeight || document.documentElement.clientHeight || 1);
        const tinyW = Math.max(1, Math.round(vw / BG_BLOCK));
        const tinyH = Math.max(1, Math.round(vh / BG_BLOCK));
        const cacheKey = src + '@bg@' + tinyW + 'x' + tinyH + '@' + BG_BLOCK;

        let dataURL = _cache.get(cacheKey);
        if (!dataURL) {
            try {
                const img = await loadImage(src);
                if (token !== _bgToken || !is8BitTheme()) return;
                const tiny = document.createElement('canvas');
                tiny.width = tinyW;
                tiny.height = tinyH;
                const tctx = tiny.getContext('2d');
                tctx.imageSmoothingEnabled = true;
                drawImageCover(tctx, img, tinyW, tinyH);
                // Re-sample nearest-neighbor so the bitmap itself is blocky when upscaled.
                const out = document.createElement('canvas');
                out.width = tinyW;
                out.height = tinyH;
                const octx = out.getContext('2d');
                octx.imageSmoothingEnabled = false;
                octx.drawImage(tiny, 0, 0, tinyW, tinyH);
                dataURL = out.toDataURL('image/png');
                _cache.set(cacheKey, dataURL);
            } catch (_) {
                return;
            }
        }
        if (token !== _bgToken || !is8BitTheme()) return;

        const root = document.documentElement;
        root.style.setProperty('--bg-logo', 'url("' + dataURL + '")');
        // Stretch the half-res bitmap to the full viewport so each texel is BG_BLOCK×BG_BLOCK.
        root.style.setProperty('--bg-logo-size', '100% 100%');
        root.setAttribute('data-aurago8bit-bg', 'pixelated-2x2');
    }

    function scheduleBackgroundPixelate() {
        if (_bgResizeTimer) window.clearTimeout(_bgResizeTimer);
        _bgResizeTimer = window.setTimeout(() => {
            _bgResizeTimer = 0;
            pixelateBackground();
        }, 80);
    }

    function sync() {
        if (is8BitTheme()) {
            setTimeout(pixelateAll, 400);
            scheduleBackgroundPixelate();
        } else {
            restoreAll();
            restoreBackground();
        }
    }

    function init() {
        sync();

        const chatBox = document.getElementById('chat-content') || document.getElementById('chat-box');
        if (chatBox && typeof MutationObserver !== 'undefined') {
            new MutationObserver(() => setTimeout(sync, 80))
                .observe(chatBox, { childList: true, subtree: true });
        }

        const personalityPicker = document.querySelector('.personality-select-wrapper')
            || document.getElementById('personality-dropdown');
        if (personalityPicker && typeof MutationObserver !== 'undefined') {
            new MutationObserver(() => setTimeout(sync, 50))
                .observe(personalityPicker, { childList: true, subtree: true, attributes: true, attributeFilter: ['src'] });
        }

        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', () => {
            if (is8BitTheme()) scheduleBackgroundPixelate();
        }, { passive: true });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
