"""Replace the fixed VARCO vampire's human hand weights with continuous tentacle skinning."""
import bpy
import json
import numpy as np
from mathutils import Vector, Quaternion

REST_CURL = [.68, .15, -.08, -.15, -.09, -.03, -.02, 0]


def rig_tentacles(root, rig, mesh):
    # These landmarks follow the retained source's centerline, including its curled tips.
    points = np.array([(.085, 0, .530), (.170, 0, .520), (.260, 0, .514),
                       (.350, 0, .506), (.430, .002, .497), (.478, .004, .521),
                       (.474, .005, .555), (.447, .005, .566), (.439, .005, .546)])
    vertices = np.array([v.co[:] for v in mesh.data.vertices])
    mirrored = vertices.copy(); mirrored[:, 0] = abs(mirrored[:, 0])
    delta = points[1:]-points[:-1]
    lengths = np.linalg.norm(delta, axis=1)
    along = np.clip(np.sum((mirrored[:, None, :]-points[:-1])*delta, axis=2)/(lengths**2), 0, 1)
    distance = np.linalg.norm(mirrored[:, None, :]-points[:-1]-along[:, :, None]*delta, axis=2)
    nearest = distance.argmin(axis=1)
    travel = np.r_[0, np.cumsum(lengths)]
    arc = travel[nearest]+along[np.arange(len(vertices)), nearest]*lengths[nearest]
    centers = (travel[:-1]+travel[1:])*.5
    mask = (abs(vertices[:, 0]) > .085) & (vertices[:, 2] > .40) & (vertices[:, 2] < .61)
    mask &= (distance.min(axis=1) < .085) & ~((abs(vertices[:, 0]) < .20) & (vertices[:, 2] > .56))
    assert mask.sum() > 7000, int(mask.sum())
    bpy.context.view_layer.objects.active = rig
    bpy.ops.object.mode_set(mode='EDIT')
    for side, sign in [('L', 1), ('R', -1)]:
        for i in range(8):
            bone = rig.data.edit_bones.new(f'Tentacle.{i+1:02}.{side}')
            bone.head = Vector(points[i]*[sign, 1, 1]); bone.tail = Vector(points[i+1]*[sign, 1, 1])
            bone.parent = rig.data.edit_bones[f'Tentacle.{i:02}.{side}'] if i else rig.data.edit_bones['Spine2']
            # The shared export scales heads individually; connected edit bones would scale a joint twice.
            bone.use_connect = False
    bpy.ops.object.mode_set(mode='OBJECT')
    for side in ['L', 'R']:
        for i in range(8): mesh.vertex_groups.new(name=f'Tentacle.{i+1:02}.{side}')
    human_arms = {g.index for g in mesh.vertex_groups if any(word in g.name for word in ('Shoulder', 'Arm', 'Hand', 'Finger'))}
    for vertex in mesh.data.vertices:
        index = vertex.index
        if mask[index]:
            for group in list(vertex.groups): mesh.vertex_groups[group.group].remove([index])
            side = 'L' if vertex.co.x > 0 else 'R'
            blend = min(1, max(0, (abs(vertex.co.x)-.085)/.060))
            blend = blend*blend*(3-2*blend)
            if blend < 1: mesh.vertex_groups['Spine2'].add([index], 1-blend, 'REPLACE')
            upper = min(7, max(1, int(np.searchsorted(centers, arc[index]))))
            lower = upper-1
            mix = min(1, max(0, (arc[index]-centers[lower])/(centers[upper]-centers[lower])))
            for bone_index, weight in [(lower, 1-mix), (upper, mix)]:
                if weight*blend > 0: mesh.vertex_groups[f'Tentacle.{bone_index+1:02}.{side}'].add([index], weight*blend, 'REPLACE')
        else:
            # Cape, collar and face must not inherit the unused human arm/finger motion.
            displaced = sum(g.weight for g in vertex.groups if g.group in human_arms)
            if displaced:
                for group in list(vertex.groups):
                    if group.group in human_arms: mesh.vertex_groups[group.group].remove([index])
                name = 'Head' if vertex.co.z > .60 else 'Spine2' if vertex.co.z > .43 else 'Spine1' if vertex.co.z > .37 else 'Hips'
                mesh.vertex_groups[name].add([index], displaced, 'ADD')
            if vertex.co.z > .54:
                # A continuous neck blend avoids the source rig's isolated skull/chest islands.
                blend = min(1, max(0, (vertex.co.z-.54)/.08))
                blend = blend*blend*(3-2*blend)
                for group in list(vertex.groups): mesh.vertex_groups[group.group].remove([index])
                if blend < 1: mesh.vertex_groups['Spine2'].add([index], 1-blend, 'REPLACE')
                if blend > 0: mesh.vertex_groups['Head'].add([index], blend, 'REPLACE')

    data = {'version': 1, 'space': 'retained VARCO source, Z up', 'points_left': points.tolist(), 'segments_per_side': 8,
            'tentacle_vertices': int(mask.sum()), 'body_motion': 'retained VARCO humanoid joints',
            'rest_curl': REST_CURL, 'weights': 'arc-length interpolation between adjacent segment centers; chest blend at roots'}
    (root/'assets/desktop-pets/vampire/rig.json').write_text(json.dumps(data, indent=2)+'\n', encoding='utf-8', newline='\n')
    print('VAMPIRE_TENTACLE_VERTICES='+str(mask.sum()), flush=True)


def pose_tentacles(rig):
    for side, sign in [('L', 1), ('R', -1)]:
        for i, angle in enumerate(REST_CURL):
            bone = rig.pose.bones[f'Tentacle.{i+1:02}.{side}']
            rest = bone.bone.matrix_local.to_quaternion()
            bone.rotation_mode = 'QUATERNION'
            bone.rotation_quaternion = rest.inverted() @ Quaternion((0, 1, 0), sign*angle) @ rest
