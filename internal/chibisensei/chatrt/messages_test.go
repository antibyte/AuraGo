package chatrt

import (
	"reflect"
	"testing"
)

func TestPrepareMessages(t *testing.T) {
	cases := []struct {
		name string
		role string
		in   []Message
		want []Message
	}{
		{"merge", "merge_into_first_user",
			[]Message{{"system", "Be brief."}, {"user", "Hi"}, {"assistant", "Hello"}, {"user", "More"}},
			[]Message{{"user", "Be brief.\n\nHi"}, {"assistant", "Hello"}, {"user", "More"}}},
		{"native keeps the system message", "native",
			[]Message{{"system", "Be brief."}, {"user", "Hi"}},
			[]Message{{"system", "Be brief."}, {"user", "Hi"}}},
		{"no system", "merge_into_first_user",
			[]Message{{"user", "Hi"}}, []Message{{"user", "Hi"}}},
		{"empty system is dropped", "merge_into_first_user",
			[]Message{{"system", ""}, {"user", "Hi"}}, []Message{{"user", "Hi"}}},
		{"empty input", "merge_into_first_user", nil, []Message{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := append([]Message(nil), tc.in...)
			got := PrepareMessages(tc.role, tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if !reflect.DeepEqual(tc.in, before) && !(len(tc.in) == 0 && len(before) == 0) {
				t.Fatalf("input changed: %v", tc.in)
			}
			if len(got) > 0 && len(tc.in) > 0 && &got[0] == &tc.in[0] {
				t.Fatal("the result aliases the input")
			}
		})
	}
}
