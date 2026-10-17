package genes

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
)

// AgePreset is a curve that changes what a gene does with the age of a
// character, which the settings and decals of morph genes refer to by name.
//
//	age_preset_child_features = {
//		mode = multiply
//		curve = { { 0.0 1 } { 0.05 0.5 } { 0.22 1.0 } }
//	}
//
// The curve goes from the age of a newborn at 0 to the oldest age at 1.
type AgePreset struct {
	database.Definition

	// Mode is how the curve combines with the value of a gene: ModeMultiply
	// or ModeAdd.
	Mode string

	Curve Curve
}

// decodeAgePreset reads one age preset.
func decodeAgePreset(r *database.Reader) *AgePreset {
	return &AgePreset{Mode: mode(r), Curve: curve(r, keyCurve)}
}
