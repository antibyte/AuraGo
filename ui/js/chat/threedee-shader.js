(function () {
    'use strict';

    let canvas;
    let renderer;
    let scene;
    let camera;
    let animationId = null;
    let active = false;
    let lastFrame = 0;
    let shapes = [];
    let particles;

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
            position: 'fixed',
            inset: '0',
            width: '100vw',
            height: '100vh',
            pointerEvents: 'none',
            zIndex: '0',
            opacity: '0.86',
            mixBlendMode: 'screen',
            display: 'none'
        });
        document.body.appendChild(canvas);
    }

    function material(color, opacity) {
        return new THREE.MeshBasicMaterial({
            color,
            wireframe: true,
            transparent: true,
            opacity,
            depthWrite: false
        });
    }

    function addShape(geometry, color, opacity, position, rotationSpeed) {
        const mesh = new THREE.Mesh(geometry, material(color, opacity));
        mesh.position.set(position[0], position[1], position[2]);
        mesh.userData.rotationSpeed = rotationSpeed;
        mesh.userData.floatOffset = Math.random() * Math.PI * 2;
        scene.add(mesh);
        shapes.push(mesh);
    }

    function createParticles() {
        const count = window.innerWidth < 1100 ? 280 : 500;
        const positions = new Float32Array(count * 3);
        for (let i = 0; i < count; i += 1) {
            positions[i * 3] = (Math.random() - 0.5) * 34;
            positions[i * 3 + 1] = (Math.random() - 0.5) * 20;
            positions[i * 3 + 2] = -Math.random() * 30 - 2;
        }

        const geometry = new THREE.BufferGeometry();
        geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
        const particleMaterial = new THREE.PointsMaterial({
            color: 0xa5b4fc,
            size: 0.035,
            transparent: true,
            opacity: 0.55,
            depthWrite: false
        });
        particles = new THREE.Points(geometry, particleMaterial);
        scene.add(particles);
    }

    function initScene() {
        if (!window.THREE) {
            console.warn('[ThreeDeeShader] Three.js not available');
            return false;
        }
        if (!canvas) createCanvas();

        try {
            renderer = new THREE.WebGLRenderer({
                canvas,
                alpha: true,
                antialias: true,
                powerPreference: 'low-power'
            });
        } catch (err) {
            console.warn('[ThreeDeeShader] WebGL renderer unavailable:', err);
            return false;
        }

        renderer.setClearColor(0x000000, 0);
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));

        scene = new THREE.Scene();
        scene.fog = new THREE.FogExp2(0x070b14, 0.015);
        camera = new THREE.PerspectiveCamera(60, window.innerWidth / window.innerHeight, 0.1, 1000);
        camera.position.z = 8;

        scene.add(new THREE.AmbientLight(0x111122, 0.35));
        const indigo = new THREE.PointLight(0x818cf8, 0.8, 30);
        indigo.position.set(-5, 3, 5);
        scene.add(indigo);
        const cyan = new THREE.PointLight(0x06b6d4, 0.5, 26);
        cyan.position.set(6, -2, 4);
        scene.add(cyan);

        addShape(new THREE.IcosahedronGeometry(1.65, 1), 0x818cf8, 0.3, [-5.2, 2.8, -4], [0.09, 0.05, 0.04]);
        addShape(new THREE.IcosahedronGeometry(1.05, 1), 0x22d3ee, 0.24, [4.7, 1.5, -5.5], [0.04, 0.08, 0.06]);
        addShape(new THREE.IcosahedronGeometry(1.35, 1), 0xc084fc, 0.18, [1.8, -3.2, -7], [0.06, 0.04, 0.08]);
        addShape(new THREE.OctahedronGeometry(1.25, 0), 0x67e8f9, 0.22, [-3.8, -2.6, -6], [0.07, 0.06, 0.03]);
        addShape(new THREE.OctahedronGeometry(0.85, 0), 0xa5b4fc, 0.2, [5.5, -2.7, -3.5], [0.05, 0.04, 0.09]);
        addShape(new THREE.TorusKnotGeometry(0.86, 0.18, 90, 8), 0x818cf8, 0.34, [0.2, 1.1, -4.2], [0.08, 0.12, 0.05]);
        createParticles();
        resize();
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

    function render(time) {
        if (!active || !renderer || !scene || !camera) return;
        animationId = window.requestAnimationFrame(render);
        if (time - lastFrame < 33) return;
        lastFrame = time;

        const seconds = time * 0.001;
        shapes.forEach((mesh, index) => {
            const speed = mesh.userData.rotationSpeed;
            mesh.rotation.x += speed[0] * 0.04;
            mesh.rotation.y += speed[1] * 0.04;
            mesh.rotation.z += speed[2] * 0.04;
            mesh.position.y += Math.sin(seconds * 0.7 + mesh.userData.floatOffset + index) * 0.0025;
        });

        if (particles) {
            particles.rotation.y = seconds * 0.018;
            particles.rotation.x = Math.sin(seconds * 0.2) * 0.025;
        }
        camera.position.x = Math.sin(seconds * 0.12) * 0.18;
        camera.position.y = Math.cos(seconds * 0.1) * 0.12;
        camera.lookAt(0, 0, -4);
        renderer.render(scene, camera);
    }

    function start() {
        if (active || !shouldRun()) return;
        if (!renderer && !initScene()) return;
        active = true;
        canvas.style.display = 'block';
        resize();
        lastFrame = 0;
        animationId = window.requestAnimationFrame(render);
    }

    function stop() {
        active = false;
        if (animationId) {
            window.cancelAnimationFrame(animationId);
            animationId = null;
        }
        if (canvas) canvas.style.display = 'none';
    }

    function sync() {
        if (shouldRun()) start();
        else stop();
    }

    function init() {
        if (document.readyState === 'loading') {
            document.addEventListener('DOMContentLoaded', init, { once: true });
            return;
        }
        window.addEventListener('aurago:themechange', sync);
        window.addEventListener('resize', () => {
            if (active) resize();
            sync();
        });
        if (window.matchMedia) {
            const mq = window.matchMedia('(prefers-reduced-motion: reduce)');
            if (mq.addEventListener) mq.addEventListener('change', sync);
            else if (mq.addListener) mq.addListener(sync);
        }
        if (typeof MutationObserver !== 'undefined') {
            new MutationObserver(sync).observe(document.documentElement, {
                attributes: true,
                attributeFilter: ['data-theme']
            });
        }
        sync();
    }

    window.AuraGoThreeDee = { start, stop, sync };
    init();
})();
