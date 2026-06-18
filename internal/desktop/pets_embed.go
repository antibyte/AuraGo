package desktop

import (
	_ "embed"
)

//go:embed pets_assets/openpets-default/spritesheet.webp
var defaultPetSpritesheet []byte

//go:embed pets_assets/snoopy/spritesheet.webp
var snoopyPetSpritesheet []byte

//go:embed pets_assets/clippit/spritesheet.webp
var clippitPetSpritesheet []byte

//go:embed pets_assets/tux/spritesheet.webp
var tuxPetSpritesheet []byte

//go:embed pets_assets/wall-e/spritesheet.webp
var wallEPetSpritesheet []byte

//go:embed pets_assets/dobby/spritesheet.webp
var dobbyPetSpritesheet []byte
