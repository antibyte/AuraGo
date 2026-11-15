package gamemaker

import (
	"fmt"
	"strings"
)

var planPlaceholderMarkers = []string{
	"Replace with",
	"Describe score",
	"Describe defeat",
	"Describe victory",
	"Choose an exact",
}

func rejectDesignPlaceholders(d GameDesign) error {
	check := func(field, value string) error {
		for _, marker := range planPlaceholderMarkers {
			if strings.Contains(value, marker) {
				return fmt.Errorf("design.%s: replace the example text (%q) with concrete content for this game", field, marker)
			}
		}
		return nil
	}
	if err := check("objective", d.Objective); err != nil {
		return err
	}
	for i, feature := range d.Features {
		if err := check(fmt.Sprintf("features[%d]", i), feature); err != nil {
			return err
		}
	}
	for i, stage := range d.Stages {
		if err := check(fmt.Sprintf("stages[%d]", i), stage); err != nil {
			return err
		}
	}
	for i, asset := range d.Assets {
		if err := check(fmt.Sprintf("assets[%d].fallback", i), asset.Fallback); err != nil {
			return err
		}
	}
	return nil
}
