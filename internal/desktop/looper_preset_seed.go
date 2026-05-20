package desktop

// DefaultLooperPresets returns the built-in example presets.
func DefaultLooperPresets() []LooperPreset {
	return []LooperPreset{
		{
			Name:     "Story Iteration",
			IsBuiltin: true,
			Prepare:  "Create a file with a fantasy short story with an emotionally moving plot.",
			Plan:     "Read the story again and judge it from a critical viewpoint. List the things that should be changed.",
			Action:   "Adjust the story according to the plan.",
			Test:     "Give the story a rating from 1 to 10.",
			ExitCond: "Are there no more criticisms and is the rating 10? Answer only true or false.",
			Finish:   "Open the Writer app on the Desktop and show the result to the User.",
			MaxIter:  20,
		},
		{
			Name:     "Code Refinement",
			IsBuiltin: true,
			Prepare:  "Read the most recently created or modified source file in the workspace and summarize what it does.",
			Plan:     "Analyze the code for bugs, performance issues, missing error handling, and style problems. List all issues found.",
			Action:   "Fix every issue identified in the plan. Apply the changes to the file.",
			Test:     "Review the updated code. Rate the code quality from 1 to 10 where 10 means production-ready with no issues.",
			ExitCond: "Is the code quality rating 9 or higher with no remaining issues? Answer only true or false.",
			Finish:   "Summarize all changes made to the file.",
			MaxIter:  10,
		},
		{
			Name:     "Document Review",
			IsBuiltin: true,
			Prepare:  "Read the document and understand its structure and content.",
			Plan:     "Evaluate the document for clarity, grammar, completeness, and logical flow. Identify specific areas that need improvement.",
			Action:   "Rewrite the document incorporating all improvements identified in the plan.",
			Test:     "Rate the document quality from 1 to 10 for clarity, professionalism, and completeness.",
			ExitCond: "Is the document quality rating 9 or higher? Answer only true or false.",
			Finish:   "Present the final polished document to the user.",
			MaxIter:  8,
		},
		{
			Name:     "Iterative Testing",
			IsBuiltin: true,
			Prepare:  "Identify the main application entry point and list all source files in the project.",
			Plan:     "Analyze the codebase and design comprehensive tests for the most critical functions. Focus on edge cases and error paths.",
			Action:   "Write the tests and run them. Fix any compilation errors in the test file.",
			Test:     "Run all tests and report how many pass vs fail.",
			ExitCond: "Do all tests pass without errors? Answer only true or false.",
			Finish:   "Report the final test coverage summary to the user.",
			MaxIter:  10,
		},
		{
			Name:      "Ralph Loop",
			IsBuiltin: true,
			Prepare:   "Load the creative brief or reference material (previous audio, lyrics, mood board, or style description). Summarize the artistic intent, emotional target, and hard constraints (length, language, instruments, vibe).",
			Plan:      "Critically review the current artifact against the brief. Identify what is working emotionally, what feels generic, structural issues, and concrete improvements that would make it unmistakably 'Ralph-quality'.",
			Action:    "Execute the plan: generate or refine the artifact (music, lyrics, arrangement, or code). Use available creative tools when appropriate. Make the change bold but coherent with the reference.",
			Test:      "Evaluate the result on four axes: (1) Emotional impact, (2) Originality vs reference, (3) Technical polish, (4) 'Would Ralph be proud?'. Give a 1-10 score per axis and an overall verdict with specific praise and criticism.",
			ExitCond:  "Are all four axes >= 8 and the overall verdict 'production ready / signature Ralph'? Answer only true or false.",
			Finish:            "Export the final artifact to the desktop (Writer, Music player, Gallery, or Radio) and write a short artist note explaining the journey and the key decision that made it special. Use the final iteration result provided below.",
			FinishContext:     "last_action_test",
			PrepareTruncation:   6000, // higher budget for creative references (style, mood, previous work)
			SummarizeIterations: true, // explicit reflection after each round → much better coherence
			MaxIter:             18,
		},
	}
}
