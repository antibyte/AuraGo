import json
with open('gosec-results', 'r', encoding='utf-8') as f:
    data = json.load(f)

highs = [i for i in data.get('Issues', []) if i.get('severity') == 'HIGH']
with open('gosec_high.txt', 'w', encoding='utf-8') as out:
    out.write(f"Total High Severity Issues: {len(highs)}\n")
    for count, issue in enumerate(highs):
        out.write(f"{count+1}. {issue.get('details')} - {issue.get('file')}:{issue.get('line')}\n")
