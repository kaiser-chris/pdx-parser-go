package genes

import (
	"math"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// FolderDNA is where the DNA of the characters the game knows in advance is.
var FolderDNA = folders.Folder{Path: "common/dna_data"}

// kindDNA names a DNA in diagnostics.
const kindDNA = "dna"

// DNA is the look of a character, written as a value of each gene:
//
//	dna_abraham_lincoln = {
//		portrait_info = {
//			genes = {
//				hair_color = { 3 248 3 248 }
//				gene_cheek_fat = { "cheek_fat" 155 "cheek_fat" 139 }
//				hairstyles = { "historical_hairstyles" 0 "european_hairstyles" 214 }
//			}
//		}
//		enabled = yes
//	}
//
// Every gene has two alleles, the dominant one the character shows and a
// recessive one it can pass on. A colour gene's allele is a place in its
// palette; a morph or accessory gene's names a template and a strength.
type DNA struct {
	database.Definition

	// String is the DNA written as the game encodes it, when the file gives
	// it that way rather than through Genes. Genes.ReadString reads it into
	// genes, with the genes it was written with.
	String string

	// Enabled says whether the game uses the DNA. It is true when the file
	// does not say.
	Enabled bool

	// Genes are the values of the genes, in the order written.
	Genes []GeneValue

	// Sex, Age, ID and RandomSeed are what a DNA the game writes out says
	// beside its genes: male, female, boy or girl; the age from 0 to 1; and
	// the numbers the game knows the portrait by, as written.
	Sex        string
	Age        database.Opt[float64]
	ID         string
	RandomSeed string
}

// Gene returns the value of a gene by name.
func (d *DNA) Gene(name string) (*GeneValue, bool) {
	for index := range d.Genes {
		if d.Genes[index].Gene == name {
			return &d.Genes[index], true
		}
	}

	return nil, false
}

// GeneValue is the value a DNA gives one gene.
type GeneValue struct {
	// Gene is the name of the gene.
	Gene string

	Dominant, Recessive Allele
}

// Allele is one of the two values of a gene.
type Allele struct {
	// Template is the template of a morph or accessory gene the allele
	// picks. It is empty for a colour gene.
	Template string

	// Value is the strength of the template, from 0 to 255. For a colour
	// gene it is the first coordinate in the palette, from 0 to 255.
	Value int

	// Second is the other coordinate in the palette of a colour gene, from 0
	// to 255. It is 0 for any other gene.
	Second int
}

// Strength is the value of the allele from 0 to 1, as the settings of a gene
// take it.
func (a Allele) Strength() float64 {
	return float64(a.Value) / maxAllele
}

// maxAllele is the largest value of an allele.
const maxAllele = 255

// The keys of a DNA.
const (
	keyDNA          = "dna"
	keyEnabled      = "enabled"
	keyPortraitInfo = "portrait_info"
	keyGenes        = "genes"
	keyEntity       = "entity"
	keyType         = "type"
	keyID           = "id"
	keyRandomSeed   = "random_seed"
)

// LoadDNA reads the DNA of a set of folders, by key.
//
// Loading never fails. Everything worth knowing about the files is in the
// diagnostics.
func LoadDNA(set *folders.Set) (database.Table[DNA], report.Diagnostics) {
	collector := &report.Collector{}
	store := database.Collect(set, FolderDNA, kindDNA, collector)

	table := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *DNA {
		return decodeDNA(r)
	})

	return table, collector.Diagnostics
}

// ReadDNA reads the DNA of one file that has already been parsed.
func ReadDNA(document *script.Document, file folders.File, collector *report.Collector) database.Table[DNA] {
	store := database.NewStore(kindDNA)
	store.AddDocument(document, file, collector)

	return database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *DNA {
		return decodeDNA(r)
	})
}

// decodeDNA reads one DNA.
func decodeDNA(r *database.Reader) *DNA {
	dna := &DNA{String: r.Text(keyDNA), Enabled: r.FlagOr(keyEnabled, true)}

	for _, block := range r.Blocks(keyPortraitInfo) {
		info := r.Child(block.Index, block.Value, keyPortraitInfo)
		readPortrait(info, dna)
		info.Finish()
	}

	// A DNA the game writes out, such as its portrait editor's persistent
	// DNA, holds what a portrait_info does at the top, with more about the
	// portrait beside it.
	readPortrait(r, dna)

	dna.Sex = r.Text(keyType)
	dna.Age = r.Number(keyAge)
	dna.ID = r.Raw(keyID).Text
	dna.RandomSeed = r.Raw(keyRandomSeed).Text

	return dna
}

// readPortrait reads the genes of a portrait_info block, or of a DNA the game
// writes out, which holds them at the top.
func readPortrait(r *database.Reader, dna *DNA) {
	// The entity the portrait is drawn with, which the shipped files always
	// leave at { 0 0 }, for the default.
	r.Known(keyEntity)

	for _, genes := range r.Blocks(keyGenes) {
		child := r.Child(genes.Index, genes.Value, keyGenes)

		for index, field := range child.Fields() {
			child.Known(field.Key)

			if value, ok := geneValue(child, index, field); ok {
				dna.Genes = append(dna.Genes, value)
			}
		}

		child.Finish()
	}
}

// geneValue reads the value of one gene: four numbers for a colour gene, or
// a template and a strength twice for any other.
func geneValue(r *database.Reader, index int, field script.Field) (GeneValue, bool) {
	value := GeneValue{Gene: field.Key}
	items := field.Value.Items

	if field.Value.Kind != script.KindList || len(items) != 4 {
		r.Warn(index, field.Value, "gene %s should be four values, found %s; left out", field.Key, field.Value.Describe())

		return value, false
	}

	if numbers, ok := field.Value.Numbers(); ok {
		var whole [4]int

		for position, number := range numbers {
			whole[position] = allele(r, index, field, number)
		}

		value.Dominant = Allele{Value: whole[0], Second: whole[1]}
		value.Recessive = Allele{Value: whole[2], Second: whole[3]}

		return value, true
	}

	alleles := [2]*Allele{&value.Dominant, &value.Recessive}

	for position, target := range alleles {
		template, okTemplate := items[position*2].Str()
		strength, okStrength := items[position*2+1].Num()

		if !okTemplate || !okStrength {
			r.Warn(index, field.Value, "gene %s should be four numbers, or a template and a strength twice; left out", field.Key)

			return value, false
		}

		*target = Allele{Template: template, Value: allele(r, index, field, strength)}
	}

	return value, true
}

// allele reads a value from 0 to 255. One outside is held to the nearest end,
// and a fraction rounded down, with a warning.
func allele(r *database.Reader, index int, field script.Field, number float64) int {
	if math.IsNaN(number) || number < 0 || number > maxAllele || number != math.Trunc(number) {
		held := int(min(max(number, 0), maxAllele))
		if math.IsNaN(number) {
			held = 0
		}

		r.Warn(index, field.Value, "gene %s: %g should be a whole number from 0 to %d; read as %d", field.Key, number, maxAllele, held)

		return held
	}

	return int(number)
}
