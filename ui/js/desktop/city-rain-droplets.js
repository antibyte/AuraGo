/**
 * Virtual Desktop City Rain wallpaper effect.
 * CanvasUI Droplets only while appearance.wallpaper === city_rain,
 * painted strictly behind icons/widgets/windows.
 * Feeds the city_rain wallpaper bitmap so drops refract real background colors
 * (original CanvasUI uHasContent path) instead of gray procedural glass.
 */
import { createDroplets } from "/js/vendor/canvasui/droplets.js";

const WALLPAPER_KEY = "city_rain";
const WALLPAPER_URL = "/img/wallpapers/city_rain.jpg";
const HOST_ID = "vd-wallpaper-fx";
const CONTENT_ID = "vd-droplets-content";
const SOURCE_ID = "vd-droplets-source";
const OUTPUT_ID = "vd-droplets-output";

export const DESKTOP_DROPLETS_MARKERS = Object.freeze({
  wallpaperGate: "city_rain",
  backgroundOnly: "vd-wallpaper-fx",
  contentBitmap: "wallpaper-bitmap",
  gracefulWebGL2Failure: "desktop-droplets:webgl2-unavailable",
  reducedMotionSkip: "desktop-droplets:reduced-motion-skip",
  active: "desktop-droplets:active",
  destroyed: "desktop-droplets:destroyed",
});

let instance = null;
let hostEl = null;
let observer = null;
let motionQuery = null;
let wallpaperImage = null;
let wallpaperLoad = null;
let startToken = 0;

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
    workspace.insertBefore(host, workspace.firstChild);
  }
  if (!host) return null;
  hostEl = host;
  return host;
}

function rainOptions() {
  return {
    intensity: 0.62,
    speed: 0.95,
    scale: 0.4,
    dropWidth: 0.95,
    dropLength: 1.05,
    refraction: 0.14,
    blur: 0.35,
    vignette: 0.15,
    fallSpeed: 1,
    wiggle: 0.9,
    staticDrops: 0.28,
    interactive: false,
    interactionRadius: 0,
    interactionStrength: 0,
    interactionDistortion: 0,
    // No gray glass tint — wallpaper colors carry the look.
    tint: [1, 1, 1],
    tintStrength: 0,
  };
}

function loadWallpaperImage() {
  if (wallpaperImage && wallpaperImage.complete && wallpaperImage.naturalWidth > 0) {
    return Promise.resolve(wallpaperImage);
  }
  if (wallpaperLoad) return wallpaperLoad;
  wallpaperLoad = new Promise((resolve, reject) => {
    const img = new Image();
    img.decoding = "async";
    img.onload = () => {
      wallpaperImage = img;
      resolve(img);
    };
    img.onerror = () => {
      wallpaperLoad = null;
      reject(new Error("city_rain wallpaper failed to load"));
    };
    img.src = WALLPAPER_URL;
  });
  return wallpaperLoad;
}

function destroyDroplets(reason) {
  startToken += 1;
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

async function startDroplets() {
  if (instance) return;
  const token = ++startToken;

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

  // Never capture desktop HTML — only the wallpaper bitmap for refraction.
  source.removeAttribute("layoutsubtree");
  host.classList.remove("is-native-capture");

  let bitmap;
  try {
    bitmap = await loadWallpaperImage();
  } catch {
    if (token !== startToken) return;
    host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.gracefulWebGL2Failure;
    return;
  }
  if (token !== startToken || !isCityRainWallpaper()) return;

  host.hidden = false;
  host.classList.add("is-active");
  output.classList.remove("is-hidden");
  output.classList.add("is-active");

  void host.offsetWidth;
  const rect = host.getBoundingClientRect();
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  output.style.width = "100%";
  output.style.height = "100%";
  output.width = Math.max(1, Math.round(rect.width * dpr));
  output.height = Math.max(1, Math.round(rect.height * dpr));
  source.style.width = "100%";
  source.style.height = "100%";

  try {
    instance = createDroplets(
      { source, content, output, bitmap },
      rainOptions()
    );
  } catch {
    instance = null;
  }

  if (!instance) {
    destroyDroplets(DESKTOP_DROPLETS_MARKERS.gracefulWebGL2Failure);
    return;
  }

  host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.active;
  host.dataset.dropletsContent = DESKTOP_DROPLETS_MARKERS.contentBitmap;
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
