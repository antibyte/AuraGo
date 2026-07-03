(() => {
    'use strict';

    const THEME = 'dark-sun';
    const CHAT_BOX_ID = 'chat-box';
    const LAYER_ID = 'dark-sun-ember-layer';
    const ASH_COUNT_DESKTOP = 16;
    const ASH_COUNT_MOBILE = 8;
    const EMBER_COUNT_DESKTOP = 12;
    const EMBER_COUNT_MOBILE = 5;
    const ASH_CLASS = 'dark-sun-ash';
    const EMBER_CLASS = 'dark-sun-ember';
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

    function ashCount() {
        return mobileQuery && mobileQuery.matches ? ASH_COUNT_MOBILE : ASH_COUNT_DESKTOP;
    }

    function emberCount() {
        return mobileQuery && mobileQuery.matches ? EMBER_COUNT_MOBILE : EMBER_COUNT_DESKTOP;
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

    function rerollAsh(particle, immediate) {
        const chatBox = document.getElementById(CHAT_BOX_ID);
        const duration = randomBetween(40, 70);
        const size = randomBetween(3, 7);
        const left = randomBetween(2, 96);
        const sway = randomBetween(-60, 60);
        const opacity = randomBetween(0.12, 0.28);
        const rotateStart = randomBetween(-30, 30);
        const rotateEnd = rotateStart + randomBetween(20, 90) * (Math.random() > 0.5 ? 1 : -1);
        const scrollH = chatBox ? chatBox.scrollHeight : window.innerHeight;
        const viewH = chatBox ? chatBox.clientHeight : window.innerHeight;
        const travel = Math.max(scrollH, viewH) + size + 120;
        const delay = immediate ? randomBetween(-duration, 0) : 0;

        particle.className = ASH_CLASS;
        particle.style.setProperty('--ember-x', `${left}%`);
        particle.style.setProperty('--ember-size', `${size}px`);
        particle.style.setProperty('--ember-duration', `${duration}s`);
        particle.style.setProperty('--ember-delay', `${delay}s`);
        particle.style.setProperty('--ember-opacity', opacity.toFixed(2));
        particle.style.setProperty('--ember-travel', `${travel}px`);
        particle.style.setProperty('--ember-sway', `${sway}px`);
        particle.style.setProperty('--ember-rotate-start', `${rotateStart.toFixed(1)}deg`);
        particle.style.setProperty('--ember-rotate-end', `${rotateEnd.toFixed(1)}deg`);
        particle.style.zIndex = String(randomInt(1, 2));
    }

    function rerollEmber(particle, immediate) {
        const chatBox = document.getElementById(CHAT_BOX_ID);
        const duration = randomBetween(20, 35);
        const size = randomBetween(2, 5);
        const left = randomBetween(2, 96);
        const sway = randomBetween(-40, 40);
        const opacity = randomBetween(0.45, 0.88);
        const rotateStart = randomBetween(-40, 40);
        const rotateEnd = rotateStart + randomBetween(80, 220) * (Math.random() > 0.5 ? 1 : -1);
        const scrollH = chatBox ? chatBox.scrollHeight : window.innerHeight;
        const viewH = chatBox ? chatBox.clientHeight : window.innerHeight;
        const travel = Math.max(scrollH, viewH) + size + 160;
        const delay = immediate ? randomBetween(-duration, 0) : 0;

        particle.className = EMBER_CLASS;
        particle.style.setProperty('--ember-x', `${left}%`);
        particle.style.setProperty('--ember-size', `${size}px`);
        particle.style.setProperty('--ember-duration', `${duration}s`);
        particle.style.setProperty('--ember-delay', `${delay}s`);
        particle.style.setProperty('--ember-opacity', opacity.toFixed(2));
        particle.style.setProperty('--ember-travel', `${travel}px`);
        particle.style.setProperty('--ember-sway', `${sway}px`);
        particle.style.setProperty('--ember-rotate-start', `${rotateStart.toFixed(1)}deg`);
        particle.style.setProperty('--ember-rotate-end', `${rotateEnd.toFixed(1)}deg`);
        particle.style.zIndex = String(randomInt(2, 4));
    }

    function createParticle(rerollFn) {
        const particle = document.createElement('span');
        rerollFn(particle, true);
        return particle;
    }

    function fillParticles(layer, typeClass, desiredCount, rerollFn) {
        const live = layer.getElementsByClassName(typeClass);
        while (live.length < desiredCount) {
            layer.appendChild(createParticle(rerollFn));
        }
        while (live.length > desiredCount) {
            const last = live[live.length - 1];
            if (last) last.remove();
        }
        Array.from(live).forEach((p) => rerollFn(p, true));
    }

    function fillLayer(layer) {
        fillParticles(layer, ASH_CLASS, ashCount(), rerollAsh);
        fillParticles(layer, EMBER_CLASS, emberCount(), rerollEmber);
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
