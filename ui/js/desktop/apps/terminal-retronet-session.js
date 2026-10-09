(function () {
    'use strict';

    const TerminalText = window.TerminalText;
    const HISTORY_LIMIT = 50;
    const FRAME_BYTES = 16 * 1024;
    const KEYS_UP = ['\x1b[A', '\x1bOA'];
    const KEYS_DOWN = ['\x1b[B', '\x1bOB'];
    // Cursor and editing keys the line editor has no use for: swallowed.
    const KEYS_IGNORED = ['\x1b[C', '\x1b[D', '\x1bOC', '\x1bOD', '\x1b[H', '\x1b[F', '\x1bOH', '\x1bOF',
        '\x1b[1~', '\x1b[2~', '\x1b[3~', '\x1b[4~', '\x1b[5~', '\x1b[6~', '\x1b[7~', '\x1b[8~'];
    // CSI, OSC/DCS/SOS/PM/APC strings (BEL or ST terminated), SS3 and two-character escapes (Alt+key).
    const ESCAPES = /\x1b\[[0-?]*[ -/]*[@-~]|\x1b[\]PX^_][\s\S]*?(?:\x07|\x1b\\)|\x1bO[\s\S]|\x1b[\s\S]/g;

    // Display width in xterm cells from the shared Unicode 6 table (emoji are one cell, as in xterm);
    // a Tab in the line is shown as one space.
    function cells(text) {
        let width = 0;
        for (const ch of String(text || '')) width += ch === '\t' ? 1 : TerminalText.cellWidth(ch.codePointAt(0));
        return width;
    }

    function isRegional(code) {
        return code >= 0x1f1e6 && code <= 0x1f1ff;
    }

    // Combining marks, variation selectors, ZWJ and skin tones belong to the character before them.
    function joinsPrevious(ch) {
        const code = ch.codePointAt(0);
        return ch !== '\t' && (TerminalText.cellWidth(code) === 0 || (code >= 0x1f3fb && code <= 0x1f3ff));
    }

    // Index where the last character starts, as xterm draws and TerminalText.fitToCells keeps it together:
    // a base with its marks and skin tones, ZWJ-joined code points, a regional-indicator pair.
    function lastCharacterStart(chars) {
        let start = chars.length - 1;
        for (;;) {
            while (start > 0 && joinsPrevious(chars[start])) start -= 1;
            if (start > 1 && chars[start - 1].codePointAt(0) === 0x200d) {
                start -= 2;
                continue;
            }
            break;
        }
        if (start > 0 && isRegional(chars[start].codePointAt(0))) {
            let run = 0;
            for (let i = start; i >= 0 && isRegional(chars[i].codePointAt(0)); i -= 1) run += 1;
            if (run % 2 === 0) start -= 1;
        }
        return start;
    }

    // Typed text without control characters; Tabs stay in the line.
    function lineText(text) {
        return String(text || '').split('\t').map(TerminalText.printable).join('\t');
    }

    // Bytes TextEncoder writes for one code point (a lone surrogate becomes U+FFFD, three bytes).
    function utf8Length(code) {
        if (code < 0x80) return 1;
        if (code < 0x800) return 2;
        return code < 0x10000 ? 3 : 4;
    }

    function positive(value, fallback) {
        const number = Math.floor(Number(value));
        return Number.isFinite(number) && number > 0 ? number : fallback;
    }

    // The browser never sends host or port: the server dials stored entries by ID.
    function socketURL(entry, cols, rows) {
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        return protocol + '//' + location.host + '/api/desktop/retronet/connect?entry=' +
            encodeURIComponent(String(entry.id || '')) + '&cols=' + cols + '&rows=' + rows;
    }

    // Answer typed at the host-key prompt: the localized letters first (they win a collision),
    // then ASCII y/n, case-insensitive. Returns true (accept), false (reject) or null (no answer).
    function hostKeyAnswer(input, yes, no) {
        const key = TerminalText.printable(input).trim().toLowerCase();
        if (!key) return null;
        const localYes = TerminalText.printable(yes).trim().toLowerCase();
        const localNo = TerminalText.printable(no).trim().toLowerCase();
        if (localYes && key === localYes) return true;
        if (localNo && key === localNo) return false;
        if (key === 'y') return true;
        if (key === 'n') return false;
        return null;
    }

    function open(options) {
        const opts = options || {};
        const term = opts.term;
        const entry = opts.entry || {};
        const onControl = typeof opts.onControl === 'function' ? opts.onControl : function () {};
        const onData = typeof opts.onData === 'function' ? opts.onData : function () {};
        const onClose = typeof opts.onClose === 'function' ? opts.onClose : function () {};
        const encoder = new TextEncoder();
        const lineCapable = entry.protocol === 'telnet' && entry.kind === 'world';
        const initial = { cols: positive(opts.cols, 80), rows: positive(opts.rows, 25) };
        const size = { cols: initial.cols, rows: initial.rows };
        const history = [];
        let remoteEcho = false; // echo.remote: character mode, every key goes out at once
        let hiddenEcho = false; // echo.hidden: line mode without local echo (password prompts)
        let line = '';
        let draft = '';
        let historyIndex = -1;
        let closed = false;
        const ws = new WebSocket(socketURL(entry, initial.cols, initial.rows));
        ws.binaryType = 'arraybuffer';

        function isOpen() {
            return !closed && ws.readyState === WebSocket.OPEN;
        }

        // UTF-8 frames of at most FRAME_BYTES, cut between code points: the server reads at most 64 KiB
        // per message, so a large paste must never become one frame.
        function sendBytes(text) {
            if (!text || !isOpen()) return;
            let start = 0;
            let index = 0;
            let bytes = 0;
            for (const ch of text) {
                const size = utf8Length(ch.codePointAt(0));
                if (bytes + size > FRAME_BYTES) {
                    ws.send(encoder.encode(text.slice(start, index)));
                    start = index;
                    bytes = 0;
                }
                bytes += size;
                index += ch.length;
            }
            ws.send(encoder.encode(text.slice(start)));
        }

        function sendControl(message) {
            if (!isOpen()) return;
            ws.send(JSON.stringify(message));
        }

        // Local echo of typed characters; silent while the server hides the echo. Server text never
        // passes here: it reaches xterm only as the raw data stream through onData.
        function echo(text) {
            if (term && text && !hiddenEcho) term.write(TerminalText.printable(text.split('\t').join(' ')));
        }

        // Moves back over `text`, blanks its cells and moves back again (nothing while hidden).
        function erase(text) {
            const width = cells(text);
            if (width > 0 && term && !hiddenEcho) term.write('\b'.repeat(width) + ' '.repeat(width) + '\b'.repeat(width));
        }

        function replaceLine(text) {
            erase(line);
            line = lineText(text);
            echo(line);
        }

        function remember(text) {
            if (!text.trim() || history[history.length - 1] === text) return;
            history.push(text);
            if (history.length > HISTORY_LIMIT) history.shift();
        }

        function submit() {
            const text = line;
            const secret = hiddenEcho;
            line = '';
            draft = '';
            historyIndex = -1;
            if (term) term.write('\r\n');
            if (!secret) remember(text);
            sendBytes(text + '\r');
        }

        function backspace() {
            if (!line) return;
            const chars = Array.from(line);
            const start = lastCharacterStart(chars);
            const removed = chars.slice(start).join('');
            line = chars.slice(0, start).join('');
            erase(removed);
        }

        function recall(direction) {
            if (!history.length) return;
            if (historyIndex === -1) {
                if (direction > 0) return;
                draft = line;
                historyIndex = history.length - 1;
                replaceLine(history[historyIndex]);
                return;
            }
            const next = historyIndex + direction;
            if (next < 0) return;
            if (next >= history.length) {
                historyIndex = -1;
                replaceLine(draft);
                return;
            }
            historyIndex = next;
            replaceLine(history[historyIndex]);
        }

        // Local line editing for Telnet world entries outside character mode (echo.remote false).
        // While echo.hidden is true the buffer is edited invisibly and never enters the history.
        function editLine(input) {
            if (KEYS_UP.indexOf(input) >= 0) {
                if (!hiddenEcho) recall(-1);
                return;
            }
            if (KEYS_DOWN.indexOf(input) >= 0) {
                if (!hiddenEcho) recall(1);
                return;
            }
            if (KEYS_IGNORED.indexOf(input) >= 0) return;
            // A lone Escape goes out as a key; escape sequences inside other input (pasted coloured text,
            // paste markers, Alt+key) are removed and the remaining text is edited.
            const text = input === '\x1b' ? input : input.replace(ESCAPES, '');
            const chars = Array.from(text);
            let typed = '';
            // Typed characters are echoed in one write per run, before anything else reaches the screen.
            const flush = function () {
                if (!typed) return;
                echo(typed);
                typed = '';
            };
            for (let i = 0; i < chars.length; i += 1) {
                const ch = chars[i];
                const code = ch.codePointAt(0);
                if (ch === '\n' && i > 0 && chars[i - 1] === '\r') continue;
                if (ch === '\t' || (code >= 0x20 && !(code >= 0x7f && code < 0xa0))) {
                    line += ch;
                    typed += ch;
                    historyIndex = -1;
                    continue;
                }
                flush();
                if (ch === '\r' || ch === '\n') {
                    submit();
                } else if (code === 0x7f || code === 0x08) {
                    backspace();
                } else if (code === 0x15) {
                    erase(line);
                    line = '';
                } else {
                    sendBytes(ch);
                }
            }
            flush();
        }

        // Keystrokes before the socket is open are dropped: the coordinator forwards keys only after the
        // connected control, which arrives on an open socket.
        function send(input) {
            if (!isOpen() || typeof input !== 'string' || !input) return;
            if (lineCapable && !remoteEcho) {
                editLine(input);
                return;
            }
            sendBytes(input);
        }

        // echo control: remote = character mode; hidden = line mode without local echo.
        function setEchoMode(remote, hidden) {
            const nextHidden = !remote && hidden;
            if (remote === remoteEcho && nextHidden === hiddenEcho) return;
            const pending = line;
            if (remote) {
                // Character mode: hand an unfinished line to the server, which handles echo from now on.
                if (pending) erase(pending);
                line = '';
                draft = '';
                historyIndex = -1;
                remoteEcho = true;
                hiddenEcho = false;
                if (pending) sendBytes(pending);
                return;
            }
            remoteEcho = false;
            if (nextHidden && !hiddenEcho) {
                // Echo goes dark mid-line: remove the visible part and keep editing invisibly.
                if (pending) erase(pending);
                hiddenEcho = true;
                draft = '';
                historyIndex = -1;
                return;
            }
            if (!nextHidden && hiddenEcho) {
                // Never reveal what was typed while the echo was hidden.
                hiddenEcho = false;
                line = '';
                draft = '';
                historyIndex = -1;
            }
        }

        function resize(cols, rows) {
            const nextCols = positive(cols, size.cols);
            const nextRows = positive(rows, size.rows);
            if (nextCols === size.cols && nextRows === size.rows) return;
            size.cols = nextCols;
            size.rows = nextRows;
            sendControl({ type: 'resize', cols: nextCols, rows: nextRows });
        }

        function hostKeyDecision(accept) {
            sendControl({ type: 'hostkey_decision', accept: accept === true });
        }

        function detach() {
            ws.onopen = null;
            ws.onmessage = null;
            ws.onerror = null;
            ws.onclose = null;
        }

        // Once the session ends, typed text (a hidden password buffer included) and the history are dropped.
        function forget() {
            line = '';
            draft = '';
            historyIndex = -1;
            history.length = 0;
        }

        // hangup() and dispose(): close the socket; no callbacks fire afterwards.
        function shutdown() {
            if (closed) return;
            closed = true;
            detach();
            forget();
            if (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN) {
                try { ws.close(1000); } catch (e) {}
            }
        }

        ws.onopen = function () {
            if (closed) return;
            if (size.cols !== initial.cols || size.rows !== initial.rows) {
                sendControl({ type: 'resize', cols: size.cols, rows: size.rows });
            }
        };
        ws.onmessage = function (event) {
            if (closed) return;
            if (event.data instanceof ArrayBuffer) {
                onData(new Uint8Array(event.data));
                return;
            }
            if (typeof event.data !== 'string') return;
            let control = null;
            try {
                control = JSON.parse(event.data);
            } catch (e) {
                return;
            }
            if (!control || typeof control.type !== 'string') return;
            if (control.type === 'echo') setEchoMode(control.remote === true, control.hidden === true);
            onControl(control);
        };
        ws.onerror = function () {};
        ws.onclose = function (event) {
            if (closed) return;
            closed = true;
            detach();
            forget();
            onClose(event);
        };

        return { send: send, resize: resize, hostKeyDecision: hostKeyDecision, hangup: shutdown, dispose: shutdown };
    }

    window.TerminalRetroNetSession = { open: open, hostKeyAnswer: hostKeyAnswer };
})();
