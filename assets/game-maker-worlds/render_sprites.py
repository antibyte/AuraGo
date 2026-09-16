"""Render native-resolution pixel frames from exported GLBs (Blender CLI)."""
from pathlib import Path
import argparse, json, sys, math
import bpy
from mathutils import Vector
HERE=Path(__file__).resolve().parent;ROOT=HERE.parents[1]
PACK=ROOT/'internal/gamemaker/asset_packs/aurago-pirates-3d'
RAW=HERE/'production/sprites'

def reset():
    for obj in list(bpy.data.objects):bpy.data.objects.remove(obj,do_unlink=True)
    bpy.ops.outliner.orphans_purge(do_recursive=True)

def activate(objects,clip):
    for obj in objects:
        ad=obj.animation_data
        if not ad:continue
        ad.action=None
        for track in ad.nla_tracks:
            track.mute=track.name!=clip

def render_asset(asset,view,actions=None,metadata_only=False):
    from bpy_extras.object_utils import world_to_camera_view
    reset();scene=bpy.context.scene;scene.render.fps=30
    bpy.ops.import_scene.gltf(filepath=str(PACK/asset['lods'][0]['file']))
    objects=list(scene.objects)
    if asset.get('animation_library'):
        before=set(scene.objects)
        bpy.ops.import_scene.gltf(filepath=str(PACK/asset['animation_library']['file']))
        lib=[o for o in scene.objects if o not in before]
        source=next(o for o in lib if o.type=='ARMATURE');target=next(o for o in objects if o.type=='ARMATURE')
        target.animation_data_create()
        for track in source.animation_data.nla_tracks:
            t=target.animation_data.nla_tracks.new();t.name=track.name
            for strip in track.strips:
                s=t.strips.new(strip.name,1,strip.action);s.action_slot=strip.action_slot
            t.mute=True
        for obj in lib:bpy.data.objects.remove(obj,do_unlink=True)
    scene.render.engine='BLENDER_WORKBENCH'
    shading=scene.display.shading;shading.light='STUDIO';shading.color_type='VERTEX'
    # Workbench screen-space cavity/shadow sampling produces stippled pixels at
    # native resolution. Flat facet lighting gives clean intentional clusters.
    shading.show_shadows=False;shading.show_cavity=False;shading.show_specular_highlight=False
    shading.show_object_outline=False;shading.background_type='WORLD'
    scene.display.render_aa='OFF';scene.render.film_transparent=True
    scene.view_settings.view_transform='Standard';scene.render.image_settings.file_format='PNG';scene.render.image_settings.color_mode='RGBA'
    iso=view=='isometric'
    size=(128 if asset['category']=='terrain' else 192 if asset['category']=='people' else 256) if iso else 256 if asset['category']=='ships' else 96
    scene.render.resolution_x=scene.render.resolution_y=size;scene.render.resolution_percentage=100
    lo,hi=asset['bounds']['min'],asset['bounds']['max'];extent=max(b-a for a,b in zip(lo,hi))
    center=Vector((0,0,(lo[1]+hi[1])/2))
    camera=bpy.data.cameras.new('PixelCamera');camera.type='ORTHO';camera.ortho_scale=extent*1.35
    if not iso:
        # Fit every authored pose in one fixed frame, including prone bodies and
        # swimming. Never fit individual frames: that makes animation pulsate.
        zmin,zmax=lo[1],hi[1];radius=0
        for clip in asset['animations'] or [dict(id='idle',duration=1)]:
            activate(objects,clip['id'])
            for t in (0,.25,.5,.75,1):
                scene.frame_set(1+int(t*clip['duration']*30));bpy.context.view_layer.update();deps=bpy.context.evaluated_depsgraph_get()
                for mesh in (o for o in objects if o.type=='MESH'):
                    evaluated=mesh.evaluated_get(deps)
                    for corner in evaluated.bound_box:
                        p=evaluated.matrix_world@Vector(corner);zmin=min(zmin,p.z);zmax=max(zmax,p.z);radius=max(radius,math.hypot(p.x,p.y))
        center.z=(zmin+zmax)/2;camera.ortho_scale=2*math.hypot(radius,(zmax-zmin)/2)*1.08
    if iso:
        # A two-metre ground cell projects to 128x64; one authoring height unit
        # projects to 32 pixels. Frames are not independently fitted to bounds.
        fit=min(1,4/max(hi[0]-lo[0],hi[2]-lo[2])) if asset['category']=='vehicles' else min(1,4/max(.01,hi[1]-lo[1])) if asset['category']=='nature' else 1
        for root in [o for o in objects if o.parent is None and o.name!='Icosphere']:
            root.scale*=fit;root.scale.z*=math.sqrt(2/3)
        camera.ortho_scale=size/(32*math.sqrt(2));center=Vector((0,0,0 if asset['category']=='terrain' else .9))
    obj=bpy.data.objects.new('PixelCamera',camera);scene.collection.objects.link(obj);scene.camera=obj
    count=(8 if asset['category'] in ('people','animals','vehicles') else 4 if asset['category'] in ('terrain','architecture','objects') else 1) if iso else 2 if view=='side' else 16 if asset['category']=='ships' else 8 if asset['category'] in ('people','animals','submarines') else 1
    directions=[dict(id=('right' if i==0 else 'left') if count==2 else ['right','down-right','down','down-left','left','up-left','up','up-right'][(i+(1 if iso else 0))%8] if count==8 else 'heading-'+str(i),radians=math.atan2(.5*math.sin(i*math.tau/count+math.pi/4),math.cos(i*math.tau/count+math.pi/4)) if iso else i*math.tau/count,yaw=i*math.tau/count) for i in range(count)]
    clips=[c for c in asset['animations'] if not actions or c['id'] in actions]
    if not clips:clips=[dict(id='idle',duration=1,loop=True,events=[])]
    manifest=dict(id=asset['id'],width=size,height=size,view=view,origin=dict(x=.5,y=.5),directions=[{k:v for k,v in d.items() if k!='yaw'} for d in directions],animations=[],source_glb=asset['lods'][0]['sha256'])
    layer_objects={l['id']:next(o for o in objects if o.name==l['node']) for l in asset.get('layers',[])}
    manifest['layers']={name:{} for name in layer_objects}
    socket_objects={s['id']:next(o for o in objects if o.name==s['node']) for s in asset.get('sockets',[])}
    manifest['sockets']={name:{} for name in socket_objects}
    for direction in directions:
        angle=direction['yaw'];theta=angle+math.pi+(math.pi/4 if iso else 0);elevation=math.pi/6 if iso else .10 if view=='side' else math.radians(62)
        obj.location=center+Vector((math.cos(theta)*math.cos(elevation),math.sin(theta)*math.cos(elevation),math.sin(elevation)))*extent*4
        obj.rotation_euler=(center-obj.location).to_track_quat('-Z','Y').to_euler()
        activate(objects,'idle');scene.frame_set(1);bpy.context.view_layer.update()
        for name,socket in socket_objects.items():
            point=world_to_camera_view(scene,obj,socket.matrix_world.translation)
            manifest['sockets'][name][direction['id']]={'x':round(point.x*size,4),'y':round((1-point.y)*size,4)}
        for name,layer in layer_objects.items():
            for mesh in (o for o in objects if o.type=='MESH'):mesh.hide_render=mesh!=layer
            activate(objects,'idle');scene.frame_set(1)
            path=RAW/view/asset['id']/direction['id']/('layer-'+name+'.png');path.parent.mkdir(parents=True,exist_ok=True)
            if not metadata_only:scene.render.filepath=str(path);bpy.ops.render.render(write_still=True)
            manifest['layers'][name][direction['id']]=path.relative_to(RAW).as_posix()
        for mesh in (o for o in objects if o.type=='MESH'):mesh.hide_render=mesh in layer_objects.values()
        for clip in clips:
            activate(objects,clip['id']);frames=(8 if clip['loop'] else 6) if asset['animations'] else 1
            paths=[]
            for i in range(frames):
                frame=1+(i/(frames if clip['loop'] else frames-1))*clip['duration']*30
                scene.frame_set(int(frame),subframe=frame-int(frame));bpy.context.view_layer.update()
                path=RAW/view/asset['id']/direction['id']/clip['id']/f'{i:02}.png';path.parent.mkdir(parents=True,exist_ok=True)
                if not metadata_only:scene.render.filepath=str(path);bpy.ops.render.render(write_still=True)
                paths.append(path.relative_to(RAW).as_posix())
            events=[dict(frame=min(frames-1,round(e['time']/clip['duration']*(frames-1))),name=e['name']) for e in clip.get('events',[])]
            manifest['animations'].append(dict(action=clip['id'],direction=direction['id'],frames=paths,frame_rate=max(1,round(frames/clip['duration'])),repeat=-1 if clip['loop'] else 0,events=events))
    # Project the documented ground/water anchor once; never trim per frame.
    from bpy_extras.object_utils import world_to_camera_view
    # Anchor stays at asset origin (waterline for ships, feet for humans).
    anchor=world_to_camera_view(scene,obj,Vector((0,0,0)))
    manifest['origin']=dict(x=round(anchor.x,5),y=round(1-anchor.y,5))
    out=RAW/view/asset['id']/'frames.json';out.write_text(json.dumps(manifest,separators=(',',':'))+'\n',encoding='utf-8')
    print('PIXEL_FRAMES',asset['id'],view,sum(len(c['frames']) for c in manifest['animations']),flush=True)

if __name__=='__main__':
    parser=argparse.ArgumentParser();parser.add_argument('--only');parser.add_argument('--views',default='top,side');parser.add_argument('--actions');parser.add_argument('--source');parser.add_argument('--metadata-only',action='store_true')
    args=parser.parse_args(sys.argv[sys.argv.index('--')+1:] if '--' in sys.argv else [])
    if args.source:PACK=Path(args.source).resolve()
    selected=set(args.only.split(',')) if args.only else None;actions=set(args.actions.split(',')) if args.actions else None
    for asset in json.loads((PACK/'manifest.json').read_text())['assets']:
        if selected and asset['id'] not in selected:continue
        for view in args.views.split(','):render_asset(asset,view,actions,args.metadata_only)
