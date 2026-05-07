    function widgetShouldAutoSize(widget) {
        if (!widget) return true;
        const configured = widget.auto_size !== undefined
            ? widget.auto_size
            : (widget.autoSize !== undefined ? widget.autoSize : widget.autosize);
        if (configured === undefined || configured === null || configured === '') return true;
        if (configured === false || configured === 0) return false;
        return String(configured).toLowerCase() !== 'false';
    }

    function scheduleWidgetAutoSize(card, widget) {
        if (!card || !widgetShouldAutoSize(widget)) return;
        card.dataset.widgetAutoSize = 'true';
        if (card._widgetResizeFrame) window.cancelAnimationFrame(card._widgetResizeFrame);
        const schedule = () => {
            if (!document.body.contains(card)) return;
            if (card._widgetResizeFrame) window.cancelAnimationFrame(card._widgetResizeFrame);
            card._widgetResizeFrame = window.requestAnimationFrame(() => applyWidgetAutoSize(card));
        };
        if (window.ResizeObserver && !card._widgetResizeObserver) {
            const observer = new ResizeObserver(schedule);
            observer.observe(card);
            ['.vd-widget-builtin', '.vd-widget-body', '.vd-widget-frame-wrap', '.vd-quickchat-response'].forEach(selector => {
                const target = card.querySelector(selector);
                if (target) observer.observe(target);
            });
            card._widgetResizeObserver = observer;
        }
        if (!card._widgetCleanupRegistered) {
            card._widgetCleanupRegistered = true;
            registerWidgetCleanup(() => {
                if (card._widgetResizeFrame) {
                    window.cancelAnimationFrame(card._widgetResizeFrame);
                    card._widgetResizeFrame = 0;
                }
                if (card._widgetResizeObserver) {
                    card._widgetResizeObserver.disconnect();
                    card._widgetResizeObserver = null;
                }
            });
        }
        schedule();
    }

    function applyWidgetAutoSize(card, payload) {
        if (!card || card.dataset.widgetAutoSize !== 'true') return;
        const data = payload && typeof payload === 'object' ? payload : {};
        const frameWrap = card.querySelector('.vd-widget-frame-wrap');
        const reportedFrameHeight = Number(data.height || data.h || 0);
        if (frameWrap && reportedFrameHeight > 0) {
            const frameHeight = clampWidgetFrameHeight(card, reportedFrameHeight + WIDGET_FRAME_SCROLLBAR_BUFFER);
            card.style.setProperty('--vd-widget-frame-height', frameHeight + 'px');
            frameWrap.style.setProperty('--vd-widget-frame-height', frameHeight + 'px');
        }
        const measuredContentHeight = widgetMeasuredContentHeight(card, data);
        const desiredHeight = Math.max(
            WIDGET_MIN_HEIGHT,
            Math.ceil(Number(data.cardHeight || data.card_height || 0)),
            measuredContentHeight,
            Math.ceil(card.scrollHeight || 0)
        );
        card.style.setProperty('--vd-widget-auto-height', clampWidgetHeight(card, desiredHeight, WIDGET_MIN_HEIGHT) + 'px');
    }

    function widgetMeasuredContentHeight(card, data) {
        if (!card) return 0;
        let bottom = 0;
        const frameWrap = card.querySelector('.vd-widget-frame-wrap');
        if (frameWrap) bottom = Math.max(bottom, widgetElementBottom(card, frameWrap));
        ['.vd-widget-builtin', '.vd-widget-body', '.vd-quickchat-response'].forEach(selector => {
            const target = card.querySelector(selector);
            if (target) bottom = Math.max(bottom, widgetElementBottom(card, target));
        });
        const requestedCardHeight = Number(data.cardHeight || data.card_height || 0);
        return Math.ceil(Math.max(bottom, requestedCardHeight, 0) + WIDGET_AUTO_SIZE_PADDING);
    }

    function widgetElementBottom(card, element) {
        if (!card || !element) return 0;
        const cardRect = typeof card.getBoundingClientRect === 'function' ? card.getBoundingClientRect() : null;
        const elementRect = typeof element.getBoundingClientRect === 'function' ? element.getBoundingClientRect() : null;
        const cardStyle = window.getComputedStyle ? window.getComputedStyle(card) : null;
        const paddingBottom = parseFloat(cardStyle && cardStyle.paddingBottom) || 0;
        const rectBottom = cardRect && elementRect ? elementRect.bottom - cardRect.top + paddingBottom : 0;
        const layoutBottom = (element.offsetTop || 0) + Math.max(element.scrollHeight || 0, element.offsetHeight || 0);
        return Math.ceil(Math.max(rectBottom, layoutBottom));
    }

    function resizeWidgetToContent(widgetId, payload) {
        const id = String(widgetId || '');
        if (!id) return;
        const card = document.querySelector(`.vd-widget[data-widget-id="${cssSel(id)}"]`);
        if (!card || card.dataset.widgetAutoSize !== 'true') return;
        const data = payload && typeof payload === 'object' ? payload : {};
        const reportedWidth = Number(data.width || data.w || 0);
        if (reportedWidth > 16) {
            const nextWidth = Math.max(220, Math.min(Math.ceil(reportedWidth), widgetMaxWidth(card)));
            card.style.width = nextWidth + 'px';
        }
        applyWidgetAutoSize(card, data);
    }

    function clampWidgetFrameHeight(card, height) {
        const available = Math.max(WIDGET_MIN_FRAME_HEIGHT, widgetAvailableHeight(card) - 32);
        return Math.max(WIDGET_MIN_FRAME_HEIGHT, Math.min(Math.ceil(height), available));
    }

    function clampWidgetHeight(card, height, minimum) {
        return Math.max(minimum, Math.min(Math.ceil(height), widgetAvailableHeight(card)));
    }

    function widgetAvailableHeight(card) {
        const workspace = $('vd-workspace');
        const workspaceHeight = (workspace && workspace.clientHeight) || window.innerHeight || 600;
        const top = parseInt(card.style.top, 10) || card.offsetTop || 0;
        return Math.max(WIDGET_MIN_HEIGHT, workspaceHeight - top - WIDGET_MAX_BOTTOM_GAP);
    }

    function widgetMaxWidth(card) {
        const workspace = $('vd-workspace');
        const workspaceWidth = (workspace && workspace.clientWidth) || window.innerWidth || 960;
        const left = parseInt(card.style.left, 10) || card.offsetLeft || 0;
        return Math.max(220, workspaceWidth - left - 18);
    }
