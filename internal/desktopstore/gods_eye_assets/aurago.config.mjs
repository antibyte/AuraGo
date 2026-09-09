import upstream from './vite.config.js';

export function frameOrigins(raw = '[]') {
    let values;
    try { values = JSON.parse(raw); } catch { return []; }
    if (!Array.isArray(values) || values.length > 8) return [];
    const origins = [];
    for (const value of values) {
        try {
            const url = new URL(value);
            if (typeof value !== 'string' || value.length > 512 || !['http:', 'https:'].includes(url.protocol)
                || url.username || url.password || url.search || url.hash || url.pathname !== '/'
                || /[\s*;'"\\]/.test(value)) return [];
            origins.push(url.origin);
        } catch { return []; }
    }
    return [...new Set(origins)];
}

export default async function auragoConfig(env) {
    const config = await upstream(env);
    const origins = frameOrigins(process.env.AURAGO_FRAME_ORIGINS);
    config.plugins = config.plugins.filter(plugin => plugin?.name !== 'gev-key-setup');
    config.plugins.unshift({
        name: 'aurago-managed-settings',
        configureServer(server) {
            server.middlewares.use((req, res, next) => {
                // App ports share the host with AuraGo: never pass its cookies
                // or Authorization header to upstream application middleware.
                delete req.headers.cookie;
                delete req.headers.authorization;
                const path = (req.url || '').split('?')[0];
                if (path === '/aurago-health') {
                    res.setHeader('Content-Type', 'application/json');
                    res.end('{"status":"ok"}');
                    return;
                }
                if (path === '/api/setup' || path.startsWith('/api/setup/')) {
                    res.statusCode = 403;
                    res.setHeader('Cache-Control', 'no-store');
                    res.setHeader('Content-Type', 'application/json');
                    res.end('{"error":"managed_by_aurago_store"}');
                    return;
                }
                next();
            });
        }
    });
    config.server = {
        ...config.server,
        host: '0.0.0.0', port: 4173, strictPort: true, hmr: false,
        allowedHosts: [...new Set(['localhost', '127.0.0.1', ...origins.map(origin => new URL(origin).hostname)])],
        fs: { ...config.server.fs, deny: [...config.server.fs.deny, '**/.gev-logs/**', '**/.gev-cache/**'] },
        headers: {
            'Content-Security-Policy': `frame-ancestors ${origins.length ? origins.join(' ') : "'none'"}`,
            'Cache-Control': 'no-store',
            ...(origins.length ? {} : { 'X-Frame-Options': 'DENY' })
        }
    };
    return config;
}
