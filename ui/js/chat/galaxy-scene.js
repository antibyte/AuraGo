(function () {
    'use strict';

    const root = document.documentElement;
    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    const MAX_PIXELS = 3840 * 2160;
    const pointerGoal = { x: 0, y: 0, mx: 50, my: 18 };
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
        uniform float uTime;
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
            vec3 col = linear * vec3(0.48, 0.68, 1.0) * (0.009 + diffuse * 1.65);
            float sea = smoothstep(0.015, 0.08, albedo.b - albedo.r) * (1.0 - smoothstep(0.2, 0.45, albedo.r));
            float seaRipple = 0.88 + 0.12 * sin(uTime * 1.8 + vUv.x * 95.0 - vUv.y * 65.0);
            float specular = pow(max(0.0, dot(reflect(-uSun, n), view)), 70.0) * seaRipple;
            col += vec3(0.85, 0.92, 1.0) * specular * sea * max(0.0, sun) * 0.68;
            vec3 city = texture2D(uNight, vUv).rgb;
            float night = 1.0 - smoothstep(-0.18, 0.15, sun);
            // The night map includes a blue terrain base; only its bright pixels emit gold light.
            float cityLight = smoothstep(0.10, 0.32, dot(city, vec3(0.3, 0.6, 0.1)));
            float cityShimmer = 0.9 + 0.1 * sin(uTime * 2.4 + vUv.x * 180.0 + vUv.y * 130.0);
            col += pow(city, vec3(1.6)) * vec3(1.0, 0.63, 0.28) * cityLight * night * 1.85 * cityShimmer * (1.0 - clouds * 0.65);
            col += pow(city, vec3(2.2)) * vec3(0.1, 0.3, 0.75) * night * 0.3;
            // Subtle high-altitude storm lightning on the night-side cloud deck.
            float stormCycle = floor(uTime / 4.4);
            float stormPhase = mod(uTime, 4.4);
            float stormFlash = exp(-pow((stormPhase - 0.08) / 0.035, 2.0)) + 0.65 * exp(-pow((stormPhase - 0.19) / 0.045, 2.0));
            vec2 stormPos = vec2(fract(sin(stormCycle * 12.9898) * 43758.5453), 0.26 + 0.48 * fract(sin(stormCycle * 78.233) * 43758.5453));
            vec2 dStorm = vec2(min(abs(vUv.x - stormPos.x), 1.0 - abs(vUv.x - stormPos.x)) * 2.0, vUv.y - stormPos.y);
            float stormSpot = exp(-dot(dStorm, dStorm) / 0.0024) * smoothstep(0.24, 0.68, clouds) * night;
            col += vec3(0.56, 0.78, 1.0) * stormSpot * stormFlash * 0.78;
            float rim = pow(1.0 - max(0.0, dot(n, view)), 3.8);
            col += vec3(0.08, 0.4, 1.0) * rim * 1.6;
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
            gl_FragColor = vec4(mix(vec3(0.035, 0.07, 0.14), vec3(0.94, 0.97, 1.0), light), cloud * mix(0.22, 0.9, light));
        }
    `;
    const atmosphereFragment = `
        precision highp float;
        uniform vec3 uSun;
        uniform float uTime;
        varying vec3 vNormal;
        varying vec3 vWorld;
        void main() {
            vec3 n = normalize(vNormal);
            vec3 view = normalize(cameraPosition - vWorld);
            float rim = pow(1.0 - abs(dot(n, view)), 4.5);
            float lit = smoothstep(-0.4, 0.6, dot(n, uSun));
            float aurora = (sin(vWorld.x * 6.0 + vWorld.y * 4.5 + uTime * 0.55) * 0.5 + 0.5) * (1.0 - lit);
            vec3 col = mix(vec3(0.12, 0.42, 1.0), vec3(0.48, 0.36, 1.0), aurora * 0.38);
            gl_FragColor = vec4(col, rim * (0.23 + lit * 0.6 + aurora * 0.07));
        }
    `;
    // The Earth's moon borrows the cloud map as noise: broad dark maria and finer crater rims.
    const moonFragment = `
        precision highp float;
        uniform vec3 uSun;
        uniform sampler2D uCloud;
        varying vec2 vUv;
        varying vec3 vNormal;
        varying vec3 vWorld;
        void main() {
            float n1 = texture2D(uCloud, vUv * vec2(2.0, 1.0)).r;
            // The map repeats horizontally only; fold the finer sample vertically so it never clamps to the pole row.
            vec2 rimUv = vec2(vUv.x * 5.0 + 0.37, 1.0 - abs(mod(vUv.y * 2.5 + 0.37, 2.0) - 1.0));
            float n2 = texture2D(uCloud, rimUv).r;
            float maria = smoothstep(0.45, 0.75, n1);
            float rims = smoothstep(0.55, 0.9, n2) * 0.3;
            vec3 col = mix(vec3(0.56, 0.55, 0.53), vec3(0.33, 0.33, 0.35), maria) + rims;
            vec3 N = normalize(vNormal);
            // The shared sun sits behind the Earth; tilt the light towards the camera so the moon reads as gibbous, not a black disc.
            vec3 L = normalize(uSun + vec3(-0.4, 0.3, 1.7));
            float lit = max(0.0, dot(N, L));
            col *= 0.09 + lit * 0.95;
            vec3 V = normalize(cameraPosition - vWorld);
            col += vec3(0.45, 0.58, 0.9) * pow(1.0 - max(0.0, dot(N, V)), 4.0) * 0.14;
            gl_FragColor = vec4(col, 1.0);
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
        const sun = new THREE.Vector3(-0.35, 0.3, -1.0).normalize();
        const sphere = new THREE.SphereGeometry(1, rt.mobile ? 64 : 128, rt.mobile ? 32 : 64);
        const backdrop = new THREE.Mesh(new THREE.PlaneGeometry(2, 2), new THREE.ShaderMaterial({
            depthTest: false, depthWrite: false,
            uniforms: {
                uSpace: { value: space }, uDetail: { value: detail || space },
                uDetailWeight: { value: detail ? 0.04 : 0 },
                uAspect: { value: 1 }, uDrift: { value: new THREE.Vector2() },
                uTime: { value: 0 }
            },
            vertexShader: 'varying vec2 vUv; void main(){vUv=uv;gl_Position=vec4(position.xy,0.999,1.0);}',
            fragmentShader: `
                precision highp float;
                uniform sampler2D uSpace;
                uniform sampler2D uDetail;
                uniform float uDetailWeight;
                uniform float uAspect;
                uniform vec2 uDrift;
                uniform float uTime;
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
                    // Subtle cosmic dust shimmer along the nebula band.
                    float nebulaWave = 0.5 + 0.5 * sin(uv.x * 7.5 - uv.y * 5.0 + uTime * 0.22);
                    col += vec3(0.018, 0.036, 0.068) * galaxy * nebulaWave * (0.35 + detail * 0.65);
                    // Staggered procedural micro-meteor streaks across deep space.
                    for (int i = 0; i < 2; i++) {
                        float fi = float(i);
                        float period = 19.0 + fi * 13.0;
                        float t = uTime + fi * 11.3;
                        float cycle = floor(t / period);
                        float phase = mod(t, period);
                        if (phase < 1.35) {
                            float prog = phase / 1.35;
                            float h1 = fract(sin(cycle * 37.17 + fi * 91.3) * 43758.5453);
                            float h2 = fract(sin(cycle * 73.41 + fi * 53.7) * 24634.6345);
                            vec2 startPos = vec2(0.16 + h1 * 0.68, 0.56 + h2 * 0.30);
                            vec2 dir = normalize(vec2(h1 > 0.5 ? -0.84 : 0.84, -0.54));
                            vec2 head = startPos + dir * (prog * 0.36);
                            vec2 rel = vUv - head;
                            float along = dot(rel, -dir);
                            float perp = length(rel - (-dir) * along);
                            float tailLen = 0.085 * sin(prog * 3.14159265);
                            float trail = smoothstep(tailLen, 0.0, along) * step(0.0, along);
                            float core = exp(-perp * 720.0) * trail;
                            float halo = exp(-perp * 190.0) * trail * 0.38;
                            float env = sin(prog * 3.14159265);
                            col += mix(vec3(0.45, 0.82, 1.0), vec3(0.95, 0.98, 1.0), along / max(0.001, tailLen)) * (core + halo) * env;
                        }
                    }
                    // A slow comet with an ion tail crosses high above the nebula about every two and a half minutes.
                    {
                        float period = 150.0;
                        float t = uTime + 37.0;
                        float cycle = floor(t / period);
                        float phase = mod(t, period);
                        if (phase < 14.0) {
                            float prog = phase / 14.0;
                            float h = fract(sin(cycle * 41.7) * 43758.5453);
                            vec2 startPos = vec2(1.12, 0.52 + h * 0.22);
                            vec2 dir = normalize(vec2(-1.0, -0.22 - h * 0.18));
                            vec2 head = startPos + dir * prog * 1.35;
                            vec2 rel = vUv - head;
                            float along = dot(rel, -dir);
                            float perp = length(rel - (-dir) * along);
                            float trail = smoothstep(0.3, 0.0, along) * step(0.0, along);
                            float width = 0.004 + max(along, 0.0) * 0.07;
                            float ion = exp(-perp * perp / (width * width)) * trail;
                            float headGlow = exp(-dot(rel, rel) * 12000.0) * 1.2 + exp(-dot(rel, rel) * 1200.0) * 0.35;
                            float env = smoothstep(0.0, 0.12, prog) * (1.0 - smoothstep(0.88, 1.0, prog));
                            col += (vec3(0.55, 0.8, 1.0) * ion * 0.42 + vec3(0.95, 0.97, 1.0) * headGlow) * env;
                        }
                    }
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
            uSun: { value: sun }, uCloudOffset: { value: 0 }, uCloudShadow: { value: rt.mobile ? 0 : 0.25 },
            uTime: { value: 0 }
        }));
        rt.surface = surface;
        rt.planet.add(surface);
        rt.clouds = new THREE.Mesh(sphere, shader(cloudFragment, {
            uCloud: { value: cloud }, uSun: { value: sun }
        }, { transparent: true, depthWrite: false }));
        rt.clouds.scale.setScalar(1.009);
        rt.planet.add(rt.clouds);
        const atmosphere = new THREE.Mesh(sphere, shader(atmosphereFragment, { uSun: { value: sun }, uTime: { value: 0 } }, {
            side: THREE.BackSide, transparent: true, depthWrite: false, blending: THREE.AdditiveBlending
        }));
        atmosphere.scale.setScalar(rt.mobile ? 1.035 : 1.045);
        rt.atmosphere = atmosphere;
        rt.planet.add(atmosphere);
        rt.planet.rotation.z = -0.19;
        rt.scene.add(rt.planet);

        // A distant, small world shares geometry; its bands are material detail.
        rt.distant = new THREE.Mesh(sphere, shader(`
            precision highp float;
            uniform vec3 uSun;
            uniform sampler2D uCloud;
            uniform float uTime;
            varying vec2 vUv; varying vec3 vNormal; varying vec3 vWorld;
            void main(){
                vec2 bandUv = vec2(fract(vUv.x + uTime * 0.0008), vUv.y);
                float turbulence = texture2D(uCloud, bandUv).r;
                float bands = sin(vUv.y * 85.0 + turbulence * 5.0 + sin(vUv.x * 12.0 + uTime * 0.05) * 0.35) * 0.5 + 0.5;
                vec3 col = mix(vec3(0.30,0.25,0.28),vec3(0.48,0.36,0.33),bands);
                col += turbulence * 0.07;
                vec3 N = normalize(vNormal);
                vec3 L = normalize(uSun + vec3(-0.4,0.3,1.7));
                float lit = max(0.0, dot(N, L));
                col *= 0.24 + lit * 0.75;
                vec3 V = normalize(cameraPosition - vWorld);
                float rim = pow(1.0 - max(0.0, dot(N, V)), 3.0) * smoothstep(-0.2, 0.6, dot(N, L));
                col += vec3(0.62, 0.48, 0.42) * rim * 0.28;
                gl_FragColor=vec4(col,1.0);
            }
        `, { uSun: { value: sun }, uCloud: { value: cloud }, uTime: { value: 0 } }));
        rt.distant.scale.setScalar(0.095);
        rt.distant.rotation.z = 0.3;
        rt.scene.add(rt.distant);

        // The Earth's moon: opaque, so the depth test hides it behind the planet on the far side of its orbit.
        rt.moon = new THREE.Mesh(new THREE.SphereGeometry(1, 40, 20), shader(moonFragment, { uSun: { value: sun }, uCloud: { value: cloud } }));
        // Its orbit leaves the viewport for part of the cycle; never cull it, so the draw count stays constant.
        rt.moon.frustumCulled = false;
        rt.scene.add(rt.moon);

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
                varying vec3 vColor; varying float vSpark;
                void main(){
                    vColor=color;
                    vSpark=0.0;
                    if(aTwinkle>0.0){
                        float clock=uTime+aTwinkle;
                        float cycle=floor(clock/6.0);
                        float delay=fract(sin(cycle*12.9898+aTwinkle*78.233)*43758.5453)*4.0;
                        float pulse=clamp((mod(clock,6.0)-delay)/1.4,0.0,1.0);
                        float envelope=sin(pulse*3.14159265);
                        vColor*=1.0+envelope*(0.12+0.16*sin(clock*13.0+aTwinkle));
                        vSpark=envelope;
                    }
                    gl_Position=vec4(position.xy+uDrift*(0.3+position.z),0.98,1.0);
                    // A pulsing star swells so its diffraction cross has room to flare.
                    gl_PointSize=aSize*uPixelRatio*(1.0+vSpark*2.2);
                }
            `,
            fragmentShader: `
                precision mediump float; varying vec3 vColor; varying float vSpark;
                void main(){
                    vec2 pc=gl_PointCoord-0.5;
                    float d=length(pc)*2.0;
                    float core=1.0-smoothstep(0.0,1.0,d);
                    // Two thin crossed spikes, strongest at the peak of the pulse.
                    float flare=(exp(-abs(pc.x)*9.0)*exp(-abs(pc.y)*40.0)+exp(-abs(pc.y)*9.0)*exp(-abs(pc.x)*40.0))*vSpark;
                    gl_FragColor=vec4(vColor,min(1.0,(core+flare*0.9)*0.6));
                }
            `
        }));
        rt.stars.frustumCulled = false;
        rt.stars.renderOrder = -5;
        rt.scene.add(rt.stars);

        // Three tiny satellites cross the sky on slow clip-space routes with blinking beacons.
        // Each route is start, direction (spanning past the opposite edge), period, phase and blink rate;
        // the vertex shader derives every position from the clock alone.
        const satRoutes = [
            [-1.1, 0.35, 2.2, 0.25, 75, 0.1, 2.4],
            [1.1, 0.75, -2.2, -0.5, 96, 0.55, 1.9],
            [-0.5, 1.1, 0.9, -2.2, 64, 0.8, 2.9]
        ];
        const satStart = new Float32Array(satRoutes.length * 2);
        const satDir = new Float32Array(satRoutes.length * 2);
        const satPeriod = new Float32Array(satRoutes.length);
        const satPhase = new Float32Array(satRoutes.length);
        const satBlink = new Float32Array(satRoutes.length);
        satRoutes.forEach((route, i) => {
            satStart.set([route[0], route[1]], i * 2);
            satDir.set([route[2], route[3]], i * 2);
            satPeriod[i] = route[4];
            satPhase[i] = route[5];
            satBlink[i] = route[6];
        });
        const satellites = new THREE.BufferGeometry();
        // Three sizes the draw call from the position attribute, so keep an unused one.
        satellites.setAttribute('position', new THREE.BufferAttribute(new Float32Array(satRoutes.length * 3), 3));
        satellites.setAttribute('aStart', new THREE.BufferAttribute(satStart, 2));
        satellites.setAttribute('aDir', new THREE.BufferAttribute(satDir, 2));
        satellites.setAttribute('aPeriod', new THREE.BufferAttribute(satPeriod, 1));
        satellites.setAttribute('aPhase', new THREE.BufferAttribute(satPhase, 1));
        satellites.setAttribute('aBlink', new THREE.BufferAttribute(satBlink, 1));
        rt.satellites = new THREE.Points(satellites, new THREE.ShaderMaterial({
            transparent: true, depthWrite: false, blending: THREE.AdditiveBlending,
            uniforms: { uTime: { value: 0 }, uPixelRatio: { value: 1 }, uDrift: { value: new THREE.Vector2() } },
            vertexShader: `
                attribute vec2 aStart; attribute vec2 aDir; attribute float aPeriod; attribute float aPhase; attribute float aBlink;
                uniform float uTime; uniform float uPixelRatio; uniform vec2 uDrift;
                varying float vBlink; varying float vFade;
                void main(){
                    float t=fract(uTime/aPeriod+aPhase);
                    vFade=smoothstep(0.0,0.08,t)*(1.0-smoothstep(0.92,1.0,t));
                    vBlink=pow(0.5+0.5*sin(uTime*aBlink+aPhase*40.0),18.0);
                    gl_Position=vec4(aStart+aDir*t+uDrift*0.6,0.97,1.0);
                    gl_PointSize=(2.2+vBlink*2.6)*uPixelRatio;
                }
            `,
            fragmentShader: `
                precision mediump float; varying float vBlink; varying float vFade;
                void main(){
                    float d=length(gl_PointCoord-0.5)*2.0;
                    float core=1.0-smoothstep(0.0,1.0,d);
                    vec3 col=mix(vec3(0.85,0.9,1.0),vec3(1.0,0.45,0.35),vBlink);
                    gl_FragColor=vec4(col,core*(0.55+vBlink*0.45)*vFade);
                }
            `
        }));
        rt.satellites.frustumCulled = false;
        rt.satellites.renderOrder = -4;
        rt.scene.add(rt.satellites);

        createFleet(rt, sun);
    }

    function createFleet(rt, sun) {
        // Bake each multipart design into one mesh: four ships, four draw calls.
        const material = new THREE.MeshStandardMaterial({ vertexColors: true, flatShading: true, metalness: 0.35, roughness: 0.48 });
        rt.fleetTime = { value: 0 };
        material.onBeforeCompile = shader => {
            shader.uniforms.uTime = rt.fleetTime;
            shader.vertexShader = 'varying vec3 vShipLocal;\n' + shader.vertexShader.replace(
                '#include <begin_vertex>',
                '#include <begin_vertex>\nvShipLocal = position;'
            );
            shader.fragmentShader = 'uniform float uTime;\nvarying vec3 vShipLocal;\n' + shader.fragmentShader.replace(
                '#include <emissivemap_fragment>',
                `#include <emissivemap_fragment>
                float cyanGlow = smoothstep(0.14, 0.38, vColor.b - vColor.r);
                float warmGlow = smoothstep(0.08, 0.28, vColor.r - vColor.b) * step(0.35, vColor.g);
                float thrusterPulse = 0.78 + 0.28 * sin(uTime * 9.5 + vShipLocal.x * 8.0 + vShipLocal.y * 5.0);
                float navStrobe = pow(0.5 + 0.5 * sin(uTime * 3.8 + abs(vShipLocal.y) * 6.0), 10.0) * smoothstep(0.32, 0.55, abs(vShipLocal.y));
                totalEmissiveRadiance += vColor.rgb * cyanGlow * (0.65 * thrusterPulse + navStrobe * 0.85) + vColor.rgb * warmGlow * 0.28;`
            );
        };
        const light = new THREE.DirectionalLight(0xffeedb, 1.8);
        light.position.copy(sun).multiplyScalar(10);
        rt.scene.add(light, new THREE.HemisphereLight(0xaacfff, 0x152031, 0.8));
        rt.ships = [];
        for (let design = 0; design < 4; design++) {
            const positions = [], normals = [], colors = [];
            const part = (geometry, hex, x, y, z, rx = 0, rz = 0) => {
                const flat = geometry.index ? geometry.toNonIndexed() : geometry;
                flat.rotateX(rx).rotateZ(rz).translate(x, y, z);
                const color = new THREE.Color(hex).convertSRGBToLinear();
                positions.push(...flat.attributes.position.array);
                normals.push(...flat.attributes.normal.array);
                for (let i = 0; i < flat.attributes.position.count; i++) colors.push(color.r, color.g, color.b);
                flat.dispose();
                if (flat !== geometry) geometry.dispose();
            };
            const box = (x, y, z, w, h, d, color) => part(new THREE.BoxGeometry(w, h, d), color, x, y, z);
            const pod = (x, y, z, length, radius, color) => {
                part(new THREE.CylinderGeometry(radius * 0.8, radius, length, 10), color, x, y, z, 0, -Math.PI / 2);
                part(new THREE.CylinderGeometry(radius * 0.75, radius * 0.75, 0.035, 10), 0x82e4ff, x - length / 2 - 0.015, y, z, 0, -Math.PI / 2);
                part(new THREE.ConeGeometry(radius * 0.68, radius * 3.2, 10), 0x5ce1ff, x - length / 2 - radius * 1.62, y, z, 0, Math.PI / 2);
            };
            if (design === 0) {
                // Survey cruiser: broad saucer, bridge dome and twin nacelles.
                box(-0.5, 0, 0, 1.2, 0.18, 0.12, 0x8297ab);
                box(-0.65, 0, -0.02, 0.24, 1.35, 0.1, 0x687d91);
                part(new THREE.SphereGeometry(1, 24, 8).scale(0.8, 0.72, 0.12), 0xcbd7df, 0.55, 0, 0.06);
                part(new THREE.SphereGeometry(1, 12, 6).scale(0.23, 0.22, 0.1), 0x819bb0, 0.5, 0, 0.18);
                box(0.65, 0, 0.24, 0.13, 0.21, 0.035, 0x9be2f2);
                for (const side of [-1, 1]) {
                    pod(-0.62, side * 0.67, 0.06, 1.65, 0.1, 0xa9bdca);
                    box(-0.62, side * 0.67, 0.15, 1.05, 0.05, 0.025, 0x68bde6);
                    box(0.67, side * 0.4, 0.17, 0.26, 0.035, 0.02, 0x53687c);
                }
            } else if (design === 1) {
                // Freighter: exposed spine, six cargo pods and a raised bridge.
                box(0, 0, 0, 2.15, 0.34, 0.22, 0x697e91);
                for (let i = 0; i < 3; i++) for (const side of [-1, 1]) {
                    box(-0.65 + i * 0.58, side * 0.35, 0.04, 0.49, 0.38, 0.32, i === 1 ? 0xc69a68 : 0x7798ab);
                    box(-0.65 + i * 0.58, side * 0.35, 0.21, 0.055, 0.37, 0.035, 0xd5d8d1);
                }
                box(0.94, 0, 0.13, 0.36, 0.52, 0.35, 0xc3c7bf);
                box(1.05, 0, 0.32, 0.13, 0.36, 0.035, 0x8bd7e9);
                for (const side of [-1, 1]) pod(-1.04, side * 0.31, -0.07, 0.5, 0.16, 0x8a9aa9);
            } else if (design === 2) {
                // Ring explorer: open annular drive and narrow instrument hull.
                part(new THREE.TorusGeometry(0.72, 0.085, 6, 28), 0xc3bdab, -0.25, 0, 0);
                box(-0.25, 0, 0, 0.12, 1.4, 0.08, 0x617d92);
                pod(0, 0, 0.05, 2.15, 0.15, 0xa5bcc9);
                box(0.65, 0, 0.2, 0.3, 0.24, 0.13, 0xd4dce0);
                box(0.73, 0, 0.28, 0.12, 0.18, 0.035, 0x8edbea);
                for (const side of [-1, 1]) {
                    box(-0.25, side * 0.7, 0.07, 0.25, 0.075, 0.04, 0x88d7f4);
                    pod(-0.92, side * 0.28, 0, 0.45, 0.09, 0x627e92);
                }
            } else {
                // Shuttle: rounded fuselage, dark canopy and straight outriggers.
                part(new THREE.SphereGeometry(1, 12, 6).scale(0.98, 0.36, 0.24), 0xc2cdd4, 0.1, 0, 0.02);
                part(new THREE.SphereGeometry(1, 10, 6).scale(0.38, 0.27, 0.13), 0x285977, 0.55, 0, 0.21);
                box(-0.28, 0, 0, 0.54, 1.3, 0.08, 0x8d9dab);
                box(-0.15, 0, 0.26, 0.44, 0.12, 0.025, 0xd8a46f);
                for (const side of [-1, 1]) {
                    pod(-0.48, side * 0.52, 0, 1.05, 0.12, 0xaabac6);
                    box(-0.65, side * 0.52, 0.18, 0.34, 0.05, 0.25, 0x72899d);
                }
            }
            const geometry = new THREE.BufferGeometry();
            geometry.setAttribute('position', new THREE.Float32BufferAttribute(positions, 3));
            geometry.setAttribute('normal', new THREE.Float32BufferAttribute(normals, 3));
            geometry.setAttribute('color', new THREE.Float32BufferAttribute(colors, 3));
            const ship = new THREE.Mesh(geometry, material);
            ship.frustumCulled = false;
            rt.ships.push(ship);
            rt.scene.add(ship);
        }
        rt.shipRoutes = [
            { duration: 84, phase: 0.28, x: 0, y: 0.28, angle: 28, scale: 0.055 },
            { duration: 108, phase: 0.54, x: 0.1, y: 0.4, angle: 145, scale: 0.045 },
            { duration: 122, phase: 0.34, x: -0.3, y: 0, angle: -58, scale: 0.04 },
            { duration: 74, phase: 0.46, x: 0.3, y: 0.3, angle: -32, scale: 0.035 }
        ];
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
        rt.planet.position.set(aspect * (aspect < 1 ? 0.88 : 1.04), -0.77, 1);
        rt.planet.scale.setScalar(aspect < 1 ? 0.64 : 1.08);
        rt.moon.scale.setScalar(rt.planet.scale.x * 0.06);
        rt.distant.position.set(-aspect * 0.78, -0.18, -2);
        rt.backdrop.material.uniforms.uAspect.value = aspect;
        rt.stars.material.uniforms.uPixelRatio.value = ratio;
        rt.satellites.material.uniforms.uPixelRatio.value = ratio;
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
        const px = rt.px || 0;
        const py = rt.py || 0;
        rt.surface.rotation.y = 3.9 + angle;
        rt.clouds.rotation.y = 3.9 + angle * 1.06;
        rt.surface.material.uniforms.uCloudOffset.value = angle * 0.06 / (Math.PI * 2);
        rt.surface.material.uniforms.uTime.value = rt.time;
        if (rt.atmosphere) rt.atmosphere.material.uniforms.uTime.value = rt.time;
        rt.distant.rotation.y = rt.time * 0.0025;
        rt.distant.material.uniforms.uTime.value = rt.time;
        // The moon circles the Earth on a tilted, slow orbit. Its depth swings through the planet's own
        // (scaled with it), so it passes in front of the disc and then behind it, where the depth test hides it.
        const orbit = rt.time * Math.PI * 2 / 420 + 1.9;
        const moonReach = rt.planet.scale.x * 1.42;
        rt.moon.position.set(
            rt.planet.position.x + Math.cos(orbit) * moonReach,
            rt.planet.position.y + 0.12 + Math.sin(orbit) * moonReach * 0.34,
            rt.planet.position.z + Math.sin(orbit) * rt.planet.scale.x * 1.3
        );
        rt.moon.rotation.y = rt.time * 0.02;
        const phase = rt.time * Math.PI * 2 / 180;
        rt.backdrop.material.uniforms.uTime.value = rt.time;
        rt.backdrop.material.uniforms.uDrift.value.set(Math.sin(phase) * 0.004 + px * 0.0035, Math.sin(phase * 0.7) * 0.002 + py * 0.0025);
        rt.stars.material.uniforms.uDrift.value.set(Math.sin(phase) * 0.009 + px * 0.008, Math.sin(phase * 0.7) * 0.004 + py * 0.006);
        rt.stars.material.uniforms.uTime.value = rt.time;
        rt.satellites.material.uniforms.uTime.value = rt.time;
        rt.satellites.material.uniforms.uDrift.value.copy(rt.stars.material.uniforms.uDrift.value);
        if (rt.fleetTime) rt.fleetTime.value = rt.time;
        rt.shipRoutes.forEach((route, i) => {
            const progress = (rt.time / route.duration + route.phase) % 1;
            const angle = route.angle * Math.PI / 180, dx = Math.cos(angle), dy = Math.sin(angle);
            // The full path extends beyond the viewport in every aspect ratio.
            const reach = Math.abs(dx) * (rt.camera.right * (1 + Math.abs(route.x)) + 0.25) + Math.abs(dy) * (1.25 + Math.abs(route.y));
            const distance = (progress * 2 - 1) * reach;
            const ship = rt.ships[i];
            ship.position.set(route.x * rt.camera.right + distance * dx, route.y + distance * dy, -3 - i);
            ship.scale.setScalar(route.scale * (rt.mobile ? 0.7 : 1));
            ship.rotation.set(
                0.25 + Math.sin(rt.time * 0.35 + i * 1.4) * 0.11,
                -0.2 + Math.cos(rt.time * 0.22 + i * 0.9) * 0.05,
                angle + Math.sin(rt.time * 0.28 + i * 2.1) * 0.018
            );
        });
        rt.camera.position.x = Math.sin(phase) * 0.012 + px * 0.016;
        rt.camera.position.y = Math.sin(phase * 0.7) * 0.006 + py * 0.01;
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
        const dt = Math.min(delta, 0.05);
        rt.time += dt;
        const smooth = Math.min(1, Math.max(0.02, dt * 4.5));
        rt.px += (pointerGoal.x - rt.px) * smooth;
        rt.py += (pointerGoal.y - rt.py) * smooth;
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
        root.style.removeProperty('--galaxy-mx');
        root.style.removeProperty('--galaxy-my');
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
        renderer.outputColorSpace = THREE.SRGBColorSpace;
        const rt = {
            canvas, renderer, scene: new THREE.Scene(), camera: new THREE.OrthographicCamera(-1, 1, 1, -1, 0.1, 100),
            textures: new Set(), mobile: window.innerWidth < 768, quality: 1, ready: false, closed: false,
            time: 0, lastFrame: 0, raf: 0, cadence: [], frameBudget: 1 / 60,
            px: 0, py: 0,
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
        const names = ['space-nebula-' + size + '.webp', 'earth-day-' + size + '.jpg', 'earth-night.png', 'earth-clouds.jpg'];
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
    window.addEventListener('pointermove', event => {
        if (root.dataset.theme !== 'galaxy' || motion.matches) return;
        const w = Math.max(1, window.innerWidth);
        const h = Math.max(1, window.innerHeight);
        pointerGoal.x = (event.clientX / w - 0.5) * 2;
        pointerGoal.y = (0.5 - event.clientY / h) * 2;
        root.style.setProperty('--galaxy-mx', (event.clientX / w * 100).toFixed(1) + '%');
        root.style.setProperty('--galaxy-my', (event.clientY / h * 100).toFixed(1) + '%');
    }, { passive: true });
    window.addEventListener('pointerleave', () => {
        pointerGoal.x = 0;
        pointerGoal.y = 0;
    }, { passive: true });
    if (motion.addEventListener) motion.addEventListener('change', sync);
    else if (motion.addListener) motion.addListener(sync);
    sync();
})();
