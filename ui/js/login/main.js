// AuraGo – login page logic
// Extracted from login.html

        // ═══════════════════════════════════════════════════════════════
        // EPIC WEBGL BACKGROUND - Neural Network Particle System
        // ═══════════════════════════════════════════════════════════════
        
        const canvas = document.getElementById('bg-canvas');
        const cssFallback = document.getElementById('css-bg');
        
        const NETWORK_NODE_COUNT = 72;
        const MAX_NODE_CONNECTIONS = 3;
        const MAX_CONNECTION_DISTANCE = 28;
        const MAX_IMPULSES = 56;
        const IMPULSE_TRAIL_SEGMENTS = 10;
        const MIN_IMPULSE_ENERGY = 0.08;

        let scene, camera, renderer, particles, lines, impulseTrails, impulseHeads;
        let nodeBasePositions, nodeCurrentPositions, nodeBaseSizes, nodeFlashLevels, nodeColorMixes, nodeSeeds;
        let networkEdges = [];
        let nodeNeighbors = [];
        let impulses = [];
        let activePalette = null;
        let ambientPulseTimer = 0;
        let lastFrameTime = 0;
        let mouseX = 0, mouseY = 0;
        let time = 0;

        function showCSSFallback() {
            canvas.classList.add('is-hidden');
            cssFallback.classList.remove('is-hidden');
        }

        function showWebGL() {
            canvas.classList.remove('is-hidden');
            cssFallback.classList.add('is-hidden');
        }
        
        // Check WebGL support
        function webglAvailable() {
            try {
                const testCanvas = document.createElement('canvas');
                return !!(window.WebGLRenderingContext && 
                    (testCanvas.getContext('webgl') || testCanvas.getContext('experimental-webgl')));
            } catch(e) {
                return false;
            }
        }
        
        // Initialize after Three.js is loaded
        function initBackground() {
            if (webglAvailable() && typeof THREE !== 'undefined' && !window._threeLoadFailed) {
                try {
                    initWebGL();
                } catch(e) {
                    console.warn('[AuraGo] WebGL init failed, using CSS fallback:', e);
                    showCSSFallback();
                }
            } else {
                // Show CSS fallback (Three.js unavailable or WebGL not supported)
                showCSSFallback();
            }
        }
        
        // Wait for Three.js to load
        if (document.readyState === 'complete') {
            initBackground();
        } else {
            window.addEventListener('load', initBackground);
        }

        // ── i18n: populate text ──
        (function applyI18N() {
            const el = id => document.getElementById(id);
            el('loginSubtitle').textContent       = t('login.subtitle');
            el('lblPassword').textContent          = t('login.password_label');
            el('eyeBtn').title                     = t('login.password_show');
            el('totpDivider').textContent           = t('login.totp_divider');
            el('lblTotp').textContent               = t('login.totp_label');
            el('btnText').textContent               = t('login.btn_submit');
            if (typeof TOTP_ENABLED !== 'undefined' && TOTP_ENABLED) el('totpSection').classList.remove('is-hidden');
        })();

        function getCurrentTheme() {
            return document.documentElement.getAttribute('data-theme') || 'dark';
        }

        function getWebGLTheme(theme) {
            if (theme === 'light') {
                return {
                    clearColor: 0xd8e4df,
                    fogColor: 0xd8e4df,
                    particleStart: 0x0f766e,
                    particleEnd: 0x0d9488,
                    nodeFlash: 0xecfeff,
                    lineColor: 0x0f766e,
                    trailColor: 0x22d3ee,
                    pulseColor: 0x5eead4,
                    orbColors: [0x5eead4, 0x2dd4bf, 0x115e59]
                };
            }
            return {
                clearColor: 0x0b0f1a,
                fogColor: 0x0b0f1a,
                particleStart: 0x2dd4bf,
                particleEnd: 0x0d9488,
                nodeFlash: 0xccfbf1,
                lineColor: 0x2dd4bf,
                trailColor: 0x67e8f9,
                pulseColor: 0x99f6e4,
                orbColors: [0x2dd4bf, 0x0d9488, 0x115e59]
            };
        }

        function resolveWebGLPalette(theme) {
            const raw = getWebGLTheme(theme);
            return {
                raw,
                nodeStart: new THREE.Color(raw.particleStart),
                nodeEnd: new THREE.Color(raw.particleEnd),
                nodeFlash: new THREE.Color(raw.nodeFlash),
                line: new THREE.Color(raw.lineColor),
                trail: new THREE.Color(raw.trailColor),
                pulse: new THREE.Color(raw.pulseColor)
            };
        }

        function lerp(a, b, t) {
            return a + (b - a) * t;
        }

        function applyThemeToWebGL(theme) {
            if (!renderer || !scene) return;
            activePalette = resolveWebGLPalette(theme);
            const palette = activePalette.raw;

            renderer.setClearColor(palette.clearColor, 1);
            if (scene.fog) scene.fog.color.setHex(palette.fogColor);

            scene.children.forEach(child => {
                if (
                    child.userData &&
                    typeof child.userData.orbIndex === 'number' &&
                    child.material &&
                    child.material.color
                ) {
                    const orbColor = palette.orbColors[child.userData.orbIndex] || palette.orbColors[0];
                    child.material.color.setHex(orbColor);
                    child.material.needsUpdate = true;
                }
            });

            if (particles && lines && impulseTrails && impulseHeads) {
                updateNodeVisuals();
                updateConnectionField();
                updateImpulseField(0);
            }
        }

        window.addEventListener('aurago:themechange', (event) => {
            applyThemeToWebGL((event.detail && event.detail.theme) || getCurrentTheme());
        });
        
        function initWebGL() {
            activePalette = resolveWebGLPalette(getCurrentTheme());
            const palette = activePalette.raw;

            // Scene setup
            scene = new THREE.Scene();
            scene.fog = new THREE.FogExp2(palette.fogColor, 0.001);
            
            // Camera
            camera = new THREE.PerspectiveCamera(75, window.innerWidth / window.innerHeight, 0.1, 1000);
            camera.position.z = 50;
            
            // Renderer
            renderer = new THREE.WebGLRenderer({ 
                canvas: canvas, 
                alpha: true, 
                antialias: true 
            });
            renderer.setSize(window.innerWidth, window.innerHeight);
            renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
            renderer.setClearColor(palette.clearColor, 1);
            
            // Create particle system
            createParticles();
            createConnections();
            createImpulseEffects();
            createFloatingOrbs();
            updateNodePositions(0);
            updateNodeVisuals();
            updateConnectionField();
            triggerNode(Math.floor(Math.random() * NETWORK_NODE_COUNT), 0.95);
            updateImpulseField(0);
            applyThemeToWebGL(getCurrentTheme());

            // WebGL ready — show canvas, hide CSS fallback
            showWebGL();
            
            // Mouse tracking
            document.addEventListener('mousemove', onMouseMove, false);
            window.addEventListener('resize', onWindowResize, false);
            
            // Start animation
            lastFrameTime = 0;
            animate();
        }
        
        function createParticles() {
            const particleCount = NETWORK_NODE_COUNT;
            const geometry = new THREE.BufferGeometry();
            const positions = new Float32Array(particleCount * 3);
            const colors = new Float32Array(particleCount * 3);
            const sizes = new Float32Array(particleCount);
            const palette = activePalette || resolveWebGLPalette(getCurrentTheme());
            nodeBasePositions = new Float32Array(particleCount * 3);
            nodeBaseSizes = new Float32Array(particleCount);
            nodeFlashLevels = new Float32Array(particleCount);
            nodeColorMixes = new Float32Array(particleCount);
            nodeSeeds = new Float32Array(particleCount);
            
            for (let i = 0; i < particleCount; i++) {
                const angle = Math.random() * Math.PI * 2;
                const radius = 12 + Math.sqrt(Math.random()) * 34;
                const x = Math.cos(angle) * radius + (Math.random() - 0.5) * 16;
                const y = (Math.random() - 0.5) * 56;
                const z = (Math.random() - 0.5) * 28;
                const mixRatio = Math.random();
                const baseColor = palette.nodeStart.clone().lerp(palette.nodeEnd, mixRatio);

                nodeBasePositions[i * 3] = x;
                nodeBasePositions[i * 3 + 1] = y;
                nodeBasePositions[i * 3 + 2] = z;

                positions[i * 3] = x;
                positions[i * 3 + 1] = y;
                positions[i * 3 + 2] = z;
                colors[i * 3] = baseColor.r;
                colors[i * 3 + 1] = baseColor.g;
                colors[i * 3 + 2] = baseColor.b;

                nodeBaseSizes[i] = 1.7 + Math.random() * 1.5;
                sizes[i] = nodeBaseSizes[i];
                nodeColorMixes[i] = mixRatio;
                nodeSeeds[i] = Math.random() * Math.PI * 2;
            }
            
            geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
            geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));
            geometry.setAttribute('size', new THREE.BufferAttribute(sizes, 1));
            nodeCurrentPositions = geometry.attributes.position.array;
            
            const material = new THREE.ShaderMaterial({
                uniforms: {
                    time: { value: 0 }
                },
                vertexShader: `
                    attribute float size;
                    attribute vec3 color;
                    varying vec3 vColor;
                    uniform float time;
                    
                    void main() {
                        vColor = color;
                        vec3 pos = position;
                        
                        // Subtle breathing so the network feels alive.
                        pos.z += sin(time * 0.45 + position.x * 0.08) * 0.35;
                        
                        vec4 mvPosition = modelViewMatrix * vec4(pos, 1.0);
                        gl_PointSize = size * (300.0 / -mvPosition.z);
                        gl_Position = projectionMatrix * mvPosition;
                    }
                `,
                fragmentShader: `
                    varying vec3 vColor;
                    
                    void main() {
                        // Create circular glow
                        vec2 center = gl_PointCoord - vec2(0.5);
                        float dist = length(center);
                        float alpha = 1.0 - smoothstep(0.0, 0.5, dist);
                        
                        // Core brightness
                        float core = 1.0 - smoothstep(0.0, 0.2, dist);
                        
                        gl_FragColor = vec4(vColor * (0.8 + core * 0.4), alpha * 0.8);
                    }
                `,
                transparent: true,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });
            
            particles = new THREE.Points(geometry, material);
            scene.add(particles);
        }
        
        function createConnections() {
            networkEdges = [];
            nodeNeighbors = Array.from({ length: NETWORK_NODE_COUNT }, () => []);
            const edgeKeys = new Set();

            for (let i = 0; i < NETWORK_NODE_COUNT; i++) {
                const candidates = [];
                const iOffset = i * 3;
                const ix = nodeBasePositions[iOffset];
                const iy = nodeBasePositions[iOffset + 1];
                const iz = nodeBasePositions[iOffset + 2];

                for (let j = 0; j < NETWORK_NODE_COUNT; j++) {
                    if (i === j) continue;
                    const jOffset = j * 3;
                    const dx = ix - nodeBasePositions[jOffset];
                    const dy = iy - nodeBasePositions[jOffset + 1];
                    const dz = iz - nodeBasePositions[jOffset + 2];
                    const distance = Math.sqrt(dx * dx + dy * dy + dz * dz);
                    if (distance <= MAX_CONNECTION_DISTANCE) {
                        candidates.push({ node: j, distance });
                    }
                }

                candidates.sort((a, b) => a.distance - b.distance);

                for (let k = 0; k < candidates.length && nodeNeighbors[i].length < MAX_NODE_CONNECTIONS; k++) {
                    const target = candidates[k].node;
                    if (nodeNeighbors[target].length >= MAX_NODE_CONNECTIONS) continue;
                    const edgeKey = i < target ? `${i}-${target}` : `${target}-${i}`;
                    if (edgeKeys.has(edgeKey)) continue;

                    edgeKeys.add(edgeKey);
                    networkEdges.push({ a: i, b: target, distance: candidates[k].distance });
                    nodeNeighbors[i].push(target);
                    nodeNeighbors[target].push(i);
                }
            }

            const lineGeometry = new THREE.BufferGeometry();
            const linePositions = new Float32Array(networkEdges.length * 2 * 3);
            const lineColors = new Float32Array(networkEdges.length * 2 * 3);
            lineGeometry.setAttribute('position', new THREE.BufferAttribute(linePositions, 3));
            lineGeometry.setAttribute('color', new THREE.BufferAttribute(lineColors, 3));
            lineGeometry.setDrawRange(0, networkEdges.length * 2);

            const lineMaterial = new THREE.LineBasicMaterial({
                vertexColors: true,
                transparent: true,
                opacity: 0.42,
                blending: THREE.AdditiveBlending,
                depthWrite: false
            });

            lines = new THREE.LineSegments(lineGeometry, lineMaterial);
            lines.userData = { positions: linePositions, colors: lineColors };
            scene.add(lines);
        }

        function createImpulseEffects() {
            const trailVertexCount = MAX_IMPULSES * IMPULSE_TRAIL_SEGMENTS * 2;
            const trailPositions = new Float32Array(trailVertexCount * 3);
            const trailColors = new Float32Array(trailVertexCount * 3);
            const trailGeometry = new THREE.BufferGeometry();
            trailGeometry.setAttribute('position', new THREE.BufferAttribute(trailPositions, 3));
            trailGeometry.setAttribute('color', new THREE.BufferAttribute(trailColors, 3));
            trailGeometry.setDrawRange(0, 0);

            const trailMaterial = new THREE.LineBasicMaterial({
                vertexColors: true,
                transparent: true,
                opacity: 0.96,
                blending: THREE.AdditiveBlending,
                depthWrite: false,
                depthTest: false
            });

            impulseTrails = new THREE.LineSegments(trailGeometry, trailMaterial);
            impulseTrails.userData = { positions: trailPositions, colors: trailColors };
            scene.add(impulseTrails);

            const headPositions = new Float32Array(MAX_IMPULSES * 3);
            const headColors = new Float32Array(MAX_IMPULSES * 3);
            const headSizes = new Float32Array(MAX_IMPULSES);
            const headAlpha = new Float32Array(MAX_IMPULSES);
            const headGeometry = new THREE.BufferGeometry();
            headGeometry.setAttribute('position', new THREE.BufferAttribute(headPositions, 3));
            headGeometry.setAttribute('color', new THREE.BufferAttribute(headColors, 3));
            headGeometry.setAttribute('size', new THREE.BufferAttribute(headSizes, 1));
            headGeometry.setAttribute('alpha', new THREE.BufferAttribute(headAlpha, 1));

            const headMaterial = new THREE.ShaderMaterial({
                vertexShader: `
                    attribute float size;
                    attribute vec3 color;
                    attribute float alpha;
                    varying vec3 vColor;
                    varying float vAlpha;
                    
                    void main() {
                        vColor = color;
                        vAlpha = alpha;
                        vec4 mvPosition = modelViewMatrix * vec4(position, 1.0);
                        gl_PointSize = size * (300.0 / -mvPosition.z);
                        gl_Position = projectionMatrix * mvPosition;
                    }
                `,
                fragmentShader: `
                    varying vec3 vColor;
                    varying float vAlpha;
                    
                    void main() {
                        vec2 center = gl_PointCoord - vec2(0.5);
                        float dist = length(center);
                        float glow = 1.0 - smoothstep(0.0, 0.5, dist);
                        float core = 1.0 - smoothstep(0.0, 0.08, dist);
                        float ring = smoothstep(0.07, 0.2, dist) * (1.0 - smoothstep(0.22, 0.46, dist));
                        float sparkle = smoothstep(0.0, 0.12, 1.0 - abs(center.x * center.y * 6.0));
                        float alpha = (glow * 0.28 + ring * 0.46 + core * 0.92 + sparkle * core * 0.22) * vAlpha;
                        vec3 color = vColor * (0.88 + ring * 0.42 + core * 0.75);

                        gl_FragColor = vec4(color, alpha);
                    }
                `,
                transparent: true,
                blending: THREE.AdditiveBlending,
                depthWrite: false,
                depthTest: false
            });

            impulseHeads = new THREE.Points(headGeometry, headMaterial);
            impulseHeads.userData = {
                positions: headPositions,
                colors: headColors,
                sizes: headSizes,
                alpha: headAlpha
            };
            scene.add(impulseHeads);
        }
        
        function createFloatingOrbs() {
            // Large glowing orbs in background
            const orbGeometry = new THREE.SphereGeometry(1, 32, 32);
            const palette = getWebGLTheme(getCurrentTheme());

            const orbPositions = [
                { x: -30, y: 20, z: -20, scale: 8, color: palette.orbColors[0] },
                { x: 35, y: -15, z: -30, scale: 6, color: palette.orbColors[1] },
                { x: 0, y: -25, z: -10, scale: 10, color: palette.orbColors[2] }
            ];
            
            orbPositions.forEach((orb, i) => {
                const material = new THREE.MeshBasicMaterial({
                    color: orb.color,
                    transparent: true,
                    opacity: 0.08,
                    blending: THREE.AdditiveBlending
                });
                
                const mesh = new THREE.Mesh(orbGeometry, material);
                mesh.position.set(orb.x, orb.y, orb.z);
                mesh.scale.setScalar(orb.scale);
                mesh.userData = { 
                    originalPos: { ...orb },
                    phase: i * Math.PI / 3,
                    orbIndex: i
                };
                
                scene.add(mesh);
                
                // Add glow halo
                const haloMaterial = new THREE.MeshBasicMaterial({
                    color: orb.color,
                    transparent: true,
                    opacity: 0.03,
                    blending: THREE.AdditiveBlending
                });
                const halo = new THREE.Mesh(orbGeometry, haloMaterial);
                halo.position.copy(mesh.position);
                halo.scale.setScalar(orb.scale * 1.5);
                halo.userData = {
                    originalPos: { ...orb },
                    phase: i * Math.PI / 3,
                    orbIndex: i
                };
                scene.add(halo);
            });
        }
        
        function updateNodePositions(delta) {
            if (!particles) return;
            const positions = particles.geometry.attributes.position.array;

            for (let i = 0; i < NETWORK_NODE_COUNT; i++) {
                nodeFlashLevels[i] = Math.max(0, nodeFlashLevels[i] - delta * 0.42);
                const offset = i * 3;
                const seed = nodeSeeds[i];

                positions[offset] = nodeBasePositions[offset]
                    + Math.sin(time * 0.65 + seed) * 1.3
                    + Math.cos(time * 0.27 + seed * 0.6) * 0.55;
                positions[offset + 1] = nodeBasePositions[offset + 1]
                    + Math.cos(time * 0.52 + seed * 1.8) * 1.1
                    + Math.sin(time * 0.24 + seed) * 0.45;
                positions[offset + 2] = nodeBasePositions[offset + 2]
                    + Math.sin(time * 0.38 + seed * 1.2) * 0.8;
            }

            particles.geometry.attributes.position.needsUpdate = true;
        }

        function updateNodeVisuals() {
            if (!particles || !activePalette) return;

            const colors = particles.geometry.attributes.color.array;
            const sizes = particles.geometry.attributes.size.array;

            for (let i = 0; i < NETWORK_NODE_COUNT; i++) {
                const flash = Math.min(1, nodeFlashLevels[i]);
                const mix = nodeColorMixes[i];
                const offset = i * 3;
                const baseR = lerp(activePalette.nodeStart.r, activePalette.nodeEnd.r, mix);
                const baseG = lerp(activePalette.nodeStart.g, activePalette.nodeEnd.g, mix);
                const baseB = lerp(activePalette.nodeStart.b, activePalette.nodeEnd.b, mix);

                colors[offset] = lerp(baseR, activePalette.nodeFlash.r, flash);
                colors[offset + 1] = lerp(baseG, activePalette.nodeFlash.g, flash);
                colors[offset + 2] = lerp(baseB, activePalette.nodeFlash.b, flash);
                sizes[i] = nodeBaseSizes[i] * (1 + flash * 1.45);
            }

            particles.geometry.attributes.color.needsUpdate = true;
            particles.geometry.attributes.size.needsUpdate = true;
        }

        function updateConnectionField() {
            if (!lines || !activePalette) return;

            const linePositions = lines.userData.positions;
            const lineColors = lines.userData.colors;
            let floatIndex = 0;

            for (let i = 0; i < networkEdges.length; i++) {
                const edge = networkEdges[i];
                const fromOffset = edge.a * 3;
                const toOffset = edge.b * 3;
                const activity = Math.min(1, Math.max(nodeFlashLevels[edge.a], nodeFlashLevels[edge.b]));
                const colorMix = Math.min(1, activity * 0.75);
                const baseStrength = 0.18 + activity * 0.42;
                const r = lerp(activePalette.line.r, activePalette.pulse.r, colorMix) * baseStrength;
                const g = lerp(activePalette.line.g, activePalette.pulse.g, colorMix) * baseStrength;
                const b = lerp(activePalette.line.b, activePalette.pulse.b, colorMix) * baseStrength;

                linePositions[floatIndex] = nodeCurrentPositions[fromOffset];
                linePositions[floatIndex + 1] = nodeCurrentPositions[fromOffset + 1];
                linePositions[floatIndex + 2] = nodeCurrentPositions[fromOffset + 2];
                lineColors[floatIndex] = r;
                lineColors[floatIndex + 1] = g;
                lineColors[floatIndex + 2] = b;
                floatIndex += 3;

                linePositions[floatIndex] = nodeCurrentPositions[toOffset];
                linePositions[floatIndex + 1] = nodeCurrentPositions[toOffset + 1];
                linePositions[floatIndex + 2] = nodeCurrentPositions[toOffset + 2];
                lineColors[floatIndex] = r;
                lineColors[floatIndex + 1] = g;
                lineColors[floatIndex + 2] = b;
                floatIndex += 3;
            }

            lines.geometry.attributes.position.needsUpdate = true;
            lines.geometry.attributes.color.needsUpdate = true;
        }

        function spawnImpulse(fromNode, toNode, energy) {
            if (fromNode === toNode || energy < MIN_IMPULSE_ENERGY || impulses.length >= MAX_IMPULSES) return;
            impulses.push({
                from: fromNode,
                to: toNode,
                energy,
                progress: 0,
                speed: 0.18 + Math.random() * 0.1 + energy * 0.1,
                tailLength: 0.28 + energy * 0.22
            });
        }

        function triggerNode(nodeIndex, energy, sourceNode) {
            nodeFlashLevels[nodeIndex] = Math.max(nodeFlashLevels[nodeIndex], energy);

            if (energy < MIN_IMPULSE_ENERGY) return;

            let targets = (nodeNeighbors[nodeIndex] || []).filter(target => target !== sourceNode);
            if (targets.length === 0 && typeof sourceNode === 'number' && sourceNode >= 0) {
                targets = [sourceNode];
            }
            if (targets.length === 0) return;

            const branchEnergy = energy * 0.82 / Math.max(1, Math.sqrt(targets.length));
            if (branchEnergy < MIN_IMPULSE_ENERGY) return;

            targets.forEach(target => spawnImpulse(nodeIndex, target, branchEnergy));
        }

        function seedAmbientImpulse(delta) {
            ambientPulseTimer -= delta;
            if (ambientPulseTimer > 0) return;

            triggerNode(Math.floor(Math.random() * NETWORK_NODE_COUNT), 0.9 + Math.random() * 0.18);
            ambientPulseTimer = 0.85 + Math.random() * 1.6;
        }

        function updateImpulseField(delta) {
            if (!impulseTrails || !impulseHeads || !activePalette) return;

            const trailPositions = impulseTrails.userData.positions;
            const trailColors = impulseTrails.userData.colors;
            const headPositions = impulseHeads.userData.positions;
            const headColors = impulseHeads.userData.colors;
            const headSizes = impulseHeads.userData.sizes;
            const headAlpha = impulseHeads.userData.alpha;

            let trailFloatIndex = 0;
            let headCount = 0;

            for (let i = impulses.length - 1; i >= 0; i--) {
                const impulse = impulses[i];
                impulse.energy = Math.max(0, impulse.energy - delta * 0.03);
                impulse.progress += impulse.speed * delta;

                if (impulse.progress >= 1 || impulse.energy < MIN_IMPULSE_ENERGY) {
                    const deliveredEnergy = impulse.energy * 0.92;
                    const targetNode = impulse.to;
                    const sourceNode = impulse.from;
                    impulses.splice(i, 1);
                    if (deliveredEnergy >= MIN_IMPULSE_ENERGY) {
                        triggerNode(targetNode, deliveredEnergy, sourceNode);
                    }
                    continue;
                }

                const fromOffset = impulse.from * 3;
                const toOffset = impulse.to * 3;
                const x1 = nodeCurrentPositions[fromOffset];
                const y1 = nodeCurrentPositions[fromOffset + 1];
                const z1 = nodeCurrentPositions[fromOffset + 2];
                const x2 = nodeCurrentPositions[toOffset];
                const y2 = nodeCurrentPositions[toOffset + 1];
                const z2 = nodeCurrentPositions[toOffset + 2];
                const headX = lerp(x1, x2, impulse.progress);
                const headY = lerp(y1, y2, impulse.progress);
                const headZ = lerp(z1, z2, impulse.progress);
                const headStrength = Math.min(1.18, 0.52 + impulse.energy * 0.82);

                if (headCount < MAX_IMPULSES) {
                    const headOffset = headCount * 3;
                    headPositions[headOffset] = headX;
                    headPositions[headOffset + 1] = headY;
                    headPositions[headOffset + 2] = headZ;
                    headColors[headOffset] = activePalette.pulse.r * headStrength;
                    headColors[headOffset + 1] = activePalette.pulse.g * headStrength;
                    headColors[headOffset + 2] = activePalette.pulse.b * headStrength;
                    headSizes[headCount] = 1.2 + impulse.energy * 2.8;
                    headAlpha[headCount] = 0.16 + impulse.energy * 0.36;
                    headCount++;
                }

                const trailStart = Math.max(0, impulse.progress - impulse.tailLength);
                let previousX = lerp(x1, x2, trailStart);
                let previousY = lerp(y1, y2, trailStart);
                let previousZ = lerp(z1, z2, trailStart);

                for (let segment = 1; segment <= IMPULSE_TRAIL_SEGMENTS; segment++) {
                    if (trailFloatIndex + 6 > trailPositions.length) break;

                    const segmentT = lerp(trailStart, impulse.progress, segment / IMPULSE_TRAIL_SEGMENTS);
                    const currentX = lerp(x1, x2, segmentT);
                    const currentY = lerp(y1, y2, segmentT);
                    const currentZ = lerp(z1, z2, segmentT);
                    const startRatio = (segment - 1) / IMPULSE_TRAIL_SEGMENTS;
                    const endRatio = segment / IMPULSE_TRAIL_SEGMENTS;
                    const startStrength = impulse.energy * (0.12 + Math.pow(startRatio, 1.6) * 0.7);
                    const endStrength = impulse.energy * (0.24 + Math.pow(endRatio, 1.08) * 1.15);

                    trailPositions[trailFloatIndex] = previousX;
                    trailPositions[trailFloatIndex + 1] = previousY;
                    trailPositions[trailFloatIndex + 2] = previousZ;
                    trailColors[trailFloatIndex] = activePalette.trail.r * startStrength;
                    trailColors[trailFloatIndex + 1] = activePalette.trail.g * startStrength;
                    trailColors[trailFloatIndex + 2] = activePalette.trail.b * startStrength;
                    trailFloatIndex += 3;

                    trailPositions[trailFloatIndex] = currentX;
                    trailPositions[trailFloatIndex + 1] = currentY;
                    trailPositions[trailFloatIndex + 2] = currentZ;
                    trailColors[trailFloatIndex] = activePalette.trail.r * endStrength;
                    trailColors[trailFloatIndex + 1] = activePalette.trail.g * endStrength;
                    trailColors[trailFloatIndex + 2] = activePalette.trail.b * endStrength;
                    trailFloatIndex += 3;

                    previousX = currentX;
                    previousY = currentY;
                    previousZ = currentZ;
                }
            }

            for (let i = headCount; i < MAX_IMPULSES; i++) {
                const headOffset = i * 3;
                headPositions[headOffset] = 0;
                headPositions[headOffset + 1] = 0;
                headPositions[headOffset + 2] = 0;
                headColors[headOffset] = 0;
                headColors[headOffset + 1] = 0;
                headColors[headOffset + 2] = 0;
                headSizes[i] = 0.01;
                headAlpha[i] = 0;
            }

            impulseTrails.geometry.setDrawRange(0, trailFloatIndex / 3);
            impulseTrails.geometry.attributes.position.needsUpdate = true;
            impulseTrails.geometry.attributes.color.needsUpdate = true;
            impulseHeads.geometry.attributes.position.needsUpdate = true;
            impulseHeads.geometry.attributes.color.needsUpdate = true;
            impulseHeads.geometry.attributes.size.needsUpdate = true;
            impulseHeads.geometry.attributes.alpha.needsUpdate = true;
        }
        
        function onMouseMove(event) {
            mouseX = (event.clientX / window.innerWidth) * 2 - 1;
            mouseY = -(event.clientY / window.innerHeight) * 2 + 1;
        }
        
        function onWindowResize() {
            camera.aspect = window.innerWidth / window.innerHeight;
            camera.updateProjectionMatrix();
            renderer.setSize(window.innerWidth, window.innerHeight);
        }
        
        function animate(frameTime) {
            requestAnimationFrame(animate);

            const now = typeof frameTime === 'number' ? frameTime : performance.now();
            if (!lastFrameTime) lastFrameTime = now;
            const delta = Math.min((now - lastFrameTime) / 1000, 0.05);
            lastFrameTime = now;
            time += delta * 1.15;

            if (particles) {
                particles.material.uniforms.time.value = time;
                updateNodePositions(delta);
                seedAmbientImpulse(delta);
                updateImpulseField(delta);
                updateNodeVisuals();
                updateConnectionField();
            }
            
            // Animate orbs
            scene.children.forEach(child => {
                if (child.userData && child.userData.originalPos) {
                    const { originalPos, phase } = child.userData;
                    child.position.x = originalPos.x + Math.sin(time * 0.5 + phase) * 5;
                    child.position.y = originalPos.y + Math.cos(time * 0.3 + phase) * 3;
                }
            });
            
            // Camera follows mouse subtly
            camera.position.x += (mouseX * 2 - camera.position.x) * 0.02;
            camera.position.y += (mouseY * 2 - camera.position.y) * 0.02;
            camera.lookAt(scene.position);
            
            renderer.render(scene, camera);
        }
        
        // ═══════════════════════════════════════════════════════════════
        // LOGIN FUNCTIONALITY
        // ═══════════════════════════════════════════════════════════════
        
        if (TOTP_ENABLED) {
            document.getElementById('totpSection').style.display = '';
        }
        
        window.addEventListener('load', () => {
            document.getElementById('password').focus();
        });
        
        function toggleEye(btn, inputId) {
            const inp = document.getElementById(inputId);
            if (inp.type === 'password') {
                inp.type = 'text';
                btn.textContent = '🙈';
            } else {
                inp.type = 'password';
                btn.textContent = '👁';
            }
        }
        
        function showError(msg) {
            const el = document.getElementById('loginError');
            el.textContent = msg;
            el.classList.add('visible');
        }
        
        function clearError() {
            document.getElementById('loginError').classList.remove('visible');
            document.getElementById('password').classList.remove('error-field');
            document.getElementById('totpCode').classList.remove('error-field');
        }

        async function waitForAuthenticatedSession(maxChecks = 6, delayMs = 120) {
            for (let i = 0; i < maxChecks; i++) {
                try {
                    const resp = await fetch('/api/auth/status', {
                        credentials: 'same-origin',
                        cache: 'no-store',
                        headers: { 'Accept': 'application/json' }
                    });
                    if (resp.ok) {
                        const data = await resp.json();
                        if (data && data.authenticated) {
                            return true;
                        }
                    }
                } catch (_) { }
                await new Promise(resolve => setTimeout(resolve, delayMs));
            }
            return false;
        }
        
        async function submitLogin() {
            clearError();
            const password = document.getElementById('password').value;
            const totpCode = document.getElementById('totpCode').value;
            
            if (!password) {
                document.getElementById('password').classList.add('error-field');
                document.getElementById('password').focus();
                return;
            }
            
            const btn = document.getElementById('btnLogin');
            btn.disabled = true;
            document.getElementById('btnText').innerHTML = '<div class="spinner-sm"></div>';
            
            try {
                const redirect = REDIRECT_URL || '/';
                const resp = await fetch('/auth/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    credentials: 'same-origin',
                    cache: 'no-store',
                    body: JSON.stringify({ password, totp_code: totpCode, redirect }),
                });
                
                const data = await resp.json();
                
                if (resp.ok && data.ok) {
                    await waitForAuthenticatedSession();
                    // Validate redirect: only allow relative paths starting with /
                    const redirect = data.redirect || '/';
                    const allowed = redirect === '/' || (/^\/[a-zA-Z0-9._~!$&'()*+,;=:@-]*$/).test(redirect);
                    window.location.replace(allowed ? redirect : '/');
                } else {
                    showError(data.error || t('login.error_failed'));
                    document.getElementById('password').classList.add('error-field');
                    if (TOTP_ENABLED && data.error && data.error.toLowerCase().includes('authenticator')) {
                        document.getElementById('totpCode').classList.add('error-field');
                        document.getElementById('totpCode').focus();
                    } else {
                        document.getElementById('password').focus();
                    }
                }
            } catch (err) {
                showError(t('login.error_network'));
            } finally {
                btn.disabled = false;
                document.getElementById('btnText').textContent = t('login.btn_submit');
            }
        }
