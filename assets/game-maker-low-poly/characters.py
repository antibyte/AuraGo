"""Skinned original characters and baked, in-place gameplay animation (MIT)."""
from math import sin, cos, pi, tau, sqrt
import bpy
from mathutils import Vector, Matrix
from geometry import Mesh, xyz
from catalog import HUMAN_ACTIONS, ANIMAL_ACTIONS, FPS_ACTIONS


def humanoid_bones():
    bones = [
        ('pelvis', (0,.94,0), (0,1.09,0), None),
        ('spine', (0,1.09,0), (0,1.29,0), 'pelvis'),
        ('chest', (0,1.29,0), (0,1.45,0), 'spine'),
        ('neck', (0,1.45,0), (0,1.53,0), 'chest'),
        ('head', (0,1.53,0), (0,1.77,0), 'neck'),
    ]
    for side, s in [('L',-1),('R',1)]:
        bones += [
            ('upper_arm_'+side, (s*.255,1.415,0),(s*.31,1.15,0),'chest'),
            ('forearm_'+side,(s*.31,1.15,0),(s*.33,.93,.025),'upper_arm_'+side),
            ('hand_'+side,(s*.33,.93,.025),(s*.33,.83,.025),'forearm_'+side),
            ('thigh_'+side,(s*.108,.98,0),(s*.115,.55,.015),'pelvis'),
            ('shin_'+side,(s*.115,.55,.015),(s*.115,.12,0),'thigh_'+side),
            ('foot_'+side,(s*.115,.12,0),(s*.115,.07,.16),'shin_'+side),
        ]
        for finger in range(5):
            x=s*(.294+finger*.016)
            y=.865 if finger else .905
            z=.034 if finger else .057
            end_y=y-(.045 if finger in (0,4) else .06)
            bones.append((f'finger_{finger}_{side}',(x,y,z),(x,end_y,z),'hand_'+side))
            bones.append((f'tip_{finger}_{side}',(x,end_y,z),(x,end_y-.035,z),f'finger_{finger}_{side}'))
    return bones


def armature(name, bones, collection):
    data=bpy.data.armatures.new(name)
    rig=bpy.data.objects.new(name,data)
    collection.objects.link(rig)
    bpy.context.view_layer.objects.active=rig
    rig.select_set(True)
    bpy.ops.object.mode_set(mode='EDIT')
    for name, a, b, parent in bones:
        bone=data.edit_bones.new(name);bone.head=xyz(a);bone.tail=xyz(b)
        if parent:bone.parent=data.edit_bones[parent]
    bpy.ops.object.mode_set(mode='OBJECT')
    rig.select_set(False)
    return rig


def limb(g, a, b, radius_a, radius_b, paint, bone, sides=10):
    g.bone=bone
    g.cylinder(a,b,radius_a,paint,radius_b,sides=sides)


def humanoid(entry):
    g=Mesh(entry['id']); role=entry['design'].rsplit('-',1)[0]; alt=entry['design'].endswith('b')
    skin='skin_dark' if entry['index']%3==1 else 'skin'
    coat={'civilian':'red','mechanic':'blue','pilot':'ivory','explorer':'green','security':'navy','trooper':'ivory'}[role]
    pants={'civilian':'navy','mechanic':'blue','pilot':'navy','explorer':'sand','security':'ink','trooper':'ink'}[role]
    width=.19 if alt else .21
    bones=humanoid_bones(); lookup={n:(a,b) for n,a,b,p in bones}
    g.bone='pelvis'
    g.ellipsoid((0,.99,0),(width,.16,.115),pants,12,7)
    g.box((0,1.08,.005),(width*1.96,.045,.22),'ink',.015)
    g.box((0,1.08,.121),(.065,.043,.018),'silver',.006,material=1)
    g.bone='spine'
    g.ellipsoid((0,1.205,-.008),(width*.88,.18,.117),coat,12,8)
    g.bone='chest'
    g.ellipsoid((0,1.345,0),(width*1.18,.145,.14),coat,12,8)
    for s in (-1,1):
        g.box((s*.09,1.36,.133),(.095,.07,.014),coat,.012)
        g.beam((s*.04,1.43,.102),(s*.015,1.32,.145),.026,'ivory')
    g.bone='neck'
    g.cylinder((0,1.445,0),(0,1.55,0),.058,skin,sides=10)
    g.bone='head'
    g.ellipsoid((0,1.657,.012),(.113,.145,.105),skin,16,10)
    g.ellipsoid((0,1.582,.052),(.071,.068,.075),skin,12,6)
    g.ellipsoid((0,1.661,.111),(.027,.037,.026),skin,8,5)
    for s in (-1,1):
        g.ellipsoid((s*.113,1.657,.005),(.021,.037,.027),skin,8,5)
        g.box((s*.043,1.687,.108),(.036,.018,.01),'white',.004)
        g.box((s*.043,1.685,.116),(.015,.014,.008),'ink',.003)
        g.beam((s*.026,1.707,.109),(s*.064,1.707,.1),.011,'hair')
    g.box((0,1.614,.117),(.043,.009,.006),'hair',.002)
    g.ellipsoid((0,1.735,-.017),(.114,.065,.105),'hair' if not alt else 'wood',12,6)
    if alt:
        g.ellipsoid((0,1.62,-.085),(.095,.14,.055),'wood',10,7)
        g.ellipsoid((0,1.687,-.139),(.058,.06,.065),'wood',8,6)
    for side,s in [('L',-1),('R',1)]:
        upper,fore,hand='upper_arm_'+side,'forearm_'+side,'hand_'+side
        a,b=lookup[upper];limb(g,a,b,.08,.06,coat,upper)
        g.ellipsoid(a,(.083,.077,.081),coat,10,6)
        g.ellipsoid(b,(.061,.065,.06),coat,10,6)
        a,b=lookup[fore];limb(g,a,b,.055,.035,skin if role in ('civilian','mechanic') else coat,fore)
        g.bone=hand
        g.box((s*.33,.885,.024),(.083,.105,.048),skin if role=='civilian' else 'ink',.018)
        for finger in range(5):
            for prefix in ('finger','tip'):
                key=f'{prefix}_{finger}_{side}';a,b=lookup[key]
                limb(g,a,b,.011,.008,skin if role=='civilian' else 'ink',key,6)
        thigh,shin,foot='thigh_'+side,'shin_'+side,'foot_'+side
        a,b=lookup[thigh];limb(g,a,b,.105,.074,pants,thigh,12)
        g.ellipsoid(b,(.076,.07,.073),pants,10,6)
        a,b=lookup[shin];limb(g,a,b,.071,.045,pants,shin,10)
        g.bone=foot
        g.box((s*.115,.065,.062),(.155,.12,.31),'ink',.035)
        g.box((s*.115,.017,.065),(.159,.03,.31),'rubber',.01)
        g.socket('hand_'+side,lookup[hand][0],hand)
    g.bone='chest'
    if role in ('explorer','mechanic'):
        for s in (-1,1):g.beam((s*.13,1.45,.06),(s*.1,1.1,.12),.025,'ink',depth=.014)
        g.box((0,1.31,-.18),(.29,.35,.17),'sand' if role=='explorer' else 'orange',.045)
    if role=='pilot':
        for s in (-1,1):g.box((s*.19,1.455,0),(.07,.025,.11),'yellow',.01)
        g.box((-.075,1.36,.15),(.065,.03,.012),'yellow',.004)
    if role in ('security','trooper'):
        g.box((0,1.32,.155),(.33,.23,.07),'ink' if role=='security' else 'ivory',.045)
        for s in (-1,1):g.box((s*.095,1.19,.155),(.077,.09,.05),'navy' if role=='security' else 'teal',.018)
    if role=='trooper':
        g.bone='head'
        g.ellipsoid((0,1.7,-.005),(.133,.125,.125),'ivory',12,8)
        g.box((0,1.673,.12),(.195,.057,.037),'glass',.02,material=2)
        g.box((0,1.677,.145),(.14,.009,.008),'cyan',.004,material=3)
        for side,s in [('L',-1),('R',1)]:
            g.bone='upper_arm_'+side;g.ellipsoid((s*.267,1.405,0),(.09,.09,.09),'ivory',10,6)
            g.bone='shin_'+side;g.box((s*.115,.53,.067),(.13,.14,.045),'ivory',.03)
    elif role in ('mechanic','security'):
        g.bone='head';g.ellipsoid((0,1.762,0),(.117,.045,.11),coat,12,5)
        g.box((0,1.75,.115),(.16,.018,.095),coat,.025)
    g.socket('back',(0,1.3,-.18),'chest');g.socket('head',(0,1.82,0),'head')
    g.bone=None
    return g,bones


def two_bone(a, target, l1, l2, bend):
    a,target=Vector(a),Vector(target)
    delta=target-a
    d=max(.001,min(delta.length,l1+l2-.0001))
    direction=delta.normalized()
    along=(l1*l1-l2*l2+d*d)/(2*d)
    height=sqrt(max(0,l1*l1-along*along))
    bend=Vector(bend);bend=(bend-direction*bend.dot(direction)).normalized()
    return a+direction*along+bend*height, a+direction*d


def human_pose(bones, action, t):
    phase=t*tau
    points={n:(Vector(a),Vector(b)) for n,a,b,p in bones}
    moving=action in ('walk','run','sprint','walk_back','strafe_left','strafe_right','crouch_walk','carry_walk','rifle_walk')
    crouch=action.startswith('crouch')
    sitting=action=='sit_idle' or action=='sit_down' and t>.85
    sit_amount=1 if sitting else (t if action=='sit_down' else 1-t if action=='stand_up' else 0)
    lower=.3 if crouch else .39*sit_amount
    if moving and not crouch:lower=max(lower,.15 if action=='sprint' else .09 if action=='run' else .05)
    if action=='pick_up':lower=.34*sin(pi*t)**2
    jump_y=.18*sin(pi*t) if action=='jump_start' else .2 if action=='jump_loop' else .2*(1-t) if action=='jump_land' else 0
    bounce=.012*(1-cos(phase*2)) if moving else .003*sin(phase)
    offset=Vector((0,-lower+jump_y+bounce,0))
    lean=.13 if action in ('run','sprint','crouch_walk','push') else .03
    for n,(a,b) in list(points.items()):
        if n in ('pelvis','spine','chest','neck','head'):
            points[n]=(a+offset+Vector((0,0,(a.y-.95)*lean)),b+offset+Vector((0,0,(b.y-.95)*lean)))
    for side,s in [('L',-1),('R',1)]:
        q=(t+(.5 if side=='R' else 0))%1
        speed=1.7 if action=='sprint' else 1.3 if action=='run' else 1
        stride=.26*speed
        step_z=stride*(1-2*q/.6) if q<.6 else -stride+2*stride*((q-.6)/.4)
        lift=0 if q<.6 else (.095*speed)*sin(pi*(q-.6)/.4)
        hip=Vector((s*.108,.98,0))+offset
        ankle=Vector((s*.115,.12+jump_y,0))
        if moving:
            if action.startswith('strafe'):
                ankle.x+=step_z*(-1 if action=='strafe_left' else 1)
            else:ankle.z+=step_z*(-1 if action=='walk_back' else 1)
            ankle.y+=lift
        if sit_amount:ankle.z+=.36*sit_amount
        if action=='climb':ankle.y+=.15+.19*(1+sin(phase+s*pi/2));ankle.z=.2
        knee,ankle=two_bone(hip,ankle,.43,.43,(0,0,1))
        points['thigh_'+side]=(hip,knee)
        points['shin_'+side]=(knee,ankle)
        points['foot_'+side]=(ankle,ankle+Vector((0,-.05,.16)))
        shoulder=Vector((s*.255,1.415,.05))+offset
        hand=Vector((s*.33,.93,.025))+offset
        if moving:hand.z+=sin(phase+(0 if side=='R' else pi))*.19*speed;hand.y+=.015
        rifle=action.startswith('rifle')
        if rifle:
            hand=Vector((s*.16,1.25 if side=='R' else 1.22,.34 if side=='R' else .57))+offset
            if action=='rifle_fire':hand.z-=.065*sin(pi*min(1,t*4))
            if action=='rifle_reload' and side=='L':hand.y-=.24*sin(pi*t)**2;hand.z-=.16*sin(pi*t)**2
        if action.startswith('carry') or action=='push':hand=Vector((s*.24,1.17,.39))+offset
        if action in ('wave','cheer') and (side=='R' or action=='cheer'):
            hand=Vector((s*(.4+.06*sin(phase*2)),1.77,.04))+offset
        if action in ('interact','punch') and side=='R':hand=Vector((s*.16,1.28,.18+.42*sin(pi*t)**2))+offset
        if action=='pick_up':hand=Vector((s*.12,.72-lower,.32))
        if action=='climb':hand=Vector((s*.24,1.62+.18*sin(phase+s*pi/2),.3))+offset
        elbow,hand=two_bone(shoulder,hand,.271,.222,(s*.35,0,-1))
        points['upper_arm_'+side]=(shoulder,elbow)
        points['forearm_'+side]=(elbow,hand)
        tip=hand+Vector((0,0,.1) if rifle or action in ('push','punch','interact') else (0,-.1,0))
        points['hand_'+side]=(hand,tip)
    if action in ('death','hit','turn_left','turn_right','fall'):
        angle=(pi*.48*min(1,t*1.5) if action=='death' else -.16*sin(pi*t) if action=='hit' else .22 if action=='fall' else 0)
        yaw=(-1 if action=='turn_left' else 1)*pi/2*t if action.startswith('turn') else 0
        rot=Matrix.Rotation(angle,3,'X')@Matrix.Rotation(yaw,3,'Y')
        origin=Vector((0,.1,0))
        for n,(a,b) in list(points.items()):
            if not n.startswith(('finger','tip')):points[n]=(rot@(a-origin)+origin,rot@(b-origin)+origin)
    return points


def pose_matrices(rig, bones, points):
    desired={}
    for name,a,b,parent in bones:
        bone=rig.data.bones[name]
        rest=bone.matrix_local
        if name not in points or name.startswith(('finger','tip')):
            mat=(desired[parent] @ rig.data.bones[parent].matrix_local.inverted() @ rest) if parent else rest.copy()
        else:
            head,tail=map(xyz,points[name])
            rest_direction=bone.tail_local-bone.head_local
            rotation=rest_direction.rotation_difference(tail-head).to_matrix().to_4x4()
            mat=Matrix.Translation(head)@rotation@rest.to_3x3().to_4x4()
        desired[name]=mat
        basis=rest.inverted()@((rig.data.bones[parent].matrix_local@desired[parent].inverted()) if parent else Matrix.Identity(4))@mat
        pb=rig.pose.bones[name]
        pb.rotation_mode='QUATERNION';pb.matrix_basis=basis


def animate(rig,bones,actions,pose_fn,prefix):
    metadata=[]
    rig.animation_data_create()
    for action in actions:
        base_action=action.removeprefix('pistol_')
        duration={'walk':1.05,'run':.72,'sprint':.57,'fire':.3,'rifle_fire':.3,
                  'death':1.3,'reload':1.8,'reload_empty':2.3,'rifle_reload':1.8,
                  'draw':.5,'holster':.45,'jump_start':.3,'jump_land':.3,
                  'aim_in':.22,'aim_out':.22,'punch':.6,'hit':.5}.get(base_action,1.6)
        end=max(2,round(duration*30))
        clip=bpy.data.actions.new(prefix+'__'+action)
        rig.animation_data.action=clip
        for frame in list(range(1,end+1,3))+([end] if (end-1)%3 else []):
            t=(frame-1)/(end-1)
            pose_matrices(rig,bones,pose_fn(bones,action,t))
            for pb in rig.pose.bones:
                pb.keyframe_insert('location',frame=frame)
                pb.keyframe_insert('rotation_quaternion',frame=frame)
        track=rig.animation_data.nla_tracks.new();track.name=action
        strip=track.strips.new(action,1,clip);strip.action_frame_start=1;strip.action_frame_end=end
        track.mute=True
        metadata.append(dict(id=action,duration=round((end-1)/30,4),
                             loop=base_action in ('idle','walk','run','sprint','walk_back','strafe_left','strafe_right','crouch_idle','crouch_walk','jump_loop','climb','fall','sit_idle','carry_idle','carry_walk','push','unarmed_idle','rifle_idle','rifle_walk','rest_idle','eat','fly','glide','aim_idle'),
                             speed={'walk':.825,'run':1.22,'sprint':1.93,'walk_back':.825,'crouch_walk':.62}.get(action,0)))
        if base_action in ('fire','rifle_fire'):metadata[-1]['events']=[dict(time=.04,name='shot')]
        if base_action in ('reload','reload_empty','rifle_reload'):
            metadata[-1]['events']=[dict(time=round(duration*.26,3),name='magazine_out'),dict(time=round(duration*.72,3),name='magazine_in')]
        if action=='grenade_throw':metadata[-1]['events']=[dict(time=round(duration*.55,3),name='release')]
    rig.animation_data.action=None
    for pb in rig.pose.bones:pb.matrix_basis=Matrix.Identity(4)
    return metadata
