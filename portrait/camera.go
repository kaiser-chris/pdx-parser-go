package portrait

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// FolderCameras is where the cameras of portraits are.
var FolderCameras = folders.Folder{Path: "gfx/portraits/cameras"}

// kindCamera is what a camera goes by in diagnostics.
const kindCamera = "portrait camera"

// PortraitCamera is one of the cameras the game draws portraits with, by
// name, such as camera_head.
type PortraitCamera struct {
	database.Definition

	// Camera is where the camera stands and what it looks at.
	Camera Camera

	// Unknown is the picture the game shows for a character it does not
	// know, below the folders' root.
	Unknown string
}

// Camera is where a camera stands, what it looks at, and how wide it sees.
type Camera struct {
	// Position is where the camera stands, measured from PositionNode, a
	// bone of the portrait, where the file names one, and otherwise from the
	// game's origin. LookAt is the point it looks at, measured from
	// LookAtNode the same way.
	Position     Position
	PositionNode string
	LookAt       [3]float64
	LookAtNode   string

	// FOV is how wide it sees from top to bottom, in degrees, and NearFar how
	// near and how far it sees, when the file says.
	FOV     database.Opt[float64]
	NearFar database.Opt[[2]float64]
}

// The keys of a camera.
const (
	keyCamera        = "camera"
	keyUnknown       = "unknown"
	keyFOV           = "fov"
	keyCameraNearFar = "camera_near_far"
)

// LoadCameras reads the cameras of a set of folders, by name.
//
// Loading never fails. Everything worth knowing about the files is in the
// diagnostics.
func LoadCameras(set *folders.Set) (database.Table[PortraitCamera], report.Diagnostics) {
	collector := &report.Collector{}
	store := database.Collect(set, FolderCameras, kindCamera, collector)

	return database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *PortraitCamera {
		return decodePortraitCamera(r)
	}), collector.Diagnostics
}

// ReadCameras reads the cameras of one file that has already been parsed.
func ReadCameras(document *script.Document, file folders.File, collector *report.Collector) database.Table[PortraitCamera] {
	store := database.NewStore(kindCamera)
	store.AddDocument(document, file, collector)

	return database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *PortraitCamera {
		return decodePortraitCamera(r)
	})
}

// decodePortraitCamera reads one camera of the game.
func decodePortraitCamera(r *database.Reader) *PortraitCamera {
	camera := &PortraitCamera{Unknown: r.Text(keyUnknown)}

	blocks := r.Blocks(keyCamera)
	if len(blocks) == 0 {
		r.Missing(keyCamera, "the camera stands nowhere")

		return camera
	}

	last := blocks[len(blocks)-1]
	child := r.Child(last.Index, last.Value, keyCamera)
	camera.Camera = decodeCamera(child)
	child.Finish()

	return camera
}

// decodeCamera reads where a camera stands and what it looks at.
func decodeCamera(r *database.Reader) Camera {
	camera := Camera{
		PositionNode: r.Text(keyPositionNode),
		LookAt:       vector(r, keyLookAt),
		LookAtNode:   r.Text(keyLookAtNode),
		FOV:          r.Number(keyFOV),
	}

	camera.Position, _ = position(r, keyPosition)

	if nearFar, ok := pair(r, keyCameraNearFar); ok {
		camera.NearFar = database.Some(nearFar)
	}

	return camera
}
