(function () {
    'use strict';

    // GLSL ES 3.00 sources for the Video Studio GL compositor (index §4.7). Fragment shaders output premultiplied colour,
    // except the clip-local passes (PICTURE, the 2c effects and 2d's KEY), which read and write straight alpha.
    // Targets hold images top-down (row 0 is the top row, like uploaded textures); QUAD_VERTEX flips v only when it
    // presents to the canvas (u_flipY = 1). Transition uniforms come from VideoStudioPicture.transitionUniforms (plan 2e
    // D1). u_P is xfade's float32 progress P = f(1 − f(i/D)) of overlap frame i, and u_progress is Q = f(1 − P), the
    // incoming weight of the dissolve.
    const QUAD_VERTEX = `#version 300 es
layout(location = 0) in vec2 a_pos;
uniform float u_flipY;
out vec2 v_uv;
void main() {
    v_uv = vec2(a_pos.x, mix(a_pos.y, 1.0 - a_pos.y, u_flipY));
    gl_Position = vec4(a_pos * 2.0 - 1.0, 0.0, 1.0);
}
`;

    // A placed quad: u_matrix maps the unit quad (0,0 = top left of the drawn rectangle) into clip space of a top-down
    // target; u_uvRect is the part of the source the quad shows (x, y, width, height in texture coordinates).
    // u_boxRect (2d) is the drawn rectangle in fractions of the clip box (VideoStudioPicture.boxRect: the fitted picture
    // for contain, [0, 0, 1, 1] for cover), so v_box is the fragment's box-local position. PLACE reads it for the wipe
    // reveal (2e) and PLACE_MASKED for the mask and the reveal; placeClip sets u_boxRect for every clip.
    const PLACE_VERTEX = `#version 300 es
layout(location = 0) in vec2 a_pos;
uniform mat3 u_matrix;
uniform vec4 u_uvRect;
uniform vec4 u_boxRect;
out vec2 v_uv;
out vec2 v_box;
void main() {
    v_uv = u_uvRect.xy + a_pos * u_uvRect.zw;
    v_box = u_boxRect.xy + a_pos * u_boxRect.zw;
    vec3 position = u_matrix * vec3(a_pos, 1.0);
    gl_Position = vec4(position.xy, 0.0, 1.0);
}
`;

    const FRAGMENT_HEAD = `#version 300 es
precision highp float;
in vec2 v_uv;
out vec4 outColor;
`;

    // ---- 2e wipe reveal (index §4.7). ----
    // u_reveal is the visible part of the clip box as (x0, y0, x1, y1) box fractions: VideoStudioEffects.placement's
    // reveal, which is VideoStudioAnim.revealPx in whole box columns, or [0, 0, 1, 1] without a wipe.
    // - Alpha is 0 outside it. The reveal is in box space, so it turns with the box and ignores crop, flips and effects.
    // - placeClip always sets u_reveal and u_boxRect: an unset vec4 is 0 and would hide the clip (risk 31).
    const REVEAL = `in vec2 v_box;
uniform vec4 u_reveal;
float vsReveal(vec2 b) { return b.x >= u_reveal.x && b.x < u_reveal.z && b.y >= u_reveal.y && b.y < u_reveal.w ? 1.0 : 0.0; }
`;

    // The source is straight alpha (uploads never premultiply); opacity scales alpha and the output is premultiplied.
    // 2e: alpha is also 0 outside the wipe reveal.
    const PLACE = FRAGMENT_HEAD + REVEAL + `uniform sampler2D u_source;
uniform float u_opacity;
void main() {
    vec4 colour = texture(u_source, v_uv);
    float alpha = colour.a * u_opacity * vsReveal(v_box);
    outColor = vec4(colour.rgb * alpha, alpha);
}
`;

    // ---- 2d mask (index §4.22, law L). ----
    // vsMask(p) is the mask at p, a point in box px (x right, y down). The uniforms come from
    // VideoStudioPicture.maskUniforms:
    // - u_maskCentre and u_maskRadii are c and (rx, ry);
    // - u_maskRot is (cos θ, sin θ), with θ positive = clockwise on screen;
    // - u_maskRamp is F = max(feather·H, 1 px);
    // - u_maskShape is 0 (rect) or 1 (ellipse), and u_maskInvert is 1 for an inverted mask.
    // s is the signed distance in box px (> 0 outside), exact for the rectangle and first order for the ellipse.
    // m = clamp(0.5 − s/F, 0, 1) is a linear edge of width F centred on the outline. At the single point p = c the
    // ellipse's gradient length is 0, so it gets a floor of 1e-12: s → −∞ there, which means fully inside.
    const MASK = `uniform vec2 u_maskCentre;
uniform vec2 u_maskRadii;
uniform vec2 u_maskRot;
uniform float u_maskRamp;
uniform float u_maskShape;
uniform float u_maskInvert;
float vsMask(vec2 p) {
    vec2 d = p - u_maskCentre;
    vec2 l = vec2(d.x * u_maskRot.x + d.y * u_maskRot.y, d.y * u_maskRot.x - d.x * u_maskRot.y);
    float s;
    if (u_maskShape > 0.5) {
        vec2 n = l / u_maskRadii;
        float r = max(length(n), 0.000001);
        s = (r - 1.0) * r / max(length(n / u_maskRadii), 1e-12);
    } else {
        vec2 q = abs(l) - u_maskRadii;
        s = length(max(q, vec2(0.0))) + min(max(q.x, q.y), 0.0);
    }
    float m = clamp(0.5 - s / u_maskRamp, 0.0, 1.0);
    return u_maskInvert > 0.5 ? 1.0 - m : m;
}
`;

    // PLACE with a mask (2d, index §4.7): alpha x vsMask at the fragment's box px (v_box x u_boxSize), applied before
    // the premultiplication. The mask therefore follows the box, its rotation and the fit; crop, flips and effects
    // never move it. 2e: the wipe reveal multiplies after the mask, and with animation the box is the animated one.
    const PLACE_MASKED = FRAGMENT_HEAD + REVEAL + `uniform sampler2D u_source;
uniform float u_opacity;
uniform vec2 u_boxSize;
` + MASK + `void main() {
    vec4 colour = texture(u_source, v_uv);
    float alpha = colour.a * u_opacity * vsMask(v_box * u_boxSize) * vsReveal(v_box);
    outColor = vec4(colour.rgb * alpha, alpha);
}
`;

    // Premultiplied copy: the composite onto the canvas, and hard cuts.
    const COPY = FRAGMENT_HEAD + `uniform sampler2D u_source;
void main() {
    outColor = texture(u_source, v_uv);
}
`;

    const TRANSITION_HEAD = FRAGMENT_HEAD + `uniform sampler2D u_from;
uniform sampler2D u_to;
uniform float u_progress;
uniform vec2 u_size;
`;

    // xfade "fade" on premultiplied track layers.
    const DISSOLVE = TRANSITION_HEAD + `void main() {
    outColor = mix(texture(u_from, v_uv), texture(u_to, v_uv), u_progress);
}
`;

    // xfade "fadeblack": u_weights is (P·u, Q·(1 − v), rest) from transitionUniforms (index §4.24), equal to
    // VideoStudioEffects.fadeBlackWeights(i/D) within float32 rounding. It is mixed with opaque black, so the tracks
    // below darken too.
    const BLACK = TRANSITION_HEAD + `uniform vec3 u_weights;
void main() {
    outColor = texture(u_from, v_uv) * u_weights.x + texture(u_to, v_uv) * u_weights.y + vec4(0.0, 0.0, 0.0, u_weights.z);
}
`;

    // Wipes: u_edge is VideoStudioEffects.wipeEdge(type, index, duration, width) as (left, reveal) in target pixels.
    // The incoming layer shows inside [left, left + reveal), which follows the integer edge z of index §4.24:
    // - "wipeleft" covers [0, z + 1), so column 0 already shows at frame 0 (D3);
    // - "wiperight" covers [z + 1, W).
    const WIPE = TRANSITION_HEAD + `uniform vec2 u_edge;
void main() {
    float x = v_uv.x * u_size.x;
    vec4 outgoing = texture(u_from, v_uv);
    vec4 incoming = texture(u_to, v_uv);
    outColor = x >= u_edge.x && x < u_edge.x + u_edge.y ? incoming : outgoing;
}
`;

    // ---- 2e transitions (index §4.7, §4.24). ----
    // Each source is TRANSITION_HEAD, the TX helpers and a body written from §4.24's normative per-pixel rules: the
    // spike's float32 models in their IDEAL form, the same arithmetic in highp float without FFmpeg's truncating 8-bit
    // store (at most 1 level off, mean −0.45). No FFmpeg source is copied.
    // - Every per-frame number is a uniform from VideoStudioPicture.transitionUniforms (D1). u_P is xfade's progress P:
    //   1 at overlap frame 0, falling to 1/D. No 2e source reads u_progress.
    // - p is the fragment's pixel, floor(v_uv · u_size): column, and row counted from the top (targets are top-down).
    // - vsA and vsB read the premultiplied outgoing and incoming layers at an integer position (texelFetch, clamped into
    //   the target). The pushes, pixelize, zoomin and squeezeh sample by index; a and b are the fragment's own texels.
    // - vsMix(x, y, m) = x·m + y·(1 − m) is xfade's mix: m weighs the FIRST argument (GLSL mix weighs the second).
    // - The fills are opaque, so the tracks below vanish where they show (risk 28): premultiplied black (0, 0, 0, 1)
    //   for circlecrop and rectcrop, white (1, 1, 1, 1) for fadewhite.
    const TX_HEAD = TRANSITION_HEAD + `precision highp int;
uniform float u_P;
ivec2 vsSize() { return ivec2(u_size); }
ivec2 vsPixel() { return ivec2(floor(v_uv * u_size)); }
vec4 vsA(ivec2 q) { return texelFetch(u_from, clamp(q, ivec2(0), vsSize() - 1), 0); }
vec4 vsB(ivec2 q) { return texelFetch(u_to, clamp(q, ivec2(0), vsSize() - 1), 0); }
vec4 vsMix(vec4 x, vec4 y, float m) { return x * m + y * (1.0 - m); }
`;
    const TX_BLACK = 'vec4(0.0, 0.0, 0.0, 1.0)';
    // A 2e source from its own declarations and the body of main(), which starts with p, a and b.
    const tx = (declarations, body) => TX_HEAD + declarations + `void main() {
    ivec2 p = vsPixel();
    vec4 a = vsA(p), b = vsB(p);
` + body + `}
`;
    const SOFT = 'uniform float u_c;\n';
    const CIRCLE = 'uniform vec2 u_centre;\nuniform float u_R;\nuniform float u_c;\n';
    const TRANSITIONS_2E = {
        // Through white: mix(mix(A, K, u), mix(K, B, v), P), K = white, u_uv = (u, v) = (ss(0.8, 1, P), ss(0.2, 1, P)).
        fadewhite: tx('uniform vec2 u_uv;\n', `    vec4 k = vec4(1.0);
    outColor = vsMix(vsMix(a, k, u_uv.x), vsMix(k, b, u_uv.y), u_P);
`),
        // Through grey: K is trunc((R8 + G8 + B8) / 3) of the premultiplied texel with its own alpha. The first inner mix
        // uses A's grey, the second B's.
        fadegrays: tx(`uniform vec2 u_uv;
vec4 vsGrey(vec4 c) {
    ivec3 v = ivec3(round(c.rgb * 255.0));
    float g = float((v.r + v.g + v.b) / 3) / 255.0;
    return vec4(g, g, g, c.a);
}
`, '    outColor = vsMix(vsMix(a, vsGrey(a), u_uv.x), vsMix(vsGrey(b), b, u_uv.y), u_P);\n'),
        // Square blocks of u_sq px from the top left; both layers are sampled at the block centre. u_sq = 0 cross-fades.
        pixelize: tx('uniform float u_sq;\n', `    ivec2 s = u_sq > 0.0 ? min(ivec2((floor(vec2(p) / u_sq) + 0.5) * u_sq), vsSize() - 1) : p;
    outColor = vsMix(vsA(s), vsB(s), u_P);
`),
        // A hard row edge u_edge. wipeup: B enters from the bottom. wipedown: from the top (row 0 shows B at i = 0).
        wipeup: tx('uniform int u_edge;\n', '    outColor = p.y > u_edge ? b : a;\n'),
        wipedown: tx('uniform int u_edge;\n', '    outColor = p.y > u_edge ? a : b;\n'),
        // Soft edges: t = smoothstep(0, 1, 1 + e − u_c) with u_c = 2P, and out = mix(B, A, t). e by type:
        // - smoothleft: x/W (B enters from the right);
        // - smoothright: (W − 1 − x)/W (from the left);
        // - diagtl: (x/W)·y/H (from the bottom right);
        // - diagbr: the mirrored product (from the top left).
        smoothleft: tx(SOFT, '    outColor = vsMix(b, a, smoothstep(0.0, 1.0, 1.0 + float(p.x) / u_size.x - u_c));\n'),
        smoothright: tx(SOFT, '    outColor = vsMix(b, a, smoothstep(0.0, 1.0, 1.0 + float(vsSize().x - 1 - p.x) / u_size.x - u_c));\n'),
        diagtl: tx(SOFT, '    outColor = vsMix(b, a, smoothstep(0.0, 1.0, 1.0 + float(p.x) / u_size.x * float(p.y) / u_size.y - u_c));\n'),
        diagbr: tx(SOFT, `    ivec2 q = vsSize() - 1 - p;
    outColor = vsMix(b, a, smoothstep(0.0, 1.0, 1.0 + float(q.x) / u_size.x * float(q.y) / u_size.y - u_c));
`),
        // Clockwise sweep from 12 o'clock: the angle is atan2(dx, dy), 0 pointing down, minus u_c = k. Above the centre
        // (dx = 0, dy < 0) it is π, as C's atan2(+0, dy) gives; at and below the centre it is 0.
        radial: tx('uniform vec2 u_centre;\nuniform float u_c;\n', `    ivec2 d = p - ivec2(u_centre);
    float angle = d.x == 0 ? (d.y < 0 ? 3.14159265358979 : 0.0) : atan(float(d.x), float(d.y));
    outColor = vsMix(b, a, smoothstep(0.0, 1.0, angle - u_c));
`),
        // Pushes: both layers move by u_shift = trunc(P·size) px.
        // - Left and up: B[z] where z = x − s ≥ 0, else A[z + size].
        // - Right and down: B[z] where 0 < z = x + s < size, else A[z mod size].
        slideleft: tx('uniform int u_shift;\n', `    int z = p.x - u_shift;
    outColor = z >= 0 ? vsB(ivec2(z, p.y)) : vsA(ivec2(z + vsSize().x, p.y));
`),
        slideright: tx('uniform int u_shift;\n', `    int n = vsSize().x, z = p.x + u_shift;
    outColor = z > 0 && z < n ? vsB(ivec2(z, p.y)) : vsA(ivec2(z >= n ? z - n : z, p.y));
`),
        slideup: tx('uniform int u_shift;\n', `    int z = p.y - u_shift;
    outColor = z >= 0 ? vsB(ivec2(p.x, z)) : vsA(ivec2(p.x, z + vsSize().y));
`),
        slidedown: tx('uniform int u_shift;\n', `    int n = vsSize().y, z = p.y + u_shift;
    outColor = z > 0 && z < n ? vsB(ivec2(p.x, z)) : vsA(ivec2(p.x, z >= n ? z - n : z));
`),
        // A squeezed vertically about the middle to height P·H, with B static behind it. Hard edges, no blending; the
        // row index rounds half to even.
        squeezeh: tx('', `    float z = 0.5 + (float(p.y) / u_size.y - 0.5) / u_P;
    outColor = z < 0.0 || z > 1.0 ? b : vsA(ivec2(p.x, int(roundEven(z * (u_size.y - 1.0)))));
`),
        // A magnified about the centre by u_zoom.x = zf (nearest sample), cross-faded to B with weight u_zoom.y = w.
        zoomin: tx(`uniform vec2 u_zoom;
int vsZoom(int i, float n) { return clamp(int(ceil(((float(i) / n - 0.5) * u_zoom.x + 0.5) * (n - 1.0))), 0, int(n) - 1); }
`, '    outColor = vsMix(vsA(ivec2(vsZoom(p.x, u_size.x), vsZoom(p.y, u_size.y))), b, u_zoom.y);\n'),
        // Circles: d = |p − u_centre|, t = smoothstep(0, 1, d/u_R + u_c).
        // - circleopen: mix(A, B, t); B opens from the centre.
        // - circleclose: mix(B, A, t); B closes in from the corners.
        circleopen: tx(CIRCLE, '    outColor = vsMix(a, b, smoothstep(0.0, 1.0, length(vec2(p - ivec2(u_centre))) / u_R + u_c));\n'),
        circleclose: tx(CIRCLE, '    outColor = vsMix(b, a, smoothstep(0.0, 1.0, length(vec2(p - ivec2(u_centre))) / u_R + u_c));\n'),
        // The disc of radius u_r shows A while P >= 0.5, then B; outside it is opaque black.
        circlecrop: tx('uniform vec2 u_centre;\nuniform float u_r;\n', '    outColor = u_r < length(vec2(p - ivec2(u_centre))) ? ' + TX_BLACK + ' : (u_P < 0.5 ? b : a);\n'),
        // The rectangle |x − cx| < zw, |y − cy| < zh shows A, then B; outside it is opaque black. The centre is
        // (W >> 1, H >> 1) of u_size; u_rect = (zw, zh). At frame 0 this leaves a 1 px black border at the left and top.
        rectcrop: tx('uniform vec2 u_rect;\n', `    ivec2 d = abs(p - vsSize() / 2), z = ivec2(u_rect);
    outColor = d.x < z.x && d.y < z.y ? (u_P < 0.5 ? b : a) : ` + TX_BLACK + `;
`)
    };
    const TRANSITIONS = Object.assign({ dissolve: DISSOLVE, black: BLACK, wipeleft: WIPE, wiperight: WIPE }, TRANSITIONS_2E);

    // ---- 2c clip-local passes (index §4.7 pass contract, §4.20 formulas). ----
    // PICTURE crops and flips. v_uv covers the clip-local target, u_flip (1 mirrors that axis) flips it, and u_uvRect is
    // the crop rect in texture coordinates of the uncropped texture. It copies straight alpha.
    const PICTURE = FRAGMENT_HEAD + `uniform sampler2D u_source;
uniform vec4 u_uvRect;
uniform vec2 u_flip;
void main() {
    vec2 local = mix(v_uv, vec2(1.0) - v_uv, u_flip);
    outColor = texture(u_source, u_uvRect.xy + local * u_uvRect.zw);
}
`;

    // Effect passes draw QUAD_VERTEX with u_flipY 0 into a clip-local RGBA8 target without blending. They read u_source
    // (straight alpha, top-down) and u_size (target px) and write straight alpha; PLACE premultiplies later. The uniform
    // values come from VideoStudioPicture.passUniforms.
    const PASS_HEAD = FRAGMENT_HEAD + `uniform sampler2D u_source;
uniform vec2 u_size;
`;

    // color, §4.20 steps 1-6, per channel:
    // - u_mul (2^EV x white balance), contrast about 0.5 and clamp, lift, gain, clamp, then 1/gamma
    //   (VideoStudioPicture.colorCoefficients);
    // - then saturation about BT.709 luma (VideoStudioPicture.LUMA).
    // Alpha is unchanged.
    const COLOR = PASS_HEAD + `uniform vec3 u_mul;
uniform float u_contrast;
uniform vec3 u_lift;
uniform vec3 u_gain;
uniform vec3 u_invGamma;
uniform float u_saturation;
void main() {
    vec4 colour = texture(u_source, v_uv);
    vec3 x = colour.rgb * u_mul;
    x = clamp((x - 0.5) * u_contrast + 0.5, 0.0, 1.0);
    x = x + u_lift * (1.0 - x);
    x = x * u_gain;
    x = pow(clamp(x, 0.0, 1.0), u_invGamma);
    float luma = dot(x, vec3(0.2126, 0.7152, 0.0722));
    outColor = vec4(clamp(vec3(luma) + u_saturation * (x - vec3(luma)), 0.0, 1.0), colour.a);
}
`;

    // lut: out = (1 - k) in + k L(in clamped to the domain). L is trilinear in the RGBA16F 3D texture (hardware LINEAR)
    // at texel centres, coord (N - 1) / N + 0.5 / N.
    const LUT = PASS_HEAD + `precision highp sampler3D;
uniform sampler3D u_lut;
uniform float u_lutSize;
uniform vec3 u_domainMin;
uniform vec3 u_domainMax;
uniform float u_intensity;
void main() {
    vec4 colour = texture(u_source, v_uv);
    vec3 coord = clamp((colour.rgb - u_domainMin) / (u_domainMax - u_domainMin), 0.0, 1.0);
    vec3 mapped = texture(u_lut, coord * ((u_lutSize - 1.0) / u_lutSize) + 0.5 / u_lutSize).rgb;
    outColor = vec4(clamp(mix(colour.rgb, mapped, u_intensity), 0.0, 1.0), colour.a);
}
`;

    // blur: one direction of a separable Gaussian per pass (u_direction is the texel step: horizontal, then vertical).
    // - Weights are exp(-i^2 / 2 sigma^2) over +-u_radius texels (min(128, ceil(3 sigma))), normalized.
    // - Neighbours are read in pairs through one bilinear tap at the pair's weighted offset.
    // - Colour is convolved premultiplied and stored straight. The pair tap filters straight colour first, which is
    //   exact for opaque pictures.
    const BLUR = PASS_HEAD + `uniform vec2 u_direction;
uniform float u_sigma;
uniform int u_radius;
vec4 premultiplied(vec2 uv) {
    vec4 colour = texture(u_source, uv);
    return vec4(colour.rgb * colour.a, colour.a);
}
void main() {
    float k = -0.5 / (u_sigma * u_sigma);
    vec4 sum = premultiplied(v_uv);
    float total = 1.0;
    for (int i = 1; i <= 128; i += 2) {
        if (i > u_radius) break;
        float w0 = exp(k * float(i * i));
        float w1 = i + 1 <= u_radius ? exp(k * float((i + 1) * (i + 1))) : 0.0;
        float w = w0 + w1;
        vec2 offset = u_direction * ((float(i) * w0 + float(i + 1) * w1) / w);
        sum += (premultiplied(v_uv + offset) + premultiplied(v_uv - offset)) * w;
        total += 2.0 * w;
    }
    vec4 blurred = sum / total;
    outColor = blurred.a > 0.0 ? vec4(blurred.rgb / blurred.a, blurred.a) : vec4(0.0);
}
`;

    // sharpen: contrast-adaptive sharpening, written from AMD's published FidelityFX CAS (no scaling). Per RGB channel on
    // the 3x3 neighbourhood a b c / d e f / g h i:
    // - mn and mx are the soft min and max of the cross plus the corners;
    // - amp = sqrt(clamp(min(mn, 2 - mx) / mx, 0, 1));
    // - w = amp x peak, with peak = -1 / mix(P0, P1, strength) (u_peak = VideoStudioPicture CAS_P0, CAS_P1);
    // - out = (w (b + d + f + h) + e) / (1 + 4w).
    // Alpha is unchanged, and the border texel repeats.
    const SHARPEN = PASS_HEAD + `uniform float u_strength;
uniform vec2 u_peak;
vec4 texel(ivec2 p) {
    return texelFetch(u_source, clamp(p, ivec2(0), ivec2(u_size) - 1), 0);
}
void main() {
    ivec2 p = ivec2(gl_FragCoord.xy);
    vec4 centre = texel(p);
    vec3 a = texel(p + ivec2(-1, -1)).rgb, b = texel(p + ivec2(0, -1)).rgb, c = texel(p + ivec2(1, -1)).rgb;
    vec3 d = texel(p + ivec2(-1, 0)).rgb, e = centre.rgb, f = texel(p + ivec2(1, 0)).rgb;
    vec3 g = texel(p + ivec2(-1, 1)).rgb, h = texel(p + ivec2(0, 1)).rgb, i = texel(p + ivec2(1, 1)).rgb;
    vec3 mn = min(min(min(d, e), min(f, b)), h);
    mn += min(mn, min(min(a, c), min(g, i)));
    vec3 mx = max(max(max(d, e), max(f, b)), h);
    mx += max(mx, max(max(a, c), max(g, i)));
    vec3 amp = sqrt(clamp(min(mn, 2.0 - mx) / max(mx, vec3(1e-5)), 0.0, 1.0));
    vec3 w = amp * (-1.0 / mix(u_peak.x, u_peak.y, clamp(u_strength, 0.0, 1.0)));
    outColor = vec4(clamp((w * (b + d + f + h) + e) / (1.0 + 4.0 * w), 0.0, 1.0), centre.a);
}
`;

    // vignette: f(r) = cos^4(u_angle r / R), with r the distance from the centre and R = hypot(W/2, H/2) of the target.
    // The colour is rgb x f and alpha is unchanged: the export's vignette with dither=0, and on image clips its alpha
    // wrapper.
    const VIGNETTE = PASS_HEAD + `uniform float u_angle;
void main() {
    vec4 colour = texture(u_source, v_uv);
    float c = cos(u_angle * length((v_uv - 0.5) * u_size) / length(0.5 * u_size));
    float f = c * c * c * c;
    outColor = vec4(colour.rgb * f, colour.a);
}
`;

    // ---- 2d chroma key (index §4.22), the real forms of colorkey and despill on straight colour in 0..1. ----
    // - d = sqrt((dr² + dg² + db²) / 3) is the distance to u_key: colorkey's d, with the channels in 0..1.
    // - k = clamp((d − similarity) / smoothness, 0, 1); at smoothness 0 it is the step d > similarity.
    // - despill works on the screen channel (u_spillBlue 0: green, 1: blue). Green: S = max(g − (r + b) / 2, 0) and
    //   g' = max(g − spill·S, 0); blue likewise with r and g. Spill 0 leaves the colour as it is.
    // - alpha' = alpha·k: the key multiplies the picture's alpha (the export's alpha wrapper keeps that for stills).
    // The key alpha comes from the input colours, before despill.
    const KEY = PASS_HEAD + `uniform vec3 u_key;
uniform float u_similarity;
uniform float u_smoothness;
uniform float u_spill;
uniform float u_spillBlue;
void main() {
    vec4 colour = texture(u_source, v_uv);
    vec3 c = colour.rgb;
    vec3 diff = c - u_key;
    float d = sqrt(dot(diff, diff) / 3.0);
    float k = u_smoothness > 0.0 ? clamp((d - u_similarity) / u_smoothness, 0.0, 1.0) : (d > u_similarity ? 1.0 : 0.0);
    if (u_spillBlue > 0.5) c.b = max(c.b - u_spill * max(c.b - 0.5 * (c.r + c.g), 0.0), 0.0);
    else c.g = max(c.g - u_spill * max(c.g - 0.5 * (c.r + c.b), 0.0), 0.0);
    outColor = vec4(c, colour.a * k);
}
`;

    // Pass sources per registry effect id, in pass order (blur: horizontal, then vertical; chroma_key: one KEY pass).
    const EFFECTS = { color: [COLOR], lut: [LUT], blur: [BLUR, BLUR], sharpen: [SHARPEN], vignette: [VIGNETTE], chroma_key: [KEY] };

    // ---- 2e blur_in mix (index §4.7, §4.24). ----
    // A clip-local pass with straight alpha in and out (QUAD_VERTEX, u_flipY 0). u_source is the overlay o, u_base the
    // base b, and u_mix = m/255 (VideoStudioAnim.blurUniforms). It mixes premultiplied, pm(b) + (pm(o) − pm(b))·m,
    // alpha included: the GL form of maskedmerge's MM(b, o, m). The result is written straight.
    const ANIM_MIX = PASS_HEAD + `uniform sampler2D u_base;
uniform float u_mix;
void main() {
    vec4 over = texture(u_source, v_uv), base = texture(u_base, v_uv);
    vec4 mixed = mix(vec4(base.rgb * base.a, base.a), vec4(over.rgb * over.a, over.a), u_mix);
    outColor = mixed.a > 0.0 ? vec4(mixed.rgb / mixed.a, mixed.a) : vec4(0.0);
}
`;

    // ---- 2d blend modes (index §4.7, §4.22). ----
    // blend(id) draws QUAD_VERTEX (u_flipY 0) into a canvas-size scratch target without blending. It reads u_dst, the
    // opaque composite below, and u_src, the placed layer (premultiplied), and writes
    //   rgb = mix(dst, F(dst, src.rgb / src.a), src.a), alpha 1,
    // which is the export's blend followed by maskedmerge with the layer alpha.
    // vsBlend(a, b) is F's real form with a = below and b = the layer's straight colour in 0..1. The roles are the same
    // for every mode, because the export's swapped inputs for dodge and burn are already in the formulas. The ends of
    // dodge (layer 255) and burn (layer 0) are tested on 8-bit levels, the only values the RGBA8 layer holds.
    const BLEND_HEAD = FRAGMENT_HEAD + `uniform sampler2D u_dst;
uniform sampler2D u_src;
vec3 vsBlend(vec3 a, vec3 b) {
`;
    const BLEND_MAIN = `}
void main() {
    vec3 below = texture(u_dst, v_uv).rgb;
    vec4 layer = texture(u_src, v_uv);
    vec3 straight = layer.a > 0.0 ? clamp(layer.rgb / layer.a, 0.0, 1.0) : vec3(0.0);
    outColor = vec4(mix(below, clamp(vsBlend(below, straight), 0.0, 1.0), layer.a), 1.0);
}
`;
    // The upper branch of overlay and hardlight: 1 − 2(1 − a)(1 − b).
    const SCREEN2 = '1.0 - 2.0 * (1.0 - a) * (1.0 - b)';
    // vsBlend bodies per registry blend mode (normal has none: it composites inside placeClip).
    const BLENDS = {
        multiply: 'return a * b;',
        screen: 'return 1.0 - (1.0 - a) * (1.0 - b);',
        overlay: 'return mix(' + SCREEN2 + ', 2.0 * a * b, vec3(lessThan(a, vec3(0.5))));',
        softlight: 'return a * a + 2.0 * b * a * (1.0 - a);',
        hardlight: 'return mix(' + SCREEN2 + ', 2.0 * a * b, vec3(lessThan(b, vec3(0.5))));',
        darken: 'return min(a, b);',
        lighten: 'return max(a, b);',
        difference: 'return abs(a - b);',
        exclusion: 'return a + b - 2.0 * a * b;',
        addition: 'return min(a + b, vec3(1.0));',
        subtract: 'return max(a - b, vec3(0.0));',
        dodge: 'return mix(min((256.0 / 255.0) * a / max(1.0 - b, vec3(0.5 / 255.0)), vec3(1.0)), vec3(1.0), vec3(greaterThan(b, vec3(254.5 / 255.0))));',
        burn: 'return mix(max(1.0 - (256.0 / 255.0) * (1.0 - a) / max(b, vec3(0.5 / 255.0)), vec3(0.0)), vec3(0.0), vec3(lessThan(b, vec3(0.5 / 255.0))));'
    };

    window.VideoStudioShaders = {
        QUAD_VERTEX, PLACE_VERTEX, PLACE, COPY, PICTURE, MASK, PLACE_MASKED, ANIM_MIX,
        // Fragment source for a registry transition id (all 24 since 2e, index §4.7), or null.
        transition(id) { return Object.prototype.hasOwnProperty.call(TRANSITIONS, id) ? TRANSITIONS[id] : null; },
        // {passes: [fragmentSource, ...]} for a registry effect id, or null (index §4.7).
        effect(id) { return Object.prototype.hasOwnProperty.call(EFFECTS, id) ? { passes: EFFECTS[id].slice() } : null; },
        // Fragment source for a registry blend mode, or null for normal and unknown ids (index §4.7, 2d).
        blend(id) { return Object.prototype.hasOwnProperty.call(BLENDS, id) ? BLEND_HEAD + '    ' + BLENDS[id] + '\n' + BLEND_MAIN : null; }
    };
})();
