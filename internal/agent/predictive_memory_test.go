package agent

import "testing"

func TestBuildPredictiveMemoryQueriesPrefersQuerySignals(t *testing.T) {
	predictions := buildPredictiveMemoryQueries("Please deploy the homepage to Netlify", "homepage", []string{"daily backup", "docker logs"}, 4)
	if len(predictions) == 0 {
		t.Fatal("expected predictions")
	}
	if predictions[0] != "please deploy the homepage to netlify" {
		t.Fatalf("expected full current query to rank first, got %q", predictions[0])
	}
	foundDeployment := false
	for _, item := range predictions {
		if item == "deployment" || item == "homepage" || item == "netlify" || item == "deploy" || item == "deploy homepage netlify" {
			foundDeployment = true
			break
		}
	}
	if !foundDeployment {
		t.Fatalf("expected deployment-family hints, got %#v", predictions)
	}
}

func TestExtractPredictiveQueryHints(t *testing.T) {
	hints := extractPredictiveQueryHints("Check docker deployment logs on the server", 3)
	if len(hints) == 0 {
		t.Fatal("expected at least one predictive hint")
	}
	if hints[0] == "" {
		t.Fatal("expected non-empty hint")
	}
}
