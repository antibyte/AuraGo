async () => {
    const AD = window.AuraDriftCore, T = window.AuraDriftThree;
    await AD.assets.loadCore();
    const verification = [];
    for (const car of AD.CARS) {
        if (AD.assets.isStandIn(AD.assets.car(car.id))) throw new Error(car.id + ': stand-in');
        const rig = AD.createCarRig(T, car, {livery:0,gl:AD.QUALITY.high.gl});
        if (rig.wheels.length !== 4) throw new Error(car.id + ': wheel count');
        const before = rig.wheels.map(w => w.node.quaternion.clone());
        const transforms = {};
        for (const key of ['cone','blade','gnome','lamp']) if (rig.parts[key]) transforms[key] = rig.parts[key].quaternion.clone();
        const vehicle = AD.createVehicle(0,0,0,AD.CARS.indexOf(car));
        vehicle.vx = 12; vehicle.r = 0.4; vehicle.steer = 0.2;
        for (let i=0;i<120;i++) rig.update(vehicle,1/60,{night:false});
        if (rig.wheels.some((w,i)=>w.node.quaternion.angleTo(before[i])<0.01)) throw new Error(car.id+': wheel spin');
        if (rig.wheels.some(w=>Math.abs(w.node.rotation.y-(w.front?-0.2:0))>0.001)) throw new Error(car.id+': steering');
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
    window.__log = {assetStats:AD.assets.stats(),verification};
}
