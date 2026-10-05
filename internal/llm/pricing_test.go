package llm

import "testing"

func TestStaticPricingForModelUsesExactThenLongestMatch(t *testing.T) {
	tests := []struct {
		model      string
		wantInput  float64
		wantOutput float64
	}{
		{model: "gpt-4o-mini", wantInput: 0.15, wantOutput: 0.60},
		{model: "gpt-4o-mini-2024-07-18", wantInput: 0.15, wantOutput: 0.60},
		{model: "gpt-4o-2024-08-06", wantInput: 2.50, wantOutput: 10.00},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got, ok := StaticPricingForModel("openai", tt.model)
			if !ok {
				t.Fatal("expected static price")
			}
			if got.InputPerMillion != tt.wantInput || got.OutputPerMillion != tt.wantOutput {
				t.Fatalf("price = %.2f/%.2f, want %.2f/%.2f", got.InputPerMillion, got.OutputPerMillion, tt.wantInput, tt.wantOutput)
			}
		})
	}
}
