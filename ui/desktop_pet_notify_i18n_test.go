package ui

import (
	"strings"
	"testing"
)

func TestDesktopPetNotifyI18n(t *testing.T) {
	t.Parallel()

	picker := readDesktopAssetText(t, "js/desktop/apps/pet-picker.js")
	if strings.Count(picker, "t('desktop.request_failed')") < 4 {
		t.Fatal("pet picker load/activate/settings/import must localize desktop.request_failed")
	}
	if strings.Contains(picker, "message: err.message") {
		t.Fatal("pet picker notifies still dump err.message")
	}
	if !strings.Contains(picker, "t('desktop.pet_import_invalid')") {
		t.Fatal("pet picker must keep desktop.pet_import_invalid for a bad ZIP name")
	}

	runtime := readDesktopAssetText(t, "js/desktop/core/pet-runtime.js")
	if !strings.Contains(runtime, "message: err.message") {
		t.Fatal("pet runtime setting toast must stay unchanged in this wave")
	}
}
