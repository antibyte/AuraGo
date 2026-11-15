/* Leuchtfeuer level. Walk surfaces, spots and colliders come from one manifest. */
(function (root) {
    const MODEL_IDS = [
        'coast-island', 'coast-beach-straight', 'harbor-pier', 'harbor-pier-corner',
        'harbor-lighthouse', 'harbor-tavern', 'ships-sloop', 'equipment-treasure-chest',
        'equipment-lantern', 'equipment-barrel', 'nature-coconut-palm', 'people-naval-sailor',
        'people-fisherman', 'people-pirate-captain', 'people-pirate-gunner', 'animals-seagull'
    ];
    const PEOPLE = ['people-naval-sailor', 'people-fisherman', 'people-pirate-captain', 'people-pirate-gunner'];
    const PEOPLE_CLIPS = ['idle', 'walk', 'run', 'interact'];
    const REACH = 1.4;
    const RADIUS = 0.28;
    const GUARD_REACH = 0.7;

    function assetOf(manifest, id) {
        const assets = manifest && manifest.assets;
        if (!assets) return null;
        for (let i = 0; i < assets.length; i++) if (assets[i].id === id) return assets[i];
        return null;
    }

    function validateWorld(manifest) {
        const errors = [];
        if (!manifest || manifest.kind !== 'model3d' || manifest.id !== 'aurago-pirates-3d') errors.push('manifest');
        MODEL_IDS.forEach(function (id) {
            const asset = assetOf(manifest, id);
            if (!asset || !asset.lods || !asset.lods[0] || !asset.lods[0].file) errors.push(id);
        });
        PEOPLE.forEach(function (id) {
            const asset = assetOf(manifest, id);
            PEOPLE_CLIPS.forEach(function (clip) {
                const found = asset && asset.animations && asset.animations.some(function (item) { return item.id === clip; });
                if (!found) errors.push(id + ':' + clip);
            });
            if (asset && asset.forward !== '+Z') errors.push(id + ':forward');
        });
        const chest = assetOf(manifest, 'equipment-treasure-chest');
        if (!chest || !chest.animations || !chest.animations.some(function (item) { return item.id === 'open'; })) errors.push('chest:open');
        const ship = assetOf(manifest, 'ships-sloop');
        const deck = ship && ship.collider && ship.collider.shapes && ship.collider.shapes[0];
        if (!deck || deck.type !== 'box' || !(deck.size[1] > 0)) errors.push('sloop:deck');
        if (errors.length) {
            const error = new Error('leuchtfeuer world ' + errors.join(','));
            error.errors = errors;
            throw error;
        }
    }

    function aabb(px, py, pz, center, size) {
        return {
            minX: px + center[0] - size[0] / 2,
            maxX: px + center[0] + size[0] / 2,
            minY: py + center[1] - size[1] / 2,
            maxY: py + center[1] + size[1] / 2,
            minZ: pz + center[2] - size[2] / 2,
            maxZ: pz + center[2] + size[2] / 2
        };
    }

    function createWorld(manifest) {
        validateWorld(manifest);
        const ship = assetOf(manifest, 'ships-sloop');
        const deckBox = ship.collider.shapes[0];
        const sloop = { x: 4.2, y: 0, z: 19 };
        const deckY = deckBox.center[1] + deckBox.size[1] / 2;
        const deck = {
            id: 'deck',
            minX: sloop.x + deckBox.center[0] - deckBox.size[0] / 2,
            maxX: sloop.x + deckBox.center[0] + deckBox.size[0] / 2,
            minZ: sloop.z + deckBox.center[2] - deckBox.size[2] / 2,
            maxZ: sloop.z + deckBox.center[2] + deckBox.size[2] / 2,
            y: sloop.y + deckY
        };
        const surfaces = [
            { id: 'island', minX: -6.2, maxX: 6.2, minZ: -6.2, maxZ: 7, y: 1.15 },
            { id: 'beach', minX: -6, maxX: 6, minZ: 7, maxZ: 11, y: 0.35 },
            { id: 'pier', minX: -1, maxX: 1, minZ: 11, maxZ: 24, y: 1.07 },
            { id: 'pier-end', minX: -1.5, maxX: 1.5, minZ: 24, maxZ: 27, y: 1.07 },
            { id: 'gangway', minX: 1, maxX: deck.minX, minZ: 18.4, maxZ: 19.6, y: 1.07 },
            deck
        ];
        const pier = [];
        for (let z = 12; z <= 24; z += 2) pier.push({ id: 'harbor-pier', x: 0, y: 0, z: z });
        const tavern = { id: 'harbor-tavern', x: 7, y: 1.15, z: 0 };
        const lighthouse = { id: 'harbor-lighthouse', x: -5, y: 1.15, z: -1 };
        const chest = { id: 'equipment-treasure-chest', x: -0.4, y: 1.07, z: 14.5 };
        const barrel = { id: 'equipment-barrel', x: sloop.x, y: deck.y, z: 21 };
        const blockers = [tavern, lighthouse, chest, barrel].map(function (place) {
            const asset = assetOf(manifest, place.id);
            const box = asset.collider;
            return aabb(place.x, place.y, place.z, box.center, box.size);
        });
        const lampX = lighthouse.x + assetOf(manifest, 'harbor-lighthouse').collider.center[0]
            + assetOf(manifest, 'harbor-lighthouse').collider.size[0] / 2 + 0.55;
        return {
            models: MODEL_IDS,
            reach: REACH,
            radius: RADIUS,
            guardReach: GUARD_REACH,
            deckY: deck.y,
            sloopDeckY: deckY,
            spawn: { x: 0, y: 0.35, z: 9, yaw: Math.PI },
            surfaces: surfaces,
            blockers: blockers,
            spots: [
                { id: 'glass', part: 'glass', x: chest.x, y: chest.y, z: chest.z },
                { id: 'wick', part: 'wick', x: 4.2, y: 1.15, z: 0 },
                { id: 'oil', part: 'oil', x: barrel.x, y: barrel.y, z: barrel.z },
                { id: 'lamp', part: 'lamp', x: lampX, y: 1.15, z: lighthouse.z }
            ],
            guards: [
                { id: 'people-pirate-captain', x: 0, y: 1.07, from: 13, to: 25, speed: 0.9, phase: 0 },
                { id: 'people-pirate-gunner', x: 0, y: 1.07, from: 13, to: 25, speed: 0.9, phase: 0.5 }
            ],
            placements: [
                { id: 'coast-island', x: 0, y: 0, z: 0 },
                { id: 'coast-beach-straight', x: -4, y: 0, z: 9.2 },
                { id: 'coast-beach-straight', x: 0, y: 0, z: 9.2 },
                { id: 'coast-beach-straight', x: 4, y: 0, z: 9.2 }
            ].concat(pier, [
                { id: 'harbor-pier-corner', x: 0, y: 0, z: 26 },
                lighthouse,
                tavern,
                { id: 'ships-sloop', x: sloop.x, y: sloop.y, z: sloop.z },
                chest,
                { id: 'equipment-lantern', x: lampX, y: 1.15, z: lighthouse.z },
                barrel,
                { id: 'nature-coconut-palm', x: -4, y: 1.15, z: -3 },
                { id: 'nature-coconut-palm', x: 3, y: 1.15, z: 3 },
                { id: 'animals-seagull', x: tavern.x, y: tavern.y + 4.7, z: -1.6 }
            ]),
            actors: [
                { id: 'people-naval-sailor', role: 'player' },
                { id: 'people-fisherman', role: 'fisher', x: 4.35, y: 1.15, z: 1.7, faceX: -1, faceZ: 0 },
                { id: 'people-pirate-captain', role: 'guard', index: 0 },
                { id: 'people-pirate-gunner', role: 'guard', index: 1 },
                { id: 'animals-seagull', role: 'bird', clip: 'idle' }
            ]
        };
    }

    root.LeuchtfeuerWorld = {
        MODEL_IDS: MODEL_IDS,
        validateWorld: validateWorld,
        createWorld: createWorld,
        assetOf: assetOf
    };
})(typeof window !== 'undefined' ? window : globalThis);
