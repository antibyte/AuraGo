(function () {
    'use strict';

    const IPTV_API_BASE = 'https://iptv-org.github.io/api';
    const CHANNELS_ENDPOINT = IPTV_API_BASE + '/channels.json';
    const STREAMS_ENDPOINT = IPTV_API_BASE + '/streams.json';
    const CATEGORIES_ENDPOINT = IPTV_API_BASE + '/categories.json';
    const catalogCache = window.TeeVeeCatalogCache || (window.TeeVeeCatalogCache = { promise: null, data: null, loadedAt: 0 });
    const { clean, cleanID, normalizeSearch, hashString } = window.AuraDesktopMediaHelpers;

    async function fetchCatalog(force) {
        const now = Date.now();
        if (!force && catalogCache.data && now - catalogCache.loadedAt < 1000 * 60 * 30) return catalogCache.data;
        if (!force && catalogCache.promise) return catalogCache.promise;
        catalogCache.promise = Promise.all([
            fetchJSON(CHANNELS_ENDPOINT, force ? 'no-store' : 'force-cache'),
            fetchJSON(STREAMS_ENDPOINT, force ? 'no-store' : 'force-cache'),
            fetchJSON(CATEGORIES_ENDPOINT, force ? 'no-store' : 'force-cache')
        ]).then(([channels, streams, categories]) => {
            const joined = joinStreamsWithChannels(channels, streams, categories);
            const data = {
                entries: joined.entries,
                countries: joined.countries,
                loadedAt: Date.now()
            };
            catalogCache.data = data;
            catalogCache.loadedAt = data.loadedAt;
            return data;
        }).finally(() => {
            catalogCache.promise = null;
        });
        return catalogCache.promise;
    }

    async function fetchJSON(url, cacheMode) {
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 20000);
        try {
            const response = await fetch(url, { cache: cacheMode || 'force-cache', signal: controller.signal });
            clearTimeout(timeout);
            if (!response.ok) throw new Error('iptv-org HTTP');
            return response.json();
        } catch (err) {
            clearTimeout(timeout);
            if (err && err.name === 'AbortError') throw new Error('iptv-org HTTP');
            throw err;
        }
    }

    function joinStreamsWithChannels(channels, streams, categories) {
        const channelByID = new Map((Array.isArray(channels) ? channels : [])
            .filter(channel => channel && channel.id)
            .map(channel => [channel.id, channel]));
        const categoryByID = new Map((Array.isArray(categories) ? categories : [])
            .filter(category => category && category.id)
            .map(category => [category.id, category.name || category.id]));
        const rows = Array.isArray(streams) ? streams : [];
        const countries = new Set();
        const entries = rows.map((stream, index) => {
            if (!stream || !stream.url) return null;
            const channel = channelByID.get(stream.channel || '') || null;
            const channelCategories = Array.isArray(channel && channel.categories) ? channel.categories.map(cleanID).filter(Boolean) : [];
            const country = clean(channel && channel.country).toUpperCase();
            if (/^[A-Z]{2}$/.test(country)) countries.add(country);
            const name = clean(channel && channel.name) || clean(stream.title) || stream.url;
            const entry = {
                id: stableID(stream.channel, stream.url),
                favoriteKey: stableID(stream.channel, stream.url),
                channelID: clean(stream.channel),
                name,
                url: clean(stream.url),
                country,
                language: Array.isArray(channel && channel.languages) ? channel.languages.map(clean).join(', ').toUpperCase() : '',
                categories: channelCategories,
                categoryNames: channelCategories.map(id => categoryByID.get(id) || id),
                quality: clean(stream.quality || stream.label),
                label: clean(stream.label),
                resolutionBucket: resolutionBucketFromStream(stream),
                logo: clean(channel && channel.logo),
                website: clean(channel && channel.website),
                isNsfw: !!(channel && channel.is_nsfw),
                closed: clean(channel && channel.closed),
                replacedBy: clean(channel && channel.replaced_by),
                unsupported: isUnsupportedStream(stream)
            };
            return entry;
        }).filter(entry => entry && entry.url && !entry.isNsfw && !entry.closed && !entry.replacedBy)
            .sort(sortEntries);
        return { entries, countries };
    }

    function stableID(channelID, url) {
        const id = clean(channelID);
        const streamURL = clean(url);
        return (id || 'unknown') + ':' + hashString(streamURL || id || 'stream');
    }

    function isUnsupportedStream(stream) {
        return !!(stream && (stream.user_agent || stream.referrer));
    }

    function sortEntries(a, b) {
        if (a.country === 'DE' && b.country !== 'DE') return -1;
        if (a.country !== 'DE' && b.country === 'DE') return 1;
        return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
    }

    function searchableText(entry) {
        return normalizeSearch([
            entry.name,
            entry.country,
            entry.quality,
            entry.label,
            entry.resolutionBucket,
            entry.channelID,
            entry.categories.join(' '),
            entry.categoryNames.join(' ')
        ].join(' '));
    }

    function categoryText(entry) {
        return (entry.categoryNames || []).slice(0, 2).join(', ');
    }

    function qualityText(entry) {
        return clean(entry.quality || entry.label) || '';
    }

    function resolutionText(entry) {
        return qualityText(entry) || resolutionFallbackLabel(entry && entry.resolutionBucket);
    }

    function resolutionBucketFromStream(stream) {
        const text = [
            stream && stream.quality,
            stream && stream.label,
            stream && stream.url
        ].map(clean).join(' ').toLowerCase();
        if (/(2160p?|4k|uhd|ultra\s*hd)/.test(text)) return 'uhd';
        if (/(1080p?|fhd|full\s*hd)/.test(text)) return 'fhd';
        if (/(720p?|hd)/.test(text)) return 'hd';
        if (/(576p?|540p?|480p?|360p?|240p?|sd)/.test(text)) return 'sd';
        return 'unknown';
    }

    function resolutionFallbackLabel(bucket) {
        switch (clean(bucket)) {
            case 'uhd': return 'UHD / 4K';
            case 'fhd': return '1080p';
            case 'hd': return '720p';
            case 'sd': return 'SD';
            default: return '';
        }
    }

    window.AuraTeeVeeCatalog = { fetchCatalog, searchableText, categoryText, resolutionText };
})();
