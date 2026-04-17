import sys
import json
import os
import socket
import time
import requests

def _check_http(target, timeout, expected, keyword):
    start = time.time()
    try:
        resp = requests.get(target, timeout=int(timeout), headers={
            "User-Agent": "AuraGo-Monitor/1.0",
        }, allow_redirects=True)
        latency_ms = round((time.time() - start) * 1000, 1)
        result = {
            "target": target,
            "type": "http",
            "status_code": resp.status_code,
            "latency_ms": latency_ms,
            "passed": True,
        }
        if expected:
            result["expected"] = int(expected)
            result["passed"] = resp.status_code == int(expected)
        if keyword:
            found = keyword.lower() in resp.text.lower()
            result["keyword"] = keyword
            result["keyword_found"] = found
            result["passed"] = result["passed"] and found
        return result
    except requests.RequestException as e:
        latency_ms = round((time.time() - start) * 1000, 1)
        return {"target": target, "type": "http", "passed": False, "error": str(e), "latency_ms": latency_ms}

def _check_tcp(target, timeout):
    if ":" not in target:
        return {"target": target, "type": "tcp", "passed": False, "error": "Format must be host:port"}
    host, port_str = target.rsplit(":", 1)
    try:
        port = int(port_str)
    except ValueError:
        return {"target": target, "type": "tcp", "passed": False, "error": "Invalid port number"}
    start = time.time()
    try:
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.settimeout(int(timeout))
        sock.connect((host, port))
        sock.close()
        latency_ms = round((time.time() - start) * 1000, 1)
        return {"target": target, "type": "tcp", "passed": True, "latency_ms": latency_ms}
    except (socket.timeout, socket.error) as e:
        latency_ms = round((time.time() - start) * 1000, 1)
        return {"target": target, "type": "tcp", "passed": False, "error": str(e), "latency_ms": latency_ms}

def _check_dns(target, expected):
    start = time.time()
    try:
        resolved = socket.gethostbyname(target)
        latency_ms = round((time.time() - start) * 1000, 1)
        result = {
            "target": target,
            "type": "dns",
            "resolved_ip": resolved,
            "latency_ms": latency_ms,
            "passed": True,
        }
        if expected:
            result["expected"] = expected
            result["passed"] = resolved == expected
        return result
    except socket.gaierror as e:
        latency_ms = round((time.time() - start) * 1000, 1)
        return {"target": target, "type": "dns", "passed": False, "error": str(e), "latency_ms": latency_ms}

def {{.FunctionName}}(target, check_type="http", timeout=10, expected=None, keyword=None):
    """{{.Description}}"""
    if not target:
        return {"status": "error", "message": "Target is required"}

    timeout = int(timeout)
    if check_type == "http":
        if not target.startswith(("http://", "https://")):
            target = f"http://{target}"
        result = _check_http(target, timeout, expected, keyword)
    elif check_type == "tcp":
        result = _check_tcp(target, timeout)
    elif check_type == "dns":
        result = _check_dns(target, expected)
    else:
        return {"status": "error", "message": f"Unknown check type: {check_type}. Use: http, tcp, dns"}

    status = "success" if result.get("passed") else "warning"
    return {"status": status, "result": result}
