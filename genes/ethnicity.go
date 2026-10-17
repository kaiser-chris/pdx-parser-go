package genes

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// FolderEthnicities is where the ethnicities are.
var FolderEthnicities = folders.Folder{Path: "common/ethnicities"}

// kindEthnicity names an ethnicity in diagnostics.
const kindEthnicity = "ethnicity"

// Ethnicity is the ranges a character's genes are drawn from:
//
//	caucasian = {
//		template = "caucasian_base"
//		eye_color = {
//			20 = { 0.05 0.5 0.33 0.8 }
//			60 = { 0.67 0.5 1.0 0.8 }
//		}
//		gene_chin_width = {
//			17 = { name = chin_width range = { 0.3 0.45 } }
//			64 = { name = chin_width range = { 0.45 0.55 } }
//		}
//	}
//
// Each gene lists weighted choices: a template of the gene and the range its
// strength is drawn from, or for a colour gene the rectangle of its palette
// the colour is drawn from. An ethnicity builds on the one its Template
// names, replacing the genes it lists; Flatten works out the whole of it.
type Ethnicity struct {
	database.Definition

	// Template is the ethnicity this one builds on, when it builds on one.
	Template string

	// Using names something of the game the ethnicity goes with, such as a
	// culture's look; the files do not say what.
	Using string

	// Visible says whether the game offers the ethnicity, as opposed to one
	// only others build on. It is true when the file does not say.
	Visible bool

	// Colors are the colour genes, and Genes the others, in the order
	// written.
	Colors []EthnicColor
	Genes  []EthnicGene
}

// EthnicColor is what an ethnicity draws a colour gene from.
type EthnicColor struct {
	Gene    string
	Choices []ColorChoice
}

// ColorChoice is one weighted rectangle of a colour gene's palette.
type ColorChoice struct {
	Weight int

	// From and To are opposite corners of the rectangle, as x and y of the
	// palette from 0 to 1.
	From, To [2]float64
}

// Middle is the point in the middle of the rectangle.
func (c ColorChoice) Middle() [2]float64 {
	return [2]float64{(c.From[0] + c.To[0]) / 2, (c.From[1] + c.To[1]) / 2}
}

// EthnicGene is what an ethnicity draws a morph or accessory gene from.
type EthnicGene struct {
	Gene    string
	Choices []GeneChoice
}

// GeneChoice is one weighted template of a gene, with the range its strength
// is drawn from, from 0 to 1.
type GeneChoice struct {
	Weight   int
	Template string
	Range    Range
}

// The keys of an ethnicity.
const (
	keyTemplate = "template"
	keyUsing    = "using"
	keyVisible  = "visible"
	keyName     = "name"
	keyRange    = "range"
)

// LoadEthnicities reads the ethnicities of a set of folders, by name.
//
// Loading never fails. Everything worth knowing about the files is in the
// diagnostics.
func LoadEthnicities(set *folders.Set) (database.Table[Ethnicity], report.Diagnostics) {
	collector := &report.Collector{}
	store := database.Collect(set, FolderEthnicities, kindEthnicity, collector)

	table := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *Ethnicity {
		return decodeEthnicity(r)
	})

	linkEthnicities(table, collector)

	return table, collector.Diagnostics
}

// ReadEthnicities reads the ethnicities of one file that has already been
// parsed.
func ReadEthnicities(document *script.Document, file folders.File, collector *report.Collector) database.Table[Ethnicity] {
	store := database.NewStore(kindEthnicity)
	store.AddDocument(document, file, collector)

	table := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *Ethnicity {
		return decodeEthnicity(r)
	})

	linkEthnicities(table, collector)

	return table
}

// decodeEthnicity reads one ethnicity. Every field other than its own is a
// gene, which is a colour gene when its choices are lists of four numbers.
func decodeEthnicity(r *database.Reader) *Ethnicity {
	ethnicity := &Ethnicity{
		Template: r.Text(keyTemplate),
		Using:    r.Text(keyUsing),
		Visible:  r.FlagOr(keyVisible, true),
	}

	seen := map[string]bool{}

	for index, field := range r.Fields() {
		if field.Key == keyTemplate || field.Key == keyUsing || field.Key == keyVisible {
			continue
		}

		r.Known(field.Key)

		if !field.Value.IsBlock() {
			r.Warn(index, field.Value, "gene %s should be a block of weighted choices, found %s; ignored", field.Key, field.Value.Describe())

			continue
		}

		// A gene given twice, which an injection makes, is read the way a
		// database field is: the last one counts.
		if seen[field.Key] {
			ethnicity.Colors = dropColor(ethnicity.Colors, field.Key)
			ethnicity.Genes = dropGene(ethnicity.Genes, field.Key)
		}

		seen[field.Key] = true

		child := r.Child(index, field.Value, "gene "+field.Key)

		if isColor(field.Value) {
			ethnicity.Colors = append(ethnicity.Colors, EthnicColor{Gene: field.Key, Choices: colorChoices(child)})
		} else {
			ethnicity.Genes = append(ethnicity.Genes, EthnicGene{Gene: field.Key, Choices: geneChoices(child)})
		}
	}

	return ethnicity
}

// isColor reports whether a gene's choices are rectangles of a palette.
func isColor(gene script.Node) bool {
	for _, field := range gene.Fields {
		if field.Value.Kind == script.KindList && len(field.Value.Items) > 0 {
			return true
		}
	}

	return false
}

// choiceWeight reads the weight a choice is written under, a whole number of at
// least 0.
func choiceWeight(r *database.Reader, index int, field script.Field) (int, bool) {
	weight, err := strconv.Atoi(field.Key)
	if err != nil || weight < 0 {
		r.Warn(index, script.Node{}, "%s should be the weight of a choice, a whole number of at least 0; the choice is left out", field.Key)

		return 0, false
	}

	return weight, true
}

// colorChoices reads the weighted rectangles of a colour gene.
func colorChoices(r *database.Reader) []ColorChoice {
	var choices []ColorChoice

	for index, field := range r.Fields() {
		r.Known(field.Key)

		weight, ok := choiceWeight(r, index, field)
		if !ok {
			continue
		}

		numbers, ok := field.Value.Numbers()
		if !ok || len(numbers) != 4 {
			r.Warn(index, field.Value, "a colour should be four numbers, the corners of a rectangle of the palette, found %s; left out", field.Value.Describe())

			continue
		}

		choices = append(choices, ColorChoice{Weight: weight, From: [2]float64{numbers[0], numbers[1]}, To: [2]float64{numbers[2], numbers[3]}})
	}

	r.Finish()

	return choices
}

// geneChoices reads the weighted templates of a gene.
func geneChoices(r *database.Reader) []GeneChoice {
	var choices []GeneChoice

	for index, field := range r.Fields() {
		r.Known(field.Key)

		weight, ok := choiceWeight(r, index, field)
		if !ok {
			continue
		}

		if !field.Value.IsBlock() {
			r.Warn(index, field.Value, "a choice should be a block of a name and a range, found %s; left out", field.Value.Describe())

			continue
		}

		child := r.Child(index, field.Value, "choice "+field.Key)
		choice := GeneChoice{Weight: weight, Template: child.Text(keyName)}

		if choice.Template == "" {
			child.Missing(keyName, "the choice is left out")
			child.Known(keyRange)
			child.Finish()

			continue
		}

		if at, ok := child.Last(keyRange); ok {
			value := child.Fields()[at].Value

			numbers, ok := value.Numbers()
			if ok && len(numbers) == 2 {
				choice.Range = Range{Min: numbers[0], Max: numbers[1]}
			} else {
				child.Warn(at, value, "%s should be two numbers, found %s; read as 0 to 1", keyRange, value.Describe())
				choice.Range = Range{Min: 0, Max: 1}
			}
		} else {
			child.Missing(keyRange, "it is taken as 0 to 1")
			choice.Range = Range{Min: 0, Max: 1}
		}

		child.Finish()
		choices = append(choices, choice)
	}

	r.Finish()

	return choices
}

func dropColor(colors []EthnicColor, gene string) []EthnicColor {
	kept := colors[:0]

	for _, color := range colors {
		if color.Gene != gene {
			kept = append(kept, color)
		}
	}

	return kept
}

func dropGene(genes []EthnicGene, gene string) []EthnicGene {
	kept := genes[:0]

	for _, each := range genes {
		if each.Gene != gene {
			kept = append(kept, each)
		}
	}

	return kept
}

// linkEthnicities checks that the ethnicity each builds on exists, and that
// no chain of them goes round in a loop.
func linkEthnicities(table database.Table[Ethnicity], collector *report.Collector) {
	for key, ethnicity := range table.All() {
		if ethnicity.Template == "" {
			continue
		}

		origin := ethnicity.Origin()

		if !table.Has(ethnicity.Template) {
			collector.Addf(report.SeverityWarning, origin.Source, origin.Path, origin.Line, 0, kindEthnicity+" "+key,
				"builds on ethnicity %s, which does not exist", ethnicity.Template)

			continue
		}

		if _, err := Flatten(table, key); err != nil {
			collector.Addf(report.SeverityWarning, origin.Source, origin.Path, origin.Line, 0, kindEthnicity+" "+key, "%v", err)
		}
	}
}

// Flatten works out the whole of an ethnicity: the genes of the ethnicities
// it builds on, each replaced by the ethnicity's own where it lists the gene,
// in the order the one furthest down the chain lists them, followed by those
// each later one adds.
//
// It fails for an ethnicity that does not exist, and for a chain that goes
// round in a loop. An ethnicity built on one that does not exist is flattened
// as far as the chain goes.
func Flatten(table database.Table[Ethnicity], key string) (*Ethnicity, error) {
	first, ok := table.Get(key)
	if !ok {
		return nil, fmt.Errorf("there is no ethnicity %s", key)
	}

	chain := []*Ethnicity{first}
	seen := map[string]bool{key: true}

	for at := first; at.Template != ""; {
		if seen[at.Template] {
			return nil, fmt.Errorf("its chain of templates goes round in a loop at ethnicity %s", at.Template)
		}

		seen[at.Template] = true

		next, ok := table.Get(at.Template)
		if !ok {
			break
		}

		chain = append(chain, next)
		at = next
	}

	flat := &Ethnicity{Definition: first.Definition, Template: first.Template, Using: first.Using, Visible: first.Visible}

	for index := len(chain) - 1; index >= 0; index-- {
		for _, color := range chain[index].Colors {
			if at := slices.IndexFunc(flat.Colors, func(each EthnicColor) bool { return each.Gene == color.Gene }); at >= 0 {
				flat.Colors[at] = color
			} else {
				flat.Colors = append(flat.Colors, color)
			}
		}

		for _, gene := range chain[index].Genes {
			if at := slices.IndexFunc(flat.Genes, func(each EthnicGene) bool { return each.Gene == gene.Gene }); at >= 0 {
				flat.Genes[at] = gene
			} else {
				flat.Genes = append(flat.Genes, gene)
			}
		}

		if chain[index].Using != "" {
			flat.Using = chain[index].Using
		}
	}

	return flat, nil
}

// Gene returns what the ethnicity draws a morph or accessory gene from.
func (e *Ethnicity) Gene(name string) (*EthnicGene, bool) {
	for index := range e.Genes {
		if e.Genes[index].Gene == name {
			return &e.Genes[index], true
		}
	}

	return nil, false
}

// Color returns what the ethnicity draws a colour gene from.
func (e *Ethnicity) Color(name string) (*EthnicColor, bool) {
	for index := range e.Colors {
		if e.Colors[index].Gene == name {
			return &e.Colors[index], true
		}
	}

	return nil, false
}

// Likeliest is the choice of the greatest weight, the first of them when
// several share it. It reports false for a gene of no choices.
func (g *EthnicGene) Likeliest() (GeneChoice, bool) {
	best := -1

	for index, choice := range g.Choices {
		if best < 0 || choice.Weight > g.Choices[best].Weight {
			best = index
		}
	}

	if best < 0 {
		return GeneChoice{}, false
	}

	return g.Choices[best], true
}

// Likeliest is the choice of the greatest weight, the first of them when
// several share it. It reports false for a colour of no choices.
func (c *EthnicColor) Likeliest() (ColorChoice, bool) {
	best := -1

	for index, choice := range c.Choices {
		if best < 0 || choice.Weight > c.Choices[best].Weight {
			best = index
		}
	}

	if best < 0 {
		return ColorChoice{}, false
	}

	return c.Choices[best], true
}

// Preview is how a DNA is made from an ethnicity without chance.
type Preview int

const (
	// PreviewNeutral is the DNA the game's portrait editor shows for an
	// ethnicity: every morph gene at the first template the genes files
	// give it and the middle of its strength, every accessory gene at its
	// first template and the start of its strength, and every colour at
	// the corner of its palette. The ethnicity itself plays no part in it.
	// That is what the editor exports for ethnicity_template, gene for gene.
	PreviewNeutral Preview = iota

	// PreviewLikeliest takes every gene the ethnicity lists from its
	// likeliest choice, at the middle of its range, and every colour from
	// the middle of its likeliest rectangle. Genes the ethnicity does not
	// list are neutral.
	PreviewLikeliest
)

// The values a DNA writes, from 0 to 255, and the middle of them, which the
// game writes for a neutral morph gene.
const (
	neutralMorph     = 127
	neutralAccessory = 0
)

// Preview makes the DNA of an ethnicity, flattened, without chance: of every
// colour gene, every morph gene and every accessory gene that is part of a
// DNA, in that order, the way the game writes a DNA out.
func (g *Genes) Preview(ethnicity *Ethnicity, preview Preview) *DNA {
	dna := &DNA{Enabled: true}

	if ethnicity != nil {
		dna.Definition = ethnicity.Definition
	}

	for key := range g.ColorGenes.All() {
		value := GeneValue{Gene: key}

		if preview == PreviewLikeliest && ethnicity != nil {
			if color, ok := ethnicity.Color(key); ok {
				if choice, ok := color.Likeliest(); ok {
					middle := choice.Middle()
					allele := Allele{Value: byteOf(middle[0]), Second: byteOf(middle[1])}
					value.Dominant, value.Recessive = allele, allele
				}
			}
		}

		dna.Genes = append(dna.Genes, value)
	}

	add := func(key string, templates []string, neutral int) {
		if len(templates) == 0 {
			return
		}

		allele := Allele{Template: templates[0], Value: neutral}

		if preview == PreviewLikeliest && ethnicity != nil {
			if gene, ok := ethnicity.Gene(key); ok {
				if choice, ok := gene.Likeliest(); ok {
					allele = Allele{Template: choice.Template, Value: byteOf((choice.Range.Min + choice.Range.Max) / 2)}
				}
			}
		}

		dna.Genes = append(dna.Genes, GeneValue{Gene: key, Dominant: allele, Recessive: allele})
	}

	for key, gene := range g.MorphGenes.All() {
		add(key, firstTemplates(gene.Templates), neutralMorph)
	}

	for key, gene := range g.AccessoryGenes.All() {
		names := make([]string, 0, len(gene.Templates))
		for _, template := range byIndex(gene.Templates) {
			names = append(names, template.Name)
		}

		add(key, names, neutralAccessory)
	}

	return dna
}

// firstTemplates lists the names of the templates of a morph gene in the
// order of their index, the first being what a neutral DNA takes.
func firstTemplates(templates []MorphTemplate) []string {
	order := make([]int, len(templates))
	for index := range order {
		order[index] = index
	}

	for at := 1; at < len(order); at++ {
		for back := at; back > 0 && templates[order[back]].Index < templates[order[back-1]].Index; back-- {
			order[back], order[back-1] = order[back-1], order[back]
		}
	}

	names := make([]string, len(order))
	for position, index := range order {
		names[position] = templates[index].Name
	}

	return names
}

// byIndex returns the templates of an accessory gene in the order of their
// index.
func byIndex(templates []AccessoryTemplate) []AccessoryTemplate {
	sorted := append([]AccessoryTemplate(nil), templates...)

	for at := 1; at < len(sorted); at++ {
		for back := at; back > 0 && sorted[back].Index < sorted[back-1].Index; back-- {
			sorted[back], sorted[back-1] = sorted[back-1], sorted[back]
		}
	}

	return sorted
}

// byteOf turns a strength from 0 to 1 into the value a DNA writes, rounded
// down the way the game writes the middle, 0.5, as 127.
func byteOf(strength float64) int {
	return int(math.Floor(min(max(strength, 0), 1) * maxAllele))
}
