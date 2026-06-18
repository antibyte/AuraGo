(function () {
    'use strict';

    function render(host, windowId, context) {
        const ctx = context || {};
        const esc = typeof ctx.esc === 'function' ? ctx.esc : function (s) { return String(s || '').replace(/[&<>"']/g, function (m) { return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[m]; }); };
        const t = typeof ctx.t === 'function' ? ctx.t : function (key, fallback) { return fallback !== undefined ? fallback : key; };
        const api = typeof ctx.api === 'function' ? ctx.api : function () { return Promise.reject(new Error('api not available')); };
        const notify = typeof ctx.notify === 'function' ? ctx.notify : function () {};

        host.innerHTML = `
            <div class="vd-pet-picker">
                <div class="vd-pet-picker-header">
                    <h2>${esc(t('desktop.pet_picker_title', 'Pet Picker'))}</h2>
                    <p class="vd-pet-picker-subtitle">${esc(t('desktop.pet_picker_subtitle', 'Choose your desktop companion'))}</p>
                </div>
                <div class="vd-pet-picker-grid"></div>
                <div class="vd-pet-picker-settings">
                    <label class="vd-pet-picker-setting">
                        <span>${esc(t('desktop.pet_scale', 'Size'))}</span>
                        <input type="range" class="vd-pet-picker-scale" min="0.5" max="2" step="0.1" value="1">
                        <span class="vd-pet-picker-scale-value">1.0x</span>
                    </label>
                    <label class="vd-pet-picker-setting">
                        <span>${esc(t('desktop.pet_enabled', 'Show pet'))}</span>
                        <input type="checkbox" class="vd-pet-picker-enabled" checked>
                    </label>
                    <label class="vd-pet-picker-setting">
                        <span>${esc(t('desktop.pet_always_on_top', 'Always on top'))}</span>
                        <input type="checkbox" class="vd-pet-picker-always-on-top">
                    </label>
                </div>
                <div class="vd-pet-picker-actions">
                    <button type="button" class="vd-pet-picker-import vd-btn-primary">${esc(t('desktop.pet_import', 'Import ZIP'))}</button>
                    <input type="file" class="vd-pet-picker-file" accept=".zip" hidden>
                </div>
            </div>
        `;

        const grid = host.querySelector('.vd-pet-picker-grid');
        const scaleInput = host.querySelector('.vd-pet-picker-scale');
        const scaleValue = host.querySelector('.vd-pet-picker-scale-value');
        const enabledInput = host.querySelector('.vd-pet-picker-enabled');
        const alwaysOnTopInput = host.querySelector('.vd-pet-picker-always-on-top');
        const importBtn = host.querySelector('.vd-pet-picker-import');
        const fileInput = host.querySelector('.vd-pet-picker-file');

        let pets = [];
        let activeId = '';

        function petAssetURL(id, relPath) {
            const cleanPath = String(relPath || 'spritesheet.webp')
                .replace(/\\/g, '/')
                .split('/')
                .map(part => part.trim())
                .filter(part => part && part !== '.' && part !== '..')
                .map(encodeURIComponent)
                .join('/');
            if (!id || !cleanPath) return '';
            return '/files/desktop/Pets/' + encodeURIComponent(id) + '/' + cleanPath;
        }

        function syncPetBootstrap(payload) {
            if (window.PetRuntime && typeof window.PetRuntime.syncBootstrap === 'function') {
                window.PetRuntime.syncBootstrap(payload);
            }
        }

        async function load() {
            try {
                const [petsBody, settingsBody] = await Promise.all([
                    api('/api/desktop/pets'),
                    api('/api/desktop/settings')
                ]);
                pets = petsBody.pets || [];
                activeId = petsBody.active_pet_id || '';
                renderGrid();
                const settings = settingsBody.settings || {};
                scaleInput.value = parseFloat(settings['pet.scale'] || '1');
                scaleValue.textContent = Number(scaleInput.value).toFixed(1) + 'x';
                enabledInput.checked = String(settings['pet.enabled']).toLowerCase() !== 'false';
                alwaysOnTopInput.checked = String(settings['pet.always_on_top']).toLowerCase() === 'true';
                syncPetBootstrap({ pets, active_pet_id: activeId, settings });
                if (window.PetRuntime && typeof window.PetRuntime.load === 'function') window.PetRuntime.load();
            } catch (err) {
                notify({ title: t('desktop.notification'), message: err.message });
            }
        }

        function renderGrid() {
            if (!pets.length) {
                grid.innerHTML = `<div class="vd-pet-picker-empty">${esc(t('desktop.pet_empty', 'No pets installed yet. Import one below.'))}</div>`;
                return;
            }
            grid.innerHTML = pets.map(pet => `
                <div class="vd-pet-picker-card${pet.id === activeId ? ' is-active' : ''}" data-id="${esc(pet.id)}">
                    <div class="vd-pet-picker-thumb" style="background-image:url('${petAssetURL(pet.id, pet.spritesheet)}')"></div>
                    <div class="vd-pet-picker-name">${esc(pet.display_name || pet.id)}</div>
                    <div class="vd-pet-picker-desc">${esc(pet.description || '')}</div>
                    <button type="button" class="vd-pet-picker-select vd-btn-secondary" data-id="${esc(pet.id)}">${esc(t('desktop.pet_select', 'Select'))}</button>
                </div>
            `).join('');
            grid.querySelectorAll('.vd-pet-picker-select').forEach(btn => {
                btn.addEventListener('click', () => activatePet(btn.dataset.id));
            });
        }

        async function activatePet(id) {
            try {
                const body = await api('/api/desktop/pets?action=activate', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id })
                });
                activeId = body.active_pet_id || id;
                syncPetBootstrap({ active_pet_id: body.active_pet_id || id });
                renderGrid();
                if (window.PetRuntime && typeof window.PetRuntime.load === 'function') window.PetRuntime.load();
            } catch (err) {
                notify({ title: t('desktop.notification'), message: err.message });
            }
        }

        async function saveSetting(key, value) {
            try {
                const body = await api('/api/desktop/settings', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ key, value })
                });
                syncPetBootstrap({ settings: body.settings || { [key]: value } });
                if (window.PetRuntime && typeof window.PetRuntime.load === 'function') window.PetRuntime.load();
            } catch (err) {
                notify({ title: t('desktop.notification'), message: err.message });
            }
        }

        scaleInput.addEventListener('input', () => {
            scaleValue.textContent = Number(scaleInput.value).toFixed(1) + 'x';
        });
        scaleInput.addEventListener('change', () => {
            saveSetting('pet.scale', Number(scaleInput.value).toFixed(2));
        });
        enabledInput.addEventListener('change', () => {
            saveSetting('pet.enabled', String(enabledInput.checked));
        });
        alwaysOnTopInput.addEventListener('change', () => {
            saveSetting('pet.always_on_top', String(alwaysOnTopInput.checked));
        });

        importBtn.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', async () => {
            const file = fileInput.files[0];
            if (!file) return;
            const id = file.name.replace(/\.zip$/i, '').toLowerCase().replace(/[^a-z0-9_-]+/g, '-').replace(/^-|-$/g, '');
            if (!id) {
                notify({ title: t('desktop.notification'), message: t('desktop.pet_import_invalid', 'Invalid file name') });
                return;
            }
            const arrayBuffer = await file.arrayBuffer();
            const bytes = new Uint8Array(arrayBuffer);
            const binary = Array.from(bytes, b => String.fromCharCode(b)).join('');
            try {
                await api('/api/desktop/pets?action=install', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ id, files: { 'data.zip': btoa(binary) } })
                });
                await load();
            } catch (err) {
                notify({ title: t('desktop.notification'), message: err.message });
            }
            fileInput.value = '';
        });

        load();
    }

    function dispose() {}

    window.PetPickerApp = { render, dispose };
})();
