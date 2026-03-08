package commands

import "fmt"

// SudoPwdCommand stores or removes the sudo password in the vault.
type SudoPwdCommand struct{}

func (c *SudoPwdCommand) Execute(args []string, ctx Context) (string, error) {
	if len(args) == 0 {
		return "❌ Bitte Passwort angeben: `/sudopwd <passwort>`\nZum Löschen: `/sudopwd --clear`", nil
	}

	if args[0] == "--clear" {
		if err := ctx.Vault.WriteSecret("sudo_password", ""); err != nil {
			return "", fmt.Errorf("fehler beim Löschen: %w", err)
		}
		return "🗑️ Sudo-Passwort wurde aus dem Vault entfernt.", nil
	}

	password := args[0]
	if len(password) == 0 {
		return "❌ Passwort darf nicht leer sein.", nil
	}

	if err := ctx.Vault.WriteSecret("sudo_password", password); err != nil {
		return "", fmt.Errorf("vault-Fehler: %w", err)
	}

	hint := ""
	if !ctx.Cfg.Agent.SudoEnabled {
		hint = "\n⚠️ Hinweis: `agent.sudo_enabled` ist noch deaktiviert. Aktiviere es in der Config, damit der Agent das Tool nutzen kann."
	}

	return fmt.Sprintf("✅ Sudo-Passwort erfolgreich im Vault gespeichert.%s", hint), nil
}

func (c *SudoPwdCommand) Help() string {
	return "Speichert das sudo-Passwort im Vault für das execute_sudo Tool. Syntax: /sudopwd <passwort> | --clear"
}

func init() {
	Register("sudopwd", &SudoPwdCommand{})
}
