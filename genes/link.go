package genes

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// link checks that the age presets the morph genes name exist. One that does
// not is reported, once for each gene, and kept as written; Genes.Age then
// reports it as not found.
func link(genes *Genes, collector *report.Collector) {
	check := func(table database.Table[MorphGene], kind string) {
		for _, gene := range table.All() {
			reported := map[string]bool{}

			note := func(age Age) {
				if age.Preset == "" || reported[age.Preset] || genes.AgePresets.Has(age.Preset) {
					return
				}

				reported[age.Preset] = true

				origin := gene.Origin()
				collector.Addf(report.SeverityWarning, origin.Source, origin.Path, origin.Line, 0, kind+" "+gene.Key,
					"age preset %s does not exist", age.Preset)
			}

			for _, template := range gene.Templates {
				for _, morph := range template.Sexes {
					for _, setting := range morph.Settings {
						note(setting.Age)
					}

					for _, decal := range morph.Decals {
						note(decal.Age)
					}

					for _, shift := range morph.HSVShifts {
						note(shift.Age)
					}
				}
			}
		}
	}

	check(genes.MorphGenes, kindMorphGene)
	check(genes.SpecialMorphGenes, kindSpecialMorph)
}
