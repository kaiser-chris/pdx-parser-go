package genes

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
)

// ColorGene picks a colour of the hair, the skin or the eyes from a palette,
// a texture whose two coordinates a DNA gives.
//
//	skin_color = {
//		sync_inheritance_with = hair_color
//		color = skin
//		blend_range = { 0.55 0.65 }
//	}
type ColorGene struct {
	database.Definition

	// Index is the place of the gene in a DNA, when the file gives it.
	Index database.Opt[int]

	// Color is what the gene colours: hair, skin or eye.
	Color string

	// BlendRange is how far between the colours of its parents a child's
	// colour is picked, from 0 for the dominant parent's to 1 for the
	// recessive parent's.
	BlendRange database.Opt[Range]

	// SyncInheritanceWith is the gene the colour is inherited from the same
	// parent as, when it is.
	SyncInheritanceWith string
}

// The keys of a colour gene.
const (
	keyColor               = "color"
	keyBlendRange          = "blend_range"
	keySyncInheritanceWith = "sync_inheritance_with"
)

// decodeColorGene reads one colour gene.
func decodeColorGene(r *database.Reader) *ColorGene {
	gene := &ColorGene{
		Color:               r.Text(keyColor),
		SyncInheritanceWith: r.Text(keySyncInheritanceWith),
	}

	if r.Has(keyIndex) {
		gene.Index = database.Some(integer(r, keyIndex, 0))
	}

	if index, ok := r.Last(keyBlendRange); ok {
		value := r.Fields()[index].Value

		numbers, ok := value.Numbers()
		if ok && len(numbers) == 2 {
			gene.BlendRange = database.Some(Range{Min: numbers[0], Max: numbers[1]})
		} else {
			r.Warn(index, value, "%s should be two numbers, found %s; ignored", keyBlendRange, value.Describe())
		}
	}

	return gene
}
