/* Aura Drift - FX atlas and shader resources. Each FX instance owns its returned texture. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const rnd = Math.random, TAU = Math.PI * 2;
    const F = { PUFF: 0, SPARK: 1, FLAME: 2, FEATHER: 3, CONFETTI: 4, RING: 5, SHARD: 6, DROP: 7, STAR: 8, BOLT: 9, HEX: 10, LEAF: 11, PUFF2: 12, GLOW: 13, STREAK: 14, DOT: 15 };

    function buildAtlas(T) {
        const S = 512, C = 128;
        const cv = document.createElement('canvas');
        cv.width = cv.height = S;
        const g = cv.getContext('2d');
        const cell = (i, fn) => {
            g.save();
            g.translate((i % 4) * C + C / 2, Math.floor(i / 4) * C + C / 2);
            g.beginPath(); g.rect(-C / 2, -C / 2, C, C); g.clip();
            fn();
            g.restore();
        };
        const blob = (x, y, r, a, shade) => {
            const c = shade | 0, gr = g.createRadialGradient(x, y, 0, x, y, r);
            gr.addColorStop(0, 'rgba(' + c + ',' + c + ',' + c + ',' + a + ')');
            gr.addColorStop(1, 'rgba(' + c + ',' + c + ',' + c + ',0)');
            g.fillStyle = gr; g.fillRect(x - r, y - r, r * 2, r * 2);
        };
        /* Radial mask: the soft-particle alpha falloff that keeps every sprite edge invisible. */
        const mask = (r0, r1) => {
            g.globalCompositeOperation = 'destination-in';
            const gr = g.createRadialGradient(0, 0, r0, 0, 0, r1);
            gr.addColorStop(0, 'rgba(0,0,0,1)'); gr.addColorStop(1, 'rgba(0,0,0,0)');
            g.fillStyle = gr; g.fillRect(-C / 2, -C / 2, C, C);
            g.globalCompositeOperation = 'source-over';
        };
        const poly = (pts, open) => { g.beginPath(); pts.forEach((p, i) => (i ? g.lineTo(p[0], p[1]) : g.moveTo(p[0], p[1]))); if (!open) g.closePath(); };
        const puff = (n, spread, rmin, rmax, a) => {
            for (let k = 0; k < n; k++) { const an = rnd() * TAU, d = Math.sqrt(rnd()) * spread; blob(Math.cos(an) * d, Math.sin(an) * d * 0.9, rmin + rnd() * (rmax - rmin), a, 95 + rnd() * 160); }
        };
        cell(F.PUFF, () => { puff(22, 26, 18, 34, 0.5); mask(22, 60); });
        cell(F.PUFF2, () => { puff(26, 30, 14, 28, 0.45); g.save(); g.scale(1.25, 0.8); puff(8, 18, 16, 26, 0.4); g.restore(); mask(20, 61); });
        cell(F.SPARK, () => { g.save(); g.scale(1, 0.2); blob(0, 0, 60, 1, 255); blob(0, 0, 30, 1, 255); g.restore(); blob(0, 0, 11, 1, 255); });
        cell(F.FLAME, () => { blob(0, 0, 56, 0.5, 255); blob(0, 0, 34, 0.75, 255); blob(0, 0, 16, 1, 255); puff(7, 20, 12, 22, 0.55); mask(26, 60); });
        cell(F.FEATHER, () => {
            g.fillStyle = '#fff'; g.beginPath(); g.moveTo(-52, 0); g.quadraticCurveTo(-8, -28, 52, -4); g.quadraticCurveTo(12, 24, -52, 0); g.fill();
            g.strokeStyle = 'rgb(150,150,150)'; g.lineWidth = 3; g.beginPath(); g.moveTo(-54, 1); g.lineTo(50, -3); g.stroke();
            g.lineWidth = 2; g.strokeStyle = 'rgb(205,205,205)';
            for (let k = -3; k <= 4; k++) { g.beginPath(); g.moveTo(k * 11, -1); g.lineTo(k * 11 + 14, -13 + Math.abs(k)); g.moveTo(k * 11, 0); g.lineTo(k * 11 + 14, 11 - Math.abs(k)); g.stroke(); }
        });
        cell(F.CONFETTI, () => { g.fillStyle = '#fff'; g.fillRect(-26, -15, 52, 15); g.fillStyle = 'rgb(205,205,205)'; g.fillRect(-26, 0, 52, 15); });
        const radial = (r, stops) => {
            const gr = g.createRadialGradient(0, 0, 0, 0, 0, r);
            for (const st of stops) gr.addColorStop(st[0], 'rgba(255,255,255,' + st[1] + ')');
            g.fillStyle = gr; g.fillRect(-64, -64, 128, 128);
        };
        cell(F.RING, () => radial(62, [[0, 0.03], [0.66, 0.08], [0.8, 1], [0.9, 0.45], [1, 0]]));
        cell(F.SHARD, () => {
            const lg = g.createLinearGradient(-40, -40, 40, 40);
            lg.addColorStop(0, '#fff'); lg.addColorStop(1, 'rgb(120,120,120)'); g.fillStyle = lg; poly([[-42, 8], [-10, -40], [36, -20], [46, 16], [6, 42]]); g.fill();
            g.fillStyle = 'rgba(255,255,255,0.8)'; poly([[-10, -40], [36, -20], [4, 0]]); g.fill();
        });
        cell(F.DROP, () => { radial(44, [[0, 1], [0.7, 0.9], [1, 0]]); blob(-12, -12, 14, 1, 255); });
        cell(F.STAR, () => {
            blob(0, 0, 62, 0.5, 255);
            const pts = [];
            for (let k = 0; k < 10; k++) { const r = k % 2 ? 22 : 52, a = -Math.PI / 2 + k * Math.PI / 5; pts.push([Math.cos(a) * r, Math.sin(a) * r]); }
            g.fillStyle = '#fff'; poly(pts); g.fill();
        });
        cell(F.BOLT, () => {
            const pts = [[-56, -6], [-30, 10], [-14, -12], [6, 12], [24, -10], [56, 8]];
            g.lineJoin = 'round';
            g.strokeStyle = 'rgba(255,255,255,0.35)'; g.lineWidth = 16; poly(pts, true); g.stroke();
            g.strokeStyle = '#fff'; g.lineWidth = 6; poly(pts, true); g.stroke();
        });
        cell(F.HEX, () => {
            const pts = []; for (let k = 0; k < 6; k++) pts.push([Math.cos(k * Math.PI / 3) * 52, Math.sin(k * Math.PI / 3) * 52]);
            g.fillStyle = 'rgba(255,255,255,0.16)'; poly(pts); g.fill();
            g.strokeStyle = '#fff'; g.lineWidth = 8; poly(pts); g.stroke();
        });
        cell(F.LEAF, () => {
            g.fillStyle = '#fff'; g.beginPath(); g.moveTo(-50, 0); g.quadraticCurveTo(0, -34, 50, 0); g.quadraticCurveTo(0, 34, -50, 0); g.fill();
            g.strokeStyle = 'rgb(160,160,160)'; g.lineWidth = 3; poly([[-50, 0], [46, 0]], true); g.stroke();
        });
        cell(F.GLOW, () => radial(62, [[0, 1], [0.2, 0.6], [0.5, 0.16], [1, 0]]));
        cell(F.STREAK, () => { g.save(); g.scale(1, 0.09); blob(0, 0, 62, 1, 255); blob(0, 0, 40, 1, 255); g.restore(); });
        cell(F.DOT, () => radial(42, [[0, 1], [0.8, 1], [1, 0]]));
        const tex = new T.CanvasTexture(cv);
        tex.generateMipmaps = false;
        tex.minFilter = T.LinearFilter; tex.magFilter = T.LinearFilter;
        tex.needsUpdate = true;
        return tex;
    }

    const PARTICLE_VS = [
        'attribute vec4 aPos; attribute vec4 aColor; attribute vec4 aMisc; attribute vec3 aVel;',
        'uniform vec3 uSunDir;',
        'varying vec2 vUv; varying vec2 vQ; varying vec4 vCol; varying float vLit; varying vec3 vL; varying float vDepth; varying float vSz;',
        'void main() {',
        '  vec4 mv = viewMatrix * vec4(aPos.xyz, 1.0);',
        '  float hs = aPos.w * 0.5; vec2 q = position.xy; vec2 off;',
        '  float c = cos(aMisc.x), s = sin(aMisc.x);',
        '  if (aMisc.z > 0.0) {',
        '    vec3 vv = (viewMatrix * vec4(aVel, 0.0)).xyz; float l = length(vv.xy);',
        '    vec2 d = l > 0.001 ? vv.xy / l : vec2(1.0, 0.0);',
        '    off = (d * q.x * (1.0 + aMisc.z * min(l * 0.09, 5.0)) + vec2(-d.y, d.x) * q.y) * hs;',
        '  } else off = vec2(c * q.x - s * q.y, s * q.x + c * q.y) * hs;',
        '  mv.xy += off;',
        '  vec3 L = normalize((viewMatrix * vec4(uSunDir, 0.0)).xyz);',
        '  vL = vec3(c * L.x + s * L.y, -s * L.x + c * L.y, L.z);',
        '  vQ = q; vCol = aColor; vLit = aMisc.w; vDepth = -mv.z; vSz = aPos.w;',
        '  vUv = (vec2(mod(aMisc.y, 4.0), 3.0 - floor(aMisc.y / 4.0)) + q * 0.5 + 0.5) / 4.0; gl_Position = projectionMatrix * mv;',
        '}'
    ].join('\n');
    /* Lit puffs get a fake sphere normal against the sun: the lit side takes the sun colour, the far side the sky. */
    const PARTICLE_FS = [
        'uniform sampler2D uMap; uniform vec3 uSunCol; uniform vec3 uAmb; uniform vec3 uFog; uniform vec2 uFogR; uniform float uAdd;',
        'varying vec2 vUv; varying vec2 vQ; varying vec4 vCol; varying float vLit; varying vec3 vL; varying float vDepth; varying float vSz;',
        'void main() {',
        '  vec4 tx = texture2D(uMap, vUv);',
        '  float a = tx.a * vCol.a * smoothstep(vSz * 0.35, vSz * 1.3 + 1.0, vDepth);',
        '  if (a < 0.003) discard; vec3 col = vCol.rgb;',
        '  if (vLit > 0.5) {',
        '    vec3 n = normalize(vec3(vQ, sqrt(max(0.0, 1.0 - dot(vQ, vQ))) * 0.9 + 0.1));',
        '    float d = clamp(dot(n, normalize(vL)) * 0.5 + 0.5, 0.0, 1.0);',
        '    col *= (uAmb + uSunCol * d * d) * (0.62 + 0.45 * tx.r);',
        '  } else if (uAdd < 0.5) col *= 0.7 + 0.3 * tx.r;',
        '  float f = smoothstep(uFogR.x, uFogR.y, vDepth);',
        '  if (uAdd > 0.5) a *= 1.0 - f; else col = mix(col, uFog, f);',
        '  gl_FragColor = vec4(col, a);',
        '#include <tonemapping_fragment>',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');
    const RING_VS = 'varying vec2 vP;\nvoid main() { vP = position.xz * 2.0; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }';
    const RING_FS = [
        'uniform vec3 uColor; uniform float uAlpha; uniform float uIn; uniform float uW; varying vec2 vP;',
        'void main() {',
        '  float d = length(vP); float w = uW;',
        '  float a = (smoothstep(uIn - w, uIn, d) * (1.0 - smoothstep(uIn, uIn + w * 0.3, d)) + (1.0 - smoothstep(0.0, uIn, d)) * 0.1) * uAlpha;',
        '  if (a < 0.003) discard; gl_FragColor = vec4(uColor, a);',
        '#include <tonemapping_fragment>',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');
    const BEAM_VS = [
        'uniform vec3 uA; uniform vec3 uB; uniform float uW0; uniform float uW1; uniform float uFlat; varying vec2 vUv;',
        'void main() {',
        '  float t = position.x * 0.5 + 0.5; vec3 p = mix(uA, uB, t); vec3 ax = uB - uA;',
        '  vec3 side = uFlat < 0.5 ? cross(ax, cameraPosition - p) : (uFlat < 1.5 ? cross(ax, vec3(0.0, 1.0, 0.0)) : vec3(0.0, 1.0, 0.0));',
        '  side = normalize(side + vec3(0.000001, 0.0, 0.0));',
        '  p += side * position.y * mix(uW0, uW1, t); vUv = vec2(t, position.y);',
        '  gl_Position = projectionMatrix * viewMatrix * vec4(p, 1.0);',
        '}'
    ].join('\n');
    const BEAM_FS = [
        'uniform vec3 uColor; uniform float uAlpha; uniform float uKind; uniform float uTime; uniform float uLen; varying vec2 vUv;',
        'void main() {',
        '  float ay = abs(vUv.y); float t = vUv.x; float a; vec3 col = uColor;',
        '  if (uKind < 0.5) {',
        '    float core = pow(1.0 - ay, 3.0);',
        '    a = core + pow(1.0 - ay, 1.6) * 0.4; col = mix(uColor, vec3(2.0), core * 0.7);',
        '    a *= smoothstep(0.0, 0.04, t) * (1.0 - smoothstep(0.92, 1.0, t));',
        '  } else if (uKind < 1.5) {',
        '    float line = 1.0 - smoothstep(0.35, 1.0, ay);',
        '    a = line * (0.65 + 0.35 * sin(t * uLen * 4.0 - uTime * 18.0 + vUv.y * 2.0)); col = mix(uColor, vec3(1.6), line * 0.5);',
        '  } else if (uKind < 2.5) {',
        '    float edge = pow(1.0 - ay, 1.4);',
        '    a = edge * (0.65 + 0.35 * sin(t * uLen * 0.9 + uTime * 5.0) * sin(vUv.y * 6.0 + uTime * 2.0)) * (0.25 + 0.75 * t);',
        '    col = mix(uColor, vec3(1.0), edge * 0.5);',
        '  } else {',
        '    a = pow(1.0 - ay, 2.0) * pow(1.0 - t, 1.1) * (0.75 + 0.25 * sin(t * 22.0 - uTime * 46.0 + vUv.y * 4.0));',
        '    col = mix(vec3(2.2, 1.7, 0.8), vec3(1.6, 0.35, 0.05), t);',
        '  }',
        '  gl_FragColor = vec4(col, a * uAlpha);',
        '#include <tonemapping_fragment>',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');
    /* Drift aura: a flat sigil on the road plus a rotating shell around the player's car. Both are
       pure geometry with a closed-form shader - no textures, no extra draw state per frame. */
    const AURA_G_VS = 'varying vec2 vP;\nvoid main() { vP = position.xz * 2.0; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }';
    const AURA_G_FS = [
        'uniform vec3 uColor; uniform vec3 uColor2; uniform float uAlpha; uniform float uSpin; varying vec2 vP;',
        'void main() {',
        '  float r = length(vP);',
        '  if (r > 1.0) discard;',
        '  float ang = atan(vP.y, vP.x);',
        '  float core = exp(-r * r * 7.0) * 0.5;',
        '  float edge = exp(-pow((r - 0.9) / 0.035, 2.0)) * 0.8;',
        '  float dash = pow(max(0.0, sin(ang * 9.0 - uSpin * 2.0)), 3.0);',
        '  float sweep = exp(-pow((r - 0.66) / 0.05, 2.0)) * (0.25 + 0.75 * dash);',
        '  float inner = exp(-pow((r - 0.42) / 0.07, 2.0)) * 0.3;',
        '  float a = (core + edge + sweep + inner) * uAlpha * smoothstep(1.0, 0.9, r);',
        '  if (a < 0.004) discard;',
        '  vec3 col = mix(uColor2, uColor, clamp(r * 1.15, 0.0, 1.0));',
        '  gl_FragColor = vec4(col, a);',
        '#include <tonemapping_fragment>',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');
    const AURA_S_VS = 'varying vec2 vUv;\nvoid main() { vUv = uv; gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0); }';
    const AURA_S_FS = [
        'uniform vec3 uColor; uniform float uAlpha; uniform float uSpin; varying vec2 vUv;',
        'void main() {',
        '  float up = vUv.y;',
        '  float band = pow(max(0.0, sin(vUv.x * 31.4159 - uSpin * 2.4)), 4.0);',
        '  float a = (pow(up, 2.6) * 0.42 + band * pow(up, 2.0) * 0.8) * uAlpha;',
        '  if (a < 0.004) discard;',
        '  gl_FragColor = vec4(uColor * (0.65 + 0.65 * band), a);',
        '#include <tonemapping_fragment>',
        '#include <colorspace_fragment>',
        '}'
    ].join('\n');

    AD.createFxResources = function (T) {
        return { atlas: buildAtlas(T), frames: F,
            PARTICLE_VS, PARTICLE_FS, RING_VS, RING_FS, BEAM_VS, BEAM_FS, AURA_G_VS, AURA_G_FS, AURA_S_VS, AURA_S_FS };
    };
})();
