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

        float ripple(vec2 uv, vec2 center, float radius, float frequency, float speed, float phase) {
            vec2 p = uv - center;
            p.x *= u_res.x / max(u_res.y, 1.0);
            float dist = length(p);
            float envelope = exp(-dist * radius);
            float wave = sin(dist * frequency - u_time * speed + phase);
            float rings = smoothstep(0.3, 0.95, wave) + smoothstep(-1.0, -0.5, wave) * 0.5;
            return rings * envelope;
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

        void main() {
            vec2 uv = v_uv;
            float t = u_time;

            float mRipple = ripple(uv, u_mouse, 4.0, 48.0, 1.8, 0.0);
            
            vec2 c1 = vec2(0.24 + sin(t * 0.06) * 0.05, 0.22 + cos(t * 0.08) * 0.03);
            vec2 c2 = vec2(0.76 + cos(t * 0.04) * 0.04, 0.54 + sin(t * 0.05) * 0.04);
            vec2 c3 = vec2(0.48 + sin(t * 0.03) * 0.03, 0.82 + cos(t * 0.07) * 0.03);
            vec2 c4 = vec2(0.58 + cos(t * 0.05) * 0.03, 0.34 + sin(t * 0.04) * 0.03);

            float r1 = ripple(uv, c1, 4.8, 54.0, 0.85, 0.3);
            float r2 = ripple(uv, c2, 4.2, 47.0, 0.68, 1.6);
            float r3 = ripple(uv, c3, 5.0, 44.0, 0.76, 2.8);
            float r4 = ripple(uv, c4, 5.2, 50.0, 0.62, 4.1);
            float rippleField = r1 + r2 + r3 + r4 + mRipple * 0.45;

            float causticA = caustics(uv, 3.8, 0.15);
            float causticB = caustics(uv + vec2(2.5, -1.8), 5.2, 0.10);
            float causticField = causticA * 0.65 + causticB * 0.35;

            float current1 = fbm(uv * 3.5 - vec2(t * 0.02, t * 0.015));
            float current2 = fbm(uv * 7.0 + vec2(t * 0.01, -t * 0.02));
            float currentField = current1 * 0.7 + current2 * 0.3;

            float s1 = lightShaft(uv, 0.35, 0.05, 0.25, 0.15);
            float s2 = lightShaft(uv, 0.42, 0.03, 0.35, 0.20);
            float s3 = lightShaft(uv, 0.50, 0.02, 0.30, 0.18);
            float shaftField = s1 * 0.5 + s2 * 0.3 + s3 * 0.2;

            float shimmer = sin((uv.y + causticField * 0.1) * 32.0 - t * 0.35) * 0.5 + 0.5;
            float bubbles = bubbleField(uv);

            float mouseGlow = smoothstep(0.24, 0.0, length((uv - u_mouse) * vec2(u_res.x / max(u_res.y, 1.0), 1.0)));

            vec3 sapphire = vec3(0.02, 0.15, 0.28);
            vec3 neonAqua = vec3(0.18, 0.82, 0.94);
            vec3 seafoam  = vec3(0.62, 0.95, 0.90);
            vec3 indigo   = vec3(0.04, 0.08, 0.16);
            vec3 glowColor = vec3(0.24, 0.90, 0.82);

            vec3 color = indigo * 0.3;
            color += sapphire * (0.4 + currentField * 0.15 + causticField * 0.1);
            color += neonAqua * (0.05 + causticField * 0.22 + rippleField * 0.12);
            color += seafoam * (shimmer * 0.05 + causticField * 0.08 + rippleField * 0.08);
            color += glowColor * (shaftField * 0.28 + mouseGlow * 0.18 + bubbles * 0.45);

            float alpha = 0.08 +
                          causticField * 0.15 +
                          rippleField * 0.12 +
                          currentField * 0.05 +
                          shaftField * 0.12 +
                          mouseGlow * 0.12 +
                          bubbles * 0.32;

            float horizon = smoothstep(0.0, 0.08, uv.y) * (1.0 - smoothstep(0.88, 1.00, uv.y));
            float sideFade = smoothstep(0.0, 0.06, uv.x) * smoothstep(0.0, 0.06, 1.0 - uv.x);
            alpha *= horizon * sideFade;
            alpha = clamp(alpha, 0.0, 0.38);

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

        gl.uniform1f(uniforms.time, time * 0.001);
        gl.uniform2f(uniforms.resolution, canvas.width, canvas.height);
        gl.uniform2f(uniforms.mouse, mouse.x, mouse.y);
        
        gl.clearColor(0, 0, 0, 0);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
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
                    mouse.targetX = (e.clientX - rect.left) / rect.width;
                    mouse.targetY = 1.0 - (e.clientY - rect.top) / rect.height;
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

