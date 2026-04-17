import sys
import json
import os

def {{.FunctionName}}(host, command, user=None, port=22, timeout=30):
    """{{.Description}}"""
    if not host:
        return {"status": "error", "message": "Host is required"}
    if not command:
        return {"status": "error", "message": "Command is required"}

    try:
        import paramiko
    except ImportError:
        return {"status": "error", "message": "paramiko not installed. Add 'paramiko' to dependencies."}

    ssh_key = os.environ.get("AURAGO_SECRET_SSH_KEY", "")
    ssh_password = os.environ.get("AURAGO_SECRET_SSH_PASSWORD", "")
    if not user:
        user = os.environ.get("AURAGO_SECRET_SSH_USER", os.environ.get("USER", "root"))

    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())

    try:
        connect_kwargs = {
            "hostname": host,
            "port": int(port),
            "username": user,
            "timeout": int(timeout),
        }

        tmp_key_path = None
        try:
            if ssh_key:
                key_file = os.path.expanduser(ssh_key) if not os.path.isabs(ssh_key) else ssh_key
                if os.path.isfile(key_file):
                    connect_kwargs["key_filename"] = key_file
                else:
                    import tempfile, stat
                    with tempfile.NamedTemporaryFile(mode="w", suffix=".key", delete=False) as kf:
                        kf.write(ssh_key)
                        tmp_key_path = kf.name
                    os.chmod(tmp_key_path, stat.S_IRUSR | stat.S_IWUSR)
                    connect_kwargs["key_filename"] = tmp_key_path
            elif ssh_password:
                connect_kwargs["password"] = ssh_password
            else:
                key_path = os.path.expanduser("~/.ssh/id_rsa")
                if os.path.isfile(key_path):
                    connect_kwargs["key_filename"] = key_path

            client.connect(**connect_kwargs)
            stdin, stdout, stderr = client.exec_command(command, timeout=int(timeout))

            exit_code = stdout.channel.recv_exit_status()
            out = stdout.read().decode("utf-8", errors="replace")
            err = stderr.read().decode("utf-8", errors="replace")

            return {
                "status": "success" if exit_code == 0 else "error",
                "result": {
                    "host": host,
                    "command": command,
                    "exit_code": exit_code,
                    "stdout": out[-10000:] if len(out) > 10000 else out,
                    "stderr": err[-5000:] if len(err) > 5000 else err,
                },
            }
        except paramiko.AuthenticationException:
            return {"status": "error", "message": f"Authentication failed for {user}@{host}"}
        except paramiko.SSHException as e:
            return {"status": "error", "message": f"SSH error: {str(e)}"}
        except Exception as e:
            return {"status": "error", "message": str(e)}
        finally:
            if tmp_key_path and os.path.exists(tmp_key_path):
                os.remove(tmp_key_path)
    finally:
        client.close()
