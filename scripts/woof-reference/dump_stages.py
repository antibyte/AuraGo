"""Dumps the intermediate values of the speech model, so the browser port can be
checked stage by stage. Development helper.

    python scripts/woof-reference/dump_stages.py "text" juno 1
"""
import os
import sys

import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, os.path.join(ROOT, "data", "aura-drift-woof"))

from underdog_woof.runtime import RNG, WoofNP, encode_text     # noqa: E402


def main():
    text = sys.argv[1] if len(sys.argv) > 1 else "Wave goodbye! That's my exhaust, not the horizon."
    voice = sys.argv[2] if len(sys.argv) > 2 else "juno"
    seed = int(sys.argv[3]) if len(sys.argv) > 3 else 1
    weights = os.path.join(ROOT, "ui", "3d", "aura-drift", "voice", "woof_int8.npz")
    out = os.path.join(HERE, "out")
    os.makedirs(out, exist_ok=True)

    model = WoofNP(weights, voice=voice)
    speech = model.speech
    RNG.seed(seed)

    ids = encode_text(text)
    x = np.concatenate([model.voice, speech.text_emb[np.asarray(ids)], speech.bos[None]], 0).astype(np.float32)
    np.save(os.path.join(out, "stage.prompt.npy"), x)
    print("prompt", x.shape)

    p0 = x.shape[0]
    for i in range(speech.L):
        x = speech._prefix_block(i, x, p0)
        np.save(os.path.join(out, "stage.layer%d.npy" % i), x)
    np.save(os.path.join(out, "stage.afterprefix.npy"), x)
    print("after prefix blocks", x.shape)

    last = np.ascontiguousarray(x[-1])
    np.save(os.path.join(out, "stage.last.npy"), last)
    s, e = speech._cond(last)
    np.save(os.path.join(out, "stage.cond_s.npy"), s)
    print("cond e = %.6f  s[0:4] = %s" % (e, np.array2string(s[:4], precision=6)))

    noise = RNG.randn(speech.latent) * np.float32(0.5)
    z = speech._head(s, noise)
    np.save(os.path.join(out, "stage.z0.npy"), z)
    print("z0[0:4] = %s" % np.array2string(z[:4], precision=6))

    # the state after one autoregressive step
    win, wb = speech.lat_in
    step = win @ z + wb
    for i in range(speech.L):
        step = speech._block1(i, step, p0)
    s2, e2 = speech._cond(step)
    np.save(os.path.join(out, "stage.step.npy"), step)
    np.save(os.path.join(out, "stage.cond_s2.npy"), s2)
    print("after one step: cond e = %.6f" % e2)


if __name__ == "__main__":
    main()
