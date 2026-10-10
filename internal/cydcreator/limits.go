package cydcreator

// Project, file and import limits (spec §3, §11, §14).
const (
	MaxLayoutBytes      = 1 << 20
	MaxScreens          = 16
	MaxWidgetsPerScreen = 200
	MaxNesting          = 4 // a widget directly on a screen has depth 1
	MaxTextBytes        = 1024
	MaxHintRunes        = 500
	MaxAppFiles         = 64
	MaxAppFileBytes     = 256 << 10
	MaxAppTotalBytes    = 2 << 20
	AssetBudget4MB      = 256 << 10
	AssetBudgetLarge    = 4 << 20
	MaxImportBytes      = 20 << 20
	MaxImportEntries    = 500
)

// Internal bounds that are not part of the cross-plan contract.
const (
	maxLayoutOps        = 200       // ops per ApplyOps call
	maxWidgetSide       = 2048      // px, any widget or asset side
	maxCoordinate       = 4096      // px, |x| and |y|
	intPropLimit        = 1_000_000 // |value| of int props without an explicit range
	canvasWarnBytes     = 64 << 10  // RGB565 canvas buffer that triggers CANVAS_LARGE without PSRAM
	maxProjectNameRunes = 64
	maxDiagnostics      = 200
	maxDiagMessageBytes = 1024
	// maxInternalDrawBuffer caps LVGL draw buffers on boards without PSRAM so
	// Wi-Fi, TLS (OTA) and WiFiManager keep enough internal heap (spec §21.5).
	maxInternalDrawBuffer = 48 << 10
)

// AssetBudget returns the decoded image budget for a board's flash size:
// 256 KiB on 4 MB boards (the app slot is ~80 % full with the template, plan 1b), 4 MiB on 8/16 MB boards.
func AssetBudget(flashMB int) int64 {
	if flashMB >= 8 {
		return AssetBudgetLarge
	}
	return AssetBudget4MB
}
