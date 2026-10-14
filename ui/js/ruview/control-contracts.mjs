// Reviewed v3122 control contracts shared by the embedded recording/training UI.
const identifier = value => typeof value === 'string' && /^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$/.test(value);

export function controlResult(data, expected) {
    if (!data || typeof data !== 'object' || data.error || data.success === false || data.status === 'error') {
        throw new Error(typeof data?.error === 'string' ? data.error : data?.message || 'RuView rejected the operation');
    }
    if (expected === 'success' ? data.success !== true : data.status !== expected) {
        throw new Error('RuView did not confirm the operation');
    }
    return data;
}

export function trainingPayload(method, config, models = []) {
    const ids = config.selectedRecordings;
    if (!Array.isArray(ids) || !ids.length || !ids.every(identifier)) throw Error('Select at least one recording');
    if (!Number.isSafeInteger(config.epochs) || config.epochs < 1) throw Error('Epochs must be a positive integer');
    const dataset_ids = [...ids], epochs = config.epochs;
    if (method === 'startLoraTraining') {
        if (!identifier(config.base_model) || !models.some(model => model.id === config.base_model)) throw Error('Select a listed base model');
        if (!identifier(config.lora_profile_name)) throw Error('Enter a profile name using letters, numbers, dots, dashes or underscores');
        return {dataset_ids, base_model_id: config.base_model, profile_name: config.lora_profile_name, epochs, rank: 8};
    }
    if (!Number.isFinite(config.learning_rate) || config.learning_rate <= 0) throw Error('Learning rate must be positive');
    if (method === 'startPretraining') return {dataset_ids, epochs, lr: config.learning_rate};
    if (method !== 'startTraining') throw Error('Unsupported training operation');
    if (!Number.isSafeInteger(config.batch_size) || config.batch_size < 1 || !Number.isSafeInteger(config.patience) || config.patience < 1) throw Error('Batch size and patience must be positive integers');
    return {dataset_ids, config: {epochs, batch_size: config.batch_size, learning_rate: config.learning_rate, early_stopping_patience: config.patience}};
}

export function readError(error) {
    return error?.status === 404 || error?.status === 501 ? 'Not supported by this service' : 'Could not load data. Retry when the service is available.';
}

// Re-rendering progress must not discard the focused input or its selection.
export function preserveFocus(root) {
    const active = root.contains(document.activeElement) ? document.activeElement : null;
    const key = active?.dataset.ruviewFocus;
    const start = active?.selectionStart, end = active?.selectionEnd;
    return () => {
        if (!key) return;
        const next = Array.from(root.querySelectorAll('[data-ruview-focus]')).find(el => el.dataset.ruviewFocus === key);
        if (!next || next.disabled) return;
        next.focus({preventScroll: true});
        if (start != null && typeof next.setSelectionRange === 'function') next.setSelectionRange(start, end);
    };
}
