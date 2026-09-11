"""Render the exported GLBs, not the authoring meshes. Blender CLI, MIT."""
from pathlib import Path
import argparse
import json
import sys
import bpy
from mathutils import Vector

ROOT=Path(__file__).resolve().parents[2]
OUT=ROOT/'internal/gamemaker/asset_packs/aurago-low-poly'


def look(obj,target):
    obj.rotation_euler=(Vector(target)-obj.location).to_track_quat('-Z','Y').to_euler()


def render(asset,size=384):
    # Hidden custom bone shapes are not selected by select_all and otherwise
    # contaminate every later model's bounds. This process owns the whole scene.
    for obj in list(bpy.data.objects):bpy.data.objects.remove(obj,do_unlink=True)
    bpy.ops.outliner.orphans_purge(do_recursive=True)
    scene=bpy.context.scene
    scene.render.engine='CYCLES';scene.cycles.samples=24;scene.cycles.use_denoising=True
    scene.cycles.max_bounces=3
    preferences=bpy.context.preferences.addons['cycles'].preferences
    try:
        preferences.compute_device_type='ONEAPI';preferences.get_devices()
        for device in preferences.devices:device.use=device.type=='ONEAPI'
        if any(d.use for d in preferences.devices):scene.cycles.device='GPU'
    except TypeError:pass
    scene.view_settings.view_transform='AgX'
    scene.view_settings.look='AgX - Medium High Contrast';scene.view_settings.exposure=-.5
    scene.render.resolution_x=scene.render.resolution_y=size
    scene.render.resolution_percentage=100
    scene.render.image_settings.file_format='WEBP';scene.render.image_settings.quality=86
    scene.render.film_transparent=False
    if not scene.world:scene.world=bpy.data.worlds.new('Studio')
    scene.world.use_nodes=True
    bg=scene.world.node_tree.nodes.get('Background')
    bg.inputs['Color'].default_value=(.19,.23,.26,1);bg.inputs['Strength'].default_value=.55
    bpy.ops.import_scene.gltf(filepath=str(OUT/asset['lods'][0]['file']))
    bpy.context.view_layer.update()
    # Blender's glTF importer creates a hidden Icosphere as a bone display shape.
    coordinates=[obj.matrix_world@Vector(p) for obj in scene.objects if obj.type=='MESH' and obj.name.startswith(asset['id']+'__') for p in obj.bound_box]
    low=Vector(tuple(min(v[i] for v in coordinates) for i in range(3)))
    high=Vector(tuple(max(v[i] for v in coordinates) for i in range(3)))
    center=(low+high)/2;extent=max(high-low)
    bpy.ops.mesh.primitive_plane_add(size=200*max(1,extent),location=(0,0,low.z-.025))
    ground=bpy.context.object;mat=bpy.data.materials.new('Backdrop');mat.diffuse_color=(.19,.23,.25,1)
    ground.data.materials.append(mat)
    for name,offset,power,color in [('Key',(-3,-4,6),550,(1,.87,.69)),('Fill',(4,-1,3),280,(.67,.83,1)),('Rim',(0,4,4),650,(1,.92,.74))]:
        light=bpy.data.lights.new(name,'AREA');light.energy=power*extent**2;light.color=color;light.shape='DISK';light.size=extent*3
        obj=bpy.data.objects.new(name,light);scene.collection.objects.link(obj);obj.location=center+Vector(offset)*extent;look(obj,center)
    camera=bpy.data.cameras.new('Camera');camera.type='ORTHO';camera.ortho_scale=extent*1.55
    obj=bpy.data.objects.new('Camera',camera);scene.collection.objects.link(obj)
    obj.location=center+Vector((1.3,-2.3,1.25))*extent;look(obj,center)
    scene.camera=obj
    target=OUT/asset['preview'];target.parent.mkdir(parents=True,exist_ok=True)
    scene.render.filepath=str(target)
    bpy.ops.render.render(write_still=True)
    print('POLY_PREVIEW',asset['id'],target.stat().st_size,flush=True)


if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--only');parser.add_argument('--size',type=int,default=384)
    options=parser.parse_args(sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else [])
    selected=set(options.only.split(',')) if options.only else None
    manifest=json.loads((OUT/'manifest.json').read_text())
    for asset in manifest['assets']:
        if not selected or asset['id'] in selected:
            render(asset,options.size)
            import hashlib
            data=(OUT/asset['preview']).read_bytes()
            asset['preview_file']=dict(file=asset['preview'],bytes=len(data),sha256=hashlib.sha256(data).hexdigest())
    (OUT/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n',encoding='utf-8')
