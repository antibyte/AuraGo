(function () {
    'use strict';

    let active = false;
    let rafId = null;
    let observer = null;
    let chatBox = null;
    let chatContent = null;
    const readyTimers = new WeakMap();

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'threedee' && !prefersReducedMotion();
    }

    function clamp(min, max, value) {
        return Math.max(min, Math.min(max, value));
    }

    function getRows() {
        return chatContent ? Array.from(chatContent.querySelectorAll('.msg-row')) : [];
    }

    function markReady(row) {
        if (!row || row.classList.contains('threedee-fold-ready')) return;
        if (readyTimers.has(row)) return;
        const makeReady = () => {
            if (!active || !shouldRun()) return;
            row.classList.add('threedee-fold-ready');
            readyTimers.delete(row);
            scheduleUpdate();
        };
        row.addEventListener('animationend', makeReady, { once: true });
        readyTimers.set(row, window.setTimeout(makeReady, 520));
    }

    function clearRow(row) {
        if (!row) return;
        row.classList.remove('folding', 'folded', 'unfolding', 'threedee-fold-ready');
        row.style.transform = '';
        row.style.opacity = '';
        row.style.filter = '';
        const timer = readyTimers.get(row);
        if (timer) window.clearTimeout(timer);
        readyTimers.delete(row);
    }

    function updateRows() {
        rafId = null;
        if (!active || !chatBox || !chatContent) return;

        const chatRect = chatBox.getBoundingClientRect();
        getRows().forEach((row) => {
            markReady(row);
            if (!row.classList.contains('threedee-fold-ready')) return;

            const rect = row.getBoundingClientRect();
            const aboveBy = chatRect.top - rect.bottom;
            const partial = chatRect.top - rect.top;

            if (aboveBy > 0) {
                const foldAngle = clamp(2, 6, aboveBy / 22);
                row.classList.add('folding', 'folded');
                row.classList.remove('unfolding');
                row.style.transform = `perspective(800px) rotateX(-${foldAngle.toFixed(2)}deg) translateY(-2px) scale(0.995)`;
                row.style.opacity = String(clamp(0.72, 0.94, 1 - foldAngle / 54));
                row.style.filter = `blur(${clamp(0, 0.28, foldAngle / 42).toFixed(2)}px)`;
            } else if (partial > 0 && rect.height > 0) {
                const progress = clamp(0, 1, partial / rect.height);
                const foldAngle = progress * 4;
                row.classList.add('folding');
                row.classList.remove('folded', 'unfolding');
                row.style.transform = `perspective(800px) rotateX(-${foldAngle.toFixed(2)}deg) scale(${(1 - progress * 0.003).toFixed(3)})`;
                row.style.opacity = String(clamp(0.88, 1, 1 - progress * 0.08));
                row.style.filter = progress > 0.72 ? `blur(${((progress - 0.72) * 0.35).toFixed(2)}px)` : '';
            } else {
                const wasFolded = row.classList.contains('folded') || row.style.transform;
                row.classList.remove('folding', 'folded');
                if (wasFolded) row.classList.add('unfolding');
                row.style.transform = '';
                row.style.opacity = '';
                row.style.filter = '';
                if (wasFolded) {
                    window.setTimeout(() => row.classList.remove('unfolding'), 420);
                }
            }
        });
    }

    function scheduleUpdate() {
        if (!active || rafId) return;
        rafId = window.requestAnimationFrame(updateRows);
    }

    function setupObserver() {
        if (!chatContent || observer || typeof MutationObserver === 'undefined') return;
        observer = new MutationObserver((mutations) => {
            mutations.forEach((mutation) => {
                mutation.addedNodes.forEach((node) => {
                    if (node.nodeType === 1 && node.classList && node.classList.contains('msg-row')) markReady(node);
                });
            });
            scheduleUpdate();
        });
        observer.observe(chatContent, { childList: true });
    }

    function start() {
        if (active || !shouldRun()) return;
        chatBox = document.getElementById('chat-box');
        chatContent = document.getElementById('chat-content');
        if (!chatBox || !chatContent) return;
        active = true;
        getRows().forEach(markReady);
        setupObserver();
        chatBox.addEventListener('scroll', scheduleUpdate, { passive: true });
        window.addEventListener('resize', scheduleUpdate);
        scheduleUpdate();
    }

    function stop() {
        active = false;
        if (rafId) {
            window.cancelAnimationFrame(rafId);
            rafId = null;
        }
        if (chatBox) chatBox.removeEventListener('scroll', scheduleUpdate);
        window.removeEventListener('resize', scheduleUpdate);
        if (observer) {
            observer.disconnect();
            observer = null;
        }
        getRows().forEach(clearRow);
    }

    function sync() {
        if (shouldRun()) start();
        else stop();
    }

    function init() {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', init, { once: true });
            return;
        }
        window.addEventListener('aurago:themechange', sync);
        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            if (mq.addEventListener) mq.addEventListener('change', sync);
            else if (mq.addListener) mq.addListener(sync);
        }
        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true,
                attributeFilter: ['data-theme']
            });
        }
        sync();
    }

    window.AuraGoThreeDeeFold = { start, stop, sync };
    init();
})();
