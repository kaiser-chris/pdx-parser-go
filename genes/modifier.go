package genes

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// Portrait modifiers change the genes of a portrait as the game draws it,
// over what its DNA gives: the clothes of a character's station, the props an
// animation holds. They come in groups, of which the game picks one modifier
// each, by weight or because something names it:
//
//	animations_props = {
//		usage = none
//		saber_attachment = {
//			dna_modifiers = {
//				accessory = {
//					mode = add
//					gene = props
//					template = gene_saber_01
//					value = 1.0
//				}
//			}
//		}
//	}
//
// An animation variant names a modifier by its group and its name, as
// animations_props = saber_attachment.

// FolderModifiers is where the portrait modifiers are.
var FolderModifiers = folders.Folder{Path: "gfx/portraits/portrait_modifiers"}

// kindModifierGroup is what a group of portrait modifiers goes by in
// diagnostics.
const kindModifierGroup = "portrait modifier group"

// ModifierGroup is a group of portrait modifiers, of which the game picks
// one.
type ModifierGroup struct {
	database.Definition

	// Usage is where the game uses the group on its own, such as game, and
	// none for a group it only uses when something names one of its
	// modifiers, as an animation does its props.
	Usage string

	// SelectionBehavior is how the game picks a modifier of the group, such
	// as weighted_random, and Fallback the modifier it takes when none can be
	// picked, when the file says.
	SelectionBehavior string
	Fallback          string

	// Priority orders the groups that change the same genes, a higher one
	// winning, and Gruesome marks a group of disfiguring looks, such as those
	// of the plague, when the file says.
	Priority database.Opt[float64]
	Gruesome database.Opt[bool]

	// Modifiers are the modifiers of the group, in the order written. Their
	// weights are not read: they depend on the character the game draws.
	Modifiers []PortraitModifier
}

// Modifier returns a modifier of the group by name.
func (g *ModifierGroup) Modifier(name string) (*PortraitModifier, bool) {
	for index := range g.Modifiers {
		if g.Modifiers[index].Name == name {
			return &g.Modifiers[index], true
		}
	}

	return nil, false
}

// PortraitModifier is one modifier of a group: the genes it changes.
type PortraitModifier struct {
	Name string

	// OutfitTags are the outfits the modifier belongs to, such as
	// no_clothes, when the file says.
	OutfitTags []string

	// Genes are what the modifier does to the genes of the portrait, in the
	// order written.
	Genes []GeneModifier
}

// GeneModifier changes one gene of a portrait.
type GeneModifier struct {
	// Kind is the kind of gene changed: accessory or morph.
	Kind string

	// Mode is how the value is taken: add puts the template at the value in
	// place of the DNA's, modify adds the value to the strength the DNA
	// gives, and replace puts the template in place of the DNA's and keeps
	// its strength.
	Mode string

	// Gene and Template are the gene and its template.
	Gene, Template string

	// Type is the sex the change is for, such as female, when the file says;
	// it is for every sex otherwise.
	Type string

	// Value is the strength the modifier gives, from 0 to 1, or Range the
	// strengths it picks one from, by chance.
	Value database.Opt[float64]
	Range database.Opt[Range]

	// Accessory is the accessory of gfx/portraits/accessories an accessory
	// gene modifier draws, when it names one rather than leaving the
	// strength to pick among the template's.
	Accessory string
}

// Strength is the strength the modifier gives the gene, from 0 to 1, as a
// portrait drawn without chance takes it: the value, or the middle of the
// range, or 0 when the file gives neither.
func (m GeneModifier) Strength() float64 {
	if m.Value.Set {
		return m.Value.Value
	}

	if m.Range.Set {
		return m.Range.Value.At(0.5)
	}

	return 0
}

// The keys of a group of portrait modifiers.
const (
	keyUsage             = "usage"
	keySelectionBehavior = "selection_behavior"
	keyFallback          = "fallback"
	keyGruesome          = "gruesome"
	keyOutfitTags        = "outfit_tags"
	keyDNAModifiers      = "dna_modifiers"
	keyGene              = "gene"
	keyMorph             = "morph"
	keyAccessory         = "accessory"
)

// LoadModifiers reads the groups of portrait modifiers of a set of folders,
// by name.
//
// Loading never fails. Everything worth knowing about the files is in the
// diagnostics.
func LoadModifiers(set *folders.Set) (database.Table[ModifierGroup], report.Diagnostics) {
	collector := &report.Collector{}
	store := database.Collect(set, FolderModifiers, kindModifierGroup, collector)

	table := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *ModifierGroup {
		return decodeModifierGroup(r)
	})

	return table, collector.Diagnostics
}

// ReadModifiers reads the groups of portrait modifiers of one file that has
// already been parsed.
func ReadModifiers(document *script.Document, file folders.File, collector *report.Collector) database.Table[ModifierGroup] {
	store := database.NewStore(kindModifierGroup)
	store.AddDocument(document, file, collector)

	return database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *ModifierGroup {
		return decodeModifierGroup(r)
	})
}

// decodeModifierGroup reads one group: its settings, and every block beside
// them a modifier.
func decodeModifierGroup(r *database.Reader) *ModifierGroup {
	group := &ModifierGroup{
		Usage:             r.Text(keyUsage),
		SelectionBehavior: r.Text(keySelectionBehavior),
		Fallback:          r.Text(keyFallback),
		Priority:          r.Number(keyPriority),
		Gruesome:          r.Flag(keyGruesome),
	}

	for index, field := range r.Fields() {
		switch field.Key {
		case keyUsage, keySelectionBehavior, keyFallback, keyPriority, keyGruesome:
			continue
		}

		r.Known(field.Key)

		if !field.Value.IsBlock() {
			r.Warn(index, field.Value, "modifier %s should be a block, found %s; ignored", field.Key, field.Value.Describe())

			continue
		}

		child := r.Child(index, field.Value, "modifier "+field.Key)
		group.Modifiers = append(group.Modifiers, decodePortraitModifier(child, field.Key))
	}

	return group
}

// decodePortraitModifier reads one modifier.
func decodePortraitModifier(r *database.Reader, name string) PortraitModifier {
	modifier := PortraitModifier{Name: name, OutfitTags: tags(r, keyOutfitTags)}

	// The weight is a script of triggers of the character the game draws.
	r.Known(keyWeight)

	for _, block := range r.Blocks(keyDNAModifiers) {
		child := r.Child(block.Index, block.Value, keyDNAModifiers)

		for index, field := range child.Fields() {
			if field.Key != keyAccessory && field.Key != keyMorph {
				continue
			}

			child.Known(field.Key)

			if !field.Value.IsBlock() {
				child.Warn(index, field.Value, "%s should be a block, found %s; ignored", field.Key, field.Value.Describe())

				continue
			}

			gene := child.Child(index, field.Value, field.Key)
			modifier.Genes = append(modifier.Genes, decodeGeneModifier(gene, field.Key))
		}

		child.Finish()
	}

	r.Finish()

	return modifier
}

// decodeGeneModifier reads what a modifier does to one gene.
func decodeGeneModifier(r *database.Reader, kind string) GeneModifier {
	modifier := GeneModifier{
		Kind:     kind,
		Mode:     r.Text(keyMode),
		Gene:     r.Text(keyGene),
		Template: r.Text(keyTemplate),
		Type:     r.Text(keyType),
		Value:    r.Number(keyValue),
	}

	if kind == keyAccessory {
		modifier.Accessory = r.Text(keyAccessory)
	}

	if modifier.Gene == "" {
		r.Missing(keyGene, "it changes no gene")
	}

	if at, ok := r.Last(keyRange); ok {
		value := r.Fields()[at].Value

		numbers, ok := value.Numbers()
		if ok && len(numbers) == 2 {
			modifier.Range = database.Some(Range{Min: numbers[0], Max: numbers[1]})
		} else {
			r.Warn(at, value, "%s should be two numbers, found %s; ignored", keyRange, value.Describe())
		}
	}

	r.Finish()

	return modifier
}
