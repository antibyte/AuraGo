if __name__ == "__main__":
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
        print(json.dumps({"status": "error", "message": "No input provided."}))
        sys.exit(1)
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    result = 
