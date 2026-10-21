"""Size of the random dressing for the island of a course (MIT License, see ../LICENSE).

``placement.PLANS`` list how many medium props and how much scatter an island of about ``REFERENCE_AREA`` m^2 gets
(the half axes 9.6 x 9.4 m of a small P3 course). Bigger dioramas would look empty with the same counts, so
``scaled`` multiplies every count with ``area / REFERENCE_AREA`` (never below 1; a single prop stays single until the
factor reaches 1.5). The area is that of the island's superellipse (``AREA_SHARE`` of its bounding box).
"""

REFERENCE_AREA = 320.0
AREA_SHARE = 0.88


def factor(half):
    """Multiplier of the counts for an island with half axes ``half`` (1.0 for a small one)."""
    return max(1.0, 4.0 * half[0] * half[1] * AREA_SHARE / REFERENCE_AREA)


def scaled(plan, half):
    """Copy of ``plan`` (``{"hero", "medium": [(name, count)], "scatter": [(name, count)]}``) with the counts scaled."""
    f = factor(half)
    scale = lambda entries: [(name, max(1, int(count * f + 0.5))) for name, count in entries]
    return dict(plan, medium=scale(plan["medium"]), scatter=scale(plan["scatter"]))
