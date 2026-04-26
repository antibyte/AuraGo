import sys
import json


def {{.FunctionName}}(text=""):
    """{{.Description}}"""
    return {
        "status": "success",
        "result": text if text else "ok",
    }
