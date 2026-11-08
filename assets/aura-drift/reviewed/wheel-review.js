async () => {
    const AD = window.AuraDriftCore;
    await AD.assets.loadCore();
    let phase = 0;
    let currentRig = null;
    const render = window.AuraDriftThree.WebGLRenderer.prototype.render;
    window.AuraDriftThree.WebGLRenderer.prototype.render = function (scene, camera) {
        if (currentRig && camera.isPerspectiveCamera && camera.fov===36 && camera.far===500) {
            camera.zoom=1.6;
            camera.lookAt(0,currentRig.size.h*.22,0);
            camera.updateProjectionMatrix();
        }
        return render.call(this,scene,camera);
    };
    const createRig = AD.createCarRig;
    AD.createCarRig = (...args) => {
        const rig = createRig(...args), update = rig.update;
        currentRig=rig;
        rig.update = (vehicle, dt, context) => {
            for (const wheel of rig.wheels) wheel.spin = phase;
            update(vehicle, 0, context);
        };
        return rig;
    };
    const filter = '__ARG__'.split(',').filter(Boolean);
    const cars = AD.CARS.filter(car => !filter.length || filter.includes(car.id));
    const poses = [
        {angle:1.95, steer:0, spin:0},
        {angle:.9, steer:.6, spin:Math.PI/8},
        {angle:-1.95, steer:-.6, spin:Math.PI/4}
    ];
    window.__steps = cars.flatMap(car => poses.map(pose => async () => {
        phase = pose.spin;
        AuraDriftApp.debug.showcase({carId:car.id,livery:0,
            view:{angle:pose.angle,elev:.10},pose:{steer:pose.steer}});
        window.__log = {car:car.id,pose};
    }));
    await window.__steps[0]();
}
