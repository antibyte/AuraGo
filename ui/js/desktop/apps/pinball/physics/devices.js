// STUB device system (T02). T03 replaces this file; the exported interface is the contract (plan 3.7).
//   createDevices(defs, ctx) -> DeviceSystem
//   { ids, preTick(), substep(h), postTick(), command(cmd) -> bool, search(cycle) -> bool, reset(), writeAnim(anim), debug() }
// The stub registers no colliders and handles no commands.
export function createDevices(defs, ctx) {
    void ctx;
    return {
        ids: (defs || []).map(d => d.id),
        preTick() {},
        substep(h) { void h; },
        postTick() {},
        command(cmd) { void cmd; return false; },
        search(cycle) { void cycle; return false; },
        reset() {},
        writeAnim(anim) { void anim; },
        debug() { return []; }
    };
}

export default createDevices;
