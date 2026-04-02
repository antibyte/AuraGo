package agent

import "testing"

func TestDecodeFritzBoxArgsUsesParamsFallbacks(t *testing.T) {
	tc := ToolCall{
		Action: "fritzbox_network",
		Params: map[string]interface{}{
			"operation":       "add_port_forward",
			"wlan_index":      float64(2),
			"enabled":         true,
			"mac_address":     "AA:BB:CC:DD:EE:FF",
			"external_port":   "443",
			"internal_port":   "8443",
			"internal_client": "192.168.1.10",
			"protocol":        "tcp",
			"description":     "AuraGo UI",
		},
	}

	req := decodeFritzBoxArgs(tc)
	if req.Action != "fritzbox_network" {
		t.Fatalf("Action = %q, want fritzbox_network", req.Action)
	}
	if req.Operation != "add_port_forward" {
		t.Fatalf("Operation = %q, want add_port_forward", req.Operation)
	}
	if req.WLANIndex != 2 || !req.Enabled {
		t.Fatalf("unexpected wlan/enabled decode: %+v", req)
	}
	if req.MACAddress != "AA:BB:CC:DD:EE:FF" {
		t.Fatalf("MACAddress = %q", req.MACAddress)
	}
	if req.ExternalPort != "443" || req.InternalPort != "8443" || req.InternalClient != "192.168.1.10" {
		t.Fatalf("unexpected port forward decode: %+v", req)
	}
	if req.Protocol != "tcp" || req.Description != "AuraGo UI" {
		t.Fatalf("unexpected protocol/description decode: %+v", req)
	}
}

func TestDecodeFritzBoxArgsUsesTelephonyAndSmartHomeFallbacks(t *testing.T) {
	tc := ToolCall{
		Action: "fritzbox",
		Params: map[string]interface{}{
			"operation":    "transcribe_tam_message",
			"phonebook_id": float64(3),
			"tam_index":    float64(2),
			"msg_index":    float64(7),
			"ain":          "12345 6789012",
			"temp_c":       float64(21.5),
			"brightness":   float64(80),
		},
	}

	req := decodeFritzBoxArgs(tc)
	if req.PhonebookID != 3 || req.TamIndex != 2 || req.MsgIndex != 7 {
		t.Fatalf("unexpected telephony indexes: %+v", req)
	}
	if req.AIN != "12345 6789012" {
		t.Fatalf("AIN = %q, want smart home identifier", req.AIN)
	}
	if req.TempC != 21.5 {
		t.Fatalf("TempC = %v, want 21.5", req.TempC)
	}
	if req.Brightness != 80 {
		t.Fatalf("Brightness = %d, want 80", req.Brightness)
	}
}
