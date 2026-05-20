(() => {
    const THEME = 'lollipop';
    const CHAT_BOX_ID = 'chat-box';
    const LAYER_ID = 'lollipop-petal-layer';
    const PETAL_COUNT_DESKTOP = 20;
    const PETAL_COUNT_MOBILE = 12;
    const BLOSSOM_CLASSES = [
        'blossom-a', 'blossom-b', 'blossom-c', 'blossom-d',
        'blossom-e', 'blossom-f', 'blossom-star', 'blossom-heart'
    ];
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
        const chatBox = document.getElementById(CHAT_BOX_ID);
        const duration = randomBetween(14, 30);
        const size = randomBetween(24, 68);
        const left = randomBetween(2, 96);
        const drift = randomBetween(-70, 70);
        const opacity = randomBetween(0.4, 0.82);
        const scale = randomBetween(0.7, 1.2);
        const rotationStart = randomBetween(-30, 30);
        const rotationEnd = rotationStart + randomBetween(160, 360) * (Math.random() > 0.5 ? 1 : -1);
        // Use scrollHeight to cover full scrollable area, not just viewport
        const scrollH = chatBox ? chatBox.scrollHeight : window.innerHeight;
        const viewH = chatBox ? chatBox.clientHeight : window.innerHeight;
        const travel = Math.max(scrollH, viewH) + size + 160;
        const delay = immediate ? randomBetween(-duration, 0) : 0;
        // Sway amplitude for playful wobble
        const swayAmp = randomBetween(15, 45);
        const swaySpeed = randomBetween(2, 5);

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
        petal.style.setProperty('--petal-travel', `${travel}px`);
        petal.style.setProperty('--petal-sway', `${swayAmp}px`);
        petal.style.setProperty('--petal-sway-speed', `${swaySpeed.toFixed(1)}s`);
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
