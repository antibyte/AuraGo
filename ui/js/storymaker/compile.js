// Storymaker player: compile(story, audioInfo, catalog, opts) -> {timeline, report, schedule, needs}. It is a pure
// function of its inputs. Scenes get whole-frame ranges (transitions overlap: a scene starts its entry transition before
// the previous one ends), storyboard scenes become entities with tracks (beats.js), code scenes are placeholders whose
// duration comes from the code-scene worker (opts.codeScenes). The report is what Go receives from a render job:
// {frame_count, fps, width, height, duration_ms, catalog_version, scenes, audio_events, captions, checks, unverified,
// issues}. Defaults are normally applied by Go, but every optional field may be missing here. Classic script; publishes
// StorymakerCore.compile.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    // fontKey(cat, name, weight) = 'font:<name>/<weight>' of the catalog font that fits best, or null.
    function fontKey(cat, name, weight) {
        var best = null;
        cat.fonts.forEach(function (f) {
            if (f.name !== name) return;
            if (!best || Math.abs(f.weight - weight) < Math.abs(best.weight - weight)) best = f;
        });
        return best ? 'font:' + best.name + '/' + best.weight : null;
    }

    // cuePrecision(scenes) = the share of cues placed by a span, or null when the film has no cue.
    function cuePrecision(scenes) {
        var total = 0, spans = 0;
        scenes.forEach(function (s) {
            s.narration.cueList.forEach(function (c) { total++; if (c.placed === 'span') spans++; });
        });
        return total ? spans / total : null;
    }

    // dropPastFilmEnd(events, filmMs, scenes, issues): the audio events that start inside the film. A scene that lost an event
    // gets one warning; the warning of a duration shorter than the narration already is that one.
    function dropPastFilmEnd(events, filmMs, scenes, issues) {
        var kept = [], lost = {};
        events.forEach(function (e) {
            if (e.start_ms < filmMs) kept.push(e); else lost[e.scene_id] = true;
        });
        scenes.forEach(function (s) {
            if (!lost[s.id]) return;
            var path = '/scenes/' + s.index + '/duration';
            if (issues.some(function (i) { return i.path === path && i.code === 'invalid_value'; })) return;
            issues.push(SM.plan.issue('warning', s.id, path, 'invalid_value', 'Audio of this scene starts after the end of the film and is not played. Shorten the speech or extend the scene.'));
        });
        return kept;
    }

    function compile(story, audioInfo, catalog, opts) {
        opts = opts || {};
        story = story || {};
        var P = SM.plan, issues = [], needs = {};
        var cat = SM.scene.indexCatalog(catalog);
        var fmt = P.normaliseFormat(story, opts);
        var fps = fmt.fps;
        var palette = P.pickPalette(story, cat, issues);
        var families = P.fontFamilies(cat);
        var scenesIn = Array.isArray(story.scenes) ? story.scenes : [];
        var castMap = {};
        (story.cast || []).forEach(function (m) { if (m && m.id) castMap[m.id] = m; });
        if (!scenesIn.length) issues.push(P.issue('error', null, '/scenes', 'missing_field', 'The story has no scenes.'));

        var plans = P.planAll({ scenes: scenesIn }, audioInfo || {}, cat, fmt, issues);
        var scenes = [], envPrev = null, narrationEvents = [], audioEvents = [];
        var styleFont = story.style && story.style.font || 'default';
        var captionsOn = !(story.style && story.style.captions === false);
        var glyphIdx = P.glyphIndex(catalog), fontChain = glyphIdx ? SM.glyphs.chain(glyphIdx, '$style') : undefined;
        var catalogFamilies = {};
        Object.keys(families).forEach(function (k) { catalogFamilies[families[k]] = true; });
        // needGlyphs(text): one font key per fallback face the text draws from. A face with a catalog font's family adds
        // nothing, because catalog.fonts loads it.
        function needGlyphs(text) {
            SM.glyphs.needsFor(glyphIdx, text).forEach(function (face) {
                if (!catalogFamilies[face.family]) needs[SM.glyphs.key(face)] = true;
            });
        }
        if (glyphIdx) needGlyphs(String.fromCharCode(0x2026));
        var rhythmFactor = SM.motion.factor(story.style);

        scenesIn.forEach(function (sc, i) {
            var plan = plans[i], isCode = sc.kind === 'code';
            var envId = sc.environment || envPrev || 'env.plain-stage';
            var envChanged = i > 0 && envId !== envPrev;
            envPrev = envId;
            var envEntry = cat.byId[envId];
            if (!isCode) {
                if (!envEntry || envEntry.kind !== 'environment') {
                    issues.push(P.issue('warning', sc.id, '/scenes/' + i + '/environment', 'unknown_asset', 'Unknown environment "' + envId + '"; the player draws a plain background.'));
                    envEntry = null;
                } else if (envEntry.file) needs['catalog:' + envEntry.file] = true;
            } else envEntry = null;
            var variant = sc.variant === 'night' ? 'night' : 'day';
            var stage = SM.stage.create({ aspect: fmt.aspect, width: fmt.width, height: fmt.height, env: envEntry, safe: catalog && catalog.stage && catalog.stage.safe });
            var built = { entities: [], camera: SM.camera.create(1, stage.vis.x + stage.vis.w / 2, stage.vis.y + stage.vis.h / 2), beatsEnd: 0, motionEnd: 0, blocksEnd: 0, holdsEnd: 0 };
            var codeInfo = null;
            if (isCode) {
                codeInfo = opts.codeScenes ? (opts.codeScenes[sc.id] || { error: { message: 'No code-scene bundle for scene "' + sc.id + '".' } }) : null;
                if (codeInfo && codeInfo.error) {
                    var pos = codeInfo.error.line ? ' (line ' + codeInfo.error.line + (codeInfo.error.column ? ', column ' + codeInfo.error.column : '') + ')' : '';
                    issues.push(P.issue('error', sc.id, '/scenes/' + i, codeInfo.error.code === 'deadline' ? 'deadline' : 'code_error', String(codeInfo.error.message || 'Code scene failed') + pos));
                }
                (sc.beats || []).forEach(function (b, j) {
                    issues.push(P.issue('minor', sc.id, '/scenes/' + i + '/beats/' + j, 'not_supported_yet', 'Beats of a code scene are ignored; the code scene draws everything.'));
                });
            } else {
                built = SM.beats.buildStoryboard({ story: story, scene: sc, index: i, plan: plan, cat: cat, fmt: fmt, stage: stage, issues: issues, needs: needs, castMap: castMap });
            }

            // duration (scene-local seconds, origin = start of the entry transition)
            var T = plan.T, n = plan.narration, tail = n.seconds > 0 ? SM.narration.TAIL_SECONDS : 0;
            var autoD = Math.max(T + n.seconds + tail, built.blocksEnd > 0 ? built.blocksEnd + SM.narration.TAIL_SECONDS : 0, built.beatsEnd, built.holdsEnd, T + (n.seconds > 0 ? 0 : P.MIN_SCENE_SECONDS));
            var D;
            if (typeof sc.duration === 'number' && sc.duration > 0) {
                D = sc.duration;
                if (n.seconds > 0 && D < T + n.seconds - 0.05) {
                    issues.push(P.issue('warning', sc.id, '/scenes/' + i + '/duration', 'invalid_value',
                        'The scene duration ' + D + ' s is shorter than its narration (' + (T + n.seconds).toFixed(1) + ' s including the transition).'));
                }
            } else if (isCode && codeInfo && codeInfo.duration > 0) {
                // With narration and without api.duration the scene also covers that narration. A silent scene keeps the length of its motion. A length the author set stays, with a warning when the speech is longer.
                D = Math.min(P.MAX_CODE_SECONDS, Math.max(0.5, !codeInfo.explicit && n.seconds > 0 ? Math.max(codeInfo.duration, autoD) : codeInfo.duration));
                if (codeInfo.explicit && n.seconds > 0 && D < T + n.seconds - 0.05) {
                    issues.push(P.issue('warning', sc.id, '/scenes/' + i + '/duration', 'invalid_value',
                        'The code scene sets api.duration(' + D + '), which is shorter than its narration (' + (T + n.seconds).toFixed(1) + ' s including the transition).'));
                }
            } else D = autoD;
            var dFrames = Math.max(SM.schedule.secondsToFrames(fps, D), plan.tf + 1);
            var prev = scenes.length ? scenes[scenes.length - 1] : null;
            var startFrame = prev ? Math.max(prev.start_frame + 1, prev.end_frame - plan.tf) : 0;

            var scene = {
                id: sc.id, index: i, kind: isCode ? 'code' : 'storyboard', start_frame: startFrame, end_frame: startFrame + dFrames,
                tf: plan.tf, transition: plan.transition, requested_transition: plan.requested, env: envEntry, variant: variant,
                narration: n, entities: built.entities, camera: built.camera, framing: built.framing,
                code: codeInfo ? { error: codeInfo.error || null, duration: codeInfo.duration || null } : null, stages: {}
            };
            scenes.push(scene);
            issues.push.apply(issues, SM.lint.scene({ scene: scene, index: i, plan: plan, built: built, stage: stage, cat: cat, fmt: fmt, envChanged: envChanged, factor: rhythmFactor }));

            // narration windows (global seconds) and, with measured audio, one audio event per piece
            var base = scene.start_frame / fps;
            if (n.seconds > 0 && !sc.silent) {
                if (n.measured) {
                    var blockMs = Math.round((base + n.start) * 1000);
                    n.block.pieces.forEach(function (p, k) {
                        narrationEvents.push({ kind: 'narration', scene_id: sc.id, ref: p.hash, start: base + n.start + p.at_ms / 1000, end: base + n.start + (p.at_ms + p.out_ms - p.in_ms) / 1000 });
                        audioEvents.push({ kind: 'narration', ref: p.hash, start_ms: blockMs + p.at_ms, scene_id: sc.id, block_id: n.block.id, piece: k });
                    });
                } else {
                    narrationEvents.push({ kind: 'narration', scene_id: sc.id, ref: '', start: base + n.start, end: base + n.start + n.seconds });
                }
            }

            // fallback faces of every text this scene draws; the narration only when captions or a say without text draws it
            if (glyphIdx) {
                var saysNoText = false;
                scene.entities.forEach(function (e) { if (e.type === 'text') needGlyphs(e.text); });
                if (!isCode) (sc.beats || []).forEach(function (b) {
                    if (!b || b.do !== 'say') return;
                    var said = SM.narration.parse(b.text);
                    if (said.words > 0) needGlyphs(said.plain); else saysNoText = true;
                });
                if (n.plain && (captionsOn || saysNoText)) needGlyphs(n.plain);
            }

            // fonts in use
            scene.entities.forEach(function (e) {
                if (e.type !== 'text') return;
                var style = cat.textStyles[e.style] || cat.textStyles.title || {};
                var name = !style.font || style.font === 'default' ? styleFont : style.font;
                var key = fontKey(cat, name, style.weight || 400) || fontKey(cat, 'default', style.weight || 400);
                if (key) needs[key] = true;
            });
            if (scene.entities.some(function (e) { return e.type === 'actor'; }) && catalog && catalog.rig && catalog.rig.file) needs['catalog:' + catalog.rig.file] = true;
        });
        if (scenes.some(function (s) { return s.kind === 'code'; }) && catalog && catalog.rig && catalog.rig.file) needs['catalog:' + catalog.rig.file] = true;

        issues.push.apply(issues, SM.lint.film({ scenesIn: scenesIn, plans: plans }));

        var frameCount = 0;
        scenes.forEach(function (s) { if (s.end_frame > frameCount) frameCount = s.end_frame; });
        var filmMs = Math.round(frameCount * 1000 / fps);
        audioEvents = dropPastFilmEnd(audioEvents, filmMs, scenes, issues);
        var schedule = {
            fps: fps, frame_count: frameCount,
            scenes: scenes.map(function (s) { return { id: s.id, kind: s.kind, start_frame: s.start_frame, end_frame: s.end_frame, tf: s.tf, transition: s.transition }; })
        };
        var report = {
            frame_count: frameCount, fps: fps, width: fmt.width, height: fmt.height,
            duration_ms: Math.round(frameCount * 1000 / fps), catalog_version: cat.version,
            scenes: scenes.map(function (s) { return { id: s.id, start_frame: s.start_frame, end_frame: s.end_frame, kind: s.kind, cues: s.narration.cueList }; }),
            audio_events: audioEvents, captions: [], cue_precision: cuePrecision(scenes), checks: SM.checks.run(scenes), unverified: SM.checks.unverified(scenes), issues: issues
        };
        var timeline = {
            version: 1, fps: fps, width: fmt.width, height: fmt.height, aspect: fmt.aspect, frame_count: frameCount,
            catalog_version: cat.version, palette: palette, style: { font: styleFont, captions: !(story.style && story.style.captions === false) },
            fontFamilies: families, fontChain: fontChain, cat: cat, rigFile: catalog && catalog.rig && catalog.rig.file || '', safe: catalog && catalog.stage && catalog.stage.safe || null,
            scenes: scenes, schedule: schedule, narrationEvents: narrationEvents
        };
        return { timeline: timeline, report: report, schedule: schedule, needs: Object.keys(needs).sort() };
    }

    // overlappingNarration(timeline) = pairs of narration events whose windows overlap (an empty list is the goal).
    function overlappingNarration(timeline) {
        var ev = timeline.narrationEvents.slice().sort(function (a, b) { return a.start - b.start; }), out = [];
        for (var i = 1; i < ev.length; i++) if (ev[i].start < ev[i - 1].end - 1e-9) out.push([ev[i - 1], ev[i]]);
        return out;
    }

    SM.compile = compile;
    SM.overlappingNarration = overlappingNarration;
})(typeof globalThis !== 'undefined' ? globalThis : self);
