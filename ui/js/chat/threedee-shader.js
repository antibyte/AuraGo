(function () {
    'use strict';

    let canvas, renderer, scene, camera;
    let animationId = null;
    let active = false;
    let lastFrame = 0;
    let globalTime = 0;
    let surface;
    let surfaceGeometry;
    let gridLines;
    let gridGeometry;
    let basePositions;
    let nextImpulseAt = 0;
    const impulses = [];
    let heightCache = null;
    let normalFrameToggle = 0;
    const _preImpulse = { ages: new Float64Array(64), fades: new Float64Array(64), strengths: new Float64Array(64), radii: new Float64Array(64), alive: 0 };

    let currentMode = 0;
    let previousMode = -1;
    let modeStartTime = 0;
    let previousModeStartTime = 0;
    let modeTransitionStart = 0;
    const MODE_DURATION = 18;
    const MODE_FADE = 2;
    const MODE_COUNT = 5;
    const COLOR_PATTERN_DURATION = 4;
    const COLOR_PATTERN_FADE = 1.4;

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

    const IMPULSE_LIFETIME = 3.6;
    const MAX_IMPULSES = 48;
    const FRAME_INTERVAL = 1000 / 30;
    const IMPULSE_FADE_CUTOFF = 0.005;

    let colorLow;
    let colorMid;
    let colorHigh;
    let colorAccent;
    let colorAccentNext;

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

    let robotGroup;
    let robotModel;
    let robotThrusterLight;
    let robotModelBaseY = 0;
    let robotLoading = false;
    let robotLoaded = false;
    let robotLoadWarned = false;
    let robotDracoLoader;
    let robotTargetPosition;
    let robotSurfaceNormal;
    let targetQuaternion;
    let composedRobotQuaternion;
    let robotHeadingQuaternion;
    let robotSwayQuaternion;
    let robotForward;
    let robotUp;
    let robotState;
    let robotVelocity;
    let robotBounds;
    const ROBOT_DUEL_DISTANCE = 3.85;
    const ROBOT_DUEL_COOLDOWN = 1.05;
    const ROBOT_PROJECTILE_SPEED = 7.4;
    const ROBOT_HIT_RECOIL = 1.55;
    const ROBOT_RED_TARGET_SIZE = 1.62;
    const ROBOT_FLIGHT_MIN_INTERVAL = 6.4;
    const ROBOT_FLIGHT_MAX_INTERVAL = 12.5;
    const ROBOT_FLIGHT_DURATION = 2.05;
    const ROBOT_FLIGHT_HEIGHT = 1.12;
    const ROBOT_FLIGHT_MAX_HEIGHT = 3.35;
    const ROBOT_WAVE_DAMPING_HEIGHT = 1.35;
    const ROBOT_DAMAGE_DENT_RADIUS = 0.38;
    const ROBOT_DAMAGE_DENT_DEPTH = 0.035;
    const ROBOT_DAMAGE_DENT_NOISE = 0.006;
    const ROBOT_DAMAGE_MAX_DENT_OFFSET = 0.075;
    const ROBOT_DAMAGE_DECAL_OFFSET = 0.018;
    const ROBOT_DAMAGE_MAX_SCORCH_MARKS = 14;
    const ROBOT_FOOT_JET_UNDERSIDE_Y = -0.2;
    const ROBOT_THRUSTER_RIPPLE_LIFETIME = 2.8;
    const ROBOT_THRUSTER_RIPPLE_MIN_GAP = 3.05;
    const ROBOT_THRUSTER_RIPPLE_MAX_ACTIVE_PER_ROBOT = 1;
    const ROBOT_THRUSTER_RIPPLE_WIDTH = 0.72;
    const MAX_ROBOT_THRUSTER_RIPPLES = 6;
    const RED_ROBOT_FOOT_JET_OFFSETS = [
        [0, ROBOT_FOOT_JET_UNDERSIDE_Y, -0.25],
        [0, ROBOT_FOOT_JET_UNDERSIDE_Y, 0.25]
    ];
    const MAX_ENERGY_PROJECTILES = 18;
    const energyProjectiles = [];
    const robotFleet = [];
    const robotThrusterRipples = [];

    const AURA_VERTEX_SHADER = [
        'uniform float time;',
        'uniform float hitIntensity;',
        'varying vec3 vNormal;',
        'varying vec3 vViewPosition;',
        'varying vec3 vPosition;',
        'varying vec2 vUv;',
        'float organicWave(vec3 p, float t) {',
        '    float w = sin(p.x * 4.0 + t * 5.0) * cos(p.y * 4.0 + t * 4.0);',
        '    w += sin(p.z * 6.0 - t * 6.0) * 0.5;',
        '    w += cos((p.x + p.y + p.z) * 8.0 + t * 8.0) * 0.25;',
        '    return w;',
        '}',
        'void main() {',
        '    vUv = uv;',
        '    vPosition = position;',
        '    vNormal = normalize(normalMatrix * normal);',
        '    float displacement = organicWave(position, time) * 0.12 * hitIntensity;',
        '    vec3 displacedPosition = position + normal * displacement;',
        '    vec4 mvPosition = modelViewMatrix * vec4(displacedPosition, 1.0);',
        '    vViewPosition = -mvPosition.xyz;',
        '    gl_Position = projectionMatrix * mvPosition;',
        '}'
    ].join('\n');

    const AURA_FRAGMENT_SHADER = [
        'uniform vec3 color;',
        'uniform float intensity;',
        'uniform float hitIntensity;',
        'uniform float glowPower;',
        'uniform float time;',
        'varying vec3 vNormal;',
        'varying vec3 vViewPosition;',
        'varying vec3 vPosition;',
        'varying vec2 vUv;',
        'float plasmaPattern(vec3 p, float t) {',
        '    float v = sin(p.x * 6.0 + t * 4.0) + sin(p.y * 6.0 - t * 3.5);',
        '    v += cos(p.z * 5.0 + t * 3.0) + cos((p.x + p.y) * 4.0 - t * 5.0);',
        '    return v * 0.25 + 0.5;',
        '}',
        'void main() {',
        '    vec3 normal = normalize(vNormal);',
        '    vec3 viewDir = normalize(vViewPosition);',
        '    float edgeFactor = pow(1.0 - abs(dot(normal, viewDir)), glowPower);',
        '    float plasma = plasmaPattern(vPosition, time);',
        '    float energyLines = smoothstep(0.42, 0.46, abs(sin(vPosition.y * 8.0 + plasma * 4.0 - time * 6.0)));',
        '    energyLines += smoothstep(0.42, 0.46, abs(cos(vPosition.x * 8.0 - plasma * 4.0 + time * 5.0))) * 0.5;',
        '    vec3 energyColor = color + vec3(0.5, 0.5, 0.5) * hitIntensity;',
        '    float finalGlow = edgeFactor * 0.6 + energyLines * 0.4 * (0.3 + 0.7 * hitIntensity);',
        '    gl_FragColor = vec4(mix(color, energyColor, energyLines * 0.6), finalGlow * intensity);',
        '}'
    ].join('\n');

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
        // Compact expired impulses in-place (avoids splice shifting)
        let write = 0;
        for (let read = 0; read < impulses.length; read++) {
            if (globalTime - impulses[read].start <= IMPULSE_LIFETIME) {
                if (write !== read) impulses[write] = impulses[read];
                write++;
            }
        }
        impulses.length = write;

        impulses.push({
            x,
            z,
            strength,
            start: globalTime
        });

        // Only evict if truly over capacity after pruning
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

    function softClip(value, limit) {
        return Math.tanh(value / limit) * limit;
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
        if (!window.energyProjectileGeom) {
            window.energyProjectileGeom = new THREE.SphereGeometry(0.12, 24, 24);
        }
        if (!window.superRocketGeom) {
            window.superRocketGeom = new THREE.ConeGeometry(0.12, 0.46, 16);
            window.superRocketGeom.rotateX(Math.PI / 2);
        }
        if (!window.superGrenadeGeom) {
            window.superGrenadeGeom = new THREE.SphereGeometry(0.24, 24, 24);
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
        // Calculate the local coordinates on the heightfield
        const worldPos = new THREE.Vector3(x, y, z);
        const localPos = worldPos.clone();
        if (surface) {
            surface.worldToLocal(localPos);
        }

        // Compute local surface normal
        const localNormal = new THREE.Vector3(0, 1, 0);
        if (surface) {
            const eps = 0.22;
            const t = globalTime;
            const left = heightAt(clamp(localPos.x - eps, -GRID.width * 0.5, GRID.width * 0.5), localPos.z, t);
            const right = heightAt(clamp(localPos.x + eps, -GRID.width * 0.5, GRID.width * 0.5), localPos.z, t);
            const back = heightAt(localPos.x, clamp(localPos.z - eps, -GRID.depth * 0.5, GRID.depth * 0.5), t);
            const front = heightAt(localPos.x, clamp(localPos.z + eps, -GRID.depth * 0.5, GRID.depth * 0.5), t);
            localNormal.set(left - right, eps * 2, back - front).normalize();
        }

        // Transform local normal to world space
        const worldNormal = new THREE.Vector3(0, 1, 0);
        if (surface) {
            worldNormal.copy(localNormal).transformDirection(surface.matrixWorld);
        }

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
        // Position it offset along the normal
        const lightPos = new THREE.Vector3(x, y, z).addScaledVector(worldNormal, 0.12);
        flash.position.copy(lightPos);
        scene.add(flash);
        impactLights.push({ light: flash, life: 0.72, maxLife: 0.72, baseIntensity: 13 * strength });

        for (let ring = 0; ring < 3; ring++) {
            const shock = new THREE.Mesh(window.shockwaveGeom, window.shockwaveMat.clone());
            
            // Position it offset along the normal to prevent z-fighting
            const offsetPos = new THREE.Vector3(x, y, z).addScaledVector(worldNormal, 0.04 + ring * 0.018);
            shock.position.copy(offsetPos);

            // Align the ring's geometry normal (0, 0, 1) to the worldNormal
            const normalAlign = new THREE.Quaternion();
            normalAlign.setFromUnitVectors(new THREE.Vector3(0, 0, 1), worldNormal);
            shock.quaternion.copy(normalAlign);

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
        mesh.castShadow = true;
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
        let write = 0;
        for (let read = 0; read < impulses.length; read++) {
            if (t - impulses[read].start <= IMPULSE_LIFETIME) {
                if (write !== read) impulses[write] = impulses[read];
                write++;
            }
        }
        impulses.length = write;
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

            if (surface) {
                const worldPos = new THREE.Vector3(s.x, s.y, s.zScene);
                const localPos = worldPos.clone();
                surface.worldToLocal(localPos);
                
                const localHeight = heightAt(localPos.x, localPos.z, t);
                if (localPos.y <= localHeight) {
                    const localCollision = new THREE.Vector3(localPos.x, localHeight, localPos.z);
                    const worldCollision = surface.localToWorld(localCollision);
                    
                    addImpulse(localPos.x, localPos.z, s.strength * 1.38);
                    spawnImpactBurst(worldCollision.x, worldCollision.y, worldCollision.z, s.strength);
                    
                    scene.remove(s.mesh);
                    spheres.splice(i, 1);
                }
            } else {
                const gridZ = s.zScene + 5.3;
                if (s.y <= heightAt(s.x, gridZ, t) - 2.55) {
                    addImpulse(s.x, gridZ, s.strength * 1.38);
                    spawnImpactBurst(s.x, s.y, s.zScene, s.strength);
                    
                    scene.remove(s.mesh);
                    spheres.splice(i, 1);
                }
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

        // Spawn ambient fireflies/embers floating upwards from the wave field
        if (active && Math.random() < 0.18 && sprites.length < 160) {
            const rx = (Math.random() - 0.5) * GRID.width;
            const rz = (Math.random() - 0.5) * GRID.depth;
            const h = heightAt(rx, rz, t);
            const ry = h + (surface ? surface.position.y : -2.55) + 0.08;
            
            // Pick ember color based on mode
            let emberColor = 0x7dd3fc; // Default ice blue
            if (currentMode === 0) {
                emberColor = 0xc084fc; // Purple during text mode
            } else if (currentMode === 2) {
                emberColor = 0xffa23a; // Fiery orange during meteor storm
            } else if (currentMode === 4) {
                emberColor = Math.random() < 0.5 ? 0xff4bb5 : 0x22d3ee; // Swirling pink/cyan in vortex mode
            } else if (h > 0.3) {
                emberColor = 0xb9d9ff; // Brighter highlights on peaks
            }
            
            createSmokeSprite(rx, ry, rz + (surface ? surface.position.z : -5.3), emberColor, 0.05 + Math.random() * 0.06, 1.8 + Math.random() * 1.4, {
                vx: (Math.random() - 0.5) * 0.4,
                vy: 0.35 + Math.random() * 0.45,
                vz: (Math.random() - 0.5) * 0.4,
                spin: (Math.random() - 0.5) * 2.5,
                opacity: 0.65,
                expansion: 0.08,
                fadePower: 1.4,
                kind: 'ember'
            });
        }
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
        const px = Math.floor(u * (textMaskSize.width - 1));
        const py = Math.floor(v * (textMaskSize.height - 1));
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

    function colorPatternAt(pattern, x, z, height, t, target) {
        if (pattern === 0) {
            target.setHSL(((x + z) * 0.035 + t * 0.12) % 1, 0.9, 0.58);
        } else if (pattern === 1) {
            const rings = Math.sin(Math.hypot(x, z) * 1.9 - t * 5.1) * 0.5 + 0.5;
            target.setHSL(0.56 + rings * 0.25, 0.95, 0.48 + rings * 0.18);
        } else if (pattern === 2) {
            const fire = clamp((height + 0.9) / 1.8 + Math.sin(x * 0.8 + t * 4.2) * 0.12, 0, 1);
            target.setRGB(0.2 + fire * 1.2, 0.04 + fire * 0.42, fire * fire * 0.08);
        } else {
            const pulse = Math.pow(Math.sin(x * 1.2 - z * 0.9 + t * 7.5) * 0.5 + 0.5, 2.2);
            target.setRGB(0.08 + pulse * 0.35, 0.35 + pulse * 0.65, 0.75 + pulse * 0.25);
        }
    }

    function colorOverrideForPosition(x, z, height, t, target) {
        const weight = Math.max(modeWeight(3, t), 0);
        if (weight <= 0.001) {
            colorForHeight(height, target);
            return;
        }

        const cycle = modeElapsed(3, t) / COLOR_PATTERN_DURATION;
        const pattern = Math.floor(cycle) % 4;
        const cycleProgress = cycle - Math.floor(cycle);
        const nextPattern = (pattern + 1) % 4;
        const blend = smoothstep(1 - COLOR_PATTERN_FADE / COLOR_PATTERN_DURATION, 1, cycleProgress);

        colorPatternAt(pattern, x, z, height, t, colorAccent);
        if (blend > 0.001) {
            colorPatternAt(nextPattern, x, z, height, t, colorAccentNext);
            colorAccent.lerp(colorAccentNext, blend);
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

    function colorVortexForPosition(x, z, height, t, target) {
        const weight = Math.max(modeWeight(4, t), 0);
        if (weight <= 0.001) return;

        const dist = Math.hypot(x, z);
        const angle = Math.atan2(z, x);
        const swirlVal = Math.sin(dist * 1.5 - angle * 2.0 - t * 5.5) * 0.5 + 0.5;

        // mix neon pink/magenta and electric blue
        colorAccent.setHSL(0.82 + swirlVal * 0.12, 0.95, 0.55); // pink to magenta
        colorAccentNext.setHSL(0.58, 0.95, 0.5); // electric blue

        colorAccent.lerp(colorAccentNext, Math.sin(dist * 0.8 - t * 2.0) * 0.5 + 0.5);
        target.lerp(colorAccent, weight * 0.88);
    }

    function vortexHeightAt(x, z, t) {
        const weight = Math.max(modeWeight(4, t), 0);
        if (weight <= 0.001) return 0;

        const dist = Math.hypot(x, z);
        const angle = Math.atan2(z, x);

        // swirling whirlpool waves
        const swirl = Math.sin(dist * 1.6 - angle * 2.0 - t * 5.5) * 0.36;
        // vortex funnel shape pulled down at the center and lifted around the walls
        const funnel = -0.9 * Math.exp(-(dist * dist) / 1.4) + 0.45 * Math.exp(-((dist - 3.2) * (dist - 3.2)) / 2.5);

        return (swirl + funnel) * weight;
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
        } else if (mode === 4) {
            // Mode 4 setup: spawn central energy blast
            ensureImpactAssets();
            spawnImpactBurst(0, surface ? surface.position.y + 0.5 : -2.0, surface ? surface.position.z : -5.3, 1.45);
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
        const now = typeof t === 'number' ? t : globalTime;
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

    function heightAt(x, z, t, options) {
        const slowWave = Math.sin(x * 0.46 + t * 0.72) * 0.22;
        const crossWave = Math.sin(z * 0.68 - t * 0.58) * 0.18;
        const diagonalWave = Math.sin((x + z) * 0.32 + t * 0.42) * 0.12;
        let height = slowWave + crossWave + diagonalWave;
        const ignoreRobotOwner = options && options.ignoreRobotOwner;
        const ignoreRobotFeedbackWaves = options && options.ignoreRobotFeedbackWaves;

        // Use pre-computed impulse invariants when available (bulk surface pass)
        const pre = _preImpulse;
        const aliveCount = pre.alive;
        if (aliveCount > 0) {
            for (let idx = 0; idx < aliveCount; idx++) {
                const dist = Math.hypot(x - impulses[idx].x, z - impulses[idx].z);
                const radius = pre.radii[idx];
                const ring = Math.sin((dist - radius) * 3.65);
                const envelope = Math.exp(-Math.abs(dist - radius) * 0.78);
                height += softClip(ring * envelope * pre.fades[idx] * pre.strengths[idx], 0.72);
            }
        } else {
            // Fallback for isolated calls (robot normal sampling, collision checks)
            for (const impulse of impulses) {
                const age = t - impulse.start;
                if (age < 0 || age > IMPULSE_LIFETIME) continue;
                const fade = Math.pow(1 - age / IMPULSE_LIFETIME, 1.4);
                if (fade < IMPULSE_FADE_CUTOFF) continue;

                const dist = Math.hypot(x - impulse.x, z - impulse.z);
                const radius = age * 3.05;
                const ring = Math.sin((dist - radius) * 3.65);
                const envelope = Math.exp(-Math.abs(dist - radius) * 0.78);
                const strength = softClip(impulse.strength, 1.08);
                height += softClip(ring * envelope * fade * strength, 0.72);
            }
        }

        // Only evaluate mode-specific height contributions when that mode is active
        if (currentMode === 0 || previousMode === 0) height += textHeightAt(x, z, t);
        if (currentMode === 1 || previousMode === 1) height += mouseHeightAt(x, z, t);
        if (currentMode === 4 || previousMode === 4) height += vortexHeightAt(x, z, t);

        // Robot thruster downdraft and ripples under the robot
        if (!ignoreRobotFeedbackWaves && robotState) {
            const activeRobots = robotFleet.length ? robotFleet : [{ id: 'blue', state: robotState }];
            for (let ri = 0; ri < activeRobots.length; ri++) {
                const bot = activeRobots[ri];
                if (!bot || !bot.state) continue;
                const botOwner = bot.id || 'robot';
                if (ignoreRobotOwner && botOwner === ignoreRobotOwner) continue;
                const distToRobot = Math.hypot(x - (bot.state.px || bot.state.x), z - (bot.state.pz || bot.state.z));
                // Skip far-away robots (Gaussian is negligible beyond ~2.5 units)
                if (distToRobot > 2.5) continue;
                const flightWaveInfluence = robotWaveInfluenceForFlightHeight(bot.state.flightLift || 0);
                const hoverDepression = -0.2 * Math.exp(-(distToRobot * distToRobot) / 0.72);
                height += hoverDepression * flightWaveInfluence;
            }
        }

        if (!ignoreRobotFeedbackWaves) {
            height += robotThrusterRippleHeightAt(x, z, t, ignoreRobotOwner);
        }

        return height;
    }

    function robotWaveInfluenceForFlightHeight(flightLift) {
        return 1 - smoothstep(0.18, ROBOT_WAVE_DAMPING_HEIGHT, Math.max(0, flightLift || 0)) * 0.86;
    }

    function robotThrusterRippleHeightAt(x, z, t, ignoreOwner) {
        let height = 0;
        for (const ripple of robotThrusterRipples) {
            if (ignoreOwner && ripple.owner === ignoreOwner) continue;
            const age = t - ripple.start;
            if (age < 0 || age > ROBOT_THRUSTER_RIPPLE_LIFETIME) continue;

            const dist = Math.hypot(x - ripple.x, z - ripple.z);
            const progress = age / ROBOT_THRUSTER_RIPPLE_LIFETIME;
            const radius = 0.18 + age * 1.25;
            const delta = dist - radius;
            const ridge = Math.exp(-(delta * delta) / ROBOT_THRUSTER_RIPPLE_WIDTH);
            const wakeDelta = delta + 0.66;
            const trailingWake = Math.exp(-(wakeDelta * wakeDelta) / (ROBOT_THRUSTER_RIPPLE_WIDTH * 1.8));
            const rippleAttack = smoothstep(0, 0.18, age);
            const rippleRelease = 1 - smoothstep(0.58, 1, progress);
            const rippleFade = rippleAttack * rippleRelease * rippleRelease;
            height += (ridge - trailingWake * 0.32) * rippleFade * ripple.strength;
        }
        return softClip(height, 0.18);
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

    function createRobotConfig(options) {
        return {
            id: options.id,
            url: options.url,
            group: null,
            model: null,
            thrusterLight: null,
            modelBaseY: 0,
            loading: false,
            loaded: false,
            loadWarned: false,
            state: {
                x: options.x,
                z: options.z,
                y: 0,
                waterY: 0,
                yaw: 0,
                seed: options.seed == null ? Math.random() * Math.PI * 2 : options.seed,
                lastBounce: 0,
                lastShot: -999,
                lastSuperweaponAt: options.id === 'blue' ? 3.0 : 7.5,
                isSuperweaponCharging: false,
                recoil: 0,
                hitFlash: 0,
                hits: 0,
                flightLift: 0,
                flightStartedAt: -999,
                flightDuration: ROBOT_FLIGHT_DURATION,
                flightPeak: ROBOT_FLIGHT_HEIGHT,
                nextFlightAt: NaN,
                lastThrusterRippleAt: -999,
                thrusterRipplePrimed: false,
                flightWasActive: false,
                pendingThrusterRipple: 0,
                px: options.x,
                pz: options.z
            },
            velocity: new THREE.Vector2(options.vx, options.vz),
            targetPosition: new THREE.Vector3(),
            surfaceNormal: new THREE.Vector3(0, 1, 0),
            targetQuaternion: new THREE.Quaternion(),
            composedQuaternion: new THREE.Quaternion(),
            headingQuaternion: new THREE.Quaternion(),
            swayQuaternion: new THREE.Quaternion(),
            forward: new THREE.Vector3(0, 0, 1),
            up: new THREE.Vector3(0, 1, 0),
            accentHex: options.accentHex,
            projectileHex: options.projectileHex,
            accentColor: new THREE.Color(options.accentHex),
            headingOffset: options.headingOffset || 0,
            opponent: null,
            damageMeshes: [],
            damageScorchMarks: []
        };
    }

    function aliasPrimaryRobot(bot) {
        if (!bot) return;
        robotGroup = bot.group;
        robotModel = bot.model;
        robotThrusterLight = bot.thrusterLight;
        robotModelBaseY = bot.modelBaseY;
        robotLoading = bot.loading;
        robotLoaded = bot.loaded;
        robotLoadWarned = bot.loadWarned;
        robotTargetPosition = bot.targetPosition;
        robotSurfaceNormal = bot.surfaceNormal;
        targetQuaternion = bot.targetQuaternion;
        composedRobotQuaternion = bot.composedQuaternion;
        robotHeadingQuaternion = bot.headingQuaternion;
        robotSwayQuaternion = bot.swayQuaternion;
        robotForward = bot.forward;
        robotUp = bot.up;
        robotState = bot.state;
        robotVelocity = bot.velocity;
    }

    function ensureRobotPhysicsState() {
        if (!robotBounds) {
            robotBounds = {
                x: GRID.width * 0.5 - 1.7,
                z: GRID.depth * 0.5 - 1.35
            };
        }
        if (robotFleet.length === 0) {
            const blueRobot = createRobotConfig({
                id: 'blue',
                url: '/3d/robot.glb',
                x: -2.55,
                z: -0.9,
                vx: 0.72,
                vz: 0.46,
                accentHex: 0x22d3ee,
                projectileHex: 0x5cf6ff,
                headingOffset: Math.PI
            });
            const redRobot = createRobotConfig({
                id: 'red',
                url: '/3d/redrobot.glb',
                x: 2.35,
                z: 0.9,
                vx: -0.68,
                vz: -0.42,
                accentHex: 0xff3b6d,
                projectileHex: 0xff4f8a,
                headingOffset: 0
            });
            blueRobot.opponent = redRobot;
            redRobot.opponent = blueRobot;
            robotFleet.push(blueRobot, redRobot);
        }
        aliasPrimaryRobot(robotFleet[0]);
    }

    function normalizeFloatingRobot(root, bot) {
        const config = bot || robotFleet[0] || null;
        if (config) {
            config.materials = [];
            config.damageMeshes = [];
        }
        const box = new THREE.Box3().setFromObject(root);
        const size = box.getSize(new THREE.Vector3());
        const maxAxis = Math.max(size.x, size.y, size.z);
        if (!Number.isFinite(maxAxis) || maxAxis <= 0) return;

        const targetSize = config && config.id === 'red' ? ROBOT_RED_TARGET_SIZE : 1.45;
        const scale = targetSize / maxAxis;
        root.scale.setScalar(scale);
        root.updateMatrixWorld(true);

        const scaledBox = new THREE.Box3().setFromObject(root);
        const center = scaledBox.getCenter(new THREE.Vector3());
        root.position.set(-center.x, -scaledBox.min.y, -center.z);
        if (config) config.modelBaseY = root.position.y;
        if (!config || config.id === 'blue') robotModelBaseY = root.position.y;
        root.rotation.y = config ? config.headingOffset : Math.PI;

        root.traverse(function (node) {
            if (!node.isMesh) return;
            node.frustumCulled = false;
            node.castShadow = true;
            node.receiveShadow = true;
            if (node.geometry && node.geometry.attributes && node.geometry.attributes.position) {
                node.geometry = node.geometry.clone();
                const position = node.geometry.attributes.position;
                if (position.setUsage && THREE.DynamicDrawUsage) position.setUsage(THREE.DynamicDrawUsage);
                node.geometry.userData.robotDamageBasePositions = new Float32Array(position.array);
                if (config) config.damageMeshes.push(node);
            }
            if (!node.material) return;
            node.material = Array.isArray(node.material)
                ? node.material.map(function (material) { return material.clone(); })
                : node.material.clone();
            const materials = Array.isArray(node.material) ? node.material : [node.material];
            materials.forEach(function (material) {
                if ('roughness' in material) material.roughness = Math.min(0.7, material.roughness == null ? 0.5 : material.roughness);
                if ('metalness' in material) material.metalness = Math.max(0.18, material.metalness == null ? 0.18 : material.metalness);
                if ('emissive' in material) {
                    material.emissive = material.emissive || new THREE.Color(0x000000);
                    material.emissive.lerp(config ? config.accentColor : new THREE.Color(0x59d5ff), config && config.id === 'red' ? 0.28 : 0.16);
                }
                if ('emissiveIntensity' in material) {
                    material.emissiveIntensity = Math.max(config && config.id === 'red' ? 0.24 : 0.18, material.emissiveIntensity || 0);
                }
                material.needsUpdate = true;
                if (config) config.materials.push(material);
            });
        });
    }

    function loadFloatingRobot() {
        if (!scene) return;
        ensureRobotPhysicsState();
        robotFleet.forEach(loadRobotAsset);
        aliasPrimaryRobot(robotFleet[0]);
    }

    function loadRobotAsset(bot) {
        if (!bot || bot.loaded || bot.loading || !scene) return;
        if (!THREE.GLTFLoader || !THREE.DRACOLoader) {
            if (!bot.loadWarned) {
                console.warn('[ThreeDeeShader] GLTFLoader or DRACOLoader unavailable for ' + bot.url);
                bot.loadWarned = true;
                if (bot.id === 'blue') robotLoadWarned = true;
            }
            return;
        }

        bot.loading = true;
        const group = new THREE.Group();
        group.name = 'threedee-floating-robot-' + bot.id;
        group.visible = false;
        group.position.set(bot.state.x, surface.position.y + 0.7, bot.state.z + surface.position.z);
        scene.add(group);
        bot.group = group;

        const loader = new THREE.GLTFLoader();
        const dracoLoader = new THREE.DRACOLoader();
        dracoLoader.setDecoderPath('/js/vendor/draco/');
        if (typeof dracoLoader.setDecoderConfig === 'function') {
            dracoLoader.setDecoderConfig({ type: 'wasm' });
        }
        loader.setDRACOLoader(dracoLoader);

        if (bot.id === 'blue') {
            robotGroup = group;
            robotLoading = true;
            robotDracoLoader = dracoLoader;
        }

        const onLoad = function (gltf) {
            const model = gltf.scene || (gltf.scenes && gltf.scenes[0]);
            if (!model) {
                bot.loading = false;
                if (bot.id === 'blue') robotLoading = false;
                scene.remove(group);
                bot.group = null;
                console.warn('[ThreeDeeShader] ' + bot.url + ' contained no scene');
                return;
            }
            bot.model = model;
            normalizeFloatingRobot(model, bot);

            const auraMat = new THREE.ShaderMaterial({
                vertexShader: AURA_VERTEX_SHADER,
                fragmentShader: AURA_FRAGMENT_SHADER,
                uniforms: {
                    color: { value: new THREE.Color(bot.projectileHex) },
                    intensity: { value: 0.0 },
                    hitIntensity: { value: 0.0 },
                    glowPower: { value: 2.2 },
                    time: { value: 0.0 }
                },
                transparent: true,
                blending: THREE.AdditiveBlending,
                depthWrite: false,
                side: THREE.DoubleSide
            });
            const targetSize = bot.id === 'red' ? ROBOT_RED_TARGET_SIZE : 1.45;
            const auraRadius = (targetSize / 2.0) * 1.25;
            const auraGeom = new THREE.SphereGeometry(auraRadius, 32, 32);
            bot.auraMesh = new THREE.Mesh(auraGeom, auraMat);
            bot.auraMesh.position.set(0, targetSize * 0.46, 0);
            group.add(bot.auraMesh);

            group.add(model);
            group.visible = true;
            bot.loaded = true;
            bot.loading = false;
            if (bot.id === 'blue') {
                robotModel = model;
                robotLoaded = true;
                robotLoading = false;
            }
        };
        const onError = function (err) {
            bot.loading = false;
            if (bot.id === 'blue') robotLoading = false;
            scene.remove(group);
            bot.group = null;
            console.warn('[ThreeDeeShader] Could not load floating robot ' + bot.url + ':', err);
        };

        if (bot.id === 'blue') {
            loader.load('/3d/robot.glb', onLoad, undefined, onError);
        } else {
            loader.load('/3d/redrobot.glb', onLoad, undefined, onError);
        }
    }

    function sampleSurfaceNormal(x, z, t, bot, options) {
        ensureRobotPhysicsState();
        const targetNormal = bot && bot.surfaceNormal ? bot.surfaceNormal : robotSurfaceNormal;
        const eps = 0.22;
        const sampleOptions = bot && bot.id ? { ignoreRobotOwner: bot.id, ignoreRobotFeedbackWaves: true } : { ignoreRobotFeedbackWaves: true };
        if (options) Object.assign(sampleOptions, options);
        const left = heightAt(clamp(x - eps, -robotBounds.x, robotBounds.x), z, t, sampleOptions);
        const right = heightAt(clamp(x + eps, -robotBounds.x, robotBounds.x), z, t, sampleOptions);
        const back = heightAt(x, clamp(z - eps, -robotBounds.z, robotBounds.z), t, sampleOptions);
        const front = heightAt(x, clamp(z + eps, -robotBounds.z, robotBounds.z), t, sampleOptions);
        targetNormal.set(left - right, eps * 2, back - front).normalize();
        return targetNormal;
    }

    function bounceFloatingRobotWithinBounds(t, bot) {
        const state = bot && bot.state ? bot.state : robotState;
        const velocity = bot && bot.velocity ? bot.velocity : robotVelocity;
        let bounced = false;
        if (state.x > robotBounds.x) {
            state.x = robotBounds.x;
            if (velocity.x > 0) {
                velocity.x = -velocity.x * 0.88;
                bounced = true;
            }
        } else if (state.x < -robotBounds.x) {
            state.x = -robotBounds.x;
            if (velocity.x < 0) {
                velocity.x = -velocity.x * 0.88;
                bounced = true;
            }
        }

        if (state.z > robotBounds.z) {
            state.z = robotBounds.z;
            if (velocity.y > 0) {
                velocity.y = -velocity.y * 0.88;
                bounced = true;
            }
        } else if (state.z < -robotBounds.z) {
            state.z = -robotBounds.z;
            if (velocity.y < 0) {
                velocity.y = -velocity.y * 0.88;
                bounced = true;
            }
        }

        if (bounced && t - state.lastBounce > 0.16) {
            state.lastBounce = t;
            addImpulse(state.x, state.z, bot && bot.id === 'red' ? 0.38 : 0.32);
            cameraShake = Math.max(cameraShake, bot && bot.id === 'red' ? 0.045 : 0.035);
        }
    }

    function steerRobotTowardDuel(bot, dt, t) {
        if (!bot || !bot.opponent || !bot.opponent.state) return;
        const dx = bot.opponent.state.x - bot.state.x;
        const dz = bot.opponent.state.z - bot.state.z;
        const dist = Math.hypot(dx, dz);
        if (dist < 0.001) return;

        const nx = dx / dist;
        const nz = dz / dist;
        const closeWeight = 1 - clamp(dist / ROBOT_DUEL_DISTANCE, 0, 1);
        const pull = dist > ROBOT_DUEL_DISTANCE * 0.68 ? 0.42 : -0.18;
        const orbit = bot.id === 'blue' ? 1 : -1;
        bot.velocity.x += nx * pull * dt;
        bot.velocity.y += nz * pull * dt;
        bot.velocity.x += -nz * orbit * (0.24 + closeWeight * 0.2) * dt;
        bot.velocity.y += nx * orbit * (0.24 + closeWeight * 0.2) * dt;

        if (dist < 1.55) {
            bot.velocity.x -= nx * 0.62 * dt;
            bot.velocity.y -= nz * 0.62 * dt;
        }

        bot.velocity.x += Math.sin(t * 1.7 + bot.state.seed) * closeWeight * 0.03 * dt;
        bot.velocity.y += Math.cos(t * 1.9 + bot.state.seed) * closeWeight * 0.03 * dt;
    }

    function createJetFlameSprite(bot, t) {
        if (!bot || !bot.group) return;
        const offsets = bot.id === 'red'
            ? RED_ROBOT_FOOT_JET_OFFSETS.map(function (point) { return new THREE.Vector3(point[0], point[1], point[2]); })
            : [new THREE.Vector3(0, ROBOT_FOOT_JET_UNDERSIDE_Y, 0)];
        offsets.forEach(function (offset, index) {
            const foot = bot.group.localToWorld(offset.clone());
            const flameCount = bot.id === 'red' ? 2 : 1;
            for (let flame = 0; flame < flameCount; flame++) {
                const hotCore = Math.random() < 0.44;
                const flameColor = bot.id === 'red'
                    ? (hotCore ? 0xfff0a8 : (Math.random() < 0.62 ? 0xff6a00 : 0xff2a00))
                    : (hotCore ? 0xffaa00 : bot.accentHex);
                const scale = bot.id === 'red'
                    ? 0.2 + Math.random() * 0.15
                    : 0.09 + Math.random() * 0.07;
                createSmokeSprite(
                    foot.x + (Math.random() - 0.5) * 0.07,
                    foot.y - 0.06 - flame * 0.05,
                    foot.z + (Math.random() - 0.5) * 0.07,
                    flameColor,
                    scale,
                    bot.id === 'red' ? 0.38 + Math.random() * 0.24 : 0.45 + Math.random() * 0.35,
                    {
                        vx: (Math.random() - 0.5) * 0.28 + bot.velocity.x * -0.12,
                        vy: bot.id === 'red' ? -2.75 - Math.random() * 1.55 : -1.4 - Math.random() * 1.0,
                        vz: (Math.random() - 0.5) * 0.28 + bot.velocity.y * -0.12,
                        spin: (Math.random() - 0.5) * 6.0 + index * 0.35,
                        opacity: bot.id === 'red' ? 0.96 : 0.85,
                        expansion: bot.id === 'red' ? 1.45 : 1.15,
                        fadePower: bot.id === 'red' ? 1.8 : 1.45,
                        kind: 'robotJetFlame'
                    }
                );
            }
        });
    }

    function scheduleNextRobotFlight(bot, t) {
        if (!bot || !bot.state) return;
        const span = ROBOT_FLIGHT_MAX_INTERVAL - ROBOT_FLIGHT_MIN_INTERVAL;
        const stagger = bot.id === 'red' ? 1.15 : 0;
        const firstDelay = bot.state.nextFlightAt !== bot.state.nextFlightAt ? 2.2 + Math.random() * 3.2 : 0;
        bot.state.nextFlightAt = t + firstDelay + ROBOT_FLIGHT_MIN_INTERVAL + Math.random() * span + stagger;
    }

    function updateRobotFlight(bot, t) {
        if (!bot || !bot.state) return;
        const state = bot.state;
        if (!Number.isFinite(state.nextFlightAt)) {
            scheduleNextRobotFlight(bot, t);
        }

        if (state.flightStartedAt < 0 && t >= state.nextFlightAt) {
            state.flightStartedAt = t;
            state.flightDuration = ROBOT_FLIGHT_DURATION * (0.84 + Math.random() * 0.34);
            const flightHeightRange = ROBOT_FLIGHT_MAX_HEIGHT - ROBOT_FLIGHT_HEIGHT;
            state.flightPeak = ROBOT_FLIGHT_HEIGHT + Math.random() * flightHeightRange;
            state.flightPeak *= bot.id === 'red' ? 1.08 : 0.96;
        }

        if (state.flightStartedAt >= 0) {
            const progress = clamp((t - state.flightStartedAt) / state.flightDuration, 0, 1);
            const rise = Math.sin(progress * Math.PI);
            const hoverBeat = 1 + Math.sin(progress * Math.PI * 4 + state.seed) * 0.035;
            state.flightLift = Math.max(0, rise * state.flightPeak * hoverBeat);
            if (progress >= 1) {
                state.flightStartedAt = -999;
                state.flightLift = 0;
                scheduleNextRobotFlight(bot, t);
            }
        } else {
            state.flightLift = Math.max(0, (state.flightLift || 0) * 0.82);
        }
    }

    function addRobotThrusterRipple(bot, t, strengthScale) {
        if (!bot || !bot.state) return;
        const flightWaveInfluence = robotWaveInfluenceForFlightHeight(bot.state.flightLift || 0);
        const owner = bot.id || 'robot';
        const lastRippleAt = Number.isFinite(bot.state.lastThrusterRippleAt) ? bot.state.lastThrusterRippleAt : -999;
        if (t - lastRippleAt < ROBOT_THRUSTER_RIPPLE_MIN_GAP) {
            return false;
        }

        let activeForRobot = 0;
        for (const ripple of robotThrusterRipples) {
            const age = t - ripple.start;
            if (ripple.owner === owner && age >= 0 && age <= ROBOT_THRUSTER_RIPPLE_LIFETIME) {
                activeForRobot++;
            }
        }
        if (activeForRobot >= ROBOT_THRUSTER_RIPPLE_MAX_ACTIVE_PER_ROBOT) {
            return false;
        }

        const rippleScale = strengthScale == null ? 1 : clamp(strengthScale, 0.35, 1.35);
        const strength = (bot.id === 'red' ? 0.078 : 0.056) * flightWaveInfluence * rippleScale;
        if (strength < 0.012) {
            return false;
        }

        robotThrusterRipples.push({
            owner,
            x: bot.state.x,
            z: bot.state.z,
            start: t,
            strength
        });
        while (robotThrusterRipples.length > MAX_ROBOT_THRUSTER_RIPPLES) {
            robotThrusterRipples.shift();
        }
        bot.state.lastThrusterRippleAt = t;
        return true;
    }

    function updateRobotThrusterRipples(t) {
        let write = 0;
        for (let read = 0; read < robotThrusterRipples.length; read++) {
            if (t - robotThrusterRipples[read].start <= ROBOT_THRUSTER_RIPPLE_LIFETIME) {
                if (write !== read) robotThrusterRipples[write] = robotThrusterRipples[read];
                write++;
            }
        }
        robotThrusterRipples.length = write;
    }

    function updateRobotPositionPhase(bot, dt, t, index) {
        if (!bot) return;
        if (bot.id === 'blue') aliasPrimaryRobot(bot);
        if (!bot.group) return;
        const wasFlightActive = bot.state.flightWasActive === true;
        let pendingThrusterRipple = 0;
        updateRobotFlight(bot, t);
        const isFlightActive = (bot.state.flightLift || 0) > 0.06 || bot.state.flightStartedAt >= 0;
        if (!bot.state.thrusterRipplePrimed) {
            pendingThrusterRipple = Math.max(pendingThrusterRipple, 0.55);
            bot.state.thrusterRipplePrimed = true;
        }
        if (!wasFlightActive && isFlightActive) {
            pendingThrusterRipple = Math.max(pendingThrusterRipple, 1);
        } else if (wasFlightActive && !isFlightActive) {
            pendingThrusterRipple = Math.max(pendingThrusterRipple, 0.78);
        }
        bot.state.flightWasActive = isFlightActive;
        bot.state.pendingThrusterRipple = pendingThrusterRipple;

        const driftX = Math.sin(t * 0.54 + bot.state.z * 0.72 + bot.state.seed) * 0.16;
        const driftZ = Math.cos(t * 0.49 + bot.state.x * 0.61 + index * 0.9) * 0.14;
        bot.velocity.x += driftX * dt;
        bot.velocity.y += driftZ * dt;
        steerRobotTowardDuel(bot, dt, t);

        const speed = bot.velocity.length();
        const maxSpeed = 1.34;
        if (speed > maxSpeed) {
            // Smoothly decay velocity towards maxSpeed instead of hard clamping instantly
            // This allows high-impulse hits to propel the robot far away and slide naturally.
            const drag = 3.2;
            bot.velocity.multiplyScalar(Math.max(maxSpeed / speed, 1 - dt * drag));
        }
        if (speed < 0.42) {
            bot.velocity.x += Math.sin(t + bot.state.seed) * dt * 0.22;
            bot.velocity.y += Math.cos(t * 0.8 + bot.state.seed) * dt * 0.22;
        }

        if (bot.state.isAiming) {
            bot.velocity.multiplyScalar(Math.max(0, 1 - dt * 5.0));
        }

        bot.state.x += bot.velocity.x * dt;
        bot.state.z += bot.velocity.y * dt;
        bounceFloatingRobotWithinBounds(t, bot);
        bot.state.px = bot.state.x;
        bot.state.pz = bot.state.z;
    }

    function updateRobotVisualsPhase(bot, dt, t, index) {
        if (!bot || !bot.group) return;
        if (bot.id === 'blue') aliasPrimaryRobot(bot);
        const pendingThrusterRipple = bot.state.pendingThrusterRipple || 0;
        const flightWaveInfluence = robotWaveInfluenceForFlightHeight(bot.state.flightLift || 0);

        const sampleOptions = bot && bot.id ? { ignoreRobotOwner: bot.id, ignoreRobotFeedbackWaves: true } : { ignoreRobotFeedbackWaves: true };
        const sampledWaterY = bot.id === 'blue' ? heightAt(robotState.x, robotState.z, t, sampleOptions) : heightAt(bot.state.x, bot.state.z, t, sampleOptions);
        const previousWaterY = Number.isFinite(bot.state.visualWaterY) ? bot.state.visualWaterY : sampledWaterY;
        bot.state.visualWaterY = previousWaterY + (sampledWaterY - previousWaterY) * clamp(dt * 3.4, 0, 1);
        const waterY = bot.state.visualWaterY;
        bot.state.waterY = waterY;
        bot.state.recoil = Math.max(0, (bot.state.recoil || 0) - dt * 1.8);
        bot.state.hitFlash = Math.max(0, (bot.state.hitFlash || 0) - dt * 1.5);

        if (bot.materials) {
            const baseEmissive = bot.id === 'red' ? 0.24 : 0.18;
            const superChargingIntensity = (bot.state.isAiming && bot.state.isSuperweaponCharging) ? 4.5 : 0.0;
            bot.materials.forEach(function (mat) {
                if ('emissiveIntensity' in mat) {
                    mat.emissiveIntensity = baseEmissive + bot.state.hitFlash * 3.8 + superChargingIntensity;
                }
            });
        }

        if (bot.auraMesh && bot.auraMesh.material.uniforms) {
            const hitScale = 1.0 + bot.state.hitFlash * 0.22;
            bot.auraMesh.scale.set(hitScale, hitScale, hitScale);
            const pulse = Math.sin(t * 15) * 0.15;
            bot.auraMesh.material.uniforms.intensity.value = bot.state.hitFlash * 0.95 + (bot.state.hitFlash > 0.01 ? pulse * bot.state.hitFlash : 0);
            bot.auraMesh.material.uniforms.hitIntensity.value = bot.state.hitFlash;
            bot.auraMesh.material.uniforms.time.value = t;
        }

        if (bot.state.isAiming && bot.opponent && bot.opponent.group) {
            const isSuper = bot.state.isSuperweaponCharging;
            const spawnChance = isSuper ? (dt * 75) : (dt * 25);
            if (Math.random() < spawnChance) {
                const muzzle = robotMuzzlePosition(bot);
                const opponentPos = bot.opponent.group.position.clone();
                opponentPos.y += 0.12;
                const dir = opponentPos.sub(muzzle).normalize();
                let sparkOffset;
                let sparkVel;
                if (isSuper) {
                    const elapsed = t - bot.state.aimStart;
                    const angle = elapsed * 15.0 + Math.random() * Math.PI;
                    const radius = 0.8 * (1.0 - elapsed / 0.85) + 0.1;
                    const right = new THREE.Vector3(0, 1, 0).cross(dir).normalize();
                    const up = dir.clone().cross(right).normalize();
                    sparkOffset = right.clone().multiplyScalar(Math.sin(angle) * radius)
                                      .add(up.clone().multiplyScalar(Math.cos(angle) * radius))
                                      .addScaledVector(dir, 0.1);
                    sparkVel = sparkOffset.clone().multiplyScalar(-3.0);
                } else {
                    sparkOffset = new THREE.Vector3(
                        (Math.random() - 0.5) * 0.45,
                        (Math.random() - 0.5) * 0.45,
                        (Math.random() - 0.5) * 0.45
                    ).addScaledVector(dir, 0.22);
                    sparkVel = sparkOffset.clone().multiplyScalar(-2.2);
                }
                const sparkPos = muzzle.clone().add(sparkOffset);
                const scale = isSuper ? (0.09 + Math.random() * 0.08) : (0.05 + Math.random() * 0.05);
                const life = isSuper ? 0.38 : 0.25;
                createSmokeSprite(sparkPos.x, sparkPos.y, sparkPos.z, bot.projectileHex, scale, life, {
                    vx: sparkVel.x,
                    vy: sparkVel.y,
                    vz: sparkVel.z,
                    spin: (Math.random() - 0.5) * 12,
                    opacity: 0.95,
                    expansion: isSuper ? 0.15 : 0.25,
                    fadePower: 1.15,
                    kind: 'energyCharge'
                });
            }
        }

        bot.state.y = surface.position.y + waterY * flightWaveInfluence + 0.62 + (bot.state.flightLift || 0) + bot.state.recoil * 0.14 + Math.sin(t * 1.9 + bot.state.seed) * 0.06;
        bot.targetPosition.set(bot.state.x, bot.state.y, bot.state.z + surface.position.z);
        if (bot.id === 'blue') {
            robotGroup.position.lerp(robotTargetPosition, clamp(dt * 4.6, 0, 1));
        } else {
            bot.group.position.lerp(bot.targetPosition, clamp(dt * 4.6, 0, 1));
        }

        const normal = sampleSurfaceNormal(bot.state.x, bot.state.z, t, bot, sampleOptions);
        normal.lerp(bot.up, 1 - flightWaveInfluence).normalize();
        const targetForward = _scratchVec3.set(0, 0, 0);
        if (bot.velocity.lengthSq() > 0.0001) {
            targetForward.set(bot.velocity.x, 0, bot.velocity.y).normalize();
        } else {
            targetForward.copy(bot.forward);
        }

        if (bot.opponent && bot.opponent.state) {
            const duelVector = new THREE.Vector3(bot.opponent.state.x - bot.state.x, 0, bot.opponent.state.z - bot.state.z);
            const duelDistance = Math.max(0.001, duelVector.length());
            if (bot.state.isAiming) {
                duelVector.normalize();
                targetForward.copy(duelVector);
            } else if (duelDistance < ROBOT_DUEL_DISTANCE + 1.1) {
                duelVector.normalize();
                const blend = clamp((ROBOT_DUEL_DISTANCE + 1.1 - duelDistance) / 2.8, 0, 0.78);
                targetForward.lerp(duelVector, blend).normalize();
            }
        }

        const lerpSpeed = bot.state.isAiming ? 15.0 : 6.0;
        bot.forward.lerp(targetForward, clamp(dt * lerpSpeed, 0, 1)).normalize();

        const heading = Math.atan2(bot.forward.x, bot.forward.z);
        if (bot.id === 'blue') {
            robotHeadingQuaternion.setFromAxisAngle(robotUp, heading);
            targetQuaternion.setFromUnitVectors(robotUp, normal);
            targetQuaternion.multiply(robotHeadingQuaternion);

            robotSwayQuaternion.setFromEuler(_scratchEuler.set(
                Math.sin(t * 2.1 + robotState.seed) * 0.075 + normal.z * 0.22,
                0,
                -normal.x * 0.28 + Math.sin(t * 1.6 + robotState.seed) * 0.055,
                'XYZ'
            ));
            composedRobotQuaternion.copy(targetQuaternion).multiply(robotSwayQuaternion);
            targetQuaternion.slerp(composedRobotQuaternion, clamp(dt * 2.8, 0, 1));
            robotGroup.quaternion.slerp(targetQuaternion, clamp(dt * 3.8, 0, 1));
        } else {
            bot.headingQuaternion.setFromAxisAngle(bot.up, heading);
            bot.targetQuaternion.setFromUnitVectors(bot.up, normal);
            bot.targetQuaternion.multiply(bot.headingQuaternion);
            bot.swayQuaternion.setFromEuler(_scratchEuler.set(
                Math.sin(t * 2.1 + bot.state.seed) * 0.075 + normal.z * 0.22,
                0,
                -normal.x * 0.28 + Math.sin(t * 1.6 + bot.state.seed) * 0.055,
                'XYZ'
            ));
            bot.composedQuaternion.copy(bot.targetQuaternion).multiply(bot.swayQuaternion);
            bot.targetQuaternion.slerp(bot.composedQuaternion, clamp(dt * 2.8, 0, 1));
            bot.group.quaternion.slerp(bot.targetQuaternion, clamp(dt * 3.8, 0, 1));
        }

        if (bot.model) {
            bot.model.position.y = bot.modelBaseY + Math.sin(t * 3.2 + bot.state.seed) * 0.035;
        }

        if (!bot.thrusterLight) {
            bot.thrusterLight = new THREE.PointLight(bot.accentHex, 1.8, 5);
            bot.thrusterLight.position.set(0, -0.2, 0);
            bot.group.add(bot.thrusterLight);
            if (bot.id === 'blue') robotThrusterLight = bot.thrusterLight;
        }
        bot.thrusterLight.intensity = 1.4 + (bot.state.flightLift || 0) * 0.9 + bot.state.hitFlash * 2.2 + Math.sin(t * 32 + bot.state.seed) * 0.35 + Math.sin(t * 4.8) * 0.15;

        if (Math.random() < ((bot.id === 'red' ? 0.9 : 0.45) + (bot.state.flightLift || 0) * 0.18) && active) {
            createJetFlameSprite(bot, t);
        }
        if (pendingThrusterRipple > 0) {
            addRobotThrusterRipple(bot, t, pendingThrusterRipple);
        }
    }

    function robotMuzzlePosition(bot) {
        const start = bot.group.position.clone();
        start.y += 0.14;
        const forward = bot.opponent && bot.opponent.group
            ? bot.opponent.group.position.clone().sub(bot.group.position)
            : new THREE.Vector3(bot.velocity.x, 0, bot.velocity.y);
        if (forward.lengthSq() < 0.001) forward.set(0, 0, bot.id === 'blue' ? 1 : -1);
        forward.normalize();
        start.addScaledVector(forward, 0.55);
        return start;
    }

    function robotAimPoint(bot) {
        if (!bot || !bot.group) return new THREE.Vector3();
        const point = bot.group.position.clone();
        if (bot.model) {
            const box = new THREE.Box3().setFromObject(bot.model);
            const height = box.max.y - box.min.y;
            if (Number.isFinite(height) && height > 0.001) {
                box.getCenter(point);
                point.y = box.min.y + height * 0.52;
                return point;
            }
        }
        point.y += 0.58;
        return point;
    }

    function disposeEnergyProjectile(projectile) {
        if (!projectile || !projectile.mesh) return;
        if (scene) scene.remove(projectile.mesh);
        if (projectile.mesh.material && projectile.mesh.material.dispose) {
            projectile.mesh.material.dispose();
        }
    }

    function spawnEnergyProjectile(source, target, t, isSuper) {
        if (!scene || !source || !target || !source.group || !target.group) return;
        ensureImpactAssets();
        while (energyProjectiles.length >= MAX_ENERGY_PROJECTILES) {
            disposeEnergyProjectile(energyProjectiles.shift());
        }

        const color = source.projectileHex;
        let mesh;
        let projectileLight;
        const start = robotMuzzlePosition(source);

        if (isSuper) {
            const material = new THREE.MeshStandardMaterial({
                color: color,
                emissive: color,
                emissiveIntensity: 3.5,
                roughness: 0.1,
                metalness: 0.9
            });
            if (source.id === 'blue') {
                mesh = new THREE.Mesh(window.superGrenadeGeom, material);
                projectileLight = new THREE.PointLight(color, 12.0, 15);
            } else {
                mesh = new THREE.Mesh(window.superRocketGeom, material);
                projectileLight = new THREE.PointLight(color, 12.0, 15);
            }
            mesh.add(projectileLight);
        } else {
            const material = new THREE.MeshBasicMaterial({
                color: color,
                transparent: true,
                opacity: 0.96,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            mesh = new THREE.Mesh(window.energyProjectileGeom, material);
            projectileLight = new THREE.PointLight(color, 4.8, 8);
            mesh.add(projectileLight);
        }

        const targetPosition = robotAimPoint(target);
        let direction = targetPosition.clone().sub(start);
        const dist = direction.length();
        if (direction.lengthSq() < 0.001) direction.set(source.id === 'blue' ? 1 : -1, 0, 0);
        direction.normalize();

        mesh.position.copy(start);

        if (isSuper) {
            mesh.scale.setScalar(1.0);
        } else {
            mesh.scale.setScalar(0.8);
        }

        scene.add(mesh);

        let velocity3D = null;
        let currentSpeed = 0;
        let life = 1.1;

        if (isSuper) {
            life = 2.2;
            source.state.lastSuperweaponAt = t;
            source.state.lastShot = t;
            source.state.recoil = 0.95;
            cameraShake = Math.max(cameraShake, 0.18);

            if (source.id === 'blue') {
                const dx2D = targetPosition.x - start.x;
                const dz2D = targetPosition.z - start.z;
                const dist2D = Math.hypot(dx2D, dz2D);
                const dir2DX = dist2D > 0.001 ? dx2D / dist2D : 1;
                const dir2DZ = dist2D > 0.001 ? dz2D / dist2D : 0;

                const flightTime = Math.max(0.9, dist2D / 4.5);
                const g = 4.2;
                const vx = dir2DX * (dist2D / flightTime);
                const vz = dir2DZ * (dist2D / flightTime);
                const vy = (targetPosition.y - start.y) / flightTime + 0.5 * g * flightTime;
                velocity3D = new THREE.Vector3(vx, vy, vz);
                life = flightTime + 0.5;
            } else {
                currentSpeed = 2.0;
                life = 3.2;
            }
        } else {
            source.state.lastShot = t;
            source.state.recoil = 0.32;
            cameraShake = Math.max(cameraShake, 0.045);
        }

        energyProjectiles.push({
            mesh,
            projectileLight,
            source,
            target,
            direction,
            speed: isSuper ? 0 : (ROBOT_PROJECTILE_SPEED + Math.random() * 1.8),
            life,
            maxLife: life,
            color,
            pulseSeed: Math.random() * Math.PI * 2,
            isSuper: !!isSuper,
            superType: isSuper ? (source.id === 'blue' ? 'grenade' : 'rocket') : null,
            velocity3D,
            currentSpeed
        });

        const muzzleScale = isSuper ? 0.48 : 0.22;
        const muzzleLife = isSuper ? 0.42 : 0.26;
        createSmokeSprite(start.x, start.y, start.z, color, muzzleScale, muzzleLife, {
            vx: direction.x * 0.4,
            vy: 0.18,
            vz: direction.z * 0.4,
            spin: 7,
            opacity: 0.95,
            expansion: isSuper ? 3.8 : 2.6,
            fadePower: 1.5,
            kind: 'energyMuzzle'
        });
    }

    function ensureRobotScorchTexture() {
        if (window.robotScorchTexture) return window.robotScorchTexture;
        if (typeof document === 'undefined') return null;

        const canvas = document.createElement('canvas');
        canvas.width = 64;
        canvas.height = 64;
        const ctx = canvas.getContext('2d');
        if (!ctx) return null;

        const gradient = ctx.createRadialGradient(32, 32, 3, 32, 32, 31);
        gradient.addColorStop(0, 'rgba(0, 0, 0, 0.82)');
        gradient.addColorStop(0.42, 'rgba(0, 0, 0, 0.58)');
        gradient.addColorStop(0.78, 'rgba(0, 0, 0, 0.18)');
        gradient.addColorStop(1, 'rgba(0, 0, 0, 0)');
        ctx.fillStyle = gradient;
        ctx.fillRect(0, 0, canvas.width, canvas.height);

        window.robotScorchTexture = new THREE.CanvasTexture(canvas);
        window.robotScorchTexture.needsUpdate = true;
        return window.robotScorchTexture;
    }

    function intersectRobotDamageBox(box, center, normal) {
        const dir = normal && normal.lengthSq && normal.lengthSq() > 0.001
            ? normal.clone().normalize()
            : new THREE.Vector3(0, 0, 1);
        const halfX = Math.max(0.001, (box.max.x - box.min.x) * 0.5);
        const halfY = Math.max(0.001, (box.max.y - box.min.y) * 0.5);
        const halfZ = Math.max(0.001, (box.max.z - box.min.z) * 0.5);
        let distance = Infinity;
        if (Math.abs(dir.x) > 0.001) distance = Math.min(distance, halfX / Math.abs(dir.x));
        if (Math.abs(dir.y) > 0.001) distance = Math.min(distance, halfY / Math.abs(dir.y));
        if (Math.abs(dir.z) > 0.001) distance = Math.min(distance, halfZ / Math.abs(dir.z));
        if (!Number.isFinite(distance)) distance = Math.max(halfX, halfY, halfZ);
        return center.clone().addScaledVector(dir, distance);
    }

    function resolveRobotDamageImpact(target, impactPosition, impactDirection) {
        const fallback = target && target.group ? target.group.position.clone() : new THREE.Vector3();
        const source = impactPosition ? impactPosition.clone() : fallback;
        let normal = impactDirection && impactDirection.lengthSq && impactDirection.lengthSq() > 0.001
            ? impactDirection.clone().multiplyScalar(-1)
            : source.clone().sub(fallback);
        if (normal.lengthSq() < 0.001) normal.set(target && target.id === 'red' ? 1 : -1, 0, 0);
        normal.normalize();

        const root = target && (target.model || target.group);
        if (!root) return { position: source, normal };
        const box = new THREE.Box3().setFromObject(root);
        const height = box.max.y - box.min.y;
        if (!Number.isFinite(height) || height <= 0.001) return { position: source, normal };

        const center = box.getCenter(new THREE.Vector3());
        center.y = box.min.y + height * 0.52;
        const position = intersectRobotDamageBox(box, center, normal);
        position.y = clamp(position.y, box.min.y + height * 0.24, box.max.y - height * 0.1);
        return { position, normal };
    }

    function applyRobotDamage(target, impactPosition, impactDirection, isSuper) {
        if (!target || !target.group) return;
        const damage = resolveRobotDamageImpact(target, impactPosition, impactDirection);
        applyRobotMeshDent(target, damage, isSuper);
        spawnRobotScorchMarks(target, damage, isSuper);
    }

    function applyRobotMeshDent(target, damage, isSuper) {
        if (!target || !target.damageMeshes || !damage || !damage.position) return;
        const impactPosition = damage.position;
        const worldDirection = damage.normal && damage.normal.lengthSq && damage.normal.lengthSq() > 0.001
            ? damage.normal.clone().multiplyScalar(-1).normalize()
            : new THREE.Vector3(0, -1, 0);
        const radius = ROBOT_DAMAGE_DENT_RADIUS * (isSuper ? 1.62 : 1);
        const depth = ROBOT_DAMAGE_DENT_DEPTH * (isSuper ? 2.15 : 1);

        target.damageMeshes.forEach(function (mesh) {
            if (!mesh || !mesh.geometry || !mesh.geometry.attributes || !mesh.geometry.attributes.position) return;
            const position = mesh.geometry.attributes.position;
            const basePositions = mesh.geometry.userData.robotDamageBasePositions;
            if (!basePositions) return;
            const localImpact = mesh.worldToLocal(impactPosition.clone());
            const localEnd = mesh.worldToLocal(impactPosition.clone().addScaledVector(worldDirection, depth));
            const dentVector = localEnd.sub(localImpact);
            const vertex = new THREE.Vector3();
            const worldVertex = new THREE.Vector3();
            let changed = false;

            for (let i = 0; i < position.count; i++) {
                vertex.fromBufferAttribute(position, i);
                worldVertex.copy(vertex).applyMatrix4(mesh.matrixWorld);
                const dist = worldVertex.distanceTo(impactPosition);
                if (dist > radius) continue;

                const falloff = 1 - smoothstep(0, radius, dist);
                const dentNoise = (Math.random() - 0.5) * ROBOT_DAMAGE_DENT_NOISE * falloff;
                const nextX = vertex.x + dentVector.x * falloff + dentNoise;
                const nextY = vertex.y + dentVector.y * falloff + dentNoise * 0.35;
                const nextZ = vertex.z + dentVector.z * falloff + dentNoise * 0.7;
                const baseIndex = i * 3;
                const currentOffsetX = nextX - basePositions[baseIndex];
                const currentOffsetY = nextY - basePositions[baseIndex + 1];
                const currentOffsetZ = nextZ - basePositions[baseIndex + 2];
                const offsetLength = Math.hypot(currentOffsetX, currentOffsetY, currentOffsetZ);
                const offsetScale = Math.min(1, ROBOT_DAMAGE_MAX_DENT_OFFSET / offsetLength);
                position.setXYZ(
                    i,
                    basePositions[baseIndex] + currentOffsetX * offsetScale,
                    basePositions[baseIndex + 1] + currentOffsetY * offsetScale,
                    basePositions[baseIndex + 2] + currentOffsetZ * offsetScale
                );
                changed = true;
            }

            if (changed) {
                position.needsUpdate = true;
                mesh.geometry.computeVertexNormals();
                if (mesh.geometry.attributes.normal) mesh.geometry.attributes.normal.needsUpdate = true;
            }
        });
    }

    function spawnRobotScorchMarks(target, damage, isSuper) {
        if (!target || !target.group || !damage || !damage.position || !damage.normal) return;
        const texture = ensureRobotScorchTexture();
        if (!texture) return;
        if (!window.robotScorchGeometry) {
            window.robotScorchGeometry = new THREE.PlaneGeometry(1, 1);
        }

        const attachParent = target.model || target.group;
        const modelScale = (target.model && target.model.scale) ? target.model.scale.x : 1.0;

        // Force a matrixWorld update on target.group so that worldToLocal uses accurate current-frame transforms
        target.group.updateMatrixWorld(true);

        const markCount = isSuper ? 3 + Math.floor(Math.random() * 2) : 1 + Math.floor(Math.random() * 2);
        for (let i = 0; i < markCount; i++) {
            const material = new THREE.MeshBasicMaterial({
                map: texture,
                color: 0x050403,
                transparent: true,
                opacity: isSuper ? 0.62 + Math.random() * 0.16 : 0.42 + Math.random() * 0.18,
                depthTest: true,
                depthWrite: false,
                side: THREE.DoubleSide,
                polygonOffset: true,
                polygonOffsetFactor: -2,
                polygonOffsetUnits: -2
            });

            const scorch = new THREE.Mesh(window.robotScorchGeometry, material);
            scorch.name = 'robot-damage-scorch';
            scorch.position.copy(damage.position).addScaledVector(damage.normal, ROBOT_DAMAGE_DECAL_OFFSET);
            attachParent.worldToLocal(scorch.position);
            const localNormalEnd = attachParent.worldToLocal(damage.position.clone().add(damage.normal));
            const localNormalStart = attachParent.worldToLocal(damage.position.clone());
            const localNormal = localNormalEnd.sub(localNormalStart).normalize();
            scorch.quaternion.setFromUnitVectors(new THREE.Vector3(0, 0, 1), localNormal);
            scorch.rotateZ(Math.random() * Math.PI * 2);

            const size = ((isSuper ? 0.16 : 0.095) + Math.random() * (isSuper ? 0.09 : 0.055)) / modelScale;
            scorch.scale.set(size * (0.8 + Math.random() * 0.45), size, size);
            scorch.renderOrder = 18;
            attachParent.add(scorch);
            target.damageScorchMarks.push(scorch);
        }

        while (target.damageScorchMarks.length > ROBOT_DAMAGE_MAX_SCORCH_MARKS) {
            const oldMark = target.damageScorchMarks.shift();
            if (oldMark && oldMark.parent) oldMark.parent.remove(oldMark);
            if (oldMark && oldMark.material) oldMark.material.dispose();
        }
    }

    function applyRobotHitRecoil(projectile, impactPosition) {
        const target = projectile && projectile.target;
        if (!target || !target.velocity || !target.state) return;
        const recoil = projectile.direction ? projectile.direction.clone() : new THREE.Vector3();
        recoil.y = 0;
        if (recoil.lengthSq() < 0.001 && target.group && impactPosition) {
            recoil.copy(target.group.position).sub(impactPosition);
            recoil.y = 0;
        }
        if (recoil.lengthSq() < 0.001) recoil.set(target.id === 'red' ? 1 : -1, 0, 0);
        recoil.normalize();

        const isSuper = projectile.isSuper;
        const pushScale = isSuper ? 1.2 : 0.14;

        if (isSuper) {
            target.velocity.x += recoil.x * ROBOT_HIT_RECOIL * 8.5;
            target.velocity.y += recoil.z * ROBOT_HIT_RECOIL * 8.5;
        } else {
            target.velocity.x += recoil.x * ROBOT_HIT_RECOIL;
            target.velocity.y += recoil.z * ROBOT_HIT_RECOIL;
        }
        target.state.x += recoil.x * pushScale;
        target.state.z += recoil.z * pushScale;

        target.state.recoil = Math.max(target.state.recoil || 0, isSuper ? 1.2 : 0.5);
        target.state.hitFlash = Math.max(target.state.hitFlash || 0, isSuper ? 1.8 : 1.0);
        target.state.hits = (target.state.hits || 0) + 1;
        applyRobotDamage(target, impactPosition, recoil, isSuper);
        bounceFloatingRobotWithinBounds(globalTime, target);
    }

    function spawnEnergyExplosion(projectile, hitTarget) {
        if (!scene || !projectile || !projectile.mesh) return;
        ensureImpactAssets();
        const pos = projectile.mesh.position.clone();
        const color = projectile.color || 0x7dd3fc;
        const worldNormal = new THREE.Vector3(0, 1, 0);
        if (surface) {
            const localPos = pos.clone();
            surface.worldToLocal(localPos);
            const eps = 0.22;
            const t = globalTime;
            const left = heightAt(clamp(localPos.x - eps, -GRID.width * 0.5, GRID.width * 0.5), localPos.z, t);
            const right = heightAt(clamp(localPos.x + eps, -GRID.width * 0.5, GRID.width * 0.5), localPos.z, t);
            const back = heightAt(localPos.x, clamp(localPos.z - eps, -GRID.depth * 0.5, GRID.depth * 0.5), t);
            const front = heightAt(localPos.x, clamp(localPos.z + eps, -GRID.depth * 0.5, GRID.depth * 0.5), t);
            worldNormal.set(left - right, eps * 2, back - front).normalize().transformDirection(surface.matrixWorld);
        }

        const burstCenter = pos.clone().addScaledVector(worldNormal, 0.08);
        for (let i = 0; i < 6; i++) {
            const coreColor = i === 0 ? 0xffffff : color;
            createSmokeSprite(
                burstCenter.x + (Math.random() - 0.5) * 0.08,
                burstCenter.y + (Math.random() - 0.5) * 0.05,
                burstCenter.z + (Math.random() - 0.5) * 0.08,
                coreColor,
                0.16 + Math.random() * 0.16,
                0.24 + Math.random() * 0.18,
                {
                    vx: (Math.random() - 0.5) * 0.45 + worldNormal.x * 0.15,
                    vy: 0.18 + Math.random() * 0.35 + worldNormal.y * 0.25,
                    vz: (Math.random() - 0.5) * 0.45 + worldNormal.z * 0.15,
                    spin: (Math.random() - 0.5) * 6,
                    opacity: 0.72,
                    expansion: 1.35,
                    fadePower: 1.8,
                    kind: 'energyImpactCore'
                }
            );
        }

        for (let i = 0; i < 16; i++) {
            const angle = Math.random() * Math.PI * 2;
            const lift = 0.15 + Math.random() * 0.55;
            const speed = 0.45 + Math.random() * (hitTarget ? 1.6 : 1.1);
            createSmokeSprite(
                burstCenter.x,
                burstCenter.y + 0.02,
                burstCenter.z,
                Math.random() < 0.3 ? 0xffffff : color,
                0.035 + Math.random() * 0.055,
                0.18 + Math.random() * 0.18,
                {
                    vx: Math.cos(angle) * speed + worldNormal.x * lift,
                    vy: lift + worldNormal.y * 0.35,
                    vz: Math.sin(angle) * speed + worldNormal.z * lift,
                    spin: (Math.random() - 0.5) * 10,
                    opacity: 0.9,
                    expansion: 0.5,
                    fadePower: 1.55,
                    kind: 'energyImpactSpark'
                }
            );
        }

        const flash = new THREE.PointLight(color, hitTarget ? 6.8 : 4.5, hitTarget ? 10 : 7);
        flash.position.copy(burstCenter);
        scene.add(flash);
        impactLights.push({ light: flash, life: 0.32, maxLife: 0.32, baseIntensity: hitTarget ? 6.8 : 4.5 });

        for (let ring = 0; ring < 2; ring++) {
            const material = window.shockwaveMat.clone();
            material.color.setHex(color);
            material.opacity = 0.48 - ring * 0.16;
            const shock = new THREE.Mesh(window.shockwaveGeom, material);
            shock.position.copy(burstCenter.clone().addScaledVector(worldNormal, 0.02 + ring * 0.012));
            const normalAlign = new THREE.Quaternion();
            normalAlign.setFromUnitVectors(new THREE.Vector3(0, 0, 1), worldNormal);
            shock.quaternion.copy(normalAlign);
            scene.add(shock);
            shockwaves.push({
                mesh: shock,
                life: 0.3 + ring * 0.08,
                maxLife: 0.3 + ring * 0.08,
                maxScale: (hitTarget ? 2.15 : 1.6) + ring * 0.65,
                baseOpacity: 0.44 - ring * 0.12,
                kind: 'energyImpactRing'
            });
        }
    }

    function spawnSuperExplosion(projectile, hitTarget) {
        if (!scene || !projectile || !projectile.mesh) return;
        ensureImpactAssets();
        const pos = projectile.mesh.position.clone();
        const color = projectile.color || 0x7dd3fc;
        const worldNormal = new THREE.Vector3(0, 1, 0);
        if (surface) {
            const localPos = pos.clone();
            surface.worldToLocal(localPos);
            const eps = 0.22;
            const t = globalTime;
            const left = heightAt(clamp(localPos.x - eps, -GRID.width * 0.5, GRID.width * 0.5), localPos.z, t);
            const right = heightAt(clamp(localPos.x + eps, -GRID.width * 0.5, GRID.width * 0.5), localPos.z, t);
            const back = heightAt(localPos.x, clamp(localPos.z - eps, -GRID.depth * 0.5, GRID.depth * 0.5), t);
            const front = heightAt(localPos.x, clamp(localPos.z + eps, -GRID.depth * 0.5, GRID.depth * 0.5), t);
            worldNormal.set(left - right, eps * 2, back - front).normalize().transformDirection(surface.matrixWorld);
        }

        const burstCenter = pos.clone().addScaledVector(worldNormal, 0.12);
        const coreCount = hitTarget ? 15 : 10;
        for (let i = 0; i < coreCount; i++) {
            const coreColor = i === 0 ? 0xffffff : (Math.random() < 0.3 ? 0xffffff : color);
            createSmokeSprite(
                burstCenter.x + (Math.random() - 0.5) * 0.2,
                burstCenter.y + (Math.random() - 0.5) * 0.1,
                burstCenter.z + (Math.random() - 0.5) * 0.2,
                coreColor,
                0.35 + Math.random() * 0.35,
                0.45 + Math.random() * 0.35,
                {
                    vx: (Math.random() - 0.5) * 0.95 + worldNormal.x * 0.3,
                    vy: 0.32 + Math.random() * 0.65 + worldNormal.y * 0.5,
                    vz: (Math.random() - 0.5) * 0.95 + worldNormal.z * 0.3,
                    spin: (Math.random() - 0.5) * 6,
                    opacity: 0.88,
                    expansion: 2.2,
                    fadePower: 1.5,
                    kind: 'superExplosionCore'
                }
            );
        }

        const sparkCount = hitTarget ? 38 : 26;
        for (let i = 0; i < sparkCount; i++) {
            const angle = Math.random() * Math.PI * 2;
            const lift = 0.25 + Math.random() * 0.95;
            const speed = 1.2 + Math.random() * (hitTarget ? 3.5 : 2.2);
            const sparkColor = Math.random() < 0.25 ? 0xffffff : (projectile.superType === 'rocket' ? 0xffaa00 : color);
            createSmokeSprite(
                burstCenter.x,
                burstCenter.y + 0.04,
                burstCenter.z,
                sparkColor,
                0.065 + Math.random() * 0.085,
                0.34 + Math.random() * 0.36,
                {
                    vx: Math.cos(angle) * speed + worldNormal.x * lift,
                    vy: lift + worldNormal.y * 0.5,
                    vz: Math.sin(angle) * speed + worldNormal.z * lift,
                    spin: (Math.random() - 0.5) * 12,
                    opacity: 0.95,
                    expansion: 0.45,
                    fadePower: 1.65,
                    kind: 'superExplosionSpark'
                }
            );
        }

        const flashIntensity = hitTarget ? 24.0 : 15.0;
        const flashDistance = hitTarget ? 22.0 : 16.0;
        const flash = new THREE.PointLight(color, flashIntensity, flashDistance);
        flash.position.copy(burstCenter);
        scene.add(flash);
        impactLights.push({ light: flash, life: 0.55, maxLife: 0.55, baseIntensity: flashIntensity });

        const ringCount = hitTarget ? 4 : 3;
        for (let ring = 0; ring < ringCount; ring++) {
            const material = window.shockwaveMat.clone();
            material.color.setHex(color);
            material.opacity = 0.65 - ring * 0.15;
            const shock = new THREE.Mesh(window.shockwaveGeom, material);
            shock.position.copy(burstCenter.clone().addScaledVector(worldNormal, 0.03 + ring * 0.022));
            const normalAlign = new THREE.Quaternion();
            normalAlign.setFromUnitVectors(new THREE.Vector3(0, 0, 1), worldNormal);
            shock.quaternion.copy(normalAlign);
            scene.add(shock);
            shockwaves.push({
                mesh: shock,
                life: 0.45 + ring * 0.12,
                maxLife: 0.45 + ring * 0.12,
                maxScale: (hitTarget ? 5.2 : 3.8) + ring * 1.5,
                baseOpacity: 0.65 - ring * 0.12,
                kind: 'superExplosionRing'
            });
        }
    }

    function explodeEnergyProjectile(projectile, hitTarget) {
        const pos = projectile.mesh.position.clone();
        const isSuper = projectile.isSuper;
        if (surface) {
            const local = pos.clone();
            surface.worldToLocal(local);
            const waveStrength = isSuper ? 1.45 : (hitTarget ? 0.34 : 0.18);
            addImpulse(clamp(local.x, -robotBounds.x, robotBounds.x), clamp(local.z, -robotBounds.z, robotBounds.z), waveStrength);
        }
        if (hitTarget) applyRobotHitRecoil(projectile, pos);
        if (isSuper) {
            spawnSuperExplosion(projectile, hitTarget);
        } else {
            spawnEnergyExplosion(projectile, hitTarget);
        }
        const shakeVal = isSuper ? (hitTarget ? 0.38 : 0.22) : (hitTarget ? 0.095 : 0.05);
        cameraShake = Math.max(cameraShake, shakeVal);
        disposeEnergyProjectile(projectile);
    }

    function updateEnergyProjectiles(dt, t) {
        for (let i = energyProjectiles.length - 1; i >= 0; i--) {
            const projectile = energyProjectiles[i];
            projectile.life -= dt;

            const isSuper = projectile.isSuper;
            const superType = projectile.superType;

            if (isSuper) {
                if (superType === 'grenade') {
                    const g = 4.5;
                    projectile.velocity3D.y -= g * dt;
                    projectile.mesh.position.addScaledVector(projectile.velocity3D, dt);
                    if (projectile.velocity3D.lengthSq() > 0.001) {
                        projectile.direction.copy(projectile.velocity3D).normalize();
                    }
                } else if (superType === 'rocket') {
                    const targetPosition = robotAimPoint(projectile.target);
                    projectile.currentSpeed = Math.min(13.5, projectile.currentSpeed + dt * 10.0);
                    const toTarget = targetPosition.clone().sub(projectile.mesh.position);
                    if (toTarget.lengthSq() > 0.001) {
                        projectile.direction.lerp(toTarget.normalize(), clamp(dt * 5.5, 0, 0.65)).normalize();
                    }
                    projectile.mesh.position.addScaledVector(projectile.direction, projectile.currentSpeed * dt);
                    const right = new THREE.Vector3(0, 1, 0).cross(projectile.direction).normalize();
                    const up = projectile.direction.clone().cross(right).normalize();
                    const spiralFreq = 16.0;
                    const spiralAmp = 0.08 * (1.0 + (projectile.maxLife - projectile.life) * 0.5);
                    const spiralOffset = right.clone().multiplyScalar(Math.sin(t * spiralFreq) * spiralAmp)
                                      .add(up.clone().multiplyScalar(Math.cos(t * spiralFreq) * spiralAmp));
                    projectile.mesh.position.addScaledVector(spiralOffset, dt * 12.0);
                    projectile.mesh.quaternion.setFromUnitVectors(new THREE.Vector3(0, 0, 1), projectile.direction);
                }
            } else {
                if (projectile.target && projectile.target.group) {
                    const targetPosition = robotAimPoint(projectile.target);
                    const toTarget = targetPosition.sub(projectile.mesh.position);
                    if (toTarget.lengthSq() > 0.001) {
                        projectile.direction.lerp(toTarget.normalize(), clamp(dt * 2.2, 0, 0.32)).normalize();
                    }
                }
                projectile.mesh.position.addScaledVector(projectile.direction, projectile.speed * dt);
            }

            const pulse = 0.88 + Math.sin(t * 26 + projectile.pulseSeed) * 0.24;
            projectile.mesh.scale.setScalar(pulse);
            if (projectile.projectileLight) {
                projectile.projectileLight.intensity = isSuper ? (8.0 + pulse * 4.0) : (3.8 + pulse * 2.8);
            }

            if (active) {
                const spawnChance = isSuper ? 0.98 : 0.86;
                if (Math.random() < spawnChance) {
                    let pColor = projectile.color;
                    let pScale = 0.08 + Math.random() * 0.06;
                    let pLife = 0.28 + Math.random() * 0.18;
                    let pVx = -projectile.direction.x * 0.22 + (Math.random() - 0.5) * 0.15;
                    let pVy = (Math.random() - 0.5) * 0.08;
                    let pVz = -projectile.direction.z * 0.22 + (Math.random() - 0.5) * 0.15;
                    let pExpansion = 1.9;

                    if (isSuper) {
                        pScale = 0.16 + Math.random() * 0.12;
                        pLife = 0.45 + Math.random() * 0.35;
                        pExpansion = 2.8;

                        if (superType === 'grenade') {
                            pColor = Math.random() < 0.52 ? 0x00d2ff : projectile.color;
                            pVx = -projectile.direction.x * 0.35 + (Math.random() - 0.5) * 0.3;
                            pVy = -projectile.direction.y * 0.35 + 0.12 + (Math.random() - 0.5) * 0.3;
                            pVz = -projectile.direction.z * 0.35 + (Math.random() - 0.5) * 0.3;
                        } else if (superType === 'rocket') {
                            pColor = Math.random() < 0.44 ? 0xffdd00 : (Math.random() < 0.62 ? 0xff4400 : 0xff9900);
                            pScale = 0.12 + Math.random() * 0.1;
                            pLife = 0.34 + Math.random() * 0.22;
                            pVx = -projectile.direction.x * 3.2 + (Math.random() - 0.5) * 0.4;
                            pVy = -projectile.direction.y * 3.2 + (Math.random() - 0.5) * 0.4;
                            pVz = -projectile.direction.z * 3.2 + (Math.random() - 0.5) * 0.4;
                        }
                    }

                    createSmokeSprite(
                        projectile.mesh.position.x - projectile.direction.x * 0.12,
                        projectile.mesh.position.y,
                        projectile.mesh.position.z - projectile.direction.z * 0.12,
                        pColor,
                        pScale,
                        pLife,
                        {
                            vx: pVx,
                            vy: pVy,
                            vz: pVz,
                            spin: (Math.random() - 0.5) * 9,
                            opacity: isSuper ? 0.96 : 0.9,
                            expansion: pExpansion,
                            fadePower: isSuper ? 1.25 : 1.35,
                            kind: isSuper ? 'superTrail' : 'energyProjectile'
                        }
                    );
                }
            }

            let hit = projectile.life <= 0;
            let hitTarget = false;

            if (isSuper && superType === 'grenade') {
                const surfaceY = surface ? surface.position.y : 0;
                const waterH = surfaceY + heightAt(projectile.mesh.position.x, projectile.mesh.position.z, t);
                if (projectile.mesh.position.y <= waterH) {
                    hit = true;
                    if (projectile.target && projectile.target.group) {
                        const distToTarget = projectile.mesh.position.distanceTo(robotAimPoint(projectile.target));
                        if (distToTarget < 1.35) {
                            hitTarget = true;
                        }
                    }
                }
            }

            if (!hit && projectile.target && projectile.target.group) {
                hitTarget = projectile.mesh.position.distanceTo(robotAimPoint(projectile.target)) < 0.72;
                hit = hitTarget;
            }
            if (hit) {
                explodeEnergyProjectile(projectile, hitTarget);
                energyProjectiles.splice(i, 1);
            }
        }
    }

    function clearEnergyProjectiles() {
        while (energyProjectiles.length) {
            disposeEnergyProjectile(energyProjectiles.pop());
        }
    }

    function updateRobotDuel(dt, t) {
        if (robotFleet.length < 2) return;
        const blueRobot = robotFleet[0];
        const redRobot = robotFleet[1];
        if (!blueRobot.group || !redRobot.group) return;

        const dx = redRobot.state.x - blueRobot.state.x;
        const dz = redRobot.state.z - blueRobot.state.z;
        const distance = Math.hypot(dx, dz);

        const ROBOT_SUPER_DISTANCE = 8.85;

        const blueAimTime = blueRobot.state.isSuperweaponCharging ? 0.85 : 0.42;
        if (blueRobot.state.isAiming && t - blueRobot.state.aimStart > blueAimTime) {
            const toOpponent = new THREE.Vector3(dx, 0, dz).normalize();
            if (blueRobot.forward.dot(toOpponent) > 0.96) {
                spawnEnergyProjectile(blueRobot, redRobot, t, blueRobot.state.isSuperweaponCharging);
                blueRobot.state.isAiming = false;
                blueRobot.state.isSuperweaponCharging = false;
            }
        }
        const redAimTime = redRobot.state.isSuperweaponCharging ? 0.85 : 0.42;
        if (redRobot.state.isAiming && t - redRobot.state.aimStart > redAimTime) {
            const toOpponent = new THREE.Vector3(-dx, 0, -dz).normalize();
            if (redRobot.forward.dot(toOpponent) > 0.96) {
                spawnEnergyProjectile(redRobot, blueRobot, t, redRobot.state.isSuperweaponCharging);
                redRobot.state.isAiming = false;
                redRobot.state.isSuperweaponCharging = false;
            }
        }

        if (distance > ROBOT_SUPER_DISTANCE) {
            blueRobot.state.isAiming = false;
            blueRobot.state.isSuperweaponCharging = false;
            redRobot.state.isAiming = false;
            redRobot.state.isSuperweaponCharging = false;
            return;
        }

        const isFar = distance > ROBOT_DUEL_DISTANCE;

        // Reset non-superweapon aiming if we drifted too far
        if (isFar) {
            if (blueRobot.state.isAiming && !blueRobot.state.isSuperweaponCharging) {
                blueRobot.state.isAiming = false;
            }
            if (redRobot.state.isAiming && !redRobot.state.isSuperweaponCharging) {
                redRobot.state.isAiming = false;
            }
        }

        const proximity = 1 - clamp(distance / ROBOT_DUEL_DISTANCE, 0, 1);
        const blueCooldown = ROBOT_DUEL_COOLDOWN + Math.sin(t * 0.8) * 0.1;
        const redCooldown = ROBOT_DUEL_COOLDOWN * 1.12 + Math.cos(t * 0.7) * 0.08;

        if (t - blueRobot.state.lastShot > blueCooldown && !blueRobot.state.isAiming) {
            const superWeaponReady = (t - blueRobot.state.lastSuperweaponAt > 15.0);
            if (superWeaponReady) {
                blueRobot.state.isAiming = true;
                blueRobot.state.aimStart = t;
                blueRobot.state.isSuperweaponCharging = true;
            } else if (!isFar) {
                blueRobot.state.isAiming = true;
                blueRobot.state.aimStart = t;
            }
        }
        if (t - redRobot.state.lastShot > redCooldown && !redRobot.state.isAiming) {
            const superWeaponReady = (t - redRobot.state.lastSuperweaponAt > 15.0);
            if (superWeaponReady) {
                redRobot.state.isAiming = true;
                redRobot.state.aimStart = t;
                redRobot.state.isSuperweaponCharging = true;
            } else if (!isFar) {
                redRobot.state.isAiming = true;
                redRobot.state.aimStart = t;
            }
        }

        if (!updateRobotDuel.lastPulse || t - updateRobotDuel.lastPulse > 0.35) {
            const midX = (blueRobot.state.x + redRobot.state.x) * 0.5;
            const midZ = (blueRobot.state.z + redRobot.state.z) * 0.5;
            addImpulse(midX, midZ, 0.035 + proximity * 0.07);
            updateRobotDuel.lastPulse = t;
        }
    }

    function updateFloatingRobot(dt, t) {
        ensureRobotPhysicsState();
        loadFloatingRobot();
        // Phase 1: Update all robot positions first so heightAt sees a consistent snapshot
        robotFleet.forEach(function (bot, index) {
            updateRobotPositionPhase(bot, dt, t, index);
        });
        // Phase 2: Compute heights, visuals and rotations with synchronized positions
        robotFleet.forEach(function (bot, index) {
            updateRobotVisualsPhase(bot, dt, t, index);
        });
        updateRobotThrusterRipples(t);
        updateRobotDuel(dt, t);
        updateEnergyProjectiles(dt, t);
        aliasPrimaryRobot(robotFleet[0]);
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
        surface.receiveShadow = true;
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
        colorAccentNext = new THREE.Color(0x7dd3fc);

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
        renderer.shadowMap.enabled = true;
        renderer.shadowMap.type = THREE.PCFSoftShadowMap;

        const ambient = new THREE.HemisphereLight(0xbfd7ff, 0x0b0f1a, 1.05);
        scene.add(ambient);

        const key = new THREE.DirectionalLight(0xb9d9ff, 1.35);
        key.position.set(-5, 8, 2);
        key.castShadow = true;
        key.shadow.mapSize.width = 1024;
        key.shadow.mapSize.height = 1024;
        key.shadow.camera.near = 0.5;
        key.shadow.camera.far = 30;
        key.shadow.camera.left = -15;
        key.shadow.camera.right = 15;
        key.shadow.camera.top = 15;
        key.shadow.camera.bottom = -15;
        key.shadow.bias = -0.0006;
        scene.add(key);

        key.target.position.set(0, -2.55, -5.3);
        scene.add(key.target);

        const rim = new THREE.DirectionalLight(0x7dd3fc, 0.55);
        rim.position.set(5, 3, -8);
        scene.add(rim);

        createHeightfield();
        ensureRobotPhysicsState();
        loadFloatingRobot();

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

    // Module-scoped scratch objects to avoid per-frame GC allocation
    const _colorScratch = typeof THREE !== 'undefined' ? new THREE.Color() : null;
    const _scratchVec3 = typeof THREE !== 'undefined' ? new THREE.Vector3() : null;
    const _scratchEuler = typeof THREE !== 'undefined' ? new THREE.Euler(0, 0, 0, 'XYZ') : null;

    function updateSurface(t) {
        if (!surfaceGeometry || !gridGeometry) return;

        // Pre-compute impulse invariants once per frame (Fix 3)
        const pre = _preImpulse;
        let alive = 0;
        for (let i = 0; i < impulses.length; i++) {
            const age = t - impulses[i].start;
            if (age < 0 || age > IMPULSE_LIFETIME) continue;
            const fade = Math.pow(1 - age / IMPULSE_LIFETIME, 1.4);
            if (fade < IMPULSE_FADE_CUTOFF) continue;
            // Compact alive impulses to front of arrays
            if (alive !== i && alive < impulses.length) {
                // We don't reorder the impulses array itself — just store indices into pre arrays
            }
            // Ensure typed arrays are large enough
            if (alive >= pre.ages.length) {
                const newLen = pre.ages.length * 2;
                const newAges = new Float64Array(newLen);
                newAges.set(pre.ages);
                pre.ages = newAges;
                const newFades = new Float64Array(newLen);
                newFades.set(pre.fades);
                pre.fades = newFades;
                const newStr = new Float64Array(newLen);
                newStr.set(pre.strengths);
                pre.strengths = newStr;
                const newRad = new Float64Array(newLen);
                newRad.set(pre.radii);
                pre.radii = newRad;
            }
            pre.ages[alive] = age;
            pre.fades[alive] = fade;
            pre.strengths[alive] = softClip(impulses[i].strength, 1.08);
            pre.radii[alive] = age * 3.05;
            // Move alive impulse to front of impulses array for contiguous access
            if (alive !== i) {
                const tmp = impulses[alive];
                impulses[alive] = impulses[i];
                impulses[i] = tmp;
            }
            alive++;
        }
        pre.alive = alive;

        const position = surfaceGeometry.getAttribute('position');
        const color = surfaceGeometry.getAttribute('color');
        const colorScratch = _colorScratch || new THREE.Color();

        // Build height cache indexed by vertex index for grid reuse (Fix 2)
        const vertexCount = position.count;
        if (!heightCache || heightCache.length < vertexCount) {
            heightCache = new Float32Array(vertexCount);
        }

        for (let i = 0; i < vertexCount; i++) {
            const x = basePositions[i * 3];
            const z = basePositions[i * 3 + 2];
            const height = heightAt(x, z, t);

            heightCache[i] = height;
            position.array[i * 3 + 1] = height;
            colorForHeight(height, colorScratch);
            if (currentMode === 3 || previousMode === 3) {
                colorOverrideForPosition(x, z, height, t, colorScratch);
            }
            if (currentMode === 4 || previousMode === 4) {
                colorVortexForPosition(x, z, height, t, colorScratch);
            }
            if (currentMode === 0 || previousMode === 0) colorTextForPosition(x, z, height, t, colorScratch);
            color.array[i * 3] = colorScratch.r;
            color.array[i * 3 + 1] = colorScratch.g;
            color.array[i * 3 + 2] = colorScratch.b;
        }

        // Clear pre-computed flag so isolated heightAt calls use fallback path
        pre.alive = 0;

        position.needsUpdate = true;
        color.needsUpdate = true;
        // Throttle normal recomputation to every 2nd frame (Fix 6)
        normalFrameToggle++;
        if (normalFrameToggle & 1) {
            surfaceGeometry.computeVertexNormals();
            surfaceGeometry.attributes.normal.needsUpdate = true;
        }

        // Reuse cached surface heights for grid line vertices (Fix 2)
        // Grid vertices lie on the same grid as the surface mesh, so we
        // look up heights from the cache by mapping (x, z) → vertex index.
        const gridPosition = gridGeometry.getAttribute('position');
        const gridArray = gridPosition.array;
        const stepX = GRID.width / GRID.cols;
        const stepZ = GRID.depth / GRID.rows;
        const halfW = GRID.width * 0.5;
        const halfD = GRID.depth * 0.5;
        const invStepX = 1 / stepX;
        const invStepZ = 1 / stepZ;
        const colsP1 = GRID.cols + 1;
        for (let i = 0; i < gridArray.length; i += 3) {
            const gx = gridArray[i];
            const gz = gridArray[i + 2];
            // Map grid position to surface vertex col/row indices
            const col = Math.round((gx + halfW) * invStepX);
            const row = Math.round((gz + halfD) * invStepZ);
            const vi = row * colsP1 + col;
            if (vi >= 0 && vi < vertexCount) {
                gridArray[i + 1] = heightCache[vi] + 0.012;
            } else {
                gridArray[i + 1] = 0.012;
            }
        }
        gridPosition.needsUpdate = true;
    }

    function render(time) {
        if (!active) return;
        animationId = requestAnimationFrame(render);

        if (lastFrame > 0 && time - lastFrame < FRAME_INTERVAL) return;

        // Use actual elapsed wall-clock time for smooth animation
        // regardless of monitor refresh rate
        const elapsed = lastFrame > 0 ? (time - lastFrame) : FRAME_INTERVAL;
        lastFrame = time;

        // Clamp to prevent physics explosion after tab-switch or stall
        const dt = Math.min(elapsed * 0.001, 0.1);
        globalTime += dt;
        const t = globalTime;
        updateMode(dt, t);
        if (currentMode === 2) {
            spawnImpulse(t, false);
        }
        pruneImpulses(t);
        updateFloatingRobot(dt, t);
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
        globalTime = 0;
        canvas.style.display = 'block';
        currentMode = 0;
        resetModeTransition(globalTime);
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
        clearEnergyProjectiles();
        robotThrusterRipples.length = 0;
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
        },
        debugState: function () {
            const duelDistance = robotFleet.length >= 2
                ? Math.hypot(robotFleet[0].state.x - robotFleet[1].state.x, robotFleet[0].state.z - robotFleet[1].state.z)
                : null;
            return {
                active,
                robotCount: robotFleet.length,
                loadedRobots: robotFleet.filter(function (bot) { return bot.loaded; }).length,
                energyProjectiles: energyProjectiles.length,
                thrusterRipples: robotThrusterRipples.length,
                duelDistance,
                robotHits: robotFleet.map(function (bot) { return bot.state.hits || 0; }),
                robotFlightLift: robotFleet.map(function (bot) { return bot.state.flightLift || 0; })
            };
        }
    };
})();
