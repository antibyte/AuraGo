// Runenpfad adaptive procedural & scored music controller.
// DOM-free and seeded: orchestrates biome/boss/duel/place/story themes, stereo panning,
// crossfades, ducking, stingers, and voice lifecycle over an external Web Audio port.
(function (root) {
    'use strict';

    const SCALES = {
        dorian: [0, 2, 3, 5, 7, 9, 10],
        phrygian: [0, 1, 3, 5, 7, 8, 10],
        phrygianDominant: [0, 1, 4, 5, 7, 8, 10],
        lydian: [0, 2, 4, 6, 7, 9, 11],
        harmonicMinor: [0, 2, 3, 5, 7, 8, 11],
        minor: [0, 2, 3, 5, 7, 8, 10]
    };
    const LAYERS = ['pad', 'bass', 'arp', 'melody', 'perc', 'boss'];
    const BIOME_ORDER = ['woodland', 'cavern', 'desert', 'glacier', 'citadel'];
    const VOICE_CAP = 48;
    const CROSSFADE = 1.4;
    const LOOKAHEAD = 0.25;
    const BEATS = 4;
    const ARP_PATTERNS = [[0, 1, 2, 3, 2, 1, 0, 1], [0, 2, 1, 2, 0, 2, 1, 3], [0, 1, 2, 1, 3, 2, 1, 2]];
    const MELODY_RHYTHMS = [[0, 1, 1.5, 2.5], [0, 0.5, 1, 2], [0, 1.5, 2, 3], [0, 1, 2, 3], [0, 0.75, 1.5, 2.5, 3]];
    const OSTINATO = [0, 0, 1, 0, 0, 4, 0, 1];
    const EIGHTHS = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5];
    const SIXTEENTHS = [0, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 2.75, 3, 3.25, 3.5, 3.75];

    const BASE_SPECS = {
        woodland: { root: 50, scale: 'dorian', bpm: 96, perc: 0.28, prog: [[0, 3, 0, 4], [5, 3, 4, 0]], arpInstr: 'harp', melInstr: 'flute', arpGrid: EIGHTHS },
        cavern: { root: 57, scale: 'phrygian', bpm: 80, perc: 0.14, prog: [[0, 1, 0, 6], [0, 3, 1, 0]], arpInstr: 'celesta', melInstr: 'darkbell', arpGrid: [0, 1.5, 2, 3.5] },
        desert: { root: 52, scale: 'phrygianDominant', bpm: 104, perc: 0.30, prog: [[0, 1, 0, 6], [0, 4, 1, 0]], arpInstr: 'oud', melInstr: 'ney', arpGrid: EIGHTHS },
        glacier: { root: 54, scale: 'lydian', bpm: 72, perc: 0.16, prog: [[0, 1, 0, 4], [5, 1, 4, 0]], arpInstr: 'glass', melInstr: 'celesta', arpGrid: [0, 1, 2, 3] },
        citadel: { root: 48, scale: 'harmonicMinor', bpm: 120, perc: 0.26, prog: [[0, 5, 3, 4], [0, 3, 4, 0]], arpInstr: 'pizz', melInstr: 'brass', arpGrid: EIGHTHS }
    };

    const BOSS_SPECS = {
        woodland: {
            leadInstr: 'horn', arpInstr: 'lute', bossArp: 0.38, bassInstr: 'bass', ostInstr: 'ostinato', ostGrid: EIGHTHS, droneVel: 1,
            drums: { kick: [0, 1.5], snare: [1, 3], hat: [0.5, 1.5, 2.5, 3.5], taiko: [0.5, 2.5, 3.5] }
        },
        cavern: {
            leadInstr: 'darkbell', arpInstr: 'darkbell', bossArp: 0.4, bassInstr: 'bass', ostInstr: 'string', ostGrid: EIGHTHS, droneVel: 1,
            drums: { kick: [0, 2.5], snare: [2], hat: [1, 3], tom: [1.5, 3.5] }
        },
        desert: {
            leadInstr: 'reed', arpInstr: 'oud', bossArp: 0.25, bassInstr: 'bass', ostInstr: 'ostinato', ostGrid: EIGHTHS, droneVel: 1.4, leadStyle: 'charmer',
            drums: { kick: [0, 1.5], snare: [2], hat: [1, 3], tom: [0.75, 2.75, 3.5] }
        },
        glacier: {
            leadInstr: 'glass', arpInstr: 'glass', bossArp: 0.5, bassInstr: 'bass', ostInstr: 'ostinato', ostGrid: SIXTEENTHS, droneVel: 1, arpGrid: EIGHTHS,
            drums: { kick: [0, 1.5], snare: [2], hat: [1, 3], tom: [3.5] }
        },
        citadel: {
            leadInstr: 'lead', arpInstr: 'pizz', bossArp: 0.38, bassInstr: 'fuzzbass', ostInstr: 'ostinato', ostGrid: EIGHTHS, droneVel: 1,
            leadStyle: 'slow', ostLow: true,
            drums: { kick: [0, 1.5], snare: [], hat: [3.5], clank: [2], tick: [1, 3], tickQuiet: true }
        }
    };

    function buildThemes() {
        const out = {};
        BIOME_ORDER.forEach(function (biome) {
            const s = BASE_SPECS[biome];
            out[biome] = {
                id: biome, biome: biome, boss: false, root: s.root, scaleName: s.scale, scale: SCALES[s.scale],
                bpm: s.bpm, perc: s.perc || 0, prog: s.prog, arpInstr: s.arpInstr, melInstr: s.melInstr, arpGrid: s.arpGrid,
                drums: { kick: [0, 2], snare: [1, 3], hat: [0.5, 1.5, 2.5, 3.5] }
            };
            const bossScale = biome === 'citadel' ? 'harmonicMinor' : 'phrygian';
            const bs = BOSS_SPECS[biome];
            out['boss_' + biome] = {
                id: 'boss_' + biome, biome: biome, boss: true, root: s.root, scaleName: bossScale, scale: SCALES[bossScale],
                bpm: Math.round(s.bpm * 1.15), prog: [[0, 5, 6, 0], [0, 1, 0, 4]], arpInstr: bs.arpInstr, melInstr: bs.leadInstr, arpGrid: bs.arpGrid || EIGHTHS,
                leadInstr: bs.leadInstr, bossArp: bs.bossArp, bassInstr: bs.bassInstr, ostInstr: bs.ostInstr, ostGrid: bs.ostGrid,
                droneVel: bs.droneVel, leadStyle: bs.leadStyle || '', ostLow: !!bs.ostLow, drums: bs.drums
            };
        });
        return out;
    }
    const THEMES = buildThemes();

    function hash32(value) {
        const s = String(value);
        let h = 2166136261 >>> 0;
        for (let i = 0; i < s.length; i++) {
            h ^= s.charCodeAt(i);
            h = Math.imul(h, 16777619) >>> 0;
        }
        h ^= h >>> 15;
        h = Math.imul(h, 2246822519) >>> 0;
        h ^= h >>> 13;
        return h >>> 0;
    }

    function rngOf(seed) {
        let a = seed >>> 0;
        return function () {
            a = (a + 0x6D2B79F5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }

    function degMidi(theme, degree) {
        const oct = Math.floor(degree / 7);
        const idx = ((degree % 7) + 7) % 7;
        return theme.root + theme.scale[idx] + 12 * oct;
    }

    function fold(midi, lo, hi) {
        let m = midi;
        while (m > hi) m -= 12;
        while (m < lo) m += 12;
        return m;
    }

    function motifFor(theme, seed) {
        const rng = rngOf(hash32(theme.id + '|motif|' + seed));
        const rhythm = MELODY_RHYTHMS[Math.floor(rng() * MELODY_RHYTHMS.length)];
        let step = 2;
        const notes = [];
        rhythm.forEach(function (t, i) {
            const next = i + 1 < rhythm.length ? rhythm[i + 1] : BEATS;
            notes.push({ t: t, dur: Math.min(1.75, (next - t) * 0.95), step: step });
            const move = [-2, -1, -1, 1, 1, 2][Math.floor(rng() * 6)];
            step = clamp(step + move, 0, 6);
        });
        return notes;
    }

    function composeBar(themeId, layer, bar, seed) {
        const theme = THEMES[themeId];
        if (!theme) return [];
        if (theme.compose) return theme.compose(theme, layer, bar, seed >>> 0);
        const Scores = root.RunenpfadMusicScores;
        if (Scores && typeof Scores.compose === 'function') {
            const scored = Scores.compose(theme, layer, bar, seed >>> 0, { degMidi: degMidi, fold: fold });
            if (Array.isArray(scored)) return scored;
        }
        // Standalone procedural fallback when RunenpfadMusicScores is not loaded (e.g. minimal sim VM tests).
        const s = seed >>> 0;
        const section = Math.floor(bar / 4) % 2;
        const phrase = ((bar % 4) + 4) % 4;
        const d = theme.prog[section][phrase];
        const rng = rngOf(hash32(themeId + '|' + layer + '|' + bar + '|' + s));
        const r = theme.root;
        const out = [];
        if (layer === 'pad') {
            [d, d + 2, d + 4].forEach(function (deg, i) {
                out.push({ t: 0, dur: BEATS, midi: fold(degMidi(theme, deg), r - 2, r + 14), vel: i === 0 ? 0.9 : 0.75, type: 'pad' });
            });
            if (theme.leadStyle === 'charmer') out.push({ t: 0, dur: BEATS, midi: fold(degMidi(theme, 1), r - 14, r - 3), vel: 0.8, type: 'pad' });
        } else if (layer === 'bass') {
            const low = fold(degMidi(theme, d), r - 14, r - 3);
            const fifth = fold(degMidi(theme, d + 4), r - 14, r - 3);
            if (theme.boss) {
                [[0, 1.4, 1], [1.5, 0.45, 0.8], [2, 1.5, 0.9]].forEach(function (n) {
                    out.push({ t: n[0], dur: n[1], midi: low, vel: n[2], type: theme.bassInstr || 'bass' });
                });
            } else {
                out.push({ t: 0, dur: 2.8, midi: low, vel: 1, type: 'bass' });
                out.push({ t: 3, dur: 0.9, midi: fifth, vel: 0.8, type: 'bass' });
            }
        } else if (layer === 'arp') {
            const pat = ARP_PATTERNS[hash32(themeId + '|arp|' + s) % ARP_PATTERNS.length];
            const tones = [d, d + 2, d + 4, d + 7];
            const gap = theme.arpGrid.length > 1 ? theme.arpGrid[1] - theme.arpGrid[0] : 1;
            theme.arpGrid.forEach(function (t, i) {
                if (!theme.boss && rng() < 0.12) return;
                out.push({ t: t, dur: Math.max(0.5, gap * 1.2), midi: fold(degMidi(theme, tones[pat[i % pat.length] % 4]) + 12, r + 7, r + 24), vel: 0.78, type: theme.arpInstr });
            });
        } else if (layer === 'melody') {
            const motif = motifFor(theme, s);
            motif.forEach(function (n, i) {
                const step = n.step + (phrase === 1 ? 1 : 0);
                out.push({ t: n.t, dur: n.dur, midi: fold(degMidi(theme, d + 7 + step), r + 12, r + 28), vel: 0.85, type: theme.leadInstr || theme.melInstr });
            });
        } else if (layer === 'perc') {
            theme.drums.kick.forEach(function (t) { out.push({ t: t, dur: 0.3, midi: null, vel: 0.9, type: 'kick' }); });
            theme.drums.snare.forEach(function (t) { out.push({ t: t, dur: 0.2, midi: null, vel: 0.6, type: 'snare' }); });
            theme.drums.hat.forEach(function (t) { out.push({ t: t, dur: 0.1, midi: null, vel: 0.5, type: 'hat' }); });
        } else if (layer === 'boss' && theme.boss) {
            out.push({ t: 0, dur: 4, midi: fold(degMidi(theme, 0), r - 24, r - 13), vel: theme.droneVel || 1, type: 'drone' });
            [[0, [d, d + 1], 1], [1.5, [d, d + 4], 0.8], [2.75, [d + 1, d], 0.9]].forEach(function (h) {
                h[1].forEach(function (deg) {
                    out.push({ t: h[0], dur: 0.4, midi: fold(degMidi(theme, deg), r - 12, r - 1), vel: h[2], type: 'stab' });
                });
            });
            (theme.ostGrid || EIGHTHS).forEach(function (t, i) {
                const fast = (theme.ostGrid || EIGHTHS).length > 8;
                out.push({ t: t, dur: fast ? 0.2 : 0.4, midi: fold(degMidi(theme, d + OSTINATO[(fast ? i >> 1 : i) % 8]), theme.ostLow ? r - 18 : r - 12, theme.ostLow ? r - 6 : r + 2), vel: i % 2 === 0 ? 1 : 0.7, type: theme.ostInstr || 'ostinato' });
            });
        }
        return out;
    }

    const BOSS_INTRO_STEPS = { woodland: [0, 7], cavern: [0, 1, 7], desert: [0, 1, 7], glacier: [0, 6, 7], citadel: [0, 3, 6] };

    function biomeOf(sim) {
        const stage = sim && sim.stage;
        if (!stage) return 'woodland';
        if (stage.biome && THEMES[stage.biome.id]) return stage.biome.id;
        return BIOME_ORDER[(((stage.worldIndex | 0) % 5) + 5) % 5];
    }

    function themeIdFor(sim, boss) {
        const biome = biomeOf(sim);
        const duel = boss && root.RunenpfadDuelMusic ? root.RunenpfadDuelMusic.themeFor(sim) : null;
        const place = !duel && root.RunenpfadPlaceMusic ? root.RunenpfadPlaceMusic.themeFor(sim) : null;
        const pick = duel || place;
        return pick && THEMES[pick] ? pick : (boss ? 'boss_' + biome : biome);
    }

    function intensityFor(sim) {
        if (!sim || !sim.stage || !sim.player) return null;
        const p = sim.player;
        const stage = sim.stage;
        if (sim.encounter && sim.encounter.kind === 'duel') return { danger: 1, boss: true, bossAlive: true, state: sim.state };
        const enemies = stage.enemies || [];
        let nearest = Infinity;
        let boss = null;
        for (let i = 0; i < enemies.length; i++) {
            const e = enemies[i];
            if (!e || e.dead) continue;
            const dist = Math.hypot(e.x - p.x, e.y - p.y);
            if (dist < nearest) nearest = dist;
            if (stage.bossId !== undefined && stage.bossId !== null && e.id === stage.bossId) boss = { e: e, dist: dist };
        }
        const danger = nearest < 360 ? clamp(1 - nearest / 360, 0, 1) : 0;
        const engaged = !!(boss && (sim.bossIntro || boss.dist < 640 || (boss.e.ai && boss.e.ai.state && boss.e.ai.state !== 'wait')));
        return { danger: danger, boss: engaged, bossAlive: !!boss, state: sim.state };
    }

    function createMusicController(port) {
        if (!port || typeof port !== 'object') return null;
        let ctx = null;
        let master = null;
        let out = null;
        let wetOut = null;
        let send = null;
        let stingOut = null;
        let noise = null;
        let enabled = true;
        let active = true;
        let disposed = false;
        let fadedOut = false;
        let curDeck = null;
        let decks = [];
        let voiceCount = 0;
        let stageRef = null;
        let stageSeed = 0;
        let forced = null;
        let latched = false;
        let prevState = null;
        let prevIntro = null;
        let lastDanger = 0;
        let evRef = null;
        let evIdx = 0;
        let override = null;
        let overrideTimer = null;

        function disc(n) { try { n.disconnect(); } catch (err) { /* already gone */ } }

        function duck(depth, hold, rel) {
            if (master && typeof master.duck === 'function') master.duck(depth, hold, rel);
        }

        function teardown() {
            decks.forEach(killDeck);
            decks = [];
            curDeck = null;
            if (master && typeof master.dispose === 'function') master.dispose();
            [out, wetOut, send, stingOut].forEach(function (n) { if (n) disc(n); });
            master = out = wetOut = send = stingOut = noise = ctx = null;
            voiceCount = 0;
        }

        function ensureGraph() {
            let c = null;
            try { c = port.ctx; } catch (err) { c = null; }
            if (!c || !port.bus) return false;
            if (ctx === c) return true;
            if (ctx) teardown();
            ctx = c;
            const Voices = root.RunenpfadMusicVoices;
            master = Voices && typeof Voices.createMasterChain === 'function' ? Voices.createMasterChain(c, port) : null;
            out = c.createGain();
            out.gain.value = 0;
            stingOut = c.createGain();
            stingOut.gain.value = 0.82;
            if (master) {
                out.connect(master.input);
                wetOut = c.createGain();
                wetOut.gain.value = 0.25;
                out.connect(wetOut);
                wetOut.connect(master.wetInput);
                stingOut.connect(master.input);
                stingOut.connect(master.wetInput);
            } else {
                out.connect(port.bus);
                send = c.createGain();
                send.gain.value = 0.32;
                out.connect(send);
                if (port.reverb) send.connect(port.reverb);
                stingOut.connect(port.bus);
                stingOut.connect(send);
            }
            const len = c.sampleRate || 44100;
            noise = c.createBuffer(1, len, len);
            const data = noise.getChannelData(0);
            const r = rngOf(0x5EED);
            for (let i = 0; i < data.length; i++) data[i] = r() * 2 - 1;
            if (typeof port.registerDuck === 'function') port.registerDuck(duck);
            applyOut(true);
            return true;
        }

        function applyOut(first) {
            if (!out || !ctx) return;
            const level = enabled && active && (!fadedOut || override) ? 1 : 0;
            const now = ctx.currentTime;
            out.gain.cancelScheduledValues(now);
            if (first) out.gain.setValueAtTime(0, now);
            out.gain.setTargetAtTime(level, now, level ? 0.22 : 0.18);
        }

        function killDeck(deck) {
            deck.queue.length = 0;
            deck.voices.forEach(function (v) {
                try { v.src.stop(); } catch (err) { /* stopped */ }
                v.nodes.forEach(disc);
                voiceCount = Math.max(0, voiceCount - 1);
            });
            deck.voices.clear();
            disc(deck.gain);
            LAYERS.forEach(function (l) { disc(deck.layers[l].node); });
        }

        function makeDeck(themeId, seed) {
            const theme = THEMES[themeId];
            const g = ctx.createGain();
            g.gain.value = 0;
            g.connect(out);
            const layers = {};
            LAYERS.forEach(function (l) {
                const n = ctx.createGain();
                n.gain.value = 0;
                n.connect(g);
                layers[l] = { node: n, v: -1 };
            });
            return { themeId: themeId, theme: theme, seed: seed, gain: g, layers: layers, bar: 0, next: null, queue: [], voices: new Set(), retireAt: null, spb: 60 / theme.bpm };
        }

        function switchTo(themeId, seed) {
            const now = ctx.currentTime;
            const old = curDeck;
            if (old) {
                old.gain.gain.cancelScheduledValues(now);
                old.gain.gain.setValueAtTime(old.gain.gain.value, now);
                old.gain.gain.linearRampToValueAtTime(0, now + CROSSFADE);
                old.retireAt = now + CROSSFADE + 0.4;
                old.queue.length = 0;
            }
            if (!themeId) { curDeck = null; return; }
            const deck = makeDeck(themeId, seed);
            deck.gain.gain.setValueAtTime(0, now);
            deck.gain.gain.linearRampToValueAtTime(deck.theme.vol || 1, now + (old ? CROSSFADE : 0.35));
            if (master) {
                master.setTempo(deck.theme.bpm);
                master.setAmbientWet(deck.theme.biome === 'cavern' || deck.theme.biome === 'glacier' || deck.theme.biome === 'citadel' ? 0.42 : 0.28);
            }
            decks.push(deck);
            curDeck = deck;
        }

        function setLayers(deck, danger, now) {
            const th = deck.theme;
            const dd = clamp(danger, 0, 1);
            const target = {
                pad: th.boss ? 0.55 : 0.92 - 0.16 * dd,
                bass: th.boss ? 0.58 + 0.16 * dd : 0.92 + 0.28 * dd,
                arp: th.boss ? (th.bossArp || 0) * 1.35 : 0.68 + 0.42 * dd,
                melody: th.boss ? 0.96 : 0.98 - 0.14 * dd,
                perc: th.boss ? 0.78 : (dd > 0.12 ? 0.56 + 0.62 * dd : th.perc || 0),
                boss: th.boss ? 0.78 : 0
            };
            LAYERS.forEach(function (l) {
                const L = deck.layers[l];
                const tv = target[l];
                if (L.v >= 0 && Math.abs(L.v - tv) < 0.04 && (L.v === 0) === (tv === 0)) return;
                L.v = tv;
                L.node.gain.setTargetAtTime(tv, now, l === 'perc' ? 0.55 : 0.45);
            });
        }

        function envelope(g, t, peak, a, end) {
            const p = Math.max(0.00015, peak);
            g.gain.setValueAtTime(0.0001, t);
            g.gain.linearRampToValueAtTime(p, t + a);
            g.gain.exponentialRampToValueAtTime(0.0001, Math.max(t + a + 0.02, end));
        }

        function buildVoice(ev, t, f, dur, dest, pan) {
            const nodes = [];
            const dsp = root.RunenpfadAudioDsp;
            const keep = function (n) { nodes.push(n); return n; };
            const h = {
                ctx: ctx,
                keep: keep,
                dsp: dsp,
                filt: function (type, freq, q) {
                    const fl = ctx.createBiquadFilter();
                    fl.type = type; fl.frequency.value = freq; fl.Q.value = q;
                    return fl;
                },
                noiseSrc: function (when, amp, kind) {
                    const s = ctx.createBufferSource();
                    s.buffer = (dsp && dsp.getNoise(ctx, kind || 'pink')) || noise;
                    s.loop = true;
                    return s;
                }
            };
            const Voices = root.RunenpfadMusicVoices;
            if (Voices && typeof Voices.buildVoice === 'function') {
                const res = Voices.buildVoice(h, ev, t, f, dur, dest, pan);
                if (res && res.main) return { nodes: nodes, src: res.main, end: res.end };
            }
            // Minimal fallback when music-voices.js is omitted in isolated tests.
            const g = keep(ctx.createGain());
            let target = dest;
            if (Math.abs(pan) > 0.05 && typeof ctx.createStereoPanner === 'function') {
                const p = keep(ctx.createStereoPanner());
                p.pan.value = pan;
                p.connect(dest);
                target = p;
            }
            g.connect(target);
            const src = keep(ev.midi === null ? h.noiseSrc(t, 0.3) : ctx.createOscillator());
            if (src.frequency) src.frequency.value = f || 220;
            src.connect(g);
            const end = t + Math.max(0.08, dur);
            envelope(g, t, 0.14 * (ev.vel || 0.7), 0.01, end);
            src.start(t);
            src.stop(end + 0.03);
            return { nodes: nodes, src: src, end: end };
        }

        function panFor(deck, ev) {
            deck.alt = deck.alt || {};
            deck.alt[ev.type] = (deck.alt[ev.type] || 0) + 1;
            const side = deck.alt[ev.type] % 2 ? 1 : -1;
            switch (ev.type) {
                case 'pad':
                case 'choir':
                case 'string': return 0.42 * side;
                case 'harp':
                case 'lute':
                case 'oud':
                case 'pizz':
                case 'pluck': return ev.layer === 'arp' ? 0.42 * side : 0.22 * side;
                case 'celesta':
                case 'glass':
                case 'bell':
                case 'darkbell':
                case 'marimba': return ev.layer === 'arp' ? 0.46 * side : 0.16 * side;
                case 'flute':
                case 'ney': return -0.18;
                case 'fiddle':
                case 'brass': return 0.26;
                case 'horn': return -0.30;
                case 'stab': return 0.48 * side;
                case 'ostinato': return 0.32 * side;
                case 'snare': return -0.24;
                case 'taiko': return 0.20 * side;
                case 'hat':
                case 'tamb':
                case 'shaker': return 0.32 * side;
                case 'tom':
                case 'darbuka_tek':
                case 'tick':
                case 'clock': return 0.36 * side;
                case 'clank':
                case 'anvil': return -0.26;
                default: return 0;
            }
        }

        function retireExpired(deck, now) {
            deck.voices.forEach(function (v) {
                if (v.end + 0.12 < now) {
                    deck.voices.delete(v);
                    voiceCount = Math.max(0, voiceCount - 1);
                }
            });
        }

        function playNote(deck, ev, at) {
            if (voiceCount >= VOICE_CAP) retireExpired(deck, at);
            if (voiceCount >= VOICE_CAP) return;
            const layer = ev.layer;
            const dest = deck.layers[layer].node;
            const f = ev.midi === null ? 0 : 440 * Math.pow(2, (ev.midi - 69) / 12);
            let voice = null;
            try {
                ev.boss = deck.theme.boss;
                voice = buildVoice(ev, at, f, ev.dur * deck.spb, dest, panFor(deck, ev));
            } catch (err) { return; }
            voiceCount++;
            deck.voices.add(voice);
            voice.src.onended = function () {
                voice.nodes.forEach(disc);
                if (deck.voices.delete(voice)) voiceCount = Math.max(0, voiceCount - 1);
            };
        }

        function composeInto(deck) {
            const t0 = deck.next;
            LAYERS.forEach(function (layer) {
                const L = deck.layers[layer];
                if (L.v >= 0 && L.v < 0.03) return;
                composeBar(deck.themeId, layer, deck.bar, deck.seed).forEach(function (n) {
                    n.layer = layer;
                    n.at = t0 + n.t * deck.spb;
                    let i = deck.queue.length;
                    while (i > 0 && deck.queue[i - 1].at > n.at) i--;
                    deck.queue.splice(i, 0, n);
                });
            });
            deck.next += BEATS * deck.spb;
            deck.bar++;
        }

        function schedule(deck, now) {
            const horizon = now + LOOKAHEAD;
            if (deck.next === null || deck.next < now - 0.4) {
                deck.next = now + 0.04;
                deck.queue.length = 0;
            }
            while (deck.next <= horizon) composeInto(deck);
            while (deck.queue.length && deck.queue[0].at <= horizon) {
                const n = deck.queue.shift();
                if (n.at >= now - 0.1) playNote(deck, n, Math.max(n.at, now));
            }
        }

        function stinger(kind) {
            if (disposed || !enabled || !ensureGraph() || ctx.state !== 'running') return;
            const theme = curDeck ? curDeck.theme : THEMES.woodland;
            const Voices = root.RunenpfadMusicVoices;
            if (Voices && typeof Voices.playStinger === 'function') {
                duck(0.24, 0.25, 0.65);
                Voices.playStinger(ctx, stingOut, kind, theme);
                return;
            }
            try {
                const t = ctx.currentTime + 0.03;
                const hz = function (m) { return 440 * Math.pow(2, (m - 69) / 12); };
                const r = theme.root + 12;
                const steps = kind === 'boss_intro' ? (BOSS_INTRO_STEPS[theme.biome] || [0, 7]) : [0, 4, 7];
                steps.forEach(function (s, i) {
                    const o = ctx.createOscillator();
                    const g = ctx.createGain();
                    o.type = kind === 'boss_intro' ? 'sawtooth' : 'triangle';
                    o.frequency.value = hz(r + s);
                    envelope(g, t + i * 0.1, 0.09, 0.01, t + i * 0.1 + 0.8);
                    o.connect(g); g.connect(stingOut);
                    o.start(t + i * 0.1); o.stop(t + i * 0.1 + 0.85);
                    o.onended = function () { disc(o); disc(g); };
                });
            } catch (err) { /* cosmetic */ }
        }

        function scanEvents(events) {
            if (!Array.isArray(events)) return;
            if (events !== evRef) { evRef = events; evIdx = 0; }
            if (evIdx > events.length) evIdx = 0;
            let kind = null;
            for (; evIdx < events.length; evIdx++) {
                const ev = events[evIdx];
                if (ev && ev.type === 'rune_pickup') kind = 'rune';
                else if (ev && ev.type === 'checkpoint' && kind !== 'rune') kind = 'checkpoint';
                else if (ev && ev.type === 'duel_start') kind = 'boss_intro';
                else if (ev && ev.type === 'duel_end') kind = ev.detail && ev.detail.result === 'won' ? 'victory' : 'defeat';
            }
            if (kind) stinger(kind);
        }

        function retireDecks(now) {
            decks = decks.filter(function (d) {
                const dead = d.retireAt !== null && d.retireAt < now && d !== curDeck;
                if (dead) killDeck(d);
                return !dead;
            });
        }

        function update(intensity, sim) {
            if (disposed || override || !intensity || !sim || !sim.stage) return;
            try {
                if (!ensureGraph()) return;
                const state = sim.state;
                const running = ctx.state === 'running';
                const newStage = sim.stage !== stageRef;
                if (newStage) {
                    stageRef = sim.stage;
                    stageSeed = hash32(sim.stage.id === undefined ? 0 : sim.stage.id);
                    latched = false;
                    if (fadedOut) { fadedOut = false; applyOut(false); }
                    prevIntro = null;
                } else if (prevState !== null && state !== prevState && (state === 'won' || state === 'lost')) {
                    if (running) stinger(state === 'won' ? 'victory' : 'defeat');
                    fadedOut = true;
                    applyOut(false);
                }
                prevState = state;
                if (sim.bossIntro && sim.bossIntro !== prevIntro && running) stinger('boss_intro');
                prevIntro = sim.bossIntro || null;
                if (running && enabled && active) scanEvents(sim.events);
                if (!running || !enabled || !active || fadedOut || state !== 'playing') return;
                if (intensity.boss) latched = true;
                if (!intensity.bossAlive) latched = false;
                const id = forced || themeIdFor(sim, latched);
                const now = ctx.currentTime;
                if (!curDeck || curDeck.themeId !== id || newStage) switchTo(id, stageSeed);
                lastDanger = intensity.danger || 0;
                setLayers(curDeck, lastDanger, now);
                schedule(curDeck, now);
                retireDecks(now);
            } catch (err) { /* music must never break game loop */ }
        }

        function pumpOverride() {
            if (disposed || !override) return;
            try {
                if (!ensureGraph() || ctx.state !== 'running' || !enabled || !active) return;
                if (!curDeck || curDeck.themeId !== override) switchTo(override, hash32(override));
                const now = ctx.currentTime;
                setLayers(curDeck, 0, now);
                schedule(curDeck, now);
                retireDecks(now);
            } catch (err) { /* cosmetic */ }
        }

        function setOverride(themeId) {
            const next = themeId && THEMES[themeId] ? themeId : null;
            if (next === override) return;
            if (overrideTimer !== null) { clearInterval(overrideTimer); overrideTimer = null; }
            override = next;
            if (next && typeof setInterval === 'function') overrideTimer = setInterval(pumpOverride, 100);
            if (!next && ctx) switchTo(null, 0);
            applyOut(false);
            pumpOverride();
        }

        function setTheme(themeId) { forced = themeId && THEMES[themeId] ? themeId : null; }
        function setEnabled(next) { enabled = !!next; applyOut(false); }
        function setActive(next) { active = !!next; applyOut(false); }

        function dispose() {
            if (disposed) return;
            disposed = true;
            if (overrideTimer !== null) clearInterval(overrideTimer);
            overrideTimer = null;
            teardown();
        }

        function getState() {
            return {
                theme: curDeck ? curDeck.themeId : null,
                themeId: curDeck ? curDeck.themeId : null,
                bar: curDeck ? curDeck.bar : 0,
                voices: voiceCount,
                decks: decks.length,
                danger: lastDanger,
                fadedOut: fadedOut,
                latched: latched,
                enabled: enabled,
                active: active,
                override: override
            };
        }

        return {
            update: update, stinger: stinger, duck: duck, setTheme: setTheme, setOverride: setOverride,
            setEnabled: setEnabled, setActive: setActive, dispose: dispose, getState: getState
        };
    }

    root.RunenpfadMusic = {
        THEMES: THEMES, SCALES: SCALES, LAYERS: LAYERS, VOICE_CAP: VOICE_CAP,
        hash32: hash32, composeBar: composeBar, themeIdFor: themeIdFor,
        intensityFor: intensityFor, createMusicController: createMusicController
    };
})(typeof window !== 'undefined' ? window : globalThis);
