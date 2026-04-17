import sys
import json
import os
import csv
import io

try:
    import yaml
except ImportError:
    yaml = None

try:
    import xml.etree.ElementTree as ET
except ImportError:
    ET = None

def _parse_xml(raw):
    root = ET.fromstring(raw)
    def elem_to_dict(el):
        d = {}
        if el.attrib:
            d.update({"@" + k: v for k, v in el.attrib.items()})
        children = list(el)
        if not children:
            return el.text or ""
        for child in children:
            tag = child.tag
            val = elem_to_dict(child)
            if tag in d:
                if not isinstance(d[tag], list):
                    d[tag] = [d[tag]]
                d[tag].append(val)
            else:
                d[tag] = val
        return d
    return elem_to_dict(root)

def _to_xml(data, tag="root"):
    root = ET.Element(tag)
    def build(parent, val):
        if isinstance(val, dict):
            for k, v in val.items():
                child = ET.SubElement(parent, k)
                build(child, v)
        elif isinstance(val, list):
            for item in val:
                child = ET.SubElement(parent, "item")
                build(child, item)
        else:
            parent.text = str(val) if val is not None else ""
    build(root, data)
    return ET.tostring(root, encoding="unicode")

def {{.FunctionName}}(input_path, output_path=None, input_format="json", output_format="json", fields=None, sort_by=None, limit=None):
    """{{.Description}}"""
    if not os.path.isabs(input_path):
        input_path = os.path.abspath(input_path)
    if not os.path.exists(input_path):
        return {"status": "error", "message": f"File not found: {input_path}"}

    try:
        with open(input_path, "r", encoding="utf-8") as f:
            raw = f.read()

        if input_format == "json":
            data = json.loads(raw)
        elif input_format == "csv":
            reader = csv.DictReader(io.StringIO(raw))
            data = list(reader)
        elif input_format == "yaml":
            if yaml is None:
                return {"status": "error", "message": "pyyaml not installed"}
            data = yaml.safe_load(raw)
        elif input_format == "xml":
            if ET is None:
                return {"status": "error", "message": "xml module not available"}
            data = _parse_xml(raw)
        else:
            return {"status": "error", "message": f"Unsupported input format: {input_format}"}

        if fields and isinstance(data, list):
            field_list = [f.strip() for f in fields.split(",")]
            data = [{k: row.get(k) for k in field_list} for row in data]

        if sort_by and isinstance(data, list):
            reverse = sort_by.startswith("-")
            key = sort_by.lstrip("-")
            data.sort(key=lambda r: r.get(key, ""), reverse=reverse)

        if limit and isinstance(data, list):
            data = data[:int(limit)]

        if output_format == "json":
            output = json.dumps(data, indent=2, ensure_ascii=False)
        elif output_format == "csv":
            if not isinstance(data, list) or not data:
                return {"status": "error", "message": "CSV output requires a list of objects"}
            buf = io.StringIO()
            writer = csv.DictWriter(buf, fieldnames=data[0].keys())
            writer.writeheader()
            writer.writerows(data)
            output = buf.getvalue()
        elif output_format == "yaml":
            if yaml is None:
                return {"status": "error", "message": "pyyaml not installed"}
            output = yaml.dump(data, allow_unicode=True, default_flow_style=False)
        elif output_format == "xml":
            output = _to_xml(data)
        else:
            return {"status": "error", "message": f"Unsupported output format: {output_format}"}

        if output_path:
            if not os.path.isabs(output_path):
                output_path = os.path.abspath(output_path)
            with open(output_path, "w", encoding="utf-8") as f:
                f.write(output)
            return {"status": "success", "result": f"Converted {input_format} -> {output_format}, written to {output_path}"}
        return {"status": "success", "result": output}
    except Exception as e:
        return {"status": "error", "message": str(e)}
