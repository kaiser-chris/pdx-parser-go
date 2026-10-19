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
// A DNA with a priority or tags is written as Europa Universalis 5's
// common/persistent_dna holds one: the priority and the tags first, and no
// enabled, which its files never write:
//
//	magnus_eriksson = {
//	    priority = 1
//	    tags = { swe_magnus_eriksson }
//	    portrait_info = { ... }
//	}
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

	persistent := dna != nil && (dna.Priority.Set || len(dna.Tags) > 0)
	if persistent {
		if dna.Priority.Set {
			line(1, keyPriority+" = "+strconv.FormatFloat(dna.Priority.Value, 'f', -1, 64))
		}

		if len(dna.Tags) > 0 {
			line(1, keyTags+" = { "+strings.Join(dna.Tags, " ")+" }")
		}
	}

	line(1, keyPortraitInfo+" = {")
	line(2, keyGenes+" = {")

	if dna != nil {
		for _, gene := range dna.Genes {
			line(3, gene.Gene+" = { "+writeAllele(gene.Dominant, gene.Colour)+" "+writeAllele(gene.Recessive, gene.Colour)+" }")
		}
	}

	line(2, "}")
	line(1, "}")

	if !persistent {
		enabled := "yes"
		if dna != nil && !dna.Enabled {
			enabled = "no"
		}

		line(1, keyEnabled+" = "+enabled)
	}
	out.WriteString("}")

	return out.String()
}

// WritePersistentDNA writes a DNA under a key the way the game writes one out,
// as its portrait editor's persistent DNA, which ReadDNA reads back as the
// same genes, sex, age and numbers:
//
//	dna_identifier={
//	    type=male
//	    id=0
//	    age=0.355000
//	    genes={
//	        hair_color={ 3 248 3 248 }
//	        gene_cheek_fat={ "cheek_fat" 155 "cheek_fat" 139 }
//	    }
//	    entity={ 0 0 }
//	}
//
// It is written as the game writes it, without spaces around the equals
// signs. A DNA that does not say its sex is written as male, the first the
// game knows, one without an ID as 0, and one without an age as 0, which the
// game takes as the youngest; the age has six decimals, as the game gives
// it. The random seed is written only when the DNA has one. The entity is
// always { 0 0 }, the default, as the game's own files have it.
//
// lineBreak ends every line, as for WriteDNA.
func WritePersistentDNA(key string, dna *DNA, lineBreak string) string {
	var out strings.Builder

	line := func(depth int, text string) {
		out.WriteString(strings.Repeat("    ", depth))
		out.WriteString(text)
		out.WriteString(lineBreak)
	}

	if dna == nil {
		dna = &DNA{}
	}

	sex := dna.Sex
	if sex == "" {
		sex = sexKeys[Male]
	}

	id := dna.ID
	if id == "" {
		id = "0"
	}

	line(0, key+"={")
	line(1, keyType+"="+sex)
	line(1, keyID+"="+id)

	if dna.RandomSeed != "" {
		line(1, keyRandomSeed+"="+dna.RandomSeed)
	}

	line(1, keyAge+"="+strconv.FormatFloat(dna.Age.Value, 'f', 6, 64))
	line(1, keyGenes+"={")

	for _, gene := range dna.Genes {
		line(2, gene.Gene+"={ "+writeAllele(gene.Dominant, gene.Colour)+" "+writeAllele(gene.Recessive, gene.Colour)+" }")
	}

	line(1, "}")
	line(1, keyEntity+"={ 0 0 }")
	out.WriteString("}")

	return out.String()
}

// writeAllele writes one allele: a place in a palette for a colour gene, or a
// template and its strength for any other, as GeneValue.Colour says; the
// game writes an allele of no template beside one that has a template as an
// empty one, "" 0.
func writeAllele(a Allele, colour bool) string {
	if colour {
		return strconv.Itoa(a.Value) + " " + strconv.Itoa(a.Second)
	}

	return `"` + a.Template + `" ` + strconv.Itoa(a.Value)
}
