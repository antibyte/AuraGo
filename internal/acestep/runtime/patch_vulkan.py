"""Preload the pinned worker without modifying native generation or stop tokens."""
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


if __name__ == '__main__':
    patch(sys.argv[1])
