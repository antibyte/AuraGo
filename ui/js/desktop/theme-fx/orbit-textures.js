(function () {
    'use strict';

    // Orbit textures (global AuraOrbitTextures). The two small data textures the scene computes on the CPU:
    //  - noise(THREE, rng): 256 x 256 random numbers behind the shader noise (see NOISE in orbit-shaders.js). Channel g is
    //    channel r shifted by (37, 17), which lets one bilinear fetch return two z layers of a value noise; b and a are
    //    independent numbers for hashes.
    //  - ringProfile(THREE, choreo, anisotropy): the radial profile of the rings, 2048 x 1: r density, g tone, b sparkle,
    //    with the C, B and A rings, the Cassini and Encke gaps, a thin outer ring and fine ringlets.
    // window.AuraOrbitTextures = { noise, ringProfile }.

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function smoothstep(a, b, x) { const t = clamp((x - a) / (b - a), 0, 1); return t * t * (3 - 2 * t); }

    function noise(THREE, rng) {
        const size = 256;
        const base = new Uint8Array(size * size);
        for (let i = 0; i < base.length; i++) base[i] = Math.floor(rng() * 256);
        const data = new Uint8Array(size * size * 4);
        for (let y = 0; y < size; y++) {
            for (let x = 0; x < size; x++) {
                const i = (y * size + x) * 4;
                data[i] = base[y * size + x];
                data[i + 1] = base[((y + 17) % size) * size + (x + 37) % size];
                data[i + 2] = Math.floor(rng() * 256);
                data[i + 3] = Math.floor(rng() * 256);
            }
        }
        const texture = new THREE.DataTexture(data, size, size, THREE.RGBAFormat);
        texture.wrapS = THREE.RepeatWrapping;
        texture.wrapT = THREE.RepeatWrapping;
        texture.minFilter = THREE.LinearFilter;
        texture.magFilter = THREE.LinearFilter;
        texture.generateMipmaps = false;
        texture.needsUpdate = true;
        return texture;
    }

    function ringProfile(THREE, choreo, anisotropy) {
        const size = 2048;
        const data = new Uint8Array(size * 4);
        const seeded = choreo.createRng(99);
        const table = new Float32Array(4096);
        for (let i = 0; i < table.length; i++) table[i] = seeded();
        const lattice = (i, seed) => table[(((i | 0) * 31 + seed * 977) % table.length + table.length) % table.length];
        const vnoise = (x, seed) => {
            const i = Math.floor(x), f = x - i, u = f * f * (3 - 2 * f);
            return lattice(i, seed) * (1 - u) + lattice(i + 1, seed) * u;
        };
        const band = (s, a, b, soft) => smoothstep(a - soft, a + soft, s) * (1 - smoothstep(b - soft, b + soft, s));
        for (let p = 0; p < size; p++) {
            const s = (p + 0.5) / size;
            let dens = Math.max(band(s, 0.0, 0.13, 0.02) * 0.26, band(s, 0.13, 0.56, 0.015) * 0.92, band(s, 0.63, 0.93, 0.012) * 0.70);
            dens = Math.max(dens, band(s, 0.56, 0.63, 0.01) * 0.035, Math.exp(-Math.pow((s - 0.972) / 0.0035, 2)) * 0.55);
            dens *= 1 - 0.88 * Math.exp(-Math.pow((s - 0.845) / 0.006, 2));
            dens *= 1 - 0.55 * Math.exp(-Math.pow((s - 0.715) / 0.004, 2));
            const n = (0.55 * vnoise(s * 55, 1) + 0.30 * vnoise(s * 150, 2) + 0.25 * vnoise(s * 420, 3) + 0.18 * vnoise(s * 1100, 4)) / 1.28 - 0.5;
            dens = clamp(dens * (0.78 + 0.95 * n), 0, 1);
            const tone = clamp(0.86 - 0.55 * s + 0.65 * n, 0, 1);
            const sparkle = clamp(vnoise(s * 300, 5) * 1.2, 0, 1);
            data[p * 4] = Math.round(dens * 255);
            data[p * 4 + 1] = Math.round(tone * 255);
            data[p * 4 + 2] = Math.round(sparkle * 255);
            data[p * 4 + 3] = 255;
        }
        const texture = new THREE.DataTexture(data, size, 1, THREE.RGBAFormat);
        texture.generateMipmaps = true;
        texture.minFilter = THREE.LinearMipmapLinearFilter;
        texture.magFilter = THREE.LinearFilter;
        texture.wrapS = THREE.ClampToEdgeWrapping;
        texture.wrapT = THREE.ClampToEdgeWrapping;
        texture.anisotropy = anisotropy || 1;
        texture.needsUpdate = true;
        return texture;
    }

    window.AuraOrbitTextures = Object.freeze({ noise, ringProfile });
}());
