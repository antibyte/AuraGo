import sys
import json
import os
import threading
import time

def {{.FunctionName}}(action, topic, payload=None, qos=0, retain=False, timeout=5):
    """{{.Description}}"""
    if not topic:
        return {"status": "error", "message": "MQTT topic is required"}

    try:
        import paho.mqtt.client as mqtt
    except ImportError:
        return {"status": "error", "message": "paho-mqtt not installed. Add 'paho-mqtt' to dependencies."}

    broker_host = os.environ.get("AURAGO_SECRET_MQTT_HOST", os.environ.get("AURAGO_SECRET_BROKER_HOST", "localhost"))
    try:
        broker_port = int(os.environ.get("AURAGO_SECRET_MQTT_PORT", "1883"))
        qos = int(qos)
        timeout = float(timeout)
    except (TypeError, ValueError):
        return {"status": "error", "message": "broker port, qos, and timeout must be numeric"}
    if timeout <= 0:
        return {"status": "error", "message": "timeout must be greater than zero"}
    if qos not in (0, 1, 2):
        return {"status": "error", "message": "qos must be 0, 1, or 2"}
    if action not in ("publish", "subscribe"):
        return {"status": "error", "message": f"Unknown action: {action}. Use: publish, subscribe"}
    if action == "publish" and payload is None:
        return {"status": "error", "message": "Payload is required for publish action"}

    mqtt_user = os.environ.get("AURAGO_SECRET_MQTT_USER", "")
    mqtt_password = os.environ.get("AURAGO_SECRET_MQTT_PASSWORD", "")

    retain = bool(retain)
    connected = threading.Event()
    subscribed = threading.Event()
    connection_rc = {"value": None}
    suback_codes = {"value": None}
    client = None
    loop_started = False
    subscription_requested = False
    success_rc = 0

    def numeric_rc(value):
        if value is None:
            return None
        try:
            return int(value)
        except (TypeError, ValueError):
            try:
                return int(getattr(value, "value"))
            except (TypeError, ValueError, AttributeError):
                return None

    success_rc = numeric_rc(getattr(mqtt, "MQTT_ERR_SUCCESS", 0))
    if success_rc is None:
        success_rc = 0

    try:
        client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2)
        if mqtt_user and mqtt_password:
            client.username_pw_set(mqtt_user, mqtt_password)

        def on_connect(_client, _userdata, _flags, rc, *_args):
            connection_rc["value"] = rc
            connected.set()

        def on_subscribe(_client, _userdata, _mid, granted_qos, *_args):
            suback_codes["value"] = list(granted_qos or [])
            subscribed.set()

        client.on_connect = on_connect
        client.on_subscribe = on_subscribe
        rc = client.connect(broker_host, broker_port, keepalive=60)
        if numeric_rc(rc) != success_rc:
            return {"status": "error", "message": f"MQTT connect failed (rc={rc})"}
        client.loop_start()
        loop_started = True
        if not connected.wait(timeout):
            return {"status": "error", "message": "MQTT CONNACK timed out"}
        conn_rc = connection_rc["value"]
        if numeric_rc(conn_rc) != success_rc:
            return {"status": "error", "message": f"MQTT CONNACK rejected (rc={conn_rc})"}

        if action == "publish":
            if not isinstance(payload, str):
                payload = json.dumps(payload)
            result = client.publish(topic, payload, qos=qos, retain=retain)
            publish_rc = getattr(result, "rc", None)
            if numeric_rc(publish_rc) != success_rc:
                return {"status": "error", "message": f"MQTT publish failed (rc={publish_rc})"}
            result.wait_for_publish(timeout=timeout)
            if not result.is_published():
                return {"status": "error", "message": "MQTT publish did not complete before timeout"}
            return {
                "status": "success",
                "result": {
                    "action": "publish",
                    "topic": topic,
                    "broker": f"{broker_host}:{broker_port}",
                    "qos": qos,
                    "retained": retain,
                    "payload_size": len(payload),
                },
            }

        elif action == "subscribe":
            messages = []

            def on_message(client, userdata, msg):
                if len(messages) < 50:
                    messages.append({"topic": msg.topic, "payload": msg.payload.decode("utf-8", errors="replace"), "qos": msg.qos})

            client.on_message = on_message
            subscribe_result = client.subscribe(topic, qos=qos)
            subscribe_rc = subscribe_result[0] if isinstance(subscribe_result, tuple) else getattr(subscribe_result, "rc", None)
            if numeric_rc(subscribe_rc) != success_rc:
                return {"status": "error", "message": f"MQTT subscribe failed (rc={subscribe_rc})"}
            subscription_requested = True
            if not subscribed.wait(timeout):
                return {"status": "error", "message": "MQTT SUBACK timed out"}
            granted = suback_codes["value"] or []
            if len(granted) != 1 or numeric_rc(granted[0]) not in (0, 1, 2) or numeric_rc(granted[0]) > qos:
                return {"status": "error", "message": f"MQTT subscription rejected (SUBACK={granted})"}

            deadline = time.time() + timeout
            while time.time() < deadline:
                time.sleep(min(0.1, max(0.0, deadline - time.time())))

            return {
                "status": "success",
                "result": {
                    "action": "subscribe",
                    "topic": topic,
                    "broker": f"{broker_host}:{broker_port}",
                    "messages_received": len(messages),
                    "messages": messages[:50],
                },
            }

    except Exception as e:
        return {"status": "error", "message": str(e)}
    finally:
        if client is not None and subscription_requested:
            try:
                client.unsubscribe(topic)
            except Exception:
                pass
        if client is not None and loop_started:
            try:
                client.loop_stop()
            except Exception:
                pass
        if client is not None:
            try:
                client.disconnect()
            except Exception:
                pass
