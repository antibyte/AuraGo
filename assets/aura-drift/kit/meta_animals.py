"""Manifest metadata for animals.glb (MIT, AuraGo contributors).

``length`` and ``height`` are measured from the GLB (nose to tail, ground to the highest point). Extra keys:
``legs`` names what the four leg nodes are, ``stride`` is the leg swing in radians the runtime should
use (flippers swing wider than legs), ``bob`` the head bob in metres.
"""

ANIMALS = {
    "cow": {"legs": "legs", "stride": 0.5, "bob": 0.05},
    "sheep": {"legs": "legs", "stride": 0.6, "bob": 0.06},
    "penguin": {"legs": "flippers+feet", "stride": 0.7, "bob": 0.04},
    "crab": {"legs": "merged", "stride": 0.35, "bob": 0.03},
    "llama": {"legs": "legs", "stride": 0.45, "bob": 0.08},
}
