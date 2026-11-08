// Storymaker texture trust boundary: catalog-only paths, bounded bytes, hashes and decoded dimensions.
(function (root) {
    'use strict';
    var SM = root.StorymakerCore || (root.StorymakerCore = {});
    var LIMIT = 8 * 1048576, TOTAL = 256 * 1048576;
    var K = [0x428a2f98,0x71374491,0xb5c0fbcf,0xe9b5dba5,0x3956c25b,0x59f111f1,0x923f82a4,0xab1c5ed5,
        0xd807aa98,0x12835b01,0x243185be,0x550c7dc3,0x72be5d74,0x80deb1fe,0x9bdc06a7,0xc19bf174,
        0xe49b69c1,0xefbe4786,0x0fc19dc6,0x240ca1cc,0x2de92c6f,0x4a7484aa,0x5cb0a9dc,0x76f988da,
        0x983e5152,0xa831c66d,0xb00327c8,0xbf597fc7,0xc6e00bf3,0xd5a79147,0x06ca6351,0x14292967,
        0x27b70a85,0x2e1b2138,0x4d2c6dfc,0x53380d13,0x650a7354,0x766a0abb,0x81c2c92e,0x92722c85,
        0xa2bfe8a1,0xa81a664b,0xc24b8b70,0xc76c51a3,0xd192e819,0xd6990624,0xf40e3585,0x106aa070,
        0x19a4c116,0x1e376c08,0x2748774c,0x34b0bcb5,0x391c0cb3,0x4ed8aa4a,0x5b9cca4f,0x682e6ff3,
        0x748f82ee,0x78a5636f,0x84c87814,0x8cc70208,0x90befffa,0xa4506ceb,0xbef9a3f7,0xc67178f2];
    function rotate(v, n) { return (v >>> n) | (v << (32 - n)); }
    function sha256(input) {
        var bytes = new Uint8Array(input), length = Math.ceil((bytes.length + 9) / 64) * 64;
        var padded = new Uint8Array(length); padded.set(bytes); padded[bytes.length] = 128;
        var view = new DataView(padded.buffer), w = new Uint32Array(64);
        view.setUint32(length - 8, Math.floor(bytes.length / 0x20000000));
        view.setUint32(length - 4, (bytes.length * 8) >>> 0);
        var h = new Uint32Array([0x6a09e667,0xbb67ae85,0x3c6ef372,0xa54ff53a,0x510e527f,0x9b05688c,0x1f83d9ab,0x5be0cd19]);
        for (var at = 0; at < length; at += 64) {
            var i;
            for (i = 0; i < 16; i++) w[i] = view.getUint32(at + i * 4);
            for (i = 16; i < 64; i++) {
                var x = w[i - 15], y = w[i - 2];
                w[i] = w[i - 16] + (rotate(x, 7) ^ rotate(x, 18) ^ (x >>> 3)) + w[i - 7] + (rotate(y, 17) ^ rotate(y, 19) ^ (y >>> 10));
            }
            var a=h[0],b=h[1],c=h[2],d=h[3],e=h[4],f=h[5],g=h[6],j=h[7];
            for (i = 0; i < 64; i++) {
                var t = (j + (rotate(e,6)^rotate(e,11)^rotate(e,25)) + ((e&f)^(~e&g)) + K[i] + w[i]) | 0;
                var s = ((rotate(a,2)^rotate(a,13)^rotate(a,22)) + ((a&b)^(a&c)^(b&c))) | 0;
                j=g;g=f;f=e;e=(d+t)|0;d=c;c=b;b=a;a=(t+s)|0;
            }
            h[0]+=a;h[1]+=b;h[2]+=c;h[3]+=d;h[4]+=e;h[5]+=f;h[6]+=g;h[7]+=j;
        }
        return Array.from(h, function (v) { return v.toString(16).padStart(8, '0'); }).join('');
    }
    function safePath(p) { return typeof p === 'string' && /^[A-Za-z0-9_./-]{1,230}$/.test(p) && p[0] !== '/' && p.split('/').every(function (s) { return s && s !== '.' && s !== '..'; }); }
    function imageSize(data, type) {
        var b = new Uint8Array(data), v = new DataView(data);
        if (type === 'image/png' && b.length >= 33 && v.getUint32(0) === 0x89504e47 && v.getUint32(4) === 0x0d0a1a0a && v.getUint32(12) === 0x49484452) return [v.getUint32(16),v.getUint32(20)];
        if (type === 'image/webp' && b.length >= 30 && v.getUint32(0) === 0x52494646 && v.getUint32(8) === 0x57454250 && v.getUint32(4,true)+8 === b.length) {
            var tag = v.getUint32(12);
            if (tag === 0x56503858) return [1+b[24]+b[25]*256+b[26]*65536,1+b[27]+b[28]*256+b[29]*65536];
            if (tag === 0x5650384c && b[20] === 0x2f) return [1+(b[21]|((b[22]&63)<<8)),1+((b[22]>>>6)|(b[23]<<2)|((b[24]&15)<<10))];
            if (tag === 0x56503820 && b[23] === 0x9d && b[24] === 1 && b[25] === 0x2a) return [v.getUint16(26,true)&16383,v.getUint16(28,true)&16383];
        }
        if (type === 'image/jpeg' && b.length > 4 && b[0] === 255 && b[1] === 216) {
            for (var at=2;at+4<b.length;) {
                if(b[at++]!==255) break;
                while(at<b.length&&b[at]===255)at++;
                var marker=b[at++];if(marker===217||marker===218)break;
                var length=v.getUint16(at);if(length<2||at+length>b.length)break;
                if([192,193,194,195,197,198,199,201,202,203,205,206,207].indexOf(marker)>=0&&length>=7)return [v.getUint16(at+5),v.getUint16(at+3)];
                at+=length;
            }
        }
        throw new Error('Invalid encoded image header');
    }
    function index(catalog) {
        var files = Object.create(null), total = 0;
        (catalog.files || []).forEach(function (f) {
            if (!safePath(f.path) || f.path === 'catalog.json' || files[f.path] || !Number.isInteger(f.bytes) || f.bytes < 1 || f.bytes > LIMIT || !/^[a-f0-9]{64}$/.test(f.sha256)) throw new Error('Invalid catalog dependency');
            if (['image/png','image/webp','image/jpeg','application/json','audio/wav'].indexOf(f.type) < 0) throw new Error('Invalid catalog media type');
            total += f.bytes; files[f.path] = f;
            var background = f.path.indexOf('textures/environments/') === 0 && f.path.split('/').pop().indexOf('foreground-') !== 0;
            var maxW = background ? 3840 : 2048, maxH = background ? 2160 : 2048;
            if (f.path.indexOf('thumbs/') === 0) { maxW = 256; maxH = 256; }
            if (f.type.indexOf('image/') === 0 && (!Number.isInteger(f.width) || !Number.isInteger(f.height) || f.width < 1 || f.height < 1 || f.width > maxW || f.height > maxH)) throw new Error('Invalid catalog image dimensions');
            if (f.type.indexOf('image/') !== 0 && (f.width || f.height)) throw new Error('Non-image dependency has dimensions');
        });
        if (total > TOTAL || Object.keys(files).length > 2000) throw new Error('Catalog exceeds its delivery budget');
        return files;
    }
    function expand(catalog, needs) {
        if (!catalog || catalog.schema !== 'storymaker-catalog/2') return needs;
        var set = new Set(needs), byFile = Object.create(null), files = index(catalog);
        (catalog.entries || []).forEach(function (e) { if (e.file) byFile[e.file] = e; });
        Array.from(set).forEach(function (key) {
            var e = key.indexOf('catalog:') === 0 && byFile[key.slice(8)];
            if (e) (e.resources || []).forEach(function (p) {
                if (!files[p]) throw new Error('Unlisted catalog resource');
                set.add((files[p].type.indexOf('image/') === 0 ? 'texture:' : 'catalog:') + p);
            });
        });
        return Array.from(set).sort();
    }
    async function decode(catalog, assets, into, current) {
        var files = index(catalog), fresh = [];
        try {
            for (var key of Object.keys(assets)) {
                if (key.indexOf('texture:') !== 0) continue;
                var p = key.slice(8), meta = files[p], data = assets[key];
                if (!meta || meta.type.indexOf('image/') !== 0 || Object.prototype.toString.call(data) !== '[object ArrayBuffer]' || data.byteLength !== meta.bytes || sha256(data) !== meta.sha256) throw new Error('Texture failed integrity verification: ' + p);
                if (into[p]) continue;
                var size = imageSize(data,meta.type);
                if (size[0] !== meta.width || size[1] !== meta.height) throw new Error('Encoded texture dimensions differ: ' + p);
                var bitmap = await root.createImageBitmap(new root.Blob([data], { type: meta.type }));
                if (current && !current()) { bitmap.close(); throw new Error('Project was disposed'); }
                if (bitmap.width !== meta.width || bitmap.height !== meta.height) { bitmap.close(); throw new Error('Texture dimensions differ: ' + p); }
                fresh.push([p, bitmap]);
            }
            fresh.forEach(function (v) { into[v[0]] = v[1]; });
        } catch (err) { fresh.forEach(function (v) { v[1].close(); }); throw err; }
    }
    function dispose(bitmaps) { Object.keys(bitmaps).forEach(function (p) { bitmaps[p].close(); delete bitmaps[p]; }); }
    SM.textures = { sha256: sha256, safePath: safePath, imageSize: imageSize, index: index, expand: expand, decode: decode, dispose: dispose, limit: LIMIT };
})(typeof globalThis !== 'undefined' ? globalThis : self);
