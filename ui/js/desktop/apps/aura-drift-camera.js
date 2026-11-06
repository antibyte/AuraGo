/* Aura Drift - pure presentation camera (no THREE, no DOM).
 *
 * Space conventions, shared with the 3D renderer and the debug API:
 *   - simulation: (x, y) on the ground plane, z up, psi = heading, +psi turns
 *     the nose to the driver's right when seen from behind;
 *   - camera space ("three space"): [x, up, y_sim]. Forward = [cos h, 0, sin h],
 *     right = norm([-f.z, 0, f.x]) = [-sin h, 0, cos h].
 * Mapping sim y onto three z mirrors the maths orientation, which is exactly
 * why steering right (psi grows) moves the nose to the right of the screen.
 * The 2D fallback rotates the world so the chase heading points up the screen
 * (screenRotation), instead of keeping a fixed north-up view that put every
 * race start (heading pi/2) upside down. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;
    const wrap = AD.wrapAngle;

    AD.CAMERA_MODES = ['chase', 'high', 'drone', 'bumper'];

    /* zoom2d scales the 2D fallback so the modes still read differently there. */
    const PARAMS = {
        chase: { dist: 9.5, height: 3.7, ahead: 7, targetY: 1.0, fov: 60, velBlend: 0.45, follow: 5.5, zoom2d: 1 },
        high: { dist: 15, height: 8, ahead: 12, targetY: 0.5, fov: 56, velBlend: 0.35, follow: 4.5, zoom2d: 0.85 },
        drone: { dist: 7, height: 46, ahead: 8, targetY: 0, fov: 48, velBlend: 0.20, follow: 3.2, zoom2d: 0.7 },
        bumper: { dist: -1.0, height: 1.05, ahead: 30, targetY: 0.9, fov: 72, velBlend: 0, follow: 30, zoom2d: 1.2 }
    };

    /* Body height and length in metres, mirrored from the 3D manifest (a contract test compares them). Tall cars (Big Bertha,
       Softeis) would fill the middle of the screen and hide the road, so the seat rises and backs off with the height. */
    AD.CAR_VIEW = {
        vektor: { h: 1.149, l: 4.605 }, kobalt: { h: 1.3, l: 4.82 }, lumen: { h: 1.35, l: 4.835 }, tundra: { h: 1.62, l: 3.9 },
        softeis: { h: 2.4, l: 5.1 }, maehkoenig: { h: 1.5, l: 2.7 }, couch: { h: 1.62, l: 3.0 }, bertha: { h: 3.1, l: 5.3 }
    };

    function worldVelocity(v) {
        if (v.airborne) return [v.wx || 0, v.wy || 0];
        const c = Math.cos(v.psi), s = Math.sin(v.psi);
        return [v.vx * c - v.vy * s, v.vx * s + v.vy * c];
    }

    function normalize(a) {
        const l = Math.hypot(a[0], a[1], a[2]);
        return l < 1e-9 ? [0, 0, 1] : [a[0] / l, a[1] / l, a[2] / l];
    }
    function cross(a, b) {
        return [a[1] * b[2] - a[2] * b[1], a[2] * b[0] - a[0] * b[2], a[0] * b[1] - a[1] * b[0]];
    }
    function dot(a, b) { return a[0] * b[0] + a[1] * b[1] + a[2] * b[2]; }
    function mix(a, b, u) { return [a[0] + (b[0] - a[0]) * u, a[1] + (b[1] - a[1]) * u, a[2] + (b[2] - a[2]) * u]; }

    /* Non-sim noise: the camera never touches the race rng. */
    function noise(state) {
        state.seed = (Math.imul(state.seed, 1664525) + 1013904223) >>> 0;
        return state.seed / 2147483648 - 1;
    }

    /* Heading follow: mostly the nose, partly the travel direction in a drift. */
    AD.chaseHeading = function (state, v, dt, params) {
        if (typeof state.h !== 'number' || !isFinite(state.h)) state.h = v.psi;
        const w = worldVelocity(v);
        const speed = Math.hypot(w[0], w[1]);
        const vdir = speed > 4 ? Math.atan2(w[1], w[0]) : v.psi;
        const off = wrap(vdir - v.psi);
        let blend = params.velBlend * clamp((speed - 6) / 24, 0, 1);
        if (v.vx < -0.5 && speed < 13) {
            /* Reversing (the solver caps it at 12 m/s): the view stays on the
               nose, so left/right keep pointing the way the keys do. A fast
               backwards slide is a spin and follows travel instead. */
            blend = 0;
        } else if (Math.abs(off) > 2.1) {
            blend = 1;                       /* spin: follow travel, no nausea */
        }
        const wanted = v.psi + off * blend;
        state.h = wrap(state.h + wrap(wanted - state.h) * (1 - Math.exp(-params.follow * Math.max(0, dt))));
        return state.h;
    };

    /* Chase pose in three space. state: { h, punch, shake, lift, seed }. */
    AD.computeChasePose = function (state, v, params) {
        const h = state.h;
        const w = worldVelocity(v);
        const speed = Math.hypot(w[0], w[1]);
        const fx = Math.cos(h), fy = Math.sin(h);
        /* bump: how far the body rises above a normal car (0..2 m). Not for the bumper view, which sits inside the car. */
        const view = AD.CAR_VIEW[v.car && v.car.id];
        const bump = params.dist > 0 && view ? clamp(view.h - 1.4, 0, 2) : 0;
        const d = params.dist + 0.025 * speed + 1.3 * bump;
        const lift = (state.lift || 0) * 1.4;
        let eye = [v.x - fx * d, v.z * 0.6 + params.height + 1.5 * bump + lift, v.y - fy * d];
        const target = [v.x + fx * (params.ahead + 2 * bump), v.z * 0.85 + params.targetY, v.y + fy * (params.ahead + 2 * bump)];
        let fov = params.fov;
        let roll = 0;
        if (!params.reduced) {
            fov += 10 * clamp((speed - 20) / 70, 0, 1) + 8 * (state.punch || 0);
            const s = state.shake || 0;
            if (s > 0.001) {
                const f = normalize([target[0] - eye[0], target[1] - eye[1], target[2] - eye[2]]);
                const right = normalize([-f[2], 0, f[0]]);
                const up = cross(right, f);
                const n1 = noise(state), n2 = noise(state), n3 = noise(state);
                eye = [
                    eye[0] + right[0] * n1 * s * 0.35 + up[0] * n2 * s * 0.25,
                    eye[1] + right[1] * n1 * s * 0.35 + up[1] * n2 * s * 0.25,
                    eye[2] + right[2] * n1 * s * 0.35 + up[2] * n2 * s * 0.25
                ];
                roll = n3 * s * 0.02;
            }
        }
        return {
            eye: eye, target: target, fov: fov, roll: roll, heading: h,
            simX: v.x, simY: v.y, scale2d: 5.6, speed: speed, shake: state.shake || 0
        };
    };

    /* Orthonormal basis of a pose (roll applied): { f, right, up }. The 3D
       renderer sets camera.up = basis.up before lookAt so both agree. */
    AD.cameraBasis = function (pose) {
        const e = pose.eye, t = pose.target;
        const f = normalize([t[0] - e[0], t[1] - e[1], t[2] - e[2]]);
        let right = [-f[2], 0, f[0]];
        right = Math.hypot(right[0], right[2]) < 1e-6 ? [1, 0, 0] : normalize(right);
        let up = cross(right, f);
        if (pose.roll) {
            const c = Math.cos(pose.roll), s = Math.sin(pose.roll);
            const r2 = [right[0] * c + up[0] * s, right[1] * c + up[1] * s, right[2] * c + up[2] * s];
            const u2 = [up[0] * c - right[0] * s, up[1] * c - right[1] * s, up[2] * c - right[2] * s];
            right = r2; up = u2;
        }
        return { f: f, right: right, up: up };
    };

    /* NDC projection of a three-space point: +x right, +y up. */
    AD.projectToScreen = function (pose, x, y, z, aspect) {
        const b = AD.cameraBasis(pose);
        const p = [x - pose.eye[0], y - pose.eye[1], z - pose.eye[2]];
        const depth = dot(p, b.f);
        if (depth <= 1e-4) return { x: 0, y: 0, depth: depth, visible: false };
        const t = Math.tan(pose.fov * Math.PI / 360);
        const a = aspect || 16 / 9;
        const sx = dot(p, b.right) / (depth * t * a);
        const sy = dot(p, b.up) / (depth * t);
        return { x: sx, y: sy, depth: depth, visible: Math.abs(sx) <= 1 && Math.abs(sy) <= 1 };
    };

    /* 2D fallback: rotate the world so the chase heading points up the screen. */
    AD.screenRotation = h => -Math.PI / 2 - h;

    AD.project2D = function (cam, x, y, w, h) {
        const theta = typeof cam.theta === 'number' ? cam.theta : AD.screenRotation(cam.heading || 0);
        const cx = cam.x !== undefined ? cam.x : cam.simX;
        const cy = cam.y !== undefined ? cam.y : cam.simY;
        const scale = cam.scale !== undefined ? cam.scale : (cam.scale2d || 5.6);
        const dx = (x - cx) * scale, dy = (y - cy) * scale;
        const c = Math.cos(theta), s = Math.sin(theta);
        return [w / 2 + dx * c - dy * s, h / 2 + dx * s + dy * c];
    };

    /* Hit-stop, slow-mo and the finish slow-down only scale how many fixed steps
       run per wall second, so tapes and the simulation stay untouched. */
    const WARPS = { hitstop: [0.06, 0.1], slowmo: [0.35, 0.4], finish: [1.0, 0.35] };
    AD.createTimeWarp = function (options) {
        const reduced = !!(options && options.reduced);
        const active = [];
        const warp = {
            value: 1,
            reduced: reduced,
            trigger(kind) {
                const def = WARPS[kind];
                if (!def || warp.reduced) return;
                active.push({ left: def[0], scale: def[1], kind: kind });
            },
            scale(dt) {
                let value = 1;
                for (let i = active.length - 1; i >= 0; i--) {
                    value = Math.min(value, active[i].scale);
                    active[i].left -= dt;
                    if (active[i].left <= 0) active.splice(i, 1);
                }
                warp.value = warp.reduced ? 1 : value;
                return dt * warp.value;
            },
            onEvent(evt) {
                if (warp.reduced) return;
                if (evt.type === 'landing' && evt.car === 0) {
                    if (evt.quality === 'crash') warp.trigger('hitstop');
                    else if (evt.quality === 'perfect') warp.trigger('slowmo');
                } else if (evt.type === 'rail_fire' && evt.car === 0) {
                    warp.trigger('hitstop');
                } else if (evt.type === 'hit' && evt.car === 0 && evt.heavy) {
                    warp.trigger('hitstop');
                } else if (evt.type === 'finish' && evt.car === 0) {
                    warp.trigger('finish');
                }
            }
        };
        return warp;
    };

    AD.createChaseCamera = function (options) {
        const st = {
            h: undefined, punch: 0, kick: 0, shake: 0, lift: 0, seed: 0x9e3779b9,
            scale: 5.6, lead: 0, orbit: 0, script: null, scriptT: 0
        };
        const cam = {
            mode: AD.CAMERA_MODES.includes(options && options.mode) ? options.mode : 'chase',
            pose: null,
            get heading() { return st.h === undefined ? 0 : st.h; },
            snap() { st.h = undefined; st.lead = 0; st.lift = 0; st.shake = 0; st.punch = 0; st.kick = 0; },
            reset() { cam.snap(); cam.script('none'); },
            setMode(m) { if (AD.CAMERA_MODES.includes(m)) cam.mode = m; return cam.mode; },
            cycle() {
                const i = AD.CAMERA_MODES.indexOf(cam.mode);
                cam.mode = AD.CAMERA_MODES[(i + 1) % AD.CAMERA_MODES.length];
                return cam.mode;
            },
            script(kind) {
                st.script = kind === 'intro' || kind === 'orbit' ? kind : null;
                st.scriptT = 0;
                st.orbit = 0;
            },
            addShake(a) { st.shake = Math.min(1, st.shake + a); },
            kick(a) { st.kick = Math.min(1, Math.max(st.kick, a)); },
            onEvent(evt) {
                const mine = evt.car === 0;
                switch (evt.type) {
                    case 'wall_hit': if (mine) cam.addShake(evt.heavy ? 0.5 : 0.2); break;
                    case 'car_hit': if (evt.a === 0 || evt.b === 0) cam.addShake(evt.heavy ? 0.55 : 0.25); break;
                    case 'hit': if (mine) cam.addShake(evt.heavy ? 0.7 : 0.4); break;
                    case 'near_miss': if (mine) cam.addShake(0.18); break;
                    case 'turbo_burst': case 'boost_pad': if (mine) cam.kick(1); break;
                    case 'landing':
                        if (mine) {
                            if (evt.quality === 'crash') cam.addShake(0.8);
                            else if (evt.quality === 'perfect') cam.kick(0.6);
                        }
                        break;
                    case 'rail_fire': if (mine) cam.addShake(0.4); break;
                    case 'eliminated': case 'bomb_blast': if (mine || evt.type === 'bomb_blast') cam.addShake(0.5); break;
                    case 'cow_land': {
                        const p = cam.pose;
                        const d = p ? Math.hypot(evt.x - p.simX, evt.y - p.simY) : 0;
                        cam.addShake(clamp(0.7 * (1 - d / 70), 0, 0.7));
                        break;
                    }
                    case 'finish': if (mine) cam.script('orbit'); break;
                    default: break;
                }
            },
            /* opts: { reduced, paused, lowZoom }. The car being followed is cars[0]. */
            update(race, dt, opts) {
                opts = opts || {};
                const reduced = !!opts.reduced;
                const step = opts.paused ? 0 : dt;
                const v = race.cars[0].v;
                const base = PARAMS[cam.mode];
                const params = Object.assign({}, base, reduced
                    ? { follow: base.follow * 1.6, reduced: true } : { reduced: false });
                AD.chaseHeading(st, v, step, params);
                const k = 1 - Math.exp(-7 * step);
                st.punch += (clamp((v.boost ? 0.7 : 0) + st.kick, 0, 1) - st.punch) * k;
                st.kick = Math.max(0, st.kick - step * 3);
                st.lift += ((v.airborne ? 1 : 0) - st.lift) * (1 - Math.exp(-3 * step));
                const pose = AD.computeChasePose(st, v, params);
                st.shake = Math.max(0, st.shake - step * 2.4);

                if (st.script) cam.applyScript(pose, v, step, reduced, params);

                /* 2D fallback fields: zoom out with speed, look ahead along the heading. */
                const boost = v.boost ? 1 : 0;
                const zoomTo = (opts.lowZoom ? 5.0 : clamp(7.0 - pose.speed * 0.02, 5.2, 7.0)) *
                    base.zoom2d * (1 - boost * 0.06);
                st.scale = AD.smooth(st.scale, zoomTo, boost ? 7 : 2.4, step);
                st.lead = AD.smooth(st.lead, reduced ? 0 : clamp(pose.speed * 0.3, 0, 24), 3, step);
                pose.scale2d = st.scale;
                pose.simX = v.x + Math.cos(st.h) * st.lead;
                pose.simY = v.y + Math.sin(st.h) * st.lead;
                pose.theta = AD.screenRotation(st.h);
                cam.pose = pose;
                return pose;
            },
            applyScript(pose, v, step, reduced, params) {
                st.scriptT += step;
                if (st.script === 'intro') {
                    if (reduced) { st.script = null; return; }
                    const dur = 3;
                    const u = clamp(st.scriptT / dur, 0, 1);
                    const e = u * u * (3 - 2 * u);
                    if (u >= 1) { st.script = null; return; }
                    /* Swing from high in front of the car round to the chase seat. */
                    const phi = Math.PI * (1 - e);
                    const dd = 14 + (params.dist + 0.025 * pose.speed - 14) * e;
                    const a = st.h + phi;
                    const eye = [v.x - Math.cos(a) * dd, 6 + (params.height - 6) * e + v.z * 0.6, v.y - Math.sin(a) * dd];
                    pose.eye = eye;
                    pose.target = mix([v.x, v.z + 0.8, v.y], pose.target, e);
                    pose.fov = params.fov;
                    pose.roll = 0;
                } else if (st.script === 'orbit') {
                    /* Victory lap: the car keeps driving, so this is a slow high chase that sways behind it. At 10.5 m up and 13 m back
                       it stays above the gantry (7 m), the stands (8.9 m) and every prop, which the scatter keeps well off the road.
                       Reduced motion: a fixed 3/4 view from behind-right of the moving car. */
                    st.orbit += reduced ? 0 : step * 0.4;
                    const phi = reduced ? 0.7 : 0.95 * Math.sin(st.orbit);
                    const a = st.h + Math.PI + phi;
                    pose.eye = [v.x + Math.cos(a) * 13, v.z * 0.6 + 10.5, v.y + Math.sin(a) * 13];
                    pose.target = [v.x + Math.cos(st.h) * 5, v.z + 0.8, v.y + Math.sin(st.h) * 5];
                    pose.fov = 58;
                    pose.roll = 0;
                }
            }
        };
        return cam;
    };
})();
