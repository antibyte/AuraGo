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
	enabled, _ := toolArgBool(tc.Params, "enabled")
	req := fritzBoxArgs{
		Action:         firstNonEmptyToolString(tc.Action, toolArgString(tc.Params, "action")),
		Operation:      firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		Enabled:        enabled,
		WLANIndex:      toolArgInt(tc.Params, 0, "wlan_index"),
		MACAddress:     toolArgString(tc.Params, "mac_address"),
		ExternalPort:   toolArgString(tc.Params, "external_port"),
		InternalPort:   toolArgString(tc.Params, "internal_port"),
		InternalClient: toolArgString(tc.Params, "internal_client"),
		Protocol:       toolArgString(tc.Params, "protocol"),
		Description:    toolArgString(tc.Params, "description"),
		PhonebookID:    toolArgInt(tc.Params, 0, "phonebook_id"),
		TamIndex:       toolArgInt(tc.Params, 0, "tam_index"),
		MsgIndex:       toolArgInt(tc.Params, 0, "msg_index"),
		AIN:            toolArgString(tc.Params, "ain"),
		TempC:          toolArgFloat64(tc.Params, "temp_c"),
		Brightness:     toolArgInt(tc.Params, 0, "brightness"),
	}
	return req
}
