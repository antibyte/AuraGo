(function () {
    'use strict';

    const PIXELS_PER_METER = 100;
    const MAX_FRAME_BOXES = 4096;
    const MAX_WINDOWS = 256;
    const MAX_MASKS = 1024;
    const MAX_BACKDROP_BLOCKS = 1800;
    const FRAME_TILE_BUDGET = 144;
    const WINDOW_Z_BASE = .16;
    const WINDOW_Z_STEP = .75;
    const WINDOW_FRAME_Z_OFFSET = .34;
    const MATERIALS = {
        wood: ['#9c6840', '#b17b4e', '#d1a36b', '#765034'],
        stone: ['#75857b', '#8d9a8c', '#a8ad99', '#596b63'],
        earth: ['#75563c', '#977047', '#b08a56', '#5d4837']
    };
    const BACKDROP_COLORS = {
        grass: ['#6d9a4d', '#7eaa58', '#91b767', '#527a43'],
        earth: ['#80513b', '#956044', '#ad7651', '#694534'],
        stone: ['#777e78', '#888d82', '#9b9c8d', '#656d69'],
        mountain: ['#7c8790', '#939a97', '#aeb0a2', '#63737f'],
        snow: ['#e9e0c7', '#f4e9d0', '#d9d5c5'],
        water: ['#538fa0', '#67a8b2', '#7ab8bd'],
        leaf: ['#47743d', '#5b8745', '#6f984f', '#3e6537'],
        cloud: ['#fff0d9', '#ffe4bd', '#f5d7ae']
    };

    function clamp(value, min, max) { return Math.max(min, Math.min(max, value)); }
    function rectFor(record) {
        const measured = record && record.element && typeof record.element.getBoundingClientRect === 'function'
            ? record.element.getBoundingClientRect()
            : record && record.rect;
        if (!measured) return null;
        const x = Number(measured.x ?? measured.left);
        const y = Number(measured.y ?? measured.top);
        const width = Number(measured.width);
        const height = Number(measured.height);
        return Number.isFinite(x) && Number.isFinite(y) && width > 0 && height > 0 ? { x, y, width, height } : null;
    }
    function materialForRecord(record) {
        const explicit = String(record && record.material || '').toLowerCase();
        if (MATERIALS[explicit]) return explicit;
        const source = String(record && (record.appId || record.id) || 'desktop-window');
        let hash = 0;
        for (let i = 0; i < source.length; i++) hash = (hash * 31 + source.charCodeAt(i)) | 0;
        return ['wood', 'stone', 'earth'][Math.abs(hash) % 3];
    }
    function frameStep(rect) {
        return clamp(Math.ceil((rect.width * 2 + rect.height * 2) / FRAME_TILE_BUDGET), 14, 34);
    }

    function create(options) {
        const { layer, fallback, fallbackContext, signal, fullWallpaper } = options;
        const read = options.readState;
        let THREE = null;
        let renderer = null;
        let scene = null;
        let camera = null;
        let frameMesh = null;
        let frameShadowMesh = null;
        let windowDepthMesh = null;
        let fragmentMesh = null;
        let fragmentOpacity = null;
        let maskMesh = null;
        let backdropMesh = null;
        let cloudMesh = null;
        let backgroundMaskMesh = null;
        let workObject = null;
        let ready = false;
        let contextLost = false;
        let backend = 'loading';
        let fallbackDirty = true;
        let frameVoxelCount = 0;
        let backdropVoxelCount = 0;

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

        function paintFallback() {
            const s = state();
            if (!fallbackContext || !fallbackDirty || s.stopped) return;
            fallbackDirty = false;
            const ctx = fallbackContext;
            const { width, height, windowRects, maskRects, desktopRects } = s;
            const dpr = Math.min(window.devicePixelRatio || 1, 1.25);
            fallback.width = Math.max(1, Math.round(width * dpr));
            fallback.height = Math.max(1, Math.round(height * dpr));
            ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
            ctx.clearRect(0, 0, width, height);
            if (fullWallpaper) {
                ctx.save();
                clipOutsideRects(ctx, windowRects.map(item => item.rect).concat(maskRects, desktopRects), width, height);
                const tile = clamp(Math.round(height / 30), 18, 30);
                const fillBlock = (x, y, w, h, color, alpha) => {
                    ctx.globalAlpha = alpha == null ? .9 : alpha;
                    ctx.fillStyle = color;
                    ctx.fillRect(Math.round(x), Math.round(y), Math.ceil(w), Math.ceil(h));
                };
                const ridgeY = height * .66;
                for (const peak of [[width * .12, width * .25, height * .19], [width * .43, width * .3, height * .28], [width * .78, width * .28, height * .18]]) {
                    const [center, span, rise] = peak;
                    const rows = Math.ceil(rise / tile);
                    for (let row = 0; row < rows; row++) {
                        const t = row / Math.max(1, rows - 1);
                        const half = span * (.54 - .42 * t);
                        for (let x = center - half; x <= center + half; x += tile) {
                            const snow = t >= .72 && Math.abs(x - center) < half * .52;
                            const colors = snow ? BACKDROP_COLORS.snow : BACKDROP_COLORS.mountain;
                            fillBlock(x, ridgeY - row * tile, tile + 1, tile + 1, colors[Math.abs(Math.floor(x / tile + row)) % colors.length], .82);
                        }
                    }
                }
                for (const cloud of [[.12, .22, 4], [.32, .17, 5], [.53, .26, 4], [.76, .18, 5], [.9, .29, 4]]) {
                    const [xRatio, yRatio, span] = cloud;
                    for (let row = 0; row < 3; row++) {
                        const count = row === 1 ? span : span - 1;
                        for (let col = 0; col < count; col++) {
                            fillBlock(width * xRatio + (col - (count - 1) / 2) * tile * .8,
                                height * yRatio + (row - 1) * tile * .35, tile * .9, tile * .62,
                                BACKDROP_COLORS.cloud[(row + col) % BACKDROP_COLORS.cloud.length], .94);
                        }
                    }
                }
                const ground = height - 52;
                for (let x = -tile; x < width + tile; x += tile) {
                    const edge = Math.abs((x + tile / 2 - width / 2) / Math.max(1, width / 2));
                    const surface = ground - clamp(height * (.04 + Math.pow(edge, 1.7) * .13 + Math.sin(x * .015) * .01), 24, height * .19);
                    fillBlock(x, surface, tile + 1, tile + 1, BACKDROP_COLORS.grass[Math.abs(Math.floor(x / tile)) % BACKDROP_COLORS.grass.length]);
                    for (let row = 1; surface + row * tile < height + tile; row++) {
                        const group = row > 4 && Math.floor(x / tile + row) % 4 === 0 ? BACKDROP_COLORS.stone : BACKDROP_COLORS.earth;
                        fillBlock(x, surface + row * tile, tile + 1, tile + 1, group[Math.abs(Math.floor(x / tile + row)) % group.length]);
                    }
                }
                ctx.globalAlpha = 1;
                ctx.restore();
            }
            ctx.lineJoin = 'miter';
            for (let windowIndex = 0; windowIndex < windowRects.length; windowIndex++) {
                const item = windowRects[windowIndex];
                const rect = item.rect;
                ctx.save();
                clipOutsideRects(ctx, [rect, ...maskRects, ...windowRects.slice(windowIndex + 1).map(higher => higher.rect)], width, height);
                const step = frameStep(rect);
                const colors = MATERIALS[materialForRecord(item.record)];
                ctx.shadowColor = 'rgba(59,39,25,.3)';
                ctx.shadowBlur = 4;
                for (let x = rect.x; x <= rect.x + rect.width; x += step) {
                    ctx.fillStyle = colors[Math.abs(Math.floor(x / step)) % colors.length];
                    ctx.fillRect(x, rect.y - step * .5, step + 1, step);
                    ctx.fillStyle = colors[(Math.abs(Math.floor(x / step)) + 2) % colors.length];
                    ctx.fillRect(x, rect.y + rect.height - step * .5, step + 1, step);
                }
                for (let y = rect.y + step; y < rect.y + rect.height - step; y += step) {
                    ctx.fillStyle = colors[(Math.abs(Math.floor(y / step)) + 1) % colors.length];
                    ctx.fillRect(rect.x - step * .5, y, step, step + 1);
                    ctx.fillStyle = colors[(Math.abs(Math.floor(y / step)) + 3) % colors.length];
                    ctx.fillRect(rect.x + rect.width - step * .5, y, step, step + 1);
                }
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

        function removeBackdropMeshes() {
            for (const mesh of [backdropMesh, cloudMesh]) {
                if (!mesh) continue;
                scene.remove(mesh);
                mesh.geometry.dispose();
                mesh.material.dispose();
            }
            backdropMesh = null;
            cloudMesh = null;
        }

        function rebuildBackdrop() {
            if (!scene || !THREE) return;
            removeBackdropMeshes();
            if (!fullWallpaper) { backdropVoxelCount = 0; return; }
            const s = state();
            const { width, height } = s;
            let seed = 0x47a812;
            const rand = () => { seed = (seed * 1664525 + 1013904223) >>> 0; return seed / 4294967296; };
            const terrain = [];
            const clouds = [];
            const block = (list, x, y, w, h, depth, z, color) => {
                if (terrain.length + clouds.length >= MAX_BACKDROP_BLOCKS) return;
                list.push({ x, y, w, h, depth, z, color });
            };
            const ground = height - 58;
            const tile = clamp(Math.round(height / 30), 22, 34);
            const palette = group => BACKDROP_COLORS[group];

            const horizon = height * .65;
            const peaks = [
                { x: width * .12, w: width * .27, h: height * .2, color: 'mountain' },
                { x: width * .41, w: width * .33, h: height * .3, color: 'stone' },
                { x: width * .77, w: width * .28, h: height * .19, color: 'mountain' }
            ];
            const ridgeTile = clamp(Math.round(height / 34), 20, 30);
            for (const peak of peaks) {
                const rows = Math.ceil(peak.h / ridgeTile);
                for (let row = 0; row < rows; row++) {
                    const t = row / Math.max(1, rows - 1);
                    const half = peak.w * (.56 - .44 * t);
                    const centerY = horizon - row * ridgeTile;
                    const first = Math.floor((peak.x - half) / ridgeTile) * ridgeTile;
                    for (let x = first; x <= peak.x + half; x += ridgeTile) {
                        const snow = t >= .72 && Math.abs(x - peak.x) < half * .52;
                        const colors = palette(snow ? 'snow' : peak.color);
                        block(terrain, x + ridgeTile / 2, centerY, ridgeTile * .94, ridgeTile * .94, ridgeTile * .68, -3.8,
                            colors[Math.floor(rand() * colors.length)]);
                    }
                }
            }

            const riverY = height * .77;
            for (let row = 0; row < 3; row++) {
                const span = width * (.1 + row * .12);
                const offset = (row % 2) * tile * .5;
                for (let x = width * .5 - span / 2 + offset; x < width * .5 + span / 2; x += tile * 1.2) {
                    const colors = palette('water');
                    block(terrain, x, riverY + row * tile * .45, tile * 1.08, tile * .34, tile * .46, -2.65,
                        colors[Math.floor(rand() * colors.length)]);
                }
            }

            const columns = Math.ceil(width / tile) + 2;
            for (let column = -1; column < columns; column++) {
                const x = column * tile + tile / 2;
                const nx = Math.abs((x - width / 2) / Math.max(1, width / 2));
                const surfaceHeight = clamp(height * (.03 + Math.pow(nx, 1.7) * .1 + Math.sin(column * .83) * .009), 20, height * .14);
                const surfaceY = ground - surfaceHeight;
                const variation = Math.floor(rand() * 4);
                block(terrain, x, surfaceY, tile * .94, tile * .94, tile * .86, -1.62, palette('grass')[variation]);
                const layers = Math.ceil(surfaceHeight / tile) + 2;
                for (let layerIndex = 1; layerIndex < layers; layerIndex++) {
                    const y = surfaceY + layerIndex * tile;
                    const group = layerIndex > 4 && (column + layerIndex) % 4 === 0 ? 'stone' : 'earth';
                    const colors = palette(group);
                    block(terrain, x, y, tile * .94, tile * .94, tile * .86, -1.62, colors[Math.floor(rand() * colors.length)]);
                }
                if ((nx > .78 && column % 5 === 1) || (nx > .91 && column % 4 === 0)) {
                    // Stagger tree depths: intersecting crowns must not share coplanar faces.
                    const treeDepth = (column % 7) * .035;
                    const trunkHeight = 3 + Math.floor(rand() * 3);
                    const trunkColor = ['#79553b', '#8a6040', '#694a35'][Math.floor(rand() * 3)];
                    for (let trunk = 1; trunk <= trunkHeight; trunk++) {
                        block(terrain, x, surfaceY - trunk * tile, tile * .34, tile * .88, tile * .4, -1.42 + treeDepth, trunkColor);
                    }
                    const canopyY = surfaceY - (trunkHeight + 1) * tile;
                    for (let canopyRow = 0; canopyRow < 3; canopyRow++) {
                        const canopySpan = (3 - Math.floor(canopyRow / 2)) * tile;
                        for (let leafX = -canopySpan; leafX <= canopySpan; leafX += tile) {
                            if (Math.abs(leafX) + canopyRow * tile * .34 > canopySpan * 1.35) continue;
                            const leaves = palette('leaf');
                            block(terrain, x + leafX, canopyY + canopyRow * tile, tile * .88, tile * .82, tile * .78, -1.36 + treeDepth,
                                leaves[Math.floor(rand() * leaves.length)]);
                        }
                    }
                }
            }

            const cloudDefs = [[.12, .24, 4], [.31, .15, 5], [.52, .25, 4], [.73, .16, 4], [.91, .28, 5]];
            for (const [xRatio, yRatio, span] of cloudDefs) {
                const cx = width * xRatio;
                const cy = height * yRatio;
                for (let row = 0; row < 3; row++) {
                    const rowSpan = row === 1 ? span : span - 1;
                    for (let col = 0; col < rowSpan; col++) {
                        const px = cx + (col - (rowSpan - 1) / 2) * tile * 1.04;
                        const py = cy + (row - 1) * tile * .78;
                        const colors = palette('cloud');
                        block(clouds, px, py, tile * .9, tile * .58, tile * .82, -4.8 - row * .12,
                            colors[Math.floor(rand() * colors.length)]);
                    }
                }
            }
            const sunX = width * .16;
            const sunY = height * .34;
            for (let row = 0; row < 3; row++) {
                for (let col = 0; col < 3; col++) {
                    block(clouds, sunX + (col - 1) * tile * .74, sunY + (row - 1) * tile * .74,
                        tile * .66, tile * .66, tile * .46, -5.3, row === 1 && col === 1 ? '#ffc66f' : '#ffdc9d');
                }
            }

            const createVoxelMesh = (items, roughness) => {
                if (!items.length) return null;
                const geometry = new THREE.BoxGeometry(1, 1, 1);
                const material = new THREE.MeshStandardMaterial({ color: 0xffffff, metalness: 0, roughness, depthWrite: true });
                setStencil(material, false, 0, 3, 0);
                const mesh = new THREE.InstancedMesh(geometry, material, Math.min(items.length, MAX_BACKDROP_BLOCKS));
                mesh.count = Math.min(items.length, MAX_BACKDROP_BLOCKS);
                mesh.frustumCulled = false;
                mesh.renderOrder = 0;
                for (let i = 0; i < mesh.count; i++) {
                    const item = items[i];
                    workObject.position.set(pixelX(item.x, width), pixelY(item.y, height), item.z);
                    workObject.rotation.set(0, 0, 0);
                    workObject.scale.set(item.w / PIXELS_PER_METER, item.h / PIXELS_PER_METER, item.depth / PIXELS_PER_METER);
                    workObject.updateMatrix();
                    mesh.setMatrixAt(i, workObject.matrix);
                    mesh.setColorAt(i, new THREE.Color(item.color));
                }
                mesh.instanceMatrix.needsUpdate = true;
                if (mesh.instanceColor) mesh.instanceColor.needsUpdate = true;
                return mesh;
            };
            backdropMesh = createVoxelMesh(terrain, .94);
            cloudMesh = createVoxelMesh(clouds, .9);
            if (backdropMesh) { backdropMesh.rotation.set(.14, .22, 0); backdropMesh.position.z = -4; }
            if (cloudMesh) { cloudMesh.rotation.set(.1, .16, 0); cloudMesh.position.z = -4; }
            backdropVoxelCount = (backdropMesh ? backdropMesh.count : 0) + (cloudMesh ? cloudMesh.count : 0);
            if (backdropMesh) scene.add(backdropMesh);
            if (cloudMesh) scene.add(cloudMesh);
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
            rebuildBackdrop();
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
            renderer.toneMapping = THREE.ACESFilmicToneMapping;
            renderer.toneMappingExposure = 1.02;
            scene = new THREE.Scene();
            camera = new THREE.OrthographicCamera(-1, 1, 1, -1, .1, 100);
            camera.position.set(0, 0, 32);
            camera.lookAt(0, 0, 0);
            workObject = new THREE.Object3D();

            const boxGeometry = new THREE.BoxGeometry(1, 1, 1);
            const boxMaterial = new THREE.MeshStandardMaterial({ color: 0xffffff, metalness: 0, roughness: .9,
                transparent: true, depthWrite: false });
            setStencil(boxMaterial, false, 0, 3, 0);
            boxGeometry.setAttribute('instanceOpacity', new THREE.InstancedBufferAttribute(new Float32Array(256).fill(1), 1));
            boxMaterial.onBeforeCompile = shader => {
                shader.vertexShader = 'attribute float instanceOpacity; varying float vInstanceOpacity;\n' + shader.vertexShader;
                shader.vertexShader = shader.vertexShader.replace('#include <begin_vertex>', '#include <begin_vertex>\n vInstanceOpacity = instanceOpacity;');
                shader.fragmentShader = 'varying float vInstanceOpacity;\n' + shader.fragmentShader;
                shader.fragmentShader = shader.fragmentShader.replace('#include <color_fragment>', '#include <color_fragment>\n diffuseColor.a *= vInstanceOpacity;');
            };
            fragmentOpacity = boxGeometry.getAttribute('instanceOpacity');
            fragmentMesh = new THREE.InstancedMesh(boxGeometry, boxMaterial, 256);
            fragmentMesh.count = 0;
            fragmentMesh.frustumCulled = false;
            fragmentMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            fragmentMesh.renderOrder = 4;
            scene.add(fragmentMesh);

            const frameMaterial = new THREE.MeshStandardMaterial({ color: 0xffffff, metalness: 0, roughness: .84 });
            setStencil(frameMaterial, false, 0, 1, 0);
            frameMesh = new THREE.InstancedMesh(new THREE.BoxGeometry(1, 1, 1), frameMaterial, MAX_FRAME_BOXES);
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

            const backgroundMaskMaterial = new THREE.MeshBasicMaterial({ colorWrite: false, depthWrite: false, depthTest: false, side: THREE.DoubleSide });
            setStencil(backgroundMaskMaterial, true, 2, 0xff, 2);
            backgroundMaskMesh = new THREE.InstancedMesh(new THREE.PlaneGeometry(1, 1), backgroundMaskMaterial, MAX_WINDOWS + MAX_MASKS);
            backgroundMaskMesh.count = 0;
            backgroundMaskMesh.frustumCulled = false;
            backgroundMaskMesh.instanceMatrix.setUsage(THREE.DynamicDrawUsage);
            backgroundMaskMesh.renderOrder = -101;
            scene.add(backgroundMaskMesh);

            const maskMaterial = new THREE.MeshBasicMaterial({ colorWrite: false, depthWrite: false, depthTest: false, side: THREE.DoubleSide });
            setStencil(maskMaterial, true, 1, 0xff, 1);
            maskMesh = new THREE.InstancedMesh(new THREE.PlaneGeometry(1, 1), maskMaterial, MAX_MASKS);
            maskMesh.count = 0;
            maskMesh.frustumCulled = false;
            maskMesh.renderOrder = -100;
            scene.add(maskMesh);

            const ambient = new THREE.AmbientLight(0xffedcf, 2.05);
            const key = new THREE.DirectionalLight(0xffd28c, 2.8);
            key.position.set(-4, 6, 7);
            const rim = new THREE.DirectionalLight(0x95bd72, 1.25);
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
            const { width, height, windowRects, maskRects, desktopRects, damageWindows, assemblingWindows } = s;
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
                const colors = MATERIALS[materialForRecord(record)];
                const step = frameStep(rect);
                const tileSize = Math.max(15, step * 1.06);
                const assembly = assemblingWindows.get(key);
                const progress = assembly ? clamp((now - assembly.at) / assembly.duration, 0, 1) : 1;
                const settle = 1 - Math.pow(1 - progress, 3);
                if (assembly && progress >= 1) assemblingWindows.delete(key);
                const tile = (side, index, cx, cy, bw, bh, color) => {
                    if (frameIndex >= MAX_FRAME_BOXES || (liveDamage && liveDamage.holes.has(side + ':' + index))) return;
                    let px = cx, py = cy, pz = planeZ + WINDOW_FRAME_Z_OFFSET;
                    if (assembly && progress < 1) {
                        const seed = (index + 1) * 12.9898 + side.charCodeAt(0) * 78.233;
                        px += Math.sin(seed) * step * 2.1 * (1 - settle);
                        py -= (24 + Math.abs(Math.cos(seed * 1.37)) * 44) * (1 - settle);
                        pz += .32 * (1 - settle);
                    }
                    workObject.position.set(pixelX(px, width), pixelY(py, height), pz);
                    workObject.rotation.set(0, 0, 0);
                    workObject.scale.set(bw / PIXELS_PER_METER, bh / PIXELS_PER_METER, tileSize / PIXELS_PER_METER);
                    workObject.updateMatrix();
                    frameMesh.setMatrixAt(frameIndex, workObject.matrix);
                    frameMesh.setColorAt(frameIndex, new THREE.Color(color));
                    workObject.position.set(pixelX(px + 2.5, width), pixelY(py + 4, height), pz - .30);
                    workObject.scale.set((bw + 5) / PIXELS_PER_METER, (bh + 5) / PIXELS_PER_METER, tileSize * .62 / PIXELS_PER_METER);
                    workObject.updateMatrix();
                    frameShadowMesh.setMatrixAt(frameIndex, workObject.matrix);
                    frameIndex++;
                };
                const horizontalCount = Math.max(1, Math.ceil(rect.width / step));
                const verticalCount = Math.max(1, Math.ceil(rect.height / step));
                for (let i = 0; i < horizontalCount; i++) {
                    const cx = rect.x + (i + .5) * rect.width / horizontalCount;
                    const widthForTile = rect.width / horizontalCount + 1;
                    tile('top', i, cx, rect.y - tileSize * .48, widthForTile, tileSize, colors[i % colors.length]);
                    tile('bottom', i, cx, rect.y + rect.height + tileSize * .48, widthForTile, tileSize, colors[(i + 2) % colors.length]);
                }
                for (let i = 0; i < verticalCount; i++) {
                    const cy = rect.y + (i + .5) * rect.height / verticalCount;
                    const heightForTile = rect.height / verticalCount + 1;
                    tile('left', i, rect.x - tileSize * .48, cy, tileSize, heightForTile, colors[(i + 1) % colors.length]);
                    tile('right', i, rect.x + rect.width + tileSize * .48, cy, tileSize, heightForTile, colors[(i + 3) % colors.length]);
                }
            }
            frameMesh.count = frameIndex;
            frameShadowMesh.count = frameIndex;
            frameVoxelCount = frameIndex;
            frameMesh.instanceMatrix.needsUpdate = true;
            frameShadowMesh.instanceMatrix.needsUpdate = true;
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

            const backgroundRects = windowRects.map(item => item.rect).concat(desktopRects);
            backgroundMaskMesh.count = Math.min(backgroundRects.length, MAX_WINDOWS + MAX_MASKS);
            for (let i = 0; i < backgroundMaskMesh.count; i++) {
                const rect = backgroundRects[i];
                workObject.position.set(pixelX(rect.x + rect.width / 2, width), pixelY(rect.y + rect.height / 2, height), 2);
                workObject.rotation.set(0, 0, 0);
                workObject.scale.set(rect.width / PIXELS_PER_METER, rect.height / PIXELS_PER_METER, 1);
                workObject.updateMatrix();
                backgroundMaskMesh.setMatrixAt(i, workObject.matrix);
            }
            backgroundMaskMesh.instanceMatrix.needsUpdate = true;
            invalidateFallback();
        }

        function setFragmentSlot(index, color) {
            if (!fragmentMesh || !workObject) return;
            workObject.position.set(0, 0, -100);
            workObject.rotation.set(0, 0, 0);
            workObject.scale.set(0, 0, 0);
            workObject.updateMatrix();
            fragmentMesh.setMatrixAt(index, workObject.matrix);
            fragmentMesh.setColorAt(index, new THREE.Color(color));
            fragmentOpacity.array[index] = 0;
        }
        function setFragmentColor(index, color, opacity) {
            if (!fragmentMesh || !fragmentOpacity) return;
            fragmentMesh.setColorAt(index, new THREE.Color(color));
            fragmentOpacity.array[index] = opacity;
        }
        function setFragmentOpacity(index, opacity) {
            if (fragmentOpacity) fragmentOpacity.array[index] = opacity;
        }
        function setFragmentMatrix(slot, index) {
            if (!fragmentMesh || !slot.active) {
                if (fragmentMesh) setFragmentSlot(index, '#75857b');
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
            if (fragmentMesh.instanceColor) fragmentMesh.instanceColor.needsUpdate = true;
        }
        function setBackdropPosition(x, y) {
            if (backdropMesh) backdropMesh.position.set(x * .4, -y * .35, -4);
            if (cloudMesh) cloudMesh.position.set(x * 1.3, -y, -4);
        }
        function setSceneryVisible(visible) {
            if (backdropMesh) backdropMesh.visible = !!visible;
            if (cloudMesh) cloudMesh.visible = !!visible;
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
            removeBackdropMeshes();
            if (renderer) {
                try { renderer.renderLists.dispose(); } catch (_) { }
                renderer.dispose();
                renderer.forceContextLoss();
                renderer.domElement.remove();
            }
            for (const mesh of [frameMesh, frameShadowMesh, windowDepthMesh, backgroundMaskMesh, fragmentMesh, maskMesh]) {
                if (mesh) { mesh.geometry.dispose(); mesh.material.dispose(); }
            }
            renderer = null;
            scene = null;
            camera = null;
            ready = false;
        }
        function inspect() {
            return { renderer: backend, rendererReady: !!renderer, ready, contextLost, frameVoxelCount, maxFrameVoxels: MAX_FRAME_BOXES,
                backdropVoxelCount, wallpaper: fullWallpaper ? 'voxel_world' : 'custom' };
        }
        return {
            paintFallback, createScene, resizeRenderer, updateSceneInstances, setFragmentSlot, setFragmentColor,
            setFragmentOpacity, setFragmentMatrix, commitFragments, setBackdropPosition, setSceneryVisible,
            invalidateFallback, render, dispose, inspect, materialForRecord, frameStep,
            setBackend(value) { backend = value; },
            get ready() { return ready; },
            get rendererReady() { return !!renderer; },
            get contextLost() { return contextLost; },
            get backend() { return backend; }
        };
    }

    window.AuraVoxelGraphics = Object.freeze({ create, materials: MATERIALS });
})();
