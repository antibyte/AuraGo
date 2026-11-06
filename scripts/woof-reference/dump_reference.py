"""Dumps the reference latents and audio of one utterance for the port comparison.

Single take, no retry logic: the port does the same, so the two can be compared
sample by sample. Writes <out>/<name>.latents.npy and <out>/<name>.audio.npy.

    python scripts/woof-reference/dump_reference.py "text" juno 1 probe
"""
import os
import sys

import numpy as np

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, os.path.join(ROOT, "data", "aura-drift-woof"))

from underdog_woof.runtime import RNG, WoofNP      # noqa: E402

OUT = os.path.join(HERE, "out")


def main():
    text = sys.argv[1] if len(sys.argv) > 1 else "Wave goodbye! That's my exhaust, not the horizon."
    voice = sys.argv[2] if len(sys.argv) > 2 else "juno"
    seed = int(sys.argv[3]) if len(sys.argv) > 3 else 1
    name = sys.argv[4] if len(sys.argv) > 4 else "probe"
    weights = os.path.join(ROOT, "ui", "3d", "aura-drift", "voice", "woof_int8.npz")

    os.makedirs(OUT, exist_ok=True)
    model = WoofNP(weights, voice=voice)
    RNG.seed(seed)
    lat = model.latents(text)
    wav = model.dec(lat * model.std + model.mean)

    np.save(os.path.join(OUT, name + ".latents.npy"), lat)
    np.save(os.path.join(OUT, name + ".audio.npy"), wav)
    lat.astype(np.float32).tofile(os.path.join(OUT, name + ".latents.npy.bin"))
    wav.astype(np.float32).tofile(os.path.join(OUT, name + ".audio.npy.bin"))
    print("frames %d  samples %d  peak %.4f  rms %.6f" % (
        lat.shape[0], wav.shape[0], float(np.abs(wav).max()),
        float(np.sqrt((wav.astype(np.float64) ** 2).mean()))))


if __name__ == "__main__":
    main()
