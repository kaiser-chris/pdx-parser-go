// Package asset reads the 3D asset definitions of Paradox games: the .asset
// files below gfx, which tell the engine which meshes there are and how
// entities are put together from them.
//
// An asset file holds definitions of three kinds, each a block that carries
// its name inside:
//
//	pdxmesh = {
//		name = "statue_01_mesh"
//		file = "statue_01.mesh"
//		import = { type = skeletal_animation_set name = "statue_animations" }
//		meshsettings = { name = "statue_01Shape" texture_diffuse = "statue_01_diffuse.dds" }
//	}
//
//	entity = {
//		name = "statue_01_entity"
//		pdxmesh = "statue_01_mesh"
//	}
//
//	skeletal_animation_set = {
//		name = "statue_animations"
//		reference_skeleton = "statue_01_mesh"
//		animation = { id = "idle" type = "statue_idle.anim" }
//	}
//
// A pdxmesh names a .mesh file and says how its parts are drawn; an entity
// draws a mesh, animates it through states and attaches other entities to it;
// a skeletal_animation_set holds animations that several meshes import.
// They are engine formats, written the same way by every game on the Jomini
// engine, so this package knows nothing about any particular game. What a
// game keeps on an entity for itself, its game_data, is handed out as script
// for the game's own package to read.
//
// The package is checked against the shipped files of Victoria 3, Europa
// Universalis 5 and Crusader Kings 3, which it reads without a field left
// unread. Each writes something the others do not, such as the animation sets
// of Crusader Kings 3, and all of it is read.
//
// # Where the files are
//
// The engine reads every .asset file anywhere below gfx, whatever the folder,
// so Load lists that whole tree, in the game, its DLCs and its mods alike. A
// file of a mod replaces the file of the same path, as anywhere else.
//
// # Names
//
// Definitions are looked up by name, and a name is unique across all files.
// The shipped files of the three games never define a name twice, so what
// the engine does with a name defined again is not known from them. This
// package reads it the way the database folders work, by load order: the
// later definition overrides the earlier one completely, which is reported,
// and the earlier one is kept in the definition's Overridden.
//
// # Files the definitions name
//
// The .mesh, .anim and texture files a definition names are written relative
// to the folder of the asset file, often as a bare file name, and sometimes
// with ../ to reach a neighbouring folder. Resolve turns such a reference into
// a path below the game's root, and every .mesh and .anim file the shipped
// definitions of the three games name is where it says. The animations a mesh
// imports are relative to the set that holds them, which AnimationsOf keeps
// track of. Textures are not always where Resolve says: the engine finds many
// of them elsewhere, in a way not settled yet. Shader files are written from
// the root, and need no resolving.
//
// # Robustness
//
// As everywhere in this module, reading never fails. A definition without a
// name, or with a name that is not a string, cannot be referred to and is
// skipped, with a warning. A field of the wrong kind is read as its default,
// with a warning, and a field this package does not know is reported and
// stays readable through Definition.Raw. References between definitions are
// checked: from an entity to its mesh, the entity it clones, the entities it
// attaches and the animations and blend shapes it uses, from a mesh to the
// animation sets it imports, and from a set to its reference skeleton. One
// that does not resolve is reported and kept as written.
package asset

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Folder is where the asset files are: anywhere below gfx.
var Folder = folders.Folder{Path: "gfx", Recursive: true, Extension: ".asset"}

// The names the kinds of definition go by in diagnostics, which are also the
// keys they are written under.
const (
	kindMesh         = "pdxmesh"
	kindEntity       = "entity"
	kindAnimationSet = "skeletal_animation_set"
)

// Assets is the meshes, entities and animation sets of a set of folders.
type Assets struct {
	// Sources are the names of the folders that could be read, in load order.
	Sources []string

	// Meshes are the pdxmesh definitions, by name, in the order the game
	// reads them.
	Meshes database.Table[Mesh]

	// Entities are the entity definitions, by name, in the order the game
	// reads them.
	Entities database.Table[Entity]

	// AnimationSets are the skeletal_animation_set definitions, by name, in
	// the order the game reads them. Victoria 3 and Europa Universalis 5 have
	// none; Crusader Kings 3 shares the animations of its portraits through
	// them.
	AnimationSets database.Table[AnimationSet]

	// Diagnostics are the problems found in the files. Problems with the
	// folders themselves are in the set's own Diagnostics.
	Diagnostics report.Diagnostics
}

// stores collects the definitions of each kind as files are read.
type stores map[string]*database.Store

func newStores() stores {
	return stores{
		kindMesh:         database.NewStore(kindMesh),
		kindEntity:       database.NewStore(kindEntity),
		kindAnimationSet: database.NewStore(kindAnimationSet),
	}
}

// Load reads the asset files of a set of folders.
//
// Loading never fails. Everything worth knowing about the files is in
// Diagnostics.
func Load(set *folders.Set) *Assets {
	collector := &report.Collector{}

	files, diagnostics := set.Files(Folder)
	collector.AddAll(diagnostics)

	collected := newStores()

	for _, parsed := range database.ParseFiles(files, collector) {
		collected.add(parsed.Document, parsed.File, collector)
	}

	assets := collected.decode(collector)
	assets.Sources = set.Names()
	assets.Diagnostics = collector.Diagnostics

	return assets
}

// Read reads the definitions of one file that has already been parsed.
//
// It is what Load does to every asset file, for a tool that has a single file
// in hand rather than a whole game. References are checked within the file
// alone, so an entity whose mesh is defined in another file is reported. file
// places the diagnostics and is what Resolve resolves against, so for a file
// that is not on disk its Relative should still say where it would be.
func Read(document *script.Document, file folders.File, collector *report.Collector) *Assets {
	collected := newStores()
	collected.add(document, file, collector)

	assets := collected.decode(collector)
	assets.Sources = []string{file.Source}

	return assets
}

// add collects the definitions of one parsed file into the stores.
func (s stores) add(document *script.Document, file folders.File, collector *report.Collector) {
	for _, field := range document.Fields {
		store, ok := s[field.Key]
		if !ok {
			// Asset files hold other things too, such as the arrow types of
			// the maps, which are no meshes or entities.
			collector.Addf(report.SeverityInfo, file.Source, file.Path, field.Line, field.Column, "",
				"%s is no kind of definition this package reads; not read", field.Key)

			continue
		}

		name, ok := definitionName(field, store.Kind, file, collector)
		if !ok {
			continue
		}

		comment, _ := document.Comment(field.Line)
		store.AddNamed(name, field, comment, file, collector)
	}
}

// definitionName finds the name a definition carries inside. A definition
// without one cannot be referred to, so it is reported and skipped.
func definitionName(field script.Field, kind string, file folders.File, collector *report.Collector) (string, bool) {
	note := func(line, column int, format string, args ...any) {
		collector.Addf(report.SeverityWarning, file.Source, file.Path, line, column, "", format, args...)
	}

	if !field.Value.IsBlock() {
		note(field.Line, field.Column, "%s should be a block, found %s; ignored", kind, field.Value.Describe())

		return "", false
	}

	value, ok := field.Value.Get(keyName)
	if !ok {
		note(field.Line, field.Column, "%s has no name; ignored, since nothing can refer to it", kind)

		return "", false
	}

	name, ok := value.Str()
	if !ok || name == "" {
		note(value.Line, value.Column, "the name of a %s should be a name or a string, found %s; ignored", kind, value.Describe())

		return "", false
	}

	if names := field.Value.All(keyName); len(names) > 1 {
		note(field.Line, field.Column, "%s %s is named %d times; the last name counts", kind, name, len(names))
	}

	return name, true
}

// decode turns the collected definitions into their types and checks the
// references between them.
func (s stores) decode(collector *report.Collector) *Assets {
	assets := &Assets{
		Meshes: database.Decode(s[kindMesh], collector, func(r *database.Reader, _ *database.Definition) *Mesh {
			return decodeMesh(r)
		}),
		Entities: database.Decode(s[kindEntity], collector, func(r *database.Reader, _ *database.Definition) *Entity {
			return decodeEntity(r)
		}),
		AnimationSets: database.Decode(s[kindAnimationSet], collector, func(r *database.Reader, _ *database.Definition) *AnimationSet {
			return decodeAnimationSet(r)
		}),
	}

	link(assets, collector)

	return assets
}
