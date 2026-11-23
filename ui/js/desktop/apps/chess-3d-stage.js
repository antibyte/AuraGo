/* AuraGo chess - stage assets for the 3D board.
 *
 * Pure builders for the textures and the studio environment that chess-3d.js
 * composes: square and frame grain, the piece-coordinate sprites, the lamp
 * pool on the floor and the image-based lighting of the Staunton pieces.
 * Nothing here touches the scene; every function returns a disposable resource.
 */
(function () {
  'use strict';

  function canvas2d(width, height) {
    if (typeof document === 'undefined') return null;
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext('2d');
    return ctx ? { canvas, ctx } : null;
  }

  function srgbTexture(THREE, canvas) {
    const tex = new THREE.CanvasTexture(canvas);
    if ('colorSpace' in tex && THREE.SRGBColorSpace) tex.colorSpace = THREE.SRGBColorSpace;
    return tex;
  }

  function createGrainTexture(THREE, contrast, streaks) {
    const surface = canvas2d(128, 128);
    if (!surface) return null;
    const ctx = surface.ctx;
    ctx.fillStyle = '#f4f4f4';
    ctx.fillRect(0, 0, 128, 128);
    let seed = 0x9e3779b9;
    const rnd = () => {
      seed ^= seed << 13; seed >>>= 0;
      seed ^= seed >> 17; seed >>>= 0;
      seed ^= seed << 5; seed >>>= 0;
      return (seed >>> 0) / 4294967296;
    };
    for (let y = 0; y < 128; y++) {
      const wave = Math.sin((y / 128) * Math.PI * (streaks || 6)) * 0.5 + 0.5;
      const shade = Math.round(244 - wave * contrast - rnd() * (contrast * 0.45));
      ctx.fillStyle = 'rgb(' + shade + ',' + shade + ',' + shade + ')';
      ctx.fillRect(0, y, 128, 1);
    }
    const tex = new THREE.CanvasTexture(surface.canvas);
    tex.wrapS = THREE.RepeatWrapping;
    tex.wrapT = THREE.RepeatWrapping;
    if ('colorSpace' in tex && THREE.SRGBColorSpace) tex.colorSpace = THREE.SRGBColorSpace;
    return tex;
  }

  /* Four offsets of one grain so neighbouring squares are not copies. The
     clones share the uploaded image, so this costs no extra texture memory. */
  function createGrainVariants(THREE, contrast, streaks) {
    const base = createGrainTexture(THREE, contrast, streaks);
    if (!base) return [];
    const variants = [base];
    [[0.37, 0.61], [0.74, 0.18], [0.12, 0.86]].forEach(([x, y]) => {
      const variant = base.clone();
      variant.needsUpdate = true;
      variant.offset.set(x, y);
      variants.push(variant);
    });
    return variants;
  }

  /* Warm key softbox + cool rim softbox for crisp Staunton collar highlights */
  function createStudioEnvironment(THREE, renderer) {
    if (!THREE.PMREMGenerator) return null;
    const surface = canvas2d(256, 128);
    if (!surface) return null;
    const ctx = surface.ctx;
    const bg = ctx.createLinearGradient(0, 0, 0, 128);
    bg.addColorStop(0, '#2a3548');
    bg.addColorStop(0.48, '#18202c');
    bg.addColorStop(0.52, '#10131a');
    bg.addColorStop(1, '#090b10');
    ctx.fillStyle = bg;
    ctx.fillRect(0, 0, 256, 128);

    const warm = ctx.createRadialGradient(74, 30, 4, 74, 30, 46);
    warm.addColorStop(0, 'rgba(255, 244, 224, 0.92)');
    warm.addColorStop(0.5, 'rgba(255, 226, 185, 0.28)');
    warm.addColorStop(1, 'rgba(255, 226, 185, 0)');
    ctx.fillStyle = warm;
    ctx.fillRect(0, 0, 256, 90);

    const cool = ctx.createRadialGradient(196, 38, 4, 196, 38, 40);
    cool.addColorStop(0, 'rgba(185, 214, 255, 0.72)');
    cool.addColorStop(1, 'rgba(185, 214, 255, 0)');
    ctx.fillStyle = cool;
    ctx.fillRect(120, 0, 136, 90);

    const equirect = srgbTexture(THREE, surface.canvas);
    equirect.mapping = THREE.EquirectangularReflectionMapping;
    try {
      const pmrem = new THREE.PMREMGenerator(renderer);
      pmrem.compileEquirectangularShader();
      const target = pmrem.fromEquirectangular(equirect);
      equirect.dispose();
      pmrem.dispose();
      return target;
    } catch (e) {
      equirect.dispose();
      return null;
    }
  }

  function createCoordTexture(THREE, text, color) {
    const surface = canvas2d(64, 64);
    if (!surface) return null;
    const ctx = surface.ctx;
    ctx.clearRect(0, 0, 64, 64);
    ctx.fillStyle = color || '#efe4c8';
    ctx.font = '700 34px system-ui, -apple-system, Segoe UI, sans-serif';
    ctx.textAlign = 'center';
    ctx.textBaseline = 'middle';
    ctx.fillText(text, 32, 34);
    return srgbTexture(THREE, surface.canvas);
  }

  /* White lamp pool for the floor's emissive map: the material's emissive
     colour tints it, so one texture serves every board skin. The falloff is
     computed per pixel instead of drawn as a canvas gradient, which dithers
     when the floor magnifies it. */
  function createPoolTexture(THREE) {
    const size = 256;
    const surface = canvas2d(size, size);
    if (!surface) return null;
    const image = surface.ctx.createImageData(size, size);
    const data = image.data;
    for (let y = 0; y < size; y++) {
      for (let x = 0; x < size; x++) {
        const fall = 1 - Math.min(1, Math.hypot((x + 0.5) / size - 0.5, (y + 0.5) / size - 0.5) * 2);
        const index = (y * size + x) * 4;
        data[index] = data[index + 1] = data[index + 2] = Math.round(255 * fall * fall * (1.6 - 0.6 * fall));
        data[index + 3] = 255;
      }
    }
    surface.ctx.putImageData(image, 0, 0);
    const tex = srgbTexture(THREE, surface.canvas);
    tex.wrapS = THREE.RepeatWrapping;
    tex.wrapT = THREE.RepeatWrapping;
    /* The floor is 4 m wide; 3.2 repeats and this offset centre a 1.25 m tile,
       so the pool reaches about 0.3 m past the board before it fades out. */
    tex.repeat.set(3.2, 3.2);
    tex.offset.set((1 - 1 / 3.2) / 2, (1 - 1 / 3.2) / 2);
    return tex;
  }

  window.ChessStage3D = {
    createGrainTexture,
    createGrainVariants,
    createStudioEnvironment,
    createCoordTexture,
    createPoolTexture
  };
})();
