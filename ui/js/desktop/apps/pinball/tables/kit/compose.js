// Compose a layout from kit parts. List fields concatenate; meta and plunger merge (later parts win). DOM-free.
const LISTS = ['flippers', 'walls', 'posts', 'devices', 'paths', 'shots', 'lamps', 'flashers', 'gi', 'toys', 'labels', 'allowGaps'];

export function composeLayout(...parts) {
    const out = { id: '', version: 1, meta: {}, plunger: null, drainY: 0.075 };
    for (const key of LISTS) out[key] = [];
    for (const part of parts) {
        if (!part) continue;
        if (part.id) out.id = part.id;
        if (part.version) out.version = part.version;
        if (part.meta) out.meta = { ...out.meta, ...part.meta };
        if (part.plunger) out.plunger = { ...(out.plunger || {}), ...part.plunger };
        if (typeof part.drainY === 'number') out.drainY = part.drainY;
        for (const key of LISTS) if (part[key]) out[key].push(...part[key]);
    }
    return out;
}
