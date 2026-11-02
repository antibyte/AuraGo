// Storymaker player: storyboard scenes. buildStoryboard turns one scene (actors, props, camera, beats) into entities
// with tracks. Beats run in order; "with": "prev" starts a beat together with the previous one; "cue": n starts it at
// narration marker {n}; delay, duration and ease are optional. Timing defaults are owned here (enter 0.6 s, walk from the
// distance at the catalog's walk speed, ...). Phase-1 support matrix: enter exit walk move pose gesture emote look show
// (item and text) hide camera wait; the other beats are reported as minor not_supported_yet and ignored. Classic script;
// publishes StorymakerCore.beats.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var ACTOR_REACH = 200;   // design pixels from the feet to the visual centre of a standing actor at scale 1
    var DEFAULT_ACTOR_SPOTS = ['left', 'right', 'far-left', 'far-right'];
    var TEXT_ROW_Y = 28;     // percent of the frame height: texts shown at a floor spot sit in the upper third

    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }

    function buildStoryboard(c) {
        var P = SM.plan, S = SM.scene, scene = c.scene, index = c.index, sid = scene.id, stage = c.stage, cat = c.cat;
        var issues = c.issues, plan = c.plan, fmt = c.fmt;
        var entities = [], byId = {}, byItem = {}, counters = {};
        var cam = SM.camera.create(1, stage.vis.x + stage.vis.w / 2, stage.vis.y + stage.vis.h / 2);
        var beats = scene.beats || [];
        var placed = 0;

        function warn(severity, path, code, message) { issues.push(P.issue(severity, sid, path, code, message)); }
        function beatPath(j, field) { return '/scenes/' + index + '/beats/' + j + (field ? '/' + field : ''); }

        function resolveAt(at, path, fallback) {
            var pos = at === undefined || at === null ? null : stage.resolve(at);
            if (!pos) {
                if (at !== undefined && at !== null) warn('warning', path, 'unknown_spot', 'Unknown position ' + JSON.stringify(at) + '; the player uses ' + fallback + '.');
                pos = stage.resolve(fallback);
            }
            return pos;
        }

        // A known name that the catalog does not have is a minor not_supported_yet and falls back; an unknown name is a warning.
        function nameOr(map, known, name, fallback, path, what) {
            if (map[name]) return name;
            if (P.contains(known, name)) warn('minor', path, 'not_supported_yet', what + ' "' + name + '" is not in the catalog yet; the player uses "' + fallback + '".');
            else warn('warning', path, 'unknown_name', 'Unknown ' + what.toLowerCase() + ' "' + name + '"; the player uses "' + fallback + '".');
            return fallback;
        }

        function register(e, item) {
            entities.push(e);
            byId[e.id] = e;
            if (item) (byItem[item] = byItem[item] || []).push(e);
        }

        function instanceId(item, given) {
            if (given) return given;
            var slug = String(item).replace(/^[a-z]+\./, '');
            counters[slug] = (counters[slug] || 0) + 1;
            return slug + '-' + counters[slug];
        }

        function find(ref) {
            if (!ref) return null;
            if (byId[ref]) return byId[ref];
            var list = byItem[ref];
            return list && list.length ? list[list.length - 1] : null;
        }

        function actorOf(ref, path) {
            var e = find(ref);
            if (e && e.type === 'actor') return e;
            warn('warning', path, 'unknown_cast', 'No actor "' + ref + '" in this scene.');
            return null;
        }

        function needFile(entry) { if (entry && entry.file) c.needs['catalog:' + entry.file] = true; }

        // ---- actors and props that exist from the first frame
        var entering = {};
        beats.forEach(function (b) { if (b && b.do === 'enter' && b.who) entering[b.who] = true; });

        function addActor(def, k, total, hidden) {
            var path = '/scenes/' + index + '/actors/' + k;
            var member = c.castMap[def.cast];
            if (!member) { warn('warning', path + '/cast', 'unknown_cast', 'Unknown cast member "' + def.cast + '".'); return; }
            if (byId[def.cast]) { warn('warning', path + '/cast', 'duplicate_id', 'Cast member "' + def.cast + '" is already in this scene.'); return; }
            var ch = cat.byId[member.character];
            if (!ch) { warn('warning', path + '/cast', 'unknown_asset', 'Unknown character "' + member.character + '".'); return; }
            var at = def.at !== undefined && def.at !== null ? def.at : (total <= 1 ? 'center' : DEFAULT_ACTOR_SPOTS[k % DEFAULT_ACTOR_SPOTS.length]);
            var pos = resolveAt(at, path + '/at', 'center');
            var facing = def.facing === 'left' ? -1 : (def.facing === 'right' ? 1 : (pos.face === 'left' ? -1 : (pos.face === 'right' ? 1 : (pos.x <= stage.focusX ? 1 : -1))));
            var pose = nameOr(cat.poses, P.KNOWN.poses, def.pose || 'stand', 'stand', path + '/pose', 'Pose');
            var expr = nameOr(cat.expressions, P.KNOWN.expressions, def.expression || 'neutral', 'neutral', path + '/expression', 'Expression');
            var e = S.createActor({
                id: def.cast, cast: def.cast, character: member.character, params: ch.params || {}, x: pos.x, y: pos.y,
                scale: def.scale, facing: facing, pose: pose, expression: expr, hidden: hidden, depth: pos.depth, phase: k * 0.37
            });
            register(e, null);
            placed++;
        }

        var actorDefs = scene.actors || [];
        actorDefs.forEach(function (a, k) { addActor(a, k, actorDefs.length, !!entering[a.cast]); });
        Object.keys(entering).forEach(function (cast) { if (!byId[cast]) addActor({ cast: cast, at: 'center' }, actorDefs.length, actorDefs.length + 1, true); });

        (scene.props || []).forEach(function (p, k) {
            var path = '/scenes/' + index + '/props/' + k;
            var entry = cat.byId[p.item];
            if (!entry || entry.kind !== 'prop') { warn('warning', path + '/item', 'unknown_asset', 'Unknown prop "' + p.item + '".'); return; }
            var pos = resolveAt(p.at, path + '/at', 'center');
            needFile(entry);
            register(S.createProp({ id: instanceId(p.item, p.id), item: p.item, x: pos.x, y: pos.y, scale: p.scale, layer: p.layer, depth: pos.depth }), p.item);
        });

        // ---- camera start
        var framing = scene.camera && SM.camera.FRAMING[scene.camera.framing] ? scene.camera.framing : 'wide';

        function focusOf(ref, t) {
            var e = find(ref);
            if (e) {
                var depth = stage.depthScale(e.y.at(t), e.depth);
                var reach = e.type === 'actor' ? ACTOR_REACH * depth * (e.baseScale || 1) : (e.type === 'prop' ? 70 * depth : 0);
                return { x: e.x.at(t), y: e.y.at(t) - reach };
            }
            var s = typeof ref === 'string' ? stage.spot(ref) : null;
            return s ? { x: s.x, y: s.y - 150 } : null;
        }

        var firstActor = null;
        for (var q = 0; q < entities.length; q++) if (entities[q].type === 'actor') { firstActor = entities[q]; break; }
        var startFocus = scene.camera && scene.camera.target ? focusOf(scene.camera.target, 0) : null;
        if (!startFocus && framing !== 'wide' && firstActor) startFocus = focusOf(firstActor.id, 0);
        cam.zoom.init = SM.camera.FRAMING[framing];
        if (startFocus) { cam.cx.init = startFocus.x; cam.cy.init = startFocus.y; }

        // ---- beats
        function walkSeconds(from, to) {
            var cyc = cat.cycles.walk, speed = cyc && cyc.speed > 0 ? cyc.speed : 12;
            var dx = to.x - from.x, dy = to.y - from.y, dist = Math.sqrt(dx * dx + dy * dy);
            var depth = (stage.depthScale(from.y) + stage.depthScale(to.y)) / 2;
            return Math.max(0.3, dist / (speed / 100 * SM.stage.DESIGN_W * depth));
        }

        function easeOf(b, fallback) { return SM.isEase(b.ease) ? b.ease : fallback; }

        function moveTo(e, t0, t1, to, ease) {
            e.x.add(t0, t1, to.x, ease);
            e.y.add(t0, t1, to.y, ease);
        }

        function faceToward(a, t, x) {
            var from = a.x.at(t);
            if (Math.abs(x - from) > 1) a.facing.set(t, x > from ? 1 : -1);
        }

        function entryAnim(e, b, t0, dur, pos) {
            var entry = b.entry || 'fade';
            if (entry === 'none' || dur <= 0) { e.alpha.add(t0, t0, 1, 'linear'); return; }
            if (entry === 'pop' && e.type !== 'text') {
                e.alpha.add(t0, t0 + dur, 1, 'ease-out');
                e.scale.add(t0, t0 + dur, 1, 'back-out', 0.6);
            } else if (entry === 'slide') {
                e.alpha.add(t0, t0 + dur, 1, 'ease-out');
                e.x.add(t0, t0 + dur, pos.x, 'ease-out', pos.x - 140);
            } else {
                e.alpha.add(t0, t0 + dur, 1, 'ease-out');
            }
        }

        var kinds = {
            enter: {
                dur: function () { return 0.6; },
                run: function (b, j, t0, dur) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a) return;
                    var to = resolveAt(b.to, beatPath(j, 'to'), 'center'), from;
                    if (b.from === 'right') from = { x: stage.offRight(), y: to.y };
                    else if (b.from === 'door') { var d = stage.spot('door'); from = d ? { x: d.x, y: d.y } : { x: stage.offRight(), y: to.y }; }
                    else from = { x: stage.offLeft(), y: to.y };
                    a.x.add(t0, t0 + dur, to.x, easeOf(b, 'ease-out'), from.x);
                    a.y.add(t0, t0 + dur, to.y, easeOf(b, 'ease-out'), from.y);
                    a.alpha.add(t0, t0, 1, 'linear');
                    a.facing.set(t0, to.x >= from.x ? 1 : -1);
                    a.look.set(t0, null);
                    a.walks.add(t0, t0 + dur, {});
                    if (to.face) a.facing.set(t0 + dur, to.face === 'left' ? -1 : 1);
                    if (typeof to.depth === 'number') a.depth = to.depth;
                }
            },
            exit: {
                dur: function () { return 0.6; },
                run: function (b, j, t0, dur) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a) return;
                    var to = b.to ? resolveAt(b.to, beatPath(j, 'to'), 'door') : { x: stage.offRight(), y: a.y.at(t0) };
                    faceToward(a, t0, to.x);
                    a.look.set(t0, null);
                    moveTo(a, t0, t0 + dur, to, easeOf(b, 'ease-in'));
                    a.walks.add(t0, t0 + dur, {});
                    a.alpha.add(t0 + dur - Math.min(0.2, dur), t0 + dur, 0, 'ease-in');
                }
            },
            walk: {
                dur: function (b, t0) {
                    var e = find(b.who || b.target);
                    var to = b.to !== undefined ? stage.resolve(b.to) : null;
                    if (!e || !to) return 0.6;
                    return walkSeconds({ x: e.x.at(t0), y: e.y.at(t0) }, to);
                },
                run: function (b, j, t0, dur) { moveBeat(b, j, t0, dur, true); }
            },
            move: {
                dur: function () { return 0.8; },
                run: function (b, j, t0, dur) { moveBeat(b, j, t0, dur, false); }
            },
            pose: {
                dur: function () { return 0.3; },
                run: function (b, j, t0, dur) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a || !b.pose) return;
                    a.pose.set(t0, nameOr(cat.poses, P.KNOWN.poses, b.pose, 'stand', beatPath(j, 'pose'), 'Pose'), dur);
                }
            },
            gesture: {
                dur: function (b) {
                    var entry = cat.gestures[b.gesture], count = clamp(Math.round(b.count || 1), 1, 5);
                    return (entry && entry.duration > 0 ? entry.duration : 0.5) * count;
                },
                run: function (b, j, t0, dur) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a) return;
                    var name = nameOr(cat.gestures, P.KNOWN.gestures, b.gesture, '', beatPath(j, 'gesture'), 'Gesture');
                    if (!name) return;
                    a.gestures.add(t0, t0 + dur, { name: name, count: clamp(Math.round(b.count || 1), 1, 5) });
                    var f = b.target ? focusOf(b.target, t0) : null;
                    if (f) faceToward(a, t0, f.x);
                }
            },
            emote: {
                dur: function () { return 0.25; },
                run: function (b, j, t0) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a || !b.expression) return;
                    a.expr.set(t0, nameOr(cat.expressions, P.KNOWN.expressions, b.expression, 'neutral', beatPath(j, 'expression'), 'Expression'));
                }
            },
            look: {
                dur: function () { return 0.3; },
                run: function (b, j, t0) {
                    var a = actorOf(b.who, beatPath(j, 'who'));
                    if (!a) return;
                    if (b.target === 'camera') { a.look.set(t0, 'camera'); return; }
                    a.look.set(t0, null);
                    var f = focusOf(b.target, t0);
                    if (f) faceToward(a, t0, f.x);
                }
            },
            show: {
                dur: function (b) { return (b.entry || 'fade') === 'none' ? 0 : 0.5; },
                run: function (b, j, t0, dur) {
                    if (b.chart || b.icon) {
                        warn('minor', beatPath(j), 'not_supported_yet', 'Show with ' + (b.chart ? 'a chart' : 'an icon') + ' is not supported yet; the beat is ignored.');
                        return;
                    }
                    if (b.text !== undefined && b.text !== null && b.item === undefined) {
                        var pos = textPosition(b.at, beatPath(j, 'at'));
                        var style = b.style && cat.textStyles[b.style] ? b.style : 'title';
                        if (b.style && !cat.textStyles[b.style] && !P.contains(['speech', 'thought'], b.style)) {
                            warn('warning', beatPath(j, 'style'), 'unknown_name', 'Unknown text style "' + b.style + '"; the player uses "title".');
                        }
                        var t = S.createText({ id: instanceId('text.text', b.id), text: b.text, style: style, x: pos.x, y: pos.y, hidden: true });
                        register(t, null);
                        entryAnim(t, b, t0, dur, pos);
                        return;
                    }
                    if (!b.item) { warn('warning', beatPath(j), 'missing_field', 'A show beat needs item, text, icon or chart.'); return; }
                    var entry = cat.byId[b.item];
                    if (!entry || entry.kind !== 'prop') { warn('warning', beatPath(j, 'item'), 'unknown_asset', 'Unknown prop "' + b.item + '".'); return; }
                    var at = resolveAt(b.at, beatPath(j, 'at'), 'center');
                    needFile(entry);
                    var prop = S.createProp({ id: instanceId(b.item, b.id), item: b.item, x: at.x, y: at.y, scale: b.scale, layer: b.layer, hidden: true, depth: at.depth });
                    register(prop, b.item);
                    entryAnim(prop, b, t0, dur, at);
                }
            },
            hide: {
                dur: function () { return 0.3; },
                run: function (b, j, t0, dur) {
                    var e = find(b.target);
                    if (!e) { warn('warning', beatPath(j, 'target'), 'unknown_name', 'Nothing named "' + b.target + '" to hide.'); return; }
                    e.alpha.add(t0, t0 + dur, 0, easeOf(b, 'ease-in'));
                }
            },
            camera: {
                dur: function (b) { return b.op === 'shake' ? 0.5 : 0.8; },
                run: function (b, j, t0, dur) {
                    var ease = easeOf(b, 'ease-in-out'), z = cam.zoom.at(t0), f = b.target ? focusOf(b.target, t0) : null;
                    if (b.op === 'zoom') {
                        SM.camera.addZoom(cam, t0, t0 + dur, z * (b.amount > 0 ? b.amount : 1.25), ease);
                        if (f) SM.camera.addPan(cam, t0, t0 + dur, f.x, f.y, ease);
                    } else if (b.op === 'pan') {
                        var cx = cam.cx.at(t0), cy = cam.cy.at(t0);
                        if (f) SM.camera.addPan(cam, t0, t0 + dur, f.x, f.y, ease);
                        else SM.camera.addPan(cam, t0, t0 + dur, cx + (typeof b.amount === 'number' ? b.amount : 10) / 100 * SM.stage.DESIGN_W, cy, ease);
                    } else if (b.op === 'focus') {
                        var target = f || (firstActor ? focusOf(firstActor.id, t0) : null);
                        SM.camera.addZoom(cam, t0, t0 + dur, b.amount > 0 ? b.amount : 1.6, ease);
                        if (target) SM.camera.addPan(cam, t0, t0 + dur, target.x, target.y, ease);
                    } else if (b.op === 'shake') {
                        SM.camera.addShake(cam, t0, t0 + dur, b.amount > 0 ? b.amount : 14);
                    } else {
                        warn('warning', beatPath(j, 'op'), 'invalid_value', 'Unknown camera op "' + b.op + '".');
                    }
                }
            },
            wait: {
                dur: function (b) { return b.seconds >= 0 ? b.seconds : 1; },
                run: function () {}
            }
        };

        function textPosition(at, path) {
            var rowY = TEXT_ROW_Y / 100 * SM.stage.DESIGN_H, home = { x: stage.focusX, y: rowY };
            if (at === undefined || at === null) return home;
            if (typeof at === 'string') {
                var s = stage.spot(at);
                if (!s) { warn('warning', path, 'unknown_spot', 'Unknown position "' + at + '"; the player centres the text.'); return home; }
                return stage.isBaseSpot(at) ? { x: s.x, y: rowY } : { x: s.x, y: s.y };
            }
            var p = stage.resolve(at);
            return p ? { x: p.x, y: p.y } : home;
        }

        function moveBeat(b, j, t0, dur, walking) {
            var ref = b.who || b.target, e = find(ref);
            if (!e) { warn('warning', beatPath(j, b.who ? 'who' : 'target'), 'unknown_name', 'Nothing named "' + ref + '" to move.'); return; }
            var to = resolveAt(b.to, beatPath(j, 'to'), 'center');
            var ease = easeOf(b, walking ? 'linear' : 'ease-in-out');
            if (e.type === 'actor' && walking) {
                faceToward(e, t0, to.x);
                e.look.set(t0, null);
                e.walks.add(t0, t0 + dur, {});
                if (to.face) e.facing.set(t0 + dur, to.face === 'left' ? -1 : 1);
            }
            if (typeof to.depth === 'number') e.depth = to.depth;
            moveTo(e, t0, t0 + dur, to, ease);
        }

        var cursor = plan.T, prevStart = plan.T, beatsEnd = 0;
        beats.forEach(function (b, j) {
            if (!b || typeof b !== 'object') return;
            var spec = kinds[b.do];
            if (!spec) {
                warn('minor', beatPath(j), 'not_supported_yet', 'Beat "' + b.do + '" is not supported yet; it is ignored.');
                return;
            }
            var t0 = cursor, usedPrev = b.with === 'prev' && j > 0;
            if (usedPrev) t0 = prevStart;
            else if (b.cue !== undefined && b.cue !== null) {
                var ct = plan.narration.cues[b.cue];
                if (ct === undefined) warn('warning', beatPath(j, 'cue'), 'unknown_cue', 'Narration has no cue marker {' + b.cue + '}; the beat follows the previous one.');
                else t0 = ct;
            }
            t0 += b.delay > 0 ? b.delay : 0;
            var dur = typeof b.duration === 'number' && b.duration >= 0 ? b.duration : spec.dur(b, t0);
            spec.run(b, j, t0, dur);
            var end = t0 + dur;
            prevStart = t0;
            cursor = usedPrev ? Math.max(cursor, end) : end;
            if (end > beatsEnd) beatsEnd = end;
        });

        var lastEnd = SM.camera.end(cam);
        for (var k = 0; k < entities.length; k++) { var ee = S.entityEnd(entities[k]); if (ee > lastEnd) lastEnd = ee; }
        return { entities: entities, camera: cam, beatsEnd: beatsEnd, motionEnd: lastEnd, framing: framing, actorsPlaced: placed };
    }

    SM.beats = { buildStoryboard: buildStoryboard };
})(typeof globalThis !== 'undefined' ? globalThis : self);
