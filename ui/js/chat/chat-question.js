(function () {
    let activeQuestionModal = null;

    function activeSessionId() {
        if (typeof getActiveSessionId === 'function') return getActiveSessionId();
        return (window.SessionDrawer && window.SessionDrawer.getActiveSessionId && window.SessionDrawer.getActiveSessionId()) || 'default';
    }

    function tr(key, fallback) {
        return typeof t === 'function' ? t(key) : fallback;
    }

    function closeQuestionModal() {
        if (!activeQuestionModal) return;
        activeQuestionModal.remove();
        activeQuestionModal = null;
    }

    async function submitQuestionResponse(sessionId, selectedValue, freeText) {
        await fetch('/api/agent/question-response', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                session_id: sessionId || activeSessionId(),
                selected_value: selectedValue || '',
                free_text: freeText || '',
            }),
        });
        closeQuestionModal();
    }

    function showQuestionModal(payload) {
        if (!payload || payload.session_id && payload.session_id !== activeSessionId()) return;
        closeQuestionModal();

        const sessionId = payload.session_id || activeSessionId();
        const timeoutSeconds = Math.max(1, Number(payload.timeout_seconds || 120));
        const startedAt = Date.now();
        const overlay = document.createElement('div');
        overlay.className = 'question-modal-overlay';
        overlay.setAttribute('role', 'dialog');
        overlay.setAttribute('aria-modal', 'true');

        const panel = document.createElement('div');
        panel.className = 'question-modal-panel';

        const status = document.createElement('div');
        status.className = 'question-modal-status';
        status.textContent = tr('chat.question_waiting', 'The agent is waiting for your answer...');

        const title = document.createElement('h2');
        title.className = 'question-modal-title';
        title.textContent = payload.question || '';

        const selectLabel = document.createElement('div');
        selectLabel.className = 'question-modal-select-label';
        selectLabel.textContent = tr('chat.question_select', 'Select an option');

        const optionsWrap = document.createElement('div');
        optionsWrap.className = 'question-modal-options';
        (payload.options || []).forEach((opt) => {
            const btn = document.createElement('button');
            btn.type = 'button';
            btn.className = 'question-modal-option';
            const label = document.createElement('span');
            label.className = 'question-modal-option-label';
            label.textContent = opt.label || opt.value || '';
            btn.appendChild(label);
            if (opt.description) {
                const desc = document.createElement('span');
                desc.className = 'question-modal-option-desc';
                desc.textContent = opt.description;
                btn.appendChild(desc);
            }
            btn.addEventListener('click', () => submitQuestionResponse(sessionId, opt.value || opt.label || '', ''));
            optionsWrap.appendChild(btn);
        });

        const timerLabel = document.createElement('div');
        timerLabel.className = 'question-modal-timer-label';
        timerLabel.textContent = tr('chat.question_timer_label', 'Time remaining');
        const timer = document.createElement('div');
        timer.className = 'question-modal-timer';
        const timerFill = document.createElement('div');
        timerFill.className = 'question-modal-timer-fill';
        timer.appendChild(timerFill);

        panel.append(status, title, selectLabel, optionsWrap);

        if (payload.allow_free_text) {
            const freeTextForm = document.createElement('form');
            freeTextForm.className = 'question-modal-free-text';
            const input = document.createElement('input');
            input.type = 'text';
            input.autocomplete = 'off';
            input.placeholder = tr('chat.question_free_text_placeholder', 'Type a custom answer...');
            const send = document.createElement('button');
            send.type = 'submit';
            send.textContent = tr('chat.btn_send', 'Send');
            freeTextForm.append(input, send);
            freeTextForm.addEventListener('submit', (event) => {
                event.preventDefault();
                const value = input.value.trim();
                if (value) submitQuestionResponse(sessionId, '', value);
            });
            panel.appendChild(freeTextForm);
            setTimeout(() => input.focus(), 50);
        }

        panel.append(timerLabel, timer);
        overlay.appendChild(panel);
        document.body.appendChild(overlay);
        activeQuestionModal = overlay;

        const tick = window.setInterval(() => {
            if (!activeQuestionModal) {
                window.clearInterval(tick);
                return;
            }
            const elapsed = (Date.now() - startedAt) / 1000;
            const remainingRatio = Math.max(0, 1 - elapsed / timeoutSeconds);
            timerFill.style.transform = 'scaleX(' + remainingRatio + ')';
            if (remainingRatio <= 0) {
                window.clearInterval(tick);
                status.textContent = tr('chat.question_timeout', 'The question timed out.');
                setTimeout(closeQuestionModal, 900);
            }
        }, 250);
    }

    async function checkPendingQuestion() {
        try {
            const sessionId = activeSessionId();
            const res = await fetch('/api/agent/question-status?session=' + encodeURIComponent(sessionId));
            if (!res.ok) return;
            const data = await res.json();
            if (data && data.status === 'pending' && data.question) {
                showQuestionModal(data.question);
            }
        } catch (e) { }
    }

    window.showQuestionModal = showQuestionModal;
    window.checkPendingQuestion = checkPendingQuestion;
})();
