    function animationsEnabled() {
        if (document.body && document.body.dataset.animations === 'false') return false;
        return !(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function animateThen(element, className, fallbackMs, done) {
        if (!element || !className || !animationsEnabled()) {
            if (typeof done === 'function') done();
            return;
        }
        let finished = false, timer = 0;
        const finish = () => {
            if (finished) return;
            finished = true;
            element.removeEventListener('animationend', onEnd);
            element.removeEventListener('transitionend', onEnd);
            element.classList.remove(className);
            if (timer) window.clearTimeout(timer);
            if (typeof done === 'function') done();
        };
        const onEnd = event => { if (event.target === element) finish(); };
        element.classList.remove(className);
        void element.offsetWidth;
        element.classList.add(className);
        element.addEventListener('animationend', onEnd);
        element.addEventListener('transitionend', onEnd);
        timer = window.setTimeout(finish, Math.max(20, Number(fallbackMs) || 160));
    }

    function closeWindowMenu() {
        document.querySelectorAll('.vd-window-menu.open').forEach(menu => {
            const popover = menu.querySelector(':scope > .vd-window-menu-popover');
            if (!animationsEnabled() || !popover) {
                menu.classList.remove('open', 'closing');
                return;
            }
            menu.classList.add('closing');
            animateThen(popover, 'vd-window-menu-popover-closing', isFruityTheme() ? 150 : 100, () => {
                menu.classList.remove('open', 'closing');
            });
        });
        state.openWindowMenu = null;
    }
