(function () {
    'use strict';

    const previews = new WeakMap();
    let observer = null;
    let mutationObserver = null;
    let modal = null;
    let stlAssetPromise = null;

    function t(key, fallback) {
        if (typeof window.t === 'function') {
            const value = window.t(key);
            if (value && value !== key) return value;
        }
        return fallback || key;
    }

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"]/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[ch]));
    }

    function safePath(path) {
        const raw = String(path || '').trim();
        if (!raw) return '';
        try {
            const parsed = new URL(raw, window.location.origin);
            if (parsed.origin !== window.location.origin) return '';
            if (!/\.stl$/i.test(parsed.pathname)) return '';
            return parsed.pathname + parsed.search;
        } catch (_err) {
            return raw.startsWith('/files/') && /\.stl(?:$|[?#])/i.test(raw) ? raw : '';
        }
    }

    function ensureSTLAssets() {
        if (window.THREE && THREE.STLLoader) return Promise.resolve();
        if (!window.AuraLazyAssets || typeof window.AuraLazyAssets.loadAll !== 'function') {
            return Promise.reject(new Error('Lazy asset loader is unavailable'));
        }
        if (!stlAssetPromise) {
            stlAssetPromise = window.AuraLazyAssets.loadAll({
                styles: ['/css/stl-viewer.css'],
                scripts: [
                    '/js/vendor/three.min.js',
                    '/js/vendor/STLLoader.min.js',
                    '/js/vendor/OrbitControls.min.js'
                ]
            }).catch(err => {
                stlAssetPromise = null;
                throw err;
            });
        }
        return stlAssetPromise;
    }

    function filenameFromPath(path) {
        try {
            const parsed = new URL(path, window.location.origin);
            return decodeURIComponent(parsed.pathname.split('/').pop() || '') || 'model.stl';
        } catch (_err) {
            return String(path || '').split('?')[0].split('/').pop() || 'model.stl';
        }
    }

    function createChatSTLCard(data, shouldObserve = true) {
        const path = safePath(data && data.path);
        if (!path) return document.createTextNode('');
        const title = (data && (data.title || data.filename)) || filenameFromPath(path);
        const wrapper = document.createElement('div');
        wrapper.className = 'chat-stl-card chat-document-card';
        wrapper.innerHTML = cardHTML({ path, title, filename: (data && data.filename) || filenameFromPath(path) });
        if (shouldObserve) observePreview(wrapper.querySelector('.chat-stl-preview'));
        return wrapper;
    }

    function cardHTML(data) {
        const expandIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup('expand') : '[ ]';
        const downloadIcon = window.chatUiIconMarkup ? window.chatUiIconMarkup('download') : '↓';
        return `<div class="chat-stl-preview" data-stl-path="${esc(data.path)}" title="${esc(t('stl_open_viewer', 'Open 3D viewer'))}">
                <canvas class="chat-stl-canvas" aria-label="${esc(t('stl_3d_model', 'STL 3D model'))}"></canvas>
                <div class="chat-stl-loading">${esc(t('stl_loading', 'Loading 3D model...'))}</div>
            </div>
            <div class="chat-stl-footer">
                <div class="chat-stl-info">
                    <div class="chat-stl-title">${esc(data.title)}</div>
                    <div class="chat-stl-format">${esc(t('stl_3d_model', 'STL 3D Model'))}</div>
                </div>
                <div class="chat-stl-actions chat-document-actions">
                    <button class="chat-stl-expand-btn" type="button" data-stl-path="${esc(data.path)}" data-stl-title="${esc(data.title)}" title="${esc(t('stl_expand', 'Expand'))}">${expandIcon}</button>
                    <a href="${esc(data.path)}" download="${esc(data.filename)}" title="${esc(t('stl_download', 'Download'))}">${downloadIcon}</a>
                </div>
            </div>`;
    }

    function renderSTLLinksAsPlayers(html) {
        const template = document.createElement('template');
        template.innerHTML = html;
        template.content.querySelectorAll('a[href]').forEach(anchor => {
            const href = safePath(anchor.getAttribute('href') || '');
            if (!href) return;
            if (typeof seenSSESTLs !== 'undefined' && seenSSESTLs.has(href)) {
                anchor.remove();
                return;
            }
            const holder = document.createElement('div');
            holder.appendChild(createChatSTLCard({ path: href, title: anchor.textContent || filenameFromPath(href) }, false));
            anchor.replaceWith(holder.firstChild);
        });

        const walker = document.createTreeWalker(template.content, NodeFilter.SHOW_TEXT);
        const textNodes = [];
        let node;
        while ((node = walker.nextNode())) {
            const parent = node.parentElement;
            if (!parent || parent.closest('a, code, pre, .chat-stl-card, .chat-video-wrapper, .chat-youtube-wrapper')) continue;
            if (/\/files\/[^\s<>()"']+\.stl(?:\?[^\s<>()"']*)?/i.test(node.nodeValue || '')) textNodes.push(node);
        }
        textNodes.forEach(textNode => {
            const text = textNode.nodeValue || '';
            const fragment = document.createDocumentFragment();
            const pattern = /\/files\/[^\s<>()"']+\.stl(?:\?[^\s<>()"']*)?/ig;
            let lastIndex = 0;
            let match;
            while ((match = pattern.exec(text)) !== null) {
                if (match.index > lastIndex) fragment.appendChild(document.createTextNode(text.slice(lastIndex, match.index)));
                const path = safePath(match[0]);
                if (path && !(typeof seenSSESTLs !== 'undefined' && seenSSESTLs.has(path))) {
                    fragment.appendChild(createChatSTLCard({ path, title: filenameFromPath(path) }, false));
                }
                lastIndex = match.index + match[0].length;
            }
            if (lastIndex < text.length) fragment.appendChild(document.createTextNode(text.slice(lastIndex)));
            textNode.parentNode.replaceChild(fragment, textNode);
        });
        return template.innerHTML;
    }

    function appendSTLMessage(data) {
        const card = createChatSTLCard(data || {});
        if (!card || !card.nodeType || card.nodeType !== 1) return false;
        const greet = document.querySelector('[data-greeting]');
        if (greet) greet.remove();
        const row = document.createElement('div');
        row.className = 'msg-row bot';
        const botIcon = typeof personaAvatarMarkup === 'function' ? personaAvatarMarkup('bot') : '';
        row.innerHTML = `<div class="avatar bot">${botIcon}</div><div class="message-stack"><div class="bubble bot"></div></div>`;
        row.querySelector('.bubble').appendChild(card);
        if (typeof appendMessageTimestamp === 'function') appendMessageTimestamp(row, 'bot');
        const chatContent = document.getElementById('chat-content');
        const chatBox = document.getElementById('chat-box');
        if (chatContent) chatContent.appendChild(row);
        if (chatBox) chatBox.scrollTop = chatBox.scrollHeight;
        return true;
    }

    function ensureObserver() {
        if (!observer && 'IntersectionObserver' in window) {
            observer = new IntersectionObserver(entries => {
                entries.forEach(entry => {
                    const preview = entry.target;
                    const instance = previews.get(preview);
                    if (entry.isIntersecting) {
                        if (instance) startPreview(instance);
                        else initPreview(preview);
                    } else if (instance) {
                        stopPreview(instance);
                    }
                });
            }, { rootMargin: '120px 0px', threshold: 0.01 });
        }
        if (!mutationObserver) {
            mutationObserver = new MutationObserver(() => observeAllPreviews());
            mutationObserver.observe(document.documentElement, { childList: true, subtree: true });
        }
    }

    function observeAllPreviews() {
        document.querySelectorAll('.chat-stl-preview:not([data-stl-observed])').forEach(observePreview);
    }

    function observePreview(preview) {
        if (!preview || preview.dataset.stlObserved) return;
        preview.dataset.stlObserved = '1';
        preview.addEventListener('dblclick', () => openSTLModal(preview.dataset.stlPath, preview.closest('.chat-stl-card')?.querySelector('.chat-stl-title')?.textContent));
        if (observer) observer.observe(preview);
        else initPreview(preview);
    }

    async function initPreview(preview) {
        if (!preview || previews.has(preview)) return;
        const path = safePath(preview.dataset.stlPath);
        if (!path) return;
        const canvas = preview.querySelector('canvas');
        const loading = preview.querySelector('.chat-stl-loading');
        try {
            await ensureSTLAssets();
        } catch (err) {
            if (loading) loading.textContent = err.message || t('viewer.error', 'Failed to load file');
            return;
        }
        if (!window.THREE || !THREE.STLLoader) return;
        const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
        renderer.setClearColor(0x000000, 0);
        const scene = new THREE.Scene();
        scene.add(new THREE.HemisphereLight(0xffffff, 0x2b3243, 0.9));
        const light = new THREE.DirectionalLight(0xffffff, 0.85);
        light.position.set(4, 5, 6);
        scene.add(light);
        const camera = new THREE.PerspectiveCamera(42, 1, 0.01, 100000);
        const material = new THREE.MeshStandardMaterial({ color: 0xa8c2e6, roughness: 0.5, metalness: 0.06, side: THREE.DoubleSide });
        const instance = { preview, renderer, scene, camera, material, mesh: null, raf: 0, active: false, dragging: false, lastX: 0, lastY: 0 };
        previews.set(preview, instance);
        wirePreviewDrag(instance);
        try {
            const geometry = await new Promise((resolve, reject) => new THREE.STLLoader().load(path, resolve, undefined, reject));
            const mesh = new THREE.Mesh(geometry, material);
            fitMesh(mesh, camera);
            scene.add(mesh);
            instance.mesh = mesh;
            if (loading) loading.remove();
            resizePreview(instance);
            startPreview(instance);
        } catch (err) {
            if (loading) loading.textContent = err.message || t('viewer.error', 'Failed to load file');
        }
    }

    function fitMesh(mesh, camera) {
        mesh.geometry.computeBoundingBox();
        const box = new THREE.Box3().setFromObject(mesh);
        const center = box.getCenter(new THREE.Vector3());
        const size = box.getSize(new THREE.Vector3());
        mesh.position.sub(center);
        const maxDim = Math.max(size.x, size.y, size.z, 1);
        const distance = maxDim / (2 * Math.tan((camera.fov * Math.PI / 180) / 2));
        camera.position.set(distance * 0.76, distance * 0.55, distance * 1.22);
        camera.near = Math.max(distance / 1000, 0.01);
        camera.far = Math.max(distance * 100, 1000);
        camera.updateProjectionMatrix();
        camera.lookAt(0, 0, 0);
    }

    function resizePreview(instance) {
        const rect = instance.preview.getBoundingClientRect();
        const w = Math.max(1, Math.floor(rect.width));
        const h = Math.max(1, Math.floor(rect.height));
        instance.renderer.setSize(w, h, false);
        instance.camera.aspect = w / h;
        instance.camera.updateProjectionMatrix();
    }

    function startPreview(instance) {
        if (!instance || instance.active) return;
        instance.active = true;
        const tick = () => {
            if (!instance.active) return;
            instance.raf = requestAnimationFrame(tick);
            resizePreview(instance);
            if (instance.mesh && !instance.dragging) instance.mesh.rotation.y += 0.005;
            instance.renderer.render(instance.scene, instance.camera);
        };
        tick();
    }

    function stopPreview(instance) {
        instance.active = false;
        if (instance.raf) cancelAnimationFrame(instance.raf);
        instance.raf = 0;
    }

    function wirePreviewDrag(instance) {
        const canvas = instance.renderer.domElement;
        canvas.addEventListener('pointerdown', event => {
            instance.dragging = true;
            instance.lastX = event.clientX;
            instance.lastY = event.clientY;
            canvas.setPointerCapture(event.pointerId);
        });
        canvas.addEventListener('pointermove', event => {
            if (!instance.dragging || !instance.mesh) return;
            const dx = event.clientX - instance.lastX;
            const dy = event.clientY - instance.lastY;
            instance.mesh.rotation.y += dx * 0.012;
            instance.mesh.rotation.x += dy * 0.012;
            instance.lastX = event.clientX;
            instance.lastY = event.clientY;
        });
        canvas.addEventListener('pointerup', () => { instance.dragging = false; });
        canvas.addEventListener('pointercancel', () => { instance.dragging = false; });
    }

    async function openSTLModal(path, title) {
        path = safePath(path);
        if (!path) return;
        try {
            await ensureSTLAssets();
        } catch (_err) {
            return;
        }
        if (!window.THREE || !THREE.STLLoader) return;
        closeSTLModal();
        modal = document.createElement('div');
        modal.className = 'stl-modal-backdrop';
        modal.innerHTML = `<div class="stl-modal-content" role="dialog" aria-modal="true">
            <div class="stl-modal-toolbar">
                <strong>${esc(title || filenameFromPath(path))}</strong>
                <div class="stl-modal-actions">
                    <button type="button" data-action="wireframe">${esc(t('stl_wireframe', 'Wireframe'))}</button>
                    <button type="button" class="is-active" data-action="rotate">${esc(t('stl_auto_rotate', 'Auto-rotate'))}</button>
                    <a href="${esc(path)}" download="${esc(filenameFromPath(path))}">${esc(t('stl_download', 'Download'))}</a>
                    <button type="button" data-action="close" aria-label="${esc(t('stl_close', 'Close'))}">×</button>
                </div>
            </div>
            <div class="stl-modal-stage"><canvas class="stl-modal-canvas"></canvas><div class="stl-modal-loading">${esc(t('stl_loading', 'Loading 3D model...'))}</div></div>
        </div>`;
        document.body.appendChild(modal);
        initModalViewer(modal, path);
    }

    async function initModalViewer(root, path) {
        const canvas = root.querySelector('canvas');
        const stage = root.querySelector('.stl-modal-stage');
        const loading = root.querySelector('.stl-modal-loading');
        const renderer = new THREE.WebGLRenderer({ canvas, antialias: true, alpha: true });
        renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, 2));
        renderer.setClearColor(0x000000, 0);
        const scene = new THREE.Scene();
        scene.add(new THREE.HemisphereLight(0xffffff, 0x222838, 0.9));
        const key = new THREE.DirectionalLight(0xffffff, 0.95);
        key.position.set(5, 7, 8);
        scene.add(key);
        const camera = new THREE.PerspectiveCamera(45, 1, 0.01, 100000);
        const material = new THREE.MeshStandardMaterial({ color: 0xb3c8ea, roughness: 0.42, metalness: 0.08, side: THREE.DoubleSide });
        let autoRotate = true;
        let raf = 0;
        let controls = null;
        let mesh = null;
        let disposed = false;
        const cleanup = () => {
            disposed = true;
            if (raf) cancelAnimationFrame(raf);
            if (controls && controls.dispose) controls.dispose();
            if (mesh && mesh.geometry) mesh.geometry.dispose();
            material.dispose();
            renderer.dispose();
        };
        root._stlCleanup = cleanup;
        root.querySelector('[data-action="close"]').addEventListener('click', closeSTLModal);
        root.addEventListener('click', event => { if (event.target === root) closeSTLModal(); });
        root.querySelector('[data-action="wireframe"]').addEventListener('click', event => {
            material.wireframe = !material.wireframe;
            event.currentTarget.classList.toggle('is-active', material.wireframe);
        });
        root.querySelector('[data-action="rotate"]').addEventListener('click', event => {
            autoRotate = !autoRotate;
            event.currentTarget.classList.toggle('is-active', autoRotate);
        });
        try {
            const geometry = await new Promise((resolve, reject) => new THREE.STLLoader().load(path, resolve, undefined, reject));
            if (disposed) {
                geometry.dispose();
                return;
            }
            mesh = new THREE.Mesh(geometry, material);
            fitMesh(mesh, camera);
            scene.add(mesh);
            if (THREE.OrbitControls) {
                controls = new THREE.OrbitControls(camera, canvas);
                controls.enableDamping = true;
                controls.dampingFactor = 0.08;
                controls.autoRotateSpeed = 1.4;
                controls.update();
            }
            if (loading) loading.remove();
            const tick = () => {
                raf = requestAnimationFrame(tick);
                const rect = stage.getBoundingClientRect();
                renderer.setSize(Math.max(1, rect.width), Math.max(1, rect.height), false);
                camera.aspect = Math.max(1, rect.width) / Math.max(1, rect.height);
                camera.updateProjectionMatrix();
                if (controls) {
                    controls.autoRotate = autoRotate;
                    controls.update();
                } else if (autoRotate && mesh) mesh.rotation.y += 0.006;
                renderer.render(scene, camera);
            };
            tick();
        } catch (err) {
            if (loading) loading.textContent = err.message || t('viewer.error', 'Failed to load file');
        }
    }

    function closeSTLModal() {
        if (!modal) return;
        if (modal._stlCleanup) modal._stlCleanup();
        modal.remove();
        modal = null;
    }

    document.addEventListener('click', event => {
        const btn = event.target.closest && event.target.closest('.chat-stl-expand-btn');
        if (btn) {
            event.preventDefault();
            openSTLModal(btn.dataset.stlPath, btn.dataset.stlTitle);
        }
    });
    document.addEventListener('keydown', event => { if (event.key === 'Escape') closeSTLModal(); });
    window.addEventListener('load', () => { ensureObserver(); observeAllPreviews(); });
    ensureObserver();

    window.createChatSTLCard = createChatSTLCard;
    window.renderSTLLinksAsPlayers = renderSTLLinksAsPlayers;
    window.appendSTLMessage = appendSTLMessage;
    window.openSTLModal = openSTLModal;
})();
