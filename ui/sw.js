self.addEventListener('install', function (event) {
    console.log('AuraGo Service Worker installing.');
});

self.addEventListener('activate', function (event) {
    console.log('AuraGo Service Worker activated.');
});

self.addEventListener('push', function (event) {
    let data = {};
    if (event.data) {
        try {
            data = event.data.json();
        } catch (e) {
            data = { title: "AuraGo", body: event.data.text() };
        }
    }

    const title = data.title || "AuraGo Notification";
    const options = {
        body: data.message || data.body || "You have a new message.",
        icon: data.icon || '/aurago_logo.png',
        badge: '/aurago_logo.png',
        data: data.url || '/'
    };

    event.waitUntil(
        self.registration.showNotification(title, options)
    );
});

self.addEventListener('notificationclick', function (event) {
    event.notification.close();
    if (event.notification.data) {
        event.waitUntil(
            clients.openWindow(event.notification.data)
        );
    }
});
