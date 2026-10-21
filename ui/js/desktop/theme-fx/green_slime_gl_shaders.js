(function () {
    'use strict';

    // GLSL ES 3.00 sources of the Green Slime WebGL2 renderer (green_slime_gl.js). Three programs:
    //  - gel: the slime around one window, drawn as strip quads (field -> height -> normal -> material),
    //  - drop: free drops, bubble spray and taskbar splats as instanced quads with the same material,
    //  - mask: instanced rounded rectangles that fill the stencil (drops never paint over windows).
    // Units: CSS pixels, y points down, output is premultiplied. The arclength walk of projectRR() mirrors
    // green_slime_geometry.js (layout): top edge, top-right arc, right edge, bottom-right arc, bottom edge,
    // bottom-left arc, left edge, top-left arc, clockwise from the end of the top-left arc.

    const PREAMBLE = [
        '#version 300 es',
        'precision highp float;',
        'precision highp int;',
        'precision highp sampler2D;',
        ''
    ].join('\n');

    // ---------------------------------------------------------------------------------------------
    // Shared by the gel and the drop program: noise, backdrop, environment and the slime material.

    const COMMON = `
uniform vec4 uView;
uniform vec2 uSize;
uniform vec4 uFrame;
uniform vec4 uBack;
uniform sampler2D uBackdrop;

const vec3 SIGMA = vec3(0.020, 0.014, 0.042);
const vec3 LIGHT = vec3(-0.4198, -0.6996, 0.5784);
const vec3 HALFV = vec3(-0.2363, -0.3938, 0.8884);
const vec2 FOCUS = vec2(0.5145, 0.8575);

float hash21(vec2 p) {
    vec3 p3 = fract(vec3(p.xyx) * 0.1031);
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.x + p3.y) * p3.z);
}

float vnoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    return mix(mix(hash21(i), hash21(i + vec2(1.0, 0.0)), f.x),
               mix(hash21(i + vec2(0.0, 1.0)), hash21(i + vec2(1.0, 1.0)), f.x), f.y);
}

vec2 cssPos() {
    return vec2(gl_FragCoord.x * uView.x, (uView.z - gl_FragCoord.y) * uView.y);
}

vec3 backdropAt(vec2 q, float lod) {
    return textureLod(uBackdrop, (q - uBack.xy) / uBack.zw, lod).rgb;
}

float boxMask(vec2 r, vec2 centre, vec2 hs, float angle, float soft) {
    vec2 a = r - centre;
    float c = cos(angle), s = sin(angle);
    vec2 b = vec2(c * a.x + s * a.y, -s * a.x + c * a.y);
    vec2 q = abs(b) - hs;
    float sd = min(max(q.x, q.y), 0.0) + length(max(q, 0.0)) - 0.05;
    return 1.0 - smoothstep(-soft * 0.4, soft, sd);
}

// The soft box seen in the gel: a bright window up-left (wide for the top edges, tall for the left ones).
float softBox(vec2 r, float fill) {
    float wide = boxMask(r, vec2(-0.30, -0.50), vec2(0.34, 0.12), -0.52, 0.16);
    float tall = boxMask(r, vec2(-0.60, 0.04), vec2(0.075, 0.27), 0.12, 0.13) * 0.75;
    // A dimmer fill card lower right gives the flat bottom and right flanks of the gel their own streak
    // of light (fill = 0 leaves it out: round bodies like drops and domes show the key light only).
    float fillA = boxMask(r, vec2(0.30, 0.54), vec2(0.36, 0.10), 0.45, 0.16) * 0.55;
    float fillB = boxMask(r, vec2(0.56, 0.02), vec2(0.09, 0.34), -0.15, 0.14) * 0.45;
    return max(max(wide, tall), max(fillA, fillB) * fill);
}

vec3 environment(vec3 r, float fill) {
    vec3 sky = mix(vec3(0.15, 0.23, 0.07), vec3(0.74, 0.88, 0.52), smoothstep(-0.25, 0.65, -r.y));
    return sky + vec3(1.0, 1.0, 0.92) * softBox(r.xy, fill) * 1.35;
}

// The slime material. q: where the backdrop is looked up (the refracted position), h: path thickness in px,
// n: surface normal, texW: how much of the real backdrop is refracted (1) versus left to the DOM below
// (0, translucent), aMin: opacity floor of the translucent part, rimK: strength of the dark refracted rim.
vec4 shadeSlime(vec2 q, float h, vec3 n, float texW, float aMin, float rimK, float gloss, float thin) {
    float ndv = clamp(n.z, 0.0, 1.0);
    float tilt = sqrt(max(1.0 - ndv * ndv, 0.0));
    float F = 0.025 + 0.975 * pow(1.0 - ndv, 5.0);
    float path = h / max(ndv, 0.35);
    vec3 Tr = exp(-SIGMA * path);
    float wrapL = clamp(dot(n, LIGHT) * 0.5 + 0.5, 0.0, 1.0);
    vec2 nd = normalize(n.xy + vec2(1e-5));
    // Which way the flank faces: lit toward the upper-left light, low toward the opposite side.
    float toLight = dot(nd, vec2(-0.5145, -0.8575));
    float lit = clamp(toLight * 1.4 - 0.1, 0.0, 1.0);
    float low = clamp(-toLight * 1.3 - 0.05, 0.0, 1.0);
    vec3 scatter = vec3(0.62, 0.82, 0.22) * (1.0 - exp(-0.05 * h)) * (0.35 + 0.65 * wrapL) * 0.19;
    // Focused light: a bright lime band just inside the dark lower rim, and a broad faint glow.
    float caustBand = exp(-pow((tilt - 0.40) / 0.14, 2.0)) * low * smoothstep(1.0, 5.0, h);
    float caustWide = pow(clamp(-toLight, 0.0, 1.0), 3.0) * smoothstep(0.04, 0.5, tilt) * smoothstep(1.0, 6.0, h);
    vec3 S = scatter + vec3(0.80, 0.98, 0.36) * (caustBand * 0.30 + caustWide * 0.10) * gloss;
    float aT = clamp(1.0 - dot(Tr, vec3(0.3333)) + 0.12, aMin, 1.0);
    // Without the real backdrop (translucent mode) a typical wallpaper tone stands in for it, so the gel
    // keeps its body colour near the frame and turns see-through only where it is thin.
    vec4 col = vec4((vec3(0.50, 0.62, 0.26) * Tr + S) * aT, aT);
    if (texW > 0.0) {
        vec3 bg = backdropAt(q, 0.25 + 0.04 * h);
        col = mix(col, vec4(bg * Tr + S, 1.0), texW);
    }
    // Contrast: darker and more saturated toward the lower inner side, then the dark refracted rim.
    col.rgb *= mix(vec3(1.0), vec3(0.78, 0.92, 0.60), low * smoothstep(0.12, 0.5, tilt) * 0.8);
    col.rgb *= mix(0.92, 1.03, wrapL);
    col = mix(col, vec4(0.10, 0.20, 0.03, 1.0), pow(1.0 - ndv, 2.5) * rimK);
    vec3 Rr = reflect(vec3(0.0, 0.0, -1.0), n);
    float fill = clamp(1.0 - (gloss - 1.0) * 2.0, 0.0, 1.0);
    col = mix(col, vec4(environment(Rr, fill), 1.0), F);
    // A narrow darker gap inside the reflected line, as on the wallpaper beads.
    col.rgb *= 1.0 - 0.32 * exp(-pow((tilt - 0.43) / 0.07, 2.0)) * lit;
    float nh = max(dot(n, HALFV), 0.0);
    // The soft box is far brighter than the sky: a gloss term keeps its reflection visible at low Fresnel.
    vec3 spec = vec3(1.0, 0.99, 0.9) * (pow(nh, 240.0) * 2.0 + pow(nh, 32.0) * 0.22 + softBox(Rr.xy * (1.0 / gloss), fill) * (0.22 + 0.6 * F) * gloss);
    // The soft box on a curved flank: a crisp thin white line near the lit silhouette, a soft sheen on the crest.
    float lineW = 0.05 + 0.25 * thin;
    spec += vec3(1.0, 1.0, 0.94) * (exp(-pow((tilt - 0.60) / lineW, 2.0)) * 1.05 + exp(-pow((tilt - 0.18) / 0.10, 2.0)) * 0.20) * lit;
    // Rim light at the very edge: toward the light a thin bright line, opposite a dim one.
    spec += vec3(1.0, 1.0, 0.9) * pow(1.0 - ndv, 3.0) * (max(toLight, 0.0) * 0.8 + max(-toLight, 0.0) * 0.30) * gloss;
    col.rgb += spec;
    col.a = min(1.0, col.a + max(spec.r, max(spec.g, spec.b)));
    return col;
}
`;

    // ---------------------------------------------------------------------------------------------
    // Gel program.

    const GEL_VERTEX = PREAMBLE + `
uniform vec2 uSize;
in vec2 aPos;
void main() {
    gl_Position = vec4(aPos.x / uSize.x * 2.0 - 1.0, 1.0 - aPos.y / uSize.y * 2.0, 0.0, 1.0);
}
`;

    const GEL_FIELD = `
uniform vec4 uRect;
uniform vec4 uRadii;
uniform vec4 uLenA;
uniform vec4 uLenB;
uniform vec4 uWin;
uniform vec4 uCounts;
uniform vec4 uDrip[12];
uniform vec4 uBub[12];
uniform vec4 uStrand[4];
uniform vec4 uOcc[24];
uniform vec4 uOccR[6];
uniform vec4 uUnder[16];
uniform vec4 uBand;
uniform float uPass;
uniform sampler2D uRadius;

const float PI = 3.14159265;

struct Gel {
    float d;
    float rl;
    float s;
    float dw;
    vec2 nrm;
    // The strand's share of the field (x) and, where it dominates, the unit vector across the thread
    // (yz) and the position across it from the axis (0) to the silhouette (1) in w.
    vec4 thr;
};

// Arclength s, distance outside the rounded rectangle and the outward normal of the nearest perimeter point.
vec4 projectRR(vec2 p) {
    vec2 hs = uRect.zw * 0.5;
    vec2 q = p - (uRect.xy + hs);
    float r = q.y < 0.0 ? (q.x < 0.0 ? uRadii.x : uRadii.y) : (q.x < 0.0 ? uRadii.w : uRadii.z);
    vec2 a = abs(q) - (hs - r);
    float dist = min(max(a.x, a.y), 0.0) + length(max(a, 0.0)) - r;
    float st1 = uLenA.x;
    float st2 = st1 + uLenA.y;
    float st3 = st2 + uLenA.z;
    float st4 = st3 + uLenA.w;
    float st5 = st4 + uLenB.x;
    float st6 = st5 + uLenB.y;
    float st7 = st6 + uLenB.z;
    float s;
    vec2 nrm;
    if (a.x > 0.0 && a.y > 0.0) {
        vec2 c;
        float t;
        if (q.y < 0.0) {
            if (q.x >= 0.0) {
                c = vec2(uRect.x + uRect.z - uRadii.y, uRect.y + uRadii.y);
                vec2 v = p - c;
                t = clamp((atan(v.y, v.x) + PI * 0.5) / (PI * 0.5), 0.0, 1.0);
                s = st1 + t * uLenA.y;
                nrm = v;
            } else {
                c = vec2(uRect.x + uRadii.x, uRect.y + uRadii.x);
                vec2 v = p - c;
                t = clamp((atan(v.y, v.x) + PI) / (PI * 0.5), 0.0, 1.0);
                s = st7 + t * uLenB.w;
                nrm = v;
            }
        } else {
            if (q.x >= 0.0) {
                c = vec2(uRect.x + uRect.z - uRadii.z, uRect.y + uRect.w - uRadii.z);
                vec2 v = p - c;
                t = clamp(atan(v.y, v.x) / (PI * 0.5), 0.0, 1.0);
                s = st3 + t * uLenA.w;
                nrm = v;
            } else {
                c = vec2(uRect.x + uRadii.w, uRect.y + uRect.w - uRadii.w);
                vec2 v = p - c;
                t = clamp((atan(v.y, v.x) - PI * 0.5) / (PI * 0.5), 0.0, 1.0);
                s = st5 + t * uLenB.y;
                nrm = v;
            }
        }
        float nl = length(nrm);
        nrm = nl > 1e-4 ? nrm / nl : vec2(0.0, -1.0);
    } else if (a.x > a.y) {
        if (q.x > 0.0) {
            s = st2 + clamp(p.y - (uRect.y + uRadii.y), 0.0, uLenA.z);
            nrm = vec2(1.0, 0.0);
        } else {
            s = st6 + clamp((uRect.y + uRect.w - uRadii.w) - p.y, 0.0, uLenB.z);
            nrm = vec2(-1.0, 0.0);
        }
    } else {
        if (q.y > 0.0) {
            s = st4 + clamp((uRect.x + uRect.z - uRadii.z) - p.x, 0.0, uLenB.x);
            nrm = vec2(0.0, 1.0);
        } else {
            s = clamp(p.x - (uRect.x + uRadii.x), 0.0, uLenA.x);
            nrm = vec2(0.0, -1.0);
        }
    }
    return vec4(s, dist, nrm);
}

float radiusTexel(int k, int row) {
    return texelFetch(uRadius, ivec2(k & 127, row), 0).r;
}

// Catmull-Rom over the 128 samples of the window's radius table (periodic).
float tableRadius(float s) {
    float f = s / uWin.z * 128.0 - 0.5;
    float fl = floor(f);
    float t = f - fl;
    int k = int(fl);
    int row = int(uWin.x + 0.5);
    float p0 = radiusTexel(k - 1, row);
    float p1 = radiusTexel(k, row);
    float p2 = radiusTexel(k + 1, row);
    float p3 = radiusTexel(k + 2, row);
    return 0.5 * (2.0 * p1 + (p2 - p0) * t + (2.0 * p0 - 5.0 * p1 + 4.0 * p2 - p3) * t * t + (3.0 * p1 - p0 - 3.0 * p2 + p3) * t * t * t);
}

vec2 sminR(vec2 a, vec2 b, float k) {
    float h = clamp(0.5 + 0.5 * (b.x - a.x) / k, 0.0, 1.0);
    return vec2(mix(b.x, a.x, h) - k * h * (1.0 - h), mix(b.y, a.y, h));
}

vec2 capsuleR(vec2 p, vec2 a, vec2 b, float ra, float rb) {
    vec2 pa = p - a;
    vec2 ba = b - a;
    float h = clamp(dot(pa, ba) / max(dot(ba, ba), 1e-4), 0.0, 1.0);
    float r = mix(ra, rb, h);
    return vec2(length(pa - ba * h) - r, r);
}

vec2 dripField(vec2 p, int i) {
    vec4 a = uDrip[2 * i];
    vec4 b = uDrip[2 * i + 1];
    if (p.y < a.y - 14.0 || p.y > a.y + a.z + b.z + 22.0 || abs(p.x - a.x) > 48.0) return vec2(1000.0, 6.0);
    vec2 tip = vec2(a.x + a.w, a.y + a.z);
    vec2 body = capsuleR(p, a.xy, tip - vec2(0.0, b.z * 0.5), b.x, b.y);
    vec2 bulb = vec2(length(p - tip) - b.z, b.z);
    return sminR(body, bulb, 5.0);
}

vec2 bezier2(vec2 a, vec2 c, vec2 b, float u) {
    float m = 1.0 - u;
    return m * m * a + 2.0 * m * u * c + u * u * b;
}

// Position across a round part: the unit vector from its axis (or centre) to the point and the distance
// in units of the radius (0 on the axis, 1 on the silhouette).
vec3 acrossInfo(vec2 p, vec2 axisPoint, float radius) {
    vec2 v = p - axisPoint;
    float l = length(v);
    return vec3(l > 1e-3 ? v / l : vec2(0.0, -1.0), clamp(l / max(radius, 0.3), 0.0, 1.0));
}

// The drawn-out thread: a polyline through the drooping curve with a short flare where it leaves the gel
// (a meniscus neck, 6-7 px wide at the surface), a body never thinner than 1.25 px, round beads once it
// is thin, and a tip bulb. info carries the across-vector and position of whichever part is nearest.
vec2 strandField(vec2 p, out vec3 info) {
    info = vec3(0.0, -1.0, 1.0);
    vec4 s0 = uStrand[0];
    vec4 s1 = uStrand[1];
    vec4 s2 = uStrand[2];
    if (p.x < s2.x || p.x > s2.z || p.y < s2.y || p.y > s2.w) return vec2(1000.0, 3.0);
    vec2 A = s0.xy;
    vec2 B = s0.zw;
    float len = distance(A, B);
    vec2 C = (A + B) * 0.5 + vec2(0.0, min(16.0, len * 0.09));
    vec2 P1 = bezier2(A, C, B, 0.3333);
    vec2 P2 = bezier2(A, C, B, 0.6667);
    float best = 1000.0;
    float bu = 0.0;
    vec2 axis = A;
    for (int i = 0; i < 3; i++) {
        vec2 a = i == 0 ? A : (i == 1 ? P1 : P2);
        vec2 b = i == 0 ? P1 : (i == 1 ? P2 : B);
        vec2 pa = p - a;
        vec2 ba = b - a;
        float h = clamp(dot(pa, ba) / max(dot(ba, ba), 1e-4), 0.0, 1.0);
        float d = length(pa - ba * h);
        if (d < best) { best = d; bu = (float(i) + h) / 3.0; axis = a + ba * h; }
    }
    float mid = max(s1.y * 1.4, 2.2);
    float neck = s1.w > 0.5 ? 0.82 : 1.0;
    // Where the thread leaves the surface (uStrand[3].x below the anchor) it swells a little; the
    // smooth union with the band does the rest, a concave meniscus fillet 6-7 px wide instead of a cone.
    float outAt = max(bu * len - uStrand[3].x, 0.0);
    float flare = 2.4 * exp(-outAt / 7.0);
    float r = mid * neck + flare + (s1.z - mid) * pow(bu, 5.0);
    vec2 f = vec2(best - r, r);
    info = acrossInfo(p, axis, r);
    int beads = int(s1.w + 0.5);
    for (int j = 0; j < beads; j++) {
        float centre = (float(j) + 1.0) / (s1.w + 1.0) + 0.012 * sin(uFrame.x * 2.1 + float(j) * 1.7);
        vec2 bc = bezier2(A, C, B, centre);
        float rb = mid * 2.3 + 0.4;
        vec2 bead = vec2(distance(p, bc) - rb, rb);
        if (bead.x < f.x) info = acrossInfo(p, bc, rb);
        f = sminR(f, bead, 2.2);
    }
    float rt = max(s1.z * 1.15, 3.2);
    vec2 tipS = vec2(distance(p, B) - rt, rt);
    if (tipS.x < f.x) info = acrossInfo(p, B, rt);
    return sminR(f, tipS, 4.0);
}

// The gel field at a point: smooth union of the window's band, its drips and the strand.
Gel gelAt(vec2 p) {
    Gel g;
    vec4 pr = projectRR(p);
    float R = tableRadius(pr.x);
    g.s = pr.x;
    g.dw = pr.y;
    g.nrm = pr.zw;
    vec2 f = vec2(pr.y - R, max(R, 4.0));
    int nd = int(uCounts.x + 0.5);
    for (int i = 0; i < nd; i++) f = sminR(f, dripField(p, i), 9.0);
    g.thr = vec4(0.0, 0.0, -1.0, 1.0);
    if (uWin.w > 0.5) {
        vec3 across;
        vec2 sf = strandField(p, across);
        // Inside the gel the thread is hidden anyway; a wide blend there would only raise a cone in the
        // surface, so the fillet radius grows from nothing to 14 px as the outline is approached.
        float kS = 14.0 * smoothstep(-9.0, -1.0, f.x) + 0.01;
        g.thr = vec4(clamp(0.5 - 0.5 * (sf.x - f.x) / kS, 0.0, 1.0), across);
        f = sminR(f, sf, kS);
    }
    g.d = f.x;
    g.rl = f.y;
    return g;
}

// Crater of a burst bubble (damped ring, central rebound) and the meniscus ring around a dome's base.
float bubbleHeight(vec2 p) {
    float h = 0.0;
    int nb = int(uCounts.y + 0.5);
    for (int i = 0; i < nb; i++) {
        vec4 b0 = uBub[4 * i];
        vec4 b1 = uBub[4 * i + 1];
        vec4 b3 = uBub[4 * i + 3];
        float a = b0.w;
        float rho = distance(p, b0.xy);
        if (rho > a * 3.4 + 8.0) continue;
        int st = int(b1.y + 0.5);
        if (st == 4) {
            float bowl = 1.0 - smoothstep(0.0, 1.1 * a, rho);
            float lipX = (rho - 1.15 * a) / (0.3 * a + 0.6);
            float waveX = rho - a * (1.0 + 2.2 * b1.z);
            h += a * (-0.5 * b3.x * bowl + 0.2 * b3.x * exp(-lipX * lipX));
            h += 0.10 * a * (1.0 - b1.z) * exp(-waveX * waveX / (0.12 * a * a + 1.5));
            float reX = rho / (0.30 * a + 0.8);
            h += b3.y * a * 0.55 * exp(-reX * reX);
        } else if (st >= 1) {
            float ringX = (rho - a - 0.4) / 1.5;
            h += min(b1.x, 1.0) * 1.3 * exp(-ringX * ringX);
        }
    }
    return h;
}

// Thickness of the gel in px: circular profile from the outline, wetting dip at the frame, a slow
// flow undulation and a fine relief that breaks up the sharp highlight.
float gelHeight(vec2 p, Gel g) {
    float rl = max(g.rl, 0.6);
    float h = 0.95 * sqrt(max(0.0, -g.d * (2.0 * rl + g.d)));
    h *= 0.86 + 0.14 * smoothstep(0.0, 2.5, g.dw);
    // A free thread is as thick as it is wide: the full chord of its round section.
    float chord = 1.8 * rl * sqrt(max(1.0 - g.thr.w * g.thr.w, 0.0));
    h = mix(h, chord, smoothstep(0.3, 0.9, g.thr.x));
    // Band coordinates (arclength as whole periods, distance from the frame): the waves run along the
    // gel and drift downward on the sides, along the band elsewhere.
    float u = g.s / uWin.z * 6.2831853;
    float ph = uWin.y * 6.2831853;
    float flow = (1.0 - 2.0 * smoothstep(0.35, 0.8, -g.nrm.x)) * uFrame.x;
    float und = 0.70 * sin(u * 13.0 + g.dw * 0.06 - flow * 0.45 + ph)
              + 0.45 * sin(u * 31.0 - g.dw * 0.10 - flow * 0.35 + ph * 2.3)
              + 0.25 * sin(u * 57.0 + g.dw * 0.16 - flow * 0.70 + ph * 3.7);
    float micro = (vnoise(p * 0.30 + 7.3) - 0.5) * 0.10;
    // The relief belongs to the broad gel, not to a thread or a drip a few pixels wide.
    h += (und + micro) * smoothstep(0.0, 4.0, -g.d) * smoothstep(2.5, 9.0, rl) * (1.0 - smoothstep(0.05, 0.5, g.thr.x));
    h += bubbleHeight(p) * smoothstep(0.0, 3.0, -g.d);
    return max(h, 0.0);
}

bool blockedAt(vec2 p) {
    int n = int(uCounts.z + 0.5);
    for (int i = 0; i < n; i++) {
        vec4 o = uOcc[i];
        float r = uOccR[i >> 2][i & 3];
        vec2 c = (o.xy + o.zw) * 0.5;
        vec2 hs = (o.zw - o.xy) * 0.5;
        vec2 q = abs(p - c) - (hs - r);
        float sd = min(max(q.x, q.y), 0.0) + length(max(q, 0.0)) - r;
        if (sd < 0.0) return true;
    }
    return false;
}

bool underAt(vec2 p) {
    int n = int(uCounts.w + 0.5);
    for (int i = 0; i < n; i++) {
        vec4 o = uUnder[i];
        if (p.x >= o.x && p.x <= o.z && p.y >= o.y && p.y <= o.w) return true;
    }
    return false;
}
`;

    // Trapped air bubbles and sparse particles inside the gel, in band coordinates (arclength, distance
    // from the frame) so they drift with the flow; deterministic per window.
    const GEL_DETAIL = `
vec4 suspended(Gel g) {
    float total = uWin.z;
    float cells = max(floor(total / 24.0), 4.0);
    float cs = total / cells;
    float u = g.s - uFrame.x * 1.4;
    float uc = u / cs;
    float ci = mod(floor(uc), cells);
    float fu = fract(uc);
    float rowH = 12.0;
    float rv = floor(g.dw / rowH);
    float fv = fract(g.dw / rowH);
    vec2 cell = vec2(ci, rv) + uWin.y * 97.0;
    vec4 out4 = vec4(0.0);
    float present = hash21(cell + 3.1);
    float cx = 0.32 + 0.36 * hash21(cell + 11.7);
    float cy = 0.32 + 0.36 * hash21(cell + 23.9);
    vec2 d = vec2((fu - cx) * cs, (fv - cy) * rowH);
    float centreDw = (rv + cy) * rowH;
    if (present < 0.11 && centreDw > 5.0 && centreDw < g.rl - 5.0) {
        float rad = 1.1 + 1.5 * hash21(cell + 41.3);
        float rho = length(d) / rad;
        if (rho < 1.15) {
            float cover = 1.0 - smoothstep(0.9, 1.1, rho);
            float ring = smoothstep(0.55, 0.95, rho);
            vec4 lens = vec4(0.88, 0.98, 0.62, 0.22) * (1.0 - rho * rho);
            lens = mix(lens, vec4(0.04, 0.11, 0.02, 0.40), ring);
            vec2 hl = d / rad - vec2(-0.38, -0.42);
            float spot = exp(-dot(hl, hl) / 0.05);
            lens = mix(lens, vec4(1.0, 1.0, 0.94, 1.0), spot * 0.85);
            out4 = vec4(lens.rgb * lens.a, lens.a) * cover;
        }
    } else if (present > 0.9 && centreDw > 4.0 && centreDw < g.rl - 4.0) {
        float rho = length(d) / (0.5 + 0.5 * hash21(cell + 5.5));
        float cover = exp(-rho * rho);
        float bright = hash21(cell + 71.0);
        out4 = bright > 0.5 ? vec4(vec3(0.9, 1.0, 0.7) * 0.35, 0.35) * cover : vec4(0.0, 0.0, 0.0, 0.3) * cover;
    }
    return out4;
}

// Gas pocket inside the gel (nucleate): the chord through the gel is gone, so the centre is clearer;
// the rim is dark (total reflection), a thin arc of focused light sits lower right, a pin highlight up left.
vec4 pocketLayer(vec4 col, vec2 p, vec4 b0) {
    vec2 d = p - b0.xy;
    float r = max(b0.w, 1.0);
    float len = length(d);
    float dpx = len - r;
    if (dpx > 1.2) return col;
    // Everything is measured in pixels from the rim, so even a 2 px lens stays a round, smooth disc.
    float cover = clamp(0.5 - dpx, 0.0, 1.0);
    float rho = min(len / r, 1.0);
    float chord = sqrt(max(1.0 - rho * rho, 0.0));
    vec4 res = col;
    res.rgb = mix(col.rgb, min(col.rgb * exp(SIGMA * chord * 7.0), vec3(1.2)) + vec3(0.06, 0.08, 0.025) * chord, 0.9);
    res.a = col.a * (1.0 - 0.3 * chord);
    float ring = exp(-pow((dpx + 0.55) / 0.75, 2.0));
    res = mix(res, vec4(0.04, 0.10, 0.015, 1.0), ring * 0.66);
    vec2 dn = d / (len + 1e-4);
    float arc = exp(-pow((dpx + 1.5) / 1.0, 2.0)) * pow(max(dot(dn, FOCUS), 0.0), 2.0);
    res.rgb += vec3(0.78, 1.0, 0.42) * arc * 0.40;
    vec2 hl = d - vec2(-0.36, -0.40) * r;
    float hr = max(0.24 * r, 0.7);
    float spot = exp(-dot(hl, hl) / (hr * hr));
    res.rgb += vec3(1.0, 1.0, 0.94) * spot * 0.9;
    res.a = min(1.0, res.a + spot * 0.5);
    return mix(col, res, cover * clamp(b0.w / 1.6, 0.25, 1.0));
}

// Dome of thin film over the gel (grow, hold, burst). The colour seen through the film is the gel's own
// shading times the film transmittance (long path at the silhouette: a darker green ring), on top come the
// curved soft-box reflection, the sharp highlight and the lime caustic; in hold a faint interference tint.
vec4 domeLayer(vec4 col, vec2 p, vec4 b0, vec4 b1, vec4 b2, vec4 b3, int stage) {
    vec2 d = p - b0.xy;
    float a = b0.w;
    float rho = length(d);
    float cover = 1.0 - smoothstep(a - 1.0, a, rho);
    if (cover <= 0.0) return col;
    float H = min(b1.x, 1.0) * a;
    if (H < 0.4) return col;
    float Rs = (a * a + H * H) / (2.0 * H);
    float zs = sqrt(max(Rs * Rs - rho * rho, 0.0));
    vec3 n = vec3(d, zs) / Rs;
    float film = b1.w;
    float thick = mix(0.9 + 2.2 * film, 2.2 + 1.6 * film, smoothstep(0.15, 1.0, rho / a));
    float path = thick / max(n.z, 0.12);
    vec3 T = exp(-SIGMA * 2.4 * path);
    vec4 res = vec4(col.rgb * T, 1.0 - (1.0 - col.a) * dot(T, vec3(0.3333)));
    // The air under the film lifts the gel a little: no wetting dip, clearer than the gel around it.
    res.rgb = res.rgb * 1.12 + vec3(0.05, 0.06, 0.025) * (1.0 - rho / a);
    vec2 dn = d / (rho + 1e-4);
    float cau = pow(clamp(dot(dn, FOCUS), 0.0, 1.0), 3.0) * (1.0 - n.z);
    float crescent = smoothstep(0.50, 0.92, rho / a) * (1.0 - smoothstep(0.95, 1.03, rho / a)) * pow(max(dot(dn, FOCUS), 0.0), 2.0);
    res.rgb += vec3(0.80, 1.0, 0.40) * (cau * 0.40 + crescent * 0.50);
    float F = 0.04 + 0.96 * pow(1.0 - n.z, 5.0);
    vec3 Rr = reflect(vec3(0.0, 0.0, -1.0), n);
    float box = softBox(Rr.xy * 0.62, 0.0);
    res = mix(res, vec4(environment(Rr, 0.0), 1.0), clamp(F * 0.9, 0.0, 1.0));
    res.rgb += vec3(1.0, 1.0, 0.92) * box * (0.30 + 0.6 * F);
    float nh = max(dot(n, HALFV), 0.0);
    res.rgb += vec3(1.0, 0.99, 0.9) * (pow(nh, 150.0) * 1.7 + pow(nh, 22.0) * 0.14);
    // Like the wallpaper beads: a crisp thin white line near the lit silhouette with a narrow dark gap inside.
    float tiltD = length(n.xy);
    float litD = clamp(dot(dn, vec2(-0.5145, -0.8575)) * 1.4 - 0.1, 0.0, 1.0);
    res.rgb *= 1.0 - 0.28 * exp(-pow((tiltD - 0.66) / 0.07, 2.0)) * litD;
    res.rgb += vec3(1.0, 1.0, 0.94) * exp(-pow((tiltD - 0.80) / 0.05, 2.0)) * litD * 0.9;
    if (stage == 2 && b3.z > 0.01) {
        float ph = (1.0 - rho / a) * 3.4 + uFrame.x * 0.3;
        vec3 tint = 0.5 + 0.5 * cos(6.2831 * (vec3(0.0, 0.33, 0.67) + ph));
        res.rgb = mix(res.rgb, res.rgb * 0.8 + tint * 0.24, b3.z * smoothstep(0.55, 0.0, rho / a) * 0.55);
    }
    float edge = smoothstep(a - 1.7, a - 0.5, rho) * (1.0 - smoothstep(a - 0.5, a, rho));
    res = mix(res, vec4(0.04, 0.10, 0.012, 1.0), edge * 0.42);
    res.a = min(1.0, res.a);
    float keep = cover;
    vec3 rimRGB = vec3(0.0);
    float rimA = 0.0;
    if (stage == 3) {
        float hd = distance(p, b2.xy);
        float holeR = b2.z;
        float rim = max(b2.w, 0.6);
        keep *= smoothstep(holeR - 0.6, holeR + 0.6, hd);
        float td = (hd - holeR) / rim;
        if (holeR > 0.4 && abs(td) < 1.15) {
            vec2 rad = (p - b2.xy) / max(hd, 1e-3);
            float tube = sqrt(max(1.0 - td * td, 0.0));
            vec3 nt = normalize(vec3(rad * td, tube + 0.12));
            float lit = clamp(dot(nt, LIGHT) * 0.5 + 0.5, 0.0, 1.0);
            vec3 rc = mix(vec3(0.20, 0.38, 0.07), vec3(0.88, 1.0, 0.52), lit);
            float nhr = max(dot(nt, HALFV), 0.0);
            rc += vec3(1.0, 1.0, 0.92) * pow(nhr, 40.0) * 1.1;
            rimRGB = rc;
            rimA = (1.0 - smoothstep(0.85, 1.1, abs(td))) * cover;
        }
    }
    vec4 domed = mix(col, res, keep);
    return mix(domed, vec4(rimRGB, 1.0), rimA);
}
`;

    const GEL_FRAGMENT = PREAMBLE + COMMON + GEL_FIELD + GEL_DETAIL + `
out vec4 outColor;

// gs: the field at the pixel shifted against the light (5, 9) px, g0: the field at the pixel.
vec4 castShadow(Gel gs, Gel g0) {
    // A thread or a drip throws a short, light shadow in proportion to its width.
    float broad = smoothstep(1.0, 9.0, gs.rl);
    float shadow = 0.30 * (0.22 + 0.78 * broad) * (1.0 - smoothstep(0.0, mix(4.0, 14.0, broad), gs.d));
    float facing = max(dot(g0.nrm, FOCUS), 0.0);
    float fringe = exp(-max(g0.d, 0.0) / 2.4) * 0.18 * facing;
    vec3 shadowRGB = vec3(0.018, 0.06, 0.008) * shadow;
    vec3 fringeRGB = vec3(0.78, 0.98, 0.40) * fringe;
    return vec4(shadowRGB * (1.0 - fringe) + fringeRGB, shadow * (1.0 - fringe) + fringe);
}

void main() {
    vec2 p = cssPos();
    if (uPass > 0.5 && p.x > uBand.x && p.x < uBand.z && p.y > uBand.y && p.y < uBand.w) discard;
    // One loop body evaluates the field at the pixel (0), at the point shifted against the light for the
    // cast shadow (1) and at two neighbours for the normal (2, 3): a single copy of the field code keeps
    // the shader small, which is most of its compile time.
    float e = 0.75;
    Gel g0;
    Gel gs;
    vec4 hs = vec4(0.0);
    bool under = false;
    bool outside = false;
    bool edge = false;
    int stop = 1;
    for (int k = 0; k < 4; k++) {
        if (k >= stop) break;
        if (k == 1 && !(outside || edge)) continue;
        vec2 pk = p;
        if (k == 1) pk -= vec2(5.0, 9.0);
        else if (k == 2) pk.x += e;
        else if (k == 3) pk.y += e;
        Gel g = gelAt(pk);
        if (k == 0) {
            g0 = g;
            if (g.dw < -0.5 || g.d > 28.0) discard;
            if (blockedAt(p)) discard;
            under = underAt(p);
            outside = g.d > 0.0;
            if (outside && under) discard;
            edge = !outside && -g.d < clamp(g.rl * 0.7, 0.55, 1.25) && !under;
            stop = outside ? 2 : 4;
            if (!outside) hs.x = gelHeight(p, g);
        } else if (k == 1) {
            gs = g;
        } else {
            hs[k] = gelHeight(pk, g);
        }
    }
    float D = g0.d;
    if (outside) {
        vec4 sh = castShadow(gs, g0);
        if (sh.a <= 0.002) discard;
        outColor = vec4(min(sh.rgb, vec3(sh.a)), sh.a) * uFrame.w;
        return;
    }
    float h0 = hs.x;
    vec3 n = normalize(vec3(-(hs.z - h0) / e, -(hs.w - h0) / e, 1.0));
    // A thread is only a few pixels wide, too fine for finite differences: its normal comes straight
    // from the round section (across-vector and position across it).
    float wT = smoothstep(0.6, 0.97, g0.thr.x) * (1.0 - smoothstep(3.2, 5.2, g0.rl));
    vec3 nRound = normalize(vec3(g0.thr.yz * g0.thr.w, sqrt(max(1.0 - g0.thr.w * g0.thr.w, 0.04))));
    n = normalize(mix(n, nRound, wT));
    bool tex = uFrame.z > 0.5 && !under;
    float aaW = clamp(g0.rl * 0.7, 0.55, 1.25);
    float texW = tex ? smoothstep(0.0, clamp(g0.rl * 0.8, 0.5, 2.5), -D) : 0.0;
    float aMin = 0.9 * (1.0 - smoothstep(8.5, 13.0, g0.dw));
    // A thread a few pixels wide has too little path to tint: count it as several times thicker so it
    // reads as a lime body, not as clear glass.
    float hShade = h0 * (1.0 + 4.5 * wT);
    vec2 look = p - n.xy * (0.55 * hShade + 2.0);
    vec4 col = shadeSlime(look, hShade, n, texW, aMin, 0.6 + 0.3 * wT, 1.0 + 0.7 * wT, wT);
    vec4 ov = suspended(g0);
    col = vec4(col.rgb * (1.0 - ov.a) + ov.rgb, min(1.0, col.a + ov.a * (1.0 - col.a)));
    int nb = int(uCounts.y + 0.5);
    for (int i = 0; i < nb; i++) {
        vec4 b0 = uBub[4 * i];
        if (distance(p, b0.xy) > b0.w * 1.5 + 4.0) continue;
        vec4 b1 = uBub[4 * i + 1];
        int st = int(b1.y + 0.5);
        if (st == 0) col = pocketLayer(col, p, b0);
        else if (st <= 3) col = domeLayer(col, p, b0, b1, uBub[4 * i + 2], uBub[4 * i + 3], st);
    }
    float contact = 1.0 - smoothstep(0.0, 1.8, g0.dw);
    col = mix(col, vec4(0.03, 0.07, 0.01, 1.0), contact * 0.55);
    float glintX = (g0.dw - 2.7) / 0.9;
    float face = clamp(dot(g0.nrm, LIGHT.xy / 0.8167) * 0.5 + 0.5, 0.0, 1.0);
    col.rgb += vec3(0.92, 1.0, 0.72) * exp(-glintX * glintX) * (0.12 + 0.4 * face);
    col.a = min(1.0, col.a + 0.2 * exp(-glintX * glintX));
    float edgeA = smoothstep(0.0, aaW, -D) * smoothstep(-0.5, 0.5, g0.dw);
    col *= edgeA;
    if (edge) {
        vec4 sh = castShadow(gs, g0);
        col = vec4(col.rgb + sh.rgb * (1.0 - col.a), col.a + sh.a * (1.0 - col.a));
    }
    // uFrame.w is the fade-in after the build: the gel oozes into view instead of popping.
    outColor = vec4(min(col.rgb, vec3(col.a)), col.a) * uFrame.w;
}
`;

    // ---------------------------------------------------------------------------------------------
    // Drop program: one instanced quad per drop, spray droplet or taskbar splat.

    const DROP_VERTEX = PREAMBLE + `
uniform vec2 uSize;
in vec4 aA;
in vec4 aB;
flat out vec4 vA;
flat out vec4 vB;
out vec2 vPos;
void main() {
    vec2 corner = vec2(float(gl_VertexID & 1), float((gl_VertexID >> 1) & 1)) * 2.0 - 1.0;
    // A resting blob reads larger than the falling drop it came from (the gel pools out).
    float r = aA.z * (aA.w > 1.5 ? 1.4 : 1.0);
    vec2 centre = aA.xy;
    vec2 hs;
    if (aA.w > 1.5) {
        float w = r * (0.95 + 0.55 * max(aB.z, 0.0)) + 14.0;
        float hh = r * 0.8 + 8.0;
        hs = vec2(w, hh * 0.5 + 2.0);
        centre.y -= hh * 0.5 - 2.0;
    } else {
        float stretch = 1.0 + min(0.6, abs(aB.y) / 420.0);
        hs = vec2(r * (1.0 - aB.z * 0.55), r * stretch * (1.0 + aB.z)) + vec2(10.0);
        hs.x += 4.0;
        centre.y += 3.0;
    }
    vPos = centre + corner * hs;
    vA = aA;
    vB = aB;
    gl_Position = vec4(vPos.x / uSize.x * 2.0 - 1.0, 1.0 - vPos.y / uSize.y * 2.0, 0.0, 1.0);
}
`;

    const DROP_FRAGMENT = PREAMBLE + COMMON + `
flat in vec4 vA;
flat in vec4 vB;
in vec2 vPos;
out vec4 outColor;

// Egg-shaped drop: semi-axes (ax, ay) with a tail on top while it falls fast. Returns the signed
// distance (px) and writes the normalised position inside the shape.
float dropShape(vec2 l, float ax, float ay, float tail, out vec2 nl) {
    float t = smoothstep(-1.0, 0.7, l.y / ay);
    float axl = ax * mix(1.0, 0.30 + 0.70 * t, tail);
    nl = vec2(l.x / axl, l.y / ay);
    return (length(nl) - 1.0) * min(axl, ay);
}

void main() {
    float r = vA.z * (vA.w > 1.5 ? 1.4 : 1.0);
    float kind = vA.w;
    float alpha = vB.w;
    if (kind > 1.5) {
        // Splat: a sessile blob resting on the taskbar edge (the baseline is y = vA.y).
        // Right after the impact it is a rounded dome about 0.7 r high; as it spreads it relaxes flatter.
        float spread = vB.z;
        float w = r * (0.95 + 0.55 * max(spread, 0.0));
        float hh = r * clamp(0.74 - 0.20 * spread, 0.5, 0.74);
        vec2 l = vPos - vA.xy;
        float skirt = 2.0 * (1.0 - smoothstep(0.0, 0.3 * hh, -l.y));
        vec2 nl = vec2(l.x / (w + skirt), l.y / hh);
        float rho = length(nl);
        float sd = (rho - 1.0) * min(w, hh);
        if (l.y > 1.0) discard;
        float below = clamp(1.0 - l.y, 0.0, 1.0);
        float cover = clamp(0.5 - sd, 0.0, 1.0) * below;
        // Contact shadow hugging the foot on the taskbar edge, plus the shadow thrown to the lower right.
        float contactD = length(vec2(l.x / (w + skirt + 3.0), (l.y + 1.1) / 2.4)) - 1.0;
        float shadowD = length(vec2((l.x - 4.0) / (w + skirt + 2.0), (l.y - 3.0) / (hh + 2.0))) - 1.0;
        float shadow = (0.36 * (1.0 - smoothstep(0.0, 0.7, contactD)) + 0.18 * (1.0 - smoothstep(0.0, 0.5, shadowD))) * below;
        float z = sqrt(max(1.0 - rho * rho, 0.0));
        vec3 n = normalize(vec3(nl.x * hh / (w + skirt) * 0.95, nl.y * 0.95, z + 0.06));
        float hgt = hh * 1.5 * z;
        vec4 col = shadeSlime(vPos - n.xy * (0.55 * hgt + 2.0), hgt, n, 1.0, 0.0, 0.9, 1.7, 0.0) * cover;
        vec4 sh = vec4(vec3(0.018, 0.06, 0.008) * shadow, shadow);
        col = col + sh * (1.0 - col.a);
        col *= alpha * uFrame.w;
        if (col.a <= 0.002) discard;
        outColor = vec4(min(col.rgb, vec3(col.a)), col.a);
        return;
    }
    float wobble = kind > 0.5 ? 0.0 : vB.z;
    float stretch = 1.0 + min(0.6, abs(vB.y) / 420.0);
    float ax = max(r * (1.0 - wobble * 0.55), 0.6);
    float ay = max(r * stretch * (1.0 + wobble), 0.6);
    float tail = kind > 0.5 ? 0.0 : clamp(abs(vB.y) / 600.0, 0.0, 0.5);
    vec2 l = vPos - vA.xy;
    vec2 nl;
    float sd = dropShape(l, ax, ay, tail, nl);
    vec2 nls;
    float sds = dropShape(l - vec2(3.0, 5.0), ax, ay, tail, nls);
    float cover = clamp(0.5 - sd, 0.0, 1.0);
    float shadow = 0.22 * (1.0 - smoothstep(0.0, 4.5, sds)) * min(1.0, r / 2.0);
    float rho = length(nl);
    float z = sqrt(max(1.0 - rho * rho, 0.0));
    float hm = min(ax, ay) * 0.95;
    vec3 n = normalize(vec3(nl.x * hm / ax, nl.y * hm / ay, z + 0.02));
    // A drop is a lens: the backdrop shows up inverted, the path is the whole chord.
    vec4 col = shadeSlime(vA.xy - l * 0.8, 1.3 * hm * z, n, 1.0, 0.0, 1.0, 1.9, 0.0) * cover;
    vec4 sh = vec4(vec3(0.018, 0.06, 0.008) * shadow, shadow);
    col = col + sh * (1.0 - col.a);
    col *= alpha * uFrame.w;
    if (col.a <= 0.002) discard;
    outColor = vec4(min(col.rgb, vec3(col.a)), col.a);
}
`;

    // ---------------------------------------------------------------------------------------------
    // Mask program: rounded rectangles into the stencil buffer (no colour).

    const MASK_VERTEX = PREAMBLE + `
uniform vec2 uSize;
in vec4 aRect;
in float aRad;
flat out vec4 vRect;
flat out float vRad;
out vec2 vPos;
void main() {
    vec2 corner = vec2(float(gl_VertexID & 1), float((gl_VertexID >> 1) & 1));
    vPos = mix(aRect.xy - 1.0, aRect.zw + 1.0, corner);
    vRect = aRect;
    vRad = aRad;
    gl_Position = vec4(vPos.x / uSize.x * 2.0 - 1.0, 1.0 - vPos.y / uSize.y * 2.0, 0.0, 1.0);
}
`;

    const MASK_FRAGMENT = PREAMBLE + `
flat in vec4 vRect;
flat in float vRad;
in vec2 vPos;
out vec4 outColor;
void main() {
    vec2 c = (vRect.xy + vRect.zw) * 0.5;
    vec2 hs = (vRect.zw - vRect.xy) * 0.5;
    vec2 q = abs(vPos - c) - (hs - vRad);
    float sd = min(max(q.x, q.y), 0.0) + length(max(q, 0.0)) - vRad;
    if (sd >= 0.0) discard;
    outColor = vec4(0.0);
}
`;

    window.AuraGreenSlimeGLShaders = Object.freeze({
        gelVertex: GEL_VERTEX, gelFragment: GEL_FRAGMENT,
        dropVertex: DROP_VERTEX, dropFragment: DROP_FRAGMENT,
        maskVertex: MASK_VERTEX, maskFragment: MASK_FRAGMENT
    });
})();
