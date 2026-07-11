// AuraGo Service Worker — cache-first for static assets, network-first for HTML/API.
const CACHE_VERSION = 'aurago-v1';
const STATIC_CACHE = `${CACHE_VERSION}-static`;
const HTML_CACHE = `${CACHE_VERSION}-html`;

// Core shell files to cache immediately on install.
const CORE_ASSETS = [
    '/',
    '/index.html',
    '/site.webmanifest',
    '/favicon.ico',
    '/favicon.svg',
    '/favicon-96x96.png',
    '/apple-touch-icon.png',
    '/aurago_logo_dark.png'
];

const API_PATH_PREFIXES = ['/api/', '/v1/', '/auth/', '/events'];
const STATIC_EXTENSIONS = ['.js', '.css', '.woff', '.woff2', '.ttf', '.otf', '.png', '.jpg', '.jpeg', '.svg', '.gif', '.webp', '.ico', '.webmanifest', '.json'];

function isApiRequest(url) {
    const path = url.pathname;
    return API_PATH_PREFIXES.some(prefix => path.startsWith(prefix));
}

function isStaticAsset(url) {
    const path = url.pathname;
    if (STATIC_EXTENSIONS.some(ext => path.endsWith(ext))) return true;
    if (path.startsWith('/fonts/') || path.startsWith('/img/') || path.startsWith('/css/') || path.startsWith('/js/')) return true;
    return false;
}

function cacheKeyFor(request) {
    // Strip query string for static assets so ?v=... does not fragment the cache.
    const url = new URL(request.url);
    if (isStaticAsset(url)) {
        return `${url.origin}${url.pathname}`;
    }
    return request.url;
}

self.addEventListener('install', event => {
    event.waitUntil(
        caches.open(STATIC_CACHE).then(cache => cache.addAll(CORE_ASSETS)).then(() => self.skipWaiting())
    );
});

self.addEventListener('activate', event => {
    event.waitUntil(
        caches.keys().then(keys =>
            Promise.all(
                keys.filter(key => key.startsWith('aurago-') && key !== STATIC_CACHE && key !== HTML_CACHE)
                    .map(key => caches.delete(key))
            )
        ).then(() => self.clients.claim())
    );
});

self.addEventListener('fetch', event => {
    const { request } = event;
    if (request.method !== 'GET') return;

    const url = new URL(request.url);
    if (isApiRequest(url)) {
        return; // network only
    }

    if (isStaticAsset(url)) {
        event.respondWith(
            caches.open(STATIC_CACHE).then(async cache => {
                const key = cacheKeyFor(request);
                const cached = await cache.match(key);
                if (cached) {
                    // Stale-while-revalidate: return cached version, then update in background.
                    fetch(request).then(response => {
                        if (response && response.ok) {
                            cache.put(key, response.clone());
                        }
                    }).catch(() => {});
                    return cached;
                }
                try {
                    const response = await fetch(request);
                    if (response && response.ok) {
                        cache.put(key, response.clone());
                    }
                    return response;
                } catch (err) {
                    return new Response('Offline', { status: 503, statusText: 'Service Unavailable' });
                }
            })
        );
        return;
    }

    // HTML and everything else: network-first with cache fallback.
    event.respondWith(
        caches.open(HTML_CACHE).then(async cache => {
            try {
                const networkResponse = await fetch(request);
                if (networkResponse && networkResponse.ok) {
                    cache.put(request.url, networkResponse.clone());
                }
                return networkResponse;
            } catch (err) {
                const cached = await cache.match(request.url);
                if (cached) return cached;
                return new Response('Offline', { status: 503, statusText: 'Service Unavailable' });
            }
        })
    );
});
