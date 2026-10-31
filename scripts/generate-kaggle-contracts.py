#!/usr/bin/env python3
"""Generate native wire contracts from a hash-pinned SDK; never execute SDK code."""
import argparse
import ast
import hashlib
import io
import json
import pathlib
import re
import subprocess
import urllib.request
import zipfile

VERSION = "0.1.38"
SHA256 = "ec9cb3cb2acaa92106f7c0334b7374e022bd6e6d1176a6780fec5606f6bc568d"
ROOT = pathlib.Path(__file__).resolve().parents[1]
# Explicit public surface: never infer permission from HTTP verb or a new SDK method.
SERVICES = {
    "datasets/dataset_api": ("datasets", "ListDatasets GetDataset ListDatasetFiles GetDatasetMetadata UpdateDatasetMetadata DownloadDataset CreateDatasetVersion CreateDataset GetDatasetStatus GetDatasetFilesSummary UploadDatasetFile DeleteDataset"),
    "models/model_api": ("models", "CreateModel CreateModelInstance CreateModelInstanceVersion DeleteModel DeleteModelInstance DeleteModelInstanceVersion GetModel GetModelInstance DownloadModelInstanceVersion ListModels ListModelInstances ListModelInstanceVersions ListModelInstanceVersionFiles UpdateModel UpdateModelInstance UploadModelFile"),
    "kernels/kernels_api": ("notebooks", "ListKernels ListKernelFiles GetKernel SaveKernel ListKernelSessionOutput GetKernelSessionStatus DownloadKernelOutput DownloadKernelOutputZip DeleteKernel CancelKernelSession GetKernelSessionLogsStream GetAcceleratorQuotaStatistics"),
    "competitions/competition_api": ("competitions", "ListCompetitions ListSubmissions ListTeamPublicSubmissions ListDataFiles GetLeaderboard DownloadLeaderboard CreateSubmission CreateCodeSubmission GetSubmission DownloadSubmission GetSubmissionLimits StartSubmissionUpload DownloadDataFiles DownloadDataFile GetCompetition GetCompetitionDataFilesSummary ListCompetitionPages ListCompetitionTopics ListTopicMessages"),
    "benchmarks/benchmark_tasks_api": ("benchmarks", "CreateBenchmarkTask BatchScheduleBenchmarkTaskRuns ApiCancelBenchmarkTaskRuns ListBenchmarkTasks GetBenchmarkTask ListBenchmarkTaskRuns DownloadBenchmarkTaskRunOutput GetBenchmarkTaskRunLogs GetBenchmarkTaskQuota ApiGetBenchmarkSchedulerStatus PublishBenchmarkTask UpdateBenchmarkTask UpdateBenchmarkTaskVersion"),
    "benchmarks/benchmarks_api": ("benchmarks", "GetBenchmarkLeaderboard ListBenchmarkModels"),
    "discussions/discussions_api": ("forums", "ListForums ListTopics GetTopic ListComments ListDatasetTopics ListModelTopics ListBenchmarkTopics ListKernelTopics"),
    "security/oauth": ("internal", "IntrospectToken"),
}

def generate(wheel):
    z = zipfile.ZipFile(io.BytesIO(wheel))
    fields, enums, methods = {}, {}, []
    for path in z.namelist():
        if not path.endswith(".py") or "/types/" not in path:
            continue
        tree = ast.parse(z.read(path))
        for node in tree.body:
            if isinstance(node, ast.Assign) and len(node.targets) == 1:
                target = node.targets[0]
                if isinstance(target, ast.Attribute) and target.attr == "_fields":
                    fields[ast.unparse(target.value)] = node.value.elts
            if isinstance(node, ast.ClassDef) and any(ast.unparse(b) in ("Enum", "enum.Enum") for b in node.bases):
                enums[node.name] = [n.targets[0].id for n in node.body if isinstance(n, ast.Assign)]

    definitions = {}
    def schema(name):
        primitives = {"str": "string", "int": "integer", "float": "number", "bool": "boolean", "datetime": "string", "bytes": "string", "FieldMask": "string", "timedelta": "string"}
        if name in primitives:
            return {"type": primitives[name]}
        if name in enums:
            return {"type": "string", "enum": enums[name]}
        if name not in definitions:
            definitions[name] = {"type": "object", "properties": {}, "additionalProperties": False}
            if name not in fields:
                raise ValueError("Unknown SDK type " + name)
            for f in fields[name]:
                wire = ast.literal_eval(f.args[0])
                item = schema(ast.unparse(f.args[3]))
                serializer = ast.unparse(f.args[5])
                if serializer.startswith("ListSerializer"):
                    item = {"type": "array", "items": item, "maxItems": 1000}
                if serializer.startswith("DictSerializer"):
                    item = {"type": "object", "additionalProperties": item}
                definitions[name]["properties"][wire] = item
        return {"$ref": "#/$defs/" + name}

    for key, (domain, allow) in SERVICES.items():
        area, stem = key.split("/")
        path = f"kagglesdk/{area}/services/{stem}_service.py"
        tree = ast.parse(z.read(path))
        for node in ast.walk(tree):
            if not isinstance(node, ast.FunctionDef) or node.name == "__init__":
                continue
            for call in ast.walk(node):
                if not isinstance(call, ast.Call) or not isinstance(call.func, ast.Attribute) or call.func.attr != "call":
                    continue
                service, method = [ast.literal_eval(x) for x in call.args[:2]]
                if method not in allow.split():
                    continue
                request = ast.unparse(node.args.args[1].annotation)
                schema(request)
                methods.append({"domain": domain, "operation": node.name, "service": service, "method": method, "request": request, "response": ast.unparse(call.args[3])})
    out = {"sdk": VERSION, "sha256": SHA256, "operations": methods, "$defs": definitions}
    go = ["// Code generated by scripts/generate-kaggle-contracts.py; DO NOT EDIT.",
          "// Wire definitions: Kaggle SDK 0.1.38 (Apache-2.0).", "package kaggle", ""]
    def typename(s):
        if "$ref" in s:
            return "*" + s["$ref"].split("/")[-1].replace(".", "")
        if s["type"] == "array":
            return "[]" + typename(s["items"])
        if s["type"] == "object":
            return "map[string]" + typename(s["additionalProperties"])
        return {"string": "*string", "integer": "*int64", "boolean": "*bool", "number": "*float64"}[s["type"]]
    for name, definition in sorted(definitions.items()):
        go.append("type " + name.replace(".", "") + " struct {")
        for field, s in definition["properties"].items():
            go.append("\t" + field[:1].upper() + field[1:] + " " + typename(s) + ' `json:"' + field + ',omitempty"`')
        go.append("}\n")
    go.append("func newWireRequest(name string) any {\n switch name {")
    for request in sorted(set(m["request"] for m in methods)):
        go.append(f' case "{request}": return &{request}{{}}')
    go.append(" }; return nil\n}\n")
    return json.dumps(out, indent=2, ensure_ascii=False) + "\n", "\n".join(go)

def main():
    p = argparse.ArgumentParser()
    p.add_argument("--wheel", type=pathlib.Path)
    p.add_argument("--check", action="store_true")
    args = p.parse_args()
    if args.wheel:
        wheel = args.wheel.read_bytes()
    else:
        metadata = json.load(urllib.request.urlopen(f"https://pypi.org/pypi/kagglesdk/{VERSION}/json", timeout=30))
        url = next(x["url"] for x in metadata["urls"] if x["filename"].endswith(".whl"))
        wheel = urllib.request.urlopen(url, timeout=60).read()
    if hashlib.sha256(wheel).hexdigest() != SHA256:
        raise SystemExit("SDK digest mismatch")
    content, go = generate(wheel)
    go = subprocess.run(["gofmt"], input=go, text=True, capture_output=True, check=True).stdout
    for filename, value in [("contracts.json", content), ("wire_generated.go", go)]:
        target = ROOT / "internal/kaggle" / filename
        if args.check:
            if target.read_text(encoding="utf-8") != value:
                raise SystemExit("Kaggle contracts are stale: " + filename)
        else:
            target.write_text(value, encoding="utf-8", newline="\n")
    print("Verified Kaggle SDK", VERSION)

if __name__ == "__main__":
    main()
