(() => {
    const mascot = document.getElementById('chat-robot-mascot');
    const footer = document.querySelector('.app-footer');
    if (!mascot || !footer) return;

    const reduceMotion = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)');
    const gestures = [
        [0, 1, 0],
        [0, 2, 0],
        [0, 5, 0],
        [0, 10, 0],
        [0, 6, 7, 6, 0],
        [0, 12, 15, 12, 0],
        [0, 3, 4, 8, 9, 8, 4, 0],
        [0, 13, 14, 13, 0]
    ];

    let timer = null;
    let activeGesture = null;
    let gestureIndex = 0;

    function setFrame(frameIndex) {
        const clamped = Math.max(0, Math.min(15, Number(frameIndex) || 0));
        const row = Math.floor(clamped / 4);
        const col = clamped % 4;
        mascot.style.setProperty('--chat-robot-row', String(row));
        mascot.style.setProperty('--chat-robot-col', String(col));
    }

    function frameDuration(frameIndex) {
        if (frameIndex === 0) return 260;
        if (frameIndex === 2 || frameIndex === 5 || frameIndex === 10 || frameIndex === 15) return 210;
        return 150;
    }

    function schedule(delay) {
        window.clearTimeout(timer);
        timer = window.setTimeout(tick, delay);
    }

    function chooseGesture() {
        const next = gestures[Math.floor(Math.random() * gestures.length)];
        activeGesture = next;
        gestureIndex = 0;
    }

    function tick() {
        if (document.hidden || (reduceMotion && reduceMotion.matches)) {
            setFrame(0);
            schedule(1600);
            return;
        }

        if (!activeGesture) {
            setFrame(0);
            chooseGesture();
            schedule(1100 + Math.random() * 2200);
            return;
        }

        const frameIndex = activeGesture[gestureIndex];
        setFrame(frameIndex);
        gestureIndex += 1;

        if (gestureIndex >= activeGesture.length) {
            activeGesture = null;
            gestureIndex = 0;
            schedule(1300 + Math.random() * 2600);
            return;
        }

        schedule(frameDuration(frameIndex));
    }

    function updateFooterOffset() {
        const footerHeight = Math.ceil(footer.getBoundingClientRect().height || 0);
        const extraClearance = window.innerWidth <= 767 ? 12 : 18;
        mascot.style.setProperty('--chat-robot-footer-offset', `${footerHeight + extraClearance}px`);
    }

    function start() {
        updateFooterOffset();
        setFrame(0);
        tick();
    }

    if (typeof ResizeObserver !== 'undefined') {
        const observer = new ResizeObserver(updateFooterOffset);
        observer.observe(footer);
    }

    window.addEventListener('resize', updateFooterOffset, { passive: true });
    document.addEventListener('visibilitychange', () => {
        if (document.hidden) {
            window.clearTimeout(timer);
            setFrame(0);
            return;
        }
        tick();
    });

    if (reduceMotion && typeof reduceMotion.addEventListener === 'function') {
        reduceMotion.addEventListener('change', () => {
            window.clearTimeout(timer);
            start();
        });
    }

    start();
})();
