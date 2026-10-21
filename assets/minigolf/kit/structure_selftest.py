"""Headless self test of the course structures (MIT License, see ../LICENSE).

blender --background --factory-startup --python assets/minigolf/kit/structure_selftest.py

Builds track layouts (no Blender objects) and checks the rules of ``support_plan``, ``tunnel_plan``, ``layout`` and
``detail``: the bridge floor look per world, the rejected tunnel covers, the support rules (posts only where the floor
is higher than ``MIN_DROP`` above the ground, none through another stretch or its rails), the tunnel opening and the
straight-run and roof-crossing errors, and the ``detail`` validation. Prints ``structure selftest ok``.
"""

import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.dirname(HERE))

from kit import detail, layout, rails, support_plan, tunnel_plan  # noqa: E402


def build(data, world):
    """The track of ``data`` (the route length range of real courses does not apply to these snippets)."""
    return layout.build(data, world, check_length=False)


def course(world, pieces, number=90):
    return {"number": number, "par": 3, "start": {"at": (0.0, 0.0, 0.0), "heading": 0}, "pieces": pieces}


def tee_green(*middle):
    return [("tee", {"length": 1.0})] + list(middle) + [("green", {"length": 3.0, "width": 3.44, "cup": (1.7, 0.0)})]


def expect_error(text, build):
    try:
        build()
    except ValueError as exc:
        assert text in str(exc), "expected %r in %r" % (text, str(exc))
        return
    raise AssertionError("no ValueError containing %r" % text)


def check_bridge_look():
    for world, look in (("pirate", "deck"), ("temple", "deck"), ("moon", "metal")):
        track = build(course(world, tee_green(("straight", {"length": 3.0, "deck": "bridge"}), ("straight", {"length": 2.0}))), world)
        bridge = next(strip for strip in track.strips if strip.deck == "bridge")
        assert all(s.look == look for s in bridge.sections[1:]), (world, look)
        assert next(strip for strip in track.strips if strip.deck is None).sections[-1].look == "felt"
    track = build(course("pirate", tee_green(("straight", {"length": 3.0, "deck": "bridge", "look": "slab"}))), "pirate")
    assert track.strips[0].sections[-1].look == "slab", "an explicit look wins over the bridge default"


def check_covers():
    expect_error("tunnel roof", lambda: build(course("moon", tee_green(("straight", {"length": 3.0}), ("halfpipe", {"length": 4.0, "cover": "tunnel"}))), "moon"))
    expect_error("ramp with a lip", lambda: build(course("pirate", tee_green(("ramp", {"length": 2.0, "rise": 0.3, "lip": 20, "cover": "tunnel"}))), "pirate"))


def flat_ground(x, y):
    return -0.04


def check_supports():
    # a ramp up, a bridge and a ramp down over flat ground: posts only where the floor is clearly above the ground
    pieces = tee_green(("straight", {"length": 1.0}), ("ramp", {"length": 3.4, "rise": 0.8}), ("straight", {"length": 3.0, "deck": "bridge"}),
                       ("ramp", {"length": 3.4, "rise": -0.8}), ("straight", {"length": 1.0}))
    track = build(course("pirate", pieces), "pirate")
    stations = support_plan.plan(track, flat_ground, rails.rail_runs(track))
    assert stations, "a bridge on 0.8 m needs supports"
    for station in stations:
        for side in ("left", "right"):
            post = station[side]
            if post:
                assert post["top"] - (post["bottom"] + support_plan.SINK) >= support_plan.MIN_DROP - 1e-9, "a post under a floor that is too low"
    assert not any(station["center"][2] < 0.1 for station in stations), "supports under a lane that rests on the ground"
    low = build(course("pirate", tee_green(("straight", {"length": 3.0}))), "pirate")
    assert not support_plan.plan(low, flat_ground, rails.rail_runs(low)), "a lane on the ground has no supports"


def check_tunnel():
    assert tunnel_plan.HALF * 2 >= tunnel_plan.OPEN_MIN[0] and tunnel_plan.CROWN >= tunnel_plan.OPEN_MIN[1]
    for world in ("pirate", "temple", "moon"):
        need = tunnel_plan.PORTAL[world][1] + 0.3
        ok = build(course(world, tee_green(("straight", {"length": 2.0}), ("straight", {"length": need + 0.5, "cover": "tunnel"}), ("straight", {"length": 1.0}))), world)
        runs = tunnel_plan.plan(ok)
        assert len(runs) == 1 and abs(runs[0]["length"] - (need + 0.5)) < 1e-6, world
    short = build(course("pirate", tee_green(("straight", {"length": 2.0}), ("straight", {"length": 1.0, "cover": "tunnel"}))), "pirate")
    expect_error("needs at least", lambda: tunnel_plan.plan(short))
    bent = build(course("pirate", tee_green(("curve", {"radius": 2.5, "angle": 90, "cover": "tunnel"}), ("straight", {"length": 3.0, "cover": "tunnel"}))), "pirate")
    expect_error("straight and level", lambda: tunnel_plan.plan(bent))


def check_detail():
    assert detail.resolve({})["island_quads"] == detail.DEFAULTS["island_quads"]
    assert detail.resolve({"detail": {"trim_facets": 2}})["trim_facets"] == 2
    expect_error("unknown detail key", lambda: detail.resolve({"detail": {"quads": 3}}))
    expect_error("whole number", lambda: detail.resolve({"detail": {"loop_sections": 12}}))
    track = build(course("moon", [("tee", {"length": 1.0}), ("straight", {"length": 8.0}), ("loop", {"radius": 1.3, "offset": 2.2}),
                                         ("straight", {"length": 2.0}), ("green", {"length": 3.0, "width": 3.44, "cup": (1.7, 0.0)})]) | {"detail": {"loop_sections": 52}}, "moon")
    loop = next(strip for strip in track.strips if strip.kind == "loop")
    assert len(loop.sections) == 53, len(loop.sections)


def main():
    check_bridge_look()
    check_covers()
    check_supports()
    check_tunnel()
    check_detail()
    print("structure selftest ok")


if __name__ == "__main__":
    main()
