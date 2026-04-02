package agent

type sqlQueryArgs struct {
	Operation      string
	ConnectionName string
	SQLQuery       string
	TableName      string
}

type mqttArgs struct {
	Topic   string
	Payload string
	QoS     int
	Retain  bool
	Limit   int
}

func decodeSQLQueryArgs(tc ToolCall) sqlQueryArgs {
	return sqlQueryArgs{
		Operation:      firstNonEmptyToolString(tc.Operation, toolArgString(tc.Params, "operation")),
		ConnectionName: firstNonEmptyToolString(tc.ConnectionName, toolArgString(tc.Params, "connection_name")),
		SQLQuery:       firstNonEmptyToolString(tc.SQLQuery, toolArgString(tc.Params, "sql_query")),
		TableName:      firstNonEmptyToolString(tc.TableName, toolArgString(tc.Params, "table_name")),
	}
}

func decodeMQTTArgs(tc ToolCall) mqttArgs {
	payload := firstNonEmptyToolString(tc.Payload, toolArgString(tc.Params, "payload"))
	if payload == "" {
		payload = firstNonEmptyToolString(tc.Message, toolArgString(tc.Params, "message"))
	}
	if payload == "" {
		payload = firstNonEmptyToolString(tc.Content, toolArgString(tc.Params, "content"))
	}
	req := mqttArgs{
		Topic:   firstNonEmptyToolString(tc.Topic, toolArgString(tc.Params, "topic")),
		Payload: payload,
		QoS:     firstNonEmptyInt(tc.QoS, toolArgInt(tc.Params, 0, "qos")),
		Retain:  tc.Retain,
		Limit:   firstNonEmptyInt(tc.Limit, toolArgInt(tc.Params, 0, "limit")),
	}
	if retain, ok := toolArgBool(tc.Params, "retain"); ok {
		req.Retain = retain
	}
	return req
}
