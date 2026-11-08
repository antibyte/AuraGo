// Persistent Pixi scene graph for catalog v2; the existing compiler, text layout and frame clock remain authoritative.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var V = function () { return root.StorymakerVendor; };
    var RG = function () { return SM.rig; };
    function softwareGL(gl) {
        try {
            var info = gl.getExtension('WEBGL_debug_renderer_info');
            return !!info && /swiftshader|llvmpipe|softpipe|swrast|basic render driver/i.test(gl.getParameter(info.UNMASKED_RENDERER_WEBGL));
        } catch (_) { return false; }
    }
    function matrix(values) { return new (V().Matrix)(values[0], values[1], values[2], values[3], values[4], values[5]); }
    function dimensions(entry, part, state, stage) {
        var vb = part.viewbox || [0, 0, 100, 100], anchor = entry.anchor || { x: 0.5, y: 1 };
        var scale = (entry.size && entry.size.h || 0.15) * SM.stage.DESIGN_H * stage.depthScale(state.y, state.depth) * state.scale / vb[3];
        return RG().mul(RG().translate(state.x, state.y), RG().mul(RG().rotate(state.rot), RG().mul([scale,0,0,scale,0,0], RG().translate(-(vb[0] + anchor.x * vb[2]), -(vb[1] + anchor.y * vb[3])))));
    }
    function character(owner, state, time, rc, entity, scene) {
        var entry = rc.cat.byId[state.character], descriptor = entry && owner.files[entry.file];
        if (!descriptor) throw new Error('Missing character artwork: ' + state.character);
        var rigState = SM.render.rigState(state, time, rc, rc.assets.rig);
        SM.rasterRig.adjust(rigState, state, entity, time, rc, SM.render.stageFor(scene, rc));
        var slots = descriptor.views && descriptor.views[rigState.view];
        if (!slots) throw new Error('Missing character view: ' + rigState.view);
        var seated = state.walkTime === null && state.pose && state.pose.u >= 0.999 && (state.pose.b === 'sit' || state.pose.b === 'typing') && descriptor.seated && descriptor.seated[rigState.view];
        if (seated) slots = slots.filter(function (slot) { return !/leg|shoe/.test(slot.name); }).map(function (slot) { return slot.name === 'hips' ? seated : slot; });
        if (seated || !rigState.planted && state.walkTime !== null && state.walkTime !== undefined) {
            var worlds = RG().boneMatrices(rc.assets.rig,rigState.view,RG().bodyOf(rc.assets.rig,state.params.body),rigState.offsets), floor = -Infinity;
            slots.filter(function (slot) { return seated ? slot === seated : slot.name === 'shoe_l' || slot.name === 'shoe_r'; }).forEach(function (slot) {
                var m = SM.rasterRig.slotMatrix(worlds[slot.bone], slot), b = slot.box;
                [b[0],b[0]+b[2]].forEach(function (x) { [b[1],b[1]+b[3]].forEach(function (y) { floor = Math.max(floor,m[1]*x+m[3]*y+m[5]); }); });
            });
            if (Number.isFinite(floor)) rigState.offsetY = -floor;
        }
        return { descriptor: descriptor, state: rigState, slots: slots };
    }
    function slotImage(slot, state) {
        if (slot.variants) {
            var pick = slot.kind === 'head' ? (state.blink ? 'blink' : state.mouth && state.mouth !== 'rest' ? 'mouth.'+state.mouth : state.expression || 'neutral')
                : slot.kind === 'mouth' ? (state.mouth || state.expr.mouth || 'rest')
                : slot.kind === 'eyes' ? (state.blink ? 'blink' : state.expr.eyes || 'neutral')
                : slot.kind === 'brows' ? state.expr.brows || 'neutral'
                : slot.kind === 'hand' ? state.hands[slot.side] || 'open' : 'default';
            var variants = slot.kind === 'hand' && state.handViews && state.handViews[slot.side] === 'edge' && slot.edgeVariants || slot.variants;
            return variants[pick] || variants.default || variants.rest || variants.neutral;
        }
        return slot.image;
    }
    function required(owner, draws, rc) {
        var wanted = new Set();
        draws.forEach(function (draw) {
            var sc = draw.scene;
            if (sc.kind === 'code') return;
            var environment = sc.env && owner.files[sc.env.file];
            if (environment) (environment.variants && environment.variants[sc.variant] || environment.layers || []).forEach(function (layer) { if (layer.image) wanted.add(layer.image.file); });
            (sc.entities || []).forEach(function (entity) {
                var state = SM.scene.evalEntity(entity, draw.t);
                if (!(state.alpha > 0)) return;
                if (state.type === 'actor') {
                    var actor = character(owner,state,draw.t,rc,entity,sc);
                    actor.slots.forEach(function (slot) { var image=slotImage(slot,actor.state); if(image)wanted.add(image.file); });
                }
                if (state.type === 'prop') {
                    var entry = rc.cat.byId[state.item], prop = entry && owner.files[entry.file];
                    if (prop && prop.image) wanted.add(prop.image.file);
                }
            });
        });
        return wanted;
    }
    async function prepare(owner, draws, rc) {
        var generation = owner.graphicsGeneration;
        var wanted = required(owner, draws, rc), meta = SM.textures.index(owner.catalog), budget = 0, assets = {};
        var active = new Set(draws.map(function (draw) { return draw.scene.id || rc.sceneId || 'code'; }));
        Object.keys(owner.renderers).forEach(function (k) { owner.renderers[k].prune(active); });
        wanted.forEach(function (p) {
            if (!meta[p]) throw new Error('Unlisted scene texture');
            budget += meta[p].width * meta[p].height * 4;
            if (!owner.bitmaps[p]) assets['texture:' + p] = owner.textureData['texture:' + p];
        });
        if (budget > 512 * 1048576) throw new Error('The active scene exceeds the 512 MiB texture budget');
        Object.keys(owner.bitmaps).forEach(function (p) {
            if (wanted.has(p)) return;
            Object.keys(owner.renderers).forEach(function (k) { owner.renderers[k].dropTexture(p); });
            owner.bitmaps[p].close(); delete owner.bitmaps[p];
        });
        await SM.textures.decode(owner.catalog, assets, owner.bitmaps, function () { return owner.graphicsGeneration === generation; });
        if (generation !== owner.graphicsGeneration) throw new Error('Project was disposed');
        var drawer = rc.assets.raster;
        if (drawer && (!drawer.ready || Object.keys(assets).length)) {
            var sink = { setTransform: function () {}, drawImage: function () {} };
            draws.forEach(function (draw) {
                if (draw.scene.kind === 'code') return;
                var scene = Object.assign({}, draw.scene, { id: draw.scene.id || rc.sceneId || 'code', variant: draw.scene.variant || 'day' });
                drawer.draw(sink, scene, draw.t, rc);
            });
            if (drawer.renderer.gl) drawer.renderer.gl.finish();
            drawer.ready = true;
        }
    }
    function Drawer(owner, renderer, canvas) {
        this.owner = owner; this.renderer = renderer; this.canvas = canvas;
        this.metadata = SM.textures.index(owner.catalog);
        this.root = new (V().Container)(); this.nodes = new Map(); this.textures = new Map(); this.used = new Set();
        this.overlay = owner.env.makeCanvas(canvas.width, canvas.height);
        this.overlayContext = this.overlay.getContext('2d');
        this.overlayTexture = new (V().Texture)({ source: new (V().CanvasSource)({ resource: this.overlay, autoGarbageCollect: false }) });
        this.overlaySprite = new (V().Sprite)(this.overlayTexture);
        this.root.addChild(this.overlaySprite);
    }
    Drawer.prototype.texture = function (image) {
        var key = image.file + ':' + (image.rect || []).join(',');
        if (!this.textures.has(key)) {
            var meta = this.metadata[image.file], rect = image.rect;
            if (!meta || rect && (rect.length !== 4 || rect.some(function (n) { return !Number.isInteger(n); }) || rect[0] < 0 || rect[1] < 0 || rect[2] < 1 || rect[3] < 1 || rect[0] + rect[2] > meta.width || rect[1] + rect[3] > meta.height)) throw new Error('Invalid atlas region');
            var sourceKey = image.file + ':source', source = this.textures.get(sourceKey);
            if (!source) {
                var bitmap = this.owner.bitmaps[image.file];
                if (!bitmap) throw new Error('Texture is not ready: ' + image.file);
                source = new (V().ImageSource)({ resource: bitmap, autoGarbageCollect: false });
                this.textures.set(sourceKey, source);
            }
            var r = image.rect;
            this.textures.set(key, new (V().Texture)({ source: source, frame: r ? new (V().Rectangle)(r[0],r[1],r[2],r[3]) : undefined }));
        }
        return this.textures.get(key);
    };
    Drawer.prototype.dropTexture = function (file) {
        var self = this;
        Array.from(this.textures.keys()).filter(function (k) { return k.indexOf(file + ':') === 0 && k !== file + ':source'; }).forEach(function (k) { self.textures.get(k).destroy(false); self.textures.delete(k); });
        var source = this.textures.get(file + ':source');
        if (source) { source.destroy(); this.textures.delete(file + ':source'); }
    };
    Drawer.prototype.prune = function (active) {
        var self = this;
        this.nodes.forEach(function (node, key) { if (!active.has(key.split(':')[0])) { node.destroy(); self.nodes.delete(key); } });
    };
    Drawer.prototype.sprite = function (key, image, transform, box, alpha, z, clip) {
        var sprite = this.nodes.get(key);
        if (!sprite) { sprite = new (V().Sprite)(); this.nodes.set(key, sprite); this.root.addChild(sprite); }
        this.used.add(key); sprite.visible = true; sprite.texture = this.texture(image);
        var r = image.rect, width = r ? r[2] : sprite.texture.width, height = r ? r[3] : sprite.texture.height;
        var local = [box[2] / width, 0, 0, box[3] / height, box[0], box[1]];
        sprite.setFromMatrix(matrix(RG().mul(transform, local))); sprite.alpha = alpha; sprite.zIndex = z;
        var maskKey = key + ':cap', mask = this.nodes.get(maskKey), cap = clip && clip.shoulderCap, torso = clip && clip.torsoCap;
        if (cap > 0 || torso) {
            if (!mask) { mask = new (V().Graphics)(); this.nodes.set(maskKey, mask); this.root.addChild(mask); }
            var shape = JSON.stringify([cap, torso, box]);
            if (mask.shapeKey !== shape) {
                mask.clear();
                if (torso) {
                    var x = box[0], y = torso.y, w = box[2], bottom = box[1] + box[3], r = torso.radius;
                    mask.moveTo(x + r, y).lineTo(x + w - r, y).quadraticCurveTo(x + w, y, x + w, y + r)
                        .lineTo(x + w, bottom).lineTo(x, bottom).lineTo(x, y + r).quadraticCurveTo(x, y, x + r, y).closePath()
                        .rect(-torso.neck, box[1], torso.neck * 2, y + r - box[1]).fill(0xffffff);
                } else mask.rect(-cap, 0, 2 * cap, box[1] + box[3]).circle(0, 0, cap).fill(0xffffff);
                mask.shapeKey = shape;
            }
            mask.setFromMatrix(matrix(transform)); sprite.mask = mask;
            this.used.add(maskKey); mask.visible = true;
        } else if (sprite.mask) sprite.mask = null;
        return sprite;
    };
    Drawer.prototype.draw = function (ctx, sc, time, rc) {
        var self = this, stage = SM.render.stageFor(sc, rc), cam = SM.camera.at(sc.camera, time);
        var win = stage.cameraWindow(cam), base = stage.baseWindow(), camera = stage.matrix(win), z = 0;
        this.used.clear(); this.root.sortableChildren = true;
        this.renderer.background.color = rc.palette.bg;
        var environment = sc.env && this.owner.files[sc.env.file];
        var layers = environment && (environment.variants && environment.variants[sc.variant] || environment.layers) || [];
        function background(layer, i) {
            var factor = layer.parallax === undefined ? 1 : layer.parallax;
            var m = RG().mul(camera, RG().translate((1-factor)*(win.x-base.x), (1-factor)*(win.y-base.y)));
            self.sprite(sc.id + ':env:' + i, layer.image, m, layer.box || [0,0,1920,1080], 1, z++);
        }
        layers.forEach(function (layer, i) { if (!layer.foreground) background(layer, i); });
        var states = sc.entities.map(function (e, i) { var s = SM.scene.evalEntity(e, time); s.k = i; s.entity = e; s.scale *= SM.render.pulseOf(s); return s; });
        function prop(state) {
            var entry = rc.cat.byId[state.item], part = entry && self.owner.files[entry.file];
            if (!part || !part.image) throw new Error('Missing prop artwork: ' + state.item);
            self.sprite(sc.id + ':' + state.id, part.image, RG().mul(camera, dimensions(entry, part, state, stage)), part.viewbox || [0,0,100,100], state.alpha, z++);
        }
        states.filter(function (s) { return s.type === 'prop' && s.layer === 'back' && s.alpha > 0; }).forEach(prop);
        states.filter(function (s) { return s.type === 'actor' && s.alpha > 0; }).sort(function (a,b) { return a.y-b.y || a.k-b.k; }).forEach(function (s) {
            var actor = character(self.owner, s, time, rc, s.entity, sc), rig = rc.assets.rig, body = RG().bodyOf(rig, s.params.body);
            var scale = RG().scaleFor(rig, SM.stage.DESIGN_H, stage.depthScale(s.y,s.depth), s.scale, body);
            var m = RG().mul(camera, RG().mul(RG().translate(s.x,s.y), [scale*(s.facing<0?-1:1),0,0,scale,0,0]));
            var shadowKey = sc.id+':'+s.id+':shadow', shadow = self.nodes.get(shadowKey);
            if (!shadow) {
                shadow = new (V().Graphics)().ellipse(0,0,160,28).fill({ color: 0x000000, alpha: 0.13 });
                self.nodes.set(shadowKey,shadow); self.root.addChild(shadow);
            }
            self.used.add(shadowKey); shadow.visible = true; shadow.setFromMatrix(matrix(m)); shadow.alpha = s.alpha; shadow.zIndex = z++;
            m = RG().mul(camera, RG().mul(RG().translate(s.x,s.y), RG().mul(RG().rotate(s.rot), [scale*(s.facing<0?-1:1),0,0,scale,0,0])));
            m = RG().mul(m, RG().translate(0, actor.state.offsetY));
            var worlds = RG().boneMatrices(rig, actor.state.view, body, actor.state.offsets);
            actor.slots.forEach(function (slot, i) {
                var image = slotImage(slot, actor.state);
                if (image) self.sprite(sc.id+':'+s.id+':'+i, image, RG().mul(m, SM.rasterRig.slotMatrix(worlds[slot.bone], slot)), image.box || slot.box, s.alpha, z++, slot);
            });
        });
        layers.forEach(function (layer, i) { if (layer.foreground) background(layer, i); });
        states.filter(function (s) { return s.type === 'prop' && s.layer !== 'back' && s.alpha > 0; }).forEach(prop);
        this.nodes.forEach(function (node, key) { node.visible = self.used.has(key); });
        var overlay = this.overlayContext;
        var visibleOverlay = states.filter(function (s) { return s.alpha > 0 && (s.type === 'shape' || s.type === 'text' && s.text || s.type === 'bubble' && s.lines && s.lines.length || s.highlight && s.highlight.style !== 'pulse'); });
        var owners = new Set(visibleOverlay.filter(function (s) { return s.type === 'bubble'; }).map(function (s) { return s.owner; }));
        var inputs = visibleOverlay.concat(states.filter(function (s) { return owners.has(s.id); })).map(function (s) { var plain = Object.assign({}, s); delete plain.entity; return plain; });
        var overlayKey = visibleOverlay.length ? JSON.stringify([sc.id, cam, inputs]) : '';
        if (this.overlayKey !== overlayKey) {
            // Reset backing pixels only when contents change; transparent CanvasSource uploads must invalidate old pixels.
            this.overlay.width = rc.W;
            states.filter(function (s) { return s.type === 'shape' && s.alpha > 0; }).forEach(function (s) { SM.render.drawShapeEntity(overlay,s,rc,camera); });
            states.filter(function (s) { return s.type === 'text'; }).forEach(function (s) { SM.render.drawText(overlay,s,rc,stage,base); });
            states.filter(function (s) { return s.type === 'bubble'; }).forEach(function (s) { SM.render.drawBubble(overlay,s,states,rc,stage,win); });
            states.forEach(function (s) { SM.render.drawHighlight(overlay,s,rc,stage,base,camera); });
            this.overlayTexture.source.update(); this.overlayKey = overlayKey;
        }
        this.overlaySprite.zIndex = z;
        this.renderer.render({ container: this.root });
        ctx.setTransform(1,0,0,1,0,0); ctx.globalAlpha = 1; ctx.drawImage(this.canvas,0,0);
    };
    Drawer.prototype.dispose = function () {
        this.root.destroy({ children: true });
        Array.from(this.textures.keys()).filter(function (k) { return k.endsWith(':source'); }).forEach(function (k) { this.dropTexture(k.slice(0,-7)); }, this);
        this.overlayTexture.destroy(true); this.renderer.destroy(); this.nodes.clear();
        this.overlay.width = this.overlay.height = this.canvas.width = this.canvas.height = 1;
    };
    async function get(owner, width, height) {
        var generation = owner.graphicsGeneration;
        var key = width + 'x' + height;
        var existing = owner.renderers[key];
        if (existing && existing.renderer.gl && existing.renderer.gl.isContextLost()) { existing.dispose(); delete owner.renderers[key]; }
        if (owner.renderers[key]) return owner.renderers[key];
        if (!V()) throw new Error('The local Storymaker renderer is unavailable');
        V().DOMAdapter.set(typeof root.document === 'undefined' ? V().WebWorkerAdapter : V().BrowserAdapter);
        var canvas = owner.env.makeCanvas(width,height), renderer;
        var options = { canvas: canvas, width: width, height: height, resolution: 1, antialias: true,
            clearBeforeRender: true, backgroundAlpha: 1, textureGCActive: false, renderableGCActive: false };
        try {
            renderer = new (V().WebGLRenderer)(); await renderer.init(options);
            if (SM.raster.softwareGL(renderer.gl)) throw new Error('Software WebGL uses the faster Canvas renderer');
        }
        catch (err) {
            if (renderer) { try { renderer.destroy(); } catch (_) {} }
            if (generation !== owner.graphicsGeneration) throw new Error('Project was disposed');
            canvas = owner.env.makeCanvas(width,height); options.canvas = canvas;
            renderer = new (V().CanvasRenderer)(); await renderer.init(options);
        }
        if (generation !== owner.graphicsGeneration) { renderer.destroy(); throw new Error('Project was disposed'); }
        var sizes = Object.keys(owner.renderers);
        if (sizes.length >= 3) { owner.renderers[sizes[0]].dispose(); delete owner.renderers[sizes[0]]; }
        owner.renderers[key] = new Drawer(owner,renderer,canvas);
        return owner.renderers[key];
    }
    SM.raster = { get: get, prepare: prepare, required: required, softwareGL: softwareGL };
})(typeof globalThis !== 'undefined' ? globalThis : self);
