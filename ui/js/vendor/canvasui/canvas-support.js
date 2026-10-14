/** CanvasUI helpers, a04a99b235299ee79a123a16ccb2e6cf9da74b6b.
 * https://github.com/DavidHDev/canvas-ui — MIT + Commons Clause (LICENSE.txt).
 * Vanilla JavaScript adaptation; TypeScript types removed.
 */
const canvasDimensionLimits = new WeakMap();

export function getCanvasPixelRatio(element, context) {
  const scale = window.visualViewport?.scale || 1;
  const requested = Math.min(window.devicePixelRatio || 1, 2) * scale;
  const width = Math.max(element.clientWidth, 1);
  const height = Math.max(element.clientHeight, 1);
  let maxDimension = 8192;
  if (context) {
    let limit = canvasDimensionLimits.get(context);
    if (limit === undefined) {
      limit = Math.min(maxDimension, context.getParameter(context.MAX_TEXTURE_SIZE), context.getParameter(context.MAX_RENDERBUFFER_SIZE));
      canvasDimensionLimits.set(context, limit);
    }
    maxDimension = limit;
  }
  return Math.min(requested, maxDimension / Math.max(width, height), Math.sqrt(16_777_216 / (width * height)));
}

export function createCanvasResizeObserver(onResize) {
  const viewport = window.visualViewport;
  let frame = 0;
  let disconnected = false;
  const schedule = () => {
    if (frame || disconnected) return;
    frame = requestAnimationFrame(() => { frame = 0; onResize(); });
  };
  const observer = new ResizeObserver(schedule);
  window.addEventListener('resize', schedule, { passive: true });
  viewport?.addEventListener('resize', schedule, { passive: true });
  return {
    observe(element, options) { observer.observe(element, options); },
    unobserve(element) { observer.unobserve(element); },
    disconnect() {
      disconnected = true;
      observer.disconnect();
      cancelAnimationFrame(frame);
      window.removeEventListener('resize', schedule);
      viewport?.removeEventListener('resize', schedule);
    },
  };
}

export function prepareHtmlInCanvas(source, content) {
  if ('content' in source) {
    source.setAttribute('content', 'drawable');
    content.setAttribute('drawable', '');
  } else source.setAttribute('layoutsubtree', '');
}

export function drawHtmlInCanvas(source, context, content) {
  const transform = context.drawElementImage(content, 0, 0);
  if (transform && typeof source.updateElementGeometry === 'function') {
    source.updateElementGeometry(content, { canvasTransform: transform });
  }
}

export function createRectCache(element) {
  let current = element.getBoundingClientRect();
  const refresh = () => { current = element.getBoundingClientRect(); };
  const observer = new ResizeObserver(refresh);
  observer.observe(element);
  window.addEventListener('resize', refresh, { passive: true });
  window.addEventListener('scroll', refresh, { capture: true, passive: true });
  return {
    get current() { return current; },
    destroy() {
      observer.disconnect();
      window.removeEventListener('resize', refresh);
      window.removeEventListener('scroll', refresh, true);
    },
  };
}
