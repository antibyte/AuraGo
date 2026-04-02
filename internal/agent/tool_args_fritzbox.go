package agent

type fritzBoxArgs struct {
	Action         string
	Operation      string
	Enabled        bool
	WLANIndex      int
	MACAddress     string
	ExternalPort   string
	InternalPort   string
	InternalClient string
	Protocol       string
	Description    string
	PhonebookID    int
	TamIndex       int
	MsgIndex       int
	AIN            string
	TempC          float64
	Brightness     int
}

func decodeFritzBoxArgs(tc ToolCall) fritzBoxArgs {
	req := fritzBoxArgs{
		Action:         firstNonEmptyToolString(tc.Action, toolArgString(tc.Params, "action")),
		Operation:      firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Enabled:        tc.Enabled,
		WLANIndex:      firstNonEmptyInt(tc.WLANIndex, toolArgInt(tc.Params, 0, "wlan_index")),
		MACAddress:     firstNonEmptyToolString(tc.MACAddress, toolArgString(tc.Params, "mac_address")),
		ExternalPort:   firstNonEmptyToolString(tc.ExternalPort, toolArgString(tc.Params, "external_port")),
		InternalPort:   firstNonEmptyToolString(tc.InternalPort, toolArgString(tc.Params, "internal_port")),
		InternalClient: firstNonEmptyToolString(tc.InternalClient, toolArgString(tc.Params, "internal_client")),
		Protocol:       firstNonEmptyToolString(tc.Protocol, toolArgString(tc.Params, "protocol")),
		Description:    firstNonEmptyToolString(tc.Description, toolArgString(tc.Params, "description")),
		PhonebookID:    firstNonEmptyInt(tc.PhonebookID, toolArgInt(tc.Params, 0, "phonebook_id")),
		TamIndex:       firstNonEmptyInt(tc.TamIndex, toolArgInt(tc.Params, 0, "tam_index")),
		MsgIndex:       firstNonEmptyInt(tc.MsgIndex, toolArgInt(tc.Params, 0, "msg_index")),
		AIN:            firstNonEmptyToolString(tc.AIN, toolArgString(tc.Params, "ain")),
		TempC:          firstNonEmptyFloat64(tc.TempC, toolArgFloat64(tc.Params, "temp_c")),
		Brightness:     firstNonEmptyInt(tc.Brightness, toolArgInt(tc.Params, 0, "brightness")),
	}
	if enabled, ok := toolArgBool(tc.Params, "enabled"); ok {
		req.Enabled = enabled
	}
	return req
}

func firstNonEmptyFloat64(values ...float64) float64 {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}
