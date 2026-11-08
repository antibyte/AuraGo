// STUB path system (T02). T04 replaces this file; the exported interface is the contract (plan 3.7).
//   createPaths(defs, ctx) -> PathSystem
//   { substep(h), command(cmd) -> bool, reset(), position(ball) -> [x, y, h], debug() }
// The stub never captures a ball.
export function createPaths(defs, ctx) {
    void defs; void ctx;
    return {
        substep(h) { void h; },
        command(cmd) { void cmd; return false; },
        reset() {},
        position(ball) { return [ball.x, ball.y, ball.h]; },
        debug() { return []; }
    };
}

export default createPaths;
