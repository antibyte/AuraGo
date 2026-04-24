(() => {
    const mascot = document.getElementById('chat-robot-mascot');
    const footer = document.querySelector('.app-footer');
    const header = document.querySelector('.app-header');
    const chatContent = document.getElementById('chat-content');
    if (!mascot || !footer || !chatContent) return;

    const reduceMotion = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)');
    const animationFrameCount = 6;
    const animations = [
        { name: 'idle', row: 0, frames: [0, 1, 2, 3, 4, 5], hold: 1200, weight: 3 },
        { name: 'wave', row: 1, frames: [0, 1, 2, 3, 4, 5], hold: 1600, weight: 2 },
        { name: 'happy', row: 2, frames: [0, 1, 2, 3, 4, 5], hold: 1600, weight: 2 },
        { name: 'sleepy', row: 3, frames: [0, 1, 2, 3, 4, 5], hold: 1900, weight: 1 },
        { name: 'curious', row: 4, frames: [0, 1, 2, 3, 4, 5], hold: 1600, weight: 2 },
        { name: 'thinking', row: 5, frames: [0, 1, 2, 3, 4, 5], hold: 1700, weight: 2 }
    ];
    const animationPool = animations.flatMap(animation => Array.from({ length: animation.weight }, () => animation));

    let timer = null;
    let activeAnimation = null;
    let animationFrameIndex = 0;
    let robotState = 'idle';
    let launchTimer = null;

    function setFrame(rowIndex, colIndex = 0) {
        const row = Math.max(0, Math.min(animations.length - 1, Number(rowIndex) || 0));
        const col = Math.max(0, Math.min(animationFrameCount - 1, Number(colIndex) || 0));
        mascot.style.setProperty('--chat-robot-row', String(row));
        mascot.style.setProperty('--chat-robot-col', String(col));
    }

    function frameDuration(animation, colIndex) {
        if (!animation) return 260;
        if (colIndex === 0 || colIndex === animationFrameCount - 1) return 230;
        if (animation.name === 'idle') return colIndex === 4 ? 210 : 180;
        if (animation.name === 'sleepy') return colIndex >= 1 && colIndex <= 3 ? 260 : 190;
        return 150;
    }

    function schedule(delay) {
        window.clearTimeout(timer);
        timer = window.setTimeout(tick, delay);
    }

    function chooseAnimation() {
        activeAnimation = animationPool[Math.floor(Math.random() * animationPool.length)] || animations[0];
        animationFrameIndex = 0;
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
            setFrame(0, 0);
            schedule(1600);
            return;
        }

        if (!activeAnimation) {
            setFrame(0, 0);
            chooseAnimation();
            schedule(1100 + Math.random() * 2200);
            return;
        }

        const colIndex = activeAnimation.frames[animationFrameIndex] || 0;
        setFrame(activeAnimation.row, colIndex);
        animationFrameIndex += 1;

        if (animationFrameIndex >= activeAnimation.frames.length) {
            const hold = activeAnimation.hold || 1500;
            activeAnimation = null;
            animationFrameIndex = 0;
            schedule(hold + Math.random() * 2200);
            return;
        }

        schedule(frameDuration(activeAnimation, colIndex));
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
        setFrame(0, 0);
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
            setFrame(0, 0);
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
