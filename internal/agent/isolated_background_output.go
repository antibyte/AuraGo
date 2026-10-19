package agent

import "fmt"

func isolatedBackgroundStartedOutput(id string) string {
	return fmt.Sprintf("Tool Output: Isolated execution queued as background task %s. Completion, bounded output and cancellation are available in System Tasks. This task has no host PID. A completion notification is enabled.", id)
}
