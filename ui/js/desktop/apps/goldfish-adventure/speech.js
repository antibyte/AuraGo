/* Spoken lines above the speaker's head: auto-advancing queues, speech log and reading preferences. */
(function () {
    'use strict';

    const PREF_KEY = 'aurago.goldfish.speech';
    const RATES = Object.freeze({ slow: 70, normal: 48, fast: 32 });
    const LOG_LIMIT = 40;
    const MARGIN = 8;

    // clamp(900 + characters x rate, 1600, 9000) milliseconds.
    const duration = (text, speed) => Math.max(1600, Math.min(9000, 900 + String(text).length * (RATES[speed] || RATES.normal)));

    function readPrefs() {
        const prefs = { autoAdvance: true, readSpeed: 'normal' };
        try {
            const stored = JSON.parse(window.localStorage.getItem(PREF_KEY) || 'null');
            if (stored && typeof stored.autoAdvance === 'boolean') prefs.autoAdvance = stored.autoAdvance;
            if (stored && Object.hasOwn(RATES, stored.readSpeed)) prefs.readSpeed = stored.readSpeed;
        } catch (_) { /* Preferences are optional; the defaults apply. */ }
        return prefs;
    }

    function writePrefs(prefs) {
        try { window.localStorage.setItem(PREF_KEY, JSON.stringify(prefs)); } catch (_) { /* Not persisted. */ }
    }

    function create(options) {
        const { scene, t, anchor, live, onRead, onEnd, onLine } = options;
        let prefs = readPrefs(), queue = [], index = -1, meta = null, line = null;
        let timer = 0, remaining = 0, startedAt = 0, paused = false, disposed = false, frame = 0;
        const entries = [];

        const box = document.createElement('div');
        box.className = 'agfish-speech';
        box.hidden = true;
        box.setAttribute('aria-hidden', 'true');
        const speaker = document.createElement('span');
        speaker.className = 'agfish-speaker';
        const text = document.createElement('p');
        text.className = 'agfish-line';
        const marker = document.createElement('span');
        marker.className = 'agfish-next-marker';
        marker.textContent = '▶';
        box.append(speaker, text, marker);
        scene.append(box);

        const speakerName = id => t('goldfish.ui.speaker_' + id);

        function stopTimer() {
            if (timer) clearTimeout(timer);
            timer = 0;
        }

        function startTimer() {
            if (timer || !line || paused || disposed || !prefs.autoAdvance) return;
            startedAt = performance.now();
            timer = setTimeout(() => { timer = 0; remaining = 0; advance(true); }, remaining);
        }

        function holdTimer() {
            if (!timer) return;
            clearTimeout(timer);
            timer = 0;
            remaining = Math.max(0, remaining - (performance.now() - startedAt));
        }

        function place() {
            frame = 0;
            if (disposed || !line || box.hidden) return;
            const bounds = scene.getBoundingClientRect();
            if (!bounds.width || !bounds.height) return;
            const point = anchor ? anchor(line.speaker) : null;
            box.classList.toggle('agfish-speech-top', !point);
            box.style.width = '';
            box.style.left = '0px';
            box.style.top = '0px';
            // Shrink the box to the widest rendered line so centring and clamping use the visible text.
            const range = document.createRange();
            range.selectNodeContents(text);
            const widest = Math.ceil(Math.max(range.getBoundingClientRect().width, speaker.offsetWidth)) + 2;
            if (widest < box.offsetWidth) box.style.width = widest + 'px';
            const width = box.offsetWidth, height = box.offsetHeight;
            const centre = point ? point.x * bounds.width : bounds.width / 2;
            const bottom = point ? point.y * bounds.height - MARGIN : MARGIN + height;
            const left = Math.max(MARGIN, Math.min(bounds.width - width - MARGIN, centre - width / 2));
            const top = Math.max(MARGIN, Math.min(bounds.height - height - MARGIN, bottom - height));
            box.style.left = Math.round(left) + 'px';
            box.style.top = Math.round(top) + 'px';
        }

        function schedulePlace() {
            if (!frame && !disposed) frame = requestAnimationFrame(place);
        }

        const observer = new ResizeObserver(entries => { if (entries.some(entry => entry.target === scene)) schedulePlace(); });
        observer.observe(scene);
        // Webfont swaps change the line breaks after the first placement.
        if (document.fonts?.addEventListener) document.fonts.addEventListener('loadingdone', schedulePlace);

        function show(next) {
            line = next;
            const message = t(next.text), name = speakerName(next.speaker);
            box.dataset.speaker = next.speaker;
            speaker.textContent = name;
            text.textContent = message;
            marker.hidden = prefs.autoAdvance;
            box.hidden = false;
            entries.push({ speaker: next.speaker, name, key: next.text, text: message });
            if (entries.length > LOG_LIMIT) entries.shift();
            if (live) live.textContent = name + ': ' + message;
            place();
            remaining = duration(message, prefs.readSpeed);
            onLine?.(next);
            startTimer();
        }

        function finish(cancelled) {
            const info = meta && { cancelled, story: meta.story, blocking: meta.blocking, allSeen: meta.allSeen };
            stopTimer();
            line = null;
            queue = [];
            index = -1;
            meta = null;
            box.hidden = true;
            if (info) {
                onLine?.(null);
                onEnd?.(info);
            }
        }

        function advance(read) {
            if (!line) return false;
            const current = line;
            stopTimer();
            if (read) onRead?.(current, meta);
            if (disposed || line !== current) return true;
            index++;
            if (index >= queue.length) finish(false);
            else show(queue[index]);
            return true;
        }

        // A story queue blocks the scene until it ends unless every line was already read.
        function play(lines, settings = {}) {
            if (disposed) return false;
            finish(true);
            const list = Array.isArray(lines) ? lines.filter(value => value && typeof value.text === 'string') : [];
            if (!list.length) return false;
            const known = typeof settings.seen === 'function' ? list.every(settings.seen) : false;
            const allSeen = settings.allSeen ?? known;
            queue = list;
            index = 0;
            meta = { story: !!settings.story, allSeen: !!allSeen, blocking: !!settings.story && !allSeen };
            show(queue[0]);
            return true;
        }

        return {
            play,
            skipLine: () => advance(true),
            skipAll: () => { if (!line) return false; finish(true); return true; },
            cancel: () => { if (!line) return false; finish(true); return true; },
            setPaused: value => {
                paused = !!value;
                box.classList.toggle('agfish-speech-paused', paused);
                if (paused) holdTimer(); else startTimer();
            },
            active: () => !!line,
            story: () => !!line && !!meta && meta.blocking,
            log: () => entries.map(value => ({ ...value })),
            prefs: () => ({ ...prefs }),
            setPrefs: next => {
                const merged = { ...prefs };
                if (next && typeof next.autoAdvance === 'boolean') merged.autoAdvance = next.autoAdvance;
                if (next && Object.hasOwn(RATES, next.readSpeed)) merged.readSpeed = next.readSpeed;
                prefs = merged;
                writePrefs(prefs);
                marker.hidden = prefs.autoAdvance;
                if (line) {
                    if (!prefs.autoAdvance) holdTimer();
                    else { remaining = Math.max(remaining, 400); startTimer(); }
                }
                return { ...prefs };
            },
            reposition: schedulePlace,
            dispose: () => {
                disposed = true;
                stopTimer();
                observer.disconnect();
                document.fonts?.removeEventListener?.('loadingdone', schedulePlace);
                if (frame) cancelAnimationFrame(frame);
                frame = 0;
                line = null;
                queue = [];
                box.remove();
            }
        };
    }

    window.GoldfishSpeech = Object.freeze({ create, duration, RATES });
}());
