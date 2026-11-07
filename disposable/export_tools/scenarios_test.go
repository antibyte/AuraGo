package main

import (
	"fmt"
	"testing"
)

func TestNoCallVariantsStayBalancedAndDistinctAcrossCatalogSizes(t *testing.T) {
	for _, toolCount := range []int{240, 243, 244, 245, 250} {
		t.Run(fmt.Sprint(toolCount), func(t *testing.T) {
			counts := [5]int{}
			seen := make(map[[2]int]bool)
			for index := 0; index < 500; index++ {
				variant := noCallVariant(index, toolCount)
				counts[variant]++
				key := [2]int{index % toolCount, variant}
				if seen[key] {
					t.Fatalf("tool %d repeats negative variant %d at row %d", key[0], variant, index)
				}
				seen[key] = true
			}
			for variant, count := range counts {
				if count != 100 {
					t.Errorf("negative variant %d has %d rows, want 100", variant, count)
				}
			}
		})
	}
}
