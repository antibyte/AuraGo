"""Per-course level-of-detail overrides (MIT License, see ../LICENSE).

Plain Python, no ``bpy``. A course may carry ``"detail": {...}`` in its data to trade mesh detail for size when it
holds heavy pieces (loop, half-pipe, fork, funnel together) and would otherwise exceed the GLB budget
(1.25 MiB, 36,000 triangles). Every key is optional:

* ``island_quads``  upper limit of the island top grid in quads (default ``DEFAULTS``, range ``LIMITS``); a smaller
                    limit makes the cells coarser, saving two triangles per quad;
* ``loop_sections`` cross-sections of every ``loop`` without its own ``sections`` (default: the piece default 72,
                    range 48-120); each section costs about 40 collision triangles plus floor, trim and rail mesh;
* ``trim_facets``   facets of the rounded rail-trim top (default 4, range 1-6); 2 saves about 30 % of the trim,
                    1 about 40 %.
"""

DEFAULTS = {"island_quads": 4400, "loop_sections": None, "trim_facets": 4}
LIMITS = {"island_quads": (1200, 8000), "loop_sections": (48, 120), "trim_facets": (1, 6)}


def resolve(data):
    """The detail settings of a course: ``DEFAULTS`` updated by ``data["detail"]``; ``ValueError`` for unknown keys or values."""
    wanted = dict(data.get("detail") or {})
    unknown = sorted(set(wanted) - set(DEFAULTS))
    if unknown:
        raise ValueError("unknown detail key(s): %s (known: %s)" % (", ".join(unknown), ", ".join(sorted(DEFAULTS))))
    out = dict(DEFAULTS)
    for key, value in wanted.items():
        low, high = LIMITS[key]
        if isinstance(value, bool) or not isinstance(value, int) or not low <= value <= high:
            raise ValueError("detail %s must be a whole number from %d to %d (got %r)" % (key, low, high, value))
        out[key] = value
    return out
