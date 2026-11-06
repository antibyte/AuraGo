"""Manifest metadata for track.glb (MIT, AuraGo contributors). Values extend the measured entry.

The runtime scales ramps, pads and rumble bars to the track: ``length`` runs along +X, ``width`` across
the track (Z), ``height`` is the rise (ramp) or the top of the piece above y = 0.
"""

TRACK = {
    "ramp": {"length": 9.0, "width": 9.0, "height": 1.6},
    "boost_pad": {"length": 10.0, "width": 6.0, "height": 0.19},
    "cone": {"height": 0.82},
    "hay_bale": {"height": 1.9},
    "barrel": {"height": 1.37},
    "tyre_stack": {"height": 1.5},
    "barrier_segment": {"length": 4.0, "height": 0.98},
    "gantry": {"width": 14.6, "height": 7.25},
    "rumble_bars": {"length": 8.3, "width": 12.0, "height": 0.1},
}
