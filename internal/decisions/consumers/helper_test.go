package consumers

import (
	"strings"
	"testing"

	"aurago/internal/decisions"
)

func TestHelperRequestFitsRejectsUnqualifiedShapes(t *testing.T) {
	valid := decisions.Request{State: "bounded", Questions: []decisions.Question{{ID: "q", Type: "noul"}}}
	if !HelperRequestFits(Router, valid) {
		t.Fatal("bounded routing request refused")
	}
	for _, mutate := range []func(*decisions.Request){
		func(r *decisions.Request) { r.State = strings.Repeat("x", 24001) },
		func(r *decisions.Request) {
			r.Questions = append(r.Questions, decisions.Question{ID: "q2", Type: "noul"})
		},
		func(r *decisions.Request) { r.Images = []decisions.Image{{MIME: "image/png"}} },
	} {
		r := valid
		mutate(&r)
		if HelperRequestFits(Router, r) {
			t.Fatal("unqualified shape accepted")
		}
	}
}

func TestValidateHelperResponseRejectsPartialAndOutOfRange(t *testing.T) {
	request := decisions.Request{Questions: []decisions.Question{{ID: "m1", Type: "score"}, {ID: "durable", Type: "noul"}}}
	score, p := 9.0, .9
	response := decisions.Response{Answers: map[string]decisions.Answer{"m1": {Score: &score}, "durable": {Noul: &p, Probabilities: map[string]float64{"yes": p, "no": 1 - p}}}}
	if err := ValidateHelperResponse(request, response); err != nil {
		t.Fatal(err)
	}
	score = 11
	if err := ValidateHelperResponse(request, response); err == nil {
		t.Fatal("out-of-range score accepted")
	}
	score = 9
	delete(response.Answers, "durable")
	if err := ValidateHelperResponse(request, response); err == nil {
		t.Fatal("partial response accepted")
	}
}
