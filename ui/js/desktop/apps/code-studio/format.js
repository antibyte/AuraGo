    // IDE package 1A: Format Document (A6) and the save actions of the editor settings.
    // Go runs gofmt in the container and JavaScript/TypeScript the TypeScript formatter of the container; both read
    // the text from stdin through /api/code-studio/exec. JSON is formatted in the browser. Other languages have no
    // formatter in the image and get a notice. A result is applied as one CodeMirror transaction with its own undo
    // step, and only when the text changed.
    const FORMAT_SCRIPT_EXTENSIONS = ['js', 'mjs', 'cjs', 'jsx', 'ts', 'tsx'];

    // Runs inside the container as: sh -c FORMAT_TS_SHELL cs-format <script> <options>. NODE_PATH points node at the
    // global packages, so require('typescript') finds the compiler the image installs with npm install -g.
    const FORMAT_TS_SHELL = 'NODE_PATH="$(npm root -g 2>/dev/null)" CS_FORMAT_OPTIONS="$2" node -e "$1" 2>/dev/null';

    const FORMAT_TS_SCRIPT = [
        "let ts = null;",
        "try { ts = require('typescript'); } catch (err) { process.stdout.write('typescript is not installed in the container: ' + err.message); process.exitCode = 2; }",
        "let input = '';",
        "process.stdin.setEncoding('utf8');",
        "process.stdin.on('data', chunk => { input += chunk; });",
        "process.stdin.on('end', () => {",
        "  if (!ts) return;",
        "  try {",
        "    const opts = JSON.parse(process.env.CS_FORMAT_OPTIONS || '{}');",
        "    const fileName = opts.fileName || '/cs-format/input.ts';",
        "    const host = {",
        "      getScriptFileNames: () => [fileName],",
        "      getScriptVersion: () => '1',",
        "      getScriptSnapshot: name => name === fileName ? ts.ScriptSnapshot.fromString(input) : undefined,",
        "      getCurrentDirectory: () => '/cs-format',",
        "      getCompilationSettings: () => ({ allowJs: true, jsx: ts.JsxEmit.Preserve, target: ts.ScriptTarget.Latest }),",
        "      getDefaultLibFileName: () => 'lib.d.ts',",
        "      fileExists: name => name === fileName,",
        "      readFile: name => name === fileName ? input : undefined",
        "    };",
        "    const service = ts.createLanguageService(host, ts.createDocumentRegistry());",
        "    const settings = ts.getDefaultFormatCodeSettings(opts.newLine === 'crlf' ? String.fromCharCode(13, 10) : String.fromCharCode(10));",
        "    settings.indentSize = opts.indentSize;",
        "    settings.tabSize = opts.tabSize;",
        "    settings.convertTabsToSpaces = opts.insertSpaces;",
        "    const edits = service.getFormattingEditsForDocument(fileName, settings).slice().sort((a, b) => b.span.start - a.span.start);",
        "    let output = input;",
        "    for (const edit of edits) output = output.slice(0, edit.span.start) + edit.newText + output.slice(edit.span.start + edit.span.length);",
        "    process.stdout.write(output);",
        "  } catch (err) {",
        "    process.stdout.write('format failed: ' + (err && err.stack || err));",
        "    process.exitCode = 2;",
        "  }",
        "});"
    ].join('\n');

    function formatterKind(tab) {
        if (!isSourceTab(tab)) return '';
        const language = tab.language || languageForPath(tab.path);
        if (language === 'go') return 'gofmt';
        if (language === 'json') return 'json';
        if (['javascript', 'jsx', 'typescript', 'tsx'].includes(language)) return 'typescript';
        return '';
    }

    function formatJsonText(text, settings) {
        // Numbers keep their source text (JSON.rawJSON), so large integers survive the round trip.
        const reviver = typeof JSON.rawJSON === 'function'
            ? (key, value, context) => (typeof value === 'number' && context && typeof context.source === 'string' ? JSON.rawJSON(context.source) : value)
            : undefined;
        const parsed = JSON.parse(text, reviver);
        const newline = text.includes('\r\n') ? '\r\n' : '\n';
        let output = JSON.stringify(parsed, null, settings.insertSpaces ? settings.tabSize : '\t');
        if (newline !== '\n') output = output.split('\n').join(newline);
        return /\r?\n$/.test(text) ? output + newline : output;
    }

    function formatScriptOptions(tab, text, settings) {
        const ext = String(tab.path || '').split('.').pop().toLowerCase();
        return {
            fileName: '/cs-format/input.' + (FORMAT_SCRIPT_EXTENSIONS.includes(ext) ? ext : 'ts'),
            indentSize: settings.tabSize,
            tabSize: settings.tabSize,
            insertSpaces: settings.insertSpaces,
            newLine: text.includes('\r\n') ? 'crlf' : 'lf'
        };
    }

    // Returns { text } or { error }.
    async function runFormatter(kind, tab, text) {
        const settings = editorSettings();
        if (kind === 'json') {
            try {
                return { text: formatJsonText(text, settings) };
            } catch (err) {
                return { error: err && err.message ? err.message : String(err) };
            }
        }
        const args = kind === 'gofmt'
            ? ['gofmt']
            : ['sh', '-c', FORMAT_TS_SHELL, 'cs-format', FORMAT_TS_SCRIPT, JSON.stringify(formatScriptOptions(tab, text, settings))];
        const result = await apiClient.execInput(args, text, 60);
        const output = String(result && result.output || '');
        if (result && result.truncated) return { error: tr('codeStudio.formatTruncated', 'The formatter output was too large and was not applied.') };
        if (!result || Number(result.exit_code) !== 0) return { error: output.trim() || tr('codeStudio.formatFailed', 'Formatting failed. Details are in the Output panel.') };
        // A formatter that prints nothing for a non-empty file failed; applying that would wipe the document.
        if (text.trim() && !output.trim()) return { error: tr('codeStudio.formatFailed', 'Formatting failed. Details are in the Output panel.') };
        return { text: output };
    }

    function minimalTextChange(current, next) {
        let from = 0;
        const limit = Math.min(current.length, next.length);
        while (from < limit && current.charCodeAt(from) === next.charCodeAt(from)) from++;
        let toCurrent = current.length;
        let toNext = next.length;
        while (toCurrent > from && toNext > from && current.charCodeAt(toCurrent - 1) === next.charCodeAt(toNext - 1)) {
            toCurrent--;
            toNext--;
        }
        return { from, to: toCurrent, insert: next.slice(from, toNext) };
    }

    // Replaces the text of a tab as one transaction with its own undo step. A mounted CodeMirror view gets the
    // smallest change, the textarea fallback its new value; a tab without a live view updates its text and its saved
    // editor state. Returns false when the text is already equal.
    function applyTabText(tab, next) {
        const current = editorValue(tab);
        if (next === current) return false;
        const change = minimalTextChange(current, next);
        const cm = state.cmModule;
        const annotations = cm && cm.isolateHistory ? [cm.isolateHistory.of('full')] : [];
        if (tab.view && tab.view.state && typeof tab.view.dispatch === 'function') {
            tab.view.dispatch({ changes: change, annotations });
            return true;
        }
        if (tab.view && typeof tab.view.setValue === 'function') {
            tab.view.setValue(next);
            return true;
        }
        const snapshot = tab.editorSnapshot;
        if (snapshot && snapshot.state && snapshot.state.doc && snapshot.state.doc.toString() === current) {
            tab.editorSnapshot = Object.assign({}, snapshot, { state: snapshot.state.update({ changes: change, annotations }).state });
        } else {
            tab.editorSnapshot = null;
        }
        tab.secondaryEditorSnapshot = null;
        updateTabContent(tab, next);
        return true;
    }

    // Formats one tab. quiet (format on save) skips the notices except failures; errors always go to the Output panel.
    async function formatTab(target, tab, options) {
        const quiet = !!(options && options.quiet);
        if (!isLiveInstance(target) || !isSourceTab(tab) || !target.openTabs.includes(tab)) return false;
        const kind = formatterKind(tab);
        if (!kind) {
            if (!quiet) {
                runWithInstance(target, () => flashStatus(tr('codeStudio.formatUnavailable', 'No formatter available for {{language}}', { language: tab.language || baseName(tab.path) })));
            }
            return false;
        }
        if (!canWrite(target)) return false;
        const source = editorValue(tab);
        const revision = tab.revision || 0;
        let result;
        try {
            result = await runFormatter(kind, tab, source);
        } catch (err) {
            result = { error: err && err.message ? err.message : String(err) };
        }
        if (!isLiveInstance(target) || !target.openTabs.includes(tab)) return false;
        return runWithInstance(target, () => {
            if (result.error) {
                appendOutput(result.error, tr('codeStudio.formatOutputSource', 'Format'));
                flashStatus(tr('codeStudio.formatFailed', 'Formatting failed. Details are in the Output panel.'));
                if (!quiet) setActivePanel('output');
                return false;
            }
            if ((tab.revision || 0) !== revision || editorValue(tab) !== source) {
                if (!quiet) flashStatus(tr('codeStudio.formatStale', 'The document changed while it was being formatted. Format it again.'));
                return false;
            }
            const changed = applyTabText(tab, result.text);
            if (!quiet) {
                flashStatus(changed
                    ? tr('codeStudio.formatDone', 'Document formatted')
                    : tr('codeStudio.formatUnchanged', 'The document is already formatted'));
            }
            return changed;
        });
    }

    function formatActiveDocument() {
        const target = state;
        const tab = activeTab();
        if (!isSourceTab(tab) || !canWrite(target)) return Promise.resolve(false);
        return formatTab(target, tab, { quiet: false });
    }

    function tabNeedsSaveActions(target, tab) {
        const settings = editorSettings();
        return canWrite(target) && isSourceTab(tab) && (settings.formatOnSave || settings.trimTrailingWhitespace || settings.insertFinalNewline);
    }

    // Format on save runs first; trimming trailing whitespace and the final newline follow as one more transaction.
    async function runSaveActions(target, tab) {
        const settings = editorSettings();
        if (settings.formatOnSave && formatterKind(tab)) await formatTab(target, tab, { quiet: true });
        if (!isLiveInstance(target) || !target.openTabs.includes(tab)) return;
        if (!settings.trimTrailingWhitespace && !settings.insertFinalNewline) return;
        runWithInstance(target, () => {
            let text = editorValue(tab);
            if (settings.trimTrailingWhitespace) text = text.replace(/[ \t]+(?=\r?$)/gm, '');
            if (settings.insertFinalNewline && text && !text.endsWith('\n')) text += text.includes('\r\n') ? '\r\n' : '\n';
            applyTabText(tab, text);
        });
    }

    // Explicit saves (Ctrl+S, Save, Save All, Run) come through here; auto save and the desktop's session save call
    // saveTab directly and write the buffer as it is.
    function saveTabWithActions(target, tab) {
        if (!tabNeedsSaveActions(target, tab)) return saveTab(target, tab);
        return runSaveActions(target, tab).catch(() => {}).then(() => saveTab(target, tab));
    }
