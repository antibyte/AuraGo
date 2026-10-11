package persondetect

import _ "embed"

// selftest_reference.jpg is "Pedestrians using crosswalk (Unsplash).jpg" by
// Peter Miranda, CC0 1.0,
// https://commons.wikimedia.org/wiki/File:Pedestrians_using_crosswalk_(Unsplash).jpg
// (1280 px Commons thumbnail resized to 800 px wide with LANCZOS, JPEG q85).
// 800×533, 104,328 bytes, SHA-256
// 26c05f7f2861024e9756683cea133a930b73b4498784d99e9212e5e26e64f1a2.
// With the Python/PIL pipeline D-FINE-N finds 4 persons ≥ 0.5 and 8 ≥ 0.3.
//
//go:embed selftest_reference.jpg
var referenceJPEG []byte

// ReferenceImage returns a copy of the embedded self-test photo.
func ReferenceImage() []byte {
	return append([]byte(nil), referenceJPEG...)
}
