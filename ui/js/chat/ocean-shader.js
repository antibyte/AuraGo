(function () {
    'use strict';

    let canvas;
    let gl;
    let program;
    let animationId = null;
    let active = false;
    let uniforms = {};
    let chatBox;
    let resizeObserver = null;
    let mouse = { x: 0.5, y: 0.5, targetX: 0.5, targetY: 0.5 };
    const maxTrail = 6;
    let trail = Array.from({ length: maxTrail }, () => ({ x: 0.5, y: 0.5, age: 0.0 }));
    let turbEl = null;

    const VERT = `
        attribute vec2 a_pos;
        varying vec2 v_uv;

        void main() {
            v_uv = a_pos * 0.5 + 0.5;
            gl_Position = vec4(a_pos, 0.0, 1.0);
        }
    `;

    const FRAG = `
        precision mediump float;
        varying vec2 v_uv;
        uniform float u_time;
        uniform vec2 u_res;
        uniform vec2 u_mouse;
        uniform vec3 u_mouseTrail[6];

        float hash(vec2 p) {
            return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
        }

        vec2 hash2(vec2 p) {
            p = vec2(dot(p, vec2(127.1, 311.7)), dot(p, vec2(269.5, 183.3)));
            return fract(sin(p) * 43758.5453123);
        }

        float noise(vec2 p) {
            vec2 i = floor(p);
            vec2 f = fract(p);
            float a = hash(i);
            float b = hash(i + vec2(1.0, 0.0));
            float c = hash(i + vec2(0.0, 1.0));
            float d = hash(i + vec2(1.0, 1.0));
            vec2 u = f * f * (3.0 - 2.0 * f);
            return mix(a, b, u.x) + (c - a) * u.y * (1.0 - u.x) + (d - b) * u.x * u.y;
        }

        float fbm(vec2 p) {
            float v = 0.0;
            float a = 0.5;
            for (int i = 0; i < 4; i++) {
                v += a * noise(p);
                p = p * 2.05 + vec2(4.1, 7.3);
                a *= 0.5;
            }
            return v;
        }

        float voronoi(vec2 p) {
            vec2 n = floor(p);
            vec2 f = fract(p);
            float md = 8.0;
            for (int j = -1; j <= 1; j++) {
                for (int i = -1; i <= 1; i++) {
                    vec2 g = vec2(float(i), float(j));
                    vec2 o = hash2(n + g);
                    o = 0.5 + 0.5 * sin(u_time * 0.3 + 6.2831 * o);
                    vec2 r = g + o - f;
                    float d = dot(r, r);
                    md = min(md, d);
                }
            }
            return md;
        }

        float caustics(vec2 uv, float scale, float speed) {
            float t = u_time * speed;
            vec2 flow = vec2(
                fbm(uv * scale * 0.4 + vec2(t, -t * 0.5)),
                fbm(uv * scale * 0.4 + vec2(-t * 0.5, t))
            );
            vec2 p = uv * scale + flow * 0.7;
            float v1 = voronoi(p + vec2(t * 0.1, t * 0.07));
            float v2 = voronoi(p * 1.3 - vec2(t * 0.05, t * 0.1) + 8.0);
            float c = sqrt(v1) * sqrt(v2);
            return smoothstep(0.02, 0.38, c);
        }

        float lightShaft(vec2 uv, float angle, float width, float speed, float sway) {
            float c = cos(angle);
            float s = sin(angle);
            vec2 ruv = vec2(uv.x * c - uv.y * s, uv.x * s + uv.y * c);
            float swayAmt = sin(u_time * sway) * 0.05;
            float ray = smoothstep(width, 0.0, abs(ruv.x - 0.2 + swayAmt));
            float falloff = smoothstep(1.2, 0.1, ruv.y);
            float shimmer = sin(ruv.y * 12.0 - u_time * speed) * 0.3 + 0.7;
            float flicker = sin(u_time * speed * 1.5 + ruv.y * 5.0) * 0.15 + 0.85;
            return ray * falloff * shimmer * flicker;
        }

        float bubbleField(vec2 uv) {
            float field = 0.0;
            vec2 st = uv * vec2(10.0, 5.0);
            vec2 ipos = floor(st);
            vec2 fpos = fract(st);
            
            for (int y = -1; y <= 1; y++) {
                for (int x = -1; x <= 1; x++) {
                    vec2 neighbor = vec2(float(x), float(y));
                    vec2 cellId = ipos + neighbor;
                    vec2 r = hash2(cellId);
                    
                    float size = 0.008 + r.x * 0.022;
                    float speed = 0.2 + r.y * 0.3;
                    float phase = r.x * 6.28;
                    
                    float yOffset = mod(u_time * speed + r.y, 1.2) - 0.1;
                    vec2 center = vec2(0.5 + 0.3 * sin(u_time * 0.5 + phase), yOffset);
                    
                    vec2 diff = fpos - neighbor - center;
                    diff.x *= u_res.x / max(u_res.y, 1.0);
                    
                    float dist = length(diff);
                    if (dist < size) {
                        float outline = smoothstep(size, size * 0.8, dist) * (1.0 - smoothstep(size * 0.8, size * 0.3, dist));
                        float highlight = smoothstep(size * 0.5, 0.0, length(diff - vec2(-size * 0.28, size * 0.28))) * 0.7;
                        float visible = smoothstep(0.0, 0.15, yOffset) * smoothstep(1.1, 0.9, yOffset);
                        field += (outline * 0.7 + highlight) * visible;
                    }
                }
            }
            return field;
        }

        float marineSnow(vec2 uv) {
            float field = 0.0;
            vec2 st = uv * vec2(18.0, 9.0);
            vec2 ipos = floor(st);
            vec2 fpos = fract(st);
            
            for (int y = -1; y <= 1; y++) {
                for (int x = -1; x <= 1; x++) {
                    vec2 neighbor = vec2(float(x), float(y));
                    vec2 cellId = ipos + neighbor;
                    vec2 r = hash2(cellId);
                    
                    float size = 0.003 + r.x * 0.005;
                    float speed = 0.06 + r.y * 0.08;
                    float phase = r.x * 6.28;
                    
                    float yOffset = mod(u_time * speed + r.y, 1.3) - 0.15;
                    vec2 center = vec2(0.5 + 0.35 * sin(u_time * 0.25 + phase), yOffset);
                    
                    vec2 diff = fpos - neighbor - center;
                    diff.x *= u_res.x / max(u_res.y, 1.0);
                    
                    float dist = length(diff);
                    if (dist < size) {
                        float glow = smoothstep(size, 0.0, dist) * (0.35 + 0.65 * sin(u_time * 2.5 + phase));
                        float visible = smoothstep(0.0, 0.12, yOffset) * smoothstep(1.2, 1.0, yOffset);
                        field += glow * visible;
                    }
                }
            }
            return field;
        }

        float seaweed(vec2 uv, float xPos, float height, float frequency, float speed, float phase) {
            if (uv.y > height) return 0.0;
            float factor = uv.y / height;
            float wave = sin(uv.y * frequency - u_time * speed + phase) * 0.022 * factor;
            float stemWidth = 0.016 * (1.0 - factor * 0.6);
            float dist = abs(uv.x - xPos - wave);
            float edge = smoothstep(stemWidth, stemWidth * 0.5, dist);
            float fade = smoothstep(height, height * 0.8, uv.y);
            return edge * (1.0 - fade);
        }

        float getTrailField(vec2 uv) {
            float field = 0.0;
            for (int i = 0; i < 6; i++) {
                vec3 pt = u_mouseTrail[i];
                vec2 diff = uv - pt.xy;
                diff.x *= u_res.x / max(u_res.y, 1.0);
                float dist = length(diff);
                float glow = smoothstep(0.12, 0.0, dist) * pt.z;
                field = max(field, glow);
            }
            return field;
        }

        void main() {
            vec2 uv = v_uv;
            float t = u_time;

            // Caustics Chromatic Aberration
            float causticA_R = caustics(uv + vec2(0.003, 0.0), 3.8, 0.15);
            float causticA_G = caustics(uv, 3.8, 0.15);
            float causticA_B = caustics(uv - vec2(0.003, 0.0), 3.8, 0.15);

            float causticB_R = caustics(uv + vec2(2.503, -1.8), 5.2, 0.10);
            float causticB_G = caustics(uv + vec2(2.5, -1.8), 5.2, 0.10);
            float causticB_B = caustics(uv - vec2(2.497, -1.8), 5.2, 0.10);

            vec3 causticColor = vec3(
                causticA_R * 0.65 + causticB_R * 0.35,
                causticA_G * 0.65 + causticB_G * 0.35,
                causticA_B * 0.65 + causticB_B * 0.35
            );

            float current1 = fbm(uv * 3.5 - vec2(t * 0.02, t * 0.015));
            float current2 = fbm(uv * 7.0 + vec2(t * 0.01, -t * 0.02));
            float currentField = current1 * 0.7 + current2 * 0.3;

            // Volumetric distorted god rays
            float rayDistort1 = fbm(uv * 1.5 + vec2(t * 0.05, -t * 0.02));
            float rayDistort2 = fbm(uv * 3.0 - vec2(t * 0.03, t * 0.04));
            vec2 rD = vec2(rayDistort1, rayDistort2) * 0.04;

            float s1 = lightShaft(uv + rD, 0.35, 0.05, 0.25, 0.15);
            float s2 = lightShaft(uv + rD, 0.42, 0.03, 0.35, 0.20);
            float s3 = lightShaft(uv + rD, 0.50, 0.02, 0.30, 0.18);
            float shaftField = s1 * 0.5 + s2 * 0.3 + s3 * 0.2;

            float shimmer = sin((uv.y + causticColor.g * 0.1) * 32.0 - t * 0.35) * 0.5 + 0.5;
            float bubbles = bubbleField(uv);
            float snow = marineSnow(uv);
            float trailField = getTrailField(uv);

            float mouseGlow = smoothstep(0.20, 0.0, length((uv - u_mouse) * vec2(u_res.x / max(u_res.y, 1.0), 1.0)));

            vec3 sapphire = vec3(0.02, 0.15, 0.28);
            vec3 neonAqua = vec3(0.18, 0.82, 0.94);
            vec3 seafoam  = vec3(0.62, 0.95, 0.90);
            vec3 indigo   = vec3(0.04, 0.08, 0.16);
            vec3 glowColor = vec3(0.24, 0.90, 0.82);

            vec3 color = indigo * 0.1;
            color += sapphire * (0.4 + currentField * 0.08);
            color += neonAqua * (0.05 + causticColor * 0.3);
            color += seafoam * (shimmer * 0.06 + causticColor * 0.15);
            color += glowColor * (shaftField * 0.48 + mouseGlow * 0.18 + bubbles * 0.4);
            
            color += vec3(0.7, 0.95, 0.92) * snow * 0.6;
            color += glowColor * trailField * 0.85;

            // Waving Seaweed Silhouettes
            float sw1 = seaweed(uv, 0.06, 0.35, 12.0, 1.4, 0.0);
            float sw2 = seaweed(uv, 0.12, 0.25, 14.0, 1.8, 1.5);
            float sw3 = seaweed(uv, 0.88, 0.40, 10.0, 1.2, 3.1);
            float sw4 = seaweed(uv, 0.94, 0.30, 15.0, 1.6, 4.2);
            float seaweedField = sw1 + sw2 + sw3 + sw4;
            vec3 seaweedColor = vec3(0.01, 0.06, 0.1) * (0.5 + 0.5 * uv.y);
            color = mix(color, seaweedColor, seaweedField * 0.85);

            float alpha = 0.03 +
                          causticColor.g * 0.22 +
                          shaftField * 0.20 +
                          mouseGlow * 0.14 +
                          bubbles * 0.24 +
                          snow * 0.30 +
                          trailField * 0.35 +
                          seaweedField * 0.85;

            float horizon = smoothstep(0.0, 0.08, uv.y) * (1.0 - smoothstep(0.88, 1.00, uv.y));
            float sideFade = smoothstep(0.0, 0.06, uv.x) * smoothstep(0.0, 0.06, 1.0 - uv.x);
            alpha *= horizon * sideFade;
            alpha = clamp(alpha, 0.0, 0.32);

            gl_FragColor = vec4(color, alpha);
        }
    `;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'ocean' &&
            !prefersReducedMotion() &&
            window.innerWidth >= 520;
    }

    function createShader(type, source) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);

        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            console.warn('[OceanShader] Shader compile error:', gl.getShaderInfoLog(shader));
            gl.deleteShader(shader);
            return null;
        }

        return shader;
    }

    function createCanvas() {
        canvas = document.createElement('canvas');
        canvas.id = 'ocean-overlay';
        Object.assign(canvas.style, {
            position: 'fixed',
            top: '0',
            left: '0',
            width: '0',
            height: '0',
            pointerEvents: 'none',
            zIndex: '2',
            opacity: '1',
            mixBlendMode: 'screen',
            display: 'none'
        });
        document.body.appendChild(canvas);
    }

    function initGL() {
        if (!canvas) createCanvas();

        gl = canvas.getContext('webgl', {
            alpha: true,
            antialias: false,
            premultipliedAlpha: false,
            powerPreference: 'high-performance'
        });

        if (!gl) {
            console.warn('[OceanShader] WebGL not available');
            return false;
        }

        const vertexShader = createShader(gl.VERTEX_SHADER, VERT);
        const fragmentShader = createShader(gl.FRAGMENT_SHADER, FRAG);
        if (!vertexShader || !fragmentShader) {
            return false;
        }

        program = gl.createProgram();
        gl.attachShader(program, vertexShader);
        gl.attachShader(program, fragmentShader);
        gl.linkProgram(program);

        if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
            console.warn('[OceanShader] Program link error:', gl.getProgramInfoLog(program));
            return false;
        }

        gl.useProgram(program);

        const quad = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, quad);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([
            -1, -1,
             1, -1,
            -1,  1,
             1,  1
        ]), gl.STATIC_DRAW);

        const position = gl.getAttribLocation(program, 'a_pos');
        gl.enableVertexAttribArray(position);
        gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0);

        uniforms.time = gl.getUniformLocation(program, 'u_time');
        uniforms.resolution = gl.getUniformLocation(program, 'u_res');
        uniforms.mouse = gl.getUniformLocation(program, 'u_mouse');
        uniforms.mouseTrail = gl.getUniformLocation(program, 'u_mouseTrail');

        gl.enable(gl.BLEND);
        gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA);
        return true;
    }

    function updateBounds() {
        if (!canvas || !chatBox) return false;
        const rect = chatBox.getBoundingClientRect();
        if (!rect.width || !rect.height) {
            canvas.style.width = '0';
            canvas.style.height = '0';
            return false;
        }

        canvas.style.left = `${Math.round(rect.left)}px`;
        canvas.style.top = `${Math.round(rect.top)}px`;
        canvas.style.width = `${Math.round(rect.width)}px`;
        canvas.style.height = `${Math.round(rect.height)}px`;
        return true;
    }

    function resize() {
        if (!canvas || !gl || !updateBounds()) return;
        const dpr = Math.min(window.devicePixelRatio || 1, 2);
        const width = Math.max(1, Math.round(canvas.getBoundingClientRect().width));
        const height = Math.max(1, Math.round(canvas.getBoundingClientRect().height));
        canvas.width = Math.floor(width * dpr);
        canvas.height = Math.floor(height * dpr);
        gl.viewport(0, 0, canvas.width, canvas.height);
    }

    function render(time) {
        if (!active || !gl) return;

        mouse.x += (mouse.targetX - mouse.x) * 0.06;
        mouse.y += (mouse.targetY - mouse.y) * 0.06;

        for (let i = 0; i < maxTrail; i++) {
            trail[i].age *= 0.95;
        }

        const trailArray = new Float32Array(maxTrail * 3);
        for (let i = 0; i < maxTrail; i++) {
            trailArray[i * 3] = trail[i].x;
            trailArray[i * 3 + 1] = trail[i].y;
            trailArray[i * 3 + 2] = trail[i].age;
        }

        gl.uniform1f(uniforms.time, time * 0.001);
        gl.uniform2f(uniforms.resolution, canvas.width, canvas.height);
        gl.uniform2f(uniforms.mouse, mouse.x, mouse.y);
        gl.uniform3fv(uniforms.mouseTrail, trailArray);
        
        gl.clearColor(0, 0, 0, 0);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);

        if (!turbEl) {
            turbEl = document.getElementById('ocean-refraction-turbulence');
        }
        if (turbEl) {
            const t = time * 0.0006;
            const bfX = 0.006 + Math.sin(t) * 0.001;
            const bfY = 0.009 + Math.cos(t * 0.8) * 0.0015;
            turbEl.setAttribute('baseFrequency', `${bfX} ${bfY}`);
        }

        animationId = window.requestAnimationFrame(render);
    }

    function start() {
        if (active || !shouldRun()) return;
        if (!gl && !initGL()) return;

        active = true;
        canvas.style.display = 'block';
        resize();
        animationId = window.requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
        if (canvas) {
            canvas.style.display = 'none';
        }
    }

    function sync() {
        if (shouldRun()) {
            start();
        } else {
            stop();
        }
    }

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

        if (chatBox) {
            chatBox.addEventListener('mousemove', (e) => {
                if (!active) return;
                const rect = chatBox.getBoundingClientRect();
                if (rect.width > 0 && rect.height > 0) {
                    const x = (e.clientX - rect.left) / rect.width;
                    const y = 1.0 - (e.clientY - rect.top) / rect.height;
                    mouse.targetX = x;
                    mouse.targetY = y;

                    for (let i = maxTrail - 1; i > 0; i--) {
                        trail[i].x = trail[i - 1].x;
                        trail[i].y = trail[i - 1].y;
                        trail[i].age = trail[i - 1].age;
                    }
                    trail[0].x = x;
                    trail[0].y = y;
                    trail[0].age = 1.0;
                }
            });
            chatBox.addEventListener('mouseleave', () => {
                mouse.targetX = 0.5;
                mouse.targetY = 0.5;
            });
        }

        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', () => {
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

        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true,
                attributeFilter: ['data-theme']
            });
        }

        sync();
    }

    window.AuraGoOcean = { start, stop, sync };
    init();
})();
