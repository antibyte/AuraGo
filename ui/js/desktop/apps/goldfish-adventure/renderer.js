(function () {
    'use strict';

    const BASE = '/img/goldfish-adventure/';
    const assetPath = value => typeof value === 'string' && value.startsWith(BASE) &&
        !value.includes('..') && !/[?#\\]/.test(value);

    function create(canvas, options = {}) {
        const context = canvas.getContext('2d', { alpha: false });
        let manifest, scene, state, disposed = false, paused = true, generation = 0;
        let frame = 0, resizeFrame = 0, previous = 0, clock = 0, images = new Map(), pendingImages = new Map();
        let actorPosition = { x: 0.24, y: 0.83 }, walking = null, performance = null, moment = null;
        let facing = 'right', audio = null, unlocked = false, speaking = null;
        const effects = new Set();
        const cameraHome = () => ({ zoom: 1, x: 0.5, y: 0.5 });
        let camera = cameraHome();
        const controller = new AbortController();
        const reduced = () => state?.settings.reducedMotion || window.matchMedia('(prefers-reduced-motion: reduce)').matches;
        const allowed = record => !state || window.GoldfishState.conditions(state, record);

        async function init() {
            const response = await fetch(BASE + 'manifest.json', { signal: controller.signal, credentials: 'same-origin' });
            if (!response.ok) throw new Error('art_failed');
            const value = await response.json();
            if (disposed) return;
            if (value.version !== 1 || !value.rooms || !value.sprites) throw new Error('art_failed');
            manifest = value;
        }

        function loadImage(src) {
            if (!assetPath(src)) return Promise.reject(new Error('art_failed'));
            return new Promise((resolve, reject) => {
                const image = new Image();
                pendingImages.set(image, reject);
                image.onload = () => {
                    pendingImages.delete(image);
                    image.onload = image.onerror = null;
                    if (disposed) { image.src = ''; reject(new Error('disposed')); }
                    else resolve(image);
                };
                image.onerror = () => { pendingImages.delete(image); reject(new Error('art_failed')); };
                image.src = src + '?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev');
            });
        }

        function stopAudio() {
            if (!audio) return;
            audio.pause();
            audio.removeAttribute('src');
            audio.load();
            audio = null;
        }

        function stopEffects() {
            for (const sound of effects) {
                sound.onended = null;
                sound.pause();
                sound.removeAttribute('src');
                sound.load();
            }
            effects.clear();
        }

        function playEffect(src) {
            if (!assetPath(src) || !unlocked || !state?.settings.sound || paused || disposed) return;
            const sound = new Audio(src);
            sound.volume = 0.45;
            sound.onended = () => { effects.delete(sound); sound.removeAttribute('src'); sound.load(); };
            effects.add(sound);
            sound.play().catch(() => {
                effects.delete(sound); sound.pause(); sound.removeAttribute('src'); sound.load();
                options.onAudioBlocked?.();
            });
        }

        function syncAudio() {
            if (!scene?.audio || !assetPath(scene.audio) || !unlocked || !state?.settings.sound || paused || disposed) {
                if (audio) audio.pause();
                return;
            }
            if (!audio) { audio = new Audio(scene.audio); audio.loop = true; audio.volume = 0.3; }
            audio.play().catch(() => options.onAudioBlocked?.());
        }

        async function load(roomID, nextState) {
            const ticket = ++generation;
            for (const [image, reject] of pendingImages) {
                image.onload = image.onerror = null;
                image.src = '';
                reject(new Error('superseded'));
            }
            pendingImages.clear();
            cancelMotion();
            stopAudio();
            state = nextState;
            options.onStatus?.('loading_art');
            if (!manifest) await init();
            if (disposed || ticket !== generation) return false;
            const nextScene = manifest.rooms[roomID];
            if (!nextScene || !assetPath(nextScene.background)) throw new Error('art_failed');
            const actors = nextScene.actors || [];
            const sprites = ['r0st'].concat(actors.map(actor => actor.sprite || actor.id));
            const spriteSources = sprites.flatMap(id => {
                const sprite = manifest.sprites[id];
                return sprite ? [sprite.src].concat((sprite.frames || []).map(value => value.src)).filter(Boolean) : [];
            });
            const sources = [...new Set([nextScene.background]
                .concat((nextScene.layers || []).map(layer => layer.src))
                .concat(spriteSources))];
            const loaded = await Promise.all(sources.map(async src => [src, await loadImage(src)]));
            if (disposed || ticket !== generation) { for (const [, image] of loaded) image.src = ''; return false; }
            const decodedBytes = loaded.reduce((sum, [, image]) => sum + image.naturalWidth * image.naturalHeight * 4, 0);
            if (decodedBytes > 192 * 1024 * 1024) {
                for (const [, image] of loaded) image.src = '';
                throw new Error('art_failed');
            }
            for (const image of images.values()) image.src = '';
            images = new Map(loaded);
            scene = nextScene;
            actorPosition = { ...nextState.position };
            options.onStatus?.('ready');
            draw();
            syncAudio();
            schedule();
            return true;
        }

        function resize() {
            if (disposed) return;
            const width = Math.max(320, Math.min(2560, Math.round(canvas.clientWidth * Math.min(window.devicePixelRatio || 1, 2))));
            const height = Math.round(width * 9 / 16);
            if (canvas.width === width && canvas.height === height) return;
            canvas.width = width;
            canvas.height = height;
            draw();
        }

        function frameRect(sprite, index) {
            if (sprite.frames?.[index]) return sprite.frames[index].rect || sprite.frames[index];
            const columns = sprite.columns || 1;
            return [(index % columns) * sprite.frameWidth, Math.floor(index / columns) * sprite.frameHeight,
                sprite.frameWidth, sprite.frameHeight];
        }

        function clipFrame(sprite, name, started = performance?.started ?? 0) {
            const clips = sprite.clips || {};
            const clip = clips[name] || clips['idle-' + facing] || clips.idle;
            if (!clip?.frames?.length) return 0;
            const time = Math.max(0, clock - started);
            if (typeof clip.frames[0] === 'object') {
                const duration = clip.frames.reduce((sum, value) => sum + (value.duration || 100), 0);
                let cursor = clip.loop === false ? Math.min(time, duration - 1) : time % duration;
                for (const value of clip.frames) {
                    if (cursor < (value.duration || 100)) return value.frame ?? value.index ?? value;
                    cursor -= value.duration || 100;
                }
            }
            const index = reduced() ? 0 : Math.floor(time / 1000 * (clip.fps || 8));
            return clip.frames[clip.loop === false ? Math.min(index, clip.frames.length - 1) : index % clip.frames.length];
        }

        function drawActor(actor, player = false) {
            const sprite = manifest.sprites[actor.sprite || actor.id];
            if (!sprite) return;
            const staging = moment?.actors.get(actor.id);
            const position = staging?.position || (player ? actorPosition : { x: actor.position[0], y: actor.position[1] });
            const isSpeaking = speaking === actor.id || (actor.speakers || []).includes(speaking);
            const clip = staging?.clip || (player ? (walking ? 'walk-' + facing : performance?.clip || (isSpeaking ? 'talk' : 'idle-' + facing)) :
                (isSpeaking ? 'talk' : actor.clip || 'idle'));
            const selected = clipFrame(sprite, clip, staging?.started);
            const frameRecord = typeof selected === 'number' ? sprite.frames?.[selected] : selected;
            const image = images.get(frameRecord?.src || sprite.src);
            if (!image) return;
            const rect = typeof selected === 'object' ? selected.rect || selected : frameRect(sprite, selected);
            if (!rect || !rect.every(Number.isFinite) || rect[2] <= 0 || rect[3] <= 0) return;
            const height = (actor.height || sprite.height || 0.29) * canvas.height * (actor.scale || 1);
            const width = height * rect[2] / rect[3];
            const pivot = frameRecord?.pivot || sprite.pivot || [0.5, 1];
            context.drawImage(image, ...rect, position.x * canvas.width - width * pivot[0],
                position.y * canvas.height - height * pivot[1], width, height);
        }

        function drawLayer(layer) {
            const image = images.get(layer.src);
            if (!image || !allowed(layer)) return;
            const rect = layer.rect || [0, 0, 1, 1];
            context.drawImage(image, rect[0] * canvas.width, rect[1] * canvas.height, rect[2] * canvas.width, rect[3] * canvas.height);
        }

        function draw() {
            if (disposed || !context) return;
            context.fillStyle = '#171c22';
            context.fillRect(0, 0, canvas.width, canvas.height);
            if (!scene) return;
            context.save();
            context.translate(canvas.width / 2, canvas.height / 2);
            context.scale(camera.zoom, camera.zoom);
            context.translate(-camera.x * canvas.width, -camera.y * canvas.height);
            const background = images.get(scene.background);
            if (background) {
                const scale = Math.min(canvas.width / background.naturalWidth, canvas.height / background.naturalHeight);
                const width = background.naturalWidth * scale, height = background.naturalHeight * scale;
                context.drawImage(background, (canvas.width - width) / 2, (canvas.height - height) / 2, width, height);
            }
            const actors = (scene.actors || []).filter(allowed).map(actor => ({
                z: actor.z ?? (moment?.actors.get(actor.id)?.position.y ?? actor.position[1]) * 1000, actor
            }));
            const layers = (scene.layers || []).map(layer => ({ z: layer.z ?? 1000, layer }));
            actors.push({ z: (moment?.actors.get('r0st')?.position.y ?? actorPosition.y) * 1000, actor: { id: 'r0st' }, player: true });
            for (const value of actors.concat(layers).sort((a, b) => a.z - b.z)) {
                if (value.layer) drawLayer(value.layer);
                else drawActor(value.actor, value.player);
            }
            context.restore();
        }

        function schedule() {
            if (!disposed && !paused && !frame) frame = requestAnimationFrame(tick);
        }

        function tick(now) {
            frame = 0;
            if (paused || disposed) return;
            const delta = previous ? Math.min(50, now - previous) : 0;
            previous = now;
            clock += delta;
            options.onTick?.(delta);
            if (moment) advanceMoment(delta);
            if (walking) {
                walking.elapsed += delta;
                const fraction = Math.min(1, walking.elapsed / walking.duration);
                actorPosition = { x: walking.from.x + (walking.to.x - walking.from.x) * fraction,
                    y: walking.from.y + (walking.to.y - walking.from.y) * fraction };
                if (fraction === 1) {
                    const finished = walking;
                    walking = null;
                    options.onPosition?.({ ...actorPosition });
                    finished.resolve(true);
                }
            }
            if (performance && clock - performance.started >= performance.duration) {
                const finished = performance;
                performance = null;
                finished.resolve(true);
            }
            draw();
            schedule();
        }

        function walk(point) {
            cancelMotion();
            const x = Math.max(0.08, Math.min(0.92, point[0]));
            const y = Math.max(scene?.walkMinY || 0.65, Math.min(scene?.walkMaxY || 0.9, point[1]));
            facing = x < actorPosition.x ? 'left' : 'right';
            if (reduced()) { actorPosition = { x, y }; options.onPosition?.({ ...actorPosition }); draw(); return Promise.resolve(true); }
            return new Promise(resolve => {
                walking = { from: { ...actorPosition }, to: { x, y }, elapsed: 0,
                    duration: Math.max(180, Math.hypot(x - actorPosition.x, y - actorPosition.y) * 2800), resolve };
                schedule();
            });
        }

        function stageBeat() {
            const beat = moment.beats[moment.index];
            moment.elapsed = 0;
            moment.reducedMotion = reduced();
            moment.duration = reduced() ? Math.min(260, beat.duration) : beat.duration;
            moment.cameraFrom = { ...camera };
            const zoom = Math.max(1, Math.min(1.6, Number(beat.camera?.zoom) || camera.zoom));
            const edge = 0.5 / zoom;
            const center = value => Math.max(edge, Math.min(1 - edge, Number.isFinite(value) ? value : 0.5));
            moment.cameraTo = reduced() ? cameraHome() : {
                zoom, x: center(beat.camera?.x ?? camera.x), y: center(beat.camera?.y ?? camera.y)
            };
            moment.targets = new Map();
            for (const [id, stage] of Object.entries(beat.actors || {})) {
                const actor = id === 'r0st' ? null : (scene.actors || []).find(value => value.id === id);
                if (id !== 'r0st' && !actor) continue;
                const prior = moment.actors.get(id);
                const position = prior?.position || (id === 'r0st' ? { ...actorPosition } : { x: actor.position[0], y: actor.position[1] });
                const point = stage.position;
                const target = !reduced() && Array.isArray(point) && point.length === 2 && point.every(Number.isFinite) ?
                    { x: Math.max(0, Math.min(1, point[0])), y: Math.max(0, Math.min(1, point[1])) } : { ...position };
                moment.actors.set(id, { position: { ...position }, clip: stage.clip || prior?.clip || 'idle', started: clock });
                moment.targets.set(id, { from: { ...position }, to: target });
            }
            moment.effectPending = beat.sfx;
        }

        function advanceMoment(delta) {
            if (reduced() && !moment.reducedMotion) {
                moment.reducedMotion = true;
                camera = cameraHome();
                moment.cameraFrom = cameraHome();
                moment.cameraTo = cameraHome();
                moment.duration = Math.min(moment.duration, moment.elapsed + 260);
                for (const [id, target] of moment.targets) {
                    target.from = { ...moment.actors.get(id).position };
                    target.to = { ...target.from };
                }
            }
            if (moment.effectPending) { playEffect(moment.effectPending); moment.effectPending = null; }
            moment.elapsed += delta;
            const fraction = Math.min(1, moment.elapsed / moment.duration);
            const smooth = fraction * fraction * (3 - 2 * fraction);
            for (const [id, target] of moment.targets) {
                const staged = moment.actors.get(id);
                staged.position.x = target.from.x + (target.to.x - target.from.x) * smooth;
                staged.position.y = target.from.y + (target.to.y - target.from.y) * smooth;
            }
            for (const key of ['zoom', 'x', 'y']) camera[key] = moment.cameraFrom[key] + (moment.cameraTo[key] - moment.cameraFrom[key]) * smooth;
            if (fraction === 1) {
                moment.index++;
                if (moment.index >= moment.beats.length) finishMoment(true);
                else stageBeat();
            }
        }

        function finishMoment(completed) {
            if (!moment) return false;
            const finished = moment;
            moment = null;
            camera = cameraHome();
            stopEffects();
            options.onMoment?.(false);
            finished.resolve(completed);
            draw();
            return true;
        }

        function act(actionID = 'use', fallbackClip = actionID) {
            cancelMotion();
            if (disposed) return Promise.resolve(false);
            const authored = manifest?.moments?.[actionID];
            const beats = (Array.isArray(authored) ? authored : authored?.beats || [])
                .filter(beat => Number.isFinite(beat.duration) && beat.duration > 0 && beat.duration <= 15000).slice(0, 24);
            if (beats.length && beats.reduce((sum, beat) => sum + beat.duration, 0) <= 60000) {
                return new Promise(resolve => {
                    moment = { beats, index: 0, actors: new Map(), resolve };
                    stageBeat();
                    options.onMoment?.(true);
                    draw();
                    schedule();
                });
            }
            if (reduced()) { draw(); return Promise.resolve(true); }
            return new Promise(resolve => {
                performance = { clip: fallbackClip, started: clock, duration: fallbackClip === 'celebrate' ? 1300 : 700, resolve };
                schedule();
            });
        }

        function cancelMotion() {
            finishMoment(false);
            if (walking) { walking.resolve(false); walking = null; }
            if (performance) { performance.resolve(false); performance = null; }
        }

        function setPaused(value) {
            paused = value;
            previous = 0;
            if (paused && frame) { cancelAnimationFrame(frame); frame = 0; }
            if (paused) { audio?.pause(); for (const sound of effects) sound.pause(); }
            else {
                syncAudio();
                if (state?.settings.sound && unlocked) for (const sound of effects) sound.play().catch(() => options.onAudioBlocked?.());
                schedule();
            }
        }

        const observer = new ResizeObserver(() => {
            if (disposed || resizeFrame) return;
            resizeFrame = requestAnimationFrame(() => { resizeFrame = 0; resize(); });
        });
        observer.observe(canvas);
        resize();

        return {
            init, load, walk, act, setPaused, cancelMotion,
            skipMoment: () => finishMoment(true),
            speak: value => { speaking = value; draw(); },
            update: value => { state = value; if (!value.settings.sound) stopEffects(); draw(); syncAudio(); },
            unlock: () => { unlocked = true; syncAudio(); },
            dispose: () => {
                disposed = true;
                generation++;
                controller.abort();
                observer.disconnect();
                if (resizeFrame) cancelAnimationFrame(resizeFrame);
                resizeFrame = 0;
                if (frame) cancelAnimationFrame(frame);
                frame = 0;
                cancelMotion();
                stopAudio();
                stopEffects();
                for (const [image, reject] of pendingImages) { image.onload = image.onerror = null; image.src = ''; reject(new Error('disposed')); }
                pendingImages.clear();
                for (const image of images.values()) image.src = '';
                images.clear();
                manifest = scene = state = null;
                canvas.width = canvas.height = 1;
            }
        };
    }

    window.GoldfishRenderer = Object.freeze({ create });
}());
