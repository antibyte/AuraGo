(function () {
    'use strict';
    // A data document has an intrinsic opaque origin. Its own blob section
    // frames share that opaque origin, never the authenticated Desktop origin.
    const frame = document.createElement('iframe');
    frame.title = 'Bookshelf'; frame.sandbox = 'allow-scripts allow-same-origin';
    frame.referrerPolicy = 'no-referrer';
    frame.style.cssText = 'border:0;width:100%;height:100%;display:block';
    const scriptURL = new URL('/js/vendor/foliate/reader.bundle.js', location.origin);
    const version = document.querySelector('meta[name="bookshelf-version"]')?.content;
    if (version) scriptURL.searchParams.set('v', version);
    const script = scriptURL.href.replaceAll('&', '&amp;').replaceAll('"', '&quot;');
    const ready = new Promise(resolve => frame.addEventListener('load', resolve, { once: true }));
    frame.src = 'data:text/html;charset=utf-8,' + encodeURIComponent('<!doctype html><meta charset="utf-8"><title>Bookshelf</title><style>html,body{margin:0;height:100%;overflow:hidden;background:#eee3cb}</style><script src="' + script + '"></script>');
    document.body.append(frame);
    let connected = false;
    window.addEventListener('message', async event => {
        if (connected || event.source !== parent || event.data?.type !== 'bookshelf-connect' || !event.ports?.[0]) return;
        connected = true; await ready;
        frame.contentWindow.postMessage({ type: 'bookshelf-connect' }, '*', [event.ports[0]]);
    });
}());
