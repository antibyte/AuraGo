import sys
import json
import os
import time
import requests

def {{.FunctionName}}(endpoint, method="GET", body=None, headers=None, auth_type="bearer", max_pages=1):
    """{{.Description}}"""
    base_url = os.environ.get("AURAGO_SECRET_BASE_URL", "{{.BaseURL}}").rstrip("/")
    api_key = os.environ.get("AURAGO_SECRET_API_KEY", "")
    username = os.environ.get("AURAGO_SECRET_USERNAME", "")
    password = os.environ.get("AURAGO_SECRET_PASSWORD", "")

    req_headers = {"Content-Type": "application/json"}
    if headers and isinstance(headers, dict):
        req_headers.update(headers)

    if auth_type == "bearer" and api_key:
        req_headers["Authorization"] = f"Bearer {api_key}"
    elif auth_type == "basic" and username and password:
        pass
    elif auth_type == "api_key" and api_key:
        req_headers["X-API-Key"] = api_key

    url = f"{base_url}/{endpoint.lstrip('/')}" if endpoint else base_url
    all_items = []
    page = 0
    retries = 3

    try:
        while page < int(max_pages):
            kwargs = {
                "method": method.upper(),
                "url": url,
                "headers": req_headers,
                "timeout": 30,
            }
            if body and method.upper() in ("POST", "PUT", "PATCH"):
                kwargs["json"] = body
            if auth_type == "basic" and username and password:
                kwargs["auth"] = (username, password)

            for attempt in range(retries):
                try:
                    resp = requests.request(**kwargs)
                    resp.raise_for_status()
                    break
                except requests.exceptions.ConnectionError as e:
                    if attempt == retries - 1:
                        raise
                    time.sleep(2 ** attempt)

            try:
                data = resp.json()
            except ValueError:
                data = resp.text

            if isinstance(data, list):
                all_items.extend(data)
            elif isinstance(data, dict):
                items = data.get("data", data.get("items", data.get("results", None)))
                if isinstance(items, list):
                    all_items.extend(items)
                else:
                    all_items.append(data)

            next_url = None
            if isinstance(data, dict):
                next_url = data.get("next") or data.get("next_page_token")
                if isinstance(next_url, str) and next_url.startswith("http"):
                    url = next_url
                elif next_url:
                    sep = "&" if "?" in url else "?"
                    url = f"{url}{sep}page_token={next_url}"
                else:
                    break
            else:
                break

            page += 1

        result_data = all_items if all_items else data
        return {"status": "success", "result": f"<external_data>{json.dumps(result_data, ensure_ascii=False)}</external_data>"}
    except requests.RequestException as e:
        return {"status": "error", "message": str(e)}
