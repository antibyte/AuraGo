// Ocean chat theme: GLSL sources and material factories of the sea life layer.
// The geometry comes from the Blender kit (assets/chat-ocean-life): TEXCOORD_0 = (along, part),
// TEXCOORD_1 = (height, side), bound as aAnim by the scene. All animation happens here.
// three.js is passed in so this module stays free of imports and can be read in isolation.

export const FOG_COLOR = [0.02, 0.10, 0.18];
export const FOG_DENSITY = 0.052;

// Shared by every fragment shader. x = colour fog: 1 - exp(-(d * density)^2), animals dissolve into the water
// tint with distance. y = alpha fog: the same with the distance multiplied by the species' uFogScale (the
// "fog" field of the species table), so big, far animals keep a readable alpha as blue-grey silhouettes.
// uHaze (the "haze" field, default 1) scales the colour fog: the whale and the manta keep most of their dark body so
// they read as dark shapes against the lighter water instead of melting into the water tint.
const FOG_CHUNK = /* glsl */`
uniform float uFogDensity;
uniform float uFogScale;
uniform float uHaze;
uniform vec3 uFogColor;
vec2 fogAmounts(float dist) {
    float fd = uFogDensity * dist;
    float fa = fd * uFogScale;
    return vec2((1.0 - exp(-fd * fd)) * uHaze, 1.0 - exp(-fa * fa));
}`;

const SHADER_ID = { fish: 0, shark: 1, turtle: 2, whale: 3 };
const MATERIAL_KIND = { body: 0, fin: 1, eye: 2, shell: 3, flipper: 4, fluke: 5 };
const DOUBLE_SIDED = { fin: true, flipper: true, fluke: true };
const JELLY_PART = { bell: 0, gonad: 1, arm: 2, tentacle: 3 };

const CREATURE_VERTEX = /* glsl */`
attribute vec2 aAnim;          // (height, side) from TEXCOORD_1
attribute vec4 aInst;          // phase, frequency, amplitude, fade (per instance)
uniform float uTime;
uniform float uShader;         // 0 fish, 1 shark, 2 turtle, 3 whale
uniform float uSwimK;
uniform float uLength;
uniform float uFront;          // z of the nose after scaling
varying vec3 vViewPos;
varying vec3 vWorldPos;
varying vec3 vWorldNormal;
varying vec2 vUv;
varying vec2 vAnim;
varying float vFade;

void main() {
    vec3 p = position;
    float partv = uv.y;
    // 0 at the nose, 1 at the tail tip, measured on the body axis so fins follow the body they sit on
    float axis = clamp((uFront - position.z) / uLength, 0.0, 1.0);
    float t = uTime * aInst.y + aInst.x * 6.2831853;
    if (uShader < 1.5) {
        // fish and sharks: lateral undulation, stronger toward the tail
        float w = axis * axis * aInst.z;
        p.x += sin(t - axis * uSwimK) * w;
        if (partv > 0.2 && partv < 0.3) p.x += sin(t * 1.7 + uv.x * 3.0) * 0.3 * aInst.z * uv.x;
    } else if (uShader < 2.5) {
        // turtle: flipper stroke about the root, the tip travels furthest
        if (partv > 0.7 && partv < 0.8) {
            float reach = clamp(uv.x, 0.0, 1.0);
            float lead = position.z > 0.0 ? 1.0 : 0.5;
            p.y += sin(t) * aInst.z * reach * lead;
            p.z -= cos(t) * aInst.z * 0.3 * reach * lead;
        }
    } else {
        // whale: vertical fluke stroke and a slow body wave
        float w = pow(axis, 2.2) * aInst.z;
        p.y += sin(t - axis * uSwimK) * w * (partv > 0.9 ? 1.3 : 1.0);
    }
    vec4 world = modelMatrix * instanceMatrix * vec4(p, 1.0);
    vec4 mv = viewMatrix * world;
    vViewPos = mv.xyz;
    vWorldPos = world.xyz;
    vWorldNormal = normalize(mat3(modelMatrix) * mat3(instanceMatrix) * normal);
    vUv = uv;
    vAnim = aAnim;
    vFade = aInst.w;
    gl_Position = projectionMatrix * mv;
}`;

const CREATURE_FRAGMENT = /* glsl */`
uniform float uTime;
uniform float uBusy;
uniform float uShader;
uniform float uVariant;
uniform float uMat;            // 0 body, 1 fin, 2 eye, 3 shell, 4 flipper, 5 fluke
uniform vec3 uLightDir;
${FOG_CHUNK}
varying vec3 vViewPos;
varying vec3 vWorldPos;
varying vec3 vWorldNormal;
varying vec2 vUv;
varying vec2 vAnim;
varying float vFade;

float hash21(vec2 p) {
    p = fract(p * vec2(123.34, 456.21));
    p += dot(p, p + 45.32);
    return fract(p.x * p.y);
}

float vnoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    float a = hash21(i);
    float b = hash21(i + vec2(1.0, 0.0));
    float c = hash21(i + vec2(0.0, 1.0));
    float d = hash21(i + vec2(1.0, 1.0));
    return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

vec3 hue(float h) {
    return clamp(abs(fract(h + vec3(0.0, 0.6667, 0.3333)) * 6.0 - 3.0) - 1.0, 0.0, 1.0);
}

vec3 fishColor(float variant, float along, float height, float partv, float mat, vec3 N, vec3 V, inout float alpha) {
    bool isFin = mat > 0.5 && mat < 1.5;
    bool isTail = isFin && partv > 0.9;
    vec3 col = vec3(0.5);
    if (variant < 0.5) {
        // blue tang: royal blue, black palette stripe with a blue edge, yellow tail
        if (isTail) {
            col = vec3(0.95, 0.8, 0.1);
        } else if (isFin) {
            col = mix(vec3(0.06, 0.2, 0.78), vec3(0.03, 0.09, 0.5), along);
        } else {
            float stripe = smoothstep(0.26, 0.34, along) * (1.0 - smoothstep(0.8, 0.9, along));
            float palette = smoothstep(0.5, 0.58, height) * stripe;
            float edge = smoothstep(0.4, 0.5, height) * (1.0 - smoothstep(0.5, 0.58, height)) * stripe;
            col = mix(vec3(0.08, 0.25, 0.85), vec3(0.16, 0.5, 1.0), edge * 0.7);
            col = mix(col, vec3(0.015, 0.02, 0.1), palette);
            col = mix(col, vec3(0.95, 0.8, 0.1), smoothstep(0.86, 0.9, along));
        }
    } else if (variant < 1.5) {
        // clownfish: orange with three white bars edged in black
        vec3 orange = vec3(0.95, 0.42, 0.08);
        if (isFin) {
            col = mix(orange, vec3(0.03, 0.02, 0.02), smoothstep(0.84, 0.9, along));
        } else {
            float bars = 0.0;
            float rims = 0.0;
            for (int i = 0; i < 3; i++) {
                float c = i == 0 ? 0.22 : (i == 1 ? 0.52 : 0.85);
                float d = abs(along - c);
                bars = max(bars, 1.0 - smoothstep(0.041, 0.049, d));
                rims = max(rims, 1.0 - smoothstep(0.053, 0.061, d));
            }
            col = mix(orange, vec3(0.03, 0.02, 0.02), clamp(rims - bars, 0.0, 1.0));
            col = mix(col, vec3(0.96, 0.95, 0.9), bars);
        }
    } else if (variant < 2.5) {
        // butterflyfish: yellow, dark eye band, tail spot with a pale ring
        vec3 yellow = vec3(0.95, 0.78, 0.15);
        if (isFin) {
            col = mix(yellow, vec3(0.22, 0.17, 0.08), smoothstep(0.78, 0.95, along));
        } else {
            // the kit puts the eyes on the flanks at along ~0.28 (long pointed head), the band runs through them
            float band = smoothstep(0.205, 0.235, along) * (1.0 - smoothstep(0.32, 0.35, along));
            col = mix(yellow, vec3(0.02, 0.02, 0.025), band);
            float r = length(vec2(along - 0.86, (height - 0.75) * 0.6));
            float ring = 1.0 - smoothstep(0.048, 0.06, r);
            float spot = 1.0 - smoothstep(0.034, 0.046, r);
            col = mix(col, vec3(0.95, 0.95, 0.9), ring - spot);
            col = mix(col, vec3(0.03, 0.03, 0.035), spot);
            col = mix(col, vec3(0.92, 0.9, 0.7), (1.0 - smoothstep(0.05, 0.08, along)) * 0.8);
        }
    } else {
        // sardine: silver, blue back, view-angle iridescence, translucent fins
        vec3 silver = vec3(0.72, 0.78, 0.82);
        if (isFin) {
            alpha *= 0.6;
            col = silver * 0.9;
        } else {
            col = mix(silver, vec3(0.15, 0.3, 0.5), smoothstep(0.6, 0.72, height));
            float fres = pow(1.0 - clamp(dot(N, V), 0.0, 1.0), 2.0);
            vec3 irid = hue(fres * 0.5 + along * 0.2 + 0.6);
            col = mix(col, col * (0.55 + irid * 0.9), 0.35 * fres + 0.12);
        }
    }
    return col;
}

vec3 sharkColor(float variant, float along, float height, float side, float partv, float mat, vec3 N) {
    bool isFin = mat > 0.5 && mat < 1.5;
    vec3 light = variant < 0.5 ? vec3(0.82, 0.85, 0.86) : vec3(0.75, 0.74, 0.70);
    vec3 dark = variant < 0.5 ? vec3(0.28, 0.33, 0.38) : vec3(0.30, 0.30, 0.30);
    if (isFin) {
        vec3 col = mix(light, dark, 0.55);
        col = mix(col, light, smoothstep(0.1, -0.6, N.y) * 0.6);
        if (variant < 0.5) col = mix(col, vec3(0.02, 0.02, 0.03), smoothstep(0.78, 0.84, along));
        return col;
    }
    vec3 col = mix(light, dark, smoothstep(0.35, 0.7, height));
    if (variant < 0.5 && mat < 0.5) {
        // five gill slits
        float gi = (along - 0.2) / 0.02;
        if (gi >= 0.0 && gi < 5.0 && height > 0.3 && height < 0.6 && abs(side - 0.5) > 0.3) {
            float line = 1.0 - smoothstep(0.1, 0.28, abs(fract(gi) - 0.5));
            col *= 1.0 - 0.55 * line;
        }
    }
    return col;
}

vec3 turtleColor(float along, float height, float side, float mat, vec3 N) {
    vec3 col;
    if (mat > 2.5 && mat < 3.5) {
        // carapace with scute seams, cream plastron
        vec3 olive = vec3(0.33, 0.38, 0.2);
        float mott = vnoise(vec2(along * 11.0 + side * 3.0, height * 7.0));
        col = olive * (0.78 + 0.45 * mott);
        float column = fract(along * 5.0);
        float seamA = 1.0 - smoothstep(0.04, 0.08, column);
        float seamB = 1.0 - smoothstep(0.04, 0.08, fract(height * 3.0 + step(0.5, column) * 0.5));
        col = mix(col, olive * 0.3, max(seamA, seamB));
        col = mix(col, vec3(0.78, 0.72, 0.55), 1.0 - smoothstep(0.08, 0.12, height));
    } else if (mat > 3.5 && mat < 4.5) {
        // flippers: dark olive with light speckles and a paler underside
        col = vec3(0.2, 0.25, 0.14);
        float speck = smoothstep(0.78, 0.84, vnoise(vec2(along * 16.0, height * 12.0 + side * 5.0)));
        col = mix(col, vec3(0.72, 0.7, 0.46), speck * 0.55);
        col = mix(col, vec3(0.6, 0.55, 0.38), smoothstep(0.1, -0.6, N.y) * 0.4);
    } else {
        // head, neck and tail
        col = mix(vec3(0.38, 0.32, 0.18), vec3(0.72, 0.64, 0.42), 1.0 - smoothstep(0.0, 0.4, height));
    }
    return col;
}

vec3 whaleColor(float along, float height, float side, float mat, vec3 N) {
    vec3 base = vec3(0.05, 0.06, 0.08);
    if (mat < 0.5) {
        vec3 col = mix(base, vec3(0.3, 0.32, 0.34), 1.0 - smoothstep(0.22, 0.32, height));
        // throat grooves
        float groove = (1.0 - smoothstep(0.3, 0.38, height)) * (1.0 - smoothstep(0.36, 0.44, along)) * (1.0 - smoothstep(0.45, 0.55, fract(side * 14.0)));
        return col * (1.0 - 0.3 * groove);
    }
    // flukes, flippers, dorsal hump: body colour with a lighter underside
    return mix(base, vec3(0.42, 0.45, 0.48), smoothstep(0.1, -0.6, N.y) * 0.55);
}

vec3 shade(vec3 albedo, vec3 N, vec3 V, vec3 L, vec3 wp, float gloss) {
    float ndl = dot(N, L);
    float diffuse = max(ndl, 0.0);
    float wrap = clamp(ndl * 0.5 + 0.5, 0.0, 1.0);
    vec3 sky = mix(vec3(0.07, 0.14, 0.24), vec3(0.6, 0.76, 0.84), N.y * 0.5 + 0.5);
    vec3 sun = vec3(0.92, 1.0, 1.0);
    vec3 lit = albedo * (sky * 0.8 + sun * (diffuse * 0.85 + wrap * 0.12));
    // caustics: two scrolling value-noise layers on world xz, only on up-facing surfaces
    float up = smoothstep(0.05, 0.75, N.y);
    vec2 cp = wp.xz * 0.9;
    float n1 = vnoise(cp * 1.3 + vec2(uTime * 0.16, uTime * 0.11));
    float n2 = vnoise(cp * 2.1 - vec2(uTime * 0.12, -uTime * 0.17) + 7.3);
    float web = clamp(1.0 - abs(n1 + n2 - 1.0), 0.0, 1.0);
    lit += albedo * sun * pow(web, 6.0) * up * 0.9;
    float rim = pow(1.0 - clamp(dot(N, V), 0.0, 1.0), 3.0);
    lit += vec3(0.12, 0.38, 0.5) * rim * 0.4;
    lit += sun * pow(max(dot(reflect(-L, N), V), 0.0), 40.0) * gloss * 0.5;
    return lit * (1.0 + 0.08 * uBusy);
}

void main() {
    vec3 N = normalize(vWorldNormal);
    if (!gl_FrontFacing) N = -N;
    vec3 V = normalize(cameraPosition - vWorldPos);
    vec3 L = normalize(uLightDir);
    float along = clamp(vUv.x, 0.0, 1.0);
    float partv = vUv.y;
    float height = vAnim.x;
    float side = vAnim.y;
    float alpha = 1.0;
    float gloss = 0.2;
    vec3 albedo;
    if (uMat > 1.5 && uMat < 2.5) {
        albedo = vec3(0.02, 0.02, 0.03);
        gloss = 3.0;
    } else if (uShader < 0.5) {
        albedo = fishColor(uVariant, along, height, partv, uMat, N, V, alpha);
        gloss = 0.5;
    } else if (uShader < 1.5) {
        albedo = sharkColor(uVariant, along, height, side, partv, uMat, N);
    } else if (uShader < 2.5) {
        albedo = turtleColor(along, height, side, uMat, N);
    } else {
        albedo = whaleColor(along, height, side, uMat, N);
        gloss = 0.1;
    }
    vec3 col = shade(albedo, N, V, L, vWorldPos, gloss);
    // the whale is a silhouette: the rim glow and ambient light of shade() would lift it to the water colour
    if (uShader > 2.5) col *= 0.45;
    vec2 fog = fogAmounts(length(vViewPos));
    col = mix(col, uFogColor, fog.x * 0.85);
    gl_FragColor = vec4(col, alpha * (1.0 - fog.y) * vFade);
}`;

// Moon jelly: ported from the Tiefsee screensaver (abyss-scene.js) with straight alpha instead of
// additive blending, so the bell reads as translucent against the Ocean water gradient.
const JELLY_VERTEX = /* glsl */`
attribute vec2 aAnim;
attribute vec4 aInst;   // pulse cycle (accumulated on the CPU), frequency (0: the CPU owns the cycle), swim, fade
attribute vec3 aTint;
uniform float uTime;
uniform float uPart;
varying vec3 vViewPos;
varying vec3 vNormal;
varying vec2 vUv;
varying vec2 vAnim;
varying vec3 vTint;
varying float vFade;
varying float vPulse;

float contraction(float c) {
    return c < 0.22 ? smoothstep(0.0, 0.22, c) : 1.0 - smoothstep(0.22, 1.0, c);
}

void main() {
    float cycle = fract(uTime * aInst.y + aInst.x);
    float k = contraction(cycle);
    float along = clamp(uv.x, 0.0, 1.0);
    float phase = uv.y;
    vec3 p = position;
    if (uPart < 0.5 || (uPart > 0.5 && uPart < 1.5)) {
        float a = uPart < 0.5 ? along : clamp(length(p.xz), 0.0, 1.0);
        p.xz *= 1.0 - 0.17 * k * pow(max(a, 0.0), 1.3);
        p.y += 0.07 * k * a;
    } else if (uPart < 3.5) {
        float lagK = contraction(fract(cycle - along * 0.35));
        p.xz *= 1.0 - 0.15 * lagK * (1.0 - along * 0.7);
        float sway = pow(max(along, 0.0), 1.5) * (uPart > 2.5 ? 0.22 : 0.16) * (1.0 - 0.5 * aInst.z);
        p.x += sin(uTime * 0.9 + phase * 6.2832 + along * 5.0) * sway;
        p.z += cos(uTime * 0.7 + phase * 6.2832 + along * 4.2) * sway;
        p.y -= aInst.z * pow(max(along, 0.0), 1.2) * 0.35;
    } else {
        p *= 1.0 + 0.025 * sin(uTime * 1.3 + aInst.x * 6.2832);
    }
    vec4 mv = modelViewMatrix * instanceMatrix * vec4(p, 1.0);
    vViewPos = mv.xyz;
    vNormal = normalize(normalMatrix * mat3(instanceMatrix) * normal);
    vUv = uv;
    vAnim = aAnim;
    vTint = aTint;
    vFade = aInst.w;
    vPulse = k;
    gl_Position = projectionMatrix * mv;
}`;

const JELLY_FRAGMENT = /* glsl */`
uniform float uTime;
uniform float uPart;
uniform float uBusy;
uniform vec3 uBase;
uniform vec3 uGlow;
${FOG_CHUNK}
varying vec3 vViewPos;
varying vec3 vNormal;
varying vec2 vUv;
varying vec2 vAnim;
varying vec3 vTint;
varying float vFade;
varying float vPulse;

void main() {
    vec3 V = normalize(-vViewPos);
    vec3 N = normalize(vNormal);
    if (!gl_FrontFacing) N = -N;
    float facing = abs(dot(N, V));
    float fresnel = pow(clamp(1.0 - facing, 0.0, 1.0), 2.2);
    float along = vUv.x;
    float phase = vUv.y;
    float boost = 1.0 + 0.4 * uBusy;
    vec3 base = uBase * vTint;
    vec3 glow = uGlow * vTint * boost;
    vec3 col;
    float alpha;
    if (uPart < 0.5) {
        float rim = exp(-(1.0 - along) * 14.0);
        float dome = exp(-along * 3.0) * 0.35;
        float veins = pow(clamp(0.5 + 0.5 * cos(vAnim.x * 6.2832 * 16.0), 0.0, 1.0), 12.0) * smoothstep(0.2, 0.9, along) * 0.35;
        col = base * (0.35 + 0.65 * fresnel + dome * 0.8 + veins * 1.5) + glow * rim * (0.35 + 0.5 * vPulse);
        alpha = clamp(0.16 + 0.55 * fresnel + 0.5 * rim + dome * 0.5 + veins, 0.0, 0.92);
    } else if (uPart < 1.5) {
        col = glow * (0.55 + 0.35 * fresnel) * (0.8 + 0.4 * vPulse);
        alpha = 0.7;
    } else if (uPart < 2.5) {
        float wave = exp(-pow((fract(along * 1.4 - uTime * 0.22 + phase) - 0.5) * 5.0, 2.0));
        col = base * (0.3 + 0.5 * fresnel) + glow * wave * 0.4;
        alpha = 0.6 * (1.0 - along * 0.5);
    } else {
        float beads = smoothstep(0.86, 1.0, sin(along * 34.0 - uTime * 2.6 + phase * 40.0));
        float travel = exp(-pow((fract(along * 0.8 - uTime * 0.35 + phase) - 0.5) * 7.0, 2.0));
        col = base * 0.4 + glow * (0.3 + beads * 0.6 + travel * 0.9);
        alpha = 0.55 * (1.0 - along * 0.6);
    }
    vec2 fog = fogAmounts(length(vViewPos));
    float nearFade = smoothstep(1.5, 5.0, -vViewPos.z);
    col = mix(col, uFogColor, fog.x * 0.85);
    gl_FragColor = vec4(col, alpha * (1.0 - fog.y) * vFade * nearFade);
}`;

// Manta ray: the Tiefsee flap shader with the model scale folded in (uUnit) and an alpha fog.
const MANTA_VERTEX = /* glsl */`
attribute vec2 aAnim;
attribute vec4 aInst;   // phase, unused, unused, fade
uniform float uTime;
uniform float uFlap;
uniform float uUnit;
varying vec3 vViewPos;
varying vec3 vWorldNormal;
varying float vBelly;
varying float vSpan;
varying vec3 vObj;
varying float vFade;
void main() {
    vec3 p = position;
    vObj = position / uUnit;
    float span = clamp(uv.x, 0.0, 1.0);
    float ph = uTime * uFlap + aInst.x * 6.2831853;
    float wave = sin(ph - span * 1.4);
    p.y += wave * pow(span, 1.6) * 0.42 * uUnit;
    p.z += cos(ph - span * 1.4) * pow(span, 2.0) * 0.08 * uUnit;
    vec4 world = modelMatrix * instanceMatrix * vec4(p, 1.0);
    vec4 mv = viewMatrix * world;
    vViewPos = mv.xyz;
    vWorldNormal = normalize(mat3(modelMatrix) * mat3(instanceMatrix) * normal);
    vBelly = aAnim.y;
    vSpan = span;
    vFade = aInst.w;
    gl_Position = projectionMatrix * mv;
}`;

const MANTA_FRAGMENT = /* glsl */`
uniform vec3 uLightDir;
${FOG_CHUNK}
varying vec3 vViewPos;
varying vec3 vWorldNormal;
varying float vBelly;
varying float vSpan;
varying vec3 vObj;
varying float vFade;
void main() {
    vec3 N = normalize(vWorldNormal);
    if (!gl_FrontFacing) N = -N;
    vec3 V = normalize(-vViewPos);
    float light = clamp(dot(N, uLightDir), 0.0, 1.0);
    float sky = clamp(N.y * 0.5 + 0.5, 0.0, 1.0);
    vec3 top = mix(vec3(0.045, 0.07, 0.1), vec3(0.09, 0.13, 0.17), vSpan * 0.5);
    float spots = smoothstep(0.72, 0.9, sin(vObj.x * 13.1) * sin(vObj.z * 11.7 + vObj.x * 3.1)) * (1.0 - smoothstep(0.2, 0.6, vSpan));
    vec3 belly = mix(vec3(0.74, 0.78, 0.8), vec3(0.28, 0.33, 0.38), smoothstep(0.45, 1.0, vSpan));
    belly *= 1.0 - 0.7 * spots;
    vec3 albedo = mix(top, belly, step(0.5, vBelly));
    vec3 ambient = mix(vec3(0.04, 0.09, 0.16), vec3(0.32, 0.55, 0.66), sky);
    float rim = pow(clamp(1.0 - abs(dot(N, V)), 0.0, 1.0), 4.0);
    vec3 col = albedo * (ambient * 0.9 + vec3(0.65, 0.9, 1.0) * light * 0.95) + vec3(0.12, 0.34, 0.46) * rim * 0.35;
    // like the whale, the manta far back is a silhouette: the shading is multiplied down so it reads as a dark wing shape
    col *= 0.5;
    vec2 fog = fogAmounts(length(vViewPos));
    col = mix(col, uFogColor, fog.x * 0.85);
    gl_FragColor = vec4(col, (1.0 - fog.y) * vFade);
}`;

export function createSharedUniforms(THREE) {
    return {
        uTime: { value: 0 },
        uBusy: { value: 0 },
        uLightDir: { value: new THREE.Vector3(0.2, 1, 0.25).normalize() },
        uFogColor: { value: new THREE.Vector3(FOG_COLOR[0], FOG_COLOR[1], FOG_COLOR[2]) },
        uFogDensity: { value: FOG_DENSITY }
    };
}

// part = glTF material name (body, fin, eye, shell, flipper, fluke). The kit's UVs, not the
// material, decide where a fin ends and a tail begins, so the material kind is only a coarse switch.
export function createCreatureMaterial(THREE, { shader, variant = 0, part = 'body', shared, swimK = 2, length = 1, front, fog = 1, haze = 1 }) {
    const kind = MATERIAL_KIND[part] == null ? 0 : MATERIAL_KIND[part];
    return new THREE.ShaderMaterial({
        uniforms: {
            uTime: shared.uTime,
            uBusy: shared.uBusy,
            uLightDir: shared.uLightDir,
            uFogColor: shared.uFogColor,
            uFogDensity: shared.uFogDensity,
            uFogScale: { value: fog },
            uHaze: { value: haze },
            uShader: { value: SHADER_ID[shader] == null ? 0 : SHADER_ID[shader] },
            uVariant: { value: variant },
            uMat: { value: kind },
            uSwimK: { value: swimK },
            uLength: { value: length },
            uFront: { value: front == null ? length * 0.5 : front }
        },
        vertexShader: CREATURE_VERTEX,
        fragmentShader: CREATURE_FRAGMENT,
        transparent: true,
        depthWrite: true,
        side: DOUBLE_SIDED[part] ? THREE.DoubleSide : THREE.FrontSide
    });
}

export function createJellyMaterial(THREE, { part = 'bell', shared, fog = 1, haze = 1 }) {
    const kind = JELLY_PART[part] == null ? 0 : JELLY_PART[part];
    return new THREE.ShaderMaterial({
        uniforms: {
            uTime: shared.uTime,
            uBusy: shared.uBusy,
            uFogColor: shared.uFogColor,
            uFogDensity: shared.uFogDensity,
            uFogScale: { value: fog },
            uHaze: { value: haze },
            uPart: { value: kind },
            uBase: { value: new THREE.Vector3(0.5, 0.72, 1.0) },
            uGlow: { value: kind === 1 ? new THREE.Vector3(1.0, 0.45, 0.85) : new THREE.Vector3(0.75, 0.55, 1.0) }
        },
        vertexShader: JELLY_VERTEX,
        fragmentShader: JELLY_FRAGMENT,
        transparent: true,
        depthWrite: false,
        side: THREE.DoubleSide
    });
}

export function createMantaMaterial(THREE, { shared, flap = 1.2, unit = 1, fog = 1, haze = 1 }) {
    return new THREE.ShaderMaterial({
        uniforms: {
            uTime: shared.uTime,
            uLightDir: shared.uLightDir,
            uFogColor: shared.uFogColor,
            uFogDensity: shared.uFogDensity,
            uFogScale: { value: fog },
            uHaze: { value: haze },
            uFlap: { value: flap },
            uUnit: { value: unit }
        },
        vertexShader: MANTA_VERTEX,
        fragmentShader: MANTA_FRAGMENT,
        transparent: true,
        depthWrite: true,
        side: THREE.DoubleSide
    });
}
