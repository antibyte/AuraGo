(function () {
    'use strict';

    const STORAGE_KEY = 'aurago.desktop.terminal.audioMuted';
    const BELL = { type: 'square', freq: 880, gain: 0.05, dur: 0.12, release: 0.02 };

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

        // One oscillator through a gain that decays after `dur` seconds (time constant `release`).
        function sound(type, freq, level, dur, release) {
            const audio = ensureCtx();
            if (!audio) return;
            if (audio.state === 'suspended') {
                audio.resume().catch(function () {});
            }
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
            const spec = timbre(profileId);
            sound(spec.type, spec.freq + Math.random() * 40, spec.gain, spec.dur, 0.008);
        }

        // The terminal bell of C64 sessions (BEL from the board): a short square tone under the key-click rules.
        function beep() {
            if (!retro || muted || shouldSilence()) return;
            sound(BELL.type, BELL.freq, BELL.gain, BELL.dur, BELL.release);
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
