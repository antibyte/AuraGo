package storymaker

import (
	"context"
	_ "embed"
)

//go:embed reference/animation.md
var animationReference string

//go:embed reference/code.md
var codeReference string

//go:embed reference/effects.js
var codeReferenceExample string

//go:embed reference/svg-assets.js
var svgReferenceExample string

func init() {
	registerStudioOp(ToolCatalog, "reference", func(_ context.Context, _ *Studio, args map[string]any) StudioResult {
		topic := studioArgString(args, "topic")
		if topic == "" {
			topic = "animation"
		}
		var guide string
		switch topic {
		case "animation":
			guide = animationReference
		case "code":
			guide = codeReference + "\n\n## Motion example\n\n```javascript\n" + codeReferenceExample + "\n```\n" +
				"\n## SVG asset example\n\nA custom charge controller between catalog solar-panel and battery props. Off-screen narration; adapt labels and reveal times to the story.\n\n```javascript\n" + svgReferenceExample + "\n```\n"
		default:
			return studioRefused("invalid_data", "Unknown reference topic.", "Use topic animation or code.")
		}
		return studioOK(map[string]any{"topic": topic, "version": "storymaker-reference/2", "guide": guide})
	})
}
