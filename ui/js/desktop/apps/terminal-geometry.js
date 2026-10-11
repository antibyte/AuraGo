(function () {
    'use strict';

    // Pinned grids (Contract C). `cell` is the font's cell in em, width and height (Px437 8x16: half an em wide
    // and one em high; C64 Pro Mono: one em square), so both 80x25 and 40x25 need 40em x 25em. `load` is the
    // font face the first fit waits for; `pixels` (C64 Pro Mono: 8 per em) scales the grid by whole device pixels.
    const MODES = {
        fit: null,
        bbs: { cols: 80, rows: 25, font: '"Aura VGA", "Aura Terminal", ui-monospace, monospace', load: '16px "Aura VGA"', cell: [0.5, 1] },
        c64: { cols: 40, rows: 25, font: '"Aura C64", "Aura Terminal", ui-monospace, monospace', load: '16px "Aura C64"', cell: [1, 1], pixels: 8 }
    };
    const MIN_FONT_SIZE = 6;
    const FIT_ATTEMPTS = 12;
    const FIT_SCROLLBACK = 1000;
    const SIDES = ['paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft'];

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

    // A minimum grid for 'fit' from { minCols, minRows } (a missing one is 1), or null.
    function gridMinimum(options) {
        if (!options) return null;
        const cols = Math.floor(Number(options.minCols)) || 0;
        const rows = Math.floor(Number(options.minRows)) || 0;
        return cols > 0 || rows > 0 ? { cols: Math.max(1, cols), rows: Math.max(1, rows) } : null;
    }

    // One geometry per Terminal window: 'fit' follows the window through the fit addon, optionally keeping a
    // minimum grid by shrinking the style's font; 'bbs' and 'c64' pin the grid, load the font, size it to the glass
    // (c64 by whole device pixels, otherwise in 0.5 px steps) and centre it with inline padding (letterbox).
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
        let fitted = { fontSize: 16, letterSpacing: 0 };
        let disposed = false;
        let dprQuery = null;
        let minimum = null;

        // Font, line height 1, scrollback and glyph atlas of the pinned grid; the fitted size and letter spacing only
        // outside a running fit, which owns them. Pixel fonts get xterm's alpha atlas: the opaque one antialiases
        // for LCD, and each glyph's colour fringe overwrites the edge column of the cell before it, a dark seam in
        // solid blocks. xterm rebuilds its atlas on any option change but redraws the rows only on font changes,
        // which always follow here (the family, and the size change once the font has loaded).
        function pin(withSize) {
            const spec = MODES[current];
            term.options.allowTransparency = !!spec.pixels;
            term.options.fontFamily = spec.font;
            term.options.lineHeight = 1;
            term.options.scrollback = 0;
            if (!withSize) return;
            term.options.fontSize = fitted.fontSize;
            term.options.letterSpacing = fitted.letterSpacing;
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

        // The style's own padding and the glass size, read with the letterbox padding removed and restored in the
        // same task: a rendered change would fire the screen's ResizeObserver and refit endlessly.
        function space(element) {
            const kept = SIDES.map(function (side) { return element.style[side]; });
            element.style.padding = '';
            const style = window.getComputedStyle(element);
            const pad = {
                left: parseFloat(style.paddingLeft) || 0,
                right: parseFloat(style.paddingRight) || 0,
                top: parseFloat(style.paddingTop) || 0,
                bottom: parseFloat(style.paddingBottom) || 0
            };
            const room = { pad: pad, width: screen.clientWidth - pad.left - pad.right, height: screen.clientHeight - pad.top - pad.bottom };
            SIDES.forEach(function (side, index) { element.style[side] = kept[index]; });
            return room;
        }

        // A grid with `pixels` takes the largest k whose grid of (pixels * k) device px cells fits: font size
        // pixels * k / dpr, so every font pixel is k device px. The WebGL renderer floors the measured advance in
        // device px, often one px short: one px of letter spacing restores it (the DOM renderer, after a lost WebGL
        // context, does not floor, so the spacing goes back to 0). The rounded font box is ceiled and never
        // corrected: a line height above 1 moves every glyph a device px down, a seam per row; such a k is skipped.
        // Without an exact k (a tiny glass; some DPRs) and for other grids: the largest size in 0.5 px steps that
        // fits. Sizes come from offsetWidth/Height (CSS transforms of window animations do not count). Centred via
        // padding, written only when it changes.
        async function fitFont() {
            const spec = MODES[current];
            if (!spec || !term.element) return;
            const mine = revision;
            const element = term.element;
            const screenEl = element.querySelector('.xterm-screen');
            if (!screenEl) return;
            pin(false);
            const room = space(element);
            if (room.width <= 0 || room.height <= 0) return;
            const ems = Math.min(room.width / (spec.cols * spec.cell[0]), room.height / (spec.rows * spec.cell[1]));
            const dpr = window.devicePixelRatio || 1;
            const fits = function (box) { return box.width <= room.width + 0.5 && box.height <= room.height + 0.5; };
            const cellOf = function (box) { return [Math.round(box.width * dpr / spec.cols), Math.round(box.height * dpr / spec.rows)]; };
            // Unchanged options (a refit at the same k) change nothing in xterm: no atlas rebuild, no frame of other cells.
            async function measure(size, letterSpacing) {
                term.options.fontSize = size;
                term.options.letterSpacing = letterSpacing;
                await nextFrame();
                return { width: screenEl.offsetWidth, height: screenEl.offsetHeight };
            }
            let rect = null;
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
            let size = Math.max(MIN_FONT_SIZE, Math.floor(ems * 2) / 2);
            for (let attempt = 0; attempt < FIT_ATTEMPTS && !rect; attempt += 1) {
                const box = await measure(size, 0);
                if (disposed || mine !== revision) return;
                if (fits(box) || size <= MIN_FONT_SIZE || attempt === FIT_ATTEMPTS - 1) rect = box;
                size -= 0.5;
            }
            const now = { fontSize: term.options.fontSize, letterSpacing: term.options.letterSpacing };
            const resized = Object.keys(now).some(function (key) { return now[key] !== fitted[key]; });
            fitted = now;
            const extraX = Math.max(0, (room.width - rect.width) / 2);
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

        // The style's font size: 'fit' uses it, and a minimum grid shrinks from it.
        function styleSize() {
            const profile = getProfile();
            return profile && profile.fontSize > 0 ? profile.fontSize : term.options.fontSize;
        }

        // 'fit' with a minimum grid (classic Retro-Net services: 80x24) keeps the style's font size when the fit
        // addon gets at least minimum.cols x minimum.rows from it, otherwise takes the largest size below it in
        // 0.5 px steps that does, MIN_FONT_SIZE at the least: a glass too small even for that keeps MIN_FONT_SIZE
        // and whatever grid fits. The fit addon then fills the glass, so the grid may exceed the minimum. Each probe
        // sets the size and asks the fit addon; xterm measures the cell synchronously on the option change, so the
        // grid is final when set() or refit() returns (the size a session opens with). A hidden glass keeps the
        // style's size.
        function shrinkToMinimum() {
            const base = styleSize();
            const sizeAt = function (step) { return Math.max(MIN_FONT_SIZE, base - step / 2); };
            let dims = null;
            const fits = function (step) {
                term.options.fontSize = sizeAt(step);
                dims = fit.proposeDimensions();
                return !dims || (dims.cols >= minimum.cols && dims.rows >= minimum.rows);
            };
            let step = 0;
            if (screen.clientWidth > 0 && screen.clientHeight > 0 && !fits(0)) {
                // Cells scale with the font: a first guess from the grid at the style's size, then 0.5 px steps up
                // while the next size still fits, or down until one fits.
                const guess = base * Math.min((dims.cols + 1) / minimum.cols, (dims.rows + 1) / minimum.rows);
                step = Math.max(1, Math.ceil((base - guess) * 2));
                if (fits(step)) {
                    while (step > 1 && fits(step - 1)) step -= 1;
                } else {
                    do { step += 1; } while (!fits(step) && sizeAt(step) > MIN_FONT_SIZE);
                }
            }
            term.options.fontSize = sizeAt(step);
        }

        // The fit addon outside pinned modes, with the minimum grid's font size or (`restore`, the minimum was
        // dropped) the style's again; reported when the font size changed.
        function fitWindow(restore) {
            if (!fit) return;
            const before = term.options.fontSize;
            if (minimum) shrinkToMinimum();
            else if (restore) term.options.fontSize = styleSize();
            fit.fit();
            if (term.options.fontSize !== before) onFitted();
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

        function enter(mode) {
            const spec = MODES[mode];
            if (current !== 'fit') clearLetterbox();
            // Another grid's letter spacing never carries over (the VGA grid has none).
            if (mode !== current) fitted.letterSpacing = 0;
            current = mode;
            revision += 1;
            const mine = revision;
            cancelFrame();
            root.setAttribute('data-terminal-geometry', mode);
            pin(true);
            term.resize(spec.cols, spec.rows);
            const ready = document.fonts && typeof document.fonts.load === 'function'
                ? document.fonts.load(spec.load).catch(function () { return []; })
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
            if (fit && minimum) shrinkToMinimum();
            if (fit) fit.fit();
            onFitted();
        }

        // 'fit' | 'bbs' | 'c64' (anything else is 'fit'); entering a pinned mode again restarts it (font and fit).
        // `options` count for 'fit' only: { minCols, minRows } keeps at least that grid (shrinkToMinimum); 'fit'
        // without them drops a minimum and restores the style's font size. Pinned modes drop it too.
        function set(mode, options) {
            if (disposed || !term) return;
            const next = pinned(mode);
            const had = minimum;
            minimum = next === 'fit' ? gridMinimum(options) : null;
            if (next !== 'fit') enter(next);
            else if (current !== 'fit') leave();
            else if (minimum || had) fitWindow(true);
        }

        // ResizeObserver and style changes: the fit addon outside pinned modes (re-evaluating a minimum grid from
        // the style's size); otherwise the pinned font at once (a style change replaced it) and a fit on the next
        // frame.
        function refit() {
            if (disposed || !term) return;
            if (current === 'fit') {
                fitWindow(false);
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

        // A minimum's font size is never saved: it goes with the terminal, a new window starts at the style's size.
        function dispose() {
            disposed = true;
            revision += 1;
            again = false;
            minimum = null;
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
