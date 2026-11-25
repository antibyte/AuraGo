package hasstest

import "time"

// DemoTime is the last_updated of every demo state.
var DemoTime = time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)

// demoJPEG is a minimal JPEG start and end marker; enough for content sniffing.
var demoJPEG = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01, 0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9}

// DemoOptions is a small home that covers every W1 domain, areas from the entity and from the device
// registry, a hidden entity, a policy-blocked script, a non-W1 sensor, an excluded alarm panel, a
// camera and recorder history. The M3 contract examples are taken from it. Each call returns fresh
// maps and slices.
func DemoOptions() Options {
	st := func(id, state string, attrs map[string]any) State {
		return State{EntityID: id, State: state, Attributes: attrs, LastChanged: DemoTime, LastUpdated: DemoTime}
	}
	return Options{
		States: []State{
			st("light.kitchen", "on", map[string]any{"friendly_name": "Kitchen", "brightness": 102.0, "color_mode": "color_temp",
				"color_temp_kelvin": 3003.0, "min_color_temp_kelvin": 2202.0, "max_color_temp_kelvin": 6535.0, "hs_color": []any{27.2, 56.1},
				"supported_color_modes": []any{"color_temp", "hs"}, "effect": "none", "effect_list": []any{"none", "colorloop"},
				"supported_features": 44.0, "icon": "mdi:ceiling-light", "entity_picture": "/api/image/serve/abc?token=demo-secret"}),
			st("switch.coffee", "off", map[string]any{"friendly_name": "Coffee machine", "device_class": "outlet"}),
			st("select.heater_mode", "comfort", map[string]any{"friendly_name": "Heater mode", "options": []any{"eco", "comfort", "boost"}}),
			st("light.sofa", "off", map[string]any{"friendly_name": "Sofa lamp", "supported_color_modes": []any{"onoff"}, "color_mode": nil}),
			st("scene.movie", "2026-11-24T20:15:00.000000+00:00", map[string]any{"friendly_name": "Movie night"}),
			st("input_number.volume", "35.0", map[string]any{"friendly_name": "Volume", "min": 0.0, "max": 100.0, "step": 5.0, "mode": "slider"}),
			st("sensor.outdoor_temperature", "4.5", map[string]any{"friendly_name": "Outdoor", "device_class": "temperature", "unit_of_measurement": "°C", "state_class": "measurement"}),
			st("input_boolean.vacation_mode", "off", map[string]any{"friendly_name": "Vacation mode"}),
			st("input_boolean.guest_mode", "off", map[string]any{"friendly_name": "Guest mode"}),
			st("button.doorbell_test", "unknown", map[string]any{"friendly_name": "Doorbell test"}),
			st("input_select.radio", "Jazz", map[string]any{"friendly_name": "Radio", "options": []any{"Jazz", "Rock", "News", "Classic", "Pop", "Lounge"}}),
			st("number.boiler_target", "55.0", map[string]any{"friendly_name": "Boiler target", "min": 30.0, "max": 70.0, "step": 1.0, "mode": "box", "unit_of_measurement": "°C"}),
			st("script.goodnight", "off", map[string]any{"friendly_name": "Good night"}),
			st("alarm_control_panel.home", "disarmed", map[string]any{"friendly_name": "Alarm"}),
			st("camera.door", "idle", map[string]any{"friendly_name": "Door", "entity_picture": "/api/camera_proxy/camera.door?token=demo-secret", "access_token": "demo-secret"}),
		},
		Areas:   []Area{{AreaID: "kitchen", Name: "Kitchen"}, {AreaID: "living_room", Name: "Living room"}, {AreaID: "garage", Name: "Garage"}},
		Devices: []Device{{ID: "dev_kitchen_light", AreaID: "kitchen"}, {ID: "dev_sofa", AreaID: "living_room"}},
		Entities: []EntityEntry{
			{EntityID: "light.kitchen", DeviceID: "dev_kitchen_light"},                // area from the device
			{EntityID: "switch.coffee", AreaID: "kitchen"},                            // own area
			{EntityID: "select.heater_mode", DeviceID: "dev_sofa", AreaID: "kitchen"}, // own area wins over the device's
			{EntityID: "light.sofa", DeviceID: "dev_sofa"},
			{EntityID: "scene.movie", AreaID: "living_room"},
			{EntityID: "input_number.volume", AreaID: "living_room"},
			{EntityID: "sensor.outdoor_temperature", AreaID: "living_room"},
			{EntityID: "input_boolean.guest_mode", HiddenBy: "user"},
		},
		History: map[string][]HistoryPoint{
			"sensor.outdoor_temperature": {{"4.0", DemoTime.Add(-2 * time.Hour)}, {"unavailable", DemoTime.Add(-90 * time.Minute)}, {"4.5", DemoTime.Add(-time.Hour)}},
			"switch.coffee":              {{"on", DemoTime.Add(-2 * time.Hour)}, {"off", DemoTime.Add(-time.Hour)}},
		},
		Cameras: map[string][]byte{"camera.door": append([]byte(nil), demoJPEG...)},
	}
}
