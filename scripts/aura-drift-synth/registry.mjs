// Registry of every synthesized file. jobs() is cheap (no rendering); renderJob(id) does the DSP.
import { WORLD_RECIPES } from './sfx-world.mjs';
import { WEAPON_RECIPES } from './sfx-weapons.mjs';
import { COMIC_RECIPES } from './sfx-comic.mjs';
import { ENGINE_LAYERS, CAR_IDS, renderEngineLayer } from './engines.mjs';
import { BIOMES, renderBiome, renderMenu, renderResults } from './music.mjs';

export { ENGINE_LAYERS, CAR_IDS, BIOMES };
export const MUSIC_LAYERS = ['bed', 'drive', 'peak'];
export const SFX_RECIPES = [...WORLD_RECIPES, ...WEAPON_RECIPES, ...COMIC_RECIPES];

const slug = text => text.replace(/[._]/g, '-');

export function jobs() {
    const list = [];
    for (const recipe of SFX_RECIPES) {
        for (let v = 0; v < recipe.variants; v++) {
            list.push({ id: `sfx:${recipe.group}:${v}`, type: 'sfx', group: recipe.group, variant: v, channels: recipe.channels, loop: !!recipe.loop, path: `sfx/${slug(recipe.group)}-${v + 1}.ogg` });
        }
    }
    for (const car of CAR_IDS) {
        for (const layer of Object.keys(ENGINE_LAYERS)) list.push({ id: `engine:${car}:${layer}`, type: 'engine', car, layer, channels: 1, loop: true, path: `engine/${car}-${layer}.ogg` });
    }
    for (const biome of BIOMES) {
        list.push({ id: `music:${biome}`, type: 'music', biome, channels: 2, loop: true, paths: Object.fromEntries(MUSIC_LAYERS.map(layer => [layer, `music/${biome}-${layer}.ogg`])) });
    }
    list.push({ id: 'music:menu', type: 'music-single', name: 'menu', channels: 2, loop: true, path: 'music/menu.ogg' });
    list.push({ id: 'music:results', type: 'music-single', name: 'results', channels: 2, loop: true, path: 'music/results.ogg' });
    return list;
}

export function renderJob(id) {
    const [type, a, b] = id.split(':');
    if (type === 'sfx') return SFX_RECIPES.find(recipe => recipe.group === a).render(Number(b));
    if (type === 'engine') return renderEngineLayer(a, b);
    if (type === 'music') return a === 'menu' ? renderMenu() : a === 'results' ? renderResults() : renderBiome(a);
    throw new Error(`unknown job ${id}`);
}

// Per-recipe render needs a seeded rng; recipes receive (variant, rng). Wrap so callers need not care.
import { rng, seedOf } from './dsp.mjs';
for (const recipe of SFX_RECIPES) {
    const inner = recipe.render;
    recipe.render = v => inner(v, rng(seedOf(`${recipe.group}:${v}`)));
}
