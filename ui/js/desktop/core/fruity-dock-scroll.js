    function wireFruityDockScroll(host) {
        const scroller = host && host.querySelector('[data-fruity-dock-scroll-region]');
        const track = host && host.querySelector('[data-fruity-dock-track]');
        if (!host || !scroller || !track) return;
        const queueUpdate = () => {
            const schedule = window.requestAnimationFrame || ((callback) => window.setTimeout(callback, 16));
            if (host._fruityDockScrollFrame) return;
            host._fruityDockScrollFrame = schedule(() => {
                host._fruityDockScrollFrame = 0;
                updateFruityDockScrollControls(host);
            });
        };
        host.querySelectorAll('[data-fruity-dock-scroll-button]').forEach(button => {
            button.addEventListener('click', event => {
                event.stopPropagation();
                const direction = button.dataset.fruityDockScrollButton === 'left' ? -1 : 1;
                const distance = Math.max(180, Math.floor(scroller.clientWidth * 0.72));
                scroller.scrollBy({ left: direction * distance, behavior: 'smooth' });
            });
        });
        scroller.addEventListener('scroll', queueUpdate, { passive: true });
        if (host._fruityDockResizeObserver) host._fruityDockResizeObserver.disconnect();
        if (window.ResizeObserver) {
            host._fruityDockResizeObserver = new ResizeObserver(queueUpdate);
            host._fruityDockResizeObserver.observe(scroller);
            host._fruityDockResizeObserver.observe(track);
        }
        queueUpdate();
    }

    function updateFruityDockScrollControls(host) {
        const scroller = host && host.querySelector('[data-fruity-dock-scroll-region]');
        if (!host || !scroller) return;
        const maxScroll = Math.max(0, scroller.scrollWidth - scroller.clientWidth);
        const overflowing = maxScroll > 2;
        const atStart = !overflowing || scroller.scrollLeft <= 2;
        const atEnd = !overflowing || scroller.scrollLeft >= maxScroll - 2;
        host.classList.toggle('vd-dock-overflowing', overflowing);
        host.classList.toggle('vd-dock-at-start', atStart);
        host.classList.toggle('vd-dock-at-end', atEnd);
        host.querySelectorAll('[data-fruity-dock-scroll-button]').forEach(button => {
            const left = button.dataset.fruityDockScrollButton === 'left';
            button.disabled = !overflowing || (left ? atStart : atEnd);
        });
    }
