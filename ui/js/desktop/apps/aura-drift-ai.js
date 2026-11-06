/* Aura Drift - opponent driving: racing line, corner speed, recovery and hooks. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    /* Marshal: puts a pinned or wrecked car back on the racing line. */
    AD.respawnCar = function (race, car) {
        const v = car.v;
        const current = AD.sampleTrack(race.track, v.x, v.y);
        const point = AD.pointAt(race.track, current.s + 10);
        v.x = point.x; v.y = point.y;
        v.psi = Math.atan2(point.ty, point.tx);
        v.vx = 10; v.vy = 0; v.r = 0;
        v.z = 0; v.vz = 0; v.airborne = false; v.roll = 0; v.pitch = 0;
        v.disabled = 0; v.impact = 0.4;
        car.lastS = current.s;
        car.ai.stuckTime = 0;
        car.ai.pinnedTime = 0;
        car.ai.recover = 0;
        car.ai.respawns++;
        if (car.tally) car.tally.respawns++;
        AD.emit(race, 'respawn', { x: v.x, y: v.y, car: car.index });
    };

    AD.aiInput = function (race, car) {
        const v = car.v, track = race.track;
        const sample = AD.sampleTrack(track, v.x, v.y);
        const speed = Math.max(0, v.vx);
        const ahead = AD.pointAt(track, sample.s + 26 + speed * 0.85);
        const curvature = Math.abs(ahead.tx * sample.ty - ahead.ty * sample.tx);
        let lane = car.ai.lane * (1 - clamp(curvature * 4, 0, 0.8));
        /* Overtaking line: a rival just ahead is worth moving around, but
           only if there is room on one side of the corridor. */
        for (const other of race.cars) {
            if (other === car || other.v.out) continue;
            const dx = other.v.x - v.x, dy = other.v.y - v.y;
            const dist = Math.hypot(dx, dy);
            if (dist > 16 || dist < 3) continue;
            const aheadDot = (dx * Math.cos(v.psi) + dy * Math.sin(v.psi)) / Math.max(0.001, dist);
            if (aheadDot < 0.7) continue;
            const side = Math.sign(-dx * Math.sin(v.psi) + dy * Math.cos(v.psi)) || 1;
            lane += side * track.width * 0.26 * (1 - dist / 16);
        }
        /* Obstacles and crates nudge the line (metres, positive = left of travel). */
        lane += AD.featureLaneBias(race, car, sample);
        lane = clamp(lane, -track.width * 0.38, track.width * 0.38);
        const tx = ahead.x + ahead.nx * lane, ty = ahead.y + ahead.ny * lane;
        const wanted = Math.atan2(ty - v.y, tx - v.x);
        const error = AD.wrapAngle(wanted - v.psi);
        const input = car.input;
        input.steer = clamp(error * 2.4 - v.r * 0.22, -1, 1);

        /* Surface grip and corridor position cap the achievable corner speed.
           Without this the narrow, muddy rally stage becomes a wall-scraping
           convoy instead of a race. */
        const surface = AD.surfaceAt(track, sample.s, sample.offset, sample);
        /* Weather bites since D1 was fixed: drivers read it like a surface. */
        const weatherGrip = race.conditions ? race.conditions.state.grip : 1;
        const gripFactor = 0.58 + 0.42 * clamp(surface.grip * weatherGrip, 0.2, 1.2);
        const corridor = Math.abs(sample.offset) / Math.max(1, sample.halfWidth);
        const corridorFactor = 1 - 0.4 * clamp((corridor - 0.55) / 0.45, 0, 1);
        let leader = car.progress;
        for (const other of race.cars) leader = Math.max(leader, other.progress);
        const gap = Math.max(0, leader - car.progress);
        /* Bounded, disclosed catch-up: at most +6% target speed, never extra grip. */
        const catchup = 1 + clamp((gap - 150) / 900, 0, 1) * 0.06;
        car.ai.catchup = catchup > 1.001;
        car.ai.catchupFactor = catchup;

        const cornerLimit = 18 + 62 * car.ai.aggression * (car.ai.skill || 1) /
            Math.max(0.35, 1 + curvature * 14 + Math.abs(sample.offset) / 40);
        const target = clamp(cornerLimit * gripFactor * corridorFactor * catchup, 10, 74);
        input.throttle = speed < target ? 1 : 0;
        input.brake = speed > target + 5 ? clamp((speed - target) / 14, 0, 1) : 0;
        /* Rivals throw the car into the corner: the handbrake makes them
           slide, which reads on screen and feeds the style score. It is only
           safe where there is grip left - on mud or ice it would be a
           wall-scraping convoy. */
        input.handbrake = (curvature > 0.22 && speed > 24 && surface.grip > 0.6) ? 1 : 0;
        /* Same nitro resource as the player: burn it on clean straights only. */
        input.boost = (v.nitro > 0.55 && curvature < 0.02 && speed > 46) ? 1 : 0;
        input.fire = 0;

        if (corridor > 0.5) {
            input.steer = clamp(input.steer - Math.sign(sample.offset) * clamp((corridor - 0.5) * 1.6, 0, 0.85), -1, 1);
        }
        /* Two-stage recovery: hold the brake so the new reverse gear backs the
           car out straight, then let a marshal put it back on the racing line
           if it stays pinned. This keeps weapon spins from parking an opponent
           against a wall forever. */
        car.ai.recover = Math.max(0, car.ai.recover - AD.STEP);
        const pinned = speed < 4 && corridor > 0.65 && !v.airborne;
        car.ai.stuckTime = pinned ? car.ai.stuckTime + AD.STEP : 0;
        /* The recovery below resets stuckTime every 0.5 s, so the marshal needs
           its own continuous count (the original 4 s rule could never fire). */
        car.ai.pinnedTime = pinned ? (car.ai.pinnedTime || 0) + AD.STEP : 0;
        if (car.ai.pinnedTime > 4) AD.respawnCar(race, car);
        else if (car.ai.stuckTime > 0.5 && car.ai.recover <= 0) {
            car.ai.recover = 1.1;
            car.ai.stuckTime = 0;
        }
        if (car.ai.recover > 0) {
            input.throttle = 0;
            input.brake = 1;
            input.handbrake = 0;
            input.fire = 0;
            input.boost = 0;
            input.steer = 0;
            return input;
        }
        if (race.weapons && car.weapon && car.cooldown <= 0 && AD.aiWantsFire(race, car)) input.fire = 1;
        /* Mode rules (hot potato chase and flee), then the air plan if in flight. */
        AD.modeAiOverride(race, car, input);
        if (v.airborne) AD.aiAirInput(race, car, input);
        return input;
    };
})();
