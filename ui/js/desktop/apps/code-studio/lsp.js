    // IDE package 2b: the LSP client core of Code Studio.
    //
    // One @codemirror/lsp-client LSPClient per language and window talks to a language server inside the container
    // through the bridge GET /api/code-studio/lsp?lang=<language> (plan 2a). The bridge carries one JSON-RPC message
    // per text frame and closes with 4404 (not installed), 4429 (too many sessions), 4500 (start failed) or 1011
    // (server exited).
    //
    // instance.lsp.connections[language] holds one connection ("conn") in one of the contract states:
    // off | starting | ready | disconnected | not-installed | too-many | error. A session is one LSPClient. It starts
    // with the first editor of its language, survives reconnects (1, 2, 5, 10, 10 s) and the 10-minute idle close,
    // and ends on 4404, on 4429, after the last failed reconnect, on a restart and when LSP is switched off.
    //
    // Editors carry the LSP extensions in a compartment of their own (view.csLsp), attached once the session's client
    // has connected, so nothing waits on a client that never started. The Workspace subclass keeps one LSP document
    // per open tab: split views of one file share it, and tab switches (which rebuild every view) neither close nor
    // reopen documents.
    //
    // Plan 2c builds on lspEnsureClient, lspClientState, lspRequest, lspOnServerRequest, lspNotifyFilesChanged,
    // lspActionExtensionFactories, lspViewForPath and lspTouch. Names starting with lspAct / LSP_ACT_ belong to plan 2c;
    // the only one declared here is the contract array lspActionExtensionFactories.

    const LSP_PROTOCOL = window.CodeStudioLSPProtocol;
    const LSP_LANGUAGES = ['go', 'typescript', 'python', 'cpp'];
    const LSP_SERVER_NAMES = { go: 'gopls', typescript: 'TypeScript', python: 'Pyright', cpp: 'clangd' };
    // Default budget of lspRequest (hover- and completion-sized requests of plan 2c).
    const LSP_REQUEST_TIMEOUT_MS = 3000;
    // The LSPClient's own timeout: 15 s, so references and rename of plan 2c survive. It also covers initialize on a
    // cold container. The library's hover, completion, signature and definition requests share it.
    const LSP_CLIENT_TIMEOUT_MS = 15000;
    const LSP_IDLE_MS = 10 * 60 * 1000;
    // Five reconnect attempts after an unexpected close, then the state is "error".
    const LSP_BACKOFF_MS = [1000, 2000, 5000, 10000, 10000];
    const LSP_TOO_MANY_RETRY_GAP_MS = 2000;
    // The exact text 6.3.1 sends after the initialize answer (JSON.stringify keeps the key order).
    const LSP_INITIALIZED_MESSAGE = JSON.stringify({ jsonrpc: '2.0', method: 'initialized', params: {} });
    const LSP_INITIALIZE_REQUEST = /^\{"jsonrpc":"2\.0","id":\d+,"method":"initialize",/;
    const LSP_REQUEST_MESSAGE = /^\{"jsonrpc":"2\.0","id":/;
    const LSP_QUIET_ERRORS = /^(Request timed out|Client not connected|Language server disconnected)/;
    // RequestCancelled, ContentModified and ServerCancelled are routine for hover and completion.
    const LSP_QUIET_ERROR_CODES = [-32800, -32801, -32802];
    // Merged into the 6.3.1 client capabilities, which declare none of these. applyEdit, documentChanges, prepareRename
    // and the code action support serve plan 2c. Servers that can watch files keep doing it themselves (no dynamic
    // registration); Code Studio's own writes are reported with didChangeWatchedFiles either way.
    const LSP_CLIENT_CAPABILITIES = {
        workspace: {
            configuration: true,
            applyEdit: true,
            workspaceEdit: { documentChanges: true },
            didChangeWatchedFiles: { dynamicRegistration: false }
        },
        textDocument: {
            synchronization: { didSave: true },
            rename: { prepareSupport: true },
            codeAction: {
                codeActionLiteralSupport: {
                    codeActionKind: {
                        valueSet: ['', 'quickfix', 'refactor', 'refactor.extract', 'refactor.inline', 'refactor.rewrite', 'source',
                            'source.organizeImports', 'source.fixAll']
                    }
                },
                isPreferredSupport: true,
                disabledSupport: true,
                dataSupport: true,
                resolveSupport: { properties: ['edit'] }
            }
        }
    };
    // The files a branch switch changed: HEAD@{1} is where HEAD pointed before the switch.
    const LSP_BRANCH_DIFF_COMMAND = "git -C /workspace -c core.quotePath=false diff --name-status --no-renames 'HEAD@{1}' HEAD";
    const LSP_SANITIZE_DROP = new Set(['SCRIPT', 'STYLE', 'IFRAME', 'FRAME', 'FRAMESET', 'OBJECT', 'EMBED', 'APPLET', 'SVG', 'MATH',
        'TEMPLATE', 'IMG', 'PICTURE', 'SOURCE', 'VIDEO', 'AUDIO', 'TRACK', 'CANVAS', 'LINK', 'META', 'BASE', 'FORM', 'INPUT', 'BUTTON',
        'TEXTAREA', 'SELECT', 'OPTION', 'NOSCRIPT', 'DIALOG']);
    const LSP_SANITIZE_KEEP = new Set(['P', 'BR', 'HR', 'PRE', 'CODE', 'SPAN', 'DIV', 'EM', 'STRONG', 'B', 'I', 'U', 'S', 'DEL', 'INS',
        'SUB', 'SUP', 'KBD', 'SMALL', 'UL', 'OL', 'LI', 'BLOCKQUOTE', 'H1', 'H2', 'H3', 'H4', 'H5', 'H6', 'TABLE', 'THEAD', 'TBODY', 'TR',
        'TH', 'TD', 'A']);
    const LSP_SERVER_REQUEST_HANDLERS = new Map();
    // Plan 2c pushes factories here at load time; see lspEditorExtensions.
    const lspActionExtensionFactories = [];
    let lspRequestSequence = 0;
    let lspWorkspaceClassCache = null;

    // --- Availability and connections ------------------------------------------------------

    function lspAvailable(instance, cm) {
        return !!instance && isLiveInstance(instance) && !instance.readOnly && !!LSP_PROTOCOL
            && instance.editorType === 'codemirror' && !!cm && typeof cm.LSPClient === 'function'
            && typeof cm.serverCompletion === 'function' && typeof WebSocket === 'function'
            && editorSettings().codeIntelligence !== false;
    }

    function lspConnectionOf(instance, language) {
        return instance && instance.lsp && instance.lsp.connections[language] || null;
    }

    // The connection of a language; with create it is made on first use and its session started when none runs.
    function lspConnection(instance, language, create) {
        let conn = lspConnectionOf(instance, language);
        if (!conn && create) {
            if (!instance.lsp) {
                instance.lsp = { connections: {} };
                addDisposer(instance, () => lspShutdownInstance(instance));
            }
            conn = {
                instance, language, state: 'off', client: null, socket: null, synced: false, attached: false,
                retries: 0, retryTimer: 0, idleTimer: 0, lastRetryAt: 0, waiters: [], pending: new Map(), transport: null,
                // The client got connect() in this session. LSPClient.connected stays true after disconnect(), so it is tracked here.
                linked: false
            };
            conn.transport = lspCreateTransport(conn);
            instance.lsp.connections[language] = conn;
        }
        if (conn && create && conn.state === 'off' && !conn.client) lspStart(conn, false);
        return conn;
    }

    function lspStart(conn, reconfigure) {
        const instance = conn.instance;
        if (!lspAvailable(instance, instance.cmModule)) return;
        lspClearTimers(conn);
        conn.retries = 0;
        conn.attached = false;
        conn.linked = false;
        try {
            conn.client = lspCreateClient(conn);
        } catch (err) {
            console.warn('[lsp ' + conn.language + '] client setup failed', err);
            conn.client = null;
            conn.state = 'error';
            lspScheduleStatus(instance);
            return;
        }
        conn.state = 'starting';
        lspOpenSocket(conn);
        lspScheduleStatus(instance);
        if (reconfigure) lspReconfigureViews(instance, conn.language);
    }

    function lspCreateClient(conn) {
        const cm = conn.instance.cmModule;
        const WorkspaceClass = lspWorkspaceClass(cm);
        const client = new cm.LSPClient({
            rootUri: LSP_PROTOCOL.ROOT_URI,
            timeout: LSP_CLIENT_TIMEOUT_MS,
            sanitizeHTML: lspSanitizeHTML,
            workspace: client => new WorkspaceClass(client, conn),
            notificationHandlers: {
                // Returning true replaces the handler of serverDiagnostics(): findings go to the Problems list,
                // which paints the squiggles of every open tab.
                'textDocument/publishDiagnostics': (client, params) => {
                    lspHandleDiagnostics(conn, client, params);
                    return true;
                },
                'window/showMessage': (_client, params) => {
                    lspHandleShowMessage(conn, params);
                    return true;
                },
                'window/logMessage': (_client, params) => {
                    if (params && params.type === 1) console.warn('[lsp ' + conn.language + '] ' + String(params.message || ''));
                    return true;
                }
            },
            // serverDiagnostics() brings autoSync (didChange 500 ms after typing) as its editor extension; the plain
            // object merges the capabilities above into the client's initialize request.
            extensions: [cm.serverDiagnostics(), { clientCapabilities: LSP_CLIENT_CAPABILITIES }]
        });
        // Library requests (hover, completion, signature help, definition) go through client.request and would wait on
        // initializing forever after an idle close; touching the connection first reconnects it.
        const libraryRequest = client.request.bind(client);
        client.request = (method, params) => {
            lspTouchConnection(conn);
            return libraryRequest(method, params);
        };
        return client;
    }

    function lspOpenSocket(conn) {
        const instance = conn.instance;
        let socket;
        try {
            // Plan C: the bridge serves the active workspace; a switch shuts the sessions down and later opens new sockets.
            socket = new WebSocket(workspaceSocketURL('/api/code-studio/lsp?lang=' + encodeURIComponent(conn.language), instance));
        } catch (_) {
            conn.socket = null;
            lspSocketClosed(conn, 1006);
            return;
        }
        conn.socket = socket;
        conn.synced = false;
        socket.onopen = bindInstance(instance, () => {
            if (conn.socket !== socket || !conn.client) return;
            const client = conn.client;
            // The findings of the previous server are stale; the new one publishes its own.
            lspClearLanguageProblems(conn);
            client.connect(conn.transport);
            conn.linked = true;
            client.initializing.then(bindInstance(instance, () => {
                if (conn.socket === socket && conn.client === client) lspBecameReady(conn);
            }), bindInstance(instance, () => {
                if (conn.socket !== socket) return;
                try { socket.close(4000, 'initialize failed'); } catch (_) {}
            }));
        });
        socket.onmessage = event => {
            if (conn.socket === socket) lspReceive(conn, event.data);
        };
        socket.onerror = () => {};
        socket.onclose = bindInstance(instance, event => {
            if (conn.socket === socket) lspSocketClosed(conn, event.code);
        });
    }

    function lspBecameReady(conn) {
        conn.state = 'ready';
        conn.retries = 0;
        lspTouchConnection(conn);
        lspDropSyntaxProblems(conn);
        lspResolveWaiters(conn, conn.client);
        lspScheduleStatus(conn.instance);
        if (!conn.attached) {
            conn.attached = true;
            lspReconfigureViews(conn.instance, conn.language);
        }
    }

    function lspSocketClosed(conn, code) {
        conn.socket = null;
        conn.synced = false;
        lspRejectPending(conn, 'disconnected');
        if (conn.client) conn.client.disconnect();
        if (code === 4404) {
            lspEndSession(conn, 'not-installed');
            return;
        }
        if (code === 4429) {
            lspEndSession(conn, 'too-many');
            return;
        }
        if (conn.retries >= LSP_BACKOFF_MS.length) {
            lspEndSession(conn, 'error');
            return;
        }
        const instance = conn.instance;
        const delay = LSP_BACKOFF_MS[conn.retries];
        conn.retries += 1;
        conn.state = 'disconnected';
        lspScheduleStatus(instance);
        conn.retryTimer = setTimeout(bindInstance(instance, () => {
            conn.retryTimer = 0;
            if (conn.state !== 'disconnected' || !conn.client) return;
            if (!lspAvailable(instance, instance.cmModule)) {
                lspEndSession(conn, 'off');
                return;
            }
            lspOpenSocket(conn);
        }), delay);
    }

    function lspEndSession(conn, nextState, reconfigure = true) {
        lspClearTimers(conn);
        lspCloseSocket(conn, 1000, 'closed');
        const client = conn.client;
        conn.client = null;
        conn.attached = false;
        if (client) {
            try { client.disconnect(); } catch (_) {}
        }
        conn.state = nextState;
        lspClearLanguageProblems(conn);
        lspResolveWaiters(conn, null);
        lspScheduleStatus(conn.instance);
        if (reconfigure) lspReconfigureViews(conn.instance, conn.language);
    }

    function lspCloseSocket(conn, code, reason) {
        const socket = conn.socket;
        conn.socket = null;
        conn.synced = false;
        lspRejectPending(conn, 'disconnected');
        if (!socket) return;
        socket.onopen = socket.onmessage = socket.onclose = socket.onerror = null;
        try { socket.close(code, reason); } catch (_) {}
    }

    function lspClearTimers(conn) {
        if (conn.retryTimer) clearTimeout(conn.retryTimer);
        if (conn.idleTimer) clearTimeout(conn.idleTimer);
        conn.retryTimer = 0;
        conn.idleTimer = 0;
    }

    // Activity: resets the idle timer and reconnects an idle-closed client.
    function lspTouchConnection(conn) {
        if (!conn) return;
        if (conn.state === 'off' && conn.client) {
            if (!lspAvailable(conn.instance, conn.instance.cmModule)) return;
            conn.state = 'starting';
            lspOpenSocket(conn);
            lspScheduleStatus(conn.instance);
        }
        if (!['starting', 'ready', 'disconnected'].includes(conn.state)) return;
        if (conn.idleTimer) clearTimeout(conn.idleTimer);
        conn.idleTimer = setTimeout(() => lspIdleClose(conn), LSP_IDLE_MS);
    }

    // Ten minutes without edits or requests: the socket closes, so the container hold ends and idle shutdown works.
    function lspIdleClose(conn) {
        conn.idleTimer = 0;
        const client = conn.client;
        if (!isLiveInstance(conn.instance) || !client || !conn.linked) return;
        if (!['starting', 'ready', 'disconnected'].includes(conn.state)) return;
        const linked = conn.state !== 'disconnected';
        if (conn.retryTimer) clearTimeout(conn.retryTimer);
        conn.retryTimer = 0;
        conn.retries = 0;
        lspCloseSocket(conn, 1000, 'idle');
        if (linked) client.disconnect();
        conn.state = 'off';
        lspResolveWaiters(conn, null);
        lspScheduleStatus(conn.instance);
    }

    function lspResolveWaiters(conn, client) {
        const waiters = conn.waiters.splice(0);
        waiters.forEach(resolve => resolve(client));
    }

    function lspRejectPending(conn, reason) {
        conn.pending.forEach(entry => {
            clearTimeout(entry.timer);
            entry.reject(new Error(reason));
        });
        conn.pending.clear();
    }

    function lspSettlePending(conn, message) {
        const entry = conn.pending.get(message.id);
        conn.pending.delete(message.id);
        clearTimeout(entry.timer);
        if (message.error) {
            const error = new Error(String(message.error.message || 'request failed'));
            error.code = message.error.code;
            error.data = message.error.data;
            entry.reject(error);
        } else {
            entry.resolve(message.result === undefined ? null : message.result);
        }
    }

    function lspScheduleStatus(instance) {
        if (!instance || instance.lspStatusTimer) return;
        instance.lspStatusTimer = setTimeout(() => {
            instance.lspStatusTimer = 0;
            if (isLiveInstance(instance)) runWithInstance(instance, () => { if (studioRoot()) renderStatus(); });
        }, 0);
    }

    // Swaps the LSP compartment of the open editors of one language (null: all languages).
    function lspReconfigureViews(instance, language) {
        if (!isLiveInstance(instance) || !LSP_PROTOCOL) return;
        runWithInstance(instance, () => {
            const cm = state.cmModule;
            if (!cm) return;
            state.openTabs.forEach(tab => {
                if (!isSourceTab(tab) || (language && LSP_PROTOCOL.languageForPath(tab.path) !== language)) return;
                (tab.views || []).forEach(view => {
                    if (!view || !view.csLsp) return;
                    const next = lspEditorExtensions(cm, tab);
                    if (!next.length && !view.csLspActive) return;
                    view.csLspActive = next.length > 0;
                    try {
                        view.dispatch({ effects: view.csLsp.reconfigure(next) });
                    } catch (err) {
                        console.warn('[lsp] editor reconfigure failed', err);
                    }
                });
            });
        });
    }

    // The settings switch and the read-only policy: turns LSP off (every session ends) or on (editors start it).
    function lspApplyAvailability() {
        const instance = state;
        if (!instance) return;
        const available = lspAvailable(instance, instance.cmModule);
        if (instance.lspAvailableApplied === available) return;
        instance.lspAvailableApplied = available;
        if (!available && instance.lsp) Object.values(instance.lsp.connections).forEach(conn => lspEndSession(conn, 'off', false));
        lspReconfigureViews(instance, null);
    }

    function lspShutdownInstance(instance) {
        const lsp = instance && instance.lsp;
        if (lsp) {
            Object.values(lsp.connections).forEach(conn => {
                lspClearTimers(conn);
                lspCloseSocket(conn, 1000, 'closed');
                if (conn.client) {
                    try { conn.client.disconnect(); } catch (_) {}
                }
                conn.client = null;
                conn.attached = false;
                conn.state = 'off';
                lspResolveWaiters(conn, null);
            });
        }
        if (instance && instance.lspStatusTimer) clearTimeout(instance.lspStatusTimer);
        if (instance) instance.lspStatusTimer = 0;
    }

    // Status bar click: a new session with a new client and a new server process.
    function lspRestart(language) {
        const instance = state;
        const conn = lspConnectionOf(instance, language);
        if (!conn || !lspAvailable(instance, instance.cmModule)) return;
        lspEndSession(conn, 'off', false);
        lspStart(conn, true);
    }

    // 4429: the next focus of an editor of that language tries again.
    function lspRetryTooMany(instance, language) {
        const conn = lspConnectionOf(instance, language);
        if (!conn || conn.state !== 'too-many' || Date.now() - conn.lastRetryAt < LSP_TOO_MANY_RETRY_GAP_MS) return;
        conn.lastRetryAt = Date.now();
        setTimeout(bindInstance(instance, () => {
            if (conn.state === 'too-many' && lspAvailable(instance, instance.cmModule)) lspStart(conn, true);
        }), 0);
    }

    // --- Transport --------------------------------------------------------------------------

    // One stable transport per connection; the socket behind it changes with every reconnect.
    function lspCreateTransport(conn) {
        const handlers = [];
        return {
            send(message) {
                let text = String(message);
                if (LSP_INITIALIZE_REQUEST.test(text)) text = lspPatchInitialize(text);
                const isRequest = LSP_REQUEST_MESSAGE.test(text);
                if (!lspSocketOpen(conn)) {
                    // A request fails at once (the client rejects it); a notification for a closed socket is dropped.
                    if (isRequest) throw new Error('Language server disconnected');
                    return;
                }
                try {
                    conn.socket.send(text);
                } catch (err) {
                    if (isRequest) throw err;
                    return;
                }
                if (text === LSP_INITIALIZED_MESSAGE) lspOpenDocuments(conn);
            },
            subscribe(handler) {
                if (!handlers.includes(handler)) handlers.push(handler);
            },
            unsubscribe(handler) {
                const index = handlers.indexOf(handler);
                if (index >= 0) handlers.splice(index, 1);
            },
            deliver(text) {
                handlers.slice().forEach(handler => {
                    try {
                        handler(text);
                    } catch (err) {
                        console.warn('[lsp ' + conn.language + '] message handling failed', err);
                    }
                });
            }
        };
    }

    // 6.3.1 sends rootUri only; the single workspace folder is added here.
    function lspPatchInitialize(text) {
        try {
            const message = JSON.parse(text);
            message.params = Object.assign({}, message.params, { workspaceFolders: [{ uri: LSP_PROTOCOL.ROOT_URI, name: 'workspace' }] });
            return JSON.stringify(message);
        } catch (_) {
            return text;
        }
    }

    function lspSocketOpen(conn) {
        return !!conn.socket && conn.socket.readyState === 1;
    }

    function lspSendRaw(conn, payload) {
        if (!lspSocketOpen(conn)) return false;
        try {
            conn.socket.send(typeof payload === 'string' ? payload : JSON.stringify(payload));
            return true;
        } catch (_) {
            return false;
        }
    }

    // Server-to-client requests (id and method) never reach the LSPClient, which would answer MethodNotFound.
    // Answers to lspRequest (string ids "cs-N") settle here too; everything else goes to the client.
    function lspReceive(conn, data) {
        if (typeof data !== 'string' || !data) return;
        let message;
        try {
            message = JSON.parse(data);
        } catch (_) {
            return;
        }
        if (!message || typeof message !== 'object' || Array.isArray(message)) return;
        const hasId = message.id !== undefined && message.id !== null;
        if (hasId && typeof message.method === 'string') {
            lspAnswerServerRequest(conn, message);
            return;
        }
        if (hasId && conn.pending.has(message.id)) {
            lspSettlePending(conn, message);
            return;
        }
        conn.transport.deliver(data);
    }

    function lspAnswerServerRequest(conn, message) {
        const reply = payload => lspSendRaw(conn, Object.assign({ jsonrpc: '2.0', id: message.id }, payload));
        const fail = err => reply({ error: { code: -32603, message: String(err && err.message ? err.message : err) } });
        const handler = LSP_SERVER_REQUEST_HANDLERS.get(message.method);
        if (!handler) {
            reply({ error: { code: -32601, message: 'Method not implemented: ' + message.method } });
            return;
        }
        let result;
        try {
            result = runWithInstance(conn.instance, () => handler(conn.language, message.params));
        } catch (err) {
            fail(err);
            return;
        }
        Promise.resolve(result).then(value => reply({ result: value === undefined ? null : value }), fail);
    }

    // Runs right after "initialized" went out: every tracked document is opened on the (new) server with its current
    // text, before any request that waited for the connection is sent.
    function lspOpenDocuments(conn) {
        const client = conn.client;
        const workspace = client && client.workspace;
        if (!workspace) return;
        const cm = conn.instance.cmModule;
        try {
            workspace.prune();
            for (const file of workspace.files) {
                const view = file.getView();
                const tab = workspace.tabFor(file.path);
                if (view) {
                    file.doc = view.state.doc;
                } else if (tab) {
                    file.doc = lspTextOf(cm, tab.content);
                    file.contentRef = tab.content;
                }
                file.version += 1;
                lspClearFilePlugins(cm, file);
                lspSendRaw(conn, {
                    jsonrpc: '2.0',
                    method: 'textDocument/didOpen',
                    params: { textDocument: { uri: file.uri, languageId: file.languageId, version: file.version, text: file.doc.toString() } }
                });
            }
        } catch (err) {
            console.warn('[lsp ' + conn.language + '] reopening documents failed', err);
        }
        conn.synced = true;
    }

    // --- Workspace ----------------------------------------------------------------------------

    // One LSP document per open file tab. version and doc are what the server has; views are the live editors of the
    // tab that carry this client's plugin (two in a split of the same file).
    class LspWorkspaceFile {
        constructor(workspace, uri, path, languageId, doc) {
            this.workspace = workspace;
            this.uri = uri;
            this.path = path;
            this.languageId = languageId;
            this.version = 0;
            this.doc = doc;
            this.views = new Set();
            this.contentRef = null;
        }

        getView(main) {
            if (main && this.views.has(main)) return main;
            const tab = this.workspace.tabFor(this.path);
            const focused = tab && (tab.focusedView || tab.view);
            if (focused && this.views.has(focused)) return focused;
            const first = this.views.values().next();
            return first.done ? null : first.value;
        }
    }

    // The bundle loads CodeMirror lazily, so the subclass is made on first use.
    function lspWorkspaceClass(cm) {
        if (lspWorkspaceClassCache && lspWorkspaceClassCache.base === cm.Workspace) return lspWorkspaceClassCache.cls;
        class CodeStudioLSPWorkspace extends cm.Workspace {
            constructor(client, conn) {
                super(client);
                this.conn = conn;
                this.files = [];
                this.pruneTimer = 0;
            }

            // A restart gives the connection a new client; the old workspace then stays silent.
            get live() {
                return this.conn.client === this.client;
            }

            get synced() {
                return this.live && this.conn.synced;
            }

            tabFor(path) {
                return this.conn.instance.openTabs.find(tab => isSourceTab(tab) && tab.path === path) || null;
            }

            // Servers may spell a URI differently (percent-encoding); the canonical form is the key.
            getFile(uri) {
                const path = LSP_PROTOCOL.pathForUri(uri);
                const canonical = path ? LSP_PROTOCOL.uriForPath(path) : String(uri || '');
                return this.files.find(file => file.uri === canonical) || null;
            }

            syncFiles() {
                if (this.live) lspTouchConnection(this.conn);
                if (!this.synced) return [];
                const updates = [];
                for (const file of this.files) {
                    const update = lspSyncWorkspaceFile(this, file);
                    if (update) updates.push(update);
                }
                return updates;
            }

            openFile(uri, languageId, view) {
                const known = this.getFile(uri);
                if (known) {
                    known.views.add(view);
                    // A view rebuilt from a snapshot can hold edits the server has not seen yet.
                    if (this.synced && !view.state.doc.eq(known.doc)) setTimeout(() => { if (this.synced) this.client.sync(); }, 0);
                    return;
                }
                const path = LSP_PROTOCOL.pathForUri(uri);
                if (!path) return;
                const file = new LspWorkspaceFile(this, uri, path, languageId, view.state.doc);
                file.views.add(view);
                this.files.push(file);
                if (this.synced) this.client.didOpen(file);
            }

            // Every tab switch rebuilds the views; the document stays open on the server until its tab closes.
            closeFile(uri, view) {
                const file = this.getFile(uri);
                if (!file) return;
                file.views.delete(view);
                file.contentRef = null;
                if (!this.pruneTimer) this.pruneTimer = setTimeout(() => { this.pruneTimer = 0; this.prune(); }, 0);
            }

            prune() {
                this.files = this.files.filter(file => {
                    if (file.views.size || this.tabFor(file.path)) return true;
                    if (this.synced) this.client.didClose(file.uri);
                    return false;
                });
            }

            // Documents are (re)opened by lspOpenDocuments right after "initialized".
            connected() {}

            disconnected() {}

            displayFile(uri) {
                return lspDisplayFile(this.conn.instance, uri);
            }
        }
        lspWorkspaceClassCache = { base: cm.Workspace, cls: CodeStudioLSPWorkspace };
        return CodeStudioLSPWorkspace;
    }

    // Cross-file jumps: open the tab (or activate it) and hand back its focused view. openFile is core.js's.
    async function lspDisplayFile(instance, uri) {
        const path = LSP_PROTOCOL.pathForUri(uri);
        if (!path || !isLiveInstance(instance)) return null;
        try {
            await runAsyncStep(instance, () => openFile(path));
        } catch (_) {
            return null;
        }
        if (!isLiveInstance(instance)) return null;
        const view = runWithInstance(instance, () => lspViewForPath(path));
        if (view) view.focus();
        return view;
    }

    // One file's update for client.sync(): the plugin's own changes when its base is what the server has, otherwise
    // (rebuilt view, buffer replaced while no view was mounted) the whole text as one replacement.
    function lspSyncWorkspaceFile(workspace, file) {
        const cm = workspace.conn.instance.cmModule;
        const view = file.getView();
        const plugin = view && cm.LSPPlugin.get(view);
        let doc;
        let changes;
        if (plugin) {
            doc = view.state.doc;
            if (doc === file.doc) {
                lspClearFilePlugins(cm, file);
                return null;
            }
            const base = plugin.syncedDoc;
            if (base === file.doc || base.eq(file.doc)) {
                if (plugin.unsyncedChanges.empty) {
                    file.doc = doc;
                    lspClearFilePlugins(cm, file);
                    return null;
                }
                changes = plugin.unsyncedChanges;
            } else if (doc.eq(file.doc)) {
                file.doc = doc;
                lspClearFilePlugins(cm, file);
                return null;
            } else {
                changes = cm.ChangeSet.of([{ from: 0, to: file.doc.length, insert: doc }], file.doc.length);
            }
        } else {
            const tab = workspace.tabFor(file.path);
            if (!tab || tab.content === file.contentRef) return null;
            file.contentRef = tab.content;
            const text = String(tab.content == null ? '' : tab.content);
            if (text === file.doc.toString()) return null;
            doc = lspTextOf(cm, text);
            changes = cm.ChangeSet.of([{ from: 0, to: file.doc.length, insert: doc }], file.doc.length);
        }
        const prevDoc = file.doc;
        file.doc = doc;
        file.version += 1;
        lspClearFilePlugins(cm, file);
        return { file, prevDoc, changes };
    }

    function lspTextOf(cm, content) {
        return cm.Text.of(String(content == null ? '' : content).split(/\r\n?|\n/));
    }

    function lspClearFilePlugins(cm, file) {
        file.views.forEach(view => {
            const plugin = cm.LSPPlugin.get(view);
            if (plugin) plugin.clear();
        });
    }

    // --- Diagnostics and messages -------------------------------------------------------------

    function lspHandleDiagnostics(conn, client, params) {
        if (conn.client !== client || !params || !isLiveInstance(conn.instance)) return;
        const path = LSP_PROTOCOL.pathForUri(params.uri);
        if (!path) return;
        const file = client.workspace.getFile(params.uri);
        // Findings for an older version of an open document would land on the wrong lines; a newer list follows.
        if (file && typeof params.version === 'number' && params.version !== file.version) return;
        const source = LSP_PROTOCOL.sourceForLanguage(conn.language);
        runWithInstance(conn.instance, () => replaceProblemsForFile(path, source, LSP_PROTOCOL.diagnosticsToProblems(path, params.diagnostics, source)));
    }

    function lspHandleShowMessage(conn, params) {
        if (!params || (params.type !== 1 && params.type !== 2) || !isLiveInstance(conn.instance)) return;
        runWithInstance(conn.instance, () => flashStatus(LSP_SERVER_NAMES[conn.language] + ': ' + String(params.message || ''), 6000));
    }

    function lspClearLanguageProblems(conn) {
        if (!isLiveInstance(conn.instance)) return;
        const source = LSP_PROTOCOL.sourceForLanguage(conn.language);
        runWithInstance(conn.instance, () => {
            const current = state.problems || [];
            const kept = current.filter(problem => problem.source !== source);
            if (kept.length !== current.length) setProblems(kept);
        });
    }

    // A ready server reports syntax errors itself; the bracket check's findings for its files would be duplicates.
    function lspDropSyntaxProblems(conn) {
        if (!isLiveInstance(conn.instance)) return;
        runWithInstance(conn.instance, () => {
            const current = state.problems || [];
            const kept = current.filter(problem => !(problem.source === 'syntax' && LSP_PROTOCOL.languageForPath(problem.file) === conn.language));
            if (kept.length !== current.length) setProblems(kept);
        });
    }

    function lspServesPath(path) {
        const language = LSP_PROTOCOL ? LSP_PROTOCOL.languageForPath(path) : null;
        return !!language && lspClientState(language) === 'ready';
    }

    // --- Editor extensions --------------------------------------------------------------------

    // The content of view.csLsp for one tab. Empty while LSP is off, read-only, not a server language, or the
    // session's client never connected; only a focus handler while the server limit is reached.
    function lspEditorExtensions(cm, tab) {
        const instance = state;
        const available = lspAvailable(instance, cm);
        if (instance) instance.lspAvailableApplied = available;
        if (!available || !isSourceTab(tab)) return [];
        const language = LSP_PROTOCOL.languageForPath(tab.path);
        const uri = language ? LSP_PROTOCOL.uriForPath(tab.path) : null;
        if (!uri) return [];
        const conn = lspConnection(instance, language, true);
        if (conn.state === 'too-many') {
            return [cm.EditorView.domEventHandlers({ focus: () => { lspRetryTooMany(instance, language); return false; } })];
        }
        // An idle-closed session reconnects when an editor opens.
        lspTouchConnection(conn);
        if (!conn.client || !conn.linked) return [];
        const extras = [];
        lspActionExtensionFactories.forEach(factory => {
            try {
                const result = factory(cm, { language, path: tab.path, tab });
                if (result) extras.push(result);
            } catch (err) {
                console.warn('[lsp] action extension failed', err);
            }
        });
        return [
            conn.client.plugin(uri, LSP_PROTOCOL.languageIdForPath(tab.path)),
            cm.serverCompletion(),
            cm.hoverTooltips(),
            // Signature help on ( and , plus Ctrl+Shift+Space (Cmd+Shift+Space on macOS).
            cm.signatureHelp(),
            cm.Prec.high(cm.keymap.of(cm.jumpToDefinitionKeymap)),
            cm.Prec.high(cm.EditorView.domEventHandlers({ mousedown: (event, view) => lspModClick(instance, language, event, view) })),
            cm.Prec.high(cm.EditorView.clickAddsSelectionRange.of(event => lspAddsCursor(instance, language, event))),
            cm.EditorView.exceptionSink.of(lspExceptionSink),
            extras
        ];
    }

    function lspIsMac() {
        return /Mac|iPhone|iPad/.test(navigator.platform || '');
    }

    // Ctrl+Click (Cmd+Click on macOS) jumps to the definition while the file's server is ready.
    function lspModClick(instance, language, event, view) {
        if (event.button !== 0 || event.altKey || event.shiftKey || !(lspIsMac() ? event.metaKey : event.ctrlKey)) return false;
        const conn = lspConnectionOf(instance, language);
        if (!conn || conn.state !== 'ready') return false;
        const pos = view.posAtCoords({ x: event.clientX, y: event.clientY });
        if (pos == null) return false;
        event.preventDefault();
        view.dispatch({ selection: { anchor: pos } });
        view.focus();
        instance.cmModule.jumpToDefinition(view);
        return true;
    }

    // While the server is ready, extra cursors move to Alt+Click, as in VS Code; otherwise the default stays.
    function lspAddsCursor(instance, language, event) {
        const conn = lspConnectionOf(instance, language);
        if (conn && conn.state === 'ready') return event.altKey;
        return lspIsMac() ? event.metaKey : event.ctrlKey;
    }

    // Timeouts and cancelled requests of hover and completion are routine while a server starts or is busy.
    function lspExceptionSink(error) {
        const message = String(error && error.message ? error.message : error);
        if (LSP_QUIET_ERRORS.test(message) || (error && LSP_QUIET_ERROR_CODES.includes(error.code))) console.warn('[lsp] ' + message);
        else console.error(error);
    }

    // Allowlist sanitizer for the Markdown HTML of hover, completion and signature docs. Markup outside the list is
    // unwrapped to its text, active content is dropped, and only class (highlighting) and http(s) links survive.
    // Parsing happens in an inert <template>, so nothing loads or runs on the way.
    function lspSanitizeHTML(html) {
        const template = document.createElement('template');
        template.innerHTML = String(html == null ? '' : html);
        lspSanitizeNode(template.content);
        const box = document.createElement('div');
        box.appendChild(template.content);
        return box.innerHTML;
    }

    function lspSanitizeNode(parent) {
        Array.from(parent.childNodes).forEach(node => {
            if (node.nodeType === 3) return;
            if (node.nodeType !== 1) {
                node.remove();
                return;
            }
            const tag = String(node.nodeName).toUpperCase();
            if (LSP_SANITIZE_DROP.has(tag)) {
                node.remove();
                return;
            }
            lspSanitizeNode(node);
            if (!LSP_SANITIZE_KEEP.has(tag)) {
                node.replaceWith(...Array.from(node.childNodes));
                return;
            }
            Array.from(node.attributes).forEach(attribute => {
                const name = attribute.name.toLowerCase();
                const keep = name === 'class' || (tag === 'A' && name === 'href' && /^https?:\/\//i.test(attribute.value.trim()));
                if (!keep) node.removeAttribute(attribute.name);
            });
            if (tag === 'A' && node.hasAttribute('href')) {
                node.setAttribute('target', '_blank');
                node.setAttribute('rel', 'noopener noreferrer');
            }
        });
    }

    // --- Status bar ---------------------------------------------------------------------------

    function lspStateText(stateName) {
        switch (stateName) {
            case 'starting': return tr('codeStudio.lspStarting', 'Starting...');
            case 'ready': return tr('codeStudio.lspReady', 'Ready');
            case 'disconnected': return tr('codeStudio.lspDisconnected', 'Disconnected');
            case 'not-installed': return workspaceIsRemote()
                ? tr('codeStudio.workspaceLspNotInstalled', 'not installed on {{host}}', { host: workspaceHostLabel() }) // Plan C
                : tr('codeStudio.lspNotInstalled', 'Language server not installed — rebuild the Code Studio image');
            case 'too-many': return tr('codeStudio.lspTooMany', 'Too many language servers');
            case 'error': return tr('codeStudio.lspError', 'Error — click to restart');
            default: return tr('codeStudio.lspIdle', 'Idle');
        }
    }

    // The active file's server: state dot plus label; a click restarts it (renderStatus wires the click).
    function lspStatusMarkup(tab) {
        const instance = state;
        if (!isSourceTab(tab) || !lspAvailable(instance, instance && instance.cmModule)) return '';
        const language = LSP_PROTOCOL.languageForPath(tab.path);
        const conn = language ? lspConnectionOf(instance, language) : null;
        if (!conn || (conn.state === 'off' && !conn.client)) return '';
        const label = tr('codeStudio.lspStatus', '{{server}}: {{state}}', { server: LSP_SERVER_NAMES[language], state: lspStateText(conn.state) });
        const hint = tr('codeStudio.lspRestartHint', 'Click to restart the language server');
        return `<span class="cs-status-item cs-status-lsp" data-clickable data-status-lsp="${esc(language)}" data-lsp-state="${esc(conn.state)}" title="${esc(label + '\n' + hint)}"><span class="cs-lsp-dot" data-lsp-state="${esc(conn.state)}"></span><span class="cs-status-lsp-label">${esc(label)}</span></span>`;
    }

    // --- Code Studio writes ---------------------------------------------------------------------

    // After a branch switch: the changed files from git (open tabs as the fallback) go to the ready servers.
    function lspNotifyBranchSwitch() {
        const instance = state;
        if (!instance || !instance.lsp || !LSP_LANGUAGES.some(language => lspClientState(language) === 'ready')) return;
        const fallback = instance.openTabs.filter(isSourceTab).map(tab => ({ path: tab.path, type: 2 }));
        Promise.resolve(apiClient.exec(LSP_BRANCH_DIFF_COMMAND, WORKSPACE_ROOT, 30)).then(result => {
            if (!isLiveInstance(instance)) return;
            const changes = lspParseNameStatus(result && result.output);
            runWithInstance(instance, () => lspNotifyFilesChanged(changes.concat(fallback)));
        }, () => {
            if (isLiveInstance(instance)) runWithInstance(instance, () => lspNotifyFilesChanged(fallback));
        });
    }

    function lspParseNameStatus(output) {
        const types = { A: 1, M: 2, T: 2, D: 3 };
        return String(output || '').split('\n')
            .map(line => /^([AMTD])\t([^"].*)$/.exec(line.replace(/\r$/, '')))
            .filter(Boolean)
            .map(match => ({ path: match[2], type: types[match[1]] }));
    }

    // --- Contract API for plan 2c ---------------------------------------------------------------

    // timeoutMs (internal, for lspRequest) bounds the wait of a starting or disconnected session: Error('timeout').
    function lspEnsureClient(language, timeoutMs) {
        const instance = state;
        if (!LSP_LANGUAGES.includes(language) || !lspAvailable(instance, instance && instance.cmModule)) return Promise.resolve(null);
        const conn = lspConnection(instance, language, true);
        lspTouchConnection(conn);
        if (conn.state === 'ready' && conn.client) return Promise.resolve(conn.client);
        if (conn.state !== 'starting' && conn.state !== 'disconnected') return Promise.resolve(null);
        return new Promise((resolve, reject) => {
            let timer = 0;
            const waiter = client => {
                clearTimeout(timer);
                resolve(client);
            };
            conn.waiters.push(waiter);
            if (timeoutMs > 0) {
                timer = setTimeout(() => {
                    const index = conn.waiters.indexOf(waiter);
                    if (index >= 0) conn.waiters.splice(index, 1);
                    reject(new Error('timeout'));
                }, timeoutMs);
            }
        });
    }

    function lspClientState(language) {
        const instance = state;
        const conn = lspConnectionOf(instance, language);
        if (!conn || !lspAvailable(instance, instance.cmModule)) return 'off';
        return conn.state;
    }

    // Own id space and own timer: the default 3 s budget holds even though the LSPClient itself waits up to 15 s.
    async function lspRequest(language, method, params, timeoutMs = LSP_REQUEST_TIMEOUT_MS) {
        const instance = state;
        const budget = Math.max(1, Number(timeoutMs) || LSP_REQUEST_TIMEOUT_MS);
        const startedAt = Date.now();
        const client = await lspEnsureClient(language, budget);
        timeoutMs = Math.max(1, budget - (Date.now() - startedAt));
        const conn = lspConnectionOf(instance, language);
        if (!client || !conn || conn.client !== client || !isLiveInstance(instance)) throw new Error('unavailable');
        lspTouchConnection(conn);
        client.sync();
        const id = 'cs-' + (++lspRequestSequence);
        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                if (!conn.pending.delete(id)) return;
                lspSendRaw(conn, { jsonrpc: '2.0', method: '$/cancelRequest', params: { id } });
                reject(new Error('timeout'));
            }, timeoutMs);
            conn.pending.set(id, { resolve, reject, timer });
            // The didChange that sync() queued goes out first: both wait on the same resolved promise.
            client.initializing.then(() => {
                if (!conn.pending.has(id)) return;
                if (!lspSendRaw(conn, { jsonrpc: '2.0', id, method, params })) {
                    clearTimeout(timer);
                    conn.pending.delete(id);
                    reject(new Error('disconnected'));
                }
            }, () => {
                if (!conn.pending.delete(id)) return;
                clearTimeout(timer);
                reject(new Error('disconnected'));
            });
        });
    }

    function lspOnServerRequest(method, handler) {
        if (typeof method === 'string' && method && typeof handler === 'function') LSP_SERVER_REQUEST_HANDLERS.set(method, handler);
    }

    function lspNotifyFilesChanged(changes) {
        const instance = state;
        if (!instance || !instance.lsp || !LSP_PROTOCOL) return;
        const ready = LSP_LANGUAGES.map(language => lspConnectionOf(instance, language))
            .filter(conn => conn && conn.state === 'ready' && conn.synced && conn.client);
        if (!ready.length) return;
        const events = [];
        (Array.isArray(changes) ? changes : []).forEach(change => {
            const path = change ? normalizeCodeStudioPath(change.path) : WORKSPACE_ROOT;
            const uri = path !== WORKSPACE_ROOT ? LSP_PROTOCOL.uriForPath(path) : null;
            if (!uri) return;
            events.push({ uri, language: LSP_PROTOCOL.languageForPath(path), type: [1, 2, 3].includes(change.type) ? change.type : 2 });
        });
        ready.forEach(conn => {
            // Source files go to their own server; project files (go.mod, package.json, tsconfig.json, ...) to every one.
            const mine = events.filter(event => !event.language || event.language === conn.language);
            if (!mine.length) return;
            lspTouchConnection(conn);
            conn.client.sync();
            mine.forEach(event => {
                if (event.type === 2 && conn.client.workspace.getFile(event.uri)) {
                    conn.client.notification('textDocument/didSave', { textDocument: { uri: event.uri } });
                }
            });
            conn.client.notification('workspace/didChangeWatchedFiles', { changes: mine.map(event => ({ uri: event.uri, type: event.type })) });
        });
    }

    function lspViewForPath(path) {
        if (!state) return null;
        const wanted = normalizeCodeStudioPath(path);
        const tab = state.openTabs.find(item => isSourceTab(item) && item.path === wanted);
        const view = tab && (tab.focusedView || tab.view);
        return view && view.state ? view : null;
    }

    function lspTouch(language) {
        lspTouchConnection(lspConnectionOf(state, language));
    }

    // Server-to-client requests the page answers itself. Plan 2c replaces the workspace/applyEdit default through
    // lspOnServerRequest (the registry is a Map, so the later registration wins).
    lspOnServerRequest('workspace/applyEdit', () => ({ applied: false, failureReason: 'Not supported yet' }));
    ['workspace/diagnostic/refresh', 'workspace/semanticTokens/refresh', 'workspace/codeLens/refresh',
        'workspace/inlayHint/refresh', 'workspace/inlineValue/refresh'].forEach(method => lspOnServerRequest(method, () => null));
    lspOnServerRequest('workspace/configuration', (_language, params) => (params && Array.isArray(params.items) ? params.items.map(() => null) : []));
    lspOnServerRequest('window/workDoneProgress/create', () => null);
    lspOnServerRequest('client/registerCapability', () => null);
    lspOnServerRequest('client/unregisterCapability', () => null);
    lspOnServerRequest('window/showMessageRequest', (language, params) => {
        if (params && params.message) flashStatus(LSP_SERVER_NAMES[language] + ': ' + String(params.message), 6000);
        return null;
    });
