package asset

import (
	"path"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/database"
)

// Resolve turns a file a definition names, such as the File of a Mesh, into a
// path below the root of the game or mod, with forward slashes:
//
//	Resolve(mesh.Origin(), "../animation/walk.anim")
//	// gfx/models/units/animation/walk.anim, for a mesh read from
//	// gfx/models/units/infantry/infantry.asset
//
// The reference is taken as relative to the folder of the asset file the
// definition was read from, which is how the shipped files write the .mesh
// and .anim files they name. Backslashes are read as forward slashes. A
// reference that climbs out of the root keeps its leading ../, so it stays
// recognisable as one that leads nowhere.
//
// The path says where the file would be within any of the folders. Which
// folder actually has it, the game or a mod that replaces it, is for
// folders.Set to decide.
//
// Textures are not always where this says. Most of the shipped ones are, but
// some meshes name a texture kept in another folder, which the engine still
// finds. How it finds those is not settled yet, so a texture that is not at
// the resolved path is not necessarily missing.
func Resolve(origin database.Origin, reference string) string {
	reference = strings.ReplaceAll(reference, "\\", "/")
	if reference == "" {
		return ""
	}

	return path.Clean(path.Join(path.Dir(origin.File), reference))
}
