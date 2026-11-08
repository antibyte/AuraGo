"""Original three-dimensional table toys; no downloaded model payloads."""
import math
import random
import bpy
from geometry import box, cylinder, sphere, rod, torus, mesh, bv


def city():
    for i,x in enumerate([-.245,-.19,-.135,.135,.19,.245]):
        h = [.20,.29,.23,.24,.31,.19][i]
        z = 1.15 + (i%2)*.045
        box('tower-core', (x,h/2+.015,z),(.041,h,.045),'Obsidian',bevel=.003)
        box('tower-crown', (x,h+.02,z),(.026,.012,.03),'Chrome')
        for dx in [-.021,.021]:
            rod('tower-spine',(x+dx,.02,z-.022),(x+dx,h+.025,z-.022),.0015,'Accent' if i%2 else 'Secondary')
        for row in range(int(h/.014)):
            for col in [-1,0,1]:
                if (row+col+i)%4:
                    box('window',(x+col*.011,.03+row*.014,z-.023),(.006,.004,.001),'Accent' if row%3 else 'Secondary',bevel=0)
        rod('antenna',(x,h+.02,z),(x,h+.07,z),.0008,'Chrome')
    for level in range(5):
        h=.075+level*.045
        box('central-tower',(0,h,1.205),(.075-level*.01,.045,.065-level*.008),'Obsidian',bevel=.003)
        for x in [-1,1]:
            rod('crystal-edge',(x*(.037-level*.005),h-.022,1.17+level*.004),(0,h+.075,1.205),.002,'Accent')
    for z in [1.13,1.17]:
        torus('antenna-halo',(0,.23,z),.057,.0015,'Secondary')
    cylinder('reactor-base',(0,.025,1.165),.06,.035,'Gold')
    sphere('reactor',(0,.055,1.165),(.028,.025,.028),'Accent')


def sea():
    rng=random.Random(811)
    for radius in [.031,.039]: torus('relic-ring',(0,.003,.76),radius,.002,'Gold')
    sphere('relic-orb',(0,.088,.76),(.020,.022,.020),'Accent')
    for side in [-1,1]:
        rod('relic-support',(side*.044,.008,.76),(side*.022,.08,.76),.002,'Gold')
    for side in [-1,1]:
        for i in range(4):
            x=side*(.12+.034*(i%2)); z=.99+.055*i
            cylinder('ruin-plinth',(x,.008,z),.025,.016,'Stone')
            cylinder('ruin-column',(x,.06,z),.012,.10,'Stone',vertices=20)
            for j in range(7):
                angle=j*math.tau/7
                rod('column-flute',(x+math.cos(angle)*.012,.015,z+math.sin(angle)*.012),(x+math.cos(angle)*.012,.11,z+math.sin(angle)*.012),.0015,'Gold')
            box('ruin-cap',(x,.12,z),(.037,.014,.034),'Stone')
        for i in range(18):
            x=side*(.245+rng.uniform(-.012,.006)); z=.35+i*.043
            h=rng.uniform(.012,.047)
            rod('coral-stem',(x,.008,z),(x+side*.006,h,z),.0023,'Accent' if i%3 else 'Secondary')
            for branch in [-1,1]:
                rod('coral-branch',(x+side*.003,h*.55,z),(x+side*.006+branch*.009,h*.9,z+.006),.0015,'Accent')
    creature_start=set(bpy.context.scene.objects)
    for i in range(14):
        t=i/13
        p=(-.16*(1-t),.035+.13*math.sin(t*math.pi*.7),1.20+.018*math.sin(t*math.pi))
        r=.022+.022*t
        sphere('leviathan-neck',p,(r,r*.85,r*.9),'Creature')
        for j in range(3):
            a=(j-1)*.7
            start=(p[0]+math.sin(a)*r,p[1]+math.cos(a)*r,p[2])
            end=(start[0]-.015,start[1]+.022,start[2]+.015)
            rod('neck-spine',start,end,.0018,'Gold')
    sphere('leviathan-cranium',(0,.151,1.13),(.064,.041,.067),'Creature',32)
    sphere('leviathan-upper-snout',(0,.136,1.073),(.052,.021,.046),'Creature',32)
    sphere('mouth-shadow',(0,.111,1.055),(.041,.020,.025),'Obsidian')
    for side in [-1,1]:
        sphere('eye-socket',(side*.046,.153,1.082),(.018,.014,.016),'Gold')
        sphere('eye',(side*.049,.154,1.070),(.010,.007,.007),'Accent')
        rod('eyebrow',(side*.025,.170,1.065),(side*.065,.170,1.104),.003,'Gold')
        for i in range(7):
            x=side*(.009+i*.006)
            rod('upper-fang',(x,.124,1.047+abs(x)*.30),(x*.92,.103,1.041+abs(x)*.3),.0016,'Ivory')
        for i in range(7):
            rod('crown-spine',(side*(.013+i*.006),.174,1.10+i*.009),(side*(.026+i*.009),.210+i*.003,1.145+i*.012),.002,'Gold')
        for i in range(6):
            for j in range(4):
                x=side*(.012+i*.007); z=1.088+j*.013
                sphere('scale',(x,.176-i*.002,z),(.007,.002,.009),'Gold' if j%3==0 else 'Creature',12)
    jaw=box('hero-jaw',(0,.087,1.085),(.082,.012,.065),'Creature',bevel=.006,dynamic=True)
    for i in range(10):
        x=(i-4.5)*.007
        tooth=rod('lower-fang',(x,.094,1.056),(x,.105,1.052),.0014,'Ivory')
        matrix=tooth.matrix_world.copy(); tooth.parent=jaw; tooth.matrix_world=matrix
    jaw.rotation_euler.x=0; jaw.keyframe_insert(data_path='rotation_euler',frame=1)
    jaw.rotation_euler.x=.42; jaw.keyframe_insert(data_path='rotation_euler',frame=25)
    jaw.rotation_euler.x=0
    for obj in set(bpy.context.scene.objects)-creature_start:
        if not obj.parent: obj.location.z += .09


def temple():
    rng=random.Random(45)
    for side in [-1,1]:
        for i in range(3):
            x=side*(.17+.035*(i%2));z=1.02+i*.07
            for level in range(5):
                box('temple-masonry',(x,.012+level*.025,z),(.058,.023,.049),'Stone',bevel=.002)
                box('carved-band',(x,.017+level*.025,z-.026),(.049,.005,.002),'Gold',bevel=0)
            cylinder('torch-bowl',(x,.15,z),.018,.008,'Gold')
            sphere('torch',(x,.169,z),(.008,.024,.008),'Secondary',16)
        for i in range(8):
            x=side*.248;z=.50+i*.08
            for j in range(4):
                a=j*math.tau/4+rng.random()*.2
                vertices=[(x,.014,z),(x+math.cos(a)*.026,.025,z+math.sin(a)*.026),(x+math.cos(a+.2)*.015,.04,z+math.sin(a+.2)*.015)]
                mesh('palm-leaf',vertices,[(0,1,2)],'Leaf')
    # The idol stands behind the playable pocket, leaving a real opening below it.
    sphere('idol-face',(0,.20,1.20),(.075,.080,.020),'Gold',32)
    torus('idol-rim',(0,.20,1.20),.105,.009,'Gold',True)
    for i in range(20):
        a=i*math.tau/20
        rod('sun-ray',(math.cos(a)*.10,.20+math.sin(a)*.10,1.20),(math.cos(a)*.145,.20+math.sin(a)*.145,1.20),.008 if i%2 else .006,'Gold')
        box('glyph',(math.cos(a)*.091,.20+math.sin(a)*.091,1.174),(.008,.009,.004),'Obsidian')
    for side in [-1,1]:
        sphere('idol-eye',(side*.029,.222,1.178),(.017,.008,.009),'Secondary')
        rod('idol-brow',(side*.010,.239,1.175),(side*.050,.232,1.175),.006,'Gold')
    sphere('idol-nose',(0,.202,1.172),(.012,.022,.017),'Gold')
    sphere('idol-mouth',(0,.165,1.177),(.033,.027,.008),'Obsidian')
    for side, name in [(-1,'left'),(1,'right')]:
        door=box('temple-door-'+name,(side*.016,.165,1.163),(.030,.047,.005),'Gold',bevel=.002,dynamic=True)
        door['pinball_stage']='qualified-targets'
        door.keyframe_insert(data_path='location',frame=1)
        door.location.x=side*.052;door.keyframe_insert(data_path='location',frame=25)
        door.location.x=side*.016
    for i in range(6):
        box('idol-teeth',((i-2.5)*.009,.184,1.165),(.006,.012,.006),'Ivory',bevel=.001)
    for i in range(9):
        box('temple-stair',(0,.009+i*.006,1.15+i*.013),(.10,.012,.02),'Stone')
    # Decorative water stays outside the ball paths.
    water=box('waterfall',(.237,.078,.99),(.025,.14,.005),'Water',bevel=0,dynamic=True)
    water['pinball_water']=True
    for i in range(8):
        rod('water-streak',(.225+i*.003,.01,.987),(.225+i*.003,.15,.987),.0006,'Accent')


def build(table):
    {'neon':city,'leviathan':sea,'temple':temple}[table]()
