import os, re

base_dir = 'C:/Users/Andi/Documents/repo/AuraGo/internal/tools'

# Count structs found and list all files with structs
structs_by_file = {}
for fname in sorted(os.listdir(base_dir)):
    if not fname.endswith('.go') or fname.endswith('_test.go'):
        continue
    filepath = os.path.join(base_dir, fname)
    with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
        content = f.read()
    
    # Remove backtick strings
    cleaned = re.sub(r'`[^`]*`', 'STR', content, flags=re.DOTALL)
    cleaned = re.sub(r'"(?:[^"\\]|\\.)*"', 'STR', cleaned)
    
    structs = re.findall(r'type\s+(\w+)\s+struct\s*\{', cleaned)
    if structs:
        structs_by_file[fname] = structs

total = sum(len(v) for v in structs_by_file.values())
print(f'Total structs in non-test files: {total}')
print(f'Files with structs: {len(structs_by_file)}')
print()
for fname, structs in sorted(structs_by_file.items()):
    print(f'{fname}: {len(structs)} structs - {structs}')
