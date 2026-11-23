#!/usr/bin/env python3
"""AuraGo Chibi Sensei trainer, job.json schema v1.

One fine-tuning run in stages (see STAGES). Every stage reports through "@@chibi {json}" lines on
stdout; chibi-status.json and chibi-manifest.json are written in a finally block. Only the standard
library is imported at module level; torch, transformers, peft, huggingface_hub and gguf are imported
inside the stages, so --selftest and the dry run work without them.

  chibi_train.py --job job.json --out /kaggle/working [--input /kaggle/input]
  chibi_train.py --selftest
  CHIBI_FAKE_ENV=1 chibi_train.py --dry-run --job job.json --out DIR --input DIR
"""
import argparse
import gc
import hashlib
import importlib.metadata
import importlib.util
import io
import json
import math
import os
import random
import re
import shutil
import signal
import struct
import subprocess
import sys
import tarfile
import tempfile
import time
import traceback
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone

LOG_PREFIX = "@@chibi"
MAX_LINE_BYTES = 4096
DEFAULT_INPUT = "/kaggle/input"
WORK_ROOT = "/tmp/chibi/work"
SESSION_LEAD_S = 90          # kernel start and notebook cell before this script began
SAVE_GGUF_RESERVE_S = 420    # merge, save, tuned conversion, manifest
BASE_GGUF_MIN_S = 360        # contract: skip the base GGUF below six spare minutes
TAIL_LINES = 40
MARKER_DEPTH = 4
HEAVY_MODULES = ("torch", "transformers", "peft", "huggingface_hub", "gguf", "safetensors")
SECRET_ENV = ("HF_TOKEN", "HUGGING_FACE_HUB_TOKEN", "HUGGINGFACE_HUB_TOKEN", "HF_API_TOKEN")
NON_TEXT_MARKERS = ("vision", "visual", "audio", "mm_projector", "multi_modal_projector")
TOKENIZER_FILES = ("tokenizer.json", "tokenizer_config.json", "tokenizer.model", "special_tokens_map.json",
                   "vocab.json", "merges.txt", "added_tokens.json", "chat_template.jinja", "chat_template.json")
SNAPSHOT_IGNORE = ["*.gguf", "*.onnx", "*.msgpack", "*.h5", "*.ot", "*.pth", "original/*", "onnx/*"]
MANIFEST_NAME = "chibi-manifest.json"
MANIFEST_KINDS = ("gguf_tuned", "gguf_base", "adapter", "adapter_config", "metrics", "eval_samples", "status")


def output_names(outtype):
    return {"gguf_tuned": "chibi-model-%s.gguf" % outtype, "gguf_base": "chibi-base-%s.gguf" % outtype,
            "adapter": "chibi-adapter-model.safetensors", "adapter_config": "chibi-adapter-config.json",
            "metrics": "chibi-metrics.jsonl", "eval_samples": "chibi-eval_samples.jsonl",
            "status": "chibi-status.json"}


def utc_now():
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def finite(x):
    try:
        f = float(x)
    except (TypeError, ValueError):
        return None
    return f if math.isfinite(f) else None


def rnd(x, n=4):
    f = finite(x)
    return None if f is None else round(f, n)


def sanitize(obj):
    """JSON-safe copy: non-finite floats become None, unknown objects become strings."""
    if isinstance(obj, float):
        return obj if math.isfinite(obj) else None
    if obj is None or isinstance(obj, (str, int, bool)):
        return obj
    if isinstance(obj, dict):
        return {str(k): sanitize(v) for k, v in obj.items()}
    if isinstance(obj, (list, tuple)):
        return [sanitize(v) for v in obj]
    return str(obj)


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def read_json(path):
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def _tmp_path(path):
    return os.path.join(os.path.dirname(path) or ".", ".tmp-" + os.path.basename(path))


def write_json_atomic(path, obj):
    tmp = _tmp_path(path)
    with open(tmp, "w", encoding="utf-8") as f:
        json.dump(sanitize(obj), f, ensure_ascii=False, indent=1, allow_nan=False)
        f.write("\n")
    os.replace(tmp, path)


def write_jsonl_atomic(path, rows):
    tmp = _tmp_path(path)
    with open(tmp, "w", encoding="utf-8") as f:
        for row in rows:
            f.write(json.dumps(sanitize(row), ensure_ascii=False, separators=(",", ":"), allow_nan=False) + "\n")
    os.replace(tmp, path)


def one_line(text, limit=700):
    text = " ".join(str(text).split())
    return text if len(text) <= limit else text[: limit - 3] + "..."


def tail_lines(text, n=TAIL_LINES):
    return [line.rstrip() for line in str(text).splitlines() if line.strip()][-n:]


def last_line(text):
    lines = tail_lines(text, 1)
    return one_line(lines[0], 300) if lines else ""


def safe_name(name):
    return (isinstance(name, str) and 0 < len(name) <= 200 and "/" not in name and "\\" not in name
            and not name.startswith(".") and "\x00" not in name)


def dist_version(name):
    try:
        return importlib.metadata.version(name)
    except importlib.metadata.PackageNotFoundError:
        return ""


def heavy_loaded():
    return sorted(m for m in HEAVY_MODULES if m in sys.modules)


def dir_bytes(path):
    total = 0
    for dirpath, _, files in os.walk(path):
        for name in files:
            total += os.path.getsize(os.path.join(dirpath, name))
    return total


def encode_line(prefix, obj, limit=MAX_LINE_BYTES):
    """One ASCII line of at most limit bytes; the largest fields are halved until it fits."""
    obj = sanitize(obj)
    for _ in range(32):
        line = prefix + " " + json.dumps(obj, ensure_ascii=True, separators=(",", ":"))
        if len(line) <= limit:
            return line
        key, size = None, 0
        for k, v in obj.items():
            if k in ("v", "seq", "ts", "t", "cut"):
                continue
            s = len(json.dumps(v, ensure_ascii=True))
            if s > size:
                key, size = k, s
        if key is None or size <= 24:
            break
        value = obj[key]
        text = value if isinstance(value, str) else json.dumps(value, ensure_ascii=True)
        obj = dict(obj)
        obj[key] = text[: max(8, len(text) // 2)] + "..."
        obj["cut"] = True
    keep = {k: obj[k] for k in ("v", "seq", "ts", "t", "stage", "state", "code", "kind") if k in obj}
    keep["cut"] = True
    return prefix + " " + json.dumps(keep, ensure_ascii=True, separators=(",", ":"))


class Log:
    """'@@chibi {json}' lines flushed one by one; metric objects also go to chibi-metrics.jsonl."""

    def __init__(self, stream=None, prefix=LOG_PREFIX):
        self.stream = stream or sys.stdout
        self.prefix = prefix
        self.seq = 0
        self.metrics_path = None

    def emit(self, t, **fields):
        self.seq += 1
        obj = {"v": 1, "seq": self.seq, "ts": utc_now(), "t": t}
        obj.update(fields)
        try:
            self.stream.write(encode_line(self.prefix, obj) + "\n")
            self.stream.flush()
        except (OSError, ValueError):
            pass
        return obj

    def info(self, kind, **fields):
        return self.emit("info", kind=kind, **fields)

    def metric(self, **fields):
        obj = self.emit("metric", **fields)
        if self.metrics_path:
            with open(self.metrics_path, "a", encoding="utf-8") as f:
                f.write(json.dumps(sanitize(obj), ensure_ascii=False, separators=(",", ":")) + "\n")
        return obj

    def raw(self, title, lines):
        """Unprefixed text for people reading the Kaggle log; the parser ignores it."""
        try:
            self.stream.write("----- %s -----\n%s\n-----\n" % (title, "\n".join(lines)))
            self.stream.flush()
        except (OSError, ValueError):
            pass


class StageError(Exception):
    """A failure with a run failure code (spec 'Error handling' plus the contract additions)."""

    def __init__(self, code, message):
        super().__init__(message)
        self.code = code
        self.message = one_line(message)


class RunFailed(Exception):
    pass


class Interrupted(BaseException):
    pass


def is_oom(exc):
    for _ in range(6):
        if exc is None:
            return False
        if isinstance(exc, MemoryError) or type(exc).__name__ == "OutOfMemoryError":
            return True
        if "out of memory" in str(exc).lower():
            return True
        exc = exc.__cause__ or exc.__context__
    return False


def exc_summary(exc):
    return one_line("%s: %s" % (type(exc).__name__, exc), 700)


def trace_tail(exc, frames=6):
    tb = traceback.extract_tb(exc.__traceback__)[-frames:]
    return ["%s:%d %s" % (os.path.basename(f.filename), f.lineno, f.name) for f in tb]


def harden_stdio():
    for stream in (sys.stdout, sys.stderr):
        try:
            stream.reconfigure(errors="backslashreplace")
        except (AttributeError, ValueError):
            pass


def scrub_environment():
    """Remove tokens and Kaggle variables (never printed). Returns (count, kernel run type)."""
    run_type = os.environ.get("KAGGLE_KERNEL_RUN_TYPE", "")
    removed = 0
    for key in list(os.environ):
        if key in SECRET_ENV or key.startswith("KAGGLE_"):
            del os.environ[key]
            removed += 1
    os.environ.update({"HF_HUB_DISABLE_TELEMETRY": "1", "HF_HUB_DISABLE_PROGRESS_BARS": "1",
                       "TOKENIZERS_PARALLELISM": "false", "CUDA_VISIBLE_DEVICES": "0",
                       "PYTORCH_CUDA_ALLOC_CONF": "expandable_segments:True",
                       "PIP_DISABLE_PIP_VERSION_CHECK": "1", "PIP_NO_INPUT": "1",
                       "PIP_ROOT_USER_ACTION": "ignore", "PYTHONUNBUFFERED": "1"})
    return removed, run_type


def install_signal_handlers():
    def handler(signum, frame):
        raise Interrupted("signal %d" % signum)
    for name in ("SIGTERM", "SIGINT"):
        sig = getattr(signal, name, None)
        if sig is not None:
            try:
                signal.signal(sig, handler)
            except (ValueError, OSError):
                pass


class Budget:
    """One monotonic clock from script start; the Kaggle session clock started SESSION_LEAD_S earlier."""

    def __init__(self, session_s, max_train_s, clock=time.monotonic, t0=None):
        self.clock = clock
        self.t0 = clock() if t0 is None else t0
        self.session_s = max(0, int(session_s))
        self.max_train_s = max(0, int(max_train_s))

    def elapsed(self):
        return self.clock() - self.t0

    def remaining(self):
        return self.session_s - SESSION_LEAD_S - self.elapsed()

    def train_deadline(self, post_reserve_s):
        """Monotonic time at which training stops: max_train_s from now, or earlier for the session."""
        by_budget = self.clock() + self.max_train_s
        by_session = self.t0 + self.session_s - SESSION_LEAD_S - post_reserve_s
        return min(by_budget, by_session)


def generation_plan(n, max_new, gen_tps, available_s):
    """(per-sample cap, total cap) in seconds for n greedy answers."""
    gen_tps = max(1.0, float(gen_tps or 1.0))
    per = max(15.0, 3.0 * max_new / gen_tps)
    want = 1.5 * n * max_new / gen_tps + 60.0
    return per, max(0.0, min(want, available_s))


def lr_factor(step, warmup, total):
    """Linear warmup, then cosine decay to zero; step is 0-based."""
    if warmup > 0 and step < warmup:
        return (step + 1) / float(warmup)
    progress = (step - warmup) / float(max(1, total - warmup))
    return 0.5 * (1.0 + math.cos(math.pi * min(1.0, max(0.0, progress))))


JOB_FIELDS = (
    ("v", int), ("run_id", str), ("profile_sha", str), ("recipe", str), ("method", str),
    ("data.marker", str), ("data.format", str),
    ("hyper.epochs", int), ("hyper.learning_rate", float), ("hyper.per_device_batch", int),
    ("hyper.grad_accum", int), ("hyper.max_seq_len", int), ("hyper.seed", int),
    ("budget.session_timeout_s", int), ("budget.max_train_s", int), ("budget.eval_max_samples", int),
    ("budget.max_new_tokens", int), ("outputs.gguf_outtype", str), ("outputs.emit_base_gguf", bool),
    ("deps.llama_cpp.tag", str), ("deps.llama_cpp.url", str),
    ("profile.id", str), ("profile.base.repo", str), ("profile.base.revision", str),
    ("profile.base.auto_class", str), ("profile.chat.system_role", str), ("profile.gguf.expected_arch", str),
    ("profile.training.min_gpu_capability", str), ("profile.training.min_vram_gb", float),
    ("profile.training.t4_gen_tokens_per_second", float), ("profile.arch.params_b", float),
)


def jget(obj, path, default=None):
    cur = obj
    for part in path.split("."):
        if not isinstance(cur, dict) or part not in cur:
            return default
        cur = cur[part]
    return cur


def type_ok(value, kind):
    if kind is bool:
        return isinstance(value, bool)
    if isinstance(value, bool):
        return False
    if kind is float:
        return isinstance(value, (int, float)) and math.isfinite(value)
    return isinstance(value, kind)


def str_list(value):
    return isinstance(value, list) and all(isinstance(x, str) for x in value)


def validate_job(job):
    """Problems of a job.json (schema v1) as 'path: reason' strings; empty when valid."""
    if not isinstance(job, dict):
        return ["job: must be a JSON object"]
    problems = []
    for path, kind in JOB_FIELDS:
        if not type_ok(jget(job, path), kind):
            problems.append("%s: missing or not %s" % (path, kind.__name__))
    if problems:
        return problems
    h, b = job["hyper"], job["budget"]
    if job["v"] != 1:
        problems.append("v: must be 1")
    if job["method"] not in ("lora", "full"):
        problems.append("method: must be lora or full")
    if job["recipe"] not in ("assistant", "specialist", "free"):
        problems.append("recipe: must be assistant, specialist or free")
    fmt = job["data"]["format"]
    if fmt not in ("messages", "text"):
        problems.append("data.format: must be messages or text")
    elif (job["recipe"] == "free") != (fmt == "text"):
        problems.append("data.format: the free recipe uses text rows, the others messages")
    if not safe_name(job["data"]["marker"]):
        problems.append("data.marker: must be a plain file name")
    for key in ("epochs", "per_device_batch", "grad_accum"):
        if h[key] < 1:
            problems.append("hyper.%s: must be at least 1" % key)
    if not 16 <= h["max_seq_len"] <= 8192:
        problems.append("hyper.max_seq_len: must be 16 to 8192")
    if h["learning_rate"] <= 0:
        problems.append("hyper.learning_rate: must be above 0")
    if job["method"] == "lora":
        lora = h.get("lora")
        if (not isinstance(lora, dict) or not type_ok(lora.get("r"), int) or lora["r"] < 1
                or not type_ok(lora.get("alpha"), float) or not type_ok(lora.get("dropout", 0.0), float)):
            problems.append("hyper.lora: r, alpha and dropout are required for method lora")
        else:
            targets = lora.get("target_modules") or jget(job, "profile.training.lora.target_modules") or []
            if not str_list(targets) or not targets:
                problems.append("hyper.lora.target_modules: needs at least one name")
    if b["max_train_s"] < 1 or b["session_timeout_s"] <= b["max_train_s"]:
        problems.append("budget: max_train_s must be positive and below session_timeout_s")
    if b["eval_max_samples"] < 0 or b["max_new_tokens"] < 1:
        problems.append("budget: eval_max_samples >= 0 and max_new_tokens >= 1 required")
    if not re.fullmatch(r"[a-z0-9_]{1,16}", job["outputs"]["gguf_outtype"]):
        problems.append("outputs.gguf_outtype: invalid")
    for path in ("deps.pip", "profile.chat.stop", "profile.training.extra_pip", "profile.base.auto_class_fallbacks"):
        if not str_list(jget(job, path) or []):
            problems.append("%s: must be a list of strings" % path)
    for path in ("deps.transformers", "deps.llama_cpp.sha256"):
        if not isinstance(jget(job, path) or "", str):
            problems.append("%s: must be a string" % path)
    return problems


def find_files(root, names, max_depth=MARKER_DEPTH):
    """Breadth-first, depth-limited search; returns ({name: path}, directories searched)."""
    found, searched, queue = {}, 0, [(root, 0)]
    while queue:
        directory, depth = queue.pop(0)
        try:
            with os.scandir(directory) as it:
                entries = sorted(it, key=lambda e: e.name)
        except OSError:
            continue
        searched += 1
        for e in entries:
            try:
                if e.is_file() and e.name in names and e.name not in found:
                    found[e.name] = e.path
                elif e.is_dir() and depth < max_depth:
                    queue.append((e.path, depth + 1))
            except OSError:
                continue
    return found, searched


def list_tree(root, max_depth=2, limit=40):
    out = []
    for dirpath, dirnames, filenames in os.walk(root):
        if dirpath[len(root):].count(os.sep) >= max_depth:
            dirnames[:] = []
        for name in sorted(dirnames + filenames):
            out.append(os.path.relpath(os.path.join(dirpath, name), root))
            if len(out) >= limit:
                return out
    return out


def load_marker(root, name, run_id):
    found, searched = find_files(root, {name})
    if name not in found:
        raise StageError("data_mismatch", "marker %s not found under %s (%d directories searched; is the "
                                          "dataset attached to the notebook?)" % (name, root, searched))
    try:
        marker = read_json(found[name])
    except (OSError, ValueError) as e:
        raise StageError("data_mismatch", "marker cannot be read: %s" % exc_summary(e))
    if not isinstance(marker, dict) or marker.get("v") != 1 or not isinstance(marker.get("files"), dict):
        raise StageError("data_mismatch", "marker has no v=1 files object")
    if marker.get("run_id") != run_id:
        raise StageError("data_mismatch", "marker belongs to run %s, not %s" % (marker.get("run_id"), run_id))
    return marker, os.path.dirname(found[name])


def verify_split_files(marker, base_dir, root):
    files = {}
    for split in ("train", "val", "eval"):
        entry = marker["files"].get(split)
        if entry is None:
            continue
        name = entry.get("name") if isinstance(entry, dict) else None
        if not safe_name(name) or not isinstance(entry.get("sha256"), str):
            raise StageError("data_mismatch", "marker entry %s is malformed" % split)
        path = os.path.join(base_dir, name)
        if not os.path.isfile(path):
            path = find_files(root, {name})[0].get(name)
        if not path:
            raise StageError("data_mismatch", "%s file %s is missing" % (split, name))
        got = sha256_file(path)
        if got != entry["sha256"].lower():
            raise StageError("data_mismatch", "%s file %s has sha256 %s, the marker says %s"
                             % (split, name, got, entry["sha256"]))
        files[split] = (path, entry)
    if "train" not in files:
        raise StageError("data_mismatch", "the marker lists no train file")
    return files


def row_problem(obj, fmt):
    if not isinstance(obj, dict):
        return "row is not an object"
    if fmt == "text":
        text = obj.get("text")
        return None if isinstance(text, str) and text.strip() else "text row without text"
    msgs = obj.get("messages")
    if not isinstance(msgs, list) or len(msgs) < 2:
        return "needs at least two messages"
    for m in msgs:
        if (not isinstance(m, dict) or m.get("role") not in ("system", "user", "assistant")
                or not isinstance(m.get("content"), str)):
            return "every message needs role system|user|assistant and string content"
    if msgs[-1]["role"] != "assistant":
        return "the last message must be from the assistant"
    if not any(m["role"] == "user" for m in msgs[:-1]):
        return "no user message before the answer"
    return None


def parse_rows(path, fmt, expect_rows, split):
    rows = []
    with open(path, "r", encoding="utf-8") as f:
        for n, line in enumerate(f, 1):
            line = line.strip()
            if not line:
                continue
            try:
                obj = json.loads(line)
            except ValueError as e:
                raise StageError("data_mismatch", "%s line %d: invalid JSON (%s)" % (split, n, e))
            problem = row_problem(obj, fmt)
            if problem:
                raise StageError("data_mismatch", "%s line %d: %s" % (split, n, problem))
            obj["_line"] = n
            rows.append(obj)
    if isinstance(expect_rows, int) and len(rows) != expect_rows:
        raise StageError("data_mismatch", "%s has %d rows, the marker says %d" % (split, len(rows), expect_rows))
    return rows


def sample_id(row):
    rid = row.get("id")
    return rid if isinstance(rid, str) and rid else "eval-%d" % row["_line"]


def merge_system(messages):
    msgs = [{"role": m["role"], "content": m["content"]} for m in messages]
    if not msgs or msgs[0]["role"] != "system":
        return msgs
    system, rest = msgs[0]["content"], msgs[1:]
    for m in rest:
        if m["role"] == "user":
            m["content"] = system + "\n\n" + m["content"]
            return rest
    return [{"role": "user", "content": system}] + rest


def render_prompt(tok, messages):
    msgs = [{"role": m["role"], "content": m["content"]} for m in messages]
    try:
        out = tok.apply_chat_template(msgs, add_generation_prompt=True, tokenize=False)
    except TypeError:
        out = tok.apply_chat_template(msgs, add_generation_prompt=True)
    if isinstance(out, str):
        return out
    ids = out.get("input_ids") if hasattr(out, "get") else out
    if ids and isinstance(ids[0], list):
        ids = ids[0]
    return tok.decode(ids, skip_special_tokens=False)


def encode(tok, text):
    return list(tok.encode(text, add_special_tokens=False))


def drop_double_bos(tok, ids):
    bos = getattr(tok, "bos_token_id", None)
    if bos is not None and len(ids) >= 2 and ids[0] == bos and ids[1] == bos:
        return ids[1:]
    return ids


def chat_prompt_ids(tok, messages, system_role):
    msgs = merge_system(messages) if system_role == "merge_into_first_user" else messages
    return drop_double_bos(tok, encode(tok, render_prompt(tok, msgs)))


def truncate_keep_first(seq, max_len):
    return seq if len(seq) <= max_len else seq[:1] + seq[len(seq) - (max_len - 1):]


def build_example(tok, row, fmt, system_role, stop, max_len):
    """(input_ids, labels, truncated): loss on the last answer plus stop, or on all text tokens."""
    if fmt == "text":
        ids = encode(tok, row["text"])
        if tok.bos_token_id is not None:
            ids = [tok.bos_token_id] + ids
        if tok.eos_token_id is not None:
            ids = ids + [tok.eos_token_id]
        labels = list(ids)
    else:
        msgs = row["messages"]
        prompt = chat_prompt_ids(tok, msgs[:-1], system_role)
        answer = encode(tok, msgs[-1]["content"] + stop)
        ids, labels = prompt + answer, [-100] * len(prompt) + answer
    truncated = len(ids) > max_len
    return truncate_keep_first(ids, max_len), truncate_keep_first(labels, max_len), truncated


def split_text_prompt(text):
    mid = len(text) // 2
    cut = text.rfind(" ", 0, mid)
    if cut <= 0:
        cut = mid
    return text[:cut], text[cut:]


def plan_batches(lengths, size, seed, shuffle=True):
    """Length buckets: indices sorted by length, cut into batches, batch order shuffled."""
    order = sorted(range(len(lengths)), key=lambda i: (lengths[i], i))
    batches = [order[i:i + size] for i in range(0, len(order), size)]
    if shuffle:
        random.Random(seed).shuffle(batches)
    return batches


def pad_batch(examples, idxs, pad_id, multiple=8):
    """Right padding to the longest row (rounded up to multiple): (ids, mask, labels)."""
    width = max(len(examples[i][0]) for i in idxs)
    width = ((width + multiple - 1) // multiple) * multiple
    ids, mask, labels = [], [], []
    for i in idxs:
        x, y = examples[i][0], examples[i][1]
        pad = width - len(x)
        ids.append(x + [pad_id] * pad)
        mask.append([1] * len(x) + [0] * pad)
        labels.append(y + [-100] * pad)
    return ids, mask, labels


def label_targets(examples, idxs):
    return sum(sum(1 for y in examples[i][1][1:] if y != -100) for i in idxs)


def steps_per_epoch(n_batches, grad_accum):
    return int(math.ceil(n_batches / float(grad_accum)))


def resolve_targets(wanted, leaves):
    """(resolved, missing, fallback): profile names present in the model, else every text leaf."""
    resolved = [n for n in wanted if n in leaves]
    missing = [n for n in wanted if n not in leaves]
    if resolved:
        return resolved, missing, False
    return sorted(leaves), missing, True


def lora_regex(names):
    alt = "|".join(re.escape(n) for n in sorted(set(names)))
    bad = "|".join(re.escape(m) for m in NON_TEXT_MARKERS)
    return r"^(?!.*(?:%s))(?:.*\.)?(?:%s)$" % (bad, alt)


def stop_token_ids(tok, stop_strings, extra):
    ids = set(x for x in extra if isinstance(x, int))
    if getattr(tok, "eos_token_id", None) is not None:
        ids.add(int(tok.eos_token_id))
    for s in stop_strings:
        enc = encode(tok, s)
        if len(enc) == 1:
            ids.add(enc[0])
    return sorted(ids)


def decode_answer(tok, new_ids, stop_ids, stop_strings):
    stopped = False
    for i, t in enumerate(new_ids):
        if t in stop_ids:
            new_ids, stopped = new_ids[:i], True
            break
    text = tok.decode(new_ids, skip_special_tokens=True)
    for s in stop_strings:
        if s and s in text:
            text, stopped = text[:text.index(s)], True
    return text.strip(), stopped


class StubTokenizer:
    """Character-level tokenizer with a ChatML-like template (selftest and dry run)."""
    SPECIAL = ("<pad>", "<bos>", "<eos>", "<|im_start|>", "<|im_end|>")

    def __init__(self):
        self.vocab = {s: i for i, s in enumerate(self.SPECIAL)}
        self.pad_token_id, self.bos_token_id, self.eos_token_id = 0, 1, 2
        self.pad_token, self.bos_token, self.eos_token = "<pad>", "<bos>", "<eos>"
        self.chat_template = "stub"

    def __len__(self):
        return len(self.vocab)

    def encode(self, text, add_special_tokens=False):
        ids, i = [], 0
        while i < len(text):
            for s in self.SPECIAL:
                if text.startswith(s, i):
                    ids.append(self.vocab[s])
                    i += len(s)
                    break
            else:
                ids.append(self.vocab.setdefault(text[i], len(self.vocab)))
                i += 1
        return ([self.bos_token_id] + ids) if add_special_tokens else ids

    def decode(self, ids, skip_special_tokens=False):
        inv = {v: k for k, v in self.vocab.items()}
        return "".join(inv.get(i, "?") for i in ids if not (skip_special_tokens and i < len(self.SPECIAL)))

    def apply_chat_template(self, messages, add_generation_prompt=False, tokenize=False):
        text = self.bos_token
        for m in messages:
            text += "<|im_start|>" + m["role"] + "\n" + m["content"] + "<|im_end|>\n"
        if add_generation_prompt:
            text += "<|im_start|>assistant\n"
        return self.encode(text) if tokenize else text


GGUF_SCALAR = {0: "<B", 1: "<b", 2: "<H", 3: "<h", 4: "<I", 5: "<i", 6: "<f", 7: "<?", 10: "<Q", 11: "<q", 12: "<d"}


def write_fake_gguf(path, arch):
    """Minimal valid GGUF v3: two keys (architecture, alignment), one F32 tensor of 4 values."""
    def s(b):
        return struct.pack("<Q", len(b)) + b
    kv = s(b"general.architecture") + struct.pack("<I", 8) + s(arch.encode("utf-8"))
    kv += s(b"general.alignment") + struct.pack("<I", 4) + struct.pack("<I", 32)
    info = s(b"chibi.fake") + struct.pack("<I", 1) + struct.pack("<Q", 4) + struct.pack("<I", 0) + struct.pack("<Q", 0)
    head = b"GGUF" + struct.pack("<I", 3) + struct.pack("<Q", 1) + struct.pack("<Q", 2) + kv + info
    with open(path, "wb") as f:
        f.write(head + b"\x00" * ((-len(head)) % 32) + struct.pack("<4f", 0.0, 0.0, 0.0, 0.0))


def read_gguf_header(path):
    """Version, tensor count, key count and general.architecture, with the standard library only."""
    with open(path, "rb") as f:
        def take(n):
            b = f.read(n)
            if len(b) != n:
                raise ValueError("truncated GGUF")
            return b

        def u32():
            return struct.unpack("<I", take(4))[0]

        def u64():
            return struct.unpack("<Q", take(8))[0]

        def string():
            n = u64()
            if n > (1 << 24):
                raise ValueError("GGUF string too long")
            return take(n)

        def skip(vtype):
            if vtype == 8:
                string()
            elif vtype == 9:
                etype, count = u32(), u64()
                if etype in GGUF_SCALAR:
                    f.seek(struct.calcsize(GGUF_SCALAR[etype]) * count, 1)
                else:
                    for _ in range(count):
                        skip(etype)
            elif vtype in GGUF_SCALAR:
                take(struct.calcsize(GGUF_SCALAR[vtype]))
            else:
                raise ValueError("unknown GGUF value type %d" % vtype)

        if take(4) != b"GGUF":
            raise ValueError("not a GGUF file")
        version, tensors, kvs = u32(), u64(), u64()
        arch = None
        for _ in range(kvs):
            key, vtype = string().decode("utf-8", "replace"), u32()
            if key == "general.architecture" and vtype == 8:
                arch = string().decode("utf-8", "replace")
                break
            skip(vtype)
    return {"version": version, "tensors": tensors, "kv": kvs, "arch": arch}


def write_safetensors_empty(path, metadata):
    header = json.dumps({"__metadata__": metadata}, separators=(",", ":")).encode("utf-8")
    header += b" " * ((-len(header)) % 8)
    with open(path, "wb") as f:
        f.write(struct.pack("<Q", len(header)) + header)


def safetensors_keys(path):
    with open(path, "rb") as f:
        n = struct.unpack("<Q", f.read(8))[0]
        if n > (100 << 20):
            raise ValueError("safetensors header too large")
        header = json.loads(f.read(n).decode("utf-8"))
    return [k for k in header if k != "__metadata__"]


def model_dir_keys(directory):
    keys = []
    for name in sorted(os.listdir(directory)):
        if name.endswith(".safetensors"):
            keys += safetensors_keys(os.path.join(directory, name))
    return keys


def write_manifest(out_dir, outtype, run_id, profile_id, profile_sha):
    names = output_names(outtype)
    files = []
    for kind in MANIFEST_KINDS:
        path = os.path.join(out_dir, names[kind])
        if os.path.isfile(path):
            files.append({"path": names[kind], "kind": kind, "bytes": os.path.getsize(path),
                          "sha256": sha256_file(path)})
    write_json_atomic(os.path.join(out_dir, MANIFEST_NAME), {"v": 1, "run_id": run_id, "profile_id": profile_id,
                                                             "profile_sha": profile_sha, "files": files})
    return files


def flatten_requirements(path, depth=0):
    lines = []
    with open(path, "r", encoding="utf-8") as f:
        for raw in f:
            line = raw.split(" #", 1)[0].strip()
            if not line or line.startswith("#"):
                continue
            if line.startswith("-r ") or line.startswith("--requirement"):
                include = line.split(None, 1)[1].strip()
                if depth < 3:
                    lines += flatten_requirements(os.path.join(os.path.dirname(path), include), depth + 1)
                continue
            lines.append(line)
    return lines


def adapt_requirements(lines, version):
    """Drop gguf (local gguf-py is installed) and relax numpy~=1.26 on Python 3.13+, which has no wheel."""
    out, relaxed = [], []
    for line in lines:
        name = re.split(r"[<>=~!;\s\[]", line, maxsplit=1)[0].lower()
        if name == "gguf":
            relaxed.append(line + " (local gguf-py)")
            continue
        if name == "numpy" and tuple(version[:2]) >= (3, 13):
            out.append("numpy>=2.1,<3")
            relaxed.append(line)
            continue
        out.append(line)
    return out, relaxed


def script_sha():
    try:
        return sha256_file(os.path.abspath(__file__))
    except OSError:
        return ""


class Ctx:
    """Shared state of one run; every stage updates status, finalize writes it."""

    def __init__(self, args, log, fake):
        self.args, self.log, self.fake = args, log, fake
        self.t0 = time.monotonic()
        self.out = os.path.abspath(args.out)
        self.input = os.path.abspath(args.input)
        self.work = tempfile.mkdtemp(prefix="chibi-dry-") if fake else WORK_ROOT
        self.job, self.profile, self.outtype = None, {}, "q8_0"
        self.budget = Budget(0, 0, t0=self.t0)
        self.status = {
            "v": 1, "run_id": "", "state": "running", "stage": "boot", "code": "", "message": "",
            "warning_code": "", "started": utc_now(), "finished": "",
            "gpu": {"name": "", "capability": "", "vram_gb": None},
            "versions": {"python": "%d.%d.%d" % sys.version_info[:3], "torch": "", "transformers": "",
                         "peft": "", "llama_cpp": ""},
            "train": {"steps": 0, "epochs": 0, "train_loss": None, "eval_loss_base": None,
                      "eval_loss_tuned": None, "stop_reason": "", "truncated_rows": 0},
            "discovery": {"script_sha256": script_sha(), "fake_env": fake, "stage_seconds": {},
                          "warnings": [], "gguf": {}}}
        self.discovery = self.status["discovery"]
        self.torch, self.device, self.model, self.tok, self.loss = None, "cuda:0", None, None, None
        self.rows, self.examples, self.samples = {}, {}, []
        self.snapshot_dir, self.llama_root, self.conv, self.pinned, self.help_cache = None, None, None, None, {}
        self.linear_leaves, self.pad_id, self.stop_ids, self.stop_strings = {}, 0, [], []
        self.base_eval_s, self.gen_manual = 0.0, False
        self.merged_dir = os.path.join(self.work, "merged")
        os.makedirs(self.out, exist_ok=True)
        os.makedirs(self.work, exist_ok=True)
        for name in list(output_names(self.outtype).values()) + [MANIFEST_NAME]:
            if os.path.isfile(os.path.join(self.out, name)):
                os.remove(os.path.join(self.out, name))
        log.metrics_path = self.path("metrics")
        open(log.metrics_path, "w").close()

    def configure(self, job):
        self.job, self.profile = job, job["profile"]
        self.outtype = job["outputs"]["gguf_outtype"]
        self.budget = Budget(job["budget"]["session_timeout_s"], job["budget"]["max_train_s"], t0=self.t0)
        self.status["run_id"] = job["run_id"]
        self.status["versions"]["llama_cpp"] = job["deps"]["llama_cpp"]["tag"]
        self.stop_strings = [s for s in (jget(self.profile, "chat.stop") or []) if s]
        for kind in ("gguf_tuned", "gguf_base"):
            if os.path.isfile(self.path(kind)):
                os.remove(self.path(kind))

    def path(self, kind):
        return os.path.join(self.out, output_names(self.outtype)[kind])

    def warn(self, code, message, primary=False, **fields):
        self.log.emit("warn", code=code, message=one_line(message), **fields)
        self.discovery["warnings"].append(code)
        if primary and not self.status["warning_code"]:
            self.status["warning_code"] = code


def run_cmd(cmd, cwd=None, env_extra=None, timeout=600):
    """(returncode, combined output, seconds); a timeout gives -9, a start failure -1."""
    env = dict(os.environ)
    env.update(env_extra or {})
    t0 = time.monotonic()
    try:
        p = subprocess.run(cmd, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
        rc, out = p.returncode, p.stdout
    except subprocess.TimeoutExpired as e:
        rc, out = -9, (e.stdout or b"") + (b"\n[timeout after %d s]" % timeout)
    except OSError as e:
        rc, out = -1, str(e).encode("utf-8", "replace")
    text = out.decode("utf-8", "replace") if isinstance(out, bytes) else str(out)
    return rc, text, time.monotonic() - t0


def torch_constraint_file(ctx):
    version = dist_version("torch")
    if not version:
        return None
    path = os.path.join(ctx.work, "constraints.txt")
    with open(path, "w", encoding="utf-8") as f:
        f.write("torch==%s\n" % version)
    return path


def pip_install(ctx, specs, required=True, timeout=900):
    cmd = [sys.executable, "-m", "pip", "install", "-q", "--disable-pip-version-check"]
    constraints = torch_constraint_file(ctx)
    if constraints:
        cmd += ["-c", constraints]
    rc, out, secs = run_cmd(cmd + list(specs), timeout=timeout)
    ctx.log.info("pip", specs=list(specs), rc=rc, seconds=rnd(secs, 1))
    if rc == 0:
        return True
    ctx.log.raw("pip install " + " ".join(specs), tail_lines(out))
    message = "pip install %s failed: %s" % (" ".join(specs), last_line(out))
    if required:
        raise StageError("install_failed", message)
    ctx.warn("pip_failed", message)
    return False


def http_get(url, timeout, limit=1 << 20):
    req = urllib.request.Request(url, headers={"User-Agent": "aurago-chibi-trainer", "Accept": "application/json"})
    try:
        with urllib.request.urlopen(req, timeout=timeout) as r:
            return r.status, r.read(limit)
    except urllib.error.HTTPError as e:
        return e.code, b""


def download(url, path, timeout):
    req = urllib.request.Request(url, headers={"User-Agent": "aurago-chibi-trainer"})
    with urllib.request.urlopen(req, timeout=timeout) as r, open(path, "wb") as f:
        shutil.copyfileobj(r, f, 1 << 20)


def vram_gb(torch):
    try:
        return round(torch.cuda.max_memory_allocated() / 1e9, 2)
    except Exception:
        return None


def note_vram(ctx):
    if ctx.torch is not None:
        peak = vram_gb(ctx.torch)
        if peak is not None:
            ctx.discovery["peak_vram_gb"] = max(peak, ctx.discovery.get("peak_vram_gb") or 0.0)


def release(ctx):
    gc.collect()
    if ctx.torch is not None:
        try:
            ctx.torch.cuda.empty_cache()
        except Exception:
            pass


def set_use_cache(model, value):
    try:
        model.config.use_cache = value
    except Exception:
        pass


def stage_boot(ctx):
    try:
        job = read_json(ctx.args.job)
    except (OSError, ValueError) as e:
        raise StageError("job_invalid", "job.json cannot be read: %s" % exc_summary(e))
    problems = validate_job(job)
    if problems:
        raise StageError("job_invalid", "; ".join(problems[:6]))
    ctx.configure(job)
    h = job["hyper"]
    ctx.log.info("job", run_id=job["run_id"], profile=ctx.profile["id"], method=job["method"], recipe=job["recipe"],
                 format=job["data"]["format"], epochs=h["epochs"], lr=h["learning_rate"],
                 per_device_batch=h["per_device_batch"], grad_accum=h["grad_accum"], max_seq_len=h["max_seq_len"],
                 gradient_checkpointing=bool(h.get("gradient_checkpointing")), budget=job["budget"],
                 emit_base_gguf=job["outputs"]["emit_base_gguf"], script_sha256=ctx.discovery["script_sha256"],
                 dry_run=ctx.fake)


def parse_capability(text):
    parts = (str(text).split(".") + ["0"])[:2]
    return int(parts[0]), int(parts[1])


def nvidia_smi():
    exe = shutil.which("nvidia-smi")
    if not exe:
        return []
    rc, out, _ = run_cmd([exe, "--query-gpu=name,compute_cap,memory.total,driver_version",
                          "--format=csv,noheader"], timeout=20)
    if rc != 0:
        return [{"error": last_line(out)}]
    keys = ("name", "compute_cap", "memory", "driver")
    return [dict(zip(keys, [x.strip() for x in line.split(",")])) for line in out.splitlines() if line.strip()]


def check_gpu(ctx, torch, smi):
    props = torch.cuda.get_device_properties(0)
    gpu = {"name": props.name, "capability": "%d.%d" % (props.major, props.minor),
           "vram_gb": round(props.total_memory / 1e9, 2)}
    ctx.status["gpu"].update(gpu)
    ctx.log.info("gpu", vram_gib=round(props.total_memory / float(1 << 30), 2),
                 visible=torch.cuda.device_count(), host_gpus=len(smi), torch_cuda=torch.version.cuda, **gpu)
    training = ctx.profile["training"]
    if (props.major, props.minor) < parse_capability(training["min_gpu_capability"]):
        raise StageError("env_unsupported_gpu", "GPU %s has compute capability %s; this profile needs %s or newer. "
                         "Choose the machine shape NvidiaTeslaT4." % (props.name, gpu["capability"],
                                                                      training["min_gpu_capability"]))
    if gpu["vram_gb"] < float(training["min_vram_gb"]):
        raise StageError("env_unsupported_gpu", "GPU %s has %.1f GB VRAM; this profile needs %.1f GB"
                         % (props.name, gpu["vram_gb"], float(training["min_vram_gb"])))


def stage_env_check(ctx):
    if sys.version_info < (3, 10):
        raise StageError("env_unsupported_python", "Python %s is too old; 3.10 or newer is needed"
                         % ctx.status["versions"]["python"])
    if ctx.fake:
        ctx.status["gpu"].update(name="dry-run T4", capability="7.5", vram_gb=15.8)
        return
    smi = nvidia_smi()
    ctx.discovery["nvidia_smi"] = smi
    import torch
    ctx.torch = torch
    if torch.cuda.is_available():
        check_gpu(ctx, torch, smi)
    elif os.environ.get("CHIBI_ALLOW_CPU") == "1":
        # Local smoke runs only: the whole pipeline on the CPU, to test a change before it costs
        # GPU quota. Nothing in AuraGo sets this variable.
        ctx.device = "cpu"
        ctx.status["gpu"].update(name="cpu", capability="0.0", vram_gb=0)
        ctx.warn("cpu_smoke_run", "CHIBI_ALLOW_CPU=1: this is a CPU smoke run; speed and memory figures mean nothing")
    else:
        raise StageError("env_no_gpu", "CUDA is not available (nvidia-smi: %s); enable the GPU accelerator"
                         % (json.dumps(smi)[:300] or "missing"))
    need_work = 3 * float(ctx.profile["arch"]["params_b"]) * 4e9
    est = float(jget(ctx.profile, "gguf.estimated_mb") or 0) * 1e6
    need_out = 1.2 * est * (2 if ctx.job["outputs"]["emit_base_gguf"] else 1)
    free_work, free_out = shutil.disk_usage(ctx.work).free, shutil.disk_usage(ctx.out).free
    ctx.log.info("disk", work_free_gb=rnd(free_work / 1e9, 1), out_free_gb=rnd(free_out / 1e9, 1),
                 work_need_gb=rnd(need_work / 1e9, 1), out_need_gb=rnd(need_out / 1e9, 1))
    if free_work < need_work or free_out < need_out:
        raise StageError("env_low_disk", "free disk %.1f GB (work) and %.1f GB (output) is below the needed "
                         "%.1f GB and %.1f GB" % (free_work / 1e9, free_out / 1e9, need_work / 1e9, need_out / 1e9))
    check_network(ctx)
    check_base_repo(ctx)


def check_network(ctx):
    for url in ("https://pypi.org/simple/pip/", "https://huggingface.co/api/models/" + ctx.profile["base"]["repo"]):
        host = urllib.parse.urlparse(url).netloc
        t0 = time.monotonic()
        try:
            status, _ = http_get(url, 10, 4096)
        except (urllib.error.URLError, OSError, ValueError) as e:
            raise StageError("env_no_internet", "%s is not reachable (%s). Turn on Internet in the notebook "
                             "settings; Kaggle needs a phone-verified account for it." % (host, exc_summary(e)))
        ctx.log.info("network", host=host, http=status, ms=int((time.monotonic() - t0) * 1000))


def check_base_repo(ctx):
    repo, rev = ctx.profile["base"]["repo"], ctx.profile["base"]["revision"]
    url = "https://huggingface.co/api/models/%s/revision/%s" % (repo, urllib.parse.quote(rev, safe=""))
    try:
        status, body = http_get(url, 15)
    except (urllib.error.URLError, OSError, ValueError) as e:
        ctx.warn("hf_api_unreachable", "model info for %s could not be read: %s" % (repo, exc_summary(e)))
        return
    if status in (401, 403, 404):
        raise StageError("base_download_failed", "%s@%s is not available anonymously (HTTP %d)" % (repo, rev, status))
    info = {"http": status}
    try:
        data = json.loads(body.decode("utf-8"))
        info.update(sha=data.get("sha"), gated=data.get("gated"), private=data.get("private"),
                    last_modified=data.get("lastModified"))
    except ValueError:
        info["parse_error"] = True
    ctx.discovery["hf_repo"] = info
    ctx.log.info("hf_repo", repo=repo, revision=rev, **info)
    if info.get("gated") not in (None, False):
        raise StageError("base_download_failed", "%s is gated now (%s); profiles need an ungated base"
                         % (repo, info.get("gated")))
    if not re.fullmatch(r"[0-9a-f]{40}", rev):
        ctx.warn("unpinned_revision", "base.revision %s is not a commit; it resolved to %s" % (rev, info.get("sha")))


def stage_data(ctx):
    try:
        marker, base = load_marker(ctx.input, ctx.job["data"]["marker"], ctx.job["run_id"])
    except StageError:
        ctx.discovery["input_tree"] = list_tree(ctx.input)
        raise
    files = verify_split_files(marker, base, ctx.input)
    for split, (path, entry) in files.items():
        ctx.rows[split] = parse_rows(path, ctx.job["data"]["format"], entry.get("rows"), split)
    counts = {k: len(v) for k, v in ctx.rows.items()}
    ctx.discovery["data"] = counts
    ctx.log.info("data", marker_dir=os.path.relpath(base, ctx.input), **counts)


def stage_install(ctx):
    lora = ctx.job["method"] == "lora"
    if ctx.fake:
        ctx.status["versions"].update(torch="dry-run", transformers="dry-run", peft="dry-run" if lora else "")
        return
    torch_before = dist_version("torch")
    pins = list(jget(ctx.job, "deps.pip") or []) + list(jget(ctx.profile, "training.extra_pip") or [])
    if pins:
        pip_install(ctx, pins)
    else:
        ctx.warn("unpinned_dependencies", "the lock names no pip pins; missing packages are installed unpinned")
    want = jget(ctx.job, "deps.transformers") or ""
    if want and want != dist_version("transformers"):
        pip_install(ctx, ["transformers==" + want])
    ensure = [("huggingface_hub", "huggingface_hub", True), ("safetensors", "safetensors", True),
              ("sentencepiece", "sentencepiece", False)]
    if lora:
        ensure.insert(0, ("peft", "peft", True))
    for module, package, required in ensure:
        importlib.invalidate_caches()
        if importlib.util.find_spec(module) is None and pip_install(ctx, [package], required=required):
            ctx.warn("unpinned_dependency", "%s was missing and is now installed unpinned (%s)"
                     % (package, dist_version(package)))
    importlib.invalidate_caches()
    for module in ["torch", "transformers", "huggingface_hub", "safetensors"] + (["peft"] if lora else []):
        try:
            importlib.import_module(module)
        except Exception as e:
            raise StageError("install_failed", "import %s failed: %s" % (module, exc_summary(e)))
    import torch
    ctx.torch = torch
    versions = ctx.status["versions"]
    versions.update(torch=torch.__version__, transformers=dist_version("transformers"),
                    peft=dist_version("peft") if lora else "")
    extra = {"huggingface_hub": dist_version("huggingface_hub"), "safetensors": dist_version("safetensors"),
             "sentencepiece": dist_version("sentencepiece"), "numpy": dist_version("numpy"),
             "cuda": torch.version.cuda, "cudnn": torch.backends.cudnn.version()}
    ctx.discovery["versions_extra"] = extra
    ctx.log.info("versions", **versions, **extra)
    if dist_version("torch") != torch_before:
        ctx.warn("torch_changed", "torch changed from %s to %s during install" % (torch_before, dist_version("torch")))


def download_base(ctx):
    from huggingface_hub import snapshot_download
    repo, rev = ctx.profile["base"]["repo"], ctx.profile["base"]["revision"]
    dest = os.path.join(ctx.work, "base")
    last = None
    for attempt in range(1, 4):
        t0 = time.monotonic()
        try:
            try:
                path = snapshot_download(repo_id=repo, revision=rev, local_dir=dest, token=False,
                                         ignore_patterns=SNAPSHOT_IGNORE)
            except TypeError:
                path = snapshot_download(repo_id=repo, revision=rev, local_dir=dest, ignore_patterns=SNAPSHOT_IGNORE)
            names = sorted(os.listdir(path))
            ctx.log.info("base_snapshot", repo=repo, revision=rev, files=names[:40], bytes=dir_bytes(path),
                         seconds=rnd(time.monotonic() - t0, 1))
            return path
        except Exception as e:
            last = e
            ctx.warn("base_download_retry", "attempt %d: %s" % (attempt, exc_summary(e)))
            time.sleep(10 * attempt)
    raise StageError("base_download_failed", "%s@%s: %s" % (repo, rev, exc_summary(last)))


def summarize_config(path):
    cfg = read_json(os.path.join(path, "config.json"))
    text = cfg.get("text_config") if isinstance(cfg.get("text_config"), dict) else {}
    return {"model_type": cfg.get("model_type"), "architectures": cfg.get("architectures"),
            "dtype": cfg.get("dtype", cfg.get("torch_dtype")), "text_model_type": text.get("model_type"),
            "has_vision_config": "vision_config" in cfg,
            "tie_word_embeddings": cfg.get("tie_word_embeddings", text.get("tie_word_embeddings")),
            "vocab_size": cfg.get("vocab_size", text.get("vocab_size")),
            "final_logit_softcapping": cfg.get("final_logit_softcapping", text.get("final_logit_softcapping"))}


def load_tokenizer(ctx, path):
    import transformers
    try:
        tok, how = transformers.AutoTokenizer.from_pretrained(path), "AutoTokenizer"
    except Exception as e:
        ctx.warn("tokenizer_fallback", "AutoTokenizer failed: %s" % exc_summary(e))
        tok, how = transformers.AutoProcessor.from_pretrained(path).tokenizer, "AutoProcessor"
    ctx.discovery["tokenizer"] = {"loader": how, "class": type(tok).__name__}
    return tok


def load_model_chain(ctx, path):
    """D5: try base.auto_class, then base.auto_class_fallbacks; fp32 via dtype, else torch_dtype."""
    import torch
    import transformers
    names = []
    for name in [ctx.profile["base"]["auto_class"]] + list(jget(ctx.profile, "base.auto_class_fallbacks") or []):
        if name not in names:
            names.append(name)
    errors = []
    for name in names:
        cls = getattr(transformers, name, None)
        if cls is None:
            errors.append("%s: not in transformers %s" % (name, transformers.__version__))
            continue
        t0 = time.monotonic()
        try:
            try:
                model = cls.from_pretrained(path, dtype=torch.float32)
            except TypeError as e:
                if "dtype" not in str(e):
                    raise
                model = cls.from_pretrained(path, torch_dtype=torch.float32)
        except Exception as e:
            if is_oom(e):
                raise
            errors.append("%s: %s" % (name, exc_summary(e)[:300]))
            ctx.warn("base_class_failed", errors[-1])
            gc.collect()
            continue
        ctx.log.info("base_loaded", cls=name, model_class=type(model).__name__, seconds=rnd(time.monotonic() - t0, 1))
        return model, name, errors
    raise StageError("base_load_failed", "no class could load the base: " + " | ".join(errors))


def dtype_counts(model):
    out = {}
    for p in model.parameters():
        key = str(p.dtype).replace("torch.", "")
        out[key] = out.get(key, 0) + p.numel()
    return out


def linear_leaves(model, nn):
    """Histogram of nn.Linear leaf names of the text model (no LM head, no vision or audio tower)."""
    counts, examples = {}, {}
    for name, module in model.named_modules():
        if not isinstance(module, nn.Linear):
            continue
        low = name.lower()
        if "lm_head" in low or any(m in low for m in NON_TEXT_MARKERS):
            continue
        leaf = name.rsplit(".", 1)[-1]
        counts[leaf] = counts.get(leaf, 0) + 1
        examples.setdefault(leaf, name)
    return counts, examples


def lora_wanted(ctx):
    return ((ctx.job["hyper"].get("lora") or {}).get("target_modules")
            or jget(ctx.profile, "training.lora.target_modules") or [])


def finish_tokenizer(ctx, model):
    tok = ctx.tok
    eos = tok.eos_token_id
    ctx.pad_id = tok.pad_token_id if tok.pad_token_id is not None else (eos if eos is not None else 0)
    extra = getattr(getattr(model, "generation_config", None), "eos_token_id", None) if model is not None else None
    extra = [extra] if isinstance(extra, int) else list(extra or [])
    ctx.stop_ids = stop_token_ids(tok, ctx.stop_strings, extra)
    info = {"bos": [str(tok.bos_token), tok.bos_token_id], "eos": [str(tok.eos_token), eos], "pad_id": ctx.pad_id,
            "stop_ids": ctx.stop_ids, "vocab": len(tok), "chat_template": bool(getattr(tok, "chat_template", None))}
    ctx.discovery.setdefault("tokenizer", {}).update(info)
    ctx.log.info("tokenizer", **ctx.discovery["tokenizer"])


def template_check(ctx):
    ex = ctx.examples.get("train") or []
    if not ex:
        return
    tok, (ids, labels, _) = ctx.tok, ex[0]
    first = next((i for i, y in enumerate(labels) if y != -100), 0)
    bos = getattr(tok, "bos_token_id", None)
    info = {"prompt_tokens": first, "answer_tokens": len(ids) - first,
            "starts_with_bos": bos is not None and ids[0] == bos, "bos_count": ids.count(bos) if bos is not None else 0,
            "prompt_tail": tok.decode(ids[max(0, first - 24):first]), "answer_head": tok.decode(ids[first:first + 12]),
            "sequence_end": tok.decode(ids[-4:])}
    ctx.discovery["template_check"] = info
    ctx.log.info("template_check", **info)


def make_samples(ctx, fmt, role, max_len):
    tok, cap = ctx.tok, ctx.job["budget"]["eval_max_samples"]
    # The prompt keeps the length a training row may have. It is not shortened by max_new_tokens: the
    # answer is generated after it and the models' context windows are far larger than max_seq_len.
    max_prompt = max(8, max_len)
    out = []
    for row in (ctx.rows.get("eval") or [])[:cap]:
        s = {"id": sample_id(row)}
        if fmt == "text":
            prompt, expected = split_text_prompt(row["text"])
            s["prompt_text"] = prompt
            ids = ([tok.bos_token_id] if tok.bos_token_id is not None else []) + encode(tok, prompt)
        else:
            msgs = row["messages"]
            s["prompt_messages"] = [{"role": m["role"], "content": m["content"]} for m in msgs[:-1]]
            expected = msgs[-1]["content"]
            ids = chat_prompt_ids(tok, msgs[:-1], role)
        s.update(expected=expected, base_output=None, tuned_output=None, base_ms=None, tuned_ms=None, truncated=False)
        s["_prompt_ids"] = truncate_keep_first(ids, max_prompt)
        out.append(s)
    return out


def write_samples(ctx):
    write_jsonl_atomic(ctx.path("eval_samples"),
                       [{k: v for k, v in s.items() if not k.startswith("_")} for s in ctx.samples])


def prepare_examples(ctx):
    tok, job = ctx.tok, ctx.job
    fmt, role = job["data"]["format"], ctx.profile["chat"]["system_role"]
    stop = ctx.stop_strings[0] if ctx.stop_strings else (getattr(tok, "eos_token", None) or "")
    max_len = job["hyper"]["max_seq_len"]
    stats = {}
    for split, rows in ctx.rows.items():
        ex = []
        for row in rows:
            ids, labels, cut = build_example(tok, row, fmt, role, stop, max_len)
            if not any(y != -100 for y in labels[1:]):
                raise StageError("data_mismatch", "%s line %d has no answer tokens after templating"
                                 % (split, row["_line"]))
            ex.append((ids, labels, cut))
        ctx.examples[split] = ex
        lens = sorted(len(e[0]) for e in ex)
        stats[split] = {"rows": len(ex), "tokens": sum(lens), "max": lens[-1] if lens else 0,
                        "p95": lens[int(0.95 * (len(lens) - 1))] if lens else 0,
                        "truncated": sum(1 for e in ex if e[2]), "targets": label_targets(ex, range(len(ex)))}
    truncated = stats.get("train", {}).get("truncated", 0)
    ctx.status["train"]["truncated_rows"] = truncated
    if truncated:
        ctx.warn("rows_truncated", "%d train rows are longer than max_seq_len %d" % (truncated, max_len))
    ctx.discovery["tokens"] = stats
    ctx.log.info("tokens", **stats)
    template_check(ctx)
    ctx.samples = make_samples(ctx, fmt, role, max_len)


def stage_load_base(ctx):
    if ctx.fake:
        ctx.tok = StubTokenizer()
        ctx.linear_leaves = {name: 4 for name in (lora_wanted(ctx) or ["q_proj"])}
        ctx.discovery.update(base_class=ctx.profile["base"]["auto_class"], linear_leaves=ctx.linear_leaves)
        finish_tokenizer(ctx, None)
        prepare_examples(ctx)
        return
    import torch
    ctx.torch = torch
    path = download_base(ctx)
    ctx.snapshot_dir = path
    ctx.discovery["base_config"] = summarize_config(path)
    ctx.log.info("base_config", **ctx.discovery["base_config"])
    ctx.tok = load_tokenizer(ctx, path)
    model, name, errors = load_model_chain(ctx, path)
    loaded = dtype_counts(model)
    model = model.to(dtype=torch.float32)
    model.to(ctx.device)
    set_use_cache(model, True)
    ctx.model = model
    leaves, examples = linear_leaves(model, torch.nn)
    ctx.linear_leaves = leaves
    non_text = any(any(m in n.lower() for m in NON_TEXT_MARKERS) for n, _ in model.named_modules())
    ctx.discovery.update(base_class=name, model_class=type(model).__name__, base_class_errors=errors,
                         params=sum(p.numel() for p in model.parameters()), dtype_loaded=loaded,
                         dtype_now=dtype_counts(model), has_non_text_modules=non_text, linear_leaves=leaves,
                         linear_examples=examples, vram_after_load_gb=vram_gb(torch),
                         attn_implementation=getattr(model.config, "_attn_implementation", None))
    ctx.log.info("linear_leaves", leaves=leaves, examples=examples)
    if non_text:
        ctx.warn("non_text_modules", "class %s loaded vision or audio modules; LoRA skips them" % name)
    finish_tokenizer(ctx, model)
    prepare_examples(ctx)


def extract_tar(path, dest):
    with tarfile.open(path, "r:gz") as tf:
        members = [m for m in tf.getmembers() if (m.isfile() or m.isdir()) and not m.name.startswith("/")
                   and ".." not in m.name.replace("\\", "/").split("/")]
        try:
            tf.extractall(dest, members=members, filter="data")
        except TypeError:
            tf.extractall(dest, members=members)


def fetch_llama(ctx):
    dep = ctx.job["deps"]["llama_cpp"]
    tar = os.path.join(ctx.work, "llama.tar.gz")
    last, t0 = None, time.monotonic()
    for attempt in range(1, 4):
        try:
            download(dep["url"], tar, 120)
            break
        except (urllib.error.URLError, OSError, ValueError) as e:
            last = e
            ctx.warn("llama_download_retry", "attempt %d: %s" % (attempt, exc_summary(e)))
            time.sleep(5 * attempt)
    else:
        raise StageError("gguf_failed", "llama.cpp %s could not be downloaded: %s" % (dep["tag"], exc_summary(last)))
    got, want = sha256_file(tar), (dep.get("sha256") or "").lower()
    ctx.discovery["llama_cpp_sha256"] = got
    ctx.log.info("llama_cpp", tag=dep["tag"], sha256=got, bytes=os.path.getsize(tar), pinned=bool(want),
                 seconds=rnd(time.monotonic() - t0, 1))
    if want and got != want:
        raise StageError("gguf_failed", "llama.cpp tarball sha256 %s does not match the pin %s" % (got, want))
    if not want:
        ctx.warn("llama_cpp_unpinned", "llama.cpp %s tarball sha256 is %s; pin it in requirements.lock" % (dep["tag"], got))
    dest = os.path.join(ctx.work, "llama")
    shutil.rmtree(dest, ignore_errors=True)
    extract_tar(tar, dest)
    candidates = [dest] + [os.path.join(dest, n) for n in sorted(os.listdir(dest))]
    root = next((c for c in candidates if os.path.isfile(os.path.join(c, "convert_hf_to_gguf.py"))), None)
    if root is None or not os.path.isdir(os.path.join(root, "gguf-py")):
        raise StageError("gguf_failed", "the llama.cpp tarball has no convert_hf_to_gguf.py with gguf-py")
    ctx.llama_root = root
    ctx.discovery["llama_cpp_layout"] = {"conversion_dir": os.path.isdir(os.path.join(root, "conversion")),
                                         "requirements": os.path.isdir(os.path.join(root, "requirements"))}


def system_env():
    return {"name": "system", "python": sys.executable, "env": {}, "detail": "notebook"}


def build_pinned_env(ctx):
    """Virtual environment from the tarball's converter requirements; pip --target when venv is unavailable."""
    root = ctx.llama_root
    reqs = flatten_requirements(os.path.join(root, "requirements", "requirements-convert_hf_to_gguf.txt"))
    reqs, relaxed = adapt_requirements(reqs, sys.version_info)
    req_path = os.path.join(ctx.work, "convert-requirements.txt")
    with open(req_path, "w", encoding="utf-8") as f:
        f.write("\n".join(reqs) + "\n")
    ctx.log.info("pinned_env_requirements", lines=reqs, relaxed=relaxed)
    gguf_py = os.path.join(root, "gguf-py")
    venv = os.path.join(ctx.work, "convenv")
    rc, out, _ = run_cmd([sys.executable, "-m", "venv", venv], timeout=300)
    if rc == 0:
        py = os.path.join(venv, "Scripts" if os.name == "nt" else "bin", "python")
        rc, out, _ = run_cmd([py, "-m", "pip", "install", "-q", "-r", req_path, gguf_py], timeout=1500)
        if rc == 0:
            return {"name": "pinned", "python": py, "env": {}, "detail": "venv"}
        ctx.log.raw("pinned venv pip", tail_lines(out))
        ctx.warn("pinned_env_failed", "venv install failed: %s" % last_line(out))
    else:
        ctx.warn("venv_unavailable", "python -m venv failed: %s" % last_line(out))
    target = os.path.join(ctx.work, "convlib")
    rc, out, _ = run_cmd([sys.executable, "-m", "pip", "install", "-q", "--target", target, "-r", req_path, gguf_py],
                         timeout=1500)
    if rc == 0:
        return {"name": "pinned", "python": sys.executable, "env": {"PYTHONPATH": target}, "detail": "target"}
    ctx.log.raw("pinned target pip", tail_lines(out))
    ctx.warn("pinned_env_failed", "pip --target failed: %s" % last_line(out))
    return None


def ensure_pinned_env(ctx):
    if ctx.pinned is not None:
        return ctx.pinned or None
    if ctx.budget.remaining() < 600:
        ctx.warn("pinned_env_skipped", "too little session time to build the pinned converter environment")
        ctx.pinned = False
        return None
    t0 = time.monotonic()
    ctx.pinned = build_pinned_env(ctx) or False
    ctx.log.info("pinned_env", ok=bool(ctx.pinned), detail=(ctx.pinned or {}).get("detail"),
                 seconds=rnd(time.monotonic() - t0, 1))
    return ctx.pinned or None


def converter_help(ctx, env):
    key = env["name"]
    if key not in ctx.help_cache:
        script = os.path.join(ctx.llama_root, "convert_hf_to_gguf.py")
        rc, out, secs = run_cmd([env["python"], script, "--help"], cwd=ctx.llama_root, env_extra=env["env"], timeout=600)
        ctx.log.info("converter_help", env=key, rc=rc, seconds=rnd(secs, 1), error=None if rc == 0 else last_line(out))
        if rc != 0:
            ctx.log.raw("convert_hf_to_gguf.py --help (%s)" % key, tail_lines(out))
        ctx.help_cache[key] = out if rc == 0 else None
    return ctx.help_cache[key]


# The converter accepts --mtp / --no-mtp only for these architectures and fails for every other one.
MTP_ARCHS = ("qwen35", "qwen35moe")


def converter_flags(ctx, help_text, model_dir):
    """--no-mtp for a Qwen3.5 checkpoint without MTP tensors, when the converter knows the flag."""
    if jget(ctx.profile, "gguf.expected_arch") not in MTP_ARCHS or "--no-mtp" not in help_text:
        return []
    try:
        keys = model_dir_keys(model_dir)
    except (OSError, ValueError):
        return []
    return ["--no-mtp"] if keys and not any("mtp" in k for k in keys) else []


def convert(ctx, src, dst, label, vocab_only=False):
    """Run the converter in the chosen environment; the system one falls back to the pinned one (D9)."""
    script = os.path.join(ctx.llama_root, "convert_hf_to_gguf.py")
    candidates = [ctx.conv or system_env()]
    if candidates[0]["name"] == "system":
        candidates.append(None)
    attempts = []
    for env in candidates:
        if env is None:
            env = ensure_pinned_env(ctx)
            if env is None:
                attempts.append("pinned: environment not available")
                break
        help_text = converter_help(ctx, env)
        if help_text is None:
            attempts.append("%s: the converter does not start" % env["name"])
            continue
        flags = converter_flags(ctx, help_text, src) + (["--vocab-only"] if vocab_only else [])
        timeout = max(120, min(1800, int(ctx.budget.remaining()) - 60))
        cmd = [env["python"], script, src, "--outtype", ctx.outtype, "--outfile", dst] + flags
        rc, out, secs = run_cmd(cmd, cwd=ctx.llama_root, env_extra=env["env"], timeout=timeout)
        ok = rc == 0 and os.path.isfile(dst) and os.path.getsize(dst) > 0
        ctx.log.info("converter_run", label=label, env=env["name"], flags=flags, rc=rc, ok=ok, seconds=rnd(secs, 1))
        if ok:
            ctx.conv = env
            ctx.discovery.update(converter_env=env["name"], converter_env_detail=env["detail"])
            return
        tail = tail_lines(out)
        ctx.log.raw("converter %s (%s)" % (label, env["name"]), tail)
        ctx.warn("converter_failed", "%s conversion in the %s environment failed: %s" % (label, env["name"], last_line(out)),
                 tail=tail[-12:])
        ctx.discovery.setdefault("converter_tail", {})["%s/%s" % (label, env["name"])] = "\n".join(tail)[-3000:]
        attempts.append("%s: %s" % (env["name"], last_line(out)))
        if os.path.isfile(dst):
            os.remove(dst)
    raise StageError("gguf_failed", "%s conversion failed (%s)" % (label, " | ".join(attempts)))


def gguf_summary(ctx, path):
    gguf_py = os.path.join(ctx.llama_root, "gguf-py")
    if gguf_py not in sys.path:
        sys.path.insert(0, gguf_py)
    from gguf import GGUFReader
    reader = GGUFReader(path)
    info = {"tensors": len(reader.tensors), "types": {}}
    field = reader.fields.get("general.architecture")
    if field is not None:
        info["arch"] = bytes(field.parts[field.data[0]]).decode("utf-8", "replace")
    for t in reader.tensors:
        key = getattr(t.tensor_type, "name", str(t.tensor_type))
        info["types"][key] = info["types"].get(key, 0) + 1
    del reader
    return info


def verify_gguf(ctx, path, label, need_tensors=True):
    hdr = read_gguf_header(path)
    info = {"bytes": os.path.getsize(path), "version": hdr["version"], "tensors": hdr["tensors"], "arch": hdr["arch"]}
    if not ctx.fake:
        try:
            r = gguf_summary(ctx, path)
            info.update(reader_arch=r.get("arch"), reader_tensors=r.get("tensors"), types=r.get("types"))
        except Exception as e:
            info["reader_error"] = exc_summary(e)[:300]
            ctx.warn("gguf_reader_failed", info["reader_error"])
    ctx.discovery["gguf"][label] = info
    ctx.log.info("gguf", label=label, **info)
    expected = ctx.profile["gguf"]["expected_arch"]
    arch = info.get("reader_arch") or info["arch"]
    if arch != expected:
        raise StageError("gguf_arch_mismatch", "%s GGUF architecture is %r, the profile expects %r" % (label, arch, expected))
    if need_tensors and not (info.get("reader_tensors") or info["tensors"]):
        raise StageError("gguf_failed", "%s GGUF has no tensors" % label)


def base_gguf_wanted(ctx):
    if not ctx.job["outputs"]["emit_base_gguf"]:
        return False
    spare = ctx.budget.remaining() - ctx.budget.max_train_s - SAVE_GGUF_RESERVE_S
    if spare < BASE_GGUF_MIN_S:
        ctx.warn("base_gguf_skipped", "base GGUF skipped: %d s of spare session time, %d s needed"
                 % (spare, BASE_GGUF_MIN_S), primary=True)
        return False
    return True


def stage_converter(ctx):
    emit_base = base_gguf_wanted(ctx)
    if ctx.fake:
        ctx.discovery.update(llama_cpp_sha256="dry-run", converter_env="system", converter_env_detail="dry-run")
        if emit_base:
            write_fake_gguf(ctx.path("gguf_base"), ctx.profile["gguf"]["expected_arch"])
            verify_gguf(ctx, ctx.path("gguf_base"), "base")
        return
    fetch_llama(ctx)
    pip_install(ctx, [os.path.join(ctx.llama_root, "gguf-py")], required=False)
    ctx.conv = system_env()
    if emit_base:
        convert(ctx, ctx.snapshot_dir, ctx.path("gguf_base"), "base")
        verify_gguf(ctx, ctx.path("gguf_base"), "base")
        return
    probe = os.path.join(ctx.work, "vocab-probe.gguf")
    convert(ctx, ctx.snapshot_dir, probe, "vocab_probe", vocab_only=True)
    verify_gguf(ctx, probe, "vocab_probe", need_tensors=False)
    os.remove(probe)


class LossFn:
    """Next-token cross entropy. 'chunked_head' runs the decoder and applies the LM head in checkpointed
    chunks over label positions only; 'full_logits' uses the model forward (fallback)."""

    def __init__(self, ctx, model):
        import torch
        import torch.utils.checkpoint  # noqa: F401
        self.torch, self.model = torch, model
        self.head = model.get_output_embeddings()
        try:
            self.decoder = model.get_decoder()
        except Exception:
            self.decoder = None
        cfg = getattr(model, "config", None)
        text_cfg = cfg.get_text_config() if hasattr(cfg, "get_text_config") else cfg
        self.softcap = getattr(text_cfg, "final_logit_softcapping", None)
        vocab = int(self.head.weight.shape[0]) if self.head is not None else 1
        self.chunk = max(64, min(4096, (1 << 27) // max(1, vocab)))
        self.path = "chunked_head" if self.decoder is not None and self.head is not None else "full_logits"

    def _ce(self, logits, targets):
        return self.torch.nn.functional.cross_entropy(logits, targets, reduction="sum")

    def _chunk(self, h, t):
        logits = self.head(h).float()
        if self.softcap:
            logits = self.torch.tanh(logits / self.softcap) * self.softcap
        return self._ce(logits, t)

    def sum_loss(self, ids, mask, labels, grad):
        """(sum of token losses, number of target tokens); (None, 0) without targets."""
        y = labels[:, 1:]
        keep = y != -100
        n = int(keep.sum().item())
        if n == 0:
            return None, 0
        if self.path == "chunked_head":
            out = self.decoder(input_ids=ids, attention_mask=mask, use_cache=False)
            h = out.last_hidden_state if hasattr(out, "last_hidden_state") else out[0]
            h, t = h[:, :-1][keep], y[keep]
            total = None
            for s in range(0, n, self.chunk):
                hs, ts = h[s:s + self.chunk], t[s:s + self.chunk]
                part = (self.torch.utils.checkpoint.checkpoint(self._chunk, hs, ts, use_reentrant=False)
                        if grad else self._chunk(hs, ts))
                total = part if total is None else total + part
            return total, n
        logits = self.model(input_ids=ids, attention_mask=mask, use_cache=False).logits
        return self._ce(logits[:, :-1][keep].float(), y[keep]), n


def to_tensors(ctx, examples, idxs):
    ids, mask, labels = pad_batch(examples, idxs, ctx.pad_id)
    t = ctx.torch
    return (t.tensor(ids, device=ctx.device), t.tensor(mask, device=ctx.device), t.tensor(labels, device=ctx.device))


def check_loss_path(ctx, model):
    """Compare the chunked loss with the plain model loss on the shortest train row once."""
    loss = ctx.loss
    if loss.path == "chunked_head":
        ex = ctx.examples["train"]
        i = min(range(len(ex)), key=lambda k: len(ex[k][0]))
        try:
            with ctx.torch.no_grad():
                a, n = loss.sum_loss(*to_tensors(ctx, ex, [i]), grad=False)
                loss.path = "full_logits"
                b, _ = loss.sum_loss(*to_tensors(ctx, ex, [i]), grad=False)
                loss.path = "chunked_head"
            a, b = float(a) / n, float(b) / n
            rel = abs(a - b) / max(1e-6, abs(b))
            ctx.discovery["loss_path_check"] = {"chunked": rnd(a, 6), "full": rnd(b, 6), "rel_diff": rnd(rel, 8)}
            ctx.log.info("loss_path_check", **ctx.discovery["loss_path_check"])
            if not rel <= 1e-3:
                loss.path = "full_logits"
                ctx.warn("loss_path_mismatch", "chunked loss %.6f differs from model loss %.6f; using full logits" % (a, b))
        except Exception as e:
            if is_oom(e):
                raise
            loss.path = "full_logits"
            ctx.warn("loss_path_fallback", "chunked loss failed, using full logits: %s" % exc_summary(e))
    ctx.discovery["loss_path"] = loss.path


def eval_loss(ctx, model, examples, batch_size):
    if not examples:
        return None
    model.eval()
    for size in (batch_size, 1):
        total, count, oom = 0.0, 0, False
        try:
            with ctx.torch.no_grad():
                for idxs in plan_batches([len(e[0]) for e in examples], size, 0, shuffle=False):
                    s, n = ctx.loss.sum_loss(*to_tensors(ctx, examples, idxs), grad=False)
                    if n:
                        total += float(s.item())
                        count += n
        except Exception as e:
            if not is_oom(e) or size == 1:
                raise
            oom = True
        if not oom:
            return total / count if count else None
        release(ctx)
        ctx.warn("eval_batch_reduced", "eval loss ran out of memory at batch %d; retrying with 1" % size)
    return None


def manual_greedy(ctx, model, prompt_ids, max_new, cap_s):
    ids, new, t0 = list(prompt_ids), [], time.monotonic()
    for _ in range(max_new):
        if time.monotonic() - t0 > cap_s:
            break
        x = ctx.torch.tensor([ids], device=ctx.device)
        nxt = int(ctx.torch.argmax(model(input_ids=x, use_cache=False).logits[0, -1]).item())
        new.append(nxt)
        ids.append(nxt)
        if nxt in ctx.stop_ids:
            break
    return new


def greedy_answer(ctx, model, prompt_ids, max_new, cap_s):
    """(text, ms, truncated, new tokens): greedy, stops at EOS and chat.stop, time-capped."""
    t0, new = time.monotonic(), None
    if not ctx.gen_manual:
        try:
            import transformers
            cfg = transformers.GenerationConfig(max_new_tokens=max_new, do_sample=False, eos_token_id=ctx.stop_ids or None,
                                                pad_token_id=ctx.pad_id, max_time=cap_s, use_cache=True)
            ids = ctx.torch.tensor([prompt_ids], device=ctx.device)
            out = model.generate(input_ids=ids, attention_mask=ctx.torch.ones_like(ids), generation_config=cfg)
            new = out[0, ids.shape[1]:].tolist()
        except Exception as e:
            if is_oom(e):
                raise
            ctx.gen_manual = True
            ctx.warn("generate_fallback", "model.generate failed, using a plain greedy loop: %s" % exc_summary(e))
    if new is None:
        new = manual_greedy(ctx, model, prompt_ids, max_new, cap_s - (time.monotonic() - t0))
    text, stopped = decode_answer(ctx.tok, new, ctx.stop_ids, ctx.stop_strings)
    return text, int((time.monotonic() - t0) * 1000), not stopped, len(new)


def run_eval(ctx, model, which, available_s):
    result = {"loss": None, "answers": 0, "tok_s": None}
    examples = ctx.examples.get("eval") or []
    if not examples:
        ctx.warn("no_eval_rows", "the snapshot has no eval split; %s evaluation skipped" % which)
        return result
    t0 = time.monotonic()
    result["loss"] = eval_loss(ctx, model, examples, ctx.job["hyper"]["per_device_batch"])
    loss_s = time.monotonic() - t0
    max_new = ctx.job["budget"]["max_new_tokens"]
    per_cap, total_cap = generation_plan(len(ctx.samples), max_new,
                                         ctx.profile["training"]["t4_gen_tokens_per_second"], available_s - loss_s)
    set_use_cache(model, True)
    model.eval()
    g0, tokens = time.monotonic(), 0
    with ctx.torch.no_grad():
        for i, s in enumerate(ctx.samples):
            if time.monotonic() - g0 > total_cap:
                ctx.warn("eval_samples_shortened", "%s answers stopped after %d of %d samples (cap %d s)"
                         % (which, i, len(ctx.samples), total_cap))
                break
            text, ms, truncated, n = greedy_answer(ctx, model, s["_prompt_ids"], max_new, per_cap)
            s[which + "_output"], s[which + "_ms"] = text, ms
            s["truncated"] = bool(s["truncated"] or truncated)
            tokens += n
            result["answers"] += 1
            if i == 0:
                ctx.log.info("first_answer", which=which, ms=ms, tokens=n, truncated=truncated, preview=text[:160])
    gen_s = time.monotonic() - g0
    result["tok_s"] = tokens / gen_s if gen_s > 0 and tokens else None
    ctx.log.emit("eval", which=which, loss=rnd(result["loss"]), rows=len(examples), answers=result["answers"],
                 samples=len(ctx.samples), tok_s=rnd(result["tok_s"], 1), loss_s=rnd(loss_s, 1), gen_s=rnd(gen_s, 1))
    return result


def fake_eval(ctx, which):
    loss = 2.5 if which == "base" else 1.2
    if ctx.examples.get("eval"):
        ctx.status["train"]["eval_loss_" + which] = loss
    for s in ctx.samples:
        s[which + "_output"] = "" if which == "base" else s["expected"]
        s[which + "_ms"] = 1
    ctx.log.emit("eval", which=which, loss=loss if ctx.examples.get("eval") else None,
                 rows=len(ctx.examples.get("eval") or []), answers=len(ctx.samples), samples=len(ctx.samples))


def stage_base_eval(ctx):
    if ctx.fake:
        fake_eval(ctx, "base")
        write_samples(ctx)
        return
    ctx.loss = LossFn(ctx, ctx.model)
    check_loss_path(ctx, ctx.model)
    t0 = time.monotonic()
    available = ctx.budget.remaining() - ctx.budget.max_train_s - SAVE_GGUF_RESERVE_S - 60
    res = run_eval(ctx, ctx.model, "base", available)
    ctx.base_eval_s = time.monotonic() - t0
    ctx.status["train"]["eval_loss_base"] = rnd(res["loss"])
    ctx.discovery["gen_tok_s_base"] = rnd(res["tok_s"], 1)
    write_samples(ctx)


def stage_tuned_eval(ctx):
    if ctx.fake:
        fake_eval(ctx, "tuned")
        write_samples(ctx)
        return
    try:
        ctx.model.gradient_checkpointing_disable()
    except Exception:
        pass
    res = run_eval(ctx, ctx.model, "tuned", ctx.budget.remaining() - SAVE_GGUF_RESERVE_S - 30)
    ctx.status["train"]["eval_loss_tuned"] = rnd(res["loss"])
    ctx.discovery["gen_tok_s_tuned"] = rnd(res["tok_s"], 1)
    write_samples(ctx)


def record_targets(ctx, wanted, resolved, missing, fallback):
    for name in missing:
        ctx.warn("lora_target_missing", "profile LoRA target %s matches no linear layer" % name)
    if fallback and resolved:
        ctx.warn("lora_targets_fallback", "no profile target matched; using every text linear layer: %s"
                 % ", ".join(resolved))
    if not resolved:
        raise StageError("lora_targets_missing", "the model has no text linear layers to adapt")
    ctx.discovery["lora_targets"] = {"wanted": wanted, "resolved": resolved, "missing": missing, "fallback": fallback}


def enable_checkpointing(ctx, model):
    try:
        try:
            model.gradient_checkpointing_enable(gradient_checkpointing_kwargs={"use_reentrant": False})
        except TypeError:
            model.gradient_checkpointing_enable()
        ctx.discovery["gradient_checkpointing"] = True
    except Exception as e:
        ctx.discovery["gradient_checkpointing"] = False
        ctx.warn("checkpointing_unavailable", "gradient checkpointing could not be enabled: %s" % exc_summary(e))


def wrap_lora(ctx, model, resolved):
    import peft
    lora = ctx.job["hyper"]["lora"]
    cfg = peft.LoraConfig(r=int(lora["r"]), lora_alpha=int(lora["alpha"]), lora_dropout=float(lora.get("dropout") or 0.0),
                          target_modules=lora_regex(resolved), bias="none", task_type="CAUSAL_LM")
    if ctx.job["hyper"].get("gradient_checkpointing") and hasattr(model, "enable_input_require_grads"):
        model.enable_input_require_grads()
    model = peft.get_peft_model(model, cfg)
    wrapped = sum(1 for _, m in model.named_modules() if hasattr(m, "lora_A") and hasattr(m, "base_layer"))
    ctx.discovery["lora_wrapped_modules"] = wrapped
    ctx.log.info("lora", targets=resolved, wrapped_modules=wrapped, r=lora["r"], alpha=lora["alpha"])
    if wrapped == 0:
        raise StageError("lora_targets_missing", "peft wrapped no module for targets %s" % ", ".join(resolved))
    return model


def probe_batch(ctx, model, params, pdb, ga):
    """Forward and backward on the longest rows with the AdamW state reserved; halve the batch on OOM."""
    torch, ex = ctx.torch, ctx.examples["train"]
    longest = sorted(range(len(ex)), key=lambda i: (-len(ex[i][0]), i))
    opt_elems = 2 * sum(p.numel() for p in params)
    while True:
        idxs, oom, t0, reserve = longest[:pdb], False, time.monotonic(), None
        on_gpu = ctx.device != "cpu"
        if on_gpu:
            torch.cuda.reset_peak_memory_stats()
        try:
            reserve = torch.empty(opt_elems, dtype=torch.float32, device=ctx.device)
            s, n = ctx.loss.sum_loss(*to_tensors(ctx, ex, idxs), grad=True)
            if s is not None:
                (s / n).backward()
            if on_gpu:
                torch.cuda.synchronize()
        except Exception as e:
            if not is_oom(e):
                raise
            oom = True
        reserve = None
        model.zero_grad(set_to_none=True)
        release(ctx)
        info = {"per_device_batch": pdb, "width": len(ex[idxs[0]][0]), "ok": not oom,
                "peak_vram_gb": rnd(vram_gb(torch) or 0.0, 2), "seconds": rnd(time.monotonic() - t0, 2)}
        ctx.discovery.setdefault("memory_probe", []).append(info)
        ctx.log.info("memory_probe", **info)
        if not oom:
            return pdb, ga
        if pdb == 1:
            raise StageError("oom", "one sequence of %d tokens does not fit into the memory of %s (method %s); "
                             "choose preset quick or a lower max_seq_len"
                             % (info["width"], ctx.status["gpu"]["name"], ctx.job["method"]))
        pdb, ga = max(1, pdb // 2), ga * 2
        ctx.warn("batch_reduced", "per_device_batch lowered to %d and grad_accum raised to %d after an "
                 "out-of-memory probe" % (pdb, ga))


def fake_train(ctx):
    h = ctx.job["hyper"]
    lengths = [len(e[0]) for e in ctx.examples["train"]]
    spe = steps_per_epoch(len(plan_batches(lengths, h["per_device_batch"], 0, shuffle=False)), h["grad_accum"])
    max_steps = spe * h["epochs"]
    warmup = int(math.ceil(max_steps * float(h.get("warmup_ratio") or 0.0)))
    every = max(1, int(jget(ctx.job, "log.metric_every_steps") or 10))
    loss = None
    for step in range(1, max_steps + 1):
        loss = 0.5 + 2.0 * math.exp(-3.0 * step / max_steps)
        if step == 1 or step % every == 0 or step == max_steps:
            ctx.log.metric(kind="train", step=step, max_steps=max_steps, epoch=rnd(step / float(spe), 3), loss=rnd(loss),
                           lr=rnd(h["learning_rate"] * lr_factor(step - 1, warmup, max_steps), 8), grad_norm=1.0,
                           tok_s=1000.0, elapsed_s=step, eta_s=max_steps - step, vram_gb=1.0)
        if step % spe == 0 and ctx.examples.get("val"):
            ctx.log.metric(kind="eval", step=step, max_steps=max_steps, epoch=step // spe, loss=1.5, eval_loss=1.5,
                           elapsed_s=step, vram_gb=1.0)
    ctx.status["train"].update(steps=max_steps, epochs=float(h["epochs"]), train_loss=rnd(loss), stop_reason="done")
    ctx.discovery["train_tok_s"] = 1000.0


def stage_train(ctx):
    h, lora = ctx.job["hyper"], ctx.job["method"] == "lora"
    resolved = []
    if lora:
        wanted = lora_wanted(ctx)
        resolved, missing, fallback = resolve_targets(wanted, ctx.linear_leaves)
        record_targets(ctx, wanted, resolved, missing, fallback)
    if ctx.fake:
        fake_train(ctx)
        return
    torch = ctx.torch
    random.seed(h["seed"])
    torch.manual_seed(h["seed"])
    torch.cuda.manual_seed_all(h["seed"])
    model = ctx.model
    if h.get("gradient_checkpointing"):
        enable_checkpointing(ctx, model)
    set_use_cache(model, False)
    if lora:
        model = wrap_lora(ctx, model, resolved)
    else:
        for p in model.parameters():
            p.requires_grad_(True)
    ctx.model = model
    params = [p for p in model.parameters() if p.requires_grad]
    ctx.discovery["trainable_params"] = sum(p.numel() for p in params)
    path = ctx.loss.path
    ctx.loss = LossFn(ctx, model)
    if ctx.loss.path == "chunked_head":
        ctx.loss.path = path
    pdb, ga = probe_batch(ctx, model, params, h["per_device_batch"], h["grad_accum"])
    opt = torch.optim.AdamW(params, lr=h["learning_rate"], weight_decay=float(h.get("weight_decay") or 0.0))
    examples = ctx.examples["train"]
    lengths = [len(e[0]) for e in examples]
    n_batches = len(plan_batches(lengths, pdb, 0, shuffle=False))
    spe = steps_per_epoch(n_batches, ga)
    max_steps = spe * h["epochs"]
    warmup = int(math.ceil(max_steps * float(h.get("warmup_ratio") or 0.0)))
    post = SAVE_GGUF_RESERVE_S + 1.2 * ctx.base_eval_s + 30
    deadline = ctx.budget.train_deadline(post)
    every = max(1, int(jget(ctx.job, "log.metric_every_steps") or 10))
    ctx.log.info("train_plan", per_device_batch=pdb, grad_accum=ga, batches_per_epoch=n_batches, steps_per_epoch=spe,
                 max_steps=max_steps, warmup_steps=warmup, train_window_s=int(deadline - time.monotonic()),
                 post_reserve_s=int(post), trainable_params=ctx.discovery["trainable_params"], loss_path=ctx.loss.path)
    if deadline - time.monotonic() < 60:
        raise StageError("time_budget", "only %d s are left for training" % int(deadline - time.monotonic()))
    model.train()
    step, stop_reason, oom_skips, rates = 0, "done", 0, []
    win_loss, win_tok, win_t, last_loss, gn, lr_now = [], 0, time.monotonic(), None, None, 0.0
    t_train = time.monotonic()

    def emit(now):
        elapsed = now - t_train
        tok_s = win_tok / (now - win_t) if now > win_t else None
        if step > 1 and tok_s:
            rates.append(tok_s)
        eta = min(elapsed / step * (max_steps - step), max(0.0, deadline - now))
        ctx.log.metric(kind="train", step=step, max_steps=max_steps, epoch=rnd(step / float(spe), 3),
                       loss=rnd(sum(win_loss) / len(win_loss)), lr=rnd(lr_now, 8), grad_norm=rnd(gn),
                       tok_s=rnd(tok_s, 1), elapsed_s=int(elapsed), eta_s=int(eta), vram_gb=vram_gb(torch))

    for epoch in range(h["epochs"]):
        batches = plan_batches(lengths, pdb, h["seed"] + epoch)
        for g in range(0, len(batches), ga):
            group = batches[g:g + ga]
            if time.monotonic() >= deadline:
                stop_reason = "time_budget"
                break
            targets = sum(label_targets(examples, b) for b in group)
            if targets == 0:
                continue
            lr_now = h["learning_rate"] * lr_factor(step, warmup, max_steps)
            for pg in opt.param_groups:
                pg["lr"] = lr_now
            opt.zero_grad(set_to_none=True)
            loss_sum, tokens, oom = 0.0, 0, False
            try:
                for b in group:
                    ids, mask, labels = to_tensors(ctx, examples, b)
                    s, n = ctx.loss.sum_loss(ids, mask, labels, grad=True)
                    if s is None:
                        continue
                    (s / targets).backward()
                    loss_sum += float(s.detach().item())
                    tokens += int(mask.sum().item())
            except Exception as e:
                if not is_oom(e):
                    raise
                oom = True
            if oom:
                opt.zero_grad(set_to_none=True)
                release(ctx)
                oom_skips += 1
                ctx.warn("oom_step_skipped", "step %d ran out of memory (width %d); skipped"
                         % (step + 1, max(lengths[i] for b in group for i in b)))
                if oom_skips > 3:
                    raise StageError("oom", "training ran out of memory %d times; choose preset quick or a lower "
                                     "max_seq_len" % oom_skips)
                continue
            loss = loss_sum / targets
            if not math.isfinite(loss):
                raise StageError("train_diverged", "loss became %s at step %d; lower the learning rate" % (loss, step + 1))
            gn = float(torch.nn.utils.clip_grad_norm_(params, 1.0))
            if not math.isfinite(gn):
                raise StageError("train_diverged", "gradient norm became %s at step %d" % (gn, step + 1))
            opt.step()
            step += 1
            win_loss.append(loss)
            win_tok += tokens
            if step == 1 or step % every == 0 or step == max_steps:
                now = time.monotonic()
                emit(now)
                last_loss = sum(win_loss) / len(win_loss)
                win_loss, win_tok, win_t = [], 0, now
        if stop_reason == "time_budget":
            break
        if ctx.examples.get("val") and time.monotonic() < deadline:
            vl = eval_loss(ctx, model, ctx.examples["val"], pdb)
            ctx.log.metric(kind="eval", step=step, max_steps=max_steps, epoch=epoch + 1, loss=rnd(vl), eval_loss=rnd(vl),
                           elapsed_s=int(time.monotonic() - t_train), vram_gb=vram_gb(torch))
            model.train()
    if win_loss and step:
        emit(time.monotonic())
        last_loss = sum(win_loss) / len(win_loss)
    if step == 0:
        raise StageError("time_budget", "no training step finished within the time budget")
    ctx.status["train"].update(steps=step, epochs=rnd(step / float(spe), 3), train_loss=rnd(last_loss),
                               stop_reason=stop_reason)
    ctx.discovery.update(train_tok_s=rnd(sorted(rates)[len(rates) // 2], 1) if rates else None, oom_skips=oom_skips,
                         train_seconds=rnd(time.monotonic() - t_train, 1))
    if stop_reason == "time_budget":
        ctx.warn("time_budget", "training stopped at step %d of %d by the time budget" % (step, max_steps), primary=True)
    model.eval()


def copy_tokenizer_files(src, dst):
    """D8: the vendor's tokenizer files from the base snapshot, not re-saved ones."""
    copied = []
    for name in sorted(os.listdir(src)):
        if name in TOKENIZER_FILES or (name.startswith("tokenizer") and not name.endswith(".safetensors")):
            path = os.path.join(src, name)
            if os.path.isfile(path):
                shutil.copyfile(path, os.path.join(dst, name))
                copied.append(name)
    gen = os.path.join(src, "generation_config.json")
    if os.path.isfile(gen) and not os.path.isfile(os.path.join(dst, "generation_config.json")):
        shutil.copyfile(gen, os.path.join(dst, "generation_config.json"))
        copied.append("generation_config.json")
    return copied


def fake_save(ctx):
    if ctx.job["method"] != "lora":
        return
    lora = ctx.job["hyper"]["lora"]
    write_safetensors_empty(ctx.path("adapter"), {"format": "pt", "chibi": "dry-run"})
    write_json_atomic(ctx.path("adapter_config"), {
        "peft_type": "LORA", "task_type": "CAUSAL_LM", "r": lora["r"], "lora_alpha": lora["alpha"],
        "lora_dropout": lora.get("dropout", 0.0), "target_modules": ctx.discovery["lora_targets"]["resolved"],
        "base_model_name_or_path": ctx.profile["base"]["repo"]})


def stage_save(ctx):
    if ctx.fake:
        fake_save(ctx)
        return
    model = ctx.model
    shutil.rmtree(ctx.merged_dir, ignore_errors=True)
    if ctx.job["method"] == "lora":
        adir = os.path.join(ctx.work, "adapter")
        shutil.rmtree(adir, ignore_errors=True)
        model.save_pretrained(adir)
        for src, kind in (("adapter_model.safetensors", "adapter"), ("adapter_config.json", "adapter_config")):
            path = os.path.join(adir, src)
            if not os.path.isfile(path):
                raise StageError("merge_failed", "peft wrote no %s (files: %s)" % (src, ", ".join(sorted(os.listdir(adir)))))
            shutil.copyfile(path, ctx.path(kind))
        t0 = time.monotonic()
        model = model.merge_and_unload()
        ctx.log.info("merged", seconds=rnd(time.monotonic() - t0, 1), model_class=type(model).__name__)
    set_use_cache(model, True)
    model = model.to("cpu")
    ctx.model = None
    release(ctx)
    try:
        model.save_pretrained(ctx.merged_dir, safe_serialization=True, max_shard_size="2GB")
    except TypeError:
        model.save_pretrained(ctx.merged_dir)
    if not any(n.endswith(".safetensors") for n in os.listdir(ctx.merged_dir)):
        raise StageError("merge_failed", "save_pretrained wrote no safetensors file")
    copied = copy_tokenizer_files(ctx.snapshot_dir, ctx.merged_dir)
    cfg = read_json(os.path.join(ctx.merged_dir, "config.json"))
    keys = model_dir_keys(ctx.merged_dir)
    info = {"architectures": cfg.get("architectures"), "model_type": cfg.get("model_type"),
            "dtype": cfg.get("dtype", cfg.get("torch_dtype")), "tensors": len(keys), "bytes": dir_bytes(ctx.merged_dir),
            "mtp_tensors": sum(1 for k in keys if "mtp" in k), "tokenizer_files": copied}
    ctx.discovery["merged"] = info
    ctx.log.info("merged_model", **info)
    del model
    release(ctx)


def stage_gguf(ctx):
    dst = ctx.path("gguf_tuned")
    if ctx.fake:
        write_fake_gguf(dst, ctx.profile["gguf"]["expected_arch"])
    else:
        convert(ctx, ctx.merged_dir, dst, "tuned")
    verify_gguf(ctx, dst, "tuned")
    shutil.rmtree(ctx.merged_dir, ignore_errors=True)


def stage_manifest(ctx):
    names = output_names(ctx.outtype)
    need = ["gguf_tuned", "metrics", "eval_samples"] + (["adapter", "adapter_config"] if ctx.job["method"] == "lora" else [])
    missing = [names[k] for k in need if not os.path.isfile(os.path.join(ctx.out, names[k]))]
    if missing:
        raise StageError("output_failed", "missing outputs: " + ", ".join(missing))
    for kind in MANIFEST_KINDS:
        path = os.path.join(ctx.out, names[kind])
        if os.path.isfile(path):
            ctx.log.info("output", kind_name=kind, path=names[kind], bytes=os.path.getsize(path))


STAGES = (
    # name          failure code for unexpected exceptions   function
    ("boot",        "job_invalid",                           stage_boot),
    ("env_check",   "env_no_gpu",                            stage_env_check),
    ("data",        "data_mismatch",                         stage_data),
    ("install",     "install_failed",                        stage_install),
    ("load_base",   "base_load_failed",                      stage_load_base),
    ("converter",   "gguf_failed",                           stage_converter),
    ("base_eval",   "eval_failed",                           stage_base_eval),
    ("train",       "train_failed",                          stage_train),
    ("tuned_eval",  "eval_failed",                           stage_tuned_eval),
    ("save",        "merge_failed",                          stage_save),
    ("gguf",        "gguf_failed",                           stage_gguf),
    ("manifest",    "output_failed",                         stage_manifest),
)


def run_stage(ctx, name, code, fn):
    ctx.status["stage"] = name
    ctx.log.emit("stage", stage=name, state="start", elapsed_s=int(ctx.budget.elapsed()),
                 remaining_s=int(ctx.budget.remaining()) if ctx.job else None)
    t0, failure = time.monotonic(), None
    try:
        fn(ctx)
    except StageError as e:
        failure = ("oom" if is_oom(e.__cause__ or e.__context__) else e.code, e.message, None)
    except Interrupted as e:
        failure = ("interrupted", "the session was stopped (%s)" % e, None)
    except Exception as e:
        failure = ("oom" if is_oom(e) else code, exc_summary(e), trace_tail(e))
        traceback.print_exc()
    secs = round(time.monotonic() - t0, 1)
    ctx.discovery["stage_seconds"][name] = secs
    note_vram(ctx)
    if not ctx.fake:
        release(ctx)
    if failure is None:
        ctx.log.emit("stage", stage=name, state="done", duration_s=secs)
        return
    fcode, message, trace = failure
    ctx.log.emit("stage", stage=name, state="failed", duration_s=secs)
    ctx.log.emit("error", stage=name, code=fcode, message=message, trace=trace)
    ctx.status.update(state="failed", code=fcode, message=message)
    raise RunFailed()


def finalize(ctx):
    st = ctx.status
    try:
        if ctx.fake and heavy_loaded() and st["state"] == "succeeded":
            st.update(state="failed", code="internal", message="the dry run imported " + ", ".join(heavy_loaded()))
        if st["state"] == "running":
            st.update(state="failed", code=st["code"] or "internal", message=st["message"] or "the run ended without a result")
        st["finished"] = utc_now()
        st["discovery"]["total_s"] = round(time.monotonic() - ctx.t0, 1)
        note_vram(ctx)
        write_json_atomic(ctx.path("status"), st)
        files = write_manifest(ctx.out, ctx.outtype, st["run_id"], ctx.profile.get("id", ""),
                               (ctx.job or {}).get("profile_sha", ""))
        ctx.log.emit("done", state=st["state"], code=st["code"], message=st["message"], warning_code=st["warning_code"],
                     files=len(files), duration_s=st["discovery"]["total_s"])
    except BaseException as e:
        ctx.log.emit("error", stage="manifest", code="output_failed", message=exc_summary(e))
        traceback.print_exc()
    finally:
        if ctx.fake:
            shutil.rmtree(ctx.work, ignore_errors=True)


def parse_args(argv):
    p = argparse.ArgumentParser(description="AuraGo Chibi Sensei trainer")
    p.add_argument("--job")
    p.add_argument("--out")
    p.add_argument("--input", default=DEFAULT_INPUT)
    p.add_argument("--selftest", action="store_true")
    p.add_argument("--dry-run", dest="dry_run", action="store_true")
    return p.parse_args(argv)


def main(argv=None):
    harden_stdio()
    args = parse_args(argv)
    if args.selftest:
        return selftest()
    removed, run_type = scrub_environment()
    log = Log()
    if not args.job or not args.out:
        log.emit("error", stage="boot", code="job_invalid", message="--job and --out are required")
        return 2
    if args.dry_run and os.environ.get("CHIBI_FAKE_ENV") != "1":
        log.emit("error", stage="boot", code="job_invalid", message="--dry-run runs only with CHIBI_FAKE_ENV=1")
        return 2
    ctx = Ctx(args, log, args.dry_run)
    ctx.discovery.update(env_removed=removed, kaggle_run_type=run_type)
    install_signal_handlers()
    rc = 1
    try:
        for name, code, fn in STAGES:
            run_stage(ctx, name, code, fn)
        ctx.status["state"] = "succeeded"
        rc = 0
    except RunFailed:
        pass
    except Interrupted as e:
        ctx.status.update(state="failed", code="interrupted", message="the session was stopped (%s)" % e)
    except BaseException as e:
        ctx.status.update(state="failed", code="internal", message=exc_summary(e))
        traceback.print_exc()
    finally:
        finalize(ctx)
    return rc


_DELETE = object()


def expect(cond, detail):
    if not cond:
        raise AssertionError(detail)


def expect_stage_error(fn, code, text):
    try:
        fn()
    except StageError as e:
        expect(e.code == code and text in e.message, "got %s: %s" % (e.code, e.message))
        return
    raise AssertionError("no StageError %s (%s)" % (code, text))


def jset(obj, path, value):
    parts = path.split(".")
    for part in parts[:-1]:
        obj = obj[part]
    if value is _DELETE:
        obj.pop(parts[-1], None)
    else:
        obj[parts[-1]] = value


def sample_job():
    profile = {"id": "stub-1", "chat": {"system_role": "native", "stop": ["<|im_end|>"]},
               "base": {"repo": "org/model", "revision": "a" * 40, "auto_class": "AutoModelForCausalLM",
                        "auto_class_fallbacks": ["AutoModelForMultimodalLM"]},
               "gguf": {"expected_arch": "stub", "estimated_mb": 10}, "arch": {"params_b": 0.1},
               "training": {"min_gpu_capability": "7.0", "min_vram_gb": 8, "t4_gen_tokens_per_second": 30,
                            "extra_pip": [], "lora": {"target_modules": ["q_proj"]}}}
    return {"v": 1, "run_id": "11111111-2222-4333-8444-555555555555", "created": "2026-11-23T00:00:00Z",
            "aurago_version": "test", "profile": profile, "profile_sha": "b" * 64, "recipe": "assistant",
            "task_kind": None, "labels": [], "method": "lora",
            "data": {"marker": "r111111112222-marker.json", "format": "messages"},
            "hyper": {"epochs": 1, "learning_rate": 0.0002, "lr_scheduler": "cosine", "warmup_ratio": 0.03,
                      "weight_decay": 0, "per_device_batch": 2, "grad_accum": 2, "max_seq_len": 256, "seed": 42,
                      "gradient_checkpointing": True, "loss_on": "assistant",
                      "lora": {"r": 8, "alpha": 16, "dropout": 0.05, "target_modules": ["q_proj"]}},
            "budget": {"session_timeout_s": 3600, "max_train_s": 1800, "eval_max_samples": 10, "max_new_tokens": 32},
            "outputs": {"emit_base_gguf": True, "gguf_outtype": "q8_0", "prefix": "chibi-"},
            "deps": {"pip": None, "transformers": "",
                     "llama_cpp": {"tag": "b9994", "url": "https://codeload.github.com/ggml-org/llama.cpp/tar.gz/refs/tags/b9994", "sha256": ""}},
            "log": {"prefix": "@@chibi", "metric_every_steps": 10}}


def st_log():
    buf = io.StringIO()
    log = Log(stream=buf)
    log.emit("info", kind="x", message="\u00e4" * 5000, value=float("nan"))
    log.emit("warn", code="c", message="m")
    lines = buf.getvalue().splitlines()
    expect(len(lines) == 2, "two lines expected, got %d" % len(lines))
    for n, line in enumerate(lines, 1):
        expect(line.startswith(LOG_PREFIX + " {") and len(line.encode("utf-8")) <= MAX_LINE_BYTES, "line %d shape" % n)
        obj = json.loads(line[len(LOG_PREFIX) + 1:])
        expect(obj["v"] == 1 and obj["seq"] == n and obj["t"] and obj["ts"].endswith("Z"), "line %d fields" % n)
    first = json.loads(lines[0][len(LOG_PREFIX) + 1:])
    expect(first.get("cut") is True and first.get("value") is None, "long line must be cut and NaN must be null")
    expect(all(ord(c) < 128 for c in buf.getvalue()), "log lines must be ASCII")


def st_job():
    expect(validate_job(sample_job()) == [], "sample job: %r" % validate_job(sample_job()))
    cases = (("v", 2, "v:"), ("run_id", _DELETE, "run_id:"), ("method", "qlora", "method:"),
             ("data.format", "text", "data.format:"), ("data.marker", "../x.json", "data.marker:"),
             ("hyper.epochs", 0, "hyper.epochs:"), ("hyper.lora", None, "hyper.lora:"),
             ("budget.max_train_s", 4000, "budget:"), ("outputs.gguf_outtype", "Q8 0", "outputs.gguf_outtype:"),
             ("hyper.learning_rate", True, "hyper.learning_rate:"), ("deps.pip", "peft", "deps.pip:"))
    for path, value, prefix in cases:
        job = sample_job()
        jset(job, path, value)
        problems = validate_job(job)
        expect(any(p.startswith(prefix) for p in problems), "%s=%r gave %r" % (path, value, problems))
    job = sample_job()
    job["method"], job["hyper"]["lora"] = "full", None
    expect(validate_job(job) == [], "full without lora: %r" % validate_job(job))


def st_marker():
    with tempfile.TemporaryDirectory() as root:
        d = os.path.join(root, "ds", "v1")
        os.makedirs(d)
        train = os.path.join(d, "r1-train.jsonl")
        write_jsonl_atomic(train, [{"messages": [{"role": "user", "content": "hi %d" % i},
                                                 {"role": "assistant", "content": "yo"}]} for i in range(3)])
        write_json_atomic(os.path.join(d, "r1-marker.json"), {"v": 1, "run_id": "run", "files": {
            "train": {"name": "r1-train.jsonl", "sha256": sha256_file(train), "rows": 3}}})
        marker, base = load_marker(root, "r1-marker.json", "run")
        files = verify_split_files(marker, base, root)
        expect(list(files) == ["train"], "splits: %r" % list(files))
        rows = parse_rows(files["train"][0], "messages", 3, "train")
        expect(len(rows) == 3 and rows[0]["_line"] == 1 and sample_id(rows[0]) == "eval-1", "rows")
        expect_stage_error(lambda: load_marker(root, "r1-marker.json", "other"), "data_mismatch", "belongs to run")
        expect_stage_error(lambda: load_marker(root, "missing.json", "run"), "data_mismatch", "not found")
        with open(train, "a", encoding="utf-8") as f:
            f.write("\n")
        expect_stage_error(lambda: verify_split_files(marker, base, root), "data_mismatch", "sha256")
        bad = os.path.join(d, "bad.jsonl")
        with open(bad, "w", encoding="utf-8") as f:
            f.write('{"messages":[{"role":"user","content":"x"}]}\n')
        expect_stage_error(lambda: parse_rows(bad, "messages", 1, "train"), "data_mismatch", "line 1")
        deep = os.path.join(root, "a", "b", "c", "d", "e")
        os.makedirs(deep)
        write_json_atomic(os.path.join(deep, "deep.json"), {})
        expect(find_files(root, {"deep.json"})[0] == {}, "a marker below depth 4 must not be found")


def st_masking():
    tok = StubTokenizer()
    msgs = [{"role": "system", "content": "S"}, {"role": "user", "content": "hi"}, {"role": "assistant", "content": "yo"}]
    ids, labels, cut = build_example(tok, {"messages": msgs}, "messages", "native", "<|im_end|>", 512)
    prompt = encode(tok, tok.apply_chat_template(msgs[:-1], add_generation_prompt=True))
    expect(ids[:len(prompt)] == prompt and labels[:len(prompt)] == [-100] * len(prompt), "prompt must be masked")
    answer = tok.decode([y for y in labels if y != -100])
    expect(answer == "yo<|im_end|>", "answer labels: %r" % answer)
    expect(ids.count(tok.bos_token_id) == 1 and not cut, "exactly one BOS, no truncation")
    merged = tok.decode(build_example(tok, {"messages": msgs}, "messages", "merge_into_first_user", "<|im_end|>", 512)[0])
    expect("S\n\nhi" in merged and "system" not in merged, "merge: %r" % merged)
    short, slab, cut = build_example(tok, {"messages": msgs}, "messages", "native", "<|im_end|>", 8)
    expect(cut and len(short) == 8 and short[0] == ids[0] and short[1:] == ids[-7:] and slab[1:] == labels[-7:],
           "truncation keeps the first and the last tokens")
    t_ids, t_labels, _ = build_example(tok, {"text": "abc"}, "text", "native", "", 512)
    expect(t_ids == [1] + encode(tok, "abc") + [2] and t_labels == t_ids, "text rows: bos + text + eos, all labelled")
    expect(drop_double_bos(tok, [1, 1, 7]) == [1, 7] and drop_double_bos(tok, [1, 7]) == [1, 7], "double BOS")
    expect(merge_system([{"role": "system", "content": "S"}, {"role": "assistant", "content": "a"}])[0]
           == {"role": "user", "content": "S"}, "system without user")


def st_batching():
    lengths = [5, 1, 9, 3, 7, 2, 8]
    batches = plan_batches(lengths, 3, 42)
    expect(sorted(i for b in batches for i in b) == list(range(7)), "every row once")
    order = sorted(range(7), key=lambda i: (lengths[i], i))
    expect(sorted(map(tuple, batches)) == sorted(tuple(order[i:i + 3]) for i in range(0, 7, 3)), "length buckets")
    ex = [([1, 2, 3], [-100, 2, 3], False), ([4], [4], False)]
    ids, mask, labels = pad_batch(ex, [0, 1], 0)
    expect(len(ids[0]) == 8 and mask[1] == [1] + [0] * 7 and labels[1] == [4] + [-100] * 7 and ids[1][1:] == [0] * 7,
           "dynamic right padding to a multiple of 8")
    expect(label_targets(ex, [0, 1]) == 2 and steps_per_epoch(7, 2) == 4, "targets and steps")
    expect(abs(lr_factor(0, 2, 10) - 0.5) < 1e-9 and abs(lr_factor(2, 2, 10) - 1.0) < 1e-9
           and lr_factor(10, 2, 10) < 1e-9, "warmup and cosine schedule")


def st_targets():
    leaves = {"q_proj": 4, "k_proj": 4, "out_proj": 6, "w1": 6}
    expect(resolve_targets(["q_proj", "o_proj", "w1"], leaves) == (["q_proj", "w1"], ["o_proj"], False), "intersection")
    expect(resolve_targets(["gate_proj"], leaves) == (sorted(leaves), ["gate_proj"], True), "fallback to every leaf")
    rx = lora_regex(["q_proj", "w1"])
    expect(re.fullmatch(rx, "model.layers.0.self_attn.q_proj") and re.fullmatch(rx, "model.layers.3.feed_forward.w1"),
           "text modules match")
    for name in ("model.visual.blocks.0.attn.q_proj", "model.layers.0.self_attn.q_proj_x", "lm_head"):
        expect(not re.fullmatch(rx, name), "%s must not match" % name)


def st_outputs():
    with tempfile.TemporaryDirectory() as d:
        names = output_names("q8_0")
        write_fake_gguf(os.path.join(d, names["gguf_tuned"]), "gemma3")
        hdr = read_gguf_header(os.path.join(d, names["gguf_tuned"]))
        expect(hdr == {"version": 3, "tensors": 1, "kv": 2, "arch": "gemma3"}, "gguf header: %r" % hdr)
        write_safetensors_empty(os.path.join(d, names["adapter"]), {"format": "pt"})
        expect(safetensors_keys(os.path.join(d, names["adapter"])) == [], "empty safetensors")
        write_json_atomic(os.path.join(d, names["status"]), {"v": 1, "x": float("nan")})
        expect(read_json(os.path.join(d, names["status"])) == {"v": 1, "x": None}, "status NaN becomes null")
        open(os.path.join(d, names["metrics"]), "w").close()
        open(os.path.join(d, "chibi-other.txt"), "w").close()
        write_manifest(d, "q8_0", "run", "prof", "sha")
        man = read_json(os.path.join(d, MANIFEST_NAME))
        expect([f["kind"] for f in man["files"]] == ["gguf_tuned", "adapter", "metrics", "status"], "kinds: %r" % man)
        for f in man["files"]:
            expect(f["sha256"] == sha256_file(os.path.join(d, f["path"])) and safe_name(f["path"])
                   and f["path"].startswith("chibi-"), "manifest entry %r" % f)
        expect(man["v"] == 1 and man["run_id"] == "run" and not any(n.startswith(".tmp-") for n in os.listdir(d)),
               "manifest header and no temp files")


def st_requirements():
    with tempfile.TemporaryDirectory() as d:
        with open(os.path.join(d, "a.txt"), "w", encoding="utf-8") as f:
            f.write('-r ./b.txt\n--extra-index-url https://download.pytorch.org/whl/cpu\ntorch==2.11.0; platform_machine != "s390x"\n')
        with open(os.path.join(d, "b.txt"), "w", encoding="utf-8") as f:
            f.write("numpy~=1.26.4\ngguf>=0.1.0\ntransformers==4.57.6\n")
        lines = flatten_requirements(os.path.join(d, "a.txt"))
        expect(lines[0] == "numpy~=1.26.4" and len(lines) == 5, "flatten: %r" % lines)
        new, relaxed = adapt_requirements(lines, (3, 13))
        expect("numpy>=2.1,<3" in new and not any(x.startswith("gguf") for x in new) and len(relaxed) == 2, "3.13: %r" % new)
        new, _ = adapt_requirements(lines, (3, 12))
        expect("numpy~=1.26.4" in new, "3.12 keeps numpy")


def st_budget():
    now = [100.0]
    b = Budget(3600, 1800, clock=lambda: now[0])
    now[0] = 200.0
    expect(abs(b.remaining() - (3600 - SESSION_LEAD_S - 100)) < 1e-9, "remaining")
    expect(abs(b.train_deadline(600) - 2000.0) < 1e-9, "deadline by max_train_s")
    expect(abs(b.train_deadline(2000) - (100 + 3600 - SESSION_LEAD_S - 2000)) < 1e-9, "deadline by session")
    expect(generation_plan(10, 64, 32.0, 1000.0) == (15.0, 90.0), "generation caps")
    expect(generation_plan(10, 64, 32.0, 30.0)[1] == 30.0, "generation capped by available time")


def st_answers():
    tok = StubTokenizer()
    expect(decode_answer(tok, encode(tok, "yes<|im_end|>more"), [4], ["<|im_end|>"]) == ("yes", True), "stop id")
    expect(decode_answer(tok, encode(tok, "abc"), [4], ["<|im_end|>"]) == ("abc", False), "no stop")
    expect(stop_token_ids(tok, ["<|im_end|>", "##"], [99]) == [2, 4, 99], "stop ids")
    expect(split_text_prompt("one two three four") == ("one two", " three four"), "text prompt split")


def st_samples():
    class Holder:
        pass
    ctx = Holder()
    ctx.tok, ctx.job = StubTokenizer(), sample_job()
    # max_new_tokens as large as max_seq_len (the assistant recipe): the question must still reach the model.
    ctx.job["budget"]["max_new_tokens"] = 256
    row = {"_line": 1, "messages": [{"role": "user", "content": "what is in the amber fizz today"},
                                    {"role": "assistant", "content": "tea"}]}
    ctx.rows = {"eval": [row]}
    full = chat_prompt_ids(ctx.tok, row["messages"][:-1], "native")
    sample = make_samples(ctx, "messages", "native", 256)[0]
    expect(len(full) > 8 and sample["_prompt_ids"] == full,
           "the eval prompt is kept whole: %d of %d tokens" % (len(sample["_prompt_ids"]), len(full)))
    expect(sample["id"] == "eval-1" and sample["base_output"] is None, "sample fields")
    short = make_samples(ctx, "messages", "native", 16)[0]["_prompt_ids"]
    expect(len(short) == 16 and short[0] == full[0] and short[1:] == full[-15:], "the eval prompt is cut at max_seq_len")


SELFTESTS = (("log encoder", st_log), ("job schema", st_job), ("marker and sha256", st_marker),
             ("masking", st_masking), ("batching", st_batching), ("lora targets", st_targets),
             ("status and manifest", st_outputs), ("converter requirements", st_requirements),
             ("budget", st_budget), ("answers", st_answers), ("eval samples", st_samples))


def selftest():
    for name, fn in SELFTESTS:
        try:
            fn()
        except Exception as e:
            print("selftest FAILED: %s: %s" % (name, exc_summary(e)))
            traceback.print_exc()
            return 1
    if heavy_loaded():
        print("selftest FAILED: imports: heavy modules loaded: %s" % ", ".join(heavy_loaded()))
        return 1
    print("selftest ok (%d checks)" % len(SELFTESTS))
    return 0


if __name__ == "__main__":
    sys.exit(main())
