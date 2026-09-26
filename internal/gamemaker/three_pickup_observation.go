package gamemaker

import "strings"

const legacyThreePickupMetric = "pickup_events:builder?sceneState.pickup_events:0"

// Add observation only at real collection branches of two known legacy helpers.
// Verify the entire template, including the accepted plan's exact asset bindings;
// a version marker or a matching fragment never authorizes instrumentation.
// Compilation leaves installed source, game rules and authored helpers intact.
func upgradeThreePickupObservation(source string, plan *GamePlan) string {
	if plan == nil || !strings.Contains(source, legacyThreePickupMetric) || (!guided3D(plan.Template) && plan.Template != "three") {
		return source
	}
	expected, err := threeTemplateSources(*plan)
	if err != nil {
		return source
	}
	normalize := func(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
	common := normalize(string(expected["common.ts"]))
	const begin = "import mechanicsPlan from './mechanics.json';\n"
	const end = "const T = A.THREE;"
	start, stop := strings.Index(common, begin), strings.Index(common, end)
	if start < 0 || stop <= start+len(begin) {
		return source
	}
	bindings := common[start+len(begin) : stop]
	canonical := normalize(source)
	if strings.Count(canonical, bindings) != 1 {
		return source
	}
	canonical = strings.Replace(canonical, bindings, "// PLAN_MODEL_IMPORTS\nconst roles: any = {};\n", 1)
	switch sourceHash(canonical) {
	case "666c8377de463dcfccc64d0f0ece81ce3865568122b5539a61e1a28be639f061": // 0f5d4b125: original feedback/lifecycle helper.
	case "47dd01a1d69778c5b1d1f1ab3922956c5ac5f72a59b7230b6ef1f64142947a56": // 6040144e3: three-2 cover helper, before pickup fix.
	default:
		return source
	}
	changes := [][2]string{
		{"let time=0,score=0,hits=0,actions=0", "let time=0,score=0,hits=0,pickups=0,actions=0"},
		{"time=score=hits=actions=reloads=0", "time=score=hits=pickups=actions=reloads=0"},
		{"mesh.visible=false;score++;hits++;if(o.role==='cargo')", "mesh.visible=false;score++;hits++;pickups++;if(o.role==='cargo')"},
		{"if(config.mode==='flight'){flow.event('pickup',mesh.position.toArray());mesh.visible=false;score++;hits++;", "if(config.mode==='flight'){flow.event('pickup',mesh.position.toArray());mesh.visible=false;score++;hits++;pickups++;"},
		{legacyThreePickupMetric, "pickup_events:builder?sceneState.pickup_events:pickups"},
	}
	upgraded := source
	for _, change := range changes {
		if strings.Count(upgraded, change[0]) != 1 {
			return source
		}
		upgraded = strings.Replace(upgraded, change[0], change[1], 1)
	}
	return upgraded
}
