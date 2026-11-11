// EasyDrag credential rule, client side. It mirrors internal/flows/credentials.go step by step
// (names, token formats, URL cut, word split, JSON pointers, key order) and is tested against
// the same vectors (internal/flows/testdata/credential_cases.json). Masking replaces each typed-in
// credential by the marker before anything is written to browser storage.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const MARKER = '[redacted]';
    const EXACT = new Set(['token', 'accesstoken', 'refreshtoken', 'idtoken', 'apikey', 'secret', 'clientsecret', 'password', 'passwd', 'pwd', 'sig', 'signature',
        'auth', 'authorization', 'xapikey', 'privatekey', 'secretkey', 'accesskey', 'sessiontoken', 'secrets', 'tokens', 'apikeys']);
    const SUFFIXES = ['token', 'secret', 'password', 'apikey', 'tokens', 'secrets', 'passwords', 'apikeys'];
    const TOKEN = /^(?:(?:sk-|sk_live_|rk_live_)[A-Za-z0-9_-]{20,}|(?:ghp_|gho_|ghu_|ghs_|ghr_)[A-Za-z0-9]{16,}|github_pat_[A-Za-z0-9_]{16,}|glpat-[A-Za-z0-9_-]{16,}|xox[bpar]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+)$/;
    const WORDS = /[ \t\r\n\f]+|[^ \t\r\n\f]+/g;
    const HEADER_EXACT = new Set(['authorization', 'proxy-authorization', 'cookie', 'x-api-key', 'api-key', 'apikey', 'x-auth-token', 'x-access-token', 'x-auth', 'x-session']);
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
        return ['', 'bearer', 'basic', 'token'].includes(asciiLower(asciiTrim(rest)));
    }

    // ASCII-only fold and trim, like asciiLower and asciiTrim in credentials.go: both sides must agree
    // on every name, so no Unicode case folding (a dotted capital I is no "i") and no Unicode white space.
    function asciiLower(s) { return String(s).replace(/[A-Z]/g, c => String.fromCharCode(c.charCodeAt(0) + 32)); }
    function asciiTrim(s) { return String(s).replace(/^[ \t\n\v\f\r]+|[ \t\n\v\f\r]+$/g, ''); }

    function normal(name) { return asciiLower(asciiTrim(name)).replace(/[-_]/g, ''); }
    function fieldName(name) { const n = normal(name); return EXACT.has(n) || SUFFIXES.some(s => n.endsWith(s)); }
    function queryKey(name) { return fieldName(name) || normal(name) === 'key'; }
    function isToken(s) { return s.length >= 16 && !s.includes('{{') && TOKEN.test(s); }
    // literalText: not blank, not the marker, and not only a template (a bare scheme around templates
    // is no typed-in text); text next to a template ("sk-abc{{x}}") is typed in.
    function literalText(s) { return asciiTrim(s) !== '' && s !== MARKER && !templateOnly(s); }
    function headerName(name) { const n = asciiLower(asciiTrim(name)); return HEADER_EXACT.has(n) || HEADER_PARTS.some(p => n.includes(p)); }
    const literalHeader = literalText;
    function pointer(k) { return String(k).replace(/~/g, '~0').replace(/\//g, '~1'); }
    // utf8At decodes one valid UTF-8 sequence of bytes at i: [text, length]; null for an invalid byte.
    function utf8At(b, i) {
        const c = b[i], cont = (j, lo, hi) => j < b.length && b[j] >= (lo || 0x80) && b[j] <= (hi || 0xBF);
        if (c < 0x80) return [String.fromCharCode(c), 1];
        if (c >= 0xC2 && c <= 0xDF && cont(i + 1)) return [String.fromCharCode(((c & 0x1F) << 6) | (b[i + 1] & 0x3F)), 2];
        if (c >= 0xE0 && c <= 0xEF) {
            const lo = c === 0xE0 ? 0xA0 : 0x80, hi = c === 0xED ? 0x9F : 0xBF;
            if (cont(i + 1, lo, hi) && cont(i + 2)) return [String.fromCharCode(((c & 0x0F) << 12) | ((b[i + 1] & 0x3F) << 6) | (b[i + 2] & 0x3F)), 3];
        }
        if (c >= 0xF0 && c <= 0xF4) {
            const lo = c === 0xF0 ? 0x90 : 0x80, hi = c === 0xF4 ? 0x8F : 0xBF;
            if (cont(i + 1, lo, hi) && cont(i + 2) && cont(i + 3)) return [String.fromCodePoint(((c & 0x07) << 18) | ((b[i + 1] & 0x3F) << 12) | ((b[i + 2] & 0x3F) << 6) | (b[i + 3] & 0x3F)), 4];
        }
        return null;
    }
    // unescapeQuery decodes "+" and every valid %XX byte by byte (an invalid sequence stays as it is),
    // like unescapeQuery in credentials.go; a byte that is no valid UTF-8 becomes U+FFFD, one per byte.
    function unescapeQuery(s) {
        let out = '', bytes = [];
        const flush = () => {
            for (let i = 0; i < bytes.length;) {
                const r = utf8At(bytes, i);
                out += r ? r[0] : '\uFFFD';
                i += r ? r[1] : 1;
            }
            bytes = [];
        };
        for (let i = 0; i < s.length; i++) {
            const c = s[i];
            if (c === '%' && /^[0-9A-Fa-f]{2}$/.test(s.slice(i + 1, i + 3))) { bytes.push(parseInt(s.slice(i + 1, i + 3), 16)); i += 2; continue; }
            flush();
            out += c === '+' ? ' ' : c;
        }
        flush();
        return out;
    }
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
        if (fieldName(k)) {
            if (typeof val === 'string' && literalText(val)) return put(w, { location: w.fields, path: p, value: val, top: false }, val);
            if (Array.isArray(val)) return secretList(w, val, p);
        }
        return walk(w, val, p);
    }

    // secretList: the value of a credential-named key that is a list; every text item is a secret.
    function secretList(w, items, ptr) {
        return items.map((item, i) => {
            const p = ptr + '/' + i;
            if (typeof item === 'string' && literalText(item)) return put(w, { location: w.fields, path: p, value: item, top: false }, item);
            if (Array.isArray(item)) return secretList(w, item, p);
            return walk(w, item, p);
        });
    }

    // text: a string that holds a JSON object or list is scanned as that value (and written back as
    // JSON when something was found); any other string is checked word by word.
    function text(w, s, ptr) {
        const parsed = decodeContainer(s);
        if (parsed.ok) {
            const before = w.hits.length, red = walk(w, parsed.value, ptr);
            return w.hits.length === before ? s : JSON.stringify(red);
        }
        return (s.match(WORDS) || []).map(tok => word(w, tok, ptr)).join('');
    }

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
        if (u.hasQuery) u.query = pairs(w, u.query, 'url_query', name => urlPath(ptr, name), ptr === '');
        // A fragment can carry a token too ("#access_token=..."). It is never sent to the server, so a
        // hit there is not movable: top stays false.
        if (u.fragment.length > 1) u.fragment = '#' + pairs(w, u.fragment.slice(1), 'url_query', name => urlPath(ptr, name), false);
        return joinURL(u);
    }

    // pairs applies the query rule to a "k=v&k=v" string (URL query, URL fragment, form body): a pair
    // whose key names a credential or whose value is a token format, with a typed-in value, is a hit.
    function pairs(w, s, location, path, top) {
        return s.split('&').map(pair => {
            const eq = pair.indexOf('=');
            if (eq < 0) return pair;
            const key = pair.slice(0, eq), val = pair.slice(eq + 1);
            if (!literalText(val)) return pair;
            const name = unescapeQuery(key), dec = unescapeQuery(val);
            if (!queryKey(name) && !isToken(dec)) return pair;
            return key + '=' + put(w, { location, path: path(name), value: dec, top }, val);
        }).join('&');
    }

    function decodeContainer(s) {
        const t = asciiTrim(s);
        if (!t || (t[0] !== '{' && t[0] !== '[')) return { ok: false };
        try { return { ok: true, value: JSON.parse(t) }; } catch (err) { return { ok: false }; }
    }

    function scanHeaders(v, drop) {
        if (!v || typeof v !== 'object' || Array.isArray(v)) return { hits: [], value: v };
        const w = { fields: 'param_field', hits: [], drop }, out = {};
        Object.keys(v).sort().forEach(k => {
            const name = asciiTrim(k);
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
            if (typeof v === 'string') {
                // No JSON: read it as form data, then check the words of what is left.
                const red = text(w, pairs(w, v, 'body_field', n => '/' + pointer(n), false), '');
                return w.hits.length ? { hits: w.hits, value: red } : { hits: [], value: v };
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
        if (hit.location === 'url_query' || hit.location === 'url_userinfo') return param === 'url' && hit.top && !hasTemplate(hit.value);
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
