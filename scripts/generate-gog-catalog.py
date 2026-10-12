"""Regenerate the reviewed CLI allowlist from `gog schema --json` (offline)."""
import json
import re
import sys
from pathlib import Path

root = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8-sig"))["command"]
# Operation, capability, exact command, permitted local flags. Positional arguments
# come from the pinned schema. Never admit root, path, raw API or execution flags.
specs = """
gmail search read messages.search max,page
gmail read read get format
gmail attachment read attachment
gmail drafts read drafts.list max,page
gmail draft read drafts.get
gmail create_draft create drafts.create to,cc,bcc,subject,body
gmail update_draft update drafts.update to,cc,bcc,subject,body
gmail delete_draft delete drafts.delete
gmail send send send to,cc,bcc,subject,body
gmail send_draft send drafts.send
gmail labels read labels.list
gmail create_label create labels.create
gmail rename_label update labels.rename
gmail delete_label delete labels.delete
gmail label update messages.modify add,remove
gmail archive update archive
gmail trash delete trash
calendar calendars read calendars max,page
calendar events read events from,to,max,page,timezone
calendar get read event timezone
calendar create create create summary,description,location,from,to,all-day,start-timezone,end-timezone,rrule
calendar update update update summary,description,location,from,to,all-day,start-timezone,end-timezone,scope,original-start,rrule
calendar delete delete delete scope,original-start
calendar freebusy read freebusy from,to
drive list read ls parent,max,page
drive search read search max,page
drive get read get
drive upload create upload parent,name,mime-type
drive replace update upload replace,if-version,mime-type
drive import_office create upload parent,name,convert-to
drive download read download format
drive mkdir create mkdir parent
drive copy create copy parent
drive move update move parent
drive rename update rename
drive trash delete delete
docs read read cat
docs info read info
docs create create create parent
docs replace_text update find-replace first,match-case,tab
docs export read export format
sheets read read get render,dimension
sheets info read metadata
sheets create create create parent,sheets
sheets update update update values-json,input
sheets append update append values-json,input,insert
sheets export read export format
slides read read info
slides list read list-slides
slides read_slide read read-slide
slides create create create-from-markdown content,parent
slides replace_text update replace-text object,match-case
slides export read export format
contacts list read list max,page
contacts search read search max
contacts get read get
contacts create create create given,family,email,phone,org,title,note
contacts update update update given,family,email,phone,org,title,notes,birthday
contacts delete delete delete
tasks lists read lists.list max,page
tasks create_list create lists.create
tasks list read list max,page,show-completed
tasks get read get
tasks create create add title,notes,due,parent
tasks update update update title,notes,due,status
tasks complete update done
tasks reopen update undo
tasks delete delete delete
"""

def name(value):
    return re.sub(r"(?<!^)(?=[A-Z])", "_", value).lower().replace("-", "_")

catalog = []
for line in specs.strip().splitlines():
    parts = line.split(); service, operation, grant, command = parts[:4]
    flags = parts[4].split(",") if len(parts)>4 else []
    node = root
    for part in (service+"."+command).split("."):
        node = next(c for c in node["subcommands"] if c["name"]==part)
    entry = dict(service=service, operation=operation, grant=grant, command=service+"."+command, fields=[])
    for pos in node.get("positionals", []):
        if pos["name"] in ["values"]: continue
        if pos["name"]=="localPath": entry["input"]=True; continue
        entry["fields"].append(dict(name=name(pos["name"]), type="string", positional=True, required=True, description=pos["help"]))
    for flag in flags:
        source=next(f for f in node["flags"] if f["name"]==flag)
        kind=source["type"].lstrip("*")
        kind={"int":"integer","int64":"integer","bool":"boolean","[]string":"array"}.get(kind,kind)
        field=dict(name=name(flag),type=kind,flag=flag,description=source["help"])
        if source.get("enum"): field["enum"]=source["enum"]
        if "--"+flag in node.get("requirements",[]): field["required"]=True
        entry["fields"].append(field)
    if command in ["export","download","attachment"]: entry["output"]=True
    required={"calendar.create":["summary","from","to"],"calendar.update":["scope"],"calendar.delete":["scope"],"calendar.freebusy":["from","to"],"drive.replace":["replace","if_version"],"drive.import_office":["convert_to"],"slides.create":["content"],"gmail.send":["to","subject","body"],"gmail.create_draft":["subject","body"],"sheets.update":["values_json"],"sheets.append":["values_json"],"tasks.create":["title"],"contacts.create":["given"]}.get(service+"."+operation,[])
    for f in entry["fields"]:
        if f["name"] in required:f["required"]=True
        if f["name"]=="scope": f["enum"]=["single","future","all"]
        if f["name"]=="convert_to":f["enum"]=["doc","sheet","slides"]
    catalog.append(entry)
Path("internal/gog/catalog.json").write_text(json.dumps(catalog,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
print(f"Validated {len(catalog)} operations against the pinned CLI schema")
