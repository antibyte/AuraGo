(function (root) {
    'use strict';

    const encoder = new TextEncoder();
    let crcTable = null;

    function codedError(code, message) {
        const error = new Error(message || code);
        error.code = code;
        return error;
    }

    function crc32(bytes) {
        if (!crcTable) {
            crcTable = new Uint32Array(256);
            for (let n = 0; n < 256; n++) {
                let value = n;
                for (let bit = 0; bit < 8; bit++) value = value & 1 ? 0xedb88320 ^ (value >>> 1) : value >>> 1;
                crcTable[n] = value >>> 0;
            }
        }
        let crc = 0xffffffff;
        for (let i = 0; i < bytes.length; i++) crc = (crc >>> 8) ^ crcTable[(crc ^ bytes[i]) & 0xff];
        return (crc ^ 0xffffffff) >>> 0;
    }

    function dosStamp(date) {
        const year = Math.min(2107, Math.max(1980, date.getFullYear()));
        return {
            time: (date.getHours() << 11) | (date.getMinutes() << 5) | Math.floor(date.getSeconds() / 2),
            date: ((year - 1980) << 9) | ((date.getMonth() + 1) << 5) | date.getDate()
        };
    }

    function sanitize(name) {
        const value = String(name || 'Track').replace(/[<>:"/\\|?*\u0000-\u001f]/g, '_').replace(/\s+/g, ' ').trim().slice(0, 96);
        return value || 'Track';
    }

    function localHeader(entry, stamp) {
        const header = new Uint8Array(30 + entry.nameBytes.length);
        const view = new DataView(header.buffer);
        view.setUint32(0, 0x04034b50, true);
        view.setUint16(4, 20, true);
        view.setUint16(6, 0x0800, true);
        view.setUint16(8, 0, true);
        view.setUint16(10, stamp.time, true);
        view.setUint16(12, stamp.date, true);
        view.setUint32(14, entry.crc, true);
        view.setUint32(18, entry.size, true);
        view.setUint32(22, entry.size, true);
        view.setUint16(26, entry.nameBytes.length, true);
        view.setUint16(28, 0, true);
        header.set(entry.nameBytes, 30);
        return header;
    }

    function centralHeader(entry, stamp) {
        const header = new Uint8Array(46 + entry.nameBytes.length);
        const view = new DataView(header.buffer);
        view.setUint32(0, 0x02014b50, true);
        view.setUint16(4, 20, true);
        view.setUint16(6, 20, true);
        view.setUint16(8, 0x0800, true);
        view.setUint16(10, 0, true);
        view.setUint16(12, stamp.time, true);
        view.setUint16(14, stamp.date, true);
        view.setUint32(16, entry.crc, true);
        view.setUint32(20, entry.size, true);
        view.setUint32(24, entry.size, true);
        view.setUint16(28, entry.nameBytes.length, true);
        view.setUint16(30, 0, true);
        view.setUint16(32, 0, true);
        view.setUint16(34, 0, true);
        view.setUint16(36, 0, true);
        view.setUint32(38, 0, true);
        view.setUint32(42, entry.offset, true);
        header.set(entry.nameBytes, 46);
        return header;
    }

    function endRecord(count, size, offset) {
        const end = new Uint8Array(22);
        const view = new DataView(end.buffer);
        view.setUint32(0, 0x06054b50, true);
        view.setUint16(4, 0, true);
        view.setUint16(6, 0, true);
        view.setUint16(8, count, true);
        view.setUint16(10, count, true);
        view.setUint32(12, size, true);
        view.setUint32(16, offset, true);
        view.setUint16(20, 0, true);
        return end;
    }

    // Store-only ZIP: one WAV per entry, no compression, UTF-8 names. The audio
    // blobs stay blobs, so a multi-stem export never concatenates the WAVs in JS.
    async function zip(files) {
        if (!Array.isArray(files) || !files.length) throw codedError('SYNTH_STUDIO_EXPORT_EMPTY', 'There are no files to archive.');
        const stamp = dosStamp(new Date());
        const entries = [];
        for (const file of files) {
            const blob = file.blob || new Blob([file.data], { type: 'application/octet-stream' });
            const bytes = new Uint8Array(await blob.arrayBuffer());
            const name = sanitize(file.name);
            entries.push({ blob, nameBytes: encoder.encode(name), size: bytes.length, crc: crc32(bytes) });
        }
        const parts = [];
        let offset = 0;
        for (const entry of entries) {
            const header = localHeader(entry, stamp);
            entry.offset = offset;
            parts.push(header, entry.blob);
            offset += header.length + entry.size;
        }
        const centralParts = [];
        let centralSize = 0;
        for (const entry of entries) {
            const header = centralHeader(entry, stamp);
            centralParts.push(header);
            centralSize += header.length;
        }
        const centralOffset = offset;
        const end = endRecord(entries.length, centralSize, centralOffset);
        return new Blob([...parts, ...centralParts, end], { type: 'application/zip' });
    }

    function stemProject(project, trackId) {
        const Model = root.SynthStudioModel;
        if (!Model) throw codedError('SYNTH_STUDIO_EXPORT_UNAVAILABLE', 'The Synth Studio model is unavailable.');
        const stem = Model.clone(project);
        stem.tracks = stem.tracks.map(track => ({ ...track, mute: track.id !== trackId, solo: track.id === trackId }));
        stem.master = { ...stem.master, volume: 1, low: 0, mid: 0, high: 0, threshold: 0, ratio: 1, knee: 0, width: 1 };
        stem.loop = { ...stem.loop, enabled: false };
        return Model.validate(stem);
    }

    async function renderStems(project, options = {}) {
        const audio = root.SynthStudioAudio;
        if (!audio || typeof audio.renderWav !== 'function') throw codedError('SYNTH_STUDIO_EXPORT_UNAVAILABLE', 'WAV rendering is unavailable.');
        const tracks = project.tracks.filter(track => track.clips.some(clip => clip.notes.length));
        if (!tracks.length) throw codedError('SYNTH_STUDIO_EXPORT_EMPTY', 'The project has no notes to export.');
        const files = [];
        for (let index = 0; index < tracks.length; index++) {
            const track = tracks[index];
            const blob = await audio.renderWav(stemProject(project, track.id));
            files.push({ name: String(index + 1).padStart(2, '0') + ' ' + sanitize(track.name) + '.wav', blob });
            options.onProgress?.(index + 1, tracks.length, track.name);
        }
        return files;
    }

    async function exportStems(project, options = {}) {
        return zip(await renderStems(project, options));
    }

    root.SynthStudioExport = { crc32, zip, renderStems, exportStems, stemProject, sanitize };
})(typeof window !== 'undefined' ? window : globalThis);
