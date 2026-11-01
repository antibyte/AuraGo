"""Resident robots of System World 2: detailed geometry, rig and gait, authored in Blender.

The courier, technician and archivist share one articulated skeleton (a real parent
hierarchy that glTF exports as nested nodes):

    structure (pelvis)
      torso (waist) -> head (neck) -> halo (archivist)
                       arm_l/arm_r (shoulders) -> forearm_l/forearm_r (elbows, with hand)
                       parcel (courier)         slate on forearm_l (archivist)
      leg_l/leg_r (hips) -> shin_l/shin_r (knees) -> foot_l/foot_r (ankles)

Coordinates are Blender Z-up, metres, front = -Y (exports to glTF +Z). Every part is
authored in rest-pose world space and moved to its own pivot when the object is made.
Legs and arms that hold something are solved with analytic two-bone IK per frame, so the
planted foot moves backwards at exactly the speed the runtime walks.
"""
from math import sin, cos, pi, atan2, acos, sqrt, radians, copysign
import bpy
from mathutils import Matrix, Vector
from build_city import Geometry, box_template

KINDS = ('courier', 'technician', 'archivist')
ACCENT = {'courier': 'bronze', 'technician': 'jade', 'archivist': 'cyan'}
EYE = {'courier': 'ivory', 'technician': 'jade', 'archivist': 'cyan'}
FOREARM = {'courier': 'bronze', 'technician': 'bronze', 'archivist': 'cyan'}
ROBOT_PALETTE = {'robot-rubber': ((.016, .018, .022, 1), .04, .86, 0)}
DEBUG_NORMALS = False

# --- skeleton (rest pose, Blender coordinates) ---------------------------------------
HX, HIP_Z, KNEE_Z, ANK_Z = .165, 1.07, .60, .13
L_THIGH, L_SHIN = HIP_Z-KNEE_Z, KNEE_Z-ANK_Z
WAIST_Z, NECK_Z = 1.18, 1.75
SHX, SHZ, ELB_Z, WRI_Z = .385, 1.60, 1.28, .98
L_UPPER, L_FORE = SHZ-ELB_Z, ELB_Z-WRI_Z
PARCEL_C = (0, -.375, 1.37)
HALO_C = (0, 0, 2.25)
SIDES = (('l', -1), ('r', 1))

RIG = {'structure': (None, (0, 0, HIP_Z)), 'torso': ('structure', (0, 0, WAIST_Z)),
       'head': ('torso', (0, 0, NECK_Z)), 'halo': ('head', HALO_C), 'parcel': ('torso', PARCEL_C)}
for _tag, _s in SIDES:
    RIG['arm_'+_tag] = ('torso', (_s*SHX, 0, SHZ))
    RIG['forearm_'+_tag] = ('arm_'+_tag, (_s*SHX, 0, ELB_Z))
    RIG['leg_'+_tag] = ('structure', (_s*HX, 0, HIP_Z))
    RIG['shin_'+_tag] = ('leg_'+_tag, (_s*HX, 0, KNEE_Z))
    RIG['foot_'+_tag] = ('shin_'+_tag, (_s*HX, 0, ANK_Z))
RIG['slate'] = ('forearm_l', (-SHX-.012, -.062, 1.12))
ORDER = ['structure', 'torso', 'head', 'halo', 'arm_l', 'forearm_l', 'slate', 'arm_r', 'forearm_r',
         'leg_l', 'shin_l', 'foot_l', 'leg_r', 'shin_r', 'foot_r', 'parcel']
# Joints that carry keyframes in every clip; halo and slate ride rigidly on their parents.
JOINTS = ['structure', 'torso', 'head', 'arm_l', 'forearm_l', 'arm_r', 'forearm_r',
          'leg_l', 'shin_l', 'foot_l', 'leg_r', 'shin_r', 'foot_r']
CLIPS = ['idle', 'walk', 'turn', 'greet', 'work', 'carry']


def parts_for(kind):
    skip = set()
    if kind != 'courier': skip.add('parcel')
    if kind != 'archivist': skip |= {'halo', 'slate'}
    return [p for p in ORDER if p not in skip]


def part_materials(part, kind):
    """Fixed, ordered material list of a part: identical at every LOD (primitive order)."""
    base = part.split('_')[0]
    return {'structure': ('ceramic', 'graphite', 'robot-rubber'), 'torso': ('ceramic', 'graphite', ACCENT[kind]),
            'head': ('ceramic', 'glass', EYE[kind]), 'arm': ('ceramic', 'graphite', 'titanium'),
            'forearm': ('ceramic', 'graphite', FOREARM[kind]), 'leg': ('ceramic', 'titanium', 'graphite'),
            'shin': ('ceramic', 'titanium', 'graphite'), 'foot': ('graphite', 'robot-rubber', 'titanium'),
            'parcel': ('bronze', 'ivory'), 'halo': ('bronze', 'cyan'), 'slate': ('glass', 'cyan')}[base]


def robot_materials(mats):
    """Robot-local PBR materials, created like the shared palette (named `city.*`)."""
    for key, (color, metal, rough, emission) in ROBOT_PALETTE.items():
        name = 'city.'+key
        mat = bpy.data.materials.get(name) or bpy.data.materials.new(name)
        mat.use_nodes = True
        mat.diffuse_color = color
        bsdf = mat.node_tree.nodes.get('Principled BSDF')
        bsdf.inputs['Base Color'].default_value = color
        bsdf.inputs['Metallic'].default_value = metal
        bsdf.inputs['Roughness'].default_value = rough
        bsdf.inputs['Emission Color'].default_value = color
        bsdf.inputs['Emission Strength'].default_value = emission
        mats[key] = mat
    return mats


# --- geometry helpers ------------------------------------------------------------------
def sgnpow(v, e):
    return copysign(abs(v)**e, v)


def rring(cx, cy, z, rx, ry, n, p=2.6):
    """Counter-clockwise super-ellipse ring (p=2 is an ellipse, larger is boxier)."""
    e = 2/p
    return [(cx+rx*sgnpow(cos(2*pi*i/n), e), cy+ry*sgnpow(sin(2*pi*i/n), e), z) for i in range(n)]


def catmull(points, steps):
    pts = [Vector(p) for p in points]
    ext = [pts[0]*2-pts[1]]+pts+[pts[-1]*2-pts[-2]]
    out = []
    for i in range(1, len(ext)-2):
        a, b, c, d = ext[i-1], ext[i], ext[i+1], ext[i+2]
        for k in range(steps):
            t = k/steps
            out.append(.5*((2*b)+(-a+c)*t+(2*a-5*b+4*c-d)*t*t+(-a+3*b-3*c+d)*t**3))
    out.append(pts[-1])
    return out


def section_at(sections, z):
    """(width, depth, centre-y) of a (z, w, d, cy) section list at height z."""
    secs = sorted(sections, key=lambda sec: sec[0])
    for (z0, w0, d0, c0), (z1, w1, d1, c1) in zip(secs, secs[1:]):
        if z0 <= z <= z1:
            t = (z-z0)/(z1-z0)
            return w0+(w1-w0)*t, d0+(d1-d0)*t, c0+(c1-c0)*t
    return secs[-1][1:] if z > secs[-1][0] else secs[0][1:]


def thin(sections, lod):
    """Fewer cross-sections for the lower detail levels (first and last always stay)."""
    if lod == 0 or len(sections) <= 2: return sections
    if lod == 1: return [s for i, s in enumerate(sections) if i % 2 == 0 or i == len(sections)-1]
    return [sections[0], sections[len(sections)//2], sections[-1]] if len(sections) > 3 else [sections[0], sections[-1]]


class RobotGeometry(Geometry):
    def counts(self, *c): return c[self.lod]

    def add(self, vertices, faces, material, center=(0, 0, 0), rotation=None, smooth=False):
        if DEBUG_NORMALS:
            pts = [(rotation @ Vector(v) if rotation else Vector(v))+Vector(center) for v in vertices]
            mid = sum(pts, Vector())/len(pts)
            bad = total = 0
            for f in faces:
                if len(f) < 3: continue
                nrm = Vector()
                for i in range(len(f)):
                    a, b = pts[f[i]], pts[f[(i+1) % len(f)]]
                    nrm += Vector(((a.y-b.y)*(a.z+b.z), (a.z-b.z)*(a.x+b.x), (a.x-b.x)*(a.y+b.y)))
                cen = sum((pts[i] for i in f), Vector())/len(f)
                score = nrm.dot(cen-mid)
                if abs(score) > 1e-9:
                    total += 1
                    bad += score < 0
            if total > 4 and bad/total > .25:
                print('NORMALS?', self.part, material, f'{bad}/{total}', tuple(round(c, 3) for c in mid))
        super().add(vertices, faces, material, center, rotation, smooth)

    def tbox(self, center, size, mat, top=None, bevel=0, shift=(0, 0), rotation=None):
        """Bevelled box whose top face can be scaled (`top` = (w, d)) and shifted."""
        w, d, h = size
        bevel = min(bevel, min(size)*.24) if self.lod == 0 else 0
        verts, faces = box_template((w, d, h), bevel)
        tw, td = top or (w, d)
        fx, fy = tw/w-1, td/d-1
        out = []
        for x, y, z in verts:
            f = z/h+.5
            out.append((x*(1+fx*f)+shift[0]*f, y*(1+fy*f)+shift[1]*f, z))
        self.add(out, faces, mat, center, rotation)

    def loft(self, rings, mat, center=(0, 0, 0), rotation=None, caps=(True, True), smooth=True):
        n = len(rings[0])
        verts = [p for ring in rings for p in ring]
        faces = [(k*n+i, k*n+(i+1) % n, (k+1)*n+(i+1) % n, (k+1)*n+i) for k in range(len(rings)-1) for i in range(n)]
        self.add(verts, faces, mat, center, rotation, smooth)
        if caps[0]: self.add(rings[0], [tuple(reversed(range(n)))], mat, center, rotation)
        if caps[1]: self.add(rings[-1], [tuple(range(n))], mat, center, rotation)

    def limb(self, cx, sections, mat, n=None, p=2.6, caps=(True, True)):
        """Tapered, rounded shell through (z, width, depth, centre-y) sections."""
        n = n or self.counts(12, 8, 6)
        sections = sorted(sections, key=lambda sec: sec[0])
        self.loft([rring(cx, cy, z, w/2, d/2, n, p) for z, w, d, cy in thin(sections, self.lod)], mat, caps=caps)

    def lathe(self, center, profile, mat, n=None, scale=(1, 1), rotation=None, caps=(True, True)):
        n = n or self.counts(12, 8, 6)
        rings = [[(r*cos(2*pi*i/n)*scale[0], r*sin(2*pi*i/n)*scale[1], z) for i in range(n)] for r, z in profile]
        self.loft(rings, mat, center, rotation, caps)

    def sphere(self, center, radius, mat, scale=(1, 1, 1), rotation=None, seg=None, rings=None):
        seg = seg or self.counts(12, 9, 6)
        rings = rings or self.counts(7, 5, 4)
        verts = [(0, 0, -1)]
        for j in range(1, rings):
            a = -pi/2+pi*j/rings
            verts += [(cos(a)*cos(2*pi*i/seg), cos(a)*sin(2*pi*i/seg), sin(a)) for i in range(seg)]
        verts.append((0, 0, 1))
        top = len(verts)-1
        faces = [(0, 1+(i+1) % seg, 1+i) for i in range(seg)]
        for j in range(rings-2):
            b = 1+j*seg
            faces += [(b+i, b+(i+1) % seg, b+seg+(i+1) % seg, b+seg+i) for i in range(seg)]
        b = 1+(rings-2)*seg
        faces += [(top, b+i, b+(i+1) % seg) for i in range(seg)]
        verts = [(x*radius*scale[0], y*radius*scale[1], z*radius*scale[2]) for x, y, z in verts]
        self.add(verts, faces, mat, center, rotation, True)

    def torus(self, center, major, minor, mat, n, sides):
        verts = [((major+minor*cos(2*pi*j/sides))*cos(2*pi*i/n), (major+minor*cos(2*pi*j/sides))*sin(2*pi*i/n),
                  minor*sin(2*pi*j/sides)) for i in range(n) for j in range(sides)]
        faces = [(i*sides+j, (i+1) % n*sides+j, (i+1) % n*sides+(j+1) % sides, i*sides+(j+1) % sides)
                 for i in range(n) for j in range(sides)]
        self.add(verts, faces, mat, center, None, True)

    def tube(self, a, b, ra, rb, mat, n=None):
        a, b = Vector(a), Vector(b)
        self.cyl((a+b)/2, ra, (b-a).length, mat, top=rb, n=n or self.counts(12, 8, 6),
                 rotation=(b-a).to_track_quat('Z', 'Y').to_matrix())

    def sweep(self, points, radius, mat, steps=4, sides=None):
        """Smooth hose along a Catmull-Rom path, parallel-transport frames."""
        sides = sides or self.counts(6, 5, 4)
        pts = catmull(points, steps if self.lod == 0 else max(2, steps//2))
        tans = [(pts[min(i+1, len(pts)-1)]-pts[max(i-1, 0)]).normalized() for i in range(len(pts))]
        nrm = tans[0].cross(Vector((0, 0, 1)))
        if nrm.length < .2: nrm = tans[0].cross(Vector((1, 0, 0)))
        rings = []
        for p, t in zip(pts, tans):
            nrm = (nrm-t*nrm.dot(t)).normalized()
            bi = t.cross(nrm)
            rings.append([tuple(p+radius*(cos(2*pi*k/sides)*nrm+sin(2*pi*k/sides)*bi)) for k in range(sides)])
        self.loft(rings, mat)

    def finger(self, base, direction, lengths, widths, curls, mat='graphite'):
        """Chain of tapered boxes; `direction` is a rotation, each joint curls about local X."""
        pos, rot = Vector(base), direction
        for length, (w, d), curl in zip(lengths, widths, curls):
            rot = rot @ Matrix.Rotation(curl, 3, 'X')
            c = pos+rot @ Vector((0, 0, -length/2))
            self.tbox(c, (w, d, length), mat, top=(w*.94, d*.94), rotation=rot)
            pos = pos+rot @ Vector((0, 0, -length))


def surface(z, helmet):
    """Interpolated (rx, ry, cy) of the helmet shell at height z."""
    for (z0, rx0, ry0, cy0), (z1, rx1, ry1, cy1) in zip(helmet, helmet[1:]):
        if z0 <= z <= z1:
            t = (z-z0)/(z1-z0)
            return rx0+(rx1-rx0)*t, ry0+(ry1-ry0)*t, cy0+(cy1-cy0)*t
    return helmet[-1][1:]


RX = lambda a: Matrix.Rotation(a, 3, 'X')
RY = lambda a: Matrix.Rotation(a, 3, 'Y')
RZ = lambda a: Matrix.Rotation(a, 3, 'Z')
Z_TO_X, Z_TO_Y = RY(pi/2), RX(-pi/2)


# --- part builders ------------------------------------------------------------------
# Level of detail: 0 = full, 1 = no fine detail (bevels, fingers, slats, bolts), 2 = silhouette
# blocks. Every level emits every material of the part so the exported primitive order matches.
def build_structure(g, kind, s):
    lod = g.lod
    if lod == 2:
        g.tbox((0, 0, 1.03), (.42, .27, .16), 'ceramic')
        g.tbox((0, 0, .965), (.31, .22, .06), 'graphite')
        g.lathe((0, 0, 0), [(.15, 1.1), (.15, 1.236)], 'robot-rubber', n=6, scale=(1, .79), caps=(False, False))
        return
    n = g.counts(14, 10, 6)
    g.limb(0, [(.955, .30, .22, 0), (1.03, .42, .265, 0), (1.115, .39, .27, 0)], 'ceramic', n=n)
    g.tbox((0, 0, .965), (.31, .22, .06), 'graphite')
    for sd in (-1, 1):
        g.cyl((sd*.225, 0, 1.04), .105, .035, 'graphite', n=g.counts(12, 7, 6), rotation=Z_TO_X)
    # belt and the ribbed flex sleeve that lets the torso bend over the pelvis
    g.limb(0, [(1.075, .445, .29, 0), (1.12, .445, .29, 0)], 'robot-rubber', n=n, p=3.2)
    g.tbox((0, -.147, 1.098), (.075, .02, .045), 'graphite')
    ribs = [(.14, 1.105), (.162, 1.125), (.14, 1.145), (.162, 1.165), (.14, 1.185), (.162, 1.205), (.14, 1.225)]
    if lod == 1: ribs = [ribs[0], ribs[2], ribs[4], ribs[6]]
    g.lathe((0, 0, 0), ribs, 'robot-rubber', n=n, scale=(1, .79), caps=(False, False))
    if kind == 'courier':
        g.tbox((0, .17, 1.085), (.2, .07, .1), 'robot-rubber')
        g.tbox((0, .17, 1.125), (.205, .075, .03), 'graphite')
    elif kind == 'technician':
        g.limb(0, [(1.0, .44, .29, 0), (1.04, .44, .29, 0)], 'graphite', n=n, p=3.2)
        for sd in (-1, 1):
            g.tbox((sd*.125, -.158, 1.065), (.085, .05, .1), 'robot-rubber')
            if lod == 0: g.tbox((sd*.125, -.158, 1.12), (.09, .052, .03), 'robot-rubber')
        g.tbox((0, .165, 1.07), (.16, .06, .1), 'robot-rubber')
        if lod == 0: g.tube((.17, .14, 1.095), (.17, .14, 1.015), .014, .014, 'graphite', n=5)
    else:
        g.tube((-.17, .15, .95), (-.17, .15, 1.1), .028, .028, 'robot-rubber', n=g.counts(10, 8, 6))
        g.tube((-.17, .15, 1.1), (-.17, .15, 1.115), .034, .034, 'graphite', n=g.counts(10, 8, 6))


def build_torso(g, kind, s):
    acc, lod = ACCENT[kind], g.lod
    if lod == 2:
        g.tbox((0, 0, 1.43), (.30, .22, .5), 'graphite')
        g.tbox((0, 0, 1.29), (.37, .285, .17), 'ceramic')
        g.tbox((0, -.005, 1.52), (.43, .30, .27), 'ceramic', top=(.54, .315))
        g.tbox((0, .205, 1.455), (.34, .15, .44), 'graphite', top=(.30, .13))
        g.tbox((0, -.186, 1.545), (.30, .012, .05), acc)
        return
    n = g.counts(14, 10, 8)
    # undersuit core, visible in the seams
    g.limb(0, [(1.17, .27, .20, 0), (1.40, .38, .27, 0), (1.70, .44, .28, 0)], 'graphite', n=n, p=2.4)
    # layered plates
    g.tbox((0, 0, 1.235), (.34, .27, .075), 'ceramic', top=(.36, .28), bevel=.014)
    g.tbox((0, 0, 1.33), (.37, .285, .095), 'ceramic', top=(.405, .30), bevel=.016)
    g.tbox((0, -.005, 1.52), (.43, .30, .27), 'ceramic', top=(.54, .315), bevel=.034, shift=(0, -.01))
    g.tbox((0, 0, 1.675), (.46, .28, .075), 'ceramic', top=(.42, .24), bevel=.02)
    g.tbox((0, -.17, 1.50), (.2, .03, .17), 'ceramic', top=(.23, .03), bevel=.012)
    g.tbox((0, -.186, 1.545), (.30, .012, .05), acc)
    if kind == 'courier':
        # bandolier strap with a bronze buckle
        g.tbox((.035, -.196, 1.48), (.05, .01, .5), 'graphite', rotation=RY(.62))
        g.tbox((-.12, -.2, 1.385), (.05, .012, .05), acc)
    else:
        g.tbox((0, -.186, 1.43), (.05, .012, .05), acc)
    for sd in (-1, 1):
        for i in range(4 if lod == 0 else 3):
            g.tbox((sd*.095, -.1535, 1.285+i*(.02 if lod == 0 else .03)), (.1, .012, .009), 'graphite')
    if lod == 0:
        g.tbox((0, -.1485, 1.33), (.006, .01, .17), 'graphite')
        g.tbox((0, -.1475, 1.285), (.34, .01, .005), 'graphite')
        for sd in (-1, 1):
            g.tbox((sd*.145, -.171, 1.49), (.004, .012, .17), 'graphite')
    # collar and cables around the neck socket
    g.lathe((0, 0, 0), [(.115, 1.685), (.115, 1.705), (.095, 1.715), (.088, 1.76)], 'graphite', n=g.counts(12, 8, 6), caps=(False, True))
    # back pack with cooling fins, status light and exhausts
    g.tbox((0, .205, 1.455), (.34, .15, .44), 'graphite', top=(.30, .13), bevel=.02)
    g.tbox((0, .21, 1.69), (.32, .16, .04), 'ceramic', bevel=.012)
    g.tbox((0, .295, 1.69), (.15, .012, .02), acc)
    for sd in (-1, 1):
        g.tbox((sd*.172, .2, 1.46), (.03, .16, .38), 'ceramic')
        if lod == 0: g.tube((sd*.085, .28, 1.36), (sd*.085, .36, 1.36), .033, .036, 'graphite', n=10)
    for i in range(5 if lod == 0 else 3):
        g.tbox((0, .292, 1.30+i*(.065 if lod == 0 else .11)), (.28, .05, .012), 'graphite')
    if lod == 0:
        for sd in (-1, 1):
            g.sweep([(sd*.06, .06, 1.73), (sd*.085, .11, 1.77), (sd*.11, .17, 1.74), (sd*.1, .2, 1.70)], .011, 'graphite', steps=2)
    if kind == 'technician':
        g.tube((.265, -.08, 1.435), (.265, -.08, 1.565), .032, .032, 'jade', n=g.counts(10, 8, 6))
        g.sweep([(.17, .28, 1.40), (.25, .22, 1.37), (.285, .10, 1.39), (.28, -.03, 1.43), (.265, -.075, 1.45)], .013, 'jade', steps=2)
        g.tbox((.265, -.08, 1.575), (.07, .07, .02), 'graphite')
    elif kind == 'archivist':
        for x in (-.085, 0, .085):
            g.tbox((x, .297, 1.46), (.012, .008, .3), acc)


def build_head(g, kind, s):
    lod = g.lod
    helmet = [(1.775, .105, .110, .012), (1.835, .150, .160, .012), (1.93, .200, .205, .012),
              (2.03, .200, .210, .016), (2.105, .170, .185, .020), (2.160, .095, .105, .022)]
    p = 2.5
    n = g.counts(16, 12, 8)
    shell = thin(helmet, lod)
    g.loft([rring(0, cy, z, rx, ry, n, p) for z, rx, ry, cy in shell], 'ceramic')
    if lod < 2:
        # ribbed neck stub
        neck = [(.058, 1.69), (.058, 1.725), (.072, 1.735), (.058, 1.748), (.068, 1.765), (.05, 1.79)]
        g.lathe((0, 0, 0), neck if lod == 0 else [neck[0], neck[2], neck[5]], 'ceramic', n=g.counts(12, 8, 6))
    front = 3*n//4

    def band(z_rows, k, lift, mat):
        # Columns share the helmet's vertex angles, so the band is an exact offset of the shell.
        rows = []
        for z in z_rows:
            rx, ry, cy = surface(z, [(sec[0], sec[1], sec[2], sec[3]) for sec in shell])
            rows.append([((rx+lift)*sgnpow(cos(2*pi*(front+j)/n), 2/p), cy+(ry+lift)*sgnpow(sin(2*pi*(front+j)/n), 2/p), z)
                         for j in range(-k, k+1)])
        cols = 2*k+1
        verts = [v for r in rows for v in r]
        faces = [(j*cols+i, j*cols+i+1, (j+1)*cols+i+1, (j+1)*cols+i) for j in range(len(rows)-1) for i in range(cols-1)]
        g.add(verts, faces, mat, smooth=True)

    band([1.905, 1.97, 2.035] if lod == 0 else [1.905, 2.035], g.counts(3, 2, 1), .007, 'glass')
    band([1.955, 1.99], g.counts(2, 1, 1), .0115, EYE[kind])
    if lod < 2:
        g.tbox((0, -.196, 2.062), (.28, .045, .03), 'ceramic', top=(.26, .035), bevel=.01)
        rx, ry, cy = surface(1.95, helmet)
        for sd in (-1, 1):
            g.cyl((sd*(rx+.018), cy, 1.95), .066, .042, 'ceramic', n=g.counts(12, 8, 6), rotation=Z_TO_X)
            if lod == 0: g.cyl((sd*(rx+.04), cy, 1.95), .034, .006, EYE[kind], n=8, rotation=Z_TO_X)
    if kind == 'courier':
        g.sphere((0, .02, 2.168), .036, 'ceramic', scale=(1, 1, .6), seg=g.counts(8, 6, 5), rings=g.counts(4, 3, 3))
    elif kind == 'technician':
        g.tbox((0, -.2, 2.1), (.07, .03, .035), 'ceramic', bevel=.008)
        if lod < 2: g.tbox((0, -.218, 2.1), (.045, .01, .02), EYE[kind])
    else:
        g.tube((.08, .07, 2.14), (.08, .07, 2.31), .009, .005, 'ceramic', n=g.counts(6, 5, 4))
        g.sphere((.08, .07, 2.318), .016, EYE[kind], seg=g.counts(6, 5, 4), rings=g.counts(3, 3, 3))


def build_halo(g, kind, s):
    c = Vector(HALO_C)
    g.torus(c, .27, .02, 'bronze', g.counts(28, 18, 10), g.counts(5, 4, 4))
    for k in range(6):
        a = k*pi/3+pi/6
        g.tbox(c+Vector((.27*cos(a), .27*sin(a), 0)), (.05, .036, .036), 'cyan', rotation=RZ(a))
    for a in (pi/2, pi/2+2*pi/3, pi/2-2*pi/3):
        g.tube((.12*cos(a), .12*sin(a)+.01, 2.14), (.268*cos(a), .268*sin(a), 2.25), .008, .008, 'bronze', n=g.counts(6, 4, 4))


def build_arm(g, kind, s):
    cx, lod = s*SHX, g.lod
    if lod == 2:
        g.sphere((cx, 0, SHZ), .088, 'graphite', seg=6, rings=3)
        g.limb(cx, [(SHZ+.03, .15, .15, 0), (1.31, .09, .096, 0)], 'ceramic', n=6)
        g.tube((cx+s*.066, 0, 1.52), (cx+s*.066, 0, 1.34), .016, .016, 'titanium', n=5)
        return
    g.sphere((cx, 0, SHZ), .088, 'graphite', seg=g.counts(9, 6, 6), rings=g.counts(5, 4, 3))
    g.sphere((cx+s*.012, 0, SHZ+.052), .105, 'ceramic', scale=(1.08, 1.2, .66), rotation=RY(-s*.28),
             seg=g.counts(10, 7, 6), rings=g.counts(5, 4, 3))
    g.limb(cx, [(SHZ-.06, .122, .13, 0), (1.40, .115, .122, 0), (1.31, .09, .096, 0)], 'ceramic')
    if lod == 0: g.limb(cx, [(1.355, .102, .108, 0), (ELB_Z+.02, .1, .106, 0)], 'graphite', n=10, p=3)
    g.cyl((cx, 0, ELB_Z), .06, .118, 'graphite', n=g.counts(12, 8, 6), rotation=Z_TO_X)
    if lod == 0: g.cyl((cx, 0, ELB_Z), .028, .134, 'titanium', n=8, rotation=Z_TO_X)
    ox = cx+s*.066
    g.tube((ox, 0, 1.52), (ox, 0, 1.43), .019, .019, 'titanium', n=g.counts(8, 6, 5))
    g.tube((ox, 0, 1.43), (ox, 0, 1.335), .011, .011, 'titanium', n=g.counts(6, 5, 4))


def build_forearm(g, kind, s):
    cx, lod = s*SHX, g.lod
    acc = FOREARM[kind]
    tool = kind == 'technician' and s > 0
    if lod == 2:
        g.limb(cx, [(1.26, .1, .105, 0), (1.0, .076, .082, 0)], 'ceramic', n=6)
        g.tbox((cx, .004, 1.125), (.116, .124, .03), acc)
        g.tbox((cx, -.004, .89), (.094, .054, .21), 'graphite')
        if tool: g.tbox((cx+s*.082, .016, 1.1), (.05, .066, .21), 'bronze')
        return
    n = g.counts(14, 8, 6)
    shell = [(1.235, .092, .1, 0), (1.15, .108, .114, .004), (1.0, .076, .082, 0)]
    g.limb(cx, shell, 'ceramic', n=n)
    if lod == 0:
        g.limb(cx, [(ELB_Z-.015, .09, .095, 0), (1.215, .098, .105, 0)], 'graphite', n=10, p=3)
        lo, hi = section_at(shell, 1.108), section_at(shell, 1.142)
        g.limb(cx, [(1.108, lo[0]+.014, lo[1]+.014, lo[2]), (1.142, hi[0]+.014, hi[1]+.014, hi[2])], acc, n=12, p=3)
        g.cyl((cx, 0, .995), .054, .04, 'graphite', n=10)
    else:
        g.tbox((cx, .004, 1.125), (.116, .124, .028), acc)
    # hand: palm + three fingers + thumb (blocks at the lower levels)
    g.tbox((cx, -.002, .925), (.094, .052, .104), 'graphite', top=(.1, .054), bevel=.012)
    if lod == 0:
        for dx, curl in zip((-.032, 0, .032), (-.12, -.2, -.12)):
            g.finger((cx+dx, -.004, .873), Matrix.Identity(3), (.05, .042), ((.024, .026), (.022, .024)), (curl, -.55))
        g.finger((cx-s*.05, -.012, .94), RZ(-s*.55) @ RX(-.5), (.048, .04), ((.026, .028), (.022, .024)), (0, -.5))
    else:
        for dx in (-.032, 0, .032):
            g.tbox((cx+dx, -.012, .81), (.025, .028, .09), 'graphite', rotation=RX(-.12))
        g.tbox((cx-s*.05, -.02, .905), (.028, .03, .085), 'graphite', rotation=RZ(-s*.55) @ RX(-.5))
    if tool:
        # bronze multitool strapped to the outer forearm
        ox = cx+s*.082
        g.tbox((ox, .016, 1.1), (.05, .066, .21), 'bronze', top=(.044, .06), bevel=.01)
        g.tbox((ox+s*.03, .0, 1.1), (.014, .05, .16), 'bronze')
        g.tube((ox, .0, 1.0), (ox, -.01, .87), .014, .006, 'bronze', n=g.counts(8, 6, 4))
        if lod == 0:
            g.tbox((ox, -.018, 1.15), (.03, .01, .03), 'bronze')
            g.tbox((ox, -.02, 1.085), (.034, .008, .012), 'bronze')


def build_slate(g, kind, s):
    cx, cy, cz = RIG['slate'][1]
    g.tbox((cx, cy, cz), (.108, .02, .21), 'glass', bevel=.006)
    for i in range(4 if g.lod == 0 else 2):
        w = (.075, .05, .066, .04)[i]
        g.tbox((cx+(w-.075)/2*.4-.012, cy-.0115, cz+.065-i*(.036 if g.lod == 0 else .07)), (w, .004, .012), 'cyan')
    if g.lod == 0: g.tbox((cx, cy-.0115, cz-.082), (.07, .004, .014), 'cyan')


def build_leg(g, kind, s):
    cx, lod = s*HX, g.lod
    if lod == 2:
        g.limb(cx, [(1.02, .17, .19, 0), (.62, .11, .135, 0)], 'ceramic', n=6)
        g.tbox((cx, 0, .98), (.12, .13, .07), 'graphite')
        g.tube((cx+s*.092, 0, .955), (cx+s*.092, 0, .66), .016, .016, 'titanium', n=5)
        return
    g.sphere((cx, 0, HIP_Z), .09, 'graphite', seg=g.counts(9, 6, 6), rings=g.counts(5, 4, 3))
    g.limb(cx, [(1.0, .17, .19, 0), (.9, .178, .2, 0), (.76, .15, .18, 0), (.635, .11, .135, 0)], 'ceramic')
    g.tbox((cx, -.096, .83), (.118, .036, .25), 'ceramic', top=(.14, .038), bevel=.014 if lod == 0 else 0)
    if lod == 0: g.limb(cx, [(.67, .118, .14, 0), (.62, .104, .124, 0)], 'graphite', n=10, p=3)
    ox = cx+s*.092
    g.tube((ox, 0, .955), (ox, 0, .79), .021, .021, 'titanium', n=g.counts(8, 6, 5))
    g.tube((ox, 0, .79), (ox, 0, .66), .012, .012, 'titanium', n=g.counts(6, 5, 4))
    g.tbox((cx, -.092, .645), (.09, .04, .06), 'titanium', top=(.075, .04), bevel=.012)
    if lod == 0:
        g.tbox((cx+s*.085, 0, .965), (.022, .05, .032), 'titanium')
        g.tbox((cx-s*.085, 0, .82), (.012, .08, .2), 'graphite')


def build_shin(g, kind, s):
    cx, lod = s*HX, g.lod
    if lod == 2:
        g.limb(cx, [(.58, .12, .13, .014), (.19, .074, .09, .01)], 'ceramic', n=6)
        g.tbox((cx, 0, .14), (.1, .12, .08), 'graphite')
        g.tube((cx, .098, .54), (cx, .098, .27), .016, .016, 'titanium', n=5)
        return
    g.cyl((cx, 0, KNEE_Z), .056, .132, 'graphite', n=g.counts(12, 8, 6), rotation=Z_TO_X)
    if lod == 0: g.limb(cx, [(.575, .1, .115, 0), (.2, .066, .08, 0)], 'graphite', n=10)
    g.limb(cx, [(.565, .12, .13, .014), (.45, .124, .142, .021), (.3, .098, .12, .019), (.19, .074, .09, .01)], 'ceramic')
    g.tbox((cx, -.068, .41), (.088, .032, .3), 'ceramic', top=(.108, .034), bevel=.008 if lod == 0 else 0)
    g.tube((cx, .098, .54), (cx, .098, .43), .02, .02, 'titanium', n=g.counts(8, 6, 5))
    g.tube((cx, .098, .43), (cx, .098, .27), .011, .011, 'titanium', n=g.counts(6, 5, 4))
    g.sphere((cx, 0, ANK_Z), .06, 'graphite', scale=(1, 1.05, 1), seg=g.counts(9, 6, 6), rings=g.counts(5, 4, 3))
    if lod == 0:
        g.tbox((cx, .092, .555), (.1, .03, .035), 'titanium')


def build_foot(g, kind, s):
    cx, lod = s*HX, g.lod
    if lod == 2:
        g.tbox((cx, -.085, .022), (.142, .37, .044), 'robot-rubber')
        g.tbox((cx, -.07, .0845), (.126, .31, .081), 'graphite')
        g.tbox((cx, -.215, .076), (.126, .095, .064), 'titanium')
        return
    g.tbox((cx, -.085, .022), (.142, .37, .044), 'robot-rubber', top=(.136, .36), bevel=.014)
    g.tbox((cx, -.07, .0845), (.126, .31, .081), 'graphite', top=(.106, .24), bevel=.02, shift=(0, -.02))
    g.tbox((cx, -.215, .076), (.126, .095, .064), 'titanium', top=(.11, .07), bevel=.02, shift=(0, -.01))
    g.tbox((cx, .075, .088), (.11, .06, .085), 'titanium')
    for sd in (-1, 1):
        g.tbox((cx+sd*.062, 0, .118), (.016, .11, .08), 'titanium')
    if lod == 0:
        g.tbox((cx, -.265, .03), (.12, .02, .04), 'robot-rubber')
        g.tbox((cx, -.05, .1305), (.07, .18, .012), 'graphite')


def build_parcel(g, kind, s):
    cx, cy, cz = PARCEL_C
    g.tbox((cx, cy, cz), (.5, .30, .30), 'bronze', bevel=.026)
    g.tbox((cx, cy, cz), (.516, .316, .05), 'ivory')
    if g.lod < 2:
        g.tbox((cx, cy-.157, cz+.02), (.2, .01, .1), 'ivory')
    if g.lod == 0:
        for sd in (-1, 1):
            g.tbox((cx+sd*.2, cy-.158, cz-.02), (.03, .008, .16), 'ivory')


BUILDERS = {'structure': build_structure, 'torso': build_torso, 'head': build_head, 'halo': build_halo,
            'arm': build_arm, 'forearm': build_forearm, 'slate': build_slate, 'leg': build_leg,
            'shin': build_shin, 'foot': build_foot, 'parcel': build_parcel}


def create_robot_asset(asset_id, kind, lod, collection, mats):
    """Build one robot LOD as a parent hierarchy with a fixed per-part material order."""
    g = RobotGeometry(lod)
    for part in parts_for(kind):
        g.part = part
        BUILDERS[part.split('_')[0]](g, kind, -1 if part.endswith('_l') else 1)
    root = bpy.data.objects.new(f'{asset_id}.lod{lod}', None)
    collection.objects.link(root)
    root['asset_id'], root['lod'], root['license'] = asset_id, lod, 'MIT'
    objects = {}
    for part in parts_for(kind):
        verts, faces, keys, smooth = g.parts[part]
        order = part_materials(part, kind)
        assert set(keys) == set(order), (asset_id, lod, part, sorted(set(keys)), order)
        pivot = Vector(RIG[part][1])
        mesh = bpy.data.meshes.new(f'{asset_id}.{part}')
        mesh.from_pydata([tuple(Vector(v)-pivot) for v in verts], [], faces)
        mesh.update()
        for key in order: mesh.materials.append(mats[key])
        for polygon, key, smoothed in zip(mesh.polygons, keys, smooth):
            polygon.material_index = order.index(key)
            polygon.use_smooth = smoothed
        obj = bpy.data.objects.new(part, mesh)
        parent = RIG[part][0]
        obj.parent = objects[parent] if parent else root
        obj.location = pivot-(Vector(RIG[parent][1]) if parent else Vector())
        collection.objects.link(obj)
        obj['component'] = part
        objects[part] = obj
    return root


# --- bounds over the whole hierarchy --------------------------------------------------
def meshes_of(root):
    return [o for o in root.children_recursive if o.type == 'MESH']


def bounds_and_triangles_tree(root):
    """`bounds_and_triangles` for nested parts (world matrices of every descendant)."""
    coords, triangles = [], 0
    for obj in meshes_of(root):
        obj.data.calc_loop_triangles()
        triangles += len(obj.data.loop_triangles)
        coords.extend(obj.matrix_world @ Vector(p) for p in obj.bound_box)
    low = [min(v[i] for v in coords) for i in range(3)]
    high = [max(v[i] for v in coords) for i in range(3)]
    return {'min': [low[0], low[2], -high[1]], 'max': [high[0], high[2], -low[1]]}, triangles


def motion_bounds_tree(root, clips, margin=.03):
    scene = bpy.context.scene
    bpy.context.view_layer.update()
    bounds, _ = bounds_and_triangles_tree(root)
    moving = [o for o in root.children_recursive if o.animation_data]
    bases = [(o, o.location.copy(), o.rotation_euler.copy()) for o in moving]
    for clip in clips:
        for o in moving:
            track = next((t for t in o.animation_data.nla_tracks if t.name == clip), None)
            o.animation_data.action = track.strips[0].action if track else None
        for frame in range(0, 49):
            scene.frame_set(frame)
            bpy.context.view_layer.update()
            current, _ = bounds_and_triangles_tree(root)
            for i in range(3):
                bounds['min'][i] = min(bounds['min'][i], current['min'][i])
                bounds['max'][i] = max(bounds['max'][i], current['max'][i])
    for o, location, rotation in bases:
        o.animation_data.action = None
        o.location, o.rotation_euler = location, rotation
    scene.frame_set(1)
    bpy.context.view_layer.update()
    return {'min': [v-margin for v in bounds['min']], 'max': [v+margin for v in bounds['max']]}


# --- kinematics ------------------------------------------------------------------------
def solve_two_bone(origin, target, l1, l2, pole, hinge_sign):
    """Analytic two-bone IK in the parent frame.

    Returns (rotation of the first bone, signed flexion about its local X, reach error).
    The first bone hangs along local -Z at rest; `pole` is where the middle joint points.
    """
    d = target-origin
    reach = d.length
    limit = (l1+l2)*.9995
    error = max(0, reach-limit)
    if reach > limit:
        d = d*(limit/reach)
        reach = limit
    reach = max(reach, abs(l1-l2)+.01)
    dhat = d.normalized()
    perp = pole-dhat*pole.dot(dhat)
    perp.normalize()
    cos_a = max(-1, min(1, (l1*l1+reach*reach-l2*l2)/(2*l1*reach)))
    a = acos(cos_a)
    knee = origin+l1*(cos(a)*dhat+sin(a)*perp)
    u1 = (knee-origin).normalized()
    u2 = (origin+d-knee).normalized()
    xt = (dhat.cross(perp) if hinge_sign > 0 else perp.cross(dhat)).normalized()
    zt = -u1
    yt = zt.cross(xt)
    rot = Matrix(((xt.x, yt.x, zt.x), (xt.y, yt.y, zt.y), (xt.z, yt.z, zt.z)))
    local2 = rot.transposed() @ u2
    return rot, atan2(local2.y, -local2.z), error


def smoothstep(x):
    x = max(0, min(1, x))
    return x*x*x*(x*(6*x-15)+10)


def hermite(p0, p1, m0, m1, t):
    return ((2*t**3-3*t*t+1)*p0+(t**3-2*t*t+t)*m0+(-2*t**3+3*t*t)*p1+(t**3-t*t)*m1)


# Gait: two cycles per clip, stance 60 % at the exact speed the runtime walks.
V = 1.65/1.15
T_CYCLE, STANCE = 1.0, .6
HEEL, TOE = (-.085, -.13), (.27, -.13)          # sole contact points (forward, up) in the foot frame
PSI_HS, PSI_TO = radians(11), radians(22)
ROCK_A, ROCK_B = .14, .58
LIFT = .11


def rot2(f, z, psi):
    return f*cos(psi)-z*sin(psi), f*sin(psi)+z*cos(psi)


def stance_foot(tau, f_heel0=0.0):
    """Ankle (forward, height, toe-up pitch) `tau` seconds after heel strike."""
    n = tau/(STANCE*T_CYCLE)
    heel = f_heel0-V*tau
    if n < ROCK_A:
        psi = PSI_HS*cos(pi/2*n/ROCK_A)**2
        rf, rz = rot2(*HEEL, psi)
        return heel-rf, -rz, psi
    if n < ROCK_B:
        return heel-HEEL[0], -HEEL[1], 0.0
    psi = -PSI_TO*sin(pi/2*(n-ROCK_B)/(1-ROCK_B))**2
    rf, rz = rot2(*TOE, psi)
    return heel+(TOE[0]-HEEL[0])-rf, -rz, psi


_samples = [stance_foot(STANCE*T_CYCLE*i/200) for i in range(201)]
F_HEEL0 = .005-(min(s[0] for s in _samples)+max(s[0] for s in _samples))/2


def gait_foot(ph):
    """Ankle target of one foot at gait phase `ph` (0 = heel strike), body frame."""
    if ph < STANCE:
        return stance_foot(ph*T_CYCLE, F_HEEL0)
    eps = 1e-4
    t_sw = (1-STANCE)*T_CYCLE
    q = (ph-STANCE)/(1-STANCE)
    e0 = stance_foot(STANCE*T_CYCLE, F_HEEL0)
    e1 = stance_foot(STANCE*T_CYCLE-eps, F_HEEL0)
    b0 = stance_foot(0, F_HEEL0)
    b1 = stance_foot(eps, F_HEEL0)
    vel0 = [(e0[i]-e1[i])/eps for i in range(2)]
    vel1 = [(b1[i]-b0[i])/eps for i in range(2)]
    f = hermite(e0[0], b0[0], vel0[0]*t_sw, vel1[0]*t_sw, q)
    z = hermite(e0[1], b0[1], vel0[1]*t_sw, vel1[1]*t_sw, q)+LIFT*sin(pi*q)**2
    psi = e0[2]+(b0[2]-e0[2])*smoothstep(q)
    return f, z, psi


FOOT_REST = (HX, 0)
A_BOB = .03


def calibrate_walk():
    """Highest mean pelvis height at which both legs reach every gait frame."""
    best = None
    for k in range(0, 80):
        z0 = -.001*k
        worst = 0
        for i in range(48):
            g = i/24
            for tag, off in (('l', 0), ('r', .5)):
                f, z, _ = gait_foot((g+off) % 1)
                hip = Vector((0, 0, HIP_Z+z0-A_BOB*cos(4*pi*(g-.03))))
                tgt = Vector((0, -f, z))
                worst = max(worst, (tgt-hip).length-(L_THIGH+L_SHIN)*.998)
        if worst <= 0:
            best = z0
            break
    return best


WALK_Z0 = calibrate_walk()


# --- poses -----------------------------------------------------------------------------
class Pose:
    def __init__(self):
        self.loc, self.rot, self.error = {}, {}, 0.0


def euler_rot(yaw=0, roll=0, pitch=0):
    """Rz(yaw) Ry(roll) Rx(pitch); positive pitch leans/looks forward, positive roll lowers +X."""
    return RZ(yaw) @ RY(roll) @ RX(pitch)


def stand_feet(lift_l=0.0, lift_r=0.0):
    return {tag: (s*HX, 0.0, ANK_Z, 0.0, s*.08) for tag, s in SIDES}


def solve_legs(pose, pelvis_loc, rp, feet):
    pelvis = Matrix.Translation(Vector((0, 0, HIP_Z))+pelvis_loc) @ rp.to_4x4()
    inverse = pelvis.inverted()
    pose.loc['structure'] = pelvis_loc
    pose.rot['structure'] = rp
    for tag, s in SIDES:
        x, f, z, psi, yaw = feet[tag]
        target = inverse @ Vector((x, -f, z))
        rot, flex, err = solve_two_bone(Vector((s*HX, 0, 0)), target, L_THIGH, L_SHIN, Vector((s*.12, -1, 0)), -1)
        pose.error = max(pose.error, err)
        pose.rot['leg_'+tag] = rot
        pose.rot['shin_'+tag] = RX(flex)
        pose.rot['foot_'+tag] = (rot @ RX(flex)).transposed() @ (rp.transposed() @ (RZ(yaw) @ RX(-psi)))


def arm_fk(pose, tag, s, pitch, abduct, elbow, wave=0.0):
    pose.rot['arm_'+tag] = RX(pitch) @ RY(abduct)
    pose.rot['forearm_'+tag] = RZ(wave) @ RX(elbow)


def arm_ik(pose, tag, s, wrist, rt):
    """Hand held at `wrist` (rest-pose world coordinates), solved in the torso frame."""
    local = Vector(wrist)-Vector((0, 0, WAIST_Z))
    rot, flex, err = solve_two_bone(Vector((s*SHX, 0, SHZ-WAIST_Z)), local, L_UPPER, L_FORE, Vector((s*.5, 1, -.35)), 1)
    pose.error = max(pose.error, err)
    pose.rot['arm_'+tag] = rot
    pose.rot['forearm_'+tag] = RX(flex)


def set_head(pose, rp, rt, yaw=0.0, pitch=0.0, roll=0.0, stab=.85):
    body = rp @ rt
    world = Matrix.Identity(3).to_quaternion().slerp(body.to_quaternion(), 1-stab).to_matrix() @ euler_rot(yaw, roll, pitch)
    pose.rot['head'] = body.transposed() @ world


def pose_idle(kind, i):
    w = 2*pi*i/48
    sway, breath = sin(w), sin(2*w)
    p = Pose()
    rp = euler_rot(.02*sin(w+1), .018*sway)
    rt = euler_rot(-.025*sin(w+1), -.012*sway, .02+.012*breath)
    solve_legs(p, Vector((.014*sway, 0, -.014+.003*breath)), rp, stand_feet())
    p.rot['torso'] = rt
    set_head(p, rp, rt, yaw=.24*sin(w)+.05*sin(2*w+1), pitch=.03*sin(w+.5)+.02, roll=.02*sin(w+2))
    for tag, s in SIDES:
        arm_fk(p, tag, s, .035*sin(w+(0 if s < 0 else pi))-.02, -s*.06, -.2-.04*breath-.03*sway*s)
    if kind == 'courier': p.loc['parcel'] = Vector()
    return p


def pose_walk(kind, i, carry=False):
    g = (i/24) % 1
    c1, s2 = cos(2*pi*g), sin(2*pi*(g+.03))
    p = Pose()
    rp = euler_rot(.06*c1, .05*s2)
    rt = euler_rot(-.11*c1, -.03*s2, .02 if carry else .07)
    feet = {}
    for (tag, s), off in zip(SIDES, (0, .5)):
        ph = (g+off) % 1
        f, z, psi = gait_foot(ph)
        swing = max(0.0, sin(pi*(ph-STANCE)/(1-STANCE))) if ph >= STANCE else 0.0
        feet[tag] = (s*HX+s*.018*swing, f, z, psi, s*.08)
    solve_legs(p, Vector((-.022*s2, 0, WALK_Z0-A_BOB*cos(4*pi*(g-.03)))), rp, feet)
    p.rot['torso'] = rt
    set_head(p, rp, rt, pitch=-.02 if not carry else .04)
    if carry:
        bob = Vector((0, 0, .007*sin(4*pi*(g+.1))))
        for tag, s in SIDES:
            arm_ik(p, tag, s, Vector((s*.2, PARCEL_C[1]+.07, PARCEL_C[2]-.17))+bob, rt)
        p.loc['parcel'] = bob
    else:
        ca = cos(2*pi*(g-.04))
        for tag, s in SIDES:
            theta = (.42 if s < 0 else -.42)*ca
            fwd = -theta/.42
            arm_fk(p, tag, s, theta, -s*.05, -(.28+.3*.5*(1+fwd)))
    return p


def pose_turn(kind, i):
    g = (i/24) % 1
    w = 2*pi*i/48
    s2 = sin(2*pi*(g+.03))
    p = Pose()
    rp = euler_rot(.1*sin(w), .04*s2)
    rt = euler_rot(-.2*sin(w+.3), -.02*s2, .03)
    feet = {}
    for (tag, s), off in zip(SIDES, (0, .5)):
        ph = (g+off) % 1
        lift = sin(pi*ph/.5)**2 if ph < .5 else 0.0
        feet[tag] = (s*HX+s*.02*lift, .05*lift-.01, ANK_Z+.055*lift, -.18*lift, s*.08+(.22 if tag == 'l' else -.22)*lift*sin(w))
    solve_legs(p, Vector((-.02*s2, 0, -.022+.006*cos(4*pi*g))), rp, feet)
    p.rot['torso'] = rt
    set_head(p, rp, rt, yaw=.5*sin(w+.9), pitch=.03, stab=.9)
    for tag, s in SIDES:
        arm_fk(p, tag, s, .12*(-s)*sin(2*pi*g), -s*.07, -.3-.05*sin(w))
    if kind == 'courier': p.loc['parcel'] = Vector()
    return p


def pose_greet(kind, i):
    t = i/48
    w = 2*pi*t
    raised = smoothstep(t/.2)*(1-smoothstep((t-.8)/.2))
    p = Pose()
    rp = euler_rot(.0, .012*sin(w))
    rt = euler_rot(.14*raised, 0, .02-.04*raised)
    solve_legs(p, Vector((.004*sin(w), 0, -.014+.003*sin(2*w))), rp, stand_feet())
    p.rot['torso'] = rt
    set_head(p, rp, rt, yaw=.22*raised, pitch=-.06*raised, roll=-.2*raised, stab=.95)
    arm_fk(p, 'l', -1, .03*sin(w), .06, -.22+.03*sin(w+1))
    wave = raised*.3*sin(2*pi*3*t)
    arm_fk(p, 'r', 1, -1.45*raised, -.05-.2*raised, -.25-1.6*raised, wave)
    if kind == 'courier': p.loc['parcel'] = Vector()
    return p


def pose_work(kind, i):
    w = 2*pi*i/48
    p = Pose()
    rp = euler_rot(.03*sin(2*w), .02*sin(2*w))
    rt = euler_rot(-.04*sin(2*w), 0, .13+.015*sin(2*w))
    solve_legs(p, Vector((.006*sin(2*w), 0, -.03)), rp, stand_feet())
    p.rot['torso'] = rt
    set_head(p, rp, rt, yaw=.06*sin(w), pitch=.34, stab=.55)
    for tag, s in SIDES:
        phase = 2*w+(0 if s < 0 else pi)
        arm_ik(p, tag, s, (s*.185, -.37+.05*sin(phase), 1.3+.035*cos(phase)), rt)
    if kind == 'courier': p.loc['parcel'] = Vector()
    return p


POSE_FUNCTIONS = {'idle': pose_idle, 'walk': pose_walk, 'turn': pose_turn, 'greet': pose_greet,
                  'work': pose_work, 'carry': lambda kind, i: pose_walk(kind, i, True)}
_pose_cache = {}


def clip_poses(kind, clip):
    """49 poses (frames 0..48 = 0..2 s); the last frame repeats the first so the loop is seamless."""
    key = (kind, clip)
    if key not in _pose_cache:
        _pose_cache[key] = [POSE_FUNCTIONS[clip](kind, frame % 48) for frame in range(0, 49)]
    return _pose_cache[key]


def animate_robot(root, kind):
    """Keyframe every joint in every clip (frames 0..48 so the exported clip runs 0..2 s exactly);
    one action, one muted NLA strip per object and clip."""
    objects = {o['component']: o for o in root.children_recursive}
    asset = root['asset_id']
    animated = JOINTS+(['parcel'] if kind == 'courier' else [])
    rest = {name: objects[name].location.copy() for name in animated}
    for name in animated: objects[name].animation_data_clear()
    for clip in CLIPS:
        poses = clip_poses(kind, clip)
        for name in animated:
            obj = objects[name]
            previous = None
            for frame in range(0, 49):
                pose = poses[frame]
                rotation = pose.rot.get(name, Matrix.Identity(3))
                euler = rotation.to_euler('XYZ', previous) if previous else rotation.to_euler('XYZ')
                previous = euler
                obj.rotation_euler = euler
                obj.location = rest[name]+pose.loc.get(name, Vector())
                obj.keyframe_insert(data_path='rotation_euler', frame=frame)
                obj.keyframe_insert(data_path='location', frame=frame)
            action = obj.animation_data.action
            action.name = f'{asset}.{name}.{clip}'
            track = obj.animation_data.nla_tracks.new()
            track.name = clip
            track.strips.new(clip, 0, action)
            track.mute = True
            obj.animation_data.action = None
            obj.rotation_euler = (0, 0, 0)
            obj.location = rest[name]
    return list(CLIPS)
