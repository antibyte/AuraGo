"""Launch llm-sandbox MCP and apply AuraGo's IMAGE to every session."""

import os

import llm_sandbox.mcp_server.server as server

_IMAGE = os.environ.get("IMAGE", "").strip()


def _install_image(cls):
    original = cls.__init__

    def init(self, *args, **kwargs):
        if _IMAGE and not kwargs.get("image"):
            kwargs["image"] = _IMAGE
        return original(self, *args, **kwargs)

    cls.__init__ = init


_install_image(server.SandboxSession)
_install_image(server.ArtifactSandboxSession)
server.main()
