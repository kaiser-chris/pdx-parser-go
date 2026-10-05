package asset_test

import (
	"fmt"

	"github.com/kaiser-chris/pdx-parser-go/asset"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
	"github.com/kaiser-chris/pdx-parser-go/script"
)

// The example game defines a statue, and the example mod an entity that
// clones it with a wreath attached.
func ExampleLoad() {
	set := folders.Open([]folders.Source{
		{Name: "Victoria 3", Path: "../testdata/game"},
		{Path: "../testdata/mod"},
	})

	assets := asset.Load(set)

	statue, _ := assets.Meshes.Get("statue_mesh")
	fmt.Println(statue.File, statue.Scale, statue.Settings[0].Diffuse)

	// The files a definition names are relative to its asset file.
	fmt.Println(asset.Resolve(statue.Origin(), statue.File))
	fmt.Println(asset.Resolve(statue.Origin(), statue.Animations[0].File))

	wreathed, _ := assets.Entities.Get("wreathed_statue_entity")
	fmt.Println(wreathed.Origin().Source, wreathed.Clone, wreathed.Attachments[0])

	// The entity names no mesh of its own, but draws the one it clones.
	mesh, _ := assets.MeshOf("wreathed_statue_entity")
	fmt.Println(mesh.Key)

	fmt.Println(len(assets.Diagnostics.Filter(report.SeverityWarning)))

	// Output:
	// statue.mesh 2 statue_diffuse.dds
	// gfx/models/statues/statue.mesh
	// gfx/models/animation/statue_idle.anim
	// Example Mod statue_entity {plinth wreath_entity decoration}
	// statue_mesh
	// 0
}

// A single file, for a tool that has one in hand rather than a whole game.
func ExampleRead() {
	document := script.Parse(`
pdxmesh = { name = "crate_mesh" file = "crate.mesh" }
entity = { name = "crate_entity" pdxmesh = "crate_mesh" }
entity = { name = "barrel_entity" pdxmesh = "barrel_mesh" }
`)

	file := folders.File{Relative: "gfx/models/props/props.asset", Path: "props.asset", Source: "my mod"}
	collector := &report.Collector{}

	assets := asset.Read(document, file, collector)

	fmt.Println(assets.Entities.Keys())

	for _, diagnostic := range collector.Diagnostics {
		fmt.Println(diagnostic)
	}

	// Output:
	// [crate_entity barrel_entity]
	// warning: props.asset:4:45: entity barrel_entity: pdxmesh barrel_mesh does not exist
}
