// Generic shot reachability + soak for every table that declares shots (plan T14). Run: node scripts/test-pinball-reach.mjs
import assert from 'node:assert/strict';
import { TABLES } from '../ui/js/desktop/apps/pinball/tables/index.js';
import { sweep, soak, crossShots } from './lib/pinball-shots.mjs';

let tested = 0;
for (const [id, table] of Object.entries(TABLES)) {
    const layout = table.layout;
    if (!(layout.shots && layout.shots.length)) { console.log('skip  ' + id + ' (no shots)'); continue; }
    tested++;
    const r = sweep(layout);
    assert.deepEqual(r.missing, [], id + ': shots never reached: ' + r.missing.join(', '));
    for (const fid of ['left', 'right']) {
        const reached = r.flippers[fid];
        assert.ok(reached.length >= 4, id + ' ' + fid + ' flipper reaches only ' + reached.join(', '));
        assert.ok(crossShots(layout, fid, reached).length >= 1, id + ' ' + fid + ' flipper reaches no cross shot (' + reached.join(', ') + ')');
    }
    console.log('ok    ' + id + ' reach (' + r.runs + ' runs): ' + Object.entries(r.flippers).map(([f, s]) => f + '=' + s.length).join(' '));
    for (const [shot, v] of Object.entries(r.shots)) {
        console.log('        ' + shot + ': ' + Object.entries(v.byFlipper).map(([f, h]) => f + '/' + h.start + '@' + h.delay + ' ' + h.peak + ' m/s').join(', '));
    }
    const s = soak(layout, { seed: 7 });
    assert.ok(s.ok, id + ' soak: ' + s.problems.slice(0, 4).join('; '));
    assert.equal(s.searchFail, 0);
    console.log('ok    ' + id + ' soak: ' + s.switches + ' switches, longest rest ' + s.maxRest + ' s, ' + (s.counts.drain || 0) + ' drains');
}
console.log('reach ok (' + tested + ' table' + (tested === 1 ? '' : 's') + ' with shots)');
