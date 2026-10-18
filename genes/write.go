package genes

import (
	"strconv"
	"strings"
)

// WriteDNA writes a DNA under a key the way the files of common/dna_data hold
// one, which ReadDNA reads back as the same genes:
//
//	dna_abraham_lincoln = {
//	    portrait_info = {
//	        genes = {
//	            hair_color = { 3 248 3 248 }
//	            gene_cheek_fat = { "cheek_fat" 155 "cheek_fat" 139 }
//	        }
//	    }
//	    enabled = yes
//	}
//
// The genes are written in the order the DNA has them, each with its dominant
// allele and then its recessive one: two numbers for a colour gene, which
// names no template, and a template and a strength for any other. The DNA
// string of a DNA given as one is not written, nor what only a DNA the game
// writes out has, such as its age; the genes are what a definition of
// common/dna_data holds.
//
// lineBreak ends every line, "\n" or "\r\n", so that a definition put into a
// file matches the lines around it. The definition ends with its closing
// brace, without a line break after it.
func WriteDNA(key string, dna *DNA, lineBreak string) string {
	var out strings.Builder

	line := func(depth int, text string) {
		out.WriteString(strings.Repeat("    ", depth))
		out.WriteString(text)
		out.WriteString(lineBreak)
	}

	line(0, key+" = {")
	line(1, keyPortraitInfo+" = {")
	line(2, keyGenes+" = {")

	if dna != nil {
		for _, gene := range dna.Genes {
			colour := gene.Dominant.Template == "" && gene.Recessive.Template == ""
			line(3, gene.Gene+" = { "+writeAllele(gene.Dominant, colour)+" "+writeAllele(gene.Recessive, colour)+" }")
		}
	}

	line(2, "}")
	line(1, "}")

	enabled := "yes"
	if dna != nil && !dna.Enabled {
		enabled = "no"
	}

	line(1, keyEnabled+" = "+enabled)
	out.WriteString("}")

	return out.String()
}

// writeAllele writes one allele: a place in a palette for a colour gene, or a
// template and its strength for any other. A gene is a colour gene when
// neither allele names a template; the game writes an allele of no template
// beside one that has a template as an empty one, "" 0.
func writeAllele(a Allele, colour bool) string {
	if colour {
		return strconv.Itoa(a.Value) + " " + strconv.Itoa(a.Second)
	}

	return `"` + a.Template + `" ` + strconv.Itoa(a.Value)
}
