/* Runenpfad procedural adaptive music. DOM-free, seeded: composition is pure, synthesis needs a Web Audio port. */
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
    const VOICE_CAP = 24;
    const CROSSFADE = 1.5;
    const LOOKAHEAD = 0.25;
    const BEATS = 4;
    const ARP_PATTERNS = [[0, 1, 2, 3, 2, 1, 0, 1], [0, 2, 1, 2, 0, 2, 1, 3], [0, 1, 2, 1, 3, 2, 1, 2]];
    const MELODY_RHYTHMS = [[0, 1, 1.5, 2.5], [0, 0.5, 1, 2], [0, 1.5, 2, 3], [0, 1, 2, 3], [0, 0.75, 1.5, 2.5, 3]];
    const OSTINATO = [0, 0, 1, 0, 0, 4, 0, 1];
    const EIGHTHS = [0, 0.5, 1, 1.5, 2, 2.5, 3, 3.5];
    const SIXTEENTHS = [0, 0.25, 0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.25, 2.5, 2.75, 3, 3.25, 3.5, 3.75];

    // Biome themes: tonic (midi), mode, tempo, two 4-bar chord loops (scale degrees), instruments, calm drum grid.
    const BASE_SPECS = {
        woodland: { root: 50, scale: 'dorian', bpm: 96, prog: [[0, 3, 0, 4], [5, 3, 4, 0]], arpInstr: 'pluck', melInstr: 'pluck', arpGrid: EIGHTHS },
        cavern: { root: 57, scale: 'phrygian', bpm: 80, prog: [[0, 1, 0, 6], [0, 3, 1, 0]], arpInstr: 'bell', melInstr: 'bell', arpGrid: [0, 1.5, 2, 3.5] },
        desert: { root: 52, scale: 'phrygianDominant', bpm: 104, prog: [[0, 1, 0, 6], [0, 4, 1, 0]], arpInstr: 'pluck', melInstr: 'pluck', arpGrid: EIGHTHS },
        glacier: { root: 54, scale: 'lydian', bpm: 72, prog: [[0, 1, 0, 4], [5, 1, 4, 0]], arpInstr: 'bell', melInstr: 'bell', arpGrid: [0, 1, 2, 3] },
        citadel: { root: 48, scale: 'harmonicMinor', bpm: 120, prog: [[0, 5, 3, 4], [0, 3, 4, 0]], arpInstr: 'pluck', melInstr: 'pluck', arpGrid: EIGHTHS }
    };

    // Boss characters. lead/arp/bass/ost name the instrument of each role; bossArp is the arp layer level (0 = silent).
    const BOSS_SPECS = {
        // Boar: war drums and a low horn-like brass lead.
        woodland: {
            leadInstr: 'horn', arpInstr: 'horn', bossArp: 0, bassInstr: 'bass', ostInstr: 'ostinato', ostGrid: EIGHTHS, droneVel: 1,
            drums: { kick: [0, 1.5], snare: [2], hat: [3], taiko: [0.5, 1, 1.5, 2.5, 3, 3.5] }
        },
        // Goblin: dark cave bells over plucked low strings.
        cavern: {
            leadInstr: 'darkbell', arpInstr: 'darkbell', bossArp: 0.4, bassInstr: 'bass', ostInstr: 'string', ostGrid: EIGHTHS, droneVel: 1,
            drums: { kick: [0, 2.5], snare: [2], hat: [1, 3], tom: [1.5, 3.5] }
        },
        // Orb: snake-charmer reed over a heavy drone, kept dark (flat 2nd, minor).
        desert: {
            leadInstr: 'reed', arpInstr: 'reed', bossArp: 0, bassInstr: 'bass', ostInstr: 'ostinato', ostGrid: EIGHTHS, droneVel: 1.5, leadStyle: 'charmer',
            drums: { kick: [0, 1.5], snare: [2], hat: [1, 3], tom: [0.75, 2.75, 3.5] }
        },
        // Quad: icy glassy bells and a fast ostinato.
        glacier: {
            leadInstr: 'glass', arpInstr: 'glass', bossArp: 0.5, bassInstr: 'bass', ostInstr: 'ostinato', ostGrid: SIXTEENTHS, droneVel: 1, arpGrid: EIGHTHS,
            drums: { kick: [0, 1.5], snare: [2], hat: [1, 3], tom: [3.5] }
        },
        // Mech: mechanical ticking percussion and a distorted bass.
        citadel: {
            leadInstr: 'lead', arpInstr: 'pluck', bossArp: 0.35, bassInstr: 'fuzzbass', ostInstr: 'ostinato', ostGrid: EIGHTHS, droneVel: 1,
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
                bpm: s.bpm, prog: s.prog, arpInstr: s.arpInstr, melInstr: s.melInstr, arpGrid: s.arpGrid,
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

    // Scale degree (may exceed 6 or go negative) -> midi note; always lands on a scale tone.
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

    // One bar of one layer as note events: {t, dur (beats), midi|null, vel, type}.
    function composeBar(themeId, layer, bar, seed) {
        const theme = THEMES[themeId];
        if (!theme) return [];
        const s = seed >>> 0;
        const section = Math.floor(bar / 4) % 2;
        const phrase = ((bar % 4) + 4) % 4;
        const d = theme.prog[section][phrase];
        const rng = rngOf(hash32(themeId + '|' + layer + '|' + bar + '|' + s));
        const root = theme.root;
        const out = [];
        if (layer === 'pad') {
            [d, d + 2, d + 4].forEach(function (deg, i) {
                out.push({ t: 0, dur: BEATS, midi: fold(degMidi(theme, deg), root - 2, root + 14), vel: i === 0 ? 0.9 : 0.75, type: 'pad' });
            });
            // Desert boss: a low sustained minor second against the drone.
            if (theme.leadStyle === 'charmer') out.push({ t: 0, dur: BEATS, midi: root - 11, vel: 0.8, type: 'pad' });
        } else if (layer === 'bass') {
            const low = fold(degMidi(theme, d), root - 14, root - 3);
            const fifth = fold(degMidi(theme, d + 4), root - 14, root - 3);
            if (theme.boss) {
                [[0, 1.4, 1], [1.5, 0.45, 0.8], [2, 1.5, 0.9]].forEach(function (n) {
                    out.push({ t: n[0], dur: n[1], midi: low, vel: n[2], type: theme.bassInstr || 'bass' });
                });
            } else if (rng() < 0.5) {
                out.push({ t: 0, dur: 2.5, midi: low, vel: 1, type: 'bass' });
                out.push({ t: 2.5, dur: 0.5, midi: low, vel: 0.7, type: 'bass' });
                out.push({ t: 3, dur: 1, midi: fifth, vel: 0.8, type: 'bass' });
            } else {
                out.push({ t: 0, dur: 3, midi: low, vel: 1, type: 'bass' });
                out.push({ t: 3, dur: 1, midi: fifth, vel: 0.75, type: 'bass' });
            }
        } else if (layer === 'arp') {
            const pat = ARP_PATTERNS[hash32(themeId + '|arp|' + s) % ARP_PATTERNS.length];
            const tones = [d, d + 2, d + 4, d + 7];
            const gap = theme.arpGrid.length > 1 ? theme.arpGrid[1] - theme.arpGrid[0] : 1;
            theme.arpGrid.forEach(function (t, i) {
                if (!theme.boss && rng() < 0.12) return;
                const idx = pat[i % pat.length] % 4;
                out.push({ t: t, dur: Math.max(0.5, gap * 1.2), midi: fold(degMidi(theme, tones[idx]) + 12, root + 7, root + 24), vel: (t % 1 === 0 ? 0.85 : 0.6) + rng() * 0.15, type: theme.arpInstr });
            });
        } else if (layer === 'melody') {
            if (theme.boss) {
                const lead = theme.leadInstr || 'lead';
                if (theme.leadStyle === 'charmer') {
                    // Sparse, low reed: long held notes on the flat second and the fifth, with rests.
                    const pairs = [[1, 4], [4, 1], [1, 0], [4, 1]][phrase];
                    [[0, 1.9, pairs[0]], [2.1, 1.7, pairs[1]]].forEach(function (n, k) {
                        out.push({ t: n[0], dur: n[1], midi: fold(degMidi(theme, n[2]), root - 12, root - 1), vel: 0.75 + k * 0.05, type: lead });
                    });
                    return out;
                }
                if (theme.leadStyle === 'slow') {
                    // Slow, low saw lead on the minor third and the flat sixth.
                    const degs = [[2, 5], [5, 2], [2, 2], [5, 2]][phrase];
                    [[0, 2.1, degs[0]], [2.3, 1.5, degs[1]]].forEach(function (n, k) {
                        out.push({ t: n[0], dur: n[1], midi: fold(degMidi(theme, n[2]), root - 7, root + 4), vel: 0.75 + k * 0.05, type: lead });
                    });
                    return out;
                }
                out.push({ t: 0, dur: 3.6, midi: fold(degMidi(theme, d + (phrase === 3 ? 1 : 0)), root, root + 11), vel: 0.8, type: lead });
                if (phrase % 2 === 1) out.push({ t: 2, dur: 1.8, midi: fold(degMidi(theme, d + 2), root, root + 11), vel: 0.6, type: lead });
                return out;
            }
            if (bar % 8 === 7) {
                out.push({ t: 0, dur: 3, midi: fold(degMidi(theme, d + 7), root + 12, root + 28), vel: 0.7, type: theme.melInstr });
                return out;
            }
            const motif = motifFor(theme, s);
            motif.forEach(function (n, i) {
                let step = n.step;
                if (phrase === 1) step += 1;
                else if (phrase === 2 && i === motif.length - 1) step += 2;
                else if (phrase === 3) step = i === motif.length - 1 ? 0 : motif[motif.length - 1 - i].step;
                if (section === 1 && phrase === 0 && i === 1) step -= 1;
                out.push({ t: n.t, dur: n.dur, midi: fold(degMidi(theme, d + 7 + step), root + 12, root + 28), vel: 0.75 + rng() * 0.2, type: theme.melInstr });
            });
        } else if (layer === 'perc') {
            const fill = phrase === 3;
            theme.drums.kick.forEach(function (t) { out.push({ t: t, dur: 0.3, midi: null, vel: 0.9, type: 'kick' }); });
            theme.drums.snare.forEach(function (t) { out.push({ t: t, dur: 0.2, midi: null, vel: 0.6, type: 'snare' }); });
            theme.drums.hat.forEach(function (t) { out.push({ t: t, dur: 0.1, midi: null, vel: 0.4 + rng() * 0.3, type: 'hat' }); });
            (theme.drums.tom || []).forEach(function (t, i) { out.push({ t: t, dur: 0.25, midi: null, vel: 0.7 + 0.1 * (i % 2), type: 'tom' }); });
            (theme.drums.taiko || []).forEach(function (t, i) { out.push({ t: t, dur: 0.4, midi: null, vel: 0.8 + 0.15 * (i % 2), type: 'taiko' }); });
            (theme.drums.tick || []).forEach(function (t, i) { out.push({ t: t, dur: 0.05, midi: null, vel: theme.drums.tickQuiet ? 0.3 : (i % 4 === 0 ? 0.9 : 0.5), type: 'tick' });
            });
            (theme.drums.clank || []).forEach(function (t) {
                out.push({ t: t, dur: 0.3, midi: null, vel: 0.8, type: 'clank' }); });
            if (fill && !theme.boss && !theme.noFill) {
                [3.25, 3.5, 3.75].forEach(function (t) { out.push({ t: t, dur: 0.15, midi: null, vel: 0.5, type: 'snare' }); });
            }
        } else if (layer === 'boss') {
            if (!theme.boss) return out;
            out.push({ t: 0, dur: 4, midi: fold(degMidi(theme, 0), root - 24, root - 13), vel: theme.droneVel || 1, type: 'drone' });
            [[0, [d, d + 1], 1], [1.5, [d, d + 4], 0.8], [2.75, [d + 1, d], 0.9]].forEach(function (h) {
                h[1].forEach(function (deg) {
                    out.push({ t: h[0], dur: 0.4, midi: fold(degMidi(theme, deg), root - 12, root - 1), vel: h[2], type: 'stab' });
                });
            });
            if (phrase === 3) out.push({ t: 0, dur: 4, midi: null, vel: 1, type: 'swell' });
            (theme.ostGrid || EIGHTHS).forEach(function (t, i) {
                const fast = (theme.ostGrid || EIGHTHS).length > 8;
                out.push({ t: t, dur: fast ? 0.2 : 0.4, midi: theme.ostLow ? fold(degMidi(theme, d + OSTINATO[(fast ? i >> 1 : i) % 8]), root - 18, root - 6) : fold(degMidi(theme, d + OSTINATO[(fast ? i >> 1 : i) % 8]), root - 12, root + 2), vel: i % 2 === 0 ? 1 : 0.7, type: theme.ostInstr || 'ostinato' });
            });
        }
        return out;
    }

    // Chord tones (semitones over the root) of each boss intro swell: dark in every biome.
    const BOSS_INTRO_STEPS = { woodland: [0, 7], cavern: [0, 1, 7], desert: [0, 1, 7], glacier: [0, 6, 7], citadel: [0, 3, 6] };

    function biomeOf(sim) {
        const stage = sim && sim.stage;
        if (!stage) return 'woodland';
        if (stage.biome && THEMES[stage.biome.id]) return stage.biome.id;
        return BIOME_ORDER[(((stage.worldIndex | 0) % 5) + 5) % 5];
    }

    function themeIdFor(sim, boss) {
        const biome = biomeOf(sim);
        const duel = boss && root.RunenpfadDuelMusic ? root.RunenpfadDuelMusic.themeFor(sim) : null; // per-NPC duel loop (duel-music.js)
        const place = !duel && root.RunenpfadPlaceMusic ? root.RunenpfadPlaceMusic.themeFor(sim) : null; // interior / village loop (place-music.js)
        const pick = duel || place;
        return pick && THEMES[pick] ? pick : (boss ? 'boss_' + biome : biome);
    }

    // Reads the simulation only; danger rises as the nearest live enemy closes inside 360 px.
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
        let out = null;
        let send = null;
        let stingOut = null;
        let noise = null;
        let shape = null;
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
        let override = null, overrideTimer = null;

        function teardown() {
            decks.forEach(killDeck);
            decks = [];
            curDeck = null;
            [out, send, stingOut].forEach(function (n) { if (n) { try { n.disconnect(); } catch (err) { /* closed */ } } });
            out = send = stingOut = noise = ctx = null;
            voiceCount = 0;
        }

        function ensureGraph() {
            let c = null;
            try { c = port.ctx; } catch (err) { c = null; }
            if (!c || !port.bus) return false;
            if (ctx === c) return true;
            if (ctx) teardown();
            ctx = c;
            out = c.createGain();
            out.gain.value = 0;
            out.connect(port.bus);
            send = c.createGain();
            send.gain.value = 0.32;
            out.connect(send);
            if (port.reverb) send.connect(port.reverb);
            stingOut = c.createGain();
            stingOut.gain.value = 0.7;
            stingOut.connect(port.bus);
            stingOut.connect(send);
            const len = c.sampleRate;
            noise = c.createBuffer(1, len, c.sampleRate);
            const data = noise.getChannelData(0);
            const r = rngOf(0x5EED);
            for (let i = 0; i < len; i++) data[i] = r() * 2 - 1;
            shape = new Float32Array(513);
            for (let i = 0; i < 513; i++) shape[i] = Math.tanh(3 * (i / 256 - 1)) / Math.tanh(3);
            applyOut(true);
            return true;
        }

        function applyOut(first) {
            if (!out || !ctx) return;
            const level = enabled && active && (!fadedOut || override) ? 1 : 0;
            const now = ctx.currentTime;
            out.gain.cancelScheduledValues(now);
            if (first) out.gain.setValueAtTime(0, now);
            out.gain.setTargetAtTime(level, now, level ? 0.3 : 0.2);
        }

        function disc(n) { try { n.disconnect(); } catch (err) { /* already gone */ } }

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
            deck.gain.gain.linearRampToValueAtTime(deck.theme.vol || 1, now + (old ? CROSSFADE : 0.6));
            decks.push(deck);
            curDeck = deck;
        }

        function setLayers(deck, danger, now) {
            const th = deck.theme;
            const dd = clamp(danger, 0, 1);
            const target = {
                pad: th.boss ? 0.7 : 1 - 0.3 * dd,
                bass: th.boss ? 0.45 + 0.2 * dd : 0.8 + 0.4 * dd,
                arp: th.boss ? (th.bossArp || 0) * 1.7 : 0.45 + 0.65 * dd,
                melody: th.boss ? 1.0 : 0.85 * (1 - 0.6 * dd),
                perc: th.boss ? 0.65 : (dd > 0.12 ? 0.4 + 1.1 * dd : th.perc || 0),
                boss: th.boss ? 1 : 0
            };
            LAYERS.forEach(function (l) {
                const L = deck.layers[l];
                const tv = target[l];
                if (L.v >= 0 && Math.abs(L.v - tv) < 0.04 && (L.v === 0) === (tv === 0)) return;
                L.v = tv;
                L.node.gain.setTargetAtTime(tv, now, l === 'perc' ? 0.7 : 0.5);
            });
        }

        // ---- instruments -------------------------------------------------
        function osc(type, freq, t, detune) {
            const o = ctx.createOscillator();
            o.type = type;
            o.frequency.value = freq;
            if (detune) o.detune.value = detune;
            o.start(t);
            return o;
        }

        function noiseSrc(t, offset) {
            const s = ctx.createBufferSource();
            s.buffer = noise;
            s.start(t, offset);
            return s;
        }

        function filt(type, freq, q) {
            const f = ctx.createBiquadFilter();
            f.type = type;
            f.frequency.value = freq;
            if (q) f.Q.value = q;
            return f;
        }

        function envelope(g, t, peak, atk, endT) {
            g.gain.setValueAtTime(0.0001, t);
            g.gain.linearRampToValueAtTime(peak, t + atk);
            g.gain.exponentialRampToValueAtTime(0.0001, endT);
        }

        function buildVoice(ev, t, f, dur, dest, pan) {
            const g = ctx.createGain();
            const nodes = [g];
            const keep = function (n) { nodes.push(n); return n; };
            if (pan && typeof ctx.createStereoPanner === 'function') {
                const p = keep(ctx.createStereoPanner());
                p.pan.value = pan;
                g.connect(p);
                p.connect(dest);
            } else {
                g.connect(dest);
            }
            const vel = ev.vel;
            let end = t + dur + 0.1;
            let main;
            if (ev.type === 'pad') {
                const hp = keep(filt('highpass', 180, 0.5));
                const lp = keep(filt('lowpass', ev.boss ? 900 : 2200, 0.6));
                const a = keep(osc('sawtooth', f, t, -9));
                const b = keep(osc('sawtooth', f, t, 9));
                const c = keep(osc('triangle', f * 2, t, 0));
                const lfo = keep(osc('sine', 0.08 + (ev.midi % 5) * 0.015, t, 0));
                const lfoG = keep(ctx.createGain());
                lfoG.gain.value = 800;
                lfo.connect(lfoG);
                lfoG.connect(lp.frequency);
                [a, b, c].forEach(function (o) { o.connect(hp); });
                hp.connect(lp);
                lp.connect(g);
                end = t + dur + 0.7;
                g.gain.setValueAtTime(0.0001, t);
                g.gain.linearRampToValueAtTime(0.026 * vel, t + 0.55);
                g.gain.setValueAtTime(0.026 * vel, t + dur - 0.05);
                g.gain.linearRampToValueAtTime(0.0001, end);
                [a, b, lfo].forEach(function (o) { o.stop(end + 0.02); });
                main = c;
            } else if (ev.type === 'bell') {
                const car = keep(osc('sine', f, t, 0));
                const mod = keep(osc('sine', f * 3.51, t, 0));
                const mg = keep(ctx.createGain());
                mg.gain.setValueAtTime(f * 3.2, t);
                mg.gain.exponentialRampToValueAtTime(f * 0.08, t + 1.0);
                mod.connect(mg);
                mg.connect(car.frequency);
                car.connect(g);
                end = t + 1.7;
                envelope(g, t, 0.2 * vel, 0.003, end);
                mod.stop(end + 0.02);
                main = car;
            } else if (ev.type === 'pluck') {
                const top = Math.min(6000, f * 9);
                const lp = keep(filt('lowpass', top, 1.0));
                lp.frequency.setValueAtTime(top, t);
                lp.frequency.exponentialRampToValueAtTime(Math.max(500, f * 1.5), t + 0.18);
                const a = keep(osc('sawtooth', f, t, 0));
                const b = keep(osc('square', f, t, 6));
                const bg = keep(ctx.createGain());
                bg.gain.value = 0.45;
                a.connect(lp);
                b.connect(bg);
                bg.connect(lp);
                lp.connect(g);
                end = t + Math.min(1.1, dur * 0.9 + 0.3);
                envelope(g, t, 0.17 * vel, 0.004, end);
                b.stop(end + 0.02);
                main = a;
            } else if (ev.type === 'bass') {
                const lp = keep(filt('lowpass', ev.boss ? 380 : 260, 0.7));
                const a = keep(osc('sine', f, t, 0));
                const b = keep(osc(ev.boss ? 'sawtooth' : 'triangle', f, t, 0));
                const bg = keep(ctx.createGain());
                bg.gain.value = ev.boss ? 0.2 : 0.25;
                a.connect(lp);
                b.connect(bg);
                bg.connect(lp);
                lp.connect(g);
                end = t + dur + 0.12;
                g.gain.setValueAtTime(0.0001, t);
                g.gain.linearRampToValueAtTime(0.13 * vel, t + 0.025);
                g.gain.setTargetAtTime(0.08 * vel, t + 0.05, 0.4);
                g.gain.setValueAtTime(0.04 * vel, Math.max(t + 0.06, end - 0.12));
                g.gain.linearRampToValueAtTime(0.0001, end);
                b.stop(end + 0.02);
                main = a;
            } else if (ev.type === 'stab') {
                const lp = keep(filt('lowpass', 1800, 1.2));
                lp.frequency.setValueAtTime(2200, t);
                lp.frequency.exponentialRampToValueAtTime(420, t + 0.5);
                const a = keep(osc('sawtooth', f, t, -7));
                const b = keep(osc('sawtooth', f, t, 7));
                a.connect(lp);
                b.connect(lp);
                lp.connect(g);
                end = t + Math.min(1.2, dur + 0.25);
                envelope(g, t, 0.07 * vel, 0.03, end);
                b.stop(end + 0.02);
                main = a;
            } else if (ev.type === 'ostinato') {
                const lp = keep(filt('lowpass', 900, 1.0));
                lp.frequency.setValueAtTime(900, t);
                lp.frequency.exponentialRampToValueAtTime(260, t + 0.2);
                const a = keep(osc('sawtooth', f, t, 0));
                const b = keep(osc('square', f, t, -5));
                const bg = keep(ctx.createGain());
                bg.gain.value = 0.5;
                a.connect(bg);
                b.connect(bg);
                if (typeof ctx.createWaveShaper === 'function' && shape) {
                    const ws = keep(ctx.createWaveShaper());
                    ws.curve = shape;
                    bg.gain.value = 2.2;
                    bg.connect(ws);
                    ws.connect(lp);
                } else {
                    bg.connect(lp);
                }
                lp.connect(g);
                end = t + 0.24;
                envelope(g, t, 0.14 * vel, 0.004, end);
                b.stop(end + 0.02);
                main = a;
            } else if (ev.type === 'drone') {
                const hp = keep(filt('highpass', 45, 0.7));
                hp.connect(g);
                const a = keep(osc('sine', f, t, 0));
                const b = keep(osc('triangle', f, t, 4));
                const bg = keep(ctx.createGain());
                bg.gain.value = 0.3;
                a.connect(hp);
                b.connect(bg);
                bg.connect(hp);
                end = t + dur + 0.4;
                g.gain.setValueAtTime(0.0001, t);
                g.gain.linearRampToValueAtTime(0.055, t + 0.4);
                g.gain.setValueAtTime(0.055, t + dur - 0.1);
                g.gain.linearRampToValueAtTime(0.0001, end);
                b.stop(end + 0.02);
                main = a;
            } else if (ev.type === 'lead') {
                const lp = keep(filt('lowpass', ev.boss ? 2600 : 1400, 0.8));
                const a = keep(osc('sawtooth', f, t, -6));
                const b = keep(osc('sawtooth', f, t, 6));
                a.connect(lp);
                b.connect(lp);
                lp.connect(g);
                end = t + dur + 0.35;
                g.gain.setValueAtTime(0.0001, t);
                g.gain.linearRampToValueAtTime(0.044 * vel, t + 0.2);
                g.gain.setValueAtTime(0.044 * vel, t + dur - 0.1);
                g.gain.linearRampToValueAtTime(0.0001, end);
                b.stop(end + 0.02);
                main = a;
            } else if (ev.type === 'horn') {
                // Low brass: detuned saws, a slow filter bloom and gentle vibrato.
                const lp = keep(filt('lowpass', 800, 0.9));
                lp.frequency.setValueAtTime(600, t);
                lp.frequency.linearRampToValueAtTime(3000, t + Math.min(0.5, dur * 0.5));
                lp.frequency.linearRampToValueAtTime(2000, t + dur);
                const a = keep(osc('sawtooth', f, t, -8));
                const b = keep(osc('sawtooth', f, t, 8));
                const c = keep(osc('square', f / 2, t, 0));
                const cg = keep(ctx.createGain());
                cg.gain.value = 0.3;
                const lfo = keep(osc('sine', 5, t, 0));
                const lg = keep(ctx.createGain());
                lg.gain.value = f * 0.007;
                lfo.connect(lg);
                lg.connect(a.frequency);
                lg.connect(b.frequency);
                a.connect(lp);
                b.connect(lp);
                c.connect(cg);
                cg.connect(lp);
                lp.connect(g);
                end = t + dur + 0.3;
                g.gain.setValueAtTime(0.0001, t);
                g.gain.linearRampToValueAtTime(0.1 * vel, t + 0.14);
                g.gain.setValueAtTime(0.1 * vel, t + dur - 0.05);
                g.gain.linearRampToValueAtTime(0.0001, end);
                [b, c, lfo].forEach(function (o) { o.stop(end + 0.02); });
                main = a;
            } else if (ev.type === 'reed') {
                // Snake-charmer reed: nasal formant, pitch scoop and wide vibrato.
                const bp = keep(filt('bandpass', clamp(f * 4, 1200, 3000), 1.6));
                const a = keep(osc('square', f, t, 0));
                const b = keep(osc('sawtooth', f, t, 5));
                a.frequency.setValueAtTime(f * 0.983, t);
                a.frequency.exponentialRampToValueAtTime(f, t + 0.1);
                const lfo = keep(osc('sine', 4.5, t, 0));
                const lg = keep(ctx.createGain());
                lg.gain.setValueAtTime(0, t);
                lg.gain.linearRampToValueAtTime(f * 0.0037, t + 0.4);
                lfo.connect(lg);
                lg.connect(a.frequency);
                lg.connect(b.frequency);
                const bg = keep(ctx.createGain());
                bg.gain.value = 0.7;
                a.connect(bp);
                b.connect(bg);
                bg.connect(bp);
                bp.connect(g);
                end = t + dur + 0.2;
                g.gain.setValueAtTime(0.0001, t);
                g.gain.linearRampToValueAtTime(0.127 * vel, t + 0.08);
                g.gain.setValueAtTime(0.103 * vel, t + dur - 0.05);
                g.gain.linearRampToValueAtTime(0.0001, end);
                [b, lfo].forEach(function (o) { o.stop(end + 0.02); });
                main = a;
            } else if (ev.type === 'darkbell') {
                // Cave bell: inharmonic FM, low-passed and long-ringing.
                const lp = keep(filt('lowpass', 2600, 0.7));
                const car = keep(osc('sine', f, t, 0));
                const mod = keep(osc('sine', f * 2.76, t, 0));
                const mg = keep(ctx.createGain());
                mg.gain.setValueAtTime(f * 1.8, t);
                mg.gain.exponentialRampToValueAtTime(f * 0.05, t + 1.4);
                mod.connect(mg);
                mg.connect(car.frequency);
                car.connect(lp);
                lp.connect(g);
                end = t + 2.3;
                envelope(g, t, 0.25 * vel, 0.004, end);
                mod.stop(end + 0.02);
                main = car;
            } else if (ev.type === 'glass') {
                // Icy glass bell: an octave up with a bright, quickly fading FM index.
                const car = keep(osc('sine', f * 2, t, 0));
                const mod = keep(osc('sine', f * 2 * 5.07, t, 0));
                const mg = keep(ctx.createGain());
                mg.gain.setValueAtTime(f * 2 * 2.6, t);
                mg.gain.exponentialRampToValueAtTime(f * 0.1, t + 0.5);
                mod.connect(mg);
                mg.connect(car.frequency);
                car.connect(g);
                end = t + 1.1;
                envelope(g, t, 0.15 * vel, 0.002, end);
                mod.stop(end + 0.02);
                main = car;
            } else if (ev.type === 'string') {
                // Plucked low string.
                const lp = keep(filt('lowpass', 1400, 0.9));
                lp.frequency.setValueAtTime(1400, t);
                lp.frequency.exponentialRampToValueAtTime(320, t + 0.35);
                const a = keep(osc('triangle', f, t, 0));
                const b = keep(osc('sawtooth', f, t, 4));
                const bg = keep(ctx.createGain());
                bg.gain.value = 0.5;
                a.connect(lp);
                b.connect(bg);
                bg.connect(lp);
                lp.connect(g);
                end = t + 0.55;
                envelope(g, t, 0.2 * vel, 0.004, end);
                b.stop(end + 0.02);
                main = a;
            } else if (ev.type === 'fuzzbass') {
                // Distorted bass for the mech boss.
                const lp = keep(filt('lowpass', 650, 0.9));
                const a = keep(osc('sawtooth', f, t, 0));
                const b = keep(osc('square', f, t, -7));
                const bg = keep(ctx.createGain());
                bg.gain.value = 3;
                a.connect(bg);
                b.connect(bg);
                if (typeof ctx.createWaveShaper === 'function' && shape) {
                    const ws = keep(ctx.createWaveShaper());
                    ws.curve = shape;
                    bg.connect(ws);
                    ws.connect(lp);
                } else {
                    bg.connect(lp);
                }
                lp.connect(g);
                end = t + dur + 0.1;
                g.gain.setValueAtTime(0.0001, t);
                g.gain.linearRampToValueAtTime(0.1 * vel, t + 0.02);
                g.gain.setValueAtTime(0.06 * vel, Math.max(t + 0.05, end - 0.12));
                g.gain.linearRampToValueAtTime(0.0001, end);
                b.stop(end + 0.02);
                main = a;
            } else if (ev.type === 'taiko') {
                // War drum: low skin thump with a short slap.
                const a = keep(osc('sine', 100, t, 0));
                a.frequency.setValueAtTime(100, t);
                a.frequency.exponentialRampToValueAtTime(40, t + 0.32);
                const tn = keep(noiseSrc(t, 0.5));
                const tb = keep(filt('bandpass', 700, 1.2));
                const tg = keep(ctx.createGain());
                tg.gain.value = 0.16;
                tn.connect(tb);
                tb.connect(tg);
                tg.connect(g);
                tn.stop(t + 0.05);
                a.connect(g);
                end = t + 0.5;
                envelope(g, t, 0.45 * vel, 0.003, end);
                main = a;
            } else if (ev.type === 'clank') {
                // Metallic clank: inharmonic square partials through narrow band-passes.
                const parts = [[310, 0.5], [517, 0.4], [830, 0.3], [1210, 0.2]];
                const m = keep(ctx.createGain());
                m.connect(g);
                let first = null;
                parts.forEach(function (pt) {
                    const o = keep(osc('square', pt[0], t, 0));
                    const bp = keep(filt('bandpass', pt[0] * 1.5, 5));
                    const pg = keep(ctx.createGain());
                    pg.gain.value = pt[1];
                    o.connect(bp);
                    bp.connect(pg);
                    pg.connect(m);
                    if (first) o.stop(t + 0.38); else first = o;
                });
                end = t + 0.4;
                envelope(g, t, 0.2 * vel, 0.002, end);
                main = first;
            } else if (ev.type === 'tick') {
                // Mechanical tick: a tiny click.
                const n = keep(noiseSrc(t, 0.2 + vel * 0.3));
                const bp = keep(filt('bandpass', 4200, 4));
                n.connect(bp);
                bp.connect(g);
                end = t + 0.045;
                envelope(g, t, 0.13 * vel, 0.001, end);
                main = n;
            } else if (ev.type === 'swell') {
                const n = keep(noiseSrc(t, 0.2));
                const bp = keep(filt('bandpass', 300, 1.4));
                bp.frequency.setValueAtTime(300, t);
                bp.frequency.exponentialRampToValueAtTime(3000, t + dur);
                n.connect(bp);
                bp.connect(g);
                end = t + dur + 0.05;
                g.gain.setValueAtTime(0.0001, t);
                g.gain.linearRampToValueAtTime(0.1, t + dur);
                g.gain.linearRampToValueAtTime(0.0001, end);
                main = n;
            } else if (ev.type === 'tom') {
                const a = keep(osc('sine', 120, t, 0));
                a.frequency.setValueAtTime(120, t);
                a.frequency.exponentialRampToValueAtTime(55, t + 0.22);
                const tn = keep(noiseSrc(t, 0.6));
                const tg = keep(ctx.createGain());
                tg.gain.value = 0.12;
                tn.connect(tg);
                tg.connect(g);
                tn.stop(t + 0.06);
                a.connect(g);
                end = t + 0.32;
                envelope(g, t, 0.35 * vel, 0.003, end);
                main = a;
            } else if (ev.type === 'kick') {
                const a = keep(osc('sine', 130, t, 0));
                a.frequency.setValueAtTime(130, t);
                a.frequency.exponentialRampToValueAtTime(44, t + 0.13);
                a.connect(g);
                end = t + 0.24;
                envelope(g, t, 0.42 * vel, 0.003, end);
                main = a;
            } else if (ev.type === 'snare') {
                const n = keep(noiseSrc(t, (ev.midi || 0) * 0.01 + 0.1));
                const bp = keep(filt('bandpass', 2400, 0.9));
                const body = keep(osc('triangle', 200, t, 0));
                const bg = keep(ctx.createGain());
                bg.gain.value = 0.35;
                n.connect(bp);
                bp.connect(g);
                body.connect(bg);
                bg.connect(g);
                end = t + 0.16;
                envelope(g, t, 0.24 * vel, 0.002, end);
                body.stop(end + 0.02);
                main = n;
            } else if (root.RunenpfadPlaceMusic && root.RunenpfadPlaceMusic.VOICES[ev.type]) { // lute, harp, flute, fiddle, plink, cello
                const pv = root.RunenpfadPlaceMusic.VOICES[ev.type]({ ctx: ctx, g: g, keep: keep, osc: osc, filt: filt, noiseSrc: noiseSrc, envelope: envelope }, f, t, dur, vel);
                end = pv.end; main = pv.main;
            } else {
                const n = keep(noiseSrc(t, 0.3 + vel * 0.2));
                const hp = keep(filt('highpass', ev.boss ? 6000 : 8000, 0.7));
                n.connect(hp);
                hp.connect(g);
                end = t + (ev.boss ? 0.07 : 0.055);
                envelope(g, t, (ev.boss ? 0.09 : 0.16) * vel, 0.002, end);
                main = n;
            }
            main.stop(end + 0.03);
            return { nodes: nodes, src: main };
        }

        // Stereo placement per instrument; arp, pad, hats and toms alternate sides per note.
        function panFor(deck, ev) {
            deck.alt = deck.alt || {};
            deck.alt[ev.type] = (deck.alt[ev.type] || 0) + 1;
            const side = deck.alt[ev.type] % 2 ? 1 : -1;
            switch (ev.type) {
                case 'pad': return 0.4 * side;
                case 'pluck':
                case 'bell': return ev.layer === 'arp' ? 0.35 * side : 0.1;
                case 'hat': return 0.2 * side;
                case 'tom': return 0.25 * side;
                case 'tick': return 0.3 * side;
                case 'clank': return -0.15;
                case 'glass':
                case 'darkbell': return ev.layer === 'arp' ? 0.3 * side : 0;
                default: return 0;
            }
        }

        function playNote(deck, ev, at) {
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
                deck.next = now + 0.06;
                deck.queue.length = 0;
            }
            while (deck.next <= horizon) composeInto(deck);
            while (deck.queue.length && deck.queue[0].at <= horizon) {
                const n = deck.queue.shift();
                if (n.at >= now - 0.1) playNote(deck, n, Math.max(n.at, now));
            }
        }

        // ---- stingers ----------------------------------------------------
        function tone(type, freq, t, dur, peak, dest) {
            const o = ctx.createOscillator();
            const g = ctx.createGain();
            o.type = type;
            o.frequency.value = freq;
            envelope(g, t, peak, 0.01, t + dur);
            o.connect(g);
            g.connect(dest);
            o.start(t);
            o.stop(t + dur + 0.03);
            o.onended = function () { disc(o); disc(g); };
        }

        function stinger(kind) {
            if (disposed || !enabled || !ensureGraph() || ctx.state !== 'running') return;
            const t = ctx.currentTime + 0.03;
            const theme = curDeck ? curDeck.theme : THEMES.woodland;
            const hz = function (m) { return 440 * Math.pow(2, (m - 69) / 12); };
            const r = theme.root + 12;
            try {
                if (kind === 'victory') {
                    [0, 4, 7, 12, 16, 19].forEach(function (s, i) {
                        tone('triangle', hz(r + s), t + i * 0.11, 1.4, 0.12, stingOut);
                        tone('sine', hz(r + s + 12), t + i * 0.11, 1.1, 0.05, stingOut);
                    });
                } else if (kind === 'defeat') {
                    [7, 3, 0, -5].forEach(function (s, i) {
                        tone('triangle', hz(r + s), t + i * 0.28, 1.6, 0.12, stingOut);
                    });
                } else if (kind === 'rune') {
                    // Quick rising sparkle.
                    [0, 7, 12, 19].forEach(function (s, i) {
                        tone('sine', hz(r + 12 + s), t + i * 0.06, 0.7, 0.07, stingOut);
                        tone('triangle', hz(r + s), t + i * 0.06, 0.5, 0.05, stingOut);
                    });
                } else if (kind === 'checkpoint') {
                    // Two warm rising notes over a held fifth.
                    [0, 7].forEach(function (s, i) {
                        tone('triangle', hz(r + s), t + i * 0.14, 1.0, 0.1, stingOut);
                        tone('sine', hz(r + s + 12), t + i * 0.14, 0.8, 0.05, stingOut);
                    });
                    tone('sine', hz(r - 5), t, 1.3, 0.05, stingOut);
                } else if (kind === 'boss_intro') {
                    [0, 0.45].forEach(function (dt) {
                        const k = ctx.createOscillator();
                        const g = ctx.createGain();
                        k.frequency.setValueAtTime(110, t + dt);
                        k.frequency.exponentialRampToValueAtTime(38, t + dt + 0.4);
                        envelope(g, t + dt, 0.5, 0.004, t + dt + 0.6);
                        k.connect(g);
                        g.connect(stingOut);
                        k.start(t + dt);
                        k.stop(t + dt + 0.65);
                        k.onended = function () { disc(k); disc(g); };
                    });
                    (BOSS_INTRO_STEPS[theme.biome] || [0, 7]).forEach(function (s, i) {
                        tone('sawtooth', hz(theme.root - 12 + s), t + 0.9, 1.8, 0.07 - i * 0.01, stingOut);
                    });
                }
            } catch (err) { /* stinger is cosmetic */ }
        }

        // ---- public ------------------------------------------------------
        // Rune pickups and checkpoints get a short stinger; each event array is read once.
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
            } catch (err) { /* music must never break the game loop */ }
        }

        function retireDecks(now) {
            decks = decks.filter(function (d) { const dead = d.retireAt !== null && d.retireAt < now && d !== curDeck; if (dead) killDeck(d); return !dead; });
        }

        function pumpOverride() { // cutscene music: update() stops while an overlay pauses the game, so a timer pumps this deck
            if (disposed || !override) return;
            try {
                if (!ensureGraph() || ctx.state !== 'running' || !enabled || !active) return;
                if (!curDeck || curDeck.themeId !== override) switchTo(override, hash32(override));
                const now = ctx.currentTime; setLayers(curDeck, 0, now); schedule(curDeck, now); retireDecks(now);
            } catch (err) { /* music must never break the game */ }
        }

        function setOverride(themeId) {
            const next = themeId && THEMES[themeId] ? themeId : null;
            if (next === override) return;
            if (overrideTimer !== null) { clearInterval(overrideTimer); overrideTimer = null; }
            override = next;
            if (next && typeof setInterval === 'function') overrideTimer = setInterval(pumpOverride, 100);
            if (!next && ctx) switchTo(null, 0); // fade the cutscene deck out; update() brings the normal theme back
            applyOut(false); pumpOverride();
        }

        function setTheme(themeId) {
            forced = themeId && THEMES[themeId] ? themeId : null;
        }

        function setEnabled(next) {
            enabled = !!next;
            applyOut(false);
        }

        function setActive(next) {
            active = !!next;
            applyOut(false);
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            if (overrideTimer !== null) clearInterval(overrideTimer);
            teardown();
        }

        return {
            update: update,
            stinger: stinger,
            setTheme: setTheme,
            setOverride: setOverride,
            setEnabled: setEnabled,
            setActive: setActive,
            dispose: dispose,
            getState: function () {
                return { theme: curDeck ? curDeck.themeId : null, voices: voiceCount, decks: decks.length, enabled: enabled, active: active, fadedOut: fadedOut, danger: lastDanger };
            }
        };
    }

    root.RunenpfadMusic = {
        THEMES: THEMES,
        SCALES: SCALES,
        LAYERS: LAYERS,
        VOICE_CAP: VOICE_CAP,
        hash32: hash32,
        composeBar: composeBar,
        themeIdFor: themeIdFor,
        intensityFor: intensityFor,
        createMusicController: createMusicController
    };
})(typeof window !== 'undefined' ? window : globalThis);
