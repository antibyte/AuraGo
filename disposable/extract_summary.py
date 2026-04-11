import re
with open('reports/translation_audit_report.md','r',encoding='utf-8') as f:
    content = f.read()

sections = re.findall(r'## `([^`]+)`', content)
print('Verzeichnisse mit Problemen:', len(sections))
print('Beispiele:', sections[:10])

lines = [l for l in content.split('\n') if l.startswith('| `')]
print('\nZusammenfassung:')
for l in lines:
    print(l)
