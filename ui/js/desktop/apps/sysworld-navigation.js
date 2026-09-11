import * as THREE from 'three';

// Pure street navigation for the decorative residents: rounded routes, static obstacles and
// steering with lane changes and U-turns. No scene objects; the life module maps poses to meshes.
const SAMPLES = 512;
// Corner handles of 7.5 m give a 90° apex radius of ~5.3 m, so every lane offset (up to 4.2 m)
// stays on the inside of the curve without folding back on itself.
const CORNER = 7.5;
export function streetRoute(corners) {
  const path = new THREE.CurvePath(), points = corners.map(([x, z]) => new THREE.Vector3(x, 0, z));
  const arrivals = [], departures = [], count = points.length;
  for (let i = 0; i < count; i++) {
    const p = points[i], before = points[(i + count - 1) % count], after = points[(i + 1) % count];
    arrivals.push(p.clone().addScaledVector(before.clone().sub(p).normalize(), CORNER));
    departures.push(p.clone().addScaledVector(after.clone().sub(p).normalize(), CORNER));
  }
  for (let i = 0; i < count; i++) {
    path.add(new THREE.QuadraticBezierCurve3(arrivals[i], points[i], departures[i]));
    path.add(new THREE.LineCurve3(departures[i], arrivals[(i + 1) % count]));
  }
  const samples = path.getSpacedPoints(SAMPLES), tangents = [];
  for (let i = 0; i < SAMPLES; i++) {
    const a = samples[(i + SAMPLES - 1) % SAMPLES], b = samples[(i + 1) % SAMPLES];
    tangents.push(new THREE.Vector3(b.x - a.x, 0, b.z - a.z).normalize());
  }
  return { points: samples, tangents, length: path.getLength() };
}

// Street furniture footprints as circles (metres) covering the kit's exported bounds: trams are
// 7.6 x 2.9, planters 3.5 x 1.6, racks 1.4 x 1.2 and lamps a slim pole whose arm overhangs the
// sidewalk. The robot radius is added during probing.
export function obstaclesFrom(placements) {
  const out = [];
  const add = (p, dx, dz, r) => {
    const c = Math.cos(p.angle), s = Math.sin(p.angle);
    out.push({ x: p.x + dx * c + dz * s, z: p.z - dx * s + dz * c, r, asset: p.asset });
  };
  for (const p of placements) {
    if (p.z < -100) continue;
    if (p.asset === 'data-tram') for (const dx of [-2.4, 0, 2.4]) add(p, dx, 0, 1.5);
    else if (p.asset === 'server-rack') add(p, 0, 0, .8);
    else if (p.asset === 'street-lamp') add(p, 0, 0, .5);
    else if (p.asset === 'planter') { add(p, -.9, 0, .85); add(p, .9, 0, .85); }
  }
  return out;
}

export function createNavigator({ routes, obstacles = [], robotRadius = 1.3, lanes = [2.2, 0, -2.2, 3.9, -3.9], defaultLane = 2.2, speeds = [] }) {
  const paths = routes.map(streetRoute);
  const agents = routes.map((_, i) => ({
    route: i, s: paths[i].length * (i * .173 % 1), dir: 1, lane: defaultLane, laneTarget: defaultLane,
    speed: 0, target: 0, laneVel: 0, cruise: speeds[i] ?? 5.1 + i * .43, state: 'cruise', heading: 0, wait: 0, cooldown: 0,
    x: 0, z: 0, fx: 0, fz: 1, rx: 1, rz: 0, vx: 0, vz: 0, turns: 0,
  }));
  const P = new THREE.Vector3(), T = new THREE.Vector3(), probe = new THREE.Vector3();
  function sample(path, s, dir, point, tangent) {
    const cursor = (((s / path.length) % 1) + 1) % 1 * SAMPLES, index = Math.floor(cursor) % SAMPLES, blend = cursor - Math.floor(cursor);
    point.lerpVectors(path.points[index], path.points[(index + 1) % SAMPLES], blend);
    tangent.lerpVectors(path.tangents[index], path.tangents[(index + 1) % SAMPLES], blend).normalize().multiplyScalar(dir);
  }
  const wrap = angle => Math.atan2(Math.sin(angle), Math.cos(angle));
  function pose(a) {
    sample(paths[a.route], a.s, a.dir, P, T);
    a.fx = T.x; a.fz = T.z; a.rx = -T.z; a.rz = T.x;
    a.x = P.x + a.rx * a.lane; a.z = P.z + a.rz * a.lane;
  }
  function hitsObstacle(x, z) {
    for (const o of obstacles) {
      const r = o.r + robotRadius;
      if ((x - o.x) ** 2 + (z - o.z) ** 2 < r * r) return true;
    }
    return false;
  }
  // A lane is clear when the next eighteen metres at that offset avoid every static footprint;
  // the probe just behind keeps a resident from cutting back into furniture it is passing.
  function laneClear(a, lane) {
    const path = paths[a.route];
    for (const ahead of [-2.5, 0, 1, 3, 6, 9, 12, 15, 18]) {
      sample(path, a.s + a.dir * ahead, a.dir, P, T);
      probe.set(P.x - T.z * lane, 0, P.z + T.x * lane);
      if (hitsObstacle(probe.x, probe.z)) return false;
    }
    return true;
  }
  function reverse(a) {
    a.dir = -a.dir; a.lane = -a.lane; a.laneTarget = defaultLane; a.laneVel = 0;
    a.state = 'turn'; a.cooldown = 4; a.wait = 0; a.target = 0; a.turns++;
  }
  function steer(a, dt) {
    if (a.cooldown > 0) a.cooldown -= dt;
    if (a.state === 'turn') {
      a.target = 0;
      if (a.speed < .05) {
        const goal = Math.atan2(a.fx, a.fz), diff = wrap(goal - a.heading);
        a.heading += diff * Math.min(1, dt * 3.2);
        if (Math.abs(diff) < .1) { a.heading = goal; a.state = 'cruise'; }
      }
      return;
    }
    // Prefer the right-hand default lane; residents return to it once an obstacle is passed.
    let chosen = null;
    const ordered = [...lanes].sort((p, q) => Math.abs(p - defaultLane) - Math.abs(q - defaultLane) || q - p);
    for (const lane of ordered) if (laneClear(a, lane)) { chosen = lane; break; }
    if (chosen === null) { if (a.cooldown <= 0) reverse(a); else a.target = 0; return; }
    a.laneTarget = chosen; a.target = a.cruise;
    // Slow down while sidestepping out of a blocked lane so the lateral move completes first.
    if (Math.abs(a.laneTarget - a.lane) > .5 && !laneClear(a, a.lane)) a.target = Math.min(a.target, 2.4);
    let conflict = false;
    for (const b of agents) {
      if (b === a) continue;
      const hit = firstConflict(a, a.laneTarget, b);
      if (!hit) continue;
      conflict = true;
      const facing = b.fx * a.fx + b.fz * a.fz, stopped = b.speed < .3;
      if (stopped || facing < -.2) {
        // Parked traffic: step into any lane whose path stays clear. Oncoming traffic: only step
        // away from where it is heading, never across its path; otherwise hold.
        const far = futures.get(b)[FUTURE.length - 1], away = stopped ? 0 : Math.sign((far.x - a.x) * a.rx + (far.z - a.z) * a.rz) || -1;
        let free = null;
        for (const lane of ordered) {
          if (away && Math.sign(lane - a.lane) === away) continue;
          if (laneClear(a, lane) && !firstConflict(a, lane, b)) { free = lane; break; }
        }
        if (free !== null) a.laneTarget = free; else a.target = 0;
      } else if (facing > .5) {
        // Following traffic keeps distance behind the leader; the leader ignores its follower.
        if (hit.da >= hit.db) a.target = Math.min(a.target, hit.da < 3.8 ? 0 : b.speed * .9);
      } else if (hit.da > hit.db || (hit.da === hit.db && a.route > b.route)) {
        // Crossing or merging traffic: whoever is further from the meeting point gives way.
        a.target = Math.min(a.target, hit.da <= 4 ? 0 : 2);
      }
    }
    if (conflict && a.target === 0) a.wait += dt; else a.wait = 0;
    if (a.wait > 2.5 && a.cooldown <= 0) reverse(a);
  }
  // Predicted meeting of two residents: samples along both routes at two-metre steps, with
  // similar arrival times, closer than two bodies plus a margin. Returns the distances to the
  // meeting point for each side or null.
  const FUTURE = [0, 2, 4, 6, 8, 10, 12], futures = new Map(), Q = new THREE.Vector3();
  function futurePoint(agent, ahead, lane, out) {
    sample(paths[agent.route], agent.s + agent.dir * ahead, agent.dir, P, T);
    out.set(P.x - T.z * lane, 0, P.z + T.x * lane);
  }
  function firstConflict(a, lane, b) {
    const points = futures.get(b), limit = (robotRadius * 2 + .6) ** 2, stopped = b.speed < .3;
    for (let i = 0; i < FUTURE.length; i++) {
      futurePoint(a, FUTURE[i], lane, Q);
      const ta = FUTURE[i] / Math.max(a.speed, 1.5);
      for (let j = 0; j < FUTURE.length; j++) {
        if (stopped && j > 0) break;
        const tb = FUTURE[j] / Math.max(b.speed, 1.5);
        if (FUTURE[i] > 4 && Math.abs(ta - tb) > 1.6) continue;
        if (Q.distanceToSquared(points[j]) < limit) return { da: FUTURE[i], db: FUTURE[j] };
      }
    }
    return null;
  }
  function step(dt) {
    if (!(dt > 0)) return;
    for (const a of agents) {
      pose(a);
      if (!futures.has(a)) futures.set(a, FUTURE.map(() => new THREE.Vector3()));
      const points = futures.get(a);
      // A stopped or turning resident only occupies its current spot.
      FUTURE.forEach((ahead, i) => futurePoint(a, a.state === 'cruise' ? ahead : 0, a.lane, points[i]));
    }
    for (const a of agents) steer(a, dt);
    // Hard separation nudges lanes so overlapping bodies never render through each other.
    for (let i = 0; i < agents.length; i++) for (let j = i + 1; j < agents.length; j++) {
      const a = agents[i], b = agents[j], dx = b.x - a.x, dz = b.z - a.z, dist = Math.hypot(dx, dz), minimum = robotRadius * 2 + .2;
      if (dist >= minimum) continue;
      // A standing resident keeps its spot; the moving one takes the whole (rate-limited) nudge.
      const stillA = a.state === 'turn' || a.speed < .3, stillB = b.state === 'turn' || b.speed < .3;
      const share = stillA === stillB ? .5 : stillA ? 0 : 1;
      const push = Math.min(minimum - Math.max(dist, .01), 2.5 * dt), sideA = dx * a.rx + dz * a.rz, sideB = -(dx * b.rx + dz * b.rz);
      a.lane = THREE.MathUtils.clamp(a.lane - Math.sign(sideA || 1) * push * share, -4.2, 4.2);
      b.lane = THREE.MathUtils.clamp(b.lane - Math.sign(sideB || 1) * push * (1 - share), -4.2, 4.2);
      a.target = Math.min(a.target, .5); b.target = Math.min(b.target, .5);
    }
    for (const a of agents) {
      const px = a.x, pz = a.z, accel = a.target > a.speed ? 6 : 9;
      a.speed += THREE.MathUtils.clamp(a.target - a.speed, -accel * dt, accel * dt);
      if (a.speed < 0) a.speed = 0;
      a.s += a.dir * a.speed * dt;
      // Lateral moves scale with forward speed and brake into the target lane, so residents
      // drive diagonally into a new lane instead of sliding sideways while stopped or turning.
      const gap = a.laneTarget - a.lane, cap = a.state === 'turn' ? 0 : Math.min(3, a.speed * .6);
      const want = Math.sign(gap) * Math.min(cap, Math.sqrt(8 * Math.abs(gap)), Math.abs(gap) / dt);
      a.laneVel += THREE.MathUtils.clamp(want - a.laneVel, -4 * dt, 4 * dt);
      a.lane += a.laneVel * dt;
      pose(a);
      a.vx = (a.x - px) / dt; a.vz = (a.z - pz) / dt;
      if (a.state === 'cruise') {
        // Moving residents face their velocity; a waiting resident squares up with the street.
        const goal = a.speed > .25 ? Math.atan2(a.vx, a.vz) : Math.atan2(a.fx, a.fz);
        a.heading += wrap(goal - a.heading) * Math.min(1, dt * (a.speed > .25 ? 8 : 3.2));
      }
    }
  }
  // Spawn on a clear stretch of street, away from furniture and the residents placed before.
  const placed = [];
  for (const a of agents) {
    for (let tries = 0; tries < 600; tries++) {
      pose(a);
      const spaced = placed.every(b => Math.hypot(a.x - b.x, a.z - b.z) > robotRadius * 2 + 6);
      if (spaced && laneClear(a, a.lane)) break;
      a.s += 1;
    }
    a.heading = Math.atan2(a.fx, a.fz); placed.push(a);
  }
  return {
    agents, step, hitsObstacle,
    stats: () => ({ states: agents.map(a => a.state), turns: agents.reduce((n, a) => n + a.turns, 0), lanes: agents.map(a => Math.round(a.lane * 10) / 10) }),
  };
}
