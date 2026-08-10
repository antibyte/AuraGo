/**
 * Virtual Desktop City Rain wallpaper effect.
 * CanvasUI Droplets only while appearance.wallpaper === city_rain,
 * painted strictly behind icons/widgets/windows.
 * Feeds the city_rain wallpaper bitmap so drops refract real background colors.
 *
 * Bootstrap sets data-wallpaper asynchronously after settings load — always
 * keep the CSS body wallpaper as fallback until the WebGL layer is ready.
 */
import { createDroplets } from "/js/vendor/canvasui/droplets.js";

const WALLPAPER_KEY = "city_rain";
const WALLPAPER_URL = "/img/wallpapers/city_rain.jpg";
const HOST_ID = "vd-wallpaper-fx";
const CONTENT_ID = "vd-droplets-content";
const SOURCE_ID = "vd-droplets-source";
const OUTPUT_ID = "vd-droplets-output";
const MIN_LAYER_SIZE = 32;
const BOOTSTRAP_RETRY_MS = [0, 50, 120, 300, 700, 1500, 3000];

export const DESKTOP_DROPLETS_MARKERS = Object.freeze({
  wallpaperGate: "city_rain",
  backgroundOnly: "vd-wallpaper-fx",
  contentBitmap: "wallpaper-bitmap",
  gracefulWebGL2Failure: "desktop-droplets:webgl2-unavailable",
  reducedMotionSkip: "desktop-droplets:reduced-motion-skip",
  active: "desktop-droplets:active",
  destroyed: "desktop-droplets:destroyed",
  waitingLayout: "desktop-droplets:waiting-layout",
});

let instance = null;
let hostEl = null;
let observer = null;
let layoutObserver = null;
let motionQuery = null;
let wallpaperImage = null;
let wallpaperLoad = null;
let startToken = 0;
let starting = false;
let retryTimers = [];
let initDone = false;

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

function clearRetryTimers() {
  for (const id of retryTimers) window.clearTimeout(id);
  retryTimers = [];
}

function scheduleBootstrapRetries() {
  clearRetryTimers();
  for (const delay of BOOTSTRAP_RETRY_MS) {
    retryTimers.push(
      window.setTimeout(() => {
        if (!instance && isCityRainWallpaper()) syncWallpaperEffect();
      }, delay)
    );
  }
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
    tint: [1, 1, 1],
    tintStrength: 0,
  };
}

/** Preload as early as possible so first open is not blank. */
function preloadWallpaperImage() {
  return loadWallpaperImage().catch(() => null);
}

function loadWallpaperImage() {
  if (wallpaperImage && wallpaperImage.complete && wallpaperImage.naturalWidth > 0) {
    return Promise.resolve(wallpaperImage);
  }
  if (wallpaperLoad) return wallpaperLoad;

  wallpaperLoad = new Promise((resolve, reject) => {
    const img = new Image();
    img.decoding = "async";
    // Help the browser treat this as a high-priority wallpaper asset.
    try {
      img.fetchPriority = "high";
    } catch {
      /* ignore */
    }

    const finishOk = () => {
      if (img.naturalWidth > 0) {
        wallpaperImage = img;
        resolve(img);
        return;
      }
      wallpaperLoad = null;
      reject(new Error("city_rain wallpaper empty"));
    };
    const finishErr = () => {
      wallpaperLoad = null;
      reject(new Error("city_rain wallpaper failed to load"));
    };

    img.onload = () => {
      if (typeof img.decode === "function") {
        img.decode().then(finishOk).catch(finishOk);
      } else {
        finishOk();
      }
    };
    img.onerror = finishErr;
    img.src = WALLPAPER_URL;

    // Cached images may already be complete before handlers attach.
    if (img.complete && img.naturalWidth > 0) {
      finishOk();
    }
  });
  return wallpaperLoad;
}

function layerSizeReady(host, output) {
  const hostRect = host.getBoundingClientRect();
  const outW = output.clientWidth || hostRect.width;
  const outH = output.clientHeight || hostRect.height;
  return outW >= MIN_LAYER_SIZE && outH >= MIN_LAYER_SIZE;
}

function destroyDroplets(reason) {
  startToken += 1;
  starting = false;
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
  if (instance || starting) return;
  if (!isCityRainWallpaper()) return;

  starting = true;
  const token = ++startToken;

  if (prefersReducedMotion()) {
    starting = false;
    const host = ensureHost();
    if (host) host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.reducedMotionSkip;
    return;
  }
  if (!hasRequiredAPIs() || !webgl2Available()) {
    starting = false;
    const host = ensureHost();
    if (host) host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.gracefulWebGL2Failure;
    return;
  }

  const host = ensureHost();
  if (!host) {
    starting = false;
    return;
  }
  const content = document.getElementById(CONTENT_ID);
  const source = document.getElementById(SOURCE_ID);
  const output = document.getElementById(OUTPUT_ID);
  if (!content || !source || !output) {
    starting = false;
    return;
  }

  source.removeAttribute("layoutsubtree");
  host.classList.remove("is-native-capture");

  // Reveal host early so layout/size can settle while the image loads.
  host.hidden = false;
  host.classList.add("is-active");
  output.classList.remove("is-hidden");
  output.classList.add("is-active");
  void host.offsetWidth;

  let bitmap;
  try {
    bitmap = await loadWallpaperImage();
  } catch {
    starting = false;
    if (token !== startToken) return;
    // Leave CSS wallpaper visible; do not claim active WebGL.
    host.classList.remove("is-active");
    host.hidden = true;
    output.classList.add("is-hidden");
    output.classList.remove("is-active");
    host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.gracefulWebGL2Failure;
    return;
  }

  if (token !== startToken || !isCityRainWallpaper()) {
    starting = false;
    return;
  }

  if (!layerSizeReady(host, output)) {
    starting = false;
    host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.waitingLayout;
    // Keep CSS wallpaper; retry when layout is ready.
    host.classList.remove("is-active");
    output.classList.add("is-hidden");
    output.classList.remove("is-active");
    return;
  }

  const rect = host.getBoundingClientRect();
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  output.style.width = "100%";
  output.style.height = "100%";
  output.width = Math.max(1, Math.round(Math.max(rect.width, output.clientWidth) * dpr));
  output.height = Math.max(1, Math.round(Math.max(rect.height, output.clientHeight) * dpr));
  source.style.width = "100%";
  source.style.height = "100%";

  if (!bitmap || !(bitmap.naturalWidth || bitmap.width)) {
    starting = false;
    host.classList.remove("is-active");
    host.hidden = true;
    output.classList.add("is-hidden");
    output.classList.remove("is-active");
    host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.gracefulWebGL2Failure;
    return;
  }

  try {
    // Bitmap is required so refraction uses wallpaper colors, not gray glass.
    instance = createDroplets(
      { source, content, output, bitmap },
      rainOptions()
    );
  } catch {
    instance = null;
  }

  starting = false;

  if (!instance) {
    if (token === startToken) {
      host.classList.remove("is-active");
      host.hidden = true;
      output.classList.add("is-hidden");
      output.classList.remove("is-active");
      host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.gracefulWebGL2Failure;
    }
    return;
  }

  host.dataset.dropletsState = DESKTOP_DROPLETS_MARKERS.active;
  host.dataset.dropletsContent = DESKTOP_DROPLETS_MARKERS.contentBitmap;

  const kick = () => {
    if (!instance || token !== startToken) return;
    try {
      if (typeof instance.setBitmap === "function") instance.setBitmap(bitmap);
      instance.resize();
    } catch {
      /* ignore */
    }
  };
  kick();
  requestAnimationFrame(() => {
    kick();
    requestAnimationFrame(kick);
  });
  // Late decode / GPU settle: re-push wallpaper a few times after mount.
  window.setTimeout(kick, 120);
  window.setTimeout(kick, 400);
  window.setTimeout(kick, 1000);
}

function syncWallpaperEffect() {
  if (isCityRainWallpaper()) startDroplets();
  else destroyDroplets(DESKTOP_DROPLETS_MARKERS.destroyed);
}

function ensureLayoutObserver() {
  if (layoutObserver || typeof ResizeObserver !== "function") return;
  const workspace = document.getElementById("vd-workspace");
  if (!workspace) return;
  layoutObserver = new ResizeObserver(() => {
    if (!isCityRainWallpaper()) return;
    if (!instance) syncWallpaperEffect();
    else {
      try {
        instance.resize();
      } catch {
        /* ignore */
      }
    }
  });
  layoutObserver.observe(workspace);
  const host = document.getElementById(HOST_ID);
  if (host) layoutObserver.observe(host);
}

function initCityRainDroplets() {
  if (initDone) return;
  initDone = true;

  // Warm the wallpaper cache immediately (even before settings arrive).
  preloadWallpaperImage();

  syncWallpaperEffect();
  scheduleBootstrapRetries();
  ensureLayoutObserver();

  observer = new MutationObserver((records) => {
    for (const record of records) {
      if (record.type === "attributes" && record.attributeName === "data-wallpaper") {
        syncWallpaperEffect();
        if (isCityRainWallpaper()) scheduleBootstrapRetries();
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
    "pageshow",
    () => {
      // bfcache / soft navigations
      preloadWallpaperImage();
      syncWallpaperEffect();
    },
    { passive: true }
  );

  window.addEventListener(
    "pagehide",
    () => {
      destroyDroplets(DESKTOP_DROPLETS_MARKERS.destroyed);
      clearRetryTimers();
      if (observer) observer.disconnect();
      observer = null;
      if (layoutObserver) layoutObserver.disconnect();
      layoutObserver = null;
      initDone = false;
    },
    { once: true }
  );
}

// Module scripts are deferred; still guard loading state.
if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", initCityRainDroplets, { once: true });
} else {
  initCityRainDroplets();
}

export default { sync: syncWallpaperEffect, destroy: destroyDroplets };
