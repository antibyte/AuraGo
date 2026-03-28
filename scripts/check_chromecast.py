import sys, json

try:
    import paramiko
except ImportError:
    print("paramiko not installed, installing...")
    import subprocess
    subprocess.run([sys.executable, "-m", "pip", "install", "paramiko", "-q"], check=True)
    import paramiko

client = paramiko.SSHClient()
client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
client.connect('192.168.6.238', username='aurago', password='2601mana0510', timeout=10)

print("=== chromecast config ===")
_, stdout, _ = client.exec_command('grep -A3 "chromecast:" ~/aurago/config.yaml')
print(stdout.read().decode())

print("=== full auth section from config ===")
_, stdout, _ = client.exec_command('python3 -c "import yaml; d=yaml.safe_load(open(\'config.yaml\')); import json; print(json.dumps(d.get(\'auth\', {}), indent=2))" 2>/dev/null || grep -A10 "^auth:" ~/aurago/config.yaml | head -15')
print(stdout.read().decode())

print("=== tool list (from /api/tools) ===")
# Try with api_key from config if present
_, stdout, _ = client.exec_command('grep -E "api_key:|token:|secret:" ~/aurago/config.yaml | grep -v "#" | head -5')
print("API keys in config:", stdout.read().decode())

# Also check if there's a token in data dir
_, stdout, _ = client.exec_command('ls ~/aurago/data/ 2>/dev/null | head -20')
print("Data dir:", stdout.read().decode())

if raw:
    try:
        tools = json.loads(raw)
        if isinstance(tools, list):
            names = []
            for t in tools:
                if isinstance(t, dict):
                    names.append(t.get("function", {}).get("name", t.get("name", "?")))
                elif isinstance(t, str):
                    names.append(t)
            for n in sorted(names):
                print(n)
        else:
            print("Unexpected format:", str(tools)[:500])
    except json.JSONDecodeError:
        print("Raw response:", raw[:500])
else:
    print("No response from API")

client.close()
