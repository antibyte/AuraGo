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

    let currentMode = 0;
    let previousMode = -1;
    let modeStartTime = 0;
    let previousModeStartTime = 0;
    let modeTransitionStart = 0;
    const MODE_DURATION = 18;
    const MODE_FADE = 2;
    const MODE_COUNT = 4;

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
    let colorAccent;

    let textCanvas;
    let textMask;
    const textMaskSize = { width: 720, height: 420 };
    const textLetters = [];

    let mouseGridX = 0;
    let mouseGridZ = 0;
    let mouseActive = false;
    let mouseDown = false;
    let mouseLastImpulseAt = 0;
    let raycaster;
    let mousePlane;
    let mouseGlow;

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

    function clamp(value, min, max) {
        return Math.max(min, Math.min(max, value));
    }

    function smoothstep(edge0, edge1, value) {
        const x = clamp((value - edge0) / (edge1 - edge0), 0, 1);
        return x * x * (3 - 2 * x);
    }

    function modeWeight(mode, t) {
        if (mode === previousMode) {
            return 1 - smoothstep(0, MODE_FADE, t - modeTransitionStart);
        }
        if (mode === currentMode) {
            return previousMode >= 0 ? smoothstep(0, MODE_FADE, t - modeTransitionStart) : 1;
        }
        return 0;
    }

    function modeElapsed(mode, t) {
        if (mode === previousMode) return Math.max(0, t - previousModeStartTime);
        if (mode === currentMode) return Math.max(0, t - modeStartTime);
        return 0;
    }

    function resetModeTransition(t) {
        previousMode = -1;
        previousModeStartTime = t;
        modeTransitionStart = t;
        modeStartTime = t;
    }

    function ensureImpactAssets() {
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
        
        ensureImpactAssets();
        
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

    function buildTextMask() {
        if (textMask) return;

        textCanvas = document.createElement('canvas');
        textCanvas.width = textMaskSize.width;
        textCanvas.height = textMaskSize.height;
        const ctx = textCanvas.getContext('2d');
        if (!ctx) return;

        ctx.clearRect(0, 0, textCanvas.width, textCanvas.height);
        ctx.fillStyle = '#ffffff';
        ctx.textAlign = 'center';
        ctx.textBaseline = 'middle';
        ctx.font = '900 132px Inter, system-ui, sans-serif';
        ctx.fillText('AURA GO', textCanvas.width / 2, textCanvas.height / 2 + 8);

        const image = ctx.getImageData(0, 0, textCanvas.width, textCanvas.height).data;
        textMask = image;
        textLetters.length = 0;
        const spans = [0.06, 0.20, 0.34, 0.48, 0.60, 0.74, 0.88];
        for (let i = 0; i < spans.length; i++) {
            textLetters.push({ center: spans[i], delay: i * 0.18 });
        }
    }

    function sampleTextMask(x, z) {
        if (!textMask) return { alpha: 0, letterIndex: -1 };

        const u = clamp((x + GRID.width * 0.5) / GRID.width, 0, 1);
        const v = clamp((z + GRID.depth * 0.5) / GRID.depth, 0, 1);
        const px = Math.floor((1 - u) * (textMaskSize.width - 1));
        const py = Math.floor((1 - v) * (textMaskSize.height - 1));
        const alpha = textMask[(py * textMaskSize.width + px) * 4 + 3] / 255;
        let letterIndex = -1;
        let nearest = Infinity;

        for (let i = 0; i < textLetters.length; i++) {
            const distance = Math.abs(u - textLetters[i].center);
            if (distance < nearest) {
                nearest = distance;
                letterIndex = i;
            }
        }

        return { alpha, letterIndex };
    }

    function textHeightAt(x, z, t) {
        const weight = Math.max(modeWeight(0, t), 0);
        if (weight <= 0.001) return 0;

        const sample = sampleTextMask(x, z);
        if (sample.alpha <= 0.02 || sample.letterIndex < 0) return 0;

        const elapsed = Math.max(0, modeElapsed(0, t) - textLetters[sample.letterIndex].delay);
        const rise = smoothstep(0, 1.4, elapsed);
        const overshoot = Math.sin(clamp(elapsed / 1.45, 0, 1) * Math.PI) * Math.exp(-elapsed * 0.9) * 0.34;
        const settle = 0.9 + Math.sin(t * 0.9 + sample.letterIndex * 0.7) * 0.08;
        const edgeLift = smoothstep(0.04, 0.6, sample.alpha);
        return edgeLift * (rise * 1.35 + overshoot) * settle * weight;
    }

    function mouseHeightAt(x, z, t) {
        const weight = Math.max(modeWeight(1, t), 0);
        if (weight <= 0.001 || !mouseActive) return 0;

        const dist = Math.hypot(x - mouseGridX, z - mouseGridZ);
        const radius = mouseDown ? 3.4 : 2.5;
        const push = Math.exp(-(dist * dist) / radius) * (mouseDown ? 1.15 : 0.7);
        const ring = Math.sin(dist * 5.2 - t * 8.4) * Math.exp(-dist * 0.72) * 0.26;
        return (push + ring) * weight;
    }

    function colorOverrideForPosition(x, z, height, t, target) {
        const weight = Math.max(modeWeight(3, t), 0);
        if (weight <= 0.001) {
            colorForHeight(height, target);
            return;
        }

        const pattern = Math.floor(modeElapsed(3, t) / 4) % 4;
        if (pattern === 0) {
            colorAccent.setHSL(((x + z) * 0.035 + t * 0.12) % 1, 0.9, 0.58);
        } else if (pattern === 1) {
            const rings = Math.sin(Math.hypot(x, z) * 1.9 - t * 5.1) * 0.5 + 0.5;
            colorAccent.setHSL(0.56 + rings * 0.25, 0.95, 0.48 + rings * 0.18);
        } else if (pattern === 2) {
            const fire = clamp((height + 0.9) / 1.8 + Math.sin(x * 0.8 + t * 4.2) * 0.12, 0, 1);
            colorAccent.setRGB(0.2 + fire * 1.2, 0.04 + fire * 0.42, fire * fire * 0.08);
        } else {
            const pulse = Math.pow(Math.sin(x * 1.2 - z * 0.9 + t * 7.5) * 0.5 + 0.5, 2.2);
            colorAccent.setRGB(0.08 + pulse * 0.35, 0.35 + pulse * 0.65, 0.75 + pulse * 0.25);
        }

        colorForHeight(height, target);
        target.lerp(colorAccent, weight * 0.88);
    }

    function colorTextForPosition(x, z, height, t, target) {
        const weight = Math.max(modeWeight(0, t), 0);
        if (weight <= 0.001) return false;

        const sample = sampleTextMask(x, z);
        if (sample.alpha <= 0.02) return false;

        colorAccent.setHSL(0.52 + sample.letterIndex * 0.035 + Math.sin(t * 0.7) * 0.03, 0.92, 0.64);
        target.lerp(colorAccent, smoothstep(0.04, 0.68, sample.alpha) * weight * 0.82);
        return true;
    }

    function projectMouseToGrid(event) {
        if (!raycaster || !camera || !mousePlane || !canvas) return;

        const rect = canvas.getBoundingClientRect();
        const ndc = new THREE.Vector2(
            ((event.clientX - rect.left) / rect.width) * 2 - 1,
            -(((event.clientY - rect.top) / rect.height) * 2 - 1)
        );
        const hit = new THREE.Vector3();
        raycaster.setFromCamera(ndc, camera);
        if (raycaster.ray.intersectPlane(mousePlane, hit)) {
            mouseGridX = clamp(hit.x, -GRID.width * 0.5, GRID.width * 0.5);
            mouseGridZ = clamp(hit.z - surface.position.z, -GRID.depth * 0.5, GRID.depth * 0.5);
            mouseActive = true;
        }
    }

    function onMouseMove(event) {
        projectMouseToGrid(event);
    }

    function onMouseDown(event) {
        mouseDown = true;
        projectMouseToGrid(event);
        ensureImpactAssets();
        addImpulse(mouseGridX, mouseGridZ, 1.35);
        spawnImpactBurst(mouseGridX, surface.position.y + 0.18, mouseGridZ + surface.position.z, 0.92);
    }

    function onMouseUp() {
        mouseDown = false;
    }

    function enterMode(mode, t) {
        if (mode === 0) {
            buildTextMask();
        } else if (mode === 1 && canvas) {
            mouseActive = false;
            mouseDown = false;
            canvas.style.pointerEvents = 'auto';
            canvas.classList.add('threedee-mouse-mode');
            window.addEventListener('mousemove', onMouseMove, true);
            window.addEventListener('mousedown', onMouseDown, true);
            window.addEventListener('mouseup', onMouseUp, true);
        } else if (mode === 2) {
            nextImpulseAt = Math.min(nextImpulseAt, t + 0.4);
        }
    }

    function exitMode(mode) {
        if (mode === 1 && canvas) {
            canvas.style.pointerEvents = 'none';
            canvas.classList.remove('threedee-mouse-mode');
            window.removeEventListener('mousemove', onMouseMove, true);
            window.removeEventListener('mousedown', onMouseDown, true);
            window.removeEventListener('mouseup', onMouseUp, true);
            mouseActive = false;
            mouseDown = false;
        }
    }

    function setMode(mode, t) {
        if (!active) return;
        const nextMode = ((mode % MODE_COUNT) + MODE_COUNT) % MODE_COUNT;
        if (nextMode === currentMode) return;
        const now = typeof t === 'number' ? t : performance.now() / 1000;
        exitMode(currentMode);
        previousMode = currentMode;
        previousModeStartTime = modeStartTime;
        currentMode = nextMode;
        modeStartTime = now;
        modeTransitionStart = now;
        enterMode(currentMode, now);
    }

    function updateMode(dt, t) {
        if (previousMode >= 0 && t - modeTransitionStart >= MODE_FADE) {
            previousMode = -1;
        }
        if (t - modeStartTime >= MODE_DURATION) {
            setMode(currentMode + 1, t);
        }
        if (currentMode === 1 && mouseActive) {
            const interval = mouseDown ? 0.12 : 0.22;
            if (t - mouseLastImpulseAt >= interval) {
                addImpulse(mouseGridX, mouseGridZ, mouseDown ? 0.34 : 0.18);
                mouseLastImpulseAt = t;
            }
        }

        if (surface && surface.material) {
            const colorWeight = modeWeight(3, t);
            surface.material.opacity = 0.62 + colorWeight * 0.08;
            surface.material.emissive = surface.material.emissive || new THREE.Color(0x000000);
            surface.material.emissive.setRGB(0.02 * colorWeight, 0.08 * colorWeight, 0.14 * colorWeight);
            surface.material.emissiveIntensity = colorWeight * 0.45;
        }
        if (mouseGlow) {
            const glowWeight = modeWeight(1, t) * (mouseActive ? 1 : 0);
            mouseGlow.visible = glowWeight > 0.01;
            mouseGlow.position.set(mouseGridX, surface.position.y + 0.5, mouseGridZ + surface.position.z);
            mouseGlow.material.opacity = 0.34 * glowWeight;
            const scale = 0.85 + Math.sin(t * 8) * 0.08 + (mouseDown ? 0.35 : 0);
            mouseGlow.scale.set(scale, scale, scale);
        }
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

        height += textHeightAt(x, z, t);
        height += mouseHeightAt(x, z, t);

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
        colorAccent = new THREE.Color(0x7dd3fc);

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

        raycaster = new THREE.Raycaster();
        mousePlane = new THREE.Plane(new THREE.Vector3(0, 1, 0), -surface.position.y);
        mouseGlow = new THREE.Sprite(new THREE.SpriteMaterial({
            map: smokeTexture,
            color: 0x7dd3fc,
            transparent: true,
            opacity: 0,
            blending: THREE.AdditiveBlending,
            depthWrite: false
        }));
        mouseGlow.visible = false;
        scene.add(mouseGlow);
        
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
            if (currentMode === 3 || previousMode === 3) {
                colorOverrideForPosition(x, z, height, t, colorScratch);
            }
            if (currentMode === 0 || previousMode === 0) colorTextForPosition(x, z, height, t, colorScratch);
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
        updateMode(dt, t);
        if (currentMode === 2) {
            spawnImpulse(t, false);
        }
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
        currentMode = 0;
        resetModeTransition(performance.now() / 1000);
        enterMode(currentMode, modeStartTime);

        if (currentMode === 2 && impulses.length === 0) {
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
        exitMode(currentMode);
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
        },
        setMode: function (mode) {
            if (typeof mode !== 'number') return;
            setMode(mode);
        }
    };
})();
