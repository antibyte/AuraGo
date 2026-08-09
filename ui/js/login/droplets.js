/**
 * Login-page CanvasUI Droplets overlay.
 * Decorative only: fails closed without touching auth or the Three.js/CSS background switch.
 */
import { createDroplets, supportsHtmlInCanvas } from "/js/vendor/canvasui/droplets.js";

const HOST_ID = "login-bg-host";
const CONTENT_ID = "login-bg-content";
const SOURCE_ID = "droplets-source";
const OUTPUT_ID = "droplets-output";

/** Markers consumed by static UI contract tests. */
export const LOGIN_DROPLETS_MARKERS = Object.freeze({
  gracefulWebGL2Failure: "login-droplets:webgl2-unavailable",
  supportsHtmlInCanvasFallback: "login-droplets:html-in-canvas-fallback",
  themeUpdates: "login-droplets:theme-update",
  destroyCleanup: "login-droplets:destroy",
  reducedMotionSkip: "login-droplets:reduced-motion-skip",
});

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

function tintForTheme(theme) {
  // Restrained teal wash matching login accent; slightly stronger on light theme for contrast.
  if (theme === "light") {
    return { tint: [0.18, 0.55, 0.5], tintStrength: 0.12 };
  }
  return { tint: [0.18, 0.83, 0.75], tintStrength: 0.08 };
}

function loginDropletsOptions(theme) {
  const tint = tintForTheme(theme);
  return {
    intensity: 0.42,
    speed: 0.85,
    scale: 0.45,
    dropWidth: 0.95,
    dropLength: 1,
    refraction: 0.12,
    blur: 0,
    vignette: 0,
    fallSpeed: 0.9,
    wiggle: 0.85,
    staticDrops: 0.18,
    interactive: true,
    interactionRadius: 0.28,
    interactionStrength: 0.45,
    interactionDistortion: 2,
    tint: tint.tint,
    tintStrength: tint.tintStrength,
  };
}

function placeContentForCapture(host, source, content, nativeCapture) {
  if (!nativeCapture) {
    if (content.parentElement !== host) {
      host.insertBefore(content, source);
    }
    host.classList.remove("is-native-capture");
    source.removeAttribute("layoutsubtree");
    return;
  }
  source.setAttribute("layoutsubtree", "true");
  if (content.parentElement !== source) {
    source.appendChild(content);
  }
  host.classList.add("is-native-capture");
}

function restoreContent(host, source, content) {
  if (!host || !content) return;
  if (content.parentElement !== host) {
    const anchor = source && source.parentElement === host ? source : host.firstChild;
    if (anchor) host.insertBefore(content, anchor);
    else host.appendChild(content);
  }
  host.classList.remove("is-native-capture");
  if (source) source.removeAttribute("layoutsubtree");
}

function initLoginDroplets() {
  const host = document.getElementById(HOST_ID);
  const content = document.getElementById(CONTENT_ID);
  const source = document.getElementById(SOURCE_ID);
  const output = document.getElementById(OUTPUT_ID);
  if (!host || !content || !source || !output) return null;

  // Ensure the rain canvas paints last among body children (under card via z-index).
  document.body.appendChild(output);

  output.setAttribute("aria-hidden", "true");
  source.setAttribute("aria-hidden", "true");
  host.setAttribute("aria-hidden", "true");

  if (prefersReducedMotion()) {
    host.dataset.dropletsState = LOGIN_DROPLETS_MARKERS.reducedMotionSkip;
    return null;
  }
  if (!hasRequiredAPIs()) {
    host.dataset.dropletsState = LOGIN_DROPLETS_MARKERS.gracefulWebGL2Failure;
    return null;
  }
  if (!webgl2Available()) {
    host.dataset.dropletsState = LOGIN_DROPLETS_MARKERS.gracefulWebGL2Failure;
    return null;
  }

  const nativeCapture = supportsHtmlInCanvas();
  if (!nativeCapture) {
    host.dataset.dropletsState = LOGIN_DROPLETS_MARKERS.supportsHtmlInCanvasFallback;
  }

  placeContentForCapture(host, source, content, nativeCapture);

  let instance = null;
  try {
    instance = createDroplets(
      { source, content, output },
      loginDropletsOptions(currentTheme())
    );
  } catch {
    instance = null;
  }

  if (!instance) {
    restoreContent(host, source, content);
    host.dataset.dropletsState = LOGIN_DROPLETS_MARKERS.gracefulWebGL2Failure;
    output.classList.add("is-hidden");
    output.classList.remove("is-active");
    return null;
  }

  output.classList.remove("is-hidden");
  output.classList.add("is-active");
  if (!host.dataset.dropletsState) {
    host.dataset.dropletsState = nativeCapture
      ? "login-droplets:native-capture"
      : LOGIN_DROPLETS_MARKERS.supportsHtmlInCanvasFallback;
  }

  const onThemeChange = (event) => {
    const theme = (event && event.detail && event.detail.theme) || currentTheme();
    host.dataset.dropletsState = LOGIN_DROPLETS_MARKERS.themeUpdates;
    try {
      instance.setOptions({ ...tintForTheme(theme) });
    } catch {
      /* ignore live option failures */
    }
  };
  window.addEventListener("aurago:themechange", onThemeChange);

  const destroy = () => {
    host.dataset.dropletsState = LOGIN_DROPLETS_MARKERS.destroyCleanup;
    window.removeEventListener("aurago:themechange", onThemeChange);
    try {
      instance.destroy();
    } catch {
      /* ignore */
    }
    restoreContent(host, source, content);
    output.classList.add("is-hidden");
    output.classList.remove("is-active");
  };

  window.addEventListener("pagehide", destroy, { once: true });
  window.addEventListener("beforeunload", destroy, { once: true });

  return { destroy, instance };
}

const loginDroplets = initLoginDroplets();
export default loginDroplets;
