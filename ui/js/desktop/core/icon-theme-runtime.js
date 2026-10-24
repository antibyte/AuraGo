    function themedArtworkMarkup(path, key, fallback, className, size, action) {
        const pixels = Math.max(8, Math.min(256, Number(size) || 42));
        return `<span class="${esc(className || '')} vd-theme-icon vd-papirus-icon vd-theme-image-icon${action ? ' vd-mini-themed-icon' : ''}" aria-hidden="true" style="--vd-theme-icon-url:${esc(iconUrlStyle(path))};width:${pixels}px;height:${pixels}px"><img class="vd-theme-image" src="${esc(versionedIconAssetPath(path))}" alt="" draggable="false"><span class="vd-theme-image-fallback" hidden>${esc(String(fallback || key || '?').slice(0, 2).toUpperCase())}</span></span>`;
    }

    function trackedIconMarkup(markup, key, fallback, className, size, usage) {
        const attributes = ` data-vd-icon-key="${esc(key)}" data-vd-icon-fallback="${esc(fallback || '')}" data-vd-icon-class="${esc(className || '')}" data-vd-icon-size="${Number(size) || 16}" data-vd-icon-usage="${esc(usage || 'icon')}"`;
        return markup.replace(/^(<span|<svg)\b/, '$1' + attributes);
    }

    function refreshThemeIconElements(root) {
        (root || document).querySelectorAll('[data-vd-icon-key]').forEach(icon => {
            const data = icon.dataset;
            const template = document.createElement('template');
            template.innerHTML = iconMarkup(data.vdIconKey, data.vdIconFallback, data.vdIconClass, Number(data.vdIconSize), data.vdIconUsage);
            const replacement = template.content.firstElementChild;
            if (replacement) {
                if (icon.hasAttribute('hidden')) replacement.setAttribute('hidden', '');
                icon.replaceWith(replacement);
            }
        });
    }

    // Broken assets reveal a readable local fallback; an old theme never survives a refresh.
    document.addEventListener('error', event => {
        const image = event.target;
        if (!image || !image.matches || !image.matches('img.vd-theme-image')) return;
        image.hidden = true;
        if (image.nextElementSibling) image.nextElementSibling.hidden = false;
    }, true);

    let desktopIconAppearanceRevision = 0;
    let desktopIconAppearanceSignature = '';
    function desktopIconAppearance() {
        const settings = {};
        ['appearance.icon_theme', 'appearance.tela_folder_color', 'appearance.fluent_folder_color'].forEach(key => { settings[key] = settingValue(key); });
        return { settings, scheme: document.body.dataset.scheme || 'light', revision: desktopIconAppearanceRevision };
    }
    function broadcastDesktopIconAppearance() {
        const appearance = desktopIconAppearance();
        const signature = JSON.stringify([appearance.settings, appearance.scheme]);
        if (signature === desktopIconAppearanceSignature) return;
        desktopIconAppearanceSignature = signature;
        appearance.revision = ++desktopIconAppearanceRevision;
        for (const client of sdkFrameClients.values()) {
            if (!isCurrentSDKClient(client)) continue;
            try { client.port.postMessage({ type: 'aurago.desktop.appearance', payload: appearance }); }
            catch (_) { revokeSDKFrameClient(client.frame); }
        }
    }
