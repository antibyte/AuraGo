"""Dumps every step of one prefix block, so the browser port can be compared piece by
piece. Development helper.

    python scripts/woof-reference/dump_block.py "text" juno 0
"""
import os
import sys

import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, os.path.join(ROOT, "data", "aura-drift-woof"))

from underdog_woof.runtime import WoofNP, encode_text, _silu    # noqa: E402

OUT = os.path.join(HERE, "out")


def main():
    text = sys.argv[1] if len(sys.argv) > 1 else "Wave goodbye! That's my exhaust, not the horizon."
    voice = sys.argv[2] if len(sys.argv) > 2 else "juno"
    layer = int(sys.argv[3]) if len(sys.argv) > 3 else 0
    weights = os.path.join(ROOT, "ui", "3d", "aura-drift", "voice", "woof_int8.npz")
    os.makedirs(OUT, exist_ok=True)

    model = WoofNP(weights, voice=voice)
    s = model.speech
    H, Dh, d, h2 = s.H, s.Dh, s.d, s.Dh // 2

    ids = encode_text(text)
    x = np.concatenate([model.voice, s.text_emb[np.asarray(ids)], s.bos[None]], 0).astype(np.float32)
    n = x.shape[0]

    (wqkv, _), (wo, _), (w1, _), (w2, _) = s.layers[layer]
    xn = x / np.sqrt((x * x).mean(-1, keepdims=True) + np.float32(np.finfo(np.float32).eps))
    np.save(os.path.join(OUT, "blk.xn.npy"), xn)

    qkv = (xn @ wqkv.T).reshape(n, 3, H, Dh)
    np.save(os.path.join(OUT, "blk.qkv.npy"), qkv.reshape(n, 3 * H * Dh))

    qk = qkv[:, :2]
    qk = qk * s.cos[:n, None, None] + np.concatenate([qk[..., h2:], qk[..., :h2]], -1) * s.sin[:n, None, None]
    np.save(os.path.join(OUT, "blk.q.npy"), qk[:, 0].reshape(n, H * Dh))
    np.save(os.path.join(OUT, "blk.k.npy"), qk[:, 1].reshape(n, H * Dh))

    K, V = s.kv[layer, 0], s.kv[layer, 1]
    K[:, :n] = qk[:, 1].transpose(1, 0, 2)
    V[:, :n] = qkv[:, 2].transpose(1, 0, 2)
    np.save(os.path.join(OUT, "blk.K.npy"), K[:, :n].transpose(1, 0, 2).reshape(n, H * Dh))
    np.save(os.path.join(OUT, "blk.V.npy"), V[:, :n].transpose(1, 0, 2).reshape(n, H * Dh))

    q = qk[:, 0].transpose(1, 0, 2)
    att = np.matmul(q, K[:, :n].transpose(0, 2, 1)) * s.scale
    att[:, np.triu(np.ones((n, n), bool), 1)] = -np.inf
    att = np.exp(att - att.max(-1, keepdims=True))
    att /= att.sum(-1, keepdims=True)
    np.save(os.path.join(OUT, "blk.att.npy"), att.transpose(1, 0, 2).reshape(n, H * n))

    a = np.matmul(att, V[:, :n]).transpose(1, 0, 2).reshape(n, d)
    np.save(os.path.join(OUT, "blk.ctx.npy"), a)

    x = x + a @ wo.T
    np.save(os.path.join(OUT, "blk.afterwo.npy"), x)

    xn = x / np.sqrt((x * x).mean(-1, keepdims=True) + np.float32(np.finfo(np.float32).eps))
    gu = xn @ w1.T
    np.save(os.path.join(OUT, "blk.gu.npy"), gu)
    out = x + (_silu(gu[:, :s.ff]) * gu[:, s.ff:]) @ w2.T
    np.save(os.path.join(OUT, "blk.out.npy"), out)
    print("dumped block", layer, "n =", n)
    print("  q[0,0:4]", np.array2string(qk[0, 0, 0, :4], precision=6))
    print("  k[0,0:4]", np.array2string(qk[0, 1, 0, :4], precision=6))
    print("  att[0,0:4]", np.array2string(att[0, 0, :4], precision=6))


if __name__ == "__main__":
    main()
