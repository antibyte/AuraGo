import sys
import json
import os
import PyPDF2

def extract_pdf(filepath):
    try:
        # Standardized path resolution
        if os.path.isabs(filepath):
            full_path = filepath
        else:
            # Assume relative to workdir if not absolute
            full_path = os.path.abspath(filepath)

        if not os.path.exists(full_path):
            print(json.dumps({"status": "error", "message": f"File not found: {filepath}"}))
            return

        text = ""
        with open(full_path, "rb") as f:
            reader = PyPDF2.PdfReader(f)
            for page in reader.pages:
                text += page.extract_text() + "\n"

        print(json.dumps({"status": "success", "content": f"<external_data>{text}</external_data>"}, ensure_ascii=False))
        
    except Exception as e:
        print(json.dumps({"status": "error", "message": str(e)}))

if __name__ == "__main__":
    if sys.platform == 'win32':
        import msvcrt, os
        msvcrt.setmode(sys.stdin.fileno(), os.O_BINARY)
        
    args = {}
    try:
        stdin_data = sys.stdin.read().strip()
        if stdin_data:
            args = json.loads(stdin_data)
    except Exception:
        pass
        
    if not args and len(sys.argv) > 1:
        try:
            args = json.loads(sys.argv[1])
        except Exception:
            pass
            
    if not args:
        print(json.dumps({"status": "error", "message": "No input provided (expected Stdin or Argv)."}))
        sys.exit(1)
        
    # Force UTF-8 stdout for Windows
    if hasattr(sys.stdout, 'reconfigure'):
        sys.stdout.reconfigure(encoding='utf-8')
    
    extract_pdf(args.get("filepath"))
    