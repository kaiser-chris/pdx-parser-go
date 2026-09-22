package database_test

import (
	"fmt"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// Good is a decoder's own type for a definition. It embeds Definition, which
// carries the key, the origins and the raw script.
type Good struct {
	database.Definition

	Cost     float64
	Category string
	Local    bool
}

// A decoder for a kind of definition the module has no type for. The example
// mod injects a higher cost into the game's steel, and adds a good.
func ExampleDecode() {
	set := folders.Open([]folders.Source{
		{Name: "Victoria 3", Path: "../testdata/game"},
		{Path: "../testdata/mod"},
	})

	collector := &report.Collector{}
	store := database.Collect(set, folders.Folder{Path: "common/goods"}, "good", collector)

	goods := database.Decode(store, collector, func(r *database.Reader, _ *database.Definition) *Good {
		// Fields this decoder has no use for are marked as known, so they are
		// not reported as unknown.
		r.Known("texture")

		return &Good{
			Cost:     r.NumberOr("cost", 0),
			Category: r.Text("category"),
			Local:    r.FlagOr("local", false),
		}
	})

	for key, good := range goods.All() {
		fmt.Println(key, good.Cost, good.Category)
	}

	steel, _ := goods.Get("steel")
	for _, origin := range steel.Origins {
		fmt.Println(origin.Source, origin.Mode, origin.File)
	}

	fmt.Println(len(collector.Diagnostics))

	// Output:
	// iron 40 industrial
	// coal 30 industrial
	// steel 55 industrial
	// aluminium 70 industrial
	// Victoria 3 CREATE common/goods/00_goods.txt
	// Example Mod INJECT common/goods/zz_mod_goods.txt
	// 0
}

func ExampleLoadDefines() {
	set := folders.Open([]folders.Source{{Path: "../testdata/game"}})

	defines := database.LoadDefines(set, &report.Collector{})

	fmt.Println(defines.NumberOr("NEconomy", "PRICE_RANGE", 0))
	fmt.Println(defines.NumberOr("NEconomy", "NOT_THERE", 1))

	// Output:
	// 0.75
	// 1
}
