// Package genes reads the genes of the portraits of Paradox games, in
// common/genes, and the DNA of the characters made from them, in
// common/dna_data, with what else makes a portrait of them: the ethnicities
// the DNA is drawn from, the accessories the genes pick, and the animation
// sets a portrait plays.
//
// A gene is a trait of a portrait that a character's DNA gives a value: the
// width of the nose, the hairstyle, the colour of the eyes. The genes files
// group them by kind, each kind a block of genes:
//
//	age_presets = {
//		age_preset_child_features = { mode = multiply curve = { { 0.0 1 } { 0.22 1.0 } } }
//	}
//
//	morph_genes = {
//		gene_eye_size = {
//			eye_size = {
//				index = 0
//				male = {
//					setting = { attribute = "head_bs_eye_size_max" curve = { { 0.5 0.0 } { 1.0 1.0 } } }
//					setting = { attribute = "head_bs_eye_size_min" curve = { { 0.0 1.0 } { 0.5 0.0 } } }
//				}
//				female = male
//				boy = male
//				girl = female
//			}
//		}
//	}
//
// A gene holds templates, such as eye_size above, which a DNA picks one of.
// A template says, for each sex and for children of each, what the strength
// the DNA gives the gene does to a portrait. A morph gene sets attributes of
// the portrait's entities, which an entity of the .asset files turns into
// the weight of a blend shape or of an additive animation, and lays decals
// over the skin. An accessory gene picks an accessory, such as a hairstyle,
// by weighted chance. A colour gene picks a colour of the hair, skin or eyes
// from a palette.
//
// special_genes holds genes of the same kinds that are not part of a
// character's DNA, which the game sets itself, such as the clothes.
//
// The format belongs to the Jomini engine rather than to any one game, so
// this package knows nothing about a particular game. It is checked against
// the shipped files of Victoria 3, which it reads without a field left
// unread.
//
// # Names
//
// A gene is looked up by name within its kind. A gene defined again, in a
// later file of the game or in a mod, overrides the earlier one completely,
// as the definitions of the database folders do, and is reported.
//
// # Robustness
//
// As everywhere in this module, reading never fails. A field of the wrong
// kind is read as its default, with a warning, a field this package does not
// know is reported and stays readable through Definition.Raw, and a sex that
// is the same as another one that does not exist is reported and left empty.
package genes

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Folder is where the genes are.
var Folder = folders.Folder{Path: "common/genes"}

// The kinds of gene, which are also the keys of the blocks that hold them.
const (
	keyAgePresets     = "age_presets"
	keyColorGenes     = "color_genes"
	keyMorphGenes     = "morph_genes"
	keyAccessoryGenes = "accessory_genes"
	keySpecialGenes   = "special_genes"
)

// The names each kind goes by in diagnostics.
const (
	kindAgePreset        = "age preset"
	kindColorGene        = "color gene"
	kindMorphGene        = "morph gene"
	kindAccessoryGene    = "accessory gene"
	kindSpecialMorph     = "special morph gene"
	kindSpecialAccessory = "special accessory gene"
)

// Genes is every gene of a set of folders.
type Genes struct {
	// Sources are the names of the folders that could be read, in load order.
	Sources []string

	// AgePresets are the curves that change what a gene does with the age of
	// a character, by name, which the settings of morph genes refer to.
	AgePresets database.Table[AgePreset]

	// ColorGenes pick the colours of the hair, skin and eyes.
	ColorGenes database.Table[ColorGene]

	// MorphGenes change the shape of a portrait and lay decals over it.
	MorphGenes database.Table[MorphGene]

	// AccessoryGenes pick accessories, such as hairstyles.
	AccessoryGenes database.Table[AccessoryGene]

	// SpecialMorphGenes and SpecialAccessoryGenes are genes the game sets
	// itself rather than taking them from a character's DNA, such as the
	// clothes a character wears.
	SpecialMorphGenes     database.Table[MorphGene]
	SpecialAccessoryGenes database.Table[AccessoryGene]

	// Diagnostics are the problems found in the files. Problems with the
	// folders themselves are in the set's own Diagnostics.
	Diagnostics report.Diagnostics
}

// stores collects the genes of each kind as files are read.
type stores struct {
	agePresets, colors, morphs, accessories, specialMorphs, specialAccessories *database.Store
}

func newStores() *stores {
	return &stores{
		agePresets:         database.NewStore(kindAgePreset),
		colors:             database.NewStore(kindColorGene),
		morphs:             database.NewStore(kindMorphGene),
		accessories:        database.NewStore(kindAccessoryGene),
		specialMorphs:      database.NewStore(kindSpecialMorph),
		specialAccessories: database.NewStore(kindSpecialAccessory),
	}
}

// Load reads the genes of a set of folders.
//
// Loading never fails. Everything worth knowing about the files is in
// Diagnostics.
func Load(set *folders.Set) *Genes {
	collector := &report.Collector{}

	files, diagnostics := set.Files(Folder)
	collector.AddAll(diagnostics)

	collected := newStores()

	for _, parsed := range database.ParseFiles(files, collector) {
		collected.add(parsed.Document, parsed.File, collector)
	}

	genes := collected.decode(collector)
	genes.Sources = set.Names()
	genes.Diagnostics = collector.Diagnostics

	return genes
}

// Read reads the genes of one file that has already been parsed, for a tool
// that has a single file in hand rather than a whole game. The age presets a
// setting refers to are looked for in the file alone.
func Read(document *script.Document, file folders.File, collector *report.Collector) *Genes {
	collected := newStores()
	collected.add(document, file, collector)

	genes := collected.decode(collector)
	genes.Sources = []string{file.Source}

	return genes
}

// add collects the genes of one parsed file into the stores.
func (s *stores) add(document *script.Document, file folders.File, collector *report.Collector) {
	kinds := map[string]*database.Store{
		keyAgePresets:     s.agePresets,
		keyColorGenes:     s.colors,
		keyMorphGenes:     s.morphs,
		keyAccessoryGenes: s.accessories,
	}

	special := map[string]*database.Store{
		keyMorphGenes:     s.specialMorphs,
		keyAccessoryGenes: s.specialAccessories,
	}

	for _, field := range document.Fields {
		if field.Key == keySpecialGenes {
			if !block(field, file, collector) {
				continue
			}

			for _, inner := range field.Value.Fields {
				addGenes(document, inner, special, file, collector)
			}

			for _, item := range field.Value.Items {
				collector.Addf(report.SeverityWarning, file.Source, file.Path, item.Line, item.Column, "",
					"ignored %s in %s, which has no key", item.Describe(), keySpecialGenes)
			}

			continue
		}

		addGenes(document, field, kinds, file, collector)
	}
}

// addGenes reads a block of genes of one kind into its store.
func addGenes(document *script.Document, field script.Field, kinds map[string]*database.Store, file folders.File, collector *report.Collector) {
	store, ok := kinds[field.Key]
	if !ok {
		collector.Addf(report.SeverityInfo, file.Source, file.Path, field.Line, field.Column, "",
			"%s is no kind of gene this package reads; not read", field.Key)

		return
	}

	if !block(field, file, collector) {
		return
	}

	for _, gene := range field.Value.Fields {
		comment, _ := document.Comment(gene.Line)
		store.Add(gene, comment, file, collector)
	}

	for _, item := range field.Value.Items {
		collector.Addf(report.SeverityWarning, file.Source, file.Path, item.Line, item.Column, "",
			"ignored %s in %s, which has no key", item.Describe(), field.Key)
	}
}

// block reports whether the field holds a block, and reports it when not.
func block(field script.Field, file folders.File, collector *report.Collector) bool {
	if field.Value.IsBlock() {
		return true
	}

	collector.Addf(report.SeverityWarning, file.Source, file.Path, field.Line, field.Column, "",
		"%s should be a block, found %s; ignored", field.Key, field.Value.Describe())

	return false
}

// decode turns the collected genes into their types and checks the age
// presets the morph genes refer to.
func (s *stores) decode(collector *report.Collector) *Genes {
	genes := &Genes{
		AgePresets: database.Decode(s.agePresets, collector, func(r *database.Reader, _ *database.Definition) *AgePreset {
			return decodeAgePreset(r)
		}),
		ColorGenes: database.Decode(s.colors, collector, func(r *database.Reader, _ *database.Definition) *ColorGene {
			return decodeColorGene(r)
		}),
		MorphGenes: database.Decode(s.morphs, collector, func(r *database.Reader, _ *database.Definition) *MorphGene {
			return decodeMorphGene(r)
		}),
		AccessoryGenes: database.Decode(s.accessories, collector, func(r *database.Reader, _ *database.Definition) *AccessoryGene {
			return decodeAccessoryGene(r)
		}),
		SpecialMorphGenes: database.Decode(s.specialMorphs, collector, func(r *database.Reader, _ *database.Definition) *MorphGene {
			return decodeMorphGene(r)
		}),
		SpecialAccessoryGenes: database.Decode(s.specialAccessories, collector, func(r *database.Reader, _ *database.Definition) *AccessoryGene {
			return decodeAccessoryGene(r)
		}),
	}

	link(genes, collector)

	return genes
}

// MorphGene returns a morph gene by name, whether it is part of the DNA or
// one of the special genes.
func (g *Genes) MorphGene(name string) (*MorphGene, bool) {
	if gene, ok := g.MorphGenes.Get(name); ok {
		return gene, true
	}

	return g.SpecialMorphGenes.Get(name)
}

// AccessoryGene returns an accessory gene by name, whether it is part of the
// DNA or one of the special genes.
func (g *Genes) AccessoryGene(name string) (*AccessoryGene, bool) {
	if gene, ok := g.AccessoryGenes.Get(name); ok {
		return gene, true
	}

	return g.SpecialAccessoryGenes.Get(name)
}

// AtAge changes a value the way an age does, at a point of a life from 0 to
// 1: multiplied by the age's curve there, or with the curve added for an age
// of ModeAdd. An age without a mode multiplies. An age that is not given, or
// names a preset that does not exist, leaves the value as it is.
//
// What part of a life a point is the files do not say. Their curves are
// written as though 1 were a hundred years: the features of a child are
// grown by 0.22 and a face starts to age from 0.25 on.
func (g *Genes) AtAge(value float64, age Age, at float64) float64 {
	mode, curve, ok := g.Age(age)
	if !ok {
		return value
	}

	if mode == ModeAdd {
		return value + curve.At(at)
	}

	return value * curve.At(at)
}

// SetAttributes adds what a morph's settings set at a strength of the gene
// from 0 to 1, at a point of a life from 0 to 1, to the values of the
// attributes, by name: Morph.SetAttributes, with the age of each setting
// applied as AtAge says.
func (g *Genes) SetAttributes(morph *Morph, strength, at float64, attributes map[string]float64) {
	for index := range morph.Settings {
		setting := &morph.Settings[index]
		if setting.Attribute != "" {
			attributes[setting.Attribute] += g.AtAge(setting.At(strength), setting.Age, at)
		}
	}
}

// HSVShift returns how far a morph shifts the hue, saturation and value of a
// colour of the portrait, hair, skin or eye as the files name them, at a
// strength of the gene from 0 to 1, at a point of a life from 0 to 1: each of
// its shifts of that colour at the strength, with the age of the shift
// applied to each of the three as AtAge says, added up.
func (g *Genes) HSVShift(morph *Morph, color string, strength, at float64) [3]float64 {
	var total [3]float64

	for index := range morph.HSVShifts {
		shift := &morph.HSVShifts[index]
		if shift.Color != color {
			continue
		}

		for channel, value := range shift.At(strength) {
			total[channel] += g.AtAge(value, shift.Age, at)
		}
	}

	return total
}

// DecalAlpha returns how opaque a decal is at a strength of the gene from 0
// to 1, at a point of a life from 0 to 1: Decal.Alpha, with the age of the
// decal applied as AtAge says, kept between 0 and 1. The games hand a decal's
// opacity to their shader as a fraction of a whole, which can be no more and
// no less.
func (g *Genes) DecalAlpha(decal *Decal, strength, at float64) float64 {
	return min(max(g.AtAge(decal.Alpha(strength), decal.Age, at), 0), 1)
}

// Age returns the curve an age refers to: its own, or that of the preset it
// names. It reports false for a preset that does not exist, and for an age
// that is not given, which leaves a gene as it is at every age.
func (g *Genes) Age(age Age) (mode string, curve Curve, ok bool) {
	if age.Preset == "" {
		return age.Mode, age.Curve, age.Mode != "" || len(age.Curve) > 0
	}

	preset, ok := g.AgePresets.Get(age.Preset)
	if !ok {
		return "", nil, false
	}

	return preset.Mode, preset.Curve, true
}
