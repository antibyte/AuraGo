import * as THREE from 'three';

const OPEN_VERT = `varying vec3 vGolfWorld;
#include <fog_pars_vertex>
void main() {
    vec4 world = modelMatrix * vec4(position, 1.0);
    vGolfWorld = world.xyz;
    vec4 mvPosition = viewMatrix * world;
    gl_Position = projectionMatrix * mvPosition;
    #include <fog_vertex>
}`;
const OPEN_FRAG = `uniform vec3 shallow; uniform vec3 deep; uniform vec3 skyColor; uniform float inner; uniform float outer; uniform float glints;
uniform float golfTime; uniform float golfMotion; uniform float golfWaterMotion;
varying vec3 vGolfWorld;
#include <fog_pars_fragment>
void main() {
    float t = golfTime * golfMotion * golfWaterMotion;
    float depth = smoothstep(inner, outer, length(vGolfWorld.xz));
    vec3 color = mix(shallow, deep, depth);
    float wave = sin(vGolfWorld.x * 1.3 + t * 0.9) * sin(vGolfWorld.z * 1.1 - t * 0.7) + 0.5 * sin((vGolfWorld.x + vGolfWorld.z) * 2.7 + t * 1.6);
    color *= 0.95 + 0.05 * wave;
    vec3 view = normalize(cameraPosition - vGolfWorld);
    color = mix(color, skyColor, pow(1.0 - clamp(view.y, 0.0, 1.0), 3.0) * 0.45);
    color += vec3(1.0) * glints * pow(max(0.0, wave - 0.95), 4.0) * 3.0 * (1.0 - depth * 0.6);
    gl_FragColor = vec4(color, 1.0);
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
    #include <fog_fragment>
}`;
const SHORE_VERT = `varying vec2 vGolfUv; varying vec3 vGolfWorld;
#include <fog_pars_vertex>
void main() {
    vGolfUv = uv;
    vec4 world = modelMatrix * vec4(position, 1.0);
    vGolfWorld = world.xyz;
    vec4 mvPosition = viewMatrix * world;
    gl_Position = projectionMatrix * mvPosition;
    #include <fog_vertex>
}`;
const SHORE_FRAG = `uniform vec3 shallow; uniform vec3 foam; uniform float golfTime; uniform float golfMotion; uniform float golfWaterMotion;
varying vec2 vGolfUv; varying vec3 vGolfWorld;
#include <fog_pars_fragment>
void main() {
    float t = golfTime * golfMotion * golfWaterMotion, u = vGolfUv.x;
    float edge = 0.16 + 0.05 * sin(t * 1.4 + vGolfWorld.x * 2.1 + vGolfWorld.z * 1.7);
    float band = 1.0 - smoothstep(edge, edge + 0.07, u);
    float ring = 1.0 - smoothstep(0.0, 0.035, abs(u - (0.45 + 0.08 * sin(t * 0.8 + vGolfWorld.x * 0.7))));
    float amount = clamp(band + ring * 0.55, 0.0, 1.0);
    gl_FragColor = vec4(mix(shallow * 1.06, foam, amount), (1.0 - smoothstep(0.55, 1.0, u)) * mix(0.55, 0.95, amount));
    #include <tonemapping_fragment>
    #include <colorspace_fragment>
    #include <fog_fragment>
}`;

// Open water disc plus the course's shore strip; the moon look has no water.
export function createWater(look, cfg, state) {
    const group = new THREE.Group();
    group.name = 'aurago-water';
    const water = look.water;
    if (!water || water.mode === 'void') return { group, attachShore() {}, setTier() {}, dispose() { group.removeFromParent(); } };
    const uniforms = THREE.UniformsUtils.merge([THREE.UniformsLib.fog, {
        shallow: { value: new THREE.Color(water.shallow) }, deep: { value: new THREE.Color(water.deep) }, foam: { value: new THREE.Color(water.foam) },
        skyColor: { value: new THREE.Color(look.sky.horizon) }, inner: { value: 4.8 }, outer: { value: 24 }, glints: { value: cfg.glints ? 1 : 0 },
        golfWaterMotion: { value: cfg.waterMotion ? 1 : 0 }
    }]);
    uniforms.golfTime = state.golfTime;
    uniforms.golfMotion = state.golfMotion;
    const material = new THREE.ShaderMaterial({ uniforms, vertexShader: OPEN_VERT, fragmentShader: OPEN_FRAG, fog: true });
    const disc = new THREE.Mesh(new THREE.CircleGeometry(60, 72).rotateX(-Math.PI / 2), material);
    disc.name = 'water-open'; disc.position.y = water.level; disc.renderOrder = -1;
    group.add(disc);
    const shores = [];
    return {
        group,
        attachShore(node) {
            const shore = new THREE.ShaderMaterial({ uniforms, vertexShader: SHORE_VERT, fragmentShader: SHORE_FRAG, transparent: true, depthWrite: false, fog: true });
            node.material?.dispose?.(); node.material = shore; node.renderOrder = 1; node.castShadow = false; node.receiveShadow = false;
            shores.push(shore);
        },
        setTier(next) { uniforms.glints.value = next.glints ? 1 : 0; uniforms.golfWaterMotion.value = next.waterMotion ? 1 : 0; },
        dispose() { group.removeFromParent(); disc.geometry.dispose(); material.dispose(); for (const shore of shores) shore.dispose(); }
    };
}
