"""Aura Drift animals: toy cow, cloud sheep, penguin, crab, haughty llama (MIT, AuraGo contributors).

Entry point of ``animals.glb``. Each ``animal-<kind>`` is an empty with the children ``body``, ``head``,
``tail``, ``leg_fl``, ``leg_fr``, ``leg_rl``, ``leg_rr``; every child's origin is its joint, so the
runtime swings legs and bobs the head with plain rotations. Facing +X, right side +Z (the pieces are
authored in the character frame, see ``toy.py``). Penguin: the flippers are ``leg_fl/fr`` and the
feet ``leg_rl/rr``. Crab: six legs merged into the four leg nodes, the eyes and claws are the ``head``.
"""

import math

from .characters import INK, eye_pair
from .core import Model, rng_for
from .shapes import arc_band, box, cylinder, ellipsoid, sphere, torus
from .toy import Parts, ensure_materials, ground_ao, install_name_hook, lp, tube

AO = ground_ao(0.9, 0.7)


class Rig:
    """One animal: an empty root plus jointed child meshes."""

    def __init__(self, model, kind):
        self.model, self.kind = model, kind
        self.root = model.empty("animal-" + kind)
        self.parts = {}
        self.shades = {}

    def part(self, name, pivot, shade="auto"):
        self.parts[name] = Parts(pivot)
        self.shades[name] = shade
        return self.parts[name]

    def finish(self):
        for name in ("body", "head", "tail", "leg_fl", "leg_fr", "leg_rl", "leg_rr"):
            part = self.parts.get(name)
            if part is None:
                raise SystemExit("animal %s lacks %s" % (self.kind, name))
            base = lp(part.pivot)[1]
            part.bake(lambda x, y, z, base=base: AO(x, y + base, z))
            self.model.mesh("%s|%s" % (self.kind, name), part.items(), shade=self.shades[name], parent=self.root, location=lp(part.pivot))


def legs(rig, hip_y, xs, zs, r0, r1, fur, hoof, hoof_h=0.1, names=("leg_fl", "leg_fr", "leg_rl", "leg_rr"), sides=7):
    """Four straight legs: front pair at zs[0], rear pair at zs[1]; x of the right leg is xs."""
    for (side, front), name in zip(((-1, True), (1, True), (-1, False), (1, False)), names):
        z = zs[0] if front else zs[1]
        x = side * xs
        leg = rig.part(name, (x, hip_y, z))
        leg.place("toy", cylinder(r0, r1, hip_y - hoof_h, sides), (x, (hip_y + hoof_h) / 2.0, z), tint=fur)
        leg.place("toy", cylinder(r1 * 1.18, r1 * 1.12, hoof_h * 1.6, sides), (x, hoof_h * 0.8, z), tint=hoof)


def cow(model):
    rig = Rig(model, "cow")
    W, K, PINK, HORN = "#fffaf2", "#2a2530", "#ff9eb0", "#fff0c9"
    b = rig.part("body", (0, 0.9, 0))
    b.place("toy", ellipsoid(0.55, 0.48, 0.92, 10, 5), (0, 0.98, -0.12), tint=W)
    for pos, scale, rot in (
        ((0.0, 1.41, -0.3), (0.34, 0.1, 0.4), (0, 0, 0)),
        ((0.5, 1.08, 0.2), (0.1, 0.3, 0.3), (0, 0, 0)),
        ((-0.5, 1.15, -0.45), (0.1, 0.32, 0.28), (0, 0, 0)),
        ((0.46, 1.2, -0.6), (0.12, 0.2, 0.2), (0, 0, 0)),
        ((-0.42, 1.36, 0.28), (0.18, 0.1, 0.26), (0, 0, -20)),
    ):
        b.place("toy", sphere(1.0, 7, 3), pos, rot=rot, scale=scale, tint=K)
    b.place("toy", ellipsoid(0.22, 0.16, 0.24, 8, 4), (0, 0.52, -0.62), tint=PINK)
    for x in (0.08, -0.08):
        b.place("toy", cylinder(0.035, 0.03, 0.14, 5), (x, 0.42, -0.62), tint="#ff7f98")
    b.place("toy", ellipsoid(0.34, 0.3, 0.3, 8, 4), (0, 1.0, 0.78), tint=W)  # shoulders into the neck
    h = rig.part("head", (0, 1.05, 0.86))
    h.place("toy", ellipsoid(0.44, 0.4, 0.42, 10, 5), (0, 1.2, 1.15), tint=W)
    h.place("toy", ellipsoid(0.3, 0.22, 0.22, 10, 5), (0, 1.02, 1.5), tint=PINK)
    h.pair("toy", sphere(0.052, 6, 3), (0.1, 1.04, 1.68), tint=K)
    eye_pair(h, 0.22, 1.36, 1.44, 0.15, look=(0.25, -0.1), lid=None, pupil=0.56, detail=0.5)
    h.pair("toy", cylinder(0.075, 0.015, 0.26, 6), (0.24, 1.6, 1.06), rot=(0, 0, -25), tint=HORN)
    h.pair("toy", ellipsoid(0.2, 0.09, 0.13, 8, 4), (0.5, 1.45, 1.05), rot=(0, 0, -25), tint=W)
    h.pair("toy", ellipsoid(0.13, 0.05, 0.08, 6, 3), (0.48, 1.45, 1.1), rot=(0, 0, -25), tint=PINK)
    h.place("toy", ellipsoid(0.15, 0.09, 0.14, 6, 3), (0, 1.58, 1.18), tint=K)
    h.place("toy", torus(0.32, 0.04, 7, 3, 1.06), (0, 0, 0.94), rot=(58, 0, 0), tint="#8a3b1e")
    h.place("toy", sphere(0.1, 7, 4), (0, 0.78, 1.18), tint="#ffcc33")
    h.place("toy", cylinder(0.09, 0.12, 0.14, 6), (0, 0.74, 1.18), tint="#ffd94a")
    t = rig.part("tail", (0, 1.28, -1.02))
    t.place("toy", tube([(0, 1.28, -1.02), (0, 1.0, -1.18), (0, 0.72, -1.2)], 0.04, 5), tint=W)
    t.place("toy", ellipsoid(0.1, 0.17, 0.1, 7, 4), (0, 0.62, -1.2), tint=K)
    legs(rig, 0.62, 0.34, (0.55, -0.7), 0.13, 0.11, W, K, 0.14)
    rig.finish()


def sheep(model):
    rig = Rig(model, "sheep")
    rng = rng_for("sheep")
    wool = ("#ffffff", "#f6f1e8", "#fbf7ef")
    dark = "#4b3c3f"
    b = rig.part("body", (0, 0.85, 0), "smooth")
    b.place("fuzz", ellipsoid(0.48, 0.42, 0.7, 8, 4), (0, 0.86, -0.05), tint=wool[0])
    for z in (-0.55, -0.1, 0.34):
        for k in range(6):
            a = math.tau * k / 6 + (0.5 if z > -0.2 else 0.0)
            r = 0.4 + 0.05 * rng.random()
            b.place("fuzz", sphere(r, 7, 4), (0.5 * math.cos(a), 0.86 + 0.46 * math.sin(a), z), tint=wool[rng.randrange(3)])
    for z in (-0.5, 0.05, 0.5):
        b.place("fuzz", sphere(0.34, 7, 4), (0, 1.3, z), tint=wool[0])
    h = rig.part("head", (0, 1.0, 0.7))
    h.place("fuzz", sphere(0.27, 8, 4), (0, 1.38, 0.78), scale=(1.0, 0.8, 1.0), tint=wool[0])  # forelock fluff
    h.place("toy", ellipsoid(0.27, 0.3, 0.34, 10, 5), (0, 1.04, 1.0), tint=dark)
    h.place("toy", ellipsoid(0.17, 0.14, 0.17, 8, 4), (0, 0.93, 1.3), tint="#6d5658")
    h.pair("toy", sphere(0.03, 5, 3), (0.07, 0.95, 1.46), tint=INK)
    eye_pair(h, 0.17, 1.12, 1.2, 0.12, look=(-0.2, 0.0), pupil=0.5, shine=True, detail=0.5)
    h.pair("toy", ellipsoid(0.24, 0.08, 0.12, 6, 3), (0.38, 1.1, 0.98), rot=(0, 0, -18), tint=dark)
    t = rig.part("tail", (0, 1.0, -0.78))
    t.place("fuzz", sphere(0.2, 7, 4), (0, 1.0, -0.88), tint=wool[1])
    legs(rig, 0.5, 0.26, (0.42, -0.45), 0.075, 0.065, dark, "#2a2227", 0.1, sides=6)
    rig.finish()


def penguin(model):
    rig = Rig(model, "penguin")
    D, W, O, RED = "#1f2a48", "#fffaf2", "#ff9a1f", "#e8262d"
    b = rig.part("body", (0, 0.7, 0))
    b.place("toy", ellipsoid(0.52, 0.72, 0.48, 10, 5), (0, 0.84, 0), tint=D)
    b.place("toy", ellipsoid(0.42, 0.62, 0.34, 10, 5), (0, 0.8, 0.22), tint=W)
    b.place("toy", torus(0.34, 0.07, 10, 3, 1.42), (0, 0, 0.02), rot=(-8, 0, 0), tint=RED)
    b.place("toy", box(0.12, 0.34, 0.1), (0.2, 1.3, 0.28), rot=(-20, 0, -8), tint=RED)
    h = rig.part("head", (0, 1.48, 0.04))
    h.place("toy", sphere(0.4, 10, 5), (0, 1.78, 0.06), tint=D)
    h.place("toy", ellipsoid(0.3, 0.26, 0.2, 8, 4), (0, 1.7, 0.3), tint=W)
    h.place("toy", cylinder(0.12, 0.02, 0.32, 6), (0, 1.7, 0.58), rot=(90, 0, 0), scale=(1.0, 1.0, 0.6), tint=O)
    eye_pair(h, 0.17, 1.86, 0.34, 0.14, look=(0.15, -0.1), pupil=0.55, detail=0.5)
    h.pair("toy", cylinder(0.12, 0.12, 0.09, 6), (0.16, 2.05, 0.38), rot=(78, 0, 10), tint="#5ff0ff")
    for name, side in (("leg_fl", -1), ("leg_fr", 1)):
        f = rig.part(name, (side * 0.5, 1.2, 0.04))
        f.place("toy", ellipsoid(0.07, 0.4, 0.2, 8, 4), (side * 0.56, 0.9, 0.04), rot=(0, 0, side * 14), tint=D)
    for name, side in (("leg_rl", -1), ("leg_rr", 1)):
        f = rig.part(name, (side * 0.22, 0.16, 0.1))
        f.place("toy", ellipsoid(0.17, 0.07, 0.3, 8, 4), (side * 0.22, 0.07, 0.2), tint=O)
    t = rig.part("tail", (0, 0.4, -0.4))
    t.place("toy", cylinder(0.02, 0.2, 0.34, 4), (0, 0.32, -0.55), rot=(-70, 0, 0), tint=D)
    rig.finish()


def crab(model):
    rig = Rig(model, "crab")
    S, L, P, E = "#ff5a3c", "#ff8a4a", "#fff0d6", "#c93a22"
    b = rig.part("body", (0, 0.5, 0))
    b.place("toy", ellipsoid(0.9, 0.34, 0.62, 12, 5), (0, 0.55, 0), tint=S)
    for x, z, r in ((-0.35, 0.05, 0.09), (0.3, -0.1, 0.08), (0.0, 0.25, 0.07), (-0.1, -0.3, 0.1), (0.5, 0.2, 0.06)):
        b.place("toy", sphere(r, 6, 3), (x, 0.84, z), tint=P)
    b.place("matte", arc_band(0.18, 0.24, 205, 335, 0.0, 0.06, 6), (0, 0.62, 0.58), rot=(-12, 0, 0), tint="#6a1f14")
    h = rig.part("head", (0, 0.8, 0.45))
    for side in (-1, 1):
        x = side * 0.3
        h.place("toy", cylinder(0.045, 0.045, 0.36, 5), (x, 0.92, 0.48), tint=L)
        h.place("toy", sphere(0.15, 8, 4), (x, 1.15, 0.5), tint="#ffffff")
        h.place("toy", sphere(0.085, 6, 3), (x + side * 0.0, 1.17, 0.62), tint=INK)
        arm = [(side * 0.8, 0.62, 0.3), (side * 1.05, 0.78, 0.52), (side * 1.12, 0.9, 0.78)]
        h.place("toy", tube(arm, 0.075, 5), tint=L)
        h.place("toy", ellipsoid(0.3, 0.24, 0.3, 8, 4), (side * 1.15, 1.0, 0.95), tint=S)
        for dx in (-0.11, 0.11):
            h.place("toy", cylinder(0.12, 0.015, 0.45, 5), (side * 1.15 + dx, 1.0, 1.28), rot=(90, 0, dx * 90), tint=S)
    t = rig.part("tail", (0, 0.5, -0.5))
    t.place("toy", ellipsoid(0.2, 0.1, 0.14, 6, 3), (0, 0.4, -0.58), tint=L)
    for name, side, zs in (("leg_fl", -1, (0.34, 0.0)), ("leg_fr", 1, (0.34, 0.0)), ("leg_rl", -1, (-0.3,)), ("leg_rr", 1, (-0.3,))):
        pivot_z = zs[0] - 0.1
        leg = rig.part(name, (side * 0.7, 0.5, pivot_z))
        for z in zs:
            path = [(side * 0.7, 0.5, z), (side * 1.05, 0.6, z + 0.06), (side * 1.22, 0.06, z + 0.12)]
            leg.place("toy", tube(path, [0.085, 0.07, 0.05], 5), tint=L)
            leg.place("toy", sphere(0.085, 5, 3), (side * 1.22, 0.07, z + 0.12), tint=E)
    rig.finish()


def llama(model):
    rig = Rig(model, "llama")
    C, F, BLANKET = "#f1e0c3", "#fff6e4", ("#ff3f8e", "#ffd21f", "#2fb7e8")
    b = rig.part("body", (0, 1.2, 0))
    b.place("fuzz", ellipsoid(0.46, 0.5, 0.82, 10, 5), (0, 1.3, -0.15), tint=C)
    b.place("fuzz", tube([(0, 1.5, 0.45), (0, 1.95, 0.62), (0, 2.3, 0.72)], [0.26, 0.2, 0.17], 8), tint=C)
    for z in (-0.2, 0.0, 0.2):
        b.place("fuzz", sphere(0.17, 6, 3), (0, 1.78 + 0.0 * z, z - 0.2), tint=F)
    b.place("toy", box(0.96, 0.07, 0.6), (0, 1.8, -0.2), tint=BLANKET[0])
    for k, tone in enumerate(BLANKET[1:]):
        b.place("toy", box(0.98, 0.075, 0.08), (0, 1.805, -0.2 + 0.18 * (k * 2 - 1) * 0.8), tint=tone)
    for side in (1, -1):
        b.place("toy", box(0.07, 0.5, 0.6), (side * 0.5, 1.52, -0.2), rot=(0, 0, -side * 8), tint=BLANKET[0])
        b.place("toy", box(0.075, 0.07, 0.62), (side * 0.52, 1.28, -0.2), tint=BLANKET[1])
    h = rig.part("head", (0, 2.25, 0.72))
    h.place("fuzz", ellipsoid(0.22, 0.26, 0.3, 10, 5), (0, 2.55, 0.86), rot=(-14, 0, 0), tint=C)
    h.place("fuzz", ellipsoid(0.16, 0.14, 0.2, 8, 4), (0, 2.46, 1.14), rot=(-14, 0, 0), tint=F)
    h.pair("toy", sphere(0.03, 5, 3), (0.06, 2.5, 1.32), tint="#5a3c3c")
    h.place("matte", arc_band(0.05, 0.075, 205, 335, 0.0, 0.03, 4), (0, 2.38, 1.28), tint="#5a3c3c")
    eye_pair(h, 0.15, 2.64, 1.06, 0.1, look=(0.0, -0.25), lid=C, pupil=0.5, shine=False, detail=0.5)
    h.pair("fuzz", ellipsoid(0.065, 0.34, 0.08, 6, 3), (0.14, 2.98, 0.86), rot=(0, 0, -12), tint=C)
    h.pair("fuzz", sphere(0.065, 6, 3), (0.21, 3.24, 0.86), tint=BLANKET[0])
    h.pair("toy", box(0.14, 0.035, 0.07), (0.15, 2.76, 1.1), rot=(0, 0, -14), tint="#8a6a4a")  # arched brow
    h.place("fuzz", sphere(0.2, 7, 4), (0, 2.9, 0.8), scale=(1.0, 0.8, 1.0), tint=F)
    t = rig.part("tail", (0, 1.6, -0.92))
    t.place("fuzz", sphere(0.17, 7, 4), (0, 1.55, -1.05), tint=F)
    legs(rig, 0.9, 0.27, (0.52, -0.6), 0.1, 0.075, C, "#6b5040", 0.12)
    for name in ("leg_fl", "leg_fr", "leg_rl", "leg_rr"):
        part = rig.parts[name]
        px, _, pz = part.pivot
        part.place("fuzz", sphere(0.15, 6, 3), (px, 0.85, pz), tint=F)
    rig.finish()


def build():
    ensure_materials()
    install_name_hook()
    model = Model("animals")
    for make in (cow, sheep, penguin, crab, llama):
        make(model)
    return model
