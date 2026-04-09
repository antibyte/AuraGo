package commands

import (
	"fmt"
	"strconv"
	"strings"

	"aurago/internal/i18n"
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
		return i18n.T(ctx.Lang, "backend.addssh_host_user_required"), nil
	}

	if pass == "" && keypath == "" {
		return i18n.T(ctx.Lang, "backend.addssh_pass_or_keypath_required"), nil
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

	return i18n.T(ctx.Lang, "backend.addssh_success", host, id), nil
}

func (c *AddSSHCommand) Help() string {
	return i18n.T("de", "backend.addssh_help")
}

func init() {
	Register("addssh", &AddSSHCommand{})
}
