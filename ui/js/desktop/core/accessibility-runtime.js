(function () {
    'use strict';

    const TEXT_SIZES = new Set(['100', '125', '150']);
    let current = { textSize: '100', highContrast: false };

    function normalize(settings) {
        settings = settings && typeof settings === 'object' ? settings : {};
        const requestedSize = String(settings['accessibility.text_size'] ?? settings.textSize ?? '100');
        const requestedContrast = settings['accessibility.high_contrast'] ?? settings.highContrast;
        return {
            textSize: TEXT_SIZES.has(requestedSize) ? requestedSize : '100',
            highContrast: requestedContrast === true || requestedContrast === 1 || String(requestedContrast).toLowerCase() === 'true' || String(requestedContrast) === '1'
        };
    }

    function apply(settings) {
        current = normalize(settings);
        const body = document.body;
        if (!body) return Object.assign({}, current);
        body.dataset.textSize = current.textSize;
        body.dataset.highContrast = String(current.highContrast);
        body.style.setProperty('--vd-accessibility-text-scale', String(Number(current.textSize) / 100));
        return Object.assign({}, current);
    }

    window.AuraDesktopAccessibility = {
        apply,
        current: () => Object.assign({}, current),
        normalize
    };
}());
