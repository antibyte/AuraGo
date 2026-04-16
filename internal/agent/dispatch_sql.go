package agent

import (
	"context"
	"encoding/json"
	"time"

	"aurago/internal/sqlconnections"
)

type safeSQLConnection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Driver      string `json:"driver"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Database    string `json:"database_name"`
	Description string `json:"description"`
	AllowRead   bool   `json:"allow_read"`
	AllowWrite  bool   `json:"allow_write"`
	AllowChange bool   `json:"allow_change"`
	AllowDelete bool   `json:"allow_delete"`
	SSLMode     string `json:"ssl_mode"`
}

func newSQLConnectionServiceForDispatch(dc *DispatchContext) *sqlconnections.Service {
	readOnly := false
	allowManage := false
	if dc.Cfg != nil {
		readOnly = dc.Cfg.SQLConnections.ReadOnly
		allowManage = dc.Cfg.SQLConnections.AllowManagement
	}

	return sqlconnections.NewService(sqlconnections.ServiceConfig{
		DB:          dc.SQLConnectionsDB,
		Vault:       dc.Vault,
		Pool:        dc.SQLConnectionPool,
		Logger:      dc.Logger,
		ReadOnly:    readOnly,
		AllowManage: allowManage,
	})
}

func sqlToolOutput(payload map[string]interface{}) string {
	b, _ := json.Marshal(payload)
	return "Tool Output: " + string(b)
}

func sqlToolError(message string) string {
	return sqlToolOutput(map[string]interface{}{"status": "error", "message": message})
}

func sqlToolSanitizedError(err error) string {
	return sqlToolError(sqlconnections.SanitizeError(err))
}

func handleSQLQueryTool(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	if dc.Cfg == nil || !dc.Cfg.SQLConnections.Enabled {
		return sqlToolError("SQL Connections feature is disabled. Enable sql_connections.enabled in config.")
	}
	if dc.SQLConnectionsDB == nil || dc.SQLConnectionPool == nil {
		return sqlToolError("SQL Connections database not available.")
	}

	req := decodeSQLQueryArgs(tc)
	if req.ConnectionName == "" {
		return sqlToolError("'connection_name' is required")
	}
	if dc.Logger != nil {
		dc.Logger.Info("LLM requested sql_query", "op", req.Operation, "connection", req.ConnectionName)
	}

	queryTimeout := time.Duration(dc.Cfg.SQLConnections.QueryTimeoutSec) * time.Second
	maxRows := dc.Cfg.SQLConnections.MaxResultRows

	switch req.Operation {
	case "query":
		if req.SQLQuery == "" {
			return sqlToolError("'sql_query' is required for query operation")
		}
		result, err := sqlconnections.ExecuteQuery(ctx, dc.SQLConnectionPool, dc.SQLConnectionsDB, req.ConnectionName, req.SQLQuery, maxRows, queryTimeout, dc.Cfg.SQLConnections.ReadOnly)
		if err != nil {
			return sqlToolSanitizedError(err)
		}
		return sqlToolOutput(map[string]interface{}{"status": "success", "result": result})
	case "describe":
		if req.TableName == "" {
			return sqlToolError("'table_name' is required for describe operation")
		}
		cols, err := sqlconnections.DescribeTable(ctx, dc.SQLConnectionPool, dc.SQLConnectionsDB, req.ConnectionName, req.TableName, queryTimeout)
		if err != nil {
			return sqlToolSanitizedError(err)
		}
		return sqlToolOutput(map[string]interface{}{"status": "success", "table": req.TableName, "columns": cols})
	case "list_tables":
		tables, err := sqlconnections.ListTables(ctx, dc.SQLConnectionPool, dc.SQLConnectionsDB, req.ConnectionName, queryTimeout)
		if err != nil {
			return sqlToolSanitizedError(err)
		}
		return sqlToolOutput(map[string]interface{}{"status": "success", "tables": tables, "count": len(tables)})
	default:
		return sqlToolError("Unknown operation. Use: query, describe, list_tables")
	}
}

func handleManageSQLConnectionsTool(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	_ = ctx
	if dc.Cfg == nil || !dc.Cfg.SQLConnections.Enabled {
		return sqlToolError("SQL Connections feature is disabled. Enable sql_connections.enabled in config.")
	}
	if dc.SQLConnectionsDB == nil || dc.SQLConnectionPool == nil {
		return sqlToolError("SQL Connections database not available.")
	}

	req := decodeManageSQLConnectionsArgs(tc)
	if dc.Logger != nil {
		dc.Logger.Info("LLM requested manage_sql_connections", "op", req.Operation)
	}
	service := newSQLConnectionServiceForDispatch(dc)

	switch req.Operation {
	case "list":
		list, err := service.List()
		if err != nil {
			return sqlToolSanitizedError(err)
		}
		safe := make([]safeSQLConnection, 0, len(list))
		for _, c := range list {
			safe = append(safe, safeSQLConnection{
				ID: c.ID, Name: c.Name, Driver: c.Driver,
				Host: c.Host, Port: c.Port, Database: c.DatabaseName,
				Description: c.Description, AllowRead: c.AllowRead,
				AllowWrite: c.AllowWrite, AllowChange: c.AllowChange,
				AllowDelete: c.AllowDelete, SSLMode: c.SSLMode,
			})
		}
		return sqlToolOutput(map[string]interface{}{"status": "success", "connections": safe, "count": len(safe)})

	case "get":
		if req.ConnectionName == "" {
			return sqlToolError("'connection_name' is required")
		}
		c, err := service.GetByName(req.ConnectionName)
		if err != nil {
			return sqlToolSanitizedError(err)
		}
		return sqlToolOutput(map[string]interface{}{
			"status":        "success",
			"id":            c.ID,
			"name":          c.Name,
			"driver":        c.Driver,
			"host":          c.Host,
			"port":          c.Port,
			"database_name": c.DatabaseName,
			"description":   c.Description,
			"allow_read":    c.AllowRead,
			"allow_write":   c.AllowWrite,
			"allow_change":  c.AllowChange,
			"allow_delete":  c.AllowDelete,
			"ssl_mode":      c.SSLMode,
		})

	case "create":
		if !service.CanManage() {
			return sqlToolError("SQL connection management is disabled. Administrator must enable sql_connections.allow_management in config to allow creating connections.")
		}
		if req.ConnectionName == "" || req.Driver == "" {
			return sqlToolError("'connection_name' and 'driver' are required for create")
		}

		allowRead := true
		if req.AllowRead != nil {
			allowRead = *req.AllowRead
		}
		allowWrite := false
		if req.AllowWrite != nil {
			allowWrite = *req.AllowWrite
		}
		allowChange := false
		if req.AllowChange != nil {
			allowChange = *req.AllowChange
		}
		allowDelete := false
		if req.AllowDelete != nil {
			allowDelete = *req.AllowDelete
		}

		sslMode := req.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}

		result, err := service.Create(sqlconnections.CreateRequest{
			Name:         req.ConnectionName,
			Driver:       req.Driver,
			Host:         req.Host,
			Port:         req.Port,
			DatabaseName: req.DatabaseName,
			Description:  req.Description,
			Username:     req.Username,
			Password:     req.Password,
			SSLMode:      sslMode,
			AllowRead:    allowRead,
			AllowWrite:   allowWrite,
			AllowChange:  allowChange,
			AllowDelete:  allowDelete,
		})
		if err != nil {
			return sqlToolSanitizedError(err)
		}

		return sqlToolOutput(map[string]interface{}{"status": "success", "message": "Connection created", "id": result.ID, "name": result.Name})

	case "update":
		if !service.CanManage() {
			return sqlToolError("SQL connection management is disabled. Administrator must enable sql_connections.allow_management in config to allow updating connections.")
		}
		if req.ConnectionName == "" {
			return sqlToolError("'connection_name' is required for update")
		}

		existing, err := service.GetByName(req.ConnectionName)
		if err != nil {
			return sqlToolSanitizedError(err)
		}

		allowRead := existing.AllowRead
		if req.AllowRead != nil {
			allowRead = *req.AllowRead
		}
		allowWrite := existing.AllowWrite
		if req.AllowWrite != nil {
			allowWrite = *req.AllowWrite
		}
		allowChange := existing.AllowChange
		if req.AllowChange != nil {
			allowChange = *req.AllowChange
		}
		allowDelete := existing.AllowDelete
		if req.AllowDelete != nil {
			allowDelete = *req.AllowDelete
		}

		credentialAction := req.CredentialAction
		if credentialAction == "" {
			if req.Username != "" || req.Password != "" {
				credentialAction = "replace"
			} else {
				credentialAction = "keep"
			}
		}

		updateReq := sqlconnections.UpdateRequest{
			ID:               existing.ID,
			Name:             existing.Name,
			Driver:           existing.Driver,
			Host:             existing.Host,
			Port:             existing.Port,
			DatabaseName:     existing.DatabaseName,
			Description:      existing.Description,
			SSLMode:          existing.SSLMode,
			AllowRead:        allowRead,
			AllowWrite:       allowWrite,
			AllowChange:      allowChange,
			AllowDelete:      allowDelete,
			CredentialAction: credentialAction,
		}

		if req.Driver != "" {
			updateReq.Driver = req.Driver
		}
		if req.Host != "" {
			updateReq.Host = req.Host
		}
		if req.Port > 0 {
			updateReq.Port = req.Port
		}
		if req.DatabaseName != "" {
			updateReq.DatabaseName = req.DatabaseName
		}
		if req.Description != "" {
			updateReq.Description = req.Description
		}
		if req.SSLMode != "" {
			updateReq.SSLMode = req.SSLMode
		}
		if credentialAction == "replace" {
			updateReq.Username = req.Username
			updateReq.Password = req.Password
		}

		if err := service.Update(updateReq); err != nil {
			return sqlToolSanitizedError(err)
		}
		return sqlToolOutput(map[string]interface{}{"status": "success", "message": "Connection updated", "name": req.ConnectionName})

	case "delete":
		if !service.CanManage() {
			return sqlToolError("SQL connection management is disabled. Administrator must enable sql_connections.allow_management in config to allow deleting connections.")
		}
		if req.ConnectionName == "" {
			return sqlToolError("'connection_name' is required for delete")
		}

		existing, err := service.GetByName(req.ConnectionName)
		if err != nil {
			return sqlToolSanitizedError(err)
		}
		if err := service.Delete(sqlconnections.DeleteRequest{ID: existing.ID}); err != nil {
			return sqlToolSanitizedError(err)
		}
		return sqlToolOutput(map[string]interface{}{"status": "success", "message": "Connection deleted", "name": req.ConnectionName})

	case "test":
		if req.ConnectionName == "" {
			return sqlToolError("'connection_name' is required for test")
		}

		rec, err := service.GetByName(req.ConnectionName)
		if err != nil {
			return sqlToolSanitizedError(err)
		}
		if err := service.TestConnection(rec.ID); err != nil {
			return sqlToolError("Connection test failed: " + sqlconnections.SanitizeError(err))
		}
		return sqlToolOutput(map[string]interface{}{"status": "success", "message": "Connection test successful", "name": req.ConnectionName, "driver": rec.Driver})

	case "docker_create":
		if !service.CanManage() {
			return sqlToolError("SQL connection management is disabled. Administrator must enable sql_connections.allow_management in config to allow creating connections via docker.")
		}
		if req.ConnectionName == "" {
			return sqlToolError("'connection_name' is required for docker_create")
		}

		templateName := req.DockerTemplate
		if templateName == "" {
			return sqlToolError("'docker_template' is required (postgres, mysql, mariadb)")
		}
		dbName := req.DatabaseName
		if dbName == "" {
			dbName = req.ConnectionName
		}

		dockerReq, err := sqlconnections.PrepareDockerDB(templateName, req.ConnectionName, dbName)
		if err != nil {
			return sqlToolSanitizedError(err)
		}
		return sqlToolOutput(map[string]interface{}{
			"status":  "success",
			"message": "Docker database prepared. Use the 'docker' tool with operation 'run' to start the container, then create the connection with 'manage_sql_connections' create.",
			"docker":  dockerReq,
		})

	default:
		return sqlToolError("Unknown operation. Use: list, get, create, update, delete, test, docker_create")
	}
}
