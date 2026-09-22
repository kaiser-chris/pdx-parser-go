package victoria3_test

import (
	"fmt"

	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/victoria3"
)

// The example game has one steel mill with one production method. The example
// mod adds a second method to its group, a new good, and a higher price for
// steel.
func ExampleLoad() {
	set := folders.Open([]folders.Source{
		{Name: "Victoria 3", Path: "../testdata/game"},
		{Path: "../testdata/mod"},
	})

	data := victoria3.Load(set)

	fmt.Println(data.Sources)
	fmt.Println(data.Goods.Keys())

	steel, _ := data.Goods.Get("steel")
	fmt.Println("steel costs", steel.Cost)

	group, _ := data.ProductionMethodGroups.Get("pmg_steelmaking")
	fmt.Println(group.ProductionMethods)

	for _, key := range group.ProductionMethods {
		method, _ := data.ProductionMethods.Get(key)
		fmt.Println(key, method.Inputs(), method.Outputs())
	}

	fmt.Println(len(data.Diagnostics), "diagnostics")

	// Output:
	// [Victoria 3 Example Mod]
	// [iron coal steel aluminium]
	// steel costs 55
	// [pm_bessemer pm_open_hearth]
	// pm_bessemer [{iron 60 workforce_scaled} {coal 30 workforce_scaled}] [{steel 90 workforce_scaled}]
	// pm_open_hearth [{iron 90 workforce_scaled} {coal 30 workforce_scaled}] [{steel 120 workforce_scaled}]
	// 0 diagnostics
}

// Settings of a building group that its children inherit are read with
// ResolveGroup, which walks up to the nearest group that sets them.
func ExampleResolveGroup() {
	data := victoria3.Load(folders.Open([]folders.Source{{Path: "../testdata/game"}}))

	mill, _ := data.Buildings.Get("building_steel_mill")

	economyOfScale, set := victoria3.ResolveGroup(data, mill.BuildingGroup,
		func(group *victoria3.BuildingGroup) database.Opt[bool] { return group.EconomyOfScale })

	fmt.Println(economyOfScale, set)

	// Output:
	// true true
}
