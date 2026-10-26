package i18n

// WorkflowWaitMessages returns the server-owned waiting phrases for a language.
func WorkflowWaitMessages(lang string) []string {
	return []string{
		T(lang, "backend.workflow_wait"),
		T(lang, "backend.workflow_wait_2"),
		T(lang, "backend.workflow_wait_3"),
		T(lang, "backend.workflow_wait_4"),
		T(lang, "backend.workflow_wait_5"),
		T(lang, "backend.workflow_wait_6"),
	}
}
