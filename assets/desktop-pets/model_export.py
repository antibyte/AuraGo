"""Shared compact GLB export and transparent poster for authored desktop rigs."""
import bpy
import hashlib
import json
from mathutils import Vector

def export_model(root, pet, source, rig, mesh):
    out=root/f'ui/3d/pets/{pet}'
    scratch=root/f'disposable/_{pet}-pet/textures'
    out.mkdir(parents=True,exist_ok=True);scratch.mkdir(parents=True,exist_ok=True)
    catalog=json.loads((root/'assets/desktop-pets/catalog.json').read_text(encoding='utf-8'))
    assert len(rig.data.bones)==catalog[pet]['joints']
    assert sum(b.parent is None for b in rig.data.bones)==1
    if catalog[pet]['kind']=='humanoid':
        assert rig.data.bones['DEF-thumb.01.L'].parent.name=='DEF-hand.L'
    for image in bpy.data.images:
        if image.type=='IMAGE':
            image.pack()
    bpy.context.preferences.filepaths.save_version=0
    bpy.ops.wm.save_as_mainfile(filepath=str(root/f'assets/desktop-pets/{pet}/source.blend'))
    bpy.ops.object.select_all(action='DESELECT')
    mesh.select_set(True);bpy.context.view_layer.objects.active=mesh
    modifier=mesh.modifiers.new('Desktop LOD','DECIMATE')
    modifier.ratio=50000/sum(len(p.vertices)-2 for p in mesh.data.polygons)
    bpy.ops.object.modifier_move_up(modifier=modifier.name)
    bpy.ops.object.modifier_apply(modifier=modifier.name)
    triangles=sum(len(p.vertices)-2 for p in mesh.data.polygons)
    assert 40000<triangles<=60000,triangles
    for vertex in mesh.data.vertices:
        weights=sorted([(g.group,g.weight) for g in vertex.groups if g.weight>0],key=lambda item:item[1],reverse=True)[:4]
        total=sum(weight for _,weight in weights)
        assert total>0,vertex.index
        for group in list(vertex.groups):
            mesh.vertex_groups[group.group].remove([vertex.index])
        for group,weight in weights:
            mesh.vertex_groups[group].add([vertex.index],weight/total,'REPLACE')
    textures=[]
    for image in bpy.data.images:
        if image.type!='IMAGE':
            continue
        if not image.has_data:
            image.pixels[0]
        assert image.has_data
        image.unpack(method='REMOVE')
        if max(image.size)>1024:
            image.scale(1024,1024)
        target=scratch/f'{len(textures):02}.png'
        image.filepath_raw,image.file_format=str(target),'PNG'
        image.save();textures.append(target)
    rig.select_set(True)
    bpy.ops.export_scene.gltf(filepath=str(out/'model.glb'),export_format='GLB',use_selection=True,export_skins=True,export_animations=False,export_cameras=False,export_lights=False)
    assert (out/'model.glb').stat().st_size<8*1024*1024
    report={'source_sha256':hashlib.sha256(source.read_bytes()).hexdigest(),'triangles':triangles,'vertices':len(mesh.data.vertices),'deform_joints':len(rig.data.bones),'texture_max_size':1024,'model_sha256':hashlib.sha256((out/'model.glb').read_bytes()).hexdigest(),'kind':catalog[pet]['kind']}
    (out/'model.json').write_text(json.dumps(report,indent=2)+'\n',encoding='utf-8',newline='\n')
    print('PET_MODEL_READY='+json.dumps({'pet':pet,**report}),flush=True)
    return textures

def render_thumbnail(root, pet, mesh, textures):
    scene=bpy.context.scene
    scene.render.engine='CYCLES';scene.cycles.samples=24
    scene.world=bpy.data.worlds.new('Studio');scene.world.use_nodes=True
    scene.world.node_tree.nodes['Background'].inputs['Color'].default_value=(.12,.13,.16,1)
    scene.world.node_tree.nodes['Background'].inputs['Strength'].default_value=.7
    center=Vector((0,0,max(v.co.z for v in mesh.data.vertices)*.5))
    for name,location,power in [('Key',(-2,-3,4),400),('Fill',(2,-2,2),180),('Rim',(1,2,3),300)]:
        data=bpy.data.lights.new(name,'AREA');data.energy,data.size=power,3
        obj=bpy.data.objects.new(name,data);scene.collection.objects.link(obj);obj.location=location
        obj.rotation_euler=(center-obj.location).to_track_quat('-Z','Y').to_euler()
    data=bpy.data.cameras.new('Preview');camera=bpy.data.objects.new('Preview',data)
    scene.collection.objects.link(camera);scene.camera=camera
    data.type,data.ortho_scale='ORTHO',2.0
    camera.location=center+Vector((.25,-4,0))
    camera.rotation_euler=(center-camera.location).to_track_quat('-Z','Y').to_euler()
    scene.render.film_transparent=True;scene.render.image_settings.color_mode='RGBA'
    scene.render.resolution_x,scene.render.resolution_y=384,416
    scene.render.resolution_percentage=100
    scene.render.filepath=str(root/f'ui/3d/pets/{pet}/thumbnail.png')
    bpy.ops.render.render(write_still=True)
    for texture in textures:
        texture.unlink()
