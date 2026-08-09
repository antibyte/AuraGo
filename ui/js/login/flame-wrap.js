/**
 * Login-card CanvasUI Flame Wrap.
 * Decorative border fire around the login modal; fails closed without blocking auth.
 */
import { createFlameWrap, supportsHtmlInCanvas } from "/js/vendor/canvasui/flame-wrap.js";

const WRAP_ID = "login-card-wrap";
const CARD_ID = "main-content";
const SOURCE_ID = "flame-source";
const OUTPUT_ID = "flame-output";

/** Markers consumed by static UI contract tests. */
export const LOGIN_FLAME_MARKERS = Object.freeze({
  gracefulWebGL2Failure: "login-flame:webgl2-unavailable",
  supportsHtmlInCanvasFallback: "login-flame:html-in-canvas-fallback",
  themeUpdates: "login-flame:theme-update",
  destroyCleanup: "login-flame:destroy",
  reducedMotionSkip: "login-flame:reduced-motion-skip",
});

const FLAME_HEIGHT = 96;
const FLAME_SPREAD = 14;

function prefersReducedMotion() {
  try {
    return Boolean(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  } catch {
    return false;
  }
}

function hasRequiredAPIs() {
  return Boolean(
    typeof window !== "undefined" &&
      window.requestAnimationFrame &&
      typeof ResizeObserver === "function" &&
      typeof IntersectionObserver === "function" &&
      window.matchMedia
  );
}

function webgl2Available() {
  try {
    const probe = document.createElement("canvas");
    const gl = probe.getContext("webgl2", { alpha: true });
    if (!gl || gl.isContextLost()) return false;
    const ext = gl.getExtension("WEBGL_lose_context");
    if (ext) ext.loseContext();
    return true;
  } catch {
    return false;
  }
}

function currentTheme() {
  const root = document.documentElement;
  const attr = root && root.getAttribute("data-theme");
  if (attr === "light" || attr === "dark") return attr;
  if (typeof window.getCurrentTheme === "function") {
    try {
      const theme = window.getCurrentTheme();
      if (theme === "light" || theme === "dark") return theme;
    } catch {
      /* ignore */
    }
  }
  return "dark";
}

function colorForTheme(theme) {
  // AuraGo teal fire (RGB 0-1).
  if (theme === "light") {
    return [0.08, 0.55, 0.5];
  }
  return [0.18, 0.83, 0.75];
}

function readCardRadius(card) {
  try {
    const raw = getComputedStyle(card).borderRadius || "";
    const match = String(raw).match(/([\d.]+)px/);
    if (match) return Math.max(8, Math.min(48, parseFloat(match[1])));
  } catch {
    /* ignore */
  }
  return 16;
}

function applyOutputGeometry(wrap, output) {
  const reach = Math.round(Math.max(FLAME_HEIGHT, 24) * 1.5) + 40;
  const glow = Math.round(Math.max(FLAME_SPREAD, 8) * 3) + 16;
  wrap.style.setProperty("--login-flame-reach", `${reach}px`);
  wrap.style.setProperty("--login-flame-glow", `${glow}px`);
  output.style.top = `-${reach}px`;
  output.style.right = `-${glow}px`;
  output.style.bottom = `-${glow}px`;
  output.style.left = `-${glow}px`;
  output.style.width = `calc(100% + ${glow * 2}px)`;
  output.style.height = `calc(100% + ${reach + glow}px)`;
}

function placeContentForCapture(wrap, source, card, nativeCapture) {
  if (!nativeCapture) {
    if (card.parentElement !== wrap) {
      wrap.insertBefore(card, source.nextSibling || null);
    }
    wrap.classList.remove("is-native-capture");
    source.removeAttribute("layoutsubtree");
    return;
  }
  source.setAttribute("layoutsubtree", "true");
  if (card.parentElement !== source) {
    source.appendChild(card);
  }
  wrap.classList.add("is-native-capture");
}

function restoreContent(wrap, source, card) {
  if (!wrap || !card) return;
  if (card.parentElement !== wrap) {
    const anchor = source && source.parentElement === wrap ? source.nextSibling : wrap.firstChild;
    if (anchor) wrap.insertBefore(card, anchor);
    else wrap.appendChild(card);
  }
  wrap.classList.remove("is-native-capture");
  if (source) source.removeAttribute("layoutsubtree");
}

function loginFlameOptions(theme, card) {
  return {
    color: colorForTheme(theme),
    intensity: 0.55,
    height: FLAME_HEIGHT,
    spread: FLAME_SPREAD,
    radius: readCardRadius(card),
    speed: 0.35,
    scale: 0.7,
    turbulence: 0.45,
    turbulenceScale: 0.55,
    turbulenceReach: 18,
    sparks: 1.2,
    sparkSize: 0.4,
    sparkDensity: 0.9,
    sparkSpeed: 0.9,
    rim: 2.2,
    melt: 3.5,
    distortion: 6,
    smoke: 1.1,
    ember: 1.6,
    scorch: 0.15,
  };
}

function initLoginFlameWrap() {
  const wrap = document.getElementById(WRAP_ID);
  const card = document.getElementById(CARD_ID);
  const source = document.getElementById(SOURCE_ID);
  const output = document.getElementById(OUTPUT_ID);
  if (!wrap || !card || !source || !output) return null;

  output.setAttribute("aria-hidden", "true");
  source.setAttribute("aria-hidden", "true");
  applyOutputGeometry(wrap, output);

  if (prefersReducedMotion()) {
    wrap.dataset.flameState = LOGIN_FLAME_MARKERS.reducedMotionSkip;
    return null;
  }
  if (!hasRequiredAPIs()) {
    wrap.dataset.flameState = LOGIN_FLAME_MARKERS.gracefulWebGL2Failure;
    return null;
  }
  if (!webgl2Available()) {
    wrap.dataset.flameState = LOGIN_FLAME_MARKERS.gracefulWebGL2Failure;
    return null;
  }

  const nativeCapture = supportsHtmlInCanvas();
  if (!nativeCapture) {
    wrap.dataset.flameState = LOGIN_FLAME_MARKERS.supportsHtmlInCanvasFallback;
  }

  placeContentForCapture(wrap, source, card, nativeCapture);

  let instance = null;
  try {
    instance = createFlameWrap(
      { source, content: card, output },
      loginFlameOptions(currentTheme(), card)
    );
  } catch {
    instance = null;
  }

  if (!instance) {
    restoreContent(wrap, source, card);
    wrap.dataset.flameState = LOGIN_FLAME_MARKERS.gracefulWebGL2Failure;
    output.classList.add("is-hidden");
    output.classList.remove("is-active");
    return null;
  }

  output.classList.remove("is-hidden");
  output.classList.add("is-active");
  if (!wrap.dataset.flameState) {
    wrap.dataset.flameState = nativeCapture
      ? "login-flame:native-capture"
      : LOGIN_FLAME_MARKERS.supportsHtmlInCanvasFallback;
  }

  const onThemeChange = (event) => {
    const theme = (event && event.detail && event.detail.theme) || currentTheme();
    wrap.dataset.flameState = LOGIN_FLAME_MARKERS.themeUpdates;
    try {
      instance.setOptions({
        color: colorForTheme(theme),
        radius: readCardRadius(card),
      });
    } catch {
      /* ignore */
    }
  };
  window.addEventListener("aurago:themechange", onThemeChange);

  const onResize = () => {
    applyOutputGeometry(wrap, output);
    try {
      instance.setOptions({ radius: readCardRadius(card) });
      instance.resize();
    } catch {
      /* ignore */
    }
  };
  window.addEventListener("resize", onResize, { passive: true });

  const destroy = () => {
    wrap.dataset.flameState = LOGIN_FLAME_MARKERS.destroyCleanup;
    window.removeEventListener("aurago:themechange", onThemeChange);
    window.removeEventListener("resize", onResize);
    try {
      instance.destroy();
    } catch {
      /* ignore */
    }
    restoreContent(wrap, source, card);
    output.classList.add("is-hidden");
    output.classList.remove("is-active");
  };

  window.addEventListener("pagehide", destroy, { once: true });
  window.addEventListener("beforeunload", destroy, { once: true });

  return { destroy, instance };
}

const loginFlameWrap = initLoginFlameWrap();
export default loginFlameWrap;
