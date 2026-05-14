(function () {
    'use strict';

    let canvas, renderer, scene, camera;
    let animationId = null;
    let active = false;
    let lastFrame = 0;
    let surface;
    let surfaceGeometry;
    let gridLines;
    let gridGeometry;
    let basePositions;
    let nextImpulseAt = 0;
    const impulses = [];

    const GRID = {
        width: 24,
        depth: 14,
        cols: 72,
        rows: 42
    };

    const IMPULSE_LIFETIME = 4.8;
    const MAX_IMPULSES = 8;
    const FRAME_INTERVAL = 1000 / 30;

    let colorLow;
    let colorMid;
    let colorHigh;

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'threedee' &&
            !prefersReducedMotion() &&
            window.innerWidth >= 768 &&
            !!window.THREE;
    }

    function createCanvas() {
        canvas = document.createElement('canvas');
        canvas.id = 'threedee-overlay';
        Object.assign(canvas.style, {
            position: 'fixed', inset: '0',
            width: '100vw', height: '100vh',
            pointerEvents: 'none', zIndex: '0',
            opacity: '0.76',
            mixBlendMode: 'normal',
            display: 'none'
        });
        document.body.appendChild(canvas);
    }

    function addImpulse(x, z, strength) {
        impulses.push({
            x,
            z,
            strength,
            start: performance.now() / 1000
        });

        while (impulses.length > MAX_IMPULSES) {
            impulses.shift();
        }
    }

    function spawnImpulse(t, immediate) {
        if (!immediate && t < nextImpulseAt) return;

        const marginX = GRID.width * 0.36;
        const marginZ = GRID.depth * 0.36;
        const x = (Math.random() - 0.5) * marginX * 2;
        const z = (Math.random() - 0.5) * marginZ * 2;
        addImpulse(x, z, 0.55 + Math.random() * 0.45);
        nextImpulseAt = t + 1.2 + Math.random() * 1.7;
    }

    function pruneImpulses(t) {
        for (let i = impulses.length - 1; i >= 0; i--) {
            if (t - impulses[i].start > IMPULSE_LIFETIME) {
                impulses.splice(i, 1);
            }
        }
    }

    function heightAt(x, z, t) {
        const slowWave = Math.sin(x * 0.46 + t * 0.72) * 0.22;
        const crossWave = Math.sin(z * 0.68 - t * 0.58) * 0.18;
        const diagonalWave = Math.sin((x + z) * 0.32 + t * 0.42) * 0.12;
        let height = slowWave + crossWave + diagonalWave;

        for (const impulse of impulses) {
            const age = t - impulse.start;
            if (age < 0 || age > IMPULSE_LIFETIME) continue;

            const dist = Math.hypot(x - impulse.x, z - impulse.z);
            const radius = age * 3.15;
            const ring = Math.sin((dist - radius) * 4.9);
            const envelope = Math.exp(-Math.abs(dist - radius) * 1.08);
            const fade = Math.pow(1 - age / IMPULSE_LIFETIME, 1.4);
            height += ring * envelope * fade * impulse.strength;
        }

        return height;
    }

    function colorForHeight(height, target) {
        const normalized = Math.max(0, Math.min(1, (height + 0.75) / 1.5));
        target.copy(colorLow).lerp(colorMid, Math.min(1, normalized * 1.2));
        if (normalized > 0.55) {
            target.lerp(colorHigh, (normalized - 0.55) / 0.45 * 0.62);
        }
    }

    function createSurfaceMaterial() {
        return new THREE.MeshStandardMaterial({
            color: 0xffffff,
            roughness: 0.96,
            metalness: 0.02,
            transparent: true,
            opacity: 0.62,
            side: THREE.DoubleSide,
            vertexColors: true
        });
    }

    function createGridMaterial() {
        return new THREE.LineBasicMaterial({
            color: 0xa8c7ff,
            transparent: true,
            opacity: 0.18,
            depthWrite: false
        });
    }

    function createRectGridGeometry() {
        const vertices = [];
        const stepX = GRID.width / GRID.cols;
        const stepZ = GRID.depth / GRID.rows;
        const left = -GRID.width / 2;
        const back = -GRID.depth / 2;

        for (let row = 0; row <= GRID.rows; row++) {
            const z = back + row * stepZ;
            for (let col = 0; col < GRID.cols; col++) {
                const x1 = left + col * stepX;
                const x2 = x1 + stepX;
                vertices.push(x1, 0, z, x2, 0, z);
            }
        }

        for (let col = 0; col <= GRID.cols; col++) {
            const x = left + col * stepX;
            for (let row = 0; row < GRID.rows; row++) {
                const z1 = back + row * stepZ;
                const z2 = z1 + stepZ;
                vertices.push(x, 0, z1, x, 0, z2);
            }
        }

        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute('position', new THREE.Float32BufferAttribute(vertices, 3));
        return geometry;
    }

    function createHeightfield() {
        surfaceGeometry = new THREE.PlaneGeometry(GRID.width, GRID.depth, GRID.cols, GRID.rows);
        surfaceGeometry.rotateX(-Math.PI / 2);

        const position = surfaceGeometry.getAttribute('position');
        basePositions = new Float32Array(position.array.length);
        basePositions.set(position.array);

        const colors = new Float32Array(position.count * 3);
        const initialColor = new THREE.Color();
        for (let i = 0; i < position.count; i++) {
            colorForHeight(0, initialColor);
            colors[i * 3] = initialColor.r;
            colors[i * 3 + 1] = initialColor.g;
            colors[i * 3 + 2] = initialColor.b;
        }
        surfaceGeometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));

        surface = new THREE.Mesh(surfaceGeometry, createSurfaceMaterial());
        surface.position.set(0, -2.55, -5.3);
        surface.rotation.z = -0.015;
        scene.add(surface);

        gridGeometry = createRectGridGeometry();
        gridLines = new THREE.LineSegments(gridGeometry, createGridMaterial());
        gridLines.position.copy(surface.position);
        gridLines.rotation.copy(surface.rotation);
        scene.add(gridLines);
    }

    function initScene() {
        if (scene) return true;

        colorLow = new THREE.Color(0x101626);
        colorMid = new THREE.Color(0x24324c);
        colorHigh = new THREE.Color(0x9bd7ff);

        scene = new THREE.Scene();
        scene.fog = new THREE.FogExp2(0x060914, 0.035);

        camera = new THREE.PerspectiveCamera(44, window.innerWidth / window.innerHeight, 0.1, 1000);
        camera.position.set(0, 5.15, 9.8);
        camera.lookAt(0, -1.95, -5.8);

        try {
            renderer = new THREE.WebGLRenderer({
                canvas,
                alpha: true,
                antialias: true,
                powerPreference: 'high-performance'
            });
        } catch (err) {
            console.warn('[ThreeDeeShader] WebGL unavailable:', err);
            scene = null;
            camera = null;
            return false;
        }
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
        renderer.setSize(window.innerWidth, window.innerHeight, false);
        renderer.setClearColor(0x000000, 0);
        if ('outputColorSpace' in renderer) {
            renderer.outputColorSpace = THREE.SRGBColorSpace;
        }

        const ambient = new THREE.HemisphereLight(0xbfd7ff, 0x0b0f1a, 1.05);
        scene.add(ambient);

        const key = new THREE.DirectionalLight(0xb9d9ff, 1.35);
        key.position.set(-3.5, 6, 6);
        scene.add(key);

        const rim = new THREE.DirectionalLight(0x7dd3fc, 0.55);
        rim.position.set(5, 3, -8);
        scene.add(rim);

        createHeightfield();
        return true;
    }

    function resize() {
        if (!renderer || !camera) return;

        const width = window.innerWidth;
        const height = window.innerHeight;
        renderer.setSize(width, height, false);
        camera.aspect = width / height;
        camera.updateProjectionMatrix();
    }

    function updateSurface(t) {
        if (!surfaceGeometry || !gridGeometry) return;

        const position = surfaceGeometry.getAttribute('position');
        const color = surfaceGeometry.getAttribute('color');
        const colorScratch = new THREE.Color();

        for (let i = 0; i < position.count; i++) {
            const x = basePositions[i * 3];
            const z = basePositions[i * 3 + 2];
            const height = heightAt(x, z, t);

            position.array[i * 3 + 1] = height;
            colorForHeight(height, colorScratch);
            color.array[i * 3] = colorScratch.r;
            color.array[i * 3 + 1] = colorScratch.g;
            color.array[i * 3 + 2] = colorScratch.b;
        }

        position.needsUpdate = true;
        color.needsUpdate = true;
        surfaceGeometry.computeVertexNormals();
        surfaceGeometry.attributes.normal.needsUpdate = true;

        const gridPosition = gridGeometry.getAttribute('position');
        const gridArray = gridPosition.array;
        for (let i = 0; i < gridArray.length; i += 3) {
            gridArray[i + 1] = heightAt(gridArray[i], gridArray[i + 2], t) + 0.012;
        }
        gridPosition.needsUpdate = true;
    }

    function render(time) {
        if (!active) return;
        animationId = requestAnimationFrame(render);

        if (time - lastFrame < FRAME_INTERVAL) return;
        lastFrame = time;

        const t = time * 0.001;
        spawnImpulse(t, false);
        pruneImpulses(t);
        updateSurface(t);

        if (camera) {
            const sway = Math.sin(t * 0.2) * 0.18;
            camera.position.x = sway;
            camera.position.y = 5.1 + Math.sin(t * 0.17) * 0.18;
            camera.lookAt(sway * 0.25, -2.0, -5.7);
        }

        renderer.render(scene, camera);
    }

    function start() {
        if (active || !shouldRun()) return;

        if (!canvas) createCanvas();
        if (!initScene()) return;
        active = true;
        lastFrame = 0;
        canvas.style.display = 'block';

        if (impulses.length === 0) {
            addImpulse(0, 0, 0.9);
        }
        animationId = requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) {
            cancelAnimationFrame(animationId);
            animationId = null;
        }
        if (canvas) canvas.style.display = 'none';
    }

    function sync() {
        if (shouldRun()) {
            start();
        } else {
            stop();
        }
    }

    function init() {
        if (!window.THREE) return;
        createCanvas();
        sync();

        const observer = new MutationObserver(sync);
        observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', function () {
            if (active) resize();
            sync();
        });
        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            if (mq.addEventListener) {
                mq.addEventListener('change', sync);
            } else if (mq.addListener) {
                mq.addListener(sync);
            }
        }
        document.addEventListener('visibilitychange', function () {
            if (document.hidden) {
                stop();
            } else {
                sync();
            }
        });
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    window.AuraGoThreeDee = {
        start,
        stop,
        sync,
        impulse: function (x, z, strength) {
            addImpulse(
                typeof x === 'number' ? x : 0,
                typeof z === 'number' ? z : 0,
                typeof strength === 'number' ? strength : 0.85
            );
        }
    };
})();
