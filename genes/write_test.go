package genes

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

var dnaFile = folders.File{Relative: "common/dna_data/test.txt", Path: "test.txt", Source: "test"}

// readBack reads a written DNA again, and fails on anything worth a warning.
func readBack(t *testing.T, written, key string) *DNA {
	t.Helper()

	document := script.Parse(written)
	for _, warning := range document.Warnings {
		t.Errorf("parse: %s", warning)
	}

	collector := &report.Collector{}
	table := ReadDNA(document, dnaFile, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("read: %s", diagnostic)
	}

	dna, ok := table.Get(key)
	if !ok {
		t.Fatalf("%s is not in\n%s", key, written)
	}

	return dna
}

// A written DNA is read back as the same genes, colours, morphs and
// accessories alike, in the same order, as the game writes them.
func TestWriteDNA(t *testing.T) {
	dna := &DNA{Enabled: true, Genes: []GeneValue{
		{Gene: "hair_color", Dominant: Allele{Value: 3, Second: 248}, Recessive: Allele{Value: 4, Second: 247}},
		{Gene: "gene_cheek_fat", Dominant: Allele{Template: "cheek_fat", Value: 155}, Recessive: Allele{Template: "cheek_fat", Value: 139}},
		{Gene: "hairstyles", Dominant: Allele{Template: "historical_hairstyles"}, Recessive: Allele{Template: "european_hairstyles", Value: 214}},
		{Gene: "gene_shrink_body", Dominant: Allele{Template: "shrink_all", Value: 255}},
	}}

	written := WriteDNA("dna_test", dna, "\n")

	want := `dna_test = {
    portrait_info = {
        genes = {
            hair_color = { 3 248 4 247 }
            gene_cheek_fat = { "cheek_fat" 155 "cheek_fat" 139 }
            hairstyles = { "historical_hairstyles" 0 "european_hairstyles" 214 }
            gene_shrink_body = { "shrink_all" 255 "" 0 }
        }
    }
    enabled = yes
}`
	if written != want {
		t.Errorf("written\n%s\nwant\n%s", written, want)
	}

	read := readBack(t, written, "dna_test")
	if !read.Enabled || !slices.Equal(read.Genes, dna.Genes) {
		t.Errorf("read back %+v, want %+v", read.Genes, dna.Genes)
	}

	// A DNA the game does not use says so, and the lines end as asked.
	dna.Enabled = false

	written = WriteDNA("dna_test", dna, "\r\n")
	if !strings.Contains(written, "enabled = no\r\n") || strings.Count(written, "\n") != strings.Count(written, "\r\n") {
		t.Errorf("written with \r\n and disabled:\n%q", written)
	}

	if readBack(t, written, "dna_test").Enabled {
		t.Error("a disabled DNA is read back as enabled")
	}

	// No DNA at all is a definition of no genes.
	if read := readBack(t, WriteDNA("dna_empty", nil, "\n"), "dna_empty"); len(read.Genes) != 0 || !read.Enabled {
		t.Errorf("an empty DNA is read back as %+v", read)
	}
}

// Every DNA of an installed game, written again, is read back as the genes it
// had.
func TestWriteInstalledDNA(t *testing.T) {
	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	table, _ := LoadDNA(folders.Open([]folders.Source{{Name: "game", Path: game}}))

	count := 0

	for _, dna := range table.All() {
		if dna.String != "" || len(dna.Genes) == 0 {
			continue
		}

		read := readBack(t, WriteDNA(dna.Key, dna, "\n"), dna.Key)
		if !slices.Equal(read.Genes, dna.Genes) || read.Enabled != dna.Enabled {
			for index := range min(len(read.Genes), len(dna.Genes)) {
				if read.Genes[index] != dna.Genes[index] {
					t.Errorf("%s: %+v read back as %+v", dna.Key, dna.Genes[index], read.Genes[index])

					break
				}
			}

			t.Errorf("%s read back differently: %d genes, %d read, enabled %v %v", dna.Key, len(dna.Genes), len(read.Genes), dna.Enabled, read.Enabled)
		}

		count++
	}

	t.Logf("%d DNA written and read back", count)
}
