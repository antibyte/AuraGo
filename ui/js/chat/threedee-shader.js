(function () {
    'use strict';

    let canvas, renderer, scene, camera;
    let animationId = null;
    let active = false;
    let lastFrame = 0;
    let shapes = [];
    let morphTargets = [];
    let particles;
    let energyLines = [];
    let glowRings = [];
    let clock;

    /* ── helpers ──────────────────────────────────────────────────── */

    function prefersReducedMotion() {
        return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
    }

    function shouldRun() {
        return document.documentElement.getAttribute('data-theme') === 'threedee' &&
            !prefersReducedMotion() &&
            window.innerWidth >= 768 &&
            !!window.THREE;
    }

    /* ── canvas ───────────────────────────────────────────────────── */

    function createCanvas() {
        canvas = document.createElement('canvas');
        canvas.id = 'threedee-overlay';
        Object.assign(canvas.style, {
            position: 'fixed', inset: '0',
            width: '100vw', height: '100vh',
            pointerEvents: 'none', zIndex: '0',
            opacity: '0.88',
            mixBlendMode: 'screen',
            display: 'none'
        });
        document.body.appendChild(canvas);
    }

    /* ── materials ────────────────────────────────────────────────── */

    function wireframeMat(color, opacity) {
        return new THREE.MeshBasicMaterial({
            color, wireframe: true, transparent: true,
            opacity, depthWrite: false
        });
    }

    function glowRingMat(color, opacity) {
        return new THREE.MeshBasicMaterial({
            color, wireframe: false, transparent: true,
            opacity, depthWrite: false, side: THREE.DoubleSide
        });
    }

    /* ── shape factory ────────────────────────────────────────────── */

    function addMorphingShape(geometry, color, opacity, pos, rotSpeed, morphAmplitude) {
        const mat = wireframeMat(color, opacity);
        const mesh = new THREE.Mesh(geometry, mat);
        mesh.position.set(pos[0], pos[1], pos[2]);
        mesh.userData = {
            rotationSpeed: rotSpeed,
            floatOffset: Math.random() * Math.PI * 2,
            morphPhase: Math.random() * Math.PI * 2,
            morphAmplitude: morphAmplitude || 0.15,
            baseScale: 1,
            colorBase: color,
            origPositions: null
        };
        /* store original vertex positions for morphing */
        const posAttr = geometry.getAttribute('position');
        if (posAttr) {
            const orig = new Float32Array(posAttr.array.length);
            orig.set(posAttr.array);
            mesh.userData.origPositions = orig;
        }
        scene.add(mesh);
        shapes.push(mesh);
    }

    /* ── glow rings around shapes ─────────────────────────────────── */

    function addGlowRing(color, pos, baseRadius, segments) {
        const geo = new THREE.RingGeometry(baseRadius - 0.02, baseRadius + 0.02, segments || 48);
        const mat = glowRingMat(color, 0);
        const mesh = new THREE.Mesh(geo, mat);
        mesh.position.set(pos[0], pos[1], pos[2]);
        mesh.userData = { phase: Math.random() * Math.PI * 2, baseRadius };
        scene.add(mesh);
        glowRings.push(mesh);
    }

    /* ── energy connections ───────────────────────────────────────── */

    function addEnergyLine(from, to, color) {
        const points = [];
        const segs = 28;
        for (let i = 0; i <= segs; i++) {
            const t = i / segs;
            points.push(new THREE.Vector3(
                from[0] + (to[0] - from[0]) * t,
                from[1] + (to[1] - from[1]) * t,
                from[2] + (to[2] - from[2]) * t
            ));
        }
        const geo = new THREE.BufferGeometry().setFromPoints(points);
        const mat = new THREE.LineBasicMaterial({
            color, transparent: true, opacity: 0, depthWrite: false
        });
        const line = new THREE.Line(geo, mat);
        line.userData = { from, to, segs, phase: Math.random() * Math.PI * 2 };
        scene.add(line);
        energyLines.push(line);
    }

    /* ── particles ────────────────────────────────────────────────── */

    function createParticles() {
        const count = window.innerWidth < 1100 ? 350 : 650;
        const positions = new Float32Array(count * 3);
        const velocities = new Float32Array(count * 3);
        for (let i = 0; i < count; i++) {
            positions[i * 3] = (Math.random() - 0.5) * 38;
            positions[i * 3 + 1] = (Math.random() - 0.5) * 22;
            positions[i * 3 + 2] = -Math.random() * 32 - 2;
            velocities[i * 3] = (Math.random() - 0.5) * 0.003;
            velocities[i * 3 + 1] = (Math.random() - 0.5) * 0.003;
            velocities[i * 3 + 2] = (Math.random() - 0.5) * 0.001;
        }
        const geo = new THREE.BufferGeometry();
        geo.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        geo.userData = { velocities };

        const mat = new THREE.PointsMaterial({
            color: 0xa5b4fc, size: 0.04, transparent: true,
            opacity: 0.55, depthWrite: false,
            blending: THREE.AdditiveBlending
        });
        particles = new THREE.Points(geo, mat);
        scene.add(particles);
    }

    /* ── scene setup ──────────────────────────────────────────────── */

    function initScene() {
        if (!window.THREE) { console.warn('[ThreeDeeShader] Three.js not available'); return false; }
        if (!canvas) createCanvas();

        try {
            renderer = new THREE.WebGLRenderer({
                canvas, alpha: true, antialias: true, powerPreference: 'low-power'
            });
        } catch (err) {
            console.warn('[ThreeDeeShader] WebGL unavailable:', err);
            return false;
        }
        renderer.setClearColor(0x000000, 0);
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));

        scene = new THREE.Scene();
        scene.fog = new THREE.FogExp2(0x070b14, 0.012);
        camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 1000);
        camera.position.z = 9;

        /* lights */
        scene.add(new THREE.AmbientLight(0x111122, 0.4));
        const indigo = new THREE.PointLight(0x818cf8, 1.0, 34);
        indigo.position.set(-5, 3, 6);
        scene.add(indigo);
        const cyan = new THREE.PointLight(0x06b6d4, 0.6, 30);
        cyan.position.set(6, -2, 5);
        scene.add(cyan);
        const purple = new THREE.PointLight(0xc084fc, 0.5, 28);
        purple.position.set(0, -4, 4);
        scene.add(purple);

        /* ── shapes ── */
        addMorphingShape(new THREE.IcosahedronGeometry(1.75, 2), 0x818cf8, 0.28, [-5.5, 2.8, -4], [0.09, 0.05, 0.04], 0.2);
        addMorphingShape(new THREE.IcosahedronGeometry(1.15, 2), 0x22d3ee, 0.22, [5.0, 1.6, -5.5], [0.04, 0.08, 0.06], 0.18);
        addMorphingShape(new THREE.IcosahedronGeometry(1.45, 2), 0xc084fc, 0.16, [1.8, -3.4, -7], [0.06, 0.04, 0.08], 0.22);
        addMorphingShape(new THREE.OctahedronGeometry(1.3, 1), 0x67e8f9, 0.2, [-4.0, -2.8, -6], [0.07, 0.06, 0.03], 0.16);
        addMorphingShape(new THREE.OctahedronGeometry(0.9, 1), 0xa5b4fc, 0.18, [5.8, -2.8, -3.5], [0.05, 0.04, 0.09], 0.14);
        addMorphingShape(new THREE.TorusKnotGeometry(0.92, 0.2, 100, 10), 0x818cf8, 0.32, [0.2, 1.2, -4.2], [0.08, 0.12, 0.05], 0.12);
        addMorphingShape(new THREE.DodecahedronGeometry(0.8, 1), 0x22d3ee, 0.2, [-2.2, 3.5, -6], [0.06, 0.03, 0.07], 0.18);
        addMorphingShape(new THREE.TetrahedronGeometry(0.7, 1), 0xc084fc, 0.24, [3.5, 3.0, -8], [0.1, 0.06, 0.04], 0.2);
        addMorphingShape(new THREE.TorusGeometry(0.6, 0.15, 12, 36), 0x67e8f9, 0.18, [-6.0, -0.5, -5], [0.04, 0.09, 0.06], 0.1);

        /* ── glow rings ── */
        addGlowRing(0x818cf8, [-5.5, 2.8, -4], 2.2, 48);
        addGlowRing(0x22d3ee, [5.0, 1.6, -5.5], 1.6, 48);
        addGlowRing(0xc084fc, [1.8, -3.4, -7], 1.9, 48);
        addGlowRing(0x67e8f9, [-4.0, -2.8, -6], 1.7, 36);
        addGlowRing(0x818cf8, [0.2, 1.2, -4.2], 1.4, 48);

        /* ── energy connections ── */
        addEnergyLine([-5.5, 2.8, -4], [0.2, 1.2, -4.2], 0x818cf8);
        addEnergyLine([0.2, 1.2, -4.2], [5.0, 1.6, -5.5], 0x22d3ee);
        addEnergyLine([-5.5, 2.8, -4], [-4.0, -2.8, -6], 0x67e8f9);
        addEnergyLine([5.0, 1.6, -5.5], [5.8, -2.8, -3.5], 0xa5b4fc);
        addEnergyLine([1.8, -3.4, -7], [-4.0, -2.8, -6], 0xc084fc);
        addEnergyLine([0.2, 1.2, -4.2], [1.8, -3.4, -7], 0xc084fc);
        addEnergyLine([-2.2, 3.5, -6], [-5.5, 2.8, -4], 0x22d3ee);
        addEnergyLine([3.5, 3.0, -8], [5.0, 1.6, -5.5], 0xc084fc);

        /* ── particles ── */
        createParticles();
        clock = new THREE.Clock();
        resize();
        return true;
    }

    /* ── resize ───────────────────────────────────────────────────── */

    function resize() {
        if (!renderer || !camera) return;
        const w = window.innerWidth, h = window.innerHeight;
        renderer.setSize(w, h, false);
        camera.aspect = w / h;
        camera.updateProjectionMatrix();
    }

    /* ── vertex morphing ──────────────────────────────────────────── */

    function morphVertices(mesh, t) {
        const geo = mesh.geometry;
        const posAttr = geo.getAttribute('position');
        const orig = mesh.userData.origPositions;
        if (!posAttr || !orig) return;

        const amp = mesh.userData.morphAmplitude;
        const phase = mesh.userData.morphPhase;
        const count = posAttr.count;
        const arr = posAttr.array;

        for (let i = 0; i < count; i++) {
            const ox = orig[i * 3], oy = orig[i * 3 + 1], oz = orig[i * 3 + 2];
            const len = Math.sqrt(ox * ox + oy * oy + oz * oz) || 1;
            const nx = ox / len, ny = oy / len, nz = oz / len;
            /* layered sine noise for organic morph */
            const noise =
                Math.sin(t * 1.3 + phase + ox * 2.5 + oy * 1.8) * 0.4 +
                Math.sin(t * 0.7 + phase * 1.5 + oz * 3.0 + ox * 1.2) * 0.35 +
                Math.sin(t * 2.1 + phase * 0.8 + oy * 2.8 + oz * 1.5) * 0.25;
            const disp = 1 + noise * amp;
            arr[i * 3] = ox * disp;
            arr[i * 3 + 1] = oy * disp;
            arr[i * 3 + 2] = oz * disp;
        }
        posAttr.needsUpdate = true;
        geo.computeVertexNormals();
    }

    /* ── render loop ──────────────────────────────────────────────── */

    function render(time) {
        if (!active || !renderer || !scene || !camera) return;
        animationId = requestAnimationFrame(render);
        if (time - lastFrame < 30) return; /* ~33 fps cap */
        lastFrame = time;

        const t = time * 0.001;

        /* morph shapes */
        shapes.forEach((mesh, idx) => {
            const sp = mesh.userData.rotationSpeed;
            mesh.rotation.x += sp[0] * 0.04;
            mesh.rotation.y += sp[1] * 0.04;
            mesh.rotation.z += sp[2] * 0.04;

            /* float */
            mesh.position.y += Math.sin(t * 0.7 + mesh.userData.floatOffset + idx) * 0.003;

            /* breathing scale */
            const breath = 1 + Math.sin(t * 0.5 + idx * 1.3) * 0.06;
            mesh.scale.setScalar(breath);

            /* vertex morphing */
            morphVertices(mesh, t);

            /* color shift */
            const hueShift = (Math.sin(t * 0.3 + idx * 0.7) * 0.08);
            mesh.material.opacity = mesh.material.opacity; // keep
        });

        /* glow rings pulse */
        glowRings.forEach((ring, idx) => {
            const pulse = (Math.sin(t * 1.2 + ring.userData.phase) + 1) * 0.5;
            ring.material.opacity = pulse * 0.18;
            const sc = 1 + Math.sin(t * 0.8 + idx) * 0.12;
            ring.scale.set(sc, sc, 1);
            ring.rotation.z = t * 0.1 + idx;
        });

        /* energy lines pulse and wave */
        energyLines.forEach((line, idx) => {
            const pulse = (Math.sin(t * 1.5 + line.userData.phase) + 1) * 0.5;
            line.material.opacity = pulse * 0.14;

            /* wave the midpoint vertices */
            const posAttr = line.geometry.getAttribute('position');
            const { from, to, segs } = line.userData;
            for (let i = 0; i <= segs; i++) {
                const frac = i / segs;
                posAttr.setX(i, from[0] + (to[0] - from[0]) * frac);
                posAttr.setY(i, from[1] + (to[1] - from[1]) * frac + Math.sin(t * 2.0 + frac * 6 + idx) * 0.25);
                posAttr.setZ(i, from[2] + (to[2] - from[2]) * frac + Math.sin(t * 1.5 + frac * 4 + idx * 0.5) * 0.15);
            }
            posAttr.needsUpdate = true;
        });

        /* particles drift */
        if (particles) {
            particles.rotation.y = t * 0.02;
            particles.rotation.x = Math.sin(t * 0.2) * 0.03;
            const posAttr = particles.geometry.getAttribute('position');
            const vels = particles.geometry.userData.velocities;
            const arr = posAttr.array;
            for (let i = 0; i < arr.length; i += 3) {
                arr[i] += vels[i] + Math.sin(t + i) * 0.0003;
                arr[i + 1] += vels[i + 1] + Math.cos(t * 0.8 + i) * 0.0003;
                arr[i + 2] += vels[i + 2];
                /* wrap bounds */
                if (arr[i] > 19) arr[i] = -19;
                if (arr[i] < -19) arr[i] = 19;
                if (arr[i + 1] > 11) arr[i + 1] = -11;
                if (arr[i + 1] < -11) arr[i + 1] = 11;
            }
            posAttr.needsUpdate = true;
        }

        /* dramatic camera */
        camera.position.x = Math.sin(t * 0.14) * 0.35;
        camera.position.y = Math.cos(t * 0.11) * 0.22;
        camera.position.z = 9 + Math.sin(t * 0.08) * 0.5;
        camera.lookAt(Math.sin(t * 0.06) * 0.4, Math.cos(t * 0.05) * 0.3, -4);

        renderer.render(scene, camera);
    }

    /* ── lifecycle ────────────────────────────────────────────────── */

    function start() {
        if (active || !shouldRun()) return;
        if (!renderer && !initScene()) return;
        active = true;
        canvas.style.display = 'block';
        resize();
        lastFrame = 0;
        animationId = requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) { cancelAnimationFrame(animationId); animationId = null; }
        if (canvas) canvas.style.display = 'none';
    }

    function sync() { shouldRun() ? start() : stop(); }

    function init() {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', init, { once: true });
            return;
        }
        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', () => { if (active) resize(); sync(); });
        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            if (mq.addEventListener) mq.addEventListener('change', sync);
            else if (mq.addListener) mq.addListener(sync);
        }
        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true, attributeFilter: ['data-theme']
            });
        }
        sync();
    }

    window.AuraGoThreeDee = { start, stop, sync };
    init();
})();