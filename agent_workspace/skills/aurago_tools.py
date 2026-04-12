"""AuraGo Tool Bridge SDK — call native AuraGo tools from Python skills.

Usage:
    from aurago_tools import AuraGoTools

    tools = AuraGoTools()
    result = tools.call("proxmox", {
        "operation": "list_vms",
        "node": "pve"
    })
    print(result)  # parsed JSON or raw string

Environment variables (injected automatically by AuraGo when internal_tools are approved):
    AURAGO_TOOL_BRIDGE_URL   — e.g. http://127.0.0.1:3080/api/internal/tool-bridge
    AURAGO_TOOL_BRIDGE_TOKEN — internal authentication token
"""

import json
import os
import urllib.request
import urllib.error


class AuraGoToolError(Exception):
    """Raised when a tool call fails."""
    pass


class AuraGoTools:
    """Client for calling AuraGo native tools from Python skills.

    This uses the internal loopback API which is only available when:
    1. python_tool_bridge.enabled is true in config
    2. The tool is listed in python_tool_bridge.allowed_tools
    3. The skill manifest declares the tool in internal_tools
    4. The user has approved the skill's internal_tools access
    """

    def __init__(self, base_url=None, token=None):
        self._base_url = base_url or os.environ.get("AURAGO_TOOL_BRIDGE_URL", "")
        self._token = token or os.environ.get("AURAGO_TOOL_BRIDGE_TOKEN", "")
        if not self._base_url:
            raise AuraGoToolError(
                "AURAGO_TOOL_BRIDGE_URL not set. "
                "Ensure the skill has internal_tools declared and approved."
            )
        if not self._token:
            raise AuraGoToolError(
                "AURAGO_TOOL_BRIDGE_TOKEN not set. "
                "Ensure the skill has internal_tools declared and approved."
            )

    def call(self, tool_name, parameters=None, timeout=60):
        """Call an AuraGo native tool.

        Args:
            tool_name: Name of the tool (e.g. "proxmox", "docker", "api_request").
            parameters: Dict of tool parameters.
            timeout: Execution timeout in seconds (default 60, max 300).

        Returns:
            Parsed JSON dict if the result is valid JSON, otherwise the raw string.

        Raises:
            AuraGoToolError: If the call fails or the tool returns an error.
        """
        if parameters is None:
            parameters = {}

        url = self._base_url.rstrip("/") + "/" + tool_name

        payload = json.dumps({
            "parameters": parameters,
            "timeout": timeout,
        }).encode("utf-8")

        req = urllib.request.Request(
            url,
            data=payload,
            method="POST",
            headers={
                "Content-Type": "application/json",
                "X-Internal-Token": self._token,
            },
        )

        try:
            with urllib.request.urlopen(req, timeout=timeout + 5) as resp:
                body = resp.read().decode("utf-8")
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8", errors="replace")
            try:
                data = json.loads(body)
                raise AuraGoToolError(f"Tool bridge HTTP {e.code}: {data.get('result', body)}")
            except (json.JSONDecodeError, ValueError):
                raise AuraGoToolError(f"Tool bridge HTTP {e.code}: {body}")
        except urllib.error.URLError as e:
            raise AuraGoToolError(f"Tool bridge connection failed: {e.reason}")

        try:
            data = json.loads(body)
        except (json.JSONDecodeError, ValueError):
            return body

        if data.get("status") == "error":
            raise AuraGoToolError(f"Tool '{tool_name}' failed: {data.get('result', 'unknown error')}")

        # Try to parse the result as JSON (many tools return JSON strings)
        result = data.get("result", "")
        try:
            return json.loads(result)
        except (json.JSONDecodeError, ValueError, TypeError):
            return result

    def call_raw(self, tool_name, parameters=None, timeout=60):
        """Call an AuraGo native tool and return the raw result string.

        Same as call() but always returns the raw string without JSON parsing.
        """
        if parameters is None:
            parameters = {}

        url = self._base_url.rstrip("/") + "/" + tool_name

        payload = json.dumps({
            "parameters": parameters,
            "timeout": timeout,
        }).encode("utf-8")

        req = urllib.request.Request(
            url,
            data=payload,
            method="POST",
            headers={
                "Content-Type": "application/json",
                "X-Internal-Token": self._token,
            },
        )

        try:
            with urllib.request.urlopen(req, timeout=timeout + 5) as resp:
                body = resp.read().decode("utf-8")
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8", errors="replace")
            raise AuraGoToolError(f"Tool bridge HTTP {e.code}: {body}")
        except urllib.error.URLError as e:
            raise AuraGoToolError(f"Tool bridge connection failed: {e.reason}")

        try:
            data = json.loads(body)
            return data.get("result", body)
        except (json.JSONDecodeError, ValueError):
            return body
