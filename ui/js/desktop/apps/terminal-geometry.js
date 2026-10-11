(function () {
    'use strict';

    const MIN_FONT_SIZE = 6;
    const FIT_ATTEMPTS = 12;
    const FIT_SCROLLBACK = 1000;
    // 'vt' spaces its rows to fill the glass height, up to this line height.
    const MAX_LINE_HEIGHT = 2;
    // The glass border of the 'vt' and 'bbs' grids in retro styles, per axis: a share of the glass width left and
    // right and of its height top and bottom, which keeps the grid's corners inside the CRT's rounded corners (an
    // ellipse of 5% of the width by 7% of the height) and its edge fade (1.8%); GLASS_MIN px at the least, never
    // more than the style's own padding (the shell's). The C64 grid keeps the style's padding.
    const GLASS_X = 0.02;
    const GLASS_Y = 0.03;
    const GLASS_MIN = 12;
    const SIDES = ['paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft'];
    // Pinned grids (Contract C). `cell` is the font's cell in em, width and height (Px437 8x16: half an em wide
    // and one em high; C64 Pro Mono: one em square), so both 80x25 and 40x25 need 40em x 25em. `load` is the
    // font face the first fit waits for; `pixels` (C64 Pro Mono: 8 per em) scales the grid by whole device pixels.
    // 'vt' (the 80x24 screen of classic services) has no font of its own: it takes the style's font, keeps the
    // scrollback, measures its cell and spaces its rows.
    const MODES = {
        fit: null,
        vt: { cols: 80, rows: 24, scrollback: FIT_SCROLLBACK },
        bbs: { cols: 80, rows: 25, font: '"Aura VGA", "Aura Terminal", ui-monospace, monospace', load: '16px "Aura VGA"', cell: [0.5, 1] },
        c64: { cols: 40, rows: 25, font: '"Aura C64", "Aura Terminal", ui-monospace, monospace', load: '16px "Aura C64"', cell: [1, 1], pixels: 8 }
    };

    function pinned(mode) {
        return Object.prototype.hasOwnProperty.call(MODES, mode) && MODES[mode] ? mode : 'fit';
    }

    function nextFrame() {
        return new Promise(function (resolve) {
            window.requestAnimationFrame(function () { resolve(); });
        });
    }

    function px(value) {
        return Math.round(value * 100) / 100 + 'px';
    }

    // One geometry per Terminal window: 'fit' follows the window through the fit addon; 'vt', 'bbs' and 'c64' pin
    // the grid, load the font, size it to the glass (c64 by whole device pixels, otherwise in 0.5 px steps) and
    // centre it with inline padding (letterbox).
    function create(options) {
        const opts = options || {};
        const term = opts.term;
        const screen = opts.screen;
        const root = opts.root;
        const fit = opts.fit && typeof opts.fit.fit === 'function' ? opts.fit : null;
        const getProfile = typeof opts.getProfile === 'function' ? opts.getProfile : function () { return null; };
        const onFitted = typeof opts.onFitted === 'function' ? opts.onFitted : function () {};
        let current = 'fit';
        let revision = 0;
        let frame = 0;
        let fitting = false;
        let again = false;
        let fitted = { fontSize: 16, letterSpacing: 0, lineHeight: 1 };
        let disposed = false;
        let dprQuery = null;
        // The last 'vt' fit: glass and font (`key`), its size, line height and grid.
        let ceiling = null;

        // A grid's font: its own, or for 'vt' the style's family, loaded at the style's size like a style change does.
        function face(spec) {
            if (spec.font) return { family: spec.font, load: spec.load };
            const profile = getProfile() || {};
            const family = profile.fontFamily || term.options.fontFamily;
            const first = String(family || '').split(',')[0].trim() || 'monospace';
            return { family: family, load: (profile.fontSize || 16) + 'px ' + first };
        }

        // Font, line height, scrollback and glyph atlas of the pinned grid; the fitted size, letter spacing and 'vt'
        // line height only outside a running fit, which owns them. Grids with their own font draw rows edge to edge
        // at line height 1 (VGA blocks come from the font; a C64 nudge moves glyphs). Pixel fonts get xterm's alpha
        // atlas: the opaque one antialiases for LCD, and each glyph's colour fringe overwrites the edge column of the
        // cell before it, a dark seam in solid blocks. xterm rebuilds its atlas on any option change but redraws the
        // rows only on font changes, which always follow here (the family, and the size change once the font has
        // loaded).
        function pin(withSize) {
            const spec = MODES[current];
            term.options.allowTransparency = !!spec.pixels;
            term.options.fontFamily = face(spec).family;
            term.options.scrollback = spec.scrollback || 0;
            if (spec.font) term.options.lineHeight = 1;
            if (!withSize) return;
            term.options.fontSize = fitted.fontSize;
            term.options.letterSpacing = fitted.letterSpacing;
            if (!spec.font) term.options.lineHeight = fitted.lineHeight;
        }

        function cancelFrame() {
            if (frame) window.cancelAnimationFrame(frame);
            frame = 0;
        }

        function clearLetterbox() {
            if (!term.element) return;
            term.element.style.padding = '';
            const screenEl = term.element.querySelector('.xterm-screen');
            if (screenEl) screenEl.style.background = '';
        }

        // The glass border and the room inside it, read with the letterbox padding removed and restored in the same
        // task: a rendered change would fire the screen's ResizeObserver and refit endlessly. The style's padding
        // resolves against the glass width on every side (retro styles: clamp(16px, 3%, 30px)); a `glass` grid
        // takes the per-axis border above instead (none in the frameless modern style).
        function space(element, glass) {
            const kept = SIDES.map(function (side) { return element.style[side]; });
            element.style.padding = '';
            const style = window.getComputedStyle(element);
            const width = screen.clientWidth;
            const height = screen.clientHeight;
            const side = function (name, share, size) {
                const own = parseFloat(style[name]) || 0;
                return glass && own > 0 ? Math.min(own, Math.max(GLASS_MIN, share * size)) : own;
            };
            const pad = {
                left: side('paddingLeft', GLASS_X, width),
                right: side('paddingRight', GLASS_X, width),
                top: side('paddingTop', GLASS_Y, height),
                bottom: side('paddingBottom', GLASS_Y, height)
            };
            const room = { pad: pad, width: width - pad.left - pad.right, height: height - pad.top - pad.bottom };
            SIDES.forEach(function (side, index) { element.style[side] = kept[index]; });
            return room;
        }

        // A grid with `pixels` takes the largest k whose grid of (pixels * k) device px cells fits: font size
        // pixels * k / dpr, so every font pixel is k device px. The WebGL renderer floors the measured advance in
        // device px, often one px short: one px of letter spacing restores it (the DOM renderer, after a lost WebGL
        // context, does not floor, so the spacing goes back to 0). The rounded font box is ceiled and never
        // corrected: a line height above 1 moves every glyph a device px down, a seam per row; such a k is skipped.
        // Without an exact k (a tiny glass; some DPRs) and for other grids: the largest size in 0.5 px steps that
        // fits (see below). Sizes come from offsetWidth/Height (CSS transforms of window animations do not count).
        // A grid with scrollback leaves xterm's scrollbar (as the fit addon does) a strip beside it. Centred via
        // padding, written only when it changes.
        async function fitFont() {
            const spec = MODES[current];
            if (!spec || !term.element) return;
            const mine = revision;
            const element = term.element;
            const screenEl = element.querySelector('.xterm-screen');
            if (!screenEl) return;
            pin(false);
            const room = space(element, !spec.pixels);
            const bar = spec.scrollback ? (term.options.overviewRuler && term.options.overviewRuler.width) || 14 : 0;
            const width = room.width - bar;
            if (width <= 0 || room.height <= 0) return;
            const dpr = window.devicePixelRatio || 1;
            const fits = function (box) { return box.width <= width + 0.5 && box.height <= room.height + 0.5; };
            const cellOf = function (box) { return [Math.round(box.width * dpr / spec.cols), Math.round(box.height * dpr / spec.rows)]; };
            // Unchanged options (a refit at the same k or size) change nothing in xterm: no atlas rebuild, no frame of other cells.
            async function measure(size, letterSpacing, lineHeight) {
                term.options.fontSize = size;
                term.options.letterSpacing = letterSpacing;
                if (lineHeight) term.options.lineHeight = lineHeight;
                await nextFrame();
                return { width: screenEl.offsetWidth, height: screenEl.offsetHeight };
            }
            const scales = !spec.cell;
            const key = [term.options.fontFamily, dpr, width, room.height].join('|');
            let rect = null;
            // An unchanged glass whose cells measure as at the last 'vt' fit keeps that fit: one probe, no writes.
            if (scales && ceiling && ceiling.key === key && ceiling.size === term.options.fontSize && ceiling.lineHeight === term.options.lineHeight) {
                const box = await measure(ceiling.size, 0, ceiling.lineHeight);
                if (disposed || mine !== revision) return;
                if (box.width === ceiling.width && box.height === ceiling.height) rect = box;
            }
            const fresh = !rect;
            const ems = spec.cell ? Math.min(width / (spec.cols * spec.cell[0]), room.height / (spec.rows * spec.cell[1])) : 0;
            for (let k = spec.pixels ? Math.floor(ems * dpr / spec.pixels) : 0; k >= 1 && !rect; k -= 1) {
                const cell = spec.pixels * k;
                const spacing = term.options.letterSpacing || 0;
                let box = await measure(cell / dpr, spacing);
                if (disposed || mine !== revision) return;
                let got = cellOf(box);
                const nudged = spacing + cell - got[0];
                if (got[1] === cell && got[0] !== cell && (nudged === 0 || nudged === 1)) {
                    box = await measure(cell / dpr, nudged);
                    if (disposed || mine !== revision) return;
                    got = cellOf(box);
                }
                if (got[0] === cell && got[1] === cell && fits(box)) rect = box;
            }
            // 0.5 px steps down until the grid fits (MIN_FONT_SIZE at the least, clipped there), from the size the
            // font's em cell allows. Without an em cell ('vt': the style's font, at line height 1 here) the search
            // starts at the current size and moves to the size the measured cells scale to, between the largest
            // size that fit and the smallest that did not; a size that fits and expects no larger one still tries the
            // next step (xterm's whole device px cells keep their size over some steps), so it ends with the two one
            // step apart: the font of the 80 columns in the glass width, or of the rows where they do not fit the
            // height even at line height 1 (a wide, short glass; the sides take the rest).
            const step = function (value) { return Math.max(MIN_FONT_SIZE, Math.floor(value * 2) / 2); };
            let size = step(scales ? term.options.fontSize : ems);
            let good = null;
            let bad = Infinity;
            for (let attempt = 0; attempt < FIT_ATTEMPTS && !rect; attempt += 1) {
                const box = await measure(size, 0, scales ? 1 : 0);
                if (disposed || mine !== revision) return;
                const last = attempt === FIT_ATTEMPTS - 1;
                const expected = scales ? step(size * Math.min(width / box.width, room.height / box.height)) : size;
                if (fits(box)) {
                    good = { size: size, box: box };
                    const next = !scales ? size : Math.min(expected > size ? expected : size + 0.5, bad - 0.5);
                    if (next > size && !last) size = next;
                    else rect = box;
                    continue;
                }
                bad = size;
                if (good && size - 0.5 <= good.size) rect = await measure(good.size, 0);
                else if (size <= MIN_FONT_SIZE || last) rect = good ? await measure(good.size, 0) : box;
                else size = Math.max(good ? good.size + 0.5 : MIN_FONT_SIZE, Math.min(expected, size - 0.5));
                if (disposed || mine !== revision) return;
            }
            // 'vt' then spaces its rows like a CRT's raster to fill the glass height: the largest device px cell
            // height whose rows fit, as a line height over the font's box (the cell at line height 1; xterm floors
            // the product), MAX_LINE_HEIGHT at the most (a tall glass keeps a letterbox). A height xterm does not
            // give (another renderer) is retried a px lower, twice, else line height 1 stays.
            if (scales && fresh && good) {
                const base = Math.round(rect.height * dpr / spec.rows);
                let spaced = false;
                for (let cell = Math.floor((room.height + 0.5) * dpr / spec.rows), tries = 0; cell > base && tries < 3 && !spaced; cell -= 1, tries += 1) {
                    const box = await measure(good.size, 0, Math.min(MAX_LINE_HEIGHT, (cell + 0.5) / base));
                    if (disposed || mine !== revision) return;
                    spaced = fits(box);
                    if (spaced) rect = box;
                }
                if (!spaced && term.options.lineHeight !== 1) rect = await measure(good.size, 0, 1);
                if (disposed || mine !== revision) return;
            }
            if (scales && fresh) ceiling = good ? { key: key, size: term.options.fontSize, lineHeight: term.options.lineHeight, width: rect.width, height: rect.height } : null;
            const now = { fontSize: term.options.fontSize, letterSpacing: term.options.letterSpacing, lineHeight: term.options.lineHeight };
            const resized = Object.keys(now).some(function (key) { return now[key] !== fitted[key]; });
            fitted = now;
            const extraX = Math.max(0, (width - rect.width) / 2);
            const extraY = Math.max(0, (room.height - rect.height) / 2);
            const next = {
                paddingLeft: px(room.pad.left + extraX),
                paddingRight: px(room.pad.right + extraX),
                paddingTop: px(room.pad.top + extraY),
                paddingBottom: px(room.pad.bottom + extraY)
            };
            let moved = false;
            Object.keys(next).forEach(function (side) {
                if (element.style[side] === next[side]) return;
                element.style[side] = next[side];
                moved = true;
            });
            screenEl.style.background = (term.options.theme && term.options.theme.background) || '';
            if (resized || moved) onFitted();
        }

        // One fit at a time: a request during a fit reruns it once afterwards, so an older, slower fit never leaves
        // its padding behind.
        function runFit() {
            if (disposed || current === 'fit') return;
            if (fitting) {
                again = true;
                return;
            }
            fitting = true;
            again = false;
            fitFont().catch(function () {}).then(function () {
                fitting = false;
                if (!again) return;
                again = false;
                schedule();
            });
        }

        function schedule() {
            if (disposed || current === 'fit') return;
            if (fitting) {
                again = true;
                return;
            }
            if (frame) return;
            frame = window.requestAnimationFrame(function () {
                frame = 0;
                runFit();
            });
        }

        // The grid is pinned at once (a session opens with it); the fit follows once the font has loaded.
        function enter(mode) {
            const spec = MODES[mode];
            if (current !== 'fit') clearLetterbox();
            // Another grid's letter spacing never carries over (only the C64 grid has any).
            if (mode !== current) fitted.letterSpacing = 0;
            current = mode;
            revision += 1;
            const mine = revision;
            cancelFrame();
            root.setAttribute('data-terminal-geometry', mode);
            pin(true);
            term.resize(spec.cols, spec.rows);
            const ready = document.fonts && typeof document.fonts.load === 'function'
                ? document.fonts.load(face(spec).load).catch(function () { return []; })
                : Promise.resolve([]);
            ready.then(function () {
                if (disposed || mine !== revision) return;
                // xterm measured the cell and built its atlas when the family was set, maybe before the font had
                // loaded, and does neither again for an unchanged size: a size change makes it do both now.
                term.options.fontSize = fitted.fontSize + 1;
                pin(!fitting);
                runFit();
            });
        }

        function leave() {
            current = 'fit';
            revision += 1;
            cancelFrame();
            root.removeAttribute('data-terminal-geometry');
            clearLetterbox();
            fitted.letterSpacing = 0;
            term.options.scrollback = FIT_SCROLLBACK;
            term.options.letterSpacing = 0;
            term.options.allowTransparency = false;
            window.TerminalStyles.applyXterm(term, getProfile());
            if (fit) fit.fit();
            onFitted();
        }

        // 'fit' | 'vt' | 'bbs' | 'c64' (anything else is 'fit'); entering a pinned mode again restarts it (font and
        // fit); 'fit' restores the style's font and size.
        function set(mode) {
            if (disposed || !term) return;
            const next = pinned(mode);
            if (next !== 'fit') enter(next);
            else if (current !== 'fit') leave();
        }

        // ResizeObserver and style changes: the fit addon outside pinned modes; otherwise the pinned font at once
        // (a style change replaced it; 'vt' takes the new style's font) and a fit on the next frame.
        function refit() {
            if (disposed || !term) return;
            if (current === 'fit') {
                if (fit) fit.fit();
                return;
            }
            pin(!fitting);
            schedule();
        }

        // Whole pixels depend on the device pixel ratio: a change without a resize (another monitor) refits too.
        function watchDpr() {
            if (dprQuery) dprQuery.removeEventListener('change', onDpr);
            dprQuery = !disposed && typeof window.matchMedia === 'function'
                ? window.matchMedia('(resolution: ' + (window.devicePixelRatio || 1) + 'dppx)')
                : null;
            if (dprQuery) dprQuery.addEventListener('change', onDpr);
        }

        function onDpr() {
            watchDpr();
            schedule();
        }

        // The fitted size is never saved: it goes with the terminal, and leaving a pinned grid restores the style's.
        function dispose() {
            disposed = true;
            revision += 1;
            again = false;
            cancelFrame();
            watchDpr();
        }

        watchDpr();
        return {
            set: set,
            mode: function () { return current; },
            refit: refit,
            dispose: dispose
        };
    }

    window.TerminalGeometry = { MODES: MODES, create: create };
})();
