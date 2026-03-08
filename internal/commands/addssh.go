package commands

import (
	"fmt"
	"strconv"
	"strings"

	"aurago/internal/services"
)

// AddSSHCommand registers a new server into the inventory and vault via slash command.
type AddSSHCommand struct{}

func (c *AddSSHCommand) Execute(args []string, ctx Context) (string, error) {
	params := make(map[string]string)
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 {
			params[parts[0]] = parts[1]
		}
	}

	host := params["host"]
	user := params["user"]
	pass := params["pass"]
	keypath := params["keypath"]
	tagsStr := params["tags"]
	portStr := params["port"]
	ip := params["ip"]

	if host == "" || user == "" {
		return "❌ Fehler: 'host' und 'user' sind erforderlich. Beispiel: `/addssh host=1.2.3.4 user=root pass=secret`", nil
	}

	if pass == "" && keypath == "" {
		return "❌ Fehler: Entweder 'pass' oder 'keypath' muss angegeben werden.", nil
	}

	port := 22
	if portStr != "" {
		p, err := strconv.Atoi(portStr)
		if err == nil {
			port = p
		}
	}

	tags := services.ParseTags(tagsStr)

	id, err := services.RegisterDevice(ctx.InventoryDB, ctx.Vault, host, "server", ip, port, user, pass, keypath, "", tags, "")
	if err != nil {
		return "", fmt.Errorf("registrierung fehlgeschlagen: %w", err)
	}

	return fmt.Sprintf("✅ Server %s erfolgreich registriert mit ID: %s", host, id), nil
}

func (c *AddSSHCommand) Help() string {
	return "Registriert einen neuen SSH-Server. Syntax: /addssh host=NAME user=USER [ip=IP] [pass=PASS|keypath=PATH] [port=22] [tags=tag1,tag2]"
}

func init() {
	Register("addssh", &AddSSHCommand{})
}
