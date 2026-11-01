import * as THREE from 'three';

// Decorative data traffic: motes travel along fixed arcs between the district
// anchors. The routes are part of the city, not a reading of the system - the
// effect never shows telemetry, and the same arcs exist for every snapshot.
//
// The pool is bounded, the geometry is one Points draw call, and the whole
// layer freezes with reduced motion so the city stays quiet.

const CHAIN = 3;           // points per mote, drawn as a short comet tail
const MOTES_PER_ROUTE = 3;
const MAX_POINTS = 192;

const TINTS = [0x8ee8ee, 0x75f4d1, 0xffc98a, 0xb9a4ff];

export function createFlow(scene, options = {}) {
  const anchors = options.anchors || [];
  const group = new THREE.Group();
  group.name = 'city-flow';
  scene.add(group);

  // Deterministic routes: every anchor reaches the next few in a fixed ring,
  // so the skyline always carries the same pattern.
  const routes = [];
  for (let i = 0; i < anchors.length; i++) {
    for (const step of [1, 3]) {
      const from = anchors[i], to = anchors[(i + step) % anchors.length];
      if (!from || !to || from.id === to.id) continue;
      routes.push({ from, to, lift: 22 + ((i + step) % 4) * 11 });
    }
  }
  if (routes.length < 2) {
    group.removeFromParent();
    return { group, update() {}, setTier() {}, stats: () => ({ routes: 0, points: 0, visible: false }), dispose() {} };
  }

  const moteCount = Math.min(MAX_POINTS / CHAIN, routes.length * MOTES_PER_ROUTE);
  const pointCount = moteCount * CHAIN;
  const positions = new Float32Array(pointCount * 3);
  const alphas = new Float32Array(pointCount);
  const sizes = new Float32Array(pointCount);
  const tints = new Float32Array(pointCount * 3);
  const geometry = new THREE.BufferGeometry();
  geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
  geometry.setAttribute('alpha', new THREE.BufferAttribute(alphas, 1));
  geometry.setAttribute('size', new THREE.BufferAttribute(sizes, 1));
  geometry.setAttribute('tint', new THREE.BufferAttribute(tints, 3));

  const material = new THREE.ShaderMaterial({
    transparent: true, depthWrite: false, blending: THREE.AdditiveBlending, fog: false,
    uniforms: { pointScale: { value: 900 }, strength: { value: .4 } },
    vertexShader: `attribute float alpha,size;attribute vec3 tint;uniform float pointScale;varying float vAlpha;varying vec3 vTint;
void main(){vec4 mv=modelViewMatrix*vec4(position,1.0);gl_Position=projectionMatrix*mv;
gl_PointSize=clamp(size*pointScale/max(1.0,-mv.z),1.0,12.0);vAlpha=alpha;vTint=tint;}`,
    fragmentShader: `uniform float strength;varying float vAlpha;varying vec3 vTint;
void main(){float d=length(gl_PointCoord-0.5)*2.0;if(d>1.0)discard;
gl_FragColor=vec4(vTint*pow(1.0-d,2.4)*vAlpha*strength,1.0);}`,
  });
  const points = new THREE.Points(geometry, material);
  points.frustumCulled = false;
  points.renderOrder = 2;
  group.add(points);

  const motes = [];
  const colour = new THREE.Color();
  for (let i = 0; i < moteCount; i++) {
    const route = routes[i % routes.length];
    colour.setHex(TINTS[i % TINTS.length]);
    motes.push({
      route, t: (i * 0.3819) % 1, speed: .045 + ((i * 7) % 9) / 190,
      tint: [colour.r, colour.g, colour.b], size: .42 + ((i * 3) % 5) * .08,
    });
  }

  // A quadratic arc between two anchors, lifted above the taller roof line.
  function sample(route, t, out) {
    const from = route.from, to = route.to, u = 1 - t;
    const cx = (from.x + to.x) / 2, cz = (from.z + to.z) / 2;
    const cy = Math.max(from.height, to.height) + route.lift;
    const ay = from.height + 4, by = to.height + 4;
    out.x = u * u * from.x + 2 * u * t * cx + t * t * to.x;
    out.y = u * u * ay + 2 * u * t * cy + t * t * by;
    out.z = u * u * from.z + 2 * u * t * cz + t * t * to.z;
    return out;
  }

  const scratch = { x: 0, y: 0, z: 0 };
  let tier = 'high', strength = .4;

  function update(dt, animated, night) {
    if (!points.visible) return;
    const step = animated ? Math.min(dt, .05) : 0;
    for (let i = 0; i < motes.length; i++) {
      const mote = motes[i];
      mote.t = (mote.t + step * mote.speed) % 1;
      for (let c = 0; c < CHAIN; c++) {
        const index = i * CHAIN + c;
        // The tail trails behind the head along the same arc.
        const t = (mote.t - c * .012 + 1) % 1;
        sample(mote.route, t, scratch);
        positions[index * 3] = scratch.x;
        positions[index * 3 + 1] = scratch.y;
        positions[index * 3 + 2] = scratch.z;
        const fade = Math.min(1, Math.min(t, 1 - t) * 7);
        alphas[index] = fade * (1 - c / CHAIN) * .9;
        sizes[index] = mote.size * (1 - c * .22);
        tints[index * 3] = mote.tint[0];
        tints[index * 3 + 1] = mote.tint[1];
        tints[index * 3 + 2] = mote.tint[2];
      }
    }
    geometry.attributes.position.needsUpdate = true;
    geometry.attributes.alpha.needsUpdate = true;
    geometry.attributes.size.needsUpdate = true;
    geometry.attributes.tint.needsUpdate = true;
    strength += (.18 + night * .82 - strength) * Math.min(1, Math.max(0, dt) * 2);
    material.uniforms.strength.value = strength;
  }

  return {
    group,
    update,
    setPointScale(value) { material.uniforms.pointScale.value = value; },
    setTier(value) {
      tier = value;
      points.visible = tier !== 'low';
      group.visible = tier !== 'low';
    },
    stats: () => ({ routes: routes.length, points: pointCount, visible: points.visible, tier }),
    dispose() {
      group.removeFromParent();
      geometry.dispose();
      material.dispose();
    },
  };
}
