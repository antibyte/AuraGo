(function () {
    'use strict';

    if (window.AuraScreensaverGL) return;

    const FULLSCREEN_VS = `#version 300 es
out vec2 vUv;
void main() {
    vec2 p = vec2(float((gl_VertexID << 1) & 2), float(gl_VertexID & 2));
    vUv = p;
    gl_Position = vec4(p * 2.0 - 1.0, 0.0, 1.0);
}`;

    const FS_HEADER = `#version 300 es
precision highp float;
precision highp int;
precision highp sampler2D;
in vec2 vUv;
out vec4 fragColor;
`;

    // Original noise/post helpers. Integer hashing follows the public PCG construction
    // described in Jarzynski & Olano, "Hash Functions for GPU Rendering" (JCGT 2020).
    const GLSL_COMMON = `
#define PI 3.14159265359
#define TAU 6.28318530718
uint pcgHash(uint v) {
    uint state = v * 747796405u + 2891336453u;
    uint word = ((state >> ((state >> 28u) + 4u)) ^ state) * 277803737u;
    return (word >> 22u) ^ word;
}
uint pcgHash3(uvec3 v) { return pcgHash(v.x + pcgHash(v.y + pcgHash(v.z))); }
float hashCell(ivec3 c) { return float(pcgHash3(uvec3(c))) * (1.0 / 4294967295.0); }
float hash12(vec2 p) { return float(pcgHash3(uvec3(floatBitsToUint(p.x), floatBitsToUint(p.y), 1u))) * (1.0 / 4294967295.0); }
float hash13(vec3 p) { return float(pcgHash3(floatBitsToUint(p))) * (1.0 / 4294967295.0); }
vec3 hash33(vec3 p) {
    uvec3 u = floatBitsToUint(p);
    uint h = pcgHash3(u);
    return vec3(float(h), float(pcgHash(h)), float(pcgHash(h ^ 0x9e3779b9u))) * (1.0 / 4294967295.0);
}
vec3 cellGradient(ivec3 c) {
    uint h = pcgHash3(uvec3(c));
    float z = float(h & 0xffffu) * (2.0 / 65535.0) - 1.0;
    float a = float(h >> 16u) * (TAU / 65535.0);
    float r = sqrt(max(0.0, 1.0 - z * z));
    return vec3(r * cos(a), r * sin(a), z);
}
float gnoise(vec3 p) {
    vec3 fl = floor(p);
    ivec3 i = ivec3(fl);
    vec3 f = p - fl;
    vec3 u = f * f * f * (f * (f * 6.0 - 15.0) + 10.0);
    float n000 = dot(cellGradient(i), f);
    float n100 = dot(cellGradient(i + ivec3(1, 0, 0)), f - vec3(1, 0, 0));
    float n010 = dot(cellGradient(i + ivec3(0, 1, 0)), f - vec3(0, 1, 0));
    float n110 = dot(cellGradient(i + ivec3(1, 1, 0)), f - vec3(1, 1, 0));
    float n001 = dot(cellGradient(i + ivec3(0, 0, 1)), f - vec3(0, 0, 1));
    float n101 = dot(cellGradient(i + ivec3(1, 0, 1)), f - vec3(1, 0, 1));
    float n011 = dot(cellGradient(i + ivec3(0, 1, 1)), f - vec3(0, 1, 1));
    float n111 = dot(cellGradient(i + ivec3(1, 1, 1)), f - vec3(1, 1, 1));
    return mix(mix(mix(n000, n100, u.x), mix(n010, n110, u.x), u.y),
               mix(mix(n001, n101, u.x), mix(n011, n111, u.x), u.y), u.z) * 1.6;
}
float vnoise(vec3 p) {
    vec3 fl = floor(p);
    ivec3 i = ivec3(fl);
    vec3 f = p - fl;
    vec3 u = f * f * (3.0 - 2.0 * f);
    return mix(mix(mix(hashCell(i), hashCell(i + ivec3(1, 0, 0)), u.x),
                   mix(hashCell(i + ivec3(0, 1, 0)), hashCell(i + ivec3(1, 1, 0)), u.x), u.y),
               mix(mix(hashCell(i + ivec3(0, 0, 1)), hashCell(i + ivec3(1, 0, 1)), u.x),
                   mix(hashCell(i + ivec3(0, 1, 1)), hashCell(i + ivec3(1, 1, 1)), u.x), u.y), u.z);
}
float fbm(vec3 p, int octaves) {
    float sum = 0.0;
    float amp = 0.5;
    for (int i = 0; i < 8; i++) {
        if (i >= octaves) break;
        sum += amp * gnoise(p);
        p = p * 2.02 + vec3(17.1, -9.7, 4.3);
        amp *= 0.5;
    }
    return sum;
}
vec3 acesTonemap(vec3 x) {
    x = max(x, vec3(0.0));
    return clamp((x * (2.51 * x + 0.03)) / (x * (2.43 * x + 0.59) + 0.14), 0.0, 1.0);
}
vec3 linearToSrgb(vec3 c) {
    c = clamp(c, 0.0, 1.0);
    return mix(c * 12.92, 1.055 * pow(c, vec3(1.0 / 2.4)) - 0.055, step(vec3(0.0031308), c));
}
float screenDither(vec2 fragCoord) {
    return (hash12(fragCoord) - 0.5) / 255.0;
}
`;

    const FORMATS = {
        rgba8: ['RGBA8', 'RGBA', 'UNSIGNED_BYTE'],
        rgba16f: ['RGBA16F', 'RGBA', 'HALF_FLOAT'],
        rgba32f: ['RGBA32F', 'RGBA', 'FLOAT'],
        rg16f: ['RG16F', 'RG', 'HALF_FLOAT'],
        r16f: ['R16F', 'RED', 'HALF_FLOAT']
    };

    const BLOOM_PREFILTER = `
uniform sampler2D uSource;
uniform vec2 uTexel;
uniform float uThreshold;
uniform float uKnee;
uniform float uClampMax;
vec3 sampleBox(vec2 uv) {
    vec4 d = uTexel.xyxy * vec4(-1.0, -1.0, 1.0, 1.0);
    return (texture(uSource, uv + d.xy).rgb + texture(uSource, uv + d.zy).rgb +
            texture(uSource, uv + d.xw).rgb + texture(uSource, uv + d.zw).rgb) * 0.25;
}
void main() {
    vec3 c = min(sampleBox(vUv), vec3(uClampMax));
    float br = max(c.r, max(c.g, c.b));
    float soft = clamp(br - uThreshold + uKnee, 0.0, 2.0 * uKnee);
    soft = soft * soft / (4.0 * uKnee + 1e-4);
    float contribution = max(soft, br - uThreshold) / max(br, 1e-4);
    fragColor = vec4(c * contribution, 1.0);
}`;

    const BLOOM_DOWN = `
uniform sampler2D uSource;
uniform vec2 uTexel;
void main() {
    vec2 t = uTexel;
    vec3 a = texture(uSource, vUv + t * vec2(-2.0, 2.0)).rgb;
    vec3 b = texture(uSource, vUv + t * vec2(0.0, 2.0)).rgb;
    vec3 c = texture(uSource, vUv + t * vec2(2.0, 2.0)).rgb;
    vec3 d = texture(uSource, vUv + t * vec2(-2.0, 0.0)).rgb;
    vec3 e = texture(uSource, vUv).rgb;
    vec3 f = texture(uSource, vUv + t * vec2(2.0, 0.0)).rgb;
    vec3 g = texture(uSource, vUv + t * vec2(-2.0, -2.0)).rgb;
    vec3 h = texture(uSource, vUv + t * vec2(0.0, -2.0)).rgb;
    vec3 i = texture(uSource, vUv + t * vec2(2.0, -2.0)).rgb;
    vec3 j = texture(uSource, vUv + t * vec2(-1.0, 1.0)).rgb;
    vec3 k = texture(uSource, vUv + t * vec2(1.0, 1.0)).rgb;
    vec3 l = texture(uSource, vUv + t * vec2(-1.0, -1.0)).rgb;
    vec3 m = texture(uSource, vUv + t * vec2(1.0, -1.0)).rgb;
    vec3 col = e * 0.125 + (a + c + g + i) * 0.03125 + (b + d + f + h) * 0.0625 + (j + k + l + m) * 0.125;
    fragColor = vec4(col, 1.0);
}`;

    const BLOOM_UP = `
uniform sampler2D uSource;
uniform sampler2D uBase;
uniform vec2 uTexel;
uniform float uScatter;
void main() {
    vec4 d = uTexel.xyxy * vec4(1.0, 1.0, -1.0, 0.0);
    vec3 s = texture(uSource, vUv - d.xy).rgb;
    s += texture(uSource, vUv - d.wy).rgb * 2.0;
    s += texture(uSource, vUv - d.zy).rgb;
    s += texture(uSource, vUv + d.zw).rgb * 2.0;
    s += texture(uSource, vUv).rgb * 4.0;
    s += texture(uSource, vUv + d.xw).rgb * 2.0;
    s += texture(uSource, vUv + d.zy).rgb;
    s += texture(uSource, vUv + d.wy).rgb * 2.0;
    s += texture(uSource, vUv + d.xy).rgb;
    fragColor = vec4(texture(uBase, vUv).rgb + s * (1.0 / 16.0) * uScatter, 1.0);
}`;

    function createRng(seed) {
        let a = (seed >>> 0) || 0x9e3779b9;
        return function () {
            a = (a + 0x6d2b79f5) >>> 0;
            let t = a;
            t = Math.imul(t ^ (t >>> 15), t | 1);
            t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
            return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
        };
    }

    function create(canvas, options) {
        const gl = canvas.getContext('webgl2', Object.assign({
            alpha: false,
            antialias: false,
            depth: false,
            stencil: false,
            premultipliedAlpha: false,
            preserveDrawingBuffer: false,
            powerPreference: 'high-performance'
        }, options || {}));
        if (!gl) throw new Error('webgl2-unavailable');

        const owned = { programs: [], shaders: [], textures: [], framebuffers: [], buffers: [], vaos: [] };
        const floatRender = !!gl.getExtension('EXT_color_buffer_float');
        const halfRender = floatRender || !!gl.getExtension('EXT_color_buffer_half_float');
        const floatLinear = !!gl.getExtension('OES_texture_float_linear');
        const emptyVao = gl.createVertexArray();
        owned.vaos.push(emptyVao);
        let disposed = false;

        function compileShader(type, source, name) {
            const shader = gl.createShader(type);
            gl.shaderSource(shader, source);
            gl.compileShader(shader);
            if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS) && !gl.isContextLost()) {
                const log = gl.getShaderInfoLog(shader);
                gl.deleteShader(shader);
                throw new Error('screensaver shader "' + name + '" failed: ' + log);
            }
            owned.shaders.push(shader);
            return shader;
        }

        function program(fragmentBody, opts) {
            const settings = opts || {};
            const name = settings.name || 'program';
            let header = FS_HEADER;
            if (settings.outputs > 1) {
                const outputs = ['layout(location = 0) out vec4 fragColor;'];
                for (let i = 1; i < settings.outputs; i++) outputs.push('layout(location = ' + i + ') out vec4 fragData' + i + ';');
                header = FS_HEADER.replace('out vec4 fragColor;', outputs.join('\n'));
            }
            const fsSource = settings.rawFragment ? fragmentBody : header + (settings.common === false ? '' : GLSL_COMMON) + fragmentBody;
            const vs = compileShader(gl.VERTEX_SHADER, settings.vertex || FULLSCREEN_VS, name + '.vs');
            const fs = compileShader(gl.FRAGMENT_SHADER, fsSource, name + '.fs');
            const prog = gl.createProgram();
            gl.attachShader(prog, vs);
            gl.attachShader(prog, fs);
            if (settings.transformFeedback) gl.transformFeedbackVaryings(prog, settings.transformFeedback, gl.SEPARATE_ATTRIBS);
            gl.linkProgram(prog);
            if (!gl.getProgramParameter(prog, gl.LINK_STATUS) && !gl.isContextLost()) {
                throw new Error('screensaver program "' + name + '" failed to link: ' + gl.getProgramInfoLog(prog));
            }
            owned.programs.push(prog);
            const uniforms = {};
            const count = gl.getProgramParameter(prog, gl.ACTIVE_UNIFORMS) || 0;
            for (let i = 0; i < count; i++) {
                const info = gl.getActiveUniform(prog, i);
                if (!info) continue;
                const key = info.name.replace(/\[0\]$/, '');
                uniforms[key] = { location: gl.getUniformLocation(prog, info.name), type: info.type, size: info.size };
            }
            return { program: prog, uniforms, name };
        }

        function textureOf(value) {
            if (!value) return null;
            if (value.texture) return value.texture;
            return value;
        }

        function applyUniforms(p, values) {
            let unit = 0;
            for (const key in values) {
                const info = p.uniforms[key];
                if (!info) continue;
                const v = values[key];
                const loc = info.location;
                switch (info.type) {
                case gl.FLOAT: if (info.size > 1) gl.uniform1fv(loc, v); else gl.uniform1f(loc, v); break;
                case gl.FLOAT_VEC2: gl.uniform2fv(loc, v); break;
                case gl.FLOAT_VEC3: gl.uniform3fv(loc, v); break;
                case gl.FLOAT_VEC4: gl.uniform4fv(loc, v); break;
                case gl.INT:
                case gl.BOOL: gl.uniform1i(loc, v | 0); break;
                case gl.FLOAT_MAT3: gl.uniformMatrix3fv(loc, false, v); break;
                case gl.FLOAT_MAT4: gl.uniformMatrix4fv(loc, false, v); break;
                case gl.SAMPLER_2D:
                case gl.SAMPLER_3D:
                    gl.activeTexture(gl.TEXTURE0 + unit);
                    gl.bindTexture(info.type === gl.SAMPLER_3D ? gl.TEXTURE_3D : gl.TEXTURE_2D, textureOf(v));
                    gl.uniform1i(loc, unit);
                    unit++;
                    break;
                default: break;
                }
            }
        }

        function bindTarget(target) {
            if (target) {
                gl.bindFramebuffer(gl.FRAMEBUFFER, target.fbo);
                gl.viewport(0, 0, target.width, target.height);
            } else {
                gl.bindFramebuffer(gl.FRAMEBUFFER, null);
                gl.viewport(0, 0, gl.drawingBufferWidth, gl.drawingBufferHeight);
            }
        }

        function draw(p, values, target) {
            gl.useProgram(p.program);
            applyUniforms(p, values || {});
            bindTarget(target || null);
            gl.bindVertexArray(emptyVao);
            gl.drawArrays(gl.TRIANGLES, 0, 3);
        }

        function resolveFormat(name) {
            let key = FORMATS[name] ? name : 'rgba8';
            if ((key === 'rgba32f') && !floatRender) key = halfRender ? 'rgba16f' : 'rgba8';
            if ((key === 'rgba16f' || key === 'rg16f' || key === 'r16f') && !halfRender) key = 'rgba8';
            const spec = FORMATS[key];
            return { key, internal: gl[spec[0]], format: gl[spec[1]], type: gl[spec[2]] };
        }

        function allocate(tex, fmt, w, h, data) {
            gl.bindTexture(gl.TEXTURE_2D, tex);
            gl.texImage2D(gl.TEXTURE_2D, 0, fmt.internal, w, h, 0, fmt.format, fmt.type, data || null);
        }

        function target(width, height, opts) {
            const settings = opts || {};
            const fmt = resolveFormat(settings.format || 'rgba8');
            let filter = settings.filter === 'nearest' ? gl.NEAREST : gl.LINEAR;
            if (fmt.key === 'rgba32f' && !floatLinear) filter = gl.NEAREST;
            const wrap = settings.wrap === 'repeat' ? gl.REPEAT : gl.CLAMP_TO_EDGE;
            const tex = gl.createTexture();
            owned.textures.push(tex);
            gl.bindTexture(gl.TEXTURE_2D, tex);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, filter);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, filter);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, wrap);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, wrap);
            const fbo = gl.createFramebuffer();
            owned.framebuffers.push(fbo);
            const t = {
                texture: tex,
                fbo,
                format: fmt.key,
                width: 0,
                height: 0,
                texel: [0, 0],
                resize(w, h, data) {
                    t.width = Math.max(1, Math.round(w));
                    t.height = Math.max(1, Math.round(h));
                    t.texel = [1 / t.width, 1 / t.height];
                    allocate(tex, fmt, t.width, t.height, data);
                    gl.bindFramebuffer(gl.FRAMEBUFFER, fbo);
                    gl.framebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, tex, 0);
                    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
                    return t;
                },
                clear(r, g, b, a) {
                    bindTarget(t);
                    gl.clearColor(r || 0, g || 0, b || 0, a || 0);
                    gl.clear(gl.COLOR_BUFFER_BIT);
                }
            };
            return t.resize(width, height, settings.data);
        }

        function doubleTarget(width, height, opts) {
            let read = target(width, height, opts);
            let write = target(width, height, opts);
            return {
                get read() { return read; },
                get write() { return write; },
                get width() { return read.width; },
                get height() { return read.height; },
                get texel() { return read.texel; },
                swap() { const tmp = read; read = write; write = tmp; },
                resize(w, h) { read.resize(w, h); write.resize(w, h); },
                clear() { read.clear(); write.clear(); }
            };
        }

        function multiTarget(width, height, formats, opts) {
            const settings = opts || {};
            const fbo = gl.createFramebuffer();
            owned.framebuffers.push(fbo);
            const filter = settings.filter === 'nearest' ? gl.NEAREST : gl.LINEAR;
            const attachments = formats.map((_, i) => gl.COLOR_ATTACHMENT0 + i);
            const textures = formats.map(name => {
                const fmt = resolveFormat(name);
                const tex = gl.createTexture();
                owned.textures.push(tex);
                gl.bindTexture(gl.TEXTURE_2D, tex);
                const texFilter = fmt.key === 'rgba32f' && !floatLinear ? gl.NEAREST : filter;
                gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, texFilter);
                gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, texFilter);
                gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
                gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
                return { texture: tex, fmt, width: 0, height: 0, texel: [0, 0] };
            });
            const mt = {
                fbo,
                textures,
                width: 0,
                height: 0,
                texel: [0, 0],
                resize(w, h) {
                    mt.width = Math.max(1, Math.round(w));
                    mt.height = Math.max(1, Math.round(h));
                    mt.texel = [1 / mt.width, 1 / mt.height];
                    gl.bindFramebuffer(gl.FRAMEBUFFER, fbo);
                    textures.forEach((entry, i) => {
                        allocate(entry.texture, entry.fmt, mt.width, mt.height);
                        entry.width = mt.width;
                        entry.height = mt.height;
                        entry.texel = mt.texel;
                        gl.framebufferTexture2D(gl.FRAMEBUFFER, attachments[i], gl.TEXTURE_2D, entry.texture, 0);
                    });
                    gl.drawBuffers(attachments);
                    gl.bindFramebuffer(gl.FRAMEBUFFER, null);
                    return mt;
                }
            };
            return mt.resize(width, height);
        }

        function dataTexture(width, height, format, data, opts) {
            const settings = opts || {};
            // Sample-only textures never need render support, so no fallback applies.
            const spec = FORMATS[format] || FORMATS.rgba8;
            const fmt = { key: format, internal: gl[spec[0]], format: gl[spec[1]], type: gl[spec[2]] };
            const tex = gl.createTexture();
            owned.textures.push(tex);
            const filter = settings.filter === 'nearest' ? gl.NEAREST : gl.LINEAR;
            const wrap = settings.wrap === 'repeat' ? gl.REPEAT : gl.CLAMP_TO_EDGE;
            gl.bindTexture(gl.TEXTURE_2D, tex);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, filter);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, filter);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, wrap);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, wrap);
            allocate(tex, fmt, width, height, data);
            return {
                texture: tex,
                width,
                height,
                update(next) { allocate(tex, fmt, width, height, next); }
            };
        }

        function buffer(data, usage) {
            const buf = gl.createBuffer();
            owned.buffers.push(buf);
            gl.bindBuffer(gl.ARRAY_BUFFER, buf);
            gl.bufferData(gl.ARRAY_BUFFER, data, usage || gl.STATIC_DRAW);
            return buf;
        }

        function vertexArray() {
            const vao = gl.createVertexArray();
            owned.vaos.push(vao);
            return vao;
        }

        function bloom(width, height, opts) {
            const settings = opts || {};
            const levels = Math.max(3, Math.min(8, settings.levels || 6));
            const format = halfRender ? 'rgba16f' : 'rgba8';
            const prefilter = program(BLOOM_PREFILTER, { name: 'bloom-prefilter', common: false });
            const down = program(BLOOM_DOWN, { name: 'bloom-down', common: false });
            const up = program(BLOOM_UP, { name: 'bloom-up', common: false });
            const mips = [];
            const ups = [];
            function layout(w, h) {
                let mw = Math.max(1, Math.floor(w / 2));
                let mh = Math.max(1, Math.floor(h / 2));
                for (let i = 0; i < levels; i++) {
                    if (!mips[i]) {
                        mips[i] = target(mw, mh, { format });
                        ups[i] = target(mw, mh, { format });
                    } else {
                        mips[i].resize(mw, mh);
                        ups[i].resize(mw, mh);
                    }
                    mw = Math.max(1, Math.floor(mw / 2));
                    mh = Math.max(1, Math.floor(mh / 2));
                }
            }
            layout(width, height);
            return {
                resize: layout,
                render(source, params) {
                    const p = params || {};
                    const src = source.texture ? source : { texture: source, texel: p.sourceTexel || [1 / width, 1 / height] };
                    draw(prefilter, {
                        uSource: src.texture,
                        uTexel: src.texel || [1 / width, 1 / height],
                        uThreshold: p.threshold == null ? 1 : p.threshold,
                        uKnee: p.knee == null ? 0.5 : p.knee,
                        uClampMax: p.clampMax || 64
                    }, mips[0]);
                    for (let i = 1; i < levels; i++) {
                        draw(down, { uSource: mips[i - 1].texture, uTexel: mips[i - 1].texel }, mips[i]);
                    }
                    let current = mips[levels - 1];
                    for (let i = levels - 2; i >= 0; i--) {
                        draw(up, { uSource: current.texture, uBase: mips[i].texture, uTexel: current.texel, uScatter: p.scatter == null ? 0.85 : p.scatter }, ups[i]);
                        current = ups[i];
                    }
                    return current;
                }
            };
        }

        function dispose() {
            if (disposed) return;
            disposed = true;
            if (!gl.isContextLost()) {
                owned.framebuffers.forEach(x => gl.deleteFramebuffer(x));
                owned.textures.forEach(x => gl.deleteTexture(x));
                owned.buffers.forEach(x => gl.deleteBuffer(x));
                owned.vaos.forEach(x => gl.deleteVertexArray(x));
                owned.programs.forEach(x => gl.deleteProgram(x));
                owned.shaders.forEach(x => gl.deleteShader(x));
            }
            const lose = gl.getExtension('WEBGL_lose_context');
            if (lose) lose.loseContext();
        }

        return {
            gl,
            caps: { floatRender, halfRender, floatLinear },
            program,
            draw,
            applyUniforms,
            bindTarget,
            target,
            doubleTarget,
            multiTarget,
            dataTexture,
            buffer,
            vertexArray,
            bloom,
            dispose
        };
    }

    window.AuraScreensaverGL = {
        create,
        createRng,
        GLSL_COMMON,
        FS_HEADER,
        FULLSCREEN_VS
    };
})();
