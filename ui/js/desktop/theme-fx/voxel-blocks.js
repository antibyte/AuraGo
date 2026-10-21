(function () {
    'use strict';

    // Pixel-art block atlas (window frames, fragments) and the shader patches that give the diorama its
    // blocky look: texel noise, ambient occlusion in the creases, cloud shadows, foliage sway, glowing
    // windows and sparkling water. Every number that decides how the terrain looks lives in this block.
    const BEVEL_LIGHT = 1.12;      // atlas pixel bevel: row 0 and column 0 brightened ...
    const BEVEL_DARK = .85;        // ... row 7 and column 7 darkened
    const TEXEL_WATER = .08;       // texel noise of the water (the terrain uses uniforms.uTexel)
    const AO_FLOOR = .42;          // brightness of a fully occluded corner
    const AO_POWER = 1.15;
    const EDGE_SHADE = .07;        // darkening of the outer rim of every voxel face (reads as a bevel)
    const EDGE_START = .40;        // distance from the face centre (0 .. .5) where the rim starts
    const CLOUD_SHADOW_LOW = .52;
    const CLOUD_SHADOW_HIGH = .78;
    const CLOUD_SHADOW_SCALE = .045;
    const WATER_SURFACE_Y = 3.9;   // water vertices above this world height belong to the moving surface
    const WATER_WAVE = .05;
    const SPARKLE_POWER = 40;
    const SPARKLE_CELLS = 6;
    const SPARKLE_GAIN = 1.5;
    const DECO_SWAY = 2;           // decorations sway this much more than the foliage blocks
    const GLOW_BASE = .85;
    const GLOW_FLICKER = .15;

    // 8x8 patterns, row 0 is the top. The palette letters are per pattern.
    const PATTERNS = {
        oak_planks: {
            palette: { H: '#d9b67a', L: '#c39a5d', M: '#ad854e', D: '#8a6539', S: '#6b4c2a' },
            rows: ['HLLLLDHL', 'LMLLMDLM', 'MMLMMDMM', 'SSSSSSSS', 'LDHLLLLL', 'MDLMLLML', 'MDMMMLMM', 'SSSSSSSS']
        },
        oak_log_end: {
            palette: { B: '#5f4527', R: '#b38b55', L: '#cfa96f' },
            rows: ['BBBBBBBB', 'BRRRRRRB', 'BRLLLLRB', 'BRLRRLRB', 'BRLRRLRB', 'BRLLLLRB', 'BRRRRRRB', 'BBBBBBBB']
        },
        cobble: {
            palette: { H: '#b9b9b9', L: '#9e9e9e', M: '#8a8a8a', D: '#5e5e5e' },
            rows: ['HLLDHLLM', 'LMMDLMMD', 'DDMDDDDD', 'HLDDHLLL', 'LMMDLMMM', 'DDDDDMDD', 'LLDHLLDH', 'MMDLMMDL']
        },
        smooth_stone: {
            palette: { H: '#c9c9c9', L: '#adadad', M: '#8f8f8f', D: '#6e6e6e' },
            rows: ['HHHHHHHL', 'HLLLLLLM', 'HLLLLLLM', 'HLLLLLLM', 'HLLLLLLM', 'HLLLLLLM', 'HLLLLLLM', 'LMMMMMMD']
        },
        dirt: {
            palette: { L: '#8f6342', M: '#7a5236', D: '#5f3e27' },
            rows: ['LMLDLMLL', 'MLLMLLDM', 'LDMLMLML', 'MLLLDMLL', 'LMDLLLMD', 'LLMMLDLM', 'DMLLMLLL', 'LLMDLMDL']
        },
        grass_side: {
            palette: { G: '#7cbd4a', g: '#62a03a', L: '#8f6342', M: '#7a5236', D: '#5f3e27' },
            rows: ['GGgGGGgG', 'gGGgGgGG', 'GgDgGDgG', 'DLgDLDgL', 'LMDLMLDM', 'MLLMLLDM', 'LDMLMLML', 'MLLLDMLL']
        }
    };
    const FRAME_PATTERNS = {
        wood: { edge: 'oak_planks', corner: 'oak_log_end', top: 'oak_planks' },
        stone: { edge: 'cobble', corner: 'smooth_stone', top: 'cobble' },
        earth: { edge: 'dirt', corner: 'dirt', top: 'grass_side' }
    };

    function hexBytes(hex) {
        const n = parseInt(hex.slice(1), 16);
        return [n >> 16 & 255, n >> 8 & 255, n & 255];
    }

    // One RGBA strip (8*N x 8) holding every pattern next to each other, with a pixel bevel on each tile.
    function buildAtlas(THREE) {
        const names = Object.keys(PATTERNS);
        const width = names.length * 8;
        const data = new Uint8Array(width * 8 * 4);
        names.forEach((name, tile) => {
            const pattern = PATTERNS[name];
            for (let r = 0; r < 8; r++) {
                for (let c = 0; c < 8; c++) {
                    const rgb = hexBytes(pattern.palette[pattern.rows[r][c]] || '#ff00ff');
                    let factor = 1;
                    if (r === 0 || c === 0) factor *= BEVEL_LIGHT;
                    if (r === 7 || c === 7) factor *= BEVEL_DARK;
                    // Texture row 0 is the bottom (v = 0), the patterns are written top-down.
                    const at = ((7 - r) * width + tile * 8 + c) * 4;
                    data[at] = Math.min(255, Math.round(rgb[0] * factor));
                    data[at + 1] = Math.min(255, Math.round(rgb[1] * factor));
                    data[at + 2] = Math.min(255, Math.round(rgb[2] * factor));
                    data[at + 3] = 255;
                }
            }
        });
        const texture = new THREE.DataTexture(data, width, 8, THREE.RGBAFormat);
        texture.magFilter = THREE.NearestFilter;
        texture.minFilter = THREE.NearestFilter;
        texture.generateMipmaps = false;
        texture.colorSpace = THREE.SRGBColorSpace;
        texture.needsUpdate = true;
        const index = {};
        names.forEach((name, i) => { index[name] = i; });
        return { texture, index, count: names.length };
    }

    // The material samples tile aPattern (an InstancedBufferAttribute on the geometry) of the atlas.
    function patchAtlasMaterial(THREE, material, atlas) {
        material.map = atlas.texture;
        material.onBeforeCompile = shader => {
            shader.vertexShader = 'attribute float aPattern;\n' + shader.vertexShader.replace('#include <uv_vertex>',
                '#include <uv_vertex>\n\tvMapUv = vec2((uv.x + aPattern) / ' + atlas.count + '.0, uv.y);');
        };
        material.customProgramCacheKey = () => 'voxel-atlas-' + atlas.count;
        material.needsUpdate = true;
        return material;
    }

    function glsl(number) {
        return Number.isInteger(number) ? number.toFixed(1) : String(number);
    }

    const FRAGMENT_HEAD = `
uniform float uTime;
uniform float uTexel;
uniform vec2 uCloudOffset;
uniform float uCloudShadow;
uniform float uNight;
uniform float uSun;
varying float vAo;
varying float vSeed;
varying vec2 vTUv;
varying vec3 vWorld;
varying float vNy;
float vxHash(vec2 p) { return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453); }
float vxNoise(vec2 p) {
    vec2 i = floor(p);
    vec2 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    return mix(mix(vxHash(i), vxHash(i + vec2(1.0, 0.0)), f.x), mix(vxHash(i + vec2(0.0, 1.0)), vxHash(i + vec2(1.0, 1.0)), f.x), f.y);
}
`;

    const CLOUD_SHADOW = `
float vxCloud = smoothstep(${glsl(CLOUD_SHADOW_LOW)}, ${glsl(CLOUD_SHADOW_HIGH)}, vxNoise(vWorld.xz * ${glsl(CLOUD_SHADOW_SCALE)} + uCloudOffset));
diffuseColor.rgb *= 1.0 - uCloudShadow * vxCloud;
`;

    function vertexHead(kind) {
        let head = 'uniform float uTime;\nuniform float uSway;\nvarying float vAo;\nvarying float vSeed;\nvarying vec2 vTUv;\nvarying vec3 vWorld;\nvarying float vNy;\n';
        if (kind !== 'deco') head += 'attribute float aAo;\nattribute float aSeed;\n';
        if (kind === 'glow') head += 'attribute vec3 aGlow;\nvarying vec3 vGlow;\n';
        return head;
    }

    function vertexBody(kind) {
        if (kind === 'deco') {
            // Instanced cubes: the instance origin drives the phase, the top of the cube sways, and the offset is
            // divided by the instance scale so it is in world units.
            return `
vec3 vxOrigin = instanceMatrix[3].xyz;
float vxTop = clamp(position.y + 0.5, 0.0, 1.0);
float vxScale = max(instanceMatrix[0][0], 0.01);
transformed.x += sin(uTime * 1.6 + vxOrigin.x * 0.37 + vxOrigin.z * 0.23) * uSway * ${glsl(DECO_SWAY)} * vxTop / vxScale;
transformed.z += cos(uTime * 1.3 + vxOrigin.x * 0.21 + vxOrigin.z * 0.41) * uSway * ${glsl(DECO_SWAY)} * 0.6 * vxTop / vxScale;
vNy = normal.y;
vAo = 1.0;
vSeed = fract(sin(dot(vxOrigin.xz, vec2(12.9898, 78.233))) * 43758.5453);
vTUv = uv;
vWorld = (modelMatrix * vec4(vxOrigin, 1.0)).xyz;
`;
        }
        let body = '\nvNy = normal.y;\nvAo = aAo;\nvSeed = aSeed;\nvTUv = uv;\n';
        if (kind === 'glow') body += 'vGlow = aGlow;\n';
        if (kind === 'foliage') {
            // The displacement depends only on the world position, so corners shared by two blocks stay together.
            body += `vec3 vxWp = (modelMatrix * vec4(transformed, 1.0)).xyz;
transformed.x += sin(uTime * 1.6 + vxWp.x * 0.37 + vxWp.z * 0.23) * uSway;
transformed.z += cos(uTime * 1.3 + vxWp.x * 0.21 + vxWp.z * 0.41) * uSway * 0.6;
`;
        }
        if (kind === 'water') {
            body += `vec3 vxWp = (modelMatrix * vec4(transformed, 1.0)).xyz;
transformed.y += sin(uTime * 1.2 + vxWp.x * 0.6) * cos(uTime * 0.9 + vxWp.z * 0.5) * ${glsl(WATER_WAVE)} * step(${glsl(WATER_SURFACE_Y)}, vxWp.y);
`;
        }
        return body + 'vWorld = (modelMatrix * vec4(transformed, 1.0)).xyz;\n';
    }

    function fragmentColor(kind) {
        if (kind === 'water') {
            return `
diffuseColor.rgb *= 1.0 + (vxHash(floor(vWorld.xz * 8.0 - vec2(uTime * 2.0, 0.0))) - 0.5) * ${glsl(TEXEL_WATER)};
diffuseColor.rgb *= mix(${glsl(AO_FLOOR)}, 1.0, pow(vAo, ${glsl(AO_POWER)}));
${CLOUD_SHADOW}`;
        }
        if (kind === 'deco') {
            return `
diffuseColor.rgb *= 1.0 + (vxHash(floor(vTUv * 8.0) + vSeed * 91.7) - 0.5) * uTexel;
${CLOUD_SHADOW}`;
        }
        return `
diffuseColor.rgb *= 1.0 + (vxHash(floor(vTUv * 8.0) + vSeed * 91.7) - 0.5) * uTexel;
diffuseColor.rgb *= mix(${glsl(AO_FLOOR)}, 1.0, pow(vAo, ${glsl(AO_POWER)}));
vec2 vxRim = abs(vTUv - 0.5);
diffuseColor.rgb *= 1.0 - ${glsl(EDGE_SHADE)} * smoothstep(${glsl(EDGE_START)}, 0.5, max(vxRim.x, vxRim.y));
${CLOUD_SHADOW}`;
    }

    function fragmentEmissive(kind) {
        if (kind === 'glow') {
            return '\ntotalEmissiveRadiance += vGlow * uNight * (' + glsl(GLOW_BASE) + ' + ' + glsl(GLOW_FLICKER) + ' * sin(uTime * 7.0 + vSeed * 40.0));\n';
        }
        if (kind === 'water') {
            return `
float vxSparkle = pow(vxHash(floor(vWorld.xz * ${glsl(SPARKLE_CELLS)}) + floor(uTime * 3.0)), ${glsl(SPARKLE_POWER)}) * uSun * step(0.5, vNy);
totalEmissiveRadiance += vec3(vxSparkle) * ${glsl(SPARKLE_GAIN)};
`;
        }
        return '';
    }

    // kind: 'solid' | 'foliage' | 'glow' | 'water' (per-vertex aAo, aSeed, uv; glow adds aGlow) or 'deco'
    // (instanced cubes with instance colours). uniforms is the scene's shared uniform object:
    // { uTime, uSway, uNight, uCloudOffset, uCloudShadow, uSun, uTexel }; shaders share the very objects.
    function patchTerrainMaterial(THREE, material, uniforms, kind) {
        material.onBeforeCompile = shader => {
            for (const name of ['uTime', 'uSway', 'uNight', 'uCloudOffset', 'uCloudShadow', 'uSun', 'uTexel']) shader.uniforms[name] = uniforms[name];
            shader.vertexShader = shader.vertexShader
                .replace('#include <common>', '#include <common>\n' + vertexHead(kind))
                .replace('#include <begin_vertex>', '#include <begin_vertex>' + vertexBody(kind));
            shader.fragmentShader = shader.fragmentShader
                .replace('#include <common>', '#include <common>\n' + FRAGMENT_HEAD + (kind === 'glow' ? 'varying vec3 vGlow;\n' : ''))
                .replace('#include <color_fragment>', '#include <color_fragment>' + fragmentColor(kind));
            const emissive = fragmentEmissive(kind);
            if (emissive) shader.fragmentShader = shader.fragmentShader.replace('#include <emissivemap_fragment>', '#include <emissivemap_fragment>' + emissive);
        };
        // Three keys its program cache on the source of onBeforeCompile, which is the same for every kind.
        material.customProgramCacheKey = () => 'voxel-terrain-' + kind;
        material.needsUpdate = true;
        return material;
    }

    window.AuraVoxelBlocks = Object.freeze({ PATTERNS, buildAtlas, patchAtlasMaterial, patchTerrainMaterial, FRAME_PATTERNS });
})();
