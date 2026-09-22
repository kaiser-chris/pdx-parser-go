package victoria3

import (
	"sort"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Scaling is how a modifier of a production method grows with its building.
type Scaling uint8

const (
	// Unscaled modifiers apply once, whatever the size of the building.
	Unscaled Scaling = iota

	// WorkforceScaled modifiers grow with how many of the building's jobs are
	// filled. Goods inputs and outputs are written this way.
	WorkforceScaled

	// LevelScaled modifiers grow with the building's level. Jobs are written
	// this way.
	LevelScaled

	// ThroughputScaled modifiers grow with the building's throughput. The
	// game's documentation does not mention them, but its monuments use them.
	ThroughputScaled

	// OtherScaled is a scaling block this package does not know. Its
	// modifiers are kept, under the block's name, in ScaledModifiers.Other.
	OtherScaled
)

var scalingNames = map[string]Scaling{
	"unscaled":          Unscaled,
	"workforce_scaled":  WorkforceScaled,
	"level_scaled":      LevelScaled,
	"throughput_scaled": ThroughputScaled,
}

func (s Scaling) String() string {
	for name, scaling := range scalingNames {
		if scaling == s {
			return name
		}
	}

	return "other"
}

// ScaledModifier is a modifier of a production method with its scaling.
type ScaledModifier struct {
	database.Modifier

	Scaling Scaling
}

// ScaledModifiers are the modifiers a production method applies to one kind
// of target, split by how they grow with the building.
type ScaledModifiers struct {
	WorkforceScaled  []database.Modifier
	LevelScaled      []database.Modifier
	ThroughputScaled []database.Modifier
	Unscaled         []database.Modifier

	// Other holds modifiers in scaling blocks this package does not know, by
	// the block's name, so that a scaling added in a game update is not lost.
	Other map[string][]database.Modifier
}

// All returns every modifier with its scaling.
func (s ScaledModifiers) All() []ScaledModifier {
	var all []ScaledModifier

	add := func(modifiers []database.Modifier, scaling Scaling) {
		for _, modifier := range modifiers {
			all = append(all, ScaledModifier{Modifier: modifier, Scaling: scaling})
		}
	}

	add(s.WorkforceScaled, WorkforceScaled)
	add(s.LevelScaled, LevelScaled)
	add(s.ThroughputScaled, ThroughputScaled)
	add(s.Unscaled, Unscaled)

	names := make([]string, 0, len(s.Other))
	for name := range s.Other {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		add(s.Other[name], OtherScaled)
	}

	return all
}

// Empty reports whether there are no modifiers at all.
func (s ScaledModifiers) Empty() bool {
	return len(s.All()) == 0
}

func (s *ScaledModifiers) add(scaling Scaling, name string, modifiers []database.Modifier) {
	switch scaling {
	case WorkforceScaled:
		s.WorkforceScaled = append(s.WorkforceScaled, modifiers...)
	case LevelScaled:
		s.LevelScaled = append(s.LevelScaled, modifiers...)
	case ThroughputScaled:
		s.ThroughputScaled = append(s.ThroughputScaled, modifiers...)
	case OtherScaled:
		if s.Other == nil {
			s.Other = map[string][]database.Modifier{}
		}

		s.Other[name] = append(s.Other[name], modifiers...)
	default:
		s.Unscaled = append(s.Unscaled, modifiers...)
	}
}

// scaledModifiers reads the modifier blocks of a production method, which
// split their modifiers by scaling:
//
//	building_modifiers = {
//		workforce_scaled = { goods_input_grain_add = 40 }
//		level_scaled = { building_employment_laborers_add = 4500 }
//	}
//
// A block with a scaling this package does not know is kept under its name
// and noted, since the game may have added it. A modifier written straight
// into the outer block, outside any scaling block, is read as unscaled with a
// warning.
func scaledModifiers(r *database.Reader, key string) ScaledModifiers {
	var scaled ScaledModifiers

	for _, block := range r.Blocks(key) {
		for _, field := range block.Value.Fields {
			scaling, known := scalingNames[field.Key]

			if !field.Value.IsBlock() {
				if known {
					r.Warn(block.Index, field.Value, "%s.%s should be a block, found %s; ignored", key, field.Key, field.Value.Describe())

					continue
				}

				r.Warn(block.Index, field.Value,
					"modifier %s should be inside a scaling block such as workforce_scaled; read as unscaled", field.Key)

				single := script.Node{Kind: script.KindBlock, Fields: []script.Field{field}}
				scaled.add(Unscaled, "", r.ModifierFields(block.Index, key, single))

				continue
			}

			if !known {
				scaling = OtherScaled

				r.Note(block.Index, script.Node{Line: field.Line, Column: field.Column},
					"unknown scaling %s in %s, modifiers kept under that name", field.Key, key)
			}

			scaled.add(scaling, field.Key, r.ModifierFields(block.Index, key+"."+field.Key, field.Value))
		}

		for _, item := range block.Value.Items {
			r.Warn(block.Index, item, "ignored %s in %s, which has no key", item.Describe(), key)
		}
	}

	return scaled
}
