package cydcreator

import "testing"

func TestContractLimits(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"MaxLayoutBytes", MaxLayoutBytes, 1048576},
		{"MaxScreens", MaxScreens, 16},
		{"MaxWidgetsPerScreen", MaxWidgetsPerScreen, 200},
		{"MaxNesting", MaxNesting, 4},
		{"MaxTextBytes", MaxTextBytes, 1024},
		{"MaxHintRunes", MaxHintRunes, 500},
		{"MaxAppFiles", MaxAppFiles, 64},
		{"MaxAppFileBytes", MaxAppFileBytes, 262144},
		{"MaxAppTotalBytes", MaxAppTotalBytes, 2097152},
		{"AssetBudget4MB", AssetBudget4MB, 262144},
		{"AssetBudgetLarge", AssetBudgetLarge, 4194304},
		{"MaxImportBytes", MaxImportBytes, 20971520},
		{"MaxImportEntries", MaxImportEntries, 500},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestAssetBudget(t *testing.T) {
	cases := map[int]int64{4: 256 << 10, 8: 4 << 20, 16: 4 << 20, 2: 256 << 10, 0: 256 << 10}
	for flash, want := range cases {
		if got := AssetBudget(flash); got != want {
			t.Errorf("AssetBudget(%d) = %d, want %d", flash, got, want)
		}
	}
}
