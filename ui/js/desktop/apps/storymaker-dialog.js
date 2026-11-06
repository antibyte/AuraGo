// Storymaker app export dialog: a native <dialog> with the options of one export: quality, captions burned into the picture, SRT,
// VTT, stems and Open in Video Studio. Burned-in captions start from style.captions. Open in Video Studio forces the stems on and
// puts the new job's id into inst.handoffWish, a session-only Set. The dialog starts nothing itself: it hands the options to
// inst.startExport, which S.createExport sets. Classic script, publishes S.createExportDialog.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};

    // The form a story starts from. A story without the field counts as captions on, the server's default.
    function presetValues(story) {
        const style = story && story.style;
        return { quality: 'standard', burnIn: !(style && style.captions === false), srt: false, vtt: false, stems: false, handoff: false };
    }

    // The POST /render body for the form values.
    function renderOptions(values) {
        return {
            burn_in_captions: values.burnIn,
            srt: values.srt,
            vtt: values.vtt,
            stems: values.stems || values.handoff,
            quality: values.quality === 'high' ? 'high' : 'standard'
        };
    }

    function createExportDialog(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const node = el('dialog', { class: 'stm-export-dialog', ariaLabel: t('storymaker.export.dialog_title'), dataset: { stm: 'export-dialog' } });
        let disposed = false;

        function checkbox(name, labelKey, checked, onChange) {
            const input = el('input', { type: 'checkbox', dataset: { stm: 'export-opt-' + name }, checked: checked || null, onChange: onChange || null });
            return { input, row: el('label', { class: 'stm-export-check' }, input, el('span', { text: t(labelKey) })) };
        }

        function close() {
            if (node.open) node.close();
        }

        async function submit(values) {
            close();
            const job = await inst.startExport(renderOptions(values));
            if (values.handoff && job && job.id) inst.handoffWish.add(job.id);
        }

        function build(values) {
            const quality = el('select', { class: 'stm-select', id: inst.domId('export-quality'), dataset: { stm: 'export-opt-quality' } },
                el('option', { value: 'standard', selected: values.quality === 'standard' || null, text: t('storymaker.export.quality_standard') }),
                el('option', { value: 'high', selected: values.quality === 'high' || null, text: t('storymaker.export.quality_high') }));
            const burnIn = checkbox('burn', 'storymaker.export.burn_in', values.burnIn);
            const srt = checkbox('srt', 'storymaker.export.label_srt', values.srt);
            const vtt = checkbox('vtt', 'storymaker.export.label_vtt', values.vtt);
            const stems = checkbox('stems', 'storymaker.export.stems', values.stems);
            const note = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'export-stems-forced' }, text: t('storymaker.export.stems_forced') });
            let stemsWanted = values.stems;
            const unavailable = el('p', { class: 'stm-hint', hidden: true, dataset: { stm: 'export-opt-handoff-reason' }, text: t('storymaker.handoff.unavailable') });
            const syncStems = () => {
                const forced = handoff.input.checked;
                if (forced && !stems.input.disabled) stemsWanted = stems.input.checked;
                stems.input.checked = forced || stemsWanted;
                stems.input.disabled = forced;
                note.hidden = !forced;
            };
            const handoff = checkbox('handoff', 'storymaker.export.open_in_studio', values.handoff, syncStems);
            // Asked each time the form opens, so nobody waits for a whole render to learn that Video Studio cannot take the film. A failed
            // read counts as unavailable. The option stays usable until the answer, because the first status read tests the server's
            // encoders and can take seconds; a tick made meanwhile goes back with its forced stems. checkHandoff in storymaker-export.js
            // checks again when the render ends, for a status that changed.
            S.handoff.status().then(S.handoff.studioReady, () => false).then(ready => {
                if (ready) return;
                handoff.input.disabled = true;
                unavailable.hidden = false;
                if (handoff.input.checked) { handoff.input.checked = false; syncStems(); }
            });
            const read = () => ({ quality: quality.value, burnIn: burnIn.input.checked, srt: srt.input.checked, vtt: vtt.input.checked, stems: stems.input.checked, handoff: handoff.input.checked });
            S.clear(node);
            node.appendChild(el('h3', { text: t('storymaker.export.dialog_title') }));
            node.appendChild(el('div', { class: 'stm-field' }, el('label', { class: 'stm-label', for: inst.domId('export-quality'), text: t('storymaker.export.quality') }), quality));
            node.appendChild(burnIn.row);
            node.appendChild(el('div', { class: 'stm-field' }, el('span', { class: 'stm-label', text: t('storymaker.export.subtitles') }), srt.row, vtt.row));
            node.appendChild(stems.row);
            node.appendChild(handoff.row);
            node.appendChild(note);
            node.appendChild(unavailable);
            node.appendChild(el('div', { class: 'stm-actions' },
                el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'export-dialog-cancel' }, onClick: () => close() }, el('span', { text: t('storymaker.export.dialog_cancel') })),
                el('button', { type: 'button', class: 'stm-btn stm-btn-primary', dataset: { stm: 'export-confirm' }, onClick: () => submit(read()) }, icon('film'), el('span', { text: t('storymaker.export.start') }))));
        }

        return {
            el: node,
            open() {
                if (disposed || node.open) return;
                build(presetValues(inst.story));
                node.showModal();
            },
            close,
            dispose() {
                disposed = true;
                close();
                node.remove();
            }
        };
    }

    S.createExportDialog = createExportDialog;
})();
