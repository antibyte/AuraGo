/* Notes graph: canvas note graph with its own Barnes-Hut force simulation.
 * Browser: window.NotesGraph. Node: module.exports. No DOM access at load time. */
(function (root, factory) {
    'use strict';
    var api = factory();
    if (typeof module !== 'undefined' && module.exports) module.exports = api;
    if (root) root.NotesGraph = api;
})(typeof window !== 'undefined' ? window : null, function () {
    'use strict';

    // ---------------------------------------------------------------- simulation

    var GOLDEN_ANGLE = Math.PI * (3 - Math.sqrt(5));
    var MAX_DEPTH = 30;

    function mulberry32(seed) {
        var s = seed >>> 0;
        return function () {
            s = (s + 0x6D2B79F5) >>> 0;
            var t = s;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    function nodeRadius(degree) {
        return Math.min(14, 3 + 1.6 * Math.sqrt(degree));
    }

    function createSimulation(config) {
        config = config || {};
        var nodes = config.nodes || [];
        var rawLinks = config.links || [];
        var rand = mulberry32(config.seed == null ? 1 : config.seed);
        var n = nodes.length;

        var alphaMin = 0.002;
        var alphaDecay = 1 - Math.pow(alphaMin, 1 / 300);
        var velocityDecay = 0.45;
        var chargeStrength = -220;
        var theta2 = 0.9 * 0.9;
        var distanceMax2 = 900 * 900;
        var linkDistance = 70;
        var centerStrength = 0.04;
        var alpha = config.alpha == null ? 1 : config.alpha;

        var byId = new Map();
        var i;
        for (i = 0; i < n; i++) byId.set(nodes[i].id, i);

        // resolve links to indices
        var ls = [], lt = [], links = [];
        var degree = new Int32Array(n);
        for (i = 0; i < rawLinks.length; i++) {
            var rs = rawLinks[i].source, rt = rawLinks[i].target;
            if (rs && typeof rs === 'object') rs = rs.id;
            if (rt && typeof rt === 'object') rt = rt.id;
            var a = byId.get(rs), b = byId.get(rt);
            if (a === undefined || b === undefined || a === b) continue;
            ls.push(a); lt.push(b);
            degree[a]++; degree[b]++;
            links.push({ source: nodes[a], target: nodes[b] });
        }
        var m = ls.length;
        var strength = new Float64Array(m);
        var bias = new Float64Array(m);
        for (i = 0; i < m; i++) {
            strength[i] = 1 / Math.min(degree[ls[i]], degree[lt[i]]);
            bias[i] = degree[ls[i]] / (degree[ls[i]] + degree[lt[i]]);
        }

        var radius = new Float64Array(n);
        var maxR = 0;
        for (i = 0; i < n; i++) {
            var nd = nodes[i];
            radius[i] = nodeRadius(degree[i]);
            if (radius[i] > maxR) maxR = radius[i];
            nd.degree = degree[i];
            nd.r = radius[i];
            if (!isFinite(nd.x) || !isFinite(nd.y) || nd.x == null || nd.y == null) {
                var rr = 10 * Math.sqrt(0.5 + i), th = i * GOLDEN_ANGLE;
                nd.x = rr * Math.cos(th) + (rand() - 0.5) * 0.2;
                nd.y = rr * Math.sin(th) + (rand() - 0.5) * 0.2;
            }
            if (!isFinite(nd.vx)) nd.vx = 0;
            if (!isFinite(nd.vy)) nd.vy = 0;
        }

        function jiggle() { return (rand() - 0.5) * 1e-6; }

        // quadtree pools (reused each tick)
        var X0 = [], Y0 = [], SZ = [], M = [], SX = [], SY = [], PT = [], CH = [];
        var cc = 0;
        function newCell(x0, y0, sz) {
            var id = cc++;
            X0[id] = x0; Y0[id] = y0; SZ[id] = sz;
            M[id] = 0; SX[id] = 0; SY[id] = 0; PT[id] = -1;
            var o = id * 4;
            CH[o] = CH[o + 1] = CH[o + 2] = CH[o + 3] = -1;
            return id;
        }
        function quadrant(c, x, y) {
            var h = SZ[c] / 2;
            return (x >= X0[c] + h ? 1 : 0) + (y >= Y0[c] + h ? 2 : 0);
        }
        function child(c, q) {
            var o = c * 4 + q;
            if (CH[o] === -1) {
                var h = SZ[c] / 2;
                CH[o] = newCell(X0[c] + (q & 1 ? h : 0), Y0[c] + (q & 2 ? h : 0), h);
            }
            return CH[o];
        }
        function build() {
            var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity, k;
            for (k = 0; k < n; k++) {
                var p = nodes[k];
                if (p.x < minX) minX = p.x;
                if (p.x > maxX) maxX = p.x;
                if (p.y < minY) minY = p.y;
                if (p.y > maxY) maxY = p.y;
            }
            cc = 0;
            newCell(minX, minY, Math.max(maxX - minX, maxY - minY) + 1e-3);
            for (k = 0; k < n; k++) insert(k);
        }
        function insert(idx) {
            var p = nodes[idx], x = p.x, y = p.y, c = 0, depth = 0;
            for (;;) {
                M[c]++; SX[c] += x; SY[c] += y;
                if (M[c] === 1) { PT[c] = idx; return; }
                if (depth >= MAX_DEPTH) return;
                if (PT[c] !== -1) {
                    var old = PT[c], op = nodes[old];
                    PT[c] = -1;
                    var cq = child(c, quadrant(c, op.x, op.y));
                    M[cq] = 1; SX[cq] = op.x; SY[cq] = op.y; PT[cq] = old;
                }
                c = child(c, quadrant(c, x, y));
                depth++;
            }
        }

        var stack = [];
        function applyCharge() {
            if (n < 2) return;
            build();
            var k = chargeStrength * alpha;
            for (var idx = 0; idx < n; idx++) {
                var nd = nodes[idx], x = nd.x, y = nd.y, vx = 0, vy = 0;
                stack.length = 0;
                stack.push(0);
                while (stack.length) {
                    var c = stack.pop();
                    var mass = M[c];
                    var leaf = PT[c] !== -1;
                    if (leaf && mass === 1 && PT[c] === idx) continue;
                    var dx = SX[c] / mass - x, dy = SY[c] / mass - y;
                    var l = dx * dx + dy * dy;
                    if (!leaf && SZ[c] * SZ[c] >= theta2 * l) {
                        var o = c * 4;
                        for (var q = 0; q < 4; q++) if (CH[o + q] !== -1) stack.push(CH[o + q]);
                        continue;
                    }
                    if (l >= distanceMax2) continue;
                    if (dx === 0) { dx = jiggle(); l += dx * dx; }
                    if (dy === 0) { dy = jiggle(); l += dy * dy; }
                    if (l < 1) l = Math.sqrt(l);
                    var w = k * mass / l;
                    vx += dx * w;
                    vy += dy * w;
                }
                nd.vx += vx;
                nd.vy += vy;
            }
        }

        function applyLinks() {
            for (var k = 0; k < m; k++) {
                var s = nodes[ls[k]], t = nodes[lt[k]];
                var x = t.x + t.vx - s.x - s.vx;
                var y = t.y + t.vy - s.y - s.vy;
                if (x === 0) x = jiggle();
                if (y === 0) y = jiggle();
                var l = Math.sqrt(x * x + y * y);
                l = (l - linkDistance) / l * alpha * strength[k];
                x *= l; y *= l;
                var b = bias[k];
                t.vx -= x * b; t.vy -= y * b;
                s.vx += x * (1 - b); s.vy += y * (1 - b);
            }
        }

        function applyCenter() {
            var k = centerStrength * alpha;
            for (var idx = 0; idx < n; idx++) {
                nodes[idx].vx -= nodes[idx].x * k;
                nodes[idx].vy -= nodes[idx].y * k;
            }
        }

        function applyCollide() {
            if (n < 2) return;
            var cell = 2 * maxR + 2;
            var grid = new Map();
            var idx, key;
            for (idx = 0; idx < n; idx++) {
                var p = nodes[idx];
                key = (Math.floor(p.x / cell) + 32768) * 65536 + (Math.floor(p.y / cell) + 32768);
                var bucket = grid.get(key);
                if (bucket) bucket.push(idx); else grid.set(key, [idx]);
            }
            for (idx = 0; idx < n; idx++) {
                var a = nodes[idx];
                var cx = Math.floor(a.x / cell), cy = Math.floor(a.y / cell);
                for (var gx = cx - 1; gx <= cx + 1; gx++) {
                    for (var gy = cy - 1; gy <= cy + 1; gy++) {
                        var bk = grid.get((gx + 32768) * 65536 + (gy + 32768));
                        if (!bk) continue;
                        for (var q = 0; q < bk.length; q++) {
                            var j = bk[q];
                            if (j <= idx) continue;
                            var b = nodes[j];
                            var dx = a.x + a.vx - b.x - b.vx;
                            var dy = a.y + a.vy - b.y - b.vy;
                            var rr = radius[idx] + radius[j] + 2;
                            var l = dx * dx + dy * dy;
                            if (l >= rr * rr) continue;
                            if (dx === 0) { dx = jiggle(); l += dx * dx; }
                            if (dy === 0) { dy = jiggle(); l += dy * dy; }
                            l = Math.sqrt(l);
                            var f = (rr - l) / l * 0.7;
                            var ra = radius[idx] * radius[idx], rb = radius[j] * radius[j];
                            var wb = rb / (ra + rb);
                            a.vx += dx * f * wb; a.vy += dy * f * wb;
                            b.vx -= dx * f * (1 - wb); b.vy -= dy * f * (1 - wb);
                        }
                    }
                }
            }
        }

        var sim = {
            nodes: nodes,
            links: links,
            alphaMin: alphaMin,
            tick: function () {
                alpha += (0 - alpha) * alphaDecay;
                applyLinks();
                applyCharge();
                applyCenter();
                applyCollide();
                var keep = 1 - velocityDecay;
                for (var k = 0; k < n; k++) {
                    var nd = nodes[k];
                    if (nd.fx != null) { nd.x = nd.fx; nd.vx = 0; } else { nd.vx *= keep; nd.x += nd.vx; }
                    if (nd.fy != null) { nd.y = nd.fy; nd.vy = 0; } else { nd.vy *= keep; nd.y += nd.vy; }
                }
                return sim;
            },
            reheat: function (a) {
                a = a == null ? 0.5 : a;
                if (a > alpha) alpha = a;
                return sim;
            },
            run: function (maxTicks) {
                var limit = maxTicks == null ? 300 : maxTicks, t = 0;
                while (alpha >= alphaMin && t < limit) { sim.tick(); t++; }
                return t;
            }
        };
        Object.defineProperty(sim, 'alpha', {
            get: function () { return alpha; },
            set: function (v) { alpha = v; },
            enumerable: true
        });
        Object.defineProperty(sim, 'stopped', { get: function () { return alpha < alphaMin; }, enumerable: true });
        // pin nodes that were handed in with fx/fy
        for (i = 0; i < n; i++) {
            if (nodes[i].fx != null) nodes[i].x = nodes[i].fx;
            if (nodes[i].fy != null) nodes[i].y = nodes[i].fy;
        }
        return sim;
    }

    // ------------------------------------------------------------------ colours

    var TEAL = [47, 179, 196];

    function parseColor(str) {
        if (!str || typeof str !== 'string') return null;
        str = str.trim();
        var m = /^#([0-9a-f]{3,8})$/i.exec(str);
        if (m) {
            var h = m[1];
            if (h.length === 3 || h.length === 4) h = h.split('').map(function (c) { return c + c; }).join('');
            if (h.length !== 6 && h.length !== 8) return null;
            return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
        }
        m = /^rgba?\(\s*([\d.]+)[\s,]+([\d.]+)[\s,]+([\d.]+)/i.exec(str);
        if (m) return [+m[1], +m[2], +m[3]];
        return null;
    }

    function mixToTeal(base, amount) {
        var c = parseColor(base);
        if (!c) return 'rgb(' + TEAL.join(',') + ')';
        var out = [0, 1, 2].map(function (i) { return Math.round(c[i] * (1 - amount) + TEAL[i] * amount); });
        return 'rgb(' + out.join(',') + ')';
    }

    // ----------------------------------------------------------------- renderer

    function truncate(s) {
        s = String(s == null ? '' : s);
        return s.length > 40 ? s.slice(0, 39) + '…' : s;
    }

    function create(host, options) {
        options = options || {};
        var onOpen = options.onOpen || function () {};
        var onHover = options.onHover || function () {};
        var label = options.label || 'Graph';

        var doc = host.ownerDocument;
        var win = doc.defaultView;
        if (win.getComputedStyle(host).position === 'static') host.style.position = 'relative';

        var canvas = doc.createElement('canvas');
        canvas.tabIndex = 0;
        canvas.setAttribute('role', 'img');
        canvas.style.cssText = 'position:absolute;left:0;top:0;width:100%;height:100%;display:block;touch-action:none;outline:none;';
        host.appendChild(canvas);
        var ctx = canvas.getContext('2d');

        var reduced;
        if (typeof options.reducedMotion === 'boolean') reduced = options.reducedMotion;
        else {
            reduced = false;
            try {
                reduced = !!(win.matchMedia && win.matchMedia('(prefers-reduced-motion: reduce)').matches);
                if (doc.body && doc.body.getAttribute('data-animations') === 'false') reduced = true;
            } catch (e) { /* keep default */ }
        }

        var w = 0, h = 0, dpr = 1;
        var view = { k: 1, x: 0, y: 0 };
        var nodes = [], byId = new Map(), adj = new Map(), sim = null;
        var colors = null, fontFamily = 'sans-serif';
        var hoverId = null, highlightId = null, kbId = null, kbOrder = [];
        var drag = null;
        var raf = 0, destroyed = false;
        var autoFit = true, fitDone = false, tickCount = 0;

        function readColors() {
            var cs = win.getComputedStyle(host);
            fontFamily = cs.fontFamily || 'sans-serif';
            function v(name, fb) { var s = cs.getPropertyValue(name); return s && s.trim() ? s.trim() : fb; }
            var muted = v('--notes-muted', '#8a8f98');
            var d = {
                node: muted,
                current: v('--notes-accent', '#7c6cf0'),
                tag: mixToTeal(muted, 0.6),
                unresolved: muted,
                link: muted,
                text: v('--notes-ink', '#d8dae0'),
                background: v('--notes-well', '#1c1d21')
            };
            if (typeof options.colors === 'function') {
                var c = options.colors() || {};
                for (var key in c) if (c[key]) d[key] = c[key];
            }
            colors = d;
        }

        function scale() { return Math.max(0.6, Math.min(view.k, 2.5)); }
        function sx(n) { return n.x * view.k + view.x; }
        function sy(n) { return n.y * view.k + view.y; }

        function focusId() { return hoverId || kbId || highlightId; }

        function draw() {
            if (!colors) readColors();
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            ctx.clearRect(0, 0, w, h);
            if (!nodes.length) return;
            var focus = focusId();
            var nb = focus ? adj.get(focus) : null;
            var sc = scale(), i, nd;

            // links
            var links = sim ? sim.links : [];
            ctx.lineWidth = 1;
            ctx.strokeStyle = colors.link;
            ctx.globalAlpha = focus ? 0.1 : 0.4;
            ctx.beginPath();
            for (i = 0; i < links.length; i++) {
                var l = links[i];
                if (focus && (l.source.id === focus || l.target.id === focus)) continue;
                ctx.moveTo(sx(l.source), sy(l.source));
                ctx.lineTo(sx(l.target), sy(l.target));
            }
            ctx.stroke();
            if (focus) {
                ctx.globalAlpha = 0.85;
                ctx.beginPath();
                for (i = 0; i < links.length; i++) {
                    var fl = links[i];
                    if (fl.source.id !== focus && fl.target.id !== focus) continue;
                    ctx.moveTo(sx(fl.source), sy(fl.source));
                    ctx.lineTo(sx(fl.target), sy(fl.target));
                }
                ctx.stroke();
            }

            // nodes
            for (i = 0; i < nodes.length; i++) {
                nd = nodes[i];
                var x = sx(nd), y = sy(nd), r = nd.r * sc;
                if (x < -30 || y < -30 || x > w + 30 || y > h + 30) continue;
                var on = !focus || nd.id === focus || (nb && nb.has(nd.id));
                ctx.globalAlpha = on ? 1 : 0.15;
                ctx.beginPath();
                ctx.arc(x, y, r, 0, Math.PI * 2);
                if (nd.kind === 'unresolved') {
                    ctx.lineWidth = 1.5;
                    ctx.strokeStyle = colors.unresolved;
                    ctx.stroke();
                } else {
                    ctx.fillStyle = nd.kind === 'tag' ? colors.tag : (nd.current ? colors.current : colors.node);
                    ctx.fill();
                    if (nd.current) {
                        ctx.lineWidth = 1.5;
                        ctx.strokeStyle = colors.current;
                        ctx.globalAlpha = on ? 0.6 : 0.1;
                        ctx.beginPath();
                        ctx.arc(x, y, r + 3.5, 0, Math.PI * 2);
                        ctx.stroke();
                    }
                }
                if (nd.id === kbId) {
                    ctx.globalAlpha = 1;
                    ctx.lineWidth = 2;
                    ctx.strokeStyle = colors.text;
                    ctx.beginPath();
                    ctx.arc(x, y, r + 5, 0, Math.PI * 2);
                    ctx.stroke();
                }
            }

            // labels
            ctx.font = '11px ' + fontFamily;
            ctx.textAlign = 'center';
            ctx.textBaseline = 'top';
            ctx.lineJoin = 'round';
            ctx.lineWidth = 3;
            for (i = 0; i < nodes.length; i++) {
                nd = nodes[i];
                var special = nd.id === focus || nd.id === kbId || (nb && nb.has(nd.id));
                if (!special && view.k <= 1.1) continue;
                var lx = sx(nd), ly = sy(nd);
                if (lx < -100 || ly < -30 || lx > w + 100 || ly > h + 30) continue;
                ctx.globalAlpha = (!focus || special) ? 1 : 0.15;
                var ty = ly + nd.r * sc + 3;
                var text = truncate(nd.label != null ? nd.label : nd.id);
                var half = ctx.measureText(text).width / 2 + 3;
                if (half * 2 < w) lx = Math.max(half, Math.min(w - half, lx));
                ctx.strokeStyle = colors.background;
                ctx.strokeText(text, lx, ty);
                ctx.fillStyle = colors.text;
                ctx.fillText(text, lx, ty);
            }
            ctx.globalAlpha = 1;

            var fv = false;
            try { fv = canvas.matches(':focus-visible'); } catch (e) { /* unsupported */ }
            if (fv) {
                ctx.strokeStyle = colors.current;
                ctx.globalAlpha = 0.5;
                ctx.lineWidth = 2;
                ctx.strokeRect(1, 1, w - 2, h - 2);
                ctx.globalAlpha = 1;
            }
        }

        function fitTarget() {
            if (!nodes.length || !w || !h) return null;
            var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
            for (var i = 0; i < nodes.length; i++) {
                var n = nodes[i];
                minX = Math.min(minX, n.x - n.r); maxX = Math.max(maxX, n.x + n.r);
                minY = Math.min(minY, n.y - n.r); maxY = Math.max(maxY, n.y + n.r);
            }
            var bw = maxX - minX, bh = maxY - minY;
            var pad = Math.min(80, Math.min(w, h) * 0.2);
            var k = Math.min(2, (w - pad) / Math.max(bw, 1), (h - pad) / Math.max(bh, 1));
            k = Math.max(0.1, Math.min(6, k));
            return { k: k, x: w / 2 - (minX + maxX) / 2 * k, y: h / 2 - (minY + maxY) / 2 * k };
        }

        function fit() {
            var t = fitTarget();
            if (t) { view.k = t.k; view.x = t.x; view.y = t.y; }
            autoFit = false;
            schedule();
        }

        function userMoved() { autoFit = false; }

        function frame() {
            raf = 0;
            if (destroyed) return;
            var again = false;
            if (sim && (!sim.stopped || (drag && drag.node))) {
                var per = nodes.length < 400 ? 2 : 1;
                for (var t = 0; t < per && !sim.stopped; t++) { sim.tick(); tickCount++; }
                again = true;
            }
            if (autoFit) {
                var tg = fitTarget();
                if (tg && (fitDone || tickCount >= 30 || (sim && sim.stopped))) {
                    fitDone = true;
                    var d = Math.abs(tg.x - view.x) + Math.abs(tg.y - view.y) + Math.abs(tg.k - view.k) * 100;
                    var f = d < 0.5 ? 1 : 0.18;
                    view.k += (tg.k - view.k) * f;
                    view.x += (tg.x - view.x) * f;
                    view.y += (tg.y - view.y) * f;
                    if (d >= 0.5) again = true;
                }
            }
            draw();
            if (again) schedule();
        }

        function schedule() {
            if (raf || destroyed) return;
            raf = win.requestAnimationFrame(frame);
        }

        function resize() {
            if (destroyed) return;
            var rect = host.getBoundingClientRect();
            var nw = Math.round(rect.width), nh = Math.round(rect.height);
            var nd = win.devicePixelRatio || 1;
            if (!nw || !nh) return;
            if (w) { view.x += (nw - w) / 2; view.y += (nh - h) / 2; } else { view.x = nw / 2; view.y = nh / 2; }
            var changed = nw !== w || nh !== h || nd !== dpr;
            w = nw; h = nh; dpr = nd;
            if (changed) {
                canvas.width = Math.round(w * dpr);
                canvas.height = Math.round(h * dpr);
            }
            readColors();
            if (autoFit && reduced) {
                var t = fitTarget();
                if (t) { view.k = t.k; view.x = t.x; view.y = t.y; }
            }
            draw();
            schedule();
        }

        function setData(data) {
            data = data || {};
            readColors();
            var prev = byId;
            var next = new Map(), list = [], i;
            var inNodes = data.nodes || [];
            for (i = 0; i < inNodes.length; i++) {
                var src = inNodes[i];
                if (src == null || src.id == null || next.has(src.id)) continue;
                var nd = { id: src.id, label: src.label != null ? src.label : String(src.id), kind: src.kind || 'note', current: !!src.current };
                var old = prev.get(src.id);
                if (old) { nd.x = old.x; nd.y = old.y; nd.vx = old.vx; nd.vy = old.vy; if (old.fx != null) { nd.fx = old.fx; nd.fy = old.fy; } }
                next.set(nd.id, nd);
                list.push(nd);
            }
            var links = [], nbrs = new Map();
            var inLinks = data.links || [];
            for (i = 0; i < inLinks.length; i++) {
                var s = inLinks[i].source, t = inLinks[i].target;
                if (s && typeof s === 'object') s = s.id;
                if (t && typeof t === 'object') t = t.id;
                if (!next.has(s) || !next.has(t) || s === t) continue;
                links.push({ source: s, target: t });
                if (!nbrs.has(s)) nbrs.set(s, new Set());
                if (!nbrs.has(t)) nbrs.set(t, new Set());
                nbrs.get(s).add(t);
                nbrs.get(t).add(s);
            }
            // seed new nodes next to an already placed neighbour
            var hadPositions = false;
            for (i = 0; i < list.length; i++) if (list[i].x != null) { hadPositions = true; break; }
            if (hadPositions) {
                for (i = 0; i < list.length; i++) {
                    var cur = list[i];
                    if (cur.x != null) continue;
                    var set = nbrs.get(cur.id);
                    if (!set) continue;
                    var found = null;
                    set.forEach(function (nid) { var o = next.get(nid); if (!found && o && o.x != null) found = o; });
                    if (found) {
                        var ang = i * GOLDEN_ANGLE;
                        cur.x = found.x + Math.cos(ang) * 30;
                        cur.y = found.y + Math.sin(ang) * 30;
                    }
                }
            }
            nodes = list;
            byId = next;
            adj = nbrs;
            if (hoverId && !byId.has(hoverId)) hoverId = null;
            if (highlightId && !byId.has(highlightId)) highlightId = null;
            if (kbId && !byId.has(kbId)) kbId = null;
            if (drag && drag.node && !byId.has(drag.node.id)) drag = null;
            kbOrder = null;
            sim = createSimulation({ nodes: nodes, links: links, seed: 1, alpha: hadPositions ? 0.5 : 1 });
            canvas.setAttribute('aria-label', label + ' – ' + nodes.length + ' nodes');
            tickCount = 0;
            if (!hadPositions) { fitDone = false; autoFit = true; }
            if (reduced) {
                sim.run(300);
                if (autoFit) {
                    var tg = fitTarget();
                    if (tg) { view.k = tg.k; view.x = tg.x; view.y = tg.y; fitDone = true; }
                }
                draw();
            } else {
                schedule();
            }
        }

        // ---- interaction
        function pointer(e) {
            var rect = canvas.getBoundingClientRect();
            return { x: e.clientX - rect.left, y: e.clientY - rect.top };
        }
        function hit(px, py) {
            var sc = scale();
            for (var i = nodes.length - 1; i >= 0; i--) {
                var n = nodes[i];
                var dx = sx(n) - px, dy = sy(n) - py, r = n.r * sc + 3;
                if (dx * dx + dy * dy <= r * r) return n;
            }
            return null;
        }
        function setHover(id) {
            if (id === hoverId) return;
            hoverId = id;
            canvas.style.cursor = id ? 'pointer' : (drag ? 'grabbing' : '');
            onHover(id);
            schedule();
        }

        function onPointerDown(e) {
            if (e.button !== 0 && e.pointerType === 'mouse') return;
            var p = pointer(e);
            var n = hit(p.x, p.y);
            drag = { id: e.pointerId, node: n, sx: p.x, sy: p.y, moved: false, vx: view.x, vy: view.y };
            try { canvas.setPointerCapture(e.pointerId); } catch (err) { /* ignore */ }
            if (!n) canvas.style.cursor = 'grabbing';
        }
        function onPointerMove(e) {
            var p = pointer(e);
            if (drag && drag.id === e.pointerId) {
                var dx = p.x - drag.sx, dy = p.y - drag.sy;
                if (!drag.moved && Math.hypot(dx, dy) >= 4) drag.moved = true;
                if (drag.moved) {
                    if (drag.node) {
                        drag.node.fx = (p.x - view.x) / view.k;
                        drag.node.fy = (p.y - view.y) / view.k;
                        sim.reheat(0.3);
                        if (reduced) { sim.run(40); draw(); } else schedule();
                    } else {
                        userMoved();
                        view.x = drag.vx + dx;
                        view.y = drag.vy + dy;
                        schedule();
                    }
                }
                return;
            }
            var n = hit(p.x, p.y);
            setHover(n ? n.id : null);
        }
        function endDrag(e, cancelled) {
            if (!drag || drag.id !== e.pointerId) return;
            var d = drag;
            drag = null;
            try { canvas.releasePointerCapture(e.pointerId); } catch (err) { /* ignore */ }
            if (d.node) {
                delete d.node.fx;
                delete d.node.fy;
                if (!d.moved && !cancelled) onOpen(d.node.id, e);
            }
            var p = pointer(e);
            var n = cancelled ? null : hit(p.x, p.y);
            canvas.style.cursor = n ? 'pointer' : '';
            if (n && n.id !== hoverId) setHover(n.id); else if (!n) setHover(null);
            schedule();
        }
        function onPointerUp(e) { endDrag(e, false); }
        function onPointerCancel(e) { endDrag(e, true); }
        function onPointerLeave() { if (!drag) setHover(null); }

        function zoomAt(px, py, factor) {
            var nk = Math.max(0.1, Math.min(6, view.k * factor));
            var r = nk / view.k;
            view.x = px - (px - view.x) * r;
            view.y = py - (py - view.y) * r;
            view.k = nk;
            userMoved();
            schedule();
        }
        function onWheel(e) {
            e.preventDefault();
            var p = pointer(e);
            var dy = e.deltaMode === 1 ? e.deltaY * 16 : e.deltaY;
            zoomAt(p.x, p.y, Math.exp(-dy * 0.0015));
        }

        function sortedNodes() {
            if (!kbOrder) {
                kbOrder = nodes.slice().sort(function (a, b) {
                    return String(a.label).localeCompare(String(b.label));
                });
            }
            return kbOrder;
        }
        function setKb(id) {
            kbId = id;
            var n = id ? byId.get(id) : null;
            if (n) {
                var x = sx(n), y = sy(n);
                if (x < 20 || y < 20 || x > w - 20 || y > h - 20) {
                    view.x += w / 2 - x;
                    view.y += h / 2 - y;
                    userMoved();
                }
            }
            onHover(id);
            schedule();
        }
        function onKeyDown(e) {
            if (e.ctrlKey || e.metaKey || e.altKey) return;
            var key = e.key;
            if (key === 'ArrowRight' || key === 'ArrowLeft') {
                var order = sortedNodes();
                if (!order.length) return;
                e.preventDefault();
                var idx = -1;
                for (var i = 0; i < order.length; i++) if (order[i].id === kbId) { idx = i; break; }
                var dir = key === 'ArrowRight' ? 1 : -1;
                idx = idx < 0 ? (dir > 0 ? 0 : order.length - 1) : (idx + dir + order.length) % order.length;
                setKb(order[idx].id);
            } else if (key === 'Enter') {
                if (kbId && byId.has(kbId)) { e.preventDefault(); onOpen(kbId, e); }
            } else if (key === '+' || key === '=') {
                e.preventDefault();
                zoomAt(w / 2, h / 2, 1.25);
            } else if (key === '-' || key === '_') {
                e.preventDefault();
                zoomAt(w / 2, h / 2, 1 / 1.25);
            } else if (key === '0') {
                e.preventDefault();
                fit();
            }
        }
        function onBlur() {
            if (kbId) { kbId = null; onHover(null); }
            schedule();
        }
        function onFocus() { schedule(); }

        canvas.addEventListener('pointerdown', onPointerDown);
        canvas.addEventListener('pointermove', onPointerMove);
        canvas.addEventListener('pointerup', onPointerUp);
        canvas.addEventListener('pointercancel', onPointerCancel);
        canvas.addEventListener('pointerleave', onPointerLeave);
        canvas.addEventListener('wheel', onWheel, { passive: false });
        canvas.addEventListener('keydown', onKeyDown);
        canvas.addEventListener('blur', onBlur);
        canvas.addEventListener('focus', onFocus);

        var observer = null;
        if (typeof win.ResizeObserver === 'function') {
            observer = new win.ResizeObserver(function () { resize(); });
            observer.observe(host);
        }
        canvas.setAttribute('aria-label', label + ' – 0 nodes');
        resize();

        return {
            setData: setData,
            highlight: function (id) {
                highlightId = id != null && byId.has(id) ? id : null;
                schedule();
            },
            fit: fit,
            resize: resize,
            nodeCount: function () { return nodes.length; },
            screenPosition: function (id) {
                var n = byId.get(id);
                if (!n || n.x == null) return null;
                return { x: sx(n), y: sy(n) };
            },
            destroy: function () {
                if (destroyed) return;
                destroyed = true;
                if (raf) win.cancelAnimationFrame(raf);
                raf = 0;
                if (observer) observer.disconnect();
                canvas.removeEventListener('pointerdown', onPointerDown);
                canvas.removeEventListener('pointermove', onPointerMove);
                canvas.removeEventListener('pointerup', onPointerUp);
                canvas.removeEventListener('pointercancel', onPointerCancel);
                canvas.removeEventListener('pointerleave', onPointerLeave);
                canvas.removeEventListener('wheel', onWheel);
                canvas.removeEventListener('keydown', onKeyDown);
                canvas.removeEventListener('blur', onBlur);
                canvas.removeEventListener('focus', onFocus);
                if (canvas.parentNode) canvas.parentNode.removeChild(canvas);
            }
        };
    }

    // --------------------------------------------------------------- graph data

    var ATTACHMENT_EXT = /\.(png|jpe?g|gif|webp|svg|bmp|avif|ico|pdf|mp3|wav|ogg|m4a|flac|mp4|webm|mov|mkv|zip|csv|json|txt)$/i;

    function wikiTarget(value) {
        var v = String(value == null ? '' : value);
        var bar = -1;
        for (var i = 0; i < v.length; i++) {
            if (v.charAt(i) === '\\' && v.charAt(i + 1) === '|') { i++; continue; }
            if (v.charAt(i) === '|') { bar = i; break; }
        }
        if (bar >= 0) v = v.slice(0, bar);
        var hash = v.indexOf('#');
        if (hash >= 0) v = v.slice(0, hash);
        return v.split('\\|').join('|').trim();
    }

    function baseTitle(path) {
        return String(path || '').split('/').pop().replace(/\.md$/i, '');
    }

    // Pure: builds {nodes, links} from the notes index.
    // options: linkPath(note, link) -> resolved path or '', tags, unresolved, orphans (default true),
    // filter, fuzzy(query, text) -> score, current (path).
    function fromIndex(notes, options) {
        options = options || {};
        notes = notes || [];
        var linkPath = options.linkPath || function (n, l) { return l.path || ''; };
        var withTags = !!options.tags, withUnresolved = !!options.unresolved, withOrphans = options.orphans !== false;
        var match = options.fuzzy || function (q, text) {
            return String(text).toLowerCase().indexOf(String(q).toLowerCase()) >= 0 ? 1 : 0;
        };

        var noteById = new Map(), order = [], i, j;
        for (i = 0; i < notes.length; i++) {
            var n = notes[i];
            if (!n || !n.path || noteById.has(n.path)) continue;
            noteById.set(n.path, n);
            order.push(n);
        }

        var edgeSeen = new Set(), edges = [], degree = new Map();
        var extra = new Map(); // tag and unresolved nodes
        function addEdge(a, b) {
            if (a === b) return;
            var key = a < b ? a + '\n' + b : b + '\n' + a;
            if (edgeSeen.has(key)) return;
            edgeSeen.add(key);
            edges.push({ source: a, target: b });
            degree.set(a, (degree.get(a) || 0) + 1);
            degree.set(b, (degree.get(b) || 0) + 1);
        }
        for (i = 0; i < order.length; i++) {
            var note = order[i], links = note.links || [];
            for (j = 0; j < links.length; j++) {
                var link = links[j], path = linkPath(note, link) || '';
                if (path) {
                    if (/\.md$/i.test(path) && noteById.has(path)) addEdge(note.path, path);
                    continue;
                }
                if (!withUnresolved || link.embed || (link.kind && link.kind !== 'wiki')) continue;
                var target = wikiTarget(link.target != null ? link.target : link.value);
                if (!target || ATTACHMENT_EXT.test(target)) continue;
                target = target.replace(/\.md$/i, '');
                if (!target) continue;
                var uid = 'unresolved:' + target.toLowerCase();
                if (!extra.has(uid)) extra.set(uid, { id: uid, label: target, kind: 'unresolved' });
                addEdge(note.path, uid);
            }
            if (withTags) {
                var seenTags = {}, tags = note.tags || [];
                for (j = 0; j < tags.length; j++) {
                    var tag = String(tags[j] || '').trim();
                    if (!tag || seenTags[tag]) continue;
                    seenTags[tag] = true;
                    var tid = 'tag:' + tag;
                    if (!extra.has(tid)) extra.set(tid, { id: tid, label: '#' + tag, kind: 'tag' });
                    addEdge(note.path, tid);
                }
            }
        }

        var keep = new Set(), nodes = [];
        var filter = String(options.filter || '').trim();
        for (i = 0; i < order.length; i++) {
            var cand = order[i];
            if (!withOrphans && !degree.get(cand.path)) continue;
            if (filter && !(match(filter, cand.title || baseTitle(cand.path)) > 0)) continue;
            keep.add(cand.path);
        }
        // tag and unresolved nodes stay while they touch a kept note
        for (i = 0; i < edges.length; i++) {
            var ed = edges[i];
            if (extra.has(ed.target) && keep.has(ed.source)) keep.add(ed.target);
            else if (extra.has(ed.source) && keep.has(ed.target)) keep.add(ed.source);
        }
        for (i = 0; i < order.length; i++) {
            if (!keep.has(order[i].path)) continue;
            nodes.push({ id: order[i].path, label: order[i].title || baseTitle(order[i].path), kind: 'note', current: options.current === order[i].path });
        }
        extra.forEach(function (v, id) { if (keep.has(id)) nodes.push(v); });
        var out = [];
        for (i = 0; i < edges.length; i++) if (keep.has(edges[i].source) && keep.has(edges[i].target)) out.push(edges[i]);
        return { nodes: nodes, links: out };
    }

    // Pure: the part of a graph within `depth` hops of `centerId`.
    function neighborhood(graph, centerId, depth) {
        var nodes = (graph && graph.nodes) || [], links = (graph && graph.links) || [];
        var byId = new Map(), adj = new Map(), i;
        for (i = 0; i < nodes.length; i++) byId.set(nodes[i].id, nodes[i]);
        if (!byId.has(centerId)) return { nodes: [], links: [] };
        for (i = 0; i < links.length; i++) {
            var a = links[i].source, b = links[i].target;
            if (!adj.has(a)) adj.set(a, []);
            if (!adj.has(b)) adj.set(b, []);
            adj.get(a).push(b);
            adj.get(b).push(a);
        }
        var seen = new Set([centerId]), frontier = [centerId];
        for (var d = 0; d < Math.max(1, depth | 0); d++) {
            var next = [];
            for (i = 0; i < frontier.length; i++) {
                var list = adj.get(frontier[i]) || [];
                for (var k = 0; k < list.length; k++) if (!seen.has(list[k])) { seen.add(list[k]); next.push(list[k]); }
            }
            frontier = next;
        }
        return {
            nodes: nodes.filter(function (nd) { return seen.has(nd.id); }),
            links: links.filter(function (l) { return seen.has(l.source) && seen.has(l.target); })
        };
    }

    return { createSimulation: createSimulation, create: create, fromIndex: fromIndex, neighborhood: neighborhood };
});
