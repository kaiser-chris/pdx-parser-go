package genes

import (
	"bytes"
	"encoding/base64"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// stringTestGenes are genes whose templates give themselves indices other
// than their places, as the game's do.
const stringTestGenes = `
color_genes = {
	hair_color = { color = hair }
	skin_color = { color = skin }
}
morph_genes = {
	gene_nose = {
		small = { index = 0 }
		large = { index = 3 }
	}
}
accessory_genes = {
	hairstyles = {
		short = { index = 1 }
		long = { index = 0 }
	}
}
`

// stringTestDNA is a DNA of every gene of stringTestGenes, and stringTestBytes
// the bytes of its DNA string: the colours, then the morphs, then the
// accessories, four bytes a gene, a template by its index.
var (
	stringTestDNA = &DNA{Enabled: true, Genes: []GeneValue{
		{Gene: "hair_color", Colour: true, Dominant: Allele{Value: 1, Second: 2}, Recessive: Allele{Value: 3, Second: 4}},
		{Gene: "skin_color", Colour: true, Dominant: Allele{Value: 5, Second: 6}, Recessive: Allele{Value: 7, Second: 8}},
		{Gene: "gene_nose", Dominant: Allele{Template: "large", Value: 9}, Recessive: Allele{Template: "small", Value: 10}},
		{Gene: "hairstyles", Dominant: Allele{Template: "short", Value: 11}, Recessive: Allele{Template: "long", Value: 255}},
	}}

	stringTestBytes = []byte{1, 2, 3, 4, 5, 6, 7, 8, 3, 9, 0, 10, 1, 11, 0, 255}
)

// A DNA string is written as the bytes of each gene in the order of the
// genes, whatever the order of the DNA, and read back as the same genes.
func TestDNAString(t *testing.T) {
	genes, _ := read(t, stringTestGenes)

	shuffled := &DNA{Genes: slices.Clone(stringTestDNA.Genes)}
	slices.Reverse(shuffled.Genes)

	written, diagnostics := genes.WriteString(shuffled)
	for _, diagnostic := range diagnostics {
		t.Errorf("write: %s", diagnostic)
	}

	if want := base64.StdEncoding.EncodeToString(stringTestBytes); written != want {
		t.Errorf("written %s, want %s", written, want)
	}

	read, diagnostics := genes.ReadString(written)
	for _, diagnostic := range diagnostics {
		t.Errorf("read: %s", diagnostic)
	}

	if !slices.Equal(read.Genes, stringTestDNA.Genes) || read.String != written || !read.Enabled {
		t.Errorf("read back %+v, want %+v", read, stringTestDNA.Genes)
	}

	// Without its padding, broken over lines as a file may have it, it
	// reads the same.
	broken := strings.TrimRight(written, "=")
	broken = broken[:8] + "\n  " + broken[8:]

	if again, diagnostics := genes.ReadString(broken); !slices.Equal(again.Genes, stringTestDNA.Genes) || len(diagnostics) != 0 {
		t.Errorf("read without padding over two lines: %+v, %v", again.Genes, diagnostics)
	}
}

// A gene of no templates, as Europa Universalis 5's pose, is written and
// read as the empty template, without a word.
func TestDNAStringOfAGeneWithoutTemplates(t *testing.T) {
	genes, _ := read(t, `morph_genes = { pose = { } gene_nose = { small = { index = 0 } } }`)

	dna := &DNA{Genes: []GeneValue{
		{Gene: "pose", Dominant: Allele{Value: 127}, Recessive: Allele{Value: 127}},
		{Gene: "gene_nose", Dominant: Allele{Template: "small", Value: 9}, Recessive: Allele{Template: "small", Value: 10}},
	}}

	written, diagnostics := genes.WriteString(dna)
	if len(diagnostics) != 0 {
		t.Errorf("write: %v", diagnostics)
	}

	read, diagnostics := genes.ReadString(written)
	if len(diagnostics) != 0 || !slices.Equal(read.Genes, dna.Genes) {
		t.Errorf("read back %+v, %v; want %+v", read.Genes, diagnostics, dna.Genes)
	}
}

// A string written with other genes reads as far as it goes, a template that
// is not there reads as the first, and text that is not base64 reads as no
// genes, each with a word why.
func TestDNAStringOfOtherGenes(t *testing.T) {
	genes, _ := read(t, stringTestGenes)

	short := base64.StdEncoding.EncodeToString(stringTestBytes[:10])

	dna, diagnostics := genes.ReadString(short)
	if len(dna.Genes) != 2 || dna.Genes[1] != stringTestDNA.Genes[1] {
		t.Errorf("a short string read as %+v, want the two colours", dna.Genes)
	}

	expect(t, diagnostics, report.SeverityWarning, "not a whole number of genes")
	expect(t, diagnostics, report.SeverityWarning, "holds 2 genes and there are 4")

	unknown := bytes.Clone(stringTestBytes)
	unknown[8] = 7

	dna, diagnostics = genes.ReadString(base64.StdEncoding.EncodeToString(unknown))
	if nose := dna.Genes[2]; nose.Dominant.Template != "small" {
		t.Errorf("template 7 of gene_nose read as %q, want small, its first", nose.Dominant.Template)
	}

	expect(t, diagnostics, report.SeverityWarning, "gene_nose has no template of index 7")

	dna, diagnostics = genes.ReadString("not base64!")
	if len(dna.Genes) != 0 {
		t.Errorf("text that is not base64 read as %+v", dna.Genes)
	}

	expect(t, diagnostics, report.SeverityError, "not base64")
}

// A DNA that lacks a gene has it written as zeros, and a gene or template
// that is not among the genes is left out or written as the first, each with
// a word why.
func TestDNAStringOfOtherDNA(t *testing.T) {
	genes, _ := read(t, stringTestGenes)

	dna := &DNA{Genes: []GeneValue{
		stringTestDNA.Genes[0],
		{Gene: "gene_nose", Dominant: Allele{Template: "huge", Value: 9}, Recessive: Allele{Template: "large", Value: 10}},
		{Gene: "gene_special", Dominant: Allele{Template: "special", Value: 1}},
	}}

	written, diagnostics := genes.WriteString(dna)

	data, _ := base64.StdEncoding.DecodeString(written)
	if want := []byte{1, 2, 3, 4, 0, 0, 0, 0, 0, 9, 3, 10, 0, 0, 0, 0}; !bytes.Equal(data, want) {
		t.Errorf("written as %v, want %v", data, want)
	}

	expect(t, diagnostics, report.SeverityInfo, "does not give skin_color, hairstyles")
	expect(t, diagnostics, report.SeverityWarning, "gene_nose has no template \"huge\"")
	expect(t, diagnostics, report.SeverityWarning, "gene_special is not among the genes")
}

// A persistent DNA is written as the game writes one out, and read back as
// the same genes, sex, age and numbers.
func TestWritePersistentDNA(t *testing.T) {
	dna := &DNA{Sex: "female", ID: "1076145720", Genes: slices.Clone(stringTestDNA.Genes)}
	dna.Age.Value, dna.Age.Set = 0.355, true

	written := WritePersistentDNA("dna_test", dna, "\n")

	want := `dna_test={
    type=female
    id=1076145720
    age=0.355000
    genes={
        hair_color={ 1 2 3 4 }
        skin_color={ 5 6 7 8 }
        gene_nose={ "large" 9 "small" 10 }
        hairstyles={ "short" 11 "long" 255 }
    }
    entity={ 0 0 }
}`
	if written != want {
		t.Errorf("written\n%s\nwant\n%s", written, want)
	}

	read := readBack(t, written, "dna_test")
	if !slices.Equal(read.Genes, dna.Genes) || read.Sex != dna.Sex || read.ID != dna.ID || read.Age != dna.Age || read.RandomSeed != "" {
		t.Errorf("read back %+v, want %+v", read, dna)
	}

	// A DNA that says nothing more than its genes is a man of no age and
	// ID 0; a random seed is written when there is one; lines end as asked.
	written = WritePersistentDNA("dna_test", &DNA{RandomSeed: "42"}, "\r\n")
	if !strings.Contains(written, "type=male\r\n    id=0\r\n    random_seed=42\r\n    age=0.000000\r\n") || strings.Count(written, "\n") != strings.Count(written, "\r\n") {
		t.Errorf("written with nothing but a seed:\n%q", written)
	}

	if read := readBack(t, written, "dna_test"); read.RandomSeed != "42" || read.Sex != "male" {
		t.Errorf("read back %+v", read)
	}
}

// TestInstalledDNAStrings reads the DNA strings the game's files of
// common/dna_data give in a comment on their first line, which the game's
// own portrait editor writes out beside the genes. A string as long as the
// genes are reads as the genes of the file, unless they were changed by hand
// since, and every string is written back as it was.
func TestInstalledDNAStrings(t *testing.T) {
	sources := installedSources(t)
	set := folders.Open(sources)
	genes := Load(set)
	files, _ := set.Files(FolderDNA)

	var read, current, same int

	for _, file := range files {
		text, err := os.ReadFile(file.Path)
		if err != nil {
			t.Fatal(err)
		}

		first, _, _ := strings.Cut(strings.TrimPrefix(string(text), string(rune(0xFEFF))), "\n")

		comment, ok := strings.CutPrefix(strings.TrimSpace(first), "#")
		if !ok {
			continue
		}

		data, err := decodeBase64(comment)
		if err != nil || len(data) < bytesPerGene*genes.ColorGenes.Len() {
			continue
		}

		read++

		// A string of other genes, or of a template its gene does not have,
		// is not read as it was meant, and so is not written back as it was.
		dna, diagnostics := genes.ReadString(comment)
		if len(diagnostics) > 0 {
			continue
		}

		current++

		if written, _ := genes.WriteString(dna); written != strings.TrimSpace(comment) {
			t.Errorf("%s: written back as\n%s\nwas\n%s", file.Relative, written, comment)
		}

		table := ReadDNA(script.Parse(string(text)), file, &report.Collector{})
		keys := table.Keys()

		if len(keys) != 1 {
			continue
		}

		given, _ := table.Get(keys[0])
		if sameGenes(dna, given) {
			same++
		}
	}

	t.Logf("%d DNA strings, %d of the genes there are, %d of them the genes of their file", read, current, same)

	if current == 0 || same < current*9/10 {
		t.Errorf("%d of %d DNA strings read as the genes of their file", same, current)
	}
}

// sameGenes reports whether every gene of a DNA read from a string is given
// the same by a DNA of a file, which can leave genes out.
func sameGenes(fromString, fromFile *DNA) bool {
	for _, value := range fromString.Genes {
		if given, ok := fromFile.Gene(value.Gene); ok && *given != value {
			return false
		}
	}

	return true
}

// Reading a DNA string never fails, and what it reads writes back as a string
// of every gene.
func FuzzReadString(f *testing.F) {
	f.Add(base64.StdEncoding.EncodeToString(stringTestBytes))
	f.Add("AAAA")
	f.Add("not base64!")
	f.Add("")

	genes := Read(script.Parse(stringTestGenes), testFile, &report.Collector{})

	f.Fuzz(func(t *testing.T, text string) {
		dna, _ := genes.ReadString(text)

		written, _ := genes.WriteString(dna)
		if data, err := base64.StdEncoding.DecodeString(written); err != nil || len(data) != len(stringTestBytes) {
			t.Errorf("%q written back as %q", text, written)
		}
	})
}
