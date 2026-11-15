// Only self-contained, catalog-verified GLBs enter the isolated player. No model URL is ever fetched here.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    function glb(data) {
        if (Object.prototype.toString.call(data) !== '[object ArrayBuffer]' || data.byteLength < 28 || data.byteLength > SM.textures.limit) throw new Error('Invalid model size');
        var v = new DataView(data), length = v.getUint32(12, true), start = 28 + length;
        if (v.getUint32(0, true) !== 0x46546c67 || v.getUint32(4, true) !== 2 || v.getUint32(8, true) !== data.byteLength || v.getUint32(16, true) !== 0x4e4f534a || length > 1048576 || start > data.byteLength || length % 4) throw new Error('Invalid GLB header');
        if (v.getUint32(24 + length, true) !== 0x004e4942 || start + v.getUint32(20 + length, true) !== data.byteLength) throw new Error('Invalid GLB binary chunk');
        var json = JSON.parse(new TextDecoder().decode(new Uint8Array(data, 20, length))), binary = data.slice(start);
        if (json.asset.version !== '2.0' || json.buffers.length !== 1 || json.buffers[0].byteLength > binary.byteLength || json.buffers[0].byteLength < binary.byteLength - 3) throw new Error('Invalid model buffer');
        var pending = [json], fields = 0;
        while (pending.length) {
            var item = pending.pop();
            Object.keys(item).forEach(function (key) {
                if (++fields > 50000 || key === 'uri' || key === '__proto__' || key === 'extensionsRequired') throw new Error('External or unsupported model resource');
                if (typeof item[key] === 'number' && !Number.isFinite(item[key])) throw new Error('Invalid model number');
                if (item[key] && typeof item[key] === 'object') pending.push(item[key]);
            });
        }
        if (!json.nodes || json.nodes.length > 128 || !json.meshes || json.meshes.length > 16 || !json.skins || json.skins.length > 4 || (json.images || []).length > 8 || (json.accessors || []).length > 512 || (json.bufferViews || []).length > 512) throw new Error('Model exceeds structural limits');
        json.bufferViews.forEach(function (b) {
            if (b.buffer !== 0 || !Number.isInteger(b.byteLength) || b.byteLength < 1 || !Number.isInteger(b.byteOffset || 0) || (b.byteOffset || 0) < 0 || (b.byteOffset || 0) + b.byteLength > binary.byteLength) throw new Error('Invalid model buffer view');
        });
        var elements = 0;
        function span(view, offset, count, size, stride) {
            if (!view || !Number.isInteger(offset) || offset < 0 || !Number.isInteger(stride) || stride < size || stride > 252 || offset + (count-1)*stride + size > view.byteLength) throw new Error('Invalid model accessor span');
        }
        json.accessors.forEach(function (a) {
            if (!Number.isInteger(a.count) || a.count < 1 || a.count > 180000 || ![5120,5121,5122,5123,5125,5126].includes(a.componentType) || !['SCALAR','VEC2','VEC3','VEC4','MAT4'].includes(a.type)) throw new Error('Invalid model accessor');
            elements += a.count;
            if (a.bufferView !== undefined && !json.bufferViews[a.bufferView]) throw new Error('Invalid model accessor view');
            if (a.sparse && (!Number.isInteger(a.sparse.count) || a.sparse.count < 1 || a.sparse.count > a.count || !json.bufferViews[a.sparse.indices.bufferView] || !json.bufferViews[a.sparse.values.bufferView])) throw new Error('Invalid sparse model accessor');
            var size = ({5120:1,5121:1,5122:2,5123:2,5125:4,5126:4})[a.componentType] * ({SCALAR:1,VEC2:2,VEC3:3,VEC4:4,MAT4:16})[a.type];
            if (a.bufferView !== undefined) { var view=json.bufferViews[a.bufferView];span(view,a.byteOffset||0,a.count,size,view.byteStride||size); }
            if (a.sparse) {
                var sparse=a.sparse,indexSize=({5121:1,5123:2,5125:4})[sparse.indices.componentType];
                if(!indexSize)throw new Error('Invalid sparse model index');
                span(json.bufferViews[sparse.indices.bufferView],sparse.indices.byteOffset||0,sparse.count,indexSize,indexSize);
                span(json.bufferViews[sparse.values.bufferView],sparse.values.byteOffset||0,sparse.count,size,size);
            }
        });
        if (elements > 3000000) throw new Error('Model accessor budget exceeded');
        var marks = new Set();
        function visit(index, ancestors) {
            if (!Number.isInteger(index) || !json.nodes[index] || ancestors.has(index)) throw new Error('Invalid model hierarchy');
            if (marks.has(index)) return;
            var next = new Set(ancestors); next.add(index);
            (json.nodes[index].children || []).forEach(function (child) { visit(child, next); }); marks.add(index);
        }
        json.nodes.forEach(function (_, i) { visit(i, new Set()); });
        json.skins.forEach(function (skin) { if (!skin.joints || skin.joints.length > 72 || skin.joints.some(function (i) { return !json.nodes[i]; })) throw new Error('Invalid model skeleton'); });
        var triangles = 0;
        json.meshes.forEach(function (mesh) { mesh.primitives.forEach(function (p) {
            var positions = json.accessors[p.attributes.POSITION], indices = json.accessors[p.indices];
            if (!positions || positions.count > 90000 || p.mode !== undefined && p.mode !== 4 || (p.targets || []).length > 16) throw new Error('Invalid model primitive');
            triangles += (indices ? indices.count : positions.count) / 3;
        }); });
        if (triangles > 60000) throw new Error('Model triangle budget exceeded');
        (json.images || []).forEach(function (img) {
            var view = json.bufferViews[img.bufferView];
            if (!view || !['image/png','image/jpeg'].includes(img.mimeType)) throw new Error('Invalid embedded model image');
            var bytes = binary.slice(view.byteOffset || 0, (view.byteOffset || 0) + view.byteLength), size = SM.textures.imageSize(bytes, img.mimeType);
            if (size[0] < 1 || size[1] < 1 || size[0] > 1024 || size[1] > 1024) throw new Error('Embedded model texture exceeds 1024 pixels');
        });
        return { json: json, binary: binary };
    }
    function dispose(model) {
        var geometries = new Set(), materials = new Set(), textures = new Set(), skeletons = new Set();
        model.traverse(function (o) {
            if (o.geometry) geometries.add(o.geometry);
            if (o.skeleton) skeletons.add(o.skeleton);
            (Array.isArray(o.material) ? o.material : o.material ? [o.material] : []).forEach(function (m) { materials.add(m); Object.values(m).forEach(function (t) { if (t && t.isTexture) textures.add(t); }); });
        });
        geometries.forEach(function (g) { g.dispose(); }); materials.forEach(function (m) { m.dispose(); });
        textures.forEach(function (t) { if (t.image && t.image.close) t.image.close(); t.dispose(); });
        skeletons.forEach(function (s) { s.dispose(); });
    }
    async function load(catalog, path, bytes, current) {
        var meta = SM.textures.index(catalog)[path];
        if (!meta || meta.type !== 'model/gltf-binary' || !bytes || bytes.byteLength !== meta.bytes || SM.textures.sha256(bytes) !== meta.sha256) throw new Error('Model failed integrity verification: ' + path);
        var parsed = glb(bytes), vendor = root.StorymakerVendor, T = vendor.THREE, bitmaps = [];
        var loader = new vendor.GLTFLoader();
        loader.register(function (parser) { return { name: 'STORYMAKER_EMBEDDED_IMAGES', loadTexture: async function (index) {
            var definition = parser.json.textures[index], img = parser.json.images[definition.source], view = parser.json.bufferViews[img.bufferView];
            var buffer = parsed.binary.slice(view.byteOffset || 0, (view.byteOffset || 0) + view.byteLength);
            var bitmap = await root.createImageBitmap(new root.Blob([buffer], { type: img.mimeType }), { colorSpaceConversion: 'none' });
            bitmaps.push(bitmap);
            var expected = SM.textures.imageSize(buffer, img.mimeType);
            if (!current() || bitmap.width !== expected[0] || bitmap.height !== expected[1]) throw new Error('Model image was disposed or has invalid dimensions');
            var texture = new T.Texture(bitmap), sampler = parser.json.samplers && parser.json.samplers[definition.sampler] || {};
            texture.flipY = false; texture.needsUpdate = true;
            texture.wrapS = sampler.wrapS === 33071 ? T.ClampToEdgeWrapping : sampler.wrapS === 33648 ? T.MirroredRepeatWrapping : T.RepeatWrapping;
            texture.wrapT = sampler.wrapT === 33071 ? T.ClampToEdgeWrapping : sampler.wrapT === 33648 ? T.MirroredRepeatWrapping : T.RepeatWrapping;
            return texture;
        } }; });
        try {
            var result = await loader.parseAsync(bytes, '');
            if (!current()) { dispose(result.scene); throw new Error('Project was disposed'); }
            return result.scene;
        } catch (error) { bitmaps.forEach(function (b) { b.close(); }); throw error; }
    }
    SM.models = { glb: glb, load: load, dispose: dispose };
})(typeof globalThis !== 'undefined' ? globalThis : self);
