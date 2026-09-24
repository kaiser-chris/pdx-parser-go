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

// The coats of arms are read apart from the economy. The example mod repaints
// the game's flag by injecting a colour into it, adds one of its own, and
// replaces one of the game's emblem files.
func ExampleLoadHeraldry() {
	set := folders.Open([]folders.Source{
		{Name: "Victoria 3", Path: "../testdata/game"},
		{Path: "../testdata/mod"},
	})

	heraldry := victoria3.LoadHeraldry(set)

	fmt.Println(heraldry.CoatOfArms.Keys(), heraldry.Templates.Keys())

	arms, _ := heraldry.CoatOfArms.Get("EXA")
	fmt.Println(arms.Pattern.File, len(arms.Layers), "layer")

	first, _ := arms.Colors.Get("color1")
	fmt.Println("color1 is", first.Value.Describe(), "from", arms.Origins[len(arms.Origins)-1].Source)

	red, _ := first.Resolve(heraldry.Colors, arms.Colors)
	fmt.Printf("which is %d %d %d\n", red.R, red.G, red.B)

	emblem := arms.Layers[0].(*victoria3.ColoredEmblem)
	for _, placement := range emblem.Placements() {
		fmt.Println(emblem.Texture.File, "at", placement.Position, "scaled", placement.Scale)
	}

	for _, texture := range heraldry.Textures {
		fmt.Println(texture.Kind, texture.Relative, "from", texture.Source)
	}

	// Output:
	// [EXA EXB MOD] [template_plain]
	// pattern_solid.tga 1 layer
	// color1 is crimson from Example Mod
	// which is 140 9 7
	// ce_star.dds at {0.5 0.5} scaled {0.6 0.6}
	// ce_star.dds at {0.25 0.5} scaled {0.3 0.3}
	// pattern gfx/coat_of_arms/patterns/pattern_solid.tga from Victoria 3
	// pattern gfx/coat_of_arms/patterns/pattern_vertical_split.tga from Victoria 3
	// colored emblem gfx/coat_of_arms/colored_emblems/ce_star.dds from Example Mod
	// textured emblem gfx/coat_of_arms/textured_emblems/te_eagle.dds from Victoria 3
}
