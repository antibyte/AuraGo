(function () {
    'use strict';

    // GLSL ES 3.00 sources for the Video Studio GL compositor (index §4.7). Fragment shaders output premultiplied colour.
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

    window.VideoStudioShaders = {
        QUAD_VERTEX, PLACE_VERTEX, PLACE, COPY,
        // Fragment source for a registry transition id, or null (2e adds the other ids).
        transition(id) { return Object.prototype.hasOwnProperty.call(TRANSITIONS, id) ? TRANSITIONS[id] : null; }
    };
})();
