import sys
import json
import os

def {{.FunctionName}}(action, topic, payload=None, qos=0, retain=False, timeout=5):
    """{{.Description}}"""
    if not topic:
        return {"status": "error", "message": "MQTT topic is required"}

    try:
        import paho.mqtt.client as mqtt
    except ImportError:
        return {"status": "error", "message": "paho-mqtt not installed. Add 'paho-mqtt' to dependencies."}

    broker_host = os.environ.get("AURAGO_SECRET_MQTT_HOST", os.environ.get("AURAGO_SECRET_BROKER_HOST", "localhost"))
    broker_port = int(os.environ.get("AURAGO_SECRET_MQTT_PORT", "1883"))
    mqtt_user = os.environ.get("AURAGO_SECRET_MQTT_USER", "")
    mqtt_password = os.environ.get("AURAGO_SECRET_MQTT_PASSWORD", "")

    qos = int(qos)
    retain = bool(retain)
    timeout = int(timeout)

    client = mqtt.Client(mqtt.CallbackAPIVersion.VERSION2)
    if mqtt_user and mqtt_password:
        client.username_pw_set(mqtt_user, mqtt_password)

    try:
        client.connect(broker_host, broker_port, keepalive=60)
        client.loop_start()

        if action == "publish":
            if payload is None:
                return {"status": "error", "message": "Payload is required for publish action"}
            if not isinstance(payload, str):
                payload = json.dumps(payload)
            result = client.publish(topic, payload, qos=qos, retain=retain)
            result.wait_for_publish(timeout=timeout)
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
                messages.append({"topic": msg.topic, "payload": msg.payload.decode("utf-8", errors="replace"), "qos": msg.qos})

            client.on_message = on_message
            client.subscribe(topic, qos=qos)

            import time
            deadline = time.time() + timeout
            while time.time() < deadline:
                time.sleep(0.1)

            client.unsubscribe(topic)
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

        else:
            return {"status": "error", "message": f"Unknown action: {action}. Use: publish, subscribe"}

    except Exception as e:
        return {"status": "error", "message": str(e)}
    finally:
        client.loop_stop()
        client.disconnect()
