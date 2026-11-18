/* Runenpfad story cutscenes: scene table, trigger rules, the chronicle list and the player state machine.
 * DOM-free (global `RunenpfadCutscenes`): no document, no window, no canvas, no random numbers, no clock.
 * Text lives in the locale files (`runenpfad.cut.*`); the canvas part is cutscene-view.js, the DOM part cutscene-ui.js. */
(function (root) {
    'use strict';

    const TYPE_CPS = 40;          // typewriter speed, characters per second
    const PANEL_SECONDS = 9;      // Ken Burns pan length of one panel
    const FADE_SECONDS = 0.6;     // crossfade between two panels
    const MAX_DT = 0.25;
    const FG_IDS = ['cutfg-woodland', 'cutfg-cavern', 'cutfg-desert', 'cutfg-glacier', 'cutfg-citadel'];
    const FX_KINDS = ['motes', 'embers', 'snow', 'mist', 'sand', 'sparks', 'rain'];
    const END_PREFIX = 'end_';

    const SCENES = Object.create(null);
    const IDS = [];

    // ---- scene table helpers ----
    // pan: [[cx, cy, zoom], [cx, cy, zoom]] (centre 0..1 of the cover-fitted image, zoom >= 1).
    function panel(art, fg, pan, fx, lights, sprites, lineCount) {
        return { art: art, fg: fg, pan: { from: pan[0], to: pan[1] }, fx: fx, lights: lights, sprites: sprites, lineCount: lineCount || 2 };
    }

    function light(x, y, r, color, pulse) {
        return { x: x, y: y, r: r, color: color, pulse: !!pulse };
    }

    function actor(clip, from, to, h, flip) {
        return { clip: clip, from: from, to: to, h: h, flip: !!flip };
    }

    function scene(id, music, panels) {
        const out = {
            id: id,
            titleKey: 'runenpfad.cut.title.' + id,
            music: music,
            panels: panels.map(function (p, i) {
                const lines = [];
                for (let k = 0; k < p.lineCount; k++) lines.push('runenpfad.cut.' + id + '.' + (i + 1) + (k ? 'b' : 'a'));
                return { art: p.art, fg: p.fg, pan: p.pan, fx: p.fx, lights: p.lights, sprites: p.sprites, lines: lines };
            })
        };
        SCENES[id] = out;
        IDS.push(id);
    }

    const GOLD = '#ffd56b';
    const BLUE = '#60a5fa';
    const ICE = '#7dd3fc';
    const AMBER = '#ffb060';
    const FIRE = '#ff7a2a';

    scene('prologue', 'story_dawn', [
        panel('cut-prologue-1', null, [[0.3, 0.5, 1], [0.7, 0.46, 1.12]], ['motes'], [light(0.5, 0.3, 0.3, GOLD, true), light(0.15, 0.55, 0.14, '#ffe9a8', false), light(0.85, 0.5, 0.14, '#ffe9a8', false)], []),
        panel('cut-prologue-2', null, [[0.5, 0.58, 1.14], [0.5, 0.46, 1]], ['sparks'], [light(0.33, 0.38, 0.2, ICE, true), light(0.62, 0.62, 0.08, AMBER, false)], []),
        panel('cut-prologue-3', null, [[0.5, 0.62, 1], [0.5, 0.36, 1.22]], ['sparks', 'embers'], [light(0.4, 0.3, 0.2, BLUE, true), light(0.7, 0.28, 0.2, AMBER, true)], []),
        panel('cut-prologue-4', 'cutfg-woodland', [[0.38, 0.56, 1.1], [0.56, 0.5, 1]], ['motes'], [light(0.5, 0.28, 0.12, '#9ae6ff', true), light(0.46, 0.6, 0.07, GOLD, true)], []),
        panel('cut-prologue-5', 'cutfg-woodland', [[0.5, 0.62, 1], [0.42, 0.5, 1.22]], ['motes', 'mist'], [light(0.72, 0.46, 0.08, AMBER, true), light(0.3, 0.7, 0.1, BLUE, false)], [])
    ]);

    scene('intro_2', 'story_dark', [
        panel('cut-intro2-1', 'cutfg-cavern', [[0.5, 0.3, 1.12], [0.5, 0.62, 1]], ['mist'], [light(0.2, 0.34, 0.12, AMBER, true), light(0.8, 0.4, 0.12, '#5eead4', true)], []),
        panel('cut-intro2-2', 'cutfg-cavern', [[0.5, 0.64, 1], [0.5, 0.4, 1.2]], ['embers'], [light(0.5, 0.3, 0.1, ICE, true), light(0.5, 0.34, 0.06, AMBER, true), light(0.15, 0.5, 0.1, AMBER, false)], [])
    ]);

    scene('intro_3', 'story_dark', [
        panel('cut-intro3-1', 'cutfg-desert', [[0.3, 0.5, 1], [0.66, 0.5, 1.14]], ['sand'], [light(0.73, 0.1, 0.3, '#ffe0a0', false), light(0.6, 0.7, 0.04, AMBER, true)], []),
        panel('cut-intro3-2', 'cutfg-desert', [[0.55, 0.38, 1.18], [0.45, 0.55, 1]], ['sand', 'motes'], [light(0.5, 0.5, 0.18, GOLD, true), light(0.5, 0.22, 0.12, '#fff1b8', true)], [])
    ]);

    scene('intro_4', 'story_dark', [
        panel('cut-intro4-1', 'cutfg-glacier', [[0.5, 0.65, 1], [0.5, 0.4, 1.2]], ['snow'], [light(0.75, 0.22, 0.07, '#ffcf70', true)], []),
        panel('cut-intro4-2', 'cutfg-glacier', [[0.4, 0.5, 1.08], [0.62, 0.5, 1.08]], ['snow'], [light(0.3, 0.55, 0.1, ICE, false)], [])
    ]);

    scene('intro_5', 'story_dark', [
        panel('cut-intro5-1', 'cutfg-citadel', [[0.5, 0.45, 1], [0.35, 0.58, 1.18]], ['embers', 'sparks'], [light(0.5, 0.42, 0.2, AMBER, true), light(0.5, 0.4, 0.1, GOLD, true)], []),
        panel('cut-intro5-2', 'cutfg-citadel', [[0.4, 0.55, 1.1], [0.6, 0.5, 1]], ['embers'], [light(0.33, 0.42, 0.12, '#ffd9a0', true), light(0.62, 0.55, 0.08, ICE, false)], [])
    ]);

    scene('after_1', 'story_triumph', [
        panel('cut-after1-1', 'cutfg-woodland', [[0.5, 0.55, 1.15], [0.5, 0.6, 1]], ['motes', 'mist'], [light(0.77, 0.62, 0.08, AMBER, true)], []),
        panel('cut-after1-2', 'cutfg-woodland', [[0.5, 0.6, 1], [0.58, 0.46, 1.16]], ['motes'], [light(0.26, 0.4, 0.16, BLUE, true), light(0.62, 0.62, 0.1, ICE, true)], [actor('fx-sparkle.play', [0.26, 0.5], [0.26, 0.5], 0.2, false)])
    ]);

    scene('after_2', 'story_triumph', [
        panel('cut-after2-1', 'cutfg-cavern', [[0.5, 0.72, 1.1], [0.5, 0.42, 1.04]], ['mist', 'motes'], [light(0.5, 0.4, 0.2, ICE, true)], []),
        panel('cut-after2-2', 'cutfg-cavern', [[0.3, 0.55, 1.1], [0.7, 0.45, 1]], ['motes'], [light(0.32, 0.52, 0.16, BLUE, true), light(0.84, 0.4, 0.2, GOLD, true)], [])
    ]);

    scene('after_3', 'story_triumph', [
        panel('cut-after3-1', 'cutfg-desert', [[0.4, 0.55, 1], [0.55, 0.5, 1.16]], ['sand', 'motes'], [light(0.5, 0.5, 0.18, ICE, true), light(0.14, 0.3, 0.26, '#ff9a4a', false)], []),
        panel('cut-after3-2', 'cutfg-desert', [[0.5, 0.58, 1.14], [0.5, 0.5, 1]], ['sand', 'motes'], [light(0.17, 0.5, 0.15, ICE, true), light(0.5, 0.58, 0.12, GOLD, true)], [])
    ]);

    scene('after_4', 'story_triumph', [
        panel('cut-after4-1', 'cutfg-glacier', [[0.3, 0.6, 1.1], [0.66, 0.46, 1]], ['snow', 'embers'], [light(0.22, 0.38, 0.12, FIRE, true), light(0.88, 0.2, 0.08, AMBER, false)], []),
        panel('cut-after4-2', 'cutfg-glacier', [[0.45, 0.55, 1], [0.7, 0.45, 1.16]], ['snow'], [light(0.1, 0.5, 0.13, BLUE, true), light(0.72, 0.2, 0.12, AMBER, true), light(0.48, 0.6, 0.1, BLUE, false)], [])
    ]);

    scene('finale', 'story_triumph', [
        panel('cut-finale-1', 'cutfg-citadel', [[0.5, 0.62, 1], [0.5, 0.42, 1.2]], ['sparks', 'motes'], [light(0.5, 0.4, 0.32, '#bfe8ff', true), light(0.5, 0.4, 0.14, '#ffffff', true)], []),
        panel('cut-finale-2', null, [[0.25, 0.5, 1], [0.75, 0.46, 1.1]], ['motes'], [light(0.12, 0.16, 0.22, GOLD, true), light(0.5, 0.3, 0.16, '#ffe9a8', true)], []),
        panel('cut-finale-3', null, [[0.5, 0.58, 1.14], [0.5, 0.48, 1]], ['motes'], [light(0.5, 0.3, 0.2, '#ffe9a8', true), light(0.5, 0.5, 0.1, '#9ae6ff', true)], [])
    ]);

    scene('end_friend', 'story_triumph', [
        panel('cut-end-friend', null, [[0.35, 0.55, 1], [0.65, 0.5, 1.12]], ['embers', 'motes'], [light(0.25, 0.55, 0.14, AMBER, true), light(0.55, 0.5, 0.14, AMBER, true), light(0.82, 0.58, 0.12, AMBER, true)], [])
    ]);

    scene('end_neutral', 'story_dawn', [
        panel('cut-end-neutral', null, [[0.7, 0.55, 1.12], [0.4, 0.55, 1]], ['motes'], [light(0.5, 0.72, 0.2, GOLD, true), light(0.15, 0.5, 0.08, AMBER, false)], [])
    ]);

    scene('end_feared', 'story_dark', [
        panel('cut-end-feared', null, [[0.5, 0.45, 1], [0.5, 0.62, 1.16]], ['mist'], [light(0.2, 0.4, 0.05, AMBER, false), light(0.75, 0.36, 0.05, AMBER, false)], [])
    ]);

    // ---- triggers ----
    const START_TRIGGERS = { 1: 'prologue', 6: 'intro_2', 11: 'intro_3', 16: 'intro_4', 21: 'intro_5' };
    const WON_TRIGGERS = { 5: 'after_1', 10: 'after_2', 15: 'after_3', 20: 'after_4', 25: 'finale' };

    function seenList(profile) {
        return profile && Array.isArray(profile.seen) ? profile.seen : [];
    }

    // 'runenpfad.npc.end_friend' | 'end_friend' | 'friend' -> 'end_friend'; anything else falls back to the quiet ending.
    function endingFor(epilogueKey) {
        const m = /^(?:runenpfad\.npc\.)?(?:end_)?(friend|neutral|feared)$/.exec(String(epilogueKey || ''));
        return END_PREFIX + (m ? m[1] : 'neutral');
    }

    // Scene ids to play now. Only campaign mode triggers scenes; a seen scene never returns, and the end scene
    // rides along with the finale on its first showing only.
    function pending(profile, event) {
        const out = [];
        if (!event || event.mode !== 'campaign') return out;
        const seen = seenList(profile);
        const level = +event.levelId;
        let id = null;
        if (event.type === 'stage_start') {
            // The prologue plays once on ANY campaign start (existing players see it too); world intros stay on their level.
            if (level >= 1 && level <= 25 && seen.indexOf('prologue') < 0) out.push('prologue');
            id = START_TRIGGERS[level] && START_TRIGGERS[level] !== 'prologue' ? START_TRIGGERS[level] : null;
        } else if (event.type === 'won') id = WON_TRIGGERS[level] || null;
        if (!id || seen.indexOf(id) >= 0) return out;
        out.push(id);
        if (id === 'finale') {
            const end = endingFor(event.epilogueKey);
            if (seen.indexOf(end) < 0) out.push(end);
        }
        return out;
    }

    function triggerLevel(id) {
        const tables = [START_TRIGGERS, WON_TRIGGERS];
        for (let i = 0; i < tables.length; i++) {
            const lv = Object.keys(tables[i]).filter(function (k) { return tables[i][k] === id; })[0];
            if (lv) return +lv;
        }
        return 0;
    }

    // The 10 main scenes plus the end scene the player has seen (the first one found, in table order).
    function chronicle(profile) {
        const seen = seenList(profile);
        const unlocked = profile && +profile.unlockedLevel > 0 ? +profile.unlockedLevel : 0;
        const list = [];
        IDS.forEach(function (id) {
            if (id.indexOf(END_PREFIX) === 0) return;
            // Old saves: progress beyond the trigger level means the player has passed the scene.
            const trigger = triggerLevel(id);
            list.push({ id: id, titleKey: SCENES[id].titleKey, seen: seen.indexOf(id) >= 0 || (trigger > 0 && unlocked > trigger) });
        });
        for (let i = 0; i < IDS.length; i++) {
            if (IDS[i].indexOf(END_PREFIX) === 0 && seen.indexOf(IDS[i]) >= 0) {
                list.push({ id: IDS[i], titleKey: SCENES[IDS[i]].titleKey, seen: true });
                break;
            }
        }
        return list;
    }

    // ---- player state ----
    function create(sceneIds) {
        const ids = [];
        (Array.isArray(sceneIds) ? sceneIds : []).forEach(function (id) {
            if (SCENES[id] && ids.indexOf(id) < 0) ids.push(id);
        });
        return { ids: ids, sceneIndex: 0, panelIndex: 0, lineIndex: 0, t: 0, panelT: 0, chars: 0, lineDone: false, done: ids.length === 0, lineLen: 0, lenSet: false };
    }

    function current(state) {
        if (!state || state.done) return null;
        const s = SCENES[state.ids[state.sceneIndex]];
        const p = s && s.panels[state.panelIndex];
        if (!p) return null;
        return { scene: s, panel: p, lineKey: p.lines[state.lineIndex] || '' };
    }

    // The UI reports the translated length of the current line; the typewriter ends there.
    function setLineLength(state, n) {
        if (!state) return;
        const len = Math.max(0, Math.floor(+n || 0));
        state.lineLen = len;
        state.lenSet = true;
        if (state.chars >= len) {
            state.chars = len;
            state.lineDone = true;
        }
    }

    function step(state, dt) {
        if (!state || state.done) return;
        const d = dt > 0 ? Math.min(dt, MAX_DT) : 0;
        state.t += d;
        state.panelT += d;
        if (state.lineDone) return;
        let n = Math.floor(state.t * TYPE_CPS);
        if (state.lenSet && n >= state.lineLen) {
            n = state.lineLen;
            state.lineDone = true;
        }
        state.chars = n;
    }

    function completeLine(state) {
        if (!state || state.done || state.lineDone) return;
        if (state.lenSet) state.chars = state.lineLen;
        state.lineDone = true;
    }

    function resetLine(state) {
        state.t = 0;
        state.chars = 0;
        state.lineDone = false;
        state.lineLen = 0;
        state.lenSet = false;
    }

    // Completes the line, else moves on. Returns 'complete' | 'line' | 'panel' | 'scene' | 'done' | 'none'.
    function advance(state) {
        if (!state || state.done) return 'none';
        if (!state.lineDone) {
            completeLine(state);
            return 'complete';
        }
        const cur = current(state);
        if (!cur) {
            state.done = true;
            return 'done';
        }
        if (state.lineIndex + 1 < cur.panel.lines.length) {
            state.lineIndex += 1;
            resetLine(state);
            return 'line';
        }
        if (state.panelIndex + 1 < cur.scene.panels.length) {
            state.panelIndex += 1;
            state.lineIndex = 0;
            state.panelT = 0;
            resetLine(state);
            return 'panel';
        }
        if (state.sceneIndex + 1 < state.ids.length) {
            state.sceneIndex += 1;
            state.panelIndex = 0;
            state.lineIndex = 0;
            state.panelT = 0;
            resetLine(state);
            return 'scene';
        }
        state.done = true;
        return 'done';
    }

    function skip(state) {
        if (state) state.done = true;
    }

    root.RunenpfadCutscenes = {
        TYPE_CPS: TYPE_CPS,
        PANEL_SECONDS: PANEL_SECONDS,
        FADE_SECONDS: FADE_SECONDS,
        FG_IDS: FG_IDS,
        FX_KINDS: FX_KINDS,
        SCENES: SCENES,
        IDS: IDS,
        endingFor: endingFor,
        pending: pending,
        chronicle: chronicle,
        create: create,
        current: current,
        setLineLength: setLineLength,
        step: step,
        completeLine: completeLine,
        advance: advance,
        skip: skip
    };
})(typeof window !== 'undefined' ? window : globalThis);
