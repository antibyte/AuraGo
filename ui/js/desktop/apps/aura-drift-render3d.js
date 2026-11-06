/* Aura Drift - 3D renderer orchestrator (three.js).
 *
 * Owns the WebGL renderer, camera, lights, image based light, the per-track module set and the
 * garage showcase. World modules (terrain, road, sky, props, fx, post) are optional and guarded
 * with typeof checks, so each lands independently; without terrain/sky a flat lit ground and a
 * gradient dome stand in. Three space is [x, up, y_sim].
 * Interface: section 1.5 "Renderer interface". The shell swaps to the 2D renderer when
 * onContextLost fires or a frame throws, and back after onContextRestored. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const SOFTWARE = /swiftshader|llvmpipe|software/i;
    const SKY_VERT = 'varying vec3 vDir;\nvoid main() { vDir = normalize(position); gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }';
    const SKY_FRAG = [
        'uniform vec3 uTop; uniform vec3 uHorizon; uniform vec3 uGround; uniform vec3 uFog; uniform vec3 uSun; uniform vec3 uSunDir; uniform float uBoost;',
        'varying vec3 vDir;',
        'void main() {',
        '  vec3 d = normalize(vDir);',
        '  float h = d.y;',
        '  vec3 col = h >= 0.0 ? mix(uHorizon, uTop, pow(clamp(h, 0.0, 1.0), 0.55)) : mix(uHorizon, uGround, smoothstep(0.0, 0.3, -h));',
        '  col = mix(col, uFog, exp(-abs(h) * 16.0) * 0.8);',
        '  float s = max(dot(d, uSunDir), 0.0);',
        '  col += uSun * (pow(s, 1400.0) * 40.0 * uBoost + pow(s, 40.0) * 0.55 + pow(s, 7.0) * 0.2);',
        '  gl_FragColor = vec4(col, 1.0);',
        '#include <tonemapping_fragment>',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');

    function noopPost() {
        return {
            available: false, enabled: false, setQuality() {}, setBiome() {}, resize() {}, impulse() {}, setHoles() {},
            render() { return false; },
            stats() { return { available: false, enabled: false, degraded: true, quality: 'none', biome: 'none' }; },
            dispose() {}
        };
    }

    function skyMaterial(T, look, boost) {
        const c = hex => new T.Color(hex);
        const sd = AD.sunDirection(look, 0);
        return new T.ShaderMaterial({
            uniforms: {
                uTop: { value: c(look.skyTop) }, uHorizon: { value: c(look.skyHorizon) },
                uGround: { value: c(look.ground).multiplyScalar(0.6) }, uFog: { value: c(look.fog) },
                uSun: { value: c(look.sun) }, uSunDir: { value: new T.Vector3(sd[0], sd[1], sd[2]) }, uBoost: { value: boost }
            },
            vertexShader: SKY_VERT, fragmentShader: SKY_FRAG, side: T.BackSide, depthWrite: false, fog: false
        });
    }

    function groundTexture(T, renderer) {
        const c = document.createElement('canvas');
        c.width = c.height = 256;
        const g = c.getContext('2d');
        g.fillStyle = '#ffffff';
        g.fillRect(0, 0, 256, 256);
        let seed = 12345;
        const rnd = () => { seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0; return seed / 4294967296; };
        for (let i = 0; i < 2600; i++) {
            const v = 205 + Math.floor(rnd() * 50);
            g.fillStyle = 'rgba(' + v + ',' + v + ',' + v + ',0.5)';
            g.fillRect(rnd() * 256, rnd() * 256, 1 + rnd() * 3, 1 + rnd() * 3);
        }
        g.strokeStyle = 'rgba(0,0,0,0.12)';
        g.lineWidth = 2;
        g.strokeRect(0, 0, 256, 256);
        const tex = new T.CanvasTexture(c);
        tex.colorSpace = T.SRGBColorSpace;
        tex.wrapS = tex.wrapT = T.RepeatWrapping;
        tex.anisotropy = Math.min(8, renderer.capabilities.getMaxAnisotropy());
        return tex;
    }

    AD.createRenderer3D = function (canvas, settings, t) {
        const T = window.AuraDriftThree;
        if (!T || !T.WebGLRenderer || !AD.LOOK || !AD.assets || typeof AD.createActors !== 'function' || typeof AD.createCarRig !== 'function') return null;
        const hasPost = typeof AD.createPost === 'function';
        let renderer;
        try {
            renderer = new T.WebGLRenderer({ canvas: canvas, antialias: !hasPost, alpha: false, powerPreference: 'high-performance', stencil: false });
        } catch (_) { return null; }
        try {
            const ctx = renderer.getContext();
            const dbg = ctx.getExtension('WEBGL_debug_renderer_info');
            const name = dbg ? String(ctx.getParameter(dbg.UNMASKED_RENDERER_WEBGL)) : '';
            if (SOFTWARE.test(name) && (!settings || settings.quality === 'auto')) {
                renderer.dispose();
                renderer.forceContextLoss();
                return null;
            }
        } catch (_) { /* keep going: the probe is optional */ }

        renderer.outputColorSpace = T.SRGBColorSpace;
        renderer.toneMapping = T.NeutralToneMapping;
        renderer.shadowMap.type = T.PCFShadowMap;
        renderer.info.autoReset = false;
        renderer.setClearColor(0x0b1018, 1);

        const scene = new T.Scene();
        const camera = new T.PerspectiveCamera(60, 16 / 9, 0.3, 5000);
        const hemi = new T.HemisphereLight(0xffffff, 0x444444, 1);
        const sun = new T.DirectionalLight(0xffffff, 3);
        scene.add(hemi, sun, sun.target);
        let lightPool = [];
        scene.userData.lightPool = lightPool;
        const pmrem = new T.PMREMGenerator(renderer);
        const sunDir = new T.Vector3(0, 1, 0);

        let tier = 'high', glq = AD.QUALITY.high.gl;
        let size = { w: 960, h: 540, dpr: 1 };
        let look = AD.LOOK.alpine, track = null, race = null, lost = false, disposed = false, held = true;
        let env = null, envKey = '', wetness = 0, time = 0, gen = 0;
        let actors = null, terrain = null, road = null, sky = null, props = null, fx = null, fallback = null;
        let post = null, realPost = null;
        let shadowMode = '', discoCount = 0;

        AD.assets.acquire();
        AD.assets.loadCore().then(() => { if (!disposed) refreshShowcase(true); });

        function eachMaterial(fn) {
            const seen = new Set();
            scene.traverse(o => {
                const list = Array.isArray(o.material) ? o.material : (o.material ? [o.material] : []);
                for (const m of list) if (!seen.has(m)) { seen.add(m); fn(m); }
            });
        }

        function buildEnv(l, envSize) {
            const skyScene = new T.Scene();
            const geo = new T.SphereGeometry(10, 32, 16);
            const mat = skyMaterial(T, l, 0.3);
            skyScene.add(new T.Mesh(geo, mat));
            const rt = pmrem.fromScene(skyScene, 0.03, 0.1, 100, { size: envSize });
            geo.dispose(); mat.dispose();
            return rt;
        }
        function setEnv(l, key) {
            const k = key + '/' + glq.envSize;
            if (envKey === k && env) return;
            if (env) env.dispose();
            env = buildEnv(l, glq.envSize);
            envKey = k;
            scene.environment = env.texture;
            scene.environmentIntensity = l.env;
        }

        /* ---- quality ---------------------------------------------------- */
        function applySun() {
            const want = glq.shadows ? 'pcf' : 'off';
            if (want !== shadowMode) {
                shadowMode = want;
                renderer.shadowMap.enabled = !!glq.shadows;
                eachMaterial(m => { m.needsUpdate = true; });
            }
            sun.castShadow = !!glq.shadows;
            if (sun.shadow.map) { sun.shadow.map.dispose(); sun.shadow.map = null; }
            if (glq.shadows) {
                const box = glq.shadowBox;
                sun.shadow.mapSize.set(glq.shadows, glq.shadows);
                const sc = sun.shadow.camera;
                sc.left = -box / 2; sc.right = box / 2; sc.top = box / 2; sc.bottom = -box / 2;
                sc.near = 1; sc.far = 900;
                sc.updateProjectionMatrix();
                sun.shadow.bias = -0.0003;
                sun.shadow.normalBias = 0.35;
                sun.shadow.radius = 3;
            }
        }
        function applyPool() {
            for (const l of lightPool) { l.removeFromParent(); l.dispose && l.dispose(); }
            lightPool = [];
            for (let i = 0; i < glq.lights; i++) {
                const l = new T.PointLight(0xffffff, 0, 30, 2);
                lightPool.push(l);
                scene.add(l);
            }
            scene.userData.lightPool = lightPool;
        }
        function applySize() {
            const dpr = Math.min(size.dpr, glq.dpr);
            renderer.setPixelRatio(dpr);
            renderer.setSize(size.w, size.h, true);
            camera.aspect = size.w / size.h;
            camera.updateProjectionMatrix();
        }

        /* ---- look --------------------------------------------------------- */
        function applyLook(l, biomeKey) {
            look = l;
            const d = AD.sunDirection(l, 16);
            sunDir.set(d[0], d[1], d[2]);
            sun.color.set(l.sun);
            sun.intensity = l.sunPower;
            hemi.color.set(l.hemiSky);
            hemi.groundColor.set(l.hemiGround);
            hemi.intensity = l.hemi * (l.night ? 1.0 : 0.6);
            renderer.toneMappingExposure = l.exposure;
            scene.fog = new T.Fog(l.fog, l.fogFar * l.fogNear, l.fogFar);
            scene.background = new T.Color(l.skyHorizon);
            setEnv(l, biomeKey);
        }

        function makeFallback() {
            const grp = new T.Group();
            grp.name = 'stand-in';
            const tex = groundTexture(T, renderer);
            tex.repeat.set(500, 500);
            const gmat = new T.MeshStandardMaterial({ map: tex, color: new T.Color(look.ground).multiplyScalar(look.night ? 1.7 : 1), roughness: 0.92, metalness: 0 });
            const ground = new T.Mesh(new T.PlaneGeometry(8000, 8000).rotateX(-Math.PI / 2), gmat);
            ground.receiveShadow = true;
            const b = track && track.bounds;
            if (b) ground.position.set((b.minX + b.maxX) / 2, 0, (b.minY + b.maxY) / 2);
            grp.add(ground);
            const domeMat = skyMaterial(T, look, 1);
            const dome = new T.Mesh(new T.SphereGeometry(2500, 32, 20), domeMat);
            dome.frustumCulled = false;
            dome.renderOrder = -10;
            grp.add(dome);
            scene.add(grp);
            return { group: grp, ground: ground, dome: dome, dispose() {
                grp.removeFromParent();
                tex.dispose(); gmat.dispose(); ground.geometry.dispose(); dome.geometry.dispose(); domeMat.dispose();
            } };
        }

        /* ---- per-track objects ------------------------------------------- */
        function stop(mod) { if (mod && typeof mod.dispose === 'function') { try { mod.dispose(); } catch (_) { /* partial module */ } } }
        function disposeTrack() {
            stop(actors); stop(terrain); stop(road); stop(sky); stop(props); stop(fx); stop(fallback);
            for (const m of [terrain, road]) if (m && m.group) m.group.removeFromParent();
            actors = terrain = road = sky = props = fx = fallback = null;
            wetness = 0; discoCount = 0;
        }
        function buildTrack() {
            disposeTrack();
            if (!track) return;
            const id = track.biomeId;
            look = AD.LOOK[id] || AD.LOOK.alpine;
            applyLook(look, id);
            if (typeof AD.createTerrain === 'function') {
                terrain = AD.createTerrain(T, track, look, glq);
                if (terrain && terrain.group) scene.add(terrain.group);
            }
            if (typeof AD.createRoad === 'function') {
                road = AD.createRoad(T, track, look, glq);
                if (road && road.group) scene.add(road.group);
            }
            if (typeof AD.createSky === 'function') sky = AD.createSky(T, scene, look, glq, track);
            if (typeof AD.createProps === 'function') props = AD.createProps(T, scene, track, id, glq, t || (k => k));
            if (!terrain && !sky) fallback = makeFallback();
            actors = AD.createActors(T, scene, glq);
            actors.setRace(race);
            if (typeof AD.createFx === 'function') fx = AD.createFx(T, scene, glq);
            const mine = ++gen;
            AD.assets.loadBiome(id).then(() => { if (mine === gen && props && typeof props.rebuild === 'function') props.rebuild(); });
        }

        /* ---- camera and sun ----------------------------------------------- */
        function applyPose(pose) {
            camera.position.set(pose.eye[0], pose.eye[1], pose.eye[2]);
            const b = AD.cameraBasis(pose);
            camera.up.set(b.up[0], b.up[1], b.up[2]);
            camera.lookAt(pose.target[0], pose.target[1], pose.target[2]);
            if (camera.fov !== pose.fov) { camera.fov = pose.fov; camera.updateProjectionMatrix(); }
            camera.updateMatrixWorld();
        }
        function followSun(x, y, z) {
            const d = sunDir;
            const dist = 420;
            if (!glq.shadows) { sun.target.position.set(x, y, z); sun.position.set(x + d.x * dist, y + d.y * dist, z + d.z * dist); return; }
            /* Snap the shadow window to whole shadow texels so the shadows do not shimmer. */
            let rx = d.z, rz = -d.x;
            const rl = Math.hypot(rx, rz) || 1;
            rx /= rl; rz /= rl;
            const ux = d.y * rz, uy = d.z * rx - d.x * rz, uz = -d.y * rx;
            const texel = glq.shadowBox / glq.shadows;
            const px = Math.round((x * rx + z * rz) / texel) * texel;
            const py = Math.round((x * ux + y * uy + z * uz) / texel) * texel;
            const pd = x * d.x + y * d.y + z * d.z;
            const tx = rx * px + ux * py + d.x * pd, ty = uy * py + d.y * pd, tz = rz * px + uz * py + d.z * pd;
            sun.target.position.set(tx, ty, tz);
            sun.position.set(tx + d.x * dist, ty + d.y * dist, tz + d.z * dist);
        }

        /* ---- garage showcase ------------------------------------------------ */
        let show = null, showCfg = { carId: 'vektor', livery: 0, biomeId: 'alpine' };
        let showEnv = null, showEnvKey = '';
        function disposeShow() {
            if (!show) return;
            if (show.rig) show.rig.dispose();
            show.scene.traverse(o => {
                if (o.userData && o.userData.owned) { if (o.geometry) o.geometry.dispose(); if (o.material) o.material.dispose(); }
            });
            if (show.blobTex) show.blobTex.dispose();
            if (show.key.shadow.map) show.key.shadow.map.dispose();
            show = null;
        }
        function buildShow() {
            disposeShow();
            const l = AD.LOOK[showCfg.biomeId] || AD.LOOK.alpine;
            const biome = (AD.BIOMES && AD.BIOMES[showCfg.biomeId]) || { accent: '#ffd166' };
            const sc = new T.Scene();
            const owned = o => { o.userData.owned = true; return o; };
            const cam = new T.PerspectiveCamera(36, size.w / size.h, 0.2, 500);
            /* Backdrop: biome-tinted vertical gradient; the floor fades into its horizon colour. */
            const horizon = new T.Color(l.skyHorizon).multiplyScalar(0.5);
            const back = owned(new T.Mesh(new T.SphereGeometry(200, 32, 16), new T.ShaderMaterial({
                uniforms: { uA: { value: horizon }, uB: { value: new T.Color(l.skyTop).multiplyScalar(0.3) } },
                vertexShader: 'varying vec3 vP; void main(){ vP = normalize(position); gl_Position = projectionMatrix * modelViewMatrix * vec4(position,1.0); }',
                fragmentShader: 'uniform vec3 uA; uniform vec3 uB; varying vec3 vP;\nvoid main(){ float h = max(vP.y, 0.0); vec3 c = mix(uA, uB, pow(h, 0.55));\n gl_FragColor = vec4(c, 1.0);\n#include <tonemapping_fragment>\n#include <colorspace_fragment>\n}',
                side: T.BackSide, depthWrite: false, fog: false
            })));
            back.renderOrder = -10;
            sc.add(back);
            const floor = owned(new T.Mesh(new T.CircleGeometry(160, 48).rotateX(-Math.PI / 2), new T.MeshStandardMaterial({ color: new T.Color(l.ground).multiplyScalar(l.night ? 0.6 : 0.4), roughness: 0.8 })));
            floor.position.y = -0.22;
            floor.receiveShadow = true;
            sc.add(floor);
            sc.fog = new T.Fog(horizon, 16, 110);
            /* The stage (disc, glow ring, props) is scaled to the car in refreshShowcase. */
            const stage = new T.Group();
            const disc = owned(new T.Mesh(new T.CylinderGeometry(3.7, 3.9, 0.22, 56), new T.MeshStandardMaterial({ color: 0x2a303c, roughness: 0.42, metalness: 0.45 })));
            disc.position.y = -0.11;
            disc.receiveShadow = true;
            const ring = owned(new T.Mesh(new T.TorusGeometry(3.74, 0.05, 8, 64).rotateX(Math.PI / 2), new T.MeshStandardMaterial({ color: 0x111111, emissive: new T.Color(biome.accent), emissiveIntensity: 2.2 })));
            stage.add(disc, ring);
            sc.add(stage);
            /* Warm key spot, cool rim, soft dome light. */
            const key = new T.SpotLight(0xffe2b8, 420, 80, 0.55, 0.75, 2);
            key.target.position.set(0, 0.6, 0);
            sc.add(key, key.target);
            const rim = new T.DirectionalLight(new T.Color(l.sun).lerp(new T.Color(0x9ab8ff), 0.6), 2.2);
            sc.add(rim);
            sc.add(new T.HemisphereLight(new T.Color(l.hemiSky), new T.Color(l.hemiGround), l.night ? 0.8 : 0.45));
            show = { scene: sc, cam: cam, key: key, rim: rim, stage: stage, rig: null, blob: null, blobTex: null, props: [], angle: -0.6, tpl: null, look: l };
            if (showEnvKey !== showCfg.biomeId + '/' + glq.envSize) {
                if (showEnv) showEnv.dispose();
                showEnv = buildEnv(l, glq.envSize);
                showEnvKey = showCfg.biomeId + '/' + glq.envSize;
            }
            sc.environment = showEnv.texture;
            sc.environmentIntensity = Math.max(0.8, l.env * 0.9);
            refreshShowcase(true);
        }
        function refreshShowcase(force) {
            if (!show || disposed) return;
            const def = AD.carById(showCfg.carId);
            const tpl = AD.assets.car(def.id);
            if (!force && show.tpl === tpl && show.rigId === def.id + ':' + showCfg.livery) return;
            if (show.rig) show.rig.dispose();
            for (const p of show.props) p.removeFromParent();
            show.props = [];
            show.tpl = tpl;
            show.rigId = def.id + ':' + showCfg.livery;
            const rig = show.rig = AD.createCarRig(T, def, { livery: showCfg.livery | 0, gl: glq, night: show.look.night });
            show.scene.add(rig.group);
            const s = rig.size;
            /* Everything is sized from the car, so a van or a monster truck fits the stage. */
            const k = Math.max(1, (s.l / 2 + 0.9) / 3.7);
            show.stage.scale.set(k, 1, k);
            show.key.position.set(7 * k, 9 * k, 6 * k);
            show.rim.position.set(-7 * k, 4 * k, -7 * k);
            show.key.castShadow = !!glq.shadows;
            if (glq.shadows) { show.key.shadow.mapSize.set(1024, 1024); show.key.shadow.bias = -0.0004; show.key.shadow.normalBias = 0.2; }
            rig.setGl(glq);
            if (!show.blob) {
                show.blobTex = AD.makeBlobTexture(T);
                show.blob = new T.Mesh(new T.PlaneGeometry(1, 1).rotateX(-Math.PI / 2), new T.MeshBasicMaterial({ map: show.blobTex, color: 0, transparent: true, opacity: glq.shadows ? 0.3 : 0.8, depthWrite: false, fog: false }));
                show.blob.userData.owned = true;
                show.blob.position.y = 0.02;
                show.scene.add(show.blob);
            }
            show.blob.scale.set(s.l * 1.35, 1, s.w * 1.5);
            const add = (obj, ang, rad, ry, sc) => {
                if (!obj) return;
                const c = obj.clone(true);
                c.position.set(Math.cos(ang) * rad * k, -0.22, Math.sin(ang) * rad * k);
                c.rotation.y = ry;
                c.scale.setScalar(sc);
                c.traverse(o => { if (o.isMesh) { o.castShadow = !!glq.shadows; o.receiveShadow = !!glq.shadows; } });
                show.scene.add(c);
                show.props.push(c);
            };
            add(AD.assets.prop('shared', 'gnome_statue'), 2.5, 6.4, 0.6, 0.85);
            add(AD.assets.trackPiece('cone'), 0.55, 6.2, 0, 1.2);
            add(AD.assets.trackPiece('cone'), 0.78, 6.5, 0.4, 1.2);
            add(AD.assets.trackPiece('tyre_stack'), 3.9, 6.3, 0.9, 1.1);
            const rb = Math.hypot(s.l / 2, s.w / 2, s.h / 2);
            show.fit = rb;
            show.centre = s.h * 0.45;
        }
        const SHOW_VEHICLE = { x: 0, y: 0, z: 0, psi: 0, roll: 0, pitch: 0, steer: 0, vx: 0, r: 0, brake: 0, handbrake: 0, airborne: false, out: false };
        function drawShowcase(dt, reduced) {
            if (!show) buildShow();
            refreshShowcase(false);
            if (!reduced) show.angle += dt * 0.25;
            const aspect = size.w / size.h;
            const cam = show.cam;
            cam.aspect = aspect;
            cam.fov = 36;
            /* Fit the car's bounding sphere into the narrower of the two view angles. */
            const half = Math.min(18, Math.atan(Math.tan(18 * Math.PI / 180) * aspect) * 180 / Math.PI) * Math.PI / 180;
            const R = show.fit / Math.sin(half) * 1.02;
            /* showCfg.view = { angle, elev } pins the camera (survey and visual checks). */
            const view = showCfg.view;
            const a = view ? view.angle : show.angle, elev = view && view.elev !== undefined ? view.elev : 0.2;
            cam.position.set(Math.cos(a) * R * Math.cos(elev), show.centre + R * Math.sin(elev), Math.sin(a) * R * Math.cos(elev));
            cam.lookAt(0, show.centre, 0);
            cam.updateProjectionMatrix();
            show.rig.update(showCfg.pose ? Object.assign({}, SHOW_VEHICLE, showCfg.pose) : SHOW_VEHICLE, dt, null);
            renderer.toneMappingExposure = 1.0;
            renderer.info.reset();
            renderer.render(show.scene, cam);
        }

        /* ---- the interface ------------------------------------------------ */
        const projectVec = new T.Vector3();
        function projectX(p) {
            projectVec.set(p[0], p[1], p[2]).project(camera);
            return projectVec.x;
        }

        realPost = hasPost ? AD.createPost(T, renderer, scene, camera, glq) : null;
        const postApi = {
            get available() { return realPost ? !!realPost.available : false; },
            get enabled() { return realPost ? !!realPost.enabled : false; },
            setQuality(x) { if (realPost) realPost.setQuality(typeof x === 'string' ? (AD.QUALITY[x] || AD.QUALITY.high).gl : x); },
            setBiome(id) { if (realPost) realPost.setBiome(id); },
            resize(w, h, dpr) { if (realPost) realPost.resize(w, h, Math.min(dpr, glq.dpr)); },
            impulse(kind, amount) { if (realPost && realPost.impulse) realPost.impulse(kind, amount); },
            setHoles(list) { if (realPost && realPost.setHoles) realPost.setHoles(list); },
            render(r, dt, reduced, pose) { return realPost ? realPost.render(dt, r, pose, reduced) : false; },
            stats() { return realPost ? realPost.stats() : noopPost().stats(); },
            dispose() { /* owned by the renderer */ }
        };
        post = postApi;

        function onLost(event) {
            event.preventDefault();
            lost = true;
            if (api.onContextLost) api.onContextLost();
        }
        function onRestored() {
            lost = false;
            env = null; envKey = ''; showEnv = null; showEnvKey = '';
            if (realPost && realPost.restore) realPost.restore();
            if (show) { const keep = showCfg; disposeShow(); showCfg = keep; }
            if (track) buildTrack();
            if (api.onContextRestored) api.onContextRestored();
        }
        canvas.addEventListener('webglcontextlost', onLost, false);
        canvas.addEventListener('webglcontextrestored', onRestored, false);

        const api = {
            kind: 'three',
            post: postApi,
            lightPool: () => lightPool,
            scene: scene, camera: camera,
            resize(w, h, dpr) {
                size = { w: Math.max(320, Math.round(w)), h: Math.max(240, Math.round(h)), dpr: dpr || 1 };
                applySize();
            },
            setQuality(name) {
                if (!AD.QUALITY[name]) return;
                tier = name;
                glq = AD.QUALITY[name].gl;
                applySize();
                applySun();
                applyPool();
                setEnv(look, track ? track.biomeId : 'alpine');
                if (actors) actors.setQuality(glq);
                if (fx && fx.setQuality) fx.setQuality(glq);
                if (props && props.setQuality) props.setQuality(glq);
                if (show) show.rig && show.rig.setGl(glq);
                if (realPost) realPost.setQuality(glq);
            },
            setTrack(next, r) {
                track = next; race = r || race;
                buildTrack();
                applySun();
            },
            setShowcase(cfg) {
                showCfg = Object.assign({}, showCfg, cfg || {});
                if (show) {
                    const l = (show.look && show.look) || AD.LOOK.alpine;
                    if (AD.LOOK[showCfg.biomeId] !== l) { disposeShow(); return; }
                    refreshShowcase(true);
                }
            },
            showcase(dt, reduced) {
                if (lost || disposed) return;
                time += dt;
                drawShowcase(dt, reduced);
            },
            render(r, pose, dt, reduced) {
                if (lost) throw new Error('webgl context lost');
                race = r;
                time += dt;
                applyPose(pose);
                const v = r.cars[0].v;
                if (!track || track !== r.track) { track = r.track; buildTrack(); applySun(); }
                actors.update(r, dt);
                if (fallback) { fallback.dome.position.copy(camera.position); }
                if (sky && sky.update) sky.update(dt, pose, r.conditions ? r.conditions.state : null);
                if (props && props.update) props.update(pose, r);
                if (fx && fx.update) fx.update(dt, pose);
                const cond = r.conditions && r.conditions.state;
                const wetTarget = clamp(look.wet * 0.5 + (cond && cond.kind === 'rain' ? cond.intensity : 0), 0, 1);
                if (Math.abs(wetTarget - wetness) > 0.002) {
                    wetness += (wetTarget - wetness) * (1 - Math.exp(-0.8 * dt));
                    if (road && road.setWetness) road.setWetness(wetness);
                    if (terrain && terrain.setWetness) terrain.setWetness(wetness);
                }
                followSun(v.x, v.z, v.y);
                renderer.toneMappingExposure = look.exposure;
                renderer.info.reset();
                if (!(realPost && realPost.enabled && realPost.render(dt, r, pose, reduced))) renderer.render(scene, camera);
            },
            event(evt) {
                if (actors) actors.event(evt, race);
                if (fx && fx.event) fx.event(evt, race);
                if (evt.type === 'disco_on' || evt.type === 'disco_off') {
                    discoCount = Math.max(0, discoCount + (evt.type === 'disco_on' ? 1 : -1));
                    const skyApi = scene.userData.agdriftSky;
                    if (skyApi && skyApi.setDisco) skyApi.setDisco(discoCount > 0);
                }
                if (realPost && evt.type && realPost.impulse && evt.car === 0) {
                    if (evt.type === 'boost_pad' || evt.type === 'turbo_burst') realPost.impulse('boost', 1);
                    else if (evt.type === 'landing' && evt.quality === 'crash') realPost.impulse('impact', 1);
                    else if (evt.type === 'hit') realPost.impulse('impact', evt.heavy ? 1 : 0.5);
                }
            },
            emit(r, dt) { if (fx && fx.emit) fx.emit(r, dt); },
            stats() {
                const info = renderer.info;
                const pts = actors ? actors.carPoints(0) : null;
                let projection = null;
                if (pts) projection = { carX: projectX(pts.car), noseX: projectX(pts.nose), tailX: projectX(pts.tail) };
                const fxs = fx && fx.stats ? fx.stats() : null, ps = props && props.stats ? props.stats() : null;
                const ts = terrain && terrain.stats ? terrain.stats() : null;
                return {
                    kind: 'three', calls: info.render.calls, triangles: info.render.triangles,
                    geometries: info.memory.geometries, textures: info.memory.textures,
                    particles: fxs ? fxs.particles || 0 : 0, props: ps ? ps.props || 0 : 0,
                    standIns: AD.assets.stats().standIns + (ps && ps.standIns ? ps.standIns : 0),
                    wetness: wetness, chunks: ts ? ts.chunks || 0 : 0, projection: projection, tier: tier
                };
            },
            dispose() {
                if (disposed) return;
                disposed = true;
                canvas.removeEventListener('webglcontextlost', onLost, false);
                canvas.removeEventListener('webglcontextrestored', onRestored, false);
                disposeTrack();
                disposeShow();
                stop(realPost);
                if (env) env.dispose();
                if (showEnv) showEnv.dispose();
                for (const l of lightPool) l.removeFromParent();
                pmrem.dispose();
                if (sun.shadow.map) sun.shadow.map.dispose();
                renderer.dispose();
                renderer.forceContextLoss();
                if (held) { held = false; AD.assets.release(); }
            }
        };
        applySize();
        applySun();
        applyPool();
        applyLook(AD.LOOK.alpine, 'alpine');
        return api;
    };
})();
