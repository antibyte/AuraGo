    function aiButtonMarkup(appId) {
        if (appId !== 'agent-chat') {
            return `<button class="vd-window-button vd-window-ai-button" type="button" data-action="ai-context" title="${esc(t('desktop.window_ai_context'))}" aria-label="${esc(t('desktop.window_ai_context'))}">${iconMarkup('chat', 'AI', 'vd-window-ai-button-icon', 16)}</button>`;
        }
        return '';
    }

    function windowContextString(value, maxLength) {
        const text = String(value || '').replace(/[\r\n]+/g, ' ').trim();
        if (!maxLength || text.length <= maxLength) return text;
        return text.slice(0, maxLength).trim();
    }

    function oliveTinWindowAIContext(base) {
        return Object.assign({}, base, {
            label: 'OliveTin',
            purpose: 'Web UI for running predefined shell automation actions.',
            guide: 'Use the virtual_desktop tool to read or edit the OliveTin config. Restart OliveTin from the Software Store if changes do not reload automatically.',
            resources: [{
                kind: 'desktop_file',
                label: 'OliveTin config',
                path: 'Shared/OliveTin/config.yaml',
                container_path: '/config/config.yaml'
            }]
        });
    }

    function buildWindowAIContext(windowId) {
        const item = state.windows.get(windowId);
        if (!item) return null;
        const app = appById(item.appId);
        const metadata = (app && app.metadata) || {};
        const storeAppId = windowContextString(metadata.store_app_id, 96);
        const fallbackPurpose = item.context && item.context.standaloneWidget ? 'Standalone desktop widget.' : 'Virtual Desktop application window.';
        const base = {
            source: 'desktop-window',
            app_id: windowContextString(item.appId, 128),
            store_app_id: storeAppId,
            window_id: windowContextString(item.id, 160),
            label: windowContextString(item.title || (app ? appName(app) : item.appId), 160),
            purpose: windowContextString((app && app.description) || fallbackPurpose, 500),
            guide: 'Use this window context to interpret references to the open app. If app-specific data is not included, ask for the missing detail instead of guessing.',
            resources: []
        };
        if (storeAppId === 'olivetin') return oliveTinWindowAIContext(base);
        return base;
    }

    function openAgentChatForWindow(windowId) {
        const context = buildWindowAIContext(windowId);
        if (!context) return;
        openApp('agent-chat', { window_context: context });
    }
