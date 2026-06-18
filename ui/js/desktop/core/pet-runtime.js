(function () {
    'use strict';

    const PET_FRAME_W = 192;
    const PET_FRAME_H = 208;
    const PET_COLUMNS = 8;
    const PET_ROWS = 9;
    const BUBBLE_DURATION_MS = 4000;
    const LONG_BUBBLE_DURATION_MS = 8000;
    const MAX_BUBBLE_CHARS = 140;

    // OpenPets-compatible reaction → animation state mapping.
    const reactionToState = {
        idle: 'idle',
        thinking: 'review',
        working: 'running',
        editing: 'running',
        running: 'running',
        testing: 'waiting',
        waiting: 'waiting',
        waving: 'waving',
        success: 'jumping',
        error: 'failed',
        celebrating: 'jumping'
    };

    // Row indices in the spritesheet (OpenPets default layout).
    const stateRows = {
        idle: { row: 0, frames: 6, duration: 5500, iterations: 'infinite' },
        'running-right': { row: 1, frames: 8, duration: 1060 },
        'running-left': { row: 2, frames: 8, duration: 1060 },
        waving: { row: 3, frames: 4, duration: 700, iterations: 2 },
        jumping: { row: 4, frames: 5, duration: 840, iterations: 2 },
        failed: { row: 5, frames: 8, duration: 1220, iterations: 2 },
        waiting: { row: 6, frames: 6, duration: 1010 },
        running: { row: 7, frames: 6, duration: 820 },
        review: { row: 8, frames: 6, duration: 1030 }
    };

    let layer = null;
    let spriteEl = null;
    let bubbleEl = null;
    let drag = null;
    let currentState = 'idle';
    let bubbleTimer = null;
    let loadedPetId = null;

    function petEnabled() {
        return String(settingValue('pet.enabled')).toLowerCase() !== 'false';
    }

    function petScale() {
        const raw = settingValue('pet.scale') || '1.0';
        const v = parseFloat(raw);
        if (!Number.isFinite(v) || v < 0.25 || v > 3) return 1;
        return v;
    }

    function petPosition() {
        const x = parseInt(settingValue('pet.position_x') || '24', 10);
        const y = parseInt(settingValue('pet.position_y') || '24', 10);
        return {
            x: Number.isFinite(x) ? x : 24,
            y: Number.isFinite(y) ? y : 24
        };
    }

    function petAlwaysOnTop() {
        return String(settingValue('pet.always_on_top')).toLowerCase() === 'true';
    }

    function activePet() {
        const boot = state.bootstrap || {};
        const id = boot.active_pet_id || settingValue('pet.active_id') || '';
        if (!id) return null;
        const pets = boot.pets || [];
        return pets.find(p => p.id === id) || null;
    }

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

    function spritesheetURL(pet) {
        if (!pet || !pet.spritesheet) return '';
        return petAssetURL(pet.id, pet.spritesheet);
    }

    function syncPetBootstrap(payload) {
        if (!state.bootstrap || !payload) return;
        if (Array.isArray(payload.pets)) {
            state.bootstrap.pets = payload.pets;
        }
        if (payload.settings) {
            state.bootstrap.settings = Object.assign({}, state.bootstrap.settings || {}, payload.settings);
        }
        if (Object.prototype.hasOwnProperty.call(payload, 'active_pet_id')) {
            state.bootstrap.active_pet_id = payload.active_pet_id || '';
            const settings = Object.assign({}, state.bootstrap.settings || {});
            settings['pet.active_id'] = state.bootstrap.active_pet_id;
            state.bootstrap.settings = settings;
        }
    }

    function clampToViewport(rect) {
        const vw = window.innerWidth;
        const vh = window.innerHeight;
        const minVisible = 48;
        return {
            x: Math.max(minVisible - rect.w, Math.min(rect.x, vw - minVisible)),
            y: Math.max(0, Math.min(rect.y, vh - minVisible))
        };
    }

    function ensureLayer() {
        if (layer) return;
        layer = document.createElement('div');
        layer.id = 'vd-pet-layer';
        layer.className = 'vd-pet-layer';
        layer.innerHTML = '<div class="vd-pet-sprite" role="img" aria-label="Desktop pet"></div><div class="vd-pet-bubble" hidden></div>';
        document.body.appendChild(layer);
        spriteEl = layer.querySelector('.vd-pet-sprite');
        bubbleEl = layer.querySelector('.vd-pet-bubble');
        wireDrag();
    }

    function removeLayer() {
        if (!layer) return;
        layer.remove();
        layer = null;
        spriteEl = null;
        bubbleEl = null;
        if (bubbleTimer) {
            clearTimeout(bubbleTimer);
            bubbleTimer = null;
        }
    }

    function updateLayerZIndex() {
        if (!layer) return;
        layer.dataset.alwaysOnTop = petAlwaysOnTop() ? 'true' : 'false';
    }

    function applyPosition() {
        if (!layer) return;
        const pos = petPosition();
        const scale = petScale();
        const w = Math.round(PET_FRAME_W * scale);
        const h = Math.round(PET_FRAME_H * scale);
        const clamped = clampToViewport({ x: pos.x, y: pos.y, w, h });
        layer.style.left = clamped.x + 'px';
        layer.style.top = clamped.y + 'px';
        layer.style.width = w + 'px';
        layer.style.height = h + 'px';
        if (spriteEl) spriteEl.style.transform = 'scale(' + scale + ')';
    }

    function setSpriteState(stateId) {
        if (!spriteEl) return;
        const def = stateRows[stateId] || stateRows.idle;
        currentState = stateId;
        spriteEl.dataset.state = stateId;
        spriteEl.style.setProperty('--pet-row', String(def.row));
        spriteEl.style.setProperty('--pet-frames', String(def.frames));
        spriteEl.style.setProperty('--pet-duration', def.duration + 'ms');
        spriteEl.style.setProperty('--pet-iterations', String(def.iterations || 'infinite'));
        // Reset animation to restart from first frame.
        spriteEl.style.animation = 'none';
        void spriteEl.offsetWidth;
        spriteEl.style.animation = '';
    }

    function loadPet() {
        const pet = activePet();
        if (!pet || !petEnabled()) {
            removeLayer();
            loadedPetId = null;
            return;
        }
        ensureLayer();
        const url = spritesheetURL(pet);
        if (!url) {
            removeLayer();
            return;
        }
        if (loadedPetId !== pet.id) {
            loadedPetId = pet.id;
            spriteEl.style.backgroundImage = 'url("' + url + '")';
            spriteEl.setAttribute('aria-label', pet.display_name || pet.id);
        }
        applyPosition();
        updateLayerZIndex();
        setSpriteState('idle');
    }

    function showBubble(message, type) {
        if (!bubbleEl) return;
        if (bubbleTimer) {
            clearTimeout(bubbleTimer);
            bubbleTimer = null;
        }
        const text = String(message || '').slice(0, MAX_BUBBLE_CHARS);
        bubbleEl.textContent = text;
        bubbleEl.hidden = false;
        bubbleEl.className = 'vd-pet-bubble' + (type ? ' is-' + type : '');
        bubbleEl.classList.add('opening');
        setTimeout(() => bubbleEl.classList.remove('opening'), 200);
        const duration = text.length > 70 ? LONG_BUBBLE_DURATION_MS : BUBBLE_DURATION_MS;
        bubbleTimer = setTimeout(() => hideBubble(), duration);
    }

    function hideBubble() {
        if (!bubbleEl) return;
        bubbleEl.hidden = true;
        bubbleEl.textContent = '';
        bubbleEl.className = 'vd-pet-bubble';
        if (bubbleTimer) {
            clearTimeout(bubbleTimer);
            bubbleTimer = null;
        }
    }

    function setReaction(reaction) {
        const stateId = reactionToState[reaction] || 'idle';
        setSpriteState(stateId);
    }

    async function saveSetting(key, value) {
        try {
            const body = await api('/api/desktop/settings', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ key, value })
            });
            syncPetBootstrap({ settings: body.settings || { [key]: value } });
        } catch (err) {
            showDesktopNotification({ title: t('desktop.notification'), message: err.message });
        }
    }

    function wireDrag() {
        if (!layer) return;
        layer.addEventListener('pointerdown', event => {
            if (event.button !== 0) return;
            drag = { pointerId: event.pointerId, startX: event.clientX, startY: event.clientY, origX: layer.offsetLeft, origY: layer.offsetTop };
            layer.setPointerCapture(event.pointerId);
            layer.classList.add('dragging');
        });
        layer.addEventListener('pointermove', event => {
            if (!drag || drag.pointerId !== event.pointerId) return;
            const dx = event.clientX - drag.startX;
            const dy = event.clientY - drag.startY;
            layer.style.left = (drag.origX + dx) + 'px';
            layer.style.top = (drag.origY + dy) + 'px';
        });
        layer.addEventListener('pointerup', event => {
            if (!drag || drag.pointerId !== event.pointerId) return;
            layer.releasePointerCapture(event.pointerId);
            layer.classList.remove('dragging');
            const x = layer.offsetLeft;
            const y = layer.offsetTop;
            drag = null;
            saveSetting('pet.position_x', String(x));
            saveSetting('pet.position_y', String(y));
        });
        layer.addEventListener('pointercancel', event => {
            if (!drag || drag.pointerId !== event.pointerId) return;
            layer.releasePointerCapture(event.pointerId);
            layer.classList.remove('dragging');
            drag = null;
            applyPosition();
        });
        layer.addEventListener('contextmenu', event => {
            event.preventDefault();
            showPetContextMenu(event);
        });
    }

    function showPetContextMenu(event) {
        const items = [
            { label: t('desktop.pet_open_picker', 'Choose pet...'), icon: 'heart', action: () => openApp('pet-picker') },
            { separator: true },
            { label: t('desktop.pet_enabled', 'Show pet'), icon: petEnabled() ? 'check-square' : 'square', action: toggleEnabled },
            { label: t('desktop.pet_always_on_top', 'Always on top'), icon: petAlwaysOnTop() ? 'check-square' : 'square', action: toggleAlwaysOnTop },
            { separator: true },
            { label: t('desktop.pet_hide_bubble', 'Hide bubble'), icon: 'x', action: hideBubble }
        ];
        showContextMenu(event.clientX, event.clientY, items);
    }

    function toggleEnabled() {
        const next = !petEnabled();
        saveSetting('pet.enabled', String(next)).then(() => {
            loadPet();
            if (next) showBubble(t('desktop.pet_hello', 'Hi there!'), 'info');
        });
    }

    function toggleAlwaysOnTop() {
        const next = !petAlwaysOnTop();
        saveSetting('pet.always_on_top', String(next)).then(updateLayerZIndex);
    }

    function handlePetEvent(event) {
        switch (event.type) {
            case 'pet_changed':
                syncPetBootstrap(event.payload);
                loadPet();
                return;
            case 'pet_reaction_changed':
                if (event.payload && event.payload.reaction) {
                    setReaction(event.payload.reaction);
                }
                return;
            case 'pet_say':
                if (event.payload && event.payload.message) {
                    showBubble(event.payload.message, event.payload.type || 'info');
                }
                return;
            case 'pet_setting_changed':
                if (event.payload) {
                    syncPetBootstrap({ settings: { [event.payload.key]: event.payload.value } });
                }
                loadPet();
                return;
        }
    }

    function initPetRuntime() {
        if (typeof window.addEventListener !== 'function') return;
        window.addEventListener('resize', () => {
            if (layer) applyPosition();
        });
        // Re-render after bootstrap is ready.
        const check = () => {
            if (state.bootstrap) {
                loadPet();
            } else {
                setTimeout(check, 100);
            }
        };
        check();
    }

    window.PetRuntime = {
        init: initPetRuntime,
        load: loadPet,
        setReaction,
        say: showBubble,
        hideBubble,
        handleEvent: handlePetEvent,
        syncBootstrap: syncPetBootstrap,
        saveSetting
    };
})();
