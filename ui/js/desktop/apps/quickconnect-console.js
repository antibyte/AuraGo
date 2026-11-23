    // Quick Connect console deck helpers: status bar and session timer, phosphor
    // switch, xterm theme resolution, boot screen markup, dialog accessibility and
    // list keyboard navigation. Pure helpers bundled inside the Desktop shell IIFE
    // (shared esc/iconMarkup); renderQuickConnect in quickconnect-launchpad-chat.js
    // owns the session state and calls them. The serial modules do not depend on
    // this file.
    const QuickConnectConsole = (() => {
        const PHOSPHORS = ['auto', 'green', 'amber', 'ice'];
        const PHOSPHOR_STORAGE_KEY = 'aurago.quickConnect.phosphor';
        const STATES = ['idle', 'connecting', 'connected', 'disconnected', 'error'];
        const ANSI_PALETTE = {
            black: '#0d1117', red: '#ff7b72', green: '#3fb950', yellow: '#d29922',
            blue: '#58a6ff', magenta: '#bc8cff', cyan: '#39c5cf', white: '#c9d1d9',
            brightBlack: '#484f58', brightRed: '#ffa198', brightGreen: '#56d364',
            brightYellow: '#e3b341', brightBlue: '#79c0ff', brightMagenta: '#d2a8ff',
            brightCyan: '#56d4dd', brightWhite: '#f0f6fc'
        };
        const FALLBACK_THEME = {
            background: '#0d1117', foreground: '#c9d1d9', cursor: '#58a6ff',
            cursorAccent: '#0d1117', selectionBackground: 'rgba(88, 166, 255, 0.3)'
        };
        const FOCUSABLE = 'button:not([disabled]), a[href], input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';
        let memoryPhosphor = 'auto';
        let dialogSeq = 0;

        // Phosphor choice. localStorage may be blocked (sandboxed frames, opaque
        // origins), so the last value is also kept in memory for re-renders.
        function readPhosphor() {
            let value = memoryPhosphor;
            try { value = localStorage.getItem(PHOSPHOR_STORAGE_KEY) || value; } catch (_) {}
            return PHOSPHORS.includes(value) ? value : 'auto';
        }

        function writePhosphor(value) {
            memoryPhosphor = value;
            try { localStorage.setItem(PHOSPHOR_STORAGE_KEY, value); } catch (_) {}
        }

        function nextPhosphor(current) {
            const index = PHOSPHORS.indexOf(current);
            return PHOSPHORS[(index + 1) % PHOSPHORS.length];
        }

        // mm:ss, or hh:mm:ss from one hour on.
        function formatDuration(ms) {
            const total = Math.max(0, Math.floor(Number(ms) / 1000)) || 0;
            const pad = value => String(value).padStart(2, '0');
            const hours = Math.floor(total / 3600);
            const minutes = Math.floor((total % 3600) / 60);
            const seconds = total % 60;
            return hours ? `${pad(hours)}:${pad(minutes)}:${pad(seconds)}` : `${pad(minutes)}:${pad(seconds)}`;
        }

        function statusBarMarkup(tr) {
            return `<div class="vd-qc-statusbar" data-qc-statusbar data-state="idle">
                <span class="vd-qc-status-state" role="status"><span class="vd-qc-led" aria-hidden="true"></span><span data-qc-status-label>${esc(tr('desktop.qc_status_ready'))}</span></span>
                <span class="vd-qc-status-item" data-qc-status-proto hidden></span>
                <span class="vd-qc-status-item vd-qc-status-target" data-qc-status-target hidden></span>
                <span class="vd-qc-status-item" data-qc-status-time title="${esc(tr('desktop.qc_session_time'))}" aria-hidden="true" hidden></span>
                <span class="vd-qc-status-spacer"></span>
                <span class="vd-qc-status-item" data-qc-status-size title="${esc(tr('desktop.qc_terminal_size'))}" hidden></span>
                <button class="vd-qc-status-btn" type="button" data-qc-phosphor-toggle>
                    <span class="vd-qc-phosphor-swatch" aria-hidden="true"></span><span data-qc-phosphor-label></span>
                </button>
            </div>`;
        }

        function bootMarkup(tr) {
            return `<div class="vd-qc-placeholder vd-qc-boot" data-qc-boot>
                <div class="vd-qc-boot-inner">
                    <div class="vd-qc-boot-title">${esc(tr('desktop.qc_title'))}</div>
                    <div class="vd-qc-boot-sub">${esc(tr('desktop.qc_protocol_ssh'))} · ${esc(tr('desktop.qc_protocol_vnc'))} · ${esc(tr('desktop.qc_serial_filter'))}</div>
                    <div class="vd-qc-boot-rule" aria-hidden="true"></div>
                    <div class="vd-qc-boot-line"><span class="vd-qc-boot-prompt" aria-hidden="true">&gt;</span><span class="vd-qc-placeholder-text">${esc(tr('desktop.qc_select_device'))}</span><span class="vd-qc-cursor" aria-hidden="true"></span></div>
                    <div class="vd-qc-boot-keys">
                        <span><kbd>↑</kbd><kbd>↓</kbd> ${esc(tr('desktop.qc_hint_navigate'))}</span>
                        <span><kbd>Enter</kbd> ${esc(tr('desktop.qc_connect'))}</span>
                        <span><kbd>/</kbd> ${esc(tr('desktop.qc_hint_search'))}</span>
                    </div>
                </div>
            </div>`;
        }

        // Status bar controller. set() takes { state, protocol, target }; the timer
        // runs only while the state is "connected".
        function createStatusBar(bar, tr) {
            const part = name => bar.querySelector('[data-qc-status-' + name + ']');
            const label = part('label');
            const proto = part('proto');
            const target = part('target');
            const time = part('time');
            const size = part('size');
            let state = 'idle';
            let timer = null;
            let startedAt = 0;

            function stopTimer() {
                if (timer !== null) { clearInterval(timer); timer = null; }
            }
            function tick() { time.textContent = formatDuration(Date.now() - startedAt); }

            function set(next) {
                const nextState = STATES.includes(next && next.state) ? next.state : 'idle';
                const active = nextState !== 'idle';
                bar.dataset.state = nextState;
                label.textContent = tr('desktop.qc_status_' + (nextState === 'idle' ? 'ready' : nextState));
                proto.textContent = active ? (next.protocol || '') : '';
                proto.hidden = !proto.textContent;
                target.textContent = active ? (next.target || '') : '';
                target.title = target.textContent;
                target.hidden = !target.textContent;
                if (nextState === 'connected') {
                    if (state !== 'connected' || timer === null) {
                        stopTimer();
                        startedAt = Date.now();
                        tick();
                        timer = setInterval(tick, 1000);
                    }
                    time.hidden = false;
                } else {
                    stopTimer();
                    time.hidden = true;
                    time.textContent = '';
                }
                if (!active) setSize();
                state = nextState;
            }

            function setSize(cols, rows) {
                const known = Number(cols) > 0 && Number(rows) > 0;
                size.textContent = known ? `${cols}×${rows}` : '';
                size.hidden = !known;
            }

            return { set, setSize, dispose: stopTimer, get state() { return state; } };
        }

        // Cycles the phosphor switch and keeps the label, aria-label and the
        // data-qc-phosphor attribute on the app root in step.
        function wirePhosphor(root, bar, tr, onChange) {
            const button = bar.querySelector('[data-qc-phosphor-toggle]');
            const labelEl = bar.querySelector('[data-qc-phosphor-label]');
            function apply(id) {
                root.dataset.qcPhosphor = id;
                const name = tr('desktop.qc_phosphor_' + id);
                const text = tr('desktop.qc_phosphor_label').replace('{{name}}', name);
                labelEl.textContent = name;
                button.setAttribute('aria-label', text);
                button.title = text;
            }
            apply(readPhosphor());
            button.addEventListener('click', () => {
                const id = nextPhosphor(root.dataset.qcPhosphor);
                writePhosphor(id);
                apply(id);
                if (typeof onChange === 'function') onChange(id);
            });
        }

        // Resolves the --qc-term-* custom properties to concrete colours. A probe
        // element gives the computed colour; a 1x1 canvas turns oklch() and
        // color-mix() results into rgb. Any failure returns the old theme.
        function quickConnectTerminalTheme(root) {
            try {
                const styles = getComputedStyle(root);
                const probe = document.createElement('span');
                probe.style.display = 'none';
                root.appendChild(probe);
                const canvas = document.createElement('canvas');
                canvas.width = 1;
                canvas.height = 1;
                const ctx = canvas.getContext('2d', { willReadFrequently: true });
                const resolve = (name, fallback) => {
                    if (!styles.getPropertyValue(name).trim() || !ctx) return fallback;
                    probe.style.color = '';
                    probe.style.color = 'var(' + name + ')';
                    const computed = getComputedStyle(probe).color;
                    ctx.clearRect(0, 0, 1, 1);
                    ctx.fillStyle = '#000000';
                    ctx.fillStyle = computed;
                    ctx.fillRect(0, 0, 1, 1);
                    const [r, g, b, a] = ctx.getImageData(0, 0, 1, 1).data;
                    return a === 255 ? `rgb(${r}, ${g}, ${b})` : `rgba(${r}, ${g}, ${b}, ${Math.round(a / 255 * 100) / 100})`;
                };
                const theme = {
                    background: resolve('--qc-term-bg', FALLBACK_THEME.background),
                    foreground: resolve('--qc-term-fg', FALLBACK_THEME.foreground),
                    cursor: resolve('--qc-term-cursor', FALLBACK_THEME.cursor),
                    selectionBackground: resolve('--qc-term-selection', FALLBACK_THEME.selectionBackground)
                };
                probe.remove();
                theme.cursorAccent = theme.background;
                return theme;
            } catch (_) {
                return { ...FALLBACK_THEME };
            }
        }

        // Dialog accessibility: role, labelled title, focus in, Tab trap and
        // Escape. Returns release(), which puts focus back on the opener; call it
        // before removing the overlay.
        function openDialog(overlay, options) {
            const opts = options || {};
            const dialog = overlay.firstElementChild;
            const title = dialog.querySelector('.vd-qc-modal-title, .vd-qc-confirm-title');
            const opener = document.activeElement;
            dialog.setAttribute('role', 'dialog');
            dialog.setAttribute('aria-modal', 'true');
            if (title) {
                if (!title.id) title.id = 'vd-qc-dialog-title-' + (++dialogSeq);
                dialog.setAttribute('aria-labelledby', title.id);
            }
            const focusable = () => Array.from(dialog.querySelectorAll(FOCUSABLE)).filter(node => node.getClientRects().length > 0);
            overlay.addEventListener('keydown', event => {
                if (event.key === 'Escape') {
                    event.preventDefault();
                    event.stopPropagation();
                    if (typeof opts.onCancel === 'function') opts.onCancel();
                    return;
                }
                if (event.key !== 'Tab') return;
                const nodes = focusable();
                if (!nodes.length) { event.preventDefault(); return; }
                const first = nodes[0];
                const last = nodes[nodes.length - 1];
                if (!dialog.contains(document.activeElement)) { event.preventDefault(); first.focus(); }
                else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
                else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
            });
            const initial = opts.initialFocus || focusable()[0];
            if (initial) initial.focus();
            return function release() {
                const current = document.activeElement;
                const stranded = !current || current === document.body || overlay.contains(current);
                if (stranded && opener && opener !== document.body && opener.isConnected && typeof opener.focus === 'function') opener.focus();
            };
        }

        // Keyboard model of the sidebar: "/" focuses the search, ArrowDown leaves
        // the search for the first row, rows move with arrows/Home/End, Enter in
        // the search connects the first device and Escape clears the query.
        function wireKeyboard({ root, searchInput, deviceList, serialList, getFilter }) {
            const visibleRows = () => [
                ...(deviceList.hidden ? [] : deviceList.querySelectorAll('[data-device-id]')),
                ...(serialList.hidden ? [] : serialList.querySelectorAll('[data-serial-profile]'))
            ];
            const firstRow = () => {
                const rows = visibleRows();
                return getFilter() === 'serial' ? rows[0] : rows.find(row => row.matches('[data-device-id]')) || rows[0];
            };
            root.addEventListener('keydown', event => {
                if (event.key !== '/' || event.ctrlKey || event.metaKey || event.altKey || event.defaultPrevented) return;
                if (event.target.closest && event.target.closest('input, textarea, select, [contenteditable=""], [contenteditable="true"]')) return;
                event.preventDefault();
                searchInput.focus();
                searchInput.select();
            });
            searchInput.addEventListener('keydown', event => {
                if (event.key === 'ArrowDown') {
                    const row = firstRow();
                    if (row) { event.preventDefault(); row.focus(); }
                } else if (event.key === 'Enter') {
                    const row = deviceList.hidden ? null : deviceList.querySelector('[data-device-id]');
                    if (row) { event.preventDefault(); row.click(); }
                } else if (event.key === 'Escape' && searchInput.value) {
                    event.preventDefault();
                    event.stopPropagation();
                    searchInput.value = '';
                    searchInput.dispatchEvent(new Event('input', { bubbles: true }));
                }
            });
            const onRowKey = event => {
                if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) return;
                const rows = visibleRows();
                const index = rows.indexOf(event.target.closest('[data-device-id], [data-serial-profile]'));
                if (index < 0) return;
                let next = null;
                if (event.key === 'ArrowDown') next = rows[Math.min(index + 1, rows.length - 1)];
                else if (event.key === 'ArrowUp') next = index === 0 ? searchInput : rows[index - 1];
                else if (event.key === 'Home') next = rows[0];
                else if (event.key === 'End') next = rows[rows.length - 1];
                if (!next) return;
                event.preventDefault();
                next.focus();
            };
            deviceList.addEventListener('keydown', onRowKey);
            serialList.addEventListener('keydown', onRowKey);
        }

        // ArrowLeft/ArrowRight move between the visible tabs.
        function wireTabs(tabs, activate) {
            tabs.addEventListener('keydown', event => {
                if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return;
                const nodes = Array.from(tabs.querySelectorAll('[role="tab"]')).filter(node => !node.hidden);
                const index = nodes.indexOf(event.target.closest('[role="tab"]'));
                if (index < 0) return;
                event.preventDefault();
                const next = nodes[(index + (event.key === 'ArrowRight' ? 1 : nodes.length - 1)) % nodes.length];
                next.focus();
                activate(next.dataset.tab);
            });
        }

        return {
            ANSI_PALETTE, PHOSPHORS, readPhosphor, writePhosphor, nextPhosphor, formatDuration,
            statusBarMarkup, bootMarkup, createStatusBar, wirePhosphor, quickConnectTerminalTheme,
            openDialog, wireKeyboard, wireTabs
        };
    })();
