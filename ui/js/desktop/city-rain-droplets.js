/**
 * Virtual Desktop City Rain wallpaper effect.
 * CanvasUI Droplets only while appearance.wallpaper === city_rain,
 * painted strictly behind icons/widgets/windows.
 */
import { createDroplets, supportsHtmlInCanvas } from "/js/vendor/canvasui/droplets.js";

const WALLPAPER_KEY = "city_rain";
const HOST_ID = "vd-wallpaper-fx";
const CONTENT_ID = "vd-droplets-content";
const SOURCE_ID = "vd-droplets-source";
const OUTPUT_ID = "vd-droplets-output";

export const DESKTOP_DROPLETS_MARKERS = Object.freeze({
  wallpaperGate: "city_rain",
  backgroundOnly: "vd-wallpaper-fx",
  gracefulWebGL2Failure: "desktop-droplets:webgl2-unavailable",
  reducedMotionSkip: "desktop-droplets:reduced-motion-skip",
  active: "desktop-droplets:active",
  destroyed: "desktop-droplets:destroyed",
});

let instance = null;
let hostEl = null;
let observer = null;
let motionQuery = null;

function prefersReducedMotion() {
  try {
    return Boolean(window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches);
  } catch {
    return false;
  }
}

function hasRequiredAPIs() {
  return Boolean(
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

function isCityRainWallpaper() {
  return document.body && document.body.dataset.wallpaper === WALLPAPER_KEY;
}

function ensureHost() {
  if (hostEl && hostEl.isConnected) return hostEl;
  const workspace = document.getElementById("vd-workspace");
  let host = document.getElementById(HOST_ID);
  if (!host && workspace) {
    host = document.createElement("div");
    host.id = HOST_ID;
    host.className = "vd-wallpaper-fx";
    host.setAttribute("aria-hidden", "true");
    host.innerHTML =
      `<div id="${CONTENT_ID}" class="vd-droplets-content" aria-hidden="true"></div>` +
      `<canvas id="${SOURCE_ID}" class="vd-droplets-source" aria-hidden="true"></canvas>` +
      `<canvas id="${OUTPUT_ID}" class="vd-droplets-output" aria-hidden="true"></canvas>`;
    // First child so paint order stays under icons/widgets/windows.
    workspace.insertBefore(host, workspace.firstChild);
  }
  if (!host) return null;
  hostEl = host;
  return host;
}

function rainOptions() {
  return {
    intensity: 0.55,
    speed: 0.95,
    scale: 0.42,
    dropWidth: 0.9,
    dropLength: 1.05,
    refraction: 0.1,
    blur: 0,
    vignette: 0,
    fallSpeed: 1,
    wiggle: 0.9,
    staticDrops: 0.22,
    interactive: false,
    interactionRadius: 0,
    interactionStrength: 0,
    interactionDistortion: 0,
    tint: [0.45, 0.72, 0.88],
    tintStrength: 0.12,
  };
}

function destroyDroplets(reason) {
  if (instance) {
    try {
      instance.destroy();
    } catch {
      /* ignore */
    }
    instance = null;
  }
  const host = hostEl || document.getElementById(HOST_ID);
  if (host) {
    host.dataset.dropletsState = reason || DESKTOP_DROPLETS_MARKERS.destroyed;
    host.classList.remove("is-active");
    host.hidden = true;
    const output = document.getElementById(OUTPUT_ID);
    if (output) {
      output.classList.add("is-hidden");
      output.classList.remove("is-active");
    }
  }
}

function startDroplets() {
  if (instance) return;
  if (prefersReducedMotion()) {
    const host = ensureHost();
    if (host) host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.reducedMotionSkip;
    return;
  }
  if (!hasRequiredAPIs() || !webgl2Available()) {
    const host = ensureHost();
    if (host) host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.gracefulWebGL2Failure;
    return;
  }

  const host = ensureHost();
  if (!host) return;
  const content = document.getElementById(CONTENT_ID);
  const source = document.getElementById(SOURCE_ID);
  const output = document.getElementById(OUTPUT_ID);
  if (!content || !source || !output) return;

  // Procedural rain only — never capture desktop HTML (keeps windows/icons clean).
  source.removeAttribute("layoutsubtree");
  host.classList.remove("is-native-capture");
  void supportsHtmlInCanvas;

  host.hidden = false;
  host.classList.add("is-active");
  output.classList.remove("is-hidden");
  output.classList.add("is-active");

  // Force measurable box before WebGL init.
  void host.offsetWidth;
  const rect = host.getBoundingClientRect();
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  output.style.width = "100%";
  output.style.height = "100%";
  output.width = Math.max(1, Math.round(rect.width * dpr));
  output.height = Math.max(1, Math.round(rect.height * dpr));

  try {
    instance = createDroplets({ source, content, output }, rainOptions());
  } catch {
    instance = null;
  }

  if (!instance) {
    destroyDroplets(DESKTOP_DROPLETS_MARKERS.gracefulWebGL2Failure);
    return;
  }

  host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.active;
  try {
    instance.resize();
  } catch {
    /* ignore */
  }
}

function syncWallpaperEffect() {
  if (isCityRainWallpaper()) startDroplets();
  else destroyDroplets(DESKTOP_DROPLETS_MARKERS.destroyed);
}

function initCityRainDroplets() {
  if (observer) return;
  syncWallpaperEffect();

  observer = new MutationObserver((records) => {
    for (const record of records) {
      if (record.type === "attributes" && record.attributeName === "data-wallpaper") {
        syncWallpaperEffect();
        break;
      }
    }
  });
  if (document.body) {
    observer.observe(document.body, { attributes: true, attributeFilter: ["data-wallpaper"] });
  }

  motionQuery = window.matchMedia ? window.matchMedia("(prefers-reduced-motion: reduce)") : null;
  if (motionQuery) {
    const onMotion = () => syncWallpaperEffect();
    if (typeof motionQuery.addEventListener === "function") motionQuery.addEventListener("change", onMotion);
    else if (typeof motionQuery.addListener === "function") motionQuery.addListener(onMotion);
  }

  window.addEventListener(
    "pagehide",
    () => {
      destroyDroplets(DESKTOP_DROPLETS_MARKERS.destroyed);
      if (observer) observer.disconnect();
      observer = null;
    },
    { once: true }
  );
}

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", initCityRainDroplets, { once: true });
} else {
  initCityRainDroplets();
}

export default { sync: syncWallpaperEffect, destroy: destroyDroplets };
