/* Homepage Studio annotate mode: captures a snapshot of the previewed page,
   lets the user mark elements (click) or areas (drag) with a comment, shows
   the marks as chips above the composer and hands them to the next chat turn
   as a composed image plus element details. */
(function () {
    'use strict';

    const MAX_MARKS = 20;
    const DRAG_THRESHOLD = 6;
    const CROP_MARGIN = 48;
    const IMAGE_MAX_WIDTH = 1000;
    const IMAGE_MAX_HEIGHT = 1600;
    const IMAGE_GAP = 16;
    /* The chat body limit is 2 MiB; the image stays well below it. */
    const IMAGE_MAX_BASE64 = 1400000;
    const AREA_OVERLAP_LIMIT = 5;
    const MARK_COLOR = '#e8590c';
    const DEVICE_WIDTHS = { desktop: 1280, tablet: 768, mobile: 390 };
    const SNAPSHOT_ENDPOINT = '/api/desktop/integrations/homepage/annotation-snapshot';
    const ERROR_CODES = ['capture_busy', 'browser_unavailable', 'capture_timeout', 'preview_unavailable', 'invalid_path', 'invalid_viewport', 'invalid_target', 'capture_blocked', 'capture_failed'];

    function boxArea(box) {
        return Math.max(0, box[2]) * Math.max(0, box[3]);
    }

    function boxContains(box, x, y) {
        return x >= box[0] && y >= box[1] && x <= box[0] + box[2] && y <= box[1] + box[3];
    }

    /* Smallest element box under the point; on equal size the later (deeper)
       element wins. Returns the element index or -1. */
    function hitTest(elements, x, y) {
        let best = -1;
        let bestArea = Infinity;
        (elements || []).forEach((item, index) => {
            if (!item || !Array.isArray(item.box) || !boxContains(item.box, x, y)) return;
            const area = boxArea(item.box);
            if (area <= bestArea) {
                best = index;
                bestArea = area;
            }
        });
        return best;
    }

    function normalizeBox(x0, y0, x1, y1) {
        return [Math.round(Math.min(x0, x1)), Math.round(Math.min(y0, y1)), Math.round(Math.abs(x1 - x0)), Math.round(Math.abs(y1 - y0))];
    }

    function intersectionArea(a, b) {
        const w = Math.min(a[0] + a[2], b[0] + b[2]) - Math.max(a[0], b[0]);
        const h = Math.min(a[1] + a[3], b[1] + b[3]) - Math.max(a[1], b[1]);
        return w > 0 && h > 0 ? w * h : 0;
    }

    /* Elements that lie at least half inside the area, largest first. */
    function overlapping(elements, box, limit) {
        return (elements || [])
            .map((item, index) => ({ item, index }))
            .filter(entry => entry.item && Array.isArray(entry.item.box) && boxArea(entry.item.box) > 0
                && intersectionArea(entry.item.box, box) >= boxArea(entry.item.box) / 2)
            .sort((a, b) => boxArea(b.item.box) - boxArea(a.item.box))
            .slice(0, limit || AREA_OVERLAP_LIMIT)
            .map(entry => entry.index);
    }

    /* Significant elements in reading order, for keyboard stepping. */
    function tabOrder(elements) {
        return (elements || [])
            .map((item, index) => ({ item, index }))
            .filter(entry => entry.item && entry.item.tab && Array.isArray(entry.item.box))
            .sort((a, b) => (a.item.box[1] - b.item.box[1]) || (a.item.box[0] - b.item.box[0]))
            .map(entry => entry.index);
    }

    /* The persisted user message carries only the user's own comments. */
    function composeMessage(text, marks, labels) {
        const base = String(text || '').trim() || labels.defaultMessage;
        if (!marks || !marks.length) return base;
        const lines = marks.map(mark => mark.n + '. ' + String(mark.comment || '').replace(/\s+/g, ' ').trim());
        return base + '\n\n' + labels.heading + '\n' + lines.join('\n');
    }

    function annotationPayload(snapshot, marks) {
        const elements = (snapshot && snapshot.elements) || [];
        const brief = index => {
            const item = elements[index];
            return item ? { selector: item.selector, tag: item.tag, text: item.text || '' } : null;
        };
        return {
            page_path: snapshot ? snapshot.path : '/',
            viewport_width: snapshot && snapshot.viewport ? snapshot.viewport.width : 0,
            items: (marks || []).map(mark => {
                const item = { n: mark.n, kind: mark.kind, comment: mark.comment, box: mark.box.slice() };
                if (mark.kind === 'element') {
                    const target = elements[mark.elementIndex] || {};
                    item.selector = target.selector || '';
                    item.tag = target.tag || '';
                    item.text = target.text || '';
                    if (target.attrs && Object.keys(target.attrs).length) item.attrs = Object.assign({}, target.attrs);
                } else {
                    item.elements = (mark.overlapIndexes || []).map(brief).filter(Boolean);
                }
                return item;
            })
        };
    }

    /* Crops for the composed image: every mark with a margin, stacked
       vertically and scaled into IMAGE_MAX_WIDTH × IMAGE_MAX_HEIGHT. */
    function cropLayout(marks, page) {
        const pageW = Math.max(1, page.width);
        const pageH = Math.max(1, page.height);
        const crops = (marks || []).map(mark => {
            const x = Math.max(0, mark.box[0] - CROP_MARGIN);
            const y = Math.max(0, mark.box[1] - CROP_MARGIN);
            const right = Math.min(pageW, mark.box[0] + mark.box[2] + CROP_MARGIN);
            const bottom = Math.min(pageH, mark.box[1] + mark.box[3] + CROP_MARGIN);
            const src = [x, y, Math.max(1, right - x), Math.max(1, bottom - y)];
            return { n: mark.n, box: mark.box, src, scale: Math.min(1, IMAGE_MAX_WIDTH / src[2]) };
        });
        const gaps = Math.max(0, crops.length - 1) * IMAGE_GAP;
        /* One pixel per crop of slack absorbs the rounding below. */
        const budget = IMAGE_MAX_HEIGHT - gaps - crops.length;
        const natural = crops.reduce((sum, crop) => sum + crop.src[3] * crop.scale, 0);
        const fit = natural > budget ? Math.max(0.05, budget / natural) : 1;
        let y = 0;
        let width = 1;
        crops.forEach(crop => {
            crop.scale *= fit;
            crop.dst = [0, Math.round(y), Math.max(1, Math.round(crop.src[2] * crop.scale)), Math.max(1, Math.round(crop.src[3] * crop.scale))];
            width = Math.max(width, crop.dst[2]);
            y = crop.dst[1] + crop.dst[3] + IMAGE_GAP;
        });
        return { crops, width, height: Math.max(1, Math.round(y - (crops.length ? IMAGE_GAP : 0))) };
    }

    window.HomepageStudioAnnotate = { hitTest, overlapping, normalizeBox, tabOrder, composeMessage, annotationPayload, cropLayout, MAX_MARKS };
})();
