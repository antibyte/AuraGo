(function () {
    'use strict';

    let canvas, renderer, scene, camera;
    let animationId = null;
    let active = false;
    let lastFrame = 0;
    let surface;
    let surfaceGeometry;
    let gridLines;
    let gridGeometry;
    let basePositions;
    let nextImpulseAt = 0;
    const impulses = [];

    let smokeTexture;
    const spheres = [];
    const sprites = [];
    const fogPlanes = [];
    const impactLights = [];
    const shockwaves = [];
    let cameraShake = 0;

    const GRID = {
        width: 24,
        depth: 14,
        cols: 72,
        rows: 42
    };

    const IMPULSE_LIFETIME = 4.8;
    const MAX_IMPULSES = 8;
    const FRAME_INTERVAL = 1000 / 30;

    let colorLow;
    let colorMid;
    let colorHigh;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'threedee' &&
            !prefersReducedMotion() &&
            window.innerWidth >= 768 &&
            !!window.THREE;
    }

    function createCanvas() {
        canvas = document.createElement('canvas');
        canvas.id = 'threedee-overlay';
        Object.assign(canvas.style, {
            position: 'fixed', inset: '0',
            width: '100vw', height: '100vh',
            pointerEvents: 'none', zIndex: '0',
            opacity: '0.76',
            mixBlendMode: 'normal',
            display: 'none'
        });
        document.body.appendChild(canvas);
    }

    function addImpulse(x, z, strength) {
        impulses.push({
            x,
            z,
            strength,
            start: performance.now() / 1000
        });

        while (impulses.length > MAX_IMPULSES) {
            impulses.shift();
        }
    }

    function createSmokeTexture() {
        const canvas = document.createElement('canvas');
        canvas.width = 32;
        canvas.height = 32;
        const ctx = canvas.getContext('2d');
        const grad = ctx.createRadialGradient(16, 16, 0, 16, 16, 16);
        grad.addColorStop(0, 'rgba(255, 255, 255, 1)');
        grad.addColorStop(1, 'rgba(255, 255, 255, 0)');
        ctx.fillStyle = grad;
        ctx.fillRect(0, 0, 32, 32);
        return new THREE.CanvasTexture(canvas);
    }

    function createSmokeSprite(x, y, z, color, scale, life, options) {
        const opts = options || {};
        const material = new THREE.SpriteMaterial({
            map: smokeTexture,
            color: color,
            transparent: true,
            opacity: opts.opacity == null ? 0.5 : opts.opacity,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        });
        const sprite = new THREE.Sprite(material);
        sprite.position.set(x, y, z);
        sprite.scale.set(scale, scale, scale);
        scene.add(sprite);
        sprites.push({
            sprite,
            life,
            maxLife: life,
            initialScale: scale,
            baseOpacity: material.opacity,
            vx: opts.vx || 0,
            vy: opts.vy || 0,
            vz: opts.vz || 0,
            spin: opts.spin || 0,
            expansion: opts.expansion == null ? 1.0 : opts.expansion,
            fadePower: opts.fadePower == null ? 1.0 : opts.fadePower,
            kind: opts.kind || 'smoke'
        });
    }

    function createTrailParticle(s, kind) {
        const trailCone = kind || 'trailCone';
        const heat = Math.random();
        const spark = trailCone === 'debrisSpark';
        const drift = spark ? 0.9 : 0.24;
        const scale = spark ? 0.045 + Math.random() * 0.07 : 0.14 + Math.random() * 0.16;
        const life = spark ? 0.24 + Math.random() * 0.22 : 0.34 + Math.random() * 0.3;
        const backX = -s.vx * (0.04 + Math.random() * 0.05);
        const backZ = -s.vz * (0.04 + Math.random() * 0.05);
        const color = spark
            ? (heat > 0.5 ? 0xfff2a8 : 0xff7a18)
            : (heat > 0.58 ? 0xffa23a : 0x7dd3fc);

        createSmokeSprite(
            s.x + backX + (Math.random() - 0.5) * 0.14,
            s.y + 0.08 + (Math.random() - 0.5) * 0.12,
            s.zScene + backZ + (Math.random() - 0.5) * 0.14,
            color,
            scale,
            life,
            {
                vx: backX * 7 + (Math.random() - 0.5) * drift,
                vy: spark ? 0.8 + Math.random() * 1.6 : 0.12 + Math.random() * 0.28,
                vz: backZ * 7 + (Math.random() - 0.5) * drift,
                spin: (Math.random() - 0.5) * 3.5,
                opacity: spark ? 0.85 : 0.42,
                expansion: spark ? 0.35 : 1.65,
                fadePower: spark ? 1.7 : 1.25,
                kind: trailCone
            }
        );
    }

    function spawnImpactBurst(x, y, z, strength) {
        for (let j = 0; j < 10; j++) {
            const angle = Math.random() * Math.PI * 2;
            const speed = 0.55 + Math.random() * 1.4;
            createSmokeSprite(x, y + 0.04, z, j % 3 === 0 ? 0xffffff : 0xff8a1c, 0.25 + Math.random() * 0.38, 0.72 + Math.random() * 0.45, {
                vx: Math.cos(angle) * speed,
                vy: 0.25 + Math.random() * 0.95,
                vz: Math.sin(angle) * speed,
                spin: (Math.random() - 0.5) * 2.2,
                opacity: 0.52,
                expansion: 2.4,
                fadePower: 1.35,
                kind: 'blastBloom'
            });
        }

        for (let j = 0; j < 22; j++) {
            const angle = Math.random() * Math.PI * 2;
            const speed = 1.2 + Math.random() * 3.2;
            createTrailParticle({
                x,
                y: y + 0.04,
                zScene: z,
                vx: -Math.cos(angle) * speed,
                vz: -Math.sin(angle) * speed
            }, 'debrisSpark');
        }

        const flash = new THREE.PointLight(0xffb347, 13 * strength, 20);
        flash.position.set(x, y + 0.12, z);
        scene.add(flash);
        impactLights.push({ light: flash, life: 0.72, maxLife: 0.72, baseIntensity: 13 * strength });

        for (let ring = 0; ring < 3; ring++) {
            const shock = new THREE.Mesh(window.shockwaveGeom, window.shockwaveMat.clone());
            shock.position.set(x, y + 0.04 + ring * 0.018, z);
            shock.rotation.x = -Math.PI / 2;
            scene.add(shock);
            shockwaves.push({
                mesh: shock,
                life: 0.68 + ring * 0.14,
                maxLife: 0.68 + ring * 0.14,
                maxScale: 7.0 + ring * 2.4,
                baseOpacity: 0.88 - ring * 0.16,
                kind: 'blastRing'
            });
        }

        cameraShake = Math.max(cameraShake, 0.16 + strength * 0.12);
    }

    function spawnImpulse(t, immediate) {
        if (!immediate && t < nextImpulseAt) return;

        const marginX = GRID.width * 0.36;
        const marginZ = GRID.depth * 0.36;
        const x = (Math.random() - 0.5) * marginX * 2;
        const z = (Math.random() - 0.5) * marginZ * 2;
        const strength = 0.55 + Math.random() * 0.45;
        
        if (!window.sphereGeom) {
            window.sphereGeom = new THREE.SphereGeometry(0.18, 24, 24);
            window.sphereMat = new THREE.MeshStandardMaterial({
                color: 0xffaa00, 
                emissive: 0xff4400, 
                emissiveIntensity: 0.8, 
                roughness: 0.2, 
                metalness: 0.7
            });
            window.shockwaveGeom = new THREE.RingGeometry(0.1, 0.4, 32);
            window.shockwaveMat = new THREE.MeshBasicMaterial({
                color: 0xffaa00,
                transparent: true,
                opacity: 0.8,
                side: THREE.DoubleSide,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
        }
        
        const driftX = (Math.random() - 0.5) * 2.2;
        const driftZ = 1.8 + Math.random() * 1.6;
        const fallTime = 0.82 + Math.random() * 0.22;
        const startX = x - driftX;
        const startZScene = z - 5.3 - driftZ;

        const mesh = new THREE.Mesh(window.sphereGeom, window.sphereMat);
        const light = new THREE.PointLight(0xff6600, 3, 10);
        mesh.add(light);
        mesh.position.set(startX, 6, startZScene);
        scene.add(mesh);
        
        spheres.push({
            mesh: mesh,
            x: startX,
            y: 6,
            zScene: startZScene,
            vx: driftX / fallTime,
            vz: driftZ / fallTime,
            vy: -8 - Math.random() * 4,
            spinX: 2 + Math.random() * 4,
            spinZ: 1.2 + Math.random() * 3,
            strength: strength
        });

        nextImpulseAt = t + 1.2 + Math.random() * 1.7;
    }

    function pruneImpulses(t) {
        for (let i = impulses.length - 1; i >= 0; i--) {
            if (t - impulses[i].start > IMPULSE_LIFETIME) {
                impulses.splice(i, 1);
            }
        }
    }

    function updateParticles(dt, t) {
        for (let i = sprites.length - 1; i >= 0; i--) {
            let p = sprites[i];
            p.life -= dt;
            if (p.life <= 0) {
                scene.remove(p.sprite);
                p.sprite.material.dispose();
                sprites.splice(i, 1);
            } else {
                const ratio = p.life / p.maxLife;
                p.sprite.material.opacity = p.baseOpacity * Math.pow(ratio, p.fadePower);
                const s = p.initialScale * (1 + p.expansion * (1 - ratio));
                p.sprite.scale.set(s, s, s);
                p.sprite.position.x += p.vx * dt;
                p.sprite.position.y += (p.vy + 0.12) * dt;
                p.sprite.position.z += p.vz * dt;
                p.sprite.material.rotation += p.spin * dt;
            }
        }

        for (let i = spheres.length - 1; i >= 0; i--) {
            let s = spheres[i];
            s.x += s.vx * dt;
            s.y += s.vy * dt;
            s.zScene += s.vz * dt;
            s.mesh.position.set(s.x, s.y, s.zScene);
            s.mesh.rotation.x += s.spinX * dt;
            s.mesh.rotation.z += s.spinZ * dt;
            
            if (Math.random() < 0.78) {
                createTrailParticle(s, 'trailCone');
            }
            if (Math.random() < 0.28) {
                createTrailParticle(s, 'debrisSpark');
            }

            const gridZ = s.zScene + 5.3;
            if (s.y <= heightAt(s.x, gridZ, t) - 2.55) {
                addImpulse(s.x, gridZ, s.strength * 1.38);
                spawnImpactBurst(s.x, s.y, s.zScene, s.strength);
                
                scene.remove(s.mesh);
                spheres.splice(i, 1);
            }
        }
        
        for (let i = impactLights.length - 1; i >= 0; i--) {
            let l = impactLights[i];
            l.life -= dt;
            if (l.life <= 0) {
                scene.remove(l.light);
                impactLights.splice(i, 1);
            } else {
                l.light.intensity = (l.baseIntensity || 8) * Math.pow(l.life / l.maxLife, 1.15);
            }
        }
        
        for (let i = shockwaves.length - 1; i >= 0; i--) {
            let s = shockwaves[i];
            s.life -= dt;
            if (s.life <= 0) {
                scene.remove(s.mesh);
                s.mesh.material.dispose();
                shockwaves.splice(i, 1);
            } else {
                const progress = 1.0 - (s.life / s.maxLife);
                const scale = 1.0 + progress * (s.maxScale || 6.0);
                s.mesh.scale.set(scale, scale, scale);
                s.mesh.material.opacity = (s.baseOpacity || 0.8) * (1.0 - Math.pow(progress, 1.35));
                s.mesh.position.y += dt * 0.08;
            }
        }
        
        fogPlanes.forEach(p => {
            p.mesh.position.x += p.vx * dt;
            p.mesh.position.z += p.vz * dt;
            p.mesh.position.y = p.baseY + Math.sin(t * 0.5 + p.mesh.position.x) * 0.2;
            
            if (p.mesh.position.x > 15) {
                p.mesh.position.x = -15;
                p.mesh.position.z = (Math.random() - 0.5) * 20;
            }
        });
    }

    function heightAt(x, z, t) {
        const slowWave = Math.sin(x * 0.46 + t * 0.72) * 0.22;
        const crossWave = Math.sin(z * 0.68 - t * 0.58) * 0.18;
        const diagonalWave = Math.sin((x + z) * 0.32 + t * 0.42) * 0.12;
        let height = slowWave + crossWave + diagonalWave;

        for (const impulse of impulses) {
            const age = t - impulse.start;
            if (age < 0 || age > IMPULSE_LIFETIME) continue;

            const dist = Math.hypot(x - impulse.x, z - impulse.z);
            const radius = age * 3.15;
            const ring = Math.sin((dist - radius) * 4.9);
            const envelope = Math.exp(-Math.abs(dist - radius) * 1.08);
            const fade = Math.pow(1 - age / IMPULSE_LIFETIME, 1.4);
            height += ring * envelope * fade * impulse.strength;
        }

        return height;
    }

    function colorForHeight(height, target) {
        const normalized = Math.max(0, Math.min(1, (height + 0.75) / 1.5));
        target.copy(colorLow).lerp(colorMid, Math.min(1, normalized * 1.2));
        if (normalized > 0.55) {
            target.lerp(colorHigh, (normalized - 0.55) / 0.45 * 0.62);
        }
    }

    function createSurfaceMaterial() {
        return new THREE.MeshStandardMaterial({
            color: 0xffffff,
            roughness: 0.96,
            metalness: 0.02,
            transparent: true,
            opacity: 0.62,
            side: THREE.DoubleSide,
            vertexColors: true
        });
    }

    function createGridMaterial() {
        return new THREE.LineBasicMaterial({
            color: 0xa8c7ff,
            transparent: true,
            opacity: 0.18,
            depthWrite: false
        });
    }

    function createRectGridGeometry() {
        const vertices = [];
        const stepX = GRID.width / GRID.cols;
        const stepZ = GRID.depth / GRID.rows;
        const left = -GRID.width / 2;
        const back = -GRID.depth / 2;

        for (let row = 0; row <= GRID.rows; row++) {
            const z = back + row * stepZ;
            for (let col = 0; col < GRID.cols; col++) {
                const x1 = left + col * stepX;
                const x2 = x1 + stepX;
                vertices.push(x1, 0, z, x2, 0, z);
            }
        }

        for (let col = 0; col <= GRID.cols; col++) {
            const x = left + col * stepX;
            for (let row = 0; row < GRID.rows; row++) {
                const z1 = back + row * stepZ;
                const z2 = z1 + stepZ;
                vertices.push(x, 0, z1, x, 0, z2);
            }
        }

        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute('position', new THREE.Float32BufferAttribute(vertices, 3));
        return geometry;
    }

    function createHeightfield() {
        surfaceGeometry = new THREE.PlaneGeometry(GRID.width, GRID.depth, GRID.cols, GRID.rows);
        surfaceGeometry.rotateX(-Math.PI / 2);

        const position = surfaceGeometry.getAttribute('position');
        basePositions = new Float32Array(position.array.length);
        basePositions.set(position.array);

        const colors = new Float32Array(position.count * 3);
        const initialColor = new THREE.Color();
        for (let i = 0; i < position.count; i++) {
            colorForHeight(0, initialColor);
            colors[i * 3] = initialColor.r;
            colors[i * 3 + 1] = initialColor.g;
            colors[i * 3 + 2] = initialColor.b;
        }
        surfaceGeometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));

        surface = new THREE.Mesh(surfaceGeometry, createSurfaceMaterial());
        surface.position.set(0, -2.55, -5.3);
        surface.rotation.z = -0.015;
        scene.add(surface);

        gridGeometry = createRectGridGeometry();
        gridLines = new THREE.LineSegments(gridGeometry, createGridMaterial());
        gridLines.position.copy(surface.position);
        gridLines.rotation.copy(surface.rotation);
        scene.add(gridLines);
    }

    function initScene() {
        if (scene) return true;

        colorLow = new THREE.Color(0x101626);
        colorMid = new THREE.Color(0x24324c);
        colorHigh = new THREE.Color(0x9bd7ff);

        scene = new THREE.Scene();
        scene.fog = new THREE.FogExp2(0x060914, 0.035);

        camera = new THREE.PerspectiveCamera(44, window.innerWidth / window.innerHeight, 0.1, 1000);
        camera.position.set(0, 5.15, 9.8);
        camera.lookAt(0, -1.95, -5.8);

        try {
            renderer = new THREE.WebGLRenderer({
                canvas,
                alpha: true,
                antialias: true,
                powerPreference: 'high-performance'
            });
        } catch (err) {
            console.warn('[ThreeDeeShader] WebGL unavailable:', err);
            scene = null;
            camera = null;
            return false;
        }
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
        renderer.setSize(window.innerWidth, window.innerHeight, false);
        renderer.setClearColor(0x000000, 0);
        if ('outputColorSpace' in renderer) {
            renderer.outputColorSpace = THREE.SRGBColorSpace;
        }

        const ambient = new THREE.HemisphereLight(0xbfd7ff, 0x0b0f1a, 1.05);
        scene.add(ambient);

        const key = new THREE.DirectionalLight(0xb9d9ff, 1.35);
        key.position.set(-3.5, 6, 6);
        scene.add(key);

        const rim = new THREE.DirectionalLight(0x7dd3fc, 0.55);
        rim.position.set(5, 3, -8);
        scene.add(rim);

        createHeightfield();
        
        if (!smokeTexture) smokeTexture = createSmokeTexture();
        
        if (fogPlanes.length === 0) {
            for (let i = 0; i < 15; i++) {
                const material = new THREE.MeshBasicMaterial({
                    map: smokeTexture,
                    transparent: true,
                    opacity: 0.12,
                    depthWrite: false,
                    color: 0x9bd7ff,
                    blending: THREE.AdditiveBlending
                });
                const plane = new THREE.Mesh(new THREE.PlaneGeometry(12, 12), material);
                plane.position.set((Math.random() - 0.5) * 30, -2.0, (Math.random() - 0.5) * 20);
                plane.rotation.x = -Math.PI / 2;
                scene.add(plane);
                fogPlanes.push({
                    mesh: plane,
                    vx: (Math.random() * 0.5 + 0.5),
                    vz: (Math.random() - 0.5) * 0.2,
                    baseY: -2.3 + Math.random() * 0.5
                });
            }
        }
        
        return true;
    }

    function resize() {
        if (!renderer || !camera) return;

        const width = window.innerWidth;
        const height = window.innerHeight;
        renderer.setSize(width, height, false);
        camera.aspect = width / height;
        camera.updateProjectionMatrix();
    }

    function updateSurface(t) {
        if (!surfaceGeometry || !gridGeometry) return;

        const position = surfaceGeometry.getAttribute('position');
        const color = surfaceGeometry.getAttribute('color');
        const colorScratch = new THREE.Color();

        for (let i = 0; i < position.count; i++) {
            const x = basePositions[i * 3];
            const z = basePositions[i * 3 + 2];
            const height = heightAt(x, z, t);

            position.array[i * 3 + 1] = height;
            colorForHeight(height, colorScratch);
            color.array[i * 3] = colorScratch.r;
            color.array[i * 3 + 1] = colorScratch.g;
            color.array[i * 3 + 2] = colorScratch.b;
        }

        position.needsUpdate = true;
        color.needsUpdate = true;
        surfaceGeometry.computeVertexNormals();
        surfaceGeometry.attributes.normal.needsUpdate = true;

        const gridPosition = gridGeometry.getAttribute('position');
        const gridArray = gridPosition.array;
        for (let i = 0; i < gridArray.length; i += 3) {
            gridArray[i + 1] = heightAt(gridArray[i], gridArray[i + 2], t) + 0.012;
        }
        gridPosition.needsUpdate = true;
    }

    function render(time) {
        if (!active) return;
        animationId = requestAnimationFrame(render);

        if (time - lastFrame < FRAME_INTERVAL) return;
        lastFrame = time;

        const dt = FRAME_INTERVAL * 0.001;
        const t = time * 0.001;
        spawnImpulse(t, false);
        pruneImpulses(t);
        updateParticles(dt, t);
        updateSurface(t);

        if (camera) {
            const sway = Math.sin(t * 0.2) * 0.18;
            const shakeX = (Math.random() - 0.5) * cameraShake;
            const shakeY = (Math.random() - 0.5) * cameraShake * 0.6;
            camera.position.x = sway + shakeX;
            camera.position.y = 5.1 + Math.sin(t * 0.17) * 0.18 + shakeY;
            camera.lookAt(sway * 0.25, -2.0, -5.7);
            cameraShake *= 0.82;
            if (cameraShake < 0.003) cameraShake = 0;
        }

        renderer.render(scene, camera);
    }

    function start() {
        if (active || !shouldRun()) return;

        if (!canvas) createCanvas();
        if (!initScene()) return;
        active = true;
        lastFrame = 0;
        canvas.style.display = 'block';

        if (impulses.length === 0) {
            addImpulse(0, 0, 0.9);
        }
        animationId = requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) {
            cancelAnimationFrame(animationId);
            animationId = null;
        }
        if (canvas) canvas.style.display = 'none';
    }

    function sync() {
        if (shouldRun()) {
            start();
        } else {
            stop();
        }
    }

    function init() {
        if (!window.THREE) return;
        createCanvas();
        sync();

        const observer = new MutationObserver(sync);
        observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', function () {
            if (active) resize();
            sync();
        });
        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            if (mq.addEventListener) {
                mq.addEventListener('change', sync);
            } else if (mq.addListener) {
                mq.addListener(sync);
            }
        }
        document.addEventListener('visibilitychange', function () {
            if (document.hidden) {
                stop();
            } else {
                sync();
            }
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    window.AuraGoThreeDee = {
        start,
        stop,
        sync,
        impulse: function (x, z, strength) {
            addImpulse(
                typeof x === 'number' ? x : 0,
                typeof z === 'number' ? z : 0,
                typeof strength === 'number' ? strength : 0.85
            );
        }
    };
})();
