// Per-table visual hooks (DOM allowed). paintArt(kit) decorates the playfield canvas above the underlay;
// buildToys(THREE, kit, layout, {tier, low}) -> {group, update(dt, t, snap, toyState, lamps, events), dispose} adds the table's 3D toys.
// T16 (Neon), T22 (Leviathan) and T25 (Temple) fill these in.
import { paintArt as neonArt } from './neon/art.js';
import { buildToys as neonToys } from './neon/toys.js';
import { paintArt as leviathanArt } from './leviathan/art.js';
import { buildToys as leviathanToys } from './leviathan/toys.js';
import { paintArt as templeArt } from './temple/art.js';
import { buildToys as templeToys } from './temple/toys.js';

export const VISUALS = {
    neon: { paintArt: neonArt, buildToys: neonToys },
    leviathan: { paintArt: leviathanArt, buildToys: leviathanToys },
    temple: { paintArt: templeArt, buildToys: templeToys }
};
export default VISUALS;
