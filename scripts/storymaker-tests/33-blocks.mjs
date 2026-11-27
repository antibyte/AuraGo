// Batch C: SM.blocks, the player's icon, diagram, chart, headline and list blocks and the env.board backdrop.
import assert from 'node:assert/strict';
import { t } from '../storymaker-test-context.mjs';

const { check } = t;

check('blocks: indexCatalog indexes the icons', () => {
    const idx = t.core().scene.indexCatalog(t.catalog);
    assert.ok(idx.icons['icon.cpu'].paths.length > 0);
    assert.ok(Array.isArray(idx.icons['icon.cpu'].tags));
    assert.deepEqual(Object.keys(t.core().scene.indexCatalog({ entries: [] }).icons), []);
});
