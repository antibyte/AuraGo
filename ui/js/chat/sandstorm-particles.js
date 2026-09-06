(function () {
    'use strict';

    // ═══════════════════════════════════════════════════════════════
    //  SANDSTORM PARTICLE + WEBGL FOG ENGINE
    // ═══════════════════════════════════════════════════════════════

    // ─── 2D Particle Config ───
    const MAX_CLOUDS = 5;
    const MAX_FLYING = 320;
    const MAX_TRAILS = 35;
    const MAX_GROUND = 2500;

    const GRAVITY = 0.42;
    const TERMINAL_VELOCITY = 5.5;
    const BASE_WIND = 1.15;
    const STORM_WIND = 6.5;
    const STORM_LIFT = -5.0;
    const IDLE_MIN = 12000;
    const IDLE_MAX = 24000;
    const STORM_DURATION = 9000;
    const GROUND_RES = 3;

    // ─── WebGL Fog Config ───
    const FOG_VERTEX = `
        attribute vec2 a_position;
        void main() {
            gl_Position = vec4(a_position, 0.0, 1.0);
        }
    `;

    const FOG_FRAGMENT = `
        #ifdef GL_FRAGMENT_PRECISION_HIGH
        precision highp float;
        #else
        precision mediump float;
        #endif
        uniform float u_time;
        uniform vec2 u_resolution;
        uniform float u_storm;
        uniform float u_drift;
        uniform vec4 u_eddies[3];

        float hash(vec2 p) {
            vec3 h = fract(vec3(p.xyx) * 0.1031);
            h += dot(h, h.yzx + 33.33);
            return fract((h.x + h.y) * h.z);
        }

        float noise(vec2 p) {
            vec2 i = floor(p);
            vec2 f = fract(p);
            f = f * f * (3.0 - 2.0 * f);
            float a = hash(i);
            float b = hash(i + vec2(1.0, 0.0));
            float c = hash(i + vec2(0.0, 1.0));
            float d = hash(i + vec2(1.0, 1.0));
            return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
        }

        float fbm(vec2 p) {
            float v = 0.0;
            float a = 0.5;
            for (int i = 0; i < 4; i++) {
                v += a * noise(p);
                p = mat2(0.8, -0.6, 0.6, 0.8) * p * 2.03;
                a *= 0.5;
            }
            return v;
        }

        void main() {
            vec2 uv = gl_FragCoord.xy / u_resolution;
            vec2 aspect = vec2(u_resolution.x / u_resolution.y, 1.0);

            // The same moving eddies bend the fog and carry the visible grains.
            vec2 p = vec2(uv.x, 1.0 - uv.y) * aspect;
            vec2 folded = p;
            for (int i = 0; i < 3; i++) {
                vec4 eddy = u_eddies[i];
                vec2 q = folded - eddy.xy;
                float falloff = 1.0 / (1.0 + dot(q, q) / (eddy.z * eddy.z));
                float angle = eddy.w * 0.075 * falloff * falloff;
                float c = cos(angle), s = sin(angle);
                folded = eddy.xy + mat2(c, -s, s, c) * q;
            }
            vec2 flow = p * vec2(3.2, 4.5) - vec2(u_drift, u_time * 0.065);
            float distant = fbm(flow * 0.55 + vec2(3.2, 1.7));
            vec2 curl = vec2(noise(flow * 0.8 + u_time * 0.11), noise(flow.yx * 0.7 - u_time * 0.09)) - 0.5;
            float closeDust = fbm(folded * vec2(5.3, 7.4) + curl * 1.4 - vec2(u_drift * 1.3, u_time * 0.1));
            float groundSheet = noise(folded * vec2(4.0, 18.0) + curl * 2.0 - vec2(u_drift * 1.1, 0.0));
            float low = 1.0 - smoothstep(0.0, 0.48, uv.y);
            float density = smoothstep(0.23, 0.78, distant * 0.5 + closeDust * 0.65);
            density += low * smoothstep(0.35, 0.72, groundSheet) * (0.35 + u_storm * 0.45);

            // Warm light scatters through thin dust; dense folds stay ochre.
            vec2 toLight = p - vec2(aspect.x * 0.82, 0.12);
            float light = exp(-dot(toLight, toLight) * 2.8);
            vec3 col = mix(vec3(0.40, 0.28, 0.16), vec3(0.89, 0.73, 0.48),
                clamp(0.35 + light * 0.4 + closeDust * 0.3, 0.0, 1.0));
            float alpha = density * (0.17 + u_storm * 0.18) * (0.65 + low * 0.35);
            alpha = clamp(alpha, 0.0, 0.34);

            gl_FragColor = vec4(col * alpha, alpha);
        }
    `;

    // ─── Shared State ───
    let chatBox;
    let resizeObserver = null;
    let animationId = null;
    let active = false;
    let lastTime = 0;

    // ─── 2D Canvas State ───
    let canvas, ctx;

    // Flying particles
    let fx, fy, fvx, fvy, fs, fa, fd, fk;
    let fCount = 0;

    // Trails
    let tx, ty, tvx, tvy, tl, ta, td;
    let tCount = 0;

    // Ground particles
    let gx, gy, gs, ga, gd;
    let gCount = 0;

    // Background clouds
    let cx, cy, cr, ca, cvx, cvy;
    let cCount = 0;

    // Ground height map
    let groundHeight = [];

    // Bubble tracking and sand accumulation state
    let cachedBubbles = [];
    let lastBubbleQueryTime = 0;
    let lastScrollTop = 0;
    const sandColors = [
        '242, 220, 185', // pale cream
        '224, 184, 126', // warm gold
        '198, 150, 96',  // amber
        '163, 112, 57',  // ochre
        '138, 89, 44'    // deep brown
    ];

    function updateBubbleBounds(time) {
        if (!chatBox) return;
        
        // Query bubble elements every 150ms to keep performance high
        if (time - lastBubbleQueryTime > 150) {
            lastBubbleQueryTime = time;
            const bubbleElements = chatBox.querySelectorAll('.bubble');
            const chatBoxRect = chatBox.getBoundingClientRect();
            
            cachedBubbles = [];
            for (let i = 0; i < bubbleElements.length; i++) {
                const el = bubbleElements[i];
                const rect = el.getBoundingClientRect();
                
                const left = rect.left - chatBoxRect.left;
                const top = rect.top - chatBoxRect.top;
                const right = rect.right - chatBoxRect.left;
                const bottom = rect.bottom - chatBoxRect.top;
                const width = rect.width;
                
                // Only track if the bubble is in/near the visible screen area
                if (bottom > -50 && top < chatBoxRect.height + 50 && width > 20) {
                    const BUCKET_SIZE = 3;
                    const numBuckets = Math.ceil(width / BUCKET_SIZE);
                    
                    // Initialize heightmap on DOM element for automatic GC
                    if (!el.__sandHeightMap || el.__sandHeightMap.length !== numBuckets) {
                        el.__sandHeightMap = new Float32Array(numBuckets);
                    }
                    
                    cachedBubbles.push({
                        el: el,
                        left: left,
                        top: top,
                        right: right,
                        bottom: bottom,
                        width: width,
                        bucketSize: BUCKET_SIZE,
                        heightMap: el.__sandHeightMap
                    });
                }
            }
        } else {
            // Just update the positions of already cached bubbles (extremely cheap!)
            const chatBoxRect = chatBox.getBoundingClientRect();
            for (let i = 0; i < cachedBubbles.length; i++) {
                const b = cachedBubbles[i];
                const rect = b.el.getBoundingClientRect();
                b.left = rect.left - chatBoxRect.left;
                b.top = rect.top - chatBoxRect.top;
                b.right = rect.right - chatBoxRect.left;
                b.bottom = rect.bottom - chatBoxRect.top;
            }
        }
    }

    function handleScrollErosion() {
        if (!chatBox) return;
        const scrollTop = chatBox.scrollTop || 0;
        const diff = Math.abs(scrollTop - lastScrollTop);
        lastScrollTop = scrollTop;
        
        if (diff > 1) {
            const decay = Math.min(0.5, diff * 0.04);
            for (let i = 0; i < cachedBubbles.length; i++) {
                const b = cachedBubbles[i];
                const map = b.heightMap;
                for (let k = 0; k < map.length; k++) {
                    map[k] = Math.max(0, map[k] - decay);
                }
            }
        }
    }

    function updateBubbleSandPhysics(dt) {
        const MAX_SLOPE = 0.8; // Maximum slope before sand slides down (angle of repose)
        for (let i = 0; i < cachedBubbles.length; i++) {
            const b = cachedBubbles[i];
            const map = b.heightMap;
            
            // 1. Sliding physics pass (angle of repose)
            for (let pass = 0; pass < 2; pass++) {
                for (let col = 0; col < map.length; col++) {
                    if (col > 0) {
                        const diff = map[col] - map[col - 1];
                        if (diff > MAX_SLOPE) {
                            const transfer = (diff - MAX_SLOPE) * 0.45 * dt;
                            map[col] -= transfer;
                            map[col - 1] += transfer;
                        }
                    }
                    if (col < map.length - 1) {
                        const diff = map[col] - map[col + 1];
                        if (diff > MAX_SLOPE) {
                            const transfer = (diff - MAX_SLOPE) * 0.45 * dt;
                            map[col] -= transfer;
                            map[col + 1] += transfer;
                        }
                    }
                }
            }
            
            // 2. Slow natural erosion/settling over time (wind or gravity)
            for (let col = 0; col < map.length; col++) {
                if (map[col] > 0) {
                    map[col] = Math.max(0, map[col] - 0.003 * dt);
                }
            }
        }
    }

    function getBubbleSurfaceY(b, x) {
        const R = 20; // corner radius of the bubble
        const r = Math.min(R, b.width / 2);
        if (x < r) {
            return b.top + r - Math.sqrt(r * r - Math.pow(r - x, 2));
        } else if (b.width - x < r) {
            return b.top + r - Math.sqrt(r * r - Math.pow(r - (b.width - x), 2));
        }
        return b.top;
    }

    function drawBubbleSand() {
        ctx.save();
        for (let i = 0; i < cachedBubbles.length; i++) {
            const b = cachedBubbles[i];
            const map = b.heightMap;
            let hasSand = false;
            for (let k = 0; k < map.length; k++) {
                if (map[k] > 0.15) { hasSand = true; break; }
            }
            if (!hasSand) continue;
            
            // Draw soft shadow under/around the sand
            ctx.shadowBlur = 3;
            ctx.shadowColor = 'rgba(163, 112, 57, 0.4)';
            
            // 1. Draw base/shadow layer
            ctx.beginPath();
            ctx.moveTo(b.left, getBubbleSurfaceY(b, 0));
            for (let col = 0; col < map.length; col++) {
                const px = b.left + col * b.bucketSize + b.bucketSize / 2;
                const x = col * b.bucketSize + b.bucketSize / 2;
                const surfaceY = getBubbleSurfaceY(b, x);
                
                // Rounded corner tapering (first 14px and last 14px of bubble width)
                const distFromLeft = x;
                const distFromRight = b.width - distFromLeft;
                let taper = 1;
                if (distFromLeft < 14) {
                    taper = distFromLeft / 14;
                } else if (distFromRight < 14) {
                    taper = distFromRight / 14;
                }
                
                const py = surfaceY - map[col] * taper;
                ctx.lineTo(px, py);
            }
            ctx.lineTo(b.right, getBubbleSurfaceY(b, b.width));
            
            // Trace back along the curved bubble surface
            for (let col = map.length - 1; col >= 0; col--) {
                const px = b.left + col * b.bucketSize + b.bucketSize / 2;
                const x = col * b.bucketSize + b.bucketSize / 2;
                ctx.lineTo(px, getBubbleSurfaceY(b, x));
            }
            ctx.closePath();
            
            // Base sand gradient
            const sandGrad = ctx.createLinearGradient(0, b.top - 18, 0, b.top);
            sandGrad.addColorStop(0, 'rgba(235, 195, 140, 0.95)'); // Bright top highlight
            sandGrad.addColorStop(0.4, 'rgba(210, 165, 110, 0.95)'); // Golden middle
            sandGrad.addColorStop(1, 'rgba(150, 95, 45, 0.95)'); // Warm base
            ctx.fillStyle = sandGrad;
            ctx.fill();
            
            // Turn off shadow for highlights
            ctx.shadowBlur = 0;
            
            // 2. Draw a beautiful golden peak highlight layer (gives 3D depth)
            ctx.beginPath();
            ctx.moveTo(b.left, getBubbleSurfaceY(b, 0));
            for (let col = 0; col < map.length; col++) {
                const px = b.left + col * b.bucketSize + b.bucketSize / 2;
                const x = col * b.bucketSize + b.bucketSize / 2;
                const surfaceY = getBubbleSurfaceY(b, x);
                
                const distFromLeft = x;
                const distFromRight = b.width - distFromLeft;
                let taper = 1;
                if (distFromLeft < 14) taper = distFromLeft / 14;
                else if (distFromRight < 14) taper = distFromRight / 14;
                
                const py = surfaceY - map[col] * taper * 0.7;
                ctx.lineTo(px, py);
            }
            ctx.lineTo(b.right, getBubbleSurfaceY(b, b.width));
            
            // Trace back along curved bubble surface
            for (let col = map.length - 1; col >= 0; col--) {
                const px = b.left + col * b.bucketSize + b.bucketSize / 2;
                const x = col * b.bucketSize + b.bucketSize / 2;
                ctx.lineTo(px, getBubbleSurfaceY(b, x));
            }
            ctx.closePath();
            
            const highlightGrad = ctx.createLinearGradient(0, b.top - 12, 0, b.top);
            highlightGrad.addColorStop(0, 'rgba(255, 235, 200, 0.6)');
            highlightGrad.addColorStop(1, 'rgba(235, 195, 140, 0)');
            ctx.fillStyle = highlightGrad;
            ctx.fill();
            
            // 3. Draw soft, organic sand speck grains
            for (let col = 0; col < map.length; col += 2) {
                const x = col * b.bucketSize + b.bucketSize / 2;
                const surfaceY = getBubbleSurfaceY(b, x);
                
                const distFromLeft = col * b.bucketSize;
                const distFromRight = b.width - distFromLeft;
                let taper = 1;
                if (distFromLeft < 14) taper = distFromLeft / 14;
                else if (distFromRight < 14) taper = distFromRight / 14;
                
                const h = map[col] * taper;
                if (h > 0.8) {
                    ctx.fillStyle = `rgba(255, 245, 220, ${rand(0.3, 0.85)})`;
                    const px = b.left + col * b.bucketSize + rand(-1.2, 1.2);
                    const py = surfaceY - h - rand(0, 1.0);
                    ctx.fillRect(px, py, 1.0, 1.0);
                    
                    if (col % 4 === 0) {
                        ctx.fillStyle = `rgba(138, 89, 44, ${rand(0.2, 0.5)})`;
                        ctx.fillRect(px, py + 1, 1.0, 1.0);
                    }
                }
            }
        }
        ctx.restore();
    }

    // Storm state
    let stormActive = false;
    let stormEndTime = 0;
    let nextStormTime = 0;
    let stormIntensity = 0;
    let windSpeed = BASE_WIND;
    let windDrift = 0;
    let weatherTime = 0;
    let liftCarry = 0;
    let windDirection = 1;
    // Three bounded vortices, in canvas-height units, shared with the shader.
    const eddies = new Float32Array(12);
    const eddyPhases = new Float32Array(3);
    let flowX = 0, flowY = 0;

    // ─── WebGL Fog State ───
    let fogCanvas, gl, fogProgram;
    let uTime, uRes, uStorm, uDrift, uEddies;
    let fogPositionBuffer;
    let fogActive = false;

    // ═══════════════════════════════════════════════════════════════
    //  UTILITIES
    // ═══════════════════════════════════════════════════════════════

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'sandstorm' &&
            !document.hidden &&
            !prefersReducedMotion() &&
            window.innerWidth >= 640;
    }

    function rand(min, max) {
        return min + Math.random() * (max - min);
    }

    // ═══════════════════════════════════════════════════════════════
    //  CANVAS & WEBGL SETUP
    // ═══════════════════════════════════════════════════════════════

    function ensureCanvas() {
        if (canvas) return;
        canvas = document.createElement('canvas');
        canvas.id = 'sandstorm-overlay';
        canvas.setAttribute('aria-hidden', 'true');
        Object.assign(canvas.style, {
            position: 'fixed', top: '0', left: '0', width: '0', height: '0',
            pointerEvents: 'none', zIndex: '2', opacity: '1',
            mixBlendMode: 'normal', display: 'none'
        });
        document.body.appendChild(canvas);
        ctx = canvas.getContext('2d', { alpha: true, desynchronized: true });
    }

    function initFogGL() {
        if (fogCanvas) return !!(gl && fogProgram && gl.getProgramParameter(fogProgram, gl.LINK_STATUS));

        fogCanvas = document.createElement('canvas');
        fogCanvas.id = 'sandstorm-fog';
        fogCanvas.setAttribute('aria-hidden', 'true');
        Object.assign(fogCanvas.style, {
            position: 'fixed', top: '0', left: '0', width: '0', height: '0',
            pointerEvents: 'none', zIndex: '2', opacity: '1',
            mixBlendMode: 'normal', display: 'none'
        });
        document.body.insertBefore(fogCanvas, canvas);

        gl = fogCanvas.getContext('webgl', {
            alpha: true,
            premultipliedAlpha: true,
            antialias: false,
            preserveDrawingBuffer: false
        });
        if (!gl) {
            console.warn('[Sandstorm] WebGL not available, fog disabled');
            return false;
        }

        const vs = gl.createShader(gl.VERTEX_SHADER);
        gl.shaderSource(vs, FOG_VERTEX);
        gl.compileShader(vs);
        if (!gl.getShaderParameter(vs, gl.COMPILE_STATUS)) {
            console.error('Fog VS error:', gl.getShaderInfoLog(vs));
            return false;
        }

        const fs = gl.createShader(gl.FRAGMENT_SHADER);
        gl.shaderSource(fs, FOG_FRAGMENT);
        gl.compileShader(fs);
        if (!gl.getShaderParameter(fs, gl.COMPILE_STATUS)) {
            console.error('Fog FS error:', gl.getShaderInfoLog(fs));
            return false;
        }

        fogProgram = gl.createProgram();
        gl.attachShader(fogProgram, vs);
        gl.attachShader(fogProgram, fs);
        gl.linkProgram(fogProgram);
        if (!gl.getProgramParameter(fogProgram, gl.LINK_STATUS)) {
            console.error('Fog link error:', gl.getProgramInfoLog(fogProgram));
            return false;
        }

        uTime = gl.getUniformLocation(fogProgram, 'u_time');
        uRes = gl.getUniformLocation(fogProgram, 'u_resolution');
        uStorm = gl.getUniformLocation(fogProgram, 'u_storm');
        uDrift = gl.getUniformLocation(fogProgram, 'u_drift');
        uEddies = gl.getUniformLocation(fogProgram, 'u_eddies[0]');
        gl.deleteShader(vs);
        gl.deleteShader(fs);

        const posLoc = gl.getAttribLocation(fogProgram, 'a_position');
        fogPositionBuffer = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, fogPositionBuffer);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([
            -1, -1,  1, -1,  -1, 1,
            -1,  1,  1, -1,   1, 1
        ]), gl.STATIC_DRAW);
        gl.enableVertexAttribArray(posLoc);
        gl.vertexAttribPointer(posLoc, 2, gl.FLOAT, false, 0, 0);

        // The fullscreen pass replaces every pixel with premultiplied dust.
        gl.disable(gl.BLEND);

        return true;
    }

    function updateBounds() {
        if (!chatBox) return false;
        const rect = chatBox.getBoundingClientRect();
        const footer = document.querySelector('.app-footer');
        let extraH = 0;
        if (footer) {
            const fr = footer.getBoundingClientRect();
            extraH = Math.max(0, fr.bottom - rect.bottom);
        }
        if (!rect.width || !rect.height) {
            if (canvas) { canvas.style.width = '0'; canvas.style.height = '0'; }
            if (fogCanvas) { fogCanvas.style.width = '0'; fogCanvas.style.height = '0'; }
            return false;
        }
        const left = `${Math.round(rect.left)}px`;
        const top = `${Math.round(rect.top)}px`;
        const w = `${Math.round(rect.width)}px`;
        const h = `${Math.round(rect.height + extraH)}px`;

        if (canvas) {
            canvas.style.left = left; canvas.style.top = top;
            canvas.style.width = w; canvas.style.height = h;
        }
        if (fogCanvas) {
            fogCanvas.style.left = left; fogCanvas.style.top = top;
            fogCanvas.style.width = w; fogCanvas.style.height = h;
        }
        return true;
    }

    // ═══════════════════════════════════════════════════════════════
    //  2D PARTICLE SYSTEM
    // ═══════════════════════════════════════════════════════════════

    function initGround(width) {
        const buckets = Math.ceil(width / GROUND_RES) + 2;
        groundHeight = new Float32Array(buckets);
        for (let i = 0; i < buckets; i++) groundHeight[i] = 0;
    }

    function getGroundY(x, canvasHeight) {
        const idx = Math.floor(x / GROUND_RES);
        if (idx < 0 || idx >= groundHeight.length) return canvasHeight;
        return canvasHeight - groundHeight[idx];
    }

    function addGroundHeight(x, amount) {
        const idx = Math.floor(x / GROUND_RES);
        if (idx >= 0 && idx < groundHeight.length) {
            groundHeight[idx] = Math.min(groundHeight[idx] + amount, 35);
        }
        if (idx > 0) groundHeight[idx - 1] = Math.min(groundHeight[idx - 1] + amount * 0.25, 35);
        if (idx + 1 < groundHeight.length) groundHeight[idx + 1] = Math.min(groundHeight[idx + 1] + amount * 0.25, 35);
    }

    function removeGroundHeight(x, amount) {
        const idx = Math.floor(x / GROUND_RES);
        if (idx >= 0 && idx < groundHeight.length) {
            groundHeight[idx] = Math.max(0, groundHeight[idx] - amount);
        }
        if (idx > 0) groundHeight[idx - 1] = Math.max(0, groundHeight[idx - 1] - amount * 0.3);
        if (idx + 1 < groundHeight.length) groundHeight[idx + 1] = Math.max(0, groundHeight[idx + 1] - amount * 0.3);
    }

    function spawnFlying(i, width, height, fromEdge) {
        fx[i] = fromEdge ? (windDirection >= 0 ? rand(-width * 0.12, -5) : rand(width + 5, width * 1.12)) : rand(-width * 0.05, width * 1.02);
        fy[i] = rand(-height * 0.08, height * 0.98);
        fvx[i] = rand(BASE_WIND * 0.4, BASE_WIND * 1.3) * windDirection;
        fvy[i] = rand(-0.8, 1.2);
        fd[i] = rand(0.45, 1.45);
        fs[i] = rand(0.65, 1.35) * fd[i];
        fa[i] = rand(0.3, 0.6) * Math.min(1, fd[i]);
        fk[i] = Math.random() > 0.88 ? 1 : 0;
    }

    function spawnTrail(i, width, height, fromEdge) {
        tx[i] = fromEdge ? (windDirection >= 0 ? rand(-width * 0.12, -5) : rand(width + 5, width * 1.12)) : rand(-width * 0.05, width);
        ty[i] = rand(-height * 0.2, height * 0.85);
        tvx[i] = rand(1.0, 2.8) * windDirection;
        tvy[i] = rand(-0.4, 0.4);
        tl[i] = rand(10, 28);
        ta[i] = rand(0.04, 0.13);
        td[i] = rand(0.8, 1.4);
    }

    function spawnCloud(i, width, height) {
        cx[i] = rand(-width * 0.3, width * 1.1);
        cy[i] = rand(height * 0.25, height * 0.95);
        cr[i] = rand(width * 0.06, width * 0.2);
        ca[i] = rand(0.06, 0.12);
        cvx[i] = rand(0.015, 0.05);
        cvy[i] = rand(-0.008, 0.008);
    }

    function addToGround(x, y, size, alpha) {
        if (gCount >= MAX_GROUND) {
            gx[0] = x; gy[0] = y; gs[0] = size; ga[0] = alpha; gd[0] = rand(0, Math.PI * 2);
            addGroundHeight(x, size * 0.35);
            return;
        }
        gx[gCount] = x; gy[gCount] = y; gs[gCount] = size; ga[gCount] = alpha; gd[gCount] = rand(0, Math.PI * 2);
        gCount++;
        addGroundHeight(x, size * 0.35);
    }

    function rebuildPools(width, height) {
        const area = width * height;
        fCount = Math.max(50, Math.min(MAX_FLYING, Math.round(area / 7000)));
        tCount = Math.max(12, Math.min(MAX_TRAILS, Math.round(area / 30000)));
        cCount = Math.max(2, Math.min(MAX_CLOUDS, Math.round(area / 180000)));

        fx = new Float32Array(MAX_FLYING); fy = new Float32Array(MAX_FLYING);
        fvx = new Float32Array(MAX_FLYING); fvy = new Float32Array(MAX_FLYING);
        fs = new Float32Array(MAX_FLYING); fa = new Float32Array(MAX_FLYING);
        fd = new Float32Array(MAX_FLYING); fk = new Uint8Array(MAX_FLYING);

        tx = new Float32Array(MAX_TRAILS); ty = new Float32Array(MAX_TRAILS);
        tvx = new Float32Array(MAX_TRAILS); tvy = new Float32Array(MAX_TRAILS);
        tl = new Float32Array(MAX_TRAILS); ta = new Float32Array(MAX_TRAILS);
        td = new Float32Array(MAX_TRAILS);

        gx = new Float32Array(MAX_GROUND); gy = new Float32Array(MAX_GROUND);
        gs = new Float32Array(MAX_GROUND); ga = new Float32Array(MAX_GROUND);
        gd = new Float32Array(MAX_GROUND);
        gCount = 0;

        cx = new Float32Array(MAX_CLOUDS); cy = new Float32Array(MAX_CLOUDS);
        cr = new Float32Array(MAX_CLOUDS); ca = new Float32Array(MAX_CLOUDS);
        cvx = new Float32Array(MAX_CLOUDS); cvy = new Float32Array(MAX_CLOUDS);

        for (let i = 0; i < fCount; i++) spawnFlying(i, width, height, false);
        for (let i = 0; i < tCount; i++) spawnTrail(i, width, height, false);
        for (let i = 0; i < cCount; i++) spawnCloud(i, width, height);

        initGround(width);
    }

    function resize() {
        ensureCanvas();
        if (!ctx || !updateBounds()) return;
        const dpr = Math.min(window.devicePixelRatio || 1, 1.75);
        const rect = canvas.getBoundingClientRect();
        const w = Math.max(1, Math.round(rect.width));
        const h = Math.max(1, Math.round(rect.height));
        canvas.width = Math.floor(w * dpr);
        canvas.height = Math.floor(h * dpr);
        ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
        rebuildPools(w, h);

        // Resize WebGL fog canvas
        if (fogActive && gl && fogCanvas) {
            const fRect = fogCanvas.getBoundingClientRect();
            const fw = Math.max(1, Math.round(fRect.width));
            const fh = Math.max(1, Math.round(fRect.height));
            // Soft dust needs no device-pixel detail, even on a 4K screen.
            const fogScale = Math.min(0.65, 960 / fw, 540 / fh);
            fogCanvas.width = Math.max(1, Math.floor(fw * fogScale));
            fogCanvas.height = Math.max(1, Math.floor(fh * fogScale));
            gl.viewport(0, 0, fogCanvas.width, fogCanvas.height);
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  2D DRAWING
    // ═══════════════════════════════════════════════════════════════

    function drawClouds(width, height, time) {
        for (let i = 0; i < cCount; i++) {
            ctx.save();
            ctx.translate(cx[i], cy[i]);
            ctx.rotate(Math.atan2(cvy[i], cvx[i]) + Math.sin(weatherTime * 0.23 + i) * 0.3);
            ctx.scale(1.35, 0.55 + Math.sin(weatherTime * 0.35 + i * 2) * 0.16);
            const g = ctx.createRadialGradient(0, 0, 0, 0, 0, cr[i]);
            const alpha = ca[i] * (0.75 + Math.sin(weatherTime * 0.4 + i * 3.1) * 0.25) * (1 + stormIntensity * 0.65);
            g.addColorStop(0, `rgba(210, 178, 132, ${alpha})`);
            g.addColorStop(0.5, `rgba(180, 142, 96, ${alpha * 0.4})`);
            g.addColorStop(1, `rgba(150, 112, 68, 0)`);
            ctx.fillStyle = g;
            ctx.fillRect(-cr[i], -cr[i], cr[i] * 2, cr[i] * 2);
            ctx.restore();
        }
    }

    function updateClouds(dt, width, height) {
        for (let i = 0; i < cCount; i++) {
            sampleWind(cx[i], cy[i], height);
            const drag = 1 - Math.exp(-dt * 0.02);
            cvx[i] += (flowX * 0.35 - cvx[i]) * drag;
            cvy[i] += (flowY * 0.35 - cvy[i]) * drag;
            cx[i] += cvx[i] * dt;
            cy[i] += cvy[i] * dt;
            const margin = cr[i] * 1.4;
            if (cx[i] - margin > width) cx[i] = -margin;
            if (cx[i] + margin < 0) cx[i] = width + margin;
            if (cy[i] + margin < 0) cy[i] = height + margin;
            if (cy[i] - margin > height) cy[i] = -margin;
        }
    }

    function updateEddies(width, height) {
        for (let i = 0; i < 3; i++) {
            const phase = eddyPhases[i], t = weatherTime;
            eddies[i * 4] = (0.2 + i * 0.3 + Math.sin(t * 0.13 + phase) * 0.12) * width / height;
            eddies[i * 4 + 1] = 0.58 + Math.sin(t * 0.19 + phase * 1.7) * 0.24;
            eddies[i * 4 + 2] = 0.17 + 0.045 * Math.sin(t * 0.21 + phase);
            eddies[i * 4 + 3] = (i === 1 ? -1 : 1) * (9 + stormIntensity * 14) * (0.7 + 0.3 * Math.sin(t * 0.37 + phase));
        }
    }

    function sampleWind(x, y, height) {
        const px = x / height, py = y / height;
        flowX = windSpeed * windDirection * 0.45 + Math.sin(py * 9 + weatherTime * 0.7) * 0.6;
        flowY = Math.cos(px * 7 - weatherTime * 0.5) * 0.65;
        for (let i = 0; i < 12; i += 4) {
            const dx = (px - eddies[i]) / eddies[i + 2], dy = (py - eddies[i + 1]) / eddies[i + 2];
            const falloff = 1 / (1 + dx * dx + dy * dy);
            const spin = eddies[i + 3] * falloff * falloff;
            flowX -= dy * spin;
            flowY += dx * spin;
        }
    }

    function drawDustRolls(height) {
        // Broken, soft arcs of dust make the circulation visible, also without WebGL.
        for (let i = 0; i < 3; i++) {
            const offset = i * 4, radius = eddies[offset + 2] * height;
            const phase = weatherTime * (i === 1 ? -0.65 : 0.55) + eddyPhases[i];
            for (let j = 0; j < 9; j++) {
                const angle = phase + j * 0.48;
                const distance = radius * (0.55 + j * 0.045);
                const x = eddies[offset] * height + Math.cos(angle) * distance;
                const y = eddies[offset + 1] * height + Math.sin(angle) * distance * 0.8;
                const size = radius * (0.22 + j * 0.016);
                const alpha = Math.sin((j + 1) * Math.PI / 10) * (0.045 + stormIntensity * 0.07);
                const dust = ctx.createRadialGradient(x, y, 0, x, y, size);
                dust.addColorStop(0, `rgba(222, 184, 125, ${alpha})`);
                dust.addColorStop(0.45, `rgba(184, 137, 78, ${alpha * 0.55})`);
                dust.addColorStop(1, 'rgba(160, 110, 58, 0)');
                ctx.fillStyle = dust;
                ctx.fillRect(x - size, y - size, size * 2, size * 2);
            }
        }
    }

    function drawGroundPile(width, height) {
        let hasSand = false;
        for (let i = 0; i < groundHeight.length; i++) {
            if (groundHeight[i] > 0.5) { hasSand = true; break; }
        }
        if (!hasSand) return;
        ctx.beginPath();
        ctx.moveTo(0, height);
        for (let x = 0; x <= width; x += 2) {
            const idx = Math.floor(x / GROUND_RES);
            let h = 0;
            if (idx >= 0 && idx < groundHeight.length) {
                h = groundHeight[idx];
                const nextIdx = Math.min(idx + 1, groundHeight.length - 1);
                const frac = (x % GROUND_RES) / GROUND_RES;
                h += (groundHeight[nextIdx] - h) * frac;
            }
            const noise = Math.sin(x * 0.12) * 1.2 + Math.sin(x * 0.05 + 0.8) * 2.0;
            ctx.lineTo(x, height - Math.max(0, h + noise * 0.25));
        }
        ctx.lineTo(width, height);
        ctx.closePath();

        const grad = ctx.createLinearGradient(0, height - 20, 0, height);
        grad.addColorStop(0, 'rgba(195, 160, 115, 0.35)');
        grad.addColorStop(0.35, 'rgba(175, 138, 92, 0.55)');
        grad.addColorStop(0.75, 'rgba(155, 118, 75, 0.75)');
        grad.addColorStop(1, 'rgba(135, 100, 62, 0.9)');
        ctx.fillStyle = grad;
        ctx.fill();
    }

    function drawGroundParticles() {
        for (let i = 0; i < gCount; i++) {
            const drift = Math.sin(gd[i] + performance.now() * 0.0008) * 0.25;
            const x = gx[i] + drift, y = gy[i], s = gs[i];
            ctx.fillStyle = `rgba(185, 148, 105, ${ga[i] * 0.85})`;
            ctx.fillRect(x, y, s, s);
            ctx.fillStyle = `rgba(215, 182, 138, ${ga[i] * 0.35})`;
            ctx.fillRect(x + s * 0.1, y + s * 0.1, s * 0.45, s * 0.45);
        }
    }

    function updateAndDrawFlying(dt, time, width, height) {
        const storm = stormIntensity > 0.35;

        for (let i = 0; i < fCount; i++) {
            const groundDist = Math.max(0, height - fy[i]);
            const heightFactor = 0.32 + 0.68 * Math.min(1, groundDist / (height * 0.35));
            sampleWind(fx[i], fy[i], height);
            const drag = 1 - Math.exp(-dt * 0.065 / fd[i]);
            fvx[i] += (flowX * heightFactor - fvx[i]) * drag;
            let targetVy = GRAVITY * fd[i] + flowY;
            if (fy[i] > height * 0.65) targetVy += STORM_LIFT * stormIntensity * 0.5;
            fvy[i] += (targetVy - fvy[i]) * drag;
            fvy[i] = Math.max(-TERMINAL_VELOCITY, Math.min(TERMINAL_VELOCITY, fvy[i]));

            fx[i] += fvx[i] * fd[i] * dt * 2.4;
            fy[i] += fvy[i] * fd[i] * dt * 2.2;

            // Check collision with bubbles
            let landed = false;
            if (!storm && fvy[i] > -0.2) {
                for (let bIdx = 0; bIdx < cachedBubbles.length; bIdx++) {
                    const b = cachedBubbles[bIdx];
                    if (fx[i] >= b.left && fx[i] <= b.right) {
                        const col = Math.floor((fx[i] - b.left) / b.bucketSize);
                        if (col >= 0 && col < b.heightMap.length) {
                            const distFromLeft = col * b.bucketSize;
                            const distFromRight = b.width - distFromLeft;
                            let taper = 1;
                            if (distFromLeft < 14) {
                                taper = distFromLeft / 14;
                            } else if (distFromRight < 14) {
                                taper = distFromRight / 14;
                            }
                            const surfaceY = getBubbleSurfaceY(b, fx[i] - b.left) - b.heightMap[col] * taper;
                            if (fy[i] >= surfaceY - 4.5 && fy[i] <= surfaceY + 2.5) {
                                b.heightMap[col] = Math.min(b.heightMap[col] + fs[i] * 0.45, 18);
                                if (col > 0) b.heightMap[col - 1] = Math.min(b.heightMap[col - 1] + fs[i] * 0.15, 18);
                                if (col < b.heightMap.length - 1) b.heightMap[col + 1] = Math.min(b.heightMap[col + 1] + fs[i] * 0.15, 18);
                                if (col > 1) b.heightMap[col - 2] = Math.min(b.heightMap[col - 2] + fs[i] * 0.05, 18);
                                if (col < b.heightMap.length - 2) b.heightMap[col + 2] = Math.min(b.heightMap[col + 2] + fs[i] * 0.05, 18);

                                spawnFlying(i, width, height, true);
                                landed = true;
                                break;
                            }
                        }
                    }
                }
            }
            if (landed) continue;

            const groundY = getGroundY(fx[i], height);
            if (!storm && fy[i] >= groundY - 2 && fvy[i] > 0) {
                addToGround(fx[i], groundY - rand(0, 3), fs[i], fa[i]);
                spawnFlying(i, width, height, true);
                continue;
            }

            if (fx[i] < -width * 0.15 || fx[i] > width * 1.15 || fy[i] < -height * 0.35 || fy[i] > height + 15) {
                spawnFlying(i, width, height, true);
            }

            if (fk[i] === 1) {
                ctx.strokeStyle = `rgba(${sandColors[i % sandColors.length]}, ${fa[i] * (storm ? 0.75 : 0.55)})`;
                ctx.lineWidth = Math.max(0.6, fs[i] * 0.4);
                ctx.beginPath();
                ctx.moveTo(fx[i], fy[i]);
                ctx.lineTo(fx[i] - fvx[i] * 3.5, fy[i] - fvy[i] * 2.5);
                ctx.stroke();
            } else {
                const s = fs[i];
                ctx.fillStyle = `rgba(${sandColors[i % sandColors.length]}, ${fa[i] * (storm ? 0.96 : 0.85)})`;
                ctx.fillRect(fx[i], fy[i], s, s * 0.7);
            }
        }
    }

    function updateAndDrawTrails(dt, time, width, height) {
        for (let i = 0; i < tCount; i++) {
            sampleWind(tx[i], ty[i], height);
            const drag = 1 - Math.exp(-dt * 0.055);
            tvx[i] += (flowX * td[i] - tvx[i]) * drag;
            tvy[i] += (flowY * td[i] - tvy[i]) * drag;

            tx[i] += tvx[i] * dt * 2.4;
            ty[i] += tvy[i] * dt * 1.8;

            if (tx[i] < -width * 0.15 || tx[i] > width * 1.15 || ty[i] < -height * 0.25 || ty[i] > height * 1.08) {
                spawnTrail(i, width, height, true);
            }

            ctx.strokeStyle = `rgba(230, 200, 155, ${ta[i] * (stormActive ? 1.1 : 0.85)})`;
            ctx.lineWidth = stormActive ? 1.3 : 0.9;
            ctx.beginPath();
            ctx.moveTo(tx[i], ty[i]);
            const length = Math.min(tl[i], Math.hypot(tvx[i], tvy[i]) * 3);
            const angle = Math.atan2(tvy[i], tvx[i]);
            ctx.lineTo(tx[i] - Math.cos(angle) * length, ty[i] - Math.sin(angle) * length);
            ctx.stroke();
        }
    }

    function whirlGroundSand(dt, width, height) {
        liftCarry += dt * stormIntensity * 2.5;
        const toWhirl = Math.min(gCount, Math.floor(liftCarry));
        liftCarry %= 1;
        for (let i = 0; i < toWhirl && gCount > 0; i++) {
            const idx = Math.floor(rand(0, gCount));
            if (fCount >= MAX_FLYING) break;

            fx[fCount] = gx[idx];
            fy[fCount] = gy[idx] - rand(10, 50);
            sampleWind(fx[fCount], fy[fCount], height);
            fvx[fCount] = flowX;
            fvy[fCount] = rand(STORM_LIFT * 0.6, STORM_LIFT * 0.1);
            fs[fCount] = gs[idx];
            fa[fCount] = Math.min(0.95, ga[idx] * 1.2);
            fd[fCount] = rand(0.8, 1.3);
            fk[fCount] = 0;
            fCount++;

            removeGroundHeight(gx[idx], gs[idx] * 1.2);

            gx[idx] = gx[gCount - 1]; gy[idx] = gy[gCount - 1];
            gs[idx] = gs[gCount - 1]; ga[idx] = ga[gCount - 1];
            gd[idx] = gd[gCount - 1];
            gCount--;
        }
    }

    function decayGround(dt) {
        for (let i = 0; i < groundHeight.length; i++) {
            groundHeight[i] *= Math.pow(0.9997, dt);
            if (groundHeight[i] < 0.05) groundHeight[i] = 0;
        }
    }

    function erodeGroundDuringStorm(dt) {
        for (let i = 0; i < groundHeight.length; i++) {
            groundHeight[i] *= Math.pow(0.985, dt * stormIntensity);
            if (groundHeight[i] < 0.3) groundHeight[i] = 0;
        }
    }

    function drawStormHaze(width, height, time) {
        if (!stormActive) return;
        const alpha = stormIntensity * 0.04;

        const grad = ctx.createLinearGradient(0, 0, width, 0);
        grad.addColorStop(0, `rgba(225, 190, 140, 0)`);
        grad.addColorStop(0.35, `rgba(215, 178, 128, ${alpha})`);
        grad.addColorStop(0.65, `rgba(205, 168, 118, ${alpha * 0.7})`);
        grad.addColorStop(1, `rgba(190, 152, 105, 0)`);
        ctx.fillStyle = grad;
        ctx.fillRect(0, 0, width, height);
    }

    // ═══════════════════════════════════════════════════════════════
    //  WEBGL FOG RENDER
    // ═══════════════════════════════════════════════════════════════

    function renderFog(now) {
        if (!fogActive || !gl) return;

        gl.useProgram(fogProgram);
        gl.uniform1f(uTime, weatherTime);
        gl.uniform2f(uRes, fogCanvas.width, fogCanvas.height);
        gl.uniform1f(uStorm, stormIntensity);
        gl.uniform1f(uDrift, windDrift);
        gl.uniform4fv(uEddies, eddies);

        gl.drawArrays(gl.TRIANGLES, 0, 6);
    }

    // ═══════════════════════════════════════════════════════════════
    //  MAIN LOOP
    // ═══════════════════════════════════════════════════════════════

    function render(time) {
        if (!active || !ctx) return;
        const rect = canvas.getBoundingClientRect();
        const width = Math.max(1, Math.round(rect.width));
        const height = Math.max(1, Math.round(rect.height));
        const dt = lastTime ? Math.max(0, Math.min(2.5, (time - lastTime) / (1000 / 60))) : 1;
        lastTime = time;

        updateWeather(dt, time);
        updateEddies(width, height);

        // Update bubble elements & track vertical scrolling/velocity
        updateBubbleBounds(time);
        handleScrollErosion();
        updateBubbleSandPhysics(dt);

        // WebGL fog (rendered first, behind particles)
        renderFog(time);

        // 2D particles
        ctx.clearRect(0, 0, width, height);

        drawClouds(width, height, time);
        updateClouds(dt, width, height);
        drawDustRolls(height);

        drawGroundPile(width, height);
        drawGroundParticles();
        drawBubbleSand();
        updateAndDrawTrails(dt, time, width, height);
        updateAndDrawFlying(dt, time, width, height);

        if (stormActive) {
            whirlGroundSand(dt, width, height);
            erodeGroundDuringStorm(dt);
            drawStormHaze(width, height, time);
        } else {
            decayGround(dt);
        }

        animationId = window.requestAnimationFrame(render);
    }

    function updateWeather(dt, time) {
        if (time >= nextStormTime && !stormActive) {
            stormActive = true;
            stormEndTime = time + STORM_DURATION;
            nextStormTime = time + STORM_DURATION + rand(IDLE_MIN, IDLE_MAX);
        }
        if (stormActive && time >= stormEndTime) {
            stormActive = false;
        }
        const elapsed = time - (stormEndTime - STORM_DURATION);
        const attack = Math.max(0, Math.min(1, elapsed / 1800));
        const release = Math.max(0, Math.min(1, (stormEndTime - time) / 2400));
        stormIntensity = stormActive ? attack * attack * (3 - 2 * attack) * release * release * (3 - 2 * release) : 0;
        weatherTime += dt / 60;
        const gust = Math.pow(Math.max(0, 0.55 + 0.3 * Math.sin(weatherTime * 0.85) + 0.15 * Math.sin(weatherTime * 1.73 + eddyPhases[0])), 3);
        const target = BASE_WIND + gust * 1.6 + Math.sin(weatherTime * 0.37) * 0.25 + stormIntensity * STORM_WIND;
        windSpeed += (target - windSpeed) * (1 - Math.exp(-dt * 0.035));
        windDirection = Math.sin(weatherTime * 0.14 + 1.4);
        windDrift += windSpeed * windDirection * dt * 0.0025;
    }

    function start() {
        if (active || !shouldRun()) return;
        ensureCanvas();
        if (!ctx) return;

        // Try to init WebGL fog (graceful fallback if it fails)
        if (initFogGL()) {
            fogActive = true;
        }

        active = true;
        canvas.style.display = 'block';
        if (fogCanvas) fogCanvas.style.display = 'block';
        resize();
        stormActive = false;
        stormIntensity = 0;
        windSpeed = BASE_WIND;
        windDirection = 1;
        windDrift = weatherTime = liftCarry = 0;
        for (let i = 0; i < 3; i++) eddyPhases[i] = rand(0, Math.PI * 2);
        nextStormTime = performance.now() + rand(5000, 9000);
        lastTime = 0;
        animationId = window.requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
        if (canvas) canvas.style.display = 'none';
        if (fogCanvas) fogCanvas.style.display = 'none';
    }

    function sync() {
        if (shouldRun()) {
            start();
        } else {
            stop();
        }
    }

    // ═══════════════════════════════════════════════════════════════
    //  INIT
    // ═══════════════════════════════════════════════════════════════

    function init() {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', init, { once: true });
            return;
        }

        chatBox = document.getElementById('chat-box');

        if (typeof ResizeObserver !== 'undefined' && chatBox) {
            resizeObserver = new ResizeObserver(() => {
                if (active) resize();
            });
            resizeObserver.observe(chatBox);
        }

        const footer = document.querySelector('.app-footer');
        if (footer && typeof ResizeObserver !== 'undefined') {
            new ResizeObserver(() => { if (active) resize(); }).observe(footer);
        }

        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', () => {
            if (active) resize();
            sync();
        });

        document.addEventListener('visibilitychange', () => {
            if (document.hidden) stop(); else sync();
        });

        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            if (mq.addEventListener) mq.addEventListener('change', sync);
            else if (mq.addListener) mq.addListener(sync);
        }

        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true, attributeFilter: ['data-theme']
            });
        }

        sync();
    }

    window.AuraGoSandstorm = { start, stop, sync };
    init();
})();
