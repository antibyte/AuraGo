// EasyDrag credential rule, client side. It mirrors internal/flows/credentials.go step by step
// (names, token formats, URL cut, word split, JSON pointers, key order) and is tested against
// the same vectors (internal/flows/testdata/credential_cases.json). Masking replaces each typed-in
// credential by the marker before anything is written to browser storage.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const MARKER = '[redacted]';
    const EXACT = new Set(['token', 'accesstoken', 'refreshtoken', 'idtoken', 'apikey', 'secret', 'clientsecret', 'password', 'passwd', 'pwd', 'sig', 'signature',
        'auth', 'authorization', 'xapikey', 'privatekey', 'secretkey', 'accesskey', 'sessiontoken']);
    const SUFFIXES = ['token', 'secret', 'password', 'apikey'];
    const TOKEN = /^(?:(?:sk-|sk_live_|rk_live_)[A-Za-z0-9_-]{20,}|(?:ghp_|gho_|ghu_|ghs_|ghr_)[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{16,}|glpat-[A-Za-z0-9_-]{16,}|xox[bpar]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)$/;
    const WORDS = /[ \t\r\n\f]+|[^ \t\r\n\f]+/g;
    const HEADER_EXACT = new Set(['authorization', 'proxy-authorization', 'cookie', 'x-api-key', 'api-key', 'apikey', 'x-auth-token', 'x-access-token']);
    const HEADER_PARTS = ['api-key', 'api_key', 'apikey', 'token', 'secret', 'password'];

    const empty = v => v == null || (typeof v === 'string' && v.trim() === '') || (Array.isArray(v) && !v.length) || (typeof v === 'object' && !Array.isArray(v) && !Object.keys(v).length);

    // hasTemplate: an unescaped "{{" (HasTemplate in template.go).
    function hasTemplate(s) {
        for (let i = 0; i + 1 < s.length; i++) {
            if (s[i] === '\\' && s.startsWith('{{', i + 1)) { i += 2; continue; }
            if (s[i] === '{' && s[i + 1] === '{') return true;
        }
        return false;
    }

    // templateOnly: nothing but templates and a bare scheme word is left (templateOnlyCredential).
    function templateOnly(s) {
        if (!hasTemplate(s) || ED.template.refs(s).some(r => r.error)) return false;
        const rest = ED.template.segments(s).filter(g => g.expr === undefined).map(g => g.literal).join('');
        return ['', 'bearer', 'basic', 'token'].includes(rest.trim().toLowerCase());
    }

    function normal(name) { return String(name).trim().toLowerCase().replace(/[-_]/g, ''); }
    function fieldName(name) { const n = normal(name); return EXACT.has(n) || SUFFIXES.some(s => n.endsWith(s)); }
    function queryKey(name) { return fieldName(name) || normal(name) === 'key'; }
    function isToken(s) { return s.length >= 16 && !s.includes('{{') && TOKEN.test(s); }
    function literalText(s) { return s.trim() !== '' && s !== MARKER && !hasTemplate(s); }
    function headerName(name) { const n = String(name).trim().toLowerCase(); return HEADER_EXACT.has(n) || HEADER_PARTS.some(p => n.includes(p)); }
    function literalHeader(s) { return s.trim() !== '' && s !== MARKER && !templateOnly(s); }
    function pointer(k) { return String(k).replace(/~/g, '~0').replace(/\//g, '~1'); }
    function unescapeQuery(s) { try { return decodeURIComponent(s.replace(/\+/g, ' ')); } catch (err) { return s; } }
    function urlPath(ptr, name) { return ptr === '' ? name : ptr + '?' + name; }

    // splitURL cuts an http:// or https:// word like splitCredentialURL; null for any other text.
    function splitURL(s) {
        const m = /^https?:\/\//i.exec(s);
        if (!m) return null;
        const u = { prefix: m[0], userinfo: '', hasUser: false, hostPath: '', query: '', hasQuery: false, fragment: '' };
        let rest = s.slice(m[0].length);
        let i = rest.indexOf('#');
        if (i >= 0) { u.fragment = rest.slice(i); rest = rest.slice(0, i); }
        i = rest.indexOf('?');
        if (i >= 0) { u.query = rest.slice(i + 1); u.hasQuery = true; rest = rest.slice(0, i); }
        let authority = rest, tail = '';
        i = rest.indexOf('/');
        if (i >= 0) { authority = rest.slice(0, i); tail = rest.slice(i); }
        i = authority.lastIndexOf('@');
        if (i >= 0) { u.userinfo = authority.slice(0, i); u.hasUser = true; authority = authority.slice(i + 1); }
        u.hostPath = authority + tail;
        return u;
    }
    function joinURL(u) { return u.prefix + (u.hasUser ? u.userinfo + '@' : '') + u.hostPath + (u.hasQuery ? '?' + u.query : '') + u.fragment; }

    function userinfoSecret(ui) {
        if (!literalText(ui)) return false;
        const c = ui.indexOf(':');
        return c >= 0 ? literalText(ui.slice(c + 1)) : isToken(ui);
    }

    // put records a hit and says what takes its place: the marker, or (remove mode, w.drop set)
    // an empty text for the one finding to clear and the original for every other.
    function put(w, hit, original) {
        w.hits.push(hit);
        if (!w.drop) return MARKER;
        return hit.location === w.drop.location && hit.path === w.drop.path ? '' : original;
    }

    function walk(w, v, ptr) {
        if (typeof v === 'string') return text(w, v, ptr);
        if (Array.isArray(v)) return v.map((item, i) => walk(w, item, ptr + '/' + i));
        if (v && typeof v === 'object') {
            const out = {};
            Object.keys(v).sort().forEach(k => { out[k] = entry(w, k, v[k], ptr); });
            return out;
        }
        return v;
    }

    function entry(w, k, val, ptr) {
        const p = ptr + '/' + pointer(k);
        if (typeof val === 'string' && fieldName(k) && literalText(val)) return put(w, { location: w.fields, path: p, value: val, top: false }, val);
        return walk(w, val, p);
    }

    function text(w, s, ptr) { return (s.match(WORDS) || []).map(tok => word(w, tok, ptr)).join(''); }

    function word(w, tok, ptr) {
        if (/^[ \t\r\n\f]/.test(tok)) return tok;
        const u = splitURL(tok);
        if (u) return urlText(w, u, ptr);
        if (isToken(tok)) return put(w, { location: 'format', path: ptr, value: tok, top: ptr === '' }, tok);
        return tok;
    }

    function urlText(w, u, ptr) {
        if (u.hasUser && userinfoSecret(u.userinfo)) {
            const r = put(w, { location: 'url_userinfo', path: urlPath(ptr, 'userinfo'), value: u.userinfo, top: ptr === '' }, u.userinfo);
            if (r === '') u.hasUser = false; else u.userinfo = r;
        }
        if (u.hasQuery) {
            u.query = u.query.split('&').map(pair => {
                const eq = pair.indexOf('=');
                if (eq < 0) return pair;
                const key = pair.slice(0, eq), val = pair.slice(eq + 1);
                if (!literalText(val)) return pair;
                const name = unescapeQuery(key), dec = unescapeQuery(val);
                if (!queryKey(name) && !isToken(dec)) return pair;
                return key + '=' + put(w, { location: 'url_query', path: urlPath(ptr, name), value: dec, top: ptr === '' }, val);
            }).join('&');
        }
        return joinURL(u);
    }

    function decodeContainer(s) {
        const t = s.trim();
        if (!t || (t[0] !== '{' && t[0] !== '[')) return { ok: false };
        try { return { ok: true, value: JSON.parse(t) }; } catch (err) { return { ok: false }; }
    }

    function scanHeaders(v, drop) {
        if (!v || typeof v !== 'object' || Array.isArray(v)) return { hits: [], value: v };
        const w = { fields: 'param_field', hits: [], drop }, out = {};
        Object.keys(v).sort().forEach(k => {
            const name = k.trim();
            if (typeof v[k] === 'string' && headerName(name)) {
                out[k] = literalHeader(v[k]) ? put(w, { location: 'header', path: name, value: v[k], top: true }, v[k]) : v[k];
                return;
            }
            out[k] = walk(w, v[k], '/' + pointer(k));
        });
        return { hits: w.hits, value: out };
    }

    // scanParam is scanCredentialParam: the hits of one node parameter and the parameter with them replaced.
    function scanParam(type, name, v, drop) {
        const w = { fields: 'param_field', hits: [], drop };
        const http = type === 'http.request';
        if (http && name === 'headers') return scanHeaders(v, drop);
        if (http && name === 'body') {
            w.fields = 'body_field';
            const parsed = typeof v === 'string' ? decodeContainer(v) : { ok: false };
            if (parsed.ok) {
                const red = walk(w, parsed.value, '');
                return w.hits.length ? { hits: w.hits, value: JSON.stringify(red) } : { hits: [], value: v };
            }
        } else if (String(type).startsWith('tool.')) {
            const red = entry(w, name, v, '');
            return { hits: w.hits, value: red };
        }
        const red = walk(w, v, '');
        return { hits: w.hits, value: red };
    }

    // redactHeaders is redactCredentialHeaders with keepTemplates: it also replaces credential
    // headers whose value is not a text.
    function redactHeaders(params) {
        const headers = params.headers;
        if (!headers || typeof headers !== 'object' || Array.isArray(headers)) return params;
        let copied = null;
        Object.keys(headers).forEach(name => {
            const v = headers[name];
            if (!headerName(name) || empty(v)) return;
            if (typeof v === 'string' && (v === MARKER || templateOnly(v))) return;
            if (!copied) copied = Object.assign({}, headers);
            copied[name] = MARKER;
        });
        return copied ? Object.assign({}, params, { headers: copied }) : params;
    }

    // maskParams is redactCredentialParams(type, params, true): a copy, or params itself when nothing changes.
    function maskParams(type, params) {
        if (!params || typeof params !== 'object') return params;
        const out = redactHeaders(params);
        let copied = null;
        Object.keys(out).sort().forEach(name => {
            const r = scanParam(type, name, out[name]);
            if (!r.hits.length) return;
            if (!copied) copied = Object.assign({}, out);
            copied[name] = r.value;
        });
        return copied || out;
    }

    function movable(node, param, hit) {
        if (node.type !== 'http.request' || !empty((node.params || {}).auth_secret)) return false;
        if (hit.location === 'header') return param === 'headers' && !hasTemplate(hit.value);
        if (hit.location === 'url_query' || hit.location === 'url_userinfo') return param === 'url' && hit.top;
        return false;
    }

    // findings lists the typed-in credentials of a node; each has the typed value (never send it).
    function findings(node) {
        const out = [];
        const params = (node && node.params) || {};
        Object.keys(params).sort().forEach(name => {
            scanParam(node.type, name, params[name]).hits.forEach(h => out.push({ nodeId: node.id, param: name, location: h.location, path: h.path, movable: movable(node, name, h), value: h.value }));
        });
        return out;
    }

    // remove returns the node's params with the value of one finding cleared.
    function remove(node, finding) {
        const params = JSON.parse(JSON.stringify(node.params || {}));
        params[finding.param] = scanParam(node.type, finding.param, params[finding.param], { location: finding.location, path: finding.path }).value;
        return params;
    }

    // mask returns a copy of a flow document (or clipboard fragment) with every finding masked.
    function mask(doc) {
        if (!doc || !Array.isArray(doc.nodes)) return doc;
        return Object.assign({}, doc, { nodes: doc.nodes.map(n => (n && n.params ? Object.assign({}, n, { params: maskParams(n.type, n.params) }) : n)) });
    }

    function hasFindings(doc) { return !!(doc && Array.isArray(doc.nodes) && doc.nodes.some(n => n && findings(n).length)); }

    ED.credentials = { findings, mask, maskParams, hasFindings, remove, MARKER };
})();
