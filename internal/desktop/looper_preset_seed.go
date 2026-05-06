package desktop

// DefaultLooperPresets returns the built-in example presets.
func DefaultLooperPresets() []LooperPreset {
	return []LooperPreset{
		{
			Name:     "Story Iteration",
			IsBuiltin: true,
			Prepare:  "Create a file with a fantasy short story with an emotionally moving plot.",
			Plan:     "Read the story again and judge it from a critical viewpoint. Create an internal list of the things that should be changed.",
			Action:   "Adjust the story according to the plan.",
			Test:     "Give the story a rating from 1 to 10.",
			ExitCond: "Are there no more criticisms and is the rating 10? Answer only true or false.",
			Finish:   "Open the Writer app on the Desktop and show the result to the User.",
			MaxIter:  20,
		},
	}
}
