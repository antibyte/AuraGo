#!/usr/bin/env python3
"""Audit translation files for nested JSON structures."""
import json
import os
from pathlib import Path

LANG_DIR = Path("ui/lang")
PROBLEMATIC_FILES = []

def check_file(filepath: Path) -> bool:
    """Check if a file has nested objects (problematic) instead of flat string values."""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            data = json.load(f)
        
        has_nested = False
        for key, value in data.items():
            if isinstance(value, dict):
                print(f"  [NESTED] {filepath} - key '{key}' is an object")
                has_nested = True
            elif not isinstance(value, str):
                print(f"  [UNEXPECTED] {filepath} - key '{key}' is {type(value).__name__}, expected string")
                has_nested = True
        
        return has_nested
    except json.JSONDecodeError as e:
        print(f"  [JSON ERROR] {filepath} - {e}")
        return True
    except Exception as e:
        print(f"  [ERROR] {filepath} - {e}")
        return True

def main():
    print("Auditing translation files in ui/lang/\n")
    
    # Get all JSON files except en.json files (which are reference)
    json_files = []
    for root, dirs, files in os.walk(LANG_DIR):
        for file in files:
            if file.endswith('.json') and not file.endswith('/en.json'):
                filepath = Path(root) / file
                json_files.append(filepath)
    
    json_files.sort()
    
    for filepath in json_files:
        is_problematic = check_file(filepath)
        if is_problematic:
            PROBLEMATIC_FILES.append(filepath)
    
    print(f"\n{'='*60}")
    print(f"Summary: {len(PROBLEMATIC_FILES)} problematic files found")
    for f in PROBLEMATIC_FILES:
        print(f"  - {f}")
    
    return len(PROBLEMATIC_FILES)

if __name__ == "__main__":
    main()
