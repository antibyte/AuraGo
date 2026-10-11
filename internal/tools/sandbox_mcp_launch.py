"""Launch llm-sandbox MCP and apply AuraGo's IMAGE to every session."""

import os

import llm_sandbox.mcp_server.server as server

_IMAGE = os.environ.get("IMAGE", "").strip()


def _wrap_class(cls):
    original = cls.__init__

    def init(self, *args, **kwargs):
        if _IMAGE and "image" not in kwargs:
            kwargs["image"] = _IMAGE
        return original(self, *args, **kwargs)

    cls.__init__ = init


def _session_function(original):
    def session(*args, **kwargs):
        if _IMAGE and "image" not in kwargs:
            kwargs["image"] = _IMAGE
        return original(*args, **kwargs)

    return session


if isinstance(server.SandboxSession, type):
    _wrap_class(server.SandboxSession)
else:
    server.SandboxSession = _session_function(server.SandboxSession)

if isinstance(server.ArtifactSandboxSession, type):
    _wrap_class(server.ArtifactSandboxSession)

server.main()
