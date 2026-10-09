(function () {
    'use strict';

    const BAUD_KEY = 'aurago.desktop.terminal.baud';
    const BAUD_RATES = [0, 300, 1200, 2400, 9600, 14400];
    const TYPE_MS = 45;
    const TONE_MS = 90;
    const GAP_MS = 70;
    const HANDSHAKE_MS = 2500;
    const MAX_QUEUE_BYTES = 1 << 20;
    const DTMF = {
        '1': [697, 1209], '2': [697, 1336], '3': [697, 1477],
        '4': [770, 1209], '5': [770, 1336], '6': [770, 1477],
        '7': [852, 1209], '8': [852, 1336], '9': [852, 1477],
        '0': [941, 1336]
    };
    // Answer tone, then band-limited noise bursts in the spirit of V.8/V.32 training.
    const HANDSHAKE = [
        { at: 0, duration: 0.85, freq: 2100, gain: 0.035 },
        { at: 0.9, duration: 0.25, freq: 1200, q: 4, gain: 0.08 },
        { at: 1.2, duration: 0.25, freq: 2400, q: 4, gain: 0.08 },
        { at: 1.5, duration: 0.2, freq: 1800, q: 1.5, gain: 0.07 },
        { at: 1.75, duration: 0.7, freq: 1800, q: 0.7, gain: 0.06 }
    ];

    function normalizeBaud(value) {
        const rate = Number(value);
        return BAUD_RATES.indexOf(rate) >= 0 ? rate : 0;
    }

    function loadBaud() {
        try {
            return normalizeBaud(window.localStorage.getItem(BAUD_KEY));
        } catch (e) {
            return 0;
        }
    }

    function saveBaud(value) {
        const next = normalizeBaud(value);
        try {
            window.localStorage.setItem(BAUD_KEY, String(next));
        } catch (e) {}
        return next;
    }

    function reducedMotion() {
        return (document.body && document.body.dataset.animations === 'false')
            || !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    // Same silence rule as the key clicks: muted flag, reduced motion, disabled animations.
    function silenced() {
        if (window.TerminalAudio && typeof window.TerminalAudio.shouldSilence === 'function') {
            return !!window.TerminalAudio.shouldSilence();
        }
        return reducedMotion();
    }

    // Deterministic 7-digit pseudo number per entry (FNV-1a), never starting with 0.
    function pseudoNumber(entryId) {
        let hash = 0x811c9dc5;
        const text = String(entryId || '');
        for (let i = 0; i < text.length; i += 1) {
            hash ^= text.charCodeAt(i);
            hash = Math.imul(hash, 0x01000193) >>> 0;
        }
        return String(1000000 + (hash % 9000000));
    }

    function create(options) {
        const opts = options || {};
        const term = opts.term;
        const getProfile = typeof opts.getProfile === 'function' ? opts.getProfile : function () { return null; };
        const isMuted = typeof opts.isMuted === 'function' ? opts.isMuted : function () { return true; };
        let audio = null;
        let noiseBuffer = null;
        let sources = [];
        let run = null;
        let disposed = false;

        function audible() {
            const profile = getProfile();
            return !disposed && !!(profile && profile.retro) && !isMuted() && !silenced();
        }

        function context() {
            if (audio) return audio;
            const Ctor = window.AudioContext || window.webkitAudioContext;
            if (!Ctor) return null;
            try {
                audio = new Ctor();
            } catch (e) {
                audio = null;
            }
            if (audio && audio.state === 'suspended' && typeof audio.resume === 'function') {
                audio.resume().catch(function () {});
            }
            return audio;
        }

        function remember(node) {
            sources.push(node);
            node.onended = function () {
                sources = sources.filter(function (item) { return item !== node; });
            };
        }

        function envelope(ctx, start, duration, gain) {
            const out = ctx.createGain();
            out.gain.setValueAtTime(0.0001, start);
            out.gain.linearRampToValueAtTime(gain, start + 0.01);
            out.gain.setValueAtTime(gain, Math.max(start + 0.01, start + duration - 0.02));
            out.gain.linearRampToValueAtTime(0.0001, start + duration);
            out.connect(ctx.destination);
            return out;
        }

        function tone(freqs, offset, duration, gain) {
            const ctx = context();
            if (!ctx) return;
            const start = ctx.currentTime + offset;
            const out = envelope(ctx, start, duration, gain);
            freqs.forEach(function (freq) {
                const osc = ctx.createOscillator();
                osc.type = 'sine';
                osc.frequency.value = freq;
                osc.connect(out);
                osc.start(start);
                osc.stop(start + duration + 0.02);
                remember(osc);
            });
        }

        function noise(offset, duration, freq, q, gain) {
            const ctx = context();
            if (!ctx) return;
            if (!noiseBuffer) {
                noiseBuffer = ctx.createBuffer(1, Math.max(1, Math.floor(ctx.sampleRate)), ctx.sampleRate);
                const samples = noiseBuffer.getChannelData(0);
                for (let i = 0; i < samples.length; i += 1) samples[i] = Math.random() * 2 - 1;
            }
            const start = ctx.currentTime + offset;
            const source = ctx.createBufferSource();
            source.buffer = noiseBuffer;
            source.loop = true;
            const filter = ctx.createBiquadFilter();
            filter.type = 'bandpass';
            filter.frequency.value = freq;
            filter.Q.value = q;
            source.connect(filter);
            filter.connect(envelope(ctx, start, duration, gain));
            source.start(start);
            source.stop(start + duration + 0.02);
            remember(source);
        }

        function handshakeSounds() {
            HANDSHAKE.forEach(function (step) {
                if (step.q) noise(step.at, step.duration, step.freq, step.q, step.gain);
                else tone([step.freq], step.at, step.duration, step.gain);
            });
        }

        function stopSounds() {
            const active = sources;
            sources = [];
            active.forEach(function (node) {
                node.onended = null;
                try { node.stop(0); } catch (e) {}
                try { node.disconnect(); } catch (e) {}
            });
        }

        function write(current, upto) {
            if (disposed || !term || upto <= current.written) return;
            term.write(current.transcript.slice(current.written, upto));
            current.written = upto;
        }

        function wait(current, ms) {
            return new Promise(function (resolve, reject) {
                if (current.done) {
                    reject(new Error('skipped'));
                    return;
                }
                const timer = setTimeout(function () {
                    current.wake = null;
                    if (current.done) reject(new Error('skipped'));
                    else resolve();
                }, ms);
                current.wake = function () {
                    clearTimeout(timer);
                    current.wake = null;
                    reject(new Error('skipped'));
                };
            });
        }

        async function typeUntil(current, upto) {
            while (current.written < upto) {
                write(current, current.written + 1);
                await wait(current, TYPE_MS);
            }
        }

        async function sequence(current, entryId) {
            const text = current.transcript;
            const atz = text.indexOf('\r\n');
            const ok = text.indexOf('\r\n', atz + 2);
            const atdt = text.indexOf('\r\n', ok + 2);
            await typeUntil(current, atz);
            write(current, atz + 2);
            await wait(current, 280);
            write(current, ok + 2);
            await wait(current, 220);
            await typeUntil(current, atdt);
            write(current, atdt + 2);
            const digits = pseudoNumber(entryId);
            for (let i = 0; i < digits.length; i += 1) {
                if (audible()) tone(DTMF[digits[i]], 0.005, TONE_MS / 1000, 0.05);
                await wait(current, TONE_MS + GAP_MS);
            }
            await wait(current, 250);
            if (audible()) handshakeSounds();
            await wait(current, HANDSHAKE_MS);
        }

        function finish(current, result, quiet) {
            if (current.done) return;
            current.done = true;
            if (current.wake) current.wake();
            if (result === 'skipped') {
                stopSounds();
                if (!quiet) write(current, current.transcript.length);
            }
            if (run === current) run = null;
            current.resolve(result);
        }

        // Prints ATZ / OK / ATDT <host>; resolves 'done' after the handshake or 'skipped'.
        function dial(host, entryId) {
            if (run) finish(run, 'skipped', true);
            const current = {
                transcript: 'ATZ\r\nOK\r\nATDT ' + window.TerminalText.printable(host) + '\r\n',
                written: 0,
                done: false,
                wake: null,
                resolve: null
            };
            const promise = new Promise(function (resolve) { current.resolve = resolve; });
            run = current;
            if (disposed || !term) {
                finish(current, 'skipped', true);
                return promise;
            }
            if (reducedMotion()) {
                write(current, current.transcript.length);
                finish(current, 'done');
                return promise;
            }
            sequence(current, entryId).then(function () {
                finish(current, 'done');
            }, function () {
                finish(current, 'skipped');
            });
            return promise;
        }

        function skip() {
            if (run) finish(run, 'skipped');
            else stopSounds();
        }

        function connectLine(baud) {
            if (disposed || !term) return;
            term.write('CONNECT ' + (normalizeBaud(baud) || 14400) + '\r\n');
        }

        function dispose() {
            if (run) finish(run, 'skipped', true);
            disposed = true;
            stopSounds();
            if (audio && typeof audio.close === 'function') audio.close().catch(function () {});
            audio = null;
            noiseBuffer = null;
        }

        return { dial: dial, skip: skip, connectLine: connectLine, dispose: dispose };
    }

    // Releases queued output per animation frame at baud/10 bytes per second.
    function createThrottle(write) {
        const sink = typeof write === 'function' ? write : function () {};
        let baud = 0;
        let queue = [];
        let queued = 0;
        let frame = 0;
        let last = 0;
        let budget = 0;
        let disposed = false;

        function schedule() {
            if (!frame && !disposed && queue.length) frame = window.requestAnimationFrame(release);
        }

        function flush() {
            if (frame) {
                window.cancelAnimationFrame(frame);
                frame = 0;
            }
            const pending = queue;
            queue = [];
            queued = 0;
            last = 0;
            budget = 0;
            pending.forEach(function (part) { sink(part); });
        }

        function release(now) {
            frame = 0;
            if (disposed) return;
            if (!baud) {
                flush();
                return;
            }
            const elapsed = last ? Math.min(1000, Math.max(0, now - last)) : 16;
            last = now;
            budget += (baud / 10) * elapsed / 1000;
            let allowance = Math.floor(budget);
            budget -= allowance;
            while (allowance > 0 && queue.length) {
                const head = queue[0];
                if (head.length <= allowance) {
                    queue.shift();
                    queued -= head.length;
                    allowance -= head.length;
                    sink(head);
                } else {
                    queue[0] = head.subarray(allowance);
                    queued -= allowance;
                    sink(head.subarray(0, allowance));
                    allowance = 0;
                }
            }
            if (queue.length) schedule();
            else {
                last = 0;
                budget = 0;
            }
        }

        function push(bytes) {
            if (disposed || !bytes || !bytes.length) return;
            if (!baud && !queue.length) {
                sink(bytes);
                return;
            }
            queue.push(bytes);
            queued += bytes.length;
            if (queued > MAX_QUEUE_BYTES) {
                flush();
                return;
            }
            schedule();
        }

        function setBaud(value) {
            baud = normalizeBaud(value);
            if (!baud) flush();
            else schedule();
        }

        function dispose() {
            disposed = true;
            if (frame) window.cancelAnimationFrame(frame);
            frame = 0;
            queue = [];
            queued = 0;
        }

        return { setBaud: setBaud, push: push, flush: flush, dispose: dispose };
    }

    window.TerminalModem = {
        BAUD_RATES: BAUD_RATES.slice(),
        loadBaud: loadBaud,
        saveBaud: saveBaud,
        create: create,
        createThrottle: createThrottle
    };
})();
