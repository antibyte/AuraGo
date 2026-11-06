/* Aura Drift - deterministic bicycle-model vehicle dynamics. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    const M = 1250;            /* kg */
    const G = 9.81;
    const IZ = 1750;           /* kg m^2 */
    const A = 1.25;            /* front axle distance from centre */
    const B = 1.45;            /* rear axle distance from centre */
    const L = A + B;
    const CG_HEIGHT = 0.52;
    const CF = 62000;          /* front cornering stiffness N/rad */
    const CR = 68000;          /* rear cornering stiffness N/rad */
    const DRAG = 0.85;
    const BRAKE_FORCE = 14000;
    const MAX_STEER = 0.50;
    const WALL_MARGIN = 1.5;

    AD.createVehicle = function (x, y, heading, carIndex) {
        const car = AD.CARS[carIndex % AD.CARS.length];
        return {
            x: x, y: y, psi: heading,
            vx: 0, vy: 0, r: 0,
            steer: 0, throttle: 0, brake: 0,
            gear: 1, rpm: 900, slip: 0, surface: 'asphalt',
            damage: 0, disabled: 0, boost: 0, shield: 0,
            nitro: 1, brakeHold: 0, slipstream: 0,
            impact: 0, carIndex: carIndex, car: car,
            wallContacts: 0,
            /* Section 2.2: height, flight, timers and effect state. */
            z: 0, vz: 0, airborne: false, airTime: 0,
            roll: 0, pitch: 0, rollRate: 0, pitchRate: 0, spinRate: 0,
            rollAccum: 0, pitchAccum: 0, spinAccum: 0,
            wx: 0, wy: 0, onRamp: -1, landWobble: 0,
            burst: 0, burstPower: 1, driftCharge: 0, driftLevel: 0, draftTime: 0,
            frozen: 0, steerWobble: 0, wobblePhase: 0, heat: 0, toast: 0,
            cloak: 0, overdrive: 0, towedBy: -1, towing: -1,
            out: false, hasBomb: false, spinKick: 0
        };
    };

    AD.resetVehicle = function (v, x, y, heading) {
        v.x = x; v.y = y; v.psi = heading;
        v.vx = 0; v.vy = 0; v.r = 0;
        v.steer = 0; v.throttle = 0; v.brake = 0;
        v.gear = 1; v.rpm = 900; v.slip = 0;
        v.damage = 0; v.disabled = 0; v.boost = 0; v.shield = 0;
        v.nitro = 1; v.brakeHold = 0; v.slipstream = 0;
        v.handbrake = 0;
        v.impact = 0; v.wallContacts = 0;
        v.z = 0; v.vz = 0; v.airborne = false; v.airTime = 0;
        v.roll = 0; v.pitch = 0; v.rollRate = 0; v.pitchRate = 0; v.spinRate = 0;
        v.rollAccum = 0; v.pitchAccum = 0; v.spinAccum = 0;
        v.wx = 0; v.wy = 0; v.onRamp = -1; v.landWobble = 0;
        v.burst = 0; v.burstPower = 1; v.driftCharge = 0; v.driftLevel = 0; v.draftTime = 0;
        v.frozen = 0; v.steerWobble = 0; v.wobblePhase = 0; v.heat = 0; v.toast = 0;
        v.cloak = 0; v.overdrive = 0; v.towedBy = -1; v.towing = -1;
        v.out = false; v.hasBomb = false; v.spinKick = 0;
    };

    AD.vehicleState = function (v) {
        return [v.x, v.y, v.psi, v.vx, v.vy, v.r, v.steer, v.rpm, v.nitro, v.slipstream,
            v.z, v.vz, v.roll, v.pitch, v.burst];
    };

    /* env: { grip, roll, disabled, boostBonus }. Pure: no randomness, no clock. */
    AD.stepVehicle = function (v, input, dt, env) {
        const grip = clamp((env && env.grip) || 1, 0.12, 1.3);
        const roll = (env && env.roll !== undefined) ? env.roll : 0.016;
        const disabled = (env && env.disabled) || v.disabled > 0;
        const handbrake = input.handbrake ? 1 : 0;
        const boost = input.boost ? 1 : 0;
        /* The handbrake is published on the vehicle so the renderer can light
           the brake lamps and the audio can hear a locked rear axle. */
        v.handbrake = handbrake;
        /* Car stats (section 2.2). Mass scales every force that is a weight
           (inertia, tyre stiffness, brakes), so it changes acceleration,
           collision weight and shove but not the handling feel. */
        const c = v.car;
        const Mm = M * c.mass, IZm = IZ * c.mass, CFm = CF * c.mass, CRm = CR * c.mass;
        const power = c.power * (1 - 0.32 * clamp(v.damage / 100, 0, 1)) * (v.toast > 0 ? 0.6 : 1);
        const overdrive = v.overdrive > 0;

        const speed = Math.abs(v.vx);
        /* Arcade nose response: left/right always rotate the nose the way the
           keys point, forward or reverse. The signed travel direction still
           drives the longitudinal forces below; only the steering term uses
           the magnitude so the response fades to zero at a standstill instead
           of snapping through zero the way a sign-gated flip did. The old
           input-side flip (threshold -0.8 m/s) disagreed with this ramp (full
           -1 below -0.6 m/s) and inverted the steering in between. */
        const dir = clamp(v.vx / 0.6, -1, 1);
        const steerMag = Math.abs(dir);
        const steerLimit = MAX_STEER * c.steerK / (1 + speed * 0.018);
        v.steer = AD.smooth(v.steer, clamp(input.steer || 0, -1, 1) * steerLimit, 14, dt);
        const delta = v.steer;

        const denom = Math.max(speed, 2.5);
        const alphaF = Math.atan2(v.vy + A * v.r, denom) - delta * steerMag;
        const alphaR = Math.atan2(v.vy - B * v.r, denom);

        const throttle = disabled ? 0 : clamp(input.throttle || 0, 0, 1);
        const brake = clamp(input.brake || 0, 0, 1);
        /* Holding the brake at a standstill engages a gentle reverse after a
           short beat, so cars can unwind from barriers; it never oscillates
           through zero the way a sign-gated brake force did. The latch keeps
           reversing until the pedal is released, so it cannot pulse. */
        if (!disabled && brake > 0.05 && throttle <= 0 && (speed < 1.4 || v.brakeHold > 0.45)) v.brakeHold += dt;
        else v.brakeHold = 0;
        const reversing = v.brakeHold > 0.45;
        let drive = throttle * 9500 * power * (1 - 0.55 * clamp(v.vx / 75, 0, 1));
        if (reversing) {
            drive = -2600 * brake * clamp(1 + v.vx / 6.5, 0, 1) * power;
        } else if (overdrive) {
            drive *= 1.9;                 /* nitro is not spent while it lasts */
        } else if (boost && !disabled && v.nitro > 0.02) {
            drive *= 1.55;
            v.nitro = Math.max(0, v.nitro - dt * 0.32);
        }
        /* Pads, drift turbos and perfect landings force a drive boost. */
        if (v.burst > 0 && !reversing && !disabled) {
            drive = Math.max(drive * v.burstPower, 3200 * c.mass);
        }
        /* Drifting at speed refills the nitro resource (deterministic state). */
        const draft = clamp((env && env.slipstream) || 0, 0, 1);
        v.slipstream = draft;
        if (!disabled && speed > 12 && v.slip > 2.0) {
            v.nitro = Math.min(1, v.nitro + dt * clamp(v.slip * 0.045, 0, 0.2) * c.driftNitro);
        }
        /* Slipstreaming a rival's wake refills nitro and trims drag. */
        if (!disabled && draft > 0.05 && speed > 18) {
            v.nitro = Math.min(1, v.nitro + dt * 0.22 * draft);
        }
        /* Brake force fades to zero at standstill: brakes hold a stopped car
           instead of pumping it backwards against the next substep. */
        const brakeFade = clamp(speed / 1.2, 0, 1);
        const braking = reversing ? 0 : (brake * BRAKE_FORCE * c.mass + handbrake * 5200 * c.mass) * brakeFade;
        const drag = DRAG * c.dragK * (1 - 0.32 * draft) * v.vx * Math.abs(v.vx);
        const rolling = roll * Mm * G * dir * brakeFade;
        const fx = drive - dir * braking - drag - rolling;

        const transfer = clamp(fx * CG_HEIGHT / L, -0.35 * Mm * G, 0.35 * Mm * G);
        const fzf = Math.max(900 * c.mass, Mm * G * B / L - transfer);
        const fzr = Math.max(900 * c.mass, Mm * G * A / L + transfer);
        /* D3: the car's own grip stat finally reaches the tyres; a rough
           landing halves the available grip for a moment. */
        const gripNow = grip * c.grip * (v.landWobble > 0 ? 0.55 : 1);
        const muFront = gripNow;
        const muRear = gripNow * (handbrake ? 0.42 : 1);
        const fyf = clamp(-CFm * alphaF, -muFront * fzf, muFront * fzf);
        const fyr = clamp(-CRm * alphaR, -muRear * fzr, muRear * fzr);

        const ax = fx / Mm + v.vy * v.r;
        const ay = (fyf * Math.cos(delta) + fyr) / Mm - v.vx * v.r;
        const rdot = (A * fyf * Math.cos(delta) - B * fyr) / IZm;

        v.vx += ax * dt;
        v.vy += ay * dt;
        v.r += rdot * dt;

        /* Low-speed stabilisation keeps the kinematic bicycle model well behaved. */
        if (speed < 3) {
            v.vy *= Math.exp(-5.5 * dt);
            v.r *= Math.exp(-4.0 * dt);
        }
        v.vy *= Math.exp(-1.2 * dt);
        v.r *= Math.exp(-0.5 * dt);
        v.vx = clamp(v.vx, -12, overdrive ? 105 : 95);
        v.vy = clamp(v.vy, -45, 45);
        v.r = clamp(v.r, -3.2, 3.2);
        v.psi = AD.wrapAngle(v.psi + v.r * dt);

        const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        v.x += (v.vx * cos - v.vy * sin) * dt;
        v.y += (v.vx * sin + v.vy * cos) * dt;

        v.slip = Math.abs(v.vy) + Math.abs(alphaF) * 2 + Math.abs(alphaR) * 2;
        v.throttle = throttle;
        v.brake = brake;
        v.boost = (!reversing && boost && !disabled && v.nitro > 0) ? 1 : 0;

        /* Gear and engine speed are presentation state derived from velocity. */
        const gearSpeeds = [0, 14, 27, 40, 54, 68, 82];
        let gear = 1;
        for (let i = 1; i < gearSpeeds.length; i++) if (v.vx >= gearSpeeds[i]) gear = i + 1;
        v.gear = reversing || v.vx < -0.5 ? 0 : clamp(gear, 1, 6);
        const low = v.gear >= 1 ? (gearSpeeds[v.gear - 1] || 0) : 0;
        const high = v.gear >= 1 ? (gearSpeeds[v.gear] || 95) : 14;
        v.rpm = 900 + 5900 * clamp((Math.abs(v.vx) - low) / Math.max(1, high - low), 0, 1) + throttle * 450;

        if (v.disabled > 0) v.disabled = Math.max(0, v.disabled - dt);
        if (v.shield > 0) v.shield = Math.max(0, v.shield - dt);
        if (v.burst > 0) v.burst = Math.max(0, v.burst - dt);
        if (v.landWobble > 0) v.landWobble = Math.max(0, v.landWobble - dt);
        v.impact = Math.max(0, v.impact - dt * 2.2);
        return v;
    };

    /* sample: { nx, ny, offset, halfWidth }. Offset is signed lateral distance.
       In flight the world velocity (wx, wy) is the truth; the body-frame
       vx, vy are kept in sync for the HUD and audio. */
    AD.resolveWall = function (v, sample, dt) {
        if (!sample || !isFinite(sample.offset)) return false;
        const limit = Math.max(2, sample.halfWidth - WALL_MARGIN);
        const over = Math.abs(sample.offset) - limit;
        if (over <= 0) return false;

        const sign = sample.offset >= 0 ? 1 : -1;
        const nx = sample.nx * sign, ny = sample.ny * sign;
        v.x -= nx * over;
        v.y -= ny * over;

        const cos = Math.cos(v.psi), sin = Math.sin(v.psi);
        let wx, wy;
        if (v.airborne) { wx = v.wx; wy = v.wy; } else {
            wx = v.vx * cos - v.vy * sin;
            wy = v.vx * sin + v.vy * cos;
        }
        const vn = wx * nx + wy * ny;
        if (vn > 0) {
            wx -= 1.25 * vn * nx;
            wy -= 1.25 * vn * ny;
            v.damage = clamp(v.damage + Math.min(18, vn * 0.55) * (v.overdrive > 0 ? 1.5 : 1), 0, 100);
            v.impact = Math.min(1, vn / 22);
            v.wallContacts++;
            wx *= 0.88; wy *= 0.88;
        }
        if (v.airborne) { v.wx = wx; v.wy = wy; }
        v.vx = wx * cos + wy * sin;
        v.vy = -wx * sin + wy * cos;
        v.r *= 0.72;
        return true;
    };

    /* Circle-based impulse for car-to-car contact; both cars share the solver.
       Heavier cars shove lighter ones: impulse_a = -rel * 1.24 * m_b / (m_a + m_b),
       which is the old 0.62 for equal masses. Returns the closing speed that was
       resolved (0 when the cars only overlapped or were already separating). */
    AD.resolveCarPair = function (a, b, radius) {
        const r = radius || 2.0;
        const dx = b.x - a.x, dy = b.y - a.y;
        const dist = Math.hypot(dx, dy);
        if (dist >= r * 2 || dist < 0.0001) return 0;
        const nx = dx / dist, ny = dy / dist;
        const overlap = r * 2 - dist;
        a.x -= nx * overlap * 0.5; a.y -= ny * overlap * 0.5;
        b.x += nx * overlap * 0.5; b.y += ny * overlap * 0.5;
        const ac = Math.cos(a.psi), as = Math.sin(a.psi);
        const bc = Math.cos(b.psi), bs = Math.sin(b.psi);
        let awx = a.airborne ? a.wx : a.vx * ac - a.vy * as;
        let awy = a.airborne ? a.wy : a.vx * as + a.vy * ac;
        let bwx = b.airborne ? b.wx : b.vx * bc - b.vy * bs;
        let bwy = b.airborne ? b.wy : b.vx * bs + b.vy * bc;
        const relative = (bwx - awx) * nx + (bwy - awy) * ny;
        if (relative > 0) return 0;
        const ma = a.car.mass, mb = b.car.mass;
        const impulseA = -relative * 1.24 * mb / (ma + mb);
        const impulseB = -relative * 1.24 * ma / (ma + mb);
        awx -= impulseA * nx; awy -= impulseA * ny;
        bwx += impulseB * nx; bwy += impulseB * ny;
        if (a.airborne) { a.wx = awx; a.wy = awy; }
        if (b.airborne) { b.wx = bwx; b.wy = bwy; }
        a.vx = awx * ac + awy * as; a.vy = -awx * as + awy * ac;
        b.vx = bwx * bc + bwy * bs; b.vy = -bwx * bs + bwy * bc;
        a.impact = Math.max(a.impact, 0.35);
        b.impact = Math.max(b.impact, 0.35);
        return -relative;
    };

    AD.vehicleSpeedKmh = v => Math.max(0, v.vx) * 3.6;
})();
