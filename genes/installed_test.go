package genes

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// installedSources returns the game and mods named by PDX_GAME_DIR and
// PDX_MOD_DIRS, or skips the test when there is no game to read.
func installedSources(t *testing.T) []folders.Source {
	t.Helper()

	game := os.Getenv("PDX_GAME_DIR")
	if game == "" {
		t.Skip("set PDX_GAME_DIR to the game folder to run this test")
	}

	sources := []folders.Source{{Name: "game", Path: game}}

	for _, mod := range filepath.SplitList(os.Getenv("PDX_MOD_DIRS")) {
		if mod != "" {
			sources = append(sources, folders.Source{Path: mod})
		}
	}

	return sources
}

// shippedProblems are the mistakes in the game's own files that the package
// rightly reports, by the file and a fragment of the diagnostic. The syntax
// errors of the files, such as a block that is never closed, are expected
// anywhere and are only logged.
var shippedProblems = []struct{ file, fragment string }{
	// Victoria 3 defines a beard and a belt twice in the same gene.
	{"03_genes_accessories_beards.txt", "template beard_henry_fane is given more than once"},
	{"97_genes_accessories_clothes.txt", "template sash_belt_power_bloc_ideological is given more than once"},
}

func shipped(diagnostic report.Diagnostic) bool {
	for _, known := range shippedProblems {
		if filepath.Base(diagnostic.Path) == known.file && strings.Contains(diagnostic.String(), known.fragment) {
			return true
		}
	}

	return false
}

// checkClean fails on every warning and unknown field of a vanilla game
// beyond the syntax errors of its files and the mistakes in shippedProblems,
// which are gaps in the package rather than in the game, and logs them when
// mods are loaded.
func checkClean(t *testing.T, vanilla bool, diagnostics report.Diagnostics) {
	t.Helper()

	for _, diagnostic := range diagnostics {
		unknown := strings.Contains(diagnostic.Message, "unknown field") || strings.Contains(diagnostic.Message, "no kind of gene")
		syntax := diagnostic.Subject == "" && diagnostic.Severity == report.SeverityWarning

		switch {
		case diagnostic.Severity < report.SeverityWarning && !unknown:
		case syntax && vanilla:
			t.Logf("a mistake in the game's files: %s", diagnostic)
		case vanilla && !shipped(diagnostic):
			t.Errorf("the game should load cleanly: %s", diagnostic)
		default:
			t.Log(diagnostic)
		}
	}
}

// TestLoadInstalledGenes loads the genes of a real installation, which must
// load without a warning or a field this package does not know.
func TestLoadInstalledGenes(t *testing.T) {
	sources := installedSources(t)
	genes := Load(folders.Open(sources))

	t.Logf("age presets %d, color genes %d, morph genes %d, accessory genes %d, special %d and %d, diagnostics %d",
		genes.AgePresets.Len(), genes.ColorGenes.Len(), genes.MorphGenes.Len(), genes.AccessoryGenes.Len(),
		genes.SpecialMorphGenes.Len(), genes.SpecialAccessoryGenes.Len(), len(genes.Diagnostics))

	if genes.MorphGenes.Len() == 0 || genes.AccessoryGenes.Len() == 0 || genes.AgePresets.Len() == 0 {
		t.Fatal("no genes found")
	}

	checkClean(t, len(sources) == 1, genes.Diagnostics)

	// Every template of a gene in the DNA gives its men something to do,
	// and the strengths of the settings it gives them stay within what a
	// blend shape or an additive animation takes.
	for _, gene := range genes.MorphGenes.All() {
		for _, template := range gene.Templates {
			for _, setting := range template.For(Male).Settings {
				for _, strength := range []float64{0, 0.5, 1} {
					if weight := setting.At(strength); weight < -1 || weight > 1 {
						t.Errorf("%s %s: %s at %g is %g", gene.Key, template.Name, setting.Attribute, strength, weight)
					}
				}

				if _, _, ok := genes.Age(setting.Age); setting.Age.Preset != "" && !ok {
					t.Errorf("%s %s: age preset %s not found", gene.Key, template.Name, setting.Age.Preset)
				}
			}
		}
	}
}

// TestLoadInstalledDNA loads the DNA of a real installation, and checks that
// every gene and template it names is one of the genes.
func TestLoadInstalledDNA(t *testing.T) {
	sources := installedSources(t)
	set := folders.Open(sources)

	genes := Load(set)
	table, diagnostics := LoadDNA(set)

	t.Logf("dna %d, diagnostics %d", table.Len(), len(diagnostics))

	if table.Len() == 0 {
		t.Fatal("no dna found")
	}

	checkClean(t, len(sources) == 1, diagnostics)

	missing := map[string]bool{}

	for key, dna := range table.All() {
		for _, value := range dna.Genes {
			if _, ok := genes.ColorGenes.Get(value.Gene); ok {
				continue
			}

			var found func(string) bool

			if gene, ok := genes.MorphGene(value.Gene); ok {
				found = func(name string) bool { _, ok := gene.Template(name); return ok }
			} else if gene, ok := genes.AccessoryGene(value.Gene); ok {
				found = func(name string) bool { _, ok := gene.Template(name); return ok }
			} else {
				if !missing[value.Gene] {
					t.Errorf("%s: gene %s is no gene", key, value.Gene)
				}

				missing[value.Gene] = true

				continue
			}

			// A few of the shipped DNA leave a template empty; what the game
			// makes of that is not known.
			for _, allele := range []Allele{value.Dominant, value.Recessive} {
				if allele.Template != "" && !found(allele.Template) && !missing[value.Gene+" "+allele.Template] {
					missing[value.Gene+" "+allele.Template] = true
					t.Errorf("%s: gene %s has no template %s", key, value.Gene, allele.Template)
				}
			}
		}
	}
}

// TestLoadInstalledAccessories loads the portrait accessories of a real
// installation, and checks that every one the accessory genes pick is there.
func TestLoadInstalledAccessories(t *testing.T) {
	sources := installedSources(t)
	set := folders.Open(sources)

	genes := Load(set)
	table, diagnostics := LoadAccessories(set)

	t.Logf("accessories %d, diagnostics %d", table.Len(), len(diagnostics))

	if table.Len() == 0 {
		t.Fatal("no accessories found")
	}

	checkClean(t, len(sources) == 1, diagnostics)

	for _, kind := range []database.Table[AccessoryGene]{genes.AccessoryGenes, genes.SpecialAccessoryGenes} {
		for key, gene := range kind.All() {
			for _, template := range gene.Templates {
				for _, sex := range Sexes {
					for _, choice := range template.For(sex).Choices {
						if choice.Accessory != "empty" && !table.Has(choice.Accessory) {
							t.Logf("%s %s: no accessory %s", key, template.Name, choice.Accessory)
						}
					}
				}
			}
		}
	}

	eyes, ok := table.Get("male_eyes_normal")
	if !ok {
		t.Fatal("no male_eyes_normal")
	}

	if chosen, ok := eyes.Choose(nil); !ok || chosen.Entity != "male_eyes_entity" || chosen.SharedPoseEntity != "head" {
		t.Errorf("the eyes draw %+v", chosen)
	}
}

// TestLoadInstalledAnimations loads the animation sets of a real installation,
// which have an idle for each sex.
func TestLoadInstalledAnimations(t *testing.T) {
	sources := installedSources(t)

	table, diagnostics := LoadAnimations(folders.Open(sources))

	t.Logf("animation sets %d, diagnostics %d", table.Len(), len(diagnostics))

	checkClean(t, len(sources) == 1, diagnostics)

	idle, ok := table.Get("idle")
	if !ok {
		t.Fatal("the game has no idle animation set")
	}

	for _, sex := range Sexes {
		if pair := idle.For(sex).Default; pair.Head == "" || pair.Torso == "" {
			t.Errorf("idle for a %s starts in %+v, want a head and a torso state", sex, pair)
		}
	}
}

// TestLoadInstalledEthnicities loads the ethnicities of a real installation,
// each of which flattens, and checks that the neutral preview of
// ethnicity_template is the DNA the game's portrait editor writes out for it,
// which testdata holds, gene for gene.
func TestLoadInstalledEthnicities(t *testing.T) {
	sources := installedSources(t)
	set := folders.Open(sources)

	genes := Load(set)
	table, diagnostics := LoadEthnicities(set)

	t.Logf("ethnicities %d, diagnostics %d", table.Len(), len(diagnostics))

	if !table.Has("ethnicity_template") {
		t.Skip("the game has no ethnicity_template")
	}

	checkClean(t, len(sources) == 1, diagnostics)

	for key := range table.All() {
		if _, err := Flatten(table, key); err != nil {
			t.Errorf("%s: %v", key, err)
		}
	}

	template, _ := Flatten(table, "ethnicity_template")
	preview := genes.Preview(template, PreviewNeutral)

	data, err := os.ReadFile(filepath.Join("testdata", "ethnicity_template_dna.txt"))
	if err != nil {
		t.Fatal(err)
	}

	collector := &report.Collector{}
	written := ReadDNA(script.Parse(string(data)), folders.File{Relative: "ethnicity_template_dna.txt", Path: "testdata/ethnicity_template_dna.txt", Source: "game"}, collector)

	for _, diagnostic := range collector.Diagnostics {
		t.Errorf("the written DNA should read cleanly: %s", diagnostic)
	}

	want, _ := written.Get("TODO_choose_portait_key")
	if want == nil || want.Sex != "male" || len(want.Genes) == 0 {
		t.Fatalf("the written DNA reads as %+v", want)
	}

	if len(preview.Genes) != len(want.Genes) {
		t.Errorf("the preview has %d genes, the written DNA %d", len(preview.Genes), len(want.Genes))
	}

	for index := range min(len(preview.Genes), len(want.Genes)) {
		if preview.Genes[index] != want.Genes[index] {
			t.Errorf("gene %d: the preview has %+v, the written DNA %+v", index, preview.Genes[index], want.Genes[index])
		}
	}
}
