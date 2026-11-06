/* Aura Drift - optional WebGL2 post-processing over the Canvas 2D world. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    const VERTEX = `#version 300 es
in vec2 aPos;
out vec2 vUv;
void main() {
    vUv = aPos * 0.5 + 0.5;
    gl_Position = vec4(aPos, 0.0, 1.0);
}`;

    const FRAGMENT = `#version 300 es
precision highp float;
in vec2 vUv;
out vec4 outColor;
uniform sampler2D uScene;
uniform vec2 uTexel;
uniform vec3 uGrade;
uniform float uTime;
uniform float uBloom;
uniform float uWet;
uniform float uHeat;
uniform float uRain;
                uniform float uImpact;
                uniform float uBoost;
                uniform float uDamage;
                uniform float uReduced;
                uniform float uSpeed;

vec3 sane(vec3 c) {
    if (!(c.r == c.r)) c.r = 0.0;
    if (!(c.g == c.g)) c.g = 0.0;
    if (!(c.b == c.b)) c.b = 0.0;
    return min(max(c, vec3(0.0)), vec3(4.0));
}

float saneF(float v) {
    if (!(v == v)) v = 0.0;
    return clamp(v, 0.0, 4.0);
}

float luma(vec3 c) { return dot(c, vec3(0.2126, 0.7152, 0.0722)); }

float grain(vec2 p) {
    return fract(sin(dot(p, vec2(12.9898, 78.233))) * 43758.5453) - 0.5;
}

void main() {
    vec2 uv = vUv;
    vec2 radial = uv - 0.5;
    float shift = (uImpact * 0.004 + uBoost * 0.0015) * (1.0 - uReduced);
    vec3 base;
    base.r = texture(uScene, uv + radial * shift).r;
    base.g = texture(uScene, uv).g;
    base.b = texture(uScene, uv - radial * shift).b;
    base = sane(base);

    /* Speed streaks: a short radial blur away from the centre, only while
       really moving, so the racing line stays readable. */
    if (uSpeed > 0.25 && uReduced < 0.5) {
        vec3 smear = vec3(0.0);
        float wsum = 0.0;
        for (int i = 1; i <= 3; i++) {
            float d = float(i) * 0.004 * uSpeed;
            float w = 1.0 / float(i);
            smear += sane(texture(uScene, uv - radial * d).rgb) * w;
            wsum += w;
        }
        base = mix(base, smear / wsum, clamp(uSpeed * 0.35, 0.0, 0.4));
    }

    /* Bright-pass bloom: twelve taps on two rings, clamped so a bad frame
       cannot blow out. */
    vec3 bloom = vec3(0.0);
    if (uBloom > 0.0) {
        for (int i = 0; i < 12; i++) {
            float ang = float(i) * 0.5236;
            float rad = (i < 6) ? 2.0 : 5.0;
            vec2 off = vec2(cos(ang), sin(ang)) * uTexel * rad;
            vec3 s = sane(texture(uScene, uv + off).rgb);
            bloom += max(s - vec3(0.62), vec3(0.0));
        }
        bloom /= 12.0;
    }

    /* Wet asphalt: stretch bright highlights vertically into short reflections. */
    vec3 wet = vec3(0.0);
    if (uWet > 0.0) {
        float wobble = sin(uv.y * 220.0 + uTime * 2.2) * 0.35 * (1.0 - uReduced);
        for (int i = 1; i <= 5; i++) {
            float d = float(i);
            vec2 off = vec2(wobble * uTexel.x * d, uTexel.y * d * 7.0);
            vec3 s = sane(texture(uScene, uv + off).rgb);
            wet += max(s - vec3(0.55), vec3(0.0)) * (1.0 / (d * d));
        }
        wet *= vec3(0.72, 0.86, 1.0);
    }

    vec3 color = base + bloom * uBloom + wet * uWet * 0.55;

    /* Heat shimmer: the air itself moves over a hot surface. */
    if (uHeat > 0.001) {
        float wob = sin(uv.y * 88.0 + uTime * 3.1) * 0.0018 * uHeat;
        color.r = saneF(texture(uScene, uv + vec2(wob, 0.0)).r);
        color.g = saneF(texture(uScene, uv + vec2(wob * 0.6, 0.0)).g);
        color.b = saneF(texture(uScene, uv + vec2(wob * 1.3, 0.0)).b);
        color = sane(color);
    }

    /* Rain on the lens: a sparse grid of drops that bend the scene. */
    if (uRain > 0.001) {
        vec2 grid = vec2(15.0, 10.0);
        vec2 cell = floor(uv * grid);
        float h = fract(sin(dot(cell, vec2(12.9898, 78.233))) * 43758.5453);
        if (h > 1.0 - uRain * 0.18) {
            vec2 local = fract(uv * grid) - 0.5;
            float d = length(local);
            vec2 off = local * d * 0.07;
            vec3 through = sane(texture(uScene, uv - off).rgb);
            color = mix(color, through, clamp(1.0 - d * 1.6, 0.0, 1.0));
            color += vec3(0.04, 0.06, 0.08) * clamp(1.0 - d * 2.2, 0.0, 1.0);
        }
    }

    color *= uGrade;
    float vignette = 1.0 - 0.22 * smoothstep(0.35, 0.78, length(radial));
    color *= vignette;
    float wound = uDamage * (0.5 + 0.5 * sin(uTime * 5.2));
    float rim = smoothstep(0.42, 0.78, length(radial));
    color = mix(color, vec3(0.55, 0.05, 0.04), rim * wound * 0.35);
    if (uReduced < 0.5) {
        color += grain(uv * 720.0 + fract(uTime) * 41.0) * 0.012;
    }
    outColor = vec4(sane(min(color, vec3(1.0))), 1.0);
}`;

    const GRADES = {
        alpine: [0.96, 1.00, 0.98],
        desert: [1.06, 0.99, 0.90],
        harbour: [0.88, 0.96, 1.12],
        salt: [1.04, 1.04, 1.00],
        forest: [0.94, 1.02, 0.94],
        industry: [1.05, 0.98, 0.92],
        glacier: [0.86, 0.95, 1.16],
        metro: [1.02, 0.95, 1.10]
    };

    function compile(gl, type, source) {
        const shader = gl.createShader(type);
        gl.shaderSource(shader, source);
        gl.compileShader(shader);
        if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
            const info = gl.getShaderInfoLog(shader);
            gl.deleteShader(shader);
            throw new Error(info || 'shader compile failed');
        }
        return shader;
    }

    AD.createPostProcessor = function (sourceCanvas, outputCanvas) {
        let gl = null, program = null, texture = null, buffer = null;
        let available = false, lost = false, quality = 'high', biome = 'alpine';
        let uniforms = {}, time = 0;
        let textureWidth = 0, textureHeight = 0;

        function hide() {
            outputCanvas.style.opacity = '0';
            outputCanvas.style.pointerEvents = 'none';
        }
        function show() {
            outputCanvas.style.opacity = '1';
        }

        function init() {
            try {
                gl = outputCanvas.getContext('webgl2', {
                    alpha: false, antialias: false, depth: false,
                    stencil: false, preserveDrawingBuffer: false,
                    powerPreference: 'low-power'
                });
            } catch (_) { gl = null; }
            if (!gl) { available = false; hide(); return; }

            let vertex, fragment;
            try {
                vertex = compile(gl, gl.VERTEX_SHADER, VERTEX);
                fragment = compile(gl, gl.FRAGMENT_SHADER, FRAGMENT);
                program = gl.createProgram();
                gl.attachShader(program, vertex);
                gl.attachShader(program, fragment);
                gl.linkProgram(program);
                if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
                    throw new Error(gl.getProgramInfoLog(program) || 'link failed');
                }
            } catch (err) {
                if (vertex) gl.deleteShader(vertex);
                if (fragment) gl.deleteShader(fragment);
                if (program) gl.deleteProgram(program);
                program = null; available = false; hide();
                return;
            }
            gl.deleteShader(vertex);
            gl.deleteShader(fragment);

            buffer = gl.createBuffer();
            gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
            gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
            const location = gl.getAttribLocation(program, 'aPos');
            gl.enableVertexAttribArray(location);
            gl.vertexAttribPointer(location, 2, gl.FLOAT, false, 0, 0);

            texture = gl.createTexture();
            gl.bindTexture(gl.TEXTURE_2D, texture);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);

            for (const name of ['uScene', 'uTexel', 'uGrade', 'uTime', 'uBloom', 'uWet', 'uHeat', 'uRain',
                'uImpact', 'uBoost', 'uDamage', 'uReduced', 'uSpeed']) {
                uniforms[name] = gl.getUniformLocation(program, name);
            }
            gl.useProgram(program);
            gl.uniform1i(uniforms.uScene, 0);
            available = true;
            lost = false;
            textureWidth = 0; textureHeight = 0;
        }

        function onLost(event) {
            event.preventDefault();
            lost = true; available = false; hide();
        }
        function onRestored() {
            if (!lost) return;
            init();
        }
        outputCanvas.addEventListener('webglcontextlost', onLost);
        outputCanvas.addEventListener('webglcontextrestored', onRestored);
        hide();
        init();

        return {
            get available() { return available; },
            get enabled() { return available && (quality === 'high' || quality === 'ultra'); },
            resize(width, height, ratio) {
                const dpr = clamp(ratio || 1, 1, 2);
                outputCanvas.width = Math.max(1, Math.round(width * dpr));
                outputCanvas.height = Math.max(1, Math.round(height * dpr));
                outputCanvas.style.width = width + 'px';
                outputCanvas.style.height = height + 'px';
                if (gl) gl.viewport(0, 0, outputCanvas.width, outputCanvas.height);
                textureWidth = 0; textureHeight = 0;
            },
            setQuality(name) { quality = AD.QUALITY[name] ? name : 'high'; },
            setBiome(id) { biome = GRADES[id] ? id : 'alpine'; },
            render(race, dt, reducedMotion) {
                if (!this.enabled) { hide(); return false; }
                if (!gl || gl.isContextLost()) { available = false; hide(); return false; }
                const sw = sourceCanvas.width, sh = sourceCanvas.height;
                if (sw < 1 || sh < 1) return false;
                time += dt;
                gl.useProgram(program);
                gl.activeTexture(gl.TEXTURE0);
                gl.bindTexture(gl.TEXTURE_2D, texture);
                if (sw !== textureWidth || sh !== textureHeight) {
                    gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA8, sw, sh, 0, gl.RGBA, gl.UNSIGNED_BYTE, null);
                    textureWidth = sw; textureHeight = sh;
                }
                gl.texSubImage2D(gl.TEXTURE_2D, 0, 0, 0, gl.RGBA, gl.UNSIGNED_BYTE, sourceCanvas);
                gl.uniform2f(uniforms.uTexel, 1 / sw, 1 / sh);
                const grade = GRADES[biome] || GRADES.alpine;
                gl.uniform3f(uniforms.uGrade, grade[0], grade[1], grade[2]);
                gl.uniform1f(uniforms.uTime, time);
                gl.uniform1f(uniforms.uBloom, quality === 'ultra' ? 0.95 : 0.55);
                /* Live weather drives the wet pass: rain darkens the asphalt
                   and puts water on the lens, heat makes the air move. */
                const conditions = race && race.conditions ? race.conditions.state : null;
                const kind = conditions ? conditions.kind : '';
                const intensity = conditions ? conditions.intensity : 0;
                const wetBase = biome === 'harbour' ? 0.75 : (biome === 'industry' ? 0.35 :
                    (biome === 'metro' ? 0.3 : 0.0));
                const wetNow = kind === 'rain' ? wetBase + intensity * 0.5 : wetBase;
                gl.uniform1f(uniforms.uWet, clamp(wetNow, 0, 1.4));
                gl.uniform1f(uniforms.uHeat, kind === 'heat' ? intensity : 0);
                gl.uniform1f(uniforms.uRain, kind === 'rain' ? intensity : 0);
                const v = race && race.cars[0] ? race.cars[0].v : null;
                gl.uniform1f(uniforms.uImpact, v ? clamp(v.impact, 0, 1) : 0);
                gl.uniform1f(uniforms.uBoost, v ? v.boost : 0);
                gl.uniform1f(uniforms.uDamage, v ? clamp(v.damage / 100, 0, 1) : 0);
                gl.uniform1f(uniforms.uReduced, reducedMotion ? 1 : 0);
                gl.uniform1f(uniforms.uSpeed, v ? clamp(Math.max(0, v.vx) / 80, 0, 1) : 0);
                gl.drawArrays(gl.TRIANGLES, 0, 3);
                show();
                return true;
            },
            stats() {
                return { available: available, enabled: this.enabled, biome: biome, quality: quality, lost: lost };
            },
            dispose() {
                outputCanvas.removeEventListener('webglcontextlost', onLost);
                outputCanvas.removeEventListener('webglcontextrestored', onRestored);
                if (gl) {
                    if (texture) gl.deleteTexture(texture);
                    if (buffer) gl.deleteBuffer(buffer);
                    if (program) gl.deleteProgram(program);
                    const ext = gl.getExtension('WEBGL_lose_context');
                    if (ext) ext.loseContext();
                }
                gl = null; program = null; texture = null; buffer = null;
                available = false; hide();
            }
        };
    };
})();
