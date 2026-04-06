import os, re

base_dir = 'C:/Users/Andi/Documents/repo/AuraGo/internal/tools'

for fname in sorted(os.listdir(base_dir)):
    if not fname.endswith('.go') or fname.endswith('_test.go'):
        continue
    
    filepath = os.path.join(base_dir, fname)
    with open(filepath, 'r', encoding='utf-8', errors='replace') as f:
        content = f.read()
    
    # Remove all backtick string literals
    cleaned = re.sub(r'`[^`]*`', 'REMOVED_STRING', content, flags=re.DOTALL)
    
    # Remove double-quoted strings
    cleaned = re.sub(r'"(?:[^"\\]|\\.)*"', 'REMOVED_STRING', cleaned)
    
    # Now find struct definitions in the cleaned content
    lines = cleaned.split('\n')
    orig_lines = content.split('\n')
    
    in_struct = False
    struct_name = ''
    struct_line = 0
    
    for i, line in enumerate(lines, 1):
        stripped = line.strip()
        
        # Remove inline comments
        comment_pos = stripped.find('//')
        if comment_pos >= 0:
            stripped = stripped[:comment_pos].strip()
        
        # Check for struct definition
        struct_match = re.match(r'^type\s+(\w+)\s+struct\s*\{', stripped)
        if struct_match:
            in_struct = True
            struct_name = struct_match.group(1)
            struct_line = i
            continue
        
        if in_struct:
            if stripped == '}' or (stripped.startswith('}') and '{' not in stripped):
                in_struct = False
                continue
            
            if not stripped:
                continue
            
            # Skip embedded struct types
            if 'struct {' in stripped or 'struct{' in stripped:
                continue
            
            # Parse field name
            match = re.match(r'^([a-zA-Z_]\w*)\s', stripped)
            if match:
                field_name = match.group(1)
                if '_' in field_name:
                    orig = orig_lines[i-1].strip()
                    print(f'VIOLATION: {filepath}:{i}')
                    print(f'  Struct: {struct_name} (line {struct_line})')
                    print(f'  Field:  {field_name}')
                    print(f'  Line:   {orig}')
                    print()

print("=== Scan complete ===")
