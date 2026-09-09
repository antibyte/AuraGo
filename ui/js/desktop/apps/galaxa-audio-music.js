(function () {
    'use strict';
    const GC = window.GalaxaCore = window.GalaxaCore || {};
    GC.createAudioMusic = function (ctx) {
        const motifs = {
            nebula:    { root: 57, bpm: 124, notes: [12,0,7,10,12,0,15,14,12,7,10,0,7,5,3,7], bass: [0,0,8,10], wave: 'triangle' },
            asteroid:  { root: 50, bpm: 132, notes: [0,12,0,3,7,0,6,7,12,0,10,7,6,3,0,3], bass: [0,0,3,1], wave: 'sawtooth' },
            crystal:   { root: 62, bpm: 116, notes: [12,19,24,22,19,15,14,19,12,14,19,22,24,19,15,14], bass: [0,8,5,10], wave: 'sine' },
            storm:     { root: 52, bpm: 140, notes: [0,7,12,7,0,10,14,10,0,7,15,14,12,10,7,3], bass: [0,0,10,8], wave: 'square' },
            blackhole: { root: 48, bpm: 108, notes: [12,0,7,0,13,12,0,7,10,0,6,0,7,6,3,0], bass: [0,1,8,7], wave: 'sine' },
            void:      { root: 45, bpm: 144, notes: [12,7,15,14,12,3,7,10,12,19,17,15,14,10,7,0], bass: [0,8,5,7], wave: 'triangle' }
        };
        const hz = midi => 440 * Math.pow(2, (midi - 69) / 12);
        let timer = 0, nextTime = 0, step = 0, pending = null, paused = false;
        const layers = new Map();
        const MusicEngine = {
            themes: {}, playing: null, masterGain: null, tempoMult: 1, intensity: 3, semitoneOffset: 0,
            resolve(name) {
                if (name === 'title' || ctx.G.demoMode) return null;
                if (['boss', 'miniboss', 'deep_boss'].includes(name)) return (ctx.G.biome || 'nebula') + '_boss';
                if (name === 'gameplay') return ctx.G.biome || 'nebula';
                return name;
            },
            play(name) {
                const id = this.resolve(name);
                if (!id) { this.stop(); return; }
                if (id === this.playing || id === pending) return;
                const a = ctx.audio(); if (!a) return;
                this.masterGain = ctx.musicBus;
                if (ctx.GalagaMusic) ctx.GalagaMusic.stop();
                if (this.playing) pending = id;
                else { this.playing = id; nextTime = a.currentTime + 0.15; step = 0; }
                pump();
            },
            stop() { clearTimeout(timer); timer = 0; ctx.stopMusicVoices(); this.playing = null; pending = null; },
            setPaused(value) {
                if (value === paused) return;
                paused = value;
                if (value) { clearTimeout(timer); timer = 0; ctx.stopMusicVoices(); }
                else if (this.playing) { nextTime = (ctx.actx ? ctx.actx.currentTime : 0) + 0.05; pump(); }
            },
            setTempo(mult) { this.tempoMult = Math.max(0.8, Math.min(1.35, mult)); },
            setMuted(m) { ctx.G.muted = m; ctx.applyAudioSettings(); },
            setIntensity(value) { this.intensity = Math.max(0, Math.min(10, value)); },
            transpose(value) { this.semitoneOffset = value; },
            addLayer(id, def) { const layer = { def, gain: Math.min(0.04, def.vol || 0.03) }; layers.set(id, layer); return layer; },
            removeLayer(theme, id) { layers.delete(id); },
            setLayerGain(theme, id, gain) { if (layers.has(id)) layers.get(id).gain = Math.min(0.04, gain); }
        };
        for (const [id, motif] of Object.entries(motifs)) {
            MusicEngine.themes[id] = motif; MusicEngine.themes[id + '_boss'] = { ...motif, bpm: motif.bpm + 10 };
        }
        Object.assign(MusicEngine.themes, { gameplay: motifs.nebula, boss: motifs.void, challenge: { ...motifs.crystal, bpm: 144 },
            shop: { ...motifs.nebula, bpm: 88 }, victory: { ...motifs.crystal, bpm: 110 }, gameover: { ...motifs.blackhole, bpm: 84 },
            gauntlet: motifs.asteroid, hyperdrive: motifs.storm, mirror: motifs.crystal, title: { bpm: 120 } });
        function sequence(time) {
            const m = MusicEngine, th = m.themes[m.playing] || motifs.nebula, beat = 60 / (th.bpm * m.tempoMult);
            const boss = m.playing.endsWith('_boss'), quiet = m.playing === 'shop' || m.playing === 'gameover';
            const root = th.root + (ctx.settings.adaptiveMusic ? m.semitoneOffset : 0), bar = Math.floor(step / 16);
            const note = th.notes[step % 16], gain = quiet ? 0.11 : 0.17;
            const tone = (wave, pitch, length, volume, pan, fm) => ctx.synthTone(wave, hz(pitch), hz(pitch), length, volume, pan, time, ctx.musicBus, fm);
            if (!quiet || step % 2 === 0) tone(th.wave, root + note, beat * 0.32, gain, ctx.W * 0.65, th.wave === 'sine' ? 2 : 0);
            if (step % 4 === 0) {
                tone('triangle', root - 24 + th.bass[bar % 4], beat * 0.85, 0.30, ctx.W * 0.4);
                if (!quiet) ctx.synthTone('sine', 110, 38, 0.15, 0.34, undefined, time, ctx.musicBus);
            }
            if (!quiet && step % 8 === 4) ctx.schedNoise(time, 0.12, 0.17, 2600, ctx.musicBus);
            if (!quiet && step % (boss ? 1 : 2) === 0) ctx.schedNoise(time, 0.025, 0.05, 8500, ctx.musicBus);
            if (boss && step % 4 === 2) tone('square', root + note - 12, beat * 0.2, 0.11, ctx.W * 0.25);
            if (ctx.settings.adaptiveMusic && m.intensity > 6 && step % 4 === 0) tone('triangle', root + note + 12, beat * 0.5, 0.08);
            if (step % 16 === 0) for (const layer of layers.values()) {
                const n = layer.def.notes && layer.def.notes[bar % layer.def.notes.length];
                if (n && n.f > 0) ctx.synthTone(layer.def.wave, n.f, n.f, Math.min(2, beat * 3.5), layer.gain, undefined, time, ctx.musicBus);
            }
            nextTime += beat / 4; step++;
        }
        function pump() {
            clearTimeout(timer); timer = 0;
            const a = ctx.actx;
            if (!a || ctx.state.disposed || !MusicEngine.playing || paused) return;
            if (a.state === 'running') {
                if (nextTime < a.currentTime - 0.1) nextTime = a.currentTime + 0.02;
                while (nextTime < a.currentTime + 0.12) {
                    if (step % 16 === 0 && pending) { MusicEngine.playing = pending; pending = null; }
                    sequence(nextTime);
                }
            }
            timer = setTimeout(pump, 25);
        }

        const GalagaMusic = {
            el: null,
            _playing: false,
            _playPending: null,
            _shouldPlay: false,
            _blocked: false,
            _retryAt: 0,
            _url: '/img/audio/galaga.mp3',
            _ensure() {
                if (this.el) return this.el;
                const a = document.createElement('audio');
                a.src = this._url;
                a.loop = true;
                a.preload = 'auto';
                a.volume = Math.max(0, Math.min(1, (ctx.G.vol ?? 0.3) * (ctx.settings.musicVol ?? 70) / 100));
                a.addEventListener('playing', () => { if (this.el === a) this._playing = true; });
                a.addEventListener('pause', () => { if (this.el === a) this._playing = false; });
                a.addEventListener('error', () => {
                    if (this.el !== a) return;
                    this._playing = false;
                    this._playPending = null;
                    this._retryAt = Date.now() + 1000;
                    this.el = null;
                });
                this.el = a;
                return a;
            },
            play(fromGesture) {
                this._shouldPlay = true;
                if (ctx.G && ctx.G.muted) return;
                if (fromGesture) { this._blocked = false; this._retryAt = 0; }
                if (this._playing || (!fromGesture && this._playPending) || this._blocked || Date.now() < this._retryAt) return;
                const a = this._ensure();
                try {
                    a.currentTime = 0;
                    const pending = a.play();
                    if (!pending || typeof pending.then !== 'function') { this._playing = !a.paused; return; }
                    this._playPending = pending;
                    pending.then(() => {
                        if (this._playPending !== pending) return;
                        this._playPending = null;
                        if (!this._shouldPlay || (ctx.G && ctx.G.muted)) { try { a.pause(); a.currentTime = 0; } catch (_) {} return; }
                        this._playing = true;
                        this._blocked = false;
                    }).catch(err => {
                        if (this._playPending !== pending) return;
                        this._playPending = null;
                        this._playing = false;
                        this._blocked = !!(err && err.name === 'NotAllowedError');
                        this._retryAt = Date.now() + 1000;
                    });
                } catch (err) {
                    this._playing = false;
                    this._blocked = !!(err && err.name === 'NotAllowedError');
                    this._retryAt = Date.now() + 1000;
                }
            },
            resumeFromGesture() {
                if (this._shouldPlay && !(ctx.G && ctx.G.muted)) this.play(true);
            },
            stop() {
                this._shouldPlay = false;
                this._playPending = null;
                if (!this._playing && !this.el) return;
                const a = this._ensure();
                try { a.pause(); a.currentTime = 0; } catch (_) {}
                this._playing = false;
            },
            setMuted(m) {
                if (!this.el && !m) this._ensure();
                if (this.el) this.el.volume = m ? 0 : Math.max(0, Math.min(1, (ctx.G.vol ?? 0.3) * (ctx.settings.musicVol ?? 70) / 100));
                if (m && this._playing) { try { this.el.pause(); } catch (_) {} this._playing = false; }
                else if (!m && this._shouldPlay && !this._playing) { this.play(); }
            }
        };
        ctx.GalagaMusic = GalagaMusic;
        ctx.MusicEngine = MusicEngine;
    };
})();
