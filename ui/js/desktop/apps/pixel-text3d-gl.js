// WebGL2 / WebGL1 GPU 3D SDF raymarching renderer for Pixel 3D Text.
// Compiles a tiny specialized fragment shader per warp mode (fast ANGLE/HLSL compile, zero giant branch inside raymarch),
// renders extruded/bevelled/warped/textured studio-lit 3D text with cast shadows, and falls back to CPU when WebGL is absent.
(function () {
    'use strict';

    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const VERT_SRC = `
attribute vec2 aPos;
varying vec2 vUv;
void main() {
    vUv = aPos * 0.5 + 0.5;
    gl_Position = vec4(aPos, 0.0, 1.0);
}`;

    const WARP_SNIPPETS = {
        0: 'return p;',
        1: 'float sgn = k >= 0.0 ? 1.0 : -1.0; float R = max(0.85, a / (abs(k) * 1.45)); float cy = p.y + sgn * R; return vec3(atan(p.x, sgn * cy) * R, sgn * (length(vec2(p.x, cy)) - R), p.z);',
        2: 'float nx = clamp(p.x / a, -1.35, 1.35); return vec3(p.x, p.y - k * 0.72 * (1.0 - nx * nx), p.z);',
        3: 'return vec3(p.x, p.y - k * 0.42 * sin((p.x / a) * uWarpFreq * PI), p.z);',
        4: 'float ph = (p.x / a) * uWarpFreq * PI; return vec3(p.x, p.y - k * 0.34 * sin(ph), p.z - k * 0.38 * cos(ph * 1.15));',
        5: 'float nx = clamp(p.x / a, -1.2, 1.2); return vec3(p.x, p.y / max(0.28, 1.0 + k * 0.62 * cos(nx * PI * 0.5)), p.z);',
        6: 'float sgn = k >= 0.0 ? 1.0 : -1.0; float R = max(0.6, a / (abs(k) * 2.15)); float cz = p.z + sgn * R; return vec3(atan(p.x, sgn * cz) * R, p.y, sgn * (length(vec2(p.x, cz)) - R));',
        7: 'float th = k * 1.35 * clamp(p.x / a, -1.4, 1.4); float ct = cos(th); float st = sin(th); return vec3(p.x, p.y * ct + p.z * st, -p.y * st + p.z * ct);',
        8: 'float nx = clamp(p.x / a, -1.25, 1.25); float ar = abs(k) * 0.62 * max(0.0, 1.0 - nx * nx); float lo = k > 0.0 ? (-1.0 + ar) : -1.0; float hi = k > 0.0 ? 1.0 : (1.0 - ar); return vec3(p.x, ((p.y - lo) / max(0.25, hi - lo)) * 2.0 - 1.0, p.z);',
        9: 'float nx = clamp(p.x / a, -1.25, 1.25); return vec3(p.x, p.y / max(0.28, 1.0 + k * 0.52 * nx), p.z);',
        10: 'return vec3(p.x, p.y - k * 0.58 * (p.x / a), p.z);',
        11: 'float nx = clamp(p.x / a, -1.25, 1.25); float ny = clamp(p.y, -1.25, 1.25); return vec3(p.x, p.y, p.z - k * 0.55 * max(0.0, 1.0 - nx * nx - ny * ny));'
    };

    function buildFragSource(warpCode) {
        const snippet = WARP_SNIPPETS[warpCode] || WARP_SNIPPETS[0];
        return `
precision highp float;
varying vec2 vUv;

uniform sampler2D uSdfTex;
uniform sampler2D uMatTex;
uniform vec2 uOutSize;
uniform vec4 uViewRect;
uniform vec2 uCenter;
uniform float uBaseScale;
uniform float uCamDist;
uniform mat3 uRot;
uniform mat3 uInvRot;
uniform vec3 uBoxHalf;
uniform vec2 uSdfSize;
uniform vec2 uTexHalf;
uniform float uSdfMode;
uniform float uSdfSpread;
uniform float uAspect;
uniform float uHalfDepth;
uniform float uBevelR;
uniform int uBevelCode;
uniform float uWarpStrength;
uniform float uWarpFreq;
uniform float uLipschitz;
uniform vec3 uLightDir;
uniform vec3 uPrimColor;
uniform vec3 uSideColor;
uniform float uSideMatchTex;
uniform float uTintSolid;
uniform vec4 uTexXform;
uniform vec4 uMatProps;
uniform float uHolo;
uniform vec3 uShadowCfg;
uniform float uBackPlaneZ;
uniform int uSteps;

const float PI = 3.14159265359;
const float TAU = 6.28318530718;

vec3 inverseWarp(vec3 p) {
    float k = uWarpStrength;
    float a = max(0.25, uAspect);
    if (abs(k) < 0.0001) return p;
    ${snippet}
}

float sampleSdf2D(vec2 uvLocal) {
    vec2 outV = max(abs(uvLocal) - uTexHalf, 0.0);
    float outBox = length(outV);
    vec2 tUv = clamp(vec2(0.5 + uvLocal.x / (2.0 * uTexHalf.x), 0.5 - uvLocal.y / (2.0 * uTexHalf.y)), 0.001, 0.999);
    float raw = texture2D(uSdfTex, tUv).r;
    float pxDist = uSdfMode > 0.5 ? raw : (raw * 2.0 - 1.0) * uSdfSpread;
    return pxDist * ((2.0 * uTexHalf.y) / max(1.0, uSdfSize.y)) + outBox;
}

float evalExtruded(float d2, float qz) {
    float dZ = abs(qz) - uHalfDepth;
    if (uBevelCode == 3) {
        float bulge = (sqrt(max(0.0, -d2) + 0.006) - 0.07746) * (uBevelR * 2.1);
        dZ -= max(0.0, bulge);
        float r = uBevelR * 0.45;
        vec2 q = vec2(d2 + r, dZ + r);
        return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - r;
    }
    if (uBevelCode == 2 || uBevelR <= 0.0001) {
        vec2 q = vec2(d2 + 0.0035, dZ + 0.0035);
        return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - 0.0035;
    }
    if (uBevelCode == 1) {
        return max(max(d2, dZ), (d2 + dZ + uBevelR) * 0.70710678);
    }
    vec2 q = vec2(d2 + uBevelR, dZ + uBevelR);
    return length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - uBevelR;
}

float mapSDF(vec3 p) {
    vec3 q = inverseWarp(p);
    return evalExtruded(sampleSdf2D(q.xy), q.z);
}

vec3 calcNormal(vec3 p) {
    float e = 0.0045;
    vec2 k = vec2(1.0, -1.0);
    return normalize(
        k.xyy * mapSDF(p + k.xyy * e) +
        k.yyx * mapSDF(p + k.yyx * e) +
        k.yxy * mapSDF(p + k.yxy * e) +
        k.xxx * mapSDF(p + k.xxx * e)
    );
}

vec2 intersectBox(vec3 ro, vec3 rd, vec3 h) {
    vec3 inv = 1.0 / (rd + vec3(equal(rd, vec3(0.0))) * 1e-7);
    vec3 t1 = (-h - ro) * inv;
    vec3 t2 = (h - ro) * inv;
    vec3 tmin = min(t1, t2);
    vec3 tmax = max(t1, t2);
    return vec2(max(0.001, max(max(tmin.x, tmin.y), tmin.z)), min(min(tmax.x, tmax.y), tmax.z));
}

vec3 sampleStudioEnv(vec3 R) {
    float up = clamp(R.y * 0.5 + 0.5, 0.0, 1.0);
    float hor = exp(-abs(R.y) * 5.2);
    vec3 sky = mix(vec3(0.11, 0.13, 0.19), vec3(0.76, 0.84, 0.96), up) + vec3(0.42, 0.45, 0.52) * hor;
    float s1 = smoothstep(0.68, 0.96, dot(R, normalize(vec3(0.45, 0.68, 0.58))));
    float s2 = smoothstep(0.74, 0.98, dot(R, normalize(vec3(-0.72, 0.34, 0.60))));
    float band = smoothstep(0.86, 0.98, abs(sin(R.y * 5.0 + R.x * 2.0))) * 0.14;
    return sky + vec3(1.0, 0.96, 0.88) * s1 * 1.05 + vec3(0.48, 0.68, 1.0) * s2 * 0.55 + vec3(band);
}

vec4 shadeRay(vec2 px) {
    vec2 docPos = uViewRect.xy + (px / uOutSize) * uViewRect.zw;
    vec2 sc = vec2((docPos.x - uCenter.x) / uBaseScale, (uCenter.y - docPos.y) / uBaseScale);
    vec3 roW = vec3(0.0, 0.0, uCamDist);
    vec3 rdW = normalize(vec3(sc.x, sc.y, -uCamDist));
    vec3 ro = uInvRot * roW;
    vec3 rd = uInvRot * rdW;

    vec2 hitBox = intersectBox(ro, rd, uBoxHalf);
    bool hit = false;
    float tHit = 0.0;
    if (hitBox.x < hitBox.y) {
        float t = hitBox.x;
        float prevT = t;
        float prevD = 1.0;
        for (int i = 0; i < 64; i++) {
            if (i >= uSteps || t > hitBox.y) break;
            float d = mapSDF(ro + rd * t);
            if (d < 0.0022) {
                float lo = prevT;
                float hi = t;
                if (prevD > 0.0) {
                    for (int k = 0; k < 4; k++) {
                        float mid = 0.5 * (lo + hi);
                        if (mapSDF(ro + rd * mid) < 0.0) hi = mid; else lo = mid;
                    }
                }
                tHit = hi;
                hit = true;
                break;
            }
            prevT = t;
            prevD = d;
            t += max(0.004, d * uLipschitz);
        }
    }

    if (!hit) {
        if (uShadowCfg.x < 0.5 || uShadowCfg.y <= 0.001 || rdW.z >= -0.05) return vec4(0.0);
        float tPlane = (uBackPlaneZ - uCamDist) / rdW.z;
        vec3 pPlaneW = roW + rdW * tPlane;
        vec3 shLightW = normalize(vec3(uLightDir.x * 0.7, uLightDir.y * 0.7, max(0.45, abs(uLightDir.z))));
        vec3 lLoc = uInvRot * shLightW;
        vec3 pLoc = uInvRot * pPlaneW;
        float res = 1.0;
        float blurK = mix(14.0, 2.2, uShadowCfg.z);
        float st = 0.06;
        for (int j = 0; j < 14; j++) {
            float ds = mapSDF(pLoc + lLoc * st);
            if (ds < 0.0015) { res = 0.0; break; }
            res = min(res, blurK * ds / max(0.08, st));
            st += clamp(ds * uLipschitz, 0.04, 0.36);
            if (st > 5.0) break;
        }
        return vec4(0.03, 0.04, 0.08, clamp(1.0 - res, 0.0, 1.0) * uShadowCfg.y);
    }

    vec3 pos = ro + rd * tHit;
    vec3 N = calcNormal(pos);
    vec3 q = inverseWarp(pos);
    float faceBorder = uHalfDepth - max(uBevelR * 1.15, 0.014);
    float frontMix = smoothstep(faceBorder - 0.02, faceBorder + 0.02, abs(q.z));
    vec2 baseUv = mix(vec2((q.x + q.y) * 0.44, q.z * 0.92), q.xy * 0.44, frontMix);
    vec2 rotUv = vec2(baseUv.x * uTexXform.y - baseUv.y * uTexXform.z, baseUv.x * uTexXform.z + baseUv.y * uTexXform.y);
    vec2 matUv = fract(rotUv * uTexXform.x + 0.5);
    vec4 tex = texture2D(uMatTex, matUv);
    vec3 albedo = uTintSolid > 0.5 ? uPrimColor : tex.rgb;
    float useTex = max(frontMix, uSideMatchTex);
    albedo = mix(uSideColor, albedo, useTex);

    float bumpAmp = uTexXform.w * useTex;
    if (bumpAmp > 0.005) {
        float hx = texture2D(uMatTex, fract(matUv + vec2(0.008, 0.0))).a;
        float hy = texture2D(uMatTex, fract(matUv + vec2(0.0, 0.008))).a;
        N = normalize(N + vec3((tex.a - hx) * bumpAmp * 1.4, (tex.a - hy) * bumpAmp * 1.4, 0.0));
    }

    vec3 Nw = normalize(uRot * N);
    vec3 V = -rdW;
    float ndv = clamp(dot(Nw, V), 0.0, 1.0);
    if (uHolo > 0.5 && useTex > 0.25) {
        float ph = (1.0 - ndv) * TAU + (q.x + q.y) * 1.5;
        albedo = mix(albedo, 0.56 + 0.44 * sin(vec3(ph, ph + 2.094, ph + 4.188)), 0.68 * useTex);
    }

    float ao = 0.0;
    ao += 0.50 * max(0.0, 0.055 - mapSDF(pos + N * 0.055));
    ao += 0.28 * max(0.0, 0.115 - mapSDF(pos + N * 0.115));
    ao += 0.15 * max(0.0, 0.185 - mapSDF(pos + N * 0.185));
    ao = clamp(1.0 - ao * 4.4, 0.38, 1.0);

    float metal = uMatProps.x;
    float gloss = uMatProps.y;
    float refAmt = uMatProps.z;
    float em = uMatProps.w * useTex;
    float ndl = max(0.0, dot(Nw, uLightDir));
    float wrap = max(0.0, (dot(Nw, uLightDir) + 0.22) / 1.22);
    float fill = max(0.0, dot(Nw, normalize(vec3(-0.58, 0.26, 0.76))));
    float rim = pow(1.0 - ndv, 3.0);
    vec3 H = normalize(uLightDir + V);
    float spec = pow(max(0.0, dot(Nw, H)), mix(10.0, 140.0, gloss)) * mix(0.22, 1.5, gloss);
    float fresnel = mix(0.04, 1.0, pow(1.0 - ndv, 4.0));
    vec3 env = sampleStudioEnv(reflect(-V, Nw));

    vec3 diffLight = (vec3(0.21, 0.23, 0.28) + vec3(0.88, 0.86, 0.83) * (ndl * 0.72 + wrap * 0.28) + vec3(0.24, 0.30, 0.42) * fill) * ao;
    vec3 specCol = mix(vec3(1.0), albedo, metal);
    vec3 envCol = env * mix(vec3(fresnel * refAmt * 0.65), albedo * refAmt * (0.65 + 0.35 * fresnel), metal);
    vec3 col = albedo * (diffLight * (1.0 - metal * 0.62) + em) + specCol * spec + envCol + vec3(0.25, 0.30, 0.42) * rim * 0.38;
    col = (col * (2.51 * col + 0.03)) / (col * (2.43 * col + 0.59) + 0.14);
    return vec4(clamp(col, 0.0, 1.0), 1.0);
}

void main() {
    vec2 px = vec2(vUv.x * uOutSize.x, (1.0 - vUv.y) * uOutSize.y);
    gl_FragColor = shadeRay(px);
}`;
    }

    const UNIFORM_NAMES = [
        'uSdfTex', 'uMatTex', 'uOutSize', 'uViewRect', 'uCenter', 'uBaseScale', 'uCamDist',
        'uRot', 'uInvRot', 'uBoxHalf', 'uSdfSize', 'uTexHalf', 'uSdfMode', 'uSdfSpread',
        'uAspect', 'uHalfDepth', 'uBevelR', 'uBevelCode', 'uWarpStrength', 'uWarpFreq',
        'uLipschitz', 'uLightDir', 'uPrimColor', 'uSideColor', 'uSideMatchTex', 'uTintSolid',
        'uTexXform', 'uMatProps', 'uHolo', 'uShadowCfg', 'uBackPlaneZ', 'uSteps'
    ];

    function compile(gl, type, src) {
        const sh = gl.createShader(type);
        if (!sh) return null;
        gl.shaderSource(sh, src);
        gl.compileShader(sh);
        if (!gl.getShaderParameter(sh, gl.COMPILE_STATUS)) {
            if (window.console) console.error('pixel text3d shader', gl.getShaderInfoLog(sh));
            gl.deleteShader(sh);
            return null;
        }
        return sh;
    }

    function parseHex(hex, fallback) {
        const s = typeof hex === 'string' ? hex.replace('#', '').trim() : '';
        if (/^[0-9a-f]{6}$/i.test(s)) {
            return [parseInt(s.slice(0, 2), 16) / 255, parseInt(s.slice(2, 4), 16) / 255, parseInt(s.slice(4, 6), 16) / 255];
        }
        return fallback;
    }

    function createRenderer() {
        const canvas = document.createElement('canvas');
        canvas.width = 1;
        canvas.height = 1;
        const opts = { alpha: true, premultipliedAlpha: false, antialias: false, depth: false, stencil: false, preserveDrawingBuffer: true };
        let isWebGL2 = false;
        let gl = null;
        try {
            gl = canvas.getContext('webgl2', opts);
            if (gl && typeof gl.createShader === 'function') isWebGL2 = true;
            else {
                gl = canvas.getContext('webgl', opts) || canvas.getContext('experimental-webgl', opts);
                if (!gl || typeof gl.createShader !== 'function') gl = null;
            }
        } catch (_) {
            gl = null;
        }
        if (!gl) return null;

        const vs = compile(gl, gl.VERTEX_SHADER, VERT_SRC);
        if (!vs) return null;

        const vbo = gl.createBuffer();
        gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
        gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]), gl.STATIC_DRAW);

        const programs = new Map();
        function programFor(warpCode) {
            const key = warpCode in WARP_SNIPPETS ? warpCode : 0;
            let entry = programs.get(key);
            if (entry) return entry;
            const fs = compile(gl, gl.FRAGMENT_SHADER, buildFragSource(key));
            if (!fs) return null;
            const prog = gl.createProgram();
            gl.attachShader(prog, vs);
            gl.attachShader(prog, fs);
            gl.linkProgram(prog);
            gl.deleteShader(fs);
            if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
                if (window.console) console.error('pixel text3d link', gl.getProgramInfoLog(prog));
                gl.deleteProgram(prog);
                return null;
            }
            const U = {};
            UNIFORM_NAMES.forEach(n => { U[n] = gl.getUniformLocation(prog, n); });
            entry = { prog, aPos: gl.getAttribLocation(prog, 'aPos'), U };
            programs.set(key, entry);
            return entry;
        }
        // Pre-verify warp 0 compiles cleanly so unsupported GPUs fall back immediately.
        if (!programFor(0)) {
            gl.deleteShader(vs);
            gl.deleteBuffer(vbo);
            return null;
        }

        const sdfTex = gl.createTexture();
        const matTex = gl.createTexture();
        let sdfKey = '', matKey = '', sdfMode = 0, lost = false, disposed = false;

        const onLost = e => { if (e && e.preventDefault) e.preventDefault(); lost = true; };
        if (typeof canvas.addEventListener === 'function') canvas.addEventListener('webglcontextlost', onLost, false);

        function uploadSdf(sdfObj, key) {
            if (key && key === sdfKey) return;
            gl.activeTexture(gl.TEXTURE0);
            gl.bindTexture(gl.TEXTURE_2D, sdfTex);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
            gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
            let usedFloat = false;
            if (isWebGL2 && sdfObj.sdf && gl.R16F && gl.RED && gl.FLOAT) {
                while (gl.getError() !== gl.NO_ERROR) { /* clear */ }
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.R16F, sdfObj.width, sdfObj.height, 0, gl.RED, gl.FLOAT, sdfObj.sdf);
                if (gl.getError() === gl.NO_ERROR) usedFloat = true;
            }
            if (!usedFloat) {
                gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, sdfObj.width, sdfObj.height, 0, gl.RGBA, gl.UNSIGNED_BYTE, sdfObj.rgba8);
            }
            sdfMode = usedFloat ? 1 : 0;
            sdfKey = key || '';
        }

        function uploadMat(texObj, key) {
            if (key && key === matKey) return;
            gl.activeTexture(gl.TEXTURE1);
            gl.bindTexture(gl.TEXTURE_2D, matTex);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
            gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
            gl.pixelStorei(gl.UNPACK_ALIGNMENT, 4);
            gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, texObj.width, texObj.height, 0, gl.RGBA, gl.UNSIGNED_BYTE, texObj.data);
            matKey = key || '';
        }

        return {
            isWebGL2,
            isUsable() { return !disposed && !lost && (!gl.isContextLost || !gl.isContextLost()); },
            render(sdfObj, sdfCacheKey, texObj, texCacheKey, params, proj, outW, outH, opts) {
                if (!this.isUsable()) return null;
                const T = window.AuraPixelLib && window.AuraPixelLib.text3d;
                if (!T) return null;
                const p = T.normalizeParams(params);
                const o = opts || {};
                const warpDef = T.warpById(p.warp);
                const entry = programFor(warpDef.code);
                if (!entry) return null;
                // Supersample 1.5x on highQuality for crisp anti-aliased downsampling onto the document canvas.
                const ss = o.highQuality ? 1.5 : 1.0;
                const w = Math.max(1, Math.min(3072, Math.round(outW * ss)));
                const h = Math.max(1, Math.min(3072, Math.round(outH * ss)));
                if (canvas.width !== w || canvas.height !== h) { canvas.width = w; canvas.height = h; }
                uploadSdf(sdfObj, sdfCacheKey);
                uploadMat(texObj, texCacheKey);

                const texDef = T.textureById(p.texture);
                const bevelDef = T.bevelById(p.bevelStyle);
                const bevelR = T.bevelRadiusOf(p.bevel, proj.halfDepth);
                const padFactor = o.padFactor || 1.25;
                const az = p.lightAzimuth * Math.PI / 180, el = p.lightElevation * Math.PI / 180;
                const lx = Math.cos(el) * Math.sin(az), ly = Math.sin(el), lz = Math.cos(el) * Math.cos(az);
                const lLen = Math.hypot(lx, ly, lz) || 1;
                const primRgb = parseHex(p.color, [0.95, 0.72, 0.24]);
                const sideRgb = parseHex(p.sideColor, [0.22, 0.28, 0.42]);
                const metal = Math.max(0, Math.min(1, (p.metalness / 100) * 0.7 + texDef.metalness * 0.3));
                const gloss = Math.max(0, Math.min(1, (p.gloss / 100) * 0.7 + texDef.gloss * 0.3));
                const reflect = Math.max(0, Math.min(1, texDef.reflect * (0.4 + metal * 0.65) * (0.35 + gloss * 0.65)));
                const bumpAmp = (p.bumpStrength / 100) * texDef.bump;
                const tRad = p.texRotate * Math.PI / 180;
                const b = proj.bounds;
                const backPlaneZ = -Math.max(b.hx, b.hy, b.hz) - 0.14;
                const U = entry.U;

                gl.viewport(0, 0, w, h);
                gl.disable(gl.BLEND);
                gl.clearColor(0, 0, 0, 0);
                gl.clear(gl.COLOR_BUFFER_BIT);
                gl.useProgram(entry.prog);
                gl.bindBuffer(gl.ARRAY_BUFFER, vbo);
                gl.enableVertexAttribArray(entry.aPos);
                gl.vertexAttribPointer(entry.aPos, 2, gl.FLOAT, false, 0, 0);

                gl.activeTexture(gl.TEXTURE0);
                gl.bindTexture(gl.TEXTURE_2D, sdfTex);
                gl.uniform1i(U.uSdfTex, 0);
                gl.activeTexture(gl.TEXTURE1);
                gl.bindTexture(gl.TEXTURE_2D, matTex);
                gl.uniform1i(U.uMatTex, 1);

                gl.uniform2f(U.uOutSize, w, h);
                gl.uniform4f(U.uViewRect, o.viewX || 0, o.viewY || 0, o.viewW || outW, o.viewH || outH);
                gl.uniform2f(U.uCenter, proj.cx, proj.cy);
                gl.uniform1f(U.uBaseScale, proj.baseScale);
                gl.uniform1f(U.uCamDist, proj.camDist);
                gl.uniformMatrix3fv(U.uRot, false, new Float32Array(proj.Rt));
                gl.uniformMatrix3fv(U.uInvRot, false, new Float32Array(proj.R));
                gl.uniform3f(U.uBoxHalf, b.hx, b.hy, b.hz);
                gl.uniform2f(U.uSdfSize, sdfObj.width, sdfObj.height);
                gl.uniform2f(U.uTexHalf, o.texHalfX || (proj.aspect * padFactor), o.texHalfY || padFactor);
                gl.uniform1f(U.uSdfMode, sdfMode);
                gl.uniform1f(U.uSdfSpread, sdfObj.spread || 48);
                gl.uniform1f(U.uAspect, proj.aspect);
                gl.uniform1f(U.uHalfDepth, proj.halfDepth);
                gl.uniform1f(U.uBevelR, bevelR);
                gl.uniform1i(U.uBevelCode, bevelDef.code);
                gl.uniform1f(U.uWarpStrength, p.warpStrength / 100);
                gl.uniform1f(U.uWarpFreq, p.warpFreq);
                gl.uniform1f(U.uLipschitz, warpDef.lip);
                gl.uniform3f(U.uLightDir, lx / lLen, ly / lLen, lz / lLen);
                gl.uniform3f(U.uPrimColor, primRgb[0], primRgb[1], primRgb[2]);
                gl.uniform3f(U.uSideColor, sideRgb[0], sideRgb[1], sideRgb[2]);
                gl.uniform1f(U.uSideMatchTex, p.sideMatchTexture ? 1 : 0);
                gl.uniform1f(U.uTintSolid, p.texture === 'solid' ? 1 : 0);
                gl.uniform4f(U.uTexXform, (p.texScale || 100) / 100, Math.cos(tRad), Math.sin(tRad), bumpAmp);
                gl.uniform4f(U.uMatProps, metal, gloss, reflect, texDef.emissive);
                gl.uniform1f(U.uHolo, texDef.holo ? 1 : 0);
                gl.uniform3f(U.uShadowCfg, p.shadow ? 1 : 0, (p.shadowOpacity || 0) / 100, (p.shadowBlur || 0) / 100);
                gl.uniform1f(U.uBackPlaneZ, backPlaneZ);
                gl.uniform1i(U.uSteps, o.highQuality ? 64 : 44);

                gl.drawArrays(gl.TRIANGLES, 0, 6);
                return canvas;
            },
            dispose() {
                if (disposed) return;
                disposed = true;
                if (typeof canvas.removeEventListener === 'function') canvas.removeEventListener('webglcontextlost', onLost, false);
                try {
                    gl.deleteTexture(sdfTex);
                    gl.deleteTexture(matTex);
                    gl.deleteBuffer(vbo);
                    gl.deleteShader(vs);
                    programs.forEach(entry => gl.deleteProgram(entry.prog));
                    programs.clear();
                    const ext = gl.getExtension && gl.getExtension('WEBGL_lose_context');
                    if (ext && typeof ext.loseContext === 'function') ext.loseContext();
                } catch (_) { /* ignore */ }
                canvas.width = 0;
                canvas.height = 0;
            }
        };
    }

    Pixel.text3dGL = { createRenderer };
})();
