(() => {
    const mascot = document.getElementById('chat-robot-mascot');
    const footer = document.querySelector('.app-footer');
    const header = document.querySelector('.app-header');
    const chatContent = document.getElementById('chat-content');
    if (!mascot || !footer || !chatContent) return;

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
    let robotState = 'idle';
    let launchTimer = null;

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

    function currentRobotSize() {
        if (window.innerWidth <= 479) return 64;
        if (window.innerWidth <= 767) return 72;
        return 92;
    }

    function greetingIconEl() {
        return chatContent.querySelector('.greeting-icon-robot');
    }

    function greetingActive() {
        return !!chatContent.querySelector('[data-greeting]');
    }

    function placeRobot(metrics) {
        if (!metrics) return;
        mascot.style.left = `${Math.round(metrics.left)}px`;
        mascot.style.top = `${Math.round(metrics.top)}px`;
        mascot.style.bottom = 'auto';
        mascot.style.width = `${Math.round(metrics.size)}px`;
        mascot.style.height = `${Math.round(metrics.size)}px`;
    }

    function greetingMetrics() {
        const icon = greetingIconEl();
        if (!icon) return null;
        const rect = icon.getBoundingClientRect();
        const size = Math.max(rect.width, rect.height, window.innerWidth <= 767 ? 78 : 96);
        const verticalLift = window.innerWidth <= 767 ? 34 : 14;
        return {
            left: rect.left + ((rect.width - size) / 2),
            top: rect.top + ((rect.height - size) / 2) - verticalLift,
            size
        };
    }

    function anchorMetrics() {
        const size = currentRobotSize();
        if (window.innerWidth <= 767) {
            const headerRect = header ? header.getBoundingClientRect() : { bottom: 0 };
            return {
                left: 8,
                top: Math.round((headerRect.bottom || 0) + 6),
                size
            };
        }

        const footerRect = footer.getBoundingClientRect();
        return {
            left: Math.round(Math.min(Math.max(12, window.innerWidth * 0.021), 24)),
            top: Math.round(window.innerHeight - footerRect.height - 18 - size),
            size
        };
    }

    function applyRobotState(nextState) {
        robotState = nextState;
        mascot.classList.toggle('is-greeting', nextState === 'greeting');
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

    function syncPlacement(forceAnchor = false) {
        updateFooterOffset();
        const metrics = (!forceAnchor && greetingActive()) ? greetingMetrics() : anchorMetrics();
        applyRobotState((!forceAnchor && greetingActive()) ? 'greeting' : 'anchored');
        placeRobot(metrics);
    }

    function anchorImmediately() {
        window.clearTimeout(launchTimer);
        applyRobotState('anchored');
        placeRobot(anchorMetrics());
    }

    function launchToAnchor() {
        if (robotState === 'launching' || !greetingActive()) {
            anchorImmediately();
            return;
        }

        const startMetrics = greetingMetrics();
        const endMetrics = anchorMetrics();
        if (!startMetrics || !endMetrics || (reduceMotion && reduceMotion.matches)) {
            anchorImmediately();
            return;
        }

        applyRobotState('launching');
        placeRobot(startMetrics);
        window.clearTimeout(launchTimer);
        window.requestAnimationFrame(() => {
            window.requestAnimationFrame(() => {
                placeRobot(endMetrics);
            });
        });

        launchTimer = window.setTimeout(() => {
            anchorImmediately();
        }, 760);
    }

    function start() {
        syncPlacement(false);
        setFrame(0);
        tick();
    }

    if (typeof ResizeObserver !== 'undefined') {
        const observer = new ResizeObserver(updateFooterOffset);
        observer.observe(footer);
        if (header) observer.observe(header);
        observer.observe(chatContent);
    }

    window.addEventListener('resize', () => {
        if (robotState !== 'launching') {
            syncPlacement(robotState !== 'greeting');
        }
    }, { passive: true });
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

    window.ChatRobotMascot = {
        launchToAnchor,
        anchorImmediately,
        resetGreeting() {
            syncPlacement(false);
        }
    };

    start();
})();
