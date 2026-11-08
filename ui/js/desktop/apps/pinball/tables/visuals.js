// Per-table visual hooks (DOM allowed). paintArt(kit) decorates the playfield canvas above the underlay;
// buildToys(THREE, kit, layout) -> {group, update(dt, t, snap, toyState, lamps), dispose} adds the table's 3D toys.
// T16 (Neon), T22 (Leviathan) and T25 (Temple) fill these in.
export const VISUALS = {
    neon: { paintArt: null, buildToys: null },
    leviathan: { paintArt: null, buildToys: null },
    temple: { paintArt: null, buildToys: null }
};
export default VISUALS;
