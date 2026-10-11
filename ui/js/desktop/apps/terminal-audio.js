(function () {
    'use strict';

    const STORAGE_KEY = 'aurago.desktop.terminal.audioMuted';
    const BELL = { type: 'square', freq: 880, gain: 0.05, dur: 0.12, release: 0.02 };
    // xterm reports every BEL byte: a burst gives one tone, BELs within this gap of the last tone are ignored.
    const BELL_GAP_MS = 150;

    function clock() {
        return window.performance && typeof window.performance.now === 'function' ? window.performance.now() : Date.now();
    }

    function shouldSilence() {
        return (document.body && document.body.dataset.animations === 'false')
            || (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function loadMuted() {
        try {
            return window.localStorage.getItem(STORAGE_KEY) === '1';
        } catch (e) {
            return false;
        }
    }

    function saveMuted(muted) {
        const next = !!muted;
        try {
            window.localStorage.setItem(STORAGE_KEY, next ? '1' : '0');
        } catch (e) {}
        return next;
    }

    function timbre(id) {
        if (id === 'commodore64') return { freq: 190, dur: 0.028, type: 'triangle', gain: 0.04 };
        if (id === 'ibm3278') return { freq: 2100, dur: 0.012, type: 'square', gain: 0.03 };
        if (id === 'apple2') return { freq: 1400, dur: 0.01, type: 'square', gain: 0.025 };
        return { freq: 720, dur: 0.016, type: 'sine', gain: 0.03 };
    }

    function create() {
        let profileId = 'modern';
        let retro = false;
        let muted = loadMuted();
        let ctx = null;
        let lastBell = -Infinity;

        function ensureCtx() {
            if (ctx) return ctx;
            const Ctor = window.AudioContext || window.webkitAudioContext;
            if (!Ctor) return null;
            try {
                ctx = new Ctor();
            } catch (e) {
                ctx = null;
            }
            return ctx;
        }

        function setProfile(profile) {
            profileId = profile && profile.id ? profile.id : 'modern';
            retro = !!(profile && profile.retro);
        }

        function setMuted(next) {
            muted = saveMuted(next);
            return muted;
        }

        // The context, asked to resume when the browser keeps or puts it in the suspended state.
        function wakeCtx() {
            const audio = ensureCtx();
            if (audio && audio.state === 'suspended') {
                audio.resume().catch(function () {});
            }
            return audio;
        }

        // A running context only: a tone scheduled against a suspended clock would wait and pile up.
        function runningCtx() {
            const audio = wakeCtx();
            if (!audio) return null;
            return audio.state === 'running' ? audio : null;
        }

        // One oscillator through a gain that decays after `dur` seconds (time constant `release`).
        function sound(audio, type, freq, level, dur, release) {
            const osc = audio.createOscillator();
            const gain = audio.createGain();
            osc.type = type;
            osc.frequency.value = freq;
            gain.gain.value = level;
            osc.connect(gain);
            gain.connect(audio.destination);
            const now = audio.currentTime;
            gain.gain.setTargetAtTime(0.0001, now + dur, release);
            osc.start(now);
            osc.stop(now + dur + release * 4);
        }

        function playKey(event) {
            if (!retro || muted || shouldSilence()) return;
            if (!event || event.repeat || event.ctrlKey || event.metaKey || event.altKey) return;
            const audio = wakeCtx();
            if (!audio) return;
            const spec = timbre(profileId);
            sound(audio, spec.type, spec.freq + Math.random() * 40, spec.gain, spec.dur, 0.008);
        }

        // The terminal bell of C64 sessions (BEL from the board): a short square tone under the key-click rules,
        // at most one per BELL_GAP_MS and only against a running clock. The gap counts from the tone itself: the
        // first BEL creates the AudioContext, whose constructor can block the page for 100-200 ms while the audio
        // device opens, and BELs parsed behind that stall must not add a second tone.
        function beep() {
            if (!retro || muted || shouldSilence()) return;
            if (clock() - lastBell < BELL_GAP_MS) return;
            const audio = runningCtx();
            if (!audio) return;
            lastBell = clock();
            sound(audio, BELL.type, BELL.freq, BELL.gain, BELL.dur, BELL.release);
        }

        function dispose() {
            if (ctx && typeof ctx.close === 'function') {
                ctx.close().catch(function () {});
            }
            ctx = null;
        }

        return { setProfile, setMuted, playKey, beep, dispose };
    }

    window.TerminalAudio = { create, loadMuted, saveMuted, shouldSilence };
})();
