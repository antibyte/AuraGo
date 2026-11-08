// Uniform broadphase grid for line and circle shapes. DOM-free.
// Shapes are inserted by their AABB; query() returns every shape whose cells overlap a box, each once.

export function createGrid({ x0 = -0.30, x1 = 0.30, y0 = -0.05, y1 = 1.12, cell = 0.04 } = {}) {
    const cols = Math.max(1, Math.ceil((x1 - x0) / cell)), rows = Math.max(1, Math.ceil((y1 - y0) / cell));
    const cells = new Array(cols * rows);
    for (let i = 0; i < cells.length; i++) cells[i] = [];
    let stamp = 0;
    const col = x => { const c = Math.floor((x - x0) / cell); return c < 0 ? 0 : (c >= cols ? cols - 1 : c); };
    const row = y => { const r = Math.floor((y - y0) / cell); return r < 0 ? 0 : (r >= rows ? rows - 1 : r); };

    function bounds(s, out) {
        if (s.kind === 'circle') { out[0] = s.cx - s.r; out[1] = s.cy - s.r; out[2] = s.cx + s.r; out[3] = s.cy + s.r; }
        else {
            out[0] = Math.min(s.ax, s.bx); out[1] = Math.min(s.ay, s.by);
            out[2] = Math.max(s.ax, s.bx); out[3] = Math.max(s.ay, s.by);
        }
        return out;
    }
    const box = [0, 0, 0, 0];

    return {
        cell, cols, rows,
        insert(shape) {
            bounds(shape, box);
            const c0 = col(box[0] - 0.001), c1 = col(box[2] + 0.001), r0 = row(box[1] - 0.001), r1 = row(box[3] + 0.001);
            for (let r = r0; r <= r1; r++) for (let c = c0; c <= c1; c++) cells[r * cols + c].push(shape);
        },
        // Fills `out` (length reset) with the unique shapes in the box and returns it.
        query(minx, miny, maxx, maxy, out) {
            out.length = 0;
            stamp++;
            const c0 = col(minx), c1 = col(maxx), r0 = row(miny), r1 = row(maxy);
            for (let r = r0; r <= r1; r++) {
                for (let c = c0; c <= c1; c++) {
                    const list = cells[r * cols + c];
                    for (let i = 0; i < list.length; i++) {
                        const s = list[i];
                        if (s._q !== stamp) { s._q = stamp; out.push(s); }
                    }
                }
            }
            return out;
        }
    };
}
