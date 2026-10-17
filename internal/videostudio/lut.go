package videostudio

// LUT assets and built-in looks (plan 2c, spec §5.7, plan index §4.20). Plan 2c Task 1 adds the limits that validation
// needs; Task 4 adds the .cube parser, sampler and writer and the built-in looks to this file.
const (
	// MaxLUTBytes bounds an uploaded .cube file.
	MaxLUTBytes = 16 << 20
	// MinLUTGrid and MaxLUTGrid bound the LUT_3D_SIZE of a .cube file (Asset.LUTSize).
	MinLUTGrid = 2
	MaxLUTGrid = 65
	// MaxLUTAssetsPerProject bounds the lut assets of one project.
	MaxLUTAssetsPerProject = 32
	// BuiltinLookGrid is the grid size of the generated built-in looks.
	BuiltinLookGrid = 33
)
