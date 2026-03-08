package commands

// BudgetCommand shows the current budget status.
type BudgetCommand struct{}

func (c *BudgetCommand) Execute(args []string, ctx Context) (string, error) {
	if ctx.BudgetTracker == nil {
		return "💰 Budget-Tracking ist nicht aktiviert.", nil
	}

	lang := "de"
	if len(args) > 0 && (args[0] == "en" || args[0] == "english") {
		lang = "en"
	}

	return ctx.BudgetTracker.FormatStatusText(lang), nil
}

func (c *BudgetCommand) Help() string {
	return "Zeigt den aktuellen Budget-Status (Tageskosten, Limit, Modelle)."
}

func init() {
	Register("budget", &BudgetCommand{})
}
