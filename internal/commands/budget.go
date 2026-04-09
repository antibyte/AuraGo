package commands

import "aurago/internal/i18n"

// BudgetCommand shows the current budget status.
type BudgetCommand struct{}

func (c *BudgetCommand) Execute(args []string, ctx Context) (string, error) {
	if ctx.BudgetTracker == nil {
		return i18n.T(ctx.Lang, "backend.budget_disabled"), nil
	}

	lang := ctx.Lang
	if len(args) > 0 && (args[0] == "en" || args[0] == "english") {
		lang = "en"
	}

	return ctx.BudgetTracker.FormatStatusText(lang), nil
}

func (c *BudgetCommand) Help() string {
	return i18n.T("de", "backend.budget_help")
}

func init() {
	Register("budget", &BudgetCommand{})
}
