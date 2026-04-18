(() => {
    const THEME = 'lollipop';
    const CHAT_BOX_ID = 'chat-box';
    const LAYER_ID = 'lollipop-petal-layer';
    const PETAL_COUNT_DESKTOP = 16;
    const PETAL_COUNT_MOBILE = 10;
    const BLOSSOM_CLASSES = ['blossom-a', 'blossom-b', 'blossom-c', 'blossom-d'];
    const motionQuery = window.matchMedia ? window.matchMedia('(prefers-reduced-motion: reduce)') : null;
    const mobileQuery = window.matchMedia ? window.matchMedia('(max-width: 767px)') : null;

    function currentTheme() {
        return document.documentElement.getAttribute('data-theme') || 'dark';
    }

    function isEnabled() {
        return currentTheme() === THEME && !(motionQuery && motionQuery.matches);
    }

    function randomBetween(min, max) {
        return min + Math.random() * (max - min);
    }

    function randomInt(min, max) {
        return Math.round(randomBetween(min, max));
    }

    function pick(values) {
        return values[Math.floor(Math.random() * values.length)];
    }

    function desiredCount() {
        return mobileQuery && mobileQuery.matches ? PETAL_COUNT_MOBILE : PETAL_COUNT_DESKTOP;
    }

    function ensureLayer() {
        const chatBox = document.getElementById(CHAT_BOX_ID);
        if (!chatBox) return null;

        let layer = document.getElementById(LAYER_ID);
        if (!layer) {
            layer = document.createElement('div');
            layer.id = LAYER_ID;
            layer.setAttribute('aria-hidden', 'true');
            chatBox.insertBefore(layer, chatBox.firstChild);
        }
        return layer;
    }

    function rerollPetal(petal, immediate) {
        const duration = randomBetween(15, 28);
        const size = randomBetween(30, 78);
        const left = randomBetween(2, 94);
        const drift = randomBetween(-56, 56);
        const opacity = randomBetween(0.44, 0.84);
        const scale = randomBetween(0.72, 1.16);
        const rotationStart = randomBetween(-28, 28);
        const rotationEnd = rotationStart + randomBetween(150, 320);
        const delay = immediate ? randomBetween(-duration, 0) : 0;

        petal.className = `lollipop-petal ${pick(BLOSSOM_CLASSES)}`;
        petal.style.setProperty('--petal-left', `${left}%`);
        petal.style.setProperty('--petal-size', `${size}px`);
        petal.style.setProperty('--petal-drift', `${drift}px`);
        petal.style.setProperty('--petal-duration', `${duration}s`);
        petal.style.setProperty('--petal-delay', `${delay}s`);
        petal.style.setProperty('--petal-opacity', opacity.toFixed(2));
        petal.style.setProperty('--petal-scale', scale.toFixed(2));
        petal.style.setProperty('--petal-rotate-start', `${rotationStart.toFixed(1)}deg`);
        petal.style.setProperty('--petal-rotate-end', `${rotationEnd.toFixed(1)}deg`);
        petal.style.zIndex = String(randomInt(1, 3));
    }

    function createPetal() {
        const petal = document.createElement('span');
        rerollPetal(petal, true);
        petal.addEventListener('animationiteration', () => {
            rerollPetal(petal, false);
        });
        return petal;
    }

    function fillLayer(layer) {
        const count = desiredCount();
        while (layer.childElementCount < count) {
            layer.appendChild(createPetal());
        }
        while (layer.childElementCount > count) {
            layer.lastElementChild.remove();
        }
        Array.from(layer.children).forEach((petal) => rerollPetal(petal, true));
    }

    function sync() {
        const layer = ensureLayer();
        if (!layer) return;

        if (!isEnabled()) {
            layer.hidden = true;
            return;
        }

        layer.hidden = false;
        fillLayer(layer);
    }

    document.addEventListener('DOMContentLoaded', sync);
    window.addEventListener('aurago:themechange', sync);

    if (motionQuery) {
        if (typeof motionQuery.addEventListener === 'function') {
            motionQuery.addEventListener('change', sync);
        } else if (typeof motionQuery.addListener === 'function') {
            motionQuery.addListener(sync);
        }
    }

    if (mobileQuery) {
        if (typeof mobileQuery.addEventListener === 'function') {
            mobileQuery.addEventListener('change', sync);
        } else if (typeof mobileQuery.addListener === 'function') {
            mobileQuery.addListener(sync);
        }
    }

    if (document.readyState !== 'loading') {
        sync();
    }
})();
