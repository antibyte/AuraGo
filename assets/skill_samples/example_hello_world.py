#!/usr/bin/env python3
"""
Example Skill: Hello World
A minimal template showing the structure of an AuraGo Python skill.
"""
import sys
import json


def main():
    # AuraGo passes arguments as a JSON object in argv[1]
    args = {}
    if len(sys.argv) > 1:
        try:
            args = json.loads(sys.argv[1])
        except json.JSONDecodeError:
            pass

    name = args.get("name", "World")
    greeting = f"Hello, {name}! This is an example skill running inside AuraGo."

    # Always print a JSON result so AuraGo can parse it
    print(json.dumps({
        "status": "ok",
        "message": greeting,
        "hint": "Use this file as a starting point for your own custom skills."
    }))


if __name__ == "__main__":
    main()
