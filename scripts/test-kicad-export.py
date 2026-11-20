"""KiCad worker contract checks; --image also runs real isolated exports."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import types
import zipfile

ROOT = Path(__file__).resolve().parents[1]
WORKER = ROOT / "deploy/docker/kicad-export/export.py"
SCHEMATIC = '''(kicad_sch (version 20250114) (generator "eeschema")
 (uuid "c4703dd9-88cb-4cb1-9fa5-c75636b419e3") (paper "A4") (lib_symbols)
 (wire (pts (xy 50 50) (xy 75 50)) (stroke (width 0) (type default))
  (uuid "ab372b71-569c-4d57-81b3-317ad8be17b5"))
 (label "TEST_NET" (at 50 50 0) (effects (font (size 1.27 1.27)) (justify left bottom))
  (uuid "c961b381-5443-4d24-94b2-4d3b59b19955"))
 (sheet_instances (path "/" (page "1"))) (embedded_fonts no))'''
BOARD = '''(kicad_pcb (version 20241229) (generator "pcbnew")
 (general (thickness 1.6)) (paper "A4")
 (layers (0 "F.Cu" signal) (2 "B.Cu" signal) (44 "Edge.Cuts" user))
 (setup (pad_to_mask_clearance 0))
 (gr_rect (start 10 10) (end 40 30) (stroke (width 0.1) (type default))
  (fill none) (layer "Edge.Cuts") (uuid "f320a8e2-b30c-4095-b39f-ad986e701361"))
 (footprint "Test:Hole" (layer "F.Cu") (at 20 20)
  (uuid "e2a41b67-e25a-4e01-a206-e1a788777c20")
  (attr through_hole)
  (pad "1" thru_hole circle (at 0 0) (size 2 2) (drill 1) (layers "*.Cu" "*.Mask")
   (uuid "1038373c-4522-4ddc-bffe-cf0a4b640c13"))))'''


def prepare(job, operation):
    source = job / "source"
    source.mkdir()
    (source / "test.kicad_sch").write_text(SCHEMATIC, encoding="utf-8")
    (source / "test.kicad_pcb").write_text(BOARD, encoding="utf-8")
    request = {"operation": operation, "input": "test.kicad_sch" if operation == "netlist" else "test.kicad_pcb", "max_bytes": 1024 * 1024}
    (job / "request.json").write_text(json.dumps(request), encoding="utf-8")
    return request


def check(job, operation):
    if operation == "probe":
        assert json.loads((job / "probe.json").read_text()) == {"version": "10.0.8", "protocol": 1}
    elif operation == "netlist":
        assert (job / "result.net").read_bytes().lstrip().startswith(b"(export")
    else:
        with zipfile.ZipFile(job / "result.zip") as archive:
            manifest = json.loads(archive.read("export-manifest.json"))
            assert manifest["formats"] == ["Gerber-X2", "Excellon"]
            assert any(n.endswith(".gbr") for n in archive.namelist())
            assert any(n.endswith(".drl") for n in archive.namelist())
    assert (job / "source/test.kicad_sch").read_text(encoding="utf-8") == SCHEMATIC


def unit():
    if sys.platform == "win32":
        sys.modules.setdefault("resource", types.ModuleType("resource"))
    spec = importlib.util.spec_from_file_location("kicad_export", WORKER)
    worker = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(worker)
    def fake_run(args):
        if args == ["version"]: return "10.0.8"
        if "--help" in args: return "usage"
        output = Path(args[args.index("--output") + 1])
        if args[:3] == ["sch", "export", "netlist"]:
            assert args[args.index("--format") + 1] == "kicadsexpr"
            output.write_text("(export (version D))")
        elif args[:3] == ["pcb", "export", "gerbers"]:
            assert "--no-x2" not in args and "--check-zones" in args
            (output / "test-F_Cu.gbr").write_text("%TF.GenerationSoftware,KiCad,Pcbnew,10.0.8*%")
        elif args[:3] == ["pcb", "export", "drill"]:
            assert args[args.index("--format") + 1] == "excellon"
            (output / "test.drl").write_text("M48\nMETRIC\nM30")
        else: raise AssertionError(args)
        return ""
    worker.run = fake_run
    for operation in ("probe", "netlist", "manufacturing"):
        with tempfile.TemporaryDirectory() as temp:
            job = Path(temp); worker.JOB = job
            request = prepare(job, operation)
            worker.export(request); check(job, operation)
            (job / "source/sym-lib-table").write_text('(sym_lib_table (lib (name "bad") (uri "../../secret")))')
            try: worker.check_references((job / "source").resolve())
            except ValueError: pass
            else: raise AssertionError("escaping reference accepted")
            (job / "source/sym-lib-table").write_text('(sym_lib_table (lib (name "bad") (uri "missing.kicad_sym")))')
            try: worker.check_references((job / "source").resolve())
            except ValueError: pass
            else: raise AssertionError("missing library accepted")
    print("KiCad worker unit contracts passed")


def live(image, platform):
    # Run as the caller's uid:gid, as internal/desktop/ziro_export.go does. The job
    # directory is mode 0700 and owned by the caller, so root with --cap-drop ALL
    # cannot even read /job/request.json on a Linux runner.
    user = ["--user", f"{os.getuid()}:{os.getgid()}"] if hasattr(os, "getuid") else []
    for operation in ("probe", "netlist", "manufacturing"):
        with tempfile.TemporaryDirectory() as temp:
            job = Path(temp); prepare(job, operation)
            result = subprocess.run(["docker", "run", "--rm", "--platform", platform, *user, "--network", "none", "--read-only",
                                     "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--pids-limit", "128",
                                     "--memory", "2g", "--cpus", "2", "--tmpfs", "/tmp:rw,nosuid,nodev,size=256m",
                                     "-v", str(job) + ":/job", image], timeout=300, check=False)
            if result.returncode:
                raise RuntimeError((job / "error.txt").read_text() if (job / "error.txt").exists() else "Container failed")
            check(job, operation)
    print("KiCad isolated live exports passed on " + platform)


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--image")
    parser.add_argument("--platform", default="linux/amd64")
    args = parser.parse_args()
    unit()
    if args.image: live(args.image, args.platform)
