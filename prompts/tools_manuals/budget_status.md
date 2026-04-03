# budget_status — AI Budget and Token Tracking

Check the token usage, costs, and current budget limits for the agent.

## Operations

| Operation | Description | Required Params |
|-----------|-------------|-----------------|
| `current` | Get current spending and remaining budget | (none) |
| `history` | List usage sorted by day or hour | (optional: `period`="daily"|"hourly") |
| `model_stats`| Show token costs grouped by LLM model | (none) |

## Key Behaviors

- Relies on internal pricing tables and tracked token counts.
- Displays spending in the configured base currency (usually USD).
- Helps the agent remain autonomous responsibly without overspending.

## Examples

```
# Check if we are near the budget limit
budget_status(operation="current")

# Review spending over the last few days
budget_status(operation="history", period="daily")

# See which model is using the most budget
budget_status(operation="model_stats")
```

## Tips
- Always respect tight budgets. If `current` shows you are close to the limit, proactively ask the user before continuing token-heavy tasks (like full codebase analysis).