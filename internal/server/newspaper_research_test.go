package server

import (
	"reflect"
	"testing"

	"aurago/internal/newspaper"
)

func TestNewspaperCandidateOrderBalancesSections(t *testing.T) {
	p := newspaper.DefaultProfile()
	p.Sections = []string{"science", "culture"}
	p.Interests = []string{"local theatre", "energy policy"}
	queries := []newspaperQuery{
		{Section: "science"}, {Section: "culture"},
		{Section: "interests"}, {Section: "interests"},
		{Section: "science"}, {Section: "science"}, {Section: "culture"},
	}
	got := newspaperCandidateOrder(p, queries, 4)
	want := []int{4, 6, 2, 5, 1, 3, 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("candidate order = %v, want %v", got, want)
	}
}
