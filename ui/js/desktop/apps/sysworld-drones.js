import * as THREE from 'three';

// Three decorative service drones on smooth closed patrol loops above the districts. They reuse
// the kit's LOD template from the scene cache; clones share geometry and materials.
const patrols = [
  { points: [[-62, 34, -62], [8, 44, -74], [62, 38, -22], [56, 47, 42], [-8, 41, 62], [-64, 36, 12]], speed: 9.5 },
  { points: [[30, 56, -12], [0, 62, 18], [-30, 58, -12], [0, 66, -42]], speed: 8 },
  { points: [[42, 29, -56], [-28, 33, -42], [-52, 27, 20], [18, 31, 52], [60, 33, 8]], speed: 11 },
];
export function createDrones(scene, options = {}) {
  const group = new THREE.Group(); group.name = 'city-drones'; scene.add(group);
  const geometries = [], materials = [], drones = [], tangent = new THREE.Vector3(), ahead = new THREE.Vector3(), up = new THREE.Vector3(0, 1, 0);
  const matrix = new THREE.Matrix4(), quaternion = new THREE.Quaternion(), ZERO = new THREE.Vector3();
  const lightGeometry = new THREE.SphereGeometry(.16, 8, 6); geometries.push(lightGeometry);
  const lightMaterial = color => { const m = new THREE.MeshBasicMaterial({ color, toneMapped: false }); materials.push(m); return m; };
  const red = lightMaterial(0xff4a3c), green = lightMaterial(0x4cff8a), strobe = lightMaterial(0xffffff);
  patrols.forEach((patrol, i) => {
    const curve = new THREE.CatmullRomCurve3(patrol.points.map(p => new THREE.Vector3(...p)), true, 'centripetal', .6);
    const root = new THREE.Group(); root.name = 'city-drone-' + (i + 1); root.visible = false;
    const body = new THREE.Group(); root.add(body);
    const lights = [new THREE.Mesh(lightGeometry, red), new THREE.Mesh(lightGeometry, green), new THREE.Mesh(lightGeometry, strobe)];
    lights[0].position.set(-2.6, .8, 0); lights[1].position.set(2.6, .8, 0); lights[2].position.set(0, 2.1, -.4);
    root.add(...lights); group.add(root);
    drones.push({ root, body, lights, curve, length: curve.getLength(), speed: patrol.speed, t: i * .37, rotors: [], roll: 0 });
  });
  let animated = true;
  function place(drone, dt) {
    const { curve, root } = drone;
    drone.t = (drone.t + dt * drone.speed / drone.length) % 1;
    curve.getPointAt(drone.t, root.position); curve.getTangentAt(drone.t, tangent);
    curve.getTangentAt((drone.t + .015) % 1, ahead);
    // Kit models face +z, so the matrix looks from the tangent tip back to the origin.
    matrix.lookAt(tangent, ZERO, up); quaternion.setFromRotationMatrix(matrix);
    root.quaternion.slerp(quaternion, dt > 0 ? Math.min(1, dt * 4) : 1);
    // Bank into turns using the heading change over a short look-ahead.
    const turn = Math.atan2(ahead.x, ahead.z) - Math.atan2(tangent.x, tangent.z);
    const wrapped = Math.atan2(Math.sin(turn), Math.cos(turn));
    drone.roll += (THREE.MathUtils.clamp(wrapped * 12, -.55, .55) - drone.roll) * (dt > 0 ? Math.min(1, dt * 3) : 1);
    drone.body.rotation.z = drone.roll;
  }
  return {
    setTemplate(template) {
      if (!template) return;
      for (const drone of drones) {
        drone.body.clear(); drone.rotors.length = 0;
        const model = template.clone(true); model.scale.setScalar(1.9);
        model.traverse(n => { if (n.isMesh) { n.castShadow = false; n.receiveShadow = false; } if (/^rotor_/.test(n.name)) drone.rotors.push(n); });
        drone.body.add(model); drone.root.visible = true;
      }
    },
    update(dt, time, active) {
      animated = active;
      const step = active ? Math.min(.1, Math.max(0, dt)) : 0;
      drones.forEach((drone, i) => {
        place(drone, step);
        drone.rotors.forEach((rotor, j) => { rotor.rotation.y += step * (j % 2 ? -46 : 46); });
        const blink = Math.sin(time * 5 + i * 1.7);
        drone.lights[0].visible = drone.lights[1].visible = !active || blink > -.2;
        drone.lights[2].visible = !active || (blink > .93);
        drone.lights[2].scale.setScalar(active ? 1.6 : 1);
      });
    },
    stats: () => ({ drones: drones.filter(d => d.root.visible).length, animated, positions: drones.map(d => d.root.position.toArray().map(v => Math.round(v))) }),
    dispose() { group.removeFromParent(); geometries.forEach(g => g.dispose()); materials.forEach(m => m.dispose()); drones.length = 0; },
  };
}
