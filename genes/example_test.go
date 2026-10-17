package genes_test

import (
	"fmt"

	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/genes"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// The example game defines a gene for the size of the nose, which morphs
// between two blend shapes, and the example mod a gene for the width of the
// chin.
func ExampleLoad() {
	set := folders.Open([]folders.Source{
		{Name: "Victoria 3", Path: "../testdata/game"},
		{Path: "../testdata/mod"},
	})

	loaded := genes.Load(set)
	fmt.Println(loaded.MorphGenes.Keys())

	nose, _ := loaded.MorphGenes.Get("gene_nose_size")
	template, _ := nose.Template("nose_size")

	// A girl is the same as a woman, who is the same as a man.
	girl := template.For(genes.Girl)
	fmt.Println(girl.SameAs)

	// The strength of the gene morphs towards one blend shape or the other.
	for _, strength := range []float64{0, 0.5, 0.75} {
		fmt.Printf("%.2f:", strength)

		for _, setting := range girl.Settings {
			fmt.Printf(" %s %.2f", setting.Attribute, setting.At(strength))
		}

		fmt.Println()
	}

	fmt.Println(len(loaded.Diagnostics.Filter(report.SeverityWarning)))

	// Output:
	// [gene_nose_size gene_chin_width]
	// male
	// 0.00: head_bs_nose_size_max 0.00 head_bs_nose_size_min 1.00
	// 0.50: head_bs_nose_size_max 0.00 head_bs_nose_size_min 0.00
	// 0.75: head_bs_nose_size_max 0.50 head_bs_nose_size_min 0.00
	// 0
}

// A DNA gives every gene a template and a strength, and every colour gene a
// place in its palette.
func ExampleLoadDNA() {
	set := folders.Open([]folders.Source{{Name: "Victoria 3", Path: "../testdata/game"}})

	loaded := genes.Load(set)
	dna, _ := genes.LoadDNA(set)

	statesman, _ := dna.Get("dna_example_statesman")

	hair, _ := statesman.Gene("hair_color")
	fmt.Println(hair.Dominant.Value, hair.Dominant.Second)

	value, _ := statesman.Gene("gene_nose_size")
	fmt.Printf("%s %.2f\n", value.Dominant.Template, value.Dominant.Strength())

	gene, _ := loaded.MorphGenes.Get(value.Gene)
	template, _ := gene.Template(value.Dominant.Template)

	for _, setting := range template.For(genes.Male).Settings {
		fmt.Printf("%s %.2f\n", setting.Attribute, setting.At(value.Dominant.Strength()))
	}

	// Output:
	// 14 244
	// nose_size 0.75
	// head_bs_nose_size_max 0.50
	// head_bs_nose_size_min 0.00
}
