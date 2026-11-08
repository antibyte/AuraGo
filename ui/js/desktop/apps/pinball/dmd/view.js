// AuraPinball DMD view (DOM): draws the 192x48 level frame as round amber dots on a canvas.
// Expected CSS hooks (owned by desktop-app-pinball.css): .pb-dmd (wrapper), .pb-dmd-canvas (canvas),
// .pb-dmd-glow (canvas while the glow is on), .pb-dmd-flat (wrapper when the glow is off / reduced motion).
const W = 192;
const H = 48;
const AMBER = [255, 140, 26];
const LEVELS = [0, 0.35, 0.65, 1];
const UNLIT = '#2b1606';
const GLOW = 'drop-shadow(0 0 3px rgba(255,140,26,.75)) drop-shadow(0 0 9px rgba(255,120,10,.4))';

function makeCanvas(w, h) {
    const c = document.createElement('canvas');
    c.width = w; c.height = h;
    return c;
}

// Mask canvas holding one round dot per cell in the given colour.
function dotLayer(scale, color) {
    const c = makeCanvas(W * scale, H * scale), g = c.getContext('2d');
    g.fillStyle = color;
    const r = scale * 0.43;
    g.beginPath();
    for (let y = 0; y < H; y++) for (let x = 0; x < W; x++) {
        const cx = (x + 0.5) * scale, cy = (y + 0.5) * scale;
        g.moveTo(cx + r, cy);
        g.arc(cx, cy, r, 0, Math.PI * 2);
    }
    g.fill();
    return c;
}

export function createDmdView(parent, { glow = true } = {}) {
    const wrap = document.createElement('div');
    wrap.className = 'pb-dmd';
    const canvas = document.createElement('canvas');
    canvas.className = 'pb-dmd-canvas';
    canvas.style.display = 'block';
    canvas.style.width = '100%';
    canvas.style.aspectRatio = '4 / 1';
    wrap.appendChild(canvas);
    parent.appendChild(wrap);

    const small = makeCanvas(W, H), smallCtx = small.getContext('2d');
    const image = smallCtx.createImageData(W, H);
    const ctx = canvas.getContext('2d');
    let scale = 0, mask = null, unlit = null, lastFrame = null, lastVersion = -1, glowOn = glow, observer = null, disposed = false;

    function applyGlow() {
        canvas.classList.toggle('pb-dmd-glow', glowOn);
        wrap.classList.toggle('pb-dmd-flat', !glowOn);
        canvas.style.filter = glowOn ? GLOW : '';
    }

    function layout() {
        const dpr = window.devicePixelRatio || 1, css = wrap.clientWidth || parent.clientWidth || W * 2;
        const next = Math.max(2, Math.floor(css * dpr / W));
        if (next === scale) return false;
        scale = next;
        canvas.width = W * scale; canvas.height = H * scale;
        mask = dotLayer(scale, '#fff');
        unlit = dotLayer(scale, UNLIT);
        return true;
    }

    function paint() {
        if (disposed || !lastFrame) return;
        const px = image.data;
        for (let i = 0; i < W * H; i++) {
            const level = lastFrame[i] & 3, o = i * 4;
            px[o] = AMBER[0] * LEVELS[level]; px[o + 1] = AMBER[1] * LEVELS[level]; px[o + 2] = AMBER[2] * LEVELS[level];
            px[o + 3] = level ? 255 : 0;
        }
        smallCtx.putImageData(image, 0, 0);
        ctx.globalCompositeOperation = 'source-over';
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        ctx.imageSmoothingEnabled = false;
        ctx.drawImage(small, 0, 0, canvas.width, canvas.height);
        ctx.globalCompositeOperation = 'destination-in';
        ctx.drawImage(mask, 0, 0);
        ctx.globalCompositeOperation = 'destination-over';
        ctx.drawImage(unlit, 0, 0);
        ctx.globalCompositeOperation = 'source-over';
    }

    applyGlow();
    layout();
    if (typeof ResizeObserver === 'function') {
        observer = new ResizeObserver(() => { if (layout()) paint(); });
        observer.observe(wrap);
    }

    return {
        element: wrap,
        canvas,
        // Redraws only when the version changed (or the size did).
        draw(frame, version) {
            if (disposed || !frame) return;
            lastFrame = frame;
            const resized = layout();
            if (version === lastVersion && !resized) return;
            lastVersion = version;
            paint();
        },
        setGlow(on) { glowOn = !!on; applyGlow(); },
        dispose() {
            disposed = true;
            if (observer) observer.disconnect();
            wrap.remove();
        }
    };
}
