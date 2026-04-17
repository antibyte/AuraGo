import sys
import json
import os
import smtplib
from email.mime.text import MIMEText
from email.mime.multipart import MIMEMultipart
from email.mime.base import MIMEBase
from email import encoders
import requests

def _send_telegram(token, chat_id, message, title=None, priority="normal"):
    url = f"https://api.telegram.org/bot{token}/sendMessage"
    text = f"*{title}*\n\n{message}" if title else message
    if priority == "high":
        text = "\u26a0\ufe0f " + text
    payload = {"chat_id": chat_id, "text": text, "parse_mode": "Markdown"}
    resp = requests.post(url, json=payload, timeout=10)
    resp.raise_for_status()
    return resp.json()

def _send_discord(webhook_url, message, title=None, priority="normal"):
    payload = {"content": ""}
    embed = {"description": message}
    if title:
        embed["title"] = title
    color_map = {"low": 3447003, "normal": 3066993, "high": 15158332}
    embed["color"] = color_map.get(priority, 3066993)
    payload["embeds"] = [embed]
    resp = requests.post(webhook_url, json=payload, timeout=10)
    resp.raise_for_status()
    return {"sent": True}

def _send_email(smtp_host, smtp_port, smtp_user, smtp_pass, from_addr, to_addr, message, title=None, attach=None, priority="normal"):
    msg = MIMEMultipart()
    msg["From"] = from_addr
    msg["To"] = to_addr
    msg["Subject"] = title or "AuraGo Notification"
    if priority == "high":
        msg["X-Priority"] = "1"
    msg.attach(MIMEText(message, "plain"))

    if attach and os.path.isfile(attach):
        with open(attach, "rb") as f:
            part = MIMEBase("application", "octet-stream")
            part.set_payload(f.read())
        encoders.encode_base64(part)
        part.add_header("Content-Disposition", f"attachment; filename={os.path.basename(attach)}")
        msg.attach(part)

    if smtp_port == 465:
        server = smtplib.SMTP_SSL(smtp_host, smtp_port, timeout=15)
    else:
        server = smtplib.SMTP(smtp_host, smtp_port, timeout=15)
        server.starttls()
    if smtp_user and smtp_pass:
        server.login(smtp_user, smtp_pass)
    server.sendmail(from_addr, to_addr, msg.as_string())
    server.quit()
    return {"sent": True}

def _send_webhook(url, message, title=None, priority="normal"):
    payload = {
        "message": message,
        "title": title,
        "priority": priority,
        "source": "AuraGo",
    }
    api_key = os.environ.get("AURAGO_SECRET_WEBHOOK_KEY", "")
    headers = {"Content-Type": "application/json"}
    if api_key:
        headers["Authorization"] = f"Bearer {api_key}"
    resp = requests.post(url, json=payload, headers=headers, timeout=10)
    resp.raise_for_status()
    try:
        return resp.json()
    except ValueError:
        return {"status": "sent", "http_code": resp.status_code}

def {{.FunctionName}}(channel, message, title=None, attach=None, priority="normal"):
    """{{.Description}}"""
    if not message:
        return {"status": "error", "message": "Message text is required"}

    try:
        if channel == "telegram":
            token = os.environ.get("AURAGO_SECRET_TELEGRAM_BOT_TOKEN", "")
            chat_id = os.environ.get("AURAGO_SECRET_TELEGRAM_CHAT_ID", "")
            if not token or not chat_id:
                return {"status": "error", "message": "Telegram requires vault keys: TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID"}
            result = _send_telegram(token, chat_id, message, title, priority)

        elif channel == "discord":
            webhook_url = os.environ.get("AURAGO_SECRET_DISCORD_WEBHOOK_URL", "")
            if not webhook_url:
                return {"status": "error", "message": "Discord requires vault key: DISCORD_WEBHOOK_URL"}
            result = _send_discord(webhook_url, message, title, priority)

        elif channel == "email":
            smtp_host = os.environ.get("AURAGO_SECRET_SMTP_HOST", "")
            smtp_port = int(os.environ.get("AURAGO_SECRET_SMTP_PORT", "587"))
            smtp_user = os.environ.get("AURAGO_SECRET_SMTP_USER", "")
            smtp_pass = os.environ.get("AURAGO_SECRET_SMTP_PASSWORD", "")
            from_addr = os.environ.get("AURAGO_SECRET_EMAIL_FROM", smtp_user)
            to_addr = os.environ.get("AURAGO_SECRET_EMAIL_TO", "")
            if not smtp_host or not to_addr:
                return {"status": "error", "message": "Email requires vault keys: SMTP_HOST, EMAIL_TO (and SMTP_USER/SMTP_PASSWORD for auth)"}
            result = _send_email(smtp_host, smtp_port, smtp_user, smtp_pass, from_addr, to_addr, message, title, attach, priority)

        elif channel == "webhook":
            url = os.environ.get("AURAGO_SECRET_WEBHOOK_URL", "{{.BaseURL}}")
            if not url:
                return {"status": "error", "message": "Webhook requires vault key: WEBHOOK_URL or 'url' parameter"}
            result = _send_webhook(url, message, title, priority)

        else:
            return {"status": "error", "message": f"Unknown channel: {channel}. Use: telegram, discord, email, webhook"}

        return {"status": "success", "result": result}
    except Exception as e:
        return {"status": "error", "message": str(e)}
