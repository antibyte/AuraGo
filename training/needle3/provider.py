"""Budgeted OpenRouter access. Keys are read from the process environment only."""
from __future__ import annotations

from decimal import Decimal
import json
import os
import urllib.error
import urllib.request

from .common import canonical

BASE_URL = "https://openrouter.ai/api/v1"


class RejectedResponse(ValueError):
    def __init__(self, message, receipt):
        super().__init__(message)
        self.receipt = receipt


def public_models():
    with urllib.request.urlopen(BASE_URL + "/models", timeout=60) as r:
        return {m["id"]: m for m in json.load(r)["data"]}


class OpenRouter:
    def __init__(self, budget, models=None):
        self.budget = budget
        self.key = os.environ.get("OPENROUTER_API_KEY", "")
        if not self.key:
            raise RuntimeError("OPENROUTER_API_KEY is not configured; no paid request was sent")
        self.models = models if models is not None else public_models()

    def complete(self, model, messages, stage, max_tokens=4096, response_schema=None):
        if model not in self.models:
            raise ValueError(f"configured model is unavailable: {model}")
        pricing = self.models[model]["pricing"]
        # Time-of-day overrides may change while a request is running. Reserve
        # the highest advertised rate and send the same ceiling to the router.
        rates = [pricing] + pricing.get("overrides", [])
        prompt_rate = max(Decimal(rate.get("prompt", pricing["prompt"])) for rate in rates)
        output_rate = max(Decimal(rate.get("completion", pricing["completion"])) for rate in rates)
        request_rate = Decimal(pricing.get("request", "0"))
        if min(prompt_rate, output_rate, request_rate) < 0:
            raise ValueError("unusable API prices")
        # Byte count plus framing is an intentionally conservative token bound.
        input_bound = len(canonical(messages).encode("utf-8")) + 4096
        upper = Decimal(input_bound) * prompt_rate + Decimal(max_tokens) * output_rate + request_rate
        request_id = self.budget.reserve(stage, model, max(upper, Decimal("0.000001")))
        payload = {"model": model, "messages": messages, "temperature": 0.7 if stage == "generate" else 0,
                   "max_tokens": max_tokens, "response_format": {"type": "json_object"},
                   "reasoning": {"effort": "minimal"}, "usage": {"include": True},
                   "provider": {"allow_fallbacks": False, "require_parameters": True,
                                "sort": "price", "max_price": {
                                    "prompt": float(prompt_rate * 1_000_000),
                                    "completion": float(output_rate * 1_000_000), "request": float(request_rate)}}}
        if response_schema:
            payload["response_format"] = {"type": "json_schema", "json_schema": {"name": "dataset_response", "strict": True, "schema": response_schema}}
        request = urllib.request.Request(BASE_URL + "/chat/completions", data=canonical(payload).encode("utf-8"),
                                         headers={"Authorization": "Bearer " + self.key, "Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(request, timeout=180) as r:
                result = json.load(r)
        except (urllib.error.URLError, TimeoutError, json.JSONDecodeError) as exc:
            # No automatic replay, no response body or credential in diagnostics.
            raise RuntimeError(f"API request {request_id} unresolved ({type(exc).__name__}); reservation retained") from None
        usage = result.get("usage", {})
        cost = usage.get("cost")
        if cost is not None:
            self.budget.settle(request_id, cost)
        else:
            raise RuntimeError(f"API request {request_id} did not report cost; reservation retained")
        receipt = {"request_id": request_id, "model": result.get("model", model), "usage": usage}
        choice = result["choices"][0]
        if choice.get("finish_reason") != "stop":
            raise RejectedResponse("incomplete API response; excluded from dataset", receipt)
        content = choice["message"]["content"]
        try:
            value = json.loads(content)
            if not isinstance(value, dict):
                raise ValueError("root must be an object")
        except (ValueError, TypeError):
            raise RejectedResponse("invalid JSON response; excluded from dataset", receipt) from None
        return value, receipt
