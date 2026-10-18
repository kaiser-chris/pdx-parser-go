package genes

import (
	"encoding/base64"
	"strings"

	"github.com/kaiser-chris/pdx-parser-go/report"
)

// A DNA string is the other way the game writes a DNA, more compact than the
// genes of a portrait_info, as the dna field of a definition of
// common/dna_data has it:
//
//	z78g+0cGKAYrmni4AAAAfACIAKEAWgB3AH8AdQCAAIAAiwCLAIgAdgBnAHsAegBbAGkAjAC...
//
// It is base64 of four bytes for each gene, a byte for each of the four
// values a gene has in a portrait_info: for a colour gene the two places in
// its palette of the dominant allele and then those of the recessive one, and
// for a morph or accessory gene the index of the template and the strength
// of the dominant allele and then those of the recessive one. The index is
// the one the template gives itself, not its place in the gene.
//
// The string names no gene. The genes follow each other in the order the
// game defines them: the colour genes, then the morph genes, then the
// accessory genes, each in the order they are read from the files of
// common/genes, and the special genes are not part of it. A string is
// therefore only read right with the genes it was written with: a gene added
// to the game since moves every gene after it, and a string written before
// reads as nonsense from there on, which nothing in the string can tell.

// subjectDNAString names a DNA string in diagnostics.
const subjectDNAString = "DNA string"

// bytesPerGene is how many bytes a gene takes in a DNA string.
const bytesPerGene = 4

// stringGene is a gene in its place in a DNA string, with the templates of a
// morph or accessory gene by the index they go by in it.
type stringGene struct {
	name      string
	colour    bool
	templates map[int]string
	indices   map[string]int

	// first is the template of the lowest index, which an allele of a
	// template the gene does not have is read and written as.
	first string
}

// stringGenes returns the genes in the order a DNA string has them.
func (g *Genes) stringGenes() []stringGene {
	order := make([]stringGene, 0, g.ColorGenes.Len()+g.MorphGenes.Len()+g.AccessoryGenes.Len())

	for name := range g.ColorGenes.All() {
		order = append(order, stringGene{name: name, colour: true})
	}

	for name, gene := range g.MorphGenes.All() {
		entry := newStringGene(name)
		for _, template := range gene.Templates {
			entry.add(template.Name, template.Index)
		}

		order = append(order, entry)
	}

	for name, gene := range g.AccessoryGenes.All() {
		entry := newStringGene(name)
		for _, template := range gene.Templates {
			entry.add(template.Name, template.Index)
		}

		order = append(order, entry)
	}

	return order
}

func newStringGene(name string) stringGene {
	return stringGene{name: name, templates: map[int]string{}, indices: map[string]int{}}
}

// add gives the gene a template. A template of an index another already has
// is left to the first, as the game would find that one first.
func (s *stringGene) add(template string, index int) {
	if _, taken := s.templates[index]; taken {
		return
	}

	if len(s.templates) == 0 || index < s.indices[s.first] {
		s.first = template
	}

	s.templates[index] = template
	s.indices[template] = index
}

// ReadString reads a DNA string, with the genes it was written with, into the
// genes of a DNA as a portrait_info would give them, in the order of the
// string.
//
// Reading never fails. Text that is not base64 reads as a DNA of no genes;
// a string of more or fewer genes than there are, written with another list
// of genes, reads as far as both go; and an allele of a template its gene
// does not have reads as the gene's first template. Each is reported.
func (g *Genes) ReadString(text string) (*DNA, report.Diagnostics) {
	collector := &report.Collector{}
	problem := func(severity report.Severity, format string, args ...any) {
		collector.Addf(severity, "", "", 0, 0, subjectDNAString, format, args...)
	}

	dna := &DNA{String: text, Enabled: true}

	data, err := decodeBase64(text)
	if err != nil {
		problem(report.SeverityError, "it is not base64: %v; no genes read", err)

		return dna, collector.Diagnostics
	}

	if len(data)%bytesPerGene != 0 {
		problem(report.SeverityWarning, "it is %d bytes long, which is not a whole number of genes of %d bytes; the last %d left out", len(data), bytesPerGene, len(data)%bytesPerGene)
	}

	order := g.stringGenes()
	count := len(data) / bytesPerGene

	if count != len(order) {
		problem(report.SeverityWarning, "it holds %d genes and there are %d: it was written with other genes, and is read as far as both go", count, len(order))
	}

	for index := range min(count, len(order)) {
		gene := order[index]
		values := data[index*bytesPerGene : (index+1)*bytesPerGene]
		value := GeneValue{Gene: gene.name}

		if gene.colour {
			value.Dominant = Allele{Value: int(values[0]), Second: int(values[1])}
			value.Recessive = Allele{Value: int(values[2]), Second: int(values[3])}
		} else {
			value.Dominant = Allele{Template: gene.template(int(values[0]), problem), Value: int(values[1])}
			value.Recessive = Allele{Template: gene.template(int(values[2]), problem), Value: int(values[3])}
		}

		dna.Genes = append(dna.Genes, value)
	}

	return dna, collector.Diagnostics
}

// decodeBase64 reads base64 with or without its padding, and with the line
// breaks or spaces a string copied from a file may have picked up.
func decodeBase64(text string) ([]byte, error) {
	text = strings.Join(strings.Fields(text), "")

	return base64.RawStdEncoding.DecodeString(strings.TrimRight(text, "="))
}

// template returns the template of an index, or the first template of a gene
// that has no template of that index.
func (s *stringGene) template(index int, problem func(report.Severity, string, ...any)) string {
	if template, ok := s.templates[index]; ok {
		return template
	}

	problem(report.SeverityWarning, "gene %s has no template of index %d; read as %q", s.name, index, s.first)

	return s.first
}

// WriteString writes the genes of a DNA as a DNA string, which ReadString
// reads back as the same genes with the same list of genes.
//
// Writing never fails. A gene the DNA does not give is written as zeros,
// which is the first template at no strength, and a gene the DNA gives that
// is not among the genes, such as a special gene, is left out, since the
// string has no place for it. An allele of a template its gene does not have
// is written as the gene's first template. Each is reported.
func (g *Genes) WriteString(dna *DNA) (string, report.Diagnostics) {
	collector := &report.Collector{}
	problem := func(severity report.Severity, format string, args ...any) {
		collector.Addf(severity, "", "", 0, 0, subjectDNAString, format, args...)
	}

	order := g.stringGenes()
	data := make([]byte, 0, len(order)*bytesPerGene)

	given := map[string]*GeneValue{}

	if dna != nil {
		for index := range dna.Genes {
			given[dna.Genes[index].Gene] = &dna.Genes[index]
		}
	}

	var missing []string

	for _, gene := range order {
		value, ok := given[gene.name]
		if !ok {
			missing = append(missing, gene.name)
			data = append(data, 0, 0, 0, 0)

			continue
		}

		delete(given, gene.name)

		if gene.colour {
			data = append(data, clampByte(value.Dominant.Value), clampByte(value.Dominant.Second), clampByte(value.Recessive.Value), clampByte(value.Recessive.Second))

			continue
		}

		data = append(data,
			gene.index(value.Dominant.Template, problem), clampByte(value.Dominant.Value),
			gene.index(value.Recessive.Template, problem), clampByte(value.Recessive.Value))
	}

	if len(missing) > 0 {
		problem(report.SeverityInfo, "the DNA does not give %s; written as zeros", strings.Join(missing, ", "))
	}

	if dna != nil {
		var unknown []string

		for _, value := range dna.Genes {
			if _, left := given[value.Gene]; left {
				unknown = append(unknown, value.Gene)
				delete(given, value.Gene)
			}
		}

		if len(unknown) > 0 {
			problem(report.SeverityWarning, "%s is not among the genes a DNA string holds; left out", strings.Join(unknown, ", "))
		}
	}

	return base64.StdEncoding.EncodeToString(data), collector.Diagnostics
}

// index returns the index of a template, or that of the gene's first
// template for one it does not have.
func (s *stringGene) index(template string, problem func(report.Severity, string, ...any)) byte {
	index, ok := s.indices[template]
	if !ok {
		problem(report.SeverityWarning, "gene %s has no template %q; written as %q", s.name, template, s.first)

		index = s.indices[s.first]
	}

	return clampByte(index)
}

// clampByte holds a value to what a byte of a DNA string holds. The values of
// a DNA are from 0 to 255 when it is read, so only one made by hand is held.
func clampByte(value int) byte {
	return byte(min(max(value, 0), maxAllele))
}
