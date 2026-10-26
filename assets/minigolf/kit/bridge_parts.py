"""Bridge parts: railing and the beam structure under the deck (MIT License, see ../LICENSE).

A strip with ``deck: "bridge"`` (``layout`` gives it the floor look ``deck`` on the pirate and temple worlds and
``metal`` on the moon unless the piece sets its own) gets

* railing posts along each side that has a rail wall, standing outside the rail trim (the lane edge, the rail and
  the trim reach 1.03 m from the centre line, the posts stand at 1.08 m), joined by a hand rail: ropes on the pirate
  bay, a stone curb with moss in the temple, a neon tube on the moon;
* stringers under both lane edges and cross joists about every 0.55 m directly under the floor (the floor has no
  underside of its own, so the joists give the deck its thickness). The temple bridge is a stone slab instead.

Everything stands outside the ball clearance (above the floor only the posts and rails outside the rails, below it the
beams). The support posts that carry the bridge are planned by ``support_plan``.
"""

import math

from . import part_utils as pu
from . import shapes
from .prop import Prop
from .track import ROWS

POST_OUT = 1.08             # centre of a railing post from the lane centre line
SAMPLE = 0.9                # route metres between railing posts
JOIST = 0.55
RAIL_HEIGHT = {"pirate": 0.62, "temple": 0.5, "moon": 0.46}      # post top above the floor edge
PLAIN = {"bevel": 0, "dens": 0, "weighted": False, "smooth_angle": 50.0}


def bridge_strips(track):
    return [strip for strip in track.strips if strip.deck == "bridge"]


def _samples(strip, spacing):
    """Section indices at about ``spacing`` metres along the strip, always including both ends."""
    picks, travelled = [0], 0.0
    for i in range(1, len(strip.sections)):
        travelled += pu.length(pu.sub(strip.sections[i].center, strip.sections[i - 1].center))
        if travelled >= spacing or i == len(strip.sections) - 1:
            if i != picks[-1]:
                picks.append(i)
            travelled = 0.0
    return picks


def _flat(across):
    return pu.unit((across[0], across[1], 0.0))


def _railing(p, look, strip, side):
    """Posts and hand rail on ``side`` (0 left, 1 right) of a bridge strip."""
    sign = 1.0 if side == 0 else -1.0
    picks = [i for i in _samples(strip, SAMPLE) if strip.sections[i].walls[side]]
    if len(picks) < 2:
        return
    tops = []
    for i in picks:
        section = strip.sections[i]
        edge = section.points[0 if side == 0 else ROWS]
        flat = _flat(pu.sub(section.points[0], section.points[ROWS]))
        reach = POST_OUT - 0.86
        base = pu.at(edge, (flat, sign * reach), ((0.0, 0.0, 1.0), -0.05))
        height = RAIL_HEIGHT[look] + 0.05
        tops.append((base[0], base[1], base[2] + height))
        _post(p, look, base, height)
    tube_pts = [(x, y, z - 0.02) for x, y, z in tops]
    if look == "pirate":
        rope = shapes.tube("rope", tube_pts, radius=0.022, seg=6)
        p.add(rope, "matte", "rope", ao=0.6, **PLAIN)
        low = [(x, y, z - 0.24) for x, y, z in tops]
        p.add(shapes.tube("rope-low", low, radius=0.018, seg=6), "matte", "rope", ao=0.6, **PLAIN)
    elif look == "temple":
        curb = shapes.tube("curb", [(x, y, z - 0.03) for x, y, z in tops], radius=0.075, seg=6, squash=0.8)
        p.add(curb, "matte", ("stone", "stone_light"), ao=0.65, **PLAIN)
        moss = shapes.tube("moss", [(x, y, z + 0.02) for x, y, z in tops], radius=0.05, seg=5, squash=0.5)
        p.add(moss, "matte", ("moss_dark", "moss_light"), ao=0.5, noise_amp=0.1, **PLAIN)
    else:
        tone = "cyan" if side == 0 else "magenta"
        neon = shapes.tube("neon", [(x, y, z - 0.01) for x, y, z in tops], radius=0.016, seg=5)
        p.add(neon, "emissive", (tone, tone), finish=False, ao=0.0, strength=1.6, jitter=0.0)


def _post(p, look, base, height):
    if look == "pirate":
        post = shapes.cylinder("rail-post", 0.04, 0.035, height, seg=6)
        p.add(post, "matte", "wood", loc=base, ao=0.65, **PLAIN)
    elif look == "temple":
        post = shapes.cylinder("rail-post", 0.065, 0.055, height - 0.03, seg=6)
        p.add(post, "matte", ("stone_dark", "stone_light"), loc=base, ao=0.65, **PLAIN)
    else:
        post = shapes.cylinder("rail-post", 0.035, 0.03, height, seg=6)
        p.add(post, "satin", "orange", loc=base, ao=0.65, **PLAIN)


def _underside(p, look, strip):
    """Stringers under the lane edges and joists (a stone slab in the temple)."""
    picks = _samples(strip, JOIST)
    rows = []
    for i in picks:
        s = strip.sections[i]
        left, right = s.points[0], s.points[ROWS]
        up = pu.unit(s.normal)
        rows.append((left, right, up, s))
    if look == "temple":
        _slab(p, rows)
        return
    depth = 0.08 if look == "pirate" else 0.07
    mat, grad = ("matte", "wood") if look == "pirate" else ("satin", "metal")
    for left, right, up, _ in rows:
        a, b = pu.at(left, (up, -(depth / 2 + 0.012))), pu.at(right, (up, -(depth / 2 + 0.012)))
        axis = pu.unit(pu.sub(b, a))
        a, b = pu.at(a, (axis, -0.02)), pu.at(b, (axis, 0.02))
        p.add(pu.beam_between("joist", a, b, 0.11, depth), mat, grad, ao=0.7, **PLAIN)
    sdepth = 0.14 if look == "pirate" else 0.12
    for k in (0, 1):
        for (l0, r0, u0, _), (l1, r1, u1, _) in zip(rows, rows[1:]):
            e0, e1 = (l0, l1) if k == 0 else (r0, r1)
            sign = 1.0 if k == 0 else -1.0
            across0 = pu.unit(pu.sub(l0, r0))
            a = pu.at(e0, (u0, -(sdepth / 2 + 0.012)), (across0, -sign * 0.06))
            b = pu.at(e1, (u1, -(sdepth / 2 + 0.012)), (across0, -sign * 0.06))
            p.add(pu.beam_between("stringer", a, b, 0.12, sdepth), "matte" if look == "pirate" else "satin", "wood_dark" if look == "pirate" else "metal",
                  ao=0.7, **PLAIN)


def _slab(p, rows):
    """Stone slab 0.2 m thick under the deck: a closed loft of rectangular rings."""
    rings = []
    for left, right, up, _ in rows:
        across = pu.unit(pu.sub(left, right))
        l, r = pu.at(left, (across, 0.04)), pu.at(right, (across, -0.04))
        rings.append([pu.at(l, (up, -0.012)), pu.at(r, (up, -0.012)), pu.at(r, (up, -0.2)), pu.at(l, (up, -0.2))])
    slab = shapes.loft("slab", rings, closed=True, cap_start=True, cap_end=True, recalc=True)
    p.add(slab, "matte", ("stone_dark", "stone"), ao=0.7, noise_amp=0.1, **PLAIN)


def build(look, track, number):
    """One joined mesh ``bridge-parts`` of all bridge strips (None without bridges)."""
    strips = bridge_strips(track)
    if not strips:
        return None
    p = Prop(look, "bridge-parts", seed=number, full_name="bridge-parts")
    for strip in strips:
        for side in (0, 1):
            _railing(p, look, strip, side)
        _underside(p, look, strip)
    return p.build()
