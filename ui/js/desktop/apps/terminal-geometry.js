(function () {
    'use strict';

    // Pinned grids (Contract C). `cell` is the font's cell in em, width and height (Px437 8x16: half an em wide
    // and one em high; C64 Pro Mono: one em square), so both 80x25 and 40x25 need 40em x 25em. `load` is the
    // font face the first fit waits for.
    const MODES = {
        fit: null,
        bbs: { cols: 80, rows: 25, font: '"Aura VGA", "Aura Terminal", ui-monospace, monospace', load: '16px "Aura VGA"', cell: [0.5, 1] },
        c64: { cols: 40, rows: 25, font: '"Aura C64", "Aura Terminal", ui-monospace, monospace', load: '16px "Aura C64"', cell: [1, 1] }
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

    // One geometry per Terminal window: 'fit' follows the window through the fit addon; 'bbs' and 'c64' pin the
    // grid, load the font, size it to the glass in 0.5 px steps and centre it with inline padding (letterbox).
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
        let fontSize = 16;
        let disposed = false;

        // Font, line height and scrollback of the pinned grid; the size only outside a running fit, which owns it.
        function pin(withSize) {
            const spec = MODES[current];
            term.options.fontFamily = spec.font;
            term.options.lineHeight = 1;
            term.options.scrollback = 0;
            if (withSize) term.options.fontSize = fontSize;
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

        // Largest font size in 0.5 px steps at which the grid fits; centred via padding, written only when it changes.
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
            let size = Math.max(MIN_FONT_SIZE, Math.floor(ems * 2) / 2);
            let rect = null;
            for (let attempt = 0; attempt < FIT_ATTEMPTS; attempt += 1) {
                term.options.fontSize = size;
                await nextFrame();
                if (disposed || mine !== revision) return;
                rect = screenEl.getBoundingClientRect();
                if ((rect.width <= room.width + 0.5 && rect.height <= room.height + 0.5) || size <= MIN_FONT_SIZE) break;
                size -= 0.5;
            }
            const resized = fontSize !== term.options.fontSize;
            fontSize = term.options.fontSize;
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
                if (!disposed && mine === revision) runFit();
            });
        }

        function leave() {
            current = 'fit';
            revision += 1;
            cancelFrame();
            root.removeAttribute('data-terminal-geometry');
            clearLetterbox();
            term.options.scrollback = FIT_SCROLLBACK;
            window.TerminalStyles.applyXterm(term, getProfile());
            if (fit) fit.fit();
            onFitted();
        }

        // 'fit' | 'bbs' | 'c64' (anything else is 'fit'); entering a pinned mode again restarts it (font and fit).
        function set(mode) {
            if (disposed || !term) return;
            const next = pinned(mode);
            if (next !== 'fit') enter(next);
            else if (current !== 'fit') leave();
        }

        // ResizeObserver and style changes: the fit addon outside pinned modes; otherwise the pinned font at once
        // (a style change replaced it) and a fit on the next frame.
        function refit() {
            if (disposed || !term) return;
            if (current === 'fit') {
                if (fit) fit.fit();
                return;
            }
            pin(!fitting);
            schedule();
        }

        function dispose() {
            disposed = true;
            revision += 1;
            again = false;
            cancelFrame();
        }

        return {
            set: set,
            mode: function () { return current; },
            refit: refit,
            dispose: dispose
        };
    }

    window.TerminalGeometry = { MODES: MODES, create: create };
})();
