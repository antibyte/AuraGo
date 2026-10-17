(function () {
    'use strict';

    // GLSL ES 3.00 sources for the Video Studio GL compositor (index §4.7). Fragment shaders output premultiplied colour,
    // except the clip-local 2c passes (PICTURE and the effects), which read and write straight alpha.
    // Targets hold images top-down (row 0 is the top row, like uploaded textures); QUAD_VERTEX flips v only when it
    // presents to the canvas (u_flipY = 1). u_progress is p = i / D for transition-local frame i, the same as xfade.
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
    const PLACE_VERTEX = `#version 300 es
layout(location = 0) in vec2 a_pos;
uniform mat3 u_matrix;
uniform vec4 u_uvRect;
out vec2 v_uv;
void main() {
    v_uv = u_uvRect.xy + a_pos * u_uvRect.zw;
    vec3 position = u_matrix * vec3(a_pos, 1.0);
    gl_Position = vec4(position.xy, 0.0, 1.0);
}
`;

    const FRAGMENT_HEAD = `#version 300 es
precision highp float;
in vec2 v_uv;
out vec4 outColor;
`;

    // The source is straight alpha (uploads never premultiply); opacity scales alpha and the output is premultiplied.
    const PLACE = FRAGMENT_HEAD + `uniform sampler2D u_source;
uniform float u_opacity;
void main() {
    vec4 colour = texture(u_source, v_uv);
    float alpha = colour.a * u_opacity;
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

    // xfade "fadeblack": u_weights is VideoStudioEffects.fadeBlackWeights(p) as (out, inc, black), mixed with opaque
    // black, so the tracks below darken too (Task 8 measured the curve; the compositor passes the numbers).
    const BLACK = TRANSITION_HEAD + `uniform vec3 u_weights;
void main() {
    outColor = texture(u_from, v_uv) * u_weights.x + texture(u_to, v_uv) * u_weights.y + vec4(0.0, 0.0, 0.0, u_weights.z);
}
`;

    // Wipes: u_edge is VideoStudioEffects.wipeEdge(type, p, width) as (left, reveal) in target pixels; the incoming
    // layer shows inside [left, left + reveal). "wipeleft" reveals from the left edge, "wiperight" from the right.
    const WIPE = TRANSITION_HEAD + `uniform vec2 u_edge;
void main() {
    float x = v_uv.x * u_size.x;
    vec4 outgoing = texture(u_from, v_uv);
    vec4 incoming = texture(u_to, v_uv);
    outColor = x >= u_edge.x && x < u_edge.x + u_edge.y ? incoming : outgoing;
}
`;

    const TRANSITIONS = { dissolve: DISSOLVE, black: BLACK, wipeleft: WIPE, wiperight: WIPE };

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

    // Pass sources per registry effect id, in pass order (blur: horizontal, then vertical). 2d adds chroma_key.
    const EFFECTS = { color: [COLOR], lut: [LUT], blur: [BLUR, BLUR], sharpen: [SHARPEN], vignette: [VIGNETTE] };

    window.VideoStudioShaders = {
        QUAD_VERTEX, PLACE_VERTEX, PLACE, COPY, PICTURE,
        // Fragment source for a registry transition id, or null (2e adds the other ids).
        transition(id) { return Object.prototype.hasOwnProperty.call(TRANSITIONS, id) ? TRANSITIONS[id] : null; },
        // {passes: [fragmentSource, ...]} for a registry effect id, or null (index §4.7).
        effect(id) { return Object.prototype.hasOwnProperty.call(EFFECTS, id) ? { passes: EFFECTS[id].slice() } : null; }
    };
})();
