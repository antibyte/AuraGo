"""Preload the pinned worker and honor explicit duration during LM code generation."""
from pathlib import Path
import sys


def patch(root):
    source = Path(root) / 'tools/ace-server.cpp'
    text = source.read_text()
    anchor = '    g_store = store_create(g_keep_loaded ? EVICT_NEVER : EVICT_STRICT);'
    assert text.count(anchor) == 1, 'native startup changed'
    preload = r'''
    // AuraGo: this image owns a single selected model set. Load it before health.
    if (!g_keep_loaded || g_registry.dit.size() != 1 || g_registry.text_enc.size() != 1 ||
        g_registry.vae.size() != 1 || g_registry.lm.size() > 1) return 1;
    ModelKey key{};
    key.adapter_scale = 1.0f;
    key.path = g_registry.dit[0].path;
    key.kind = MODEL_DIT;
    auto dit = store_require_dit(g_store, key);
    if (!dit) return 1;
    store_release(g_store, dit);
    key.kind = MODEL_COND_ENC;
    auto cond = store_require_cond_enc(g_store, key);
    if (!cond) return 1;
    store_release(g_store, cond);
    key.kind = MODEL_TEXT_ENC;
    key.path = g_registry.text_enc[0].path;
    auto enc = store_require_text_enc(g_store, key);
    if (!enc) return 1;
    store_release(g_store, enc);
    key.kind = MODEL_VAE_DEC;
    key.path = g_registry.vae[0].path;
    auto vae = store_require_vae_dec(g_store, key);
    if (!vae) return 1;
    store_release(g_store, vae);
    if (!g_registry.lm.empty()) {
        key.kind = MODEL_LM;
        key.path = g_registry.lm[0].path;
        key.max_seq = g_lm_params.max_seq;
        key.n_kv_sets = 2 * g_max_batch;
        auto lm = store_require_lm(g_store, key);
        if (!lm) return 1;
        store_release(g_store, lm);
    }
'''
    source.write_text(text.replace(anchor, anchor + preload))
    source = Path(root) / 'src/pipeline-lm.cpp'
    text = source.read_text()
    replacements = {
        '        int tok            = sample_top_k_p(lg.data(), V, temperature, top_p, top_k, seqs[i].rng);':
        '''        // AuraGo: explicit duration owns the 5Hz code count, not an early EOS.
        if (aces[i].duration > 0) lg[TOKEN_IM_END] = -1e9f;
        int tok            = sample_top_k_p(lg.data(), V, temperature, top_p, top_k, seqs[i].rng);''',
        '            compact_logits[0] = lc[eos_idx];':
        '''            compact_logits[0] = aces[orig_i].duration > 0 ? -1e9f : lc[eos_idx];''',
        '                seqs[orig_i].audio_codes.push_back(tok - AUDIO_CODE_BASE);':
        '''                seqs[orig_i].audio_codes.push_back(tok - AUDIO_CODE_BASE);
                if (aces[orig_i].duration > 0 &&
                    seqs[orig_i].audio_codes.size() >= (size_t) ceil(aces[orig_i].duration * 5))
                    seqs[orig_i].done = true;''',
    }
    for anchor, replacement in replacements.items():
        assert text.count(anchor) == 1, 'native LM duration path changed'
        text = text.replace(anchor, replacement)
    source.write_text(text)


if __name__ == '__main__':
    patch(sys.argv[1])
