package genes

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// The animations of a portrait are sets, such as idle, which the game plays
// a character in. A set says for each sex which state of its head and which
// of its torso the portrait starts in, the entities' own states of the .asset
// files, which play the animations and go on from one to the next:
//
//	idle = {
//		male = {
//			default = { head = "idle_entry" torso = "idle_entry" }
//
//			smoker_character_idle = {
//				animation = { head = "idle_entry" torso = "idle_entry" }
//				weight = { ... }
//				portrait_modifier = { animations_props = smoking_attachment }
//			}
//		}
//		girl = boy
//	}
//
// Beside the default, a sex can have variants, which the game picks one of
// by their weights, triggers of the character it draws, and which can dress
// the portrait with props. Portrait modifiers written beside the default
// dress the portrait whichever state it plays.

// FolderAnimations is where the animations of portraits are.
var FolderAnimations = folders.Folder{Path: "gfx/portraits/portrait_animations"}

// kindAnimation is what an animation set goes by in diagnostics.
const kindAnimation = "portrait animation"

// AnimationSet is one set of animations, such as idle, for every sex.
type AnimationSet struct {
	database.Definition

	// Sexes are the animations of each sex, in the order of Sex.
	Sexes [len(sexKeys)]PortraitAnimations
}

// For returns the animations of a sex.
func (s *AnimationSet) For(sex Sex) *PortraitAnimations {
	if sex < Male || sex > Girl {
		return &PortraitAnimations{}
	}

	return &s.Sexes[sex]
}

// PortraitAnimations are the animations a set gives one sex.
type PortraitAnimations struct {
	// SameAs is the sex the file says this one is the same as, such as boy
	// for girl = boy, and empty when it writes this one out.
	SameAs string

	// Default is the states the portrait starts in when no variant is
	// picked.
	Default StatePair

	// Modifiers are the portrait modifiers written beside the default, which
	// dress the portrait whichever of the states of the set it plays, such as
	// the expression of an angry set, in the order written.
	Modifiers []ModifierChoice

	// Variants are the other states the game can pick, in the order written.
	Variants []AnimationVariant
}

// StatePair is a state of the head and one of the torso, by the names their
// entities give them.
type StatePair struct {
	Head, Torso string
}

// AnimationVariant is a variant of a set, by its name. Its weight is not
// read: it depends on the character the game draws.
type AnimationVariant struct {
	Name      string
	Animation StatePair

	// Modifiers are the portrait modifiers the variant dresses the portrait
	// with, such as the props it holds, in the order written.
	Modifiers []ModifierChoice
}

// ModifierChoice names a portrait modifier of gfx/portraits/portrait_modifiers
// by its group and its own name, as animations_props = smoking_attachment
// does.
type ModifierChoice struct {
	Group, Modifier string
}

// The keys of an animation set.
const (
	keyDefault          = "default"
	keyAnimation        = "animation"
	keyHead             = "head"
	keyTorso            = "torso"
	keyPortraitModifier = "portrait_modifier"
)

// LoadAnimations reads the animation sets of a set of folders, by name.
//
// Loading never fails. Everything worth knowing about the files is in the
// diagnostics.
func LoadAnimations(set *folders.Set) (database.Table[AnimationSet], report.Diagnostics) {
	collector := &report.Collector{}
	store := database.Collect(set, FolderAnimations, kindAnimation, collector)

	table := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *AnimationSet {
		return decodeAnimationSet(r)
	})

	return table, collector.Diagnostics
}

// ReadAnimations reads the animation sets of one file that has already been
// parsed.
func ReadAnimations(document *script.Document, file folders.File, collector *report.Collector) database.Table[AnimationSet] {
	store := database.NewStore(kindAnimation)
	store.AddDocument(document, file, collector)

	return database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *AnimationSet {
		return decodeAnimationSet(r)
	})
}

// decodeAnimationSet reads one set.
func decodeAnimationSet(r *database.Reader) *AnimationSet {
	set := &AnimationSet{}

	animations, sameAs := sexes(r, decodePortraitAnimations)

	for sex := range sexKeys {
		set.Sexes[sex] = animations[sex]
		set.Sexes[sex].SameAs = sameAs[sex]
	}

	return set
}

// decodePortraitAnimations reads the animations of one sex: the default, and
// every other block a variant.
func decodePortraitAnimations(r *database.Reader) PortraitAnimations {
	animations := PortraitAnimations{}

	if index, ok := r.Last(keyDefault); ok {
		animations.Default = statePair(r, index)
	} else {
		r.Missing(keyDefault, "the portrait starts in no state")
	}

	for index, field := range r.Fields() {
		if field.Key == keyDefault {
			continue
		}

		// A portrait_modifier beside the default is the sex's own, not a
		// variant, as Europa Universalis 5 writes the expressions of its sets.
		if field.Key == keyPortraitModifier {
			r.Known(field.Key)

			if field.Value.IsBlock() {
				animations.Modifiers = append(animations.Modifiers, modifierChoices(r.Child(index, field.Value, keyPortraitModifier))...)
			} else {
				r.Warn(index, field.Value, "%s should be a block, found %s; ignored", field.Key, field.Value.Describe())
			}

			continue
		}

		r.Known(field.Key)

		if !field.Value.IsBlock() {
			r.Warn(index, field.Value, "variant %s should be a block, found %s; ignored", field.Key, field.Value.Describe())

			continue
		}

		child := r.Child(index, field.Value, "variant "+field.Key)
		child.Known(keyWeight)

		variant := AnimationVariant{Name: field.Key}

		if at, ok := child.Last(keyAnimation); ok {
			variant.Animation = statePair(child, at)
		} else {
			child.Missing(keyAnimation, "the variant plays no state")
		}

		for _, block := range child.Blocks(keyPortraitModifier) {
			variant.Modifiers = append(variant.Modifiers, modifierChoices(child.Child(block.Index, block.Value, keyPortraitModifier))...)
		}

		child.Finish()

		animations.Variants = append(animations.Variants, variant)
	}

	return animations
}

// modifierChoices reads a block of portrait modifiers, each the name of a
// modifier under the key of its group.
func modifierChoices(r *database.Reader) []ModifierChoice {
	var choices []ModifierChoice

	for index, field := range r.Fields() {
		r.Known(field.Key)

		name, ok := field.Value.Str()
		if !ok || field.Value.IsBlock() {
			r.Warn(index, field.Value, "modifier group %s should name a modifier, found %s; ignored", field.Key, field.Value.Describe())

			continue
		}

		choices = append(choices, ModifierChoice{Group: field.Key, Modifier: name})
	}

	r.Finish()

	return choices
}

// statePair reads a block of a state of the head and one of the torso, at a
// field of a reader.
func statePair(r *database.Reader, index int) StatePair {
	value := r.Fields()[index].Value
	if !value.IsBlock() {
		r.Warn(index, value, "%s should be a block of a head and a torso, found %s; ignored", r.Fields()[index].Key, value.Describe())

		return StatePair{}
	}

	child := r.Child(index, value, r.Fields()[index].Key)
	pair := StatePair{Head: child.Text(keyHead), Torso: child.Text(keyTorso)}
	child.Finish()

	return pair
}
