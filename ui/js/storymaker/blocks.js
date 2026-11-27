// Storymaker player: blocks. A block is a drawn explainer element (an icon, a diagram, a chart, a headline or a list) in one
// area of the frame. This file holds the model: the areas and the board rectangles, check and create, the reveal timing,
// serialize and revive for code scenes. blocks-draw.js paints them. A block is an entity of type 'block' whose tracks (x, y,
// alpha, scale, rot) are filled by the storyboard beats or by the code-scene API; its drawing is a pure function of the
// evaluated state and the scene time. Classic script; publishes StorymakerCore.blocks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    var BOARD_ID = 'env.board';
    var AREAS = ['full', 'left', 'right', 'top', 'center', 'board'];
    var KINDS = ['icon', 'diagram', 'chart', 'headline', 'list'];
    var ICON_STYLES = ['line', 'badge', 'card'];
    var LAYOUTS = { flow: [2, 5], cycle: [3, 6], hub: [3, 7], stack: [2, 5], compare: [2, 2] };
    var CHARTS = { bar: [1, 8], line: [2, 24], pie: [2, 6], counter: [1, 1] };
    var ENTRIES = ['draw', 'pop', 'fade', 'slide', 'none'];
    var SURFACES = ['panel', 'light', 'dark'];
    var LIST_MAX = 4, POINTS_MAX = 3, MAX_BLOCKS = 32, MAX_SEGS = 64, MAX_MARKS = 64, MAX_STRING = 200;
    var LABEL_MAX = 40, HEADLINE_MAX = 80, CHART_LABEL_MAX = 24, CHART_UNIT_MAX = 8, CHART_TITLE_MAX = 40;
    var ITEM_SPAN = 0.5, WORD_SPAN = 0.35, FLOW_PERIOD = 1.6;
    var SLUG_RE = /^[a-z0-9][a-z0-9-]{0,47}$/;
    // The eases a track may carry: the six public ones and the internal curves that SM.ease knows (see ease.js).
    var EASES = ['linear', 'ease-in', 'ease-out', 'ease-in-out', 'back-out', 'bounce-out', 'quart-out', 'expo-out', 'spring'];

    // The boards of the raster environments, in design pixels of the 1920x1080 frame. Measured on the delivered masters
    // (ui/img/storymaker/textures/environments, 1672x941, scaled to the design frame) by decoding the pixels: the first row
    // and column on each side that still has the colour of the board surface, taken as the median of three scan lines.
    //   office-day.png     pinboard   left 809, top 121, right 1451, bottom 432 (seed 810, 120, 1452, 432)
    //   classroom-day.png  chalkboard left 515, top 104, right 1535, bottom 442 (seed 511, 101, 1536, 445)
    // office-night.png is the same board in the dark: mid brown (luminance 0.18), so dark ink reads at 3.1:1 and light ink at
    // 4.1:1. The night tone of the office is therefore dark. Every other environment has no board in its master.
    var BOARD_RECTS = {
        'env.office': { x: 809, y: 121, w: 642, h: 311, tone: { day: 'light', night: 'dark' } },
        'env.classroom': { x: 515, y: 104, w: 1020, h: 338, tone: { day: 'dark', night: 'dark' } }
    };

    function isObject(v) { return v !== null && typeof v === 'object' && !Array.isArray(v); }
    function finite(v) { return typeof v === 'number' && isFinite(v); }
    function clamp(v, lo, hi) { return v < lo ? lo : (v > hi ? hi : v); }
    function own(map, key) { return Object.prototype.hasOwnProperty.call(map, key) ? map[key] : undefined; }
    function isBoard(sc) { return !!(sc && sc.env && sc.env.drawn === 'board'); }

    // ---- areas

    // area(stage, env, variant, at) = {x, y, w, h, surface} in design pixels, or null for an unknown name. at is a name of
    // AREAS or {x, y} in percent of the safe area.
    function area(stage, env, variant, at) {
        var S = SM.stage.safeBox(stage.aspect), v = variant === 'night' ? 'night' : 'day';
        var onBoard = !!(env && env.drawn === 'board'), panel = onBoard ? (v === 'night' ? 'dark' : 'light') : 'panel';
        function box(x, y, w, h, surface) { return { x: x, y: y, w: w, h: h, surface: surface || panel }; }
        if (at && typeof at === 'object') {
            if (!finite(at.x) || !finite(at.y)) return null;
            var w = 0.45 * S.w, h = 0.5 * S.h, p = stage.safePoint(at.x, at.y);
            return box(clamp(p.x - w / 2, S.x, S.x + S.w - w), clamp(p.y - h / 2, S.y, S.y + S.h - h), w, h);
        }
        switch (at) {
            case 'full': return box(S.x, S.y, S.w, S.h);
            case 'left': return box(S.x, S.y, 0.48 * S.w, S.h);
            case 'right': return box(S.x + 0.52 * S.w, S.y, 0.48 * S.w, S.h);
            case 'top': return box(S.x, S.y, S.w, 0.45 * S.h);
            case 'center': return box(S.x + 0.2 * S.w, S.y + 0.1 * S.h, 0.6 * S.w, 0.8 * S.h);
            case 'board': break;
            default: return null;
        }
        if (onBoard) return box(S.x, S.y, S.w, S.h);
        var rect = env ? own(BOARD_RECTS, env.id) : undefined, vis = stage.vis || { x: 0, y: 0, w: SM.stage.DESIGN_W, h: SM.stage.DESIGN_H };
        if (rect) {
            // the board inset by 3 % per side, cut to the safe box and to the slice that the format shows
            var x0 = Math.max(rect.x + 0.03 * rect.w, S.x, vis.x), y0 = Math.max(rect.y + 0.03 * rect.h, S.y, vis.y);
            var x1 = Math.min(rect.x + 0.97 * rect.w, S.x + S.w, vis.x + vis.w), y1 = Math.min(rect.y + 0.97 * rect.h, S.y + S.h, vis.y + vis.h);
            if (x1 > x0 && y1 > y0 && (x1 - x0) * (y1 - y0) >= 0.5 * rect.w * rect.h) return box(x0, y0, x1 - x0, y1 - y0, rect.tone[v]);
        }
        if (stage.aspect !== '16:9') return area(stage, env, v, 'top');
        var left = Math.max(S.x, vis.x + vis.w / 2);
        return box(left, S.y, S.x + S.w - left, S.h);
    }

    // ---- check

    function problem(field, message) { return { field: field, message: message }; }

    function lines(text) {
        return typeof text === 'string' ? text.split('\n').map(function (l) { return l.replace(/\s+$/, ''); }).filter(function (l) { return l.length > 0; }) : [];
    }

    // parseHeadline(text) = {words, mark}: the words of a headline; the word inside asterisks is the marked one (mark is its index, or -1).
    function parseHeadline(text) {
        var words = [], mark = -1;
        String(text || '').split(/\s+/).filter(Boolean).forEach(function (token) {
            var word = token.replace(/\*/g, '');
            if (!word) return;
            if (mark < 0 && token.indexOf('*') >= 0) mark = words.length;
            words.push(word);
        });
        return { words: words, mark: mark };
    }

    function itemCount(b) {
        if (b.kind === 'diagram') return b.diagram && Array.isArray(b.diagram.nodes) ? b.diagram.nodes.length : 0;
        if (b.kind === 'list') return lines(b.text).length;
        if (b.kind === 'headline') return parseHeadline(b.text).words.length;
        return 1;
    }

    function checkIcon(id, field, cat) {
        if (typeof id !== 'string' || !cat || !cat.icons || !own(cat.icons, id)) {
            return problem(field, 'Unknown icon "' + String(id).slice(0, 60) + '". Search storymaker_catalog with kind icon and use an exact id such as icon.cpu.');
        }
        return null;
    }

    function checkLabel(label, field, max) {
        if (typeof label !== 'string' || label.length < 1 || label.length > max) return problem(field, 'A label has 1 to ' + max + ' characters. Shorten it.');
        return null;
    }

    function checkDiagram(d, cat) {
        if (!isObject(d)) return problem('diagram', 'A diagram block needs diagram {layout, nodes}.');
        var range = own(LAYOUTS, d.layout);
        if (!range) return problem('diagram.layout', 'Unknown diagram layout "' + String(d.layout).slice(0, 20) + '". Use ' + Object.keys(LAYOUTS).join(', ') + '.');
        var list = d.nodes;
        if (!Array.isArray(list) || list.length < range[0] || list.length > range[1]) {
            return problem('diagram.nodes', 'A ' + d.layout + ' diagram takes ' + range[0] + ' to ' + range[1] + ' nodes, got ' + (Array.isArray(list) ? list.length : 0) + '. Change the number of nodes or the layout.');
        }
        var seen = {};
        for (var i = 0; i < list.length; i++) {
            var n = list[i], at = 'diagram.nodes[' + i + ']';
            if (!isObject(n)) return problem('diagram.nodes', 'Node ' + i + ' must be an object {id, label}.');
            if (typeof n.id !== 'string' || !SLUG_RE.test(n.id)) return problem('diagram.nodes', 'Node ' + i + ' needs an id made of lowercase letters, digits and dashes, for example "srv".');
            if (seen[n.id]) return problem('diagram.nodes', 'The node id "' + n.id + '" is used twice. Give every node its own id.');
            seen[n.id] = true;
            var bad = checkLabel(n.label, at + '.label', LABEL_MAX);
            if (bad) return bad;
            if (n.icon !== undefined && n.icon !== null && n.icon !== '') { bad = checkIcon(n.icon, at + '.icon', cat); if (bad) return bad; }
            if (n.points !== undefined) {
                if (!Array.isArray(n.points) || n.points.length > POINTS_MAX) return problem(at + '.points', 'A node has at most ' + POINTS_MAX + ' points.');
                for (var k = 0; k < n.points.length; k++) {
                    if (typeof n.points[k] !== 'string' || n.points[k].length < 1 || n.points[k].length > LABEL_MAX) return problem(at + '.points', 'A point has 1 to ' + LABEL_MAX + ' characters. Shorten it.');
                }
            }
        }
        return null;
    }

    function checkChart(c) {
        if (!isObject(c)) return problem('chart', 'A chart block needs chart {type, labels, values}.');
        var range = own(CHARTS, c.type);
        if (!range) return problem('chart.type', 'Unknown chart type "' + String(c.type).slice(0, 20) + '". Use ' + Object.keys(CHARTS).join(', ') + '.');
        if (!Array.isArray(c.labels) || !Array.isArray(c.values) || c.labels.length !== c.values.length) return problem('chart.values', 'labels and values need the same length.');
        if (c.values.length < range[0] || c.values.length > range[1]) return problem('chart.values', 'A ' + c.type + ' chart takes ' + range[0] + ' to ' + range[1] + ' values, got ' + c.values.length + '.');
        var sum = 0;
        for (var i = 0; i < c.values.length; i++) {
            if (!finite(c.values[i])) return problem('chart.values', 'Every value must be a finite number.');
            if (c.type === 'pie' && c.values[i] < 0) return problem('chart.values', 'A pie value must be 0 or more, got ' + c.values[i] + '.');
            sum += c.values[i];
            if (typeof c.labels[i] !== 'string' || c.labels[i].length > CHART_LABEL_MAX) return problem('chart.labels', 'A chart label has at most ' + CHART_LABEL_MAX + ' characters.');
        }
        if (c.type === 'pie' && !(sum > 0)) return problem('chart.values', 'A pie needs a sum above 0.');
        if (c.unit !== undefined && (typeof c.unit !== 'string' || c.unit.length > CHART_UNIT_MAX)) return problem('chart.unit', 'The unit has at most ' + CHART_UNIT_MAX + ' characters.');
        if (c.title !== undefined && (typeof c.title !== 'string' || c.title.length > CHART_TITLE_MAX)) return problem('chart.title', 'The title has at most ' + CHART_TITLE_MAX + ' characters.');
        return null;
    }

    // check(def, cat) = null, or {field, message} for the first problem; the message names the fix. cat is SM.scene.indexCatalog.
    function check(def, cat) {
        if (!isObject(def)) return problem('block', 'A block is an object.');
        if (typeof def.id !== 'string' || !def.id || def.id.length > 64) return problem('id', 'A block needs an id of 1 to 64 characters.');
        if (KINDS.indexOf(def.kind) < 0) return problem('kind', 'Unknown block kind "' + String(def.kind).slice(0, 20) + '". Use ' + KINDS.join(', ') + '.');
        var bad = null, n = itemCount(def);
        if (def.kind === 'icon') {
            bad = checkIcon(def.icon, 'icon', cat);
            if (!bad && def.style !== undefined && ICON_STYLES.indexOf(def.style) < 0) bad = problem('style', 'Unknown icon style "' + String(def.style).slice(0, 20) + '". Use ' + ICON_STYLES.join(', ') + '.');
            if (!bad && def.label !== undefined && def.label !== '') bad = checkLabel(def.label, 'label', LABEL_MAX);
        } else if (def.kind === 'diagram') bad = checkDiagram(def.diagram, cat);
        else if (def.kind === 'chart') bad = checkChart(def.chart);
        else if (def.kind === 'headline') {
            if (typeof def.text !== 'string' || !parseHeadline(def.text).words.length || def.text.length > HEADLINE_MAX) bad = problem('text', 'A headline has 1 to ' + HEADLINE_MAX + ' characters. Shorten it.');
        } else {
            var rows = lines(def.text);
            if (!rows.length || rows.length > LIST_MAX) bad = problem('text', 'A list has 1 to ' + LIST_MAX + ' lines, got ' + rows.length + '. Split the list.');
            for (var r = 0; !bad && r < rows.length; r++) if (rows[r].length > LABEL_MAX) bad = problem('text', 'A list line has at most ' + LABEL_MAX + ' characters. Shorten line ' + (r + 1) + '.');
        }
        if (bad) return bad;
        if (def.reveals !== undefined && def.reveals !== null) {
            if (!Array.isArray(def.reveals) || def.reveals.length !== n || !def.reveals.every(finite)) return problem('reveals', 'reveals needs one finite number per node or line (' + n + ').');
        }
        if (def.area !== undefined && !(isObject(def.area) && finite(def.area.x) && finite(def.area.y) && def.area.w > 0 && def.area.h > 0 && isFinite(def.area.w) && isFinite(def.area.h))) {
            return problem('area', 'area needs finite x, y, w and h from SM.blocks.area.');
        }
        return null;
    }

    // revealSeconds(def): the default entry span of a block in seconds, before the rhythm factor.
    function revealSeconds(def) {
        switch (def.kind) {
            case 'diagram': return 0.5 + 0.6 * itemCount(def);
            case 'chart': return 1.2;
            case 'headline': return 0.4 + 0.15 * parseHeadline(def.text).words.length;
            case 'list': return 0.5 * itemCount(def);
            default: return 0.8;
        }
    }

    // itemProgress(st, k): 0..1 reveal of item k of an evaluated block state (a node, a line or a word; k = 0 for icon and chart).
    function itemProgress(st, k) {
        var kind = st.block.kind, p;
        if (kind === 'icon' || kind === 'chart') p = st.dur > 0 ? (st.time - st.t0) / st.dur : (st.time >= st.t0 ? 1 : 0);
        else {
            var n = Math.max(1, itemCount(st.block)), full = kind === 'headline' ? WORD_SPAN : ITEM_SPAN, start, span;
            if (st.reveals && finite(st.reveals[k])) { start = st.reveals[k]; span = full; }
            else { start = st.t0 + k * st.dur / n; span = Math.min(full, st.dur / n); }
            p = span > 0 ? (st.time - start) / span : (st.time >= start ? 1 : 0);
        }
        return SM.ease('ease-out', clamp(p, 0, 1));
    }

    // ---- diagram layouts

    // ring(n, W, H, size, from): n boxes of size x size * 1.1 on the ellipse that fills the area, from the top, clockwise.
    function ring(n, W, H, s, first, step, into) {
        var bw = s * 1.1, rx = (W - bw) / 2, ry = (H - s) / 2;
        for (var i = 0; i < n; i++) {
            var a = -Math.PI / 2 + (first + i) * step;
            into.push({ x: rx * Math.cos(a) - bw / 2, y: ry * Math.sin(a) - s / 2, w: bw, h: s });
        }
    }

    // layout(layout, n, W, H) = {boxes: [{x, y, w, h}], links: [[from, to]]}: the node boxes around the centre of a W x H area at scale
    // 1 (x and y are the top left corner), and the arrows between them. The boxes never overlap and never leave the area.
    function layout(kind, n, W, H) {
        var pad = 0.04 * Math.min(W, H), boxes = [], links = [], m, i, nw, nh, s;
        W -= 2 * pad;   // the layout keeps a margin to the area, so outlines and highlights stay inside it
        H -= 2 * pad;
        m = Math.min(W, H);
        if (kind === 'flow' && W >= H) {
            nw = Math.min(0.665 * H, W / (n + 0.45 * (n - 1)));
            nh = Math.min(0.7 * H, nw * 1.1);
            var left = -(n * nw + 0.45 * nw * (n - 1)) / 2;
            for (i = 0; i < n; i++) { boxes.push({ x: left + i * 1.45 * nw, y: -nh / 2, w: nw, h: nh }); if (i) links.push([i - 1, i]); }
        } else if (kind === 'flow') {
            nh = Math.min(H / (n + 0.45 * (n - 1)), 0.5 * W);
            nw = Math.min(0.92 * W, nh * 2.6);
            var top = -(n * nh + 0.45 * nh * (n - 1)) / 2;
            for (i = 0; i < n; i++) { boxes.push({ x: -nw / 2, y: top + i * 1.45 * nh, w: nw, h: nh }); if (i) links.push([i - 1, i]); }
        } else if (kind === 'stack') {
            nh = Math.min(H / (n + 0.12 * (n - 1)), 0.26 * H);
            nw = Math.min(0.92 * W, nh * 5.5);
            var total = n * nh + 0.12 * nh * (n - 1);
            for (i = 0; i < n; i++) boxes.push({ x: -nw / 2, y: total / 2 - nh - i * 1.12 * nh, w: nw, h: nh });
        } else if (kind === 'cycle') {
            ring(n, W, H, m * ({ 3: 0.28, 4: 0.26, 5: 0.24 }[n] || 0.21), 0, 2 * Math.PI / n, boxes);
            for (i = 0; i < n; i++) links.push([i, (i + 1) % n]);
        } else if (kind === 'hub') {
            s = m * 0.26;
            boxes.push({ x: -s * 0.55, y: -s / 2, w: s * 1.1, h: s });
            ring(n - 1, W, H, m * ({ 2: 0.24, 3: 0.24, 4: 0.22, 5: 0.22 }[n - 1] || 0.2), 0, 2 * Math.PI / (n - 1), boxes);
            for (i = 1; i < n; i++) links.push([0, i]);
        } else {
            var gap = Math.max(24, 0.05 * W);
            nw = Math.min((W - gap) / 2, 1.25 * H);
            nh = Math.min(H, nw * 1.3);
            boxes.push({ x: -gap / 2 - nw, y: -nh / 2, w: nw, h: nh }, { x: gap / 2, y: -nh / 2, w: nw, h: nh });
        }
        return { boxes: boxes, links: links };
    }

    // layoutNodes(st) = [{id, x, y, w, h}]: the node boxes of an evaluated diagram state in design pixels at scale 1.
    function layoutNodes(st) {
        var d = st.block.diagram, l = layout(d.layout, d.nodes.length, st.w, st.h);
        return d.nodes.map(function (n, i) { var b = l.boxes[i]; return { id: n.id, x: st.x + b.x, y: st.y + b.y, w: b.w, h: b.h }; });
    }

    // ---- create

    var DEF_ONLY = { area: 1, t0: 1, dur: 1, entry: 1, reveals: 1, hidden: 1 };

    // make(block, geometry): the entity. The block field is a plain copy of the definition without the timing fields.
    function make(block, g) {
        var e = SM.scene.base('block', block.id, g.x, g.y);
        e.block = block;
        e.w = g.w;
        e.h = g.h;
        e.surface = SURFACES.indexOf(g.surface) >= 0 ? g.surface : 'panel';
        e.t0 = g.t0;
        e.dur = g.dur;
        e.entry = ENTRIES.indexOf(g.entry) >= 0 ? g.entry : 'fade';
        e.reveals = g.reveals ? g.reveals.slice() : null;
        e.marks = new SM.timeline.Intervals();
        e.highlights = new SM.timeline.Intervals();
        e.baseScale = 1;
        if (g.hidden) e.alpha.init = 0;
        return e;
    }

    // create(def) = the entity of type 'block' for a definition (see the seams document); def.area comes from area().
    function create(def) {
        if (!isObject(def) || !isObject(def.area)) throw new TypeError('SM.blocks.create needs a definition with an area from SM.blocks.area.');
        var block = {};
        Object.keys(def).forEach(function (k) { if (!DEF_ONLY[k]) block[k] = JSON.parse(JSON.stringify(def[k])); });
        var t0 = finite(def.t0) ? def.t0 : 0, dur = finite(def.dur) ? def.dur : revealSeconds(def), a = def.area;
        return make(block, { x: a.x + a.w / 2, y: a.y + a.h / 2, w: a.w, h: a.h, surface: a.surface, t0: t0, dur: dur < 0 ? 0 : dur, entry: def.entry, reveals: def.reveals, hidden: def.hidden === true });
    }

    // ---- serialize and revive

    function trackOut(track) {
        track.finalize();
        return { init: track.init, segs: track.segs.map(function (s) { return { t0: s.t0, t1: s.t1, from: s.from, to: s.to, ease: s.ease, log: !!s.log }; }) };
    }

    function listOut(list, keys) {
        return list.list.map(function (it) {
            var o = { t0: it.t0, t1: it.t1 };
            keys.forEach(function (k) { o[k] = it.data[k]; });
            return o;
        });
    }

    // serialize(entities, camera) = a plain payload of the block entities and the camera, or null without blocks.
    function serialize(entities, camera) {
        var list = (entities || []).filter(function (e) { return e && e.type === 'block'; });
        if (!list.length) return null;
        var cam = null;
        if (camera) cam = { zoom: trackOut(camera.zoom), cx: trackOut(camera.cx), cy: trackOut(camera.cy), shakes: (camera.shakes || []).map(function (s) { return { t0: s.t0, t1: s.t1, amp: s.amp }; }) };
        return {
            v: 1, camera: cam,
            blocks: list.map(function (e) {
                return {
                    id: e.id, block: e.block, w: e.w, h: e.h, surface: e.surface, t0: e.t0, dur: e.dur, entry: e.entry, reveals: e.reveals,
                    x: trackOut(e.x), y: trackOut(e.y), alpha: trackOut(e.alpha), scale: trackOut(e.scale), rot: trackOut(e.rot),
                    marks: listOut(e.marks, ['node', 'style']), highlights: listOut(e.highlights, ['style'])
                };
            })
        };
    }

    // sane(value, depth): false for a non-finite number, a string over MAX_STRING characters or a structure that is too deep.
    function sane(value, depth) {
        if (typeof value === 'number') return isFinite(value);
        if (typeof value === 'string') return value.length <= MAX_STRING;
        if (value === null || typeof value === 'boolean' || value === undefined) return true;
        if (depth > 8 || typeof value !== 'object') return false;
        return Object.keys(value).every(function (k) { return k.length <= MAX_STRING && sane(value[k], depth + 1); });
    }

    function trackIn(plain, T) {
        if (!isObject(plain) || !finite(plain.init) || !Array.isArray(plain.segs) || plain.segs.length > MAX_SEGS) return null;
        var track = new T.Track(plain.init);
        for (var i = 0; i < plain.segs.length; i++) {
            var s = plain.segs[i];
            if (!isObject(s) || !finite(s.t0) || !finite(s.t1) || !finite(s.from) || !finite(s.to) || EASES.indexOf(s.ease) < 0) return null;
            track.segs.push({ t0: s.t0, t1: s.t1, from: s.from, to: s.to, ease: s.ease, log: s.log === true, order: i });
        }
        return track;
    }

    function listIn(plain, keys, into) {
        if (!Array.isArray(plain) || plain.length > MAX_MARKS) return false;
        for (var i = 0; i < plain.length; i++) {
            var m = plain[i];
            if (!isObject(m) || !finite(m.t0) || !finite(m.t1)) return false;
            var data = {};
            keys.forEach(function (k) { data[k] = m[k]; });
            into.add(m.t0, m.t1, data);
        }
        return true;
    }

    function reviveBlock(p, cat) {
        var T = SM.timeline;
        if (!isObject(p) || !isObject(p.block) || p.block.id !== p.id || check(p.block, cat)) return null;
        if (!finite(p.w) || !finite(p.h) || !(p.w > 0) || !(p.h > 0) || !finite(p.t0) || !finite(p.dur) || p.dur < 0) return null;
        if (SURFACES.indexOf(p.surface) < 0 || ENTRIES.indexOf(p.entry) < 0) return null;
        if (p.reveals !== null && p.reveals !== undefined && (!Array.isArray(p.reveals) || p.reveals.length !== itemCount(p.block) || !p.reveals.every(finite))) return null;
        var tracks = {};
        for (var k = 0; k < 5; k++) {
            var name = ['x', 'y', 'alpha', 'scale', 'rot'][k];
            tracks[name] = trackIn(p[name], T);
            if (!tracks[name]) return null;
        }
        var e = make(p.block, { x: tracks.x.init, y: tracks.y.init, w: p.w, h: p.h, surface: p.surface, t0: p.t0, dur: p.dur, entry: p.entry, reveals: p.reveals, hidden: false });
        Object.keys(tracks).forEach(function (name) { e[name] = tracks[name]; });
        return listIn(p.marks, ['node', 'style'], e.marks) && listIn(p.highlights, ['style'], e.highlights) ? e : null;
    }

    // revive(plain, cat) = {entities, camera} from a payload of serialize, or null when anything in it is invalid.
    function revive(plain, cat) {
        if (!isObject(plain) || plain.v !== 1 || !Array.isArray(plain.blocks) || plain.blocks.length > MAX_BLOCKS || !sane(plain, 0)) return null;
        var T = SM.timeline, camera = null, entities = [];
        if (plain.camera !== null && plain.camera !== undefined) {
            var c = plain.camera;
            if (!isObject(c) || !Array.isArray(c.shakes) || c.shakes.length > MAX_MARKS) return null;
            camera = { zoom: trackIn(c.zoom, T), cx: trackIn(c.cx, T), cy: trackIn(c.cy, T), shakes: [] };
            if (!camera.zoom || !camera.cx || !camera.cy) return null;
            for (var s = 0; s < c.shakes.length; s++) {
                var sh = c.shakes[s];
                if (!isObject(sh) || !finite(sh.t0) || !finite(sh.t1) || !finite(sh.amp)) return null;
                camera.shakes.push({ t0: sh.t0, t1: sh.t1, amp: sh.amp });
            }
        }
        for (var i = 0; i < plain.blocks.length; i++) {
            var e = reviveBlock(plain.blocks[i], cat);
            if (!e) return null;
            entities.push(e);
        }
        return { entities: entities, camera: camera };
    }

    SM.blocks = {
        BOARD_ID: BOARD_ID, AREAS: AREAS, KINDS: KINDS, ICON_STYLES: ICON_STYLES, LAYOUTS: LAYOUTS, CHARTS: CHARTS, LIST_MAX: LIST_MAX,
        POINTS_MAX: POINTS_MAX, MAX_BLOCKS: MAX_BLOCKS, LABEL_BASE: 0.034, LABEL_MIN: 0.026, ITEM_SPAN: ITEM_SPAN, WORD_SPAN: WORD_SPAN,
        FLOW_PERIOD: FLOW_PERIOD, BOARD_RECTS: BOARD_RECTS, isBoard: isBoard, area: area, check: check, create: create,
        revealSeconds: revealSeconds, itemProgress: itemProgress, serialize: serialize, revive: revive,
        itemCount: itemCount, lines: lines, parseHeadline: parseHeadline, layout: layout, layoutNodes: layoutNodes
    };
})(typeof globalThis !== 'undefined' ? globalThis : self);
