self.addEventListener('install', function (event) {
    console.log('[AuraGo SW] Installing...');
    self.skipWaiting();
});

self.addEventListener('activate', function (event) {
    console.log('[AuraGo SW] Activated.');
    event.waitUntil(clients.claim());
});

self.addEventListener('push', function (event) {
    let data = {};
    if (event.data) {
        try {
            data = event.data.json();
        } catch (e) {
            data = { title: 'AuraGo', body: event.data.text() };
        }
    }

    const title = data.title || 'AuraGo';
    const options = {
        body: data.message || data.body || 'You have a new message.',
        icon: data.icon || '/web-app-manifest-192x192.png',
        badge: '/web-app-manifest-192x192.png',
        tag: 'aurago-notification',
        renotify: true,
        data: { url: data.url || '/' },
        actions: [
            { action: 'open', title: 'Open' }
        ]
    };

    event.waitUntil(
        self.registration.showNotification(title, options)
    );
});

self.addEventListener('notificationclick', function (event) {
    event.notification.close();
    const rawUrl = (event.notification.data && event.notification.data.url) ? event.notification.data.url : '/';
    // Validate URL: only allow relative paths starting with /
    const allowed = rawUrl === '/' || (/^\/[a-zA-Z0-9._~!$&'()*+,;=:@-]*$/).test(rawUrl);
    const targetUrl = allowed ? rawUrl : '/';

    event.waitUntil(
        clients.matchAll({ type: 'window', includeUncontrolled: true }).then(function (windowClients) {
            // Focus an existing tab at the target URL if possible
            for (let i = 0; i < windowClients.length; i++) {
                const client = windowClients[i];
                if (client.url === targetUrl && 'focus' in client) {
                    return client.focus();
                }
            }
            // Otherwise open a new window
            if (clients.openWindow) {
                return clients.openWindow(targetUrl);
            }
        })
    );
});

self.addEventListener('notificationclose', function (event) {
    console.log('[AuraGo SW] Notification dismissed:', event.notification.tag);
});

