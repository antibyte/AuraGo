// Storymaker player: the static motion lint of phase 2. It reads the trusted schedule of a storyboard scene (built.log) and the
// plans; nothing is sampled, so each finding is exact. A finding is a warning or a minor issue and never blocks a render. Code
// scenes have no trusted schedule and get no scene lint. Phase 3 moves this into checks.js. Classic script; publishes
// StorymakerCore.lint.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var DEAD_STRETCH_SECONDS = 3, FONT_FLOOR = 0.026, MONOTONY_SHOWS = 3, MAX_TRANSITION_KINDS = 5, EPSILON = 1e-6;
    var SECONDARY_STYLES = { caption: true, label: true };

    function seconds(v) { return v.toFixed(1); }

    function landingOf(entry) { return entry.landing === null || entry.landing === undefined ? entry.start : entry.landing; }

    // holdOf(text entity, hasTitle, factor) = {base, scaled}: a caption or label beside a title is a secondary line.
    function holdOf(e, hasTitle, factor) {
        return SM.motion.holdSeconds(e.text, { secondary: hasTitle && !!SECONDARY_STYLES[e.style], factor: factor });
    }

    // scene(c): c = {scene (compiled, with its frame range), index, plan, built, stage, cat, fmt, envChanged, factor}.
    function scene(c) {
        var out = [], sc = c.scene, index = c.index, log = c.built.log || [];
        if (sc.kind === 'code') return out;
        var T = c.plan.T, n = c.plan.narration, fps = c.fmt.fps, D = (sc.end_frame - sc.start_frame) / fps, vis = c.stage.vis;
        var scenePath = '/scenes/' + index;

        function warn(path, code, message, at) {
            out.push(SM.plan.issue('warning', sc.id, path, code, message, at === undefined ? null : { at_seconds: at }));
        }

        var shows = log.filter(function (l) { return l.kind === 'show' && l.tgt && l.tgt.type === 'text'; });
        var hasTitle = shows.some(function (l) { return l.tgt.style === 'title'; });
        var busy = [];

        shows.forEach(function (l) {
            var landing = landingOf(l), hold = holdOf(l.tgt, hasTitle, c.factor), until = D;
            log.forEach(function (o) { if (o.kind === 'hide' && o.tgt === l.tgt && o.start >= l.start && o.start < until) until = o.start; });
            // the scene end is a whole frame, at most half a frame below the hold that compile asked for
            if (until - landing < hold.base - 0.5 / fps - EPSILON) {
                warn(scenePath + '/beats/' + l.j, 'line_unread', 'This text is on screen for ' + seconds(until - landing) + ' s; it needs ' + seconds(hold.base) + ' s to be read.', landing);
            }
            busy.push([landing, landing + hold.scaled]);
        });

        log.forEach(function (l) {
            if (l.kind !== 'wait' && l.end > l.start) busy.push([l.start, l.end]);
            // a say plays its whole block, also past a shorter explicit duration (frozen 4.6 step 4)
            var spoken = l.kind === 'say' ? n.dialogue[l.j] : null;
            if (spoken) busy.push([l.start, l.start + spoken.duration_ms / 1000]);
        });
        if (n.seconds > 0) busy.push([n.start, n.start + n.seconds]);
        busy.sort(function (a, b) { return a[0] - b[0]; });
        var reached = T;
        function gap(to) {
            if (to - reached > DEAD_STRETCH_SECONDS + EPSILON) warn(scenePath, 'dead_stretch', 'Nothing moves for ' + seconds(to - reached) + ' s here.', reached);
        }
        busy.forEach(function (span) {
            gap(Math.min(span[0], D));
            if (span[1] > reached) reached = span[1];
        });
        gap(D);

        var seen = c.built.entities.some(function (e) {
            if (!(e.alpha.at(T) > 0)) return false;
            if (e.type !== 'actor') return true;
            var x = e.x.at(T), y = e.y.at(T);
            return x >= vis.x && x <= vis.x + vis.w && y >= vis.y && y <= vis.y + vis.h;
        });
        if (!seen) warn(scenePath, 'first_frame_empty', 'The first frame after the transition shows no actor, prop or text.', T);

        var flagged = {};
        shows.forEach(function (l) {
            var style = c.cat.textStyles[l.tgt.style];
            if (!style || flagged[l.tgt.style]) return;
            var size = style.size > 0 ? style.size : 0.05;
            if (size * c.fmt.height / Math.min(c.fmt.width, c.fmt.height) < FONT_FLOOR) {
                flagged[l.tgt.style] = true;
                warn(scenePath + '/beats/' + l.j + '/style', 'text_too_small', 'The text style "' + l.tgt.style + '" is below 2.6 % of the short side of the frame.');
            }
        });

        if (c.plan.transition === 'fade' && index > 0 && c.envChanged) {
            warn(scenePath + '/transition', 'fade_between_worlds', 'This scene fades into another environment. Use cut or wipe between worlds.');
        }
        return out;
    }

    // film(c): c = {scenesIn, plans}. The first scene plays no transition (plan.js transitionFor), so it does not count as a kind.
    function film(c) {
        var out = [], entries = [], kinds = {};
        c.scenesIn.forEach(function (sc, i) {
            if (i > 0 && c.plans[i]) kinds[c.plans[i].requested] = true;
            if (!sc || sc.kind === 'code') return;
            (sc.beats || []).forEach(function (b) {
                if (b && b.do === 'show' && !b.chart && !b.icon) entries.push(b.entry || 'fade');
            });
        });
        if (entries.length >= MONOTONY_SHOWS && entries.every(function (e) { return e === entries[0]; })) {
            out.push(SM.plan.issue('minor', null, '/scenes', 'entry_monotony', 'All ' + entries.length + ' show beats use the entry "' + entries[0] + '".'));
        }
        var count = Object.keys(kinds).length;
        if (count > MAX_TRANSITION_KINDS) {
            out.push(SM.plan.issue('minor', null, '/scenes', 'transition_variety', 'The film uses ' + count + ' transition kinds; ' + MAX_TRANSITION_KINDS + ' or fewer read as a style.'));
        }
        return out;
    }

    SM.lint = { scene: scene, film: film };
})(typeof globalThis !== 'undefined' ? globalThis : self);
