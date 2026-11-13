"""Clean mechanical reconstruction of the tracked pet's pixel-derived surface."""
import bpy
from math import cos, sin, pi

def build_surface():
    parts=[]
    def material(name,color,metal=.0,rough=.4):
        mat=bpy.data.materials.new(name);mat.use_nodes=True
        shader=mat.node_tree.nodes.get('Principled BSDF')
        shader.inputs['Base Color'].default_value=(*color,1)
        shader.inputs['Metallic'].default_value=metal
        shader.inputs['Roughness'].default_value=rough
        return mat
    yellow=material('Ochre enamel',(.65,.32,.018),.48)
    trim=material('Worn yellow edges',(.92,.58,.065),.55)
    steel=material('Brushed steel',(.30,.32,.32),.78,.28)
    dark=material('Recessed graphite',(.038,.046,.048),.55)
    rubber=material('Rubber tracks',(.023,.027,.025),.0,.72)
    glass=material('Camera glass',(.012,.026,.035),.6,.12)
    glint=material('Lens reflection',(.32,.46,.50),.7,.16)
    led=material('Status green',(.16,.60,.10),.3)
    def finish(obj,mat,bone,bevel=0):
        bpy.context.view_layer.objects.active=obj
        bpy.ops.object.transform_apply(location=False,rotation=False,scale=True)
        if bevel:
            mod=obj.modifiers.new('Machined edges','BEVEL');mod.width=bevel;mod.segments=3
            bpy.ops.object.modifier_apply(modifier=mod.name)
        obj.data.materials.append(mat)
        obj.vertex_groups.new(name=bone).add(list(range(len(obj.data.vertices))),1,'REPLACE')
        parts.append(obj);return obj
    def box(position,size,mat,bone,bevel=.008):
        bpy.ops.mesh.primitive_cube_add(size=1,location=position)
        obj=bpy.context.object;obj.scale=size
        return finish(obj,mat,bone,bevel)
    def cylinder(position,radius,depth,mat,bone,axis='Z'):
        rotation={'Z':(0,0,0),'X':(0,pi/2,0),'Y':(pi/2,0,0)}[axis]
        bpy.ops.mesh.primitive_cylinder_add(vertices=48,radius=radius,depth=depth,location=position,rotation=rotation)
        return finish(bpy.context.object,mat,bone,.002)
    def ring(position,major,minor,mat,bone,axis='Y'):
        rotation=(pi/2,0,0) if axis=='Y' else (0,pi/2,0)
        bpy.ops.mesh.primitive_torus_add(major_radius=major,minor_radius=minor,major_segments=64,minor_segments=12,location=position,rotation=rotation)
        return finish(bpy.context.object,mat,bone)
    body='DEF-spine.002';head='DEF-spine.006'
    box((0,0,.415),(.49,.37,.37),yellow,body,.025)
    box((0,-.192,.42),(.425,.018,.29),trim,body)
    box((0,-.206,.48),(.22,.014,.075),dark,body,.003)
    box((-.065,-.216,.48),(.055,.008,.048),steel,body,.002)
    for z in [.465,.484,.503]:box((.025,-.217,z),(.055,.007,.009),led,body,.001)
    box((0,-.211,.345),(.23,.015,.045),steel,body,.004)
    box((0,0,.611),(.43,.32,.027),steel,body)
    for x in [-.19,.19]:
        for z in [.30,.54]:cylinder((x,-.211,z),.012,.009,steel,body,'Y')
    for x in [-.26,.26]:
        box((x,0,.415),(.025,.27,.27),trim,body)
        for z in [.36,.39,.42,.45,.48]:box((x*1.06,-.02,z),(.009,.16,.009),dark,body,.002)
    box((0,.015,.68),(.065,.07,.13),yellow,'DEF-spine.004')
    cylinder((0,0,.736),.040,.11,steel,'DEF-spine.004','X')
    box((0,0,.785),(.056,.065,.11),steel,head)
    cylinder((0,0,.84),.035,.12,steel,head,'X')
    for side,sign in [('L',1),('R',-1)]:
        tread='Tread.'+side
        box((sign*.327,.005,.145),(.12,.46,.23),dark,tread,.055)
        for y in [-.165,0,.165]:
            cylinder((sign*.40,y,.145),.095 if y else .07,.025,steel,tread,'X')
            cylinder((sign*.417,y,.145),.036,.009,dark,tread,'X')
            cylinder((sign*.425,y,.145),.014,.01,steel,tread,'X')
        for i in range(32):
            angle=2*pi*i/32;y=.22*cos(angle);z=.145+.14*sin(angle)
            block=box((sign*.34,y,z),(.17,.045,.032),rubber,tread,.004)
            block.rotation_euler.x=angle
            # The outside ribs read as tread blocks at desktop scale.
            rib=box((sign*.34,y*1.012,.145+(z-.145)*1.04),(.176,.018,.012),dark,tread,.002)
            rib.rotation_euler.x=angle
        upper='DEF-upper_arm.'+side;fore='DEF-forearm.'+side;hand='DEF-hand.'+side
        cylinder((sign*.265,0,.54),.035,.07,steel,upper,'X')
        box((sign*.31,0,.485),(.055,.065,.13),yellow,upper)
        cylinder((sign*.33,0,.425),.032,.07,steel,fore,'Y')
        box((sign*.35,-.01,.37),(.045,.055,.11),steel,fore)
        box((sign*.35,-.025,.308),(.085,.065,.035),dark,hand)
        for dx in [-.034,.034]:box((sign*.35+dx,-.045,.27),(.023,.065,.065),steel,hand,.005)
        x=sign*.14
        housing=box((x,0,.91),(.25,.15,.205),steel,head,.045)
        housing.rotation_euler.y=sign*.07
        cylinder((x,-.09,.91),.089,.035,dark,head,'Y')
        ring((x,-.111,.91),.084,.008,trim,head)
        cylinder((x,-.117,.91),.071,.009,glass,head,'Y')
        ring((x,-.124,.91),.054,.004,steel,head)
        cylinder((x,-.13,.91),.041,.006,dark,head,'Y')
        cylinder((x-.018,-.135,.937),.014,.004,glint,head,'Y')
        box((x,0,1.012),(.19,.12,.018),dark,head,.006)
    bpy.ops.object.select_all(action='DESELECT')
    for part in parts:part.select_set(True)
    bpy.context.view_layer.objects.active=parts[0];bpy.ops.object.join()
    mesh=bpy.context.object
    bpy.ops.object.transform_apply(location=True,rotation=True,scale=True)
    for polygon in mesh.data.polygons:polygon.use_smooth=True
    while sum(len(p.vertices)-2 for p in mesh.data.polygons)<70000:
        mod=mesh.modifiers.new('Mechanical surface density','SUBSURF');mod.subdivision_type='SIMPLE';mod.levels=1
        bpy.ops.object.modifier_apply(modifier=mod.name)
    colors=mesh.data.color_attributes.new(name='SurfaceColor',type='FLOAT_COLOR',domain='CORNER')
    for polygon in mesh.data.polygons:
        color=mesh.data.materials[polygon.material_index].node_tree.nodes.get('Principled BSDF').inputs['Base Color'].default_value
        for index in polygon.loop_indices:colors.data[index].color=color
        polygon.material_index=0
    # One vertex-colored material preserves the existing single-surface skin contract.
    combined=material('Painted mechanical surface',(.5,.5,.5),.35,.38)
    vertex_color=combined.node_tree.nodes.new('ShaderNodeVertexColor');vertex_color.layer_name='SurfaceColor'
    combined.node_tree.links.new(vertex_color.outputs['Color'],combined.node_tree.nodes.get('Principled BSDF').inputs['Base Color'])
    mesh.data.materials.clear();mesh.data.materials.append(combined)
    assert len(mesh.vertex_groups)>8
    return mesh
