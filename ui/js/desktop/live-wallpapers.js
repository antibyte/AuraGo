/**
 * Animated desktop wallpapers: silk_flow and firefly_dusk (calm), neon_overdrive and
 * fractal_trip (wild). One WebGL canvas sits behind the whole shell (fixed, z-index -1)
 * and runs the active wallpaper as a full-screen fragment shader, rendered below device
 * resolution and frame-capped. It pauses while the tab is hidden, the screensaver runs or a
 * maximized window covers the desktop, and lowers its resolution when frames fall behind.
 * Reduced motion or data-animations="false" paint one still frame; without WebGL the
 * wallpaper's CSS gradient (desktop-wallpaper-live.css) stays visible.
 */

const HOST_ID = "vd-wallpaper-live";
const TIME_WRAP = 8192; // seconds; keeps shader float precision over long sessions
const MAX_PIXELS = 900000;
const OCCLUSION_CHECK_MS = 1000;
const ADAPT_WINDOW_MS = 2500;

export const LIVE_WALLPAPER_MARKERS = Object.freeze({
  running: "live-wallpaper:running",
  still: "live-wallpaper:still",
  paused: "live-wallpaper:paused",
  unavailable: "live-wallpaper:webgl-unavailable",
  off: "live-wallpaper:off",
});

const VERTEX = `
attribute vec2 a_pos;
void main() { gl_Position = vec4(a_pos, 0.0, 1.0); }
`;

const COMMON = `
#ifdef GL_FRAGMENT_PRECISION_HIGH
precision highp float;
#else
precision mediump float;
#endif
uniform vec2 u_res;
uniform float u_time;
uniform vec2 u_pointer;
uniform float u_pulse;
uniform vec2 u_pulseAt;
#define TAU 6.28318530718

float hash12(vec2 p) {
    vec3 p3 = fract(vec3(p.xyx) * 0.1031);
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.x + p3.y) * p3.z);
}

vec2 hash22(vec2 p) {
    vec3 p3 = fract(vec3(p.xyx) * vec3(0.1031, 0.1030, 0.0973));
    p3 += dot(p3, p3.yzx + 33.33);
    return fract((p3.xx + p3.yz) * p3.zy);
}

float noise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    vec2 u = f * f * (3.0 - 2.0 * f);
    return mix(mix(hash12(i), hash12(i + vec2(1.0, 0.0)), u.x),
               mix(hash12(i + vec2(0.0, 1.0)), hash12(i + vec2(1.0, 1.0)), u.x), u.y);
}

float fbm(vec2 p) {
    float v = 0.0;
    float a = 0.5;
    mat2 m = mat2(1.6, 1.2, -1.2, 1.6);
    for (int i = 0; i < 5; i++) {
        v += a * noise(p);
        p = m * p;
        a *= 0.5;
    }
    return v;
}

// Dither against banding in the slow gradients.
vec3 finish(vec3 c) {
    return c + (hash12(gl_FragCoord.xy + fract(u_time * 7.0) * 97.0) - 0.5) / 255.0;
}
`;

// Calm: folds of dark silk drifting through indigo and teal; the sheen follows the pointer.
const SILK_FLOW = `
void main() {
    vec2 p = (gl_FragCoord.xy - 0.5 * u_res) / u_res.y;
    float t = u_time;
    vec2 w = p * 1.6;
    for (int i = 1; i < 7; i++) {
        float fi = float(i);
        w.x += 0.55 / fi * cos(fi * 2.1 * w.y + t * 0.09 + fi * 1.3);
        w.y += 0.45 / fi * cos(fi * 1.6 * w.x - t * 0.07 + fi * 0.7);
    }
    float phase = (w.x * 1.1 + w.y * 0.7) * 2.0 + t * 0.05 + (u_pointer.x - 0.5) * 0.8;
    float band = 0.5 + 0.5 * sin(phase);
    float tint = smoothstep(0.3, 0.75, noise(p * 0.9 + vec2(t * 0.02, -t * 0.015)));
    vec3 deep = vec3(0.022, 0.022, 0.080);
    vec3 mid = vec3(0.150, 0.110, 0.380);
    vec3 light = vec3(0.480, 0.410, 0.900);
    vec3 teal = vec3(0.050, 0.340, 0.400);
    vec3 col = mix(mid, deep, pow(1.0 - band, 2.0));
    col = mix(col, teal * (0.3 + 0.7 * band), tint * 0.45);
    col += light * pow(band, 6.0) * 0.4;
    col += vec3(1.0, 0.84, 0.94) * pow(band, 48.0) * 0.22;
    col *= 0.95 + 0.05 * sin(t * 0.11);
    col *= mix(0.6, 1.0, smoothstep(1.3, 0.2, length(p * vec2(0.8, 1.0))));
    gl_FragColor = vec4(finish(col), 1.0);
}
`;

// Calm: dusk over layered hills, drifting mist, a crescent moon and slowly blinking fireflies.
const FIREFLY_DUSK = `
float ridge(float x, float s) {
    return noise(vec2(x, s)) * 0.6 + noise(vec2(x * 2.3, s + 7.0)) * 0.28 + noise(vec2(x * 5.1, s + 13.0)) * 0.12;
}

vec3 fireflies(vec2 p, float cell, float size, float seed, float t) {
    vec3 acc = vec3(0.0);
    vec2 id = floor(p / cell);
    for (int j = -1; j <= 1; j++) {
        for (int i = -1; i <= 1; i++) {
            vec2 c = id + vec2(float(i), float(j));
            vec2 h = hash22(c + seed);
            if (h.x < 0.42) continue;
            float ph = h.y * TAU;
            vec2 wander = vec2(sin(t * (0.10 + 0.08 * h.x) + ph), cos(t * (0.08 + 0.07 * h.y) + ph * 1.7)) * 0.4;
            wander.y += 0.06 * sin(t * 0.9 + ph * 2.0);
            float d = length(p - (c + 0.5 + wander) * cell);
            float blink = 0.5 + 0.5 * sin(t * (0.35 + 0.45 * h.y) + ph * 3.0);
            blink = blink * blink * blink;
            float glow = exp(-d * d / (size * size)) + exp(-d / (size * 4.0)) * 0.35;
            acc += mix(vec3(0.78, 1.0, 0.42), vec3(1.0, 0.82, 0.36), h.y) * blink * glow;
        }
    }
    return acc;
}

void main() {
    vec2 p = (gl_FragCoord.xy - 0.5 * u_res) / u_res.y;
    float t = u_time;
    float aa = 1.5 / u_res.y;
    float par = u_pointer.x - 0.5;
    vec3 col = mix(vec3(0.80, 0.42, 0.30), vec3(0.40, 0.20, 0.42), smoothstep(-0.04, 0.10, p.y));
    col = mix(col, vec3(0.12, 0.11, 0.28), smoothstep(0.06, 0.24, p.y));
    col = mix(col, vec3(0.020, 0.028, 0.085), smoothstep(0.2, 0.5, p.y));
    vec2 sc = p * 70.0;
    vec2 sid = floor(sc);
    vec2 sh = hash22(sid + 11.0);
    float sd = length(sc - sid - 0.25 - 0.5 * hash22(sid + 37.0));
    float twinkle = 0.55 + 0.45 * sin(t * (0.7 + 1.8 * sh.y) + sh.x * 40.0);
    col += vec3(0.85, 0.90, 1.0) * step(0.962, sh.x) * exp(-sd * sd * 14.0) * twinkle * (0.35 + 0.65 * sh.y) * smoothstep(0.08, 0.34, p.y);
    vec2 mc = vec2(0.46 - par * 0.02, 0.29);
    float md = length(p - mc);
    float crescent = smoothstep(0.046, 0.043, md) * (1.0 - smoothstep(0.046, 0.043, length(p - mc - vec2(0.019, 0.011))));
    col += vec3(1.0, 0.95, 0.82) * crescent * 0.9 + vec3(0.55, 0.52, 0.70) * exp(-md * 10.0) * 0.10;
    float yFar = -0.08 + 0.12 * ridge(p.x * 1.1 + par * 0.03, 1.0);
    vec3 farHill = mix(vec3(0.13, 0.09, 0.22), vec3(0.24, 0.16, 0.32), smoothstep(yFar - 0.2, yFar, p.y));
    col = mix(col, mix(col, farHill, 0.8), smoothstep(yFar + aa, yFar - aa, p.y));
    float mist = fbm(vec2(p.x * 1.6 + t * 0.015, p.y * 5.0 - t * 0.004));
    col = mix(col, vec3(0.60, 0.44, 0.58), mist * smoothstep(0.02, -0.16, p.y) * smoothstep(-0.42, -0.12, p.y) * 0.22);
    float yMid = -0.17 + 0.10 * ridge(p.x * 1.5 + par * 0.06 + 4.0, 5.0);
    col = mix(col, vec3(0.055, 0.050, 0.110), smoothstep(yMid + aa, yMid - aa, p.y));
    col += fireflies(p + vec2(par * 0.06, 0.0), 0.085, 0.0035, 3.0, t) * smoothstep(yMid + 0.12, yMid - 0.02, p.y) * 0.9;
    float g = noise(vec2(p.x * 150.0, 2.0));
    float yNear = -0.33 + 0.07 * ridge(p.x * 2.0 + par * 0.1 + 9.0, 9.0) + 0.018 * g * g;
    col = mix(col, vec3(0.016, 0.020, 0.034), smoothstep(yNear + aa, yNear - aa, p.y));
    col += fireflies(p + vec2(par * 0.12, 0.0), 0.15, 0.0065, 17.0, t * 0.9) * smoothstep(0.02, -0.12, p.y);
    col *= mix(0.7, 1.0, smoothstep(1.35, 0.25, length(p * vec2(0.75, 1.0))));
    gl_FragColor = vec4(finish(col), 1.0);
}
`;

// Wild: a synthwave run over a neon grid with glitch bursts, hyperdrive surges and a
// shockwave wherever the empty desktop is clicked.
const NEON_OVERDRIVE = `
float ridged(float x) { return 1.0 - abs(noise(vec2(x, 3.7)) * 2.0 - 1.0); }

vec3 neonScene(vec2 p, float t, float kick, float travel, float hyper) {
    float hz = -0.07;
    vec3 pink = vec3(1.0, 0.18, 0.62);
    vec3 cyan = vec3(0.10, 0.90, 1.0);
    float sx = p.x + (u_pointer.x - 0.5) * 0.06;
    vec3 col;
    if (p.y > hz) {
        float y = (p.y - hz) / (0.5 - hz);
        col = mix(vec3(0.90, 0.16, 0.50), vec3(0.20, 0.04, 0.36), smoothstep(0.0, 0.5, y));
        col = mix(col, vec3(0.025, 0.010, 0.070), smoothstep(0.4, 1.0, y));
        vec2 sg = vec2(p.x * 60.0, p.y * 60.0 / (1.0 + hyper * 6.0));
        vec2 sid = floor(sg);
        vec2 sh = hash22(sid + 5.0);
        float sd = length(sg - sid - 0.5);
        col += vec3(0.9, 0.85, 1.0) * step(0.94, sh.x) * exp(-sd * sd * 10.0) * smoothstep(0.1, 0.6, y) * (0.6 + 0.4 * sin(t * 3.0 + sh.y * 30.0));
        vec2 sc = vec2(0.0, hz + 0.17);
        float r = 0.2 + 0.006 * kick;
        float d = length(vec2(sx, p.y) - sc);
        float cutZone = clamp((sc.y + 0.04 - p.y) / r, 0.0, 1.0);
        float band = fract((sc.y - p.y) * 24.0 + t * 0.7);
        float mask = smoothstep(r, r - 0.004, d) * (1.0 - step(band, cutZone * 0.7) * step(p.y, sc.y + 0.04));
        vec3 sun = mix(vec3(1.0, 0.86, 0.28), vec3(1.0, 0.22, 0.58), smoothstep(sc.y + r, sc.y - r * 0.8, p.y));
        col = mix(col, sun, mask);
        col += pink * exp(-max(d - r, 0.0) * 7.0) * (0.28 + 0.2 * kick);
        float side = smoothstep(0.08, 0.55, abs(sx));
        float mh = hz + 0.015 + (0.05 + 0.13 * pow(ridged(sx * 2.6 + 11.0), 2.0)) * side;
        if (p.y < mh) {
            col = vec3(0.045, 0.010, 0.090);
            col += cyan * exp(-(mh - p.y) * 110.0) * 0.9 + pink * exp(-(mh - p.y) * 18.0) * 0.12;
        }
    } else {
        float dy = max(hz - p.y, 0.0006);
        float depth = 0.14 / dy;
        float fx = abs(fract(sx * depth * 4.0) - 0.5);
        float fz = abs(fract((depth + travel) * 2.0) - 0.5);
        float lineX = 1.0 - smoothstep(0.0, 0.0096 * depth + 0.012, 0.5 - fx);
        float lineZ = (1.0 - smoothstep(0.0, 0.034 * depth * depth + 0.012, 0.5 - fz)) * smoothstep(3.6, 1.2, depth);
        float fade = smoothstep(10.0, 2.5, depth);
        vec3 grid = mix(cyan, pink, smoothstep(0.8, 6.0, depth));
        col = vec3(0.030, 0.004, 0.060);
        col += grid * max(lineX, lineZ) * fade * (0.85 + 0.6 * kick + hyper);
        col += pink * exp(-dy * 16.0) * 0.55;
        col += vec3(1.0, 0.5, 0.4) * exp(-abs(sx) * 7.0) * exp(-dy * 9.0) * 0.25;
    }
    return col;
}

void main() {
    vec2 p = (gl_FragCoord.xy - 0.5 * u_res) / u_res.y;
    vec2 uv = gl_FragCoord.xy / u_res;
    float t = u_time;
    float kick = exp(-fract(t * 118.0 / 60.0) * 5.0);
    float hs = floor(t / 16.0);
    float hx = clamp((t - hs * 16.0) / 2.6, 0.0, 1.0);
    float hyperOn = step(0.45, hash12(vec2(hs, 3.0)));
    float hyper = hyperOn * sin(3.14159 * hx);
    // The surge covers a whole number of grid periods, so the floor never jumps afterwards.
    float travel = t * 1.6 + hyperOn * 24.0 * (0.5 - 0.5 * cos(3.14159 * hx));
    float slot = floor(t / 2.7);
    float local = t - slot * 2.7;
    float burst = step(0.62, hash12(vec2(slot, 9.0))) * smoothstep(0.0, 0.04, local) * (1.0 - smoothstep(0.22, 0.4, local));
    float g = max(burst, exp(-u_pulse * 4.0));
    vec3 col;
    if (g > 0.02) {
        float r = hash12(vec2(floor(uv.y * 28.0), floor(t * 24.0)));
        vec2 q = p + vec2((r - 0.5) * 0.22 * g * step(0.6, r), 0.0);
        float shift = 0.014 * g;
        col.r = neonScene(q + vec2(shift, 0.0), t, kick, travel, hyper).r;
        col.g = neonScene(q, t, kick, travel, hyper).g;
        col.b = neonScene(q - vec2(shift, 0.0), t, kick, travel, hyper).b;
        col = mix(col, col.gbr, step(0.93, r) * g);
    } else {
        col = neonScene(p, t, kick, travel, hyper);
    }
    vec2 pulseP = (u_pulseAt - 0.5) * vec2(u_res.x / u_res.y, 1.0);
    float ring = abs(length(p - pulseP) - u_pulse * 0.9);
    col += vec3(0.1, 0.9, 1.0) * exp(-ring * 70.0) * exp(-u_pulse * 1.6) * 0.9;
    col *= 0.9 + 0.1 * sin(gl_FragCoord.y * 2.2);
    col += vec3(0.6, 0.5, 1.0) * exp(-abs(uv.y - fract(t * 0.13)) * 260.0) * 0.08;
    col *= smoothstep(1.35, 0.35, length(p * vec2(0.82, 1.05)));
    col = col / (1.0 + col * 0.35);
    gl_FragColor = vec4(finish(col), 1.0);
}
`;

// Wild: a sixfold kaleidoscope over nested rings of neon that repeat into themselves (the
// ring technique follows kishimisu's shader-art tutorial, the palette Inigo Quilez's cosine
// palettes); colour runs with the beat, the pattern twists into a spiral and a click sends a
// wave through it.
const FRACTAL_TRIP = `
vec3 pal(float x) { return 0.5 + 0.5 * cos(TAU * (x + vec3(0.263, 0.416, 0.557))); }

void main() {
    float aspect = u_res.x / u_res.y;
    vec2 p = (gl_FragCoord.xy - 0.5 * u_res) / u_res.y;
    float t = u_time;
    float kick = exp(-fract(t * 96.0 / 60.0) * 4.0);
    vec2 drift = (u_pointer - 0.5) * vec2(aspect, 1.0) * 0.22;
    p -= drift;
    vec2 pulseP = (u_pulseAt - 0.5) * vec2(aspect, 1.0) - drift;
    float wave = exp(-abs(length(p - pulseP) - u_pulse * 0.8) * 18.0) * exp(-u_pulse * 1.2);
    p += normalize(p - pulseP + 1e-4) * wave * 0.05;
    float r = length(p);
    float a = atan(p.y, p.x) + t * 0.06 + 0.5 * sin(t * 0.21) * r;
    float seg = TAU / 6.0;
    a = abs(mod(a, seg) - seg * 0.5);
    vec2 uv = vec2(cos(a), sin(a)) * r;
    vec2 uv0 = uv;
    float zoom = 1.45 + 0.2 * sin(t * 0.11) + 0.03 * kick;
    vec3 col = vec3(0.0);
    for (int i = 0; i < 4; i++) {
        uv = fract(uv * zoom) - 0.5;
        float d = length(uv) * exp(-length(uv0));
        vec3 c = pal(length(uv0) + float(i) * 0.4 + t * 0.12 + wave * 0.5);
        d = abs(sin(d * 8.0 + t * 0.9) / 8.0);
        col += c * pow(0.011 / max(d, 0.0005), 1.25);
    }
    col *= 0.5 + 0.2 * kick;
    col *= smoothstep(1.5, 0.2, r);
    col += vec3(1.0, 0.85, 1.0) * wave * 0.3;
    col = col / (1.0 + col);
    gl_FragColor = vec4(finish(col), 1.0);
}
`;

const WALLPAPERS = Object.freeze({
  silk_flow: { fps: 30, scale: 0.4, minScale: 0.25, still: 42, shader: SILK_FLOW },
  firefly_dusk: { fps: 30, scale: 0.5, minScale: 0.3, still: 17, shader: FIREFLY_DUSK },
  neon_overdrive: { fps: 60, scale: 0.5, minScale: 0.3, still: 5.3, shader: NEON_OVERDRIVE },
  fractal_trip: { fps: 60, scale: 0.5, minScale: 0.3, still: 23, shader: FRACTAL_TRIP },
});

export const LIVE_WALLPAPER_IDS = Object.freeze(Object.keys(WALLPAPERS));

let hostEl = null;
let canvas = null;
let gl = null;
let buffer = null;
const programs = new Map();
const failed = new Set();
let activeId = "";
let spec = null;
let program = null;
let state = LIVE_WALLPAPER_MARKERS.off;
let raf = 0;
let running = false;
let lastTick = 0;
let lastDraw = 0;
let clock = 0;
let scale = 1;
let frames = 0;
let drawsInWindow = 0;
let windowStart = 0;
let occluded = false;
let lastOcclusionCheck = 0;
let syncQueued = false;
let resizeTimer = 0;
const pointer = [0.5, 0.5];
const pointerTarget = [0.5, 0.5];
const pulseAt = [0.5, 0.5];
let pulseStart = -1e4;

function motionAllowed() {
  if (document.body && document.body.dataset.animations === "false") return false;
  try {
    return !(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  } catch {
    return true;
  }
}

function setState(next) {
  state = next;
  if (hostEl) hostEl.dataset.liveState = next;
}

function ensureHost() {
  if (hostEl && hostEl.isConnected) return hostEl;
  let host = document.getElementById(HOST_ID);
  if (!host) {
    host = document.createElement("div");
    host.id = HOST_ID;
    host.className = "vd-wallpaper-live";
    host.setAttribute("aria-hidden", "true");
    host.hidden = true;
    document.body.insertBefore(host, document.body.firstChild);
  }
  hostEl = host;
  return host;
}

function initBuffer() {
  buffer = gl.createBuffer();
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW);
}

function onContextLost(event) {
  event.preventDefault();
  stop();
  programs.clear();
  program = null;
  setState(LIVE_WALLPAPER_MARKERS.unavailable);
}

function onContextRestored() {
  initBuffer();
  sync();
}

function ensureContext(host) {
  if (gl && canvas && canvas.isConnected) return !gl.isContextLost();
  canvas = document.createElement("canvas");
  canvas.className = "vd-wallpaper-live-canvas";
  canvas.addEventListener("webglcontextlost", onContextLost, false);
  canvas.addEventListener("webglcontextrestored", onContextRestored, false);
  host.appendChild(canvas);
  try {
    gl = canvas.getContext("webgl", {
      alpha: false,
      antialias: false,
      depth: false,
      stencil: false,
      premultipliedAlpha: false,
      preserveDrawingBuffer: false,
      powerPreference: "low-power",
    });
  } catch {
    gl = null;
  }
  if (!gl) {
    canvas.remove();
    canvas = null;
    return false;
  }
  initBuffer();
  return true;
}

function teardown() {
  stop();
  programs.clear();
  program = null;
  if (canvas) {
    canvas.removeEventListener("webglcontextlost", onContextLost, false);
    canvas.removeEventListener("webglcontextrestored", onContextRestored, false);
    const lose = gl && gl.getExtension("WEBGL_lose_context");
    if (lose) lose.loseContext();
    canvas.remove();
  }
  canvas = null;
  gl = null;
  buffer = null;
  activeId = "";
  spec = null;
  if (hostEl) hostEl.hidden = true;
}

function compileShader(type, source) {
  const shader = gl.createShader(type);
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    if (!gl.isContextLost()) console.warn("[live-wallpaper] shader failed:", gl.getShaderInfoLog(shader));
    gl.deleteShader(shader);
    return null;
  }
  return shader;
}

function compile(id) {
  if (programs.has(id)) return programs.get(id);
  const vs = compileShader(gl.VERTEX_SHADER, VERTEX);
  const fs = compileShader(gl.FRAGMENT_SHADER, COMMON + WALLPAPERS[id].shader);
  if (!vs || !fs) return null;
  const prog = gl.createProgram();
  gl.attachShader(prog, vs);
  gl.attachShader(prog, fs);
  gl.bindAttribLocation(prog, 0, "a_pos");
  gl.linkProgram(prog);
  gl.deleteShader(vs);
  gl.deleteShader(fs);
  if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
    if (!gl.isContextLost()) console.warn("[live-wallpaper] link failed:", gl.getProgramInfoLog(prog));
    gl.deleteProgram(prog);
    return null;
  }
  const uniforms = {};
  for (const name of ["u_res", "u_time", "u_pointer", "u_pulse", "u_pulseAt"]) uniforms[name] = gl.getUniformLocation(prog, name);
  const entry = { prog, uniforms };
  programs.set(id, entry);
  return entry;
}

function draw(time) {
  if (!gl || !program || gl.isContextLost()) return false;
  gl.viewport(0, 0, canvas.width, canvas.height);
  gl.useProgram(program.prog);
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer);
  gl.enableVertexAttribArray(0);
  gl.vertexAttribPointer(0, 2, gl.FLOAT, false, 0, 0);
  const u = program.uniforms;
  gl.uniform2f(u.u_res, canvas.width, canvas.height);
  gl.uniform1f(u.u_time, time % TIME_WRAP);
  gl.uniform2f(u.u_pointer, pointer[0], pointer[1]);
  gl.uniform1f(u.u_pulse, Math.min(60, Math.max(0, clock - pulseStart)));
  gl.uniform2f(u.u_pulseAt, pulseAt[0], pulseAt[1]);
  gl.drawArrays(gl.TRIANGLES, 0, 3);
  frames += 1;
  if (!canvas.classList.contains("is-ready")) canvas.classList.add("is-ready");
  return true;
}

function currentTime() {
  return running ? clock : spec.still;
}

// Resizing clears the drawing buffer, so the new size is painted in the same task.
function resize() {
  if (!canvas || !spec) return;
  const w = Math.max(1, window.innerWidth);
  const h = Math.max(1, window.innerHeight);
  let s = scale * Math.min(window.devicePixelRatio || 1, 2);
  const pixels = w * h * s * s;
  if (pixels > MAX_PIXELS) s *= Math.sqrt(MAX_PIXELS / pixels);
  const bw = Math.max(16, Math.round(w * s));
  const bh = Math.max(16, Math.round(h * s));
  if (canvas.width === bw && canvas.height === bh) return;
  canvas.width = bw;
  canvas.height = bh;
  if (program) draw(currentTime());
}

function desktopCovered() {
  const saver = document.getElementById("vd-screensaver");
  if (saver && saver.dataset.state !== "stopping" && saver.getClientRects().length) return true;
  const area = window.innerWidth * window.innerHeight;
  for (const win of document.querySelectorAll(".vd-window.maximized")) {
    if (win.classList.contains("vd-space-hidden") || win.classList.contains("minimized")) continue;
    const rect = win.getBoundingClientRect();
    if (rect.width * rect.height < area * 0.82) continue;
    const style = getComputedStyle(win);
    if (style.display === "none" || style.visibility === "hidden" || parseFloat(style.opacity) < 0.99) continue;
    return true;
  }
  return false;
}

// Fewer frames than planned mean the GPU is struggling: render fewer pixels.
function adapt(now) {
  const span = now - windowStart;
  if (span < ADAPT_WINDOW_MS) return;
  const achieved = (drawsInWindow * 1000) / span;
  windowStart = now;
  drawsInWindow = 0;
  if (achieved < spec.fps * 0.72 && scale > spec.minScale + 0.001) {
    scale = Math.max(spec.minScale, scale * 0.82);
    resize();
  }
}

function tick(now) {
  raf = 0;
  if (!running) return;
  raf = requestAnimationFrame(tick);
  if (!lastTick) {
    lastTick = now;
    lastDraw = now - 1000;
    windowStart = now;
  }
  const dt = Math.min(0.1, (now - lastTick) / 1000);
  lastTick = now;
  if (now - lastOcclusionCheck > OCCLUSION_CHECK_MS) {
    lastOcclusionCheck = now;
    const covered = desktopCovered();
    if (covered !== occluded) {
      occluded = covered;
      setState(covered ? LIVE_WALLPAPER_MARKERS.paused : LIVE_WALLPAPER_MARKERS.running);
    }
  }
  if (occluded) {
    windowStart = now;
    drawsInWindow = 0;
    return;
  }
  clock += dt;
  const k = 1 - Math.exp(-dt * 2.5);
  pointer[0] += (pointerTarget[0] - pointer[0]) * k;
  pointer[1] += (pointerTarget[1] - pointer[1]) * k;
  if (now - lastDraw < 1000 / spec.fps - 2) return;
  lastDraw = now;
  if (draw(clock)) drawsInWindow += 1;
  adapt(now);
}

function start() {
  if (running) return;
  running = true;
  occluded = false;
  lastTick = 0;
  lastOcclusionCheck = 0;
  setState(LIVE_WALLPAPER_MARKERS.running);
  raf = requestAnimationFrame(tick);
}

function stop() {
  running = false;
  if (raf) cancelAnimationFrame(raf);
  raf = 0;
}

function sync() {
  syncQueued = false;
  const id = (document.body && document.body.dataset.wallpaper) || "";
  if (!Object.prototype.hasOwnProperty.call(WALLPAPERS, id)) {
    if (activeId || canvas) teardown();
    setState(LIVE_WALLPAPER_MARKERS.off);
    return;
  }
  const host = ensureHost();
  if (id !== activeId) {
    activeId = id;
    spec = WALLPAPERS[id];
    scale = spec.scale;
    clock = spec.still;
    program = null;
  }
  if (failed.has(id) || !ensureContext(host)) {
    stop();
    host.hidden = true;
    setState(LIVE_WALLPAPER_MARKERS.unavailable);
    return;
  }
  program = compile(id);
  if (!program) {
    failed.add(id);
    teardown();
    setState(LIVE_WALLPAPER_MARKERS.unavailable);
    return;
  }
  host.hidden = false;
  resize();
  if (document.hidden) {
    stop();
    setState(LIVE_WALLPAPER_MARKERS.paused);
  } else if (motionAllowed()) {
    start();
  } else {
    stop();
    draw(spec.still);
    setState(LIVE_WALLPAPER_MARKERS.still);
  }
}

function scheduleSync() {
  if (syncQueued) return;
  syncQueued = true;
  queueMicrotask(sync);
}

function onPointerMove(event) {
  if (!activeId) return;
  pointerTarget[0] = event.clientX / Math.max(1, window.innerWidth);
  pointerTarget[1] = 1 - event.clientY / Math.max(1, window.innerHeight);
}

// A press on the empty desktop sends a pulse through the wild wallpapers.
function onPointerDown(event) {
  if (!running || !(event.target instanceof Element)) return;
  const target = event.target;
  if (target.closest(".vd-window, .vd-icon, .vd-widget, .vd-taskbar, .vd-start-menu, .vd-context-menu, .vd-global-bar, button, a, input, textarea, select")) return;
  if (target !== document.body && !target.closest("#vd-workspace")) return;
  pulseAt[0] = event.clientX / Math.max(1, window.innerWidth);
  pulseAt[1] = 1 - event.clientY / Math.max(1, window.innerHeight);
  pulseStart = clock;
}

function onResize() {
  window.clearTimeout(resizeTimer);
  resizeTimer = window.setTimeout(resize, 150);
}

// Debug and test hook: state plus a pixel sample read right after drawing.
function inspect() {
  return {
    id: activeId,
    state,
    frames,
    scale,
    fps: spec ? spec.fps : 0,
    width: canvas ? canvas.width : 0,
    height: canvas ? canvas.height : 0,
    occluded,
  };
}

function sample() {
  if (!spec || !draw(currentTime())) return null;
  const px = new Uint8Array(4);
  const lum = [];
  const mean = [0, 0, 0];
  for (let j = 0; j < 8; j++) {
    for (let i = 0; i < 8; i++) {
      gl.readPixels(Math.floor(((i + 0.5) * canvas.width) / 8), Math.floor(((j + 0.5) * canvas.height) / 8), 1, 1, gl.RGBA, gl.UNSIGNED_BYTE, px);
      mean[0] += px[0] / 64;
      mean[1] += px[1] / 64;
      mean[2] += px[2] / 64;
      lum.push(0.2126 * px[0] + 0.7152 * px[1] + 0.0722 * px[2]);
    }
  }
  const avg = lum.reduce((a, b) => a + b, 0) / lum.length;
  const spread = Math.sqrt(lum.reduce((a, b) => a + (b - avg) * (b - avg), 0) / lum.length);
  return { mean: mean.map(Math.round), luminance: Math.round(avg), spread: Math.round(spread) };
}

function pulse(x, y) {
  pulseAt[0] = x;
  pulseAt[1] = y;
  pulseStart = clock;
}

function seek(seconds) {
  clock = Math.max(0, Number(seconds) || 0);
  return draw(clock);
}

function init() {
  ensureHost();
  sync();
  new MutationObserver(scheduleSync).observe(document.body, { attributes: true, attributeFilter: ["data-wallpaper", "data-animations"] });
  const motionQuery = window.matchMedia ? window.matchMedia("(prefers-reduced-motion: reduce)") : null;
  if (motionQuery && typeof motionQuery.addEventListener === "function") motionQuery.addEventListener("change", scheduleSync);
  document.addEventListener("visibilitychange", scheduleSync);
  document.addEventListener("pointermove", onPointerMove, { passive: true });
  document.addEventListener("pointerdown", onPointerDown, { passive: true });
  window.addEventListener("resize", onResize, { passive: true });
  window.addEventListener("pagehide", teardown);
  window.addEventListener("pageshow", scheduleSync);
}

window.AuraLiveWallpaper = Object.freeze({ inspect, sample, pulse, seek, sync, ids: LIVE_WALLPAPER_IDS });

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", init, { once: true });
} else {
  init();
}

export default { sync, inspect, sample };
