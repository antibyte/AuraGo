(function () {
    'use strict';

    // Reads ID3 metadata (tags plus embedded cover art) straight from the
    // audio file. Only the first 256 KB are requested, so a long track does
    // not pull the whole file into the browser.

    const FETCH_LIMIT = 262144;
    const TEXT_FRAMES_V3 = { TIT2: 'title', TPE1: 'artist', TPE2: 'albumArtist', TALB: 'album', TYER: 'year', TDRC: 'year', TCON: 'genre' };
    const TEXT_FRAMES_V2 = { TT2: 'title', TP1: 'artist', TP2: 'albumArtist', TAL: 'album', TYE: 'year', TCO: 'genre' };

    function latin1(bytes, start, length) {
        let out = '';
        for (let i = start; i < start + length && i < bytes.length; i++) out += String.fromCharCode(bytes[i]);
        return out;
    }

    function readUint24BE(bytes, offset) {
        return (bytes[offset] << 16) | (bytes[offset + 1] << 8) | bytes[offset + 2];
    }

    function readUint32BE(bytes, offset) {
        return ((bytes[offset] << 24) | (bytes[offset + 1] << 16) | (bytes[offset + 2] << 8) | bytes[offset + 3]) >>> 0;
    }

    function readSyncSafe(bytes, offset) {
        return ((bytes[offset] & 0x7f) << 21) | ((bytes[offset + 1] & 0x7f) << 14) | ((bytes[offset + 2] & 0x7f) << 7) | (bytes[offset + 3] & 0x7f);
    }

    function decodeWith(label, bytes) {
        try {
            return new TextDecoder(label).decode(bytes);
        } catch (_) {
            return '';
        }
    }

    function decodeText(bytes, encoding) {
        if (!bytes || !bytes.length) return '';
        if (encoding === 1) {
            if (bytes.length >= 2 && bytes[0] === 0xff && bytes[1] === 0xfe) return decodeWith('utf-16le', bytes.subarray(2));
            if (bytes.length >= 2 && bytes[0] === 0xfe && bytes[1] === 0xff) return decodeWith('utf-16be', bytes.subarray(2));
            return decodeWith('utf-16le', bytes);
        }
        if (encoding === 2) return decodeWith('utf-16be', bytes);
        if (encoding === 3) return decodeWith('utf-8', bytes);
        return decodeWith('iso-8859-1', bytes);
    }

    function cleanText(value) {
        return String(value || '').replace(/\u0000+$/g, '').replace(/\s+/g, ' ').trim();
    }

    function findTerminator(bytes, start, end, encoding) {
        if (encoding === 1 || encoding === 2) {
            for (let i = start; i + 1 < end; i += 2) {
                if (bytes[i] === 0 && bytes[i + 1] === 0) return i;
            }
            return end;
        }
        for (let i = start; i < end; i++) {
            if (bytes[i] === 0) return i;
        }
        return end;
    }

    function toBase64(bytes) {
        let binary = '';
        const chunk = 0x8000;
        for (let i = 0; i < bytes.length; i += chunk) {
            binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunk));
        }
        try {
            return btoa(binary);
        } catch (_) {
            return '';
        }
    }

    function deUnsynchronise(bytes) {
        const out = new Uint8Array(bytes.length);
        let index = 0;
        for (let i = 0; i < bytes.length; i++) {
            out[index++] = bytes[i];
            if (bytes[i] === 0xff && bytes[i + 1] === 0x00) i++;
        }
        return out.subarray(0, index);
    }

    function parsePicture(bytes, start, end, version) {
        let offset = start;
        if (offset >= end) return null;
        const encoding = bytes[offset++];
        let mime = 'image/jpeg';
        if (version === 2) {
            const format = latin1(bytes, offset, 3).toLowerCase();
            offset += 3;
            mime = format === 'png' ? 'image/png' : 'image/jpeg';
        } else {
            const mimeEnd = findTerminator(bytes, offset, end, 0);
            const raw = latin1(bytes, offset, mimeEnd).trim();
            if (raw) mime = raw.indexOf('/') > 0 ? raw : 'image/' + raw.toLowerCase();
            offset = mimeEnd + 1;
        }
        offset += 1; // picture type
        const descriptionEnd = findTerminator(bytes, offset, end, encoding);
        offset = descriptionEnd + ((encoding === 1 || encoding === 2) ? 2 : 1);
        if (offset >= end) return null;
        const data = bytes.subarray(offset, end);
        if (!data.length) return null;
        const base64 = toBase64(data);
        if (!base64) return null;
        return { mime, dataUrl: 'data:' + mime + ';base64,' + base64 };
    }

    function parseFrames(bytes, start, end, version) {
        const tags = {};
        let picture = null;
        const mapping = version === 2 ? TEXT_FRAMES_V2 : TEXT_FRAMES_V3;
        let offset = start;
        while (offset + 6 <= end) {
            const id = latin1(bytes, offset, version === 2 ? 3 : 4);
            if (!/^[A-Z0-9]{3,4}$/.test(id)) break;
            let size;
            let headerSize;
            if (version === 2) {
                size = readUint24BE(bytes, offset + 3);
                headerSize = 6;
            } else {
                size = version === 4 ? readSyncSafe(bytes, offset + 4) : readUint32BE(bytes, offset + 4);
                headerSize = 10;
            }
            if (!size) {
                offset += headerSize;
                continue;
            }
            let dataStart = offset + headerSize;
            let frameData = null;
            if (version >= 3) {
                const frameFlags = (bytes[offset + 8] << 8) | bytes[offset + 9];
                // v2.4 flags: compression 0x0400, encryption 0x0200, unsync 0x0100, data length 0x0008.
                // v2.3 flags: compression 0x0080, encryption 0x0040.
                const compressed = version === 4 ? (frameFlags & 0x0400) : (frameFlags & 0x0080);
                const encrypted = version === 4 ? (frameFlags & 0x0200) : (frameFlags & 0x0040);
                const hasDataLength = version === 4 ? (frameFlags & 0x0008) : 0;
                const frameUnsynced = version === 4 ? (frameFlags & 0x0100) : 0;
                if (compressed || encrypted) {
                    offset = dataStart + size;
                    continue;
                }
                if (hasDataLength) dataStart += 4;
                frameData = bytes.subarray(dataStart, Math.min(end, dataStart + size));
                if (frameUnsynced) frameData = deUnsynchronise(frameData);
            }
            const dataEnd = Math.min(end, dataStart + size);
            if (dataEnd <= dataStart) {
                offset = dataStart + size;
                continue;
            }
            const payload = frameData || bytes.subarray(dataStart, dataEnd);
            const field = mapping[id];
            if (field && !tags[field]) {
                const encoding = payload[0];
                tags[field] = cleanText(decodeText(payload.subarray(1), encoding));
            } else if ((id === 'APIC' || id === 'PIC') && !picture) {
                picture = parsePicture(payload, 0, payload.length, version);
            }
            offset = dataStart + size;
        }
        return { tags, picture };
    }

    function parseID3v2(bytes) {
        if (bytes.length < 10) return null;
        if (bytes[0] !== 0x49 || bytes[1] !== 0x44 || bytes[2] !== 0x33) return null; // 'ID3'
        const version = bytes[3];
        if (version < 2 || version > 4) return null;
        const flags = bytes[5];
        const size = readSyncSafe(bytes, 6);
        const total = Math.min(bytes.length, 10 + size);
        let body = bytes.subarray(10, total);
        if (flags & 0x80) body = deUnsynchronise(body);
        let offset = 0;
        if (flags & 0x40) { // extended header
            if (body.length < 4) return null;
            const extendedSize = version === 4 ? readSyncSafe(body, 0) : readUint32BE(body, 0);
            offset = 4 + extendedSize;
        }
        if (offset >= body.length) return { tags: {}, picture: null };
        const parsed = parseFrames(body, offset, body.length, version);
        return { tags: parsed.tags, picture: parsed.picture };
    }

    function parseID3v1(bytes) {
        if (bytes.length < 128) return null;
        const tag = latin1(bytes, bytes.length - 128, 3);
        if (tag !== 'TAG') return null;
        const slice = (start, length) => cleanText(latin1(bytes, bytes.length - 128 + start, length));
        const tags = {};
        const title = slice(3, 30);
        const artist = slice(33, 30);
        const album = slice(63, 30);
        const year = slice(93, 4);
        if (title) tags.title = title;
        if (artist) tags.artist = artist;
        if (album) tags.album = album;
        if (year) tags.year = year;
        return { tags, picture: null };
    }

    function totalSizeFromRange(response) {
        const header = response.headers.get('content-range') || '';
        const match = /\/\s*(\d+)\s*$/.exec(header);
        return match ? parseInt(match[1], 10) : 0;
    }

    async function read(url, options) {
        const opts = options || {};
        if (!url) return null;
        let response;
        try {
            response = await fetch(url, {
                credentials: 'same-origin',
                signal: opts.signal,
                headers: { Range: 'bytes=0-' + (FETCH_LIMIT - 1) }
            });
        } catch (_) {
            return null;
        }
        if (!response.ok && response.status !== 206) return null;
        let buffer;
        try {
            buffer = await response.arrayBuffer();
        } catch (_) {
            return null;
        }
        const bytes = new Uint8Array(buffer);
        const v2 = parseID3v2(bytes);
        const tags = Object.assign({}, v2 ? v2.tags : {});
        let picture = v2 ? v2.picture : null;
        if (!tags.title) {
            const total = totalSizeFromRange(response);
            if (total > FETCH_LIMIT) {
                const v1 = await readID3v1(url, total, opts.signal);
                if (v1) Object.assign(tags, v1.tags);
            }
        }
        if (!tags.title && !tags.artist && !picture) return null;
        return { tags, picture };
    }

    async function readID3v1(url, total, signal) {
        try {
            const response = await fetch(url, {
                credentials: 'same-origin',
                signal,
                headers: { Range: 'bytes-' + Math.max(0, total - 128) + '-' + (total - 1) }
            });
            if (!response.ok && response.status !== 206) return null;
            const buffer = await response.arrayBuffer();
            return parseID3v1(new Uint8Array(buffer));
        } catch (_) {
            return null;
        }
    }

    window.AudioPlayerTags = Object.freeze({ read, parseID3v2, parseID3v1 });
})();
