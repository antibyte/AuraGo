package commands

import (
	"fmt"
	"strings"

	"aurago/internal/i18n"
)

// WarningsCommand lists active system warnings.
type WarningsCommand struct{}

func (c *WarningsCommand) Execute(args []string, ctx Context) (string, error) {
	if ctx.WarningsRegistry == nil {
		return i18n.T(ctx.Lang, "backend.warnings_unavailable"), nil
	}

	all := ctx.WarningsRegistry.Warnings()
	if len(all) == 0 {
		return i18n.T(ctx.Lang, "backend.warnings_none"), nil
	}

	total, unack := ctx.WarningsRegistry.Count()
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(i18n.T(ctx.Lang, "backend.warnings_header"), total, unack) + "\n\n")

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
			ack = i18n.T(ctx.Lang, "backend.warnings_ack")
		}
		sb.WriteString(fmt.Sprintf("%s **%s**%s [%s]\n   %s\n\n", icon, w.Title, ack, w.Category, w.Description))
	}

	return sb.String(), nil
}

func (c *WarningsCommand) Help() string {
	return i18n.T("de", "backend.warnings_help")
}

func init() {
	Register("warnings", &WarningsCommand{})
}
