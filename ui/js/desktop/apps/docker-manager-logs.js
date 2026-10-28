/* Docker Manager logs: the follow panel of the container drawer, the stack pane and the external project pane. A classic script
   (window.DockerManagerLogs) with no DOM work at load.
   - createModel: the pure part. A ring of 5000 lines (older ones fall off), a filter by text and by service, the follow flag with the
     count of lines that arrived while it was off, the stderr flag, ANSI codes and carriage-return progress output removed, and what a
     reconnect needs: the time of the newest line (`since`) and a duplicate filter for the lines the server repeats.
   - followUrl: the URL of the three SSE routes (container, stack, external project).
   - create(deps).open(el, target, options): the panel. One EventSource per open panel; it follows only while the panel is active
     (the tab is shown), reconnects with backoff and `since` after a broken connection, shows the notices and the end message of the
     server, and appends the lines to the DOM in animation frames. Lines are built with the DOM API (textContent), never as HTML. */
(function () {
    'use strict';

    const BASE = '/api/docker-manager';
    const TAILS = [100, 200, 500, 1000, 5000];
    const DEFAULT_TAIL = { container: 200, stack: 100, project: 100 };
    const MAX_LINES = 5000;
    const RECONNECT_MS = [1000, 2000, 4000, 8000, 15000];
    // After `end` with these codes the host may come back: follow again by itself. Every other end waits for the Reconnect button.
    const RETRY_CODES = ['host_unreachable', 'timeout', 'docker_error', 'internal_error', 'network'];
    // A 4xx answer to the diagnosis ends the retries whatever its code; these codes do the same for the 5xx answers that mean "not now".
    const FINAL_CODES = ['docker_disabled', 'docker_hosts_unavailable', 'stacks_unavailable'];
    const ANSI = /\x1b\[[0-9;?]*[ -/]*[@-~]/g;
    const HUES = 6;
    const enc = encodeURIComponent;

    // ---- the pure part -------------------------------------------------------------------------------------------------

    // The text of one log line for the screen: colour codes out, and a carriage-return progress bar reduced to its last state.
    function cleanText(value) {
        let text = String(value == null ? '' : value).replace(ANSI, '');
        if (text.includes('\r')) {
            const parts = text.split('\r').filter(part => part !== '');
            text = parts.length ? parts[parts.length - 1] : '';
        }
        return text;
    }

    const hueOf = name => {
        let hash = 0;
        for (const ch of String(name || '')) hash = (hash * 31 + ch.charCodeAt(0)) % 9973;
        return hash % HUES;
    };

    function entryOf(raw, id) {
        const r = raw && typeof raw === 'object' ? raw : {};
        const t = String(r.t || ''), ms = Date.parse(t);
        const entry = { id, t, ms: Number.isFinite(ms) ? ms : 0, s: r.s === 'err' ? 'err' : 'out', text: cleanText(r.text), trunc: !!r.trunc, svc: String(r.svc || ''), ctr: String(r.ctr || ''), el: null };
        entry.key = entry.ctr + '|' + entry.svc + '|' + entry.t + '|' + entry.text;
        return entry;
    }

    function createModel(options) {
        const max = options && options.max > 0 ? Math.floor(options.max) : MAX_LINES;
        let lines = [], seq = 0, query = '', follow = true, unseen = 0, lastMs = 0, lastT = '', lastKeys = new Set(), resume = null;
        const hidden = new Set(), services = [];

        const matches = entry => !hidden.has(entry.svc) && (!query || entry.text.toLowerCase().includes(query) || entry.svc.toLowerCase().includes(query));

        // Adds the lines of one batch. Returns {added: entries kept, shown: how many of them pass the filter, dropped: entries pushed out
        // of the ring}. After beginResume() the lines a reconnect repeats (older than the newest line, or the same line again) are skipped.
        function append(raws) {
            const added = [];
            for (const raw of Array.isArray(raws) ? raws : []) {
                const entry = entryOf(raw, ++seq);
                if (resume) {
                    if (entry.ms && entry.ms < resume.ms) continue;
                    if (entry.ms === resume.ms && resume.keys.has(entry.key)) continue;
                    resume = null;
                }
                if (entry.ms > lastMs) { lastMs = entry.ms; lastT = entry.t; lastKeys = new Set([entry.key]); }
                else if (entry.ms === lastMs) { if (entry.t) lastT = entry.t; lastKeys.add(entry.key); }
                if (entry.svc && !services.includes(entry.svc)) services.push(entry.svc);
                lines.push(entry);
                added.push(entry);
            }
            const dropped = lines.length > max ? lines.splice(0, lines.length - max) : [];
            return { added, shown: added.filter(matches).length, dropped };
        }

        return {
            max, append, matches,
            visible: () => lines.filter(matches),
            all: () => lines.slice(),
            count: () => lines.length,
            // Forget the lines on screen but keep where the stream stood, so a reconnect does not bring them back.
            clear() { lines = []; unseen = 0; },
            // Forget everything, including the resume point (another tail was chosen: the stream starts from scratch).
            reset() { lines = []; unseen = 0; lastMs = 0; lastT = ''; lastKeys = new Set(); resume = null; services.length = 0; },
            beginResume() { resume = lastMs ? { ms: lastMs, keys: new Set(lastKeys) } : null; },
            lastTime: () => lastT,
            setQuery(value) { query = String(value || '').trim().toLowerCase(); },
            toggleService(name) { if (hidden.has(name)) hidden.delete(name); else hidden.add(name); return !hidden.has(name); },
            serviceShown: name => !hidden.has(name),
            addService(name) { if (name && !services.includes(name)) services.push(name); },
            services: () => services.slice(),
            isFollowing: () => follow,
            setFollow(value) { follow = !!value; if (follow) unseen = 0; },
            addUnseen(count) { unseen += count; },
            unseen: () => unseen,
            // The visible lines as text for the clipboard; with timestamps and service names when asked for.
            text(o) {
                const withTime = !!(o && o.timestamps);
                return lines.filter(matches).map(entry => (withTime && entry.t ? entry.t + ' ' : '') + (entry.svc ? '[' + entry.svc + '] ' : '') + entry.text).join('\n');
            }
        };
    }

    // target = {kind: 'container', hostId, ref} | {kind: 'stack', hostId, name} | {kind: 'project', hostId, name}. A reconnect asks for
    // everything since the newest line it has (up to the ring size) instead of a tail.
    function followUrl(target, o) {
        const opt = o || {}, host = BASE + '/hosts/' + enc(target.hostId);
        let path;
        if (target.kind === 'stack') path = host + '/stacks/' + enc(target.name) + '/logs';
        else if (target.kind === 'project') path = host + '/projects/' + enc(target.name) + '/logs';
        else path = host + '/containers/' + enc(target.ref) + '/logs';
        const query = ['follow=1', 'tail=' + (opt.since ? MAX_LINES : TAILS.includes(opt.tail) ? opt.tail : DEFAULT_TAIL[target.kind] || 200)];
        if (opt.since) query.push('since=' + enc(opt.since));
        return path + '?' + query.join('&');
    }

    // The diagnosis of a failed stream request (store.diagnose: the error info with its HTTP status) is final for any 4xx answer and for
    // the codes in FINAL_CODES; a 5xx answer, a timeout and a network failure are tried again.
    const isFinalDiagnosis = info => !!info && ((info.status >= 400 && info.status < 500) || FINAL_CODES.includes(info.code));

    // What a notice talks about: its services, else its container. attached and detached name the first service, else the container.
    const noticeNames = notice => (Array.isArray(notice.services) && notice.services.length ? notice.services.map(String) : notice.container ? [String(notice.container)] : []);
    const noticeName = notice => (Array.isArray(notice.services) && notice.services.length ? String(notice.services[0]) : String(notice.container || ''));

    // Adds a notice to the list (at most 4 entries). The server sends one not_followed notice for every container whose history or follow
    // failed and one for the services the stream budget left out: all of them become one entry (at the place of the first) with the
    // services of every notice, each once, and the first message as detail.
    function mergeNotice(list, notice) {
        const n = notice && typeof notice === 'object' ? notice : {};
        if (n.code !== 'not_followed') return list.concat([n]).slice(-4);
        const existing = list.find(item => item.code === 'not_followed');
        const services = [...new Set((existing ? existing.services : []).concat(noticeNames(n)))];
        const merged = { code: 'not_followed', message: existing && existing.message ? existing.message : String(n.message || ''), services };
        return (existing ? list.map(item => (item === existing ? merged : item)) : list.concat([merged])).slice(-4);
    }

    // ---- the panel -----------------------------------------------------------------------------------------------------

    function create(deps) {
        const { t, esc, views, copy, diagnose } = deps;
        const openSource = deps.eventSource;
        const setT = typeof deps.setTimeout === 'function' ? deps.setTimeout : (fn, ms) => setTimeout(fn, ms);
        const clearT = typeof deps.clearTimeout === 'function' ? deps.clearTimeout : id => clearTimeout(id);
        const frame = typeof deps.requestFrame === 'function' ? deps.requestFrame : fn => (typeof requestAnimationFrame === 'function' ? requestAnimationFrame(fn) : setT(fn, 16));
        const lang = deps.lang || 'en';

        function open(el, target, options) {
            const opt = options || {};
            const model = createModel(), lifetime = new AbortController(), signal = lifetime.signal;
            const grouped = target.kind !== 'container';
            let tail = TAILS.includes(opt.tail) ? opt.tail : DEFAULT_TAIL[target.kind] || 200;
            let source = null, active = false, disposed = false, phase = 'idle', failures = 0, retryTimer = 0, opened = false, ended = null, flushing = false;
            let pending = [], notices = [], timestamps = false, chipSig = '', searchTimer = 0, finalError = null, started = false, lastUrl = '';
            const knownServices = new Set(Array.isArray(target.services) ? target.services : []);

            const pad = (n, size) => String(n).padStart(size || 2, '0');
            const clock = ms => { const d = new Date(ms); return pad(d.getHours()) + ':' + pad(d.getMinutes()) + ':' + pad(d.getSeconds()) + '.' + pad(d.getMilliseconds(), 3); };

            el.innerHTML = '<div class="dm-lp" data-logs data-ts="off" data-wrap="on">'
                + '<div class="dm-lp-bar"><label class="dm-inline"><span>' + esc(t('logs_tail')) + '</span><select data-lp-tail' + (grouped ? ' title="' + esc(t('logs_tail_each')) + '"' : '') + '>'
                + TAILS.map(n => '<option value="' + n + '"' + (n === tail ? ' selected' : '') + '>' + n + '</option>').join('') + '</select></label>'
                + '<input type="search" class="dm-lp-search" data-lp-search autocomplete="off" spellcheck="false" placeholder="' + esc(t('logs_search')) + '" aria-label="' + esc(t('logs_search')) + '">'
                + '<button type="button" class="dm-btn dm-toggle" data-lp-follow aria-pressed="true"><span>' + esc(t('logs_follow')) + '</span></button>'
                + '<button type="button" class="dm-btn dm-toggle" data-lp-ts aria-pressed="false"><span>' + esc(t('logs_timestamps')) + '</span></button>'
                + '<button type="button" class="dm-btn dm-toggle" data-lp-wrap aria-pressed="true"><span>' + esc(t('logs_wrap')) + '</span></button>'
                + '<button type="button" class="dm-btn" data-lp-copy><span>' + esc(t('logs_copy')) + '</span></button>'
                + '<button type="button" class="dm-btn" data-lp-clear><span>' + esc(t('logs_clear')) + '</span></button>'
                + '<span class="dm-lp-state" data-lp-state role="status"></span>'
                + '<button type="button" class="dm-btn" data-lp-reconnect hidden><span>' + esc(t('logs_reconnect')) + '</span></button></div>'
                + '<div class="dm-lp-chips" data-lp-chips role="group" aria-label="' + esc(t('overview_services')) + '" hidden></div>'
                + '<div class="dm-lp-notices" data-lp-notices></div>'
                + '<div class="dm-lp-body"><div class="dm-lp-view" data-lp-view tabindex="0" role="log" aria-live="off" aria-label="' + esc(t('logs_label', { name: target.label || target.name || target.ref || '' })) + '"></div>'
                + '<button type="button" class="dm-btn is-primary dm-lp-new" data-lp-new hidden></button><p class="dm-lp-empty" data-lp-empty></p></div></div>';
            const q = selector => el.querySelector(selector);
            const root = q('[data-logs]'), view = q('[data-lp-view]'), stateEl = q('[data-lp-state]'), emptyEl = q('[data-lp-empty]'), newEl = q('[data-lp-new]');
            const chipsEl = q('[data-lp-chips]'), noticesEl = q('[data-lp-notices]'), reconnectEl = q('[data-lp-reconnect]'), followEl = q('[data-lp-follow]'), searchEl = q('[data-lp-search]');

            // ---- painting ------------------------------------------------------------------------------------------------

            function lineEl(entry) {
                const row = document.createElement('div');
                row.className = 'dm-ll';
                if (entry.s === 'err') {
                    row.dataset.s = 'err';
                    const label = document.createElement('span');
                    label.className = 'dm-sr-only';
                    label.textContent = t('logs_stderr') + ' ';
                    row.append(label);
                }
                if (entry.ms) {
                    const time = document.createElement('time');
                    time.className = 'dm-ll-ts';
                    time.dateTime = entry.t;
                    time.title = entry.t;
                    time.textContent = clock(entry.ms);
                    row.append(time);
                }
                if (entry.svc) {
                    const svc = document.createElement('span');
                    svc.className = 'dm-ll-svc';
                    svc.dataset.h = String(hueOf(entry.svc));
                    svc.textContent = entry.svc;
                    row.append(svc);
                }
                const text = document.createElement('span');
                text.className = 'dm-ll-text';
                text.textContent = entry.text;
                row.append(text);
                if (entry.trunc) {
                    const cut = document.createElement('span');
                    cut.className = 'dm-ll-cut';
                    cut.title = t('logs_truncated');
                    cut.textContent = '…';
                    row.append(cut);
                }
                entry.el = row;
                return row;
            }
            const nearBottom = () => view.scrollHeight - view.scrollTop - view.clientHeight < 24;
            const toBottom = () => { view.scrollTop = view.scrollHeight; };

            function paintState() {
                let text = t('logs_state_' + phase);
                if (phase === 'ended' && ended && ended.reason === 'error') text = t('logs_end_error', { message: views.errorText({ code: ended.code, message: ended.message }) });
                else if (phase === 'error' && finalError) text = t('logs_end_error', { message: views.errorText(finalError) });
                stateEl.textContent = text;
                stateEl.dataset.phase = phase;
                reconnectEl.hidden = !(phase === 'ended' || phase === 'error');
                followEl.setAttribute('aria-pressed', String(model.isFollowing()));
                const waiting = model.visible().length === 0;
                emptyEl.hidden = !waiting;
                emptyEl.textContent = waiting ? (model.count() ? t('logs_no_match') : phase === 'live' || phase === 'connecting' ? t('logs_empty_live') : '') : '';
                const unseen = model.unseen();
                newEl.hidden = model.isFollowing() || unseen === 0;
                newEl.textContent = unseen ? t('logs_new', { count: unseen }) : '';
            }

            function paintChips() {
                if (!grouped) return;
                for (const name of model.services()) knownServices.add(name);
                const names = [...knownServices].sort((a, b) => a.localeCompare(b, lang, { numeric: true }));
                const sig = names.join('\n') + '|' + names.filter(name => !model.serviceShown(name)).join('\n');
                if (sig === chipSig) return;
                chipSig = sig;
                chipsEl.hidden = names.length < 2;
                chipsEl.innerHTML = names.map(name => '<button type="button" class="dm-chipbtn" data-lp-chip="' + esc(name) + '" aria-pressed="' + model.serviceShown(name) + '"><span class="dm-ll-svc" data-h="' + hueOf(name) + '">' + esc(name) + '</span></button>').join('');
            }

            function noticeText(notice) {
                switch (notice.code) {
                    case 'not_followed': return t('logs_notice_not_followed', { services: noticeNames(notice).join(', ') });
                    case 'attached': return t('logs_notice_attached', { name: noticeName(notice) });
                    case 'detached': return t('logs_notice_detached', { name: noticeName(notice) });
                    case 'truncated_history': return t('logs_notice_truncated_history');
                    default: return String(notice.message || '');
                }
            }
            function paintNotices() {
                noticesEl.replaceChildren(...notices.map(notice => {
                    const p = document.createElement('p');
                    p.className = 'dm-lp-notice';
                    p.dataset.code = notice.code;
                    p.textContent = noticeText(notice);
                    p.title = String(notice.message || '');
                    return p;
                }));
            }

            // Appends the pending lines in one go: the rows, the scroll position (follow) or the "n new" counter (paused).
            function flush() {
                flushing = false;
                if (disposed || !pending.length) return;
                const batch = pending;
                pending = [];
                const result = model.append(batch);
                for (const entry of result.dropped) { entry.el?.remove(); entry.el = null; }
                const fragment = document.createDocumentFragment();
                for (const entry of result.added) if (model.matches(entry)) fragment.append(lineEl(entry));
                const following = model.isFollowing();
                view.append(fragment);
                if (following) toBottom(); else if (result.shown) { model.addUnseen(result.shown); }
                paintChips();
                paintState();
            }
            function queueFlush() {
                if (flushing) return;
                flushing = true;
                frame(flush);
            }

            // Rebuilds the rows from the model (filter, clear).
            function repaintAll() {
                view.replaceChildren();
                const fragment = document.createDocumentFragment();
                for (const entry of model.visible()) fragment.append(lineEl(entry));
                view.append(fragment);
                if (model.isFollowing()) toBottom();
                paintChips();
                paintState();
            }

            // ---- connection ----------------------------------------------------------------------------------------------

            function closeSource() {
                if (!source) return;
                const old = source;
                source = null;
                try { old.close(); } catch (error) { /* already closed */ }
            }
            function setPhase(next) { phase = next; paintState(); }

            function schedule(delay) {
                clearT(retryTimer);
                retryTimer = setT(() => { retryTimer = 0; connect(true); }, delay);
            }

            async function failedBeforeOpen() {
                // An EventSource does not tell why a request failed. Ask for the same URL as a plain request: the JSON error of the
                // server (stack_not_found, too_many_streams, ...) ends the retries; anything else (a down server) is retried.
                const mine = source;
                let info = null;
                if (typeof diagnose === 'function') {
                    try { info = await diagnose(lastUrl.slice(BASE.length)); } catch (error) { info = null; }
                }
                if (disposed || !active || source !== mine) return;
                if (isFinalDiagnosis(info)) { closeSource(); finalError = info; setPhase('error'); return; }
                retry();
            }

            function retry() {
                closeSource();
                if (disposed || !active) return;
                setPhase('reconnecting');
                schedule(RECONNECT_MS[Math.min(failures, RECONNECT_MS.length - 1)]);
                failures += 1;
            }

            // fresh = start from the tail (first connect, another tail); otherwise resume after the newest line.
            function connect(resuming) {
                if (disposed || !active || typeof openSource !== 'function') return;
                closeSource();
                clearT(retryTimer);
                retryTimer = 0;
                ended = null;
                finalError = null;
                opened = false;
                const resume = !!resuming && !!model.lastTime();
                if (resume) model.beginResume();
                lastUrl = followUrl(target, resume ? { since: model.lastTime() } : { tail });
                let next;
                try { next = openSource(lastUrl); } catch (error) { retry(); return; }
                source = next;
                started = true;
                setPhase(phase === 'live' ? 'live' : failures ? 'reconnecting' : 'connecting');
                const on = (type, handler) => next.addEventListener(type, event => {
                    if (source !== next || disposed) return;
                    let data = null;
                    try { data = JSON.parse(event && event.data); } catch (error) { data = null; }
                    if (!data || typeof data !== 'object') return;
                    if (!opened) { opened = true; failures = 0; if (phase !== 'live') setPhase('live'); }
                    handler(data);
                });
                next.onopen = () => { if (source !== next) return; opened = true; failures = 0; setPhase('live'); };
                on('lines', data => { if (Array.isArray(data.lines)) { pending.push(...data.lines); queueFlush(); } });
                on('notice', data => {
                    notices = mergeNotice(notices, data);
                    paintNotices();
                });
                on('end', data => {
                    ended = { reason: String(data.reason || 'ended'), code: String(data.code || ''), message: String(data.message || '') };
                    closeSource();
                    flush();
                    setPhase('ended');
                    if (ended.reason === 'error' && RETRY_CODES.includes(ended.code) && failures < RECONNECT_MS.length) { failures += 1; schedule(RECONNECT_MS[failures - 1]); }
                });
                next.onerror = () => {
                    if (source !== next || disposed) return;
                    if (opened) { retry(); return; }
                    void failedBeforeOpen();
                };
            }

            // ---- controls ------------------------------------------------------------------------------------------------

            function setFollow(value) {
                model.setFollow(value);
                if (model.isFollowing()) toBottom();
                paintState();
            }
            view.addEventListener('scroll', () => {
                const bottom = nearBottom();
                if (bottom && !model.isFollowing()) { model.setFollow(true); paintState(); } else if (!bottom && model.isFollowing()) { model.setFollow(false); paintState(); }
            }, { signal, passive: true });
            el.addEventListener('click', event => {
                const button = event.target.closest('button');
                if (!button || !el.contains(button)) return;
                if (button.hasAttribute('data-lp-follow')) setFollow(!model.isFollowing());
                else if (button.hasAttribute('data-lp-new')) { setFollow(true); view.focus({ preventScroll: true }); }
                else if (button.hasAttribute('data-lp-ts')) {
                    timestamps = !timestamps;
                    root.dataset.ts = timestamps ? 'on' : 'off';
                    button.setAttribute('aria-pressed', String(timestamps));
                } else if (button.hasAttribute('data-lp-wrap')) {
                    const on = root.dataset.wrap !== 'on';
                    root.dataset.wrap = on ? 'on' : 'off';
                    button.setAttribute('aria-pressed', String(on));
                } else if (button.hasAttribute('data-lp-copy')) copy?.(model.text({ timestamps }));
                else if (button.hasAttribute('data-lp-clear')) { model.clear(); repaintAll(); } else if (button.hasAttribute('data-lp-reconnect')) { failures = 0; connect(true); } else if (button.hasAttribute('data-lp-chip')) {
                    model.toggleService(button.dataset.lpChip);
                    chipSig = '';
                    repaintAll();
                }
            }, { signal });
            q('[data-lp-tail]').addEventListener('change', event => {
                const next = Number(event.target.value);
                if (!TAILS.includes(next) || next === tail) return;
                tail = next;
                model.reset();
                notices = [];
                paintNotices();
                pending = [];
                chipSig = '';
                failures = 0;
                repaintAll();
                connect(false);
            }, { signal });
            searchEl.addEventListener('input', () => {
                clearT(searchTimer);
                searchTimer = setT(() => { model.setQuery(searchEl.value); repaintAll(); }, 150);
            }, { signal });
            // Escape clears the filter first; only an empty field lets the app close the drawer.
            searchEl.addEventListener('keydown', event => {
                if (event.key !== 'Escape' || !searchEl.value) return;
                event.preventDefault();
                event.stopPropagation();
                searchEl.value = '';
                model.setQuery('');
                repaintAll();
            }, { signal });

            paintState();
            paintChips();

            return {
                // Follow while the panel is shown; leaving the tab closes the stream (the lines stay) and coming back resumes it.
                setActive(next) {
                    if (disposed || next === active) return;
                    active = !!next;
                    if (active) {
                        failures = 0;
                        connect(started);
                        if (model.isFollowing()) frame(toBottom);
                    } else {
                        clearT(retryTimer);
                        retryTimer = 0;
                        closeSource();
                        phase = 'idle';
                        paintState();
                    }
                },
                // The service chips of a stack follow its service list.
                update(next) {
                    if (disposed || !next) return;
                    for (const name of Array.isArray(next.services) ? next.services : []) knownServices.add(name);
                    paintChips();
                },
                focus() { view.focus({ preventScroll: true }); },
                model,
                isActive: () => active,
                phase: () => phase,
                dispose() {
                    if (disposed) return;
                    disposed = true;
                    clearT(retryTimer);
                    clearT(searchTimer);
                    closeSource();
                    lifetime.abort();
                    pending = [];
                    el.replaceChildren();
                }
            };
        }

        return { open };
    }

    window.DockerManagerLogs = { create, createModel, followUrl, mergeNotice, noticeName, isFinalDiagnosis, cleanText, hueOf, TAILS, DEFAULT_TAIL, MAX_LINES, RECONNECT_MS };
})();
