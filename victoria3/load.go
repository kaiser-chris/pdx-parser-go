// Package victoria3 reads the economic definitions of Victoria 3 and its
// mods: goods, buildings and their groups, production methods and their
// groups, pop types, pop needs, buy packages, companies, technologies, eras and
// defines.
//
// This is the part of the module that follows the game. Its types describe the
// fields the game's files use as of version 1.13, and a game update that adds
// or renames fields makes it incomplete rather than broken: unknown fields
// are reported and kept in each definition's raw script, and missing ones take
// their defaults. The packages it builds on, script, folders and database,
// know nothing about Victoria 3 and are not expected to change with it.
//
// Load takes the game and any mods, already combined by the folders package:
//
//	set := folders.Open([]folders.Source{
//		{Name: "Victoria 3", Path: `C:\Steam\steamapps\common\Victoria 3\game`},
//		{Path: `C:\mods\my_mod`},
//	})
//
//	data := victoria3.Load(set)
//
//	steel, _ := data.Goods.Get("steel")
//	fmt.Println(steel.Cost)
//
// Loading never fails. Everything worth knowing about the files, from syntax
// errors to references to things that do not exist, is in Data.Diagnostics.
package victoria3

import (
	"github.com/kaiser-chris/pdx-parser-go/database"
	"github.com/kaiser-chris/pdx-parser-go/folders"
	"github.com/kaiser-chris/pdx-parser-go/report"
)

// Data is everything read from a set of folders.
type Data struct {
	// Sources are the names of the folders that could be read, in load order.
	Sources []string

	Goods                  database.Table[Good]
	BuildingGroups         database.Table[BuildingGroup]
	Buildings              database.Table[Building]
	ProductionMethodGroups database.Table[ProductionMethodGroup]
	ProductionMethods      database.Table[ProductionMethod]
	PopTypes               database.Table[PopType]
	PopNeeds               database.Table[PopNeed]
	BuyPackages            database.Table[BuyPackage]
	Companies              database.Table[Company]
	Technologies           database.Table[Technology]
	Eras                   database.Table[Era]
	Defines                database.Defines

	// Diagnostics are the problems found in the files. Problems with the
	// folders themselves are in the set's own Diagnostics.
	Diagnostics report.Diagnostics
}

// The folders each kind of definition is read from.
var (
	FolderGoods                  = folders.Folder{Path: "common/goods"}
	FolderBuildingGroups         = folders.Folder{Path: "common/building_groups"}
	FolderBuildings              = folders.Folder{Path: "common/buildings"}
	FolderProductionMethodGroups = folders.Folder{Path: "common/production_method_groups"}
	FolderProductionMethods      = folders.Folder{Path: "common/production_methods"}
	FolderPopTypes               = folders.Folder{Path: "common/pop_types"}
	FolderPopNeeds               = folders.Folder{Path: "common/pop_needs"}
	FolderBuyPackages            = folders.Folder{Path: "common/buy_packages"}
	FolderCompanies              = folders.Folder{Path: "common/company_types"}
	FolderTechnologies           = folders.Folder{Path: "common/technology/technologies"}
	FolderEras                   = folders.Folder{Path: "common/technology/eras"}
)

// Load reads the economic definitions of a set of folders.
func Load(set *folders.Set) *Data {
	collector := &report.Collector{}
	data := &Data{Sources: set.Names()}

	collect := func(folder folders.Folder, kind string) *database.Store {
		return database.Collect(set, folder, kind, collector)
	}

	data.Defines = database.LoadDefines(set, collector)

	data.Goods = database.Decode(collect(FolderGoods, "good"), collector,
		func(r *database.Reader, _ *database.Definition) *Good { return decodeGood(r, data.Defines) })

	data.BuildingGroups = database.Decode(collect(FolderBuildingGroups, "building group"), collector,
		func(r *database.Reader, _ *database.Definition) *BuildingGroup { return decodeBuildingGroup(r) })

	data.Buildings = database.Decode(collect(FolderBuildings, "building"), collector,
		func(r *database.Reader, _ *database.Definition) *Building { return decodeBuilding(r) })

	data.ProductionMethodGroups = database.Decode(collect(FolderProductionMethodGroups, "production method group"), collector,
		func(r *database.Reader, _ *database.Definition) *ProductionMethodGroup {
			return decodeProductionMethodGroup(r)
		})

	data.ProductionMethods = database.Decode(collect(FolderProductionMethods, "production method"), collector,
		func(r *database.Reader, _ *database.Definition) *ProductionMethod { return decodeProductionMethod(r) })

	data.PopTypes = database.Decode(collect(FolderPopTypes, "pop type"), collector,
		func(r *database.Reader, _ *database.Definition) *PopType { return decodePopType(r) })

	data.PopNeeds = database.Decode(collect(FolderPopNeeds, "pop need"), collector,
		func(r *database.Reader, _ *database.Definition) *PopNeed { return decodePopNeed(r) })

	data.BuyPackages = database.Decode(collect(FolderBuyPackages, "buy package"), collector,
		func(r *database.Reader, definition *database.Definition) *BuyPackage {
			return decodeBuyPackage(r, definition.Key)
		})

	data.Companies = database.Decode(collect(FolderCompanies, "company"), collector,
		func(r *database.Reader, _ *database.Definition) *Company { return decodeCompany(r) })

	data.Technologies = database.Decode(collect(FolderTechnologies, "technology"), collector,
		func(r *database.Reader, _ *database.Definition) *Technology { return decodeTechnology(r) })

	eraIndex := 0
	data.Eras = database.Decode(collect(FolderEras, "era"), collector,
		func(r *database.Reader, definition *database.Definition) *Era {
			era := decodeEra(r, eraIndex, definition.Comment)
			eraIndex++

			return era
		})

	link(data, collector)

	data.Diagnostics = collector.Diagnostics

	return data
}
