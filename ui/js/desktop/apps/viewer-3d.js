(function () {
    'use strict';

    const instances = new Map();

    function render(host, windowId, context) {
        if (!host) return;
        dispose(windowId);
        const ctx = context || {};
        const esc = ctx.esc || escapeHtml;
        const rawT = ctx.t || ((key) => key);
        const t = (key, fallback) => {
            const value = rawT(key);
            return value && value !== key ? value : (fallback || key);
        };
        const iconMarkup = ctx.iconMarkup || ((key, fallback) => `<span>${esc(fallback || key || '')}</span>`);
        const notify = ctx.notify || (() => {});
        const currentPath = ctx.path || '';
        const fileName = currentPath.split('/').pop() || currentPath || t('stl_viewer', 'STL Viewer');
        const fileUrl = '/api/desktop/download?path=' + encodeURIComponent(currentPath);

        host.innerHTML = `<div class="vd-viewer vd-viewer-3d" data-viewer-3d="${esc(windowId)}">
            <div class="vd-viewer-toolbar">
                <div class="vd-viewer-toolbar-left">
                    ${iconMarkup('theme-threedee', '3D', 'vd-viewer-file-icon', 20)}
                    <span class="vd-viewer-filename">${esc(fileName)}</span>
                </div>
                <div class="vd-viewer-toolbar-right">
                    <button class="vd-tool-button" type="button" data-action="wireframe">${iconMarkup('grid', 'W')}<span>${esc(t('stl_wireframe', 'Wireframe'))}</span></button>
                    <button class="vd-tool-button is-active" type="button" data-action="rotate">${iconMarkup('refresh', 'R')}<span>${esc(t('stl_auto_rotate', 'Auto-rotate'))}</span></button>
                    <button class="vd-tool-button" type="button" data-action="fullscreen">${iconMarkup('maximize', 'F')}<span>${esc(t('stl_expand', 'Fullscreen'))}</span></button>
                    <button class="vd-tool-button" type="button" data-action="download">${iconMarkup('download', 'D')}<span>${esc(t('stl_download', 'Download'))}</span></button>
                </div>
            </div>
            <div class="vd-viewer-3d-stage" data-stage>
                <div class="vd-viewer-loading" data-loading>${esc(t('stl_loading', 'Loading 3D model...'))}</div>
            </div>
        </div>`;

        const stage = host.querySelector('[data-stage]');
        const loading = host.querySelector('[data-loading]');
        const record = { host, renderer: null, scene: null, camera: null, controls: null, mesh: null, material: null, resizeObserver: null, raf: 0, autoRotate: true, disposed: false };
        instances.set(windowId, record);

        host.querySelectorAll('[data-action]').forEach(btn => {
            btn.addEventListener('click', () => handleAction(btn.dataset.action, btn));
        });
        if (typeof ctx.wireContextMenuBoundary === 'function') ctx.wireContextMenuBoundary(host);

        init().catch(err => {
            if (loading) loading.textContent = (t('viewer.error', 'Failed to load file') + ': ' + err.message);
            notify(t('viewer.error', 'Failed to load file') + ': ' + err.message);
        });

        async function init() {
            if (!window.THREE || !THREE.STLLoader) throw new Error('Three.js STLLoader is unavailable');
            const renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
            renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
            renderer.setClearColor(0x000000, 0);
            renderer.domElement.className = 'vd-viewer-3d-canvas';
            stage.appendChild(renderer.domElement);
            record.renderer = renderer;

            const scene = new THREE.Scene();
            record.scene = scene;
            scene.add(new THREE.HemisphereLight(0xf4f7ff, 0x273044, 0.82));
            const key = new THREE.DirectionalLight(0xffffff, 0.95);
            key.position.set(4, 6, 8);
            scene.add(key);
            const fill = new THREE.DirectionalLight(0x9fc7ff, 0.45);
            fill.position.set(-5, 2, -3);
            scene.add(fill);

            const camera = new THREE.PerspectiveCamera(45, 1, 0.01, 100000);
            const material = new THREE.MeshStandardMaterial({ color: 0x9fb7d7, roughness: 0.46, metalness: 0.08 });
            Object.assign(record, { camera, material });
            const geometry = await loadGeometry(fileUrl);
            if (record.disposed) {
                geometry.dispose();
                return;
            }
            geometry.computeBoundingBox();
            geometry.computeBoundingSphere();
            const mesh = new THREE.Mesh(geometry, material);
            centerGeometry(mesh, camera);
            scene.add(mesh);

            let controls = null;
            if (THREE.OrbitControls) {
                controls = new THREE.OrbitControls(camera, renderer.domElement);
                controls.enableDamping = true;
                controls.dampingFactor = 0.08;
                controls.autoRotate = true;
                controls.autoRotateSpeed = 1.5;
                controls.target.set(0, 0, 0);
                controls.update();
            }

            Object.assign(record, { controls, mesh });
            if (loading) loading.remove();
            resize();
            record.resizeObserver = new ResizeObserver(resize);
            record.resizeObserver.observe(stage);
            animate();
        }

        function loadGeometry(url) {
            return new Promise((resolve, reject) => {
                new THREE.STLLoader().load(url, resolve, undefined, reject);
            });
        }

        function centerGeometry(mesh, camera) {
            const box = new THREE.Box3().setFromObject(mesh);
            const center = box.getCenter(new THREE.Vector3());
            const size = box.getSize(new THREE.Vector3());
            mesh.position.sub(center);
            const maxDim = Math.max(size.x, size.y, size.z, 1);
            const distance = maxDim / (2 * Math.tan((camera.fov * Math.PI / 180) / 2));
            camera.position.set(distance * 0.72, distance * 0.58, distance * 1.18);
            camera.near = Math.max(distance / 1000, 0.01);
            camera.far = Math.max(distance * 100, 1000);
            camera.updateProjectionMatrix();
            camera.lookAt(0, 0, 0);
        }

        function resize() {
            if (!record.renderer || !record.camera) return;
            const rect = stage.getBoundingClientRect();
            const width = Math.max(1, Math.floor(rect.width));
            const height = Math.max(1, Math.floor(rect.height));
            record.renderer.setSize(width, height, false);
            record.camera.aspect = width / height;
            record.camera.updateProjectionMatrix();
        }

        function animate() {
            if (record.disposed) return;
            record.raf = requestAnimationFrame(animate);
            if (record.controls) {
                record.controls.autoRotate = record.autoRotate;
                record.controls.update();
            } else if (record.autoRotate && record.mesh) {
                record.mesh.rotation.y += 0.008;
            }
            record.renderer.render(record.scene, record.camera);
        }

        function handleAction(action, btn) {
            if (action === 'download') return downloadFile();
            if (action === 'wireframe' && record.material) {
                record.material.wireframe = !record.material.wireframe;
                btn.classList.toggle('is-active', record.material.wireframe);
                return;
            }
            if (action === 'rotate') {
                record.autoRotate = !record.autoRotate;
                btn.classList.toggle('is-active', record.autoRotate);
                return;
            }
            if (action === 'fullscreen' && stage.requestFullscreen) stage.requestFullscreen();
        }

        function downloadFile() {
            const link = document.createElement('a');
            link.href = fileUrl;
            link.download = fileName;
            document.body.appendChild(link);
            link.click();
            link.remove();
        }
    }

    function dispose(windowId) {
        const record = instances.get(windowId);
        if (!record) return;
        record.disposed = true;
        if (record.raf) cancelAnimationFrame(record.raf);
        if (record.resizeObserver) record.resizeObserver.disconnect();
        if (record.controls && record.controls.dispose) record.controls.dispose();
        if (record.mesh && record.mesh.geometry) record.mesh.geometry.dispose();
        if (record.material) record.material.dispose();
        if (record.renderer) {
            record.renderer.dispose();
            if (record.renderer.domElement) record.renderer.domElement.remove();
        }
        instances.delete(windowId);
    }

    function escapeHtml(value) {
        return String(value == null ? '' : value).replace(/[&<>"]/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[ch]));
    }

    window.Viewer3DApp = { render, dispose };
})();
