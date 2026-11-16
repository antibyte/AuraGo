"""Fixed KiCad 10 export protocol. No arbitrary commands or source writes."""
import hashlib
import json
import os
from pathlib import Path
import re
import resource
import subprocess
import zipfile

VERSION = "10.0.8"
JOB = Path("/job")


def run(args):
    result = subprocess.run(["kicad-cli", *args], stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT, timeout=180, check=False)
    if result.returncode:
        raise ValueError("KiCad rejected the saved project (incompatible format or missing library): " +
                         result.stdout.decode("utf-8", errors="replace")[-3000:])
    return result.stdout.decode("utf-8", errors="replace").strip()


def check_references(source):
    quoted = r'"(?:\\.|[^"\\])*"'
    for file in source.rglob("*"):
        if not file.is_file() or file.suffix not in (".kicad_sch", ""):
            continue
        if file.suffix == "" and file.name not in ("sym-lib-table", "fp-lib-table"):
            continue
        text = file.read_text(encoding="utf-8")
        refs = re.findall(r'\(uri\s+(' + quoted + ')', text)
        refs += re.findall(r'\(property\s+"Sheetfile"\s+(' + quoted + ')', text, re.I)
        for value in refs:
            value = json.loads(value)
            value = value.replace("${KIPRJMOD}", str(source))
            value = value.replace("${KICAD10_SYMBOL_DIR}", "/usr/share/kicad/symbols")
            value = value.replace("${KICAD10_FOOTPRINT_DIR}", "/usr/share/kicad/footprints")
            target = Path(value)
            if not target.is_absolute():
                target = file.parent / target
            target = target.resolve()
            if (not target.is_relative_to(source) and
                    not target.is_relative_to(Path("/usr/share/kicad"))):
                raise ValueError("Library or sheet must be relative to the saved project: " + file.name)
            if not target.exists():
                raise ValueError("Missing project library or sheet: " + str(target).replace(str(source), "."))


def export(request):
    version = run(["version"])
    if version != VERSION:
        raise ValueError("Incompatible KiCad version: " + version)
    if request["operation"] == "probe":
        for args in (["sch", "export", "netlist", "--help"],
                     ["pcb", "export", "gerbers", "--help"], ["pcb", "export", "drill", "--help"]):
            run(args)
        (JOB / "probe.json").write_text(json.dumps({"version": version, "protocol": 1}))
        return
    source = (JOB / "source").resolve()
    name = request["input"]
    if Path(name).name != name or name in ("", ".", ".."):
        raise ValueError("Invalid input filename")
    file = source / name
    if not file.is_file() or file.is_symlink():
        raise ValueError("Saved input file is missing")
    check_references(source)
    limit = request["max_bytes"]
    if not isinstance(limit, int) or not 0 < limit <= 64 * 1024 * 1024:
        raise ValueError("Invalid output limit")
    if request["operation"] == "netlist":
        if file.suffix != ".kicad_sch":
            raise ValueError("Netlist requires .kicad_sch")
        run(["sch", "export", "netlist", "--format", "kicadsexpr", "--output", str(JOB / "result.net"), str(file)])
        result = JOB / "result.net"
        if not 0 < result.stat().st_size <= limit or not result.read_bytes().lstrip().startswith(b"(export"):
            raise ValueError("Invalid or oversized KiCad netlist")
    elif request["operation"] == "manufacturing":
        if file.suffix != ".kicad_pcb":
            raise ValueError("Manufacturing export requires .kicad_pcb")
        output = JOB / "output"
        output.mkdir()
        run(["pcb", "export", "gerbers", "--no-protel-ext", "--check-zones", "--output", str(output) + "/", str(file)])
        run(["pcb", "export", "drill", "--format", "excellon", "--excellon-units", "mm", "--output", str(output) + "/", str(file)])
        files = sorted(output.iterdir())
        if not any(f.suffix == ".gbr" for f in files) or not any(f.suffix == ".drl" for f in files):
            raise ValueError("KiCad did not produce Gerber and Excellon files")
        if len(files) > 128 or any(not f.is_file() or f.is_symlink() for f in files) or sum(f.stat().st_size for f in files) > limit:
            raise ValueError("Manufacturing output exceeds limits")
        manifest = {"protocol": 1, "kicad": version, "source": name,
                    "formats": ["Gerber-X2", "Excellon"],
                    "sources": {str(f.relative_to(source)): hashlib.sha256(f.read_bytes()).hexdigest()
                                for f in sorted(source.rglob("*")) if f.is_file()},
                    "files": {f.name: hashlib.sha256(f.read_bytes()).hexdigest() for f in files}}
        with zipfile.ZipFile(JOB / "result.zip", "w", zipfile.ZIP_DEFLATED) as archive:
            for f in files:
                archive.write(f, f.name)
            archive.writestr("export-manifest.json", json.dumps(manifest, indent=2, sort_keys=True))
        if (JOB / "result.zip").stat().st_size > limit:
            raise ValueError("Manufacturing archive exceeds limits")
    else:
        raise ValueError("Unsupported export operation")


if __name__ == "__main__":
    resource.setrlimit(resource.RLIMIT_FSIZE, (64 * 1024 * 1024, 64 * 1024 * 1024))
    resource.setrlimit(resource.RLIMIT_CPU, (240, 240))
    try:
        export(json.loads((JOB / "request.json").read_text(encoding="utf-8")))
    except Exception as error:
        (JOB / "error.txt").write_bytes(str(error).encode("utf-8")[:4000])
        raise SystemExit(1)
