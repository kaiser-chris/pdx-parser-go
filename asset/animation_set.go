package asset

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
)

// AnimationSet is a skeletal_animation_set definition: animations made for
// one skeleton, which meshes with a skeleton like it import rather than list
// each for themselves.
//
//	skeletal_animation_set = {
//		name = "common_body"
//		reference_skeleton = "male_body_mesh"
//		animation = { id = "body_idle" type = "male_body_idle.anim" }
//	}
//
//	pdxmesh = {
//		name = "female_body_mesh"
//		import = { type = skeletal_animation_set name = "common_body" }
//	}
//
// Crusader Kings 3 shares the animations of its bodies and heads this way, so
// that the female body plays the animations made for the male one. The Key of
// the embedded Definition is the set's name.
type AnimationSet struct {
	database.Definition

	// ReferenceSkeleton is the name of the pdxmesh whose skeleton the
	// animations were made for.
	ReferenceSkeleton string

	// Animations are the animations of the set. Their files are relative to
	// the folder of the asset file the set is defined in, not that of a mesh
	// importing it.
	Animations []Animation
}

// Import brings definitions of another kind into a mesh.
type Import struct {
	// Type is the kind of definition imported. The shipped files only import
	// skeletal_animation_set, the one this package reads.
	Type string

	// Name is the name of the definition.
	Name string
}

// MeshAnimation is an animation a mesh can play, with the definition it comes
// from, the mesh itself or an animation set it imports, whose origin its file
// is relative to.
type MeshAnimation struct {
	Animation

	// Origin is where the animation is defined; Resolve(Origin, File) is
	// where its file is.
	Origin database.Origin
}

// The keys of a skeletal_animation_set definition and of an import.
const (
	keyReferenceSkeleton = "reference_skeleton"
	keyImport            = "import"
)

// decodeAnimationSet reads one skeletal_animation_set definition.
func decodeAnimationSet(r *database.Reader) *AnimationSet {
	r.Known(keyName)

	set := &AnimationSet{
		ReferenceSkeleton: r.Text(keyReferenceSkeleton),
		Animations:        animations(r, keyAnimation),
	}

	if len(set.Animations) == 0 {
		// Not worth a warning: a set may be left empty while its animations
		// are being made.
		r.NoteDefinition("has no animations")
	}

	return set
}

// imports reads the import blocks of a mesh.
func imports(r *database.Reader) []Import {
	var found []Import

	for _, block := range r.Blocks(keyImport) {
		child := r.Child(block.Index, block.Value, keyImport)
		imported := Import{Type: child.Text(keyType), Name: child.Text(keyName)}

		if imported.Name == "" {
			child.Missing(keyName, "nothing is imported")
		}

		child.Finish()
		found = append(found, imported)
	}

	return found
}

// AnimationsOf returns every animation a mesh can play: its own, in the order
// written, then those of the animation sets it imports, in the order it
// imports them. An import of a set that does not exist adds nothing.
//
// Which of two animations of the same id the engine plays is not known from
// the shipped files, which never give one twice; both are returned.
func (a *Assets) AnimationsOf(mesh *Mesh) []MeshAnimation {
	found := make([]MeshAnimation, 0, len(mesh.Animations))

	for _, animation := range mesh.Animations {
		found = append(found, MeshAnimation{Animation: animation, Origin: mesh.Origin()})
	}

	for _, imported := range mesh.Imports {
		if imported.Type != kindAnimationSet {
			continue
		}

		set, ok := a.AnimationSets.Get(imported.Name)
		if !ok {
			continue
		}

		for _, animation := range set.Animations {
			found = append(found, MeshAnimation{Animation: animation, Origin: set.Origin()})
		}
	}

	return found
}
