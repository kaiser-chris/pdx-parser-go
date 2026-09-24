package victoria3

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Heraldry is the coats of arms of a set of folders, the named colours they
// are drawn in, and the pictures they draw.
//
// It is loaded apart from the economy: a game and its mods hold thousands of
// coats of arms, which only a tool that draws flags has any use for.
type Heraldry struct {
	// Sources are the names of the folders that could be read, in load order.
	Sources []string

	// CoatOfArms are the coats of arms, by key.
	CoatOfArms database.Table[CoatOfArms]

	// Templates are the building blocks in the template blocks of the files,
	// which the game puts the coats of arms of new countries together from.
	// They are read the same way and kept apart, because they are not coats
	// of arms a country can be given.
	Templates database.Table[CoatOfArms]

	// Colors are the named colours, resolved.
	Colors Palette

	// Textures are the pictures on disk the coats of arms can draw.
	Textures []TextureFile

	// Diagnostics are the problems found in the files. Problems with the
	// folders themselves are in the set's own Diagnostics.
	Diagnostics report.Diagnostics
}

// FolderCoatOfArms is where the coats of arms are defined.
var FolderCoatOfArms = folders.Folder{Path: "common/coat_of_arms/coat_of_arms"}

// The names the two kinds of definition go by in diagnostics.
const (
	kindCoatOfArms = "coat of arms"
	kindTemplate   = "coat of arms template"
)

// TextureKind is which part of a coat of arms a picture can be used for.
type TextureKind uint8

const (
	// PatternTexture is a background, drawn recoloured through the coat of
	// arms' own colours.
	PatternTexture TextureKind = iota

	// ColoredEmblemTexture is an emblem whose red, green and blue stand for
	// the emblem's three colour slots.
	ColoredEmblemTexture

	// TexturedEmblemTexture is an emblem drawn in the colours it was painted
	// in.
	TexturedEmblemTexture
)

func (k TextureKind) String() string {
	switch k {
	case PatternTexture:
		return "pattern"
	case ColoredEmblemTexture:
		return "colored emblem"
	case TexturedEmblemTexture:
		return "textured emblem"
	}

	return "unknown"
}

// Folder is where the pictures of this kind are kept.
func (k TextureKind) Folder() string {
	switch k {
	case PatternTexture:
		return "gfx/coat_of_arms/patterns"
	case ColoredEmblemTexture:
		return "gfx/coat_of_arms/colored_emblems"
	case TexturedEmblemTexture:
		return "gfx/coat_of_arms/textured_emblems"
	}

	return ""
}

// TextureKinds are the kinds of picture a coat of arms draws.
var TextureKinds = []TextureKind{PatternTexture, ColoredEmblemTexture, TexturedEmblemTexture}

// TextureExtensions are the image formats the games load coat of arms
// pictures from.
var TextureExtensions = []string{".dds", ".tga", ".png"}

// TextureFile is one picture on disk that a coat of arms can draw. A file of
// a mod replaces the file of the same name of the game, as it does in the
// game itself.
type TextureFile struct {
	folders.File

	Kind TextureKind
}

// LoadHeraldry reads the coats of arms of a set of folders, with the colours
// and the pictures they use.
//
// Loading never fails. Everything worth knowing about the files is in
// Diagnostics.
func LoadHeraldry(set *folders.Set) *Heraldry {
	collector := &report.Collector{}
	heraldry := &Heraldry{Sources: set.Names()}

	heraldry.Colors = LoadPalette(set, collector)

	arms, templates := collectCoatOfArms(set, collector)

	heraldry.CoatOfArms = decodeArms(arms, collector, false)
	heraldry.Templates = decodeArms(templates, collector, true)

	heraldry.Textures = LoadTextures(set, collector)
	heraldry.Diagnostics = collector.Diagnostics

	return heraldry
}

// ReadCoatOfArms reads the coats of arms of one file that has already been
// parsed: the definitions in it, and the templates of its template block.
//
// It is what LoadHeraldry does to every file of the coat of arms folder, for
// a tool that has a single file in hand rather than a whole game. file only
// places the diagnostics, so the zero value will do for a file that is not on
// disk.
func ReadCoatOfArms(document *script.Document, file folders.File, collector *report.Collector) (arms, templates database.Table[CoatOfArms]) {
	armsStore := database.NewStore(kindCoatOfArms)
	templateStore := database.NewStore(kindTemplate)

	addCoatOfArms(document, file, armsStore, templateStore, collector)

	return decodeArms(armsStore, collector, false), decodeArms(templateStore, collector, true)
}

// decodeArms turns collected definitions into coats of arms.
func decodeArms(store *database.Store, collector *report.Collector, templates bool) database.Table[CoatOfArms] {
	return database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *CoatOfArms {
		arms := decodeCoatOfArms(r)
		arms.Template = templates

		return arms
	})
}

// collectCoatOfArms reads the coat of arms folder into two stores: the coats
// of arms themselves, and the templates.
//
// The templates of a file sit together in one template block, which every
// file that has them writes again. Read as an ordinary definition, each of
// those blocks would override the one before it and all but the last file's
// templates would be lost, so the block is opened here and its contents are
// collected as definitions of their own.
func collectCoatOfArms(set *folders.Set, collector *report.Collector) (arms, templates *database.Store) {
	files, diagnostics := set.Files(FolderCoatOfArms)
	collector.AddAll(diagnostics)

	arms = database.NewStore(kindCoatOfArms)
	templates = database.NewStore(kindTemplate)

	for _, parsed := range database.ParseFiles(files, collector) {
		addCoatOfArms(parsed.Document, parsed.File, arms, templates, collector)
	}

	return arms, templates
}

// addCoatOfArms adds the definitions of one parsed file to the two stores.
func addCoatOfArms(document *script.Document, file folders.File, arms, templates *database.Store, collector *report.Collector) {
	for _, field := range document.Fields {
		comment, _ := document.Comment(field.Line)

		if field.Key == keyTemplate && field.Value.IsBlock() {
			for _, template := range field.Value.Fields {
				comment, _ := document.Comment(template.Line)
				templates.Add(template, comment, file, collector)
			}

			continue
		}

		arms.Add(field, comment, file, collector)
	}
}

// LoadTextures lists the pictures the coats of arms of a set of folders can
// draw, in the order the game reads them.
func LoadTextures(set *folders.Set, collector *report.Collector) []TextureFile {
	var textures []TextureFile

	for _, kind := range TextureKinds {
		for _, extension := range TextureExtensions {
			folder := folders.Folder{Path: kind.Folder(), Extension: extension}

			files, diagnostics := set.Files(folder)
			collector.AddAll(diagnostics)

			for _, file := range files {
				textures = append(textures, TextureFile{File: file, Kind: kind})
			}
		}
	}

	return textures
}
