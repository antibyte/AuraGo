/* Docker Manager terminal: an xterm.js panel on a WebSocket to /api/docker-manager/hosts/{h}/containers/{ref}/terminal, for the
   container drawer (one container) and the stack pane (a picker over the running service containers). A classic script
   (window.DockerManagerTerminal) with no DOM work at load.
   - Protocol (the same as the Containers page): input and output are binary frames; {"type":"resize","cols","rows"} and
     {"type":"end"} are text frames. Closing the panel sends "end" first, so the shell does not stay behind in the container.
   - A browser hides the answer of a refused handshake. After a failed attempt the panel asks the store for the JSON error of a plain
     GET (store.diagnose): a protected container (409 container_protected_confirmation_required) opens the confirmation dialog and
     connects again with ?confirm=protected; everything else is shown as a sentence.
   - Keys: every key goes to the terminal, Escape and Ctrl+K included; the keydown never leaves the screen element, so the palette,
     the shortcuts of the app and the desktop never see it. Ctrl+] (or the Leave terminal button) moves the focus to the toolbar.
   - Colours come from the --dm-term-* custom properties of the stylesheet, resolved through a probe element and a canvas so that
     color-mix() and light-dark() reach xterm as plain rgb(). */
(function () {
    'use strict';

    const BASE = '/api/docker-manager';
    const THEME_KEYS = ['background', 'foreground', 'cursor', 'selectionBackground', 'black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white'];
    const THEME_VARS = { background: 'bg', foreground: 'fg', cursor: 'cursor', selectionBackground: 'sel' };
    const enc = encodeURIComponent;

    // ---- pure helpers ------------------------------------------------------------------------------------------------

    function socketUrl(location, hostId, ref, confirmed) {
        const scheme = location && location.protocol === 'https:' ? 'wss' : 'ws';
        return scheme + '://' + (location && location.host) + BASE + '/hosts/' + enc(hostId) + '/containers/' + enc(ref) + '/terminal' + (confirmed ? '?confirm=protected' : '');
    }

    // The path (below /api/docker-manager) the diagnosis of a failed handshake asks for: the same URL as a plain GET. After the person confirmed a
    // protected container it carries the confirmation too, otherwise a later 429 or 502 would be answered as the protection again.
    function diagnosePath(hostId, ref, confirmed) {
        return '/hosts/' + enc(hostId) + '/containers/' + enc(ref) + '/terminal' + (confirmed ? '?confirm=protected' : '');
    }

    function targetOf(container, label) {
        const model = window.DockerManagerModel, c = container || {};
        return { ref: model.containerId(c), label: label || model.containerName(c), c, state: model.terminalState(c), protected: model.isProtected(c) };
    }
    // The picker of a stack: its running services, each with the container the list knows (for the lock and the full id). The service's
    // container id may be the 12-character id or the full one; the list's `id` is the 12-character one.
    function stackTargets(detail, containers) {
        const out = [], list = Array.isArray(containers) ? containers : [];
        for (const service of Array.isArray(detail && detail.service_list) ? detail.service_list : []) {
            if (!service || service.state !== 'running' || !service.container_id) continue;
            const wanted = String(service.container_id).slice(0, 12), c = list.find(item => item && String(item.id).slice(0, 12) === wanted);
            if (c) out.push(targetOf(c, service.name));
        }
        return out;
    }
    // Why no terminal is opened for a target without asking the server: a key of docker_manager.term_*, or ''.
    function blockedBy(target, readonly) {
        if (readonly) return 'term_readonly';
        if (!target) return 'term_none_running';
        if (target.state === 'helper') return 'term_helper';
        if (target.state === 'not_running') return 'term_not_running';
        return '';
    }
    const utf8 = text => new TextEncoder().encode(text);

    // Resolves a CSS colour (also color(srgb ...) from color-mix) to rgb()/rgba() through a one-pixel canvas.
    function toRgb(css) {
        const canvas = document.createElement('canvas');
        canvas.width = canvas.height = 1;
        const context = canvas.getContext('2d', { willReadFrequently: true });
        if (!context) return css;
        context.clearRect(0, 0, 1, 1);
        context.fillStyle = '#000000';
        context.fillStyle = css;
        context.fillRect(0, 0, 1, 1);
        const d = context.getImageData(0, 0, 1, 1).data;
        return d[3] === 255 ? 'rgb(' + d[0] + ', ' + d[1] + ', ' + d[2] + ')' : 'rgba(' + d[0] + ', ' + d[1] + ', ' + d[2] + ', ' + Math.round(d[3] / 255 * 100) / 100 + ')';
    }
    // The xterm theme for an element that defines the --dm-term-* properties.
    function readTheme(element) {
        const probe = document.createElement('span');
        probe.className = 'dm-tp-probe';
        element.append(probe);
        const theme = {};
        try {
            for (const key of THEME_KEYS) {
                probe.style.color = 'var(--dm-term-' + (THEME_VARS[key] || key) + ')';
                theme[key] = toRgb(getComputedStyle(probe).color);
            }
        } finally { probe.remove(); }
        for (const key of ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white']) theme['bright' + key[0].toUpperCase() + key.slice(1)] = theme[key];
        theme.cursorAccent = theme.background;
        return theme;
    }

    // ---- the panel -----------------------------------------------------------------------------------------------------

    function create(deps) {
        const { t, esc, views, diagnose, confirmProtected } = deps;
        const openSocket = typeof deps.socket === 'function' ? deps.socket : url => new WebSocket(url);
        const where = deps.location || (typeof window !== 'undefined' ? window.location : { protocol: 'http:', host: '' });
        const frame = typeof deps.requestFrame === 'function' ? deps.requestFrame : fn => requestAnimationFrame(fn);

        // config = {hostId, targets: () => [target], readonly: () => bool, autofocus}. The targets function is asked again on update().
        function open(el, config) {
            const lifetime = new AbortController(), signal = lifetime.signal;
            let term = null, fit = null, socket = null, resizeObserver = null, disposed = false, active = false, phase = 'idle', message = '', generation = 0;
            let current = '', opened = false, optionSig = '', fitQueued = false, wantFocus = !!config.autofocus;
            const readonly = () => !!(config.readonly && config.readonly());
            const targets = () => (typeof config.targets === 'function' ? config.targets() : []);
            const targetFor = ref => targets().find(item => item.ref === ref) || null;

            el.innerHTML = '<div class="dm-tp" data-term>'
                + '<div class="dm-tp-bar"><label class="dm-inline" data-tp-pick-wrap hidden><span>' + esc(t('term_container')) + '</span><select data-tp-pick></select></label>'
                + '<span class="dm-tp-status" data-tp-status role="status"></span>'
                + '<button type="button" class="dm-btn" data-tp-start hidden><span></span></button><span class="dm-spacer"></span>'
                + '<span class="dm-tp-hint">' + esc(t('term_hint')) + '</span>'
                + '<button type="button" class="dm-btn" data-tp-leave><span>' + esc(t('term_leave')) + '</span></button></div>'
                + '<p class="dm-tp-msg" data-tp-msg role="status" hidden></p>'
                + '<div class="dm-tp-screen" data-tp-screen></div></div>';
            const q = selector => el.querySelector(selector);
            const screen = q('[data-tp-screen]'), statusEl = q('[data-tp-status]'), msgEl = q('[data-tp-msg]'), pickEl = q('[data-tp-pick]'), pickWrap = q('[data-tp-pick-wrap]');
            const startEl = q('[data-tp-start]'), leaveEl = q('[data-tp-leave]');

            function paint() {
                statusEl.textContent = t('term_status_' + phase);
                statusEl.dataset.phase = phase;
                msgEl.hidden = !message;
                msgEl.textContent = message;
                screen.hidden = !!message && phase !== 'connected' && phase !== 'closed';
                const label = phase === 'idle' ? (message ? '' : t('term_open')) : phase === 'closed' || phase === 'error' ? t('term_reconnect') : '';
                startEl.hidden = !label || !!blockedBy(targetFor(current), readonly());
                if (label) startEl.querySelector('span').textContent = label;
            }
            function paintPicker() {
                const list = targets();
                const sig = list.map(item => item.ref + '|' + item.label).join('\n') + '#' + current;
                if (sig === optionSig) return;
                optionSig = sig;
                pickWrap.hidden = list.length < 2;
                pickEl.innerHTML = list.map(item => '<option value="' + esc(item.ref) + '"' + (item.ref === current ? ' selected' : '') + '>' + esc(item.label) + '</option>').join('');
            }

            // ---- xterm and the socket --------------------------------------------------------------------------------------

            function ensureTerm() {
                if (term) return true;
                if (typeof window.Terminal !== 'function') return false;
                screen.hidden = false;
                term = new window.Terminal({
                    cursorBlink: true, scrollback: 2000, fontSize: 13, fontFamily: getComputedStyle(screen).fontFamily, minimumContrastRatio: 4.5,
                    theme: readTheme(screen)
                });
                if (window.FitAddon && window.FitAddon.FitAddon) { fit = new window.FitAddon.FitAddon(); term.loadAddon(fit); }
                term.open(screen);
                if (term.textarea) term.textarea.setAttribute('aria-label', t('term_input_label'));
                term.attachCustomKeyEventHandler(event => {
                    if (event.type === 'keydown' && event.ctrlKey && !event.altKey && !event.metaKey && (event.key === ']' || event.code === 'BracketRight')) { leave(); return false; }
                    return true;
                });
                term.onData(data => send(utf8(data)));
                term.onBinary(data => send(Uint8Array.from(data, ch => ch.charCodeAt(0))));
                // The keydown stays inside the screen: the app's shortcuts and the desktop's Spotlight never see what is typed here.
                screen.addEventListener('keydown', event => event.stopPropagation(), { signal });
                if (typeof ResizeObserver === 'function') { resizeObserver = new ResizeObserver(() => queueFit()); resizeObserver.observe(screen); }
                return true;
            }
            // Input is dropped while read-only is on, even if the socket is still open (update() ends the session on the next paint).
            function send(data) { if (!readonly() && socket && socket.readyState === 1) socket.send(data); }
            function doFit() {
                fitQueued = false;
                if (disposed || !term || !fit || !active || screen.clientWidth < 20 || screen.clientHeight < 20) return;
                try { fit.fit(); } catch (error) { return; }
                if (socket && socket.readyState === 1) socket.send(JSON.stringify({ type: 'resize', cols: term.cols, rows: term.rows }));
            }
            function queueFit() { if (!fitQueued) { fitQueued = true; frame(doFit); } }

            function closeSocket(askEnd) {
                generation += 1;
                if (!socket) return;
                const old = socket;
                socket = null;
                old.onopen = old.onmessage = old.onerror = old.onclose = null;
                try {
                    if (askEnd && old.readyState === 1) old.send(JSON.stringify({ type: 'end' }));
                    if (old.readyState === 0 || old.readyState === 1) old.close();
                } catch (error) { /* already closed */ }
            }

            async function explainFailure(target, confirmed, mine) {
                const info = typeof diagnose === 'function' ? await diagnose(diagnosePath(config.hostId, target.ref, confirmed)) : null;
                if (disposed || generation !== mine) return;
                if (info && info.code === 'container_protected_confirmation_required' && !confirmed) {
                    if (typeof confirmProtected === 'function' && await confirmProtected(target, info) && !disposed && generation === mine) { void connect(target.ref, true); return; }
                    phase = 'idle';
                    message = '';
                } else {
                    phase = 'error';
                    message = info ? t('term_failed', { message: views.errorText(info) }) : t('term_failed_unknown');
                }
                paint();
            }

            // confirmed = the person said yes to the protected container already (the request then carries ?confirm=protected).
            async function connect(ref, confirmed) {
                if (disposed) return;
                closeSocket(true);
                current = ref;
                const target = targetFor(ref);
                const blocked = blockedBy(target, readonly());
                if (blocked) { phase = 'idle'; message = t(blocked); paintPicker(); paint(); return; }
                if (!ensureTerm()) { phase = 'error'; message = t('term_unavailable'); paintPicker(); paint(); return; }
                if (target.protected && !confirmed) {
                    phase = 'idle';
                    message = '';
                    paintPicker();
                    paint();
                    const before = generation;
                    if (typeof confirmProtected !== 'function' || !await confirmProtected(target, null) || disposed || current !== ref || generation !== before) return;
                    confirmed = true;
                }
                const mine = generation;
                phase = 'connecting';
                message = '';
                opened = false;
                term.reset();
                paintPicker();
                paint();
                let next;
                try { next = openSocket(socketUrl(where, config.hostId, ref, confirmed)); } catch (error) { phase = 'error'; message = t('term_failed_unknown'); paint(); return; }
                socket = next;
                next.binaryType = 'arraybuffer';
                next.onopen = () => {
                    if (generation !== mine || disposed) return;
                    opened = true;
                    phase = 'connected';
                    paint();
                    frame(() => { doFit(); if (wantFocus && active && term) { term.focus(); wantFocus = false; } });
                };
                next.onmessage = event => {
                    if (generation !== mine || disposed || !term) return;
                    term.write(typeof event.data === 'string' ? event.data : new Uint8Array(event.data));
                };
                next.onerror = () => { /* onclose follows and tells what happened */ };
                next.onclose = () => {
                    if (generation !== mine || disposed) return;
                    socket = null;
                    if (!opened) { phase = 'connecting'; paint(); void explainFailure(target, !!confirmed, mine); return; }
                    phase = 'closed';
                    term.write('\r\n[' + t('term_closed_line') + ']\r\n');
                    paint();
                };
            }

            function leave() {
                leaveEl.focus({ preventScroll: true });
            }

            // ---- events --------------------------------------------------------------------------------------------------

            leaveEl.addEventListener('click', leave, { signal });
            startEl.addEventListener('click', () => { wantFocus = true; void connect(current || (targets()[0] && targets()[0].ref) || '', false); }, { signal });
            pickEl.addEventListener('change', () => { wantFocus = false; void connect(pickEl.value, false); }, { signal });

            // First use: pick the first target. Whatever is possible starts at once; the rest explains itself.
            function start() {
                const list = targets();
                if (!current || !targetFor(current)) current = list.length ? list[0].ref : '';
                paintPicker();
                if (!current) { message = t(blockedBy(null, readonly())); paint(); return; }
                void connect(current, false);
            }
            paint();

            return {
                setActive(next) {
                    if (disposed || next === active) return;
                    active = !!next;
                    if (!active) return;
                    if (phase === 'idle' && !message && !opened) start();
                    else { queueFit(); frame(() => { doFit(); if (wantFocus && phase === 'connected' && term) { term.focus(); wantFocus = false; } }); }
                },
                // The list of containers changed: refresh the picker and the explanation; a running session is left alone, unless read-only is on.
                update() {
                    if (disposed) return;
                    // Read-only can be switched on while a shell is open (desktop policy, docker_readonly of the host list, a 403 read_only):
                    // the session ends at once, with the "end" frame, instead of staying usable until the person leaves.
                    if (readonly() && (phase === 'connecting' || phase === 'connected')) {
                        closeSocket(true);
                        phase = 'idle';
                        message = t('term_readonly');
                        paint();
                        return;
                    }
                    if ((phase === 'idle' || phase === 'error') && !targetFor(current) && targets().length) { current = targets()[0].ref; optionSig = ''; }
                    paintPicker();
                    if (phase === 'idle' || phase === 'error') {
                        const blocked = blockedBy(targetFor(current), readonly());
                        const text = blocked ? t(blocked) : phase === 'idle' ? '' : message;
                        if (text !== message) { message = text; paint(); }
                    }
                },
                focus() { if (term && phase === 'connected') term.focus(); else wantFocus = true; },
                phase: () => phase,
                isActive: () => active,
                dispose() {
                    if (disposed) return;
                    disposed = true;
                    closeSocket(true);
                    lifetime.abort();
                    resizeObserver?.disconnect();
                    try { term?.dispose(); } catch (error) { /* already disposed */ }
                    term = null;
                    el.replaceChildren();
                }
            };
        }

        return { open };
    }

    window.DockerManagerTerminal = { create, socketUrl, diagnosePath, targetOf, stackTargets, blockedBy, readTheme };
})();
