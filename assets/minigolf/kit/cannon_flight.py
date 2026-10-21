"""Cannon barrel, launch velocity and flight checks (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. The ball is caught at the breech funnel above the zone, shows up at the muzzle,
is held ``delay`` seconds and is fired with ``velocity``. The generator derives the velocity from the landing
point and the flight time ``T`` in runtime coordinates, ``velocity = (target - muzzle - g T^2 / 2) / T`` with
``g = [0, -gravity, 0]``. The barrel axis is the launch direction: the muzzle sits ``barrel`` metres from the breech
point along it (a fixed point iteration, the elevation converges within a few steps).

``problems`` samples the flight (20 points) and reports every sample that is closer than ``CLEARANCE`` (0.5 m) to
another part of the course or leaves ``bounds``. The descent onto the landing spot is exempt from the floor and
rail checks within ``LANDING`` (1.0 m, plan view) of the target: there the ball comes down on purpose.
"""

import math

from .mechanic_floor import blender, rounded, runtime

FLIGHT, DELAY, BARREL = 1.4, 0.6, 1.5
BREECH_FORWARD, BREECH_HEIGHT = 0.55, 0.62     # breech point ahead of and above the catch zone
ELEVATION = (math.radians(18), math.radians(72))
SAMPLES = 20
CLEARANCE = 0.5
LANDING = 1.0


def solve(zone, target, gravity, flight=FLIGHT, barrel=BARREL, breech=BREECH_HEIGHT, where="cannon"):
    """Barrel and launch data for the catch zone ``zone`` (x, y, z floor) and the ball landing point ``target`` (Blender metres).

    Returns ``{muzzle, velocity, breech, heading, elevation}`` with Blender ``muzzle``/``breech`` points and the
    runtime ``velocity``; the ``muzzle`` of the manifest is ``runtime(muzzle)``.
    """
    heading = math.atan2(target[1] - zone[1], target[0] - zone[0])
    forward = (math.cos(heading), math.sin(heading))
    point = (zone[0] + BREECH_FORWARD * forward[0], zone[1] + BREECH_FORWARD * forward[1], zone[2] + breech)
    goal = runtime(target)
    lift = 0.5 * gravity * flight * flight
    alpha = math.radians(45)
    for _ in range(40):
        muzzle = (point[0] + barrel * math.cos(alpha) * forward[0], point[1] + barrel * math.cos(alpha) * forward[1], point[2] + barrel * math.sin(alpha))
        start = runtime(muzzle)
        velocity = [(goal[0] - start[0]) / flight, (goal[1] - start[1] + lift) / flight, (goal[2] - start[2]) / flight]
        updated = math.atan2(velocity[1], math.hypot(velocity[0], velocity[2]))
        if abs(updated - alpha) < 1e-10:
            alpha = updated
            break
        alpha = updated
    if not ELEVATION[0] <= alpha <= ELEVATION[1]:
        raise ValueError("%s: the barrel would point up at %.0f degrees (allowed %.0f-%.0f); move the target, change 'flight' "
                         "or 'barrel'" % (where, math.degrees(alpha), math.degrees(ELEVATION[0]), math.degrees(ELEVATION[1])))
    muzzle = (point[0] + barrel * math.cos(alpha) * forward[0], point[1] + barrel * math.cos(alpha) * forward[1], point[2] + barrel * math.sin(alpha))
    start = runtime(muzzle)
    velocity = [(goal[0] - start[0]) / flight, (goal[1] - start[1] + lift) / flight, (goal[2] - start[2]) / flight]
    return {"muzzle": muzzle, "velocity": rounded(velocity), "breech": point, "heading": heading, "elevation": alpha,
            "muzzle_rt": rounded(runtime(muzzle))}


def samples(muzzle_rt, velocity, gravity, flight, count=SAMPLES):
    """``count`` runtime points of the flight at ``flight * k / (count + 1)`` (the ends are the muzzle and the landing)."""
    out = []
    for k in range(1, count + 1):
        t = flight * k / (count + 1)
        out.append((muzzle_rt[0] + velocity[0] * t, muzzle_rt[1] + velocity[1] * t - 0.5 * gravity * t * t, muzzle_rt[2] + velocity[2] * t, t))
    return out


def _segment_distance(p, a, b):
    ab = [b[i] - a[i] for i in range(3)]
    length2 = sum(c * c for c in ab)
    t = 0.0 if length2 < 1e-12 else max(0.0, min(1.0, sum((p[i] - a[i]) * ab[i] for i in range(3)) / length2))
    return math.dist(p, [a[i] + t * ab[i] for i in range(3)])


def problems(sol, gravity, flight, target, floor, rail_tops, spheres, cylinders, limits):
    """Messages for every sample of the flight that is too close to the course or outside ``limits`` (runtime ``min``/``max``).

    ``floor`` is a ``FloorField``; ``rail_tops`` a list of Blender polylines (the top edge of every rail run);
    ``spheres`` are ``(name, x, y, z, radius)`` and ``cylinders`` ``(name, x, y, foot_z, radius, height)`` obstructions
    in Blender metres; ``target`` is the Blender landing point of the ball centre.
    """
    out = []
    for px, py, pz, t in samples(sol["muzzle_rt"], sol["velocity"], gravity, flight):
        b = blender((px, py, pz))
        where = "sample at %.2f s (%.1f, %.1f, %.1f m)" % (t, b[0], b[1], b[2])
        if any(not limits["min"][i] <= (px, py, pz)[i] <= limits["max"][i] for i in range(3)):
            out.append("%s leaves the course bounds" % where)
            continue
        landing = t > flight / 2 and math.hypot(b[0] - target[0], b[1] - target[1]) <= LANDING
        if not landing:
            for distance, z, _ in floor.field.nearest(b[0], b[1], CLEARANCE):
                if math.hypot(distance, b[2] - z) < CLEARANCE:
                    out.append("%s passes %.2f m above or beside the floor" % (where, math.hypot(distance, b[2] - z)))
                    break
            else:
                for line in rail_tops:
                    near = min((_segment_distance(b, a, c) for a, c in zip(line, line[1:])), default=9.0)
                    if near < CLEARANCE:
                        out.append("%s comes within %.2f m of a rail" % (where, near))
                        break
        for name, sx, sy, sz, radius in spheres:
            if math.dist(b, (sx, sy, sz)) < radius + CLEARANCE:
                out.append("%s comes within %.2f m of %s" % (where, math.dist(b, (sx, sy, sz)) - radius, name))
        for name, cx, cy, foot, radius, height in cylinders:
            if math.hypot(b[0] - cx, b[1] - cy) < radius + CLEARANCE and foot - CLEARANCE < b[2] < foot + height + CLEARANCE:
                out.append("%s passes through the space of %s" % (where, name))
    return out
