(function () {
    'use strict';

    const LOCAL_SHELL_ID = 'local-shell';
    const LAST_KEY = 'aurago.desktop.terminal.retronet.last';
    const CATEGORIES = ['classics', 'bbs', 'muds', 'games', 'own'];
    const MARKERS = { online: '[*]', offline: '[ ]', unknown: '[?]' };
    const MARKER_COLORS = { online: '\x1b[32m', offline: '\x1b[31m', unknown: '\x1b[33m' };
    const HEADER_ROWS = 2;
    const DOUBLE_TAP_MS = 500;
    const DIGIT_WINDOW_MS = 1000;
    const RESET = '\x1b[0m';
    const DIM = '\x1b[2m';
    const ELLIPSIS = '\u2026';
    const RULE = '\u2500';
    const HINT_SEPARATOR = ' \u00b7 ';
    const KEYS = {
        up: ['\x1b[A', '\x1bOA'],
        down: ['\x1b[B', '\x1bOB'],
        pageUp: ['\x1b[5~'],
        pageDown: ['\x1b[6~'],
        home: ['\x1b[H', '\x1bOH', '\x1b[1~', '\x1b[7~'],
        end: ['\x1b[F', '\x1bOF', '\x1b[4~', '\x1b[8~'],
        remove: ['\x1b[3~'],
        enter: ['\r', '\n', '\r\n']
    };
    // Zero-width ranges of xterm's default Unicode 6 provider (vendored xterm.js, after Markus Kuhn's wcwidth).
    const ZERO_WIDTH = [
        0x300, 0x36f, 0x483, 0x486, 0x488, 0x489, 0x591, 0x5bd, 0x5bf, 0x5bf, 0x5c1, 0x5c2, 0x5c4, 0x5c5,
        0x5c7, 0x5c7, 0x600, 0x603, 0x610, 0x615, 0x64b, 0x65e, 0x670, 0x670, 0x6d6, 0x6e4, 0x6e7, 0x6e8,
        0x6ea, 0x6ed, 0x70f, 0x70f, 0x711, 0x711, 0x730, 0x74a, 0x7a6, 0x7b0, 0x7eb, 0x7f3, 0x901, 0x902,
        0x93c, 0x93c, 0x941, 0x948, 0x94d, 0x94d, 0x951, 0x954, 0x962, 0x963, 0x981, 0x981, 0x9bc, 0x9bc,
        0x9c1, 0x9c4, 0x9cd, 0x9cd, 0x9e2, 0x9e3, 0xa01, 0xa02, 0xa3c, 0xa3c, 0xa41, 0xa42, 0xa47, 0xa48,
        0xa4b, 0xa4d, 0xa70, 0xa71, 0xa81, 0xa82, 0xabc, 0xabc, 0xac1, 0xac5, 0xac7, 0xac8, 0xacd, 0xacd,
        0xae2, 0xae3, 0xb01, 0xb01, 0xb3c, 0xb3c, 0xb3f, 0xb3f, 0xb41, 0xb43, 0xb4d, 0xb4d, 0xb56, 0xb56,
        0xb82, 0xb82, 0xbc0, 0xbc0, 0xbcd, 0xbcd, 0xc3e, 0xc40, 0xc46, 0xc48, 0xc4a, 0xc4d, 0xc55, 0xc56,
        0xcbc, 0xcbc, 0xcbf, 0xcbf, 0xcc6, 0xcc6, 0xccc, 0xccd, 0xce2, 0xce3, 0xd41, 0xd43, 0xd4d, 0xd4d,
        0xdca, 0xdca, 0xdd2, 0xdd4, 0xdd6, 0xdd6, 0xe31, 0xe31, 0xe34, 0xe3a, 0xe47, 0xe4e, 0xeb1, 0xeb1,
        0xeb4, 0xeb9, 0xebb, 0xebc, 0xec8, 0xecd, 0xf18, 0xf19, 0xf35, 0xf35, 0xf37, 0xf37, 0xf39, 0xf39,
        0xf71, 0xf7e, 0xf80, 0xf84, 0xf86, 0xf87, 0xf90, 0xf97, 0xf99, 0xfbc, 0xfc6, 0xfc6, 0x102d, 0x1030,
        0x1032, 0x1032, 0x1036, 0x1037, 0x1039, 0x1039, 0x1058, 0x1059, 0x1160, 0x11ff, 0x135f, 0x135f,
        0x1712, 0x1714, 0x1732, 0x1734, 0x1752, 0x1753, 0x1772, 0x1773, 0x17b4, 0x17b5, 0x17b7, 0x17bd,
        0x17c6, 0x17c6, 0x17c9, 0x17d3, 0x17dd, 0x17dd, 0x180b, 0x180d, 0x18a9, 0x18a9, 0x1920, 0x1922,
        0x1927, 0x1928, 0x1932, 0x1932, 0x1939, 0x193b, 0x1a17, 0x1a18, 0x1b00, 0x1b03, 0x1b34, 0x1b34,
        0x1b36, 0x1b3a, 0x1b3c, 0x1b3c, 0x1b42, 0x1b42, 0x1b6b, 0x1b73, 0x1dc0, 0x1dca, 0x1dfe, 0x1dff,
        0x200b, 0x200f, 0x202a, 0x202e, 0x2060, 0x2063, 0x206a, 0x206f, 0x20d0, 0x20ef, 0x302a, 0x302f,
        0x3099, 0x309a, 0xa806, 0xa806, 0xa80b, 0xa80b, 0xa825, 0xa826, 0xfb1e, 0xfb1e, 0xfe00, 0xfe0f,
        0xfe20, 0xfe23, 0xfeff, 0xfeff, 0xfff9, 0xfffb, 0x10a01, 0x10a03, 0x10a05, 0x10a06, 0x10a0c, 0x10a0f,
        0x10a38, 0x10a3a, 0x10a3f, 0x10a3f, 0x1d167, 0x1d169, 0x1d173, 0x1d182, 0x1d185, 0x1d18b, 0x1d1aa, 0x1d1ad,
        0x1d242, 0x1d244, 0xe0001, 0xe0001, 0xe0020, 0xe007f, 0xe0100, 0xe01ef
    ];
    // Double-width ranges of the same provider; zero-width ranges take precedence.
    const WIDE = [
        0x1100, 0x115f, 0x2329, 0x232a, 0x2e80, 0x303e, 0x3040, 0xa4cf, 0xac00, 0xd7a3, 0xf900, 0xfaff,
        0xfe10, 0xfe19, 0xfe30, 0xfe6f, 0xff00, 0xff60, 0xffe0, 0xffe6, 0x20000, 0x2fffd, 0x30000, 0x3fffd
    ];
    // Unicode 6 draws emoji in one cell; newer providers (term.unicode.activeVersion) use two.
    const WIDE_EMOJI = [0x1f300, 0x1f64f, 0x1f680, 0x1f6ff, 0x1f900, 0x1f9ff, 0x1fa70, 0x1faff];

    function inRanges(code, ranges) {
        if (code < ranges[0] || code > ranges[ranges.length - 1]) return false;
        let low = 0;
        let high = ranges.length / 2 - 1;
        while (low <= high) {
            const mid = (low + high) >> 1;
            if (code > ranges[mid * 2 + 1]) low = mid + 1;
            else if (code < ranges[mid * 2]) high = mid - 1;
            else return true;
        }
        return false;
    }

    // Terminal cells xterm advances for one code point (0, 1 or 2); `wide` = two-cell emoji.
    function cellWidth(code, wide) {
        if (code < 0x20 || (code >= 0x7f && code < 0xa0)) return 0;
        if (code < 0x300) return 1;
        if (inRanges(code, ZERO_WIDTH)) return 0;
        return inRanges(code, WIDE) || (wide && inRanges(code, WIDE_EMOJI)) ? 2 : 1;
    }

    // Names and descriptions never reach xterm with control characters (no escape injection).
    function printable(value) {
        return String(value == null ? '' : value).replace(/[\u0000-\u001f\u007f-\u009f]/g, '');
    }

    function isRegional(code) {
        return code >= 0x1f1e6 && code <= 0x1f1ff;
    }

    // Units that are never cut: a base with its combining marks, variation selectors and
    // skin tones, code points joined by ZWJ, and regional-indicator pairs (flags).
    function clusters(text, wide) {
        const out = [];
        let joined = false;
        for (const ch of text) {
            const code = ch.codePointAt(0);
            const width = cellWidth(code, wide);
            const last = out[out.length - 1];
            const flag = !!last && last.regional === 1 && isRegional(code);
            if (last && (joined || flag || width === 0 || (code >= 0x1f3fb && code <= 0x1f3ff))) {
                last.text += ch;
                last.width += width;
                if (flag) last.regional = 2;
            } else {
                out.push({ text: ch, width: width, regional: isRegional(code) ? 1 : 0 });
            }
            joined = code === 0x200d;
        }
        return out;
    }

    function displayWidth(value, wide) {
        let width = 0;
        for (const ch of printable(value)) width += cellWidth(ch.codePointAt(0), wide);
        return width;
    }

    // At most `width` cells; a cut ends in an ellipsis and never splits a surrogate pair or cluster.
    function fitToCells(value, width, wide) {
        const text = printable(value);
        const limit = Math.floor(Number(width) || 0);
        if (limit <= 0) return '';
        const parts = clusters(text, wide);
        if (parts.reduce(function (sum, part) { return sum + part.width; }, 0) <= limit) return text;
        let out = '';
        let used = 0;
        for (let i = 0; i < parts.length && used + parts[i].width <= limit - 1; i += 1) {
            out += parts[i].text;
            used += parts[i].width;
        }
        return out + ELLIPSIS;
    }

    function padToCells(value, width, wide) {
        const text = fitToCells(value, width, wide);
        return text + ' '.repeat(Math.max(0, width - displayWidth(text, wide)));
    }

    // Packs the middle-dot-separated key hints into as few lines as fit; an overlong hint is cut.
    function wrapHints(value, width, wide) {
        const lines = [];
        printable(value).split(HINT_SEPARATOR).forEach(function (part) {
            const last = lines.length - 1;
            if (last >= 0 && displayWidth(lines[last] + HINT_SEPARATOR + part, wide) <= width) lines[last] += HINT_SEPARATOR + part;
            else lines.push(fitToCells(part, width, wide));
        });
        return lines;
    }

    function pad2(number) {
        return number < 10 ? '0' + number : String(number);
    }

    function at(row) {
        return '\x1b[' + (row + 1) + ';1H\x1b[2K';
    }

    function readLast() {
        try {
            return window.localStorage.getItem(LAST_KEY) || '';
        } catch (e) {
            return '';
        }
    }

    function categoryOf(entry) {
        if (entry.own) return 'own';
        return CATEGORIES.indexOf(entry.category) >= 0 ? entry.category : 'own';
    }

    function relativeTime(iso) {
        const when = Date.parse(iso);
        if (!Number.isFinite(when) || typeof Intl === 'undefined' || !Intl.RelativeTimeFormat) return '';
        const seconds = Math.round((when - Date.now()) / 1000);
        let format;
        try {
            format = new Intl.RelativeTimeFormat(window.SYSTEM_LANG || document.documentElement.lang || 'en', { numeric: 'auto' });
        } catch (e) {
            format = new Intl.RelativeTimeFormat('en', { numeric: 'auto' });
        }
        const abs = Math.abs(seconds);
        if (abs < 3600) return format.format(Math.round(seconds / 60), 'minute');
        if (abs < 86400) return format.format(Math.round(seconds / 3600), 'hour');
        return format.format(Math.round(seconds / 86400), 'day');
    }

    function create(options) {
        const opts = options || {};
        const term = opts.term;
        const t = typeof opts.t === 'function' ? opts.t : function (key) { return key; };
        const api = opts.api;
        const announce = typeof opts.announce === 'function' ? opts.announce : function () {};
        const canEdit = opts.canEdit;
        const onDial = typeof opts.onDial === 'function' ? opts.onDial : function () {};
        const onEdit = typeof opts.onEdit === 'function' ? opts.onEdit : function () {};
        const onDelete = typeof opts.onDelete === 'function' ? opts.onDelete : function () {};
        const element = term && term.element;
        let data = { entries: [], status: {}, stale: false, canEdit: false };
        let list = [];
        let lines = [];
        let cursor = 0;
        let offset = 0;
        let visibleRows = 1;
        let wide = false;
        let notice = '';
        let loaded = false;
        let failed = false;
        let checking = false;
        let disposed = false;
        let dialing = false;
        let generation = 0;
        let pendingDigit = null;
        let lastTap = { index: -1, at: 0 };

        function tr(key, params) {
            let text = String(t(key, params));
            if (params) {
                Object.keys(params).forEach(function (name) {
                    text = text.split('{{' + name + '}}').join(String(params[name]));
                });
            }
            return text;
        }

        // term.unicode is a proposed xterm API: it throws unless allowProposedApi is set,
        // and then xterm keeps its default Unicode 6 widths.
        function wideEmoji() {
            try {
                const version = term.unicode && term.unicode.activeVersion;
                return typeof version === 'string' && version !== '6';
            } catch (e) {
                return false;
            }
        }

        function cells(text) {
            return displayWidth(text, wide);
        }

        function fit(text, width) {
            return fitToCells(text, width, wide);
        }

        function editable() {
            const allowed = typeof canEdit === 'function' ? !!canEdit() : canEdit !== false;
            return allowed && data.canEdit;
        }

        function rebuild() {
            const sorted = data.entries
                .filter(function (entry) { return entry && typeof entry.id === 'string'; })
                .map(function (entry, index) { return { entry: entry, index: index }; })
                .sort(function (a, b) {
                    return (CATEGORIES.indexOf(categoryOf(a.entry)) - CATEGORIES.indexOf(categoryOf(b.entry))) || (a.index - b.index);
                })
                .map(function (item) { return item.entry; });
            list = [{ entry: { id: LOCAL_SHELL_ID, name: tr('desktop.terminal_retronet_local_shell'), local: true }, number: 0 }];
            sorted.forEach(function (entry, index) { list.push({ entry: entry, number: index + 1 }); });
            lines = [{ kind: 'entry', index: 0 }];
            let current = '';
            for (let i = 1; i < list.length; i += 1) {
                const category = categoryOf(list[i].entry);
                if (category !== current) {
                    lines.push({ kind: 'heading', category: category });
                    current = category;
                }
                lines.push({ kind: 'entry', index: i });
            }
            if (cursor >= list.length) cursor = list.length - 1;
        }

        function description(entry) {
            if (entry.local) return tr('desktop.terminal_retronet_local_shell_desc');
            if (entry.own) return printable(entry.description);
            if (!entry.description_key) return '';
            const text = String(t(entry.description_key));
            return text === entry.description_key ? '' : printable(text);
        }

        function statusOf(entry) {
            if (entry.local) return null;
            const status = data.status && data.status[entry.id];
            const state = status && (status.state === 'online' || status.state === 'offline') ? status.state : 'unknown';
            return { state: state, lastOnline: status ? status.last_online_at : '' };
        }

        function lastSeen(status) {
            if (!status || status.state !== 'offline' || !status.lastOnline) return '';
            const when = relativeTime(status.lastOnline);
            return when ? printable(tr('desktop.terminal_retronet_last_seen', { when: when })) : '';
        }

        function statusLabel(status) {
            if (!status) return '';
            const base = tr('desktop.terminal_retronet_status_' + status.state);
            const seen = lastSeen(status);
            return seen ? base + ', ' + seen : base;
        }

        // Every entry row is exactly `width` cells: name and description are padded by display width.
        function entryLine(item, width, selected) {
            const entry = item.entry;
            const status = statusOf(entry);
            const nameWidth = Math.max(8, Math.min(24, Math.floor(width * 0.3)));
            const fixed = 1 + 2 + 1 + 3 + 1 + nameWidth + 1;
            let seen = lastSeen(status);
            if (seen && width - fixed - cells(seen) - 1 < 12) seen = '';
            const descWidth = Math.max(0, width - fixed - (seen ? cells(seen) + 1 : 0));
            let out = selected ? '\x1b[7m>' : ' ';
            out += pad2(item.number) + ' ';
            out += status ? MARKER_COLORS[status.state] + MARKERS[status.state] + '\x1b[39m' : '   ';
            out += ' \x1b[1m' + padToCells(entry.name, nameWidth, wide) + '\x1b[22m ';
            out += padToCells(description(entry), descWidth, wide);
            if (seen) out += ' ' + DIM + seen + '\x1b[22m';
            return out + RESET;
        }

        function headingLine(category, width) {
            return ' \x1b[1;4m' + fit(tr('desktop.terminal_retronet_cat_' + category).toLocaleUpperCase(), width - 1) + RESET;
        }

        function headerLine(cols) {
            const title = fit(tr('desktop.terminal_retronet_title'), cols - 2);
            let out = ' \x1b[1m' + title + RESET;
            const room = cols - 4 - cells(title);
            if (notice && room > 3) {
                const text = fit(tr(notice), room);
                out += ' '.repeat(Math.max(2, cols - 2 - cells(title) - cells(text))) + DIM + text + RESET;
            }
            return out;
        }

        function lineText(line, width) {
            if (!line) return '';
            if (line.kind === 'heading') return headingLine(line.category, width);
            return entryLine(list[line.index], width, line.index === cursor);
        }

        function lineOf(index) {
            for (let i = 0; i < lines.length; i += 1) {
                if (lines[i].kind === 'entry' && lines[i].index === index) return i;
            }
            return 0;
        }

        function scrollToCursor() {
            const line = lineOf(cursor);
            const top = line > 0 && lines[line - 1].kind === 'heading' ? line - 1 : line;
            if (top < offset) offset = top;
            if (line >= offset + visibleRows) offset = line - visibleRows + 1;
            offset = Math.max(0, Math.min(offset, Math.max(0, lines.length - visibleRows)));
        }

        function render() {
            if (disposed || !term) return;
            wide = wideEmoji();
            const cols = Math.max(12, term.cols);
            const rows = Math.max(3, term.rows);
            let help = wrapHints(tr('desktop.terminal_retronet_help'), cols - 1, wide);
            if (editable()) help = help.concat(wrapHints(tr('desktop.terminal_retronet_help_admin'), cols - 1, wide));
            if (rows < HEADER_ROWS + help.length + 4) help = [];
            const footerRows = help.length ? help.length + 1 : 0;
            visibleRows = Math.max(1, rows - HEADER_ROWS - footerRows);
            scrollToCursor();
            // Auto-wrap off: overlong lines are clipped instead of scrolling the screen.
            let out = '\x1b[?7l\x1b[?25l' + RESET + at(0) + headerLine(cols) + at(1) + DIM + RULE.repeat(cols) + RESET;
            for (let i = 0; i < visibleRows; i += 1) {
                const row = HEADER_ROWS + i;
                out += at(row) + lineText(lines[offset + i], cols - 1);
                if (i === 0 && offset > 0) out += '\x1b[' + (row + 1) + ';' + cols + 'H' + DIM + '\u25b2' + RESET;
                else if (i === visibleRows - 1 && offset + visibleRows < lines.length) out += '\x1b[' + (row + 1) + ';' + cols + 'H' + DIM + '\u25bc' + RESET;
            }
            if (footerRows) {
                out += at(HEADER_ROWS + visibleRows) + DIM + RULE.repeat(cols) + RESET;
                help.forEach(function (text, index) {
                    out += at(HEADER_ROWS + visibleRows + 1 + index) + DIM + text + RESET;
                });
            }
            term.write(out);
        }

        function announceCursor() {
            const item = list[cursor];
            if (!item) return;
            const parts = [
                pad2(item.number) + ' ' + printable(item.entry.name),
                description(item.entry),
                statusLabel(statusOf(item.entry))
            ].filter(Boolean);
            announce(printable(tr('desktop.terminal_retronet_announce', { entry: parts.join('. '), position: cursor + 1, total: list.length })));
        }

        function select(index) {
            if (!list.length) return;
            cursor = Math.max(0, Math.min(list.length - 1, index));
            if (notice === 'desktop.terminal_retronet_not_own' || notice === 'desktop.terminal_retronet_status_failed') notice = '';
            render();
            announceCursor();
        }

        function jumpTo(number) {
            const index = list.findIndex(function (item) { return item.number === number; });
            if (index >= 0) select(index);
        }

        function typeDigit(digit) {
            const now = Date.now();
            if (pendingDigit && now - pendingDigit.at <= DIGIT_WINDOW_MS) {
                const number = pendingDigit.value * 10 + digit;
                pendingDigit = null;
                jumpTo(number);
                return;
            }
            pendingDigit = { value: digit, at: now };
            jumpTo(digit);
        }

        function dial() {
            const item = list[cursor];
            if (!item || dialing || disposed) return;
            dialing = true;
            onDial(item.entry);
        }

        function withOwnEntry(action) {
            const item = list[cursor];
            if (!item || !item.entry.own) {
                notice = 'desktop.terminal_retronet_not_own';
                render();
                return;
            }
            action(item.entry);
        }

        function handleData(input) {
            if (disposed || dialing || typeof input !== 'string' || !input) return;
            if (KEYS.up.indexOf(input) >= 0) { select(cursor - 1); return; }
            if (KEYS.down.indexOf(input) >= 0) { select(cursor + 1); return; }
            if (KEYS.pageUp.indexOf(input) >= 0) { select(cursor - Math.max(1, visibleRows - 1)); return; }
            if (KEYS.pageDown.indexOf(input) >= 0) { select(cursor + Math.max(1, visibleRows - 1)); return; }
            if (KEYS.home.indexOf(input) >= 0) { select(0); return; }
            if (KEYS.end.indexOf(input) >= 0) { select(list.length - 1); return; }
            if (KEYS.enter.indexOf(input) >= 0) { dial(); return; }
            if (KEYS.remove.indexOf(input) >= 0) {
                if (editable()) withOwnEntry(onDelete);
                return;
            }
            if (input.length !== 1) return;
            if (input >= '0' && input <= '9') {
                typeDigit(Number(input));
                return;
            }
            const key = input.toLowerCase();
            if (key === 'r') {
                refreshStatus();
                return;
            }
            if (!editable()) return;
            if (key === 'n') onEdit(null);
            else if (key === 'e') withOwnEntry(onEdit);
        }

        function handleMouse(row, double) {
            if (disposed || dialing) return;
            const listRow = Number(row) - HEADER_ROWS;
            if (!(listRow >= 0 && listRow < visibleRows)) return;
            const line = lines[offset + listRow];
            if (!line || line.kind !== 'entry') return;
            const now = Date.now();
            const again = lastTap.index === line.index && now - lastTap.at <= DOUBLE_TAP_MS && cursor === line.index;
            if (double || again) {
                lastTap = { index: -1, at: 0 };
                if (cursor !== line.index) select(line.index);
                dial();
                return;
            }
            lastTap = { index: line.index, at: now };
            select(line.index);
        }

        // Rejects after rendering the error view so the caller can react (403 -> shell).
        function load(preferId) {
            const gen = ++generation;
            const keep = preferId || (loaded ? (list[cursor] && list[cursor].entry.id) : readLast());
            failed = false;
            notice = 'desktop.terminal_retronet_loading';
            render();
            return Promise.resolve().then(function () {
                return api('/api/desktop/retronet/directory');
            }).then(function (body) {
                if (disposed || gen !== generation) return false;
                const payload = body && typeof body === 'object' ? body : {};
                data = {
                    entries: Array.isArray(payload.entries) ? payload.entries : [],
                    status: payload.status && typeof payload.status === 'object' ? payload.status : {},
                    stale: payload.stale === true,
                    canEdit: payload.can_edit === true
                };
                loaded = true;
                notice = '';
                rebuild();
                const index = list.findIndex(function (item) { return item.entry.id === keep; });
                cursor = index >= 0 ? index : 0;
                offset = 0;
                render();
                announceCursor();
                if (data.stale) refreshStatus();
                return true;
            }, function (err) {
                if (disposed || gen !== generation) return false;
                failed = true;
                notice = 'desktop.terminal_retronet_load_failed';
                render();
                throw err;
            });
        }

        function refreshStatus() {
            if (disposed) return Promise.resolve(false);
            if (failed) return load().then(function () { return true; }, function () { return false; });
            if (!loaded || checking) return Promise.resolve(false);
            checking = true;
            notice = 'desktop.terminal_retronet_checking';
            render();
            return Promise.resolve().then(function () {
                return api('/api/desktop/retronet/status', { method: 'POST' });
            }).then(function (body) {
                checking = false;
                if (disposed) return false;
                if (body && body.status && typeof body.status === 'object') data.status = body.status;
                notice = '';
                render();
                return true;
            }, function () {
                checking = false;
                if (disposed) return false;
                notice = 'desktop.terminal_retronet_status_failed';
                render();
                return false;
            });
        }

        function rowAt(event) {
            const screenEl = element && element.querySelector('.xterm-screen');
            if (!screenEl || !term.rows) return -1;
            const rect = screenEl.getBoundingClientRect();
            if (!rect.height || event.clientY < rect.top || event.clientY >= rect.bottom) return -1;
            return Math.floor((event.clientY - rect.top) / (rect.height / term.rows));
        }

        function onClick(event) {
            const row = rowAt(event);
            if (row >= 0) handleMouse(row, false);
        }

        function onDoubleClick(event) {
            const row = rowAt(event);
            if (row >= 0) handleMouse(row, true);
        }

        function dispose() {
            disposed = true;
            generation += 1;
            pendingDigit = null;
            if (element) {
                element.removeEventListener('click', onClick);
                element.removeEventListener('dblclick', onDoubleClick);
            }
        }

        if (element) {
            element.addEventListener('click', onClick);
            element.addEventListener('dblclick', onDoubleClick);
        }
        rebuild();

        return {
            load: load,
            render: render,
            handleData: handleData,
            handleMouse: handleMouse,
            refreshStatus: refreshStatus,
            selected: function () { return list[cursor] ? list[cursor].entry : null; },
            entries: function () { return data.entries.slice(); },
            dispose: dispose
        };
    }

    window.TerminalRetroNetDirectory = { LOCAL_SHELL_ID: LOCAL_SHELL_ID, create: create, cellWidth: cellWidth, fitToCells: fitToCells, printable: printable };
})();
