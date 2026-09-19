(function () {
    'use strict';
    const languages = ['de','en','fr','es','it','nl','pl','pt','cs','da','el','hi','ja','no','sv','zh'];
    function render(host, station, deps) {
        const { t, esc } = deps, p = JSON.parse(JSON.stringify(station));
        let step = 0;
        const field = (name, label, type, min, max) => '<label class="pr-field"><span>' + esc(t(label)) + '</span><input name="' + name + '" type="' + (type || 'text') + '" value="' + esc(p[name] == null ? '' : p[name]) + '"' + (min != null ? ' min="' + min + '"' : '') + (max != null ? ' max="' + max + '"' : '') + (type === 'number' ? ' required' : '') + '></label>';
        const select = (name, label, values) => '<label class="pr-field"><span>' + esc(t(label)) + '</span><select name="' + name + '">' + values.map(([value, text]) => '<option value="' + value + '"' + (String(p[name]) === String(value) ? ' selected' : '') + '>' + esc(text) + '</option>').join('') + '</select></label>';
        const check = (name, label) => '<label class="pr-check"><input name="' + name + '" type="checkbox"' + (p[name] ? ' checked' : '') + '><span>' + esc(t(label)) + '</span></label>';
        let timezones = []; try { timezones = Intl.supportedValuesOf('timeZone'); } catch (_) {}
        timezones = [...new Set([p.timezone || 'UTC','UTC',...timezones])];
        let display; try { display = new Intl.DisplayNames([document.documentElement.lang || 'en'], { type: 'language' }); } catch (_) {}
        host.innerHTML = '<form class="pr-settings" novalidate><nav class="pr-steps" aria-label="' + esc(t('settings')) + '">' + ['station','music','news','operation'].map((label, i) => '<button type="button" data-step="' + i + '">' + (i + 1) + ' · ' + esc(t(label)) + '</button>').join('') + '</nav>' +
            '<section data-panel="0"><h2>' + esc(t('station')) + '</h2>' + field('name','name') + '<label class="pr-field"><span>' + esc(t('topics')) + '</span><textarea name="topics" maxlength="2000" rows="3">' + esc(p.topics) + '</textarea></label>' + select('language','language',languages.map(l => [l, display ? display.of(l) : l])) + field('moderation_style','style') + select('moderation','moderation',[['off',t('off')],['little',t('little')],['balanced',t('balanced')],['much',t('much')]]) + '</section>' +
            '<section data-panel="1" hidden><h2>' + esc(t('music')) + '</h2>' + select('mode','source',[['local',t('local')],['generated',t('generated')],['mixed',t('mixed')]]) + '<label class="pr-field"><span>' + esc(t('genres')) + '</span><textarea name="genre_lines" rows="4">' + esc((p.genres || []).map(g => g.name + ':' + g.weight).join('\n')) + '</textarea><small>' + esc(t('genres_hint')) + '</small></label>' + field('mood','mood') + select('vocals','vocals',[['instrumental',t('instrumental')],['vocals',t('vocals')],['mixed',t('mixed')]]) + field('bpm','tempo','number',0,300) + field('generated_percent','mix','number',0,100) + '</section>' +
            '<section data-panel="2" hidden><h2>' + esc(t('news')) + '</h2>' + select('news_minutes','interval',[[0,t('off')],[30,'30 '+t('minutes')],[60,'60 '+t('minutes')]]) + '<p class="pr-hint">' + esc(t('news_hint')) + '</p>' + check('news_topics','topic_news') + check('international','international') + check('national','national') + check('regional','regional') + field('country','country') + field('region','region') + select('timezone','timezone',timezones.map(z => [z,z])) + '</section>' +
            '<section data-panel="3" hidden><h2>' + esc(t('operation')) + '</h2><p class="pr-hint">' + esc(t('reserve_hint')) + '</p><div class="pr-form-grid">' + field('reserve_minutes','reserve','number',5,180) + field('min_tracks','min_tracks','number',2,100) + field('repeat_minutes','repeat','number',0,1440) + field('repeat_tracks','repeat_tracks','number',0,200) + field('library_minutes','target','number',5,1440) + field('library_mb','storage','number',100,20000) + field('daily_generations','daily_music','number',0,200) + field('daily_editorial','daily_editorial','number',0,200) + field('daily_tts_chars','daily_tts','number',0,200000) + '</div>' + check('strict','strict') + '</section>' +
            '<div class="pr-settings-footer"><button type="button" data-action="cancel">' + esc(t('cancel')) + '</button><span></span><button type="button" data-action="back">' + esc(t('back')) + '</button><button type="button" data-action="next">' + esc(t('next')) + '</button><button class="pr-primary" type="submit">' + esc(t('save')) + '</button></div></form>';
        if(p.id) host.querySelector('.pr-settings-footer').insertAdjacentHTML('afterbegin','<button type="button" data-action="delete">'+esc(t('delete'))+'</button><button type="button" data-action="preview">'+esc(t('preview'))+'</button>');
        const form = host.querySelector('form');
        const show = () => {
            form.querySelectorAll('[data-panel]').forEach(el => { el.hidden = Number(el.dataset.panel) !== step; });
            form.querySelectorAll('[data-step]').forEach(el => { el.classList.toggle('is-active', Number(el.dataset.step) === step); el.setAttribute('aria-current', Number(el.dataset.step) === step ? 'step' : 'false'); });
            form.querySelector('[data-action="back"]').disabled = step === 0; form.querySelector('[data-action="next"]').hidden = step === 3;
        };
        form.addEventListener('click', e => {
            const button = e.target.closest('button'); if (!button) return;
            if (button.dataset.step) step = Number(button.dataset.step);
            if (button.dataset.action === 'next') step = Math.min(3, step + 1);
            if (button.dataset.action === 'back') step = Math.max(0, step - 1);
            if (button.dataset.action === 'cancel') return deps.cancel();
            show();
        });
        form.addEventListener('submit', async e => {
            e.preventDefault();
            const invalid=form.querySelector(':invalid');if(invalid){step=Number(invalid.closest('[data-panel]').dataset.panel);show();invalid.reportValidity();return;}
            for (const input of form.elements) {
                if (!input.name || input.name === 'genre_lines') continue;
                p[input.name] = input.type === 'checkbox' ? input.checked : input.type === 'number' ? Number(input.value) : input.value;
            }
            p.news_minutes = Number(p.news_minutes); p.country = p.country.trim().toUpperCase();
            p.genres = form.elements.genre_lines.value.split('\n').map(x => x.trim()).filter(Boolean).map(x => { const split = x.lastIndexOf(':'); return { name: split < 0 ? x : x.slice(0, split).trim(), weight: split < 0 ? 1 : Number(x.slice(split + 1)) }; });
            const button = form.querySelector('[type="submit"]'); button.disabled = true;
            try { await deps.save(p); } finally { button.disabled = false; }
        });
        show();
    }
    window.PersonalRadioSettings = { render };
})();
