(function () {
    'use strict';

    const PIXELS_PER_METER = 100;
    const MAX_FRAME_BOXES = 4096;
    const MAX_WINDOWS = 256;
    const MAX_MASKS = 1024;
    const MAX_FRAGMENT_SLOTS = 256;
    const MAX_DUST = 96;
    const FRAME_TILE_BUDGET = 144;
    const WINDOW_Z_BASE = .16;
    const WINDOW_Z_STEP = .75;
    const WINDOW_FRAME_Z_OFFSET = .34;
    // Overlapping tiles are separated by far less than a pixel in depth, so their shared 1 px seams never z-fight.
    const CORNER_Z_LIFT = .008;
    const ODD_TILE_Z_LIFT = .004;
    const FRAME_BRIGHTNESS_JITTER = .08;
    const FRAGMENT_BRIGHTNESS_JITTER = .12;
    // Sequenced assembly: tile k of n starts at k/n * ASSEMBLY_SPAN of the assembly and needs ASSEMBLY_SPAN of it.
    const ASSEMBLY_SPAN = .5;
    const ASSEMBLY_DROP_PX = 40;
    const ASSEMBLY_START_SCALE = .6;
    const FLIGHT_Z_GAP = .02;
    const DUST_COLOR = 0xefe6d0;
    const DUST_LIFE = .45;
    const DUST_OPACITY = .7;
    const DUST_GROWTH = .6;
    // Dust sits just in front of the frame tiles and behind the next window plane.
    const DUST_Z_LIFT = .05;
    const DUST_DEFAULT_Z = .12;
    const MATERIALS = {
        wood: ['#c39a5d', '#ad854e', '#d9b67a', '#8a6539'],
        stone: ['#9e9e9e', '#8a8a8a', '#b9b9b9', '#5e5e5e'],
        earth: ['#8f6342', '#7a5236', '#5f3e27', '#7cbd4a']
    };
    const blocks = window.AuraVoxelBlocks;

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function easeOutBack(t) { return 1 + 2.70158 * Math.pow(t - 1, 3) + 1.70158 * Math.pow(t - 1, 2); }
    function hashString(text) {
        let hash = 0;
        for (let i = 0; i < text.length; i++) hash = (hash * 31 + text.charCodeAt(i)) | 0;
        return hash;
    }
    function hash01(n) {
        const v = Math.sin(n * 12.9898) * 43758.5453;
        return v - Math.floor(v);
    }
    function materialForRecord(record) {
        const explicit = String(record && record.material || '').toLowerCase();
        if (MATERIALS[explicit]) return explicit;
        const source = String(record && (record.appId || record.id) || 'desktop-window');
        return ['wood', 'stone', 'earth'][Math.abs(hashString(source)) % 3];
    }
    function frameStep(rect) {
        return clamp(Math.ceil((rect.width * 2 + rect.height * 2) / FRAME_TILE_BUDGET), 14, 34);
    }

    // The frame of one window as block tiles: four corners plus the edge strips. visit(side, index, order, total,
    // cx, cy, bw, bh, pattern, tileSize) gets every tile; order runs clockwise around the perimeter from the
    // top-left corner. The edge geometry is pinned by the occlusion browser test (left tile at x - tileSize * .48).
    function layoutTiles(rect, kind, visit) {
        const step = frameStep(rect);
        const tileSize = Math.max(15, step * 1.06);
        const columns = Math.max(1, Math.ceil(rect.width / step));
        const rows = Math.max(1, Math.ceil(rect.height / step));
        const total = 4 + 2 * columns + 2 * rows;
        const patterns = blocks.FRAME_PATTERNS[kind];
        const topCorner = kind === 'earth' ? patterns.top : patterns.corner;
        const inset = tileSize * .48;
        const left = rect.x - inset, right = rect.x + rect.width + inset;
        const top = rect.y - inset, bottom = rect.y + rect.height + inset;
        const pitchX = rect.width / columns, pitchY = rect.height / rows;
        visit('corner', 0, 0, total, left, top, tileSize, tileSize, topCorner, tileSize);
        for (let i = 0; i < columns; i++) visit('top', i, 1 + i, total, rect.x + (i + .5) * pitchX, top, pitchX + 1, tileSize, patterns.top, tileSize);
        visit('corner', 1, 1 + columns, total, right, top, tileSize, tileSize, topCorner, tileSize);
        for (let i = 0; i < rows; i++) visit('right', i, 2 + columns + i, total, right, rect.y + (i + .5) * pitchY, tileSize, pitchY + 1, patterns.edge, tileSize);
        visit('corner', 2, 2 + columns + rows, total, right, bottom, tileSize, tileSize, patterns.corner, tileSize);
        for (let i = 0; i < columns; i++) visit('bottom', i, 3 + columns + rows + columns - 1 - i, total, rect.x + (i + .5) * pitchX, bottom, pitchX + 1, tileSize, patterns.edge, tileSize);
        visit('corner', 3, 3 + 2 * columns + rows, total, left, bottom, tileSize, tileSize, patterns.corner, tileSize);
        for (let i = 0; i < rows; i++) visit('left', i, 4 + 2 * columns + rows + rows - 1 - i, total, left, rect.y + (i + .5) * pitchY, tileSize, pitchY + 1, patterns.edge, tileSize);
    }

    function create(options) {
        const { layer, fallback, fallbackContext, signal, fullWallpaper } = options;
        const read = options.readState;
        let THREE = null;
        let renderer = null;
        let scene = null;
        let camera = null;
        let atlas = null;
        let frameMesh = null;
        let framePattern = null;
        let frameShadowMesh = null;
        let windowDepthMesh = null;
        let fragmentMesh = null;
        let fragmentOpacity = null;
        let fragmentPattern = null;
        let dustMesh = null;
        let dustOpacity = null;
        let maskMesh = null;
        let workObject = null;
        let tint = null;
        let ready = false;
        let contextLost = false;
        let backend = 'loading';
        let fallbackDirty = true;
        let frameVoxelCount = 0;
        const dust = [];
        const patternCanvases = new Map();

        function state() { return read(); }
        function noteError(stage, error) { options.noteError(stage, error); }
        function pixelX(x, width) { return (x - width / 2) / PIXELS_PER_METER; }
        function pixelY(y, height) { return (height / 2 - y) / PIXELS_PER_METER; }

        function clipOutsideRects(ctx, rects, width, height) {
            for (const rect of rects) {
                if (!rect) continue;
                ctx.beginPath();
                ctx.rect(0, 0, width, height);
                ctx.rect(rect.x, rect.y, rect.width, rect.height);
                ctx.clip('evenodd');
            }
        }

        // One 8x8 canvas per pattern for the 2D fallback; the bevel is already part of the pixels.
        function patternCanvas(name) {
            let canvas = patternCanvases.get(name);
            if (canvas) return canvas;
            canvas = document.createElement('canvas');
            canvas.width = 8;
            canvas.height = 8;
            const context = canvas.getContext('2d');
            if (context) context.putImageData(new ImageData(new Uint8ClampedArray(blocks.tilePixels(name)), 8, 8), 0, 0);
            patternCanvases.set(name, canvas);
            return canvas;
        }

        function paintFallback() {
            const s = state();
            if (!fallbackContext || !fallbackDirty || s.stopped) return;
            fallbackDirty = false;
            const ctx = fallbackContext;
            const { width, height, windowRects, maskRects } = s;
            const dpr = Math.min(window.devicePixelRatio || 1, 1.25);
            fallback.width = Math.max(1, Math.round(width * dpr));
            fallback.height = Math.max(1, Math.round(height * dpr));
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            ctx.clearRect(0, 0, width, height);
            ctx.imageSmoothingEnabled = false;
            for (let windowIndex = 0; windowIndex < windowRects.length; windowIndex++) {
                const item = windowRects[windowIndex];
                const rect = item.rect;
                ctx.save();
                clipOutsideRects(ctx, [rect, ...maskRects, ...windowRects.slice(windowIndex + 1).map(higher => higher.rect)], width, height);
                ctx.shadowColor = 'rgba(59,39,25,.3)';
                ctx.shadowBlur = 4;
                layoutTiles(rect, materialForRecord(item.record), (side, index, order, total, cx, cy, bw, bh, pattern) => {
                    ctx.drawImage(patternCanvas(pattern), cx - bw / 2, cy - bh / 2, bw, bh);
                });
                ctx.shadowBlur = 0;
                ctx.restore();
            }
            ctx.globalAlpha = 1;
        }

        function setStencil(material, masked, reference, compareMask, writeMask) {
            material.stencilWrite = true;
            material.stencilRef = reference == null ? 1 : reference;
            material.stencilFuncMask = compareMask == null ? 0xff : compareMask;
            material.stencilWriteMask = writeMask == null ? 0xff : writeMask;
            material.stencilFunc = masked ? THREE.AlwaysStencilFunc : THREE.EqualStencilFunc;
            material.stencilFail = THREE.KeepStencilOp;
            material.stencilZFail = THREE.KeepStencilOp;
            material.stencilZPass = masked ? THREE.ReplaceStencilOp : THREE.KeepStencilOp;
        }

        // Per-instance fade: the geometry carries an instanceOpacity attribute, the fragment shader scales alpha by it.
        function patchInstanceOpacity(material, cacheKey) {
            const previous = material.onBeforeCompile;
            material.onBeforeCompile = (shader, rendererRef) => {
                if (previous) previous.call(material, shader, rendererRef);
                shader.vertexShader = 'attribute float instanceOpacity; varying float vInstanceOpacity;\n' + shader.vertexShader;
                shader.vertexShader = shader.vertexShader.replace('#include <begin_vertex>', '#include <begin_vertex>\n vInstanceOpacity = instanceOpacity;');
                shader.fragmentShader = 'varying float vInstanceOpacity;\n' + shader.fragmentShader;
                shader.fragmentShader = shader.fragmentShader.replace('#include <color_fragment>', '#include <color_fragment>\n diffuseColor.a *= vInstanceOpacity;');
            };
            material.customProgramCacheKey = () => cacheKey;
            material.needsUpdate = true;
        }

        function instancedAttribute(count, fill, dynamic) {
            const attribute = new THREE.InstancedBufferAttribute(new Float32Array(count).fill(fill), 1);
            if (dynamic) attribute.setUsage(THREE.DynamicDrawUsage);
            return attribute;
        }

        function resizeRenderer() {
            if (!renderer || !camera) return;
            const s = state();
            const qualityDpr = [1.25, 1.05, .78, .78, .65][s.qualityLevel] || .65;
            renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, qualityDpr));
            renderer.setSize(s.width, s.height, false);
            camera.left = -s.width / (2 * PIXELS_PER_METER);
            camera.right = s.width / (2 * PIXELS_PER_METER);
            camera.top = s.height / (2 * PIXELS_PER_METER);
            camera.bottom = -s.height / (2 * PIXELS_PER_METER);
            camera.updateProjectionMatrix();
            updateSceneInstances();
            invalidateFallback();
        }

        function createScene(vendor) {
            THREE = vendor && vendor.THREE;
            if (!THREE || !vendor.RAPIER || state().stopped) {
                if (!THREE || !vendor.RAPIER) noteError('vendor module', 'Three.js or Rapier export is missing');
                return false;
            }
            const canvas = document.createElement('canvas');
            canvas.className = 'vd-fx-voxel-webgl';
            canvas.setAttribute('aria-hidden', 'true');
            canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;display:block;pointer-events:none;';
            try {
                renderer = new THREE.WebGLRenderer({ canvas, alpha: true, antialias: false, stencil: true, depth: true,
                    powerPreference: 'low-power', preserveDrawingBuffer: false });
            } catch (error) {
                backend = 'fallback';
                noteError('WebGL initialization', error);
                return false;
            }
            canvas.addEventListener('webglcontextlost', event => {
                event.preventDefault();
                contextLost = true;
                backend = 'context-lost';
                canvas.style.display = 'none';
                fallback.style.display = 'block';
                invalidateFallback();
                paintFallback();
                options.onContextLost();
            }, { signal });
            canvas.addEventListener('webglcontextrestored', () => {
                contextLost = false;
                backend = 'three-webgl';
                canvas.style.display = 'block';
                fallback.style.display = 'none';
                options.onContextRestored();
            }, { signal });
            renderer.setClearColor(0x000000, 0);
            renderer.outputColorSpace = THREE.SRGBColorSpace;
            // The pixel-art palettes are meant to be seen as drawn, so the overlay is not tone mapped.
            renderer.toneMapping = THREE.NoToneMapping;
            scene = new THREE.Scene();
            camera = new THREE.OrthographicCamera(-1, 1, 1, -1, .1, 100);
            camera.position.set(0, 0, 32);
            camera.lookAt(0, 0, 0);
            workObject = new THREE.Object3D();
            tint = new THREE.Color();
            atlas = blocks.buildAtlas(THREE);

            // Fragments: the atlas pattern of the window material on all six faces of the cube, faded per instance.
            const fragmentGeometry = new THREE.BoxGeometry(1, 1, 1);
            const fragmentMaterial = new THREE.MeshLambertMaterial({ color: 0xffffff, transparent: true, depthWrite: false });
            fragmentMaterial.toneMapped = false;
            setStencil(fragmentMaterial, false, 0, 1, 0);
            blocks.patchAtlasMaterial(THREE, fragmentMaterial, atlas);
            patchInstanceOpacity(fragmentMaterial, 'voxel-fragment-' + atlas.count);
            fragmentOpacity = instancedAttribute(MAX_FRAGMENT_SLOTS, 1, false);
            fragmentPattern = instancedAttribute(MAX_FRAGMENT_SLOTS, 0, true);
            fragmentGeometry.setAttribute('instanceOpacity', fragmentOpacity);
            fragmentGeometry.setAttribute('aPattern', fragmentPattern);
            fragmentMesh = new THREE.InstancedMesh(fragmentGeometry, fragmentMaterial, MAX_FRAGMENT_SLOTS);
            fragmentMesh.count = 0;
            fragmentMesh.frustumCulled = false;
            fragmentMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            fragmentMesh.renderOrder = 4;
            scene.add(fragmentMesh);

            const frameGeometry = new THREE.BoxGeometry(1, 1, 1);
            const frameMaterial = new THREE.MeshLambertMaterial({ color: 0xffffff });
            frameMaterial.toneMapped = false;
            setStencil(frameMaterial, false, 0, 1, 0);
            blocks.patchAtlasMaterial(THREE, frameMaterial, atlas);
            framePattern = instancedAttribute(MAX_FRAME_BOXES, 0, true);
            frameGeometry.setAttribute('aPattern', framePattern);
            frameMesh = new THREE.InstancedMesh(frameGeometry, frameMaterial, MAX_FRAME_BOXES);
            frameMesh.count = 0;
            frameMesh.frustumCulled = false;
            frameMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            frameMesh.renderOrder = 3;
            scene.add(frameMesh);

            const shadowMaterial = new THREE.MeshBasicMaterial({ color: 0x293224, transparent: true, opacity: .22, depthWrite: false });
            setStencil(shadowMaterial, false, 0, 1, 0);
            frameShadowMesh = new THREE.InstancedMesh(new THREE.BoxGeometry(1, 1, 1), shadowMaterial, MAX_FRAME_BOXES);
            frameShadowMesh.count = 0;
            frameShadowMesh.frustumCulled = false;
            frameShadowMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            frameShadowMesh.renderOrder = 2;
            scene.add(frameShadowMesh);

            const depthMaterial = new THREE.MeshBasicMaterial({ colorWrite: false, depthWrite: true, depthTest: true, side: THREE.DoubleSide });
            windowDepthMesh = new THREE.InstancedMesh(new THREE.PlaneGeometry(1, 1), depthMaterial, MAX_WINDOWS);
            windowDepthMesh.count = 0;
            windowDepthMesh.frustumCulled = false;
            windowDepthMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            windowDepthMesh.renderOrder = 1;
            scene.add(windowDepthMesh);

            const maskMaterial = new THREE.MeshBasicMaterial({ colorWrite: false, depthWrite: false, depthTest: false, side: THREE.DoubleSide });
            setStencil(maskMaterial, true, 1, 0xff, 1);
            maskMesh = new THREE.InstancedMesh(new THREE.PlaneGeometry(1, 1), maskMaterial, MAX_MASKS);
            maskMesh.count = 0;
            maskMesh.frustumCulled = false;
            maskMesh.renderOrder = -100;
            scene.add(maskMesh);

            // Dust puffs: flat squares that fade out, drawn under the same stencil as the frames.
            const dustGeometry = new THREE.BoxGeometry(1, 1, 1);
            const dustMaterial = new THREE.MeshBasicMaterial({ color: DUST_COLOR, transparent: true, depthWrite: false });
            dustMaterial.toneMapped = false;
            setStencil(dustMaterial, false, 0, 1, 0);
            patchInstanceOpacity(dustMaterial, 'voxel-dust');
            dustOpacity = instancedAttribute(MAX_DUST, 0, true);
            dustGeometry.setAttribute('instanceOpacity', dustOpacity);
            dustMesh = new THREE.InstancedMesh(dustGeometry, dustMaterial, MAX_DUST);
            dustMesh.count = 0;
            dustMesh.frustumCulled = false;
            dustMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            dustMesh.renderOrder = 5;
            scene.add(dustMesh);

            // Front faces catch about the palette colour; the tumbling fragments get visibly lit and shaded sides.
            const ambient = new THREE.AmbientLight(0xfff6e8, 1.85);
            const key = new THREE.DirectionalLight(0xfff0d6, 1.7);
            key.position.set(-4, 6, 7);
            const rim = new THREE.DirectionalLight(0xd9e6ff, .9);
            rim.position.set(4, -2, -4);
            scene.add(ambient, key, rim);
            layer.appendChild(canvas);
            fallback.style.display = 'none';
            backend = 'three-webgl';
            ready = true;
            resizeRenderer();
            updateSceneInstances();
            return true;
        }

        function updateSceneInstances() {
            const s = state();
            if (!ready || !scene || !THREE || !windowDepthMesh) return;
            const { width, height, windowRects, maskRects, damageWindows, assemblingWindows } = s;
            let frameIndex = 0;
            const now = performance.now();
            const windowCount = Math.min(windowRects.length, MAX_WINDOWS);
            windowDepthMesh.count = windowCount;
            for (let windowRank = 0; windowRank < windowCount; windowRank++) {
                const item = windowRects[windowRank];
                const record = item.record;
                const rect = item.rect;
                const planeZ = WINDOW_Z_BASE + windowRank * WINDOW_Z_STEP;
                workObject.position.set(pixelX(rect.x + rect.width / 2, width), pixelY(rect.y + rect.height / 2, height), planeZ);
                workObject.rotation.set(0, 0, 0);
                workObject.scale.set(rect.width / PIXELS_PER_METER, rect.height / PIXELS_PER_METER, 1);
                workObject.updateMatrix();
                windowDepthMesh.setMatrixAt(windowRank, workObject.matrix);
                const key = String(record.id);
                const damage = damageWindows.get(key);
                if (damage && !damage.dragging && now >= damage.repairAt) damageWindows.delete(key);
                const liveDamage = damage && (damage.dragging || now < damage.repairAt) ? damage : null;
                const assembly = assemblingWindows.get(key);
                const progress = assembly ? clamp((now - assembly.at) / assembly.duration, 0, 1) : 1;
                const seed = hashString(key);
                layoutTiles(rect, materialForRecord(record), (side, index, order, total, cx, cy, bw, bh, pattern, tileSize) => {
                    if (frameIndex >= MAX_FRAME_BOXES || (liveDamage && liveDamage.holes.has(side + ':' + index))) return;
                    let py = cy, scale = 1;
                    let pz = planeZ + WINDOW_FRAME_Z_OFFSET + (side === 'corner' ? CORNER_Z_LIFT : (index & 1) * ODD_TILE_Z_LIFT);
                    if (assembly) {
                        const p = clamp((progress - order / total * ASSEMBLY_SPAN) / ASSEMBLY_SPAN, 0, 1);
                        if (p <= 0) return;
                        if (p >= 1 && side === 'corner' && !(assembly.landed & (1 << index))) {
                            assembly.landed = (assembly.landed | (1 << index));
                            spawnDust(cx, cy, 3, pz + tileSize / (2 * PIXELS_PER_METER) + DUST_Z_LIFT, index === 0 || index === 3 ? -1 : 1);
                        }
                        if (p < 1) {
                            const eased = easeOutBack(p);
                            py -= ASSEMBLY_DROP_PX * (1 - eased);
                            scale = ASSEMBLY_START_SCALE + (1 - ASSEMBLY_START_SCALE) * eased;
                            // A falling tile stays just behind its own window, so it never covers the window contents.
                            pz = planeZ - tileSize * scale / (2 * PIXELS_PER_METER) - FLIGHT_Z_GAP;
                        }
                    }
                    workObject.position.set(pixelX(cx, width), pixelY(py, height), pz);
                    workObject.rotation.set(0, 0, 0);
                    workObject.scale.set(bw * scale / PIXELS_PER_METER, bh * scale / PIXELS_PER_METER, tileSize * scale / PIXELS_PER_METER);
                    workObject.updateMatrix();
                    frameMesh.setMatrixAt(frameIndex, workObject.matrix);
                    const shade = 1 + (hash01(seed + order * 7.31) - .5) * FRAME_BRIGHTNESS_JITTER;
                    frameMesh.setColorAt(frameIndex, tint.setRGB(shade, shade, shade));
                    framePattern.array[frameIndex] = atlas.index[pattern];
                    workObject.position.set(pixelX(cx + 2.5, width), pixelY(py + 4, height), pz - .30);
                    workObject.scale.set((bw + 5) * scale / PIXELS_PER_METER, (bh + 5) * scale / PIXELS_PER_METER, tileSize * .62 * scale / PIXELS_PER_METER);
                    workObject.updateMatrix();
                    frameShadowMesh.setMatrixAt(frameIndex, workObject.matrix);
                    frameIndex++;
                });
                if (assembly && progress >= 1) assemblingWindows.delete(key);
            }
            frameMesh.count = frameIndex;
            frameShadowMesh.count = frameIndex;
            frameVoxelCount = frameIndex;
            frameMesh.instanceMatrix.needsUpdate = true;
            frameShadowMesh.instanceMatrix.needsUpdate = true;
            framePattern.needsUpdate = true;
            if (frameMesh.instanceColor) frameMesh.instanceColor.needsUpdate = true;
            windowDepthMesh.instanceMatrix.needsUpdate = true;

            maskMesh.count = Math.min(maskRects.length, MAX_MASKS);
            for (let i = 0; i < maskMesh.count; i++) {
                const rect = maskRects[i];
                workObject.position.set(pixelX(rect.x + rect.width / 2, width), pixelY(rect.y + rect.height / 2, height), 2);
                workObject.rotation.set(0, 0, 0);
                workObject.scale.set(rect.width / PIXELS_PER_METER, rect.height / PIXELS_PER_METER, 1);
                workObject.updateMatrix();
                maskMesh.setMatrixAt(i, workObject.matrix);
            }
            maskMesh.instanceMatrix.needsUpdate = true;

            invalidateFallback();
        }

        // Dust: x/y in screen px, vx/vy in px/s (up is negative), z in scene metres. dirX is the outward side
        // (-1 left, 1 right, 0 either).
        function spawnDust(x, y, count, z, dirX) {
            if (!dustMesh) return;
            for (let i = 0; i < count && dust.length < MAX_DUST; i++) {
                const speed = 30 + Math.random() * 30;
                const side = dirX || (Math.random() < .5 ? -1 : 1);
                dust.push({ x: x + (Math.random() - .5) * 6, y: y + (Math.random() - .5) * 6, z: z == null ? DUST_DEFAULT_Z : z,
                    vx: side * speed * (.3 + Math.random() * .5), vy: -speed * (.6 + Math.random() * .4), size: 4 + Math.random() * 3, age: 0 });
            }
            writeDust();
        }
        function writeDust() {
            if (!dustMesh) return;
            const { width, height } = state();
            for (let i = 0; i < dust.length; i++) {
                const puff = dust[i];
                const life = puff.age / DUST_LIFE;
                const size = puff.size * (1 + DUST_GROWTH * life) / PIXELS_PER_METER;
                workObject.position.set(pixelX(puff.x, width), pixelY(puff.y, height), puff.z);
                workObject.rotation.set(0, 0, 0);
                workObject.scale.set(size, size, size);
                workObject.updateMatrix();
                dustMesh.setMatrixAt(i, workObject.matrix);
                dustOpacity.array[i] = DUST_OPACITY * (1 - life);
            }
            dustMesh.count = dust.length;
            dustMesh.instanceMatrix.needsUpdate = true;
            dustOpacity.needsUpdate = true;
        }
        function updateDust(dt) {
            for (let i = dust.length - 1; i >= 0; i--) {
                const puff = dust[i];
                puff.age += dt;
                if (puff.age >= DUST_LIFE) { dust.splice(i, 1); continue; }
                puff.x += puff.vx * dt;
                puff.y += puff.vy * dt;
            }
            writeDust();
        }

        function setFragmentSlot(index) {
            if (!fragmentMesh || !workObject) return;
            workObject.position.set(0, 0, -100);
            workObject.rotation.set(0, 0, 0);
            workObject.scale.set(0, 0, 0);
            workObject.updateMatrix();
            fragmentMesh.setMatrixAt(index, workObject.matrix);
            fragmentMesh.setColorAt(index, tint.setRGB(1, 1, 1));
            fragmentOpacity.array[index] = 0;
        }
        // A fragment shows the edge pattern of its window material on every face, with a little brightness jitter.
        function setFragmentLook(index, material, opacity) {
            if (!fragmentMesh || !fragmentOpacity) return;
            const patterns = blocks.FRAME_PATTERNS[material] || blocks.FRAME_PATTERNS.stone;
            const shade = 1 + (Math.random() - .5) * FRAGMENT_BRIGHTNESS_JITTER;
            fragmentPattern.array[index] = atlas.index[patterns.edge];
            fragmentMesh.setColorAt(index, tint.setRGB(shade, shade, shade));
            fragmentOpacity.array[index] = opacity;
        }
        function setFragmentOpacity(index, opacity) {
            if (fragmentOpacity) fragmentOpacity.array[index] = opacity;
        }
        function setFragmentMatrix(slot, index) {
            if (!fragmentMesh || !slot.active) {
                if (fragmentMesh) setFragmentSlot(index);
                return;
            }
            const translation = slot.body.translation();
            const rotation = slot.body.rotation();
            const scale = slot.size / PIXELS_PER_METER;
            const fade = clamp((slot.age - 3.8) / 1.8, 0, 1);
            const shrink = 1 - fade * .62;
            workObject.position.set(translation.x, translation.y, translation.z);
            workObject.quaternion.set(rotation.x, rotation.y, rotation.z, rotation.w);
            workObject.scale.set(scale * shrink, scale * shrink, scale * shrink);
            workObject.updateMatrix();
            fragmentMesh.setMatrixAt(index, workObject.matrix);
            fragmentOpacity.array[index] = slot.alpha;
        }
        function commitFragments(count) {
            if (!fragmentMesh || !fragmentOpacity) return;
            if (count != null) fragmentMesh.count = count;
            fragmentMesh.instanceMatrix.needsUpdate = true;
            fragmentOpacity.needsUpdate = true;
            fragmentPattern.needsUpdate = true;
            if (fragmentMesh.instanceColor) fragmentMesh.instanceColor.needsUpdate = true;
        }
        function invalidateFallback() { fallbackDirty = true; }
        function render() {
            if (!renderer || !scene || !camera || state().stopped || contextLost) return null;
            const started = performance.now();
            try { renderer.render(scene, camera); }
            catch (error) { backend = 'three-webgl-error'; noteError('WebGL render', error); }
            if (!renderer || contextLost) paintFallback();
            return performance.now() - started;
        }
        function dispose() {
            if (renderer) {
                try { renderer.renderLists.dispose(); } catch (_) { }
                renderer.dispose();
                renderer.forceContextLoss();
                renderer.domElement.remove();
            }
            for (const mesh of [frameMesh, frameShadowMesh, windowDepthMesh, fragmentMesh, dustMesh, maskMesh]) {
                if (mesh) { mesh.geometry.dispose(); mesh.material.dispose(); }
            }
            if (atlas) atlas.texture.dispose();
            dust.length = 0;
            renderer = null;
            scene = null;
            camera = null;
            ready = false;
        }
        function inspect() {
            return { renderer: backend, rendererReady: !!renderer, ready, contextLost, frameVoxelCount, maxFrameVoxels: MAX_FRAME_BOXES,
                backdropVoxelCount: 0, wallpaper: fullWallpaper ? 'voxel_world' : 'custom' };
        }
        return {
            paintFallback, createScene, resizeRenderer, updateSceneInstances, setFragmentSlot, setFragmentLook,
            setFragmentOpacity, setFragmentMatrix, commitFragments, spawnDust, updateDust,
            invalidateFallback, render, dispose, inspect, materialForRecord, frameStep,
            dustActive() { return dust.length > 0; },
            setBackend(value) { backend = value; },
            get ready() { return ready; },
            get rendererReady() { return !!renderer; },
            get contextLost() { return contextLost; },
            get backend() { return backend; }
        };
    }

    window.AuraVoxelGraphics = Object.freeze({ create, materials: MATERIALS });
})();
