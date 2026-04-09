package commands

import (
	"fmt"

	"aurago/internal/i18n"
)

// SudoPwdCommand stores or removes the sudo password in the vault.
type SudoPwdCommand struct{}

func (c *SudoPwdCommand) Execute(args []string, ctx Context) (string, error) {
	if len(args) == 0 {
		return i18n.T(ctx.Lang, "backend.sudopwd_usage"), nil
	}

	if args[0] == "--clear" {
		if err := ctx.Vault.WriteSecret("sudo_password", ""); err != nil {
			return "", fmt.Errorf("fehler beim Löschen: %w", err)
		}
		return i18n.T(ctx.Lang, "backend.sudopwd_cleared"), nil
	}

	password := args[0]
	if len(password) == 0 {
		return i18n.T(ctx.Lang, "backend.sudopwd_empty"), nil
	}

	if err := ctx.Vault.WriteSecret("sudo_password", password); err != nil {
		return "", fmt.Errorf("vault-Fehler: %w", err)
	}

	hint := ""
	if !ctx.Cfg.Agent.SudoEnabled {
		hint = "\n" + i18n.T(ctx.Lang, "backend.sudopwd_hint")
	}

	return i18n.T(ctx.Lang, "backend.sudopwd_success") + hint, nil
}

func (c *SudoPwdCommand) Help() string {
	return i18n.T("de", "backend.sudopwd_help")
}

func init() {
	Register("sudopwd", &SudoPwdCommand{})
}
