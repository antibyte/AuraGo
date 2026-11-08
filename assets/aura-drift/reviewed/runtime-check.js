async () => {
    const AD = window.AuraDriftCore, T = window.AuraDriftThree;
    await AD.assets.loadCore();
    // Exercise the real shader: the packed alpha channel must tint paint, never cut holes.
    const renderer = new T.WebGLRenderer({alpha:true,preserveDrawingBuffer:true});
    renderer.setSize(16,16);
    const scene = new T.Scene(), camera = new T.OrthographicCamera(-1,1,1,-1,.1,10);
    camera.position.z=2;
    scene.add(new T.AmbientLight(0xffffff,2));
    const plane = new T.Mesh(new T.PlaneGeometry(2,2)); scene.add(plane);
    const textures = [], mats = [];
    function pixel(hex, alpha, ghost=false) {
        const map = new T.DataTexture(new Uint8Array([100,140,180,alpha]),1,1);
        map.colorSpace=T.SRGBColorSpace; map.needsUpdate=true; textures.push(map);
        const mat=AD.carPaint(T,hex,AD.QUALITY.high.gl,false,ghost,AD.carRimFor(AD.LOOK.alpine),map);
        mats.push(mat); plane.material=mat; renderer.render(scene,camera);
        const data=new Uint8Array(4), gl=renderer.getContext();
        gl.readPixels(8,8,1,1,gl.RGBA,gl.UNSIGNED_BYTE,data);
        return Array.from(data);
    }
    const unpaintedA=pixel('#ff0000',128), unpaintedB=pixel('#0000ff',128);
    const paintedA=pixel('#ff0000',255), paintedB=pixel('#0000ff',255), ghost=pixel('#ff0000',128,true);
    if (unpaintedA.some((v,i)=>Math.abs(v-unpaintedB[i])>3) || unpaintedA[3]!==255) throw new Error('Unpainted artwork tinted or transparent');
    if (paintedA[0]<paintedB[0]+30 || paintedB[2]<paintedA[2]+30) throw new Error('Paint atlas did not change livery');
    if (ghost[3]<80 || ghost[3]>100) throw new Error('Paint mask changed ghost opacity');
    for (const m of mats) m.dispose(); for (const t of textures) t.dispose();
    plane.geometry.dispose(); renderer.dispose(); renderer.forceContextLoss();
    const verification = [];
    for (const car of AD.CARS) {
        if (AD.assets.isStandIn(AD.assets.car(car.id))) throw new Error(car.id + ': stand-in');
        const rig = AD.createCarRig(T, car, {livery:0,gl:AD.QUALITY.high.gl});
        const maps=[];
        rig.fxRoot.traverse(o=>{if(o.isMesh && o.userData.srcName==='paint') {
            if(!o.material.map || !o.geometry.attributes.uv || o.material.map.image.width!==1024 || o.material.map.image.height!==1024) throw new Error(car.id+': missing retained UV artwork');
            maps.push(o.material.map);
        }});
        for(const tier of ['low','high']) {
            rig.setGl(AD.QUALITY[tier].gl);
            rig.fxRoot.traverse(o=>{if(o.isMesh && o.userData.srcName==='paint' && !maps.includes(o.material.map)) throw new Error(car.id+': lost atlas on quality change');});
        }
        if (rig.wheels.length !== 4) throw new Error(car.id + ': wheel count');
        for (const w of rig.wheels) {
            if (Math.abs(w.radius-w.node.position.y)>w.radius*.03) throw new Error(car.id+': wheel ground clearance or scaled radius');
        }
        const before = rig.wheels.map(w => w.node.quaternion.clone());
        const transforms = {};
        for (const key of ['cone','blade','gnome','lamp']) if (rig.parts[key]) transforms[key] = rig.parts[key].quaternion.clone();
        const vehicle = AD.createVehicle(0,0,0,AD.CARS.indexOf(car));
        vehicle.vx = 12; vehicle.r = 0.4; vehicle.steer = 0.2;
        for (let i=0;i<120;i++) rig.update(vehicle,1/60,{night:false});
        if (rig.wheels.some((w,i)=>w.node.quaternion.angleTo(before[i])<0.01)) throw new Error(car.id+': wheel spin');
        if (rig.wheels.some(w=>Math.abs(w.node.rotation.y-(w.front?-0.2:0))>0.001)) throw new Error(car.id+': steering');
        const frontWheel=rig.wheels.find(w=>w.front), rearWheel=rig.wheels.find(w=>!w.front);
        if(Math.abs(frontWheel.spin*frontWheel.radius-rearWheel.spin*rearWheel.radius)>.0001) throw new Error(car.id+': axle sizes changed rolling distance');
        for (const key in transforms) if (rig.parts[key].quaternion.angleTo(transforms[key])<0.001) throw new Error(car.id+': '+key+' did not move');
        if (car.id==='couch' && !rig.parts.cushion) throw new Error('Missing cushion');
        if (rig.parts.cushion) {
            const y = rig.parts.cushion.position.y;
            rig.kick('landing',1);
            for (let i=0;i<10;i++) rig.update(vehicle,1/60,{night:false});
            if (Math.abs(rig.parts.cushion.position.y-y)<0.001) throw new Error('Cushion did not bounce');
        }
        for (const alpha of [0.4,1]) {
            rig.look({soot:0,glow:[0,0,0],alpha});
            rig.fxRoot.traverse(o => {
                for (const m of (Array.isArray(o.material)?o.material:[o.material]).filter(Boolean)) {
                    if (m.name==='glass' && (m.depthWrite !== (alpha===1) || m.transparent !== (alpha<1))) throw new Error(car.id+': glazing/cloak depth');
                }
            });
        }
        verification.push({car:car.id,wheels:4,animated:Object.keys(transforms),size:rig.size});
        rig.dispose();
    }
    for (const biome of ['alpine','desert','harbour','salt','forest','industry','glacier','metro']) await AD.assets.loadBiome(biome);
    const props = {
        shared:['grandstand','gnome_statue','hotdog_stand','camper_van'],
        alpine:['chalet','cablecar_pylon'],desert:['water_tower','windmill_pump'],
        harbour:['lighthouse','tugboat'],salt:['ufo','rover'],forest:['watermill','hunting_hide'],
        industry:['cooling_tower','chem_tank'],glacier:['research_station','snow_cat'],metro:['food_truck','metro_entrance']
    };
    for (const [biome,names] of Object.entries(props)) for (const name of names) {
        if (AD.assets.isStandIn(AD.assets.prop(biome,name))) throw new Error(biome+'/'+name+': stand-in');
    }
    if (!AD.assets.prop('desert','windmill_pump').getObjectByName('rotor')) throw new Error('Missing windmill rotor');
    if (AD.assets.stats().failed) throw new Error('Asset loading failed');
    await AuraDriftApp.debug.start({mode:'quick',carId:'maehkoenig',trackId:'alpenpass'});
    AuraDriftApp.debug.autopilot(true,{skill:1});
    AuraDriftApp.debug.fastForward(12,{renderEvery:30});
    window.__log = {assetStats:AD.assets.stats(),verification,shader:{unpaintedA,unpaintedB,paintedA,paintedB,ghost}};
    window.__steps = [async()=>{}];
}
