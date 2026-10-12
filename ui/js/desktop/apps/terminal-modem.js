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

        // Created synchronously in dial() (the Enter keydown is the user gesture) and resumed
        // whenever the browser keeps or puts it in the suspended state.
        function context() {
            if (!audio) {
                const Ctor = window.AudioContext || window.webkitAudioContext;
                if (!Ctor) return null;
                try {
                    audio = new Ctor();
                } catch (e) {
                    return null;
                }
            }
            if (audio.state === 'suspended' && typeof audio.resume === 'function') {
                const resumed = audio.resume();
                if (resumed && typeof resumed.catch === 'function') resumed.catch(function () {});
            }
            return audio;
        }

        // Tones are scheduled only against a running clock; a suspended one would pile them up.
        function runningContext() {
            const ctx = context();
            return ctx && ctx.state === 'running' ? ctx : null;
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
            const ctx = runningContext();
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
            const ctx = runningContext();
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
        // A new dial() ends a running one as 'skipped' without printing its rest: callers reset the screen.
        function dial(host, entryId) {
            if (run) finish(run, 'skipped', true);
            if (audible()) context();
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

    // Throttle units (Contract C): one per code point outside escape sequences and one per complete escape sequence
    // (CSI; OSC, DCS, SOS, PM and APC strings ended by BEL or ST; SS2/SS3 with their byte; ESC with intermediates and
    // a final byte; ESC with one byte), so 24-bit colours and C64 glyphs cost what one character costs on a real
    // line. A synchronized-output block, ESC[?2026h through ESC[?2026l (the PETSCII engine's repaints and row
    // redraws), is one unit as a whole and leaves at once. An unterminated block is never held for long: it ends as
    // one unit once it holds SYNC_LIMIT bytes, or SYNC_WAIT_MS after the throttle stalled on it (up to its last
    // complete code point or sequence; the rest continues outside the block); push() beyond MAX_QUEUE_BYTES,
    // setBaud(0) and flush() release everything. An escape that has not ended SEQUENCE_LIMIT bytes after its ESC
    // counts as one unit there and its rest as text, so a broken stream cannot hold the queue back.
    const SEQUENCE_LIMIT = 1024;
    const SYNC_LIMIT = 64 * 1024;
    const SYNC_WAIT_MS = 1000;
    const SYNC_BEGIN = '?2026h';
    const SYNC_END = '?2026l';
    const TEXT = 0;
    const ESC = 1;
    const CSI = 2;
    const STRING = 3;
    const STRING_ESC = 4;
    const INTERMEDIATE = 5;
    const SHIFT = 6;
    // What a scanned byte completes: nothing, a code point or sequence inside an open synchronized block, a unit.
    const NONE = 0;
    const INNER = 1;
    const UNIT = 2;

    // A scanner for bytes that start outside any code point, sequence or synchronized block. step(byte) tells
    // whether the byte completes a unit (which leaves the scanner outside again) or a code point or sequence inside
    // an open block; the state carries over between calls, so every byte is scanned once.
    function unitScanner() {
        let state = TEXT;
        let pending = 0; // UTF-8 continuation bytes still expected
        let length = 0; // bytes since the ESC that opened the sequence
        let csi = ''; // the first parameter and intermediate bytes of the open CSI (enough for the 2026 modes)
        let ended = ''; // parameters and final byte of the CSI that just ended
        let sync = false; // inside ESC[?2026h ... ESC[?2026l
        let block = 0; // bytes of the open block after its ESC[?2026h
        let blocks = 0; // blocks opened so far (names the open one)

        // The byte after an ESC (also after the ESC inside a string that is not ST): true when the escape ends.
        function afterEscape(b) {
            if (b === 0x5b) {
                state = CSI;
                csi = '';
            } else if (b === 0x5d || b === 0x50 || b === 0x58 || b === 0x5e || b === 0x5f) state = STRING;
            else if (b === 0x4e || b === 0x4f) state = SHIFT;
            else if (b >= 0x20 && b <= 0x2f) state = INTERMEDIATE;
            else if (b === 0x1b || b < 0x20) state = ESC;
            else return true;
            return false;
        }

        // The next byte of an open escape: true when it ends the sequence. CAN and SUB cancel any sequence, as in
        // xterm's parser.
        function inSequence(b) {
            if (b === 0x18 || b === 0x1a) return true;
            if (state === ESC) return afterEscape(b);
            if (state === CSI) {
                if (b >= 0x40 && b <= 0x7e) {
                    ended = csi + String.fromCharCode(b);
                    return true;
                }
                if (b === 0x1b) state = ESC;
                else if (csi.length < 8) csi += String.fromCharCode(b);
                return false;
            }
            if (state === INTERMEDIATE) {
                if (b === 0x1b) state = ESC;
                return b >= 0x30 && b <= 0x7e;
            }
            if (state === STRING) {
                if (b === 0x1b) state = STRING_ESC;
                return b === 0x07;
            }
            if (state === STRING_ESC) return b === 0x5c || afterEscape(b);
            return true;
        }

        // A code point or sequence ended (`sequence`: the CSI's parameters and final byte, else ''): a unit outside
        // a synchronized block; inside one an inner boundary, and the block ends as one unit with ESC[?2026l or
        // once it holds SYNC_LIMIT bytes.
        function boundary(sequence) {
            if (!sync) {
                if (sequence !== SYNC_BEGIN) return UNIT;
                sync = true;
                block = 0;
                blocks += 1;
                return INNER;
            }
            if (sequence !== SYNC_END && block < SYNC_LIMIT) return INNER;
            sync = false;
            return UNIT;
        }

        function step(b) {
            if (sync) block += 1;
            if (state === TEXT) {
                if (pending && (b & 0xc0) === 0x80) {
                    pending -= 1;
                    return pending === 0 ? boundary('') : NONE;
                }
                pending = 0;
                if (b === 0x1b) {
                    state = ESC;
                    length = 1;
                    return NONE;
                }
                if (b >= 0xc0 && b < 0xf8) {
                    pending = b >= 0xf0 ? 3 : b >= 0xe0 ? 2 : 1;
                    return NONE;
                }
                return boundary('');
            }
            length += 1;
            if (inSequence(b) || length >= SEQUENCE_LIMIT) {
                const sequence = ended;
                ended = '';
                state = TEXT;
                return boundary(sequence);
            }
            return NONE;
        }

        return {
            step: step,
            // The open block's number, or 0 outside a block.
            openBlock: function () { return sync ? blocks : 0; },
            // Ends the open block where the scan stands: its bytes so far count as one unit.
            endBlock: function () { sync = false; }
        };
    }

    // Releases queued output per animation frame at baud/10 units per second (see unitScanner).
    function createThrottle(write) {
        const sink = typeof write === 'function' ? write : function () {};
        let baud = 0;
        let queue = [];
        let queued = 0;
        let frame = 0;
        let last = 0;
        let budget = 0;
        let disposed = false;
        let scanner = unitScanner();
        let scanPart = 0; // scanning goes on at queue[scanPart][scanIndex]
        let scanIndex = 0;
        let scanned = 0; // bytes at the queue head already scanned: an unfinished unit or an open block
        let inner = 0; // of those, the bytes up to the open block's last complete code point or sequence
        let stall = 0; // timer that releases the open block the throttle stalled on
        let stallBlock = 0;

        function schedule() {
            if (!frame && !disposed && queue.length) frame = window.requestAnimationFrame(release);
        }

        function stopStall() {
            if (stall) window.clearTimeout(stall);
            stall = 0;
            stallBlock = 0;
        }

        // The queue was emptied: scanning starts afresh.
        function resetScan() {
            scanner = unitScanner();
            scanPart = 0;
            scanIndex = 0;
            scanned = 0;
            inner = 0;
            stopStall();
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
            resetScan();
            pending.forEach(function (part) { sink(part); });
        }

        // Writes the first `count` queued bytes (all of them scanned already), part by part.
        function take(count) {
            let left = count;
            scanned -= count;
            inner = Math.max(0, inner - count);
            while (left > 0 && queue.length) {
                const head = queue[0];
                if (head.length <= left) {
                    queue.shift();
                    queued -= head.length;
                    left -= head.length;
                    if (scanPart > 0) scanPart -= 1;
                    else scanIndex = 0;
                    sink(head);
                } else {
                    queue[0] = head.subarray(left);
                    queued -= left;
                    if (scanPart === 0) scanIndex -= left;
                    sink(head.subarray(0, left));
                    left = 0;
                }
            }
        }

        // Scans on from where the last scan stopped until `limit` units are complete or the queue ends: the units
        // found and the bytes they take from the queue head.
        function scan(limit) {
            let units = 0;
            let bytes = 0;
            while (units < limit && scanPart < queue.length) {
                const part = queue[scanPart];
                while (units < limit && scanIndex < part.length) {
                    const found = scanner.step(part[scanIndex]);
                    scanIndex += 1;
                    scanned += 1;
                    if (found === UNIT) {
                        units += 1;
                        bytes = scanned;
                        inner = 0;
                    } else if (found === INNER) inner = scanned;
                }
                if (scanIndex >= part.length) {
                    scanPart += 1;
                    scanIndex = 0;
                }
            }
            return { units: units, bytes: bytes };
        }

        // SYNC_WAIT_MS after the throttle stalled on an open block, the block leaves as one unit up to its last
        // complete code point or sequence; an unfinished one after it continues outside the block.
        function expire() {
            const block = stallBlock;
            stall = 0;
            stallBlock = 0;
            if (disposed || !block || scanner.openBlock() !== block) return;
            scanner.endBlock();
            take(inner);
            schedule();
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
            const allowance = Math.floor(budget);
            budget -= allowance;
            const found = allowance > 0 ? scan(allowance) : { units: 0, bytes: 0 };
            take(found.bytes);
            const open = scanner.openBlock();
            if (stallBlock && open !== stallBlock) stopStall();
            // Fewer units than allowed: the queue is empty or holds only an unfinished unit or block, which push()
            // resumes (an open block also leaves SYNC_WAIT_MS later).
            if (queue.length && found.units === allowance) schedule();
            else {
                last = 0;
                budget = 0;
                if (open && !stall) {
                    stallBlock = open;
                    stall = window.setTimeout(expire, SYNC_WAIT_MS);
                }
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
            resetScan();
        }

        return { setBaud: setBaud, push: push, flush: flush, dispose: dispose };
    }

    window.TerminalModem = {
        BAUD_RATES: BAUD_RATES.slice(),
        MAX_QUEUE_BYTES: MAX_QUEUE_BYTES,
        loadBaud: loadBaud,
        saveBaud: saveBaud,
        create: create,
        createThrottle: createThrottle
    };
})();
