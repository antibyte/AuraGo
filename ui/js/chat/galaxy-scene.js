(function () {
    'use strict';

    const root = document.documentElement;
    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    const MAX_PIXELS = 3840 * 2160;
    let runtime = null;
    let generation = 0;
    let failed = false;

    function asset(name) {
        return '/img/galaxy/' + name + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
    }

    const surfaceVertex = `
        varying vec2 vUv;
        varying vec3 vNormal;
        varying vec3 vWorld;
        void main() {
            vUv = uv;
            vNormal = normalize(mat3(modelMatrix) * normal);
            vec4 world = modelMatrix * vec4(position, 1.0);
            vWorld = world.xyz;
            gl_Position = projectionMatrix * viewMatrix * world;
        }
    `;
    const surfaceFragment = `
        precision highp float;
        uniform sampler2D uDay;
        uniform sampler2D uNight;
        uniform sampler2D uCloud;
        uniform vec3 uSun;
        uniform float uCloudOffset;
        uniform float uCloudShadow;
        varying vec2 vUv;
        varying vec3 vNormal;
        varying vec3 vWorld;
        void main() {
            vec3 n = normalize(vNormal);
            vec3 view = normalize(cameraPosition - vWorld);
            float sun = dot(n, uSun);
            vec3 albedo = texture2D(uDay, vUv).rgb;
            vec3 linear = pow(albedo, vec3(2.2));
            float clouds = texture2D(uCloud, vUv + vec2(uCloudOffset + 0.003, 0.002)).r;
            float diffuse = max(0.0, sun) * (1.0 - clouds * uCloudShadow);
            vec3 col = linear * (0.025 + diffuse * 1.65);
            float sea = smoothstep(0.015, 0.08, albedo.b - albedo.r) * (1.0 - smoothstep(0.2, 0.45, albedo.r));
            float specular = pow(max(0.0, dot(reflect(-uSun, n), view)), 70.0);
            col += vec3(0.85, 0.92, 1.0) * specular * sea * max(0.0, sun) * 0.65;
            vec3 city = texture2D(uNight, vUv).rgb;
            float night = 1.0 - smoothstep(-0.18, 0.15, sun);
            col += pow(city, vec3(1.6)) * vec3(1.0, 0.63, 0.28) * night * 1.8 * (1.0 - clouds * 0.65);
            float rim = pow(1.0 - max(0.0, dot(n, view)), 3.8);
            col += vec3(0.05, 0.25, 0.68) * rim * smoothstep(-0.22, 0.55, sun) * 0.5;
            gl_FragColor = vec4(pow(max(col, vec3(0.0)), vec3(1.0 / 2.2)), 1.0);
        }
    `;
    const cloudFragment = `
        precision highp float;
        uniform sampler2D uCloud;
        uniform vec3 uSun;
        varying vec2 vUv;
        varying vec3 vNormal;
        void main() {
            float cloud = smoothstep(0.16, 0.85, texture2D(uCloud, vUv).r);
            float light = smoothstep(-0.15, 0.7, dot(normalize(vNormal), uSun));
            gl_FragColor = vec4(mix(vec3(0.10, 0.15, 0.23), vec3(0.94, 0.97, 1.0), light), cloud * 0.9);
        }
    `;
    const atmosphereFragment = `
        precision highp float;
        uniform vec3 uSun;
        varying vec3 vNormal;
        varying vec3 vWorld;
        void main() {
            vec3 n = normalize(vNormal);
            vec3 view = normalize(cameraPosition - vWorld);
            float rim = pow(1.0 - abs(dot(n, view)), 4.5);
            float lit = smoothstep(-0.4, 0.6, dot(n, uSun));
            gl_FragColor = vec4(vec3(0.12, 0.42, 1.0), rim * (0.025 + lit * 0.4));
        }
    `;

    function shader(fragmentShader, uniforms, options) {
        return new THREE.ShaderMaterial(Object.assign({ vertexShader: surfaceVertex, fragmentShader, uniforms }, options));
    }

    function loadTexture(rt, name) {
        return new Promise((resolve, reject) => {
            const texture = new THREE.TextureLoader().load(asset(name), () => {
                if (rt.closed) { texture.dispose(); resolve(null); return; }
                texture.wrapS = THREE.RepeatWrapping;
                texture.anisotropy = Math.min(4, rt.renderer.capabilities.getMaxAnisotropy());
                resolve(texture);
            }, undefined, reject);
            rt.textures.add(texture);
        });
    }

    function createScene(rt, maps) {
        const [space, day, night, cloud, detail] = maps;
        const sun = new THREE.Vector3(-0.75, 0.5, 0.55).normalize();
        const sphere = new THREE.SphereGeometry(1, rt.mobile ? 64 : 128, rt.mobile ? 32 : 64);
        const backdrop = new THREE.Mesh(new THREE.PlaneGeometry(2, 2), new THREE.ShaderMaterial({
            depthTest: false, depthWrite: false,
            uniforms: {
                uSpace: { value: space }, uDetail: { value: detail || space },
                uDetailWeight: { value: detail ? 0.16 : 0 },
                uAspect: { value: 1 }, uDrift: { value: new THREE.Vector2() }
            },
            vertexShader: 'varying vec2 vUv; void main(){vUv=uv;gl_Position=vec4(position.xy,0.999,1.0);}',
            fragmentShader: `
                precision highp float;
                uniform sampler2D uSpace;
                uniform sampler2D uDetail;
                uniform float uDetailWeight;
                uniform float uAspect;
                uniform vec2 uDrift;
                varying vec2 vUv;
                void main() {
                    vec2 uv = vUv - 0.5;
                    if (uAspect > 1.77778) uv.y *= 1.77778 / uAspect;
                    else uv.x *= uAspect / 1.77778;
                    uv = uv * 0.975 + 0.5 + uDrift;
                    vec3 col = texture2D(uSpace, uv).rgb;
                    // Resolved Hubble stars add fine detail to the authored galaxy layer.
                    float detail = dot(texture2D(uDetail, uv * vec2(1.3, 2.1)).rgb, vec3(0.333));
                    float galaxy = smoothstep(0.09, 0.5, max(col.r, max(col.g, col.b)));
                    col *= 1.0 + (detail - 0.35) * uDetailWeight * galaxy;
                    gl_FragColor = vec4(col, 1.0);
                }
            `
        }));
        backdrop.frustumCulled = false;
        backdrop.renderOrder = -10;
        rt.scene.add(backdrop);
        rt.backdrop = backdrop;

        rt.planet = new THREE.Group();
        const surface = new THREE.Mesh(sphere, shader(surfaceFragment, {
            uDay: { value: day }, uNight: { value: night }, uCloud: { value: cloud },
            uSun: { value: sun }, uCloudOffset: { value: 0 }, uCloudShadow: { value: rt.mobile ? 0 : 0.25 }
        }));
        rt.surface = surface;
        rt.planet.add(surface);
        rt.clouds = new THREE.Mesh(sphere, shader(cloudFragment, {
            uCloud: { value: cloud }, uSun: { value: sun }
        }, { transparent: true, depthWrite: false }));
        rt.clouds.scale.setScalar(1.009);
        rt.planet.add(rt.clouds);
        const atmosphere = new THREE.Mesh(sphere, shader(atmosphereFragment, { uSun: { value: sun } }, {
            side: THREE.BackSide, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
        }));
        atmosphere.scale.setScalar(rt.mobile ? 1.035 : 1.045);
        rt.planet.add(atmosphere);
        rt.planet.rotation.z = -0.19;
        rt.scene.add(rt.planet);

        // A distant, small world shares geometry; its bands are material detail.
        rt.distant = new THREE.Mesh(sphere, shader(`
            precision highp float;
            uniform vec3 uSun;
            uniform sampler2D uCloud;
            varying vec2 vUv; varying vec3 vNormal;
            void main(){
                float turbulence = texture2D(uCloud, vUv).r;
                float bands = sin(vUv.y * 85.0 + turbulence * 5.0) * 0.5 + 0.5;
                vec3 col = mix(vec3(0.30,0.25,0.28),vec3(0.46,0.35,0.32),bands);
                col += turbulence * 0.07;
                col *= 0.08 + max(0.0,dot(normalize(vNormal),uSun)) * 0.75;
                gl_FragColor=vec4(col,1.0);
            }
        `, { uSun: { value: sun }, uCloud: { value: cloud } }));
        rt.distant.scale.setScalar(0.075);
        rt.distant.rotation.z = 0.3;
        rt.scene.add(rt.distant);

        let seed = 7341;
        const random = () => ((seed = (Math.imul(seed, 1664525) + 1013904223) >>> 0) / 4294967296);
        const count = rt.mobile ? 850 : 3500;
        const positions = new Float32Array(count * 3);
        const colors = new Float32Array(count * 3);
        const sizes = new Float32Array(count);
        const twinkle = new Float32Array(count);
        for (let i = 0; i < count; i++) {
            positions.set([random() * 2 - 1, random() * 2 - 1, random()], i * 3);
            const warmth = random();
            colors.set([0.65 + warmth * 0.35, 0.75 + warmth * 0.2, 1.0 - warmth * 0.3], i * 3);
            sizes[i] = 0.5 + Math.pow(random(), 7) * 2.4;
        }
        // Spread twenty subtle scintillating stars across the viewport.
        for (let i = 0; i < 20; i++) {
            positions[i * 3] = -0.88 + (i % 5) * 0.43 + random() * 0.12;
            positions[i * 3 + 1] = -0.72 + Math.floor(i / 5) * 0.46 + random() * 0.12;
            sizes[i] = 2.1 + random() * 1.2;
            twinkle[i] = 1 + random() * 100;
        }
        const stars = new THREE.BufferGeometry();
        stars.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        stars.setAttribute('color', new THREE.BufferAttribute(colors, 3));
        stars.setAttribute('aSize', new THREE.BufferAttribute(sizes, 1));
        stars.setAttribute('aTwinkle', new THREE.BufferAttribute(twinkle, 1));
        rt.stars = new THREE.Points(stars, new THREE.ShaderMaterial({
            transparent: true, depthWrite: false, blending: THREE.AdditiveBlending,
            uniforms: { uDrift: { value: new THREE.Vector2() }, uPixelRatio: { value: 1 }, uTime: { value: 0 } },
            vertexShader: `
                attribute float aSize; attribute vec3 color; attribute float aTwinkle;
                uniform vec2 uDrift; uniform float uPixelRatio; uniform float uTime;
                varying vec3 vColor;
                void main(){
                    vColor=color;
                    if(aTwinkle>0.0){
                        float clock=uTime+aTwinkle;
                        float cycle=floor(clock/6.0);
                        float delay=fract(sin(cycle*12.9898+aTwinkle*78.233)*43758.5453)*4.0;
                        float pulse=clamp((mod(clock,6.0)-delay)/1.4,0.0,1.0);
                        float envelope=sin(pulse*3.14159265);
                        vColor*=1.0+envelope*(0.12+0.16*sin(clock*13.0+aTwinkle));
                    }
                    gl_Position=vec4(position.xy+uDrift*(0.3+position.z),0.98,1.0);
                    gl_PointSize=aSize*uPixelRatio;
                }
            `,
            fragmentShader: `
                precision mediump float; varying vec3 vColor;
                void main(){
                    float d=length(gl_PointCoord-0.5)*2.0;
                    gl_FragColor=vec4(vColor,(1.0-smoothstep(0.0,1.0,d))*0.6);
                }
            `
        }));
        rt.stars.frustumCulled = false;
        rt.stars.renderOrder = -5;
        rt.scene.add(rt.stars);

        // Faceted scout hull, swept wings and small dorsal engine strips.
        // All three distant ships share one buffer and one draw call.
        const hull = [
            [1.6, 0, 0], [-1, -0.18, 0], [-1, 0.18, 0],
            [-0.3, 0, 0.28], [-0.3, 0, -0.16],
            [-1.25, -0.8, -0.04], [-1.25, 0.8, -0.04],
            [0.2, -0.12, 0.03], [0.2, 0.12, 0.03]
        ];
        const shipPositions = [], shipColors = [];
        const face = (a, b, c, color) => {
            shipPositions.push(...a, ...b, ...c);
            for (let i = 0; i < 3; i++) shipColors.push(...color);
        };
        [[0, 1, 3], [0, 3, 2], [1, 2, 3], [0, 4, 1], [0, 2, 4], [1, 4, 2], [7, 5, 1], [8, 2, 6]].forEach((f, i) => {
            const shade = i % 2 ? 0.4 : 0.65;
            face(hull[f[0]], hull[f[1]], hull[f[2]], [shade * 0.7, shade * 0.85, shade]);
        });
        for (const side of [-1, 1]) {
            face([-1.14, side * 0.65, 0.01], [-0.88, side * 0.48, 0.02], [-1.02, side * 0.48, 0.02], [0.16, 0.65, 1]);
        }
        const shipGeometry = new THREE.BufferGeometry();
        shipGeometry.setAttribute('position', new THREE.Float32BufferAttribute(shipPositions, 3));
        shipGeometry.setAttribute('color', new THREE.Float32BufferAttribute(shipColors, 3));
        rt.ships = new THREE.InstancedMesh(shipGeometry, new THREE.MeshBasicMaterial({ vertexColors: true, side: THREE.DoubleSide }), 3);
        rt.ships.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
        rt.ships.frustumCulled = false;
        rt.shipTransform = new THREE.Object3D();
        rt.shipRoutes = [
            { duration: 85, phase: 0.25, y: 0.48, slope: -0.12, scale: 0.025, direction: 1 },
            { duration: 115, phase: 0.65, y: 0.1, slope: 0.18, scale: 0.018, direction: -1 },
            { duration: 145, phase: 0.02, y: -0.35, slope: -0.08, scale: 0.013, direction: 1 }
        ];
        rt.scene.add(rt.ships);
    }

    function resize() {
        const rt = runtime;
        if (!rt || rt.closed) return;
        const width = Math.max(1, window.innerWidth);
        const height = Math.max(1, window.innerHeight);
        const aspect = width / height;
        rt.camera.left = -aspect;
        rt.camera.right = aspect;
        rt.camera.updateProjectionMatrix();
        const density = Math.min(window.devicePixelRatio || 1, rt.mobile ? 1 : 1.5);
        const ratio = Math.min(density * rt.quality, Math.sqrt(MAX_PIXELS / (width * height)));
        rt.renderer.setPixelRatio(ratio);
        rt.renderer.setSize(width, height, false);
        if (!rt.ready) return;
        rt.planet.position.set(aspect * (aspect < 1 ? 0.74 : 0.78), -0.66, 1);
        rt.planet.scale.setScalar(aspect < 1 ? 0.48 : 0.73);
        rt.distant.position.set(-aspect * 0.73, -0.48, -2);
        rt.backdrop.material.uniforms.uAspect.value = aspect;
        rt.stars.material.uniforms.uPixelRatio.value = ratio;
    }

    function updateQuality(rt, delta) {
        rt.sampleTime += delta;
        rt.sampleFrames++;
        if (rt.sampleTime < 3) return;
        if (rt.sampleTime / rt.sampleFrames > Math.max(0.024, rt.frameBudget * 1.45) && rt.quality > 0.65) {
            rt.quality = Math.max(0.65, rt.quality * 0.85);
            resize();
        }
        rt.sampleTime = rt.sampleFrames = 0;
    }

    function draw(rt) {
        const angle = rt.time * Math.PI * 2 / 1200;
        rt.surface.rotation.y = 2.1 + angle;
        rt.clouds.rotation.y = 2.1 + angle * 1.06;
        rt.surface.material.uniforms.uCloudOffset.value = angle * 0.06 / (Math.PI * 2);
        rt.distant.rotation.y = rt.time * 0.002;
        const phase = rt.time * Math.PI * 2 / 180;
        rt.backdrop.material.uniforms.uDrift.value.set(Math.sin(phase) * 0.004, Math.sin(phase * 0.7) * 0.002);
        rt.stars.material.uniforms.uDrift.value.set(Math.sin(phase) * 0.009, Math.sin(phase * 0.7) * 0.004);
        rt.stars.material.uniforms.uTime.value = rt.time;
        rt.shipRoutes.forEach((route, i) => {
            const progress = (rt.time / route.duration + route.phase) % 1;
            const x = (progress * 2 - 1) * (rt.camera.right + 0.15) * route.direction;
            const ship = rt.shipTransform;
            ship.position.set(x, route.y + (progress - 0.5) * route.slope, -3 - i);
            ship.scale.setScalar(route.scale);
            ship.rotation.set(0.35, 0.2, Math.atan2(route.slope, 2 * (rt.camera.right + 0.15) * route.direction));
            ship.updateMatrix();
            rt.ships.setMatrixAt(i, ship.matrix);
        });
        rt.ships.instanceMatrix.needsUpdate = true;
        rt.camera.position.x = Math.sin(phase) * 0.012;
        rt.camera.position.y = Math.sin(phase * 0.7) * 0.006;
        rt.renderer.render(rt.scene, rt.camera);
    }

    function frame(time) {
        const rt = runtime;
        if (!rt || !rt.ready || rt.closed || rt.lost || document.hidden) return;
        rt.raf = 0;
        const delta = rt.lastFrame ? Math.max(0, (time - rt.lastFrame) / 1000) : 0;
        rt.lastFrame = time;
        // Measure the display cadence while the first complete frame fades in.
        // A 30/32 Hz remote display must not be mistaken for an overloaded GPU.
        if (rt.cadence) {
            if (delta > 0) rt.cadence.push(delta);
            if (rt.cadence.length >= 24) {
                rt.cadence.sort((a, b) => a - b);
                rt.frameBudget = Math.max(1 / 60, rt.cadence[4]);
                rt.cadence = null;
            }
            rt.raf = requestAnimationFrame(frame);
            return;
        }
        rt.time += Math.min(delta, 0.05);
        if (delta > 0 && delta < 1) updateQuality(rt, delta);
        draw(rt);
        rt.raf = requestAnimationFrame(frame);
    }

    function pause() {
        if (!runtime) return;
        cancelAnimationFrame(runtime.raf);
        runtime.raf = runtime.lastFrame = 0;
        runtime.sampleTime = runtime.sampleFrames = 0;
    }

    function stop() {
        generation++;
        const rt = runtime;
        if (!rt) return;
        pause();
        runtime = null;
        rt.closed = true;
        rt.canvas.removeEventListener('webglcontextlost', rt.onLost);
        rt.canvas.removeEventListener('webglcontextrestored', rt.onRestored);
        const geometries = new Set();
        rt.scene.traverse(node => {
            if (node.geometry) geometries.add(node.geometry);
            if (node.material) node.material.dispose();
        });
        geometries.forEach(geometry => geometry.dispose());
        rt.textures.forEach(texture => texture.dispose());
        rt.renderer.dispose();
        rt.renderer.forceContextLoss();
        rt.canvas.remove();
    }

    function start() {
        if (runtime || failed || !window.THREE || root.dataset.theme !== 'galaxy' || motion.matches || document.hidden) return;
        const canvas = document.createElement('canvas');
        canvas.id = 'galaxy-scene';
        canvas.setAttribute('aria-hidden', 'true');
        let renderer;
        try {
            renderer = new THREE.WebGLRenderer({ canvas, alpha: false, antialias: true, powerPreference: 'high-performance' });
        } catch (_) { failed = true; return; }
        renderer.outputEncoding = THREE.sRGBEncoding;
        const rt = {
            canvas, renderer, scene: new THREE.Scene(), camera: new THREE.OrthographicCamera(-1, 1, 1, -1, 0.1, 100),
            textures: new Set(), mobile: window.innerWidth < 768, quality: 1, ready: false, closed: false,
            time: 0, lastFrame: 0, raf: 0, cadence: [], frameBudget: 1 / 60,
            sampleTime: 0, sampleFrames: 0, generation: ++generation
        };
        runtime = rt;
        rt.camera.position.z = 10;
        rt.onLost = event => { event.preventDefault(); pause(); rt.canvas.classList.remove('is-ready'); rt.lost = true; };
        rt.onRestored = () => { if (runtime === rt) { stop(); failed = false; sync(); } };
        canvas.addEventListener('webglcontextlost', rt.onLost);
        canvas.addEventListener('webglcontextrestored', rt.onRestored);
        document.body.prepend(canvas);
        resize();
        const size = rt.mobile ? '2k' : '4k';
        const names = ['space-' + size + '.webp', 'earth-day-' + size + '.jpg', 'earth-night.png', 'earth-clouds.jpg'];
        if (!rt.mobile) names.push('galaxy-detail.jpg');
        Promise.all(names.map(name => loadTexture(rt, name))).then(maps => {
            if (runtime !== rt || rt.closed || rt.lost || generation !== rt.generation) return;
            createScene(rt, maps);
            rt.ready = true;
            resize();
            renderer.compile(rt.scene, rt.camera);
            draw(rt);
            canvas.classList.add('is-ready');
            sync();
        }).catch(() => {
            if (runtime !== rt) return;
            stop();
            failed = true;
        });
    }

    function sync() {
        if (root.dataset.theme !== 'galaxy' || motion.matches) { stop(); failed = false; return; }
        if (document.hidden) { pause(); return; }
        if (!runtime) { start(); return; }
        if (runtime.ready && !runtime.lost && !runtime.raf) runtime.raf = requestAnimationFrame(frame);
    }

    function updatePoster() {
        const size = window.innerWidth < 768 ? 'mobile' : window.innerWidth >= 1600 ? '4k' : '2k';
        root.style.setProperty('--galaxy-poster', 'url("' + asset('poster-' + size + '.webp') + '")');
    }

    window.AuraGoGalaxy = { start, stop, sync };
    const font = new FontFace('Galaxy Console', 'url("/fonts/BarlowCondensed-SemiBold.ttf?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev') + '")', { weight: '600', display: 'swap' });
    document.fonts.add(font);
    font.load().catch(() => { });
    updatePoster();
    new MutationObserver(sync).observe(root, { attributes: true, attributeFilter: ['data-theme'] });
    window.addEventListener('aurago:themechange', sync);
    window.addEventListener('resize', () => { updatePoster(); resize(); });
    window.addEventListener('pagehide', stop);
    window.addEventListener('pageshow', sync);
    document.addEventListener('visibilitychange', sync);
    if (motion.addEventListener) motion.addEventListener('change', sync);
    else if (motion.addListener) motion.addListener(sync);
    sync();
})();
