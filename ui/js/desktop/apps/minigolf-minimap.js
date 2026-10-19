import { colorValue, FLOOR_SURFACES } from './minigolf-shared.js';

const FLAT = 0.55, MARGIN = 0.5;

// Top-down lane in world x/z: floor triangles per surface, rail tops, water and the view frame.
// Triangles are stored with one winding so a single nonzero fill can never cancel overlaps.
export function laneGeometry(course) {
    const floor = {}, walls = [], min = [Infinity, Infinity], max = [-Infinity, -Infinity];
    for (const shape of course?.colliders || []) {
        if (shape?.type !== 'mesh' || !Array.isArray(shape.vertices) || !Array.isArray(shape.indices)) continue;
        const v = shape.vertices, surface = shape.surface || 'grass', isFloor = FLOOR_SURFACES.has(surface);
        const target = isFloor ? (floor[surface] ||= []) : walls;
        for (let i = 0; i + 2 < shape.indices.length; i += 3) {
            const a = shape.indices[i] * 3, b = shape.indices[i + 1] * 3, c = shape.indices[i + 2] * 3;
            const ux = v[b] - v[a], uy = v[b + 1] - v[a + 1], uz = v[b + 2] - v[a + 2], wx = v[c] - v[a], wy = v[c + 1] - v[a + 1], wz = v[c + 2] - v[a + 2];
            const ny = uz * wx - ux * wz, length = Math.hypot(uy * wz - uz * wy, ny, ux * wy - uy * wx);
            if (!(length > 0) || Math.abs(ny) / length < FLAT) continue;
            const [p, q] = ny < 0 ? [c, b] : [b, c];
            target.push(v[a], v[a + 2], v[p], v[p + 2], v[q], v[q + 2]);
            if (isFloor) for (const index of [a, b, c]) { min[0] = Math.min(min[0], v[index]); min[1] = Math.min(min[1], v[index + 2]); max[0] = Math.max(max[0], v[index]); max[1] = Math.max(max[1], v[index + 2]); }
        }
    }
    const frame = Number.isFinite(min[0])
        ? { min: [min[0] - MARGIN, min[1] - MARGIN], max: [max[0] + MARGIN, max[1] + MARGIN] }
        : { min: [course?.bounds?.min?.[0] ?? -10, course?.bounds?.min?.[2] ?? -10], max: [course?.bounds?.max?.[0] ?? 10, course?.bounds?.max?.[2] ?? 10] };
    const water = (course?.water || []).map(volume => ({ min: [volume.min[0], volume.min[2]], max: [volume.max[0], volume.max[2]] }));
    return { frame, floor, walls, water };
}

function rotate(q, [x, y, z]) {
    if (!q) return [x, y, z];
    const [qx, qy, qz, qw] = q;
    const ix = qw * x + qy * z - qz * y, iy = qw * y + qz * x - qx * z, iz = qw * z + qx * y - qy * x, iw = -qx * x - qy * y - qz * z;
    return [ix * qw + iw * -qx + iy * -qz - iz * -qy, iy * qw + iw * -qy + iz * -qx - ix * -qz, iz * qw + iw * -qz + ix * -qy - iy * -qx];
}

// One rotated rectangle [x,z × 4] per obstacle part, posed like physics.getObstacles().
export function obstacleOutlines(course, poses = []) {
    const outlines = [];
    for (const obstacle of course?.obstacles || []) {
        const pose = poses.find(item => item.id === obstacle.id) || { position: obstacle.position, rotation: obstacle.rotation || [0, 0, 0, 1] };
        const parts = Array.isArray(obstacle.colliders) && obstacle.colliders.length ? obstacle.colliders : [{ position: [0, 0, 0], halfExtents: obstacle.halfExtents }];
        for (const part of parts) {
            const [hx, , hz] = part.halfExtents, center = part.position || [0, 0, 0], outline = [];
            for (const [sx, sz] of [[-1, -1], [1, -1], [1, 1], [-1, 1]]) {
                const local = rotate(part.rotation, [sx * hx, 0, sz * hz]);
                const world = rotate(pose.rotation, [local[0] + center[0], local[1] + center[1], local[2] + center[2]]);
                outline.push(world[0] + pose.position[0], world[2] + pose.position[2]);
            }
            outlines.push(outline);
        }
    }
    return outlines;
}

const DEFAULT_COLORS = { water: '#5fb8d8', grass: '#7cc47a', sand: '#e8d39a', wall: '#5b4a36', tee: '#fff9ec', cup: '#1b2a24' };

// Two canvas layers in device pixels inside element (.mg-map, markup written by the HUD):
// the lane is drawn once per course and size, balls and obstacles only when a rounded position changed.
export function createMinimap(element) {
    const frameEl = element.querySelector('.mg-map-frame'), toggle = element.querySelector('.mg-map-toggle');
    const laneCanvas = element.querySelector('.mg-map-lane'), liveCanvas = element.querySelector('.mg-map-live');
    const lane = laneCanvas.getContext('2d'), live = liveCanvas.getContext('2d');
    let course = null, geometry = null, colors = DEFAULT_COLORS, projection = null, ratio = 1;
    let visible = true, drawn = false, balls = 0, liveKey = '', last = null;
    const px = x => projection.ox + x * projection.scale, pz = z => projection.oz + z * projection.scale;

    function readColors() {
        const style = getComputedStyle(frameEl);
        colors = Object.fromEntries(Object.entries(DEFAULT_COLORS).map(([name, fallback]) => [name, style.getPropertyValue('--mg-map-' + name).trim() || fallback]));
    }
    // North is up: x grows right, z grows down. Keeps the aspect ratio and centres the lane.
    function layout() {
        ratio = window.devicePixelRatio || 1;
        const width = Math.round(frameEl.clientWidth * ratio), height = Math.round(frameEl.clientHeight * ratio);
        if (laneCanvas.width !== width || laneCanvas.height !== height) { laneCanvas.width = liveCanvas.width = width; laneCanvas.height = liveCanvas.height = height; }
        projection = null;
        if (!geometry || !width || !height) return;
        const { min, max } = geometry.frame, spanX = max[0] - min[0] || 1, spanZ = max[1] - min[1] || 1, scale = Math.min(width / spanX, height / spanZ);
        projection = { scale, ox: (width - spanX * scale) / 2 - min[0] * scale, oz: (height - spanZ * scale) / 2 - min[1] * scale };
    }
    function fillTriangles(list, color) {
        if (!list?.length) return;
        lane.fillStyle = color; lane.beginPath();
        for (let i = 0; i < list.length; i += 6) { lane.moveTo(px(list[i]), pz(list[i + 1])); lane.lineTo(px(list[i + 2]), pz(list[i + 3])); lane.lineTo(px(list[i + 4]), pz(list[i + 5])); lane.closePath(); }
        lane.fill();
    }
    function drawLane() {
        lane.clearRect(0, 0, laneCanvas.width, laneCanvas.height);
        drawn = false;
        if (!projection) return;
        lane.fillStyle = colors.water;
        for (const rect of geometry.water) lane.fillRect(px(rect.min[0]), pz(rect.min[1]), (rect.max[0] - rect.min[0]) * projection.scale, (rect.max[1] - rect.min[1]) * projection.scale);
        for (const surface of Object.keys(geometry.floor).sort((a, b) => (b === 'grass') - (a === 'grass'))) fillTriangles(geometry.floor[surface], colors[surface] || colors.grass);
        fillTriangles(geometry.walls, colors.wall);
        const [tx, , tz] = course.tee || [0, 0, 0], [cx, , cz] = course.cup || [0, 0, 0], unit = ratio;
        lane.fillStyle = colors.tee; lane.fillRect(px(tx) - 2 * unit, pz(tz) - 2 * unit, 4 * unit, 4 * unit);
        lane.beginPath(); lane.arc(px(cx), pz(cz), 4.5 * unit, 0, Math.PI * 2);
        lane.strokeStyle = colors.tee; lane.lineWidth = 3.5 * unit; lane.stroke();
        lane.strokeStyle = colors.cup; lane.lineWidth = 1.6 * unit; lane.stroke();
        lane.beginPath(); lane.arc(px(cx), pz(cz), 1.6 * unit, 0, Math.PI * 2); lane.fillStyle = colors.cup; lane.fill();
        drawn = true;
    }
    function drawLive(outlines, players, active, positions) {
        live.clearRect(0, 0, liveCanvas.width, liveCanvas.height);
        live.fillStyle = colors.wall;
        for (const outline of outlines) {
            live.beginPath(); live.moveTo(px(outline[0]), pz(outline[1]));
            for (let i = 2; i < outline.length; i += 2) live.lineTo(px(outline[i]), pz(outline[i + 1]));
            live.closePath(); live.fill();
        }
        balls = 0;
        const ball = (index, big) => {
            const at = positions[index]; if (!at) return;
            const radius = (big ? 5.25 : 3.5) * ratio;
            live.beginPath(); live.arc(px(at[0]), pz(at[2]), radius, 0, Math.PI * 2);
            live.fillStyle = colorValue(players[index].color); live.fill();
            live.strokeStyle = colors.cup; live.lineWidth = 1.4 * ratio; live.stroke();
            if (big) { live.beginPath(); live.arc(px(at[0]), pz(at[2]), radius + 2.2 * ratio, 0, Math.PI * 2); live.strokeStyle = colors.tee; live.lineWidth = 1.6 * ratio; live.stroke(); }
            balls++;
        };
        players.forEach((player, index) => { if (index !== active && !player.done) ball(index, false); });
        if (players[active] && !players[active].done) ball(active, true);
    }
    function update(players, active, positions, poses) {
        last = { players, active, positions, poses };
        if (!visible || !projection || !course) return;
        const outlines = obstacleOutlines(course, poses), parts = [];
        for (const outline of outlines) for (let i = 0; i < outline.length; i += 2) parts.push(Math.round(px(outline[i])), Math.round(pz(outline[i + 1])));
        players.forEach((player, index) => { if (!player.done && positions[index]) parts.push(index === active ? 'a' : 'b', player.color, Math.round(px(positions[index][0])), Math.round(pz(positions[index][2]))); });
        const key = parts.join(',');
        if (key === liveKey) return;
        liveKey = key; drawLive(outlines, players, active, positions);
    }
    function redraw() {
        layout(); drawLane(); liveKey = '';
        if (!projection) { live.clearRect(0, 0, liveCanvas.width, liveCanvas.height); balls = 0; }
        else if (last) update(last.players, last.active, last.positions, last.poses);
    }
    const observer = new ResizeObserver(redraw);
    observer.observe(frameEl);

    return {
        setCourse(next) {
            course = next; geometry = laneGeometry(next); last = null;
            readColors(); redraw();
            live.clearRect(0, 0, liveCanvas.width, liveCanvas.height); balls = 0;
        },
        update,
        setVisible(on) {
            visible = !!on; element.dataset.visible = String(visible); toggle.setAttribute('aria-pressed', String(visible));
            if (visible) { liveKey = ''; redraw(); }
        },
        inspect() { return { visible, drawn, balls }; },
        dispose() { observer.disconnect(); }
    };
}
