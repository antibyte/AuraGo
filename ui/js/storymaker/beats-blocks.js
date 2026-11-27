// Storymaker player: block beats. A show beat with an icon, a diagram, a chart, a headline or a list (and the legacy chart
// beat) makes one block entity (blocks.js). This file builds the definition from the beat, checks it, resolves the area and
// gives the diagram nodes their reveal times; beats.js schedules the beat like any other show. Classic script; publishes
// StorymakerCore.beatBlocks.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});

    function isText(b) { return b.text !== undefined && b.text !== null; }

    // isBlockBeat(b): true for a beat that shows a block. An item always wins, as in the validator.
    function isBlockBeat(b) {
        if (!b || typeof b !== 'object') return false;
        if (b.do === 'chart') return true;
        if (b.do !== 'show' || b.item) return false;
        return !!(b.icon || b.chart || b.diagram || (isText(b) && (b.style === 'headline' || b.style === 'list')));
    }

    function kindOf(b) {
        if (b.diagram) return 'diagram';
        if (b.icon) return 'icon';
        if (b.chart) return 'chart';
        return b.style === 'list' ? 'list' : 'headline';
    }

    // slugOf(b): the base of the instance id, as Go's showSlug names it (icon.cpu becomes icon-cpu).
    function slugOf(b, kind) {
        if (kind === 'icon') return String(b.icon).toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 40) || 'icon';
        return kind === 'headline' || kind === 'list' ? 'text.text' : kind;
    }

    // defOf(b, kind, id): the block definition of a beat, without area and timing.
    function defOf(b, kind, id) {
        var def = { id: id, kind: kind };
        if (kind === 'icon') {
            def.icon = b.icon;
            if (b.style) def.style = b.style;
            if (isText(b) && b.text !== '') def.label = b.text;
        } else if (kind === 'diagram') {
            var d = b.diagram && typeof b.diagram === 'object' ? b.diagram : {};
            def.diagram = { layout: d.layout, nodes: (Array.isArray(d.nodes) ? d.nodes : []).map(function (n) {
                var out = n && typeof n === 'object' ? { id: n.id, label: n.label } : n;
                if (out && n.icon) out.icon = n.icon;
                if (out && n.points !== undefined) out.points = n.points;
                return out;
            }) };
            if (d.flow === true) def.diagram.flow = true;
        } else if (kind === 'chart') {
            var data = b.data || {};
            def.chart = { type: b.chart, labels: data.labels, values: data.values };
            if (data.unit !== undefined) def.chart.unit = data.unit;
            if (data.title !== undefined) def.chart.title = data.title;
        } else def.text = b.text;
        return def;
    }

    // issueOf(problem): the issue code and the beat field of a failed SM.blocks.check.
    function issueOf(p) {
        var head = String(p.field).split(/[.\[]/)[0], last = String(p.field).split('.').pop();
        if (head === 'icon' || last === 'icon') return { code: 'unknown_asset', field: head === 'diagram' ? 'diagram' : 'icon' };
        return { code: 'invalid_value', field: head === 'chart' ? 'data' : head };
    }

    // target(c, b, j, h): the block entity of a beat, hidden and registered, or null (with a warning) when the beat cannot be
    // shown. h = {warn, beatPath, instanceId, register}. The timing is set by reveals() when the beat is scheduled.
    function target(c, b, j, h) {
        var kind = b.do === 'chart' ? 'chart' : kindOf(b);
        if (kind === 'chart' && (b.data === undefined || b.data === null)) {
            h.warn('warning', h.beatPath(j, 'data'), 'missing_field', 'A chart needs data {labels, values}.');
            return null;
        }
        var probe = defOf(b, kind, 'block'), bad = SM.blocks.check(probe, c.cat);
        if (bad) {
            var is = issueOf(bad);
            h.warn('warning', h.beatPath(j, is.field), is.code, bad.message + ' The beat is ignored.');
            return null;
        }
        var at = b.at === undefined || b.at === null ? 'full' : b.at, area = SM.blocks.area(c.stage, c.env, c.variant, at);
        if (!area) {
            h.warn('warning', h.beatPath(j, 'at'), 'unknown_spot', 'Unknown block area ' + JSON.stringify(at) + '; the player uses "full".');
            area = SM.blocks.area(c.stage, c.env, c.variant, 'full');
        }
        var def = defOf(b, kind, h.instanceId(slugOf(b, kind), b.id));
        def.area = area;
        def.hidden = true;
        def.entry = b.entry || (kind === 'icon' || kind === 'diagram' ? 'draw' : 'fade');
        def.t0 = 0;
        def.dur = 0;
        var e = SM.blocks.create(def);
        h.register(e, null);
        return e;
    }

    // reveals(c, b, def, t0, dur): the reveal times of the diagram nodes, scene-local seconds, or null when the player spreads
    // the items over dur by itself. A node with a cue appears on its marker.
    function reveals(c, b, def, t0, dur) {
        if (def.kind !== 'diagram' || !b.diagram || !Array.isArray(b.diagram.nodes)) return null;
        var nodes = b.diagram.nodes, cues = c.plan && c.plan.narration && c.plan.narration.cues || {}, any = false, out = [];
        for (var k = 0; k < nodes.length; k++) {
            var t = nodes[k] && nodes[k].cue !== undefined && nodes[k].cue !== null ? cues[nodes[k].cue] : undefined;
            if (typeof t === 'number' && isFinite(t)) any = true;
            else t = t0 + k * dur / Math.max(1, nodes.length);
            out.push(t);
        }
        return any ? out : null;
    }

    // highlightNode(c, b, j, t0, dur, find): handles a highlight whose target is "<block id>.<node id>". It returns false when
    // the target is no such reference, so the caller reports the unknown name.
    function highlightNode(c, b, j, t0, dur, find) {
        var target = String(b.target || ''), cut = target.lastIndexOf('.');
        if (cut <= 0) return false;
        var e = find(target.slice(0, cut));
        if (!e || e.type !== 'block' || e.block.kind !== 'diagram') return false;
        var id = target.slice(cut + 1);
        var known = e.block.diagram.nodes.some(function (n) { return n.id === id; });
        if (!known) {
            c.issues.push(SM.plan.issue('warning', c.scene.id, '/scenes/' + c.index + '/beats/' + j + '/target', 'unknown_name', 'The diagram ' + e.id + ' has no node "' + id + '".'));
            return true;
        }
        e.marks.add(t0, t0 + dur, { node: id, style: b.style === 'outline' || b.style === 'arrow' ? b.style : 'pulse' });
        return true;
    }

    // texts(e): every text a block draws, for the glyph needs of the film.
    function texts(e) {
        var b = e.block, out = [];
        if (b.label) out.push(b.label);
        if (b.text) out.push(b.text.replace(/\*/g, ''));
        if (b.diagram) b.diagram.nodes.forEach(function (n) { out.push(n.label); (n.points || []).forEach(function (p) { out.push(p); }); });
        if (b.chart) { out.push(b.chart.unit || '', b.chart.title || ''); b.chart.labels.forEach(function (l) { out.push(l); }); }
        return out;
    }

    SM.beatBlocks = { isBlockBeat: isBlockBeat, target: target, reveals: reveals, highlightNode: highlightNode, texts: texts };
})(typeof globalThis !== 'undefined' ? globalThis : self);
