import paramiko, json
c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect('192.168.6.238', username='aurago', password='2601mana0510', timeout=8)

# Use OpenAI-compatible endpoint with internal bypass
payload = json.dumps({
    "model": "aurago",
    "messages": [{"role": "user", "content": "Do you have a chromecast tool? Just answer yes or no."}],
    "max_tokens": 50,
    "stream": False
})
cmd = ('curl -s -m 20 -X POST '
       '-H "Content-Type: application/json" '
       '-H "X-Internal-FollowUp: true" '
       f"-d '{payload}' "
       'http://localhost:8088/v1/chat/completions 2>&1')
_, stdout, _ = c.exec_command(cmd)
raw = stdout.read().decode()
print("=== v1/chat/completions ===")
print(raw[:500])

# Also reset rate limit by clearing it via SSH (if stored in file)
_, stdout, _ = c.exec_command('ls /home/aurago/aurago/data/ | grep -i rate')
print("\n=== rate limit files ===")
print(stdout.read().decode())

c.close()
