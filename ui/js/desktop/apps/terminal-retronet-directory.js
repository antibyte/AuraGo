(function () {
    'use strict';

    const TerminalText = window.TerminalText;
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

    // Display width in xterm cells (control characters do not count).
    function cells(value) {
        let width = 0;
        for (const ch of TerminalText.printable(value)) width += TerminalText.cellWidth(ch.codePointAt(0));
        return width;
    }

    // Packs the middle-dot-separated key hints into as few lines as fit; an overlong hint is cut.
    function wrapHints(value, width) {
        const lines = [];
        TerminalText.printable(value).split(HINT_SEPARATOR).forEach(function (part) {
            const last = lines.length - 1;
            if (last >= 0 && cells(lines[last] + HINT_SEPARATOR + part) <= width) lines[last] += HINT_SEPARATOR + part;
            else lines.push(TerminalText.fitToCells(part, width));
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
            if (entry.own) return TerminalText.printable(entry.description);
            if (!entry.description_key) return '';
            const text = String(t(entry.description_key));
            return text === entry.description_key ? '' : TerminalText.printable(text);
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
            return when ? TerminalText.printable(tr('desktop.terminal_retronet_last_seen', { when: when })) : '';
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
            out += ' \x1b[1m' + TerminalText.fitToCells(entry.name, nameWidth, true) + '\x1b[22m ';
            out += TerminalText.fitToCells(description(entry), descWidth, true);
            if (seen) out += ' ' + DIM + seen + '\x1b[22m';
            return out + RESET;
        }

        function headingLine(category, width) {
            return ' \x1b[1;4m' + TerminalText.fitToCells(tr('desktop.terminal_retronet_cat_' + category).toLocaleUpperCase(), width - 1) + RESET;
        }

        function headerLine(cols) {
            const title = TerminalText.fitToCells(tr('desktop.terminal_retronet_title'), cols - 2);
            let out = ' \x1b[1m' + title + RESET;
            const room = cols - 4 - cells(title);
            if (notice && room > 3) {
                const text = TerminalText.fitToCells(tr(notice), room);
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
            const cols = Math.max(12, term.cols);
            const rows = Math.max(3, term.rows);
            let help = wrapHints(tr('desktop.terminal_retronet_help'), cols - 1);
            if (editable()) help = help.concat(wrapHints(tr('desktop.terminal_retronet_help_admin'), cols - 1));
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
                pad2(item.number) + ' ' + TerminalText.printable(item.entry.name),
                description(item.entry),
                statusLabel(statusOf(item.entry))
            ].filter(Boolean);
            announce(TerminalText.printable(tr('desktop.terminal_retronet_announce', { entry: parts.join('. '), position: cursor + 1, total: list.length })));
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

    window.TerminalRetroNetDirectory = { LOCAL_SHELL_ID: LOCAL_SHELL_ID, create: create };
})();
