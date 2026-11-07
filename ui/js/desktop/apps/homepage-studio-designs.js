/* Homepage Studio design picker — bundled DESIGN.md templates with previews.
   The catalog ships with AuraGo, so the picker works without network access.
   One selection belongs to one site: picking a design writes it into the
   project as DESIGN.md, which is what the agent follows while building. */
(function () {
    'use strict';

    const DESIGN_ID_PATTERN = /^[a-z0-9][a-z0-9-]{0,63}$/;
    const COLOR_PATTERN = /^#[0-9a-f]{6}$/i;

    /* Only a plain slug may reach the preview URL. */
    function safeDesignID(value) {
        const id = String(value == null ? '' : value).trim().toLowerCase();
        return DESIGN_ID_PATTERN.test(id) ? id : '';
    }

    function safeColor(value) {
        const color = String(value == null ? '' : value).trim();
        return COLOR_PATTERN.test(color) ? color : '';
    }

    function create(deps) {
        const { api, esc, t, notify, readonly, container, isDisposed, onDesignChange } = deps;

        let designs = [];
        let selectedId = '';
        let projectDir = '';
        let loading = false;
        let loadedFor = null;

        function text(key, fallback) {
            const value = t(key);
            return value === key ? fallback : value;
        }

        /* Translations use {name} or {{name}} placeholders. */
        function fill(template, key, value) {
            return String(template)
                .split('{{' + key + '}}').join(value)
                .split('{' + key + '}').join(value);
        }

        function previewURL(id, variant) {
            const safe = safeDesignID(id);
            if (!safe) return '';
            const mode = variant === 'dark' ? 'dark' : 'light';
            return '/api/homepage/designs/' + safe + '/preview?variant=' + mode;
        }

        function renderCard(design) {
            const id = safeDesignID(design.id);
            if (!id) return '';
            const selected = id === selectedId;
            const palette = (Array.isArray(design.palette) ? design.palette : [])
                .map(safeColor)
                .filter(Boolean)
                .map(color => `<i class="vd-hp-design-swatch" style="background-color:${color}"></i>`)
                .join('');
            const likes = Number(design.likes) > 0 ? `<span class="vd-hp-design-likes">${esc(Number(design.likes))} &#9825;</span>` : '';
            const author = design.author ? `<span class="vd-hp-design-author">@${esc(design.author)}</span>` : '';
            const src = previewURL(id, 'light');
            return `<button type="button" class="vd-hp-design-card${selected ? ' is-selected' : ''}" data-hp-design="${esc(id)}" aria-pressed="${selected ? 'true' : 'false'}">
                <span class="vd-hp-design-preview">
                    ${src ? `<img src="${esc(src)}" alt="${esc(design.name || id)}" loading="lazy" decoding="async">` : ''}
                </span>
                <span class="vd-hp-design-info">
                    <span class="vd-hp-design-name" title="${esc(design.name || id)}">${esc(design.name || id)}</span>
                    <span class="vd-hp-design-sub">${author}${likes}</span>
                    ${palette ? `<span class="vd-hp-design-palette">${palette}</span>` : ''}
                </span>
            </button>`;
        }

        function renderEmpty(message) {
            container.innerHTML = `<div class="vd-hp-designs-empty">${esc(message)}</div>`;
        }

        function render() {
            if (!container) return;
            if (!designs.length) {
                renderEmpty(loading ? text('homepage_studio.designs_loading', 'Loading designs…') : text('homepage_studio.designs_empty', 'No design templates available.'));
                return;
            }
            const cards = designs.map(renderCard).join('');
            const bound = designs.find(design => safeDesignID(design.id) === selectedId);
            const headline = bound
                ? fill(text('homepage_studio.designs_selected', 'Selected: {name}'), 'name', bound.name || bound.id)
                : text('homepage_studio.designs_headline', 'Pick a design system for this site.');
            const scope = projectDir
                ? fill(text('homepage_studio.designs_scope_site', 'Applies to the selected site ({dir}).'), 'dir', projectDir)
                : text('homepage_studio.designs_scope_new', 'Applies to the next site you build in this window.');
            container.innerHTML = `<div class="vd-hp-designs-head">
                    <p class="vd-hp-designs-title">${esc(headline)}</p>
                    <p class="vd-hp-designs-scope">${esc(scope)}</p>
                </div>
                <div class="vd-hp-designs-grid" role="listbox" aria-label="${esc(text('homepage_studio.designs_tab', 'Designs'))}">${cards}</div>`;
        }

        async function load(dir) {
            if (dir !== undefined) projectDir = String(dir == null ? '' : dir).trim();
            if (loadedFor === projectDir && !loading) { render(); return; }
            loadedFor = projectDir;
            loading = true;
            render();
            try {
                const query = projectDir ? '?project_dir=' + encodeURIComponent(projectDir) : '';
                const data = await api('/api/homepage/designs' + query);
                if (isDisposed && isDisposed()) return;
                designs = Array.isArray(data && data.designs) ? data.designs : [];
                const serverSelected = safeDesignID(data && data.selected_design);
                /* The binding lives with the site: a site without a design must not
                   keep the design of the site that was selected before. Without a
                   site the local selection stays, because it rides along with the
                   next chat turn. */
                if (serverSelected || projectDir) selectedId = serverSelected;
            } catch (err) {
                if (isDisposed && isDisposed()) return;
                designs = [];
                if (notify) notify(text('homepage_studio.designs_load_failed', 'Could not load the design templates.'), 'error');
            } finally {
                loading = false;
                render();
            }
        }

        async function select(designId) {
            const id = safeDesignID(designId);
            if (!id) return;
            if (readonly) {
                if (notify) notify(text('homepage_studio.designs_readonly', 'The design selection is read-only here.'), 'error');
                return;
            }
            const previous = selectedId;
            selectedId = id;
            render();
            if (!projectDir) {
                /* No site yet: the selection rides along with the next chat turn. */
                if (onDesignChange) onDesignChange(id);
                return;
            }
            try {
                await api('/api/homepage/designs/select', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ project_dir: projectDir, design_id: id })
                });
                if (notify) notify(text('homepage_studio.designs_applied', 'Design applied to the site.'), 'success');
                if (onDesignChange) onDesignChange(id);
            } catch (err) {
                selectedId = previous;
                render();
                if (notify) notify(text('homepage_studio.designs_apply_failed', 'Could not apply the design.'), 'error');
            }
        }

        if (container) {
            container.addEventListener('click', event => {
                const card = event.target.closest('[data-hp-design]');
                if (!card || !container.contains(card)) return;
                select(card.getAttribute('data-hp-design'));
            });
        }

        return {
            load,
            render,
            select,
            /* The binding belongs to one site: a site without a design must not
               inherit the design of the site that was selected before. */
            setSite(detail) {
                projectDir = detail && detail.project_dir ? detail.project_dir : '';
                selectedId = detail && detail.design_id ? detail.design_id : '';
                loadedFor = null;
                render();
            },
            /* An empty id clears the selection: a site without a design has none. */
            setSelectedId(id) {
                const raw = String(id == null ? '' : id).trim();
                if (raw === '') { selectedId = ''; render(); return; }
                const safe = safeDesignID(raw);
                if (safe) { selectedId = safe; render(); }
            },
            getSelectedId() { return selectedId; },
            getProjectDir() { return projectDir; },
            /* The chat turn carries the design and the site it belongs to. */
            chatContext() {
                const context = {};
                if (selectedId) context.design_id = selectedId;
                if (projectDir) context.project_dir = projectDir;
                return context;
            },
            dispose() { designs = []; }
        };
    }

    /* The inspector tab and its panel belong to the module, so the workbench
       shell stays small and the picker owns its own markup. */
    function tabMarkup(windowId, esc, tr) {
        return `<button type="button" class="vd-hp-preview-tab" id="hp-tab-designs-${windowId}" role="tab" aria-selected="false" aria-controls="hp-panel-designs-${windowId}" tabindex="-1">
                    ${esc(tr('homepage_studio.designs_tab', 'Designs'))}
                </button>`;
    }

    function panelMarkup(windowId) {
        return `<div class="vd-hp-inspector-panel vd-hp-designs-panel" id="hp-panel-designs-${windowId}" role="tabpanel" aria-labelledby="hp-tab-designs-${windowId}" hidden>
                    <div class="vd-hp-designs-list" id="hp-designs-list-${windowId}"></div>
                </div>`;
    }

    window.HomepageStudioDesigns = { create, tabMarkup, panelMarkup };
})();
