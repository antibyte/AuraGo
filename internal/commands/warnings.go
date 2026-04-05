package commands

import (
	"fmt"
	"strings"
)

// WarningsCommand lists active system warnings.
type WarningsCommand struct{}

func (c *WarningsCommand) Execute(args []string, ctx Context) (string, error) {
	if ctx.WarningsRegistry == nil {
		return "⚠️ Warnings system is not available.", nil
	}

	all := ctx.WarningsRegistry.Warnings()
	if len(all) == 0 {
		return "✅ No active warnings. Everything looks good!", nil
	}

	total, unack := ctx.WarningsRegistry.Count()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("⚠️ **System Warnings** (%d total, %d unacknowledged)\n\n", total, unack))

	for _, w := range all {
		icon := "ℹ️"
		switch w.Severity {
		case "critical":
			icon = "🔴"
		case "warning":
			icon = "🟡"
		}
		ack := ""
		if w.Acknowledged {
			ack = " ✓"
		}
		sb.WriteString(fmt.Sprintf("%s **%s**%s [%s]\n   %s\n\n", icon, w.Title, ack, w.Category, w.Description))
	}

	return sb.String(), nil
}

func (c *WarningsCommand) Help() string {
	return "Shows active system warnings and health issues."
}

func init() {
	Register("warnings", &WarningsCommand{})
}
