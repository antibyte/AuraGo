// AuraGo Service Worker — versioned static assets and Web Push delivery.
const CACHE_SCHEMA_VERSION = '2';
const SERVICE_WORKER_BUILD = new URL(self.location.href).searchParams.get('v') || 'dev';
const CACHE_VERSION = `aurago-${CACHE_SCHEMA_VERSION}-${SERVICE_WORKER_BUILD}`;
const STATIC_CACHE = `${CACHE_VERSION}-static`;

// Keep the install shell static-only. The cache namespace changes with every
// service-worker build, while request query strings remain part of cache keys.
const CORE_ASSETS = [
    '/site.webmanifest',
    '/favicon.ico',
    '/favicon.svg',
    '/favicon-96x96.png',
    '/apple-touch-icon.png',
    '/aurago_logo_dark.png'
];

const NETWORK_ONLY_PATH_PREFIXES = ['/api/', '/v1/', '/auth/', '/events'];
const STATIC_EXTENSIONS = ['.js', '.css', '.woff', '.woff2', '.ttf', '.otf', '.png', '.jpg', '.jpeg', '.svg', '.gif', '.webp', '.ico', '.webmanifest', '.json', '.wasm', '.glb'];

function isApiRequest(url) {
    return NETWORK_ONLY_PATH_PREFIXES.some(prefix => url.pathname.startsWith(prefix));
}

function isStaticAsset(url) {
    const path = url.pathname.toLowerCase();
    if (STATIC_EXTENSIONS.some(ext => path.endsWith(ext))) return true;
    return path.startsWith('/fonts/') || path.startsWith('/img/') || path.startsWith('/css/') || path.startsWith('/js/');
}

function cacheKeyFor(request) {
    // Preserve the complete versioned URL. Stripping ?v= would serve stale
    // assets when two builds share a service-worker lifecycle.
    return request.url;
}

function normalizeNotificationTarget(rawTarget) {
    try {
        const target = new URL(rawTarget, self.location.origin);
        if (target.origin === self.location.origin) return target;
    } catch (_) { }
    return new URL('/', self.location.origin);
}

self.addEventListener('install', event => {
    event.waitUntil(
        caches.open(STATIC_CACHE)
            .then(cache => cache.addAll(CORE_ASSETS))
            .then(() => self.skipWaiting())
    );
});

self.addEventListener('activate', event => {
    event.waitUntil(
        caches.keys().then(keys => Promise.all(
            keys
                .filter(key => key.startsWith('aurago-') && key !== STATIC_CACHE)
                .map(key => caches.delete(key))
        )).then(() => self.clients.claim())
    );
});

self.addEventListener('fetch', event => {
    const { request } = event;
    if (request.method !== 'GET') return;

    const url = new URL(request.url);
    if (url.origin !== self.location.origin || isApiRequest(url) || !isStaticAsset(url)) {
        return;
    }

    event.respondWith(
        caches.open(STATIC_CACHE).then(async cache => {
            const key = cacheKeyFor(request);
            const cached = await cache.match(key);
            if (cached) return cached;
            try {
                const response = await fetch(request);
                if (response && response.ok) {
                    await cache.put(key, response.clone());
                }
                return response;
            } catch (_) {
                return new Response('Offline', { status: 503, statusText: 'Service Unavailable' });
            }
        })
    );
});

self.addEventListener('push', event => {
    let data = {};
    if (event.data) {
        try {
            data = event.data.json();
        } catch (_) {
            data = { title: 'AuraGo', body: event.data.text() };
        }
    }

    const target = normalizeNotificationTarget(data.url || '/');
    const options = {
        body: data.message || data.body || 'You have a new message.',
        icon: data.icon || '/web-app-manifest-192x192.png',
        badge: '/web-app-manifest-192x192.png',
        tag: data.tag || 'aurago-notification',
        renotify: true,
        data: { url: target.href },
        actions: [{ action: 'open', title: 'Open' }]
    };
    event.waitUntil(self.registration.showNotification(data.title || 'AuraGo', options));
});

self.addEventListener('notificationclick', event => {
    event.notification.close();
    const rawTarget = event.notification.data && event.notification.data.url;
    const target = normalizeNotificationTarget(rawTarget || '/');

    event.waitUntil(
        self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(windowClients => {
            for (const client of windowClients) {
                if (client.url === target.href && 'focus' in client) return client.focus();
            }
            if (self.clients.openWindow) return self.clients.openWindow(target.href);
        })
    );
});

self.addEventListener('notificationclose', event => {
    console.log('[AuraGo SW] Notification dismissed:', event.notification.tag);
});
