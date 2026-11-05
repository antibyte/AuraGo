// Storymaker player: timing plan. The scene-local timing rule of the spec: scene time 0 is the start of the scene's entry
// transition; the narration starts when that transition ends; cue and beat times use the same origin. This file computes,
// per scene and without looking at any beat, the transition frames, the narration window and the cue times. The host asks
// for it (planTiming) before it builds the code scenes, because api.cue(n) needs the cue times. Also the shared issue helper
// and the phase-1 support tables. Classic script; publishes StorymakerCore.plan.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // Closed name sets of the schema (contract section 2).
    var KNOWN = {
        poses: 'stand sit typing point-left point-right wave arms-crossed hands-on-hips shrug think present thumbs-up celebrate facepalm phone hold lean'.split(' '),
        gestures: 'point wave shrug think present thumbs-up celebrate facepalm count'.split(' '),
        expressions: 'neutral happy laugh sad angry surprised confused thinking worried determined'.split(' '),
        transitions: 'cut fade slide-left slide-right slide-up slide-down wipe zoom iris whip'.split(' ')
    };
    // What the phase-1 player implements.
    var SUPPORTED_TRANSITIONS = ['cut', 'fade', 'slide-left', 'wipe'];
    var SUPPORTED_BEATS = ['enter', 'exit', 'walk', 'move', 'pose', 'gesture', 'emote', 'look', 'show', 'hide', 'highlight', 'camera', 'wait'];
    var DEFAULT_TRANSITION_SECONDS = { 'cut': 0, 'fade': 0.5, 'slide-left': 0.5, 'slide-right': 0.5, 'slide-up': 0.5, 'slide-down': 0.5, 'wipe': 0.5, 'zoom': 0.6, 'iris': 0.6, 'whip': 0.35 };
    var MIN_SCENE_SECONDS = 1.0;
    var MAX_CODE_SECONDS = 600;

    function issue(severity, sceneId, path, code, message, extra) {
        var out = { severity: severity, path: path, code: code, message: message };
        if (sceneId) out.scene_id = sceneId;
        if (extra) for (var k in extra) if (extra[k] !== undefined) out[k] = extra[k];
        return out;
    }

    function contains(list, v) { return list.indexOf(v) >= 0; }

    var SIZES = { '16:9': [1920, 1080], '9:16': [1080, 1920], '1:1': [1080, 1080] };

    // normaliseFormat(story, opts) = {aspect, fps, width, height}; opts.width scales the canvas, keeping the aspect.
    function normaliseFormat(story, opts) {
        var f = (story && story.format) || {};
        var aspect = SIZES[f.aspect] ? f.aspect : '16:9';
        var fps = f.fps === 24 || f.fps === 25 || f.fps === 30 ? f.fps : 30;
        var size = SM.stage.sizeFor(aspect, opts && opts.width);
        if (opts && opts.width > 0 && opts.height > 0) size = { width: Math.round(opts.width), height: Math.round(opts.height) };
        return { aspect: aspect, fps: fps, width: size.width, height: size.height };
    }

    // transitionFor(scene, index, cat, fps, issues): the entry transition of a scene. Scene 0 has none. A name the player
    // does not implement falls back to fade with the same duration and a minor not_supported_yet issue.
    function transitionFor(scene, index, cat, fps, issues) {
        if (index === 0) return { kind: 'cut', requested: scene.transition || 'cut', tf: 0 };
        var req = scene.transition || 'fade', effective = req, fallback = false;
        var entry = cat.transitions[req];
        var seconds = entry && typeof entry.duration === 'number' ? entry.duration : DEFAULT_TRANSITION_SECONDS[req];
        if (seconds === undefined) seconds = 0.5;
        if (!contains(SUPPORTED_TRANSITIONS, req)) {
            effective = 'fade';
            fallback = true;
            if (contains(KNOWN.transitions, req)) {
                issues.push(issue('minor', scene.id, '/scenes/' + index + '/transition', 'not_supported_yet',
                    'Transition "' + req + '" is not supported yet; the player uses a fade.'));
            } else {
                issues.push(issue('warning', scene.id, '/scenes/' + index + '/transition', 'unknown_name',
                    'Unknown transition "' + req + '"; the player uses a fade.'));
            }
        }
        var tf = effective === 'cut' ? 0 : SM.schedule.secondsToFrames(fps, seconds);
        return { kind: effective, requested: req, tf: tf < 0 ? 0 : tf, fallback: fallback };
    }

    var FALLBACK_PALETTE = { bg: '#fff7e6', bg2: '#ffe0a3', fg: '#1f2a44', accent: '#ff7a45', accent2: '#36cfc9', text: '#1f2a44', outline: '#1f2a44' };

    function pickPalette(story, cat, issues) {
        var want = story.style && story.style.palette || 'sunny';
        var pal = cat.palettes[want];
        if (!pal) {
            var names = Object.keys(cat.palettes);
            if (names.length) {
                issues.push(issue('warning', null, '/style/palette', 'unknown_name', 'Unknown palette "' + want + '"; the player uses "' + names[0] + '".'));
                pal = cat.palettes[names[0]];
            }
        }
        var tokens = {}, k;
        for (k in FALLBACK_PALETTE) tokens[k] = FALLBACK_PALETTE[k];
        if (pal && pal.tokens) for (k in pal.tokens) tokens[k] = pal.tokens[k];
        return tokens;
    }

    // glyphIndex(catalog): the decoded coverage of catalog.glyph_data, or null. That key exists only in memory, never in catalog.json.
    function glyphIndex(catalog) {
        return catalog && catalog.glyph_data ? SM.glyphs.index(catalog.glyph_data) : null;
    }

    function fontFamilies(cat) {
        var out = {};
        cat.fonts.forEach(function (f) { if (f && f.name && !out[f.name]) out[f.name] = f.family; });
        return out;
    }

    // planScene(...) = {tf, T, kind, requested, narration: {text, plain, words, start, seconds, measured, blocks, cues}}
    function planScene(story, audioInfo, cat, fmt, index, issues) {
        var scene = story.scenes[index];
        var tr = transitionFor(scene, index, cat, fmt.fps, issues);
        var T = tr.tf / fmt.fps;
        var silent = scene.silent === true;
        var parsed = SM.narration.parse(silent ? '' : scene.narration);
        var info = audioInfo && audioInfo.narration && audioInfo.narration[scene.id];
        var blocks = info && info.blocks && info.blocks.length ? info.blocks : null;
        var narration = { plain: parsed.plain, words: parsed.words, start: T, seconds: 0, measured: false, blocks: [], cues: {} };
        if (parsed.words > 0) {
            if (blocks) {
                var at = T;
                narration.measured = true;
                blocks.forEach(function (b) {
                    var d = (b.duration_ms > 0 ? b.duration_ms : 0) / 1000;
                    narration.blocks.push({ hash: b.hash, start: at, seconds: d });
                    at += d;
                });
                narration.seconds = at - T;
                narration.cues = SM.narration.cueTimes(parsed, T, narration.blocks[0].seconds);
            } else {
                narration.seconds = SM.narration.estimate(parsed.words);
                narration.cues = SM.narration.cueTimes(parsed, T, narration.seconds);
                issues.push(issue('warning', scene.id, '/scenes/' + index + '/narration', 'narration_missing',
                    'No narration audio for this scene; the duration is estimated at ' + narration.seconds.toFixed(1) + ' s.'));
            }
        }
        return { index: index, tf: tr.tf, T: T, transition: tr.kind, requested: tr.requested, narration: narration };
    }

    function planAll(story, audioInfo, cat, fmt, issues) {
        var out = [];
        for (var i = 0; i < story.scenes.length; i++) out.push(planScene(story, audioInfo, cat, fmt, i, issues));
        return out;
    }

    // planTiming(story, audioInfo, catalog, opts) = {fps, scenes: {<scene id>: {index, transition_frames, narration_start,
    // narration_seconds, cues: {n: seconds}}}, issues, language, fontChain}. Pure; used by the host before it builds code scenes.
    // fontChain stays undefined without glyph data.
    function planTiming(story, audioInfo, catalog, opts) {
        var cat = SM.scene.indexCatalog(catalog), issues = [];
        var fmt = normaliseFormat(story, opts);
        var plans = planAll(story, audioInfo || {}, cat, fmt, issues), scenes = {}, glyphIdx = glyphIndex(catalog);
        plans.forEach(function (p) {
            var s = story.scenes[p.index];
            scenes[s.id] = {
                index: p.index, kind: s.kind === 'code' ? 'code' : 'storyboard', transition_frames: p.tf,
                narration_start: p.narration.start, narration_seconds: p.narration.seconds, cues: p.narration.cues
            };
        });
        return {
            fps: fmt.fps, width: fmt.width, height: fmt.height, aspect: fmt.aspect, scenes: scenes, issues: issues,
            palette: pickPalette(story, cat, []), fontFamilies: fontFamilies(cat), styleFont: story.style && story.style.font || 'default',
            language: story.voice && story.voice.language || '', fontChain: glyphIdx ? SM.glyphs.chain(glyphIdx, '$style') : undefined
        };
    }

    SM.plan = {
        KNOWN: KNOWN, SUPPORTED_TRANSITIONS: SUPPORTED_TRANSITIONS, SUPPORTED_BEATS: SUPPORTED_BEATS,
        MIN_SCENE_SECONDS: MIN_SCENE_SECONDS, MAX_CODE_SECONDS: MAX_CODE_SECONDS, issue: issue, contains: contains,
        normaliseFormat: normaliseFormat, pickPalette: pickPalette, fontFamilies: fontFamilies, transitionFor: transitionFor, planScene: planScene, planAll: planAll,
        planTiming: planTiming, glyphIndex: glyphIndex
    };
    SM.planTiming = planTiming;
})(typeof globalThis !== 'undefined' ? globalThis : self);
