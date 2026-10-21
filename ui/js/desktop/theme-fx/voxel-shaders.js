(function () {
    'use strict';

    // GLSL of the floating-isle diorama (voxel-scene.js): the screen-space sky and the tilt-shift composite pass.

    function srgbToLinear(c) { return c <= .04045 ? c / 12.92 : Math.pow((c + .055) / 1.055, 2.4); }
    function hexRGB(hex) {
        const n = parseInt(hex.slice(1), 16);
        return [(n >> 16 & 255) / 255, (n >> 8 & 255) / 255, (n & 255) / 255];
    }

    function glslFloat(value) { return Number.isInteger(value) ? value.toFixed(1) : String(Number(value.toFixed(5))); }
    function glslVec2(pair) { return 'vec2(' + pair.map(glslFloat).join(', ') + ')'; }
    function glslVec3(rgb) { return 'vec3(' + rgb.map(glslFloat).join(', ') + ')'; }
    function linearHex(hex, gain) { return hexRGB(hex).map(c => srgbToLinear(c) * (gain || 1)); }

    // The sky is one full-screen triangle pair drawn first: a vertical gradient over the picture height, the sun and the moon as
    // crisp pixel squares, and a pixel star field. Everything is in screen pixels (gl_FragCoord), so no horizon is needed.
    const SKY_VERTEX = `
void main() {
    gl_Position = vec4(position.xy, 1.0, 1.0);
}`;

    // sky: the SKY tunables of voxel-scene.js (horizonAt, sun, moon, stars).
    function skyFragment(sky) {
        const sun = sky.sun;
        const moon = sky.moon;
        const stars = sky.stars;
        const haloCode = sun.halos.map(h => '    col = mix(col, uSunColor, pxSquare(p, sunC, sunHalf * ' + glslFloat(h[0]) + ') * ' + glslFloat(h[1]) + ' * uSunVisible);').join('\n');
        const craterCode = moon.craters.map(c => '    crater += step(abs(mc.x - ' + glslFloat(c[0]) + '), 0.01) * step(abs(mc.y - ' + glslFloat(c[1]) + '), 0.01);').join('\n');
        return `
uniform vec3 uTop;
uniform vec3 uHorizon;
uniform vec3 uBottom;
uniform vec3 uKey;
uniform vec3 uSunColor;
uniform vec2 uSunPos;
uniform vec2 uMoonPos;
uniform vec2 uResolution;
uniform float uSunVisible;
uniform float uMoonVisible;
uniform float uStars;
uniform float uTime;
uniform float uYaw;
float skyHash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }
// 1 inside the axis-aligned square of half size h around c, else 0
float pxSquare(vec2 p, vec2 c, float h) { vec2 d = abs(p - c); return step(max(d.x, d.y), h); }
float pxRange(float v, float lo, float hi) { return step(lo, v) * (1.0 - step(hi, v)); }
void main() {
    float H = uResolution.y;
    vec2 p = vec2(gl_FragCoord.x, H - gl_FragCoord.y);
    float sy = p.y / H;
    vec3 col = mix(mix(uTop, uHorizon, smoothstep(0.0, ${glslFloat(sky.horizonAt)}, sy)), uBottom, smoothstep(${glslFloat(sky.horizonAt)}, 1.0, sy));

    // Stars: a grid of cells; a hashed share of them holds a 2x2 star, a few a bright 3x3 plus. The grid scales with the height and
    // slides sideways with the camera yaw.
    float unit = H / 1080.0;
    vec2 lp = vec2(p.x / unit + uYaw * ${glslFloat(stars.parallax)}, p.y / unit);
    vec2 cell = floor(lp / ${glslFloat(stars.cell)});
    vec2 q = lp - cell * ${glslFloat(stars.cell)} - floor(vec2(skyHash(cell + 3.1), skyHash(cell + 9.4)) * ${glslFloat(stars.cell - 3)});
    float h = skyHash(cell);
    float density = 1.0 - smoothstep(${glslFloat(stars.fadeFrom)}, ${glslFloat(stars.fadeTo)}, sy);
    float twinkle = 0.55 + 0.45 * sin(uTime * (1.0 + h * 3.0) + h * 40.0);
    float small = step(h, ${glslFloat(stars.small)} * density) * pxRange(q.x, 0.0, 2.0) * pxRange(q.y, 0.0, 2.0) * 0.75 * twinkle;
    float plus = max(pxRange(q.x, 1.0, 2.0) * pxRange(q.y, 0.0, 3.0), pxRange(q.y, 1.0, 2.0) * pxRange(q.x, 0.0, 3.0));
    float bright = step(1.0 - ${glslFloat(stars.bright)} * density, h) * plus * (0.7 + 0.3 * twinkle);
    col += vec3(0.82, 0.88, 1.0) * (small + bright) * uStars * ${glslFloat(stars.gain)};

    // Sun: a soft glow, two stepped square halos and a crisp square.
    vec2 sunC = uSunPos * uResolution;
    float sunHalf = ${glslFloat(sun.size)} * H * 0.5;
    col += uKey * exp(-length(p - sunC) / (sunHalf * ${glslFloat(sun.glowReach)})) * ${glslFloat(sun.glow)} * uSunVisible;
${haloCode}
    col = mix(col, uSunColor, pxSquare(p, sunC, sunHalf) * uSunVisible);

    // Moon: a faint blue square halo and a pixel-art crescent: a disc of moon.cells blocks across minus an offset disc, with a few
    // darker one-block craters. mc is the block centre relative to the moon's centre, in blocks.
    vec2 moonC = uMoonPos * uResolution;
    float moonSize = ${glslFloat(moon.size)} * H;
    col = mix(col, ${glslVec3(linearHex(moon.haloColor, moon.haloGain))}, pxSquare(p, moonC, moonSize * 0.5 * ${glslFloat(moon.halo)}) * ${glslFloat(moon.haloAlpha)} * uMoonVisible);
    vec2 mc = floor((p - moonC) / (moonSize / ${glslFloat(moon.cells)})) + 0.5;
    float lit = step(length(mc), ${glslFloat(moon.cells / 2)}) * (1.0 - step(length(mc - ${glslVec2(moon.cut)}), ${glslFloat(moon.cutRadius)}));
    float crater = 0.0;
${craterCode}
    col = mix(col, ${glslVec3(linearHex(moon.color, moon.gain))} * (1.0 - ${glslFloat(moon.crater)} * min(crater, 1.0)), lit * uMoonVisible);

    gl_FragColor = vec4(col, 1.0);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
}`;
    }

    const POST_VERTEX = `
varying vec2 vUv;
void main() {
    vUv = uv;
    gl_Position = vec4(position.xy, 0.0, 1.0);
}`;

    // Tilt-shift: a 12-tap golden-angle disc blur that grows towards the top and bottom of the picture, plus a
    // light vignette and a little extra saturation. It runs in linear light and ends with the renderer's tone mapping
    // and colour conversion.
    const POST_FRAGMENT = `
uniform sampler2D tDiffuse;
uniform vec2 uPx;
uniform float uMaxBlur;
uniform float uBand;
uniform float uFeather;
uniform float uFocusY;
uniform float uVignette;
uniform float uSaturation;
varying vec2 vUv;
void main() {
    float amount = smoothstep(uBand, uBand + uFeather, abs(vUv.y - uFocusY));
    float radius = uMaxBlur * amount;
    vec3 color = texture2D(tDiffuse, vUv).rgb;
    if (radius > 0.25) {
        vec3 sum = color;
        for (int i = 0; i < 12; i++) {
            float fi = float(i) + 0.5;
            float r = sqrt(fi / 12.0) * radius;
            float a = fi * 2.39996323;
            sum += texture2D(tDiffuse, vUv + vec2(cos(a), sin(a)) * r * uPx).rgb;
        }
        color = sum / 13.0;
    }
    color *= 1.0 - uVignette * pow(length((vUv - 0.5) * vec2(1.1, 1.0)) * 1.25, 2.2);
    color = mix(vec3(dot(color, vec3(0.2126, 0.7152, 0.0722))), color, uSaturation);
    gl_FragColor = vec4(color, 1.0);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
}`;

    window.AuraVoxelShaders = Object.freeze({ SKY_VERTEX, skyFragment, POST_VERTEX, POST_FRAGMENT });
})();
