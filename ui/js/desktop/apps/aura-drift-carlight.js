/* Aura Drift - car readability in the 3D renderer.
 *
 * AD.carRimFor(look)            -> { key, rim:[r,g,b], fill } shading constants for one biome (data only).
 * AD.carShade(material, rim)    adds a fresnel rim and a small albedo fill to a standard or physical
 *                               material. The constants are baked into the shader, so every car of a biome
 *                               shares one program and no uniform is touched per frame.
 * AD.createCarLights(T, scene, q, camera, look)
 *                               -> { update(race), setQuality(q), dispose() }: two instanced draws for all cars,
 *                               additive lamp and tail glows (billboards, they bloom) and a headlight pool on the
 *                               road at night. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;
    const clamp = AD.clamp;

    const lin = hex => {
        const n = parseInt(String(hex || '#ffffff').slice(1), 16);
        return [((n >> 16) & 255) / 255, ((n >> 8) & 255) / 255, (n & 255) / 255].map(c => Math.pow(c, 2.2));
    };

    AD.carRimFor = function (look) {
        if (!look) return null;
        const h = lin(look.skyHorizon), s = lin(look.hemiSky || look.skyHorizon);
        const night = !!look.night, k = night ? 0.9 : 0.3;
        /* Horizon and dome colour, pulled towards white so a pink or teal sky does not tint the paint. */
        const rim = [0, 1, 2].map(i => (0.5 * h[i] + 0.5 * s[i]) * 0.55 * k + 0.5 * k * 0.3);
        const fill = night ? 0.16 : 0.02;
        return { key: rim.map(c => c.toFixed(2)).join(',') + '/' + fill, rim: rim, fill: fill };
    };

    AD.carShade = function (material, spec) {
        if (!material || !spec) return;
        const f = n => n.toFixed(3);
        const code = [
            '{',
            '  float agRim = pow(1.0 - clamp(dot(normalize(normal), normalize(vViewPosition)), 0.0, 1.0), 2.2);',
            '  outgoingLight += vec3(' + spec.rim.map(f).join(',') + ') * agRim + diffuseColor.rgb * ' + f(spec.fill) + ';',
            '}'
        ].join('\n');
        material.onBeforeCompile = shader => {
            shader.fragmentShader = shader.fragmentShader.replace('#include <opaque_fragment>', code + '\n#include <opaque_fragment>');
        };
        const key = 'agrim:' + spec.key;
        material.customProgramCacheKey = () => key;
    };

    const SHARED_TUNE = {
        glass: m => { m.envMapIntensity = 1.8; m.depthWrite = false; m.roughness = 0.05; },
        chrome: m => { m.metalness = 1; m.roughness = 0.1; m.envMapIntensity = 1.5; },
        tyre: m => { m.envMapIntensity = 0.35; },
        trim: m => { m.envMapIntensity = 0.7; }
    };

    /* A near-black accent (Big Bertha's #2a2a2a) would turn its flames and trim into holes in the
       paint, so dark accents are lifted to a warm orange. fx uses the same colour for flames. */
    AD.accentColour = function (carDef) {
        const hex = String(carDef.accent || '#ffffff');
        const n = parseInt(hex.slice(1), 16);
        const lum = (0.2126 * ((n >> 16) & 255) + 0.7152 * ((n >> 8) & 255) + 0.0722 * (n & 255)) / 255;
        return lum < 0.3 ? '#ffa01e' : hex;
    };

    AD.carPaint = function (T, hex, gl, vertexColors, ghost, rim) {
        const base = { color: hex, vertexColors: vertexColors, roughness: gl.clearcoat ? 0.28 : 0.3, metalness: gl.clearcoat ? 0.35 : 0.25 };
        const m = gl.clearcoat
            ? new T.MeshPhysicalMaterial(Object.assign(base, { clearcoat: 1, clearcoatRoughness: 0.08 }))
            : new T.MeshStandardMaterial(base);
        m.envMapIntensity = gl.clearcoat ? 1.0 : 1.35;
        if (ghost) { m.transparent = true; m.opacity = 0.35; m.depthWrite = false; } else if (rim && AD.carShade) AD.carShade(m, rim);
        return m;
    };

    /* Shared template materials are tuned once, by their GLB name. */
    AD.carTune = function (src) {
        if (SHARED_TUNE[src.name] && !src.userData.adTuned) { SHARED_TUNE[src.name](src); src.userData.adTuned = true; }
    };

    /* Per-frame shadow-caster gate for cars on the lean tier (gl.casters 1): only cars near the camera cast, and
       only their painted shell. High tiers keep every part casting (set once by the rig). */
    const BODY = { paint: 1, paint_accent: 1 };
    AD.carCast = function (rig, near, gl) {
        if (!gl || gl.casters !== 1 || rig.castNear === near) return;
        rig.castNear = near;
        rig.group.traverse(o => { if (o.isMesh) o.castShadow = !!near && !!gl.shadows && !!BODY[o.userData.srcName]; });
    };

    /* The four wheels as one instanced draw per car. Every wheel node of a car carries the same tyre mesh, the
       right-hand ones mirrored in z; a wheel spins, so a half turn about y (hub out, no mirror) is equivalent.
       The original wheel nodes stay in the rig as the animated transforms and are only hidden. */
    AD.carWheelInst = function (T, wheels, root) {
        if (wheels.length !== 4 || wheels.some(w => !w.node.isMesh)) return null;
        const src = wheels[0].node;
        const im = new T.InstancedMesh(src.geometry, src.material, 4);
        im.frustumCulled = false;
        im.userData.srcName = src.userData.srcName || (src.material && src.material.name) || 'tyre';
        const flip = new T.Matrix4().makeRotationY(Math.PI), m = new T.Matrix4();
        for (const w of wheels) w.node.visible = false;
        root.add(im);
        return {
            mesh: im,
            sync() {
                wheels.forEach((w, i) => {
                    w.node.updateMatrix();
                    m.copy(w.node.matrix);
                    if (w.node.position.z > 0) m.multiply(flip);
                    im.setMatrixAt(i, m);
                });
                im.instanceMatrix.needsUpdate = true;
            }
        };
    };

    function glowTexture(T) {
        const c = document.createElement('canvas');
        c.width = c.height = 64;
        const g = c.getContext('2d');
        const grad = g.createRadialGradient(32, 32, 0, 32, 32, 32);
        grad.addColorStop(0, 'rgba(255,255,255,1)');
        grad.addColorStop(0.18, 'rgba(255,255,255,0.75)');
        grad.addColorStop(0.5, 'rgba(255,255,255,0.18)');
        grad.addColorStop(1, 'rgba(255,255,255,0)');
        g.fillStyle = grad;
        g.fillRect(0, 0, 64, 64);
        return new T.CanvasTexture(c);
    }

    /* Beam pool: u runs forward from the lamps, v across the road. A narrow neck that opens into a soft cone. */
    function beamTexture(T) {
        const W = 128, H = 64;
        const c = document.createElement('canvas');
        c.width = W; c.height = H;
        const g = c.getContext('2d');
        const img = g.createImageData(W, H);
        for (let y = 0; y < H; y++) {
            for (let x = 0; x < W; x++) {
                const u = x / (W - 1), v = (y / (H - 1) - 0.5) * 2;
                const half = 0.16 + 0.84 * Math.pow(u, 0.7);
                const edge = clamp(1 - Math.abs(v) / half, 0, 1);
                const a = Math.pow(edge, 1.5) * Math.pow(1 - u, 1.2) * clamp(u * 14, 0, 1) * 0.7;
                const i = (y * W + x) * 4;
                img.data[i] = img.data[i + 1] = img.data[i + 2] = 255;
                img.data[i + 3] = Math.round(a * 255);
            }
        }
        g.putImageData(img, 0, 0);
        return new T.CanvasTexture(c);
    }

    AD.createCarLights = function (T, scene, q, camera, look) {
        const MAX = 8, night = !!(look && look.night), headK = look && look.headlights ? look.headlights : 0.6;
        let useBeams = night && !(q && q.shadows === 0);
        const glowTex = glowTexture(T), beamTex = beamTexture(T);
        const additive = map => new T.MeshBasicMaterial({
            map: map, transparent: true, blending: T.AdditiveBlending, depthWrite: false, fog: false, side: T.DoubleSide,
            polygonOffset: true, polygonOffsetFactor: -2, polygonOffsetUnits: -2
        });
        const glowMat = additive(glowTex), beamMat = additive(beamTex);
        const glowGeo = new T.PlaneGeometry(1, 1);
        const beamGeo = new T.PlaneGeometry(1, 1).rotateX(-Math.PI / 2).translate(0.5, 0, 0);
        const glows = new T.InstancedMesh(glowGeo, glowMat, MAX * 6);
        const beams = new T.InstancedMesh(beamGeo, beamMat, MAX * 2);
        for (const m of [glows, beams]) {
            m.frustumCulled = false; m.count = 0; m.renderOrder = 4;
            m.instanceMatrix.setUsage(T.DynamicDrawUsage);
            m.setColorAt(0, new T.Color(0, 0, 0));
            m.instanceColor.setUsage(T.DynamicDrawUsage);
            m.name = 'agdrift-carlights';
            scene.add(m);
        }
        const o = new T.Object3D(), col = new T.Color(), q0 = new T.Quaternion(), e0 = new T.Euler(0, 0, 0, 'YXZ');
        const sc = new T.Vector3(), pos = new T.Vector3();

        function put(mesh, i, x, y, z, qt, sx, sy, sz, r, g, b) {
            pos.set(x, y, z); sc.set(sx, sy, sz);
            o.matrix.compose(pos, qt, sc);
            mesh.setMatrixAt(i, o.matrix);
            mesh.setColorAt(i, col.setRGB(r, g, b));
        }

        return {
            update(race, halfOf) {
                let gi = 0, bi = 0;
                const eye = camera.position;
                for (const car of race.cars) {
                    const v = car.v, idx = race.cars.indexOf(car);
                    if (v.out || v.cloak > 0 || gi + 6 > MAX * 6) continue;
                    const dx = v.x - eye.x, dz = v.y - eye.z;
                    if (dx * dx + dz * dz > 190 * 190) continue;
                    const c = Math.cos(v.psi), s = Math.sin(v.psi);
                    const half = halfOf ? halfOf(idx) : 2.2, wid = half * 0.2;
                    const braking = v.brake > 0.1 || v.handbrake || v.vx < -0.5;
                    const reversing = v.vx < -0.5;
                    const lift = v.z + 0.7;
                    /* Seen from the front the lamps glow, seen from behind the tail lights do. */
                    const fx = eye.x - v.x, fz = eye.z - v.y, fl = Math.hypot(fx, fz) || 1;
                    const facing = (fx * c + fz * s) / fl;
                    const headK2 = clamp((facing + 0.15) / 0.6, 0, 1), tailK = clamp((0.25 - facing) / 0.6, 0, 1);
                    for (const side of [-1, 1]) {
                        /* Lamps: left of the car is -z_local; in sim space the side vector is (-s, c). */
                        const lx = -s * wid * side, lz = c * wid * side;
                        if (night) put(glows, gi++, v.x + c * (half + 0.12) + lx, lift, v.y + s * (half + 0.12) + lz, camera.quaternion, 0.8, 0.8, 1, 1.9 * headK * headK2, 1.75 * headK * headK2, 1.35 * headK * headK2);
                        if (night || braking) {
                            const big = braking ? 0.95 : 0.55;
                            put(glows, gi++, v.x - c * (half + 0.1) + lx, lift, v.y - s * (half + 0.1) + lz, camera.quaternion, big, big, 1, (braking ? 2.4 : 1.5) * tailK, (braking ? 0.18 : 0.09) * tailK, (braking ? 0.1 : 0.05) * tailK);
                        }
                        if (reversing && tailK > 0.05 && gi < MAX * 6) {
                            put(glows, gi++, v.x - c * (half + 0.11) + lx * 0.55, lift - 0.04, v.y - s * (half + 0.11) + lz * 0.55, camera.quaternion, 0.42, 0.42, 1, 1.5 * tailK, 1.55 * tailK, 1.7 * tailK);
                        }
                    }
                    if (car.carId === 'softeis' && (night || v.boost || v.overdrive > 0 || v.burst > 0) && gi < MAX * 6) {
                        const pulse = 0.65 + 0.35 * Math.sin((race.time || 0) * 14 + idx);
                        put(glows, gi++, v.x - c * 0.15, v.z + 2.25, v.y - s * 0.15, camera.quaternion, 0.85, 0.85, 1, 2.1 * pulse, 0.38 * pulse, 0.65 * pulse);
                    }
                    if (v.z < 1.2 && !v.airborne) {
                        e0.set(0, -v.psi, 0);
                        q0.setFromEuler(e0);
                        const hasUnder = v.overdrive > 0 || v.boost || v.burst > 0 || v.shield > 0 || (v.driftLevel > 0 && v.slip > 1.1);
                        if (hasUnder && bi < MAX * 2) {
                            const ur = v.overdrive > 0 ? 1.35 : v.shield > 0 ? 0.25 : v.driftLevel === 3 ? 1.35 : v.driftLevel === 2 ? 1.25 : 0.2;
                            const ug = v.overdrive > 0 ? 0.48 : v.shield > 0 ? 1.15 : v.driftLevel === 3 ? 0.22 : v.driftLevel === 2 ? 0.82 : 0.95;
                            const ub = v.overdrive > 0 ? 0.12 : v.shield > 0 ? 0.85 : v.driftLevel === 3 ? 0.88 : v.driftLevel === 2 ? 0.18 : 1.35;
                            put(beams, bi++, v.x - c * half * 0.85, v.z + 0.08, v.y - s * half * 0.85, q0, half * 1.9, 1, half * 1.45, ur, ug, ub);
                        }
                        if (useBeams && bi < MAX * 2) {
                            put(beams, bi++, v.x + c * (half - 0.2), v.z + 0.09, v.y + s * (half - 0.2), q0, 12 + 8 * headK, 1, 8.5, 0.95, 0.85, 0.6);
                        }
                    }
                }
                glows.count = gi; beams.count = bi;
                glows.visible = gi > 0; beams.visible = bi > 0;
                for (const m of [glows, beams]) {
                    m.instanceMatrix.needsUpdate = true;
                    if (m.instanceColor) m.instanceColor.needsUpdate = true;
                }
            },
            setQuality(next) { q = next; useBeams = night && !(q && q.shadows === 0); },
            dispose() {
                for (const m of [glows, beams]) { m.removeFromParent(); m.dispose(); }
                glowGeo.dispose(); beamGeo.dispose(); glowMat.dispose(); beamMat.dispose(); glowTex.dispose(); beamTex.dispose();
            }
        };
    };
})();
